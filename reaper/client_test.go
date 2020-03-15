package reaper

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

const (
	reaperURL = "http://localhost:8080"
)

type clientTest func(*testing.T, *Client)

func run(client *Client, test clientTest) func (*testing.T) {
	return func(t *testing.T) {
		test(t, client)
	}
}

func TestClient(t *testing.T) {
	client, err := NewClient(reaperURL)
	if err != nil {
		t.Fatalf("failed to create reaper client: (%s)", err)
	}

	t.Run("GetClusterNames", run(client, testGetClusterNames))
	t.Run("GetCluster", run(client, testGetCluster))
}

func testGetClusterNames(t *testing.T, client *Client) {
	expected := []string{"cluster-1", "cluster-2"}

	actual, err := client.GetClusterNames()
	if err != nil {
		t.Fatalf("failed to get cluster names: (%s)", err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func testGetCluster(t *testing.T, client *Client) {
	name := "cluster-1"
	cluster, err := client.GetCluster(name)
	if err != nil {
		t.Fatalf("failed to get cluster (%s): %s", name, err)
	}

	assert.Equal(t, cluster.Name, name)
	assert.Equal(t, cluster.JmxUsername, "reaperUser")
	assert.True(t, cluster.JmxPasswordSet)
	assert.Equal(t, len(cluster.Seeds), 2)
	assert.Equal(t, 1, len(cluster.NodeStatus.GossipStates))

	gossipState := cluster.NodeStatus.GossipStates[0]
	assert.NotEmpty(t, gossipState.SourceNode)
	assert.True(t, gossipState.TotalLoad > 0.0)
	assert.Equal(t, 2, len(gossipState.EndpointNames))

	assert.Equal(t, 1, len(gossipState.DataCenters))
	dcName := "datacenter1"
	dc, found := gossipState.DataCenters[dcName]
	if !found {
		t.Fatalf("failed to find DataCenter (%s)", dcName)
	}
	assert.Equal(t, dcName, dc.Name)

	assert.Equal(t, 1, len(dc.Racks))
	rackName := "rack1"
	rack, found := dc.Racks[rackName]
	if !found {
		t.Fatalf("failed to find Rack (%s)", rackName)
	}

	assert.Equal(t, 2, len(rack.Endpoints))
	for _, ep := range rack.Endpoints {
		assert.True(t, ep.Endpoint == gossipState.EndpointNames[0] || ep.Endpoint == gossipState.EndpointNames[1])
		assert.NotEmpty(t, ep.HostId)
		assert.Equal(t, dcName, ep.DataCenter)
		assert.Equal(t, rackName, ep.Rack)
		assert.NotEmpty(t, ep.Status)
		assert.Equal(t, "3.11.4", ep.ReleaseVersion)
		assert.NotEmpty(t, ep.Tokens)
	}
}
