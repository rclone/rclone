package log

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExported(t *testing.T) {
	l, err := NewTerminalLogger("DEBUG")
	assert.NoError(t, err)
	SetGlobalLogger(l)

	c := context.Background()

	l.Debug(c, "Singleton logger")
	assert.NotNil(t, InfoEvent(context.Background()))

	Debugf(c, "Debug message test, hi %s", "Apple")
}
