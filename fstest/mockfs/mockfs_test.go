package mockfs

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/mockdir"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This lists dir and checks the listing is as expected
func checkList(t *testing.T, mfs *Fs, dir string, want []string) {
	var got []string
	ctx := context.Background()
	entries, err := mfs.List(ctx, dir)
	require.NoError(t, err)
	for _, entry := range entries {
		got = append(got, entry.String())
	}
	assert.Equal(t, want, got)
}

func NewMockfs(t *testing.T) (mfs *Fs, ctx context.Context) {
	ctx = context.Background()
	testfs, err := NewFs(ctx, "test", "root", nil)
	require.NoError(t, err)
	mfs = testfs.(*Fs)
	return mfs, ctx
}

func TestList(t *testing.T) {
	mfs, ctx := NewMockfs(t)

	checkList(t, mfs, "", nil)

	mfs.AddObject(mockobject.New("main.go"))
	mfs.AddDir(mockdir.New("internal"))
	mfs.AddObject(mockobject.New("internal/pkg1"))
	mfs.AddObject(mockobject.New("internal/pkg2"))

	checkList(t, mfs, "", []string{"main.go", "internal"})
	checkList(t, mfs, "internal", []string{"internal/pkg1", "internal/pkg2"})

	_, err := mfs.List(ctx, "interna")
	require.ErrorIs(t, err, fs.ErrorDirNotFound)
	_, err = mfs.List(ctx, "main.go")
	require.ErrorIs(t, err, fs.ErrorDirNotFound)
	_, err = mfs.List(ctx, "internal/") // trailing slash not allowed
	require.ErrorIs(t, err, fs.ErrorDirNotFound)
}

func checkNewObject(t *testing.T, mfs *Fs, obj fs.Object) {
	ctx := context.Background()
	got, err := mfs.NewObject(ctx, obj.String())
	require.NoError(t, err)
	require.Equal(t, obj, got)
}


func TestNewObject(t *testing.T) {
	mfs, ctx := NewMockfs(t)

	main := mockobject.New("main.go")
	internal := mockdir.New("internal")
	pkg1 := mockobject.New("internal/pkg1")

	mfs.AddObject(main)
	mfs.AddDir(internal)
	mfs.AddObject(pkg1)

	checkNewObject(t, mfs, main)
	checkNewObject(t, mfs, pkg1)

	_, err := mfs.NewObject(ctx, "internal")
	require.ErrorIs(t, err, fs.ErrorIsDir)

	_, err = mfs.NewObject(ctx, "intern")
	require.ErrorIs(t, err, fs.ErrorObjectNotFound)
}
