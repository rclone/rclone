// Package azureblob provides an interface to the Microsoft Azure blob object storage system

// +build !plan9,!solaris,!js,go1.14

package azureblob

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/env"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/pool"
)

const (
	minSleep              = 10 * time.Millisecond
	maxSleep              = 10 * time.Second
	decayConstant         = 1    // bigger for slower decay, exponential
	maxListChunkSize      = 5000 // number of items to read at once
	modTimeKey            = "mtime"
	timeFormatIn          = time.RFC3339
	timeFormatOut         = "2006-01-02T15:04:05.000000000Z07:00"
	storageDefaultBaseURL = "blob.core.windows.net"
	defaultChunkSize      = 4 * fs.MebiByte
	maxChunkSize          = 100 * fs.MebiByte
	uploadConcurrency     = 4
	defaultAccessTier     = azblob.AccessTierNone
	maxTryTimeout         = time.Hour * 24 * 365 //max time of an azure web request response window (whether or not data is flowing)
	// Default storage account, key and blob endpoint for emulator support,
	// though it is a base64 key checked in here, it is publicly available secret.
	emulatorAccount      = "devstoreaccount1"
	emulatorAccountKey   = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
	emulatorBlobEndpoint = "http://127.0.0.1:10000/devstoreaccount1"
	memoryPoolFlushTime  = fs.Duration(time.Minute) // flush the cached buffers after this long
	memoryPoolUseMmap    = false
)

