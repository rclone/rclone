package fs

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interfaces
var (
	_ Flagger   = (*Tristate)(nil)
	_ FlaggerNP = Tristate{}
)

func TestTristateString(t *testing.T) {
	for _, test := range []struct {
		in   Tristate
		want string
	}{
		{Tristate{}, "unset"},
		{Tristate{Valid: false, Value: false}, "unset"},
		{Tristate{Valid: false, Value: true}, "unset"},
		{Tristate{Valid: true, Value: false}, "false"},
		{Tristate{Valid: true, Value: true}, "true"},
	} {
		got := test.in.String()
		assert.Equal(t, test.want, got)
	}
}

func TestTristateSet(t *testing.T) {
	for _, test := range []struct {
		in   string
		want Tristate
		err  bool
	}{
		{"", Tristate{Valid: false, Value: false}, false},
		{"nil", Tristate{Valid: false, Value: false}, false},
		{"null", Tristate{Valid: false, Value: false}, false},
		{"UNSET", Tristate{Valid: false, Value: false}, false},
		{"true", Tristate{Valid: true, Value: true}, false},
		{"1", Tristate{Valid: true, Value: true}, false},
		{"false", Tristate{Valid: true, Value: false}, false},
		{"0", Tristate{Valid: true, Value: false}, false},
		{"potato", Tristate{Valid: false, Value: false}, true},
	} {
		var got Tristate
		err := got.Set(test.in)
		if test.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		}
	}
}

func TestTristateScan(t *testing.T) {
	var v Tristate
	n, err := fmt.Sscan(" true ", &v)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, Tristate{Valid: true, Value: true}, v)
}

func TestTristateUnmarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   string
		want Tristate
		err  bool
	}{
		{`null`, Tristate{}, false},
		{`true`, Tristate{Valid: true, Value: true}, false},
		{`false`, Tristate{Valid: true, Value: false}, false},
		{`potato`, Tristate{}, true},
		{``, Tristate{}, true},
	} {
		var got Tristate
		err := json.Unmarshal([]byte(test.in), &got)
		if test.err {
			require.Error(t, err, test.in)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestTristateMarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   Tristate
		want string
	}{
		{Tristate{}, `null`},
		{Tristate{Valid: true, Value: true}, `true`},
		{Tristate{Valid: true, Value: false}, `false`},
	} {
		got, err := json.Marshal(&test.in)
		require.NoError(t, err)
		assert.Equal(t, test.want, string(got), fmt.Sprintf("%#v", test.in))
	}
}
