// Config handling

package main

import (
	"fmt"
	"os"
	"path"

	"github.com/rclone/rclone/fs"
	yaml "gopkg.in/yaml.v2"
)

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

// includeTest returns true if this backend should be included in this
// test
func (b *Backend) includeTest(t *Test) bool {
	if len(b.Tests) == 0 {
		return true
	}
	for _, testPath := range b.Tests {
		if testPath == t.Path {
			return true
		}
	}
	return false
}

// MakeRuns creates Run objects the Backend and Test
//
// There can be several created, one for each combination of optional
// flags (e.g. FastList)
func (b *Backend) MakeRuns(t *Test) (runs []*Run) {
	if !b.includeTest(t) {
		return runs
	}
	maxSize := fs.SizeSuffix(0)
	if b.MaxFile != "" {
		if err := maxSize.Set(b.MaxFile); err != nil {
			fs.Logf(nil, "Invalid maxfile value %q: %v", b.MaxFile, err)
		}
	}
	fastlists := []bool{false}
	if b.FastList && t.FastList {
		fastlists = append(fastlists, true)
	}
	ignore := make(map[string]struct{}, len(b.Ignore))
	for _, item := range b.Ignore {
		ignore[item] = struct{}{}
	}
	for _, fastlist := range fastlists {
		if t.LocalOnly && b.Backend != "local" {
			continue
		}
		run := &Run{
			Remote:      b.Remote,
			Backend:     b.Backend,
			Path:        t.Path,
			FastList:    fastlist,
			Short:       (b.Short && t.Short),
			NoRetries:   t.NoRetries,
			OneOnly:     b.OneOnly,
			NoBinary:    t.NoBinary,
			SizeLimit:   int64(maxSize),
			Ignore:      ignore,
			ListRetries: b.ListRetries,
			ExtraTime:   b.ExtraTime,
		}
		if t.AddBackend {
			run.Path = path.Join(run.Path, b.Backend)
		}
		runs = append(runs, run)
	}
	return runs
}

// Config describes the config for this program
type Config struct {
	Tests    []Test
	Backends []Backend
}

// NewConfig reads the config file
func NewConfig(configFile string) (*Config, error) {
	d, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	config := &Config{}
	err = yaml.Unmarshal(d, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	// d, err = yaml.Marshal(&config)
	// if err != nil {
	// 	log.Fatalf("error: %v", err)
	// }
	// fmt.Printf("--- m dump:\n%s\n\n", string(d))
	return config, nil
}

// MakeRuns makes Run objects for each combination of Backend and Test
// in the config
func (c *Config) MakeRuns() (runs Runs) {
	for _, backend := range c.Backends {
		for _, test := range c.Tests {
			runs = append(runs, backend.MakeRuns(&test)...)
		}
	}
	return runs
}

// Filter the Backends with the remotes passed in.
//
// If no backend is found with a remote is found then synthesize one
func (c *Config) filterBackendsByRemotes(remotes []string) {
	var newBackends []Backend
	for _, name := range remotes {
		found := false
		for i := range c.Backends {
			if c.Backends[i].Remote == name {
				newBackends = append(newBackends, c.Backends[i])
				found = true
			}
		}
		if !found {
			fs.Logf(nil, "Remote %q not found - inserting with default flags", name)
			// Lookup which backend
			fsInfo, _, _, _, err := fs.ConfigFs(name)
			if err != nil {
				fs.Fatalf(nil, "couldn't find remote %q: %v", name, err)
			}
			newBackends = append(newBackends, Backend{Backend: fsInfo.FileName(), Remote: name})
		}
	}
	c.Backends = newBackends
}

// Filter the Backends with the backendNames passed in
func (c *Config) filterBackendsByBackends(backendNames []string) {
	var newBackends []Backend
	for _, name := range backendNames {
		for i := range c.Backends {
			if c.Backends[i].Backend == name {
				newBackends = append(newBackends, c.Backends[i])
			}
		}
	}
	c.Backends = newBackends
}

// Filter the incoming tests into the backends selected
func (c *Config) filterTests(paths []string) {
	var newTests []Test
	for _, path := range paths {
		for i := range c.Tests {
			if c.Tests[i].Path == path {
				newTests = append(newTests, c.Tests[i])
			}
		}
	}
	c.Tests = newTests
}
