// Sync files and directories to and from local and remote object stores
//
// Nick Craig-Wood <nick@craig-wood.com>
package main

import (
	_ "github.com/ncw/rclone/backend/all" // import all backends
	"github.com/ncw/rclone/cmd"
	_ "github.com/ncw/rclone/cmd/all" // import all commands
)

func main() {
	cmd.Main()
}
