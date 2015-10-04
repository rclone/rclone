// +build ignore

// Make the test files from fstests.go
package main

import (
	"bufio"
	"html/template"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Search fstests.go and return all the test function names
func findTestFunctions() []string {
	fns := []string{}
	matcher := regexp.MustCompile(`^func\s+(Test.*?)\(`)

	in, err := os.Open("fstests.go")
	if err != nil {
		log.Fatalf("Couldn't open fstests.go: %v", err)
	}
	defer in.Close()

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		matches := matcher.FindStringSubmatch(line)
		if len(matches) > 0 {
			fns = append(fns, matches[1])
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error scanning file: %v", err)
	}
	return fns
}

// Data to substitute
type Data struct {
	Regenerate  string
	FsName      string
	UpperFsName string
	TestName    string
	ObjectName  string
	Fns         []string
}

var testProgram = `
// Test {{ .UpperFsName }} filesystem interface
//
// Automatically generated - DO NOT EDIT
// Regenerate with: {{ .Regenerate }}
package {{ .FsName }}_test

import (
	"testing"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest/fstests"
	"github.com/ncw/rclone/{{ .FsName }}"
)

func init() {
	fstests.NilObject = fs.Object((*{{ .FsName }}.{{ .ObjectName }})(nil))
	fstests.RemoteName = "{{ .TestName }}"
}

// Generic tests for the Fs
{{ range $fn := .Fns }}func {{ $fn }}(t *testing.T){ fstests.{{ $fn }}(t) }
{{ end }}
`

// Generate test file piping it through gofmt
func generateTestProgram(t *template.Template, fns []string, Fsname, ObjectName string) {
	fsname := strings.ToLower(Fsname)
	TestName := "Test" + Fsname + ":"
	outfile := "../../" + fsname + "/" + fsname + "_test.go"

	if fsname == "local" {
		TestName = ""
	}

	data := Data{
		Regenerate:  "make gen_tests",
		FsName:      fsname,
		UpperFsName: Fsname,
		TestName:    TestName,
		ObjectName:  ObjectName,
		Fns:         fns,
	}

	cmd := exec.Command("gofmt")

	log.Printf("Writing %q", outfile)
	out, err := os.Create(outfile)
	if err != nil {
		log.Fatal(err)
	}
	cmd.Stdout = out

	gofmt, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		log.Fatal(err)
	}
	if err = t.Execute(gofmt, data); err != nil {
		log.Fatal(err)
	}
	if err = gofmt.Close(); err != nil {
		log.Fatal(err)
	}
	if err = cmd.Wait(); err != nil {
		log.Fatal(err)
	}
	if err = out.Close(); err != nil {
		log.Fatal(err)
	}
}

func main() {
	fns := findTestFunctions()
	t := template.Must(template.New("main").Parse(testProgram))
	generateTestProgram(t, fns, "Local", "FsObjectLocal")
	generateTestProgram(t, fns, "Swift", "FsObjectSwift")
	generateTestProgram(t, fns, "S3", "FsObjectS3")
	generateTestProgram(t, fns, "Drive", "FsObjectDrive")
	generateTestProgram(t, fns, "GoogleCloudStorage", "FsObjectStorage")
	generateTestProgram(t, fns, "Dropbox", "FsObjectDropbox")
	generateTestProgram(t, fns, "AmazonCloudDrive", "FsObjectAcd")
	generateTestProgram(t, fns, "OneDrive", "Object")
	log.Printf("Done")
}
