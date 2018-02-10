// +build !plan9,go1.7

package cache_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"

	"encoding/base64"
	goflag "flag"
	"fmt"
	"runtime/debug"

	"github.com/ncw/rclone/backend/cache"
	"github.com/ncw/rclone/backend/crypt"
	_ "github.com/ncw/rclone/backend/drive"
	"github.com/ncw/rclone/backend/local"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
	"github.com/ncw/rclone/fs/object"
	"github.com/ncw/rclone/fstest"
	"github.com/ncw/rclone/vfs"
	"github.com/ncw/rclone/vfs/vfsflags"
	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

const (
	// these 2 passwords are test random
	cryptPassword1     = "3XcvMMdsV3d-HGAReTMdNH-5FcX5q32_lUeA"                                                     // oGJdUbQc7s8
	cryptPassword2     = "NlgTBEIe-qibA7v-FoMfuX6Cw8KlLai_aMvV"                                                     // mv4mZW572HM
	cryptedTextBase64  = "UkNMT05FAAC320i2xIee0BiNyknSPBn+Qcw3q9FhIFp3tvq6qlqvbsno3PnxmEFeJG3jDBnR/wku2gHWeQ=="     // one content
	cryptedText2Base64 = "UkNMT05FAAATcQkVsgjBh8KafCKcr0wdTa1fMmV0U8hsCLGFoqcvxKVmvv7wx3Hf5EXxFcki2FFV4sdpmSrb9Q==" // updated content
)

var (
	remoteName                  string
	mountDir                    string
	uploadDir                   string
	useMount                    bool
	runInstance                 *run
	errNotSupported             = errors.New("not supported")
	decryptedToEncryptedRemotes = map[string]string{
		"one":                "lm4u7jjt3c85bf56vjqgeenuno",
		"second":             "qvt1ochrkcfbptp5mu9ugb2l14",
		"test":               "jn4tegjtpqro30t3o11thb4b5s",
		"test2":              "qakvqnh8ttei89e0gc76crpql4",
		"data.bin":           "0q2847tfko6mhj3dag3r809qbc",
		"ticw/data.bin":      "5mv97b0ule6pht33srae5pice8/0q2847tfko6mhj3dag3r809qbc",
		"tiuufo/test/one":    "vi6u1olqhirqv14cd8qlej1mgo/jn4tegjtpqro30t3o11thb4b5s/lm4u7jjt3c85bf56vjqgeenuno",
		"tiuufo/test/second": "vi6u1olqhirqv14cd8qlej1mgo/jn4tegjtpqro30t3o11thb4b5s/qvt1ochrkcfbptp5mu9ugb2l14",
		"tiutfo/test/one":    "legd371aa8ol36tjfklt347qnc/jn4tegjtpqro30t3o11thb4b5s/lm4u7jjt3c85bf56vjqgeenuno",
		"tiutfo/second/one":  "legd371aa8ol36tjfklt347qnc/qvt1ochrkcfbptp5mu9ugb2l14/lm4u7jjt3c85bf56vjqgeenuno",
		"second/one":         "qvt1ochrkcfbptp5mu9ugb2l14/lm4u7jjt3c85bf56vjqgeenuno",
		"test/one":           "jn4tegjtpqro30t3o11thb4b5s/lm4u7jjt3c85bf56vjqgeenuno",
		"test/second":        "jn4tegjtpqro30t3o11thb4b5s/qvt1ochrkcfbptp5mu9ugb2l14",
		"test/third":         "jn4tegjtpqro30t3o11thb4b5s/2nd7fjiop5h3ihfj1vl953aa5g",
		"test/0.bin":         "jn4tegjtpqro30t3o11thb4b5s/e6frddt058b6kvbpmlstlndmtk",
		"test/1.bin":         "jn4tegjtpqro30t3o11thb4b5s/kck472nt1k7qbmob0mt1p1crgc",
		"test/2.bin":         "jn4tegjtpqro30t3o11thb4b5s/744oe9ven2rmak4u27if51qk24",
		"test/3.bin":         "jn4tegjtpqro30t3o11thb4b5s/2bjd8kef0u5lmsu6qhqll34bcs",
		"test/4.bin":         "jn4tegjtpqro30t3o11thb4b5s/cvjs73iv0a82v0c7r67avllh7s",
		"test/5.bin":         "jn4tegjtpqro30t3o11thb4b5s/0plkdo790b6bnmt33qsdqmhv9c",
		"test/6.bin":         "jn4tegjtpqro30t3o11thb4b5s/s5r633srnjtbh83893jovjt5d0",
		"test/7.bin":         "jn4tegjtpqro30t3o11thb4b5s/6rq45tr9bjsammku622flmqsu4",
		"test/8.bin":         "jn4tegjtpqro30t3o11thb4b5s/37bc6tcl3e31qb8cadvjb749vk",
		"test/9.bin":         "jn4tegjtpqro30t3o11thb4b5s/t4pr35hnls32789o8fk0chk1ec",
	}
)

func init() {
	goflag.StringVar(&remoteName, "remote-internal", "TestInternalCache", "Remote to test with, defaults to local filesystem")
	goflag.StringVar(&mountDir, "mount-dir-internal", "", "")
	goflag.StringVar(&uploadDir, "upload-dir-internal", "", "")
	goflag.BoolVar(&useMount, "cache-use-mount", false, "Test only with mount")
}

// TestMain drives the tests
func TestMain(m *testing.M) {
	goflag.Parse()
	var rc int

	runInstance = newRun()
	rc = m.Run()
	os.Exit(rc)
}

