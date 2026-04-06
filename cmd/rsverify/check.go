package main

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/rclone/rclone/backend/rs"
	"github.com/spf13/cobra"
)

var (
	checkReference string
	checkStrict    bool
)

var cmdCheck = &cobra.Command{
	Use:   "check [flags] <shard-file> [<shard-file> ...]",
	Short: "Verify particles: footers, CRCs, optional reference file / hashes",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runCheck,
}

func init() {
	cmdCheck.Flags().StringVarP(&checkReference, "reference", "r", "", "original file to compare decoded bytes")
	cmdCheck.Flags().BoolVar(&checkStrict, "strict", false, "require all k+m shards present")
}

func runCheck(_ *cobra.Command, args []string) error {
	shards, k, m, contentLen, refFooter, err := loadShardsFromParticles(args)
	if err != nil {
		return err
	}
	present := 0
	for _, s := range shards {
		if s != nil {
			present++
		}
	}
	if checkStrict && present != k+m {
		return fmt.Errorf("strict: want %d shards, have %d", k+m, present)
	}
	if err := countShardsOK(shards, k); err != nil {
		return err
	}
	out, err := rs.ReconstructDataFromShards(shards, k, m, contentLen)
	if err != nil {
		return fmt.Errorf("reconstruct: %w", err)
	}
	if checkReference != "" {
		ref, err := os.ReadFile(checkReference)
		if err != nil {
			return err
		}
		if !bytes.Equal(out, ref) {
			return fmt.Errorf("decoded bytes differ from reference %q", checkReference)
		}
	}
	if refFooter != nil {
		if h := md5.Sum(out); !bytes.Equal(h[:], refFooter.MD5[:]) {
			return fmt.Errorf("MD5 mismatch vs footer")
		}
		if h := sha256.Sum256(out); !bytes.Equal(h[:], refFooter.SHA256[:]) {
			return fmt.Errorf("SHA256 mismatch vs footer")
		}
	}
	fmt.Fprintf(os.Stderr, "rsverify check: OK (%d data + %d parity shards, %d bytes)\n", k, m, len(out))
	return nil
}
