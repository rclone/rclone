[![Logo](http://rclone.org/img/rclone-120x120.png)](http://rclone.org/)

[Website](http://rclone.org) |
[Documentation](http://rclone.org/docs/) |
[Contributing](CONTRIBUTING.md) |
[Changelog](http://rclone.org/changelog/) |
[Installation](http://rclone.org/install/) |
[Forum](https://forum.rclone.org/)
[G+](https://google.com/+RcloneOrg)


[![Build Status](https://travis-ci.org/ncw/rclone.svg?branch=master)](https://travis-ci.org/ncw/rclone) [![Windows Build Status](https://ci.appveyor.com/api/projects/status/github/ncw/rclone?branch=master&passingText=windows%20-%20ok&svg=true)](https://ci.appveyor.com/project/ncw/rclone) [![GoDoc](https://godoc.org/github.com/ncw/rclone?status.svg)](https://godoc.org/github.com/ncw/rclone) 

Rclone is a command line program to sync files and directories to and from

  * Google Drive
  * Amazon S3
  * Openstack Swift / Rackspace cloud files / Memset Memstore
  * Dropbox
  * Google Cloud Storage
  * Amazon Drive
  * Microsoft One Drive
  * Hubic
  * Backblaze B2
  * Yandex Disk
  * SFTP
  * The local filesystem

Features

  * MD5/SHA1 hashes checked at all times for file integrity
  * Timestamps preserved on files
  * Partial syncs supported on a whole file basis
  * Copy mode to just copy new/changed files
  * Sync (one way) mode to make a directory identical
  * Check mode to check for file hash equality
  * Can sync to and from network, eg two different cloud accounts
  * Optional encryption (Crypt)
  * Optional FUSE mount

See the home page for installation, usage, documentation, changelog
and configuration walkthroughs.

  * http://rclone.org/

License
-------

This is free software under the terms of MIT the license (check the
COPYING file included in this package).
