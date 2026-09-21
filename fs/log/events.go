package log

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
)

// Number of frames buffered for each subscriber before they get dropped.
const subscriberBacklog = 1024

// newline ends each frame in the stream
var newline = []byte{'\n'}

// subscriber receives log entries as they are made.
type subscriber struct {
	filter  Filter
	frames  chan json.RawMessage
	mu      sync.Mutex
	dropped int64 // frames dropped because frames was full
}

// subscribers receiving log entries
var (
	subscribersMu sync.Mutex
	subscribers   = map[*subscriber]struct{}{}
)

// subscribed returns whether anything is subscribed to the log.
func subscribed() bool {
	subscribersMu.Lock()
	defer subscribersMu.Unlock()
	return len(subscribers) > 0
}

// subscribe returns a subscriber receiving the log entries which
// match filter and a function to unsubscribe it again.
func subscribe(filter Filter) (*subscriber, func()) {
	s := &subscriber{
		filter: filter,
		frames: make(chan json.RawMessage, subscriberBacklog),
	}
	subscribersMu.Lock()
	subscribers[s] = struct{}{}
	subscribersMu.Unlock()
	updateOutput()
	return s, func() {
		subscribersMu.Lock()
		delete(subscribers, s)
		subscribersMu.Unlock()
		updateOutput()
	}
}

// takeDropped returns the number of frames dropped since it was last called.
func (s *subscriber) takeDropped() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	dropped := s.dropped
	s.dropped = 0
	return dropped
}

// send queues frame for the subscriber, dropping it if it isn't
// keeping up. It never blocks as it is called while logging.
//
// Any frames dropped earlier are reported first so the client is
// told where the gap in the stream is.
func (s *subscriber) send(frame json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dropped > 0 {
		select {
		case s.frames <- droppedFrame(s.dropped):
			s.dropped = 0
		default:
			s.dropped++
			return
		}
	}
	select {
	case s.frames <- frame:
	default:
		s.dropped++
	}
}

// logFrame makes the frame sent to the subscribers for a log entry.
func logFrame(entry json.RawMessage) json.RawMessage {
	const prefix = `{"type":"log",`
	frame := make(json.RawMessage, 0, len(prefix)+len(entry))
	frame = append(frame, prefix...)
	frame = append(frame, entry[1:]...)
	return frame
}

// publish sends a log entry to the subscribers which want it.
//
// This is called with the log handler's mutex held so it must not log
// or block.
func publish(level slog.Level, attribution Attribution, entry json.RawMessage) {
	subscribersMu.Lock()
	defer subscribersMu.Unlock()
	var frame json.RawMessage
	for s := range subscribers {
		if !s.filter.match(level, attribution) {
			continue
		}
		if frame == nil {
			frame = logFrame(entry)
		}
		s.send(frame)
	}
}

func init() {
	rc.Add(rc.Call{
		Path:           "core/events",
		Fn:             rcEvents,
		NeedsResponse:  true,
		WritesResponse: true,
		Title:          "Streams events, such as the logs, as they happen.",
		Help: strings.ReplaceAll(`
This streams events as they happen until the connection is closed. It
is only available over HTTP.

Events are returned as newline delimited JSON objects, one per line,
each with a |type| saying what it is. The types are:

- |start| - sent when the stream starts
    - seq - the sequence number the next log entry will be given
    - jobid - the rc job of the stream, which [job/stop](#job-stop) stops
- |log| - a log entry in the format used by [core/log](#core-log)
- |dropped| - sent if the client isn't reading the stream fast enough
    - count - the number of events dropped
- |heartbeat| - sent when the stream is idle so it doesn't time out

Unlike [core/log](#core-log) this doesn't need |--log-buffer-size| to
be set as the events are not stored - they are sent as they happen.

Parameters:

- level - only stream logs at this level or more severe, e.g. "INFO" - optional
- group - only stream logs attributed to this stats group, e.g. "job/1" - optional
- jobid - only stream logs attributed to this rc job, e.g. 1 - optional
- unattributed - if set with group or jobid, also stream the logs which
  aren't attributed to a job - optional
- since - send the entries from this sequence number before streaming
  the new ones - optional
    - This needs the log buffer to be enabled with |--log-buffer-size|.
    - Use it with the |seq| from the last entry seen to carry on where a
      previous stream left off.
- heartbeat - seconds between heartbeats, 0 to disable - optional, defaults to 30

Example:

|||sh
curl -u user:pass -N -X POST http://127.0.0.1:5572/core/events
|||

Returns

|||json
{"type":"start","seq":42}
{"type":"log","seq":42,"time":"2026-09-21T12:16:41.134507+01:00","level":"info","msg":"Copied (new)","object":"file.txt","objectType":"*local.Object","size":3,"source":"operations/copy.go:385","jobid":1,"group":"job/1"}
{"type":"heartbeat","time":"2026-09-21T12:17:11.134507+01:00"}
|||

Note that the rc calls made by the client reading the stream are
logged at DEBUG level like any other, so a client streaming at that
level sees its own polling. Use |--rc-log-calls=false| to turn that
off.
`, "|", "`"),
	})
}

