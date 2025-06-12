package reaper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/k8ssandra/reaper-client-go/testenv"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
)

const (
	reaperURL = "http://localhost:8080"
	keyspace  = "reaper_client_test"
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

	ctx := context.Background()

	prepareEnvironment(t, ctx)
	t.Run("Login", run(client, testLogin))
	t.Run("Ping", run(client, testIsReaperUp))

	registerClusters(t, ctx, client)

	t.Log("running Cluster resource tests...")

	t.Run("GetClusterNames", run(client, testGetClusterNames))
	t.Run("GetCluster", run(client, testGetCluster))
	t.Run("GetClusterNotFound", run(client, testGetClusterNotFound))
	t.Run("GetClusters", run(client, testGetClusters))
	t.Run("GetClustersSync", run(client, testGetClustersSync))
	t.Run("AddDeleteCluster", run(client, testAddDeleteCluster))

	createFixtures(t, ctx)

	t.Log("running RepairRun resource tests...")

	t.Run("GetRepairRun", run(client, testGetRepairRun))
	t.Run("GetRepairRunNotFound", run(client, testGetRepairRunNotFound))
	t.Run("GetRepairRunIgnoredTables", run(client, testGetRepairRunIgnoredTables))
	t.Run("GetRepairRuns", run(client, testGetRepairRuns))
	t.Run("GetRepairRunsFilteredByCluster", run(client, testGetRepairRunsFilteredByCluster))
	t.Run("GetRepairRunsFilteredByKeyspace", run(client, testGetRepairRunsFilteredByKeyspace))
	t.Run("GetRepairRunsFilteredByState", run(client, testGetRepairRunsFilteredByState))
	t.Run("CreateDeleteRepairRun", run(client, testCreateDeleteRepairRun))
	t.Run("DeleteRepairRunNotFound", run(client, testDeleteRepairRunNotFound))
	t.Run("CreateStartFinishRepairRun", run(client, testCreateStartFinishRepairRun))
	t.Run("CreateStartPauseUpdateResumeRepairRun", run(client, testCreateStartPauseUpdateResumeRepairRun))
	t.Run("CreateAbortRepairRun", run(client, testCreateAbortRepairRun))
	t.Run("GetRepairRunSegments", run(client, testGetRepairRunSegments))
	t.Run("AbortRepairRunSegments", run(client, testAbortRepairRunSegments))
	t.Run("PurgeRepairRun", run(client, testPurgeRepairRun))

}

func prepareEnvironment(t *testing.T, parent context.Context) {
	if err := testenv.ResetServices(t); err != nil {
		t.Fatalf("failed to reset docker services: %s", err)
	}
	clusterReadinessGroup, ctx := errgroup.WithContext(parent)
	t.Log("checking cassandra cluster-1 status...")
	clusterReadinessGroup.Go(func() error {
		if err := testenv.WaitForClusterReady(ctx, "cluster-1-node-0", 2); err != nil {
			return fmt.Errorf("cluster-1 readiness check failed: %w", err)
		}
		return nil
	})
	t.Log("checking cassandra cluster-2 status...")
	clusterReadinessGroup.Go(func() error {
		if err := testenv.WaitForClusterReady(ctx, "cluster-2-node-0", 2); err != nil {
			return fmt.Errorf("cluster-2 readiness check failed: %w", err)
		}
		return nil
	})
	t.Log("checking cassandra cluster-3 status...")
	clusterReadinessGroup.Go(func() error {
		if err := testenv.WaitForClusterReady(ctx, "cluster-3-node-0", 1); err != nil {
			return fmt.Errorf("cluster-3 readiness check failed: %w", err)
		}
		return nil
	})
	if err := clusterReadinessGroup.Wait(); err != nil {
		t.Fatal(err)
	}
}

func registerClusters(t *testing.T, parent context.Context, client Client) {
	addClusterGroup, ctx := errgroup.WithContext(parent)
	t.Log("adding cluster-1 in Reaper...")
	addClusterGroup.Go(func() error {
		if err := client.AddCluster(ctx, "cluster-1", "cluster-1-node-0"); err != nil {
			return fmt.Errorf("failed to add cluster-1: %w", err)
		}
		return nil
	})
	t.Log("adding cluster-2 in Reaper...")
	addClusterGroup.Go(func() error {
		if err := client.AddCluster(ctx, "cluster-2", "cluster-2-node-0"); err != nil {
			return fmt.Errorf("failed to add cluster-2: %w", err)
		}
		return nil
	})
	// cluster-3 will be added by a test
	if err := addClusterGroup.Wait(); err != nil {
		t.Fatal(err)
	}
}

