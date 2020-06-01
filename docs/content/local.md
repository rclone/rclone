---
title: "Local Filesystem"
description: "Rclone docs for the local filesystem"
---

{{< icon "fas fa-hdd" >}} Local Filesystem
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

Filenames should be encoded in UTF-8 on disk. This is the normal case
for Windows and OS X.

There is a bit more uncertainty in the Linux world, but new
distributions will have UTF-8 encoded files names. If you are using an
old Linux filesystem with non UTF-8 file names (eg latin1) then you
can use the `convmv` tool to convert the filesystem to UTF-8. This
tool is available in most distributions' package managers.

If an invalid (non-UTF8) filename is read, the invalid characters will
be replaced with a quoted representation of the invalid bytes. The name
`gro\xdf` will be transferred as `gro‛DF`. `rclone` will emit a debug
message in this case (use `-v` to see), eg

```
Local file system at .: Replacing invalid UTF-8 characters in "gro\xdf"
```

#### Restricted characters

On non Windows platforms the following characters are replaced when
handling file names.

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| /         | 0x2F  | ／           |

When running on Windows the following characters are replaced. This
list is based on the [Windows file naming conventions](https://docs.microsoft.com/de-de/windows/desktop/FileIO/naming-a-file#naming-conventions).

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| SOH       | 0x01  | ␁           |
| STX       | 0x02  | ␂           |
| ETX       | 0x03  | ␃           |
| EOT       | 0x04  | ␄           |
| ENQ       | 0x05  | ␅           |
| ACK       | 0x06  | ␆           |
| BEL       | 0x07  | ␇           |
| BS        | 0x08  | ␈           |
| HT        | 0x09  | ␉           |
| LF        | 0x0A  | ␊           |
| VT        | 0x0B  | ␋           |
| FF        | 0x0C  | ␌           |
| CR        | 0x0D  | ␍           |
| SO        | 0x0E  | ␎           |
| SI        | 0x0F  | ␏           |
| DLE       | 0x10  | ␐           |
| DC1       | 0x11  | ␑           |
| DC2       | 0x12  | ␒           |
| DC3       | 0x13  | ␓           |
| DC4       | 0x14  | ␔           |
| NAK       | 0x15  | ␕           |
| SYN       | 0x16  | ␖           |
| ETB       | 0x17  | ␗           |
| CAN       | 0x18  | ␘           |
| EM        | 0x19  | ␙           |
| SUB       | 0x1A  | ␚           |
| ESC       | 0x1B  | ␛           |
| FS        | 0x1C  | ␜           |
| GS        | 0x1D  | ␝           |
| RS        | 0x1E  | ␞           |
| US        | 0x1F  | ␟           |
| /         | 0x2F  | ／           |
| "         | 0x22  | ＂           |
| *         | 0x2A  | ＊           |
| :         | 0x3A  | ：           |
| <         | 0x3C  | ＜           |
| >         | 0x3E  | ＞           |
| ?         | 0x3F  | ？           |
| \         | 0x5C  | ＼           |
| \|        | 0x7C  | ｜           |

File names on Windows can also not end with the following characters.
These only get replaced if they are the last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |
| .         | 0x2E  | ．           |

Invalid UTF-8 bytes will also be [replaced](/overview/#invalid-utf8),
as they can't be converted to UTF-16.

### Long paths on Windows ###

Rclone handles long paths automatically, by converting all paths to long
[UNC paths](https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath)
which allows paths up to 32,767 characters.

This is why you will see that your paths, for instance `c:\files` is
converted to the UNC path `\\?\c:\files` in the output,
and `\\server\share` is converted to `\\?\UNC\server\share`.

However, in rare cases this may cause problems with buggy file
system drivers like [EncFS](https://github.com/rclone/rclone/issues/261).
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

### Symlinks / Junction points

Normally rclone will ignore symlinks or junction points (which behave
like symlinks under Windows).

If you supply `--copy-links` or `-L` then rclone will follow the
symlink and copy the pointed to file or directory.  Note that this
flag is incompatible with `-links` / `-l`.

This flag applies to all commands.

For example, supposing you have a directory structure like this

```
$ tree /tmp/a
/tmp/a
├── b -> ../b
├── expected -> ../expected
├── one
└── two
    └── three
```

Then you can see the difference with and without the flag like this

```
$ rclone ls /tmp/a
        6 one
        6 two/three
```

and

```
$ rclone -L ls /tmp/a
     4174 expected
        6 one
        6 two/three
        6 b/two
        6 b/one
```

#### --links, -l 

Normally rclone will ignore symlinks or junction points (which behave
like symlinks under Windows).

If you supply this flag then rclone will copy symbolic links from the local storage,
and store them as text files, with a '.rclonelink' suffix in the remote storage.

The text file will contain the target of the symbolic link (see example).

This flag applies to all commands.

For example, supposing you have a directory structure like this

```
$ tree /tmp/a
/tmp/a
├── file1 -> ./file4
└── file2 -> /home/user/file3
```

Copying the entire directory with '-l'

```
$ rclone copyto -l /tmp/a/file1 remote:/tmp/a/
```

The remote files are created with a '.rclonelink' suffix

```
$ rclone ls remote:/tmp/a
       5 file1.rclonelink
      14 file2.rclonelink
```

The remote files will contain the target of the symbolic links

```
$ rclone cat remote:/tmp/a/file1.rclonelink
./file4

$ rclone cat remote:/tmp/a/file2.rclonelink
/home/user/file3
```

Copying them back with '-l'

```
$ rclone copyto -l remote:/tmp/a/ /tmp/b/

$ tree /tmp/b
/tmp/b
├── file1 -> ./file4
└── file2 -> /home/user/file3
```

However, if copied back without '-l'

```
$ rclone copyto remote:/tmp/a/ /tmp/b/

$ tree /tmp/b
/tmp/b
├── file1.rclonelink
└── file2.rclonelink
````

Note that this flag is incompatible with `-copy-links` / `-L`.

### Restricting filesystems with --one-file-system

Normally rclone will recurse through filesystems as mounted.

However if you set `--one-file-system` or `-x` this tells rclone to
stay in the filesystem specified by the root and not to recurse into
different file systems.

For example if you have a directory hierarchy like this

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
where it isn't supported (eg Windows) it will be ignored.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/local/local.go then run make backenddocs" >}}
### Standard Options

Here are the standard options specific to local (Local Disk).

#### --local-nounc

Disable UNC (long path names) conversion on Windows

- Config:      nounc
- Env Var:     RCLONE_LOCAL_NOUNC
- Type:        string
- Default:     ""
- Examples:
    - "true"
        - Disables long file names

### Advanced Options

Here are the advanced options specific to local (Local Disk).

#### --copy-links / -L

Follow symlinks and copy the pointed to item.

- Config:      copy_links
- Env Var:     RCLONE_LOCAL_COPY_LINKS
- Type:        bool
- Default:     false

#### --links / -l

Translate symlinks to/from regular files with a '.rclonelink' extension

- Config:      links
- Env Var:     RCLONE_LOCAL_LINKS
- Type:        bool
- Default:     false

#### --skip-links

Don't warn about skipped symlinks.
This flag disables warning messages on skipped symlinks or junction
points, as you explicitly acknowledge that they should be skipped.

- Config:      skip_links
- Env Var:     RCLONE_LOCAL_SKIP_LINKS
- Type:        bool
- Default:     false

#### --local-no-unicode-normalization

Don't apply unicode normalization to paths and filenames (Deprecated)

This flag is deprecated now.  Rclone no longer normalizes unicode file
names, but it compares them with unicode normalization in the sync
routine instead.

- Config:      no_unicode_normalization
- Env Var:     RCLONE_LOCAL_NO_UNICODE_NORMALIZATION
- Type:        bool
- Default:     false

#### --local-no-check-updated

Don't check to see if the files change during upload

Normally rclone checks the size and modification time of files as they
are being uploaded and aborts with a message which starts "can't copy
- source file is being updated" if the file changes during upload.

However on some file systems this modification time check may fail (eg
[Glusterfs #2206](https://github.com/rclone/rclone/issues/2206)) so this
check can be disabled with this flag.

- Config:      no_check_updated
- Env Var:     RCLONE_LOCAL_NO_CHECK_UPDATED
- Type:        bool
- Default:     false

#### --one-file-system / -x

Don't cross filesystem boundaries (unix/macOS only).

- Config:      one_file_system
- Env Var:     RCLONE_LOCAL_ONE_FILE_SYSTEM
- Type:        bool
- Default:     false

#### --local-case-sensitive

Force the filesystem to report itself as case sensitive.

Normally the local backend declares itself as case insensitive on
Windows/macOS and case sensitive for everything else.  Use this flag
to override the default choice.

- Config:      case_sensitive
- Env Var:     RCLONE_LOCAL_CASE_SENSITIVE
- Type:        bool
- Default:     false

#### --local-case-insensitive

Force the filesystem to report itself as case insensitive

Normally the local backend declares itself as case insensitive on
Windows/macOS and case sensitive for everything else.  Use this flag
to override the default choice.

- Config:      case_insensitive
- Env Var:     RCLONE_LOCAL_CASE_INSENSITIVE
- Type:        bool
- Default:     false

#### --local-no-sparse

Disable sparse files for multi-thread downloads

On Windows platforms rclone will make sparse files when doing
multi-thread downloads. This avoids long pauses on large files where
the OS zeros the file. However sparse files may be undesirable as they
cause disk fragmentation and can be slow to work with.

- Config:      no_sparse
- Env Var:     RCLONE_LOCAL_NO_SPARSE
- Type:        bool
- Default:     false

#### --local-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_LOCAL_ENCODING
- Type:        MultiEncoder
- Default:     Slash,Dot

### Backend commands

Here are the commands specific to the local backend.

Run them with

    rclone backend COMMAND remote:

The help below will explain what arguments each command takes.

See [the "rclone backend" command](/commands/rclone_backend/) for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](/rc/#backend/command).

#### noop

A null operation for testing backend commands

    rclone backend noop remote: [options] [<arguments>+]

This is a test command which has some options
you can try to change the output.

Options:

- "echo": echo the input arguments
- "error": return an error based on option value

{{< rem autogenerated options stop >}}
