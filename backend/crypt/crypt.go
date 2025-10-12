// Package crypt provides wrappers for Fs and Object which implement encryption
package crypt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fspath"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/list"
)

// Globals
// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "crypt",
		Description: "Encrypt/Decrypt a remote",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		MetadataInfo: &fs.MetadataInfo{
			Help: `Any metadata supported by the underlying remote is read and written.`,
		},
		Options: []fs.Option{{
			Name:     "remote",
			Help:     "Remote to encrypt/decrypt.\n\nNormally should contain a ':' and a path, e.g. \"myremote:path/to/dir\",\n\"myremote:bucket\" or maybe \"myremote:\" (not recommended).",
			Required: true,
		}, {
			Name:    "filename_encryption",
			Help:    "How to encrypt the filenames.",
			Default: "standard",
			Examples: []fs.OptionExample{
				{
					Value: "standard",
					Help:  "Encrypt the filenames.\nSee the docs for the details.",
				}, {
					Value: "obfuscate",
					Help:  "Very simple filename obfuscation.",
				}, {
					Value: "off",
					Help:  "Don't encrypt the file names.\nAdds a \".bin\", or \"suffix\" extension only.",
				},
			},
		}, {
			Name: "directory_name_encryption",
			Help: `Option to either encrypt directory names or leave them intact.

NB If filename_encryption is "off" then this option will do nothing.`,
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
			Required:   true,
		}, {
			Name:       "password2",
			Help:       "Password or pass phrase for salt.\n\nOptional but recommended.\nShould be different to the previous password.",
			IsPassword: true,
		}, {
			Name:    "server_side_across_configs",
			Default: false,
			Help: `Deprecated: use --server-side-across-configs instead.

Allow server-side operations (e.g. copy) to work across different crypt configs.

Normally this option is not what you want, but if you have two crypts
pointing to the same backend you can use it.

This can be used, for example, to change file name encryption type
without re-uploading all the data. Just make two crypt backends
pointing to two different directories with the single changed
parameter and use rclone move to move the files between the crypt
remotes.`,
			Advanced: true,
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
		}, {
			Name:     "no_data_encryption",
			Help:     "Option to either encrypt file data or leave it unencrypted.",
			Default:  false,
			Advanced: true,
			Examples: []fs.OptionExample{
				{
					Value: "true",
					Help:  "Don't encrypt file data, leave it unencrypted.",
				},
				{
					Value: "false",
					Help:  "Encrypt file data.",
				},
			},
		}, {
			Name: "pass_bad_blocks",
			Help: `If set this will pass bad blocks through as all 0.

This should not be set in normal operation, it should only be set if
trying to recover an encrypted file with errors and it is desired to
recover as much of the file as possible.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "strict_names",
			Help: `If set, this will raise an error when crypt comes across a filename that can't be decrypted.

(By default, rclone will just log a NOTICE and continue as normal.)
This can happen if encrypted and unencrypted files are stored in the same
directory (which is not recommended.) It may also indicate a more serious
problem that should be investigated.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "filename_encoding",
			Help: `How to encode the encrypted filename to text string.

This option could help with shortening the encrypted filename. The 
suitable option would depend on the way your remote count the filename
length and if it's case sensitive.`,
			Default: "base32",
			Examples: []fs.OptionExample{
				{
					Value: "base32",
					Help:  "Encode using base32. Suitable for all remote.",
				},
				{
					Value: "base64",
					Help:  "Encode using base64. Suitable for case sensitive remote.",
				},
				{
					Value: "base32768",
					Help:  "Encode using base32768. Suitable if your remote counts UTF-16 or\nUnicode codepoint instead of UTF-8 byte length. (Eg. Onedrive, Dropbox)",
				},
			},
			Advanced: true,
		}, {
			Name: "suffix",
			Help: `If this is set it will override the default suffix of ".bin".

Setting suffix to "none" will result in an empty suffix. This may be useful 
when the path length is critical.`,
			Default:  ".bin",
			Advanced: true,
		}},
	})
}

