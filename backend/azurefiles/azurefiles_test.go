package azurefiles

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"path"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
)

func TestIntegration(t *testing.T) {
	// t.Skip("skipping because uploading files and setting time beind tested")
	var objPtr *Object
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestAzureFiles:",
		NilObject:  objPtr,
	})
}

var pre_existing_file_name = "preexistingfile.txt"
var pre_existing_dir = "pre_existing_dir"
var file_in_pre_existing_dir = "lorem.txt"
var pre_existing_file_contents = "This pre existing file has some content"

func TestNonCommonIntegration(t *testing.T) {
	// t.Skip("Skipping because we are working with integration tests from rclone")
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "TestAzureFiles:")
	assert.NoError(t, err)

	t.Run("new", newTests)

	if c, ok := f.(*Fs); ok {
		wrapAndPassC := func(fc func(*testing.T, *Fs)) func(*testing.T) {
			return func(t *testing.T) {
				fc(t, c)
			}
		}
		t.Run("NewObject Return error if object not found", wrapAndPassC(testNewObjectErrorOnObjectNotExisting))
		t.Run("NewObject does not return an error if file is found", wrapAndPassC(testNewObjectNoErrorIfObjectExists))
		t.Run("modTime", wrapAndPassC(testModTime))
		t.Run("put", wrapAndPassC(testPutObject))
		t.Run("list dir", wrapAndPassC(testListDir))
		t.Run("mkDir", wrapAndPassC(testMkDir))
		t.Run("rmDir", wrapAndPassC(testRmDir))
		t.Run("remove", wrapAndPassC(testRemove))
		t.Run("open", wrapAndPassC(testOpen))
		t.Run("update", wrapAndPassC(testUpdate))
		t.Run("walkAll", wrapAndPassC(testWalkAll))
		t.Run("encoding", wrapAndPassC(testEncoding))

	} else {
		t.Fatal("could not convert f to Client pointer")
	}
}

// TODO: test put object in an inner directory
func testPutObject(t *testing.T, f *Fs) {

	// TODO: correct hash is set

	name := randomString(10) + ".txt"
	in, src := randomPuttableObject(f, name)
	_, putErr := f.Put(context.TODO(), in, src)
	assert.NoError(t, putErr)
	obj, newObjErr := f.NewObject(context.TODO(), name)
	assert.NoError(t, newObjErr)
	t.Run("modtime is correctly set", func(t *testing.T) {
		expectedTime := src.properties.lastWriteTime
		gotTime := obj.ModTime(context.TODO())
		assert.Equal(t, expectedTime, gotTime, "modTime is correctly set")
	})

	assert.Equal(t, obj.Size(), src.properties.contentLength, "size is correctly set")

}

func testEncoding(t *testing.T, f *Fs) {
	t.Run("leading space", func(t *testing.T) {
		dirName := " " + randomString(10)
		err := f.Mkdir(context.TODO(), dirName)
		assert.NoError(t, err)
	})

	t.Run("punctuation", func(t *testing.T) {
		testingPunctionList := "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
		assert.NoError(t, f.Mkdir(context.TODO(), testingPunctionList))
		assert.NoError(t, f.Rmdir(context.TODO(), testingPunctionList))
		for idx, r := range testingPunctionList {
			t.Run(fmt.Sprintf("punctuation idx=%d", idx), func(t *testing.T) {
				nameBuilder := strings.Builder{}
				nameBuilder.WriteString("prefix_")
				nameBuilder.WriteRune(r)
				nameBuilder.WriteString(randomString(5))
				nameBuilder.WriteRune(r)
				dirName := nameBuilder.String()
				assert.NoError(t, f.Mkdir(context.TODO(), dirName))
				assert.NoError(t, f.Rmdir(context.TODO(), dirName))
			})
		}

	})
}

func testModTime(t *testing.T, f *Fs) {
	name := randomString(10)
	rdr, src := randomPuttableObject(f, name)
	_, err := f.Put(context.TODO(), rdr, src, nil)
	assert.NoError(t, err)
	obj, err := f.NewObject(context.TODO(), name)
	assert.NoError(t, err)
	timeBeingSet := randomTime()

	setModTimeErr := obj.SetModTime(context.TODO(), timeBeingSet)
	assert.NoError(t, setModTimeErr)

	t.Run("IsSetOnLocalObject", func(t *testing.T) {
		gotModTime := obj.ModTime(context.TODO())
		assert.Equal(t, timeBeingSet.UTC(), gotModTime.UTC())
	})

	t.Run("IsSetInCloud", func(t *testing.T) {
		newObj, newObjErr := f.NewObject(context.TODO(), name)
		assert.NoError(t, newObjErr)
		gotModTime := newObj.ModTime(context.TODO())
		assert.Equal(t, timeBeingSet.UTC(), gotModTime.UTC())
	})

}

