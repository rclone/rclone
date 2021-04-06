// +build !plan9,!js
// +build !race

package cache_test

import (
	"bytes"
	"context"
	"encoding/base64"
	goflag "flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/cache"
	"github.com/rclone/rclone/backend/crypt"
	_ "github.com/rclone/rclone/backend/drive"
	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/testy"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/stretchr/testify/require"
)

const (
	// these 2 passwords are test random
	cryptPassword1     = "3XcvMMdsV3d-HGAReTMdNH-5FcX5q32_lUeA"                                                     // oGJdUbQc7s8
	cryptPassword2     = "NlgTBEIe-qibA7v-FoMfuX6Cw8KlLai_aMvV"                                                     // mv4mZW572HM
	cryptedTextBase64  = "UkNMT05FAAC320i2xIee0BiNyknSPBn+Qcw3q9FhIFp3tvq6qlqvbsno3PnxmEFeJG3jDBnR/wku2gHWeQ=="     // one content
	cryptedText2Base64 = "UkNMT05FAAATcQkVsgjBh8KafCKcr0wdTa1fMmV0U8hsCLGFoqcvxKVmvv7wx3Hf5EXxFcki2FFV4sdpmSrb9Q==" // updated content
	cryptedText3Base64 = "UkNMT05FAAB/f7YtYKbPfmk9+OX/ffN3qG3OEdWT+z74kxCX9V/YZwJ4X2DN3HOnUC3gKQ4Gcoud5UtNvQ=="     // test content
	letterBytes        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

var (
	remoteName                  string
	uploadDir                   string
	runInstance                 *run
	errNotSupported             = errors.New("not supported")
	decryptedToEncryptedRemotes = map[string]string{
		"one":                  "lm4u7jjt3c85bf56vjqgeenuno",
		"second":               "qvt1ochrkcfbptp5mu9ugb2l14",
		"test":                 "jn4tegjtpqro30t3o11thb4b5s",
		"test2":                "qakvqnh8ttei89e0gc76crpql4",
		"data.bin":             "0q2847tfko6mhj3dag3r809qbc",
		"ticw/data.bin":        "5mv97b0ule6pht33srae5pice8/0q2847tfko6mhj3dag3r809qbc",
		"tiuufo/test/one":      "vi6u1olqhirqv14cd8qlej1mgo/jn4tegjtpqro30t3o11thb4b5s/lm4u7jjt3c85bf56vjqgeenuno",
		"tiuufo/test/second":   "vi6u1olqhirqv14cd8qlej1mgo/jn4tegjtpqro30t3o11thb4b5s/qvt1ochrkcfbptp5mu9ugb2l14",
		"tiutfo/test/one":      "legd371aa8ol36tjfklt347qnc/jn4tegjtpqro30t3o11thb4b5s/lm4u7jjt3c85bf56vjqgeenuno",
		"tiutfo/second/one":    "legd371aa8ol36tjfklt347qnc/qvt1ochrkcfbptp5mu9ugb2l14/lm4u7jjt3c85bf56vjqgeenuno",
		"second/one":           "qvt1ochrkcfbptp5mu9ugb2l14/lm4u7jjt3c85bf56vjqgeenuno",
		"test/one":             "jn4tegjtpqro30t3o11thb4b5s/lm4u7jjt3c85bf56vjqgeenuno",
		"test/second":          "jn4tegjtpqro30t3o11thb4b5s/qvt1ochrkcfbptp5mu9ugb2l14",
		"one/test":             "lm4u7jjt3c85bf56vjqgeenuno/jn4tegjtpqro30t3o11thb4b5s",
		"one/test/data.bin":    "lm4u7jjt3c85bf56vjqgeenuno/jn4tegjtpqro30t3o11thb4b5s/0q2847tfko6mhj3dag3r809qbc",
		"second/test/data.bin": "qvt1ochrkcfbptp5mu9ugb2l14/jn4tegjtpqro30t3o11thb4b5s/0q2847tfko6mhj3dag3r809qbc",
		"test/third":           "jn4tegjtpqro30t3o11thb4b5s/2nd7fjiop5h3ihfj1vl953aa5g",
		"test/0.bin":           "jn4tegjtpqro30t3o11thb4b5s/e6frddt058b6kvbpmlstlndmtk",
		"test/1.bin":           "jn4tegjtpqro30t3o11thb4b5s/kck472nt1k7qbmob0mt1p1crgc",
		"test/2.bin":           "jn4tegjtpqro30t3o11thb4b5s/744oe9ven2rmak4u27if51qk24",
		"test/3.bin":           "jn4tegjtpqro30t3o11thb4b5s/2bjd8kef0u5lmsu6qhqll34bcs",
		"test/4.bin":           "jn4tegjtpqro30t3o11thb4b5s/cvjs73iv0a82v0c7r67avllh7s",
		"test/5.bin":           "jn4tegjtpqro30t3o11thb4b5s/0plkdo790b6bnmt33qsdqmhv9c",
		"test/6.bin":           "jn4tegjtpqro30t3o11thb4b5s/s5r633srnjtbh83893jovjt5d0",
		"test/7.bin":           "jn4tegjtpqro30t3o11thb4b5s/6rq45tr9bjsammku622flmqsu4",
		"test/8.bin":           "jn4tegjtpqro30t3o11thb4b5s/37bc6tcl3e31qb8cadvjb749vk",
		"test/9.bin":           "jn4tegjtpqro30t3o11thb4b5s/t4pr35hnls32789o8fk0chk1ec",
	}
)

func init() {
	goflag.StringVar(&remoteName, "remote-internal", "TestInternalCache", "Remote to test with, defaults to local filesystem")
	goflag.StringVar(&uploadDir, "upload-dir-internal", "", "")
}

// TestMain drives the tests
func TestMain(m *testing.M) {
	goflag.Parse()
	var rc int

	log.Printf("Running with the following params: \n remote: %v", remoteName)
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
	listInner, err := rootFs2.List(context.Background(), "")
	require.NoError(t, err)

	require.Len(t, listRoot, 1)
	require.Len(t, listRootInner, 1)
	require.Len(t, listInner, 1)
}

/* TODO: is this testing something?
func TestInternalVfsCache(t *testing.T) {
	vfsflags.Opt.DirCacheTime = time.Second * 30
	testSize := int64(524288000)

	vfsflags.Opt.CacheMode = vfs.CacheModeWrites
	id := "tiuufo"
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, true, true, nil, map[string]string{"writes": "true", "info_age": "1h"})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	err := rootFs.Mkdir(context.Background(), "test")
	require.NoError(t, err)
	runInstance.writeObjectString(t, rootFs, "test/second", "content")
	_, err = rootFs.List(context.Background(), "test")
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
*/

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

	obj, err := rootFs.NewObject(context.Background(), "404")
	require.Error(t, err)
	require.Nil(t, obj)
}