// newCipherForConfig constructs a Cipher for the given config name
func newCipherForConfig(opt *Options) (*Cipher, error) {
	mode, err := NewNameEncryptionMode(opt.FilenameEncryption)
	if err != nil {
		return nil, err
	}
	if opt.Password == "" {
		return nil, errors.New("password not set in config file")
	}
	password, err := obscure.Reveal(opt.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt password: %w", err)
	}
	var salt string
	if opt.Password2 != "" {
		salt, err = obscure.Reveal(opt.Password2)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt password2: %w", err)
		}
	}
	enc, err := NewNameEncoding(opt.FilenameEncoding)
	if err != nil {
		return nil, err
	}
	cipher, err := newCipher(mode, password, salt, opt.DirectoryNameEncryption, enc)
	if err != nil {
		return nil, fmt.Errorf("failed to make cipher: %w", err)
	}
	cipher.setEncryptedSuffix(opt.Suffix)
	cipher.setPassBadBlocks(opt.PassBadBlocks)
	return cipher, nil
}

// NewCipher constructs a Cipher for the given config
func NewCipher(m configmap.Mapper) (*Cipher, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	return newCipherForConfig(opt)
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, rpath string, m configmap.Mapper) (fs.Fs, error) {
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
	// Make sure to remove trailing . referring to the current dir
	if path.Base(rpath) == "." {
		rpath = strings.TrimSuffix(rpath, ".")
	}
	// Look for a file first
	var wrappedFs fs.Fs
	if rpath == "" {
		wrappedFs, err = cache.Get(ctx, remote)
	} else {
		remotePath := fspath.JoinRootPath(remote, cipher.EncryptFileName(rpath))
		wrappedFs, err = cache.Get(ctx, remotePath)
		// if that didn't produce a file, look for a directory
		if err != fs.ErrorIsFile {
			remotePath = fspath.JoinRootPath(remote, cipher.EncryptDirName(rpath))
			wrappedFs, err = cache.Get(ctx, remotePath)
		}
	}
	if err != fs.ErrorIsFile && err != nil {
		return nil, fmt.Errorf("failed to make remote %q to wrap: %w", remote, err)
	}
	f := &Fs{
		Fs:     wrappedFs,
		name:   name,
		root:   rpath,
		opt:    *opt,
		cipher: cipher,
	}
	cache.PinUntilFinalized(f.Fs, f)
	// Correct root if definitely pointing to a file
	if err == fs.ErrorIsFile {
		f.root = path.Dir(f.root)
		if f.root == "." || f.root == "/" {
			f.root = ""
		}
	}
	// the features here are ones we could support, and they are
	// ANDed with the ones from wrappedFs
	f.features = (&fs.Features{
		CaseInsensitive:          !cipher.dirNameEncrypt || cipher.NameEncryptionMode() == NameEncryptionOff,
		DuplicateFiles:           true,
		ReadMimeType:             false, // MimeTypes not supported with crypt
		WriteMimeType:            false,
		BucketBased:              true,
		CanHaveEmptyDirectories:  true,
		SetTier:                  true,
		GetTier:                  true,
		ServerSideAcrossConfigs:  opt.ServerSideAcrossConfigs,
		ReadMetadata:             true,
		WriteMetadata:            true,
		UserMetadata:             true,
		ReadDirMetadata:          true,
		WriteDirMetadata:         true,
		WriteDirSetModTime:       true,
		UserDirMetadata:          true,
		DirModTimeUpdatesOnWrite: true,
		PartialUploads:           true,
	}).Fill(ctx, f).Mask(ctx, wrappedFs).WrapsFs(f, wrappedFs)

	// Enable ListP always
	f.features.ListP = f.ListP

	return f, err
}

