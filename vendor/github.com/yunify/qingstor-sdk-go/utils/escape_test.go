package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURLQueryEscape(t *testing.T) {
	assert.Equal(t, "/", URLQueryEscape("/"))
	assert.Equal(t, "%20", URLQueryEscape(" "))
	assert.Equal(t,
		"%21%40%23%24%25%5E%26%2A%28%29_%2B%7B%7D%7C",
		URLQueryEscape("!@#$%^&*()_+{}|"),
	)
}

func TestURLQueryUnescape(t *testing.T) {
	x, err := URLQueryUnescape("/")
	assert.Nil(t, err)
	assert.Equal(t, "/", x)

	x, err = URLQueryUnescape("%20")
	assert.Nil(t, err)
	assert.Equal(t, " ", x)

	x, err = URLQueryUnescape("%21%40%23%24%25%5E%26%2A%28%29_%2B%7B%7D%7C")
	assert.Nil(t, err)
	assert.Equal(t, "!@#$%^&*()_+{}|", x)
}
