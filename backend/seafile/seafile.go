package seafile

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/seafile/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/cache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/rest"
)

const (
	librariesCacheKey   = "all"
	retryAfterHeader    = "Retry-After"
	configURL           = "url"
	configUser          = "user"
	configPassword      = "pass"
	config2FA           = "2fa"
	configLibrary       = "library"
	configLibraryKey    = "library_key"
	configCreateLibrary = "create_library"
	configAuthToken     = "auth_token"
)

// This is global to all instances of fs
// (copying from a seafile remote to another remote would create 2 fs)
var (
	rangeDownloadNotice sync.Once  // Display the notice only once
	createLibraryMutex  sync.Mutex // Mutex to protect library creation
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "seafile",
		Description: "seafile",
		NewFs:       NewFs,
		Config:      Config,
		Options: []fs.Option{{
			Name:     configURL,
			Help:     "URL of seafile host to connect to",
			Required: true,
			Examples: []fs.OptionExample{{
				Value: "https://cloud.seafile.com/",
				Help:  "Connect to cloud.seafile.com",
			}},
		}, {
			Name:     configUser,
			Help:     "User name (usually email address)",
			Required: true,
		}, {
			// Password is not required, it will be left blank for 2FA
			Name:       configPassword,
			Help:       "Password",
			IsPassword: true,
		}, {
			Name:    config2FA,
			Help:    "Two-factor authentication ('true' if the account has 2FA enabled)",
			Default: false,
		}, {
			Name: configLibrary,
			Help: "Name of the library. Leave blank to access all non-encrypted libraries.",
		}, {
			Name:       configLibraryKey,
			Help:       "Library password (for encrypted libraries only). Leave blank if you pass it through the command line.",
			IsPassword: true,
		}, {
			Name:     configCreateLibrary,
			Help:     "Should rclone create a library if it doesn't exist",
			Advanced: true,
			Default:  false,
		}, {
			// Keep the authentication token after entering the 2FA code
			Name: configAuthToken,
			Help: "Authentication token",
			Hide: fs.OptionHideBoth,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.EncodeZero |
				encoder.EncodeCtl |
				encoder.EncodeSlash |
				encoder.EncodeBackSlash |
				encoder.EncodeDoubleQuote |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	URL           string               `config:"url"`
	User          string               `config:"user"`
	Password      string               `config:"pass"`
	Is2FA         bool                 `config:"2fa"`
	AuthToken     string               `config:"auth_token"`
	LibraryName   string               `config:"library"`
	LibraryKey    string               `config:"library_key"`
	CreateLibrary bool                 `config:"create_library"`
	Enc           encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote seafile
type Fs struct {
	name                string       // name of this remote
	root                string       // the path we are working on
	libraryName         string       // current library
	encrypted           bool         // Is this an encrypted library
	rootDirectory       string       // directory part of root (if any)
	opt                 Options      // parsed options
	libraries           *cache.Cache // Keep a cache of libraries
	librariesMutex      sync.Mutex   // Mutex to protect getLibraryID
	features            *fs.Features // optional features
	endpoint            *url.URL     // URL of the host
	endpointURL         string       // endpoint as a string
	srv                 *rest.Client // the connection to the one drive server
	pacer               *fs.Pacer    // pacer for API calls
	authMu              sync.Mutex   // Mutex to protect library decryption
	createDirMutex      sync.Mutex   // Protect creation of directories
	useOldDirectoryAPI  bool         // Use the old API v2 if seafile < 7
	moveDirNotAvailable bool         // Version < 7.0 don't have an API to move a directory
}

// ------------------------------------------------------------

// NewFs constructs an Fs from the path, container:path
func NewFs(name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	root = strings.Trim(root, "/")
	isLibraryRooted := opt.LibraryName != ""
	var libraryName, rootDirectory string
	if isLibraryRooted {
		libraryName = opt.LibraryName
		rootDirectory = root
	} else {
		libraryName, rootDirectory = bucket.Split(root)
	}

	if !strings.HasSuffix(opt.URL, "/") {
		opt.URL += "/"
	}
	if opt.Password != "" {
		var err error
		opt.Password, err = obscure.Reveal(opt.Password)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't decrypt user password")
		}
	}
	if opt.LibraryKey != "" {
		var err error
		opt.LibraryKey, err = obscure.Reveal(opt.LibraryKey)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't decrypt library password")
		}
	}

	// Parse the endpoint
	u, err := url.Parse(opt.URL)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:          name,
		root:          root,
		libraryName:   libraryName,
		rootDirectory: rootDirectory,
		libraries:     cache.New(),
		opt:           *opt,
		endpoint:      u,
		endpointURL:   u.String(),
		srv:           rest.NewClient(fshttp.NewClient(fs.Config)).SetRoot(u.String()),
		pacer:         getPacer(opt.URL),
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		BucketBased:             opt.LibraryName == "",
	}).Fill(f)

	ctx := context.Background()
	serverInfo, err := f.getServerInfo(ctx)
	if err != nil {
		return nil, err
	}
	fs.Debugf(nil, "Seafile server version %s", serverInfo.Version)

	// We don't support lower than seafile v6.0 (version 6.0 is already more than 3 years old)
	serverVersion := semver.New(serverInfo.Version)
	if serverVersion.Major < 6 {
		return nil, errors.New("unsupported Seafile server (version < 6.0)")
	}
	if serverVersion.Major < 7 {
		// Seafile 6 does not support recursive listing
		f.useOldDirectoryAPI = true
		f.features.ListR = nil
		// It also does no support moving directories
		f.moveDirNotAvailable = true
	}

	// Take the authentication token from the configuration first
	token := f.opt.AuthToken
	if token == "" {
		// If not available, send the user/password instead
		token, err = f.authorizeAccount(ctx)
		if err != nil {
			return nil, err
		}
	}
	f.setAuthorizationToken(token)

	if f.libraryName != "" {
		// Check if the library exists
		exists, err := f.libraryExists(ctx, f.libraryName)
		if err != nil {
			return f, err
		}
		if !exists {
			if f.opt.CreateLibrary {
				err := f.mkLibrary(ctx, f.libraryName, "")
				if err != nil {
					return f, err
				}
			} else {
				return f, fmt.Errorf("library '%s' was not found, and the option to create it is not activated (advanced option)", f.libraryName)
			}
		}
		libraryID, err := f.getLibraryID(ctx, f.libraryName)
		if err != nil {
			return f, err
		}
		f.encrypted, err = f.isEncrypted(ctx, libraryID)
		if err != nil {
			return f, err
		}
		if f.encrypted {
			// If we're inside an encrypted library, let's decrypt it now
			err = f.authorizeLibrary(ctx, libraryID)
			if err != nil {
				return f, err
			}
			// And remove the public link feature
			f.features.PublicLink = nil
		}
	} else {
		// Deactivate the cleaner feature since there's no library selected
		f.features.CleanUp = nil
	}

	if f.rootDirectory != "" {
		// Check to see if the root is an existing file
		remote := path.Base(rootDirectory)
		f.rootDirectory = path.Dir(rootDirectory)
		if f.rootDirectory == "." {
			f.rootDirectory = ""
		}
		_, err := f.NewObject(ctx, remote)
		if err != nil {
			if errors.Cause(err) == fs.ErrorObjectNotFound || errors.Cause(err) == fs.ErrorNotAFile {
				// File doesn't exist so return the original f
				f.rootDirectory = rootDirectory
				return f, nil
			}
			return f, err
		}
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// Config callback for 2FA
func Config(name string, m configmap.Mapper) {
	serverURL, ok := m.Get(configURL)
	if !ok || serverURL == "" {
		// If there's no server URL, it means we're trying an operation at the backend level, like a "rclone authorize seafile"
		fmt.Print("\nOperation not supported on this remote.\nIf you need a 2FA code on your account, use the command:\n\nrclone config reconnect <remote name>:\n\n")
		return
	}

	// Stop if we are running non-interactive config
	if fs.Config.AutoConfirm {
		return
	}

	u, err := url.Parse(serverURL)
	if err != nil {
		fs.Errorf(nil, "Invalid server URL %s", serverURL)
		return
	}

	is2faEnabled, _ := m.Get(config2FA)
	if is2faEnabled != "true" {
		fmt.Println("Two-factor authentication is not enabled on this account.")
		return
	}

	username, _ := m.Get(configUser)
	if username == "" {
		fs.Errorf(nil, "A username is required")
		return
	}

	password, _ := m.Get(configPassword)
	if password != "" {
		password, _ = obscure.Reveal(password)
	}
	// Just make sure we do have a password
	for password == "" {
		fmt.Print("Two-factor authentication: please enter your password (it won't be saved in the configuration)\npassword> ")
		password = config.ReadPassword()
	}

	// Create rest client for getAuthorizationToken
	url := u.String()
	if !strings.HasPrefix(url, "/") {
		url += "/"
	}
	srv := rest.NewClient(fshttp.NewClient(fs.Config)).SetRoot(url)

	// We loop asking for a 2FA code
	for {
		code := ""
		for code == "" {
			fmt.Print("Two-factor authentication: please enter your 2FA code\n2fa code> ")
			code = config.ReadLine()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		fmt.Println("Authenticating...")
		token, err := getAuthorizationToken(ctx, srv, username, password, code)
		if err != nil {
			fmt.Printf("Authentication failed: %v\n", err)
			tryAgain := strings.ToLower(config.ReadNonEmptyLine("Do you want to try again (y/n)?"))
			if tryAgain != "y" && tryAgain != "yes" {
				// The user is giving up, we're done here
				break
			}
		}
		if token != "" {
			fmt.Println("Success!")
			// Let's save the token into the configuration
			m.Set(configAuthToken, token)
			// And delete any previous entry for password
			m.Set(configPassword, "")
			config.SaveConfig()
			// And we're done here
			break
		}
	}
}

// sets the AuthorizationToken up
func (f *Fs) setAuthorizationToken(token string) {
	f.srv.SetHeader("Authorization", "Token "+token)
}

// authorizeAccount gets the auth token.
func (f *Fs) authorizeAccount(ctx context.Context) (string, error) {
	f.authMu.Lock()
	defer f.authMu.Unlock()

	token, err := f.getAuthorizationToken(ctx)
	if err != nil {
		return "", err
	}
	return token, nil
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	408, // Request Timeout
	429, // Rate exceeded.
	500, // Get occasional 500 Internal Server Error
	503, // Service Unavailable
	504, // Gateway Time-out
	520, // Operation failed (We get them sometimes when running tests in parallel)
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetry(resp *http.Response, err error) (bool, error) {
	// For 429 errors look at the Retry-After: header and
	// set the retry appropriately, starting with a minimum of 1
	// second if it isn't set.
	if resp != nil && (resp.StatusCode == 429) {
		var retryAfter = 1
		retryAfterString := resp.Header.Get(retryAfterHeader)
		if retryAfterString != "" {
			var err error
			retryAfter, err = strconv.Atoi(retryAfterString)
			if err != nil {
				fs.Errorf(f, "Malformed %s header %q: %v", retryAfterHeader, retryAfterString, err)
			}
		}
		return true, pacer.RetryAfterError(err, time.Duration(retryAfter)*time.Second)
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

func (f *Fs) shouldRetryUpload(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if err != nil || (resp != nil && resp.StatusCode > 400) {
		return true, err
	}
	return false, nil
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
	if f.libraryName == "" {
		return fmt.Sprintf("seafile root")
	}
	library := "library"
	if f.encrypted {
		library = "encrypted " + library
	}
	if f.rootDirectory == "" {
		return fmt.Sprintf("seafile %s '%s'", library, f.libraryName)
	}
	return fmt.Sprintf("seafile %s '%s' path '%s'", library, f.libraryName, f.rootDirectory)
}

// Precision of the ModTimes in this Fs
func (f *Fs) Precision() time.Duration {
	// The API doesn't support setting the modified time
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

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return fs.ErrorDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	if dir == "" && f.libraryName == "" {
		return f.listLibraries(ctx)
	}
	return f.listDir(ctx, dir, false)
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	libraryName, filePath := f.splitPath(remote)
	libraryID, err := f.getLibraryID(ctx, libraryName)
	if err != nil {
		return nil, err
	}
	err = f.authorizeLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}

	fileDetails, err := f.getFileDetails(ctx, libraryID, filePath)
	if err != nil {
		return nil, err
	}

	modTime, err := time.Parse(time.RFC3339, fileDetails.Modified)
	if err != nil {
		fs.LogPrintf(fs.LogLevelWarning, fileDetails.Modified, "Cannot parse datetime")
	}

	o := &Object{
		fs:            f,
		libraryID:     libraryID,
		id:            fileDetails.ID,
		remote:        remote,
		pathInLibrary: filePath,
		modTime:       modTime,
		size:          fileDetails.Size,
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
	object := f.newObject(ctx, src.Remote(), src.Size(), src.ModTime(ctx))
	// Check if we need to create a new library at that point
	if object.libraryID == "" {
		library, _ := f.splitPath(object.remote)
		err := f.Mkdir(ctx, library)
		if err != nil {
			return object, err
		}
		libraryID, err := f.getLibraryID(ctx, library)
		if err != nil {
			return object, err
		}
		object.libraryID = libraryID
	}
	err := object.Update(ctx, in, src, options...)
	if err != nil {
		return object, err
	}
	return object, nil
}

// PutStream uploads to the remote path with the modTime given but of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir makes the directory or library
//
// Shouldn't return an error if it already exists
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	libraryName, folder := f.splitPath(dir)
	if strings.HasPrefix(dir, libraryName) {
		err := f.mkLibrary(ctx, libraryName, "")
		if err != nil {
			return err
		}
		if folder == "" {
			// No directory to create after the library
			return nil
		}
	}
	err := f.mkDir(ctx, dir)
	if err != nil {
		return err
	}
	return nil
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	libraryName, dirPath := f.splitPath(dir)
	libraryID, err := f.getLibraryID(ctx, libraryName)
	if err != nil {
		return err
	}

	if check {
		directoryEntries, err := f.getDirectoryEntries(ctx, libraryID, dirPath, false)
		if err != nil {
			return err
		}
		if len(directoryEntries) > 0 {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	if dirPath == "" || dirPath == "/" {
		return f.deleteLibrary(ctx, libraryID)
	}
	return f.deleteDir(ctx, libraryID, dirPath)
}

// Rmdir removes the directory or library if empty
//
// Return an error if it doesn't exist or isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// ==================== Optional Interface fs.ListRer ====================

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
//
// It should call callback for each tranche of entries read.
// These need not be returned in any particular order.  If
// callback returns an error then the listing will stop
// immediately.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) error {
	var err error

	if dir == "" && f.libraryName == "" {
		libraries, err := f.listLibraries(ctx)
		if err != nil {
			return err
		}
		// Send the library list as folders
		err = callback(libraries)
		if err != nil {
			return err
		}

		// Then list each library
		for _, library := range libraries {
			err = f.listDirCallback(ctx, library.Remote(), callback)
			if err != nil {
				return err
			}
		}
		return nil
	}
	err = f.listDirCallback(ctx, dir, callback)
	if err != nil {
		return err
	}
	return nil
}

// ==================== Optional Interface fs.Copier ====================

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantCopy
	}
	srcLibraryName, srcPath := srcObj.fs.splitPath(src.Remote())
	srcLibraryID, err := srcObj.fs.getLibraryID(ctx, srcLibraryName)
	if err != nil {
		return nil, err
	}
	dstLibraryName, dstPath := f.splitPath(remote)
	dstLibraryID, err := f.getLibraryID(ctx, dstLibraryName)
	if err != nil {
		return nil, err
	}

	// Seafile does not accept a file name as a destination, only a path.
	// The destination filename will be the same as the original, or with (1) added in case it was already existing
	dstDir, dstFilename := path.Split(dstPath)

	// We have to make sure the destination path exists on the server or it's going to bomb out with an obscure error message
	err = f.mkMultiDir(ctx, dstLibraryID, dstDir)
	if err != nil {
		return nil, err
	}

	op, err := f.copyFile(ctx, srcLibraryID, srcPath, dstLibraryID, dstDir)
	if err != nil {
		return nil, err
	}

	if op.Name != dstFilename {
		// Destination was existing, so we need to move the file back into place
		err = f.adjustDestination(ctx, dstLibraryID, op.Name, dstPath, dstDir, dstFilename)
		if err != nil {
			return nil, err
		}
	}
	// Create a new object from the result
	return f.NewObject(ctx, remote)
}

// ==================== Optional Interface fs.Mover ====================

// Move src to this remote using server side move operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		return nil, fs.ErrorCantMove
	}

	srcLibraryName, srcPath := srcObj.fs.splitPath(src.Remote())
	srcLibraryID, err := srcObj.fs.getLibraryID(ctx, srcLibraryName)
	if err != nil {
		return nil, err
	}
	dstLibraryName, dstPath := f.splitPath(remote)
	dstLibraryID, err := f.getLibraryID(ctx, dstLibraryName)
	if err != nil {
		return nil, err
	}

	// anchor both source and destination paths from the root so we can compare them
	srcPath = path.Join("/", srcPath)
	dstPath = path.Join("/", dstPath)

	srcDir := path.Dir(srcPath)
	dstDir, dstFilename := path.Split(dstPath)

	if srcLibraryID == dstLibraryID && srcDir == dstDir {
		// It's only a simple case of renaming the file
		_, err := f.renameFile(ctx, srcLibraryID, srcPath, dstFilename)
		if err != nil {
			return nil, err
		}
		return f.NewObject(ctx, remote)
	}

	// We have to make sure the destination path exists on the server
	err = f.mkMultiDir(ctx, dstLibraryID, dstDir)
	if err != nil {
		return nil, err
	}

	// Seafile does not accept a file name as a destination, only a path.
	// The destination filename will be the same as the original, or with (1) added in case it already exists
	op, err := f.moveFile(ctx, srcLibraryID, srcPath, dstLibraryID, dstDir)
	if err != nil {
		return nil, err
	}

	if op.Name != dstFilename {
		// Destination was existing, so we need to move the file back into place
		err = f.adjustDestination(ctx, dstLibraryID, op.Name, dstPath, dstDir, dstFilename)
		if err != nil {
			return nil, err
		}
	}

	// Create a new object from the result
	return f.NewObject(ctx, remote)
}

// adjustDestination rename the file
func (f *Fs) adjustDestination(ctx context.Context, libraryID, srcFilename, dstPath, dstDir, dstFilename string) error {
	// Seafile seems to be acting strangely if the renamed file already exists (some cache issue maybe?)
	// It's better to delete the destination if it already exists
	fileDetail, err := f.getFileDetails(ctx, libraryID, dstPath)
	if err != nil && err != fs.ErrorObjectNotFound {
		return err
	}
	if fileDetail != nil {
		err = f.deleteFile(ctx, libraryID, dstPath)
		if err != nil {
			return err
		}
	}
	_, err = f.renameFile(ctx, libraryID, path.Join(dstDir, srcFilename), dstFilename)
	if err != nil {
		return err
	}

	return nil
}

// ==================== Optional Interface fs.DirMover ====================

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {

	// Cast into a seafile Fs
	srcFs, ok := src.(*Fs)
	if !ok {
		return fs.ErrorCantDirMove
	}

	srcLibraryName, srcPath := srcFs.splitPath(srcRemote)
	srcLibraryID, err := srcFs.getLibraryID(ctx, srcLibraryName)
	if err != nil {
		return err
	}
	dstLibraryName, dstPath := f.splitPath(dstRemote)
	dstLibraryID, err := f.getLibraryID(ctx, dstLibraryName)
	if err != nil {
		return err
	}

	srcDir := path.Dir(srcPath)
	dstDir, dstName := path.Split(dstPath)

	// anchor both source and destination to the root so we can compare them
	srcDir = path.Join("/", srcDir)
	dstDir = path.Join("/", dstDir)

	// The destination should not exist
	entries, err := f.getDirectoryEntries(ctx, dstLibraryID, dstDir, false)
	if err != nil && err != fs.ErrorDirNotFound {
		return err
	}
	if err == nil {
		for _, entry := range entries {
			if entry.Name == dstName {
				// Destination exists
				return fs.ErrorDirExists
			}
		}
	}
	if srcLibraryID == dstLibraryID && srcDir == dstDir {
		// It's only renaming
		err = srcFs.renameDir(ctx, dstLibraryID, srcPath, dstName)
		if err != nil {
			return err
		}
		return nil
	}

	// Seafile < 7 does not support moving directories
	if f.moveDirNotAvailable {
		return fs.ErrorCantDirMove
	}

	// Make sure the destination path exists
	err = f.mkMultiDir(ctx, dstLibraryID, dstDir)
	if err != nil {
		return err
	}

	// If the destination already exists, seafile will add a " (n)" to the name.
	// Sadly this API call will not return the new given name like the move file version does
	// So the trick is to rename the directory to something random before moving it
	// After the move we rename the random name back to the expected one
	// Hopefully there won't be anything with the same name existing at destination ;)
	tempName := ".rclone-move-" + random.String(32)

	// 1- rename source
	err = srcFs.renameDir(ctx, srcLibraryID, srcPath, tempName)
	if err != nil {
		return errors.Wrap(err, "Cannot rename source directory to a temporary name")
	}

	// 2- move source to destination
	err = f.moveDir(ctx, srcLibraryID, srcDir, tempName, dstLibraryID, dstDir)
	if err != nil {
		// Doh! Let's rename the source back to its original name
		_ = srcFs.renameDir(ctx, srcLibraryID, path.Join(srcDir, tempName), path.Base(srcPath))
		return err
	}

	// 3- rename destination back to source name
	err = f.renameDir(ctx, dstLibraryID, path.Join(dstDir, tempName), dstName)
	if err != nil {
		return errors.Wrap(err, "Cannot rename temporary directory to destination name")
	}

	return nil
}

// ==================== Optional Interface fs.Purger ====================

// Purge all files in the directory
//
// Implement this if you have a way of deleting all the files
// quicker than just running Remove() on the result of List()
//
// Return an error if it doesn't exist
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// ==================== Optional Interface fs.CleanUpper ====================

// CleanUp the trash in the Fs
func (f *Fs) CleanUp(ctx context.Context) error {
	if f.libraryName == "" {
		return errors.New("Cannot clean up at the root of the seafile server: please select a library to clean up")
	}
	libraryID, err := f.getLibraryID(ctx, f.libraryName)
	if err != nil {
		return err
	}
	return f.emptyLibraryTrash(ctx, libraryID)
}

// ==================== Optional Interface fs.Abouter ====================

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	accountInfo, err := f.getUserAccountInfo(ctx)
	if err != nil {
		return nil, err
	}

	usage = &fs.Usage{
		Used: fs.NewUsageValue(accountInfo.Usage), // bytes in use
	}
	if accountInfo.Total > 0 {
		usage.Total = fs.NewUsageValue(accountInfo.Total)                    // quota of bytes that can be used
		usage.Free = fs.NewUsageValue(accountInfo.Total - accountInfo.Usage) // bytes which can be uploaded before reaching the quota
	}
	return usage, nil
}