// Options defines the configuration for this backend
type Options struct {
	Remote                  string `config:"remote"`
	FilenameEncryption      string `config:"filename_encryption"`
	DirectoryNameEncryption bool   `config:"directory_name_encryption"`
	NoDataEncryption        bool   `config:"no_data_encryption"`
	Password                string `config:"password"`
	Password2               string `config:"password2"`
	ServerSideAcrossConfigs bool   `config:"server_side_across_configs"`
	ShowMapping             bool   `config:"show_mapping"`
	PassBadBlocks           bool   `config:"pass_bad_blocks"`
	FilenameEncoding        string `config:"filename_encoding"`
	Suffix                  string `config:"suffix"`
	StrictNames             bool   `config:"strict_names"`
}

// Fs represents a wrapped fs.Fs
type Fs struct {
	fs.Fs
	wrapper  fs.Fs
	name     string
	root     string
	opt      Options
	features *fs.Features // optional features
	cipher   *Cipher
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
func (f *Fs) add(entries *fs.DirEntries, obj fs.Object) error {
	remote := obj.Remote()
	decryptedRemote, err := f.cipher.DecryptFileName(remote)
	if err != nil {
		if f.opt.StrictNames {
			return fmt.Errorf("%s: undecryptable file name detected: %v", remote, err)
		}
		fs.Logf(remote, "Skipping undecryptable file name: %v", err)
		return nil
	}
	if f.opt.ShowMapping {
		fs.Logf(decryptedRemote, "Encrypts to %q", remote)
	}
	*entries = append(*entries, f.newObject(obj))
	return nil
}

// Encrypt a directory file name to entries.
func (f *Fs) addDir(ctx context.Context, entries *fs.DirEntries, dir fs.Directory) error {
	remote := dir.Remote()
	decryptedRemote, err := f.cipher.DecryptDirName(remote)
	if err != nil {
		if f.opt.StrictNames {
			return fmt.Errorf("%s: undecryptable dir name detected: %v", remote, err)
		}
		fs.Logf(remote, "Skipping undecryptable dir name: %v", err)
		return nil
	}
	if f.opt.ShowMapping {
		fs.Logf(decryptedRemote, "Encrypts to %q", remote)
	}
	*entries = append(*entries, f.newDir(ctx, dir))
	return nil
}

// Encrypt some directory entries.  This alters entries returning it as newEntries.
func (f *Fs) encryptEntries(ctx context.Context, entries fs.DirEntries) (newEntries fs.DirEntries, err error) {
	newEntries = entries[:0] // in place filter
	errors := 0
	var firsterr error
	for _, entry := range entries {
		switch x := entry.(type) {
		case fs.Object:
			err = f.add(&newEntries, x)
		case fs.Directory:
			err = f.addDir(ctx, &newEntries, x)
		default:
			return nil, fmt.Errorf("unknown object type %T", entry)
		}
		if err != nil {
			errors++
			if firsterr == nil {
				firsterr = err
			}
		}
	}
	if firsterr != nil {
		return nil, fmt.Errorf("there were %v undecryptable name errors. first error: %v", errors, firsterr)
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
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	return list.WithListP(ctx, dir, f)
}

// ListP lists the objects and directories of the Fs starting
// from dir non recursively into out.
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
func (f *Fs) ListP(ctx context.Context, dir string, callback fs.ListRCallback) error {
	wrappedCallback := func(entries fs.DirEntries) error {
		entries, err := f.encryptEntries(ctx, entries)
		if err != nil {
			return err
		}
		return callback(entries)
	}
	listP := f.Fs.Features().ListP
	encryptedDir := f.cipher.EncryptDirName(dir)
	if listP == nil {
		entries, err := f.Fs.List(ctx, encryptedDir)
		if err != nil {
			return err
		}
		return wrappedCallback(entries)
	}
	return listP(ctx, encryptedDir, wrappedCallback)
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
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	return f.Fs.Features().ListR(ctx, f.cipher.EncryptDirName(dir), func(entries fs.DirEntries) error {
		newEntries, err := f.encryptEntries(ctx, entries)
		if err != nil {
			return err
		}
		return callback(newEntries)
	})
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	o, err := f.Fs.NewObject(ctx, f.cipher.EncryptFileName(remote))
	if err != nil {
		return nil, err
	}
	return f.newObject(o), nil
}

type putFn func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error)

// put implements Put or PutStream
func (f *Fs) put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options []fs.OpenOption, put putFn) (fs.Object, error) {
	ci := fs.GetConfig(ctx)

	if f.opt.NoDataEncryption {
		o, err := put(ctx, in, f.newObjectInfo(src, nonce{}), options...)
		if err == nil && o != nil {
			o = f.newObject(o)
		}
		return o, err
	}

	// Encrypt the data into wrappedIn
	wrappedIn, encrypter, err := f.cipher.encryptData(in)
	if err != nil {
		return nil, err
	}

	// Find a hash the destination supports to compute a hash of
	// the encrypted data
	ht := f.Fs.Hashes().GetOne()
	if ci.IgnoreChecksum {
		ht = hash.None
	}
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
	o, err := put(ctx, wrappedIn, f.newObjectInfo(src, encrypter.nonce), options...)
	if err != nil {
		return nil, err
	}

	// Check the hashes of the encrypted data if we were comparing them
	if ht != hash.None && hasher != nil {
		srcHash := hasher.Sums()[ht]
		var dstHash string
		dstHash, err = o.Hash(ctx, ht)
		if err != nil {
			return nil, fmt.Errorf("failed to read destination hash: %w", err)
		}
		if srcHash != "" && dstHash != "" {
			if srcHash != dstHash {
				// remove object
				err = o.Remove(ctx)
				if err != nil {
					fs.Errorf(o, "Failed to remove corrupted object: %v", err)
				}
				return nil, fmt.Errorf("corrupted on transfer: %v encrypted hashes differ src(%s) %q vs dst(%s) %q", ht, f.Fs, srcHash, o.Fs(), dstHash)
			}
			fs.Debugf(src, "%v = %s OK", ht, srcHash)
		}
	}

	return f.newObject(o), nil
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(ctx, in, src, options, f.Fs.Put)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.put(ctx, in, src, options, f.Fs.Features().PutStream)
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return f.Fs.Mkdir(ctx, f.cipher.EncryptDirName(dir))
}

