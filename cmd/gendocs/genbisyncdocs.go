// Package gendocs provides the gendocs command.
package gendocs

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rclone/rclone/fs/operations"
	"github.com/stretchr/testify/assert/yaml"
)

const (
	configFile              = "fstest/test_all/config.yaml"
	startListIgnores        = "<!--- start list_ignores - DO NOT EDIT THIS SECTION - use rclone gendocs --->"
	endListIgnores          = "<!--- end list_ignores - DO NOT EDIT THIS SECTION - use rclone gendocs --->"
	startListFailures       = "<!--- start list_failures - DO NOT EDIT THIS SECTION - use rclone gendocs --->"
	endListFailures         = "<!--- end list_failures - DO NOT EDIT THIS SECTION - use rclone gendocs --->"
	integrationTestsJSONURL = "https://pub.rclone.org/integration-tests/current/index.json"
	integrationTestsHTMLURL = "https://pub.rclone.org/integration-tests/current/"
)

// genBisyncDocs updates bisync.md by replacing text between separators.
// It is run at the end of rclone gendocs.
func genBisyncDocs(path string) error {
	err := replaceBetween(path, startListIgnores, endListIgnores, getIgnores)
	if err != nil {
		return err
	}
	return replaceBetween(path, startListFailures, endListFailures, getFailures)
}

// replaceBetween replaces the text between startSep and endSep with fn()
func replaceBetween(path, startSep, endSep string, fn func() (string, error)) error {
	b, err := os.ReadFile(filepath.Join(path, "bisync.md"))
	if err != nil {
		return err
	}
	doc := string(b)

	before, after, found := strings.Cut(doc, startSep)
	if !found {
		return fmt.Errorf("could not find: %v", startSep)
	}
	_, after, found = strings.Cut(after, endSep)
	if !found {
		return fmt.Errorf("could not find: %v", endSep)
	}

	replaceSection, err := fn()
	if err != nil {
		return err
	}

	newDoc := before + startSep + "\n" + strings.TrimSpace(replaceSection) + "\n" + endSep + after

	err = os.WriteFile(filepath.Join(path, "bisync.md"), []byte(newDoc), 0777)
	if err != nil {
		return err
	}
	return nil
}

// getIgnores updates the list of ignores from config.yaml
func getIgnores() (string, error) {
	config, err := parseConfig()
	if err != nil {
		return "", fmt.Errorf("failed to parse config: %v", err)
	}
	s := ""
	slices.SortFunc(config.Backends, func(a, b backend) int {
		return cmp.Compare(a.Remote, b.Remote)
	})
	for _, backend := range config.Backends {
		include := false

		if slices.Contains(backend.IgnoreTests, "cmd/bisync") {
			include = true
			s += fmt.Sprintf("- `%s` (`%s`)\n", strings.TrimSuffix(backend.Remote, ":"), backend.Backend)
		}

		for _, ignore := range backend.Ignore {
			if strings.Contains(strings.ToLower(ignore), "bisync") {
				if !include { // don't have header row yet
					s += fmt.Sprintf("- `%s` (`%s`)\n", strings.TrimSuffix(backend.Remote, ":"), backend.Backend)
				}
				include = true
				s += fmt.Sprintf("  - `%s`\n", ignore)
				// TODO: might be neat to add a "reason" param displaying the reason the test is ignored
			}
		}
	}
	return s, nil
}

