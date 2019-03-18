// Metadata manipulation in and out of Headers

package swift

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Metadata stores account, container or object metadata.
type Metadata map[string]string

// Metadata gets the Metadata starting with the metaPrefix out of the Headers.
//
// The keys in the Metadata will be converted to lower case
func (h Headers) Metadata(metaPrefix string) Metadata {
	m := Metadata{}
	metaPrefix = http.CanonicalHeaderKey(metaPrefix)
	for key, value := range h {
		if strings.HasPrefix(key, metaPrefix) {
			metaKey := strings.ToLower(key[len(metaPrefix):])
			m[metaKey] = value
		}
	}
	return m
}

// AccountMetadata converts Headers from account to a Metadata.
//
// The keys in the Metadata will be converted to lower case.
func (h Headers) AccountMetadata() Metadata {
	return h.Metadata("X-Account-Meta-")
}

// ContainerMetadata converts Headers from container to a Metadata.
//
// The keys in the Metadata will be converted to lower case.
func (h Headers) ContainerMetadata() Metadata {
	return h.Metadata("X-Container-Meta-")
}

// ObjectMetadata converts Headers from object to a Metadata.
//
// The keys in the Metadata will be converted to lower case.
func (h Headers) ObjectMetadata() Metadata {
	return h.Metadata("X-Object-Meta-")
}

// Headers convert the Metadata starting with the metaPrefix into a
// Headers.
//
// The keys in the Metadata will be converted from lower case to http
// Canonical (see http.CanonicalHeaderKey).
func (m Metadata) Headers(metaPrefix string) Headers {
	h := Headers{}
	for key, value := range m {
		key = http.CanonicalHeaderKey(metaPrefix + key)
		h[key] = value
	}
	return h
}

// AccountHeaders converts the Metadata for the account.
func (m Metadata) AccountHeaders() Headers {
	return m.Headers("X-Account-Meta-")
}

// ContainerHeaders converts the Metadata for the container.
func (m Metadata) ContainerHeaders() Headers {
	return m.Headers("X-Container-Meta-")
}

// ObjectHeaders converts the Metadata for the object.
func (m Metadata) ObjectHeaders() Headers {
	return m.Headers("X-Object-Meta-")
}

// Turns a number of ns into a floating point string in seconds
//
// Trims trailing zeros and guaranteed to be perfectly accurate
func nsToFloatString(ns int64) string {
	if ns < 0 {
		return "-" + nsToFloatString(-ns)
	}
	result := fmt.Sprintf("%010d", ns)
	split := len(result) - 9
	result, decimals := result[:split], result[split:]
	decimals = strings.TrimRight(decimals, "0")
	if decimals != "" {
		result += "."
		result += decimals
	}
	return result
}

// Turns a floating point string in seconds into a ns integer
//
// Guaranteed to be perfectly accurate
func floatStringToNs(s string) (int64, error) {
	const zeros = "000000000"
	if point := strings.IndexRune(s, '.'); point >= 0 {
		tail := s[point+1:]
		if fill := 9 - len(tail); fill < 0 {
			tail = tail[:9]
		} else {
			tail += zeros[:fill]
		}
		s = s[:point] + tail
	} else if len(s) > 0 { // Make sure empty string produces an error
		s += zeros
	}
	return strconv.ParseInt(s, 10, 64)
}

// FloatStringToTime converts a floating point number string to a time.Time
//
// The string is floating point number of seconds since the epoch
// (Unix time).  The number should be in fixed point format (not
// exponential), eg "1354040105.123456789" which represents the time
// "2012-11-27T18:15:05.123456789Z"
//
// Some care is taken to preserve all the accuracy in the time.Time
// (which wouldn't happen with a naive conversion through float64) so
// a round trip conversion won't change the data.
//
// If an error is returned then time will be returned as the zero time.
func FloatStringToTime(s string) (t time.Time, err error) {
	ns, err := floatStringToNs(s)
	if err != nil {
		return
	}
	t = time.Unix(0, ns)
	return
}

// TimeToFloatString converts a time.Time object to a floating point string
//
// The string is floating point number of seconds since the epoch
// (Unix time).  The number is in fixed point format (not
// exponential), eg "1354040105.123456789" which represents the time
// "2012-11-27T18:15:05.123456789Z".  Trailing zeros will be dropped
// from the output.
//
// Some care is taken to preserve all the accuracy in the time.Time
// (which wouldn't happen with a naive conversion through float64) so
// a round trip conversion won't change the data.
func TimeToFloatString(t time.Time) string {
	return nsToFloatString(t.UnixNano())
}

// GetModTime reads a modification time (mtime) from a Metadata object
//
// This is a defacto standard (used in the official python-swiftclient
// amongst others) for storing the modification time (as read using
// os.Stat) for an object.  It is stored using the key 'mtime', which
// for example when written to an object will be 'X-Object-Meta-Mtime'.
//
// If an error is returned then time will be returned as the zero time.
func (m Metadata) GetModTime() (t time.Time, err error) {
	return FloatStringToTime(m["mtime"])
}

// SetModTime writes an modification time (mtime) to a Metadata object
//
// This is a defacto standard (used in the official python-swiftclient
// amongst others) for storing the modification time (as read using
// os.Stat) for an object.  It is stored using the key 'mtime', which
// for example when written to an object will be 'X-Object-Meta-Mtime'.
func (m Metadata) SetModTime(t time.Time) {
	m["mtime"] = TimeToFloatString(t)
}
