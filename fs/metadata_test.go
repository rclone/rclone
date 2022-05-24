package fs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetadataSet(t *testing.T) {
	var m Metadata
	assert.Nil(t, m)
	m.Set("key", "value")
	assert.NotNil(t, m)
	assert.Equal(t, "value", m["key"])
	m.Set("key", "value2")
	assert.Equal(t, "value2", m["key"])
}
