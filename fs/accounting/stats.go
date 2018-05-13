package accounting

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/rc"
	"github.com/pkg/errors"
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
	s := &StatsInfo{
		checking:     newStringSet(fs.Config.Checkers),
		transferring: newStringSet(fs.Config.Transfers),
		start:        time.Now(),
		inProgress:   newInProgress(),
	}

	rc.Add(rc.Call{
		Path:  "stats/transfers",
		Fn:    s.bwStats,
		Title: "Get transfer stats",
		Help: `
Show statistics for all transfers.
`,
	})

	rc.Add(rc.Call{
		Path:  "stats/reset",
		Fn:    s.bwResetStats,
		Title: "Reset transfer stats",
		Help: `
Resets the global transfer statistic counters.
`,
	})

	return s
}

// reset statistics as requested by rc stats/reset
func (s *StatsInfo) bwResetStats(in rc.Params) (out rc.Params, err error) {
	out = make(rc.Params)

	s.mu.Lock()
	s.bytes = 0
	s.errors = 0
	s.checks = 0
	s.transfers = 0
	s.deletes = 0
	s.start = time.Now()
	s.mu.Unlock()

	out["status"] = "ok"
	out["message"] = "Global transfer statistic counters were reset to Zero"
	return out, nil
}

// create map with statistics for rc stats/transfers
func (s *StatsInfo) bwStats(in rc.Params) (out rc.Params, err error) {
	out = make(rc.Params)
	g, t, c, err := s.Map()
	if err != nil {
		return out, errors.Errorf("error while getting bw stats")
	}
	out["status"] = "ok"
	out["global"] = g
	out["transferring"] = t
	out["checking"] = c
	return out, nil
}

// Map convert the StatsInfo to a map for rc stats
func (s *StatsInfo) Map() (map[string]interface{}, map[string]map[string]interface{}, map[string]map[string]interface{}, error) {
	dataRateMultiplier := 1.0
	if fs.Config.DataRateUnit == "bits" {
		dataRateMultiplier = 8.0
	}
	s.mu.RLock()
	dt := time.Now().Sub(s.start)
	dtSeconds := dt.Seconds()
	speed := 0.0
	if dt > 0 {
		speed = float64(s.bytes) / dtSeconds
	}

	g := make(map[string]interface{})
	g["bytes"] = s.bytes
	g["seconds"] = dtSeconds
	g["speed"] = speed * dataRateMultiplier
	g["transfers"] = s.transfers
	g["checks"] = s.checks
	g["errors"] = s.errors
	g["deletes"] = s.deletes
	s.mu.RUnlock()

	r := make(map[string]map[string]interface{})
	s.transferring.mu.RLock()
	for name := range s.transferring.items {
		acc := s.inProgress.get(name)
		r[name] = make(map[string]interface{})
		r[name]["inProgress"] = (acc != nil)
		if acc != nil {
			bytes, size := acc.progress()
			speedBps, speedMavg := acc.speed()
			eta, etaOk := acc.eta()

			r[name]["bytes"] = bytes
			r[name]["size"] = size
			r[name]["speed_moving_avg"] = speedMavg * dataRateMultiplier
			r[name]["speed"] = speedBps * dataRateMultiplier
			if etaOk {
				r[name]["eta"] = eta
			}
			percentageDone := 0.0
			if bytes > 0 {
				percentageDone = 100 * float64(bytes) / float64(size)
			}
			r[name]["percentageDone"] = percentageDone

			acc.statmu.Lock()
			r[name]["name"] = acc.name
			r[name]["start"] = acc.start
			acc.statmu.Unlock()
		}
	}
	s.transferring.mu.RUnlock()

	c := make(map[string]map[string]interface{})
	s.checking.mu.RLock()
	for name := range s.checking.items {
		c[name] = make(map[string]interface{})
		c[name]["name"] = name
	}
	s.checking.mu.RUnlock()
	return g, r, c, nil
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

	_, _ = fmt.Fprintf(buf, `
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
		_, _ = fmt.Fprintf(buf, "Checking:\n%s\n", s.checking)
	}
	if !s.transferring.empty() {
		_, _ = fmt.Fprintf(buf, "Transferring:\n%s\n", s.transferring)
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
