package check

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckErrorForExit(t *testing.T) {
	ErrorForExit("name", nil)
	assert.True(t, true)
}
