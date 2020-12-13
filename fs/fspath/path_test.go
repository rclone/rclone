package fspath

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckConfigName(t *testing.T) {
	for _, test := range []struct {
		in   string
		want error
	}{
		{"remote", nil},
		{"", errInvalidCharacters},
		{":remote:", errInvalidCharacters},
		{"remote:", errInvalidCharacters},
		{"rem:ote", errInvalidCharacters},
		{"rem/ote", errInvalidCharacters},
		{"rem\\ote", errInvalidCharacters},
		{"[remote", errInvalidCharacters},
		{"*", errInvalidCharacters},
		{"-remote", errCantStartWithDash},
		{"r-emote-", nil},
		{"_rem_ote_", nil},
	} {
		got := CheckConfigName(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestCheckRemoteName(t *testing.T) {
	for _, test := range []struct {
		in   string
		want error
	}{
		{":remote:", nil},
		{"remote:", nil},
		{"", errInvalidCharacters},
		{"rem:ote", errInvalidCharacters},
		{"rem:ote:", errInvalidCharacters},
		{"remote", errInvalidCharacters},
		{"rem/ote:", errInvalidCharacters},
		{"rem\\ote:", errInvalidCharacters},
		{"[remote:", errInvalidCharacters},
		{"*:", errInvalidCharacters},
	} {
		got := CheckRemoteName(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestParse(t *testing.T) {
	for _, test := range []struct {
		in, wantConfigName, wantFsPath string
		wantErr                        error
	}{
		{"", "", "", errCantBeEmpty},
		{":", "", "", errInvalidCharacters},
		{"::", ":", "", errInvalidCharacters},
		{":/:", "", "/:", errInvalidCharacters},
		{"/:", "", "/:", nil},
		{"\\backslash:", "", "\\backslash:", nil},
		{"/slash:", "", "/slash:", nil},
		{"with\\backslash:", "", "with\\backslash:", nil},
		{"with/slash:", "", "with/slash:", nil},
		{"/path/to/file", "", "/path/to/file", nil},
		{"/path:/to/file", "", "/path:/to/file", nil},
		{"./path:/to/file", "", "./path:/to/file", nil},
		{"./:colon.txt", "", "./:colon.txt", nil},
		{"path/to/file", "", "path/to/file", nil},
		{"remote:path/to/file", "remote", "path/to/file", nil},
		{"rem*ote:path/to/file", "rem*ote", "path/to/file", errInvalidCharacters},
		{"remote:/path/to/file", "remote", "/path/to/file", nil},
		{"rem.ote:/path/to/file", "rem.ote", "/path/to/file", errInvalidCharacters},
		{":backend:/path/to/file", ":backend", "/path/to/file", nil},
		{":bac*kend:/path/to/file", ":bac*kend", "/path/to/file", errInvalidCharacters},
	} {
		gotConfigName, gotFsPath, gotErr := Parse(test.in)
		if runtime.GOOS == "windows" {
			test.wantFsPath = strings.Replace(test.wantFsPath, `\`, `/`, -1)
		}
		assert.Equal(t, test.wantErr, gotErr)
		assert.Equal(t, test.wantConfigName, gotConfigName)
		assert.Equal(t, test.wantFsPath, gotFsPath)
	}
}

func TestSplit(t *testing.T) {
	for _, test := range []struct {
		remote, wantParent, wantLeaf string
		wantErr                      error
	}{
		{"", "", "", errCantBeEmpty},

		{"remote:", "remote:", "", nil},
		{"remote:potato", "remote:", "potato", nil},
		{"remote:/", "remote:/", "", nil},
		{"remote:/potato", "remote:/", "potato", nil},
		{"remote:/potato/potato", "remote:/potato/", "potato", nil},
		{"remote:potato/sausage", "remote:potato/", "sausage", nil},
		{"rem.ote:potato/sausage", "", "", errInvalidCharacters},

		{":remote:", ":remote:", "", nil},
		{":remote:potato", ":remote:", "potato", nil},
		{":remote:/", ":remote:/", "", nil},
		{":remote:/potato", ":remote:/", "potato", nil},
		{":remote:/potato/potato", ":remote:/potato/", "potato", nil},
		{":remote:potato/sausage", ":remote:potato/", "sausage", nil},
		{":rem[ote:potato/sausage", "", "", errInvalidCharacters},

		{"/", "/", "", nil},
		{"/root", "/", "root", nil},
		{"/a/b", "/a/", "b", nil},
		{"root", "", "root", nil},
		{"a/b", "a/", "b", nil},
		{"root/", "root/", "", nil},
		{"a/b/", "a/b/", "", nil},
	} {
		gotParent, gotLeaf, gotErr := Split(test.remote)
		assert.Equal(t, test.wantErr, gotErr)
		assert.Equal(t, test.wantParent, gotParent, test.remote)
		assert.Equal(t, test.wantLeaf, gotLeaf, test.remote)
		if gotErr == nil {
			assert.Equal(t, test.remote, gotParent+gotLeaf, fmt.Sprintf("%s: %q + %q != %q", test.remote, gotParent, gotLeaf, test.remote))
		}
	}
}

func TestMakeAbsolute(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{".", ""},
		{"/.", "/"},
		{"../potato", "potato"},
		{"/../potato", "/potato"},
		{"./../potato", "potato"},
		{"//../potato", "/potato"},
		{"././../potato", "potato"},
		{"././potato/../../onion", "onion"},
	} {
		got := makeAbsolute(test.in)
		assert.Equal(t, test.want, got, test)
	}
}

func TestJoinRootPath(t *testing.T) {
	for _, test := range []struct {
		remote   string
		filePath string
		want     string
	}{
		{"", "", ""},
		{"", "/", "/"},
		{"/", "", "/"},
		{"/", "/", "/"},
		{"/", "//", "/"},
		{"/root", "", "/root"},
		{"/root", "/", "/root"},
		{"/root", "//", "/root"},
		{"/a/b", "", "/a/b"},
		{"//", "/", "//"},
		{"//server", "path", "//server/path"},
		{"//server/sub", "path", "//server/sub/path"},
		{"//server", "//path", "//server/path"},
		{"//server/sub", "//path", "//server/sub/path"},
		{"//", "/", "//"},
		{"//server", "path", "//server/path"},
		{"//server/sub", "path", "//server/sub/path"},
		{"//server", "//path", "//server/path"},
		{"//server/sub", "//path", "//server/sub/path"},
		{filepath.FromSlash("//server/sub"), filepath.FromSlash("//path"), "//server/sub/path"},
		{"s3:", "", "s3:"},
		{"s3:", ".", "s3:"},
		{"s3:.", ".", "s3:"},
		{"s3:", "..", "s3:"},
		{"s3:dir", "sub", "s3:dir/sub"},
		{"s3:dir", "/sub", "s3:dir/sub"},
		{"s3:dir", "./sub", "s3:dir/sub"},
		{"s3:/dir", "/sub/", "s3:/dir/sub"},
		{"s3:dir", "..", "s3:dir"},
		{"s3:dir", "/..", "s3:dir"},
		{"s3:dir", "/../", "s3:dir"},
	} {
		got := JoinRootPath(test.remote, test.filePath)
		assert.Equal(t, test.want, got, test)
	}
}
