package reaper

import (
	"context"
	"testing"
	"time"

	"github.com/k8ssandra/reaper-client-go/testenv"
	"github.com/stretchr/testify/assert"
)

const (
	reaperURL = "http://localhost:8080"
)

type clientTest func(*testing.T, *Client)

func run(client *Client, test clientTest) func (*testing.T) {
	return func(t *testing.T) {
		//name := runtime.FuncForPC(reflect.ValueOf(test).Pointer()).Name()
		//t.Logf("running %s\n", name)
		test(t, client)
	}
}

func TestClient(t *testing.T) {
	t.Log("starting test")

	client, err := newClient(reaperURL)
	if err != nil {
		t.Fatalf("failed to create reaper client: (%s)", err)
	}

	if err = testenv.ResetServices(t); err != nil {
		t.Fatalf("failed to reset docker services: %s", err)
	}

	if err = testenv.WaitForClusterReady(t,"cluster-1-node-0", 2); err != nil {
		t.Fatalf("cluster-1 readiness check failed: %s", err)
	}
	if err = testenv.WaitForClusterReady(t,"cluster-2-node-0", 2); err != nil {
		t.Fatalf("cluster-2 readiness check failed: %s", err)
	}
	if err = testenv.WaitForClusterReady(t,"cluster-3-node-0", 1); err != nil {
		t.Fatalf("cluster-1 readiness check failed: %s", err)
	}

	isUp := false
	for i := 0; i < 10; i++ {
		t.Log("checking if reaper is ready")
		if isUp, err = client.IsReaperUp(context.Background()); err == nil {
			if isUp {
				t.Log("reaper is ready!")
				break
			}
		} else {
			t.Logf("reaper readiness check failed: %s", err)
		}
		time.Sleep(6 * time.Second)
	}
	if !isUp {
		t.Fatalf("reaper readiness check timed out")
	}

	if err = testenv.AddCluster(t,"cluster-1", "cluster-1-node-0"); err != nil {
		t.Fatalf("failed to add cluster-1: %s", err)
	}
	if err = testenv.AddCluster(t,"cluster-2", "cluster-2-node-0"); err != nil {
		t.Fatalf("failed to add cluster-2: %s", err)
	}

	t.Run("GetClusterNames", run(client, testGetClusterNames))
	t.Run("GetCluster", run(client, testGetCluster))
	t.Run("GetClusterNotFound", run(client, testGetClusterNotFound))
	t.Run("GetClusters", run(client, testGetClusters))
	t.Run("GetClustersSync", run(client, testGetClustersSyc))
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

func testGetClusterNotFound(t *testing.T, client *Client) {
	name := "cluster-notfound"
	cluster, err := client.GetCluster(context.TODO(), name)

	if err != CassandraClusterNotFound {
		t.Errorf("expected (%s) but got (%s)", CassandraClusterNotFound, err)
	}

	assert.Nil(t, cluster, "expected non-existent cluster to be nil")
}

func testGetClusters(t *testing.T, client *Client) {
	results := make([]GetClusterResult, 0)

	for result := range client.GetClusters(context.TODO()) {
		results = append(results, result)
	}

	// Verify that we got the expected number of results
	assert.Equal(t, 2, len(results))

	// Verify that there were no errors
	for _, result := range results {
		assert.Nil(t, result.Error)
	}

	assertGetClusterResultsContains(t, results, "cluster-1")
	assertGetClusterResultsContains(t, results, "cluster-2")
}

func assertGetClusterResultsContains(t *testing.T, results []GetClusterResult, clusterName string) {
	var cluster *Cluster
	for _, result := range results {
		if result.Cluster.Name == clusterName {
			cluster = result.Cluster
			break
		}
	}
	assert.NotNil(t, cluster, "failed to find %s", clusterName)
}

func testGetClustersSyc(t *testing.T, client *Client) {
	clusters, err := client.GetClustersSync(context.TODO())

	if err != nil {
		t.Fatalf("failed to get clusters synchronously: %s", err)
	}

	// Verify that we got the expected number of results
	assert.Equal(t, 2, len(clusters))

	assertClustersContains(t, clusters, "cluster-1")
	assertClustersContains(t, clusters, "cluster-2")
}

func assertClustersContains(t *testing.T, clusters []*Cluster, clusterName string) {
	for _, cluster := range clusters {
		if cluster.Name == clusterName {
			return
		}
	}
	t.Errorf("failed to find cluster (%s)", clusterName)
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
