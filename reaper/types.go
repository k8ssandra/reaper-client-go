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
	State string `json:"state,omitempty"`
	Intensity float64 `json:"intensity,omitempty"`
	//private final UUID id;
	//
	//private final UUID repairUnitId;
	//private final State state;
	//private final int daysBetween;
	//private final DateTime nextActivation;
	//private final ImmutableList<UUID> runHistory;
	//@Deprecated private final int segmentCount;
	//private final RepairParallelism repairParallelism;
	//private final double intensity;
	//private final DateTime creationTime;
	//private final String owner;
	//private final DateTime pauseTime;
	//private final int segmentCountPerNode;

	//public enum State {
	//ACTIVE,
	//PAUSED,
	//DELETED
	//}

	//public enum RepairParallelism {
	//SEQUENTIAL("sequential"),
	//PARALLEL("parallel"),
	//DATACENTER_AWARE("dc_parallel");


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
