package reaper

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func testGetRepairRun(t *testing.T, client Client) {
	expected := createRepairRun(t, client, "cluster-1")
	defer deleteRepairRun(t, client, expected)
	actual, err := client.GetRepairRun(
		context.Background(),
		expected.Id,
	)
	require.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func testGetRepairRunNotFound(t *testing.T, client Client) {
	nonExistentRepairRun, _ := uuid.NewUUID()
	actual, err := client.GetRepairRun(
		context.Background(),
		nonExistentRepairRun,
	)
	assert.Nil(t, actual)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("repair run %v doesn't exist", nonExistentRepairRun))
}

func testGetRepairRunIgnoredTables(t *testing.T, client Client) {
	runId, err := client.CreateRepairRun(
		context.Background(),
		"cluster-2",
		keyspace,
		"Bob",
		&RepairRunCreateOptions{IgnoredTables: []string{"table2"}},
	)
	require.Nil(t, err)
	repairRun, err := client.GetRepairRun(context.Background(), runId)
	assert.Nil(t, err)
	assert.Equal(t, repairRun.Tables, []string{"table1"})
	assert.Equal(t, repairRun.IgnoredTables, []string{"table2"})
	err = client.DeleteRepairRun(context.Background(), runId, "Bob")
	assert.Nil(t, err)
}

func testGetRepairRuns(t *testing.T, client Client) {
	run1 := createRepairRun(t, client, "cluster-1")
	run2 := createRepairRun(t, client, "cluster-2")
	defer deleteRepairRun(t, client, run1)
	defer deleteRepairRun(t, client, run2)
	repairRuns, err := client.GetRepairRuns(context.Background(), nil)
	require.Nil(t, err)
	assert.Len(t, repairRuns, 2)
	assert.Contains(t, repairRuns, run1)
	assert.Contains(t, repairRuns, run2)
}

func testGetRepairRunsFilteredByCluster(t *testing.T, client Client) {
	run1 := createRepairRun(t, client, "cluster-1")
	run2 := createRepairRun(t, client, "cluster-2")
	defer deleteRepairRun(t, client, run1)
	defer deleteRepairRun(t, client, run2)
	repairRuns, err := client.GetRepairRuns(
		context.Background(),
		&RepairRunSearchOptions{
			Cluster: "cluster-1",
		},
	)
	require.Nil(t, err)
	assert.Len(t, repairRuns, 1)
	assert.Contains(t, repairRuns, run1)
	assert.NotContains(t, repairRuns, run2)
}

func testGetRepairRunsFilteredByKeyspace(t *testing.T, client Client) {
	run1 := createRepairRun(t, client, "cluster-1")
	run2 := createRepairRun(t, client, "cluster-2")
	defer deleteRepairRun(t, client, run1)
	defer deleteRepairRun(t, client, run2)
	repairRuns, err := client.GetRepairRuns(
		context.Background(),
		&RepairRunSearchOptions{
			Keyspace: keyspace,
		},
	)
	require.Nil(t, err)
	assert.Len(t, repairRuns, 2)
	assert.Contains(t, repairRuns, run1)
	assert.Contains(t, repairRuns, run2)
	repairRuns, err = client.GetRepairRuns(
		context.Background(),
		&RepairRunSearchOptions{
			Keyspace: "nonexistent_keyspace",
		},
	)
	require.Nil(t, err)
	assert.Len(t, repairRuns, 0)
}

func testGetRepairRunsFilteredByState(t *testing.T, client Client) {
	run1 := createRepairRun(t, client, "cluster-1")
	run2 := createRepairRun(t, client, "cluster-2")
	defer deleteRepairRun(t, client, run1)
	defer deleteRepairRun(t, client, run2)
	repairRuns, err := client.GetRepairRuns(
		context.Background(),
		&RepairRunSearchOptions{
			States: []RepairRunState{RepairRunStateNotStarted},
		},
	)
	require.Nil(t, err)
	assert.Len(t, repairRuns, 2)
	assert.Contains(t, repairRuns, run1)
	assert.Contains(t, repairRuns, run2)
	repairRuns, err = client.GetRepairRuns(
		context.Background(),
		&RepairRunSearchOptions{
			States: []RepairRunState{RepairRunStateRunning, RepairRunStateDone},
		},
	)
	require.Nil(t, err)
	assert.Len(t, repairRuns, 0)
}

