---
type: plan-ready
story: S1-02
from_red_at: 2026-06-13T23:41:12Z
---

# S1-02 plan-ready — Google Calendar backend core

RED verified: all 37 tests FAIL BY ASSERTION. Scaffolder may proceed.

## T01 — skeleton / OAuth / NewFs

- [x] `backend/gcalfs/gcalfs_test.go::TestNewFs_RequiresClientID` — FAIL: got nil error, want non-nil
- [x] `backend/gcalfs/gcalfs_test.go::TestNewFs_SetsReadMimeTypeFeature` — FAIL: ReadMimeType false
- [x] `backend/gcalfs/gcalfs_test.go::TestFs_HashesNone` — FAIL: wrong hash set
- [x] `backend/gcalfs/gcalfs_test.go::TestFs_PrecisionNotSupported` — FAIL: wrong precision
- [x] `backend/gcalfs/gcalfs_test.go::TestRegInfo_PrefixIsGcalfs` — FAIL: prefix "" not "gcalfs"

## T02 — dirPattern tree

- [x] `backend/gcalfs/gcalfs_test.go::TestPatternMatch_RootIsDir` — FAIL: nil match
- [x] `backend/gcalfs/gcalfs_test.go::TestPatternMatch_CalendarCaptured` — FAIL: nil match
- [x] `backend/gcalfs/gcalfs_test.go::TestPatternMatch_YearMonthDay` — FAIL: nil match
- [x] `backend/gcalfs/gcalfs_test.go::TestPatternMatch_IcsIsFile` — FAIL: nil match
- [x] `backend/gcalfs/gcalfs_test.go::TestPatternMatch_UnknownReturnsNil` — FAIL: surrounding assertions

## T03 — root List / caching / disambiguation

- [x] `backend/gcalfs/gcalfs_test.go::TestRootList_OneDirPerCalendar` — FAIL: nil entries
- [x] `backend/gcalfs/gcalfs_test.go::TestRootList_PaginationExhausted` — FAIL: nil entries
- [x] `backend/gcalfs/gcalfs_test.go::TestRootList_DisambiguatesDuplicateSummary` — FAIL: nil entries
- [x] `backend/gcalfs/gcalfs_test.go::TestCache_ResolvesNameToCalendarID` — FAIL: empty cache

## T04 — day List

- [x] `backend/gcalfs/gcalfs_test.go::TestDayList_OneIcsPerEvent` — FAIL: nil entries
- [x] `backend/gcalfs/gcalfs_test.go::TestDayList_SendsSingleEventsTrue` — FAIL: request not captured
- [x] `backend/gcalfs/gcalfs_test.go::TestDayList_PaginationExhausted` — FAIL: nil entries
- [x] `backend/gcalfs/gcalfs_test.go::TestDayList_EmptyDayReturnsEmptySlice` — FAIL: nil != empty slice assertion

## T05 — NewObject

- [x] `backend/gcalfs/gcalfs_test.go::TestNewObject_ResolvesIcs` — FAIL: nil object
- [x] `backend/gcalfs/gcalfs_test.go::TestNewObject_DirectoryReturnsObjectNotFound` — FAIL: wrong error check
- [x] `backend/gcalfs/gcalfs_test.go::TestNewObject_BadPathReturnsObjectNotFound` — FAIL: wrong error check

## T06 — .ics synthesis

- [x] `backend/gcalfs/gcalfs_test.go::TestIcsSynthesis_AllLinesCRLF` — FAIL: nil output
- [x] `backend/gcalfs/gcalfs_test.go::TestIcsSynthesis_MandatoryProperties` — FAIL: nil output
- [x] `backend/gcalfs/gcalfs_test.go::TestIcsSynthesis_LineFoldingAt75Octets` — FAIL: nil output
- [x] `backend/gcalfs/gcalfs_test.go::TestIcsSynthesis_AllDayUsesValueDate` — FAIL: nil output
- [x] `backend/gcalfs/gcalfs_test.go::TestIcsSynthesis_TimedEventUsesUTC` — FAIL: nil output
- [x] `backend/gcalfs/gcalfs_test.go::TestIcsSynthesis_DeterministicExceptDtstamp` — FAIL: nil output

## T07 — metadata

- [x] `backend/gcalfs/gcalfs_test.go::TestObject_ModTimeFromUpdated` — FAIL: zero time
- [x] `backend/gcalfs/gcalfs_test.go::TestObject_SizeMinusOneAllowed` — FAIL: zero not -1 or positive
- [x] `backend/gcalfs/gcalfs_test.go::TestObject_HashUnsupported` — FAIL: wrong error
- [x] `backend/gcalfs/gcalfs_test.go::TestObject_MimeTypeCalendar` — FAIL: "" != "text/calendar"

## T08 — read-only enforcement

- [x] `backend/gcalfs/gcalfs_test.go::TestReadOnly_PutDenied` — FAIL: shim returns nil
- [x] `backend/gcalfs/gcalfs_test.go::TestReadOnly_MkdirDenied` — FAIL: shim returns nil
- [x] `backend/gcalfs/gcalfs_test.go::TestReadOnly_RmdirDenied` — FAIL: shim returns nil
- [x] `backend/gcalfs/gcalfs_test.go::TestReadOnly_RemoveDenied` — FAIL: shim returns nil
- [x] `backend/gcalfs/gcalfs_test.go::TestReadOnly_SetModTimeDenied` — FAIL: shim returns nil

## Config respected

- [x] `backend/gcalfs/gcalfs_test.go::TestStartYear_YearListHonorsStartYear` — FAIL: nil entries
