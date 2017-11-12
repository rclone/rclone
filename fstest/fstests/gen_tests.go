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
	Regenerate      string
	FsName          string
	UpperFsName     string
	TestName        string
	Fns             []string
	Suffix          string
	BuildConstraint string
}

var testProgram = `
// Test {{ .UpperFsName }} filesystem interface
//
// Automatically generated - DO NOT EDIT
// Regenerate with: {{ .Regenerate }}{{ if ne .BuildConstraint "" }}

// +build {{ .BuildConstraint }}
{{end}}
package {{ .FsName }}_test

import (
	"testing"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest/fstests"
	"github.com/ncw/rclone/{{ .FsName }}"
{{ if (or (eq .FsName "crypt") (eq .FsName "cache")) }}	_ "github.com/ncw/rclone/local"
{{end}})

func TestSetup{{ .Suffix }}(t *testing.T)() {
	fstests.NilObject = fs.Object((*{{ .FsName }}.Object)(nil))
	fstests.RemoteName = "{{ .TestName }}"
}

// Generic tests for the Fs
{{ range $fn := .Fns }}func {{ $fn }}{{ $.Suffix }}(t *testing.T){ fstests.{{ $fn }}(t) }
{{ end }}
`

// options for generateTestProgram
type (
	suffix          string
	buildConstraint string
)

// Generate test file piping it through gofmt
func generateTestProgram(t *template.Template, fns []string, Fsname string, options ...interface{}) {
	data := Data{
		Regenerate:  "make gen_tests",
		FsName:      strings.ToLower(Fsname),
		UpperFsName: Fsname,
		Fns:         fns,
	}

	for _, option := range options {
		switch x := option.(type) {
		case suffix:
			data.Suffix = string(x)
		case buildConstraint:
			data.BuildConstraint = string(x)
		default:
			log.Fatalf("Unknown option type %T", option)
		}
	}

	data.TestName = "Test" + data.UpperFsName + data.Suffix + ":"
	outfile := "../../" + data.FsName + "/" + data.FsName + data.Suffix + "_test.go"

	if data.FsName == "local" {
		data.TestName = ""
	}

	cmd := exec.Command("gofmt")

	log.Printf("Writing %q", outfile)
	out, err := os.Create(outfile)
	if err != nil {
		log.Fatalf("Failed to write %q: %v", outfile, err)
	}
	cmd.Stdout = out

	gofmt, err := cmd.StdinPipe()
	if err != nil {
		log.Fatalf("Failed to StdinPipe %q: %v", outfile, err)
	}
	if err = cmd.Start(); err != nil {
		log.Fatalf("Failed to Start %q: %v", outfile, err)
	}
	if err = t.Execute(gofmt, data); err != nil {
		log.Fatalf("Failed to Execute %q: %v", outfile, err)
	}
	if err = gofmt.Close(); err != nil {
		log.Fatalf("Failed to Close gofmt on %q: %v", outfile, err)
	}
	if err = cmd.Wait(); err != nil {
		log.Fatalf("Failed to Wait %q: %v", outfile, err)
	}
	if err = out.Close(); err != nil {
		log.Fatalf("Failed to Close out %q: %v", outfile, err)
	}
}

func main() {
	fns := findTestFunctions()
	t := template.Must(template.New("main").Parse(testProgram))
	generateTestProgram(t, fns, "Local")
	generateTestProgram(t, fns, "Swift")
	generateTestProgram(t, fns, "S3")
	generateTestProgram(t, fns, "Drive")
	generateTestProgram(t, fns, "GoogleCloudStorage")
	generateTestProgram(t, fns, "Dropbox")
	generateTestProgram(t, fns, "AmazonCloudDrive")
	generateTestProgram(t, fns, "OneDrive")
	generateTestProgram(t, fns, "Hubic")
	generateTestProgram(t, fns, "B2")
	generateTestProgram(t, fns, "Yandex")
	generateTestProgram(t, fns, "Crypt")
	generateTestProgram(t, fns, "Crypt", suffix("2"))
	generateTestProgram(t, fns, "Crypt", suffix("3"))
	generateTestProgram(t, fns, "Sftp")
	generateTestProgram(t, fns, "FTP")
	generateTestProgram(t, fns, "Box")
	generateTestProgram(t, fns, "QingStor", buildConstraint("!plan9"))
	generateTestProgram(t, fns, "AzureBlob", buildConstraint("go1.7"))
	generateTestProgram(t, fns, "Pcloud")
	generateTestProgram(t, fns, "Webdav")
	generateTestProgram(t, fns, "Cache", buildConstraint("!plan9"))
	log.Printf("Done")
}
