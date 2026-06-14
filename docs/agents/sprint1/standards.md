<!-- standards · sprint 1 · validated by bin/gate-standards-cited (not planning-artifacts.kdl) -->

# Sprint 1 Standards — gmailfs & gcalfs Backends

## 1. Stack Summary

| Item | Value |
|------|-------|
| Language | Go 1.25.0 |
| Module path | `github.com/rclone/rclone` |
| Primary template | `backend/googlephotos/` |
| OAuth | `lib/oauthutil` (`oauthutil.Config`, `oauthutil.SharedOptions`) |
| REST client | `lib/rest.Client` |
| Pacer | `lib/pacer` — `pacer.NewGoogleDrive` |
| Path encoder | `lib/encoder` — `encoder.Base | encoder.EncodeCrLf | encoder.EncodeInvalidUtf8` |
| Hash | `fs/hash` — backends return `hash.None` |
| Dir-tree routing | `dirPattern` / `dirPatterns` slice (see `backend/googlephotos/pattern.go`) |
| Linter config | `.golangci.yml` (golangci-lint v2, formatters: goimports) |
| Enabled linters | errcheck, govet, ineffassign, staticcheck, unused, gocritic, misspell, revive, unconvert |
| Test harness | `fstest.Run` / `fstest.Initialise` from `github.com/rclone/rclone/fstest/fstests` |

Key Google API dependencies already in `go.mod`:
- `golang.org/x/oauth2/google` (Google OAuth endpoints)
- Gmail REST API: `https://gmail.googleapis.com/gmail/v1`
- Calendar REST API: `https://www.googleapis.com/calendar/v3`

---

## 2. Active Rules

Each rule cites the authoritative source from this repo or an external spec.

### Formatting & Static Analysis

| # | Rule | Source |
|---|------|--------|
| F-1 | All `.go` files must be `gofmt`-clean before commit. Run: `gofmt -l ./backend/gmailfs/ ./backend/gcalfs/` — output must be empty. | Go toolchain: `go fmt ./...` |
| F-2 | `goimports` is the project formatter (declared in `.golangci.yml` `formatters.enable`). Import grouping: stdlib / external / internal rclone packages. | `/Users/adeelahmad/work/rclone/.golangci.yml` line 149 |
| F-3 | `go vet` must exit 0 on both backends. `govet.enable-all: true` with `fieldalignment` and `shadow` disabled. | `/Users/adeelahmad/work/rclone/.golangci.yml` lines 33-38 |
| F-4 | golangci-lint must pass (errcheck, ineffassign, staticcheck, unused, gocritic, misspell, revive, unconvert). Run: `golangci-lint run ./backend/gmailfs/... ./backend/gcalfs/...` | `/Users/adeelahmad/work/rclone/.golangci.yml` lines 7-19 |
| F-5 | No import cycles. Each backend package imports only `lib/`, `fs/`, `backend/<name>/api/` — never peer backends. | Go language specification: package initialization |

### Read-Only Contract

| # | Rule | Source |
|---|------|--------|
| R-1 | `Put`, `Mkdir`, `Rmdir`, `Remove`, and `SetModTime` must each return `fs.ErrorPermissionDenied` with no side effects. No exceptions. | rclone `fs.Fs` interface contract + sprint requirement (both backends are read-only) |
| R-2 | `Fs.Features()` must set `ReadMimeType: true` and leave `WriteMimeType`, `CanHaveEmptyDirectories`, `BucketBased`, `BucketBasedRootOK` at their zero/false defaults for these read-only backends. | `backend/googlephotos/googlephotos.go` — `fs.Features` struct usage |
| R-3 | `Hash` must return `hash.Set(hash.None)` — rclone cannot compute content hashes for synthesized virtual files. | `backend/googlephotos/googlephotos.go` line 847; `fs/hash` package |

### OAuth & Credentials

