package fs

import (
	"fmt"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// Check it satisfies the interface
var _ pflag.Value = (*LogLevel)(nil)
var _ fmt.Stringer = LogValueItem{}

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
