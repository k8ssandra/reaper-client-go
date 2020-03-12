package reaper

import (
	"encoding/json"
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
	jmxUsername     string `json:"jmx_username,omitempty"`
	jmxPasswordSet  bool `json:"jmx_password_is_set,omitempty"`
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
