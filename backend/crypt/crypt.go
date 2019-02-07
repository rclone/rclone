// Package crypt provides wrappers for Fs and Object which implement encryption
package crypt

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/config/obscure"
	"github.com/ncw/rclone/fs/fspath"
	"github.com/ncw/rclone/fs/hash"
	"github.com/pkg/errors"
)

// Globals
// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "crypt",
		Description: "Encrypt/Decrypt a remote",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "remote",
			Help:     "Remote to encrypt/decrypt.\nNormally should contain a ':' and a path, eg \"myremote:path/to/dir\",\n\"myremote:bucket\" or maybe \"myremote:\" (not recommended).",
			Required: true,
		}, {
			Name:    "filename_encryption",
			Help:    "How to encrypt the filenames.",
			Default: "standard",
			Examples: []fs.OptionExample{
				{
					Value: "off",
					Help:  "Don't encrypt the file names.  Adds a \".bin\" extension only.",
				}, {
					Value: "standard",
					Help:  "Encrypt the filenames see the docs for the details.",
				}, {
					Value: "obfuscate",
					Help:  "Very simple filename obfuscation.",
				},
			},
		}, {
			Name:    "directory_name_encryption",
			Help:    "Option to either encrypt directory names or leave them intact.",
			Default: true,
			Examples: []fs.OptionExample{
				{
					Value: "true",
					Help:  "Encrypt directory names.",
				},
				{
					Value: "false",
					Help:  "Don't encrypt directory names, leave them intact.",
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
		}, {
			Name: "show_mapping",
			Help: `For all files listed show how the names encrypt.

If this flag is set then for each file that the remote is asked to
list, it will log (at level INFO) a line stating the decrypted file
name and the encrypted file name.

This is so you can work out which encrypted names are which decrypted
names just in case you need to do something with the encrypted file
names, or for debugging purposes.`,
			Default:  false,
			Hide:     fs.OptionHideConfigurator,
			Advanced: true,
		}},
	})
}

// newCipherForConfig constructs a Cipher for the given config name
func newCipherForConfig(opt *Options) (Cipher, error) {
	mode, err := NewNameEncryptionMode(opt.FilenameEncryption)
	if err != nil {
		return nil, err
	}
	if opt.Password == "" {
		return nil, errors.New("password not set in config file")
	}
	password, err := obscure.Reveal(opt.Password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt password")
	}
	var salt string
	if opt.Password2 != "" {
		salt, err = obscure.Reveal(opt.Password2)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decrypt password2")
		}
	}
	cipher, err := newCipher(mode, password, salt, opt.DirectoryNameEncryption)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make cipher")
	}
	return cipher, nil
}

// NewCipher constructs a Cipher for the given config
func NewCipher(m configmap.Mapper) (Cipher, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	return newCipherForConfig(opt)
}

// NewFs constructs an Fs from the path, container:path
func NewFs(name, rpath string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	cipher, err := newCipherForConfig(opt)
	if err != nil {
		return nil, err
	}
	remote := opt.Remote
	if strings.HasPrefix(remote, name+":") {
		return nil, errors.New("can't point crypt remote at itself - check the value of the remote setting")
	}
	wInfo, wName, wPath, wConfig, err := fs.ConfigFs(remote)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse remote %q to wrap", remote)
	}
	// Look for a file first
	remotePath := fspath.JoinRootPath(wPath, cipher.EncryptFileName(rpath))
	wrappedFs, err := wInfo.NewFs(wName, remotePath, wConfig)
	// if that didn't produce a file, look for a directory
	if err != fs.ErrorIsFile {
		remotePath = fspath.JoinRootPath(wPath, cipher.EncryptDirName(rpath))
		wrappedFs, err = wInfo.NewFs(wName, remotePath, wConfig)
	}
	if err != fs.ErrorIsFile && err != nil {
		return nil, errors.Wrapf(err, "failed to make remote %s:%q to wrap", wName, remotePath)
	}
	f := &Fs{
		Fs:     wrappedFs,
		name:   name,
		root:   rpath,
		opt:    *opt,
		cipher: cipher,
	}
	// the features here are ones we could support, and they are
	// ANDed with the ones from wrappedFs
	f.features = (&fs.Features{
		CaseInsensitive:         cipher.NameEncryptionMode() == NameEncryptionOff,
		DuplicateFiles:          true,
		ReadMimeType:            false, // MimeTypes not supported with crypt
		WriteMimeType:           false,
		BucketBased:             true,
		CanHaveEmptyDirectories: true,
	}).Fill(f).Mask(wrappedFs).WrapsFs(f, wrappedFs)

	doChangeNotify := wrappedFs.Features().ChangeNotify
	if doChangeNotify != nil {
		f.features.ChangeNotify = func(notifyFunc func(string, fs.EntryType), pollInterval <-chan time.Duration) {
			wrappedNotifyFunc := func(path string, entryType fs.EntryType) {
				decrypted, err := f.DecryptFileName(path)
				if err != nil {
					fs.Logf(f, "ChangeNotify was unable to decrypt %q: %s", path, err)
					return
				}
				notifyFunc(decrypted, entryType)
			}
			doChangeNotify(wrappedNotifyFunc, pollInterval)
		}
	}

	return f, err
}