func TestInternalCachedWrittenContentMatches(t *testing.T) {
	testy.SkipUnreliable(t)
	id := fmt.Sprintf("ticwcm%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	// create some rand test data
	testData := randStringBytes(int(chunkSize*4 + chunkSize/2))

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

func TestInternalDoubleWrittenContentMatches(t *testing.T) {
	id := fmt.Sprintf("tidwcm%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	// write the object
	runInstance.writeRemoteString(t, rootFs, "one", "one content")
	err := runInstance.updateData(t, rootFs, "one", "one content", " updated")
	require.NoError(t, err)
	err = runInstance.updateData(t, rootFs, "one", "one content updated", " double")
	require.NoError(t, err)

	// check sample of data from in-file
	data, err := runInstance.readDataFromRemote(t, rootFs, "one", int64(0), int64(len("one content updated double")), true)
	require.NoError(t, err)
	require.Equal(t, "one content updated double", string(data))
}

func TestInternalCachedUpdatedContentMatches(t *testing.T) {
	testy.SkipUnreliable(t)
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
		testData1 = []byte(random.String(100))
		testData2 = []byte(random.String(200))
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
	testData := randStringBytes(int(testSize))

	// write the object
	o := runInstance.writeObjectBytes(t, cfs.UnWrap(), "data.bin", testData)
	require.Equal(t, o.Size(), testSize)
	time.Sleep(time.Second * 3)

	checkSample, err := runInstance.readDataFromRemote(t, rootFs, "data.bin", 0, testSize, false)
	require.NoError(t, err)
	require.Equal(t, int64(len(checkSample)), o.Size())

	for i := 0; i < len(checkSample); i++ {
		require.Equal(t, testData[i], checkSample[i])
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
	testData := randStringBytes(int(testSize))

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
	testData := randStringBytes(int(chunkSize*4 + chunkSize/2))
	runInstance.writeRemoteBytes(t, rootFs, "data.bin", testData)

	// update in the wrapped fs
	originalSize, err := runInstance.size(t, rootFs, "data.bin")
	require.NoError(t, err)
	log.Printf("original size: %v", originalSize)

	o, err := cfs.UnWrap().NewObject(context.Background(), runInstance.encryptRemoteIfNeeded(t, "data.bin"))
	require.NoError(t, err)
	expectedSize := int64(len([]byte("test content")))
	var data2 []byte
	if runInstance.rootIsCrypt {
		data2, err = base64.StdEncoding.DecodeString(cryptedText3Base64)
		require.NoError(t, err)
		expectedSize = expectedSize + 1 // FIXME newline gets in, likely test data issue
	} else {
		data2 = []byte("test content")
	}
	objInfo := object.NewStaticObjectInfo(runInstance.encryptRemoteIfNeeded(t, "data.bin"), time.Now(), int64(len(data2)), true, nil, cfs.UnWrap())
	err = o.Update(context.Background(), bytes.NewReader(data2), objInfo)
	require.NoError(t, err)
	require.Equal(t, int64(len(data2)), o.Size())
	log.Printf("updated size: %v", len(data2))

	// get a new instance from the cache
	if runInstance.wrappedIsExternal {
		err = runInstance.retryBlock(func() error {
			coSize, err := runInstance.size(t, rootFs, "data.bin")
			if err != nil {
				return err
			}
			if coSize != expectedSize {
				return errors.Errorf("%v <> %v", coSize, expectedSize)
			}
			return nil
		}, 12, time.Second*10)
		require.NoError(t, err)
	} else {
		coSize, err := runInstance.size(t, rootFs, "data.bin")
		require.NoError(t, err)
		require.NotEqual(t, coSize, expectedSize)
	}
}

func TestInternalMoveWithNotify(t *testing.T) {
	id := fmt.Sprintf("timwn%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)
	if !runInstance.wrappedIsExternal {
		t.Skipf("Not external")
	}

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)

	srcName := runInstance.encryptRemoteIfNeeded(t, "test") + "/" + runInstance.encryptRemoteIfNeeded(t, "one") + "/" + runInstance.encryptRemoteIfNeeded(t, "data.bin")
	dstName := runInstance.encryptRemoteIfNeeded(t, "test") + "/" + runInstance.encryptRemoteIfNeeded(t, "second") + "/" + runInstance.encryptRemoteIfNeeded(t, "data.bin")
	// create some rand test data
	var testData []byte
	if runInstance.rootIsCrypt {
		testData, err = base64.StdEncoding.DecodeString(cryptedTextBase64)
		require.NoError(t, err)
	} else {
		testData = []byte("test content")
	}
	_ = cfs.UnWrap().Mkdir(context.Background(), runInstance.encryptRemoteIfNeeded(t, "test"))
	_ = cfs.UnWrap().Mkdir(context.Background(), runInstance.encryptRemoteIfNeeded(t, "test/one"))
	_ = cfs.UnWrap().Mkdir(context.Background(), runInstance.encryptRemoteIfNeeded(t, "test/second"))
	srcObj := runInstance.writeObjectBytes(t, cfs.UnWrap(), srcName, testData)

	// list in mount
	_, err = runInstance.list(t, rootFs, "test")
	require.NoError(t, err)
	_, err = runInstance.list(t, rootFs, "test/one")
	require.NoError(t, err)

	// move file
	_, err = cfs.UnWrap().Features().Move(context.Background(), srcObj, dstName)
	require.NoError(t, err)

	err = runInstance.retryBlock(func() error {
		li, err := runInstance.list(t, rootFs, "test")
		if err != nil {
			log.Printf("err: %v", err)
			return err
		}
		if len(li) != 2 {
			log.Printf("not expected listing /test: %v", li)
			return errors.Errorf("not expected listing /test: %v", li)
		}

		li, err = runInstance.list(t, rootFs, "test/one")
		if err != nil {
			log.Printf("err: %v", err)
			return err
		}
		if len(li) != 0 {
			log.Printf("not expected listing /test/one: %v", li)
			return errors.Errorf("not expected listing /test/one: %v", li)
		}

		li, err = runInstance.list(t, rootFs, "test/second")
		if err != nil {
			log.Printf("err: %v", err)
			return err
		}
		if len(li) != 1 {
			log.Printf("not expected listing /test/second: %v", li)
			return errors.Errorf("not expected listing /test/second: %v", li)
		}
		if fi, ok := li[0].(os.FileInfo); ok {
			if fi.Name() != "data.bin" {
				log.Printf("not expected name: %v", fi.Name())
				return errors.Errorf("not expected name: %v", fi.Name())
			}
		} else if di, ok := li[0].(fs.DirEntry); ok {
			if di.Remote() != "test/second/data.bin" {
				log.Printf("not expected remote: %v", di.Remote())
				return errors.Errorf("not expected remote: %v", di.Remote())
			}
		} else {
			log.Printf("unexpected listing: %v", li)
			return errors.Errorf("unexpected listing: %v", li)
		}

		log.Printf("complete listing: %v", li)
		return nil
	}, 12, time.Second*10)
	require.NoError(t, err)
}

func TestInternalNotifyCreatesEmptyParts(t *testing.T) {
	id := fmt.Sprintf("tincep%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)
	if !runInstance.wrappedIsExternal {
		t.Skipf("Not external")
	}
	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)

	srcName := runInstance.encryptRemoteIfNeeded(t, "test") + "/" + runInstance.encryptRemoteIfNeeded(t, "one") + "/" + runInstance.encryptRemoteIfNeeded(t, "test")
	dstName := runInstance.encryptRemoteIfNeeded(t, "test") + "/" + runInstance.encryptRemoteIfNeeded(t, "one") + "/" + runInstance.encryptRemoteIfNeeded(t, "test2")
	// create some rand test data
	var testData []byte
	if runInstance.rootIsCrypt {
		testData, err = base64.StdEncoding.DecodeString(cryptedTextBase64)
		require.NoError(t, err)
	} else {
		testData = []byte("test content")
	}
	err = rootFs.Mkdir(context.Background(), "test")
	require.NoError(t, err)
	err = rootFs.Mkdir(context.Background(), "test/one")
	require.NoError(t, err)
	srcObj := runInstance.writeObjectBytes(t, cfs.UnWrap(), srcName, testData)

	// list in mount
	_, err = runInstance.list(t, rootFs, "test")
	require.NoError(t, err)
	_, err = runInstance.list(t, rootFs, "test/one")
	require.NoError(t, err)

	found := boltDb.HasEntry(path.Join(cfs.Root(), runInstance.encryptRemoteIfNeeded(t, "test")))
	require.True(t, found)
	boltDb.Purge()
	found = boltDb.HasEntry(path.Join(cfs.Root(), runInstance.encryptRemoteIfNeeded(t, "test")))
	require.False(t, found)

	// move file
	_, err = cfs.UnWrap().Features().Move(context.Background(), srcObj, dstName)
	require.NoError(t, err)

	err = runInstance.retryBlock(func() error {
		found = boltDb.HasEntry(path.Join(cfs.Root(), runInstance.encryptRemoteIfNeeded(t, "test")))
		if !found {
			log.Printf("not found /test")
			return errors.Errorf("not found /test")
		}
		found = boltDb.HasEntry(path.Join(cfs.Root(), runInstance.encryptRemoteIfNeeded(t, "test"), runInstance.encryptRemoteIfNeeded(t, "one")))
		if !found {
			log.Printf("not found /test/one")
			return errors.Errorf("not found /test/one")
		}
		found = boltDb.HasEntry(path.Join(cfs.Root(), runInstance.encryptRemoteIfNeeded(t, "test"), runInstance.encryptRemoteIfNeeded(t, "one"), runInstance.encryptRemoteIfNeeded(t, "test2")))
		if !found {
			log.Printf("not found /test/one/test2")
			return errors.Errorf("not found /test/one/test2")
		}
		li, err := runInstance.list(t, rootFs, "test/one")
		if err != nil {
			log.Printf("err: %v", err)
			return err
		}
		if len(li) != 1 {
			log.Printf("not expected listing /test/one: %v", li)
			return errors.Errorf("not expected listing /test/one: %v", li)
		}
		if fi, ok := li[0].(os.FileInfo); ok {
			if fi.Name() != "test2" {
				log.Printf("not expected name: %v", fi.Name())
				return errors.Errorf("not expected name: %v", fi.Name())
			}
		} else if di, ok := li[0].(fs.DirEntry); ok {
			if di.Remote() != "test/one/test2" {
				log.Printf("not expected remote: %v", di.Remote())
				return errors.Errorf("not expected remote: %v", di.Remote())
			}
		} else {
			log.Printf("unexpected listing: %v", li)
			return errors.Errorf("unexpected listing: %v", li)
		}
		log.Printf("complete listing /test/one/test2")
		return nil
	}, 12, time.Second*10)
	require.NoError(t, err)
}

func TestInternalChangeSeenAfterDirCacheFlush(t *testing.T) {
	id := fmt.Sprintf("ticsadcf%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, nil)
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	// create some rand test data
	testData := randStringBytes(int(chunkSize*4 + chunkSize/2))
	runInstance.writeRemoteBytes(t, rootFs, "data.bin", testData)

	// update in the wrapped fs
	o, err := cfs.UnWrap().NewObject(context.Background(), runInstance.encryptRemoteIfNeeded(t, "data.bin"))
	require.NoError(t, err)
	wrappedTime := time.Now().Add(-1 * time.Hour)
	err = o.SetModTime(context.Background(), wrappedTime)
	require.NoError(t, err)

	// get a new instance from the cache
	co, err := rootFs.NewObject(context.Background(), "data.bin")
	require.NoError(t, err)
	require.NotEqual(t, o.ModTime(context.Background()).String(), co.ModTime(context.Background()).String())

	cfs.DirCacheFlush() // flush the cache

	// get a new instance from the cache
	co, err = rootFs.NewObject(context.Background(), "data.bin")
	require.NoError(t, err)
	require.Equal(t, wrappedTime.Unix(), co.ModTime(context.Background()).Unix())
}

func TestInternalCacheWrites(t *testing.T) {
	id := "ticw"
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, map[string]string{"writes": "true"})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()

	// create some rand test data
	earliestTime := time.Now()
	testData := randStringBytes(int(chunkSize*4 + chunkSize/2))
	runInstance.writeRemoteBytes(t, rootFs, "data.bin", testData)
	expectedTs := time.Now()
	ts, err := boltDb.GetChunkTs(runInstance.encryptRemoteIfNeeded(t, path.Join(rootFs.Root(), "data.bin")), 0)
	require.NoError(t, err)
	require.WithinDuration(t, expectedTs, ts, expectedTs.Sub(earliestTime))
}

func TestInternalMaxChunkSizeRespected(t *testing.T) {
	id := fmt.Sprintf("timcsr%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil, map[string]string{"workers": "1"})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)
	chunkSize := cfs.ChunkSize()
	totalChunks := 20

	// create some rand test data
	testData := randStringBytes(int(int64(totalChunks-1)*chunkSize + chunkSize/2))
	runInstance.writeRemoteBytes(t, rootFs, "data.bin", testData)
	o, err := cfs.NewObject(context.Background(), runInstance.encryptRemoteIfNeeded(t, "data.bin"))
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

	err = cfs.UnWrap().Mkdir(context.Background(), runInstance.encryptRemoteIfNeeded(t, "test/third"))
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

func TestInternalBug2117(t *testing.T) {
	vfsflags.Opt.DirCacheTime = time.Second * 10

	id := fmt.Sprintf("tib2117%v", time.Now().Unix())
	rootFs, boltDb := runInstance.newCacheFs(t, remoteName, id, false, true, nil,
		map[string]string{"info_age": "72h", "chunk_clean_interval": "15m"})
	defer runInstance.cleanupFs(t, rootFs, boltDb)

	if runInstance.rootIsCrypt {
		t.Skipf("skipping crypt")
	}

	cfs, err := runInstance.getCacheFs(rootFs)
	require.NoError(t, err)

	err = cfs.UnWrap().Mkdir(context.Background(), "test")
	require.NoError(t, err)
	for i := 1; i <= 4; i++ {
		err = cfs.UnWrap().Mkdir(context.Background(), fmt.Sprintf("test/dir%d", i))
		require.NoError(t, err)

		for j := 1; j <= 4; j++ {
			err = cfs.UnWrap().Mkdir(context.Background(), fmt.Sprintf("test/dir%d/dir%d", i, j))
			require.NoError(t, err)

			runInstance.writeObjectString(t, cfs.UnWrap(), fmt.Sprintf("test/dir%d/dir%d/test.txt", i, j), "test")
		}
	}

	di, err := runInstance.list(t, rootFs, "test/dir1/dir2")
	require.NoError(t, err)
	log.Printf("len: %v", len(di))
	require.Len(t, di, 1)

	time.Sleep(time.Second * 30)

	di, err = runInstance.list(t, rootFs, "test/dir1/dir2")
	require.NoError(t, err)
	log.Printf("len: %v", len(di))
	require.Len(t, di, 1)

	di, err = runInstance.list(t, rootFs, "test/dir1")
	require.NoError(t, err)
	log.Printf("len: %v", len(di))
	require.Len(t, di, 4)

	di, err = runInstance.list(t, rootFs, "test")
	require.NoError(t, err)
	log.Printf("len: %v", len(di))
	require.Len(t, di, 4)
}

// run holds the remotes for a test run
type run struct {
	okDiff            time.Duration
	runDefaultCfgMap  configmap.Simple
	tmpUploadDir      string
	rootIsCrypt       bool
	wrappedIsExternal bool
	tempFiles         []*os.File
	dbPath            string
	chunkPath         string
	vfsCachePath      string
}

func newRun() *run {
	var err error
	r := &run{
		okDiff: time.Second * 9, // really big diff here but the build machines seem to be slow. need a different way for this
	}

	// Read in all the defaults for all the options
	fsInfo, err := fs.Find("cache")
	if err != nil {
		panic(fmt.Sprintf("Couldn't find cache remote: %v", err))
	}
	r.runDefaultCfgMap = configmap.Simple{}
	for _, option := range fsInfo.Options {
		r.runDefaultCfgMap.Set(option.Name, fmt.Sprint(option.Default))
	}

	if uploadDir == "" {
		r.tmpUploadDir, err = ioutil.TempDir("", "rclonecache-tmp")
		if err != nil {
			panic(fmt.Sprintf("Failed to create temp dir: %v", err))
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

	// Config to pass to NewFs
	m := configmap.Simple{}
	for k, v := range r.runDefaultCfgMap {
		m.Set(k, v)
	}
	for k, v := range flags {
		m.Set(k, v)
	}

	// if the remote doesn't exist, create a new one with a local one for it
	// identify which is the cache remote (it can be wrapped by a crypt too)
	rootIsCrypt := false
	cacheRemote := remote
	if !remoteExists {
		localRemote := remote + "-local"
		config.FileSet(localRemote, "type", "local")
		config.FileSet(localRemote, "nounc", "true")
		m.Set("type", "cache")
		m.Set("remote", localRemote+":"+filepath.Join(os.TempDir(), localRemote))
	} else {
		remoteType := config.FileGet(remote, "type")
		if remoteType == "" {
			t.Skipf("skipped due to invalid remote type for %v", remote)
			return nil, nil
		}
		if remoteType != "cache" {
			if remoteType == "crypt" {
				rootIsCrypt = true
				m.Set("password", cryptPassword1)
				m.Set("password2", cryptPassword2)
			}
			remoteRemote := config.FileGet(remote, "remote")
			if remoteRemote == "" {
				t.Skipf("skipped due to invalid remote wrapper for %v", remote)
				return nil, nil
			}
			remoteRemoteParts := strings.Split(remoteRemote, ":")
			remoteWrapping := remoteRemoteParts[0]
			remoteType := config.FileGet(remoteWrapping, "type")
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

	ci := fs.GetConfig(context.Background())
	ci.LowLevelRetries = 1

	// Instantiate root
	if purge {
		boltDb.PurgeTempUploads()
		_ = os.RemoveAll(path.Join(runInstance.tmpUploadDir, id))
	}
	f, err := cache.NewFs(context.Background(), remote, id, m)
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
		_ = f.Features().Purge(context.Background(), "")
		require.NoError(t, err)
	}
	err = f.Mkdir(context.Background(), "")
	require.NoError(t, err)
	return f, boltDb
}

func (r *run) cleanupFs(t *testing.T, f fs.Fs, b *cache.Persistent) {
	err := f.Features().Purge(context.Background(), "")
	require.NoError(t, err)
	cfs, err := r.getCacheFs(f)
	require.NoError(t, err)
	cfs.StopBackgroundRunners()

	err = os.RemoveAll(r.tmpUploadDir)
	require.NoError(t, err)

	for _, f := range r.tempFiles {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}
	r.tempFiles = nil
	debug.FreeOSMemory()
}

func (r *run) randomReader(t *testing.T, size int64) io.ReadCloser {
	chunk := int64(1024)
	cnt := size / chunk
	left := size % chunk
	f, err := ioutil.TempFile("", "rclonecache-tempfile")
	require.NoError(t, err)

	for i := 0; i < int(cnt); i++ {
		data := randStringBytes(int(chunk))
		_, _ = f.Write(data)
	}
	data := randStringBytes(int(left))
	_, _ = f.Write(data)
	_, _ = f.Seek(int64(0), io.SeekStart)
	r.tempFiles = append(r.tempFiles, f)

	return f
}

func (r *run) writeRemoteString(t *testing.T, f fs.Fs, remote, content string) {
	r.writeRemoteBytes(t, f, remote, []byte(content))
}

func (r *run) writeObjectString(t *testing.T, f fs.Fs, remote, content string) fs.Object {
	return r.writeObjectBytes(t, f, remote, []byte(content))
}

func (r *run) writeRemoteBytes(t *testing.T, f fs.Fs, remote string, data []byte) {
	r.writeObjectBytes(t, f, remote, data)
}

func (r *run) writeRemoteReader(t *testing.T, f fs.Fs, remote string, in io.ReadCloser) {
	r.writeObjectReader(t, f, remote, in)
}

func (r *run) writeObjectBytes(t *testing.T, f fs.Fs, remote string, data []byte) fs.Object {
	in := bytes.NewReader(data)
	_ = r.writeObjectReader(t, f, remote, in)
	o, err := f.NewObject(context.Background(), remote)
	require.NoError(t, err)
	require.Equal(t, int64(len(data)), o.Size())
	return o
}

func (r *run) writeObjectReader(t *testing.T, f fs.Fs, remote string, in io.Reader) fs.Object {
	modTime := time.Now()
	objInfo := object.NewStaticObjectInfo(remote, modTime, -1, true, nil, f)
	obj, err := f.Put(context.Background(), in, objInfo)
	require.NoError(t, err)
	return obj
}

func (r *run) updateObjectRemote(t *testing.T, f fs.Fs, remote string, data1 []byte, data2 []byte) fs.Object {
	var err error
	var obj fs.Object

	in1 := bytes.NewReader(data1)
	in2 := bytes.NewReader(data2)
	objInfo1 := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data1)), true, nil, f)
	objInfo2 := object.NewStaticObjectInfo(remote, time.Now(), int64(len(data2)), true, nil, f)

	_, err = f.Put(context.Background(), in1, objInfo1)
	require.NoError(t, err)
	obj, err = f.NewObject(context.Background(), remote)
	require.NoError(t, err)
	err = obj.Update(context.Background(), in2, objInfo2)
	require.NoError(t, err)

	return obj
}

func (r *run) readDataFromRemote(t *testing.T, f fs.Fs, remote string, offset, end int64, noLengthCheck bool) ([]byte, error) {
	size := end - offset
	checkSample := make([]byte, size)

	co, err := f.NewObject(context.Background(), remote)
	if err != nil {
		return checkSample, err
	}
	checkSample = r.readDataFromObj(t, co, offset, end, noLengthCheck)

	if !noLengthCheck && size != int64(len(checkSample)) {
		return checkSample, errors.Errorf("read size doesn't match expected: %v <> %v", len(checkSample), size)
	}
	return checkSample, nil
}

func (r *run) readDataFromObj(t *testing.T, o fs.Object, offset, end int64, noLengthCheck bool) []byte {
	size := end - offset
	checkSample := make([]byte, size)
	reader, err := o.Open(context.Background(), &fs.SeekOption{Offset: offset})
	require.NoError(t, err)
	totalRead, err := io.ReadFull(reader, checkSample)
	if (err == io.EOF || err == io.ErrUnexpectedEOF) && noLengthCheck {
		err = nil
		checkSample = checkSample[:totalRead]
	}
	require.NoError(t, err, "with string -%v-", string(checkSample))
	_ = reader.Close()
	return checkSample
}

func (r *run) mkdir(t *testing.T, f fs.Fs, remote string) {
	err := f.Mkdir(context.Background(), remote)
	require.NoError(t, err)
}

func (r *run) rm(t *testing.T, f fs.Fs, remote string) error {
	var err error

	var obj fs.Object
	obj, err = f.NewObject(context.Background(), remote)
	if err != nil {
		err = f.Rmdir(context.Background(), remote)
	} else {
		err = obj.Remove(context.Background())
	}

	return err
}

func (r *run) list(t *testing.T, f fs.Fs, remote string) ([]interface{}, error) {
	var err error
	var l []interface{}
	var list fs.DirEntries
	list, err = f.List(context.Background(), remote)
	for _, ll := range list {
		l = append(l, ll)
	}
	return l, err
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

	if rootFs.Features().DirMove != nil {
		err = rootFs.Features().DirMove(context.Background(), rootFs, src, dst)
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

	if rootFs.Features().Move != nil {
		obj1, err := rootFs.NewObject(context.Background(), src)
		if err != nil {
			return err
		}
		_, err = rootFs.Features().Move(context.Background(), obj1, dst)
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

	if rootFs.Features().Copy != nil {
		obj, err := rootFs.NewObject(context.Background(), src)
		if err != nil {
			return err
		}
		_, err = rootFs.Features().Copy(context.Background(), obj, dst)
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

	obj1, err := rootFs.NewObject(context.Background(), src)
	if err != nil {
		return time.Time{}, err
	}
	return obj1.ModTime(context.Background()), nil
}

func (r *run) size(t *testing.T, rootFs fs.Fs, src string) (int64, error) {
	var err error

	obj1, err := rootFs.NewObject(context.Background(), src)
	if err != nil {
		return int64(0), err
	}
	return obj1.Size(), nil
}

func (r *run) updateData(t *testing.T, rootFs fs.Fs, src, data, append string) error {
	var err error

	var obj1 fs.Object
	obj1, err = rootFs.NewObject(context.Background(), src)
	if err != nil {
		return err
	}
	data1 := []byte(data + append)
	reader := bytes.NewReader(data1)
	objInfo1 := object.NewStaticObjectInfo(src, time.Now(), int64(len(data1)), true, nil, rootFs)
	err = obj1.Update(context.Background(), reader, objInfo1)

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
	}
	if f.Features().UnWrap != nil {
		cfs, ok := f.Features().UnWrap().(*cache.Fs)
		if ok {
			return cfs, nil
		}
	}
	return nil, errors.New("didn't found a cache fs")
}

func randStringBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return b
}

var (
	_ fs.Fs = (*cache.Fs)(nil)
	_ fs.Fs = (*local.Fs)(nil)
)
