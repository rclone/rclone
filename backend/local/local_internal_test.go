package local

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapper(t *testing.T) {
	m := newMapper()
	assert.Equal(t, m.m, map[string]string{})
	assert.Equal(t, "potato", m.Save("potato", "potato"))
	assert.Equal(t, m.m, map[string]string{})
	assert.Equal(t, "-r'áö", m.Save("-r?'a´o¨", "-r'áö"))
	assert.Equal(t, m.m, map[string]string{
		"-r'áö": "-r?'a´o¨",
	})
	assert.Equal(t, "potato", m.Load("potato"))
	assert.Equal(t, "-r?'a´o¨", m.Load("-r'áö"))
}
