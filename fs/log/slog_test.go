package log

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"log/slog"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	utcPlusOne = time.FixedZone("UTC+1", 1*60*60)
	t0         = time.Date(2020, 1, 2, 3, 4, 5, 123456000, utcPlusOne)
)

// Test slogLevelToString covers all mapped levels and an unknown level.
func TestSlogLevelToString(t *testing.T) {
	tests := []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{fs.SlogLevelNotice, "NOTICE"},
		{slog.LevelWarn, "WARNING"},
		{slog.LevelError, "ERROR"},
		{fs.SlogLevelCritical, "CRITICAL"},
		{fs.SlogLevelAlert, "ALERT"},
		{fs.SlogLevelEmergency, "EMERGENCY"},
		// Unknown level should fall back to .String()
		{slog.Level(1234), slog.Level(1234).String()},
	}
	for _, tc := range tests {
		got := slogLevelToString(tc.level)
		assert.Equal(t, tc.want, got)
	}
}

// Test mapLogLevelNames replaces only the LevelKey attr and lowercases it.
func TestMapLogLevelNames(t *testing.T) {
	a := slog.Any(slog.LevelKey, slog.LevelWarn)
	mapped := mapLogLevelNames(nil, a)
	val, ok := mapped.Value.Any().(string)
	if !ok || val != "warning" {
		t.Errorf("mapLogLevelNames did not lowercase level: got %v", mapped.Value.Any())
	}
	// non-level attr should remain unchanged
	other := slog.String("foo", "bar")
	out := mapLogLevelNames(nil, other)
	assert.Equal(t, out.Value, other.Value, "mapLogLevelNames changed a non-level attr")
}

// Test getCaller returns a file:line string of the correct form.
func TestGetCaller(t *testing.T) {
	out := getCaller(0)
	assert.NotEqual(t, "", out)
	match := regexp.MustCompile(`^([^:]+):(\d+)$`).FindStringSubmatch(out)
	assert.NotNil(t, match)
	// Can't test this as it skips the /log/ directory!
	// assert.Equal(t, "slog_test.go", match[1])
}

// Test formatStdLogHeader for various flag combinations.
func TestFormatStdLogHeader(t *testing.T) {
	cases := []struct {
		name       string
		format     logFormat
		lineInfo   string
		object     string
		wantPrefix string
	}{
		{"dateTime", logFormatDate | logFormatTime, "", "", "2020/01/02 03:04:05 "},
		{"time", logFormatTime, "", "", "03:04:05 "},
		{"date", logFormatDate, "", "", "2020/01/02 "},
		{"dateTimeUTC", logFormatDate | logFormatTime | logFormatUTC, "", "", "2020/01/02 02:04:05 "},
		{"dateTimeMicro", logFormatDate | logFormatTime | logFormatMicroseconds, "", "", "2020/01/02 03:04:05.123456 "},
		{"micro", logFormatMicroseconds, "", "", "03:04:05.123456 "},
		{"shortFile", logFormatShortFile, "foo.go:10", "03:04:05 ", "foo.go:10: "},
		{"longFile", logFormatLongFile, "foo.go:10", "03:04:05 ", "foo.go:10: "},
		{"timePID", logFormatPid, "", "", fmt.Sprintf("[%d] ", os.Getpid())},
		{"levelObject", 0, "", "myobj", "INFO  : myobj: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &OutputHandler{format: tc.format}
			buf := &bytes.Buffer{}
			h.formatStdLogHeader(buf, slog.LevelInfo, t0, tc.object, tc.lineInfo)
			if !strings.HasPrefix(buf.String(), tc.wantPrefix) {
				t.Errorf("%s: got %q; want prefix %q", tc.name, buf.String(), tc.wantPrefix)
			}
		})
	}
}

// Test Enabled honors the HandlerOptions.Level.
func TestEnabled(t *testing.T) {
	h := NewOutputHandler(&bytes.Buffer{}, nil, 0)
	assert.True(t, h.Enabled(context.Background(), slog.LevelInfo))
	assert.False(t, h.Enabled(context.Background(), slog.LevelDebug))

	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	h2 := NewOutputHandler(&bytes.Buffer{}, opts, 0)
	assert.True(t, h2.Enabled(context.Background(), slog.LevelDebug))
}

// Test clearFormatFlags and setFormatFlags bitwise ops.
func TestClearSetFormatFlags(t *testing.T) {
	h := &OutputHandler{format: logFormatDate | logFormatTime}

	h.clearFormatFlags(logFormatTime)
	assert.True(t, h.format&logFormatTime == 0)

	h.setFormatFlags(logFormatMicroseconds)
	assert.True(t, h.format&logFormatMicroseconds != 0)
}

// Test SetOutput and ResetOutput override the default writer.
func TestSetResetOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	h := NewOutputHandler(buf, nil, 0)
	var gotOverride string
	out := func(_ slog.Level, txt string) {
		gotOverride = txt
	}

	h.SetOutput(out)
	r := slog.NewRecord(t0, slog.LevelInfo, "hello", 0)
	require.NoError(t, h.Handle(context.Background(), r))
	assert.NotEqual(t, "", gotOverride)
	require.Equal(t, "", buf.String())

	h.ResetOutput()
	require.NoError(t, h.Handle(context.Background(), r))
	require.NotEqual(t, "", buf.String())
}

