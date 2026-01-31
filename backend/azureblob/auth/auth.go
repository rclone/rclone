// Package auth supplies the authentication and client creation for the azure SDK
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/env"
)

const (
	// Default storage account, key and blob endpoint for emulator support,
	// though it is a base64 key checked in here, it is publicly available secret.
	emulatorAccount      = "devstoreaccount1"
	emulatorAccountKey   = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
	emulatorBlobEndpoint = "http://127.0.0.1:10000/devstoreaccount1"
)

// ConfigOptions is the common authentication options for azure
var ConfigOptions = []fs.Option{{
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
	Name: "connection_string",
	Help: `Storage Connection String.

Connection string for the storage. Leave blank if using other auth methods.
`,
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
	Name: "disable_instance_discovery",
	Help: `Skip requesting Microsoft Entra instance metadata

This should be set true only by applications authenticating in
disconnected clouds, or private clouds such as Azure Stack.

It determines whether rclone requests Microsoft Entra instance
metadata from ` + "`https://login.microsoft.com/`" + ` before
authenticating.

Setting this to true will skip this request, making you responsible
for ensuring the configured authority is valid and trustworthy.
`,
	Default:  false,
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
	Name: "use_az",
	Help: `Use Azure CLI tool az for authentication

Set to use the [Azure CLI tool az](https://learn.microsoft.com/en-us/cli/azure/)
as the sole means of authentication.

Setting this can be useful if you wish to use the az CLI on a host with
a System Managed Identity that you do not want to use.

Don't set env_auth at the same time.
`,
	Default:  false,
	Advanced: true,
}, {
	Name:     "endpoint",
	Help:     "Endpoint for the service.\n\nLeave blank normally.",
	Advanced: true,
}}

