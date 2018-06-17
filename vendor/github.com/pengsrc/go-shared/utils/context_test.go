package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextKey(t *testing.T) {
	var value ContextKey = "Hello world."

	assert.Equal(t, "Hello world.", string(value))
}
