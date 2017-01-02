package swift

import (
	"io"
	"time"
)

// An io.Reader which resets a watchdog timer whenever data is read
type watchdogReader struct {
	timeout     time.Duration
	reader      io.Reader
	timer       *time.Timer
	bytes       int64
	replayFirst bool
	readFirst   bool
	first       byte
}

// Returns a new reader which will kick the watchdog timer whenever data is read
//
// It also has a 1 byte buffer which can be reset with the Reset() method.
func newWatchdogReader(reader io.Reader, timeout time.Duration, timer *time.Timer) *watchdogReader {
	return &watchdogReader{
		timeout: timeout,
		reader:  reader,
		timer:   timer,
	}
}

// Read reads up to len(p) bytes into p
func (t *watchdogReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	// FIXME limit the amount of data read in one chunk so as to not exceed the timeout?
	resetTimer(t.timer, t.timeout)
	if t.replayFirst {
		p[0] = t.first
		t.replayFirst = false
		n = 1
		err = nil
	} else {
		n, err = t.reader.Read(p)
		if !t.readFirst && n > 0 {
			t.first = p[0]
			t.readFirst = true
		}
		t.bytes += int64(n)
	}
	resetTimer(t.timer, t.timeout)
	return
}

// Resets the buffer if only 1 byte read
//
// Returns true if successful
func (t *watchdogReader) Reset() bool {
	switch t.bytes {
	case 0:
		t.replayFirst = false
		return true
	case 1:
		t.replayFirst = true
		return true
	default:
		return false
	}
}

// Check it satisfies the interface
var _ io.Reader = &watchdogReader{}