| # | Rule | Source |
|---|------|--------|
| O-1 | No hardcoded `ClientID` or `ClientSecret` constants. Credentials come exclusively from user config via `oauthutil.SharedOptions` (which surfaces `client_id` / `client_secret` config keys). | Sprint requirement: no bundled credentials; `lib/oauthutil/oauthutil.go` SharedOptions |
| O-2 | Use `oauthutil.Config` struct and `oauthutil.NewClientWithBaseClient` or `oauthutil.Config.Client` for token refresh. Never use `golang.org/x/oauth2` directly for token management. | `lib/oauthutil` package; `backend/googlephotos/googlephotos.go` lines 79-86 |
| O-3 | Gmail backend OAuth scope: `https://www.googleapis.com/auth/gmail.readonly`. Calendar backend OAuth scope: `https://www.googleapis.com/auth/calendar.readonly`. | Google API documentation; principle of least privilege |

### Path Routing (dirPattern)

| # | Rule | Source |
|---|------|--------|
| D-1 | Every path level exposed by the backend must have a corresponding `dirPattern` entry with a compiled `*regexp.Regexp`. | `backend/googlephotos/pattern.go` lines 29-43 |
| D-2 | Any path that matches no `dirPattern` must return `fs.ErrorObjectNotFound` (for `NewObject`) or `fs.ErrorDirNotFound` (for `List`). Unrecognized paths must never panic or return nil error. | `backend/googlephotos/googlephotos.go` lines 719-720, 936-937 |
| D-3 | `dirPattern.isFile = true` entries are leaves (objects). `isFile = false` entries are directories. The `match` function must distinguish these before dispatching. | `backend/googlephotos/pattern.go` `dirPattern` struct fields |

### File Synthesis

| # | Rule | Source |
|---|------|--------|
| S-1 | gmailfs must synthesize RFC 2822-compliant `.eml` files. Required headers: `Date`, `From`, `To`, `Subject`, `Message-ID`, `MIME-Version`, `Content-Type`. Body encoding must follow MIME Part 1 (RFC 2045). | RFC 2822 (Internet Message Format); RFC 2045 (MIME) |
| S-2 | gcalfs must synthesize RFC 5545-compliant `.ics` files. Line endings must be CRLF (`\r\n`) throughout. Lines exceeding 75 octets must be folded with CRLF + single space. `BEGIN:VCALENDAR` / `END:VCALENDAR` wrappers required. | RFC 5545 §3.1 (line folding), §3.4 (iCalendar object) |
| S-3 | Synthesized file content must be deterministic for a given API response — same API payload always produces the same bytes. This ensures rclone's deduplication logic works correctly. | rclone design principle: reproducible Object.Open |

### Path Encoding

| # | Rule | Source |
|---|------|--------|
| E-1 | Use `encoder.Base | encoder.EncodeCrLf | encoder.EncodeInvalidUtf8` for all path encoding. This is the standard encoding for Google-family backends in this repo. | `lib/encoder` package; `backend/googlephotos/googlephotos.go` lines 213-215 |

### Pacing

| # | Rule | Source |
|---|------|--------|
| P-1 | Wrap all outbound API calls with `fs.Pacer` using `pacer.NewGoogleDrive` policy. This provides the retry-with-backoff behaviour required by Google API rate limits. | `lib/pacer` package; `backend/googlephotos/googlephotos.go` pacer usage |

### Testing

| # | Rule | Source |
|---|------|--------|
| T-1 | Integration tests must skip (not fail) when the remote is not configured: check for `fs.ErrorNotFoundInConfigFile` and call `t.Skipf`. | `backend/googlephotos/googlephotos_test.go` lines 39-40 |
| T-2 | Test file must call `fstest.Initialise()` before creating any `fs.Fs`. | `backend/googlephotos/googlephotos_test.go` line 33 |
| T-3 | Minimum 80% statement coverage on synthesized-file logic (encoder functions, dirPattern match, MIME/iCal builders) via unit tests that do not require network access. | `rules/common/testing.md` |
| T-4 | Race detector must be enabled when running tests: `go test -race ./backend/gmailfs/... ./backend/gcalfs/...` | `rules/golang/testing.md` |

### Security

| # | Rule | Source |
|---|------|--------|
| SC-1 | No secrets, tokens, or credentials in source files. `gosec ./...` must pass. | `rules/golang/security.md`; `rules/common/security.md` |
| SC-2 | All HTTP requests must use a `context.Context` with timeout; never use `http.DefaultClient`. Use `fshttp.NewClient(ctx)` as the base. | `rules/golang/security.md`; `fs/fshttp` package |

