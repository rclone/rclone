package seafile

import (
	"path"
	"testing"

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
		pathData{
			configLibrary:   "",
			configRoot:      "",
			argumentPath:    "",
			expectedLibrary: "",
			expectedPath:    "",
		},
		pathData{
			configLibrary:   "",
			configRoot:      "",
			argumentPath:    "Library",
			expectedLibrary: "Library",
			expectedPath:    "",
		},
		pathData{
			configLibrary:   "",
			configRoot:      "",
			argumentPath:    path.Join("Library", "path", "to", "file"),
			expectedLibrary: "Library",
			expectedPath:    path.Join("path", "to", "file"),
		},
		pathData{
			configLibrary:   "Library",
			configRoot:      "",
			argumentPath:    "",
			expectedLibrary: "Library",
			expectedPath:    "",
		},
		pathData{
			configLibrary:   "Library",
			configRoot:      "",
			argumentPath:    "path",
			expectedLibrary: "Library",
			expectedPath:    "path",
		},
		pathData{
			configLibrary:   "Library",
			configRoot:      "",
			argumentPath:    path.Join("path", "to", "file"),
			expectedLibrary: "Library",
			expectedPath:    path.Join("path", "to", "file"),
		},
		pathData{
			configLibrary:   "Library",
			configRoot:      "root",
			argumentPath:    "",
			expectedLibrary: "Library",
			expectedPath:    "root",
		},
		pathData{
			configLibrary:   "Library",
			configRoot:      path.Join("root", "path"),
			argumentPath:    "",
			expectedLibrary: "Library",
			expectedPath:    path.Join("root", "path"),
		},
		pathData{
			configLibrary:   "Library",
			configRoot:      "root",
			argumentPath:    "path",
			expectedLibrary: "Library",
			expectedPath:    path.Join("root", "path"),
		},
		pathData{
			configLibrary:   "Library",
			configRoot:      "root",
			argumentPath:    path.Join("path", "to", "file"),
			expectedLibrary: "Library",
			expectedPath:    path.Join("root", "path", "to", "file"),
		},
		pathData{
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