var (
	errCantUpdateArchiveTierBlobs = fserrors.NoRetryError(errors.New("can't update archive tier blob without --azureblob-archive-tier-delete"))
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "azureblob",
		Description: "Microsoft Azure Blob Storage",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "account",
			Help: "Storage Account Name (leave blank to use SAS URL or Emulator)",
		}, {
			Name: "service_principal_file",
			Help: `Path to file containing credentials for use with a service principal.

Leave blank normally. Needed only if you want to use a service principal instead of interactive login.

    $ az sp create-for-rbac --name "<name>" \
      --role "Storage Blob Data Owner" \
      --scopes "/subscriptions/<subscription>/resourceGroups/<resource-group>/providers/Microsoft.Storage/storageAccounts/<storage-account>/blobServices/default/containers/<container>" \
      > azure-principal.json

See [Use Azure CLI to assign an Azure role for access to blob and queue data](https://docs.microsoft.com/en-us/azure/storage/common/storage-auth-aad-rbac-cli)
for more details.
`,
		}, {
			Name: "key",
			Help: "Storage Account Key (leave blank to use SAS URL or Emulator)",
		}, {
			Name: "sas_url",
			Help: "SAS URL for container level access only\n(leave blank if using account/key or Emulator)",
		}, {
			Name: "use_msi",
			Help: `Use a managed service identity to authenticate (only works in Azure)

When true, use a [managed service identity](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/)
to authenticate to Azure Storage instead of a SAS token or account key.

If the VM(SS) on which this program is running has a system-assigned identity, it will
be used by default. If the resource has no system-assigned but exactly one user-assigned identity,
the user-assigned identity will be used by default. If the resource has multiple user-assigned
identities, the identity to use must be explicitly specified using exactly one of the msi_object_id,
msi_client_id, or msi_mi_res_id parameters.`,
			Default: false,
		}, {
			Name:     "msi_object_id",
			Help:     "Object ID of the user-assigned MSI to use, if any. Leave blank if msi_client_id or msi_mi_res_id specified.",
			Advanced: true,
		}, {
			Name:     "msi_client_id",
			Help:     "Object ID of the user-assigned MSI to use, if any. Leave blank if msi_object_id or msi_mi_res_id specified.",
			Advanced: true,
		}, {
			Name:     "msi_mi_res_id",
			Help:     "Azure resource ID of the user-assigned MSI to use, if any. Leave blank if msi_client_id or msi_object_id specified.",
			Advanced: true,
		}, {
			Name:    "use_emulator",
			Help:    "Uses local storage emulator if provided as 'true' (leave blank if using real azure storage endpoint)",
			Default: false,
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for the service\nLeave blank normally.",
			Advanced: true,
		}, {
			Name:     "upload_cutoff",
			Help:     "Cutoff for switching to chunked upload (<= 256MB). (Deprecated)",
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: `Upload chunk size (<= 100MB).

Note that this is stored in memory and there may be up to
"--transfers" chunks stored at once in memory.`,
			Default:  defaultChunkSize,
			Advanced: true,
		}, {
			Name: "list_chunk",
			Help: `Size of blob list.

This sets the number of blobs requested in each listing chunk. Default
is the maximum, 5000. "List blobs" requests are permitted 2 minutes
per megabyte to complete. If an operation is taking longer than 2
minutes per megabyte on average, it will time out (
[source](https://docs.microsoft.com/en-us/rest/api/storageservices/setting-timeouts-for-blob-service-operations#exceptions-to-default-timeout-interval)
). This can be used to limit the number of blobs items to return, to
avoid the time out.`,
			Default:  maxListChunkSize,
			Advanced: true,
		}, {
			Name: "access_tier",
			Help: `Access tier of blob: hot, cool or archive.

Archived blobs can be restored by setting access tier to hot or
cool. Leave blank if you intend to use default access tier, which is
set at account level

If there is no "access tier" specified, rclone doesn't apply any tier.
rclone performs "Set Tier" operation on blobs while uploading, if objects
are not modified, specifying "access tier" to new one will have no effect.
If blobs are in "archive tier" at remote, trying to perform data transfer
operations from remote will not be allowed. User should first restore by
tiering blob to "Hot" or "Cool".`,
			Advanced: true,
		}, {
			Name:    "archive_tier_delete",
			Default: false,
			Help: fmt.Sprintf(`Delete archive tier blobs before overwriting.

Archive tier blobs cannot be updated. So without this flag, if you
attempt to update an archive tier blob, then rclone will produce the
error:

    %v

With this flag set then before rclone attempts to overwrite an archive
tier blob, it will delete the existing blob before uploading its
replacement.  This has the potential for data loss if the upload fails
(unlike updating a normal blob) and also may cost more since deleting
archive tier blobs early may be chargable.
`, errCantUpdateArchiveTierBlobs),
			Advanced: true,
		}, {
			Name: "disable_checksum",
			Help: `Don't store MD5 checksum with object metadata.

Normally rclone will calculate the MD5 checksum of the input before
uploading it so it can add it to metadata on the object. This is great
for data integrity checking but can cause long delays for large files
to start uploading.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:     "memory_pool_flush_time",
			Default:  memoryPoolFlushTime,
			Advanced: true,
			Help: `How often internal memory buffer pools will be flushed.
Uploads which requires additional buffers (f.e multipart) will use memory pool for allocations.
This option controls how often unused buffers will be removed from the pool.`,
		}, {
			Name:     "memory_pool_use_mmap",
			Default:  memoryPoolUseMmap,
			Advanced: true,
			Help:     `Whether to use mmap buffers in internal memory pool.`,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.EncodeInvalidUtf8 |
				encoder.EncodeSlash |
				encoder.EncodeCtl |
				encoder.EncodeDel |
				encoder.EncodeBackSlash |
				encoder.EncodeRightPeriod),
		}, {
			Name:    "public_access",
			Help:    "Public access level of a container: blob, container.",
			Default: string(azblob.PublicAccessNone),
			Examples: []fs.OptionExample{
				{
					Value: string(azblob.PublicAccessNone),
					Help:  "The container and its blobs can be accessed only with an authorized request. It's a default value",
				}, {
					Value: string(azblob.PublicAccessBlob),
					Help:  "Blob data within this container can be read via anonymous request.",
				}, {
					Value: string(azblob.PublicAccessContainer),
					Help:  "Allow full public read access for container and blob data.",
				},
			},
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Account              string               `config:"account"`
	ServicePrincipalFile string               `config:"service_principal_file"`
	Key                  string               `config:"key"`
	UseMSI               bool                 `config:"use_msi"`
	MSIObjectID          string               `config:"msi_object_id"`
	MSIClientID          string               `config:"msi_client_id"`
	MSIResourceID        string               `config:"msi_mi_res_id"`
	Endpoint             string               `config:"endpoint"`
	SASURL               string               `config:"sas_url"`
	ChunkSize            fs.SizeSuffix        `config:"chunk_size"`
	ListChunkSize        uint                 `config:"list_chunk"`
	AccessTier           string               `config:"access_tier"`
	ArchiveTierDelete    bool                 `config:"archive_tier_delete"`
	UseEmulator          bool                 `config:"use_emulator"`
	DisableCheckSum      bool                 `config:"disable_checksum"`
	MemoryPoolFlushTime  fs.Duration          `config:"memory_pool_flush_time"`
	MemoryPoolUseMmap    bool                 `config:"memory_pool_use_mmap"`
	Enc                  encoder.MultiEncoder `config:"encoding"`
	PublicAccess         string               `config:"public_access"`
}

// Fs represents a remote azure server
type Fs struct {
	name          string                          // name of this remote
	root          string                          // the path we are working on if any
	opt           Options                         // parsed config options
	ci            *fs.ConfigInfo                  // global config
	features      *fs.Features                    // optional features
	client        *http.Client                    // http client we are using
	svcURL        *azblob.ServiceURL              // reference to serviceURL
	cntURLcacheMu sync.Mutex                      // mutex to protect cntURLcache
	cntURLcache   map[string]*azblob.ContainerURL // reference to containerURL per container
	rootContainer string                          // container part of root (if any)
	rootDirectory string                          // directory part of root (if any)
	isLimited     bool                            // if limited to one container
	cache         *bucket.Cache                   // cache for container creation status
	pacer         *fs.Pacer                       // To pace and retry the API calls
	imdsPacer     *fs.Pacer                       // Same but for IMDS
	uploadToken   *pacer.TokenDispenser           // control concurrency
	pool          *pool.Pool                      // memory pool
	publicAccess  azblob.PublicAccessType         // Container Public Access Level
}

// Object describes an azure object
type Object struct {
	fs         *Fs                   // what this object is part of
	remote     string                // The remote path
	modTime    time.Time             // The modified time of the object if known
	md5        string                // MD5 hash if known
	size       int64                 // Size of the object
	mimeType   string                // Content-Type of the object
	accessTier azblob.AccessTierType // Blob Access Tier
	meta       map[string]string     // blob metadata
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
		return "Azure root"
	}
	if f.rootDirectory == "" {
		return fmt.Sprintf("Azure container %s", f.rootContainer)
	}
	return fmt.Sprintf("Azure container %s path %s", f.rootContainer, f.rootDirectory)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a remote 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// split returns container and containerPath from the rootRelativePath
// relative to f.root
func (f *Fs) split(rootRelativePath string) (containerName, containerPath string) {
	containerName, containerPath = bucket.Split(path.Join(f.root, rootRelativePath))
	return f.opt.Enc.FromStandardName(containerName), f.opt.Enc.FromStandardPath(containerPath)
}

// split returns container and containerPath from the object
func (o *Object) split() (container, containerPath string) {
	return o.fs.split(o.remote)
}

// validateAccessTier checks if azureblob supports user supplied tier
func validateAccessTier(tier string) bool {
	switch tier {
	case string(azblob.AccessTierHot),
		string(azblob.AccessTierCool),
		string(azblob.AccessTierArchive):
		// valid cases
		return true
	default:
		return false
	}
}

// validatePublicAccess checks if azureblob supports use supplied public access level
func validatePublicAccess(publicAccess string) bool {
	switch publicAccess {
	case string(azblob.PublicAccessNone),
		string(azblob.PublicAccessBlob),
		string(azblob.PublicAccessContainer):
		// valid cases
		return true
	default:
		return false
	}
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	401, // Unauthorized (e.g. "Token has expired")
	408, // Request Timeout
	429, // Rate exceeded.
	500, // Get occasional 500 Internal Server Error
	503, // Service Unavailable
	504, // Gateway Time-out
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetry(ctx context.Context, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	// FIXME interpret special errors - more to do here
	if storageErr, ok := err.(azblob.StorageError); ok {
		switch storageErr.ServiceCode() {
		case "InvalidBlobOrBlock":
			// These errors happen sometimes in multipart uploads
			// because of block concurrency issues
			return true, err
		}
		statusCode := storageErr.Response().StatusCode
		for _, e := range retryErrorCodes {
			if statusCode == e {
				return true, err
			}
		}
	} else if httpErr, ok := err.(httpError); ok {
		return fserrors.ShouldRetryHTTP(httpErr.Response, retryErrorCodes), err
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

// httpClientFactory creates a Factory object that sends HTTP requests
// to an rclone's http.Client.
//
// copied from azblob.newDefaultHTTPClientFactory
func httpClientFactory(client *http.Client) pipeline.Factory {
	return pipeline.FactoryFunc(func(next pipeline.Policy, po *pipeline.PolicyOptions) pipeline.PolicyFunc {
		return func(ctx context.Context, request pipeline.Request) (pipeline.Response, error) {
			r, err := client.Do(request.WithContext(ctx))
			if err != nil {
				err = pipeline.NewError(err, "HTTP request failed")
			}
			return pipeline.NewHTTPResponse(r), err
		}
	})
}

type servicePrincipalCredentials struct {
	AppID    string `json:"appId"`
	Password string `json:"password"`
	Tenant   string `json:"tenant"`
}

const azureActiveDirectoryEndpoint = "https://login.microsoftonline.com/"
const azureStorageEndpoint = "https://storage.azure.com/"

// newServicePrincipalTokenRefresher takes the client ID and secret, and returns a refresh-able access token.
func newServicePrincipalTokenRefresher(ctx context.Context, credentialsData []byte) (azblob.TokenRefresher, error) {
	var spCredentials servicePrincipalCredentials
	if err := json.Unmarshal(credentialsData, &spCredentials); err != nil {
		return nil, errors.Wrap(err, "error parsing credentials from JSON file")
	}
	oauthConfig, err := adal.NewOAuthConfig(azureActiveDirectoryEndpoint, spCredentials.Tenant)
	if err != nil {
		return nil, errors.Wrap(err, "error creating oauth config")
	}

	// Create service principal token for Azure Storage.
	servicePrincipalToken, err := adal.NewServicePrincipalToken(
		*oauthConfig,
		spCredentials.AppID,
		spCredentials.Password,
		azureStorageEndpoint)
	if err != nil {
		return nil, errors.Wrap(err, "error creating service principal token")
	}

	// Wrap token inside a refresher closure.
	var tokenRefresher azblob.TokenRefresher = func(credential azblob.TokenCredential) time.Duration {
		if err := servicePrincipalToken.Refresh(); err != nil {
			panic(err)
		}
		refreshedToken := servicePrincipalToken.Token()
		credential.SetToken(refreshedToken.AccessToken)
		exp := refreshedToken.Expires().Sub(time.Now().Add(2 * time.Minute))
		return exp
	}

	return tokenRefresher, nil
}

// newPipeline creates a Pipeline using the specified credentials and options.
//
// this code was copied from azblob.NewPipeline
func (f *Fs) newPipeline(c azblob.Credential, o azblob.PipelineOptions) pipeline.Pipeline {
	// Don't log stuff to syslog/Windows Event log
	pipeline.SetForceLogEnabled(false)

	// Closest to API goes first; closest to the wire goes last
	factories := []pipeline.Factory{
		azblob.NewTelemetryPolicyFactory(o.Telemetry),
		azblob.NewUniqueRequestIDPolicyFactory(),
		azblob.NewRetryPolicyFactory(o.Retry),
		c,
		pipeline.MethodFactoryMarker(), // indicates at what stage in the pipeline the method factory is invoked
		azblob.NewRequestLogPolicyFactory(o.RequestLog),
	}
	return pipeline.NewPipeline(factories, pipeline.Options{HTTPSender: httpClientFactory(f.client), Log: o.Log})
}

// setRoot changes the root of the Fs
func (f *Fs) setRoot(root string) {
	f.root = parsePath(root)
	f.rootContainer, f.rootDirectory = bucket.Split(f.root)
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
		return nil, errors.Wrap(err, "azure: chunk size")
	}
	if opt.ListChunkSize > maxListChunkSize {
		return nil, errors.Errorf("azure: blob list size can't be greater than %v - was %v", maxListChunkSize, opt.ListChunkSize)
	}
	if opt.Endpoint == "" {
		opt.Endpoint = storageDefaultBaseURL
	}

	if opt.AccessTier == "" {
		opt.AccessTier = string(defaultAccessTier)
	} else if !validateAccessTier(opt.AccessTier) {
		return nil, errors.Errorf("Azure Blob: Supported access tiers are %s, %s and %s",
			string(azblob.AccessTierHot), string(azblob.AccessTierCool), string(azblob.AccessTierArchive))
	}

	if !validatePublicAccess((opt.PublicAccess)) {
		return nil, errors.Errorf("Azure Blob: Supported public access level are %s and %s",
			string(azblob.PublicAccessBlob), string(azblob.PublicAccessContainer))
	}

	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:        name,
		opt:         *opt,
		ci:          ci,
		pacer:       fs.NewPacer(ctx, pacer.NewS3(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		imdsPacer:   fs.NewPacer(ctx, pacer.NewAzureIMDS()),
		uploadToken: pacer.NewTokenDispenser(ci.Transfers),
		client:      fshttp.NewClient(ctx),
		cache:       bucket.NewCache(),
		cntURLcache: make(map[string]*azblob.ContainerURL, 1),
		pool: pool.New(
			time.Duration(opt.MemoryPoolFlushTime),
			int(opt.ChunkSize),
			ci.Transfers,
			opt.MemoryPoolUseMmap,
		),
	}
	f.publicAccess = azblob.PublicAccessType(opt.PublicAccess)
	f.imdsPacer.SetRetries(5) // per IMDS documentation
	f.setRoot(root)
	f.features = (&fs.Features{
		ReadMimeType:      true,
		WriteMimeType:     true,
		BucketBased:       true,
		BucketBasedRootOK: true,
		SetTier:           true,
		GetTier:           true,
	}).Fill(ctx, f)

	var (
		u          *url.URL
		serviceURL azblob.ServiceURL
	)
	switch {
	case opt.UseEmulator:
		credential, err := azblob.NewSharedKeyCredential(emulatorAccount, emulatorAccountKey)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse credentials")
		}
		u, err = url.Parse(emulatorBlobEndpoint)
		if err != nil {
			return nil, errors.Wrap(err, "failed to make azure storage url from account and endpoint")
		}
		pipeline := f.newPipeline(credential, azblob.PipelineOptions{Retry: azblob.RetryOptions{TryTimeout: maxTryTimeout}})
		serviceURL = azblob.NewServiceURL(*u, pipeline)
	case opt.UseMSI:
		var token adal.Token
		var userMSI *userMSI = &userMSI{}
		if len(opt.MSIClientID) > 0 || len(opt.MSIObjectID) > 0 || len(opt.MSIResourceID) > 0 {
			// Specifying a user-assigned identity. Exactly one of the above IDs must be specified.
			// Validate and ensure exactly one is set. (To do: better validation.)
			if len(opt.MSIClientID) > 0 {
				if len(opt.MSIObjectID) > 0 || len(opt.MSIResourceID) > 0 {
					return nil, errors.New("more than one user-assigned identity ID is set")
				}
				userMSI.Type = msiClientID
				userMSI.Value = opt.MSIClientID
			}
			if len(opt.MSIObjectID) > 0 {
				if len(opt.MSIClientID) > 0 || len(opt.MSIResourceID) > 0 {
					return nil, errors.New("more than one user-assigned identity ID is set")
				}
				userMSI.Type = msiObjectID
				userMSI.Value = opt.MSIObjectID
			}
			if len(opt.MSIResourceID) > 0 {
				if len(opt.MSIClientID) > 0 || len(opt.MSIObjectID) > 0 {
					return nil, errors.New("more than one user-assigned identity ID is set")
				}
				userMSI.Type = msiResourceID
				userMSI.Value = opt.MSIResourceID
			}
		} else {
			userMSI = nil
		}
		err = f.imdsPacer.Call(func() (bool, error) {
			// Retry as specified by the documentation:
			// https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/how-to-use-vm-token#retry-guidance
			token, err = GetMSIToken(ctx, userMSI)
			return f.shouldRetry(ctx, err)
		})

		if err != nil {
			return nil, errors.Wrapf(err, "Failed to acquire MSI token")
		}

		u, err = url.Parse(fmt.Sprintf("https://%s.%s", opt.Account, opt.Endpoint))
		if err != nil {
			return nil, errors.Wrap(err, "failed to make azure storage url from account and endpoint")
		}
		credential := azblob.NewTokenCredential(token.AccessToken, func(credential azblob.TokenCredential) time.Duration {
			fs.Debugf(f, "Token refresher called.")
			var refreshedToken adal.Token
			err := f.imdsPacer.Call(func() (bool, error) {
				refreshedToken, err = GetMSIToken(ctx, userMSI)
				return f.shouldRetry(ctx, err)
			})
			if err != nil {
				// Failed to refresh.
				return 0
			}
			credential.SetToken(refreshedToken.AccessToken)
			now := time.Now().UTC()
			// Refresh one minute before expiry.
			refreshAt := refreshedToken.Expires().UTC().Add(-1 * time.Minute)
			fs.Debugf(f, "Acquired new token that expires at %v; refreshing in %d s", refreshedToken.Expires(),
				int(refreshAt.Sub(now).Seconds()))
			if now.After(refreshAt) {
				// Acquired a causality violation.
				return 0
			}
			return refreshAt.Sub(now)
		})
		pipeline := f.newPipeline(credential, azblob.PipelineOptions{Retry: azblob.RetryOptions{TryTimeout: maxTryTimeout}})
		serviceURL = azblob.NewServiceURL(*u, pipeline)
	case opt.Account != "" && opt.Key != "":
		credential, err := azblob.NewSharedKeyCredential(opt.Account, opt.Key)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse credentials")
		}

		u, err = url.Parse(fmt.Sprintf("https://%s.%s", opt.Account, opt.Endpoint))
		if err != nil {
			return nil, errors.Wrap(err, "failed to make azure storage url from account and endpoint")
		}
		pipeline := f.newPipeline(credential, azblob.PipelineOptions{Retry: azblob.RetryOptions{TryTimeout: maxTryTimeout}})
		serviceURL = azblob.NewServiceURL(*u, pipeline)
	case opt.SASURL != "":
		u, err = url.Parse(opt.SASURL)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse SAS URL")
		}
		// use anonymous credentials in case of sas url
		pipeline := f.newPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{Retry: azblob.RetryOptions{TryTimeout: maxTryTimeout}})
		// Check if we have container level SAS or account level sas
		parts := azblob.NewBlobURLParts(*u)
		if parts.ContainerName != "" {
			if f.rootContainer != "" && parts.ContainerName != f.rootContainer {
				return nil, errors.New("Container name in SAS URL and container provided in command do not match")
			}
			containerURL := azblob.NewContainerURL(*u, pipeline)
			f.cntURLcache[parts.ContainerName] = &containerURL
			f.isLimited = true
		} else {
			serviceURL = azblob.NewServiceURL(*u, pipeline)
		}
	case opt.ServicePrincipalFile != "":
		// Create a standard URL.
		u, err = url.Parse(fmt.Sprintf("https://%s.%s", opt.Account, opt.Endpoint))
		if err != nil {
			return nil, errors.Wrap(err, "failed to make azure storage url from account and endpoint")
		}
		// Try loading service principal credentials from file.
		loadedCreds, err := ioutil.ReadFile(env.ShellExpand(opt.ServicePrincipalFile))
		if err != nil {
			return nil, errors.Wrap(err, "error opening service principal credentials file")
		}
		// Create a token refresher from service principal credentials.
		tokenRefresher, err := newServicePrincipalTokenRefresher(ctx, loadedCreds)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create a service principal token")
		}
		options := azblob.PipelineOptions{Retry: azblob.RetryOptions{TryTimeout: maxTryTimeout}}
		pipe := f.newPipeline(azblob.NewTokenCredential("", tokenRefresher), options)
		serviceURL = azblob.NewServiceURL(*u, pipe)
	default:
		return nil, errors.New("No authentication method configured")
	}
	f.svcURL = &serviceURL

	if f.rootContainer != "" && f.rootDirectory != "" {
		// Check to see if the (container,directory) is actually an existing file
		oldRoot := f.root
		newRoot, leaf := path.Split(oldRoot)
		f.setRoot(newRoot)
		_, err := f.NewObject(ctx, leaf)
		if err != nil {
			if err == fs.ErrorObjectNotFound || err == fs.ErrorNotAFile {
				// File doesn't exist or is a directory so return old f
				f.setRoot(oldRoot)
				return f, nil
			}
			return nil, err
		}
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// return the container URL for the container passed in
func (f *Fs) cntURL(container string) (containerURL *azblob.ContainerURL) {
	f.cntURLcacheMu.Lock()
	defer f.cntURLcacheMu.Unlock()
	var ok bool
	if containerURL, ok = f.cntURLcache[container]; !ok {
		cntURL := f.svcURL.NewContainerURL(container)
		containerURL = &cntURL
		f.cntURLcache[container] = containerURL
	}
	return containerURL

}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(remote string, info *azblob.BlobItemInternal) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		err := o.decodeMetaDataFromBlob(info)
		if err != nil {
			return nil, err
		}
	} else {
		err := o.readMetaData() // reads info and headers, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(remote, nil)
}

// getBlobReference creates an empty blob reference with no metadata
func (f *Fs) getBlobReference(container, containerPath string) azblob.BlobURL {
	return f.cntURL(container).NewBlobURL(containerPath)
}

// updateMetadataWithModTime adds the modTime passed in to o.meta.
func (o *Object) updateMetadataWithModTime(modTime time.Time) {
	// Make sure o.meta is not nil
	if o.meta == nil {
		o.meta = make(map[string]string, 1)
	}

	// Set modTimeKey in it
	o.meta[modTimeKey] = modTime.Format(timeFormatOut)
}

// Returns whether file is a directory marker or not
func isDirectoryMarker(size int64, metadata azblob.Metadata, remote string) bool {
	// Directory markers are 0 length
	if size == 0 {
		// Note that metadata with hdi_isfolder = true seems to be a
		// defacto standard for marking blobs as directories.
		endsWithSlash := strings.HasSuffix(remote, "/")
		if endsWithSlash || remote == "" || metadata["hdi_isfolder"] == "true" {
			return true
		}

	}
	return false
}

// listFn is called from list to handle an object
type listFn func(remote string, object *azblob.BlobItemInternal, isDirectory bool) error

// list lists the objects into the function supplied from
// the container and root supplied
//
// dir is the starting directory, "" for root
//
// The remote has prefix removed from it and if addContainer is set then
// it adds the container to the start.
func (f *Fs) list(ctx context.Context, container, directory, prefix string, addContainer bool, recurse bool, maxResults uint, fn listFn) error {
	if f.cache.IsDeleted(container) {
		return fs.ErrorDirNotFound
	}
	if prefix != "" {
		prefix += "/"
	}
	if directory != "" {
		directory += "/"
	}
	delimiter := ""
	if !recurse {
		delimiter = "/"
	}

	options := azblob.ListBlobsSegmentOptions{
		Details: azblob.BlobListingDetails{
			Copy:             false,
			Metadata:         true,
			Snapshots:        false,
			UncommittedBlobs: false,
			Deleted:          false,
		},
		Prefix:     directory,
		MaxResults: int32(maxResults),
	}
	for marker := (azblob.Marker{}); marker.NotDone(); {
		var response *azblob.ListBlobsHierarchySegmentResponse
		err := f.pacer.Call(func() (bool, error) {
			var err error
			response, err = f.cntURL(container).ListBlobsHierarchySegment(ctx, marker, delimiter, options)
			return f.shouldRetry(ctx, err)
		})

		if err != nil {
			// Check http error code along with service code, current SDK doesn't populate service code correctly sometimes
			if storageErr, ok := err.(azblob.StorageError); ok && (storageErr.ServiceCode() == azblob.ServiceCodeContainerNotFound || storageErr.Response().StatusCode == http.StatusNotFound) {
				return fs.ErrorDirNotFound
			}
			return err
		}
		// Advance marker to next
		marker = response.NextMarker
		for i := range response.Segment.BlobItems {
			file := &response.Segment.BlobItems[i]
			// Finish if file name no longer has prefix
			// if prefix != "" && !strings.HasPrefix(file.Name, prefix) {
			// 	return nil
			// }
			remote := f.opt.Enc.ToStandardPath(file.Name)
			if !strings.HasPrefix(remote, prefix) {
				fs.Debugf(f, "Odd name received %q", remote)
				continue
			}
			remote = remote[len(prefix):]
			if isDirectoryMarker(*file.Properties.ContentLength, file.Metadata, remote) {
				continue // skip directory marker
			}
			if addContainer {
				remote = path.Join(container, remote)
			}
			// Send object
			err = fn(remote, file, false)
			if err != nil {
				return err
			}
		}
		// Send the subdirectories
		for _, remote := range response.Segment.BlobPrefixes {
			remote := strings.TrimRight(remote.Name, "/")
			remote = f.opt.Enc.ToStandardPath(remote)
			if !strings.HasPrefix(remote, prefix) {
				fs.Debugf(f, "Odd directory name received %q", remote)
				continue
			}
			remote = remote[len(prefix):]
			if addContainer {
				remote = path.Join(container, remote)
			}
			// Send object
			err = fn(remote, nil, true)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(remote string, object *azblob.BlobItemInternal, isDirectory bool) (fs.DirEntry, error) {
	if isDirectory {
		d := fs.NewDir(remote, time.Time{})
		return d, nil
	}
	o, err := f.newObjectWithInfo(remote, object)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Check to see if this is a limited container and the container is not found
func (f *Fs) containerOK(container string) bool {
	if !f.isLimited {
		return true
	}
	f.cntURLcacheMu.Lock()
	defer f.cntURLcacheMu.Unlock()
	for limitedContainer := range f.cntURLcache {
		if container == limitedContainer {
			return true
		}
	}
	return false
}

// listDir lists a single directory
func (f *Fs) listDir(ctx context.Context, container, directory, prefix string, addContainer bool) (entries fs.DirEntries, err error) {
	if !f.containerOK(container) {
		return nil, fs.ErrorDirNotFound
	}
	err = f.list(ctx, container, directory, prefix, addContainer, false, f.opt.ListChunkSize, func(remote string, object *azblob.BlobItemInternal, isDirectory bool) error {
		entry, err := f.itemToDirEntry(remote, object, isDirectory)
		if err != nil {
			return err
		}
		if entry != nil {
			entries = append(entries, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// container must be present if listing succeeded
	f.cache.MarkOK(container)
	return entries, nil
}

// listContainers returns all the containers to out
func (f *Fs) listContainers(ctx context.Context) (entries fs.DirEntries, err error) {
	if f.isLimited {
		f.cntURLcacheMu.Lock()
		for container := range f.cntURLcache {
			d := fs.NewDir(container, time.Time{})
			entries = append(entries, d)
		}
		f.cntURLcacheMu.Unlock()
		return entries, nil
	}
	err = f.listContainersToFn(func(container *azblob.ContainerItem) error {
		d := fs.NewDir(f.opt.Enc.ToStandardName(container.Name), container.Properties.LastModified)
		f.cache.MarkOK(container.Name)
		entries = append(entries, d)
		return nil
	})
	if err != nil {
		return nil, err
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
// of listing recursively that doing a directory traversal.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	container, directory := f.split(dir)
	list := walk.NewListRHelper(callback)
	listR := func(container, directory, prefix string, addContainer bool) error {
		return f.list(ctx, container, directory, prefix, addContainer, true, f.opt.ListChunkSize, func(remote string, object *azblob.BlobItemInternal, isDirectory bool) error {
			entry, err := f.itemToDirEntry(remote, object, isDirectory)
			if err != nil {
				return err
			}
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
		if !f.containerOK(container) {
			return fs.ErrorDirNotFound
		}
		err = listR(container, directory, f.rootDirectory, f.rootContainer == "")
		if err != nil {
			return err
		}
		// container must be present if listing succeeded
		f.cache.MarkOK(container)
	}
	return list.Flush()
}

// listContainerFn is called from listContainersToFn to handle a container
type listContainerFn func(*azblob.ContainerItem) error

// listContainersToFn lists the containers to the function supplied
func (f *Fs) listContainersToFn(fn listContainerFn) error {
	params := azblob.ListContainersSegmentOptions{
		MaxResults: int32(f.opt.ListChunkSize),
	}
	ctx := context.Background()
	for marker := (azblob.Marker{}); marker.NotDone(); {
		var response *azblob.ListContainersSegmentResponse
		err := f.pacer.Call(func() (bool, error) {
			var err error
			response, err = f.svcURL.ListContainersSegment(ctx, marker, params)
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			return err
		}

		for i := range response.ContainerItems {
			err = fn(&response.ContainerItems[i])
			if err != nil {
				return err
			}
		}
		marker = response.NextMarker
	}

	return nil
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Temporary Object under construction
	fs := &Object{
		fs:     f,
		remote: src.Remote(),
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
		// If this is a SAS URL limited to a container then assume it is already created
		if f.isLimited {
			return nil
		}
		// now try to create the container
		return f.pacer.Call(func() (bool, error) {
			_, err := f.cntURL(container).Create(ctx, azblob.Metadata{}, f.publicAccess)
			if err != nil {
				if storageErr, ok := err.(azblob.StorageError); ok {
					switch storageErr.ServiceCode() {
					case azblob.ServiceCodeContainerAlreadyExists:
						return false, nil
					case azblob.ServiceCodeContainerBeingDeleted:
						// From https://docs.microsoft.com/en-us/rest/api/storageservices/delete-container
						// When a container is deleted, a container with the same name cannot be created
						// for at least 30 seconds; the container may not be available for more than 30
						// seconds if the service is still processing the request.
						time.Sleep(6 * time.Second) // default 10 retries will be 60 seconds
						f.cache.MarkDeleted(container)
						return true, err
					}
				}
			}
			return f.shouldRetry(ctx, err)
		})
	}, nil)
}

// isEmpty checks to see if a given (container, directory) is empty and returns an error if not
func (f *Fs) isEmpty(ctx context.Context, container, directory string) (err error) {
	empty := true
	err = f.list(ctx, container, directory, f.rootDirectory, f.rootContainer == "", true, 1, func(remote string, object *azblob.BlobItemInternal, isDirectory bool) error {
		empty = false
		return nil
	})
	if err != nil {
		return err
	}
	if !empty {
		return fs.ErrorDirectoryNotEmpty
	}
	return nil
}

// deleteContainer deletes the container.  It can delete a full
// container so use isEmpty if you don't want that.
func (f *Fs) deleteContainer(ctx context.Context, container string) error {
	return f.cache.Remove(container, func() error {
		options := azblob.ContainerAccessConditions{}
		return f.pacer.Call(func() (bool, error) {
			_, err := f.cntURL(container).GetProperties(ctx, azblob.LeaseAccessConditions{})
			if err == nil {
				_, err = f.cntURL(container).Delete(ctx, options)
			}

			if err != nil {
				// Check http error code along with service code, current SDK doesn't populate service code correctly sometimes
				if storageErr, ok := err.(azblob.StorageError); ok && (storageErr.ServiceCode() == azblob.ServiceCodeContainerNotFound || storageErr.Response().StatusCode == http.StatusNotFound) {
					return false, fs.ErrorDirNotFound
				}

				return f.shouldRetry(ctx, err)
			}

			return f.shouldRetry(ctx, err)
		})
	})
}

// Rmdir deletes the container if the fs is at the root
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	container, directory := f.split(dir)
	if container == "" || directory != "" {
		return nil
	}
	err := f.isEmpty(ctx, container, directory)
	if err != nil {
		return err
	}
	return f.deleteContainer(ctx, container)
}

// Precision of the remote
func (f *Fs) Precision() time.Duration {
	return time.Nanosecond
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// Purge deletes all the files and directories including the old versions.
func (f *Fs) Purge(ctx context.Context, dir string) error {
	container, directory := f.split(dir)
	if container == "" || directory != "" {
		// Delegate to caller if not root of a container
		return fs.ErrorCantPurge
	}
	return f.deleteContainer(ctx, container)
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
	dstBlobURL := f.getBlobReference(dstContainer, dstPath)
	srcBlobURL := srcObj.getBlobReference()

	source, err := url.Parse(srcBlobURL.String())
	if err != nil {
		return nil, err
	}

	options := azblob.BlobAccessConditions{}
	var startCopy *azblob.BlobStartCopyFromURLResponse

	err = f.pacer.Call(func() (bool, error) {
		startCopy, err = dstBlobURL.StartCopyFromURL(ctx, *source, nil, azblob.ModifiedAccessConditions{}, options, azblob.AccessTierType(f.opt.AccessTier), nil)
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}

	copyStatus := startCopy.CopyStatus()
	for copyStatus == azblob.CopyStatusPending {
		time.Sleep(1 * time.Second)
		getMetadata, err := dstBlobURL.GetProperties(ctx, options, azblob.ClientProvidedKeyOptions{})
		if err != nil {
			return nil, err
		}
		copyStatus = getMetadata.CopyStatus()
	}

	return f.NewObject(ctx, remote)
}

func (f *Fs) getMemoryPool(size int64) *pool.Pool {
	if size == int64(f.opt.ChunkSize) {
		return f.pool
	}

	return pool.New(
		time.Duration(f.opt.MemoryPoolFlushTime),
		int(size),
		f.ci.Transfers,
		f.opt.MemoryPoolUseMmap,
	)
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

// Hash returns the MD5 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	// Convert base64 encoded md5 into lower case hex
	if o.md5 == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(o.md5)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to decode Content-MD5: %q", o.md5)
	}
	return hex.EncodeToString(data), nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

func (o *Object) setMetadata(metadata azblob.Metadata) {
	if len(metadata) > 0 {
		o.meta = metadata
		if modTime, ok := metadata[modTimeKey]; ok {
			when, err := time.Parse(timeFormatIn, modTime)
			if err != nil {
				fs.Debugf(o, "Couldn't parse %v = %q: %v", modTimeKey, modTime, err)
			}
			o.modTime = when
		}
	} else {
		o.meta = nil
	}
}

// decodeMetaDataFromPropertiesResponse sets the metadata from the data passed in
//
// Sets
//  o.id
//  o.modTime
//  o.size
//  o.md5
//  o.meta
func (o *Object) decodeMetaDataFromPropertiesResponse(info *azblob.BlobGetPropertiesResponse) (err error) {
	metadata := info.NewMetadata()
	size := info.ContentLength()
	if isDirectoryMarker(size, metadata, o.remote) {
		return fs.ErrorNotAFile
	}
	// NOTE - Client library always returns MD5 as base64 decoded string, Object needs to maintain
	// this as base64 encoded string.
	o.md5 = base64.StdEncoding.EncodeToString(info.ContentMD5())
	o.mimeType = info.ContentType()
	o.size = size
	o.modTime = info.LastModified()
	o.accessTier = azblob.AccessTierType(info.AccessTier())
	o.setMetadata(metadata)

	return nil
}

func (o *Object) decodeMetaDataFromBlob(info *azblob.BlobItemInternal) (err error) {
	metadata := info.Metadata
	size := *info.Properties.ContentLength
	if isDirectoryMarker(size, metadata, o.remote) {
		return fs.ErrorNotAFile
	}
	// NOTE - Client library always returns MD5 as base64 decoded string, Object needs to maintain
	// this as base64 encoded string.
	o.md5 = base64.StdEncoding.EncodeToString(info.Properties.ContentMD5)
	o.mimeType = *info.Properties.ContentType
	o.size = size
	o.modTime = info.Properties.LastModified
	o.accessTier = info.Properties.AccessTier
	o.setMetadata(metadata)
	return nil
}

// getBlobReference creates an empty blob reference with no metadata
func (o *Object) getBlobReference() azblob.BlobURL {
	container, directory := o.split()
	return o.fs.getBlobReference(container, directory)
}

// clearMetaData clears enough metadata so readMetaData will re-read it
func (o *Object) clearMetaData() {
	o.modTime = time.Time{}
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// Sets
//  o.id
//  o.modTime
//  o.size
//  o.md5
func (o *Object) readMetaData() (err error) {
	if !o.modTime.IsZero() {
		return nil
	}
	blob := o.getBlobReference()

	// Read metadata (this includes metadata)
	options := azblob.BlobAccessConditions{}
	ctx := context.Background()
	var blobProperties *azblob.BlobGetPropertiesResponse
	err = o.fs.pacer.Call(func() (bool, error) {
		blobProperties, err = blob.GetProperties(ctx, options, azblob.ClientProvidedKeyOptions{})
		return o.fs.shouldRetry(ctx, err)
	})
	if err != nil {
		// On directories - GetProperties does not work and current SDK does not populate service code correctly hence check regular http response as well
		if storageErr, ok := err.(azblob.StorageError); ok && (storageErr.ServiceCode() == azblob.ServiceCodeBlobNotFound || storageErr.Response().StatusCode == http.StatusNotFound) {
			return fs.ErrorObjectNotFound
		}
		return err
	}

	return o.decodeMetaDataFromPropertiesResponse(blobProperties)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) (result time.Time) {
	// The error is logged in readMetaData
	_ = o.readMetaData()
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	// Make sure o.meta is not nil
	if o.meta == nil {
		o.meta = make(map[string]string, 1)
	}
	// Set modTimeKey in it
	o.meta[modTimeKey] = modTime.Format(timeFormatOut)

	blob := o.getBlobReference()
	err := o.fs.pacer.Call(func() (bool, error) {
		_, err := blob.SetMetadata(ctx, o.meta, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		return o.fs.shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}
	o.modTime = modTime
	return nil
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	// Offset and Count for range download
	var offset int64
	var count int64
	if o.AccessTier() == azblob.AccessTierArchive {
		return nil, errors.Errorf("Blob in archive tier, you need to set tier to hot or cool first")
	}
	fs.FixRangeOption(options, o.size)
	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			offset, count = x.Decode(o.size)
			if count < 0 {
				count = o.size - offset
			}
		case *fs.SeekOption:
			offset = x.Offset
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}
	blob := o.getBlobReference()
	ac := azblob.BlobAccessConditions{}
	var downloadResponse *azblob.DownloadResponse
	err = o.fs.pacer.Call(func() (bool, error) {
		downloadResponse, err = blob.Download(ctx, offset, count, ac, false, azblob.ClientProvidedKeyOptions{})
		return o.fs.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to open for download")
	}
	in = downloadResponse.Body(azblob.RetryReaderOptions{})
	return in, nil
}

// dontEncode is the characters that do not need percent-encoding
//
// The characters that do not need percent-encoding are a subset of
// the printable ASCII characters: upper-case letters, lower-case
// letters, digits, ".", "_", "-", "/", "~", "!", "$", "'", "(", ")",
// "*", ";", "=", ":", and "@". All other byte values in a UTF-8 must
// be replaced with "%" and the two-digit hex value of the byte.
const dontEncode = (`abcdefghijklmnopqrstuvwxyz` +
	`ABCDEFGHIJKLMNOPQRSTUVWXYZ` +
	`0123456789` +
	`._-/~!$'()*;=:@`)

// noNeedToEncode is a bitmap of characters which don't need % encoding
var noNeedToEncode [256]bool

func init() {
	for _, c := range dontEncode {
		noNeedToEncode[c] = true
	}
}

// increment the slice passed in as LSB binary
func increment(xs []byte) {
	for i, digit := range xs {
		newDigit := digit + 1
		xs[i] = newDigit
		if newDigit >= digit {
			// exit if no carry
			break
		}
	}
}

// poolWrapper wraps a pool.Pool as an azblob.TransferManager
type poolWrapper struct {
	pool     *pool.Pool
	bufToken chan struct{}
	runToken chan struct{}
}

// newPoolWrapper creates an azblob.TransferManager that will use a
// pool.Pool with maximum concurrency as specified.
func (f *Fs) newPoolWrapper(concurrency int) azblob.TransferManager {
	return &poolWrapper{
		pool:     f.pool,
		bufToken: make(chan struct{}, concurrency),
		runToken: make(chan struct{}, concurrency),
	}
}

// Get implements TransferManager.Get().
func (pw *poolWrapper) Get() []byte {
	pw.bufToken <- struct{}{}
	return pw.pool.Get()
}

// Put implements TransferManager.Put().
func (pw *poolWrapper) Put(b []byte) {
	pw.pool.Put(b)
	<-pw.bufToken
}

// Run implements TransferManager.Run().
func (pw *poolWrapper) Run(f func()) {
	pw.runToken <- struct{}{}
	go func() {
		f()
		<-pw.runToken
	}()
}

// Close implements TransferManager.Close().
func (pw *poolWrapper) Close() {
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	if o.accessTier == azblob.AccessTierArchive {
		if o.fs.opt.ArchiveTierDelete {
			fs.Debugf(o, "deleting archive tier blob before updating")
			err = o.Remove(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to delete archive blob before updating")
			}
		} else {
			return errCantUpdateArchiveTierBlobs
		}
	}
	container, _ := o.split()
	err = o.fs.makeContainer(ctx, container)
	if err != nil {
		return err
	}

	// Update Mod time
	o.updateMetadataWithModTime(src.ModTime(ctx))
	if err != nil {
		return err
	}

	blob := o.getBlobReference()
	httpHeaders := azblob.BlobHTTPHeaders{}
	httpHeaders.ContentType = fs.MimeType(ctx, src)

	// Compute the Content-MD5 of the file. As we stream all uploads it
	// will be set in PutBlockList API call using the 'x-ms-blob-content-md5' header
	if !o.fs.opt.DisableCheckSum {
		if sourceMD5, _ := src.Hash(ctx, hash.MD5); sourceMD5 != "" {
			sourceMD5bytes, err := hex.DecodeString(sourceMD5)
			if err == nil {
				httpHeaders.ContentMD5 = sourceMD5bytes
			} else {
				fs.Debugf(o, "Failed to decode %q as MD5: %v", sourceMD5, err)
			}
		}
	}

	putBlobOptions := azblob.UploadStreamToBlockBlobOptions{
		BufferSize:      int(o.fs.opt.ChunkSize),
		MaxBuffers:      uploadConcurrency,
		Metadata:        o.meta,
		BlobHTTPHeaders: httpHeaders,
		TransferManager: o.fs.newPoolWrapper(uploadConcurrency),
	}

	// Don't retry, return a retry error instead
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		// Stream contents of the reader object to the given blob URL
		blockBlobURL := blob.ToBlockBlobURL()
		_, err = azblob.UploadStreamToBlockBlob(ctx, in, blockBlobURL, putBlobOptions)
		return o.fs.shouldRetry(ctx, err)
	})
	if err != nil {
		return err
	}
	// Refresh metadata on object
	o.clearMetaData()
	err = o.readMetaData()
	if err != nil {
		return err
	}

	// If tier is not changed or not specified, do not attempt to invoke `SetBlobTier` operation
	if o.fs.opt.AccessTier == string(defaultAccessTier) || o.fs.opt.AccessTier == string(o.AccessTier()) {
		return nil
	}

	// Now, set blob tier based on configured access tier
	return o.SetTier(o.fs.opt.AccessTier)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	blob := o.getBlobReference()
	snapShotOptions := azblob.DeleteSnapshotsOptionNone
	ac := azblob.BlobAccessConditions{}
	return o.fs.pacer.Call(func() (bool, error) {
		_, err := blob.Delete(ctx, snapShotOptions, ac)
		return o.fs.shouldRetry(ctx, err)
	})
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// AccessTier of an object, default is of type none
func (o *Object) AccessTier() azblob.AccessTierType {
	return o.accessTier
}

