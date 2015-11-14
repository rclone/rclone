---
title: "Dropbox"
description: "Rclone docs for Dropbox"
date: "2014-07-17"
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
What type of source is it?
Choose a number from below
 1) swift
 2) s3
 3) local
 4) google cloud storage
 5) dropbox
 6) drive
type> 5
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

Dropbox doesn't have the capability of storing modification times or
MD5SUMs so syncs will effectively have the `--size-only` flag set.

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
