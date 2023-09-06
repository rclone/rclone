package putio

import "time"

// Time is a wrapper around time.Time that can be unmarshalled from a JSON
// string formatted as "2016-04-19T15:44:42". All methods of time.Time can be
// called on Time.
type Time struct {
	time.Time
}

func (t *Time) String() string {
	return t.Time.String()
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (t *Time) UnmarshalJSON(data []byte) error {
	// put.io API has inconsistent time layouts for different endpoints, such
	// as /files and /events
	var timeLayouts = []string{`"2006-01-02T15:04:05"`, `"2006-01-02 15:04:05"`}

	s := string(data)
	var err error
	var tm time.Time
	for _, layout := range timeLayouts {
		tm, err = time.ParseInLocation(layout, s, time.UTC)
		if err == nil {
			t.Time = tm
			return nil
		}
	}
	return err
}
