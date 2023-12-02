// These are in an external package because we need to import configfile
//
// Internal tests are in ui_internal_test.go

package config_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var simpleOptions = []fs.Option{{
	Name:       "bool",
	Default:    false,
	IsPassword: false,
}, {
	Name:       "pass",
	Default:    "",
	IsPassword: true,
}}

func testConfigFile(t *testing.T, options []fs.Option, configFileName string) func() {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	config.ClearConfigPassword()
	_ = os.Unsetenv("_RCLONE_CONFIG_KEY_FILE")
	_ = os.Unsetenv("RCLONE_CONFIG_PASS")
	// create temp config file
	tempFile, err := os.CreateTemp("", configFileName)
	assert.NoError(t, err)
	path := tempFile.Name()
	assert.NoError(t, tempFile.Close())

	// temporarily adapt configuration
	oldOsStdout := os.Stdout
	oldConfigPath := config.GetConfigPath()
	oldConfig := *ci
	oldConfigFile := config.Data()
	oldReadLine := config.ReadLine
	oldPassword := config.Password
	os.Stdout = nil
	assert.NoError(t, config.SetConfigPath(path))
	ci = &fs.ConfigInfo{}

	configfile.Install()
	assert.Equal(t, []string{}, config.Data().GetSectionList())

	// Fake a filesystem/backend
	backendName := "config_test_remote"
	if regInfo, _ := fs.Find(backendName); regInfo != nil {
		regInfo.Options = options
	} else {
		fs.Register(&fs.RegInfo{
			Name:    backendName,
			Options: options,
		})
	}

	// Undo the above (except registered backend, unfortunately)
	return func() {
		err := os.Remove(path)
		assert.NoError(t, err)

		os.Stdout = oldOsStdout
		assert.NoError(t, config.SetConfigPath(oldConfigPath))
		config.ReadLine = oldReadLine
		config.Password = oldPassword
		*ci = oldConfig
		config.SetData(oldConfigFile)

		_ = os.Unsetenv("_RCLONE_CONFIG_KEY_FILE")
		_ = os.Unsetenv("RCLONE_CONFIG_PASS")
	}
}

// makeReadLine makes a simple readLine which returns a fixed list of
// strings
func makeReadLine(answers []string) func() string {
	i := 0
	return func() string {
		i = i + 1
		return answers[i-1]
	}
}

func TestCRUD(t *testing.T) {
	defer testConfigFile(t, simpleOptions, "crud.conf")()
	ctx := context.Background()

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"true",               // bool value
		"y",                  // type my own password
		"secret",             // password
		"secret",             // repeat
		"n",                  // don't edit advanced config
		"y",                  // looks good, save
	})
	require.NoError(t, config.NewRemote(ctx, "test"))

	assert.Equal(t, []string{"test"}, config.Data().GetSectionList())
	assert.Equal(t, "config_test_remote", config.FileGet("test", "type"))
	assert.Equal(t, "true", config.FileGet("test", "bool"))
	assert.Equal(t, "secret", obscure.MustReveal(config.FileGet("test", "pass")))

	// normal rename, test → asdf
	config.ReadLine = makeReadLine([]string{
		"asdf",
		"asdf",
		"asdf",
	})
	config.RenameRemote("test")

	assert.Equal(t, []string{"asdf"}, config.Data().GetSectionList())
	assert.Equal(t, "config_test_remote", config.FileGet("asdf", "type"))
	assert.Equal(t, "true", config.FileGet("asdf", "bool"))
	assert.Equal(t, "secret", obscure.MustReveal(config.FileGet("asdf", "pass")))

	// delete remote
	config.DeleteRemote("asdf")
	assert.Equal(t, []string{}, config.Data().GetSectionList())
}

func TestChooseOption(t *testing.T) {
	defer testConfigFile(t, simpleOptions, "crud.conf")()
	ctx := context.Background()

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"false",              // bool value
		"x",                  // bad choice
		"g",                  // generate password
		"1024",               // very big
		"y",                  // password OK
		"y",                  // looks good, save
	})
	config.Password = func(bits int) (string, error) {
		assert.Equal(t, 1024, bits)
		return "not very random password", nil
	}
	require.NoError(t, config.NewRemote(ctx, "test"))

	assert.Equal(t, "", config.FileGet("test", "bool")) // this is the default now
	assert.Equal(t, "not very random password", obscure.MustReveal(config.FileGet("test", "pass")))

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"true",               // bool value
		"n",                  // not required
		"y",                  // looks good, save
	})
	require.NoError(t, config.NewRemote(ctx, "test"))

	assert.Equal(t, "true", config.FileGet("test", "bool"))
	assert.Equal(t, "", config.FileGet("test", "pass"))
}

