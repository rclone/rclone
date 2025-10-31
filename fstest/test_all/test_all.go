// Run tests for all the remotes.  Run this with package names which
// need integration testing.
//
// See the `test` target in the Makefile.
package main

/* FIXME

Make TesTrun have a []string of flags to try - that then makes it generic

*/

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"strings"
	"time"

	_ "github.com/rclone/rclone/backend/all" // import all fs
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fstest/runs"
	"github.com/rclone/rclone/lib/pacer"
)

func init() {
	// Flags
	flag.IntVar(&Opt.MaxTries, "maxtries", 5, "Number of times to try each test")
	flag.IntVar(&Opt.MaxN, "n", 20, "Maximum number of tests to run at once")
	flag.StringVar(&Opt.TestRemotes, "remotes", "", "Comma separated list of remotes to test, e.g. 'TestSwift:,TestS3'")
	flag.StringVar(&Opt.TestBackends, "backends", "", "Comma separated list of backends to test, e.g. 's3,googlecloudstorage")
	flag.StringVar(&Opt.TestTests, "tests", "", "Comma separated list of tests to test, e.g. 'fs/sync,fs/operations'")
	flag.BoolVar(&Opt.Clean, "clean", false, "Instead of testing, clean all left over test directories")
	flag.StringVar(&Opt.RunOnly, "run", "", "Run only those tests matching the regexp supplied")
	flag.DurationVar(&Opt.Timeout, "timeout", 60*time.Minute, "Maximum time to run each test for before giving up")
	flag.BoolVar(&Opt.Race, "race", false, "If set run the tests under the race detector")
	flag.StringVar(&Opt.ConfigFile, "config", "fstest/test_all/config.yaml", "Path to config file")
	flag.StringVar(&Opt.OutputDir, "output", path.Join(os.TempDir(), "rclone-integration-tests"), "Place to store results")
	flag.StringVar(&Opt.EmailReport, "email", "", "Set to email the report to the address supplied")
	flag.BoolVar(&Opt.DryRun, "dry-run", false, "Print commands which would be executed only")
	flag.StringVar(&Opt.URLBase, "url-base", "https://pub.rclone.org/integration-tests/", "Base for the online version")
	flag.StringVar(&Opt.UploadPath, "upload", "", "Set this to an rclone path to upload the results here")
	flag.BoolVar(&Opt.Verbose, "verbose", false, "Set to enable verbose logging in the tests")
	flag.IntVar(&Opt.ListRetries, "list-retries", -1, "Number or times to retry listing - set to override the default")
}

var Opt = &runs.RunOpt{}

func main() {
	flag.Parse()
	conf, err := runs.NewConfig(Opt.ConfigFile)
	if err != nil {
		fs.Log(nil, "test_all should be run from the root of the rclone source code")
		fs.Fatal(nil, fmt.Sprint(err))
	}
	configfile.Install()

	// Seed the random number generator
	randInstance := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))

	// Filter selection
	if Opt.TestRemotes != "" {
		// CSV parse to support connection string remotes with commas like -remotes local,\"TestGoogleCloudStorage,directory_markers:\"
		r := csv.NewReader(strings.NewReader(Opt.TestRemotes))
		remotes, err := r.Read()
		if err != nil {
			fs.Fatalf(Opt.TestRemotes, "error CSV-parsing -remotes string: %v", err)
		}
		fs.Debugf(Opt.TestRemotes, "using remotes: %v", remotes)
		conf.FilterBackendsByRemotes(remotes)
	}
	if Opt.TestBackends != "" {
		conf.FilterBackendsByBackends(strings.Split(Opt.TestBackends, ","))
	}
	if Opt.TestTests != "" {
		conf.FilterTests(strings.Split(Opt.TestTests, ","))
	}

	// Just clean the directories if required
	if Opt.Clean {
		err := cleanRemotes(conf, *Opt)
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
	testRuns := conf.MakeRuns()
	randInstance.Shuffle(len(testRuns), testRuns.Swap)

	// Create Report
	report := runs.NewReport(*Opt)

	// Make the test binaries, one per Path found in the tests
	done := map[string]struct{}{}
	for _, run := range testRuns {
		if _, found := done[run.Path]; !found {
			done[run.Path] = struct{}{}
			if !run.NoBinary {
				run.MakeTestBinary(*Opt)
				defer run.RemoveTestBinary(*Opt)
			}
		}
	}

	// workaround for cache backend as we run simultaneous tests
	_ = os.Setenv("RCLONE_CACHE_DB_WAIT_TIME", "30m")

	// start the tests
	results := make(chan *runs.Run, len(testRuns))
	awaiting := 0
	tokens := pacer.NewTokenDispenser(Opt.MaxN)
	for _, run := range testRuns {
		tokens.Get()
		go func(run *runs.Run) {
			defer tokens.Put()
			run.Run(*Opt, report.LogDir, results)
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
	report.EmailHTML(*Opt)
	report.Upload(*Opt)
	if !report.AllPassed() {
		os.Exit(1)
	}
}
