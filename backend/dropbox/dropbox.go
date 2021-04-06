// Package dropbox provides an interface to Dropbox object storage
package dropbox

// FIXME dropbox for business would be quite easy to add

/*
The Case folding of PathDisplay problem

From the docs:

path_display String. The cased path to be used for display purposes
only. In rare instances the casing will not correctly match the user's
filesystem, but this behavior will match the path provided in the Core
API v1, and at least the last path component will have the correct
casing. Changes to only the casing of paths won't be returned by
list_folder/continue. This field will be null if the file or folder is
not mounted. This field is optional.

We solve this by not implementing the ListR interface.  The dropbox
remote will recurse directory by directory only using the last element
of path_display and all will be well.
*/

import (
	"context"
	"fmt"
	"io"
	"log"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/auth"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/common"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/sharing"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/team"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/users"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/dropbox/dbhash"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
	"golang.org/x/oauth2"
)

// Constants
const (
	rcloneClientID              = "5jcck7diasz0rqy"
	rcloneEncryptedClientSecret = "fRS5vVLr2v6FbyXYnIgjwBuUAt0osq_QZTXAEcmZ7g"
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	// Upload chunk size - setting too small makes uploads slow.
	// Chunks are buffered into memory for retries.
	//
	// Speed vs chunk size uploading a 1 GB file on 2017-11-22
	//
	// Chunk Size MB, Speed Mbyte/s, % of max
	// 1	1.364	11%
	// 2	2.443	19%
	// 4	4.288	33%
	// 8	6.79	52%
	// 16	8.916	69%
	// 24	10.195	79%
	// 32	10.427	81%
	// 40	10.96	85%
	// 48	11.828	91%
	// 56	11.763	91%
	// 64	12.047	93%
	// 96	12.302	95%
	// 128	12.945	100%
	//
	// Choose 48MB which is 91% of Maximum speed.  rclone by
	// default does 4 transfers so this should use 4*48MB = 192MB
	// by default.
	defaultChunkSize = 48 * fs.MebiByte
	maxChunkSize     = 150 * fs.MebiByte
	// Max length of filename parts: https://help.dropbox.com/installs-integrations/sync-uploads/files-not-syncing
	maxFileNameLength = 255
)

var (
	// Description of how to auth for this app
	dropboxConfig = &oauth2.Config{
		Scopes: []string{
			"files.metadata.write",
			"files.content.write",
			"files.content.read",
			"sharing.write",
			// "file_requests.write",
			// "members.read", // needed for impersonate - but causes app to need to be approved by Dropbox Team Admin during the flow
		},
		// Endpoint: oauth2.Endpoint{
		// 	AuthURL:  "https://www.dropbox.com/1/oauth2/authorize",
		// 	TokenURL: "https://api.dropboxapi.com/1/oauth2/token",
		// },
		Endpoint:     dropbox.OAuthEndpoint(""),
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}
	// A regexp matching path names for files Dropbox ignores
	// See https://www.dropbox.com/en/help/145 - Ignored files
	ignoredFiles = regexp.MustCompile(`(?i)(^|/)(desktop\.ini|thumbs\.db|\.ds_store|icon\r|\.dropbox|\.dropbox.attr)$`)

	// DbHashType is the hash.Type for Dropbox
	DbHashType hash.Type

	// Errors
	errNotSupportedInSharedMode = fserrors.NoRetryError(errors.New("not supported in shared files mode"))
)

// Gets an oauth config with the right scopes
func getOauthConfig(m configmap.Mapper) *oauth2.Config {
	// If not impersonating, use standard scopes
	if impersonate, _ := m.Get("impersonate"); impersonate == "" {
		return dropboxConfig
	}
	// Make a copy of the config
	config := *dropboxConfig
	// Make a copy of the scopes with "members.read" appended
	config.Scopes = append(config.Scopes, "members.read")
	return &config
}

