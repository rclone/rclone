package ftp

import (
	"strings"
	"testing"
	"time"
)

var (
	// now is the current time for all tests
	now = newTime(2017, time.March, 10, 23, 00)

	thisYear, _, _ = now.Date()
	previousYear   = thisYear - 1
)

type line struct {
	line      string
	name      string
	size      uint64
	entryType EntryType
	time      time.Time
}

type unsupportedLine struct {
	line string
	err  string
}

var listTests = []line{
	// UNIX ls -l style
	{"drwxr-xr-x    3 110      1002            3 Dec 02  2009 pub", "pub", 0, EntryTypeFolder, newTime(2009, time.December, 2)},
	{"drwxr-xr-x    3 110      1002            3 Dec 02  2009 p u b", "p u b", 0, EntryTypeFolder, newTime(2009, time.December, 2)},
	{"-rw-r--r--   1 marketwired marketwired    12016 Mar 16  2016 2016031611G087802-001.newsml", "2016031611G087802-001.newsml", 12016, EntryTypeFile, newTime(2016, time.March, 16)},

	{"-rwxr-xr-x    3 110      1002            1234567 Dec 02  2009 fileName", "fileName", 1234567, EntryTypeFile, newTime(2009, time.December, 2)},
	{"lrwxrwxrwx   1 root     other          7 Jan 25 00:17 bin -> usr/bin", "bin -> usr/bin", 0, EntryTypeLink, newTime(thisYear, time.January, 25, 0, 17)},

	// Another ls style
	{"drwxr-xr-x               folder        0 Aug 15 05:49 !!!-Tipp des Haus!", "!!!-Tipp des Haus!", 0, EntryTypeFolder, newTime(thisYear, time.August, 15, 5, 49)},
	{"drwxrwxrwx               folder        0 Aug 11 20:32 P0RN", "P0RN", 0, EntryTypeFolder, newTime(thisYear, time.August, 11, 20, 32)},
	{"-rw-r--r--        0   18446744073709551615 18446744073709551615 Nov 16  2006 VIDEO_TS.VOB", "VIDEO_TS.VOB", 18446744073709551615, EntryTypeFile, newTime(2006, time.November, 16)},

	// Microsoft's FTP servers for Windows
	{"----------   1 owner    group         1803128 Jul 10 10:18 ls-lR.Z", "ls-lR.Z", 1803128, EntryTypeFile, newTime(thisYear, time.July, 10, 10, 18)},
	{"d---------   1 owner    group               0 Nov  9 19:45 Softlib", "Softlib", 0, EntryTypeFolder, newTime(previousYear, time.November, 9, 19, 45)},

	// WFTPD for MSDOS
	{"-rwxrwxrwx   1 noone    nogroup      322 Aug 19  1996 message.ftp", "message.ftp", 322, EntryTypeFile, newTime(1996, time.August, 19)},

	// RFC3659 format: https://tools.ietf.org/html/rfc3659#section-7
	{"modify=20150813224845;perm=fle;type=cdir;unique=119FBB87U4;UNIX.group=0;UNIX.mode=0755;UNIX.owner=0; .", ".", 0, EntryTypeFolder, newTime(2015, time.August, 13, 22, 48, 45)},
	{"modify=20150813224845;perm=fle;type=pdir;unique=119FBB87U4;UNIX.group=0;UNIX.mode=0755;UNIX.owner=0; ..", "..", 0, EntryTypeFolder, newTime(2015, time.August, 13, 22, 48, 45)},
	{"modify=20150806235817;perm=fle;type=dir;unique=1B20F360U4;UNIX.group=0;UNIX.mode=0755;UNIX.owner=0; movies", "movies", 0, EntryTypeFolder, newTime(2015, time.August, 6, 23, 58, 17)},
	{"modify=20150814172949;perm=flcdmpe;type=dir;unique=85A0C168U4;UNIX.group=0;UNIX.mode=0777;UNIX.owner=0; _upload", "_upload", 0, EntryTypeFolder, newTime(2015, time.August, 14, 17, 29, 49)},
	{"modify=20150813175250;perm=adfr;size=951;type=file;unique=119FBB87UE;UNIX.group=0;UNIX.mode=0644;UNIX.owner=0; welcome.msg", "welcome.msg", 951, EntryTypeFile, newTime(2015, time.August, 13, 17, 52, 50)},
	// Format and types have first letter UpperCase
	{"Modify=20150813175250;Perm=adfr;Size=951;Type=file;Unique=119FBB87UE;UNIX.group=0;UNIX.mode=0644;UNIX.owner=0; welcome.msg", "welcome.msg", 951, EntryTypeFile, newTime(2015, time.August, 13, 17, 52, 50)},

	// DOS DIR command output
	{"08-07-15  07:50PM                  718 Post_PRR_20150901_1166_265118_13049.dat", "Post_PRR_20150901_1166_265118_13049.dat", 718, EntryTypeFile, newTime(2015, time.August, 7, 19, 50)},
	{"08-10-15  02:04PM       <DIR>          Billing", "Billing", 0, EntryTypeFolder, newTime(2015, time.August, 10, 14, 4)},

	// dir and file names that contain multiple spaces
	{"drwxr-xr-x    3 110      1002            3 Dec 02  2009 spaces   dir   name", "spaces   dir   name", 0, EntryTypeFolder, newTime(2009, time.December, 2)},
	{"-rwxr-xr-x    3 110      1002            1234567 Dec 02  2009 file   name", "file   name", 1234567, EntryTypeFile, newTime(2009, time.December, 2)},
	{"-rwxr-xr-x    3 110      1002            1234567 Dec 02  2009  foo bar ", " foo bar ", 1234567, EntryTypeFile, newTime(2009, time.December, 2)},

	// Odd link count from hostedftp.com
	{"-r--------   0 user group     65222236 Feb 24 00:39 RegularFile", "RegularFile", 65222236, EntryTypeFile, newTime(thisYear, time.February, 24, 0, 39)},
}

