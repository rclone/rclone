// Clockfs implements a file system with the current time in a file.
// It was written to demonstrate kernel cache invalidation.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	_ "bazil.org/fuse/fs/fstestutil"
	"bazil.org/fuse/fuseutil"
	"golang.org/x/net/context"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s MOUNTPOINT\n", os.Args[0])
	flag.PrintDefaults()
}

func run(mountpoint string) error {
	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName("clock"),
		fuse.Subtype("clockfsfs"),
		fuse.LocalVolume(),
		fuse.VolumeName("Clock filesystem"),
	)
	if err != nil {
		return err
	}
	defer c.Close()

	if p := c.Protocol(); !p.HasInvalidate() {
		return fmt.Errorf("kernel FUSE support is too old to have invalidations: version %v", p)
	}

	srv := fs.New(c, nil)
	filesys := &FS{
		// We pre-create the clock node so that it's always the same
		// object returned from all the Lookups. You could carefully
		// track its lifetime between Lookup&Forget, and have the
		// ticking & invalidation happen only when active, but let's
		// keep this example simple.
		clockFile: &File{
			fuse: srv,
		},
	}
	filesys.clockFile.tick()
	// This goroutine never exits. That's fine for this example.
	go filesys.clockFile.update()
	if err := srv.Serve(filesys); err != nil {
		return err
	}

	// Check if the mount process has an error to report.
	<-c.Ready
	if err := c.MountError; err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	mountpoint := flag.Arg(0)

	if err := run(mountpoint); err != nil {
		log.Fatal(err)
	}
}

type FS struct {
	clockFile *File
}

var _ fs.FS = (*FS)(nil)

func (f *FS) Root() (fs.Node, error) {
	return &Dir{fs: f}, nil
}

// Dir implements both Node and Handle for the root directory.
type Dir struct {
	fs *FS
}

var _ fs.Node = (*Dir)(nil)

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 1
	a.Mode = os.ModeDir | 0555
	return nil
}

var _ fs.NodeStringLookuper = (*Dir)(nil)

func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if name == "clock" {
		return d.fs.clockFile, nil
	}
	return nil, fuse.ENOENT
}

var dirDirs = []fuse.Dirent{
	{Inode: 2, Name: "clock", Type: fuse.DT_File},
}

var _ fs.HandleReadDirAller = (*Dir)(nil)

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	return dirDirs, nil
}

type File struct {
	fs.NodeRef
	fuse    *fs.Server
	content atomic.Value
	count   uint64
}

var _ fs.Node = (*File)(nil)

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 2
	a.Mode = 0444
	t := f.content.Load().(string)
	a.Size = uint64(len(t))
	return nil
}

var _ fs.NodeOpener = (*File)(nil)

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	if !req.Flags.IsReadOnly() {
		return nil, fuse.Errno(syscall.EACCES)
	}
	resp.Flags |= fuse.OpenKeepCache
	return f, nil
}

var _ fs.Handle = (*File)(nil)

var _ fs.HandleReader = (*File)(nil)

func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	t := f.content.Load().(string)
	fuseutil.HandleRead(req, resp, []byte(t))
	return nil
}

func (f *File) tick() {
	// Intentionally a variable-length format, to demonstrate size changes.
	f.count++
	s := fmt.Sprintf("%d\t%s\n", f.count, time.Now())
	f.content.Store(s)

	// For simplicity, this example tries to send invalidate
	// notifications even when the kernel does not hold a reference to
	// the node, so be extra sure to ignore ErrNotCached.
	if err := f.fuse.InvalidateNodeData(f); err != nil && err != fuse.ErrNotCached {
		log.Printf("invalidate error: %v", err)
	}
}

func (f *File) update() {
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()
	for range tick.C {
		f.tick()
	}
}
