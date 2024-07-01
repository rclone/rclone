//go:build !plan9

package hdfs

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	"github.com/colinmarc/hdfs/v2"
	krb "github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/credentials"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/pacer"
)

// Fs represents a HDFS server
type Fs struct {
	name     string
	root     string
	features *fs.Features   // optional features
	opt      Options        // options for this backend
	ci       *fs.ConfigInfo // global config
	client   *hdfs.Client
	pacer    *fs.Pacer // pacer for API calls
}

const (
	minSleep      = 20 * time.Millisecond
	maxSleep      = 10 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
)

// copy-paste from https://github.com/colinmarc/hdfs/blob/master/cmd/hdfs/kerberos.go
func getKerberosClient() (*krb.Client, error) {
	configPath := os.Getenv("KRB5_CONFIG")
	if configPath == "" {
		configPath = "/etc/krb5.conf"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	// Determine the ccache location from the environment, falling back to the
	// default location.
	ccachePath := os.Getenv("KRB5CCNAME")
	if strings.Contains(ccachePath, ":") {
		if strings.HasPrefix(ccachePath, "FILE:") {
			ccachePath = strings.SplitN(ccachePath, ":", 2)[1]
		} else {
			return nil, fmt.Errorf("unusable ccache: %s", ccachePath)
		}
	} else if ccachePath == "" {
		u, err := user.Current()
		if err != nil {
			return nil, err
		}

		ccachePath = fmt.Sprintf("/tmp/krb5cc_%s", u.Uid)
	}

	ccache, err := credentials.LoadCCache(ccachePath)
	if err != nil {
		return nil, err
	}

	client, err := krb.NewFromCCache(ccache, cfg)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	options := hdfs.ClientOptions{
		Addresses:           opt.Namenode,
		UseDatanodeHostname: false,
	}

	if opt.ServicePrincipalName != "" {
		options.KerberosClient, err = getKerberosClient()
		if err != nil {
			return nil, fmt.Errorf("problem with kerberos authentication: %w", err)
		}
		options.KerberosServicePrincipleName = opt.ServicePrincipalName

		if opt.DataTransferProtection != "" {
			options.DataTransferProtection = opt.DataTransferProtection
		}
	} else {
		options.User = opt.Username
	}

	client, err := hdfs.NewClient(options)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:   name,
		root:   root,
		opt:    *opt,
		ci:     fs.GetConfig(ctx),
		client: client,
		pacer:  fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	info, err := f.client.Stat(f.realpath(""))
	if err == nil && !info.IsDir() {
		f.root = path.Dir(f.root)
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// Name of this fs
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("hdfs://%s/%s", f.opt.Namenode, f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes are not supported
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// NewObject finds file at remote or return fs.ErrorObjectNotFound
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	realpath := f.realpath(remote)
	fs.Debugf(f, "new [%s]", realpath)

	info, err := f.ensureFile(realpath)
	if err != nil {
		return nil, err
	}

	return &Object{
		fs:      f,
		remote:  remote,
		size:    info.Size(),
		modTime: info.ModTime(),
	}, nil
}

// List the objects and directories in dir into entries.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	realpath := f.realpath(dir)
	fs.Debugf(f, "list [%s]", realpath)

	err = f.ensureDirectory(realpath)
	if err != nil {
		return nil, err
	}

	list, err := f.client.ReadDir(realpath)
	if err != nil {
		return nil, err
	}
	for _, x := range list {
		stdName := f.opt.Enc.ToStandardName(x.Name())
		remote := path.Join(dir, stdName)
		if x.IsDir() {
			entries = append(entries, fs.NewDir(remote, x.ModTime()))
		} else {
			entries = append(entries, &Object{
				fs:      f,
				remote:  remote,
				size:    x.Size(),
				modTime: x.ModTime(),
			})
		}
	}
	return entries, nil
}

// Put the object
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	err := o.Update(ctx, in, src, options...)
	return o, err
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir makes a directory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	fs.Debugf(f, "mkdir [%s]", f.realpath(dir))
	return f.client.MkdirAll(f.realpath(dir), 0755)
}

// Rmdir deletes the directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	realpath := f.realpath(dir)
	fs.Debugf(f, "rmdir [%s]", realpath)

	err := f.ensureDirectory(realpath)
	if err != nil {
		return err
	}

	// do not remove empty directory
	list, err := f.client.ReadDir(realpath)
	if err != nil {
		return err
	}
	if len(list) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	return f.client.Remove(realpath)
}

