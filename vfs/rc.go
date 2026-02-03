package vfs

import (
	"context"
	"errors"
	"fmt"
	pathpkg "path"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs/vfscache"
	"github.com/rclone/rclone/vfs/vfscache/writeback"
	"github.com/rclone/rclone/vfs/vfscommon"
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
//
// This deletes the "fs" parameter from in if it is valid
func getVFS(in rc.Params) (vfs *VFS, err error) {
	fsString, err := in.GetString("fs")
	if rc.IsErrParamNotFound(err) {
		var count int
		vfs, count = activeCacheEntries()
		switch count {
		case 1:
			return vfs, nil
		case 0:
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
		return nil, fmt.Errorf("no VFS found with name %q", fsString)
	} else if len(activeVFS) > 1 {
		return nil, fmt.Errorf("more than one VFS active with name %q", fsString)
	}
	delete(in, "fs") // delete the fs parameter
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
starting with dir will refresh that directory, e.g.

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
				return out, fmt.Errorf("value must be string %q=%v", k, v)
			}
			recursive, err = strconv.ParseBool(s)
			if err != nil {
				return out, fmt.Errorf("invalid value %q=%v", k, v)
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
				return out, fmt.Errorf("value must be string %q=%v", k, v)
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
				return out, fmt.Errorf("unknown key %q", k)
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
starting with dir will forget that dir, e.g.

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
				return out, fmt.Errorf("value must be string %q=%v", k, v)
			}
			path = strings.Trim(path, "/")
			if strings.HasPrefix(k, "file") {
				root.ForgetPath(path, fs.EntryObject)
			} else if strings.HasPrefix(k, "dir") {
				root.ForgetPath(path, fs.EntryDirectory)
			} else {
				return out, fmt.Errorf("unknown key %q", k)
			}
			forgotten = append(forgotten, path)
		}
	}
	out = rc.Params{
		"forgotten": forgotten,
	}
	return out, nil
}

func getDuration(k string, v any) (time.Duration, error) {
	s, ok := v.(string)
	if !ok {
		return 0, fmt.Errorf("value must be string %q=%v", k, v)
	}
	interval, err := fs.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("parse duration: %w", err)
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
		return 0, true, rc.NewErrParamInvalid(errors.New("interval must be >= 0"))
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
		return nil, rc.NewErrParamInvalid(errors.New(fmt.Sprintf("invalid parameter: %s=%s", k, v)))
	}
	return rc.Params{
		"enabled":   vfs.Opt.PollInterval != 0,
		"supported": vfs.pollChan != nil,
		"interval": map[string]any{
			"raw":     vfs.Opt.PollInterval,
			"seconds": time.Duration(vfs.Opt.PollInterval) / time.Second,
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
		return nil, rc.NewErrParamInvalid(errors.New(fmt.Sprintf("invalid parameter: %s=%s", k, v)))
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
		vfs.Opt.PollInterval = fs.Duration(interval)
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
	names := make([]string, 0)
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

func init() {
	rc.Add(rc.Call{
		Path:  "vfs/stats",
		Title: "Stats for a VFS.",
		Help: `
This returns stats for the selected VFS.

    {
        // Status of the disk cache - only present if --vfs-cache-mode > off
        "diskCache": {
            "bytesUsed": 0,
            "erroredFiles": 0,
            "files": 0,
            "hashType": 1,
            "outOfSpace": false,
            "path": "/home/user/.cache/rclone/vfs/local/mnt/a",
            "pathMeta": "/home/user/.cache/rclone/vfsMeta/local/mnt/a",
            "uploadsInProgress": 0,
            "uploadsQueued": 0
        },
        "fs": "/mnt/a",
        "inUse": 1,
        // Status of the in memory metadata cache
        "metadataCache": {
            "dirs": 1,
            "files": 0
        },
        // Options as returned by options/get
        "opt": {
            "CacheMaxAge": 3600000000000,
            // ...
            "WriteWait": 1000000000
        }
    }

` + getVFSHelp,
		Fn: rcStats,
	})
}

func rcStats(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}
	return vfs.Stats(), nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "vfs/queue",
		Title: "Queue info for a VFS.",
		Help: strings.ReplaceAll(`
This returns info about the upload queue for the selected VFS.

This is only useful if |--vfs-cache-mode| > off. If you call it when
the |--vfs-cache-mode| is off, it will return an empty result.

    {
        "queued": // an array of files queued for upload
        [
            {
                "name":      "file",   // string: name (full path) of the file,
                "id":        123,      // integer: id of this item in the queue,
                "size":      79,       // integer: size of the file in bytes
                "expiry":    1.5       // float: time until file is eligible for transfer, lowest goes first
                "tries":     1,        // integer: number of times we have tried to upload
                "delay":     5.0,      // float: seconds between upload attempts
                "uploading": false,    // boolean: true if item is being uploaded
            },
       ],
    }

The |expiry| time is the time until the file is eligible for being
uploaded in floating point seconds. This may go negative. As rclone
only transfers |--transfers| files at once, only the lowest
|--transfers| expiry times will have |uploading| as |true|. So there
may be files with negative expiry times for which |uploading| is
|false|.

`, "|", "`") + getVFSHelp,
		Fn: rcQueue,
	})
}

