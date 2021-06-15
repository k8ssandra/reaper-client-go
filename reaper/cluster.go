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

type Cluster struct {
	Name           string
	JmxUsername    string
	JmxPasswordSet bool
	Seeds          []string
	NodeState      NodeState
}

type NodeState struct {
	GossipStates []GossipState
}

type GossipState struct {
	SourceNode    string
	EndpointNames []string
	TotalLoad     float64
	DataCenters   map[string]DataCenterState
}

type DataCenterState struct {
	Name  string
	Racks map[string]RackState
}

type RackState struct {
	Name      string
	Endpoints []EndpointState
}

type EndpointState struct {
	Endpoint       string
	DataCenter     string
	Rack           string
	HostId         string
	Status         string
	Severity       float64
	ReleaseVersion string
	Tokens         string
	Load           float64
}

type GetClusterResult struct {
	Cluster *Cluster
	Error   error
}

type RepairSchedule struct {
	Id                string    `json:"id"`
	Owner             string    `json:"owner,omitempty"`
	State             string    `json:"state,omitempty"`
	Intensity         float64   `json:"intensity,omitempty"`
	ClusterName       string    `json:"cluster_name,omitempty"`
	KeyspaceName      string    `json:"keyspace_name,omitempty"`
	RepairParallelism string    `json:"repair_parallelism,omitempty"`
	IncrementalRepair bool      `json:"incremental_repair,omitempty"`
	ThreadCount       int       `json:"repair_thread_count,omitempty"`
	UnitId            string    `json:"repair_unit_id,omitempty"`
	DaysBetween       int       `json:"scheduled_days_between,omitempty"`
	Created           time.Time `json:"creation_time,omitempty"`
	Paused            time.Time `json:"pause_time,omitempty"`
	NextActivation    time.Time `json:"next_activation,omitempty"`
}

// All the following types are used internally by the client and not part of the public API

type clusterStatus struct {
	Name           string     `json:"name"`
	JmxUsername    string     `json:"jmx_username,omitempty"`
	JmxPasswordSet bool       `json:"jmx_password_is_set,omitempty"`
	Seeds          []string   `json:"seed_hosts,omitempty"`
	NodeStatus     nodeStatus `json:"nodes_status"`
}

type nodeStatus struct {
	EndpointStates []gossipStatus `json:"endpointStates,omitempty"`
}

type gossipStatus struct {
	SourceNode    string   `json:"sourceNode"`
	EndpointNames []string `json:"endpointNames,omitempty"`
	TotalLoad     float64  `json:"totalLoad,omitempty"`
	Endpoints     map[string]map[string][]endpointStatus
}

type endpointStatus struct {
	Endpoint       string  `json:"endpoint"`
	DataCenter     string  `json:"dc"`
	Rack           string  `json:"rack"`
	HostId         string  `json:"hostId"`
	Status         string  `json:"status"`
	Severity       float64 `json:"severity"`
	ReleaseVersion string  `json:"releaseVersion"`
	Tokens         string  `json:"tokens"`
	Load           float64 `json:"load"`
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

// GetClusters fetches all clusters. This function is async and may return before any or all results are
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

// GetClustersSync fetches all clusters in a synchronous or blocking manner. Note that this function fails
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
	queryParams := &url.Values{"seedHost": {seed}}
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
