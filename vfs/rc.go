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
	rc.Add(rc.Call{
		Path: "vfs/refresh",
		Fn: func(in rc.Params) (out rc.Params, err error) {
			root, err := vfs.Root()
			if err != nil {
				return nil, err
			}
			getDir := func(path string) (*Dir, error) {
				path = strings.Trim(path, "/")
				segments := strings.Split(path, "/")
				var node Node = root
				for _, s := range segments {
					if dir, ok := node.(*Dir); ok {
						node, err = dir.stat(s)
						if err != nil {
							return nil, err
						}
					}
				}
				if dir, ok := node.(*Dir); ok {
					return dir, nil
				}
				return nil, EINVAL
			}

			result := map[string]string{}
			if len(in) == 0 {
				err = root.readDirTree()
				if err != nil {
					result[""] = err.Error()
				} else {
					result[""] = "OK"
				}
			} else {
				for k, v := range in {
					path, ok := v.(string)
					if !ok {
						return out, errors.Errorf("value must be string %q=%v", k, v)
					}
					if strings.HasPrefix(k, "dir") {
						dir, err := getDir(path)
						if err != nil {
							result[path] = err.Error()
						} else {
							err = dir.readDirTree()
							if err != nil {
								result[path] = err.Error()
							} else {
								result[path] = "OK"
							}

						}
					} else {
						return out, errors.Errorf("unknown key %q", k)
					}
				}
			}
			out = rc.Params{
				"result": result,
			}
			return out, nil
		},
		Title: "Refresh the directory cache tree.",
		Help: `
This reads the full directory tree for the paths and freshens the
directory cache.

If no paths are passed in then it will refresh the root directory.

    rclone rc vfs/refresh

Otherwise pass directories in as dir=path. Any parameter key
starting with dir will refresh that directory, eg

    rclone rc vfs/refresh dir=home/junk dir2=data/misc

This refresh will use --fast-list if enabled.

`,
	})
}
