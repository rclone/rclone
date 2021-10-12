package hasher

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
)

// obtain hash for an object
func (o *Object) getHash(ctx context.Context, hashType hash.Type) (string, error) {
	maxAge := time.Duration(o.f.opt.MaxAge)
	if maxAge <= 0 {
		return "", nil
	}
	fp := o.fingerprint(ctx)
	if fp == "" {
		return "", errors.New("fingerprint failed")
	}
	return o.f.getRawHash(ctx, hashType, o.Remote(), fp, maxAge)
}

// obtain hash for a path
func (f *Fs) getRawHash(ctx context.Context, hashType hash.Type, remote, fp string, age time.Duration) (string, error) {
	key := path.Join(f.Fs.Root(), remote)
	op := &kvGet{
		key:  key,
		fp:   fp,
		hash: hashType.String(),
		age:  age,
	}
	err := f.db.Do(false, op)
	return op.val, err
}

// put new hashes for an object
func (o *Object) putHashes(ctx context.Context, rawHashes hashMap) error {
	if o.f.opt.MaxAge <= 0 {
		return nil
	}
	fp := o.fingerprint(ctx)
	if fp == "" {
		return nil
	}
	key := path.Join(o.f.Fs.Root(), o.Remote())
	hashes := operations.HashSums{}
	for hashType, hashVal := range rawHashes {
		hashes[hashType.String()] = hashVal
	}
	return o.f.putRawHashes(ctx, key, fp, hashes)
}

// set hashes for a path without any validation
func (f *Fs) putRawHashes(ctx context.Context, key, fp string, hashes operations.HashSums) error {
	return f.db.Do(true, &kvPut{
		key:    key,
		fp:     fp,
		hashes: hashes,
		age:    time.Duration(f.opt.MaxAge),
	})
}

// Hash returns the selected checksum of the file or "" if unavailable.
func (o *Object) Hash(ctx context.Context, hashType hash.Type) (hashVal string, err error) {
	f := o.f
	if f.passHashes.Contains(hashType) {
		fs.Debugf(o, "pass %s", hashType)
		return o.Object.Hash(ctx, hashType)
	}
	if !f.suppHashes.Contains(hashType) {
		fs.Debugf(o, "unsupp %s", hashType)
		return "", hash.ErrUnsupported
	}
	if hashVal, err = o.getHash(ctx, hashType); err != nil {
		fs.Debugf(o, "getHash: %v", err)
		err = nil
		hashVal = ""
	}
	if hashVal != "" {
		fs.Debugf(o, "cached %s = %q", hashType, hashVal)
		return hashVal, nil
	}
	if f.slowHashes.Contains(hashType) {
		fs.Debugf(o, "slow %s", hashType)
		hashVal, err = o.Object.Hash(ctx, hashType)
		if err == nil && hashVal != "" && f.keepHashes.Contains(hashType) {
			if err = o.putHashes(ctx, hashMap{hashType: hashVal}); err != nil {
				fs.Debugf(o, "putHashes: %v", err)
				err = nil
			}
		}
		return hashVal, err
	}
	if f.autoHashes.Contains(hashType) && o.Size() < int64(f.opt.AutoSize) {
		_ = o.updateHashes(ctx)
		if hashVal, err = o.getHash(ctx, hashType); err != nil {
			fs.Debugf(o, "auto %s = %q (%v)", hashType, hashVal, err)
			err = nil
		}
	}
	return hashVal, err
}

// updateHashes performs implicit "rclone hashsum --download" and updates cache.
func (o *Object) updateHashes(ctx context.Context) error {
	r, err := o.Open(ctx)
	if err != nil {
		fs.Infof(o, "update failed (open): %v", err)
		return err
	}
	defer func() {
		_ = r.Close()
	}()
	if _, err = io.Copy(ioutil.Discard, r); err != nil {
		fs.Infof(o, "update failed (copy): %v", err)
		return err
	}
	return nil
}

// Update the object with the given data, time and size.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	_ = o.f.pruneHash(src.Remote())
	return o.Object.Update(ctx, in, src, options...)
}

// Remove an object.
func (o *Object) Remove(ctx context.Context) error {
	_ = o.f.pruneHash(o.Remote())
	return o.Object.Remove(ctx)
}

// SetModTime sets the modification time of the file.
// Also prunes the cache entry when modtime changes so that
// touching a file will trigger checksum recalculation even
// on backends that don't provide modTime with fingerprint.
func (o *Object) SetModTime(ctx context.Context, mtime time.Time) error {
	if mtime != o.Object.ModTime(ctx) {
		_ = o.f.pruneHash(o.Remote())
	}
	return o.Object.SetModTime(ctx, mtime)
}

