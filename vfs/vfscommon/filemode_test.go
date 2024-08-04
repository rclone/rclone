package vfscommon

import (
	"encoding/json"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interfaces
var (
	_ fs.Flagger   = (*FileMode)(nil)
	_ fs.FlaggerNP = FileMode(0)
)

func TestFileModeString(t *testing.T) {
	for _, test := range []struct {
		in   FileMode
		want string
	}{
		{0, "000"},
		{0666, "666"},
		{02666, "2666"},
	} {
		got := test.in.String()
		assert.Equal(t, test.want, got)
	}
}

func TestFileModeSet(t *testing.T) {
	for _, test := range []struct {
		in   string
		want FileMode
		err  bool
	}{
		{"0", 0, false},
		{"0666", 0666, false},
		{"666", 0666, false},
		{"2666", 02666, false},
		{"999", 0, true},
	} {
		got := FileMode(0)
		err := got.Set(test.in)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, got)
	}
}

func TestFileModeUnmarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   string
		want FileMode
		err  bool
	}{
		{`"0"`, 0, false},
		{`"666"`, 0666, false},
		{`"02666"`, 02666, false},
		{`"999"`, 0, true},
		{`438`, 0666, false},
		{`"999"`, 0, true},
	} {
		var ss FileMode
		err := json.Unmarshal([]byte(test.in), &ss)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, ss, test.in)
	}
}
