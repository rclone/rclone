<!-- intake · sprint 1 · validated by human approval + bin/gate-intake (not planning-artifacts.kdl) -->

# Sprint 1 Intake — Gmail and Google Calendar Read-Only Backends

## Restatement

Build two new read-only rclone storage backends in Go — `gmailfs` and `gcalfs` — that expose Gmail messages and Google Calendar events as navigable virtual filesystems. Both backends follow the pattern established by `backend/googlephotos/` (dirPattern tree, oauthutil OAuth flow, rest.Client, fs.Pacer). Neither backend exposes any writable surface; all mutation entry-points return `fs.ErrorPermissionDenied` at the Go level. Users supply their own OAuth2 `client_id` and `client_secret`; no bundled credential is acceptable because these are restricted-scope apps. Both backends are registered in `backend/all/all.go`, documented under `docs/content/`, tested with an integration test that skips when the remote is unconfigured, and added to `fstest/test_all/config.yaml`.

---

## S1-01 — Gmail Backend Core (`gmailfs`)

### What's wanted

A compilable, passing `go build ./backend/gmailfs/...` package that implements the rclone `fs.Fs` interface for Gmail, with:

- OAuth2 via `oauthutil` using the caller-supplied `client_id`/`client_secret`; scope `https://www.googleapis.com/auth/gmail.readonly`; no bundled credential.
- A `dirPattern`-based virtual tree rooted at the configured remote:
  ```
  {Year}/
    {Month}/
      {Day}/
        {threadId} — {Subject}/
          {messageId} — {Subject}.eml
          attachments/
            {messageId} — {filename}
  ```
- `List` at the day level queries `threads.list` with `q:"after:YYYY/MM/DD before:YYYY/MM/DD+1"`.
- `List` at the thread level calls `threads.get(threadId, format=FULL)` and returns per-message `.eml` entries plus an `attachments/` sub-dir when any message carries attachments.
- `Open` on an `.eml` synthesizes an RFC 2822 stream (MIME-correct: `multipart/alternative` for body alternatives, `multipart/mixed` when attachments are present) from the Gmail message payload. The stream is generated on-the-fly; no temp file on disk.
- `Open` on an attachment calls `messages.attachments.get`, base64url-decodes the response, and returns the raw bytes as an `io.ReadCloser`.
- `NewObject` resolves a full remote path to the correct `Object` by walking path segments: year → month → day → thread → (message | attachment file).
- `ModTime` for `.eml` = `internalDate` (milliseconds since epoch → `time.Time`). `ModTime` for an attachment = same message's `internalDate`.
- `Size` for `.eml` = synthesized byte length; may return -1 if unknown before full generation. `Size` for attachment = `size` field from the Gmail part metadata returned by `threads.get`.
- `Hash` returns `hash.None` for all objects.
- `MimeType` returns `message/rfc822` for `.eml`; the `Content-Type` from the Gmail part header for attachments.
- `Put`, `Mkdir`, `Rmdir`, `Remove`, `SetModTime` all return `fs.ErrorPermissionDenied`.
- `start_year` config option (integer, default 2000); the year tree does not list years before `start_year`.
- `fs.Pacer` with `pacer.NewGoogleDrive` policy for rate-limiting API calls.

### Constraints

- Must-haves:
  - No bundled OAuth credential. The `oauthutil.Config` `ClientID` and `ClientSecret` fields must be populated from the user's config, not from constants in the source.
  - All five write entry-points (`Put`, `Mkdir`, `Rmdir`, `Remove`, `SetModTime`) must return `fs.ErrorPermissionDenied` — not just be undocumented or unimplemented with `nil` returns.
  - The `.eml` synthesis must produce valid RFC 2822 / MIME output parseable by standard mail clients. Specifically: correct `MIME-Version`, `Content-Type` with boundary, and base64 or quoted-printable encoding for non-ASCII body parts.
  - `dirPattern` regex set must cover all tree levels and file patterns; unrecognized paths must return `fs.ErrorObjectNotFound` from `NewObject` and an empty listing from `List`.
  - The package must compile with no import cycles within the rclone module.
  - Thread and message IDs may contain characters that are invalid in filesystem paths on some platforms; the backend must sanitize folder/file names (replace `/` at minimum) following rclone encoder conventions.

