package log

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sync"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEntry is a decoded log entry from the Buffer.
type testEntry struct {
	Seq   int64  `json:"seq"`
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

// decodeEntries decodes entries checking they are all valid JSON.
func decodeEntries(t *testing.T, entries Entries) (out []testEntry) {
	t.Helper()
	for _, entry := range entries {
		var e testEntry
		require.NoError(t, json.Unmarshal(entry, &e), string(entry))
		out = append(out, e)
	}
	return out
}

// addEntry adds a log entry in the same format as the JSON log to b.
func addEntry(b *Buffer, level slog.Level, msg string) {
	b.add(level, fmt.Sprintf(`{"level":%q,"msg":%q}`+"\n", slogLevelToString(level), msg))
}

func TestBufferDisabled(t *testing.T) {
	b := &Buffer{}
	assert.False(t, b.Enabled())
	addEntry(b, slog.LevelInfo, "dropped")
	assert.Equal(t, int64(0), b.Seq())
	entries, next, lost := b.Get(0, math.MaxInt64, slog.LevelDebug, 0)
	assert.Equal(t, Entries{}, entries)
	assert.Equal(t, int64(0), next)
	assert.Equal(t, int64(0), lost)
}

func TestBufferGet(t *testing.T) {
	b := &Buffer{}
	b.SetSize(1024)
	assert.True(t, b.Enabled())
	for i := range 10 {
		level := slog.LevelInfo
		if i%2 == 1 {
			level = slog.LevelError
		}
		addEntry(b, level, fmt.Sprintf("msg%d", i))
	}
	assert.Equal(t, int64(10), b.Seq())

	for _, test := range []struct {
		name     string
		from, to int64
		level    slog.Level
		limit    int
		wantSeqs []int64
		wantNext int64
		wantLost int64
	}{
		{name: "all", from: 0, to: math.MaxInt64, level: slog.LevelDebug, wantSeqs: []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, wantNext: 10},
		{name: "range", from: 3, to: 6, level: slog.LevelDebug, wantSeqs: []int64{3, 4, 5}, wantNext: 6},
		{name: "level", from: 0, to: math.MaxInt64, level: slog.LevelError, wantSeqs: []int64{1, 3, 5, 7, 9}, wantNext: 10},
		{name: "limit", from: 2, to: math.MaxInt64, level: slog.LevelDebug, limit: 3, wantSeqs: []int64{2, 3, 4}, wantNext: 5},
		{name: "limit and level", from: 0, to: math.MaxInt64, level: slog.LevelError, limit: 2, wantSeqs: []int64{1, 3}, wantNext: 4},
		{name: "negative from", from: -5, to: 2, level: slog.LevelDebug, wantSeqs: []int64{0, 1}, wantNext: 2},
		{name: "from the end", from: 10, to: math.MaxInt64, level: slog.LevelDebug, wantNext: 10},
		{name: "from past the end", from: 100, to: math.MaxInt64, level: slog.LevelDebug, wantNext: 10},
		{name: "to before from", from: 5, to: 2, level: slog.LevelDebug, wantNext: 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			entries, next, lost := b.Get(test.from, test.to, test.level, test.limit)
			var gotSeqs []int64
			for _, e := range decodeEntries(t, entries) {
				gotSeqs = append(gotSeqs, e.Seq)
				assert.Equal(t, fmt.Sprintf("msg%d", e.Seq), e.Msg)
			}
			assert.Equal(t, test.wantSeqs, gotSeqs)
			assert.Equal(t, test.wantNext, next)
			assert.Equal(t, test.wantLost, lost)
		})
	}
}

