// Package protondrive implements the Proton Drive backend
package protondrive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	protonDriveAPI "github.com/henrybear327/Proton-API-Bridge"
	"github.com/henrybear327/go-proton-api"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
)

/*
- dirCache operates on relative path to root
- path sanitization
	- rule of thumb: sanitize before use, but store things as-is
	- the paths cached in dirCache are after sanitizing
	- the remote/dir passed in aren't, and are stored as-is
*/

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential

	clientUIDKey           = "client_uid"
	clientAccessTokenKey   = "client_access_token"
	clientRefreshTokenKey  = "client_refresh_token"
	clientSaltedKeyPassKey = "client_salted_key_pass"
)

var (
	errCanNotUploadFileWithUnknownSize = errors.New("proton Drive can't upload files with unknown size")
	errCanNotPurgeRootDirectory        = errors.New("can't purge root directory")

	// for the auth/deauth handler
	_mapper        configmap.Mapper
	_saltedKeyPass string
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "protondrive",
		Description: "Proton Drive",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "username",
			Help:     `The username of your proton account`,
			Required: true,
		}, {
			Name:       "password",
			Help:       "The password of your proton account.",
			Required:   true,
			IsPassword: true,
		}, {
			Name: "mailbox_password",
			Help: `The mailbox password of your two-password proton account.

For more information regarding the mailbox password, please check the 
following official knowledge base article: 
https://proton.me/support/the-difference-between-the-mailbox-password-and-login-password
`,
			IsPassword: true,
			Advanced:   true,
		}, {
			Name: "2fa",
			Help: `The 2FA code

The value can also be provided with --protondrive-2fa=000000

The 2FA code of your proton drive account if the account is set up with 
two-factor authentication`,
			Required: false,
		}, {
			Name:      clientUIDKey,
			Help:      "Client uid key (internal use only)",
			Required:  false,
			Advanced:  true,
			Sensitive: true,
			Hide:      fs.OptionHideBoth,
		}, {
			Name:      clientAccessTokenKey,
			Help:      "Client access token key (internal use only)",
			Required:  false,
			Advanced:  true,
			Sensitive: true,
			Hide:      fs.OptionHideBoth,
		}, {
			Name:      clientRefreshTokenKey,
			Help:      "Client refresh token key (internal use only)",
			Required:  false,
			Advanced:  true,
			Sensitive: true,
			Hide:      fs.OptionHideBoth,
		}, {
			Name:      clientSaltedKeyPassKey,
			Help:      "Client salted key pass key (internal use only)",
			Required:  false,
			Advanced:  true,
			Sensitive: true,
			Hide:      fs.OptionHideBoth,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Base |
				encoder.EncodeInvalidUtf8 |
				encoder.EncodeLeftSpace |
				encoder.EncodeRightSpace),
		}, {
			Name: "original_file_size",
			Help: `Return the file size before encryption
			
The size of the encrypted file will be different from (bigger than) the 
original file size. Unless there is a reason to return the file size 
after encryption is performed, otherwise, set this option to true, as 
features like Open() which will need to be supplied with original content 
size, will fail to operate properly`,
			Advanced: true,
			Default:  true,
		}, {
			Name: "app_version",
			Help: `The app version string 

The app version string indicates the client that is currently performing 
the API request. This information is required and will be sent with every 
API request.`,
			Advanced: true,
			Default:  "macos-drive@1.0.0-alpha.1+rclone",
		}, {
			Name: "replace_existing_draft",
			Help: `Create a new revision when filename conflict is detected

When a file upload is cancelled or failed before completion, a draft will be 
created and the subsequent upload of the same file to the same location will be 
reported as a conflict.

The value can also be set by --protondrive-replace-existing-draft=true

If the option is set to true, the draft will be replaced and then the upload 
operation will restart. If there are other clients also uploading at the same 
file location at the same time, the behavior is currently unknown. Need to set 
to true for integration tests.
If the option is set to false, an error "a draft exist - usually this means a 
file is being uploaded at another client, or, there was a failed upload attempt" 
will be returned, and no upload will happen.`,
			Advanced: true,
			Default:  false,
		}, {
			Name: "enable_caching",
			Help: `Caches the files and folders metadata to reduce API calls

Notice: If you are mounting ProtonDrive as a VFS, please disable this feature, 
as the current implementation doesn't update or clear the cache when there are 
external changes. 

The files and folders on ProtonDrive are represented as links with keyrings, 
which can be cached to improve performance and be friendly to the API server.

The cache is currently built for the case when the rclone is the only instance 
performing operations to the mount point. The event system, which is the proton
API system that provides visibility of what has changed on the drive, is yet 
to be implemented, so updates from other clients won’t be reflected in the 
cache. Thus, if there are concurrent clients accessing the same mount point, 
then we might have a problem with caching the stale data.`,
			Advanced: true,
			Default:  true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Username        string `config:"username"`
	Password        string `config:"password"`
	MailboxPassword string `config:"mailbox_password"`
	TwoFA           string `config:"2fa"`

	// advanced
	Enc                  encoder.MultiEncoder `config:"encoding"`
	ReportOriginalSize   bool                 `config:"original_file_size"`
	AppVersion           string               `config:"app_version"`
	ReplaceExistingDraft bool                 `config:"replace_existing_draft"`
	EnableCaching        bool                 `config:"enable_caching"`
}

