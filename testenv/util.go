package testenv

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

var cassandraReadyStatusRegex = regexp.MustCompile(`\nUN `)

// StopServices stops all services declared in docker-compose.yaml. This function blocks until the
// operation completes.
func StopServices(t *testing.T) error {
	t.Log("stopping services")
	stopServices := exec.Command("docker-compose", "down")
	return stopServices.Run()
}

// StartServices starts all services declared in docker-compose.yaml in detached mode.
func StartServices(t *testing.T) error {
	t.Log("starting services")
	startServices := exec.Command("docker-compose", "up", "-d")
	return startServices.Run()
}

// PurgeCassandraDataDir deletes all contents under PROJECT_ROOT/data/cassandra
func PurgeCassandraDataDir(t *testing.T) error {
	t.Log("purging cassandra data dir")
	cassandraDataDir, err := filepath.Abs("../data/cassandra")
	if err != nil {
		return fmt.Errorf("failed to get path of cassandra data dir: %w", err)
	}

	if err := os.RemoveAll(cassandraDataDir); err != nil {
		return fmt.Errorf("failed to purge %s: %w", cassandraDataDir, err)
	}

	return nil
}

// ResetServices is a convenience function that does the following:
//
//    * stop all services
//    * purge cassandra data directory
//    * start all services
func ResetServices(t *testing.T) error {
	if err := StopServices(t); err != nil {
		return fmt.Errorf("failed to stop services: %w", err)
	}

	if err := PurgeCassandraDataDir(t); err != nil {
		return fmt.Errorf("failed to purge cassandra data dir: %w", err)
	}

	if err := StartServices(t); err != nil {
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

// WaitForClusterReady runs nodetool status against the seed node. Blocks until numNodes nodes report a status of UN.
func WaitForClusterReady(ctx context.Context, seed string, numNodes int) error {
	// TODO make the timeout configurable
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for {
		select {
		default:
			b, err := checkCassandraStatus(seed)
			if err == nil {
				matches := cassandraReadyStatusRegex.FindAll(b, -1)
				if matches != nil && len(matches) == numNodes {
					return nil
				}
			}
			// TODO make the duration configurable
			time.Sleep(time.Second)
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for nodetool status with seed (%s)", seed)
		}
	}
}

func WaitForCqlReady(ctx context.Context, seed string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for {
		select {
		default:
			err := checkCqlStatus(ctx, seed)
			if err == nil {
				return nil
			}
			time.Sleep(time.Second)
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for CQL readiness with seed (%s)", seed)
		}
	}
}

func checkCqlStatus(ctx context.Context, node string) error {
	s := "SELECT release_version FROM system.local"
	checkStatus := exec.CommandContext(
		ctx,
		"docker-compose",
		"exec",
		"-T",
		node,
		"cqlsh",
		"-u",
		"reaperUser",
		"-p",
		"reaperPass",
		"-e",
		s,
		node,
		"9042",
	)
	return checkStatus.Run()
}

func CreateKeyspace(ctx context.Context, node string, keyspace string, rf int) error {
	stmt := fmt.Sprintf(
		"CREATE KEYSPACE IF NOT EXISTS \"%s\" "+
			"WITH replication = {'class':'NetworkTopologyStrategy', 'datacenter1':%d} "+
			"AND durable_writes = true",
		keyspace,
		rf,
	)
	createKeyspace := exec.CommandContext(
		ctx,
		"docker-compose",
		"exec",
		"-T",
		node,
		"cqlsh",
		"-u",
		"reaperUser",
		"-p",
		"reaperPass",
		"-e",
		stmt,
		node,
		"9042",
	)
	return createKeyspace.Run()
}

func CreateTable(ctx context.Context, node string, keyspace string, table string) error {
	stmt := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS \"%s\".\"%s\" "+
			"(pk int, cc timeuuid, v text, "+
			"PRIMARY KEY (pk, cc))",
		keyspace,
		table,
	)
	createTable := exec.CommandContext(
		ctx,
		"docker-compose",
		"exec",
		"-T",
		node,
		"cqlsh",
		"-u",
		"reaperUser",
		"-p",
		"reaperPass",
		"-e",
		stmt,
		node,
		"9042",
	)
	return createTable.Run()
}

func CreateCqlInsertScript(keyspace string, table string) (*os.File, error) {
	script, err := ioutil.TempFile(os.TempDir(), "insert-*.cql")
	if err != nil {
		return nil, err
	}
	defer func(script *os.File) {
		_ = script.Close()
	}(script)
	for pk := 0; pk < 1000; pk++ {
		insert := fmt.Sprintf(
			"INSERT INTO \"%s\".\"%s\" (pk, cc, v) VALUES (%d, now(), '%s');\n",
			keyspace,
			table,
			pk,
			randomString(10, 100),
		)
		if _, err = script.WriteString(insert); err != nil {
			return nil, err
		}
	}
	return script, nil
}

func ExecuteCqlScript(ctx context.Context, node string, script *os.File) error {
	remotePath := "/tmp/" + path.Base(script.Name())
	copyScript := exec.Command("docker", "cp", script.Name(), "reaper-client-go_"+node+"_1:"+remotePath)
	err := copyScript.Run()
	if err != nil {
		return err
	}
	execScript := exec.CommandContext(
		ctx,
		"docker-compose",
		"exec",
		"-T",
		node,
		"cqlsh",
		"-u",
		"reaperUser",
		"-p",
		"reaperPass",
		"-f",
		remotePath,
		node,
		"9042",
	)
	return execScript.Run()
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randomString(min int, max int) string {
	rand.Seed(time.Now().UnixNano())
	length := rand.Intn(max-min+1) + min
	s := make([]rune, length)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
