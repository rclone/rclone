package chunkedreader_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunkedreader"
)

func TestSequentialRealFileEOF(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		size, offset, remaining, length int
		truncate                        bool
	}{
		{name: "empty", size: 0},
		{name: "short-final-chunk", size: 1500},
		{name: "exact-chunk", size: 1024},
		{name: "multiple-chunks", size: 3073},
		{name: "seek", size: 3073, offset: 513},
		{name: "bounded-first-chunk", size: 1500, offset: 127, length: 31},
		{name: "bounded-exact-end", size: 1500, offset: 1470, length: 30},
		{name: "bounded-past-end", size: 1500, offset: 1470, length: 64},
		{name: "premature-zero", size: 1500, truncate: true, remaining: 0},
		{name: "premature-partial", size: 1500, truncate: true, remaining: 127},
		{name: "premature-after-seek", size: 1500, offset: 100, truncate: true, remaining: 227},
		{name: "premature-bounded-range", size: 1500, offset: 100, length: 31, truncate: true, remaining: 127},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			payload := bytes.Repeat([]byte("x"), tc.size)
			path := filepath.Join(dir, "payload")
			if err := os.WriteFile(path, payload, 0600); err != nil {
				t.Fatal(err)
			}
			f, err := fs.NewFs(ctx, dir)
			if err != nil {
				t.Fatal(err)
			}
			obj, err := f.NewObject(ctx, "payload")
			if err != nil {
				t.Fatal(err)
			}
			reader := chunkedreader.New(ctx, obj, 1024, 4096, 0)
			if tc.length > 0 {
				if _, err = reader.RangeSeek(ctx, int64(tc.offset), io.SeekStart, int64(tc.length)); err != nil {
					t.Fatal(err)
				}
			} else if tc.offset != 0 {
				if _, err = reader.Seek(int64(tc.offset), io.SeekStart); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = reader.Open(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := reader.Close(); err != nil {
					t.Error(err)
				}
			})
			// Truncate the real open file after metadata was obtained: a short source
			// must not become successful EOF before the advertised object end.
			if tc.truncate {
				if err = os.Truncate(path, int64(tc.remaining)); err != nil {
					t.Fatal(err)
				}
			}
			actual, err := io.ReadAll(reader)
			end := tc.size
			if tc.truncate {
				end = tc.remaining
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatalf("short source returned %v; wanted unexpected EOF (read %d bytes)", err, len(actual))
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, payload[tc.offset:end]) {
				t.Fatalf("payload mismatch: got %d bytes, expected %d", len(actual), end-tc.offset)
			}
		})
	}
}
