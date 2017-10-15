package all

import (
	// Active file systems
	_ "github.com/ncw/rclone/backend/alias"
	_ "github.com/ncw/rclone/backend/amazonclouddrive"
	_ "github.com/ncw/rclone/backend/azureblob"
	_ "github.com/ncw/rclone/backend/b2"
	_ "github.com/ncw/rclone/backend/box"
	_ "github.com/ncw/rclone/backend/cache"
	_ "github.com/ncw/rclone/backend/crypt"
	_ "github.com/ncw/rclone/backend/drive"
	_ "github.com/ncw/rclone/backend/dropbox"
	_ "github.com/ncw/rclone/backend/ftp"
	_ "github.com/ncw/rclone/backend/googlecloudstorage"
	_ "github.com/ncw/rclone/backend/http"
	_ "github.com/ncw/rclone/backend/hubic"
	_ "github.com/ncw/rclone/backend/local"
	_ "github.com/ncw/rclone/backend/mega"
	_ "github.com/ncw/rclone/backend/onedrive"
	_ "github.com/ncw/rclone/backend/pcloud"
	_ "github.com/ncw/rclone/backend/qingstor"
	_ "github.com/ncw/rclone/backend/s3"
	_ "github.com/ncw/rclone/backend/sftp"
	_ "github.com/ncw/rclone/backend/swift"
	_ "github.com/ncw/rclone/backend/webdav"
	_ "github.com/ncw/rclone/backend/yandex"
)
