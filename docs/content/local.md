---
title: "Local Filesystem"
description: "Rclone docs for the local filesystem"
---

# {{< icon "fas fa-hdd" >}} Local Filesystem

Local paths are specified as normal filesystem paths, e.g. `/path/to/wherever`, so

    rclone sync -i /home/source /tmp/destination

Will sync `/home/source` to `/tmp/destination`.

## Configuration

For consistencies sake one can also configure a remote of type
`local` in the config file, and access the local filesystem using
rclone remote paths, e.g. `remote:path/to/wherever`, but it is probably
easier not to.

### Modified time ###

Rclone reads and writes the modified time using an accuracy determined by
the OS. Typically this is 1ns on Linux, 10 ns on Windows and 1 Second
on OS X.

### Filenames ###

Filenames should be encoded in UTF-8 on disk. This is the normal case
for Windows and OS X.

There is a bit more uncertainty in the Linux world, but new
distributions will have UTF-8 encoded files names. If you are using an
old Linux filesystem with non UTF-8 file names (e.g. latin1) then you
can use the `convmv` tool to convert the filesystem to UTF-8. This
tool is available in most distributions' package managers.

If an invalid (non-UTF8) filename is read, the invalid characters will
be replaced with a quoted representation of the invalid bytes. The name
`gro\xdf` will be transferred as `gro‛DF`. `rclone` will emit a debug
message in this case (use `-v` to see), e.g.

```
Local file system at .: Replacing invalid UTF-8 characters in "gro\xdf"
```

#### Restricted characters

With the local backend, restrictions on the characters that are usable in
file or directory names depend on the operating system. To check what
rclone will replace by default on your system, run `rclone help flags local-encoding`.

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

### Paths on Windows ###