func TestInternalListRootAndInnerRemotes(t *testing.T) {
	id := fmt.Sprintf("tilrair%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	// Instantiate inner fs
	innerFolder := "inner"
	runInstance.mkdir(t, rootFs, innerFolder)
	rootFs2, boltDb2 := runInstance.newCacheFs(t, remoteName, id+"/"+innerFolder, true, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs2, boltDb2)

	runInstance.writeObjectString(t, rootFs2, "one", "content")
	listRoot, err := runInstance.list(t, rootFs, "")
	require.NoError(t, err)
	listRootInner, err := runInstance.list(t, rootFs, innerFolder)
	require.NoError(t, err)
	listInner, err := rootFs2.List("")
	require.NoError(t, err)

	require.Len(t, listRoot, 1)
	require.Len(t, listRootInner, 1)
	require.Len(t, listInner, 1)
}

func TestInternalVfsCache(t *testing.T) {
	vfsflags.Opt.DirCacheTime = time.Second * 30
	testSize := int64(524288000)

	vfsflags.Opt.CacheMode = vfs.CacheModeWrites
	id := "tiuufo"
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true, nil, map[string]string{"cache-writes": "true", "cache-info-age": "1h"})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	err := rootFs.Mkdir("test")
	require.NoError(t, err)
	runInstance.writeObjectString(t, rootFs, "test/second", "content")
	_, err = rootFs.List("test")
	require.NoError(t, err)

	testReader := runInstance.randomReader(t, testSize)
	writeCh := make(chan interface{})
	//write2Ch := make(chan interface{})
	readCh := make(chan interface{})
	cacheCh := make(chan interface{})
	// write the main file
	go func() {
		defer func() {
			writeCh <- true
		}()

		log.Printf("========== started writing file 'test/one'")
		runInstance.writeRemoteReader(t, rootFs, "test/one", testReader)
		log.Printf("========== done writing file 'test/one'")
	}()
	// routine to check which cache has what, autostarts
	go func() {
		for {
			select {
			case <-cacheCh:
				log.Printf("========== finished checking caches")
				return
			default:
			}
			li2 := [2]string{path.Join("test", "one"), path.Join("test", "second")}
			for _, r := range li2 {
				var err error
				ci, err := ioutil.ReadDir(path.Join(runInstance.chunkPath, runInstance.encryptRemoteIfNeeded(t, path.Join(id, r))))
				if err != nil || len(ci) == 0 {
					log.Printf("========== '%v' not in cache", r)
				} else {
					log.Printf("========== '%v' IN CACHE", r)
				}
				_, err = os.Stat(path.Join(runInstance.vfsCachePath, id, r))
				if err != nil {
					log.Printf("========== '%v' not in vfs", r)
				} else {
					log.Printf("========== '%v' IN VFS", r)
				}
			}
			time.Sleep(time.Second * 10)
		}
	}()
	// routine to list, autostarts
	go func() {
		for {
			select {
			case <-readCh:
				log.Printf("========== finished checking listings and readings")
				return
			default:
			}
			li, err := runInstance.list(t, rootFs, "test")
			if err != nil {
				log.Printf("========== error listing 'test' folder: %v", err)
			} else {
				log.Printf("========== list 'test' folder count: %v", len(li))
			}

			time.Sleep(time.Second * 10)
		}
	}()

	// wait for main file to be written
	<-writeCh
	log.Printf("========== waiting for VFS to expire")
	time.Sleep(time.Second * 120)

	// try a final read
	li2 := [2]string{"test/one", "test/second"}
	for _, r := range li2 {
		_, err := runInstance.readDataFromRemote(t, rootFs, r, int64(0), int64(2), false)
		if err != nil {
			log.Printf("========== error reading '%v': %v", r, err)
		} else {
			log.Printf("========== read '%v'", r)
		}
	}
	// close the cache and list checkers
	cacheCh <- true
	readCh <- true
}

