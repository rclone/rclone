// Run a test

package main

import (
	"bytes"
	"context"
	"fmt"
	"go/build"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/testserver"
)

// Control concurrency per backend if required
var (
	oneOnlyMu sync.Mutex
	oneOnly   = map[string]*sync.Mutex{}
)

// Run holds info about a running test
//
// A run just runs one command line, but it can be run multiple times
// if retries are needed.
type Run struct {
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
	ListRetries int     // -list-retries if > 0
	ExtraTime   float64 // multiply the timeout by this
	// Internals
	CmdLine     []string
	CmdString   string
	Try         int
	err         error
	output      []byte
	FailedTests []string
	RunFlag     string
	LogDir      string   // directory to place the logs
	TrialName   string   // name/log file name of current trial
	TrialNames  []string // list of all the trials
}

// Runs records multiple Run objects
type Runs []*Run

// Sort interface
func (rs Runs) Len() int      { return len(rs) }
func (rs Runs) Swap(i, j int) { rs[i], rs[j] = rs[j], rs[i] }
func (rs Runs) Less(i, j int) bool {
	a, b := rs[i], rs[j]
	if a.Backend < b.Backend {
		return true
	} else if a.Backend > b.Backend {
		return false
	}
	if a.Remote < b.Remote {
		return true
	} else if a.Remote > b.Remote {
		return false
	}
	if a.Path < b.Path {
		return true
	} else if a.Path > b.Path {
		return false
	}
	if !a.FastList && b.FastList {
		return true
	} else if a.FastList && !b.FastList {
		return false
	}
	return false
}

// dumpOutput prints the error output
func (r *Run) dumpOutput() {
	log.Println("------------------------------------------------------------")
	log.Printf("---- %q ----", r.CmdString)
	log.Println(string(r.output))
	log.Println("------------------------------------------------------------")
}

// This converts a slice of test names into a regexp which matches
// them.
func testsToRegexp(tests []string) string {
	var split []map[string]struct{}
	// Make a slice with maps of the used parts at each level
	for _, test := range tests {
		for i, name := range strings.Split(test, "/") {
			if i >= len(split) {
				split = append(split, make(map[string]struct{}))
			}
			split[i][name] = struct{}{}
		}
	}
	var out []string
	for _, level := range split {
		var testsInLevel = []string{}
		for name := range level {
			testsInLevel = append(testsInLevel, name)
		}
		sort.Strings(testsInLevel)
		if len(testsInLevel) > 1 {
			out = append(out, "^("+strings.Join(testsInLevel, "|")+")$")
		} else {
			out = append(out, "^"+testsInLevel[0]+"$")
		}
	}
	return strings.Join(out, "/")
}

var failRe = regexp.MustCompile(`(?m)^\s*--- FAIL: (Test.*?) \(`)

// findFailures looks for all the tests which failed
func (r *Run) findFailures() {
	oldFailedTests := r.FailedTests
	r.FailedTests = nil
	excludeParents := map[string]struct{}{}
	ignored := 0
	for _, matches := range failRe.FindAllSubmatch(r.output, -1) {
		failedTest := string(matches[1])
		// Skip any ignored failures
		if _, found := r.Ignore[failedTest]; found {
			ignored++
		} else {
			r.FailedTests = append(r.FailedTests, failedTest)
		}
		// Find all the parents of this test
		parts := strings.Split(failedTest, "/")
		for i := len(parts) - 1; i >= 1; i-- {
			excludeParents[strings.Join(parts[:i], "/")] = struct{}{}
		}
	}
	// Exclude the parents
	var newTests = r.FailedTests[:0]
	for _, failedTest := range r.FailedTests {
		if _, excluded := excludeParents[failedTest]; !excluded {
			newTests = append(newTests, failedTest)
		}
	}
	r.FailedTests = newTests
	if len(r.FailedTests) == 0 && ignored > 0 {
		log.Printf("%q - Found %d ignored errors only - marking as good", r.CmdString, ignored)
		r.err = nil
		r.dumpOutput()
		return
	}
	if len(r.FailedTests) != 0 {
		r.RunFlag = testsToRegexp(r.FailedTests)
	} else {
		r.RunFlag = ""
	}
	if r.passed() && len(r.FailedTests) != 0 {
		log.Printf("%q - Expecting no errors but got: %v", r.CmdString, r.FailedTests)
		r.dumpOutput()
	} else if !r.passed() && len(r.FailedTests) == 0 {
		log.Printf("%q - Expecting errors but got none: %v", r.CmdString, r.FailedTests)
		r.dumpOutput()
		r.FailedTests = oldFailedTests
	}
}

