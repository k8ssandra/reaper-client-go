package reaper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
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

	// RepairRuns returns a list of repair runs, optionally filtering according to the provided search options.
	RepairRuns(ctx context.Context, searchOptions *RepairRunSearchOptions) (map[uuid.UUID]*RepairRun, error)

	// RepairRun returns a repair run object identified by its id.
	RepairRun(ctx context.Context, repairRunId uuid.UUID) (*RepairRun, error)

	// CreateRepairRun creates a new repair run for the given cluster and keyspace. Does not actually trigger the run:
	// creating a repair run includes generating the repair segments. Returns the id of the newly-created repair run if
	// successful. The owner name can be any string identifying the owner.
	CreateRepairRun(
		ctx context.Context,
		cluster string,
		keyspace string,
		owner string,
		options *RepairRunCreateOptions,
	) (uuid.UUID, error)

	// UpdateRepairRun modifies the intensity of a PAUSED repair run identified by its id.
	UpdateRepairRun(ctx context.Context, repairRunId uuid.UUID, newIntensity Intensity) error

	// StartRepairRun starts (or resumes) a repair run identified by its id. Can also be used to reattempt a repair run
	// in state “ERROR”, picking up where it left off.
	StartRepairRun(ctx context.Context, repairRunId uuid.UUID) error

	// PauseRepairRun pauses a repair run identified by its id. The repair run must be in RUNNING state.
	PauseRepairRun(ctx context.Context, repairRunId uuid.UUID) error

	// ResumeRepairRun is an alias to StartRepairRun.
	ResumeRepairRun(ctx context.Context, repairRunId uuid.UUID) error

	// AbortRepairRun aborts a repair run identified bu its id. The repair run must not be in ERROR state.
	AbortRepairRun(ctx context.Context, repairRunId uuid.UUID) error

	// RepairRunSegments returns the list of segments of a repair run identified by its id.
	RepairRunSegments(ctx context.Context, repairRunId uuid.UUID) (map[uuid.UUID]*RepairSegment, error)

	// AbortRepairRunSegment aborts a running segment and puts it back in NOT_STARTED state. The segment will be
	// processed again later during the lifetime of the repair run.
	AbortRepairRunSegment(ctx context.Context, repairRunId uuid.UUID, segmentId uuid.UUID) error

	// DeleteRepairRun deletes a repair run object identified by its id. Repair run and all the related repair segments
	// will be deleted from the database. If the given owner does not match the stored owner, the delete request will
	// fail.
	DeleteRepairRun(ctx context.Context, repairRunId uuid.UUID, owner string) error

	// PurgeRepairRuns purges repairs and returns the number of repair runs purged.
	PurgeRepairRuns(ctx context.Context) (int, error)

	RepairSchedules(ctx context.Context) ([]RepairSchedule, error)

	RepairSchedulesForCluster(ctx context.Context, clusterName string) ([]RepairSchedule, error)

	Login(ctx context.Context, username string, password string) error
}

type client struct {
	baseURL    *url.URL
	userAgent  string
	httpClient *http.Client
	jSessionId *string
	jwt        *string
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

func (c *client) Login(ctx context.Context, username, password string) error {
	formData := map[string]string{}
	formData["username"] = username
	formData["password"] = password
	formData["rememberMe"] = "false"
	if resp, err := c.doPost(ctx, "/login", nil, formData); err == nil {
		cookies := resp.Cookies()
		for _, cookie := range cookies {
			if cookie.Name == "JSESSIONID" {
				c.jSessionId = &cookie.Value
				return c.getJwt(ctx)
			}
		}
		respBody, _ := c.readBodyAsString(resp)
		return fmt.Errorf("unable to log in. no JSESSIONID cookie. resp: %s - cookies: %v", respBody, cookies)
	} else {
		return err
	}
}

func (c *client) getJwt(ctx context.Context) error {
	if resp, err := c.doGet(ctx, "/jwt", nil); err == nil {
		if jwt, err := c.readBodyAsString(resp); err == nil {
			c.jwt = &jwt
			c.jSessionId = nil
			return nil
		} else {
			return err
		}
	} else {
		return err
	}
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
