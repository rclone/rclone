package transform

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sync tests are in fs/sync/sync_transform_test.go to avoid import cycle issues

func newOptions(s ...string) (context.Context, error) {
	ctx := context.Background()
	err := SetOptions(ctx, s...)
	return ctx, err
}

func TestPath(t *testing.T) {
	for _, test := range []struct {
		path string
		want string
	}{
		{"", ""},
		{"toe/toe/toe", "tictactoe/tictactoe/tictactoe"},
		{"a/b/c", "tictaca/tictacb/tictacc"},
	} {
		ctx, err := newOptions("all,prefix=tac", "all,prefix=tic")
		require.NoError(t, err)

		got := Path(ctx, test.path, false)
		assert.Equal(t, test.want, got)
	}
}

func TestFileTagOnFile(t *testing.T) {
	for _, test := range []struct {
		path string
		want string
	}{
		{"a/b/c.txt", "a/b/1c.txt"},
	} {
		ctx, err := newOptions("file,prefix=1")
		require.NoError(t, err)

		got := Path(ctx, test.path, false)
		assert.Equal(t, test.want, got)
	}
}

func TestDirTagOnFile(t *testing.T) {
	for _, test := range []struct {
		path string
		want string
	}{
		{"a/b/c.txt", "1a/1b/c.txt"},
	} {
		ctx, err := newOptions("dir,prefix=1")
		require.NoError(t, err)

		got := Path(ctx, test.path, false)
		assert.Equal(t, test.want, got)
	}
}

func TestAllTag(t *testing.T) {
	for _, test := range []struct {
		path string
		want string
	}{
		{"a/b/c.txt", "1a/1b/1c.txt"},
	} {
		ctx, err := newOptions("all,prefix=1")
		require.NoError(t, err)

		got := Path(ctx, test.path, false)
		assert.Equal(t, test.want, got)
	}
}

func TestFileTagOnDir(t *testing.T) {
	for _, test := range []struct {
		path string
		want string
	}{
		{"a/b", "a/b"},
	} {
		ctx, err := newOptions("file,prefix=1")
		require.NoError(t, err)

		got := Path(ctx, test.path, true)
		assert.Equal(t, test.want, got)
	}
}

func TestDirTagOnDir(t *testing.T) {
	for _, test := range []struct {
		path string
		want string
	}{
		{"a/b", "1a/1b"},
	} {
		ctx, err := newOptions("dir,prefix=1")
		require.NoError(t, err)

		got := Path(ctx, test.path, true)
		assert.Equal(t, test.want, got)
	}
}

