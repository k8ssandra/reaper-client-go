# Reaper Go Client - API Enhancement Proposal


# State of the Art

The full REST API is documented here: [http://cassandra-reaper.io/docs/api](http://cassandra-reaper.io/docs/api). 

However this page is not entirely up to date. A more exhaustive list is as follows; implemented methods are shown in bold (even if only partially implemented):



*   Ping
    *   <b>`GET /ping`</b>
    *   `HEAD /ping`
*   Authentication
    *   `POST /login`
    *   `POST /logout`
    *   `GET /jwt`
*   Cluster
    *   <b>`GET /cluster`</b>
    *   <b>`POST /cluster`</b>
    *   `POST /cluster/auth`
    *   <b>`DELETE /cluster/{cluster_name}`</b>
    *   <b>`GET /cluster/{cluster_name}`</b>
    *   <b>`PUT /cluster/{cluster_name}`</b>
    *   `PUT /cluster/{cluster_name}/auth`
    *   `GET /cluster/{cluster_name}/tables`
*   Repair Runs
    *   `GET /repair_run`
    *   `POST /repair_run`
    *   `GET /repair_run/cluster/{cluster_name}`
    *   `POST /repair_run/purge`
    *   `DELETE /repair_run/{id}`
    *   `GET /repair_run/{id}`
    *   `PUT /repair_run/{id}`
    *   `PUT /repair_run/{id}/intensity/{intensity}`
    *   `GET /repair_run/{id}/segments`
    *   `POST /repair_run/{id}/segments/abort/{segment_id}`
    *   `PUT /repair_run/{id}/state/{state}`
*   Repair Schedules
    *   <b>`GET /repair_schedule`</b>
    *   `POST /repair_schedule`
    *   <b>`GET /repair_schedule/cluster/{cluster_name}`</b>
    *   `POST /repair_schedule/start/{id}`
    *   `DELETE /repair_schedule/{id}`
    *   `GET /repair_schedule/{id}`
    *   `PUT /repair_schedule/{id}`
    *   `GET /repair_schedule/{clusterName}/{id}/percent_repaired`
*   Snapshot
    *   `GET /snapshot/cluster/{clusterName}`
    *   `POST /snapshot/cluster/{clusterName}`
    *   `DELETE /snapshot/cluster/{clusterName}/{snapshotName}`
    *   `GET /snapshot/{clusterName}/{host}`
    *   `POST /snapshot/{clusterName}/{host}`
    *   `DELETE /snapshot/{clusterName}/{host}/{snapshotName}`
*   Node
    *   `GET /node/clientRequestLatencies/{clusterName}/{host}`
    *   `GET /node/compactions/{clusterName}/{host}`
    *   `GET /node/dropped/{clusterName}/{host}`
    *   `GET /node/streams/{clusterName}/{host}`
    *   `GET /node/tokens/{clusterName}/{host}`
    *   `GET /node/tpstats/{clusterName}/{host}`
*   Diagnostic Events
    *   `GET /diag_event/sse_listen/{id}`
    *   `GET /diag_event/subscription`
    *   `POST /diag_event/subscription`
    *   `DELETE /diag_event/subscription/{id}`
    *   `GET /diag_event/subscription/{id}`

# Common principles for the revised API

1. Language conventions
    1. The client interface should be named `reaper.Client` instead of `reaper.ReaperClient` to avoid repetition of the word "Reaper"
    2. Use pointers to structs in API methods (no passing struct parameters by value)
    3. Use Google's `uuid.UUID` for fields of uuid type
    4. Use `big.Int` for Java `BigInteger`
    5. Use `time.Time` for JodaTime `DateTime` structs
        1. Deserialization will require a custom JSON unmarshaller because `DateTime` fields are sent as millis since the Epoch
        
    6. Use `github.com/google/go-querystring` to simplify creation of query strings from structs for optional REST parameters.
    7. Use string-based types to capture Java enums, e.g. 
	    
			   type RepairSegmentState string
			   const (
					RepairSegmentStateNotStarted = RepairSegmentState("NOT_STARTED")
					RepairSegmentStateRunning    = RepairSegmentState("RUNNING")
					RepairSegmentStateDone       = RepairSegmentState("DONE")
					RepairSegmentStateStarted    = RepairSegmentState("STARTED")
				)
        
    8. Create type `Intensity` as an alias to float64 to better document its properties
2. REST endpoint mappings
    1. All API methods should take a first `context.Context `parameter, e.g. 

				Ping(ctx context.Context) error
        
    2. Mandatory parameters should be captured in corresponding API method parameters, e.g. 

				Login(ctx context.Context, username string, password string) error
        
    3. Optional parameters should be captured in dedicated structs, and the struct should be included as a last, nilable parameter, e.g. 

				GetClusterNames(ctx context.Context, searchOptions *ClusterSearchOptions) ([]string, error)
        
    4. API methods should start with a standard prefix except GET methods. This should be followed by the REST resource 
       name (`Cluster`, `RepairRun`, etc.):
        1. GET: no prefix; methods should be named after the entity's plural when returning lists of entities, or its 
           singular form  when returning a single entity. E.g. `Cluster`, `RepairRuns`.
        2. POST: prefix should be `Create`, or also `Start`, `Abort`, `Purge` if appropriate (state transitions) + 
           entity name. E.g.`CreateRepairRun`, `AbortRepairRunSegment`.
        3. PUT: prefix should be `Update`, or also `Start`, `Pause`, `Resume` if appropriate (state transitions) + 
           entity name. E.g. `UpdateRepairRun`, `ResumeRepairRun`.
        4. DELETE: prefix should be `Delete` + entity name, e.g.: `DeleteCluster`.
3. Naming conventions:
    1. Use the word "table" instead of "column family"
    2. Prefer variables named `cluster` or `keyspace` instead of `clusterName`, `keyspaceName`
    3. Use the word `Datacenter` instead of `DataCenter` in variable names
    4. Use the word "node" instead of "host"
    5. Use "ignore" instead of "blacklist" 
    6. Use the suffix "state" for enums (`RUNNING`, `DONE`, etc.), e.g. `RepairRunState`
    7. Don't append "status" to struct names (e.g. `RepairRun` instead of `RepairRunStatus`)


# Proposed API Methods


## Login and Authentication Resources


### Methods


<table>
  <tr>
   <td><strong>REST endpoint</strong>
   </td>
   <td><strong>Client API method</strong>
   </td>
   <td><strong>Comments</strong>
   </td>
  </tr>
  <tr>
   <td><code>GET /ping</code>
   </td>
   <td><code>N/A</code>
   </td>
   <td>Already present in the current API. I suggest switching to the HEAD version below.
   </td>
  </tr>
  <tr>
   <td><code>HEAD /ping</code>
   </td>
   <td><code>Ping(ctx context.Context) error</code>
   </td>
   <td>Already present in the current API (named <code>IsReaperUp</code>). I suggest returning only <code>error</code> instead of <code>(bool, error)</code>.
   </td>
  </tr>
  <tr>
   <td><code>POST /login</code>
   </td>
   <td><code>Login(ctx context.Context, username string, password string) error</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>GET /jwt</code>
   </td>
   <td><code>N/A</code>
   </td>
   <td>WIll be used underneath by Login to generate the JWT token.
   </td>
  </tr>
  <tr>
   <td><code>POST /logout</code>
   </td>
   <td><code>Logout(ctx context.Context) error</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td>
   </td>
   <td><code>IsAuthenticated() bool</code>
   </td>
   <td>Non-endpoint method. returns true if the client has been authenticated through a successful call to Login.
<p>
Authenticated clients will send the authentication token along with every request to the Reaper backend.
   </td>
  </tr>
</table>



## Cluster Resource


### Methods

Two existing methods are actually a combination of REST method calls in a loop:

```
// GetClusters fetches all clusters. This function is async and may return before any or all results are
// available. The concurrency is currently determined by min(5, NUM_CPUS).
GetClusters(ctx context.Context) <-chan GetClusterResult

// GetClustersSync fetches all clusters in a synchronous or blocking manner. Note that this function fails
// fast if there is an error and no clusters will be returned.
GetClustersSync(ctx context.Context) ([]*Cluster, error)
```

I am not sure that these should be preserved. Applications can easily reproduce their behavior themselves if necessary using the proposed API below.


<table>
  <tr>
   <td><strong>REST endpoint</strong>
   </td>
   <td><strong>Client API method</strong>
   </td>
   <td><strong>Comments</strong>
   </td>
  </tr>
  <tr>
   <td><code>GET /cluster</code>
   </td>
   <td><code>ClusterNames(ctx context.Context, seedHost string) ([]string, error)</code>
   </td>
   <td>Already present in the current API. Suffix <code>Names</code> because it's only a list of names, not entities.
   </td>
  </tr>
  <tr>
   <td><code>GET /cluster/{cluster_name}</code>
   </td>
   <td><code>Cluster(ctx context.Context, name string, renderOptions *ClusterRenderOptions) (*Cluster, error)</code>
   </td>
   <td>Already present in the current API. 
   </td>
  </tr>
  <tr>
   <td><code>GET /cluster/{cluster_name}/tables</code>
   </td>
   <td><code>ClusterSchema(ctx context.Context, name string) (map[string]*Keyspace, error)</code>
   </td>
   <td><code>Schema</code> rather than <code>Tables</code> since it is a map of tables keyed by keyspace.
   </td>
  </tr>
  <tr>
   <td><code>POST /cluster</code>
<p>
<code>POST /cluster/auth</code>
   </td>
   <td><code>CreateCluster(ctx context.Context, seed string, jmxOptions *ClusterJmxOptions) (string, error)</code>
   </td>
   <td>Already present in the current API. Will use the special<code> /auth</code> endpoint if authenticated, otherwise the regular one.
   </td>
  </tr>
  <tr>
   <td><code>PUT /cluster/{cluster_name}</code>
<p>
<code>PUT /cluster/{cluster_name}/auth</code>
   </td>
   <td><code>UpdateCluster(ctx context.Context, name string, newSeed string, jmxOptions *ClusterJmxOptions) error</code>
   </td>
   <td>Already present in the current API. Will use the special<code> /auth</code> endpoint if authenticated, otherwise the regular one.
   </td>
  </tr>
  <tr>
   <td><code>DELETE /cluster/{cluster_name}</code>
   </td>
   <td><code>DeleteCluster(ctx context.Context, name string, force bool) error</code>
   </td>
   <td>Already present in the current API. 
   </td>
  </tr>
</table>


### Types

Note: the `NodeState` struct attempts to capture datacenter and rack names in a user-friendly manner, but does not correspond 1:1 with the JSON payload and will require a custom JSON unmarshaller. The current API already does this translation.

```
type Cluster struct {
  Name            string
  JmxUsername     string
  JmxPasswordSet  bool
  Seeds           []string
  RepairRuns      []*RepairRun
  RepairSchedules []*RepairSchedule
  NodeStates      []*NodeState
}

type Keyspace struct {
  Name   string
  Tables map[string]*Table
}

type Table struct {
  Name string
}

type NodeState struct {
  SourceNode  string
  TotalLoad   float64
  Endpoints   []string
  Datacenters map[string]*Datacenter
}

type Datacenter struct {
  Name  string
  Racks map[string]*Rack
}

type Rack struct {
  Name      string
  Endpoints []*Endpoint
}

type Endpoint struct {
  Endpoint       string  
  Datacenter     string  
  Rack           string  
  HostId         string  
  Status         string  
  Severity       float64 
  ReleaseVersion string  
  Tokens         string  
  Load           float64 
}
```



## RepairRun Resource


### Methods


<table>
  <tr>
   <td><strong>REST endpoint</strong>
   </td>
   <td><strong>Client API method</strong>
   </td>
   <td><strong>Comments</strong>
   </td>
  </tr>
  <tr>
   <td><code>GET /repair_run</code>
   </td>
   <td><code>RepairRuns(ctx context.Context, searchOptions *RepairRunSearchOptions) ([]*RepairRun, error)</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>GET /repair_run/cluster/{cluster_name}</code>
   </td>
   <td><code>N/A</code>
   </td>
   <td>Not mapped, can be achieved with the method above + search options
   </td>
  </tr>
  <tr>
   <td><code>GET /repair_run/{id}</code>
   </td>
   <td><code>RepairRun(ctx context.Context, repairRunId uuid.UUID) (*RepairRun, error)</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>POST /repair_run</code>
   </td>
   <td><code>CreateRepairRun(ctx context.Context, cluster string, keyspace string, owner string, options *RepairRunCreateOptions) (uuid.UUID, error)</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>PUT /repair_run/{id}</code>
   </td>
   <td><code>N/A</code>
   </td>
   <td>Deprecated REST endpoint, moved to \
<code>PUT repair_run/{id}/state/{state}</code>
   </td>
  </tr>
  <tr>
   <td><code>PUT /repair_run/{id}/intensity/{intensity}</code>
   </td>
   <td><code>UpdateRepairRun(ctx context.Context, repairRunId uuid.UUID, newIntensity Intensity) error</code>
   </td>
   <td>Only intensity can be changed.
   </td>
  </tr>
  <tr>
   <td><code>PUT /repair_run/{id}/state/{state}</code>
   </td>
   <td><code>StartRepairRun(ctx context.Context, repairRunId uuid.UUID) error</code>
<p>
<code>ResumeRepairRun(ctx context.Context, repairRunId uuid.UUID) error</code>
   </td>
   <td>Two API methods to better distinguish start from resume. Endpoint will be called with<code> state = RUNNING</code>
   </td>
  </tr>
  <tr>
   <td><code>PUT /repair_run/{id}/state/{state}</code>
   </td>
   <td><code>PauseRepairRun(ctx context.Context, repairRunId uuid.UUID) error</code>
   </td>
   <td>Endpoint will be called with <code>state = PAUSED</code>
   </td>
  </tr>
  <tr>
   <td><code>GET /repair_run/{id}/segments</code>
   </td>
   <td><code>RepairRunSegments(ctx context.Context, repairRunId uuid.UUID) ([]*RepairSegment, error)</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>PUT /repair_run/{id}/state/{state}</code>
   </td>
   <td><code>AbortRepairRun(ctx context.Context, repairRunId uuid.UUID) error</code>
   </td>
   <td>Endpoint will be called with <code>state = ABORTED</code>
   </td>
  </tr>
  <tr>
   <td><code>GET /repair_run/{id}/segments</code>
   </td>
   <td><code>RepairRunSegments(ctx context.Context, repairRunId uuid.UUID) ([]*RepairSegment, error)</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>POST /repair_run/{id}/segments/abort/{segment_id}</code>
   </td>
   <td><code>AbortRepairRunSegment(ctx context.Context, repairRunId uuid.UUID, segmentId uuid.UUID) error</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>DELETE /repair_run/{id}</code>
   </td>
   <td><code>DeleteRepairRun(ctx context.Context, repairRunId uuid.UUID, owner string) error</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>POST /repair_run/purge</code>
   </td>
   <td><code>PurgeRepairRuns(ctx context.Context) (int, error)</code>
   </td>
   <td>
   </td>
  </tr>
</table>

### Types

```
type RepairRun struct {
  Id                uuid.UUID         
  Cluster           string            
  Owner             string            
  Keyspace          string            
  Tables            []string          
  Cause             string            
  State             RepairRunState    
  Intensity         Intensity         
  IncrementalRepair bool              
  TotalSegments     int               
  RepairParallelism RepairParallelism 
  SegmentsRepaired  int               
  LastEvent         string            
  Duration          string            
  Nodes             []string          
  Datacenters       []string          
  IgnoredTables     []string          
  RepairThreadCount int               
  RepairUnitId      uuid.UUID         
}

type RepairSegment struct {
  Id           uuid.UUID          
  RunId        uuid.UUID          
  RepairUnitId uuid.UUID          
  TokenRange   *Segment           
  FailCount    int                
  State        RepairSegmentState 
  Coordinator  string             
  StartTime    *time.Time    
  EndTime      *time.Time    
  Replicas     map[string]string  
}

type Segment struct {
  BaseRange   *TokenRange       
  TokenRanges []*TokenRange     
  Replicas    map[string]string 
}

type TokenRange struct {
  Start *big.Int `json:"start"`
  End   *big.Int `json:"end"`
}

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

  // Restrict the search to repair runs whose states are in this list
  States []RepairRunState `url:"state,comma,omitempty"`
}

type RepairRunCreateOptions struct {

  // Allows to specify which tables are targeted by a repair run. When this parameter is omitted, then the
  // repair run will target all the tables in its target keyspace.
  Tables []string `url:"tables,comma,omitempty"`

  // Allows to specify a list of tables that should not be repaired. Cannot be used in conjunction with Tables.
  IgnoredTables []string `url:"blacklistedTables,comma,omitempty"`

  // Identifies the process, or cause that cause the repair to run.
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
```

## RepairSchedule Resource


### Methods


<table>
  <tr>
   <td><strong>REST endpoint</strong>
   </td>
   <td><strong>Client API method</strong>
   </td>
   <td><strong>Comments</strong>
   </td>
  </tr>
  <tr>
   <td><code>GET /repair_schedule</code>
   </td>
   <td><code>RepairSchedules(ctx context.Context, searchOptions *RepairScheduleSearchOptions) ([]*RepairSchedule, error)</code>
   </td>
   <td>Already present in the current PR #3. 
   </td>
  </tr>
  <tr>
   <td><code>GET /repair_schedule/{id}</code>
   </td>
   <td><code>RepairSchedule(ctx context.Context, repairScheduleId uuid.UUID) (*RepairSchedule, error)</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>POST /repair_schedule</code>
   </td>
   <td><code>CreateRepairSchedule(ctx context.Context, cluster string, keyspace string, owner string, scheduleDaysBetween int, options *RepairScheduleCreateOptions) (uuid.UUID, error)</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>POST /repair_schedule/start/{id}</code>
   </td>
   <td><code>StartRepairSchedule(ctx context.Context, repairScheduleId uuid.UUID) error</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>PUT /repair_schedule/{id}</code>
   </td>
   <td><code>PauseRepairSchedule(ctx context.Context, repairScheduleId uuid.UUID) error</code>
   </td>
   <td>Endpoint will be called with <code>state = PAUSED</code>
   </td>
  </tr>
  <tr>
   <td><code>PUT /repair_schedule/{id}</code>
   </td>
   <td><code>ResumeRepairSchedule(ctx context.Context, repairScheduleId uuid.UUID) error</code>
   </td>
   <td>Endpoint will be called with<code> state = ACTIVE</code>. Contrary to <code>RepairRun</code>, resuming a <code>RepairSchedule</code> is not the same as starting one.
   </td>
  </tr>
  <tr>
   <td><code>GET /repair_schedule/{clusterName}/{id}/percent_repaired</code>
   </td>
   <td><code>RepairSchedulePercentRepaired(ctx context.Context, cluster string, repairScheduleId uuid.UUID) ([]*PercentRepairedMetric, error)</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>DELETE /repair_schedule/{id}</code>
   </td>
   <td><code>DeleteRepairSchedule(ctx context.Context, repairScheduleId uuid.UUID, owner string) error</code>
   </td>
   <td>
   </td>
  </tr>
</table>

### Types

```
type RepairSchedule struct {
  Id                  uuid.UUID           
  Owner               string              
  Cluster             string              
  Keyspace            string              
  Tables              []string            
  State               RepairScheduleState 
  Intensity           Intensity           
  IncrementalRepair   bool                
  RepairParallelism   RepairParallelism   
  DaysBetween         int                 
  Nodes               []string            
  Datacenters         []string            
  IgnoredTables       []string            
  SegmentCountPerNode int                 
  RepairThreadCount   int                 
  RepairUnitId        uuid.UUID           
}

type RepairScheduleSearchOptions struct {

  // Only return repair schedules belonging to this cluster.
  Cluster string `url:"clusterName,omitempty"`

  // Only return repair schedules belonging to this keyspace.
  Keyspace string `url:"keyspace,omitempty"`
}

type RepairScheduleCreateOptions struct {

  // Allows to specify which tables are targeted by a repair run. When this parameter is omitted, then the
  // repair run will target all the tables in its target keyspace.
  Tables []string `url:"tables,comma,omitempty"`

  // Allows to specify a list of tables that should not be repaired. Cannot be used in conjunction with Tables.
  IgnoredTables []string `url:"blacklistedTables,comma,omitempty"`

  // Defines the amount of segments per node to create for the repair run. The value must be >0 and <=1000.
  SegmentCountPerNode int `url:"segmentCountPerNode,omitempty"`

  // Defines the used repair parallelism for repair run.
  RepairParallelism RepairParallelism `url:"repairParallelism,omitempty"`

  // Defines the used repair parallelism for repair run.
  Intensity Intensity `url:"intensity,omitempty"`

  // Defines if incremental repair should be done.
  IncrementalRepair bool `url:"incrementalRepair,omitempty"`

  // When to trigger the next repair run. If not specified, defaults to the next day, at start of day.
  TriggerTime *time.Time `url:"scheduleTriggerTime,omitempty"`

  // Allows to specify a list of nodes whose tokens should be repaired.
  Nodes []string `url:"nodes,comma,omitempty"`

  // Allows to specify a list of datacenters to repair.
  Datacenters []string `url:"datacenters,comma,omitempty"`

  // Defines the thread count to use for repair. Since Cassandra 2.2, repairs can be performed with
  // up to 4 threads in order to parallelize the work on different token ranges.
  RepairThreadCount int `url:"repairThreadCount,omitempty"`
}

type PercentRepairedMetric struct {
  Cluster          string    
  Node             string    
  RepairScheduleId uuid.UUID 
  Keyspace         string    
  Table            string    
  PercentRepaired  int       
}

type RepairScheduleState string

const (
  RepairScheduleStateActive  = RepairScheduleState("ACTIVE")
  RepairScheduleStatePaused  = RepairScheduleState("PAUSED")
  RepairScheduleStateDeleted = RepairScheduleState("DELETED")
)
```

## Snapshot Resource


### Methods

Methods in this resource come in pairs: one for a specific node, one for a cluster-wide equivalent. They are distinguished by "[VERB]NodeSnapshot"vs "[VERB]ClusterSnapshot".


<table>
  <tr>
   <td><strong>REST endpoint</strong>
   </td>
   <td><strong>Client API method</strong>
   </td>
   <td><strong>Comments</strong>
   </td>
  </tr>
  <tr>
   <td><code>GET /snapshot/{clusterName}/{host}</code>
   </td>
   <td><code>NodeSnapshots(ctx context.Context, cluster string, node string) ([]*Snapshot, error)</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>GET /snapshot/cluster/{clusterName}</code>
   </td>
   <td><code>ClusterSnapshots(ctx context.Context, cluster string) ([]*Snapshot, error)</code>
   </td>
   <td> 
   </td>
  </tr>
  <tr>
   <td><code>POST /snapshot/{clusterName}/{node}</code>
   </td>
   <td><code>CreateNodeSnapshot(ctx context.Context, cluster string, node string, options *NodeSnapshotCreateOptions) (string, error)</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>POST /snapshot/cluster/{clusterName</code>
   </td>
   <td><code>CreateClusterSnapshot(ctx context.Context, cluster string, options *ClusterSnapshotCreateOptions) (string, error)</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>DELETE /snapshot/{clusterName}/{host}/{snapshotName}</code>
   </td>
   <td><code>DeleteNodeSnapshot(ctx context.Context, cluster string, node string, snapshot string) error</code>
   </td>
   <td>
   </td>
  </tr>
  <tr>
   <td><code>DELETE /snapshot/cluster/{clusterName}/{snapshotName}</code>
   </td>
   <td><code>DeleteClusterSnapshot(ctx context.Context, cluster string, snapshot string) error</code>
   </td>
   <td>
   </td>
  </tr>
</table>



### Types


```
type Snapshot struct {
  Name         string          
  Node         string          
  Cluster      string          
  Keyspace     string          
  Table        string          
  TrueSize     float64         
  SizeOnDisk   float64         
  Owner        string          
  Cause        string          
  CreationTime *time.Time
}

type NodeSnapshotCreateOptions struct {
  // The name of the snapshot. If omitted, a default name will be generated.
  Name string `url:"name,omitempty"`

  // The keyspace to create a snapshot for. If omitted, all keyspaces will be snapshot.
  Keyspace string `url:"keyspace,omitempty"`
}

type ClusterSnapshotCreateOptions struct {
  NodeSnapshotCreateOptions

  // The owner of the snapshot. If omitted, the owner will be "reaper".
  Owner string `url:"owner,omitempty"`

  // The cause of the snapshot. If omitted, the cause will be "Snapshot taken with Reaper".
  Cause string `url:"cause,omitempty"`
}
```

## Node Resource

TODO


## Diagnostic Events Resource

TODO
