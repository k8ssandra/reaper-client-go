package testenv

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"
)

var cassandraReadyStatusRegex = regexp.MustCompile(`\nUN `)

// Stops all services declared in docker-compose.yaml. This function blocks until the
// operation completes.
func StopServices() error {
	stopServices := exec.Command("docker-compose", "down")
	return stopServices.Run()
}

// Starts all services declared in docker-compose.yaml in detached mode.
func StartServices() error {
	startServices := exec.Command("docker-compose", "up", "-d")
	return startServices.Run()
}

// Deletes all contents under PROJECT_ROOT/data/cassandra
func PurgeCassandraDataDir() error {
	cassandrDataDir, err := filepath.Abs("../data/cassandra")
	if err != nil {
		return fmt.Errorf("failed to get path of cassandra data dir: %w", err)
	}

	if err := os.RemoveAll(cassandrDataDir); err != nil {
		return fmt.Errorf("failed to purge %s: %w", cassandrDataDir, err)
	}

	return nil
}

// A convenience function that does the following:
//
//    * stop all services
//    * purge cassandra data directory
//    * start all services
func ResetServices() error {
	if err := StopServices(); err != nil {
		return fmt.Errorf("failed to stop services: %w", err)
	}

	if err := PurgeCassandraDataDir(); err != nil {
		return fmt.Errorf("failed to purge cassandra data dir: %w", err)
	}

	if err := StartServices(); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

	return nil
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
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err = checkStatus.Start(); err != nil {
		return nil, fmt.Errorf("failed to check cassandra status with seed node (%s): %w", seed, err)
	}

	bytes, err := ioutil.ReadAll(outPipe)
	if err != nil {
		return nil, fmt.Errorf("failed to read stdout for cassandra status check with seed node (%s): %w", seed, err)
	}

	err = checkStatus.Wait()
	if err != nil {
		return nil, fmt.Errorf("failed waiting for cassandra status check to complete with seed node (%s): %w", seed, err)
	}

	return bytes, nil
}

// Runs nodetool status against the seed node. Blocks until numNodes nodes report a status of UN.
// This function will perform a max of 10 checks with a delay of one second between retries.
func WaitForClusterReady(seed string, numNodes int) error {
	// TODO make the number of checks configurable
	for i := 0; i < 10; i++ {
		bytes, err := checkCassandraStatus(seed)
		if err == nil {
			matches := cassandraReadyStatusRegex.FindAll(bytes, -1)
			if matches != nil && len(matches) == numNodes {
				return nil
			}
		}
		// TODO make the duration configurable
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timed out waiting for nodetool status with seed (%s)", seed)
}

// Adds a cluster to Reaper without using the client.
func AddCluster(cluster string, seed string) error {
	relPath := "../scripts/add-cluster.sh"
	path, err := filepath.Abs(relPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of (%s): %w", relPath, err)
	}
	script := exec.Command(path, cluster, seed)
	if err = script.Run(); err != nil {
		return fmt.Errorf("add cluster script (%s) failed with seed (%s): %w", path, seed, err)
	}

	return nil
}