// Register with Fs
func init() {
	DbHashType = hash.RegisterHash("DropboxHash", 64, dbhash.New)
	fs.Register(&fs.RegInfo{
		Name:        "dropbox",
		Description: "Dropbox",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper) {
			opt := oauthutil.Options{
				NoOffline: true,
				OAuth2Opts: []oauth2.AuthCodeOption{
					oauth2.SetAuthURLParam("token_access_type", "offline"),
				},
			}
			err := oauthutil.Config(ctx, "dropbox", name, m, getOauthConfig(m), &opt)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name: "chunk_size",
			Help: fmt.Sprintf(`Upload chunk size. (< %v).

Any files larger than this will be uploaded in chunks of this size.

Note that chunks are buffered in memory (one at a time) so rclone can
deal with retries.  Setting this larger will increase the speed
slightly (at most 10%% for 128MB in tests) at the cost of using more
memory.  It can be set smaller if you are tight on memory.`, maxChunkSize),
			Default:  defaultChunkSize,
			Advanced: true,
		}, {
			Name: "impersonate",
			Help: `Impersonate this user when using a business account.

Note that if you want to use impersonate, you should make sure this
flag is set when running "rclone config" as this will cause rclone to
request the "members.read" scope which it won't normally. This is
needed to lookup a members email address into the internal ID that
dropbox uses in the API.

Using the "members.read" scope will require a Dropbox Team Admin
to approve during the OAuth flow.

You will have to use your own App (setting your own client_id and
client_secret) to use this option as currently rclone's default set of
permissions doesn't include "members.read". This can be added once
v1.55 or later is in use everywhere.
`,
			Default:  "",
			Advanced: true,
		}, {
			Name: "shared_files",
			Help: `Instructs rclone to work on individual shared files.

In this mode rclone's features are extremely limited - only list (ls, lsl, etc.) 
operations and read operations (e.g. downloading) are supported in this mode.
All other operations will be disabled.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "shared_folders",
			Help: `Instructs rclone to work on shared folders.
			
When this flag is used with no path only the List operation is supported and 
all available shared folders will be listed. If you specify a path the first part 
will be interpreted as the name of shared folder. Rclone will then try to mount this 
shared to the root namespace. On success shared folder rclone proceeds normally. 
The shared folder is now pretty much a normal folder and all normal operations 
are supported. 

Note that we don't unmount the shared folder afterwards so the 
--dropbox-shared-folders can be omitted after the first use of a particular 
shared folder.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// https://www.dropbox.com/help/syncing-uploads/files-not-syncing lists / and \
			// as invalid characters.
			// Testing revealed names with trailing spaces and the DEL character don't work.
			// Also encode invalid UTF-8 bytes as json doesn't handle them properly.
			Default: encoder.Base |
				encoder.EncodeBackSlash |
				encoder.EncodeDel |
				encoder.EncodeRightSpace |
				encoder.EncodeInvalidUtf8,
		}}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	ChunkSize     fs.SizeSuffix        `config:"chunk_size"`
	Impersonate   string               `config:"impersonate"`
	SharedFiles   bool                 `config:"shared_files"`
	SharedFolders bool                 `config:"shared_folders"`
	Enc           encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote dropbox server
type Fs struct {
	name           string         // name of this remote
	root           string         // the path we are working on
	opt            Options        // parsed options
	ci             *fs.ConfigInfo // global config
	features       *fs.Features   // optional features
	srv            files.Client   // the connection to the dropbox server
	svc            files.Client   // the connection to the dropbox server (unauthorized)
	sharing        sharing.Client // as above, but for generating sharing links
	users          users.Client   // as above, but for accessing user information
	team           team.Client    // for the Teams API
	slashRoot      string         // root with "/" prefix, lowercase
	slashRootSlash string         // root with "/" prefix and postfix, lowercase
	pacer          *fs.Pacer      // To pace the API calls
	ns             string         // The namespace we are using or "" for none
}

// Object describes a dropbox object
//
// Dropbox Objects always have full metadata
type Object struct {
	fs      *Fs // what this object is part of
	id      string
	url     string
	remote  string    // The remote path
	bytes   int64     // size of the object
	modTime time.Time // time it was last modified
	hash    string    // content_hash of the object
}

// ------------------------------------------------------------

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
	return fmt.Sprintf("Dropbox root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// shouldRetry returns a boolean as to whether this err deserves to be
// retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if err == nil {
		return false, err
	}
	baseErrString := errors.Cause(err).Error()
	// First check for specific errors
	if strings.Contains(baseErrString, "insufficient_space") {
		return false, fserrors.FatalError(err)
	} else if strings.Contains(baseErrString, "malformed_path") {
		return false, fserrors.NoRetryError(err)
	}
	// Then handle any official Retry-After header from Dropbox's SDK
	switch e := err.(type) {
	case auth.RateLimitAPIError:
		if e.RateLimitError.RetryAfter > 0 {
			fs.Logf(baseErrString, "Too many requests or write operations. Trying again in %d seconds.", e.RateLimitError.RetryAfter)
			err = pacer.RetryAfterError(err, time.Duration(e.RateLimitError.RetryAfter)*time.Second)
		}
		return true, err
	}
	// Keep old behavior for backward compatibility
	if strings.Contains(baseErrString, "too_many_write_operations") || strings.Contains(baseErrString, "too_many_requests") || baseErrString == "" {
		return true, err
	}
	return fserrors.ShouldRetry(err), err
}

func checkUploadChunkSize(cs fs.SizeSuffix) error {
	const minChunkSize = fs.Byte
	if cs < minChunkSize {
		return errors.Errorf("%s is less than %s", cs, minChunkSize)
	}
	if cs > maxChunkSize {
		return errors.Errorf("%s is greater than %s", cs, maxChunkSize)
	}
	return nil
}

