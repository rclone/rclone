// +build !plan9,go1.7

package cache_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ncw/rclone/cache"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest"
	"github.com/ncw/rclone/local"
	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

var (
	WrapRemote = flag.String("wrap-remote", "", "Remote to wrap")
	RemoteName = flag.String("remote-name", "TestCacheInternal", "Root remote")
	rootFs     fs.Fs
	boltDb     *cache.Persistent
	infoAge    = time.Second * 10
	chunkClean = time.Second
	okDiff     = time.Second * 9 // really big diff here but the build machines seem to be slow. need a different way for this
	workers    = 2
)

// prepare the test server and return a function to tidy it up afterwards
func TestInternalInit(t *testing.T) {
	var err error

	// delete the default path
	dbPath := filepath.Join(fs.CacheDir, "cache-backend", *RemoteName+".db")
	boltDb, err = cache.GetPersistent(dbPath, &cache.Features{PurgeDb: true})
	require.NoError(t, err)
	fstest.Initialise()

	if len(*WrapRemote) == 0 {
		*WrapRemote = "localInternal:/var/tmp/rclone-cache"
		fs.ConfigFileSet("localInternal", "type", "local")
		fs.ConfigFileSet("localInternal", "nounc", "true")
	}

	remoteExists := false
	for _, s := range fs.ConfigFileSections() {
		if s == *RemoteName {
			remoteExists = true
		}
	}

	if !remoteExists {
		fs.ConfigFileSet(*RemoteName, "type", "cache")
		fs.ConfigFileSet(*RemoteName, "remote", *WrapRemote)
		fs.ConfigFileSet(*RemoteName, "chunk_size", "1024")
		fs.ConfigFileSet(*RemoteName, "chunk_total_size", "2048")
		fs.ConfigFileSet(*RemoteName, "info_age", infoAge.String())
	}

	_ = flag.Set("cache-chunk-no-memory", "true")
	_ = flag.Set("cache-workers", strconv.Itoa(workers))
	_ = flag.Set("cache-chunk-clean-interval", chunkClean.String())

	// Instantiate root
	rootFs, err = fs.NewFs(*RemoteName + ":")
	require.NoError(t, err)
	_ = rootFs.Features().Purge()
	require.NoError(t, err)
	err = rootFs.Mkdir("")
	require.NoError(t, err)

	// flush cache
	_, err = getCacheFs(rootFs)
	require.NoError(t, err)
}

func TestInternalListRootAndInnerRemotes(t *testing.T) {
	// Instantiate inner fs
	innerFolder := "inner"
	err := rootFs.Mkdir(innerFolder)
	require.NoError(t, err)
	innerFs, err := fs.NewFs(*RemoteName + ":" + innerFolder)
	require.NoError(t, err)

	obj := writeObjectString(t, innerFs, "one", "content")

	listRoot, err := rootFs.List("")
	require.NoError(t, err)
	listRootInner, err := rootFs.List(innerFolder)
	require.NoError(t, err)
	listInner, err := innerFs.List("")
	require.NoError(t, err)

	require.Lenf(t, listRoot, 1, "remote %v should have 1 entry", rootFs.Root())
	require.Lenf(t, listRootInner, 1, "remote %v should have 1 entry in %v", rootFs.Root(), innerFolder)
	require.Lenf(t, listInner, 1, "remote %v should have 1 entry", innerFs.Root())

	err = obj.Remove()
	require.NoError(t, err)

	err = innerFs.Features().Purge()
	require.NoError(t, err)
	innerFs = nil
}

func TestInternalObjWrapFsFound(t *testing.T) {
	reset(t)
	cfs, err := getCacheFs(rootFs)
	require.NoError(t, err)
	wrappedFs := cfs.UnWrap()
	data := "content"
	writeObjectString(t, wrappedFs, "second", data)

	listRoot, err := rootFs.List("")
	require.NoError(t, err)
	require.Lenf(t, listRoot, 1, "remote %v should have 1 entry", rootFs.Root())

	co, err := rootFs.NewObject("second")
	require.NoError(t, err)
	r, err := co.Open()
	require.NoError(t, err)
	cachedData, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	err = r.Close()
	require.NoError(t, err)

	strCached := string(cachedData)
	require.Equal(t, data, strCached)

	err = co.Remove()
	require.NoError(t, err)

	listRoot, err = wrappedFs.List("")
	require.NoError(t, err)
	require.Lenf(t, listRoot, 0, "remote %v should have 0 entries: %v", wrappedFs.Root(), listRoot)
}

