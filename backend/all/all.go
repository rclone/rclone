// Package all imports all the backends
package all

import (
	// Active file systems
	_ "github.com/rclone/rclone/backend/alias"
	_ "github.com/rclone/rclone/backend/cache"
	_ "github.com/rclone/rclone/backend/chunker"
	_ "github.com/rclone/rclone/backend/combine"
	_ "github.com/rclone/rclone/backend/compress"
	_ "github.com/rclone/rclone/backend/crypt"
	_ "github.com/rclone/rclone/backend/dropbox"
	_ "github.com/rclone/rclone/backend/ftp"
	_ "github.com/rclone/rclone/backend/hasher"
	_ "github.com/rclone/rclone/backend/http"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/mega"
	_ "github.com/rclone/rclone/backend/memory"
	_ "github.com/rclone/rclone/backend/onedrive"
	_ "github.com/rclone/rclone/backend/sftp"
	_ "github.com/rclone/rclone/backend/smb"
	_ "github.com/rclone/rclone/backend/terabox"
	_ "github.com/rclone/rclone/backend/union"
	_ "github.com/rclone/rclone/backend/webdav"
)
