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
n) New remote
d) Delete remote
q) Quit config
e/n/d/q> n
name> remote
What type of source is it?
Choose a number from below
 1) amazon cloud drive
 2) drive
 3) dropbox
 4) google cloud storage
 5) local
 6) onedrive
 7) s3
 8) swift
type> 6
Microsoft App Client Id - leave blank normally.
client_id> 
Microsoft App Client Secret - leave blank normally.
client_secret> 
Remote config
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

### Modified time and MD5SUMs ###

One Drive allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.

One drive does not support MD5SUMs. This means the `--checksum` flag
will be equivalent to the `--size-only` flag.

### Deleting files ###

Any files you delete with rclone will end up in the trash.  Microsoft
doesn't provide an API to permanently delete files, nor to empty the
trash, so you will have to do that with one of Microsoft's apps or via
the One Drive website.

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