func createFixtures(t *testing.T, parent context.Context) {
	scriptsGroup, ctx := errgroup.WithContext(parent)
	scripts := make(chan *os.File, 2)
	t.Log("generating CQL scripts...")
	scriptsGroup.Go(func() error {
		if script, err := testenv.CreateCqlInsertScript(keyspace, "table1"); err != nil {
			return fmt.Errorf("failed to create table1 CQL script: %w", err)
		} else {
			scripts <- script
			return nil
		}
	})
	scriptsGroup.Go(func() error {
		if script, err := testenv.CreateCqlInsertScript(keyspace, "table2"); err != nil {
			return fmt.Errorf("failed to create table2 CQL script: %w", err)
		} else {
			scripts <- script
			return nil
		}
	})
	if err := scriptsGroup.Wait(); err != nil {
		t.Fatal(err)
	}
	script1 := <-scripts
	script2 := <-scripts
	cqlFixturesGroup, ctx := errgroup.WithContext(parent)
	t.Log("populating test keyspace in cluster-1...")
	cqlFixturesGroup.Go(func() error {
		if err := testenv.WaitForCqlReady(ctx, "cluster-1-node-0"); err != nil {
			return fmt.Errorf("CQL cluster-1 readiness check failed: %w", err)
		} else if err = testenv.CreateKeyspace(ctx, "cluster-1-node-0", keyspace, 2); err != nil {
			return fmt.Errorf("failed to create keyspace on cluster-1: %w", err)
		} else if err = testenv.CreateTable(ctx, "cluster-1-node-0", keyspace, "table1"); err != nil {
			return fmt.Errorf("failed to create keyspace on cluster-1: %w", err)
		} else if err = testenv.CreateTable(ctx, "cluster-1-node-0", keyspace, "table2"); err != nil {
			return fmt.Errorf("failed to create keyspace on cluster-1: %w", err)
		} else if err := testenv.ExecuteCqlScript(ctx, "cluster-1-node-0", script1); err != nil {
			return fmt.Errorf("failed to execute CQL script 1 on cluster-1: %w", err)
		} else if err := testenv.ExecuteCqlScript(ctx, "cluster-1-node-0", script2); err != nil {
			return fmt.Errorf("failed to execute CQL script 2 on cluster-1: %w", err)
		}
		return nil
	})
	t.Log("populating test keyspace in cluster-2...")
	cqlFixturesGroup.Go(func() error {
		if err := testenv.WaitForCqlReady(ctx, "cluster-2-node-0"); err != nil {
			return fmt.Errorf("CQL cluster-2 readiness check failed: %s", err)
		} else if err = testenv.CreateKeyspace(ctx, "cluster-2-node-0", keyspace, 2); err != nil {
			return fmt.Errorf("failed to create keyspace on cluster-2: %s", err)
		} else if err = testenv.CreateTable(ctx, "cluster-2-node-0", keyspace, "table1"); err != nil {
			return fmt.Errorf("failed to create keyspace on cluster-2: %s", err)
		} else if err = testenv.CreateTable(ctx, "cluster-2-node-0", keyspace, "table2"); err != nil {
			return fmt.Errorf("failed to create keyspace on cluster-2: %s", err)
		} else if err := testenv.ExecuteCqlScript(ctx, "cluster-2-node-0", script1); err != nil {
			return fmt.Errorf("failed to execute CQL script 1 on cluster-2: %s", err)
		} else if err := testenv.ExecuteCqlScript(ctx, "cluster-2-node-0", script2); err != nil {
			return fmt.Errorf("failed to execute CQL script 2 on cluster-2: %s", err)
		}
		return nil
	})
	if err := cqlFixturesGroup.Wait(); err != nil {
		t.Fatal(err)
	}
}

func testLogin(t *testing.T, client Client) {
	err := client.Login(context.TODO(), "reaperUser", "reaperPass")
	assert.NoError(t, err)
}

// Unit tests for the Login function logic using mocked HTTP responses
func TestLoginScenarios(t *testing.T) {
	t.Run("LoginWithJSessionIdFlow", testLoginWithJSessionIdFlow)
	t.Run("LoginWithDirectJwtToken", testLoginWithDirectJwtToken)
	t.Run("LoginWithNoCookies", testLoginWithNoCookies)
	t.Run("LoginWithInvalidJsonResponse", testLoginWithInvalidJsonResponse)
	t.Run("LoginWithJSessionIdButJwtFails", testLoginWithJSessionIdButJwtFails)
	t.Run("LoginPostFails", testLoginPostFails)
}