func TestNewRemoteName(t *testing.T) {
	defer testConfigFile(t, simpleOptions, "crud.conf")()
	ctx := context.Background()

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"true",               // bool value
		"n",                  // not required
		"y",                  // looks good, save
	})
	require.NoError(t, config.NewRemote(ctx, "test"))

	config.ReadLine = makeReadLine([]string{
		"test",           // already exists
		"",               // empty string not allowed
		"bad^characters", // bad characters
		"newname",        // OK
	})

	assert.Equal(t, "newname", config.NewRemoteName())
}

func TestCreateUpdatePasswordRemote(t *testing.T) {
	ctx := context.Background()
	defer testConfigFile(t, simpleOptions, "update.conf")()

	for _, doObscure := range []bool{false, true} {
		for _, noObscure := range []bool{false, true} {
			if doObscure && noObscure {
				break
			}
			t.Run(fmt.Sprintf("doObscure=%v,noObscure=%v", doObscure, noObscure), func(t *testing.T) {
				opt := config.UpdateRemoteOpt{
					Obscure:   doObscure,
					NoObscure: noObscure,
				}
				_, err := config.CreateRemote(ctx, "test2", "config_test_remote", rc.Params{
					"bool": true,
					"pass": "potato",
				}, opt)
				require.NoError(t, err)

				assert.Equal(t, []string{"test2"}, config.Data().GetSectionList())
				assert.Equal(t, "config_test_remote", config.FileGet("test2", "type"))
				assert.Equal(t, "true", config.FileGet("test2", "bool"))
				gotPw := config.FileGet("test2", "pass")
				if !noObscure {
					gotPw = obscure.MustReveal(gotPw)
				}
				assert.Equal(t, "potato", gotPw)

				wantPw := obscure.MustObscure("potato2")
				_, err = config.UpdateRemote(ctx, "test2", rc.Params{
					"bool":  false,
					"pass":  wantPw,
					"spare": "spare",
				}, opt)
				require.NoError(t, err)

				assert.Equal(t, []string{"test2"}, config.Data().GetSectionList())
				assert.Equal(t, "config_test_remote", config.FileGet("test2", "type"))
				assert.Equal(t, "false", config.FileGet("test2", "bool"))
				gotPw = config.FileGet("test2", "pass")
				if doObscure {
					gotPw = obscure.MustReveal(gotPw)
				}
				assert.Equal(t, wantPw, gotPw)

				require.NoError(t, config.PasswordRemote(ctx, "test2", rc.Params{
					"pass": "potato3",
				}))

				assert.Equal(t, []string{"test2"}, config.Data().GetSectionList())
				assert.Equal(t, "config_test_remote", config.FileGet("test2", "type"))
				assert.Equal(t, "false", config.FileGet("test2", "bool"))
				assert.Equal(t, "potato3", obscure.MustReveal(config.FileGet("test2", "pass")))
			})
		}
	}
}

func TestDefaultRequired(t *testing.T) {
	// By default options are optional (sic), regardless if a default value is defined.
	// Setting Required=true means empty string is no longer allowed, except when
	// a default value is set: Default value means empty string is always allowed!
	options := []fs.Option{{
		Name:     "string_required",
		Required: true,
	}, {
		Name:    "string_default",
		Default: "AAA",
	}, {
		Name:     "string_required_default",
		Default:  "BBB",
		Required: true,
	}}

	defer testConfigFile(t, options, "crud.conf")()
	ctx := context.Background()

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"111",                // string_required
		"222",                // string_default
		"333",                // string_required_default
		"y",                  // looks good, save
	})
	require.NoError(t, config.NewRemote(ctx, "test"))

	assert.Equal(t, []string{"test"}, config.Data().GetSectionList())
	assert.Equal(t, "config_test_remote", config.FileGet("test", "type"))
	assert.Equal(t, "111", config.FileGet("test", "string_required"))
	assert.Equal(t, "222", config.FileGet("test", "string_default"))
	assert.Equal(t, "333", config.FileGet("test", "string_required_default"))

	// delete remote
	config.DeleteRemote("test")
	assert.Equal(t, []string{}, config.Data().GetSectionList())

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"",                   // string_required - invalid (empty string not allowed)
		"111",                // string_required - valid
		"",                   // string_default (empty string allowed, means use default)
		"",                   // string_required_default (empty string allowed, means use default)
		"y",                  // looks good, save
	})
	require.NoError(t, config.NewRemote(ctx, "test"))

	assert.Equal(t, []string{"test"}, config.Data().GetSectionList())
	assert.Equal(t, "config_test_remote", config.FileGet("test", "type"))
	assert.Equal(t, "111", config.FileGet("test", "string_required"))
	assert.Equal(t, "", config.FileGet("test", "string_default"))
	assert.Equal(t, "", config.FileGet("test", "string_required_default"))
}

