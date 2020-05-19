package fspath

import (
	"fmt"
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
func TestJoinRootPath(t *testing.T) {
	for _, test := range []struct {
		elements []string
		want     string
	}{
		{nil, ""},
		{[]string{""}, ""},
		{[]string{"/"}, "/"},
		{[]string{"/", "/"}, "/"},
		{[]string{"/", "//"}, "/"},
		{[]string{"/root", ""}, "/root"},
		{[]string{"/root", "/"}, "/root"},
		{[]string{"/root", "//"}, "/root"},
		{[]string{"/a/b"}, "/a/b"},
		{[]string{"//", "/"}, "//"},
		{[]string{"//server", "path"}, "//server/path"},
		{[]string{"//server/sub", "path"}, "//server/sub/path"},
		{[]string{"//server", "//path"}, "//server/path"},
		{[]string{"//server/sub", "//path"}, "//server/sub/path"},
		{[]string{"", "//", "/"}, "//"},
		{[]string{"", "//server", "path"}, "//server/path"},
		{[]string{"", "//server/sub", "path"}, "//server/sub/path"},
		{[]string{"", "//server", "//path"}, "//server/path"},
		{[]string{"", "//server/sub", "//path"}, "//server/sub/path"},
	} {
		got := JoinRootPath(test.elements...)
		assert.Equal(t, test.want, got)
	}
}