On Windows there are many ways of specifying a path to a file system resource.
Local paths can be absolute, like `C:\path\to\wherever`, or relative,
like `..\wherever`. Network paths in UNC format, `\\server\share`, are also supported.
Path separator can be either `\` (as in `C:\path\to\wherever`) or `/` (as in `C:/path/to/wherever`).
Length of these paths are limited to 259 characters for files and 247
characters for directories, but there is an alternative extended-length
path format increasing the limit to (approximately) 32,767 characters.
This format requires absolute paths and the use of prefix `\\?\`,
e.g. `\\?\D:\some\very\long\path`. For convenience rclone will automatically
convert regular paths into the corresponding extended-length paths,
so in most cases you do not have to worry about this (read more [below](#long-paths)).

Note that Windows supports using the same prefix `\\?\` to
specify path to volumes identified by their GUID, e.g.
`\\?\Volume{b75e2c83-0000-0000-0000-602f00000000}\some\path`.
This is *not* supported in rclone, due to an [issue](https://github.com/golang/go/issues/39785)
in go.

#### Long paths ####

Rclone handles long paths automatically, by converting all paths to
[extended-length path format](https://docs.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation), which allows paths up to 32,767 characters.

This conversion will ensure paths are absolute and prefix them with
the `\\?\`. This is why you will see that your paths, for instance
`.\files` is shown as path `\\?\C:\files` in the output, and `\\server\share`
as `\\?\UNC\server\share`.

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
file exceeds 259 characters on z, so only use this option if you have to.

### Symlinks / Junction points

Normally rclone will ignore symlinks or junction points (which behave
like symlinks under Windows).

If you supply `--copy-links` or `-L` then rclone will follow the
symlink and copy the pointed to file or directory.  Note that this
flag is incompatible with `--links` / `-l`.

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
where it isn't supported (e.g. Windows) it will be ignored.

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/local/local.go then run make backenddocs" >}}
### Advanced options

Here are the advanced options specific to local (Local Disk).

#### --local-nounc

Disable UNC (long path names) conversion on Windows.

Properties:

- Config:      nounc
- Env Var:     RCLONE_LOCAL_NOUNC
- Type:        string
- Required:    false
- Examples:
    - "true"
        - Disables long file names.

#### --copy-links / -L

Follow symlinks and copy the pointed to item.

Properties:

- Config:      copy_links
- Env Var:     RCLONE_LOCAL_COPY_LINKS
- Type:        bool
- Default:     false

#### --links / -l

Translate symlinks to/from regular files with a '.rclonelink' extension.

Properties:

- Config:      links
- Env Var:     RCLONE_LOCAL_LINKS
- Type:        bool
- Default:     false

#### --skip-links

Don't warn about skipped symlinks.

This flag disables warning messages on skipped symlinks or junction
points, as you explicitly acknowledge that they should be skipped.

Properties:

- Config:      skip_links
- Env Var:     RCLONE_LOCAL_SKIP_LINKS
- Type:        bool
- Default:     false

#### --local-zero-size-links

Assume the Stat size of links is zero (and read them instead) (deprecated).

Rclone used to use the Stat size of links as the link size, but this fails in quite a few places:

- Windows
- On some virtual filesystems (such ash LucidLink)
- Android

So rclone now always reads the link.


Properties:

- Config:      zero_size_links
- Env Var:     RCLONE_LOCAL_ZERO_SIZE_LINKS
- Type:        bool
- Default:     false

#### --local-unicode-normalization

Apply unicode NFC normalization to paths and filenames.

This flag can be used to normalize file names into unicode NFC form
that are read from the local filesystem.

Rclone does not normally touch the encoding of file names it reads from
the file system.

This can be useful when using macOS as it normally provides decomposed (NFD)
unicode which in some language (eg Korean) doesn't display properly on
some OSes.

Note that rclone compares filenames with unicode normalization in the sync
routine so this flag shouldn't normally be used.

Properties:

- Config:      unicode_normalization
- Env Var:     RCLONE_LOCAL_UNICODE_NORMALIZATION
- Type:        bool
- Default:     false

#### --local-no-check-updated

Don't check to see if the files change during upload.

Normally rclone checks the size and modification time of files as they
are being uploaded and aborts with a message which starts "can't copy
- source file is being updated" if the file changes during upload.

However on some file systems this modification time check may fail (e.g.
[Glusterfs #2206](https://github.com/rclone/rclone/issues/2206)) so this
check can be disabled with this flag.

If this flag is set, rclone will use its best efforts to transfer a
file which is being updated. If the file is only having things
appended to it (e.g. a log) then rclone will transfer the log file with
the size it had the first time rclone saw it.

If the file is being modified throughout (not just appended to) then
the transfer may fail with a hash check failure.

In detail, once the file has had stat() called on it for the first
time we:

- Only transfer the size that stat gave
- Only checksum the size that stat gave
- Don't update the stat info for the file



Properties:

- Config:      no_check_updated
- Env Var:     RCLONE_LOCAL_NO_CHECK_UPDATED
- Type:        bool
- Default:     false

#### --one-file-system / -x

Don't cross filesystem boundaries (unix/macOS only).

Properties:

- Config:      one_file_system
- Env Var:     RCLONE_LOCAL_ONE_FILE_SYSTEM
- Type:        bool
- Default:     false

#### --local-case-sensitive

Force the filesystem to report itself as case sensitive.

Normally the local backend declares itself as case insensitive on
Windows/macOS and case sensitive for everything else.  Use this flag
to override the default choice.

Properties:

- Config:      case_sensitive
- Env Var:     RCLONE_LOCAL_CASE_SENSITIVE
- Type:        bool
- Default:     false

#### --local-case-insensitive

Force the filesystem to report itself as case insensitive.

Normally the local backend declares itself as case insensitive on
Windows/macOS and case sensitive for everything else.  Use this flag
to override the default choice.

Properties:

- Config:      case_insensitive
- Env Var:     RCLONE_LOCAL_CASE_INSENSITIVE
- Type:        bool
- Default:     false

#### --local-no-preallocate

Disable preallocation of disk space for transferred files.

Preallocation of disk space helps prevent filesystem fragmentation.
However, some virtual filesystem layers (such as Google Drive File
Stream) may incorrectly set the actual file size equal to the
preallocated space, causing checksum and file size checks to fail.
Use this flag to disable preallocation.

Properties:

- Config:      no_preallocate
- Env Var:     RCLONE_LOCAL_NO_PREALLOCATE
- Type:        bool
- Default:     false

#### --local-no-sparse

Disable sparse files for multi-thread downloads.

On Windows platforms rclone will make sparse files when doing
multi-thread downloads. This avoids long pauses on large files where
the OS zeros the file. However sparse files may be undesirable as they
cause disk fragmentation and can be slow to work with.

Properties:

- Config:      no_sparse
- Env Var:     RCLONE_LOCAL_NO_SPARSE
- Type:        bool
- Default:     false

#### --local-no-set-modtime

Disable setting modtime.

Normally rclone updates modification time of files after they are done
uploading. This can cause permissions issues on Linux platforms when 
the user rclone is running as does not own the file uploaded, such as
when copying to a CIFS mount owned by another user. If this option is 
enabled, rclone will no longer update the modtime after copying a file.

Properties:

- Config:      no_set_modtime
- Env Var:     RCLONE_LOCAL_NO_SET_MODTIME
- Type:        bool
- Default:     false

#### --local-encoding

The encoding for the backend.

See the [encoding section in the overview](/overview/#encoding) for more info.

Properties:

- Config:      encoding
- Env Var:     RCLONE_LOCAL_ENCODING
- Type:        MultiEncoder
- Default:     Slash,Dot

## Backend commands

Here are the commands specific to the local backend.

Run them with

    rclone backend COMMAND remote:

The help below will explain what arguments each command takes.

See [the "rclone backend" command](/commands/rclone_backend/) for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](/rc/#backend-command).

### noop

A null operation for testing backend commands

    rclone backend noop remote: [options] [<arguments>+]

This is a test command which has some options
you can try to change the output.

Options:

- "echo": echo the input arguments
- "error": return an error based on option value

{{< rem autogenerated options stop >}}
