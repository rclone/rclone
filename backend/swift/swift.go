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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

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
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/readers"
)

// Constants
const (
	directoryMarkerContentType = "application/directory" // content type of directory marker objects
	listChunks                 = 1000                    // chunk size to read directory listings
	defaultChunkSize           = 5 * fs.Gibi
	minSleep                   = 10 * time.Millisecond // In case of error, start at 10ms sleep.
	segmentsContainerSuffix    = "_segments"
	segmentsDirectory          = ".file-segments"
	segmentsDirectorySlash     = segmentsDirectory + "/"
)

// Auth URLs which imply using fileSegmentsDirectory
var needFileSegmentsDirectory = regexp.MustCompile(`(?s)\.(ain?\.net|blomp\.com|praetector\.com|signmy\.name|rackfactory\.com)($|/)`)

// SharedOptions are shared between swift and backends which depend on swift
var SharedOptions = []fs.Option{{
	Name: "chunk_size",
	Help: strings.ReplaceAll(`Above this size files will be chunked.

Above this size files will be chunked into a a |`+segmentsContainerSuffix+`| container
or a |`+segmentsDirectory+`| directory. (See the |use_segments_container| option
for more info). Default for this is 5 GiB which is its maximum value, which
means only files above this size will be chunked.

Rclone uploads chunked files as dynamic large objects (DLO).
`, "|", "`"),
	Default:  defaultChunkSize,
	Advanced: true,
}, {
	Name: "no_chunk",
	Help: strings.ReplaceAll(`Don't chunk files during streaming upload.

When doing streaming uploads (e.g. using |rcat| or |mount| with
|--vfs-cache-mode off|) setting this flag will cause the swift backend
to not upload chunked files.

This will limit the maximum streamed upload size to 5 GiB. This is
useful because non chunked files are easier to deal with and have an
MD5SUM.

Rclone will still chunk files bigger than |chunk_size| when doing
normal copy operations.`, "|", "`"),
	Default:  false,
	Advanced: true,
}, {
	Name: "no_large_objects",
	Help: strings.ReplaceAll(`Disable support for static and dynamic large objects

Swift cannot transparently store files bigger than 5 GiB. There are
two schemes for chunking large files, static large objects (SLO) or
dynamic large objects (DLO), and the API does not allow rclone to
determine whether a file is a static or dynamic large object without
doing a HEAD on the object. Since these need to be treated
differently, this means rclone has to issue HEAD requests for objects
for example when reading checksums.

When |no_large_objects| is set, rclone will assume that there are no
static or dynamic large objects stored. This means it can stop doing
the extra HEAD calls which in turn increases performance greatly
especially when doing a swift to swift transfer with |--checksum| set.

Setting this option implies |no_chunk| and also that no files will be
uploaded in chunks, so files bigger than 5 GiB will just fail on
upload.

If you set this option and there **are** static or dynamic large objects,
then this will give incorrect hashes for them. Downloads will succeed,
but other operations such as Remove and Copy will fail.
`, "|", "`"),
	Default:  false,
	Advanced: true,
}, {
	Name: "use_segments_container",
	Help: strings.ReplaceAll(`Choose destination for large object segments

Swift cannot transparently store files bigger than 5 GiB and rclone
will chunk files larger than |chunk_size| (default 5 GiB) in order to
upload them.

If this value is |true| the chunks will be stored in an additional
container named the same as the destination container but with
|`+segmentsContainerSuffix+`| appended. This means that there won't be any duplicated
data in the original container but having another container may not be
acceptable.

If this value is |false| the chunks will be stored in a
|`+segmentsDirectory+`| directory in the root of the container. This
directory will be omitted when listing the container. Some
providers (eg Blomp) require this mode as creating additional
containers isn't allowed. If it is desired to see the |`+segmentsDirectory+`|
directory in the root then this flag must be set to |true|.

If this value is |unset| (the default), then rclone will choose the value
to use. It will be |false| unless rclone detects any |auth_url|s that
it knows need it to be |true|. In this case you'll see a message in
the DEBUG log.
`, "|", "`"),
	Default:  fs.Tristate{},
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
		}, {
			Name: "fetch_until_empty_page",
			Help: `When paginating, always fetch unless we received an empty page.

Consider using this option if rclone listings show fewer objects
than expected, or if repeated syncs copy unchanged objects.

It is safe to enable this, but rclone may make more API calls than
necessary.

This is one of a pair of workarounds to handle implementations
of the Swift API that do not implement pagination as expected.  See
also "partial_page_fetch_threshold".`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "partial_page_fetch_threshold",
			Help: `When paginating, fetch if the current page is within this percentage of the limit.

Consider using this option if rclone listings show fewer objects
than expected, or if repeated syncs copy unchanged objects.

It is safe to enable this, but rclone may make more API calls than
necessary.

This is one of a pair of workarounds to handle implementations
of the Swift API that do not implement pagination as expected.  See
also "fetch_until_empty_page".`,
			Default:  0,
			Advanced: true,
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
	UseSegmentsContainer        fs.Tristate          `config:"use_segments_container"`
	Enc                         encoder.MultiEncoder `config:"encoding"`
	FetchUntilEmptyPage         bool                 `config:"fetch_until_empty_page"`
	PartialPageFetchThreshold   int                  `config:"partial_page_fetch_threshold"`
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
		FetchUntilEmptyPage:         opt.FetchUntilEmptyPage,
		PartialPageFetchThreshold:   opt.PartialPageFetchThreshold,
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
	if !f.opt.UseSegmentsContainer.Valid {
		f.opt.UseSegmentsContainer.Value = !needFileSegmentsDirectory.MatchString(opt.Auth)
		f.opt.UseSegmentsContainer.Valid = true
		fs.Debugf(f, "Auto set use_segments_container to %v", f.opt.UseSegmentsContainer.Value)
	}
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
				if !includeDirMarkers && !f.opt.UseSegmentsContainer.Value && (object.Name == segmentsDirectory || strings.HasPrefix(object.Name, segmentsDirectorySlash)) {
					// Don't show segments in listing unless showing directory markers
					continue
				}
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
	var used, objects, total int64
	if f.rootContainer != "" {
		var container swift.Container
		err = f.pacer.Call(func() (bool, error) {
			container, _, err = f.c.Container(ctx, f.rootContainer)
			return shouldRetry(ctx, err)
		})
		if err != nil {
			return nil, fmt.Errorf("container info failed: %w", err)
		}
		used = container.Bytes
		objects = container.Count
		total = container.QuotaBytes
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
			used += c.Bytes
			objects += c.Count
			total += c.QuotaBytes
		}
	}
	usage = &fs.Usage{
		Used:    fs.NewUsageValue(used),    // bytes in use
		Objects: fs.NewUsageValue(objects), // objects in use
	}
	if total > 0 {
		usage.Total = fs.NewUsageValue(total)
		usage.Free = fs.NewUsageValue(total - used)
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
		// handle large object
		err = f.copyLargeObject(ctx, srcObj, dstContainer, dstPath)
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

// Represents a segmented upload or copy
type segmentedUpload struct {
	f            *Fs        // parent
	dstContainer string     // container for the file to live once uploaded
	container    string     // container for the segments
	dstPath      string     // path for the object to live once uploaded
	path         string     // unique path for the segments
	mu           sync.Mutex // protects the variables below
	segments     []string   // segments successfully uploaded
}

// Create a new segmented upload using the correct container and path
func (f *Fs) newSegmentedUpload(ctx context.Context, dstContainer string, dstPath string) (su *segmentedUpload, err error) {
	randomString, err := random.Password(128)
	if err != nil {
		return nil, fmt.Errorf("failed to create random string for upload: %w", err)
	}
	uniqueString := time.Now().Format("2006-01-02-150405") + "-" + randomString
	su = &segmentedUpload{
		f:            f,
		dstPath:      dstPath,
		path:         dstPath + "/" + uniqueString,
		dstContainer: dstContainer,
		container:    dstContainer,
	}
	if f.opt.UseSegmentsContainer.Value {
		su.container += segmentsContainerSuffix
		err = f.makeContainer(ctx, su.container)
		if err != nil {
			return nil, err
		}
	} else {
		su.path = segmentsDirectorySlash + su.path
	}
	return su, nil
}

// Return the path of the i-th segment
func (su *segmentedUpload) segmentPath(i int) string {
	return fmt.Sprintf("%s/%08d", su.path, i)
}

// Mark segment as successfully uploaded
func (su *segmentedUpload) uploaded(segment string) {
	su.mu.Lock()
	defer su.mu.Unlock()
	su.segments = append(su.segments, segment)
}

// Return the full path including the container
func (su *segmentedUpload) fullPath() string {
	return fmt.Sprintf("%s/%s", su.container, su.path)
}

// Remove segments when upload/copy process fails
func (su *segmentedUpload) onFail() {
	f := su.f
	if f.opt.LeavePartsOnError {
		return
	}
	ctx := context.Background()
	fs.Debugf(f, "Segment operation failed: bulk deleting failed segments")
	if len(su.container) == 0 {
		fs.Debugf(f, "Invalid segments container")
		return
	}
	if len(su.segments) == 0 {
		fs.Debugf(f, "No segments to delete")
		return
	}
	_, err := f.c.BulkDelete(ctx, su.container, su.segments)
	if err != nil {
		fs.Errorf(f, "Failed to bulk delete failed segments: %v", err)
	}
}

// upload the manifest when upload is done
func (su *segmentedUpload) uploadManifest(ctx context.Context, contentType string, headers swift.Headers) (err error) {
	delete(headers, "Etag") // remove Etag if present as it is wrong for the manifest
	headers["X-Object-Manifest"] = urlEncode(su.fullPath())
	headers["Content-Length"] = "0" // set Content-Length as we know it
	emptyReader := bytes.NewReader(nil)
	fs.Debugf(su.f, "uploading manifest %q to %q", su.dstPath, su.dstContainer)
	err = su.f.pacer.Call(func() (bool, error) {
		var rxHeaders swift.Headers
		rxHeaders, err = su.f.c.ObjectPut(ctx, su.dstContainer, su.dstPath, emptyReader, true, "", contentType, headers)
		return shouldRetryHeaders(ctx, rxHeaders, err)
	})
	return err
}

// Copy a large object src into (dstContainer, dstPath)
func (f *Fs) copyLargeObject(ctx context.Context, src *Object, dstContainer string, dstPath string) (err error) {
	su, err := f.newSegmentedUpload(ctx, dstContainer, dstPath)
	if err != nil {
		return err
	}
	srcSegmentsContainer, srcSegments, err := src.getSegmentsLargeObject(ctx)
	if err != nil {
		return fmt.Errorf("copy large object: %w", err)
	}
	if len(srcSegments) == 0 {
		return errors.New("could not copy object, list segments are empty")
	}
	defer atexit.OnError(&err, su.onFail)()
	for i, srcSegment := range srcSegments {
		dstSegment := su.segmentPath(i)
		err = f.pacer.Call(func() (bool, error) {
			var rxHeaders swift.Headers
			rxHeaders, err = f.c.ObjectCopy(ctx, srcSegmentsContainer, srcSegment, su.container, dstSegment, nil)
			return shouldRetryHeaders(ctx, rxHeaders, err)
		})
		if err != nil {
			return err
		}
		su.uploaded(dstSegment)
	}
	return su.uploadManifest(ctx, src.contentType, src.headers)
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

// Get the segments for a large object
//
// It returns the names of the segments and the container that they live in
func (o *Object) getSegmentsLargeObject(ctx context.Context) (container string, segments []string, err error) {
	container, objectName := o.split()
	container, segmentObjects, err := o.fs.c.LargeObjectGetSegments(ctx, container, objectName)
	if err != nil {
		return container, segments, fmt.Errorf("failed to get list segments of object: %w", err)
	}
	segments = make([]string, len(segmentObjects))
	for i := range segmentObjects {
		segments[i] = segmentObjects[i].Name
	}
	return container, segments, nil
}

// Remove the segments for a large object
func (o *Object) removeSegmentsLargeObject(ctx context.Context, container string, segments []string) error {
	if len(segments) == 0 {
		return nil
	}
	_, err := o.fs.c.BulkDelete(ctx, container, segments)
	if err != nil {
		return fmt.Errorf("failed to delete bulk segments: %w", err)
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
// container.
func (o *Object) updateChunks(ctx context.Context, in0 io.Reader, headers swift.Headers, size int64, contentType string) (err error) {
	container, containerPath := o.split()
	su, err := o.fs.newSegmentedUpload(ctx, container, containerPath)
	if err != nil {
		return err
	}
	// Upload the chunks
	left := size
	i := 0
	in := bufio.NewReader(in0)
	defer atexit.OnError(&err, su.onFail)()
	for {
		// can we read at least one byte?
		if _, err = in.Peek(1); err != nil {
			if left > 0 {
				return err // read less than expected
			}
			fs.Debugf(o, "Uploading segments into %q seems done (%v)", su.container, err)
			break
		}
		n := int64(o.fs.opt.ChunkSize)
		if size != -1 {
			n = min(left, n)
			headers["Content-Length"] = strconv.FormatInt(n, 10) // set Content-Length as we know it
			left -= n
		}
		segmentReader := io.LimitReader(in, n)
		segmentPath := su.segmentPath(i)
		fs.Debugf(o, "Uploading segment file %q into %q", segmentPath, su.container)
		err = o.fs.pacer.CallNoRetry(func() (bool, error) {
			var rxHeaders swift.Headers
			rxHeaders, err = o.fs.c.ObjectPut(ctx, su.container, segmentPath, segmentReader, true, "", "", headers)
			return shouldRetryHeaders(ctx, rxHeaders, err)
		})
		if err != nil {
			return err
		}
		su.uploaded(segmentPath)
		i++
	}
	return su.uploadManifest(ctx, contentType, headers)
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

	// Capture segments before upload
	var segmentsContainer string
	var segments []string
	if isLargeObject {
		segmentsContainer, segments, _ = o.getSegmentsLargeObject(ctx)
	}

	// Set the mtime
	m := swift.Metadata{}
	m.SetModTime(modTime)
	contentType := fs.MimeType(ctx, src)
	headers := m.ObjectHeaders()
	fs.OpenOptionAddHeaders(options, headers)

	if (size > int64(o.fs.opt.ChunkSize) || (size == -1 && !o.fs.opt.NoChunk)) && !o.fs.opt.NoLargeObjects {
		err = o.updateChunks(ctx, in, headers, size, contentType)
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
			err := o.removeSegmentsLargeObject(ctx, segmentsContainer, segments)
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
	// Capture segments object if this object is large object
	var segmentsContainer string
	var segments []string
	if isLargeObject {
		segmentsContainer, segments, err = o.getSegmentsLargeObject(ctx)
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
		return o.removeSegmentsLargeObject(ctx, segmentsContainer, segments)
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
