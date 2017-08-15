---
title: "Microsoft Azure Blob Storage"
description: "Rclone docs for Microsoft Azure Blob Storage"
date: "2017-07-30"
---

<i class="fa fa-windows"></i> Microsoft Azure Blob Storage
-----------------------------------------

Paths are specified as `remote:container` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, eg
`remote:container/path/to/dir`.

Here is an example of making a Microsoft Azure Blob Storage
configuration.  For a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
 1 / Amazon Drive
   \ "amazon cloud drive"
 2 / Amazon S3 (also Dreamhost, Ceph, Minio)
   \ "s3"
 3 / Backblaze B2
   \ "b2"
 4 / Box
   \ "box"
 5 / Dropbox
   \ "dropbox"
 6 / Encrypt/Decrypt a remote
   \ "crypt"
 7 / FTP Connection
   \ "ftp"
 8 / Google Cloud Storage (this is not Google Drive)
   \ "google cloud storage"
 9 / Google Drive
   \ "drive"
10 / Hubic
   \ "hubic"
11 / Local Disk
   \ "local"
12 / Microsoft Azure Blob Storage
   \ "azureblob"
13 / Microsoft OneDrive
   \ "onedrive"
14 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
   \ "swift"
15 / SSH/SFTP Connection
   \ "sftp"
16 / Yandex Disk
   \ "yandex"
17 / http Connection
   \ "http"
Storage> azureblob
Storage Account Name
account> account_name
Storage Account Key
key> base64encodedkey==
Endpoint for the service - leave blank normally.
endpoint> 
Remote config
--------------------
[remote]
account = account_name
key = base64encodedkey==
endpoint = 
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See all containers

    rclone lsd remote:

Make a new container

    rclone mkdir remote:container

List the contents of a container

    rclone ls remote:container

Sync `/home/local/directory` to the remote container, deleting any excess
files in the container.

    rclone sync /home/local/directory remote:container

### --fast-list ###

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](/docs/#fast-list) for more details.

### Modified time ###

The modified time is stored as metadata on the object with the `mtime`
key.  It is stored using RFC3339 Format time with nanosecond
precision.  The metadata is supplied during directory listings so
there is no overhead to using it.

### Hashes ###

MD5 hashes are stored with blobs.  However blobs that were uploaded in
chunks only have an MD5 if the source remote was capable of MD5
hashes, eg the local disk.

### Multipart uploads ###

Rclone supports multipart uploads with Azure Blob storage.  Files
bigger than 256MB will be uploaded using chunked upload by default.

The files will be uploaded in parallel in 4MB chunks (by default).
Note that these chunks are buffered in memory and there may be up to
`--transfers` of them being uploaded at once.

Files can't be split into more than 50,000 chunks so by default, so
the largest file that can be uploaded with 4MB chunk size is 195GB.
Above this rclone will double the chunk size until it creates less
than 50,000 chunks.  By default this will mean a maximum file size of
3.2TB can be uploaded.  This can be raised to 5TB using
`--azureblob-chunk-size 100M`.

Note that rclone doesn't commit the block list until the end of the
upload which means that there is a limit of 9.5TB of multipart uploads
in progress as Azure won't allow more than that amount of uncommitted
blocks.

### Specific options ###

Here are the command line options specific to this cloud storage
system.

#### --azureblob-upload-cutoff=SIZE ####

Cutoff for switching to chunked upload - must be <= 256MB. The default
is 256MB.

#### --azureblob-chunk-size=SIZE ####

Upload chunk size.  Default 4MB.  Note that this is stored in memory
and there may be up to `--transfers` chunks stored at once in memory.
This can be at most 100MB.

### Limitations ###

MD5 sums are only uploaded with chunked files if the source has an MD5
sum.  This will always be the case for a local to azure copy.
