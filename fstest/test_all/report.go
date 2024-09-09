package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"sort"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/file"
	"github.com/skratchdot/open-golang/open"
)

const timeFormat = "2006-01-02-150405"

// Report holds the info to make a report on a series of test runs
type Report struct {
	LogDir    string        // output directory for logs and report
	StartTime time.Time     // time started
	DateTime  string        // directory name for output
	Duration  time.Duration // time the run took
	Failed    Runs          // failed runs
	Passed    Runs          // passed runs
	Runs      []ReportRun   // runs to report
	Version   string        // rclone version
	Previous  string        // previous test name if known
	IndexHTML string        // path to the index.html file
	URL       string        // online version
	Branch    string        // rclone branch
	Commit    string        // rclone commit
	GOOS      string        // Go OS
	GOARCH    string        // Go Arch
	GoVersion string        // Go Version
}

// ReportRun is used in the templates to report on a test run
type ReportRun struct {
	Name string
	Runs Runs
}

// Parse version numbers
// v1.49.0
// v1.49.0-031-g2298834e-beta
// v1.49.0-032-g20793a5f-sharefile-beta
// match 1 is commit number
// match 2 is branch name
var parseVersion = regexp.MustCompile(`^v(?:[0-9.]+)-(?:\d+)-g([0-9a-f]+)(?:-(.*))?-beta$`)

// FIXME take -issue or -pr parameter...

// NewReport initialises and returns a Report
func NewReport() *Report {
	r := &Report{
		StartTime: time.Now(),
		Version:   fs.Version,
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		GoVersion: runtime.Version(),
	}
	r.DateTime = r.StartTime.Format(timeFormat)

	// Find previous log directory if possible
	names, err := os.ReadDir(*outputDir)
	if err == nil && len(names) > 0 {
		r.Previous = names[len(names)-1].Name()
	}

	// Create output directory for logs and report
	r.LogDir = path.Join(*outputDir, r.DateTime)
	err = file.MkdirAll(r.LogDir, 0777)
	if err != nil {
		fs.Fatalf(nil, "Failed to make log directory: %v", err)
	}

	// Online version
	r.URL = *urlBase + r.DateTime + "/index.html"

	// Get branch/commit out of version
	parts := parseVersion.FindStringSubmatch(r.Version)
	if len(parts) >= 3 {
		r.Commit = parts[1]
		r.Branch = parts[2]
	}
	if r.Branch == "" {
		r.Branch = "master"
	}

	return r
}

// End should be called when the tests are complete
func (r *Report) End() {
	r.Duration = time.Since(r.StartTime)
	sort.Sort(r.Failed)
	sort.Sort(r.Passed)
	r.Runs = []ReportRun{
		{Name: "Failed", Runs: r.Failed},
		{Name: "Passed", Runs: r.Passed},
	}
}

// AllPassed returns true if there were no failed tests
func (r *Report) AllPassed() bool {
	return len(r.Failed) == 0
}

// RecordResult should be called with a Run when it has finished to be
// recorded into the Report
func (r *Report) RecordResult(t *Run) {
	if !t.passed() {
		r.Failed = append(r.Failed, t)
	} else {
		r.Passed = append(r.Passed, t)
	}
}

// Title returns a human-readable summary title for the Report
func (r *Report) Title() string {
	if r.AllPassed() {
		return fmt.Sprintf("PASS: All tests finished OK in %v", r.Duration)
	}
	return fmt.Sprintf("FAIL: %d tests failed in %v", len(r.Failed), r.Duration)
}

// LogSummary writes the summary to the log file
func (r *Report) LogSummary() {
	fs.Logf(nil, "Logs in %q", r.LogDir)

	// Summarise results
	fs.Logf(nil, "SUMMARY")
	fs.Log(nil, r.Title())
	if !r.AllPassed() {
		for _, t := range r.Failed {
			fs.Logf(nil, "  * %s", toShell(t.nextCmdLine()))
			fs.Logf(nil, "    * Failed tests: %v", t.FailedTests)
		}
	}
}

// LogJSON writes the summary to index.json in LogDir
func (r *Report) LogJSON() {
	out, err := json.MarshalIndent(r, "", "\t")
	if err != nil {
		fs.Fatalf(nil, "Failed to marshal data for index.json: %v", err)
	}
	err = os.WriteFile(path.Join(r.LogDir, "index.json"), out, 0666)
	if err != nil {
		fs.Fatalf(nil, "Failed to write index.json: %v", err)
	}
}

// LogHTML writes the summary to index.html in LogDir
func (r *Report) LogHTML() {
	r.IndexHTML = path.Join(r.LogDir, "index.html")
	out, err := os.Create(r.IndexHTML)
	if err != nil {
		fs.Fatalf(nil, "Failed to open index.html: %v", err)
	}
	defer func() {
		err := out.Close()
		if err != nil {
			fs.Fatalf(nil, "Failed to close index.html: %v", err)
		}
	}()
	err = reportTemplate.Execute(out, r)
	if err != nil {
		fs.Fatalf(nil, "Failed to execute template: %v", err)
	}
	_ = open.Start("file://" + r.IndexHTML)
}

var reportHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>{{ .Title }}</title>
<style>
table {
	border-collapse: collapse;
	border-spacing: 0;
	border: 1px solid #ddd;
}
table.tests {
	width: 100%;
}
table, th, td {
	border: 1px solid #264653;
}
.Failed {
	color: #BE5B43;
}
.Passed {
	color: #17564E;
}
.false {
	font-weight: lighter;
}
.true {
	font-weight: bold;
}

th, td {
	text-align: left;
	padding: 4px;
}

tr:nth-child(even) {
    background-color: #f2f2f2;
}

a {
	color: #5B1955;
	text-decoration: none;
}
a:hover, a:focus {
	color: #F4A261;
	text-decoration:underline;
}
a:focus {
	outline: thin dotted;
	outline: 5px auto;
}
</style>
</head>
<body>
<h1>{{ .Title }}</h1>

<table>
<tr><th>Version</th><td>{{ .Version }}</td></tr>
<tr><th>Test</th><td><a href="{{ .URL }}">{{ .DateTime}}</a></td></tr>
<tr><th>Branch</th><td><a href="https://github.com/rclone/rclone/tree/{{ .Branch }}">{{ .Branch }}</a></td></tr>
{{ if .Commit}}<tr><th>Commit</th><td><a href="https://github.com/rclone/rclone/commit/{{ .Commit }}">{{ .Commit }}</a></td></tr>{{ end }}
<tr><th>Go</th><td>{{ .GoVersion }} {{ .GOOS }}/{{ .GOARCH }}</td></tr>
<tr><th>Duration</th><td>{{ .Duration }}</td></tr>
{{ if .Previous}}<tr><th>Previous</th><td><a href="../{{ .Previous }}/index.html">{{ .Previous }}</a></td></tr>{{ end }}
<tr><th>Up</th><td><a href="../">Older Tests</a></td></tr>
</table>

{{ range .Runs }}
{{ if .Runs }}
<h2 class="{{ .Name }}">{{ .Name }}: {{ len .Runs }}</h2>
<table class="{{ .Name }} tests">
<tr>
<th>Backend</th>
<th>Remote</th>
<th>Test</th>
<th>FastList</th>
<th>Failed</th>
<th>Logs</th>
</tr>
{{ $prevBackend := "" }}
{{ $prevRemote := "" }}
{{ range .Runs}}
<tr>
<td>{{ if ne $prevBackend .Backend }}{{ .Backend }}{{ end }}{{ $prevBackend = .Backend }}</td>
<td>{{ if ne $prevRemote .Remote }}{{ .Remote }}{{ end }}{{ $prevRemote = .Remote }}</td>
<td>{{ .Path }}</td>
<td><span class="{{ .FastList }}">{{ .FastList }}</span></td>
<td>{{ .FailedTestsCSV }}</td>
<td>{{ range $i, $v := .Logs }}<a href="{{ $v }}">#{{ $i }}</a> {{ end }}</td>
</tr>
{{ end }}
</table>
{{ end }}
{{ end }}
</body>
</html>
`

var reportTemplate = template.Must(template.New("Report").Parse(reportHTML))

// EmailHTML sends the summary report to the email address supplied
func (r *Report) EmailHTML() {
	if *emailReport == "" || r.IndexHTML == "" {
		return
	}
	fs.Logf(nil, "Sending email summary to %q", *emailReport)
	cmdLine := []string{"mail", "-a", "Content-Type: text/html", *emailReport, "-s", "rclone integration tests: " + r.Title()}
	cmd := exec.Command(cmdLine[0], cmdLine[1:]...)
	in, err := os.Open(r.IndexHTML)
	if err != nil {
		fs.Fatalf(nil, "Failed to open index.html: %v", err)
	}
	cmd.Stdin = in
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fs.Fatalf(nil, "Failed to send email: %v", err)
	}
	_ = in.Close()
}

// uploadTo uploads a copy of the report online to the dir given
func (r *Report) uploadTo(uploadDir string) {
	dst := path.Join(*uploadPath, uploadDir)
	fs.Logf(nil, "Uploading results to %q", dst)
	cmdLine := []string{"rclone", "sync", "--stats-log-level", "NOTICE", r.LogDir, dst}
	cmd := exec.Command(cmdLine[0], cmdLine[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fs.Fatalf(nil, "Failed to upload results: %v", err)
	}
}

// Upload uploads a copy of the report online
func (r *Report) Upload() {
	if *uploadPath == "" || r.IndexHTML == "" {
		return
	}
	// Upload into dated directory
	r.uploadTo(r.DateTime)
	// And again into current
	r.uploadTo("current")
}