func TestInternalObjWrapFsFound(t *testing.T) {
	id := fmt.Sprintf("tiowff%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)
	wrappedFs := cfs.UnWrap()

	var testData []byte
	if runInstance.rootIsCrypt {
		testData, err = base64.StdEncoding.DecodeString(cryptedTextBase64)
		require.NoError(t, err)
	} else {
		testData = []byte("test content")
	}

	runInstance.writeObjectBytes(t, wrappedFs, runInstance.encryptRemoteIfNeeded(t, "test"), testData)
	listRoot, err := runInstance.list(t, rootFs, "")
	require.NoError(t, err)
	require.Len(t, listRoot, 1)

	cachedData, err := runInstance.readDataFromRemote(t, rootFs, "test", 0, int64(len([]byte("test content"))), false)
	require.NoError(t, err)
	require.Equal(t, "test content", string(cachedData))

	err = runInstance.rm(t, rootFs, "test")
	require.NoError(t, err)
	listRoot, err = runInstance.list(t, rootFs, "")
	require.NoError(t, err)
	require.Len(t, listRoot, 0)
}

func TestInternalObjNotFound(t *testing.T) {
	id := fmt.Sprintf("tionf%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	obj, err := rootFs.NewObject("404")
	require.Error(t, err)
	require.Nil(t, obj)
}

func TestInternalRemoteWrittenFileFoundInMount(t *testing.T) {
	if !runInstance.useMount {
		t.Skip("test needs mount mode")
	}
	id := fmt.Sprintf("tirwffim%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)

	var testData []byte
	if runInstance.rootIsCrypt {
		testData, err = base64.StdEncoding.DecodeString(cryptedTextBase64)
		require.NoError(t, err)
	} else {
		testData = []byte("test content")
	}

	runInstance.writeObjectBytes(t, cfs.UnWrap(), runInstance.encryptRemoteIfNeeded(t, "test"), testData)
	data, err := runInstance.readDataFromRemote(t, rootFs, "test", 0, int64(len([]byte("test content"))), false)
	require.NoError(t, err)
	require.Equal(t, "test content", string(data))
}

func TestInternalCachedWrittenContentMatches(t *testing.T) {
	id := fmt.Sprintf("ticwcm%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	// create some rand test data
	testData := runInstance.randomBytes(t, chunkSize*4+chunkSize/2)

	// write the object
	runInstance.writeRemoteBytes(t, rootFs, "data.bin", testData)

	// check sample of data from in-file
	sampleStart := chunkSize / 2
	sampleEnd := chunkSize
	testSample := testData[sampleStart:sampleEnd]
	checkSample, err := runInstance.readDataFromRemote(t, rootFs, "data.bin", sampleStart, sampleEnd, false)
	require.NoError(t, err)
	require.Equal(t, int64(len(checkSample)), sampleEnd-sampleStart)
	require.Equal(t, checkSample, testSample)
}

func TestInternalCachedUpdatedContentMatches(t *testing.T) {
	id := fmt.Sprintf("ticucm%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)
	var err error

	// create some rand test data
	var testData1 []byte
	var testData2 []byte
	if runInstance.rootIsCrypt {
		testData1, err = base64.StdEncoding.DecodeString(cryptedTextBase64)
		require.NoError(t, err)
		testData2, err = base64.StdEncoding.DecodeString(cryptedText2Base64)
		require.NoError(t, err)
	} else {
		testData1 = []byte(fstest.RandomString(100))
		testData2 = []byte(fstest.RandomString(200))
	}

	// write the object
	o := runInstance.updateObjectRemote(t, rootFs, "data.bin", testData1, testData2)
	require.Equal(t, o.Size(), int64(len(testData2)))

	// check data from in-file
	checkSample, err := runInstance.readDataFromRemote(t, rootFs, "data.bin", 0, int64(len(testData2)), false)
	require.NoError(t, err)
	require.Equal(t, checkSample, testData2)
}

func TestInternalWrappedWrittenContentMatches(t *testing.T) {
	id := fmt.Sprintf("tiwwcm%v", time.Now().Unix())
	vfsflags.Opt.DirCacheTime = time.Second
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)
	if runInstance.rootIsCrypt {
		t.Skip("test skipped with crypt remote")
	}

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	// create some rand test data
	testSize := chunkSize*4 + chunkSize/2
	testData := runInstance.randomBytes(t, testSize)

	// write the object
	o := runInstance.writeObjectBytes(t, cfs.UnWrap(), "data.bin", testData)
	require.Equal(t, o.Size(), int64(testSize))
	time.Sleep(time.Second * 3)

	data2, err := runInstance.readDataFromRemote(t, rootFs, "data.bin", 0, int64(testSize), false)
	require.NoError(t, err)
	require.Equal(t, int64(len(data2)), o.Size())

	// check sample of data from in-file
	sampleStart := chunkSize / 2
	sampleEnd := chunkSize
	testSample := testData[sampleStart:sampleEnd]
	checkSample, err := runInstance.readDataFromRemote(t, rootFs, "data.bin", sampleStart, sampleEnd, false)
	require.NoError(t, err)
	require.Equal(t, len(checkSample), len(testSample))

	for i := 0; i < len(checkSample); i++ {
		require.Equal(t, testSample[i], checkSample[i])
	}
}

func TestInternalLargeWrittenContentMatches(t *testing.T) {
	id := fmt.Sprintf("tilwcm%v", time.Now().Unix())
	vfsflags.Opt.DirCacheTime = time.Second
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)
	if runInstance.rootIsCrypt {
		t.Skip("test skipped with crypt remote")
	}

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	// create some rand test data
	testSize := chunkSize*10 + chunkSize/2
	testData := runInstance.randomBytes(t, testSize)

	// write the object
	runInstance.writeObjectBytes(t, cfs.UnWrap(), "data.bin", testData)
	time.Sleep(time.Second * 3)

	readData, err := runInstance.readDataFromRemote(t, rootFs, "data.bin", 0, testSize, false)
	require.NoError(t, err)
	for i := 0; i < len(readData); i++ {
		require.Equalf(t, testData[i], readData[i], "at byte %v", i)
	}
}

func TestInternalWrappedFsChangeNotSeen(t *testing.T) {
	id := fmt.Sprintf("tiwfcns%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	// create some rand test data
	testData := runInstance.randomBytes(t, (chunkSize*4 + chunkSize/2))
	runInstance.writeRemoteBytes(t, rootFs, "data.bin", testData)

	// update in the wrapped fs
	o, err := cfs.UnWrap().NewObject(runInstance.encryptRemoteIfNeeded(t, "data.bin"))
	require.NoError(t, err)
	wrappedTime := time.Now().Add(time.Hour * -1)
	err = o.SetModTime(wrappedTime)
	require.NoError(t, err)

	// get a new instance from the cache
	if runInstance.wrappedIsExternal {
		err = runInstance.retryBlock(func() error {
			coModTime, err := runInstance.modTime(t, rootFs, "data.bin")
			if err != nil {
				return err
			}
			if coModTime.Unix() != o.ModTime().Unix() {
				return errors.Errorf("%v <> %v", coModTime, o.ModTime())
			}
			return nil
		}, 12, time.Second*10)
		require.NoError(t, err)
	} else {
		coModTime, err := runInstance.modTime(t, rootFs, "data.bin")
		require.NoError(t, err)
		require.NotEqual(t, coModTime.Unix(), o.ModTime().Unix())
	}
}

func TestInternalChangeSeenAfterDirCacheFlush(t *testing.T) {
	id := fmt.Sprintf("ticsadcf%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	// create some rand test data
	testData := runInstance.randomBytes(t, (chunkSize*4 + chunkSize/2))
	runInstance.writeRemoteBytes(t, rootFs, "data.bin", testData)

	// update in the wrapped fs
	o, err := cfs.UnWrap().NewObject(runInstance.encryptRemoteIfNeeded(t, "data.bin"))
	require.NoError(t, err)
	wrappedTime := time.Now().Add(-1 * time.Hour)
	err = o.SetModTime(wrappedTime)
	require.NoError(t, err)

	// get a new instance from the cache
	co, err := rootFs.NewObject("data.bin")
	require.NoError(t, err)
	require.NotEqual(t, o.ModTime().String(), co.ModTime().String())

	cfs.DirCacheFlush() // flush the cache

	// get a new instance from the cache
	co, err = rootFs.NewObject("data.bin")
	require.NoError(t, err)
	require.Equal(t, wrappedTime.Unix(), co.ModTime().Unix())
}

func TestInternalCacheWrites(t *testing.T) {
	id := "ticw"
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, map[string]string{"cache-writes": "true"})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	// create some rand test data
	earliestTime := time.Now()
	testData := runInstance.randomBytes(t, (chunkSize*4 + chunkSize/2))
	runInstance.writeRemoteBytes(t, rootFs, "data.bin", testData)
	expectedTs := time.Now()
	ts, err := boltDb.GetChunkTs(runInstance.encryptRemoteIfNeeded(t, path.Join(rootFs.Root(), "data.bin")), 0)
	require.NoError(t, err)
	require.WithinDuration(t, expectedTs, ts, expectedTs.Sub(earliestTime))
}

func TestInternalMaxChunkSizeRespected(t *testing.T) {
	id := fmt.Sprintf("timcsr%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, map[string]string{"cache-workers": "1"})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()
	totalChunks := 20

	// create some rand test data
	testData := runInstance.randomBytes(t, (int64(totalChunks-1)*chunkSize + chunkSize/2))
	runInstance.writeRemoteBytes(t, rootFs, "data.bin", testData)
	o, err := cfs.NewObject(runInstance.encryptRemoteIfNeeded(t, "data.bin"))
	require.NoError(t, err)
	co, ok := o.(*cache.Object)
	require.True(t, ok)

	for i := 0; i < 4; i++ { // read first 4
		_ = runInstance.readDataFromObj(t, co, chunkSize*int64(i), chunkSize*int64(i+1), false)
	}
	cfs.CleanUpCache(true)
	// the last 2 **must** be in the cache
	require.True(t, boltDb.HasChunk(co, chunkSize*2))
	require.True(t, boltDb.HasChunk(co, chunkSize*3))

	for i := 4; i < 6; i++ { // read next 2
		_ = runInstance.readDataFromObj(t, co, chunkSize*int64(i), chunkSize*int64(i+1), false)
	}
	cfs.CleanUpCache(true)
	// the last 2 **must** be in the cache
	require.True(t, boltDb.HasChunk(co, chunkSize*4))
	require.True(t, boltDb.HasChunk(co, chunkSize*5))
}

func TestInternalExpiredEntriesRemoved(t *testing.T) {
	id := fmt.Sprintf("tieer%v", time.Now().Unix())
	vfsflags.Opt.DirCacheTime = time.Second * 4 // needs to be lower than the defined
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true, map[string]string{"info_age": "5s"}, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)
	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)

	// create some rand test data
	runInstance.writeRemoteString(t, rootFs, "one", "one content")
	runInstance.mkdir(t, rootFs, "test")
	runInstance.writeRemoteString(t, rootFs, "test/second", "second content")

	l, err := runInstance.list(t, rootFs, "test")
	require.NoError(t, err)
	require.Len(t, l, 1)

	err = cfs.UnWrap().Mkdir(runInstance.encryptRemoteIfNeeded(t, "test/third"))
	require.NoError(t, err)

	l, err = runInstance.list(t, rootFs, "test")
	require.NoError(t, err)
	require.Len(t, l, 1)

	err = runInstance.retryBlock(func() error {
		l, err = runInstance.list(t, rootFs, "test")
		if err != nil {
			return err
		}
		if len(l) != 2 {
			return errors.New("list is not 2")
		}
		return nil
	}, 10, time.Second)
	require.NoError(t, err)
}

func TestInternalUploadTempDirCreated(t *testing.T) {
	id := fmt.Sprintf("tiutdc%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true,
		nil,
		map[string]string{"cache-tmp-upload-path": path.Join(runInstance.tmpUploadDir, id)})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

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
		nil,
		map[string]string{"cache-tmp-upload-path": path.Join(runInstance.tmpUploadDir, id), "cache-tmp-wait-time": "0s"})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	testInternalUploadQueueOneFile(t, id, rootFs, boltDb)
}

func TestInternalUploadQueueOneFileWithRest(t *testing.T) {
	id := fmt.Sprintf("tiuqofwr%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true,
		nil,
		map[string]string{"cache-tmp-upload-path": path.Join(runInstance.tmpUploadDir, id), "cache-tmp-wait-time": "1m"})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	testInternalUploadQueueOneFile(t, id, rootFs, boltDb)
}

func TestInternalUploadQueueMoreFiles(t *testing.T) {
	id := fmt.Sprintf("tiuqmf%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true,
		nil,
		map[string]string{"cache-tmp-upload-path": path.Join(runInstance.tmpUploadDir, id), "cache-tmp-wait-time": "1s"})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	err := rootFs.Mkdir("test")
	require.NoError(t, err)
	minSize := 5242880
	maxSize := 10485760
	totalFiles := 10
	rand.Seed(time.Now().Unix())

	lastFile := ""
	for i := 0; i < totalFiles; i++ {
		size := int64(rand.Intn(maxSize-minSize) + minSize)
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
	tf, err := ioutil.ReadDir(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test")))
	require.NoError(t, err)
	require.Len(t, tf, 0)

	// check if cache lists all files
	de1, err = runInstance.list(t, rootFs, "test")
	require.NoError(t, err)
	require.Len(t, de1, totalFiles)
}

func TestInternalUploadTempFileOperations(t *testing.T) {
	id := "tiutfo"
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true,
		nil,
		map[string]string{"cache-tmp-upload-path": path.Join(runInstance.tmpUploadDir, id), "cache-tmp-wait-time": "1h"})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

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
		_, err = rootFs.NewObject("test/one")
		require.Error(t, err)
		_, err = rootFs.NewObject("second/one")
		require.NoError(t, err)
		// validate that it exists in temp fs
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
		require.Error(t, err)
		_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "second/one")))
		require.NoError(t, err)
		started, err := boltDb.SearchPendingUpload(runInstance.encryptRemoteIfNeeded(t, path.Join(id, "test/one")))
		require.Error(t, err)
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
	_, err = rootFs.NewObject("test/one")
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
		_, err = rootFs.NewObject("test/one")
		require.Error(t, err)
		_, err = rootFs.NewObject("test/second")
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
		_, err = rootFs.NewObject("test/one")
		require.NoError(t, err)
		_, err = rootFs.NewObject("test/third")
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
	_, err = rootFs.NewObject("test/one")
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
	obj2, err := rootFs.NewObject("test/one")
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
		nil,
		map[string]string{"cache-tmp-upload-path": path.Join(runInstance.tmpUploadDir, id), "cache-tmp-wait-time": "1h"})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

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
		_, err = rootFs.NewObject("test/one")
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
	_, err = rootFs.NewObject("test/one")
	require.NoError(t, err)
	// validate that it doesn't exist in temp fs
	_, err = os.Stat(path.Join(runInstance.tmpUploadDir, id, runInstance.encryptRemoteIfNeeded(t, "test/one")))
	require.NoError(t, err)

	// test Move/Rename
	err = runInstance.move(t, rootFs, path.Join("test", "one"), path.Join("test", "second"))
	if err != errNotSupported {
		require.Error(t, err)
		// try to read from it
		_, err = rootFs.NewObject("test/one")
		require.NoError(t, err)
		_, err = rootFs.NewObject("test/second")
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
		_, err = rootFs.NewObject("test/one")
		require.NoError(t, err)
		_, err = rootFs.NewObject("test/third")
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
	_, err = rootFs.NewObject("test/one")
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

// FIXME, enable this when mount is sorted out
//func TestInternalFilesMissingInMount1904(t *testing.T) {
//	t.Skip("Not yet")
//	if runtime.GOOS == "windows" {
//		t.Skip("Not yet")
//	}
//	id := "tifm1904"
//	rootFs, _ := newCacheFs(t, RemoteName, id, false,
//		map[string]string{"chunk_size": "5M", "info_age": "1m", "chunk_total_size": "500M", "cache-writes": "true"})
//	mntPoint := path.Join("/tmp", "tifm1904-mnt")
//	testPoint := path.Join(mntPoint, id)
//	checkOutput := "1 10 100 11 12 13 14 15 16 17 18 19 2 20 21 22 23 24 25 26 27 28 29 3 30 31 32 33 34 35 36 37 38 39 4 40 41 42 43 44 45 46 47 48 49 5 50 51 52 53 54 55 56 57 58 59 6 60 61 62 63 64 65 66 67 68 69 7 70 71 72 73 74 75 76 77 78 79 8 80 81 82 83 84 85 86 87 88 89 9 90 91 92 93 94 95 96 97 98 99 "
//
//	_ = os.MkdirAll(mntPoint, os.ModePerm)
//
//	list, err := rootFs.List("")
//	require.NoError(t, err)
//	found := false
//	list.ForDir(func(d fs.Directory) {
//		if strings.Contains(d.Remote(), id) {
//			found = true
//		}
//	})
//
//	if !found {
//		t.Skip("Test folder '%v' doesn't exist", id)
//	}
//
//	mountFs(t, rootFs, mntPoint)
//	defer unmountFs(t, mntPoint)
//
//	for i := 1; i <= 2; i++ {
//		out, err := exec.Command("ls", testPoint).Output()
//		require.NoError(t, err)
//		require.Equal(t, checkOutput, strings.Replace(string(out), "\n", " ", -1))
//		t.Logf("root path has all files")
//		_ = writeObjectString(t, rootFs, path.Join(id, strconv.Itoa(i), strconv.Itoa(i), "one_file"), "one content")
//
//		for j := 1; j <= 100; j++ {
//			out, err := exec.Command("ls", path.Join(testPoint, strconv.Itoa(j))).Output()
//			require.NoError(t, err)
//			require.Equal(t, checkOutput, strings.Replace(string(out), "\n", " ", -1), "'%v' doesn't match", j)
//		}
//		obj, err := rootFs.NewObject(path.Join(id, strconv.Itoa(i), strconv.Itoa(i), "one_file"))
//		require.NoError(t, err)
//		err = obj.Remove()
//		require.NoError(t, err)
//		t.Logf("folders contain all the files")
//
//		out, err = exec.Command("date").Output()
//		require.NoError(t, err)
//		t.Logf("check #%v date: '%v'", i, strings.Replace(string(out), "\n", " ", -1))
//
//		if i < 2 {
//			time.Sleep(time.Second * 60)
//		}
//	}
//}

// run holds the remotes for a test run
type run struct {
	okDiff            time.Duration
	allCfgMap         map[string]string
	allFlagMap        map[string]string
	runDefaultCfgMap  map[string]string
	runDefaultFlagMap map[string]string
	mntDir            string
	tmpUploadDir      string
	useMount          bool
	isMounted         bool
	rootIsCrypt       bool
	wrappedIsExternal bool
	unmountFn         func() error
	unmountRes        chan error
	vfs               *vfs.VFS
	tempFiles         []*os.File
	dbPath            string
	chunkPath         string
	vfsCachePath      string
}

func newRun() *run {
	var err error
	r := &run{
		okDiff:    time.Second * 9, // really big diff here but the build machines seem to be slow. need a different way for this
		useMount:  useMount,
		isMounted: false,
	}

	r.allCfgMap = map[string]string{
		"plex_url":         "",
		"plex_username":    "",
		"plex_password":    "",
		"chunk_size":       cache.DefCacheChunkSize,
		"info_age":         cache.DefCacheInfoAge,
		"chunk_total_size": cache.DefCacheTotalChunkSize,
	}
	r.allFlagMap = map[string]string{
		"cache-db-path":              filepath.Join(config.CacheDir, "cache-backend"),
		"cache-chunk-path":           filepath.Join(config.CacheDir, "cache-backend"),
		"cache-db-purge":             "true",
		"cache-chunk-size":           cache.DefCacheChunkSize,
		"cache-total-chunk-size":     cache.DefCacheTotalChunkSize,
		"cache-chunk-clean-interval": cache.DefCacheChunkCleanInterval,
		"cache-info-age":             cache.DefCacheInfoAge,
		"cache-read-retries":         strconv.Itoa(cache.DefCacheReadRetries),
		"cache-workers":              strconv.Itoa(cache.DefCacheTotalWorkers),
		"cache-chunk-no-memory":      "false",
		"cache-rps":                  strconv.Itoa(cache.DefCacheRps),
		"cache-writes":               "false",
		"cache-tmp-upload-path":      "",
		"cache-tmp-wait-time":        cache.DefCacheTmpWaitTime,
	}
	r.runDefaultCfgMap = make(map[string]string)
	for key, value := range r.allCfgMap {
		r.runDefaultCfgMap[key] = value
	}
	r.runDefaultFlagMap = make(map[string]string)
	for key, value := range r.allFlagMap {
		r.runDefaultFlagMap[key] = value
	}
	if mountDir == "" {
		if runtime.GOOS != "windows" {
			r.mntDir, err = ioutil.TempDir("", "rclonecache-mount")
			if err != nil {
				log.Fatalf("Failed to create mount dir: %v", err)
				return nil
			}
		} else {
			// Find a free drive letter
			drive := ""
			for letter := 'E'; letter <= 'Z'; letter++ {
				drive = string(letter) + ":"
				_, err := os.Stat(drive + "\\")
				if os.IsNotExist(err) {
					goto found
				}
			}
			log.Print("Couldn't find free drive letter for test")
		found:
			r.mntDir = drive
		}
	} else {
		r.mntDir = mountDir
	}
	log.Printf("Mount Dir: %v", r.mntDir)

	if uploadDir == "" {
		r.tmpUploadDir, err = ioutil.TempDir("", "rclonecache-tmp")
		if err != nil {
			log.Fatalf("Failed to create temp dir: %v", err)
		}
	} else {
		r.tmpUploadDir = uploadDir
	}
	log.Printf("Temp Upload Dir: %v", r.tmpUploadDir)

	return r
}

func (r *run) encryptRemoteIfNeeded(t *testing.T, remote string) string {
	if !runInstance.rootIsCrypt || len(decryptedToEncryptedRemotes) == 0 {
		return remote
	}

	enc, ok := decryptedToEncryptedRemotes[remote]
	if !ok {
		t.Fatalf("Failed to find decrypted -> encrypted mapping for '%v'", remote)
		return remote
	}
	return enc
}

func (r *run) newCacheFs(t *testing.T, remote, id string, needRemote, purge bool, cfg map[string]string, flags map[string]string) (fs.Fs, *cache.Persistent) {
	fstest.Initialise()
	remoteExists := false
	for _, s := range config.FileSections() {
		if s == remote {
			remoteExists = true
		}
	}
	if !remoteExists && needRemote {
		t.Skipf("Need remote (%v) to exist", remote)
		return nil, nil
	}

	// if the remote doesn't exist, create a new one with a local one for it
	// identify which is the cache remote (it can be wrapped by a crypt too)
	rootIsCrypt := false
	cacheRemote := remote
	if !remoteExists {
		localRemote := remote + "-local"
		config.FileSet(localRemote, "type", "local")
		config.FileSet(localRemote, "nounc", "true")
		config.FileSet(remote, "type", "cache")
		config.FileSet(remote, "remote", localRemote+":/var/tmp/"+localRemote)
	} else {
		remoteType := fs.ConfigFileGet(remote, "type", "")
		if remoteType == "" {
			t.Skipf("skipped due to invalid remote type for %v", remote)
			return nil, nil
		}
		if remoteType != "cache" {
			if remoteType == "crypt" {
				rootIsCrypt = true
				config.FileSet(remote, "password", cryptPassword1)
				config.FileSet(remote, "password2", cryptPassword2)
			}
			remoteRemote := fs.ConfigFileGet(remote, "remote", "")
			if remoteRemote == "" {
				t.Skipf("skipped due to invalid remote wrapper for %v", remote)
				return nil, nil
			}
			remoteRemoteParts := strings.Split(remoteRemote, ":")
			remoteWrapping := remoteRemoteParts[0]
			remoteType := fs.ConfigFileGet(remoteWrapping, "type", "")
			if remoteType != "cache" {
				t.Skipf("skipped due to invalid remote type for %v: '%v'", remoteWrapping, remoteType)
				return nil, nil
			}
			cacheRemote = remoteWrapping
		}
	}
	runInstance.rootIsCrypt = rootIsCrypt
	runInstance.dbPath = filepath.Join(config.CacheDir, "cache-backend", cacheRemote+".db")
	runInstance.chunkPath = filepath.Join(config.CacheDir, "cache-backend", cacheRemote)
	runInstance.vfsCachePath = filepath.Join(config.CacheDir, "vfs", remote)
	boltDb, err := cache.GetPersistent(runInstance.dbPath, runInstance.chunkPath, &cache.Features{PurgeDb: true})
	require.NoError(t, err)

	for k, v := range r.runDefaultCfgMap {
		if c, ok := cfg[k]; ok {
			config.FileSet(cacheRemote, k, c)
		} else {
			config.FileSet(cacheRemote, k, v)
		}
	}
	for k, v := range r.runDefaultFlagMap {
		if c, ok := flags[k]; ok {
			_ = flag.Set(k, c)
		} else {
			_ = flag.Set(k, v)
		}
	}
	fs.Config.LowLevelRetries = 1

	// Instantiate root
	if purge {
		boltDb.PurgeTempUploads()
		_ = os.RemoveAll(path.Join(runInstance.tmpUploadDir, id))
	}
	f, err := fs.NewFs(remote + ":" + id)
	require.NoError(t, err)
	cfs, err := r.getCacheFs(f)
	require.NoError(t, err)
	_, isCache := cfs.Features().UnWrap().(*cache.Fs)
	_, isCrypt := cfs.Features().UnWrap().(*crypt.Fs)
	_, isLocal := cfs.Features().UnWrap().(*local.Fs)
	if isCache || isCrypt || isLocal {
		r.wrappedIsExternal = false
	} else {
		r.wrappedIsExternal = true
	}

	if purge {
		_ = f.Features().Purge()
		require.NoError(t, err)
	}
	err = f.Mkdir("")
	require.NoError(t, err)
	if r.useMount && !r.isMounted {
		r.mountFs(t, f)
	}

	return f, boltDb
}

func (r *run) cleanupFs(t *testing.T, f fs.Fs, b *cache.Persistent) {
	if r.useMount && r.isMounted {
		r.unmountFs(t, f)
	}

	err := f.Features().Purge()
	require.NoError(t, err)
	cfs, err := r.getCacheFs(f)
	require.NoError(t, err)
	cfs.StopBackgroundRunners()

	if r.useMount && runtime.GOOS != "windows" {
		err = os.RemoveAll(r.mntDir)
		require.NoError(t, err)
	}
	err = os.RemoveAll(r.tmpUploadDir)
	require.NoError(t, err)

	for _, f := range r.tempFiles {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}
	r.tempFiles = nil
	debug.FreeOSMemory()
	for k, v := range r.runDefaultFlagMap {
		_ = flag.Set(k, v)
	}
}

func (r *run) randomBytes(t *testing.T, size int64) []byte {
	testData := make([]byte, size)
	testSize, err := rand.Read(testData)
	require.Equal(t, size, int64(len(testData)))
	require.Equal(t, size, int64(testSize))
	require.NoError(t, err)
	return testData
}

func (r *run) randomReader(t *testing.T, size int64) io.ReadCloser {
	chunk := int64(1024)
	cnt := size / chunk
	left := size % chunk
	f, err := ioutil.TempFile("", "rclonecache-tempfile")
	require.NoError(t, err)

	for i := 0; i < int(cnt); i++ {
		data := r.randomBytes(t, chunk)
		_, _ = f.Write(data)
	}
	data := r.randomBytes(t, int64(left))
	_, _ = f.Write(data)
	_, _ = f.Seek(int64(0), 0)
	r.tempFiles = append(r.tempFiles, f)

	return f
}

func (r *run) writeRemoteRandomBytes(t *testing.T, f fs.Fs, p string, size int64) string {
	remote := path.Join(p, strconv.Itoa(rand.Int())+".bin")
	// create some rand test data
	testData := r.randomBytes(t, size)

	r.writeRemoteBytes(t, f, remote, testData)
	return remote
}

func (r *run) writeObjectRandomBytes(t *testing.T, f fs.Fs, p string, size int64) fs.Object {
	remote := path.Join(p, strconv.Itoa(rand.Int())+".bin")
	// create some rand test data
	testData := r.randomBytes(t, size)

	return r.writeObjectBytes(t, f, remote, testData)
}

func (r *run) writeRemoteString(t *testing.T, f fs.Fs, remote, content string) {
	r.writeRemoteBytes(t, f, remote, []byte(content))
}

func (r *run) writeObjectString(t *testing.T, f fs.Fs, remote, content string) fs.Object {
	return r.writeObjectBytes(t, f, remote, []byte(content))
}

func (r *run) writeRemoteBytes(t *testing.T, f fs.Fs, remote string, data []byte) {
	var err error

	if r.useMount {
		err = r.retryBlock(func() error {
			return ioutil.WriteFile(path.Join(r.mntDir, remote), data, 0600)
		}, 3, time.Second*3)
		require.NoError(t, err)
		r.vfs.WaitForWriters(10 * time.Second)
	} else {
		r.writeObjectBytes(t, f, remote, data)
	}
}

func (r *run) writeRemoteReader(t *testing.T, f fs.Fs, remote string, in io.ReadCloser) {
	defer func() {
		_ = in.Close()
	}()

	if r.useMount {
		out, err := os.Create(path.Join(r.mntDir, remote))
		require.NoError(t, err)
		defer func() {
			_ = out.Close()
		}()

		_, err = io.Copy(out, in)
		require.NoError(t, err)
		r.vfs.WaitForWriters(10 * time.Second)
	} else {
		r.writeObjectReader(t, f, remote, in)
	}
}

func (r *run) writeObjectBytes(t *testing.T, f fs.Fs, remote string, data []byte) fs.Object {
	in := bytes.NewReader(data)
	_ = r.writeObjectReader(t, f, remote, in)
	o, err := f.NewObject(remote)
	require.NoError(t, err)
	require.Equal(t, int64(len(data)), o.Size())
	return o
}

func (r *run) writeObjectReader(t *testing.T, f fs.Fs, remote string, in io.Reader) fs.Object {
	modTime := time.Now()
	objInfo := object.NewStaticObjectInfo(remote, modTime, -1, true, nil, f)
	obj, err := f.Put(in, objInfo)
	require.NoError(t, err)
	if r.useMount {
		r.vfs.WaitForWriters(10 * time.Second)
	}

	return obj
}

func (r *run) updateObjectRemote(t *testing.T, f fs.Fs, remote string, data1 []byte, data2 []byte) fs.Object {
	var err error
	var obj fs.Object

	if r.useMount {
		err = ioutil.WriteFile(path.Join(r.mntDir, remote), data1, 0600)
		require.NoError(t, err)
		r.vfs.WaitForWriters(10 * time.Second)
		err = ioutil.WriteFile(path.Join(r.mntDir, remote), data2, 0600)
		require.NoError(t, err)
		r.vfs.WaitForWriters(10 * time.Second)
		obj, err = f.NewObject(remote)
	} else {
		in1 := bytes.NewReader(data1)
		in2 := bytes.NewReader(data2)
		objInfo1 := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data1)), true, nil, f)
		objInfo2 := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data2)), true, nil, f)

		obj, err = f.Put(in1, objInfo1)
		require.NoError(t, err)
		obj, err = f.NewObject(remote)
		require.NoError(t, err)
		err = obj.Update(in2, objInfo2)
	}
	require.NoError(t, err)

	return obj
}

