// Package smb provides an interface to SMB servers
package smb

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/env"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
)

const (
	minSleep      = 100 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
)

var (
	currentUser = env.CurrentUser()
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "smb",
		Description: "SMB / CIFS",
		NewFs:       NewFs,

		Options: []fs.Option{{
			Name:      "host",
			Help:      "SMB server hostname to connect to.\n\nE.g. \"example.com\".",
			Required:  true,
			Sensitive: true,
		}, {
			Name:      "user",
			Help:      "SMB username.",
			Default:   currentUser,
			Sensitive: true,
		}, {
			Name:    "port",
			Help:    "SMB port number.",
			Default: 445,
		}, {
			Name:       "pass",
			Help:       "SMB password.",
			IsPassword: true,
		}, {
			Name:      "domain",
			Help:      "Domain name for NTLM authentication.",
			Default:   "WORKGROUP",
			Sensitive: true,
		}, {
			Name: "spn",
			Help: `Service principal name.

Rclone presents this name to the server. Some servers use this as further
authentication, and it often needs to be set for clusters. For example:

    cifs/remotehost:1020

Leave blank if not sure.
`,
			Sensitive: true,
		}, {
			Name:    "idle_timeout",
			Default: fs.Duration(60 * time.Second),
			Help: `Max time before closing idle connections.

If no connections have been returned to the connection pool in the time
given, rclone will empty the connection pool.

Set to 0 to keep connections indefinitely.
`,
			Advanced: true,
		}, {
			Name:     "hide_special_share",
			Help:     "Hide special shares (e.g. print$) which users aren't supposed to access.",
			Default:  true,
			Advanced: true,
		}, {
			Name:     "case_insensitive",
			Help:     "Whether the server is configured to be case-insensitive.\n\nAlways true on Windows shares.",
			Default:  true,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: encoder.EncodeZero |
				// path separator
				encoder.EncodeSlash |
				encoder.EncodeBackSlash |
				// windows
				encoder.EncodeWin |
				encoder.EncodeCtl |
				encoder.EncodeDot |
				// the file turns into 8.3 names (and cannot be converted back)
				encoder.EncodeRightSpace |
				encoder.EncodeRightPeriod |
				//
				encoder.EncodeInvalidUtf8,
		},
		}})
}

// Options defines the configuration for this backend
type Options struct {
	Host            string      `config:"host"`
	Port            string      `config:"port"`
	User            string      `config:"user"`
	Pass            string      `config:"pass"`
	Domain          string      `config:"domain"`
	SPN             string      `config:"spn"`
	HideSpecial     bool        `config:"hide_special_share"`
	CaseInsensitive bool        `config:"case_insensitive"`
	IdleTimeout     fs.Duration `config:"idle_timeout"`

	Enc encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a SMB remote
type Fs struct {
	name     string       // name of this remote
	root     string       // the path we are working on if any
	opt      Options      // parsed config options
	features *fs.Features // optional features
	pacer    *fs.Pacer    // pacer for operations

	sessions atomic.Int32
	poolMu   sync.Mutex
	pool     []*conn
	drain    *time.Timer // used to drain the pool when we stop using the connections

	ctx context.Context
}

// Object describes a file at the server
type Object struct {
	fs         *Fs    // reference to Fs
	remote     string // the remote path
	statResult os.FileInfo
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = strings.Trim(root, "/")

	f := &Fs{
		name: name,
		opt:  *opt,
		ctx:  ctx,
		root: root,
	}
	f.features = (&fs.Features{
		CaseInsensitive:         opt.CaseInsensitive,
		CanHaveEmptyDirectories: true,
		BucketBased:             true,
		PartialUploads:          true,
	}).Fill(ctx, f)

	f.pacer = fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant)))
	// set the pool drainer timer going
	if opt.IdleTimeout > 0 {
		f.drain = time.AfterFunc(time.Duration(opt.IdleTimeout), func() { _ = f.drainPool(ctx) })
	}

	// test if the root exists as a file
	share, dir := f.split("")
	if share == "" || dir == "" {
		return f, nil
	}
	cn, err := f.getConnection(ctx, share)
	if err != nil {
		return nil, err
	}
	stat, err := cn.smbShare.Stat(f.toSambaPath(dir))
	f.putConnection(&cn)
	if err != nil {
		// ignore stat error here
		return f, nil
	}
	if !stat.IsDir() {
		f.root, err = path.Dir(root), fs.ErrorIsFile
	}
	fs.Debugf(f, "Using root directory %q", f.root)
	return f, err
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
	bucket, file := f.split("")
	if bucket == "" {
		return fmt.Sprintf("smb://%s@%s:%s/", f.opt.User, f.opt.Host, f.opt.Port)
	}
	return fmt.Sprintf("smb://%s@%s:%s/%s/%s", f.opt.User, f.opt.Host, f.opt.Port, bucket, file)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes returns nothing as SMB itself doesn't have a way to tell checksums
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet()
}

