---
title: "Storj"
description: "Rclone docs for Storj"
versionIntroduced: "v1.52"
---

# {{< icon "fas fa-dove" >}} Storj

[Storj](https://storj.io) is an encrypted, secure, and 
cost-effective object storage service that enables you to store, back up, and 
archive large amounts of data in a decentralized manner.

## Backend options

Storj can be used both with this native backend and with the [s3
backend using the Storj S3 compatible gateway](/s3/#storj) (shared or private).

Use this backend to take advantage of client-side encryption as well
as to achieve the best possible download performance. Uploads will be
erasure-coded locally, thus a 1gb upload will result in 2.68gb of data
being uploaded to storage nodes across the network.

Use the s3 backend and one of the S3 compatible Hosted Gateways to
increase upload performance and reduce the load on your systems and
network. Uploads will be encrypted and erasure-coded server-side, thus
a 1GB upload will result in only in 1GB of data being uploaded to
storage nodes across the network.

Side by side comparison with more details:

* Characteristics:
  * *Storj backend*: Uses native RPC protocol, connects directly
    to the storage nodes which hosts the data. Requires more CPU
    resource of encoding/decoding and has network amplification
    (especially during the upload), uses lots of TCP connections
  * *S3 backend*: Uses S3 compatible HTTP Rest API via the shared
    gateways. There is no network amplification, but performance
    depends on the shared gateways and the secret encryption key is
    shared with the gateway.
* Typical usage:
  * *Storj backend*: Server environments and desktops with enough
    resources, internet speed and connectivity - and applications
    where storjs client-side encryption is required.
  * *S3 backend*: Desktops and similar with limited resources,
    internet speed or connectivity.
* Security:
  * *Storj backend*: __strong__. Private encryption key doesn't
    need to leave the local computer.
  * *S3 backend*: __weaker__. Private encryption key is [shared
    with](https://docs.storj.io/dcs/api-reference/s3-compatible-gateway#security-and-encryption)
    the authentication service of the hosted gateway, where it's
    stored encrypted. It can be stronger when combining with the
    rclone [crypt](/crypt) backend.
* Bandwidth usage (upload):
  * *Storj backend*: __higher__. As data is erasure coded on the
    client side both the original data and the parities should be
    uploaded. About ~2.7 times more data is required to be uploaded.
    Client may start to upload with even higher number of nodes (~3.7
    times more) and abandon/stop the slow uploads.
  * *S3 backend*: __normal__. Only the raw data is uploaded, erasure
    coding happens on the gateway.
* Bandwidth usage (download)
  * *Storj backend*: __almost normal__. Only the minimal number
    of data is required, but to avoid very slow data providers a few
    more sources are used and the slowest are ignored (max 1.2x
    overhead).
  * *S3 backend*: __normal__. Only the raw data is downloaded, erasure coding happens on the shared gateway.
* CPU usage:
  * *Storj backend*: __higher__, but more predictable. Erasure
    code and encryption/decryption happens locally which requires
    significant CPU usage.
  * *S3 backend*: __less__. Erasure code and encryption/decryption
    happens on shared s3 gateways (and as is, it depends on the
    current load on the gateways)
* TCP connection usage:
  * *Storj backend*: __high__. A direct connection is required to
    each of the Storj nodes resulting in 110 connections on upload and
    35 on download per 64 MB segment. Not all the connections are
    actively used (slow ones are pruned), but they are all opened.
    [Adjusting the max open file limit](/storj/#known-issues) may
    be required.
  * *S3 backend*: __normal__. Only one connection per download/upload
    thread is required to the shared gateway.
* Overall performance:
  * *Storj backend*: with enough resources (CPU and bandwidth)
    *storj* backend can provide even 2x better performance. Data
    is directly downloaded to / uploaded from to the client instead of
    the gateway.
  * *S3 backend*: Can be faster on edge devices where CPU and network
    bandwidth is limited as the shared S3 compatible gateways take
    care about the encrypting/decryption and erasure coding and no
    download/upload amplification.
* Decentralization:
  * *Storj backend*: __high__. Data is downloaded directly from
    the distributed cloud of storage providers.
  * *S3 backend*: __low__. Requires a running S3 gateway (either
    self-hosted or Storj-hosted).
* Limitations:
  * *Storj backend*: `rclone checksum` is not possible without
    download, as checksum metadata is not calculated during upload
  * *S3 backend*: secret encryption key is shared with the gateway

## Configuration

To make a new Storj configuration you need one of the following:
* Access Grant that someone else shared with you.
* [API Key](https://documentation.storj.io/getting-started/uploading-your-first-object/create-an-api-key)
of a Storj project you are a member of.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

### Setup with access grant

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Storj Decentralized Cloud Storage
   \ "storj"
[snip]
Storage> storj
** See help for storj backend at: https://rclone.org/storj/ **

Choose an authentication method.
Enter a string value. Press Enter for the default ("existing").
Choose a number from below, or type in your own value
 1 / Use an existing access grant.
   \ "existing"
 2 / Create a new access grant from satellite address, API key, and passphrase.
   \ "new"
provider> existing
Access Grant.
Enter a string value. Press Enter for the default ("").
access_grant> your-access-grant-received-by-someone-else
Remote config
--------------------
[remote]
type = storj
access_grant = your-access-grant-received-by-someone-else
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Setup with API key and passphrase

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Storj Decentralized Cloud Storage
   \ "storj"
[snip]
Storage> storj
** See help for storj backend at: https://rclone.org/storj/ **

Choose an authentication method.
Enter a string value. Press Enter for the default ("existing").
Choose a number from below, or type in your own value
 1 / Use an existing access grant.
   \ "existing"
 2 / Create a new access grant from satellite address, API key, and passphrase.
   \ "new"
provider> new
Satellite Address. Custom satellite address should match the format: `<nodeid>@<address>:<port>`.
Enter a string value. Press Enter for the default ("us1.storj.io").
Choose a number from below, or type in your own value
 1 / US1
   \ "us1.storj.io"
 2 / EU1
   \ "eu1.storj.io"
 3 / AP1
   \ "ap1.storj.io"
satellite_address> 1
API Key.
Enter a string value. Press Enter for the default ("").
api_key> your-api-key-for-your-storj-project
Encryption Passphrase. To access existing objects enter passphrase used for uploading.
Enter a string value. Press Enter for the default ("").
passphrase> your-human-readable-encryption-passphrase
Remote config
--------------------
[remote]
type = storj
satellite_address = 12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us1.storj.io:7777
api_key = your-api-key-for-your-storj-project
passphrase = your-human-readable-encryption-passphrase
access_grant = the-access-grant-generated-from-the-api-key-and-passphrase
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/storj/storj.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to storj (Storj Decentralized Cloud Storage).

#### --storj-provider

Choose an authentication method.

Properties:

- Config:      provider
- Env Var:     RCLONE_STORJ_PROVIDER
- Type:        string
- Default:     "existing"
- Examples:
    - "existing"
        - Use an existing access grant.
    - "new"
        - Create a new access grant from satellite address, API key, and passphrase.

#### --storj-access-grant

Access grant.

Properties:

- Config:      access_grant
- Env Var:     RCLONE_STORJ_ACCESS_GRANT
- Provider:    existing
- Type:        string
- Required:    false

#### --storj-satellite-address

Satellite address.

Custom satellite address should match the format: `<nodeid>@<address>:<port>`.

Properties:

- Config:      satellite_address
- Env Var:     RCLONE_STORJ_SATELLITE_ADDRESS
- Provider:    new
- Type:        string
- Default:     "us1.storj.io"
- Examples:
    - "us1.storj.io"
        - US1
    - "eu1.storj.io"
        - EU1
    - "ap1.storj.io"
        - AP1

#### --storj-api-key

API key.

Properties:

- Config:      api_key
- Env Var:     RCLONE_STORJ_API_KEY
- Provider:    new
- Type:        string
- Required:    false

#### --storj-passphrase

Encryption passphrase.

To access existing objects enter passphrase used for uploading.

Properties:

- Config:      passphrase
- Env Var:     RCLONE_STORJ_PASSPHRASE
- Provider:    new
- Type:        string
- Required:    false

{{< rem autogenerated options stop >}}

## Usage

Paths are specified as `remote:bucket` (or `remote:` for the `lsf`
command.)  You may put subdirectories in too, e.g. `remote:bucket/path/to/dir`.

Once configured you can then use `rclone` like this.

### Create a new bucket

Use the `mkdir` command to create new bucket, e.g. `bucket`.

    rclone mkdir remote:bucket

### List all buckets

Use the `lsf` command to list all buckets.

    rclone lsf remote:

Note the colon (`:`) character at the end of the command line.

### Delete a bucket

Use the `rmdir` command to delete an empty bucket.

    rclone rmdir remote:bucket

Use the `purge` command to delete a non-empty bucket with all its content.

    rclone purge remote:bucket

### Upload objects

Use the `copy` command to upload an object.

    rclone copy --progress /home/local/directory/file.ext remote:bucket/path/to/dir/

The `--progress` flag is for displaying progress information.
Remove it if you don't need this information.

Use a folder in the local path to upload all its objects.

    rclone copy --progress /home/local/directory/ remote:bucket/path/to/dir/

Only modified files will be copied.

### List objects

Use the `ls` command to list recursively all objects in a bucket.

    rclone ls remote:bucket

Add the folder to the remote path to list recursively all objects in this folder.

    rclone ls remote:bucket/path/to/dir/

Use the `lsf` command to list non-recursively all objects in a bucket or a folder.

    rclone lsf remote:bucket/path/to/dir/

### Download objects

Use the `copy` command to download an object.

    rclone copy --progress remote:bucket/path/to/dir/file.ext /home/local/directory/

The `--progress` flag is for displaying progress information.
Remove it if you don't need this information.

Use a folder in the remote path to download all its objects.

    rclone copy --progress remote:bucket/path/to/dir/ /home/local/directory/

### Delete objects

Use the `deletefile` command to delete a single object.

    rclone deletefile remote:bucket/path/to/dir/file.ext

Use the `delete` command to delete all object in a folder.

    rclone delete remote:bucket/path/to/dir/

### Print the total size of objects

Use the `size` command to print the total size of objects in a bucket or a folder.

    rclone size remote:bucket/path/to/dir/

### Sync two Locations

Use the `sync` command to sync the source to the destination,
changing the destination only, deleting any excess files.

    rclone sync --interactive --progress /home/local/directory/ remote:bucket/path/to/dir/

The `--progress` flag is for displaying progress information.
Remove it if you don't need this information.

Since this can cause data loss, test first with the `--dry-run` flag
to see exactly what would be copied and deleted.

The sync can be done also from Storj to the local file system.

    rclone sync --interactive --progress remote:bucket/path/to/dir/ /home/local/directory/

Or between two Storj buckets.

    rclone sync --interactive --progress remote-us:bucket/path/to/dir/ remote-europe:bucket/path/to/dir/

Or even between another cloud storage and Storj.

    rclone sync --interactive --progress s3:bucket/path/to/dir/ storj:bucket/path/to/dir/

## Limitations

`rclone about` is not supported by the rclone Storj backend. Backends without
this capability cannot determine free space for an rclone mount or
use policy `mfs` (most free space) as a member of an rclone union
remote.

See [List of backends that do not support rclone about](https://rclone.org/overview/#optional-features) and [rclone about](https://rclone.org/commands/rclone_about/)

## Known issues

If you get errors like `too many open files` this usually happens when the default `ulimit` for system max open files is exceeded. Native Storj protocol opens a large number of TCP connections (each of which is counted as an open file). For a single upload stream you can expect 110 TCP connections to be opened. For a single download stream you can expect 35. This batch of connections will be opened for every 64 MiB segment and you should also expect TCP connections to be reused. If you do many transfers you eventually open a connection to most storage nodes (thousands of nodes).

To fix these, please raise your system limits. You can do this issuing a `ulimit -n 65536` just before you run rclone. To change the limits more permanently you can add this to your shell startup script, e.g. `$HOME/.bashrc`, or change the system-wide configuration, usually `/etc/sysctl.conf` and/or `/etc/security/limits.conf`, but please refer to your operating system manual.
