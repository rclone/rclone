// VFS remote control for the file system
//
// This is for integrating with the rclone rc system

package vfs

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs/vfscache"
)

// Adds the vfs/* commands
func init() {
	rc.Add(rc.Call{
		Path:  "vfs/status",
		Fn:    rcStatus,
		Title: "Get aggregate cache status statistics.",
		Help: `
This returns aggregate cache status statistics for the VFS.

This takes the following parameters:

- fs - select the VFS in use (optional)

This returns a JSON object with the following fields:

- totalFiles - total number of files in VFS
- fullCount - number of files with FULL cache status
- partialCount - number of files with PARTIAL cache status  
- noneCount - number of files with NONE cache status
- dirtyCount - number of files with DIRTY cache status
- uploadingCount - number of files with UPLOADING cache status
- totalCachedBytes - total bytes cached across all files
- averageCachePercentage - average cache percentage across all files
` + getVFSHelp,
	})

	rc.Add(rc.Call{
		Path:  "vfs/file-status",
		Fn:    rcFileStatus,
		Title: "Get detailed cache status of one or more files.",
		Help: `
This returns detailed cache status of files including name and percentage.

This takes the following parameters:

- fs - select the VFS in use (optional)
- file - the path to the file to get the status of (can be repeated as file1, file2, etc.)

This returns a JSON object with the following fields:

- files - array of file objects with fields:
  - name - leaf name of the file
  - status - one of "FULL", "PARTIAL", "NONE", "DIRTY", "UPLOADING"
  - percentage - percentage cached (0-100)
` + getVFSHelp,
	})

	rc.Add(rc.Call{
		Path:  "vfs/dir-status",
		Fn:    rcDirStatus,
		Title: "Get cache status of files in a directory.",
		Help: `
This returns cache status for all files in a specified directory, optionally including subdirectories. This is ideal for file manager integrations that need to display cache status overlays for directory listings.

This takes the following parameters:

- fs - select the VFS in use (optional)
- dir - the path to the directory to get the status of
- recursive - if true, include all subdirectories (optional, defaults to false)

This returns a JSON object with the following fields:

- dir - the directory path that was scanned
- files - object containing arrays of files grouped by their cache status:
  - FULL - array of completely cached files
  - PARTIAL - array of partially cached files  
  - NONE - array of files not cached
  - DIRTY - array of files modified locally but not uploaded
  - UPLOADING - array of files currently being uploaded
- Each file entry includes:
  - name - the file name
  - percentage - cache percentage (0-100)
  - uploading - whether the file is currently being uploaded
- recursive - whether subdirectories were included in the scan
- fs - the file system path

Example:
  rclone rc vfs/dir-status dir=/documents
  rclone rc vfs/dir-status dir=/documents recursive=true
` + getVFSHelp,
	})

	rc.Add(rc.Call{
		Path:  "vfs/refresh",
		Fn:    rcRefresh,
		Title: "Refresh the directory cache.",
		Help: `
This reads the directories for the specified paths and freshens the
directory cache.

If no paths are passed in then it will refresh the root directory.

    rclone rc vfs/refresh

Otherwise pass directories in as dir=path.  Any parameter key
starting with dir will refresh that directory, e.g.

    rclone rc vfs/refresh dir=home/junk dir2=data/misc

If the parameter recursive=true is given the whole directory tree
will get refreshed.  This refresh will use --fast-list if enabled.
` + getVFSHelp,
	})

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
	recursive, _ := in.GetBool("recursive")

	// Get root directory
	root, err := vfs.Root()
	if err != nil {
		return nil, err
	}

	// Navigate to the target directory
	targetDir := root
	if dirPath != "" {
		dirPath = strings.Trim(dirPath, "/")
		segments := strings.Split(dirPath, "/")
		var node Node = targetDir
		for _, s := range segments {
			if dir, ok := node.(*Dir); ok {
				node, err = dir.stat(s)
				if err != nil {
					return nil, fmt.Errorf("directory not found: %w", err)
				}
			} else {
				return nil, fmt.Errorf("path component is not a directory: %s", s)
			}
		}
		if dir, ok := node.(*Dir); ok {
			targetDir = dir
		} else {
			return nil, fmt.Errorf("target path is not a directory")
		}
	}

	// Collect status for each file
	filesByStatus := map[string][]rc.Params{
		"FULL":    {},
		"PARTIAL": {},
		"NONE":    {},
		"DIRTY":   {},
		"UPLOADING": {},
	}

	// Function to collect files from a directory
	var collectFiles func(dir *Dir, dirPath string) error
	collectFiles = func(dir *Dir, dirPath string) error {
		nodes, err := dir.ReadDirAll()
		if err != nil {
			return fmt.Errorf("failed to list directory contents: %w", err)
		}

		for _, node := range nodes {
			if file, ok := node.(*File); ok {
				var status string
				var percentage int64
				var isUploading bool
				if vfs.cache == nil {
					status = "NONE"
					percentage = 0
					isUploading = false
				} else {
					item := vfs.cache.Item(file.Path())
					status, percentage = item.VFSStatusCacheWithPercentage()
					
					// If status is UPLOADING, then the file is uploading
					isUploading = (status == "UPLOADING")
				}

				fileInfo := rc.Params{
					"name":       file.Name(),
					"percentage": percentage,
					"uploading":  isUploading,
				}

				// Add to the appropriate status category
				if files, exists := filesByStatus[status]; exists {
					filesByStatus[status] = append(files, fileInfo)
				} else {
				}
			} else if subDir, ok := node.(*Dir); ok {
				// If recursive is true, traverse subdirectories
				if recursive {
					if err := collectFiles(subDir, filepath.Join(dirPath, subDir.Name())); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}

	// Start collecting files from the target directory
	if err := collectFiles(targetDir, dirPath); err != nil {
		return nil, err
	}

	// Prepare the response, only include categories that have files
	responseFiles := rc.Params{}
	for status, files := range filesByStatus {
		if len(files) > 0 {
			responseFiles[status] = files
		}
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
		paths = []string{path}
	} else if !rc.IsErrParamNotFound(err) {
		return nil, err
	} else {
		// Check for multiple file parameters (file1, file2, etc.)
		for i := 1; ; i++ {
			key := "file" + strconv.Itoa(i)
			path, pathErr := in.GetString(key)
			if pathErr != nil {
				if rc.IsErrParamNotFound(pathErr) {
					break // No more file parameters
				}
				return nil, pathErr
			}
			paths = append(paths, path)
		}
		
		// If no files found, return error
		if len(paths) == 0 {
			return nil, errors.New("no file parameter(s) provided")
		}
	}
	
	// Collect status for each file
	var results []rc.Params
	for _, path := range paths {
		if vfs.cache == nil {
			results = append(results, rc.Params{
				"name":       filepath.Base(path),
				"status":     "NONE",
				"percentage": 0,
			})
		} else {
			item := vfs.cache.Item(path)
			status, percentage := item.VFSStatusCacheWithPercentage()
			results = append(results, rc.Params{
				"name":       filepath.Base(path),
				"status":     status,
				"percentage": percentage,
			})
		}
	}
	
	// Always return results in 'files' array format for consistency
	return rc.Params{
		"files": results,
	}, nil
}

func rcStatus(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}
	
	if vfs.cache == nil {
		return rc.Params{
			"totalFiles":           0,
			"fullCount":            0,
			"partialCount":         0,
			"noneCount":            0,
			"dirtyCount":           0,
			"uploadingCount":       0,
			"totalCachedBytes":     0,
			"averageCachePercentage": 0,
		}, nil
	}
	
	// Get aggregate statistics from cache
	stats := vfs.cache.GetAggregateStats()
	
	return rc.Params{
		"totalFiles":           stats.TotalFiles,
		"fullCount":            stats.FullCount,
		"partialCount":         stats.PartialCount,
		"noneCount":            stats.NoneCount,
		"dirtyCount":           stats.DirtyCount,
		"uploadingCount":       stats.UploadingCount,
		"totalCachedBytes":     stats.TotalCachedBytes,
		"averageCachePercentage": stats.AverageCachePercentage,
	}, nil
}

func rcRefresh(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return out, err
	}
	remote := vfs.Fs()
	features := remote.Features()
	if features.Refresh == nil {
		return out, fmt.Errorf("backend %s does not support refresh", remote.Name())
	}
	opts := fs.ListRHelper(in)

	// Get the paths to refresh
	var paths []string
	for key, value := range in {
		if strings.HasPrefix(key, "dir") || strings.HasPrefix(key, "file") {
			valueString, ok := value.(string)
			if !ok {
				continue
			}
			paths = append(paths, valueString)
		}
	}

	// If no paths passed in then use the root directory only
	if len(paths) == 0 {
		paths = []string{""}
	}

	// Refresh the paths
	for _, path := range paths {
		err = features.Refresh(ctx, path, opts)
		if err != nil {
			return out, fmt.Errorf("failed to refresh %q: %w", path, err)
		}
	}

	out = make(rc.Params)
	return out, nil
}

func rcForget(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}

	// Get the paths to forget
	var filePaths, dirPaths []string
	for key, value := range in {
		if strings.HasPrefix(key, "file") {
			valueString, ok := value.(string)
			if !ok {
				continue
			}
			filePaths = append(filePaths, valueString)
		} else if strings.HasPrefix(key, "dir") {
			valueString, ok := value.(string)
			if !ok {
				continue
			}
			dirPaths = append(dirPaths, valueString)
		}
	}

	// If no paths passed in then forget all files and dirs
	if len(filePaths) == 0 && len(dirPaths) == 0 {
		vfs.ForgetAll()
		fs.Debugf(nil, "All files and directories forgotten")
		return out, nil
	}

	// Forget the files
	for _, path := range filePaths {
		node, err := vfs.Stat(path)
		if err != nil {
			fs.Errorf(nil, "File not found %q: %v", path, err)
			continue
		}
		file, ok := node.(*File)
		if !ok {
			fs.Errorf(nil, "Path is not a file %q", path)
			continue
		}
		file.Forget()
		fs.Debugf(nil, "File forgotten %q", path)
	}

	// Forget the dirs
	for _, path := range dirPaths {
		node, err := vfs.Stat(path)
		if err != nil {
			fs.Errorf(nil, "Directory not found %q: %v", path, err)
			continue
		}
		dir, ok := node.(*Dir)
		if !ok {
			fs.Errorf(nil, "Path is not a directory %q", path)
			continue
		}
		dir.ForgetAll()
		fs.Debugf(nil, "Directory forgotten %q", path)
	}

	return out, nil
}

