//go:build !plan9 && !js
// +build !plan9,!js

// Package azurefiles provides an interface to Microsoft Azure Files
package azurefiles

/*
   TODO

   This uses LastWriteTime which seems to work. The API return also
   has LastModified - needs investigation

   Needs pacer to have retries

   HTTP headers need to be passed

   Could support Metadata

   FIXME write mime type

   See FIXME markers

   Optional interfaces for Object
   - ID

*/

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/directory"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/file"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/fileerror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/service"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/share"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/env"
	"github.com/rclone/rclone/lib/readers"
)

const (
	maxFileSize           = 4 * fs.Tebi
	defaultChunkSize      = 4 * fs.Mebi
	storageDefaultBaseURL = "file.core.windows.net"
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "azurefiles",
		Description: "Microsoft Azure Files",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "account",
			Help: `Azure Storage Account Name.

Set this to the Azure Storage Account Name in use.

Leave blank to use SAS URL or connection string, otherwise it needs to be set.

If this is blank and if env_auth is set it will be read from the
environment variable ` + "`AZURE_STORAGE_ACCOUNT_NAME`" + ` if possible.
`,
			Sensitive: true,
		}, {
			Name: "share_name",
			Help: `Azure Files Share Name.

This is required and is the name of the share to access.
`,
		}, {
			Name: "env_auth",
			Help: `Read credentials from runtime (environment variables, CLI or MSI).

See the [authentication docs](/azurefiles#authentication) for full info.`,
			Default: false,
		}, {
			Name: "key",
			Help: `Storage Account Shared Key.

Leave blank to use SAS URL or connection string.`,
			Sensitive: true,
		}, {
			Name: "sas_url",
			Help: `SAS URL.

Leave blank if using account/key or connection string.`,
			Sensitive: true,
		}, {
			Name:      "connection_string",
			Help:      `Azure Files Connection String.`,
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
      --role "Storage Files Data Owner" \
      --scopes "/subscriptions/<subscription>/resourceGroups/<resource-group>/providers/Microsoft.Storage/storageAccounts/<storage-account>/blobServices/default/containers/<container>" \
      > azure-principal.json

See ["Create an Azure service principal"](https://docs.microsoft.com/en-us/cli/azure/create-an-azure-service-principal-azure-cli) and ["Assign an Azure role for access to files data"](https://docs.microsoft.com/en-us/azure/storage/common/storage-auth-aad-rbac-cli) pages for more details.

**NB** this section needs updating for Azure Files - pull requests appreciated!

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
			Name:     "endpoint",
			Help:     "Endpoint for the service.\n\nLeave blank normally.",
			Advanced: true,
		}, {
			Name: "chunk_size",
			Help: `Upload chunk size.

Note that this is stored in memory and there may be up to
"--transfers" * "--azurefile-upload-concurrency" chunks stored at once
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

Note that chunks are stored in memory and there may be up to
"--transfers" * "--azurefile-upload-concurrency" chunks stored at once
in memory.`,
			Default:  16,
			Advanced: true,
		}, {
			Name: "max_stream_size",
			Help: strings.ReplaceAll(`Max size for streamed files.

Azure files needs to know in advance how big the file will be. When
rclone doesn't know it uses this value instead.

This will be used when rclone is streaming data, the most common uses are:

- Uploading files with |--vfs-cache-mode off| with |rclone mount|
- Using |rclone rcat|
- Copying files with unknown length

You will need this much free space in the share as the file will be this size temporarily.
`, "|", "`"),
			Default:  10 * fs.Gibi,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.EncodeDoubleQuote |
				encoder.EncodeBackSlash |
				encoder.EncodeSlash |
				encoder.EncodeColon |
				encoder.EncodePipe |
				encoder.EncodeLtGt |
				encoder.EncodeAsterisk |
				encoder.EncodeQuestion |
				encoder.EncodeInvalidUtf8 |
				encoder.EncodeCtl | encoder.EncodeDel |
				encoder.EncodeDot | encoder.EncodeRightPeriod),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Account                    string               `config:"account"`
	ShareName                  string               `config:"share_name"`
	EnvAuth                    bool                 `config:"env_auth"`
	Key                        string               `config:"key"`
	SASURL                     string               `config:"sas_url"`
	ConnectionString           string               `config:"connection_string"`
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
	MaxStreamSize              fs.SizeSuffix        `config:"max_stream_size"`
	UploadConcurrency          int                  `config:"upload_concurrency"`
	Enc                        encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a root directory inside a share. The root directory can be ""
type Fs struct {
	name        string            // name of this remote
	root        string            // the path we are working on if any
	opt         Options           // parsed config options
	features    *fs.Features      // optional features
	shareClient *share.Client     // a client for the share itself
	svc         *directory.Client // the root service
}

// Object describes a Azure File Share File
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	size        int64     // Size of the object
	md5         []byte    // MD5 hash if known
	modTime     time.Time // The modified time of the object if known
	contentType string    // content type if known
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

// Factored out from NewFs so that it can be tested with opt *Options and without m configmap.Mapper
func newFsFromOptions(ctx context.Context, name, root string, opt *Options) (fs.Fs, error) {
	// Client options specifying our own transport
	policyClientOptions := policy.ClientOptions{
		Transport: newTransporter(ctx),
	}
	clientOpt := service.ClientOptions{
		ClientOptions: policyClientOptions,
	}

	// Here we auth by setting one of cred, sharedKeyCred or f.client
	var (
		cred          azcore.TokenCredential
		sharedKeyCred *service.SharedKeyCredential
		client        *service.Client
		err           error
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
	case opt.Account != "" && opt.Key != "":
		sharedKeyCred, err = service.NewSharedKeyCredential(opt.Account, opt.Key)
		if err != nil {
			return nil, fmt.Errorf("create new shared key credential failed: %w", err)
		}
	case opt.SASURL != "":
		client, err = service.NewClientWithNoCredential(opt.SASURL, &clientOpt)
		if err != nil {
			return nil, fmt.Errorf("unable to create SAS URL client: %w", err)
		}
	case opt.ConnectionString != "":
		client, err = service.NewClientFromConnectionString(opt.ConnectionString, &clientOpt)
		if err != nil {
			return nil, fmt.Errorf("unable to create connection string client: %w", err)
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
	default:
		return nil, errors.New("no authentication method configured")
	}

	// Make the client if not already created
	if client == nil {
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
			client, err = service.NewClientWithSharedKeyCredential(opt.Endpoint, sharedKeyCred, &clientOpt)
			if err != nil {
				return nil, fmt.Errorf("create client with shared key failed: %w", err)
			}
		} else if cred != nil {
			// Azidentity cred
			client, err = service.NewClient(opt.Endpoint, cred, &clientOpt)
			if err != nil {
				return nil, fmt.Errorf("create client failed: %w", err)
			}
		}
	}
	if client == nil {
		return nil, fmt.Errorf("internal error: auth failed to make credentials or client")
	}

	shareClient := client.NewShareClient(opt.ShareName)
	svc := shareClient.NewRootDirectoryClient()
	f := &Fs{
		shareClient: shareClient,
		svc:         svc,
		name:        name,
		root:        root,
		opt:         *opt,
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		PartialUploads:          true, // files are visible as they are being uploaded
		CaseInsensitive:         true,
		SlowHash:                true, // calling Hash() generally takes an extra transaction
		ReadMimeType:            true,
		WriteMimeType:           true,
	}).Fill(ctx, f)

	// Check whether a file exists at this location
	_, propsErr := f.fileClient("").GetProperties(ctx, nil)
	if propsErr == nil {
		f.root = path.Dir(root)
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// NewFs constructs an Fs from the root
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}
	return newFsFromOptions(ctx, name, root, opt)
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
	return fmt.Sprintf("azurefiles root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision return the precision of this Fs
//
// One second. FileREST API times are in RFC1123 which in the example shows a precision of seconds
// Source: https://learn.microsoft.com/en-us/rest/api/storageservices/representation-of-date-time-values-in-headers
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash sets.
//
// MD5: since it is listed as header in the response for file properties
// Source: https://learn.microsoft.com/en-us/rest/api/storageservices/get-file-properties
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet(hash.MD5)
}

// Encode remote and turn it into an absolute path in the share
func (f *Fs) absPath(remote string) string {
	return f.opt.Enc.FromStandardPath(path.Join(f.root, remote))
}

// Make a directory client from the dir
func (f *Fs) dirClient(dir string) *directory.Client {
	return f.svc.NewSubdirectoryClient(f.absPath(dir))
}

// Make a file client from the remote
func (f *Fs) fileClient(remote string) *file.Client {
	return f.svc.NewFileClient(f.absPath(remote))
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
//
// Does not return ErrorIsDir when a directory exists instead of file. since the documentation
// for [rclone.fs.Fs.NewObject] rqeuires no extra work to determine whether it is directory
//
// This initiates a network request and returns an error if object is not found.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	resp, err := f.fileClient(remote).GetProperties(ctx, nil)
	if fileerror.HasCode(err, fileerror.ParentNotFound, fileerror.ResourceNotFound) {
		return nil, fs.ErrorObjectNotFound
	} else if err != nil {
		return nil, fmt.Errorf("unable to find object remote %q: %w", remote, err)
	}

	o := &Object{
		fs:     f,
		remote: remote,
	}
	o.setMetadata(&resp)
	return o, nil
}

// Make a directory using the absolute path from the root of the share
//
// This recursiely creating parent directories all the way to the root
// of the share.
func (f *Fs) absMkdir(ctx context.Context, absPath string) error {
	if absPath == "" {
		return nil
	}
	dirClient := f.svc.NewSubdirectoryClient(absPath)

	// now := time.Now()
	// smbProps := &file.SMBProperties{
	// 	LastWriteTime: &now,
	// }
	// dirCreateOptions := &directory.CreateOptions{
	// 	FileSMBProperties: smbProps,
	// }

	_, createDirErr := dirClient.Create(ctx, nil)
	if fileerror.HasCode(createDirErr, fileerror.ParentNotFound) {
		parentDir := path.Dir(absPath)
		if parentDir == absPath {
			return fmt.Errorf("internal error: infinite recursion since parent and remote are equal")
		}
		makeParentErr := f.absMkdir(ctx, parentDir)
		if makeParentErr != nil {
			return fmt.Errorf("could not make parent of %q: %w", absPath, makeParentErr)
		}
		return f.absMkdir(ctx, absPath)
	} else if fileerror.HasCode(createDirErr, fileerror.ResourceAlreadyExists) {
		return nil
	} else if createDirErr != nil {
		return fmt.Errorf("unable to MkDir: %w", createDirErr)
	}
	return nil
}

// Mkdir creates nested directories
func (f *Fs) Mkdir(ctx context.Context, remote string) error {
	return f.absMkdir(ctx, f.absPath(remote))
}

// Make the parent directory of remote
func (f *Fs) mkParentDir(ctx context.Context, remote string) error {
	// Can't make the parent of root
	if remote == "" {
		return nil
	}
	return f.Mkdir(ctx, path.Dir(remote))
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	dirClient := f.dirClient(dir)
	_, err := dirClient.Delete(ctx, nil)
	if err != nil {
		if fileerror.HasCode(err, fileerror.DirectoryNotEmpty) {
			return fs.ErrorDirectoryNotEmpty
		} else if fileerror.HasCode(err, fileerror.ResourceNotFound) {
			return fs.ErrorDirNotFound
		}
		return fmt.Errorf("could not rmdir dir %q: %w", dir, err)
	}
	return nil
}

// Put the object
//
// Copies the reader in to the new object. This new object is returned.
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

// List the objects and directories in dir into entries. The entries can be
// returned in any order but should be for a complete directory.
//
// dir should be "" to list the root, and should not have trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't found.
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	var entries fs.DirEntries
	subDirClient := f.dirClient(dir)

	// Checking whether directory exists
	_, err := subDirClient.GetProperties(ctx, nil)
	if fileerror.HasCode(err, fileerror.ParentNotFound, fileerror.ResourceNotFound) {
		return entries, fs.ErrorDirNotFound
	} else if err != nil {
		return entries, err
	}

	var opt = &directory.ListFilesAndDirectoriesOptions{
		Include: directory.ListFilesInclude{
			Timestamps: true,
		},
	}
	pager := subDirClient.NewListFilesAndDirectoriesPager(opt)
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return entries, err
		}
		for _, directory := range resp.Segment.Directories {
			// Name          *string `xml:"Name"`
			// Attributes    *string `xml:"Attributes"`
			// ID            *string `xml:"FileId"`
			// PermissionKey *string `xml:"PermissionKey"`
			// Properties.ContentLength  *int64       `xml:"Content-Length"`
			// Properties.ChangeTime     *time.Time   `xml:"ChangeTime"`
			// Properties.CreationTime   *time.Time   `xml:"CreationTime"`
			// Properties.ETag           *azcore.ETag `xml:"Etag"`
			// Properties.LastAccessTime *time.Time   `xml:"LastAccessTime"`
			// Properties.LastModified   *time.Time   `xml:"Last-Modified"`
			// Properties.LastWriteTime  *time.Time   `xml:"LastWriteTime"`
			var modTime time.Time
			if directory.Properties.LastWriteTime != nil {
				modTime = *directory.Properties.LastWriteTime
			}
			leaf := f.opt.Enc.ToStandardPath(*directory.Name)
			entry := fs.NewDir(path.Join(dir, leaf), modTime)
			if directory.ID != nil {
				entry.SetID(*directory.ID)
			}
			if directory.Properties.ContentLength != nil {
				entry.SetSize(*directory.Properties.ContentLength)
			}
			entries = append(entries, entry)
		}
		for _, file := range resp.Segment.Files {
			leaf := f.opt.Enc.ToStandardPath(*file.Name)
			entry := &Object{
				fs:     f,
				remote: path.Join(dir, leaf),
			}
			if file.Properties.ContentLength != nil {
				entry.size = *file.Properties.ContentLength
			}
			if file.Properties.LastWriteTime != nil {
				entry.modTime = *file.Properties.LastWriteTime
			}
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Size of object in bytes
func (o *Object) Size() int64 {
	return o.size
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

// fileClient makes a specialized client for this object
func (o *Object) fileClient() *file.Client {
	return o.fs.fileClient(o.remote)
}

// set the metadata from file.GetPropertiesResponse
func (o *Object) setMetadata(resp *file.GetPropertiesResponse) {
	if resp.ContentLength != nil {
		o.size = *resp.ContentLength
	}
	o.md5 = resp.ContentMD5
	if resp.FileLastWriteTime != nil {
		o.modTime = *resp.FileLastWriteTime
	}
	if resp.ContentType != nil {
		o.contentType = *resp.ContentType
	}
}

// readMetaData gets the metadata if it hasn't already been fetched
func (o *Object) getMetadata(ctx context.Context) error {
	resp, err := o.fileClient().GetProperties(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch properties: %w", err)
	}
	o.setMetadata(&resp)
	return nil
}

// Hash returns the MD5 of an object returning a lowercase hex string
//
// May make a network request becaue the [fs.List] method does not
// return MD5 hashes for DirEntry
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if ty != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	if len(o.md5) == 0 {
		err := o.getMetadata(ctx)
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(o.md5), nil
}

// MimeType returns the content type of the Object if
// known, or "" if not
func (o *Object) MimeType(ctx context.Context) string {
	if o.contentType == "" {
		err := o.getMetadata(ctx)
		if err != nil {
			fs.Errorf(o, "Failed to fetch Content-Type")
		}
	}
	return o.contentType
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// ModTime returns the modification time of the object
//
// Returns time.Now() if not present
func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.modTime.IsZero() {
		return time.Now()
	}
	return o.modTime
}

// SetModTime sets the modification time
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	opt := file.SetHTTPHeadersOptions{
		SMBProperties: &file.SMBProperties{
			LastWriteTime: &t,
		},
	}
	_, err := o.fileClient().SetHTTPHeaders(ctx, &opt)
	if err != nil {
		return fmt.Errorf("unable to set modTime: %w", err)
	}
	o.modTime = t
	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	if _, err := o.fileClient().Delete(ctx, nil); err != nil {
		return fmt.Errorf("unable to delete remote %q: %w", o.remote, err)
	}
	return nil
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Offset and Count for range download
	var offset int64
	var count int64
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
	opt := file.DownloadStreamOptions{
		Range: file.HTTPRange{
			Offset: offset,
			Count:  count,
		},
	}
	resp, err := o.fileClient().DownloadStream(ctx, &opt)
	if err != nil {
		return nil, fmt.Errorf("could not open remote %q: %w", o.remote, err)
	}
	return resp.Body, nil
}

// Returns a pointer to t - useful for returning pointers to constants
func ptr[T any](t T) *T {
	return &t
}

var warnStreamUpload sync.Once

// Update the object with the contents of the io.Reader, modTime, size and MD5 hash
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	var (
		size           = src.Size()
		sizeUnknown    = false
		hashUnknown    = true
		fc             = o.fileClient()
		isNewlyCreated = o.modTime.IsZero()
		counter        *readers.CountingReader
		md5Hash        []byte
		hasher         = md5.New()
	)

	if size > int64(maxFileSize) {
		return fmt.Errorf("update: max supported file size is %vB. provided size is %vB", maxFileSize, fs.SizeSuffix(size))
	} else if size < 0 {
		size = int64(o.fs.opt.MaxStreamSize)
		sizeUnknown = true
		warnStreamUpload.Do(func() {
			fs.Logf(o.fs, "Streaming uploads will have maximum file size of %v - adjust with --azurefiles-max-stream-size", o.fs.opt.MaxStreamSize)
		})
	}

	if isNewlyCreated {
		// Make parent directory
		if mkDirErr := o.fs.mkParentDir(ctx, src.Remote()); mkDirErr != nil {
			return fmt.Errorf("update: unable to make parent directories: %w", mkDirErr)
		}
		// Create the file at the size given
		if _, createErr := fc.Create(ctx, size, nil); createErr != nil {
			return fmt.Errorf("update: unable to create file: %w", createErr)
		}
	} else {
		// Resize the file if needed
		if size != o.Size() {
			if _, resizeErr := fc.Resize(ctx, size, nil); resizeErr != nil {
				return fmt.Errorf("update: unable to resize while trying to update: %w ", resizeErr)
			}
		}
	}

	// Measure the size if it is unknown
	if sizeUnknown {
		counter = readers.NewCountingReader(in)
		in = counter
	}

	// Check we have a source MD5 hash...
	if hashStr, err := src.Hash(ctx, hash.MD5); err == nil && hashStr != "" {
		md5Hash, err = hex.DecodeString(hashStr)
		if err == nil {
			hashUnknown = false
		} else {
			fs.Errorf(o, "internal error: decoding hex encoded md5 %q: %v", hashStr, err)
		}
	}

	// ...if not calculate one
	if hashUnknown {
		in = io.TeeReader(in, hasher)
	}

	// Upload the file
	opt := file.UploadStreamOptions{
		ChunkSize:   int64(o.fs.opt.ChunkSize),
		Concurrency: o.fs.opt.UploadConcurrency,
	}
	if err := fc.UploadStream(ctx, in, &opt); err != nil {
		// Remove partially uploaded file on error
		if isNewlyCreated {
			if _, delErr := fc.Delete(ctx, nil); delErr != nil {
				fs.Errorf(o, "failed to delete partially uploaded file: %v", delErr)
			}
		}
		return fmt.Errorf("update: failed to upload stream: %w", err)
	}

	if sizeUnknown {
		// Read the uploaded size - the file will be truncated to that size by updateSizeHashModTime
		size = int64(counter.BytesRead())
	}
	if hashUnknown {
		md5Hash = hasher.Sum(nil)
	}

	// Update the properties
	modTime := src.ModTime(ctx)
	contentType := fs.MimeType(ctx, src)
	httpHeaders := file.HTTPHeaders{
		ContentMD5:  md5Hash,
		ContentType: &contentType,
	}
	// Apply upload options (also allows one to overwrite content-type)
	for _, option := range options {
		key, value := option.Header()
		lowerKey := strings.ToLower(key)
		switch lowerKey {
		case "cache-control":
			httpHeaders.CacheControl = &value
		case "content-disposition":
			httpHeaders.ContentDisposition = &value
		case "content-encoding":
			httpHeaders.ContentEncoding = &value
		case "content-language":
			httpHeaders.ContentLanguage = &value
		case "content-type":
			httpHeaders.ContentType = &value
		}
	}
	_, err = fc.SetHTTPHeaders(ctx, &file.SetHTTPHeadersOptions{
		FileContentLength: &size,
		SMBProperties: &file.SMBProperties{
			LastWriteTime: &modTime,
		},
		HTTPHeaders: &httpHeaders,
	})
	if err != nil {
		return fmt.Errorf("update: failed to set properties: %w", err)
	}

	// Make sure Object is in sync
	o.size = size
	o.md5 = md5Hash
	o.modTime = modTime
	o.contentType = contentType
	return nil
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
	err := f.mkParentDir(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Move: mkParentDir failed: %w", err)
	}
	opt := file.RenameOptions{
		IgnoreReadOnly:  ptr(true),
		ReplaceIfExists: ptr(true),
	}
	dstAbsPath := f.absPath(remote)
	fc := srcObj.fileClient()
	_, err = fc.Rename(ctx, dstAbsPath, &opt)
	if err != nil {
		return nil, fmt.Errorf("Move: Rename failed: %w", err)
	}
	dstObj, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Move: NewObject failed: %w", err)
	}
	return dstObj, nil
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
	dstFs := f
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	_, err := dstFs.dirClient(dstRemote).GetProperties(ctx, nil)
	if err == nil {
		return fs.ErrorDirExists
	}
	if !fileerror.HasCode(err, fileerror.ParentNotFound, fileerror.ResourceNotFound) {
		return fmt.Errorf("DirMove: failed to get status of destination directory: %w", err)
	}

	err = dstFs.mkParentDir(ctx, dstRemote)
	if err != nil {
		return fmt.Errorf("DirMove: mkParentDir failed: %w", err)
	}

	opt := directory.RenameOptions{
		IgnoreReadOnly:  ptr(false),
		ReplaceIfExists: ptr(false),
	}
	dstAbsPath := dstFs.absPath(dstRemote)
	dirClient := srcFs.dirClient(srcRemote)
	_, err = dirClient.Rename(ctx, dstAbsPath, &opt)
	if err != nil {
		if fileerror.HasCode(err, fileerror.ResourceAlreadyExists) {
			return fs.ErrorDirExists
		}
		return fmt.Errorf("DirMove: Rename failed: %w", err)
	}
	return nil
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
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	err := f.mkParentDir(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Copy: mkParentDir failed: %w", err)
	}
	opt := file.StartCopyFromURLOptions{
		CopyFileSMBInfo: &file.CopyFileSMBInfo{
			Attributes:         file.SourceCopyFileAttributes{},
			ChangeTime:         file.SourceCopyFileChangeTime{},
			CreationTime:       file.SourceCopyFileCreationTime{},
			LastWriteTime:      file.SourceCopyFileLastWriteTime{},
			PermissionCopyMode: ptr(file.PermissionCopyModeTypeSource),
			IgnoreReadOnly:     ptr(true),
		},
	}
	srcURL := srcObj.fileClient().URL()
	fc := f.fileClient(remote)
	_, err = fc.StartCopyFromURL(ctx, srcURL, &opt)
	if err != nil {
		return nil, fmt.Errorf("Copy failed: %w", err)
	}
	dstObj, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Copy: NewObject failed: %w", err)
	}
	return dstObj, nil
}

