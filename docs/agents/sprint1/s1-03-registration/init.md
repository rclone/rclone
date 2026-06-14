---
type: init
story: S1-03
---

## S1-03 · attempt 1 · red-worker · 2026-06-14T00:40:00Z

### Mandate

Write all RED tests for S1-03 (registration, docs, test stubs). Tests ONLY — no production code, no doc pages, no config changes. Every test must FAIL BY ASSERTION in the current repo state (Wave 1 backends exist in `backend/gmailfs/` and `backend/gcalfs/` but are NOT yet wired into `all.go`, have no doc pages, no config.yaml entries, and no navigation entries).

### Scope

**May:**
- Write `backend/gmailfs/integration_test.go` (new file) with `TestRegistrationFilePresence`
- Add `TestIntegration` to `backend/gmailfs/gmailfs_test.go` and `backend/gcalfs/gcalfs_test.go`
- Write minimal compile shims if needed (e.g. imports in test files)
- Touch only `backend/gmailfs/` and `backend/gcalfs/` source files and new test files

**May not:**
- Edit `backend/all/all.go` (that is GREEN work)
- Create `docs/content/gmailfs.md` or `docs/content/gcalfs.md` (GREEN work)
- Edit `fstest/test_all/config.yaml` (GREEN work)
- Edit `docs/layouts/chrome/navbar.html`, `bin/make_manual.py`, `README.md`, `docs/content/docs.md`, `docs/content/_index.md` (GREEN work)
- Edit any existing test assertions (only add new tests)
- Write production code

### Inputs

- Sprint plan: `docs/agents/sprint1/plan.md`
- Story tasks: `docs/agents/sprint1/s1-03-registration/tasks.md`
- Test plan: `docs/agents/sprint1/s1-03-registration/plan.md`
- Validate rubric: `docs/agents/sprint1/s1-03-registration/validate.md`
- Reference for TestIntegration pattern: `backend/googlephotos/googlephotos_test.go`
- Reference for existing test structure: `backend/gmailfs/gmailfs_test.go`, `backend/gcalfs/gcalfs_test.go`

### Acceptance

Every test listed in `s1-03-registration/plan.md` must:
1. Exist at the exact `path::fn` specified
2. FAIL BY ASSERTION (not by compile error, panic, or missing symbol)
3. Zero existing tests may be broken (regressed=0)
4. `go build ./backend/gmailfs/... ./backend/gcalfs/...` exits 0 after your changes
5. No production logic implemented — only test code

### RED test strategy for S1-03

**T06 — `TestRegistrationFilePresence`** (in `backend/gmailfs/integration_test.go`):
Write a Go test that shells out to grep for `gmailfs` and `gcalfs` in each of the 7 registration files listed in tasks.md T06. Currently none of those files have the entries, so every grep assertion fails → RED FAIL BY ASSERTION.

**T04 — `TestIntegration`** (added to `backend/gmailfs/gmailfs_test.go` and `backend/gcalfs/gcalfs_test.go`):
Write `TestIntegration` WITHOUT the `t.Skipf` guard for `fs.ErrorNotFoundInConfigFile`. In the current state, `fstest.Initialise()` will return an error because neither backend is in `rclone.conf` — the test should fail hard (not skip) to satisfy RED. GREEN will add the proper skip guard.

Wait — actually think carefully here: if TestIntegration tries `fs.NewFs` on a backend that IS registered (via `backend/all/all.go` which does NOT include gmailfs/gcalfs yet), it will return `fs.ErrorNotFoundInConfigFile`. Without the skip guard it fails → RED FAIL. With the skip guard (GREEN) it skips → PASS.

Actually, the backends won't be in all.go yet, so fs.NewFs("TestGmailFs:") will return `fs.ErrorNotFoundInConfigFile`. Without the guard → test fails. ✓ RED.

**T01, T02, T03, T05 plan items** are shell/grep/file-existence checks verified by the gate, not Go tests. The only Go tests to write are `TestRegistrationFilePresence` (T06) and `TestIntegration` (T04).

### Memory

No prior sprint memories (fresh project). Key invariants from standards.md:
- Read-only: `Put`/`Mkdir`/`Rmdir`/`Remove`/`SetModTime` return `ErrorPermissionDenied` — already implemented, do not weaken
- No bundled credentials — already correct, do not change
- No test suppression: no `t.Skip`, `t.SkipNow` in RED tests (except the `TestIntegration` skip-on-error guard IS intentional for the final GREEN form — but in RED, omit it so the test fails)
