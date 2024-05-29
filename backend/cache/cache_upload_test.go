//go:build !plan9 && !js && !race

package cache_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/cache"
	_ "github.com/rclone/rclone/backend/drive"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/require"
)

func TestInternalUploadTempDirCreated(t *testing.T) {
	id := fmt.Sprintf("tiutdc%v", time.Now().Unix())
	runInstance.newCacheFs(t, remoteName, id, false, true,
		map[string]string{"tmp_upload_path": path.Join(runInstance.tmpUploadDir, id)})

	_, err := os.Stat(path.Join(runInstance.tmpUploadDir, id))
	require.NoError(t, err)
}

func testInternalUploadQueueOneFile(t *testing.T, id string, rootFs fs.Fs, boltDb *cache.Persistent) {
	// create some rand test data
	testSize := int64(524288000)
	testReader := runInstance.randomReader(t, testSize)
	bu := runInstance.listenForBackgroundUpload(t, rootFs, "one")
	runInstance.writeRemoteReader(t, rootFs, "one", testReader)
	// validate that it exists in temp fs
	ti, err := os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "one")))
	require.NoError(t, err)

	if runInstance.rootIsCrypt {
		require.Equal(t, int64(524416032), ti.Size())
	} else {
		require.Equal(t, testSize, ti.Size())
	}
	de1, err := runInstance.list(t, rootFs, "")
	require.NoError(t, err)
	require.Len(t, de1, 1)

	runInstance.completeBackgroundUpload(t, "one", bu)
	// check if it was removed from temp fs
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "one")))
	require.True(t, os.IsNotExist(err))

	// check if it can be read
	data2, err := runInstance.readDataFromRemote(t, rootFs, "one", 0, int64(1024), false)
	require.NoError(t, err)
	require.Len(t, data2, 1024)
}

func TestInternalUploadQueueOneFileNoRest(t *testing.T) {
	id := fmt.Sprintf("tiuqofnr%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true,
		map[string]string{"tmp_upload_path": path.Join(runInstance.tmpUploadDir, id), "tmp_wait_time": "0s"})

	testInternalUploadQueueOneFile(t, id, rootFs, boltDb)
}

func TestInternalUploadQueueOneFileWithRest(t *testing.T) {
	id := fmt.Sprintf("tiuqofwr%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true,
		map[string]string{"tmp_upload_path": path.Join(runInstance.tmpUploadDir, id), "tmp_wait_time": "1m"})

	testInternalUploadQueueOneFile(t, id, rootFs, boltDb)
}

func TestInternalUploadMoveExistingFile(t *testing.T) {
	id := fmt.Sprintf("tiumef%v", time.Now().Unix())
	rootFs, _ := runInstance.newCacheFs(t, remoteName, id, true, true,
		map[string]string{"tmp_upload_path": path.Join(runInstance.tmpUploadDir, id), "tmp_wait_time": "3s"})

	err := rootFs.Mkdir(context.Background(), "one")
	require.NoError(t, err)
	err = rootFs.Mkdir(context.Background(), "one/test")
	require.NoError(t, err)
	err = rootFs.Mkdir(context.Background(), "second")
	require.NoError(t, err)

	// create some rand test data
	testSize := int64(10485760)
	testReader := runInstance.randomReader(t, testSize)
	runInstance.writeObjectReader(t, rootFs, "one/test/data.bin", testReader)
	runInstance.completeAllBackgroundUploads(t, rootFs, "one/test/data.bin")

	de1, err := runInstance.list(t, rootFs, "one/test")
	require.NoError(t, err)
	require.Len(t, de1, 1)

	time.Sleep(time.Second * 5)
	//_ = os.Remove(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "one/test")))
	//require.NoError(t, err)

	err = runInstance.dirMove(t, rootFs, "one/test", "second/test")
	require.NoError(t, err)

	// check if it can be read
	de1, err = runInstance.list(t, rootFs, "second/test")
	require.NoError(t, err)
	require.Len(t, de1, 1)
}

