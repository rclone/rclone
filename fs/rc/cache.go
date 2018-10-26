// This implements the Fs cache

package rc

import (
	"sync"

	"github.com/ncw/rclone/fs"
)

var (
	fsCacheMu sync.Mutex
	fsCache   = map[string]fs.Fs{}
	fsNewFs   = fs.NewFs // for tests
)

// GetFsNamed gets a fs.Fs named fsName either from the cache or creates it afresh
func GetFsNamed(in Params, fsName string) (f fs.Fs, err error) {
	fsCacheMu.Lock()
	defer fsCacheMu.Unlock()

	fsString, err := in.GetString(fsName)
	if err != nil {
		return nil, err
	}

	f = fsCache[fsString]
	if f == nil {
		f, err = fsNewFs(fsString)
		if err == nil {
			fsCache[fsString] = f
		}
	}
	return f, err
}

// GetFs gets a fs.Fs named "fs" either from the cache or creates it afresh
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