func testLoginWithJSessionIdFlow(t *testing.T) {
	// Mock server that returns JSESSIONID cookie for login and JWT for /jwt endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			// Verify form data
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

			// Read and verify form data
			body := make([]byte, r.ContentLength)
			r.Body.Read(body)
			formData := string(body)
			assert.Contains(t, formData, "username=testuser")
			assert.Contains(t, formData, "password=testpass")
			assert.Contains(t, formData, "rememberMe=false")

			// Set JSESSIONID cookie
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "test-session-id"})
			w.WriteHeader(http.StatusOK)
		case "/jwt":
			// Verify session cookie is present
			cookie, err := r.Cookie("JSESSIONID")
			assert.NoError(t, err)
			assert.Equal(t, "test-session-id", cookie.Value)

			// Return JWT token
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test-jwt-token"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create client with mock server URL
	u, _ := url.Parse(server.URL)
	reaperClient := NewClient(u)

	// Test login
	err := reaperClient.Login(context.Background(), "testuser", "testpass")
	assert.NoError(t, err)

	// Verify client state
	clientImpl := reaperClient.(*client)
	assert.Nil(t, clientImpl.jSessionId) // Should be cleared after JWT retrieval
	assert.NotNil(t, clientImpl.jwt)
	assert.Equal(t, "test-jwt-token", *clientImpl.jwt)
}

func testLoginWithDirectJwtToken(t *testing.T) {
	// Mock server that returns JWT token in JSON response body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			assert.Equal(t, "POST", r.Method)

			// Return JSON response with JWT token
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			jsonResp := `{"token":"eyJhbGciOiJIUzM4NCJ9.test-jwt-token","username":"testuser","roles":["operator"]}`
			w.Write([]byte(jsonResp))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create client with mock server URL
	u, _ := url.Parse(server.URL)
	reaperClient := NewClient(u)

	// Test login
	err := reaperClient.Login(context.Background(), "testuser", "testpass")
	assert.NoError(t, err)

	// Verify client state
	clientImpl := reaperClient.(*client)
	assert.Nil(t, clientImpl.jSessionId) // Should remain nil
	assert.NotNil(t, clientImpl.jwt)
	assert.Equal(t, "eyJhbGciOiJIUzM4NCJ9.test-jwt-token", *clientImpl.jwt)
}

func testLoginWithNoCookies(t *testing.T) {
	// Mock server that returns no cookies and no JWT token in response body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			assert.Equal(t, "POST", r.Method)

			// Return response without any cookies or JWT token
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("login response body"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create client with mock server URL
	u, _ := url.Parse(server.URL)
	reaperClient := NewClient(u)

	// Test login - should fail
	err := reaperClient.Login(context.Background(), "testuser", "testpass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to log in. no JSESSIONID cookie and no JWT token in response body")
	assert.Contains(t, err.Error(), "login response body")

	// Verify client state remains empty
	clientImpl := reaperClient.(*client)
	assert.Nil(t, clientImpl.jSessionId)
	assert.Nil(t, clientImpl.jwt)
}

func testLoginWithInvalidJsonResponse(t *testing.T) {
	// Mock server that returns invalid JSON response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid JSON response"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create client with mock server URL
	u, _ := url.Parse(server.URL)
	reaperClient := NewClient(u)

	// Test login - should fail
	err := reaperClient.Login(context.Background(), "testuser", "testpass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON response")

	// Verify client state remains empty
	clientImpl := reaperClient.(*client)
	assert.Nil(t, clientImpl.jSessionId)
	assert.Nil(t, clientImpl.jwt)
}

func testLoginWithJSessionIdButJwtFails(t *testing.T) {
	// Mock server where login succeeds but JWT endpoint fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			// Set JSESSIONID cookie
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "test-session-id"})
			w.WriteHeader(http.StatusOK)
		case "/jwt":
			// JWT endpoint fails
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("JWT service unavailable"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create client with mock server URL
	u, _ := url.Parse(server.URL)
	reaperClient := NewClient(u)

	// Test login - should fail when trying to get JWT
	err := reaperClient.Login(context.Background(), "testuser", "testpass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT service unavailable")

	// Verify client state has session ID but no JWT (login sets session ID before calling getJwt)
	clientImpl := reaperClient.(*client)
	assert.NotNil(t, clientImpl.jSessionId)
	assert.Equal(t, "test-session-id", *clientImpl.jSessionId)
	assert.Nil(t, clientImpl.jwt)
}

func testLoginPostFails(t *testing.T) {
	// Mock server that returns error for login
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Invalid credentials"))
		}
	}))
	defer server.Close()

	// Create client with mock server URL
	u, _ := url.Parse(server.URL)
	reaperClient := NewClient(u)

	// Test login - should fail immediately
	err := reaperClient.Login(context.Background(), "testuser", "testpass")
	assert.Error(t, err)

	// Verify client state remains empty
	clientImpl := reaperClient.(*client)
	assert.Nil(t, clientImpl.jSessionId)
	assert.Nil(t, clientImpl.jwt)
}
