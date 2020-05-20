// Utilities for accessing the Fs cache

package rc

import (
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
)

// GetFsNamed gets an fs.Fs named fsName either from the cache or creates it afresh
func GetFsNamed(in Params, fsName string) (f fs.Fs, err error) {
	fsString, err := in.GetString(fsName)
	if err != nil {
		return nil, err
	}

	return cache.Get(fsString)
}

// GetFs gets an fs.Fs named "fs" either from the cache or creates it afresh
func GetFs(in Params) (f fs.Fs, err error) {
	return GetFsNamed(in, "fs")
}

// GetFsAndRemoteNamed gets the fsName parameter from in, makes a
// remote or fetches it from the cache then gets the remoteName
// parameter from in too.
func GetFsAndRemoteNamed(in Params, fsName, remoteName string) (f fs.Fs, remote string, err error) {
	remote, err = in.GetString(remoteName)
	if err != nil {
		return
	}
	f, err = GetFsNamed(in, fsName)
	return

}

// GetFsAndRemote gets the `fs` parameter from in, makes a remote or
// fetches it from the cache then gets the `remote` parameter from in
// too.
func GetFsAndRemote(in Params) (f fs.Fs, remote string, err error) {
	return GetFsAndRemoteNamed(in, "fs", "remote")
}
