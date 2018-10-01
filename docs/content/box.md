---
title: "Box"
description: "Rclone docs for Box"
date: "2015-10-14"
---

<i class="fa fa-archive"></i> Box
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

The initial setup for Box involves getting a token from Box which you
need to do in your browser.  `rclone config` walks you through it.

Here is an example of how to make a remote called `remote`.  First run:

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
12 / Microsoft OneDrive
   \ "onedrive"
13 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
   \ "swift"
14 / SSH/SFTP Connection
   \ "sftp"
15 / Yandex Disk
   \ "yandex"
16 / http Connection
   \ "http"
Storage> box
Box App Client Id - leave blank normally.
client_id> 
Box App Client Secret - leave blank normally.
client_secret> 
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
client_id = 
client_secret = 
token = {"access_token":"XXX","token_type":"bearer","refresh_token":"XXX","expiry":"XXX"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Box. This only runs from the moment it opens
your browser to the moment you get back the verification code.  This
is on `http://127.0.0.1:53682/` and this it may require you to unblock
it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List directories in top level of your Box

    rclone lsd remote:

List all the files in your Box

    rclone ls remote:

To copy a local directory to an Box directory called backup

    rclone copy /home/source remote:backup

### Invalid refresh token ###

According to the [box docs](https://developer.box.com/v2.0/docs/oauth-20#section-6-using-the-access-and-refresh-tokens):

> Each refresh_token is valid for one use in 60 days.

This means that if you

  * Don't use the box remote for 60 days
  * Copy the config file with a box refresh token in and use it in two places
  * Get an error on a token refresh

then rclone will return an error which includes the text `Invalid
refresh token`.

To fix this you will need to use oauth2 again to update the refresh
token.  You can use the methods in [the remote setup
docs](/remote_setup/), bearing in mind that if you use the copy the
config file method, you should not use that remote on the computer you
did the authentication on.

Here is how to do it.

```
$ rclone config
Current remotes:

Name                 Type
====                 ====
remote               box

e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> e
Choose a number from below, or type in an existing value
 1 > remote
remote> remote
--------------------
[remote]
type = box
token = {"access_token":"XXX","token_type":"bearer","refresh_token":"XXX","expiry":"2017-07-08T23:40:08.059167677+01:00"}
--------------------
Edit remote
Value "client_id" = ""
Edit? (y/n)>
y) Yes
n) No
y/n> n
Value "client_secret" = ""
Edit? (y/n)>
y) Yes
n) No
y/n> n
Remote config
Already have a token - refresh?
y) Yes
n) No
y/n> y
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
type = box
token = {"access_token":"YYY","token_type":"bearer","refresh_token":"YYY","expiry":"2017-07-23T12:22:29.259137901+01:00"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Modified time and hashes ###

Box allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.

Box supports SHA1 type hashes, so you can use the `--checksum`
flag.

### Transfers ###

For files above 50MB rclone will use a chunked transfer.  Rclone will
upload up to `--transfers` chunks at the same time (shared among all
the multipart uploads).  Chunks are buffered in memory and are
normally 8MB so increasing `--transfers` will increase memory use.

### Deleting files ###

Depending on the enterprise settings for your user, the item will
either be actually deleted from Box or moved to the trash.

<!--- autogenerated options start - DO NOT EDIT, instead edit fs.RegInfo in backend/box/box.go then run make backenddocs -->
### Standard Options

Here are the standard options specific to box (Box).

#### --box-client-id

Box App Client Id.
Leave blank normally.

- Config:      client_id
- Env Var:     RCLONE_BOX_CLIENT_ID
- Type:        string
- Default:     ""

#### --box-client-secret

Box App Client Secret
Leave blank normally.

- Config:      client_secret
- Env Var:     RCLONE_BOX_CLIENT_SECRET
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to box (Box).

#### --box-upload-cutoff

Cutoff for switching to multipart upload (>= 50MB).

- Config:      upload_cutoff
- Env Var:     RCLONE_BOX_UPLOAD_CUTOFF
- Type:        SizeSuffix
- Default:     50M

#### --box-commit-retries

Max number of times to try committing a multipart file.

- Config:      commit_retries
- Env Var:     RCLONE_BOX_COMMIT_RETRIES
- Type:        int
- Default:     100

<!--- autogenerated options stop -->

### Limitations ###

Note that Box is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

Box file names can't have the `\` character in.  rclone maps this to
and from an identical looking unicode equivalent `＼`.

Box only supports filenames up to 255 characters in length.
