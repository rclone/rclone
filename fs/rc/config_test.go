package rc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearOptionBlock() func() {
	oldOptionBlock := fs.OptionsRegistry
	fs.OptionsRegistry = map[string]fs.OptionsInfo{}
	return func() {
		fs.OptionsRegistry = oldOptionBlock
	}
}

var testInfo = fs.Options{{
	Name:    "string",
	Default: "str",
	Help:    "It is a string",
}, {
	Name:    "int",
	Default: 17,
	Help:    "It is an int",
}}

var testOptions = struct {
	String string
	Int    int
}{
	String: "hello",
	Int:    42,
}

func registerTestOptions() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "potato", Opt: &testOptions, Options: testInfo})
}

func registerTestOptionsReload(reload func(context.Context) error) {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "potato", Opt: &testOptions, Options: testInfo, Reload: reload})
}

func TestAddOption(t *testing.T) {
	defer clearOptionBlock()()
	assert.Equal(t, len(fs.OptionsRegistry), 0)
	registerTestOptions()
	assert.Equal(t, len(fs.OptionsRegistry), 1)
	assert.Equal(t, &testOptions, fs.OptionsRegistry["potato"].Opt)
}

func TestAddOptionReload(t *testing.T) {
	defer clearOptionBlock()()
	assert.Equal(t, len(fs.OptionsRegistry), 0)
	reload := func(ctx context.Context) error { return nil }
	registerTestOptionsReload(reload)
	assert.Equal(t, len(fs.OptionsRegistry), 1)
	assert.Equal(t, &testOptions, fs.OptionsRegistry["potato"].Opt)
	assert.Equal(t, fmt.Sprintf("%p", reload), fmt.Sprintf("%p", fs.OptionsRegistry["potato"].Reload))
}

func TestOptionsBlocks(t *testing.T) {
	defer clearOptionBlock()()
	registerTestOptions()
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
	registerTestOptions()
	call := Calls.Get("options/get")
	require.NotNil(t, call)
	in := Params{}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, Params{"potato": &testOptions}, out)
	in = Params{"blocks": "sausage,potato,rhubarb"}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, Params{"potato": &testOptions}, out)
	in = Params{"blocks": "sausage"}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, Params{}, out)
}

func TestOptionsGetMarshal(t *testing.T) {
	defer clearOptionBlock()()
	ctx := context.Background()
	ci := fs.GetConfig(ctx)

	// Add some real options
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "main", Opt: ci, Options: nil})
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "rc", Opt: &Opt, Options: nil})

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

func TestOptionsInfo(t *testing.T) {
	defer clearOptionBlock()()
	registerTestOptions()
	call := Calls.Get("options/info")
	require.NotNil(t, call)
	in := Params{}
	out, err := call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, Params{"potato": testInfo}, out)
	in = Params{"blocks": "sausage,potato,rhubarb"}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, Params{"potato": testInfo}, out)
	in = Params{"blocks": "sausage"}
	out, err = call.Fn(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, Params{}, out)
}

func TestOptionsSet(t *testing.T) {
	defer clearOptionBlock()()
	var reloaded int
	registerTestOptionsReload(func(ctx context.Context) error {
		if reloaded > 1 {
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
	assert.Equal(t, "str", testOptions.String)
	assert.Equal(t, 2, reloaded)

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
