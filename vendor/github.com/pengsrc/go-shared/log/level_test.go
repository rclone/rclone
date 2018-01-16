package log

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLevel(t *testing.T) {
	var l Level
	var err error
	assert.Equal(t, l, MuteLevel)
	assert.NoError(t, err)

	l, err = ParseLevel("FATAL")
	assert.Equal(t, l, FatalLevel)
	assert.NoError(t, err)
	l, err = ParseLevel("PANIC")
	assert.Equal(t, l, PanicLevel)
	assert.NoError(t, err)
	l, err = ParseLevel("ERROR")
	assert.Equal(t, l, ErrorLevel)
	assert.NoError(t, err)
	l, err = ParseLevel("WARN")
	assert.Equal(t, l, WarnLevel)
	assert.NoError(t, err)
	l, err = ParseLevel("INFO")
	assert.Equal(t, l, InfoLevel)
	assert.NoError(t, err)
	l, err = ParseLevel("DEBUG")
	assert.Equal(t, l, DebugLevel)
	assert.NoError(t, err)

	l, err = ParseLevel("invalid")
	assert.Error(t, err)
}

func TestLevelString(t *testing.T) {
	assert.Equal(t, "FATAL", FatalLevel.String())
	assert.Equal(t, "PANIC", PanicLevel.String())
	assert.Equal(t, "ERROR", ErrorLevel.String())
	assert.Equal(t, "WARN", WarnLevel.String())
	assert.Equal(t, "INFO", InfoLevel.String())
	assert.Equal(t, "DEBUG", DebugLevel.String())
}
