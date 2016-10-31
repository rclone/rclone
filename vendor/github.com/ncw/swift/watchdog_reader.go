package swift

import (
	"io"
	"time"
)

// An io.Reader which resets a watchdog timer whenever data is read
type watchdogReader struct {
	timeout time.Duration
	reader  io.Reader
	timer   *time.Timer
}

// Returns a new reader which will kick the watchdog timer whenever data is read
func newWatchdogReader(reader io.Reader, timeout time.Duration, timer *time.Timer) *watchdogReader {
	return &watchdogReader{
		timeout: timeout,
		reader:  reader,
		timer:   timer,
	}
}

// Read reads up to len(p) bytes into p
func (t *watchdogReader) Read(p []byte) (n int, err error) {
	// FIXME limit the amount of data read in one chunk so as to not exceed the timeout?
	resetTimer(t.timer, t.timeout)
	n, err = t.reader.Read(p)
	resetTimer(t.timer, t.timeout)
	return
}

// Check it satisfies the interface
var _ io.Reader = &watchdogReader{}