func TestInternalUploadTempPathCleaned(t *testing.T) {
	id := fmt.Sprintf("tiutpc%v", time.Now().Unix())
	rootFs, _ := runInstance.newCacheFs(t, remoteName, id, true, true,
		map[string]string{"cache-tmp-upload-path": path.Join(runInstance.tmpUploadDir, id), "cache-tmp-wait-time": "5s"})

	err := rootFs.Mkdir(context.Background(), "one")
	require.NoError(t, err)
	err = rootFs.Mkdir(context.Background(), "one/test")
	require.NoError(t, err)
	err = rootFs.Mkdir(context.Background(), "second")
	require.NoError(t, err)

	// create some rand test data
	testSize := int64(1048576)
	testReader := runInstance.randomReader(t, testSize)
	testReader2 := runInstance.randomReader(t, testSize)
	runInstance.writeObjectReader(t, rootFs, "one/test/data.bin", testReader)
	runInstance.writeObjectReader(t, rootFs, "second/data.bin", testReader2)

	runInstance.completeAllBackgroundUploads(t, rootFs, "one/test/data.bin")
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "one/test")))
	require.True(t, os.IsNotExist(err))
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "one")))
	require.True(t, os.IsNotExist(err))
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "second")))
	require.False(t, os.IsNotExist(err))

	runInstance.completeAllBackgroundUploads(t, rootFs, "second/data.bin")
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "second/data.bin")))
	require.True(t, os.IsNotExist(err))

	de1, err := runInstance.list(t, rootFs, "one/test")
	require.NoError(t, err)
	require.Len(t, de1, 1)

	// check if it can be read
	de1, err = runInstance.list(t, rootFs, "second")
	require.NoError(t, err)
	require.Len(t, de1, 1)
}

func TestInternalUploadQueueMoreFiles(t *testing.T) {
	id := fmt.Sprintf("tiuqmf%v", time.Now().Unix())
	rootFs, _ := runInstance.newCacheFs(t, remoteName, id, true, true,
		map[string]string{"tmp_upload_path": path.Join(runInstance.tmpUploadDir, id), "tmp_wait_time": "1s"})

	err := rootFs.Mkdir(context.Background(), "test")
	require.NoError(t, err)
	minSize := 5242880
	maxSize := 10485760
	totalFiles := 10
	randInstance := rand.New(rand.NewSource(time.Now().Unix()))

	lastFile := ""
	for i := 0; i < totalFiles; i++ {
		size := int64(randInstance.Intn(maxSize-minSize) + minSize)
		testReader := runInstance.randomReader(t, size)
		remote := "test/" + strconv.Itoa(i) + ".bin"
		runInstance.writeRemoteReader(t, rootFs, remote, testReader)

		// validate that it exists in temp fs
		ti, err := os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, remote)))
		require.NoError(t, err)
		require.Equal(t, size, runInstance.cleanSize(t, ti.Size()))

		if runInstance.wrappedIsExternal && i < totalFiles-1 {
			time.Sleep(time.Second * 3)
		}
		lastFile = remote
	}

	// check if cache lists all files, likely temp upload didn't finish yet
	de1, err := runInstance.list(t, rootFs, "test")
	require.NoError(t, err)
	require.Len(t, de1, totalFiles)

	// wait for background uploader to do its thing
	runInstance.completeAllBackgroundUploads(t, rootFs, lastFile)

	// retry until we have no more temp files and fail if they don't go down to 0
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test")))
	require.True(t, os.IsNotExist(err))

	// check if cache lists all files
	de1, err = runInstance.list(t, rootFs, "test")
	require.NoError(t, err)
	require.Len(t, de1, totalFiles)
}

