package reaper

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type RepairRun struct {
	Id                  uuid.UUID         `json:"id"`
	Cluster             string            `json:"cluster_name"`
	Owner               string            `json:"owner"`
	Keyspace            string            `json:"keyspace_name"`
	Tables              []string          `json:"column_families"`
	Cause               string            `json:"cause"`
	State               RepairRunState    `json:"state"`
	Intensity           Intensity         `json:"intensity"`
	IncrementalRepair   bool              `json:"incremental_repair"`
	SegmentCountPerNode int               `json:"total_segments"`
	RepairParallelism   RepairParallelism `json:"repair_parallelism"`
	SegmentsRepaired    int               `json:"segments_repaired"`
	LastEvent           string            `json:"last_event"`
	Duration            string            `json:"duration"`
	Nodes               []string          `json:"nodes"`
	Datacenters         []string          `json:"datacenters"`
	IgnoredTables       []string          `json:"blacklisted_tables"`
	RepairThreadCount   int               `json:"repair_thread_count"`
	RepairUnitId        uuid.UUID         `json:"repair_unit_id"`
}

func (r RepairRun) String() string {
	return fmt.Sprintf("Repair run %v on %v/%v (%v)", r.Id, r.Cluster, r.Keyspace, r.State)
}

type RepairSegment struct {
	Id           uuid.UUID
	RunId        uuid.UUID
	RepairUnitId uuid.UUID
	TokenRange   *Segment
	FailCount    int
	State        RepairSegmentState
	Coordinator  string
	Replicas     map[string]string
	StartTime    *time.Time
	EndTime      *time.Time
}

func (r RepairSegment) String() string {
	return fmt.Sprintf("Repair run segment %v (%v)", r.Id, r.State)
}

func (r *RepairSegment) UnmarshalJSON(data []byte) error {
	temp := struct {
		Id              uuid.UUID          `json:"id"`
		RunId           uuid.UUID          `json:"runId"`
		RepairUnitId    uuid.UUID          `json:"repairUnitId"`
		TokenRange      *Segment           `json:"tokenRange"`
		FailCount       int                `json:"failCount"`
		State           RepairSegmentState `json:"state"`
		Coordinator     string             `json:"coordinatorHost"`
		Replicas        map[string]string  `json:"replicas"`
		StartTimeMillis int64              `json:"startTime,omitempty"`
		EndTimeMillis   int64              `json:"endTime,omitempty"`
	}{}
	err := json.Unmarshal(data, &temp)
	if err != nil {
		return err
	}
	r.Id = temp.Id
	r.RunId = temp.RunId
	r.RepairUnitId = temp.RepairUnitId
	r.TokenRange = temp.TokenRange
	r.FailCount = temp.FailCount
	r.State = temp.State
	r.Coordinator = temp.Coordinator
	r.Replicas = temp.Replicas
	if temp.StartTimeMillis != 0 {
		unix := time.Unix(0, temp.StartTimeMillis*int64(time.Millisecond))
		r.StartTime = &unix
	}
	if temp.EndTimeMillis != 0 {
		unix := time.Unix(0, temp.EndTimeMillis*int64(time.Millisecond))
		r.EndTime = &unix
	}
	return nil
}

type Segment struct {
	BaseRange   *TokenRange       `json:"baseRange"`
	TokenRanges []*TokenRange     `json:"tokenRanges"`
	Replicas    map[string]string `json:"replicas"`
}

type TokenRange struct {
	Start *big.Int `json:"start"`
	End   *big.Int `json:"end"`
}

// Intensity controls the eagerness by which Reaper triggers repair segments. Must be in range (0.0, 1.0]. Reaper will
// use the duration of the previous repair segment to compute how much time to wait before triggering the next one. The
// idea behind this is that long segments mean a lot of data mismatch, and thus a lot of streaming and compaction.
// Intensity allows Reaper to adequately back off and give the cluster time to handle the load caused by the repair.
type Intensity = float64

type RepairRunState string

const (
	RepairRunStateNotStarted = RepairRunState("NOT_STARTED")
	RepairRunStateRunning    = RepairRunState("RUNNING")
	RepairRunStateError      = RepairRunState("ERROR")
	RepairRunStateDone       = RepairRunState("DONE")
	RepairRunStatePaused     = RepairRunState("PAUSED")
	RepairRunStateAborted    = RepairRunState("ABORTED")
	RepairRunStateDeleted    = RepairRunState("DELETED")
)

func (s RepairRunState) isActive() bool {
	return s == RepairRunStateRunning || s == RepairRunStatePaused
}

