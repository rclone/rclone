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

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {

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

func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	remotepath := path.Join(f.root, dir)
	opRequest := sdk.OperationRequest{
		OperationType: constants.FileOperationCreateDir,
		RemotePath:    remotepath,
	}
	err = f.alloc.DoMultiOperation([]sdk.OperationRequest{opRequest})
	return err
}

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

func getDefaultConfigDir() (string, error) {
	homeDir, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(homeDir, ".zcn")

	return configDir, nil
}
