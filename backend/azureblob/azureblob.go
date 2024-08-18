//go:build !plan9 && !solaris && !js

// Package azureblob provides an interface to the Microsoft Azure blob object storage system
package azureblob

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/chunksize"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/bucket"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/env"
	"github.com/rclone/rclone/lib/multipart"
	"github.com/rclone/rclone/lib/pacer"
)

const (
	minSleep              = 10 * time.Millisecond
	maxSleep              = 10 * time.Second
	decayConstant         = 1    // bigger for slower decay, exponential
	maxListChunkSize      = 5000 // number of items to read at once
	modTimeKey            = "mtime"
	dirMetaKey            = "hdi_isfolder"
	dirMetaValue          = "true"
	timeFormatIn          = time.RFC3339
	timeFormatOut         = "2006-01-02T15:04:05.000000000Z07:00"
	storageDefaultBaseURL = "blob.core.windows.net"
	defaultChunkSize      = 4 * fs.Mebi
	defaultAccessTier     = blob.AccessTier("") // FIXME AccessTierNone
	// Default storage account, key and blob endpoint for emulator support,
	// though it is a base64 key checked in here, it is publicly available secret.
	emulatorAccount      = "devstoreaccount1"
	emulatorAccountKey   = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
	emulatorBlobEndpoint = "http://127.0.0.1:10000/devstoreaccount1"
)

var (
	errCantUpdateArchiveTierBlobs = fserrors.NoRetryError(errors.New("can't update archive tier blob without --azureblob-archive-tier-delete"))

	// Take this when changing or reading metadata.
	//
	// It acts as global metadata lock so we don't bloat Object
	// with an extra lock that will only very rarely be contended.
	metadataMu sync.Mutex
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "azureblob",
		Description: "Microsoft Azure Blob Storage",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "account",
			Help: `Azure Storage Account Name.

Set this to the Azure Storage Account Name in use.

Leave blank to use SAS URL or Emulator, otherwise it needs to be set.

If this is blank and if env_auth is set it will be read from the
environment variable ` + "`AZURE_STORAGE_ACCOUNT_NAME`" + ` if possible.
`,
			Sensitive: true,
		}, {
			Name: "env_auth",
			Help: `Read credentials from runtime (environment variables, CLI or MSI).

See the [authentication docs](/azureblob#authentication) for full info.`,
			Default: false,
		}, {
			Name: "key",
			Help: `Storage Account Shared Key.

Leave blank to use SAS URL or Emulator.`,
			Sensitive: true,
		}, {
			Name: "sas_url",
			Help: `SAS URL for container level access only.

Leave blank if using account/key or Emulator.`,
			Sensitive: true,
		}, {
			Name: "tenant",
			Help: `ID of the service principal's tenant. Also called its directory ID.

Set this if using
- Service principal with client secret
- Service principal with certificate
- User with username and password
`,
			Sensitive: true,
		}, {
			Name: "client_id",
			Help: `The ID of the client in use.

Set this if using
- Service principal with client secret
- Service principal with certificate
- User with username and password
`,
			Sensitive: true,
		}, {
			Name: "client_secret",
			Help: `One of the service principal's client secrets

Set this if using
- Service principal with client secret
`,
			Sensitive: true,
		}, {
			Name: "client_certificate_path",
			Help: `Path to a PEM or PKCS12 certificate file including the private key.

Set this if using
- Service principal with certificate
`,
		}, {
			Name: "client_certificate_password",
			Help: `Password for the certificate file (optional).

Optionally set this if using
- Service principal with certificate

And the certificate has a password.
`,
			IsPassword: true,
		}, {
			Name: "client_send_certificate_chain",
			Help: `Send the certificate chain when using certificate auth.

Specifies whether an authentication request will include an x5c header
to support subject name / issuer based authentication. When set to
true, authentication requests include the x5c header.

Optionally set this if using
- Service principal with certificate
`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "username",
			Help: `User name (usually an email address)

Set this if using
- User with username and password
`,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name: "password",
			Help: `The user's password

Set this if using
- User with username and password
`,
			IsPassword: true,
			Advanced:   true,
		}, {
			Name: "service_principal_file",
			Help: `Path to file containing credentials for use with a service principal.

Leave blank normally. Needed only if you want to use a service principal instead of interactive login.

    $ az ad sp create-for-rbac --name "<name>" \
      --role "Storage Blob Data Owner" \
      --scopes "/subscriptions/<subscription>/resourceGroups/<resource-group>/providers/Microsoft.Storage/storageAccounts/<storage-account>/blobServices/default/containers/<container>" \
      > azure-principal.json

See ["Create an Azure service principal"](https://docs.microsoft.com/en-us/cli/azure/create-an-azure-service-principal-azure-cli) and ["Assign an Azure role for access to blob data"](https://docs.microsoft.com/en-us/azure/storage/common/storage-auth-aad-rbac-cli) pages for more details.

It may be more convenient to put the credentials directly into the
rclone config file under the ` + "`client_id`, `tenant` and `client_secret`" + `
keys instead of setting ` + "`service_principal_file`" + `.
`,
			Advanced: true,
		}, {
			Name: "use_msi",
			Help: `Use a managed service identity to authenticate (only works in Azure).

When true, use a [managed service identity](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/)
to authenticate to Azure Storage instead of a SAS token or account key.

If the VM(SS) on which this program is running has a system-assigned identity, it will
be used by default. If the resource has no system-assigned but exactly one user-assigned identity,
the user-assigned identity will be used by default. If the resource has multiple user-assigned
identities, the identity to use must be explicitly specified using exactly one of the msi_object_id,
msi_client_id, or msi_mi_res_id parameters.`,
			Default:  false,
			Advanced: true,
		}, {
			Name:      "msi_object_id",
			Help:      "Object ID of the user-assigned MSI to use, if any.\n\nLeave blank if msi_client_id or msi_mi_res_id specified.",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:      "msi_client_id",
			Help:      "Object ID of the user-assigned MSI to use, if any.\n\nLeave blank if msi_object_id or msi_mi_res_id specified.",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:      "msi_mi_res_id",
			Help:      "Azure resource ID of the user-assigned MSI to use, if any.\n\nLeave blank if msi_client_id or msi_object_id specified.",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:     "use_emulator",
			Help:     "Uses local storage emulator if provided as 'true'.\n\nLeave blank if using real azure storage endpoint.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "endpoint",
			Help:     "Endpoint for the service.\n\nLeave blank normally.",
			Advanced: true,
		}, {
			Name:     "upload_cutoff",
			Help:     "Cutoff for switching to chunked upload (<= 256 MiB) (deprecated).",
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: `Upload chunk size.

Note that this is stored in memory and there may be up to
"--transfers" * "--azureblob-upload-concurrency" chunks stored at once
in memory.`,
			Default:  defaultChunkSize,
			Advanced: true,
		}, {
			Name: "upload_concurrency",
			Help: `Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently.

If you are uploading small numbers of large files over high-speed
links and these uploads do not fully utilize your bandwidth, then
increasing this may help to speed up the transfers.

In tests, upload speed increases almost linearly with upload
concurrency. For example to fill a gigabit pipe it may be necessary to
raise this to 64. Note that this will use more memory.

Note that chunks are stored in memory and there may be up to
"--transfers" * "--azureblob-upload-concurrency" chunks stored at once
in memory.`,
			Default:  16,
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
			Help: `Access tier of blob: hot, cool, cold or archive.

Archived blobs can be restored by setting access tier to hot, cool or
cold. Leave blank if you intend to use default access tier, which is
set at account level

If there is no "access tier" specified, rclone doesn't apply any tier.
rclone performs "Set Tier" operation on blobs while uploading, if objects
are not modified, specifying "access tier" to new one will have no effect.
If blobs are in "archive tier" at remote, trying to perform data transfer
operations from remote will not be allowed. User should first restore by
tiering blob to "Hot", "Cool" or "Cold".`,
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
			Default:  fs.Duration(time.Minute),
			Advanced: true,
			Hide:     fs.OptionHideBoth,
			Help:     `How often internal memory buffer pools will be flushed. (no longer used)`,
		}, {
			Name:     "memory_pool_use_mmap",
			Default:  false,
			Advanced: true,
			Hide:     fs.OptionHideBoth,
			Help:     `Whether to use mmap buffers in internal memory pool. (no longer used)`,
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
			Help:    "Public access level of a container: blob or container.",
			Default: "",
			Examples: []fs.OptionExample{
				{
					Value: "",
					Help:  "The container and its blobs can be accessed only with an authorized request.\nIt's a default value.",
				}, {
					Value: string(container.PublicAccessTypeBlob),
					Help:  "Blob data within this container can be read via anonymous request.",
				}, {
					Value: string(container.PublicAccessTypeContainer),
					Help:  "Allow full public read access for container and blob data.",
				},
			},
			Advanced: true,
		}, {
			Name:     "directory_markers",
			Default:  false,
			Advanced: true,
			Help: `Upload an empty object with a trailing slash when a new directory is created

Empty folders are unsupported for bucket based remotes, this option
creates an empty object ending with "/", to persist the folder.

This object also has the metadata "` + dirMetaKey + ` = ` + dirMetaValue + `" to conform to
the Microsoft standard.
 `,
		}, {
			Name: "no_check_container",
			Help: `If set, don't attempt to check the container exists or create it.

This can be useful when trying to minimise the number of transactions
rclone does if you know the container exists already.
`,
			Default:  false,
			Advanced: true,
		}, {
			Name:     "no_head_object",
			Help:     `If set, do not do HEAD before GET when getting objects.`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "delete_snapshots",
			Help: `Set to specify how to deal with snapshots on blob deletion.`,
			Examples: []fs.OptionExample{
				{
					Value: "",
					Help:  "By default, the delete operation fails if a blob has snapshots",
				}, {
					Value: string(blob.DeleteSnapshotsOptionTypeInclude),
					Help:  "Specify 'include' to remove the root blob and all its snapshots",
				}, {
					Value: string(blob.DeleteSnapshotsOptionTypeOnly),
					Help:  "Specify 'only' to remove only the snapshots but keep the root blob.",
				},
			},
			Default:   "",
			Exclusive: true,
			Advanced:  true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Account                    string               `config:"account"`
	EnvAuth                    bool                 `config:"env_auth"`
	Key                        string               `config:"key"`
	SASURL                     string               `config:"sas_url"`
	Tenant                     string               `config:"tenant"`
	ClientID                   string               `config:"client_id"`
	ClientSecret               string               `config:"client_secret"`
	ClientCertificatePath      string               `config:"client_certificate_path"`
	ClientCertificatePassword  string               `config:"client_certificate_password"`
	ClientSendCertificateChain bool                 `config:"client_send_certificate_chain"`
	Username                   string               `config:"username"`
	Password                   string               `config:"password"`
	ServicePrincipalFile       string               `config:"service_principal_file"`
	UseMSI                     bool                 `config:"use_msi"`
	MSIObjectID                string               `config:"msi_object_id"`
	MSIClientID                string               `config:"msi_client_id"`
	MSIResourceID              string               `config:"msi_mi_res_id"`
	Endpoint                   string               `config:"endpoint"`
	ChunkSize                  fs.SizeSuffix        `config:"chunk_size"`
	UploadConcurrency          int                  `config:"upload_concurrency"`
	ListChunkSize              uint                 `config:"list_chunk"`
	AccessTier                 string               `config:"access_tier"`
	ArchiveTierDelete          bool                 `config:"archive_tier_delete"`
	UseEmulator                bool                 `config:"use_emulator"`
	DisableCheckSum            bool                 `config:"disable_checksum"`
	Enc                        encoder.MultiEncoder `config:"encoding"`
	PublicAccess               string               `config:"public_access"`
	DirectoryMarkers           bool                 `config:"directory_markers"`
	NoCheckContainer           bool                 `config:"no_check_container"`
	NoHeadObject               bool                 `config:"no_head_object"`
	DeleteSnapshots            string               `config:"delete_snapshots"`
}

