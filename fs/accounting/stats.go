package accounting

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/fserrors"
	"github.com/ncw/rclone/fs/rc"
)

// StatsInfo accounts all transfers
type StatsInfo struct {
	mu                sync.RWMutex
	bytes             int64
	errors            int64
	lastError         error
	fatalError        bool
	retryError        bool
	retryAfter        time.Time
	checks            int64
	checking          *stringSet
	checkQueue        int
	checkQueueSize    int64
	transfers         int64
	transferring      *stringSet
	transferQueue     int
	transferQueueSize int64
	renameQueue       int
	renameQueueSize   int64
	deletes           int64
	inProgress        *inProgress
	startedTransfers  []*Transfer
}

// NewStats creates an initialised StatsInfo
func NewStats() *StatsInfo {
	return &StatsInfo{
		checking:     newStringSet(fs.Config.Checkers, "checking"),
		transferring: newStringSet(fs.Config.Transfers, "transferring"),
		inProgress:   newInProgress(),
	}
}

// RemoteStats returns stats for rc
func (s *StatsInfo) RemoteStats() (out rc.Params, err error) {
	out = make(rc.Params)
	s.mu.RLock()
	dt := s.totalDuration()
	dtSeconds := dt.Seconds()
	speed := 0.0
	if dt > 0 {
		speed = float64(s.bytes) / dtSeconds
	}
	out["speed"] = speed
	out["bytes"] = s.bytes
	out["errors"] = s.errors
	out["fatalError"] = s.fatalError
	out["retryError"] = s.retryError
	out["checks"] = s.checks
	out["transfers"] = s.transfers
	out["deletes"] = s.deletes
	out["elapsedTime"] = dtSeconds
	s.mu.RUnlock()
	if !s.checking.empty() {
		var c []string
		s.checking.mu.RLock()
		defer s.checking.mu.RUnlock()
		for name := range s.checking.items {
			c = append(c, name)
		}
		out["checking"] = c
	}
	if !s.transferring.empty() {
		var t []interface{}
		s.transferring.mu.RLock()
		defer s.transferring.mu.RUnlock()
		for name := range s.transferring.items {
			if acc := s.inProgress.get(name); acc != nil {
				t = append(t, acc.RemoteStats())
			} else {
				t = append(t, name)
			}
		}
		out["transferring"] = t
	}
	if s.errors > 0 {
		out["lastError"] = s.lastError
	}
	return out, nil
}

type timeRange struct {
	start time.Time
	end   time.Time
}

// Total duration is union of durations of all transfers belonging to this
// object.
// Needs to be protected by mutex.
func (s *StatsInfo) totalDuration() time.Duration {
	now := time.Now()
	// Extract time ranges of all transfers.
	timeRanges := make([]timeRange, len(s.startedTransfers))
	for i := range s.startedTransfers {
		start, end := s.startedTransfers[i].TimeRange()
		if end.IsZero() {
			end = now
		}
		timeRanges[i] = timeRange{start, end}
	}

	// Sort by the starting time.
	sort.Slice(timeRanges, func(i, j int) bool {
		return timeRanges[i].start.Before(timeRanges[j].start)
	})

	// Merge overlaps and add distinctive ranges together for total.
	var total time.Duration
	var i, j = 0, 1
	for i < len(timeRanges) {
		if j < len(timeRanges)-1 {
			if timeRanges[j].start.Before(timeRanges[i].end) {
				if timeRanges[i].end.Before(timeRanges[j].end) {
					timeRanges[i].end = timeRanges[j].end
				}
				j++
				continue
			}
		}
		total += timeRanges[i].end.Sub(timeRanges[i].start)
		i = j
		j++
	}

	return total
}

// eta returns the ETA of the current operation,
// rounded to full seconds.
// If the ETA cannot be determined 'ok' returns false.
func eta(size, total int64, rate float64) (eta time.Duration, ok bool) {
	if total <= 0 || size < 0 || rate <= 0 {
		return 0, false
	}
	remaining := total - size
	if remaining < 0 {
		return 0, false
	}
	seconds := float64(remaining) / rate
	return time.Second * time.Duration(seconds), true
}

// etaString returns the ETA of the current operation,
// rounded to full seconds.
// If the ETA cannot be determined it returns "-"
func etaString(done, total int64, rate float64) string {
	d, ok := eta(done, total, rate)
	if !ok {
		return "-"
	}
	return fs.Duration(d).ReadableString()
}