func TestMultipleChoice(t *testing.T) {
	// Multiple-choice options can be set to the number of a predefined choice, or
	// its text. Unless Exclusive=true, tested later, any free text input is accepted.
	//
	// By default options are optional, regardless if a default value is defined.
	// Setting Required=true means empty string is no longer allowed, except when
	// a default value is set: Default value means empty string is always allowed!
	options := []fs.Option{{
		Name: "multiple_choice",
		Examples: []fs.OptionExample{{
			Value: "AAA",
			Help:  "This is value AAA",
		}, {
			Value: "BBB",
			Help:  "This is value BBB",
		}, {
			Value: "CCC",
			Help:  "This is value CCC",
		}},
	}, {
		Name:     "multiple_choice_required",
		Required: true,
		Examples: []fs.OptionExample{{
			Value: "AAA",
			Help:  "This is value AAA",
		}, {
			Value: "BBB",
			Help:  "This is value BBB",
		}, {
			Value: "CCC",
			Help:  "This is value CCC",
		}},
	}, {
		Name:    "multiple_choice_default",
		Default: "BBB",
		Examples: []fs.OptionExample{{
			Value: "AAA",
			Help:  "This is value AAA",
		}, {
			Value: "BBB",
			Help:  "This is value BBB",
		}, {
			Value: "CCC",
			Help:  "This is value CCC",
		}},
	}, {
		Name:     "multiple_choice_required_default",
		Required: true,
		Default:  "BBB",
		Examples: []fs.OptionExample{{
			Value: "AAA",
			Help:  "This is value AAA",
		}, {
			Value: "BBB",
			Help:  "This is value BBB",
		}, {
			Value: "CCC",
			Help:  "This is value CCC",
		}},
	}}

	defer testConfigFile(t, options, "crud.conf")()
	ctx := context.Background()

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"3",                  // multiple_choice
		"3",                  // multiple_choice_required
		"3",                  // multiple_choice_default
		"3",                  // multiple_choice_required_default
		"y",                  // looks good, save
	})
	require.NoError(t, config.NewRemote(ctx, "test"))

	assert.Equal(t, []string{"test"}, config.Data().GetSectionList())
	assert.Equal(t, "config_test_remote", config.FileGet("test", "type"))
	assert.Equal(t, "CCC", config.FileGet("test", "multiple_choice"))
	assert.Equal(t, "CCC", config.FileGet("test", "multiple_choice_required"))
	assert.Equal(t, "CCC", config.FileGet("test", "multiple_choice_default"))
	assert.Equal(t, "CCC", config.FileGet("test", "multiple_choice_required_default"))

	// delete remote
	config.DeleteRemote("test")
	assert.Equal(t, []string{}, config.Data().GetSectionList())

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"XXX",                // multiple_choice
		"XXX",                // multiple_choice_required
		"XXX",                // multiple_choice_default
		"XXX",                // multiple_choice_required_default
		"y",                  // looks good, save
	})
	require.NoError(t, config.NewRemote(ctx, "test"))

	assert.Equal(t, []string{"test"}, config.Data().GetSectionList())
	assert.Equal(t, "config_test_remote", config.FileGet("test", "type"))
	assert.Equal(t, "XXX", config.FileGet("test", "multiple_choice"))
	assert.Equal(t, "XXX", config.FileGet("test", "multiple_choice_required"))
	assert.Equal(t, "XXX", config.FileGet("test", "multiple_choice_default"))
	assert.Equal(t, "XXX", config.FileGet("test", "multiple_choice_required_default"))

	// delete remote
	config.DeleteRemote("test")
	assert.Equal(t, []string{}, config.Data().GetSectionList())

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"",                   // multiple_choice (empty string allowed)
		"",                   // multiple_choice_required - invalid (empty string not allowed)
		"XXX",                // multiple_choice_required - valid (value not restricted to examples)
		"",                   // multiple_choice_default (empty string allowed)
		"",                   // multiple_choice_required_default (required does nothing when default is set)
		"y",                  // looks good, save
	})
	require.NoError(t, config.NewRemote(ctx, "test"))

	assert.Equal(t, []string{"test"}, config.Data().GetSectionList())
	assert.Equal(t, "config_test_remote", config.FileGet("test", "type"))
	assert.Equal(t, "", config.FileGet("test", "multiple_choice"))
	assert.Equal(t, "XXX", config.FileGet("test", "multiple_choice_required"))
	assert.Equal(t, "", config.FileGet("test", "multiple_choice_default"))
	assert.Equal(t, "", config.FileGet("test", "multiple_choice_required_default"))
}

