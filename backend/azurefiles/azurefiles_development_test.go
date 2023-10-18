package azurefiles

import (
	"bytes"
	"context"
	"crypto/md5"
	"io"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/walk"
	"github.com/stretchr/testify/assert"
)

// TODO: new object dir cases
// TODO: set modtime on directories

func testNewObjectErrorOnObjectNotExisting(t *testing.T, c *Fs) {
	_, err := c.NewObject(context.TODO(), "somefilethatdoesnotexist.txt")
	assert.Error(t, err)
}

func testNewObjectNoErrorIfObjectExists(t *testing.T, c *Fs) {
	_, err := c.NewObject(context.TODO(), pre_existing_file_name)
	assert.NoError(t, err)
}

func testListDir(t *testing.T, c *Fs) {
	des, err := c.List(context.TODO(), "")
	assert.NoError(t, err)

	t.Run("list contains pre existing files", func(t *testing.T) {
		assert.Contains(t, dirEntriesBases(des), pre_existing_file_name)
	})

	t.Run("subdir contents", func(t *testing.T) {
		des, err := c.List(context.TODO(), pre_existing_dir)
		assert.NoError(t, err)
		assert.Contains(t, dirEntriesBases(des), file_in_pre_existing_dir)
	})

	// TODO: listing contents of dir that does not exist

}

func testMkDir(t *testing.T, c *Fs) {
	dirName := "mkDirTest_" + RandomString(10)
	err := c.Mkdir(context.TODO(), dirName)
	assert.NoError(t, err)

	t.Run("dir shows up in listDir", func(t *testing.T) {
		des, err := c.List(context.TODO(), "")
		assert.NoError(t, err)
		assert.Contains(t, dirEntriesBases(des), dirName)
	})

	t.Run("nested dir where parent does not exist", func(t *testing.T) {
		parent := "mkDirTest_" + RandomString(10)
		child := "mkDirTest_" + RandomString(10)
		path.Join([]string{parent, child}...)
		fullPath := path.Join([]string{parent, child}...)
		err := c.Mkdir(context.TODO(), fullPath)
		assert.NoError(t, err)
		rootDes, rootListErr := c.List(context.TODO(), "")
		assert.NoError(t, rootListErr)
		assert.Contains(t, dirEntriesBases(rootDes), parent, "presence of parent directory in root")

		parentDes, parentListErr := c.List(context.TODO(), parent)
		assert.NoError(t, parentListErr)
		assert.Contains(t, dirEntriesBases(parentDes), child, "presence of child directory in parent")
	})

	t.Run("subdir where parent exists", func(t *testing.T) {
		subdirName := "mkDirTest_" + RandomString(10)
		fullPath := path.Join([]string{dirName, subdirName}...)
		err := c.Mkdir(context.TODO(), fullPath)
		assert.NoError(t, err)

		des, err := c.List(context.TODO(), dirName)
		assert.NoError(t, err)
		assert.Contains(t, dirEntriesBases(des), subdirName, "presence of subDir in dirName")
	})

	// t.Run("creating a directory inside existing subdir", func(t *testing.T) {
	// 	dirName := "mkDirTest_" + randomString(10)
	// 	path := filepath.Join(pre_existing_dir, dirName)
	// 	err := c.Mkdir(context.TODO(), path)
	// 	assert.NoError(t, err)

	// 	des, err := c.List(context.TODO(), pre_existing_dir)
	// 	assert.NoError(t, err)
	// 	assert.Contains(t, dirEntriesBases(des), dirName)
	// })

	t.Run("no error when directory already exists", func(t *testing.T) {
		err := c.Mkdir(context.TODO(), pre_existing_dir)
		assert.NoError(t, err)
	})

	// TODO: what happens if parent path does not exist
}

func testRmDir(t *testing.T, c *Fs) {
	dirToBeRemoved := "rmdirTest_" + RandomString(10)
	err := c.Mkdir(context.TODO(), dirToBeRemoved)
	assert.NoError(t, err)
	err = c.Rmdir(context.Background(), dirToBeRemoved)
	assert.NoError(t, err)
	des, err := c.List(context.TODO(), "")
	assert.NoError(t, err)
	assert.NotContains(t, dirEntriesBases(des), dirToBeRemoved)

	t.Run("remove subdir", func(t *testing.T) {
		parentDir := pre_existing_dir
		tempDirName := "rmdirTest_" + RandomString(10)
		dirToBeRemoved := path.Join(parentDir, tempDirName)
		err := c.Mkdir(context.TODO(), dirToBeRemoved)
		assert.NoError(t, err)
		err = c.Rmdir(context.Background(), dirToBeRemoved)
		assert.NoError(t, err)
	})

	// TODO: assert the exact error returned when rmdir fails
	t.Run("rmdir must fail if directory has contents", func(t *testing.T) {
		tempDir := "rmdirTest_" + RandomString(10)
		err := c.Mkdir(context.TODO(), tempDir)
		assert.NoError(t, err)
		fileName := RandomString(10) + ".txt"
		filePath := path.Join(tempDir, fileName)
		in, src := RandomPuttableObject(c, filePath)
		_, err = c.Put(context.TODO(), in, src, nil)
		assert.NoError(t, err)
		err = c.Rmdir(context.TODO(), filePath)
		assert.Error(t, err)
	})

}

