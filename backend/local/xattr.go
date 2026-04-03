//go:build !openbsd && !plan9

package local

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/pkg/xattr"
	"github.com/rclone/rclone/fs"
)

const (
	xattrPrefix    = "user." // FIXME is this correct for all unixes?
	xattrSupported = xattr.XATTR_SUPPORTED
)

// Check to see if the error supplied is a not supported error, and if
// so, disable xattrs
func (f *Fs) xattrIsNotSupported(err error) bool {
	xattrErr, ok := err.(*xattr.Error)
	if !ok {
		return false
	}
	// Xattrs not supported can be ENOTSUP or ENOATTR or EINVAL (on Solaris)
	if xattrErr.Err == syscall.EINVAL || xattrErr.Err == syscall.ENOTSUP || xattrErr.Err == xattr.ENOATTR {
		// Show xattrs not supported
		if f.xattrSupported.CompareAndSwap(1, 0) {
			fs.Errorf(f, "xattrs not supported - disabling: %v", err)
		}
		return true
	}
	return false
}

// getXattr returns the extended attributes for an object
//
// It doesn't return any attributes owned by this backend in
// metadataKeys
func (o *Object) getXattr() (metadata fs.Metadata, err error) {
	if !xattrSupported || o.fs.xattrSupported.Load() == 0 {
		return nil, nil
	}
	var list []string
	if o.fs.opt.FollowSymlinks {
		list, err = xattr.List(o.path)
	} else {
		list, err = xattr.LList(o.path)
	}
	if err != nil {
		if o.fs.xattrIsNotSupported(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read xattr: %w", err)
	}
	if len(list) == 0 {
		return nil, nil
	}
	metadata = make(fs.Metadata, len(list))
	for _, k := range list {
		var v []byte
		if o.fs.opt.FollowSymlinks {
			v, err = xattr.Get(o.path, k)
		} else {
			v, err = xattr.LGet(o.path, k)
		}
		if err != nil {
			if o.fs.xattrIsNotSupported(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to read xattr key %q: %w", k, err)
		}
		k = strings.ToLower(k)
		if !strings.HasPrefix(k, xattrPrefix) {
			continue
		}
		k = k[len(xattrPrefix):]
		if _, found := systemMetadataInfo[k]; found {
			continue
		}
		metadata[k] = string(v)
	}
	return metadata, nil
}

// setXattr sets the metadata on the file Xattrs
//
// It doesn't set any attributes owned by this backend in metadataKeys
func (o *Object) setXattr(metadata fs.Metadata) (err error) {
	if !xattrSupported || o.fs.xattrSupported.Load() == 0 {
		return nil
	}
	for k, value := range metadata {
		k = strings.ToLower(k)
		if _, found := systemMetadataInfo[k]; found {
			continue
		}
		k = xattrPrefix + k
		v := []byte(value)
		if o.fs.opt.FollowSymlinks {
			err = xattr.Set(o.path, k, v)
		} else {
			err = xattr.LSet(o.path, k, v)
		}
		if err != nil {
			if o.fs.xattrIsNotSupported(err) {
				return nil
			}
			return fmt.Errorf("failed to set xattr key %q: %w", k, err)
		}
	}
	return nil
}
