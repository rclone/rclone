package vfs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/rc"
)

const getVFSHelp = ` 
This command takes an "fs" parameter. If this parameter is not
supplied and if there is only one VFS in use then that VFS will be
used. If there is more than one VFS in use then the "fs" parameter
must be supplied.`

// GetVFS gets a VFS with config name "fs" from the cache or returns an error.
//
// If "fs" is not set and there is one and only one VFS in the active
// cache then it returns it. This is for backwards compatibility.
func getVFS(in rc.Params) (vfs *VFS, err error) {
	fsString, err := in.GetString("fs")
	if rc.IsErrParamNotFound(err) {
		var count int
		vfs, count = activeCacheEntries()
		if count == 1 {
			return vfs, nil
		} else if count == 0 {
			return nil, errors.New(`no VFS active and "fs" parameter not supplied`)
		}
		return nil, errors.New(`more than one VFS active - need "fs" parameter`)
	} else if err != nil {
		return nil, err
	}
	activeMu.Lock()
	defer activeMu.Unlock()
	fsString = cache.Canonicalize(fsString)
	activeVFS := active[fsString]
	if len(activeVFS) == 0 {
		return nil, errors.Errorf("no VFS found with name %q", fsString)
	} else if len(activeVFS) > 1 {
		return nil, errors.Errorf("more than one VFS active with name %q", fsString)
	}
	return activeVFS[0], nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "vfs/refresh",
		Fn:    rcRefresh,
		Title: "Refresh the directory cache.",
		Help: `
This reads the directories for the specified paths and freshens the
directory cache.

If no paths are passed in then it will refresh the root directory.

    rclone rc vfs/refresh

Otherwise pass directories in as dir=path. Any parameter key
starting with dir will refresh that directory, eg

    rclone rc vfs/refresh dir=home/junk dir2=data/misc

If the parameter recursive=true is given the whole directory tree
will get refreshed. This refresh will use --fast-list if enabled.
` + getVFSHelp,
	})
}

func rcRefresh(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}

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

	recursive := false
	{
		const k = "recursive"

		if v, ok := in[k]; ok {
			s, ok := v.(string)
			if !ok {
				return out, errors.Errorf("value must be string %q=%v", k, v)
			}
			recursive, err = strconv.ParseBool(s)
			if err != nil {
				return out, errors.Errorf("invalid value %q=%v", k, v)
			}
			delete(in, k)
		}
	}

	result := map[string]string{}
	if len(in) == 0 {
		if recursive {
			err = root.readDirTree()
		} else {
			err = root.readDir()
		}
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
					if recursive {
						err = dir.readDirTree()
					} else {
						err = dir.readDir()
					}
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
}

// Add remote control for the VFS
func init() {
	rc.Add(rc.Call{
		Path:  "vfs/forget",
		Fn:    rcForget,
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
` + getVFSHelp,
	})
}

func rcForget(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}

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
}

func getDuration(k string, v interface{}) (time.Duration, error) {
	s, ok := v.(string)
	if !ok {
		return 0, errors.Errorf("value must be string %q=%v", k, v)
	}
	interval, err := fs.ParseDuration(s)
	if err != nil {
		return 0, errors.Wrap(err, "parse duration")
	}
	return interval, nil
}

func getInterval(in rc.Params) (time.Duration, bool, error) {
	k := "interval"
	v, ok := in[k]
	if !ok {
		return 0, false, nil
	}
	interval, err := getDuration(k, v)
	if err != nil {
		return 0, true, err
	}
	if interval < 0 {
		return 0, true, errors.New("interval must be >= 0")
	}
	delete(in, k)
	return interval, true, nil
}

func getTimeout(in rc.Params) (time.Duration, error) {
	k := "timeout"
	v, ok := in[k]
	if !ok {
		return 10 * time.Second, nil
	}
	timeout, err := getDuration(k, v)
	if err != nil {
		return 0, err
	}
	delete(in, k)
	return timeout, nil
}

func getStatus(vfs *VFS, in rc.Params) (out rc.Params, err error) {
	for k, v := range in {
		return nil, errors.Errorf("invalid parameter: %s=%s", k, v)
	}
	return rc.Params{
		"enabled":   vfs.Opt.PollInterval != 0,
		"supported": vfs.pollChan != nil,
		"interval": map[string]interface{}{
			"raw":     vfs.Opt.PollInterval,
			"seconds": vfs.Opt.PollInterval / time.Second,
			"string":  vfs.Opt.PollInterval.String(),
		},
	}, nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "vfs/poll-interval",
		Fn:    rcPollInterval,
		Title: "Get the status or update the value of the poll-interval option.",
		Help: `
Without any parameter given this returns the current status of the
poll-interval setting.

When the interval=duration parameter is set, the poll-interval value
is updated and the polling function is notified.
Setting interval=0 disables poll-interval.

    rclone rc vfs/poll-interval interval=5m

The timeout=duration parameter can be used to specify a time to wait
for the current poll function to apply the new value.
If timeout is less or equal 0, which is the default, wait indefinitely.

The new poll-interval value will only be active when the timeout is
not reached.

If poll-interval is updated or disabled temporarily, some changes
might not get picked up by the polling function, depending on the
used remote.
` + getVFSHelp,
	})
}

func rcPollInterval(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}

	interval, intervalPresent, err := getInterval(in)
	if err != nil {
		return nil, err
	}
	timeout, err := getTimeout(in)
	if err != nil {
		return nil, err
	}
	for k, v := range in {
		return nil, errors.Errorf("invalid parameter: %s=%s", k, v)
	}
	if vfs.pollChan == nil {
		return nil, errors.New("poll-interval is not supported by this remote")
	}

	if !intervalPresent {
		return getStatus(vfs, in)
	}
	var timeoutHit bool
	var timeoutChan <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		timeoutChan = timer.C
	}
	select {
	case vfs.pollChan <- interval:
		vfs.Opt.PollInterval = interval
	case <-timeoutChan:
		timeoutHit = true
	}
	out, err = getStatus(vfs, in)
	if out != nil {
		out["timeout"] = timeoutHit
	}
	return
}

func init() {
	rc.Add(rc.Call{
		Path:  "vfs/list",
		Title: "List active VFSes.",
		Help: `
This lists the active VFSes.

It returns a list under the key "vfses" where the values are the VFS
names that could be passed to the other VFS commands in the "fs"
parameter.`,
		Fn: rcList,
	})
}

func rcList(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	activeMu.Lock()
	defer activeMu.Unlock()
	var names = []string{}
	for name, vfses := range active {
		if len(vfses) == 1 {
			names = append(names, name)
		} else {
			for i := range vfses {
				names = append(names, fmt.Sprintf("%s[%d]", name, i))
			}
		}
	}
	out = rc.Params{}
	out["vfses"] = names
	return out, nil
}
