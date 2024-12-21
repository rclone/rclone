//go:build !plan9 && !solaris && !js

package azureblob

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (f *Fs) InternalTest(t *testing.T) {
	// Check first feature flags are set on this
	// remote
	enabled := f.Features().SetTier
	assert.True(t, enabled)
	enabled = f.Features().GetTier
	assert.True(t, enabled)
}

func TestBlockIDCreator(t *testing.T) {
	// Check creation and random number
	bic, err := newBlockIDCreator()
	require.NoError(t, err)
	bic2, err := newBlockIDCreator()
	require.NoError(t, err)
	assert.NotEqual(t, bic.random, bic2.random)
	assert.NotEqual(t, bic.random, [8]byte{})

	// Set random to known value for tests
	bic.random = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	chunkNumber := uint64(0xFEDCBA9876543210)

	// Check creation of ID
	want := base64.StdEncoding.EncodeToString([]byte{0xFE, 0xDC, 0xBA, 0x98, 0x76, 0x54, 0x32, 0x10, 1, 2, 3, 4, 5, 6, 7, 8})
	assert.Equal(t, "/ty6mHZUMhABAgMEBQYHCA==", want)
	got := bic.newBlockID(chunkNumber)
	assert.Equal(t, want, got)
	assert.Equal(t, "/ty6mHZUMhABAgMEBQYHCA==", got)

	// Test checkID is working
	assert.NoError(t, bic.checkID(chunkNumber, got))
	assert.ErrorContains(t, bic.checkID(chunkNumber, "$"+got), "illegal base64")
	assert.ErrorContains(t, bic.checkID(chunkNumber, "AAAA"+got), "bad block ID length")
	assert.ErrorContains(t, bic.checkID(chunkNumber+1, got), "expecting decoded")
	assert.ErrorContains(t, bic2.checkID(chunkNumber, got), "random bytes")
}
