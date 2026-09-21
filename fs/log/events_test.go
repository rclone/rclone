package log

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// frame is a decoded event from the stream.
type frame struct {
	Type  string `json:"type"`
	Seq   int64  `json:"seq"`
	Msg   string `json:"msg"`
	JobID int64  `json:"jobid"`
	Count int64  `json:"count"`
}

// eventServer serves core/events and returns a function to read the
// frames from it one at a time and a function to stop it.
func eventServer(t *testing.T, in rc.Params) (next func() frame, stop func()) {
	call := rc.Calls.Get("core/events")
	require.NotNil(t, call)
	assert.True(t, call.NeedsResponse)
	assert.True(t, call.WritesResponse)

	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := rc.Params{"_response": w}
		for k, v := range in {
			params[k] = v
		}
		_, err := call.Fn(r.Context(), params)
		assert.NoError(t, err)
		close(done)
	}))

	resp, err := http.Get(server.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/x-ndjson", resp.Header.Get("Content-Type"))
	scanner := bufio.NewScanner(resp.Body)

	next = func() frame {
		t.Helper()
		require.True(t, scanner.Scan(), "no more frames: %v", scanner.Err())
		var f frame
		require.NoError(t, json.Unmarshal(scanner.Bytes(), &f), scanner.Text())
		return f
	}
	stop = func() {
		_ = resp.Body.Close()
		server.CloseClientConnections()
		server.Close()
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Error("core/events didn't return after the client went away")
		}
	}
	return next, stop
}

// logAt logs msg via the real log handler so it goes through the
// output to the subscribers.
func logAt(ctx context.Context, level fs.LogLevel, msg string) {
	fs.LogPrintfCtx(ctx, level, nil, "%s", msg)
}

func TestRcEvents(t *testing.T) {
	ctx := context.Background()
	setTestLogLevel(t, fs.LogLevelInfo)

	next, stop := eventServer(t, rc.Params{"heartbeat": 0})
	defer stop()

	start := next()
	assert.Equal(t, "start", start.Type)

	logAt(ctx, fs.LogLevelNotice, "first")
	f := next()
	assert.Equal(t, "log", f.Type)
	assert.Equal(t, "first", f.Msg)
	assert.Equal(t, start.Seq, f.Seq, "the stream starts at the sequence number in the start frame")

	logAt(testAttributionCtx, fs.LogLevelNotice, "attributed")
	f = next()
	assert.Equal(t, "attributed", f.Msg)
	assert.Equal(t, int64(1), f.JobID)

	// Logs below the requested level aren't sent
	logAt(ctx, fs.LogLevelDebug, "debug")
	logAt(ctx, fs.LogLevelNotice, "second")
	f = next()
	assert.Equal(t, "second", f.Msg)
}

func TestRcEventsFilter(t *testing.T) {
	ctx := context.Background()
	setTestLogLevel(t, fs.LogLevelInfo)

	next, stop := eventServer(t, rc.Params{"heartbeat": 0, "jobid": 1})
	defer stop()
	assert.Equal(t, "start", next().Type)

	// Only the logs of job 1 are sent
	logAt(ctx, fs.LogLevelNotice, "unattributed")
	logAt(testAttributionCtx, fs.LogLevelNotice, "job 1")
	f := next()
	assert.Equal(t, "job 1", f.Msg)
	assert.Equal(t, int64(1), f.JobID)
}

func TestRcEventsHeartbeat(t *testing.T) {
	setTestLogLevel(t, fs.LogLevelInfo)
	next, stop := eventServer(t, rc.Params{"heartbeat": 1})
	defer stop()
	assert.Equal(t, "start", next().Type)
	assert.Equal(t, "heartbeat", next().Type)
}

func TestRcEventsSince(t *testing.T) {
	ctx := context.Background()
	setTestLogLevel(t, fs.LogLevelInfo)

	// Needs the log buffer for the replay
	_, err := rc.Calls.Get("core/events").Fn(ctx, rc.Params{"_response": httptest.NewRecorder(), "since": 0})
	assert.Equal(t, ErrBufferDisabled, err)

	oldRecent, oldSize := Recent, Opt.BufferSize
	t.Cleanup(func() {
		Recent, Opt.BufferSize = oldRecent, oldSize
		setBufferSize()
	})
	Recent = newTestBuffer(0)
	Opt.BufferSize = 64 * fs.Kibi
	setBufferSize()

	logAt(ctx, fs.LogLevelNotice, "before the stream")
	since := Recent.Seq() - 1

	next, stop := eventServer(t, rc.Params{"heartbeat": 0, "since": since})
	defer stop()
	assert.Equal(t, "start", next().Type)

	// The entry from before the stream started is replayed first
	f := next()
	assert.Equal(t, "log", f.Type)
	assert.Equal(t, "before the stream", f.Msg)
	assert.Equal(t, since, f.Seq)

	// then the new ones follow with no repeats
	logAt(ctx, fs.LogLevelNotice, "during the stream")
	f = next()
	assert.Equal(t, "during the stream", f.Msg)
	assert.Equal(t, since+1, f.Seq)
}