func (r *run) readDataFromRemote(t *testing.T, f fs.Fs, remote string, offset, end int64, noLengthCheck bool) ([]byte, error) {
	size := end - offset
	checkSample := make([]byte, size)

	if r.useMount {
		f, err := os.Open(path.Join(r.mntDir, remote))
		defer func() {
			_ = f.Close()
		}()
		if err != nil {
			return checkSample, err
		}
		_, _ = f.Seek(offset, 0)
		totalRead, err := io.ReadFull(f, checkSample)
		checkSample = checkSample[:totalRead]
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			err = nil
		}
		if err != nil {
			return checkSample, err
		}
		if !noLengthCheck && size != int64(totalRead) {
			return checkSample, errors.Errorf("read size doesn't match expected: %v <> %v", totalRead, size)
		}
	} else {
		co, err := f.NewObject(remote)
		if err != nil {
			return checkSample, err
		}
		checkSample = r.readDataFromObj(t, co, offset, end, noLengthCheck)
	}
	if !noLengthCheck && size != int64(len(checkSample)) {
		return checkSample, errors.Errorf("read size doesn't match expected: %v <> %v", len(checkSample), size)
	}
	return checkSample, nil
}

func (r *run) readDataFromObj(t *testing.T, o fs.Object, offset, end int64, noLengthCheck bool) []byte {
	size := end - offset
	checkSample := make([]byte, size)
	reader, err := o.Open(&fs.SeekOption{Offset: offset})
	require.NoError(t, err)
	totalRead, err := io.ReadFull(reader, checkSample)
	if (err == io.EOF || err == io.ErrUnexpectedEOF) && noLengthCheck {
		err = nil
		checkSample = checkSample[:totalRead]
	}
	require.NoError(t, err)
	_ = reader.Close()
	return checkSample
}

