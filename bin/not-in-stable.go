// This shows the commits not yet in the stable branch
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"

	"github.com/coreos/go-semver/semver"
)

// version=$(sed <VERSION -e 's/\.[0-9]+*$//g')
// echo "Checking version ${version}"
// echo
//
// git log --oneline ${version}.0..${version}-stable | cut -c11- | sort > /tmp/in-stable
// git log --oneline ${version}.0..master  | cut -c11- | sort > /tmp/in-master
//
// comm -23 /tmp/in-master /tmp/in-stable

var logRe = regexp.MustCompile(`^([0-9a-f]{4,}) (.*)$`)

// run the test passed in with the -run passed in
func readCommits(from, to string) (logMap map[string]string, logs []string) {
	cmd := exec.Command("git", "log", "--oneline", from+".."+to)
	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("failed to run git log %s: %v", from+".."+to, err) //nolint:gocritic // Don't include gocritic when running golangci-lint to avoid ruleguard suggesting fs. intead of log.
	}
	logMap = map[string]string{}
	logs = []string{}
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		match := logRe.FindSubmatch(line)
		if match == nil {
			log.Fatalf("failed to parse line: %q", line) //nolint:gocritic // Don't include gocritic when running golangci-lint to avoid ruleguard suggesting fs. intead of log.
		}
		var hash, logMessage = string(match[1]), string(match[2])
		logMap[logMessage] = hash
		logs = append(logs, logMessage)
	}
	return logMap, logs
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 0 {
		log.Fatalf("Syntax: %s", os.Args[0]) //nolint:gocritic // Don't include gocritic when running golangci-lint to avoid ruleguard suggesting fs. intead of log.
	}
	// v1.54.0
	versionBytes, err := os.ReadFile("VERSION")
	if err != nil {
		log.Fatalf("Failed to read version: %v", err) //nolint:gocritic // Don't include gocritic when running golangci-lint to avoid ruleguard suggesting fs. intead of log.
	}
	if versionBytes[0] == 'v' {
		versionBytes = versionBytes[1:]
	}
	versionBytes = bytes.TrimSpace(versionBytes)
	semver := semver.New(string(versionBytes))
	stable := fmt.Sprintf("v%d.%d", semver.Major, semver.Minor-1)
	log.Printf("Finding commits in %v not in stable %s", semver, stable) //nolint:gocritic // Don't include gocritic when running golangci-lint to avoid ruleguard suggesting fs. intead of log.
	masterMap, masterLogs := readCommits(stable+".0", "master")
	stableMap, _ := readCommits(stable+".0", stable+"-stable")
	for _, logMessage := range masterLogs {
		// Commit found in stable already
		if _, found := stableMap[logMessage]; found {
			continue
		}
		hash := masterMap[logMessage]
		fmt.Printf("%s %s\n", hash, logMessage)
	}
}