// Fs represents a remote proton drive
type Fs struct {
	name string // name of this remote
	// Notice that for ProtonDrive, it's attached under rootLink (usually /root)
	root        string                      // the path we are working on.
	opt         Options                     // parsed config options
	ci          *fs.ConfigInfo              // global config
	features    *fs.Features                // optional features
	pacer       *fs.Pacer                   // pacer for API calls
	dirCache    *dircache.DirCache          // Map of directory path to directory id
	protonDrive *protonDriveAPI.ProtonDrive // the Proton API bridging library
}

// Object describes an object
type Object struct {
	fs           *Fs       // what this object is part of
	remote       string    // The remote path (relative to the fs.root)
	size         int64     // size of the object (on server, after encryption)
	originalSize *int64    // size of the object (after decryption)
	digests      *string   // object original content
	blockSizes   []int64   // the block sizes of the encrypted file
	modTime      time.Time // modification time of the object
	createdTime  time.Time // creation time of the object
	id           string    // ID of the object
	mimetype     string    // mimetype of the file

	link *proton.Link // link data on proton server
}

// shouldRetry returns a boolean as to whether this err deserves to be
// retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, err error) (bool, error) {
	return false, err
}

//------------------------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.opt.Enc.ToStandardPath(f.root)
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("proton drive root link ID '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// run all the dir/remote through this
func (f *Fs) sanitizePath(_path string) string {
	_path = path.Clean(_path)
	if _path == "." || _path == "/" {
		return ""
	}

	return f.opt.Enc.FromStandardPath(_path)
}

func getConfigMap(m configmap.Mapper) (uid, accessToken, refreshToken, saltedKeyPass string, ok bool) {
	if accessToken, ok = m.Get(clientAccessTokenKey); !ok {
		return
	}

	if uid, ok = m.Get(clientUIDKey); !ok {
		return
	}

	if refreshToken, ok = m.Get(clientRefreshTokenKey); !ok {
		return
	}

	if saltedKeyPass, ok = m.Get(clientSaltedKeyPassKey); !ok {
		return
	}
	_saltedKeyPass = saltedKeyPass

	// empty strings are considered "ok" by m.Get, which is not true business-wise
	ok = accessToken != "" && uid != "" && refreshToken != "" && saltedKeyPass != ""

	return
}

func setConfigMap(m configmap.Mapper, uid, accessToken, refreshToken, saltedKeyPass string) {
	m.Set(clientUIDKey, uid)
	m.Set(clientAccessTokenKey, accessToken)
	m.Set(clientRefreshTokenKey, refreshToken)
	m.Set(clientSaltedKeyPassKey, saltedKeyPass)
	_saltedKeyPass = saltedKeyPass
}

func clearConfigMap(m configmap.Mapper) {
	setConfigMap(m, "", "", "", "")
	_saltedKeyPass = ""
}

func authHandler(auth proton.Auth) {
	// fs.Debugf("authHandler called")
	setConfigMap(_mapper, auth.UID, auth.AccessToken, auth.RefreshToken, _saltedKeyPass)
}

func deAuthHandler() {
	// fs.Debugf("deAuthHandler called")
	clearConfigMap(_mapper)
}

func newProtonDrive(ctx context.Context, f *Fs, opt *Options, m configmap.Mapper) (*protonDriveAPI.ProtonDrive, error) {
	config := protonDriveAPI.NewDefaultConfig()
	config.AppVersion = opt.AppVersion
	config.UserAgent = f.ci.UserAgent // opt.UserAgent

	config.ReplaceExistingDraft = opt.ReplaceExistingDraft
	config.EnableCaching = opt.EnableCaching

	// let's see if we have the cached access credential
	uid, accessToken, refreshToken, saltedKeyPass, hasUseReusableLoginCredentials := getConfigMap(m)
	_saltedKeyPass = saltedKeyPass

	if hasUseReusableLoginCredentials {
		fs.Debugf(f, "Has cached credentials")
		config.UseReusableLogin = true

		config.ReusableCredential.UID = uid
		config.ReusableCredential.AccessToken = accessToken
		config.ReusableCredential.RefreshToken = refreshToken
		config.ReusableCredential.SaltedKeyPass = saltedKeyPass

		protonDrive /* credential will be nil since access credentials are passed in */, _, err := protonDriveAPI.NewProtonDrive(ctx, config, authHandler, deAuthHandler)
		if err != nil {
			fs.Debugf(f, "Cached credential doesn't work, clearing and using the fallback login method")
			// clear the access token on failure
			clearConfigMap(m)

			fs.Debugf(f, "couldn't initialize a new proton drive instance using cached credentials: %v", err)
			// we fallback to username+password login -> don't throw an error here
			// return nil, fmt.Errorf("couldn't initialize a new proton drive instance: %w", err)
		} else {
			fs.Debugf(f, "Used cached credential to initialize the ProtonDrive API")
			return protonDrive, nil
		}
	}

	// if not, let's try to log the user in using username and password (and 2FA if required)
	fs.Debugf(f, "Using username and password to log in")
	config.UseReusableLogin = false
	config.FirstLoginCredential.Username = opt.Username
	config.FirstLoginCredential.Password = opt.Password
	config.FirstLoginCredential.MailboxPassword = opt.MailboxPassword
	config.FirstLoginCredential.TwoFA = opt.TwoFA
	protonDrive, auth, err := protonDriveAPI.NewProtonDrive(ctx, config, authHandler, deAuthHandler)
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize a new proton drive instance: %w", err)
	}

	fs.Debugf(f, "Used username and password to initialize the ProtonDrive API")
	setConfigMap(m, auth.UID, auth.AccessToken, auth.RefreshToken, auth.SaltedKeyPass)

	return protonDrive, nil
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// pacer is not used in NewFs()
	_mapper = m

	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	if opt.Password != "" {
		var err error
		opt.Password, err = obscure.Reveal(opt.Password)
		if err != nil {
			return nil, fmt.Errorf("couldn't decrypt password: %w", err)
		}
	}

	if opt.MailboxPassword != "" {
		var err error
		opt.MailboxPassword, err = obscure.Reveal(opt.MailboxPassword)
		if err != nil {
			return nil, fmt.Errorf("couldn't decrypt mailbox password: %w", err)
		}
	}

	ci := fs.GetConfig(ctx)

	root = strings.Trim(root, "/")

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		ci:    ci,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.features = (&fs.Features{
		ReadMimeType:            true,
		CanHaveEmptyDirectories: true,
		/* can't have multiple threads downloading
		The raw file is split into equally-sized (currently 4MB, but it might change in the future, say to 8MB, 16MB, etc.) blocks, except the last one which might be smaller than 4MB.
		Each block is encrypted separately, where the size and sha1 after the encryption is performed on the block is added to the metadata of the block, but the original block size and sha1 is not in the metadata.
		We can make assumption and implement the chunker, but for now, we would rather be safe about it, and let the block being concurrently downloaded and decrypted in the background, to speed up the download operation!
		*/
		NoMultiThreading: true,
	}).Fill(ctx, f)

	protonDrive, err := newProtonDrive(ctx, f, opt, m)
	if err != nil {
		return nil, err
	}
	f.protonDrive = protonDrive

	root = f.sanitizePath(root)
	f.dirCache = dircache.New(
		root,                         /* root folder path */
		protonDrive.MainShare.LinkID, /* real root ID is the root folder, since we can't go past this folder */
		f,
	)
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// if the root directory is not found, the initialization will still work
		// but if it's other kinds of error, then we raise it
		if err != fs.ErrorDirNotFound {
			return nil, fmt.Errorf("couldn't initialize a new root remote: %w", err)
		}

		// Assume it is a file (taken and modified from box.go)
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, protonDrive.MainShare.LinkID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.newObject(ctx, remote)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		f.features.Fill(ctx, &tempF)
		// XXX: update the old f here instead of returning tempF, since
		// `features` were already filled with functions having *f as a receiver.
		// See https://github.com/rclone/rclone/issues/2182
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}

	return f, nil
}