// getFailures updates the list of currently failing tests from the integration tests server
func getFailures() (string, error) {
	var buf bytes.Buffer
	err := operations.CopyURLToWriter(context.Background(), integrationTestsJSONURL, &buf)
	if err != nil {
		return "", err
	}

	r := report{}
	err = json.Unmarshal(buf.Bytes(), &r)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal json: %v", err)
	}

	s := ""
	for _, run := range r.Failed {
		for i, t := range run.FailedTests {
			if strings.Contains(strings.ToLower(t), "bisync") {

				if i == 0 { // don't have header row yet
					s += fmt.Sprintf("- `%s` (`%s`)\n", strings.TrimSuffix(run.Remote, ":"), run.Backend)
				}

				url := integrationTestsHTMLURL + run.TrialName
				url = url[:len(url)-5] + "1.txt" // numbers higher than 1 could change from night to night
				s += fmt.Sprintf("  - [`%s`](%v)\n", t, url)

				if i == 4 && len(run.FailedTests) > 5 { // stop after 5
					s += fmt.Sprintf("  - [%v more](%v)\n", len(run.FailedTests)-5, integrationTestsHTMLURL)
					break
				}
			}
		}
	}
	s += fmt.Sprintf("- Updated: %v", r.DateTime)
	return s, nil
}

// parseConfig reads and parses the config.yaml file
func parseConfig() (*config, error) {
	d, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	config := &config{}
	err = yaml.Unmarshal(d, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	return config, nil
}

/* types copied from /fstest/test_all */

// config describes the config for this program
type config struct {
	Tests    []test
	Backends []backend
}

// test describes an integration test to run with `go test`
type test struct {
	Path       string // path to the source directory
	FastList   bool   // if it is possible to add -fast-list to tests
	Short      bool   // if it is possible to run the test with -short
	AddBackend bool   // set if Path needs the current backend appending
	NoRetries  bool   // set if no retries should be performed
	NoBinary   bool   // set to not build a binary in advance
	LocalOnly  bool   // if set only run with the local backend
}

// backend describes a backend test
type backend struct {
	Backend     string   // name of the backend directory
	Remote      string   // name of the test remote
	FastList    bool     // set to test with -fast-list
	Short       bool     // set to test with -short
	OneOnly     bool     // set to run only one backend test at once
	MaxFile     string   // file size limit
	CleanUp     bool     // when running clean, run cleanup first
	Ignore      []string // test names to ignore the failure of
	Tests       []string // paths of tests to run, blank for all
	IgnoreTests []string // paths of tests not to run, blank for none
	ListRetries int      // -list-retries if > 0
	ExtraTime   float64  // factor to multiply the timeout by
	Env         []string // environment variables to set in form KEY=VALUE
}

// report holds the info to make a report on a series of test runs
type report struct {
	LogDir    string        // output directory for logs and report
	StartTime time.Time     // time started
	DateTime  string        // directory name for output
	Duration  time.Duration // time the run took
	Failed    runs          // failed runs
	Passed    runs          // passed runs
	Runs      []reportRun   // runs to report
	Version   string        // rclone version
	Previous  string        // previous test name if known
	IndexHTML string        // path to the index.html file
	URL       string        // online version
	Branch    string        // rclone branch
	Commit    string        // rclone commit
	GOOS      string        // Go OS
	GOARCH    string        // Go Arch
	GoVersion string        // Go Version
}

// reportRun is used in the templates to report on a test run
type reportRun struct {
	Name string
	Runs runs
}

// runs records multiple Run objects
type runs []*run

// run holds info about a running test
type run struct {
	// Config
	Remote      string // name of the test remote
	Backend     string // name of the backend
	Path        string // path to the source directory
	FastList    bool   // add -fast-list to tests
	Short       bool   // add -short
	NoRetries   bool   // don't retry if set
	OneOnly     bool   // only run test for this backend at once
	NoBinary    bool   // set to not build a binary
	SizeLimit   int64  // maximum test file size
	Ignore      map[string]struct{}
	ListRetries int      // -list-retries if > 0
	ExtraTime   float64  // multiply the timeout by this
	Env         []string // environment variables in form KEY=VALUE
	// Internals
	CmdLine   []string
	CmdString string
	Try       int
	// err         error
	// output      []byte
	FailedTests []string
	RunFlag     string
	LogDir      string   // directory to place the logs
	TrialName   string   // name/log file name of current trial
	TrialNames  []string // list of all the trials
}
