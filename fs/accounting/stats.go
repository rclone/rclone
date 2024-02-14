package accounting

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/lib/terminal"
)

const (
	averagePeriodLength = time.Second
	averageStopAfter    = time.Minute
)

// MaxCompletedTransfers specifies maximum number of completed transfers in startedTransfers list
var MaxCompletedTransfers = 100

// StatsInfo accounts all transfers
// N.B.: if this struct is modified, please remember to also update sum() function in stats_groups
// to correctly count the updated fields
type StatsInfo struct {
	mu                  sync.RWMutex
	ctx                 context.Context
	ci                  *fs.ConfigInfo
	bytes               int64
	errors              int64
	lastError           error
	fatalError          bool
	retryError          bool
	retryAfter          time.Time
	checks              int64
	checking            *transferMap
	checkQueue          int
	checkQueueSize      int64
	transfers           int64
	transferring        *transferMap
	transferQueue       int
	transferQueueSize   int64
	renames             int64
	renameQueue         int
	renameQueueSize     int64
	deletes             int64
	deletesSize         int64
	deletedDirs         int64
	inProgress          *inProgress
	startedTransfers    []*Transfer   // currently active transfers
	oldTimeRanges       timeRanges    // a merged list of time ranges for the transfers
	oldDuration         time.Duration // duration of transfers we have culled
	group               string
	startTime           time.Time // the moment these stats were initialized or reset
	average             averageValues
	serverSideCopies    int64
	serverSideCopyBytes int64
	serverSideMoves     int64
	serverSideMoveBytes int64
}

type averageValues struct {
	mu        sync.Mutex
	lpBytes   int64
	lpTime    time.Time
	speed     float64
	stop      chan bool
	stopped   sync.WaitGroup
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewStats creates an initialised StatsInfo
func NewStats(ctx context.Context) *StatsInfo {
	ci := fs.GetConfig(ctx)
	return &StatsInfo{
		ctx:          ctx,
		ci:           ci,
		checking:     newTransferMap(ci.Checkers, "checking"),
		transferring: newTransferMap(ci.Transfers, "transferring"),
		inProgress:   newInProgress(ctx),
		startTime:    time.Now(),
		average:      averageValues{stop: make(chan bool)},
	}
}

// RemoteStats returns stats for rc
func (s *StatsInfo) RemoteStats() (out rc.Params, err error) {
	// NB if adding values here - make sure you update the docs in
	// stats_groups.go

	out = make(rc.Params)

	ts := s.calculateTransferStats()
	out["totalChecks"] = ts.totalChecks
	out["totalTransfers"] = ts.totalTransfers
	out["totalBytes"] = ts.totalBytes
	out["transferTime"] = ts.transferTime
	out["speed"] = ts.speed

	s.mu.RLock()
	out["bytes"] = s.bytes
	out["errors"] = s.errors
	out["fatalError"] = s.fatalError
	out["retryError"] = s.retryError
	out["checks"] = s.checks
	out["transfers"] = s.transfers
	out["deletes"] = s.deletes
	out["deletedDirs"] = s.deletedDirs
	out["renames"] = s.renames
	out["elapsedTime"] = time.Since(s.startTime).Seconds()
	out["serverSideCopies"] = s.serverSideCopies
	out["serverSideCopyBytes"] = s.serverSideCopyBytes
	out["serverSideMoves"] = s.serverSideMoves
	out["serverSideMoveBytes"] = s.serverSideMoveBytes
	eta, etaOK := eta(s.bytes, ts.totalBytes, ts.speed)
	if etaOK {
		out["eta"] = eta.Seconds()
	} else {
		out["eta"] = nil
	}
	s.mu.RUnlock()

	if !s.checking.empty() {
		out["checking"] = s.checking.remotes()
	}
	if !s.transferring.empty() {
		out["transferring"] = s.transferring.rcStats(s.inProgress)
	}
	if s.errors > 0 {
		out["lastError"] = s.lastError.Error()
	}

	return out, nil
}

// _speed returns the average speed of the transfer in bytes/second
//
// Call with lock held
func (s *StatsInfo) _speed() float64 {
	return s.average.speed
}

// timeRange is a start and end time of a transfer
type timeRange struct {
	start time.Time
	end   time.Time
}

// timeRanges is a list of non-overlapping start and end times for
// transfers
type timeRanges []timeRange

// merge all the overlapping time ranges
func (trs *timeRanges) merge() {
	Trs := *trs

	// Sort by the starting time.
	sort.Slice(Trs, func(i, j int) bool {
		return Trs[i].start.Before(Trs[j].start)
	})

	// Merge overlaps and add distinctive ranges together
	var (
		newTrs = Trs[:0]
		i, j   = 0, 1
	)
	for i < len(Trs) {
		if j < len(Trs) {
			if !Trs[i].end.Before(Trs[j].start) {
				if Trs[i].end.Before(Trs[j].end) {
					Trs[i].end = Trs[j].end
				}
				j++
				continue
			}
		}
		newTrs = append(newTrs, Trs[i])
		i = j
		j++
	}

	*trs = newTrs
}

// cull remove any ranges whose start and end are before cutoff
// returning their duration sum
func (trs *timeRanges) cull(cutoff time.Time) (d time.Duration) {
	var newTrs = (*trs)[:0]
	for _, tr := range *trs {
		if cutoff.Before(tr.start) || cutoff.Before(tr.end) {
			newTrs = append(newTrs, tr)
		} else {
			d += tr.end.Sub(tr.start)
		}
	}
	*trs = newTrs
	return d
}

// total the time out of the time ranges
func (trs timeRanges) total() (total time.Duration) {
	for _, tr := range trs {
		total += tr.end.Sub(tr.start)
	}
	return total
}

// Total duration is union of durations of all transfers belonging to this
// object.
//
// Needs to be protected by mutex.
func (s *StatsInfo) _totalDuration() time.Duration {
	// copy of s.oldTimeRanges with extra room for the current transfers
	timeRanges := make(timeRanges, len(s.oldTimeRanges), len(s.oldTimeRanges)+len(s.startedTransfers))
	copy(timeRanges, s.oldTimeRanges)

	// Extract time ranges of all transfers.
	now := time.Now()
	for i := range s.startedTransfers {
		start, end := s.startedTransfers[i].TimeRange()
		if end.IsZero() {
			end = now
		}
		timeRanges = append(timeRanges, timeRange{start, end})
	}

	timeRanges.merge()
	return s.oldDuration + timeRanges.total()
}

const (
	etaMaxSeconds = (1<<63 - 1) / int64(time.Second)           // Largest possible ETA as number of seconds
	etaMax        = time.Duration(etaMaxSeconds) * time.Second // Largest possible ETA, which is in second precision, representing "292y24w3d23h47m16s"
)

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
	seconds := int64(float64(remaining) / rate)
	if seconds < 0 {
		// Got Int64 overflow
		eta = etaMax
	} else if seconds >= etaMaxSeconds {
		// Would get Int64 overflow if converting from seconds to Duration (nanoseconds)
		eta = etaMax
	} else {
		eta = time.Duration(seconds) * time.Second
	}
	return eta, true
}