// Options defines the common auth configuration for azure backends
type Options struct {
	Account                    string `config:"account"`
	EnvAuth                    bool   `config:"env_auth"`
	Key                        string `config:"key"`
	SASURL                     string `config:"sas_url"`
	ConnectionString           string `config:"connection_string"`
	Tenant                     string `config:"tenant"`
	ClientID                   string `config:"client_id"`
	ClientSecret               string `config:"client_secret"`
	ClientCertificatePath      string `config:"client_certificate_path"`
	ClientCertificatePassword  string `config:"client_certificate_password"`
	ClientSendCertificateChain bool   `config:"client_send_certificate_chain"`
	Username                   string `config:"username"`
	Password                   string `config:"password"`
	ServicePrincipalFile       string `config:"service_principal_file"`
	DisableInstanceDiscovery   bool   `config:"disable_instance_discovery"`
	UseMSI                     bool   `config:"use_msi"`
	MSIObjectID                string `config:"msi_object_id"`
	MSIClientID                string `config:"msi_client_id"`
	MSIResourceID              string `config:"msi_mi_res_id"`
	UseEmulator                bool   `config:"use_emulator"`
	UseAZ                      bool   `config:"use_az"`
	Endpoint                   string `config:"endpoint"`
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

// NewClientOpts should be passed to configure NewClient
type NewClientOpts[Client, ClientOptions, SharedKeyCredential any] struct {
	DefaultBaseURL                   string // Base URL, eg blob.core.windows.net
	Blob                             bool   // set if this is blob storage
	RootContainer                    string // Container that rclone is looking at
	NewClient                        func(serviceURL string, cred azcore.TokenCredential, options *ClientOptions) (*Client, error)
	NewClientFromConnectionString    func(connectionString string, options *ClientOptions) (*Client, error)
	NewClientWithNoCredential        func(serviceURL string, options *ClientOptions) (*Client, error)
	NewClientWithSharedKeyCredential func(serviceURL string, cred *SharedKeyCredential, options *ClientOptions) (*Client, error)
	NewSharedKeyCredential           func(accountName, accountKey string) (*SharedKeyCredential, error)
	SetClientOptions                 func(options *ClientOptions, policyClientOptions policy.ClientOptions)
}

// NewClientResult is returned from NewClient
type NewClientResult[Client any] struct {
	Client             *Client                // Client to access the Service
	Cred               azcore.TokenCredential // how to generate tokens (may be nil)
	UsingSharedKeyCred bool                   // set if using shared key credentials
	Anonymous          bool                   // true if anonymous authentication was used
	Container          string                 // Container that SAS URL points to
}

// NewClient creates a service client from the rclone options
func NewClient[Client, ClientOptions, SharedKeyCredential any](ctx context.Context, conf NewClientOpts[Client, ClientOptions, SharedKeyCredential], opt *Options) (r NewClientResult[Client], err error) {
	var sharedKeyCred *SharedKeyCredential

	// Client options specifying our own transport
	policyClientOptions := policy.ClientOptions{
		Transport: newTransporter(ctx),
	}
	// Can't do this with generics (yet)
	// clientOpt := service.ClientOptions{
	// 	ClientOptions: policyClientOptions,
	// }
	// So call back to user
	var clientOpt ClientOptions
	conf.SetClientOptions(&clientOpt, policyClientOptions)

	// Here we auth by setting one of cred, sharedKeyCred, client or anonymous
	switch {
	case opt.EnvAuth:
		// Read account from environment if needed
		if opt.Account == "" {
			opt.Account, _ = os.LookupEnv("AZURE_STORAGE_ACCOUNT_NAME")
		}
		// Read credentials from the environment
		options := azidentity.DefaultAzureCredentialOptions{
			ClientOptions:            policyClientOptions,
			DisableInstanceDiscovery: opt.DisableInstanceDiscovery,
		}
		r.Cred, err = azidentity.NewDefaultAzureCredential(&options)
		if err != nil {
			return r, fmt.Errorf("create azure environment credential failed: %w", err)
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
		if conf.NewSharedKeyCredential == nil {
			return r, errors.New("emulator use not supported")
		}
		sharedKeyCred, err = conf.NewSharedKeyCredential(opt.Account, opt.Key)
		if err != nil {
			return r, fmt.Errorf("create new shared key credential for emulator failed: %w", err)
		}
	case opt.Account != "" && opt.Key != "":
		if conf.NewSharedKeyCredential == nil {
			return r, errors.New("shared key credentials not supported")
		}
		sharedKeyCred, err = conf.NewSharedKeyCredential(opt.Account, opt.Key)
		if err != nil {
			return r, fmt.Errorf("create new shared key credential failed: %w", err)
		}
	case opt.SASURL != "":
		parts, err := sas.ParseURL(opt.SASURL)
		if err != nil {
			return r, fmt.Errorf("failed to parse SAS URL: %w", err)
		}
		endpoint := opt.SASURL
		r.Container = parts.ContainerName
		// Check if we have container level SAS or account level SAS
		if conf.Blob && r.Container != "" {
			// Container level SAS
			if conf.RootContainer != "" && r.Container != conf.RootContainer {
				return r, fmt.Errorf("container name in SAS URL (%q) and container provided in command (%q) do not match", r.Container, conf.RootContainer)
			}
			// Rewrite the endpoint string to be without the container
			parts.ContainerName = ""
			endpoint = parts.String()
		}
		r.Client, err = conf.NewClientWithNoCredential(endpoint, &clientOpt)
		if err != nil {
			return r, fmt.Errorf("unable to create SAS URL client: %w", err)
		}
	case opt.ConnectionString != "":
		r.Client, err = conf.NewClientFromConnectionString(opt.ConnectionString, &clientOpt)
		if err != nil {
			return r, fmt.Errorf("unable to create connection string client: %w", err)
		}
	case opt.ClientID != "" && opt.Tenant != "" && opt.ClientSecret != "":
		// Service principal with client secret
		options := azidentity.ClientSecretCredentialOptions{
			ClientOptions: policyClientOptions,
		}
		r.Cred, err = azidentity.NewClientSecretCredential(opt.Tenant, opt.ClientID, opt.ClientSecret, &options)
		if err != nil {
			return r, fmt.Errorf("error creating a client secret credential: %w", err)
		}
	case opt.ClientID != "" && opt.Tenant != "" && opt.ClientCertificatePath != "":
		// Service principal with certificate
		//
		// Read the certificate
		data, err := os.ReadFile(env.ShellExpand(opt.ClientCertificatePath))
		if err != nil {
			return r, fmt.Errorf("error reading client certificate file: %w", err)
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
				return r, fmt.Errorf("certificate password decode failed - did you obscure it?: %w", err)
			}
			password = []byte(pw)
		}
		certs, key, err := azidentity.ParseCertificates(data, password)
		if err != nil {
			return r, fmt.Errorf("failed to parse client certificate file: %w", err)
		}
		options := azidentity.ClientCertificateCredentialOptions{
			ClientOptions:        policyClientOptions,
			SendCertificateChain: opt.ClientSendCertificateChain,
		}
		r.Cred, err = azidentity.NewClientCertificateCredential(
			opt.Tenant, opt.ClientID, certs, key, &options,
		)
		if err != nil {
			return r, fmt.Errorf("create azure service principal with client certificate credential failed: %w", err)
		}
	case opt.ClientID != "" && opt.Tenant != "" && opt.Username != "" && opt.Password != "":
		// User with username and password
		//nolint:staticcheck // this is deprecated due to Azure policy
		options := azidentity.UsernamePasswordCredentialOptions{
			ClientOptions: policyClientOptions,
		}
		password, err := obscure.Reveal(opt.Password)
		if err != nil {
			return r, fmt.Errorf("user password decode failed - did you obscure it?: %w", err)
		}
		r.Cred, err = azidentity.NewUsernamePasswordCredential(
			opt.Tenant, opt.ClientID, opt.Username, password, &options,
		)
		if err != nil {
			return r, fmt.Errorf("authenticate user with password failed: %w", err)
		}
	case opt.ServicePrincipalFile != "":
		// Loading service principal credentials from file.
		loadedCreds, err := os.ReadFile(env.ShellExpand(opt.ServicePrincipalFile))
		if err != nil {
			return r, fmt.Errorf("error opening service principal credentials file: %w", err)
		}
		parsedCreds, err := parseServicePrincipalCredentials(ctx, loadedCreds)
		if err != nil {
			return r, fmt.Errorf("error parsing service principal credentials file: %w", err)
		}
		options := azidentity.ClientSecretCredentialOptions{
			ClientOptions: policyClientOptions,
		}
		r.Cred, err = azidentity.NewClientSecretCredential(parsedCreds.Tenant, parsedCreds.AppID, parsedCreds.Password, &options)
		if err != nil {
			return r, fmt.Errorf("error creating a client secret credential: %w", err)
		}
	case opt.UseMSI:
		// Specifying a user-assigned identity. Exactly one of the above IDs must be specified.
		// Validate and ensure exactly one is set. (To do: better validation.)
		var b2i = map[bool]int{false: 0, true: 1}
		set := b2i[opt.MSIClientID != ""] + b2i[opt.MSIObjectID != ""] + b2i[opt.MSIResourceID != ""]
		if set > 1 {
			return r, errors.New("more than one user-assigned identity ID is set")
		}
		var options azidentity.ManagedIdentityCredentialOptions
		switch {
		case opt.MSIClientID != "":
			options.ID = azidentity.ClientID(opt.MSIClientID)
		case opt.MSIObjectID != "":
			// FIXME this doesn't appear to be in the new SDK?
			return r, fmt.Errorf("MSI object ID is currently unsupported")
		case opt.MSIResourceID != "":
			options.ID = azidentity.ResourceID(opt.MSIResourceID)
		}
		r.Cred, err = azidentity.NewManagedIdentityCredential(&options)
		if err != nil {
			return r, fmt.Errorf("failed to acquire MSI token: %w", err)
		}
	case opt.ClientID != "" && opt.Tenant != "" && opt.MSIClientID != "":
		// Workload Identity based authentication
		var options azidentity.ManagedIdentityCredentialOptions
		options.ID = azidentity.ClientID(opt.MSIClientID)

		msiCred, err := azidentity.NewManagedIdentityCredential(&options)
		if err != nil {
			return r, fmt.Errorf("failed to acquire MSI token: %w", err)
		}

		getClientAssertions := func(context.Context) (string, error) {
			token, err := msiCred.GetToken(context.Background(), policy.TokenRequestOptions{
				Scopes: []string{"api://AzureADTokenExchange"},
			})

			if err != nil {
				return "", fmt.Errorf("failed to acquire MSI token: %w", err)
			}

			return token.Token, nil
		}

		assertOpts := &azidentity.ClientAssertionCredentialOptions{}
		r.Cred, err = azidentity.NewClientAssertionCredential(
			opt.Tenant,
			opt.ClientID,
			getClientAssertions,
			assertOpts)

		if err != nil {
			return r, fmt.Errorf("failed to acquire client assertion token: %w", err)
		}
	case opt.UseAZ:
		var options = azidentity.AzureCLICredentialOptions{}
		r.Cred, err = azidentity.NewAzureCLICredential(&options)
		if err != nil {
			return r, fmt.Errorf("failed to create Azure CLI credentials: %w", err)
		}
	case opt.Account != "":
		// Anonymous access
		r.Anonymous = true
	default:
		return r, errors.New("no authentication method configured")
	}

	// Make the client if not already created
	if r.Client == nil {
		// Work out what the endpoint is if it is still unset
		if opt.Endpoint == "" {
			if opt.Account == "" {
				return r, fmt.Errorf("account must be set: can't make service URL")
			}
			u, err := url.Parse(fmt.Sprintf("https://%s.%s", opt.Account, conf.DefaultBaseURL))
			if err != nil {
				return r, fmt.Errorf("failed to make azure storage URL from account: %w", err)
			}
			opt.Endpoint = u.String()
		}
		if sharedKeyCred != nil {
			// Shared key cred
			r.Client, err = conf.NewClientWithSharedKeyCredential(opt.Endpoint, sharedKeyCred, &clientOpt)
			if err != nil {
				return r, fmt.Errorf("create client with shared key failed: %w", err)
			}
			r.UsingSharedKeyCred = true
		} else if r.Cred != nil {
			// Azidentity cred
			r.Client, err = conf.NewClient(opt.Endpoint, r.Cred, &clientOpt)
			if err != nil {
				return r, fmt.Errorf("create client failed: %w", err)
			}
		} else if r.Anonymous {
			// Anonymous public access
			r.Client, err = conf.NewClientWithNoCredential(opt.Endpoint, &clientOpt)
			if err != nil {
				return r, fmt.Errorf("create public client failed: %w", err)
			}
		}
	}
	if r.Client == nil {
		return r, fmt.Errorf("internal error: auth failed to make credentials or client")
	}
	return r, nil
}
