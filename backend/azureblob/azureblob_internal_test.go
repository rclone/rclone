// +build !plan9,!solaris,go1.8

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
