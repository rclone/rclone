// +build ignore

// Run tests for all the remotes
//
// Run with go run test_all.go
package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
	_ "github.com/ncw/rclone/fs/all" // import all fs
	"github.com/ncw/rclone/fstest"
)

var (
	remotes = []string{
		"TestAmazonCloudDrive:",
		"TestB2:",
		"TestCryptDrive:",
		"TestCryptSwift:",
		"TestDrive:",
		"TestDropbox:",
		"TestGoogleCloudStorage:",
		"TestHubic:",
		"TestOneDrive:",
		"TestS3:",
		"TestSwift:",
		"TestYandex:",
	}
	binary = "fs.test"
	// Flags
	maxTries = flag.Int("maxtries", 5, "Number of times to try each test")
	runTests = flag.String("remotes", "", "Comma separated list of remotes to test, eg 'TestSwift:,TestS3'")
	verbose  = flag.Bool("verbose", false, "Run the tests with -v")
	clean    = flag.Bool("clean", false, "Instead of testing, clean all left over test directories")
	runOnly  = flag.String("run-only", "", "Run only those tests matching the regexp supplied")
)

// test holds info about a running test
type test struct {
	remote      string
	subdir      bool
	cmdLine     []string
	cmdString   string
	try         int
	err         error
	output      []byte
	failedTests []string
	runFlag     string
}

// newTest creates a new test
func newTest(remote string, subdir bool) *test {
	t := &test{
		remote:  remote,
		subdir:  subdir,
		cmdLine: []string{"./" + binary, "-remote", remote},
		try:     1,
	}
	if *verbose {
		t.cmdLine = append(t.cmdLine, "-test.v")
	}
	if *runOnly != "" {
		t.cmdLine = append(t.cmdLine, "-test.run", *runOnly)
	}
	if subdir {
		t.cmdLine = append(t.cmdLine, "-subdir")
	}
	t.cmdString = strings.Join(t.cmdLine, " ")
	return t
}

// dumpOutput prints the error output
func (t *test) dumpOutput() {
	log.Println("------------------------------------------------------------")
	log.Printf("---- %q ----", t.cmdString)
	log.Println(string(t.output))
	log.Println("------------------------------------------------------------")
}

var failRe = regexp.MustCompile(`(?m)^--- FAIL: (Test\w*) \(`)

// findFailures looks for all the tests which failed
func (t *test) findFailures() {
	oldFailedTests := t.failedTests
	t.failedTests = nil
	for _, matches := range failRe.FindAllSubmatch(t.output, -1) {
		t.failedTests = append(t.failedTests, string(matches[1]))
	}
	if len(t.failedTests) != 0 {
		t.runFlag = "^(" + strings.Join(t.failedTests, "|") + ")$"
	} else {
		t.runFlag = ""
	}
	if t.passed() && len(t.failedTests) != 0 {
		log.Printf("%q - Expecting no errors but got: %v", t.cmdString, t.failedTests)
		t.dumpOutput()
	} else if !t.passed() && len(t.failedTests) == 0 {
		log.Printf("%q - Expecting errors but got none: %v", t.cmdString, t.failedTests)
		t.dumpOutput()
		t.failedTests = oldFailedTests
	}
}

// trial runs a single test
func (t *test) trial() {
	cmdLine := t.cmdLine[:]
	if t.runFlag != "" {
		cmdLine = append(cmdLine, "-test.run", t.runFlag)
	}
	cmdString := strings.Join(cmdLine, " ")
	log.Printf("%q - Starting (try %d/%d)", cmdString, t.try, *maxTries)
	cmd := exec.Command(cmdLine[0], cmdLine[1:]...)
	start := time.Now()
	t.output, t.err = cmd.CombinedOutput()
	duration := time.Since(start)
	t.findFailures()
	if t.passed() {
		log.Printf("%q - Finished OK in %v (try %d/%d)", cmdString, duration, t.try, *maxTries)
	} else {
		log.Printf("%q - Finished ERROR in %v (try %d/%d): %v: Failed %v", cmdString, duration, t.try, *maxTries, t.err, t.failedTests)
	}
}

// cleanFs runs a single clean fs for left over directories
func (t *test) cleanFs() error {
	f, err := fs.NewFs(t.remote)
	if err != nil {
		return err
	}
	dirs, err := fs.NewLister().SetLevel(1).Start(f, "").GetDirs()
	for _, dir := range dirs {
		if fstest.MatchTestRemote.MatchString(dir.Name) {
			log.Printf("Purging %s%s", t.remote, dir.Name)
			dir, err := fs.NewFs(t.remote + dir.Name)
			if err != nil {
				return err
			}
			err = fs.Purge(dir)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// clean runs a single clean on a fs for left over directories
func (t *test) clean() {
	log.Printf("%q - Starting clean (try %d/%d)", t.remote, t.try, *maxTries)
	start := time.Now()
	t.err = t.cleanFs()
	if t.err != nil {
		log.Printf("%q - Failed to purge %v", t.remote, t.err)
	}
	duration := time.Since(start)
	if t.passed() {
		log.Printf("%q - Finished OK in %v (try %d/%d)", t.cmdString, duration, t.try, *maxTries)
	} else {
		log.Printf("%q - Finished ERROR in %v (try %d/%d): %v", t.cmdString, duration, t.try, *maxTries, t.err)
	}
}

// passed returns true if the test passed
func (t *test) passed() bool {
	return t.err == nil
}

// run runs all the trials for this test
func (t *test) run(result chan<- *test) {
	for t.try = 1; t.try <= *maxTries; t.try++ {
		if *clean {
			if !t.subdir {
				t.clean()
			}
		} else {
			t.trial()
		}
		if t.passed() {
			break
		}
	}
	if !t.passed() {
		t.dumpOutput()
	}
	result <- t
}

// makeTestBinary makes the binary we will run
func makeTestBinary() {
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	log.Printf("Making test binary %q", binary)
	err := exec.Command("go", "test", "-c", "-o", binary).Run()
	if err != nil {
		log.Fatalf("Failed to make test binary: %v", err)
	}
	if _, err := os.Stat(binary); err != nil {
		log.Fatalf("Couldn't find test binary %q", binary)
	}
}

// removeTestBinary removes the binary made in makeTestBinary
func removeTestBinary() {
	err := os.Remove(binary) // Delete the binary when finished
	if err != nil {
		log.Printf("Error removing test binary %q: %v", binary, err)
	}
}

func main() {
	flag.Parse()
	if *runTests != "" {
		remotes = strings.Split(*runTests, ",")
	}
	log.Printf("Testing remotes: %s", strings.Join(remotes, ", "))

	start := time.Now()
	if *clean {
		fs.LoadConfig()
	} else {
		makeTestBinary()
		defer removeTestBinary()
	}

	// start the tests
	results := make(chan *test, 8)
	awaiting := 0
	for _, remote := range remotes {
		awaiting += 2
		go newTest(remote, false).run(results)
		go newTest(remote, true).run(results)
	}

	// Wait for the tests to finish
	var failed []*test
	for ; awaiting > 0; awaiting-- {
		t := <-results
		if !t.passed() {
			failed = append(failed, t)
		}
	}
	duration := time.Since(start)

	// Summarise results
	if len(failed) == 0 {
		log.Printf("PASS: All tests finished OK in %v", duration)
	} else {
		log.Printf("FAIL: %d tests failed in %v", len(failed), duration)
		for _, t := range failed {
			log.Printf("  * %s", t.cmdString)
		}
		os.Exit(1)
	}
}
