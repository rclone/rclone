// +build !plan9,!solaris,!js,go1.13

package azureblob

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func (f *Fs) InternalTest(t *testing.T) {
	// Check first feature flags are set on this
	// remote
	enabled := f.Features().SetTier
	assert.True(t, enabled)
	enabled = f.Features().GetTier
	assert.True(t, enabled)
}

func TestIncrement(t *testing.T) {
	for _, test := range []struct {
		in   []byte
		want []byte
	}{
		{[]byte{0, 0, 0, 0}, []byte{1, 0, 0, 0}},
		{[]byte{0xFE, 0, 0, 0}, []byte{0xFF, 0, 0, 0}},
		{[]byte{0xFF, 0, 0, 0}, []byte{0, 1, 0, 0}},
		{[]byte{0, 1, 0, 0}, []byte{1, 1, 0, 0}},
		{[]byte{0xFF, 0xFF, 0xFF, 0xFE}, []byte{0, 0, 0, 0xFF}},
		{[]byte{0xFF, 0xFF, 0xFF, 0xFF}, []byte{0, 0, 0, 0}},
	} {
		increment(test.in)
		assert.Equal(t, test.want, test.in)
	}
}
