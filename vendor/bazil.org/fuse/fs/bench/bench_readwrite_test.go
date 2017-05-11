package bench_test

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fs/fstestutil"
	"golang.org/x/net/context"
)

type benchConfig struct {
	directIO bool
}

type benchFS struct {
	conf *benchConfig
}

var _ = fs.FS(benchFS{})

func (f benchFS) Root() (fs.Node, error) {
	return benchDir{conf: f.conf}, nil
}

type benchDir struct {
	conf *benchConfig
}

var _ = fs.Node(benchDir{})
var _ = fs.NodeStringLookuper(benchDir{})
var _ = fs.Handle(benchDir{})
var _ = fs.HandleReadDirAller(benchDir{})

func (benchDir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 1
	a.Mode = os.ModeDir | 0555
	return nil
}

func (d benchDir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if name == "bench" {
		return benchFile{conf: d.conf}, nil
	}
	return nil, fuse.ENOENT
}

func (benchDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	l := []fuse.Dirent{
		{Inode: 2, Name: "bench", Type: fuse.DT_File},
	}
	return l, nil
}

type benchFile struct {
	conf *benchConfig
}

var _ = fs.Node(benchFile{})
var _ = fs.NodeOpener(benchFile{})
var _ = fs.NodeFsyncer(benchFile{})
var _ = fs.Handle(benchFile{})
var _ = fs.HandleReader(benchFile{})
var _ = fs.HandleWriter(benchFile{})

func (benchFile) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 2
	a.Mode = 0644
	a.Size = 9999999999999999
	return nil
}

func (f benchFile) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	if f.conf.directIO {
		resp.Flags |= fuse.OpenDirectIO
	}
	// TODO configurable?
	resp.Flags |= fuse.OpenKeepCache
	return f, nil
}

func (benchFile) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	resp.Data = resp.Data[:cap(resp.Data)]
	return nil
}

func (benchFile) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	resp.Size = len(req.Data)
	return nil
}

func (benchFile) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}

func benchmark(b *testing.B, fn func(b *testing.B, mnt string), conf *benchConfig) {
	filesys := benchFS{
		conf: conf,
	}
	mnt, err := fstestutil.Mounted(filesys, nil,
		fuse.MaxReadahead(64*1024*1024),
		fuse.AsyncRead(),
		fuse.WritebackCache(),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer mnt.Close()

	fn(b, mnt.Dir)
}

type zero struct{}

func (zero) Read(p []byte) (n int, err error) {
	return len(p), nil
}

var Zero io.Reader = zero{}

func doWrites(size int64) func(b *testing.B, mnt string) {
	return func(b *testing.B, mnt string) {
		p := path.Join(mnt, "bench")

		f, err := os.Create(p)
		if err != nil {
			b.Fatalf("create: %v", err)
		}
		defer f.Close()

		b.ResetTimer()
		b.SetBytes(size)

		for i := 0; i < b.N; i++ {
			_, err = io.CopyN(f, Zero, size)
			if err != nil {
				b.Fatalf("write: %v", err)
			}
		}
	}
}

func BenchmarkWrite100(b *testing.B) {
	benchmark(b, doWrites(100), &benchConfig{})
}

func BenchmarkWrite10MB(b *testing.B) {
	benchmark(b, doWrites(10*1024*1024), &benchConfig{})
}

func BenchmarkWrite100MB(b *testing.B) {
	benchmark(b, doWrites(100*1024*1024), &benchConfig{})
}

func BenchmarkDirectWrite100(b *testing.B) {
	benchmark(b, doWrites(100), &benchConfig{
		directIO: true,
	})
}

func BenchmarkDirectWrite10MB(b *testing.B) {
	benchmark(b, doWrites(10*1024*1024), &benchConfig{
		directIO: true,
	})
}

func BenchmarkDirectWrite100MB(b *testing.B) {
	benchmark(b, doWrites(100*1024*1024), &benchConfig{
		directIO: true,
	})
}

func doWritesSync(size int64) func(b *testing.B, mnt string) {
	return func(b *testing.B, mnt string) {
		p := path.Join(mnt, "bench")

		f, err := os.Create(p)
		if err != nil {
			b.Fatalf("create: %v", err)
		}
		defer f.Close()

		b.ResetTimer()
		b.SetBytes(size)

		for i := 0; i < b.N; i++ {
			_, err = io.CopyN(f, Zero, size)
			if err != nil {
				b.Fatalf("write: %v", err)
			}

			if err := f.Sync(); err != nil {
				b.Fatalf("sync: %v", err)
			}
		}
	}
}

func BenchmarkWriteSync100(b *testing.B) {
	benchmark(b, doWritesSync(100), &benchConfig{})
}

func BenchmarkWriteSync10MB(b *testing.B) {
	benchmark(b, doWritesSync(10*1024*1024), &benchConfig{})
}

func BenchmarkWriteSync100MB(b *testing.B) {
	benchmark(b, doWritesSync(100*1024*1024), &benchConfig{})
}

func doReads(size int64) func(b *testing.B, mnt string) {
	return func(b *testing.B, mnt string) {
		p := path.Join(mnt, "bench")

		f, err := os.Open(p)
		if err != nil {
			b.Fatalf("close: %v", err)
		}
		defer f.Close()

		b.ResetTimer()
		b.SetBytes(size)

		for i := 0; i < b.N; i++ {
			n, err := io.CopyN(ioutil.Discard, f, size)
			if err != nil {
				b.Fatalf("read: %v", err)
			}
			if n != size {
				b.Errorf("unexpected size: %d != %d", n, size)
			}
		}
	}
}

func BenchmarkRead100(b *testing.B) {
	benchmark(b, doReads(100), &benchConfig{})
}

func BenchmarkRead10MB(b *testing.B) {
	benchmark(b, doReads(10*1024*1024), &benchConfig{})
}

func BenchmarkRead100MB(b *testing.B) {
	benchmark(b, doReads(100*1024*1024), &benchConfig{})
}

func BenchmarkDirectRead100(b *testing.B) {
	benchmark(b, doReads(100), &benchConfig{
		directIO: true,
	})
}

func BenchmarkDirectRead10MB(b *testing.B) {
	benchmark(b, doReads(10*1024*1024), &benchConfig{
		directIO: true,
	})
}

func BenchmarkDirectRead100MB(b *testing.B) {
	benchmark(b, doReads(100*1024*1024), &benchConfig{
		directIO: true,
	})
}
