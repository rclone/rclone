package main

import (
	"fmt"
	"os"

	"github.com/rclone/rclone/backend/rs"
	"github.com/spf13/cobra"
)

var (
	decodeOutPath string
	decodeRaw     bool
	decodeK       int
	decodeM       int
	decodeLength  int64
)

var cmdDecode = &cobra.Command{
	Use:   "decode [flags] <shard-file> [<shard-file> ...]",
	Short: "Reconstruct original bytes from shards (auto-detect footer unless --raw)",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runDecode,
}

func init() {
	cmdDecode.Flags().StringVarP(&decodeOutPath, "output", "o", "-", "output file (- for stdout)")
	cmdDecode.Flags().BoolVar(&decodeRaw, "raw", false, "shards are raw payloads only (no footer); requires -k, -m, --length")
	cmdDecode.Flags().IntVarP(&decodeK, "data-shards", "k", 0, "data shards (required with --raw)")
	cmdDecode.Flags().IntVarP(&decodeM, "parity-shards", "m", 0, "parity shards (required with --raw)")
	cmdDecode.Flags().Int64Var(&decodeLength, "length", -1, "original content length (required with --raw)")
}

func runDecode(_ *cobra.Command, args []string) error {
	if decodeRaw {
		if decodeK < 1 || decodeM < 1 || decodeLength < 0 {
			return fmt.Errorf("--raw requires -k, -m, and --length")
		}
		return decodeRawShards(args, decodeOutPath)
	}
	return decodeWithFooters(args, decodeOutPath)
}

func decodeWithFooters(paths []string, outPath string) error {
	shards, k, m, contentLen, _, err := loadShardsFromParticles(paths)
	if err != nil {
		return err
	}
	if err := countShardsOK(shards, k); err != nil {
		return err
	}
	out, err := rs.ReconstructDataFromShards(shards, k, m, contentLen)
	if err != nil {
		return err
	}
	return writeOutput(outPath, out)
}

func decodeRawShards(paths []string, outPath string) error {
	if len(paths) != decodeK+decodeM {
		return fmt.Errorf("raw mode: need exactly k+m=%d shard files, got %d", decodeK+decodeM, len(paths))
	}
	shards := make([][]byte, len(paths))
	for i, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		shards[i] = b
	}
	out, err := rs.ReconstructDataFromShards(shards, decodeK, decodeM, decodeLength)
	if err != nil {
		return err
	}
	return writeOutput(outPath, out)
}

func writeOutput(path string, data []byte) error {
	if path == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
