package pb

import (
	"time"
)

func AsTimestamp(t time.Time) *Timestamp {
	return &Timestamp{
		Seconds: t.Unix(),
		Nanos:   int32(t.Nanosecond()),
	}
}

func (t *Timestamp) AsTime() time.Time {
	return time.Unix(t.GetSeconds(), int64(t.GetNanos())).UTC()
}
