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

| Name                   | MD5SUM  | ModTime | Case Sensitive | Duplicate Files |
| ---------------------- |:-------:|:-------:|:--------------:|:---------------:|
| Google Drive           | Yes     | Yes     | No             | Yes             |
| Amazon S3              | Yes     | Yes     | No             | No              |
| Openstack Swift        | Yes     | Yes     | No             | No              |
| Dropbox                | No      | No      | Yes            | No              |
| Google Cloud Storage   | Yes     | Yes     | No             | No              |
| Amazon Cloud Drive     | Yes     | No      | Yes            | No              |
| The local filesystem   | Yes     | Yes     | Depends        | No              |

### MD5SUM ###

The cloud storage system supports MD5SUMs of the objects.  This
is used if available when transferring data as an integrity check and
can be specifically used with the `--checksum` flag in syncs and in
the `check` command.

### ModTime ###

The cloud storage system supports setting modification times on
objects.  If it does then this enables a using the modification times
as part of the sync.  If not then only the size will be checked by
default, though the MD5SUM can be checked with the `--checksum` flag.

All cloud storage systems support some kind of date on the object and
these will be set when transferring from the cloud storage system.

### Case Sensitive ###

If a cloud storage systems is case sensitive then it is possible to
have two files which differ only in case, eg `file.txt` and
`FILE.txt`.  If a cloud storage system is case insensitive then that
isn't possible.

This can cause problems when syncing between a case insensitive
system and a case sensitive system.  The symptom of this is that no
matter how many times you run the sync it never completes fully.

The local filesystem may or may not be case sensitive depending on OS.

  * Windows - usuall case insensitive
  * OSX - usually case insensitive, though it is possible to format case sensitive
  * Linux - usually case sensitive, but there are case insensitive file systems (eg FAT formatted USB keys)

Most of the time this doesn't cause any problems as people tend to
avoid files whose name differs only by case even on case sensitive
systems.

### Duplicate files ###

If a cloud storage system allows duplicate files then it can have two
objects with the same name.

This confuses rclone greatly when syncing.
