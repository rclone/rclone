---
title: "Tardigrade"
description: "Rclone docs for Tardigrade"
---

{{< icon "fas fa-dove" >}} Tardigrade
-----------------------------------------

[Tardigrade](https://tardigrade.io) is an encrypted, secure, and 
cost-effective object storage service that enables you to store, back up, and 
archive large amounts of data in a decentralized manner.

## Setup

To make a new Tardigrade configuration you need one of the following:
* Access Grant that someone else shared with you.
* [API Key](https://documentation.tardigrade.io/getting-started/uploading-your-first-object/create-an-api-key)
of a Tardigrade project you are a member of.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

### Setup with access grant

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Tardigrade Decentralized Cloud Storage
   \ "tardigrade"
[snip]
Storage> tardigrade
** See help for tardigrade backend at: https://rclone.org/tardigrade/ **

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
type = tardigrade
access_grant = your-access-grant-received-by-someone-else
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Setup with API key and passphrase

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Tardigrade Decentralized Cloud Storage
   \ "tardigrade"
[snip]
Storage> tardigrade
** See help for tardigrade backend at: https://rclone.org/tardigrade/ **

Choose an authentication method.
Enter a string value. Press Enter for the default ("existing").
Choose a number from below, or type in your own value
 1 / Use an existing access grant.
   \ "existing"
 2 / Create a new access grant from satellite address, API key, and passphrase.
   \ "new"
provider> new
Satellite Address. Custom satellite address should match the format: `<nodeid>@<address>:<port>`.
Enter a string value. Press Enter for the default ("us-central-1.tardigrade.io").
Choose a number from below, or type in your own value
 1 / US Central 1
   \ "us-central-1.tardigrade.io"
 2 / Europe West 1
   \ "europe-west-1.tardigrade.io"
 3 / Asia East 1
   \ "asia-east-1.tardigrade.io"
satellite_address> 1
API Key.
Enter a string value. Press Enter for the default ("").
api_key> your-api-key-for-your-tardigrade-project
Encryption Passphrase. To access existing objects enter passphrase used for uploading.
Enter a string value. Press Enter for the default ("").
passphrase> your-human-readable-encryption-passphrase
Remote config
--------------------
[remote]
type = tardigrade
satellite_address = 12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777
api_key = your-api-key-for-your-tardigrade-project
passphrase = your-human-readable-encryption-passphrase
access_grant = the-access-grant-generated-from-the-api-key-and-passphrase
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

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

    rclone sync -i --progress /home/local/directory/ remote:bucket/path/to/dir/

The `--progress` flag is for displaying progress information.
Remove it if you don't need this information.

Since this can cause data loss, test first with the `--dry-run` flag
to see exactly what would be copied and deleted.

The sync can be done also from Tardigrade to the local file system.

    rclone sync -i --progress remote:bucket/path/to/dir/ /home/local/directory/

Or between two Tardigrade buckets.

    rclone sync -i --progress remote-us:bucket/path/to/dir/ remote-europe:bucket/path/to/dir/

Or even between another cloud storage and Tardigrade.

    rclone sync -i --progress s3:bucket/path/to/dir/ tardigrade:bucket/path/to/dir/

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/tardigrade/tardigrade.go then run make backenddocs" >}}
### Standard Options

Here are the standard options specific to tardigrade (Tardigrade Decentralized Cloud Storage).

#### --tardigrade-provider

Choose an authentication method.

- Config:      provider
- Env Var:     RCLONE_TARDIGRADE_PROVIDER
- Type:        string
- Default:     "existing"
- Examples:
    - "existing"
        - Use an existing access grant.
    - "new"
        - Create a new access grant from satellite address, API key, and passphrase.

#### --tardigrade-access-grant

Access Grant.

- Config:      access_grant
- Env Var:     RCLONE_TARDIGRADE_ACCESS_GRANT
- Type:        string
- Default:     ""

#### --tardigrade-satellite-address

Satellite Address. Custom satellite address should match the format: `<nodeid>@<address>:<port>`.

- Config:      satellite_address
- Env Var:     RCLONE_TARDIGRADE_SATELLITE_ADDRESS
- Type:        string
- Default:     "us-central-1.tardigrade.io"
- Examples:
    - "us-central-1.tardigrade.io"
        - US Central 1
    - "europe-west-1.tardigrade.io"
        - Europe West 1
    - "asia-east-1.tardigrade.io"
        - Asia East 1

#### --tardigrade-api-key

API Key.

- Config:      api_key
- Env Var:     RCLONE_TARDIGRADE_API_KEY
- Type:        string
- Default:     ""

#### --tardigrade-passphrase

Encryption Passphrase. To access existing objects enter passphrase used for uploading.

- Config:      passphrase
- Env Var:     RCLONE_TARDIGRADE_PASSPHRASE
- Type:        string
- Default:     ""

{{< rem autogenerated options stop >}}
### Limitations

`rclone about` is not supported by the rclone Tardigrade backend. Backends without
this capability cannot determine free space for an rclone mount or
use policy `mfs` (most free space) as a member of an rclone union
remote.

See [List of backends that do not support rclone about](https://rclone.org/overview/#optional-features)
See [rclone about](https://rclone.org/commands/rclone_about/)

### Known issues

If you get errors like `too many open files` this usually happens when the default `ulimit` for system max open files is exceeded. Native Storj protocol opens a large number of TCP connections (each of which is counted as an open file). For a single upload stream you can expect 110 TCP connections to be opened. For a single download stream you can expect 35. This batch of connections will be opened for every 64 MiB segment and you should also expect TCP connections to be reused. If you do many transfers you eventually open a connection to most storage nodes (thousands of nodes).

To fix these, please raise your system limits. You can do this issuing a `ulimit -n 65536` just before you run rclone, inside a bash script or inside the `$USER/.bashrc` file. If you need to change limits system-wide, this usually takes place at `/etc/sysctl.conf` and/or `/etc/security/limits.conf` on Linux, but please refer to your operating system manual.
