package transform

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sync tests are in fs/sync/sync_transform_test.go to avoid import cycle issues

func TestPath(t *testing.T) {
	for _, test := range []struct {
		path string
		want string
	}{
		{"", ""},
		{"toe/toe/toe", "tictactoe/tictactoe/tictactoe"},
		{"a/b/c", "tictaca/tictacb/tictacc"},
	} {
		err := SetOptions(context.Background(), "all,prefix=tac", "all,prefix=tic")
		require.NoError(t, err)

		got := Path(test.path, false)
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
		err := SetOptions(context.Background(), "file,prefix=1")
		require.NoError(t, err)

		got := Path(test.path, false)
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
		err := SetOptions(context.Background(), "dir,prefix=1")
		require.NoError(t, err)

		got := Path(test.path, false)
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
		err := SetOptions(context.Background(), "all,prefix=1")
		require.NoError(t, err)

		got := Path(test.path, false)
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
		err := SetOptions(context.Background(), "file,prefix=1")
		require.NoError(t, err)

		got := Path(test.path, true)
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
		err := SetOptions(context.Background(), "dir,prefix=1")
		require.NoError(t, err)

		got := Path(test.path, true)
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
		{"stories/The Quick Brown Fox!.txt", "stories/The Quick Brown Fox!", []string{"all,trimsuffix=.txt"}},
		{"stories/The Quick Brown Fox!.txt", "OLD_stories/OLD_The Quick Brown Fox!.txt", []string{"all,prefix=OLD_"}},
		{"stories/The Quick Brown 🦊 Fox Went to the Café!.txt", "stories/The Quick Brown _ Fox Went to the Caf_!.txt", []string{"all,charmap=ISO-8859-7"}},
		{"stories/The Quick Brown Fox: A Memoir [draft].txt", "stories/The Quick Brown Fox： A Memoir ［draft］.txt", []string{"all,encoder=Colon,SquareBracket"}},
		{"stories/The Quick Brown 🦊 Fox Went to the Café!.txt", "stories/The Quick Brown 🦊 Fox", []string{"all,truncate=21"}},
	} {
		err := SetOptions(context.Background(), test.flags...)
		require.NoError(t, err)

		got := Path(test.path, false)
		assert.Equal(t, test.want, got)
	}
}
