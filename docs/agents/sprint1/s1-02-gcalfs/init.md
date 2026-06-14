---
type: init
story: S1-02
---

## S1-02 · attempt 1 · red-worker · 2026-06-13T23:32:21Z

### Mandate

Write every test listed in `docs/agents/sprint1/s1-02-gcalfs/plan.md` at the
exact `path::fn` specified. Each test MUST FAIL BY ASSERTION (not by a missing
symbol or compile error) when run against minimal compile shims. No production
code beyond the shims.

Story: Google Calendar backend core (`backend/gcalfs/`)
Sprint: 1 | Wave: 1 | Role: red-worker

### Scope (May / May Not)

**May:**
- Create `backend/gcalfs/gcalfs_test.go` with all test functions from plan.md
- Create `backend/gcalfs/pattern_test.go` if pattern tests are listed in plan.md
- Create `backend/gcalfs/gcalfs.go` with MINIMAL compile shims only
  (each shim: `// agentic:shim` marker, zero logic, returns zero values)
- Create `backend/gcalfs/pattern.go` with minimal shims
- Create `backend/gcalfs/api/types.go` with minimal API struct shims
- Write `.agentic/task.env` in the worktree
- Append to `docs/agents/sprint1/s1-02-gcalfs/output.md`

**May NOT:**
- Write any production logic (all shim bodies must return zero values / nil / "")
- Write production code beyond what is needed to compile the tests
- Mark any test with `t.Skip`, `testing.Short()` guards, or `//go:build ignore`
- Pass any test (every test must FAIL BY ASSERTION)
- Edit any file outside `backend/gcalfs/` (except output.md and task.env)
- Touch S1-01, S1-03, or any planning artifact

### Inputs

- Tests-only plan: `docs/agents/sprint1/s1-02-gcalfs/plan.md`
- Tasks reference: `docs/agents/sprint1/s1-02-gcalfs/tasks.md`
- Template backend: `backend/googlephotos/googlephotos.go`, `backend/googlephotos/pattern.go`, `backend/googlephotos/api/types.go`
- Standards: `docs/agents/sprint1/standards.md`
- Go module: `github.com/rclone/rclone` (go 1.25.0)
- Key packages: `lib/oauthutil`, `lib/pacer`, `lib/rest`, `lib/encoder`, `fs/hash`, `fstest/fstests`

### Acceptance

RED gate criteria (run `gate-red-verify` via selfcheck before writing output.md):
1. Every test in plan.md exists at its listed `path::fn`.
2. `go test ./backend/gcalfs/...` → all new tests FAIL (non-zero exit), each with an assertion failure (not a compile error or missing symbol panic).
3. `regressed=0` — no previously passing test now fails.
4. `git diff --stat` touches ONLY `backend/gcalfs/**` (test files + shim files).
5. No test is marked Skip/ignore.
6. output.md block appended with `status: ok` and the result table.

### Memory

- Fresh project — no prior sprint history. Follow the playbook strictly.
- Template: `backend/googlephotos/pattern.go` is the dirPattern model; study it before writing any test.
- Key decisions resolved: encoder = `encoder.Base | encoder.EncodeCrLf | encoder.EncodeInvalidUtf8`; Size=-1 acceptable; all writes return `fs.ErrorPermissionDenied`; NewObject on directory path returns `fs.ErrorObjectNotFound`.
- Calendar-specific: root-level segment is a dynamic capture (calendar name), unlike googlephotos' fixed `media/` prefix; test for this distinction.
- All-day events: `DTSTART;VALUE=DATE` format (no `T`/`Z` time component).
- RFC 5545 `.ics`: CRLF line endings throughout; line folding at 75 octets.
- Calendar name disambiguation: ` {calendarId[:8]}` suffix when two calendars share a summary.
- OAuth: no bundled credentials — `oauthConfig` empty ClientID/ClientSecret.

---

## S1-02 · attempt 1 · scaffolder · 2026-06-14T00:00:00Z

### Mandate

RED verified (37 tests, all FAIL BY ASSERTION). Replace every `// agentic:shim` body
with `panic("SUB-AGENT-TODO: <recipe>")`. Delete no test files. Write production symbol
stubs exactly once. Write `.agentic/scaffold-symbols`.

Story: Google Calendar backend core (`backend/gcalfs/`)
Sprint: 1 | Wave: 1 | Role: scaffolder

### Scope (May / May Not)

**May:**
- Modify `backend/gcalfs/gcalfs.go` — replace shim bodies with panic stubs
- Modify `backend/gcalfs/pattern.go` — replace shim bodies with panic stubs
- Modify `backend/gcalfs/api/types.go` — leave struct definitions, only replace any shim function bodies
- Write `.agentic/scaffold-symbols` listing every stubbed symbol
- Append to `docs/agents/sprint1/s1-02-gcalfs/output.md`

**May NOT:**
- Touch any `_test.go` file
- Implement any real logic (panic stub only)
- Pass any test
- Edit files outside `backend/gcalfs/` (except output.md and scaffold-symbols)
- Touch S1-01, S1-03, or any planning artifact

### Inputs

- Red-phase files: worktree at `/Users/adeelahmad/work/rclone/.agentic/worktrees/agent-af763a25ab473a5e6/backend/gcalfs/`
- plan-ready.md: `docs/agents/sprint1/s1-02-gcalfs/plan-ready.md`
- Tasks reference: `docs/agents/sprint1/s1-02-gcalfs/tasks.md`
- Standards: `docs/agents/sprint1/standards.md`

### Acceptance

SCAFFOLD gate criteria:
1. `go build ./backend/gcalfs/...` → exits 0.
2. `go test ./backend/gcalfs/...` → exits non-zero (panics or assertions).
3. `grep -c "SUB-AGENT-TODO" backend/gcalfs/*.go` → all production symbols stubbed.
4. `grep -rn "agentic:shim" backend/gcalfs/*.go` → empty (shims removed from function bodies).
5. No test file modified.
6. `.agentic/scaffold-symbols` written.
7. output.md block appended with `status: ok`.

### Memory

- Each shim function body becomes: `panic("SUB-AGENT-TODO: <what the production code must do>")`
- Struct definitions (types) are kept as-is — only function bodies get the panic stub
- The `oauthConfig` var stays with empty ClientID/ClientSecret (already correct)
- `// agentic:shim` comment markers may be removed from function bodies (they served their purpose)
- Work in worktree: `/Users/adeelahmad/work/rclone/.agentic/worktrees/agent-af763a25ab473a5e6/`
