package bisync_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rclone/rclone/cmd/bisync/bilib"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

const configFile = "../../fstest/test_all/config.yaml"

// Config describes the config for this program
type Config struct {
	Tests    []Test
	Backends []Backend
}

// Test describes an integration test to run with `go test`
type Test struct {
	Path       string // path to the source directory
	FastList   bool   // if it is possible to add -fast-list to tests
	Short      bool   // if it is possible to run the test with -short
	AddBackend bool   // set if Path needs the current backend appending
	NoRetries  bool   // set if no retries should be performed
	NoBinary   bool   // set to not build a binary in advance
	LocalOnly  bool   // if set only run with the local backend
}

// Backend describes a backend test
//
// FIXME make bucket-based remotes set sub-dir automatically???
type Backend struct {
	Backend     string   // name of the backend directory
	Remote      string   // name of the test remote
	FastList    bool     // set to test with -fast-list
	Short       bool     // set to test with -short
	OneOnly     bool     // set to run only one backend test at once
	MaxFile     string   // file size limit
	CleanUp     bool     // when running clean, run cleanup first
	Ignore      []string // test names to ignore the failure of
	Tests       []string // paths of tests to run, blank for all
	ListRetries int      // -list-retries if > 0
	ExtraTime   float64  // factor to multiply the timeout by
}

func parseConfig() (*Config, error) {
	d, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	config := &Config{}
	err = yaml.Unmarshal(d, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	return config, nil
}

const debugFormat = `		{
			"name": %q,
			"type": "go",
			"request": "launch",
			"mode": "test",
			"program": "./cmd/bisync",
			"args": ["-remote", %q, "-remote2", %q, "-case", %q, "-no-cleanup"]
		},
`

const docFormat = `{
    "version": "0.2.0",
    "configurations": [
%s
    ]
}`

// generates a launch.json file for debugging in VS Code.
// note: just copy the ones you need into your real launch.json file, as VS Code will crash if there are too many!
func (b *bisyncTest) generateDebuggers() {
	config, err := parseConfig()
	if err != nil {
		fs.Errorf(config, "failed to parse config: %v", err)
	}

	testList := []string{}
	for _, testCase := range b.listDir(b.dataRoot) {
		if strings.HasPrefix(testCase, "test_") {
			// if dir is empty, skip it (can happen due to gitignored files/dirs when checking out branch)
			if len(b.listDir(filepath.Join(b.dataRoot, testCase))) == 0 {
				continue
			}
			testList = append(testList, testCase)
		}
	}

	variations := []string{"LocalRemote", "RemoteLocal", "RemoteRemote"}
	debuggers := ""

	for _, backend := range config.Backends {
		if backend.Remote == "" {
			backend.Remote = "local"
		}
		for _, testcase := range testList {
			for _, variation := range variations {
				if variation != "RemoteRemote" && backend.Remote == "local" {
					continue
				}

				name := fmt.Sprintf("Test %s %s %s", backend.Remote, testcase, variation)
				switch variation {
				case "LocalRemote":
					debuggers += fmt.Sprintf(debugFormat, name, "local", backend.Remote, testcase)
				case "RemoteLocal":
					debuggers += fmt.Sprintf(debugFormat, name, backend.Remote, "local", testcase)
				case "RemoteRemote":
					debuggers += fmt.Sprintf(debugFormat, name, backend.Remote, backend.Remote, testcase)
				}
			}
		}
	}

	out := fmt.Sprintf(docFormat, debuggers)
	outpath := "./testdata/bisync_vscode_debuggers_launch.json"
	err = os.WriteFile(outpath, []byte(out), bilib.PermSecure)
	assert.NoError(b.t, err, "writing golden file %s", outpath)
}
