package sftp

import (
	"os"

	"github.com/stretchr/testify/assert"

	"testing"
)

func TestRequestPflags(t *testing.T) {
	pflags := newPflags(ssh_FXF_READ | ssh_FXF_WRITE | ssh_FXF_APPEND)
	assert.True(t, pflags.Read)
	assert.True(t, pflags.Write)
	assert.True(t, pflags.Append)
	assert.False(t, pflags.Creat)
	assert.False(t, pflags.Trunc)
	assert.False(t, pflags.Excl)
}

func TestRequestAflags(t *testing.T) {
	aflags := newAflags(ssh_FILEXFER_ATTR_SIZE | ssh_FILEXFER_ATTR_UIDGID)
	assert.True(t, aflags.Size)
	assert.True(t, aflags.UidGid)
	assert.False(t, aflags.Acmodtime)
	assert.False(t, aflags.Permissions)
}

func TestRequestAttributes(t *testing.T) {
	// UID/GID
	fa := fileattrs{UID: 1, GID: 2}
	fl := uint32(ssh_FILEXFER_ATTR_UIDGID)
	at := []byte{}
	at = marshalUint32(at, 1)
	at = marshalUint32(at, 2)
	test_fs, _ := getFileStat(fl, at)
	assert.Equal(t, fa, fileattrs(*test_fs))
	// Size and Mode
	fa = fileattrs{Mode: 700, Size: 99}
	fl = uint32(ssh_FILEXFER_ATTR_SIZE | ssh_FILEXFER_ATTR_PERMISSIONS)
	at = []byte{}
	at = marshalUint64(at, 99)
	at = marshalUint32(at, 700)
	test_fs, _ = getFileStat(fl, at)
	test_fa := fileattrs(*test_fs)
	assert.Equal(t, fa, test_fa)
	// FileMode
	assert.True(t, test_fa.FileMode().IsRegular())
	assert.False(t, test_fa.FileMode().IsDir())
	assert.Equal(t, test_fa.FileMode().Perm(), os.FileMode(700).Perm())
}
