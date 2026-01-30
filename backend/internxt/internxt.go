// Package internxt provides an interface to Internxt's Drive API
package internxt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/internxt/rclone-adapter/auth"
	"github.com/internxt/rclone-adapter/buckets"
	config "github.com/internxt/rclone-adapter/config"
	sdkerrors "github.com/internxt/rclone-adapter/errors"
	"github.com/internxt/rclone-adapter/files"
	"github.com/internxt/rclone-adapter/folders"
	"github.com/internxt/rclone-adapter/users"
	"github.com/rclone/rclone/fs"
	rclone_config "github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
)

// shouldRetry determines if an error should be retried
func shouldRetry(ctx context.Context, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	var httpErr *sdkerrors.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode() == 401 {
		return true, err
	}

	return fserrors.ShouldRetry(err), err
}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "internxt",
		Description: "Internxt Drive",
		NewFs:       NewFs,
		Config:      Config,
		Options: []fs.Option{{
			Name:      "email",
			Help:      "Email of your Internxt account.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:       "pass",
			Help:       "Password.",
			Required:   true,
			IsPassword: true,
		}, {
			Name:      "mnemonic",
			Help:      "Mnemonic (internal use only)",
			Required:  false,
			Advanced:  true,
			Sensitive: true,
			Hide:      fs.OptionHideBoth,
		}, {
			Name:     "skip_hash_validation",
			Default:  true,
			Advanced: true,
			Help:     "Skip hash validation when downloading files.\n\nBy default, hash validation is disabled. Set this to false to enable validation.",
		}, {
			Name:     rclone_config.ConfigEncoding,
			Help:     rclone_config.ConfigEncodingHelp,
			Advanced: true,
			Default: encoder.EncodeInvalidUtf8 |
				encoder.EncodeSlash |
				encoder.EncodeBackSlash |
				encoder.EncodeRightPeriod |
				encoder.EncodeDot |
				encoder.EncodeCrLf,
		}},
	})
}

// Config configures the Internxt remote by performing login
func Config(ctx context.Context, name string, m configmap.Mapper, configIn fs.ConfigIn) (*fs.ConfigOut, error) {
	email, _ := m.Get("email")
	if email == "" {
		return nil, errors.New("email is required")
	}

	pass, _ := m.Get("pass")
	if pass != "" {
		var err error
		pass, err = obscure.Reveal(pass)
		if err != nil {
			return nil, fmt.Errorf("couldn't decrypt password: %w", err)
		}
	}

	cfg := config.NewDefaultToken("")
	cfg.HTTPClient = fshttp.NewClient(ctx)

	switch configIn.State {
	case "":
		// Check if 2FA is required
		loginResp, err := auth.Login(ctx, cfg, email)
		if err != nil {
			return nil, fmt.Errorf("failed to check login requirements: %w", err)
		}

		if loginResp.TFA {
			return fs.ConfigInput("2fa", "config_2fa", "Two-factor authentication code")
		}

		// No 2FA required, do login directly
		return fs.ConfigGoto("login")

	case "2fa":
		twoFA := configIn.Result
		if twoFA == "" {
			return fs.ConfigError("", "2FA code is required")
		}
		m.Set("2fa_code", twoFA)
		return fs.ConfigGoto("login")

	case "login":
		twoFA, _ := m.Get("2fa_code")

		loginResp, err := auth.DoLogin(ctx, cfg, email, pass, twoFA)
		if err != nil {
			return nil, fmt.Errorf("login failed: %w", err)
		}

		// Store mnemonic (obscured)
		m.Set("mnemonic", obscure.MustObscure(loginResp.User.Mnemonic))

		// Store token
		oauthToken, err := jwtToOAuth2Token(loginResp.NewToken)
		if err != nil {
			return nil, fmt.Errorf("failed to parse token: %w", err)
		}
		err = oauthutil.PutToken(name, m, oauthToken, true)
		if err != nil {
			return nil, fmt.Errorf("failed to save token: %w", err)
		}

		// Clear temporary 2FA code
		m.Set("2fa_code", "")

		return nil, nil
	}

	return nil, fmt.Errorf("unknown state %q", configIn.State)
}

