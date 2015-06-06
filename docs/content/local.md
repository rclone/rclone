---
title: "Local Filesystem"
description: "Rclone docs for the local filesystem"
date: "2014-04-26"
---

<i class="fa fa-file"></i> Local Filesystem
-------------------------------------------

Local paths are specified as normal filesystem paths, eg `/path/to/wherever`, so

    rclone sync /home/source /tmp/destination

Will sync `/home/source` to `/tmp/destination`

These can be configured into the config file for consistencies sake,
but it is probably easier not to.

### Modified time ###

Rclone reads and writes the modified time using an accuracy determined by
the OS.  Typically this is 1ns on Linux, 10 ns on Windows and 1 Second
on OS X.

### Filenames ###

Filenames are expected to be encoded in UTF-8 on disk.  This is the
normal case for Windows and OS X.  There is a bit more uncertainty in
the Linux world, but new distributions will have UTF-8 encoded files
names.

If an invalid (non-UTF8) filename is read, the invalid caracters will
be replaced with the unicode replacement character, '�'.  `rclone`
will emit a debug message in this case (use `-v` to see), eg

```
Local file system at .: Replacing invalid UTF-8 characters in "gro\xdf"
```