- Must-nots:
  - Must not buffer entire attachment bodies in memory; stream from API response directly.
  - Must not expose the Gmail `client_id`/`client_secret` in log output at any log level.
  - Must not implement any writable surface, even behind a flag.
  - Must not use a bundled or hard-coded OAuth token.

### Failure scenarios

- **Credential omission**: If `client_id` or `client_secret` is absent from config, `NewFs` must return a descriptive error rather than a nil-dereference panic or an unhelpful API error.
- **Invalid path resolution**: A path like `2024/01/15/badthread/nomessage.eml` must return `fs.ErrorObjectNotFound` from `NewObject` rather than a nil `Object` or a panic.
- **API rate limit**: The pacer is misconfigured or absent — calls may get HTTP 429 responses and flood logs, or the backend hangs indefinitely.
- **Malformed `.eml` synthesis**: If `multipart` boundary is shared across nested parts or headers are encoded incorrectly, the synthesized `.eml` will fail to parse in standard mail clients.
- **Attachment size mismatch**: Returning the wrong `Size` (e.g., the base64-encoded size instead of the decoded byte count) causes rclone copy to report incorrect transfer statistics or fail hash checks.
- **Thread name collision**: Two threads with identical subjects on the same day produce duplicate directory names. The implementation must ensure uniqueness by including `threadId` in the folder name (which the specified format already does, but any deviation breaks this).
- **Write operation silently succeeds**: If a write method returns `nil` instead of `fs.ErrorPermissionDenied`, callers may believe mutations succeeded, leading to silent data-loss semantics from the caller's perspective.

### Success scenarios

- `rclone lsd gmailfs:2024/01/15` returns a list of thread folders for that date, one entry per thread, each named `{threadId} — {Subject}`.
- `rclone ls gmailfs:2024/01/15/some-thread-id — Some Subject/` returns `.eml` entries and an `attachments/` dir when attachments exist.
- `rclone copy gmailfs:2024/01/15/some-thread-id — Some Subject/msg-id — Some Subject.eml /tmp/` produces a file parseable by `python3 -c "import email, sys; email.message_from_file(sys.stdin)"`.
- `rclone copy gmailfs:.../attachments/msg-id — document.pdf /tmp/` produces a byte-for-byte correct PDF (i.e., base64url decoding is correct).
- `rclone touch gmailfs:2024/01/15/test` or any mutation attempt returns an error containing "permission denied".
- `go build ./backend/gmailfs/...` and `go vet ./backend/gmailfs/...` pass with no errors.
- The integration test (`TestIntegration`) in `backend/gmailfs/gmailfs_test.go` skips cleanly when no `TestGmailFs:` remote is configured.
- `start_year=2020` config causes the root listing to show only years >= 2020.

### Connections

- Depends on existing `lib/oauthutil` for OAuth2 token acquisition and storage — must not fork or duplicate this package.
- Depends on `lib/pacer` (`pacer.NewGoogleDrive`) for rate-limiting, same as `backend/googlephotos/`.
- Depends on `lib/rest` for HTTP client construction.
- The `dirPattern` type and approach is modeled on `backend/googlephotos/pattern.go`; the implementation must follow the same interface so future maintainers recognize the idiom.
- S1-03 (registration and docs) depends on this story completing a compilable package with a valid `fs.RegInfo` `init()` block.
- No dependency on S1-02 (gcalfs).

---

## S1-02 — Google Calendar Backend Core (`gcalfs`)

### What's wanted

A compilable, passing `go build ./backend/gcalfs/...` package that implements the rclone `fs.Fs` interface for Google Calendar, with:

