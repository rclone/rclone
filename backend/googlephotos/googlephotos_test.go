package googlephotos

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// We have two different files here as Google Photos will uniq
	// them otherwise which confuses the tests as the filename is
	// unexpected.
	fileNameAlbum  = "rclone-test-image1.jpg"
	fileNameUpload = "rclone-test-image2.jpg"
)

func TestIntegration(t *testing.T) {
	ctx := context.Background()
	fstest.Initialise()

	// Create Fs
	if *fstest.RemoteName == "" {
		*fstest.RemoteName = "TestGooglePhotos:"
	}
	f, err := fs.NewFs(ctx, *fstest.RemoteName)
	if err == fs.ErrorNotFoundInConfigFile {
		t.Skipf("Couldn't create google photos backend - skipping tests: %v", err)
	}
	require.NoError(t, err)

	// Create local Fs pointing at testfiles
	localFs, err := fs.NewFs(ctx, "testfiles")
	require.NoError(t, err)

	t.Run("CreateAlbum", func(t *testing.T) {
		albumName := "album/rclone-test-" + random.String(24)
		err = f.Mkdir(ctx, albumName)
		require.NoError(t, err)
		remote := albumName + "/" + fileNameAlbum

		t.Run("PutFile", func(t *testing.T) {
			srcObj, err := localFs.NewObject(ctx, fileNameAlbum)
			require.NoError(t, err)
			in, err := srcObj.Open(ctx)
			require.NoError(t, err)
			dstObj, err := f.Put(ctx, in, operations.NewOverrideRemote(srcObj, remote))
			require.NoError(t, err)
			assert.Equal(t, remote, dstObj.Remote())
			_ = in.Close()
			remoteWithID := addFileID(remote, dstObj.(*Object).id)

			t.Run("ObjectFs", func(t *testing.T) {
				assert.Equal(t, f, dstObj.Fs())
			})

			t.Run("ObjectString", func(t *testing.T) {
				assert.Equal(t, remote, dstObj.String())
				assert.Equal(t, "<nil>", (*Object)(nil).String())
			})

			t.Run("ObjectHash", func(t *testing.T) {
				h, err := dstObj.Hash(ctx, hash.MD5)
				assert.Equal(t, "", h)
				assert.Equal(t, hash.ErrUnsupported, err)
			})

			t.Run("ObjectSize", func(t *testing.T) {
				assert.Equal(t, int64(-1), dstObj.Size())
				f.(*Fs).opt.ReadSize = true
				defer func() {
					f.(*Fs).opt.ReadSize = false
				}()
				size := dstObj.Size()
				assert.True(t, size > 1000, fmt.Sprintf("Size too small %d", size))
			})

			t.Run("ObjectSetModTime", func(t *testing.T) {
				err := dstObj.SetModTime(ctx, time.Now())
				assert.Equal(t, fs.ErrorCantSetModTime, err)
			})

			t.Run("ObjectStorable", func(t *testing.T) {
				assert.True(t, dstObj.Storable())
			})

			t.Run("ObjectOpen", func(t *testing.T) {
				in, err := dstObj.Open(ctx)
				require.NoError(t, err)
				buf, err := ioutil.ReadAll(in)
				require.NoError(t, err)
				require.NoError(t, in.Close())
				assert.True(t, len(buf) > 1000)
				contentType := http.DetectContentType(buf[:512])
				assert.Equal(t, "image/jpeg", contentType)
			})

			t.Run("CheckFileInAlbum", func(t *testing.T) {
				entries, err := f.List(ctx, albumName)
				require.NoError(t, err)
				assert.Equal(t, 1, len(entries))
				assert.Equal(t, remote, entries[0].Remote())
				assert.Equal(t, "2013-07-26 08:57:21 +0000 UTC", entries[0].ModTime(ctx).String())
			})

			// Check it is there in the date/month/year hierarchy
			// 2013-07-13 is the creation date of the folder
			checkPresent := func(t *testing.T, objPath string) {
				entries, err := f.List(ctx, objPath)
				require.NoError(t, err)
				found := false
				for _, entry := range entries {
					leaf := path.Base(entry.Remote())
					if leaf == fileNameAlbum || leaf == remoteWithID {
						found = true
					}
				}
				assert.True(t, found, fmt.Sprintf("didn't find %q in %q", fileNameAlbum, objPath))
			}

			t.Run("CheckInByYear", func(t *testing.T) {
				checkPresent(t, "media/by-year/2013")
			})

			t.Run("CheckInByMonth", func(t *testing.T) {
				checkPresent(t, "media/by-month/2013/2013-07")
			})

			t.Run("CheckInByDay", func(t *testing.T) {
				checkPresent(t, "media/by-day/2013/2013-07-26")
			})

			t.Run("NewObject", func(t *testing.T) {
				o, err := f.NewObject(ctx, remote)
				require.NoError(t, err)
				require.Equal(t, remote, o.Remote())
			})

			t.Run("NewObjectWithID", func(t *testing.T) {
				o, err := f.NewObject(ctx, remoteWithID)
				require.NoError(t, err)
				require.Equal(t, remoteWithID, o.Remote())
			})

			t.Run("NewFsIsFile", func(t *testing.T) {
				fNew, err := fs.NewFs(ctx, *fstest.RemoteName+remote)
				assert.Equal(t, fs.ErrorIsFile, err)
				leaf := path.Base(remote)
				o, err := fNew.NewObject(ctx, leaf)
				require.NoError(t, err)
				require.Equal(t, leaf, o.Remote())
			})

			t.Run("RemoveFileFromAlbum", func(t *testing.T) {
				err = dstObj.Remove(ctx)
				require.NoError(t, err)

				time.Sleep(time.Second)

				// Check album empty
				entries, err := f.List(ctx, albumName)
				require.NoError(t, err)
				assert.Equal(t, 0, len(entries))
			})
		})

		// remove the album
		err = f.Rmdir(ctx, albumName)
		require.Error(t, err) // FIXME doesn't work yet
	})

	t.Run("UploadMkdir", func(t *testing.T) {
		assert.NoError(t, f.Mkdir(ctx, "upload/dir"))
		assert.NoError(t, f.Mkdir(ctx, "upload/dir/subdir"))

		t.Run("List", func(t *testing.T) {
			entries, err := f.List(ctx, "upload")
			require.NoError(t, err)
			assert.Equal(t, 1, len(entries))
			assert.Equal(t, "upload/dir", entries[0].Remote())

			entries, err = f.List(ctx, "upload/dir")
			require.NoError(t, err)
			assert.Equal(t, 1, len(entries))
			assert.Equal(t, "upload/dir/subdir", entries[0].Remote())
		})

		t.Run("Rmdir", func(t *testing.T) {
			assert.NoError(t, f.Rmdir(ctx, "upload/dir/subdir"))
			assert.NoError(t, f.Rmdir(ctx, "upload/dir"))

		})

		t.Run("ListEmpty", func(t *testing.T) {
			entries, err := f.List(ctx, "upload")
			require.NoError(t, err)
			assert.Equal(t, 0, len(entries))

			_, err = f.List(ctx, "upload/dir")
			assert.Equal(t, fs.ErrorDirNotFound, err)
		})
	})

	t.Run("Upload", func(t *testing.T) {
		uploadDir := "upload/dir/subdir"
		remote := path.Join(uploadDir, fileNameUpload)

		srcObj, err := localFs.NewObject(ctx, fileNameUpload)
		require.NoError(t, err)
		in, err := srcObj.Open(ctx)
		require.NoError(t, err)
		dstObj, err := f.Put(ctx, in, operations.NewOverrideRemote(srcObj, remote))
		require.NoError(t, err)
		assert.Equal(t, remote, dstObj.Remote())
		_ = in.Close()
		remoteWithID := addFileID(remote, dstObj.(*Object).id)

		t.Run("List", func(t *testing.T) {
			entries, err := f.List(ctx, uploadDir)
			require.NoError(t, err)
			require.Equal(t, 1, len(entries))
			assert.Equal(t, remote, entries[0].Remote())
			assert.Equal(t, "2013-07-26 08:57:21 +0000 UTC", entries[0].ModTime(ctx).String())
		})

		t.Run("NewObject", func(t *testing.T) {
			o, err := f.NewObject(ctx, remote)
			require.NoError(t, err)
			require.Equal(t, remote, o.Remote())
		})

		t.Run("NewObjectWithID", func(t *testing.T) {
			o, err := f.NewObject(ctx, remoteWithID)
			require.NoError(t, err)
			require.Equal(t, remoteWithID, o.Remote())
		})

	})

	t.Run("Name", func(t *testing.T) {
		assert.Equal(t, (*fstest.RemoteName)[:len(*fstest.RemoteName)-1], f.Name())
	})

	t.Run("Root", func(t *testing.T) {
		assert.Equal(t, "", f.Root())
	})

	t.Run("String", func(t *testing.T) {
		assert.Equal(t, `Google Photos path ""`, f.String())
	})

	t.Run("Features", func(t *testing.T) {
		features := f.Features()
		assert.False(t, features.CaseInsensitive)
		assert.True(t, features.ReadMimeType)
	})

	t.Run("Precision", func(t *testing.T) {
		assert.Equal(t, fs.ModTimeNotSupported, f.Precision())
	})

	t.Run("Hashes", func(t *testing.T) {
		assert.Equal(t, hash.Set(hash.None), f.Hashes())
	})

}

func TestAddID(t *testing.T) {
	assert.Equal(t, "potato {123}", addID("potato", "123"))
	assert.Equal(t, "{123}", addID("", "123"))
}

func TestFileAddID(t *testing.T) {
	assert.Equal(t, "potato {123}.txt", addFileID("potato.txt", "123"))
	assert.Equal(t, "potato {123}", addFileID("potato", "123"))
	assert.Equal(t, "{123}", addFileID("", "123"))
}

func TestFindID(t *testing.T) {
	assert.Equal(t, "", findID("potato"))
	ID := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	assert.Equal(t, ID, findID("potato {"+ID+"}.txt"))
	ID = ID[1:]
	assert.Equal(t, "", findID("potato {"+ID+"}.txt"))
}
