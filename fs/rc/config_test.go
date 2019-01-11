package rc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearOptionBlock() {
	optionBlock = map[string]interface{}{}
}

var testOptions = struct {
	String string
	Int    int
}{
	String: "hello",
	Int:    42,
}

func TestAddOption(t *testing.T) {
	defer clearOptionBlock()
	assert.Equal(t, len(optionBlock), 0)
	AddOption("potato", &testOptions)
	assert.Equal(t, len(optionBlock), 1)
	assert.Equal(t, &testOptions, optionBlock["potato"])
}

func TestOptionsBlocks(t *testing.T) {
	defer clearOptionBlock()
	AddOption("potato", &testOptions)
	call := Calls.Get("options/blocks")
	require.NotNil(t, call)
	in := Params{}
	out, err := call.Fn(in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, Params{"options": []string{"potato"}}, out)
}

func TestOptionsGet(t *testing.T) {
	defer clearOptionBlock()
	AddOption("potato", &testOptions)
	call := Calls.Get("options/get")
	require.NotNil(t, call)
	in := Params{}
	out, err := call.Fn(in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, Params{"potato": &testOptions}, out)
}

func TestOptionsSet(t *testing.T) {
	defer clearOptionBlock()
	AddOption("potato", &testOptions)
	call := Calls.Get("options/set")
	require.NotNil(t, call)

	in := Params{
		"potato": Params{
			"Int": 50,
		},
	}
	out, err := call.Fn(in)
	require.NoError(t, err)
	require.Nil(t, out)
	assert.Equal(t, 50, testOptions.Int)
	assert.Equal(t, "hello", testOptions.String)

	// unknown option block
	in = Params{
		"sausage": Params{
			"Int": 50,
		},
	}
	_, err = call.Fn(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown option block")

	// bad shape
	in = Params{
		"potato": []string{"a", "b"},
	}
	_, err = call.Fn(in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write options")
}
