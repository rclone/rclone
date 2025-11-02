package files

import (
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubWriter struct {
	n   int
	err error
}

func (s stubWriter) Write(p []byte) (int, error) {
	if s.n > len(p) {
		return len(p), s.err
	}
	return s.n, s.err
}

func TestCountWriter(t *testing.T) {
	t.Parallel()

	t.Run("initial count is zero", func(t *testing.T) {
		cw := NewCountWriter(io.Discard)
		require.Equal(t, uint64(0), cw.Count())
	})

	t.Run("counts bytes with real writes", func(t *testing.T) {
		cw := NewCountWriter(io.Discard)
		n, err := cw.Write([]byte("abcd"))
		require.NoError(t, err)
		require.Equal(t, 4, n)
		assert.Equal(t, uint64(4), cw.Count())

		n, err = cw.Write([]byte("xyz"))
		require.NoError(t, err)
		require.Equal(t, 3, n)
		assert.Equal(t, uint64(7), cw.Count())
	})

	t.Run("nil writer uses io.Discard", func(t *testing.T) {
		cw := NewCountWriter(nil)
		n, err := cw.Write([]byte("ok"))
		require.NoError(t, err)
		require.Equal(t, 2, n)
		assert.Equal(t, uint64(2), cw.Count())
	})

	t.Run("zero-length write does not change count", func(t *testing.T) {
		cw := NewCountWriter(io.Discard)
		n, err := cw.Write(nil)
		require.NoError(t, err)
		require.Equal(t, 0, n)
		assert.Equal(t, uint64(0), cw.Count())
	})

	t.Run("partial write with error counts n and returns error", func(t *testing.T) {
		s := stubWriter{n: 3, err: errors.New("boom")}
		cw := NewCountWriter(s)
		n, err := cw.Write([]byte("abcdef"))
		require.Error(t, err)
		require.Equal(t, 3, n)
		assert.Equal(t, uint64(3), cw.Count())
	})

	t.Run("short successful write counts returned n", func(t *testing.T) {
		s := stubWriter{n: 1}
		cw := NewCountWriter(s)
		n, err := cw.Write([]byte("hi"))
		require.NoError(t, err)
		require.Equal(t, 1, n)
		assert.Equal(t, uint64(1), cw.Count())
	})
}

func TestCountWriterConcurrent(t *testing.T) {
	t.Parallel()

	const (
		goroutines = 32
		loops      = 200
		chunkSize  = 64
	)
	data := make([]byte, chunkSize)

	cw := NewCountWriter(io.Discard)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < loops; i++ {
				n, err := cw.Write(data)
				assert.NoError(t, err)
				assert.Equal(t, chunkSize, n)
			}
		}()
	}
	wg.Wait()

	want := uint64(goroutines * loops * chunkSize)
	assert.Equal(t, want, cw.Count())
}