// ==================== Optional Interface fs.UserInfoer ====================

// UserInfo returns info about the connected user
func (f *Fs) UserInfo(ctx context.Context) (map[string]string, error) {
	accountInfo, err := f.getUserAccountInfo(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"Name":  accountInfo.Name,
		"Email": accountInfo.Email,
	}, nil
}

// ==================== Optional Interface fs.PublicLinker ====================

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	libraryName, filePath := f.splitPath(remote)
	if libraryName == "" {
		// We cannot share the whole seafile server, we need at least a library
		return "", errors.New("Cannot share the root of the seafile server. Please select a library to share")
	}
	libraryID, err := f.getLibraryID(ctx, libraryName)
	if err != nil {
		return "", err
	}

	// List existing links first
	shareLinks, err := f.listShareLinks(ctx, libraryID, filePath)
	if err != nil {
		return "", err
	}
	if shareLinks != nil && len(shareLinks) > 0 {
		for _, shareLink := range shareLinks {
			if shareLink.IsExpired == false {
				return shareLink.Link, nil
			}
		}
	}
	// No link was found
	shareLink, err := f.createShareLink(ctx, libraryID, filePath)
	if err != nil {
		return "", err
	}
	if shareLink.IsExpired {
		return "", nil
	}
	return shareLink.Link, nil
}

