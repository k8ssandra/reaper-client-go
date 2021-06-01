package reaper

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"time"
)

type Client interface {
	IsReaperUp(ctx context.Context) (bool, error)

	GetClusterNames(ctx context.Context) ([]string, error)

	GetCluster(ctx context.Context, name string) (*Cluster, error)

	// Fetches all clusters. This function is async and may return before any or all results are
	// available. The concurrency is currently determined by min(5, NUM_CPUS).
	GetClusters(ctx context.Context) <-chan GetClusterResult

	// Fetches all clusters in a synchronous or blocking manner. Note that this function fails
	// fast if there is an error and no clusters will be returned.
	GetClustersSync(ctx context.Context) ([]*Cluster, error)

	AddCluster(ctx context.Context, cluster string, seed string) error

	DeleteCluster(ctx context.Context, cluster string) error

	RepairSchedules(ctx context.Context) ([]RepairSchedule, error)

	RepairSchedulesForCluster(ctx context.Context, clusterName string) ([]RepairSchedule, error)
}

type client struct {
	baseURL    *url.URL
	userAgent  string
	httpClient *http.Client
}

func NewClient(reaperBaseURL *url.URL, options ...ClientCreateOption) Client {
	client := &client{baseURL: reaperBaseURL, httpClient: &http.Client{
		Timeout: 10 * time.Second,
	}}
	for _, option := range options {
		option(client)
	}
	return client
}

type ClientCreateOption func(client *client)

func WithUserAgent(userAgent string) ClientCreateOption {
	return func(client *client) {
		client.userAgent = userAgent
	}
}

func WithHttpClient(httpClient *http.Client) ClientCreateOption {
	return func(client *client) {
		client.httpClient = httpClient
	}
}

func (c *client) IsReaperUp(ctx context.Context) (bool, error) {
	if resp, err := c.doHead(ctx, "/ping", nil); err == nil {
		return resp.StatusCode == http.StatusNoContent, nil
	} else {
		return false, err
	}
}

func (c *client) GetClusterNames(ctx context.Context) ([]string, error) {
	res, err := c.doGet(ctx, "/cluster", nil, http.StatusOK)
	if err == nil {
		clusterNames := make([]string, 0)
		err = c.readBodyAsJson(res, &clusterNames)
		if err == nil {
			return clusterNames, nil
		}
	}
	return nil, fmt.Errorf("failed to get cluster names: %w", err)
}

func (c *client) GetCluster(ctx context.Context, name string) (*Cluster, error) {
	path := "/cluster/" + url.PathEscape(name)
	res, err := c.doGet(ctx, path, nil, http.StatusOK)
	if err == nil {
		clusterStatus := &clusterStatus{}
		err = c.readBodyAsJson(res, clusterStatus)
		if err == nil {
			return newCluster(clusterStatus), nil
		}
	}
	return nil, fmt.Errorf("failed to get cluster %s: %w", name, err)
}

// Fetches all clusters. This function is async and may return before any or all results are
// available. The concurrency is currently determined by min(5, NUM_CPUS).
func (c *client) GetClusters(ctx context.Context) <-chan GetClusterResult {
	// TODO Make the concurrency configurable
	concurrency := int(math.Min(5, float64(runtime.NumCPU())))
	results := make(chan GetClusterResult, concurrency)

	clusterNames, err := c.GetClusterNames(ctx)
	if err != nil {
		close(results)
		return results
	}

	var wg sync.WaitGroup

	go func() {
		defer close(results)
		for _, clusterName := range clusterNames {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				cluster, err := c.GetCluster(ctx, name)
				result := GetClusterResult{Cluster: cluster, Error: err}
				results <- result
			}(clusterName)
		}
		wg.Wait()
	}()

	return results
}

// Fetches all clusters in a synchronous or blocking manner. Note that this function fails
// fast if there is an error and no clusters will be returned.
func (c *client) GetClustersSync(ctx context.Context) ([]*Cluster, error) {
	clusters := make([]*Cluster, 0)

	for result := range c.GetClusters(ctx) {
		if result.Error != nil {
			return nil, result.Error
		}
		clusters = append(clusters, result.Cluster)
	}

	return clusters, nil
}

func (c *client) AddCluster(ctx context.Context, cluster string, seed string) error {
	queryParams := &url.Values{}
	queryParams.Set("seedHost", seed)
	path := "/cluster/" + url.PathEscape(cluster)
	_, err := c.doPut(ctx, path, queryParams, nil, http.StatusCreated, http.StatusNoContent, http.StatusOK)
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to create cluster %s: %w", cluster, err)
}

func (c *client) DeleteCluster(ctx context.Context, cluster string) error {
	path := "/cluster/" + url.PathEscape(cluster)
	_, err := c.doDelete(ctx, path, nil, http.StatusAccepted)
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to delete cluster %s: %w", cluster, err)
}

func (c *client) RepairSchedules(ctx context.Context) ([]RepairSchedule, error) {
	return c.fetchRepairSchedules(ctx, "/repair_schedule")
}

func (c *client) RepairSchedulesForCluster(ctx context.Context, clusterName string) ([]RepairSchedule, error) {
	path := "/repair_schedule/cluster/" + url.PathEscape(clusterName)
	return c.fetchRepairSchedules(ctx, path)
}

func (c *client) fetchRepairSchedules(ctx context.Context, path string) ([]RepairSchedule, error) {
	res, err := c.doGet(ctx, path, nil, http.StatusOK)
	if err == nil {
		repairSchedules := make([]RepairSchedule, 0)
		err = c.readBodyAsJson(res, &repairSchedules)
		if err == nil {
			return repairSchedules, nil
		}
	}
	return nil, fmt.Errorf("failed to fetch repair schedules: %w", err)
}

func newCluster(state *clusterStatus) *Cluster {
	cluster := Cluster{
		Name:           state.Name,
		JmxUsername:    state.JmxUsername,
		JmxPasswordSet: state.JmxPasswordSet,
		Seeds:          state.Seeds,
		NodeState:      NodeState{},
	}

	for _, gs := range state.NodeStatus.EndpointStates {
		gossipState := GossipState{
			SourceNode:    gs.SourceNode,
			EndpointNames: gs.EndpointNames,
			TotalLoad:     gs.TotalLoad,
			DataCenters:   map[string]DataCenterState{},
		}
		for dc, dcStateInternal := range gs.Endpoints {
			dcState := DataCenterState{Name: dc, Racks: map[string]RackState{}}
			for rack, endpoints := range dcStateInternal {
				rackState := RackState{Name: rack}
				for _, ep := range endpoints {
					endpoint := EndpointState{
						Endpoint:       ep.Endpoint,
						DataCenter:     ep.DataCenter,
						Rack:           ep.Rack,
						HostId:         ep.HostId,
						Status:         ep.Status,
						Severity:       ep.Severity,
						ReleaseVersion: ep.ReleaseVersion,
						Tokens:         ep.Tokens,
						Load:           ep.Load,
					}
					rackState.Endpoints = append(rackState.Endpoints, endpoint)
				}
				dcState.Racks[rack] = rackState
			}
			gossipState.DataCenters[dc] = dcState
		}
		cluster.NodeState.GossipStates = append(cluster.NodeState.GossipStates, gossipState)
	}

	return &cluster
}
