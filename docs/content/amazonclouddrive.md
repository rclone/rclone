---
title: "Amazon Cloud Drive"
description: "Rclone docs for Amazon Cloud Drive"
date: "2015-09-06"
---

<i class="fa fa-google"></i> Amazon Cloud Drive
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

The initial setup for Amazon cloud drive involves getting a token from
Amazon which you need to do in your browser.  `rclone config` walks
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
 6) s3
 7) swift
type> 1
Amazon Application Client Id - leave blank to use rclone's.
client_id> 
Amazon Application Client Secret - leave blank to use rclone's.
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
token = {"access_token":"xxxxxxxxxxxxxxxxxxxxxxx","token_type":"bearer","refresh_token":"xxxxxxxxxxxxxxxxxx","expiry":"2015-09-06T16:07:39.658438471+01:00"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Note that rclone runs a webserver on your local machine to collect the
token as returned from Amazon. This only runs from the moment it
opens your browser to the moment you get back the verification
code.  This is on `http://127.0.0.1:53682/` and this it may require
you to unblock it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List directories in top level of your Amazon cloud drive

    rclone lsd remote:

List all the files in your Amazon cloud drive

    rclone ls remote:

To copy a local directory to an Amazon cloud drive directory called backup

    rclone copy /home/source remote:backup

### Modified time and MD5SUMs ###

Amazon cloud drive doesn't allow modification times to be changed via
the API so these won't be accurate or used for syncing.

It does store MD5SUMs so for a more accurate sync, you can use the
`--checksum` flag.

### Deleting files ###

Any files you delete with rclone will end up in the trash.  Amazon
don't provide an API to permanently delete files, nor to empty the
trash, so you will have to do that with one of Amazon's apps or via
the Amazon cloud drive website.

### Limitations ###

Note that Amazon cloud drive is case sensitive so you can't have a
file called "Hello.doc" and one called "hello.doc".

Amazon cloud drive has rate limiting so you may notice errors in the
sync (429 errors).  rclone will automatically retry the sync up to 3
times by default (see `--retries` flag) which should hopefully work
around this problem.
