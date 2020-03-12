package reaper

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

const (
	reaperURL = "http://localhost:8080"
)

func TestGetClusterNames(t *testing.T) {
	client, err := NewClient(reaperURL)
	if err != nil {
		t.Fatalf("failed to create reaper client: (%s)", err)
	}

	expected := []string{"cluster-1", "cluster-2"}

	actual, err := client.GetClusterNames()
	if err != nil {
		t.Fatalf("failed to get cluster names: (%s)", err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestGetCluster(t *testing.T) {
	client, err := NewClient(reaperURL)
	if err != nil {
		t.Fatalf("failed to create reaper client: (%s)", err)
	}

	name := "cluster-1"
	cluster, err := client.GetCluster(name)
	if err != nil {
		t.Fatalf("failed to get cluster (%s): %s", name, err)
	}

	assert.Equal(t, cluster.Name, name)
	assert.Equal(t, cluster.JmxUsername, "reaperUser")
	assert.True(t, cluster.JmxPasswordSet)
	assert.Equal(t, len(cluster.Seeds), 2)
}
