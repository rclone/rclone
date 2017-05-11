package bench_test

import (
	"os"
	"testing"

	"golang.org/x/net/context"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fs/fstestutil"
)

type benchLookupDir struct {
	fstestutil.Dir
}

var _ fs.NodeRequestLookuper = (*benchLookupDir)(nil)

func (f *benchLookupDir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	return nil, fuse.ENOENT
}

func BenchmarkLookup(b *testing.B) {
	f := &benchLookupDir{}
	mnt, err := fstestutil.MountedT(b, fstestutil.SimpleFS{f}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer mnt.Close()

	name := mnt.Dir + "/does-not-exist"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			b.Fatalf("Stat: wrong error: %v", err)
		}
	}

	b.StopTimer()
}