// Test AddOutput sends to extra destinations.
func TestAddOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	h := NewOutputHandler(buf, nil, logFormatDate|logFormatTime)
	var extraText string
	out := func(_ slog.Level, txt string) {
		extraText = txt
	}

	h.AddOutput(false, out)

	r := slog.NewRecord(t0, slog.LevelInfo, "world", 0)
	require.NoError(t, h.Handle(context.Background(), r))
	assert.Equal(t, "2020/01/02 03:04:05 INFO  : world\n", buf.String())
	assert.Equal(t, "2020/01/02 03:04:05 INFO  : world\n", extraText)
}

// Test AddOutputJSON sends JSON to extra destinations.
func TestAddOutputJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	h := NewOutputHandler(buf, nil, logFormatDate|logFormatTime)
	var extraText string
	out := func(_ slog.Level, txt string) {
		extraText = txt
	}

	h.AddOutput(true, out)

	r := slog.NewRecord(t0, slog.LevelInfo, "world", 0)
	require.NoError(t, h.Handle(context.Background(), r))
	assert.NotEqual(t, "", extraText)
	assert.Equal(t, "2020/01/02 03:04:05 INFO  : world\n", buf.String())
	assert.True(t, strings.HasPrefix(extraText, `{"time":"2020-01-02T03:04:05.123456+01:00","level":"info","msg":"world","source":"`))
	assert.True(t, strings.HasSuffix(extraText, "\"}\n"))
}

// Test AddOutputUseJSONLog sends text to extra destinations.
func TestAddOutputUseJSONLog(t *testing.T) {
	buf := &bytes.Buffer{}
	h := NewOutputHandler(buf, nil, logFormatDate|logFormatTime|logFormatJSON)
	var extraText string
	out := func(_ slog.Level, txt string) {
		extraText = txt
	}

	h.AddOutput(false, out)

	r := slog.NewRecord(t0, slog.LevelInfo, "world", 0)
	require.NoError(t, h.Handle(context.Background(), r))
	assert.NotEqual(t, "", extraText)
	assert.True(t, strings.HasPrefix(buf.String(), `{"time":"2020-01-02T03:04:05.123456+01:00","level":"info","msg":"world","source":"`))
	assert.True(t, strings.HasSuffix(buf.String(), "\"}\n"))
	assert.Equal(t, "2020/01/02 03:04:05 INFO  : world\n", extraText)
}

// Test WithAttrs and WithGroup return new handlers with same settings.
func TestWithAttrsAndGroup(t *testing.T) {
	buf := &bytes.Buffer{}
	h := NewOutputHandler(buf, nil, logFormatDate)
	h2 := h.WithAttrs([]slog.Attr{slog.String("k", "v")})
	if _, ok := h2.(*OutputHandler); !ok {
		t.Error("WithAttrs returned wrong type")
	}
	h3 := h.WithGroup("grp")
	if _, ok := h3.(*OutputHandler); !ok {
		t.Error("WithGroup returned wrong type")
	}
}

// Test textLog and jsonLog directly for basic correctness.
func TestTextLogAndJsonLog(t *testing.T) {
	h := NewOutputHandler(&bytes.Buffer{}, nil, logFormatDate|logFormatTime)
	r := slog.NewRecord(t0, slog.LevelWarn, "msg!", 0)
	r.AddAttrs(slog.String("object", "obj"))

	// textLog
	bufText := &bytes.Buffer{}
	require.NoError(t, h.textLog(context.Background(), bufText, r))
	out := bufText.String()
	if !strings.Contains(out, "WARNING") || !strings.Contains(out, "obj:") || !strings.HasSuffix(out, "\n") {
		t.Errorf("textLog output = %q", out)
	}

	// jsonLog
	bufJSON := &bytes.Buffer{}
	require.NoError(t, h.jsonLog(context.Background(), bufJSON, r))
	j := bufJSON.String()
	if !strings.Contains(j, `"level":"warning"`) || !strings.Contains(j, `"msg":"msg!"`) {
		t.Errorf("jsonLog output = %q", j)
	}
}

// Table-driven test for JSON vs text Handle behavior.
func TestHandleFormatFlags(t *testing.T) {
	r := slog.NewRecord(t0, slog.LevelInfo, "hi", 0)
	cases := []struct {
		name     string
		format   logFormat
		wantJSON bool
	}{
		{"textMode", 0, false},
		{"jsonMode", logFormatJSON, true},
	}
	for _, tc := range cases {
		buf := &bytes.Buffer{}
		h := NewOutputHandler(buf, nil, tc.format)
		require.NoError(t, h.Handle(context.Background(), r))
		out := buf.String()
		if tc.wantJSON {
			if !strings.HasPrefix(out, "{") || !strings.Contains(out, `"level":"info"`) {
				t.Errorf("%s: got %q; want JSON", tc.name, out)
			}
		} else {
			if !strings.Contains(out, "INFO") {
				t.Errorf("%s: got %q; want text INFO", tc.name, out)
			}
		}
	}
}