// Fs represents a remote azure server
type Fs struct {
	name          string                       // name of this remote
	root          string                       // the path we are working on if any
	opt           Options                      // parsed config options
	ci            *fs.ConfigInfo               // global config
	features      *fs.Features                 // optional features
	cntSVCcacheMu sync.Mutex                   // mutex to protect cntSVCcache
	cntSVCcache   map[string]*container.Client // reference to containerClient per container
	svc           *service.Client              // client to access azblob
	rootContainer string                       // container part of root (if any)
	rootDirectory string                       // directory part of root (if any)
	isLimited     bool                         // if limited to one container
	cache         *bucket.Cache                // cache for container creation status
	pacer         *fs.Pacer                    // To pace and retry the API calls
	uploadToken   *pacer.TokenDispenser        // control concurrency
	publicAccess  container.PublicAccessType   // Container Public Access Level
}

// Object describes an azure object
type Object struct {
	fs         *Fs               // what this object is part of
	remote     string            // The remote path
	modTime    time.Time         // The modified time of the object if known
	md5        string            // MD5 hash if known
	size       int64             // Size of the object
	mimeType   string            // Content-Type of the object
	accessTier blob.AccessTier   // Blob Access Tier
	meta       map[string]string // blob metadata - take metadataMu when accessing
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
	containerName, containerPath = bucket.Split(bucket.Join(f.root, rootRelativePath))
	return f.opt.Enc.FromStandardName(containerName), f.opt.Enc.FromStandardPath(containerPath)
}

// split returns container and containerPath from the object
func (o *Object) split() (container, containerPath string) {
	return o.fs.split(o.remote)
}

// validateAccessTier checks if azureblob supports user supplied tier
func validateAccessTier(tier string) bool {
	return strings.EqualFold(tier, string(blob.AccessTierHot)) ||
		strings.EqualFold(tier, string(blob.AccessTierCool)) ||
		strings.EqualFold(tier, string(blob.AccessTierCold)) ||
		strings.EqualFold(tier, string(blob.AccessTierArchive))
}

