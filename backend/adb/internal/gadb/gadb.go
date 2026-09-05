// Package gadb is an inline-vendored subset of github.com/electricbubble/gadb
// at commit 2e108649b dated 2025-03-14, MIT licensed. See
// backend/adb/internal/gadb/LICENSE for the original copyright and license
// text. The subset covers the ADB wire protocol (transport, sync transport,
// client, device) sufficient for the rclone backend's needs. Modifications
// are confined to mode.go (the fixupMode helper for os.FileMode mode_t bit
// translation) and sync_transport.go ReadDirectoryEntry (which calls into
// fixupMode); both are flagged at the modification site.
package gadb

import "log"

// debugFlag gates the package-internal debug log. Stays unexported because
// the vendored gadb subset is internal and rclone callers should use
// rclone's own --log-level / -vv flags. To enable verbose gadb wire-level
// logging during local debugging, flip this to true and rebuild.
var debugFlag = false

// debugLog emits a wire-level trace line when debugFlag is on. The vendored
// leaf cannot import the rclone fs package without creating an import
// cycle, so the gocritic nolint on log.Println stays.
func debugLog(msg string) {
	if !debugFlag {
		return
	}
	log.Println("[DEBUG] [gadb] " + msg) //nolint:gocritic // vendored leaf cannot import rclone fs
}
