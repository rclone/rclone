package azurefiles

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/stretchr/testify/assert"
)

func TestIntegration(t *testing.T) {
	// t.Skip("skipping because uploading files and setting time beind tested")
	fstests.Run(t, &fstests.Opt{
		RemoteName: "TestAzureFiles:",
		NilObject:  (*Object)(nil),
	})
}

var pre_existing_file_name = "preexistingfile.txt"
var pre_existing_dir = "pre_existing_dir"
var file_in_pre_existing_dir = "lorem.txt"

func testNewObjectErrorOnObjectNotExisting(t *testing.T, c *Client) {
	_, err := c.NewObject(context.TODO(), "somefilethatdoesnotexist.txt")
	assert.Error(t, err)
}

func testNewObjectNoErrorIfObjectExists(t *testing.T, c *Client) {
	_, err := c.NewObject(context.TODO(), pre_existing_file_name)
	assert.NoError(t, err)
}

func testSetModTimeNoError(t *testing.T, c *Client) {
	obj, err := c.NewObject(context.TODO(), pre_existing_file_name)
	assert.NoError(t, err)
	randomTime := time.Date(1990+rand.Intn(20), time.December, rand.Intn(31), 0, 0, 0, 0, time.UTC)
	setModTimeErr := obj.SetModTime(context.TODO(), randomTime)
	assert.NoError(t, setModTimeErr)
}

func testSetModTimeStepWise(t *testing.T, c *Client) {
	fc := c.RootDirClient.NewFileClient(pre_existing_file_name)
	metaData := make(map[string]*string)
	someString := "1_" + randomString(10)
	metaData["a"] = &someString
	metaDataOptions := file.SetMetadataOptions{
		Metadata: metaData,
	}
	setMetadataResp, err := fc.SetMetadata(context.TODO(), &metaDataOptions)
	assert.NoError(t, err)
	t.Logf("%v", setMetadataResp)
}

// TODO: test put object in an inner directory
func testPutObject(t *testing.T, c *Client) {
	var fileSize int64 = 20
	fileName := randomString(10) + ".txt"
	fileContent := randomString(int(fileSize))
	r := bytes.NewReader([]byte(fileContent))
	metaData := make(map[string]*string)
	modTime := time.Now().Truncate(time.Second)
	nowStr := fmt.Sprintf("%d", modTime.Unix())
	metaData[modTimeKey] = &nowStr
	obj, err := c.Put(context.TODO(), r, &Object{
		remote:        fileName,
		metaData:      metaData,
		contentLength: &fileSize,
	})
	assert.NoError(t, err)
	assert.Equal(t, obj.ModTime(context.TODO()), modTime)
}

func dirEntriesBases(des fs.DirEntries) []string {
	bases := []string{}
	for _, de := range des {
		bases = append(bases, filepath.Base(de.Remote()))
	}
	return bases
}

