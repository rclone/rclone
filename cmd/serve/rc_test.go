package serve

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyServer struct {
	addr           *net.TCPAddr
	shutdownCh     chan struct{}
	shutdownCalled bool
}

func (d *dummyServer) Addr() net.Addr {
	return d.addr
}

func (d *dummyServer) Shutdown() error {
	d.shutdownCalled = true
	close(d.shutdownCh)
	return nil
}

func (d *dummyServer) Serve() error {
	<-d.shutdownCh
	return nil
}

func newServer(ctx context.Context, f fs.Fs, in rc.Params) (Handle, error) {
	return &dummyServer{
		addr: &net.TCPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: 8080,
		},
		shutdownCh: make(chan struct{}),
	}, nil
}

func newServerError(ctx context.Context, f fs.Fs, in rc.Params) (Handle, error) {
	return nil, errors.New("serve error")
}

func newServerImmediateStop(ctx context.Context, f fs.Fs, in rc.Params) (Handle, error) {
	h, _ := newServer(ctx, f, in)
	close(h.(*dummyServer).shutdownCh)
	return h, nil
}

func resetGlobals() {
	serveMu.Lock()
	defer serveMu.Unlock()
	serveFns = make(map[string]Fn)
	servers = make(map[string]*server)
}

func newTest(t *testing.T) {
	_, err := fs.Find("mockfs")
	if err != nil {
		mockfs.Register()
	}
	resetGlobals()
	t.Cleanup(resetGlobals)
}

func TestRcStartServeType(t *testing.T) {
	newTest(t)
	serveStart := rc.Calls.Get("serve/start")

	in := rc.Params{"fs": ":mockfs:", "type": "nonexistent"}
	_, err := serveStart.Fn(context.Background(), in)
	assert.ErrorContains(t, err, "could not find serve type")
}

func TestRcStartServeFnError(t *testing.T) {
	newTest(t)
	serveStart := rc.Calls.Get("serve/start")

	AddRc("error", newServerError)
	in := rc.Params{"fs": ":mockfs:", "type": "error"}
	_, err := serveStart.Fn(context.Background(), in)
	assert.ErrorContains(t, err, "could not start serve")
}

func TestRcStartImmediateStop(t *testing.T) {
	newTest(t)
	serveStart := rc.Calls.Get("serve/start")

	AddRc("immediate", newServerImmediateStop)
	in := rc.Params{"fs": ":mockfs:", "type": "immediate"}
	_, err := serveStart.Fn(context.Background(), in)
	assert.ErrorContains(t, err, "server stopped immediately")
}

func TestRcStartAndStop(t *testing.T) {
	newTest(t)
	serveStart := rc.Calls.Get("serve/start")
	serveStop := rc.Calls.Get("serve/stop")

	AddRc("dummy", newServer)
	in := rc.Params{"fs": ":mockfs:", "type": "dummy"}

	out, err := serveStart.Fn(context.Background(), in)
	require.NoError(t, err)
	id := out["id"].(string)
	assert.Contains(t, id, "dummy")
	assert.Equal(t, 1, len(servers))

	_, err = serveStop.Fn(context.Background(), rc.Params{"id": id})
	require.NoError(t, err)
	assert.Equal(t, 0, len(servers))
}

func TestRcStopNonexistent(t *testing.T) {
	newTest(t)
	serveStop := rc.Calls.Get("serve/stop")

	_, err := serveStop.Fn(context.Background(), rc.Params{"id": "nonexistent"})
	assert.ErrorContains(t, err, "not found")
}

func TestRcServeTypes(t *testing.T) {
	newTest(t)
	serveTypes := rc.Calls.Get("serve/types")

	AddRc("a", newServer)
	AddRc("c", newServer)
	AddRc("b", newServer)
	out, err := serveTypes.Fn(context.Background(), nil)
	require.NoError(t, err)
	types := out["types"].([]string)
	assert.Equal(t, types, []string{"a", "b", "c"})
}

func TestRcList(t *testing.T) {
	newTest(t)
	serveStart := rc.Calls.Get("serve/start")
	serveList := rc.Calls.Get("serve/list")

	AddRc("dummy", newServer)

	// Start two servers.
	_, err := serveStart.Fn(context.Background(), rc.Params{"fs": ":mockfs:", "type": "dummy"})
	require.NoError(t, err)

	_, err = serveStart.Fn(context.Background(), rc.Params{"fs": ":mockfs:", "type": "dummy"})
	require.NoError(t, err)

	// Check list
	out, err := serveList.Fn(context.Background(), nil)
	require.NoError(t, err)

	list := out["list"].([]*server)
	assert.Equal(t, 2, len(list))
}

func TestRcStopAll(t *testing.T) {
	newTest(t)
	serveStart := rc.Calls.Get("serve/start")
	serveStopAll := rc.Calls.Get("serve/stopall")

	AddRc("dummy", newServer)

	_, err := serveStart.Fn(context.Background(), rc.Params{"fs": ":mockfs:", "type": "dummy"})
	require.NoError(t, err)
	_, err = serveStart.Fn(context.Background(), rc.Params{"fs": ":mockfs:", "type": "dummy"})
	require.NoError(t, err)
	assert.Equal(t, 2, len(servers))

	_, err = serveStopAll.Fn(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, 0, len(servers))
}

type mockServeOptions struct {
	StringOpt string `config:"string_opt"`
	IntOpt    int    `config:"int_opt"`
}

func newMockServer(ctx context.Context, f fs.Fs, in rc.Params) (Handle, error) {
	var opt mockServeOptions
	err := rc.ParseOptions(in, "serveOpt", &opt)
	if err != nil {
		return nil, err
	}
	return &dummyServer{
		addr: &net.TCPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: 8080,
		},
		shutdownCh: make(chan struct{}),
	}, nil
}

func TestRcStartFlatNestedAndUnknownRejection(t *testing.T) {
	newTest(t)
	serveStart := rc.Calls.Get("serve/start")
	serveStop := rc.Calls.Get("serve/stop")

	AddRc("mockserve", newMockServer)

	t.Run("FlatAndNested", func(t *testing.T) {
		in := rc.Params{
			"fs":         ":mockfs:",
			"type":       "mockserve",
			"string_opt": "flat",
			"serveOpt": rc.Params{
				"IntOpt": 42,
			},
		}
		out, err := serveStart.Fn(context.Background(), in)
		require.NoError(t, err)
		id := out["id"].(string)

		// Verify the running server holds a copy of the original parameters
		s := servers[id]
		require.NotNil(t, s)
		assert.Equal(t, "flat", s.Params["string_opt"])
		assert.Equal(t, rc.Params{"IntOpt": 42}, s.Params["serveOpt"])

		_, err = serveStop.Fn(context.Background(), rc.Params{"id": id})
		require.NoError(t, err)
	})

	t.Run("UnknownRejection", func(t *testing.T) {
		in := rc.Params{
			"fs":            ":mockfs:",
			"type":          "mockserve",
			"unknown_param": "leftover",
		}
		_, err := serveStart.Fn(context.Background(), in)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "unknown parameters: unknown_param")
	})
}