// SetTier performs changing object tier
func (o *Object) SetTier(tier string) error {
	if !validateAccessTier(tier) {
		return errors.Errorf("Tier %s not supported by Azure Blob Storage", tier)
	}

	// Check if current tier already matches with desired tier
	if o.GetTier() == tier {
		return nil
	}
	desiredAccessTier := azblob.AccessTierType(tier)
	blob := o.getBlobReference()
	ctx := context.Background()
	err := o.fs.pacer.Call(func() (bool, error) {
		_, err := blob.SetTier(ctx, desiredAccessTier, azblob.LeaseAccessConditions{})
		return o.fs.shouldRetry(ctx, err)
	})

	if err != nil {
		return errors.Wrap(err, "Failed to set Blob Tier")
	}

	// Set access tier on local object also, this typically
	// gets updated on get blob properties
	o.accessTier = desiredAccessTier
	fs.Debugf(o, "Successfully changed object tier to %s", tier)

	return nil
}

// GetTier returns object tier in azure as string
func (o *Object) GetTier() string {
	return string(o.accessTier)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = &Fs{}
	_ fs.Copier      = &Fs{}
	_ fs.PutStreamer = &Fs{}
	_ fs.Purger      = &Fs{}
	_ fs.ListRer     = &Fs{}
	_ fs.Object      = &Object{}
	_ fs.MimeTyper   = &Object{}
	_ fs.GetTierer   = &Object{}
	_ fs.SetTierer   = &Object{}
)
