---
type: validate
story: S1-02
---

# S1-02 Validate — Google Calendar backend core

Literal PASS/FAIL checks, one block per task in `tasks.md`. Run from
`/Users/adeelahmad/work/rclone`. `export PATH="$HOME/.local/bin:$PATH"` first.

## Pre-flight

- `go version` → `go1.25` or newer. FAIL: wrong toolchain.
- `test -f backend/gcalfs/gcalfs.go` → exit 0 once T01 lands. FAIL: skeleton missing.
- `grep -q 'pacer.NewGoogleDrive' backend/gcalfs/gcalfs.go` → exit 0 after T01. FAIL:
  pacer policy missing (P-1).

### T01 — skeleton / OAuth / NewFs
- `go build ./backend/gcalfs/...` → exit 0. FAIL: compile error.
- `grep -rnE 'rcloneClientID|rcloneClientSecret|ClientID\s*=\s*"[A-Za-z0-9]' backend/gcalfs/`
  → no output, exit 1. FAIL (no-bundled-creds, O-1).
- `grep -q 'calendar.readonly' backend/gcalfs/gcalfs.go` → exit 0. FAIL: wrong/missing
  scope (O-3).
- `grep -q 'oauthutil.SharedOptions' backend/gcalfs/gcalfs.go` → exit 0. FAIL (O-1).
- `grep -q 'ReadMimeType' backend/gcalfs/gcalfs.go` → exit 0. FAIL (R-2).
- Unit: `go test ./backend/gcalfs/ -run TestNewFs_RequiresClientID` → PASS; empty
  client_id returns a non-nil error, no panic. FAIL: panic or nil error.

### T02 — dirPattern tree
- `go vet ./backend/gcalfs/...` → exit 0. FAIL: vet error.
- Unit: `go test ./backend/gcalfs/ -run TestPatternMatch` → PASS; every level
  (`""`, `<cal>`, `<cal>/2024`, `<cal>/2024/2024-01`, `<cal>/2024/2024-01/2024-01-15`,
  `.ics`) matches with the right `isFile` flag; dynamic root capture group captures the
  calendar segment; unmatched path → nil pattern. FAIL (D-1/D-2/D-3).

### T03 — root List / caching / disambiguation
- Unit (mock `calendarList.list`): `go test ./backend/gcalfs/ -run TestRootList_OneDirPerCalendar`
  → PASS; one dir per calendar named by `summary`. FAIL: wrong count/name.
- Unit: `go test ./backend/gcalfs/ -run TestRootList_PaginationExhausted` → PASS;
  multi-page calendar list returns all calendars. FAIL: only first page.
- Unit: `go test ./backend/gcalfs/ -run TestRootList_DisambiguatesDuplicateSummary` →
  PASS; two calendars sharing a summary get ` {calendarId[:8]}` suffixes (Decision 4).
  FAIL: duplicate dir names.
- Unit: `go test ./backend/gcalfs/ -run TestCache_ResolvesNameToCalendarID` → PASS; the
  cache maps a directory-name segment back to the calendar ID. FAIL: unresolved.

### T04 — day List
- Unit (mock `events.list`): `go test ./backend/gcalfs/ -run TestDayList_OneIcsPerEvent`
  → PASS; one `<id> — <Summary>.ics` per event. FAIL: wrong count/name.
- Unit: `go test ./backend/gcalfs/ -run TestDayList_SendsSingleEvents` → PASS; the
  request carries `singleEvents=true`. FAIL: missing param.
- Unit: `go test ./backend/gcalfs/ -run TestDayList_PaginationExhausted` → PASS;
  two-page event list returns all events. FAIL: only first page.
- Unit: `go test ./backend/gcalfs/ -run TestDayList_EmptyDayEmptySlice` → PASS; a day
  with no events returns an empty slice, no error. FAIL: returns error.

### T05 — NewObject
- Unit: `go test ./backend/gcalfs/ -run TestNewObject_ResolvesIcs` → PASS; a valid
  `.ics` path resolves to an Object whose Remote() matches. FAIL: error on valid path.
