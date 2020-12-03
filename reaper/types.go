package reaper

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
	Id string `json:"id"`
	Owner string `json:"owner,omitempty"`
	State string `json:"state,omitempty"`
	Intensity float64 `json:"intensity,omitempty"`
	ClusterName string `json:"cluster_name,omitempty"`
	KeyspaceName string `json:"keyspace_name,omitempty"`
	RepairParallism string `json:"repair_parallelism,omitempty"`
	IncrementalRepair bool `json:"incremental_repair,omitempty"`
	ThreadCount int `json:"repair_thread_count,omitempty"`
	UnitId string `json:"repair_unit_id,omitempty"`

	//public enum State {
	//ACTIVE,
	//PAUSED,
	//DELETED
	//}

	//public enum RepairParallelism {
	//SEQUENTIAL("sequential"),
	//PARALLEL("parallel"),
	//DATACENTER_AWARE("dc_parallel");

/*
[
{
	"id": "9ee7f6e0-3575-11eb-8fca-273b55edb18f",
	"owner": "auto-scheduling",
	"state": "ACTIVE",
	"intensity": 0.8999999761581421,
	"cluster_name": "k8ssandra",
	"keyspace_name": "reaper_db",
	"column_families": [],
	"incremental_repair": false,
	"segment_count": 0,
	"repair_parallelism": "DATACENTER_AWARE",
	"scheduled_days_between": 7,
	"nodes": [],
	"datacenters": [],
	"blacklisted_tables": [],
	"segment_count_per_node": 64,
	"repair_thread_count": 1,
	"repair_unit_id": "9ee6be60-3575-11eb-8fca-273b55edb18f",
	"creation_time": "2020-12-03T14:41:25Z",
	"pause_time": "2020-12-03T14:46:28Z",
	"next_activation": "2020-12-10T14:46:25Z"
}
]
 */
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
	SourceNode    string `json:"sourceNode"`
	EndpointNames []string `json:"endpointNames,omitempty"`
	TotalLoad     float64 `json:"totalLoad,omitempty"`
	Endpoints     map[string]map[string][]endpointStatus
}

type endpointStatus struct {
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