// Precision returns the precision of mtime
func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

// NewObject creates a new file object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	share, path := f.split(remote)
	return f.findObjectSeparate(ctx, share, path)
}

func (f *Fs) findObjectSeparate(ctx context.Context, share, path string) (fs.Object, error) {
	if share == "" || path == "" {
		return nil, fs.ErrorIsDir
	}
	cn, err := f.getConnection(ctx, share)
	if err != nil {
		return nil, err
	}
	stat, err := cn.smbShare.Stat(f.toSambaPath(path))
	f.putConnection(&cn)
	if err != nil {
		return nil, translateError(err, false)
	}
	if stat.IsDir() {
		return nil, fs.ErrorIsDir
	}

	return f.makeEntry(share, path, stat), nil
}

// Mkdir creates a directory on the server
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	share, path := f.split(dir)
	if share == "" || path == "" {
		return nil
	}
	cn, err := f.getConnection(ctx, share)
	if err != nil {
		return err
	}
	err = cn.smbShare.MkdirAll(f.toSambaPath(path), 0o755)
	f.putConnection(&cn)
	return err
}

// Rmdir removes an empty directory on the server
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	share, path := f.split(dir)
	if share == "" || path == "" {
		return nil
	}
	cn, err := f.getConnection(ctx, share)
	if err != nil {
		return err
	}
	err = cn.smbShare.Remove(f.toSambaPath(path))
	f.putConnection(&cn)
	return err
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}

	err := o.Update(ctx, in, src, options...)
	if err == nil {
		return o, nil
	}

	return nil, err
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
//
// May create the object even if it returns an error - if so
// will return the object and the error, otherwise will return
// nil and the error
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}

	err := o.Update(ctx, in, src, options...)
	if err == nil {
		return o, nil
	}

	return nil, err
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
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (_ fs.Object, err error) {
	dstShare, dstPath := f.split(remote)
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	srcShare, srcPath := srcObj.split()
	if dstShare != srcShare {
		fs.Debugf(src, "Can't move - must be on the same share")
		return nil, fs.ErrorCantMove
	}

	err = f.ensureDirectory(ctx, dstShare, dstPath)
	if err != nil {
		return nil, fmt.Errorf("failed to make parent directories: %w", err)
	}

	cn, err := f.getConnection(ctx, dstShare)
	if err != nil {
		return nil, err
	}
	err = cn.smbShare.Rename(f.toSambaPath(srcPath), f.toSambaPath(dstPath))
	f.putConnection(&cn)
	if err != nil {
		return nil, translateError(err, false)
	}
	return f.findObjectSeparate(ctx, dstShare, dstPath)
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) (err error) {
	dstShare, dstPath := f.split(dstRemote)
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcShare, srcPath := srcFs.split(srcRemote)
	if dstShare != srcShare {
		fs.Debugf(src, "Can't move - must be on the same share")
		return fs.ErrorCantDirMove
	}

	err = f.ensureDirectory(ctx, dstShare, dstPath)
	if err != nil {
		return fmt.Errorf("failed to make parent directories: %w", err)
	}

	cn, err := f.getConnection(ctx, dstShare)
	if err != nil {
		return err
	}
	defer f.putConnection(&cn)

	_, err = cn.smbShare.Stat(dstPath)
	if os.IsNotExist(err) {
		err = cn.smbShare.Rename(f.toSambaPath(srcPath), f.toSambaPath(dstPath))
		return translateError(err, true)
	}
	return fs.ErrorDirExists
}

// List files and directories in a directory
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	share, _path := f.split(dir)

	cn, err := f.getConnection(ctx, share)
	if err != nil {
		return nil, err
	}
	defer f.putConnection(&cn)

	if share == "" {
		shares, err := cn.smbSession.ListSharenames()
		for _, shh := range shares {
			shh = f.toNativePath(shh)
			if strings.HasSuffix(shh, "$") && f.opt.HideSpecial {
				continue
			}
			entries = append(entries, fs.NewDir(shh, time.Time{}))
		}
		return entries, err
	}

	dirents, err := cn.smbShare.ReadDir(f.toSambaPath(_path))
	if err != nil {
		return entries, translateError(err, true)
	}
	for _, file := range dirents {
		nfn := f.toNativePath(file.Name())
		if file.IsDir() {
			entries = append(entries, fs.NewDir(path.Join(dir, nfn), file.ModTime()))
		} else {
			entries = append(entries, f.makeEntryRelative(share, _path, nfn, file))
		}
	}

	return entries, nil
}

