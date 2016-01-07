---
title: "Yandex"
description: "Yandex Disk"
date: "2015-12-30"
---

<i class="fa fa-space-shuttle"></i>Yandex Disk
----------------------------------------

[Yandex Disk](https://disk.yandex.com) is a cloud storage solution created by [Yandex](http://yandex.com).

Yandex paths may be as deep as required, eg `remote:directory/subdirectory`.

Here is an example of making a yandex configuration.  First run

    rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
q) Quit config
n/q> n
name> remote
What type of source is it?
Choose a number from below
 1) amazon cloud drive
 2) b2
 3) drive
 4) dropbox
 5) google cloud storage
 6) swift
 7) hubic
 8) local
 9) onedrive
10) s3
11) yandex
type> 11
Yandex Client Id - leave blank normally.
client_id> 
Yandex Client Secret - leave blank normally.
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
token = {"access_token":"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","token_type":"bearer","expiry":"2016-12-29T12:27:11.362788025Z"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Yandex Disk. This only runs from the moment it
opens your browser to the moment you get back the verification code.
This is on `http://127.0.0.1:53682/` and this it may require you to
unblock it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

See top level directories

    rclone lsd remote:

Make a new directory

    rclone mkdir remote:directory

List the contents of a directory

    rclone ls remote:directory

Sync `/home/local/directory` to the remote path, deleting any
excess files in the path.

    rclone sync /home/local/directory remote:directory

### Modified time ###

Modified times are supported and are stored accurate to 1 ns in custom
metadata called `rclone_modified` in RFC3339 with nanoseconds format.

### MD5 checksums ###

MD5 checksums are natively supported by Yandex Disk.
