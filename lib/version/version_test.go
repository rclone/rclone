package version_test

import (
	"testing"
	"time"

	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/version"
	"github.com/stretchr/testify/assert"
)

var (
	emptyT time.Time
	t0     = fstest.Time("1970-01-01T01:01:01.123456789Z")
	t0r    = fstest.Time("1970-01-01T01:01:01.123000000Z")
	t1     = fstest.Time("2001-02-03T04:05:06.123000000Z")
)

func TestVersionAdd(t *testing.T) {
	for _, test := range []struct {
		t        time.Time
		in       string
		expected string
	}{
		{t0, "potato.txt", "potato-v1970-01-01-010101-123.txt"},
		{t0, "potato-v2001-02-03-040506-123.txt", "potato-v2001-02-03-040506-123-v1970-01-01-010101-123.txt"},
		{t0, "123.!!lipps", "123-v1970-01-01-010101-123.!!lipps"},
		{t1, "potato", "potato-v2001-02-03-040506-123"},
		{t1, ".potato", ".potato-v2001-02-03-040506-123"},
		{t1, ".potato.conf", ".potato-v2001-02-03-040506-123.conf"},
		{t1, "", "-v2001-02-03-040506-123"},
	} {
		actual := version.Add(test.in, test.t)
		assert.Equal(t, test.expected, actual, test.in)
	}
}

func TestVersionRemove(t *testing.T) {
	for _, test := range []struct {
		in             string
		expectedT      time.Time
		expectedRemote string
	}{
		{"potato.txt", emptyT, "potato.txt"},
		{"potato-v1970-01-01-010101-123.txt", t0r, "potato.txt"},
		{"potato-v2001-02-03-040506-123-v1970-01-01-010101-123.txt", t0r, "potato-v2001-02-03-040506-123.txt"},
		{"potato-v2001-02-03-040506-123", t1, "potato"},
		{".potato-v2001-02-03-040506-123", t1, ".potato"},
		{".potato-v2001-02-03-040506-123.conf", t1, ".potato.conf"},
		{"-v2001-02-03-040506-123", t1, ""},
		{"potato-v2A01-02-03-040506-123", emptyT, "potato-v2A01-02-03-040506-123"},
		{"potato-v2001-02-03-040506=123", emptyT, "potato-v2001-02-03-040506=123"},
	} {
		actualT, actualRemote := version.Remove(test.in)
		assert.Equal(t, test.expectedT, actualT, test.in)
		assert.Equal(t, test.expectedRemote, actualRemote, test.in)
	}
}

func TestVersionMatch(t *testing.T) {
	for _, test := range []struct {
		in       string
		expected bool
	}{
		{"potato.txt", false},
		{"potato", false},
		{"", false},
		{"potato-v1970-01-01-010101-123.txt", true},
		{"potato-v2001-02-03-040506-123-v1970-01-01-010101-123.txt", true},
		{"potato-v2001-02-03-040506-123", true},
		{"-v2001-02-03-040506-123", true},
		{"-v9999-99-99-999999-999", true},
	} {
		actual := version.Match(test.in)
		assert.Equal(t, test.expected, actual, test.in)
	}
}