// rcEvents streams the events to the HTTP response until the
// connection is closed.
func rcEvents(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	w, err := in.GetHTTPResponseWriter()
	if err != nil {
		return nil, fmt.Errorf("response object is required: %w", err)
	}
	filter, err := eventsFilter(in)
	if err != nil {
		return nil, err
	}
	since, err := in.GetInt64("since")
	if rc.NotErrParamNotFound(err) {
		return nil, err
	}
	replay := err == nil
	if replay && !Recent.Enabled() {
		return nil, ErrBufferDisabled
	}
	heartbeat := 30 * time.Second
	if seconds, err := in.GetInt64("heartbeat"); err == nil {
		heartbeat = time.Duration(seconds) * time.Second
	} else if rc.NotErrParamNotFound(err) {
		return nil, err
	}

	// Subscribe before reading the buffer so no entry is missed
	// between the two - the ones sent from the buffer are skipped
	// from the stream below.
	s, unsubscribe := subscribe(filter)
	defer unsubscribe()
	startSeq := Recent.Seq()

	// The write deadline is cleared as this runs until the client
	// goes away, then set for each write so a stuck client doesn't
	// block forever.
	rcw := http.NewResponseController(w)
	if err := rcw.SetWriteDeadline(time.Time{}); err != nil {
		return nil, fmt.Errorf("can't stream events: %w", err)
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	// The frames are shared between the subscribers so they must
	// not be written to - the newline goes in a separate write.
	write := func(frame json.RawMessage) error {
		if err := rcw.SetWriteDeadline(time.Now().Add(time.Minute)); err != nil {
			return err
		}
		if _, err := w.Write(frame); err != nil {
			return err
		}
		if _, err := w.Write(newline); err != nil {
			return err
		}
		return rcw.Flush()
	}

	start := `{"type":"start","seq":` + strconv.FormatInt(startSeq, 10)
	if jobID, ok := fs.JobIDFromContext(ctx); ok {
		start += `,"jobid":` + strconv.FormatInt(jobID, 10)
	}
	if err := write(json.RawMessage(start + `}`)); err != nil {
		return nil, nil
	}

	if replay {
		entries, _, lost := Recent.Get(since, startSeq, filter, 0)
		if lost > 0 {
			if err := write(droppedFrame(lost)); err != nil {
				return nil, nil
			}
		}
		for _, entry := range entries {
			if err := write(logFrame(entry)); err != nil {
				return nil, nil
			}
		}
	}

	var ticker *time.Ticker
	tick := make(<-chan time.Time)
	if heartbeat > 0 {
		ticker = time.NewTicker(heartbeat)
		defer ticker.Stop()
		tick = ticker.C
	}
	for {
		select {
		case <-ctx.Done():
			return nil, nil
		case frame := <-s.frames:
			// Skip the entries already sent from the buffer
			if replay && frameSeq(frame) < startSeq {
				continue
			}
			if err := write(frame); err != nil {
				return nil, nil
			}
		case now := <-tick:
			// Report any drops which the log going quiet left
			// unreported
			if dropped := s.takeDropped(); dropped > 0 {
				if err := write(droppedFrame(dropped)); err != nil {
					return nil, nil
				}
			}
			frame := `{"type":"heartbeat","time":` + strconv.Quote(now.Format(time.RFC3339Nano)) + `}`
			if err := write(json.RawMessage(frame)); err != nil {
				return nil, nil
			}
		}
	}
}

// droppedFrame makes a frame saying count events were dropped.
func droppedFrame(count int64) json.RawMessage {
	return json.RawMessage(`{"type":"dropped","count":` + strconv.FormatInt(count, 10) + `}`)
}

// frameSeq returns the sequence number of a log frame or -1 if it doesn't have one.
func frameSeq(frame json.RawMessage) int64 {
	var entry struct {
		Seq *int64 `json:"seq"`
	}
	if err := json.Unmarshal(frame, &entry); err != nil || entry.Seq == nil {
		return -1
	}
	return *entry.Seq
}

// eventsFilter reads the log filter from the parameters.
func eventsFilter(in rc.Params) (filter Filter, err error) {
	filter.Level = slog.LevelDebug
	levelString, err := in.GetString("level")
	if err == nil {
		filter.Level, err = ParseLevel(levelString)
		if err != nil {
			return filter, rc.NewErrParamInvalid(err)
		}
	} else if rc.NotErrParamNotFound(err) {
		return filter, err
	}
	filter.Group, err = in.GetString("group")
	if rc.NotErrParamNotFound(err) {
		return filter, err
	}
	filter.JobID, err = in.GetInt64("jobid")
	if rc.NotErrParamNotFound(err) {
		return filter, err
	}
	filter.Unattributed, err = in.GetBool("unattributed")
	if rc.NotErrParamNotFound(err) {
		return filter, err
	}
	return filter, nil
}