func (f *Fs) setUploadChunkSize(cs fs.SizeSuffix) (old fs.SizeSuffix, err error) {
	err = checkUploadChunkSize(cs)
	if err == nil {
		old, f.opt.ChunkSize = f.opt.ChunkSize, cs
	}
	return
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	err = checkUploadChunkSize(opt.ChunkSize)
	if err != nil {
		return nil, errors.Wrap(err, "dropbox: chunk size")
	}

	// Convert the old token if it exists.  The old token was just
	// just a string, the new one is a JSON blob
	oldToken, ok := m.Get(config.ConfigToken)
	oldToken = strings.TrimSpace(oldToken)
	if ok && oldToken != "" && oldToken[0] != '{' {
		fs.Infof(name, "Converting token to new format")
		newToken := fmt.Sprintf(`{"access_token":"%s","token_type":"bearer","expiry":"0001-01-01T00:00:00Z"}`, oldToken)
		err := config.SetValueAndSave(name, config.ConfigToken, newToken)
		if err != nil {
			return nil, errors.Wrap(err, "NewFS convert token")
		}
	}

	oAuthClient, _, err := oauthutil.NewClient(ctx, name, m, getOauthConfig(m))
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure dropbox")
	}

	ci := fs.GetConfig(ctx)

	f := &Fs{
		name:  name,
		opt:   *opt,
		ci:    ci,
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	cfg := dropbox.Config{
		LogLevel:        dropbox.LogOff, // logging in the SDK: LogOff, LogDebug, LogInfo
		Client:          oAuthClient,    // maybe???
		HeaderGenerator: f.headerGenerator,
	}

	// unauthorized config for endpoints that fail with auth
	ucfg := dropbox.Config{
		LogLevel: dropbox.LogOff, // logging in the SDK: LogOff, LogDebug, LogInfo
	}

	// NOTE: needs to be created pre-impersonation so we can look up the impersonated user
	f.team = team.New(cfg)

	if opt.Impersonate != "" {
		user := team.UserSelectorArg{
			Email: opt.Impersonate,
		}
		user.Tag = "email"

		members := []*team.UserSelectorArg{&user}
		args := team.NewMembersGetInfoArgs(members)

		memberIds, err := f.team.MembersGetInfo(args)

		if err != nil {
			return nil, errors.Wrapf(err, "invalid dropbox team member: %q", opt.Impersonate)
		}

		cfg.AsMemberID = memberIds[0].MemberInfo.Profile.MemberProfile.TeamMemberId
	}

	f.srv = files.New(cfg)
	f.svc = files.New(ucfg)
	f.sharing = sharing.New(cfg)
	f.users = users.New(cfg)
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		ReadMimeType:            false,
		CanHaveEmptyDirectories: true,
	})

	// do not fill features yet
	if f.opt.SharedFiles {
		f.setRoot(root)
		if f.root == "" {
			return f, nil
		}
		_, err := f.findSharedFile(ctx, f.root)
		f.root = ""
		if err == nil {
			return f, fs.ErrorIsFile
		}
		return f, nil
	}

	if f.opt.SharedFolders {
		f.setRoot(root)
		if f.root == "" {
			return f, nil // our root it empty so we probably want to list shared folders
		}

		dir := path.Dir(f.root)
		if dir == "." {
			dir = f.root
		}

		// root is not empty so we have find the right shared folder if it exists
		id, err := f.findSharedFolder(ctx, dir)
		if err != nil {
			// if we didn't find the specified shared folder we have to bail out here
			return nil, err
		}
		// we found the specified shared folder so let's mount it
		// this will add it to the users normal root namespace and allows us
		// to actually perform operations on it using the normal api endpoints.
		err = f.mountSharedFolder(ctx, id)
		if err != nil {
			switch e := err.(type) {
			case sharing.MountFolderAPIError:
				if e.EndpointError == nil || (e.EndpointError != nil && e.EndpointError.Tag != sharing.MountFolderErrorAlreadyMounted) {
					return nil, err
				}
			default:
				return nil, err
			}
			// if the moint failed we have to abort here
		}
		// if the mount succeeded it's now a normal folder in the users root namespace
		// we disable shared folder mode and proceed normally
		f.opt.SharedFolders = false
	}

	f.features.Fill(ctx, f)

	// If root starts with / then use the actual root
	if strings.HasPrefix(root, "/") {
		var acc *users.FullAccount
		err = f.pacer.Call(func() (bool, error) {
			acc, err = f.users.GetCurrentAccount()
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return nil, errors.Wrap(err, "get current account failed")
		}
		switch x := acc.RootInfo.(type) {
		case *common.TeamRootInfo:
			f.ns = x.RootNamespaceId
		case *common.UserRootInfo:
			f.ns = x.RootNamespaceId
		default:
			return nil, errors.Errorf("unknown RootInfo type %v %T", acc.RootInfo, acc.RootInfo)
		}
		fs.Debugf(f, "Using root namespace %q", f.ns)
	}
	f.setRoot(root)

	// See if the root is actually an object
	_, err = f.getFileMetadata(ctx, f.slashRoot)
	if err == nil {
		newRoot := path.Dir(f.root)
		if newRoot == "." {
			newRoot = ""
		}
		f.setRoot(newRoot)
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// headerGenerator for dropbox sdk
func (f *Fs) headerGenerator(hostType string, style string, namespace string, route string) map[string]string {
	if f.ns == "" {
		return map[string]string{}
	}
	return map[string]string{
		"Dropbox-API-Path-Root": `{".tag": "namespace_id", "namespace_id": "` + f.ns + `"}`,
	}
}

// Sets root in f
func (f *Fs) setRoot(root string) {
	f.root = strings.Trim(root, "/")
	f.slashRoot = "/" + f.root
	f.slashRootSlash = f.slashRoot
	if f.root != "" {
		f.slashRootSlash += "/"
	}
}

// getMetadata gets the metadata for a file or directory
func (f *Fs) getMetadata(ctx context.Context, objPath string) (entry files.IsMetadata, notFound bool, err error) {
	err = f.pacer.Call(func() (bool, error) {
		entry, err = f.srv.GetMetadata(&files.GetMetadataArg{
			Path: f.opt.Enc.FromStandardPath(objPath),
		})
		return shouldRetry(ctx, err)
	})
	if err != nil {
		switch e := err.(type) {
		case files.GetMetadataAPIError:
			if e.EndpointError != nil && e.EndpointError.Path != nil && e.EndpointError.Path.Tag == files.LookupErrorNotFound {
				notFound = true
				err = nil
			}
		}
	}
	return
}

// getFileMetadata gets the metadata for a file
func (f *Fs) getFileMetadata(ctx context.Context, filePath string) (fileInfo *files.FileMetadata, err error) {
	entry, notFound, err := f.getMetadata(ctx, filePath)
	if err != nil {
		return nil, err
	}
	if notFound {
		return nil, fs.ErrorObjectNotFound
	}
	fileInfo, ok := entry.(*files.FileMetadata)
	if !ok {
		return nil, fs.ErrorNotAFile
	}
	return fileInfo, nil
}

// getDirMetadata gets the metadata for a directory
func (f *Fs) getDirMetadata(ctx context.Context, dirPath string) (dirInfo *files.FolderMetadata, err error) {
	entry, notFound, err := f.getMetadata(ctx, dirPath)
	if err != nil {
		return nil, err
	}
	if notFound {
		return nil, fs.ErrorDirNotFound
	}
	dirInfo, ok := entry.(*files.FolderMetadata)
	if !ok {
		return nil, fs.ErrorIsFile
	}
	return dirInfo, nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *files.FileMetadata) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		err = o.setMetadataFromEntry(info)
	} else {
		err = o.readEntryAndSetMetadata(ctx)
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	if f.opt.SharedFiles {
		return f.findSharedFile(ctx, remote)
	}
	return f.newObjectWithInfo(ctx, remote, nil)
}

// listSharedFoldersApi lists all available shared folders mounted and not mounted
// we'll need the id later so we have to return them in original format
func (f *Fs) listSharedFolders(ctx context.Context) (entries fs.DirEntries, err error) {
	started := false
	var res *sharing.ListFoldersResult
	for {
		if !started {
			arg := sharing.ListFoldersArgs{
				Limit: 100,
			}
			err := f.pacer.Call(func() (bool, error) {
				res, err = f.sharing.ListFolders(&arg)
				return shouldRetry(ctx, err)
			})
			if err != nil {
				return nil, err
			}
			started = true
		} else {
			arg := sharing.ListFoldersContinueArg{
				Cursor: res.Cursor,
			}
			err := f.pacer.Call(func() (bool, error) {
				res, err = f.sharing.ListFoldersContinue(&arg)
				return shouldRetry(ctx, err)
			})
			if err != nil {
				return nil, errors.Wrap(err, "list continue")
			}
		}
		for _, entry := range res.Entries {
			leaf := f.opt.Enc.ToStandardName(entry.Name)
			d := fs.NewDir(leaf, time.Now()).SetID(entry.SharedFolderId)
			entries = append(entries, d)
			if err != nil {
				return nil, err
			}
		}
		if res.Cursor == "" {
			break
		}
	}

	return entries, nil
}

// findSharedFolder find the id for a given shared folder name
// somewhat annoyingly there is no endpoint to query a shared folder by it's name
// so our only option is to iterate over all shared folders
func (f *Fs) findSharedFolder(ctx context.Context, name string) (id string, err error) {
	entries, err := f.listSharedFolders(ctx)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.(*fs.Dir).Remote() == name {
			return entry.(*fs.Dir).ID(), nil
		}
	}
	return "", fs.ErrorDirNotFound
}

