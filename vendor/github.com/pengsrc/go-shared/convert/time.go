package convert

import (
	"fmt"
	"time"

	"github.com/pengsrc/go-shared/check"
)

// Supported time layouts
const (
	RFC822       = "Mon, 02 Jan 2006 15:04:05 GMT"
	ISO8601      = "2006-01-02T15:04:05Z"
	ISO8601Milli = "2006-01-02T15:04:05.000Z"
	NGINXTime    = "02/Jan/2006:15:04:05 -0700"
)

// TimeToString transforms given time to string.
func TimeToString(timeValue time.Time, format string) string {
	if check.StringSliceContains([]string{RFC822, ISO8601, ISO8601Milli}, format) {
		timeValue = timeValue.UTC()
	}
	return timeValue.Format(format)
}

// StringToTime transforms given string to time.
func StringToTime(timeString string, format string) (time.Time, error) {
	result, err := time.Parse(format, timeString)
	if timeString != "0001-01-01T00:00:00Z" {
		zero := time.Time{}
		if result == zero {
			err = fmt.Errorf(`failed to parse "%s" like "%s"`, timeString, format)
		}
	}
	return result, err
}

// TimeToTimestamp transforms given time to unix time int.
func TimeToTimestamp(t time.Time) int64 {
	zero := time.Time{}
	if t == zero {
		t = time.Unix(0, 0).UTC()
	}
	return t.Unix()
}

// TimestampToTime transforms given unix time int64 to time in UTC.
func TimestampToTime(unix int64) time.Time {
	return time.Unix(unix, 0).UTC()
}

// TimestampToTimePointer transforms given unix time int64 to time pointer in UTC.
func TimestampToTimePointer(unix int64) *time.Time {
	if unix == 0 {
		return nil
	}
	t := time.Unix(unix, 0).UTC()
	return &t
}

// TimePointerToTimestamp transforms given time pointer to unix time int64.
func TimePointerToTimestamp(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.Unix()
}

// StringToTimestamp transforms given string to unix time int64. It will
// return -1 when time string parse error.
func StringToTimestamp(timeString string, format string) int64 {
	t, err := StringToTime(timeString, format)
	if err != nil {
		return -1
	}
	return t.Unix()
}

// TimestampToString converts unix timestamp to formatted string.
func TimestampToString(unix int64, format string) string {
	return TimeToString(time.Unix(unix, 0).UTC(), format)
}
