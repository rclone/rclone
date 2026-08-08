package fs

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUseSnapshotModeString(t *testing.T) {
	for _, test := range []struct {
		in   UseSnapshotMode
		want string
	}{
		{UseSnapshotModeNever, "NEVER"},
		{UseSnapshotModeAttempt, "ATTEMPT"},
		{UseSnapshotModeAlways, "ALWAYS"},
		{42, "Unknown(42)"},
	} {
		mode := test.in
		assert.Equal(t, test.want, mode.String(), test.in)
	}
}

func TestUseSnapshotModeSet(t *testing.T) {
	for _, test := range []struct {
		in          string
		want        UseSnapshotMode
		expectError bool
	}{
		{"ALWAYS", UseSnapshotModeAlways, false},
		{"attempt", UseSnapshotModeAttempt, false},
		{"Never", UseSnapshotModeNever, false},
		{"Invalid", 0, true},
	} {
		mode := UseSnapshotMode(0)
		err := mode.Set(test.in)
		if test.expectError {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, mode, test.in)
	}
}

func TestUseSnapshotModeUnmarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in          string
		want        UseSnapshotMode
		expectError bool
	}{
		{`"ALWAYS"`, UseSnapshotModeAlways, false},
		{`"attempt"`, UseSnapshotModeAttempt, false},
		{`"Never"`, UseSnapshotModeNever, false},
		{`"Invalid"`, 0, true},
		{strconv.Itoa(int(UseSnapshotModeAlways)), UseSnapshotModeAlways, false},
		{strconv.Itoa(int(UseSnapshotModeAttempt)), UseSnapshotModeAttempt, false},
		{strconv.Itoa(int(UseSnapshotModeNever)), UseSnapshotModeNever, false},
		{`99`, 0, true},
		{`-99`, 0, true},
	} {
		var mode UseSnapshotMode
		err := json.Unmarshal([]byte(test.in), &mode)
		if test.expectError {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, mode, test.in)
	}
}
