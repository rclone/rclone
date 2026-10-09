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

func TestRedactValue(t *testing.T) {
	ci := &ConfigInfo{}
	assert.Equal(t, `""`, RedactValue(ci, ""))
	assert.Equal(t, "XXX", RedactValue(ci, "potato"))
	ci.Dump = DumpAuth
	assert.Equal(t, `""`, RedactValue(ci, ""))
	assert.Equal(t, `"potato"`, RedactValue(ci, "potato"))
}

func TestRedactOptionValue(t *testing.T) {
	ci := &ConfigInfo{}
	plain := &Option{Name: "chunk_size"}
	password := &Option{Name: "pass", IsPassword: true}
	sensitive := &Option{Name: "token", Sensitive: true}
	assert.Equal(t, `"potato"`, RedactOptionValue(ci, plain, "potato"))
	assert.Equal(t, "XXX", RedactOptionValue(ci, password, "potato"))
	assert.Equal(t, "XXX", RedactOptionValue(ci, sensitive, "potato"))
	assert.Equal(t, "XXX", RedactOptionValue(ci, nil, "potato"))
	assert.Equal(t, `""`, RedactOptionValue(ci, password, ""))
	ci.Dump = DumpAuth
	assert.Equal(t, `"potato"`, RedactOptionValue(ci, password, "potato"))
	assert.Equal(t, `"potato"`, RedactOptionValue(ci, nil, "potato"))
}

func TestRedactConfigOut(t *testing.T) {
	ci := &ConfigInfo{}
	assert.Equal(t, "<nil>", redactConfigOut(ci, nil))
	assert.Equal(t, `{State:"state" Result:""}`, redactConfigOut(ci, &ConfigOut{State: "state"}))
	out := &ConfigOut{
		State:  "state",
		Option: &Option{Name: "pass", IsPassword: true, Value: "obscured"},
		OAuth:  struct{}{},
		Error:  "boom",
		Result: "secret",
	}
	assert.Equal(t, `{State:"state" Option:pass=XXX OAuth:set Error:"boom" Result:XXX}`, redactConfigOut(ci, out))
	ci.Dump = DumpAuth
	assert.Equal(t, `{State:"state" Option:pass="obscured" OAuth:set Error:"boom" Result:"secret"}`, redactConfigOut(ci, out))
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
