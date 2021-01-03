package syncfiles

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/all" // import all backends
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fstest"
)

func TestMain(m *testing.M) {
	fstest.TestMain(m)
}

func TestSyncFile(t *testing.T) {
	ctx := context.Background()
	// Create a temp file as source for sync
	f, err := ioutil.TempFile("", "input")
	if err != nil {
		t.Error("Failed to create temp file:" + err.Error())
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString("Test data for sync:" + time.Now().String())
	if err != nil {
		t.Error("Failed to write to temp file " + f.Name() + ":" + err.Error())
	}
	f.Close()

	// Create temp dir as dest for sync
	dir, err := ioutil.TempDir("", "destDir")
	if err != nil {
		t.Error("Failed to create temp dir:" + err.Error())
	}
	defer os.RemoveAll(dir)
	fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst([]string{f.Name(), dir})
	err = SyncFile(ctx, fsrc, fdst, srcFileName)
	if err != nil {
		t.Error("Sync file test failed:" + err.Error())
	}
}