// MkdirMetadata makes the root directory of the Fs object
func (f *Fs) MkdirMetadata(ctx context.Context, dir string, metadata fs.Metadata) (fs.Directory, error) {
	do := f.Fs.Features().MkdirMetadata
	if do == nil {
		return nil, fs.ErrorNotImplemented
	}
	newDir, err := do(ctx, f.cipher.EncryptDirName(dir), metadata)
	if err != nil {
		return nil, err
	}
	var entries = make(fs.DirEntries, 0, 1)
	err = f.addDir(ctx, &entries, newDir)
	if err != nil {
		return nil, err
	}
	newDir, ok := entries[0].(fs.Directory)
	if !ok {
		return nil, fmt.Errorf("internal error: expecting %T to be fs.Directory", entries[0])
	}
	return newDir, nil
}

// DirSetModTime sets the directory modtime for dir
func (f *Fs) DirSetModTime(ctx context.Context, dir string, modTime time.Time) error {
	do := f.Fs.Features().DirSetModTime
	if do == nil {
		return fs.ErrorNotImplemented
	}
	return do(ctx, f.cipher.EncryptDirName(dir), modTime)
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.Fs.Rmdir(ctx, f.cipher.EncryptDirName(dir))
}

// Purge all files in the directory specified
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context, dir string) error {
	do := f.Fs.Features().Purge
	if do == nil {
		return fs.ErrorCantPurge
	}
	return do(ctx, f.cipher.EncryptDirName(dir))
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Copy
	if do == nil {
		return nil, fs.ErrorCantCopy
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	oResult, err := do(ctx, o.Object, f.cipher.EncryptFileName(remote))
	if err != nil {
		return nil, err
	}
	return f.newObject(oResult), nil
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	do := f.Fs.Features().Move
	if do == nil {
		return nil, fs.ErrorCantMove
	}
	o, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}
	oResult, err := do(ctx, o.Object, f.cipher.EncryptFileName(remote))
	if err != nil {
		return nil, err
	}
	return f.newObject(oResult), nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	do := f.Fs.Features().DirMove
	if do == nil {
		return fs.ErrorCantDirMove
	}
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	return do(ctx, srcFs.Fs, f.cipher.EncryptDirName(srcRemote), f.cipher.EncryptDirName(dstRemote))
}

