package filelu

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SetUploadChunkSize changes the configured chunk size, returning the old value.
//
// It is only called by the integration tests while no transfer is in progress.
func (f *Fs) SetUploadChunkSize(cs fs.SizeSuffix) (fs.SizeSuffix, error) {
	var old fs.SizeSuffix
	old, f.opt.ChunkSize = f.opt.ChunkSize, cs
	return old, nil
}

// SetUploadCutoff changes the configured upload cutoff, returning the old value.
//
// It is only called by the integration tests while no transfer is in progress.
func (f *Fs) SetUploadCutoff(cutoff fs.SizeSuffix) (fs.SizeSuffix, error) {
	var old fs.SizeSuffix
	old, f.opt.UploadCutoff = f.opt.UploadCutoff, cutoff
	return old, nil
}

var (
	_ fstests.SetUploadChunkSizer = (*Fs)(nil)
	_ fstests.SetUploadCutoffer   = (*Fs)(nil)
)

// rangeRecorder records the Range header of each download request.
type rangeRecorder struct {
	mu     sync.Mutex
	ranges []string
}

func (r *rangeRecorder) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ranges = append(r.ranges, s)
}

func (r *rangeRecorder) last() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.ranges) == 0 {
		return "<no request>"
	}
	return r.ranges[len(r.ranges)-1]
}

// newTestObject returns an Object backed by a local server providing the
// direct_link API and the file. If honourRange is false the file server
// ignores Range and always returns the whole file with a 200.
func newTestObject(t *testing.T, content []byte, honourRange bool) (*Object, *rangeRecorder) {
	rec := &rangeRecorder{}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/rclone/file/direct_link", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":200,"msg":"OK","result":{"url":%q,"size":%d}}`,
			srv.URL+"/d/token/file.bin", len(content))
	})
	mux.HandleFunc("/d/token/file.bin", func(w http.ResponseWriter, r *http.Request) {
		rec.add(r.Header.Get("Range"))
		if honourRange {
			http.ServeContent(w, r, "file.bin", time.Time{}, bytes.NewReader(content))
			return
		}
		_, _ = w.Write(content)
	})
	f := &Fs{
		name:   "test",
		client: srv.Client(),
		srv:    rest.NewClient(srv.Client()).SetRoot(srv.URL + "/rclone"),
		pacer:  pacer.New(),
	}
	return &Object{fs: f, remote: "file.bin", size: int64(len(content))}, rec
}

func TestOpenRange(t *testing.T) {
	ctx := context.Background()
	content := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	for _, honour := range []bool{true, false} {
		t.Run(fmt.Sprintf("serverHonoursRange=%v", honour), func(t *testing.T) {
			o, rec := newTestObject(t, content, honour)
			for _, tc := range []struct {
				name      string
				opts      []fs.OpenOption
				want      string
				wantRange string
			}{
				{"whole file", nil, string(content), ""},
				{"range", []fs.OpenOption{&fs.RangeOption{Start: 10, End: 19}}, "abcdefghij", "bytes=10-19"},
				{"open-ended range", []fs.OpenOption{&fs.RangeOption{Start: 30, End: -1}}, "uvwxyz", "bytes=30-35"},
				{"last 5 bytes", []fs.OpenOption{&fs.RangeOption{Start: -1, End: 5}}, "vwxyz", "bytes=31-35"},
				{"seek", []fs.OpenOption{&fs.SeekOption{Offset: 30}}, "uvwxyz", "bytes=30-35"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					rc, err := o.Open(ctx, tc.opts...)
					require.NoError(t, err)
					got, err := io.ReadAll(rc)
					require.NoError(t, err)
					require.NoError(t, rc.Close())
					assert.Equal(t, tc.want, string(got))
					assert.Equal(t, tc.wantRange, rec.last(), "Range header sent to the file server")
				})
			}
		})
	}
}
