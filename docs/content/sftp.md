---
title: "SFTP"
description: "SFTP"
date: "2017-02-01"
---

<i class="fa fa-server"></i> SFTP
----------------------------------------

SFTP is the [Secure (or SSH) File Transfer
Protocol](https://en.wikipedia.org/wiki/SSH_File_Transfer_Protocol).

It runs over SSH v2 and is standard with most modern SSH
installations.

Paths are specified as `remote:path`. If the path does not begin with
a `/` it is relative to the home directory of the user.  An empty path
`remote:` refers to the users home directory.

Here is an example of making a SFTP configuration.  First run

    rclone config

This will guide you through an interactive setup process.  You will
need your account number (a short hex number) and key (a long hex
number) which you can get from the SFTP control panel.
```
No remotes found - make a new one
n) New remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
n/r/c/s/q> n
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
12 / SSH/SFTP Connection
   \ "sftp"
13 / Yandex Disk
   \ "yandex"
Storage> 12  
SSH host to connect to
Choose a number from below, or type in your own value
 1 / Connect to example.com
   \ "example.com"
host> example.com
SSH username, leave blank for current username, ncw
user> 
SSH port
port> 
SSH password, leave blank to use ssh-agent
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank
y/g/n> n
Remote config
--------------------
[remote]
host = example.com
user = 
port = 
pass = 
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `remote` and can now be used like this

See all directories in the home directory

    rclone lsd remote:

Make a new directory

    rclone mkdir remote:path/to/directory

List the contents of a directory

    rclone ls remote:path/to/directory

Sync `/home/local/directory` to the remote directory, deleting any
excess files in the directory.

    rclone sync /home/local/directory remote:directory

### Modified time ###

Modified times are stored on the server to 1 second precision.

Modified times are used in syncing and are fully supported.

### Limitations ###

SFTP does not support any checksums.

SFTP isn't supported under plan9 until [this
issue](https://github.com/pkg/sftp/issues/156) is fixed.

Note that since SFTP isn't HTTP based the following flags don't work
with it: `--dump-headers`, `--dump-bodies`, `--dump-auth`

Note that `--timeout` isn't supported (but `--contimeout` is).
