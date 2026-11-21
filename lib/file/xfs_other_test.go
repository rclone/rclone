//go:build !linux || mips || mipsle || mips64 || mips64le || ppc64 || ppc64le

package file

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestXFSUnsupportedPlatform(t *testing.T) {
	got, err := IsXFS(t.TempDir())
	assert.False(t, got)
	assert.NoError(t, err)
	token, err := StartXFSCommit(nil)
	assert.Nil(t, token)
	assert.ErrorIs(t, err, ErrXFSCommitRangeUnsupported)
	assert.ErrorIs(t, CommitXFSRange(nil, nil, nil, false), ErrXFSCommitRangeUnsupported)
	assert.ErrorIs(t, CommitXFSRange(nil, nil, nil, true), ErrXFSCommitRangeUnsupported)
}
