//go:build darwin && cgo

// Package local provides a filesystem interface
package local

import (
	"context"
	"fmt"
	"runtime"

	"github.com/go-darwin/apfs"
	"github.com/rclone/rclone/fs"
)

// Copy src to this remote using server-side copy operations.
//
// # This is stored with the remote path given
//
// # It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	if runtime.GOOS != "darwin" || f.opt.TranslateSymlinks || f.opt.NoClone {
		return nil, fs.ErrorCantCopy
	}
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't clone - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	// Fetch metadata if --metadata is in use
	meta, err := fs.GetMetadataOptions(ctx, f, src, fs.MetadataAsOpenOptions(ctx))
	if err != nil {
		return nil, fmt.Errorf("copy: failed to read metadata: %w", err)
	}

	// Create destination
	dstObj := f.newObject(remote)
	err = dstObj.mkdirAll()
	if err != nil {
		return nil, err
	}

	err = Clone(srcObj.path, f.localPath(remote))
	if err != nil {
		return nil, err
	}
	fs.Debugf(remote, "server-side cloned!")

	// Set metadata if --metadata is in use
	if meta != nil {
		err = dstObj.writeMetadata(meta)
		if err != nil {
			return nil, fmt.Errorf("copy: failed to set metadata: %w", err)
		}
	}

	return f.NewObject(ctx, remote)
}

// Clone uses APFS cloning if possible, otherwise falls back to copying (with full metadata preservation)
// note that this is closely related to unix.Clonefile(src, dst, unix.CLONE_NOFOLLOW) but not 100% identical
// https://opensource.apple.com/source/copyfile/copyfile-173.40.2/copyfile.c.auto.html
func Clone(src, dst string) error {
	state := apfs.CopyFileStateAlloc()
	defer func() {
		if err := apfs.CopyFileStateFree(state); err != nil {
			fs.Errorf(dst, "free state error: %v", err)
		}
	}()
	cloned, err := apfs.CopyFile(src, dst, state, apfs.COPYFILE_CLONE)
	fs.Debugf(dst, "isCloned: %v, error: %v", cloned, err)
	return err
}

// Check the interfaces are satisfied
var (
	_ fs.Copier = &Fs{}
)
