---
title: "Dropbox"
description: "Rclone docs for Dropbox"
date: "2016-02-21"
---

<i class="fa fa-dropbox"></i> Dropbox
---------------------------------

Paths are specified as `remote:path`

Dropbox paths may be as deep as required, eg
`remote:directory/subdirectory`.

The initial setup for dropbox involves getting a token from Dropbox
which you need to do in your browser.  `rclone config` walks you
through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
n) New remote
d) Delete remote
q) Quit config
e/n/d/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
 1 / Amazon Drive
   \ "amazon cloud drive"
 2 / Amazon S3 (also Dreamhost, Ceph, Minio)
   \ "s3"
 3 / Backblaze B2
   \ "b2"
 4 / Dropbox
   \ "dropbox"
 5 / Encrypt/Decrypt a remote
   \ "crypt"
 6 / Google Cloud Storage (this is not Google Drive)
   \ "google cloud storage"
 7 / Google Drive
   \ "drive"
 8 / Hubic
   \ "hubic"
 9 / Local Disk
   \ "local"
10 / Microsoft OneDrive
   \ "onedrive"
11 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
   \ "swift"
12 / Yandex Disk
   \ "yandex"
Storage> 4
Dropbox App Key - leave blank normally.
app_key>
Dropbox App Secret - leave blank normally.
app_secret>
Remote config
Please visit:
https://www.dropbox.com/1/oauth2/authorize?client_id=XXXXXXXXXXXXXXX&response_type=code
Enter the code: XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX_XXXXXXXXXX
--------------------
[remote]
app_key =
app_secret =
token = XXXXXXXXXXXXXXXXXXXXXXXXXXXXX_XXXX_XXXXXXXXXXXXXXXXXXXXXXXXXXXXX
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

You can then use it like this,

List directories in top level of your dropbox

    rclone lsd remote:

List all the files in your dropbox

    rclone ls remote:

To copy a local directory to a dropbox directory called backup

    rclone copy /home/source remote:backup

### Modified time and MD5SUMs ###

Dropbox doesn't provide the ability to set modification times in the
V1 public API, so rclone can't support modified time with Dropbox.

This may change in the future - see these issues for details:

  * [Dropbox V2 API](https://github.com/ncw/rclone/issues/349)
  * [Allow syncs for remotes that can't set modtime on existing objects](https://github.com/ncw/rclone/issues/348)

Dropbox doesn't return any sort of checksum (MD5 or SHA1).

Together that means that syncs to dropbox will effectively have the
`--size-only` flag set.

### Specific options ###

Here are the command line options specific to this cloud storage
system.

#### --dropbox-chunk-size=SIZE ####

Upload chunk size. Max 150M. The default is 128MB.  Note that this
isn't buffered into memory.

### Limitations ###

Note that Dropbox is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

There are some file names such as `thumbs.db` which Dropbox can't
store.  There is a full list of them in the ["Ignored Files" section
of this document](https://www.dropbox.com/en/help/145).  Rclone will
issue an error message `File name disallowed - not uploading` if it
attempt to upload one of those file names, but the sync won't fail.

If you have more than 10,000 files in a directory then `rclone purge
dropbox:dir` will return the error `Failed to purge: There are too
many files involved in this operation`.  As a work-around do an
`rclone delete dropbix:dir` followed by an `rclone rmdir dropbox:dir`.