// About returns things about remaining and used spaces
func (f *Fs) About(ctx context.Context) (_ *fs.Usage, err error) {
	share, dir := f.split("/")
	if share == "" {
		// Just return empty info rather than an error if called on the root
		return &fs.Usage{}, nil
	}
	dir = f.toSambaPath(dir)

	cn, err := f.getConnection(ctx, share)
	if err != nil {
		return nil, err
	}
	stat, err := cn.smbShare.Statfs(dir)
	f.putConnection(&cn)
	if err != nil {
		return nil, err
	}

	bs := int64(stat.BlockSize())
	usage := &fs.Usage{
		Total: fs.NewUsageValue(bs * int64(stat.TotalBlockCount())),
		Used:  fs.NewUsageValue(bs * int64(stat.TotalBlockCount()-stat.FreeBlockCount())),
		Free:  fs.NewUsageValue(bs * int64(stat.AvailableBlockCount())),
	}
	return usage, nil
}

// OpenWriterAt opens with a handle for random access writes
//
// Pass in the remote desired and the size if known.
//
// It truncates any existing object
func (f *Fs) OpenWriterAt(ctx context.Context, remote string, size int64) (fs.WriterAtCloser, error) {
	var err error
	o := &Object{
		fs:     f,
		remote: remote,
	}
	share, filename := o.split()
	if share == "" || filename == "" {
		return nil, fs.ErrorIsDir
	}

	err = o.fs.ensureDirectory(ctx, share, filename)
	if err != nil {
		return nil, fmt.Errorf("failed to make parent directories: %w", err)
	}

	filename = o.fs.toSambaPath(filename)

	o.fs.addSession() // Show session in use
	defer o.fs.removeSession()

	cn, err := o.fs.getConnection(ctx, share)
	if err != nil {
		return nil, err
	}

	fl, err := cn.smbShare.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %w", err)
	}

	return fl, nil
}

// Shutdown the backend, closing any background tasks and any
// cached connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	return f.drainPool(ctx)
}

func (f *Fs) makeEntry(share, _path string, stat os.FileInfo) *Object {
	remote := path.Join(share, _path)
	return &Object{
		fs:         f,
		remote:     trimPathPrefix(remote, f.root),
		statResult: stat,
	}
}

func (f *Fs) makeEntryRelative(share, _path, relative string, stat os.FileInfo) *Object {
	return f.makeEntry(share, path.Join(_path, relative), stat)
}

func (f *Fs) ensureDirectory(ctx context.Context, share, _path string) error {
	dir := path.Dir(_path)
	if dir == "." {
		return nil
	}
	cn, err := f.getConnection(ctx, share)
	if err != nil {
		return err
	}
	err = cn.smbShare.MkdirAll(f.toSambaPath(dir), 0o755)
	f.putConnection(&cn)
	return err
}

/// Object

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// ModTime is the last modified time (read-only)
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.statResult.ModTime()
}

// Size is the file length
func (o *Object) Size() int64 {
	return o.statResult.Size()
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash always returns empty value
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets modTime on a particular file
func (o *Object) SetModTime(ctx context.Context, t time.Time) (err error) {
	share, reqDir := o.split()
	if share == "" || reqDir == "" {
		return fs.ErrorCantSetModTime
	}
	reqDir = o.fs.toSambaPath(reqDir)

	cn, err := o.fs.getConnection(ctx, share)
	if err != nil {
		return err
	}
	defer o.fs.putConnection(&cn)

	err = cn.smbShare.Chtimes(reqDir, t, t)
	if err != nil {
		return err
	}

	fi, err := cn.smbShare.Stat(reqDir)
	if err == nil {
		o.statResult = fi
	}
	return err
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	share, filename := o.split()
	if share == "" || filename == "" {
		return nil, fs.ErrorIsDir
	}
	filename = o.fs.toSambaPath(filename)

	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.Size())
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}

	o.fs.addSession() // Show session in use
	defer o.fs.removeSession()

	cn, err := o.fs.getConnection(ctx, share)
	if err != nil {
		return nil, err
	}
	fl, err := cn.smbShare.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		o.fs.putConnection(&cn)
		return nil, fmt.Errorf("failed to open: %w", err)
	}
	pos, err := fl.Seek(offset, io.SeekStart)
	if err != nil {
		o.fs.putConnection(&cn)
		return nil, fmt.Errorf("failed to seek: %w", err)
	}
	if pos != offset {
		o.fs.putConnection(&cn)
		return nil, fmt.Errorf("failed to seek: wrong position (expected=%d, reported=%d)", offset, pos)
	}

	in = readers.NewLimitedReadCloser(fl, limit)
	in = &boundReadCloser{
		rc: in,
		close: func() error {
			o.fs.putConnection(&cn)
			return nil
		},
	}

	return in, nil
}

