// Package crypt provides wrappers for Fs and Object which implement encryption
package crypt

import (
	"fmt"
	"io"
	"path"
	"strings"
	"sync"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "crypt",
		Description: "Encrypt/Decrypt a remote",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "remote",
			Help: "Remote to encrypt/decrypt.",
		}, {
			Name: "flatten",
			Help: "Flatten the directory structure - more secure, less useful - see docs for tradeoffs.",
			Examples: []fs.OptionExample{
				{
					Value: "0",
					Help:  "Don't flatten files (default) - good for unlimited files, but doesn't hide directory structure.",
				}, {
					Value: "1",
					Help:  "Spread files over 1 directory good for <10,000 files.",
				}, {
					Value: "2",
					Help:  "Spread files over 32 directories good for <320,000 files.",
				}, {
					Value: "3",
					Help:  "Spread files over 1024 directories good for <10,000,000 files.",
				}, {
					Value: "4",
					Help:  "Spread files over 32,768 directories good for <320,000,000 files.",
				}, {
					Value: "5",
					Help:  "Spread files over 1,048,576 levels good for <10,000,000,000 files.",
				},
			},
		}, {
			Name:       "password",
			Help:       "Password or pass phrase for encryption.",
			IsPassword: true,
		}, {
			Name:       "password2",
			Help:       "Password or pass phrase for salt. Optional but recommended.\nShould be different to the previous password.",
			IsPassword: true,
			Optional:   true,
		}},
	})
}

// NewFs contstructs an Fs from the path, container:path
func NewFs(name, rpath string) (fs.Fs, error) {
	flatten := fs.ConfigFile.MustInt(name, "flatten", 0)
	password := fs.ConfigFile.MustValue(name, "password", "")
	if password == "" {
		return nil, errors.New("password not set in config file")
	}
	password, err := fs.Reveal(password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt password")
	}
	salt := fs.ConfigFile.MustValue(name, "password2", "")
	if salt != "" {
		salt, err = fs.Reveal(salt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decrypt password2")
		}
	}
	cipher, err := newCipher(flatten, password, salt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make cipher")
	}
	remote := fs.ConfigFile.MustValue(name, "remote")
	remotePath := path.Join(remote, cipher.EncryptName(rpath))
	wrappedFs, err := fs.NewFs(remotePath)
	if err != fs.ErrorIsFile && err != nil {
		return nil, errors.Wrapf(err, "failed to make remote %q to wrap", remotePath)
	}
	f := &Fs{
		Fs:      wrappedFs,
		cipher:  cipher,
		flatten: flatten,
	}
	return f, err
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
	cipher  Cipher
	flatten int
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("%s with cipher", f.Fs.String())
}