func (s RepairRunState) isTerminated() bool {
	return s == RepairRunStateDone || s == RepairRunStateError || s == RepairRunStateAborted || s == RepairRunStateDeleted
}

type RepairSegmentState string

const (
	RepairSegmentStateNotStarted = RepairSegmentState("NOT_STARTED")
	RepairSegmentStateRunning    = RepairSegmentState("RUNNING")
	RepairSegmentStateDone       = RepairSegmentState("DONE")
	RepairSegmentStateStarted    = RepairSegmentState("STARTED")
)

type RepairParallelism string

const (
	RepairParallelismSequential      = RepairParallelism("SEQUENTIAL")
	RepairParallelismParallel        = RepairParallelism("PARALLEL")
	RepairParallelismDatacenterAware = RepairParallelism("DATACENTER_AWARE")
)

type RepairRunSearchOptions struct {

	// Only return repair runs belonging to this cluster.
	Cluster string `url:"cluster_name,omitempty"`

	// Only return repair runs belonging to this keyspace.
	Keyspace string `url:"keyspace_name,omitempty"`

	// Restrict the search to repair runs whose states are in this list.
	States []RepairRunState `url:"state,comma,omitempty"`
}

type RepairRunCreateOptions struct {

	// Allows to specify which tables are targeted by a repair run. When this parameter is omitted, then the
	// repair run will target all the tables in its target keyspace.
	Tables []string `url:"tables,comma,omitempty"`

	// Allows to specify a list of tables that should not be repaired. Cannot be used in conjunction with Tables.
	IgnoredTables []string `url:"blacklistedTables,comma,omitempty"`

	// Identifies the process, or cause that caused the repair to run.
	Cause string `url:"cause,omitempty"`

	// Defines the amount of segments per node to create for the repair run. The value must be >0 and <=1000.
	SegmentCountPerNode int `url:"segmentCountPerNode,omitempty"`

	// Defines the used repair parallelism for repair run.
	RepairParallelism RepairParallelism `url:"repairParallelism,omitempty"`

	// Defines the used repair parallelism for repair run.
	Intensity Intensity `url:"intensity,omitempty"`

	// Defines if incremental repair should be done.
	IncrementalRepair bool `url:"incrementalRepair,omitempty"`

	// Allows to specify a list of nodes whose tokens should be repaired.
	Nodes []string `url:"nodes,comma,omitempty"`

	// Allows to specify a list of datacenters to repair.
	Datacenters []string `url:"datacenters,comma,omitempty"`

	// Defines the thread count to use for repair. Since Cassandra 2.2, repairs can be performed with
	// up to 4 threads in order to parallelize the work on different token ranges.
	RepairThreadCount int `url:"repairThreadCount,omitempty"`
}

func (c *client) GetRepairRuns(ctx context.Context, searchOptions *RepairRunSearchOptions) (map[uuid.UUID]*RepairRun, error) {
	res, err := c.doGet(ctx, "/repair_run", searchOptions, http.StatusOK)
	if err == nil {
		repairRuns := make([]*RepairRun, 0)
		err = c.readBodyAsJson(res, &repairRuns)
		if err == nil {
			repairRunsMap := make(map[uuid.UUID]*RepairRun, len(repairRuns))
			for _, repairRun := range repairRuns {
				repairRunsMap[repairRun.Id] = repairRun
			}
			return repairRunsMap, nil
		}
	}
	return nil, fmt.Errorf("failed to get repair runs: %w", err)
}

func (c *client) GetRepairRun(ctx context.Context, repairRunId uuid.UUID) (*RepairRun, error) {
	path := fmt.Sprint("/repair_run/", repairRunId)
	res, err := c.doGet(ctx, path, nil, http.StatusOK)
	if err == nil {
		repairRun := &RepairRun{}
		err = c.readBodyAsJson(res, repairRun)
		if err == nil {
			return repairRun, nil
		}
	}
	return nil, fmt.Errorf("failed to get repair run %v: %w", repairRunId, err)
}

func (c *client) CreateRepairRun(ctx context.Context, cluster string, keyspace string, owner string, options *RepairRunCreateOptions) (uuid.UUID, error) {
	queryParams, err := c.mergeParamSources(
		map[string]string{
			"clusterName": cluster,
			"keyspace":    keyspace,
			"owner":       owner,
		},
		options,
	)
	if err == nil {
		if options != nil && options.SegmentCountPerNode > 0 {
			// Some Reaper versions accept "segmentCount", others "segmentCountPerNode";
			// make sure we include both in the query string.
			queryParams.Set("segmentCount", strconv.Itoa(options.SegmentCountPerNode))
		}
		var res *http.Response
		res, err = c.doPost(ctx, "/repair_run", queryParams, nil, http.StatusCreated)
		if err == nil {
			repairRun := &RepairRun{}
			err = c.readBodyAsJson(res, repairRun)
			if err == nil {
				return repairRun.Id, nil
			}
		}
	}
	return uuid.Nil, fmt.Errorf("failed to create repair run: %w", err)
}