// etaString returns the ETA of the current operation,
// rounded to full seconds.
// If the ETA cannot be determined it returns "-"
func etaString(done, total int64, rate float64) string {
	d, ok := eta(done, total, rate)
	if !ok {
		return "-"
	}
	if d == etaMax {
		return "-"
	}
	return fs.Duration(d).ShortReadableString()
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

// returned from calculateTransferStats
type transferStats struct {
	totalChecks    int64
	totalTransfers int64
	totalBytes     int64
	transferTime   float64
	speed          float64
}

// calculateTransferStats calculates some additional transfer stats not
// stored directly in StatsInfo
func (s *StatsInfo) calculateTransferStats() (ts transferStats) {
	// checking and transferring have their own locking so read
	// here before lock to prevent deadlock on GetBytes
	transferring, checking := s.transferring.count(), s.checking.count()
	transferringBytesDone, transferringBytesTotal := s.transferring.progress(s)

	s.mu.RLock()
	defer s.mu.RUnlock()

	ts.totalChecks = int64(s.checkQueue) + s.checks + int64(checking)
	ts.totalTransfers = int64(s.transferQueue) + s.transfers + int64(transferring)
	// note that s.bytes already includes transferringBytesDone so
	// we take it off here to avoid double counting
	ts.totalBytes = s.transferQueueSize + s.bytes + transferringBytesTotal - transferringBytesDone
	ts.speed = s.average.speed
	dt := s._totalDuration()
	ts.transferTime = dt.Seconds()

	return ts
}

func (s *StatsInfo) averageLoop() {
	var period float64

	ticker := time.NewTicker(averagePeriodLength)
	defer ticker.Stop()

	startTime := time.Now()
	a := &s.average
	defer a.stopped.Done()
	for {
		select {
		case now := <-ticker.C:
			a.mu.Lock()
			var elapsed float64
			if a.lpTime.IsZero() {
				elapsed = now.Sub(startTime).Seconds()
			} else {
				elapsed = now.Sub(a.lpTime).Seconds()
			}
			avg := 0.0
			if elapsed > 0 {
				avg = float64(a.lpBytes) / elapsed
			}
			if period < averagePeriod {
				period++
			}
			a.speed = (avg + a.speed*(period-1)) / period
			a.lpBytes = 0
			a.lpTime = now
			a.mu.Unlock()
		case <-a.stop:
			return
		}
	}
}

// Start the average loop
func (s *StatsInfo) startAverageLoop() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.average.startOnce.Do(func() {
		s.average.stopped.Add(1)
		go s.averageLoop()
	})
}