func rcQueue(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}
	if vfs.cache == nil {
		return nil, nil
	}
	return vfs.cache.Queue(), nil
}

func init() {
	rc.Add(rc.Call{
		Path:  "vfs/queue-set-expiry",
		Title: "Set the expiry time for an item queued for upload.",
		Help: strings.ReplaceAll(`

Use this to adjust the |expiry| time for an item in the upload queue.
You will need to read the |id| of the item using |vfs/queue| before
using this call.

You can then set |expiry| to a floating point number of seconds from
now when the item is eligible for upload. If you want the item to be
uploaded as soon as possible then set it to a large negative number (eg
-1000000000). If you want the upload of the item to be delayed
for a long time then set it to a large positive number.

Setting the |expiry| of an item which has already has started uploading
will have no effect - the item will carry on being uploaded.

This will return an error if called with |--vfs-cache-mode| off or if
the |id| passed is not found.

This takes the following parameters

- |fs| - select the VFS in use (optional)
- |id| - a numeric ID as returned from |vfs/queue|
- |expiry| - a new expiry time as floating point seconds
- |relative| - if set, expiry is to be treated as relative to the current expiry (optional, boolean)

This returns an empty result on success, or an error.

`, "|", "`") + getVFSHelp,
		Fn: rcQueueSetExpiry,
	})
}

func rcQueueSetExpiry(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}
	if vfs.cache == nil {
		return nil, rc.NewErrParamInvalid(errors.New("can't call this unless using the VFS cache"))
	}

	// Read input values
	id, err := in.GetInt64("id")
	if err != nil {
		return nil, err
	}
	expiry, err := in.GetFloat64("expiry")
	if err != nil {
		return nil, err
	}
	relative, err := in.GetBool("relative")
	if err != nil && !rc.IsErrParamNotFound(err) {
		return nil, err
	}

	// Set expiry
	var refTime time.Time
	if !relative {
		refTime = time.Now()
	}
	err = vfs.cache.QueueSetExpiry(writeback.Handle(id), refTime, time.Duration(float64(time.Second)*expiry))
	return nil, err
}

