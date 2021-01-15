package union

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

// Object describes a union Object
//
// This is a wrapped object which returns the Union Fs as its parent
type Object struct {
	*upstream.Object
	fs *Fs // what this object is part of
	co []upstream.Entry
}

// Directory describes a union Directory
//
// This is a wrapped object contains all candidates
type Directory struct {
	*upstream.Directory
	cd []upstream.Entry
}

type entry interface {
	upstream.Entry
	candidates() []upstream.Entry
}

// UnWrap returns the Object that this Object is wrapping or
// nil if it isn't wrapping anything
func (o *Object) UnWrap() *upstream.Object {
	return o.Object
}

// Fs returns the union Fs as the parent
func (o *Object) Fs() fs.Info {
	return o.fs
}

func (o *Object) candidates() []upstream.Entry {
	return o.co
}

func (d *Directory) candidates() []upstream.Entry {
	return d.cd
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	entries, err := o.fs.actionEntries(o.candidates()...)
	if err == fs.ErrorPermissionDenied {
		// There are no candidates in this object which can be written to
		// So attempt to create a new object instead
		newO, err := o.fs.put(ctx, in, src, false, options...)
		if err != nil {
			return err
		}
		// Update current object
		*o = *newO.(*Object)
		return nil
	} else if err != nil {
		return err
	}
	if len(entries) == 1 {
		obj := entries[0].(*upstream.Object)
		return obj.Update(ctx, in, src, options...)
	}
	// Multi-threading
	readers, errChan := multiReader(len(entries), in)
	errs := Errors(make([]error, len(entries)+1))
	multithread(len(entries), func(i int) {
		if o, ok := entries[i].(*upstream.Object); ok {
			err := o.Update(ctx, readers[i], src, options...)
			errs[i] = errors.Wrap(err, o.UpstreamFs().Name())
		} else {
			errs[i] = fs.ErrorNotAFile
		}
	})
	errs[len(entries)] = <-errChan
	return errs.Err()
}

// Remove candidate objects selected by ACTION policy
func (o *Object) Remove(ctx context.Context) error {
	entries, err := o.fs.actionEntries(o.candidates()...)
	if err != nil {
		return err
	}
	errs := Errors(make([]error, len(entries)))
	multithread(len(entries), func(i int) {
		if o, ok := entries[i].(*upstream.Object); ok {
			err := o.Remove(ctx)
			errs[i] = errors.Wrap(err, o.UpstreamFs().Name())
		} else {
			errs[i] = fs.ErrorNotAFile
		}
	})
	return errs.Err()
}

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	entries, err := o.fs.actionEntries(o.candidates()...)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	errs := Errors(make([]error, len(entries)))
	multithread(len(entries), func(i int) {
		if o, ok := entries[i].(*upstream.Object); ok {
			err := o.SetModTime(ctx, t)
			errs[i] = errors.Wrap(err, o.UpstreamFs().Name())
		} else {
			errs[i] = fs.ErrorNotAFile
		}
	})
	wg.Wait()
	return errs.Err()
}

// ModTime returns the modification date of the directory
// It returns the latest ModTime of all candidates
func (d *Directory) ModTime(ctx context.Context) (t time.Time) {
	entries := d.candidates()
	times := make([]time.Time, len(entries))
	multithread(len(entries), func(i int) {
		times[i] = entries[i].ModTime(ctx)
	})
	for _, ti := range times {
		if t.Before(ti) {
			t = ti
		}
	}
	return t
}

// Size returns the size of the directory
// It returns the sum of all candidates
func (d *Directory) Size() (s int64) {
	for _, e := range d.candidates() {
		s += e.Size()
	}
	return s
}