func TestBufferDropsOldEntries(t *testing.T) {
	b := &Buffer{}
	b.SetSize(1024)
	for i := range 100 {
		addEntry(b, slog.LevelInfo, fmt.Sprintf("message number %d", i))
	}
	assert.LessOrEqual(t, b.size, int64(1024))

	entries, next, lost := b.Get(0, math.MaxInt64, slog.LevelDebug, 0)
	decoded := decodeEntries(t, entries)
	require.NotEmpty(t, decoded)
	assert.Greater(t, lost, int64(0))
	assert.Equal(t, lost, decoded[0].Seq, "lost should be the number of entries dropped")
	assert.Equal(t, int64(100), next)
	assert.Equal(t, int64(99), decoded[len(decoded)-1].Seq)
	assert.Equal(t, int(100-lost), len(decoded))

	// The window is entirely in the dropped entries
	entries, next, gotLost := b.Get(1, 3, slog.LevelDebug, 0)
	assert.Empty(t, entries)
	assert.Equal(t, int64(2), gotLost)
	assert.Equal(t, int64(3), next)

	// An entry bigger than the Buffer uses a sequence number but
	// isn't stored and doesn't evict the other entries
	addEntry(b, slog.LevelInfo, string(make([]byte, 2048)))
	assert.Equal(t, int64(101), b.Seq())
	entries, next, lost = b.Get(0, math.MaxInt64, slog.LevelDebug, 0)
	assert.Equal(t, len(decoded), len(entries))
	assert.Equal(t, int64(101), next)
	assert.Equal(t, decoded[0].Seq+1, lost, "dropped entries plus the oversized one")
	entries, next, lost = b.Get(100, math.MaxInt64, slog.LevelDebug, 0)
	assert.Empty(t, entries)
	assert.Equal(t, int64(101), next)
	assert.Equal(t, int64(1), lost)

	// A gap in the middle of the range is counted as lost too
	addEntry(b, slog.LevelInfo, "after the gap")
	entries, next, lost = b.Get(99, math.MaxInt64, slog.LevelDebug, 0)
	assert.Len(t, entries, 2)
	assert.Equal(t, int64(102), next)
	assert.Equal(t, int64(1), lost)

	// Limit stops before the gap
	entries, next, lost = b.Get(99, math.MaxInt64, slog.LevelDebug, 1)
	assert.Len(t, entries, 1)
	assert.Equal(t, int64(101), next)
	assert.Equal(t, int64(1), lost)

	// Disabling discards the entries
	addEntry(b, slog.LevelInfo, "kept")
	b.SetSize(0)
	assert.False(t, b.Enabled())
	entries, _, _ = b.Get(0, math.MaxInt64, slog.LevelDebug, 0)
	assert.Empty(t, entries)
	assert.Equal(t, int64(103), b.Seq(), "sequence numbers carry on")
}

func TestBufferInvalidJSON(t *testing.T) {
	b := &Buffer{}
	b.SetSize(1024)
	for _, text := range []string{
		"not \"JSON\"\n",
		"control chars \a\x00\xff\U0001F600",
		"{",
		"{ }",
		"{}",
	} {
		b.add(slog.LevelInfo, text)
	}
	entries, _, _ := b.Get(0, math.MaxInt64, slog.LevelDebug, 0)
	decoded := decodeEntries(t, entries)
	require.Len(t, decoded, 5)
	assert.Equal(t, `not "JSON"`, decoded[0].Msg)
	assert.Equal(t, "control chars \a\x00\ufffd\U0001F600", decoded[1].Msg)
	assert.Equal(t, "{", decoded[2].Msg)
	assert.Equal(t, "{ }", decoded[3].Msg)
	assert.Equal(t, "{}", decoded[4].Msg)
	// Check they can all be marshalled
	_, err := json.Marshal(entries)
	require.NoError(t, err)
}

func TestEntriesString(t *testing.T) {
	entries := Entries{json.RawMessage(`{"seq":0}`), json.RawMessage(`{"seq":1}`)}
	assert.Equal(t, "2 log entries", entries.String())
	// Check that the entries aren't rendered when logging an rc reply
	assert.Equal(t, "map[_logs:map[entries:2 log entries]]", fmt.Sprintf("%+v", rc.Params{"_logs": rc.Params{"entries": entries}}))
	// but that they marshal as JSON
	out, err := json.Marshal(entries)
	require.NoError(t, err)
	assert.Equal(t, `[{"seq":0},{"seq":1}]`, string(out))
}

// Check setting the option connects and disconnects the buffer
func TestSetBufferSize(t *testing.T) {
	oldOpt := Opt.BufferSize
	defer func() {
		Opt.BufferSize = oldOpt
		setBufferSize()
	}()
	outputs := func() int {
		Handler.mu.Lock()
		defer Handler.mu.Unlock()
		return len(Handler.outputExtra)
	}
	Opt.BufferSize = 0
	setBufferSize()
	assert.False(t, Recent.Enabled())
	start := outputs()
	fs.Errorf(nil, "not buffered")

	Opt.BufferSize = 64 * 1024
	setBufferSize()
	assert.True(t, Recent.Enabled())
	assert.Equal(t, start+1, outputs())
	seq := Recent.Seq()
	fs.Errorf(nil, "buffered")
	entries, _, _ := Recent.Get(seq, math.MaxInt64, slog.LevelDebug, 0)
	decoded := decodeEntries(t, entries)
	require.Len(t, decoded, 1)
	assert.Equal(t, "buffered", decoded[0].Msg)

	// Setting again shouldn't connect again
	setBufferSize()
	assert.Equal(t, start+1, outputs())

	Opt.BufferSize = 0
	setBufferSize()
	assert.False(t, Recent.Enabled())
	assert.Equal(t, start, outputs())
	fs.Errorf(nil, "not buffered")
	entries, _, _ = Recent.Get(0, math.MaxInt64, slog.LevelDebug, 0)
	assert.Empty(t, entries)
}

