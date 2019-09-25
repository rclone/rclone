package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"text/template"
	"time"
)

// Config for gometalinter. This can be loaded from a JSON file with --config.
type Config struct { // nolint: maligned
	// A map from linter name -> <LinterConfig|string>.
	//
	// For backwards compatibility, the value stored in the JSON blob can also
	// be a string of the form "<command>:<pattern>".
	Linters map[string]StringOrLinterConfig

	// The set of linters that should be enabled.
	Enable  []string
	Disable []string

	// A map of linter name to message that is displayed. This is useful when linters display text
	// that is useful only in isolation, such as errcheck which just reports the construct.
	MessageOverride map[string]string
	Severity        map[string]string
	VendoredLinters bool
	Format          string
	Fast            bool
	Install         bool
	Update          bool
	Force           bool
	DownloadOnly    bool
	Debug           bool
	Concurrency     int
	Exclude         []string
	Include         []string
	Skip            []string
	Vendor          bool
	Cyclo           int
	LineLength      int
	MisspellLocale  string
	MinConfidence   float64
	MinOccurrences  int
	MinConstLength  int
	DuplThreshold   int
	Sort            []string
	Test            bool
	Deadline        jsonDuration
	Errors          bool
	JSON            bool
	Checkstyle      bool
	EnableGC        bool
	Aggregate       bool
	EnableAll       bool

	// Warn if a nolint directive was never matched to a linter issue
	WarnUnmatchedDirective bool

	formatTemplate *template.Template
}

type StringOrLinterConfig LinterConfig

func (c *StringOrLinterConfig) UnmarshalJSON(raw []byte) error {
	var linterConfig LinterConfig
	// first try to un-marshall directly into struct
	origErr := json.Unmarshal(raw, &linterConfig)
	if origErr == nil {
		*c = StringOrLinterConfig(linterConfig)
		return nil
	}

	// i.e. bytes didn't represent the struct, treat them as a string
	var linterSpec string
	if err := json.Unmarshal(raw, &linterSpec); err != nil {
		return origErr
	}
	linter, err := parseLinterConfigSpec("", linterSpec)
	if err != nil {
		return err
	}
	*c = StringOrLinterConfig(linter)
	return nil
}

type jsonDuration time.Duration

func (td *jsonDuration) UnmarshalJSON(raw []byte) error {
	var durationAsString string
	if err := json.Unmarshal(raw, &durationAsString); err != nil {
		return err
	}
	duration, err := time.ParseDuration(durationAsString)
	*td = jsonDuration(duration)
	return err
}

// Duration returns the value as a time.Duration
func (td *jsonDuration) Duration() time.Duration {
	return time.Duration(*td)
}

var sortKeys = []string{"none", "path", "line", "column", "severity", "message", "linter"}

// Configuration defaults.
var config = &Config{
	Format: DefaultIssueFormat,

	Linters: map[string]StringOrLinterConfig{},
	Severity: map[string]string{
		"gotype":  "error",
		"gotypex": "error",
		"test":    "error",
		"testify": "error",
		"vet":     "error",
	},
	MessageOverride: map[string]string{
		"errcheck":    "error return value not checked ({message})",
		"gocyclo":     "cyclomatic complexity {cyclo} of function {function}() is high (> {mincyclo})",
		"gofmt":       "file is not gofmted with -s",
		"goimports":   "file is not goimported",
		"safesql":     "potentially unsafe SQL statement",
		"structcheck": "unused struct field {message}",
		"unparam":     "parameter {message}",
		"varcheck":    "unused variable or constant {message}",
	},
	Enable:          defaultEnabled(),
	VendoredLinters: true,
	Concurrency:     runtime.NumCPU(),
	Cyclo:           10,
	LineLength:      80,
	MisspellLocale:  "",
	MinConfidence:   0.8,
	MinOccurrences:  3,
	MinConstLength:  3,
	DuplThreshold:   50,
	Sort:            []string{"none"},
	Deadline:        jsonDuration(time.Second * 30),
}

func loadConfigFile(filename string) error {
	r, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer r.Close() // nolint: errcheck
	err = json.NewDecoder(r).Decode(config)
	if err != nil {
		return err
	}
	for _, disable := range config.Disable {
		for i, enable := range config.Enable {
			if enable == disable {
				config.Enable = append(config.Enable[:i], config.Enable[i+1:]...)
				break
			}
		}
	}
	return err
}

func findDefaultConfigFile() (fullPath string, found bool, err error) {
	prevPath := ""
	dirPath, err := os.Getwd()
	if err != nil {
		return "", false, err
	}

	for dirPath != prevPath {
		fullPath, found, err = findConfigFileInDir(dirPath)
		if err != nil || found {
			return fullPath, found, err
		}
		prevPath, dirPath = dirPath, filepath.Dir(dirPath)
	}

	return "", false, nil
}

func findConfigFileInDir(dirPath string) (fullPath string, found bool, err error) {
	fullPath = filepath.Join(dirPath, defaultConfigPath)
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}

	return fullPath, true, nil
}