// PutUnchecked uploads the object
//
// This will create a duplicate if we upload a new file without
// checking to see if there is one already - use Put() for that.
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	do := f.Fs.Features().PutUnchecked
	if do == nil {
		return nil, errors.New("can't PutUnchecked")
	}
	wrappedIn, encrypter, err := f.cipher.encryptData(in)
	if err != nil {
		return nil, err
	}
	o, err := do(ctx, wrappedIn, f.newObjectInfo(src, encrypter.nonce))
	if err != nil {
		return nil, err
	}
	return f.newObject(o), nil
}

// CleanUp the trash in the Fs
//
// Implement this if you have a way of emptying the trash or
// otherwise cleaning up old versions of files.
func (f *Fs) CleanUp(ctx context.Context) error {
	do := f.Fs.Features().CleanUp
	if do == nil {
		return errors.New("not supported by underlying remote")
	}
	return do(ctx)
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	do := f.Fs.Features().About
	if do == nil {
		return nil, errors.New("not supported by underlying remote")
	}
	return do(ctx)
}

// UnWrap returns the Fs that this Fs is wrapping
func (f *Fs) UnWrap() fs.Fs {
	return f.Fs
}

// WrapFs returns the Fs that is wrapping this Fs
func (f *Fs) WrapFs() fs.Fs {
	return f.wrapper
}

// SetWrapper sets the Fs that is wrapping this Fs
func (f *Fs) SetWrapper(wrapper fs.Fs) {
	f.wrapper = wrapper
}

// EncryptFileName returns an encrypted file name
func (f *Fs) EncryptFileName(fileName string) string {
	return f.cipher.EncryptFileName(fileName)
}

// DecryptFileName returns a decrypted file name
func (f *Fs) DecryptFileName(encryptedFileName string) (string, error) {
	return f.cipher.DecryptFileName(encryptedFileName)
}

// computeHashWithNonce takes the nonce and encrypts the contents of
// src with it, and calculates the hash given by HashType on the fly
//
// Note that we break lots of encapsulation in this function.
func (f *Fs) computeHashWithNonce(ctx context.Context, nonce nonce, src fs.Object, hashType hash.Type) (hashStr string, err error) {
	// Open the src for input
	in, err := src.Open(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open src: %w", err)
	}
	defer fs.CheckClose(in, &err)

	// Now encrypt the src with the nonce
	out, err := f.cipher.newEncrypter(in, &nonce)
	if err != nil {
		return "", fmt.Errorf("failed to make encrypter: %w", err)
	}

	// pipe into hash
	m, err := hash.NewMultiHasherTypes(hash.NewHashSet(hashType))
	if err != nil {
		return "", fmt.Errorf("failed to make hasher: %w", err)
	}
	_, err = io.Copy(m, out)
	if err != nil {
		return "", fmt.Errorf("failed to hash data: %w", err)
	}

	return m.Sums()[hashType], nil
}

