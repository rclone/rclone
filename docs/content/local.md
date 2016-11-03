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
normal case for Windows and OS X.

There is a bit more uncertainty in the Linux world, but new
distributions will have UTF-8 encoded files names. If you are using an
old Linux filesystem with non UTF-8 file names (eg latin1) then you
can use the `convmv` tool to convert the filesystem to UTF-8. This
tool is available in most distributions' package managers.

If an invalid (non-UTF8) filename is read, the invalid caracters will
be replaced with the unicode replacement character, '�'.  `rclone`
will emit a debug message in this case (use `-v` to see), eg

```
Local file system at .: Replacing invalid UTF-8 characters in "gro\xdf"
```

### Long paths on Windows ###

Rclone handles long paths automatically, by converting all paths to long
[UNC paths](https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath)
which allows paths up to 32,767 characters.

This is why you will see that your paths, for instance `c:\files` is
converted to the UNC path `\\?\c:\files` in the output,
and `\\server\share` is converted to `\\?\UNC\server\share`.

However, in rare cases this may cause problems with buggy file
system drivers like [EncFS](https://github.com/ncw/rclone/issues/261).
To disable UNC conversion globally, add this to your `.rclone.conf` file:

```
[local]
nounc = true
```

If you want to selectively disable UNC, you can add it to a separate entry like this:

```
[nounc]
type = local
nounc = true
```
And use rclone like this:

`rclone copy c:\src nounc:z:\dst`

This will use UNC paths on `c:\src` but not on `z:\dst`.
Of course this will cause problems if the absolute path length of a
file exceeds 258 characters on z, so only use this option if you have to.

### Specific options ###

Here are the command line options specific to local storage

#### --one-file-system, -x ####

This tells rclone to stay in the filesystem specified by the root and
not to recurse into different file systems.

For example if you have a directory heirachy like this

```
root
├── disk1     - disk1 mounted on the root
│   └── file3 - stored on disk1
├── disk2     - disk2 mounted on the root
│   └── file4 - stored on disk12
├── file1     - stored on the root disk
└── file2     - stored on the root disk
```

Using `rclone --one-file-system copy root remote:` will only copy `file1` and `file2`.  Eg

```
$ rclone -q --one-file-system ls root
        0 file1
        0 file2
```

```
$ rclone -q ls root
        0 disk1/file3
        0 disk2/file4
        0 file1
        0 file2
```

**NB** Rclone (like most unix tools such as `du`, `rsync` and `tar`)
treats a bind mount to the same device as being on the same
filesystem.

**NB** This flag is only available on Unix based systems.  On systems
where it isn't supported (eg Windows) it will not appear as an valid
flag.