// Stop the average loop
//
// Call with the mutex held
func (s *StatsInfo) _stopAverageLoop() {
	s.average.stopOnce.Do(func() {
		close(s.average.stop)
		s.average.stopped.Wait()
	})
}

// Stop the average loop
func (s *StatsInfo) stopAverageLoop() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s._stopAverageLoop()
}

// String convert the StatsInfo to a string for printing
func (s *StatsInfo) String() string {
	// NB if adding more stats in here, remember to add them into
	// RemoteStats() too.

	ts := s.calculateTransferStats()

	s.mu.RLock()

	var (
		buf                    = &bytes.Buffer{}
		xfrchkString           = ""
		dateString             = ""
		elapsedTime            = time.Since(s.startTime)
		elapsedTimeSecondsOnly = elapsedTime.Truncate(time.Second/10) % time.Minute
		displaySpeedString     string
	)

	if s.ci.DataRateUnit == "bits" {
		displaySpeedString = fs.SizeSuffix(ts.speed * 8).BitRateUnit()
	} else {
		displaySpeedString = fs.SizeSuffix(ts.speed).ByteRateUnit()
	}

	if !s.ci.StatsOneLine {
		_, _ = fmt.Fprintf(buf, "\nTransferred:   	")
	} else {
		xfrchk := []string{}
		if ts.totalTransfers > 0 && s.transferQueue > 0 {
			xfrchk = append(xfrchk, fmt.Sprintf("xfr#%d/%d", s.transfers, ts.totalTransfers))
		}
		if ts.totalChecks > 0 && s.checkQueue > 0 {
			xfrchk = append(xfrchk, fmt.Sprintf("chk#%d/%d", s.checks, ts.totalChecks))
		}
		if len(xfrchk) > 0 {
			xfrchkString = fmt.Sprintf(" (%s)", strings.Join(xfrchk, ", "))
		}
		if s.ci.StatsOneLineDate {
			t := time.Now()
			dateString = t.Format(s.ci.StatsOneLineDateFormat) // Including the separator so people can customize it
		}
	}

	_, _ = fmt.Fprintf(buf, "%s%13s / %s, %s, %s, ETA %s%s",
		dateString,
		fs.SizeSuffix(s.bytes).ByteUnit(),
		fs.SizeSuffix(ts.totalBytes).ByteUnit(),
		percent(s.bytes, ts.totalBytes),
		displaySpeedString,
		etaString(s.bytes, ts.totalBytes, ts.speed),
		xfrchkString,
	)

	if s.ci.ProgressTerminalTitle {
		// Writes ETA to the terminal title
		terminal.WriteTerminalTitle("ETA: " + etaString(s.bytes, ts.totalBytes, ts.speed))
	}

	if !s.ci.StatsOneLine {
		_, _ = buf.WriteRune('\n')
		errorDetails := ""
		switch {
		case s.fatalError:
			errorDetails = " (fatal error encountered)"
		case s.retryError:
			errorDetails = " (retrying may help)"
		case s.errors != 0:
			errorDetails = " (no need to retry)"

		}

		// Add only non zero stats
		if s.errors != 0 {
			_, _ = fmt.Fprintf(buf, "Errors:        %10d%s\n",
				s.errors, errorDetails)
		}
		if s.checks != 0 || ts.totalChecks != 0 {
			_, _ = fmt.Fprintf(buf, "Checks:        %10d / %d, %s\n",
				s.checks, ts.totalChecks, percent(s.checks, ts.totalChecks))
		}
		if s.deletes != 0 || s.deletedDirs != 0 {
			_, _ = fmt.Fprintf(buf, "Deleted:       %10d (files), %d (dirs)\n", s.deletes, s.deletedDirs)
		}
		if s.renames != 0 {
			_, _ = fmt.Fprintf(buf, "Renamed:       %10d\n", s.renames)
		}
		if s.transfers != 0 || ts.totalTransfers != 0 {
			_, _ = fmt.Fprintf(buf, "Transferred:   %10d / %d, %s\n",
				s.transfers, ts.totalTransfers, percent(s.transfers, ts.totalTransfers))
		}
		if s.serverSideCopies != 0 || s.serverSideCopyBytes != 0 {
			_, _ = fmt.Fprintf(buf, "Server Side Copies:%6d @ %s\n",
				s.serverSideCopies, fs.SizeSuffix(s.serverSideCopyBytes).ByteUnit(),
			)
		}
		if s.serverSideMoves != 0 || s.serverSideMoveBytes != 0 {
			_, _ = fmt.Fprintf(buf, "Server Side Moves:%7d @ %s\n",
				s.serverSideMoves, fs.SizeSuffix(s.serverSideMoveBytes).ByteUnit(),
			)
		}
		_, _ = fmt.Fprintf(buf, "Elapsed time:  %10ss\n", strings.TrimRight(fs.Duration(elapsedTime.Truncate(time.Minute)).ReadableString(), "0s")+fmt.Sprintf("%.1f", elapsedTimeSecondsOnly.Seconds()))
	}

	// checking and transferring have their own locking so unlock
	// here to prevent deadlock on GetBytes
	s.mu.RUnlock()

	// Add per transfer stats if required
	if !s.ci.StatsOneLine {
		if !s.checking.empty() {
			_, _ = fmt.Fprintf(buf, "Checking:\n%s\n", s.checking.String(s.ctx, s.inProgress, s.transferring))
		}
		if !s.transferring.empty() {
			_, _ = fmt.Fprintf(buf, "Transferring:\n%s\n", s.transferring.String(s.ctx, s.inProgress, nil))
		}
	}

	return buf.String()
}

