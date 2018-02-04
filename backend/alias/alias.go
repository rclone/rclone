package alias

import (
	"errors"
	"path"
	"path/filepath"
	"strings"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config"
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "alias",
		Description: "Alias for a existing remote",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "remote",
			Help: "Remote or path to alias.\nCan be \"myremote:path/to/dir\", \"myremote:bucket\", \"myremote:\" or \"/local/path\".",
		}},
	}
	fs.Register(fsi)
}

// NewFs contstructs an Fs from the path.
//
// The returned Fs is the actual Fs, referenced by remote in the config
func NewFs(name, root string) (fs.Fs, error) {
	remote := config.FileGet(name, "remote")
	if remote == "" {
		return nil, errors.New("alias can't point to an empty remote - check the value of the remote setting")
	}
	if strings.HasPrefix(remote, name+":") {
		return nil, errors.New("can't point alias remote at itself - check the value of the remote setting")
	}
	fsInfo, configName, fsPath, err := fs.ParseRemote(remote)
	if err != nil {
		return nil, err
	}

	root = filepath.ToSlash(root)
	return fsInfo.NewFs(configName, path.Join(fsPath, root))
}
