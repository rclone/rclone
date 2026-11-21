package genfilters

import (
	"context"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs/filter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// samples are the remotes the tests check rules against
var samples = []string{
	"a.txt", "b.txt", "tmp/", "tmp/junk.tmp", "tmp/keep.txt",
	"docs/", "docs/one.md", "docs/sub/", "docs/sub/deep.md",
	"music/", "music/song.mp3", "we*rd[1]/", "we*rd[1]/{x}.txt", "space /", "space /end ",
}

// engine returns a function reporting whether rclone includes key
// with rules, the way sync would: every directory above key must be
// included too
func engine(t *testing.T, rules []string) func(key string) bool {
	filt, err := filter.NewFilter(nil)
	require.NoError(t, err)
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)
		if rule == "" || rule[0] == '#' || rule[0] == ';' {
			continue
		}
		require.NoError(t, filt.AddRule(rule))
	}
	includeDir := filt.IncludeDirectory(context.Background(), nil)
	return func(key string) bool {
		parts := strings.Split(strings.TrimSuffix(key, "/"), "/")
		for i := 1; i < len(parts); i++ {
			ok, err := includeDir(strings.Join(parts[:i], "/"))
			require.NoError(t, err)
			if !ok {
				return false
			}
		}
		if strings.HasSuffix(key, "/") {
			ok, err := includeDir(key)
			require.NoError(t, err)
			if !ok {
				return false
			}
		}
		return filt.IncludeRemote(key)
	}
}

// checkSame checks the model includes the same samples as rules do
func checkSame(t *testing.T, mo *model, rules []string) {
	t.Helper()
	includes := engine(t, rules)
	for _, key := range samples {
		assert.Equal(t, includes(key), mo.included(key), "included(%q) with rules %q", key, rules)
	}
}

func TestModelIncludeOthers(t *testing.T) {
	mo, err := newModel(includeOthers)
	require.NoError(t, err)
	assert.Equal(t, []string{"+ **"}, mo.rules())

	require.NoError(t, mo.set("tmp/", false))
	require.NoError(t, mo.set("a.txt", false))
	assert.Equal(t, []string{"- /tmp/**", "- /a.txt", "+ **"}, mo.rules())
	assert.False(t, mo.included("tmp/junk.tmp"))
	assert.True(t, mo.included("b.txt"))
	assert.False(t, mo.partial("tmp/"), "fully excluded")
	checkSame(t, mo, mo.rules())

	// A file inside an excluded directory
	require.NoError(t, mo.set("tmp/keep.txt", true))
	assert.Equal(t, []string{"+ /tmp/keep.txt", "- /tmp/**", "- /a.txt", "+ **"}, mo.rules())
	assert.True(t, mo.included("tmp/keep.txt"))
	assert.False(t, mo.included("tmp/junk.tmp"))
	assert.True(t, mo.partial("tmp/"))
	checkSame(t, mo, mo.rules())

	// Setting the directory replaces the rules inside it
	require.NoError(t, mo.set("tmp/", true))
	assert.Equal(t, []string{"- /a.txt", "+ **"}, mo.rules())

	// No rule is needed to include what is already included
	require.NoError(t, mo.set("b.txt", true))
	require.NoError(t, mo.set("a.txt", true))
	assert.Equal(t, []string{"+ **"}, mo.rules())
}

func TestModelExcludeOthers(t *testing.T) {
	mo, err := newModel(excludeOthers)
	require.NoError(t, err)
	require.NoError(t, mo.set("docs/one.md", true))
	require.NoError(t, mo.set("music/", true))
	assert.Equal(t, []string{"+ /music/**", "+ /docs/one.md", "- **"}, mo.rules())
	assert.True(t, mo.included("docs/one.md"))
	assert.False(t, mo.included("docs/sub/deep.md"))
	assert.True(t, mo.partial("docs/"))
	assert.False(t, mo.partial("music/"))
	assert.False(t, mo.partial("tmp/"), "fully excluded")
	assert.True(t, mo.included("music/song.mp3"))
	assert.False(t, mo.included("a.txt"))
	checkSame(t, mo, mo.rules())

	// A directory inside an excluded directory inside an included one
	require.NoError(t, mo.set("docs/", true))
	require.NoError(t, mo.set("docs/sub/", false))
	require.NoError(t, mo.set("docs/sub/deep.md", true))
	assert.Equal(t, []string{"+ /music/**", "+ /docs/sub/deep.md", "- /docs/sub/**", "+ /docs/**", "- **"}, mo.rules())
	checkSame(t, mo, mo.rules())

	require.NoError(t, mo.setMode(includeOthers))
	assert.Equal(t, []string{"+ **"}, mo.rules())
}

func TestModelSpecialNames(t *testing.T) {
	mo, err := newModel(includeOthers)
	require.NoError(t, err)
	require.NoError(t, mo.set("we*rd[1]/{x}.txt", false))
	require.NoError(t, mo.set("space /end ", false))
	assert.Equal(t, []string{`- /we\*rd\[1\]/\{x\}.txt`, "- /space /end[ ]", "+ **"}, mo.rules())
	assert.False(t, mo.included("we*rd[1]/{x}.txt"))
	assert.True(t, mo.included("we*rd[1]/"))
	checkSame(t, mo, mo.rules())
}
