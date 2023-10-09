package azurefiles

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
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
		t.Run("setModTime", wrapAndPassC(testSetModTime))
		t.Run("modTime", wrapAndPassC(testModTime))
		t.Run("put", wrapAndPassC(testPutObject))
		t.Run("list dir", wrapAndPassC(testListDir))
		t.Run("mkDir", wrapAndPassC(testMkDir))
		t.Run("rmDir", wrapAndPassC(testRmDir))
		t.Run("remove", wrapAndPassC(testRemove))
		t.Run("open", wrapAndPassC(testOpen))
		t.Run("update", wrapAndPassC(testUpdate))
		t.Run("walkAll", wrapAndPassC(testWalkAll))
		t.Run("set properties", wrapAndPassC(testSettingMetaDataWorks))

		t.Run("encoding", wrapAndPassC(testEncoding))

	} else {
		t.Fatal("could not convert f to Client pointer")
	}
}

// TODO: test put object in an inner directory
func testPutObject(t *testing.T, f *Fs) {

	// TODO: correct hash is set

	name := RandomString(10) + ".txt"
	in, src := RandomPuttableObject(name)
	_, putErr := f.Put(context.TODO(), in, src)
	assert.NoError(t, putErr)
	obj, newObjErr := f.NewObject(context.TODO(), name)
	assert.NoError(t, newObjErr)
	t.Run("modtime is correctly set", func(t *testing.T) {
		expectedUnix, err := strconv.ParseInt(*src.metaData[modTimeKey], 10, 64)
		assert.NoError(t, err)
		expectedTime := time.Unix(expectedUnix, 0)
		gotTime := obj.ModTime(context.TODO())
		assert.Equal(t, expectedTime, gotTime, "modTime is correctly set")
	})

	assert.Equal(t, obj.Size(), *src.properties.contentLength, "size is correctly set")

}

func testSettingMetaDataWorks(t *testing.T, c *Fs) {
	fcSetting := c.shareRootDirClient.NewFileClient(pre_existing_file_name)
	metaData := make(map[string]*string)
	someString := "1_isgreat"
	metaData["a"] = &someString
	metaDataOptions := file.SetMetadataOptions{
		Metadata: metaData,
	}
	resp, err := fcSetting.SetMetadata(context.TODO(), &metaDataOptions)
	assert.NoError(t, err)
	t.Log(resp)

	// Now checking whether the metadata was actually set

	fcGetting := c.shareRootDirClient.NewFileClient(pre_existing_file_name)
	getResp, getErr := fcGetting.GetProperties(context.TODO(), nil)
	assert.NoError(t, getErr)
	actualPtr, ok := getCaseInvariantMetaDataValue(getResp.Metadata, "a")
	assert.True(t, ok)
	assert.Equal(t, someString, *actualPtr)

}

func testSetModTime(t *testing.T, f *Fs) {
	obj, err := f.NewObject(context.TODO(), pre_existing_file_name)
	assert.NoError(t, err)
	timeBeingSet := randomTime()
	setModTimeErr := obj.SetModTime(context.TODO(), timeBeingSet)
	assert.NoError(t, setModTimeErr)

	fc := f.shareRootDirClient.NewFileClient(pre_existing_file_name)
	res, getPropErr := fc.GetProperties(context.TODO(), nil)
	assert.NoError(t, getPropErr)
	gotTimeStr, ok := getCaseInvariantMetaDataValue(res.Metadata, modTimeKey)
	assert.True(t, ok)
	gotTimeInt, err := strconv.ParseInt(*gotTimeStr, 10, 64)
	assert.NoError(t, err)
	gotTime := time.Unix(gotTimeInt, 0)

	assert.Equal(t, timeBeingSet, gotTime)
}

func testEncoding(t *testing.T, f *Fs) {
	t.Run("leading space", func(t *testing.T) {
		dirName := " " + RandomString(10)
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
				nameBuilder.WriteString(RandomString(5))
				nameBuilder.WriteRune(r)
				dirName := nameBuilder.String()
				assert.NoError(t, f.Mkdir(context.TODO(), dirName))
				assert.NoError(t, f.Rmdir(context.TODO(), dirName))
			})
		}

	})
}

func testModTime(t *testing.T, f *Fs) {
	obj, err := f.NewObject(context.TODO(), pre_existing_file_name)
	assert.NoError(t, err)
	timeBeingSet := randomTime()

	sleepDuration := time.Second * 3
	t.Logf("sleeping for %s", sleepDuration)
	time.Sleep(sleepDuration)

	setModTimeErr := obj.SetModTime(context.TODO(), timeBeingSet)
	assert.NoError(t, setModTimeErr)

	t.Logf("sleeping for %s", sleepDuration)
	time.Sleep(sleepDuration)

	gotModTime := obj.ModTime(context.TODO())
	assert.Equal(t, timeBeingSet, gotModTime)
}