// Purge deletes all the files in the directory
func (f *Fs) Purge(ctx context.Context, dir string) error {
	realpath := f.realpath(dir)
	fs.Debugf(f, "purge [%s]", realpath)

	err := f.ensureDirectory(realpath)
	if err != nil {
		return err
	}

	return f.client.RemoveAll(realpath)
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
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Get the real paths from the remote specs:
	sourcePath := srcObj.fs.realpath(srcObj.remote)
	targetPath := f.realpath(remote)
	fs.Debugf(f, "rename [%s] to [%s]", sourcePath, targetPath)

	// Make sure the target folder exists:
	dirname := path.Dir(targetPath)
	err := f.client.MkdirAll(dirname, 0755)
	if err != nil {
		return nil, err
	}

	// Do the move
	// Note that the underlying HDFS library hard-codes Overwrite=True, but this is expected rclone behaviour.
	err = f.client.Rename(sourcePath, targetPath)
	if err != nil {
		return nil, err
	}

	// Look up the resulting object
	info, err := f.client.Stat(targetPath)
	if err != nil {
		return nil, err
	}

	// And return it:
	return &Object{
		fs:      f,
		remote:  remote,
		size:    info.Size(),
		modTime: info.ModTime(),
	}, nil
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
	srcFs, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}

	// Get the real paths from the remote specs:
	sourcePath := srcFs.realpath(srcRemote)
	targetPath := f.realpath(dstRemote)
	fs.Debugf(f, "rename [%s] to [%s]", sourcePath, targetPath)

	// Check if the destination exists:
	info, err := f.client.Stat(targetPath)
	if err == nil {
		fs.Debugf(f, "target directory already exits, IsDir = [%t]", info.IsDir())
		return fs.ErrorDirExists
	}

	// Make sure the targets parent folder exists:
	dirname := path.Dir(targetPath)
	err = f.client.MkdirAll(dirname, 0755)
	if err != nil {
		return err
	}

	// Do the move
	err = f.client.Rename(sourcePath, targetPath)
	if err != nil {
		return err
	}

	return nil
}

// About gets quota information from the Fs
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	info, err := f.client.StatFs()
	if err != nil {
		return nil, err
	}
	return &fs.Usage{
		Total: fs.NewUsageValue(int64(info.Capacity)),
		Used:  fs.NewUsageValue(int64(info.Used)),
		Free:  fs.NewUsageValue(int64(info.Remaining)),
	}, nil
}

func (f *Fs) ensureDirectory(realpath string) error {
	info, err := f.client.Stat(realpath)

	if e, ok := err.(*os.PathError); ok && e.Err == os.ErrNotExist {
		return fs.ErrorDirNotFound
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fs.ErrorDirNotFound
	}

	return nil
}

func (f *Fs) ensureFile(realpath string) (os.FileInfo, error) {
	info, err := f.client.Stat(realpath)

	if e, ok := err.(*os.PathError); ok && e.Err == os.ErrNotExist {
		return nil, fs.ErrorObjectNotFound
	}
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fs.ErrorObjectNotFound
	}

	return info, nil
}

func (f *Fs) realpath(dir string) string {
	return f.opt.Enc.FromStandardPath(xPath(f.Root(), dir))
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = (*Fs)(nil)
	_ fs.Purger      = (*Fs)(nil)
	_ fs.PutStreamer = (*Fs)(nil)
	_ fs.Abouter     = (*Fs)(nil)
	_ fs.Mover       = (*Fs)(nil)
	_ fs.DirMover    = (*Fs)(nil)
)
