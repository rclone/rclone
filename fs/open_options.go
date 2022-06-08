// Options for Open

package fs

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/rclone/rclone/fs/hash"
)

// OpenOption is an interface describing options for Open
type OpenOption interface {
	fmt.Stringer

	// Header returns the option as an HTTP header
	Header() (key string, value string)

	// Mandatory returns whether this option can be ignored or not
	Mandatory() bool
}

// RangeOption defines an HTTP Range option with start and end.  If
// either start or end are < 0 then they will be omitted.
//
// End may be bigger than the Size of the object in which case it will
// be capped to the size of the object.
//
// Note that the End is inclusive, so to fetch 100 bytes you would use
// RangeOption{Start: 0, End: 99}
//
// If Start is specified but End is not then it will fetch from Start
// to the end of the file.
//
// If End is specified, but Start is not then it will fetch the last
// End bytes.
//
// Examples:
//
//     RangeOption{Start: 0, End: 99} - fetch the first 100 bytes
//     RangeOption{Start: 100, End: 199} - fetch the second 100 bytes
//     RangeOption{Start: 100, End: -1} - fetch bytes from offset 100 to the end
//     RangeOption{Start: -1, End: 100} - fetch the last 100 bytes
//
// A RangeOption implements a single byte-range-spec from
// https://tools.ietf.org/html/rfc7233#section-2.1
type RangeOption struct {
	Start int64
	End   int64
}

// Header formats the option as an http header
func (o *RangeOption) Header() (key string, value string) {
	key = "Range"
	value = "bytes="
	if o.Start >= 0 {
		value += strconv.FormatInt(o.Start, 10)

	}
	value += "-"
	if o.End >= 0 {
		value += strconv.FormatInt(o.End, 10)
	}
	return key, value
}

// ParseRangeOption parses a RangeOption from a Range: header.
// It only accepts single ranges.
func ParseRangeOption(s string) (po *RangeOption, err error) {
	const preamble = "bytes="
	if !strings.HasPrefix(s, preamble) {
		return nil, errors.New("Range: header invalid: doesn't start with " + preamble)
	}
	s = s[len(preamble):]
	if strings.ContainsRune(s, ',') {
		return nil, errors.New("Range: header invalid: contains multiple ranges which isn't supported")
	}
	dash := strings.IndexRune(s, '-')
	if dash < 0 {
		return nil, errors.New("Range: header invalid: contains no '-'")
	}
	start, end := strings.TrimSpace(s[:dash]), strings.TrimSpace(s[dash+1:])
	o := RangeOption{Start: -1, End: -1}
	if start != "" {
		o.Start, err = strconv.ParseInt(start, 10, 64)
		if err != nil || o.Start < 0 {
			return nil, errors.New("Range: header invalid: bad start")
		}
	}
	if end != "" {
		o.End, err = strconv.ParseInt(end, 10, 64)
		if err != nil || o.End < 0 {
			return nil, errors.New("Range: header invalid: bad end")
		}
	}
	return &o, nil
}

// String formats the option into human-readable form
func (o *RangeOption) String() string {
	return fmt.Sprintf("RangeOption(%d,%d)", o.Start, o.End)
}

// Mandatory returns whether the option must be parsed or can be ignored
func (o *RangeOption) Mandatory() bool {
	return true
}

// Decode interprets the RangeOption into an offset and a limit
//
// The offset is the start of the stream and the limit is how many
// bytes should be read from it.  If the limit is -1 then the stream
// should be read to the end.
func (o *RangeOption) Decode(size int64) (offset, limit int64) {
	if o.Start >= 0 {
		offset = o.Start
		if o.End >= 0 {
			limit = o.End - o.Start + 1
		} else {
			limit = -1
		}
	} else {
		if o.End >= 0 {
			offset = size - o.End
		} else {
			offset = 0
		}
		limit = -1
	}
	return offset, limit
}

