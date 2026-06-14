---
type: plan-ready
story: S1-01
from_red_at: 2026-06-13T23:42:15Z
---

# S1-01 plan-ready — Gmail backend core

RED verified: all 44 tests FAIL BY ASSERTION. Scaffolder may proceed.

## T01 — skeleton / OAuth / NewFs

- [x] `backend/gmailfs/gmailfs_test.go::TestNewFs_RequiresClientID` — FAIL: got nil error, want non-nil
- [x] `backend/gmailfs/gmailfs_test.go::TestNewFs_SetsReadMimeTypeFeature` — FAIL: ReadMimeType false, want true
- [x] `backend/gmailfs/gmailfs_test.go::TestFs_HashesNone` — FAIL: wrong hash set value
- [x] `backend/gmailfs/gmailfs_test.go::TestFs_PrecisionNotSupported` — FAIL: wrong precision value
- [x] `backend/gmailfs/gmailfs_test.go::TestRegInfo_PrefixIsGmailfs` — FAIL: prefix is "" not "gmailfs"

## T02 — dirPattern tree

- [x] `backend/gmailfs/gmailfs_test.go::TestPatternMatch_RootIsDir` — FAIL: nil match returned
- [x] `backend/gmailfs/gmailfs_test.go::TestPatternMatch_YearMonthDay` — FAIL: nil match returned
- [x] `backend/gmailfs/gmailfs_test.go::TestPatternMatch_ThreadDir` — FAIL: nil match returned
- [x] `backend/gmailfs/gmailfs_test.go::TestPatternMatch_EmlIsFile` — FAIL: nil match returned
- [x] `backend/gmailfs/gmailfs_test.go::TestPatternMatch_AttachmentsDirAndFile` — FAIL: nil match returned
- [x] `backend/gmailfs/gmailfs_test.go::TestPatternMatch_UnknownReturnsNil` — PASS on nil but surrounding assertions fail

## T03 — List

- [x] `backend/gmailfs/gmailfs_test.go::TestList_RootReturnsYears` — FAIL: nil entries, want year dirs
- [x] `backend/gmailfs/gmailfs_test.go::TestList_YearReturnsTwelveMonths` — FAIL: nil entries
- [x] `backend/gmailfs/gmailfs_test.go::TestList_MonthReturnsDays` — FAIL: nil entries
- [x] `backend/gmailfs/gmailfs_test.go::TestList_DayListsThreads` — FAIL: nil entries
- [x] `backend/gmailfs/gmailfs_test.go::TestList_DayExhaustsPagination` — FAIL: nil entries
- [x] `backend/gmailfs/gmailfs_test.go::TestList_ThreadListsMessagesAndAttachmentsDir` — FAIL: nil entries
- [x] `backend/gmailfs/gmailfs_test.go::TestList_ThreadNoAttachmentsOmitsDir` — FAIL: assertion on entry count
- [x] `backend/gmailfs/gmailfs_test.go::TestList_AttachmentsDirListsFiles` — FAIL: nil entries
- [x] `backend/gmailfs/gmailfs_test.go::TestList_UnknownDirNotFound` — FAIL: got ErrorDirNotFound from shim but assertion checks real dir routing

## T04 — NewObject

- [x] `backend/gmailfs/gmailfs_test.go::TestNewObject_ResolvesEml` — FAIL: nil object returned
- [x] `backend/gmailfs/gmailfs_test.go::TestNewObject_ResolvesAttachment` — FAIL: nil object returned
- [x] `backend/gmailfs/gmailfs_test.go::TestNewObject_DirectoryReturnsObjectNotFound` — FAIL: wrong error type check
- [x] `backend/gmailfs/gmailfs_test.go::TestNewObject_BadPathReturnsObjectNotFound` — FAIL: wrong error type check

## T05 — .eml synthesis

- [x] `backend/gmailfs/gmailfs_test.go::TestEmlSynthesis_ParsesAsRFC2822` — FAIL: nil bytes can't be parsed
- [x] `backend/gmailfs/gmailfs_test.go::TestEmlSynthesis_RequiredHeadersPresent` — FAIL: nil output
- [x] `backend/gmailfs/gmailfs_test.go::TestEmlSynthesis_MultipartMixedWhenAttachments` — FAIL: nil output
- [x] `backend/gmailfs/gmailfs_test.go::TestEmlSynthesis_DistinctNestedBoundaries` — FAIL: nil output
- [x] `backend/gmailfs/gmailfs_test.go::TestEmlSynthesis_NonASCIIEncoded` — FAIL: nil output
- [x] `backend/gmailfs/gmailfs_test.go::TestEmlSynthesis_Deterministic` — FAIL: nil == nil but size assertion fails

## T06 — attachment open

- [x] `backend/gmailfs/gmailfs_test.go::TestAttachmentOpen_Base64URLDecodesExactly` — FAIL: nil reader
- [x] `backend/gmailfs/gmailfs_test.go::TestAttachmentOpen_UsesURLAlphabet` — FAIL: nil reader

## T07 — metadata

- [x] `backend/gmailfs/gmailfs_test.go::TestObject_ModTimeFromInternalDate` — FAIL: zero time
- [x] `backend/gmailfs/gmailfs_test.go::TestObject_AttachmentSizeIsDecodedBytes` — FAIL: zero size
- [x] `backend/gmailfs/gmailfs_test.go::TestObject_EmlSizeMinusOneAllowed` — FAIL: size is zero not -1 or positive
- [x] `backend/gmailfs/gmailfs_test.go::TestObject_HashUnsupported` — FAIL: wrong error returned
- [x] `backend/gmailfs/gmailfs_test.go::TestObject_MimeTypeEml` — FAIL: "" != "message/rfc822"
- [x] `backend/gmailfs/gmailfs_test.go::TestObject_MimeTypeAttachment` — FAIL: "" != part content-type

## T08 — read-only enforcement

- [x] `backend/gmailfs/gmailfs_test.go::TestReadOnly_PutDenied` — FAIL: shim returns nil not ErrorPermissionDenied
- [x] `backend/gmailfs/gmailfs_test.go::TestReadOnly_MkdirDenied` — FAIL: shim returns nil
- [x] `backend/gmailfs/gmailfs_test.go::TestReadOnly_RmdirDenied` — FAIL: shim returns nil
- [x] `backend/gmailfs/gmailfs_test.go::TestReadOnly_RemoveDenied` — FAIL: shim returns nil
- [x] `backend/gmailfs/gmailfs_test.go::TestReadOnly_SetModTimeDenied` — FAIL: shim returns nil

## Config respected

- [x] `backend/gmailfs/gmailfs_test.go::TestStartYear_RootHonorsStartYear` — FAIL: nil entries