//------------------------------------------------------------------------------

// CleanUp deletes all files currently in trash
func (f *Fs) CleanUp(ctx context.Context) error {
	return f.pacer.Call(func() (bool, error) {
		err := f.protonDrive.EmptyTrash(ctx)
		return shouldRetry(ctx, err)
	})
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error ErrorObjectNotFound.
//
// If remote points to a directory then it should return
// ErrorIsDir if possible without doing any extra work,
// otherwise ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObject(ctx, remote)
}

func (f *Fs) getObjectLink(ctx context.Context, remote string) (*proton.Link, error) {
	// attempt to locate the file
	leaf, folderLinkID, err := f.dirCache.FindPath(ctx, f.sanitizePath(remote), false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			// parent folder of the file not found, we for sure can't find the file
			return nil, fs.ErrorObjectNotFound
		}
		// other error has occurred
		return nil, err
	}

	var link *proton.Link
	if err = f.pacer.Call(func() (bool, error) {
		link, err = f.protonDrive.SearchByNameInActiveFolderByID(ctx, folderLinkID, leaf, true, false, proton.LinkStateActive)
		return shouldRetry(ctx, err)
	}); err != nil {
		return nil, err
	}
	if link == nil { // both link and err are nil, file not found
		return nil, fs.ErrorObjectNotFound
	}

	return link, nil
}

