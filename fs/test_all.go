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
)

var (
	remotes = []string{
		"TestSwift:",
		"TestS3:",
		"TestDrive:",
		"TestGoogleCloudStorage:",
		"TestDropbox:",
		"TestAmazonCloudDrive:",
		"TestOneDrive:",
		"TestHubic:",
		"TestB2:",
		"TestYandex:",
	}
	binary = "fs.test"
	// Flags
	maxTries = flag.Int("maxtries", 3, "Number of times to try each test")
	runTests = flag.String("run", "", "Comma separated list of remotes to test, eg 'TestSwift:,TestS3'")
	verbose  = flag.Bool("verbose", false, "Run the tests with -v")
	clean    = flag.Bool("clean", false, "Instead of testing, clean all left over test directories")
)

// test holds info about a running test
type test struct {
	remote    string
	subdir    bool
	cmdLine   []string
	cmdString string
	try       int
	err       error
	output    []byte
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
	if subdir {
		t.cmdLine = append(t.cmdLine, "-subdir")
	}
	t.cmdString = strings.Join(t.cmdLine, " ")
	return t
}

// trial runs a single test
func (t *test) trial() {
	log.Printf("%q - Starting (try %d/%d)", t.cmdString, t.try, *maxTries)
	cmd := exec.Command(t.cmdLine[0], t.cmdLine[1:]...)
	start := time.Now()
	t.output, t.err = cmd.CombinedOutput()
	duration := time.Since(start)
	if t.passed() {
		log.Printf("%q - Finished OK in %v (try %d/%d)", t.cmdString, duration, t.try, *maxTries)
	} else {
		log.Printf("%q - Finished ERROR in %v (try %d/%d): %v", t.cmdString, duration, t.try, *maxTries, t.err)
	}
}

var (
	// matchTestRemote matches the remote names used for testing
	matchTestRemote = regexp.MustCompile(`^[abcdefghijklmnopqrstuvwxyz0123456789]{32}$`)
	// findInteriorDigits makes sure there are digits inside the string
	findInteriorDigits = regexp.MustCompile(`[a-z][0-9]+[a-z]`)
)

// cleanFs runs a single clean fs for left over directories
func (t *test) cleanFs() error {
	f, err := fs.NewFs(t.remote)
	if err != nil {
		return err
	}
	for dir := range f.ListDir() {
		insideDigits := len(findInteriorDigits.FindAllString(dir.Name, -1))
		if matchTestRemote.MatchString(dir.Name) && insideDigits >= 2 {
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
		log.Println("------------------------------------------------------------")
		log.Println(string(t.output))
		log.Println("------------------------------------------------------------")
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
