package azurefiles

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

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
		t.Run("no error with set mod time", wrapAndPassC(testSetModTimeNoError))
		t.Run("set mod time step wise", wrapAndPassC(testSetModTimeStepWise))
		t.Run("put object without error", wrapAndPassC(testPutObject))
		t.Run("list dir", wrapAndPassC(testListDir))
		t.Run("mkDir", wrapAndPassC(testMkDir))
		t.Run("rmDir", wrapAndPassC(testRmDir))
		t.Run("remove", wrapAndPassC(testRemove))
		t.Run("open", wrapAndPassC(testOpen))
		t.Run("update", wrapAndPassC(testUpdate))
		t.Run("walkAll", wrapAndPassC(testWalkAll))
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

func randomPuttableObject(remote string) (io.Reader, fs.ObjectInfo) {
	var fileSize int64 = 20
	fileContent := randomString(int(fileSize))
	r := bytes.NewReader([]byte(fileContent))
	metaData := make(map[string]*string)
	modTime := time.Now().Truncate(time.Second)
	nowStr := fmt.Sprintf("%d", modTime.Unix())
	metaData[modTimeKey] = &nowStr
	return r, &Object{common{
		remote:     remote,
		metaData:   metaData,
		properties: properties{contentLength: &fileSize},
	}}
}
