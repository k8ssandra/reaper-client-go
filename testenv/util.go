package testenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

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
