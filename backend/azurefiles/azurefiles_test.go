package azurefiles

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"strconv"
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
var pre_existing_file_contents = "This pre existing file has some content"

func TestNonCommonIntegration(t *testing.T) {
	// t.Skip("Skipping because we are working with integration tests from rclone")
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "TestAzureFiles:")
	if err != nil {
		t.Fatal("unable to create new FileSystem: %w", err)
	}

	if c, ok := f.(*Fs); ok {
		wrapAndPassC := func(fc func(*testing.T, *Fs)) func(*testing.T) {
			return func(t *testing.T) {
				fc(t, c)
			}
		}
		t.Run("NewObject Return error if object not found", wrapAndPassC(testNewObjectErrorOnObjectNotExisting))
		t.Run("NewObject does not return an error if file is found", wrapAndPassC(testNewObjectNoErrorIfObjectExists))
		t.Run("setModTime", wrapAndPassC(testSetModTime))
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

func TestJoinPaths(t *testing.T) {
	segments := []string{"parent", "child"}
	assert.Equal(t, "parent/child", joinPaths(segments...))

	segments = []string{"", "folder"}
	assert.Equal(t, "folder", joinPaths(segments...))

}

func randomString(charCount int) string {
	bs := make([]byte, charCount)
	for i := 0; i < charCount; i++ {
		bs[i] = byte(97 + rand.Intn(26))
	}
	return string(bs)
}

func randomTime() time.Time {
	return time.Unix(int64(rand.Int31()), 0)
}

func randomPuttableObject(remote string) (io.Reader, *Object) {
	var fileSize int64 = int64(10 + rand.Intn(50))
	fileContent := randomString(int(fileSize))
	r := bytes.NewReader([]byte(fileContent))
	metaData := make(map[string]*string)
	modTime := randomTime().Truncate(time.Second)
	nowStr := fmt.Sprintf("%d", modTime.Unix())
	metaData[modTimeKey] = &nowStr
	return r, &Object{common{
		remote:     remote,
		metaData:   metaData,
		properties: properties{contentLength: &fileSize},
	}}
}

// TODO: test put object in an inner directory
func testPutObject(t *testing.T, f *Fs) {

	// TODO: correct hash is set

	name := randomString(10) + ".txt"
	in, src := randomPuttableObject(name)
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
	fcSetting := c.RootDirClient.NewFileClient(pre_existing_file_name)
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

	fcGetting := c.RootDirClient.NewFileClient(pre_existing_file_name)
	getResp, getErr := fcGetting.GetProperties(context.TODO(), nil)
	assert.NoError(t, getErr)
	actualPtr, getValueErr := getMetaDataValue(getResp.Metadata, "a")
	assert.NoError(t, getValueErr)
	assert.Equal(t, someString, *actualPtr)

}

func testSetModTime(t *testing.T, f *Fs) {
	obj, err := f.NewObject(context.TODO(), pre_existing_file_name)
	assert.NoError(t, err)
	timeBeingSet := randomTime()
	setModTimeErr := obj.SetModTime(context.TODO(), timeBeingSet)
	assert.NoError(t, setModTimeErr)

	fc := f.RootDirClient.NewFileClient(pre_existing_file_name)
	res, getPropErr := fc.GetProperties(context.TODO(), nil)
	assert.NoError(t, getPropErr)
	gotTimeStr, getValueErr := getMetaDataValue(res.Metadata, modTimeKey)
	assert.NoError(t, getValueErr)
	gotTimeInt, err := strconv.ParseInt(*gotTimeStr, 10, 64)
	assert.NoError(t, err)
	gotTime := time.Unix(gotTimeInt, 0)

	assert.Equal(t, timeBeingSet, gotTime)
}

func testEncoding(t *testing.T, f *Fs) {

}
