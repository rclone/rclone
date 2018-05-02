package accounting

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
)

var (
	// Stats is global statistics counter
	Stats = NewStats()
)

func init() {
	// Set the function pointer up in fs
	fs.CountError = Stats.Error
}

// StatsInfo accounts all transfers
type StatsInfo struct {
	mu           sync.RWMutex
	bytes        int64
	errors       int64
	lastError    error
	checks       int64
	checking     *stringSet
	transfers    int64
	transferring *stringSet
	deletes      int64
	start        time.Time
	inProgress   *inProgress
}

// NewStats cretates an initialised StatsInfo
func NewStats() *StatsInfo {
	return &StatsInfo{
		checking:     newStringSet(fs.Config.Checkers),
		transferring: newStringSet(fs.Config.Transfers),
		start:        time.Now(),
		inProgress:   newInProgress(),
	}
}

// String convert the StatsInfo to a string for printing
func (s *StatsInfo) String() string {
	s.mu.RLock()

	dt := time.Now().Sub(s.start)
	dtSeconds := dt.Seconds()
	speed := 0.0
	if dt > 0 {
		speed = float64(s.bytes) / dtSeconds
	}
	dtRounded := dt - (dt % (time.Second / 10))
	buf := &bytes.Buffer{}

	if fs.Config.DataRateUnit == "bits" {
		speed = speed * 8
	}

	fmt.Fprintf(buf, `
Transferred:   %10s (%s)
Errors:        %10d
Checks:        %10d
Transferred:   %10d
Elapsed time:  %10v
`,
		fs.SizeSuffix(s.bytes).Unit("Bytes"), fs.SizeSuffix(speed).Unit(strings.Title(fs.Config.DataRateUnit)+"/s"),
		s.errors,
		s.checks,
		s.transfers,
		dtRounded)

	// checking and transferring have their own locking so unlock
	// here to prevent deadlock on GetBytes
	s.mu.RUnlock()

	if !s.checking.empty() {
		fmt.Fprintf(buf, "Checking:\n%s\n", s.checking)
	}
	if !s.transferring.empty() {
		fmt.Fprintf(buf, "Transferring:\n%s\n", s.transferring)
	}
	return buf.String()
}

// Log outputs the StatsInfo to the log
func (s *StatsInfo) Log() {
	fs.LogLevelPrintf(fs.Config.StatsLogLevel, nil, "%v\n", s)
}

// Bytes updates the stats for bytes bytes
func (s *StatsInfo) Bytes(bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bytes += bytes
}

// GetBytes returns the number of bytes transferred so far
func (s *StatsInfo) GetBytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bytes
}

// Errors updates the stats for errors
func (s *StatsInfo) Errors(errors int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors += errors
}

// GetErrors reads the number of errors
func (s *StatsInfo) GetErrors() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errors
}

// GetLastError returns the lastError
func (s *StatsInfo) GetLastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// Deletes updates the stats for deletes
func (s *StatsInfo) Deletes(deletes int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes += deletes
	return s.deletes
}

// ResetCounters sets the counters (bytes, checks, errors, transfers) to 0
func (s *StatsInfo) ResetCounters() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.bytes = 0
	s.errors = 0
	s.checks = 0
	s.transfers = 0
	s.deletes = 0
}

// ResetErrors sets the errors count to 0
func (s *StatsInfo) ResetErrors() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.errors = 0
}

// Errored returns whether there have been any errors
func (s *StatsInfo) Errored() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errors != 0
}

// Error adds a single error into the stats and assigns lastError
func (s *StatsInfo) Error(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors++
	s.lastError = err
}

// Checking adds a check into the stats
func (s *StatsInfo) Checking(remote string) {
	s.checking.add(remote)
}

// DoneChecking removes a check from the stats
func (s *StatsInfo) DoneChecking(remote string) {
	s.checking.del(remote)
	s.mu.Lock()
	s.checks++
	s.mu.Unlock()
}

// GetTransfers reads the number of transfers
func (s *StatsInfo) GetTransfers() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.transfers
}

// Transferring adds a transfer into the stats
func (s *StatsInfo) Transferring(remote string) {
	s.transferring.add(remote)
}

// DoneTransferring removes a transfer from the stats
//
// if ok is true then it increments the transfers count
func (s *StatsInfo) DoneTransferring(remote string, ok bool) {
	s.transferring.del(remote)
	if ok {
		s.mu.Lock()
		s.transfers++
		s.mu.Unlock()
	}
}
