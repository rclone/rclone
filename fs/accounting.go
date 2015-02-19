// Accounting and limiting reader

package fs

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/tsenart/tb"
)

// Globals
var (
	Stats       = NewStats()
	tokenBucket *tb.Bucket
)

// Start the token bucket if necessary
func startTokenBucket() {
	if bwLimit > 0 {
		tokenBucket = tb.NewBucket(int64(bwLimit), 100*time.Millisecond)
		Log(nil, "Starting bandwidth limiter at %vBytes/s", &bwLimit)
	}
}

// Stringset holds some strings
type StringSet map[string]bool

// Strings returns all the strings in the StringSet
func (ss StringSet) Strings() []string {
	strings := make([]string, 0, len(ss))
	for k := range ss {
		strings = append(strings, k)
	}
	return strings
}

// String returns all the strings in the StringSet joined by comma
func (ss StringSet) String() string {
	return strings.Join(ss.Strings(), ", ")
}

// Stats limits and accounts all transfers
type StatsInfo struct {
	lock         sync.RWMutex
	bytes        int64
	errors       int64
	checks       int64
	checking     StringSet
	transfers    int64
	transferring StringSet
	start        time.Time
}

// NewStats cretates an initialised StatsInfo
func NewStats() *StatsInfo {
	return &StatsInfo{
		checking:     make(StringSet, Config.Checkers),
		transferring: make(StringSet, Config.Transfers),
		start:        time.Now(),
	}
}

// String convert the StatsInfo to a string for printing
func (s *StatsInfo) String() string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	dt := time.Now().Sub(s.start)
	dt_seconds := dt.Seconds()
	speed := 0.0
	if dt > 0 {
		speed = float64(s.bytes) / 1024 / dt_seconds
	}
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, `
Transferred:   %10d Bytes (%7.2f kByte/s)
Errors:        %10d
Checks:        %10d
Transferred:   %10d
Elapsed time:  %v
`,
		s.bytes, speed,
		s.errors,
		s.checks,
		s.transfers,
		dt)
	if len(s.checking) > 0 {
		fmt.Fprintf(buf, "Checking:      %s\n", s.checking)
	}
	if len(s.transferring) > 0 {
		fmt.Fprintf(buf, "Transferring:  %s\n", s.transferring)
	}
	return buf.String()
}

// Log outputs the StatsInfo to the log
func (s *StatsInfo) Log() {
	log.Printf("%v\n", s)
}

// Bytes updates the stats for bytes bytes
func (s *StatsInfo) Bytes(bytes int64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.bytes += bytes
}

// Errors updates the stats for errors
func (s *StatsInfo) Errors(errors int64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.errors += errors
}

// GetErrors reads the number of errors
func (s *StatsInfo) GetErrors() int64 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.errors
}

// Errored returns whether there have been any errors
func (s *StatsInfo) Errored() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.errors != 0
}

// Error adds a single error into the stats
func (s *StatsInfo) Error() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.errors += 1
}

// Checking adds a check into the stats
func (s *StatsInfo) Checking(o Object) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.checking[o.Remote()] = true
}

// DoneChecking removes a check from the stats
func (s *StatsInfo) DoneChecking(o Object) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.checking, o.Remote())
	s.checks += 1
}

// Transferring adds a transfer into the stats
func (s *StatsInfo) Transferring(o Object) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.transferring[o.Remote()] = true
}

// DoneTransferring removes a transfer from the stats
func (s *StatsInfo) DoneTransferring(o Object) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.transferring, o.Remote())
	s.transfers += 1
}

// Account limits and accounts for one transfer
type Account struct {
	in    io.ReadCloser
	bytes int64
}

// NewAccount makes a Account reader
func NewAccount(in io.ReadCloser) *Account {
	return &Account{
		in: in,
	}
}

// Read bytes from the object - see io.Reader
func (file *Account) Read(p []byte) (n int, err error) {
	n, err = file.in.Read(p)
	file.bytes += int64(n)
	Stats.Bytes(int64(n))
	if err == io.EOF {
		// FIXME Do something?
	}
	// Limit the transfer speed if required
	if tokenBucket != nil {
		tokenBucket.Wait(int64(n))
	}
	return
}

// Close the object
func (file *Account) Close() error {
	// FIXME do something?
	return file.in.Close()
}

// Check it satisfies the interface
var _ io.ReadCloser = &Account{}
