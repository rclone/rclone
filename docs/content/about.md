---
title: "Rclone"
description: "rclone syncs files to and from Google Drive, S3, Swift, Cloudfiles, Dropbox, Google Cloud Storage and Amazon Drive."
type: page
date: "2017-09-25"
groups: ["about"]
---

Rclone
======

[![Logo](/img/rclone-120x120.png)](https://rclone.org/)

Rclone is a command line program to sync files and directories to and from:

* {{< provider name="1Fichier" home="https://1fichier.com/" config="/fichier/" >}}
* {{< provider name="Alibaba Cloud (Aliyun) Object Storage System (OSS)" home="https://www.alibabacloud.com/product/oss/" config="/s3/#alibaba-oss" >}}
* {{< provider name="Amazon Drive" home="https://www.amazon.com/clouddrive" config="/amazonclouddrive/" >}} ([See note](/amazonclouddrive/#status))
* {{< provider name="Amazon S3" home="https://aws.amazon.com/s3/" config="/s3/" >}}
* {{< provider name="Backblaze B2" home="https://www.backblaze.com/b2/cloud-storage.html" config="/b2/" >}}
* {{< provider name="Box" home="https://www.box.com/" config="/box/" >}}
* {{< provider name="Ceph" home="http://ceph.com/" config="/s3/#ceph" >}}
* {{< provider name="DigitalOcean Spaces" home="https://www.digitalocean.com/products/object-storage/" config="/s3/#digitalocean-spaces" >}}
* {{< provider name="Dreamhost" home="https://www.dreamhost.com/cloud/storage/" config="/s3/#dreamhost" >}}
* {{< provider name="Dropbox" home="https://www.dropbox.com/" config="/dropbox/" >}}
* {{< provider name="FTP" home="https://en.wikipedia.org/wiki/File_Transfer_Protocol" config="/ftp/" >}}
* {{< provider name="Google Cloud Storage" home="https://cloud.google.com/storage/" config="/googlecloudstorage/" >}}
* {{< provider name="Google Drive" home="https://www.google.com/drive/" config="/drive/" >}}
* {{< provider name="Google Photos" home="https://www.google.com/photos/about/" config="/googlephotos/" >}}
* {{< provider name="HTTP" home="https://en.wikipedia.org/wiki/Hypertext_Transfer_Protocol" config="/http/" >}}
* {{< provider name="Hubic" home="https://hubic.com/" config="/hubic/" >}}
* {{< provider name="Jottacloud" home="https://www.jottacloud.com/en/" config="/jottacloud/" >}}
* {{< provider name="IBM COS S3" home="http://www.ibm.com/cloud/object-storage" config="/s3/#ibm-cos-s3" >}}
* {{< provider name="Koofr" home="https://koofr.eu/" config="/koofr/" >}}
* {{< provider name="Memset Memstore" home="https://www.memset.com/cloud/storage/" config="/swift/" >}}
* {{< provider name="Mega" home="https://mega.nz/" config="/mega/" >}}
* {{< provider name="Microsoft Azure Blob Storage" home="https://azure.microsoft.com/en-us/services/storage/blobs/" config="/azureblob/" >}}
* {{< provider name="Microsoft OneDrive" home="https://onedrive.live.com/" config="/onedrive/" >}}
* {{< provider name="Minio" home="https://www.minio.io/" config="/s3/#minio" >}}
* {{< provider name="Nextcloud" home="https://nextcloud.com/" config="/webdav/#nextcloud" >}}
* {{< provider name="OVH" home="https://www.ovh.co.uk/public-cloud/storage/object-storage/" config="/swift/" >}}
* {{< provider name="OpenDrive" home="https://www.opendrive.com/" config="/opendrive/" >}}
* {{< provider name="Openstack Swift" home="https://docs.openstack.org/swift/latest/" config="/swift/" >}}
* {{< provider name="Oracle Cloud Storage" home="https://cloud.oracle.com/storage-opc" config="/swift/" >}}
* {{< provider name="ownCloud" home="https://owncloud.org/" config="/webdav/#owncloud" >}}
* {{< provider name="pCloud" home="https://www.pcloud.com/" config="/pcloud/" >}}
* {{< provider name="put.io" home="https://put.io/" config="/webdav/#put-io" >}}
* {{< provider name="QingStor" home="https://www.qingcloud.com/products/storage" config="/qingstor/" >}}
* {{< provider name="Rackspace Cloud Files" home="https://www.rackspace.com/cloud/files" config="/swift/" >}}
* {{< provider name="rsync.net" home="https://rsync.net/products/rclone.html" config="/sftp/" >}}
* {{< provider name="Scaleway" home="https://www.scaleway.com/object-storage/" config="/s3/#scaleway" >}}
* {{< provider name="SFTP" home="https://en.wikipedia.org/wiki/SFTP" config="/sftp/" >}}
* {{< provider name="Wasabi" home="https://wasabi.com/" config="/s3/#wasabi" >}}
* {{< provider name="WebDAV" home="https://en.wikipedia.org/wiki/WebDAV" config="/webdav/" >}}
* {{< provider name="Yandex Disk" home="https://disk.yandex.com/" config="/yandex/" >}}
* {{< provider name="The local filesystem" home="/local/" config="/local/" >}}

Features

  * MD5/SHA1 hashes checked at all times for file integrity
  * Timestamps preserved on files
  * Partial syncs supported on a whole file basis
  * [Copy](/commands/rclone_copy/) mode to just copy new/changed files
  * [Sync](/commands/rclone_sync/) (one way) mode to make a directory identical
  * [Check](/commands/rclone_check/) mode to check for file hash equality
  * Can sync to and from network, eg two different cloud accounts
  * [Encryption](/crypt/) backend
  * [Cache](/cache/) backend
  * [Union](/union/) backend
  * Optional FUSE mount ([rclone mount](/commands/rclone_mount/))
  * Multi-threaded downloads to local disk
  * Can [serve](/commands/rclone_serve/) local or remote files over [HTTP](/commands/rclone_serve_http/)/[WebDav](/commands/rclone_serve_webdav/)/[FTP](/commands/rclone_serve_ftp/)/[SFTP](/commands/rclone_serve_sftp/)/[dlna](/commands/rclone_serve_dlna/)

Links

  * <i class="fa fa-home"></i> [Home page](https://rclone.org/)
  * <i class="fa fa-github"></i> [GitHub project page for source and bug tracker](https://github.com/ncw/rclone)
  * <i class="fa fa-comments"></i> [Rclone Forum](https://forum.rclone.org)
  * <i class="fa fa-cloud-download"></i>[Downloads](/downloads/)
