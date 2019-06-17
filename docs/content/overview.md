---
title: "Overview of cloud storage systems"
description: "Overview of cloud storage systems"
type: page
date: "2019-02-25"
---

# Overview of cloud storage systems #

Each cloud storage system is slightly different.  Rclone attempts to
provide a unified interface to them, but some underlying differences
show through.

## Features ##

Here is an overview of the major features of each cloud storage system.

| Name                         | Hash        | ModTime | Case Insensitive | Duplicate Files | MIME Type |
| ---------------------------- |:-----------:|:-------:|:----------------:|:---------------:|:---------:|
| 1Fichier                     | Whirlpool   | No      | No               | Yes             | R         |
| Amazon Drive                 | MD5         | No      | Yes              | No              | R         |
| Amazon S3                    | MD5         | Yes     | No               | No              | R/W       |
| Backblaze B2                 | SHA1        | Yes     | No               | No              | R/W       |
| Box                          | SHA1        | Yes     | Yes              | No              | -         |
| Dropbox                      | DBHASH †    | Yes     | Yes              | No              | -         |
| FTP                          | -           | No      | No               | No              | -         |
| Google Cloud Storage         | MD5         | Yes     | No               | No              | R/W       |
| Google Drive                 | MD5         | Yes     | No               | Yes             | R/W       |
| Google Photos                | -           | No      | No               | Yes             | R         |
| HTTP                         | -           | No      | No               | No              | R         |
| Hubic                        | MD5         | Yes     | No               | No              | R/W       |
| Jottacloud                   | MD5         | Yes     | Yes              | No              | R/W       |
| Koofr                        | MD5         | No      | Yes              | No              | -         |
| Mega                         | -           | No      | No               | Yes             | -         |
| Microsoft Azure Blob Storage | MD5         | Yes     | No               | No              | R/W       |
| Microsoft OneDrive           | SHA1 ‡‡     | Yes     | Yes              | No              | R         |
| OpenDrive                    | MD5         | Yes     | Yes              | No              | -         |
| Openstack Swift              | MD5         | Yes     | No               | No              | R/W       |
| pCloud                       | MD5, SHA1   | Yes     | No               | No              | W         |
| QingStor                     | MD5         | No      | No               | No              | R/W       |
| SFTP                         | MD5, SHA1 ‡ | Yes     | Depends          | No              | -         |
| WebDAV                       | MD5, SHA1 ††| Yes ††† | Depends          | No              | -         |
| Yandex Disk                  | MD5         | Yes     | No               | No              | R/W       |
| The local filesystem         | All         | Yes     | Depends          | No              | -         |

### Hash ###

The cloud storage system supports various hash types of the objects.
The hashes are used when transferring data as an integrity check and
can be specifically used with the `--checksum` flag in syncs and in
the `check` command.

To use the verify checksums when transferring between cloud storage
systems they must support a common hash type.

