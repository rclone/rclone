//go:build !openbsd && !plan9
// +build !openbsd,!plan9

package local

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/pkg/xattr"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/file"
)

var (
	xattrPrefix       = "user." // FIXME is this correct for all unixes?
	appledoubleKey    = "rcapple2"
	appledoublePrefix = "._"
	xattrSupported    = xattr.XATTR_SUPPORTED
	tempdir           = setTempdir()
)

// doing it this way to avoid cgo dependency on non-darwin (darwin already has some)
type appleDoubleFns interface {
	packAppleDouble(src, dst string) error
	unpackAppleDouble(src, dst string, deleteAppleDouble bool) error
}

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
	if o.fs.opt.AppleDouble && runtime.GOOS == "darwin" {
		tempfile := setTempfile(o.remote)
		fs.Debugf("packing", "%s to %s", o.path, tempfile)
		var fns interface{} = o
		do, ok := fns.(appleDoubleFns)
		if !ok {
			return metadata, fs.ErrorNotImplemented
		}
		err = do.packAppleDouble(o.path, tempfile)
		if err != nil {
			return metadata, err
		}
		appledoubleBytes, err := os.ReadFile(tempfile)
		if err != nil {
			fs.Errorf(o, "error reading appledouble temp file: %v", err)
			return metadata, err
		}
		if o.fs.opt.MetadataMaxLength > 0 && len(string(appledoubleBytes)) > o.fs.opt.MetadataMaxLength {
			fs.Debugf(o, "skipping appledouble metadata as length (%d) is greater than max length (%d)", len(string(appledoubleBytes)), o.fs.opt.MetadataMaxLength)
		} else {
			metadata[appledoubleKey] = string(appledoubleBytes)
		}
		defer func() { _ = os.Remove(tempfile) }()
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
		if o.fs.opt.AppleDouble && runtime.GOOS == "darwin" && k == xattrPrefix+appledoubleKey {
			tempfile := setTempfile(o.remote)
			err = os.WriteFile(tempfile, v, 0600)
			if err != nil {
				return err
			}
			fs.Debugf("unpacking", "%s to %s", tempfile, o.path)
			var fns interface{} = o
			do, ok := fns.(appleDoubleFns)
			if !ok {
				return fs.ErrorNotImplemented
			}
			err = do.unpackAppleDouble(tempfile, o.path, true)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func setTempdir() string {
	dirPath := filepath.Join(os.TempDir(), appledoubleKey)
	err := file.MkdirAll(dirPath, 0777)
	if err != nil {
		fs.Errorf(dirPath, "error creating temp dir: %v", err)
		return ""
	}
	dir, err := os.MkdirTemp(dirPath, appledoubleKey+"-")
	if err != nil {
		fs.Errorf(dir, "error creating temp dir: %v", err)
		return ""
	}
	return dir
}

func setTempfile(remote string) string {
	prefixedRemote := filepath.Join(filepath.Dir(remote), appledoublePrefix+filepath.Base(remote))
	dst := filepath.Join(tempdir, prefixedRemote)
	err := file.MkdirAll(filepath.Dir(dst), 0777)
	if err != nil {
		fs.Errorf(dst, "error creating temp dir: %v", err)
		return ""
	}
	return dst
}
