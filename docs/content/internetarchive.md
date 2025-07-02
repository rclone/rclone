---
title: "Internet Archive"
description: "Rclone docs for Internet Archive"
versionIntroduced: "v1.59"
---

# {{< icon "fa fa-archive" >}} Internet Archive

The Internet Archive backend utilizes Items on [archive.org](https://archive.org/)

Refer to [IAS3 API documentation](https://archive.org/services/docs/api/ias3.html) for the API this backend uses.

Paths are specified as `remote:bucket` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, e.g. `remote:item/path/to/dir`.

Unlike S3, listing up all items uploaded by you isn't supported.

Once you have made a remote, you can use it like this:

Make a new item

    rclone mkdir remote:item

List the contents of a item

    rclone ls remote:item

Sync `/home/local/directory` to the remote item, deleting any excess
files in the item.

    rclone sync --interactive /home/local/directory remote:item

## Notes
Because of Internet Archive's architecture, it enqueues write operations (and extra post-processings) in a per-item queue. You can check item's queue at https://catalogd.archive.org/history/item-name-here . Because of that, all uploads/deletes will not show up immediately and takes some time to be available.
The per-item queue is enqueued to an another queue, Item Deriver Queue. [You can check the status of Item Deriver Queue here.](https://catalogd.archive.org/catalog.php?whereami=1) This queue has a limit, and it may block you from uploading, or even deleting. You should avoid uploading a lot of small files for better behavior.

You can optionally wait for the server's processing to finish, by setting non-zero value to `wait_archive` key.
By making it wait, rclone can do normal file comparison.
Make sure to set a large enough value (e.g. `30m0s` for smaller files) as it can take a long time depending on server's queue.

## About metadata
This backend supports setting, updating and reading metadata of each file.
The metadata will appear as file metadata on Internet Archive.
However, some fields are reserved by both Internet Archive and rclone.

The following are reserved by Internet Archive:
- `name`
- `source`
- `size`
- `md5`
- `crc32`
- `sha1`
- `format`
- `old_version`
- `viruscheck`
- `summation`

Trying to set values to these keys is ignored with a warning.
Only setting `mtime` is an exception. Doing so make it the identical behavior as setting ModTime.

rclone reserves all the keys starting with `rclone-`. Setting value for these keys will give you warnings, but values are set according to request.

If there are multiple values for a key, only the first one is returned.
This is a limitation of rclone, that supports one value per one key.
It can be triggered when you did a server-side copy.

Reading metadata will also provide custom (non-standard nor reserved) ones.

## Filtering auto generated files

The Internet Archive automatically creates metadata files after
upload. These can cause problems when doing an `rclone sync` as rclone
will try, and fail, to delete them. These metadata files are not
changeable, as they are created by the Internet Archive automatically.

These auto-created files can be excluded from the sync using [metadata
filtering](/filtering/#metadata).

    rclone sync ... --metadata-exclude "source=metadata" --metadata-exclude "format=Metadata"

Which excludes from the sync any files which have the
`source=metadata` or `format=Metadata` flags which are added to
Internet Archive auto-created files.

## Configuration

Here is an example of making an internetarchive configuration.
Most applies to the other providers as well, any differences are described [below](#providers).

First run

    rclone config

This will guide you through an interactive setup process.

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
XX / InternetArchive Items
   \ (internetarchive)
Storage> internetarchive
Option access_key_id.
IAS3 Access Key.
Leave blank for anonymous access.
You can find one here: https://archive.org/account/s3.php
Enter a value. Press Enter to leave empty.
access_key_id> XXXX
Option secret_access_key.
IAS3 Secret Key (password).
Leave blank for anonymous access.
Enter a value. Press Enter to leave empty.
secret_access_key> XXXX
Edit advanced config?
y) Yes
n) No (default)
y/n> y
Option endpoint.
IAS3 Endpoint.
Leave blank for default value.
Enter a string value. Press Enter for the default (https://s3.us.archive.org).
endpoint> 
Option front_endpoint.
Host of InternetArchive Frontend.
Leave blank for default value.
Enter a string value. Press Enter for the default (https://archive.org).
front_endpoint> 
Option disable_checksum.
Don't store MD5 checksum with object metadata.
Normally rclone will calculate the MD5 checksum of the input before
uploading it so it can ask the server to check the object against checksum.
This is great for data integrity checking but can cause long delays for
large files to start uploading.
Enter a boolean value (true or false). Press Enter for the default (true).
disable_checksum> true
Option encoding.
The encoding for the backend.
See the [encoding section in the overview](/overview/#encoding) for more info.
Enter a encoder.MultiEncoder value. Press Enter for the default (Slash,Question,Hash,Percent,Del,Ctl,InvalidUtf8,Dot).
encoding> 
Edit advanced config?
y) Yes
n) No (default)
y/n> n
Configuration complete.
Options:
- type: internetarchive
- access_key_id: XXXX
- secret_access_key: XXXX
Keep this "remote" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/internetarchive/internetarchive.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to internetarchive (Internet Archive).

#### --internetarchive-access-key-id

IAS3 Access Key.

Leave blank for anonymous access.
You can find one here: https://archive.org/account/s3.php

Properties:

- Config:      access_key_id
- Env Var:     RCLONE_INTERNETARCHIVE_ACCESS_KEY_ID
- Type:        string
- Required:    false

#### --internetarchive-secret-access-key

IAS3 Secret Key (password).

Leave blank for anonymous access.

Properties:

- Config:      secret_access_key
- Env Var:     RCLONE_INTERNETARCHIVE_SECRET_ACCESS_KEY
- Type:        string
- Required:    false

#### --internetarchive-item-derive

Whether to trigger derive on the IA item or not. If set to false, the item will not be derived by IA upon upload.
The derive process produces a number of secondary files from an upload to make an upload more usable on the web.
Setting this to false is useful for uploading files that are already in a format that IA can display or reduce burden on IA's infrastructure.

Properties:

- Config:      item_derive
- Env Var:     RCLONE_INTERNETARCHIVE_ITEM_DERIVE
- Type:        bool
- Default:     true

### Advanced options

Here are the Advanced options specific to internetarchive (Internet Archive).

#### --internetarchive-endpoint

IAS3 Endpoint.

Leave blank for default value.

Properties:

- Config:      endpoint
- Env Var:     RCLONE_INTERNETARCHIVE_ENDPOINT
- Type:        string
- Default:     "https://s3.us.archive.org"

#### --internetarchive-front-endpoint

Host of InternetArchive Frontend.

Leave blank for default value.

Properties:

- Config:      front_endpoint
- Env Var:     RCLONE_INTERNETARCHIVE_FRONT_ENDPOINT
- Type:        string
- Default:     "https://archive.org"

#### --internetarchive-item-metadata

Metadata to be set on the IA item, this is different from file-level metadata that can be set using --metadata-set.
Format is key=value and the 'x-archive-meta-' prefix is automatically added.

Properties:

- Config:      item_metadata
- Env Var:     RCLONE_INTERNETARCHIVE_ITEM_METADATA
- Type:        stringArray
- Default:     []

#### --internetarchive-disable-checksum

Don't ask the server to test against MD5 checksum calculated by rclone.
Normally rclone will calculate the MD5 checksum of the input before
uploading it so it can ask the server to check the object against checksum.
This is great for data integrity checking but can cause long delays for
large files to start uploading.

Properties:

- Config:      disable_checksum
- Env Var:     RCLONE_INTERNETARCHIVE_DISABLE_CHECKSUM
- Type:        bool
- Default:     true

#### --internetarchive-wait-archive

Timeout for waiting the server's processing tasks (specifically archive and book_op) to finish.
Only enable if you need to be guaranteed to be reflected after write operations.
0 to disable waiting. No errors to be thrown in case of timeout.

Properties:

- Config:      wait_archive
- Env Var:     RCLONE_INTERNETARCHIVE_WAIT_ARCHIVE
- Type:        Duration
- Default:     0s

#### --internetarchive-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_INTERNETARCHIVE_ENCODING
- Type:        Encoding
- Default:     Slash,LtGt,CrLf,Del,Ctl,InvalidUtf8,Dot

#### --internetarchive-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_INTERNETARCHIVE_DESCRIPTION
- Type:        string
- Required:    false

### Metadata

Metadata fields provided by Internet Archive.
If there are multiple values for a key, only the first one is returned.
This is a limitation of Rclone, that supports one value per one key.

Owner is able to add custom keys. Metadata feature grabs all the keys including them.

Here are the possible system metadata items for the internetarchive backend.

| Name | Help | Type | Example | Read Only |
|------|------|------|---------|-----------|
| crc32 | CRC32 calculated by Internet Archive | string | 01234567 | **Y** |
| format | Name of format identified by Internet Archive | string | Comma-Separated Values | **Y** |
| md5 | MD5 hash calculated by Internet Archive | string | 01234567012345670123456701234567 | **Y** |
| mtime | Time of last modification, managed by Rclone | RFC 3339 | 2006-01-02T15:04:05.999999999Z | **Y** |
| name | Full file path, without the bucket part | filename | backend/internetarchive/internetarchive.go | **Y** |
| old_version | Whether the file was replaced and moved by keep-old-version flag | boolean | true | **Y** |
| rclone-ia-mtime | Time of last modification, managed by Internet Archive | RFC 3339 | 2006-01-02T15:04:05.999999999Z | N |
| rclone-mtime | Time of last modification, managed by Rclone | RFC 3339 | 2006-01-02T15:04:05.999999999Z | N |
| rclone-update-track | Random value used by Rclone for tracking changes inside Internet Archive | string | aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa | N |
| sha1 | SHA1 hash calculated by Internet Archive | string | 0123456701234567012345670123456701234567 | **Y** |
| size | File size in bytes | decimal number | 123456 | **Y** |
| source | The source of the file | string | original | **Y** |
| summation | Check https://forum.rclone.org/t/31922 for how it is used | string | md5 | **Y** |
| viruscheck | The last time viruscheck process was run for the file (?) | unixtime | 1654191352 | **Y** |

See the [metadata](/docs/#metadata) docs for more info.

{{< rem autogenerated options stop >}}