// ComputeHash takes the nonce from o, and encrypts the contents of
// src with it, and calculates the hash given by HashType on the fly
//
// Note that we break lots of encapsulation in this function.
func (f *Fs) ComputeHash(ctx context.Context, o *Object, src fs.Object, hashType hash.Type) (hashStr string, err error) {
	if f.opt.NoDataEncryption {
		return src.Hash(ctx, hashType)
	}

	// Read the nonce - opening the file is sufficient to read the nonce in
	// use a limited read so we only read the header
	in, err := o.Object.Open(ctx, &fs.RangeOption{Start: 0, End: int64(fileHeaderSize) - 1})
	if err != nil {
		return "", fmt.Errorf("failed to open object to read nonce: %w", err)
	}
	d, err := f.cipher.newDecrypter(in)
	if err != nil {
		_ = in.Close()
		return "", fmt.Errorf("failed to open object to read nonce: %w", err)
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
		return "", fmt.Errorf("failed to close nonce read: %w", err)
	}

	return f.computeHashWithNonce(ctx, nonce, src, hashType)
}

// MergeDirs merges the contents of all the directories passed
// in into the first one and rmdirs the other directories.
func (f *Fs) MergeDirs(ctx context.Context, dirs []fs.Directory) error {
	do := f.Fs.Features().MergeDirs
	if do == nil {
		return errors.New("MergeDirs not supported")
	}
	out := make([]fs.Directory, len(dirs))
	for i, dir := range dirs {
		out[i] = fs.NewDirWrapper(f.cipher.EncryptDirName(dir.Remote()), dir)
	}
	return do(ctx, out)
}

// DirCacheFlush resets the directory cache - used in testing
// as an optional interface
func (f *Fs) DirCacheFlush() {
	do := f.Fs.Features().DirCacheFlush
	if do != nil {
		do()
	}
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	do := f.Fs.Features().PublicLink
	if do == nil {
		return "", errors.New("PublicLink not supported")
	}
	o, err := f.NewObject(ctx, remote)
	if err != nil {
		// assume it is a directory
		return do(ctx, f.cipher.EncryptDirName(remote), expire, unlink)
	}
	return do(ctx, o.(*Object).Object.Remote(), expire, unlink)
}

// ChangeNotify calls the passed function with a path
// that has had changes. If the implementation
// uses polling, it should adhere to the given interval.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	do := f.Fs.Features().ChangeNotify
	if do == nil {
		return
	}
	wrappedNotifyFunc := func(path string, entryType fs.EntryType) {
		// fs.Debugf(f, "ChangeNotify: path %q entryType %d", path, entryType)
		var (
			err       error
			decrypted string
		)
		switch entryType {
		case fs.EntryDirectory:
			decrypted, err = f.cipher.DecryptDirName(path)
		case fs.EntryObject:
			decrypted, err = f.cipher.DecryptFileName(path)
		default:
			fs.Errorf(path, "crypt ChangeNotify: ignoring unknown EntryType %d", entryType)
			return
		}
		if err != nil {
			fs.Logf(f, "ChangeNotify was unable to decrypt %q: %s", path, err)
			return
		}
		notifyFunc(decrypted, entryType)
	}
	do(ctx, wrappedNotifyFunc, pollIntervalChan)
}

var commandHelp = []fs.CommandHelp{
	{
		Name:  "encode",
		Short: "Encode the given filename(s)",
		Long: `This encodes the filenames given as arguments returning a list of
strings of the encoded results.

Usage Example:

    rclone backend encode crypt: file1 [file2...]
    rclone rc backend/command command=encode fs=crypt: file1 [file2...]
`,
	},
	{
		Name:  "decode",
		Short: "Decode the given filename(s)",
		Long: `This decodes the filenames given as arguments returning a list of
strings of the decoded results. It will return an error if any of the
inputs are invalid.

Usage Example:

    rclone backend decode crypt: encryptedfile1 [encryptedfile2...]
    rclone rc backend/command command=decode fs=crypt: encryptedfile1 [encryptedfile2...]
`,
	},
}

