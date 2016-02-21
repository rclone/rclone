---
title: "Microsoft One Drive"
description: "Rclone docs for Microsoft One Drive"
date: "2015-10-14"
---

<i class="fa fa-windows"></i> Microsoft One Drive
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

The initial setup for One Drive involves getting a token from
Microsoft which you need to do in your browser.  `rclone config` walks
you through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
n/s> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
 1 / Amazon Cloud Drive
   \ "amazon cloud drive"
 2 / Amazon S3 (also Dreamhost, Ceph)
   \ "s3"
 3 / Backblaze B2
   \ "b2"
 4 / Dropbox
   \ "dropbox"
 5 / Google Cloud Storage (this is not Google Drive)
   \ "google cloud storage"
 6 / Google Drive
   \ "drive"
 7 / Hubic
   \ "hubic"
 8 / Local Disk
   \ "local"
 9 / Microsoft OneDrive
   \ "onedrive"
10 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
   \ "swift"
11 / Yandex Disk
   \ "yandex"
Storage> 9
Microsoft App Client Id - leave blank normally.
client_id> 
Microsoft App Client Secret - leave blank normally.
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
token = {"access_token":"XXXXXX"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Microsoft. This only runs from the moment it
opens your browser to the moment you get back the verification
code.  This is on `http://127.0.0.1:53682/` and this it may require
you to unblock it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List directories in top level of your One Drive

    rclone lsd remote:

List all the files in your One Drive

    rclone ls remote:

To copy a local directory to an One Drive directory called backup

    rclone copy /home/source remote:backup

### Modified time and hashes ###

One Drive allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.

One drive supports SHA1 type hashes, so you can use `--checksum` flag.


### Deleting files ###

Any files you delete with rclone will end up in the trash.  Microsoft
doesn't provide an API to permanently delete files, nor to empty the
trash, so you will have to do that with one of Microsoft's apps or via
the One Drive website.

### Specific options ###

Here are the command line options specific to this cloud storage
system.

#### --onedrive-chunk-size=SIZE ####

Above this size files will be chunked - must be multiple of 320k. The
default is 10MB.  Note that the chunks will be buffered into memory.

#### --onedrive-upload-cutoff=SIZE ####

Cutoff for switching to chunked upload - must be <= 100MB. The default
is 10MB.

### Limitations ###

Note that One Drive is case insensitive so you can't have a
file called "Hello.doc" and one called "hello.doc".

Rclone only supports your default One Drive, and doesn't work with One
Drive for business.  Both these issues may be fixed at some point
depending on user demand!

There are quite a few characters that can't be in One Drive file
names.  These can't occur on Windows platforms, but on non-Windows
platforms they are common.  Rclone will map these names to and from an
identical looking unicode equivalent.  For example if a file has a `?`
in it will be mapped to `？` instead.
