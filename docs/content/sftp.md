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

This will guide you through an interactive setup process.

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
 4 / Dropbox
   \ "dropbox"
 5 / Encrypt/Decrypt a remote
   \ "crypt"
 6 / FTP Connection
   \ "ftp"
 7 / Google Cloud Storage (this is not Google Drive)
   \ "google cloud storage"
 8 / Google Drive
   \ "drive"
 9 / Hubic
   \ "hubic"
10 / Local Disk
   \ "local"
11 / Microsoft OneDrive
   \ "onedrive"
12 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
   \ "swift"
13 / SSH/SFTP Connection
   \ "sftp"
14 / Yandex Disk
   \ "yandex"
15 / http Connection
   \ "http"
Storage> sftp
SSH host to connect to
Choose a number from below, or type in your own value
 1 / Connect to example.com
   \ "example.com"
host> example.com
SSH username, leave blank for current username, ncw
user> sftpuser
SSH port, leave blank to use default (22)
port> 
SSH password, leave blank to use ssh-agent.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank
y/g/n> n
Path to unencrypted PEM-encoded private key file, leave blank to use ssh-agent.
key_file> 
Remote config
--------------------
[remote]
host = example.com
user = sftpuser
port = 
pass = 
key_file = 
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

### SSH Authentication ###

The SFTP remote supports 3 authentication methods

  * Password
  * Key file
  * ssh-agent

Key files should be unencrypted PEM-encoded private key files.  For
instance `/home/$USER/.ssh/id_rsa`.

If you don't specify `pass` or `key_file` then it will attempt to
contact an ssh-agent.

### ssh-agent on macOS ###

Note that there seem to be various problems with using an ssh-agent on
macOS due to recent changes in the OS.  The most effective work-around
seems to be to start an ssh-agent in each session, eg

    eval `ssh-agent -s` && ssh-add -A

And then at the end of the session

    eval `ssh-agent -k`

These commands can be used in scripts of course.

### Modified time ###

Modified times are stored on the server to 1 second precision.

Modified times are used in syncing and are fully supported.

### Limitations ###

SFTP supports checksums if the same login has shell access and `md5sum`
or `sha1sum` as well as `echo` are in the remote's PATH.

The only ssh agent supported under Windows is Putty's pagent.

SFTP isn't supported under plan9 until [this
issue](https://github.com/pkg/sftp/issues/156) is fixed.

Note that since SFTP isn't HTTP based the following flags don't work
with it: `--dump-headers`, `--dump-bodies`, `--dump-auth`

Note that `--timeout` isn't supported (but `--contimeout` is).