// mountSharedFolder mount a shared folder to the root namespace
func (f *Fs) mountSharedFolder(ctx context.Context, id string) error {
	arg := sharing.MountFolderArg{
		SharedFolderId: id,
	}
	err := f.pacer.Call(func() (bool, error) {
		_, err := f.sharing.MountFolder(&arg)
		return shouldRetry(ctx, err)
	})
	return err
}

// listReceivedFiles lists shared the user as access to (note this means individual
// files not files contained in shared folders)
func (f *Fs) listReceivedFiles(ctx context.Context) (entries fs.DirEntries, err error) {
	started := false
	var res *sharing.ListFilesResult
	for {
		if !started {
			arg := sharing.ListFilesArg{
				Limit: 100,
			}
			err := f.pacer.Call(func() (bool, error) {
				res, err = f.sharing.ListReceivedFiles(&arg)
				return shouldRetry(ctx, err)
			})
			if err != nil {
				return nil, err
			}
			started = true
		} else {
			arg := sharing.ListFilesContinueArg{
				Cursor: res.Cursor,
			}
			err := f.pacer.Call(func() (bool, error) {
				res, err = f.sharing.ListReceivedFilesContinue(&arg)
				return shouldRetry(ctx, err)
			})
			if err != nil {
				return nil, errors.Wrap(err, "list continue")
			}
		}
		for _, entry := range res.Entries {
			fmt.Printf("%+v\n", entry)
			entryPath := entry.Name
			o := &Object{
				fs:      f,
				url:     entry.PreviewUrl,
				remote:  entryPath,
				modTime: entry.TimeInvited,
			}
			if err != nil {
				return nil, err
			}
			entries = append(entries, o)
		}
		if res.Cursor == "" {
			break
		}
	}
	return entries, nil
}