// nextCmdLine returns the next command line
func (r *Run) nextCmdLine() []string {
	CmdLine := r.CmdLine
	if r.RunFlag != "" {
		CmdLine = append(CmdLine, "-test.run", r.RunFlag)
	}
	return CmdLine
}

// trial runs a single test
func (r *Run) trial() {
	CmdLine := r.nextCmdLine()
	CmdString := toShell(CmdLine)
	msg := fmt.Sprintf("%q - Starting (try %d/%d)", CmdString, r.Try, *maxTries)
	log.Println(msg)
	logName := path.Join(r.LogDir, r.TrialName)
	out, err := os.Create(logName)
	if err != nil {
		log.Fatalf("Couldn't create log file: %v", err)
	}
	defer func() {
		err := out.Close()
		if err != nil {
			log.Fatalf("Failed to close log file: %v", err)
		}
	}()
	_, _ = fmt.Fprintln(out, msg)

	// Early exit if --try-run
	if *dryRun {
		log.Printf("Not executing as --dry-run: %v", CmdLine)
		_, _ = fmt.Fprintln(out, "--dry-run is set - not running")
		return
	}

	// Start the test server if required
	finish, err := testserver.Start(r.Remote)
	if err != nil {
		log.Printf("%s: Failed to start test server: %v", r.Remote, err)
		_, _ = fmt.Fprintf(out, "%s: Failed to start test server: %v\n", r.Remote, err)
		r.err = err
		return
	}
	defer finish()

	// Internal buffer
	var b bytes.Buffer
	multiOut := io.MultiWriter(out, &b)

	cmd := exec.Command(CmdLine[0], CmdLine[1:]...)
	cmd.Stderr = multiOut
	cmd.Stdout = multiOut
	cmd.Dir = r.Path
	start := time.Now()
	r.err = cmd.Run()
	r.output = b.Bytes()
	duration := time.Since(start)
	r.findFailures()
	if r.passed() {
		msg = fmt.Sprintf("%q - Finished OK in %v (try %d/%d)", CmdString, duration, r.Try, *maxTries)
	} else {
		msg = fmt.Sprintf("%q - Finished ERROR in %v (try %d/%d): %v: Failed %v", CmdString, duration, r.Try, *maxTries, r.err, r.FailedTests)
	}
	log.Println(msg)
	_, _ = fmt.Fprintln(out, msg)
}

// passed returns true if the test passed
func (r *Run) passed() bool {
	return r.err == nil
}

// GOPATH returns the current GOPATH
func GOPATH() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	return gopath
}

// BinaryName turns a package name into a binary name
func (r *Run) BinaryName() string {
	binary := path.Base(r.Path) + ".test"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	return binary
}

// BinaryPath turns a package name into a binary path
func (r *Run) BinaryPath() string {
	return path.Join(r.Path, r.BinaryName())
}

// PackagePath returns the path to the package
func (r *Run) PackagePath() string {
	return path.Join(GOPATH(), "src", r.Path)
}

// MakeTestBinary makes the binary we will run
func (r *Run) MakeTestBinary() {
	binary := r.BinaryPath()
	binaryName := r.BinaryName()
	log.Printf("%s: Making test binary %q", r.Path, binaryName)
	CmdLine := []string{"go", "test", "-c"}
	if *dryRun {
		log.Printf("Not executing: %v", CmdLine)
		return
	}
	cmd := exec.Command(CmdLine[0], CmdLine[1:]...)
	cmd.Dir = r.Path
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Failed to make test binary: %v", err)
	}
	if _, err := os.Stat(binary); err != nil {
		log.Fatalf("Couldn't find test binary %q", binary)
	}
}