// Implementation of WriterAt
type writerAt struct {
	ctx  context.Context
	f    *Fs
	fc   *file.Client
	mu   sync.Mutex // protects variables below
	size int64
}

// Adaptor to add a Close method to bytes.Reader
type bytesReaderCloser struct {
	*bytes.Reader
}

// Close the bytesReaderCloser
func (bytesReaderCloser) Close() error {
	return nil
}

// WriteAt writes len(p) bytes from p to the underlying data stream
// at offset off. It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
// WriteAt must return a non-nil error if it returns n < len(p).
//
// If WriteAt is writing to a destination with a seek offset,
// WriteAt should not affect nor be affected by the underlying
// seek offset.
//
// Clients of WriteAt can execute parallel WriteAt calls on the same
// destination if the ranges do not overlap.
//
// Implementations must not retain p.
func (w *writerAt) WriteAt(p []byte, off int64) (n int, err error) {
	endOffset := off + int64(len(p))
	w.mu.Lock()
	if w.size < endOffset {
		_, err = w.fc.Resize(w.ctx, endOffset, nil)
		if err != nil {
			w.mu.Unlock()
			return 0, fmt.Errorf("WriteAt: failed to resize file: %w ", err)
		}
		w.size = endOffset
	}
	w.mu.Unlock()

	in := bytesReaderCloser{bytes.NewReader(p)}
	_, err = w.fc.UploadRange(w.ctx, off, in, nil)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close the writer
func (w *writerAt) Close() error {
	// FIXME should we be doing something here?
	return nil
}

// OpenWriterAt opens with a handle for random access writes
//
// Pass in the remote desired and the size if known.
//
// It truncates any existing object
func (f *Fs) OpenWriterAt(ctx context.Context, remote string, size int64) (fs.WriterAtCloser, error) {
	err := f.mkParentDir(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("OpenWriterAt: failed to create parent directory: %w", err)
	}
	fc := f.fileClient(remote)
	if size < 0 {
		size = 0
	}
	_, err = fc.Create(ctx, size, nil)
	if err != nil {
		return nil, fmt.Errorf("OpenWriterAt: unable to create file: %w", err)
	}
	w := &writerAt{
		ctx:  ctx,
		f:    f,
		fc:   fc,
		size: size,
	}
	return w, nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	stats, err := f.shareClient.GetStatistics(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read share statistics: %w", err)
	}
	usage := &fs.Usage{
		Used: stats.ShareUsageBytes, // bytes in use
	}
	return usage, nil
}

// Check the interfaces are satisfied
var (
	_ fs.Fs             = &Fs{}
	_ fs.PutStreamer    = &Fs{}
	_ fs.Abouter        = &Fs{}
	_ fs.Mover          = &Fs{}
	_ fs.DirMover       = &Fs{}
	_ fs.Copier         = &Fs{}
	_ fs.OpenWriterAter = &Fs{}
	_ fs.Object         = &Object{}
	_ fs.MimeTyper      = &Object{}
)
