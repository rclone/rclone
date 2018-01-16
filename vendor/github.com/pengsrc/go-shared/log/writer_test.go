package log

import (
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStandardWriter(t *testing.T) {
	var w io.Writer
	w = &StandardWriter{
		w: os.Stdout, ew: os.Stderr,
		dl: MuteLevel, pid: os.Getpid(),
	}

	lw, ok := w.(LevelWriter)
	assert.True(t, ok)

	_, ok = w.(Flusher)
	assert.True(t, ok)

	_, err := lw.Write([]byte("Hello World!"))
	assert.NoError(t, err)
}

func BenchmarkStandardWriter(b *testing.B) {
	lw := &StandardWriter{w: ioutil.Discard, ew: ioutil.Discard}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lw.Write([]byte("Hello World!"))
	}
}