func (f *Fs) findSharedFile(ctx context.Context, name string) (o *Object, err error) {
	files, err := f.listReceivedFiles(ctx)
	if err != nil {
		return nil, err
	}
	for _, entry := range files {
		if entry.(*Object).remote == name {
			return entry.(*Object), nil
		}
	}
	return nil, fs.ErrorObjectNotFound
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
	if f.opt.SharedFiles {
		return f.listReceivedFiles(ctx)
	}
	if f.opt.SharedFolders {
		return f.listSharedFolders(ctx)
	}

	root := f.slashRoot
	if dir != "" {
		root += "/" + dir
	}

	started := false
	var res *files.ListFolderResult
	for {
		if !started {
			arg := files.ListFolderArg{
				Path:      f.opt.Enc.FromStandardPath(root),
				Recursive: false,
			}
			if root == "/" {
				arg.Path = "" // Specify root folder as empty string
			}
			err = f.pacer.Call(func() (bool, error) {
				res, err = f.srv.ListFolder(&arg)
				return shouldRetry(ctx, err)
			})
			if err != nil {
				switch e := err.(type) {
				case files.ListFolderAPIError:
					if e.EndpointError != nil && e.EndpointError.Path != nil && e.EndpointError.Path.Tag == files.LookupErrorNotFound {
						err = fs.ErrorDirNotFound
					}
				}
				return nil, err
			}
			started = true
		} else {
			arg := files.ListFolderContinueArg{
				Cursor: res.Cursor,
			}
			err = f.pacer.Call(func() (bool, error) {
				res, err = f.srv.ListFolderContinue(&arg)
				return shouldRetry(ctx, err)
			})
			if err != nil {
				return nil, errors.Wrap(err, "list continue")
			}
		}
		for _, entry := range res.Entries {
			var fileInfo *files.FileMetadata
			var folderInfo *files.FolderMetadata
			var metadata *files.Metadata
			switch info := entry.(type) {
			case *files.FolderMetadata:
				folderInfo = info
				metadata = &info.Metadata
			case *files.FileMetadata:
				fileInfo = info
				metadata = &info.Metadata
			default:
				fs.Errorf(f, "Unknown type %T", entry)
				continue
			}

			// Only the last element is reliably cased in PathDisplay
			entryPath := metadata.PathDisplay
			leaf := f.opt.Enc.ToStandardName(path.Base(entryPath))
			remote := path.Join(dir, leaf)
			if folderInfo != nil {
				d := fs.NewDir(remote, time.Now()).SetID(folderInfo.Id)
				entries = append(entries, d)
			} else if fileInfo != nil {
				o, err := f.newObjectWithInfo(ctx, remote, fileInfo)
				if err != nil {
					return nil, err
				}
				entries = append(entries, o)
			}
		}
		if !res.HasMore {
			break
		}
	}
	return entries, nil
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if f.opt.SharedFiles || f.opt.SharedFolders {
		return nil, errNotSupportedInSharedMode
	}
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: src.Remote(),
	}
	return o, o.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	if f.opt.SharedFiles || f.opt.SharedFolders {
		return errNotSupportedInSharedMode
	}
	root := path.Join(f.slashRoot, dir)

	// can't create or run metadata on root
	if root == "/" {
		return nil
	}

	// check directory doesn't exist
	_, err := f.getDirMetadata(ctx, root)
	if err == nil {
		return nil // directory exists already
	} else if err != fs.ErrorDirNotFound {
		return err // some other error
	}

	// create it
	arg2 := files.CreateFolderArg{
		Path: f.opt.Enc.FromStandardPath(root),
	}
	// Don't attempt to create filenames that are too long
	if cErr := checkPathLength(arg2.Path); cErr != nil {
		return cErr
	}
	err = f.pacer.Call(func() (bool, error) {
		_, err = f.srv.CreateFolderV2(&arg2)
		return shouldRetry(ctx, err)
	})
	return err
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) (err error) {
	root := path.Join(f.slashRoot, dir)

	// can't remove root
	if root == "/" {
		return errors.New("can't remove root directory")
	}

	if check {
		// check directory exists
		_, err = f.getDirMetadata(ctx, root)
		if err != nil {
			return errors.Wrap(err, "Rmdir")
		}

		root = f.opt.Enc.FromStandardPath(root)
		// check directory empty
		arg := files.ListFolderArg{
			Path:      root,
			Recursive: false,
		}
		if root == "/" {
			arg.Path = "" // Specify root folder as empty string
		}
		var res *files.ListFolderResult
		err = f.pacer.Call(func() (bool, error) {
			res, err = f.srv.ListFolder(&arg)
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return errors.Wrap(err, "Rmdir")
		}
		if len(res.Entries) != 0 {
			return errors.New("directory not empty")
		}
	}

	// remove it
	err = f.pacer.Call(func() (bool, error) {
		_, err = f.srv.DeleteV2(&files.DeleteArg{Path: root})
		return shouldRetry(ctx, err)
	})
	return err
}

// Rmdir deletes the container
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	if f.opt.SharedFiles || f.opt.SharedFolders {
		return errNotSupportedInSharedMode
	}
	return f.purgeCheck(ctx, dir, true)
}

// Precision returns the precision
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}

	// Temporary Object under construction
	dstObj := &Object{
		fs:     f,
		remote: remote,
	}

	// Copy
	arg := files.RelocationArg{
		RelocationPath: files.RelocationPath{
			FromPath: f.opt.Enc.FromStandardPath(srcObj.remotePath()),
			ToPath:   f.opt.Enc.FromStandardPath(dstObj.remotePath()),
		},
	}
	var err error
	var result *files.RelocationResult
	err = f.pacer.Call(func() (bool, error) {
		result, err = f.srv.CopyV2(&arg)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "copy failed")
	}

	// Set the metadata
	fileInfo, ok := result.Metadata.(*files.FileMetadata)
	if !ok {
		return nil, fs.ErrorNotAFile
	}
	err = dstObj.setMetadataFromEntry(fileInfo)
	if err != nil {
		return nil, errors.Wrap(err, "copy failed")
	}

	return dstObj, nil
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) (err error) {
	return f.purgeCheck(ctx, dir, false)
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
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

	// Temporary Object under construction
	dstObj := &Object{
		fs:     f,
		remote: remote,
	}

	// Do the move
	arg := files.RelocationArg{
		RelocationPath: files.RelocationPath{
			FromPath: f.opt.Enc.FromStandardPath(srcObj.remotePath()),
			ToPath:   f.opt.Enc.FromStandardPath(dstObj.remotePath()),
		},
	}
	var err error
	var result *files.RelocationResult
	err = f.pacer.Call(func() (bool, error) {
		result, err = f.srv.MoveV2(&arg)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "move failed")
	}

	// Set the metadata
	fileInfo, ok := result.Metadata.(*files.FileMetadata)
	if !ok {
		return nil, fs.ErrorNotAFile
	}
	err = dstObj.setMetadataFromEntry(fileInfo)
	if err != nil {
		return nil, errors.Wrap(err, "move failed")
	}
	return dstObj, nil
}

