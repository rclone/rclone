package accounting

import (
	"context"
	"sync"

	"github.com/rclone/rclone/fs/rc"

	"github.com/rclone/rclone/fs"
)

const globalStats = "global_stats"

var groups *statsGroups

func init() {
	// Init stats container
	groups = newStatsGroups()

	// Set the function pointer up in fs
	fs.CountError = GlobalStats().Error
}

func rcListStats(ctx context.Context, in rc.Params) (rc.Params, error) {
	out := make(rc.Params)

	out["groups"] = groups.names()

	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "core/group-list",
		Fn:    rcListStats,
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
}

func rcRemoteStats(ctx context.Context, in rc.Params) (rc.Params, error) {
	// Check to see if we should filter by group.
	group, err := in.GetString("group")
	if rc.NotErrParamNotFound(err) {
		return rc.Params{}, err
	}
	if group != "" {
		return StatsGroup(ctx, group).RemoteStats()
	}

	return groups.sum(ctx).RemoteStats()
}

func init() {
	rc.Add(rc.Call{
		Path:  "core/stats",
		Fn:    rcRemoteStats,
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
	"bytes": total transferred bytes since the start of the group,
	"checks": number of files checked,
	"deletes" : number of files deleted,
	"elapsedTime": time in floating point seconds since rclone was started,
	"errors": number of errors,
	"eta": estimated time in seconds until the group completes,
	"fatalError": boolean whether there has been at least one fatal error,
	"lastError": last error string,
	"renames" : number of files renamed,
	"retryError": boolean showing whether there has been at least one non-NoRetryError,
	"speed": average speed in bytes per second since start of the group,
	"totalBytes": total number of bytes in the group,
	"totalChecks": total number of checks in the group,
	"totalTransfers": total number of transfers in the group,
	"transferTime" : total time spent on running jobs,
	"transfers": number of transferred files,
	"transferring": an array of currently active file transfers:
		[
			{
				"bytes": total transferred bytes for this file,
				"eta": estimated time in seconds until file transfer completion
				"name": name of the file,
				"percentage": progress of the file transfer in percent,
				"speed": average speed over the whole transfer in bytes per second,
				"speedAvg": current speed in bytes per second as an exponentially weighted moving average,
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

func rcTransferredStats(ctx context.Context, in rc.Params) (rc.Params, error) {
	// Check to see if we should filter by group.
	group, err := in.GetString("group")
	if rc.NotErrParamNotFound(err) {
		return rc.Params{}, err
	}

	out := make(rc.Params)
	if group != "" {
		out["transferred"] = StatsGroup(ctx, group).Transferred()
	} else {
		out["transferred"] = groups.sum(ctx).Transferred()
	}

	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "core/transferred",
		Fn:    rcTransferredStats,
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
				"error": string description of the error (empty if successful),
				"jobid": id of the job that this transfer belongs to
			}
		]
}
` + "```" + `
`,
	})
}

func rcResetStats(ctx context.Context, in rc.Params) (rc.Params, error) {
	// Check to see if we should filter by group.
	group, err := in.GetString("group")
	if rc.NotErrParamNotFound(err) {
		return rc.Params{}, err
	}

	if group != "" {
		stats := groups.get(group)
		stats.ResetErrors()
		stats.ResetCounters()
	} else {
		groups.reset()
	}

	return rc.Params{}, nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "core/stats-reset",
		Fn:    rcResetStats,
		Title: "Reset stats.",
		Help: `
This clears counters, errors and finished transfers for all stats or specific 
stats group if group is provided.

Parameters

- group - name of the stats group (string)
`,
	})
}

func rcDeleteStats(ctx context.Context, in rc.Params) (rc.Params, error) {
	// Group name required because we only do single group.
	group, err := in.GetString("group")
	if rc.NotErrParamNotFound(err) {
		return rc.Params{}, err
	}

	if group != "" {
		groups.delete(group)
	}

	return rc.Params{}, nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "core/stats-delete",
		Fn:    rcDeleteStats,
		Title: "Delete stats group.",
		Help: `
This deletes entire stats group

Parameters

- group - name of the stats group (string)
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
	return StatsGroup(ctx, group)
}

// StatsGroup gets stats by group name.
func StatsGroup(ctx context.Context, group string) *StatsInfo {
	stats := groups.get(group)
	if stats == nil {
		return NewStatsGroup(ctx, group)
	}
	return stats
}

// GlobalStats returns special stats used for global accounting.
func GlobalStats() *StatsInfo {
	return StatsGroup(context.Background(), globalStats)
}

// NewStatsGroup creates new stats under named group.
func NewStatsGroup(ctx context.Context, group string) *StatsInfo {
	stats := NewStats(ctx)
	stats.group = group
	groups.set(ctx, group, stats)
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
func (sg *statsGroups) set(ctx context.Context, group string, stats *StatsInfo) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	ci := fs.GetConfig(ctx)

	// Limit number of groups kept in memory.
	if len(sg.order) >= ci.MaxStatsGroups {
		group := sg.order[0]
		fs.LogPrintf(fs.LogLevelDebug, nil, "Max number of stats groups reached removing %s", group)
		delete(sg.m, group)
		r := (len(sg.order) - ci.MaxStatsGroups) + 1
		sg.order = sg.order[r:]
	}

	// Exclude global stats from listing
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

// sum returns aggregate stats that contains summation of all groups.
func (sg *statsGroups) sum(ctx context.Context) *StatsInfo {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	sum := NewStats(ctx)
	for _, stats := range sg.m {
		stats.mu.RLock()
		{
			sum.bytes += stats.bytes
			sum.errors += stats.errors
			sum.fatalError = sum.fatalError || stats.fatalError
			sum.retryError = sum.retryError || stats.retryError
			sum.checks += stats.checks
			sum.transfers += stats.transfers
			sum.deletes += stats.deletes
			sum.deletedDirs += stats.deletedDirs
			sum.renames += stats.renames
			sum.checking.merge(stats.checking)
			sum.transferring.merge(stats.transferring)
			sum.inProgress.merge(stats.inProgress)
			if sum.lastError == nil && stats.lastError != nil {
				sum.lastError = stats.lastError
			}
			sum.startedTransfers = append(sum.startedTransfers, stats.startedTransfers...)
			sum.oldDuration += stats.oldDuration
			sum.oldTimeRanges = append(sum.oldTimeRanges, stats.oldTimeRanges...)
		}
		stats.mu.RUnlock()
	}
	return sum
}

func (sg *statsGroups) reset() {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	for _, stats := range sg.m {
		stats.ResetErrors()
		stats.ResetCounters()
	}

	sg.m = make(map[string]*StatsInfo)
	sg.order = nil
}

// delete removes all references to the group.
func (sg *statsGroups) delete(group string) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	stats := sg.m[group]
	if stats == nil {
		return
	}
	stats.ResetErrors()
	stats.ResetCounters()
	delete(sg.m, group)

	// Remove group reference from the ordering slice.
	tmp := sg.order[:0]
	for _, g := range sg.order {
		if g != group {
			tmp = append(tmp, g)
		}
	}
	sg.order = tmp
}
