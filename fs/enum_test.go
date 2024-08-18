package fs

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type choices struct{}

func (choices) Choices() []string {
	return []string{
		choiceA: "A",
		choiceB: "B",
		choiceC: "C",
	}
}

type choice = Enum[choices]

const (
	choiceA choice = iota
	choiceB
	choiceC
)

// Check it satisfies the interfaces
var (
	_ Flagger   = (*choice)(nil)
	_ FlaggerNP = choice(0)
)

func TestEnumString(t *testing.T) {
	for _, test := range []struct {
		in   choice
		want string
	}{
		{choiceA, "A"},
		{choiceB, "B"},
		{choiceC, "C"},
		{choice(100), "Unknown(100)"},
	} {
		got := test.in.String()
		assert.Equal(t, test.want, got)
	}
}

func TestEnumType(t *testing.T) {
	assert.Equal(t, "A|B|C", choiceA.Type())
}

// Enum with Type() on the choices
type choicestype struct{}

func (choicestype) Choices() []string {
	return []string{}
}

func (choicestype) Type() string {
	return "potato"
}

type choicetype = Enum[choicestype]

func TestEnumTypeWithFunction(t *testing.T) {
	assert.Equal(t, "potato", choicetype(0).Type())
}

func TestEnumHelp(t *testing.T) {
	assert.Equal(t, "A, B, C", choice(0).Help())
}

func TestEnumSet(t *testing.T) {
	for _, test := range []struct {
		in   string
		want choice
		err  bool
	}{
		{"A", choiceA, false},
		{"B", choiceB, false},
		{"C", choiceC, false},
		{"D", choice(100), true},
	} {
		var got choice
		err := got.Set(test.in)
		if test.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		}
	}
}

func TestEnumScan(t *testing.T) {
	var v choice
	n, err := fmt.Sscan(" A ", &v)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, choiceA, v)
}

func TestEnumUnmarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   string
		want choice
		err  string
	}{
		{`"A"`, choiceA, ""},
		{`"B"`, choiceB, ""},
		{`0`, choiceA, ""},
		{`1`, choiceB, ""},
		{`"D"`, choice(0), `invalid choice "D" from: A, B, C`},
		{`100`, choice(0), `100 is out of range: must be 0..3`},
	} {
		var got choice
		err := json.Unmarshal([]byte(test.in), &got)
		if test.err != "" {
			require.Error(t, err, test.in)
			assert.ErrorContains(t, err, test.err)
		} else {
			require.NoError(t, err, test.in)
		}
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestEnumMarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   choice
		want string
	}{
		{choiceA, `"A"`},
		{choiceB, `"B"`},
	} {
		got, err := json.Marshal(&test.in)
		require.NoError(t, err)
		assert.Equal(t, test.want, string(got), fmt.Sprintf("%#v", test.in))
	}
}
