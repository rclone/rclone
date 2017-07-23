package sftp

import (
	"fmt"
	"io"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var _ = fmt.Print

type csPair struct {
	cli *Client
	svr *RequestServer
}

// these must be closed in order, else client.Close will hang
func (cs csPair) Close() {
	cs.svr.Close()
	cs.cli.Close()
	os.Remove(sock)
}

func (cs csPair) testHandler() *root {
	return cs.svr.Handlers.FileGet.(*root)
}

const sock = "/tmp/rstest.sock"

func clientRequestServerPair(t *testing.T) *csPair {
	ready := make(chan bool)
	os.Remove(sock) // either this or signal handling
	var server *RequestServer
	go func() {
		l, err := net.Listen("unix", sock)
		if err != nil {
			// neither assert nor t.Fatal reliably exit before Accept errors
			panic(err)
		}
		ready <- true
		fd, err := l.Accept()
		assert.Nil(t, err)
		handlers := InMemHandler()
		server = NewRequestServer(fd, handlers)
		server.Serve()
	}()
	<-ready
	defer os.Remove(sock)
	c, err := net.Dial("unix", sock)
	assert.Nil(t, err)
	client, err := NewClientPipe(c, c)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	return &csPair{client, server}
}

// after adding logging, maybe check log to make sure packet handling
// was split over more than one worker
func TestRequestSplitWrite(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	w, err := p.cli.Create("/foo")
	assert.Nil(t, err)
	p.cli.maxPacket = 3 // force it to send in small chunks
	contents := "one two three four five six seven eight nine ten"
	w.Write([]byte(contents))
	w.Close()
	r := p.testHandler()
	f, _ := r.fetch("/foo")
	assert.Equal(t, contents, string(f.content))
}

func TestRequestCache(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	foo := NewRequest("", "foo")
	bar := NewRequest("", "bar")
	fh := p.svr.nextRequest(foo)
	bh := p.svr.nextRequest(bar)
	assert.Len(t, p.svr.openRequests, 2)
	_foo, ok := p.svr.getRequest(fh)
	assert.Equal(t, foo, _foo)
	assert.True(t, ok)
	_, ok = p.svr.getRequest("zed")
	assert.False(t, ok)
	p.svr.closeRequest(fh)
	p.svr.closeRequest(bh)
	assert.Len(t, p.svr.openRequests, 0)
}

func TestRequestCacheState(t *testing.T) {
	// test operation that uses open/close
	p := clientRequestServerPair(t)
	defer p.Close()
	_, err := putTestFile(p.cli, "/foo", "hello")
	assert.Nil(t, err)
	assert.Len(t, p.svr.openRequests, 0)
	// test operation that doesn't open/close
	err = p.cli.Remove("/foo")
	assert.Nil(t, err)
	assert.Len(t, p.svr.openRequests, 0)
}

func putTestFile(cli *Client, path, content string) (int, error) {
	w, err := cli.Create(path)
	if err == nil {
		defer w.Close()
		return w.Write([]byte(content))
	}
	return 0, err
}

func TestRequestWrite(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	n, err := putTestFile(p.cli, "/foo", "hello")
	assert.Nil(t, err)
	assert.Equal(t, 5, n)
	r := p.testHandler()
	f, err := r.fetch("/foo")
	assert.Nil(t, err)
	assert.False(t, f.isdir)
	assert.Equal(t, f.content, []byte("hello"))
}

// needs fail check
func TestRequestFilename(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	_, err := putTestFile(p.cli, "/foo", "hello")
	assert.Nil(t, err)
	r := p.testHandler()
	f, err := r.fetch("/foo")
	assert.Nil(t, err)
	assert.Equal(t, f.Name(), "foo")
}

func TestRequestRead(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	_, err := putTestFile(p.cli, "/foo", "hello")
	assert.Nil(t, err)
	rf, err := p.cli.Open("/foo")
	assert.Nil(t, err)
	defer rf.Close()
	contents := make([]byte, 5)
	n, err := rf.Read(contents)
	if err != nil && err != io.EOF {
		t.Fatalf("err: %v", err)
	}
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", string(contents[0:5]))
}

func TestRequestReadFail(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	rf, err := p.cli.Open("/foo")
	assert.Nil(t, err)
	contents := make([]byte, 5)
	n, err := rf.Read(contents)
	assert.Equal(t, n, 0)
	assert.Exactly(t, os.ErrNotExist, err)
}

func TestRequestOpen(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	fh, err := p.cli.Open("foo")
	assert.Nil(t, err)
	err = fh.Close()
	assert.Nil(t, err)
}

func TestRequestMkdir(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	err := p.cli.Mkdir("/foo")
	assert.Nil(t, err)
	r := p.testHandler()
	f, err := r.fetch("/foo")
	assert.Nil(t, err)
	assert.True(t, f.isdir)
}

func TestRequestRemove(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	_, err := putTestFile(p.cli, "/foo", "hello")
	assert.Nil(t, err)
	r := p.testHandler()
	_, err = r.fetch("/foo")
	assert.Nil(t, err)
	err = p.cli.Remove("/foo")
	assert.Nil(t, err)
	_, err = r.fetch("/foo")
	assert.Equal(t, err, os.ErrNotExist)
}

func TestRequestRename(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	_, err := putTestFile(p.cli, "/foo", "hello")
	assert.Nil(t, err)
	r := p.testHandler()
	_, err = r.fetch("/foo")
	assert.Nil(t, err)
	err = p.cli.Rename("/foo", "/bar")
	assert.Nil(t, err)
	_, err = r.fetch("/bar")
	assert.Nil(t, err)
	_, err = r.fetch("/foo")
	assert.Equal(t, err, os.ErrNotExist)
}

func TestRequestRenameFail(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	_, err := putTestFile(p.cli, "/foo", "hello")
	assert.Nil(t, err)
	_, err = putTestFile(p.cli, "/bar", "goodbye")
	assert.Nil(t, err)
	err = p.cli.Rename("/foo", "/bar")
	assert.IsType(t, &StatusError{}, err)
}

func TestRequestStat(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	_, err := putTestFile(p.cli, "/foo", "hello")
	assert.Nil(t, err)
	fi, err := p.cli.Stat("/foo")
	assert.Equal(t, fi.Name(), "foo")
	assert.Equal(t, fi.Size(), int64(5))
	assert.Equal(t, fi.Mode(), os.FileMode(0644))
	assert.NoError(t, testOsSys(fi.Sys()))
}

// NOTE: Setstat is a noop in the request server tests, but we want to test
// that is does nothing without crapping out.
func TestRequestSetstat(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	_, err := putTestFile(p.cli, "/foo", "hello")
	assert.Nil(t, err)
	mode := os.FileMode(0644)
	err = p.cli.Chmod("/foo", mode)
	assert.Nil(t, err)
	fi, err := p.cli.Stat("/foo")
	assert.Nil(t, err)
	assert.Equal(t, fi.Name(), "foo")
	assert.Equal(t, fi.Size(), int64(5))
	assert.Equal(t, fi.Mode(), os.FileMode(0644))
	assert.NoError(t, testOsSys(fi.Sys()))
}

func TestRequestFstat(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	_, err := putTestFile(p.cli, "/foo", "hello")
	assert.Nil(t, err)
	fp, err := p.cli.Open("/foo")
	assert.Nil(t, err)
	fi, err := fp.Stat()
	assert.Nil(t, err)
	assert.Equal(t, fi.Name(), "foo")
	assert.Equal(t, fi.Size(), int64(5))
	assert.Equal(t, fi.Mode(), os.FileMode(0644))
	assert.NoError(t, testOsSys(fi.Sys()))
}

func TestRequestStatFail(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	fi, err := p.cli.Stat("/foo")
	assert.Nil(t, fi)
	assert.True(t, os.IsNotExist(err))
}

func TestRequestSymlink(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	_, err := putTestFile(p.cli, "/foo", "hello")
	assert.Nil(t, err)
	err = p.cli.Symlink("/foo", "/bar")
	assert.Nil(t, err)
	r := p.testHandler()
	fi, err := r.fetch("/bar")
	assert.Nil(t, err)
	assert.True(t, fi.Mode()&os.ModeSymlink == os.ModeSymlink)
}

func TestRequestSymlinkFail(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	err := p.cli.Symlink("/foo", "/bar")
	assert.True(t, os.IsNotExist(err))
}

func TestRequestReadlink(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	_, err := putTestFile(p.cli, "/foo", "hello")
	assert.Nil(t, err)
	err = p.cli.Symlink("/foo", "/bar")
	assert.Nil(t, err)
	rl, err := p.cli.ReadLink("/bar")
	assert.Nil(t, err)
	assert.Equal(t, "foo", rl)
}

func TestRequestReaddir(t *testing.T) {
	p := clientRequestServerPair(t)
	defer p.Close()
	for i := 0; i < 100; i++ {
		fname := fmt.Sprintf("/foo_%02d", i)
		_, err := putTestFile(p.cli, fname, fname)
		assert.Nil(t, err)
	}
	di, err := p.cli.ReadDir("/")
	assert.Nil(t, err)
	assert.Len(t, di, 100)
	names := []string{di[18].Name(), di[81].Name()}
	assert.Equal(t, []string{"foo_18", "foo_81"}, names)
}