† Note that Dropbox supports [its own custom
hash](https://www.dropbox.com/developers/reference/content-hash).
This is an SHA256 sum of all the 4MB block SHA256s.

‡ SFTP supports checksums if the same login has shell access and `md5sum`
or `sha1sum` as well as `echo` are in the remote's PATH.

†† WebDAV supports hashes when used with Owncloud and Nextcloud only.

††† WebDAV supports modtimes when used with Owncloud and Nextcloud only.

‡‡ Microsoft OneDrive Personal supports SHA1 hashes, whereas OneDrive
for business and SharePoint server support Microsoft's own
[QuickXorHash](https://docs.microsoft.com/en-us/onedrive/developer/code-snippets/quickxorhash).

### ModTime ###

The cloud storage system supports setting modification times on
objects.  If it does then this enables a using the modification times
as part of the sync.  If not then only the size will be checked by
default, though the MD5SUM can be checked with the `--checksum` flag.

All cloud storage systems support some kind of date on the object and
these will be set when transferring from the cloud storage system.

### Case Insensitive ###

If a cloud storage systems is case sensitive then it is possible to
have two files which differ only in case, eg `file.txt` and
`FILE.txt`.  If a cloud storage system is case insensitive then that
isn't possible.

This can cause problems when syncing between a case insensitive
system and a case sensitive system.  The symptom of this is that no
matter how many times you run the sync it never completes fully.

The local filesystem and SFTP may or may not be case sensitive
depending on OS.

  * Windows - usually case insensitive, though case is preserved
  * OSX - usually case insensitive, though it is possible to format case sensitive
  * Linux - usually case sensitive, but there are case insensitive file systems (eg FAT formatted USB keys)

Most of the time this doesn't cause any problems as people tend to
avoid files whose name differs only by case even on case sensitive
systems.

### Duplicate files ###

If a cloud storage system allows duplicate files then it can have two
objects with the same name.

This confuses rclone greatly when syncing - use the `rclone dedupe`
command to rename or remove duplicates.

### MIME Type ###

MIME types (also known as media types) classify types of documents
using a simple text classification, eg `text/html` or
`application/pdf`.

Some cloud storage systems support reading (`R`) the MIME type of
objects and some support writing (`W`) the MIME type of objects.

The MIME type can be important if you are serving files directly to
HTTP from the storage system.

If you are copying from a remote which supports reading (`R`) to a
remote which supports writing (`W`) then rclone will preserve the MIME
types.  Otherwise they will be guessed from the extension, or the
remote itself may assign the MIME type.

## Optional Features ##

All the remotes support a basic set of features, but there are some
optional features supported by some remotes used to make some
operations more efficient.

| Name                         | Purge | Copy | Move | DirMove | CleanUp | ListR | StreamUpload | LinkSharing | About |
| ---------------------------- |:-----:|:----:|:----:|:-------:|:-------:|:-----:|:------------:|:------------:|:-----:|
| 1Fichier                     | No    | No   | No   | No      | No      | No    | No           | No           |   No  |
| Amazon Drive                 | Yes   | No   | Yes  | Yes     | No [#575](https://github.com/ncw/rclone/issues/575) | No  | No  | No [#2178](https://github.com/ncw/rclone/issues/2178) | No  |
| Amazon S3                    | No    | Yes  | No   | No      | No      | Yes   | Yes          | No [#2178](https://github.com/ncw/rclone/issues/2178) | No  |
| Backblaze B2                 | No    | Yes  | No   | No      | Yes     | Yes   | Yes          | No [#2178](https://github.com/ncw/rclone/issues/2178) | No  |
| Box                          | Yes   | Yes  | Yes  | Yes     | No [#575](https://github.com/ncw/rclone/issues/575) | No  | Yes | Yes | No  |
| Dropbox                      | Yes   | Yes  | Yes  | Yes     | No [#575](https://github.com/ncw/rclone/issues/575) | No  | Yes | Yes | Yes |
| FTP                          | No    | No   | Yes  | Yes     | No      | No    | Yes          | No [#2178](https://github.com/ncw/rclone/issues/2178) | No  |
| Google Cloud Storage         | Yes   | Yes  | No   | No      | No      | Yes   | Yes          | No [#2178](https://github.com/ncw/rclone/issues/2178) | No  |
| Google Drive                 | Yes   | Yes  | Yes  | Yes     | Yes     | Yes   | Yes          | Yes         | Yes |
| Google Photos                | No    | No   | No   | No      | No      | No    | No           | No          | No |
| HTTP                         | No    | No   | No   | No      | No      | No    | No           | No [#2178](https://github.com/ncw/rclone/issues/2178) | No  |
| Hubic                        | Yes † | Yes  | No   | No      | No      | Yes   | Yes          | No [#2178](https://github.com/ncw/rclone/issues/2178) | Yes |
| Jottacloud                   | Yes   | Yes  | Yes  | Yes     | No      | Yes   | No           | Yes                                                   | Yes |
| Mega                         | Yes   | No   | Yes  | Yes     | Yes     | No    | No           | No [#2178](https://github.com/ncw/rclone/issues/2178) | Yes |
| Microsoft Azure Blob Storage | Yes   | Yes  | No   | No      | No      | Yes   | No           | No [#2178](https://github.com/ncw/rclone/issues/2178) | No  |
| Microsoft OneDrive           | Yes   | Yes  | Yes  | Yes     | No [#575](https://github.com/ncw/rclone/issues/575) | No | No | Yes | Yes |
| OpenDrive                    | Yes   | Yes  | Yes  | Yes     | No      | No    | No           | No                                                    | No  |
| Openstack Swift              | Yes † | Yes  | No   | No      | No      | Yes   | Yes          | No [#2178](https://github.com/ncw/rclone/issues/2178) | Yes |
| pCloud                       | Yes   | Yes  | Yes  | Yes     | Yes     | No    | No           | No [#2178](https://github.com/ncw/rclone/issues/2178) | Yes |
| QingStor                     | No    | Yes  | No   | No      | No      | Yes   | No           | No [#2178](https://github.com/ncw/rclone/issues/2178) | No  |
| SFTP                         | No    | No   | Yes  | Yes     | No      | No    | Yes          | No [#2178](https://github.com/ncw/rclone/issues/2178) | Yes  |
| WebDAV                       | Yes   | Yes  | Yes  | Yes     | No      | No    | Yes ‡        | No [#2178](https://github.com/ncw/rclone/issues/2178) | Yes  |
| Yandex Disk                  | Yes   | Yes  | Yes  | Yes     | Yes     | No    | Yes          | Yes         | Yes |
| The local filesystem         | Yes   | No   | Yes  | Yes     | No      | No    | Yes          | No          | Yes |

### Purge ###

This deletes a directory quicker than just deleting all the files in
the directory.

† Note Swift and Hubic implement this in order to delete directory
markers but they don't actually have a quicker way of deleting files
other than deleting them individually.

‡ StreamUpload is not supported with Nextcloud

### Copy ###

Used when copying an object to and from the same remote.  This known
as a server side copy so you can copy a file without downloading it
and uploading it again.  It is used if you use `rclone copy` or
`rclone move` if the remote doesn't support `Move` directly.

If the server doesn't support `Copy` directly then for copy operations
the file is downloaded then re-uploaded.

### Move ###

Used when moving/renaming an object on the same remote.  This is known
as a server side move of a file.  This is used in `rclone move` if the
server doesn't support `DirMove`.

If the server isn't capable of `Move` then rclone simulates it with
`Copy` then delete.  If the server doesn't support `Copy` then rclone
will download the file and re-upload it.

### DirMove ###

This is used to implement `rclone move` to move a directory if
possible.  If it isn't then it will use `Move` on each file (which
falls back to `Copy` then download and upload - see `Move` section).

### CleanUp ###

This is used for emptying the trash for a remote by `rclone cleanup`.

If the server can't do `CleanUp` then `rclone cleanup` will return an
error.

### ListR ###

The remote supports a recursive list to list all the contents beneath
a directory quickly.  This enables the `--fast-list` flag to work.
See the [rclone docs](/docs/#fast-list) for more details.

### StreamUpload ###

Some remotes allow files to be uploaded without knowing the file size
in advance. This allows certain operations to work without spooling the
file to local disk first, e.g. `rclone rcat`.

### LinkSharing ###

Sets the necessary permissions on a file or folder and prints a link
that allows others to access them, even if they don't have an account
on the particular cloud provider.

### About ###

This is used to fetch quota information from the remote, like bytes
used/free/quota and bytes used in the trash.

This is also used to return the space used, available for `rclone mount`.

If the server can't do `About` then `rclone about` will return an
error.