func (c *client) UpdateRepairRun(ctx context.Context, repairRunId uuid.UUID, newIntensity Intensity) error {
	path := fmt.Sprint("/repair_run/", repairRunId, "/intensity/", newIntensity)
	_, err := c.doPut(ctx, path, nil, nil, http.StatusOK)
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to update intensity of repair run %v: %w", repairRunId, err)
}

func (c *client) StartRepairRun(ctx context.Context, repairRunId uuid.UUID) error {
	path := fmt.Sprint("/repair_run/", repairRunId, "/state/", RepairRunStateRunning)
	_, err := c.doPut(ctx, path, nil, nil, http.StatusOK, http.StatusNoContent)
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to start repair run %v: %w", repairRunId, err)
}

func (c *client) PauseRepairRun(ctx context.Context, repairRunId uuid.UUID) error {
	path := fmt.Sprint("/repair_run/", repairRunId, "/state/", RepairRunStatePaused)
	_, err := c.doPut(ctx, path, nil, nil, http.StatusOK, http.StatusNoContent)
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to pause repair run %v: %w", repairRunId, err)
}

func (c *client) ResumeRepairRun(ctx context.Context, repairRunId uuid.UUID) error {
	path := fmt.Sprint("/repair_run/", repairRunId, "/state/", RepairRunStateRunning)
	_, err := c.doPut(ctx, path, nil, nil, http.StatusOK, http.StatusNoContent)
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to resume repair run %v: %w", repairRunId, err)
}

func (c *client) AbortRepairRun(ctx context.Context, repairRunId uuid.UUID) error {
	path := fmt.Sprint("/repair_run/", repairRunId, "/state/", RepairRunStateAborted)
	_, err := c.doPut(ctx, path, nil, nil, http.StatusOK, http.StatusNoContent)
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to abort repair run %v: %w", repairRunId, err)
}

func (c *client) GetRepairRunSegments(ctx context.Context, repairRunId uuid.UUID) (map[uuid.UUID]*RepairSegment, error) {
	path := fmt.Sprint("/repair_run/", repairRunId, "/segments")
	res, err := c.doGet(ctx, path, nil, http.StatusOK)
	if err == nil {
		repairRunSegments := make([]*RepairSegment, 0)
		err = c.readBodyAsJson(res, &repairRunSegments)
		if err == nil {
			repairRunSegmentsMap := make(map[uuid.UUID]*RepairSegment, len(repairRunSegments))
			for _, segment := range repairRunSegments {
				repairRunSegmentsMap[segment.Id] = segment
			}
			return repairRunSegmentsMap, nil
		}
	}
	return nil, fmt.Errorf("failed to get segments of repair run %v: %w", repairRunId, err)
}

func (c *client) AbortRepairRunSegment(ctx context.Context, repairRunId uuid.UUID, segmentId uuid.UUID) error {
	path := fmt.Sprint("/repair_run/", repairRunId, "/segments/abort/", segmentId)
	_, err := c.doPost(ctx, path, nil, nil, http.StatusOK)
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to abort segment %v of repair run %v: %w", segmentId, repairRunId, err)
}

func (c *client) DeleteRepairRun(ctx context.Context, repairRunId uuid.UUID, owner string) error {
	path := fmt.Sprint("/repair_run/", repairRunId)
	queryParams := &url.Values{"owner": {owner}}
	res, err := c.doDelete(ctx, path, queryParams, http.StatusAccepted)
	if err == nil {
		return nil
	} else {
		// FIXME this REST resource currently returns 500 for succeeded deletes
		if res != nil && res.StatusCode == http.StatusInternalServerError {
			_, err2 := c.doGet(ctx, path, nil, http.StatusNotFound)
			if err2 == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("failed to delete repair run %v: %w", repairRunId, err)
}

func (c *client) PurgeRepairRuns(ctx context.Context) (int, error) {
	res, err := c.doPost(ctx, "/repair_run/purge", nil, nil, http.StatusOK)
	if err == nil {
		var purgedStr string
		purgedStr, err = c.readBodyAsString(res)
		if err == nil {
			var purged int
			purged, err = strconv.Atoi(purgedStr)
			if err == nil {
				return purged, nil
			}
		}
	}
	return 0, fmt.Errorf("failed to purge repair runs: %w", err)
}
