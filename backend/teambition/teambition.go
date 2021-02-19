package teambition

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/K265/teambition-pan-api/pkg/teambition/pan/api"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
)

const (
	maxFileNameLength = 1024
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "teambition",
		Description: "Teambition cloud storage",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "cookie",
			Help: `Cookie used to login to teambition cloud storage
must include TEAMBITION_SESSIONID and TEAMBITION_SESSIONID.sig`,
			Required: true,
			Examples: []fs.OptionExample{{
				Value: "TEAMBITION_SESSIONID=xxx;TEAMBITION_SESSIONID.sig=xxx",
				Help:  "Cookie used to login to teambition cloud storage",
			}},
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default:  encoder.Base | encoder.EncodeInvalidUtf8,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Cookie string `config:"cookie"`
}

// Fs represents a remote teambition server
type Fs struct {
	name     string         // name of this remote
	root     string         // the path we are working on if any
	opt      Options        // parsed config options
	ci       *fs.ConfigInfo // global config
	features *fs.Features   // optional features
	srv      api.Fs         // the connection to the teambition api
}

// Object describes a teambition object
type Object struct {
	fs     *Fs    // what this object is part of
	remote string // The remote path
	info   *api.Node
}

// NewFs constructs an Fs from the path, bucket:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	ci := fs.GetConfig(ctx)

	config := &api.Config{
		Cookie: opt.Cookie,
	}

	srv, err := api.NewFs(ctx, config)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create teambition api srv")
	}

	f := &Fs{
		name: name,
		root: root,
		opt:  *opt,
		ci:   ci,
		srv:  srv,
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)
	return f, nil
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
	return fmt.Sprintf("teambition root '%s'", f.root)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

func getNodeModTime(node *api.Node) time.Time {
	t, _ := node.GetTime()
	return t
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
	nodes, err := f.srv.List(ctx, path.Join(f.root, dir))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, _node := range nodes {
		node := _node
		remote := path.Join(dir, node.GetName())
		if node.IsDirectory() {
			entries = append(entries, fs.NewDir(remote, getNodeModTime(&node)))
		} else {
			o := &Object{
				fs:     f,
				remote: remote,
				info:   &node,
			}
			entries = append(entries, o)
		}
	}
	return entries, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	node, err := f.srv.Get(ctx, path.Join(f.root, remote), api.AnyKind)
	if err != nil {
		return nil, err
	}
	o := &Object{
		fs:     f,
		remote: remote,
		info:   node,
	}
	return o, nil
}

// Put in to the remote path with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Put should either
// return an error or upload it properly (rather than e.g. calling panic).
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	fs1 := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return fs1, fs1.Update(ctx, in, src, options...)
}

// from dropbox.go
func checkPathLength(name string) (err error) {
	for next := ""; len(name) > 0; name = next {
		if slash := strings.IndexRune(name, '/'); slash >= 0 {
			name, next = name[:slash], name[slash+1:]
		} else {
			next = ""
		}
		length := utf8.RuneCountInString(name)
		if length > maxFileNameLength {
			return fserrors.NoRetryError(fs.ErrorFileNameTooLong)
		}
	}
	return nil
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	p := path.Join(f.root, dir)
	if cErr := checkPathLength(p); cErr != nil {
		return cErr
	}
	_, err := f.srv.CreateFolder(ctx, p)
	return err
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	node, err := f.srv.Get(ctx, path.Join(f.root, dir), api.FolderKind)
	if err != nil {
		return errors.Wrap(err, "Rmdir error")
	}
	return f.srv.Remove(ctx, node)
}

// String returns a description of the Object
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

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o *Object) ModTime(context.Context) time.Time {
	return getNodeModTime(o.info)
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return o.info.Size
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.FixRangeOption(options, o.Size())
	headers := map[string]string{}
	fs.OpenOptionAddHeaders(options, headers)
	return o.fs.srv.Open(ctx, o.info, headers)
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	p := path.Join(o.fs.root, o.Remote())
	if cErr := checkPathLength(p); cErr != nil {
		return cErr
	}

	node, err := o.fs.srv.CreateFile(ctx, p, src.Size(), in, true)
	if err != nil {
		return err
	}
	o.info = node
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.srv.Remove(ctx, o.info)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs     = (*Fs)(nil)
	_ fs.Object = (*Object)(nil)
)
