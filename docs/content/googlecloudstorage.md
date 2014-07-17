---
title: "Google Cloud Storage"
description: "Rclone docs for Google Cloud Storage"
date: "2014-07-17"
---

<i class="fa fa-google"></i> Google Cloud Storage
-------------------------------------------------

Paths are specified as `remote:bucket` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, eg `remote:bucket/path/to/dir`.

The initial setup for google cloud storage involves getting a token from Google Cloud Storage
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
type> 4
Google Application Client Id - leave blank to use rclone's.
client_id> 
Google Application Client Secret - leave blank to use rclone's.
client_secret> 
Project number optional - needed only for list/create/delete buckets - see your developer console.
project_number> 12345678
Access Control List for new objects.
Choose a number from below, or type in your own value
 * Object owner gets OWNER access, and all Authenticated Users get READER access.
 1) authenticatedRead
 * Object owner gets OWNER access, and project team owners get OWNER access.
 2) bucketOwnerFullControl
 * Object owner gets OWNER access, and project team owners get READER access.
 3) bucketOwnerRead
 * Object owner gets OWNER access [default if left blank].
 4) private
 * Object owner gets OWNER access, and project team members get access according to their roles.
 5) projectPrivate
 * Object owner gets OWNER access, and all Users get READER access.
 6) publicRead
object_acl> 4
Access Control List for new buckets.
Choose a number from below, or type in your own value
 * Project team owners get OWNER access, and all Authenticated Users get READER access.
 1) authenticatedRead
 * Project team owners get OWNER access [default if left blank].
 2) private
 * Project team members get access according to their roles.
 3) projectPrivate
 * Project team owners get OWNER access, and all Users get READER access.
 4) publicRead
 * Project team owners get OWNER access, and all Users get WRITER access.
 5) publicReadWrite
bucket_acl> 2
Remote config
Go to the following link in your browser
https://accounts.google.com/o/oauth2/auth?access_type=&approval_prompt=&client_id=XXXXXXXXXXXX.apps.googleusercontent.com&redirect_uri=urn%3Aietf%3Awg%3Aoauth%3A2.0%3Aoob&response_type=code&scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fdevstorage.full_control&state=state
Log in, then type paste the token that is returned in the browser here
Enter verification code> x/xxxxxxxxxxxxxxxxxxxxxxxxxxxx.xxxxxxxxxxxxxxxxxxxxxx_xxxxxxxx
--------------------
[remote]
type = google cloud storage
client_id = 
client_secret = 
token = {"AccessToken":"xxxx.xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","RefreshToken":"x/xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx_xxxxxxxxx","Expiry":"2014-07-17T20:49:14.929208288+01:00","Extra":null}
project_number = 12345678
object_acl = private
bucket_acl = private
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `remote` and can now be used like this

See all the buckets in your project

    rclone lsd remote:

Make a new bucket

    rclone mkdir remote:bucket

List the contents of a bucket

    rclone ls remote:bucket

Sync `/home/local/directory` to the remote bucket, deleting any excess
files in the bucket.

    rclone sync /home/local/directory remote:bucket

Modified time
-------------

Google google cloud storage stores md5sums natively and rclone stores
modification times as metadata on the object, under the "mtime" key in
RFC3339 format accurate to 1ns.
