package main

import (
	"sort"
	"strings"
)

type issueKey struct {
	path      string
	line, col int
	message   string
}

type multiIssue struct {
	*Issue
	linterNames []string
}

// AggregateIssueChan reads issues from a channel, aggregates issues which have
// the same file, line, vol, and message, and returns aggregated issues on
// a new channel.
func AggregateIssueChan(issues chan *Issue) chan *Issue {
	out := make(chan *Issue, 1000000)
	issueMap := make(map[issueKey]*multiIssue)
	go func() {
		for issue := range issues {
			key := issueKey{
				path:    issue.Path.String(),
				line:    issue.Line,
				col:     issue.Col,
				message: issue.Message,
			}
			if existing, ok := issueMap[key]; ok {
				existing.linterNames = append(existing.linterNames, issue.Linter)
			} else {
				issueMap[key] = &multiIssue{
					Issue:       issue,
					linterNames: []string{issue.Linter},
				}
			}
		}
		for _, multi := range issueMap {
			issue := multi.Issue
			sort.Strings(multi.linterNames)
			issue.Linter = strings.Join(multi.linterNames, ", ")
			out <- issue
		}
		close(out)
	}()
	return out
}