- Unit: `go test ./backend/gcalfs/ -run TestNewObject_DirectoryReturnsObjectNotFound`
  → PASS; `NewObject("My Calendar/2024")` returns `fs.ErrorObjectNotFound` (Decision 5).
  FAIL: wrong error / panic.
- Unit: `go test ./backend/gcalfs/ -run TestNewObject_BadPathNotFound` → PASS; a
  nonexistent event path returns `fs.ErrorObjectNotFound`. FAIL: panic.

### T06 — `.ics` synthesis
- Unit: `go test ./backend/gcalfs/ -run TestIcsSynthesis_AllLinesCRLF` → PASS; every
  line ends `\r\n`; the equivalent of `grep -cP '\r\n' fixture.ics` equals
  `wc -l fixture.ics` (ics-crlf gate, S-2). FAIL: any `\n`-only line.
- Unit: `go test ./backend/gcalfs/ -run TestIcsSynthesis_MandatoryProperties` → PASS;
  output contains `BEGIN:VCALENDAR`, `VERSION:2.0`, `PRODID`, `BEGIN:VEVENT`, `UID`,
  `DTSTAMP`, `DTSTART`, `DTEND`, `SUMMARY`, `END:VEVENT`, `END:VCALENDAR`. FAIL: missing.
- Unit: `go test ./backend/gcalfs/ -run TestIcsSynthesis_LineFoldingAt75Octets` → PASS;
  a long DESCRIPTION is folded with CRLF + single space at ≤75 octets. FAIL: unfolded.
- Unit: `go test ./backend/gcalfs/ -run TestIcsSynthesis_AllDayUsesValueDate` → PASS; an
  all-day event renders `DTSTART;VALUE=DATE:YYYYMMDD` (no `T`/`Z`). FAIL: dateTime form.
- Unit: `go test ./backend/gcalfs/ -run TestIcsSynthesis_TimedUsesUTC` → PASS; a timed
  event renders `DTSTART:YYYYMMDDTHHMMSSZ`. FAIL: wrong form.
- Unit: `go test ./backend/gcalfs/ -run TestIcsSynthesis_DeterministicExceptDtstamp` →
  PASS; same payload → identical bytes apart from DTSTAMP (S-3). FAIL: other drift.

### T07 — metadata
- Unit: `go test ./backend/gcalfs/ -run TestObject_ModTimeFromUpdated` → PASS; ModTime
  equals the event `updated` RFC 3339 value. FAIL: wrong time.
- Unit: `go test ./backend/gcalfs/ -run TestObject_HashNone` → PASS; `Hash` returns
  `"", hash.ErrUnsupported`; `Fs.Hashes()` == `hash.Set(hash.None)` (R-3). FAIL: other.
- Unit: `go test ./backend/gcalfs/ -run TestObject_MimeTypeCalendar` → PASS; MimeType is
  `text/calendar`. FAIL: wrong mime.

### T08 — read-only enforcement
- `grep -n 'ErrorPermissionDenied' backend/gcalfs/*.go` → at least 5 matches across
  Put/Mkdir/Rmdir/Remove/SetModTime (write-ops gate, R-1). FAIL: any missing.
- Unit: `go test ./backend/gcalfs/ -run TestReadOnly_AllWriteOpsDenied` → PASS; each of
  Put, Mkdir, Rmdir, Remove, SetModTime returns exactly `fs.ErrorPermissionDenied`. FAIL.

## Final sign-off

All checks above PASS, plus the per-task gates from `standards.md` §3 applied to S1-02:

- `gofmt -l ./backend/gcalfs/` → empty output.
- `go vet ./backend/gcalfs/...` → exit 0.
- `golangci-lint run ./backend/gcalfs/...` → exit 0.
- write-ops gate: `grep -n "ErrorPermissionDenied" backend/gcalfs/*.go` shows all five.
- no-bundled-creds gate: credential grep returns nothing.
- ics-crlf gate: a synthesized `.ics` fixture has every line ending CRLF
  (`grep -cP '\r\n' fixture.ics` == `wc -l fixture.ics`).
- `go test -race -cover ./backend/gcalfs/...` → exit 0, ≥80% on synthesis/encoder/
  pattern logic, no DATA RACE.

S1-02 is DONE only when every box above is checked and no FAIL remains.