// Options defines the configuration for this backend
type Options struct {
	Remote                  string `config:"remote"`
	FilenameEncryption      string `config:"filename_encryption"`
	DirectoryNameEncryption bool   `config:"directory_name_encryption"`
	Password                string `config:"password"`
	Password2               string `config:"password2"`
	ShowMapping             bool   `config:"show_mapping"`
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
	name     string
	root     string
	opt      Options
	features *fs.Features // optional features
	cipher   Cipher
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
	return fmt.Sprintf("Encrypted drive '%s:%s'", f.name, f.root)
}

// Encrypt an object file name to entries.
func (f *Fs) add(entries *fs.DirEntries, obj fs.Object) {
	remote := obj.Remote()
	decryptedRemote, err := f.cipher.DecryptFileName(remote)
	if err != nil {
		fs.Debugf(remote, "Skipping undecryptable file name: %v", err)
		return
	}
	if f.opt.ShowMapping {
		fs.Logf(decryptedRemote, "Encrypts to %q", remote)
	}
	*entries = append(*entries, f.newObject(obj))
}

// Encrypt an directory file name to entries.
func (f *Fs) addDir(entries *fs.DirEntries, dir fs.Directory) {
	remote := dir.Remote()
	decryptedRemote, err := f.cipher.DecryptDirName(remote)
	if err != nil {
		fs.Debugf(remote, "Skipping undecryptable dir name: %v", err)
		return
	}
	if f.opt.ShowMapping {
		fs.Logf(decryptedRemote, "Encrypts to %q", remote)
	}
	*entries = append(*entries, f.newDir(dir))
}

