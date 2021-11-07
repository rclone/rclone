package rc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/rclone/rclone/cmd/serve/httplib"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearOptionBlock() func() {
	oldOptionBlock := optionBlock
	optionBlock = map[string]interface{}{}
	return func() {
		optionBlock = oldOptionBlock
	}
}

var testOptions = struct {
	String string
	Int    int
}{
	String: "hello",
	Int:    42,
}

func TestAddOption(t *testing.T) {
	defer clearOptionBlock()()
	assert.Equal(t, len(optionBlock), 0)
	AddOption("potato", &testOptions)
	assert.Equal(t, len(optionBlock), 1)
	assert.Equal(t, len(optionReload), 0)
	assert.Equal(t, &testOptions, optionBlock["potato"])
}

func TestAddOptionReload(t *testing.T) {
	defer clearOptionBlock()()
	assert.Equal(t, len(optionBlock), 0)
	reload := func(ctx context.Context) error { return nil }
	AddOptionReload("potato", &testOptions, reload)
	assert.Equal(t, len(optionBlock), 1)
	assert.Equal(t, len(optionReload), 1)
	assert.Equal(t, &testOptions, optionBlock["potato"])
	assert.Equal(t, fmt.Sprintf("%p", reload), fmt.Sprintf("%p", optionReload["potato"]))
}

func TestOptionsBlocks(t *testing.T) {
	defer clearOptionBlock()()
	AddOption("potato", &testOptions)
	call := Calls.Get("options/blocks")
	require.NotNil(t, call)
	in := Params{}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, Params{"options": []string{"potato"}}, out)
}

func TestOptionsGet(t *testing.T) {
	defer clearOptionBlock()()
	AddOption("potato", &testOptions)
	call := Calls.Get("options/get")
	require.NotNil(t, call)
	in := Params{}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, Params{"potato": &testOptions}, out)
}

func TestOptionsGetMarshal(t *testing.T) {
	defer clearOptionBlock()()
	ctx := context.Background()
	ci := fs.GetConfig(ctx)

	// Add some real options
	AddOption("http", &httplib.DefaultOpt)
	AddOption("main", ci)
	AddOption("rc", &DefaultOpt)

	// get them
	call := Calls.Get("options/get")
	require.NotNil(t, call)
	in := Params{}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)

	// Check that they marshal
	_, err = json.Marshal(out)
	require.NoError(t, err)
}

func TestOptionsSet(t *testing.T) {
	defer clearOptionBlock()()
	var reloaded int
	AddOptionReload("potato", &testOptions, func(ctx context.Context) error {
		if reloaded > 0 {
			return errors.New("error while reloading")
		}
		reloaded++
		return nil
	})
	call := Calls.Get("options/set")
	require.NotNil(t, call)

	in := Params{
		"potato": Params{
			"Int": 50,
		},
	}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.Nil(t, out)
	assert.Equal(t, 50, testOptions.Int)
	assert.Equal(t, "hello", testOptions.String)
	assert.Equal(t, 1, reloaded)

	// error from reload
	_, err = call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error while reloading")

	// unknown option block
	in = Params{
		"sausage": Params{
			"Int": 50,
		},
	}
	_, err = call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown option block")

	// bad shape
	in = Params{
		"potato": []string{"a", "b"},
	}
	_, err = call.Fn(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write options")

}