func testListDir(t *testing.T, c *Client) {
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

func testMkDir(t *testing.T, c *Client) {
	dirName := "mkDirTest_" + randomString(10)
	err := c.Mkdir(context.TODO(), dirName)
	assert.NoError(t, err)

	t.Run("dir shows up in listDir", func(t *testing.T) {
		des, err := c.List(context.TODO(), "")
		assert.NoError(t, err)
		assert.Contains(t, dirEntriesBases(des), dirName)
	})

	t.Run("creating a directory inside existing subdir", func(t *testing.T) {
		dirName := "mkDirTest_" + randomString(10)
		path := filepath.Join(pre_existing_dir, dirName)
		err := c.Mkdir(context.TODO(), path)
		assert.NoError(t, err)

		des, err := c.List(context.TODO(), pre_existing_dir)
		assert.NoError(t, err)
		assert.Contains(t, dirEntriesBases(des), dirName)
	})

	t.Run("no error when directory already exists", func(t *testing.T) {
		err := c.Mkdir(context.TODO(), pre_existing_dir)
		assert.NoError(t, err)
	})

	// TODO: what happens if parent path does not exist
}

func testRmDir(t *testing.T, c *Client) {
	dirToBeRemoved := "rmdirTest_" + randomString(10)
	err := c.Mkdir(context.TODO(), dirToBeRemoved)
	assert.NoError(t, err)
	err = c.Rmdir(context.Background(), dirToBeRemoved)
	assert.NoError(t, err)
}

func TestAll(t *testing.T) {
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "TestAzureFiles:")
	if err != nil {
		t.Fatal("unable to create new FileSystem: %w", err)
	}

	if c, ok := f.(*Client); ok {
		wrapAndPassC := func(fc func(*testing.T, *Client)) func(*testing.T) {
			return func(t *testing.T) {
				fc(t, c)
			}
		}
		t.Run("NewObject Return error if object not found", wrapAndPassC(testNewObjectErrorOnObjectNotExisting))
		t.Run("NewObject does not return an error if file is found", wrapAndPassC(testNewObjectNoErrorIfObjectExists))
		t.Run("no error with set mod time", wrapAndPassC(testSetModTimeNoError))
		t.Run("set mod time step wise", wrapAndPassC(testSetModTimeStepWise))
		t.Run("put object without error", wrapAndPassC(testPutObject))
		t.Run("list dir", wrapAndPassC(testListDir))
		t.Run("mkDir", wrapAndPassC(testMkDir))

		t.Run("rmDir", wrapAndPassC(testRmDir))
	} else {
		t.Fatal("could not convert f to Client pointer")
	}

}

func TestPut(t *testing.T) {
	t.Skip("skipping because setModTime appears to be incorrect")
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "TestAzureFiles:")
	if err != nil {
		t.Fatal("unable to create new FileSystem: %w", err)
	}
	r := bytes.NewReader([]byte("This is some data from TestPut"))
	oi := Object{
		remote: randomString(10) + ".txt",
	}
	_, err = f.Put(context.Background(), r, &oi)
	if err != nil {
		t.Fatalf("error in putting %s:  %v", oi.remote, err)
	}

	des, err := f.List(context.Background(), "")
	if err != nil {
		t.Fatalf("error in listing contents:  %v", err)
	}
	for _, de := range des {
		if strings.HasSuffix(de.Remote(), oi.remote) {
			return
		}
	}
	t.Fatalf("could not create file %s", oi.remote)

}

func TestSetModTime(t *testing.T) {
	t.Skip("skipping beacuse the problem does not appear to not be only in setModTime ")
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "TestAzureFiles:")
	assert.NoError(t, err, "unable to create new FileSystem")

	des, err := f.List(context.Background(), "")
	assert.NoError(t, err, "unable to list")

	names := []string{}
	for _, de := range des {
		names = append(names, de.Remote())
	}
	sample_xml_filename := "sample_response_azure_files_list.xml"
	assert.Contains(t, names, sample_xml_filename)

	obj, err := f.NewObject(context.Background(), sample_xml_filename)
	assert.NoError(t, err, "creating new object from fs")

	t.Run("set mod time to some time in past", func(t *testing.T) {
		past := time.UnixMilli(919881000000) // UTC time: Wed Feb 24 1999 18:30:00
		err = obj.SetModTime(context.Background(), past)
		assert.NoError(t, err, "setting mod time")

		obj, err := f.NewObject(context.Background(), sample_xml_filename)
		assert.NoError(t, err, "getting object from fs")
		assert.Equal(t, obj.ModTime(context.Background()).Year(), past.Year())
		assert.Equal(t, obj.ModTime(context.Background()).Month(), past.Month())
		assert.Equal(t, obj.ModTime(context.Background()).Day(), past.Day())
	})

	t.Run("set mod time to some time to now", func(t *testing.T) {
		now := time.Now()
		err = obj.SetModTime(context.Background(), time.Now()) // UTC time: Wed Feb 24 1999 18:30:00
		assert.NoError(t, err, "setting mod time")

		obj, err := f.NewObject(context.Background(), sample_xml_filename)
		assert.NoError(t, err, "getting object from fs")
		assert.Equal(t, obj.ModTime(context.Background()).Year(), now.Year())
	})

}

func randomString(charCount int) string {
	bs := make([]byte, charCount)
	for i := 0; i < charCount; i++ {
		bs[i] = byte(97 + rand.Intn(26))
	}
	return string(bs)
}