// FixRangeOption looks through the slice of options and adjusts any
// RangeOption~s found that request a fetch from the end into an
// absolute fetch using the size passed in and makes sure the range does
// not exceed filesize. Some remotes (e.g. Onedrive, Box) don't support
// range requests which index from the end.
//
// It also adjusts any SeekOption~s, turning them into absolute
// RangeOption~s instead.
func FixRangeOption(options []OpenOption, size int64) {
	if size < 0 {
		// Can't do anything for unknown length objects
		return
	} else if size == 0 {
		// if size 0 then remove RangeOption~s
		// replacing with a NullOptions~s which won't be rendered
		for i := range options {
			if _, ok := options[i].(*RangeOption); ok {
				options[i] = NullOption{}

			}
		}
		return
	}
	for i, option := range options {
		switch x := option.(type) {
		case *RangeOption:
			// If start is < 0 then fetch from the end
			if x.Start < 0 {
				x = &RangeOption{Start: size - x.End, End: -1}
				options[i] = x
			}
			// If end is too big or undefined, fetch to the end
			if x.End > size || x.End < 0 {
				x = &RangeOption{Start: x.Start, End: size - 1}
				options[i] = x
			}
		case *SeekOption:
			options[i] = &RangeOption{Start: x.Offset, End: size - 1}
		}
	}
}

// SeekOption defines an HTTP Range option with start only.
type SeekOption struct {
	Offset int64
}

// Header formats the option as an http header
func (o *SeekOption) Header() (key string, value string) {
	key = "Range"
	value = fmt.Sprintf("bytes=%d-", o.Offset)
	return key, value
}

// String formats the option into human-readable form
func (o *SeekOption) String() string {
	return fmt.Sprintf("SeekOption(%d)", o.Offset)
}

// Mandatory returns whether the option must be parsed or can be ignored
func (o *SeekOption) Mandatory() bool {
	return true
}

// HTTPOption defines a general purpose HTTP option
type HTTPOption struct {
	Key   string
	Value string
}

// Header formats the option as an http header
func (o *HTTPOption) Header() (key string, value string) {
	return o.Key, o.Value
}

// String formats the option into human-readable form
func (o *HTTPOption) String() string {
	return fmt.Sprintf("HTTPOption(%q,%q)", o.Key, o.Value)
}

// Mandatory returns whether the option must be parsed or can be ignored
func (o *HTTPOption) Mandatory() bool {
	return false
}

// HashesOption defines an option used to tell the local fs to limit
// the number of hashes it calculates.
type HashesOption struct {
	Hashes hash.Set
}

// Header formats the option as an http header
func (o *HashesOption) Header() (key string, value string) {
	return "", ""
}

// String formats the option into human-readable form
func (o *HashesOption) String() string {
	return fmt.Sprintf("HashesOption(%v)", o.Hashes)
}

// Mandatory returns whether the option must be parsed or can be ignored
func (o *HashesOption) Mandatory() bool {
	return false
}

// NullOption defines an Option which does nothing
type NullOption struct {
}

// Header formats the option as an http header
func (o NullOption) Header() (key string, value string) {
	return "", ""
}

// String formats the option into human-readable form
func (o NullOption) String() string {
	return "NullOption()"
}

// Mandatory returns whether the option must be parsed or can be ignored
func (o NullOption) Mandatory() bool {
	return false
}

// OpenOptionAddHeaders adds each header found in options to the
// headers map provided the key was non empty.
func OpenOptionAddHeaders(options []OpenOption, headers map[string]string) {
	for _, option := range options {
		key, value := option.Header()
		if key != "" && value != "" {
			headers[key] = value
		}
	}
}

// OpenOptionHeaders adds each header found in options to the
// headers map provided the key was non empty.
//
// It returns a nil map if options was empty
func OpenOptionHeaders(options []OpenOption) (headers map[string]string) {
	if len(options) == 0 {
		return nil
	}
	headers = make(map[string]string, len(options))
	OpenOptionAddHeaders(options, headers)
	return headers
}

// OpenOptionAddHTTPHeaders Sets each header found in options to the
// http.Header map provided the key was non empty.
func OpenOptionAddHTTPHeaders(headers http.Header, options []OpenOption) {
	for _, option := range options {
		key, value := option.Header()
		if key != "" && value != "" {
			headers.Set(key, value)
		}
	}
}
