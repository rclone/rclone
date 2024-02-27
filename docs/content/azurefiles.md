---
title: "Microsoft Azure Files Storage"
description: "Rclone docs for Microsoft Azure Files Storage"
versionIntroduced: "v1.65"
---

# {{< icon "fab fa-windows" >}} Microsoft Azure Files Storage

Paths are specified as `remote:` You may put subdirectories in too,
e.g. `remote:path/to/dir`.

## Configuration

Here is an example of making a Microsoft Azure Files Storage
configuration.  For a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Microsoft Azure Files Storage
   \ "azurefiles"
[snip]

Option account.
Azure Storage Account Name.
Set this to the Azure Storage Account Name in use.
Leave blank to use SAS URL or connection string, otherwise it needs to be set.
If this is blank and if env_auth is set it will be read from the
environment variable `AZURE_STORAGE_ACCOUNT_NAME` if possible.
Enter a value. Press Enter to leave empty.
account> account_name

Option share_name.
Azure Files Share Name.
This is required and is the name of the share to access.
Enter a value. Press Enter to leave empty.
share_name> share_name

Option env_auth.
Read credentials from runtime (environment variables, CLI or MSI).
See the [authentication docs](/azurefiles#authentication) for full info.
Enter a boolean value (true or false). Press Enter for the default (false).
env_auth> 

Option key.
Storage Account Shared Key.
Leave blank to use SAS URL or connection string.
Enter a value. Press Enter to leave empty.
key> base64encodedkey==

Option sas_url.
SAS URL.
Leave blank if using account/key or connection string.
Enter a value. Press Enter to leave empty.
sas_url> 

Option connection_string.
Azure Files Connection String.
Enter a value. Press Enter to leave empty.
connection_string> 
[snip]

Configuration complete.
Options:
- type: azurefiles
- account: account_name
- share_name: share_name
- key: base64encodedkey==
Keep this "remote" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> 
```

Once configured you can use rclone.

See all files in the top level:

    rclone lsf remote:

Make a new directory in the root:

    rclone mkdir remote:dir

Recursively List the contents:

    rclone ls remote:

Sync `/home/local/directory` to the remote directory, deleting any
excess files in the directory.

    rclone sync --interactive /home/local/directory remote:dir

### Modified time

The modified time is stored as Azure standard `LastModified` time on
files

### Performance

When uploading large files, increasing the value of
`--azurefiles-upload-concurrency` will increase performance at the cost
of using more memory. The default of 16 is set quite conservatively to
use less memory. It maybe be necessary raise it to 64 or higher to
fully utilize a 1 GBit/s link with a single file transfer.

### Restricted filename characters

In addition to the [default restricted characters set](/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| "         | 0x22  | ＂          |
| *         | 0x2A  | ＊          |
| :         | 0x3A  | ：          |
| <         | 0x3C  | ＜          |
| >         | 0x3E  | ＞          |
| ?         | 0x3F  | ？          |
| \         | 0x5C  | ＼          |
| \|        | 0x7C  | ｜          |

File names can also not end with the following characters.
These only get replaced if they are the last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| .         | 0x2E  | ．          |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Hashes

MD5 hashes are stored with files. Not all files will have MD5 hashes
as these have to be uploaded with the file.

### Authentication {#authentication}

There are a number of ways of supplying credentials for Azure Files
Storage. Rclone tries them in the order of the sections below.

#### Env Auth

If the `env_auth` config parameter is `true` then rclone will pull
credentials from the environment or runtime.

It tries these authentication methods in this order:

1. Environment Variables
2. Managed Service Identity Credentials
3. Azure CLI credentials (as used by the az tool)

These are described in the following sections

##### Env Auth: 1. Environment Variables

If `env_auth` is set and environment variables are present rclone
authenticates a service principal with a secret or certificate, or a
user with a password, depending on which environment variable are set.
It reads configuration from these variables, in the following order:

1. Service principal with client secret
    - `AZURE_TENANT_ID`: ID of the service principal's tenant. Also called its "directory" ID.
    - `AZURE_CLIENT_ID`: the service principal's client ID
    - `AZURE_CLIENT_SECRET`: one of the service principal's client secrets
2. Service principal with certificate
    - `AZURE_TENANT_ID`: ID of the service principal's tenant. Also called its "directory" ID.
    - `AZURE_CLIENT_ID`: the service principal's client ID
    - `AZURE_CLIENT_CERTIFICATE_PATH`: path to a PEM or PKCS12 certificate file including the private key.
    - `AZURE_CLIENT_CERTIFICATE_PASSWORD`: (optional) password for the certificate file.
    - `AZURE_CLIENT_SEND_CERTIFICATE_CHAIN`: (optional) Specifies whether an authentication request will include an x5c header to support subject name / issuer based authentication. When set to "true" or "1", authentication requests include the x5c header.
3. User with username and password
    - `AZURE_TENANT_ID`: (optional) tenant to authenticate in. Defaults to "organizations".
    - `AZURE_CLIENT_ID`: client ID of the application the user will authenticate to
    - `AZURE_USERNAME`: a username (usually an email address)
    - `AZURE_PASSWORD`: the user's password
4. Workload Identity
    - `AZURE_TENANT_ID`: Tenant to authenticate in.
    - `AZURE_CLIENT_ID`: Client ID of the application the user will authenticate to.
    - `AZURE_FEDERATED_TOKEN_FILE`: Path to projected service account token file.
    - `AZURE_AUTHORITY_HOST`: Authority of an Azure Active Directory endpoint (default: login.microsoftonline.com).


##### Env Auth: 2. Managed Service Identity Credentials

When using Managed Service Identity if the VM(SS) on which this
program is running has a system-assigned identity, it will be used by
default. If the resource has no system-assigned but exactly one
user-assigned identity, the user-assigned identity will be used by
default.

If the resource has multiple user-assigned identities you will need to
unset `env_auth` and set `use_msi` instead. See the [`use_msi`
section](#use_msi).

##### Env Auth: 3. Azure CLI credentials (as used by the az tool)

Credentials created with the `az` tool can be picked up using `env_auth`.

For example if you were to login with a service principal like this:

    az login --service-principal -u XXX -p XXX --tenant XXX

Then you could access rclone resources like this:

    rclone lsf :azurefiles,env_auth,account=ACCOUNT:

Or

    rclone lsf --azurefiles-env-auth --azurefiles-account=ACCOUNT :azurefiles:

#### Account and Shared Key

This is the most straight forward and least flexible way.  Just fill
in the `account` and `key` lines and leave the rest blank.

#### SAS URL

To use it leave `account`, `key` and `connection_string` blank and fill in `sas_url`.

#### Connection String

To use it leave `account`, `key` and "sas_url" blank and fill in `connection_string`.

#### Service principal with client secret

If these variables are set, rclone will authenticate with a service principal with a client secret.

- `tenant`: ID of the service principal's tenant. Also called its "directory" ID.
- `client_id`: the service principal's client ID
- `client_secret`: one of the service principal's client secrets

The credentials can also be placed in a file using the
`service_principal_file` configuration option.

#### Service principal with certificate

If these variables are set, rclone will authenticate with a service principal with certificate.

- `tenant`: ID of the service principal's tenant. Also called its "directory" ID.
- `client_id`: the service principal's client ID
- `client_certificate_path`: path to a PEM or PKCS12 certificate file including the private key.
- `client_certificate_password`: (optional) password for the certificate file.
- `client_send_certificate_chain`: (optional) Specifies whether an authentication request will include an x5c header to support subject name / issuer based authentication. When set to "true" or "1", authentication requests include the x5c header.

**NB** `client_certificate_password` must be obscured - see [rclone obscure](/commands/rclone_obscure/).

#### User with username and password

If these variables are set, rclone will authenticate with username and password.

- `tenant`: (optional) tenant to authenticate in. Defaults to "organizations".
- `client_id`: client ID of the application the user will authenticate to
- `username`: a username (usually an email address)
- `password`: the user's password

Microsoft doesn't recommend this kind of authentication, because it's
less secure than other authentication flows. This method is not
interactive, so it isn't compatible with any form of multi-factor
authentication, and the application must already have user or admin
consent. This credential can only authenticate work and school
accounts; it can't authenticate Microsoft accounts.

**NB** `password` must be obscured - see [rclone obscure](/commands/rclone_obscure/).

#### Managed Service Identity Credentials {#use_msi}

If `use_msi` is set then managed service identity credentials are
used. This authentication only works when running in an Azure service.
`env_auth` needs to be unset to use this.

However if you have multiple user identities to choose from these must
be explicitly specified using exactly one of the `msi_object_id`,
`msi_client_id`, or `msi_mi_res_id` parameters.

If none of `msi_object_id`, `msi_client_id`, or `msi_mi_res_id` is
set, this is is equivalent to using `env_auth`.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/azurefiles/azurefiles.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to azurefiles (Microsoft Azure Files).

#### --azurefiles-account

Azure Storage Account Name.

Set this to the Azure Storage Account Name in use.

Leave blank to use SAS URL or connection string, otherwise it needs to be set.

If this is blank and if env_auth is set it will be read from the
environment variable `AZURE_STORAGE_ACCOUNT_NAME` if possible.


Properties:

- Config:      account
- Env Var:     RCLONE_AZUREFILES_ACCOUNT
- Type:        string
- Required:    false

#### --azurefiles-share-name

Azure Files Share Name.

This is required and is the name of the share to access.


Properties:

- Config:      share_name
- Env Var:     RCLONE_AZUREFILES_SHARE_NAME
- Type:        string
- Required:    false

#### --azurefiles-env-auth

Read credentials from runtime (environment variables, CLI or MSI).

See the [authentication docs](/azurefiles#authentication) for full info.

Properties:

- Config:      env_auth
- Env Var:     RCLONE_AZUREFILES_ENV_AUTH
- Type:        bool
- Default:     false

#### --azurefiles-key

Storage Account Shared Key.

Leave blank to use SAS URL or connection string.

Properties:

- Config:      key
- Env Var:     RCLONE_AZUREFILES_KEY
- Type:        string
- Required:    false

#### --azurefiles-sas-url

SAS URL.

Leave blank if using account/key or connection string.

Properties:

- Config:      sas_url
- Env Var:     RCLONE_AZUREFILES_SAS_URL
- Type:        string
- Required:    false

#### --azurefiles-connection-string

Azure Files Connection String.

Properties:

- Config:      connection_string
- Env Var:     RCLONE_AZUREFILES_CONNECTION_STRING
- Type:        string
- Required:    false

#### --azurefiles-tenant

ID of the service principal's tenant. Also called its directory ID.

Set this if using
- Service principal with client secret
- Service principal with certificate
- User with username and password


Properties:

- Config:      tenant
- Env Var:     RCLONE_AZUREFILES_TENANT
- Type:        string
- Required:    false

#### --azurefiles-client-id

The ID of the client in use.

Set this if using
- Service principal with client secret
- Service principal with certificate
- User with username and password


Properties:

- Config:      client_id
- Env Var:     RCLONE_AZUREFILES_CLIENT_ID
- Type:        string
- Required:    false

#### --azurefiles-client-secret

One of the service principal's client secrets

Set this if using
- Service principal with client secret


Properties:

- Config:      client_secret
- Env Var:     RCLONE_AZUREFILES_CLIENT_SECRET
- Type:        string
- Required:    false

#### --azurefiles-client-certificate-path

Path to a PEM or PKCS12 certificate file including the private key.

Set this if using
- Service principal with certificate


Properties:

- Config:      client_certificate_path
- Env Var:     RCLONE_AZUREFILES_CLIENT_CERTIFICATE_PATH
- Type:        string
- Required:    false

#### --azurefiles-client-certificate-password

Password for the certificate file (optional).

Optionally set this if using
- Service principal with certificate

And the certificate has a password.


**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      client_certificate_password
- Env Var:     RCLONE_AZUREFILES_CLIENT_CERTIFICATE_PASSWORD
- Type:        string
- Required:    false

### Advanced options

Here are the Advanced options specific to azurefiles (Microsoft Azure Files).

#### --azurefiles-client-send-certificate-chain

Send the certificate chain when using certificate auth.

Specifies whether an authentication request will include an x5c header
to support subject name / issuer based authentication. When set to
true, authentication requests include the x5c header.

Optionally set this if using
- Service principal with certificate


Properties:

- Config:      client_send_certificate_chain
- Env Var:     RCLONE_AZUREFILES_CLIENT_SEND_CERTIFICATE_CHAIN
- Type:        bool
- Default:     false

#### --azurefiles-username

User name (usually an email address)

Set this if using
- User with username and password


Properties:

- Config:      username
- Env Var:     RCLONE_AZUREFILES_USERNAME
- Type:        string
- Required:    false

#### --azurefiles-password

The user's password

Set this if using
- User with username and password


**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      password
- Env Var:     RCLONE_AZUREFILES_PASSWORD
- Type:        string
- Required:    false

#### --azurefiles-service-principal-file

Path to file containing credentials for use with a service principal.

Leave blank normally. Needed only if you want to use a service principal instead of interactive login.

    $ az ad sp create-for-rbac --name "<name>" \
      --role "Storage Files Data Owner" \
      --scopes "/subscriptions/<subscription>/resourceGroups/<resource-group>/providers/Microsoft.Storage/storageAccounts/<storage-account>/blobServices/default/containers/<container>" \
      > azure-principal.json

See ["Create an Azure service principal"](https://docs.microsoft.com/en-us/cli/azure/create-an-azure-service-principal-azure-cli) and ["Assign an Azure role for access to files data"](https://docs.microsoft.com/en-us/azure/storage/common/storage-auth-aad-rbac-cli) pages for more details.

**NB** this section needs updating for Azure Files - pull requests appreciated!

It may be more convenient to put the credentials directly into the
rclone config file under the `client_id`, `tenant` and `client_secret`
keys instead of setting `service_principal_file`.


Properties:

- Config:      service_principal_file
- Env Var:     RCLONE_AZUREFILES_SERVICE_PRINCIPAL_FILE
- Type:        string
- Required:    false

#### --azurefiles-use-msi

Use a managed service identity to authenticate (only works in Azure).

When true, use a [managed service identity](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/)
to authenticate to Azure Storage instead of a SAS token or account key.

If the VM(SS) on which this program is running has a system-assigned identity, it will
be used by default. If the resource has no system-assigned but exactly one user-assigned identity,
the user-assigned identity will be used by default. If the resource has multiple user-assigned
identities, the identity to use must be explicitly specified using exactly one of the msi_object_id,
msi_client_id, or msi_mi_res_id parameters.

Properties:

- Config:      use_msi
- Env Var:     RCLONE_AZUREFILES_USE_MSI
- Type:        bool
- Default:     false

#### --azurefiles-msi-object-id

Object ID of the user-assigned MSI to use, if any.

Leave blank if msi_client_id or msi_mi_res_id specified.

Properties:

- Config:      msi_object_id
- Env Var:     RCLONE_AZUREFILES_MSI_OBJECT_ID
- Type:        string
- Required:    false

#### --azurefiles-msi-client-id

Object ID of the user-assigned MSI to use, if any.

Leave blank if msi_object_id or msi_mi_res_id specified.

Properties:

- Config:      msi_client_id
- Env Var:     RCLONE_AZUREFILES_MSI_CLIENT_ID
- Type:        string
- Required:    false

#### --azurefiles-msi-mi-res-id

Azure resource ID of the user-assigned MSI to use, if any.

Leave blank if msi_client_id or msi_object_id specified.

Properties:

- Config:      msi_mi_res_id
- Env Var:     RCLONE_AZUREFILES_MSI_MI_RES_ID
- Type:        string
- Required:    false

#### --azurefiles-endpoint

Endpoint for the service.

Leave blank normally.

Properties:

- Config:      endpoint
- Env Var:     RCLONE_AZUREFILES_ENDPOINT
- Type:        string
- Required:    false

#### --azurefiles-chunk-size

Upload chunk size.

Note that this is stored in memory and there may be up to
"--transfers" * "--azurefile-upload-concurrency" chunks stored at once
in memory.

Properties:

- Config:      chunk_size
- Env Var:     RCLONE_AZUREFILES_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     4Mi

#### --azurefiles-upload-concurrency

Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently.

If you are uploading small numbers of large files over high-speed
links and these uploads do not fully utilize your bandwidth, then
increasing this may help to speed up the transfers.

Note that chunks are stored in memory and there may be up to
"--transfers" * "--azurefile-upload-concurrency" chunks stored at once
in memory.

Properties:

- Config:      upload_concurrency
- Env Var:     RCLONE_AZUREFILES_UPLOAD_CONCURRENCY
- Type:        int
- Default:     16

#### --azurefiles-max-stream-size

Max size for streamed files.

Azure files needs to know in advance how big the file will be. When
rclone doesn't know it uses this value instead.

This will be used when rclone is streaming data, the most common uses are:

- Uploading files with `--vfs-cache-mode off` with `rclone mount`
- Using `rclone rcat`
- Copying files with unknown length

You will need this much free space in the share as the file will be this size temporarily.


Properties:

- Config:      max_stream_size
- Env Var:     RCLONE_AZUREFILES_MAX_STREAM_SIZE
- Type:        SizeSuffix
- Default:     10Gi

#### --azurefiles-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_AZUREFILES_ENCODING
- Type:        Encoding
- Default:     Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,Del,Ctl,RightPeriod,InvalidUtf8,Dot

{{< rem autogenerated options stop >}}

### Custom upload headers

You can set custom upload headers with the `--header-upload` flag. 

- Cache-Control
- Content-Disposition
- Content-Encoding
- Content-Language
- Content-Type

Eg `--header-upload "Content-Type: text/potato"`

## Limitations

MD5 sums are only uploaded with chunked files if the source has an MD5
sum.  This will always be the case for a local to azure copy.