- OAuth2 via `oauthutil` using caller-supplied `client_id`/`client_secret`; scope `https://www.googleapis.com/auth/calendar.readonly`; no bundled credential.
- A `dirPattern`-based virtual tree:
  ```
  {CalendarName}/
    {Year}/
      {Month}/
        {Day}/
          {eventId} — {Summary}.ics
  ```
- Root `List` calls `calendarList.list` (Google Calendar API v3) and returns one directory entry per calendar. The calendar's `summary` field is used as the directory name; the calendar's `id` is tracked internally for subsequent API calls.
- `List` at the day level calls `events.list(calendarId, timeMin=YYYY-MM-DDT00:00:00Z, timeMax=YYYY-MM-DDT23:59:59Z, singleEvents=true)` and returns one `.ics` file entry per event.
- `Open` on a `.ics` file synthesizes a minimal valid iCalendar document: `VCALENDAR` wrapper containing one `VEVENT` with `DTSTART`, `DTEND`, `SUMMARY`, `DESCRIPTION`, `LOCATION`, `UID` (= `eventId`), `DTSTAMP` (= current UTC time at generation), and `VERSION:2.0` / `PRODID`. The stream is generated on-the-fly.
- `NewObject` resolves a full remote path to the correct `Object` by walking: calendar name → year → month → day → event file.
- `ModTime` for a `.ics` = the event's `updated` field (RFC 3339 → `time.Time`).
- `Size` = synthesized byte length of the iCalendar document; -1 is acceptable.
- `Hash` returns `hash.None`.
- `MimeType` returns `text/calendar`.
- `Put`, `Mkdir`, `Rmdir`, `Remove`, `SetModTime` all return `fs.ErrorPermissionDenied`.
- `start_year` config option (integer, default 2000); year tree does not list years before `start_year`.
- `fs.Pacer` with `pacer.NewGoogleDrive` for rate-limiting.

### Constraints

- Must-haves:
  - No bundled OAuth credential; `client_id` and `client_secret` come from user config exclusively.
  - All five write entry-points must return `fs.ErrorPermissionDenied`.
  - The `.ics` synthesis must produce iCalendar output that passes `icalendar` validation (RFC 5545): mandatory properties present, lines folded at 75 octets, `CRLF` line endings.
  - Calendar names used as directory names may contain characters that conflict with path separators or filesystem naming rules; names must be sanitized following rclone encoder conventions.
  - When a calendar has no events on a given day, `List` returns an empty slice (not an error).
  - All-day events (which have `date` rather than `dateTime` in `start`/`end`) must be represented with `DTSTART;VALUE=DATE` and `DTEND;VALUE=DATE` format in the `.ics` output.
  - The package must compile with no import cycles within the rclone module.

- Must-nots:
  - Must not hard-code any calendar ID; all IDs must come from the API `calendarList.list` response.
  - Must not expose `client_id`/`client_secret` in logs.
  - Must not implement any writable surface.
  - Must not produce `\n`-only line endings in `.ics` output (RFC 5545 requires `CRLF`).

### Failure scenarios

- **Credential omission**: Absent `client_id` or `client_secret` produces a descriptive error from `NewFs`, not a panic.
- **Calendar name collision**: Two calendars with the same `summary` produce duplicate root-level directories. The implementation should detect this and either append a disambiguator (e.g., short calendar ID suffix) or document the limitation explicitly.
- **All-day event encoding error**: Rendering an all-day event's `DTSTART`/`DTEND` as `dateTime` format (with `T` and `Z`) instead of `DATE` format causes calendar apps to interpret the event incorrectly.
- **Recurring event expansion**: `singleEvents=true` is required in the API call; if omitted, recurrence master events appear instead of individual instances, making the date-based tree incorrect.
- **Invalid path resolution**: A path like `My Calendar/2024/01/15/nonexistent-event-id — Title.ics` must return `fs.ErrorObjectNotFound` rather than a nil `Object` or panic.
- **CRLF omission in `.ics`**: If the synthesized output uses `\n` only, importing into major calendar clients (Apple Calendar, Google Calendar import, Outlook) will either fail or produce malformed events.
- **Write operation silently succeeds**: Same risk as S1-01 — any write method returning `nil` instead of `fs.ErrorPermissionDenied` is a defect.

