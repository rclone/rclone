package tree

import (
	"bytes"
	"testing"

	"github.com/a8m/tree"
	"github.com/ncw/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ncw/rclone/local"
)

func TestTree(t *testing.T) {
	buf := new(bytes.Buffer)
	// Never ask for passwords, fail instead.
	// If your local config is encrypted set environment variable
	// "RCLONE_CONFIG_PASS=hunter2" (or your password)
	*fs.AskPassword = false
	fs.LoadConfig()
	f, err := fs.NewFs("testfiles")
	require.NoError(t, err)
	err = Tree(f, buf, new(tree.Options))
	require.NoError(t, err)
	assert.Equal(t, `/
├── file1
├── file2
├── file3
└── subdir
    ├── file4
    └── file5

1 directories, 5 files
`, buf.String())
}
