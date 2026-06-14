---
type: stories
sprint: 1
---

# Sprint 1 Stories — Gmail & Google Calendar Read-Only Backends

## Sprint goal

Ship two new read-only rclone storage backends — `gmailfs` (Gmail messages as `.eml`)
and `gcalfs` (Google Calendar events as `.ics`) — modelled on `backend/googlephotos/`.
Both expose a `dirPattern`-routed virtual filesystem over an OAuth2 connection that uses
the caller's own `client_id`/`client_secret` (no bundled credential), synthesize valid
RFC 2822 / RFC 5545 files on the fly, return `fs.ErrorPermissionDenied` from every write
entry-point, and are registered, documented, and wired into the test harness.

## Sprint demo

A live walkthrough on a configured machine that shows:

1. `rclone lsd gmailfs:2024/01/15` lists thread folders for that day.
2. `rclone copy "gmailfs:2024/01/15/<thread> — <Subject>/<msg> — <Subject>.eml" /tmp/`
   produces a file that `python3 -c "import email; email.message_from_file(open('…eml'))"`
   parses with no exception.
3. `rclone copy "gmailfs:.../attachments/<msg> — document.pdf" /tmp/` yields a
   byte-for-byte correct PDF (base64url decode verified).
4. `rclone lsd gcalfs:` lists one directory per calendar; `rclone copy
   "gcalfs:My Calendar/2024/01/15/<event> — Meeting.ics" /tmp/` yields an `.ics`
   whose every line ends `CRLF` and which imports cleanly into Apple/Google Calendar.
5. `rclone touch gmailfs:2024/01/15/x` and `rclone touch gcalfs:"My Calendar/x"` both
   fail with an error containing `permission denied`.
6. `go build ./...`, `go vet`, and `go test ./backend/gmailfs/ ./backend/gcalfs/
   -run TestIntegration` (which SKIPs without credentials) all pass.

## Definition of Done

A story is DONE only when all of the following hold (gate matrix in `plan.md`
and `standards.md` §3 is authoritative):

- `gofmt -l` reports no files for both backend dirs.
- `go vet ./backend/gmailfs/... ./backend/gcalfs/...` exits 0.
- `golangci-lint run ./backend/gmailfs/... ./backend/gcalfs/...` exits 0.
- `go build ./...` exits 0 (after S1-03 wires the blank imports).
- Every one of `Put`, `Mkdir`, `Rmdir`, `Remove`, `SetModTime` returns
  `fs.ErrorPermissionDenied` in both backends (verified by grep + unit test).
- No hardcoded `ClientID`/`ClientSecret` constant in either backend package.
- gmailfs synthesizes RFC 2822-parseable `.eml`; gcalfs synthesizes RFC 5545
  `.ics` with CRLF line endings throughout.
- Every `dirPattern` tree level has a compiled regex; unmatched paths return
  `fs.ErrorObjectNotFound` (NewObject) / `fs.ErrorDirNotFound` (List), never panic.
- Both backends registered in `backend/all/all.go`; doc pages exist; integration
  tests SKIP cleanly without credentials; `config.yaml` has both entries.
- Unit tests give ≥80% statement coverage on synthesis/encoder/dirPattern logic;
  `go test -race` exits 0 with no DATA RACE output.

## Out of scope

- Any writable surface (upload, mkdir, delete, set-modtime) — explicitly forbidden.
- Bundled or hard-coded OAuth credentials or tokens.
- Gmail label/folder navigation, search syntax beyond the date-range `q:` query.
- Calendar free/busy, ACL, reminders, attendees-as-files, or recurrence-master views
  (`singleEvents=true` flattens recurrence into instances; masters are not exposed).
- Hash computation for synthesized files (always `hash.None`).
- `rclone mount` correctness guarantees that depend on a precomputed `Size`
  (Size -1 is acceptable for `.eml` and `.ics`).
- Incremental/streaming polling, change notifications, or caching beyond the
  per-`Fs` calendar-list cache in gcalfs.
- A shared helper package between the two backends — each is self-contained
  (only `lib/`, `fs/`, and its own `backend/<name>/api/` are imported).

## User stories

### S1-01 — Gmail backend core (`backend/gmailfs/`)

**What's wanted.** A package that builds with `go build ./backend/gmailfs/...` and
implements `fs.Fs` for Gmail:

