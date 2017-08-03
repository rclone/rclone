// Accounting and limiting reader

package fs

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VividCortex/ewma"
	"golang.org/x/net/context" // switch to "context" when we stop supporting go1.6
	"golang.org/x/time/rate"
)

// Globals
var (
	Stats           = NewStats()
	tokenBucketMu   sync.Mutex // protects the token bucket variables
	tokenBucket     *rate.Limiter
	prevTokenBucket = tokenBucket
	currLimitMu     sync.Mutex // protects changes to the timeslot
	currLimit       BwTimeSlot
)

const maxBurstSize = 1 * 1024 * 1024 // must be bigger than the biggest request

// make a new empty token bucket with the bandwidth given
func newTokenBucket(bandwidth SizeSuffix) *rate.Limiter {
	tokenBucket = rate.NewLimiter(rate.Limit(bandwidth), maxBurstSize)
	// empty the bucket
	err := tokenBucket.WaitN(context.Background(), maxBurstSize)
	if err != nil {
		Errorf(nil, "Failed to empty token bucket: %v", err)
	}
	return tokenBucket
}

// Start the token bucket if necessary
func startTokenBucket() {
	currLimitMu.Lock()
	currLimit := bwLimit.LimitAt(time.Now())
	currLimitMu.Unlock()

	if currLimit.bandwidth > 0 {
		tokenBucket = newTokenBucket(currLimit.bandwidth)
		Infof(nil, "Starting bandwidth limiter at %vBytes/s", &currLimit.bandwidth)

		// Start the SIGUSR2 signal handler to toggle bandwidth.
		// This function does nothing in windows systems.
		startSignalHandler()
	}
}

// startTokenTicker creates a ticker to update the bandwidth limiter every minute.
func startTokenTicker() {
	// If the timetable has a single entry or was not specified, we don't need
	// a ticker to update the bandwidth.
	if len(bwLimit) <= 1 {
		return
	}

	ticker := time.NewTicker(time.Minute)
	go func() {
		for range ticker.C {
			limitNow := bwLimit.LimitAt(time.Now())
			currLimitMu.Lock()

			if currLimit.bandwidth != limitNow.bandwidth {
				tokenBucketMu.Lock()

				// Set new bandwidth. If unlimited, set tokenbucket to nil.
				if limitNow.bandwidth > 0 {
					tokenBucket = newTokenBucket(limitNow.bandwidth)
					Logf(nil, "Scheduled bandwidth change. Limit set to %vBytes/s", &limitNow.bandwidth)
				} else {
					tokenBucket = nil
					Logf(nil, "Scheduled bandwidth change. Bandwidth limits disabled")
				}

				currLimit = limitNow
				tokenBucketMu.Unlock()
			}
			currLimitMu.Unlock()
		}
	}()
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
		speed = float64(s.bytes) / dtSeconds
	}
	dtRounded := dt - (dt % (time.Second / 10))
	buf := &bytes.Buffer{}

	if Config.DataRateUnit == "bits" {
		speed = speed * 8
	}

	fmt.Fprintf(buf, `
Transferred:   %10s (%s)
Errors:        %10d
Checks:        %10d
Transferred:   %10d
Elapsed time:  %10v
`,
		SizeSuffix(s.bytes).Unit("Bytes"), SizeSuffix(speed).Unit(strings.Title(Config.DataRateUnit)+"/s"),
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
	LogLevelPrintf(Config.StatsLogLevel, nil, "%v\n", s)
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
func (s *StatsInfo) Checking(remote string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.checking[remote] = struct{}{}
}

// DoneChecking removes a check from the stats
func (s *StatsInfo) DoneChecking(remote string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.checking, remote)
	s.checks++
}

// GetTransfers reads the number of transfers
func (s *StatsInfo) GetTransfers() int64 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.transfers
}

// Transferring adds a transfer into the stats
func (s *StatsInfo) Transferring(remote string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.transferring[remote] = struct{}{}
}

// DoneTransferring removes a transfer from the stats
//
// if ok is true then it increments the transfers count
func (s *StatsInfo) DoneTransferring(remote string, ok bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.transferring, remote)
	if ok {
		s.transfers++
	}
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
	origIn  io.ReadCloser
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
	withBuf bool               // is using a buffered in

	wholeFileDisabled bool // disables the whole file when doing parts
}