// Encrypt some directory entries.  This alters entries returning it as newEntries.
func (f *Fs) encryptEntries(entries fs.DirEntries) (newEntries fs.DirEntries, err error) {
	newEntries = entries[:0] // in place filter
	for _, entry := range entries {
		switch x := entry.(type) {
		case fs.Object:
			f.add(&newEntries, x)
		case fs.Directory:
			f.addDir(&newEntries, x)
		default:
			return nil, errors.Errorf("Unknown object type %T", entry)
		}
	}
	return newEntries, nil
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {
	entries, err = f.Fs.List(f.cipher.EncryptDirName(dir))
	if err != nil {
		return nil, err
	}
	return f.encryptEntries(entries)
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
//
// Don't implement this unless you have a more efficient way
// of listing recursively that doing a directory traversal.
func (f *Fs) ListR(dir string, callback fs.ListRCallback) (err error) {
	return f.Fs.Features().ListR(f.cipher.EncryptDirName(dir), func(entries fs.DirEntries) error {
		newEntries, err := f.encryptEntries(entries)
		if err != nil {
			return err
		}
		return callback(newEntries)
	})
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(remote string) (fs.Object, error) {
	o, err := f.Fs.NewObject(f.cipher.EncryptFileName(remote))
	if err != nil {
		return nil, err
	}
	return f.newObject(o), nil
}

type putFn func(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)

// put implements Put or PutStream
func (f *Fs) put(in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn) (fs.Object, error) {
	// Encrypt the data into wrappedIn
	wrappedIn, err := f.cipher.EncryptData(in)
	if err != nil {
		return nil, err
	}

	// Find a hash the destination supports to compute a hash of
	// the encrypted data
	ht := f.Fs.Hashes().GetOne()
	var hasher *hash.MultiHasher
	if ht != hash.None {
		hasher, err = hash.NewMultiHasherTypes(hash.NewHashSet(ht))
		if err != nil {
			return nil, err
		}
		// unwrap the accounting
		var wrap accounting.WrapFn
		wrappedIn, wrap = accounting.UnWrap(wrappedIn)
		// add the hasher
		wrappedIn = io.TeeReader(wrappedIn, hasher)
		// wrap the accounting back on
		wrappedIn = wrap(wrappedIn)
	}

	// Transfer the data
	o, err := put(wrappedIn, f.newObjectInfo(src), options...)
	if err != nil {
		return nil, err
	}

	// Check the hashes of the encrypted data if we were comparing them
	if ht != hash.None && hasher != nil {
		srcHash := hasher.Sums()[ht]
		var dstHash string
		dstHash, err = o.Hash(ht)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read destination hash")
		}
		if srcHash != "" && dstHash != "" && srcHash != dstHash {
			// remove object
			err = o.Remove()
			if err != nil {
				fs.Errorf(o, "Failed to remove corrupted object: %v", err)
			}
			return nil, errors.Errorf("corrupted on transfer: %v crypted hash differ %q vs %q", ht, srcHash, dstHash)
		}
	}

	return f.newObject(o), nil
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(in, src, options, f.Fs.Put)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(in, src, options, f.Fs.Features().PutStream)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
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

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(src fs.Fs, srcRemote, dstRemote string) error {
	do := f.Fs.Features().DirMove
	if do == nil {
		return fs.ErrorCantDirMove
	}
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	return do(srcFs.Fs, f.cipher.EncryptDirName(srcRemote), f.cipher.EncryptDirName(dstRemote))
}

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
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

// About gets quota information from the Fs
func (f *Fs) About() (*fs.Usage, error) {
	do := f.Fs.Features().About
	if do == nil {
		return nil, errors.New("About not supported")
	}
	return do()
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f.Fs
}

// EncryptFileName returns an encrypted file name
func (f *Fs) EncryptFileName(fileName string) string {
	return f.cipher.EncryptFileName(fileName)
}

// DecryptFileName returns a decrypted file name
func (f *Fs) DecryptFileName(encryptedFileName string) (string, error) {
	return f.cipher.DecryptFileName(encryptedFileName)
}

// ComputeHash takes the nonce from o, and encrypts the contents of
// src with it, and calculates the hash given by HashType on the fly
//
// Note that we break lots of encapsulation in this function.
func (f *Fs) ComputeHash(o *Object, src fs.Object, hashType hash.Type) (hashStr string, err error) {
	// Read the nonce - opening the file is sufficient to read the nonce in
	// use a limited read so we only read the header
	in, err := o.Object.Open(&fs.RangeOption{Start: 0, End: int64(fileHeaderSize) - 1})
	if err != nil {
		return "", errors.Wrap(err, "failed to open object to read nonce")
	}
	d, err := f.cipher.(*cipher).newDecrypter(in)
	if err != nil {
		_ = in.Close()
		return "", errors.Wrap(err, "failed to open object to read nonce")
	}
	nonce := d.nonce
	// fs.Debugf(o, "Read nonce % 2x", nonce)

	// Check nonce isn't all zeros
	isZero := true
	for i := range nonce {
		if nonce[i] != 0 {
			isZero = false
		}
	}
	if isZero {
		fs.Errorf(o, "empty nonce read")
	}

	// Close d (and hence in) once we have read the nonce
	err = d.Close()
	if err != nil {
		return "", errors.Wrap(err, "failed to close nonce read")
	}

	// Open the src for input
	in, err = src.Open()
	if err != nil {
		return "", errors.Wrap(err, "failed to open src")
	}
	defer fs.CheckClose(in, &err)

	// Now encrypt the src with the nonce
	out, err := f.cipher.(*cipher).newEncrypter(in, &nonce)
	if err != nil {
		return "", errors.Wrap(err, "failed to make encrypter")
	}

	// pipe into hash
	m, err := hash.NewMultiHasherTypes(hash.NewHashSet(hashType))
	if err != nil {
		return "", errors.Wrap(err, "failed to make hasher")
	}
	_, err = io.Copy(m, out)
	if err != nil {
		return "", errors.Wrap(err, "failed to hash data")
	}

	return m.Sums()[hashType], nil
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
		fs.Debugf(remote, "Undecryptable file name: %v", err)
		return remote
	}
	return decryptedName
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	size, err := o.f.cipher.DecryptedSize(o.Object.Size())
	if err != nil {
		fs.Debugf(o, "Bad size for decrypt: %v", err)
	}
	return size
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ht hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// UnWrap returns the wrapped Object
func (o *Object) UnWrap() fs.Object {
	return o.Object
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	var openOptions []fs.OpenOption
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.Size())
		default:
			// pass on Options to underlying open if appropriate
			openOptions = append(openOptions, option)
		}
	}
	rc, err = o.f.cipher.DecryptDataSeek(func(underlyingOffset, underlyingLimit int64) (io.ReadCloser, error) {
		if underlyingOffset == 0 && underlyingLimit < 0 {
			// Open with no seek
			return o.Object.Open(openOptions...)
		}
		// Open stream with a range of underlyingOffset, underlyingLimit
		end := int64(-1)
		if underlyingLimit >= 0 {
			end = underlyingOffset + underlyingLimit - 1
			if end >= o.Object.Size() {
				end = -1
			}
		}
		newOpenOptions := append(openOptions, &fs.RangeOption{Start: underlyingOffset, End: end})
		return o.Object.Open(newOpenOptions...)
	}, offset, limit)
	if err != nil {
		return nil, err
	}
	return rc, nil
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	update := func(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
		return o.Object, o.Object.Update(in, src, options...)
	}
	_, err := o.f.put(in, src, options, update)
	return err
}

// newDir returns a dir with the Name decrypted
func (f *Fs) newDir(dir fs.Directory) fs.Directory {
	newDir := fs.NewDirCopy(dir)
	remote := dir.Remote()
	decryptedRemote, err := f.cipher.DecryptDirName(remote)
	if err != nil {
		fs.Debugf(remote, "Undecryptable dir name: %v", err)
	} else {
		newDir.SetRemote(decryptedRemote)
	}
	return newDir
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
	size := o.ObjectInfo.Size()
	if size < 0 {
		return size
	}
	return o.f.cipher.EncryptedSize(size)
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *ObjectInfo) Hash(hash hash.Type) (string, error) {
	return "", nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.UnWrapper       = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.ObjectInfo      = (*ObjectInfo)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.ObjectUnWrapper = (*Object)(nil)
)