// Options defines the configuration for this backend
type Options struct {
	Email              string               `config:"email"`
	Pass               string               `config:"pass"`
	TwoFA              string               `config:"2fa"`
	Mnemonic           string               `config:"mnemonic"`
	SkipHashValidation bool                 `config:"skip_hash_validation"`
	Encoding           encoder.MultiEncoder `config:"encoding"`
}

// Fs represents an Internxt remote
type Fs struct {
	name         string
	root         string
	opt          Options
	dirCache     *dircache.DirCache
	cfg          *config.Config
	features     *fs.Features
	pacer        *fs.Pacer
	tokenRenewer *oauthutil.Renew
	bridgeUser   string
	userID       string
}

// Object holds the data for a remote file object
type Object struct {
	f       *Fs
	remote  string
	id      string
	uuid    string
	size    int64
	modTime time.Time
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string { return f.name }

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string { return f.root }

// String converts this Fs to a string
func (f *Fs) String() string { return fmt.Sprintf("Internxt root '%s'", f.root) }

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes returns type of hashes supported by Internxt
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet()
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}

	if opt.Mnemonic == "" {
		return nil, errors.New("mnemonic is required - please run: rclone config reconnect " + name + ":")
	}

	var err error
	opt.Mnemonic, err = obscure.Reveal(opt.Mnemonic)
	if err != nil {
		return nil, fmt.Errorf("couldn't decrypt mnemonic: %w", err)
	}

	oauthToken, err := oauthutil.GetToken(name, m)
	if err != nil {
		return nil, fmt.Errorf("failed to get token - please run: rclone config reconnect %s: - %w", name, err)
	}

	oauthConfig := &oauthutil.Config{
		TokenURL: "https://gateway.internxt.com/drive/users/refresh",
	}

	_, ts, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create oauth client: %w", err)
	}

	cfg := config.NewDefaultToken(oauthToken.AccessToken)
	cfg.Mnemonic = opt.Mnemonic
	cfg.SkipHashValidation = opt.SkipHashValidation
	cfg.HTTPClient = fshttp.NewClient(ctx)

	userInfo, err := getUserInfo(ctx, &userInfoConfig{Token: cfg.Token})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	cfg.RootFolderID = userInfo.RootFolderID
	cfg.Bucket = userInfo.Bucket
	cfg.BasicAuthHeader = computeBasicAuthHeader(userInfo.BridgeUser, userInfo.UserID)

	f := &Fs{
		name:       name,
		root:       strings.Trim(root, "/"),
		opt:        *opt,
		cfg:        cfg,
		bridgeUser: userInfo.BridgeUser,
		userID:     userInfo.UserID,
	}

	f.pacer = fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant)))

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	if ts != nil {
		f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
			err := refreshJWTToken(ctx, name, m)
			if err != nil {
				return err
			}

			newToken, err := oauthutil.GetToken(name, m)
			if err != nil {
				return fmt.Errorf("failed to get refreshed token: %w", err)
			}
			f.cfg.Token = newToken.AccessToken
			f.cfg.BasicAuthHeader = computeBasicAuthHeader(f.bridgeUser, f.userID)

			return nil
		})
		f.tokenRenewer.Start()
	}

	f.dirCache = dircache.New(f.root, cfg.RootFolderID, f)

	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it might be a file
		newRoot, remote := dircache.SplitPath(f.root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, f.cfg.RootFolderID, &tempF)
		tempF.root = newRoot

		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			return f, nil
		}

		_, err := tempF.NewObject(ctx, remote)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				return f, nil
			}
			return nil, err
		}

		f.dirCache = tempF.dirCache
		f.root = tempF.root
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// Mkdir creates a new directory
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	id, err := f.dirCache.FindDir(ctx, dir, true)
	if err != nil {
		return err
	}

	f.dirCache.Put(dir, id)

	return nil
}