### Success scenarios

- `rclone lsd gcalfs:` returns a directory per calendar in the authenticated account.
- `rclone lsd gcalfs:"My Calendar/2024/01/"` returns one directory entry per day that has events in January 2024.
- `rclone ls gcalfs:"My Calendar/2024/01/15/"` returns one `.ics` file per event on that date.
- `rclone copy "gcalfs:My Calendar/2024/01/15/event-id — Meeting.ics" /tmp/` produces a file importable into Apple Calendar or Google Calendar without errors.
- All-day events render with `DTSTART;VALUE=DATE:YYYYMMDD` (no time component) in the `.ics`.
- `rclone touch gcalfs:"My Calendar/test"` returns an error containing "permission denied".
- `go build ./backend/gcalfs/...` and `go vet ./backend/gcalfs/...` pass with no errors.
- The integration test in `backend/gcalfs/gcalfs_test.go` skips cleanly when no `TestGcalFs:` remote is configured.
- `start_year=2022` config causes the year listing under any calendar to show only years >= 2022.

### Connections

- Depends on `lib/oauthutil`, `lib/pacer` (same as S1-01 and `backend/googlephotos/`), `lib/rest`.
- The `dirPattern` approach is the same as S1-01 and `backend/googlephotos/pattern.go`; the calendar name at the root level is a dynamic segment (not a fixed constant like `media/` or `album/` in googlephotos), which requires a regex capture group pattern rather than a literal.
- S1-03 (registration and docs) depends on this story completing a compilable package with a valid `fs.RegInfo` `init()` block.
- No dependency on S1-01 (gmailfs); the two backends are independent.

---

## S1-03 — Registration, Docs, and Test Stubs for Both Backends

### What's wanted

Integration of both backends into the rclone binary and test infrastructure:

- `backend/all/all.go`: two new blank-import lines:
  ```go
  _ "github.com/rclone/rclone/backend/gmailfs"
  _ "github.com/rclone/rclone/backend/gcalfs"
  ```
- `docs/content/gmailfs.md`: a documentation page following the structure of `docs/content/googlephotos.md` — title, description, configuration walkthrough (OAuth setup, `client_id`, `client_secret`, `start_year`), filesystem layout section, and a "Limitations" section noting read-only status and no hash support.
- `docs/content/gcalfs.md`: same structure as above for the Calendar backend.
- `backend/gmailfs/gmailfs_test.go`: an integration test `TestIntegration` that:
  - Calls `fstest.Initialise()`
  - Defaults `*fstest.RemoteName` to `"TestGmailFs:"`
  - Calls `fs.NewFs(ctx, *fstest.RemoteName)`
  - Skips (via `t.Skipf`) if `err` is `fs.ErrorNotFoundInConfigFile`
  - Exercises at minimum: root list returns non-zero entries, a known day path lists without error (if remote configured), and all write ops return an error.
- `backend/gcalfs/gcalfs_test.go`: same pattern, defaulting to `"TestGcalFs:"`.
- `fstest/test_all/config.yaml`: two new backend entries:
  ```yaml
  - backend:  "gmailfs"
    remote:   "TestGmailFs:"
    fastlist: false
  - backend:  "gcalfs"
    remote:   "TestGcalFs:"
    fastlist: false
  ```

### Constraints

- Must-haves:
  - `go build ./...` from the repo root must succeed after this story is applied (the blank imports must resolve to real packages).
  - Both doc pages must include a note that `client_id` and `client_secret` are required and that rclone provides no default credential.
  - Both test files must guard against missing remote config via `t.Skipf` — they must never fail in CI environments that have no Gmail or Calendar credentials.
  - Doc pages must not describe write operations as available.