func testCreateDeleteRepairRun(t *testing.T, client Client) {
	run := createRepairRun(t, client, "cluster-1")
	deleteRepairRun(t, client, run)
	repairRuns, err := client.GetRepairRuns(context.Background(), nil)
	require.Nil(t, err)
	assert.Len(t, repairRuns, 0)
}

func testCreateStartFinishRepairRun(t *testing.T, client Client) {
	run := createRepairRun(t, client, "cluster-1")
	defer deleteRepairRun(t, client, run)
	err := client.StartRepairRun(context.Background(), run.Id)
	require.Nil(t, err)
	done := waitForRepairRun(t, client, run, RepairRunStateDone)
	assert.Equal(t, RepairRunStateDone, done.State)
	segments, err := client.GetRepairRunSegments(context.Background(), done.Id)
	require.Nil(t, err)
	assert.Len(t, segments, 2)
	assert.NotNil(t, segments[0].StartTime)
	assert.NotNil(t, segments[1].StartTime)
	assert.NotNil(t, segments[0].EndTime)
	assert.NotNil(t, segments[1].EndTime)
	assert.Equal(t, RepairSegmentStateDone, segments[0].State)
	assert.Equal(t, RepairSegmentStateDone, segments[1].State)
}

func testCreateStartPauseUpdateResumeRepairRun(t *testing.T, client Client) {
	run := createRepairRun(t, client, "cluster-1")
	defer deleteRepairRun(t, client, run)
	err := client.StartRepairRun(context.Background(), run.Id)
	require.Nil(t, err)
	started, err := client.GetRepairRun(context.Background(), run.Id)
	require.Nil(t, err)
	assert.Equal(t, RepairRunStateRunning, started.State)
	err = client.PauseRepairRun(context.Background(), run.Id)
	if err != nil {
		// pause not possible because repair is DONE
		require.Contains(t, err.Error(), "Transition DONE->PAUSED not supported")
		done, err := client.GetRepairRun(context.Background(), run.Id)
		require.Nil(t, err)
		assert.Equal(t, RepairRunStateDone, done.State)
	} else {
		paused, err := client.GetRepairRun(context.Background(), run.Id)
		require.Nil(t, err)
		assert.Equal(t, RepairRunStatePaused, paused.State)
		err = client.UpdateRepairRun(context.Background(), run.Id, 0.5)
		require.Nil(t, err)
		updated, err := client.GetRepairRun(context.Background(), run.Id)
		require.Nil(t, err)
		assert.InDelta(t, 0.5, updated.Intensity, 0.001)
		err = client.ResumeRepairRun(context.Background(), run.Id)
		require.Nil(t, err)
		done := waitForRepairRun(t, client, run, RepairRunStateDone)
		assert.Equal(t, RepairRunStateDone, done.State)
	}
}