// Open opens the file for read.
// Full reads will also update object hashes.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (r io.ReadCloser, err error) {
	size := o.Size()
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch opt := option.(type) {
		case *fs.SeekOption:
			offset = opt.Offset
		case *fs.RangeOption:
			offset, limit = opt.Decode(size)
		}
	}
	if offset < 0 {
		return nil, errors.New("invalid offset")
	}
	if limit < 0 {
		limit = size - offset
	}
	if r, err = o.Object.Open(ctx, options...); err != nil {
		return nil, err
	}
	if offset != 0 || limit < size {
		// It's a partial read
		return r, err
	}
	return o.f.newHashingReader(ctx, r, func(sums hashMap) {
		if err := o.putHashes(ctx, sums); err != nil {
			fs.Infof(o, "auto hashing error: %v", err)
		}
	})
}

// Put data into the remote path with given modTime and size
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	var (
		o      *Object
		common hash.Set
		rehash bool
		hashes hashMap
	)
	if fsrc := src.Fs(); fsrc != nil {
		common = fsrc.Hashes().Overlap(f.keepHashes)
		// Rehash if source does not have all required hashes or hashing is slow
		rehash = fsrc.Features().SlowHash || common != f.keepHashes
	}

	wrapIn := in
	if rehash {
		r, err := f.newHashingReader(ctx, in, func(sums hashMap) {
			hashes = sums
		})
		fs.Debugf(src, "Rehash in-fly due to incomplete or slow source set %v (err: %v)", common, err)
		if err == nil {
			wrapIn = r
		} else {
			rehash = false
		}
	}

	_ = f.pruneHash(src.Remote())
	oResult, err := f.Fs.Put(ctx, wrapIn, src, options...)
	o = f.wrapObject(oResult, err)
	if o == nil {
		return nil, err
	}

	if !rehash {
		hashes = hashMap{}
		for _, ht := range common.Array() {
			if h, e := src.Hash(ctx, ht); e == nil && h != "" {
				hashes[ht] = h
			}
		}
	}
	if len(hashes) > 0 {
		err := o.putHashes(ctx, hashes)
		fs.Debugf(o, "Applied %d source hashes, err: %v", len(hashes), err)
	}
	return o, err
}

type hashingReader struct {
	rd     io.Reader
	hasher *hash.MultiHasher
	fun    func(hashMap)
}

func (f *Fs) newHashingReader(ctx context.Context, rd io.Reader, fun func(hashMap)) (*hashingReader, error) {
	hasher, err := hash.NewMultiHasherTypes(f.keepHashes)
	if err != nil {
		return nil, err
	}
	hr := &hashingReader{
		rd:     rd,
		hasher: hasher,
		fun:    fun,
	}
	return hr, nil
}

func (r *hashingReader) Read(p []byte) (n int, err error) {
	n, err = r.rd.Read(p)
	if err != nil && err != io.EOF {
		r.hasher = nil
	}
	if r.hasher != nil {
		if _, errHash := r.hasher.Write(p[:n]); errHash != nil {
			r.hasher = nil
			err = errHash
		}
	}
	if err == io.EOF && r.hasher != nil {
		r.fun(r.hasher.Sums())
		r.hasher = nil
	}
	return
}

func (r *hashingReader) Close() error {
	if rc, ok := r.rd.(io.ReadCloser); ok {
		return rc.Close()
	}
	return nil
}

// Return object fingerprint or empty string in case of errors
//
// Note that we can't use the generic `fs.Fingerprint` here because
// this fingerprint is used to pick _derived hashes_ that are slow
// to calculate or completely unsupported by the base remote.
//
// The hasher fingerprint must be based on `fsHash`, the first _fast_
// hash supported _by the underlying remote_ (if there is one),
// while `fs.Fingerprint` would select a hash _produced by hasher_
// creating unresolvable fingerprint loop.
func (o *Object) fingerprint(ctx context.Context) string {
	size := o.Object.Size()
	timeStr := "-"
	if o.f.fpTime {
		timeStr = o.Object.ModTime(ctx).UTC().Format(timeFormat)
		if timeStr == "" {
			return ""
		}
	}
	hashStr := "-"
	if o.f.fpHash != hash.None {
		var err error
		hashStr, err = o.Object.Hash(ctx, o.f.fpHash)
		if hashStr == "" || err != nil {
			return ""
		}
	}
	return fmt.Sprintf("%d,%s,%s", size, timeStr, hashStr)
}