// Command the backend to run a named command
//
// The command run is name
// args may be used to read arguments from
// opts may be used to read optional arguments from
//
// The result should be capable of being JSON encoded
// If it is a string or a []string it will be shown to the user
// otherwise it will be JSON encoded and shown to the user like that
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (out any, err error) {
	switch name {
	case "decode":
		out := make([]string, 0, len(arg))
		for _, encryptedFileName := range arg {
			fileName, err := f.DecryptFileName(encryptedFileName)
			if err != nil {
				return out, fmt.Errorf("failed to decrypt: %s: %w", encryptedFileName, err)
			}
			out = append(out, fileName)
		}
		return out, nil
	case "encode":
		out := make([]string, 0, len(arg))
		for _, fileName := range arg {
			encryptedFileName := f.EncryptFileName(fileName)
			out = append(out, encryptedFileName)
		}
		return out, nil
	default:
		return nil, fs.ErrorCommandNotFound
	}
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
	size := o.Object.Size()
	if !o.f.opt.NoDataEncryption {
		var err error
		size, err = o.f.cipher.DecryptedSize(size)
		if err != nil {
			fs.Debugf(o, "Bad size for decrypt: %v", err)
		}
	}
	return size
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ht hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// UnWrap returns the wrapped Object
func (o *Object) UnWrap() fs.Object {
	return o.Object
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (rc io.ReadCloser, err error) {
	if o.f.opt.NoDataEncryption {
		return o.Object.Open(ctx, options...)
	}

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
	rc, err = o.f.cipher.DecryptDataSeek(ctx, func(ctx context.Context, underlyingOffset, underlyingLimit int64) (io.ReadCloser, error) {
		if underlyingOffset == 0 && underlyingLimit < 0 {
			// Open with no seek
			return o.Object.Open(ctx, openOptions...)
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
		return o.Object.Open(ctx, newOpenOptions...)
	}, offset, limit)
	if err != nil {
		return nil, err
	}
	return rc, nil
}

// Update in to the object with the modTime given of the given size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	update := func(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
		return o.Object, o.Object.Update(ctx, in, src, options...)
	}
	_, err := o.f.put(ctx, in, src, options, update)
	return err
}

// newDir returns a dir with the Name decrypted
func (f *Fs) newDir(ctx context.Context, dir fs.Directory) fs.Directory {
	remote := dir.Remote()
	decryptedRemote, err := f.cipher.DecryptDirName(remote)
	if err != nil {
		fs.Debugf(remote, "Undecryptable dir name: %v", err)
	} else {
		remote = decryptedRemote
	}
	newDir := fs.NewDirWrapper(remote, dir)
	return newDir
}

// UserInfo returns info about the connected user
func (f *Fs) UserInfo(ctx context.Context) (map[string]string, error) {
	do := f.Fs.Features().UserInfo
	if do == nil {
		return nil, fs.ErrorNotImplemented
	}
	return do(ctx)
}

// Disconnect the current user
func (f *Fs) Disconnect(ctx context.Context) error {
	do := f.Fs.Features().Disconnect
	if do == nil {
		return fs.ErrorNotImplemented
	}
	return do(ctx)
}

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	do := f.Fs.Features().Shutdown
	if do == nil {
		return nil
	}
	return do(ctx)
}

// ObjectInfo describes a wrapped fs.ObjectInfo for being the source
//
// This encrypts the remote name and adjusts the size
type ObjectInfo struct {
	fs.ObjectInfo
	f     *Fs
	nonce nonce
}