func testRemove(t *testing.T, c *Fs) {
	fileName := "testRemove_" + RandomString(10) + ".txt"
	in, src := RandomPuttableObject(c, fileName)
	obj, err := c.Put(context.TODO(), in, src, nil)
	assert.NoError(t, err)
	err = obj.Remove(context.TODO())
	assert.NoError(t, err)
	des, err := c.List(context.TODO(), "")
	assert.NoError(t, err)
	assert.NotContains(t, dirEntriesBases(des), fileName)

	t.Run("works on files inside subdirectory", func(t *testing.T) {
		fileName := "testRemove_" + RandomString(10) + ".txt"
		filePath := path.Join(pre_existing_dir, fileName)
		in, src := RandomPuttableObject(c, filePath)
		obj, err := c.Put(context.TODO(), in, src, nil)
		assert.NoError(t, err)
		err = obj.Remove(context.TODO())
		assert.NoError(t, err)
		des, err := c.List(context.TODO(), pre_existing_dir)
		assert.NoError(t, err)
		assert.NotContains(t, dirEntriesBases(des), fileName)
	})

	t.Run("fails when file does not exist", func(t *testing.T) {
		fileName := "testRemove_" + RandomString(10) + ".txt"
		obj := &Object{common{
			f:      c,
			remote: fileName,
		}}
		err := obj.Remove(context.TODO())
		assert.Error(t, err)
	})

	// TODO: what happens if object is directory

}

func testOpen(t *testing.T, c *Fs) {
	obj, err := c.NewObject(context.TODO(), pre_existing_file_name)
	assert.NoError(t, err)
	r, err := obj.Open(context.TODO(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	bs, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, pre_existing_file_contents, string(bs))

}

func testUpdate(t *testing.T, c *Fs) {
	// Setup: creating file that will be updated
	fileName := "testUpdate_" + RandomString(10) + ".txt"
	r, src := RandomPuttableObject(c, fileName)
	obj, err := c.Put(context.TODO(), r, src, nil)
	assert.NoError(t, err, "was there an error while putting file to create initial test file")

	sleepSeconds := 3
	t.Log("sleeping for 3 seconds to create time difference between the creating of file and updation of file")
	for i := 0; i < sleepSeconds; i++ {
		t.Logf("about to sleep for %dth out of %d seconds", i+1, sleepSeconds)
		time.Sleep(time.Second)
	}

	// Setup: creating ojbects that will result in update
	updateContent, _ := RandomPuttableObject(c, fileName)
	updatedBytes, _ := io.ReadAll(updateContent)
	err = obj.Update(context.TODO(), bytes.NewReader(updatedBytes), src, nil)
	assert.NoError(t, err, "was there an error while updating file")

	t.Run("content md5 modtime size", func(t *testing.T) {
		o, err := c.NewObject(context.TODO(), fileName)
		assert.NoError(t, err, "creating object for update file to fetch hash")
		obj := o.(*Object)

		t.Run("content", func(t *testing.T) {
			actualContentReader, err := obj.Open(context.TODO(), nil)
			assert.NoError(t, err, "was there an error while opening updated file")
			actualBytes, err := io.ReadAll(actualContentReader)
			assert.NoError(t, err, "was there an error while reading contents of opened file")
			assert.Equal(t, actualBytes, updatedBytes, "comparing bytes")
		})

		t.Run("md5", func(t *testing.T) {
			expectedMd5 := md5.Sum(updatedBytes)

			resp, err := obj.fileClient().GetProperties(context.TODO(), nil)
			assert.NoError(t, err, "getting properties")

			assert.Equal(t, expectedMd5[:], resp.ContentMD5)
		})

		t.Run("modtime", func(t *testing.T) {
			// TODO: check whether modTime is updated by update
			assert.Equal(t, 1, 2)
		})

		t.Run("size", func(t *testing.T) {
			assert.EqualValues(t, obj.Size(), len(updatedBytes))
		})
	})
}

func dirEntriesBases(des fs.DirEntries) []string {
	bases := []string{}
	for _, de := range des {
		bases = append(bases, filepath.Base(de.Remote()))
	}
	return bases
}

func testWalkAll(t *testing.T, c *Fs) {
	// objs, dirs, err := walk.GetAll(context.TODO(), c, "", true, -1)
	// assert.NoError(t, err)
	// assert.Len(t, objs, 0)
	// assert.Len(t, dirs, 1)
	fn := func(path string, entries fs.DirEntries, err error) error {
		names := []string{}
		for _, en := range entries {
			names = append(names, en.String())
		}
		t.Logf("walk fn args path=%s entries=%s err=%s", path, strings.Join(names, ", "), err)
		return err
	}
	walk.Walk(context.TODO(), c, "", true, -1, fn)

}