// readMetaDataForLink reads the metadata from the remote
func (f *Fs) readMetaDataForLink(ctx context.Context, link *proton.Link) (*protonDriveAPI.FileSystemAttrs, error) {
	var fileSystemAttrs *protonDriveAPI.FileSystemAttrs
	var err error
	if err = f.pacer.Call(func() (bool, error) {
		fileSystemAttrs, err = f.protonDrive.GetActiveRevisionAttrs(ctx, link)
		return shouldRetry(ctx, err)
	}); err != nil {
		return nil, err
	}

	return fileSystemAttrs, nil
}

// Return an Object from a path and link
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithLink(ctx context.Context, remote string, link *proton.Link) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}

	o.id = link.LinkID
	o.size = link.Size
	o.modTime = time.Unix(link.ModifyTime, 0)
	o.createdTime = time.Unix(link.CreateTime, 0)
	o.mimetype = link.MIMEType
	o.link = link

	fileSystemAttrs, err := o.fs.readMetaDataForLink(ctx, link)
	if err != nil {
		return nil, err
	}
	if fileSystemAttrs != nil {
		o.modTime = fileSystemAttrs.ModificationTime
		o.originalSize = &fileSystemAttrs.Size
		o.blockSizes = fileSystemAttrs.BlockSizes
		o.digests = &fileSystemAttrs.Digests
	}

	return o, nil
}