func (f *Fs) listLibraries(ctx context.Context) (entries fs.DirEntries, err error) {
	libraries, err := f.getCachedLibraries(ctx)
	if err != nil {
		return nil, errors.New("cannot load libraries")
	}

	for _, library := range libraries {
		d := fs.NewDir(library.Name, time.Unix(library.Modified, 0))
		d.SetSize(int64(library.Size))
		entries = append(entries, d)
	}

	return entries, nil
}

func (f *Fs) libraryExists(ctx context.Context, libraryName string) (bool, error) {
	libraries, err := f.getCachedLibraries(ctx)
	if err != nil {
		return false, err
	}

	for _, library := range libraries {
		if library.Name == libraryName {
			return true, nil
		}
	}
	return false, nil
}

func (f *Fs) getLibraryID(ctx context.Context, name string) (string, error) {
	libraries, err := f.getCachedLibraries(ctx)
	if err != nil {
		return "", err
	}

	for _, library := range libraries {
		if library.Name == name {
			return library.ID, nil
		}
	}
	return "", fmt.Errorf("cannot find library '%s'", name)
}

func (f *Fs) isLibraryInCache(libraryName string) bool {
	f.librariesMutex.Lock()
	defer f.librariesMutex.Unlock()

	if f.libraries == nil {
		return false
	}
	value, found := f.libraries.GetMaybe(librariesCacheKey)
	if found == false {
		return false
	}
	libraries := value.([]api.Library)
	for _, library := range libraries {
		if library.Name == libraryName {
			return true
		}
	}
	return false
}

