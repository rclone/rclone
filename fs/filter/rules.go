package filter

import (
	"bufio"
	"bytes"
	"fmt"
	"maps"
	"os"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rclone/rclone/fs"
)

// RulesOpt is configuration for a rule set
type RulesOpt struct {
	FilterRule  []string `config:"filter"`
	FilterFrom  []string `config:"filter_from"`
	ExcludeRule []string `config:"exclude"`
	ExcludeFrom []string `config:"exclude_from"`
	IncludeRule []string `config:"include"`
	IncludeFrom []string `config:"include_from"`
}

// rule is one filter rule
type rule struct {
	Include bool
	Regexp  *regexp.Regexp
}

// Match returns true if rule matches path
func (r *rule) Match(path string) bool {
	return r.Regexp.MatchString(path)
}

// String the rule
func (r *rule) String() string {
	c := "-"
	if r.Include {
		c = "+"
	}
	return fmt.Sprintf("%s %s", c, r.Regexp.String())
}

// rules is a slice of rules
type rules struct {
	rules           []rule
	existing        map[string]struct{}
	byDirectory     map[string][]int
	foldedDirectory map[string][]int
	otherRules      []int
}

// Small rule sets are faster to scan without maintaining an index.
const minIndexedRules = 64

type addFn func(Include bool, glob string) error

// add adds a rule if it doesn't exist already
func (rs *rules) add(Include bool, re *regexp.Regexp) {
	if rs.existing == nil {
		rs.existing = make(map[string]struct{})
	}
	newRule := rule{
		Include: Include,
		Regexp:  re,
	}
	newRuleString := newRule.String()
	if _, ok := rs.existing[newRuleString]; ok {
		return // rule already exists
	}
	rs.rules = append(rs.rules, newRule)
	rs.existing[newRuleString] = struct{}{}
	if rs.byDirectory != nil {
		rs.index(len(rs.rules) - 1)
	} else if len(rs.rules) == minIndexedRules {
		rs.byDirectory = make(map[string][]int)
		for i := range rs.rules {
			rs.index(i)
		}
	}
}

// foldDirectory uses the same Unicode simple-fold equivalence as regexp.
func foldDirectory(directory string) string {
	return strings.Map(func(r rune) rune {
		if r < utf8.RuneSelf {
			if r >= 'a' && r <= 'z' {
				return r - 'a' + 'A'
			}
			return r
		}
		minimum := r
		for folded := unicode.SimpleFold(r); folded != r; folded = unicode.SimpleFold(folded) {
			minimum = min(minimum, folded)
		}
		return minimum
	}, directory)
}

// index groups anchored rules by their first literal directory.
func (rs *rules) index(i int) {
	parsed, err := syntax.Parse(rs.rules[i].Regexp.String(), syntax.Perl)
	if err == nil && parsed.Op == syntax.OpConcat && len(parsed.Sub) >= 2 &&
		parsed.Sub[0].Op == syntax.OpBeginText && parsed.Sub[1].Op == syntax.OpLiteral {
		if directory, _, ok := strings.Cut(string(parsed.Sub[1].Rune), "/"); ok {
			if parsed.Sub[1].Flags&syntax.FoldCase != 0 {
				if rs.foldedDirectory == nil {
					rs.foldedDirectory = make(map[string][]int)
				}
				directory = foldDirectory(directory)
				rs.foldedDirectory[directory] = append(rs.foldedDirectory[directory], i)
			} else {
				rs.byDirectory[directory] = append(rs.byDirectory[directory], i)
			}
			return
		}
	}
	rs.otherRules = append(rs.otherRules, i)
}

// Add adds a filter rule with include or exclude status indicated
func (rs *rules) Add(Include bool, glob string) error {
	re, err := GlobPathToRegexp(glob, false /* f.Opt.IgnoreCase */)
	if err != nil {
		return err
	}
	rs.add(Include, re)
	return nil
}

type clearFn func()

// clear clears all the rules
func (rs *rules) clear() {
	rs.rules = nil
	rs.existing = nil
	rs.byDirectory = nil
	rs.foldedDirectory = nil
	rs.otherRules = nil
}

// clone copies the mutable rule storage, including directory buckets.
func (rs *rules) clone() rules {
	copy := *rs
	copy.rules = slices.Clone(rs.rules)
	copy.existing = maps.Clone(rs.existing)
	copy.otherRules = slices.Clone(rs.otherRules)
	copy.byDirectory = cloneDirectoryIndex(rs.byDirectory)
	copy.foldedDirectory = cloneDirectoryIndex(rs.foldedDirectory)
	return copy
}

func cloneDirectoryIndex(index map[string][]int) map[string][]int {
	copy := maps.Clone(index)
	for directory, bucket := range copy {
		copy[directory] = slices.Clone(bucket)
	}
	return copy
}

// len returns the number of rules
func (rs *rules) len() int {
	return len(rs.rules)
}

