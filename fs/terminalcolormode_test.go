package fs

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerminalColorModeString(t *testing.T) {
	for _, test := range []struct {
		in   TerminalColorMode
		want string
	}{
		{TerminalColorModeAuto, "AUTO"},
		{TerminalColorModeAlways, "ALWAYS"},
		{TerminalColorModeNever, "NEVER"},
		{36, "Unknown(36)"},
	} {
		tcm := test.in
		assert.Equal(t, test.want, tcm.String(), test.in)
	}
}

func TestTerminalColorModeSet(t *testing.T) {
	for _, test := range []struct {
		in          string
		want        TerminalColorMode
		expectError bool
	}{
		{"auto", TerminalColorModeAuto, false},
		{"ALWAYS", TerminalColorModeAlways, false},
		{"Never", TerminalColorModeNever, false},
		{"INVALID", 0, true},
	} {
		tcm := TerminalColorMode(0)
		err := tcm.Set(test.in)
		if test.expectError {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, tcm, test.in)
	}
}

func TestTerminalColorModeUnmarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in          string
		want        TerminalColorMode
		expectError bool
	}{
		{`"auto"`, TerminalColorModeAuto, false},
		{`"ALWAYS"`, TerminalColorModeAlways, false},
		{`"Never"`, TerminalColorModeNever, false},
		{`"Invalid"`, 0, true},
		{strconv.Itoa(int(TerminalColorModeAuto)), TerminalColorModeAuto, false},
		{strconv.Itoa(int(TerminalColorModeAlways)), TerminalColorModeAlways, false},
		{strconv.Itoa(int(TerminalColorModeNever)), TerminalColorModeNever, false},
		{`99`, 0, true},
		{`-99`, 0, true},
	} {
		var tcm TerminalColorMode
		err := json.Unmarshal([]byte(test.in), &tcm)
		if test.expectError {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, tcm, test.in)
	}
}