// Not supported, we expect a specific error message
var listTestsFail = []unsupportedLine{
	{"d [R----F--] supervisor            512       Jan 16 18:53 login", "Unsupported LIST line"},
	{"- [R----F--] rhesus             214059       Oct 20 15:27 cx.exe", "Unsupported LIST line"},
	{"drwxr-xr-x    3 110      1002            3 Dec 02  209 pub", "Invalid year format in time string"},
	{"modify=20150806235817;invalid;UNIX.owner=0; movies", "Unsupported LIST line"},
	{"Zrwxrwxrwx   1 root     other          7 Jan 25 00:17 bin -> usr/bin", "Unknown entry type"},
	{"total 1", "Unsupported LIST line"},
	{"000000000x ", "Unsupported LIST line"}, // see https://github.com/jlaffaye/ftp/issues/97
	{"", "Unsupported LIST line"},
}

func TestParseValidListLine(t *testing.T) {
	for _, lt := range listTests {
		entry, err := parseListLine(lt.line, now, time.UTC)
		if err != nil {
			t.Errorf("parseListLine(%v) returned err = %v", lt.line, err)
			continue
		}
		if entry.Name != lt.name {
			t.Errorf("parseListLine(%v).Name = '%v', want '%v'", lt.line, entry.Name, lt.name)
		}
		if entry.Type != lt.entryType {
			t.Errorf("parseListLine(%v).EntryType = %v, want %v", lt.line, entry.Type, lt.entryType)
		}
		if entry.Size != lt.size {
			t.Errorf("parseListLine(%v).Size = %v, want %v", lt.line, entry.Size, lt.size)
		}
		if !entry.Time.Equal(lt.time) {
			t.Errorf("parseListLine(%v).Time = %v, want %v", lt.line, entry.Time, lt.time)
		}
	}
}

func TestParseUnsupportedListLine(t *testing.T) {
	for _, lt := range listTestsFail {
		_, err := parseListLine(lt.line, now, time.UTC)
		if err == nil {
			t.Errorf("parseListLine(%v) expected to fail", lt.line)
		}
		if err.Error() != lt.err {
			t.Errorf("parseListLine(%v) expected to fail with error: '%s'; was: '%s'", lt.line, lt.err, err.Error())
		}
	}
}

func TestSettime(t *testing.T) {
	tests := []struct {
		line     string
		expected time.Time
	}{
		// this year, in the past
		{"Feb 10 23:00", newTime(thisYear, time.February, 10, 23)},

		// this year, less than six months in the future
		{"Sep 10 22:59", newTime(thisYear, time.September, 10, 22, 59)},

		// previous year, otherwise it would be more than 6 months in the future
		{"Sep 10 23:00", newTime(previousYear, time.September, 10, 23)},

		// far in the future
		{"Jan 23  2019", newTime(2019, time.January, 23)},
	}

	for _, test := range tests {
		entry := &Entry{}
		entry.setTime(strings.Fields(test.line), now, time.UTC)

		if !entry.Time.Equal(test.expected) {
			t.Errorf("setTime(%v).Time = %v, want %v", test.line, entry.Time, test.expected)
		}
	}
}

// newTime builds a UTC time from the given year, month, day, hour and minute
func newTime(year int, month time.Month, day int, hourMinSec ...int) time.Time {
	var hour, min, sec int

	switch len(hourMinSec) {
	case 0:
		// nothing
	case 3:
		sec = hourMinSec[2]
		fallthrough
	case 2:
		min = hourMinSec[1]
		fallthrough
	case 1:
		hour = hourMinSec[0]
	default:
		panic("too many arguments")
	}

	return time.Date(year, month, day, hour, min, sec, 0, time.UTC)
}