// NewAccountSizeName makes a Account reader for an io.ReadCloser of
// the given size and name
func NewAccountSizeName(in io.ReadCloser, size int64, name string) *Account {
	acc := &Account{
		in:     in,
		origIn: in,
		size:   size,
		name:   name,
		exit:   make(chan struct{}),
		avg:    ewma.NewMovingAverage(),
		lpTime: time.Now(),
	}
	go acc.averageLoop()
	Stats.inProgress.set(acc.name, acc)
	return acc
}

// NewAccount makes a Account reader for an object
func NewAccount(in io.ReadCloser, obj Object) *Account {
	return NewAccountSizeName(in, obj.Size(), obj.Remote())
}

// WithBuffer - If the file is above a certain size it adds an Async reader
func (acc *Account) WithBuffer() *Account {
	acc.withBuf = true
	var buffers int
	if acc.size >= int64(Config.BufferSize) || acc.size == -1 {
		buffers = int(int64(Config.BufferSize) / asyncBufferSize)
	} else {
		buffers = int(acc.size / asyncBufferSize)
	}
	// On big files add a buffer
	if buffers > 0 {
		in, err := newAsyncReader(acc.in, buffers)
		if err != nil {
			Errorf(acc.name, "Failed to make buffer: %v", err)
		} else {
			acc.in = in
		}
	}
	return acc
}

// GetReader returns the underlying io.ReadCloser
func (acc *Account) GetReader() io.ReadCloser {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	return acc.origIn
}

// StopBuffering stops the async buffer doing any more buffering
func (acc *Account) StopBuffering() {
	if asyncIn, ok := acc.in.(*asyncReader); ok {
		asyncIn.Abandon()
	}
}

// UpdateReader updates the underlying io.ReadCloser
func (acc *Account) UpdateReader(in io.ReadCloser) {
	acc.mu.Lock()
	acc.StopBuffering()
	acc.in = in
	acc.origIn = in
	acc.WithBuffer()
	acc.mu.Unlock()
}

// disableWholeFileAccounting turns off the whole file accounting
func (acc *Account) disableWholeFileAccounting() {
	acc.mu.Lock()
	acc.wholeFileDisabled = true
	acc.mu.Unlock()
}

// accountPart disables the whole file counter and returns an
// io.Reader to wrap a segment of the transfer.
func (acc *Account) accountPart(in io.Reader) io.Reader {
	return newAccountStream(acc, in)
}

func (acc *Account) averageLoop() {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case now := <-tick.C:
			acc.statmu.Lock()
			// Add average of last second.
			elapsed := now.Sub(acc.lpTime).Seconds()
			avg := float64(acc.lpBytes) / elapsed
			acc.avg.Add(avg)
			acc.lpBytes = 0
			acc.lpTime = now
			// Unlock stats
			acc.statmu.Unlock()
		case <-acc.exit:
			return
		}
	}
}

// read bytes from the io.Reader passed in and account them
func (acc *Account) read(in io.Reader, p []byte) (n int, err error) {
	// Set start time.
	acc.statmu.Lock()
	if acc.start.IsZero() {
		acc.start = time.Now()
	}
	acc.statmu.Unlock()

	n, err = in.Read(p)

	// Update Stats
	acc.statmu.Lock()
	acc.lpBytes += n
	acc.bytes += int64(n)
	acc.statmu.Unlock()

	Stats.Bytes(int64(n))

	// Get the token bucket in use
	tokenBucketMu.Lock()

	// Limit the transfer speed if required
	if tokenBucket != nil {
		tbErr := tokenBucket.WaitN(context.Background(), n)
		if tbErr != nil {
			Errorf(nil, "Token bucket error: %v", err)
		}
	}
	tokenBucketMu.Unlock()
	return
}

// Read bytes from the object - see io.Reader
func (acc *Account) Read(p []byte) (n int, err error) {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	if acc.wholeFileDisabled {
		// Don't account
		return acc.in.Read(p)
	}
	return acc.read(acc.in, p)
}

