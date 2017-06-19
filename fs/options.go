// Define the options for Open

package fs

import (
	"fmt"
	"net/http"
	"strconv"
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

// String formats the option into human readable form
func (o *RangeOption) String() string {
	return fmt.Sprintf("RangeOption(%d,%d)", o.Start, o.End)
}

// Mandatory returns whether the option must be parsed or can be ignored
func (o *RangeOption) Mandatory() bool {
	return false
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

// String formats the option into human readable form
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

// String formats the option into human readable form
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
	Hashes HashSet
}

// Header formats the option as an http header
func (o *HashesOption) Header() (key string, value string) {
	return "", ""
}

// String formats the option into human readable form
func (o *HashesOption) String() string {
	return fmt.Sprintf("HashesOption(%v)", o.Hashes)
}

// Mandatory returns whether the option must be parsed or can be ignored
func (o *HashesOption) Mandatory() bool {
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

// check interface
var (
	_ OpenOption = (*RangeOption)(nil)
	_ OpenOption = (*SeekOption)(nil)
	_ OpenOption = (*HTTPOption)(nil)
)
