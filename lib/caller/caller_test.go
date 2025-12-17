package caller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPresent(t *testing.T) {
	assert.False(t, Present("NotFound"))
	assert.False(t, Present("TestPresent"))
	f := func() {
		assert.True(t, Present("TestPresent"))
	}
	f()
}

func BenchmarkPresent(b *testing.B) {
	for b.Loop() {
		_ = Present("NotFound")
	}
}

func BenchmarkPresent100(b *testing.B) {
	var fn func(level int)
	fn = func(level int) {
		if level > 0 {
			fn(level - 1)
			return
		}
		for b.Loop() {
			_ = Present("NotFound")
		}

	}
	fn(100)
}
