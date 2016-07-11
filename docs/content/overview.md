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

| Name                   | Hash    | ModTime | Case Insensitive | Duplicate Files |
| ---------------------- |:-------:|:-------:|:----------------:|:---------------:|
| Google Drive           | MD5     | Yes     | No               | Yes             |
| Amazon S3              | MD5     | Yes     | No               | No              |
| Openstack Swift        | MD5     | Yes     | No               | No              |
| Dropbox                | -       | No      | Yes              | No              |
| Google Cloud Storage   | MD5     | Yes     | No               | No              |
| Amazon Drive           | MD5     | No      | Yes              | No              |
| Microsoft One Drive    | SHA1    | Yes     | Yes              | No              |
| Hubic                  | MD5     | Yes     | No               | No              |
| Backblaze B2           | SHA1    | Yes     | No               | No              |
| Yandex Disk            | MD5     | Yes     | No               | No              |
| The local filesystem   | All     | Yes     | Depends          | No              |

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