// include returns whether this remote passes the filter rules.
func (rs *rules) include(remote string) bool {
	if len(rs.byDirectory) != 0 || len(rs.foldedDirectory) != 0 {
		directory, _, _ := strings.Cut(remote, "/")
		// Regexp decodes invalid UTF-8 bytes as U+FFFD, which can match a
		// literal U+FFFD. Use the linear scan when byte lookup could miss it.
		if !utf8.ValidString(directory) {
			for _, rule := range rs.rules {
				if rule.Match(remote) {
					return rule.Include
				}
			}
			return true
		}
		candidates := [3][]int{rs.byDirectory[directory], nil, rs.otherRules}
		if len(rs.foldedDirectory) != 0 {
			candidates[1] = rs.foldedDirectory[foldDirectory(directory)]
		}
		for {
			// Merge the sorted buckets to preserve first-match precedence.
			first := -1
			for j, bucket := range candidates {
				if len(bucket) != 0 && (first == -1 || bucket[0] < candidates[first][0]) {
					first = j
				}
			}
			if first == -1 {
				return true
			}
			i := candidates[first][0]
			candidates[first] = candidates[first][1:]
			if rs.rules[i].Match(remote) {
				return rs.rules[i].Include
			}
		}
	}
	for _, rule := range rs.rules {
		if rule.Match(remote) {
			return rule.Include
		}
	}
	return true
}

// include returns whether this collection of strings remote passes
// the filter rules.
//
// the first rule is evaluated on all the remotes and if it matches
// then the result is returned. If not the next rule is tested and so
// on.
func (rs *rules) includeMany(remotes []string) bool {
	for _, rule := range rs.rules {
		if slices.ContainsFunc(remotes, rule.Match) {
			return rule.Include
		}
	}
	return true
}

// scanNul is a split function for a Scanner that returns each NUL-terminated
// sequence of bytes. It correctly handles the final segment even if it
// lacks a trailing NUL.
func scanNul(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\x00'); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// forEachLine calls fn on every line in the file pointed to by path
//
// It ignores empty lines and lines starting with '#' or ';' if raw is false
func forEachLine(path string, raw bool, useNulDelimiter bool, fn func(string) error) (err error) {
	var scanner *bufio.Scanner
	if path == "-" {
		scanner = bufio.NewScanner(os.Stdin)
	} else {
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		scanner = bufio.NewScanner(in)
		defer fs.CheckClose(in, &err)
	}

	if useNulDelimiter {
		scanner.Split(scanNul)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if !raw {
			line = strings.TrimSpace(line)
			if len(line) == 0 || line[0] == '#' || line[0] == ';' {
				continue
			}
		}
		err := fn(line)
		if err != nil {
			return err
		}
	}
	return scanner.Err()
}

// AddRule adds a filter rule with include/exclude indicated by the prefix
//
// These are
//
//	# Comment
//	+ glob
//	- glob
//	!
//
// '+' includes the glob, '-' excludes it and '!' resets the filter list
//
// Line comments may be introduced with '#' or ';'
func addRule(rule string, add addFn, clear clearFn) error {
	switch {
	case rule == "!":
		clear()
		return nil
	case strings.HasPrefix(rule, "- "):
		return add(false, rule[2:])
	case strings.HasPrefix(rule, "+ "):
		return add(true, rule[2:])
	}
	return fmt.Errorf("malformed rule %q", rule)
}

// AddRule adds a filter rule with include/exclude indicated by the prefix
//
// These are
//
//	# Comment
//	+ glob
//	- glob
//	!
//
// '+' includes the glob, '-' excludes it and '!' resets the filter list
//
// Line comments may be introduced with '#' or ';'
func (rs *rules) AddRule(rule string) error {
	return addRule(rule, rs.Add, rs.clear)
}

// Parse the rules passed in and add them to the function
func parseRules(opt *RulesOpt, add addFn, clear clearFn) (err error) {
	addImplicitExclude := false
	foundExcludeRule := false

	for _, rule := range opt.IncludeRule {
		err = add(true, rule)
		if err != nil {
			return err
		}
		addImplicitExclude = true
	}
	for _, rule := range opt.IncludeFrom {
		err := forEachLine(rule, false, false, func(line string) error {
			return add(true, line)
		})
		if err != nil {
			return err
		}
		addImplicitExclude = true
	}
	for _, rule := range opt.ExcludeRule {
		err = add(false, rule)
		if err != nil {
			return err
		}
		foundExcludeRule = true
	}
	for _, rule := range opt.ExcludeFrom {
		err := forEachLine(rule, false, false, func(line string) error {
			return add(false, line)
		})
		if err != nil {
			return err
		}
		foundExcludeRule = true
	}

	if addImplicitExclude && foundExcludeRule {
		fs.Errorf(nil, "Using --filter is recommended instead of both --include and --exclude as the order they are parsed in is indeterminate")
	}

	for _, rule := range opt.FilterRule {
		err = addRule(rule, add, clear)
		if err != nil {
			return err
		}
	}
	for _, rule := range opt.FilterFrom {
		err := forEachLine(rule, false, false, func(rule string) error {
			return addRule(rule, add, clear)
		})
		if err != nil {
			return err
		}
	}

	if addImplicitExclude {
		err = add(false, "/**")
		if err != nil {
			return err
		}
	}

	return nil
}
