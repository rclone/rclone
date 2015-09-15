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
	"sort"

	"github.com/VividCortex/ewma"
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
type StringSet map[string]Tracker

// Strings returns all the strings in the StringSet
func (ss StringSet) Strings() []string {
	strings := make([]string, 0, len(ss))
	for _, v := range ss {
		strings = append(strings, " * "+v.String())
	}
	sorted := sort.StringSlice(strings)
	sorted.Sort()
	return sorted
}

// String returns all the strings in the StringSet joined by comma
func (ss StringSet) String() string {
	return strings.Join(ss.Strings(), "\n")
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
	dt_rounded := dt - (dt % (time.Second / 10))
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, `
Transferred:   %10d Bytes (%7.2f kByte/s)
Errors:        %10d
Checks:        %10d
Transferred:   %10d
Elapsed time:  %10v
`,
		s.bytes, speed,
		s.errors,
		s.checks,
		s.transfers,
		dt_rounded)
	if len(s.checking) > 0 {
		fmt.Fprintf(buf, "Checking:\n%s\n", s.checking)
	}
	if len(s.transferring) > 0 {
		fmt.Fprintf(buf, "Transferring:\n%s\n", s.transferring)
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

// ResetCounters sets the counters (bytes, checks, errors, transfers) to 0
func (s *StatsInfo) ResetCounters() {
	s.lock.RLock()
	defer s.lock.RUnlock()
	s.bytes = 0
	s.errors = 0
	s.checks = 0
	s.transfers = 0
}

// ResetErrors sets the errors count to 0
func (s *StatsInfo) ResetErrors() {
	s.lock.RLock()
	defer s.lock.RUnlock()
	s.errors = 0
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
func (s *StatsInfo) Checking(p Tracker) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.checking[p.Remote()] = p
}

// DoneChecking removes a check from the stats
func (s *StatsInfo) DoneChecking(p Tracker) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.checking, p.Remote())
	s.checks += 1
}

// GetTransfers reads the number of transfers
func (s *StatsInfo) GetTransfers() int64 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.transfers
}

// Transferring adds a transfer into the stats
func (s *StatsInfo) Transferring(p Tracker) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.transferring[p.Remote()] = p
}

// DoneTransferring removes a transfer from the stats
func (s *StatsInfo) DoneTransferring(p Tracker) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.transferring, p.Remote())
	s.transfers += 1
}

// Account limits and accounts for one transfer
type Account struct {
	// The mutex is to make sure Read() and Close() aren't called
	// concurrently.  Unfortunately the persistent connection loop
	// in http transport calls Read() after Do() returns on
	// CancelRequest so this race can happen when it apparently
	// shouldn't.
	mu     sync.Mutex
	in     io.ReadCloser
	size   int64
	statmu sync.Mutex // Separate mutex for stat values.
	bytes  int64
	start  time.Time
	lpTime time.Time
	lpBytes int
	avg    ewma.MovingAverage
}

// NewAccount makes a Account reader
func NewAccount(in io.ReadCloser) *Account {
	return &Account{
		in: in,
	}
}

// NewAccount makes a Account reader with a specified size
func NewAccountSize(in io.ReadCloser, size int64) *Account {
	return &Account{
		in:   in,
		size: size,
	}
}

// Read bytes from the object - see io.Reader
func (file *Account) Read(p []byte) (n int, err error) {
	file.mu.Lock()
	defer file.mu.Unlock()

	// Set start time.
	file.statmu.Lock()
	if file.start.IsZero() {
		file.start = time.Now()
		file.lpTime = time.Now()
		file.avg = ewma.NewMovingAverage()
	}
	file.statmu.Unlock()

	n, err = file.in.Read(p)

	// Update Stats
	file.statmu.Lock()
	file.lpBytes += n
	elapsed := time.Now().Sub(file.lpTime)

	// We only update the moving average every second, otherwise
	// the variance is too big.
	if elapsed > time.Second {
		avg := float64(file.lpBytes) / (float64(elapsed) / float64(time.Second))
		file.avg.Add(avg)
		file.lpBytes = 0
		file.lpTime = time.Now()
	}

	file.bytes += int64(n)
	file.statmu.Unlock()

	Stats.Bytes(int64(n))

	// Limit the transfer speed if required
	if tokenBucket != nil {
		tokenBucket.Wait(int64(n))
	}
	return
}

// Returns bytes read as well as the size.
// Size can be <= 0 if the size is unknown.
func (file *Account) Progress() (bytes, size int64) {
	if file == nil {
		return 0, 0
	}
	file.statmu.Lock()
	if bytes > size {
		size = 0
	}
	defer file.statmu.Unlock()
	return file.bytes, file.size
}

// Speed returns the speed of the current file transfer
// in bytes per second, as well a an exponentially weighted moving average
// If no read has completed yet, 0 is returned for both values.
func (file *Account) Speed() (bps, current float64) {
	if file == nil {
		return 0, 0
	}
	file.statmu.Lock()
	defer file.statmu.Unlock()
	if file.bytes == 0 {
		return 0, 0
	}
	// Calculate speed from first read.
	total := float64(time.Now().Sub(file.start)) / float64(time.Second)
	bps = float64(file.bytes) / total
	current = file.avg.Value()
	return
}

// ETA returns the ETA of the current operation,
// rounded to full seconds.
// If the ETA cannot be determined 'ok' returns false.
func (file *Account) ETA() (eta time.Duration, ok bool) {
	if file == nil || file.size <= 0 {
		return 0, false
	}
	file.statmu.Lock()
	defer file.statmu.Unlock()
	if file.bytes == 0 {
		return 0, false
	}
	left := file.size - file.bytes
	if left <= 0 {
		return 0, true
	}
	avg := file.avg.Value()
	if avg <= 0 {
		return 0, false
	}
	seconds := float64(left)/file.avg.Value()

	return time.Duration(time.Second * time.Duration(int(seconds))), true
}

// Close the object
func (file *Account) Close() error {
	file.mu.Lock()
	defer file.mu.Unlock()
	// FIXME do something?
	return file.in.Close()
}

type tracker struct {
	Object
	mu  sync.Mutex
	acc *Account
}

func NewTracker(o Object) Tracker {
	return &tracker{Object: o}
}

func (t *tracker) SetAccount(a *Account) {
	t.mu.Lock()
	t.acc = a
	t.mu.Unlock()
}

func (t *tracker) GetAccount() *Account {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.acc
}

func (t *tracker) String() string {
	acc := t.GetAccount()
	if acc == nil {
		return t.Remote()
	}
	a, b := acc.Progress()
	avg, cur := acc.Speed()
	eta, etaok := acc.ETA()
	etas := "-"
	if etaok {
		if eta > 0 {
			etas = fmt.Sprintf("%v", eta)
		} else {
			etas = "0s"
		}
	}
	name := []rune(t.Remote())
	if len(name) > 25 {
			name = name[:25]
	}
	if b <= 0 {
		return fmt.Sprintf("%s: avg:%7.1f, cur: %6.1f kByte/s. ETA: %s", string(name) , avg/1024, cur/1024, etas)
	}
	return fmt.Sprintf("%s: %2d%% done. avg: %6.1f, cur: %6.1f kByte/s. ETA: %s", string(name), int(100*float64(a)/float64(b)), avg/1024, cur/1024, etas)
}

// A Tracker interface includes an
// Object with an optional Account object
// attached for tracking stats.
type Tracker interface {
	Object
	SetAccount(*Account)
}

// Check it satisfies the interface
var _ io.ReadCloser = &Account{}
