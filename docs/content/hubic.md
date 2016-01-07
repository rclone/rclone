---
title: "Hubic"
description: "Rclone docs for Hubic"
date: "2015-11-08"
---

<i class="fa fa-space-shuttle"></i> Hubic
-----------------------------------------

Paths are specified as `remote:path`

Paths are specified as `remote:container` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, eg `remote:container/path/to/dir`.

The initial setup for Hubic involves getting a token from Hubic which
you need to do in your browser.  `rclone config` walks you through it.

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
 7) hubic
 8) s3
 9) swift
type> 7
Hubic App Client Id - leave blank normally.
client_id> 
Hubic App Client Secret - leave blank normally.
client_secret> 
Remote config
If your browser doesn't open automatically go to the following link: http://localhost:53682/auth
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
token as returned from Hubic. This only runs from the moment it opens
your browser to the moment you get back the verification code.  This
is on `http://127.0.0.1:53682/` and this it may require you to unblock
it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List containers in the top level of your Hubic

    rclone lsd remote:

List all the files in your Hubic

    rclone ls remote:

To copy a local directory to an Hubic directory called backup

    rclone copy /home/source remote:backup

### Modified time ###

The modified time is stored as metadata on the object as
`X-Object-Meta-Mtime` as floating point since the epoch accurate to 1
ns.

This is a defacto standard (used in the official python-swiftclient
amongst others) for storing the modification time for an object.

Note that Hubic wraps the Swift backend, so most of the properties of
are the same.

### Limitations ###

Code to refresh the OpenStack token isn't done yet which may cause
problems with very long transfers.
