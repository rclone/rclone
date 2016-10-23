package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteSplit(t *testing.T) {

	for _, test := range []struct {
		remote, wantParent, wantLeaf string
	}{
		{"", "", ""},
		{"remote:", "", ""},
		{"remote:potato", "remote:", "potato"},
		{"remote:potato/sausage", "remote:potato", "sausage"},
		{"/", "", ""},
		{"/root", "/", "root"},
		{"/a/b", "/a", "b"},
		{"root", ".", "root"},
		{"a/b", "a", "b"},
	} {
		gotParent, gotLeaf := RemoteSplit(test.remote)
		assert.Equal(t, test.wantParent, gotParent, test.remote)
		assert.Equal(t, test.wantLeaf, gotLeaf, test.remote)
	}
}