// RemoveTestBinary removes the binary made in makeTestBinary
func (r *Run) RemoveTestBinary() {
	if *dryRun {
		return
	}
	binary := r.BinaryPath()
	err := os.Remove(binary) // Delete the binary when finished
	if err != nil {
		log.Printf("Error removing test binary %q: %v", binary, err)
	}
}

// Name returns the run name as a file name friendly string
func (r *Run) Name() string {
	ns := []string{
		r.Backend,
		strings.ReplaceAll(r.Path, "/", "."),
		r.Remote,
	}
	if r.FastList {
		ns = append(ns, "fastlist")
	}
	ns = append(ns, fmt.Sprintf("%d", r.Try))
	s := strings.Join(ns, "-")
	s = strings.ReplaceAll(s, ":", "")
	return s
}

// Init the Run
func (r *Run) Init() {
	prefix := "-test."
	if r.NoBinary {
		prefix = "-"
		r.CmdLine = []string{"go", "test"}
	} else {
		r.CmdLine = []string{"./" + r.BinaryName()}
	}
	testTimeout := *timeout
	if r.ExtraTime > 0 {
		testTimeout = time.Duration(float64(testTimeout) * r.ExtraTime)
	}
	r.CmdLine = append(r.CmdLine, prefix+"v", prefix+"timeout", testTimeout.String(), "-remote", r.Remote)
	listRetries := *listRetries
	if r.ListRetries > 0 {
		listRetries = r.ListRetries
	}
	if listRetries > 0 {
		r.CmdLine = append(r.CmdLine, "-list-retries", fmt.Sprint(listRetries))
	}
	r.Try = 1
	ci := fs.GetConfig(context.Background())
	if *verbose {
		r.CmdLine = append(r.CmdLine, "-verbose")
		ci.LogLevel = fs.LogLevelDebug
	}
	if *runOnly != "" {
		r.CmdLine = append(r.CmdLine, prefix+"run", *runOnly)
	}
	if r.FastList {
		r.CmdLine = append(r.CmdLine, "-fast-list")
	}
	if r.Short {
		r.CmdLine = append(r.CmdLine, "-short")
	}
	if r.SizeLimit > 0 {
		r.CmdLine = append(r.CmdLine, "-size-limit", strconv.FormatInt(r.SizeLimit, 10))
	}
	r.CmdString = toShell(r.CmdLine)
}

// Logs returns all the log names
func (r *Run) Logs() []string {
	return r.TrialNames
}

// FailedTestsCSV returns the failed tests as a comma separated string, limiting the number
func (r *Run) FailedTestsCSV() string {
	const maxTests = 5
	ts := r.FailedTests
	if len(ts) > maxTests {
		ts = ts[:maxTests:maxTests]
		ts = append(ts, fmt.Sprintf("… (%d more)", len(r.FailedTests)-maxTests))
	}
	return strings.Join(ts, ", ")
}

// Run runs all the trials for this test
func (r *Run) Run(LogDir string, result chan<- *Run) {
	if r.OneOnly {
		oneOnlyMu.Lock()
		mu := oneOnly[r.Backend]
		if mu == nil {
			mu = new(sync.Mutex)
			oneOnly[r.Backend] = mu
		}
		oneOnlyMu.Unlock()
		mu.Lock()
		defer mu.Unlock()
	}
	r.Init()
	r.LogDir = LogDir
	for r.Try = 1; r.Try <= *maxTries; r.Try++ {
		r.TrialName = r.Name() + ".txt"
		r.TrialNames = append(r.TrialNames, r.TrialName)
		log.Printf("Starting run with log %q", r.TrialName)
		r.trial()
		if r.passed() || r.NoRetries {
			break
		}
	}
	if !r.passed() {
		r.dumpOutput()
	}
	result <- r
}