- OAuth2 via `oauthutil` with caller-supplied `client_id`/`client_secret`; scope
  `https://www.googleapis.com/auth/gmail.readonly`; `oauthConfig` has empty
  `ClientID`/`ClientSecret` (populated from config via `oauthutil.SharedOptions`).
- `dirPattern` tree rooted at the remote:
  ```
  {Year}/
    {Month}/                    # Month formatted YYYY-MM
      {Day}/                    # Day formatted YYYY-MM-DD
        {threadId} — {Subject}/
          {messageId} — {Subject}.eml
          attachments/
            {messageId} — {filename}
  ```
- Day-level `List` queries `threads.list` with
  `q:"after:YYYY/MM/DD before:YYYY/MM/DD+1"`, exhausting all `nextPageToken` pages.
- Thread-level `List` calls `threads.get(threadId, format=FULL)`, returns one `.eml`
  per message plus an `attachments/` sub-dir when any message carries attachments.
- `Open` on `.eml` synthesizes an RFC 2822 / MIME stream on the fly
  (`multipart/alternative` for body alternatives, `multipart/mixed` when attachments
  exist); no temp file.
- `Open` on an attachment calls `messages.attachments.get`, base64url-decodes, and
  returns the raw bytes as an `io.ReadCloser` (streamed, not fully buffered).
- `NewObject` resolves a full path by walking year → month → day → thread →
  (message `.eml` | attachment file).
- `ModTime` = `internalDate` (ms epoch → `time.Time`); `Size` = synthesized byte
  length or -1; attachment `Size` = part `size` (decoded bytes).
- `Hash` = `hash.None`; `MimeType` = `message/rfc822` for `.eml`, the part's
  `Content-Type` for attachments.
- `start_year` option (int, default 2000); root tree omits years before it.
- `fs.Pacer` with `pacer.NewGoogleDrive`.

**Constraints.**
- Must: no bundled credential; all five write ops return `fs.ErrorPermissionDenied`
  in Go (not just docs); `.eml` is RFC 2822/MIME-correct (`MIME-Version`,
  `Content-Type` with boundary, base64/quoted-printable for non-ASCII); every tree
  level has a `dirPattern` regex; unmatched paths return `fs.ErrorObjectNotFound`
  (NewObject) / empty or `fs.ErrorDirNotFound` (List); no import cycles; folder/file
  names sanitized via `encoder.Base | encoder.EncodeCrLf | encoder.EncodeInvalidUtf8`.
- Must not: buffer whole attachment bodies in memory; log `client_id`/`client_secret`;
  expose any writable surface; use a bundled/hard-coded token.

**Failure scenarios.**
- Missing `client_id`/`client_secret` → `NewFs` returns a descriptive error, not a
  nil-deref panic. (Decision: oauthutil surfaces this; gmailfs must wrap, not panic.)
- `2024/01/15/badthread/nomessage.eml` → `NewObject` returns `fs.ErrorObjectNotFound`.
- Missing/misconfigured pacer → HTTP 429 floods or hangs; pacer is mandatory.
- Shared MIME boundary across nested parts or mis-encoded headers → unparseable `.eml`.
- Returning base64-encoded size instead of decoded byte count for attachments →
  wrong transfer stats / failed checks.
- Two threads with identical subject same day → duplicate dir names; `threadId` in
  the folder name guarantees uniqueness (the format already does this).
- A write op returning `nil` → silent data-loss semantics for callers (a defect).

**Success scenarios.**
- `rclone lsd gmailfs:2024/01/15` → one entry per thread, `{threadId} — {Subject}`.
- `rclone ls "gmailfs:2024/01/15/<thread> — <Subject>/"` → `.eml` entries + an
  `attachments/` dir when attachments exist.
- The copied `.eml` parses under `email.message_from_file`.
- The copied attachment is byte-for-byte correct (base64url decode correct).
- Any mutation attempt returns an error containing "permission denied".
- `go build` / `go vet` on `./backend/gmailfs/...` pass.
- `TestIntegration` SKIPs cleanly when `TestGmailFs:` is unconfigured.
- `start_year=2020` → root shows only years ≥ 2020.

**Connections.**
- Uses existing `lib/oauthutil`, `lib/pacer` (`pacer.NewGoogleDrive`), `lib/rest`,
  `lib/encoder`, `fs/hash` — never forks them.
