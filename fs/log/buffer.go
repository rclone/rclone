package log

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
)

// ErrBufferDisabled is returned when the recent logs are needed but
// --log-buffer-size has not been set.
var ErrBufferDisabled = rc.NewErrParamInvalid(errors.New("log buffer not enabled - set --log-buffer-size"))

// Recent holds the most recent log entries if --log-buffer-size is set.
var Recent = &Buffer{}

// Maximum number of entries Get looks at when a limit is in force.
const maxExamine = 100_000

// bufferEntry is a single log entry in the Buffer.
type bufferEntry struct {
	seq         int64
	level       slog.Level
	attribution Attribution
	json        json.RawMessage
}

// Filter selects which entries Buffer.Get returns.
//
// Note that the zero value of Level is INFO, so the zero value
// returns the entries at INFO or more severe.
type Filter struct {
	Level        slog.Level // only return entries at this level or more severe - note the zero value is INFO
	JobID        int64      // if set, only return entries attributed to this rc job
	Group        string     // if set, only return entries attributed to this stats group
	Unattributed bool       // if set with JobID or Group, also return entries which aren't attributed
}

// match returns true if an entry at level attributed to attribution
// should be returned.
func (f *Filter) match(level slog.Level, attribution Attribution) bool {
	if level < f.Level {
		return false
	}
	if f.JobID == 0 && f.Group == "" {
		return true
	}
	if f.JobID != 0 && attribution.JobID == f.JobID {
		return true
	}
	if f.Group != "" && attribution.Group == f.Group {
		return true
	}
	return f.Unattributed && !attribution.Attributed()
}

// Entries is a list of log entries in JSON format as returned by Buffer.Get.
type Entries []json.RawMessage

// String returns a summary of the entries.
//
// The entries aren't rendered as they are logged when rc replies
// are logged at DEBUG level and that would cause the log to feed
// back into itself.
func (e Entries) String() string {
	return fmt.Sprintf("%d log entries", len(e))
}

// Buffer is a size limited in memory store of the most recent log
// entries in JSON format.
//
// Each entry is given a sequence number starting from 0 which
// increases by one for each entry added. This is included in the
// entry as the "seq" field.
type Buffer struct {
	mu      sync.Mutex
	entries []bufferEntry // oldest first in increasing sequence number
	size    int64         // total size of the JSON in entries
	maxSize int64         // drop old entries to keep size <= this - disabled if <= 0
}

// SetSize sets the maximum size in bytes of the log entries kept.
//
// Setting this <= 0 disables the Buffer and discards its entries.
func (b *Buffer) SetSize(maxSize int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maxSize = maxSize
	b.trim()
}

// Enabled returns whether the Buffer is storing log entries.
func (b *Buffer) Enabled() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.maxSize > 0
}

// Seq returns the sequence number the next log entry will be given.
func (b *Buffer) Seq() int64 {
	return nextSeq.Load()
}

// trim drops the oldest entries until the Buffer is within its size.
//
// Call with the mutex held.
func (b *Buffer) trim() {
	for len(b.entries) > 0 && b.size > b.maxSize {
		b.size -= int64(len(b.entries[0].json))
		b.entries[0] = bufferEntry{} // release the memory
		b.entries = b.entries[1:]
	}
	if len(b.entries) == 0 {
		b.entries = nil
	}
}

// add adds a log entry made by makeEntry to the Buffer.
//
// This is called with the log handler's mutex held so it must not log.
func (b *Buffer) add(seq int64, level slog.Level, attribution Attribution, entry json.RawMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.maxSize <= 0 {
		return
	}
	// Don't store an entry bigger than the buffer as it would evict
	// everything else - it will show up as lost
	if int64(len(entry)) > b.maxSize {
		return
	}
	b.entries = append(b.entries, bufferEntry{seq: seq, level: level, attribution: attribution, json: entry})
	b.size += int64(len(entry))
	b.trim()
}