func TestVarious(t *testing.T) {
	for _, test := range []struct {
		path  string
		want  string
		flags []string
	}{
		{"stories/The Quick Brown Fox!.txt", "STORIES/THE QUICK BROWN FOX!.TXT", []string{"all,uppercase"}},
		{"stories/The Quick Brown Fox!.txt", "stories/The Slow Brown Turtle!.txt", []string{"all,replace=Fox:Turtle", "all,replace=Quick:Slow"}},
		{"stories/The Quick Brown Fox!.txt", "c3Rvcmllcw==/VGhlIFF1aWNrIEJyb3duIEZveCEudHh0", []string{"all,base64encode"}},
		{"c3Rvcmllcw==/VGhlIFF1aWNrIEJyb3duIEZveCEudHh0", "stories/The Quick Brown Fox!.txt", []string{"all,base64decode"}},
		{"stories/The Quick Brown 🦊 Fox Went to the Café!.txt", "stories/The Quick Brown 🦊 Fox Went to the Café!.txt", []string{"all,nfc"}},
		{"stories/The Quick Brown 🦊 Fox Went to the Café!.txt", "stories/The Quick Brown 🦊 Fox Went to the Café!.txt", []string{"all,nfd"}},
		{"stories/The Quick Brown 🦊 Fox!.txt", "stories/The Quick Brown  Fox!.txt", []string{"all,ascii"}},
		{"stories/The Quick Brown 🦊 Fox!.txt", "stories/The+Quick+Brown+%F0%9F%A6%8A+Fox%21.txt", []string{"all,url"}},
		{"stories/The Quick Brown Fox!.txt", "stories/The Quick Brown Fox!", []string{"all,trimsuffix=.txt"}},
		{"stories/The Quick Brown Fox!.txt", "OLD_stories/OLD_The Quick Brown Fox!.txt", []string{"all,prefix=OLD_"}},
		{"stories/The Quick Brown 🦊 Fox Went to the Café!.txt", "stories/The Quick Brown _ Fox Went to the Caf_!.txt", []string{"all,charmap=ISO-8859-7"}},
		{"stories/The Quick Brown Fox: A Memoir [draft].txt", "stories/The Quick Brown Fox： A Memoir ［draft］.txt", []string{"all,encoder=Colon,SquareBracket"}},
		{"stories/The Quick Brown 🦊 Fox Went to the Café!.txt", "stories/The Quick Brown 🦊 Fox", []string{"all,truncate=21"}},
		{"stories/Вот русское предложение, в котором байтов больше, чем символов.txt", "stories/Вот русское предложение, в котором байтов больше, чем символов.txt", []string{"truncate=70"}},
		{"stories/Вот русское предложение, в котором байтов больше, чем символов.txt", "stories/Вот русское предложение, в котором байтов больше, чем символ", []string{"truncate=60"}},
		{"stories/Вот русское предложение, в котором байтов больше, чем символов.txt", "stories/Вот русское предложение, в котором байтов больше, чем символов.txt", []string{"truncate_bytes=300"}},
		{"stories/Вот русское предложение, в котором байтов больше, чем символов.txt", "stories/Вот русское предложение, в котором бай", []string{"truncate_bytes=70"}},
		{"stories/Вот русское предложение, в котором байтов больше, чем символов.txt", "stories/Вот русское предложение, в котором байтов больше, чем си.txt", []string{"truncate_keep_extension=60"}},
		{"stories/Вот русское предложение, в котором байтов больше, чем символов.txt", "stories/Вот русское предложение, в котором б.txt", []string{"truncate_bytes_keep_extension=70"}},
		{"stories/The Quick Brown Fox!.txt", "stories/The Quick Brown Fox!.txt", []string{"all,command=echo"}},
		{"stories/The Quick Brown Fox!.txt", "stories/The Quick Brown Fox!.txt-" + time.Now().Local().Format("20060102"), []string{"date=-{YYYYMMDD}"}},
		{"stories/The Quick Brown Fox!.txt", "stories/The Quick Brown Fox!.txt-" + time.Now().Local().Format("2006-01-02 0304PM"), []string{"date=-{macfriendlytime}"}},
		{"stories/The Quick Brown Fox!.txt", "ababababababab/ababab ababababab ababababab ababab!abababab", []string{"all,regex=[\\.\\w]/ab"}},
	} {
		ctx, err := newOptions(test.flags...)
		require.NoError(t, err)

		got := Path(ctx, test.path, false)
		assert.Equal(t, test.want, got)
	}
}

// an invalid value must be rejected when the flag is parsed, so that the
// command fails before transferring anything instead of failing per file
func TestInvalidValueRejectedWhenParsed(t *testing.T) {
	for _, test := range []struct {
		flag string
		want string
	}{
		{"all,replace=10:30:10-30", "wrong number of values: 10:30:10-30"},
		{"all,replace=onlyone", "wrong number of values: onlyone"},
		{"all,regex=a/b/c", "regex syntax error: a/b/c"},
		{"all,regex=[/x", "regex syntax error: [/x: error parsing regexp: missing closing ]: `[`"},
	} {
		_, err := newOptions(test.flag)
		require.Error(t, err, test.flag)
		assert.Equal(t, test.want, err.Error())
	}
}

// a valid regex is compiled once when parsed, not once per path segment
func TestRegexCompiledOnce(t *testing.T) {
	tr, err := parse("all,regex=[0-9]/#")
	require.NoError(t, err)
	require.NotNil(t, tr.re)
	assert.Equal(t, "[0-9]", tr.re.String())
}

// a failing transform must leave the name alone rather than return an empty
// one, which would then be used as the destination name
//
// The existing "number of path segments must match" check only catches this
// for a path containing a separator: a bare file name has as many segments as
// the empty string, so it used to pass straight through.
func TestFailedTransformKeepsOriginalName(t *testing.T) {
	for _, path := range []string{"file.txt", "dir/file.txt"} {
		ctx, err := newOptions("all,truncate=notanumber")
		require.NoError(t, err) // truncate takes any value at parse time
		assert.Equal(t, path, Path(ctx, path, false))
	}
}
