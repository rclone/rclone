package vfscommon

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// Check CacheMode it satisfies the pflag interface
var _ pflag.Value = (*CacheMode)(nil)

func TestCacheModeString(t *testing.T) {
	assert.Equal(t, "off", CacheModeOff.String())
	assert.Equal(t, "full", CacheModeFull.String())
	assert.Equal(t, "CacheMode(17)", CacheMode(17).String())
}

func TestCacheModeSet(t *testing.T) {
	var m CacheMode

	err := m.Set("full")
	assert.NoError(t, err)
	assert.Equal(t, CacheModeFull, m)

	err = m.Set("potato")
	assert.Error(t, err, "Unknown cache mode level")

	err = m.Set("")
	assert.Error(t, err, "Unknown cache mode level")
}

func TestCacheModeType(t *testing.T) {
	var m CacheMode
	assert.Equal(t, "CacheMode", m.Type())
}
