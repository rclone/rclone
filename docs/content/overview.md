---
title: "Overview of cloud storage systems"
description: "Overview of cloud storage systems"
type: page
date: "2015-09-06"
---

# Overview of cloud storage systems #

Each cloud storage system is slighly different.  Rclone attempts to
provide a unified interface to them, but some underlying differences
show through.

## Features ##

Here is an overview of the major features of each cloud storage system.

| Name                   | Hash    | ModTime | Case Insensitive | Duplicate Files | MIME Type |
| ---------------------- |:-------:|:-------:|:----------------:|:---------------:|:---------:|
| Google Drive           | MD5     | Yes     | No               | Yes             | R/W       |
| Amazon S3              | MD5     | Yes     | No               | No              | R/W       |
| Openstack Swift        | MD5     | Yes     | No               | No              | R/W       |
| Dropbox                | -       | No      | Yes              | No              | R         |
| Google Cloud Storage   | MD5     | Yes     | No               | No              | R/W       |
| Amazon Drive           | MD5     | No      | Yes              | No              | R         |
| Microsoft One Drive    | SHA1    | Yes     | Yes              | No              | R         |
| Hubic                  | MD5     | Yes     | No               | No              | R/W       |
| Backblaze B2           | SHA1    | Yes     | No               | No              | R/W       |
| Yandex Disk            | MD5     | Yes     | No               | No              | R/W       |
| The local filesystem   | All     | Yes     | Depends          | No              | -         |

### Hash ###

The cloud storage system supports various hash types of the objects.  
The hashes are used when transferring data as an integrity check and
can be specifically used with the `--checksum` flag in syncs and in
the `check` command.

To use the checksum checks between filesystems they must support a 
common hash type.

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

The local filesystem may or may not be case sensitive depending on OS.

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

| Name                   | Purge | Copy | Move | DirMove | CleanUp |
| ---------------------- |:-----:|:----:|:----:|:-------:|:-------:|
| Google Drive           | Yes   | Yes  | Yes  | Yes     | No  [#575](https://github.com/ncw/rclone/issues/575) | 
| Amazon S3              | No    | Yes  | No   | No      | No      |
| Openstack Swift        | Yes † | Yes  | No   | No      | No      |
| Dropbox                | Yes   | Yes  | Yes  | Yes     | No  [#575](https://github.com/ncw/rclone/issues/575) |
| Google Cloud Storage   | Yes   | Yes  | No   | No      | No      |
| Amazon Drive           | Yes   | No   | Yes  | Yes     | No [#575](https://github.com/ncw/rclone/issues/575) |
| Microsoft One Drive    | Yes   | Yes  | No [#197](https://github.com/ncw/rclone/issues/197) | No [#197](https://github.com/ncw/rclone/issues/197)    | No [#575](https://github.com/ncw/rclone/issues/575) |
| Hubic                  | Yes † | Yes  | No   | No      | No      |
| Backblaze B2           | No    | No   | No   | No      | Yes     |
| Yandex Disk            | Yes   | No   | No   | No      | No  [#575](https://github.com/ncw/rclone/issues/575) |
| The local filesystem   | Yes   | No   | Yes  | Yes     | No      |


### Purge ###

This deletes a directory quicker than just deleting all the files in
the directory.

† Note Swift and Hubic implement this in order to delete directory
markers but they don't actually have a quicker way of deleting files
other than deleting them individually.

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
