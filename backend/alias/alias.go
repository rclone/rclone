// Package alias implements a virtual provider to rename existing remotes.
package alias

import (
	"context"
	"errors"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fspath"
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "alias",
		Description: "Alias for an existing remote",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "remote",
			Help:     "Remote or path to alias.\n\nCan be \"myremote:path/to/dir\", \"myremote:bucket\", \"myremote:\" or \"/local/path\".",
			Required: true,
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	Remote string `config:"remote"`
}

// NewFs constructs an Fs from the path.
//
// The returned Fs is the actual Fs, referenced by remote in the config
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.Remote == "" {
		return nil, errors.New("alias can't point to an empty remote - check the value of the remote setting")
	}
	if strings.HasPrefix(opt.Remote, name+":") {
		return nil, errors.New("can't point alias remote at itself - check the value of the remote setting")
	}
	return cache.Get(ctx, fspath.JoinRootPath(opt.Remote, root))
}
