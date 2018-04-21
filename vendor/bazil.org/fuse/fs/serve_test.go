package fs_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fs/fstestutil"
	"bazil.org/fuse/fs/fstestutil/record"
	"bazil.org/fuse/fuseutil"
	"bazil.org/fuse/syscallx"
	"golang.org/x/net/context"
)

// TO TEST:
//	Lookup(*LookupRequest, *LookupResponse)
//	Getattr(*GetattrRequest, *GetattrResponse)
//	Attr with explicit inode
//	Setattr(*SetattrRequest, *SetattrResponse)
//	Access(*AccessRequest)
//	Open(*OpenRequest, *OpenResponse)
//	Write(*WriteRequest, *WriteResponse)
//	Flush(*FlushRequest, *FlushResponse)

func init() {
	fstestutil.DebugByDefault()
}

// symlink can be embedded in a struct to make it look like a symlink.
type symlink struct {
	target string
}

func (f symlink) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeSymlink | 0666
	return nil
}

// fifo can be embedded in a struct to make it look like a named pipe.
type fifo struct{}

func (f fifo) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeNamedPipe | 0666
	return nil
}

func TestMountpointDoesNotExist(t *testing.T) {
	t.Parallel()
	tmp, err := ioutil.TempDir("", "fusetest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp)

	mountpoint := path.Join(tmp, "does-not-exist")
	conn, err := fuse.Mount(mountpoint)
	if err == nil {
		conn.Close()
		t.Fatalf("expected error with non-existent mountpoint")
	}
	if _, ok := err.(*fuse.MountpointDoesNotExistError); !ok {
		t.Fatalf("wrong error from mount: %T: %v", err, err)
	}
}

type badRootFS struct{}

func (badRootFS) Root() (fs.Node, error) {
	// pick a really distinct error, to identify it later
	return nil, fuse.Errno(syscall.ENAMETOOLONG)
}

func TestRootErr(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, badRootFS{}, nil)
	if err == nil {
		// path for synchronous mounts (linux): started out fine, now
		// wait for Serve to cycle through
		err = <-mnt.Error
		// without this, unmount will keep failing with EBUSY; nudge
		// kernel into realizing InitResponse will not happen
		mnt.Conn.Close()
		mnt.Close()
	}

	if err == nil {
		t.Fatal("expected an error")
	}
	// TODO this should not be a textual comparison, Serve hides
	// details
	if err.Error() != "cannot obtain root node: file name too long" {
		t.Errorf("Unexpected error: %v", err)
	}
}

type testPanic struct{}

type panicSentinel struct{}

var _ error = panicSentinel{}

func (panicSentinel) Error() string { return "just a test" }

var _ fuse.ErrorNumber = panicSentinel{}

func (panicSentinel) Errno() fuse.Errno {
	return fuse.Errno(syscall.ENAMETOOLONG)
}

func (f testPanic) Root() (fs.Node, error) {
	return f, nil
}

func (f testPanic) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 1
	a.Mode = os.ModeDir | 0777
	return nil
}

func (f testPanic) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	panic(panicSentinel{})
}

func TestPanic(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, testPanic{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = os.Mkdir(mnt.Dir+"/trigger-a-panic", 0700)
	if nerr, ok := err.(*os.PathError); !ok || nerr.Err != syscall.ENAMETOOLONG {
		t.Fatalf("wrong error from panicking handler: %T: %v", err, err)
	}
}

type testStatFS struct{}

func (f testStatFS) Root() (fs.Node, error) {
	return f, nil
}

func (f testStatFS) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 1
	a.Mode = os.ModeDir | 0777
	return nil
}

func (f testStatFS) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error {
	resp.Blocks = 42
	resp.Bfree = 10
	resp.Bavail = 3
	resp.Files = 13
	resp.Ffree = 11
	resp.Bsize = 1000
	resp.Namelen = 34
	resp.Frsize = 7
	return nil
}

func TestStatfs(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, testStatFS{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	// Perform an operation that forces the OS X mount to be ready, so
	// we know the Statfs handler will really be called. OS X insists
	// on volumes answering Statfs calls very early (before FUSE
	// handshake), so OSXFUSE gives made-up answers for a few brief moments
	// during the mount process.
	if _, err := os.Stat(mnt.Dir + "/does-not-exist"); !os.IsNotExist(err) {
		t.Fatal(err)
	}

	{
		var st syscall.Statfs_t
		err = syscall.Statfs(mnt.Dir, &st)
		if err != nil {
			t.Errorf("Statfs failed: %v", err)
		}
		t.Logf("Statfs got: %#v", st)
		if g, e := st.Blocks, uint64(42); g != e {
			t.Errorf("got Blocks = %d; want %d", g, e)
		}
		if g, e := st.Bfree, uint64(10); g != e {
			t.Errorf("got Bfree = %d; want %d", g, e)
		}
		if g, e := st.Bavail, uint64(3); g != e {
			t.Errorf("got Bavail = %d; want %d", g, e)
		}
		if g, e := st.Files, uint64(13); g != e {
			t.Errorf("got Files = %d; want %d", g, e)
		}
		if g, e := st.Ffree, uint64(11); g != e {
			t.Errorf("got Ffree = %d; want %d", g, e)
		}
		if g, e := st.Bsize, int64(1000); g != e {
			t.Errorf("got Bsize = %d; want %d", g, e)
		}
		if g, e := st.Namelen, int64(34); g != e {
			t.Errorf("got Namelen = %d; want %d", g, e)
		}
		if g, e := st.Frsize, int64(7); g != e {
			t.Errorf("got Frsize = %d; want %d", g, e)
		}
	}

	{
		var st syscall.Statfs_t
		f, err := os.Open(mnt.Dir)
		if err != nil {
			t.Errorf("Open for fstatfs failed: %v", err)
		}
		defer f.Close()
		err = syscall.Fstatfs(int(f.Fd()), &st)
		if err != nil {
			t.Errorf("Fstatfs failed: %v", err)
		}
		t.Logf("Fstatfs got: %#v", st)
		if g, e := st.Blocks, uint64(42); g != e {
			t.Errorf("got Blocks = %d; want %d", g, e)
		}
		if g, e := st.Files, uint64(13); g != e {
			t.Errorf("got Files = %d; want %d", g, e)
		}
	}
}

// Test Stat of root.

type root struct{}

func (f root) Root() (fs.Node, error) {
	return f, nil
}

func (root) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 1
	a.Mode = os.ModeDir | 0555
	// This has to be a power of two, but try to pick something that's an unlikely default.
	a.BlockSize = 65536
	return nil
}

func TestStatRoot(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, root{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	fi, err := os.Stat(mnt.Dir)
	if err != nil {
		t.Fatalf("root getattr failed with %v", err)
	}
	mode := fi.Mode()
	if (mode & os.ModeType) != os.ModeDir {
		t.Errorf("root is not a directory: %#v", fi)
	}
	if mode.Perm() != 0555 {
		t.Errorf("root has weird access mode: %v", mode.Perm())
	}
	switch stat := fi.Sys().(type) {
	case *syscall.Stat_t:
		if stat.Ino != 1 {
			t.Errorf("root has wrong inode: %v", stat.Ino)
		}
		if stat.Nlink != 1 {
			t.Errorf("root has wrong link count: %v", stat.Nlink)
		}
		if stat.Uid != 0 {
			t.Errorf("root has wrong uid: %d", stat.Uid)
		}
		if stat.Gid != 0 {
			t.Errorf("root has wrong gid: %d", stat.Gid)
		}
		if mnt.Conn.Protocol().HasAttrBlockSize() {
			// convert stat.Blksize too because it's int64 on Linux but
			// int32 on Darwin.
			if g, e := int64(stat.Blksize), int64(65536); g != e {
				t.Errorf("root has wrong blocksize: %d != %d", g, e)
			}
		}
	}
}

// Test Read calling ReadAll.

type readAll struct {
	fstestutil.File
}

const hi = "hello, world"

func (readAll) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = 0666
	a.Size = uint64(len(hi))
	return nil
}

func (readAll) ReadAll(ctx context.Context) ([]byte, error) {
	return []byte(hi), nil
}

func testReadAll(t *testing.T, path string) {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data := make([]byte, 4096)
	n, err := f.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if g, e := string(data[:n]), hi; g != e {
		t.Errorf("readAll = %q, want %q", g, e)
	}
}

