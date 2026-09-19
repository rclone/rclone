package fs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interfaces
var (
	_ Flagger      = (*LogLevel)(nil)
	_ FlaggerNP    = LogLevel(0)
	_ fmt.Stringer = LogValueItem{}
)

type withString struct{}

func (withString) String() string {
	return "hello"
}

func TestLogValue(t *testing.T) {
	x := LogValue("x", 1)
	assert.Equal(t, "1", x.String())
	x = LogValue("x", withString{})
	assert.Equal(t, "hello", x.String())
	x = LogValueHide("x", withString{})
	assert.Equal(t, "", x.String())
}

func TestLogSlogWithObject(t *testing.T) {
	var buf bytes.Buffer
	oldLogger := logger
	defer func() { logger = oldLogger }()
	SetLogger(slog.NewTextHandler(&buf, nil))

	// Objects with a String method are rendered with it
	logSlogWithObject(context.Background(), LogLevelError, withString{}, "message", nil)
	assert.Contains(t, buf.String(), "object=hello")

	// Plain strings are rendered as themselves
	buf.Reset()
	logSlogWithObject(context.Background(), LogLevelError, "potato", "message", nil)
	assert.Contains(t, buf.String(), "object=potato")

	// Anything else shows only its type as it may contain
	// sensitive data such as credentials
	buf.Reset()
	type secrets struct{ Password string }
	logSlogWithObject(context.Background(), LogLevelError, &secrets{Password: "SECRET"}, "message", nil)
	assert.Contains(t, buf.String(), "object=*fs.secrets")
	assert.NotContains(t, buf.String(), "SECRET")
}

func TestLogLevelString(t *testing.T) {
	for _, test := range []struct {
		in   LogLevel
		want string
	}{
		{LogLevelEmergency, "EMERGENCY"},
		{LogLevelDebug, "DEBUG"},
		{99, "Unknown(99)"},
	} {
		logLevel := test.in
		got := logLevel.String()
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestLogLevelSet(t *testing.T) {
	for _, test := range []struct {
		in   string
		want LogLevel
		err  bool
	}{
		{"EMERGENCY", LogLevelEmergency, false},
		{"DEBUG", LogLevelDebug, false},
		{"Potato", 100, true},
	} {
		logLevel := LogLevel(100)
		err := logLevel.Set(test.in)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, logLevel, test.in)
	}
}

func TestLogLevelUnmarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   string
		want LogLevel
		err  bool
	}{
		{`"EMERGENCY"`, LogLevelEmergency, false},
		{`"DEBUG"`, LogLevelDebug, false},
		{`"Potato"`, 100, true},
		{strconv.Itoa(int(LogLevelEmergency)), LogLevelEmergency, false},
		{strconv.Itoa(int(LogLevelDebug)), LogLevelDebug, false},
		{"Potato", 100, true},
		{`99`, 100, true},
		{`-99`, 100, true},
	} {
		logLevel := LogLevel(100)
		err := json.Unmarshal([]byte(test.in), &logLevel)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, logLevel, test.in)
	}
}

type testGroupKey struct{}

// ctxHandler records the contexts it was called with
type ctxHandler struct {
	slog.Handler
	ctxs []context.Context
}

func (h *ctxHandler) Handle(ctx context.Context, r slog.Record) error {
	h.ctxs = append(h.ctxs, ctx)
	return h.Handler.Handle(ctx, r)
}

// The context aware log functions pass their context to the log
// handler which uses it to attribute the log.
func TestLogCtx(t *testing.T) {
	var buf bytes.Buffer
	oldLogger := logger
	defer func() { logger = oldLogger }()
	h := &ctxHandler{Handler: slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})}
	SetLogger(h)
	ci := GetConfig(context.Background())
	oldLogLevel := ci.LogLevel
	ci.LogLevel = LogLevelDebug
	defer func() { ci.LogLevel = oldLogLevel }()

	groupCtx := context.WithValue(context.Background(), testGroupKey{}, "job/1")
	check := func(want string) {
		t.Helper()
		var got struct {
			Level  string `json:"level"`
			Msg    string `json:"msg"`
			Object string `json:"object"`
			Size   int    `json:"size"`
		}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &got), buf.String())
		assert.Equal(t, want, fmt.Sprintf("%s|%s|%s|%d", got.Level, got.Msg, got.Object, got.Size))
		buf.Reset()
	}

	ErrorfCtx(groupCtx, "obj", "error %d", 1)
	check("ERROR|error 1|obj|0")
	LogfCtx(groupCtx, nil, "notice")
	check("INFO+2|notice||0")
	InfofCtx(groupCtx, nil, "info")
	check("INFO|info||0")
	DebugfCtx(groupCtx, nil, "debug")
	check("DEBUG|debug||0")
	LogPrintfCtx(groupCtx, LogLevelInfo, nil, "size %v", LogValue("size", 3))
	check("INFO|size 3||3")

	// All of those were passed the context they were called with
	require.Len(t, h.ctxs, 5)
	for _, ctx := range h.ctxs {
		assert.Equal(t, "job/1", ctx.Value(testGroupKey{}))
	}

	// The non context functions and a nil context log with a
	// context which isn't attributed to anything
	h.ctxs = nil
	Errorf(nil, "plain")
	check("ERROR|plain||0")
	ErrorfCtx(nil, nil, "nil ctx") //nolint:staticcheck // deliberately testing a nil context
	check("ERROR|nil ctx||0")
	require.Len(t, h.ctxs, 2)
	for _, ctx := range h.ctxs {
		require.NotNil(t, ctx)
		assert.Nil(t, ctx.Value(testGroupKey{}))
	}

	// Logs below the log level are dropped
	ci.LogLevel = LogLevelNotice
	InfofCtx(groupCtx, nil, "dropped")
	assert.Equal(t, "", buf.String())
}