// Check the Buffer works as an output of the handler.
func TestBufferFromHandler(t *testing.T) {
	b := &Buffer{}
	b.SetSize(64 * 1024)
	h := NewOutputHandler(io.Discard, nil, logFormatDate|logFormatTime)
	h.AddOutput(true, b.add)

	r := slog.NewRecord(t0, fs.SlogLevelNotice, "hello", 0)
	r.AddAttrs(slog.String("object", "file.txt"))
	require.NoError(t, h.Handle(context.Background(), r))
	require.NoError(t, h.Handle(context.Background(), slog.NewRecord(t0, slog.LevelError, "oops", 0)))

	entries, next, lost := b.Get(0, math.MaxInt64, slog.LevelDebug, 0)
	assert.Equal(t, int64(2), next)
	assert.Equal(t, int64(0), lost)
	decoded := decodeEntries(t, entries)
	require.Len(t, decoded, 2)
	assert.Equal(t, testEntry{Seq: 0, Level: "notice", Msg: "hello"}, decoded[0])
	assert.Equal(t, testEntry{Seq: 1, Level: "error", Msg: "oops"}, decoded[1])

	var full map[string]any
	require.NoError(t, json.Unmarshal(entries[0], &full))
	assert.Equal(t, "file.txt", full["object"])
	assert.Contains(t, full, "time")
	assert.Contains(t, full, "source")
}

func TestBufferConcurrency(t *testing.T) {
	b := &Buffer{}
	b.SetSize(4096)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for i := range 500 {
				addEntry(b, slog.LevelInfo, fmt.Sprintf("msg%d", i))
			}
		})
	}
	wg.Go(func() {
		var from int64
		for range 500 {
			var entries []json.RawMessage
			entries, from, _ = b.Get(from, math.MaxInt64, slog.LevelDebug, 10)
			_ = decodeEntries(t, entries)
			_ = b.Seq()
		}
	})
	wg.Wait()
	assert.Equal(t, int64(2000), b.Seq())
}

func TestRcLog(t *testing.T) {
	call := rc.Calls.Get("core/log")
	require.NotNil(t, call)
	ctx := context.Background()

	oldRecent := Recent
	defer func() { Recent = oldRecent }()
	Recent = &Buffer{}

	_, err := call.Fn(ctx, rc.Params{})
	assert.Equal(t, ErrBufferDisabled, err)
	assert.True(t, rc.IsErrParamInvalid(err), err)

	Recent.SetSize(64 * 1024)
	for i := range 10 {
		level := slog.LevelInfo
		if i%2 == 1 {
			level = slog.LevelError
		}
		addEntry(Recent, level, fmt.Sprintf("msg%d", i))
	}

	seqs := func(out rc.Params) (seqs []int64) {
		for _, e := range decodeEntries(t, out["entries"].(Entries)) {
			seqs = append(seqs, e.Seq)
		}
		return seqs
	}

	out, err := call.Fn(ctx, rc.Params{})
	require.NoError(t, err)
	assert.Equal(t, []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, seqs(out))
	assert.Equal(t, int64(10), out["next"])
	assert.Equal(t, int64(0), out["lost"])

	out, err = call.Fn(ctx, rc.Params{"since": 4, "limit": 2})
	require.NoError(t, err)
	assert.Equal(t, []int64{4, 5}, seqs(out))
	assert.Equal(t, int64(6), out["next"])

	out, err = call.Fn(ctx, rc.Params{"since": -3})
	require.NoError(t, err)
	assert.Equal(t, []int64{7, 8, 9}, seqs(out))

	out, err = call.Fn(ctx, rc.Params{"since": -100, "level": "error"})
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 3, 5, 7, 9}, seqs(out))

	_, err = call.Fn(ctx, rc.Params{"level": "potato"})
	require.Error(t, err)
	assert.True(t, rc.IsErrParamInvalid(err), err)

	// since at the end is OK but beyond the end is an error
	out, err = call.Fn(ctx, rc.Params{"since": 10})
	require.NoError(t, err)
	assert.Empty(t, out["entries"])
	assert.Equal(t, int64(10), out["next"])
	_, err = call.Fn(ctx, rc.Params{"since": 11})
	require.Error(t, err)
	assert.True(t, rc.IsErrParamInvalid(err), err)
	assert.Contains(t, err.Error(), "beyond the end")

	// Check the output can be serialized
	out, err = call.Fn(ctx, rc.Params{"since": 9})
	require.NoError(t, err)
	var buf []byte
	buf, err = json.Marshal(out)
	require.NoError(t, err)
	assert.JSONEq(t, `{"entries":[{"seq":9,"level":"ERROR","msg":"msg9"}],"next":10,"lost":0}`, string(buf))
}
