---
title: "Bugs"
description: "Rclone Bugs and Limitations"
date: "2014-06-16"
---

Bugs and Limitations
--------------------

### Empty directories are left behind / not created ##

With remotes that have a concept of directory, eg Local and Drive,
empty directories may be left behind, or not created when one was
expected.

This is because rclone doesn't have a concept of a directory - it only
works on objects.  Most of the object storage systems can't actually
store a directory so there is nowhere for rclone to store anything
about directories.

You can work round this to some extent with the`purge` command which
will delete everything under the path, **inluding** empty directories.

This may be fixed at some point in
[Issue #100](https://github.com/ncw/rclone/issues/100)

### Directory timestamps aren't preserved ##

For the same reason as the above, rclone doesn't have a concept of a
directory - it only works on objects, therefore it can't preserve the
timestamps of directories.
