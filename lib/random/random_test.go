package random

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	for i := 0; i < 100; i++ {
		assert.Equal(t, i, len(String(i)))
	}
}
