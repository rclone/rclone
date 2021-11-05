// Specific Unix-only tests

//go:build !windows
// +build !windows

package configfile

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func WriteToPipe(name string, data string) {
	time.Sleep(100 * time.Millisecond)
	f, err := os.OpenFile(name, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		panic(err)
	}
	f.Write([]byte(data))
	f.Close()
}

func FillStorageFromPipe(name string, stor *Storage, wg *sync.WaitGroup) {
	defer wg.Done()
	time.Sleep(100 * time.Millisecond)
	f, err := os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, os.ModeNamedPipe)
	if err != nil {
		panic(err)
	}
	data, err := ioutil.ReadAll(f)
	var b bytes.Buffer
	stor.dataReader = bufio.NewWriter(&b)
	//io.Copy(stor.dataReader, bytes.NewReader(data))
	if err != nil {
		panic(err)
	}
	f.Close()
}

// b rclone/fs/config/configfile/configfile_unix_test.go:
func TestConfigFileFIFO(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	fifoName := config.GetConfigPath() + ".fifo"
	require.NoError(t, syscall.Mkfifo(fifoName, 0600))

	savedPath := config.GetConfigPath()
	config.SetConfigPath(fifoName)
	defer config.SetConfigPath(savedPath)
	data := &Storage{}

	go WriteToPipe(fifoName, configData)
	go FillStorageFromPipe(fifoName, data, &wg)

	loadErr := data.Load()
	assert.NoError(t, loadErr)

	buf := new(bytes.Buffer)
	io.Copy(buf, data.dataReader)
	assert.Equal(t, configData, buf.String())

	wg.Wait()
	defer func() {
		assert.NoError(t, os.Remove(fifoName))
	}()
}

/*
testData := "[sect1]\nvar1=data1\n"

// sops creates fifo BEFORE exec, right?
testFifoFd, err := mkfifo(testFifoPath)
go func() {
   // but sops can be slow to fill fifo, right?
   time.Sleep(100 * time.Millisecond) // simulate slow sops
  ... Write(testFifo, testData) ...
  wg.Done()
}()

// wg.Wait WOULD be here if SOPS guaranteed to finish writing BEFORE exec - but I assume it can be lazy, right?
// wg.Wait()

// point rclone at test fifo - just for test
savedPath := config.GetConfigPath()
config.SetConfigPath(testFifoPath)
defer func() {
	config.SetConfigPath(savedPath)
}()

stor := &configfile.Storage{}
err := stor.Load() // test must not stuck here!
assert.NoError(err) // must be able to read fifo!

// Wait being here means that Sops might delay writing - I assume yes
wg.Wait()

// ReadAll in _loadFifo must wait till the data end, **but not get stuck**!
// will it get stuck if sops is keeping fifo open until exec end?
// if it deadlocks,  then ReadAll in _loadFifo must be replaced by something else,
// something that exits after a reasonable timeout, e.g. 1-2 seconds (will sops write longer?)

// we get here if Load didn't deadlock
// that's fine, but did it read FULL contents? let's check
// please fix this part after me!
// stor.dataReader might have been already emptied by _load!
// if yes, then instead of dataReader you'd better pass dataBytes!
assert.Equal(testData, readAll(stor.dataReader))

// more detailed read test
val1, err := stor.GetValue("sect1","val1")
assert.NoError...
assert.Equal("data1",val1)

// writing to fifo must fail or warn loudly - see comment below
errSet := stor.SetValue("sect1","val1","data2")
// assert.Error(errSet)

// sops will remove fifo only AFTER rclone exits, right?
_ = testFifoFd.Close()
_ = os.Remove(testFifoPath)

// etc etc

*/