// TODO: root should not be a file
func newTests(t *testing.T) {
	f, err := fs.NewFs(context.Background(), "TestAzureFiles:subdirAsRoot"+randomString(10))
	assert.NoError(t, err)

	azf := f.(*Fs)
	// entries, err := f.List(context.TODO(), "")
	// assert.NoError(t, err)
	// assert.Equal(t, 0, len(entries))

	// TODO: what happens if the directory already exists
	// TODO: what happens if file exists and a directory is being created
	t.Run("mkdir", func(t *testing.T) {
		assert.NotEmpty(t, f.Root())
		t.Run("no error when creating dir inside root", func(t *testing.T) {
			dirName := randomString(10)
			assert.NoError(t, f.Mkdir(context.Background(), dirName))
		})

		t.Run("no error when creating multilevel dir where parent does not exist", func(t *testing.T) {
			dirPath := path.Join(randomString(5), randomString(5))
			assert.NoError(t, f.Mkdir(context.Background(), dirPath))
		})

		t.Run("no error when creating multilevel dir where parent exists", func(t *testing.T) {
			// Setup: creating parent
			parent := randomString(5)
			assert.NoError(t, f.Mkdir(context.Background(), parent))

			dirPath := path.Join(parent, randomString(5))
			assert.NoError(t, f.Mkdir(context.Background(), dirPath))
		})

		t.Run("no error when directory already exists", func(t *testing.T) {
			// Setup: creating dir once
			dirPath := randomString(5)
			assert.NoError(t, f.Mkdir(context.Background(), dirPath))

			// creating directory second time
			assert.NoError(t, f.Mkdir(context.Background(), dirPath))
		})
	})

	t.Run("listdir", func(t *testing.T) {
		t.Run("dir does not exist", func(t *testing.T) {
			dirName := randomString(10)
			_, err := f.List(context.Background(), dirName)
			assert.Equal(t, fs.ErrorDirNotFound, err)
		})
		t.Run("both parent and dir dont exist", func(t *testing.T) {
			dirPath := path.Join(randomString(10), randomString(10))
			_, err := f.List(context.Background(), dirPath)
			assert.Equal(t, fs.ErrorDirNotFound, err)
		})
		t.Run("listdir works after mkdir in root", func(t *testing.T) {
			dirName := randomString(10)
			assert.NoError(t, f.Mkdir(context.Background(), dirName))
			entries, err := f.List(context.Background(), "")
			assert.NoError(t, err)
			assert.Contains(t, dirEntriesBases(entries), dirName)
		})

		t.Run("listdir works after mkdir on subdir where subdir's parent does not exist", func(t *testing.T) {
			parent := randomString(10)
			dirName := randomString(10)
			dirPath := path.Join(parent, dirName)
			assert.NoError(t, f.Mkdir(context.Background(), dirPath))
			entries, err := f.List(context.Background(), parent)
			assert.NoError(t, err)
			assert.Contains(t, dirEntriesBases(entries), dirName)
		})
	})

	t.Run("rmdir", func(t *testing.T) {
		t.Run("error when directory does not exist", func(t *testing.T) {
			dirName := randomString(10)
			assert.Error(t, f.Rmdir(context.TODO(), dirName))
		})

		t.Run("no error when directory exists", func(t *testing.T) {
			dirName := randomString(10)
			assert.NoError(t, f.Mkdir(context.TODO(), dirName))
			assert.NoError(t, f.Rmdir(context.TODO(), dirName))

			assert.Error(t, f.Rmdir(context.TODO(), dirName), "confirming that there is an error when directory does not exist")
		})

		t.Run("error when directory is not empty", func(t *testing.T) {
			parent := randomString(10)
			dirName := randomString(10)
			dirPath := path.Join(parent, dirName)
			assert.NoError(t, f.Mkdir(context.TODO(), dirPath))
			assert.Equal(t, fs.ErrorDirectoryNotEmpty, f.Rmdir(context.TODO(), parent))
		})

		t.Run("error when file is located at remote path not directory", func(t *testing.T) {
			filepath := randomString(10)
			r, obj := randomPuttableObject(azf, filepath)
			_, errOnPut := f.Put(context.TODO(), r, obj, nil)
			assert.NoError(t, errOnPut)
			assert.Error(t, f.Rmdir(context.TODO(), filepath))
		})
	})

	t.Run("put", func(t *testing.T) {

		t.Run("CorrectDataWritten", func(t *testing.T) {
			filename := randomString(10)
			r, src := randomPuttableObject(azf, filename)
			writtenBytes := []byte{}
			obj, putErr := f.Put(context.TODO(), io.TeeReader(r, bytes.NewBuffer(writtenBytes)), src)
			assert.NoError(t, putErr)

			readSeeker, openErr := obj.Open(context.TODO(), nil)
			assert.NoError(t, openErr)
			readBytes := []byte{}
			_, copyErr := io.Copy(bytes.NewBuffer(readBytes), readSeeker)
			assert.NoError(t, copyErr)
			assert.Equal(t, writtenBytes, readBytes)
		})
		t.Run("no errors when putting in root", func(t *testing.T) {
			filename := randomString(10)
			r, src := randomPuttableObject(azf, filename)
			_, err := f.Put(context.TODO(), r, src)
			assert.NoError(t, err)

			assertListDirEntriesContainsName(t, f, "", filename)
		})

		t.Run("no errors when putting in existant subdir", func(t *testing.T) {
			// Setup: creating the parent directory that exists before put
			parent := randomString(10)
			assert.NoError(t, f.Mkdir(context.TODO(), parent))

			fileName := randomString(10)
			filePath := path.Join(parent, fileName)
			r, src := randomPuttableObject(azf, filePath)
			_, err := f.Put(context.TODO(), r, src)
			assert.NoError(t, err)

			assertListDirEntriesContainsName(t, f, parent, fileName)

		})

		t.Run("no errors when putting in non existant subdir", func(t *testing.T) {
			parent := randomString(10)
			fileName := randomString(10)
			filePath := path.Join(parent, fileName)
			r, src := randomPuttableObject(azf, filePath)
			_, err := f.Put(context.TODO(), r, src)
			assert.NoError(t, err)

			assertListDirEntriesContainsName(t, f, parent, fileName)

		})

		// overwriting existing allowed as per meaning of HTTP PUT verb,
		// also because SFTP: backed allow overwritting
		t.Run("overwritesExistingFile", func(t *testing.T) {
			// Setup: putting a file
			filename := randomString(10)
			r, src := randomPuttableObject(azf, filename)
			_, err := f.Put(context.TODO(), r, src)
			assert.NoError(t, err)

			// Overwritting the previously put file
			newR, newSrc := randomPuttableObject(azf, filename)
			_, newPutErr := f.Put(context.TODO(), newR, newSrc)
			assert.NoError(t, newPutErr)
		})

		// SFTP: also returns an error when a file is put when a directory already exists
		t.Run("error when dir exists at that location", func(t *testing.T) {
			name := randomString(10)
			assert.NoError(t, f.Mkdir(context.TODO(), name))

			// now putting a file at the same location
			r, src := randomPuttableObject(azf, name)
			_, errPut := f.Put(context.TODO(), r, src)
			assert.Error(t, errPut)
		})

		t.Run("SizeModTimeMd5Hash", func(t *testing.T) {
			name := randomString(10)
			r, src := randomPuttableObject(azf, name)
			putRetVal, errPut := f.Put(context.TODO(), r, src)
			assert.NoError(t, errPut)

			newObj, err := f.NewObject(context.TODO(), name)
			assert.NoError(t, err)

			for idx, obj := range []fs.Object{putRetVal, newObj} {
				t.Run(fmt.Sprintf("%d", idx), func(t *testing.T) {
					o, ok := obj.(*Object)
					assert.True(t, ok)
					assert.Equal(t, src.Size(), o.contentLength)
					assert.Equal(t, src.Size(), o.Size())
					assert.Equal(t, src.ModTime(context.TODO()).UTC(), o.lastWriteTime.UTC())
					srcHash, err := src.Hash(context.TODO(), hash.MD5)
					assert.NoError(t, err, "hashing src should not result in error")
					assert.Equal(t, srcHash, hex.EncodeToString(o.md5Hash))
				})
			}
		})
	})

	t.Run("newfs", func(t *testing.T) {
		// Setup: creatinf file to test whether newFs returns error when a file is located at root
		parent := randomString(5)
		fileName := randomString(5)
		filePath := path.Join(parent, fileName)
		r, src := randomPuttableObject(azf, filePath)
		_, putErr := f.Put(context.TODO(), r, src, nil)
		assert.NoError(t, putErr)
		t.Run("ErrWhenFileExistsAtRoot", func(t *testing.T) {
			oldRoot := f.Root()
			newRoot := path.Join(oldRoot, filePath)
			_, newFsErr := fs.NewFs(context.Background(), "TestAzureFiles:"+newRoot)
			assert.Error(t, newFsErr)
		})
	})

	t.Run("NewObject", func(t *testing.T) {
		t.Run("returns ErrorObjectNotFound if directory exists at location", func(t *testing.T) {
			randomDir := randomString(10)
			assert.NoError(t, f.Mkdir(context.TODO(), randomDir))
			_, err := f.NewObject(context.TODO(), randomDir)
			assert.ErrorIs(t, err, fs.ErrorObjectNotFound)
		})

		t.Run("ErrorObjectNotFound", func(t *testing.T) {
			randomDir := randomString(10)
			_, err := f.NewObject(context.TODO(), randomDir)
			assert.ErrorIs(t, err, fs.ErrorObjectNotFound)
		})
	})

	t.Run("FsRootIsMultilevelAndDeep", func(t *testing.T) {
		pathOfDepth := func(d int) string {
			parts := []string{}
			for i := 0; i < d; i++ {
				parts = append(parts, randomString(10))
			}
			return path.Join(parts...)
		}
		for d := 0; d < 4; d++ {
			t.Run(fmt.Sprintf("Depth%d", d), func(t *testing.T) {
				path := pathOfDepth(d)
				t.Logf("path=%s", path)
				f, err := fs.NewFs(context.Background(), "TestAzureFiles:"+path)
				assert.NoError(t, err)
				assert.NoError(t, f.Mkdir(context.TODO(), randomString(10)))
			})

		}
	})

	t.Run("update", func(t *testing.T) {
		name := randomString(10)
		rdr, src := randomPuttableObject(azf, name)

		obj, err := f.Put(context.TODO(), rdr, src, nil)
		assert.NoError(t, err)

		smallerSize := src.Size() - rand.Int63n(src.Size()/2)
		largerSize := src.Size() - rand.Int63n(100)
		for idx, updatedSize := range []int64{smallerSize, largerSize} {
			t.Run(fmt.Sprintf("idx=%d updatedSize=%d origSize=%d", idx, updatedSize, src.Size()), func(t *testing.T) {
				updatedRdr, updatedSrc := randomPuttableObjectWithSize(azf, name, updatedSize)
				updatedBytes := []byte{}
				teedRdr := io.TeeReader(updatedRdr, bytes.NewBuffer(updatedBytes))
				assert.NoError(t, obj.Update(context.TODO(), teedRdr, updatedSrc, nil))

				readCloser, openErr := obj.Open(context.TODO(), nil)
				assert.NoError(t, openErr)
				readBytes := []byte{}
				nBsCopied, copyErr := io.Copy(bytes.NewBuffer(readBytes), readCloser)
				assert.NoError(t, copyErr)
				assert.Equal(t, updatedSize, nBsCopied)
				assert.Equal(t, updatedBytes, readBytes)
			})
		}
		// TODO: are size,modTime and MD5 updated
	})

}

func assertListDirEntriesContainsName(t *testing.T, f fs.Fs, dir string, name string) {
	entries, listErr := f.List(context.TODO(), dir)
	assert.NoError(t, listErr)
	assert.Contains(t, dirEntriesBases(entries), name)
}

func TestNewFsWithAccountAndKey(t *testing.T) {
	opt := &Options{
		ShareName: "test-rclone-sep-2023",
	}
	fs, err := newFsFromOptions(context.TODO(), "TestAzureFiles", "", opt)
	assert.NoError(t, err)
	dirName := randomString(10)
	assert.NoError(t, fs.Mkdir(context.TODO(), dirName))
	assertListDirEntriesContainsName(t, fs, "", dirName)
}
