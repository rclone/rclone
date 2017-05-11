package swift

import (
	"io"
	"time"
)

var watchdogChunkSize = 1 << 20 // 1 MiB

// An io.Reader which resets a watchdog timer whenever data is read
type watchdogReader struct {
	timeout   time.Duration
	reader    io.Reader
	timer     *time.Timer
	chunkSize int
}

// Returns a new reader which will kick the watchdog timer whenever data is read
func newWatchdogReader(reader io.Reader, timeout time.Duration, timer *time.Timer) *watchdogReader {
	return &watchdogReader{
		timeout:   timeout,
		reader:    reader,
		timer:     timer,
		chunkSize: watchdogChunkSize,
	}
}

// Read reads up to len(p) bytes into p
func (t *watchdogReader) Read(p []byte) (int, error) {
	//read from underlying reader in chunks not larger than t.chunkSize
	//while resetting the watchdog timer before every read; the small chunk
	//size ensures that the timer does not fire when reading a large amount of
	//data from a slow connection
	start := 0
	end := len(p)
	for start < end {
		length := end - start
		if length > t.chunkSize {
			length = t.chunkSize
		}

		resetTimer(t.timer, t.timeout)
		n, err := t.reader.Read(p[start : start+length])
		start += n
		if n == 0 || err != nil {
			return start, err
		}
	}

	resetTimer(t.timer, t.timeout)
	return start, nil
}

// Check it satisfies the interface
var _ io.Reader = &watchdogReader{}
