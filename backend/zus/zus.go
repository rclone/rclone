package zus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/core/client"
	"github.com/0chain/gosdk/core/conf"
	"github.com/0chain/gosdk/zboxcore/sdk"
	"github.com/0chain/gosdk/zcncore"
	"github.com/mitchellh/go-homedir"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
)

type Options struct {
	AllocationID string `config:"allocation_id"`
	ConfigDir    string `config:"config_dir"`
	Encrypt      bool   `config:"encrypt"`
	WorkDir      string `config:"work_dir"`
}

type Fs struct {
	name string //name of the remote
	root string //root of the remote

	opts     Options      // parsed options
	features *fs.Features // optional features
	alloc    *sdk.Allocation
}

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "zus",
		Description: "Zus Decentralized Storage",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name: "allocation_id",
				Help: "Allocation ID to use for this remote",
			},
			{
				Name:    "config_dir",
				Help:    "Directory where the configuration files are stored",
				Default: nil,
			},
			{
				Name:    "work_dir",
				Help:    "Directory where the work files are stored",
				Default: nil,
			},
			{
				Name:    "encrypt",
				Help:    "Encrypt the data before uploading",
				Default: false,
			},
		},
	})
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {

	if root == "" {
		root = "/"
	}

	if root[0] != '/' {
		return nil, errors.New("root must start with '/'")
	}
	root = path.Clean(root)

	f := &Fs{
		name: name,
		root: root,
	}

	// Parse config into Options struct
	err := configstruct.Set(m, &f.opts)
	if err != nil {
		return nil, err
	}

	if f.opts.ConfigDir == "" {
		f.opts.ConfigDir, err = getDefaultConfigDir()
		if err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(f.opts.ConfigDir); err != nil {
		return nil, err
	}

	if f.opts.WorkDir == "" {
		f.opts.WorkDir, err = homedir.Dir()
		if err != nil {
			return nil, err
		}
	}

	if f.opts.AllocationID == "" {
		allocFile := filepath.Join(f.opts.ConfigDir, "allocation.txt")
		allocBytes, err := os.ReadFile(allocFile)
		if err != nil {
			return nil, err
		}

		allocationID := strings.ReplaceAll(string(allocBytes), " ", "")
		allocationID = strings.ReplaceAll(allocationID, "\n", "")

		if len(allocationID) != 64 {
			return nil, fmt.Errorf("allocation id has length %d, should be 64", len(allocationID))
		}
		f.opts.AllocationID = allocationID
	}

	cfg, err := conf.LoadConfigFile(filepath.Join(f.opts.ConfigDir, "config.yaml"))
	if err != nil {
		return nil, err
	}
	var walletInfo string
	walletFile := filepath.Join(f.opts.ConfigDir, "wallet.json")

	walletBytes, err := os.ReadFile(walletFile)
	if err != nil {
		return nil, err
	}
	walletInfo = string(walletBytes)
	err = client.InitSDK("{}", cfg.BlockWorker, cfg.ChainID, cfg.SignatureScheme, 0, true, cfg.MinSubmit, cfg.MinConfirmation, cfg.ConfirmationChainLength, cfg.SharderConsensous)
	if err != nil {
		return nil, err
	}
	conf.InitClientConfig(&cfg)

	err = zcncore.SetGeneralWalletInfo(walletInfo, cfg.SignatureScheme)
	if err != nil {
		return nil, err
	}

	if client.GetClient().IsSplit {
		zcncore.RegisterZauthServer(cfg.ZauthServer)
	}
	sdk.SetNumBlockDownloads(100)
	allocation, err := sdk.GetAllocation(f.opts.AllocationID)
	if err != nil {
		return nil, err
	}
	f.alloc = allocation
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

// String returns a description of the FS
func (f *Fs) String() string {
	return fmt.Sprintf("FS zus://%s", f.root)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// Hashes are not exposed anywhere
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
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
	remotepath := path.Join(f.root, dir)
	level := len(strings.Split(strings.TrimSuffix(remotepath, "/"), "/"))
	oREsult, err := f.alloc.GetRefs(remotepath, "", "", "", "", "regular", level, 1)
	if err != nil {
		return nil, err
	}
	if len(oREsult.Refs) == 0 {
		return nil, fs.ErrorDirNotFound
	}
	res := f.alloc.ListObjects(ctx, remotepath, "", "", "", "", "regular", level+1, 1000)

	for child := range res {
		var entry fs.DirEntry
		if child.Err != nil {
			return nil, child.Err
		}
		if child.Type == "d" {
			sep := "/"
			if f.root == "/" {
				sep = ""
			}
			entry = fs.NewDir(strings.TrimPrefix(child.Path, f.root+sep), child.UpdatedAt.ToTime())
		} else {
			mp := make(map[string]string)
			if child.CustomMeta != "" {
				err = json.Unmarshal([]byte(child.CustomMeta), &mp)
				if err != nil {
					return nil, err
				}
			}
			modTime := child.UpdatedAt.ToTime()
			t, ok := mp["rclone:mtime"]
			if ok {
				// try to parse the time
				tm, err := time.Parse(time.RFC3339Nano, t)
				if err == nil {
					modTime = tm
				}
			}
			entry = &Object{
				fs:        f,
				remote:    child.Path,
				modTime:   modTime,
				size:      child.Size,
				encrypted: child.EncryptedKey != "",
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (o fs.Object, err error) {
	remote = strings.TrimPrefix(remote, "/")
	remotepath := path.Join(f.root, remote)
	level := len(strings.Split(strings.TrimSuffix(remotepath, "/"), "/"))
	oREsult, err := f.alloc.GetRefs(remotepath, "", "", "", "", "regular", level, 1)
	if err != nil {
		return nil, err
	}
	if len(oREsult.Refs) == 0 {
		return nil, fs.ErrorObjectNotFound
	}
	ref := oREsult.Refs[0]
	modTime := ref.UpdatedAt.ToTime()
	mp := make(map[string]string)
	if ref.CustomMeta != "" {
		err = json.Unmarshal([]byte(ref.CustomMeta), &mp)
		if err != nil {
			return nil, err
		}
		t, ok := mp["rclone:mtime"]
		if ok {
			// try to parse the time
			tm, err := time.Parse(time.RFC3339Nano, t)
			if err == nil {
				modTime = tm
			}
		}
	}
	return &Object{
		fs:        f,
		remote:    remote,
		modTime:   modTime,
		size:      ref.ActualFileSize,
		encrypted: ref.EncryptedKey != "",
	}, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	for _, option := range options {
		if option.Mandatory() {
			fs.Errorf(f, "Unsupported mandatory option: %v", option)

			return nil, errors.New("unsupported mandatory option")
		}
	}
	remotepath := path.Join(f.root, src.Remote())
	obj := &Object{
		fs:     f,
		remote: remotepath,
	}
	err := obj.put(ctx, in, src, false)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	remotepath := path.Join(f.root, dir)
	opRequest := sdk.OperationRequest{
		OperationType: constants.FileOperationCreateDir,
		RemotePath:    remotepath,
	}
	err = f.alloc.DoMultiOperation([]sdk.OperationRequest{opRequest})
	return err
}

// Rmdir deletes the given folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) (err error) {
	remotepath := path.Join(f.root, dir)
	level := len(strings.Split(strings.TrimSuffix(remotepath, "/"), "/"))
	oREsult, err := f.alloc.GetRefs(remotepath, "", "", "", "", "regular", level, 1)
	if err != nil {
		return err
	}
	if len(oREsult.Refs) == 0 {
		return fs.ErrorDirNotFound
	}
	if oREsult.Refs[0].Type != "d" {
		return fs.ErrorDirNotFound
	}
	oREsult, err = f.alloc.GetRefs(remotepath, "", "", "", "", "regular", level+1, 1)
	if err != nil {
		return err
	}
	if len(oREsult.Refs) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}
	opRequest := sdk.OperationRequest{
		OperationType: constants.FileOperationDelete,
		RemotePath:    remotepath,
	}
	err = f.alloc.DoMultiOperation([]sdk.OperationRequest{opRequest})
	return err
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	remotepath := path.Join(f.root, dir)
	level := len(strings.Split(strings.TrimSuffix(remotepath, "/"), "/"))
	oREsult, err := f.alloc.GetRefs(remotepath, "", "", "", "", "regular", level, 1)
	if err != nil {
		return err
	}
	if len(oREsult.Refs) == 0 {
		return fs.ErrorDirNotFound
	}
	if oREsult.Refs[0].Type != "d" {
		return fs.ErrorDirNotFound
	}
	opRequest := sdk.OperationRequest{
		OperationType: constants.FileOperationDelete,
		RemotePath:    remotepath,
	}
	err = f.alloc.DoMultiOperation([]sdk.OperationRequest{opRequest})
	return err
}

type Object struct {
	fs        *Fs
	remote    string
	modTime   time.Time
	size      int64
	encrypted bool
}

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}

	return o.Remote()
}

// Remote returns the remote path
func (o *Object) Remote() string {
	if o.fs.root == "/" {
		return o.remote
	}

	return strings.TrimPrefix(o.remote, o.fs.root+"/")
}

// ModTime returns the modification date of the file
// It should return a best guess if one isn't available
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size returns the size of the file
func (o *Object) Size() int64 {
	return o.size
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash returns the selected checksum of the file
// If no checksum is available it returns ""
func (o *Object) Hash(ctx context.Context, ty hash.Type) (_ string, err error) {
	return "", hash.ErrUnsupported
}

// Storable says whether this object can be stored
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the metadata on the object to set the modification date
func (o *Object) SetModTime(ctx context.Context, t time.Time) (err error) {
	return fs.ErrorCantSetModTime
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	var (
		rangeStart int64
		rangeEnd   int64 = -1
	)

	for _, option := range options {
		switch opt := option.(type) {
		case *fs.RangeOption:
			rangeStart = opt.Start
			rangeEnd = opt.End
		case *fs.SeekOption:
			if opt.Offset > 0 {
				rangeStart = opt.Offset
			} else {
				rangeStart = o.size + opt.Offset
			}
		default:
			if option.Mandatory() {
				fs.Errorf(o, "Unsupported mandatory option: %v", option)

				return nil, errors.New("unsupported mandatory option")
			}

		}
	}
	return o.fs.alloc.DownloadObject(ctx, o.remote, rangeStart, rangeEnd)
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	for _, option := range options {
		if option.Mandatory() {
			fs.Errorf(o.fs, "Unsupported mandatory option: %v", option)

			return errors.New("unsupported mandatory option")
		}
	}
	mp := make(map[string]string)
	modified := src.ModTime(ctx)
	mp["rclone:mtime"] = modified.Format(time.RFC3339)
	marshal, err := json.Marshal(mp)
	if err != nil {
		return err
	}
	fileMeta := sdk.FileMeta{
		Path:       "",
		RemotePath: o.remote,
		ActualSize: src.Size(),
		RemoteName: path.Base(o.remote),
		CustomMeta: string(marshal),
	}
	isStreamUpload := src.Size() == -1
	if isStreamUpload {
		fileMeta.ActualSize = 0
	}
	rb := &ReaderBytes{
		reader: in,
	}
	opRequest := sdk.OperationRequest{
		OperationType: constants.FileOperationUpdate,
		FileReader:    rb,
		Workdir:       o.fs.opts.WorkDir,
		RemotePath:    o.remote,
		FileMeta:      fileMeta,
		Opts: []sdk.ChunkedUploadOption{
			sdk.WithChunkNumber(120),
			sdk.WithEncrypt(o.fs.opts.Encrypt),
		},
		StreamUpload: isStreamUpload,
	}
	err = o.fs.alloc.DoMultiOperation([]sdk.OperationRequest{opRequest})
	if err != nil {
		return err
	}
	o.modTime = modified
	o.size = rb.size
	o.encrypted = o.fs.opts.Encrypt

	return nil
}

func (o *Object) put(ctx context.Context, in io.Reader, src fs.ObjectInfo, toUpdate bool) (err error) {
	mp := make(map[string]string)
	modified := src.ModTime(ctx)
	mp["rclone:mtime"] = modified.Format(time.RFC3339)
	marshal, err := json.Marshal(mp)
	if err != nil {
		return err
	}
	fileMeta := sdk.FileMeta{
		Path:       "",
		RemotePath: o.remote,
		ActualSize: src.Size(),
		RemoteName: path.Base(o.remote),
		CustomMeta: string(marshal),
	}
	isStreamUpload := src.Size() == -1
	if isStreamUpload {
		fileMeta.ActualSize = 0
	}
	rb := &ReaderBytes{
		reader: in,
	}
	opRequest := sdk.OperationRequest{
		OperationType: constants.FileOperationInsert,
		FileReader:    rb,
		Workdir:       o.fs.opts.WorkDir,
		RemotePath:    o.remote,
		FileMeta:      fileMeta,
		Opts: []sdk.ChunkedUploadOption{
			sdk.WithChunkNumber(120),
			sdk.WithEncrypt(o.fs.opts.Encrypt),
		},
		StreamUpload: isStreamUpload,
	}
	if toUpdate {
		opRequest.OperationType = constants.FileOperationUpdate
	}
	err = o.fs.alloc.DoMultiOperation([]sdk.OperationRequest{opRequest})
	if err != nil {
		return err
	}
	o.modTime = modified
	o.size = rb.size
	o.encrypted = o.fs.opts.Encrypt
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) (err error) {
	opRequest := sdk.OperationRequest{
		OperationType: constants.FileOperationDelete,
		RemotePath:    o.remote,
	}
	err = o.fs.alloc.DoMultiOperation([]sdk.OperationRequest{opRequest})
	return err
}

type ReaderBytes struct {
	reader io.Reader
	size   int64
}

func (r *ReaderBytes) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	r.size += int64(n)
	return n, nil
}

func getDefaultConfigDir() (string, error) {
	homeDir, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(homeDir, ".zcn")

	return configDir, nil
}