func TestInternalObjNotFound(t *testing.T) {
	reset(t)
	obj, err := rootFs.NewObject("404")
	require.Error(t, err)
	require.Nil(t, obj)
}

func TestInternalCachedWrittenContentMatches(t *testing.T) {
	reset(t)
	cfs, err := getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	// create some rand test data
	testData := make([]byte, (chunkSize*4 + chunkSize/2))
	testSize, err := rand.Read(testData)
	require.Equal(t, len(testData), testSize, "data size doesn't match")
	require.NoError(t, err)

	// write the object
	o := writeObjectBytes(t, rootFs, "data.bin", testData)
	require.Equal(t, o.Size(), int64(testSize))

	// check sample of data from in-file
	sampleStart := chunkSize / 2
	sampleEnd := chunkSize
	testSample := testData[sampleStart:sampleEnd]
	checkSample := readDataFromObj(t, o, sampleStart, sampleEnd, false)
	require.Equal(t, int64(len(checkSample)), sampleEnd-sampleStart)
	require.Equal(t, checkSample, testSample)
}

func TestInternalCachedUpdatedContentMatches(t *testing.T) {
	reset(t)

	// create some rand test data
	testData1 := []byte(fstest.RandomString(100))
	testData2 := []byte(fstest.RandomString(200))

	// write the object
	o := updateObjectBytes(t, rootFs, "data.bin", testData1, testData2)
	require.Equal(t, o.Size(), int64(len(testData2)))

	// check data from in-file
	reader, err := o.Open()
	require.NoError(t, err)
	checkSample, err := ioutil.ReadAll(reader)
	_ = reader.Close()
	require.NoError(t, err)
	require.Equal(t, checkSample, testData2)
}

func TestInternalWrappedWrittenContentMatches(t *testing.T) {
	cfs, err := getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	reset(t)

	// create some rand test data
	testData := make([]byte, (chunkSize*4 + chunkSize/2))
	testSize, err := rand.Read(testData)
	require.Equal(t, len(testData), testSize)
	require.NoError(t, err)

	// write the object
	o := writeObjectBytes(t, cfs.UnWrap(), "data.bin", testData)
	require.Equal(t, o.Size(), int64(testSize))

	o2, err := rootFs.NewObject("data.bin")
	require.NoError(t, err)
	require.Equal(t, o2.Size(), o.Size())

	// check sample of data from in-file
	sampleStart := chunkSize / 2
	sampleEnd := chunkSize
	testSample := testData[sampleStart:sampleEnd]
	checkSample := readDataFromObj(t, o2, sampleStart, sampleEnd, false)
	require.Equal(t, len(checkSample), len(testSample))

	for i := 0; i < len(checkSample); i++ {
		require.Equal(t, testSample[i], checkSample[i])
	}
}

func TestInternalLargeWrittenContentMatches(t *testing.T) {
	t.Skip("FIXME disabled because it is unreliable")

	cfs, err := getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	reset(t)

	// create some rand test data
	testData := make([]byte, (chunkSize*10 + chunkSize/2))
	testSize, err := rand.Read(testData)
	require.Equal(t, len(testData), testSize)
	require.NoError(t, err)

	// write the object
	o := writeObjectBytes(t, cfs.UnWrap(), "data.bin", testData)
	require.Equal(t, o.Size(), int64(testSize))

	o2, err := rootFs.NewObject("data.bin")
	require.NoError(t, err)
	require.Equal(t, o2.Size(), o.Size())

	// check data from in-file
	checkSample := readDataFromObj(t, o2, int64(0), int64(testSize), false)
	require.Equal(t, len(checkSample), len(testData))

	for i := 0; i < len(checkSample); i++ {
		require.Equal(t, testData[i], checkSample[i], "byte: %d (%d), chunk: %d", int64(i)%chunkSize, i, int64(i)/chunkSize)
	}
}