// PublicLink adds a "readable by anyone with link" permission on the given file or folder.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (link string, err error) {
	absPath := f.opt.Enc.FromStandardPath(path.Join(f.slashRoot, remote))
	fs.Debugf(f, "attempting to share '%s' (absolute path: %s)", remote, absPath)
	createArg := sharing.CreateSharedLinkWithSettingsArg{
		Path: absPath,
		Settings: &sharing.SharedLinkSettings{
			RequestedVisibility: &sharing.RequestedVisibility{
				Tagged: dropbox.Tagged{Tag: sharing.RequestedVisibilityPublic},
			},
			Audience: &sharing.LinkAudience{
				Tagged: dropbox.Tagged{Tag: sharing.LinkAudiencePublic},
			},
			Access: &sharing.RequestedLinkAccessLevel{
				Tagged: dropbox.Tagged{Tag: sharing.RequestedLinkAccessLevelViewer},
			},
		},
	}
	if expire < fs.DurationOff {
		expiryTime := time.Now().Add(time.Duration(expire)).UTC().Round(time.Second)
		createArg.Settings.Expires = expiryTime
	}
	// FIXME note we can't set Settings for non enterprise dropbox
	// because of https://github.com/dropbox/dropbox-sdk-go-unofficial/issues/75
	// however this only goes wrong when we set Expires, so as a
	// work-around remove Settings unless expire is set.
	if expire == fs.DurationOff {
		createArg.Settings = nil
	}

	var linkRes sharing.IsSharedLinkMetadata
	err = f.pacer.Call(func() (bool, error) {
		linkRes, err = f.sharing.CreateSharedLinkWithSettings(&createArg)
		return shouldRetry(ctx, err)
	})

	if err != nil && strings.Contains(err.Error(),
		sharing.CreateSharedLinkWithSettingsErrorSharedLinkAlreadyExists) {
		fs.Debugf(absPath, "has a public link already, attempting to retrieve it")
		listArg := sharing.ListSharedLinksArg{
			Path:       absPath,
			DirectOnly: true,
		}
		var listRes *sharing.ListSharedLinksResult
		err = f.pacer.Call(func() (bool, error) {
			listRes, err = f.sharing.ListSharedLinks(&listArg)
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return
		}
		if len(listRes.Links) == 0 {
			err = errors.New("Dropbox says the sharing link already exists, but list came back empty")
			return
		}
		linkRes = listRes.Links[0]
	}
	if err == nil {
		switch res := linkRes.(type) {
		case *sharing.FileLinkMetadata:
			link = res.Url
		case *sharing.FolderLinkMetadata:
			link = res.Url
		default:
			err = fmt.Errorf("Don't know how to extract link, response has unknown format: %T", res)
		}
	}
	return
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
	srcPath := path.Join(srcFs.slashRoot, srcRemote)
	dstPath := path.Join(f.slashRoot, dstRemote)

	// Check if destination exists
	_, err := f.getDirMetadata(ctx, dstPath)
	if err == nil {
		return fs.ErrorDirExists
	} else if err != fs.ErrorDirNotFound {
		return err
	}

	// Make sure the parent directory exists
	// ...apparently not necessary

	// Do the move
	arg := files.RelocationArg{
		RelocationPath: files.RelocationPath{
			FromPath: f.opt.Enc.FromStandardPath(srcPath),
			ToPath:   f.opt.Enc.FromStandardPath(dstPath),
		},
	}
	err = f.pacer.Call(func() (bool, error) {
		_, err = f.srv.MoveV2(&arg)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return errors.Wrap(err, "MoveDir failed")
	}

	return nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	var q *users.SpaceUsage
	err = f.pacer.Call(func() (bool, error) {
		q, err = f.users.GetSpaceUsage()
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "about failed")
	}
	var total uint64
	if q.Allocation != nil {
		if q.Allocation.Individual != nil {
			total += q.Allocation.Individual.Allocated
		}
		if q.Allocation.Team != nil {
			total += q.Allocation.Team.Allocated
		}
	}
	usage = &fs.Usage{
		Total: fs.NewUsageValue(int64(total)),          // quota of bytes that can be used
		Used:  fs.NewUsageValue(int64(q.Used)),         // bytes in use
		Free:  fs.NewUsageValue(int64(total - q.Used)), // bytes which can be uploaded before reaching the quota
	}
	return usage, nil
}

// ChangeNotify calls the passed function with a path that has had changes.
// If the implementation uses polling, it should adhere to the given interval.
//
// Automatically restarts itself in case of unexpected behavior of the remote.
//
// Close the returned channel to stop being notified.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	go func() {
		// get the StartCursor early so all changes from now on get processed
		startCursor, err := f.changeNotifyCursor(ctx)
		if err != nil {
			fs.Infof(f, "Failed to get StartCursor: %s", err)
		}
		var ticker *time.Ticker
		var tickerC <-chan time.Time
		for {
			select {
			case pollInterval, ok := <-pollIntervalChan:
				if !ok {
					if ticker != nil {
						ticker.Stop()
					}
					return
				}
				if ticker != nil {
					ticker.Stop()
					ticker, tickerC = nil, nil
				}
				if pollInterval != 0 {
					ticker = time.NewTicker(pollInterval)
					tickerC = ticker.C
				}
			case <-tickerC:
				if startCursor == "" {
					startCursor, err = f.changeNotifyCursor(ctx)
					if err != nil {
						fs.Infof(f, "Failed to get StartCursor: %s", err)
						continue
					}
				}
				fs.Debugf(f, "Checking for changes on remote")
				startCursor, err = f.changeNotifyRunner(ctx, notifyFunc, startCursor)
				if err != nil {
					fs.Infof(f, "Change notify listener failure: %s", err)
				}
			}
		}
	}()
}

func (f *Fs) changeNotifyCursor(ctx context.Context) (cursor string, err error) {
	var startCursor *files.ListFolderGetLatestCursorResult

	err = f.pacer.Call(func() (bool, error) {
		arg := files.ListFolderArg{
			Path:      f.opt.Enc.FromStandardPath(f.slashRoot),
			Recursive: true,
		}

		if arg.Path == "/" {
			arg.Path = ""
		}

		startCursor, err = f.srv.ListFolderGetLatestCursor(&arg)

		return shouldRetry(ctx, err)
	})
	if err != nil {
		return
	}
	return startCursor.Cursor, nil
}

