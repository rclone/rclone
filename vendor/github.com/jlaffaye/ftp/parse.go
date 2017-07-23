package ftp

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

var errUnsupportedListLine = errors.New("Unsupported LIST line")

var listLineParsers = []func(line string) (*Entry, error){
	parseRFC3659ListLine,
	parseLsListLine,
	parseDirListLine,
	parseHostedFTPLine,
}

var dirTimeFormats = []string{
	"01-02-06  03:04PM",
	"2006-01-02  15:04",
}

// parseRFC3659ListLine parses the style of directory line defined in RFC 3659.
func parseRFC3659ListLine(line string) (*Entry, error) {
	iSemicolon := strings.Index(line, ";")
	iWhitespace := strings.Index(line, " ")

	if iSemicolon < 0 || iSemicolon > iWhitespace {
		return nil, errUnsupportedListLine
	}

	e := &Entry{
		Name: line[iWhitespace+1:],
	}

	for _, field := range strings.Split(line[:iWhitespace-1], ";") {
		i := strings.Index(field, "=")
		if i < 1 {
			return nil, errUnsupportedListLine
		}

		key := field[:i]
		value := field[i+1:]

		switch key {
		case "modify":
			var err error
			e.Time, err = time.Parse("20060102150405", value)
			if err != nil {
				return nil, err
			}
		case "type":
			switch value {
			case "dir", "cdir", "pdir":
				e.Type = EntryTypeFolder
			case "file":
				e.Type = EntryTypeFile
			}
		case "size":
			e.setSize(value)
		}
	}
	return e, nil
}

// parseLsListLine parses a directory line in a format based on the output of
// the UNIX ls command.
func parseLsListLine(line string) (*Entry, error) {

	// Has the first field a length of 10 bytes?
	if strings.IndexByte(line, ' ') != 10 {
		return nil, errUnsupportedListLine
	}

	scanner := newScanner(line)
	fields := scanner.NextFields(6)

	if len(fields) < 6 {
		return nil, errUnsupportedListLine
	}

	if fields[1] == "folder" && fields[2] == "0" {
		e := &Entry{
			Type: EntryTypeFolder,
			Name: scanner.Remaining(),
		}
		if err := e.setTime(fields[3:6]); err != nil {
			return nil, err
		}

		return e, nil
	}

	if fields[1] == "0" {
		fields = append(fields, scanner.Next())
		e := &Entry{
			Type: EntryTypeFile,
			Name: scanner.Remaining(),
		}

		if err := e.setSize(fields[2]); err != nil {
			return nil, errUnsupportedListLine
		}
		if err := e.setTime(fields[4:7]); err != nil {
			return nil, err
		}

		return e, nil
	}

	// Read two more fields
	fields = append(fields, scanner.NextFields(2)...)
	if len(fields) < 8 {
		return nil, errUnsupportedListLine
	}

	e := &Entry{
		Name: scanner.Remaining(),
	}
	switch fields[0][0] {
	case '-':
		e.Type = EntryTypeFile
		if err := e.setSize(fields[4]); err != nil {
			return nil, err
		}
	case 'd':
		e.Type = EntryTypeFolder
	case 'l':
		e.Type = EntryTypeLink
	default:
		return nil, errors.New("Unknown entry type")
	}

	if err := e.setTime(fields[5:8]); err != nil {
		return nil, err
	}

	return e, nil
}

// parseDirListLine parses a directory line in a format based on the output of
// the MS-DOS DIR command.
func parseDirListLine(line string) (*Entry, error) {
	e := &Entry{}
	var err error

	// Try various time formats that DIR might use, and stop when one works.
	for _, format := range dirTimeFormats {
		if len(line) > len(format) {
			e.Time, err = time.Parse(format, line[:len(format)])
			if err == nil {
				line = line[len(format):]
				break
			}
		}
	}
	if err != nil {
		// None of the time formats worked.
		return nil, errUnsupportedListLine
	}

	line = strings.TrimLeft(line, " ")
	if strings.HasPrefix(line, "<DIR>") {
		e.Type = EntryTypeFolder
		line = strings.TrimPrefix(line, "<DIR>")
	} else {
		space := strings.Index(line, " ")
		if space == -1 {
			return nil, errUnsupportedListLine
		}
		e.Size, err = strconv.ParseUint(line[:space], 10, 64)
		if err != nil {
			return nil, errUnsupportedListLine
		}
		e.Type = EntryTypeFile
		line = line[space:]
	}

	e.Name = strings.TrimLeft(line, " ")
	return e, nil
}

// parseHostedFTPLine parses a directory line in the non-standard format used
// by hostedftp.com
// -r--------   0 user group     65222236 Feb 24 00:39 UABlacklistingWeek8.csv
// (The link count is inexplicably 0)
func parseHostedFTPLine(line string) (*Entry, error) {
	// Has the first field a length of 10 bytes?
	if strings.IndexByte(line, ' ') != 10 {
		return nil, errUnsupportedListLine
	}

	scanner := newScanner(line)
	fields := scanner.NextFields(2)

	if len(fields) < 2 || fields[1] != "0" {
		return nil, errUnsupportedListLine
	}

	// Set link count to 1 and attempt to parse as Unix.
	return parseLsListLine(fields[0] + " 1 " + scanner.Remaining())
}

// parseListLine parses the various non-standard format returned by the LIST
// FTP command.
func parseListLine(line string) (*Entry, error) {
	for _, f := range listLineParsers {
		e, err := f(line)
		if err != errUnsupportedListLine {
			return e, err
		}
	}
	return nil, errUnsupportedListLine
}

func (e *Entry) setSize(str string) (err error) {
	e.Size, err = strconv.ParseUint(str, 0, 64)
	return
}

func (e *Entry) setTime(fields []string) (err error) {
	var timeStr string
	if strings.Contains(fields[2], ":") { // this year
		thisYear, _, _ := time.Now().Date()
		timeStr = fields[1] + " " + fields[0] + " " + strconv.Itoa(thisYear)[2:4] + " " + fields[2] + " GMT"
	} else { // not this year
		if len(fields[2]) != 4 {
			return errors.New("Invalid year format in time string")
		}
		timeStr = fields[1] + " " + fields[0] + " " + fields[2][2:4] + " 00:00 GMT"
	}
	e.Time, err = time.Parse("_2 Jan 06 15:04 MST", timeStr)
	return
}