func init() {
	rc.Add(rc.Call{
		Path:  "vfs/status",
		Fn:    rcStatus,
		Title: "Get aggregate cache status statistics.",
		Help: `
This returns aggregate cache status statistics for the VFS, including
counts for each cache status type.

This takes the following parameters:

- fs - select the VFS in use (optional)

This returns a JSON object with the following fields:

- totalFiles - total number of files currently tracked by the cache (only includes files that have been accessed)
- totalCachedBytes - total bytes cached across all tracked files
- averageCachePercentage - average cache percentage across all tracked files (0-100)
- counts - object containing counts for each cache status:
  - FULL - number of files completely cached locally
  - PARTIAL - number of files partially cached
  - NONE - number of tracked files not cached (remote only)
  - DIRTY - number of files modified locally but not uploaded
  - UPLOADING - number of files currently being uploaded
  - ERROR - number of files in error state (items that have experienced errors like reset failures and are tracked in the error items map, regardless of their current cache status)
- fs - file system path

Note: These statistics only reflect files that are currently tracked by the VFS cache.
Files that have never been accessed through the VFS are not included in these counts.
` + getVFSHelp,
	})

	rc.Add(rc.Call{
		Path:  "vfs/file-status",
		Fn:    rcFileStatus,
		Title: "Get detailed cache status of one or more files.",
		Help: `
This returns detailed cache status of files including name, status, percentage,
size, cached bytes, dirty flag, and uploading status.

This takes the following parameters:

- fs - select the VFS in use (optional)
- file - the path to the file to get the status of (can be repeated as file1, file2, etc.)

This returns a JSON object with the following fields:

- files - array of file objects with fields:
  - name - leaf name of the file
  - status - one of "FULL", "PARTIAL", "NONE", "DIRTY", "UPLOADING", "ERROR"
  - percentage - cache percentage (0-100), representing the percentage of the file cached locally
  - uploading - whether the file is currently being uploaded
  - size - total file size in bytes
  - cachedBytes - bytes cached locally
  - dirty - whether the file has uncommitted modifications
  - error - generic error message if there was an error getting file information (optional).
    For security, only a generic message is returned; detailed error information is logged internally.
  - fs - file system path

Note: The percentage field indicates how much of the file is cached locally (0-100).
For "FULL" and "DIRTY" status, it is always 100 since the local file is complete.
For "UPLOADING" status, it is also 100 which represents the percentage of the file
that is cached locally (not the upload progress). It is only meaningful for "PARTIAL"
status files where it shows the actual percentage cached.
If the file cannot be found or accessed, the status will be "ERROR" and an
"error" field will be included with a generic message for security (detailed
errors are logged internally).
` + getVFSHelp,
	})

	rc.Add(rc.Call{
		Path:  "vfs/dir-status",
		Fn:    rcDirStatus,
		Title: "Get cache status of files in a directory.",
		Help: `
This returns cache status for files in a specified directory that are currently
tracked by the VFS cache, optionally including subdirectories. This is ideal for
file manager integrations that need to display cache status overlays for directory
listings.

This takes the following parameters:

- fs - select the VFS in use (optional)
- dir - the path to the directory to get the status of (optional, defaults to root)
- recursive - if true, include all subdirectories (optional, defaults to false)

This returns a JSON object with the following fields:

- dir - the directory path that was scanned
- files - object containing arrays of files grouped by their cache status.
  All status categories are always present (may be empty arrays):
  - FULL - array of completely cached files
  - PARTIAL - array of partially cached files
  - NONE - array of tracked files not cached (remote only)
  - DIRTY - array of files modified locally but not uploaded
  - UPLOADING - array of files currently being uploaded
  - ERROR - array of files with errors (e.g., reset failures)
- Each file entry includes:
  - name - the file name (use / as path separator)
  - percentage - cache percentage (0-100). For UPLOADING files, this represents
    the percentage of the file cached locally, not upload progress
  - uploading - whether the file is currently being uploaded
- recursive - whether subdirectories were included in the scan
- fs - the file system path

Note: This endpoint only returns files that are currently tracked by the VFS cache
(files that have been accessed). It does not list all files in the remote directory.

Example:
  rclone rc vfs/dir-status dir=/documents
  rclone rc vfs/dir-status dir=/documents recursive=true
` + getVFSHelp,
	})
}

func rcDirStatus(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}

	// dir parameter is optional - defaults to root
	dirPath, err := in.GetString("dir")
	if err != nil && !rc.IsErrParamNotFound(err) {
		return nil, err
	}

	// Check for recursive parameter
	recursive, err := in.GetBool("recursive")
	if err != nil && !rc.IsErrParamNotFound(err) {
		return nil, rc.NewErrParamInvalid(errors.New(fmt.Sprintf("invalid recursive parameter: %v", err)))
	}

	// Validate directory if specified - ensure it's not a file
	// We prefer checking the cache first to avoid expensive remote lookups.
	// If cache is available, use it to validate without remote calls.
	// We don't check if directory exists in VFS because cache may contain
	// items under that path even if directory node itself hasn't been read
	if dirPath != "" {
		// Normalize path
		cleanPath := vfscommon.NormalizePath(dirPath)

		// Check if path is a file (not a directory)
		// First check cache, then fall back to vfs.Stat for files not in cache
		isFile := false
		if vfs.cache != nil {
			if item := vfs.cache.FindItem(cleanPath); item != nil {
				// Path is in cache, check if it's a file
				if node, err := vfs.Stat(cleanPath); err == nil && !node.IsDir() {
					isFile = true
				}
			} else {
				// Path not in cache - still need to check if it exists as a file
				// This handles the case where a file exists but isn't cached
				if node, err := vfs.Stat(cleanPath); err == nil && !node.IsDir() {
					isFile = true
				}
			}
		} else {
			// Cache is disabled, must use vfs.Stat
			if node, err := vfs.Stat(cleanPath); err == nil && !node.IsDir() {
				isFile = true
			}
		}

		// If path exists and is not a directory, return error
		if isFile {
			return nil, rc.NewErrParamInvalid(errors.New(fmt.Sprintf("path %q is not a directory", dirPath)))
		}
	}

	// Get files status using the cache
	var filesByStatus map[string][]rc.Params
	if vfs.cache == nil {
		// If cache is not enabled, return empty results with all categories
		filesByStatus = make(map[string][]rc.Params)
		for _, status := range vfscache.CacheStatuses {
			filesByStatus[status] = []rc.Params{}
		}
	} else {
		filesByStatus = vfs.cache.GetStatusForDir(dirPath, recursive)
	}

	// Prepare the response - always include all categories for a stable API
	responseFiles := rc.Params{}
	for _, status := range vfscache.CacheStatuses {
		responseFiles[status] = filesByStatus[status]
	}

	return rc.Params{
		"dir":       dirPath,
		"files":     responseFiles,
		"recursive": recursive,
		"fs":        fs.ConfigString(vfs.Fs()),
	}, nil
}

