---
type: validate
story: S1-01
---

# S1-01 Validate — Gmail backend core

Literal PASS/FAIL checks, one block per task in `tasks.md`. No judgement calls:
each check is a command with an expected result. Run from
`/Users/adeelahmad/work/rclone`. `export PATH="$HOME/.local/bin:$PATH"` first.

## Pre-flight

- `go version` → `go1.25` or newer. FAIL: wrong toolchain.
- `test -f backend/gmailfs/gmailfs.go` → exit 0 once T01 lands. FAIL: package
  skeleton missing.
- `python3 -c "import email"` → exit 0 (needed for the `.eml` MIME check). FAIL:
  no python3.
- `grep -q 'pacer.NewGoogleDrive' backend/gmailfs/gmailfs.go` → exit 0 after T01.
  FAIL: pacer policy missing (P-1).

### T01 — skeleton / OAuth / NewFs
- `go build ./backend/gmailfs/...` → exit 0. FAIL: compile error.
- `grep -rnE 'rcloneClientID|rcloneClientSecret|ClientID\s*=\s*"[A-Za-z0-9]' backend/gmailfs/`
  → no output, exit 1. FAIL (no-bundled-creds, O-1): a hardcoded credential exists.
- `grep -q 'gmail.readonly' backend/gmailfs/gmailfs.go` → exit 0. FAIL: wrong/missing
  scope (O-3).
- `grep -q 'oauthutil.SharedOptions' backend/gmailfs/gmailfs.go` → exit 0. FAIL: not
  surfacing user client_id/client_secret (O-1).
- `grep -q 'ReadMimeType' backend/gmailfs/gmailfs.go` → exit 0. FAIL: features not set
  (R-2).
- Unit (RED→GREEN): `go test ./backend/gmailfs/ -run TestNewFs_RequiresClientID` → PASS;
  NewFs with empty client_id returns a non-nil error and does not panic. FAIL: panic or
  nil error.

### T02 — dirPattern tree
- `go vet ./backend/gmailfs/...` → exit 0. FAIL: vet error.
- Unit: `go test ./backend/gmailfs/ -run TestPatternMatch` → PASS; every level
  (`""`, `2024`, `2024/2024-01`, `2024/2024-01/2024-01-15`, thread, `.eml`,
  `attachments`, attachment) matches the expected pattern and `isFile` flag; an
  unrecognized path returns nil pattern. FAIL (D-1/D-2/D-3): missing level or wrong
  isFile.

### T03 — List
- Unit: `go test ./backend/gmailfs/ -run TestList_RootReturnsYears` → PASS; `List("")`
  returns dirs from start_year..current year. FAIL: wrong range.
- Unit: `go test ./backend/gmailfs/ -run 'TestList_(Months|Days)'` → PASS; month level
  returns 12 `YYYY-MM` dirs, day level returns the year's days. FAIL: wrong count/format.
- Unit (mock transport): `go test ./backend/gmailfs/ -run TestList_DayPaginates` → PASS;
  a two-page `threads.list` mock yields all threads from both pages (Decision 3). FAIL:
  only first page returned.
- `go test ./backend/gmailfs/ -run TestList_UnknownDirNotFound` → PASS; an unmatched dir
  returns `fs.ErrorDirNotFound`. FAIL (D-2): nil error or panic.

### T04 — NewObject
- Unit: `go test ./backend/gmailfs/ -run TestNewObject_ResolvesEml` → PASS; a valid
  `.eml` path resolves to an Object with matching Remote(). FAIL: error on valid path.
- Unit: `go test ./backend/gmailfs/ -run TestNewObject_DirectoryReturnsNotFound` → PASS;
  `NewObject("2024/01")` returns `fs.ErrorObjectNotFound` (Decision 5). FAIL: wrong error
  / nil / panic.
- Unit: `go test ./backend/gmailfs/ -run TestNewObject_BadPathNotFound` → PASS;
  `2024/01/15/badthread/nomessage.eml` returns `fs.ErrorObjectNotFound`. FAIL: panic.

### T05 — `.eml` synthesis
- Unit: `go test ./backend/gmailfs/ -run TestEmlSynthesis_Parses` → PASS; the synthesized
  bytes parse via Go `net/mail`/`mime/multipart` (or the test writes a fixture and the
  harness shells `python3 -c "import email; email.message_from_file(open(p))"` returning
  exit 0). FAIL (S-1, eml-mime gate): parse exception.
- Unit: `go test ./backend/gmailfs/ -run TestEmlSynthesis_DistinctBoundaries` → PASS;
  nested `multipart/mixed` and `multipart/alternative` use distinct boundaries. FAIL:
  shared boundary.
- Unit: `go test ./backend/gmailfs/ -run TestEmlSynthesis_Deterministic` → PASS; same
  payload → identical bytes twice (S-3). FAIL: nondeterministic.

### T06 — attachment open
- Unit (mock): `go test ./backend/gmailfs/ -run TestAttachmentOpen_Base64URL` → PASS;
  base64url payload decodes byte-for-byte to the expected bytes. FAIL: wrong alphabet
  (standard base64) or wrong bytes.

### T07 — metadata
- Unit: `go test ./backend/gmailfs/ -run TestObject_ModTimeFromInternalDate` → PASS;
  ModTime equals the ms-epoch `internalDate` converted to time.Time. FAIL: wrong time.
- Unit: `go test ./backend/gmailfs/ -run TestObject_AttachmentSizeDecoded` → PASS;
  attachment Size is the decoded byte count, not base64 length. FAIL: encoded length.
- Unit: `go test ./backend/gmailfs/ -run TestObject_HashNone` → PASS; `Hash` returns
  `"", hash.ErrUnsupported` and `Fs.Hashes()` == `hash.Set(hash.None)` (R-3). FAIL: other.
- Unit: `go test ./backend/gmailfs/ -run TestObject_MimeType` → PASS; `.eml` →
  `message/rfc822`; attachment → part Content-Type. FAIL: wrong mime.

### T08 — read-only enforcement
- `grep -n 'ErrorPermissionDenied' backend/gmailfs/*.go` → at least 5 matches across
  Put/Mkdir/Rmdir/Remove/SetModTime (write-ops gate, R-1). FAIL: any missing.
- Unit: `go test ./backend/gmailfs/ -run TestReadOnly_AllWriteOpsDenied` → PASS; each of
  Put, Mkdir, Rmdir, Remove, SetModTime returns exactly `fs.ErrorPermissionDenied`. FAIL:
  any returns nil or a different error.

## Final sign-off

All checks above PASS, plus the per-task gates from `standards.md` §3 applied to S1-01:

- `gofmt -l ./backend/gmailfs/` → empty output.
- `go vet ./backend/gmailfs/...` → exit 0.
- `golangci-lint run ./backend/gmailfs/...` → exit 0.
- write-ops gate: `grep -n "ErrorPermissionDenied" backend/gmailfs/*.go` shows all five.
- no-bundled-creds gate: credential grep returns nothing.
- eml-mime gate: a synthesized `.eml` fixture parses under python3 `email`.
- `go test -race -cover ./backend/gmailfs/...` → exit 0, ≥80% on synthesis/encoder/
  pattern logic, no DATA RACE.

S1-01 is DONE only when every box above is checked and no FAIL remains.
