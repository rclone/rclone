package vfscommon

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// Check CacheMode it satisfies the pflag interface
var _ pflag.Value = (*CacheMode)(nil)

// Check CacheMode it satisfies the json.Unmarshaller interface
var _ json.Unmarshaler = (*CacheMode)(nil)

func TestCacheModeString(t *testing.T) {
	assert.Equal(t, "off", CacheModeOff.String())
	assert.Equal(t, "full", CacheModeFull.String())
	assert.Equal(t, "Unknown(17)", CacheMode(17).String())
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

func TestCacheModeUnmarshalJSON(t *testing.T) {
	var m CacheMode

	err := json.Unmarshal([]byte(`"full"`), &m)
	assert.NoError(t, err)
	assert.Equal(t, CacheModeFull, m)

	err = json.Unmarshal([]byte(`"potato"`), &m)
	assert.Error(t, err, "Unknown cache mode level")

	err = json.Unmarshal([]byte(`""`), &m)
	assert.Error(t, err, "Unknown cache mode level")

	err = json.Unmarshal([]byte(strconv.Itoa(int(CacheModeFull))), &m)
	assert.NoError(t, err)
	assert.Equal(t, CacheModeFull, m)

	err = json.Unmarshal([]byte("-1"), &m)
	assert.Error(t, err, "Unknown cache mode level")

	err = json.Unmarshal([]byte("99"), &m)
	assert.Error(t, err, "Unknown cache mode level")
}
