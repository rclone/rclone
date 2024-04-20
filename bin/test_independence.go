//go:build ignore

// Test that the tests in the suite passed in are independent

package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"regexp"
)

var matchLine = regexp.MustCompile(`(?m)^=== RUN\s*(TestIntegration/\S*)\s*$`)

// run the test pass in and grep out the test names
func findTests(packageToTest string) (tests []string) {
	cmd := exec.Command("go", "test", "-v", packageToTest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_, _ = os.Stderr.Write(out)
		log.Fatal(err)
	}
	results := matchLine.FindAllSubmatch(out, -1)
	if results == nil {
		log.Fatal("No tests found")
	}
	for _, line := range results {
		tests = append(tests, string(line[1]))
	}
	return tests
}

// run the test passed in with the -run passed in
func runTest(packageToTest string, testName string) {
	cmd := exec.Command("go", "test", "-v", packageToTest, "-run", "^"+testName+"$")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("%s FAILED ------------------", testName)
		_, _ = os.Stderr.Write(out)
		log.Printf("%s FAILED ------------------", testName)
	} else {
		log.Printf("%s OK", testName)
	}
}
func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Fatalf("Syntax: %s <test_to_run>", os.Args[0])
	}
	packageToTest := args[0]
	testNames := findTests(packageToTest)
	// fmt.Printf("%s\n", testNames)
	for _, testName := range testNames {
		runTest(packageToTest, testName)
	}
}