// percent returns a/b as a percentage rounded to the nearest integer
// as a string
//
// if the percentage is invalid it returns "-"
func percent(a int64, b int64) string {
	if a < 0 || b <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d%%", int(float64(a)*100/float64(b)+0.5))
}

// String convert the StatsInfo to a string for printing
func (s *StatsInfo) String() string {
	// checking and transferring have their own locking so read
	// here before lock to prevent deadlock on GetBytes
	transferring, checking := s.transferring.count(), s.checking.count()
	transferringBytesDone, transferringBytesTotal := s.transferring.progress(s)

	s.mu.RLock()

	dt := s.totalDuration()
	dtSeconds := dt.Seconds()
	speed := 0.0
	if dt > 0 {
		speed = float64(s.bytes) / dtSeconds
	}
	dtRounded := dt - (dt % (time.Second / 10))

	displaySpeed := speed
	if fs.Config.DataRateUnit == "bits" {
		displaySpeed *= 8
	}

	var (
		totalChecks   = int64(s.checkQueue) + s.checks + int64(checking)
		totalTransfer = int64(s.transferQueue) + s.transfers + int64(transferring)
		// note that s.bytes already includes transferringBytesDone so
		// we take it off here to avoid double counting
		totalSize    = s.transferQueueSize + s.bytes + transferringBytesTotal - transferringBytesDone
		currentSize  = s.bytes
		buf          = &bytes.Buffer{}
		xfrchkString = ""
		dateString   = ""
	)

	if !fs.Config.StatsOneLine {
		_, _ = fmt.Fprintf(buf, "\nTransferred:   	")
	} else {
		xfrchk := []string{}
		if totalTransfer > 0 && s.transferQueue > 0 {
			xfrchk = append(xfrchk, fmt.Sprintf("xfr#%d/%d", s.transfers, totalTransfer))
		}
		if totalChecks > 0 && s.checkQueue > 0 {
			xfrchk = append(xfrchk, fmt.Sprintf("chk#%d/%d", s.checks, totalChecks))
		}
		if len(xfrchk) > 0 {
			xfrchkString = fmt.Sprintf(" (%s)", strings.Join(xfrchk, ", "))
		}
		if fs.Config.StatsOneLineDate {
			t := time.Now()
			dateString = t.Format(fs.Config.StatsOneLineDateFormat) // Including the separator so people can customize it
		}
	}

	_, _ = fmt.Fprintf(buf, "%s%10s / %s, %s, %s, ETA %s%s",
		dateString,
		fs.SizeSuffix(s.bytes),
		fs.SizeSuffix(totalSize).Unit("Bytes"),
		percent(s.bytes, totalSize),
		fs.SizeSuffix(displaySpeed).Unit(strings.Title(fs.Config.DataRateUnit)+"/s"),
		etaString(currentSize, totalSize, speed),
		xfrchkString,
	)

	if !fs.Config.StatsOneLine {
		errorDetails := ""
		switch {
		case s.fatalError:
			errorDetails = " (fatal error encountered)"
		case s.retryError:
			errorDetails = " (retrying may help)"
		case s.errors != 0:
			errorDetails = " (no need to retry)"
		}

		_, _ = fmt.Fprintf(buf, `
Errors:        %10d%s
Checks:        %10d / %d, %s
Transferred:   %10d / %d, %s
Elapsed time:  %10v
`,
			s.errors, errorDetails,
			s.checks, totalChecks, percent(s.checks, totalChecks),
			s.transfers, totalTransfer, percent(s.transfers, totalTransfer),
			dtRounded)
	}

	// checking and transferring have their own locking so unlock
	// here to prevent deadlock on GetBytes
	s.mu.RUnlock()

	// Add per transfer stats if required
	if !fs.Config.StatsOneLine {
		if !s.checking.empty() {
			_, _ = fmt.Fprintf(buf, "Checking:\n%s\n", s.checking.String(s.inProgress))
		}
		if !s.transferring.empty() {
			_, _ = fmt.Fprintf(buf, "Transferring:\n%s\n", s.transferring.String(s.inProgress))
		}
	}

	return buf.String()
}

