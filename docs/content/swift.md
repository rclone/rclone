---
title: "Swift"
description: "Swift"
date: "2014-04-26"
---

<i class="fa fa-space-shuttle"></i>Swift
----------------------------------------

Swift refers to [Openstack Object Storage](https://docs.openstack.org/swift/latest/).
Commercial implementations of that being:

  * [Rackspace Cloud Files](https://www.rackspace.com/cloud/files/)
  * [Memset Memstore](https://www.memset.com/cloud/storage/)

Paths are specified as `remote:container` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, eg `remote:container/path/to/dir`.

Here is an example of making a swift configuration.  First run

    rclone config

This will guide you through an interactive setup process.

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
 1 / Amazon Drive
   \ "amazon cloud drive"
 2 / Amazon S3 (also Dreamhost, Ceph, Minio)
   \ "s3"
 3 / Backblaze B2
   \ "b2"
 4 / Box
   \ "box"
 5 / Dropbox
   \ "dropbox"
 6 / Encrypt/Decrypt a remote
   \ "crypt"
 7 / FTP Connection
   \ "ftp"
 8 / Google Cloud Storage (this is not Google Drive)
   \ "google cloud storage"
 9 / Google Drive
   \ "drive"
10 / Hubic
   \ "hubic"
11 / Local Disk
   \ "local"
12 / Microsoft Azure Blob Storage
   \ "azureblob"
13 / Microsoft OneDrive
   \ "onedrive"
14 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
   \ "swift"
15 / QingClound Object Storage
   \ "qingstor"
16 / SSH/SFTP Connection
   \ "sftp"
17 / Yandex Disk
   \ "yandex"
18 / http Connection
   \ "http"
Storage> swift
Get swift credentials from environment variables in standard OpenStack form.
Choose a number from below, or type in your own value
 1 / Enter swift credentials in the next step
   \ "false"
 2 / Get swift credentials from environment vars. Leave other fields blank if using this.
   \ "true"
env_auth> 1
User name to log in.
user> user_name
API key or password.
key> password_or_api_key
Authentication URL for server.
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
   \ "https://auth.cloud.ovh.net/v2.0"
auth> 1
User domain - optional (v3 auth)
domain> Default
Tenant name - optional for v1 auth, required otherwise
tenant> tenant_name
Tenant domain - optional (v3 auth)
tenant_domain>
Region name - optional
region>
Storage URL - optional
storage_url>
AuthVersion - optional - set to (1,2,3) if your auth URL has no version
auth_version>
Endpoint type to choose from the service catalogue
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
[remote]
env_auth = false
user = user_name
key = password_or_api_key
auth = https://auth.api.rackspacecloud.com/v1.0
domain = Default
tenant =
tenant_domain =
region =
storage_url =
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

    rclone sync /home/local/directory remote:container

### Configuration from an Openstack credentials file ###

An Opentstack credentials file typically looks something something
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

### Configuration from the environment ###

If you prefer you can configure rclone to use swift using a standard
set of OpenStack environment variables.

When you run through the config, make sure you choose `true` for
`env_auth` and leave everything else blank.

rclone will then set any empty config parameters from the enviroment
using standard OpenStack environment variables.  There is [a list of
the
variables](https://godoc.org/github.com/ncw/swift#Connection.ApplyEnvironment)
in the docs for the swift library.

#### Using rclone without a config file ####

You can use rclone with swift without a config file, if desired, like
this:

```
source openstack-credentials-file
export RCLONE_CONFIG_MYREMOTE_TYPE=swift
export RCLONE_CONFIG_MYREMOTE_ENV_AUTH=true
rclone lsd myremote:
```

### --fast-list ###

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](/docs/#fast-list) for more details.

### Specific options ###

Here are the command line options specific to this cloud storage
system.

#### --swift-chunk-size=SIZE ####

Above this size files will be chunked into a _segments container.  The
default for this is 5GB which is its maximum value.

### Modified time ###

The modified time is stored as metadata on the object as
`X-Object-Meta-Mtime` as floating point since the epoch accurate to 1
ns.

This is a defacto standard (used in the official python-swiftclient
amongst others) for storing the modification time for an object.

### Limitations ###

The Swift API doesn't return a correct MD5SUM for segmented files
(Dynamic or Static Large Objects) so rclone won't check or use the
MD5SUM for these.

### Troubleshooting ###

#### Rclone gives Failed to create file system for "remote:": Bad Request ####

Due to an oddity of the underlying swift library, it gives a "Bad
Request" error rather than a more sensible error when the
authentication fails for Swift.

So this most likely means your username / password is wrong.  You can
investigate further with the `--dump-bodies` flag.

This may also be caused by specifying the region when you shouldn't
have (eg OVH).

#### Rclone gives Failed to create file system: Response didn't have storage storage url and auth token ####

This is most likely caused by forgetting to specify your tenant when
setting up a swift remote.
