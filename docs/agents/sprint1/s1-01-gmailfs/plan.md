---
type: plan
story: S1-01
scope: "tests only"
---

# S1-01 Plan (tests only) — Gmail backend core

Tests ONLY. No production code in this story's RED phase. Each checkbox is one test,
one line. Tests live in `backend/gmailfs/gmailfs_test.go` (and a small `mock`/fixture
helper in the same package) and fail by assertion against minimal `mod-common` shims
until SCAFFOLD/GREEN.

## T01 — skeleton / OAuth / NewFs

- [ ] `backend/gmailfs/gmailfs_test.go::TestNewFs_RequiresClientID` — NewFs with empty `client_id` config returns a descriptive non-nil error, not a panic.
- [ ] `backend/gmailfs/gmailfs_test.go::TestNewFs_SetsReadMimeTypeFeature` — `Features().ReadMimeType` is true and write-mime features stay false.
- [ ] `backend/gmailfs/gmailfs_test.go::TestFs_HashesNone` — `Fs.Hashes()` equals `hash.Set(hash.None)`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestFs_PrecisionNotSupported` — `Precision()` equals `fs.ModTimeNotSupported`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestRegInfo_PrefixIsGmailfs` — registered `fs.RegInfo.Prefix` is `"gmailfs"` (must match config.yaml).

## T02 — dirPattern tree

- [ ] `backend/gmailfs/gmailfs_test.go::TestPatternMatch_RootIsDir` — `""` matches the root pattern with `isFile=false`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestPatternMatch_YearMonthDay` — `2024`, `2024/2024-01`, `2024/2024-01/2024-01-15` each match their level as directories.
- [ ] `backend/gmailfs/gmailfs_test.go::TestPatternMatch_ThreadDir` — a thread path matches as a directory and captures the thread segment.
- [ ] `backend/gmailfs/gmailfs_test.go::TestPatternMatch_EmlIsFile` — a `…/<msg> — <Subj>.eml` path matches with `isFile=true`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestPatternMatch_AttachmentsDirAndFile` — `…/attachments` matches as dir; `…/attachments/<file>` matches as file.
- [ ] `backend/gmailfs/gmailfs_test.go::TestPatternMatch_UnknownReturnsNil` — an unrecognized path returns a nil pattern.

## T03 — List

