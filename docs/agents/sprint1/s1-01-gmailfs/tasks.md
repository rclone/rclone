---
type: tasks
story: S1-01
---

# S1-01 Tasks — Gmail backend core (`backend/gmailfs/`)

Each task is atomic, independently testable, and cites the rules from `standards.md`
it must satisfy. Tasks are ordered by dependency; T02 depends on T01's structs, etc.
All production symbols land in `backend/gmailfs/gmailfs.go`, `backend/gmailfs/pattern.go`,
and `backend/gmailfs/api/types.go` unless noted.

## T01 — Package skeleton, OAuth registration, Options/Fs/Object, NewFs

**Scope.** Create the `gmailfs` package; `init()` registers `fs.RegInfo{Name:"gmail",
Prefix:"gmailfs", Description:"Gmail", NewFs:NewFs}` with
`Options: append(oauthutil.SharedOptions, …)` adding `start_year` (int, default 2000)
and the standard `encoding` option (`encoder.Base | encoder.EncodeCrLf |
encoder.EncodeInvalidUtf8`). Define `oauthConfig` with empty `ClientID`/`ClientSecret`,
scope `https://www.googleapis.com/auth/gmail.readonly`, Google auth/token URLs. Define
`Options`, `Fs` (name, root, opt, features, srv `*rest.Client`, ts, pacer
`pacer.NewGoogleDrive`, startTime), and `Object` (fs, remote, id, threadID, messageID,
modTime, bytes, mimeType, isAttachment, attachmentID, partSize) structs. Implement
`NewFs` (parse config, build oauth client via `oauthutil.NewClientWithBaseClient`,
trim root, set `Features{ReadMimeType:true}`, the `ErrorIsFile` probe), `Name`, `Root`,
`String`, `Features`, `Precision` (=`fs.ModTimeNotSupported`), `Hashes`
(=`hash.Set(hash.None)`), `dirTime`, `startYear`.

**Rules.** O-1, O-2, O-3, R-2, R-3, E-1, P-1, F-1..F-5.
**Done when.** `go build ./backend/gmailfs/...` compiles; `NewFs` returns a
descriptive error (not panic) when `client_id`/`client_secret` absent;
`grep ErrorPermissionDenied` not yet required (T08); no hardcoded credential constant.

## T02 — dirPattern tree (all regex levels)

**Scope.** `backend/gmailfs/pattern.go`: define `dirPattern`/`dirPatterns` (modelled on
`backend/googlephotos/pattern.go`), `mustCompile`, and `match(root, itemPath, isFile)`.
Compile the full tree:
- `^$` → year dirs (start_year..current year), `isFile:false`
- `^(\d{4})$` → month dirs `YYYY-MM` (01..12)
- `^\d{4}/(\d{4})-(\d{2})$` → day dirs `YYYY-MM-DD`
- `^\d{4}/\d{4}-\d{2}/(\d{4})-(\d{2})-(\d{2})$` → thread dirs (day-level List)
- `^\d{4}/\d{4}-\d{2}/\d{4}-\d{2}-\d{2}/([^/]+)$` → thread contents (thread-level List), `isFile:false`
- `^\d{4}/\d{4}-\d{2}/\d{4}-\d{2}-\d{2}/[^/]+/([^/]+)\.eml$` → `.eml` leaf, `isFile:true`
- `^\d{4}/\d{4}-\d{2}/\d{4}-\d{2}-\d{2}/[^/]+/attachments$` → attachments dir, `isFile:false`
- `^\d{4}/\d{4}-\d{2}/\d{4}-\d{2}-\d{2}/[^/]+/attachments/([^/]+)$` → attachment leaf, `isFile:true`

**Rules.** D-1, D-2, D-3.
**Done when.** Every level has a compiled regex; `match` returns `(nil,"",nil)` for
unrecognized paths and distinguishes `isFile` before dispatch.

## T03 — List implementation

