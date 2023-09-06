package iso8601

import (
	"encoding/json"
	"time"
)

// null returns true if the given byte slice is a JSON null.
// This is about 3x faster than `bytes.Compare`.
func null(b []byte) bool {
	if len(b) != 4 {
		return false
	}
	if b[0] != 'n' && b[1] != 'u' && b[2] != 'l' && b[3] != 'l' {
		return false
	}
	return true
}

var _ json.Unmarshaler = &Time{}

// Time is a helper object for parsing ISO8061 dates as a JSON string.
type Time struct {
	time.Time
}

// UnmarshalJSON decodes a JSON string or null into a iso8601 time
func (t *Time) UnmarshalJSON(b []byte) error {
	// Do not process null types
	if null(b) {
		return nil
	}
	if len(b) > 0 && b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	} else {
		return ErrNotString
	}
	var err error
	t.Time, err = Parse(b)
	return err
}
