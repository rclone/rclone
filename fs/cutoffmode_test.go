package fs

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interfaces
var (
	_ Flagger   = (*CutoffMode)(nil)
	_ FlaggerNP = CutoffMode(0)
)

func TestCutoffModeString(t *testing.T) {
	for _, test := range []struct {
		in   CutoffMode
		want string
	}{
		{CutoffModeHard, "HARD"},
		{CutoffModeSoft, "SOFT"},
		{99, "Unknown(99)"},
	} {
		cm := test.in
		got := cm.String()
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestCutoffModeSet(t *testing.T) {
	for _, test := range []struct {
		in   string
		want CutoffMode
		err  bool
	}{
		{"hard", CutoffModeHard, false},
		{"SOFT", CutoffModeSoft, false},
		{"Cautious", CutoffModeCautious, false},
		{"Potato", 0, true},
	} {
		cm := CutoffMode(0)
		err := cm.Set(test.in)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, cm, test.in)
	}
}

func TestCutoffModeUnmarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   string
		want CutoffMode
		err  bool
	}{
		{`"hard"`, CutoffModeHard, false},
		{`"SOFT"`, CutoffModeSoft, false},
		{`"Cautious"`, CutoffModeCautious, false},
		{`"Potato"`, 0, true},
		{strconv.Itoa(int(CutoffModeHard)), CutoffModeHard, false},
		{strconv.Itoa(int(CutoffModeSoft)), CutoffModeSoft, false},
		{`99`, 0, true},
		{`-99`, 0, true},
	} {
		var cm CutoffMode
		err := json.Unmarshal([]byte(test.in), &cm)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, cm, test.in)
	}
}
