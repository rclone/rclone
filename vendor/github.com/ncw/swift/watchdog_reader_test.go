// This tests WatchdogReader

package swift

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"
	"time"
)

// Uses testReader from timeout_reader_test.go

func testWatchdogReaderTimeout(t *testing.T, initialTimeout, watchdogTimeout time.Duration, expectedTimeout bool) {
	test := newTestReader(3, 10*time.Millisecond)
	timer, firedChan := setupTimer(initialTimeout)
	wr := newWatchdogReader(test, watchdogTimeout, timer)
	b, err := ioutil.ReadAll(wr)
	if err != nil || string(b) != "AAA" {
		t.Fatalf("Bad read %s %s", err, b)
	}
	checkTimer(t, firedChan, expectedTimeout)
}

func setupTimer(initialTimeout time.Duration) (timer *time.Timer, fired <-chan bool) {
	timer = time.NewTimer(initialTimeout)
	firedChan := make(chan bool)
	started := make(chan bool)
	go func() {
		started <- true
		select {
		case <-timer.C:
			firedChan <- true
		}
	}()
	<-started
	return timer, firedChan
}

func checkTimer(t *testing.T, firedChan <-chan bool, expectedTimeout bool) {
	fired := false
	select {
	case fired = <-firedChan:
	default:
	}
	if expectedTimeout {
		if !fired {
			t.Fatal("Timer should have fired")
		}
	} else {
		if fired {
			t.Fatal("Timer should not have fired")
		}
	}
}

func TestWatchdogReaderNoTimeout(t *testing.T) {
	testWatchdogReaderTimeout(t, 100*time.Millisecond, 100*time.Millisecond, false)
}

func TestWatchdogReaderTimeout(t *testing.T) {
	testWatchdogReaderTimeout(t, 5*time.Millisecond, 5*time.Millisecond, true)
}

func TestWatchdogReaderNoTimeoutShortInitial(t *testing.T) {
	testWatchdogReaderTimeout(t, 5*time.Millisecond, 100*time.Millisecond, false)
}

func TestWatchdogReaderTimeoutLongInitial(t *testing.T) {
	testWatchdogReaderTimeout(t, 100*time.Millisecond, 5*time.Millisecond, true)
}

//slowReader simulates reading from a slow network connection by introducing a delay
//in each Read() proportional to the amount of bytes read.
type slowReader struct {
	reader       io.Reader
	delayPerByte time.Duration
}

func (r *slowReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	if n > 0 {
		time.Sleep(time.Duration(n) * r.delayPerByte)
	}
	return
}

//This test verifies that the watchdogReader's timeout is not triggered by data
//that comes in very slowly. (It should only be triggered if no data arrives at
//all.)
func TestWatchdogReaderOnSlowNetwork(t *testing.T) {
	byteString := make([]byte, 8*watchdogChunkSize)
	reader := &slowReader{
		reader: bytes.NewReader(byteString),
		//reading everything at once would take 100 ms, which is longer than the
		//watchdog timeout below
		delayPerByte: 200 * time.Millisecond / time.Duration(len(byteString)),
	}

	timer, firedChan := setupTimer(10 * time.Millisecond)
	wr := newWatchdogReader(reader, 190*time.Millisecond, timer)

	//use io.ReadFull instead of ioutil.ReadAll here because ReadAll already does
	//some chunking that would keep this testcase from failing
	b := make([]byte, len(byteString))
	n, err := io.ReadFull(wr, b)
	if err != nil || n != len(b) || !bytes.Equal(b, byteString) {
		t.Fatal("Bad read %s %d", err, n)
	}

	checkTimer(t, firedChan, false)
}

//This test verifies that the watchdogReader's chunking logic does not mess up
//the byte strings that are read.
func TestWatchdogReaderValidity(t *testing.T) {
	byteString := []byte("abcdefghij")
	//make a reader with a non-standard chunk size (1 MiB would be much too huge
	//to comfortably look at the bytestring that comes out of the reader)
	wr := &watchdogReader{
		reader:    bytes.NewReader(byteString),
		chunkSize: 3, //len(byteString) % chunkSize != 0 to be extra rude :)
		//don't care about the timeout stuff here
		timeout: 5 * time.Minute,
		timer:   time.NewTimer(5 * time.Minute),
	}

	b := make([]byte, len(byteString))
	n, err := io.ReadFull(wr, b)
	if err != nil || n != len(b) {
		t.Fatal("Read error: %s", err)
	}
	if !bytes.Equal(b, byteString) {
		t.Fatal("Bad read: %#v != %#v", string(b), string(byteString))
	}
}