func (f *Fs) isEncrypted(ctx context.Context, libraryID string) (bool, error) {
	libraries, err := f.getCachedLibraries(ctx)
	if err != nil {
		return false, err
	}

	for _, library := range libraries {
		if library.ID == libraryID {
			return library.Encrypted, nil
		}
	}
	return false, fmt.Errorf("cannot find library ID %s", libraryID)
}

func (f *Fs) authorizeLibrary(ctx context.Context, libraryID string) error {
	if libraryID == "" {
		return errors.New("a library ID is needed")
	}
	if f.opt.LibraryKey == "" {
		// We have no password to send
		return nil
	}
	encrypted, err := f.isEncrypted(ctx, libraryID)
	if err != nil {
		return err
	}
	if encrypted {
		fs.Debugf(nil, "Decrypting library %s", libraryID)
		f.authMu.Lock()
		defer f.authMu.Unlock()
		err := f.decryptLibrary(ctx, libraryID, f.opt.LibraryKey)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Fs) mkLibrary(ctx context.Context, libraryName, password string) error {
	// lock specific to library creation
	// we cannot reuse the same lock as we will dead-lock ourself if the libraries are not in cache
	createLibraryMutex.Lock()
	defer createLibraryMutex.Unlock()

	if libraryName == "" {
		return errors.New("a library name is needed")
	}

	// It's quite likely that multiple go routines are going to try creating the same library
	// at the start of a sync/copy. After releasing the mutex the calls waiting would try to create
	// the same library again. So we'd better check the library exists first
	if f.isLibraryInCache(libraryName) {
		return nil
	}

	fs.Debugf(nil, "%s: Create library '%s'", f.Name(), libraryName)
	f.librariesMutex.Lock()
	defer f.librariesMutex.Unlock()

	library, err := f.createLibrary(ctx, libraryName, password)
	if err != nil {
		return err
	}
	// Stores the library details into the cache
	value, found := f.libraries.GetMaybe(librariesCacheKey)
	if found == false {
		// Don't update the cache at that point
		return nil
	}
	libraries := value.([]api.Library)
	libraries = append(libraries, api.Library{
		ID:   library.ID,
		Name: library.Name,
	})
	f.libraries.Put(librariesCacheKey, libraries)
	return nil
}

// splitPath returns the library name and the full path inside the library
func (f *Fs) splitPath(dir string) (library, folder string) {
	library = f.libraryName
	folder = dir
	if library == "" {
		// The first part of the path is the library
		library, folder = bucket.Split(dir)
	} else if f.rootDirectory != "" {
		// Adds the root folder to the path to get a full path
		folder = path.Join(f.rootDirectory, folder)
	}
	return
}

func (f *Fs) listDir(ctx context.Context, dir string, recursive bool) (entries fs.DirEntries, err error) {
	libraryName, dirPath := f.splitPath(dir)
	libraryID, err := f.getLibraryID(ctx, libraryName)
	if err != nil {
		return nil, err
	}

	directoryEntries, err := f.getDirectoryEntries(ctx, libraryID, dirPath, recursive)
	if err != nil {
		return nil, err
	}

	return f.buildDirEntries(dir, libraryID, dirPath, directoryEntries, recursive), nil
}

// listDirCallback is calling listDir with the recursive option and is sending the result to the callback
func (f *Fs) listDirCallback(ctx context.Context, dir string, callback fs.ListRCallback) error {
	entries, err := f.listDir(ctx, dir, true)
	if err != nil {
		return err
	}
	err = callback(entries)
	if err != nil {
		return err
	}
	return nil
}

func (f *Fs) buildDirEntries(parentPath, libraryID, parentPathInLibrary string, directoryEntries []api.DirEntry, recursive bool) (entries fs.DirEntries) {
	for _, entry := range directoryEntries {
		var filePath, filePathInLibrary string
		if recursive {
			// In recursive mode, paths are built from DirEntry (+ a starting point)
			entryPath := strings.TrimPrefix(entry.Path, "/")
			// If we're listing from some path inside the library (not the root)
			// there's already a path in parameter, which will also be included in the entry path
			entryPath = strings.TrimPrefix(entryPath, parentPathInLibrary)
			entryPath = strings.TrimPrefix(entryPath, "/")

			filePath = path.Join(parentPath, entryPath, entry.Name)
			filePathInLibrary = path.Join(parentPathInLibrary, entryPath, entry.Name)
		} else {
			// In non-recursive mode, paths are build from the parameters
			filePath = path.Join(parentPath, entry.Name)
			filePathInLibrary = path.Join(parentPathInLibrary, entry.Name)
		}
		if entry.Type == api.FileTypeDir {
			d := fs.
				NewDir(filePath, time.Unix(entry.Modified, 0)).
				SetSize(entry.Size).
				SetID(entry.ID)
			entries = append(entries, d)
		} else if entry.Type == api.FileTypeFile {
			object := &Object{
				fs:            f,
				id:            entry.ID,
				remote:        filePath,
				pathInLibrary: filePathInLibrary,
				size:          entry.Size,
				modTime:       time.Unix(entry.Modified, 0),
				libraryID:     libraryID,
			}
			entries = append(entries, object)
		}
	}
	return entries
}

func (f *Fs) mkDir(ctx context.Context, dir string) error {
	library, fullPath := f.splitPath(dir)
	libraryID, err := f.getLibraryID(ctx, library)
	if err != nil {
		return err
	}
	return f.mkMultiDir(ctx, libraryID, fullPath)
}

func (f *Fs) mkMultiDir(ctx context.Context, libraryID, dir string) error {
	// rebuild the path one by one
	currentPath := ""
	for _, singleDir := range splitPath(dir) {
		currentPath = path.Join(currentPath, singleDir)
		err := f.mkSingleDir(ctx, libraryID, currentPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Fs) mkSingleDir(ctx context.Context, libraryID, dir string) error {
	f.createDirMutex.Lock()
	defer f.createDirMutex.Unlock()

	dirDetails, err := f.getDirectoryDetails(ctx, libraryID, dir)
	if err == nil && dirDetails != nil {
		// Don't fail if the directory exists
		return nil
	}
	if err == fs.ErrorDirNotFound {
		err = f.createDir(ctx, libraryID, dir)
		if err != nil {
			return err
		}
		return nil
	}
	return err
}

func (f *Fs) getDirectoryEntries(ctx context.Context, libraryID, folder string, recursive bool) ([]api.DirEntry, error) {
	if f.useOldDirectoryAPI {
		return f.getDirectoryEntriesAPIv2(ctx, libraryID, folder)
	}
	return f.getDirectoryEntriesAPIv21(ctx, libraryID, folder, recursive)
}

// splitPath creates a slice of paths
func splitPath(tree string) (paths []string) {
	tree, leaf := path.Split(path.Clean(tree))
	for leaf != "" && leaf != "." {
		paths = append([]string{leaf}, paths...)
		tree, leaf = path.Split(path.Clean(tree))
	}
	return
}

func (f *Fs) getCachedLibraries(ctx context.Context) ([]api.Library, error) {
	f.librariesMutex.Lock()
	defer f.librariesMutex.Unlock()

	libraries, err := f.libraries.Get(librariesCacheKey, func(key string) (value interface{}, ok bool, error error) {
		// Load the libraries if not present in the cache
		libraries, err := f.getLibraries(ctx)
		if err != nil {
			return nil, false, err
		}
		return libraries, true, nil
	})
	if err != nil {
		return nil, err
	}
	// Type assertion
	return libraries.([]api.Library), nil
}

func (f *Fs) newObject(ctx context.Context, remote string, size int64, modTime time.Time) *Object {
	libraryName, remotePath := f.splitPath(remote)
	libraryID, _ := f.getLibraryID(ctx, libraryName) // If error it means the library does not exist (yet)

	object := &Object{
		fs:            f,
		remote:        remote,
		libraryID:     libraryID,
		pathInLibrary: remotePath,
		size:          size,
		modTime:       modTime,
	}
	return object
}

// Check the interfaces are satisfied
var (
	_ fs.Fs           = &Fs{}
	_ fs.Abouter      = &Fs{}
	_ fs.CleanUpper   = &Fs{}
	_ fs.Copier       = &Fs{}
	_ fs.Mover        = &Fs{}
	_ fs.DirMover     = &Fs{}
	_ fs.ListRer      = &Fs{}
	_ fs.Purger       = &Fs{}
	_ fs.PutStreamer  = &Fs{}
	_ fs.PublicLinker = &Fs{}
	_ fs.UserInfoer   = &Fs{}
	_ fs.Object       = &Object{}
	_ fs.IDer         = &Object{}
)
