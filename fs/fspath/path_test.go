package fspath

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	for _, test := range []struct {
		in, wantConfigName, wantFsPath string
	}{
		{"", "", ""},
		{"/path/to/file", "", "/path/to/file"},
		{"path/to/file", "", "path/to/file"},
		{"remote:path/to/file", "remote", "path/to/file"},
		{"remote:/path/to/file", "remote", "/path/to/file"},
		{":backend:/path/to/file", ":backend", "/path/to/file"},
	} {
		gotConfigName, gotFsPath := Parse(test.in)
		assert.Equal(t, test.wantConfigName, gotConfigName)
		assert.Equal(t, test.wantFsPath, gotFsPath)
	}
}

func TestSplit(t *testing.T) {
	for _, test := range []struct {
		remote, wantParent, wantLeaf string
	}{
		{"", "", ""},

		{"remote:", "remote:", ""},
		{"remote:potato", "remote:", "potato"},
		{"remote:/", "remote:/", ""},
		{"remote:/potato", "remote:/", "potato"},
		{"remote:/potato/potato", "remote:/potato/", "potato"},
		{"remote:potato/sausage", "remote:potato/", "sausage"},

		{":remote:", ":remote:", ""},
		{":remote:potato", ":remote:", "potato"},
		{":remote:/", ":remote:/", ""},
		{":remote:/potato", ":remote:/", "potato"},
		{":remote:/potato/potato", ":remote:/potato/", "potato"},
		{":remote:potato/sausage", ":remote:potato/", "sausage"},

		{"/", "/", ""},
		{"/root", "/", "root"},
		{"/a/b", "/a/", "b"},
		{"root", "", "root"},
		{"a/b", "a/", "b"},
		{"root/", "root/", ""},
		{"a/b/", "a/b/", ""},
	} {
		gotParent, gotLeaf := Split(test.remote)
		assert.Equal(t, test.wantParent, gotParent, test.remote)
		assert.Equal(t, test.wantLeaf, gotLeaf, test.remote)
		assert.Equal(t, test.remote, gotParent+gotLeaf, fmt.Sprintf("%s: %q + %q != %q", test.remote, gotParent, gotLeaf, test.remote))
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
