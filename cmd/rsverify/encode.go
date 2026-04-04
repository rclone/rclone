package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rclone/rclone/backend/rs"
	"github.com/rclone/rclone/fs/object"
	"github.com/spf13/cobra"
)

var (
	encodeK, encodeM int
	encodeOutDir     string
	encodeWithFooter = true
	encodeObjectName string
)

var cmdEncode = &cobra.Command{
	Use:   "encode [flags] <input-file>",
	Short: "Reed-Solomon encode a file into shard files",
	Args:  cobra.ExactArgs(1),
	RunE:  runEncode,
}

func init() {
	cmdEncode.Flags().IntVarP(&encodeK, "data-shards", "k", 0, "number of data shards (required)")
	cmdEncode.Flags().IntVarP(&encodeM, "parity-shards", "m", 0, "number of parity shards (required)")
	cmdEncode.Flags().StringVarP(&encodeOutDir, "out-dir", "o", ".", "output directory for shard files")
	cmdEncode.Flags().BoolVar(&encodeWithFooter, "footer", true, "append RCLONE/EC v2 footer to each shard (rclone particle format)")
	cmdEncode.Flags().StringVar(&encodeObjectName, "name", "", "logical object name in footers (default: input basename; unused if --footer=false)")
	_ = cmdEncode.MarkFlagRequired("data-shards")
	_ = cmdEncode.MarkFlagRequired("parity-shards")
}

func runEncode(_ *cobra.Command, args []string) error {
	if encodeK < 1 || encodeM < 1 {
		return fmt.Errorf("k and m must be >= 1")
	}
	inPath := args[0]
	data, err := os.ReadFile(inPath)
	if err != nil {
		return err
	}
	st, err := os.Stat(inPath)
	if err != nil {
		return err
	}
	name := encodeObjectName
	if name == "" {
		name = filepath.Base(inPath)
	}
	mtime := st.ModTime()
	if mtime.IsZero() {
		mtime = time.Unix(0, 0)
	}
	src := object.NewStaticObjectInfo(name, mtime, int64(len(data)), true, nil, nil)

	n := encodeK + encodeM
	buffers := make([]*bytes.Buffer, n)
	writers := make([]io.Writer, n)
	for i := range buffers {
		buffers[i] = &bytes.Buffer{}
		writers[i] = buffers[i]
	}

	ctx := context.Background()
	_, err = rs.BuildRSShardsToWriters(ctx, bytes.NewReader(data), src, encodeK, encodeM, writers, encodeWithFooter)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(encodeOutDir, 0o755); err != nil {
		return err
	}
	for i := range buffers {
		outPath := filepath.Join(encodeOutDir, fmt.Sprintf("shard-%02d.bin", i))
		if err := os.WriteFile(outPath, buffers[i].Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}
