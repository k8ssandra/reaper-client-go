package reaper

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
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
	rel := &url.URL{Path: "/ping"}
	u := c.baseURL.ResolveReference(rel)
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return false, err
	}

	if resp, err := c.doRequest(ctx, req, nil); err == nil {
		return resp.StatusCode == http.StatusNoContent, nil
	} else {
		return false, err
	}
}

func (c *client) GetClusterNames(ctx context.Context) ([]string, error) {
	rel := &url.URL{Path: "/cluster"}
	u := c.baseURL.ResolveReference(rel)
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	//req.Header.Set("User-Agent", c.UserAgent)

	clusterNames := []string{}
	_, err = c.doJsonRequest(ctx, req, &clusterNames)

	if err != nil {
		return nil, fmt.Errorf("failed to get cluster names: %w", err)
	}

	return clusterNames, nil
}

func (c *client) GetCluster(ctx context.Context, name string) (*Cluster, error) {
	rel := &url.URL{Path: fmt.Sprintf("/cluster/%s", name)}
	u := c.baseURL.ResolveReference(rel)
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	clusterState := &clusterStatus{}
	resp, err := c.doJsonRequest(ctx, req, clusterState)

	if err != nil {
		fmt.Printf("response: %+v", resp)
		return nil, fmt.Errorf("failed to get cluster (%s): %w", name, err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, CassandraClusterNotFound
	}

	cluster := newCluster(clusterState)

	return cluster, nil
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
	rel := &url.URL{Path: fmt.Sprintf("/cluster/%s", cluster)}
	u := c.baseURL.ResolveReference(rel)

	req, err := http.NewRequest(http.MethodPut, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	q := req.URL.Query()
	q.Add("seedHost", seed)
	req.URL.RawQuery = q.Encode()
	req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode < 300:
		return nil
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return ErrRedirectsNotSupported
	default:
		if body, err := getBodyAsString(resp); err == nil {
			return fmt.Errorf("request failed: msg (%s), status code (%d)", body, resp.StatusCode)
		}
		log.Printf("failed to get response body: %s", err)
		return fmt.Errorf("request failed: status code (%d)", resp.StatusCode)
	}
}

func (c *client) DeleteCluster(ctx context.Context, cluster string) error {
	rel := &url.URL{Path: fmt.Sprintf("/cluster/%s", cluster)}
	u := c.baseURL.ResolveReference(rel)
	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}

	_, err = c.doJsonRequest(ctx, req, nil)

	// TODO check response status code

	if err != nil {
		return fmt.Errorf("failed to delete cluster (%s): %w", cluster, err)
	}

	return nil
}

func (c *client) RepairSchedules(ctx context.Context) ([]RepairSchedule, error) {
	rel := &url.URL{Path: "/repair_schedule"}
	return c.fetchRepairSchedules(ctx, rel)
}

func (c *client) RepairSchedulesForCluster(ctx context.Context, clusterName string) ([]RepairSchedule, error) {
	rel := &url.URL{Path: fmt.Sprintf("/repair_schedule/cluster/%s", clusterName)}
	return c.fetchRepairSchedules(ctx, rel)
}

func (c *client) fetchRepairSchedules(ctx context.Context, rel *url.URL) ([]RepairSchedule, error) {
	u := c.baseURL.ResolveReference(rel)
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)

	if err != nil {
		return nil, err
	}

	schedules := make([]RepairSchedule, 0)
	resp, err := c.doJsonRequest(ctx, req, &schedules)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to fetch repair schedules: %v\n", resp.StatusCode)
	}

	return schedules, nil
}

func (c *client) doJsonRequest(ctx context.Context, req *http.Request, v interface{}) (*http.Response, error) {
	req.Header.Set("Accept", "application/json")
	return c.doRequest(ctx, req, v)
}

func (c *client) doRequest(ctx context.Context, req *http.Request, v interface{}) (*http.Response, error) {
	req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return resp, nil
	}

	if v != nil {
		err = json.NewDecoder(resp.Body).Decode(v)
	}

	return resp, err
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

func getBodyAsString(resp *http.Response) (string, error) {
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
