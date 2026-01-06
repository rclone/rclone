// Package kvfs provides an interface to the kv backend.
package kvfs

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
)

// Register Fs with rclone
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "kvfs",
		Description: "kv based filesystem",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			return nil, nil
		},
		Options: []fs.Option{
			{
				Name:      "config_dir",
				Help:      "Where you would like to store the kvfs kv db (e.g. /tmp/my/kv) ?",
				Required:  true,
				Sensitive: true,
				Default:   "~/.config/rclone/kvfs",
			}, {
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				// Encode invalid UTF-8 bytes as json doesn't handle them properly.
				// Don't encode / as it's a valid name character in drive.
				Default: (encoder.EncodeInvalidUtf8 |
					encoder.EncodeSlash |
					encoder.EncodeCtl |
					encoder.EncodeDel |
					encoder.EncodeBackSlash |
					encoder.EncodeRightPeriod),
			},
		},
	})
}

// NewFs constructs a new filesystem given a root path and rclone configuration options
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (ff fs.Fs, err error) {
	fs.Debugf(nil, "[NewFs] name: %q root: %q", name, root)
	opt := new(Options)
	err = configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
		features: &fs.Features{
			CanHaveEmptyDirectories: true,
			FilterAware:             true,
		},
	}

	f.db, err = f.getDb()
	if err != nil {
		return nil, err
	}
	err = f.db.Do(true, &opPut{
		key:   "NewFs",
		value: []byte(time.Now().UTC().Format(time.RFC3339)),
	})
	if err != nil {
		return nil, err
	}

	file, err := f.findFile(root)
	if err != nil {
		return nil, err
	}

	// check if file or directory
	if file != nil && file.Type != "dir" {
		root = path.Dir(file.Filename)
		err = fs.ErrorIsFile
		f.root = root
	}

	return f, err
}

// List returns a list of items in a directory
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	fullPath := f.fullPath(dir)
	fs.Debugf(nil, "[List] dir: %q fullPath: %q", dir, fullPath)
	files, err := f.getFiles(fullPath)
	if err != nil {
		return nil, err
	}

	entries = make([]fs.DirEntry, len(*files))
	for i, file := range *files {
		// remote is the fullpath of the file.Filename relative to the root
		remote := strings.TrimPrefix(strings.TrimPrefix(file.Filename, f.root), "/")

		if file.Type == "dir" {
			entries[i] = fs.NewDir(remote, time.UnixMilli(file.ModTime))
		} else {
			obj := &Object{
				fs:      f,
				info:    file,
				remote:  remote,
				size:    file.Size,
				modTime: time.UnixMilli(file.ModTime),
				sha1:    file.SHA1,
			}
			entries[i] = obj
		}
	}

	return entries, nil
}

// NewObject creates a new remote Object for a given remote path
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	fs.Debugf(nil, "[NewObject] remote: %q", remote)
	// Find the file with matching remote path
	file, err := f.findFile(path.Join(f.root, remote))
	if err != nil {
		return nil, err
	}
	if file != nil && file.Type != "dir" {
		return &Object{
			fs:      f,
			remote:  remote,
			info:    *file,
			size:    file.Size,
			modTime: time.UnixMilli(file.ModTime),
			sha1:    file.SHA1,
		}, nil
	}

	return nil, fs.ErrorObjectNotFound
}

// Put updates a remote Object
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (obj fs.Object, err error) {
	modTime := src.ModTime(ctx)
	remote := src.Remote()
	fullPath := f.fullPath(src.Remote())
	dirPath := path.Dir(fullPath)

	fs.Debugf(nil, "[Put] saving file: %q", fullPath)
	err = f.mkDir(dirPath)
	if err != nil {
		return nil, err
	}

	file, err := f.putFile(in, fullPath, modTime)
	if err != nil {
		return nil, err
	}
	return &Object{
		fs:      f,
		info:    *file,
		remote:  remote,
		modTime: time.UnixMilli(file.ModTime),
		size:    file.Size,
		sha1:    file.SHA1,
	}, nil
}

// Mkdir makes the directory (container, bucket)
// Creates ancestors if necessary
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return f.mkDir(f.fullPath(dir))
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	fullPath := f.fullPath(dir)
	fs.Debugf(nil, "[Rmdir] attempting removing dir: %q", fullPath)
	return f.rmDir(fullPath)
}

// Object Methods
// ------------------

// SetModTime is not supported
func (o *Object) SetModTime(ctx context.Context, mtime time.Time) error {
	return fs.ErrorCantSetModTimeWithoutDelete
}

// Open opens the Object for reading
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.Debugf(nil, "[Open] opening object: %q options: %+v", o.info.Filename, options)

	sOff, eOff := 0, len(o.info.Content)

	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			sOff = int(x.Offset)
			if sOff < 0 {
				sOff = eOff - (1 * sOff)
			}
		case *fs.RangeOption:
			sOff = int(x.Start)
			eOff = int(x.End) + 1
			fmt.Printf("[Open][RangeOption] sOff: %d eOff: %d\n", sOff, eOff)
			if x.End < 0 {
				eOff = len(o.info.Content)
			}
			if sOff < 0 {
				sOff = len(o.info.Content) - eOff + 1
				eOff = len(o.info.Content)
			}

			if eOff > len(o.info.Content) {
				eOff = len(o.info.Content)
			}
		default:
			if option.Mandatory() {
				fs.Debugf(o, "[Open] Unsupported mandatory option: %v", option)
			}
		}
	}

	content := o.info.Content[sOff:eOff]
	reader := io.NopCloser(strings.NewReader(content))
	return reader, nil
}

// Update updates the Object contents
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	modTime := src.ModTime(ctx)
	fs.Debugf(nil, "[Update] updating object: %q", o.info.Filename)
	file, err := o.fs.putFile(in, o.info.Filename, modTime)
	if err != nil {
		return err
	}
	o.info = *file
	o.size = file.Size
	o.sha1 = file.SHA1
	o.modTime = modTime
	return nil
}

// Remove deletes the remote Object
func (o *Object) Remove(ctx context.Context) error {
	fs.Debugf(nil, "[Remove] removing object: %q", o.info.Filename)
	err := o.fs.remove(o.info.Filename)
	if err != nil {
		return err
	}
	return nil
}

// ObjectInfo Methods
// ------------------

// Hash returns an SHA1 hash of the Object
func (o *Object) Hash(ctx context.Context, typ hash.Type) (string, error) {
	if typ == hash.SHA1 {
		return o.sha1, nil
	}
	return "", nil
}

// Storable returns true if the Object is storable
func (o *Object) Storable() bool {
	return true
}

// Info Methods
// ------------------

// Name returns the name of the Fs
func (f *Fs) Name() string {
	return f.name
}

// Root returns the root path of the Fs
func (f *Fs) Root() string {
	return f.root
}

// String returns a string representation of the Fs
func (f *Fs) String() string {
	return fmt.Sprintf("['%s']", f.root)
}

// Precision denotes that setting modification times is not supported
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

// Hashes returns a set of hashes are Provided by the Fs
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.SHA1)
}

// Features returns the optional features supported by this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// DirEntry Methods
// ------------------

// Fs returns a reference to the Stub Fs containing the Object
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns a string representation of the remote Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path of the Object, relative to Fs root
func (o *Object) Remote() string {
	return o.remote
}

// ModTime returns the modification time of the Object
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size return the size of the Object in bytes
func (o *Object) Size() int64 {
	return o.size
}
