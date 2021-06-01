package reaper

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/k8ssandra/reaper-client-go/testenv"
)

const (
	reaperURL = "http://localhost:8080"
)

type clientTest func(*testing.T, Client)

func run(client Client, test clientTest) func(*testing.T) {
	return func(t *testing.T) {
		// name := runtime.FuncForPC(reflect.ValueOf(test).Pointer()).Name()
		// t.Logf("running %s\n", name)
		test(t, client)
	}
}

func TestClient(t *testing.T) {
	t.Log("starting test")

	u, _ := url.Parse(reaperURL)
	client := NewClient(u)

	if err := testenv.ResetServices(t); err != nil {
		t.Fatalf("failed to reset docker services: %s", err)
	}

	if err := testenv.WaitForClusterReady(t, "cluster-1-node-0", 2); err != nil {
		t.Fatalf("cluster-1 readiness check failed: %s", err)
	}
	if err := testenv.WaitForClusterReady(t, "cluster-2-node-0", 2); err != nil {
		t.Fatalf("cluster-2 readiness check failed: %s", err)
	}
	if err := testenv.WaitForClusterReady(t, "cluster-3-node-0", 1); err != nil {
		t.Fatalf("cluster-1 readiness check failed: %s", err)
	}

	isUp := false
	for i := 0; i < 10; i++ {
		t.Log("checking if reaper is ready")
		var err error
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

	if err := testenv.AddCluster(t, "cluster-1", "cluster-1-node-0"); err != nil {
		t.Fatalf("failed to add cluster-1: %s", err)
	}
	if err := testenv.AddCluster(t, "cluster-2", "cluster-2-node-0"); err != nil {
		t.Fatalf("failed to add cluster-2: %s", err)
	}

	t.Run("GetClusterNames", run(client, testGetClusterNames))
	t.Run("GetCluster", run(client, testGetCluster))
	t.Run("GetClusterNotFound", run(client, testGetClusterNotFound))
	t.Run("GetClusters", run(client, testGetClusters))
	t.Run("GetClustersSync", run(client, testGetClustersSync))
	t.Run("AddDeleteCluster", run(client, testAddDeleteCluster))
}
