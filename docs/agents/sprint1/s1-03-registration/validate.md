---
type: validate
story: S1-03
---

# S1-03 Validate — Registration, docs, test stubs

Literal PASS/FAIL checks, one block per task in `tasks.md`. Run from
`/Users/adeelahmad/work/rclone`.

## Pre-flight

- `go build ./backend/gmailfs/...` → exit 0. FAIL: S1-01 incomplete; do not start S1-03.
- `go build ./backend/gcalfs/...` → exit 0. FAIL: S1-02 incomplete; do not start S1-03.
- `test -f docs/content/googlephotos.md` → exit 0 (doc template reference present).

### T01 — blank imports
- `grep -c 'backend/gmailfs\|backend/gcalfs' backend/all/all.go` → `2`. FAIL: missing or
  duplicated import.
- `go build ./...` → exit 0. FAIL (build gate): an import does not resolve / whole-binary
  build broken.
- `git diff backend/all/all.go` shows only two added lines, no removals/reorders. FAIL:
  existing entries changed.

### T02 — gmailfs doc page
- `test -f docs/content/gmailfs.md` → exit 0. FAIL: missing.
- `grep -qi 'client_id' docs/content/gmailfs.md && grep -qi 'client_secret' docs/content/gmailfs.md`
  → exit 0. FAIL: required-credential note absent.
- `grep -qi 'read.only\|read only' docs/content/gmailfs.md` → exit 0. FAIL: read-only
  limitation absent.
- `grep -qiE 'limitations?' docs/content/gmailfs.md` → exit 0. FAIL: no limitations
  section.
- `grep -qi 'start_year' docs/content/gmailfs.md` → exit 0. FAIL: config option
  undocumented.
- Manual: no sentence implies upload/write works. FAIL: write capability described.

### T03 — gcalfs doc page
- `test -f docs/content/gcalfs.md` → exit 0. FAIL: missing.
- `grep -qi 'client_id' docs/content/gcalfs.md && grep -qi 'client_secret' docs/content/gcalfs.md`
  → exit 0. FAIL: required-credential note absent.
- `grep -qi 'read.only\|read only' docs/content/gcalfs.md` → exit 0. FAIL.
- `grep -qiE 'limitations?' docs/content/gcalfs.md` → exit 0. FAIL.
- `grep -qi 'start_year' docs/content/gcalfs.md` → exit 0. FAIL.
- Manual: no sentence implies write works. FAIL.

### T04 — integration test stubs
- `test -f backend/gmailfs/gmailfs_test.go && test -f backend/gcalfs/gcalfs_test.go`
  → exit 0. FAIL: missing.
- `grep -q 'fstest.Initialise' backend/gmailfs/gmailfs_test.go` → exit 0. FAIL (T-2).
- `grep -q 'ErrorNotFoundInConfigFile' backend/gmailfs/gmailfs_test.go` → exit 0.
  FAIL (T-1): no skip guard.
- `grep -q 'TestGmailFs:' backend/gmailfs/gmailfs_test.go` → exit 0. FAIL: wrong default
  remote.
- `grep -q 'TestGcalFs:' backend/gcalfs/gcalfs_test.go` → exit 0. FAIL.
- `go test ./backend/gmailfs/ ./backend/gcalfs/ -run TestIntegration -v 2>&1 | grep -q SKIP`
  → exit 0 on a machine without credentials. FAIL (test-skip gate): a FAIL line appears.
- `go test ./backend/gmailfs/ ./backend/gcalfs/ -run TestIntegration -v 2>&1 | grep -q -- '--- FAIL'`
  → exit 1 (no FAIL lines). FAIL: integration test fails without credentials.

### T05 — config.yaml entries
- `grep -c 'TestGmailFs:\|TestGcalFs:' fstest/test_all/config.yaml` → `2`. FAIL: missing
  or duplicated.
- `grep -c '"gmailfs"\|"gcalfs"' fstest/test_all/config.yaml` → `2`. FAIL: backend key
  missing/duplicated.
- `python3 -c "import yaml,sys; yaml.safe_load(open('fstest/test_all/config.yaml'))"`
  → exit 0. FAIL: YAML no longer parses.
- The `backend` value in each new entry equals the package `fs.RegInfo.Prefix`
  (`gmailfs`, `gcalfs`). FAIL: mismatch would make `test_all` target a nonexistent
  remote prefix.

### T06 — backend data files, site navigation, index entries
- `grep -c 'gmailfs\|gcalfs' docs/data/backends/gmailfs.yaml docs/data/backends/gcalfs.yaml`
  → `1` per file. FAIL: backend data YAML missing or wrong prefix.
- `grep -c 'gmailfs\|gcalfs' docs/layouts/chrome/navbar.html` → `2`. FAIL: navbar entries
  missing or duplicated.
- `grep -c 'gmailfs\|gcalfs' bin/make_manual.py` → `2`. FAIL: doc pages not added to the
  `docs` list constant.
- `grep -c 'gmailfs\|gcalfs' README.md` → `2`. FAIL: backends-list entries missing or
  duplicated.
- `grep -c 'gmailfs\|gcalfs' docs/content/docs.md` → `2`. FAIL: remote-list entries
  missing or duplicated.
- `grep -c 'gmailfs\|gcalfs' docs/content/_index.md` → `2`. FAIL: provider entries missing
  or duplicated.
- Manual: all entries are in alphabetical position; no duplicate lines added. FAIL:
  out-of-order or duplicate registration.

## Final sign-off

- `go build ./...` → exit 0 (build gate, FINAL).
- `go vet ./...` (at least `./backend/gmailfs/... ./backend/gcalfs/...`) → exit 0.
- `go test ./backend/gmailfs/ ./backend/gcalfs/ -run TestIntegration -v` → SKIP, no FAIL
  (test-skip gate).
- Both doc pages present with required sections; both `config.yaml` entries present and
  non-duplicate; exactly two blank imports added to `all.go` with no reorder.

S1-03 is DONE only when every box above is checked and no FAIL remains.