func TestReadAll(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": readAll{}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	testReadAll(t, mnt.Dir+"/child")
}

// Test Read.

type readWithHandleRead struct {
	fstestutil.File
}

func (readWithHandleRead) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = 0666
	a.Size = uint64(len(hi))
	return nil
}

func (readWithHandleRead) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	fuseutil.HandleRead(req, resp, []byte(hi))
	return nil
}

func TestReadAllWithHandleRead(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": readWithHandleRead{}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	testReadAll(t, mnt.Dir+"/child")
}

type readFlags struct {
	fstestutil.File
	fileFlags record.Recorder
}

func (r *readFlags) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = 0666
	a.Size = uint64(len(hi))
	return nil
}

func (r *readFlags) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	r.fileFlags.Record(req.FileFlags)
	fuseutil.HandleRead(req, resp, []byte(hi))
	return nil
}

func TestReadFileFlags(t *testing.T) {
	t.Parallel()
	r := &readFlags{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": r}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	if !mnt.Conn.Protocol().HasReadWriteFlags() {
		t.Skip("Old FUSE protocol")
	}

	f, err := os.OpenFile(mnt.Dir+"/child", os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Read(make([]byte, 4096)); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	want := fuse.OpenReadWrite | fuse.OpenAppend
	if runtime.GOOS == "darwin" {
		// OSXFUSE shares one read and one write handle for all
		// clients, so it uses a OpenReadOnly handle for performing
		// our read.
		//
		// If this test starts failing in the future, that probably
		// means they added the feature, and we want to notice that!
		want = fuse.OpenReadOnly
	}
	if g, e := r.fileFlags.Recorded().(fuse.OpenFlags), want; g != e {
		t.Errorf("read saw file flags %+v, want %+v", g, e)
	}
}

type writeFlags struct {
	fstestutil.File
	fileFlags record.Recorder
}

func (r *writeFlags) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = 0666
	a.Size = uint64(len(hi))
	return nil
}

func (r *writeFlags) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	// OSXFUSE 3.0.4 does a read-modify-write cycle even when the
	// write was for 4096 bytes.
	fuseutil.HandleRead(req, resp, []byte(hi))
	return nil
}

func (r *writeFlags) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	r.fileFlags.Record(req.FileFlags)
	resp.Size = len(req.Data)
	return nil
}

