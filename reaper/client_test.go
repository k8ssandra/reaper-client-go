package reaper

import (
	"context"
	"github.com/jsanda/reaper-client-go/testenv"
	"github.com/stretchr/testify/assert"
	"testing"
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

	if err = testenv.ResetServices(); err != nil {
		t.Fatalf("failed to reset docker services: %s", err)
	}

	if err = testenv.WaitForClusterReady("cluster-1-node-0", 2); err != nil {
		t.Fatalf("cluster-1 readiness check failed: %s", err)
	}
	if err = testenv.WaitForClusterReady("cluster-2-node-0", 2); err != nil {
		t.Fatalf("cluster-2 readiness check failed: %s", err)
	}
	if err = testenv.WaitForClusterReady("cluster-3-node-0", 1); err != nil {
		t.Fatalf("cluster-1 readiness check failed: %s", err)
	}
	// TODO add ready check for reaper

	testenv.AddCluster("cluster-1", "cluster-1-node-0")
	testenv.AddCluster("cluster-2", "cluster-2-node-0")

	t.Run("GetClusterNames", run(client, testGetClusterNames))
	t.Run("GetCluster", run(client, testGetCluster))
	t.Run("AddDeleteCluster", run(client, testAddDeleteCluster))
}

func testGetClusterNames(t *testing.T, client *Client) {
	expected := []string{"cluster-1", "cluster-2"}

	actual, err := client.GetClusterNames(context.TODO())
	if err != nil {
		t.Fatalf("failed to get cluster names: (%s)", err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func testGetCluster(t *testing.T, client *Client) {
	name := "cluster-1"
	cluster, err := client.GetCluster(context.TODO(), name)
	if err != nil {
		t.Fatalf("failed to get cluster (%s): %s", name, err)
	}

	assert.Equal(t, cluster.Name, name)
	assert.Equal(t, cluster.JmxUsername, "reaperUser")
	assert.True(t, cluster.JmxPasswordSet)
	assert.Equal(t, len(cluster.Seeds), 2)
	assert.Equal(t, 1, len(cluster.NodeState.GossipStates))

	gossipState := cluster.NodeState.GossipStates[0]
	assert.NotEmpty(t, gossipState.SourceNode)
	assert.True(t, gossipState.TotalLoad > 0.0)
	assert.Equal(t, 2, len(gossipState.EndpointNames), "EndpointNames (%s)", gossipState.EndpointNames)

	assert.Equal(t, 1, len(gossipState.DataCenters), "DataCenters (%+v)", gossipState.DataCenters)
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

func testAddDeleteCluster(t *testing.T, client *Client) {
	cluster := "cluster-3"
	seed := "cluster-3-node-0"

	if err := client.AddCluster(context.TODO(), cluster, seed); err != nil {
		t.Fatalf("failed to add cluster (%s): %s", cluster, err)
	}

	if clusterNames, err := client.GetClusterNames(context.TODO()); err != nil {
		t.Fatalf("failed to get cluster names: %s", err)
	} else {
		assert.Equal(t, 3, len(clusterNames))
	}

	if err := client.DeleteCluster(context.TODO(), cluster); err != nil {
		t.Fatalf("failed to delete cluster (%s): %s", cluster, err)
	}

	if clusterNames, err := client.GetClusterNames(context.TODO()); err != nil {
		t.Fatalf("failed to get cluster names: %s", err)
	} else {
		assert.Equal(t, 2, len(clusterNames))
		assert.NotContains(t, clusterNames, cluster)
	}
}