func (f *Fs) newObjectInfo(src fs.ObjectInfo, nonce nonce) *ObjectInfo {
	return &ObjectInfo{
		ObjectInfo: src,
		f:          f,
		nonce:      nonce,
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
	if o.f.opt.NoDataEncryption {
		return size
	}
	return o.f.cipher.EncryptedSize(size)
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *ObjectInfo) Hash(ctx context.Context, hash hash.Type) (string, error) {
	var srcObj fs.Object
	var ok bool
	// Get the underlying object if there is one
	if srcObj, ok = o.ObjectInfo.(fs.Object); ok {
		// Prefer direct interface assertion
	} else if do, ok := o.ObjectInfo.(*fs.OverrideRemote); ok {
		// Unwrap if it is an operations.OverrideRemote
		srcObj = do.UnWrap()
	} else {
		// Otherwise don't unwrap any further
		return "", nil
	}
	// if this is wrapping a local object then we work out the hash
	if srcObj.Fs().Features().IsLocal {
		// Read the data and encrypt it to calculate the hash
		fs.Debugf(o, "Computing %v hash of encrypted source", hash)
		return o.f.computeHashWithNonce(ctx, o.nonce, srcObj, hash)
	}
	return "", nil
}

// GetTier returns storage tier or class of the Object
func (o *ObjectInfo) GetTier() string {
	do, ok := o.ObjectInfo.(fs.GetTierer)
	if !ok {
		return ""
	}
	return do.GetTier()
}

// ID returns the ID of the Object if known, or "" if not
func (o *ObjectInfo) ID() string {
	do, ok := o.ObjectInfo.(fs.IDer)
	if !ok {
		return ""
	}
	return do.ID()
}

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (o *ObjectInfo) Metadata(ctx context.Context) (fs.Metadata, error) {
	do, ok := o.ObjectInfo.(fs.Metadataer)
	if !ok {
		return nil, nil
	}
	return do.Metadata(ctx)
}

// MimeType returns the content type of the Object if
// known, or "" if not
//
// This is deliberately unsupported so we don't leak mime type info by
// default.
func (o *ObjectInfo) MimeType(ctx context.Context) string {
	return ""
}

// UnWrap returns the Object that this Object is wrapping or
// nil if it isn't wrapping anything
func (o *ObjectInfo) UnWrap() fs.Object {
	return fs.UnWrapObjectInfo(o.ObjectInfo)
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	do, ok := o.Object.(fs.IDer)
	if !ok {
		return ""
	}
	return do.ID()
}

// SetTier performs changing storage tier of the Object if
// multiple storage classes supported
func (o *Object) SetTier(tier string) error {
	do, ok := o.Object.(fs.SetTierer)
	if !ok {
		return errors.New("crypt: underlying remote does not support SetTier")
	}
	return do.SetTier(tier)
}

// GetTier returns storage tier or class of the Object
func (o *Object) GetTier() string {
	do, ok := o.Object.(fs.GetTierer)
	if !ok {
		return ""
	}
	return do.GetTier()
}

// Metadata returns metadata for an object
//
// It should return nil if there is no Metadata
func (o *Object) Metadata(ctx context.Context) (fs.Metadata, error) {
	do, ok := o.Object.(fs.Metadataer)
	if !ok {
		return nil, nil
	}
	return do.Metadata(ctx)
}

// SetMetadata sets metadata for an Object
//
// It should return fs.ErrorNotImplemented if it can't set metadata
func (o *Object) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	do, ok := o.Object.(fs.SetMetadataer)
	if !ok {
		return fs.ErrorNotImplemented
	}
	return do.SetMetadata(ctx, metadata)
}

// MimeType returns the content type of the Object if
// known, or "" if not
//
// This is deliberately unsupported so we don't leak mime type info by
// default.
func (o *Object) MimeType(ctx context.Context) string {
	return ""
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.Commander       = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.UnWrapper       = (*Fs)(nil)
	_ fs.ListRer         = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Wrapper         = (*Fs)(nil)
	_ fs.MergeDirser     = (*Fs)(nil)
	_ fs.DirSetModTimer  = (*Fs)(nil)
	_ fs.MkdirMetadataer = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.ChangeNotifier  = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.UserInfoer      = (*Fs)(nil)
	_ fs.Disconnecter    = (*Fs)(nil)
	_ fs.Shutdowner      = (*Fs)(nil)
	_ fs.FullObjectInfo  = (*ObjectInfo)(nil)
	_ fs.FullObject      = (*Object)(nil)
)
