//go:build !plan9

package hdfs

import (
	"context"
	"errors"
	"io"
	"path"
	"time"

	"github.com/colinmarc/hdfs/v2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/readers"
)

// Object describes an HDFS file
type Object struct {
	fs      *Fs
	remote  string
	size    int64
	modTime time.Time
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time of the object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	realpath := o.fs.realpath(o.Remote())
	err := o.fs.client.Chtimes(realpath, modTime, modTime)
	if err != nil {
		fs.Errorf(o, "SetModTime: ChTimes(%q, %v, %v) returned error: %v", realpath, modTime, modTime, err)
		return err
	}
	o.modTime = modTime
	return nil
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.Remote()
}

// Hash is not supported
func (o *Object) Hash(ctx context.Context, r hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	realpath := o.realpath()
	fs.Debugf(o.fs, "open [%s]", realpath)
	f, err := o.fs.client.Open(realpath)
	if err != nil {
		return nil, err
	}

	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.Size())
		}
	}

	_, err = f.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	if limit != -1 {
		in = readers.NewLimitedReadCloser(f, limit)
	} else {
		in = f
	}

	return in, err
}

// Update object
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	realpath := o.fs.realpath(o.remote)
	dirname := path.Dir(realpath)
	fs.Debugf(o.fs, "update [%s]", realpath)

	err := o.fs.client.MkdirAll(dirname, 0755)
	if err != nil {
		fs.Errorf(o, "update: MkdirAll(%q, 0755) returned error: %v", dirname, err)
		return err
	}

	_, err = o.fs.client.Stat(realpath)
	if err == nil {
		fs.Errorf(o, "update: Stat(%q) returned error: %v", realpath, err)
		err = o.fs.client.Remove(realpath)
		if err != nil {
			fs.Errorf(o, "update: Remove(%q) returned error: %v", realpath, err)
			return err
		}
	}

	out, err := o.fs.client.Create(realpath)
	if err != nil {
		fs.Errorf(o, "update: Create(%q) returned error: %v", realpath, err)
		return err
	}

	cleanup := func() {
		rerr := o.fs.client.Remove(realpath)
		if rerr != nil {
			fs.Errorf(o, "update: cleanup: Remove(%q) returned error: %v", realpath, err)
			fs.Errorf(o.fs, "failed to remove [%v]: %v", realpath, rerr)
		}
	}

	_, err = io.Copy(out, in)
	if err != nil {
		fs.Errorf(o, "update: io.Copy returned error: %v", err)
		cleanup()
		return err
	}

	// If the datanodes have acknowledged all writes but not yet
	// to the namenode, FileWriter.Close can return ErrReplicating
	// (wrapped in an os.PathError). This indicates that all data
	// has been written, but the lease is still open for the file.
	//
	// It is safe in this case to either ignore the error (and let
	// the lease expire on its own) or to call Close multiple
	// times until it completes without an error. The Java client,
	// for context, always chooses to retry, with exponential
	// backoff.
	err = o.fs.pacer.Call(func() (bool, error) {
		err := out.Close()
		if err == nil {
			return false, nil
		}
		return errors.Is(err, hdfs.ErrReplicating), err
	})
	if err != nil {
		fs.Errorf(o, "update: Close(%#v) returned error: %v", out, err)
		cleanup()
		return err
	}

	info, err := o.fs.client.Stat(realpath)
	if err != nil {
		fs.Errorf(o, "update: Stat#2(%q) returned error: %v", realpath, err)
		return err
	}

	err = o.SetModTime(ctx, src.ModTime(ctx))
	if err != nil {
		fs.Errorf(o, "update: SetModTime(%v) returned error: %v", src.ModTime(ctx), err)
		return err
	}
	o.size = info.Size()

	fs.Errorf(o, "update: returned no error")
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	realpath := o.fs.realpath(o.remote)
	fs.Debugf(o.fs, "remove [%s]", realpath)
	return o.fs.client.Remove(realpath)
}

func (o *Object) realpath() string {
	return o.fs.opt.Enc.FromStandardPath(xPath(o.Fs().Root(), o.remote))
}

// Check the interfaces are satisfied
var (
	_ fs.Object = (*Object)(nil)
)
