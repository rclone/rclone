package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/shlex"
	kingpin "gopkg.in/alecthomas/kingpin.v3-unstable"
)

type Vars map[string]string

func (v Vars) Copy() Vars {
	out := Vars{}
	for k, v := range v {
		out[k] = v
	}
	return out
}

func (v Vars) Replace(s string) string {
	for k, v := range v {
		prefix := regexp.MustCompile(fmt.Sprintf("{%s=([^}]*)}", k))
		if v != "" {
			s = prefix.ReplaceAllString(s, "$1")
		} else {
			s = prefix.ReplaceAllString(s, "")
		}
		s = strings.Replace(s, fmt.Sprintf("{%s}", k), v, -1)
	}
	return s
}

type linterState struct {
	*Linter
	issues   chan *Issue
	vars     Vars
	exclude  *regexp.Regexp
	include  *regexp.Regexp
	deadline <-chan time.Time
}

func (l *linterState) Partitions(paths []string) ([][]string, error) {
	cmdArgs, err := parseCommand(l.command())
	if err != nil {
		return nil, err
	}
	parts, err := l.Linter.PartitionStrategy(cmdArgs, paths)
	if err != nil {
		return nil, err
	}
	return parts, nil
}

func (l *linterState) command() string {
	return l.vars.Replace(l.Command)
}

func runLinters(linters map[string]*Linter, paths []string, concurrency int, exclude, include *regexp.Regexp) (chan *Issue, chan error) {
	errch := make(chan error, len(linters))
	concurrencych := make(chan bool, concurrency)
	incomingIssues := make(chan *Issue, 1000000)

	directiveParser := newDirectiveParser()
	if config.WarnUnmatchedDirective {
		directiveParser.LoadFiles(paths)
	}

	processedIssues := maybeSortIssues(filterIssuesViaDirectives(
		directiveParser, maybeAggregateIssues(incomingIssues)))

	vars := Vars{
		"duplthreshold":    fmt.Sprintf("%d", config.DuplThreshold),
		"mincyclo":         fmt.Sprintf("%d", config.Cyclo),
		"maxlinelength":    fmt.Sprintf("%d", config.LineLength),
		"misspelllocale":   fmt.Sprintf("%s", config.MisspellLocale),
		"min_confidence":   fmt.Sprintf("%f", config.MinConfidence),
		"min_occurrences":  fmt.Sprintf("%d", config.MinOccurrences),
		"min_const_length": fmt.Sprintf("%d", config.MinConstLength),
		"tests":            "",
		"not_tests":        "true",
	}
	if config.Test {
		vars["tests"] = "true"
		vars["not_tests"] = ""
	}

	wg := &sync.WaitGroup{}
	id := 1
	for _, linter := range linters {
		deadline := time.After(config.Deadline.Duration())
		state := &linterState{
			Linter:   linter,
			issues:   incomingIssues,
			vars:     vars,
			exclude:  exclude,
			include:  include,
			deadline: deadline,
		}

		partitions, err := state.Partitions(paths)
		if err != nil {
			errch <- err
			continue
		}
		for _, args := range partitions {
			wg.Add(1)
			concurrencych <- true
			// Call the goroutine with a copy of the args array so that the
			// contents of the array are not modified by the next iteration of
			// the above for loop
			go func(id int, args []string) {
				err := executeLinter(id, state, args)
				if err != nil {
					errch <- err
				}
				<-concurrencych
				wg.Done()
			}(id, args)
			id++
		}
	}

	go func() {
		wg.Wait()
		close(incomingIssues)
		close(errch)
	}()
	return processedIssues, errch
}

func executeLinter(id int, state *linterState, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing linter command")
	}

	start := time.Now()
	dbg := namespacedDebug(fmt.Sprintf("[%s.%d]: ", state.Name, id))
	dbg("executing %s", strings.Join(args, " "))
	buf := bytes.NewBuffer(nil)
	command := args[0]
	cmd := exec.Command(command, args[1:]...) // nolint: gosec
	cmd.Stdout = buf
	cmd.Stderr = buf
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to execute linter %s: %s", command, err)
	}

	done := make(chan bool)
	go func() {
		err = cmd.Wait()
		done <- true
	}()

	// Wait for process to complete or deadline to expire.
	select {
	case <-done:

	case <-state.deadline:
		err = fmt.Errorf("deadline exceeded by linter %s (try increasing --deadline)",
			state.Name)
		kerr := cmd.Process.Kill()
		if kerr != nil {
			warning("failed to kill %s: %s", state.Name, kerr)
		}
		return err
	}

	if err != nil {
		dbg("warning: %s returned %s: %s", command, err, buf.String())
	}

	processOutput(dbg, state, buf.Bytes())
	elapsed := time.Since(start)
	dbg("%s linter took %s", state.Name, elapsed)
	return nil
}

func parseCommand(command string) ([]string, error) {
	args, err := shlex.Split(command)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("invalid command %q", command)
	}
	exe, err := exec.LookPath(args[0])
	if err != nil {
		return nil, err
	}
	return append([]string{exe}, args[1:]...), nil
}

// nolint: gocyclo
func processOutput(dbg debugFunction, state *linterState, out []byte) {
	re := state.regex
	all := re.FindAllSubmatchIndex(out, -1)
	dbg("%s hits %d: %s", state.Name, len(all), state.Pattern)

	cwd, err := os.Getwd()
	if err != nil {
		warning("failed to get working directory %s", err)
	}

	// Create a local copy of vars so they can be modified by the linter output
	vars := state.vars.Copy()

	for _, indices := range all {
		group := [][]byte{}
		for i := 0; i < len(indices); i += 2 {
			var fragment []byte
			if indices[i] != -1 {
				fragment = out[indices[i]:indices[i+1]]
			}
			group = append(group, fragment)
		}

		issue, err := NewIssue(state.Linter.Name, config.formatTemplate)
		kingpin.FatalIfError(err, "Invalid output format")

		for i, name := range re.SubexpNames() {
			if group[i] == nil {
				continue
			}
			part := string(group[i])
			if name != "" {
				vars[name] = part
			}
			switch name {
			case "path":
				issue.Path, err = newIssuePathFromAbsPath(cwd, part)
				if err != nil {
					warning("failed to make %s a relative path: %s", part, err)
				}
			case "line":
				n, err := strconv.ParseInt(part, 10, 32)
				kingpin.FatalIfError(err, "line matched invalid integer")
				issue.Line = int(n)

			case "col":
				n, err := strconv.ParseInt(part, 10, 32)
				kingpin.FatalIfError(err, "col matched invalid integer")
				issue.Col = int(n)

			case "message":
				issue.Message = part

			case "":
			}
		}
		// TODO: set messageOveride and severity on the Linter instead of reading
		// them directly from the static config
		if m, ok := config.MessageOverride[state.Name]; ok {
			issue.Message = vars.Replace(m)
		}
		if sev, ok := config.Severity[state.Name]; ok {
			issue.Severity = Severity(sev)
		}
		if state.exclude != nil && state.exclude.MatchString(issue.String()) {
			continue
		}
		if state.include != nil && !state.include.MatchString(issue.String()) {
			continue
		}
		state.issues <- issue
	}
}

func maybeSortIssues(issues chan *Issue) chan *Issue {
	if reflect.DeepEqual([]string{"none"}, config.Sort) {
		return issues
	}
	return SortIssueChan(issues, config.Sort)
}

func maybeAggregateIssues(issues chan *Issue) chan *Issue {
	if !config.Aggregate {
		return issues
	}
	return AggregateIssueChan(issues)
}
