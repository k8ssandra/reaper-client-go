package reaper

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

const (
	reaperURL = "http://localhost:8080"
)

var cassandraReadyStatusRegex = regexp.MustCompile(`\nUN `)

type clientTest func(*testing.T, *Client)

func run(client *Client, test clientTest) func (*testing.T) {
	return func(t *testing.T) {
		test(t, client)
	}
}

func checkCassandraStatus(seed string) ([]byte, error) {
	checkStatus := exec.Command(
		"docker-compose",
		"exec",
		"-T",
		seed,
		"nodetool",
		"-u",
		"reaperUser",
		"-pw",
		"reaperPass",
		"status",
	)

	outPipe, err := checkStatus.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err = checkStatus.Start(); err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadAll(outPipe)
	if err != nil {
		return nil, err
	}

	err = checkStatus.Wait()
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func waitForClusterReady(t *testing.T, seed string, numNodes int) {
	for i := 0; i < 10; i++ {
		bytes, err := checkCassandraStatus(seed)
		if err == nil {
			matches := cassandraReadyStatusRegex.FindAll(bytes, -1)
			if matches != nil && len(matches) == numNodes {
				return
			}
		}

		time.Sleep(1 * time.Second)
	}

	t.Fatalf("timed out waiting for nodetool status with seed (%s)", seed)
}

func addCluster(cluster string, seed string) {
	relPath := "../scripts/add-cluster.sh"
	path, err := filepath.Abs(relPath)
	if err != nil {
		log.Fatalf("failed to get absolute path of (%s): %s", relPath, err)
	}
	script := exec.Command(path, cluster, seed)
	if err = script.Run(); err != nil {
		log.Fatalf("failed to add cluster (%s) with seed (%s): %s", cluster, seed, err)
	}
}

func TestClient(t *testing.T) {
	client, err := NewClient(reaperURL)
	if err != nil {
		t.Fatalf("failed to create reaper client: (%s)", err)
	}

	stopServices := exec.Command("docker-compose", "down")
	if err := stopServices.Run(); err != nil {
		t.Fatalf("failed to stop docker services: %s", err)
	}

	cassandrDataDir, err := filepath.Abs("../data/cassandra")
	if err != nil {
		t.Fatalf("failed to get absolute path of cassandra data directory: %s", err)
	}
	if err := os.RemoveAll(cassandrDataDir); err != nil {
		t.Fatalf("failed to purge cassandra data directory: %s", err)
	}

	startServices := exec.Command("docker-compose", "up", "-d")
	if err := startServices.Run(); err != nil {
		t.Fatalf("failed to start docker services: %s", err)
	}

	waitForClusterReady(t, "cluster-1-node-0", 2)
	waitForClusterReady(t, "cluster-2-node-0", 2)
	waitForClusterReady(t, "cluster-3-node-0", 1)
	// TODO add ready check for reaper

	fmt.Println("Wait for services...")
	time.Sleep(2 * time.Second)

	addCluster("cluster-1", "cluster-1-node-0")
	addCluster("cluster-2", "cluster-2-node-0")

	t.Run("GetClusterNames", run(client, testGetClusterNames))
	t.Run("GetCluster", run(client, testGetCluster))
	t.Run("AddDeleteCluster", run(client, testAddDeleteCluster))
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

	if err := client.AddCluster(cluster, seed); err != nil {
		t.Fatalf("failed to add cluster (%s): %s", cluster, err)
	}

	if clusterNames, err := client.GetClusterNames(); err != nil {
		t.Fatalf("failed to get cluster names: %s", err)
	} else {
		assert.Equal(t, 3, len(clusterNames))
	}

	if err := client.DeleteCluster(cluster); err != nil {
		t.Fatalf("failed to delete cluster (%s): %s", cluster, err)
	}

	if clusterNames, err := client.GetClusterNames(); err != nil {
		t.Fatalf("failed to get cluster names: %s", err)
	} else {
		assert.Equal(t, 2, len(clusterNames))
		assert.NotContains(t, clusterNames, cluster)
	}
}
