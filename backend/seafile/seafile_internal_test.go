package seafile

import (
	"path"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
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

func TestRiConfig(t *testing.T) {
	states := []fstest.ConfigStateTestFixture{
		{
			Name:       "no url",
			Mapper:     configmap.Simple{},
			Input:      fs.ConfigIn{State: ""},
			ExpectFail: true,
		},
		{
			Name:       "unknown state",
			Mapper:     configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "userName"},
			Input:      fs.ConfigIn{State: "unknown"},
			ExpectFail: true,
		},
		{
			Name:        "2fa not set",
			Mapper:      configmap.Simple{"url": "http://localhost/"},
			Input:       fs.ConfigIn{State: ""},
			ExpectState: "description_complete",
		},
		{
			Name:        "no password in config",
			Mapper:      configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "userName"},
			Input:       fs.ConfigIn{State: ""},
			ExpectState: "password",
		},
		{
			Name:        "config ready for 2fa token",
			Mapper:      configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "userName", "pass": obscure.MustObscure("password")},
			Input:       fs.ConfigIn{State: ""},
			ExpectState: "2fa",
		},
		{
			Name:               "password not entered",
			Mapper:             configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "userName"},
			Input:              fs.ConfigIn{State: "password"},
			ExpectState:        "",
			ExpectErrorMessage: "Password can't be blank",
		},
		{
			Name:        "password entered",
			Mapper:      configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "userName"},
			Input:       fs.ConfigIn{State: "password", Result: "password"},
			ExpectState: "2fa",
		},
		{
			Name:        "ask for a 2fa code",
			Mapper:      configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "userName"},
			Input:       fs.ConfigIn{State: "2fa"},
			ExpectState: "2fa_do",
		},
		{
			Name:               "no 2fa code entered",
			Mapper:             configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "userName"},
			Input:              fs.ConfigIn{State: "2fa_do"},
			ExpectState:        "2fa", // ask for a code again
			ExpectErrorMessage: "2FA codes can't be blank",
		},
		{
			Name:        "2fa error and retry",
			Mapper:      configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "userName"},
			Input:       fs.ConfigIn{State: "2fa_error", Result: "true"},
			ExpectState: "2fa", // ask for a code again
		},
		{
			Name:       "2fa error and fail",
			Mapper:     configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "userName"},
			Input:      fs.ConfigIn{State: "2fa_error"},
			ExpectFail: true,
		},
		{
			Name:            "description complete",
			Mapper:          configmap.Simple{"url": "http://localhost/", "2fa": "true", "user": "userName"},
			Input:           fs.ConfigIn{State: "description_complete", Result: "new description"},
			ExpectMapper:    configmap.Simple{fs.ConfigDescription: "new description", "url": "http://localhost/", "2fa": "true", "user": "userName"},
			ExpectNilOutput: true,
		},
	}
	fstest.AssertConfigStates(t, states, riConfig)
}