// Return an Object from a path only
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObject(ctx context.Context, remote string) (fs.Object, error) {
	link, err := f.getObjectLink(ctx, remote)
	if err != nil {
		return nil, err
	}
	return f.newObjectWithLink(ctx, remote, link)
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
// Notice that this function is expensive since everything on proton is encrypted
// So having a remote with 10k files, during operations like sync, might take a while and lots of bandwidth!
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	folderLinkID, err := f.dirCache.FindDir(ctx, f.sanitizePath(dir), false) // will handle ErrDirNotFound here
	if err != nil {
		return nil, err
	}

	var foldersAndFiles []*protonDriveAPI.ProtonDirectoryData
	if err = f.pacer.Call(func() (bool, error) {
		foldersAndFiles, err = f.protonDrive.ListDirectory(ctx, folderLinkID)
		return shouldRetry(ctx, err)
	}); err != nil {
		return nil, err
	}

	entries := make(fs.DirEntries, 0)
	for i := range foldersAndFiles {
		remote := path.Join(dir, f.opt.Enc.ToStandardName(foldersAndFiles[i].Name))

		if foldersAndFiles[i].IsFolder {
			f.dirCache.Put(remote, foldersAndFiles[i].Link.LinkID)
			d := fs.NewDir(remote, time.Unix(foldersAndFiles[i].Link.ModifyTime, 0)).SetID(foldersAndFiles[i].Link.LinkID)
			entries = append(entries, d)
		} else {
			obj, err := f.newObjectWithLink(ctx, remote, foldersAndFiles[i].Link)
			if err != nil {
				return nil, err
			}
			entries = append(entries, obj)
		}
	}

	return entries, nil
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
//
// This should be implemented by the backend and will be called by the
// dircache package when appropriate.
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (string, bool, error) {
	/* f.opt.Enc.FromStandardName(leaf) not required since the DirCache only process sanitized path */

	var link *proton.Link
	var err error
	if err = f.pacer.Call(func() (bool, error) {
		link, err = f.protonDrive.SearchByNameInActiveFolderByID(ctx, pathID, leaf, false, true, proton.LinkStateActive)
		return shouldRetry(ctx, err)
	}); err != nil {
		return "", false, err
	}
	if link == nil {
		return "", false, nil
	}

	return link.LinkID, true, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
//
// This should be implemented by the backend and will be called by the
// dircache package when appropriate.
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (string, error) {
	/* f.opt.Enc.FromStandardName(leaf) not required since the DirCache only process sanitized path */

	var newID string
	var err error
	if err = f.pacer.Call(func() (bool, error) {
		newID, err = f.protonDrive.CreateNewFolderByID(ctx, pathID, leaf)
		return shouldRetry(ctx, err)
	}); err != nil {
		return "", err
	}

	return newID, err
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
	size := src.Size()
	if size < 0 {
		return nil, errCanNotUploadFileWithUnknownSize
	}

	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		// object is found, we add an revision to it
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		// object not found, so we need to create it
		remote := src.Remote()
		size := src.Size()
		modTime := src.ModTime(ctx)

		obj, err := f.createObject(ctx, remote, modTime, size)
		if err != nil {
			return nil, err
		}
		return obj, obj.Update(ctx, in, src, options...)
	default:
		// real error caught
		return nil, err
	}
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Returns the object, leaf, directoryID and error.
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, modTime time.Time, size int64) (*Object, error) {
	//                 ˇ-------ˇ filename
	// e.g. /root/a/b/c/test.txt
	//      ^~~~~~~~~~~^ dirPath

	// Create the directory for the object if it doesn't exist
	_, _, err := f.dirCache.FindPath(ctx, f.sanitizePath(remote), true)
	if err != nil {
		return nil, err
	}

	// Temporary Object under construction
	obj := &Object{
		fs:           f,
		remote:       remote,
		size:         size,
		originalSize: nil,
		id:           "",
		modTime:      modTime,
		mimetype:     "",
		link:         nil,
	}
	return obj, nil
}

// Mkdir makes the directory (container, bucket)
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, f.sanitizePath(dir), true)
	return err
}

