package operations_test

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListDirSorted is integration testing code in fs/list/list.go
// which can't be tested there due to import loops.
func TestListDirSorted(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx := context.Background()
	fi := filter.GetConfig(ctx)
	fi.Opt.MaxSize = 10
	defer func() {
		fi.Opt.MaxSize = -1
	}()

	files := []fstest.Item{
		r.WriteObject(context.Background(), "a.txt", "hello world", t1),
		r.WriteObject(context.Background(), "zend.txt", "hello", t1),
		r.WriteObject(context.Background(), "sub dir/hello world", "hello world", t1),
		r.WriteObject(context.Background(), "sub dir/hello world2", "hello world", t1),
		r.WriteObject(context.Background(), "sub dir/ignore dir/.ignore", "-", t1),
		r.WriteObject(context.Background(), "sub dir/ignore dir/should be ignored", "to ignore", t1),
		r.WriteObject(context.Background(), "sub dir/sub sub dir/hello world3", "hello world", t1),
	}
	r.CheckRemoteItems(t, files...)
	var items fs.DirEntries
	var err error

	// Turn the DirEntry into a name, ending with a / if it is a
	// dir
	str := func(i int) string {
		item := items[i]
		name := item.Remote()
		switch item.(type) {
		case fs.Object:
		case fs.Directory:
			name += "/"
		default:
			t.Fatalf("Unknown type %+v", item)
		}
		return name
	}

	items, err = list.DirSorted(context.Background(), r.Fremote, true, "")
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, "a.txt", str(0))
	assert.Equal(t, "sub dir/", str(1))
	assert.Equal(t, "zend.txt", str(2))

	items, err = list.DirSorted(context.Background(), r.Fremote, false, "")
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "sub dir/", str(0))
	assert.Equal(t, "zend.txt", str(1))

	items, err = list.DirSorted(context.Background(), r.Fremote, true, "sub dir")
	require.NoError(t, err)
	require.Len(t, items, 4)
	assert.Equal(t, "sub dir/hello world", str(0))
	assert.Equal(t, "sub dir/hello world2", str(1))
	assert.Equal(t, "sub dir/ignore dir/", str(2))
	assert.Equal(t, "sub dir/sub sub dir/", str(3))

	items, err = list.DirSorted(context.Background(), r.Fremote, false, "sub dir")
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "sub dir/ignore dir/", str(0))
	assert.Equal(t, "sub dir/sub sub dir/", str(1))

	// testing ignore file
	fi.Opt.ExcludeFile = []string{".ignore"}

	items, err = list.DirSorted(context.Background(), r.Fremote, false, "sub dir")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "sub dir/sub sub dir/", str(0))

	items, err = list.DirSorted(context.Background(), r.Fremote, false, "sub dir/ignore dir")
	require.NoError(t, err)
	require.Len(t, items, 0)

	items, err = list.DirSorted(context.Background(), r.Fremote, true, "sub dir/ignore dir")
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "sub dir/ignore dir/.ignore", str(0))
	assert.Equal(t, "sub dir/ignore dir/should be ignored", str(1))

	fi.Opt.ExcludeFile = nil
	items, err = list.DirSorted(context.Background(), r.Fremote, false, "sub dir/ignore dir")
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "sub dir/ignore dir/.ignore", str(0))
	assert.Equal(t, "sub dir/ignore dir/should be ignored", str(1))
}