func TestInternalWrappedFsChangeNotSeen(t *testing.T) {
	reset(t)
	cfs, err := getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	// create some rand test data
	co := writeObjectRandomBytes(t, rootFs, (chunkSize*4 + chunkSize/2))

	// update in the wrapped fs
	o, err := cfs.UnWrap().NewObject(co.Remote())
	require.NoError(t, err)
	err = o.SetModTime(co.ModTime().Truncate(time.Hour))
	require.NoError(t, err)

	// get a new instance from the cache
	co2, err := rootFs.NewObject(o.Remote())
	require.NoError(t, err)

	require.NotEqual(t, o.ModTime(), co.ModTime())
	require.NotEqual(t, o.ModTime(), co2.ModTime())
	require.Equal(t, co.ModTime(), co2.ModTime())
}

func TestInternalChangeSeenAfterDirCacheFlush(t *testing.T) {
	cfs, err := getCacheFs(rootFs)
	require.NoError(t, err)

	cfs.DirCacheFlush() // flush the cache

	l, err := cfs.UnWrap().List("")
	require.NoError(t, err)
	require.Len(t, l, 1)
	o := l[0]

	// get a new instance from the cache
	co, err := rootFs.NewObject(o.Remote())
	require.NoError(t, err)
	require.Equal(t, o.ModTime(), co.ModTime())
}

func TestInternalCacheWrites(t *testing.T) {
	reset(t)
	_ = flag.Set("cache-writes", "true")
	rootFs, err := fs.NewFs(*RemoteName + ":")
	require.NoError(t, err)
	cfs, err := getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	// create some rand test data
	co := writeObjectRandomBytes(t, rootFs, (chunkSize*4 + chunkSize/2))
	expectedTs := time.Now()
	ts, err := boltDb.GetChunkTs(path.Join(rootFs.Root(), co.Remote()), 0)
	require.NoError(t, err)
	require.WithinDuration(t, expectedTs, ts, okDiff)

	// reset fs
	_ = flag.Set("cache-writes", "false")
	rootFs, err = fs.NewFs(*RemoteName + ":")
	require.NoError(t, err)
}

func TestInternalMaxChunkSizeRespected(t *testing.T) {
	reset(t)
	_ = flag.Set("cache-workers", "1")
	rootFs, err := fs.NewFs(*RemoteName + ":")
	require.NoError(t, err)
	cfs, err := getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()
	totalChunks := 20

	// create some rand test data
	o := writeObjectRandomBytes(t, cfs, (int64(totalChunks-1)*chunkSize + chunkSize/2))
	co, ok := o.(*cache.Object)
	require.True(t, ok)

	for i := 0; i < 4; i++ { // read first 4
		_ = readDataFromObj(t, co, chunkSize*int64(i), chunkSize*int64(i+1), false)
	}
	cfs.CleanUpCache(true)
	// the last 2 **must** be in the cache
	require.True(t, boltDb.HasChunk(co, chunkSize*2))
	require.True(t, boltDb.HasChunk(co, chunkSize*3))

	for i := 4; i < 6; i++ { // read next 2
		_ = readDataFromObj(t, co, chunkSize*int64(i), chunkSize*int64(i+1), false)
	}
	cfs.CleanUpCache(true)
	// the last 2 **must** be in the cache
	require.True(t, boltDb.HasChunk(co, chunkSize*4))
	require.True(t, boltDb.HasChunk(co, chunkSize*5))

	// reset fs
	_ = flag.Set("cache-workers", strconv.Itoa(workers))
	rootFs, err = fs.NewFs(*RemoteName + ":")
	require.NoError(t, err)
}

func TestInternalExpiredEntriesRemoved(t *testing.T) {
	reset(t)
	cfs, err := getCacheFs(rootFs)
	require.NoError(t, err)

	// create some rand test data
	_ = writeObjectString(t, cfs, "one", "one content")
	err = cfs.Mkdir("test")
	require.NoError(t, err)
	_ = writeObjectString(t, cfs, "test/second", "second content")

	objOne, err := cfs.NewObject("one")
	require.NoError(t, err)
	require.Equal(t, int64(len([]byte("one content"))), objOne.Size())

	waitTime := infoAge + time.Second*2
	t.Logf("Waiting %v seconds for cache to expire\n", waitTime)
	time.Sleep(infoAge)

	_, err = cfs.List("test")
	require.NoError(t, err)
	time.Sleep(time.Second * 2)
	require.False(t, boltDb.HasEntry("one"))
}

