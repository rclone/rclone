package smb

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newLimitedFs makes an Fs whose connections are fakes, limited to max
func newLimitedFs(t *testing.T, max int) (*Fs, *atomic.Int32) {
	ctx := context.Background()
	var dials atomic.Int32
	f := &Fs{
		name:     "test",
		maxConns: max,
		connFree: make(chan struct{}, 1),
		pacer:    fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.dialConn = func(ctx context.Context, share string) (*conn, error) {
		dials.Add(1)
		return &conn{shareName: share}, nil
	}
	return f, &dials
}

func (f *Fs) liveConns() int {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	return f.live
}

func TestConnPoolCapWaitsAndReuses(t *testing.T) {
	ctx := context.Background()
	f, dials := newLimitedFs(t, 2)

	c1, err := f.getConnection(ctx, "share")
	require.NoError(t, err)
	c2, err := f.getConnection(ctx, "share")
	require.NoError(t, err)
	assert.Equal(t, 2, f.liveConns())

	got := make(chan *conn)
	go func() {
		c, err := f.getConnection(ctx, "share")
		assert.NoError(t, err)
		got <- c
	}()
	select {
	case <-got:
		t.Fatal("third connection opened past the limit")
	case <-time.After(300 * time.Millisecond):
	}

	want := c1
	f.putConnection(&c1, nil)
	select {
	case c3 := <-got:
		assert.Same(t, want, c3, "waiter should reuse the returned connection")
	case <-time.After(2 * time.Second):
		t.Fatal("waiter not woken when a connection was returned")
	}
	assert.Equal(t, int32(2), dials.Load())
	assert.Equal(t, 2, f.liveConns())
	f.putConnection(&c2, nil)
}

func TestConnPoolWaitRespectsContext(t *testing.T) {
	f, _ := newLimitedFs(t, 1)
	c1, err := f.getConnection(context.Background(), "share")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = f.getConnection(ctx, "share")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 2*time.Second)
	assert.Equal(t, 1, f.liveConns())
	f.putConnection(&c1, nil)
}

func TestConnPoolWake(t *testing.T) {
	f, _ := newLimitedFs(t, 1)
	c1, err := f.getConnection(context.Background(), "share")
	require.NoError(t, err)

	wake := make(chan struct{}, 1)
	done := make(chan error)
	go func() {
		_, err := f.getConnectionWake(context.Background(), "share", wake)
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	wake <- struct{}{}
	select {
	case err := <-done:
		assert.ErrorIs(t, err, errConnWoken)
	case <-time.After(2 * time.Second):
		t.Fatal("not woken")
	}
	assert.Equal(t, 1, f.liveConns())
	f.putConnection(&c1, nil)
}

func TestConnPoolDialFailureFreesSlot(t *testing.T) {
	ctx := context.Background()
	f, _ := newLimitedFs(t, 1)
	f.dialConn = func(ctx context.Context, share string) (*conn, error) {
		return nil, errors.New("dial failed")
	}
	f.pacer = fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(time.Millisecond), pacer.MaxSleep(time.Millisecond)))
	f.pacer.SetRetries(1)
	_, err := f.getConnection(ctx, "share")
	assert.Error(t, err)
	assert.Equal(t, 0, f.liveConns())
}

func TestConnPoolUnlimited(t *testing.T) {
	ctx := context.Background()
	f, dials := newLimitedFs(t, 0)
	var conns []*conn
	for range 30 {
		c, err := f.getConnection(ctx, "share")
		require.NoError(t, err)
		conns = append(conns, c)
	}
	assert.Equal(t, int32(30), dials.Load())
	for i := range conns {
		f.putConnection(&conns[i], nil)
	}
}

func TestConnPoolNeverExceedsLimit(t *testing.T) {
	ctx := context.Background()
	const max = 3
	f, dials := newLimitedFs(t, max)
	var inUse, peak atomic.Int32
	var wg sync.WaitGroup
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := f.getConnection(ctx, "share")
			if !assert.NoError(t, err) {
				return
			}
			n := inUse.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			inUse.Add(-1)
			f.putConnection(&c, nil)
		}()
	}
	wg.Wait()
	assert.LessOrEqual(t, peak.Load(), int32(max))
	assert.LessOrEqual(t, dials.Load(), int32(max))
	assert.Equal(t, max, f.liveConns())
}

// A writer waiting for a connection takes a handle another writer of
// the same file puts back, instead of waiting for the whole limit.
func TestPoolFilesGetWokenByPut(t *testing.T) {
	f, _ := newLimitedFs(t, 1)
	held, err := f.getConnection(context.Background(), "share")
	require.NoError(t, err)

	p := f.newPoolFiles(context.Background(), "share", "file")
	got := make(chan *file)
	go func() {
		fl, err := p.Get()
		assert.NoError(t, err)
		got <- fl
	}()
	time.Sleep(150 * time.Millisecond)
	handle := &file{c: held}
	p.Put(handle, nil)
	select {
	case fl := <-got:
		assert.Same(t, handle, fl)
	case <-time.After(2 * time.Second):
		t.Fatal("filepool.Pool.Get not woken by put")
	}
	assert.Equal(t, 1, f.liveConns())
}

// The limit follows the live config, so rc options/set applies.
func TestConnPoolLimitFollowsConfig(t *testing.T) {
	ctx := context.Background()
	f, _ := newLimitedFs(t, 0)
	f.ci = &fs.ConfigInfo{MaxConnections: 1}
	c1, err := f.getConnection(ctx, "share")
	require.NoError(t, err)

	short, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	_, err = f.getConnection(short, "share")
	cancel()
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	f.ci.MaxConnections = 2
	c2, err := f.getConnection(ctx, "share")
	require.NoError(t, err)
	assert.Equal(t, 2, f.liveConns())
	f.putConnection(&c1, nil)
	f.putConnection(&c2, nil)
}