// Update the Object from in with modTime and size
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	share, filename := o.split()
	if share == "" || filename == "" {
		return fs.ErrorIsDir
	}

	err = o.fs.ensureDirectory(ctx, share, filename)
	if err != nil {
		return fmt.Errorf("failed to make parent directories: %w", err)
	}

	filename = o.fs.toSambaPath(filename)

	o.fs.addSession() // Show session in use
	defer o.fs.removeSession()

	cn, err := o.fs.getConnection(ctx, share)
	if err != nil {
		return err
	}
	defer func() {
		o.statResult, _ = cn.smbShare.Stat(filename)
		o.fs.putConnection(&cn)
	}()

	fl, err := cn.smbShare.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open: %w", err)
	}

	// remove the file if upload failed
	remove := func() {
		// Windows doesn't allow removal of files without closing file
		removeErr := fl.Close()
		if removeErr != nil {
			fs.Debugf(src, "failed to close the file for delete: %v", removeErr)
			// try to remove the file anyway; the file may be already closed
		}

		removeErr = cn.smbShare.Remove(filename)
		if removeErr != nil {
			fs.Debugf(src, "failed to remove: %v", removeErr)
		} else {
			fs.Debugf(src, "removed after failed upload: %v", err)
		}
	}

	_, err = fl.ReadFrom(in)
	if err != nil {
		remove()
		return fmt.Errorf("Update ReadFrom failed: %w", err)
	}

	err = fl.Close()
	if err != nil {
		remove()
		return fmt.Errorf("Update Close failed: %w", err)
	}

	// Set the modified time
	err = o.SetModTime(ctx, src.ModTime(ctx))
	if err != nil {
		return fmt.Errorf("Update SetModTime failed: %w", err)
	}

	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) (err error) {
	share, filename := o.split()
	if share == "" || filename == "" {
		return fs.ErrorIsDir
	}
	filename = o.fs.toSambaPath(filename)

	cn, err := o.fs.getConnection(ctx, share)
	if err != nil {
		return err
	}

	err = cn.smbShare.Remove(filename)
	o.fs.putConnection(&cn)

	return err
}

// String converts this Object to a string
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

/// Misc

// split returns share name and path in the share from the rootRelativePath
// relative to f.root
func (f *Fs) split(rootRelativePath string) (shareName, filepath string) {
	return bucket.Split(path.Join(f.root, rootRelativePath))
}

// split returns share name and path in the share from the object
func (o *Object) split() (shareName, filepath string) {
	return o.fs.split(o.remote)
}

func (f *Fs) toSambaPath(path string) string {
	// 1. encode via Rclone's escaping system
	// 2. convert to backslash-separated path
	return strings.ReplaceAll(f.opt.Enc.FromStandardPath(path), "/", "\\")
}

func (f *Fs) toNativePath(path string) string {
	// 1. convert *back* to slash-separated path
	// 2. encode via Rclone's escaping system
	return f.opt.Enc.ToStandardPath(strings.ReplaceAll(path, "\\", "/"))
}

func ensureSuffix(s, suffix string) string {
	if strings.HasSuffix(s, suffix) {
		return s
	}
	return s + suffix
}

func trimPathPrefix(s, prefix string) string {
	// we need to clean the paths to make tests pass!
	s = betterPathClean(s)
	prefix = betterPathClean(prefix)
	if s == prefix || s == prefix+"/" {
		return ""
	}
	prefix = ensureSuffix(prefix, "/")
	return strings.TrimPrefix(s, prefix)
}

func betterPathClean(p string) string {
	d := path.Clean(p)
	if d == "." {
		return ""
	}
	return d
}

type boundReadCloser struct {
	rc    io.ReadCloser
	close func() error
}

func (r *boundReadCloser) Read(p []byte) (n int, err error) {
	return r.rc.Read(p)
}

func (r *boundReadCloser) Close() error {
	err1 := r.rc.Close()
	err2 := r.close()
	if err1 != nil {
		return err1
	}
	return err2
}

func translateError(e error, dir bool) error {
	if os.IsNotExist(e) {
		if dir {
			return fs.ErrorDirNotFound
		}
		return fs.ErrorObjectNotFound
	}

	return e
}

var (
	_ fs.Fs          = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.Mover       = &Fs{}
	_ fs.DirMover    = &Fs{}
	_ fs.Abouter     = &Fs{}
	_ fs.Shutdowner  = &Fs{}
	_ fs.Object      = &Object{}
	_ io.ReadCloser  = &boundReadCloser{}
)
