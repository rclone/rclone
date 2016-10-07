// Sync files and directories to and from local and remote object stores
//
// Nick Craig-Wood <nick@craig-wood.com>
package main

import (
	"log"

	"github.com/ncw/rclone/cmd"
	_ "github.com/ncw/rclone/cmd/all" // import all commands
	_ "github.com/ncw/rclone/fs/all"  // import all fs
)

func main() {
	if err := cmd.Root.Execute(); err != nil {
		log.Fatalf("Fatal error: %v", err)
	}
}