// Rmdir removes the directory (container, bucket) if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	folderLinkID, err := f.dirCache.FindDir(ctx, f.sanitizePath(dir), false)
	if err == fs.ErrorDirNotFound {
		return fmt.Errorf("[Rmdir] cannot find LinkID for dir %s (%s)", dir, f.sanitizePath(dir))
	} else if err != nil {
		return err
	}

	if err = f.pacer.Call(func() (bool, error) {
		err = f.protonDrive.MoveFolderToTrashByID(ctx, folderLinkID, true)
		return shouldRetry(ctx, err)
	}); err != nil {
		return err
	}

	f.dirCache.FlushDir(f.sanitizePath(dir))
	return nil
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// DirCacheFlush an optional interface to flush internal directory cache
// DirCacheFlush resets the directory cache - used in testing
// as an optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
	f.protonDrive.ClearCache()
}

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.SHA1)
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	var user *proton.User
	var err error
	if err = f.pacer.Call(func() (bool, error) {
		user, err = f.protonDrive.About(ctx)
		return shouldRetry(ctx, err)
	}); err != nil {
		return nil, err
	}

	total := user.MaxSpace
	used := user.UsedSpace
	free := total - used

	usage := &fs.Usage{
		Total: &total,
		Used:  &used,
		Free:  &free,
	}

	return usage, nil
}

// ------------------------------------------------------------

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

// Hash returns the hashes of an object
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.SHA1 {
		return "", hash.ErrUnsupported
	}

	if o.digests != nil {
		return *o.digests, nil
	}

	// sha1 not cached: we fetch and try to obtain the sha1 of the link
	fileSystemAttrs, err := o.fs.protonDrive.GetActiveRevisionAttrsByID(ctx, o.ID())
	if err != nil {
		return "", err
	}

	if fileSystemAttrs == nil || fileSystemAttrs.Digests == "" {
		fs.Debugf(o, "file sha1 digest missing")
		return "", nil
	}
	return fileSystemAttrs.Digests, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	if o.fs.opt.ReportOriginalSize {
		// if ReportOriginalSize is set, we will generate an error when the original size failed to be parsed
		// this is crucial as features like Open() will need to use the proper size to operate the seek/range operator
		if o.originalSize != nil {
			return *o.originalSize
		}

		fs.Debugf(o, "Original file size missing")
	}
	return o.size
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open opens the file for read.  Call Close() on the returned io.ReadCloser
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	fs.FixRangeOption(options, *o.originalSize)
	var offset, limit int64 = 0, -1
	for _, option := range options { // if the caller passes in nil for options, it will become array of nil
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

	// download and decrypt the file
	var reader io.ReadCloser
	var fileSystemAttrs *protonDriveAPI.FileSystemAttrs
	var sizeOnServer int64
	var err error
	if err = o.fs.pacer.Call(func() (bool, error) {
		reader, sizeOnServer, fileSystemAttrs, err = o.fs.protonDrive.DownloadFileByID(ctx, o.id, offset)
		return shouldRetry(ctx, err)
	}); err != nil {
		return nil, err
	}

	if fileSystemAttrs != nil {
		o.originalSize = &fileSystemAttrs.Size
		o.modTime = fileSystemAttrs.ModificationTime
		o.digests = &fileSystemAttrs.Digests
		o.blockSizes = fileSystemAttrs.BlockSizes
	} else {
		fs.Debugf(o, "fileSystemAttrs is nil: using fallback size, and now digests and blocksizes available")
		o.originalSize = &sizeOnServer
		o.size = sizeOnServer
		o.digests = nil
		o.blockSizes = nil
	}

	retReader := io.NopCloser(reader) // the NewLimitedReadCloser will deal with the limit

	// deal with limit
	return readers.NewLimitedReadCloser(retReader, limit), nil
}

