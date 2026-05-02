package smb

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// alwaysClosed implements closeder for a dead connection in tests.
type alwaysClosed struct{}

func (alwaysClosed) closed() bool { return true }

// neverClosed implements closeder for an alive connection in tests.
type neverClosed struct{}

func (neverClosed) closed() bool { return false }

// newDeadConn creates a conn that reports itself as closed and has been
// in the pool long enough to trigger the liveness check.
func newDeadConn(share string) *conn {
	return &conn{
		shareName: share,
		pooledAt:  time.Now().Add(-10 * time.Minute), // old enough to be checked
		closeder:  alwaysClosed{},
	}
}

// newAliveConn creates a conn that reports itself as alive.
func newAliveConn(share string) *conn {
	return &conn{
		shareName: share,
		closeder:  neverClosed{},
	}
}

// newTestFs creates a minimal Fs suitable for pool tests.
func newTestFs() *Fs {
	ctx := context.Background()
	f := &Fs{
		name: "test",
		opt: Options{
			Host:        "localhost",
			Port:        "445",
			IdleTimeout: fs.Duration(60 * time.Second),
		},
		ctx:   ctx,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(1*time.Millisecond), pacer.MaxSleep(10*time.Millisecond))),
	}
	f.pool = nil
	f.drain = time.AfterFunc(time.Duration(f.opt.IdleTimeout), func() {})
	return f
}

// TestConnClosed verifies the closed() method via the closeder interface.
func TestConnClosed(t *testing.T) {
	t.Run("dead connection", func(t *testing.T) {
		c := newDeadConn("share")
		assert.True(t, c.closed())
	})

	t.Run("alive connection", func(t *testing.T) {
		c := newAliveConn("share")
		assert.False(t, c.closed())
	})
}

// TestGetConnectionDiscardsDeadConnections verifies that getConnection
// skips over dead pooled connections instead of returning them to the caller.
func TestGetConnectionDiscardsDeadConnections(t *testing.T) {
	f := newTestFs()

	alive := newAliveConn("myshare")

	// Pool: 3 dead connections, then 1 alive one.
	f.pool = []*conn{
		newDeadConn("myshare"),
		newDeadConn("myshare"),
		newDeadConn("myshare"),
		alive,
	}

	c, err := f.getConnection(context.Background(), "myshare")
	require.NoError(t, err)
	assert.Equal(t, alive, c, "should return the alive connection, not a dead one")

	// Pool should be empty now (3 dead discarded, 1 alive returned).
	f.poolMu.Lock()
	assert.Empty(t, f.pool)
	f.poolMu.Unlock()
}

// TestGetConnectionAllDeadFallsThrough verifies that when ALL pooled
// connections are dead, getConnection falls through to create a new one.
func TestGetConnectionAllDeadFallsThrough(t *testing.T) {
	f := newTestFs()

	f.pool = []*conn{
		newDeadConn("myshare"),
		newDeadConn("myshare"),
		newDeadConn("myshare"),
	}

	// getConnection will drain the pool, then try newConnection which
	// will fail (no real server). That's fine — we're testing pool cleanup.
	_, err := f.getConnection(context.Background(), "myshare")
	assert.Error(t, err, "should fail because no real server to connect to")

	// All dead connections should have been discarded.
	f.poolMu.Lock()
	assert.Empty(t, f.pool, "all dead connections should be discarded")
	f.poolMu.Unlock()
}

// TestGetConnectionAliveReturnedDirectly verifies that a healthy pooled
// connection is returned immediately without creating a new one.
func TestGetConnectionAliveReturnedDirectly(t *testing.T) {
	f := newTestFs()

	alive := newAliveConn("myshare")
	f.pool = []*conn{alive}

	c, err := f.getConnection(context.Background(), "myshare")
	require.NoError(t, err)
	assert.Equal(t, alive, c)
}

// TestGetConnectionRecentNotChecked verifies that a recently pooled connection
// is returned without an Echo check, even if it would report as dead.
func TestGetConnectionRecentNotChecked(t *testing.T) {
	f := newTestFs()

	recentDead := &conn{
		shareName: "myshare",
		pooledAt:  time.Now(), // just now — within IdleTimeout
		closeder:  alwaysClosed{},
	}
	f.pool = []*conn{recentDead}

	c, err := f.getConnection(context.Background(), "myshare")
	require.NoError(t, err)
	assert.Equal(t, recentDead, c, "recently pooled connection should be returned without Echo check")
}

// TestGetConnectionMixedPool verifies the correct ordering: dead connections
// are discarded from the front, and the first alive one is returned.
func TestGetConnectionMixedPool(t *testing.T) {
	f := newTestFs()

	first := newAliveConn("myshare")
	second := newAliveConn("myshare")

	f.pool = []*conn{
		newDeadConn("myshare"),
		first,
		second, // should stay in pool
	}

	c, err := f.getConnection(context.Background(), "myshare")
	require.NoError(t, err)
	assert.Equal(t, first, c, "should return first alive connection")

	// second should still be in the pool.
	f.poolMu.Lock()
	assert.Len(t, f.pool, 1)
	assert.Equal(t, second, f.pool[0])
	f.poolMu.Unlock()
}

// TestPutConnectionReturnsToPool verifies that putConnection returns
// a healthy connection to the pool.
func TestPutConnectionReturnsToPool(t *testing.T) {
	f := newTestFs()

	c := newAliveConn("myshare")
	f.putConnection(&c, nil)

	f.poolMu.Lock()
	assert.Len(t, f.pool, 1)
	f.poolMu.Unlock()

	assert.Nil(t, c)
}

// TestPutConnectionWithErrorChecksLiveness verifies that putConnection
// calls closed() when there's an error that isn't a standard fs error.
func TestPutConnectionWithErrorChecksLiveness(t *testing.T) {
	f := newTestFs()

	t.Run("alive after error goes back to pool", func(t *testing.T) {
		c := newAliveConn("myshare")
		someErr := assert.AnError
		f.putConnection(&c, someErr)

		f.poolMu.Lock()
		assert.Len(t, f.pool, 1, "alive connection should be returned to pool")
		f.pool = nil // reset
		f.poolMu.Unlock()
	})

	t.Run("dead after error is discarded", func(t *testing.T) {
		c := newDeadConn("myshare")
		someErr := assert.AnError
		f.putConnection(&c, someErr)

		f.poolMu.Lock()
		assert.Empty(t, f.pool, "dead connection should NOT be returned to pool")
		f.poolMu.Unlock()
	})
}

// TestConcurrentGetConnectionWithDeadPool verifies thread safety of the
// pool cleanup under concurrent access.
func TestConcurrentGetConnectionWithDeadPool(t *testing.T) {
	f := newTestFs()

	// Fill pool with a mix of dead and alive connections.
	for i := 0; i < 5; i++ {
		f.pool = append(f.pool, newDeadConn("myshare"))
		f.pool = append(f.pool, newAliveConn("myshare"))
	}

	var wg sync.WaitGroup
	var gotCount atomic.Int32

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := f.getConnection(context.Background(), "myshare")
			if err == nil && c != nil {
				gotCount.Add(1)
				f.putConnection(&c, nil)
			}
		}()
	}
	wg.Wait()

	got := gotCount.Load()
	assert.Equal(t, int32(5), got, "should have gotten 5 alive connections")
}
