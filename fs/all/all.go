package all

import (
	// Active file systems
	_ "github.com/ncw/rclone/amazonclouddrive"
	_ "github.com/ncw/rclone/b2"
	_ "github.com/ncw/rclone/crypt"
	_ "github.com/ncw/rclone/drive"
	_ "github.com/ncw/rclone/dropbox"
	_ "github.com/ncw/rclone/googlecloudstorage"
	_ "github.com/ncw/rclone/hubic"
	_ "github.com/ncw/rclone/local"
	_ "github.com/ncw/rclone/onedrive"
	_ "github.com/ncw/rclone/s3"
	_ "github.com/ncw/rclone/swift"
	_ "github.com/ncw/rclone/yandex"
)