func TestInternalUploadTempFileOperations(t *testing.T) {
	id := "tiutfo"
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true,
		map[string]string{"tmp_upload_path": path.Join(runInstance.tmpUploadDir, id), "tmp_wait_time": "1h"})

	boltDb.PurgeTempUploads()

	// create some rand test data
	runInstance.mkdir(t, rootFs, "test")
	runInstance.writeRemoteString(t, rootFs, "test/one", "one content")

	// check if it can be read
	data1, err := runInstance.readDataFromRemote(t, rootFs, "test/one", 0, int64(len([]byte("one content"))), false)
	require.NoError(t, err)
	require.Equal(t, []byte("one content"), data1)
	// validate that it exists in temp fs
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
	require.NoError(t, err)

	// test DirMove - allowed
	err = runInstance.dirMove(t, rootFs, "test", "second")
	if err != errNotSupported {
		require.NoError(t, err)
		_, err = rootFs.NewObject(context.Background(), "test/one")
		require.Error(t, err)
		_, err = rootFs.NewObject(context.Background(), "second/one")
		require.NoError(t, err)
		// validate that it exists in temp fs
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
		require.Error(t, err)
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "second/one")))
		require.NoError(t, err)
		_, err = boltDb.SearchPendingUpload(runInstance.encryptRemoteIfNeeded(t, path.Join(id, "test/one")))
		require.Error(t, err)
		var started bool
		started, err = boltDb.SearchPendingUpload(runInstance.encryptRemoteIfNeeded(t, path.Join(id, "second/one")))
		require.NoError(t, err)
		require.False(t, started)
		runInstance.mkdir(t, rootFs, "test")
		runInstance.writeRemoteString(t, rootFs, "test/one", "one content")
	}

	// test Rmdir - allowed
	err = runInstance.rm(t, rootFs, "test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "directory not empty")
	_, err = rootFs.NewObject(context.Background(), "test/one")
	require.NoError(t, err)
	// validate that it exists in temp fs
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
	require.NoError(t, err)
	started, err := boltDb.SearchPendingUpload(runInstance.encryptRemoteIfNeeded(t, path.Join(id, "test/one")))
	require.False(t, started)
	require.NoError(t, err)

	// test Move/Rename -- allowed
	err = runInstance.move(t, rootFs, path.Join("test", "one"), path.Join("test", "second"))
	if err != errNotSupported {
		require.NoError(t, err)
		// try to read from it
		_, err = rootFs.NewObject(context.Background(), "test/one")
		require.Error(t, err)
		_, err = rootFs.NewObject(context.Background(), "test/second")
		require.NoError(t, err)
		data2, err := runInstance.readDataFromRemote(t, rootFs, "test/second", 0, int64(len([]byte("one content"))), false)
		require.NoError(t, err)
		require.Equal(t, []byte("one content"), data2)
		// validate that it exists in temp fs
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
		require.Error(t, err)
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/second")))
		require.NoError(t, err)
		runInstance.writeRemoteString(t, rootFs, "test/one", "one content")
	}

	// test Copy -- allowed
	err = runInstance.copy(t, rootFs, path.Join("test", "one"), path.Join("test", "third"))
	if err != errNotSupported {
		require.NoError(t, err)
		_, err = rootFs.NewObject(context.Background(), "test/one")
		require.NoError(t, err)
		_, err = rootFs.NewObject(context.Background(), "test/third")
		require.NoError(t, err)
		data2, err := runInstance.readDataFromRemote(t, rootFs, "test/third", 0, int64(len([]byte("one content"))), false)
		require.NoError(t, err)
		require.Equal(t, []byte("one content"), data2)
		// validate that it exists in temp fs
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
		require.NoError(t, err)
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/third")))
		require.NoError(t, err)
	}

	// test Remove -- allowed
	err = runInstance.rm(t, rootFs, "test/one")
	require.NoError(t, err)
	_, err = rootFs.NewObject(context.Background(), "test/one")
	require.Error(t, err)
	// validate that it doesn't exist in temp fs
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
	require.Error(t, err)
	runInstance.writeRemoteString(t, rootFs, "test/one", "one content")

	// test Update -- allowed
	firstModTime, err := runInstance.modTime(t, rootFs, "test/one")
	require.NoError(t, err)
	err = runInstance.updateData(t, rootFs, "test/one", "one content", " updated")
	require.NoError(t, err)
	obj2, err := rootFs.NewObject(context.Background(), "test/one")
	require.NoError(t, err)
	data2 := runInstance.readDataFromObj(t, obj2, 0, int64(len("one content updated")), false)
	require.Equal(t, "one content updated", string(data2))
	tmpInfo, err := os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
	require.NoError(t, err)
	if runInstance.rootIsCrypt {
		require.Equal(t, int64(67), tmpInfo.Size())
	} else {
		require.Equal(t, int64(len(data2)), tmpInfo.Size())
	}

	// test SetModTime -- allowed
	secondModTime, err := runInstance.modTime(t, rootFs, "test/one")
	require.NoError(t, err)
	require.NotEqual(t, secondModTime, firstModTime)
	require.NotEqual(t, time.Time{}, firstModTime)
	require.NotEqual(t, time.Time{}, secondModTime)
}

