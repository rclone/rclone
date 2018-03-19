package pacer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenDispenser(t *testing.T) {
	td := NewTokenDispenser(5)
	assert.Equal(t, 5, len(td.tokens))
	td.Get()
	assert.Equal(t, 4, len(td.tokens))
	td.Put()
	assert.Equal(t, 5, len(td.tokens))
}
