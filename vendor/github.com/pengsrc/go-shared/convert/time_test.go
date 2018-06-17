package convert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeToString(t *testing.T) {
	tz, err := time.LoadLocation("Asia/Shanghai")
	assert.NoError(t, err)

	someTime := time.Date(2016, 9, 1, 15, 30, 0, 500000000, tz)
	assert.Equal(t, "Thu, 01 Sep 2016 07:30:00 GMT", TimeToString(someTime, RFC822))
	assert.Equal(t, "2016-09-01T07:30:00Z", TimeToString(someTime, ISO8601))
	assert.Equal(t, "2016-09-01T07:30:00.500Z", TimeToString(someTime, ISO8601Milli))
	assert.Equal(t, "01/Sep/2016:15:30:00 +0800", TimeToString(someTime, NGINXTime))
	assert.Equal(t, "01/Sep/2016:07:30:00 +0000", TimeToString(someTime.UTC(), NGINXTime))
}

func TestStringToTime(t *testing.T) {
	tz, err := time.LoadLocation("Asia/Shanghai")
	assert.NoError(t, err)
	someTime := time.Date(2016, 9, 1, 15, 30, 0, 0, tz)

	parsedTime, err := StringToTime("Thu, 01 Sep 2016 07:30:00 GMT", RFC822)
	assert.NoError(t, err)
	assert.Equal(t, someTime.UTC(), parsedTime)

	parsedTime, err = StringToTime("2016-09-01T07:30:00Z", ISO8601)
	assert.NoError(t, err)
	assert.Equal(t, someTime.UTC(), parsedTime)

	parsedTime, err = StringToTime("1472715000", ISO8601)
	assert.Error(t, err)
	assert.Equal(t, time.Time{}, parsedTime)

	someTime = time.Date(2016, 9, 1, 15, 30, 0, 500000000, tz)
	parsedTime, err = StringToTime("2016-09-01T07:30:00.500Z", ISO8601Milli)
	assert.NoError(t, err)
	assert.Equal(t, someTime.UTC(), parsedTime)
	someTime = time.Date(2016, 9, 1, 15, 30, 0, 0, tz)

	parsedTime, err = StringToTime("01/Sep/2016:15:30:00 +0800", NGINXTime)
	assert.NoError(t, err)
	assert.Equal(t, someTime.UTC(), parsedTime.UTC())

	parsedTime, err = StringToTime("01/Sep/2016:07:30:00 +0000", NGINXTime)
	assert.NoError(t, err)
	assert.Equal(t, someTime.UTC(), parsedTime.UTC())
}

func TestTimeToTimestamp(t *testing.T) {
	tz, err := time.LoadLocation("Asia/Shanghai")
	assert.NoError(t, err)
	someTime := time.Date(2016, 9, 1, 15, 30, 0, 0, tz)

	assert.Equal(t, int64(1472715000), TimeToTimestamp(someTime))
}

func TestTimestampToTime(t *testing.T) {
	tz, err := time.LoadLocation("Asia/Shanghai")
	assert.NoError(t, err)
	someTime := time.Date(2016, 9, 1, 15, 30, 0, 0, tz)

	assert.Equal(t, someTime.UTC(), TimestampToTime(1472715000))
}

func TestTimestampToTimePointer(t *testing.T) {
	tz, err := time.LoadLocation("Asia/Shanghai")
	assert.NoError(t, err)
	someTime := time.Date(2016, 9, 1, 15, 30, 0, 0, tz).UTC()

	assert.Equal(t, &someTime, TimestampToTimePointer(1472715000))
	assert.Nil(t, TimestampToTimePointer(0))
}

func TestTimePointerToTimestamp(t *testing.T) {
	tz, err := time.LoadLocation("Asia/Shanghai")
	assert.NoError(t, err)
	unixTime := int64(1472715000)
	someTime := time.Date(2016, 9, 1, 15, 30, 0, 0, tz)

	assert.Equal(t, unixTime, TimePointerToTimestamp(&someTime))
	assert.Equal(t, int64(0), TimePointerToTimestamp(nil))
}

func TestStringToTimestamp(t *testing.T) {
	assert.Equal(t, int64(1472715000), StringToTimestamp("Thu, 01 Sep 2016 07:30:00 GMT", RFC822))
	assert.Equal(t, int64(-1), StringToTimestamp("2016-09-01T07:30:00.000Z", RFC822))
	assert.Equal(t, int64(1472715000), StringToTimestamp("2016-09-01T07:30:00Z", ISO8601))
	assert.Equal(t, int64(1472715000), StringToTimestamp("2016-09-01T07:30:00.000Z", ISO8601Milli))
	assert.Equal(t, int64(1472715000), StringToTimestamp("2016-09-01T07:30:00.500Z", ISO8601Milli))
	assert.Equal(t, int64(1472715000), StringToTimestamp("01/Sep/2016:15:30:00 +0800", NGINXTime))
	assert.Equal(t, int64(1472715000), StringToTimestamp("01/Sep/2016:07:30:00 +0000", NGINXTime))
}

func TestTimestampToString(t *testing.T) {
	assert.Equal(t, StringToTimestamp("Thu, 01 Sep 2016 07:30:00 GMT", RFC822), int64(1472715000))
	assert.Equal(t, StringToTimestamp("2016-09-01T07:30:00.000Z", RFC822), int64(-1))
	assert.Equal(t, StringToTimestamp("2016-09-01T07:30:00Z", ISO8601), int64(1472715000))
	assert.Equal(t, StringToTimestamp("2016-09-01T07:30:00.000Z", ISO8601Milli), int64(1472715000))
	assert.Equal(t, StringToTimestamp("2016-09-01T07:30:00.500Z", ISO8601Milli), int64(1472715000))
	assert.Equal(t, StringToTimestamp("01/Sep/2016:15:30:00 +0800", NGINXTime), int64(1472715000))
	assert.Equal(t, StringToTimestamp("01/Sep/2016:07:30:00 +0000", NGINXTime), int64(1472715000))
}