- `dirPattern`/`dirPatterns` modelled on `backend/googlephotos/pattern.go`.
- S1-03 depends on this producing a compilable package with a valid `fs.RegInfo`
  `init()`. No dependency on S1-02.

---

### S1-02 — Google Calendar backend core (`backend/gcalfs/`)

**What's wanted.** A package that builds with `go build ./backend/gcalfs/...` and
implements `fs.Fs` for Google Calendar:

- OAuth2 via `oauthutil` with caller-supplied `client_id`/`client_secret`; scope
  `https://www.googleapis.com/auth/calendar.readonly`; empty `ClientID`/`ClientSecret`
  in `oauthConfig`.
- `dirPattern` tree:
  ```
  {CalendarName}/
    {Year}/
      {Month}/                  # YYYY-MM
        {Day}/                  # YYYY-MM-DD
          {eventId} — {Summary}.ics
  ```
- Root `List` calls `calendarList.list` (Calendar API v3); one directory per calendar
  named by `summary`, with `id` tracked internally in a per-`Fs` cache. When two
  calendars share a `summary`, append ` {calendarId[:8]}` to disambiguate (Decision 4).
- Day-level `List` calls `events.list(calendarId, timeMin=…T00:00:00Z,
  timeMax=…T23:59:59Z, singleEvents=true)`, exhausting all pages; one `.ics` per event;
  empty slice (not error) when a day has no events.
- `Open` on `.ics` synthesizes a minimal valid iCalendar object: `VCALENDAR` wrapper
  with `VERSION:2.0`/`PRODID`, one `VEVENT` with `UID`(=eventId), `DTSTAMP`(=current
  UTC at generation), `DTSTART`/`DTEND`, `SUMMARY`, `DESCRIPTION`, `LOCATION`. CRLF
  line endings, 75-octet line folding. All-day events use `DTSTART;VALUE=DATE` /
  `DTEND;VALUE=DATE`.
- `NewObject` resolves by walking calendar → year → month → day → event file.
- `ModTime` = event `updated` (RFC 3339 → `time.Time`); `Size` = synthesized byte
  length or -1; `Hash` = `hash.None`; `MimeType` = `text/calendar`.
- `start_year` option (int, default 2000); year tree omits years before it.
- `fs.Pacer` with `pacer.NewGoogleDrive`.

**Constraints.**
- Must: no bundled credential; all five write ops return `fs.ErrorPermissionDenied`;
  `.ics` passes RFC 5545 (mandatory props present, lines folded at 75 octets, CRLF
  throughout); calendar names sanitized via the standard encoder; empty day → empty
  slice; all-day events use `VALUE=DATE`; no import cycles.
- Must not: hard-code any calendar ID (all from `calendarList.list`); log
  credentials; expose a writable surface; emit `\n`-only line endings.

**Failure scenarios.**
- Missing credential → descriptive `NewFs` error, not a panic.
- Two calendars with same `summary` → handled by the ` {calendarId[:8]}` suffix
  (Decision 4), not duplicate dirs.
- All-day event rendered as `dateTime` (with `T`/`Z`) instead of `DATE` → calendar
  apps misinterpret it.
- `singleEvents` omitted → recurrence masters appear instead of instances; the
  date tree becomes wrong. `singleEvents=true` is mandatory.
- `My Calendar/2024/01/15/nonexistent — Title.ics` → `fs.ErrorObjectNotFound`.
- `\n`-only `.ics` → import fails / malformed in Apple/Google/Outlook.
- A write op returning `nil` instead of `fs.ErrorPermissionDenied` is a defect.

**Success scenarios.**
- `rclone lsd gcalfs:` → one dir per calendar.
- `rclone lsd "gcalfs:My Calendar/2024/01/"` → one dir per day that has events.
- `rclone ls "gcalfs:My Calendar/2024/01/15/"` → one `.ics` per event.
- The copied `.ics` imports into Apple/Google Calendar without errors.
- All-day events render `DTSTART;VALUE=DATE:YYYYMMDD` (no time component).
- `rclone touch "gcalfs:My Calendar/test"` returns an error containing "permission
  denied".
- `go build` / `go vet` on `./backend/gcalfs/...` pass.
- `TestIntegration` SKIPs cleanly when `TestGcalFs:` is unconfigured.
- `start_year=2022` → year listing under any calendar shows only years ≥ 2022.

