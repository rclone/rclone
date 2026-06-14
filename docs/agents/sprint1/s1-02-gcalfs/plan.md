---
type: plan
story: S1-02
scope: "tests only"
---

# S1-02 Plan (tests only) — Google Calendar backend core

Tests ONLY. No production code in this story's RED phase. Each checkbox is one test,
one line. Tests live in `backend/gcalfs/gcalfs_test.go` (plus mock/fixture helpers in
the same package) and fail by assertion against minimal shims until SCAFFOLD/GREEN.

## T01 — skeleton / OAuth / NewFs

- [ ] `backend/gcalfs/gcalfs_test.go::TestNewFs_RequiresClientID` — NewFs with empty `client_id` config returns a descriptive non-nil error, not a panic.
- [ ] `backend/gcalfs/gcalfs_test.go::TestNewFs_SetsReadMimeTypeFeature` — `Features().ReadMimeType` is true; write-mime features stay false.
- [ ] `backend/gcalfs/gcalfs_test.go::TestFs_HashesNone` — `Fs.Hashes()` equals `hash.Set(hash.None)`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestFs_PrecisionNotSupported` — `Precision()` equals `fs.ModTimeNotSupported`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestRegInfo_PrefixIsGcalfs` — registered `fs.RegInfo.Prefix` is `"gcalfs"` (must match config.yaml).

## T02 — dirPattern tree

- [ ] `backend/gcalfs/gcalfs_test.go::TestPatternMatch_RootIsDir` — `""` matches the root pattern with `isFile=false`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestPatternMatch_CalendarCaptured` — `My Calendar` matches the dynamic root capture group as a directory.
- [ ] `backend/gcalfs/gcalfs_test.go::TestPatternMatch_YearMonthDay` — `<cal>/2024`, `<cal>/2024/2024-01`, `<cal>/2024/2024-01/2024-01-15` each match their level as directories.
- [ ] `backend/gcalfs/gcalfs_test.go::TestPatternMatch_IcsIsFile` — `<cal>/2024/2024-01/2024-01-15/<id> — <Summary>.ics` matches with `isFile=true`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestPatternMatch_UnknownReturnsNil` — an unrecognized path returns a nil pattern.

## T03 — root List / caching / disambiguation

- [ ] `backend/gcalfs/gcalfs_test.go::TestRootList_OneDirPerCalendar` — `List("")` against a mocked `calendarList.list` returns one dir per calendar named by `summary`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestRootList_PaginationExhausted` — a two-page calendar list returns all calendars.
- [ ] `backend/gcalfs/gcalfs_test.go::TestRootList_DisambiguatesDuplicateSummary` — two calendars sharing a summary get ` {calendarId[:8]}` suffixes (Decision 4).
- [ ] `backend/gcalfs/gcalfs_test.go::TestCache_ResolvesNameToCalendarID` — the per-Fs cache maps a directory-name segment back to its calendar ID.

## T04 — day List

- [ ] `backend/gcalfs/gcalfs_test.go::TestDayList_OneIcsPerEvent` — day-level List against a mocked `events.list` returns one `<id> — <Summary>.ics` per event.
- [ ] `backend/gcalfs/gcalfs_test.go::TestDayList_SendsSingleEventsTrue` — the request carries `singleEvents=true`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestDayList_PaginationExhausted` — a two-page event list returns all events.
- [ ] `backend/gcalfs/gcalfs_test.go::TestDayList_EmptyDayReturnsEmptySlice` — a day with no events returns an empty slice and no error.

## T05 — NewObject

- [ ] `backend/gcalfs/gcalfs_test.go::TestNewObject_ResolvesIcs` — a valid `.ics` path resolves to an Object whose `Remote()` matches.
- [ ] `backend/gcalfs/gcalfs_test.go::TestNewObject_DirectoryReturnsObjectNotFound` — `NewObject("My Calendar/2024")` returns `fs.ErrorObjectNotFound` (Decision 5).
- [ ] `backend/gcalfs/gcalfs_test.go::TestNewObject_BadPathReturnsObjectNotFound` — a nonexistent event path returns `fs.ErrorObjectNotFound`, never panics.

## T06 — `.ics` synthesis

- [ ] `backend/gcalfs/gcalfs_test.go::TestIcsSynthesis_AllLinesCRLF` — every line of the synthesized `.ics` ends with `\r\n`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestIcsSynthesis_MandatoryProperties` — output contains VCALENDAR/VERSION/PRODID/VEVENT/UID/DTSTAMP/DTSTART/DTEND/SUMMARY wrappers.
- [ ] `backend/gcalfs/gcalfs_test.go::TestIcsSynthesis_LineFoldingAt75Octets` — a long DESCRIPTION folds with CRLF + single space at ≤75 octets.
- [ ] `backend/gcalfs/gcalfs_test.go::TestIcsSynthesis_AllDayUsesValueDate` — an all-day event renders `DTSTART;VALUE=DATE:YYYYMMDD` with no time component.
- [ ] `backend/gcalfs/gcalfs_test.go::TestIcsSynthesis_TimedEventUsesUTC` — a timed event renders `DTSTART:YYYYMMDDTHHMMSSZ`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestIcsSynthesis_DeterministicExceptDtstamp` — same payload produces identical bytes apart from DTSTAMP.

## T07 — metadata

- [ ] `backend/gcalfs/gcalfs_test.go::TestObject_ModTimeFromUpdated` — ModTime equals the event `updated` RFC 3339 value as `time.Time`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestObject_SizeMinusOneAllowed` — `Size()` returns the synthesized length or -1 (both accepted).
- [ ] `backend/gcalfs/gcalfs_test.go::TestObject_HashUnsupported` — `Hash(ctx, hash.MD5)` returns `("", hash.ErrUnsupported)`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestObject_MimeTypeCalendar` — MimeType is `text/calendar`.

## T08 — read-only enforcement

- [ ] `backend/gcalfs/gcalfs_test.go::TestReadOnly_PutDenied` — `Put` returns `fs.ErrorPermissionDenied`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestReadOnly_MkdirDenied` — `Mkdir` returns `fs.ErrorPermissionDenied`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestReadOnly_RmdirDenied` — `Rmdir` returns `fs.ErrorPermissionDenied`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestReadOnly_RemoveDenied` — `Object.Remove` returns `fs.ErrorPermissionDenied`.
- [ ] `backend/gcalfs/gcalfs_test.go::TestReadOnly_SetModTimeDenied` — `Object.SetModTime` returns `fs.ErrorPermissionDenied`.

## Config respected

- [ ] `backend/gcalfs/gcalfs_test.go::TestStartYear_YearListHonorsStartYear` — with `start_year=2022`, the year listing under a calendar returns no year before 2022.