- Must-nots:
  - Must not add these backends to the "Active file systems" comment block in `all.go` without also having the packages exist and compile.
  - Must not remove or reorder any existing entries in `backend/all/all.go`.
  - Must not duplicate existing entries in `config.yaml`.

### Failure scenarios

- **Import before package exists**: Adding the blank import to `all.go` before the `backend/gmailfs` or `backend/gcalfs` packages are in place causes `go build ./...` to fail for the entire rclone binary.
- **Test does not skip**: If the integration test calls `require.NoError(t, err)` before checking for `fs.ErrorNotFoundInConfigFile`, CI will fail for anyone without credentials configured.
- **Doc page describes write capability**: If the doc pages omit the read-only limitation or imply uploads work, users will waste time attempting writes.
- **config.yaml duplicate**: Adding a backend entry that duplicates an existing key causes `test_all` to run the same backend twice or error on startup.

### Success scenarios

- `go build ./...` from `/Users/adeelahmad/work/rclone` succeeds with both backend packages present.
- `go test ./backend/gmailfs/ -run TestIntegration` outputs `SKIP` (not FAIL) on a machine without `TestGmailFs:` configured.
- `go test ./backend/gcalfs/ -run TestIntegration` outputs `SKIP` (not FAIL) on a machine without `TestGcalFs:` configured.
- Both `docs/content/gmailfs.md` and `docs/content/gcalfs.md` exist and contain sections covering: overview, configuration (client_id, client_secret, start_year, OAuth flow), filesystem layout, and limitations (read-only, no hash).
- `grep -c 'gmailfs\|gcalfs' backend/all/all.go` returns 2.
- `grep 'TestGmailFs\|TestGcalFs' fstest/test_all/config.yaml` returns both entries.

### Connections

- Directly depends on S1-01 (gmailfs package) and S1-02 (gcalfs package) being compilable. This story cannot be merged before both S1-01 and S1-02 produce valid `go build`-passing packages.
- The `fs.RegInfo` `Name` field in each backend's `init()` must match the prefix used in `config.yaml` remote strings (`gmailfs`, `gcalfs`).
- The doc page structure follows `docs/content/googlephotos.md` — the existing Hugo front matter and section conventions must be maintained.

---

## Open Questions (Blocking)

The following questions must be answered before planning can begin. An unresolved answer here is a scope error waiting to become a build error.

1. **Thread subject sanitization scope**: The thread folder name is `{threadId} — {Subject}`. Email subjects can contain `\`, `:`, `?`, `*`, `"`, `<`, `>`, `|`, newlines, and null bytes. The request says "following rclone encoder conventions" but does not specify which encoder preset (`Base32768`, `Standard`, `Display`, etc.) is appropriate for Gmail subjects. Which encoder preset should `gmailfs` use?

2. **`Size` contract for `.eml`**: The request allows -1 for `.eml` size. Some rclone operations (e.g., copy with progress) behave differently when size is -1 vs a known value. Must the `.eml` synthesis pre-compute size (requiring two passes or a buffer), or is -1 always acceptable? If -1 is used, does the backend need to set `Fs.Features().CanHaveEmptyDirectories` or any other feature flag to avoid rclone warnings?

3. **Pagination of thread/event listings**: `threads.list` and `events.list` both paginate via `nextPageToken`. The request does not specify whether a single `List` call should exhaust all pages (potentially hundreds of threads in a day) or cap at one page. What is the expected behavior for days with more than the API default page size of results?

4. **Calendar name as path segment disambiguation**: Two Google calendars can share the same `summary` (e.g., two "Holidays" calendars from different providers). The request does not specify a disambiguation strategy. Should the backend append a short hash of the calendar ID, use the full calendar ID as a suffix, or document this as a known limitation?

5. **`NewObject` for intermediate directories**: `NewObject("2024/01")` (a directory path, not a file path) — should this return `fs.ErrorNotAFile` or `fs.ErrorObjectNotFound`? The googlephotos backend's behavior here should be checked and matched for consistency.
