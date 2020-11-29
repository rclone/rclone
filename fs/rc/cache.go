// Utilities for accessing the Fs cache

package rc

import (
	"context"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
)

// GetFsNamed gets an fs.Fs named fsName either from the cache or creates it afresh
func GetFsNamed(ctx context.Context, in Params, fsName string) (f fs.Fs, err error) {
	fsString, err := in.GetString(fsName)
	if err != nil {
		return nil, err
	}

	return cache.Get(ctx, fsString)
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
