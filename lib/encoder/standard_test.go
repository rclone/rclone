package encoder

import (
	"strings"
	"testing"

	"github.com/Max-Sum/base32768"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStandardBase32768(t *testing.T) {
	const name = "擡纮䟀䫏鸔␁皇慃號"
	data, err := base32768.SafeEncoding.DecodeString(name)
	require.NoError(t, err)

	stored := Display.FromStandardName(name)
	assert.Equal(t, name, stored)
	assert.Equal(t, name, Display.ToStandardName(stored))

	listed := Base.ToStandardName(stored)
	assert.Equal(t, strings.Replace(name, "␁", "‛␁", 1), listed)
	_, err = base32768.SafeEncoding.DecodeString(listed)
	assert.Error(t, err)

	stored = Base.FromStandardName(name)
	assert.Equal(t, strings.Replace(name, "␁", "\x01", 1), stored)
	assert.NotEqual(t, name, Display.ToStandardName(stored))
	decoded, err := base32768.SafeEncoding.DecodeString(Base.ToStandardName(stored))
	require.NoError(t, err)
	assert.Equal(t, data, decoded)
}

func TestControlPictureQuoting(t *testing.T) {
	for r := rune(0); r <= 0x1F; r++ {
		t.Run(string(symbolOffset+r), func(t *testing.T) {
			control := string(r)
			picture := string(symbolOffset + r)
			quoted := string(QuoteRune) + picture
			assert.Equal(t, picture, EncodeCtl.Encode(control))
			assert.Equal(t, quoted, EncodeCtl.Encode(picture))
			assert.Equal(t, control, EncodeCtl.Decode(picture))
			assert.Equal(t, picture, EncodeCtl.Decode(quoted))
		})
	}
}
