package accounting

import (
	"context"
	"sync"

	"github.com/ncw/rclone/fs/rc"

	"github.com/ncw/rclone/fs"
)

const globalStats = "global_stats"

var groups *statsGroups

func remoteStats(ctx context.Context, in rc.Params) (rc.Params, error) {
	// Check to see if we should filter by group.
	group, err := in.GetString("group")
	if rc.NotErrParamNotFound(err) {
		return rc.Params{}, err
	}
	if group != "" {
		return StatsGroup(group).RemoteStats()
	}

	return groups.sum().RemoteStats()
}

func init() {
	// Init stats container
	groups = newStatsGroups()

	// Set the function pointer up in fs
	fs.CountError = GlobalStats().Error

	rc.Add(rc.Call{
		Path:  "core/stats",
		Fn:    remoteStats,
		Title: "Returns stats about current transfers.",
		Help: `
This returns all available stats

	rclone rc core/stats

Returns the following values:

` + "```" + `
{
	"speed": average speed in bytes/sec since start of the process,
	"bytes": total transferred bytes since the start of the process,
	"errors": number of errors,
	"fatalError": whether there has been at least one FatalError,
	"retryError": whether there has been at least one non-NoRetryError,
	"checks": number of checked files,
	"transfers": number of transferred files,
	"deletes" : number of deleted files,
	"elapsedTime": time in seconds since the start of the process,
	"lastError": last occurred error,
	"transferring": an array of currently active file transfers:
		[
			{
				"bytes": total transferred bytes for this file,
				"eta": estimated time in seconds until file transfer completion
				"name": name of the file,
				"percentage": progress of the file transfer in percent,
				"speed": speed in bytes/sec,
				"speedAvg": speed in bytes/sec as an exponentially weighted moving average,
				"size": size of the file in bytes
			}
		],
	"checking": an array of names of currently active file checks
		[]
}
` + "```" + `
Values for "transferring", "checking" and "lastError" are only assigned if data is available.
The value for "eta" is null if an eta cannot be determined.
`,
	})
}

type statsGroupCtx int64

const statsGroupKey statsGroupCtx = 1

// WithStatsGroup returns copy of the parent context with assigned group.
func WithStatsGroup(parent context.Context, group string) context.Context {
	return context.WithValue(parent, statsGroupKey, group)
}

// StatsGroupFromContext returns group from the context if it's available.
// Returns false if group is empty.
func StatsGroupFromContext(ctx context.Context) (string, bool) {
	statsGroup, ok := ctx.Value(statsGroupKey).(string)
	if statsGroup == "" {
		ok = false
	}
	return statsGroup, ok
}

// Stats gets stats by extracting group from context.
func Stats(ctx context.Context) *StatsInfo {
	group, ok := StatsGroupFromContext(ctx)
	if !ok {
		return GlobalStats()
	}
	return StatsGroup(group)
}

// StatsGroup gets stats by group name.
func StatsGroup(group string) *StatsInfo {
	stats := groups.get(group)
	if stats == nil {
		return NewStatsGroup(group)
	}
	return stats
}

// GlobalStats returns special stats used for global accounting.
func GlobalStats() *StatsInfo {
	return StatsGroup(globalStats)
}

// NewStatsGroup creates new stats under named group.
func NewStatsGroup(group string) *StatsInfo {
	stats := NewStats()
	groups.set(group, stats)
	return stats
}

// statsGroups holds a synchronized map of stats
type statsGroups struct {
	mu sync.Mutex
	m  map[string]*StatsInfo
}

// newStatsGroups makes a new statsGroups object
func newStatsGroups() *statsGroups {
	return &statsGroups{
		m: make(map[string]*StatsInfo),
	}
}

// set marks the stats as belonging to a group
func (sg *statsGroups) set(group string, acc *StatsInfo) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	sg.m[group] = acc
}

// clear discards reference to group
func (sg *statsGroups) clear(group string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	delete(sg.m, group)
}

// get gets the stats for group, or nil if not found
func (sg *statsGroups) get(group string) *StatsInfo {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	stats, ok := sg.m[group]
	if !ok {
		return nil
	}
	return stats
}

// get gets the stats for group, or nil if not found
func (sg *statsGroups) sum() *StatsInfo {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	sum := NewStats()
	for _, stats := range sg.m {
		sum.bytes += stats.bytes
		sum.errors += stats.errors
		sum.fatalError = sum.fatalError || stats.fatalError
		sum.retryError = sum.retryError || stats.retryError
		sum.checks += stats.checks
		sum.transfers += stats.transfers
		sum.deletes += stats.deletes
		sum.checking.merge(stats.checking)
		sum.transferring.merge(stats.transferring)
		sum.inProgress.merge(stats.inProgress)
		if sum.lastError == nil && stats.lastError != nil {
			sum.lastError = stats.lastError
		}
	}
	return sum
}
