// Run tests for all the remotes.  Run this with package names which
// need integration testing.
//
// See the `test` target in the Makefile.
package main

/* FIXME

Make TesTrun have a []string of flags to try - that then makes it generic

*/

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	_ "github.com/rclone/rclone/backend/all" // import all fs
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/lib/pacer"
)

var (
	// Flags
	maxTries     = flag.Int("maxtries", 5, "Number of times to try each test")
	maxN         = flag.Int("n", 20, "Maximum number of tests to run at once")
	testRemotes  = flag.String("remotes", "", "Comma separated list of remotes to test, e.g. 'TestSwift:,TestS3'")
	testBackends = flag.String("backends", "", "Comma separated list of backends to test, e.g. 's3,googlecloudstorage")
	testTests    = flag.String("tests", "", "Comma separated list of tests to test, e.g. 'fs/sync,fs/operations'")
	clean        = flag.Bool("clean", false, "Instead of testing, clean all left over test directories")
	runOnly      = flag.String("run", "", "Run only those tests matching the regexp supplied")
	timeout      = flag.Duration("timeout", 60*time.Minute, "Maximum time to run each test for before giving up")
	race         = flag.Bool("race", false, "If set run the tests under the race detector")
	configFile   = flag.String("config", "fstest/test_all/config.yaml", "Path to config file")
	outputDir    = flag.String("output", path.Join(os.TempDir(), "rclone-integration-tests"), "Place to store results")
	emailReport  = flag.String("email", "", "Set to email the report to the address supplied")
	dryRun       = flag.Bool("dry-run", false, "Print commands which would be executed only")
	urlBase      = flag.String("url-base", "https://pub.rclone.org/integration-tests/", "Base for the online version")
	uploadPath   = flag.String("upload", "", "Set this to an rclone path to upload the results here")
	verbose      = flag.Bool("verbose", false, "Set to enable verbose logging in the tests")
	listRetries  = flag.Int("list-retries", -1, "Number or times to retry listing - set to override the default")
)

// if matches then is definitely OK in the shell
var shellOK = regexp.MustCompile("^[A-Za-z0-9./_:-]+$")

// converts an argv style input into a shell command
func toShell(args []string) (result string) {
	for _, arg := range args {
		if result != "" {
			result += " "
		}
		if shellOK.MatchString(arg) {
			result += arg
		} else {
			result += "'" + arg + "'"
		}
	}
	return result
}

func main() {
	flag.Parse()
	conf, err := NewConfig(*configFile)
	if err != nil {
		fs.Log(nil, "test_all should be run from the root of the rclone source code")
		fs.Fatal(nil, fmt.Sprint(err))
	}
	configfile.Install()

	// Seed the random number generator
	randInstance := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))

	// Filter selection
	if *testRemotes != "" {
		conf.filterBackendsByRemotes(strings.Split(*testRemotes, ","))
	}
	if *testBackends != "" {
		conf.filterBackendsByBackends(strings.Split(*testBackends, ","))
	}
	if *testTests != "" {
		conf.filterTests(strings.Split(*testTests, ","))
	}

	// Just clean the directories if required
	if *clean {
		err := cleanRemotes(conf)
		if err != nil {
			fs.Fatalf(nil, "Failed to clean: %v", err)
		}
		return
	}

	var names []string
	for _, remote := range conf.Backends {
		names = append(names, remote.Remote)
	}
	fs.Logf(nil, "Testing remotes: %s", strings.Join(names, ", "))

	// Runs we will do for this test in random order
	runs := conf.MakeRuns()
	randInstance.Shuffle(len(runs), runs.Swap)

	// Create Report
	report := NewReport()

	// Make the test binaries, one per Path found in the tests
	done := map[string]struct{}{}
	for _, run := range runs {
		if _, found := done[run.Path]; !found {
			done[run.Path] = struct{}{}
			if !run.NoBinary {
				run.MakeTestBinary()
				defer run.RemoveTestBinary()
			}
		}
	}

	// workaround for cache backend as we run simultaneous tests
	_ = os.Setenv("RCLONE_CACHE_DB_WAIT_TIME", "30m")

	// start the tests
	results := make(chan *Run, len(runs))
	awaiting := 0
	tokens := pacer.NewTokenDispenser(*maxN)
	for _, run := range runs {
		tokens.Get()
		go func(run *Run) {
			defer tokens.Put()
			run.Run(report.LogDir, results)
		}(run)
		awaiting++
	}

	// Wait for the tests to finish
	for ; awaiting > 0; awaiting-- {
		t := <-results
		report.RecordResult(t)
	}

	// Log and exit
	report.End()
	report.LogSummary()
	report.LogJSON()
	report.LogHTML()
	report.EmailHTML()
	report.Upload()
	if !report.AllPassed() {
		os.Exit(1)
	}
}
