package alias

import (
	"errors"
	"fmt"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/hash"
	"io"
	"path"
	"path/filepath"
	"time"
)

var (
	errorReadOnly = errors.New("merge remotes are read only")
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "merge",
		Description: "Merge multiple existing remotes into one",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "remotes",
			Help:     "List of comma seperated remotes.\nCan be \"myremote:/,mysecondremote:/\", \"myremote:/,mysecondremote:/,mythrirdremote:/\", etc.",
			Required: true,
		}},
	}
	fs.Register(fsi)
}

// Options defines the configuration for this backend
type Options struct {
	Remotes fs.RemoteList `config:"remotes"`
}

// Fs represents a remote acd server
type Fs struct {
	name     string       // name of this remote
	features *fs.Features // optional features
	opt      Options      // options for this Fs
	root     string       // the path we are working on
	remotes  []fs.Fs      // slice of remotes
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
	return fmt.Sprintf("amazon drive root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Rmdir removes the root directory of the Fs object
func (f *Fs) Rmdir(dir string) error {
	return errorReadOnly
}

// Hashes returns hash.HashNone to indicate remote hashing is unavailable
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Mkdir makes the root directory of the Fs object
func (f *Fs) Mkdir(dir string) error {
	return errorReadOnly
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, errorReadOnly
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, errorReadOnly
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
	set := make(map[string]fs.DirEntry)
	for _, remote := range f.remotes {
		var remoteEntries, err = remote.List(dir)
		if err != nil {
			// ignore this remote here
			continue
		}
		for _, remoteEntry := range remoteEntries {
			set[remoteEntry.Remote()] = remoteEntry
		}
	}

	for key := range set {
		entries = append(entries, set[key])
	}
	return entries, nil
}

// NewObject creates a new remote merge file object
func (f *Fs) NewObject(path string) (fs.Object, error) {
	for _, remote := range f.remotes {
		var obj, err = remote.NewObject(path)
		if err != nil {
			// ignore this remote here
		}
		return obj, nil
	}
	return nil, errors.New("no match")
}

// Precision is the remote http file system's modtime precision, which we have no way of knowing. We estimate at 1s
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// NewFs contstructs an Fs from the path.
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
		return nil, errors.New("merge can't point to an empty remote - check the value of the remotes setting")
	}
	if len(opt.Remotes) == 1 {
		return nil, errors.New("merge can't point to a single remote - check the value of the remotes setting")
	}

	var remotes []fs.Fs
	for _, remote := range opt.Remotes {
		_, configName, fsPath, err := fs.ParseRemote(remote)
		if err != nil {
			return nil, err
		}
		root = path.Join(fsPath, filepath.ToSlash(root))
		if configName == "local" {
			myFs, err := fs.NewFs(root)
			if err != nil {
				// handle
			}
			remotes = append(remotes, myFs)
			continue
		}
		myFs, err := fs.NewFs(configName + ":" + root)
		remotes = append(remotes, myFs)
	}

	f := &Fs{
		name:    name,
		root:    root,
		opt:     *opt,
		remotes: remotes,
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		ReadMimeType:            true,
		CanHaveEmptyDirectories: true,
	}).Fill(f)

	// skip for now
	//if strings.HasPrefix(opt.Remotes, name+":") {
	//	return nil, errors.New("can't point alias remote at itself - check the value of the remote setting")
	//}

	return f, nil
}
