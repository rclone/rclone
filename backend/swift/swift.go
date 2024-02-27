// Package swift provides an interface to the Swift object storage system
package swift

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ncw/swift/v2"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/readers"
)

// Constants
const (
	directoryMarkerContentType = "application/directory" // content type of directory marker objects
	listChunks                 = 1000                    // chunk size to read directory listings
	defaultChunkSize           = 5 * fs.Gibi
	minSleep                   = 10 * time.Millisecond // In case of error, start at 10ms sleep.
)

// SharedOptions are shared between swift and backends which depend on swift
var SharedOptions = []fs.Option{{
	Name: "chunk_size",
	Help: `Above this size files will be chunked into a _segments container.

Above this size files will be chunked into a _segments container.  The
default for this is 5 GiB which is its maximum value.`,
	Default:  defaultChunkSize,
	Advanced: true,
}, {
	Name: "no_chunk",
	Help: `Don't chunk files during streaming upload.

When doing streaming uploads (e.g. using rcat or mount) setting this
flag will cause the swift backend to not upload chunked files.

This will limit the maximum upload size to 5 GiB. However non chunked
files are easier to deal with and have an MD5SUM.

Rclone will still chunk files bigger than chunk_size when doing normal
copy operations.`,
	Default:  false,
	Advanced: true,
}, {
	Name: "no_large_objects",
	Help: strings.ReplaceAll(`Disable support for static and dynamic large objects

Swift cannot transparently store files bigger than 5 GiB. There are
two schemes for doing that, static or dynamic large objects, and the
API does not allow rclone to determine whether a file is a static or
dynamic large object without doing a HEAD on the object. Since these
need to be treated differently, this means rclone has to issue HEAD
requests for objects for example when reading checksums.

When |no_large_objects| is set, rclone will assume that there are no
static or dynamic large objects stored. This means it can stop doing
the extra HEAD calls which in turn increases performance greatly
especially when doing a swift to swift transfer with |--checksum| set.

Setting this option implies |no_chunk| and also that no files will be
uploaded in chunks, so files bigger than 5 GiB will just fail on
upload.

If you set this option and there *are* static or dynamic large objects,
then this will give incorrect hashes for them. Downloads will succeed,
but other operations such as Remove and Copy will fail.
`, "|", "`"),
	Default:  false,
	Advanced: true,
}, {
	Name:     config.ConfigEncoding,
	Help:     config.ConfigEncodingHelp,
	Advanced: true,
	Default: (encoder.EncodeInvalidUtf8 |
		encoder.EncodeSlash),
}}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "swift",
		Description: "OpenStack Swift (Rackspace Cloud Files, Blomp Cloud Storage, Memset Memstore, OVH)",
		NewFs:       NewFs,
		Options: append([]fs.Option{{
			Name:    "env_auth",
			Help:    "Get swift credentials from environment variables in standard OpenStack form.",
			Default: false,
			Examples: []fs.OptionExample{
				{
					Value: "false",
					Help:  "Enter swift credentials in the next step.",
				}, {
					Value: "true",
					Help:  "Get swift credentials from environment vars.\nLeave other fields blank if using this.",
				},
			},
		}, {
			Name:      "user",
			Help:      "User name to log in (OS_USERNAME).",
			Sensitive: true,
		}, {
			Name:      "key",
			Help:      "API key or password (OS_PASSWORD).",
			Sensitive: true,
		}, {
			Name: "auth",
			Help: "Authentication URL for server (OS_AUTH_URL).",
			Examples: []fs.OptionExample{{
				Value: "https://auth.api.rackspacecloud.com/v1.0",
				Help:  "Rackspace US",
			}, {
				Value: "https://lon.auth.api.rackspacecloud.com/v1.0",
				Help:  "Rackspace UK",
			}, {
				Value: "https://identity.api.rackspacecloud.com/v2.0",
				Help:  "Rackspace v2",
			}, {
				Value: "https://auth.storage.memset.com/v1.0",
				Help:  "Memset Memstore UK",
			}, {
				Value: "https://auth.storage.memset.com/v2.0",
				Help:  "Memset Memstore UK v2",
			}, {
				Value: "https://auth.cloud.ovh.net/v3",
				Help:  "OVH",
			}, {
				Value: "https://authenticate.ain.net",
				Help:  "Blomp Cloud Storage",
			}},
		}, {
			Name:      "user_id",
			Help:      "User ID to log in - optional - most swift systems use user and leave this blank (v3 auth) (OS_USER_ID).",
			Sensitive: true,
		}, {
			Name:      "domain",
			Help:      "User domain - optional (v3 auth) (OS_USER_DOMAIN_NAME)",
			Sensitive: true,
		}, {
			Name:      "tenant",
			Help:      "Tenant name - optional for v1 auth, this or tenant_id required otherwise (OS_TENANT_NAME or OS_PROJECT_NAME).",
			Sensitive: true,
		}, {
			Name:      "tenant_id",
			Help:      "Tenant ID - optional for v1 auth, this or tenant required otherwise (OS_TENANT_ID).",
			Sensitive: true,
		}, {
			Name:      "tenant_domain",
			Help:      "Tenant domain - optional (v3 auth) (OS_PROJECT_DOMAIN_NAME).",
			Sensitive: true,
		}, {
			Name: "region",
			Help: "Region name - optional (OS_REGION_NAME).",
		}, {
			Name: "storage_url",
			Help: "Storage URL - optional (OS_STORAGE_URL).",
		}, {
			Name:      "auth_token",
			Help:      "Auth Token from alternate authentication - optional (OS_AUTH_TOKEN).",
			Sensitive: true,
		}, {
			Name:      "application_credential_id",
			Help:      "Application Credential ID (OS_APPLICATION_CREDENTIAL_ID).",
			Sensitive: true,
		}, {
			Name:      "application_credential_name",
			Help:      "Application Credential Name (OS_APPLICATION_CREDENTIAL_NAME).",
			Sensitive: true,
		}, {
			Name:      "application_credential_secret",
			Help:      "Application Credential Secret (OS_APPLICATION_CREDENTIAL_SECRET).",
			Sensitive: true,
		}, {
			Name:    "auth_version",
			Help:    "AuthVersion - optional - set to (1,2,3) if your auth URL has no version (ST_AUTH_VERSION).",
			Default: 0,
		}, {
			Name:    "endpoint_type",
			Help:    "Endpoint type to choose from the service catalogue (OS_ENDPOINT_TYPE).",
			Default: "public",
			Examples: []fs.OptionExample{{
				Value: "public",
				Help:  "Public (default, choose this if not sure)",
			}, {
				Value: "internal",
				Help:  "Internal (use internal service net)",
			}, {
				Value: "admin",
				Help:  "Admin",
			}},
		}, {
			Name: "leave_parts_on_error",
			Help: `If true avoid calling abort upload on a failure.

It should be set to true for resuming uploads across different sessions.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "storage_policy",
			Help: `The storage policy to use when creating a new container.

This applies the specified storage policy when creating a new
container. The policy cannot be changed afterwards. The allowed
configuration values and their meaning depend on your Swift storage
provider.`,
			Default: "",
			Examples: []fs.OptionExample{{
				Value: "",
				Help:  "Default",
			}, {
				Value: "pcs",
				Help:  "OVH Public Cloud Storage",
			}, {
				Value: "pca",
				Help:  "OVH Public Cloud Archive",
			}},
		}}, SharedOptions...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	EnvAuth                     bool                 `config:"env_auth"`
	User                        string               `config:"user"`
	Key                         string               `config:"key"`
	Auth                        string               `config:"auth"`
	UserID                      string               `config:"user_id"`
	Domain                      string               `config:"domain"`
	Tenant                      string               `config:"tenant"`
	TenantID                    string               `config:"tenant_id"`
	TenantDomain                string               `config:"tenant_domain"`
	Region                      string               `config:"region"`
	StorageURL                  string               `config:"storage_url"`
	AuthToken                   string               `config:"auth_token"`
	AuthVersion                 int                  `config:"auth_version"`
	ApplicationCredentialID     string               `config:"application_credential_id"`
	ApplicationCredentialName   string               `config:"application_credential_name"`
	ApplicationCredentialSecret string               `config:"application_credential_secret"`
	LeavePartsOnError           bool                 `config:"leave_parts_on_error"`
	StoragePolicy               string               `config:"storage_policy"`
	EndpointType                string               `config:"endpoint_type"`
	ChunkSize                   fs.SizeSuffix        `config:"chunk_size"`
	NoChunk                     bool                 `config:"no_chunk"`
	NoLargeObjects              bool                 `config:"no_large_objects"`
	Enc                         encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote swift server
type Fs struct {
	name             string            // name of this remote
	root             string            // the path we are working on if any
	features         *fs.Features      // optional features
	opt              Options           // options for this backend
	ci               *fs.ConfigInfo    // global config
	c                *swift.Connection // the connection to the swift server
	rootContainer    string            // container part of root (if any)
	rootDirectory    string            // directory part of root (if any)
	cache            *bucket.Cache     // cache of container status
	noCheckContainer bool              // don't check the container before creating it
	pacer            *fs.Pacer         // To pace the API calls
}

// Object describes a swift object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs           *Fs    // what this object is part of
	remote       string // The remote path
	size         int64
	lastModified time.Time
	contentType  string
	md5          string
	headers      swift.Headers // The object headers if known
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
	if f.rootContainer == "" {
		return "Swift root"
	}
	if f.rootDirectory == "" {
		return fmt.Sprintf("Swift container %s", f.rootContainer)
	}
	return fmt.Sprintf("Swift container %s path %s", f.rootContainer, f.rootDirectory)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	401, // Unauthorized (e.g. "Token has expired")
	408, // Request Timeout
	409, // Conflict - various states that could be resolved on a retry
	429, // Rate exceeded.
	500, // Get occasional 500 Internal Server Error
	503, // Service Unavailable/Slow Down - "Reduce your request rate"
	504, // Gateway Time-out
}

// shouldRetry returns a boolean as to whether this err deserves to be
// retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	// If this is a swift.Error object extract the HTTP error code
	if swiftError, ok := err.(*swift.Error); ok {
		for _, e := range retryErrorCodes {
			if swiftError.StatusCode == e {
				return true, err
			}
		}
	}
	// Check for generic failure conditions
	return fserrors.ShouldRetry(err), err
}

// shouldRetryHeaders returns a boolean as to whether this err
// deserves to be retried.  It reads the headers passed in looking for
// `Retry-After`. It returns the err as a convenience
func shouldRetryHeaders(ctx context.Context, headers swift.Headers, err error) (bool, error) {
	if swiftError, ok := err.(*swift.Error); ok && swiftError.StatusCode == 429 {
		if value := headers["Retry-After"]; value != "" {
			retryAfter, parseErr := strconv.Atoi(value)
			if parseErr != nil {
				fs.Errorf(nil, "Failed to parse Retry-After: %q: %v", value, parseErr)
			} else {
				duration := time.Second * time.Duration(retryAfter)
				if duration <= 60*time.Second {
					// Do a short sleep immediately
					fs.Debugf(nil, "Sleeping for %v to obey Retry-After", duration)
					time.Sleep(duration)
					return true, err
				}
				// Delay a long sleep for a retry
				return false, fserrors.NewErrorRetryAfter(duration)
			}
		}
	}
	return shouldRetry(ctx, err)
}

// parsePath parses a remote 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// split returns container and containerPath from the rootRelativePath
// relative to f.root
func (f *Fs) split(rootRelativePath string) (container, containerPath string) {
	container, containerPath = bucket.Split(path.Join(f.root, rootRelativePath))
	return f.opt.Enc.FromStandardName(container), f.opt.Enc.FromStandardPath(containerPath)
}

// split returns container and containerPath from the object
func (o *Object) split() (container, containerPath string) {
	return o.fs.split(o.remote)
}

// swiftConnection makes a connection to swift
func swiftConnection(ctx context.Context, opt *Options, name string) (*swift.Connection, error) {
	ci := fs.GetConfig(ctx)
	c := &swift.Connection{
		// Keep these in the same order as the Config for ease of checking
		UserName:                    opt.User,
		ApiKey:                      opt.Key,
		AuthUrl:                     opt.Auth,
		UserId:                      opt.UserID,
		Domain:                      opt.Domain,
		Tenant:                      opt.Tenant,
		TenantId:                    opt.TenantID,
		TenantDomain:                opt.TenantDomain,
		Region:                      opt.Region,
		StorageUrl:                  opt.StorageURL,
		AuthToken:                   opt.AuthToken,
		AuthVersion:                 opt.AuthVersion,
		ApplicationCredentialId:     opt.ApplicationCredentialID,
		ApplicationCredentialName:   opt.ApplicationCredentialName,
		ApplicationCredentialSecret: opt.ApplicationCredentialSecret,
		EndpointType:                swift.EndpointType(opt.EndpointType),
		ConnectTimeout:              10 * ci.ConnectTimeout, // Use the timeouts in the transport
		Timeout:                     10 * ci.Timeout,        // Use the timeouts in the transport
		Transport:                   fshttp.NewTransport(ctx),
	}
	if opt.EnvAuth {
		err := c.ApplyEnvironment()
		if err != nil {
			return nil, fmt.Errorf("failed to read environment variables: %w", err)
		}
	}
	StorageUrl, AuthToken := c.StorageUrl, c.AuthToken // nolint
	if !c.Authenticated() {
		if (c.ApplicationCredentialId != "" || c.ApplicationCredentialName != "") && c.ApplicationCredentialSecret == "" {
			if c.UserName == "" && c.UserId == "" {
				return nil, errors.New("user name or user id not found for authentication (and no storage_url+auth_token is provided)")
			}
			if c.ApiKey == "" {
				return nil, errors.New("key not found")
			}
		}
		if c.AuthUrl == "" {
			return nil, errors.New("auth not found")
		}
		err := c.Authenticate(ctx) // fills in c.StorageUrl and c.AuthToken
		if err != nil {
			return nil, err
		}
	}
	// Make sure we re-auth with the AuthToken and StorageUrl
	// provided by wrapping the existing auth, so we can just
	// override one or the other or both.
	if StorageUrl != "" || AuthToken != "" {
		// Re-write StorageURL and AuthToken if they are being
		// overridden as c.Authenticate above will have
		// overwritten them.
		if StorageUrl != "" {
			c.StorageUrl = StorageUrl
		}
		if AuthToken != "" {
			c.AuthToken = AuthToken
		}
		c.Auth = newAuth(c.Auth, StorageUrl, AuthToken)
	}
	return c, nil
}

func checkUploadChunkSize(cs fs.SizeSuffix) error {
	const minChunkSize = fs.SizeSuffixBase
	if cs < minChunkSize {
		return fmt.Errorf("%s is less than %s", cs, minChunkSize)
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

// setRoot changes the root of the Fs
func (f *Fs) setRoot(root string) {
	f.root = parsePath(root)
	f.rootContainer, f.rootDirectory = bucket.Split(f.root)
}

// NewFsWithConnection constructs an Fs from the path, container:path
// and authenticated connection.
//
// if noCheckContainer is set then the Fs won't check the container
// exists before creating it.
func NewFsWithConnection(ctx context.Context, opt *Options, name, root string, c *swift.Connection, noCheckContainer bool) (fs.Fs, error) {
	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:             name,
		opt:              *opt,
		ci:               ci,
		c:                c,
		noCheckContainer: noCheckContainer,
		pacer:            fs.NewPacer(ctx, pacer.NewS3(pacer.MinSleep(minSleep))),
		cache:            bucket.NewCache(),
	}
	f.setRoot(root)
	f.features = (&fs.Features{
		ReadMimeType:      true,
		WriteMimeType:     true,
		BucketBased:       true,
		BucketBasedRootOK: true,
		SlowModTime:       true,
	}).Fill(ctx, f)
	if f.rootContainer != "" && f.rootDirectory != "" {
		// Check to see if the object exists - ignoring directory markers
		var info swift.Object
		var err error
		encodedDirectory := f.opt.Enc.FromStandardPath(f.rootDirectory)
		err = f.pacer.Call(func() (bool, error) {
			var rxHeaders swift.Headers
			info, rxHeaders, err = f.c.Object(ctx, f.rootContainer, encodedDirectory)
			return shouldRetryHeaders(ctx, rxHeaders, err)
		})
		if err == nil && info.ContentType != directoryMarkerContentType {
			newRoot := path.Dir(f.root)
			if newRoot == "." {
				newRoot = ""
			}
			f.setRoot(newRoot)
			// return an error with an fs which points to the parent
			return f, fs.ErrorIsFile
		}
	}
	return f, nil
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
		return nil, fmt.Errorf("swift: chunk size: %w", err)
	}

	c, err := swiftConnection(ctx, opt, name)
	if err != nil {
		return nil, err
	}
	return NewFsWithConnection(ctx, opt, name, root, c, false)
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *swift.Object) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	// Note that due to a quirk of swift, dynamic large objects are
	// returned as 0 bytes in the listing.  Correct this here by
	// making sure we read the full metadata for all 0 byte files.
	// We don't read the metadata for directory marker objects.
	if info != nil && info.Bytes == 0 && info.ContentType != "application/directory" && !o.fs.opt.NoLargeObjects {
		err := o.readMetaData(ctx) // reads info and headers, returning an error
		if err == fs.ErrorObjectNotFound {
			// We have a dangling large object here so just return the original metadata
			fs.Errorf(o, "dangling large object with no contents")
		} else if err != nil {
			return nil, err
		} else {
			return o, nil
		}
	}
	if info != nil {
		// Set info but not headers
		err := o.decodeMetaData(info)
		if err != nil {
			return nil, err
		}
	} else {
		err := o.readMetaData(ctx) // reads info and headers, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found it
// returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// listFn is called from list and listContainerRoot to handle an object.
type listFn func(remote string, object *swift.Object, isDirectory bool) error

// listContainerRoot lists the objects into the function supplied from
// the container and directory supplied.  The remote has prefix
// removed from it and if addContainer is set then it adds the
// container to the start.
//
// Set recurse to read sub directories
func (f *Fs) listContainerRoot(ctx context.Context, container, directory, prefix string, addContainer bool, recurse bool, includeDirMarkers bool, fn listFn) error {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if directory != "" && !strings.HasSuffix(directory, "/") {
		directory += "/"
	}
	// Options for ObjectsWalk
	opts := swift.ObjectsOpts{
		Prefix: directory,
		Limit:  listChunks,
	}
	if !recurse {
		opts.Delimiter = '/'
	}
	return f.c.ObjectsWalk(ctx, container, &opts, func(ctx context.Context, opts *swift.ObjectsOpts) (interface{}, error) {
		var objects []swift.Object
		var err error
		err = f.pacer.Call(func() (bool, error) {
			objects, err = f.c.Objects(ctx, container, opts)
			return shouldRetry(ctx, err)
		})
		if err == nil {
			for i := range objects {
				object := &objects[i]
				isDirectory := false
				if !recurse {
					isDirectory = strings.HasSuffix(object.Name, "/")
				}
				remote := f.opt.Enc.ToStandardPath(object.Name)
				if !strings.HasPrefix(remote, prefix) {
					fs.Logf(f, "Odd name received %q", remote)
					continue
				}
				if !includeDirMarkers && remote == prefix {
					// If we have zero length directory markers ending in / then swift
					// will return them in the listing for the directory which causes
					// duplicate directories.  Ignore them here.
					continue
				}
				remote = remote[len(prefix):]
				if addContainer {
					remote = path.Join(container, remote)
				}
				err = fn(remote, object, isDirectory)
				if err != nil {
					break
				}
			}
		}
		return objects, err
	})
}

type addEntryFn func(fs.DirEntry) error

// list the objects into the function supplied
func (f *Fs) list(ctx context.Context, container, directory, prefix string, addContainer bool, recurse bool, includeDirMarkers bool, fn addEntryFn) error {
	err := f.listContainerRoot(ctx, container, directory, prefix, addContainer, recurse, includeDirMarkers, func(remote string, object *swift.Object, isDirectory bool) (err error) {
		if isDirectory {
			remote = strings.TrimRight(remote, "/")
			d := fs.NewDir(remote, time.Time{}).SetSize(object.Bytes)
			err = fn(d)
		} else {
			// newObjectWithInfo does a full metadata read on 0 size objects which might be dynamic large objects
			var o fs.Object
			o, err = f.newObjectWithInfo(ctx, remote, object)
			if err != nil {
				return err
			}
			if includeDirMarkers || o.Storable() {
				err = fn(o)
			}
		}
		return err
	})
	if err == swift.ContainerNotFound {
		err = fs.ErrorDirNotFound
	}
	return err
}

// listDir lists a single directory
func (f *Fs) listDir(ctx context.Context, container, directory, prefix string, addContainer bool) (entries fs.DirEntries, err error) {
	if container == "" {
		return nil, fs.ErrorListBucketRequired
	}
	// List the objects
	err = f.list(ctx, container, directory, prefix, addContainer, false, false, func(entry fs.DirEntry) error {
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// container must be present if listing succeeded
	f.cache.MarkOK(container)
	return entries, nil
}

// listContainers lists the containers
func (f *Fs) listContainers(ctx context.Context) (entries fs.DirEntries, err error) {
	var containers []swift.Container
	err = f.pacer.Call(func() (bool, error) {
		containers, err = f.c.ContainersAll(ctx, nil)
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, fmt.Errorf("container listing failed: %w", err)
	}
	for _, container := range containers {
		f.cache.MarkOK(container.Name)
		d := fs.NewDir(f.opt.Enc.ToStandardName(container.Name), time.Time{}).SetSize(container.Bytes).SetItems(container.Count)
		entries = append(entries, d)
	}
	return entries, nil
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
	container, directory := f.split(dir)
	if container == "" {
		if directory != "" {
			return nil, fs.ErrorListBucketRequired
		}
		return f.listContainers(ctx)
	}
	return f.listDir(ctx, container, directory, f.rootDirectory, f.rootContainer == "")
}

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
//
// Don't implement this unless you have a more efficient way
// of listing recursively than doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	container, directory := f.split(dir)
	list := walk.NewListRHelper(callback)
	listR := func(container, directory, prefix string, addContainer bool) error {
		return f.list(ctx, container, directory, prefix, addContainer, true, false, func(entry fs.DirEntry) error {
			return list.Add(entry)
		})
	}
	if container == "" {
		entries, err := f.listContainers(ctx)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			err = list.Add(entry)
			if err != nil {
				return err
			}
			container := entry.Remote()
			err = listR(container, "", f.rootDirectory, true)
			if err != nil {
				return err
			}
			// container must be present if listing succeeded
			f.cache.MarkOK(container)
		}
	} else {
		err = listR(container, directory, f.rootDirectory, f.rootContainer == "")
		if err != nil {
			return err
		}
		// container must be present if listing succeeded
		f.cache.MarkOK(container)
	}
	return list.Flush()
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	var total, objects int64
	if f.rootContainer != "" {
		var container swift.Container
		err = f.pacer.Call(func() (bool, error) {
			container, _, err = f.c.Container(ctx, f.rootContainer)
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return nil, fmt.Errorf("container info failed: %w", err)
		}
		total = container.Bytes
		objects = container.Count
	} else {
		var containers []swift.Container
		err = f.pacer.Call(func() (bool, error) {
			containers, err = f.c.ContainersAll(ctx, nil)
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return nil, fmt.Errorf("container listing failed: %w", err)
		}
		for _, c := range containers {
			total += c.Bytes
			objects += c.Count
		}
	}
	usage = &fs.Usage{
		Used:    fs.NewUsageValue(total),   // bytes in use
		Objects: fs.NewUsageValue(objects), // objects in use
	}
	return usage, nil
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	fs := &Object{
		fs:      f,
		remote:  src.Remote(),
		headers: swift.Headers{}, // Empty object headers to stop readMetaData being called
	}
	return fs, fs.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	container, _ := f.split(dir)
	return f.makeContainer(ctx, container)
}

// makeContainer creates the container if it doesn't exist
func (f *Fs) makeContainer(ctx context.Context, container string) error {
	return f.cache.Create(container, func() error {
		// Check to see if container exists first
		var err error = swift.ContainerNotFound
		if !f.noCheckContainer {
			err = f.pacer.Call(func() (bool, error) {
				var rxHeaders swift.Headers
				_, rxHeaders, err = f.c.Container(ctx, container)
				return shouldRetryHeaders(ctx, rxHeaders, err)
			})
		}
		if err == swift.ContainerNotFound {
			headers := swift.Headers{}
			if f.opt.StoragePolicy != "" {
				headers["X-Storage-Policy"] = f.opt.StoragePolicy
			}
			err = f.pacer.Call(func() (bool, error) {
				err = f.c.ContainerCreate(ctx, container, headers)
				return shouldRetry(ctx, err)
			})
			if err == nil {
				fs.Infof(f, "Container %q created", container)
			}
		}
		return err
	}, nil)
}

// Rmdir deletes the container if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	container, directory := f.split(dir)
	if container == "" || directory != "" {
		return nil
	}
	err := f.cache.Remove(container, func() error {
		err := f.pacer.Call(func() (bool, error) {
			err := f.c.ContainerDelete(ctx, container)
			return shouldRetry(ctx, err)
		})
		if err == nil {
			fs.Infof(f, "Container %q removed", container)
		}
		return err
	})
	return err
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// Purge deletes all the files in the directory
//
// Implemented here so we can make sure we delete directory markers
func (f *Fs) Purge(ctx context.Context, dir string) error {
	container, directory := f.split(dir)
	if container == "" {
		return fs.ErrorListBucketRequired
	}
	// Delete all the files including the directory markers
	toBeDeleted := make(chan fs.Object, f.ci.Transfers)
	delErr := make(chan error, 1)
	go func() {
		delErr <- operations.DeleteFiles(ctx, toBeDeleted)
	}()
	err := f.list(ctx, container, directory, f.rootDirectory, false, true, true, func(entry fs.DirEntry) error {
		if o, ok := entry.(*Object); ok {
			toBeDeleted <- o
		}
		return nil
	})
	close(toBeDeleted)
	delError := <-delErr
	if err == nil {
		err = delError
	}
	if err != nil {
		return err
	}
	return f.Rmdir(ctx, dir)
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	dstContainer, dstPath := f.split(remote)
	err := f.makeContainer(ctx, dstContainer)
	if err != nil {
		return nil, err
	}
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	isLargeObject, err := srcObj.isLargeObject(ctx)
	if err != nil {
		return nil, err
	}
	if isLargeObject {
		/*handle large object*/
		err = copyLargeObject(ctx, f, srcObj, dstContainer, dstPath)
	} else {
		srcContainer, srcPath := srcObj.split()
		err = f.pacer.Call(func() (bool, error) {
			var rxHeaders swift.Headers
			rxHeaders, err = f.c.ObjectCopy(ctx, srcContainer, srcPath, dstContainer, dstPath, nil)
			return shouldRetryHeaders(ctx, rxHeaders, err)
		})
	}
	if err != nil {
		return nil, err
	}
	return f.NewObject(ctx, remote)
}

func copyLargeObject(ctx context.Context, f *Fs, src *Object, dstContainer string, dstPath string) error {
	segmentsContainer := dstContainer + "_segments"
	err := f.makeContainer(ctx, segmentsContainer)
	if err != nil {
		return err
	}
	segments, err := src.getSegmentsLargeObject(ctx)
	if err != nil {
		return err
	}
	if len(segments) == 0 {
		return errors.New("could not copy object, list segments are empty")
	}
	nanoSeconds := time.Now().Nanosecond()
	prefixSegment := fmt.Sprintf("%v/%v/%s", nanoSeconds, src.size, strings.ReplaceAll(uuid.New().String(), "-", ""))
	copiedSegmentsLen := 10
	for _, value := range segments {
		if len(value) <= 0 {
			continue
		}
		fragment := value[0]
		if len(fragment) <= 0 {
			continue
		}
		copiedSegmentsLen = len(value)
		firstIndex := strings.Index(fragment, "/")
		if firstIndex < 0 {
			firstIndex = 0
		} else {
			firstIndex = firstIndex + 1
		}
		lastIndex := strings.LastIndex(fragment, "/")
		if lastIndex < 0 {
			lastIndex = len(fragment)
		} else {
			lastIndex = lastIndex - 1
		}
		prefixSegment = fragment[firstIndex:lastIndex]
		break
	}
	copiedSegments := make([]string, copiedSegmentsLen)
	defer handleCopyFail(ctx, f, segmentsContainer, copiedSegments, err)
	for c, ss := range segments {
		if len(ss) <= 0 {
			continue
		}
		for _, s := range ss {
			lastIndex := strings.LastIndex(s, "/")
			if lastIndex <= 0 {
				lastIndex = 0
			} else {
				lastIndex = lastIndex + 1
			}
			segmentName := dstPath + "/" + prefixSegment + "/" + s[lastIndex:]
			err = f.pacer.Call(func() (bool, error) {
				var rxHeaders swift.Headers
				rxHeaders, err = f.c.ObjectCopy(ctx, c, s, segmentsContainer, segmentName, nil)
				copiedSegments = append(copiedSegments, segmentName)
				return shouldRetryHeaders(ctx, rxHeaders, err)
			})
			if err != nil {
				return err
			}
		}
	}
	m := swift.Metadata{}
	headers := m.ObjectHeaders()
	headers["X-Object-Manifest"] = urlEncode(fmt.Sprintf("%s/%s/%s", segmentsContainer, dstPath, prefixSegment))
	headers["Content-Length"] = "0"
	emptyReader := bytes.NewReader(nil)
	err = f.pacer.Call(func() (bool, error) {
		var rxHeaders swift.Headers
		rxHeaders, err = f.c.ObjectPut(ctx, dstContainer, dstPath, emptyReader, true, "", src.contentType, headers)
		return shouldRetryHeaders(ctx, rxHeaders, err)
	})
	return err
}

// remove copied segments when copy process failed
func handleCopyFail(ctx context.Context, f *Fs, segmentsContainer string, segments []string, err error) {
	fs.Debugf(f, "handle copy segment fail")
	if err == nil {
		return
	}
	if len(segmentsContainer) == 0 {
		fs.Debugf(f, "invalid segments container")
		return
	}
	if len(segments) == 0 {
		fs.Debugf(f, "segments is empty")
		return
	}
	fs.Debugf(f, "action delete segments what copied")
	for _, v := range segments {
		_ = f.c.ObjectDelete(ctx, segmentsContainer, v)
	}
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
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

// Hash returns the Md5sum of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	isDynamicLargeObject, err := o.isDynamicLargeObject(ctx)
	if err != nil {
		return "", err
	}
	isStaticLargeObject, err := o.isStaticLargeObject(ctx)
	if err != nil {
		return "", err
	}
	if isDynamicLargeObject || isStaticLargeObject {
		fs.Debugf(o, "Returning empty Md5sum for swift large object")
		return "", nil
	}
	return strings.ToLower(o.md5), nil
}

// hasHeader checks for the header passed in returning false if the
// object isn't found.
func (o *Object) hasHeader(ctx context.Context, header string) (bool, error) {
	err := o.readMetaData(ctx)
	if err != nil {
		if err == fs.ErrorObjectNotFound {
			return false, nil
		}
		return false, err
	}
	_, isDynamicLargeObject := o.headers[header]
	return isDynamicLargeObject, nil
}

// isDynamicLargeObject checks for X-Object-Manifest header
func (o *Object) isDynamicLargeObject(ctx context.Context) (bool, error) {
	if o.fs.opt.NoLargeObjects {
		return false, nil
	}
	return o.hasHeader(ctx, "X-Object-Manifest")
}

// isStaticLargeObjectFile checks for the X-Static-Large-Object header
func (o *Object) isStaticLargeObject(ctx context.Context) (bool, error) {
	if o.fs.opt.NoLargeObjects {
		return false, nil
	}
	return o.hasHeader(ctx, "X-Static-Large-Object")
}

func (o *Object) isLargeObject(ctx context.Context) (result bool, err error) {
	if o.fs.opt.NoLargeObjects {
		return false, nil
	}
	result, err = o.hasHeader(ctx, "X-Static-Large-Object")
	if result {
		return
	}
	result, err = o.hasHeader(ctx, "X-Object-Manifest")
	if result {
		return
	}
	return false, nil
}

func (o *Object) isInContainerVersioning(ctx context.Context, container string) (bool, error) {
	_, headers, err := o.fs.c.Container(ctx, container)
	if err != nil {
		return false, err
	}
	xHistoryLocation := headers["X-History-Location"]
	if len(xHistoryLocation) > 0 {
		return true, nil
	}
	return false, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// decodeMetaData sets the metadata in the object from a swift.Object
//
// Sets
//
//	o.lastModified
//	o.size
//	o.md5
//	o.contentType
func (o *Object) decodeMetaData(info *swift.Object) (err error) {
	o.lastModified = info.LastModified
	o.size = info.Bytes
	o.md5 = info.Hash
	o.contentType = info.ContentType
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
//
// it returns fs.ErrorObjectNotFound if the object isn't found
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.headers != nil {
		return nil
	}
	var info swift.Object
	var h swift.Headers
	container, containerPath := o.split()
	err = o.fs.pacer.Call(func() (bool, error) {
		info, h, err = o.fs.c.Object(ctx, container, containerPath)
		return shouldRetryHeaders(ctx, h, err)
	})
	if err != nil {
		if err == swift.ObjectNotFound {
			return fs.ErrorObjectNotFound
		}
		return err
	}
	o.headers = h
	err = o.decodeMetaData(&info)
	if err != nil {
		return err
	}
	return nil
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.fs.ci.UseServerModTime {
		return o.lastModified
	}
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Debugf(o, "Failed to read metadata: %s", err)
		return o.lastModified
	}
	modTime, err := o.headers.ObjectMetadata().GetModTime()
	if err != nil {
		// fs.Logf(o, "Failed to read mtime from object: %v", err)
		return o.lastModified
	}
	return modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	err := o.readMetaData(ctx)
	if err != nil {
		return err
	}
	meta := o.headers.ObjectMetadata()
	meta.SetModTime(modTime)
	newHeaders := meta.ObjectHeaders()
	for k, v := range newHeaders {
		o.headers[k] = v
	}
	// Include any other metadata from request
	for k, v := range o.headers {
		if strings.HasPrefix(k, "X-Object-") {
			newHeaders[k] = v
		}
	}
	container, containerPath := o.split()
	return o.fs.pacer.Call(func() (bool, error) {
		err = o.fs.c.ObjectUpdate(ctx, container, containerPath, newHeaders)
		return shouldRetry(ctx, err)
	})
}

// Storable returns if this object is storable
//
// It compares the Content-Type to directoryMarkerContentType - that
// makes it a directory marker which is not storable.
func (o *Object) Storable() bool {
	return o.contentType != directoryMarkerContentType
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	fs.FixRangeOption(options, o.size)
	headers := fs.OpenOptionHeaders(options)
	_, isRanging := headers["Range"]
	container, containerPath := o.split()
	err = o.fs.pacer.Call(func() (bool, error) {
		var rxHeaders swift.Headers
		in, rxHeaders, err = o.fs.c.ObjectOpen(ctx, container, containerPath, !isRanging, headers)
		return shouldRetryHeaders(ctx, rxHeaders, err)
	})
	return
}

// min returns the smallest of x, y
func min(x, y int64) int64 {
	if x < y {
		return x
	}
	return y
}

func (o *Object) getSegmentsLargeObject(ctx context.Context) (map[string][]string, error) {
	container, objectName := o.split()
	segmentContainer, segmentObjects, err := o.fs.c.LargeObjectGetSegments(ctx, container, objectName)
	if err != nil {
		fs.Debugf(o, "Failed to get list segments of object: %v", err)
		return nil, err
	}
	var containerSegments = make(map[string][]string)
	for _, segment := range segmentObjects {
		if _, ok := containerSegments[segmentContainer]; !ok {
			containerSegments[segmentContainer] = make([]string, 0, len(segmentObjects))
		}
		segments := containerSegments[segmentContainer]
		segments = append(segments, segment.Name)
		containerSegments[segmentContainer] = segments
	}
	return containerSegments, nil
}

func (o *Object) removeSegmentsLargeObject(ctx context.Context, containerSegments map[string][]string) error {
	if containerSegments == nil || len(containerSegments) <= 0 {
		return nil
	}
	for container, segments := range containerSegments {
		_, err := o.fs.c.BulkDelete(ctx, container, segments)
		if err != nil {
			fs.Debugf(o, "Failed to delete bulk segments %v", err)
			return err
		}
	}
	return nil
}

// urlEncode encodes a string so that it is a valid URL
//
// We don't use any of Go's standard methods as we need `/` not
// encoded but we need '&' encoded.
func urlEncode(str string) string {
	var buf bytes.Buffer
	for i := 0; i < len(str); i++ {
		c := str[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '/' || c == '.' || c == '_' || c == '-' {
			_ = buf.WriteByte(c)
		} else {
			_, _ = buf.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return buf.String()
}

// updateChunks updates the existing object using chunks to a separate
// container.  It returns a string which prefixes current segments.
func (o *Object) updateChunks(ctx context.Context, in0 io.Reader, headers swift.Headers, size int64, contentType string) (string, error) {
	container, containerPath := o.split()
	segmentsContainer := container + "_segments"
	// Create the segmentsContainer if it doesn't exist
	var err error
	err = o.fs.pacer.Call(func() (bool, error) {
		var rxHeaders swift.Headers
		_, rxHeaders, err = o.fs.c.Container(ctx, segmentsContainer)
		return shouldRetryHeaders(ctx, rxHeaders, err)
	})
	if err == swift.ContainerNotFound {
		headers := swift.Headers{}
		if o.fs.opt.StoragePolicy != "" {
			headers["X-Storage-Policy"] = o.fs.opt.StoragePolicy
		}
		err = o.fs.pacer.Call(func() (bool, error) {
			err = o.fs.c.ContainerCreate(ctx, segmentsContainer, headers)
			return shouldRetry(ctx, err)
		})
	}
	if err != nil {
		return "", err
	}
	// Upload the chunks
	left := size
	i := 0
	uniquePrefix := fmt.Sprintf("%s/%d", swift.TimeToFloatString(time.Now()), size)
	segmentsPath := path.Join(containerPath, uniquePrefix)
	in := bufio.NewReader(in0)
	segmentInfos := make([]string, 0, (size/int64(o.fs.opt.ChunkSize))+1)
	defer atexit.OnError(&err, func() {
		if o.fs.opt.LeavePartsOnError {
			return
		}
		fs.Debugf(o, "Delete segments when err raise %v", err)
		if len(segmentInfos) == 0 {
			return
		}
		_ctx := context.Background()
		deleteChunks(_ctx, o, segmentsContainer, segmentInfos)
	})()
	for {
		// can we read at least one byte?
		if _, err = in.Peek(1); err != nil {
			if left > 0 {
				return "", err // read less than expected
			}
			fs.Debugf(o, "Uploading segments into %q seems done (%v)", segmentsContainer, err)
			break
		}
		n := int64(o.fs.opt.ChunkSize)
		if size != -1 {
			n = min(left, n)
			headers["Content-Length"] = strconv.FormatInt(n, 10) // set Content-Length as we know it
			left -= n
		}
		segmentReader := io.LimitReader(in, n)
		segmentPath := fmt.Sprintf("%s/%08d", segmentsPath, i)
		fs.Debugf(o, "Uploading segment file %q into %q", segmentPath, segmentsContainer)
		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			var rxHeaders swift.Headers
			rxHeaders, err = o.fs.c.ObjectPut(ctx, segmentsContainer, segmentPath, segmentReader, true, "", "", headers)
			if err == nil {
				segmentInfos = append(segmentInfos, segmentPath)
			}
			return shouldRetryHeaders(ctx, rxHeaders, err)
		})
		if err != nil {
			return "", err
		}
		i++
	}
	// Upload the manifest
	headers["X-Object-Manifest"] = urlEncode(fmt.Sprintf("%s/%s", segmentsContainer, segmentsPath))
	headers["Content-Length"] = "0" // set Content-Length as we know it
	emptyReader := bytes.NewReader(nil)
	err = o.fs.pacer.Call(func() (bool, error) {
		var rxHeaders swift.Headers
		rxHeaders, err = o.fs.c.ObjectPut(ctx, container, containerPath, emptyReader, true, "", contentType, headers)
		return shouldRetryHeaders(ctx, rxHeaders, err)
	})

	if err == nil {
		//reset data
		segmentInfos = nil
	}
	return uniquePrefix + "/", err
}

func deleteChunks(ctx context.Context, o *Object, segmentsContainer string, segmentInfos []string) {
	if len(segmentInfos) == 0 {
		return
	}
	for _, v := range segmentInfos {
		fs.Debugf(o, "Delete segment file %q on %q", v, segmentsContainer)
		e := o.fs.c.ObjectDelete(ctx, segmentsContainer, v)
		if e != nil {
			fs.Errorf(o, "Error occurred in delete segment file %q on %q, error: %q", v, segmentsContainer, e)
		}
	}
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	container, containerPath := o.split()
	if container == "" {
		return fserrors.FatalError(errors.New("can't upload files to the root"))
	}
	err := o.fs.makeContainer(ctx, container)
	if err != nil {
		return err
	}
	size := src.Size()
	modTime := src.ModTime(ctx)

	// Note whether this is a dynamic large object before starting
	isLargeObject, err := o.isLargeObject(ctx)
	if err != nil {
		return err
	}

	//capture segments before upload
	var segmentsContainer map[string][]string
	if isLargeObject {
		segmentsContainer, _ = o.getSegmentsLargeObject(ctx)
	}

	// Set the mtime
	m := swift.Metadata{}
	m.SetModTime(modTime)
	contentType := fs.MimeType(ctx, src)
	headers := m.ObjectHeaders()
	fs.OpenOptionAddHeaders(options, headers)

	if (size > int64(o.fs.opt.ChunkSize) || (size == -1 && !o.fs.opt.NoChunk)) && !o.fs.opt.NoLargeObjects {
		_, err = o.updateChunks(ctx, in, headers, size, contentType)
		if err != nil {
			return err
		}
		o.headers = nil // wipe old metadata
	} else {
		var inCount *readers.CountingReader
		if size >= 0 {
			headers["Content-Length"] = strconv.FormatInt(size, 10) // set Content-Length if we know it
		} else {
			// otherwise count the size for later
			inCount = readers.NewCountingReader(in)
			in = inCount
		}
		var rxHeaders swift.Headers
		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			rxHeaders, err = o.fs.c.ObjectPut(ctx, container, containerPath, in, true, "", contentType, headers)
			return shouldRetryHeaders(ctx, rxHeaders, err)
		})
		if err != nil {
			return err
		}
		// set Metadata since ObjectPut checked the hash and length so we know the
		// object has been safely uploaded
		o.lastModified = modTime
		o.size = size
		o.md5 = rxHeaders["Etag"]
		o.contentType = contentType
		o.headers = headers
		if inCount != nil {
			// update the size if streaming from the reader
			o.size = int64(inCount.BytesRead())
		}
	}
	// If file was a large object and the container is not enable versioning then remove old/all segments
	if isLargeObject && len(segmentsContainer) > 0 {
		isInContainerVersioning, _ := o.isInContainerVersioning(ctx, container)
		if !isInContainerVersioning {
			err := o.removeSegmentsLargeObject(ctx, segmentsContainer)
			if err != nil {
				fs.Logf(o, "Failed to remove old segments - carrying on with upload: %v", err)
			}
		}
	}

	// Read the metadata from the newly created object if necessary
	return o.readMetaData(ctx)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) (err error) {
	container, containerPath := o.split()

	//check object is large object
	isLargeObject, err := o.isLargeObject(ctx)
	if err != nil {
		return err
	}
	//check container has enabled version to reserve segment when delete
	isInContainerVersioning := false
	if isLargeObject {
		isInContainerVersioning, err = o.isInContainerVersioning(ctx, container)
		if err != nil {
			return err
		}
	}
	//capture segments object if this object is large object
	var containerSegments map[string][]string
	if isLargeObject {
		containerSegments, err = o.getSegmentsLargeObject(ctx)
		if err != nil {
			return err
		}
	}
	// Remove file/manifest first
	err = o.fs.pacer.Call(func() (bool, error) {
		err = o.fs.c.ObjectDelete(ctx, container, containerPath)
		if err == swift.ObjectNotFound {
			fs.Errorf(o, "Dangling object - ignoring: %v", err)
			err = nil
		}
		return shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}

	if !isLargeObject || isInContainerVersioning {
		return nil
	}

	if isLargeObject {
		return o.removeSegmentsLargeObject(ctx, containerSegments)
	}
	return nil
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.contentType
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.Purger      = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.Copier      = &Fs{}
	_ fs.ListRer     = &Fs{}
	_ fs.Object      = &Object{}
	_ fs.MimeTyper   = &Object{}
)