- [ ] `backend/gmailfs/gmailfs_test.go::TestList_RootReturnsYears` — `List("")` returns year dirs from `start_year` to current year inclusive.
- [ ] `backend/gmailfs/gmailfs_test.go::TestList_YearReturnsTwelveMonths` — `List("2024")` returns 12 `2024-MM` dirs.
- [ ] `backend/gmailfs/gmailfs_test.go::TestList_MonthReturnsDays` — `List("2024/2024-01")` returns the 31 days of January as `2024-01-DD` dirs.
- [ ] `backend/gmailfs/gmailfs_test.go::TestList_DayListsThreads` — day-level List against a mocked `threads.list` returns one dir per thread named `<id> — <Subject>`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestList_DayExhaustsPagination` — a two-page mocked `threads.list` returns threads from both pages.
- [ ] `backend/gmailfs/gmailfs_test.go::TestList_ThreadListsMessagesAndAttachmentsDir` — thread-level List returns one `.eml` per message plus an `attachments/` dir when an attachment exists.
- [ ] `backend/gmailfs/gmailfs_test.go::TestList_ThreadNoAttachmentsOmitsDir` — thread with no attachments returns no `attachments/` dir.
- [ ] `backend/gmailfs/gmailfs_test.go::TestList_AttachmentsDirListsFiles` — attachments-dir List returns one `<msg> — <filename>` entry per attachment part.
- [ ] `backend/gmailfs/gmailfs_test.go::TestList_UnknownDirNotFound` — an unmatched directory path returns `fs.ErrorDirNotFound`.

## T04 — NewObject

- [ ] `backend/gmailfs/gmailfs_test.go::TestNewObject_ResolvesEml` — a valid `.eml` path resolves to an Object whose `Remote()` matches.
- [ ] `backend/gmailfs/gmailfs_test.go::TestNewObject_ResolvesAttachment` — a valid attachment path resolves to an Object with `isAttachment` set.
- [ ] `backend/gmailfs/gmailfs_test.go::TestNewObject_DirectoryReturnsObjectNotFound` — `NewObject("2024/01")` returns `fs.ErrorObjectNotFound` (Decision 5).
- [ ] `backend/gmailfs/gmailfs_test.go::TestNewObject_BadPathReturnsObjectNotFound` — `2024/01/15/badthread/nomessage.eml` returns `fs.ErrorObjectNotFound`, never panics.

## T05 — `.eml` synthesis

- [ ] `backend/gmailfs/gmailfs_test.go::TestEmlSynthesis_ParsesAsRFC2822` — synthesized `.eml` bytes parse via `net/mail`/`mime/multipart` with no error.
- [ ] `backend/gmailfs/gmailfs_test.go::TestEmlSynthesis_RequiredHeadersPresent` — output contains `Date`, `From`, `To`, `Subject`, `Message-ID`, `MIME-Version`, `Content-Type`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestEmlSynthesis_MultipartMixedWhenAttachments` — a message with attachments yields `multipart/mixed`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestEmlSynthesis_DistinctNestedBoundaries` — nested multiparts use distinct boundary strings.
- [ ] `backend/gmailfs/gmailfs_test.go::TestEmlSynthesis_NonASCIIEncoded` — a non-ASCII body part is base64 or quoted-printable encoded, not raw 8-bit.
- [ ] `backend/gmailfs/gmailfs_test.go::TestEmlSynthesis_Deterministic` — the same Gmail payload produces identical bytes on two calls.

## T06 — attachment open

- [ ] `backend/gmailfs/gmailfs_test.go::TestAttachmentOpen_Base64URLDecodesExactly` — a mocked `messages.attachments.get` base64url payload decodes byte-for-byte to the expected bytes.
- [ ] `backend/gmailfs/gmailfs_test.go::TestAttachmentOpen_UsesURLAlphabet` — a payload with `-`/`_` characters decodes correctly (proves base64url, not standard base64).

## T07 — metadata

- [ ] `backend/gmailfs/gmailfs_test.go::TestObject_ModTimeFromInternalDate` — ModTime equals the ms-epoch `internalDate` as `time.Time`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestObject_AttachmentSizeIsDecodedBytes` — attachment `Size()` equals the decoded byte count, not the base64 length.
- [ ] `backend/gmailfs/gmailfs_test.go::TestObject_EmlSizeMinusOneAllowed` — `.eml` `Size()` returns either the synthesized length or -1 (both accepted).
- [ ] `backend/gmailfs/gmailfs_test.go::TestObject_HashUnsupported` — `Hash(ctx, hash.MD5)` returns `("", hash.ErrUnsupported)`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestObject_MimeTypeEml` — `.eml` MimeType is `message/rfc822`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestObject_MimeTypeAttachment` — attachment MimeType is the part's Content-Type.

## T08 — read-only enforcement

- [ ] `backend/gmailfs/gmailfs_test.go::TestReadOnly_PutDenied` — `Put` returns `fs.ErrorPermissionDenied`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestReadOnly_MkdirDenied` — `Mkdir` returns `fs.ErrorPermissionDenied`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestReadOnly_RmdirDenied` — `Rmdir` returns `fs.ErrorPermissionDenied`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestReadOnly_RemoveDenied` — `Object.Remove` returns `fs.ErrorPermissionDenied`.
- [ ] `backend/gmailfs/gmailfs_test.go::TestReadOnly_SetModTimeDenied` — `Object.SetModTime` returns `fs.ErrorPermissionDenied`.

## Config respected

- [ ] `backend/gmailfs/gmailfs_test.go::TestStartYear_RootHonorsStartYear` — with `start_year=2020`, `List("")` returns no year before 2020.
