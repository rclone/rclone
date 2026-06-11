package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/rs"
	"github.com/rclone/rclone/fs/object"
)

func TestLoadShardsValidatesVirtualPaddingLayout(t *testing.T) {
	ctx := context.Background()
	const k, m, S = 2, 2, 32
	data := bytes.Repeat([]byte("xy"), 50)
	src := object.NewStaticObjectInfo("t.bin", time.Unix(1700000000, 0), int64(len(data)), true, nil, nil)
	writers := make([]*bytes.Buffer, k+m)
	ios := make([]io.Writer, k+m)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	if _, err := rs.BuildRSShardsToWriters(ctx, bytes.NewReader(data), src, k, m, S, ios, true); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	paths := make([]string, k+m)
	for i := range writers {
		p := filepath.Join(dir, fmt.Sprintf("shard%d.bin", i))
		paths[i] = p
		if err := os.WriteFile(p, writers[i].Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, _, _, _, err := loadShardsFromParticles(paths); err != nil {
		t.Fatalf("load: %v", err)
	}
	// Truncate data shard 1 to legacy uniform size — should fail layout check.
	bad := append([]byte(nil), writers[1].Bytes()...)
	bad = append(bad, make([]byte, S)...)
	if err := os.WriteFile(paths[1], bad, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, _, err := loadShardsFromParticles(paths); err == nil {
		t.Fatal("expected layout validation error for padded legacy-sized particle")
	}
}
