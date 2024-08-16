---
title: "Rclone"
description: "Rclone syncs your files to cloud storage: Google Drive, S3, Swift, Dropbox, Google Cloud Storage, Azure, Box and many more."
type: page
notoc: true
---

# Rclone syncs your files to cloud storage

{{< img width="50%" src="/img/logo_on_light__horizontal_color.svg" alt="rclone logo" style="float:right; padding: 5px;" >}}

- [About rclone](#about)
- [What can rclone do for you?](#what)
- [What features does rclone have?](#features)
- [What providers does rclone support?](#providers)
- [Download](/downloads/)
- [Install](/install/)
{{< rem MAINPAGELINK >}}

## About rclone {#about}

Rclone is a command-line program to manage files on cloud storage. It
is a feature-rich alternative to cloud vendors' web storage
interfaces. [Over 70 cloud storage products](#providers) support
rclone including S3 object stores, business & consumer file storage
services, as well as standard transfer protocols.

Rclone has powerful cloud equivalents to the unix commands rsync, cp,
mv, mount, ls, ncdu, tree, rm, and cat. Rclone's familiar syntax
includes shell pipeline support, and `--dry-run` protection. It is
used at the command line, in scripts or via its [API](/rc).

Users call rclone *"The Swiss army knife of cloud storage"*, and
*"Technology indistinguishable from magic"*.

Rclone really looks after your data. It preserves timestamps and
verifies checksums at all times. Transfers over limited bandwidth;
intermittent connections, or subject to quota can be restarted, from
the last good file transferred. You can
[check](/commands/rclone_check/) the integrity of your files. Where
possible, rclone employs server-side transfers to minimise local
bandwidth use and transfers from one provider to another without
using local disk.

Virtual backends wrap local and cloud file systems to apply
[encryption](/crypt/),
[compression](/compress/),
[chunking](/chunker/),
[hashing](/hasher/) and
[joining](/union/).

Rclone [mounts](/commands/rclone_mount/) any local, cloud or
virtual filesystem as a disk on Windows,
macOS, linux and FreeBSD, and also serves these over
[SFTP](/commands/rclone_serve_sftp/),
[HTTP](/commands/rclone_serve_http/),
[WebDAV](/commands/rclone_serve_webdav/),
[FTP](/commands/rclone_serve_ftp/) and
[DLNA](/commands/rclone_serve_dlna/).

Rclone is mature, open-source software originally inspired by rsync
and written in [Go](https://golang.org). The friendly support
community is familiar with varied use cases. Official Ubuntu, Debian,
Fedora, Brew and Chocolatey repos. include rclone. For the latest
version [downloading from rclone.org](/downloads/) is recommended.

Rclone is widely used on Linux, Windows and Mac. Third-party
developers create innovative backup, restore, GUI and business
process solutions using the rclone command line or API.

Rclone does the heavy lifting of communicating with cloud storage.

## What can rclone do for you? {#what}

Rclone helps you:

- Backup (and encrypt) files to cloud storage
- Restore (and decrypt) files from cloud storage
- Mirror cloud data to other cloud services or locally
- Migrate data to the cloud, or between cloud storage vendors
- Mount multiple, encrypted, cached or diverse cloud storage as a disk
- Analyse and account for data held on cloud storage using [lsf](/commands/rclone_lsf/), [ljson](/commands/rclone_lsjson/), [size](/commands/rclone_size/), [ncdu](/commands/rclone_ncdu/)
- [Union](/union/) file systems together to present multiple local and/or cloud file systems as one

## Features {#features}

- Transfers
    - MD5, SHA1 hashes are checked at all times for file integrity
    - Timestamps are preserved on files
    - Operations can be restarted at any time
    - Can be to and from network, e.g. two different cloud providers
    - Can use multi-threaded downloads to local disk
- [Copy](/commands/rclone_copy/) new or changed files to cloud storage
- [Sync](/commands/rclone_sync/) (one way) to make a directory identical
- [Bisync](/bisync/) (two way) to keep two directories in sync bidirectionally
- [Move](/commands/rclone_move/) files to cloud storage deleting the local after verification
- [Check](/commands/rclone_check/) hashes and for missing/extra files
- [Mount](/commands/rclone_mount/) your cloud storage as a network disk
- [Serve](/commands/rclone_serve/) local or remote files over [HTTP](/commands/rclone_serve_http/)/[WebDav](/commands/rclone_serve_webdav/)/[FTP](/commands/rclone_serve_ftp/)/[SFTP](/commands/rclone_serve_sftp/)/[DLNA](/commands/rclone_serve_dlna/)
- Experimental [Web based GUI](/gui/)

## Supported providers {#providers}

(There are many others, built on standard protocols such as
WebDAV or S3, that work out of the box.)

{{< provider_list >}}
{{< provider name="1Fichier" home="https://1fichier.com/" config="/fichier/" start="true">}}
{{< provider name="Akamai Netstorage" home="https://www.akamai.com/us/en/products/media-delivery/netstorage.jsp" config="/netstorage/" >}}
{{< provider name="Alibaba Cloud (Aliyun) Object Storage System (OSS)" home="https://www.alibabacloud.com/product/oss/" config="/s3/#alibaba-oss" >}}
{{< provider name="Amazon S3" home="https://aws.amazon.com/s3/" config="/s3/" >}}
{{< provider name="Backblaze B2" home="https://www.backblaze.com/cloud-storage" config="/b2/" >}}
{{< provider name="Box" home="https://www.box.com/" config="/box/" >}}
{{< provider name="Ceph" home="http://ceph.com/" config="/s3/#ceph" >}}
{{< provider name="China Mobile Ecloud Elastic Object Storage (EOS)" home="https://ecloud.10086.cn/home/product-introduction/eos/" config="/s3/#china-mobile-ecloud-eos" >}}
{{< provider name="Arvan Cloud Object Storage (AOS)" home="https://www.arvancloud.ir/en/products/cloud-storage" config="/s3/#arvan-cloud-object-storage-aos" >}}
{{< provider name="Citrix ShareFile" home="http://sharefile.com/" config="/sharefile/" >}}
{{< provider name="Cloudflare R2" home="https://blog.cloudflare.com/r2-open-beta/" config="/s3/#cloudflare-r2" >}}
{{< provider name="DigitalOcean Spaces" home="https://www.digitalocean.com/products/object-storage/" config="/s3/#digitalocean-spaces" >}}
{{< provider name="Digi Storage" home="https://storage.rcs-rds.ro/" config="/koofr/#digi-storage" >}}
{{< provider name="Dreamhost" home="https://www.dreamhost.com/cloud/storage/" config="/s3/#dreamhost" >}}
{{< provider name="Dropbox" home="https://www.dropbox.com/" config="/dropbox/" >}}
{{< provider name="Enterprise File Fabric" home="https://storagemadeeasy.com/about/" config="/filefabric/" >}}
{{< provider name="Fastmail Files" home="https://www.fastmail.com/" config="/webdav/#fastmail-files" >}}
{{< provider name="Files.com" home="https://www.files.com/" config="/filescom/" >}}
{{< provider name="FTP" home="https://en.wikipedia.org/wiki/File_Transfer_Protocol" config="/ftp/" >}}
{{< provider name="Gofile" home="https://gofile.io/" config="/gofile/" >}}
{{< provider name="Google Cloud Storage" home="https://cloud.google.com/storage/" config="/googlecloudstorage/" >}}
{{< provider name="Google Drive" home="https://www.google.com/drive/" config="/drive/" >}}
{{< provider name="Google Photos" home="https://www.google.com/photos/about/" config="/googlephotos/" >}}
{{< provider name="HDFS" home="https://hadoop.apache.org/" config="/hdfs/" >}}
{{< provider name="Hetzner Storage Box" home="https://www.hetzner.com/storage/storage-box" config="/sftp/#hetzner-storage-box" >}}
{{< provider name="HiDrive" home="https://www.strato.de/cloud-speicher/" config="/hidrive/" >}}
{{< provider name="HTTP" home="https://en.wikipedia.org/wiki/Hypertext_Transfer_Protocol" config="/http/" >}}
{{< provider name="ImageKit" home="https://imagekit.io" config="/imagekit/" >}}
{{< provider name="Internet Archive" home="https://archive.org/" config="/internetarchive/" >}}
{{< provider name="Jottacloud" home="https://www.jottacloud.com/en/" config="/jottacloud/" >}}
{{< provider name="IBM COS S3" home="http://www.ibm.com/cloud/object-storage" config="/s3/#ibm-cos-s3" >}}
{{< provider name="IDrive e2" home="https://www.idrive.com/e2/?refer=rclone" config="/s3/#idrive-e2" >}}
{{< provider name="IONOS Cloud" home="https://cloud.ionos.com/storage/object-storage" config="/s3/#ionos" >}}
{{< provider name="Koofr" home="https://koofr.eu/" config="/koofr/" >}}
{{< provider name="Leviia Object Storage" home="https://www.leviia.com/object-storage" config="/s3/#leviia" >}}
{{< provider name="Liara Object Storage" home="https://liara.ir/landing/object-storage" config="/s3/#liara-object-storage" >}}
{{< provider name="Linkbox" home="https://linkbox.to/" config="/linkbox/" >}}
{{< provider name="Linode Object Storage" home="https://www.linode.com/products/object-storage/" config="/s3/#linode" >}}
{{< provider name="Magalu" home="https://magalu.cloud/object-storage/" config="/s3/#magalu" >}}
{{< provider name="Mail.ru Cloud" home="https://cloud.mail.ru/" config="/mailru/" >}}
{{< provider name="Memset Memstore" home="https://www.memset.com/cloud/storage/" config="/swift/" >}}
{{< provider name="Mega" home="https://mega.nz/" config="/mega/" >}}
{{< provider name="Memory" home="/memory/" config="/memory/" >}}
{{< provider name="Microsoft Azure Blob Storage" home="https://azure.microsoft.com/en-us/services/storage/blobs/" config="/azureblob/" >}}
{{< provider name="Microsoft Azure Files Storage" home="https://azure.microsoft.com/en-us/services/storage/files/" config="/azurefiles/" >}}
{{< provider name="Microsoft OneDrive" home="https://onedrive.live.com/" config="/onedrive/" >}}
{{< provider name="Minio" home="https://www.minio.io/" config="/s3/#minio" >}}
{{< provider name="Nextcloud" home="https://nextcloud.com/" config="/webdav/#nextcloud" >}}
{{< provider name="OVH" home="https://www.ovh.co.uk/public-cloud/storage/object-storage/" config="/swift/" >}}
{{< provider name="Blomp Cloud Storage" home="https://rclone.org/swift/" config="/swift/" >}}
{{< provider name="OpenDrive" home="https://www.opendrive.com/" config="/opendrive/" >}}
{{< provider name="OpenStack Swift" home="https://docs.openstack.org/swift/latest/" config="/swift/" >}}
{{< provider name="Oracle Cloud Storage Swift" home="https://docs.oracle.com/en-us/iaas/integration/doc/configure-object-storage.html" config="/swift/" >}}
{{< provider name="Oracle Object Storage" home="https://www.oracle.com/cloud/storage/object-storage" config="/oracleobjectstorage/" >}}
{{< provider name="ownCloud" home="https://owncloud.org/" config="/webdav/#owncloud" >}}
{{< provider name="pCloud" home="https://www.pcloud.com/" config="/pcloud/" >}}
{{< provider name="Petabox" home="https://petabox.io/" config="/s3/#petabox" >}}
{{< provider name="PikPak" home="https://mypikpak.com/" config="/pikpak/" >}}
{{< provider name="Pixeldrain" home="https://pixeldrain.com/" config="/pixeldrain/" >}}
{{< provider name="premiumize.me" home="https://premiumize.me/" config="/premiumizeme/" >}}
{{< provider name="put.io" home="https://put.io/" config="/putio/" >}}
{{< provider name="Proton Drive" home="https://proton.me/drive" config="/protondrive/" >}}
{{< provider name="QingStor" home="https://www.qingcloud.com/products/storage" config="/qingstor/" >}}
{{< provider name="Qiniu Cloud Object Storage (Kodo)" home="https://www.qiniu.com/en/products/kodo" config="/s3/#qiniu" >}}
{{< provider name="Quatrix by Maytech" home="https://www.maytech.net/products/quatrix-business" config="/quatrix/" >}}
{{< provider name="Rackspace Cloud Files" home="https://www.rackspace.com/cloud/files" config="/swift/" >}}
{{< provider name="rsync.net" home="https://rsync.net/products/rclone.html" config="/sftp/#rsync-net" >}}
{{< provider name="Scaleway" home="https://www.scaleway.com/object-storage/" config="/s3/#scaleway" >}}
{{< provider name="Seafile" home="https://www.seafile.com/" config="/seafile/" >}}
{{< provider name="Seagate Lyve Cloud" home="https://www.seagate.com/gb/en/services/cloud/storage/" config="/s3/#lyve" >}}
{{< provider name="SeaweedFS" home="https://github.com/chrislusf/seaweedfs/" config="/s3/#seaweedfs" >}}
{{< provider name="SFTP" home="https://en.wikipedia.org/wiki/SSH_File_Transfer_Protocol" config="/sftp/" >}}
{{< provider name="Sia" home="https://sia.tech/" config="/sia/" >}}
{{< provider name="SMB / CIFS" home="https://en.wikipedia.org/wiki/Server_Message_Block" config="/smb/" >}}
{{< provider name="StackPath" home="https://www.stackpath.com/products/object-storage/" config="/s3/#stackpath" >}}
{{< provider name="Storj" home="https://storj.io/" config="/storj/" >}}
{{< provider name="Synology" home="https://c2.synology.com/en-global/object-storage/overview" config="/s3/#synology-c2" >}}
{{< provider name="SugarSync" home="https://sugarsync.com/" config="/sugarsync/" >}}
{{< provider name="Tencent Cloud Object Storage (COS)" home="https://intl.cloud.tencent.com/product/cos" config="/s3/#tencent-cos" >}}
{{< provider name="Uloz.to" home="https://uloz.to" config="/ulozto/" >}}
{{< provider name="Uptobox" home="https://uptobox.com" config="/uptobox/" >}}
{{< provider name="Wasabi" home="https://wasabi.com/" config="/s3/#wasabi" >}}
{{< provider name="WebDAV" home="https://en.wikipedia.org/wiki/WebDAV" config="/webdav/" >}}
{{< provider name="Yandex Disk" home="https://disk.yandex.com/" config="/yandex/" >}}
{{< provider name="Zoho WorkDrive" home="https://www.zoho.com/workdrive/" config="/zoho/" >}}
{{< provider name="The local filesystem" home="/local/" config="/local/" end="true">}}
{{< /provider_list >}}

## Virtual providers

These backends adapt or modify other storage providers:

{{< provider name="Alias: Rename existing remotes" home="/alias/" config="/alias/" >}}
{{< provider name="Cache: Cache remotes (DEPRECATED)" home="/cache/" config="/cache/" >}}
{{< provider name="Chunker: Split large files" home="/chunker/" config="/chunker/" >}}
{{< provider name="Combine: Combine multiple remotes into a directory tree" home="/combine/" config="/combine/" >}}
{{< provider name="Compress: Compress files" home="/compress/" config="/compress/" >}}
{{< provider name="Crypt: Encrypt files" home="/crypt/" config="/crypt/" >}}
{{< provider name="Hasher: Hash files" home="/hasher/" config="/hasher/" >}}
{{< provider name="Union: Join multiple remotes to work together" home="/union/" config="/union/" >}}


## Links

  * {{< icon "fa fa-home" >}} [Home page](https://rclone.org/)
  * {{< icon "fab fa-github" >}} [GitHub project page for source and bug tracker](https://github.com/rclone/rclone)
  * {{< icon "fa fa-comments" >}} [Rclone Forum](https://forum.rclone.org)
  * {{< icon "fas fa-cloud-download-alt" >}}[Downloads](/downloads/)