func testGetRepairRunSegments(t *testing.T, client Client) {
	run := createRepairRun(t, client, "cluster-1")
	defer deleteRepairRun(t, client, run)
	segments, err := client.GetRepairRunSegments(context.Background(), run.Id)
	require.Nil(t, err)
	assert.Len(t, segments, 2)
	assert.Nil(t, segments[0].StartTime)
	assert.Nil(t, segments[1].StartTime)
	assert.Nil(t, segments[0].EndTime)
	assert.Nil(t, segments[1].EndTime)
	assert.Equal(t, RepairSegmentStateNotStarted, segments[0].State)
	assert.Equal(t, RepairSegmentStateNotStarted, segments[1].State)
	checkRepairRunSegment(t, run, segments[0])
	checkRepairRunSegment(t, run, segments[1])
	err = client.StartRepairRun(context.Background(), run.Id)
	require.Nil(t, err)
	segments = waitForSegmentsStarted(t, client, run)
	// could be STARTED, RUNNING or DONE
	assert.NotEqual(t, RepairSegmentStateNotStarted, segments[0].State)
	assert.NotEqual(t, RepairSegmentStateNotStarted, segments[1].State)
	assert.NotNil(t, segments[0].StartTime)
	assert.NotNil(t, segments[1].StartTime)
	assert.True(t, segments[0].EndTime == nil || segments[0].State == RepairSegmentStateDone)
	assert.True(t, segments[1].EndTime == nil || segments[1].State == RepairSegmentStateDone)
	checkRepairRunSegment(t, run, segments[0])
	checkRepairRunSegment(t, run, segments[1])
	err = client.PauseRepairRun(context.Background(), run.Id)
	if err != nil {
		// pause not possible because repair is DONE
		require.Contains(t, err.Error(), "Transition DONE->PAUSED not supported")
	} else {
		segments, err = client.GetRepairRunSegments(context.Background(), run.Id)
		require.Nil(t, err)
		assert.Len(t, segments, 2)
		// some segments may be DONE or even RUNNING: cannot assert state here
		assert.NotNil(t, segments[0].StartTime)
		assert.NotNil(t, segments[1].StartTime)
		assert.True(t, segments[0].EndTime == nil || segments[0].State == RepairSegmentStateDone)
		assert.True(t, segments[1].EndTime == nil || segments[1].State == RepairSegmentStateDone)
		checkRepairRunSegment(t, run, segments[0])
		checkRepairRunSegment(t, run, segments[1])
		err = client.StartRepairRun(context.Background(), run.Id)
		require.Nil(t, err)
		done := waitForRepairRun(t, client, run, RepairRunStateDone)
		assert.Equal(t, RepairRunStateDone, done.State)
	}
	segments, err = client.GetRepairRunSegments(context.Background(), run.Id)
	require.Nil(t, err)
	assert.Len(t, segments, 2)
	assert.Equal(t, RepairSegmentStateDone, segments[0].State)
	assert.Equal(t, RepairSegmentStateDone, segments[1].State)
	assert.NotNil(t, segments[0].StartTime)
	assert.NotNil(t, segments[1].StartTime)
	assert.NotNil(t, segments[0].EndTime)
	assert.NotNil(t, segments[1].EndTime)
	checkRepairRunSegment(t, run, segments[0])
	checkRepairRunSegment(t, run, segments[1])
}

func testAbortRepairRunSegments(t *testing.T, client Client) {
	run := createRepairRun(t, client, "cluster-1")
	defer deleteRepairRun(t, client, run)
	err := client.StartRepairRun(context.Background(), run.Id)
	require.Nil(t, err)
	segments := waitForSegmentsStarted(t, client, run)
	// could be STARTED, RUNNING or DONE
	assert.NotEqual(t, RepairSegmentStateNotStarted, segments[0].State)
	assert.NotEqual(t, RepairSegmentStateNotStarted, segments[1].State)
	expectedStates := map[uuid.UUID]RepairSegmentState{
		segments[0].Id: RepairSegmentStateNotStarted,
		segments[1].Id: RepairSegmentStateNotStarted,
	}
	err = client.AbortRepairRunSegment(context.Background(), run.Id, segments[0].Id)
	if err != nil {
		require.Contains(t, err.Error(), "Cannot abort segment on repair run with status DONE")
		expectedStates[segments[0].Id] = RepairSegmentStateDone
	}
	err = client.AbortRepairRunSegment(context.Background(), run.Id, segments[1].Id)
	if err != nil {
		require.Contains(t, err.Error(), "Cannot abort segment on repair run with status DONE")
		expectedStates[segments[1].Id] = RepairSegmentStateDone
	}
	segments, err = client.GetRepairRunSegments(context.Background(), run.Id)
	require.Nil(t, err)
	require.Len(t, segments, 2)
	assert.Equal(t, expectedStates[segments[0].Id], segments[0].State)
	assert.Equal(t, expectedStates[segments[1].Id], segments[1].State)
}

func testDeleteRepairRunNotFound(t *testing.T, client Client) {
	nonExistentRepairRun, _ := uuid.NewUUID()
	err := client.DeleteRepairRun(context.Background(), nonExistentRepairRun, "Alice")
	assert.NotNil(t, err)
	// Reaper returns a spurious '%s' in the error message
	assert.Contains(t, err.Error(), fmt.Sprintf("Repair run %%s%v not found", nonExistentRepairRun))
}

func testPurgeRepairRun(t *testing.T, client Client) {
	run := createRepairRun(t, client, "cluster-1")
	defer deleteRepairRun(t, client, run)
	purged, err := client.PurgeRepairRuns(context.Background())
	require.Nil(t, err)
	assert.Equal(t, 0, purged)
}

