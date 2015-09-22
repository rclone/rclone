package fs

import (
	"fmt"
	"io"
	"time"
)

// Limited defines a Fs which can only return the Objects passed in
// from the Fs passed in
type Limited struct {
	objects []Object
	fs      Fs
}

// NewLimited maks a limited Fs limited to the objects passed in
func NewLimited(fs Fs, objects ...Object) Fs {
	f := &Limited{
		objects: objects,
		fs:      fs,
	}
	return f
}

// Name is name of the remote (as passed into NewFs)
func (f *Limited) Name() string {
	return f.fs.Name() // return name of underlying remote
}

// Root is the root of the remote (as passed into NewFs)
func (f *Limited) Root() string {
	return f.fs.Root() // return root of underlying remote
}

// String returns a description of the FS
func (f *Limited) String() string {
	return fmt.Sprintf("%s limited to %d objects", f.fs.String(), len(f.objects))
}

// List the Fs into a channel
func (f *Limited) List() ObjectsChan {
	out := make(ObjectsChan, Config.Checkers)
	go func() {
		for _, obj := range f.objects {
			out <- obj
		}
		close(out)
	}()
	return out
}

// ListDir lists the Fs directories/buckets/containers into a channel
func (f *Limited) ListDir() DirChan {
	out := make(DirChan, Config.Checkers)
	close(out)
	return out
}

// NewFsObject finds the Object at remote.  Returns nil if can't be found
func (f *Limited) NewFsObject(remote string) Object {
	for _, obj := range f.objects {
		if obj.Remote() == remote {
			return obj
		}
	}
	return nil
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Limited) Put(in io.Reader, remote string, modTime time.Time, size int64) (Object, error) {
	obj := f.NewFsObject(remote)
	if obj == nil {
		return nil, fmt.Errorf("Can't create %q in limited fs", remote)
	}
	return obj, obj.Update(in, modTime, size)
}

// Mkdir make the directory (container, bucket)
func (f *Limited) Mkdir() error {
	// All directories are already made - just ignore
	return nil
}

// Rmdir removes the directory (container, bucket) if empty
func (f *Limited) Rmdir() error {
	// Ignore this in a limited fs
	return nil
}

// Precision of the ModTimes in this Fs
func (f *Limited) Precision() time.Duration {
	return f.fs.Precision()
}

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Limited) Copy(src Object, remote string) (Object, error) {
	fCopy, ok := f.fs.(Copier)
	if !ok {
		return nil, ErrorCantCopy
	}
	return fCopy.Copy(src, remote)
}

// Check the interfaces are satisfied
var _ Fs = &Limited{}
var _ Copier = &Limited{}
