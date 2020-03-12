package reaper

import (
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

type Cluster struct {
	Name            string `json:"name"`
	JmxUsername     string `json:"jmx_username,omitempty"`
	JmxPasswordSet  bool `json:"jmx_password_is_set,omitempty"`
	Seeds           []string `json:"seed_hosts,omitempty"`
}

func NewClient(reaperBaseURL string) (*Client, error) {
	if baseURL, err := url.Parse(reaperBaseURL); err != nil {
		return nil, err
	} else {
		return &Client{BaseURL: baseURL, UserAgent: "", httpClient: &http.Client{}}, nil
	}

}

func (c *Client) GetClusterNames() ([]string, error) {
	rel := &url.URL{Path: "/cluster"}
	u := c.BaseURL.ResolveReference(rel)
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	//req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	clusterNames := []string{}
	err = json.NewDecoder(resp.Body).Decode(&clusterNames)

	return clusterNames, err
}

func (c *Client) GetCluster(name string) (*Cluster, error) {
	rel := &url.URL{Path: fmt.Sprintf("/cluster/%s", name)}
	u := c.BaseURL.ResolveReference(rel)
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	cluster := &Cluster{}
	err = json.NewDecoder(resp.Body).Decode(cluster)

	return cluster, err
}

//func (c *Client) GetClusters() ([]Cluster, error) {
//	clusters := []Cluster{}
//	clusterNames, err := c.GetClusterNames()
//	if err != nil {
//		return clusters, err
//	}
//
//	for name := range clusterNames {
//		cluster
//	}
//}