func rcFileStatus(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}

	// Support both single file and multiple files
	var paths []string

	// Check for "file" parameter (single file)
	if path, err := in.GetString("file"); err == nil {
		if path == "" {
			return nil, rc.NewErrParamInvalid(errors.New("empty file parameter"))
		}
		paths = append(paths, path)
	} else if !rc.IsErrParamNotFound(err) {
		return nil, err
	}

	// Check for multiple file parameters (file1, file2, etc.)
	for i := 1; ; i++ {
		if len(paths) >= 100 {
			return nil, rc.NewErrParamInvalid(errors.New("too many file parameters provided (max 100)"))
		}
		key := "file" + strconv.Itoa(i)
		path, pathErr := in.GetString(key)
		if pathErr != nil {
			if rc.IsErrParamNotFound(pathErr) {
				break // No more file parameters
			}
			return nil, pathErr
		}
		if path == "" {
			return nil, rc.NewErrParamInvalid(fmt.Errorf("empty %s parameter", key))
		}
		paths = append(paths, path)
	}

	// If no files found, return error
	if len(paths) == 0 {
		return nil, rc.NewErrParamInvalid(errors.New("no file parameter(s) provided"))
	}

	// Collect status for each file
	results := make([]rc.Params, 0, len(paths))
	for _, path := range paths {
		var result rc.Params

		// Normalize path to match cache key format
		cleanPath := vfscommon.NormalizePath(path)
		baseName := pathpkg.Base(cleanPath)

		// Check if cache is enabled and file exists in cache
		if vfs.cache != nil {
			if item := vfs.cache.FindItem(cleanPath); item != nil {
				status, percentage, totalSize, cachedSize, isDirty := item.VFSStatusCacheDetailed()
				isUploading := status == vfscache.CacheStatusUploading
				result = rc.Params{
					"name":        baseName,
					"status":      status,
					"percentage":  percentage,
					"uploading":   isUploading,
					"size":        totalSize,
					"cachedBytes": cachedSize,
					"dirty":       isDirty,
				}
				results = append(results, result)
				continue
			}
		}

		// File not in cache or cache disabled, return NONE or ERROR status
		size := int64(0)
		hasError := false
		// Attempt to get file size from VFS using normalized path
		if node, err := vfs.Stat(cleanPath); err == nil {
			size = node.Size()
		} else {
			// Log detailed error internally for debugging
			fs.Debugf(vfs.Fs(), "vfs/file-status: error getting file info for %q: %v", cleanPath, err)
			hasError = true
		}
		fileStatus := vfscache.CacheStatusNone
		if hasError {
			fileStatus = vfscache.CacheStatusError
		}
		result = rc.Params{
			"name":        baseName,
			"status":      fileStatus,
			"percentage":  0,
			"uploading":   false,
			"size":        size,
			"cachedBytes": 0,
			"dirty":       false,
		}
		if hasError {
			result["error"] = "file not found or not accessible"
		}
		results = append(results, result)
	}

	// Always return results in 'files' array format for consistency
	return rc.Params{
		"files": results,
		"fs":    fs.ConfigString(vfs.Fs()),
	}, nil
}

func rcStatus(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}

	if vfs.cache == nil {
		counts := rc.Params{}
		for _, status := range vfscache.CacheStatuses {
			counts[status] = 0
		}
		return rc.Params{
			"totalFiles":             int64(0),
			"totalCachedBytes":       int64(0),
			"averageCachePercentage": int64(0),
			"counts":                 counts,
			"fs":                     fs.ConfigString(vfs.Fs()),
		}, nil
	}

	stats := vfs.cache.GetAggregateStats()
	counts := rc.Params{}
	for k, v := range stats.Counts {
		counts[k] = v
	}

	return rc.Params{
		"totalFiles":             stats.TotalFiles,
		"totalCachedBytes":       stats.TotalCachedBytes,
		"averageCachePercentage": stats.AverageCachePercentage,
		"counts":                 counts,
		"fs":                     fs.ConfigString(vfs.Fs()),
	}, nil
}
