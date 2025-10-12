package accounting

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/rclone/rclone/fstest/testy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsGroupOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("empty group returns nil", func(t *testing.T) {
		t.Parallel()
		sg := newStatsGroups()
		sg.get("invalid-group")
	})

	t.Run("set assigns stats to group", func(t *testing.T) {
		t.Parallel()
		stats := NewStats(ctx)
		sg := newStatsGroups()
		sg.set(ctx, "test", stats)
		sg.set(ctx, "test1", stats)
		if len(sg.m) != len(sg.names()) || len(sg.m) != 2 {
			t.Fatalf("Expected two stats got %d, %d", len(sg.m), len(sg.order))
		}
	})

	t.Run("get returns correct group", func(t *testing.T) {
		t.Parallel()
		stats := NewStats(ctx)
		sg := newStatsGroups()
		sg.set(ctx, "test", stats)
		sg.set(ctx, "test1", stats)
		got := sg.get("test")
		if got != stats {
			t.Fatal("get returns incorrect stats")
		}
	})

	t.Run("sum returns correct values", func(t *testing.T) {
		t.Parallel()
		stats1 := NewStats(ctx)
		stats1.bytes = 5
		stats1.transferQueueSize = 10
		stats1.errors = 6
		stats1.oldDuration = time.Second
		stats1.oldTimeRanges = []timeRange{{time.Now(), time.Now().Add(time.Second)}}
		stats2 := NewStats(ctx)
		stats2.bytes = 10
		stats2.errors = 12
		stats1.transferQueueSize = 20
		stats2.oldDuration = 2 * time.Second
		stats2.oldTimeRanges = []timeRange{{time.Now(), time.Now().Add(2 * time.Second)}}
		sg := newStatsGroups()
		sg.set(ctx, "test1", stats1)
		sg.set(ctx, "test2", stats2)
		sum := sg.sum(ctx)
		assert.Equal(t, stats1.bytes+stats2.bytes, sum.bytes)
		assert.Equal(t, stats1.transferQueueSize+stats2.transferQueueSize, sum.transferQueueSize)
		assert.Equal(t, stats1.errors+stats2.errors, sum.errors)
		assert.Equal(t, stats1.oldDuration+stats2.oldDuration, sum.oldDuration)
		assert.Equal(t, stats1.average.speed+stats2.average.speed, sum.average.speed)
		// dict can iterate in either order
		a := timeRanges{stats1.oldTimeRanges[0], stats2.oldTimeRanges[0]}
		b := timeRanges{stats2.oldTimeRanges[0], stats1.oldTimeRanges[0]}
		if !assert.ObjectsAreEqual(a, sum.oldTimeRanges) {
			assert.Equal(t, b, sum.oldTimeRanges)
		}
	})

	t.Run("delete removes stats", func(t *testing.T) {
		t.Parallel()
		stats := NewStats(ctx)
		sg := newStatsGroups()
		sg.set(ctx, "test", stats)
		sg.set(ctx, "test1", stats)
		sg.delete("test1")
		if sg.get("test1") != nil {
			t.Fatal("stats not deleted")
		}
		if len(sg.m) != len(sg.names()) || len(sg.m) != 1 {
			t.Fatalf("Expected two stats got %d, %d", len(sg.m), len(sg.order))
		}
	})

	t.Run("memory is reclaimed", func(t *testing.T) {
		testy.SkipUnreliable(t)
		var (
			count      = 1000
			start, end runtime.MemStats
			sg         = newStatsGroups()
		)

		runtime.GC()
		runtime.ReadMemStats(&start)

		for i := range count {
			sg.set(ctx, fmt.Sprintf("test-%d", i), NewStats(ctx))
		}

		for i := range count {
			sg.delete(fmt.Sprintf("test-%d", i))
		}

		runtime.GC()
		runtime.ReadMemStats(&end)

		t.Logf("%+v\n%+v", start, end)
		diff := percentDiff(start.HeapObjects, end.HeapObjects)
		if diff > 1 {
			t.Errorf("HeapObjects = %d, expected %d", end.HeapObjects, start.HeapObjects)
		}
	})

	testGroupStatsInfo := NewStatsGroup(ctx, "test-group")
	require.NoError(t, testGroupStatsInfo.DeleteFile(ctx, 0))
	for range 41 {
		require.NoError(t, GlobalStats().DeleteFile(ctx, 0))
	}

	t.Run("core/group-list", func(t *testing.T) {
		call := rc.Calls.Get("core/group-list")
		require.NotNil(t, call)
		got, err := call.Fn(ctx, rc.Params{})
		require.NoError(t, err)
		require.Equal(t, rc.Params{
			"groups": []string{
				"test-group",
			},
		}, got)
	})

	t.Run("core/stats", func(t *testing.T) {
		tr := Stats(ctx).NewCheckingTransfer(mockobject.New("core-check"), "deleting")
		// defer tr.Done(ctx, nil)
		_ = tr // don't finish the transfer so we don't mess up the other tests
		tr2 := Stats(ctx).NewTransfer(mockobject.New("core-transfer"), nil)
		//defer tr2.Done(ctx, nil)
		_ = tr2 // don't finish the transfer so we don't mess up the other tests

		call := rc.Calls.Get("core/stats")
		require.NotNil(t, call)

		got, err := call.Fn(ctx, rc.Params{})
		require.NoError(t, err)
		assert.Equal(t, int64(42), got["deletes"])
		data, err := json.Marshal(got["transferring"])
		require.NoError(t, err)
		assert.Contains(t, string(data), "core-transfer")
		data, err = json.Marshal(got["checking"])
		require.NoError(t, err)
		assert.Contains(t, string(data), "core-check")

		got, err = call.Fn(ctx, rc.Params{"short": true})
		require.NoError(t, err)
		assert.Equal(t, int64(42), got["deletes"])
		assert.Nil(t, got["transferring"])
		assert.Nil(t, got["checking"])

		got, err = call.Fn(ctx, rc.Params{"group": "test-group"})
		require.NoError(t, err)
		assert.Equal(t, int64(1), got["deletes"])
	})

	t.Run("core/transferred", func(t *testing.T) {
		call := rc.Calls.Get("core/transferred")
		require.NotNil(t, call)
		gotNoGroup, err := call.Fn(ctx, rc.Params{})
		require.NoError(t, err)
		gotGroup, err := call.Fn(ctx, rc.Params{"group": "test-group"})
		require.NoError(t, err)
		assert.Equal(t, rc.Params{
			"transferred": []TransferSnapshot{},
		}, gotNoGroup)
		assert.Equal(t, rc.Params{
			"transferred": []TransferSnapshot{},
		}, gotGroup)
	})

	t.Run("core/stats-reset", func(t *testing.T) {
		call := rc.Calls.Get("core/stats-reset")
		require.NotNil(t, call)

		assert.Equal(t, int64(41), GlobalStats().deletes)
		assert.Equal(t, int64(1), testGroupStatsInfo.deletes)

		_, err := call.Fn(ctx, rc.Params{"group": "test-group"})
		require.NoError(t, err)

		assert.Equal(t, int64(41), GlobalStats().deletes)
		assert.Equal(t, int64(0), testGroupStatsInfo.deletes)

		_, err = call.Fn(ctx, rc.Params{})
		require.NoError(t, err)

		assert.Equal(t, int64(0), GlobalStats().deletes)
		assert.Equal(t, int64(0), testGroupStatsInfo.deletes)

		_, err = call.Fn(ctx, rc.Params{"group": "not-found"})
		require.ErrorContains(t, err, `group "not-found" not found`)

	})

	testGroupStatsInfo = NewStatsGroup(ctx, "test-group")

	t.Run("core/stats-delete", func(t *testing.T) {
		call := rc.Calls.Get("core/stats-delete")
		require.NotNil(t, call)

		assert.Equal(t, []string{"test-group"}, groups.names())

		_, err := call.Fn(ctx, rc.Params{"group": "test-group"})
		require.NoError(t, err)

		assert.Equal(t, []string{}, groups.names())

		_, err = call.Fn(ctx, rc.Params{"group": "not-found"})
		require.NoError(t, err)
	})
}

