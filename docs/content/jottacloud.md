---
title: "Jottacloud"
description: "Rclone docs for Jottacloud"
date: "2018-08-07"
---

<i class="fa fa-archive"></i> Jottacloud
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

To configure Jottacloud you will need to enter your username and password and select a mountpoint.

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
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
13 / JottaCloud
   \ "jottacloud"
[snip]
Storage> jottacloud
User Name
Enter a string value. Press Enter for the default ("").
user> user
Password.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank
y/g/n> y
Enter the password:
password:
Confirm the password:
password:
The mountpoint to use.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / Will be synced by the official client.
   \ "Sync"
 2 / Archive
   \ "Archive"
mountpoint> Archive
Remote config
--------------------
[remote]
type = jottacloud
user = user
pass = *** ENCRYPTED ***
mountpoint = Archive
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```
Once configured you can then use `rclone` like this,

List directories in top level of your Jottacloud

    rclone lsd remote:

List all the files in your Jottacloud

    rclone ls remote:

To copy a local directory to an Jottacloud directory called backup

    rclone copy /home/source remote:backup


### Modified time and hashes ###

Jottacloud allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.

Jottacloud supports MD5 type hashes, so you can use the `--checksum`
flag.

### Deleting files ###

Any files you delete with rclone will end up in the trash. Due to a lack of API documentation emptying the trash is currently only possible via the Jottacloud website.

### Versions ###

Jottacloud supports file versioning. When rclone uploads a new version of a file it creates a new version of it. Currently rclone only supports retrieving the current version but older versions can be accessed via the Jottacloud Website.

### Limitations ###

Note that Jottacloud is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

There are quite a few characters that can't be in Jottacloud file names. Rclone will map these names to and from an identical looking unicode equivalent. For example if a file has a ? in it will be mapped to ？ instead.

Jottacloud only supports filenames up to 255 characters in length.

### Troubleshooting ###

Jottacloud exhibits some inconsistent behaviours regarding deleted files and folders which may cause Copy, Move and DirMove operations to previously deleted paths to fail. Emptying the trash should help in such cases.