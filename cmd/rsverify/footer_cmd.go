package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rclone/rclone/backend/rs"
	"github.com/spf13/cobra"
)

var footerJSON bool

var cmdFooter = &cobra.Command{
	Use:   "footer <particle-file>",
	Short: "Parse and display EC footer from a particle file",
	Args:  cobra.ExactArgs(1),
	RunE:  runFooter,
}

func init() {
	cmdFooter.Flags().BoolVar(&footerJSON, "json", false, "print footer as JSON")
}

func runFooter(_ *cobra.Command, args []string) error {
	raw, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	payload, ft, ok := tryParticleFooter(raw)
	if !ok {
		return fmt.Errorf("not a valid rclone EC particle (footer magic/CRC)")
	}
	if footerJSON {
		return printFooterJSON(ft, len(payload))
	}
	fmt.Printf("Valid particle: payload=%d bytes, footer=%d bytes\n", len(payload), rs.FooterSize)
	fmt.Printf("ContentLength: %d\n", ft.ContentLength)
	fmt.Printf("DataShards: %d  ParityShards: %d  CurrentShard: %d\n", ft.DataShards, ft.ParityShards, ft.CurrentShard)
	fmt.Printf("StripeSize: %d\n", ft.StripeSize)
	fmt.Printf("PayloadCRC32C: 0x%08x\n", ft.PayloadCRC32C)
	fmt.Printf("MD5: %x\n", ft.MD5)
	fmt.Printf("SHA256: %x\n", ft.SHA256)
	return nil
}

type footerDump struct {
	PayloadBytes  int    `json:"payload_bytes"`
	ContentLength int64  `json:"content_length"`
	DataShards    int    `json:"data_shards"`
	ParityShards  int    `json:"parity_shards"`
	CurrentShard  int    `json:"current_shard"`
	StripeSize    uint32 `json:"stripe_size"`
	PayloadCRC32C uint32 `json:"payload_crc32c"`
	MD5           string `json:"md5"`
	SHA256        string `json:"sha256"`
}

func printFooterJSON(ft *rs.Footer, payloadLen int) error {
	d := footerDump{
		PayloadBytes:  payloadLen,
		ContentLength: ft.ContentLength,
		DataShards:    int(ft.DataShards),
		ParityShards:  int(ft.ParityShards),
		CurrentShard:  int(ft.CurrentShard),
		StripeSize:    ft.StripeSize,
		PayloadCRC32C: ft.PayloadCRC32C,
		MD5:           fmt.Sprintf("%x", ft.MD5),
		SHA256:        fmt.Sprintf("%x", ft.SHA256),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}