func (r *run) mkdir(t *testing.T, f fs.Fs, remote string) {
	var err error
	if r.useMount {
		err = os.Mkdir(path.Join(r.mntDir, remote), 0700)
	} else {
		err = f.Mkdir(remote)
	}
	require.NoError(t, err)
}

func (r *run) rm(t *testing.T, f fs.Fs, remote string) error {
	var err error

	if r.useMount {
		err = os.Remove(path.Join(r.mntDir, remote))
	} else {
		var obj fs.Object
		obj, err = f.NewObject(remote)
		if err != nil {
			err = f.Rmdir(remote)
		} else {
			err = obj.Remove()
		}
	}

	return err
}

func (r *run) list(t *testing.T, f fs.Fs, remote string) ([]interface{}, error) {
	var err error
	var l []interface{}
	if r.useMount {
		var list []os.FileInfo
		list, err = ioutil.ReadDir(path.Join(r.mntDir, remote))
		for _, ll := range list {
			l = append(l, ll)
		}
	} else {
		var list fs.DirEntries
		list, err = f.List(remote)
		for _, ll := range list {
			l = append(l, ll)
		}
	}
	return l, err
}

func (r *run) listPath(t *testing.T, f fs.Fs, remote string) []string {
	var err error
	var l []string
	if r.useMount {
		var list []os.FileInfo
		list, err = ioutil.ReadDir(path.Join(r.mntDir, remote))
		for _, ll := range list {
			l = append(l, ll.Name())
		}
	} else {
		var list fs.DirEntries
		list, err = f.List(remote)
		for _, ll := range list {
			l = append(l, ll.Remote())
		}
	}
	require.NoError(t, err)
	return l
}

