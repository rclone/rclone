---
type: sprint-plan
sprint: 1
stage: "1"
---

# Sprint 1 Plan — gmailfs & gcalfs

## Sprint goal

Deliver `gmailfs` and `gcalfs` as compilable, registered, documented, read-only
rclone backends in two waves: Wave 1 builds the two independent backend cores in
parallel; Wave 2 wires them into the binary, docs, and test harness. See
`stories.md` for the full five-part Intent of each story and `standards.md` for the
binding rule digest and gate matrix.

## Wave layout

| Wave | Stories | Mode | Rationale |
|------|---------|------|-----------|
| 1 | S1-01 (gmailfs), S1-02 (gcalfs) | Parallel | No shared source; each imports only `lib/`, `fs/`, and its own `api/`. Independent worktrees. |
| 2 | S1-03 (registration/docs/tests) | After Wave 1 GREEN | `all.go` blank imports and `go build ./...` require both Wave 1 packages to compile. |

## Dependency graph

```
S1-01 (gmailfs) ─┐
                 ├─▶ S1-03 (registration, docs, test stubs, config.yaml)
S1-02 (gcalfs) ──┘
```

- S1-01 ⟂ S1-02 (independent — may run concurrently).
- S1-03 requires `go build` success of both S1-01 and S1-02.

## Parallelism table

| Story | Depends on | Can start when | Parallel with | Isolation |
|-------|-----------|----------------|---------------|-----------|
| S1-01 | — (lib/oauthutil, lib/pacer, lib/rest, lib/encoder, fs/hash exist) | Wave 1 start | S1-02 | own worktree |
| S1-02 | — (same shared libs) | Wave 1 start | S1-01 | own worktree |
| S1-03 | S1-01 GREEN, S1-02 GREEN | Wave 2 start | — | own worktree |

Max concurrency: 2 (Wave 1). Wave 2 is single-story.

## Critical path

```
S1-01 ─┐
       ├─▶ S1-03
S1-02 ─┘
```

Critical path length = 2 waves. The Wave-1 critical contributor is whichever of
S1-01 / S1-02 finishes last (both are non-trivial; S1-01's `.eml` MIME synthesis is
the heavier task and is the expected pacing item). S1-03 cannot begin until BOTH
Wave-1 packages pass `go build`. There is no path that lets S1-03 start early.

## Per-story plan pointers (Stage 2)

| Story | Tasks | Validate | Tests-only plan |
|-------|-------|----------|-----------------|
| S1-01 | `s1-01-gmailfs/tasks.md` | `s1-01-gmailfs/validate.md` | `s1-01-gmailfs/plan.md` |
| S1-02 | `s1-02-gcalfs/tasks.md` | `s1-02-gcalfs/validate.md` | `s1-02-gcalfs/plan.md` |
| S1-03 | `s1-03-registration/tasks.md` | `s1-03-registration/validate.md` | `s1-03-registration/plan.md` |

## Cross-cutting gates

Authoritative matrix lives in `standards.md` §3. Every gate there binds execution.
Summary of what each gate covers and where it applies:

| Gate | Command (from `standards.md` §3) | Applies to |
|------|-----------------------------------|------------|
| fmt | `gofmt -l ./backend/gmailfs/ ./backend/gcalfs/` → empty | all tasks |
| vet | `go vet ./backend/gmailfs/... ./backend/gcalfs/...` → exit 0 | all tasks |
| lint | `golangci-lint run ./backend/gmailfs/... ./backend/gcalfs/...` → exit 0 | all tasks |
| build | `go build ./...` → exit 0 | S1-03, FINAL |
| test-skip | `go test ./backend/gmailfs/ ./backend/gcalfs/ -run TestIntegration -v` → SKIP, no FAIL | S1-03 |
| write-ops | `grep -n "ErrorPermissionDenied" backend/gmailfs/*.go backend/gcalfs/*.go` → present in Put/Mkdir/Rmdir/Remove/SetModTime | S1-01, S1-02 |
| no-bundled-creds | `grep -rn 'rcloneClientID\|rcloneClientSecret\|ClientID\s*=\s*"[A-Za-z0-9]' backend/gmailfs/ backend/gcalfs/` → absent | S1-01, S1-02 |
| eml-mime | `python3 -c "import email; email.message_from_file(open('test.eml'))"` → no exception | S1-01 |
| ics-crlf | `grep -cP '\r\n' test.ics` matches `wc -l test.ics` | S1-02 |
| race | `go test -race ./backend/gmailfs/... ./backend/gcalfs/...` → exit 0, no DATA RACE | FINAL |
| coverage | `go test -cover ./backend/gmailfs/... ./backend/gcalfs/...` → ≥80% on synthesis/encoder logic | FINAL |

**Read-only invariant (R-1, standards.md):** `Put`, `Mkdir`, `Rmdir`, `Remove`,
`SetModTime` each return `fs.ErrorPermissionDenied` in BOTH backends. This is the
single most load-bearing cross-cutting rule and is checked at GREEN per task and at
FINAL across both backends.

**PATH requirement:** `export PATH="$HOME/.local/bin:$PATH"` before running md-db /
ctx-symbols gate tooling.
