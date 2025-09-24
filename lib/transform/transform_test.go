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
