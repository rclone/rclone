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
			Help: "Remote to encrypt/decrypt.\nNormally should contain a ':' and a path, eg \"myremote:path/to/dir\",\n\"myremote:bucket\" or maybe \"myremote:\" (not recommended).",
		}, {
			Name: "filename_encryption",
			Help: "How to encrypt the filenames.",
			Examples: []fs.OptionExample{
				{
					Value: "off",
					Help:  "Don't encrypt the file names.  Adds a \".bin\" extension only.",
				}, {
					Value: "standard",
					Help:  "Encrypt the filenames see the docs for the details.",
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
	mode, err := NewNameEncryptionMode(fs.ConfigFileGet(name, "filename_encryption", "standard"))
	if err != nil {
		return nil, err
	}
	password := fs.ConfigFileGet(name, "password", "")
	if password == "" {
		return nil, errors.New("password not set in config file")
	}
	password, err = fs.Reveal(password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt password")
	}
	salt := fs.ConfigFileGet(name, "password2", "")
	if salt != "" {
		salt, err = fs.Reveal(salt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decrypt password2")
		}
	}
	cipher, err := newCipher(mode, password, salt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make cipher")
	}
	remote := fs.ConfigFileGet(name, "remote")
	if strings.HasPrefix(remote, name+":") {
		return nil, errors.New("can't point crypt remote at itself - check the value of the remote setting")
	}
	// Look for a file first
	remotePath := path.Join(remote, cipher.EncryptFileName(rpath))
	wrappedFs, err := fs.NewFs(remotePath)
	// if that didn't produce a file, look for a directory
	if err != fs.ErrorIsFile {
		remotePath = path.Join(remote, cipher.EncryptDirName(rpath))
		wrappedFs, err = fs.NewFs(remotePath)
	}
	if err != fs.ErrorIsFile && err != nil {
		return nil, errors.Wrapf(err, "failed to make remote %q to wrap", remotePath)
	}
	f := &Fs{
		Fs:     wrappedFs,
		name:   name,
		root:   rpath,
		cipher: cipher,
		mode:   mode,
	}
	// the features here are ones we could support, and they are
	// ANDed with the ones from wrappedFs
	f.features = (&fs.Features{
		CaseInsensitive: mode == NameEncryptionOff,
		DuplicateFiles:  true,
		ReadMimeType:    false, // MimeTypes not supported with crypt
		WriteMimeType:   false,
	}).Fill(f).Mask(wrappedFs)
	return f, err
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
	name     string
	root     string
	features *fs.Features // optional features
	cipher   Cipher
	mode     NameEncryptionMode
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("Encrypted %s", f.Fs.String())
}

// List the Fs into a channel
func (f *Fs) List(opts fs.ListOpts, dir string) {
	f.Fs.List(f.newListOpts(opts, dir), f.cipher.EncryptDirName(dir))
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	o, err := f.Fs.NewObject(f.cipher.EncryptFileName(remote))
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

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(dir string) error {
	return f.Fs.Mkdir(f.cipher.EncryptDirName(dir))
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(dir string) error {
	return f.Fs.Rmdir(f.cipher.EncryptDirName(dir))
}

// Purge all files in the root and the root directory
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge() error {
	do := f.Fs.Features().Purge
	if do == nil {
		return fs.ErrorCantPurge
	}
	return do()
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
	do := f.Fs.Features().Copy
	if do == nil {
		return nil, fs.ErrorCantCopy
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	oResult, err := do(o.Object, f.cipher.EncryptFileName(remote))
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
	do := f.Fs.Features().Move
	if do == nil {
		return nil, fs.ErrorCantMove
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	oResult, err := do(o.Object, f.cipher.EncryptFileName(remote))
	if err != nil {
		return nil, err
	}
	return f.newObject(oResult), nil
}

// DirMove moves src to this remote using server side move
// operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(src fs.Fs) error {
	do := f.Fs.Features().DirMove
	if do == nil {
		return fs.ErrorCantDirMove
	}
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debug(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	return do(srcFs.Fs)
}

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(in io.Reader, src fs.ObjectInfo) (fs.Object, error) {
	do := f.Fs.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}
	wrappedIn, err := f.cipher.EncryptData(in)
	if err != nil {
		return nil, err
	}
	o, err := do(wrappedIn, f.newObjectInfo(src))
	if err != nil {
		return nil, err
	}
	return f.newObject(o), nil
}

// CleanUp the trash in the Fs
//
// Implement this if you have a way of emptying the trash or
// otherwise cleaning up old versions of files.
func (f *Fs) CleanUp() error {
	do := f.Fs.Features().CleanUp
	if do == nil {
		return errors.New("can't CleanUp")
	}
	return do()
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
	decryptedName, err := o.f.cipher.DecryptFileName(remote)
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
func (o *Object) Open(options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	var offset int64
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		default:
			if option.Mandatory() {
				fs.Log(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	rc, err = o.f.cipher.DecryptDataSeek(func(underlyingOffset int64) (io.ReadCloser, error) {
		if underlyingOffset == 0 {
			// Open with no seek
			return o.Object.Open()
		}
		// Open stream with a seek of underlyingOffset
		return o.Object.Open(&fs.SeekOption{Offset: underlyingOffset})
	}, offset)
	if err != nil {
		return nil, err
	}
	return rc, err
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
	decryptedRemote, err := f.cipher.DecryptDirName(remote)
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
	return o.f.cipher.EncryptFileName(o.ObjectInfo.Remote())
}

// Size returns the size of the file
func (o *ObjectInfo) Size() int64 {
	return o.f.cipher.EncryptedSize(o.ObjectInfo.Size())
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *ObjectInfo) Hash(hash fs.HashType) (string, error) {
	return "", nil
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
	return lo.ListOpts.Level()
}

// Add an object to the output.
// If the function returns true, the operation has been aborted.
// Multiple goroutines can safely add objects concurrently.
func (lo *ListOpts) Add(obj fs.Object) (abort bool) {
	remote := obj.Remote()
	_, err := lo.f.cipher.DecryptFileName(remote)
	if err != nil {
		fs.Debug(remote, "Skipping undecryptable file name: %v", err)
		return lo.ListOpts.IsFinished()
	}
	return lo.ListOpts.Add(lo.f.newObject(obj))
}

// AddDir adds a directory to the output.
// If the function returns true, the operation has been aborted.
// Multiple goroutines can safely add objects concurrently.
func (lo *ListOpts) AddDir(dir *fs.Dir) (abort bool) {
	remote := dir.Name
	_, err := lo.f.cipher.DecryptDirName(remote)
	if err != nil {
		fs.Debug(remote, "Skipping undecryptable dir name: %v", err)
		return lo.ListOpts.IsFinished()
	}
	return lo.ListOpts.AddDir(lo.f.newDir(dir))
}

// IncludeDirectory returns whether this directory should be
// included in the listing (and recursed into or not).
func (lo *ListOpts) IncludeDirectory(remote string) bool {
	decryptedRemote, err := lo.f.cipher.DecryptDirName(remote)
	if err != nil {
		fs.Debug(remote, "Not including undecryptable directory name: %v", err)
		return false
	}
	return lo.ListOpts.IncludeDirectory(decryptedRemote)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs             = (*Fs)(nil)
	_ fs.Purger         = (*Fs)(nil)
	_ fs.Copier         = (*Fs)(nil)
	_ fs.Mover          = (*Fs)(nil)
	_ fs.DirMover       = (*Fs)(nil)
	_ fs.PutUncheckeder = (*Fs)(nil)
	_ fs.CleanUpper     = (*Fs)(nil)
	_ fs.UnWrapper      = (*Fs)(nil)
	_ fs.ObjectInfo     = (*ObjectInfo)(nil)
	_ fs.Object         = (*Object)(nil)
	_ fs.ListOpts       = (*ListOpts)(nil)
)
