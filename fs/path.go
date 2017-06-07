package fs

import (
	"path"
	"strings"
)

// RemoteSplit splits a remote into a parent and a leaf
//
// if it returns leaf as an empty string then remote is a directory
//
// if it returns parent as an empty string then that means the current directory
//
// The returned values have the property that parent + leaf == remote
func RemoteSplit(remote string) (parent string, leaf string) {
	// Split remote on :
	i := strings.Index(remote, ":")
	remoteName := ""
	remotePath := remote
	if i >= 0 {
		remoteName = remote[:i+1]
		remotePath = remote[i+1:]
	} else if strings.HasSuffix(remotePath, "/") {
		// if no : and ends with / must be directory
		return remotePath, ""
	}
	// Construct new remote name without last segment
	parent, leaf = path.Split(remotePath)
	return remoteName + parent, leaf
}
