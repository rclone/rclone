package alias

import (
	"errors"
	"path"
	"path/filepath"
	"strings"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "alias",
		Description: "Alias for a existing remote",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "remote",
			Help:     "Remote or path to alias.\nCan be \"myremote:path/to/dir\", \"myremote:bucket\", \"myremote:\" or \"/local/path\".",
			Required: true,
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	Remote string `config:"remote"`
}

// NewFs contstructs an Fs from the path.
//
// The returned Fs is the actual Fs, referenced by remote in the config
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
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
	_, configName, fsPath, err := fs.ParseRemote(opt.Remote)
	if err != nil {
		return nil, err
	}
	root = path.Join(fsPath, filepath.ToSlash(root))
	if configName == "local" {
		return fs.NewFs(root)
	}
	return fs.NewFs(configName + ":" + root)
}
