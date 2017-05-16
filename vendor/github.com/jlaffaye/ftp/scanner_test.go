package ftp

import "testing"
import "github.com/stretchr/testify/assert"

func TestScanner(t *testing.T) {
	assert := assert.New(t)

	s := newScanner("foo  bar x  y")
	assert.Equal("foo", s.Next())
	assert.Equal(" bar x  y", s.Remaining())
	assert.Equal("bar", s.Next())
	assert.Equal("x  y", s.Remaining())
	assert.Equal("x", s.Next())
	assert.Equal(" y", s.Remaining())
	assert.Equal("y", s.Next())
	assert.Equal("", s.Next())
	assert.Equal("", s.Remaining())
}

func TestScannerEmpty(t *testing.T) {
	assert := assert.New(t)

	s := newScanner("")
	assert.Equal("", s.Next())
	assert.Equal("", s.Next())
	assert.Equal("", s.Remaining())
}
