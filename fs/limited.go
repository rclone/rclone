package fs

import (
	"fmt"
	"io"
	"time"
)

// This defines a Limited Fs which can only return the Objects passed in from the Fs passed in
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

// List the Fs directories/buckets/containers into a channel
func (f *Limited) ListDir() DirChan {
	out := make(DirChan, Config.Checkers)
	close(out)
	return out
}

// Find the Object at remote.  Returns nil if can't be found
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

// Make the directory (container, bucket)
func (f *Limited) Mkdir() error {
	// All directories are already made - just ignore
	return nil
}

// Remove the directory (container, bucket) if empty
func (f *Limited) Rmdir() error {
	return fmt.Errorf("Can't rmdir in limited fs")
}

// Precision of the ModTimes in this Fs
func (f *Limited) Precision() time.Duration {
	return f.fs.Precision()
}

// Check the interfaces are satisfied
var _ Fs = &Limited{}
