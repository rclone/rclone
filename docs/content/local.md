---
title: "Local Filesystem"
description: "Rclone docs for the local filesystem"
date: "2014-04-26"
---

Local Filesystem
----------------

Local paths are specified as normal filesystem paths, eg `/path/to/wherever`, so

    rclone sync /home/source /tmp/destination

Will sync `/home/source` to `/tmp/destination`

These can be configured into the config file for consistencies sake,
but it is probably easier not to.

Modified time
-------------

Rclone reads and writes the modified time using an accuracy determined by
the OS.  Typically this is 1ns on Linux, 10 ns on Windows and 1 Second
on OS X.