func rcPollInterval(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}

	intervalStr, err := in.GetString("interval")
	if rc.IsErrParamNotFound(err) {
		// No interval parameter - just return the current status
		return getStatus(vfs, in)
	} else if err != nil {
		return nil, err
	}

	timeoutStr, _ := in.GetString("timeout")

	interval, err := fs.ParseDuration(intervalStr)
	if err != nil {
		return nil, err
	}

	var timeout time.Duration
	if timeoutStr != "" {
		timeout, err = fs.ParseDuration(timeoutStr)
		if err != nil {
			return nil, err
		}
	}

	err = vfs.SetPollInterval(interval, timeout)
	if err != nil {
		return nil, err
	}

	return getStatus(vfs, in)
}

func getStatus(vfs *VFS, in rc.Params) (out rc.Params, err error) {
	pollInterval := vfs.Opt.PollInterval
	isExternal := vfs.IsExternal()

	return rc.Params{
		"enabled":   !pollInterval.IsZero(),
		"interval":  pollInterval,
		"external":  isExternal,
	}, nil
}

func rcList(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	activeMu.Lock()
	defer activeMu.Unlock()

	var vfses []string
	for _, vfs := range active {
		vfses = append(vfses, fs.ConfigString(vfs.Fs()))
	}

	return rc.Params{
		"vfses": vfses,
	}, nil
}

func rcStats(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}
	return vfs.Stats(), nil
}

func rcQueue(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}
	if vfs.cache == nil {
		return rc.Params{}, nil
	}
	return vfs.cache.Queue(), nil
}

func rcQueueSetExpiry(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	vfs, err := getVFS(in)
	if err != nil {
		return nil, err
	}
	if vfs.cache == nil {
		return nil, errors.New("VFS cache not enabled")
	}

	id, err := in.GetInt64("id")
	if err != nil {
		return nil, err
	}

	expiry, err := in.GetFloat64("expiry")
	if err != nil {
		return nil, err
	}

	relative := false
	relative, err := in.GetBool("relative")
	if err != nil && !rc.IsErrParamNotFound(err) {
		return nil, err
	}

	err = vfs.cache.SetExpiry(id, expiry, relative)
	if err != nil {
		return nil, err
	}

	return out, nil
}