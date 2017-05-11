// This tests TimeoutReader

package swift

import (
	"io"
	"io/ioutil"
	"sync"
	"testing"
	"time"
)

// An io.ReadCloser for testing
type testReader struct {
	sync.Mutex
	n      int
	delay  time.Duration
	closed bool
}

// Returns n bytes with at time.Duration delay
func newTestReader(n int, delay time.Duration) *testReader {
	return &testReader{
		n:     n,
		delay: delay,
	}
}

// Returns 1 byte at a time after delay
func (t *testReader) Read(p []byte) (n int, err error) {
	if t.n <= 0 {
		return 0, io.EOF
	}
	time.Sleep(t.delay)
	p[0] = 'A'
	t.Lock()
	t.n--
	t.Unlock()
	return 1, nil
}

// Close the channel
func (t *testReader) Close() error {
	t.Lock()
	t.closed = true
	t.Unlock()
	return nil
}

func TestTimeoutReaderNoTimeout(t *testing.T) {
	test := newTestReader(3, 10*time.Millisecond)
	cancelled := false
	cancel := func() {
		cancelled = true
	}
	tr := newTimeoutReader(test, 100*time.Millisecond, cancel)
	b, err := ioutil.ReadAll(tr)
	if err != nil || string(b) != "AAA" {
		t.Fatalf("Bad read %s %s", err, b)
	}
	if cancelled {
		t.Fatal("Cancelled when shouldn't have been")
	}
	if test.n != 0 {
		t.Fatal("Didn't read all")
	}
	if test.closed {
		t.Fatal("Shouldn't be closed")
	}
	tr.Close()
	if !test.closed {
		t.Fatal("Should be closed")
	}
}

func TestTimeoutReaderTimeout(t *testing.T) {
	// Return those bytes slowly so we get an idle timeout
	test := newTestReader(3, 100*time.Millisecond)
	cancelled := false
	cancel := func() {
		cancelled = true
	}
	tr := newTimeoutReader(test, 10*time.Millisecond, cancel)
	_, err := ioutil.ReadAll(tr)
	if err != TimeoutError {
		t.Fatal("Expecting TimeoutError, got", err)
	}
	if !cancelled {
		t.Fatal("Not cancelled when should have been")
	}
	test.Lock()
	n := test.n
	test.Unlock()
	if n == 0 {
		t.Fatal("Read all")
	}
	if n != 3 {
		t.Fatal("Didn't read any")
	}
	if test.closed {
		t.Fatal("Shouldn't be closed")
	}
	tr.Close()
	if !test.closed {
		t.Fatal("Should be closed")
	}
}
