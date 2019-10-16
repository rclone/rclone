package accounting

import (
	"context"
	"sync"

	"github.com/rclone/rclone/fs/rc"

	"github.com/rclone/rclone/fs"
)

const globalStats = "global_stats"

var groups *statsGroups

func listStats(ctx context.Context, in rc.Params) (rc.Params, error) {
	out := make(rc.Params)

	out["groups"] = groups.names()

	return out, nil
}

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

func transferredStats(ctx context.Context, in rc.Params) (rc.Params, error) {
	// Check to see if we should filter by group.
	group, err := in.GetString("group")
	if rc.NotErrParamNotFound(err) {
		return rc.Params{}, err
	}

	out := make(rc.Params)
	if group != "" {
		out["transferred"] = StatsGroup(group).Transferred()
	} else {
		out["transferred"] = groups.sum().Transferred()
	}

	return out, nil
}

func resetStats(ctx context.Context, in rc.Params) (rc.Params, error) {
	// Check to see if we should filter by group.
	group, err := in.GetString("group")
	if rc.NotErrParamNotFound(err) {
		return rc.Params{}, err
	}

	if group != "" {
		groups.get(group).ResetCounters()
		groups.get(group).ResetErrors()
	} else {
		groups.clear()
	}

	return rc.Params{}, nil
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
This returns all available stats:

	rclone rc core/stats

If group is not provided then summed up stats for all groups will be
returned.

Parameters
- group - name of the stats group (string)

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

	rc.Add(rc.Call{
		Path:  "core/transferred",
		Fn:    transferredStats,
		Title: "Returns stats about completed transfers.",
		Help: `
This returns stats about completed transfers:

	rclone rc core/transferred

If group is not provided then completed transfers for all groups will be
returned.

Note only the last 100 completed transfers are returned.

Parameters
- group - name of the stats group (string)

Returns the following values:
` + "```" + `
{
	"transferred":  an array of completed transfers (including failed ones):
		[
			{
				"name": name of the file,
				"size": size of the file in bytes,
				"bytes": total transferred bytes for this file,
				"checked": if the transfer is only checked (skipped, deleted),
				"timestamp": integer representing millisecond unix epoch,
				"error": string description of the error (empty if successfull),
				"jobid": id of the job that this transfer belongs to
			}
		]
}
` + "```" + `
`,
	})

	rc.Add(rc.Call{
		Path:  "core/group-list",
		Fn:    listStats,
		Title: "Returns list of stats.",
		Help: `
This returns list of stats groups currently in memory. 

Returns the following values:
` + "```" + `
{
	"groups":  an array of group names:
		[
			"group1",
			"group2",
			...
		]
}
` + "```" + `
`,
	})

	rc.Add(rc.Call{
		Path:  "core/stats-reset",
		Fn:    resetStats,
		Title: "Reset stats.",
		Help: `
This clears counters and errors for all stats or specific stats group if group
is provided.

Parameters
- group - name of the stats group (string)
` + "```" + `
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
	mu    sync.Mutex
	m     map[string]*StatsInfo
	order []string
}

// newStatsGroups makes a new statsGroups object
func newStatsGroups() *statsGroups {
	return &statsGroups{
		m: make(map[string]*StatsInfo),
	}
}

// set marks the stats as belonging to a group
func (sg *statsGroups) set(group string, stats *StatsInfo) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	// Limit number of groups kept in memory.
	if len(sg.order) >= fs.Config.MaxStatsGroups {
		group := sg.order[0]
		fs.LogPrintf(fs.LogLevelInfo, nil, "Max number of stats groups reached removing %s", group)
		delete(sg.m, group)
		r := (len(sg.order) - fs.Config.MaxStatsGroups) + 1
		sg.order = sg.order[r:]
	}

	// Exclude global stats from
	if group != globalStats {
		sg.order = append(sg.order, group)
	}
	sg.m[group] = stats
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

func (sg *statsGroups) names() []string {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	return sg.order
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
		sum.startedTransfers = append(sum.startedTransfers, stats.startedTransfers...)
	}
	return sum
}

func (sg *statsGroups) clear() {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	for _, stats := range sg.m {
		stats.ResetErrors()
		stats.ResetCounters()
	}

	sg.m = make(map[string]*StatsInfo)
	sg.order = nil
}
