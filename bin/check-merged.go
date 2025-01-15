//go:build ignore

// Attempt to work out if branches have already been merged
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
)

var (
	// Flags
	master  = flag.String("master", "master", "Branch to work out if merged into")
	version = "development version" // overridden by goreleaser
)

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: %s [options]
Version: %s

Attempt to work out if in the current git repo branches have been
merged into master.

Example usage:

    %s

Full options:
`, os.Args[0], version, os.Args[0])
	flag.PrintDefaults()
}

var (
	printedSep = false
)

const (
	sep1 = "============================================================"
	sep2 = "------------------------------------------------------------"
)

// Show the diff between two git revisions
func gitDiffDiff(rev1, rev2 string) {
	fmt.Printf("Diff of diffs of %q and %q\n", rev1, rev2)
	cmd := exec.Command("bash", "-c", fmt.Sprintf(`diff <(git show "%s")  <(git show "%s")`, rev1, rev2))
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			// OK just different
		} else {
			log.Fatalf("git diff failed: %#v", err)
		}
	}
	_, _ = os.Stdout.Write(out)
}

var reCommit = regexp.MustCompile(`commit ([0-9a-f]{32,})`)

// Grep the git log for logLine
func gitLogGrep(branch, rev, logLine string) {
	cmd := exec.Command("git", "log", "--grep", regexp.QuoteMeta(logLine), *master)
	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("git log grep failed: %v", err)
	}
	if len(out) > 0 {
		if !printedSep {
			fmt.Println(sep1)
			printedSep = true
		}
		fmt.Printf("Branch: %s - MAY BE MERGED to %s\nLog: %s\n\n", branch, *master, logLine)
		_, _ = os.Stdout.Write(out)
		match := reCommit.FindSubmatch(out)
		if len(match) != 0 {
			commit := string(match[1])
			fmt.Println(sep2)
			gitDiffDiff(rev, commit)
		}
		fmt.Println(sep1)
	}
}

// * b2-fix-download-url      4209c768a [gone] b2: fix transfers when using download_url
var reLine = regexp.MustCompile(`^[ *] (\S+)\s+([0-9a-f]+)\s+(?:\[[^]]+\] )?(.*)$`)

// Run git branch -v, parse the output and check it in the log
func gitBranch() {
	cmd := exec.Command("git", "branch", "-v")
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("git branch pipe failed: %v", err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatalf("git branch failed: %v", err)
	}
	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		line := scanner.Text()
		match := reLine.FindStringSubmatch(line)
		if len(match) != 4 {
			log.Printf("Invalid line %q", line)
			continue
		}
		branch, rev, logLine := match[1], match[2], match[3]
		if branch == *master {
			continue
		}
		//fmt.Printf("branch = %q, rev = %q, logLine = %q\n", branch, rev, logLine)
		gitLogGrep(branch, rev, logLine)
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("failed reading git branch: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		log.Fatalf("git branch wait failed: %v", err)
	}
}

func main() {
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) != 0 {
		usage()
		log.Fatal("Wrong number of arguments")
	}
	gitBranch()
}