func (f *Fs) changeNotifyRunner(ctx context.Context, notifyFunc func(string, fs.EntryType), startCursor string) (newCursor string, err error) {
	cursor := startCursor
	var res *files.ListFolderLongpollResult

	// Dropbox sets a timeout range of 30 - 480
	timeout := uint64(f.ci.TimeoutOrInfinite() / time.Second)

	if timeout < 30 {
		timeout = 30
	}

	if timeout > 480 {
		timeout = 480
	}

	err = f.pacer.Call(func() (bool, error) {
		args := files.ListFolderLongpollArg{
			Cursor:  cursor,
			Timeout: timeout,
		}

		res, err = f.svc.ListFolderLongpoll(&args)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return
	}

	if !res.Changes {
		return cursor, nil
	}

	if res.Backoff != 0 {
		fs.Debugf(f, "Waiting to poll for %d seconds", res.Backoff)
		time.Sleep(time.Duration(res.Backoff) * time.Second)
	}

	for {
		var changeList *files.ListFolderResult

		arg := files.ListFolderContinueArg{
			Cursor: cursor,
		}
		err = f.pacer.Call(func() (bool, error) {
			changeList, err = f.srv.ListFolderContinue(&arg)
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return "", errors.Wrap(err, "list continue")
		}
		cursor = changeList.Cursor
		var entryType fs.EntryType
		for _, entry := range changeList.Entries {
			entryPath := ""
			switch info := entry.(type) {
			case *files.FolderMetadata:
				entryType = fs.EntryDirectory
				entryPath = strings.TrimLeft(info.PathDisplay, f.slashRootSlash)
			case *files.FileMetadata:
				entryType = fs.EntryObject
				entryPath = strings.TrimLeft(info.PathDisplay, f.slashRootSlash)
			case *files.DeletedMetadata:
				entryType = fs.EntryObject
				entryPath = strings.TrimLeft(info.PathDisplay, f.slashRootSlash)
			default:
				fs.Errorf(entry, "dropbox ChangeNotify: ignoring unknown EntryType %T", entry)
				continue
			}

			if entryPath != "" {
				notifyFunc(entryPath, entryType)
			}
		}
		if !changeList.HasMore {
			break
		}
	}
	return cursor, nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(DbHashType)
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

// ID returns the object id
func (o *Object) ID() string {
	return o.id
}

// Hash returns the dropbox special hash
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if o.fs.opt.SharedFiles || o.fs.opt.SharedFolders {
		return "", errNotSupportedInSharedMode
	}
	if t != DbHashType {
		return "", hash.ErrUnsupported
	}
	err := o.readMetaData(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to read hash from metadata")
	}
	return o.hash, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.bytes
}

// setMetadataFromEntry sets the fs data from a files.FileMetadata
//
// This isn't a complete set of metadata and has an inaccurate date
func (o *Object) setMetadataFromEntry(info *files.FileMetadata) error {
	o.id = info.Id
	o.bytes = int64(info.Size)
	o.modTime = info.ClientModified
	o.hash = info.ContentHash
	return nil
}

// Reads the entry for a file from dropbox
func (o *Object) readEntry(ctx context.Context) (*files.FileMetadata, error) {
	return o.fs.getFileMetadata(ctx, o.remotePath())
}

// Read entry if not set and set metadata from it
func (o *Object) readEntryAndSetMetadata(ctx context.Context) error {
	// Last resort set time from client
	if !o.modTime.IsZero() {
		return nil
	}
	entry, err := o.readEntry(ctx)
	if err != nil {
		return err
	}
	return o.setMetadataFromEntry(entry)
}

// Returns the remote path for the object
func (o *Object) remotePath() string {
	return o.fs.slashRootSlash + o.remote
}

// readMetaData gets the info if it hasn't already been fetched
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if !o.modTime.IsZero() {
		return nil
	}
	// Last resort
	return o.readEntryAndSetMetadata(ctx)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Debugf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
//
// Commits the datastore
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	// Dropbox doesn't have a way of doing this so returning this
	// error will cause the file to be deleted first then
	// re-uploaded to set the time.
	return fs.ErrorCantSetModTimeWithoutDelete
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.fs.opt.SharedFiles {
		if len(options) != 0 {
			return nil, errors.New("OpenOptions not supported for shared files")
		}
		arg := sharing.GetSharedLinkMetadataArg{
			Url: o.url,
		}
		err = o.fs.pacer.Call(func() (bool, error) {
			_, in, err = o.fs.sharing.GetSharedLinkFile(&arg)
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return nil, err
		}
		return
	}

	fs.FixRangeOption(options, o.bytes)
	headers := fs.OpenOptionHeaders(options)
	arg := files.DownloadArg{
		Path:         o.id,
		ExtraHeaders: headers,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		_, in, err = o.fs.srv.Download(&arg)
		return shouldRetry(ctx, err)
	})

	switch e := err.(type) {
	case files.DownloadAPIError:
		// Don't attempt to retry copyright violation errors
		if e.EndpointError != nil && e.EndpointError.Path != nil && e.EndpointError.Path.Tag == files.LookupErrorRestrictedContent {
			return nil, fserrors.NoRetryError(err)
		}
	}

	return
}

// uploadChunked uploads the object in parts
//
// Will work optimally if size is >= uploadChunkSize. If the size is either
// unknown (i.e. -1) or smaller than uploadChunkSize, the method incurs an
// avoidable request to the Dropbox API that does not carry payload.
func (o *Object) uploadChunked(ctx context.Context, in0 io.Reader, commitInfo *files.CommitInfo, size int64) (entry *files.FileMetadata, err error) {
	chunkSize := int64(o.fs.opt.ChunkSize)
	chunks := 0
	if size != -1 {
		chunks = int(size/chunkSize) + 1
	}
	in := readers.NewCountingReader(in0)
	buf := make([]byte, int(chunkSize))

	fmtChunk := func(cur int, last bool) {
		if chunks == 0 && last {
			fs.Debugf(o, "Streaming chunk %d/%d", cur, cur)
		} else if chunks == 0 {
			fs.Debugf(o, "Streaming chunk %d/unknown", cur)
		} else {
			fs.Debugf(o, "Uploading chunk %d/%d", cur, chunks)
		}
	}

	// write the first chunk
	fmtChunk(1, false)
	var res *files.UploadSessionStartResult
	chunk := readers.NewRepeatableLimitReaderBuffer(in, buf, chunkSize)
	err = o.fs.pacer.Call(func() (bool, error) {
		// seek to the start in case this is a retry
		if _, err = chunk.Seek(0, io.SeekStart); err != nil {
			return false, nil
		}
		res, err = o.fs.srv.UploadSessionStart(&files.UploadSessionStartArg{}, chunk)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}

	cursor := files.UploadSessionCursor{
		SessionId: res.SessionId,
		Offset:    0,
	}
	appendArg := files.UploadSessionAppendArg{
		Cursor: &cursor,
		Close:  false,
	}

	// write more whole chunks (if any)
	currentChunk := 2
	for {
		if chunks > 0 && currentChunk >= chunks {
			// if the size is known, only upload full chunks. Remaining bytes are uploaded with
			// the UploadSessionFinish request.
			break
		} else if chunks == 0 && in.BytesRead()-cursor.Offset < uint64(chunkSize) {
			// if the size is unknown, upload as long as we can read full chunks from the reader.
			// The UploadSessionFinish request will not contain any payload.
			break
		}
		cursor.Offset = in.BytesRead()
		fmtChunk(currentChunk, false)
		chunk = readers.NewRepeatableLimitReaderBuffer(in, buf, chunkSize)
		err = o.fs.pacer.Call(func() (bool, error) {
			// seek to the start in case this is a retry
			if _, err = chunk.Seek(0, io.SeekStart); err != nil {
				return false, nil
			}
			err = o.fs.srv.UploadSessionAppendV2(&appendArg, chunk)
			// after the first chunk is uploaded, we retry everything
			return err != nil, err
		})
		if err != nil {
			return nil, err
		}
		currentChunk++
	}

	// write the remains
	cursor.Offset = in.BytesRead()
	args := &files.UploadSessionFinishArg{
		Cursor: &cursor,
		Commit: commitInfo,
	}
	fmtChunk(currentChunk, true)
	chunk = readers.NewRepeatableReaderBuffer(in, buf)
	err = o.fs.pacer.Call(func() (bool, error) {
		// seek to the start in case this is a retry
		if _, err = chunk.Seek(0, io.SeekStart); err != nil {
			return false, nil
		}
		entry, err = o.fs.srv.UploadSessionFinish(args, chunk)
		// If error is insufficient space then don't retry
		if e, ok := err.(files.UploadSessionFinishAPIError); ok {
			if e.EndpointError != nil && e.EndpointError.Path != nil && e.EndpointError.Path.Tag == files.WriteErrorInsufficientSpace {
				err = fserrors.NoRetryError(err)
				return false, err
			}
		}
		// after the first chunk is uploaded, we retry everything
		return err != nil, err
	})
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// checks all the parts of name to see they are below
// maxFileNameLength runes.
//
// This checks the length as runes which isn't quite right as dropbox
// seems to encode some symbols (eg ☺) as two "characters". This seems
// like utf-16 except that ☺ doesn't need two characters in utf-16.
//
// Using runes instead of what dropbox is using will work for most
// cases, and when it goes wrong we will upload something we should
// have detected as too long which is the least damaging way to fail.
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

// Update the already existing object
//
// Copy the reader into the object updating modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	if o.fs.opt.SharedFiles || o.fs.opt.SharedFolders {
		return errNotSupportedInSharedMode
	}
	remote := o.remotePath()
	if ignoredFiles.MatchString(remote) {
		return fserrors.NoRetryError(errors.Errorf("file name %q is disallowed - not uploading", path.Base(remote)))
	}
	commitInfo := files.NewCommitInfo(o.fs.opt.Enc.FromStandardPath(o.remotePath()))
	commitInfo.Mode.Tag = "overwrite"
	// The Dropbox API only accepts timestamps in UTC with second precision.
	commitInfo.ClientModified = src.ModTime(ctx).UTC().Round(time.Second)
	// Don't attempt to create filenames that are too long
	if cErr := checkPathLength(commitInfo.Path); cErr != nil {
		return cErr
	}

	size := src.Size()
	var err error
	var entry *files.FileMetadata
	if size > int64(o.fs.opt.ChunkSize) || size == -1 {
		entry, err = o.uploadChunked(ctx, in, commitInfo, size)
	} else {
		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			entry, err = o.fs.srv.Upload(commitInfo, in)
			return shouldRetry(ctx, err)
		})
	}
	if err != nil {
		return errors.Wrap(err, "upload failed")
	}
	return o.setMetadataFromEntry(entry)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) (err error) {
	if o.fs.opt.SharedFiles || o.fs.opt.SharedFolders {
		return errNotSupportedInSharedMode
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		_, err = o.fs.srv.DeleteV2(&files.DeleteArg{
			Path: o.fs.opt.Enc.FromStandardPath(o.remotePath()),
		})
		return shouldRetry(ctx, err)
	})
	return err
}

// Check the interfaces are satisfied
var (
	_ fs.Fs           = (*Fs)(nil)
	_ fs.Copier       = (*Fs)(nil)
	_ fs.Purger       = (*Fs)(nil)
	_ fs.PutStreamer  = (*Fs)(nil)
	_ fs.Mover        = (*Fs)(nil)
	_ fs.PublicLinker = (*Fs)(nil)
	_ fs.DirMover     = (*Fs)(nil)
	_ fs.Abouter      = (*Fs)(nil)
	_ fs.Object       = (*Object)(nil)
	_ fs.IDer         = (*Object)(nil)
)