func TestWriteFileFlags(t *testing.T) {
	t.Parallel()
	r := &writeFlags{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": r}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	if !mnt.Conn.Protocol().HasReadWriteFlags() {
		t.Skip("Old FUSE protocol")
	}

	f, err := os.OpenFile(mnt.Dir+"/child", os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(make([]byte, 4096)); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	want := fuse.OpenReadWrite | fuse.OpenAppend
	if runtime.GOOS == "darwin" {
		// OSXFUSE shares one read and one write handle for all
		// clients, so it uses a OpenWriteOnly handle for performing
		// our read.
		//
		// If this test starts failing in the future, that probably
		// means they added the feature, and we want to notice that!
		want = fuse.OpenWriteOnly
	}
	if g, e := r.fileFlags.Recorded().(fuse.OpenFlags), want; g != e {
		t.Errorf("write saw file flags %+v, want %+v", g, e)
	}
}

// Test Release.

type release struct {
	fstestutil.File
	record.ReleaseWaiter
}

func TestRelease(t *testing.T) {
	t.Parallel()
	r := &release{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": r}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	f, err := os.Open(mnt.Dir + "/child")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if !r.WaitForRelease(1 * time.Second) {
		t.Error("Close did not Release in time")
	}
}

// Test Write calling basic Write, with an fsync thrown in too.

type write struct {
	fstestutil.File
	record.Writes
	record.Fsyncs
}

func TestWrite(t *testing.T) {
	t.Parallel()
	w := &write{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": w}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	f, err := os.Create(mnt.Dir + "/child")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer f.Close()
	n, err := f.Write([]byte(hi))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(hi) {
		t.Fatalf("short write; n=%d; hi=%d", n, len(hi))
	}

	err = syscall.Fsync(int(f.Fd()))
	if err != nil {
		t.Fatalf("Fsync = %v", err)
	}
	if w.RecordedFsync() == (fuse.FsyncRequest{}) {
		t.Errorf("never received expected fsync call")
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := string(w.RecordedWriteData()); got != hi {
		t.Errorf("write = %q, want %q", got, hi)
	}
}

// Test Write of a larger buffer.

type writeLarge struct {
	fstestutil.File
	record.Writes
}

func TestWriteLarge(t *testing.T) {
	t.Parallel()
	w := &write{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": w}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	f, err := os.Create(mnt.Dir + "/child")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer f.Close()
	const one = "xyzzyfoo"
	large := bytes.Repeat([]byte(one), 8192)
	n, err := f.Write(large)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if g, e := n, len(large); g != e {
		t.Fatalf("short write: %d != %d", g, e)
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := w.RecordedWriteData()
	if g, e := len(got), len(large); g != e {
		t.Errorf("write wrong length: %d != %d", g, e)
	}
	if g := strings.Replace(string(got), one, "", -1); g != "" {
		t.Errorf("write wrong data: expected repeats of %q, also got %q", one, g)
	}
}

// Test Write calling Setattr+Write+Flush.

type writeTruncateFlush struct {
	fstestutil.File
	record.Writes
	record.Setattrs
	record.Flushes
}

func TestWriteTruncateFlush(t *testing.T) {
	t.Parallel()
	w := &writeTruncateFlush{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": w}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = ioutil.WriteFile(mnt.Dir+"/child", []byte(hi), 0666)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if w.RecordedSetattr() == (fuse.SetattrRequest{}) {
		t.Errorf("writeTruncateFlush expected Setattr")
	}
	if !w.RecordedFlush() {
		t.Errorf("writeTruncateFlush expected Setattr")
	}
	if got := string(w.RecordedWriteData()); got != hi {
		t.Errorf("writeTruncateFlush = %q, want %q", got, hi)
	}
}

// Test Mkdir.

type mkdir1 struct {
	fstestutil.Dir
	record.Mkdirs
}

func (f *mkdir1) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	f.Mkdirs.Mkdir(ctx, req)
	return &mkdir1{}, nil
}

func TestMkdir(t *testing.T) {
	f := &mkdir1{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	// uniform umask needed to make os.Mkdir's mode into something
	// reproducible
	defer syscall.Umask(syscall.Umask(0022))
	err = os.Mkdir(mnt.Dir+"/foo", 0771)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := fuse.MkdirRequest{Name: "foo", Mode: os.ModeDir | 0751}
	if mnt.Conn.Protocol().HasUmask() {
		want.Umask = 0022
	}
	if runtime.GOOS == "darwin" {
		// https://github.com/osxfuse/osxfuse/issues/225
		want.Umask = 0
	}
	if g, e := f.RecordedMkdir(), want; g != e {
		t.Errorf("mkdir saw %+v, want %+v", g, e)
	}
}

// Test Create (and fsync)

type create1file struct {
	fstestutil.File
	record.Creates
	record.Fsyncs
}

type create1 struct {
	fstestutil.Dir
	f create1file
}

func (f *create1) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	if req.Name != "foo" {
		log.Printf("ERROR create1.Create unexpected name: %q\n", req.Name)
		return nil, nil, fuse.EPERM
	}

	_, _, _ = f.f.Creates.Create(ctx, req, resp)
	return &f.f, &f.f, nil
}

func TestCreate(t *testing.T) {
	f := &create1{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	// uniform umask needed to make os.Create's 0666 into something
	// reproducible
	defer syscall.Umask(syscall.Umask(0022))
	ff, err := os.OpenFile(mnt.Dir+"/foo", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		t.Fatalf("create1 WriteFile: %v", err)
	}
	defer ff.Close()

	want := fuse.CreateRequest{
		Name:  "foo",
		Flags: fuse.OpenReadWrite | fuse.OpenCreate | fuse.OpenTruncate,
		Mode:  0640,
	}
	if mnt.Conn.Protocol().HasUmask() {
		want.Umask = 0022
	}
	if runtime.GOOS == "darwin" {
		// OS X does not pass O_TRUNC here, Linux does; as this is a
		// Create, that's acceptable
		want.Flags &^= fuse.OpenTruncate

		// https://github.com/osxfuse/osxfuse/issues/225
		want.Umask = 0
	}
	got := f.f.RecordedCreate()
	if runtime.GOOS == "linux" {
		// Linux <3.7 accidentally leaks O_CLOEXEC through to FUSE;
		// avoid spurious test failures
		got.Flags &^= fuse.OpenFlags(syscall.O_CLOEXEC)
	}
	if g, e := got, want; g != e {
		t.Fatalf("create saw %+v, want %+v", g, e)
	}

	err = syscall.Fsync(int(ff.Fd()))
	if err != nil {
		t.Fatalf("Fsync = %v", err)
	}

	if f.f.RecordedFsync() == (fuse.FsyncRequest{}) {
		t.Errorf("never received expected fsync call")
	}

	ff.Close()
}

// Test Create + Write + Remove

type create3file struct {
	fstestutil.File
	record.Writes
}

type create3 struct {
	fstestutil.Dir
	f          create3file
	fooCreated record.MarkRecorder
	fooRemoved record.MarkRecorder
}

func (f *create3) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	if req.Name != "foo" {
		log.Printf("ERROR create3.Create unexpected name: %q\n", req.Name)
		return nil, nil, fuse.EPERM
	}
	f.fooCreated.Mark()
	return &f.f, &f.f, nil
}

func (f *create3) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if f.fooCreated.Recorded() && !f.fooRemoved.Recorded() && name == "foo" {
		return &f.f, nil
	}
	return nil, fuse.ENOENT
}

func (f *create3) Remove(ctx context.Context, r *fuse.RemoveRequest) error {
	if f.fooCreated.Recorded() && !f.fooRemoved.Recorded() &&
		r.Name == "foo" && !r.Dir {
		f.fooRemoved.Mark()
		return nil
	}
	return fuse.ENOENT
}

func TestCreateWriteRemove(t *testing.T) {
	t.Parallel()
	f := &create3{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = ioutil.WriteFile(mnt.Dir+"/foo", []byte(hi), 0666)
	if err != nil {
		t.Fatalf("create3 WriteFile: %v", err)
	}
	if got := string(f.f.RecordedWriteData()); got != hi {
		t.Fatalf("create3 write = %q, want %q", got, hi)
	}

	err = os.Remove(mnt.Dir + "/foo")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	err = os.Remove(mnt.Dir + "/foo")
	if err == nil {
		t.Fatalf("second Remove = nil; want some error")
	}
}

// Test symlink + readlink

// is a Node that is a symlink to target
type symlink1link struct {
	symlink
	target string
}

func (f symlink1link) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	return f.target, nil
}

type symlink1 struct {
	fstestutil.Dir
	record.Symlinks
}

func (f *symlink1) Symlink(ctx context.Context, req *fuse.SymlinkRequest) (fs.Node, error) {
	f.Symlinks.Symlink(ctx, req)
	return symlink1link{target: req.Target}, nil
}

func TestSymlink(t *testing.T) {
	t.Parallel()
	f := &symlink1{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	const target = "/some-target"

	err = os.Symlink(target, mnt.Dir+"/symlink.file")
	if err != nil {
		t.Fatalf("os.Symlink: %v", err)
	}

	want := fuse.SymlinkRequest{NewName: "symlink.file", Target: target}
	if g, e := f.RecordedSymlink(), want; g != e {
		t.Errorf("symlink saw %+v, want %+v", g, e)
	}

	gotName, err := os.Readlink(mnt.Dir + "/symlink.file")
	if err != nil {
		t.Fatalf("os.Readlink: %v", err)
	}
	if gotName != target {
		t.Errorf("os.Readlink = %q; want %q", gotName, target)
	}
}

// Test link

type link1 struct {
	fstestutil.Dir
	record.Links
}

func (f *link1) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if name == "old" {
		return fstestutil.File{}, nil
	}
	return nil, fuse.ENOENT
}

func (f *link1) Link(ctx context.Context, r *fuse.LinkRequest, old fs.Node) (fs.Node, error) {
	f.Links.Link(ctx, r, old)
	return fstestutil.File{}, nil
}

func TestLink(t *testing.T) {
	t.Parallel()
	f := &link1{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = os.Link(mnt.Dir+"/old", mnt.Dir+"/new")
	if err != nil {
		t.Fatalf("Link: %v", err)
	}

	got := f.RecordedLink()
	want := fuse.LinkRequest{
		NewName: "new",
		// unpredictable
		OldNode: got.OldNode,
	}
	if g, e := got, want; g != e {
		t.Fatalf("link saw %+v, want %+v", g, e)
	}
}

// Test Rename

type rename1 struct {
	fstestutil.Dir
	renamed record.Counter
}

func (f *rename1) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if name == "old" {
		return fstestutil.File{}, nil
	}
	return nil, fuse.ENOENT
}

func (f *rename1) Rename(ctx context.Context, r *fuse.RenameRequest, newDir fs.Node) error {
	if r.OldName == "old" && r.NewName == "new" && newDir == f {
		f.renamed.Inc()
		return nil
	}
	return fuse.EIO
}

func TestRename(t *testing.T) {
	t.Parallel()
	f := &rename1{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = os.Rename(mnt.Dir+"/old", mnt.Dir+"/new")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if g, e := f.renamed.Count(), uint32(1); g != e {
		t.Fatalf("expected rename didn't happen: %d != %d", g, e)
	}
	err = os.Rename(mnt.Dir+"/old2", mnt.Dir+"/new2")
	if err == nil {
		t.Fatal("expected error on second Rename; got nil")
	}
}

// Test mknod

type mknod1 struct {
	fstestutil.Dir
	record.Mknods
}

func (f *mknod1) Mknod(ctx context.Context, r *fuse.MknodRequest) (fs.Node, error) {
	f.Mknods.Mknod(ctx, r)
	return fifo{}, nil
}

func TestMknod(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipping unless root")
	}

	f := &mknod1{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	defer syscall.Umask(syscall.Umask(0022))
	err = syscall.Mknod(mnt.Dir+"/node", syscall.S_IFIFO|0660, 123)
	if err != nil {
		t.Fatalf("mknod: %v", err)
	}

	want := fuse.MknodRequest{
		Name: "node",
		Mode: os.FileMode(os.ModeNamedPipe | 0640),
		Rdev: uint32(123),
	}
	if runtime.GOOS == "linux" {
		// Linux fuse doesn't echo back the rdev if the node
		// isn't a device (we're using a FIFO here, as that
		// bit is portable.)
		want.Rdev = 0
	}
	if mnt.Conn.Protocol().HasUmask() {
		want.Umask = 0022
	}
	if runtime.GOOS == "darwin" {
		// https://github.com/osxfuse/osxfuse/issues/225
		want.Umask = 0
	}
	if g, e := f.RecordedMknod(), want; g != e {
		t.Fatalf("mknod saw %+v, want %+v", g, e)
	}
}

// Test Read served with DataHandle.

type dataHandleTest struct {
	fstestutil.File
}

func (dataHandleTest) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = 0666
	a.Size = uint64(len(hi))
	return nil
}

func (dataHandleTest) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	return fs.DataHandle([]byte(hi)), nil
}

func TestDataHandle(t *testing.T) {
	t.Parallel()
	f := &dataHandleTest{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	data, err := ioutil.ReadFile(mnt.Dir + "/child")
	if err != nil {
		t.Errorf("readAll: %v", err)
		return
	}
	if string(data) != hi {
		t.Errorf("readAll = %q, want %q", data, hi)
	}
}

// Test interrupt

type interrupt struct {
	fstestutil.File

	// strobes to signal we have a read hanging
	hanging chan struct{}
}

func (interrupt) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = 0666
	a.Size = 1
	return nil
}

func (it *interrupt) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	select {
	case it.hanging <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return ctx.Err()
}

func helperInterrupt() {
	log.SetPrefix("interrupt child: ")
	log.SetFlags(0)

	log.Printf("starting...")

	f, err := os.Open("child")
	if err != nil {
		log.Fatalf("cannot open file: %v", err)
	}
	defer f.Close()

	log.Printf("reading...")
	buf := make([]byte, 4096)
	n, err := syscall.Read(int(f.Fd()), buf)
	switch err {
	case nil:
		log.Fatalf("read: expected error, got data: %q", buf[:n])
	case syscall.EINTR:
		log.Printf("read: saw EINTR, all good")
	default:
		log.Fatalf("read: wrong error: %v", err)
	}

	log.Printf("exiting...")
}

func init() {
	childHelpers["interrupt"] = helperInterrupt
}

func TestInterrupt(t *testing.T) {
	t.Parallel()
	f := &interrupt{}
	f.hanging = make(chan struct{}, 1)
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	// start a subprocess that can hang until signaled
	child, err := childCmd("interrupt")
	if err != nil {
		t.Fatal(err)
	}
	child.Dir = mnt.Dir

	if err := child.Start(); err != nil {
		t.Errorf("cannot start child: %v", err)
		return
	}

	// try to clean up if child is still alive when returning
	defer child.Process.Kill()

	// wait till we're sure it's hanging in read
	<-f.hanging

	//	err = child.Process.Signal(os.Interrupt)
	var sig os.Signal = syscall.SIGIO
	if runtime.GOOS == "darwin" {
		// I can't get OSXFUSE 3.2.0 to trigger EINTR return from
		// read(2), at least in a Go application. Works on Linux. So,
		// on OS X, we just check that the signal at least kills the
		// child, aborting the read, so operations on hanging FUSE
		// filesystems can be aborted.
		sig = os.Interrupt
	}

	err = child.Process.Signal(sig)
	if err != nil {
		t.Errorf("cannot interrupt child: %v", err)
		return
	}

	p, err := child.Process.Wait()
	if err != nil {
		t.Errorf("child failed: %v", err)
		return
	}
	switch ws := p.Sys().(type) {
	case syscall.WaitStatus:
		if ws.CoreDump() {
			t.Fatalf("interrupt: didn't expect child to dump core: %v", ws)
		}
		switch runtime.GOOS {
		case "darwin":
			// see comment above about EINTR on OS X
			if ws.Exited() {
				t.Fatalf("interrupt: expected child to die from signal, got exit status: %v", ws.ExitStatus())
			}
			if !ws.Signaled() {
				t.Fatalf("interrupt: expected child to die from signal: %v", ws)
			}
			if got := ws.Signal(); got != sig {
				t.Errorf("interrupt: child failed: signal %d", got)
			}
		default:
			if ws.Signaled() {
				t.Fatalf("interrupt: didn't expect child to exit with a signal: %v", ws)
			}
			if !ws.Exited() {
				t.Fatalf("interrupt: expected child to exit normally: %v", ws)
			}
			if status := ws.ExitStatus(); status != 0 {
				t.Errorf("interrupt: child failed: exit status %d", status)
			}
		}
	default:
		t.Logf("interrupt: this platform has no test coverage")
	}
}

// Test deadline

type deadline struct {
	fstestutil.File
}

var _ fs.NodeOpener = (*deadline)(nil)

func (it *deadline) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestDeadline(t *testing.T) {
	t.Parallel()
	child := &deadline{}
	config := &fs.Config{
		WithContext: func(ctx context.Context, req fuse.Request) context.Context {
			// return a context that has already deadlined

			// Server.serve will cancel the parent context, which will
			// cancel this one, so discarding cancel here should be
			// safe.
			ctx, _ = context.WithDeadline(ctx, time.Unix(0, 0))
			return ctx
		},
	}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": child}}, config)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	f, err := os.Open(mnt.Dir + "/child")
	if err == nil {
		f.Close()
	}

	// not caused by signal -> should not get EINTR;
	// context.DeadlineExceeded will be translated into EIO
	if nerr, ok := err.(*os.PathError); !ok || nerr.Err != syscall.EIO {
		t.Fatalf("wrong error from deadline open: %T: %v", err, err)
	}
}

// Test truncate

type truncate struct {
	fstestutil.File
	record.Setattrs
}

func testTruncate(t *testing.T, toSize int64) {
	t.Parallel()
	f := &truncate{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = os.Truncate(mnt.Dir+"/child", toSize)
	if err != nil {
		t.Fatalf("Truncate: %v", err)
	}
	gotr := f.RecordedSetattr()
	if gotr == (fuse.SetattrRequest{}) {
		t.Fatalf("no recorded SetattrRequest")
	}
	if g, e := gotr.Size, uint64(toSize); g != e {
		t.Errorf("got Size = %q; want %q", g, e)
	}
	if g, e := gotr.Valid&^fuse.SetattrLockOwner, fuse.SetattrSize; g != e {
		t.Errorf("got Valid = %q; want %q", g, e)
	}
	t.Logf("Got request: %#v", gotr)
}

func TestTruncate42(t *testing.T) {
	testTruncate(t, 42)
}

func TestTruncate0(t *testing.T) {
	testTruncate(t, 0)
}

// Test ftruncate

type ftruncate struct {
	fstestutil.File
	record.Setattrs
}

func testFtruncate(t *testing.T, toSize int64) {
	t.Parallel()
	f := &ftruncate{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	{
		fil, err := os.OpenFile(mnt.Dir+"/child", os.O_WRONLY, 0666)
		if err != nil {
			t.Error(err)
			return
		}
		defer fil.Close()

		err = fil.Truncate(toSize)
		if err != nil {
			t.Fatalf("Ftruncate: %v", err)
		}
	}
	gotr := f.RecordedSetattr()
	if gotr == (fuse.SetattrRequest{}) {
		t.Fatalf("no recorded SetattrRequest")
	}
	if g, e := gotr.Size, uint64(toSize); g != e {
		t.Errorf("got Size = %q; want %q", g, e)
	}
	if g, e := gotr.Valid&^fuse.SetattrLockOwner, fuse.SetattrHandle|fuse.SetattrSize; g != e {
		t.Errorf("got Valid = %q; want %q", g, e)
	}
	t.Logf("Got request: %#v", gotr)
}

func TestFtruncate42(t *testing.T) {
	testFtruncate(t, 42)
}

func TestFtruncate0(t *testing.T) {
	testFtruncate(t, 0)
}

// Test opening existing file truncates

type truncateWithOpen struct {
	fstestutil.File
	record.Setattrs
}

func TestTruncateWithOpen(t *testing.T) {
	t.Parallel()
	f := &truncateWithOpen{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	fil, err := os.OpenFile(mnt.Dir+"/child", os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		t.Error(err)
		return
	}
	fil.Close()

	gotr := f.RecordedSetattr()
	if gotr == (fuse.SetattrRequest{}) {
		t.Fatalf("no recorded SetattrRequest")
	}
	if g, e := gotr.Size, uint64(0); g != e {
		t.Errorf("got Size = %q; want %q", g, e)
	}
	// osxfuse sets SetattrHandle here, linux does not
	if g, e := gotr.Valid&^(fuse.SetattrLockOwner|fuse.SetattrHandle), fuse.SetattrSize; g != e {
		t.Errorf("got Valid = %q; want %q", g, e)
	}
	t.Logf("Got request: %#v", gotr)
}

// Test readdir calling ReadDirAll

type readDirAll struct {
	fstestutil.Dir
}

func (d *readDirAll) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	return []fuse.Dirent{
		{Name: "one", Inode: 11, Type: fuse.DT_Dir},
		{Name: "three", Inode: 13},
		{Name: "two", Inode: 12, Type: fuse.DT_File},
	}, nil
}

func TestReadDirAll(t *testing.T) {
	t.Parallel()
	f := &readDirAll{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	fil, err := os.Open(mnt.Dir)
	if err != nil {
		t.Error(err)
		return
	}
	defer fil.Close()

	// go Readdir is just Readdirnames + Lstat, there's no point in
	// testing that here; we have no consumption API for the real
	// dirent data
	names, err := fil.Readdirnames(100)
	if err != nil {
		t.Error(err)
		return
	}

	t.Logf("Got readdir: %q", names)

	if len(names) != 3 ||
		names[0] != "one" ||
		names[1] != "three" ||
		names[2] != "two" {
		t.Errorf(`expected 3 entries of "one", "three", "two", got: %q`, names)
		return
	}
}

type readDirAllBad struct {
	fstestutil.Dir
}

func (d *readDirAllBad) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	r := []fuse.Dirent{
		{Name: "one", Inode: 11, Type: fuse.DT_Dir},
		{Name: "three", Inode: 13},
		{Name: "two", Inode: 12, Type: fuse.DT_File},
	}
	// pick a really distinct error, to identify it later
	return r, fuse.Errno(syscall.ENAMETOOLONG)
}

func TestReadDirAllBad(t *testing.T) {
	t.Parallel()
	f := &readDirAllBad{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	fil, err := os.Open(mnt.Dir)
	if err != nil {
		t.Error(err)
		return
	}
	defer fil.Close()

	var names []string
	for {
		n, err := fil.Readdirnames(1)
		if err != nil {
			if nerr, ok := err.(*os.SyscallError); !ok || nerr.Err != syscall.ENAMETOOLONG {
				t.Fatalf("wrong error: %v", err)
			}
			break
		}
		names = append(names, n...)
	}

	t.Logf("Got readdir: %q", names)

	// TODO could serve partial results from ReadDirAll but the
	// shandle.readData mechanism makes that awkward.
	if len(names) != 0 {
		t.Errorf(`expected 0 entries, got: %q`, names)
		return
	}
}

// Test readdir without any ReadDir methods implemented.

type readDirNotImplemented struct {
	fstestutil.Dir
}

func TestReadDirNotImplemented(t *testing.T) {
	t.Parallel()
	f := &readDirNotImplemented{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	fil, err := os.Open(mnt.Dir)
	if err != nil {
		t.Error(err)
		return
	}
	defer fil.Close()

	// go Readdir is just Readdirnames + Lstat, there's no point in
	// testing that here; we have no consumption API for the real
	// dirent data
	names, err := fil.Readdirnames(100)
	if len(names) > 0 || err != io.EOF {
		t.Fatalf("expected EOF got names=%v err=%v", names, err)
	}
}

type readDirAllRewind struct {
	fstestutil.Dir
	entries atomic.Value
}

func (d *readDirAllRewind) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	entries := d.entries.Load().([]fuse.Dirent)
	return entries, nil
}

func TestReadDirAllRewind(t *testing.T) {
	t.Parallel()
	f := &readDirAllRewind{}
	f.entries.Store([]fuse.Dirent{
		{Name: "one", Inode: 11, Type: fuse.DT_Dir},
	})
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	fil, err := os.Open(mnt.Dir)
	if err != nil {
		t.Error(err)
		return
	}
	defer fil.Close()

	{
		names, err := fil.Readdirnames(100)
		if err != nil {
			t.Error(err)
			return
		}
		t.Logf("Got readdir: %q", names)
		if len(names) != 1 ||
			names[0] != "one" {
			t.Errorf(`expected  entry of "one", got: %q`, names)
			return
		}
	}

	f.entries.Store([]fuse.Dirent{
		{Name: "two", Inode: 12, Type: fuse.DT_File},
		{Name: "one", Inode: 11, Type: fuse.DT_Dir},
	})
	if _, err := fil.Seek(0, os.SEEK_SET); err != nil {
		t.Fatal(err)
	}

	{
		names, err := fil.Readdirnames(100)
		if err != nil {
			t.Error(err)
			return
		}
		t.Logf("Got readdir: %q", names)
		if len(names) != 2 ||
			names[0] != "two" ||
			names[1] != "one" {
			t.Errorf(`expected 2 entries of "two", "one", got: %q`, names)
			return
		}
	}
}

// Test Chmod.

type chmod struct {
	fstestutil.File
	record.Setattrs
}

func (f *chmod) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	if !req.Valid.Mode() {
		log.Printf("setattr not a chmod: %v", req.Valid)
		return fuse.EIO
	}
	f.Setattrs.Setattr(ctx, req, resp)
	return nil
}

func TestChmod(t *testing.T) {
	t.Parallel()
	f := &chmod{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = os.Chmod(mnt.Dir+"/child", 0764)
	if err != nil {
		t.Errorf("chmod: %v", err)
		return
	}
	got := f.RecordedSetattr()
	if g, e := got.Mode, os.FileMode(0764); g != e {
		t.Errorf("wrong mode: %v != %v", g, e)
	}
}

// Test open

type open struct {
	fstestutil.File
	record.Opens
}

func (f *open) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	f.Opens.Open(ctx, req, resp)
	// pick a really distinct error, to identify it later
	return nil, fuse.Errno(syscall.ENAMETOOLONG)
}

func TestOpen(t *testing.T) {
	t.Parallel()
	f := &open{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	// node: mode only matters with O_CREATE
	fil, err := os.OpenFile(mnt.Dir+"/child", os.O_WRONLY|os.O_APPEND, 0)
	if err == nil {
		t.Error("Open err == nil, expected ENAMETOOLONG")
		fil.Close()
		return
	}

	switch err2 := err.(type) {
	case *os.PathError:
		if err2.Err == syscall.ENAMETOOLONG {
			break
		}
		t.Errorf("unexpected inner error: %#v", err2)
	default:
		t.Errorf("unexpected error: %v", err)
	}

	want := fuse.OpenRequest{Dir: false, Flags: fuse.OpenWriteOnly | fuse.OpenAppend}
	if runtime.GOOS == "darwin" {
		// osxfuse does not let O_APPEND through at all
		//
		// https://code.google.com/p/macfuse/issues/detail?id=233
		// https://code.google.com/p/macfuse/issues/detail?id=132
		// https://code.google.com/p/macfuse/issues/detail?id=133
		want.Flags &^= fuse.OpenAppend
	}
	got := f.RecordedOpen()

	if runtime.GOOS == "linux" {
		// Linux <3.7 accidentally leaks O_CLOEXEC through to FUSE;
		// avoid spurious test failures
		got.Flags &^= fuse.OpenFlags(syscall.O_CLOEXEC)
	}

	if g, e := got, want; g != e {
		t.Errorf("open saw %v, want %v", g, e)
		return
	}
}

type openNonSeekable struct {
	fstestutil.File
}

func (f *openNonSeekable) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	resp.Flags |= fuse.OpenNonSeekable
	return f, nil
}

func TestOpenNonSeekable(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("OSXFUSE shares one read and one write handle for all clients, does not support open modes")
	}

	t.Parallel()
	f := &openNonSeekable{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	if !mnt.Conn.Protocol().HasOpenNonSeekable() {
		t.Skip("Old FUSE protocol")
	}

	fil, err := os.Open(mnt.Dir + "/child")
	if err != nil {
		t.Fatal(err)
	}
	defer fil.Close()

	_, err = fil.Seek(0, os.SEEK_SET)
	if nerr, ok := err.(*os.PathError); !ok || nerr.Err != syscall.ESPIPE {
		t.Fatalf("wrong error: %v", err)
	}
}

// Test Fsync on a dir

type fsyncDir struct {
	fstestutil.Dir
	record.Fsyncs
}

func TestFsyncDir(t *testing.T) {
	t.Parallel()
	f := &fsyncDir{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	fil, err := os.Open(mnt.Dir)
	if err != nil {
		t.Errorf("fsyncDir open: %v", err)
		return
	}
	defer fil.Close()
	err = fil.Sync()
	if err != nil {
		t.Errorf("fsyncDir sync: %v", err)
		return
	}

	got := f.RecordedFsync()
	want := fuse.FsyncRequest{
		Flags: 0,
		Dir:   true,
		// unpredictable
		Handle: got.Handle,
	}
	if runtime.GOOS == "darwin" {
		// TODO document the meaning of these flags, figure out why
		// they differ
		want.Flags = 1
	}
	if g, e := got, want; g != e {
		t.Fatalf("fsyncDir saw %+v, want %+v", g, e)
	}
}

// Test Getxattr

type getxattr struct {
	fstestutil.File
	record.Getxattrs
}

func (f *getxattr) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error {
	f.Getxattrs.Getxattr(ctx, req, resp)
	resp.Xattr = []byte("hello, world")
	return nil
}

func TestGetxattr(t *testing.T) {
	t.Parallel()
	f := &getxattr{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	buf := make([]byte, 8192)
	n, err := syscallx.Getxattr(mnt.Dir+"/child", "not-there", buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	buf = buf[:n]
	if g, e := string(buf), "hello, world"; g != e {
		t.Errorf("wrong getxattr content: %#v != %#v", g, e)
	}
	seen := f.RecordedGetxattr()
	if g, e := seen.Name, "not-there"; g != e {
		t.Errorf("wrong getxattr name: %#v != %#v", g, e)
	}
}

// Test Getxattr that has no space to return value

type getxattrTooSmall struct {
	fstestutil.File
}

func (f *getxattrTooSmall) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error {
	resp.Xattr = []byte("hello, world")
	return nil
}

func TestGetxattrTooSmall(t *testing.T) {
	t.Parallel()
	f := &getxattrTooSmall{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	buf := make([]byte, 3)
	_, err = syscallx.Getxattr(mnt.Dir+"/child", "whatever", buf)
	if err == nil {
		t.Error("Getxattr = nil; want some error")
	}
	if err != syscall.ERANGE {
		t.Errorf("unexpected error: %v", err)
		return
	}
}

// Test Getxattr used to probe result size

type getxattrSize struct {
	fstestutil.File
}

func (f *getxattrSize) Getxattr(ctx context.Context, req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse) error {
	resp.Xattr = []byte("hello, world")
	return nil
}

func TestGetxattrSize(t *testing.T) {
	t.Parallel()
	f := &getxattrSize{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	n, err := syscallx.Getxattr(mnt.Dir+"/child", "whatever", nil)
	if err != nil {
		t.Errorf("Getxattr unexpected error: %v", err)
		return
	}
	if g, e := n, len("hello, world"); g != e {
		t.Errorf("Getxattr incorrect size: %d != %d", g, e)
	}
}

// Test Listxattr

type listxattr struct {
	fstestutil.File
	record.Listxattrs
}

func (f *listxattr) Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) error {
	f.Listxattrs.Listxattr(ctx, req, resp)
	resp.Append("one", "two")
	return nil
}

func TestListxattr(t *testing.T) {
	t.Parallel()
	f := &listxattr{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	buf := make([]byte, 8192)
	n, err := syscallx.Listxattr(mnt.Dir+"/child", buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	buf = buf[:n]
	if g, e := string(buf), "one\x00two\x00"; g != e {
		t.Errorf("wrong listxattr content: %#v != %#v", g, e)
	}

	want := fuse.ListxattrRequest{
		Size: 8192,
	}
	if g, e := f.RecordedListxattr(), want; g != e {
		t.Fatalf("listxattr saw %+v, want %+v", g, e)
	}
}

// Test Listxattr that has no space to return value

type listxattrTooSmall struct {
	fstestutil.File
}

func (f *listxattrTooSmall) Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) error {
	resp.Xattr = []byte("one\x00two\x00")
	return nil
}

func TestListxattrTooSmall(t *testing.T) {
	t.Parallel()
	f := &listxattrTooSmall{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	buf := make([]byte, 3)
	_, err = syscallx.Listxattr(mnt.Dir+"/child", buf)
	if err == nil {
		t.Error("Listxattr = nil; want some error")
	}
	if err != syscall.ERANGE {
		t.Errorf("unexpected error: %v", err)
		return
	}
}

// Test Listxattr used to probe result size

type listxattrSize struct {
	fstestutil.File
}

func (f *listxattrSize) Listxattr(ctx context.Context, req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse) error {
	resp.Xattr = []byte("one\x00two\x00")
	return nil
}

func TestListxattrSize(t *testing.T) {
	t.Parallel()
	f := &listxattrSize{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	n, err := syscallx.Listxattr(mnt.Dir+"/child", nil)
	if err != nil {
		t.Errorf("Listxattr unexpected error: %v", err)
		return
	}
	if g, e := n, len("one\x00two\x00"); g != e {
		t.Errorf("Getxattr incorrect size: %d != %d", g, e)
	}
}

// Test Setxattr

type setxattr struct {
	fstestutil.File
	record.Setxattrs
}

func testSetxattr(t *testing.T, size int) {
	const linux_XATTR_NAME_MAX = 64 * 1024
	if size > linux_XATTR_NAME_MAX && runtime.GOOS == "linux" {
		t.Skip("large xattrs are not supported by linux")
	}

	t.Parallel()
	f := &setxattr{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	const g = "hello, world"
	greeting := strings.Repeat(g, size/len(g)+1)[:size]
	err = syscallx.Setxattr(mnt.Dir+"/child", "greeting", []byte(greeting), 0)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	// fuse.SetxattrRequest contains a byte slice and thus cannot be
	// directly compared
	got := f.RecordedSetxattr()

	if g, e := got.Name, "greeting"; g != e {
		t.Errorf("Setxattr incorrect name: %q != %q", g, e)
	}

	if g, e := got.Flags, uint32(0); g != e {
		t.Errorf("Setxattr incorrect flags: %d != %d", g, e)
	}

	if g, e := string(got.Xattr), greeting; g != e {
		t.Errorf("Setxattr incorrect data: %q != %q", g, e)
	}
}

func TestSetxattr(t *testing.T) {
	testSetxattr(t, 20)
}

func TestSetxattr64kB(t *testing.T) {
	testSetxattr(t, 64*1024)
}

func TestSetxattr16MB(t *testing.T) {
	testSetxattr(t, 16*1024*1024)
}

// Test Removexattr

type removexattr struct {
	fstestutil.File
	record.Removexattrs
}

func TestRemovexattr(t *testing.T) {
	t.Parallel()
	f := &removexattr{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": f}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = syscallx.Removexattr(mnt.Dir+"/child", "greeting")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	want := fuse.RemovexattrRequest{Name: "greeting"}
	if g, e := f.RecordedRemovexattr(), want; g != e {
		t.Errorf("removexattr saw %v, want %v", g, e)
	}
}

// Test default error.

type defaultErrno struct {
	fstestutil.Dir
}

func (f defaultErrno) Lookup(ctx context.Context, name string) (fs.Node, error) {
	return nil, errors.New("bork")
}

func TestDefaultErrno(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{defaultErrno{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	_, err = os.Stat(mnt.Dir + "/trigger")
	if err == nil {
		t.Fatalf("expected error")
	}

	switch err2 := err.(type) {
	case *os.PathError:
		if err2.Err == syscall.EIO {
			break
		}
		t.Errorf("unexpected inner error: Err=%v %#v", err2.Err, err2)
	default:
		t.Errorf("unexpected error: %v", err)
	}
}

// Test custom error.

type customErrNode struct {
	fstestutil.Dir
}

type myCustomError struct {
	fuse.ErrorNumber
}

var _ = fuse.ErrorNumber(myCustomError{})

func (myCustomError) Error() string {
	return "bork"
}

func (f customErrNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	return nil, myCustomError{
		ErrorNumber: fuse.Errno(syscall.ENAMETOOLONG),
	}
}

func TestCustomErrno(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{customErrNode{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	_, err = os.Stat(mnt.Dir + "/trigger")
	if err == nil {
		t.Fatalf("expected error")
	}

	switch err2 := err.(type) {
	case *os.PathError:
		if err2.Err == syscall.ENAMETOOLONG {
			break
		}
		t.Errorf("unexpected inner error: %#v", err2)
	default:
		t.Errorf("unexpected error: %v", err)
	}
}

// Test Mmap writing

type inMemoryFile struct {
	mu   sync.Mutex
	data []byte
}

func (f *inMemoryFile) bytes() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.data
}

func (f *inMemoryFile) Attr(ctx context.Context, a *fuse.Attr) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	a.Mode = 0666
	a.Size = uint64(len(f.data))
	return nil
}

func (f *inMemoryFile) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	fuseutil.HandleRead(req, resp, f.data)
	return nil
}

func (f *inMemoryFile) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	resp.Size = copy(f.data[req.Offset:], req.Data)
	return nil
}

const mmapSize = 16 * 4096

var mmapWrites = map[int]byte{
	10:              'a',
	4096:            'b',
	4097:            'c',
	mmapSize - 4096: 'd',
	mmapSize - 1:    'z',
}

func helperMmap() {
	f, err := os.Create("child")
	if err != nil {
		log.Fatalf("Create: %v", err)
	}
	defer f.Close()

	data, err := syscall.Mmap(int(f.Fd()), 0, mmapSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		log.Fatalf("Mmap: %v", err)
	}

	for i, b := range mmapWrites {
		data[i] = b
	}

	if err := syscallx.Msync(data, syscall.MS_SYNC); err != nil {
		log.Fatalf("Msync: %v", err)
	}

	if err := syscall.Munmap(data); err != nil {
		log.Fatalf("Munmap: %v", err)
	}

	if err := f.Sync(); err != nil {
		log.Fatalf("Fsync = %v", err)
	}

	err = f.Close()
	if err != nil {
		log.Fatalf("Close: %v", err)
	}
}

func init() {
	childHelpers["mmap"] = helperMmap
}

type mmap struct {
	inMemoryFile
	// We don't actually care about whether the fsync happened or not;
	// this just lets us force the page cache to send the writes to
	// FUSE, so we can reliably verify they came through.
	record.Fsyncs
}

func TestMmap(t *testing.T) {

	w := &mmap{}
	w.data = make([]byte, mmapSize)
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": w}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	// Run the mmap-using parts of the test in a subprocess, to avoid
	// an intentional page fault hanging the whole process (because it
	// would need to be served by the same process, and there might
	// not be a thread free to do that). Merely bumping GOMAXPROCS is
	// not enough to prevent the hangs reliably.
	child, err := childCmd("mmap")
	if err != nil {
		t.Fatal(err)
	}
	child.Dir = mnt.Dir
	if err := child.Run(); err != nil {
		t.Fatal(err)
	}

	got := w.bytes()
	if g, e := len(got), mmapSize; g != e {
		t.Fatalf("bad write length: %d != %d", g, e)
	}
	for i, g := range got {
		// default '\x00' for writes[i] is good here
		if e := mmapWrites[i]; g != e {
			t.Errorf("wrong byte at offset %d: %q != %q", i, g, e)
		}
	}
}

// Test direct Read.

type directRead struct {
	fstestutil.File
}

// explicitly not defining Attr and setting Size

func (f directRead) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	// do not allow the kernel to use page cache
	resp.Flags |= fuse.OpenDirectIO
	return f, nil
}

func (directRead) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	fuseutil.HandleRead(req, resp, []byte(hi))
	return nil
}

func TestDirectRead(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": directRead{}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	testReadAll(t, mnt.Dir+"/child")
}

// Test direct Write.

type directWrite struct {
	fstestutil.File
	record.Writes
}

// explicitly not defining Attr / Setattr and managing Size

func (f *directWrite) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	// do not allow the kernel to use page cache
	resp.Flags |= fuse.OpenDirectIO
	return f, nil
}

func TestDirectWrite(t *testing.T) {
	t.Parallel()
	w := &directWrite{}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": w}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	f, err := os.OpenFile(mnt.Dir+"/child", os.O_RDWR, 0666)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer f.Close()
	n, err := f.Write([]byte(hi))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(hi) {
		t.Fatalf("short write; n=%d; hi=%d", n, len(hi))
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := string(w.RecordedWriteData()); got != hi {
		t.Errorf("write = %q, want %q", got, hi)
	}
}

// Test Attr

// attrUnlinked is a file that is unlinked (Nlink==0).
type attrUnlinked struct {
	fstestutil.File
}

var _ fs.Node = attrUnlinked{}

func (f attrUnlinked) Attr(ctx context.Context, a *fuse.Attr) error {
	if err := f.File.Attr(ctx, a); err != nil {
		return err
	}
	a.Nlink = 0
	return nil
}

func TestAttrUnlinked(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": attrUnlinked{}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	fi, err := os.Stat(mnt.Dir + "/child")
	if err != nil {
		t.Fatalf("Stat failed with %v", err)
	}
	switch stat := fi.Sys().(type) {
	case *syscall.Stat_t:
		if stat.Nlink != 0 {
			t.Errorf("wrong link count: %v", stat.Nlink)
		}
	}
}

// Test behavior when Attr method fails

type attrBad struct {
}

var _ fs.Node = attrBad{}

func (attrBad) Attr(ctx context.Context, attr *fuse.Attr) error {
	return fuse.Errno(syscall.ENAMETOOLONG)
}

func TestAttrBad(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": attrBad{}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	_, err = os.Stat(mnt.Dir + "/child")
	if nerr, ok := err.(*os.PathError); !ok || nerr.Err != syscall.ENAMETOOLONG {
		t.Fatalf("wrong error: %v", err)
	}
}

// Test kernel cache invalidation

type invalidateAttr struct {
	fs.NodeRef
	t    testing.TB
	attr record.Counter
}

var _ fs.Node = (*invalidateAttr)(nil)

func (i *invalidateAttr) Attr(ctx context.Context, a *fuse.Attr) error {
	i.attr.Inc()
	i.t.Logf("Attr called, #%d", i.attr.Count())
	a.Mode = 0600
	return nil
}

func TestInvalidateNodeAttr(t *testing.T) {
	// This test may see false positive failures when run under
	// extreme memory pressure.
	t.Parallel()
	a := &invalidateAttr{
		t: t,
	}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": a}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	if !mnt.Conn.Protocol().HasInvalidate() {
		t.Skip("Old FUSE protocol")
	}

	for i := 0; i < 10; i++ {
		if _, err := os.Stat(mnt.Dir + "/child"); err != nil {
			t.Fatalf("stat error: %v", err)
		}
	}
	// With OSXFUSE 3.0.4, we seem to see typically two Attr calls by
	// this point; something not populating the in-kernel cache
	// properly? Cope with it; we care more about seeing a new Attr
	// call after the invalidation.
	//
	// We still enforce a max number here so that we know that the
	// invalidate actually did something, and it's not just that every
	// Stat results in an Attr.
	before := a.attr.Count()
	if before == 0 {
		t.Error("no Attr call seen")
	}
	if g, e := before, uint32(3); g > e {
		t.Errorf("too many Attr calls seen: %d > %d", g, e)
	}

	t.Logf("invalidating...")
	if err := mnt.Server.InvalidateNodeAttr(a); err != nil {
		t.Fatalf("invalidate error: %v", err)
	}

	for i := 0; i < 10; i++ {
		if _, err := os.Stat(mnt.Dir + "/child"); err != nil {
			t.Fatalf("stat error: %v", err)
		}
	}
	if g, e := a.attr.Count(), before+1; g != e {
		t.Errorf("wrong Attr call count: %d != %d", g, e)
	}
}

type invalidateData struct {
	fs.NodeRef
	t    testing.TB
	attr record.Counter
	read record.Counter
	data atomic.Value
}

const (
	invalidateDataContent1 = "hello, world\n"
	invalidateDataContent2 = "so long!\n"
)

var _ fs.Node = (*invalidateData)(nil)

func (i *invalidateData) Attr(ctx context.Context, a *fuse.Attr) error {
	i.attr.Inc()
	i.t.Logf("Attr called, #%d", i.attr.Count())
	a.Mode = 0600
	a.Size = uint64(len(i.data.Load().(string)))
	return nil
}

var _ fs.HandleReader = (*invalidateData)(nil)

func (i *invalidateData) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	i.read.Inc()
	i.t.Logf("Read called, #%d", i.read.Count())
	fuseutil.HandleRead(req, resp, []byte(i.data.Load().(string)))
	return nil
}

func TestInvalidateNodeDataInvalidatesAttr(t *testing.T) {
	// This test may see false positive failures when run under
	// extreme memory pressure.
	t.Parallel()
	a := &invalidateData{
		t: t,
	}
	a.data.Store(invalidateDataContent1)
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": a}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	if !mnt.Conn.Protocol().HasInvalidate() {
		t.Skip("Old FUSE protocol")
	}

	f, err := os.Open(mnt.Dir + "/child")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	attrBefore := a.attr.Count()
	if g, min := attrBefore, uint32(1); g < min {
		t.Errorf("wrong Attr call count: %d < %d", g, min)
	}

	t.Logf("invalidating...")
	a.data.Store(invalidateDataContent2)
	if err := mnt.Server.InvalidateNodeData(a); err != nil {
		t.Fatalf("invalidate error: %v", err)
	}

	// on OSXFUSE 3.0.6, the Attr has already triggered here, so don't
	// check the count at this point

	if _, err := f.Stat(); err != nil {
		t.Errorf("stat error: %v", err)
	}
	if g, prev := a.attr.Count(), attrBefore; g <= prev {
		t.Errorf("did not see Attr call after invalidate: %d <= %d", g, prev)
	}
}

func TestInvalidateNodeDataInvalidatesData(t *testing.T) {
	// This test may see false positive failures when run under
	// extreme memory pressure.
	t.Parallel()
	a := &invalidateData{
		t: t,
	}
	a.data.Store(invalidateDataContent1)
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": a}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	if !mnt.Conn.Protocol().HasInvalidate() {
		t.Skip("Old FUSE protocol")
	}

	f, err := os.Open(mnt.Dir + "/child")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	{
		buf := make([]byte, 100)
		for i := 0; i < 10; i++ {
			n, err := f.ReadAt(buf, 0)
			if err != nil && err != io.EOF {
				t.Fatalf("readat error: %v", err)
			}
			if g, e := string(buf[:n]), invalidateDataContent1; g != e {
				t.Errorf("wrong content: %q != %q", g, e)
			}
		}
	}
	if g, e := a.read.Count(), uint32(1); g != e {
		t.Errorf("wrong Read call count: %d != %d", g, e)
	}

	t.Logf("invalidating...")
	a.data.Store(invalidateDataContent2)
	if err := mnt.Server.InvalidateNodeData(a); err != nil {
		t.Fatalf("invalidate error: %v", err)
	}

	if g, e := a.read.Count(), uint32(1); g != e {
		t.Errorf("wrong Read call count: %d != %d", g, e)
	}

	{
		// explicitly don't cross the EOF, to trigger more edge cases
		// (Linux will always do Getattr if you cross what it believes
		// the EOF to be)
		const bufSize = len(invalidateDataContent2) - 3
		buf := make([]byte, bufSize)
		for i := 0; i < 10; i++ {
			n, err := f.ReadAt(buf, 0)
			if err != nil && err != io.EOF {
				t.Fatalf("readat error: %v", err)
			}
			if g, e := string(buf[:n]), invalidateDataContent2[:bufSize]; g != e {
				t.Errorf("wrong content: %q != %q", g, e)
			}
		}
	}
	if g, e := a.read.Count(), uint32(2); g != e {
		t.Errorf("wrong Read call count: %d != %d", g, e)
	}
}

type invalidateDataPartial struct {
	fs.NodeRef
	t    testing.TB
	attr record.Counter
	read record.Counter
}

var invalidateDataPartialContent = strings.Repeat("hello, world\n", 1000)

var _ fs.Node = (*invalidateDataPartial)(nil)

func (i *invalidateDataPartial) Attr(ctx context.Context, a *fuse.Attr) error {
	i.attr.Inc()
	i.t.Logf("Attr called, #%d", i.attr.Count())
	a.Mode = 0600
	a.Size = uint64(len(invalidateDataPartialContent))
	return nil
}

var _ fs.HandleReader = (*invalidateDataPartial)(nil)

func (i *invalidateDataPartial) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	i.read.Inc()
	i.t.Logf("Read called, #%d", i.read.Count())
	fuseutil.HandleRead(req, resp, []byte(invalidateDataPartialContent))
	return nil
}

func TestInvalidateNodeDataRangeMiss(t *testing.T) {
	// This test may see false positive failures when run under
	// extreme memory pressure.
	t.Parallel()
	a := &invalidateDataPartial{
		t: t,
	}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": a}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	if !mnt.Conn.Protocol().HasInvalidate() {
		t.Skip("Old FUSE protocol")
	}

	f, err := os.Open(mnt.Dir + "/child")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	buf := make([]byte, 4)
	for i := 0; i < 10; i++ {
		if _, err := f.ReadAt(buf, 0); err != nil {
			t.Fatalf("readat error: %v", err)
		}
	}
	if g, e := a.read.Count(), uint32(1); g != e {
		t.Errorf("wrong Read call count: %d != %d", g, e)
	}

	t.Logf("invalidating an uninteresting block...")
	if err := mnt.Server.InvalidateNodeDataRange(a, 4096, 4096); err != nil {
		t.Fatalf("invalidate error: %v", err)
	}

	for i := 0; i < 10; i++ {
		if _, err := f.ReadAt(buf, 0); err != nil {
			t.Fatalf("readat error: %v", err)
		}
	}
	// The page invalidated is not the page we're reading, so it
	// should stay in cache.
	if g, e := a.read.Count(), uint32(1); g != e {
		t.Errorf("wrong Read call count: %d != %d", g, e)
	}
}

func TestInvalidateNodeDataRangeHit(t *testing.T) {
	// This test may see false positive failures when run under
	// extreme memory pressure.
	t.Parallel()
	a := &invalidateDataPartial{
		t: t,
	}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{&fstestutil.ChildMap{"child": a}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	if !mnt.Conn.Protocol().HasInvalidate() {
		t.Skip("Old FUSE protocol")
	}

	f, err := os.Open(mnt.Dir + "/child")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	const offset = 4096
	buf := make([]byte, 4)
	for i := 0; i < 10; i++ {
		if _, err := f.ReadAt(buf, offset); err != nil {
			t.Fatalf("readat error: %v", err)
		}
	}
	if g, e := a.read.Count(), uint32(1); g != e {
		t.Errorf("wrong Read call count: %d != %d", g, e)
	}

	t.Logf("invalidating where the reads are...")
	if err := mnt.Server.InvalidateNodeDataRange(a, offset, 4096); err != nil {
		t.Fatalf("invalidate error: %v", err)
	}

	for i := 0; i < 10; i++ {
		if _, err := f.ReadAt(buf, offset); err != nil {
			t.Fatalf("readat error: %v", err)
		}
	}
	// One new read
	if g, e := a.read.Count(), uint32(2); g != e {
		t.Errorf("wrong Read call count: %d != %d", g, e)
	}
}

type invalidateEntryRoot struct {
	fs.NodeRef
	t      testing.TB
	lookup record.Counter
}

var _ fs.Node = (*invalidateEntryRoot)(nil)

func (i *invalidateEntryRoot) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = 0600 | os.ModeDir
	return nil
}

var _ fs.NodeStringLookuper = (*invalidateEntryRoot)(nil)

func (i *invalidateEntryRoot) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if name != "child" {
		return nil, fuse.ENOENT
	}
	i.lookup.Inc()
	i.t.Logf("Lookup called, #%d", i.lookup.Count())
	return fstestutil.File{}, nil
}

func TestInvalidateEntry(t *testing.T) {
	// This test may see false positive failures when run under
	// extreme memory pressure.
	t.Parallel()
	a := &invalidateEntryRoot{
		t: t,
	}
	mnt, err := fstestutil.MountedT(t, fstestutil.SimpleFS{a}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	if !mnt.Conn.Protocol().HasInvalidate() {
		t.Skip("Old FUSE protocol")
	}

	for i := 0; i < 10; i++ {
		if _, err := os.Stat(mnt.Dir + "/child"); err != nil {
			t.Fatalf("stat error: %v", err)
		}
	}
	if g, e := a.lookup.Count(), uint32(1); g != e {
		t.Errorf("wrong Lookup call count: %d != %d", g, e)
	}

	t.Logf("invalidating...")
	if err := mnt.Server.InvalidateEntry(a, "child"); err != nil {
		t.Fatalf("invalidate error: %v", err)
	}

	for i := 0; i < 10; i++ {
		if _, err := os.Stat(mnt.Dir + "/child"); err != nil {
			t.Fatalf("stat error: %v", err)
		}
	}
	if g, e := a.lookup.Count(), uint32(2); g != e {
		t.Errorf("wrong Lookup call count: %d != %d", g, e)
	}
}

type contextFile struct {
	fstestutil.File
}

var contextFileSentinel int

func (contextFile) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	v := ctx.Value(&contextFileSentinel)
	if v == nil {
		return nil, fuse.ESTALE
	}
	data, ok := v.(string)
	if !ok {
		return nil, fuse.EIO
	}
	resp.Flags |= fuse.OpenDirectIO
	return fs.DataHandle([]byte(data)), nil
}

func TestContext(t *testing.T) {
	t.Parallel()
	const input = "kilroy was here"
	mnt, err := fstestutil.MountedT(t,
		fstestutil.SimpleFS{&fstestutil.ChildMap{"child": contextFile{}}},
		&fs.Config{
			WithContext: func(ctx context.Context, req fuse.Request) context.Context {
				return context.WithValue(ctx, &contextFileSentinel, input)
			},
		})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	data, err := ioutil.ReadFile(mnt.Dir + "/child")
	if err != nil {
		t.Fatalf("cannot read context file: %v", err)
	}
	if g, e := string(data), input; g != e {
		t.Errorf("read wrong data: %q != %q", g, e)
	}
}

type goexitFile struct {
	fstestutil.File
}

func (goexitFile) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	log.Println("calling runtime.Goexit...")
	runtime.Goexit()
	panic("not reached")
}

func TestGoexit(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t,
		fstestutil.SimpleFS{&fstestutil.ChildMap{"child": goexitFile{}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	_, err = ioutil.ReadFile(mnt.Dir + "/child")
	if nerr, ok := err.(*os.PathError); !ok || nerr.Err != syscall.EIO {
		t.Fatalf("wrong error from exiting handler: %T: %v", err, err)
	}
}
