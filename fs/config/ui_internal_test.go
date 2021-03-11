package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchProvider(t *testing.T) {
	for _, test := range []struct {
		config   string
		provider string
		want     bool
	}{
		{"", "", true},
		{"one", "one", true},
		{"one,two", "two", true},
		{"one,two,three", "two", true},
		{"one", "on", false},
		{"one,two,three", "tw", false},
		{"!one,two,three", "two", false},
		{"!one,two,three", "four", true},
	} {
		what := fmt.Sprintf("%q,%q", test.config, test.provider)
		got := matchProvider(test.config, test.provider)
		assert.Equal(t, test.want, got, what)
	}
}
