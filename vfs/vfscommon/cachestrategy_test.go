package vfscommon

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// Check CacheStrategy it satisfies the pflag interface
var _ pflag.Value = (*CacheStrategy)(nil)

// Check CacheStrategy it satisfies the json.Unmarshaller interface
var _ json.Unmarshaler = (*CacheStrategy)(nil)

func TestCacheStrategyString(t *testing.T) {
	assert.Equal(t, "lru", CacheStrategyLRU.String())
	assert.Equal(t, "lfu", CacheStrategyLFU.String())
	assert.Equal(t, "lff", CacheStrategyLFF.String())
	assert.Equal(t, "lru-sp", CacheStrategyLRUSP.String())
	assert.Equal(t, "Unknown(17)", CacheStrategy(17).String())
}

func TestCacheStrategySet(t *testing.T) {
	var m CacheStrategy

	err := m.Set("lru-sp")
	assert.NoError(t, err)
	assert.Equal(t, CacheStrategyLRUSP, m)

	err = m.Set("potato")
	assert.Error(t, err, "Unknown cache strategy")

	err = m.Set("")
	assert.Error(t, err, "Unknown cache strategy")
}

func TestCacheStrategyType(t *testing.T) {
	var m CacheStrategy
	assert.Equal(t, "CacheStrategy", m.Type())
}

func TestCacheStrategyUnmarshalJSON(t *testing.T) {
	var m CacheStrategy

	err := json.Unmarshal([]byte(`"lru-sp"`), &m)
	assert.NoError(t, err)
	assert.Equal(t, CacheStrategyLRUSP, m)

	err = json.Unmarshal([]byte(`"potato"`), &m)
	assert.Error(t, err, "Unknown cache strategy")

	err = json.Unmarshal([]byte(`""`), &m)
	assert.Error(t, err, "Unknown cache strategy")

	err = json.Unmarshal([]byte(strconv.Itoa(int(CacheStrategyLRUSP))), &m)
	assert.NoError(t, err)
	assert.Equal(t, CacheStrategyLRUSP, m)

	err = json.Unmarshal([]byte("-1"), &m)
	assert.Error(t, err, "Unknown cache strategy")

	err = json.Unmarshal([]byte("99"), &m)
	assert.Error(t, err, "Unknown cache strategy")
}
