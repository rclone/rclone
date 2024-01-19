// Utilities for accessing the Fs cache

package rc

import (
	"context"
	"errors"
	"fmt"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fspath"
)

// getFsName gets an fs name from fsName either from the cache or direct
func getFsName(in Params, fsName string) (fsString string, err error) {
	fsString, err = in.GetString(fsName)
	if err != nil {
		if !IsErrParamInvalid(err) {
			return fsString, err
		}
		fsString, err = getConfigMap(in, fsName)
		if err != nil {
			return fsString, err
		}
	}
	return fsString, err
}

// GetFsNamed gets an fs.Fs named fsName either from the cache or creates it afresh
func GetFsNamed(ctx context.Context, in Params, fsName string) (f fs.Fs, err error) {
	fsString, err := getFsName(in, fsName)
	if err != nil {
		return nil, err
	}
	return cache.Get(ctx, fsString)
}

// GetFsNamedFileOK gets an fs.Fs named fsName either from the cache or creates it afresh
//
// If the fs.Fs points to a single file then it returns a new ctx with
// filters applied to make the listings return only that file.
func GetFsNamedFileOK(ctx context.Context, in Params, fsName string) (newCtx context.Context, f fs.Fs, err error) {
	fsString, err := getFsName(in, fsName)
	if err != nil {
		return ctx, nil, err
	}
	f, err = cache.Get(ctx, fsString)
	if err == nil {
		return ctx, f, nil
	} else if !errors.Is(err, fs.ErrorIsFile) {
		return ctx, nil, err
	}
	// f points to the directory above the file so find the remote name
	_, fileName, err := fspath.Split(fsString)
	if err != nil {
		return ctx, f, err
	}
	ctx, fi := filter.AddConfig(ctx)
	if !fi.InActive() {
		return ctx, f, fmt.Errorf("can't limit to single files when using filters: %q", fileName)
	}
	// Limit transfers to this file
	err = fi.AddFile(fileName)
	if err != nil {
		return ctx, f, fmt.Errorf("failed to limit to single file: %w", err)
	}
	return ctx, f, nil
}

// getConfigMap gets the config as a map from in and converts it to a
// config string
//
// It uses the special parameters _name to name the remote and _root
// to make the root of the remote.
func getConfigMap(in Params, fsName string) (fsString string, err error) {
	var m configmap.Simple
	err = in.GetStruct(fsName, &m)
	if err != nil {
		return fsString, err
	}
	pop := func(key string) string {
		value := m[key]
		delete(m, key)
		return value
	}
	Type := pop("type")
	name := pop("_name")
	root := pop("_root")
	if name != "" {
		fsString = name
	} else if Type != "" {
		fsString = ":" + Type
	} else {
		return fsString, errors.New(`couldn't find "type" or "_name" in JSON config definition`)
	}
	config := m.String()
	if config != "" {
		fsString += ","
		fsString += config
	}
	fsString += ":"
	fsString += root
	return fsString, nil
}

// GetFs gets an fs.Fs named "fs" either from the cache or creates it afresh
func GetFs(ctx context.Context, in Params) (f fs.Fs, err error) {
	return GetFsNamed(ctx, in, "fs")
}

// GetFsAndRemoteNamed gets the fsName parameter from in, makes a
// remote or fetches it from the cache then gets the remoteName
// parameter from in too.
func GetFsAndRemoteNamed(ctx context.Context, in Params, fsName, remoteName string) (f fs.Fs, remote string, err error) {
	remote, err = in.GetString(remoteName)
	if err != nil {
		return
	}
	f, err = GetFsNamed(ctx, in, fsName)
	return

}

// GetFsAndRemote gets the `fs` parameter from in, makes a remote or
// fetches it from the cache then gets the `remote` parameter from in
// too.
func GetFsAndRemote(ctx context.Context, in Params) (f fs.Fs, remote string, err error) {
	return GetFsAndRemoteNamed(ctx, in, "fs", "remote")
}

func init() {
	Add(Call{
		Path:         "fscache/clear",
		Fn:           rcCacheClear,
		Title:        "Clear the Fs cache.",
		AuthRequired: true,
		Help: `
This clears the fs cache. This is where remotes created from backends
are cached for a short while to make repeated rc calls more efficient.

If you change the parameters of a backend then you may want to call
this to clear an existing remote out of the cache before re-creating
it.
`,
	})
}

// Clear the fs cache
func rcCacheClear(ctx context.Context, in Params) (out Params, err error) {
	cache.Clear()
	return nil, nil
}

func init() {
	Add(Call{
		Path:         "fscache/entries",
		Fn:           rcCacheEntries,
		Title:        "Returns the number of entries in the fs cache.",
		AuthRequired: true,
		Help: `
This returns the number of entries in the fs cache.

Returns
- entries - number of items in the cache
`,
	})
}

// Return the Entries the fs cache
func rcCacheEntries(ctx context.Context, in Params) (out Params, err error) {
	return Params{
		"entries": cache.Entries(),
	}, nil
}
