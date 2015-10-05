// Accounting and limiting reader

package fs

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

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

// stringSet holds a set of strings
type stringSet map[string]struct{}

// inProgress holds a synchronizes map of in progress transfers
type inProgress struct {
	mu sync.Mutex
	m  map[string]*Account
}

// newInProgress makes a new inProgress object
func newInProgress() *inProgress {
	return &inProgress{
		m: make(map[string]*Account, Config.Transfers),
	}
}

// set marks the name as in progress
func (ip *inProgress) set(name string, acc *Account) {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	ip.m[name] = acc
}

// clear marks the name as no longer in progress
func (ip *inProgress) clear(name string) {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	delete(ip.m, name)
}

// get gets the account for name, of nil if not found
func (ip *inProgress) get(name string) *Account {
	ip.mu.Lock()
	defer ip.mu.Unlock()
	return ip.m[name]
}

// Strings returns all the strings in the stringSet
func (ss stringSet) Strings() []string {
	strings := make([]string, 0, len(ss))
	for name := range ss {
		var out string
		if acc := Stats.inProgress.get(name); acc != nil {
			out = acc.String()
		} else {
			out = name
		}
		strings = append(strings, " * "+out)
	}
	sorted := sort.StringSlice(strings)
	sorted.Sort()
	return sorted
}

// String returns all the file names in the stringSet joined by newline
func (ss stringSet) String() string {
	return strings.Join(ss.Strings(), "\n")
}

// StatsInfo limits and accounts all transfers
type StatsInfo struct {
	lock         sync.RWMutex
	bytes        int64
	errors       int64
	checks       int64
	checking     stringSet
	transfers    int64
	transferring stringSet
	start        time.Time
	inProgress   *inProgress
}

// NewStats cretates an initialised StatsInfo
func NewStats() *StatsInfo {
	return &StatsInfo{
		checking:     make(stringSet, Config.Checkers),
		transferring: make(stringSet, Config.Transfers),
		start:        time.Now(),
		inProgress:   newInProgress(),
	}
}

// String convert the StatsInfo to a string for printing
func (s *StatsInfo) String() string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	dt := time.Now().Sub(s.start)
	dtSeconds := dt.Seconds()
	speed := 0.0
	if dt > 0 {
		speed = float64(s.bytes) / 1024 / dtSeconds
	}
	dtRounded := dt - (dt % (time.Second / 10))
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
		dtRounded)
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
	s.errors++
}

// Checking adds a check into the stats
func (s *StatsInfo) Checking(o Object) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.checking[o.Remote()] = struct{}{}
}

// DoneChecking removes a check from the stats
func (s *StatsInfo) DoneChecking(o Object) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.checking, o.Remote())
	s.checks++
}

// GetTransfers reads the number of transfers
func (s *StatsInfo) GetTransfers() int64 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.transfers
}

// Transferring adds a transfer into the stats
func (s *StatsInfo) Transferring(o Object) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.transferring[o.Remote()] = struct{}{}
}

// DoneTransferring removes a transfer from the stats
func (s *StatsInfo) DoneTransferring(o Object) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.transferring, o.Remote())
	s.transfers++
}

// Account limits and accounts for one transfer
type Account struct {
	// The mutex is to make sure Read() and Close() aren't called
	// concurrently.  Unfortunately the persistent connection loop
	// in http transport calls Read() after Do() returns on
	// CancelRequest so this race can happen when it apparently
	// shouldn't.
	mu      sync.Mutex
	in      io.ReadCloser
	size    int64
	name    string
	statmu  sync.Mutex         // Separate mutex for stat values.
	bytes   int64              // Total number of bytes read
	start   time.Time          // Start time of first read
	lpTime  time.Time          // Time of last average measurement
	lpBytes int                // Number of bytes read since last measurement
	avg     ewma.MovingAverage // Moving average of last few measurements
	closed  bool               // set if the file is closed
	exit    chan struct{}      // channel that will be closed when transfer is finished
}

// NewAccount makes a Account reader for an object
func NewAccount(in io.ReadCloser, obj Object) *Account {
	acc := &Account{
		in:     in,
		size:   obj.Size(),
		name:   obj.Remote(),
		exit:   make(chan struct{}),
		avg:    ewma.NewMovingAverage(),
		lpTime: time.Now(),
	}
	go acc.averageLoop()
	Stats.inProgress.set(acc.name, acc)
	return acc
}

func (file *Account) averageLoop() {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case now := <-tick.C:
			file.statmu.Lock()
			// Add average of last second.
			elapsed := now.Sub(file.lpTime).Seconds()
			avg := float64(file.lpBytes) / elapsed
			file.avg.Add(avg)
			file.lpBytes = 0
			file.lpTime = now
			// Unlock stats
			file.statmu.Unlock()
		case <-file.exit:
			return
		}
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
	}
	file.statmu.Unlock()

	n, err = file.in.Read(p)

	// Update Stats
	file.statmu.Lock()
	file.lpBytes += n
	file.bytes += int64(n)
	file.statmu.Unlock()

	Stats.Bytes(int64(n))

	// Limit the transfer speed if required
	if tokenBucket != nil {
		tokenBucket.Wait(int64(n))
	}
	return
}

// Progress returns bytes read as well as the size.
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
	seconds := float64(left) / file.avg.Value()

	return time.Duration(time.Second * time.Duration(int(seconds))), true
}

// String produces stats for this file
func (file *Account) String() string {
	a, b := file.Progress()
	avg, cur := file.Speed()
	eta, etaok := file.ETA()
	etas := "-"
	if etaok {
		if eta > 0 {
			etas = fmt.Sprintf("%v", eta)
		} else {
			etas = "0s"
		}
	}
	name := []rune(file.name)
	if len(name) > 45 {
		where := len(name) - 42
		name = append([]rune{'.', '.', '.'}, name[where:]...)
	}
	if b <= 0 {
		return fmt.Sprintf("%45s: avg:%7.1f, cur: %6.1f kByte/s. ETA: %s", string(name), avg/1024, cur/1024, etas)
	}
	return fmt.Sprintf("%45s: %2d%% done. avg: %6.1f, cur: %6.1f kByte/s. ETA: %s", string(name), int(100*float64(a)/float64(b)), avg/1024, cur/1024, etas)
}

// Close the object
func (file *Account) Close() error {
	file.mu.Lock()
	defer file.mu.Unlock()
	if file.closed {
		return nil
	}
	file.closed = true
	close(file.exit)
	Stats.inProgress.clear(file.name)
	return file.in.Close()
}

// Check it satisfies the interface
var _ io.ReadCloser = &Account{}