func (r *run) copyFile(t *testing.T, f fs.Fs, src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		_ = out.Close()
	}()

	_, err = io.Copy(out, in)
	return err
}

func (r *run) dirMove(t *testing.T, rootFs fs.Fs, src, dst string) error {
	var err error

	if runInstance.useMount {
		err = os.Rename(path.Join(runInstance.mntDir, src), path.Join(runInstance.mntDir, dst))
		if err != nil {
			return err
		}
		r.vfs.WaitForWriters(10 * time.Second)
	} else if rootFs.Features().DirMove != nil {
		err = rootFs.Features().DirMove(rootFs, src, dst)
		if err != nil {
			return err
		}
	} else {
		t.Logf("DirMove not supported by %v", rootFs)
		return errNotSupported
	}

	return err
}

func (r *run) move(t *testing.T, rootFs fs.Fs, src, dst string) error {
	var err error

	if runInstance.useMount {
		err = os.Rename(path.Join(runInstance.mntDir, src), path.Join(runInstance.mntDir, dst))
		if err != nil {
			return err
		}
		r.vfs.WaitForWriters(10 * time.Second)
	} else if rootFs.Features().Move != nil {
		obj1, err := rootFs.NewObject(src)
		if err != nil {
			return err
		}
		_, err = rootFs.Features().Move(obj1, dst)
		if err != nil {
			return err
		}
	} else {
		t.Logf("Move not supported by %v", rootFs)
		return errNotSupported
	}

	return err
}