// Check a subscriber which isn't reading gets told what it missed
func TestRcEventsDropped(t *testing.T) {
	ctx := context.Background()
	setTestLogLevel(t, fs.LogLevelInfo)

	s, unsubscribe := subscribe(Filter{Level: slog.LevelDebug})
	defer unsubscribe()
	for i := range subscriberBacklog + 10 {
		logAt(ctx, fs.LogLevelNotice, fmt.Sprintf("msg%d", i))
	}
	assert.Equal(t, subscriberBacklog, len(s.frames))
	assert.Equal(t, int64(10), s.dropped)

	// Read the backlog - the dropped marker comes after it, where
	// the gap in the stream is, and not before
	var msgs []string
	var dropped int64
	for range subscriberBacklog {
		var f frame
		require.NoError(t, json.Unmarshal(<-s.frames, &f))
		if f.Type == "dropped" {
			dropped = f.Count
		} else {
			msgs = append(msgs, f.Msg)
		}
	}
	assert.Equal(t, int64(0), dropped, "nothing is reported before the backlog is read")
	assert.Equal(t, "msg0", msgs[0])

	// The marker is queued as soon as there is room for it
	logAt(ctx, fs.LogLevelNotice, "after the gap")
	var f frame
	require.NoError(t, json.Unmarshal(<-s.frames, &f))
	assert.Equal(t, "dropped", f.Type)
	assert.Equal(t, int64(10), f.Count)
	require.NoError(t, json.Unmarshal(<-s.frames, &f))
	assert.Equal(t, "after the gap", f.Msg)
	assert.Equal(t, int64(0), s.takeDropped())
}

// Two streams must not write to each other's frames
func TestRcEventsTwoStreams(t *testing.T) {
	ctx := context.Background()
	setTestLogLevel(t, fs.LogLevelInfo)

	next1, stop1 := eventServer(t, rc.Params{"heartbeat": 0})
	defer stop1()
	next2, stop2 := eventServer(t, rc.Params{"heartbeat": 0})
	defer stop2()
	assert.Equal(t, "start", next1().Type)
	assert.Equal(t, "start", next2().Type)

	var wg sync.WaitGroup
	for _, next := range []func() frame{next1, next2} {
		wg.Go(func() {
			for range 100 {
				f := next()
				assert.Equal(t, "log", f.Type)
			}
		})
	}
	for i := range 100 {
		logAt(ctx, fs.LogLevelNotice, fmt.Sprintf("msg%d", i))
	}
	wg.Wait()
}

// Check the log output is only connected while it is needed
func TestSubscribeConnectsOutput(t *testing.T) {
	outputs := func() int {
		Handler.mu.Lock()
		defer Handler.mu.Unlock()
		return len(Handler.outputExtra)
	}
	before := outputs()
	s1, unsubscribe1 := subscribe(Filter{Level: slog.LevelDebug})
	assert.Equal(t, before+1, outputs())
	_, unsubscribe2 := subscribe(Filter{Level: slog.LevelDebug})
	assert.Equal(t, before+1, outputs(), "one output serves all the subscribers")
	unsubscribe1()
	assert.Equal(t, before+1, outputs())
	unsubscribe2()
	assert.Equal(t, before, outputs())
	assert.Empty(t, s1.frames)
}

// setTestLogLevel sets the log level for the duration of the test
func setTestLogLevel(t *testing.T, level fs.LogLevel) {
	ci := fs.GetConfig(context.Background())
	oldLevel := ci.LogLevel
	oldSlogLevel := Handler.SetLevel(fs.LogLevelToSlog(level))
	ci.LogLevel = level
	t.Cleanup(func() {
		ci.LogLevel = oldLevel
		Handler.SetLevel(oldSlogLevel)
	})
}
