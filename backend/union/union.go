package union

import (
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/hash"
)

// Register with Fs
func init() {
	fsi := &fs.RegInfo{
		Name:        "union",
		Description: "Builds a stackable unification remote, which can appear to merge the contents of several remotes",
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
	Remotes fs.SpaceSepList `config:"remotes"`
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
	return fmt.Sprintf("union root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Rmdir removes the root directory of the Fs object
func (f *Fs) Rmdir(dir string) error {
	return f.remotes[len(f.remotes)-1].Rmdir(dir)
}

// Hashes returns hash.HashNone to indicate remote hashing is unavailable
func (f *Fs) Hashes() hash.Set {
	// This could probably be set if all remotes share the same hashing algorithm
	return hash.Set(hash.None)
}

// Mkdir makes the root directory of the Fs object
func (f *Fs) Mkdir(dir string) error {
	return f.remotes[len(f.remotes)-1].Mkdir(dir)
}

// Put in to the remote path with the modTime given of the given size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.remotes[len(f.remotes)-1].Put(in, src, options...)
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

// NewObject creates a new remote union file object based on the first Object it finds (reverse remote order)
func (f *Fs) NewObject(path string) (fs.Object, error) {
	for i := range f.remotes {
		var remote = f.remotes[len(f.remotes)-i-1]
		var obj, err = remote.NewObject(path)
		if err != nil {
			continue
		}
		return obj, nil
	}
	return nil, fs.ErrorObjectNotFound
}

// Precision is the greatest Precision of all remotes
func (f *Fs) Precision() time.Duration {
	var greatestPrecision = time.Second
	for _, remote := range f.remotes {
		if remote.Precision() <= greatestPrecision {
			continue
		}
		greatestPrecision = remote.Precision()
	}
	return greatestPrecision
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
		myFs, err := fs.NewFs(rootString)
		if err != nil {
			if err == fs.ErrorIsFile {
				return myFs, err
			} else {
				return nil, err
			}
		}
		remotes = append(remotes, myFs)
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
	var features = (&fs.Features{}).Fill(f)
	for _, remote := range f.remotes {
		features = features.Mask(remote)
	}
	f.features = features

	return f, nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs = &Fs{}
)
