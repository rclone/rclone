package vfs

import (
	"strings"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/rc"
	"github.com/pkg/errors"
)

// Add remote control for the VFS
func (vfs *VFS) addRC() {
	rc.Add(rc.Call{
		Path: "vfs/forget",
		Fn: func(in rc.Params) (out rc.Params, err error) {
			root, err := vfs.Root()
			if err != nil {
				return nil, err
			}

			forgotten := []string{}
			if len(in) == 0 {
				root.ForgetAll()
			} else {
				for k, v := range in {
					path, ok := v.(string)
					if !ok {
						return out, errors.Errorf("value must be string %q=%v", k, v)
					}
					path = strings.Trim(path, "/")
					if strings.HasPrefix(k, "file") {
						root.ForgetPath(path, fs.EntryObject)
					} else if strings.HasPrefix(k, "dir") {
						root.ForgetPath(path, fs.EntryDirectory)
					} else {
						return out, errors.Errorf("unknown key %q", k)
					}
					forgotten = append(forgotten, path)
				}
			}
			out = rc.Params{
				"forgotten": forgotten,
			}
			return out, nil
		},
		Title: "Forget files or directories in the directory cache.",
		Help: `
This forgets the paths in the directory cache causing them to be
re-read from the remote when needed.

If no paths are passed in then it will forget all the paths in the
directory cache.

    rclone rc vfs/forget

Otherwise pass files or dirs in as file=path or dir=path.  Any
parameter key starting with file will forget that file and any
starting with dir will forget that dir, eg

    rclone rc vfs/forget file=hello file2=goodbye dir=home/junk

`,
	})
}