func TestInternalFinalise(t *testing.T) {
	var err error

	err = rootFs.Features().Purge()
	require.NoError(t, err)
}

func writeObjectRandomBytes(t *testing.T, f fs.Fs, size int64) fs.Object {
	remote := strconv.Itoa(rand.Int()) + ".bin"
	// create some rand test data
	testData := make([]byte, size)
	testSize, err := rand.Read(testData)
	require.Equal(t, size, int64(len(testData)))
	require.Equal(t, size, int64(testSize))
	require.NoError(t, err)

	o := writeObjectBytes(t, f, remote, testData)
	require.Equal(t, size, o.Size())

	return o
}

func writeObjectString(t *testing.T, f fs.Fs, remote, content string) fs.Object {
	return writeObjectBytes(t, f, remote, []byte(content))
}

func writeObjectBytes(t *testing.T, f fs.Fs, remote string, data []byte) fs.Object {
	in := bytes.NewReader(data)
	modTime := time.Now()
	objInfo := fs.NewStaticObjectInfo(remote, modTime, int64(len(data)), true, nil, f)

	obj, err := f.Put(in, objInfo)
	require.NoError(t, err)

	return obj
}

func updateObjectBytes(t *testing.T, f fs.Fs, remote string, data1 []byte, data2 []byte) fs.Object {
	in1 := bytes.NewReader(data1)
	in2 := bytes.NewReader(data2)
	objInfo1 := fs.NewStaticObjectInfo(remote, time.Now(), int64(len(data1)), true, nil, f)
	objInfo2 := fs.NewStaticObjectInfo(remote, time.Now(), int64(len(data2)), true, nil, f)

	obj, err := f.Put(in1, objInfo1)
	require.NoError(t, err)
	obj, err = f.NewObject(remote)
	require.NoError(t, err)
	err = obj.Update(in2, objInfo2)

	return obj
}

func readDataFromObj(t *testing.T, co fs.Object, offset, end int64, useSeek bool) []byte {
	var reader io.ReadCloser
	var err error
	size := end - offset
	checkSample := make([]byte, size)

	reader, err = co.Open(&fs.SeekOption{Offset: offset})
	require.NoError(t, err)

	totalRead, err := io.ReadFull(reader, checkSample)
	require.NoError(t, err)
	_ = reader.Close()
	require.Equal(t, int64(totalRead), size, "wrong data read size from file")

	return checkSample
}

func doStuff(t *testing.T, times int, maxDuration time.Duration, stuff func()) {
	var wg sync.WaitGroup

	for i := 0; i < times; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(maxDuration / 2)
			stuff()
			time.Sleep(maxDuration / 2)
		}()
	}

	wg.Wait()
}

func reset(t *testing.T) {
	var err error

	err = rootFs.Features().Purge()
	require.NoError(t, err)

	// Instantiate root
	rootFs, err = fs.NewFs(*RemoteName + ":")
	require.NoError(t, err)
	err = rootFs.Mkdir("")
	require.NoError(t, err)
}

func getCacheFs(f fs.Fs) (*cache.Fs, error) {
	cfs, ok := f.(*cache.Fs)
	if ok {
		return cfs, nil
	} else {
		if f.Features().UnWrap != nil {
			cfs, ok := f.Features().UnWrap().(*cache.Fs)
			if ok {
				return cfs, nil
			}
		}
	}

	return nil, fmt.Errorf("didn't found a cache fs")
}

func getSourceFs(f fs.Fs) (fs.Fs, error) {
	if f.Features().UnWrap != nil {
		sfs := f.Features().UnWrap()
		_, ok := sfs.(*cache.Fs)
		if !ok {
			return sfs, nil
		}

		return getSourceFs(sfs)
	}

	return nil, fmt.Errorf("didn't found a source fs")
}

var (
	_ fs.Fs = (*cache.Fs)(nil)
	_ fs.Fs = (*local.Fs)(nil)
)
