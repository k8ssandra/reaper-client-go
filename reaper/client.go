package reaper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Client struct {
	BaseURL    *url.URL
	UserAgent  string
	httpClient *http.Client
}

type clusterState struct {
	Name           string             `json:"name"`
	JmxUsername    string             `json:"jmx_username,omitempty"`
	JmxPasswordSet bool               `json:"jmx_password_is_set,omitempty"`
	Seeds          []string           `json:"seed_hosts,omitempty"`
	NodeStatus     nodeStatusInternal `json:"nodes_status"`
}

type nodeStatusInternal struct {
	EndpointStates []gossipStateInternal `json:"endpointStates,omitempty"`
}

type gossipStateInternal struct {
	SourceNode    string `json:"sourceNode"`
	EndpointNames []string `json:"endpointNames,omitempty"`
	TotalLoad     float64 `json:"totalLoad,omitempty"`
	Endpoints     map[string]map[string][]EndpointState
}

type Cluster struct {
	Name            string `json:"name"`
	JmxUsername     string `json:"jmx_username,omitempty"`
	JmxPasswordSet  bool `json:"jmx_password_is_set,omitempty"`
	Seeds           []string `json:"seed_hosts,omitempty"`
	NodeStatus     	NodeStatus
}

type NodeStatus struct {
	GossipStates []GossipState
}

type GossipState struct {
	SourceNode    string `json:"sourceNode"`
	EndpointNames []string `json:"endpointNames,omitempty"`
	TotalLoad     float64 `json:"totalLoad,omitempty"`
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
	Endpoint       string `json:"endpoint"`
	DataCenter     string `json:"dc"`
	Rack           string `json:"rack"`
	HostId         string `json:"hostId"`
	Status         string `json:"status"`
	Severity       float64 `json:"severity"`
	ReleaseVersion string `json:"releaseVersion"`
	Tokens         string `json:"tokens"`
	Load           float64 `json:"load"`
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
	req.Header.Set("Accept", "application/json")
	req.WithContext(ctx)
	//req.Header.Set("User-Agent", c.UserAgent)

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

	clusterNames := []string{}
	err = json.NewDecoder(resp.Body).Decode(&clusterNames)

	return clusterNames, err
}

func (c *Client) GetCluster(ctx context.Context, name string) (*Cluster, error) {
	rel := &url.URL{Path: fmt.Sprintf("/cluster/%s", name)}
	u := c.BaseURL.ResolveReference(rel)
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	clusterState := &clusterState{}
	_, err = c.do(ctx, req, clusterState)

	if err != nil {
		return nil, fmt.Errorf("failed to get cluster (%s): %w", name, err)
	}

	cluster := newCluster(clusterState)

	return cluster, nil
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

	// TODO check status code

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

func newCluster(state *clusterState) *Cluster {
	cluster := Cluster{
		Name: state.Name,
		JmxUsername: state.JmxUsername,
		JmxPasswordSet: state.JmxPasswordSet,
		Seeds: state.Seeds,
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
				for _, endpoint := range endpoints {
					rackState.Endpoints = append(rackState.Endpoints, endpoint)
				}
				dcState.Racks[rack] = rackState
			}
			gossipState.DataCenters[dc] = dcState
		}
		cluster.NodeStatus.GossipStates = append(cluster.NodeStatus.GossipStates, gossipState)
	}

	return &cluster
}