// Rmdir removes a directory
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("cannot remove root directory")
	}

	id, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return fs.ErrorDirNotFound
	}

	// Check if directory is empty
	var childFolders []folders.Folder
	err = f.pacer.Call(func() (bool, error) {
		var err error
		childFolders, err = folders.ListAllFolders(ctx, f.cfg, id)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}
	if len(childFolders) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	var childFiles []folders.File
	err = f.pacer.Call(func() (bool, error) {
		var err error
		childFiles, err = folders.ListAllFiles(ctx, f.cfg, id)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}
	if len(childFiles) > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	// Delete the directory
	err = f.pacer.Call(func() (bool, error) {
		err := folders.DeleteFolder(ctx, f.cfg, id)
		if err != nil && strings.Contains(err.Error(), "404") {
			return false, fs.ErrorDirNotFound
		}
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// FindLeaf looks for a sub‑folder named `leaf` under the Internxt folder `pathID`.
// If found, it returns its UUID and true. If not found, returns "", false.
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (string, bool, error) {
	var entries []folders.Folder
	err := f.pacer.Call(func() (bool, error) {
		var err error
		entries, err = folders.ListAllFolders(ctx, f.cfg, pathID)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return "", false, err
	}
	for _, e := range entries {
		if f.opt.Encoding.ToStandardName(e.PlainName) == leaf {
			return e.UUID, true, nil
		}
	}
	return "", false, nil
}

// CreateDir creates a new directory
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (string, error) {
	request := folders.CreateFolderRequest{
		PlainName:        f.opt.Encoding.FromStandardName(leaf),
		ParentFolderUUID: pathID,
		ModificationTime: time.Now().UTC().Format(time.RFC3339),
	}

	var resp *folders.Folder
	err := f.pacer.CallNoRetry(func() (bool, error) {
		var err error
		resp, err = folders.CreateFolder(ctx, f.cfg, request)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		// If folder already exists (409 conflict), try to find it
		if strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "Conflict") {
			existingID, found, findErr := f.FindLeaf(ctx, pathID, leaf)
			if findErr == nil && found {
				fs.Debugf(f, "Folder %q already exists in %q, using existing UUID: %s", leaf, pathID, existingID)
				return existingID, nil
			}
		}
		return "", fmt.Errorf("can't create folder, %w", err)
	}

	return resp.UUID, nil
}

// preUploadCheck checks if a file exists in the given directory
// Returns the file metadata if it exists, nil if not
func (f *Fs) preUploadCheck(ctx context.Context, leaf, directoryID string) (*folders.File, error) {
	// Parse name and extension from the leaf
	baseName := f.opt.Encoding.FromStandardName(leaf)
	name := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	ext := strings.TrimPrefix(filepath.Ext(baseName), ".")

	checkResult, err := files.CheckFilesExistence(ctx, f.cfg, directoryID, []files.FileExistenceCheck{
		{
			PlainName:    name,
			Type:         ext,
			OriginalFile: struct{}{},
		},
	})

	if err != nil {
		// If existence check fails, assume file doesn't exist to allow upload to proceed
		return nil, nil
	}

	if len(checkResult.Files) > 0 && checkResult.Files[0].FileExists() {
		result := checkResult.Files[0]
		if result.Type != ext {
			return nil, nil
		}

		existingUUID := result.UUID
		if existingUUID != "" {
			fileMeta, err := files.GetFileMeta(ctx, f.cfg, existingUUID)
			if err == nil && fileMeta != nil {
				return convertFileMetaToFile(fileMeta), nil
			}

			if err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}

// convertFileMetaToFile converts files.FileMeta to folders.File
func convertFileMetaToFile(meta *files.FileMeta) *folders.File {
	// FileMeta and folders.File have compatible structures
	return &folders.File{
		ID:               meta.ID,
		UUID:             meta.UUID,
		FileID:           meta.FileID,
		PlainName:        meta.PlainName,
		Type:             meta.Type,
		Size:             meta.Size,
		Bucket:           meta.Bucket,
		FolderUUID:       meta.FolderUUID,
		EncryptVersion:   meta.EncryptVersion,
		ModificationTime: meta.ModificationTime,
	}
}

// List lists a directory
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	dirID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var out fs.DirEntries

	var foldersList []folders.Folder
	err = f.pacer.Call(func() (bool, error) {
		var err error
		foldersList, err = folders.ListAllFolders(ctx, f.cfg, dirID)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	for _, e := range foldersList {
		remote := filepath.Join(dir, f.opt.Encoding.ToStandardName(e.PlainName))
		out = append(out, fs.NewDir(remote, e.ModificationTime))
	}
	var filesList []folders.File
	err = f.pacer.Call(func() (bool, error) {
		var err error
		filesList, err = folders.ListAllFiles(ctx, f.cfg, dirID)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	for _, e := range filesList {
		remote := e.PlainName
		if len(e.Type) > 0 {
			remote += "." + e.Type
		}
		remote = filepath.Join(dir, f.opt.Encoding.ToStandardName(remote))
		out = append(out, newObjectWithFile(f, remote, &e))
	}
	return out, nil
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()

	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			o := &Object{
				f:       f,
				remote:  remote,
				size:    src.Size(),
				modTime: src.ModTime(ctx),
			}
			return o, o.Update(ctx, in, src, options...)
		}
		return nil, err
	}

	// Check if file already exists
	existingFile, err := f.preUploadCheck(ctx, leaf, directoryID)
	if err != nil {
		return nil, err
	}

	// Create object - if file exists, populate it with existing metadata
	o := &Object{
		f:       f,
		remote:  remote,
		size:    src.Size(),
		modTime: src.ModTime(ctx),
	}

	if existingFile != nil {
		// File exists - populate object with existing metadata
		size, _ := existingFile.Size.Int64()
		o.id = existingFile.FileID
		o.uuid = existingFile.UUID
		o.size = size
		o.modTime = existingFile.ModificationTime
	}

	return o, o.Update(ctx, in, src, options...)
}

// Remove removes an object
func (f *Fs) Remove(ctx context.Context, remote string) error {
	obj, err := f.NewObject(ctx, remote)
	if err == nil {
		if err := obj.Remove(ctx); err != nil {
			return err
		}
		parent := path.Dir(remote)
		f.dirCache.FlushDir(parent)
		return nil
	}

	dirID, err := f.dirCache.FindDir(ctx, remote, false)
	if err != nil {
		return err
	}
	err = f.pacer.Call(func() (bool, error) {
		err := folders.DeleteFolder(ctx, f.cfg, dirID)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}
	f.dirCache.FlushDir(remote)
	return nil
}

// NewObject creates a new object
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	parentDir := filepath.Dir(remote)

	if parentDir == "." {
		parentDir = ""
	}

	dirID, err := f.dirCache.FindDir(ctx, parentDir, false)
	if err != nil {
		return nil, fs.ErrorObjectNotFound
	}

	var files []folders.File
	err = f.pacer.Call(func() (bool, error) {
		var err error
		files, err = folders.ListAllFiles(ctx, f.cfg, dirID)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	targetName := filepath.Base(remote)
	for _, e := range files {
		name := e.PlainName
		if len(e.Type) > 0 {
			name += "." + e.Type
		}
		decodedName := f.opt.Encoding.ToStandardName(name)
		if decodedName == targetName {
			return newObjectWithFile(f, remote, &e), nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

// newObjectWithFile returns a new object by file info
func newObjectWithFile(f *Fs, remote string, file *folders.File) fs.Object {
	size, _ := file.Size.Int64()
	return &Object{
		f:       f,
		remote:  remote,
		id:      file.FileID,
		uuid:    file.UUID,
		size:    size,
		modTime: file.ModificationTime,
	}
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.f
}

// String returns the remote path
func (o *Object) String() string {
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Size is the file length
func (o *Object) Size() int64 {
	return o.size
}

// ModTime is the last modified time (read-only)
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Hash returns the hash value (not implemented)
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modified time
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	var internxtLimit *users.LimitResponse
	err := f.pacer.Call(func() (bool, error) {
		var err error
		internxtLimit, err = users.GetLimit(ctx, f.cfg)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}

	var internxtUsage *users.UsageResponse
	err = f.pacer.Call(func() (bool, error) {
		var err error
		internxtUsage, err = users.GetUsage(ctx, f.cfg)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}

	usage := &fs.Usage{
		Used: fs.NewUsageValue(internxtUsage.Drive),
	}

	usage.Total = fs.NewUsageValue(internxtLimit.MaxSpaceBytes)
	usage.Free = fs.NewUsageValue(*usage.Total - *usage.Used)

	return usage, nil
}

// Shutdown the backend, closing any background tasks and any cached
// connections.
func (f *Fs) Shutdown(ctx context.Context) error {
	buckets.WaitForPendingThumbnails()

	if f.tokenRenewer != nil {
		f.tokenRenewer.Shutdown()
	}
	return nil
}

// Open opens a file for streaming
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.FixRangeOption(options, o.size)
	rangeValue := ""
	for _, option := range options {
		switch option.(type) {
		case *fs.RangeOption, *fs.SeekOption:
			_, rangeValue = option.Header()
		}
	}

	if o.size == 0 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}

	var stream io.ReadCloser
	err := o.f.pacer.Call(func() (bool, error) {
		var err error
		stream, err = buckets.DownloadFileStream(ctx, o.f.cfg, o.id, rangeValue)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}
	return stream, nil
}

// Update updates an existing file or creates a new one
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	remote := o.remote

	origBaseName := filepath.Base(remote)
	origName := strings.TrimSuffix(origBaseName, filepath.Ext(origBaseName))
	origType := strings.TrimPrefix(filepath.Ext(origBaseName), ".")

	// Create directory if it doesn't exist
	_, dirID, err := o.f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	// rename based rollback pattern
	// old file is preserved until new upload succeeds

	var backupUUID string
	var backupName, backupType string
	oldUUID := o.uuid

	// Step 1: If file exists, rename to backup (preserves old file during upload)
	if oldUUID != "" {
		// Generate unique backup name
		baseName := filepath.Base(remote)
		name := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		ext := strings.TrimPrefix(filepath.Ext(baseName), ".")

		backupSuffix := fmt.Sprintf(".rclone-backup-%s", random.String(8))
		backupName = o.f.opt.Encoding.FromStandardName(name + backupSuffix)
		backupType = ext

		// Rename existing file to backup name
		err = o.f.pacer.Call(func() (bool, error) {
			err := files.RenameFile(ctx, o.f.cfg, oldUUID, backupName, backupType)
			if err != nil {
				// Handle 409 Conflict: Treat as success.
				var httpErr *sdkerrors.HTTPError
				if errors.As(err, &httpErr) && httpErr.StatusCode() == 409 {
					return false, nil
				}
			}
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return fmt.Errorf("failed to rename existing file to backup: %w", err)
		}
		backupUUID = oldUUID

		fs.Debugf(o.f, "Renamed existing file %s to backup %s.%s (UUID: %s)", remote, backupName, backupType, backupUUID)
	}

	var meta *buckets.CreateMetaResponse
	err = o.f.pacer.CallNoRetry(func() (bool, error) {
		var err error
		meta, err = buckets.UploadFileStreamAuto(ctx,
			o.f.cfg,
			dirID,
			o.f.opt.Encoding.FromStandardName(filepath.Base(remote)),
			in,
			src.Size(),
			src.ModTime(ctx),
		)
		return shouldRetry(ctx, err)
	})

	if err != nil && isEmptyFileLimitError(err) {
		o.restoreBackupFile(ctx, backupUUID, origName, origType)
		return fs.ErrorCantUploadEmptyFiles
	}

	if err != nil {
		meta, err = o.recoverFromTimeoutConflict(ctx, err, remote, dirID)
	}

	if err != nil {
		o.restoreBackupFile(ctx, backupUUID, origName, origType)
		return err
	}

	// Update object metadata
	o.uuid = meta.UUID
	o.id = meta.FileID
	o.size = src.Size()
	o.remote = remote

	// Step 3: Upload succeeded - delete the backup file
	if backupUUID != "" {
		fs.Debugf(o.f, "Upload succeeded, deleting backup file %s.%s (UUID: %s)", backupName, backupType, backupUUID)
		err := o.f.pacer.Call(func() (bool, error) {
			err := files.DeleteFile(ctx, o.f.cfg, backupUUID)
			if err != nil {
				var httpErr *sdkerrors.HTTPError
				if errors.As(err, &httpErr) {
					// Treat 404 (Not Found) and 204 (No Content) as success
					switch httpErr.StatusCode() {
					case 404, 204:
						return false, nil
					}
				}
			}
			return shouldRetry(ctx, err)
		})
		if err != nil {
			fs.Errorf(o.f, "Failed to delete backup file %s.%s (UUID: %s): %v. This may leave an orphaned backup file.",
				backupName, backupType, backupUUID, err)
			// Don't fail the upload just because backup deletion failed
		} else {
			fs.Debugf(o.f, "Successfully deleted backup file")
		}
	}

	return nil
}

// isTimeoutError checks if an error is a timeout using proper error type checking
func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

// isConflictError checks if an error indicates a file conflict (409)
func isConflictError(err error) bool {
	errMsg := err.Error()
	return strings.Contains(errMsg, "409") ||
		strings.Contains(errMsg, "Conflict") ||
		strings.Contains(errMsg, "already exists")
}

func isEmptyFileLimitError(err error) bool {
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "can not have more empty files") ||
		strings.Contains(errMsg, "cannot have more empty files") ||
		strings.Contains(errMsg, "you can not have empty files")
}

// recoverFromTimeoutConflict attempts to recover from a timeout or conflict error
func (o *Object) recoverFromTimeoutConflict(ctx context.Context, uploadErr error, remote, dirID string) (*buckets.CreateMetaResponse, error) {
	if !isTimeoutError(uploadErr) && !isConflictError(uploadErr) {
		return nil, uploadErr
	}

	baseName := filepath.Base(remote)
	encodedName := o.f.opt.Encoding.FromStandardName(baseName)

	var meta *buckets.CreateMetaResponse
	checkErr := o.f.pacer.Call(func() (bool, error) {
		existingFile, err := o.f.preUploadCheck(ctx, encodedName, dirID)
		if err != nil {
			return shouldRetry(ctx, err)
		}
		if existingFile != nil {
			name := strings.TrimSuffix(baseName, filepath.Ext(baseName))
			ext := strings.TrimPrefix(filepath.Ext(baseName), ".")

			meta = &buckets.CreateMetaResponse{
				UUID:      existingFile.UUID,
				FileID:    existingFile.FileID,
				Name:      name,
				PlainName: name,
				Type:      ext,
				Size:      existingFile.Size,
			}
			o.id = existingFile.FileID
		}
		return false, nil
	})

	if checkErr != nil {
		return nil, uploadErr
	}

	if meta != nil {
		return meta, nil
	}

	return nil, uploadErr
}

// restoreBackupFile restores a backup file after upload failure
func (o *Object) restoreBackupFile(ctx context.Context, backupUUID, origName, origType string) {
	if backupUUID == "" {
		return
	}

	_ = o.f.pacer.Call(func() (bool, error) {
		err := files.RenameFile(ctx, o.f.cfg, backupUUID,
			o.f.opt.Encoding.FromStandardName(origName), origType)
		return shouldRetry(ctx, err)
	})
}

// Remove deletes a file
func (o *Object) Remove(ctx context.Context) error {
	return o.f.pacer.Call(func() (bool, error) {
		err := files.DeleteFile(ctx, o.f.cfg, o.uuid)
		return shouldRetry(ctx, err)
	})
}
