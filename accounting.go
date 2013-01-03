// Accounting and limiting reader

package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"
)

// Globals
var (
	stats = NewStats()
)

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
type Stats struct {
	lock         sync.RWMutex
	bytes        int64
	errors       int64
	checks       int64
	checking     StringSet
	transfers    int64
	transferring StringSet
	start        time.Time
}

// NewStats cretates an initialised Stats
func NewStats() *Stats {
	return &Stats{
		checking:     make(StringSet, *checkers),
		transferring: make(StringSet, *transfers),
		start:        time.Now(),
	}
}

// String convert the Stats to a string for printing
func (s *Stats) String() string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	dt := time.Now().Sub(stats.start)
	dt_seconds := dt.Seconds()
	speed := 0.0
	if dt > 0 {
		speed = float64(stats.bytes) / 1024 / dt_seconds
	}
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, `
Transferred:   %10d Bytes (%7.2f kByte/s)
Errors:        %10d
Checks:        %10d
Transferred:   %10d
Elapsed time:  %v
`,
		stats.bytes, speed,
		stats.errors,
		stats.checks,
		stats.transfers,
		dt)
	if len(s.checking) > 0 {
		fmt.Fprintf(buf, "Checking:      %s\n", s.checking)
	}
	if len(s.transferring) > 0 {
		fmt.Fprintf(buf, "Transferring:  %s\n", s.transferring)
	}
	return buf.String()
}

// Log outputs the Stats to the log
func (s *Stats) Log() {
	log.Printf("%v\n", stats)
}

// Bytes updates the stats for bytes bytes
func (s *Stats) Bytes(bytes int64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.bytes += bytes
}

// Errors updates the stats for errors
func (s *Stats) Errors(errors int64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.errors += errors
}

// Error adds a single error into the stats
func (s *Stats) Error() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.errors += 1
}

// Checking adds a check into the stats
func (s *Stats) Checking(fs FsObject) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.checking[fs.Remote()] = true
}

// DoneChecking removes a check from the stats
func (s *Stats) DoneChecking(fs FsObject) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.checking, fs.Remote())
	s.checks += 1
}

// Transferring adds a transfer into the stats
func (s *Stats) Transferring(fs FsObject) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.transferring[fs.Remote()] = true
}

// DoneTransferring removes a transfer from the stats
func (s *Stats) DoneTransferring(fs FsObject) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.transferring, fs.Remote())
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
	stats.Bytes(int64(n))
	if err == io.EOF {
		// FIXME Do something?
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
