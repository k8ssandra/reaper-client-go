package reaper

import (
	"context"
	"net/http"
	"net/url"
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
