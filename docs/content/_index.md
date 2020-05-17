---
title: "Rclone"
description: "Rclone syncs your files to cloud storage: Google Drive, S3, Swift, Dropbox, Google Cloud Storage, Azure, Box and many more."
type: page
date: "2020-05-16"
---

# Rclone syncs your files to cloud storage

<img width="50%" src="/img/logo_on_light__horizontal_color.svg" alt="rclone logo" style="float:right; padding: 5px;">

- [About rclone.](#about)
- [What is rclone for.](#what)
- [What features does rclone have.](#features)
- [What providers does rclone support.](#providers)
- [Download.](/downloads/)
- [Install.](/install/)

## About rclone {#about}

Rclone is a command line tool to manage files on cloud storage. It is
a feature rich alternative to cloud vendor's web storage
interfaces. [Over 40 cloud storage products](#providers) are supported
by rclone including S3 object stores, business & consumer file storage
services, as well as standard transfer protocols.

Rclone is a powerful tool being the cloud storage equivalent to the
unix commands rsync, cp, mv, mount, ls, ncdu, tree, rm, and
cat. Rclone's familiar syntax includes shell pipeline support, and
`--dry-run` protection. It can be used at the command line, in scripts
or via its [API](/rc).

Rclone really cares about your data. It preserves your timestamps and
verifies your data at all times. Transfers over limited bandwidth;
intermittent connections, or subject to quota can be restarted, from
the last good file transferred. You can
[check](/commands/rclone_check/) the integrity of your files. Where
possible, rclone employs server side transfers to minimise local
bandwidth use and can transfer from one provider to another without
using your local disk.

Virtual backends can wrap local and cloud file systems to apply
[encryption](/crypt/), 
[caching](/cache/),
[chunking](/chunker/) and
[joining](/union/).

Rclone can [mount](/commands/rclone_mount/) any local, cloud or
virtual filesystem so it will appear as a local disk on your Windows,
macOS, linux or FreeBSD computer. Rclone can also serve these over
[SFTP](/commands/rclone_serve_sftp/),
[HTTP](/commands/rclone_serve_http/),
[WebDAV](/commands/rclone_serve_webdav/),
[FTP](/commands/rclone_serve_ftp/) and
[DLNA](/commands/rclone_serve_dlna/).

Rclone is mature, open source software originally inspired by
rsync. It is written in [Go](https://golang.org) and is energetically
maintained, and supported by a welcoming community with a wide
experience of varied use cases. Official repos such as Ubuntu, Debian,
Brew, Chocolatey include rclone. For the latest version [download from
rclone.org](/downloads/) is recommended.

Rclone is widely used on Linux, Windows and Mac. Third party
developers have built innovative backup, restore, GUI and business
process solutions using the rclone command line or API.

Let rclone do the heavy lifting of communicating with cloud cloud
storage so you can concentrate on your problems.

## What is rclone for {#what}

Rclone is great for:-

- Backup and encryption of files to cloud storage
- Restore and decryption of files from cloud storage
- Mirroring cloud data to other cloud services or locally
- Migration of data to cloud, or between cloud storage vendors
- Mounting multiple, encrypted, cached or diverse cloud storage
- Union file systems presenting multiple local and/or cloud file systems
- Analysing file data held on cloud storage

## Features {#features}

- On all transfers
    - MD5, SHA1 hashes checked at all times for file integrity
    - Timestamps preserved on files
    - Operations can be restarted at any time
    - Can sync to and from network, eg two different cloud accounts
    - multi-threaded downloads to local disk
- [Copy](/commands/rclone_copy/) new or changed files to cloud storage
- [Sync](/commands/rclone_sync/) (one way) to make a directory identical
- [Move](/commands/rclone_move/) move files to cloud storage deleting the local after verification
- [Check](/commands/rclone_check/) check hashes and for missing/extra files
- [Mount](/commands/rclone_mount/) your cloud storage as a network disk
- [Serve](/commands/rclone_serve/) local or remote files over [HTTP](/commands/rclone_serve_http/)/[WebDav](/commands/rclone_serve_webdav/)/[FTP](/commands/rclone_serve_ftp/)/[SFTP](/commands/rclone_serve_sftp/)/[dlna](/commands/rclone_serve_dlna/)
- Experimental [Web based GUI](/gui/)

## Supported providers {#providers}

Here is a list of providers that rclone supports. This isn't an
exhaustive list as there are many more that support standard protocols
such as WebDAV or S3 that work out of the box.

* {{< provider name="1Fichier" home="https://1fichier.com/" config="/fichier/" >}}
* {{< provider name="Alibaba Cloud (Aliyun) Object Storage System (OSS)" home="https://www.alibabacloud.com/product/oss/" config="/s3/#alibaba-oss" >}}
* {{< provider name="Amazon Drive" home="https://www.amazon.com/clouddrive" config="/amazonclouddrive/" >}} ([See note](/amazonclouddrive/#status))
* {{< provider name="Amazon S3" home="https://aws.amazon.com/s3/" config="/s3/" >}}
* {{< provider name="Backblaze B2" home="https://www.backblaze.com/b2/cloud-storage.html" config="/b2/" >}}
* {{< provider name="Box" home="https://www.box.com/" config="/box/" >}}
* {{< provider name="Ceph" home="http://ceph.com/" config="/s3/#ceph" >}}
* {{< provider name="Citrix ShareFile" home="http://sharefile.com/" config="/sharefile/" >}}
* {{< provider name="C14" home="https://www.online.net/en/storage/c14-cold-storage" config="/sftp/#c14" >}}
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
* {{< provider name="Mail.ru Cloud" home="https://cloud.mail.ru/" config="/mailru/" >}}
* {{< provider name="Memset Memstore" home="https://www.memset.com/cloud/storage/" config="/swift/" >}}
* {{< provider name="Mega" home="https://mega.nz/" config="/mega/" >}}
* {{< provider name="Memory" home="/memory/" config="/memory/" >}}
* {{< provider name="Microsoft Azure Blob Storage" home="https://azure.microsoft.com/en-us/services/storage/blobs/" config="/azureblob/" >}}
* {{< provider name="Microsoft OneDrive" home="https://onedrive.live.com/" config="/onedrive/" >}}
* {{< provider name="Minio" home="https://www.minio.io/" config="/s3/#minio" >}}
* {{< provider name="Nextcloud" home="https://nextcloud.com/" config="/webdav/#nextcloud" >}}
* {{< provider name="OVH" home="https://www.ovh.co.uk/public-cloud/storage/object-storage/" config="/swift/" >}}
* {{< provider name="OpenDrive" home="https://www.opendrive.com/" config="/opendrive/" >}}
* {{< provider name="OpenStack Swift" home="https://docs.openstack.org/swift/latest/" config="/swift/" >}}
* {{< provider name="Oracle Cloud Storage" home="https://cloud.oracle.com/storage-opc" config="/swift/" >}}
* {{< provider name="ownCloud" home="https://owncloud.org/" config="/webdav/#owncloud" >}}
* {{< provider name="pCloud" home="https://www.pcloud.com/" config="/pcloud/" >}}
* {{< provider name="premiumize.me" home="https://premiumize.me/" config="/premiumizeme/" >}}
* {{< provider name="put.io" home="https://put.io/" config="/putio/" >}}
* {{< provider name="QingStor" home="https://www.qingcloud.com/products/storage" config="/qingstor/" >}}
* {{< provider name="Rackspace Cloud Files" home="https://www.rackspace.com/cloud/files" config="/swift/" >}}
* {{< provider name="rsync.net" home="https://rsync.net/products/rclone.html" config="/sftp/#rsync-net" >}}
* {{< provider name="Scaleway" home="https://www.scaleway.com/object-storage/" config="/s3/#scaleway" >}}
* {{< provider name="Seafile" home="https://www.seafile.com/" config="/seafile/" >}}
* {{< provider name="SFTP" home="https://en.wikipedia.org/wiki/SSH_File_Transfer_Protocol" config="/sftp/" >}}
* {{< provider name="StackPath" home="https://www.stackpath.com/products/object-storage/" config="/s3/#stackpath" >}}
* {{< provider name="SugarSync" home="https://sugarsync.com/" config="/sugarsync/" >}}
* {{< provider name="Tardigrade" home="https://tardigrade.io/" config="/tardigrade/" >}}
* {{< provider name="Wasabi" home="https://wasabi.com/" config="/s3/#wasabi" >}}
* {{< provider name="WebDAV" home="https://en.wikipedia.org/wiki/WebDAV" config="/webdav/" >}}
* {{< provider name="Yandex Disk" home="https://disk.yandex.com/" config="/yandex/" >}}
* {{< provider name="The local filesystem" home="/local/" config="/local/" >}}

Links

  * <i class="fa fa-home"></i> [Home page](https://rclone.org/)
  * <i class="fab fa-github"></i> [GitHub project page for source and bug tracker](https://github.com/rclone/rclone)
  * <i class="fa fa-comments"></i> [Rclone Forum](https://forum.rclone.org)
  * <i class="fas fa-cloud-download-alt"></i>[Downloads](/downloads/)
