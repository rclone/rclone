---
title: "KVFS"
description: "Rclone docs for KVFS"
versionIntroduced: "v1.70"
status: Beta
---

# {{< icon "<i class="fa-regular fa-key"></i>"}} KVFS


## Configuration

The initial setup for an KVFS backend. 

`rclone config` walks you through the backend configuration.  
**<ins>Notice:</ins>** the configuration process will create the kvfs folder with the database file if it doesn't exist.

Here is an example of how to make a remote called `kvfs`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> kvfs1
Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
[snip]
XX / kv based filesystem
   \ (kvfs)
[snip]
Storage> kvfs

Option config_dir.
Where you would like to store the kvfs kv db (e.g. /tmp/my/kv) ?
Enter a value of type string. Press Enter for the default (~/.config/rclone/kvfs).
config_dir> /tmp/my/kv

Edit advanced config?
y) Yes
n) No (default)
y/n> n

Configuration complete.
Options:
- type: kvfs
- config_dir: /tmp/my/kv
Keep this "kvfs1" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
```
