---
title: "Microsoft Azure Blob Storage"
description: "Rclone docs for Microsoft Azure Blob Storage"
versionIntroduced: "v1.38"
---

# {{< icon "fab fa-windows" >}} Microsoft Azure Blob Storage

Paths are specified as `remote:container` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, e.g.
`remote:container/path/to/dir`.

## Configuration

Here is an example of making a Microsoft Azure Blob Storage
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
XX / Microsoft Azure Blob Storage
   \ "azureblob"
[snip]
Storage> azureblob
Storage Account Name
account> account_name
Storage Account Key
key> base64encodedkey==
Endpoint for the service - leave blank normally.
endpoint> 
Remote config
Configuration complete.
Options:
- type: azureblob
- account: account_name
- key: base64encodedkey==
- endpoint:
Keep this "remote" remote?
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See all containers

    rclone lsd remote:

Make a new container

    rclone mkdir remote:container

List the contents of a container

    rclone ls remote:container

Sync `/home/local/directory` to the remote container, deleting any excess
files in the container.

    rclone sync --interactive /home/local/directory remote:container

### --fast-list

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](/docs/#fast-list) for more details.

### Modification times and hashes

The modification time is stored as metadata on the object with the
`mtime` key.  It is stored using RFC3339 Format time with nanosecond
precision.  The metadata is supplied during directory listings so
there is no performance overhead to using it.

If you wish to use the Azure standard `LastModified` time stored on
the object as the modified time, then use the `--use-server-modtime`
flag. Note that rclone can't set `LastModified`, so using the
`--update` flag when syncing is recommended if using
`--use-server-modtime`.

MD5 hashes are stored with blobs. However blobs that were uploaded in
chunks only have an MD5 if the source remote was capable of MD5
hashes, e.g. the local disk.

### Performance

When uploading large files, increasing the value of
`--azureblob-upload-concurrency` will increase performance at the cost
of using more memory. The default of 16 is set quite conservatively to
use less memory. It maybe be necessary raise it to 64 or higher to
fully utilize a 1 GBit/s link with a single file transfer.

### Restricted filename characters

In addition to the [default restricted characters set](/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| /         | 0x2F  | ／           |
| \         | 0x5C  | ＼           |

File names can also not end with the following characters.
These only get replaced if they are the last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| .         | 0x2E  | ．          |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Authentication {#authentication}

There are a number of ways of supplying credentials for Azure Blob
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

    rclone lsf :azureblob,env_auth,account=ACCOUNT:CONTAINER

Or

    rclone lsf --azureblob-env-auth --azureblob-account=ACCOUNT :azureblob:CONTAINER

Which is analogous to using the `az` tool:

    az storage blob list --container-name CONTAINER --account-name ACCOUNT --auth-mode login

#### Account and Shared Key

This is the most straight forward and least flexible way.  Just fill
in the `account` and `key` lines and leave the rest blank.

#### SAS URL

This can be an account level SAS URL or container level SAS URL.

To use it leave `account` and `key` blank and fill in `sas_url`.

An account level SAS URL or container level SAS URL can be obtained
from the Azure portal or the Azure Storage Explorer.  To get a
container level SAS URL right click on a container in the Azure Blob
explorer in the Azure portal.

If you use a container level SAS URL, rclone operations are permitted
only on a particular container, e.g.

    rclone ls azureblob:container

You can also list the single container from the root. This will only
show the container specified by the SAS URL.

    $ rclone lsd azureblob:
    container/

Note that you can't see or access any other containers - this will
fail

    rclone ls azureblob:othercontainer

Container level SAS URLs are useful for temporarily allowing third
parties access to a single container or putting credentials into an
untrusted environment such as a CI build server.

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

#### Anonymous {#anonymous}

If you want to access resources with public anonymous access then set
`account` only. You can do this without making an rclone config:

    rclone lsf :azureblob,account=ACCOUNT:CONTAINER

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/azureblob/azureblob.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to azureblob (Microsoft Azure Blob Storage).

#### --azureblob-account

Azure Storage Account Name.

Set this to the Azure Storage Account Name in use.

Leave blank to use SAS URL or Emulator, otherwise it needs to be set.

If this is blank and if env_auth is set it will be read from the
environment variable `AZURE_STORAGE_ACCOUNT_NAME` if possible.


Properties:

- Config:      account
- Env Var:     RCLONE_AZUREBLOB_ACCOUNT
- Type:        string
- Required:    false

#### --azureblob-env-auth

Read credentials from runtime (environment variables, CLI or MSI).

See the [authentication docs](/azureblob#authentication) for full info.

Properties:

- Config:      env_auth
- Env Var:     RCLONE_AZUREBLOB_ENV_AUTH
- Type:        bool
- Default:     false

#### --azureblob-key

Storage Account Shared Key.

Leave blank to use SAS URL or Emulator.

Properties:

- Config:      key
- Env Var:     RCLONE_AZUREBLOB_KEY
- Type:        string
- Required:    false

#### --azureblob-sas-url

SAS URL for container level access only.

Leave blank if using account/key or Emulator.

Properties:

- Config:      sas_url
- Env Var:     RCLONE_AZUREBLOB_SAS_URL
- Type:        string
- Required:    false

#### --azureblob-tenant

ID of the service principal's tenant. Also called its directory ID.

Set this if using
- Service principal with client secret
- Service principal with certificate
- User with username and password


Properties:

- Config:      tenant
- Env Var:     RCLONE_AZUREBLOB_TENANT
- Type:        string
- Required:    false

#### --azureblob-client-id

The ID of the client in use.

Set this if using
- Service principal with client secret
- Service principal with certificate
- User with username and password


Properties:

- Config:      client_id
- Env Var:     RCLONE_AZUREBLOB_CLIENT_ID
- Type:        string
- Required:    false

#### --azureblob-client-secret

One of the service principal's client secrets

Set this if using
- Service principal with client secret


Properties:

- Config:      client_secret
- Env Var:     RCLONE_AZUREBLOB_CLIENT_SECRET
- Type:        string
- Required:    false

#### --azureblob-client-certificate-path

Path to a PEM or PKCS12 certificate file including the private key.

Set this if using
- Service principal with certificate


Properties:

- Config:      client_certificate_path
- Env Var:     RCLONE_AZUREBLOB_CLIENT_CERTIFICATE_PATH
- Type:        string
- Required:    false

#### --azureblob-client-certificate-password

Password for the certificate file (optional).

Optionally set this if using
- Service principal with certificate

And the certificate has a password.


**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      client_certificate_password
- Env Var:     RCLONE_AZUREBLOB_CLIENT_CERTIFICATE_PASSWORD
- Type:        string
- Required:    false

### Advanced options

Here are the Advanced options specific to azureblob (Microsoft Azure Blob Storage).

#### --azureblob-client-send-certificate-chain

Send the certificate chain when using certificate auth.

Specifies whether an authentication request will include an x5c header
to support subject name / issuer based authentication. When set to
true, authentication requests include the x5c header.

Optionally set this if using
- Service principal with certificate


Properties:

- Config:      client_send_certificate_chain
- Env Var:     RCLONE_AZUREBLOB_CLIENT_SEND_CERTIFICATE_CHAIN
- Type:        bool
- Default:     false

#### --azureblob-username

User name (usually an email address)

Set this if using
- User with username and password


Properties:

- Config:      username
- Env Var:     RCLONE_AZUREBLOB_USERNAME
- Type:        string
- Required:    false

#### --azureblob-password

The user's password

Set this if using
- User with username and password


**NB** Input to this must be obscured - see [rclone obscure](/commands/rclone_obscure/).

Properties:

- Config:      password
- Env Var:     RCLONE_AZUREBLOB_PASSWORD
- Type:        string
- Required:    false

#### --azureblob-service-principal-file

Path to file containing credentials for use with a service principal.

Leave blank normally. Needed only if you want to use a service principal instead of interactive login.

    $ az ad sp create-for-rbac --name "<name>" \
      --role "Storage Blob Data Owner" \
      --scopes "/subscriptions/<subscription>/resourceGroups/<resource-group>/providers/Microsoft.Storage/storageAccounts/<storage-account>/blobServices/default/containers/<container>" \
      > azure-principal.json

See ["Create an Azure service principal"](https://docs.microsoft.com/en-us/cli/azure/create-an-azure-service-principal-azure-cli) and ["Assign an Azure role for access to blob data"](https://docs.microsoft.com/en-us/azure/storage/common/storage-auth-aad-rbac-cli) pages for more details.

It may be more convenient to put the credentials directly into the
rclone config file under the `client_id`, `tenant` and `client_secret`
keys instead of setting `service_principal_file`.


Properties:

- Config:      service_principal_file
- Env Var:     RCLONE_AZUREBLOB_SERVICE_PRINCIPAL_FILE
- Type:        string
- Required:    false

#### --azureblob-use-msi

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
- Env Var:     RCLONE_AZUREBLOB_USE_MSI
- Type:        bool
- Default:     false

#### --azureblob-msi-object-id

Object ID of the user-assigned MSI to use, if any.

Leave blank if msi_client_id or msi_mi_res_id specified.

Properties:

- Config:      msi_object_id
- Env Var:     RCLONE_AZUREBLOB_MSI_OBJECT_ID
- Type:        string
- Required:    false

#### --azureblob-msi-client-id

Object ID of the user-assigned MSI to use, if any.

Leave blank if msi_object_id or msi_mi_res_id specified.

Properties:

- Config:      msi_client_id
- Env Var:     RCLONE_AZUREBLOB_MSI_CLIENT_ID
- Type:        string
- Required:    false

#### --azureblob-msi-mi-res-id

Azure resource ID of the user-assigned MSI to use, if any.

Leave blank if msi_client_id or msi_object_id specified.

Properties:

- Config:      msi_mi_res_id
- Env Var:     RCLONE_AZUREBLOB_MSI_MI_RES_ID
- Type:        string
- Required:    false

#### --azureblob-use-emulator

Uses local storage emulator if provided as 'true'.

Leave blank if using real azure storage endpoint.

Properties:

- Config:      use_emulator
- Env Var:     RCLONE_AZUREBLOB_USE_EMULATOR
- Type:        bool
- Default:     false

#### --azureblob-endpoint

Endpoint for the service.

Leave blank normally.

Properties:

- Config:      endpoint
- Env Var:     RCLONE_AZUREBLOB_ENDPOINT
- Type:        string
- Required:    false

#### --azureblob-upload-cutoff

Cutoff for switching to chunked upload (<= 256 MiB) (deprecated).

Properties:

- Config:      upload_cutoff
- Env Var:     RCLONE_AZUREBLOB_UPLOAD_CUTOFF
- Type:        string
- Required:    false

#### --azureblob-chunk-size

Upload chunk size.

Note that this is stored in memory and there may be up to
"--transfers" * "--azureblob-upload-concurrency" chunks stored at once
in memory.

Properties:

- Config:      chunk_size
- Env Var:     RCLONE_AZUREBLOB_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     4Mi

#### --azureblob-upload-concurrency

Concurrency for multipart uploads.

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
in memory.

Properties:

- Config:      upload_concurrency
- Env Var:     RCLONE_AZUREBLOB_UPLOAD_CONCURRENCY
- Type:        int
- Default:     16

#### --azureblob-list-chunk

Size of blob list.

This sets the number of blobs requested in each listing chunk. Default
is the maximum, 5000. "List blobs" requests are permitted 2 minutes
per megabyte to complete. If an operation is taking longer than 2
minutes per megabyte on average, it will time out (
[source](https://docs.microsoft.com/en-us/rest/api/storageservices/setting-timeouts-for-blob-service-operations#exceptions-to-default-timeout-interval)
). This can be used to limit the number of blobs items to return, to
avoid the time out.

Properties:

- Config:      list_chunk
- Env Var:     RCLONE_AZUREBLOB_LIST_CHUNK
- Type:        int
- Default:     5000

#### --azureblob-access-tier

Access tier of blob: hot, cool, cold or archive.

Archived blobs can be restored by setting access tier to hot, cool or
cold. Leave blank if you intend to use default access tier, which is
set at account level

If there is no "access tier" specified, rclone doesn't apply any tier.
rclone performs "Set Tier" operation on blobs while uploading, if objects
are not modified, specifying "access tier" to new one will have no effect.
If blobs are in "archive tier" at remote, trying to perform data transfer
operations from remote will not be allowed. User should first restore by
tiering blob to "Hot", "Cool" or "Cold".

Properties:

- Config:      access_tier
- Env Var:     RCLONE_AZUREBLOB_ACCESS_TIER
- Type:        string
- Required:    false

#### --azureblob-archive-tier-delete

Delete archive tier blobs before overwriting.

Archive tier blobs cannot be updated. So without this flag, if you
attempt to update an archive tier blob, then rclone will produce the
error:

    can't update archive tier blob without --azureblob-archive-tier-delete

With this flag set then before rclone attempts to overwrite an archive
tier blob, it will delete the existing blob before uploading its
replacement.  This has the potential for data loss if the upload fails
(unlike updating a normal blob) and also may cost more since deleting
archive tier blobs early may be chargable.


Properties:

- Config:      archive_tier_delete
- Env Var:     RCLONE_AZUREBLOB_ARCHIVE_TIER_DELETE
- Type:        bool
- Default:     false

#### --azureblob-disable-checksum

Don't store MD5 checksum with object metadata.

Normally rclone will calculate the MD5 checksum of the input before
uploading it so it can add it to metadata on the object. This is great
for data integrity checking but can cause long delays for large files
to start uploading.

Properties:

- Config:      disable_checksum
- Env Var:     RCLONE_AZUREBLOB_DISABLE_CHECKSUM
- Type:        bool
- Default:     false

#### --azureblob-memory-pool-flush-time

How often internal memory buffer pools will be flushed. (no longer used)

Properties:

- Config:      memory_pool_flush_time
- Env Var:     RCLONE_AZUREBLOB_MEMORY_POOL_FLUSH_TIME
- Type:        Duration
- Default:     1m0s

#### --azureblob-memory-pool-use-mmap

Whether to use mmap buffers in internal memory pool. (no longer used)

Properties:

- Config:      memory_pool_use_mmap
- Env Var:     RCLONE_AZUREBLOB_MEMORY_POOL_USE_MMAP
- Type:        bool
- Default:     false

#### --azureblob-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_AZUREBLOB_ENCODING
- Type:        Encoding
- Default:     Slash,BackSlash,Del,Ctl,RightPeriod,InvalidUtf8

#### --azureblob-public-access

Public access level of a container: blob or container.

Properties:

- Config:      public_access
- Env Var:     RCLONE_AZUREBLOB_PUBLIC_ACCESS
- Type:        string
- Required:    false
- Examples:
    - ""
        - The container and its blobs can be accessed only with an authorized request.
        - It's a default value.
    - "blob"
        - Blob data within this container can be read via anonymous request.
    - "container"
        - Allow full public read access for container and blob data.

#### --azureblob-directory-markers

Upload an empty object with a trailing slash when a new directory is created

Empty folders are unsupported for bucket based remotes, this option
creates an empty object ending with "/", to persist the folder.

This object also has the metadata "hdi_isfolder = true" to conform to
the Microsoft standard.
 

Properties:

- Config:      directory_markers
- Env Var:     RCLONE_AZUREBLOB_DIRECTORY_MARKERS
- Type:        bool
- Default:     false

#### --azureblob-no-check-container

If set, don't attempt to check the container exists or create it.

This can be useful when trying to minimise the number of transactions
rclone does if you know the container exists already.


Properties:

- Config:      no_check_container
- Env Var:     RCLONE_AZUREBLOB_NO_CHECK_CONTAINER
- Type:        bool
- Default:     false

#### --azureblob-no-head-object

If set, do not do HEAD before GET when getting objects.

Properties:

- Config:      no_head_object
- Env Var:     RCLONE_AZUREBLOB_NO_HEAD_OBJECT
- Type:        bool
- Default:     false

#### --azureblob-delete-snapshots

Set to specify how to deal with snapshots on blob deletion.

Properties:

- Config:      delete_snapshots
- Env Var:     RCLONE_AZUREBLOB_DELETE_SNAPSHOTS
- Type:        string
- Required:    false
- Choices:
    - ""
        - By default, the delete operation fails if a blob has snapshots
    - "include"
        - Specify 'include' to remove the root blob and all its snapshots
    - "only"
        - Specify 'only' to remove only the snapshots but keep the root blob.

#### --azureblob-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_AZUREBLOB_DESCRIPTION
- Type:        string
- Required:    false

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

`rclone about` is not supported by the Microsoft Azure Blob storage backend. Backends without
this capability cannot determine free space for an rclone mount or
use policy `mfs` (most free space) as a member of an rclone union
remote.

See [List of backends that do not support rclone about](https://rclone.org/overview/#optional-features) and [rclone about](https://rclone.org/commands/rclone_about/)

## Azure Storage Emulator Support

You can run rclone with the storage emulator (usually _azurite_).

To do this, just set up a new remote with `rclone config` following
the instructions in the introduction and set `use_emulator` in the
advanced settings as `true`. You do not need to provide a default
account name nor an account key. But you can override them in the
`account` and `key` options. (Prior to v1.61 they were hard coded to
_azurite_'s `devstoreaccount1`.)

Also, if you want to access a storage emulator instance running on a
different machine, you can override the `endpoint` parameter in the
advanced settings, setting it to
`http(s)://<host>:<port>/devstoreaccount1`
(e.g. `http://10.254.2.5:10000/devstoreaccount1`).