// Get returns the entries with sequence numbers from <= seq < to
// which match filter, oldest first.
//
// If limit > 0 then at most limit entries are returned and at most
// maxExamine entries are looked at, so Get may return early with next
// short of to.
//
// It returns next which is the sequence number to pass as from to
// carry on reading where this call finished, and lost which is the
// number of entries in the range which are no longer in the Buffer,
// either because they were dropped to make space or because they
// were too big to store.
func (b *Buffer) Get(from, to int64, filter Filter, limit int) (entries Entries, next, lost int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Sequence numbers are given out for every log entry, whether
	// it is stored or not, so use that as the end of the log
	end := nextSeq.Load()
	to = min(to, end)
	from = min(max(from, 0), end)
	entries = Entries{}
	next = max(to, from)
	// Find the first entry with seq >= from
	i := sort.Search(len(b.entries), func(i int) bool {
		return b.entries[i].seq >= from
	})
	examined := 0
	for ; i < len(b.entries) && b.entries[i].seq < to; i++ {
		entry := &b.entries[i]
		// Stop when we have enough entries, or have looked at
		// enough of them, so this doesn't hold the lock for
		// too long when the filter matches very little.
		if limit > 0 && (len(entries) >= limit || examined >= maxExamine) {
			next = entry.seq
			break
		}
		examined++
		if filter.match(entry.level, entry.attribution) {
			entries = append(entries, entry.json)
		}
	}
	lost = max(next-from, 0) - int64(examined)
	return entries, next, lost
}

// setBufferSize sets the size of Recent from the options.
func setBufferSize() {
	Recent.SetSize(int64(Opt.BufferSize))
	updateOutput()
}

// nextSeq is the sequence number the next log entry will be given.
var nextSeq atomic.Int64

// makeEntry makes the JSON for a log entry from the JSON formatted
// log text, adding the sequence number to it.
func makeEntry(seq int64, text string) json.RawMessage {
	text = strings.TrimSpace(text)
	if !(strings.HasPrefix(text, `{"`) && strings.HasSuffix(text, "}")) {
		// This shouldn't happen but make sure we always make valid JSON
		msg, _ := json.Marshal(text)
		text = `{"msg":` + string(msg) + `}`
	}
	// Build the entry with the sequence number added in a single allocation
	const prefix = `{"seq":`
	entry := make(json.RawMessage, 0, len(prefix)+20+len(text))
	entry = append(entry, prefix...)
	entry = strconv.AppendInt(entry, seq, 10)
	entry = append(entry, ',')
	entry = append(entry, text[1:]...)
	return entry
}

// logOutput is the log output which feeds the log buffer and the
// core/events subscribers.
//
// This is called with the log handler's mutex held so it must not log.
func logOutput(r slog.Record, attribution Attribution, text string) {
	seq := nextSeq.Add(1) - 1
	entry := makeEntry(seq, text)
	Recent.add(seq, r.Level, attribution, entry)
	publish(r.Level, attribution, entry)
}

// updateOutput connects logOutput to the log handler if the log
// buffer or any subscriber needs it and disconnects it if not.
//
// The output is only connected when needed as it causes every log
// entry to be formatted as JSON.
func updateOutput() {
	outputMu.Lock()
	defer outputMu.Unlock()
	needed := Recent.Enabled() || subscribed()
	if needed && outputRemove == nil {
		outputRemove = Handler.addRecordOutput(true, logOutput)
	} else if !needed && outputRemove != nil {
		outputRemove()
		outputRemove = nil
	}
}

// outputRemove disconnects logOutput from the log handler if not nil.
var (
	outputMu     sync.Mutex
	outputRemove func()
)

