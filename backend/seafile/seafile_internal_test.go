package seafile

import (
	"context"
	"path"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pathData struct {
	configLibrary   string // Library specified in the config
	configRoot      string // Root directory specified in the config
	argumentPath    string // Path given as an argument in the command line
	expectedLibrary string
	expectedPath    string
}

// Test the method to split a library name and a path
// from a mix of configuration data and path command line argument
func TestSplitPath(t *testing.T) {
	testData := []pathData{
		{
			configLibrary:   "",
			configRoot:      "",
			argumentPath:    "",
			expectedLibrary: "",
			expectedPath:    "",
		},
		{
			configLibrary:   "",
			configRoot:      "",
			argumentPath:    "Library",
			expectedLibrary: "Library",
			expectedPath:    "",
		},
		{
			configLibrary:   "",
			configRoot:      "",
			argumentPath:    path.Join("Library", "path", "to", "file"),
			expectedLibrary: "Library",
			expectedPath:    path.Join("path", "to", "file"),
		},
		{
			configLibrary:   "Library",
			configRoot:      "",
			argumentPath:    "",
			expectedLibrary: "Library",
			expectedPath:    "",
		},
		{
			configLibrary:   "Library",
			configRoot:      "",
			argumentPath:    "path",
			expectedLibrary: "Library",
			expectedPath:    "path",
		},
		{
			configLibrary:   "Library",
			configRoot:      "",
			argumentPath:    path.Join("path", "to", "file"),
			expectedLibrary: "Library",
			expectedPath:    path.Join("path", "to", "file"),
		},
		{
			configLibrary:   "Library",
			configRoot:      "root",
			argumentPath:    "",
			expectedLibrary: "Library",
			expectedPath:    "root",
		},
		{
			configLibrary:   "Library",
			configRoot:      path.Join("root", "path"),
			argumentPath:    "",
			expectedLibrary: "Library",
			expectedPath:    path.Join("root", "path"),
		},
		{
			configLibrary:   "Library",
			configRoot:      "root",
			argumentPath:    "path",
			expectedLibrary: "Library",
			expectedPath:    path.Join("root", "path"),
		},
		{
			configLibrary:   "Library",
			configRoot:      "root",
			argumentPath:    path.Join("path", "to", "file"),
			expectedLibrary: "Library",
			expectedPath:    path.Join("root", "path", "to", "file"),
		},
		{
			configLibrary:   "Library",
			configRoot:      path.Join("root", "path"),
			argumentPath:    path.Join("subpath", "to", "file"),
			expectedLibrary: "Library",
			expectedPath:    path.Join("root", "path", "subpath", "to", "file"),
		},
	}
	for _, test := range testData {
		fs := &Fs{
			libraryName:   test.configLibrary,
			rootDirectory: test.configRoot,
		}
		libraryName, path := fs.splitPath(test.argumentPath)

		assert.Equal(t, test.expectedLibrary, libraryName)
		assert.Equal(t, test.expectedPath, path)
	}
}

func TestSplitPathIntoSlice(t *testing.T) {
	testData := map[string][]string{
		"1":     {"1"},
		"/1":    {"1"},
		"/1/":   {"1"},
		"1/2/3": {"1", "2", "3"},
	}
	for input, expected := range testData {
		output := splitPath(input)
		assert.Equal(t, expected, output)
	}
}

func Test2FAStateMachine(t *testing.T) {
	fixtures := []struct {
		name               string
		mapper             configmap.Mapper
		input              fs.ConfigIn
		expectState        string
		expectErrorMessage string
		expectResult       string
		expectFail         bool
		expectNil          bool
	}{
		{
			name:       "no url",
			mapper:     configmap.Simple{},
			input:      fs.ConfigIn{State: ""},
			expectFail: true,
		},
		{
			name:       "unknown state",
			mapper:     configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "username"},
			input:      fs.ConfigIn{State: "unknown"},
			expectFail: true,
		},
		{
			name:      "2fa not set",
			mapper:    configmap.Simple{"url": "http://localhost/"},
			input:     fs.ConfigIn{State: ""},
			expectNil: true,
		},
		{
			name:        "no password in config",
			mapper:      configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "username"},
			input:       fs.ConfigIn{State: ""},
			expectState: "password",
		},
		{
			name:        "config ready for 2fa token",
			mapper:      configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "username", "pass": obscure.MustObscure("password")},
			input:       fs.ConfigIn{State: ""},
			expectState: "2fa",
		},
		{
			name:               "password not entered",
			mapper:             configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "username"},
			input:              fs.ConfigIn{State: "password"},
			expectState:        "",
			expectErrorMessage: "Password can't be blank",
		},
		{
			name:        "password entered",
			mapper:      configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "username"},
			input:       fs.ConfigIn{State: "password", Result: "password"},
			expectState: "2fa",
		},
		{
			name:        "ask for a 2fa code",
			mapper:      configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "username"},
			input:       fs.ConfigIn{State: "2fa"},
			expectState: "2fa_do",
		},
		{
			name:               "no 2fa code entered",
			mapper:             configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "username"},
			input:              fs.ConfigIn{State: "2fa_do"},
			expectState:        "2fa", // ask for a code again
			expectErrorMessage: "2FA codes can't be blank",
		},
		{
			name:        "2fa error and retry",
			mapper:      configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "username"},
			input:       fs.ConfigIn{State: "2fa_error", Result: "true"},
			expectState: "2fa", // ask for a code again
		},
		{
			name:       "2fa error and fail",
			mapper:     configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "username"},
			input:      fs.ConfigIn{State: "2fa_error"},
			expectFail: true,
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			output, err := Config(context.Background(), "test", fixture.mapper, fixture.input)
			if fixture.expectFail {
				require.Error(t, err)
				t.Log(err)
				return
			}
			if fixture.expectNil {
				require.Nil(t, output)
				return
			}
			assert.Equal(t, fixture.expectState, output.State)
			assert.Equal(t, fixture.expectErrorMessage, output.Error)
			assert.Equal(t, fixture.expectResult, output.Result)
		})
	}
}