// Transferred returns list of all completed transfers including checked and
// failed ones.
func (s *StatsInfo) Transferred() []TransferSnapshot {
	ts := make([]TransferSnapshot, 0, len(s.startedTransfers))

	for _, tr := range s.startedTransfers {
		if tr.IsDone() {
			ts = append(ts, tr.Snapshot())
		}
	}

	return ts
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
	s.mu.RLock()
	defer s.mu.RUnlock()
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

// GetChecks returns the number of checks
func (s *StatsInfo) GetChecks() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.checks
}

// FatalError sets the fatalError flag
func (s *StatsInfo) FatalError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fatalError = true
}

// HadFatalError returns whether there has been at least one FatalError
func (s *StatsInfo) HadFatalError() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fatalError
}

// RetryError sets the retryError flag
func (s *StatsInfo) RetryError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retryError = true
}

// HadRetryError returns whether there has been at least one non-NoRetryError
func (s *StatsInfo) HadRetryError() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.retryError
}

// Deletes updates the stats for deletes
func (s *StatsInfo) Deletes(deletes int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes += deletes
	return s.deletes
}

// ResetCounters sets the counters (bytes, checks, errors, transfers, deletes) to 0 and resets lastError, fatalError and retryError
func (s *StatsInfo) ResetCounters() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bytes = 0
	s.errors = 0
	s.lastError = nil
	s.fatalError = false
	s.retryError = false
	s.retryAfter = time.Time{}
	s.checks = 0
	s.transfers = 0
	s.deletes = 0
	s.startedTransfers = nil
}

// ResetErrors sets the errors count to 0 and resets lastError, fatalError and retryError
func (s *StatsInfo) ResetErrors() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = 0
	s.lastError = nil
	s.fatalError = false
	s.retryError = false
	s.retryAfter = time.Time{}
}

// Errored returns whether there have been any errors
func (s *StatsInfo) Errored() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errors != 0
}

// Error adds a single error into the stats, assigns lastError and eventually sets fatalError or retryError
func (s *StatsInfo) Error(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors++
	s.lastError = err
	switch {
	case fserrors.IsFatalError(err):
		s.fatalError = true
	case fserrors.IsRetryAfterError(err):
		retryAfter := fserrors.RetryAfterErrorTime(err)
		if s.retryAfter.IsZero() || retryAfter.Sub(s.retryAfter) > 0 {
			s.retryAfter = retryAfter
		}
		s.retryError = true
	case !fserrors.IsNoRetryError(err):
		s.retryError = true
	}
}

// RetryAfter returns the time to retry after if it is set.  It will
// be Zero if it isn't set.
func (s *StatsInfo) RetryAfter() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.retryAfter
}

// NewCheckingTransfer adds a checking transfer to the stats, from the object.
func (s *StatsInfo) NewCheckingTransfer(obj fs.Object) *Transfer {
	s.checking.add(obj.Remote())
	return newCheckingTransfer(s, obj)
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

// NewTransfer adds a transfer to the stats from the object.
func (s *StatsInfo) NewTransfer(obj fs.Object) *Transfer {
	s.transferring.add(obj.Remote())
	return newTransfer(s, obj)
}

// NewTransferRemoteSize adds a transfer to the stats based on remote and size.
func (s *StatsInfo) NewTransferRemoteSize(remote string, size int64) *Transfer {
	s.transferring.add(remote)
	return newTransferRemoteSize(s, remote, size, false)
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

// SetCheckQueue sets the number of queued checks
func (s *StatsInfo) SetCheckQueue(n int, size int64) {
	s.mu.Lock()
	s.checkQueue = n
	s.checkQueueSize = size
	s.mu.Unlock()
}

// SetTransferQueue sets the number of queued transfers
func (s *StatsInfo) SetTransferQueue(n int, size int64) {
	s.mu.Lock()
	s.transferQueue = n
	s.transferQueueSize = size
	s.mu.Unlock()
}

// SetRenameQueue sets the number of queued transfers
func (s *StatsInfo) SetRenameQueue(n int, size int64) {
	s.mu.Lock()
	s.renameQueue = n
	s.renameQueueSize = size
	s.mu.Unlock()
}

// AddTransfer adds reference to the started transfer.
func (s *StatsInfo) AddTransfer(transfer *Transfer) {
	s.mu.Lock()
	s.startedTransfers = append(s.startedTransfers, transfer)
	s.mu.Unlock()
}