**Connections.**
- Uses `lib/oauthutil`, `lib/pacer`, `lib/rest`, `lib/encoder`, `fs/hash`.
- `dirPattern` approach same as S1-01 / googlephotos, but the root segment
  (`{CalendarName}`) is a dynamic capture group rather than a literal like `media/`.
- S1-03 depends on this producing a compilable package with a valid `fs.RegInfo`
  `init()`. No dependency on S1-01.

---

### S1-03 — Registration, docs, and test stubs (both backends)

**What's wanted.** Wire both backends into the binary and test infrastructure:

- `backend/all/all.go`: two blank imports
  (`_ ".../backend/gmailfs"`, `_ ".../backend/gcalfs"`), inserted in alphabetical
  position, without reordering or removing existing entries.
- `docs/content/gmailfs.md` and `docs/content/gcalfs.md`: doc pages following
  `docs/content/googlephotos.md` structure — Hugo front matter, overview,
  configuration (OAuth, `client_id`, `client_secret`, `start_year`), filesystem
  layout, and a Limitations section (read-only, no hash, required credentials).
- `backend/gmailfs/gmailfs_test.go` and `backend/gcalfs/gcalfs_test.go`:
  `TestIntegration` that calls `fstest.Initialise()`, defaults `*fstest.RemoteName`
  to `"TestGmailFs:"` / `"TestGcalFs:"`, calls `fs.NewFs`, and `t.Skipf`s on
  `fs.ErrorNotFoundInConfigFile`; when configured, exercises root list non-empty,
  a day list without error, and all write ops returning an error.
- `fstest/test_all/config.yaml`: two new entries
  (`gmailfs`/`TestGmailFs:`, `gcalfs`/`TestGcalFs:`, `fastlist: false`),
  no duplicates.

**Constraints.**
- Must: `go build ./...` succeeds (blank imports resolve to real packages); both doc
  pages state `client_id`/`client_secret` are required with no default credential;
  both tests guard with `t.Skipf` so CI without credentials never FAILs; doc pages
  do not describe writes as available.
- Must not: add blank imports before the packages compile; reorder/remove existing
  `all.go` entries; duplicate `config.yaml` keys.

**Failure scenarios.**
- Blank import added before the package exists → `go build ./...` fails for the whole
  binary. (Ordering: S1-03's `all.go` edit lands only after S1-01 and S1-02 build.)
- Test calls `require.NoError` before the `fs.ErrorNotFoundInConfigFile` check → CI
  FAILs for anyone without credentials.
- Doc page omits the read-only limitation → users waste time attempting writes.
- Duplicate `config.yaml` entry → `test_all` runs a backend twice or errors at start.

**Success scenarios.**
- `go build ./...` from the repo root succeeds with both packages present.
- `go test ./backend/gmailfs/ -run TestIntegration` and the gcalfs equivalent output
  SKIP (not FAIL) without credentials.
- Both doc pages exist with overview, configuration, layout, and limitations sections.
- `grep -c 'gmailfs\|gcalfs' backend/all/all.go` returns 2.
- `grep 'TestGmailFs\|TestGcalFs' fstest/test_all/config.yaml` returns both entries.

**Connections.**
- Depends on S1-01 and S1-02 both being `go build`-passing packages — cannot merge
  before both compile.
- Each backend's `fs.RegInfo.Prefix` must match the `config.yaml` remote prefixes
  (`gmailfs`, `gcalfs`).
- Doc page structure follows `docs/content/googlephotos.md` (Hugo front matter,
  section conventions).

## Story dependency graph

```
        ┌─────────────┐        ┌─────────────┐
        │   S1-01     │        │   S1-02     │
        │  gmailfs    │        │   gcalfs    │
        │  (Wave 1)   │        │  (Wave 1)   │
        └──────┬──────┘        └──────┬──────┘
               │                      │
               └──────────┬───────────┘
                          ▼
                   ┌─────────────┐
                   │   S1-03     │
                   │ registration│
                   │  (Wave 2)   │
                   └─────────────┘
```

- **S1-01** and **S1-02** are independent — they share no source, import only
  `lib/`, `fs/`, and their own `api/` sub-package, and may be built in parallel.
- **S1-03** depends on BOTH S1-01 and S1-02: its `all.go` blank imports and
  `go build ./...` gate require both packages to compile first.
