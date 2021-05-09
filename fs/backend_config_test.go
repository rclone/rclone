package fs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatePush(t *testing.T) {
	assert.Equal(t, "", StatePush(""))
	assert.Equal(t, "", StatePush("", ""))
	assert.Equal(t, "a", StatePush("", "a"))
	assert.Equal(t, "a,1，2，3", StatePush("", "a", "1,2,3"))

	assert.Equal(t, "potato", StatePush("potato"))
	assert.Equal(t, ",potato", StatePush("potato", ""))
	assert.Equal(t, "a,potato", StatePush("potato", "a"))
	assert.Equal(t, "a,1，2，3,potato", StatePush("potato", "a", "1,2,3"))
}

func TestStatePop(t *testing.T) {
	state, value := StatePop("")
	assert.Equal(t, "", value)
	assert.Equal(t, "", state)

	state, value = StatePop("a")
	assert.Equal(t, "a", value)
	assert.Equal(t, "", state)

	state, value = StatePop("a,1，2，3")
	assert.Equal(t, "a", value)
	assert.Equal(t, "1，2，3", state)

	state, value = StatePop("1，2，3,a")
	assert.Equal(t, "1,2,3", value)
	assert.Equal(t, "a", state)
}

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
		got := MatchProvider(test.config, test.provider)
		assert.Equal(t, test.want, got, what)
	}
}
