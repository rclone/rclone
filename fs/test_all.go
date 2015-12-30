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
	"runtime"
	"strings"
	"time"
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
	}
	binary = "fs.test"
	// Flags
	maxTries = flag.Int("maxtries", 3, "Number of times to try each test")
	runTests = flag.String("run", "", "Comma separated list of remotes to test, eg 'TestSwift:,TestS3'")
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
		cmdLine: []string{"./" + binary, "-test.v", "-remote", remote},
		try:     1,
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

// passed returns true if the test passed
func (t *test) passed() bool {
	return t.err == nil
}

// run runs all the trials for this test
func (t *test) run(result chan<- *test) {
	for try := 1; try <= *maxTries; try++ {
		t.trial()
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
	makeTestBinary()
	defer removeTestBinary()

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
