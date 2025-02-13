---
title: "SQLite"
description: "Rclone docs for SQLite"
versionIntroduced: "v1.70"
status: Beta
---

# {{< icon "<i class="fa-light fa-database"></i>" >}} SQLite


## Configuration

The initial setup for an SQLite backend. 

`rclone config` walks you through the backend configuration.  
**<ins>Notice:</ins>** the configuration will create the sqlite database file if it doesn't exist and will also create the relevant tables.

Here is an example of how to make a remote called `sqlite`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> sqlite
Option Storage.
Type of storage to configure.
Choose a number from below, or type in your own value.
[snip]
XX / sqlite
   \ (sqlite)
[snip]
Storage> sqlite
Option sqlite_file.
Enter the full path to the SQLite database file. e.g. /tmp/rclone_sqlite.db
Enter a value of type string. Press Enter for the default (~/.config/rclone/sqlite.db).
sqlite_file> /tmp/rclone_sqlite.db
Edit advanced config?
y) Yes
n) No (default)
y/n> n
Configuration complete.
Options:
- type: sqlite
- sqlite_file: /tmp/rclone_sqlite.db
Keep this "sqlite2" remote?
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```
