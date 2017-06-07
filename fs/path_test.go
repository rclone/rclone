package fs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteSplit(t *testing.T) {

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
		{"/", "/", ""},
		{"/root", "/", "root"},
		{"/a/b", "/a/", "b"},
		{"root", "", "root"},
		{"a/b", "a/", "b"},
		{"root/", "root/", ""},
		{"a/b/", "a/b/", ""},
	} {
		gotParent, gotLeaf := RemoteSplit(test.remote)
		assert.Equal(t, test.wantParent, gotParent, test.remote)
		assert.Equal(t, test.wantLeaf, gotLeaf, test.remote)
		assert.Equal(t, test.remote, gotParent+gotLeaf, fmt.Sprintf("%s: %q + %q != %q", test.remote, gotParent, gotLeaf, test.remote))
	}
}