func TestMultipleChoiceExclusive(t *testing.T) {
	// Setting Exclusive=true on multiple-choice option means any input
	// value must be from the predefined list, but empty string is allowed.
	// Setting a default value makes no difference.
	options := []fs.Option{{
		Name:      "multiple_choice_exclusive",
		Exclusive: true,
		Examples: []fs.OptionExample{{
			Value: "AAA",
			Help:  "This is value AAA",
		}, {
			Value: "BBB",
			Help:  "This is value BBB",
		}, {
			Value: "CCC",
			Help:  "This is value CCC",
		}},
	}, {
		Name:      "multiple_choice_exclusive_default",
		Exclusive: true,
		Default:   "CCC",
		Examples: []fs.OptionExample{{
			Value: "AAA",
			Help:  "This is value AAA",
		}, {
			Value: "BBB",
			Help:  "This is value BBB",
		}, {
			Value: "CCC",
			Help:  "This is value CCC",
		}},
	}}

	defer testConfigFile(t, options, "crud.conf")()
	ctx := context.Background()

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"XXX",                // multiple_choice_exclusive - invalid (not a value from examples)
		"",                   // multiple_choice_exclusive - valid (empty string allowed)
		"YYY",                // multiple_choice_exclusive_default - invalid (not a value from examples)
		"",                   // multiple_choice_exclusive_default - valid (empty string allowed)
		"y",                  // looks good, save
	})
	require.NoError(t, config.NewRemote(ctx, "test"))

	assert.Equal(t, []string{"test"}, config.Data().GetSectionList())
	assert.Equal(t, "config_test_remote", config.FileGet("test", "type"))
	assert.Equal(t, "", config.FileGet("test", "multiple_choice_exclusive"))
	assert.Equal(t, "", config.FileGet("test", "multiple_choice_exclusive_default"))
}

func TestMultipleChoiceExclusiveRequired(t *testing.T) {
	// Setting Required=true together with Exclusive=true on multiple-choice option
	// means empty string is no longer allowed, except when a default value is set
	// (default value means empty string is always allowed).
	options := []fs.Option{{
		Name:      "multiple_choice_exclusive_required",
		Exclusive: true,
		Required:  true,
		Examples: []fs.OptionExample{{
			Value: "AAA",
			Help:  "This is value AAA",
		}, {
			Value: "BBB",
			Help:  "This is value BBB",
		}, {
			Value: "CCC",
			Help:  "This is value CCC",
		}},
	}, {
		Name:      "multiple_choice_exclusive_required_default",
		Exclusive: true,
		Required:  true,
		Default:   "CCC",
		Examples: []fs.OptionExample{{
			Value: "AAA",
			Help:  "This is value AAA",
		}, {
			Value: "BBB",
			Help:  "This is value BBB",
		}, {
			Value: "CCC",
			Help:  "This is value CCC",
		}},
	}}

	defer testConfigFile(t, options, "crud.conf")()
	ctx := context.Background()

	// script for creating remote
	config.ReadLine = makeReadLine([]string{
		"config_test_remote", // type
		"XXX",                // multiple_choice_exclusive_required - invalid (not a value from examples)
		"",                   // multiple_choice_exclusive_required - invalid (empty string not allowed)
		"CCC",                // multiple_choice_exclusive_required - valid
		"XXX",                // multiple_choice_exclusive_required_default - invalid (not a value from examples)
		"",                   // multiple_choice_exclusive_required_default - valid (empty string allowed)
		"y",                  // looks good, save
	})
	require.NoError(t, config.NewRemote(ctx, "test"))

	assert.Equal(t, []string{"test"}, config.Data().GetSectionList())
	assert.Equal(t, "config_test_remote", config.FileGet("test", "type"))
	assert.Equal(t, "CCC", config.FileGet("test", "multiple_choice_exclusive_required"))
	assert.Equal(t, "", config.FileGet("test", "multiple_choice_exclusive_required_default"))
}