---

## 3. Cross-Cutting Gate Matrix

All gates must pass before a task is marked DONE. GREEN gate = passes for each task individually. FINAL gate = passes across both backends together.

| Gate | Command | Pass Condition | Applies To |
|------|---------|----------------|------------|
| fmt | `gofmt -l ./backend/gmailfs/ ./backend/gcalfs/` | empty output (no files listed) | all tasks |
| vet | `go vet ./backend/gmailfs/... ./backend/gcalfs/...` | exit 0 | all tasks |
| lint | `golangci-lint run ./backend/gmailfs/... ./backend/gcalfs/...` | exit 0, no errors | all tasks |
| build | `go build ./...` | exit 0 | S1-03 and FINAL |
| test-skip | `go test ./backend/gmailfs/ ./backend/gcalfs/ -run TestIntegration -v` | output contains `SKIP`, no `FAIL` lines | S1-03 |
| write-ops | `grep -n "ErrorPermissionDenied" backend/gmailfs/*.go backend/gcalfs/*.go` | present in Put, Mkdir, Rmdir, Remove, SetModTime implementations in both backends | S1-01, S1-02 |
| no-bundled-creds | `grep -rn "rcloneClientID\|rcloneClientSecret\|ClientID\s*=\s*\"[A-Za-z0-9]" backend/gmailfs/ backend/gcalfs/` | absent (exit 1 / empty) | S1-01, S1-02 |
| eml-mime | `python3 -c "import email, sys; email.message_from_file(open('test.eml'))"` on synthesized .eml output | no exception raised | S1-01 |
| ics-crlf | `grep -cP '\r\n' test.ics` vs `wc -l test.ics` | line counts match (every line ends CRLF) | S1-02 |
| race | `go test -race ./backend/gmailfs/... ./backend/gcalfs/...` | exit 0, no DATA RACE output | FINAL |
| coverage | `go test -cover ./backend/gmailfs/... ./backend/gcalfs/...` | coverage >= 80% for synthesis/encoder logic | FINAL |

**PATH requirement:** `~/.local/bin` must be on PATH for `md-db` and `ctx-symbols` gate tooling.
Export before running gates: `export PATH="$HOME/.local/bin:$PATH"`

### Gate matrix (machine-runnable)

The gate runner executes the fenced commands below verbatim (each must exit 0). These
are the GREEN-applicable subset of the table above, expressed as runnable shell so the
gate binds to THIS Go matrix instead of a language default. `gofmt -l` must print
nothing, and the bundled-credential grep must find nothing.

```
test -z "$(gofmt -l ./backend/gcalfs/)"
go vet ./backend/gcalfs/...
go build ./backend/gcalfs/...
grep -q "ErrorPermissionDenied" backend/gcalfs/gcalfs.go
! grep -rnE 'rcloneClientID|rcloneClientSecret|ClientID[[:space:]]*=[[:space:]]*"[A-Za-z0-9]' backend/gcalfs/
go test ./backend/gcalfs/...
```

---

## 4. Appendix — Key File References

| File | Role |
|------|------|
| `/Users/adeelahmad/work/rclone/backend/googlephotos/googlephotos.go` | Primary template: Fs struct, init(), NewFs, write-op stubs, encoder config |
| `/Users/adeelahmad/work/rclone/backend/googlephotos/pattern.go` | dirPattern/dirPatterns types; match() function — copy and adapt tree for each backend |
| `/Users/adeelahmad/work/rclone/backend/googlephotos/googlephotos_test.go` | Test skip pattern; fstest.Initialise() usage |
| `/Users/adeelahmad/work/rclone/lib/oauthutil/` | OAuth helpers — use instead of raw golang.org/x/oauth2 |
| `/Users/adeelahmad/work/rclone/lib/pacer/` | pacer.NewGoogleDrive policy |
| `/Users/adeelahmad/work/rclone/lib/encoder/` | Path encoder constants |
| `/Users/adeelahmad/work/rclone/fs/hash/hash.go` | hash.None constant |
| `/Users/adeelahmad/work/rclone/.golangci.yml` | Full linter configuration — reference for enabled checks |
