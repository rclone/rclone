//go:build !openbsd && !plan9

package local

import (
	"fmt"
	"os"
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
	// Return nil if all xattrs were filtered out (e.g. com.apple.provenance on macOS)
	if len(metadata) == 0 {
		return nil, nil
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
	// When translating symlinks (-l/--links) write the xattrs through a
	// descriptor opened under os.Root so a symlink planted in the destination
	// can't redirect the write outside it (CWE-59). A translated link is
	// itself the leaf, so it keeps the non-following path based write.
	var fd *os.File
	for k, value := range metadata {
		k = strings.ToLower(k)
		if _, found := systemMetadataInfo[k]; found {
			continue
		}
		k = xattrPrefix + k
		v := []byte(value)
		switch {
		case o.fs.opt.TranslateSymlinks && !o.translatedLink:
			if fd == nil {
				fd, err = o.fs.openFile(o.path, os.O_RDONLY, 0)
				if os.IsPermission(err) {
					// A write only file can't be opened for reading but its
					// owner may still set xattrs on it, so try opening it for
					// writing instead (which can't succeed for a directory).
					if wfd, werr := o.fs.openFile(o.path, os.O_WRONLY, 0); werr == nil {
						fd, err = wfd, nil
					}
				}
				if err != nil {
					return fmt.Errorf("failed to open for xattr: %w", err)
				}
				defer fs.CheckClose(fd, &err)
			}
			err = xattr.FSet(fd, k, v)
		case o.fs.opt.FollowSymlinks:
			err = xattr.Set(o.path, k, v)
		default:
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
