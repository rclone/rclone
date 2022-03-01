package digistorage

import (
	"context"

	"github.com/rclone/rclone/backend/koofr"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/lib/encoder"
)

// Register Fs with rclone
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "digistorage",
		Description: "Digi Storage",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "mountid",
			Help:     "Mount ID of the mount to use.\n\nIf omitted, the primary mount is used.",
			Advanced: true,
		}, {
			Name:     "setmtime",
			Help:     "Does the backend support setting modification time.\n\nSet this to false if you use a mount ID that points to a Dropbox or Amazon Drive backend.",
			Default:  true,
			Advanced: true,
		}, {
			Name:     "user",
			Help:     "Your Digi Storage user name.",
			Required: true,
		}, {
			Name:       "password",
			Help:       "Your Digi Storage password for rclone (generate one at https://storage.rcs-rds.ro/app/admin/preferences/password).",
			IsPassword: true,
			Required:   true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Encode invalid UTF-8 bytes as json doesn't handle them properly.
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// NewFs constructs a new filesystem given a root path and configuration options
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (ff fs.Fs, err error) {
	opt := new(koofr.Options)
	err = configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	opt.Endpoint = "https://storage.rcs-rds.ro"
	return koofr.NewFsFromOptions(ctx, name, root, opt)
}
