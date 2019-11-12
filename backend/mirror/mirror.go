package mirror

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
)

const (
	cachePrefix = "rclone-mcache-"
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "mirror",
		Description: "Mirror merges the contents of several remotes",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "remotes",
			Help:     "List of space separated remotes.\nCan be 'remotea:test/dir remoteb:', '\"remotea:test/space dir\" remoteb:', etc.\nThe last remote is used to write to.",
			Required: true,
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	Remotes               fs.SpaceSepList `config:"remotes"`
	InMemoryCacheTreshold fs.SizeSuffix
}

// Fs represents a mirror of remotes
type Fs struct {
	name     string       // name of this remote
	features *fs.Features // optional features
	opt      Options      // options for this Fs
	root     string       // the path we are working on
	remotes  []fs.Fs      // slice of remotes
	hashSet  hash.Set     // intersection of hash types
}

// Object describes a mirror object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs      *Fs
	remote  string
	objects []fs.Object
}

// Dir is the special Mirrored dir
type Dir struct {
	fs     *Fs
	id     string
	remote string
	dirs   []*fs.Dir
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("mirror root '%s'", f.root)
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Used to create new objects
func (f *Fs) createObject(remote string) (o *Object) {
	// Temporary Object under construction
	o = &Object{
		fs:     f,
		remote: remote,
	}
	return o
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Rmdir removes the root directory of the Fs object
func (f *Fs) Rmdir(ctx context.Context, dir string) (err error) {
	for _, remote := range f.remotes {
		err = remote.Rmdir(ctx, dir)
		if err != nil {
			return err
		}
	}
	return nil
}

// Hashes returns hash.HashNone to indicate remote hashing is unavailable
func (f *Fs) Hashes() hash.Set {
	return f.hashSet
}

// Mkdir makes the root directory of the Fs object
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	for _, remote := range f.remotes {
		err = remote.Mkdir(ctx, dir)
		if err != nil {
			return err
		}
	}
	return nil
}

// Purge all files in the root and the root directory
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context) (err error) {
	for _, remote := range f.remotes {
		err = remote.Features().Purge(ctx)
		if err != nil {
			return err
		}
	}
	return nil
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
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (o fs.Object, err error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(srcObj, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	obj := f.createObject(remote)
	for _, roSrc := range srcObj.getRemotes() {
		roDst, err := roSrc.Fs().Features().Copy(ctx, roSrc, remote)
		if err != nil {
			return nil, err
		}
		obj.addRemote(roDst)
	}

	return obj, nil
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
/*func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (o fs.Object, err error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(srcObj, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	for _, wr := range f.remotes {
		o, err = wr.Features().Move(ctx, src, remote)
		if err != nil {
			return nil, err
		}
	}

	return f.wrapObject(o), nil
}*/

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
/*func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	for _, wr := range f.remotes {
		err := wr.Features().DirMove(ctx, wr, srcRemote, dstRemote)
		if err != nil {
			return err
		}
	}
	return nil
}*/

// PutStream uploads to the remote path with the modTime given of indeterminate size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
/*func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o, err := f.wr.Features().PutStream(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return f.wrapObject(o), err
}*/

func min(a, b *int64) *int64 {
	if a == nil && b != nil {
		return b
	}
	if a != nil && b == nil {
		return a
	}
	if *a < *b {
		return a
	}
	return b
}

func max(a, b *int64) *int64 {
	if a == nil && b != nil {
		return b
	}
	if a != nil && b == nil {
		return a
	}
	if *a > *b {
		return a
	}
	return b
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	usage := &fs.Usage{}

	for _, remote := range f.remotes {
		about, err := remote.Features().About(ctx)
		if err != nil {
			return nil, err
		}

		usage.Total = min(usage.Total, about.Total)
		usage.Used = max(usage.Used, about.Used)
		usage.Trashed = max(usage.Trashed, about.Trashed)
		usage.Other = max(usage.Other, about.Other)
		usage.Free = min(usage.Free, about.Free)
	}

	return usage, nil
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := f.createObject(src.Remote())
	if src.Size() > int64(f.opt.InMemoryCacheTreshold) {
		var tempFile *os.File

		// create the cache file
		tempFile, err := ioutil.TempFile("", cachePrefix)
		if err != nil {
			return nil, err
		}

		_ = os.Remove(tempFile.Name()) // Delete the file - may not work on Windows

		// clean up the file after we are done downloading
		defer func() {
			// the file should normally already be close, but just to make sure
			_ = tempFile.Close()
			_ = os.Remove(tempFile.Name()) // delete the cache file after we are done - may be deleted already
		}()

		if _, err = io.Copy(tempFile, in); err != nil {
			return nil, err
		}
		// jump to the start of the local file so we can pass it along
		if _, err = tempFile.Seek(0, 0); err != nil {
			return nil, err
		}

		for _, remote := range f.remotes {
			ro, err := remote.Put(ctx, tempFile, src, options...)
			if err != nil {
				return nil, err
			}
			o.addRemote(ro)
			if _, err = tempFile.Seek(0, 0); err != nil {
				return nil, err
			}
		}
	} else {
		// that's a small file, just read it into memory
		var inData []byte
		inData, err := ioutil.ReadAll(in)
		if err != nil {
			return nil, err
		}

		// set the reader to our read memory block
		out := bytes.NewReader(inData)
		for _, remote := range f.remotes {
			ro, err := remote.Put(ctx, out, src, options...)
			if err != nil {
				return nil, err
			}
			o.addRemote(ro)
			if _, err = out.Seek(0, 0); err != nil {
				return nil, err
			}
		}
	}

	return o, nil
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
	set2 := make(map[string]fs.DirEntry)

	remoteEntries, err := f.remotes[0].List(ctx, dir)
	if err != nil {
		return nil, err
	}
	for _, remoteEntry := range remoteEntries {
		if o, ok := remoteEntry.(fs.Object); ok {
			mirrorObject := &Object{
				fs:      f,
				remote:  remoteEntry.Remote(),
				objects: []fs.Object{o},
			}
			set2[remoteEntry.Remote()] = mirrorObject
		}
		if d, ok := remoteEntry.(*fs.Dir); ok {
			mirrorDir := &Dir{
				fs:     f,
				remote: remoteEntry.Remote(),
				dirs:   []*fs.Dir{d},
			}
			set2[remoteEntry.Remote()] = mirrorDir
		}
	}

	for i := 1; i < len(f.remotes); i = i + 1 {
		remoteEntries, err := f.remotes[i].List(ctx, dir)
		if err != nil {
			return nil, err
		}
		if len(remoteEntries) != len(set2) {
			return nil, errors.New("remotes out of sync")
		}
		for _, remoteEntry := range remoteEntries {
			if mirrorEntry, ok := set2[remoteEntry.Remote()]; ok {
				if mirrorObject, ok := mirrorEntry.(*Object); ok {
					if remoteObject, ok := remoteEntry.(fs.Object); ok {
						mirrorObject.addRemote(remoteObject)
					} else {
						return nil, errors.New("remote mismatch")
					}
				}
				if mirrorDir, ok := mirrorEntry.(*Dir); ok {
					if remoteDir, ok := remoteEntry.(*fs.Dir); ok {
						mirrorDir.addRemote(remoteDir)
					} else {
						return nil, errors.New("remote mismatch")
					}
				}
			} else {
				return nil, errors.New("remotes out of sync")
			}
		}
	}

	for _, entry := range set2 {
		entries = append(entries, entry)
	}
	return entries, nil
}

// NewObject creates a new remote union file object based on the first Object it finds (reverse remote order)
func (f *Fs) NewObject(ctx context.Context, path string) (fs.Object, error) {
	o := f.createObject(path)
	for _, remote := range f.remotes {
		obj, err := remote.NewObject(ctx, path)
		if err != nil {
			return nil, err
		}
		o.addRemote(obj)
	}
	return o, nil
}

// Precision is the greatest Precision of all remotes
func (f *Fs) Precision() time.Duration {
	var greatestPrecision time.Duration
	for _, remote := range f.remotes {
		if remote.Precision() > greatestPrecision {
			greatestPrecision = remote.Precision()
		}
	}
	return greatestPrecision
}

// ---------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the MD5 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	for _, remote := range o.objects {
		hash, err := remote.Hash(ctx, t)
		if err == nil {
			return hash, err
		}
	}
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.objects[0].Size()
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	for _, remote := range o.objects {
		if m, ok := remote.(fs.MimeTyper); ok {
			return m.MimeType(ctx)
		}
	}
	return ""
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.objects[len(o.objects)-1].ModTime(ctx)
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	return o.objects[len(o.objects)-1].Open(ctx, options...)
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	return errors.New("unsupported")
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	for _, remote := range o.objects {
		err := remote.Remove(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *Object) addRemote(obj fs.Object) {
	o.objects = append(o.objects, obj)
}

func (o *Object) getRemotes() []fs.Object {
	return o.objects
}

// ---------------------------------------------------

// String returns the name
func (d *Dir) String() string {
	return d.remote
}

// Remote returns the remote path
func (d *Dir) Remote() string {
	return d.remote
}

// SetRemote sets the remote
func (d *Dir) SetRemote(remote string) *Dir {
	for _, dir := range d.dirs {
		dir.SetRemote(remote)
	}
	d.remote = remote
	return d
}

// ID gets the optional ID
func (d *Dir) ID() string {
	return d.id
}

// SetID sets the optional ID
func (d *Dir) SetID(id string) *Dir {
	d.id = id
	return d
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (d *Dir) ModTime(ctx context.Context) time.Time {
	return d.dirs[len(d.dirs)-1].ModTime(ctx)
}

// Size returns the size of the file
func (d *Dir) Size() int64 {
	return d.dirs[len(d.dirs)-1].Size()
}

// SetSize sets the size of the directory
func (d *Dir) SetSize(size int64) *Dir {
	return d
}

// Items returns the count of items in this directory or this
// directory and subdirectories if known, -1 for unknown
func (d *Dir) Items() int64 {
	return d.dirs[len(d.dirs)-1].Items()
}

// SetItems sets the number of items in the directory
func (d *Dir) SetItems(items int64) *Dir {
	return d
}

func (d *Dir) addRemote(dir *fs.Dir) {
	d.dirs = append(d.dirs, dir)
}

func (d *Dir) getRemotes() []*fs.Dir {
	return d.dirs
}

// -----------------------------------------------------------------------------------

func checkErrors(errors []error, err error) bool {
	for _, v := range errors {
		if v != err {
			return false
		}
	}
	return true
}

// NewFs constructs an Fs from the path.
//
// The returned Fs is the actual Fs, referenced by remote in the config
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if len(opt.Remotes) == 0 {
		return nil, errors.New("union can't point to an empty remote - check the value of the remotes setting")
	}
	if len(opt.Remotes) == 1 {
		return nil, errors.New("union can't point to a single remote - check the value of the remotes setting")
	}
	for _, remote := range opt.Remotes {
		if strings.HasPrefix(remote, name+":") {
			return nil, errors.New("can't point union remote at itself - check the value of the remote setting")
		}
	}

	var remotes []fs.Fs
	var errors []error
	for i := range opt.Remotes {
		// Last remote first so we return the correct (last) matching fs in case of fs.ErrorIsFile
		var remote = opt.Remotes[len(opt.Remotes)-i-1]
		_, configName, fsPath, err := fs.ParseRemote(remote)
		if err != nil {
			return nil, err
		}
		var rootString = path.Join(fsPath, filepath.ToSlash(root))
		if configName != "local" {
			rootString = configName + ":" + rootString
		}
		myFs, err := cache.Get(rootString)
		if err != nil && err != fs.ErrorIsFile {
			return nil, err
		}
		remotes = append(remotes, myFs)
		errors = append(errors, err)
	}
	if checkErrors(errors, fs.ErrorIsFile) {
		err = fs.ErrorIsFile
	}

	// Reverse the remotes again so they are in the order as before
	for i, j := 0, len(remotes)-1; i < j; i, j = i+1, j-1 {
		remotes[i], remotes[j] = remotes[j], remotes[i]
	}

	f := &Fs{
		name:    name,
		root:    root,
		opt:     *opt,
		remotes: remotes,
	}
	var features = (&fs.Features{
		CaseInsensitive:         true,
		DuplicateFiles:          false,
		ReadMimeType:            true,
		WriteMimeType:           true,
		CanHaveEmptyDirectories: true,
		BucketBased:             true,
		SetTier:                 true,
		GetTier:                 true,
	}).Fill(f)
	for _, remote := range remotes {
		features = features.Mask(remote)
	}

	f.features = features

	// Get common intersection of hashes
	hashSet := f.remotes[0].Hashes()
	for _, remote := range f.remotes[1:] {
		hashSet = hashSet.Overlap(remote.Hashes())
	}
	f.hashSet = hashSet

	return f, err
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Purger = (*Fs)(nil)
	//_ fs.PutStreamer     = (*Fs)(nil)
	//_ fs.Copier          = (*Fs)(nil)
	//_ fs.Mover           = (*Fs)(nil)
	//_ fs.DirMover        = (*Fs)(nil)
	_ fs.Abouter = (*Fs)(nil)
)
