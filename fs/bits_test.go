package fs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bits = Bits[bitsChoices]

const (
	bitA bits = 1 << iota
	bitB
	bitC
)

type bitsChoices struct{}

func (bitsChoices) Choices() []BitsChoicesInfo {
	return []BitsChoicesInfo{
		{uint64(0), "OFF"},
		{uint64(bitA), "A"},
		{uint64(bitB), "B"},
		{uint64(bitC), "C"},
	}
}

// Check it satisfies the interfaces
var (
	_ Flagger   = (*bits)(nil)
	_ FlaggerNP = bits(0)
)

func TestBitsString(t *testing.T) {
	assert.Equal(t, "OFF", bits(0).String())
	assert.Equal(t, "A", (bitA).String())
	assert.Equal(t, "A,B", (bitA | bitB).String())
	assert.Equal(t, "A,B,C", (bitA | bitB | bitC).String())
	assert.Equal(t, "A,Unknown-0x8000", (bitA | bits(0x8000)).String())
}

func TestBitsHelp(t *testing.T) {
	assert.Equal(t, "OFF, A, B, C", bits(0).Help())
}

func TestBitsSet(t *testing.T) {
	for _, test := range []struct {
		in      string
		want    bits
		wantErr string
	}{
		{"", bits(0), ""},
		{"B", bitB, ""},
		{"B,A", bitB | bitA, ""},
		{"a,b,C", bitA | bitB | bitC, ""},
		{"A,B,unknown,E", 0, `invalid choice "unknown" from: OFF, A, B, C`},
	} {
		f := bits(0xffffffffffffffff)
		initial := f
		err := f.Set(test.in)
		if err != nil {
			if test.wantErr == "" {
				t.Errorf("Got an error when not expecting one on %q: %v", test.in, err)
			} else {
				assert.Contains(t, err.Error(), test.wantErr)
			}
			assert.Equal(t, initial, f, test.want)
		} else {
			if test.wantErr != "" {
				t.Errorf("Got no error when expecting one on %q", test.in)
			} else {
				assert.Equal(t, test.want, f)
			}
		}

	}
}

func TestBitsIsSet(t *testing.T) {
	b := bitA | bitB
	assert.True(t, b.IsSet(bitA))
	assert.True(t, b.IsSet(bitB))
	assert.True(t, b.IsSet(bitA|bitB))
	assert.False(t, b.IsSet(bitC))
	assert.False(t, b.IsSet(bitA|bitC))
}

func TestBitsType(t *testing.T) {
	f := bits(0)
	assert.Equal(t, "Bits", f.Type())
}

func TestBitsScan(t *testing.T) {
	var v bits
	n, err := fmt.Sscan(" C,B ", &v)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, bitC|bitB, v)
}

func TestBitsUnmarshallJSON(t *testing.T) {
	for _, test := range []struct {
		in      string
		want    bits
		wantErr string
	}{
		{`""`, bits(0), ""},
		{`"B"`, bitB, ""},
		{`"B,A"`, bitB | bitA, ""},
		{`"A,B,C"`, bitA | bitB | bitC, ""},
		{`"A,B,unknown,E"`, 0, `invalid choice "unknown" from: OFF, A, B, C`},
		{`0`, bits(0), ""},
		{strconv.Itoa(int(bitB)), bitB, ""},
		{strconv.Itoa(int(bitB | bitA)), bitB | bitA, ""},
	} {
		f := bits(0xffffffffffffffff)
		initial := f
		err := json.Unmarshal([]byte(test.in), &f)
		if err != nil {
			if test.wantErr == "" {
				t.Errorf("Got an error when not expecting one on %q: %v", test.in, err)
			} else {
				assert.Contains(t, err.Error(), test.wantErr)
			}
			assert.Equal(t, initial, f, test.want)
		} else {
			if test.wantErr != "" {
				t.Errorf("Got no error when expecting one on %q", test.in)
			} else {
				assert.Equal(t, test.want, f)
			}
		}
	}
}
func TestBitsMarshalJSON(t *testing.T) {
	for _, test := range []struct {
		in   bits
		want string
	}{
		{bitA | bitC, `"A,C"`},
		{0, `"OFF"`},
	} {
		got, err := json.Marshal(&test.in)
		require.NoError(t, err)
		assert.Equal(t, test.want, string(got), fmt.Sprintf("%#v", test.in))
	}
}