func init() {
	rc.Add(rc.Call{
		Path:  "core/log",
		Fn:    rcLog,
		Title: "Returns the most recent log entries.",
		Help: strings.ReplaceAll(`
This returns the most recent log entries from the in memory log buffer.
The log buffer must be enabled with the |--log-buffer-size| flag
otherwise this will return an error.

Note that the log buffer only contains logs which were output at the
current |--log-level|.

Parameters:

- since - return entries with sequence numbers starting from this - optional
    - Defaults to 0 which returns the oldest entries first.
    - If negative, e.g. -N then this starts from the most recent N entries.
    - It is an error for this to be beyond the end of the log (which will
      happen if rclone has been restarted).
- limit - maximum number of entries to return - optional, defaults to 1000, 0 for no limit
- level - only return entries at this level or more severe, e.g. "INFO" - optional
- group - only return entries attributed to this stats group, e.g. "job/1" - optional
- jobid - only return entries attributed to this rc job, e.g. 1 - optional

Returns:

- entries - array of log entries, oldest first.
- next - pass this as |since| to carry on reading the log from where this call finished.
- lost - the number of entries from |since| onwards which had already been dropped from the log buffer.

Each entry is an object in the same format as used by |--use-json-log|
with an additional |seq| entry which is the sequence number of
the log entry. Sequence numbers start from 0 and increase by one for
each log entry.

Entries which rclone can attribute to the rc job which made them, such
as the per file logs made by a job doing a sync or copy, have a
|jobid| entry with the ID of the job and a |group| entry with the name
of its stats group, e.g. "job/1" or the value of |_group| passed to
the rc call.

Not all logs can be attributed - see the [_logs](/rc/#logs) docs for
which can.

Example:

|||sh
rclone rc core/log since=-2
|||

Returns:

|||json
{
    "entries": [
        {
            "level": "notice",
            "msg": "Serving remote control on http://127.0.0.1:5572/",
            "seq": 1,
            "source": "rcserver/rcserver.go:119",
            "time": "2026-09-17T12:39:29.875104149+01:00"
        },
        {
            "level": "info",
            "msg": "Copied (new)",
            "object": "file.txt",
            "objectType": "*local.Object",
            "seq": 2,
            "size": 3,
            "source": "operations/copy.go:385",
            "time": "2026-09-17T12:39:29.960848893+01:00"
        }
    ],
    "lost": 0,
    "next": 3
}
|||

To follow the log, call this repeatedly passing in the |next| value
returned as the |since| parameter. If |lost| is non zero then the log
buffer was too small to hold all the entries since the last call and
that many entries were missed.

Sequence numbers restart from 0 when rclone restarts, but not when the
log buffer is turned off and on again - the entries made while it was
off show up as |lost|.

Note that this filters strictly, so |core/log jobid=N| returns fewer
entries than that job's [_logs](/rc/#logs), which also returns the
entries which aren't attributed to any job.
`, "|", "`"),
	})
}

// ParseLevel parses a log level as used in the rc, e.g. "INFO", into a slog.Level.
func ParseLevel(s string) (slog.Level, error) {
	var level fs.LogLevel
	if err := level.Set(s); err != nil {
		return 0, err
	}
	return fs.LogLevelToSlog(level), nil
}

// rcLog returns the most recent log entries.
func rcLog(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	if !Recent.Enabled() {
		return nil, ErrBufferDisabled
	}
	since, err := in.GetInt64("since")
	if rc.NotErrParamNotFound(err) {
		return nil, err
	}
	seq := Recent.Seq()
	if since < 0 {
		since = max(seq+since, 0)
	} else if since > seq {
		return nil, rc.NewErrParamInvalid(fmt.Errorf("since %d is beyond the end of the log %d - has rclone been restarted?", since, seq))
	}
	limit, err := in.GetInt64("limit")
	if rc.IsErrParamNotFound(err) {
		limit = 1000
	} else if err != nil {
		return nil, err
	}
	level := slog.LevelDebug
	levelString, err := in.GetString("level")
	if err == nil {
		level, err = ParseLevel(levelString)
		if err != nil {
			return nil, rc.NewErrParamInvalid(err)
		}
	} else if rc.NotErrParamNotFound(err) {
		return nil, err
	}
	group, err := in.GetString("group")
	if rc.NotErrParamNotFound(err) {
		return nil, err
	}
	jobID, err := in.GetInt64("jobid")
	if rc.NotErrParamNotFound(err) {
		return nil, err
	}
	filter := Filter{Level: level, Group: group, JobID: jobID}
	entries, next, lost := Recent.Get(since, math.MaxInt64, filter, int(min(limit, math.MaxInt32)))
	return rc.Params{
		"entries": entries,
		"next":    next,
		"lost":    lost,
	}, nil
}
