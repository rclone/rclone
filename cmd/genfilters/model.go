package genfilters

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/rclone/rclone/fs/filter"
)

// mode is what happens to everything without its own rule
type mode uint8

const (
	includeOthers mode = iota // + **
	excludeOthers             // - **
)

// sign returns the rule prefix for include ("+" or "-")
func sign(include bool) string {
	if include {
		return "+"
	}
	return "-"
}

// model is the set of filter rules being built.
//
// Keys are remotes relative to the root, with a trailing "/" for
// directories. A directory rule covers everything in it.
type model struct {
	mode     mode
	header   []string        // comment lines written before the rules
	selected map[string]bool // key -> included
	filt     *filter.Filter
}

// newModel returns an empty model in mode m
func newModel(m mode) (*model, error) {
	mo := &model{
		mode:     m,
		selected: map[string]bool{},
	}
	return mo, mo.build()
}

// keys returns the keys with rules, each before any directory
// containing it, so the most specific rule matches first
func (mo *model) keys() []string {
	keys := make([]string, 0, len(mo.selected))
	for k := range mo.selected {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	return keys
}

// glob returns the pattern a rule for key uses
func glob(key string) string {
	g := "/" + escape(strings.TrimSuffix(key, "/"))
	if strings.HasSuffix(key, "/") {
		g += "/**"
	}
	return g
}

// rules returns the filter rules, ending with the catch-all
func (mo *model) rules() []string {
	var rules []string
	for _, k := range mo.keys() {
		rules = append(rules, sign(mo.selected[k])+" "+glob(k))
	}
	return append(rules, sign(mo.mode == includeOthers)+" **")
}

// lines returns the header followed by the rules
func (mo *model) lines() []string {
	return append(append([]string(nil), mo.header...), mo.rules()...)
}

// build makes the filter from the rules
func (mo *model) build() error {
	filt, err := filter.NewFilter(nil)
	if err != nil {
		return err
	}
	for _, rule := range mo.rules() {
		if err := filt.AddRule(rule); err != nil {
			return fmt.Errorf("bad rule %q: %w", rule, err)
		}
	}
	mo.filt = filt
	return nil
}

// traversed returns whether rclone would list the directories
// containing key, and key itself if it is a directory
func (mo *model) traversed(key string) bool {
	includeDir := mo.filt.IncludeDirectory(context.Background(), nil)
	dir := key
	if !strings.HasSuffix(key, "/") {
		dir = path.Dir(key)
	}
	for ; dir != "." && dir != "/"; dir = path.Dir(strings.TrimSuffix(dir, "/")) {
		// fs is nil, so this can't fail
		if ok, _ := includeDir(dir); !ok {
			return false
		}
	}
	return true
}

// included returns whether rclone would include key with the rules
func (mo *model) included(key string) bool {
	return mo.traversed(key) && mo.filt.IncludeRemote(key)
}

// partial returns whether directory key is listed but only the
// things in it with their own rules are included
func (mo *model) partial(key string) bool {
	return strings.HasSuffix(key, "/") && mo.traversed(key) && !mo.filt.IncludeRemote(key)
}

// set makes key included or excluded with as few rules as possible.
// Setting a directory replaces the rules for everything in it.
func (mo *model) set(key string, include bool) error {
	delete(mo.selected, key)
	if strings.HasSuffix(key, "/") {
		for k := range mo.selected {
			if strings.HasPrefix(k, key) {
				delete(mo.selected, k)
			}
		}
	}
	if err := mo.build(); err != nil {
		return err
	}
	if mo.included(key) == include {
		return nil
	}
	mo.selected[key] = include
	if err := mo.build(); err != nil {
		return err
	}
	if mo.included(key) != include {
		return fmt.Errorf("couldn't make a rule for %q", key)
	}
	return nil
}

// setMode changes the mode, removing all the rules
func (mo *model) setMode(m mode) error {
	mo.mode = m
	mo.selected = map[string]bool{}
	return mo.build()
}
