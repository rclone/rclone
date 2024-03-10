package union

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/union/upstream"
	"github.com/rclone/rclone/fs"
)

// Object describes a union Object
//
// This is a wrapped object which returns the Union Fs as its parent
type Object struct {
	*upstream.Object
	fs          *Fs // what this object is part of
	co          []upstream.Entry
	writebackMu sync.Mutex
}

// Directory describes a union Directory
//
// This is a wrapped object contains all candidates
type Directory struct {
	*upstream.Directory
	fs *Fs // what this directory is part of
	cd []upstream.Entry
}

type entry interface {
	upstream.Entry
	candidates() []upstream.Entry
}

// Update o with the contents of newO excluding the lock
func (o *Object) update(newO *Object) {
	o.Object = newO.Object
	o.fs = newO.fs
	o.co = newO.co
}

// UnWrapUpstream returns the upstream Object that this Object is wrapping
func (o *Object) UnWrapUpstream() *upstream.Object {
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
		o.update(newO.(*Object))
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
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", o.UpstreamFs().Name(), err)
				if len(entries) > 1 {
					// Drain the input buffer to allow other uploads to continue
					_, _ = io.Copy(io.Discard, readers[i])
				}
			}
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
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", o.UpstreamFs().Name(), err)
			}
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
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", o.UpstreamFs().Name(), err)
			}
		} else {
			errs[i] = fs.ErrorNotAFile
		}
	})
	wg.Wait()
	return errs.Err()
}

// GetTier returns storage tier or class of the Object
func (o *Object) GetTier() string {
	do, ok := o.Object.Object.(fs.GetTierer)
	if !ok {
		return ""
	}
	return do.GetTier()
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	do, ok := o.Object.Object.(fs.IDer)
	if !ok {
		return ""
	}
	return do.ID()
}

// MimeType returns the content type of the Object if known
func (o *Object) MimeType(ctx context.Context) (mimeType string) {
	if do, ok := o.Object.Object.(fs.MimeTyper); ok {
		mimeType = do.MimeType(ctx)
	}
	return mimeType
}

// SetTier performs changing storage tier of the Object if
// multiple storage classes supported
func (o *Object) SetTier(tier string) error {
	do, ok := o.Object.Object.(fs.SetTierer)
	if !ok {
		return errors.New("underlying remote does not support SetTier")
	}
	return do.SetTier(tier)
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Need some sort of locking to prevent multiple downloads
	o.writebackMu.Lock()
	defer o.writebackMu.Unlock()

	// FIXME what if correct object is already in o.co

	newObj, err := o.Object.Writeback(ctx)
	if err != nil {
		return nil, err
	}
	if newObj != nil {
		o.Object = newObj
		o.co = append(o.co, newObj) // FIXME should this append or overwrite or update?
	}
	return o.Object.Object.Open(ctx, options...)
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

// SetMetadata sets metadata for an DirEntry
//
// It should return fs.ErrorNotImplemented if it can't set metadata
func (d *Directory) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	entries, err := d.fs.actionEntries(d.candidates()...)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	errs := Errors(make([]error, len(entries)))
	multithread(len(entries), func(i int) {
		if d, ok := entries[i].(*upstream.Directory); ok {
			err := d.SetMetadata(ctx, metadata)
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", d.UpstreamFs().Name(), err)
			}
		} else {
			errs[i] = fs.ErrorIsFile
		}
	})
	wg.Wait()
	return errs.Err()
}

// SetModTime sets the metadata on the DirEntry to set the modification date
//
// If there is any other metadata it does not overwrite it.
func (d *Directory) SetModTime(ctx context.Context, t time.Time) error {
	entries, err := d.fs.actionEntries(d.candidates()...)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	errs := Errors(make([]error, len(entries)))
	multithread(len(entries), func(i int) {
		if d, ok := entries[i].(*upstream.Directory); ok {
			err := d.SetModTime(ctx, t)
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", d.UpstreamFs().Name(), err)
			}
		} else {
			errs[i] = fs.ErrorIsFile
		}
	})
	wg.Wait()
	return errs.Err()
}

// Check the interfaces are satisfied
var (
	_ fs.FullObject    = (*Object)(nil)
	_ fs.FullDirectory = (*Directory)(nil)
)
