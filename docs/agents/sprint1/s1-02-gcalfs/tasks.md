---
type: tasks
story: S1-02
---

# S1-02 Tasks — Google Calendar backend core (`backend/gcalfs/`)

Atomic, independently testable tasks for the Calendar backend, each citing the
`standards.md` rules it satisfies. Production symbols land in
`backend/gcalfs/gcalfs.go`, `backend/gcalfs/pattern.go`, `backend/gcalfs/api/types.go`.

## T01 — Package skeleton, OAuth registration, Options/Fs/Object, NewFs

**Scope.** Create the `gcalfs` package; `init()` registers
`fs.RegInfo{Name:"calendar", Prefix:"gcalfs", Description:"Google Calendar",
NewFs:NewFs}` with `Options: append(oauthutil.SharedOptions, …)` adding `start_year`
(int, default 2000) and the standard `encoding` option (`encoder.Base |
encoder.EncodeCrLf | encoder.EncodeInvalidUtf8`). Define `oauthConfig` with empty
`ClientID`/`ClientSecret`, scope `https://www.googleapis.com/auth/calendar.readonly`,
Google auth/token URLs. Define `Options`, `Fs` (name, root, opt, features, srv
`*rest.Client`, ts, pacer `pacer.NewGoogleDrive`, startTime, calendarsMu, calendars
cache map), and `Object` (fs, remote, calendarID, eventID, summary, description,
location, start, end, allDay bool, updated, mimeType). Implement `NewFs` (parse
config, oauth client via `oauthutil.NewClientWithBaseClient`, trim root,
`Features{ReadMimeType:true}`, `ErrorIsFile` probe), `Name`, `Root`, `String`,
`Features`, `Precision` (=`fs.ModTimeNotSupported`), `Hashes` (=`hash.Set(hash.None)`),
`dirTime`, `startYear`.

**Rules.** O-1, O-2, O-3, R-2, R-3, E-1, P-1, F-1..F-5.
**Done when.** `go build ./backend/gcalfs/...` compiles; `NewFs` returns a descriptive
error (not panic) when `client_id`/`client_secret` absent; no hardcoded credential.

## T02 — dirPattern tree

**Scope.** `backend/gcalfs/pattern.go`: `dirPattern`/`dirPatterns`, `mustCompile`,
`match`. The root segment is a DYNAMIC calendar-name capture group (unlike
googlephotos' literal `media/`):
- `^$` → calendar dirs (from `calendarList.list`), `isFile:false`
- `^([^/]+)$` → year dirs for that calendar (start_year..current), `isFile:false`
- `^[^/]+/(\d{4})$` → month dirs `YYYY-MM`
- `^[^/]+/\d{4}/(\d{4})-(\d{2})$` → day dirs `YYYY-MM-DD`
- `^[^/]+/\d{4}/\d{4}-\d{2}/(\d{4})-(\d{2})-(\d{2})$` → event-file dirs (day-level List)
- `^[^/]+/\d{4}/\d{4}-\d{2}/\d{4}-\d{2}-\d{2}/([^/]+)\.ics$` → `.ics` leaf, `isFile:true`

**Rules.** D-1, D-2, D-3.
**Done when.** Every level has a compiled regex; the dynamic root capture is correct;
`match` returns `(nil,"",nil)` for unmatched paths and distinguishes `isFile`.

## T03 — Root List, calendar caching, disambiguation

**Scope.** Root `List("")` calls `calendarList.list`, exhausting all pages; builds a
per-`Fs` cache mapping the directory name → calendar ID. Directory name = `summary`,
passed through `f.opt.Enc.FromStandardPath`. When two calendars share a `summary`,
append ` {calendarId[:8]}` to BOTH colliding names to disambiguate (Decision 4). The
cache also maps name → ID so subsequent levels can resolve the calendar ID from the
path segment. Year/month/day synthetic levels generate dir entries (years/months/days
helpers) under a resolved calendar.

**Rules.** D-1, D-2, E-1, P-1, sprint must-not (no hard-coded calendar ID).
**Done when.** One dir per calendar; collisions disambiguated; the cache resolves a
directory-name segment back to its calendar ID; calendar ID never hard-coded.

## T04 — Day List (events.list, pagination)

**Scope.** Day-level `List` resolves the calendar ID from the path's first segment via
the cache, then calls `events.list(calendarId, timeMin=YYYY-MM-DDT00:00:00Z,
timeMax=YYYY-MM-DDT23:59:59Z, singleEvents=true)`, exhausting all `nextPageToken`
pages (Decision 3). Returns one `{eventId} — {Summary}.ics` entry per event, names
encoded. A day with no events returns an empty slice (not an error).

**Rules.** D-1, D-2, E-1, P-1; sprint must (singleEvents=true).
**Done when.** `singleEvents=true` is sent; pagination exhausts pages; empty day →
empty slice; one `.ics` per event.

## T05 — NewObject path resolution

**Scope.** `NewObject(ctx, remote)`: match with `isFile=true`. No file pattern match →
`fs.ErrorObjectNotFound` (Decision 5). Parse calendar segment, date, and event ID from
the path; resolve calendar ID via the cache; build an `Object`. A path whose IDs don't
resolve → `fs.ErrorObjectNotFound`.

**Rules.** D-2, D-3.
**Done when.** Valid `.ics` paths resolve to an Object; invalid or directory paths
return `fs.ErrorObjectNotFound`; never panics.

## T06 — Open for `.ics` (RFC 5545 synthesis)

**Scope.** `Object.Open` for `.ics`: fetch the event via `events.get` (or reuse the
list payload), synthesize a minimal valid iCalendar object — `BEGIN:VCALENDAR`,
`VERSION:2.0`, `PRODID`, one `VEVENT` with `UID`(=eventId), `DTSTAMP`(=current UTC at
generation), `DTSTART`/`DTEND`, `SUMMARY`, `DESCRIPTION`, `LOCATION`, `END:VEVENT`,
`END:VCALENDAR`. CRLF (`\r\n`) line endings throughout (Decision 8). Lines exceeding
75 octets folded with CRLF + single space. All-day events (which carry `date`, not
`dateTime`) render `DTSTART;VALUE=DATE:YYYYMMDD` / `DTEND;VALUE=DATE:YYYYMMDD`;
timed events render `DTSTART:YYYYMMDDTHHMMSSZ`. Streamed on the fly; deterministic
for a given payload (DTSTAMP is the one allowed time-varying field).

**Rules.** S-2, S-3.
**Done when.** Every line ends CRLF; mandatory props present; line folding at 75
octets; all-day events use `VALUE=DATE`; output imports cleanly.

## T07 — Object metadata

**Scope.** `ModTime` = event `updated` (RFC 3339 → `time.Time`). `Size` = synthesized
byte length or -1 (Decision 2). `Hash` = `"", hash.ErrUnsupported`; `Fs.Hashes` =
`hash.Set(hash.None)`. `MimeType` = `text/calendar`. `Storable` = true.

**Rules.** R-3.
**Done when.** Each accessor returns the specified value.

## T08 — Read-only enforcement

**Scope.** `Put`, `Mkdir`, `Rmdir`, `Remove`, `SetModTime` each return
`fs.ErrorPermissionDenied` with no side effects.

**Rules.** R-1.
**Done when.** `grep -n ErrorPermissionDenied backend/gcalfs/*.go` shows all five;
unit tests confirm each returns exactly `fs.ErrorPermissionDenied`.
