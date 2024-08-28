<div align="center">
<sup>Special thanks to our sponsor:</sup>
<br>
<br>
<a href="https://www.warp.dev/?utm_source=github&utm_medium=referral&utm_campaign=rclone_20231103">
  <div>
    <img src="https://rclone.org/img/logos/warp-github.svg" width="300" alt="Warp">
  </div>
  <b>Warp is a modern, Rust-based terminal with AI built in so you and your team can build great software, faster.</b>
  <div>
    <sup>Visit warp.dev to learn more.</sup>
  </div>
</a>
<br>
<hr>
</div>
<br>

[<img src="https://rclone.org/img/logo_on_light__horizontal_color.svg" width="50%" alt="rclone logo">](https://rclone.org/#gh-light-mode-only)
[<img src="https://rclone.org/img/logo_on_dark__horizontal_color.svg" width="50%" alt="rclone logo">](https://rclone.org/#gh-dark-mode-only)

[Website](https://rclone.org) |
[Documentation](https://rclone.org/docs/) |
[Download](https://rclone.org/downloads/) |
[Contributing](CONTRIBUTING.md) |
[Changelog](https://rclone.org/changelog/) |
[Installation](https://rclone.org/install/) |
[Forum](https://forum.rclone.org/)

[![Build Status](https://github.com/rclone/rclone/workflows/build/badge.svg)](https://github.com/rclone/rclone/actions?query=workflow%3Abuild)
[![Go Report Card](https://goreportcard.com/badge/github.com/rclone/rclone)](https://goreportcard.com/report/github.com/rclone/rclone)
[![GoDoc](https://godoc.org/github.com/rclone/rclone?status.svg)](https://godoc.org/github.com/rclone/rclone)
[![Docker Pulls](https://img.shields.io/docker/pulls/rclone/rclone)](https://hub.docker.com/r/rclone/rclone)

# Rclone

Rclone *("rsync for cloud storage")* is a command-line program to sync files and directories to and from different cloud storage providers.

## Storage providers

  * 1Fichier [:page_facing_up:](https://rclone.org/fichier/)
  * Akamai Netstorage [:page_facing_up:](https://rclone.org/netstorage/)
  * Alibaba Cloud (Aliyun) Object Storage System (OSS) [:page_facing_up:](https://rclone.org/s3/#alibaba-oss)
  * Amazon S3 [:page_facing_up:](https://rclone.org/s3/)
  * ArvanCloud Object Storage (AOS) [:page_facing_up:](https://rclone.org/s3/#arvan-cloud-object-storage-aos)
  * Backblaze B2 [:page_facing_up:](https://rclone.org/b2/)
  * Box [:page_facing_up:](https://rclone.org/box/)
  * Ceph [:page_facing_up:](https://rclone.org/s3/#ceph)
  * China Mobile Ecloud Elastic Object Storage (EOS) [:page_facing_up:](https://rclone.org/s3/#china-mobile-ecloud-eos)
  * Cloudflare R2 [:page_facing_up:](https://rclone.org/s3/#cloudflare-r2)
  * Citrix ShareFile [:page_facing_up:](https://rclone.org/sharefile/)
  * DigitalOcean Spaces [:page_facing_up:](https://rclone.org/s3/#digitalocean-spaces)
  * Digi Storage [:page_facing_up:](https://rclone.org/koofr/#digi-storage)
  * Dreamhost [:page_facing_up:](https://rclone.org/s3/#dreamhost)
  * Dropbox [:page_facing_up:](https://rclone.org/dropbox/)
  * Enterprise File Fabric [:page_facing_up:](https://rclone.org/filefabric/)
  * Fastmail Files [:page_facing_up:](https://rclone.org/webdav/#fastmail-files)
  * Files.com [:page_facing_up:](https://rclone.org/filescom/)
  * FTP [:page_facing_up:](https://rclone.org/ftp/)
  * GoFile [:page_facing_up:](https://rclone.org/gofile/)
  * Google Cloud Storage [:page_facing_up:](https://rclone.org/googlecloudstorage/)
  * Google Drive [:page_facing_up:](https://rclone.org/drive/)
  * Google Photos [:page_facing_up:](https://rclone.org/googlephotos/)
  * HDFS (Hadoop Distributed Filesystem) [:page_facing_up:](https://rclone.org/hdfs/)
  * Hetzner Storage Box [:page_facing_up:](https://rclone.org/sftp/#hetzner-storage-box)
  * HiDrive [:page_facing_up:](https://rclone.org/hidrive/)
  * HTTP [:page_facing_up:](https://rclone.org/http/)
  * Huawei Cloud Object Storage Service(OBS) [:page_facing_up:](https://rclone.org/s3/#huawei-obs)
  * ImageKit [:page_facing_up:](https://rclone.org/imagekit/)
  * Internet Archive [:page_facing_up:](https://rclone.org/internetarchive/)
  * Jottacloud [:page_facing_up:](https://rclone.org/jottacloud/)
  * IBM COS S3 [:page_facing_up:](https://rclone.org/s3/#ibm-cos-s3)
  * IONOS Cloud [:page_facing_up:](https://rclone.org/s3/#ionos)
  * Koofr [:page_facing_up:](https://rclone.org/koofr/)
  * Leviia Object Storage [:page_facing_up:](https://rclone.org/s3/#leviia)
  * Liara Object Storage [:page_facing_up:](https://rclone.org/s3/#liara-object-storage)
  * Linkbox [:page_facing_up:](https://rclone.org/linkbox)
  * Linode Object Storage [:page_facing_up:](https://rclone.org/s3/#linode)
  * Magalu Object Storage [:page_facing_up:](https://rclone.org/s3/#magalu)
  * Mail.ru Cloud [:page_facing_up:](https://rclone.org/mailru/)
  * Memset Memstore [:page_facing_up:](https://rclone.org/swift/)
  * Mega [:page_facing_up:](https://rclone.org/mega/)
  * Memory [:page_facing_up:](https://rclone.org/memory/)
  * Microsoft Azure Blob Storage [:page_facing_up:](https://rclone.org/azureblob/)
  * Microsoft Azure Files Storage [:page_facing_up:](https://rclone.org/azurefiles/)
  * Microsoft OneDrive [:page_facing_up:](https://rclone.org/onedrive/)
  * Minio [:page_facing_up:](https://rclone.org/s3/#minio)
  * Nextcloud [:page_facing_up:](https://rclone.org/webdav/#nextcloud)
  * OVH [:page_facing_up:](https://rclone.org/swift/)
  * Blomp Cloud Storage [:page_facing_up:](https://rclone.org/swift/)
  * OpenDrive [:page_facing_up:](https://rclone.org/opendrive/)
  * OpenStack Swift [:page_facing_up:](https://rclone.org/swift/)
  * Oracle Cloud Storage [:page_facing_up:](https://rclone.org/swift/)
  * Oracle Object Storage [:page_facing_up:](https://rclone.org/oracleobjectstorage/)
  * ownCloud [:page_facing_up:](https://rclone.org/webdav/#owncloud)
  * pCloud [:page_facing_up:](https://rclone.org/pcloud/)
  * Petabox [:page_facing_up:](https://rclone.org/s3/#petabox)
  * PikPak [:page_facing_up:](https://rclone.org/pikpak/)
  * Pixeldrain [:page_facing_up:](https://rclone.org/pixeldrain/)
  * premiumize.me [:page_facing_up:](https://rclone.org/premiumizeme/)
  * put.io [:page_facing_up:](https://rclone.org/putio/)
  * Proton Drive [:page_facing_up:](https://rclone.org/protondrive/)
  * QingStor [:page_facing_up:](https://rclone.org/qingstor/)
  * Qiniu Cloud Object Storage (Kodo) [:page_facing_up:](https://rclone.org/s3/#qiniu)
  * Quatrix [:page_facing_up:](https://rclone.org/quatrix/)
  * Rackspace Cloud Files [:page_facing_up:](https://rclone.org/swift/)
  * RackCorp Object Storage [:page_facing_up:](https://rclone.org/s3/#RackCorp)
  * rsync.net [:page_facing_up:](https://rclone.org/sftp/#rsync-net)
  * Scaleway [:page_facing_up:](https://rclone.org/s3/#scaleway)
  * Seafile [:page_facing_up:](https://rclone.org/seafile/)
  * SeaweedFS [:page_facing_up:](https://rclone.org/s3/#seaweedfs)
  * SFTP [:page_facing_up:](https://rclone.org/sftp/)
  * SMB / CIFS [:page_facing_up:](https://rclone.org/smb/)
  * StackPath [:page_facing_up:](https://rclone.org/s3/#stackpath)
  * Storj [:page_facing_up:](https://rclone.org/storj/)
  * SugarSync [:page_facing_up:](https://rclone.org/sugarsync/)
  * Synology C2 Object Storage [:page_facing_up:](https://rclone.org/s3/#synology-c2)
  * Tencent Cloud Object Storage (COS) [:page_facing_up:](https://rclone.org/s3/#tencent-cos)
  * Uloz.to [:page_facing_up:](https://rclone.org/ulozto/)
  * Wasabi [:page_facing_up:](https://rclone.org/s3/#wasabi)
  * WebDAV [:page_facing_up:](https://rclone.org/webdav/)
  * Yandex Disk [:page_facing_up:](https://rclone.org/yandex/)
  * Zoho WorkDrive [:page_facing_up:](https://rclone.org/zoho/)
  * The local filesystem [:page_facing_up:](https://rclone.org/local/)

Please see [the full list of all storage providers and their features](https://rclone.org/overview/)

### Virtual storage providers

These backends adapt or modify other storage providers

  * Alias: rename existing remotes [:page_facing_up:](https://rclone.org/alias/)
  * Cache: cache remotes (DEPRECATED) [:page_facing_up:](https://rclone.org/cache/)
  * Chunker: split large files [:page_facing_up:](https://rclone.org/chunker/)
  * Combine: combine multiple remotes into a directory tree [:page_facing_up:](https://rclone.org/combine/)
  * Compress: compress files [:page_facing_up:](https://rclone.org/compress/)
  * Crypt: encrypt files [:page_facing_up:](https://rclone.org/crypt/)
  * Hasher: hash files [:page_facing_up:](https://rclone.org/hasher/)
  * Union: join multiple remotes to work together [:page_facing_up:](https://rclone.org/union/)

## Features

  * MD5/SHA-1 hashes checked at all times for file integrity
  * Timestamps preserved on files
  * Partial syncs supported on a whole file basis
  * [Copy](https://rclone.org/commands/rclone_copy/) mode to just copy new/changed files
  * [Sync](https://rclone.org/commands/rclone_sync/) (one way) mode to make a directory identical
  * [Bisync](https://rclone.org/bisync/) (two way) to keep two directories in sync bidirectionally
  * [Check](https://rclone.org/commands/rclone_check/) mode to check for file hash equality
  * Can sync to and from network, e.g. two different cloud accounts
  * Optional large file chunking ([Chunker](https://rclone.org/chunker/))
  * Optional transparent compression ([Compress](https://rclone.org/compress/))
  * Optional encryption ([Crypt](https://rclone.org/crypt/))
  * Optional FUSE mount ([rclone mount](https://rclone.org/commands/rclone_mount/))
  * Multi-threaded downloads to local disk
  * Can [serve](https://rclone.org/commands/rclone_serve/) local or remote files over HTTP/WebDAV/FTP/SFTP/DLNA

## Installation & documentation

Please see the [rclone website](https://rclone.org/) for:

  * [Installation](https://rclone.org/install/)
  * [Documentation & configuration](https://rclone.org/docs/)
  * [Changelog](https://rclone.org/changelog/)
  * [FAQ](https://rclone.org/faq/)
  * [Storage providers](https://rclone.org/overview/)
  * [Forum](https://forum.rclone.org/)
  * ...and more

## Downloads

  * https://rclone.org/downloads/

License
-------

This is free software under the terms of the MIT license (check the
[COPYING file](/COPYING) included in this package).
