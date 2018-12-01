package meta

import (
	"sort"
	"strings"
)

// BuildTags are tags used for building apps
type BuildTags []string

// String returns the tags in the form of:
// "foo bar baz" (with the quotes!)
func (t BuildTags) String() string {
	return strings.Join(t, " ")
}

// BuildTags combines the passed in env, and any additional tags,
// with tags that Buffalo decides the build process requires.
// An example would be adding the "sqlite" build tag if using
// SQLite3.
func (a App) BuildTags(env string, tags ...string) BuildTags {
	m := map[string]string{}
	m[env] = env
	for _, t := range tags {
		m[t] = t
	}
	if a.WithSQLite {
		m["sqlite"] = "sqlite"
	}
	var tt []string
	for k := range m {
		k = strings.TrimSpace(k)
		if len(k) != 0 {
			tt = append(tt, k)
		}
	}
	sort.Strings(tt)
	return BuildTags(tt)
}