func TestCountError(t *testing.T) {
	ctx := context.Background()
	Start(ctx)
	defer func() {
		groups = newStatsGroups()
	}()
	t.Run("global stats", func(t *testing.T) {
		GlobalStats().ResetCounters()
		err := fs.CountError(ctx, fmt.Errorf("global err"))
		assert.Equal(t, int64(1), GlobalStats().errors)

		assert.True(t, fserrors.IsCounted(err))
	})
	t.Run("group stats", func(t *testing.T) {
		statGroupName := fmt.Sprintf("%s-error_group", t.Name())
		GlobalStats().ResetCounters()
		stCtx := WithStatsGroup(ctx, statGroupName)
		st := StatsGroup(stCtx, statGroupName)

		err := fs.CountError(stCtx, fmt.Errorf("group err"))

		assert.Equal(t, int64(0), GlobalStats().errors)
		assert.Equal(t, int64(1), st.errors)
		assert.True(t, fserrors.IsCounted(err))
	})

}

func percentDiff(start, end uint64) uint64 {
	if start == 0 {
		return 0 // Handle zero start value to avoid division by zero
	}
	var diff uint64
	if end > start {
		diff = end - start // Handle case where end is larger than start
	} else {
		diff = start - end
	}
	return (diff * 100) / start
}