func createRepairRun(t *testing.T, client Client, cluster string) *RepairRun {
	runId, err := client.CreateRepairRun(
		context.Background(),
		cluster,
		keyspace,
		"Alice",
		&RepairRunCreateOptions{
			Tables:              []string{"table1", "table2"},
			SegmentCountPerNode: 1,
			RepairParallelism:   RepairParallelismParallel,
			Intensity:           0.1,
			IncrementalRepair:   true,
			RepairThreadCount:   4,
			Cause:               "testing repair runs",
		},
	)
	require.Nil(t, err)
	repairRun, err := client.GetRepairRun(context.Background(), runId)
	require.Nil(t, err)
	return checkRepairRun(t, cluster, repairRun)
}

func checkRepairRun(t *testing.T, cluster string, actual *RepairRun) *RepairRun {
	assert.NotNil(t, actual.Id)
	assert.Equal(t, cluster, actual.Cluster)
	assert.Equal(t, keyspace, actual.Keyspace)
	assert.Equal(t, "Alice", actual.Owner)
	assert.Equal(t, "testing repair runs", actual.Cause)
	assert.ElementsMatch(t, []string{"table1", "table2"}, actual.Tables)
	assert.Equal(t, RepairRunStateNotStarted, actual.State)
	assert.InDelta(t, 0.1, actual.Intensity, 0.001)
	assert.Equal(t, true, actual.IncrementalRepair)
	// FIXME segment count per node is always 2
	// assert.Equal(t, 1, actual.SegmentCountPerNode)
	assert.Equal(t, RepairParallelismParallel, actual.RepairParallelism)
	assert.Equal(t, 0, actual.SegmentsRepaired)
	assert.Equal(t, "no events", actual.LastEvent)
	assert.Empty(t, actual.Duration)
	assert.Empty(t, actual.Nodes)
	assert.Empty(t, actual.Datacenters)
	assert.Empty(t, actual.IgnoredTables)
	assert.Equal(t, 4, actual.RepairThreadCount)
	assert.NotNil(t, actual.RepairUnitId)
	return actual
}

func checkRepairRunSegment(t *testing.T, run *RepairRun, actual *RepairSegment) {
	assert.NotNil(t, actual.Id)
	assert.Equal(t, run.Id, actual.RunId)
	assert.NotNil(t, actual.RepairUnitId)
	assert.NotNil(t, actual.TokenRange)
	assert.NotNil(t, actual.Coordinator)
}

func deleteRepairRun(t *testing.T, client Client, run *RepairRun) {
	_ = client.PauseRepairRun(context.Background(), run.Id)
	err := client.DeleteRepairRun(context.Background(), run.Id, "Alice")
	if err != nil && strings.Contains(err.Error(), "is currently running") {
		waitForRepairRun(t, client, run, RepairRunStateNotStarted)
		err = client.DeleteRepairRun(context.Background(), run.Id, "Alice")
	}
	require.Nil(t, err)
}

func waitForRepairRun(t *testing.T, client Client, run *RepairRun, state RepairRunState) *RepairRun {
	success := assert.Eventually(
		t,
		func() bool {
			actual, err := client.GetRepairRun(context.Background(), run.Id)
			return err == nil && actual.State == state
		},
		5*time.Minute,
		5*time.Second,
	)
	actual, err := client.GetRepairRun(context.Background(), run.Id)
	require.Nil(t, err)
	if success {
		return actual
	}
	t.Fatalf(
		"timed out waiting for repair to reach state %s, last state was: %s",
		state,
		actual.State,
	)
	return nil
}

func waitForSegmentsStarted(t *testing.T, client Client, run *RepairRun) []*RepairSegment {
	success := assert.Eventually(
		t,
		func() bool {
			segments, err := client.GetRepairRunSegments(context.Background(), run.Id)
			return err == nil &&
				segments[0].State != RepairSegmentStateNotStarted &&
				segments[1].State != RepairSegmentStateNotStarted
		},
		5*time.Minute,
		5*time.Second,
	)
	segments, err := client.GetRepairRunSegments(context.Background(), run.Id)
	require.Nil(t, err)
	if success {
		return segments
	}
	t.Fatalf(
		"timed out waiting for repair to start, last states were: %s, %s",
		segments[0].State,
		segments[1].State,
	)
	return nil
}
