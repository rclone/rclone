package fs

import (
	"bytes"
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
	logSlogWithObject(LogLevelError, withString{}, "message", nil)
	assert.Contains(t, buf.String(), "object=hello")

	// Plain strings are rendered as themselves
	buf.Reset()
	logSlogWithObject(LogLevelError, "potato", "message", nil)
	assert.Contains(t, buf.String(), "object=potato")

	// Anything else shows only its type as it may contain
	// sensitive data such as credentials
	buf.Reset()
	type secrets struct{ Password string }
	logSlogWithObject(LogLevelError, &secrets{Password: "SECRET"}, "message", nil)
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