// List the Fs into a channel
func (f *Fs) List(opts fs.ListOpts, dir string) {
	f.Fs.List(f.newListOpts(opts, dir), f.cipher.EncryptName(dir))
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	o, err := f.Fs.NewObject(f.cipher.EncryptName(remote))
	if err != nil {
		return nil, err
	}
	return f.newObject(o), nil
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	wrappedIn, err := f.cipher.EncryptData(in)
	if err != nil {
		return nil, err
	}
	o, err := f.Fs.Put(wrappedIn, f.newObjectInfo(src))
	if err != nil {
		return nil, err
	}
	return f.newObject(o), nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() fs.HashSet {
	return fs.HashSet(fs.HashNone)
}

// Purge all files in the root and the root directory
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge() error {
	do, ok := f.Fs.(fs.Purger)
	if !ok {
		return fs.ErrorCantPurge
	}
	return do.Purge()
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
func (f *Fs) Copy(src fs.Object, remote string) (fs.Object, error) {
	do, ok := f.Fs.(fs.Copier)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	oResult, err := do.Copy(o.Object, f.cipher.EncryptName(remote))
	if err != nil {
		return nil, err
	}
	return f.newObject(oResult), nil
}

// Move src to this remote using server side move operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(src fs.Object, remote string) (fs.Object, error) {
	do, ok := f.Fs.(fs.Mover)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	oResult, err := do.Move(o.Object, f.cipher.EncryptName(remote))
	if err != nil {
		return nil, err
	}
	return f.newObject(oResult), nil
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f.Fs
}

// Object describes a wrapped for being read from the Fs
//
// This decrypts the remote name and decrypts the data
type Object struct {
	fs.Object
	f *Fs
}

func (f *Fs) newObject(o fs.Object) *Object {
	return &Object{
		Object: o,
		f:      f,
	}
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.f
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.Remote()
}

// Remote returns the remote path
func (o *Object) Remote() string {
	remote := o.Object.Remote()
	decryptedName, err := o.f.cipher.DecryptName(remote)
	if err != nil {
		fs.Debug(remote, "Undecryptable file name: %v", err)
		return remote
	}
	return decryptedName
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	size, err := o.f.cipher.DecryptedSize(o.Object.Size())
	if err != nil {
		fs.Debug(o, "Bad size for decrypt: %v", err)
	}
	return size
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(hash fs.HashType) (string, error) {
	return "", nil
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open() (io.ReadCloser, error) {
	in, err := o.Object.Open()
	if err != nil {
		return in, err
	}
	return o.f.cipher.DecryptData(in)
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(in io.Reader, src fs.ObjectInfo) error {
	wrappedIn, err := o.f.cipher.EncryptData(in)
	if err != nil {
		return err
	}
	return o.Object.Update(wrappedIn, o.f.newObjectInfo(src))
}

// newDir returns a dir with the Name decrypted
func (f *Fs) newDir(dir *fs.Dir) *fs.Dir {
	new := *dir
	remote := dir.Name
	decryptedRemote, err := f.cipher.DecryptName(remote)
	if err != nil {
		fs.Debug(remote, "Undecryptable dir name: %v", err)
	} else {
		new.Name = decryptedRemote
	}
	return &new
}

// ObjectInfo describes a wrapped fs.ObjectInfo for being the source
//
// This encrypts the remote name and adjusts the size
type ObjectInfo struct {
	fs.ObjectInfo
	f *Fs
}

func (f *Fs) newObjectInfo(src fs.ObjectInfo) *ObjectInfo {
	return &ObjectInfo{
		ObjectInfo: src,
		f:          f,
	}
}

// Fs returns read only access to the Fs that this object is part of
func (o *ObjectInfo) Fs() fs.Info {
	return o.f
}

// Remote returns the remote path
func (o *ObjectInfo) Remote() string {
	return o.f.cipher.EncryptName(o.ObjectInfo.Remote())
}

// Size returns the size of the file
func (o *ObjectInfo) Size() int64 {
	return o.f.cipher.EncryptedSize(o.ObjectInfo.Size())
}

// ListOpts wraps a listopts decrypting the directory listing and
// replacing the Objects
type ListOpts struct {
	fs.ListOpts
	f    *Fs
	dir  string              // dir we are listing
	mu   sync.Mutex          // to protect dirs
	dirs map[string]struct{} // keep track of synthetic directory objects added
}

// Make a ListOpts wrapper
func (f *Fs) newListOpts(lo fs.ListOpts, dir string) *ListOpts {
	if dir != "" {
		dir += "/"
	}
	return &ListOpts{
		ListOpts: lo,
		f:        f,
		dir:      dir,
		dirs:     make(map[string]struct{}),
	}

}

// Level gets the recursion level for this listing.
//
// Fses may ignore this, but should implement it for improved efficiency if possible.
//
// Level 1 means list just the contents of the directory
//
// Each returned item must have less than level `/`s in.
func (lo *ListOpts) Level() int {
	// If flattened recurse fully
	if lo.f.flatten > 0 {
		return fs.MaxLevel
	}
	return lo.ListOpts.Level()
}

// addSyntheticDirs makes up directory objects for the path passed in
func (lo *ListOpts) addSyntheticDirs(path string) {
	lo.mu.Lock()
	defer lo.mu.Unlock()
	for {
		i := strings.LastIndexByte(path, '/')
		if i < 0 {
			break
		}
		path = path[:i]
		if path == "" {
			break
		}
		if _, found := lo.dirs[path]; found {
			break
		}
		slashes := strings.Count(path, "/")
		if slashes < lo.ListOpts.Level() {
			lo.ListOpts.AddDir(&fs.Dir{Name: path})
		}
		lo.dirs[path] = struct{}{}
	}
}

// Add an object to the output.
// If the function returns true, the operation has been aborted.
// Multiple goroutines can safely add objects concurrently.
func (lo *ListOpts) Add(obj fs.Object) (abort bool) {
	remote := obj.Remote()
	decryptedRemote, err := lo.f.cipher.DecryptName(remote)
	if err != nil {
		fs.Debug(remote, "Skipping undecryptable file name: %v", err)
		return lo.ListOpts.IsFinished()
	}
	// If flattened add synthetic directories
	if lo.f.flatten > 0 {
		lo.addSyntheticDirs(decryptedRemote)
		slashes := strings.Count(decryptedRemote, "/")
		if slashes >= lo.ListOpts.Level() {
			return lo.ListOpts.IsFinished()
		}
	}
	return lo.ListOpts.Add(lo.f.newObject(obj))
}

// AddDir adds a directory to the output.
// If the function returns true, the operation has been aborted.
// Multiple goroutines can safely add objects concurrently.
func (lo *ListOpts) AddDir(dir *fs.Dir) (abort bool) {
	// If flattened we don't add any directories from the underlying remote
	if lo.f.flatten > 0 {
		return lo.ListOpts.IsFinished()
	}
	remote := dir.Name
	_, err := lo.f.cipher.DecryptName(remote)
	if err != nil {
		fs.Debug(remote, "Skipping undecryptable dir name: %v", err)
		return lo.ListOpts.IsFinished()
	}
	return lo.ListOpts.AddDir(lo.f.newDir(dir))
}

// IncludeDirectory returns whether this directory should be
// included in the listing (and recursed into or not).
func (lo *ListOpts) IncludeDirectory(remote string) bool {
	// If flattened we look in all directories
	if lo.f.flatten > 0 {
		return true
	}
	decryptedRemote, err := lo.f.cipher.DecryptName(remote)
	if err != nil {
		fs.Debug(remote, "Not including undecryptable directory name: %v", err)
		return false
	}
	return lo.ListOpts.IncludeDirectory(decryptedRemote)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Purger = (*Fs)(nil)
	_ fs.Copier = (*Fs)(nil)
	_ fs.Mover  = (*Fs)(nil)
	// _ fs.DirMover       = (*Fs)(nil)
	// _ fs.PutUncheckeder = (*Fs)(nil)
	_ fs.UnWrapper  = (*Fs)(nil)
	_ fs.ObjectInfo = (*ObjectInfo)(nil)
	_ fs.Object     = (*Object)(nil)
	_ fs.ListOpts   = (*ListOpts)(nil)
)