// Update in to the object with the modTime given of the given size
//
// When called from outside an Fs by rclone, src.Size() will always be >= 0.
// But for unknown-sized objects (indicated by src.Size() == -1), Upload should either
// return an error or update the object properly (rather than e.g. calling panic).
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	size := src.Size()
	if size < 0 {
		return errCanNotUploadFileWithUnknownSize
	}

	remote := o.Remote()
	leaf, folderLinkID, err := o.fs.dirCache.FindPath(ctx, o.fs.sanitizePath(remote), true)
	if err != nil {
		return err
	}

	modTime := src.ModTime(ctx)
	var linkID string
	var fileSystemAttrs *proton.RevisionXAttrCommon
	if err = o.fs.pacer.Call(func() (bool, error) {
		linkID, fileSystemAttrs, err = o.fs.protonDrive.UploadFileByReader(ctx, folderLinkID, leaf, modTime, in, 0)
		return shouldRetry(ctx, err)
	}); err != nil {
		return err
	}

	var sha1Hash string
	if val, ok := fileSystemAttrs.Digests["SHA1"]; ok {
		sha1Hash = val
	} else {
		sha1Hash = ""
	}

	o.id = linkID
	o.originalSize = &fileSystemAttrs.Size
	o.modTime = modTime
	o.blockSizes = fileSystemAttrs.BlockSizes
	o.digests = &sha1Hash

	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.pacer.Call(func() (bool, error) {
		err := o.fs.protonDrive.MoveFileToTrashByID(ctx, o.id)
		return shouldRetry(ctx, err)
	})
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Purge all files in the directory specified
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context, dir string) error {
	root := path.Join(f.root, dir)
	if root == "" {
		// we can't remove the root directory, but we can list the directory and delete every folder and file in here
		return errCanNotPurgeRootDirectory
	}

	folderLinkID, err := f.dirCache.FindDir(ctx, f.sanitizePath(dir), false)
	if err != nil {
		return err
	}

	if err = f.pacer.Call(func() (bool, error) {
		err = f.protonDrive.MoveFolderToTrashByID(ctx, folderLinkID, false)
		return shouldRetry(ctx, err)
	}); err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimetype
}

// Disconnect the current user
func (f *Fs) Disconnect(ctx context.Context) error {
	return f.pacer.Call(func() (bool, error) {
		err := f.protonDrive.Logout(ctx)
		return shouldRetry(ctx, err)
	})
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

	// check if the remote (dst) exists
	_, err := f.NewObject(ctx, remote)
	if err != nil {
		if err != fs.ErrorObjectNotFound {
			return nil, err
		}
		// object is indeed not found
	} else {
		// object at the dst exists
		return nil, fs.ErrorCantMove
	}

	// attempt the move
	dstLeaf, dstDirectoryID, err := f.dirCache.FindPath(ctx, f.sanitizePath(remote), true)
	if err != nil {
		return nil, err
	}
	if err = f.pacer.Call(func() (bool, error) {
		err = f.protonDrive.MoveFileByID(ctx, srcObj.id, dstDirectoryID, dstLeaf)
		return shouldRetry(ctx, err)
	}); err != nil {
		return nil, err
	}

	f.dirCache.FlushDir(f.sanitizePath(src.Remote()))

	return f.NewObject(ctx, remote)
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	srcID, _, _, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, f.sanitizePath(srcFs.root), f.sanitizePath(srcRemote), f.sanitizePath(f.root), f.sanitizePath(dstRemote))
	if err != nil {
		return err
	}

	if err = f.pacer.Call(func() (bool, error) {
		err = f.protonDrive.MoveFolderByID(ctx, srcID, dstDirectoryID, dstLeaf)
		return shouldRetry(ctx, err)
	}); err != nil {
		return err
	}

	srcFs.dirCache.FlushDir(f.sanitizePath(srcRemote))

	return nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