func (r *run) copy(t *testing.T, rootFs fs.Fs, src, dst string) error {
	var err error

	if r.useMount {
		err = r.copyFile(t, rootFs, path.Join(r.mntDir, src), path.Join(r.mntDir, dst))
		if err != nil {
			return err
		}
		r.vfs.WaitForWriters(10 * time.Second)
	} else if rootFs.Features().Copy != nil {
		obj, err := rootFs.NewObject(src)
		if err != nil {
			return err
		}
		_, err = rootFs.Features().Copy(obj, dst)
		if err != nil {
			return err
		}
	} else {
		t.Logf("Copy not supported by %v", rootFs)
		return errNotSupported
	}

	return err
}

func (r *run) modTime(t *testing.T, rootFs fs.Fs, src string) (time.Time, error) {
	var err error

	if r.useMount {
		fi, err := os.Stat(path.Join(runInstance.mntDir, src))
		if err != nil {
			return time.Time{}, err
		}
		return fi.ModTime(), nil
	}
	obj1, err := rootFs.NewObject(src)
	if err != nil {
		return time.Time{}, err
	}
	return obj1.ModTime(), nil
}

func (r *run) updateData(t *testing.T, rootFs fs.Fs, src, data, append string) error {
	var err error

	if r.useMount {
		f, err := os.OpenFile(path.Join(runInstance.mntDir, src), os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		_, err = f.WriteString(data + append)
		if err != nil {
			_ = f.Close()
			return err
		}
		err = f.Close()
		if err != nil {
			return err
		}
		r.vfs.WaitForWriters(10 * time.Second)
	} else {
		obj1, err := rootFs.NewObject(src)
		if err != nil {
			return err
		}
		data1 := []byte(data + append)
		r := bytes.NewReader(data1)
		objInfo1 := object.NewStaticObjectInfo(src, time.Now(), int64(len(data1)), true, nil, rootFs)
		err = obj1.Update(r, objInfo1)
		if err != nil {
			return err
		}
	}

	return err
}

func (r *run) cleanSize(t *testing.T, size int64) int64 {
	if r.rootIsCrypt {
		denominator := int64(65536 + 16)
		size = size - 32
		quotient := size / denominator
		remainder := size % denominator
		return (quotient*65536 + remainder - 16)
	}

	return size
}

func (r *run) listenForBackgroundUpload(t *testing.T, f fs.Fs, remote string) chan error {
	cfs, err := r.getCacheFs(f)
	require.NoError(t, err)
	buCh := cfs.GetBackgroundUploadChannel()
	require.NotNil(t, buCh)
	maxDuration := time.Minute * 3
	if r.wrappedIsExternal {
		maxDuration = time.Minute * 10
	}

	waitCh := make(chan error)
	go func() {
		var err error
		var state cache.BackgroundUploadState

		for i := 0; i < 2; i++ {
			select {
			case state = <-buCh:
				// continue
			case <-time.After(maxDuration):
				waitCh <- errors.Errorf("Timed out waiting for background upload: %v", remote)
				return
			}
			checkRemote := state.Remote
			if r.rootIsCrypt {
				cryptFs := f.(*crypt.Fs)
				checkRemote, err = cryptFs.DecryptFileName(checkRemote)
				if err != nil {
					waitCh <- err
					return
				}
			}
			if checkRemote == remote && cache.BackgroundUploadStarted != state.Status {
				waitCh <- state.Error
				return
			}
		}
		waitCh <- errors.Errorf("Too many attempts to wait for the background upload: %v", remote)
	}()
	return waitCh
}

func (r *run) completeBackgroundUpload(t *testing.T, remote string, waitCh chan error) {
	var err error
	maxDuration := time.Minute * 3
	if r.wrappedIsExternal {
		maxDuration = time.Minute * 10
	}
	select {
	case err = <-waitCh:
		// continue
	case <-time.After(maxDuration):
		t.Fatalf("Timed out waiting to complete the background upload %v", remote)
		return
	}
	require.NoError(t, err)
}

func (r *run) completeAllBackgroundUploads(t *testing.T, f fs.Fs, lastRemote string) {
	var state cache.BackgroundUploadState
	var err error

	maxDuration := time.Minute * 5
	if r.wrappedIsExternal {
		maxDuration = time.Minute * 15
	}
	cfs, err := r.getCacheFs(f)
	require.NoError(t, err)
	buCh := cfs.GetBackgroundUploadChannel()
	require.NotNil(t, buCh)

	for {
		select {
		case state = <-buCh:
			checkRemote := state.Remote
			if r.rootIsCrypt {
				cryptFs := f.(*crypt.Fs)
				checkRemote, err = cryptFs.DecryptFileName(checkRemote)
				require.NoError(t, err)
			}
			if checkRemote == lastRemote && cache.BackgroundUploadCompleted == state.Status {
				require.NoError(t, state.Error)
				return
			}
		case <-time.After(maxDuration):
			t.Fatalf("Timed out waiting to complete the background upload %v", lastRemote)
			return
		}
	}
}

func (r *run) retryBlock(block func() error, maxRetries int, rate time.Duration) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = block()
		if err == nil {
			return nil
		}
		time.Sleep(rate)
	}
	return err
}

func (r *run) getCacheFs(f fs.Fs) (*cache.Fs, error) {
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

	return nil, errors.New("didn't found a cache fs")
}

var (
	_ fs.Fs = (*cache.Fs)(nil)
	_ fs.Fs = (*local.Fs)(nil)
)
