---
title: "Swift"
description: "Swift"
versionIntroduced: "v0.91"
---

# {{< icon "fa fa-space-shuttle" >}} Swift

Swift refers to [OpenStack Object Storage](https://docs.openstack.org/swift/latest/).
Commercial implementations of that being:

  * [Rackspace Cloud Files](https://www.rackspace.com/cloud/files/)
  * [Memset Memstore](https://www.memset.com/cloud/storage/)
  * [OVH Object Storage](https://www.ovh.co.uk/public-cloud/storage/object-storage/)
  * [Oracle Cloud Storage](https://docs.oracle.com/en-us/iaas/integration/doc/configure-object-storage.html)
  * [Blomp Cloud Storage](https://www.blomp.com/cloud-storage/)
  * [IBM Bluemix Cloud ObjectStorage Swift](https://console.bluemix.net/docs/infrastructure/objectstorage-swift/index.html)

Paths are specified as `remote:container` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, e.g. `remote:container/path/to/dir`.

## Configuration

Here is an example of making a swift configuration.  First run

    rclone config

This will guide you through an interactive setup process.

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
XX / OpenStack Swift (Rackspace Cloud Files, Blomp Cloud Storage, Memset Memstore, OVH)
   \ "swift"
[snip]
Storage> swift
Get swift credentials from environment variables in standard OpenStack form.
Choose a number from below, or type in your own value
 1 / Enter swift credentials in the next step
   \ "false"
 2 / Get swift credentials from environment vars. Leave other fields blank if using this.
   \ "true"
env_auth> true
User name to log in (OS_USERNAME).
user> 
API key or password (OS_PASSWORD).
key> 
Authentication URL for server (OS_AUTH_URL).
Choose a number from below, or type in your own value
 1 / Rackspace US
   \ "https://auth.api.rackspacecloud.com/v1.0"
 2 / Rackspace UK
   \ "https://lon.auth.api.rackspacecloud.com/v1.0"
 3 / Rackspace v2
   \ "https://identity.api.rackspacecloud.com/v2.0"
 4 / Memset Memstore UK
   \ "https://auth.storage.memset.com/v1.0"
 5 / Memset Memstore UK v2
   \ "https://auth.storage.memset.com/v2.0"
 6 / OVH
   \ "https://auth.cloud.ovh.net/v3"
 7  / Blomp Cloud Storage
   \ "https://authenticate.ain.net"
auth> 
User ID to log in - optional - most swift systems use user and leave this blank (v3 auth) (OS_USER_ID).
user_id> 
User domain - optional (v3 auth) (OS_USER_DOMAIN_NAME)
domain> 
Tenant name - optional for v1 auth, this or tenant_id required otherwise (OS_TENANT_NAME or OS_PROJECT_NAME)
tenant> 
Tenant ID - optional for v1 auth, this or tenant required otherwise (OS_TENANT_ID)
tenant_id> 
Tenant domain - optional (v3 auth) (OS_PROJECT_DOMAIN_NAME)
tenant_domain> 
Region name - optional (OS_REGION_NAME)
region> 
Storage URL - optional (OS_STORAGE_URL)
storage_url> 
Auth Token from alternate authentication - optional (OS_AUTH_TOKEN)
auth_token> 
AuthVersion - optional - set to (1,2,3) if your auth URL has no version (ST_AUTH_VERSION)
auth_version> 
Endpoint type to choose from the service catalogue (OS_ENDPOINT_TYPE)
Choose a number from below, or type in your own value
 1 / Public (default, choose this if not sure)
   \ "public"
 2 / Internal (use internal service net)
   \ "internal"
 3 / Admin
   \ "admin"
endpoint_type> 
Remote config
--------------------
[test]
env_auth = true
user = 
key = 
auth = 
user_id = 
domain = 
tenant = 
tenant_id = 
tenant_domain = 
region = 
storage_url = 
auth_token = 
auth_version = 
endpoint_type = 
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `remote` and can now be used like this

See all containers

    rclone lsd remote:

Make a new container

    rclone mkdir remote:container

List the contents of a container

    rclone ls remote:container

Sync `/home/local/directory` to the remote container, deleting any
excess files in the container.

    rclone sync --interactive /home/local/directory remote:container

### Configuration from an OpenStack credentials file

An OpenStack credentials file typically looks something something
like this (without the comments)

```
export OS_AUTH_URL=https://a.provider.net/v2.0
export OS_TENANT_ID=ffffffffffffffffffffffffffffffff
export OS_TENANT_NAME="1234567890123456"
export OS_USERNAME="123abc567xy"
echo "Please enter your OpenStack Password: "
read -sr OS_PASSWORD_INPUT
export OS_PASSWORD=$OS_PASSWORD_INPUT
export OS_REGION_NAME="SBG1"
if [ -z "$OS_REGION_NAME" ]; then unset OS_REGION_NAME; fi
```

The config file needs to look something like this where `$OS_USERNAME`
represents the value of the `OS_USERNAME` variable - `123abc567xy` in
the example above.

```
[remote]
type = swift
user = $OS_USERNAME
key = $OS_PASSWORD
auth = $OS_AUTH_URL
tenant = $OS_TENANT_NAME
```

Note that you may (or may not) need to set `region` too - try without first.

### Configuration from the environment

If you prefer you can configure rclone to use swift using a standard
set of OpenStack environment variables.

When you run through the config, make sure you choose `true` for
`env_auth` and leave everything else blank.

rclone will then set any empty config parameters from the environment
using standard OpenStack environment variables.  There is [a list of
the
variables](https://godoc.org/github.com/ncw/swift#Connection.ApplyEnvironment)
in the docs for the swift library.

### Using an alternate authentication method

If your OpenStack installation uses a non-standard authentication method
that might not be yet supported by rclone or the underlying swift library, 
you can authenticate externally (e.g. calling manually the `openstack` 
commands to get a token). Then, you just need to pass the two 
configuration variables ``auth_token`` and ``storage_url``. 
If they are both provided, the other variables are ignored. rclone will 
not try to authenticate but instead assume it is already authenticated 
and use these two variables to access the OpenStack installation.

#### Using rclone without a config file

You can use rclone with swift without a config file, if desired, like
this:

```
source openstack-credentials-file
export RCLONE_CONFIG_MYREMOTE_TYPE=swift
export RCLONE_CONFIG_MYREMOTE_ENV_AUTH=true
rclone lsd myremote:
```

### --fast-list

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](/docs/#fast-list) for more details.

### --update and --use-server-modtime

As noted below, the modified time is stored on metadata on the object. It is
used by default for all operations that require checking the time a file was
last updated. It allows rclone to treat the remote more like a true filesystem,
but it is inefficient because it requires an extra API call to retrieve the
metadata.

For many operations, the time the object was last uploaded to the remote is
sufficient to determine if it is "dirty". By using `--update` along with
`--use-server-modtime`, you can avoid the extra API call and simply upload
files whose local modtime is newer than the time it was last uploaded.

### Modification times and hashes

The modified time is stored as metadata on the object as
`X-Object-Meta-Mtime` as floating point since the epoch accurate to 1
ns.

This is a de facto standard (used in the official python-swiftclient
amongst others) for storing the modification time for an object.

The MD5 hash algorithm is supported.

### Restricted filename characters

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| /         | 0x2F  | ／          |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8),
as they can't be used in JSON strings.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/swift/swift.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to swift (OpenStack Swift (Rackspace Cloud Files, Blomp Cloud Storage, Memset Memstore, OVH)).

#### --swift-env-auth

Get swift credentials from environment variables in standard OpenStack form.

Properties:

- Config:      env_auth
- Env Var:     RCLONE_SWIFT_ENV_AUTH
- Type:        bool
- Default:     false
- Examples:
    - "false"
        - Enter swift credentials in the next step.
    - "true"
        - Get swift credentials from environment vars.
        - Leave other fields blank if using this.

#### --swift-user

User name to log in (OS_USERNAME).

Properties:

- Config:      user
- Env Var:     RCLONE_SWIFT_USER
- Type:        string
- Required:    false

#### --swift-key

API key or password (OS_PASSWORD).

Properties:

- Config:      key
- Env Var:     RCLONE_SWIFT_KEY
- Type:        string
- Required:    false

#### --swift-auth

Authentication URL for server (OS_AUTH_URL).

Properties:

- Config:      auth
- Env Var:     RCLONE_SWIFT_AUTH
- Type:        string
- Required:    false
- Examples:
    - "https://auth.api.rackspacecloud.com/v1.0"
        - Rackspace US
    - "https://lon.auth.api.rackspacecloud.com/v1.0"
        - Rackspace UK
    - "https://identity.api.rackspacecloud.com/v2.0"
        - Rackspace v2
    - "https://auth.storage.memset.com/v1.0"
        - Memset Memstore UK
    - "https://auth.storage.memset.com/v2.0"
        - Memset Memstore UK v2
    - "https://auth.cloud.ovh.net/v3"
        - OVH
    - "https://authenticate.ain.net"
        - Blomp Cloud Storage

#### --swift-user-id

User ID to log in - optional - most swift systems use user and leave this blank (v3 auth) (OS_USER_ID).

Properties:

- Config:      user_id
- Env Var:     RCLONE_SWIFT_USER_ID
- Type:        string
- Required:    false

#### --swift-domain

User domain - optional (v3 auth) (OS_USER_DOMAIN_NAME)

Properties:

- Config:      domain
- Env Var:     RCLONE_SWIFT_DOMAIN
- Type:        string
- Required:    false

#### --swift-tenant

Tenant name - optional for v1 auth, this or tenant_id required otherwise (OS_TENANT_NAME or OS_PROJECT_NAME).

Properties:

- Config:      tenant
- Env Var:     RCLONE_SWIFT_TENANT
- Type:        string
- Required:    false

#### --swift-tenant-id

Tenant ID - optional for v1 auth, this or tenant required otherwise (OS_TENANT_ID).

Properties:

- Config:      tenant_id
- Env Var:     RCLONE_SWIFT_TENANT_ID
- Type:        string
- Required:    false

#### --swift-tenant-domain

Tenant domain - optional (v3 auth) (OS_PROJECT_DOMAIN_NAME).

Properties:

- Config:      tenant_domain
- Env Var:     RCLONE_SWIFT_TENANT_DOMAIN
- Type:        string
- Required:    false

#### --swift-region

Region name - optional (OS_REGION_NAME).

Properties:

- Config:      region
- Env Var:     RCLONE_SWIFT_REGION
- Type:        string
- Required:    false

#### --swift-storage-url

Storage URL - optional (OS_STORAGE_URL).

Properties:

- Config:      storage_url
- Env Var:     RCLONE_SWIFT_STORAGE_URL
- Type:        string
- Required:    false

#### --swift-auth-token

Auth Token from alternate authentication - optional (OS_AUTH_TOKEN).

Properties:

- Config:      auth_token
- Env Var:     RCLONE_SWIFT_AUTH_TOKEN
- Type:        string
- Required:    false

#### --swift-application-credential-id

Application Credential ID (OS_APPLICATION_CREDENTIAL_ID).

Properties:

- Config:      application_credential_id
- Env Var:     RCLONE_SWIFT_APPLICATION_CREDENTIAL_ID
- Type:        string
- Required:    false

#### --swift-application-credential-name

Application Credential Name (OS_APPLICATION_CREDENTIAL_NAME).

Properties:

- Config:      application_credential_name
- Env Var:     RCLONE_SWIFT_APPLICATION_CREDENTIAL_NAME
- Type:        string
- Required:    false

#### --swift-application-credential-secret

Application Credential Secret (OS_APPLICATION_CREDENTIAL_SECRET).

Properties:

- Config:      application_credential_secret
- Env Var:     RCLONE_SWIFT_APPLICATION_CREDENTIAL_SECRET
- Type:        string
- Required:    false

#### --swift-auth-version

AuthVersion - optional - set to (1,2,3) if your auth URL has no version (ST_AUTH_VERSION).

Properties:

- Config:      auth_version
- Env Var:     RCLONE_SWIFT_AUTH_VERSION
- Type:        int
- Default:     0

#### --swift-endpoint-type

Endpoint type to choose from the service catalogue (OS_ENDPOINT_TYPE).

Properties:

- Config:      endpoint_type
- Env Var:     RCLONE_SWIFT_ENDPOINT_TYPE
- Type:        string
- Default:     "public"
- Examples:
    - "public"
        - Public (default, choose this if not sure)
    - "internal"
        - Internal (use internal service net)
    - "admin"
        - Admin

#### --swift-storage-policy

The storage policy to use when creating a new container.

This applies the specified storage policy when creating a new
container. The policy cannot be changed afterwards. The allowed
configuration values and their meaning depend on your Swift storage
provider.

Properties:

- Config:      storage_policy
- Env Var:     RCLONE_SWIFT_STORAGE_POLICY
- Type:        string
- Required:    false
- Examples:
    - ""
        - Default
    - "pcs"
        - OVH Public Cloud Storage
    - "pca"
        - OVH Public Cloud Archive

### Advanced options

Here are the Advanced options specific to swift (OpenStack Swift (Rackspace Cloud Files, Blomp Cloud Storage, Memset Memstore, OVH)).

#### --swift-leave-parts-on-error

If true avoid calling abort upload on a failure.

It should be set to true for resuming uploads across different sessions.

Properties:

- Config:      leave_parts_on_error
- Env Var:     RCLONE_SWIFT_LEAVE_PARTS_ON_ERROR
- Type:        bool
- Default:     false

#### --swift-fetch-until-empty-page

When paginating, always fetch unless we received an empty page.

Consider using this option if rclone listings show fewer objects
than expected, or if repeated syncs copy unchanged objects.

It is safe to enable this, but rclone may make more API calls than
necessary.

This is one of a pair of workarounds to handle implementations
of the Swift API that do not implement pagination as expected.  See
also "partial_page_fetch_threshold".

Properties:

- Config:      fetch_until_empty_page
- Env Var:     RCLONE_SWIFT_FETCH_UNTIL_EMPTY_PAGE
- Type:        bool
- Default:     false

#### --swift-partial-page-fetch-threshold

When paginating, fetch if the current page is within this percentage of the limit.

Consider using this option if rclone listings show fewer objects
than expected, or if repeated syncs copy unchanged objects.

It is safe to enable this, but rclone may make more API calls than
necessary.

This is one of a pair of workarounds to handle implementations
of the Swift API that do not implement pagination as expected.  See
also "fetch_until_empty_page".

Properties:

- Config:      partial_page_fetch_threshold
- Env Var:     RCLONE_SWIFT_PARTIAL_PAGE_FETCH_THRESHOLD
- Type:        int
- Default:     0

#### --swift-chunk-size

Above this size files will be chunked.

Above this size files will be chunked into a a `_segments` container
or a `.file-segments` directory. (See the `use_segments_container` option
for more info). Default for this is 5 GiB which is its maximum value, which
means only files above this size will be chunked.

Rclone uploads chunked files as dynamic large objects (DLO).


Properties:

- Config:      chunk_size
- Env Var:     RCLONE_SWIFT_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     5Gi

#### --swift-no-chunk

Don't chunk files during streaming upload.

When doing streaming uploads (e.g. using `rcat` or `mount` with
`--vfs-cache-mode off`) setting this flag will cause the swift backend
to not upload chunked files.

This will limit the maximum streamed upload size to 5 GiB. This is
useful because non chunked files are easier to deal with and have an
MD5SUM.

Rclone will still chunk files bigger than `chunk_size` when doing
normal copy operations.

Properties:

- Config:      no_chunk
- Env Var:     RCLONE_SWIFT_NO_CHUNK
- Type:        bool
- Default:     false

#### --swift-no-large-objects

Disable support for static and dynamic large objects

Swift cannot transparently store files bigger than 5 GiB. There are
two schemes for chunking large files, static large objects (SLO) or
dynamic large objects (DLO), and the API does not allow rclone to
determine whether a file is a static or dynamic large object without
doing a HEAD on the object. Since these need to be treated
differently, this means rclone has to issue HEAD requests for objects
for example when reading checksums.

When `no_large_objects` is set, rclone will assume that there are no
static or dynamic large objects stored. This means it can stop doing
the extra HEAD calls which in turn increases performance greatly
especially when doing a swift to swift transfer with `--checksum` set.

Setting this option implies `no_chunk` and also that no files will be
uploaded in chunks, so files bigger than 5 GiB will just fail on
upload.

If you set this option and there **are** static or dynamic large objects,
then this will give incorrect hashes for them. Downloads will succeed,
but other operations such as Remove and Copy will fail.


Properties:

- Config:      no_large_objects
- Env Var:     RCLONE_SWIFT_NO_LARGE_OBJECTS
- Type:        bool
- Default:     false

#### --swift-use-segments-container

Choose destination for large object segments

Swift cannot transparently store files bigger than 5 GiB and rclone
will chunk files larger than `chunk_size` (default 5 GiB) in order to
upload them.

If this value is `true` the chunks will be stored in an additional
container named the same as the destination container but with
`_segments` appended. This means that there won't be any duplicated
data in the original container but having another container may not be
acceptable.

If this value is `false` the chunks will be stored in a
`.file-segments` directory in the root of the container. This
directory will be omitted when listing the container. Some
providers (eg Blomp) require this mode as creating additional
containers isn't allowed. If it is desired to see the `.file-segments`
directory in the root then this flag must be set to `true`.

If this value is `unset` (the default), then rclone will choose the value
to use. It will be `false` unless rclone detects any `auth_url`s that
it knows need it to be `true`. In this case you'll see a message in
the DEBUG log.


Properties:

- Config:      use_segments_container
- Env Var:     RCLONE_SWIFT_USE_SEGMENTS_CONTAINER
- Type:        Tristate
- Default:     unset

#### --swift-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_SWIFT_ENCODING
- Type:        Encoding
- Default:     Slash,InvalidUtf8

#### --swift-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_SWIFT_DESCRIPTION
- Type:        string
- Required:    false

{{< rem autogenerated options stop >}}

## Limitations

The Swift API doesn't return a correct MD5SUM for segmented files
(Dynamic or Static Large Objects) so rclone won't check or use the
MD5SUM for these.

## Troubleshooting

### Rclone gives Failed to create file system for "remote:": Bad Request

Due to an oddity of the underlying swift library, it gives a "Bad
Request" error rather than a more sensible error when the
authentication fails for Swift.

So this most likely means your username / password is wrong.  You can
investigate further with the `--dump-bodies` flag.

This may also be caused by specifying the region when you shouldn't
have (e.g. OVH).

### Rclone gives Failed to create file system: Response didn't have storage url and auth token

This is most likely caused by forgetting to specify your tenant when
setting up a swift remote.

## OVH Cloud Archive

To use rclone with OVH cloud archive, first use `rclone config` to set up a `swift` backend with OVH, choosing `pca` as the `storage_policy`.

### Uploading Objects

Uploading objects to OVH cloud archive is no different to object storage, you just simply run the command you like (move, copy or sync) to upload the objects. Once uploaded the objects will show in a "Frozen" state within the OVH control panel.

### Retrieving Objects

To retrieve objects use `rclone copy` as normal. If the objects are in a frozen state then rclone will ask for them all to be unfrozen and it will wait at the end of the output with a message like the following:

`2019/03/23 13:06:33 NOTICE: Received retry after error - sleeping until 2019-03-23T13:16:33.481657164+01:00 (9m59.99985121s)`

Rclone will wait for the time specified then retry the copy.