// validatePublicAccess checks if azureblob supports use supplied public access level
func validatePublicAccess(publicAccess string) bool {
	switch publicAccess {
	case "",
		string(container.PublicAccessTypeBlob),
		string(container.PublicAccessTypeContainer):
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
	if storageErr, ok := err.(*azcore.ResponseError); ok {
		switch storageErr.ErrorCode {
		case "InvalidBlobOrBlock":
			// These errors happen sometimes in multipart uploads
			// because of block concurrency issues
			return true, err
		}
		statusCode := storageErr.StatusCode
		for _, e := range retryErrorCodes {
			if statusCode == e {
				return true, err
			}
		}
	}
	return fserrors.ShouldRetry(err), err
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

type servicePrincipalCredentials struct {
	AppID    string `json:"appId"`
	Password string `json:"password"`
	Tenant   string `json:"tenant"`
}

// parseServicePrincipalCredentials unmarshals a service principal credentials JSON file as generated by az cli.
func parseServicePrincipalCredentials(ctx context.Context, credentialsData []byte) (*servicePrincipalCredentials, error) {
	var spCredentials servicePrincipalCredentials
	if err := json.Unmarshal(credentialsData, &spCredentials); err != nil {
		return nil, fmt.Errorf("error parsing credentials from JSON file: %w", err)
	}
	// TODO: support certificate credentials
	// Validate all fields present
	if spCredentials.AppID == "" || spCredentials.Password == "" || spCredentials.Tenant == "" {
		return nil, fmt.Errorf("missing fields in credentials file")
	}
	return &spCredentials, nil
}

// setRoot changes the root of the Fs
func (f *Fs) setRoot(root string) {
	f.root = parsePath(root)
	f.rootContainer, f.rootDirectory = bucket.Split(f.root)
}

// Wrap the http.Transport to satisfy the Transporter interface
type transporter struct {
	http.RoundTripper
}

// Make a new transporter
func newTransporter(ctx context.Context) transporter {
	return transporter{
		RoundTripper: fshttp.NewTransport(ctx),
	}
}

// Do sends the HTTP request and returns the HTTP response or error.
func (tr transporter) Do(req *http.Request) (*http.Response, error) {
	return tr.RoundTripper.RoundTrip(req)
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
		return nil, fmt.Errorf("chunk size: %w", err)
	}
	if opt.ListChunkSize > maxListChunkSize {
		return nil, fmt.Errorf("blob list size can't be greater than %v - was %v", maxListChunkSize, opt.ListChunkSize)
	}

	if opt.AccessTier == "" {
		opt.AccessTier = string(defaultAccessTier)
	} else if !validateAccessTier(opt.AccessTier) {
		return nil, fmt.Errorf("supported access tiers are %s, %s, %s and %s",
			string(blob.AccessTierHot), string(blob.AccessTierCool), string(blob.AccessTierCold), string(blob.AccessTierArchive))
	}

	if !validatePublicAccess((opt.PublicAccess)) {
		return nil, fmt.Errorf("supported public access level are %s and %s",
			string(container.PublicAccessTypeBlob), string(container.PublicAccessTypeContainer))
	}

	ci := fs.GetConfig(ctx)
	f := &Fs{
		name:        name,
		opt:         *opt,
		ci:          ci,
		pacer:       fs.NewPacer(ctx, pacer.NewS3(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		uploadToken: pacer.NewTokenDispenser(ci.Transfers),
		cache:       bucket.NewCache(),
		cntSVCcache: make(map[string]*container.Client, 1),
	}
	f.publicAccess = container.PublicAccessType(opt.PublicAccess)
	f.setRoot(root)
	f.features = (&fs.Features{
		ReadMimeType:      true,
		WriteMimeType:     true,
		BucketBased:       true,
		BucketBasedRootOK: true,
		SetTier:           true,
		GetTier:           true,
	}).Fill(ctx, f)
	if opt.DirectoryMarkers {
		f.features.CanHaveEmptyDirectories = true
		fs.Debugf(f, "Using directory markers")
	}

	// Client options specifying our own transport
	policyClientOptions := policy.ClientOptions{
		Transport: newTransporter(ctx),
	}
	clientOpt := service.ClientOptions{
		ClientOptions: policyClientOptions,
	}

	// Here we auth by setting one of cred, sharedKeyCred, f.svc or anonymous
	var (
		cred          azcore.TokenCredential
		sharedKeyCred *service.SharedKeyCredential
		anonymous     = false
	)
	switch {
	case opt.EnvAuth:
		// Read account from environment if needed
		if opt.Account == "" {
			opt.Account, _ = os.LookupEnv("AZURE_STORAGE_ACCOUNT_NAME")
		}
		// Read credentials from the environment
		options := azidentity.DefaultAzureCredentialOptions{
			ClientOptions: policyClientOptions,
		}
		cred, err = azidentity.NewDefaultAzureCredential(&options)
		if err != nil {
			return nil, fmt.Errorf("create azure environment credential failed: %w", err)
		}
	case opt.UseEmulator:
		if opt.Account == "" {
			opt.Account = emulatorAccount
		}
		if opt.Key == "" {
			opt.Key = emulatorAccountKey
		}
		if opt.Endpoint == "" {
			opt.Endpoint = emulatorBlobEndpoint
		}
		sharedKeyCred, err = service.NewSharedKeyCredential(opt.Account, opt.Key)
		if err != nil {
			return nil, fmt.Errorf("create new shared key credential for emulator failed: %w", err)
		}
	case opt.Account != "" && opt.Key != "":
		sharedKeyCred, err = service.NewSharedKeyCredential(opt.Account, opt.Key)
		if err != nil {
			return nil, fmt.Errorf("create new shared key credential failed: %w", err)
		}
	case opt.SASURL != "":
		parts, err := sas.ParseURL(opt.SASURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SAS URL: %w", err)
		}
		endpoint := opt.SASURL
		containerName := parts.ContainerName
		// Check if we have container level SAS or account level SAS
		if containerName != "" {
			// Container level SAS
			if f.rootContainer != "" && containerName != f.rootContainer {
				return nil, fmt.Errorf("container name in SAS URL (%q) and container provided in command (%q) do not match", containerName, f.rootContainer)
			}
			// Rewrite the endpoint string to be without the container
			parts.ContainerName = ""
			endpoint = parts.String()
		}
		f.svc, err = service.NewClientWithNoCredential(endpoint, &clientOpt)
		if err != nil {
			return nil, fmt.Errorf("unable to create SAS URL client: %w", err)
		}
		// if using Container level SAS put the container client into the cache
		if containerName != "" {
			_ = f.cntSVC(containerName)
			f.isLimited = true
		}
	case opt.ClientID != "" && opt.Tenant != "" && opt.ClientSecret != "":
		// Service principal with client secret
		options := azidentity.ClientSecretCredentialOptions{
			ClientOptions: policyClientOptions,
		}
		cred, err = azidentity.NewClientSecretCredential(opt.Tenant, opt.ClientID, opt.ClientSecret, &options)
		if err != nil {
			return nil, fmt.Errorf("error creating a client secret credential: %w", err)
		}
	case opt.ClientID != "" && opt.Tenant != "" && opt.ClientCertificatePath != "":
		// Service principal with certificate
		//
		// Read the certificate
		data, err := os.ReadFile(env.ShellExpand(opt.ClientCertificatePath))
		if err != nil {
			return nil, fmt.Errorf("error reading client certificate file: %w", err)
		}
		// NewClientCertificateCredential requires at least one *x509.Certificate, and a
		// crypto.PrivateKey.
		//
		// ParseCertificates returns these given certificate data in PEM or PKCS12 format.
		// It handles common scenarios but has limitations, for example it doesn't load PEM
		// encrypted private keys.
		var password []byte
		if opt.ClientCertificatePassword != "" {
			pw, err := obscure.Reveal(opt.Password)
			if err != nil {
				return nil, fmt.Errorf("certificate password decode failed - did you obscure it?: %w", err)
			}
			password = []byte(pw)
		}
		certs, key, err := azidentity.ParseCertificates(data, password)
		if err != nil {
			return nil, fmt.Errorf("failed to parse client certificate file: %w", err)
		}
		options := azidentity.ClientCertificateCredentialOptions{
			ClientOptions:        policyClientOptions,
			SendCertificateChain: opt.ClientSendCertificateChain,
		}
		cred, err = azidentity.NewClientCertificateCredential(
			opt.Tenant, opt.ClientID, certs, key, &options,
		)
		if err != nil {
			return nil, fmt.Errorf("create azure service principal with client certificate credential failed: %w", err)
		}
	case opt.ClientID != "" && opt.Tenant != "" && opt.Username != "" && opt.Password != "":
		// User with username and password
		options := azidentity.UsernamePasswordCredentialOptions{
			ClientOptions: policyClientOptions,
		}
		password, err := obscure.Reveal(opt.Password)
		if err != nil {
			return nil, fmt.Errorf("user password decode failed - did you obscure it?: %w", err)
		}
		cred, err = azidentity.NewUsernamePasswordCredential(
			opt.Tenant, opt.ClientID, opt.Username, password, &options,
		)
		if err != nil {
			return nil, fmt.Errorf("authenticate user with password failed: %w", err)
		}
	case opt.ServicePrincipalFile != "":
		// Loading service principal credentials from file.
		loadedCreds, err := os.ReadFile(env.ShellExpand(opt.ServicePrincipalFile))
		if err != nil {
			return nil, fmt.Errorf("error opening service principal credentials file: %w", err)
		}
		parsedCreds, err := parseServicePrincipalCredentials(ctx, loadedCreds)
		if err != nil {
			return nil, fmt.Errorf("error parsing service principal credentials file: %w", err)
		}
		options := azidentity.ClientSecretCredentialOptions{
			ClientOptions: policyClientOptions,
		}
		cred, err = azidentity.NewClientSecretCredential(parsedCreds.Tenant, parsedCreds.AppID, parsedCreds.Password, &options)
		if err != nil {
			return nil, fmt.Errorf("error creating a client secret credential: %w", err)
		}
	case opt.UseMSI:
		// Specifying a user-assigned identity. Exactly one of the above IDs must be specified.
		// Validate and ensure exactly one is set. (To do: better validation.)
		var b2i = map[bool]int{false: 0, true: 1}
		set := b2i[opt.MSIClientID != ""] + b2i[opt.MSIObjectID != ""] + b2i[opt.MSIResourceID != ""]
		if set > 1 {
			return nil, errors.New("more than one user-assigned identity ID is set")
		}
		var options azidentity.ManagedIdentityCredentialOptions
		switch {
		case opt.MSIClientID != "":
			options.ID = azidentity.ClientID(opt.MSIClientID)
		case opt.MSIObjectID != "":
			// FIXME this doesn't appear to be in the new SDK?
			return nil, fmt.Errorf("MSI object ID is currently unsupported")
		case opt.MSIResourceID != "":
			options.ID = azidentity.ResourceID(opt.MSIResourceID)
		}
		cred, err = azidentity.NewManagedIdentityCredential(&options)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire MSI token: %w", err)
		}
	case opt.Account != "":
		// Anonymous access
		anonymous = true
	default:
		return nil, errors.New("no authentication method configured")
	}

	// Make the client if not already created
	if f.svc == nil {
		// Work out what the endpoint is if it is still unset
		if opt.Endpoint == "" {
			if opt.Account == "" {
				return nil, fmt.Errorf("account must be set: can't make service URL")
			}
			u, err := url.Parse(fmt.Sprintf("https://%s.%s", opt.Account, storageDefaultBaseURL))
			if err != nil {
				return nil, fmt.Errorf("failed to make azure storage URL from account: %w", err)
			}
			opt.Endpoint = u.String()
		}
		if sharedKeyCred != nil {
			// Shared key cred
			f.svc, err = service.NewClientWithSharedKeyCredential(opt.Endpoint, sharedKeyCred, &clientOpt)
			if err != nil {
				return nil, fmt.Errorf("create client with shared key failed: %w", err)
			}
		} else if cred != nil {
			// Azidentity cred
			f.svc, err = service.NewClient(opt.Endpoint, cred, &clientOpt)
			if err != nil {
				return nil, fmt.Errorf("create client failed: %w", err)
			}
		} else if anonymous {
			// Anonymous public access
			f.svc, err = service.NewClientWithNoCredential(opt.Endpoint, &clientOpt)
			if err != nil {
				return nil, fmt.Errorf("create public client failed: %w", err)
			}
		}
	}
	if f.svc == nil {
		return nil, fmt.Errorf("internal error: auth failed to make credentials or client")
	}

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

// return the container client for the container passed in
func (f *Fs) cntSVC(containerName string) (containerClient *container.Client) {
	f.cntSVCcacheMu.Lock()
	defer f.cntSVCcacheMu.Unlock()
	var ok bool
	if containerClient, ok = f.cntSVCcache[containerName]; !ok {
		containerClient = f.svc.NewContainerClient(containerName)
		f.cntSVCcache[containerName] = containerClient
	}
	return containerClient
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *container.BlobItem) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	if info != nil {
		err := o.decodeMetaDataFromBlob(info)
		if err != nil {
			return nil, err
		}
	} else if !o.fs.opt.NoHeadObject {
		err := o.readMetaData(ctx) // reads info and headers, returning an error
		if err != nil {
			return nil, err
		}
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// getBlobSVC creates a blob client
func (f *Fs) getBlobSVC(container, containerPath string) *blob.Client {
	return f.cntSVC(container).NewBlobClient(containerPath)
}

// getBlockBlobSVC creates a block blob client
func (f *Fs) getBlockBlobSVC(container, containerPath string) *blockblob.Client {
	return f.cntSVC(container).NewBlockBlobClient(containerPath)
}

// updateMetadataWithModTime adds the modTime passed in to o.meta.
func (o *Object) updateMetadataWithModTime(modTime time.Time) {
	metadataMu.Lock()
	defer metadataMu.Unlock()

	// Make sure o.meta is not nil
	if o.meta == nil {
		o.meta = make(map[string]string, 1)
	}

	// Set modTimeKey in it
	o.meta[modTimeKey] = modTime.Format(timeFormatOut)
}

// Returns whether file is a directory marker or not
func isDirectoryMarker(size int64, metadata map[string]*string, remote string) bool {
	// Directory markers are 0 length
	if size == 0 {
		endsWithSlash := strings.HasSuffix(remote, "/")
		if endsWithSlash || remote == "" {
			return true
		}
		// Note that metadata with hdi_isfolder = true seems to be a
		// defacto standard for marking blobs as directories.
		// Note also that the metadata hasn't been normalised to lower case yet
		for k, v := range metadata {
			if v != nil && strings.EqualFold(k, dirMetaKey) && *v == dirMetaValue {
				return true
			}
		}
	}
	return false
}

// listFn is called from list to handle an object
type listFn func(remote string, object *container.BlobItem, isDirectory bool) error

// list lists the objects into the function supplied from
// the container and root supplied
//
// dir is the starting directory, "" for root
//
// The remote has prefix removed from it and if addContainer is set then
// it adds the container to the start.
func (f *Fs) list(ctx context.Context, containerName, directory, prefix string, addContainer bool, recurse bool, maxResults int32, fn listFn) error {
	if f.cache.IsDeleted(containerName) {
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

	pager := f.cntSVC(containerName).NewListBlobsHierarchyPager(delimiter, &container.ListBlobsHierarchyOptions{
		// Copy, Metadata, Snapshots, UncommittedBlobs, Deleted, Tags, Versions, LegalHold, ImmutabilityPolicy, DeletedWithVersions bool
		Include: container.ListBlobsInclude{
			Copy:             false,
			Metadata:         true,
			Snapshots:        false,
			UncommittedBlobs: false,
			Deleted:          false,
		},
		Prefix:     &directory,
		MaxResults: &maxResults,
	})
	foundItems := 0
	for pager.More() {
		var response container.ListBlobsHierarchyResponse
		err := f.pacer.Call(func() (bool, error) {
			var err error
			response, err = pager.NextPage(ctx)
			//response, err = f.srv.ListBlobsHierarchySegment(ctx, marker, delimiter, options)
			return f.shouldRetry(ctx, err)
		})

		if err != nil {
			// Check http error code along with service code, current SDK doesn't populate service code correctly sometimes
			if storageErr, ok := err.(*azcore.ResponseError); ok && (storageErr.ErrorCode == string(bloberror.ContainerNotFound) || storageErr.StatusCode == http.StatusNotFound) {
				return fs.ErrorDirNotFound
			}
			return err
		}
		// Advance marker to next
		// marker = response.NextMarker
		foundItems += len(response.Segment.BlobItems)
		for i := range response.Segment.BlobItems {
			file := response.Segment.BlobItems[i]
			// Finish if file name no longer has prefix
			// if prefix != "" && !strings.HasPrefix(file.Name, prefix) {
			// 	return nil
			// }
			if file.Name == nil {
				fs.Debugf(f, "Nil name received")
				continue
			}
			remote := f.opt.Enc.ToStandardPath(*file.Name)
			if !strings.HasPrefix(remote, prefix) {
				fs.Debugf(f, "Odd name received %q", remote)
				continue
			}
			isDirectory := isDirectoryMarker(*file.Properties.ContentLength, file.Metadata, remote)
			if isDirectory {
				// Don't insert the root directory
				if remote == f.opt.Enc.ToStandardPath(directory) {
					continue
				}
				// process directory markers as directories
				remote = strings.TrimRight(remote, "/")
			}
			remote = remote[len(prefix):]
			if addContainer {
				remote = path.Join(containerName, remote)
			}
			// Send object
			err = fn(remote, file, isDirectory)
			if err != nil {
				return err
			}
		}
		// Send the subdirectories
		foundItems += len(response.Segment.BlobPrefixes)
		for _, remote := range response.Segment.BlobPrefixes {
			if remote.Name == nil {
				fs.Debugf(f, "Nil prefix received")
				continue
			}
			remote := strings.TrimRight(*remote.Name, "/")
			remote = f.opt.Enc.ToStandardPath(remote)
			if !strings.HasPrefix(remote, prefix) {
				fs.Debugf(f, "Odd directory name received %q", remote)
				continue
			}
			remote = remote[len(prefix):]
			if addContainer {
				remote = path.Join(containerName, remote)
			}
			// Send object
			err = fn(remote, nil, true)
			if err != nil {
				return err
			}
		}
	}
	if f.opt.DirectoryMarkers && foundItems == 0 && directory != "" {
		// Determine whether the directory exists or not by whether it has a marker
		_, err := f.readMetaData(ctx, containerName, directory)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				return fs.ErrorDirNotFound
			}
			return err
		}
	}
	return nil
}

// Convert a list item into a DirEntry
func (f *Fs) itemToDirEntry(ctx context.Context, remote string, object *container.BlobItem, isDirectory bool) (fs.DirEntry, error) {
	if isDirectory {
		d := fs.NewDir(remote, time.Time{})
		return d, nil
	}
	o, err := f.newObjectWithInfo(ctx, remote, object)
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
	f.cntSVCcacheMu.Lock()
	defer f.cntSVCcacheMu.Unlock()
	for limitedContainer := range f.cntSVCcache {
		if container == limitedContainer {
			return true
		}
	}
	return false
}

// listDir lists a single directory
func (f *Fs) listDir(ctx context.Context, containerName, directory, prefix string, addContainer bool) (entries fs.DirEntries, err error) {
	if !f.containerOK(containerName) {
		return nil, fs.ErrorDirNotFound
	}
	err = f.list(ctx, containerName, directory, prefix, addContainer, false, int32(f.opt.ListChunkSize), func(remote string, object *container.BlobItem, isDirectory bool) error {
		entry, err := f.itemToDirEntry(ctx, remote, object, isDirectory)
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
	f.cache.MarkOK(containerName)
	return entries, nil
}

// listContainers returns all the containers to out
func (f *Fs) listContainers(ctx context.Context) (entries fs.DirEntries, err error) {
	if f.isLimited {
		f.cntSVCcacheMu.Lock()
		for container := range f.cntSVCcache {
			d := fs.NewDir(container, time.Time{})
			entries = append(entries, d)
		}
		f.cntSVCcacheMu.Unlock()
		return entries, nil
	}
	err = f.listContainersToFn(func(Name string, LastModified time.Time) error {
		d := fs.NewDir(f.opt.Enc.ToStandardName(Name), LastModified)
		f.cache.MarkOK(Name)
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
	containerName, directory := f.split(dir)
	list := walk.NewListRHelper(callback)
	listR := func(containerName, directory, prefix string, addContainer bool) error {
		return f.list(ctx, containerName, directory, prefix, addContainer, true, int32(f.opt.ListChunkSize), func(remote string, object *container.BlobItem, isDirectory bool) error {
			entry, err := f.itemToDirEntry(ctx, remote, object, isDirectory)
			if err != nil {
				return err
			}
			return list.Add(entry)
		})
	}
	if containerName == "" {
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
		if !f.containerOK(containerName) {
			return fs.ErrorDirNotFound
		}
		err = listR(containerName, directory, f.rootDirectory, f.rootContainer == "")
		if err != nil {
			return err
		}
		// container must be present if listing succeeded
		f.cache.MarkOK(containerName)
	}
	return list.Flush()
}

// listContainerFn is called from listContainersToFn to handle a container
type listContainerFn func(Name string, LastModified time.Time) error

// listContainersToFn lists the containers to the function supplied
func (f *Fs) listContainersToFn(fn listContainerFn) error {
	max := int32(f.opt.ListChunkSize)
	pager := f.svc.NewListContainersPager(&service.ListContainersOptions{
		Include:    service.ListContainersInclude{Metadata: true, Deleted: true},
		MaxResults: &max,
	})
	ctx := context.Background()
	for pager.More() {
		var response service.ListContainersResponse
		err := f.pacer.Call(func() (bool, error) {
			var err error
			response, err = pager.NextPage(ctx)
			return f.shouldRetry(ctx, err)
		})
		if err != nil {
			return err
		}

		for _, cnt := range response.ContainerItems {
			if cnt == nil || cnt.Name == nil || cnt.Properties == nil || cnt.Properties.LastModified == nil {
				fs.Debugf(f, "nil returned in container info")
			}
			err = fn(*cnt.Name, *cnt.Properties.LastModified)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned.
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

// Create directory marker file and parents
func (f *Fs) createDirectoryMarker(ctx context.Context, container, dir string) error {
	if !f.opt.DirectoryMarkers || container == "" {
		return nil
	}

	// Object to be uploaded
	o := &Object{
		fs:      f,
		modTime: time.Now(),
		meta: map[string]string{
			dirMetaKey: dirMetaValue,
		},
	}

	for {
		_, containerPath := f.split(dir)
		// Don't create the directory marker if it is the bucket or at the very root
		if containerPath == "" {
			break
		}
		o.remote = dir + "/"

		// Check to see if object already exists
		_, err := f.readMetaData(ctx, container, containerPath+"/")
		if err == nil {
			return nil
		}

		// Upload it if not
		fs.Debugf(o, "Creating directory marker")
		content := io.Reader(strings.NewReader(""))
		err = o.Update(ctx, content, o)
		if err != nil {
			return fmt.Errorf("creating directory marker failed: %w", err)
		}

		// Now check parent directory exists
		dir = path.Dir(dir)
		if dir == "/" || dir == "." {
			break
		}
	}

	return nil
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	container, _ := f.split(dir)
	e := f.makeContainer(ctx, container)
	if e != nil {
		return e
	}
	return f.createDirectoryMarker(ctx, container, dir)
}

// mkdirParent creates the parent bucket/directory if it doesn't exist
func (f *Fs) mkdirParent(ctx context.Context, remote string) error {
	remote = strings.TrimRight(remote, "/")
	dir := path.Dir(remote)
	if dir == "/" || dir == "." {
		dir = ""
	}
	return f.Mkdir(ctx, dir)
}

// makeContainer creates the container if it doesn't exist
func (f *Fs) makeContainer(ctx context.Context, container string) error {
	if f.opt.NoCheckContainer {
		return nil
	}
	return f.cache.Create(container, func() error {
		// If this is a SAS URL limited to a container then assume it is already created
		if f.isLimited {
			return nil
		}
		opt := service.CreateContainerOptions{
			// Optional. Specifies a user-defined name-value pair associated with the blob.
			//Metadata map[string]string

			// Optional. Specifies the encryption scope settings to set on the container.
			//CpkScopeInfo *CpkScopeInfo
		}
		if f.publicAccess != "" {
			// Specifies whether data in the container may be accessed publicly and the level of access
			opt.Access = &f.publicAccess
		}
		// now try to create the container
		return f.pacer.Call(func() (bool, error) {
			_, err := f.svc.CreateContainer(ctx, container, &opt)
			if err != nil {
				if storageErr, ok := err.(*azcore.ResponseError); ok {
					switch bloberror.Code(storageErr.ErrorCode) {
					case bloberror.ContainerAlreadyExists:
						return false, nil
					case bloberror.ContainerBeingDeleted:
						// From https://docs.microsoft.com/en-us/rest/api/storageservices/delete-container
						// When a container is deleted, a container with the same name cannot be created
						// for at least 30 seconds; the container may not be available for more than 30
						// seconds if the service is still processing the request.
						time.Sleep(6 * time.Second) // default 10 retries will be 60 seconds
						f.cache.MarkDeleted(container)
						return true, err
					case bloberror.AuthorizationFailure:
						// Assume that the user does not have permission to
						// create the container and carry on anyway.
						fs.Debugf(f, "Tried to create container but got %s error - carrying on assuming container exists. Use no_check_container to stop this check..", storageErr.ErrorCode)
						return false, nil
					}
				}
			}
			return f.shouldRetry(ctx, err)
		})
	}, nil)
}

// isEmpty checks to see if a given (container, directory) is empty and returns an error if not
func (f *Fs) isEmpty(ctx context.Context, containerName, directory string) (err error) {
	empty := true
	err = f.list(ctx, containerName, directory, f.rootDirectory, f.rootContainer == "", true, 1, func(remote string, object *container.BlobItem, isDirectory bool) error {
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
func (f *Fs) deleteContainer(ctx context.Context, containerName string) error {
	return f.cache.Remove(containerName, func() error {
		getOptions := container.GetPropertiesOptions{}
		delOptions := container.DeleteOptions{}
		return f.pacer.Call(func() (bool, error) {
			_, err := f.cntSVC(containerName).GetProperties(ctx, &getOptions)
			if err == nil {
				_, err = f.cntSVC(containerName).Delete(ctx, &delOptions)
			}

			if err != nil {
				// Check http error code along with service code, current SDK doesn't populate service code correctly sometimes
				if storageErr, ok := err.(*azcore.ResponseError); ok && (storageErr.ErrorCode == string(bloberror.ContainerNotFound) || storageErr.StatusCode == http.StatusNotFound) {
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
	// Remove directory marker file
	if f.opt.DirectoryMarkers && container != "" && directory != "" {
		o := &Object{
			fs:     f,
			remote: dir + "/",
		}
		fs.Debugf(o, "Removing directory marker")
		err := o.Remove(ctx)
		if err != nil {
			return fmt.Errorf("removing directory marker failed: %w", err)
		}
	}
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
	if container == "" {
		return errors.New("can't purge from root")
	}
	if directory != "" {
		// Delegate to caller if not root of a container
		return fs.ErrorCantPurge
	}
	return f.deleteContainer(ctx, container)
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
	err := f.mkdirParent(ctx, remote)
	if err != nil {
		return nil, err
	}
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	dstBlobSVC := f.getBlobSVC(dstContainer, dstPath)
	srcBlobSVC := srcObj.getBlobSVC()
	srcURL := srcBlobSVC.URL()

	options := blob.StartCopyFromURLOptions{
		Tier: parseTier(f.opt.AccessTier),
	}
	var startCopy blob.StartCopyFromURLResponse
	err = f.pacer.Call(func() (bool, error) {
		startCopy, err = dstBlobSVC.StartCopyFromURL(ctx, srcURL, &options)
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, err
	}

	copyStatus := startCopy.CopyStatus
	getOptions := blob.GetPropertiesOptions{}
	for copyStatus != nil && string(*copyStatus) == string(container.CopyStatusTypePending) {
		time.Sleep(1 * time.Second)
		getMetadata, err := dstBlobSVC.GetProperties(ctx, &getOptions)
		if err != nil {
			return nil, err
		}
		copyStatus = getMetadata.CopyStatus
	}

	return f.NewObject(ctx, remote)
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
		return "", fmt.Errorf("failed to decode Content-MD5: %q: %w", o.md5, err)
	}
	return hex.EncodeToString(data), nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	return o.size
}

// Set o.metadata from metadata
func (o *Object) setMetadata(metadata map[string]*string) {
	metadataMu.Lock()
	defer metadataMu.Unlock()

	if len(metadata) > 0 {
		// Lower case the metadata
		o.meta = make(map[string]string, len(metadata))
		for k, v := range metadata {
			if v != nil {
				o.meta[strings.ToLower(k)] = *v
			}
		}
		// Set o.modTime from metadata if it exists and
		// UseServerModTime isn't in use.
		if modTime, ok := o.meta[modTimeKey]; !o.fs.ci.UseServerModTime && ok {
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

// Get metadata from o.meta
func (o *Object) getMetadata() (metadata map[string]*string) {
	metadataMu.Lock()
	defer metadataMu.Unlock()

	if len(o.meta) == 0 {
		return nil
	}
	metadata = make(map[string]*string, len(o.meta))
	for k, v := range o.meta {
		v := v
		metadata[k] = &v
	}
	return metadata
}

// decodeMetaDataFromPropertiesResponse sets the metadata from the data passed in
//
// Sets
//
//	o.id
//	o.modTime
//	o.size
//	o.md5
//	o.meta
func (o *Object) decodeMetaDataFromPropertiesResponse(info *blob.GetPropertiesResponse) (err error) {
	metadata := info.Metadata
	var size int64
	if info.ContentLength == nil {
		size = -1
	} else {
		size = *info.ContentLength
	}
	if isDirectoryMarker(size, metadata, o.remote) {
		return fs.ErrorNotAFile
	}
	// NOTE - Client library always returns MD5 as base64 decoded string, Object needs to maintain
	// this as base64 encoded string.
	o.md5 = base64.StdEncoding.EncodeToString(info.ContentMD5)
	if info.ContentType == nil {
		o.mimeType = ""
	} else {
		o.mimeType = *info.ContentType
	}
	o.size = size
	if info.LastModified == nil {
		o.modTime = time.Now()
	} else {
		o.modTime = *info.LastModified
	}
	if info.AccessTier == nil {
		o.accessTier = blob.AccessTier("")
	} else {
		o.accessTier = blob.AccessTier(*info.AccessTier)
	}
	o.setMetadata(metadata)

	return nil
}

func (o *Object) decodeMetaDataFromDownloadResponse(info *blob.DownloadStreamResponse) (err error) {
	metadata := info.Metadata
	var size int64
	if info.ContentLength == nil {
		size = -1
	} else {
		size = *info.ContentLength
	}
	if isDirectoryMarker(size, metadata, o.remote) {
		return fs.ErrorNotAFile
	}
	// NOTE - Client library always returns MD5 as base64 decoded string, Object needs to maintain
	// this as base64 encoded string.
	o.md5 = base64.StdEncoding.EncodeToString(info.ContentMD5)
	if info.ContentType == nil {
		o.mimeType = ""
	} else {
		o.mimeType = *info.ContentType
	}
	o.size = size
	if info.LastModified == nil {
		o.modTime = time.Now()
	} else {
		o.modTime = *info.LastModified
	}
	// FIXME response doesn't appear to have AccessTier in?
	// if info.AccessTier == nil {
	// 	o.accessTier = blob.AccessTier("")
	// } else {
	// 	o.accessTier = blob.AccessTier(*info.AccessTier)
	// }
	o.setMetadata(metadata)

	// If it was a Range request, the size is wrong, so correct it
	if info.ContentRange != nil {
		contentRange := *info.ContentRange
		slash := strings.IndexRune(contentRange, '/')
		if slash >= 0 {
			i, err := strconv.ParseInt(contentRange[slash+1:], 10, 64)
			if err == nil {
				o.size = i
			} else {
				fs.Debugf(o, "Failed to find parse integer from in %q: %v", contentRange, err)
			}
		} else {
			fs.Debugf(o, "Failed to find length in %q", contentRange)
		}
	}

	return nil
}

func (o *Object) decodeMetaDataFromBlob(info *container.BlobItem) (err error) {
	if info.Properties == nil {
		return errors.New("nil Properties in decodeMetaDataFromBlob")
	}
	metadata := info.Metadata
	var size int64
	if info.Properties.ContentLength == nil {
		size = -1
	} else {
		size = *info.Properties.ContentLength
	}
	if isDirectoryMarker(size, metadata, o.remote) {
		return fs.ErrorNotAFile
	}
	// NOTE - Client library always returns MD5 as base64 decoded string, Object needs to maintain
	// this as base64 encoded string.
	o.md5 = base64.StdEncoding.EncodeToString(info.Properties.ContentMD5)
	if info.Properties.ContentType == nil {
		o.mimeType = ""
	} else {
		o.mimeType = *info.Properties.ContentType
	}
	o.size = size
	if info.Properties.LastModified == nil {
		o.modTime = time.Now()
	} else {
		o.modTime = *info.Properties.LastModified
	}
	if info.Properties.AccessTier == nil {
		o.accessTier = blob.AccessTier("")
	} else {
		o.accessTier = *info.Properties.AccessTier
	}
	o.setMetadata(metadata)

	return nil
}

// getBlobSVC creates a blob client
func (o *Object) getBlobSVC() *blob.Client {
	container, directory := o.split()
	return o.fs.getBlobSVC(container, directory)
}

// clearMetaData clears enough metadata so readMetaData will re-read it
func (o *Object) clearMetaData() {
	o.modTime = time.Time{}
}

// readMetaData gets the metadata if it hasn't already been fetched
func (f *Fs) readMetaData(ctx context.Context, container, containerPath string) (blobProperties blob.GetPropertiesResponse, err error) {
	if !f.containerOK(container) {
		return blobProperties, fs.ErrorObjectNotFound
	}
	blb := f.getBlobSVC(container, containerPath)

	// Read metadata (this includes metadata)
	options := blob.GetPropertiesOptions{}
	err = f.pacer.Call(func() (bool, error) {
		blobProperties, err = blb.GetProperties(ctx, &options)
		return f.shouldRetry(ctx, err)
	})
	if err != nil {
		// On directories - GetProperties does not work and current SDK does not populate service code correctly hence check regular http response as well
		if storageErr, ok := err.(*azcore.ResponseError); ok && (storageErr.ErrorCode == string(bloberror.BlobNotFound) || storageErr.StatusCode == http.StatusNotFound) {
			return blobProperties, fs.ErrorObjectNotFound
		}
		return blobProperties, err
	}
	return blobProperties, nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// Sets
//
//	o.id
//	o.modTime
//	o.size
//	o.md5
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if !o.modTime.IsZero() {
		return nil
	}
	container, containerPath := o.split()
	blobProperties, err := o.fs.readMetaData(ctx, container, containerPath)
	if err != nil {
		return err
	}
	return o.decodeMetaDataFromPropertiesResponse(&blobProperties)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) (result time.Time) {
	// The error is logged in readMetaData
	_ = o.readMetaData(ctx)
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	o.updateMetadataWithModTime(modTime)

	blb := o.getBlobSVC()
	opt := blob.SetMetadataOptions{}
	err := o.fs.pacer.Call(func() (bool, error) {
		_, err := blb.SetMetadata(ctx, o.getMetadata(), &opt)
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
	if o.AccessTier() == blob.AccessTierArchive {
		return nil, fmt.Errorf("blob in archive tier, you need to set tier to hot, cool, cold first")
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
	blb := o.getBlobSVC()
	opt := blob.DownloadStreamOptions{
		// When set to true and specified together with the Range, the service returns the MD5 hash for the range, as long as the
		// range is less than or equal to 4 MB in size.
		//RangeGetContentMD5 *bool

		// Range specifies a range of bytes.  The default value is all bytes.
		//Range HTTPRange
		Range: blob.HTTPRange{
			Offset: offset,
			Count:  count,
		},

		// AccessConditions *AccessConditions
		// CpkInfo          *CpkInfo
		// CpkScopeInfo     *CpkScopeInfo
	}
	var downloadResponse blob.DownloadStreamResponse
	err = o.fs.pacer.Call(func() (bool, error) {
		downloadResponse, err = blb.DownloadStream(ctx, &opt)
		return o.fs.shouldRetry(ctx, err)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open for download: %w", err)
	}
	err = o.decodeMetaDataFromDownloadResponse(&downloadResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode metadata for download: %w", err)
	}
	return downloadResponse.Body, nil
}

// Converts a string into a pointer to a string
func pString(s string) *string {
	return &s
}

// readSeekCloser joins an io.Reader and an io.Seeker and provides a no-op io.Closer
type readSeekCloser struct {
	io.Reader
	io.Seeker
}

// Close does nothing
func (rs *readSeekCloser) Close() error {
	return nil
}

// record chunk number and id for Close
type azBlock struct {
	chunkNumber uint64
	id          string
}

// Implements the fs.ChunkWriter interface
type azChunkWriter struct {
	chunkSize int64
	size      int64
	f         *Fs
	ui        uploadInfo
	blocksMu  sync.Mutex // protects the below
	blocks    []azBlock  // list of blocks for finalize
	o         *Object
}

// OpenChunkWriter returns the chunk size and a ChunkWriter
//
// Pass in the remote and the src object
// You can also use options to hint at the desired chunk size
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	// Temporary Object under construction
	o := &Object{
		fs:     f,
		remote: remote,
	}
	ui, err := o.prepareUpload(ctx, src, options)
	if err != nil {
		return info, nil, fmt.Errorf("failed to prepare upload: %w", err)
	}

	// Calculate correct partSize
	partSize := f.opt.ChunkSize
	totalParts := -1
	size := src.Size()

	// Note that the max size of file is 4.75 TB (100 MB X 50,000
	// blocks) and this is bigger than the max uncommitted block
	// size (9.52 TB) so we do not need to part commit block lists
	// or garbage collect uncommitted blocks.
	//
	// See: https://docs.microsoft.com/en-gb/rest/api/storageservices/put-block

	// size can be -1 here meaning we don't know the size of the incoming file.  We use ChunkSize
	// buffers here (default 4MB). With a maximum number of parts (50,000) this will be a file of
	// 195GB which seems like a not too unreasonable limit.
	if size == -1 {
		warnStreamUpload.Do(func() {
			fs.Logf(f, "Streaming uploads using chunk size %v will have maximum file size of %v",
				f.opt.ChunkSize, partSize*fs.SizeSuffix(blockblob.MaxBlocks))
		})
	} else {
		partSize = chunksize.Calculator(remote, size, blockblob.MaxBlocks, f.opt.ChunkSize)
		if partSize > fs.SizeSuffix(blockblob.MaxStageBlockBytes) {
			return info, nil, fmt.Errorf("can't upload as it is too big %v - takes more than %d chunks of %v", fs.SizeSuffix(size), fs.SizeSuffix(blockblob.MaxBlocks), fs.SizeSuffix(blockblob.MaxStageBlockBytes))
		}
		totalParts = int(fs.SizeSuffix(size) / partSize)
		if fs.SizeSuffix(size)%partSize != 0 {
			totalParts++
		}
	}

	fs.Debugf(o, "Multipart upload session started for %d parts of size %v", totalParts, partSize)

	chunkWriter := &azChunkWriter{
		chunkSize: int64(partSize),
		size:      size,
		f:         f,
		ui:        ui,
		o:         o,
	}
	info = fs.ChunkWriterInfo{
		ChunkSize:   int64(partSize),
		Concurrency: o.fs.opt.UploadConcurrency,
		//LeavePartsOnError: o.fs.opt.LeavePartsOnError,
	}
	fs.Debugf(o, "open chunk writer: started multipart upload")
	return info, chunkWriter, nil
}

// WriteChunk will write chunk number with reader bytes, where chunk number >= 0
func (w *azChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (int64, error) {
	if chunkNumber < 0 {
		err := fmt.Errorf("invalid chunk number provided: %v", chunkNumber)
		return -1, err
	}

	// Upload the block, with MD5 for check
	m := md5.New()
	currentChunkSize, err := io.Copy(m, reader)
	if err != nil {
		return -1, err
	}
	// If no data read, don't write the chunk
	if currentChunkSize == 0 {
		return 0, nil
	}
	md5sum := m.Sum(nil)

	// increment the blockID and save the blocks for finalize
	var binaryBlockID [8]byte // block counter as LSB first 8 bytes
	binary.LittleEndian.PutUint64(binaryBlockID[:], uint64(chunkNumber))
	blockID := base64.StdEncoding.EncodeToString(binaryBlockID[:])

	// Save the blockID for the commit
	w.blocksMu.Lock()
	w.blocks = append(w.blocks, azBlock{
		chunkNumber: uint64(chunkNumber),
		id:          blockID,
	})
	w.blocksMu.Unlock()

	err = w.f.pacer.Call(func() (bool, error) {
		// rewind the reader on retry and after reading md5
		_, err = reader.Seek(0, io.SeekStart)
		if err != nil {
			return false, err
		}
		options := blockblob.StageBlockOptions{
			// Specify the transactional md5 for the body, to be validated by the service.
			TransactionalValidation: blob.TransferValidationTypeMD5(md5sum),
		}
		_, err = w.ui.blb.StageBlock(ctx, blockID, &readSeekCloser{Reader: reader, Seeker: reader}, &options)
		if err != nil {
			if chunkNumber <= 8 {
				return w.f.shouldRetry(ctx, err)
			}
			// retry all chunks once have done the first few
			return true, err
		}
		return false, nil
	})
	if err != nil {
		return -1, fmt.Errorf("failed to upload chunk %d with %v bytes: %w", chunkNumber+1, currentChunkSize, err)
	}

	fs.Debugf(w.o, "multipart upload wrote chunk %d with %v bytes", chunkNumber+1, currentChunkSize)
	return currentChunkSize, err
}

// Abort the multipart upload.
//
// FIXME it would be nice to delete uncommitted blocks.
//
// See: https://github.com/rclone/rclone/issues/5583
//
// However there doesn't seem to be an easy way of doing this other than
// by deleting the target.
//
// This means that a failed upload deletes the target which isn't ideal.
//
// Uploading a zero length blob and deleting it will remove the
// uncommitted blocks I think.
//
// Could check to see if a file exists already and if it doesn't then
// create a 0 length file and delete it to flush the uncommitted
// blocks.
//
// This is what azcopy does
// https://github.com/MicrosoftDocs/azure-docs/issues/36347#issuecomment-541457962
func (w *azChunkWriter) Abort(ctx context.Context) error {
	fs.Debugf(w.o, "multipart upload aborted (did nothing - see issue #5583)")
	return nil
}

// Close and finalise the multipart upload
func (w *azChunkWriter) Close(ctx context.Context) (err error) {
	// sort the completed parts by part number
	sort.Slice(w.blocks, func(i, j int) bool {
		return w.blocks[i].chunkNumber < w.blocks[j].chunkNumber
	})

	// Create and check a list of block IDs
	blockIDs := make([]string, len(w.blocks))
	for i := range w.blocks {
		if w.blocks[i].chunkNumber != uint64(i) {
			return fmt.Errorf("internal error: expecting chunkNumber %d but got %d", i, w.blocks[i].chunkNumber)
		}
		chunkBytes, err := base64.StdEncoding.DecodeString(w.blocks[i].id)
		if err != nil {
			return fmt.Errorf("internal error: bad block ID: %w", err)
		}
		chunkNumber := binary.LittleEndian.Uint64(chunkBytes)
		if w.blocks[i].chunkNumber != chunkNumber {
			return fmt.Errorf("internal error: expecting decoded chunkNumber %d but got %d", w.blocks[i].chunkNumber, chunkNumber)
		}
		blockIDs[i] = w.blocks[i].id
	}

	options := blockblob.CommitBlockListOptions{
		Metadata:    w.o.getMetadata(),
		Tier:        parseTier(w.f.opt.AccessTier),
		HTTPHeaders: &w.ui.httpHeaders,
	}

	// Finalise the upload session
	err = w.f.pacer.Call(func() (bool, error) {
		_, err := w.ui.blb.CommitBlockList(ctx, blockIDs, &options)
		return w.f.shouldRetry(ctx, err)
	})
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}
	fs.Debugf(w.o, "multipart upload finished")
	return err
}

var warnStreamUpload sync.Once

// uploadMultipart uploads a file using multipart upload
//
// Write a larger blob, using CreateBlockBlob, PutBlock, and PutBlockList.
func (o *Object) uploadMultipart(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (ui uploadInfo, err error) {
	chunkWriter, err := multipart.UploadMultipart(ctx, src, in, multipart.UploadMultipartOptions{
		Open:        o.fs,
		OpenOptions: options,
	})
	if err != nil {
		return ui, err
	}
	return chunkWriter.(*azChunkWriter).ui, nil
}

// uploadSinglepart uploads a short blob using a single part upload
func (o *Object) uploadSinglepart(ctx context.Context, in io.Reader, size int64, ui uploadInfo) (err error) {
	chunkSize := int64(o.fs.opt.ChunkSize)
	// fs.Debugf(o, "Single part upload starting of object %d bytes", size)
	if size > chunkSize || size < 0 {
		return fmt.Errorf("internal error: single part upload size too big %d > %d", size, chunkSize)
	}

	rw := multipart.NewRW()
	defer fs.CheckClose(rw, &err)

	n, err := io.CopyN(rw, in, size+1)
	if err != nil && err != io.EOF {
		return fmt.Errorf("single part upload read failed: %w", err)
	}
	if n != size {
		return fmt.Errorf("single part upload: expecting to read %d bytes but read %d", size, n)
	}

	rs := &readSeekCloser{Reader: rw, Seeker: rw}

	options := blockblob.UploadOptions{
		Metadata:    o.getMetadata(),
		Tier:        parseTier(o.fs.opt.AccessTier),
		HTTPHeaders: &ui.httpHeaders,
	}

	return o.fs.pacer.Call(func() (bool, error) {
		// rewind the reader on retry
		_, err = rs.Seek(0, io.SeekStart)
		if err != nil {
			return false, err
		}
		_, err = ui.blb.Upload(ctx, rs, &options)
		return o.fs.shouldRetry(ctx, err)
	})
}

// Info needed for an upload
type uploadInfo struct {
	blb         *blockblob.Client
	httpHeaders blob.HTTPHeaders
	isDirMarker bool
}

// Prepare the object for upload
func (o *Object) prepareUpload(ctx context.Context, src fs.ObjectInfo, options []fs.OpenOption) (ui uploadInfo, err error) {
	container, containerPath := o.split()
	if container == "" || containerPath == "" {
		return ui, fmt.Errorf("can't upload to root - need a container")
	}
	// Create parent dir/bucket if not saving directory marker
	metadataMu.Lock()
	_, ui.isDirMarker = o.meta[dirMetaKey]
	metadataMu.Unlock()
	if !ui.isDirMarker {
		err = o.fs.mkdirParent(ctx, o.remote)
		if err != nil {
			return ui, err
		}
	}

	// Update Mod time
	o.updateMetadataWithModTime(src.ModTime(ctx))
	if err != nil {
		return ui, err
	}

	// Create the HTTP headers for the upload
	ui.httpHeaders = blob.HTTPHeaders{
		BlobContentType: pString(fs.MimeType(ctx, src)),
	}

	// Compute the Content-MD5 of the file. As we stream all uploads it
	// will be set in PutBlockList API call using the 'x-ms-blob-content-md5' header
	if !o.fs.opt.DisableCheckSum {
		if sourceMD5, _ := src.Hash(ctx, hash.MD5); sourceMD5 != "" {
			sourceMD5bytes, err := hex.DecodeString(sourceMD5)
			if err == nil {
				ui.httpHeaders.BlobContentMD5 = sourceMD5bytes
			} else {
				fs.Debugf(o, "Failed to decode %q as MD5: %v", sourceMD5, err)
			}
		}
	}

	// Apply upload options (also allows one to overwrite content-type)
	for _, option := range options {
		key, value := option.Header()
		lowerKey := strings.ToLower(key)
		switch lowerKey {
		case "":
			// ignore
		case "cache-control":
			ui.httpHeaders.BlobCacheControl = pString(value)
		case "content-disposition":
			ui.httpHeaders.BlobContentDisposition = pString(value)
		case "content-encoding":
			ui.httpHeaders.BlobContentEncoding = pString(value)
		case "content-language":
			ui.httpHeaders.BlobContentLanguage = pString(value)
		case "content-type":
			ui.httpHeaders.BlobContentType = pString(value)
		}
	}

	ui.blb = o.fs.getBlockBlobSVC(container, containerPath)
	return ui, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	if o.accessTier == blob.AccessTierArchive {
		if o.fs.opt.ArchiveTierDelete {
			fs.Debugf(o, "deleting archive tier blob before updating")
			err = o.Remove(ctx)
			if err != nil {
				return fmt.Errorf("failed to delete archive blob before updating: %w", err)
			}
		} else {
			return errCantUpdateArchiveTierBlobs
		}
	}

	size := src.Size()
	multipartUpload := size < 0 || size > int64(o.fs.opt.ChunkSize)
	var ui uploadInfo

	if multipartUpload {
		ui, err = o.uploadMultipart(ctx, in, src, options...)
	} else {
		ui, err = o.prepareUpload(ctx, src, options)
		if err != nil {
			return fmt.Errorf("failed to prepare upload: %w", err)
		}
		err = o.uploadSinglepart(ctx, in, size, ui)
	}
	if err != nil {
		return err
	}

	// Refresh metadata on object
	if !ui.isDirMarker {
		o.clearMetaData()
		err = o.readMetaData(ctx)
		if err != nil {
			return err
		}
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
	blb := o.getBlobSVC()
	opt := blob.DeleteOptions{}
	if o.fs.opt.DeleteSnapshots != "" {
		action := blob.DeleteSnapshotsOptionType(o.fs.opt.DeleteSnapshots)
		opt.DeleteSnapshots = &action
	}
	return o.fs.pacer.Call(func() (bool, error) {
		_, err := blb.Delete(ctx, &opt)
		return o.fs.shouldRetry(ctx, err)
	})
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// AccessTier of an object, default is of type none
func (o *Object) AccessTier() blob.AccessTier {
	return o.accessTier
}

// SetTier performs changing object tier
func (o *Object) SetTier(tier string) error {
	if !validateAccessTier(tier) {
		return fmt.Errorf("tier %s not supported by Azure Blob Storage", tier)
	}

	// Check if current tier already matches with desired tier
	if o.GetTier() == tier {
		return nil
	}
	desiredAccessTier := blob.AccessTier(tier)
	blb := o.getBlobSVC()
	ctx := context.Background()
	priority := blob.RehydratePriorityStandard
	opt := blob.SetTierOptions{
		RehydratePriority: &priority,
	}
	err := o.fs.pacer.Call(func() (bool, error) {
		_, err := blb.SetTier(ctx, desiredAccessTier, &opt)
		return o.fs.shouldRetry(ctx, err)
	})

	if err != nil {
		return fmt.Errorf("failed to set Blob Tier: %w", err)
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

func parseTier(tier string) *blob.AccessTier {
	if tier == "" {
		return nil
	}
	msTier := blob.AccessTier(tier)
	return &msTier
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = &Fs{}
	_ fs.Copier          = &Fs{}
	_ fs.PutStreamer     = &Fs{}
	_ fs.Purger          = &Fs{}
	_ fs.ListRer         = &Fs{}
	_ fs.OpenChunkWriter = &Fs{}
	_ fs.Object          = &Object{}
	_ fs.MimeTyper       = &Object{}
	_ fs.GetTierer       = &Object{}
	_ fs.SetTierer       = &Object{}
)
