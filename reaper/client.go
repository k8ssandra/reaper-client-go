package reaper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
)

type Client struct {
	BaseURL    *url.URL
	UserAgent  string
	httpClient *http.Client
}

func NewClient(reaperBaseURL string) (*Client, error) {
	if baseURL, err := url.Parse(reaperBaseURL); err != nil {
		return nil, err
	} else {
		return &Client{BaseURL: baseURL, UserAgent: "", httpClient: &http.Client{}}, nil
	}

}

func (c *Client) GetClusterNames(ctx context.Context) ([]string, error) {
	rel := &url.URL{Path: "/cluster"}
	u := c.BaseURL.ResolveReference(rel)
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	//req.Header.Set("User-Agent", c.UserAgent)

	clusterNames := []string{}
	_, err = c.do(ctx, req, &clusterNames)

	if err != nil {
		return nil, fmt.Errorf("failed to get cluster names: %w", err)
	}

	return clusterNames, nil
}

func (c *Client) GetCluster(ctx context.Context, name string) (*Cluster, error) {
	rel := &url.URL{Path: fmt.Sprintf("/cluster/%s", name)}
	u := c.BaseURL.ResolveReference(rel)
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	clusterState := &clusterStatus{}
	_, err = c.do(ctx, req, clusterState)

	if err != nil {
		return nil, fmt.Errorf("failed to get cluster (%s): %w", name, err)
	}

	cluster := newCluster(clusterState)

	// TODO check response status code

	return cluster, nil
}

func (c *Client) GetClusters(ctx context.Context) <-chan GetClusterResult {
	results := make(chan GetClusterResult)

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

func (c *Client) AddCluster(ctx context.Context, cluster string, seed string) error {
	rel := &url.URL{Path: fmt.Sprintf("/cluster/%s", cluster)}
	u := c.BaseURL.ResolveReference(rel)

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
		case <- ctx.Done():
			return ctx.Err()
		default:
		}
		return err
	}
	defer resp.Body.Close()

	// TODO check status code
	return nil
}

func (c *Client) DeleteCluster(ctx context.Context, cluster string) error {
	rel := &url.URL{Path: fmt.Sprintf("/cluster/%s", cluster)}
	u := c.BaseURL.ResolveReference(rel)
	req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
	if err != nil {
		return err
	}

	_, err = c.do(ctx, req, nil)

	// TODO check response status code

	if err != nil {
		return fmt.Errorf("failed to delete cluster (%s): %w", cluster, err)
	}

	return nil
}

func (c *Client) do(ctx context.Context, req *http.Request, v interface{}) (*http.Response, error) {
	req.Header.Set("Accept", "application/json")
	req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		select {
		case <- ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return nil, err
	}
	defer resp.Body.Close()

	if v != nil {
		err = json.NewDecoder(resp.Body).Decode(v)
	}

	return resp, err
}

func newCluster(state *clusterStatus) *Cluster {
	cluster := Cluster{
		Name: state.Name,
		JmxUsername: state.JmxUsername,
		JmxPasswordSet: state.JmxPasswordSet,
		Seeds: state.Seeds,
		NodeState: NodeState{},
	}

	for _, gs := range state.NodeStatus.EndpointStates {
		gossipState := GossipState{
			SourceNode: gs.SourceNode,
			EndpointNames: gs.EndpointNames,
			TotalLoad: gs.TotalLoad,
			DataCenters: map[string]DataCenterState{},
		}
		for dc, dcStateInternal := range gs.Endpoints {
			dcState := DataCenterState{Name: dc, Racks: map[string]RackState{}}
			for rack, endpoints := range dcStateInternal {
				rackState := RackState{Name: rack}
				for _, ep := range endpoints {
					endpoint := EndpointState{
						Endpoint: ep.Endpoint,
						DataCenter: ep.DataCenter,
						Rack: ep.Rack,
						HostId: ep.HostId,
						Status: ep.Status,
						Severity: ep.Severity,
						ReleaseVersion: ep.ReleaseVersion,
						Tokens: ep.Tokens,
						Load: ep.Load,
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