// Transferred returns list of all completed transfers including checked and
// failed ones.
func (s *StatsInfo) Transferred() []TransferSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	if s.ci.UseJSONLog {
		out, _ := s.RemoteStats()
		fs.LogLevelPrintf(s.ci.StatsLogLevel, nil, "%v%v\n", s, fs.LogValueHide("stats", out))
	} else {
		fs.LogLevelPrintf(s.ci.StatsLogLevel, nil, "%v\n", s)
	}

}

// Bytes updates the stats for bytes bytes
func (s *StatsInfo) Bytes(bytes int64) {
	s.average.mu.Lock()
	s.average.lpBytes += bytes
	s.average.mu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.bytes += bytes
}

// BytesNoNetwork updates the stats for bytes bytes but doesn't include the transfer stats
func (s *StatsInfo) BytesNoNetwork(bytes int64) {
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

// GetBytesWithPending returns the number of bytes transferred and remaining transfers
func (s *StatsInfo) GetBytesWithPending() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pending := int64(0)
	for _, tr := range s.startedTransfers {
		if tr.acc != nil {
			bytes, size := tr.acc.progress()
			if bytes < size {
				pending += size - bytes
			}
		}
	}
	return s.bytes + pending
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

var (
	errMaxDelete     = fserrors.FatalError(errors.New("--max-delete threshold reached"))
	errMaxDeleteSize = fserrors.FatalError(errors.New("--max-delete-size threshold reached"))
)

// DeleteFile updates the stats for deleting a file
//
// It may return fatal errors if the threshold for --max-delete or
// --max-delete-size have been reached.
func (s *StatsInfo) DeleteFile(ctx context.Context, size int64) error {
	ci := fs.GetConfig(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if size < 0 {
		size = 0
	}
	if ci.MaxDelete >= 0 && s.deletes+1 > ci.MaxDelete {
		return errMaxDelete
	}
	if ci.MaxDeleteSize >= 0 && s.deletesSize+size > int64(ci.MaxDeleteSize) {
		return errMaxDeleteSize
	}
	s.deletes++
	s.deletesSize += size
	return nil
}

// GetDeletes returns the number of deletes
func (s *StatsInfo) GetDeletes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deletes
}

// DeletedDirs updates the stats for deletedDirs
func (s *StatsInfo) DeletedDirs(deletedDirs int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletedDirs += deletedDirs
	return s.deletedDirs
}

// Renames updates the stats for renames
func (s *StatsInfo) Renames(renames int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renames += renames
	return s.renames
}

// ResetCounters sets the counters (bytes, checks, errors, transfers, deletes, renames) to 0 and resets lastError, fatalError and retryError
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
	s.deletesSize = 0
	s.deletedDirs = 0
	s.renames = 0
	s.startedTransfers = nil
	s.oldDuration = 0

	s._stopAverageLoop()
	s.average = averageValues{stop: make(chan bool)}
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
func (s *StatsInfo) Error(err error) error {
	if err == nil || fserrors.IsCounted(err) {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors++
	s.lastError = err
	err = fserrors.FsError(err)
	fserrors.Count(err)
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
	return err
}

// RetryAfter returns the time to retry after if it is set.  It will
// be Zero if it isn't set.
func (s *StatsInfo) RetryAfter() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.retryAfter
}

// NewCheckingTransfer adds a checking transfer to the stats, from the object.
func (s *StatsInfo) NewCheckingTransfer(obj fs.DirEntry, what string) *Transfer {
	tr := newCheckingTransfer(s, obj, what)
	s.checking.add(tr)
	return tr
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
//
// The obj is uses as the srcFs, the dstFs must be supplied
func (s *StatsInfo) NewTransfer(obj fs.DirEntry, dstFs fs.Fs) *Transfer {
	var srcFs fs.Fs
	if oi, ok := obj.(fs.ObjectInfo); ok {
		if f, ok := oi.Fs().(fs.Fs); ok {
			srcFs = f
		}
	}
	tr := newTransfer(s, obj, srcFs, dstFs)
	s.transferring.add(tr)
	s.startAverageLoop()
	return tr
}

// NewTransferRemoteSize adds a transfer to the stats based on remote and size.
func (s *StatsInfo) NewTransferRemoteSize(remote string, size int64, srcFs, dstFs fs.Fs) *Transfer {
	tr := newTransferRemoteSize(s, remote, size, false, "", srcFs, dstFs)
	s.transferring.add(tr)
	s.startAverageLoop()
	return tr
}

// DoneTransferring removes a transfer from the stats
//
// if ok is true and it was in the transfermap (to avoid incrementing in case of nested calls, #6213) then it increments the transfers count
func (s *StatsInfo) DoneTransferring(remote string, ok bool) {
	existed := s.transferring.del(remote)
	if ok && existed {
		s.mu.Lock()
		s.transfers++
		s.mu.Unlock()
	}
	if s.transferring.empty() && s.checking.empty() {
		time.AfterFunc(averageStopAfter, s.stopAverageLoop)
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

// _removeTransfer removes a reference to the started transfer in
// position i.
//
// Must be called with the lock held
func (s *StatsInfo) _removeTransfer(transfer *Transfer, i int) {
	now := time.Now()

	// add finished transfer onto old time ranges
	start, end := transfer.TimeRange()
	if end.IsZero() {
		end = now
	}
	s.oldTimeRanges = append(s.oldTimeRanges, timeRange{start, end})
	s.oldTimeRanges.merge()

	// remove the found entry
	s.startedTransfers = append(s.startedTransfers[:i], s.startedTransfers[i+1:]...)

	// Find youngest active transfer
	oldestStart := now
	for i := range s.startedTransfers {
		start, _ := s.startedTransfers[i].TimeRange()
		if start.Before(oldestStart) {
			oldestStart = start
		}
	}

	// remove old entries older than that
	s.oldDuration += s.oldTimeRanges.cull(oldestStart)
}

// RemoveTransfer removes a reference to the started transfer.
func (s *StatsInfo) RemoveTransfer(transfer *Transfer) {
	s.mu.Lock()
	for i, tr := range s.startedTransfers {
		if tr == transfer {
			s._removeTransfer(tr, i)
			break
		}
	}
	s.mu.Unlock()
}

// PruneTransfers makes sure there aren't too many old transfers by removing
// single finished transfer.
func (s *StatsInfo) PruneTransfers() {
	if MaxCompletedTransfers < 0 {
		return
	}
	s.mu.Lock()
	// remove a transfer from the start if we are over quota
	if len(s.startedTransfers) > MaxCompletedTransfers+s.ci.Transfers {
		for i, tr := range s.startedTransfers {
			if tr.IsDone() {
				s._removeTransfer(tr, i)
				break
			}
		}
	}
	s.mu.Unlock()
}

// AddServerSideMove counts a server side move
func (s *StatsInfo) AddServerSideMove(n int64) {
	s.mu.Lock()
	s.serverSideMoves += 1
	s.serverSideMoveBytes += n
	s.mu.Unlock()
}

// AddServerSideCopy counts a server side copy
func (s *StatsInfo) AddServerSideCopy(n int64) {
	s.mu.Lock()
	s.serverSideCopies += 1
	s.serverSideCopyBytes += n
	s.mu.Unlock()
}
