package swift

import (
	"io"
	"time"
)

// An io.ReadCloser which obeys an idle timeout
type timeoutReader struct {
	reader  io.ReadCloser
	timeout time.Duration
	cancel  func()
}

// Returns a wrapper around the reader which obeys an idle
// timeout. The cancel function is called if the timeout happens
func newTimeoutReader(reader io.ReadCloser, timeout time.Duration, cancel func()) *timeoutReader {
	return &timeoutReader{
		reader:  reader,
		timeout: timeout,
		cancel:  cancel,
	}
}

// Read reads up to len(p) bytes into p
//
// Waits at most for timeout for the read to complete otherwise returns a timeout
func (t *timeoutReader) Read(p []byte) (int, error) {
	// FIXME limit the amount of data read in one chunk so as to not exceed the timeout?
	// Do the read in the background
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		n, err := t.reader.Read(p)
		done <- result{n, err}
	}()
	// Wait for the read or the timeout
	timer := time.NewTimer(t.timeout)
	defer timer.Stop()
	select {
	case r := <-done:
		return r.n, r.err
	case <-timer.C:
		t.cancel()
		return 0, TimeoutError
	}
	panic("unreachable") // for Go 1.0
}

// Close the channel
func (t *timeoutReader) Close() error {
	return t.reader.Close()
}

// Check it satisfies the interface
var _ io.ReadCloser = &timeoutReader{}
