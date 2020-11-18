package random

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringLength(t *testing.T) {
	for i := 0; i < 100; i++ {
		s := String(i)
		assert.Equal(t, i, len(s))
	}
}

func TestStringDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		s := String(8)
		assert.False(t, seen[s])
		assert.Equal(t, 8, len(s))
		seen[s] = true
	}
}

func TestPasswordLength(t *testing.T) {
	for i := 0; i <= 128; i++ {
		s, err := Password(i)
		require.NoError(t, err)
		// expected length is number of bytes rounded up
		expected := i / 8
		if i%8 != 0 {
			expected++
		}
		// then converted to base 64
		expected = (expected*8 + 5) / 6
		assert.Equal(t, expected, len(s), i)
	}
}

func TestPasswordDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		s, err := Password(64)
		require.NoError(t, err)
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestSeed(t *testing.T) {
	// seed 100 times and check the first random number doesn't repeat
	// This test could fail with a probability of ~ 10**-15
	const n = 100
	var seen = map[int64]bool{}
	for i := 0; i < n; i++ {
		assert.NoError(t, Seed())
		first := rand.Int63()
		assert.False(t, seen[first])
		seen[first] = true
	}
}
