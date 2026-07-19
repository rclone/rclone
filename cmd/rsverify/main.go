// Command rsverify encodes, decodes, and checks rclone rs (Reed-Solomon) particles.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "rsverify",
	Short:         "Encode, decode, or verify rclone rs EC particles",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "rsverify: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(cmdEncode, cmdDecode, cmdCheck, cmdFooter)
}