// Progress returns bytes read as well as the size.
// Size can be <= 0 if the size is unknown.
func (acc *Account) Progress() (bytes, size int64) {
	if acc == nil {
		return 0, 0
	}
	acc.statmu.Lock()
	bytes, size = acc.bytes, acc.size
	acc.statmu.Unlock()
	return bytes, size
}

// Speed returns the speed of the current file transfer
// in bytes per second, as well a an exponentially weighted moving average
// If no read has completed yet, 0 is returned for both values.
func (acc *Account) Speed() (bps, current float64) {
	if acc == nil {
		return 0, 0
	}
	acc.statmu.Lock()
	defer acc.statmu.Unlock()
	if acc.bytes == 0 {
		return 0, 0
	}
	// Calculate speed from first read.
	total := float64(time.Now().Sub(acc.start)) / float64(time.Second)
	bps = float64(acc.bytes) / total
	current = acc.avg.Value()
	return
}

// ETA returns the ETA of the current operation,
// rounded to full seconds.
// If the ETA cannot be determined 'ok' returns false.
func (acc *Account) ETA() (eta time.Duration, ok bool) {
	if acc == nil || acc.size <= 0 {
		return 0, false
	}
	acc.statmu.Lock()
	defer acc.statmu.Unlock()
	if acc.bytes == 0 {
		return 0, false
	}
	left := acc.size - acc.bytes
	if left <= 0 {
		return 0, true
	}
	avg := acc.avg.Value()
	if avg <= 0 {
		return 0, false
	}
	seconds := float64(left) / acc.avg.Value()

	return time.Duration(time.Second * time.Duration(int(seconds))), true
}

// String produces stats for this file
func (acc *Account) String() string {
	a, b := acc.Progress()
	_, cur := acc.Speed()
	eta, etaok := acc.ETA()
	etas := "-"
	if etaok {
		if eta > 0 {
			etas = fmt.Sprintf("%v", eta)
		} else {
			etas = "0s"
		}
	}
	name := []rune(acc.name)
	if len(name) > 45 {
		where := len(name) - 42
		name = append([]rune{'.', '.', '.'}, name[where:]...)
	}

	if Config.DataRateUnit == "bits" {
		cur = cur * 8
	}

	done := ""
	if b > 0 {
		done = fmt.Sprintf("%2d%% done, ", int(100*float64(a)/float64(b)))
	}
	return fmt.Sprintf("%45s: %s%s, ETA: %s",
		string(name),
		done,
		SizeSuffix(cur).Unit(strings.Title(Config.DataRateUnit)+"/s"),
		etas,
	)
}

// Close the object
func (acc *Account) Close() error {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	if acc.closed {
		return nil
	}
	acc.closed = true
	close(acc.exit)
	Stats.inProgress.clear(acc.name)
	return acc.in.Close()
}

// accountStream accounts a single io.Reader into a parent *Account
type accountStream struct {
	acc *Account
	in  io.Reader
}

// newAccountStream makes a new accountStream for an in
func newAccountStream(acc *Account, in io.Reader) *accountStream {
	return &accountStream{
		acc: acc,
		in:  in,
	}
}

// Read bytes from the object - see io.Reader
func (a *accountStream) Read(p []byte) (n int, err error) {
	return a.acc.read(a.in, p)
}

// AccountByPart turns off whole file accounting
//
// Returns the current account or nil if not found
func AccountByPart(obj Object) *Account {
	acc := Stats.inProgress.get(obj.Remote())
	if acc == nil {
		Debugf(obj, "Didn't find object to account part transfer")
		return nil
	}
	acc.disableWholeFileAccounting()
	return acc
}

// AccountPart accounts for part of a transfer
//
// It disables the whole file counter and returns an io.Reader to wrap
// a segment of the transfer.
func AccountPart(obj Object, in io.Reader) io.Reader {
	acc := AccountByPart(obj)
	if acc == nil {
		return in
	}
	return acc.accountPart(in)
}

// Check it satisfies the interface
var (
	_ io.ReadCloser = &Account{}
	_ io.Reader     = &accountStream{}
)
