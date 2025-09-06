//go:build ignore

package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest/runs"
	"github.com/stretchr/testify/assert/yaml"
)

var path = flag.String("path", "./docs/content/", "root path")

const (
	configFile              = "fstest/test_all/config.yaml"
	startListIgnores        = "<!--- start list_ignores - DO NOT EDIT THIS SECTION - use make commanddocs --->"
	endListIgnores          = "<!--- end list_ignores - DO NOT EDIT THIS SECTION - use make commanddocs --->"
	startListFailures       = "<!--- start list_failures - DO NOT EDIT THIS SECTION - use make commanddocs --->"
	endListFailures         = "<!--- end list_failures - DO NOT EDIT THIS SECTION - use make commanddocs --->"
	integrationTestsJSONURL = "https://pub.rclone.org/integration-tests/current/index.json"
	integrationTestsHTMLURL = "https://pub.rclone.org/integration-tests/current/"
)

func main() {
	err := replaceBetween(*path, startListIgnores, endListIgnores, getIgnores)
	if err != nil {
		fs.Errorf(*path, "error replacing ignores: %v", err)
	}
	err = replaceBetween(*path, startListFailures, endListFailures, getFailures)
	if err != nil {
		fs.Errorf(*path, "error replacing failures: %v", err)
	}
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
	slices.SortFunc(config.Backends, func(a, b runs.Backend) int {
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

	r := runs.Report{}
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
func parseConfig() (*runs.Config, error) {
	d, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	config := &runs.Config{}
	err = yaml.Unmarshal(d, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	return config, nil
}
