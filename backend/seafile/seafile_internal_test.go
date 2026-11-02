package seafile

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/asyncreader"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
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
		// paths inside a library are returned in NFC, library names are kept as they are
		{
			configLibrary:   "Library",
			configRoot:      "",
			argumentPath:    norm.NFD.String("path/to/équivalent.txt"),
			expectedLibrary: "Library",
			expectedPath:    norm.NFC.String("path/to/équivalent.txt"),
		},
		{
			configLibrary:   "Library",
			configRoot:      norm.NFD.String("répertoire"),
			argumentPath:    norm.NFD.String("équivalent.txt"),
			expectedLibrary: "Library",
			expectedPath:    norm.NFC.String("répertoire/équivalent.txt"),
		},
		{
			configLibrary:   "",
			configRoot:      "",
			argumentPath:    norm.NFD.String("Bibliothèque/équivalent.txt"),
			expectedLibrary: norm.NFD.String("Bibliothèque"),
			expectedPath:    norm.NFC.String("équivalent.txt"),
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

// Test that a failed upload stops reading the source so it can be closed
func TestUploadErrorReleasesSource(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reply without reading the body, like a server rejecting the upload early
		http.Error(w, "rejected", http.StatusBadRequest)
	}))
	defer server.Close()

	f := &Fs{
		srv:   rest.NewClient(server.Client()),
		pacer: fs.NewPacer(ctx, pacer.NewDefault()),
	}
	// An AsyncReader holds its lock while writing to the upload
	in, err := asyncreader.New(ctx, io.NopCloser(readers.NewPatternReader(64<<20)), 4)
	require.NoError(t, err)

	_, err = f.upload(ctx, in, server.URL+"/upload", "/file.bin")
	require.Error(t, err)

	closed := make(chan struct{})
	go func() {
		_ = in.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("source still in use after upload returned")
	}
}