func TestCaseInvariantGetMetaDataValue(t *testing.T) {

	t.Run("returns the correct value for case invariant key", func(t *testing.T) {
		md := make(map[string]*string)
		expectedValue := RandomString(10)
		md["key"] = &expectedValue
		gotValue, ok := getCaseInvariantMetaDataValue(md, "KEY")
		assert.True(t, ok)
		assert.Equal(t, expectedValue, *gotValue)
	})

	t.Run("return false if key does not exist in map", func(t *testing.T) {
		md := make(map[string]*string)
		_, ok := getCaseInvariantMetaDataValue(md, "KEY")
		assert.False(t, ok)
	})

}

func TestCaseInvariantSetMetaDataValue(t *testing.T) {
	md := make(map[string]*string)
	setCaseInvariantMetaDataValue(md, "key", "lowercase")
	setCaseInvariantMetaDataValue(md, "KEY", "uppercase")

	gotValue, ok := getCaseInvariantMetaDataValue(md, "key")
	assert.True(t, ok)
	assert.Equal(t, "uppercase", *gotValue)
}

// TODO: root should not be a file
func newTests(t *testing.T) {
	f, err := fs.NewFs(context.Background(), "TestAzureFiles:subdirAsRoot"+RandomString(10))
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
			dirName := RandomString(10)
			assert.NoError(t, f.Mkdir(context.Background(), dirName))
		})

		t.Run("no error when creating multilevel dir where parent does not exist", func(t *testing.T) {
			dirPath := path.Join(RandomString(5), RandomString(5))
			assert.NoError(t, f.Mkdir(context.Background(), dirPath))
		})

		t.Run("no error when creating multilevel dir where parent exists", func(t *testing.T) {
			// Setup: creating parent
			parent := RandomString(5)
			assert.NoError(t, f.Mkdir(context.Background(), parent))

			dirPath := path.Join(parent, RandomString(5))
			assert.NoError(t, f.Mkdir(context.Background(), dirPath))
		})

		t.Run("no error when directory already exists", func(t *testing.T) {
			// Setup: creating dir once
			dirPath := RandomString(5)
			assert.NoError(t, f.Mkdir(context.Background(), dirPath))

			// creating directory second time
			assert.NoError(t, f.Mkdir(context.Background(), dirPath))
		})
	})

	t.Run("listdir", func(t *testing.T) {
		t.Run("dir does not exist", func(t *testing.T) {
			dirName := RandomString(10)
			_, err := f.List(context.Background(), dirName)
			assert.Equal(t, fs.ErrorDirNotFound, err)
		})
		t.Run("both parent and dir dont exist", func(t *testing.T) {
			dirPath := path.Join(RandomString(10), RandomString(10))
			_, err := f.List(context.Background(), dirPath)
			assert.Equal(t, fs.ErrorDirNotFound, err)
		})
		t.Run("listdir works after mkdir in root", func(t *testing.T) {
			dirName := RandomString(10)
			assert.NoError(t, f.Mkdir(context.Background(), dirName))
			entries, err := f.List(context.Background(), "")
			assert.NoError(t, err)
			assert.Contains(t, dirEntriesBases(entries), dirName)
		})

		t.Run("listdir works after mkdir on subdir where subdir's parent does not exist", func(t *testing.T) {
			parent := RandomString(10)
			dirName := RandomString(10)
			dirPath := path.Join(parent, dirName)
			assert.NoError(t, f.Mkdir(context.Background(), dirPath))
			entries, err := f.List(context.Background(), parent)
			assert.NoError(t, err)
			assert.Contains(t, dirEntriesBases(entries), dirName)
		})
	})

	t.Run("rmdir", func(t *testing.T) {
		t.Run("error when directory does not exist", func(t *testing.T) {
			dirName := RandomString(10)
			assert.Error(t, f.Rmdir(context.TODO(), dirName))
		})

		t.Run("no error when directory exists", func(t *testing.T) {
			dirName := RandomString(10)
			assert.NoError(t, f.Mkdir(context.TODO(), dirName))
			assert.NoError(t, f.Rmdir(context.TODO(), dirName))

			assert.Error(t, f.Rmdir(context.TODO(), dirName), "confirming that there is an error when directory does not exist")
		})

		t.Run("error when directory is not empty", func(t *testing.T) {
			parent := RandomString(10)
			dirName := RandomString(10)
			dirPath := path.Join(parent, dirName)
			assert.NoError(t, f.Mkdir(context.TODO(), dirPath))
			assert.Equal(t, fs.ErrorDirectoryNotEmpty, f.Rmdir(context.TODO(), parent))
		})

		t.Run("file is located at remote path not directory", func(t *testing.T) {
			assert.Equal(t, 1, 2)
		})
	})

	t.Run("put", func(t *testing.T) {
		t.Run("no errors when putting in root", func(t *testing.T) {
			filename := RandomString(10)
			r, src := RandomPuttableObject(filename)
			_, err := f.Put(context.TODO(), r, src)
			assert.NoError(t, err)

			assertListDirEntriesContainsName(t, f, "", filename)
		})

		t.Run("no errors when putting in existant subdir", func(t *testing.T) {
			// Setup: creating the parent directory that exists before put
			parent := RandomString(10)
			assert.NoError(t, f.Mkdir(context.TODO(), parent))

			fileName := RandomString(10)
			filePath := path.Join(parent, fileName)
			r, src := RandomPuttableObject(filePath)
			_, err := f.Put(context.TODO(), r, src)
			assert.NoError(t, err)

			assertListDirEntriesContainsName(t, f, parent, fileName)

		})

		t.Run("no errors when putting in non existant subdir", func(t *testing.T) {
			parent := RandomString(10)
			fileName := RandomString(10)
			filePath := path.Join(parent, fileName)
			r, src := RandomPuttableObject(filePath)
			_, err := f.Put(context.TODO(), r, src)
			assert.NoError(t, err)

			assertListDirEntriesContainsName(t, f, parent, fileName)

		})

		// overwriting existing allowed as per meaning of HTTP PUT verb,
		// also because SFTP: backed allow overwritting
		t.Run("overwritesExistingFile", func(t *testing.T) {
			// Setup: putting a file
			filename := RandomString(10)
			r, src := RandomPuttableObject(filename)
			_, err := f.Put(context.TODO(), r, src)
			assert.NoError(t, err)

			// Overwritting the previously put file
			newR, newSrc := RandomPuttableObject(filename)
			_, newPutErr := f.Put(context.TODO(), newR, newSrc)
			assert.NoError(t, newPutErr)
		})

		// SFTP: also returns an error when a file is put when a directory already exists
		t.Run("error when dir exists at that location", func(t *testing.T) {
			name := RandomString(10)
			assert.NoError(t, f.Mkdir(context.TODO(), name))

			// now putting a file at the same location
			r, src := RandomPuttableObject(name)
			_, errPut := f.Put(context.TODO(), r, src)
			assert.Error(t, errPut)
		})
	})

	t.Run("newfs", func(t *testing.T) {
		// Setup: creatinf file to test whether newFs returns error when a file is located at root
		parent := RandomString(5)
		fileName := RandomString(5)
		filePath := path.Join(parent, fileName)
		r, src := RandomPuttableObject(filePath)
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
		t.Run("returns ErrorIsDir if directory found", func(t *testing.T) {
			randomDir := RandomString(10)
			assert.NoError(t, f.Mkdir(context.TODO(), randomDir))
			_, err := f.NewObject(context.TODO(), randomDir)
			assert.ErrorIs(t, err, fs.ErrorIsDir)

		})
	})

	t.Run("isDirectory", func(t *testing.T) {
		t.Run("DirExists", func(t *testing.T) {
			randomPath := RandomString(10)
			assert.NoError(t, f.Mkdir(context.TODO(), randomPath))
			isDir, isDirErr := azf.isDirectory(context.TODO(), randomPath)
			assert.NoError(t, isDirErr)
			assert.True(t, isDir)
		})
		t.Run("FileExists", func(t *testing.T) {
			randomPath := RandomString(10)
			r, obj := RandomPuttableObject(randomPath)
			_, putErr := f.Put(context.TODO(), r, obj, nil)
			assert.NoError(t, putErr)
			_, isDirErr := azf.isDirectory(context.TODO(), randomPath)
			assert.Error(t, isDirErr)
		})
		t.Run("PathDoesNotExist", func(t *testing.T) {
			randomPath := RandomString(10)
			_, isDirErr := azf.isDirectory(context.TODO(), randomPath)
			assert.Error(t, isDirErr)
		})
	})
	t.Run("CopyFile", func(t *testing.T) {
		randomName := RandomString(10)
		data, srcForPutting := RandomPuttableObject(randomName)
		srcObj, errPut := f.Put(context.TODO(), data, srcForPutting, nil)
		assert.NoError(t, errPut)

		destPath := path.Join(RandomString(10), RandomString(10))

		destObj, errCopy := azf.CopyFile(context.TODO(), srcObj, destPath)
		assert.NoError(t, errCopy)
		srcHash, srcHashErr := srcObj.Hash(context.TODO(), hash.MD5)
		assert.NoError(t, srcHashErr)
		destHash, destHashErr := destObj.Hash(context.TODO(), hash.MD5)
		assert.NoError(t, destHashErr)
		assert.Equal(t, srcHash, destHash)
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
	dirName := RandomString(10)
	assert.NoError(t, fs.Mkdir(context.TODO(), dirName))
	assertListDirEntriesContainsName(t, fs, "", dirName)
}