**Scope.** `List(ctx, dir)`: route via `patterns.match`; for the year/month/day
synthetic levels return generated dir entries (years/months/days helpers). For the
day level, call `threads.list` with `q:"after:YYYY/MM/DD before:YYYY/MM/DD+1"`,
exhausting all `nextPageToken` pages (Decision 3); for each thread fetch its subject
(via `threads.get` minimal or the snippet) and emit `{threadId} — {Subject}` dirs.
For the thread level, call `threads.get(threadId, format=FULL)`, emit one
`{messageId} — {Subject}.eml` per message and an `attachments/` dir if any message
has an attachment part. For the attachments dir, list `{messageId} — {filename}` per
attachment part. All names passed through `f.opt.Enc.FromStandardPath`. Unmatched dir
→ `fs.ErrorDirNotFound`.

**Rules.** D-1, D-2, E-1, P-1, S-3.
**Done when.** Each routed level returns the correct entry shape; pagination
exhausts pages; empty results return empty slice (not error) for valid dirs.

## T04 — NewObject path resolution

**Scope.** `NewObject(ctx, remote)`: match with `isFile=true`. If no file pattern
matches → `fs.ErrorObjectNotFound` (Decision 5 — a directory path finds no file
pattern, so returns ObjectNotFound). Parse threadId/messageId/(attachment filename)
from the path, build an `Object` with `isAttachment` set appropriately, and populate
identity fields. Fetch metadata lazily (readMetaData) or eagerly enough to confirm
existence; a path whose IDs don't resolve → `fs.ErrorObjectNotFound`.

**Rules.** D-2, D-3.
**Done when.** Valid `.eml`/attachment paths resolve to an `Object`; invalid or
directory paths return `fs.ErrorObjectNotFound`; never panics.

## T05 — Open for `.eml` (RFC 2822 synthesis)

**Scope.** `Object.Open` for `.eml`: fetch the message via `messages.get` /
reuse `threads.get` payload, then synthesize an RFC 2822 stream from the Gmail
payload — copy `Date`, `From`, `To`, `Cc`, `Subject`, `Message-ID` headers; emit
`MIME-Version: 1.0`; choose `multipart/alternative` for text/html body alternatives,
`multipart/mixed` when attachments are present, with a unique non-colliding boundary
per nesting level; base64- or quoted-printable-encode non-ASCII parts. Stream
on-the-fly (no temp file). Build the bytes deterministically for a given payload.

**Rules.** S-1, S-3.
**Done when.** Output parses under `python3 -c "import email;
email.message_from_file(open('test.eml'))"` with no exception; nested boundaries are
distinct; required headers present.

## T06 — Open for attachments (base64url decode)

**Scope.** `Object.Open` for an attachment: call
`messages.attachments.get(messageId, attachmentId)`, base64url-decode the `data`
field, and return the raw bytes as an `io.ReadCloser`. Stream from the API response
(via a decoding reader) rather than buffering the whole body in memory.

**Rules.** Sprint must-not (no full-buffer); S-3; P-1.
**Done when.** Decoded bytes are byte-for-byte correct; decoding is base64url (not
standard base64); no whole-body buffer.

## T07 — Object metadata (ModTime, Size, Hash, MimeType, Storable)

**Scope.** `ModTime` = `internalDate` (ms epoch → `time.Time`) for both `.eml` and
attachments. `Size`: `.eml` = synthesized byte length or -1 (Decision 2 — -1 is
acceptable); attachment = decoded `size` from the Gmail part metadata. `Hash` =
`"", hash.ErrUnsupported`. `MimeType` = `message/rfc822` for `.eml`, the part's
`Content-Type` for attachments. `Storable` = true. `Fs.Hashes` = `hash.Set(hash.None)`.

**Rules.** R-3.
**Done when.** Each accessor returns the specified value; attachment Size is decoded
bytes (not base64-encoded length).

## T08 — Read-only enforcement

**Scope.** `Put`, `Mkdir`, `Rmdir`, `Remove`, `SetModTime` each return
`fs.ErrorPermissionDenied` with no side effects. (`Put` and `Mkdir`/`Rmdir` on `Fs`;
`Remove` and `SetModTime` on `Object`.)

**Rules.** R-1.
**Done when.** `grep -n ErrorPermissionDenied backend/gmailfs/*.go` shows all five;
unit tests confirm each returns exactly `fs.ErrorPermissionDenied`.