func TestInternalUploadUploadingFileOperations(t *testing.T) {
	id := "tiuufo"
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true,
		map[string]string{"tmp_upload_path": path.Join(runInstance.tmpUploadDir, id), "tmp_wait_time": "1h"})

	boltDb.PurgeTempUploads()

	// create some rand test data
	runInstance.mkdir(t, rootFs, "test")
	runInstance.writeRemoteString(t, rootFs, "test/one", "one content")

	// check if it can be read
	data1, err := runInstance.readDataFromRemote(t, rootFs, "test/one", 0, int64(len([]byte("one content"))), false)
	require.NoError(t, err)
	require.Equal(t, []byte("one content"), data1)
	// validate that it exists in temp fs
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
	require.NoError(t, err)

	err = boltDb.SetPendingUploadToStarted(runInstance.encryptRemoteIfNeeded(t, path.Join(rootFs.Root(), "test/one")))
	require.NoError(t, err)

	// test DirMove
	err = runInstance.dirMove(t, rootFs, "test", "second")
	if err != errNotSupported {
		require.Error(t, err)
		_, err = rootFs.NewObject(context.Background(), "test/one")
		require.NoError(t, err)
		// validate that it exists in temp fs
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
		require.NoError(t, err)
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "second/one")))
		require.Error(t, err)
	}

	// test Rmdir
	err = runInstance.rm(t, rootFs, "test")
	require.Error(t, err)
	_, err = rootFs.NewObject(context.Background(), "test/one")
	require.NoError(t, err)
	// validate that it doesn't exist in temp fs
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
	require.NoError(t, err)

	// test Move/Rename
	err = runInstance.move(t, rootFs, path.Join("test", "one"), path.Join("test", "second"))
	if err != errNotSupported {
		require.Error(t, err)
		// try to read from it
		_, err = rootFs.NewObject(context.Background(), "test/one")
		require.NoError(t, err)
		_, err = rootFs.NewObject(context.Background(), "test/second")
		require.Error(t, err)
		// validate that it exists in temp fs
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
		require.NoError(t, err)
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/second")))
		require.Error(t, err)
	}

	// test Copy -- allowed
	err = runInstance.copy(t, rootFs, path.Join("test", "one"), path.Join("test", "third"))
	if err != errNotSupported {
		require.NoError(t, err)
		_, err = rootFs.NewObject(context.Background(), "test/one")
		require.NoError(t, err)
		_, err = rootFs.NewObject(context.Background(), "test/third")
		require.NoError(t, err)
		data2, err := runInstance.readDataFromRemote(t, rootFs, "test/third", 0, int64(len([]byte("one content"))), false)
		require.NoError(t, err)
		require.Equal(t, []byte("one content"), data2)
		// validate that it exists in temp fs
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
		require.NoError(t, err)
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/third")))
		require.NoError(t, err)
	}

	// test Remove
	err = runInstance.rm(t, rootFs, "test/one")
	require.Error(t, err)
	_, err = rootFs.NewObject(context.Background(), "test/one")
	require.NoError(t, err)
	// validate that it doesn't exist in temp fs
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
	require.NoError(t, err)
	runInstance.writeRemoteString(t, rootFs, "test/one", "one content")

	// test Update - this seems to work. Why? FIXME
	//firstModTime, err := runInstance.modTime(t, rootFs, "test/one")
	//require.NoError(t, err)
	//err = runInstance.updateData(t, rootFs, "test/one", "one content", " updated", func() {
	//	data2 := runInstance.readDataFromRemote(t, rootFs, "test/one", 0, int64(len("one content updated")), true)
	//	require.Equal(t, "one content", string(data2))
	//
	//	tmpInfo, err := os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
	//	require.NoError(t, err)
	//	if runInstance.rootIsCrypt {
	//		require.Equal(t, int64(67), tmpInfo.Size())
	//	} else {
	//		require.Equal(t, int64(len(data2)), tmpInfo.Size())
	//	}
	//})
	//require.Error(t, err)

	// test SetModTime -- seems to work cause of previous
	//secondModTime, err := runInstance.modTime(t, rootFs, "test/one")
	//require.NoError(t, err)
	//require.Equal(t, secondModTime, firstModTime)
	//require.NotEqual(t, time.Time{}, firstModTime)
	//require.NotEqual(t, time.Time{}, secondModTime)
}
