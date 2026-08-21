---
title: "Overview of cloud storage systems"
description: "Overview of cloud storage systems"
type: page
---

# Overview of cloud storage systems

Each cloud storage system is slightly different.  Rclone attempts to
provide a unified interface to them, but some underlying differences
show through.

## Features

Here is an overview of the major features of each cloud storage system.

{{< features-table >}}

### Hash

The cloud storage system supports various hash types of the objects.
The hashes are used when transferring data as an integrity check and
can be specifically used with the `--checksum` flag in syncs and in
the `check` command.

To use the verify checksums when transferring between cloud storage
systems they must support a common hash type.

### ModTime

Almost all cloud storage systems store some sort of timestamp
on objects, but several of them not something that is appropriate
to use for syncing. E.g. some backends will only write a timestamp
that represents the time of the upload. To be relevant for syncing
it should be able to store the modification time of the source
object. If this is not the case, rclone will only check the file
size by default, though can be configured to check the file hash
(with the `--checksum` flag). Ideally it should also be possible to
change the timestamp of an existing file without having to re-upload it.

| Key | Explanation |
|-----|-------------|
| `-` | ModTimes not supported - times likely the upload time |
| `R` | ModTimes supported on files but can't be changed without re-upload |
| `R/W` | Read and Write ModTimes fully supported on files |
| `DR` | ModTimes supported on files and directories but can't be changed without re-upload |
| `DR/W` | Read and Write ModTimes fully supported on files and directories |

Storage systems with a `-` in the ModTime column, means the
modification read on objects is not the modification time of the
file when uploaded. It is most likely the time the file was uploaded,
or possibly something else (like the time the picture was taken in
Google Photos).

Storage systems with a `R` (for read-only) in the ModTime column,
means the it keeps modification times on objects, and updates them
when uploading objects, but it does not support changing only the
modification time (`SetModTime` operation) without re-uploading,
possibly not even without deleting existing first. Some operations
in rclone, such as `copy` and `sync` commands, will automatically
check for `SetModTime` support and re-upload if necessary to keep
the modification times in sync. Other commands will not work
without `SetModTime` support, e.g. `touch` command on an existing
file will fail, and changes to modification time only on a files
in a `mount` will be silently ignored.

Storage systems with `R/W` (for read/write) in the ModTime column,
means they do also support modtime-only operations.

Storage systems with `D` in the ModTime column means that the
following symbols apply to directories as well as files.

### Case Insensitive

If a cloud storage systems is case sensitive then it is possible to
have two files which differ only in case, e.g. `file.txt` and
`FILE.txt`.  If a cloud storage system is case insensitive then that
isn't possible.

This can cause problems when syncing between a case insensitive
system and a case sensitive system.  The symptom of this is that no
matter how many times you run the sync it never completes fully.

The local filesystem and SFTP may or may not be case sensitive
depending on OS.

- Windows - usually case insensitive, though case is preserved
- OSX - usually case insensitive, though it is possible to format case sensitive
- Linux - usually case sensitive, but there are case insensitive file systems
  (e.g. FAT formatted USB keys)

Most of the time this doesn't cause any problems as people tend to
avoid files whose name differs only by case even on case sensitive
systems.

### Duplicate files

If a cloud storage system allows duplicate files then it can have two
objects with the same name.

This confuses rclone greatly when syncing - use the `rclone dedupe`
command to rename or remove duplicates.

### Restricted filenames

Some cloud storage systems might have restrictions on the characters
that are usable in file or directory names.
When `rclone` detects such a name during a file upload, it will
transparently replace the restricted characters with similar looking
Unicode characters. To handle the different sets of restricted characters
for different backends, rclone uses something it calls [encoding](#encoding).

This process is designed to avoid ambiguous file names as much as
possible and allow to move files between many cloud storage systems
transparently.

The name shown by `rclone` to the user or during log output will only
contain a minimal set of [replaced characters](#restricted-characters)
to ensure correct formatting and not necessarily the actual name used
on the cloud storage.

This transformation is reversed when downloading a file or parsing
`rclone` arguments. For example, when uploading a file named `my file?.txt`
to Onedrive, it will be displayed as `my file?.txt` on the console, but
stored as `my file？.txt` to Onedrive (the `?` gets replaced by the similar
looking `？` character, the so-called "fullwidth question mark").
The reverse transformation allows to read a file `unusual/name.txt`
from Google Drive, by passing the name `unusual／name.txt` on the command line
(the `/` needs to be replaced by the similar looking `／` character).

#### Caveats {#restricted-filenames-caveats}

The filename encoding system works well in most cases, at least
where file names are written in English or similar languages.
You might not even notice it: It just works. In some cases it may
lead to issues, though. E.g. when file names are written in Chinese,
or Japanese, where it is always the Unicode fullwidth variants of the
punctuation marks that are used.

On Windows, the characters `:`, `*` and `?` are examples of restricted
characters. If these are used in filenames on a remote that supports it,
Rclone will transparently convert them to their fullwidth Unicode
variants `＊`, `？` and `：` when downloading to Windows, and back again
when uploading. This way files with names that are not allowed on Windows
can still be stored.

However, if you have files on your Windows system originally with these same
Unicode characters in their names, they will be included in the same conversion
process. E.g. if you create a file in your Windows filesystem with name
`Test：1.jpg`, where `：` is the Unicode fullwidth colon symbol, and use
rclone to upload it to Google Drive, which supports regular `:` (halfwidth
question mark), rclone will replace the fullwidth `:` with the
halfwidth `:` and store the file as `Test:1.jpg` in Google Drive. Since
both Windows and Google Drive allows the name `Test：1.jpg`, it would
probably be better if rclone just kept the name as is in this case.

With the opposite situation; if you have a file named `Test:1.jpg`,
in your Google Drive, e.g. uploaded from a Linux system where `:` is valid
in file names. Then later use rclone to copy this file to your Windows
computer you will notice that on your local disk it gets renamed
to `Test：1.jpg`. The original filename is not legal on Windows, due to
the `:`, and rclone therefore renames it to make the copy possible.
That is all good. However, this can also lead to an issue: If you already
had a *different* file named `Test：1.jpg` on Windows, and then use rclone
to copy either way. Rclone will then treat the file originally named
`Test:1.jpg` on Google Drive and the file originally named `Test：1.jpg`
on Windows as the same file, and replace the contents from one with the other.

Its virtually impossible to handle all cases like these correctly in all
situations, but by customizing the [encoding option](#encoding), changing the
set of characters that rclone should convert, you should be able to
create a configuration that works well for your specific situation.
See also the [example](/overview/#encoding-example-windows) below.

(Windows was used as an example of a file system with many restricted
characters, and Google drive a storage system with few.)

#### Default restricted characters {#restricted-characters}

The table below shows the characters that are replaced by default.

When a replacement character is found in a filename, this character
will be escaped with the `‛` character to avoid ambiguous file names.
(e.g. a file named `␀.txt` would shown as `‛␀.txt`)

Each cloud storage backend can use a different set of characters,
which will be specified in the documentation for each backend.

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
| DEL       | 0x7F  | ␡           |

The default encoding will also encode these file names as they are
problematic with many cloud storage systems.

| File name | Replacement |
| --------- |:-----------:|
| .         | ．          |
| ..        | ．．         |

#### Invalid UTF-8 bytes {#invalid-utf8}

Some backends only support a sequence of well formed UTF-8 bytes
as file or directory names.

In this case all invalid UTF-8 bytes will be replaced with a quoted
representation of the byte value to allow uploading a file to such a
backend. For example, the invalid byte `0xFE` will be encoded as `‛FE`.

A common source of invalid UTF-8 bytes are local filesystems, that store
names in a different encoding than UTF-8 or UTF-16, like latin1. See the
[local filenames](/local/#filenames) section for details.

#### Encoding option {#encoding}

Most backends have an encoding option, specified as a flag
`--backend-encoding` where `backend` is the name of the backend, or as
a config parameter `encoding` (you'll need to select the Advanced
config in `rclone config` to see it).

This will have default value which encodes and decodes characters in
such a way as to preserve the maximum number of characters (see
above).

However this can be incorrect in some scenarios, for example if you
have a Windows file system with Unicode fullwidth characters
`＊`, `？` or `：`, that you want to remain as those characters on the
remote rather than being translated to regular (halfwidth) `*`, `?` and `:`.

The `--backend-encoding` flags allow you to change that. You can
disable the encoding completely with `--backend-encoding Raw` or set
`encoding = Raw` in the config file.

Encoding takes a comma separated list of encodings. You can see the
list of all possible values by passing an invalid value to this
flag, e.g. `--local-encoding "help"`. The command `rclone help flags encoding`
will show you the defaults for the backends.

| Encoding  | Characters | Encoded as |
| --------- | ---------- | ---------- |
| Asterisk | `*` | `＊` |
| BackQuote | `` ` `` | `｀` |
| BackSlash | `\` | `＼` |
| Colon | `:` | `：` |
| CrLf | CR 0x0D, LF 0x0A | `␍`, `␊` |
| Ctl | All control characters 0x00-0x1F | `␀␁␂␃␄␅␆␇␈␉␊␋␌␍␎␏␐␑␒␓␔␕␖␗␘␙␚␛␜␝␞␟` |
| Del | DEL 0x7F | `␡` |
| Dollar | `$` | `＄` |
| Dot | `.` or `..` as entire string | `．`, `．．` |
| DoubleQuote | `"` | `＂` |
| Exclamation | `!` | `！` |
| Hash | `#` | `＃` |
| InvalidUtf8 | An invalid UTF-8 character (e.g. latin1) | `�` |
| LeftCrLfHtVt | CR 0x0D, LF 0x0A, HT 0x09, VT 0x0B on the left of a string | `␍`, `␊`, `␉`, `␋` |
| LeftPeriod | `.` on the left of a string | `.` |
| LeftSpace | SPACE on the left of a string | `␠` |
| LeftTilde | `~` on the left of a string | `～` |
| LtGt | `<`, `>` | `＜`, `＞` |
| None ¹ | NUL 0x00 | ␀ |
| Percent | `%` | `％` |
| Pipe | \| | `｜` |
| Question | `?` | `？` |
| RightCrLfHtVt | CR 0x0D, LF 0x0A, HT 0x09, VT 0x0B on the right of a string | `␍`, `␊`, `␉`, `␋` |
| RightPeriod | `.` on the right of a string | `.` |
| RightSpace | SPACE on the right of a string | `␠` |
| Semicolon | `;` | `；` |
| SingleQuote | `'` | `＇` |
| Slash | `/` | `／` |
| SquareBracket | `[`, `]` | `［`, `］` |

¹ Encoding from NUL 0x00 to ␀ is always implicit except when using Raw.
It was previously incorrectly documented as disabling encoding,
and to maintain backward compatibility, its behavior has not been changed.

##### Encoding example: FTP

To take a specific example, the FTP backend's default encoding is

```text
--ftp-encoding "Slash,Del,Ctl,RightSpace,Dot"
```

However, let's say the FTP server is running on Windows and can't have
any of the invalid Windows characters in file names. You are backing
up Linux servers to this FTP server which do have those characters in
file names. So you would add the Windows set which are

```text
Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,Ctl,RightSpace,RightPeriod,InvalidUtf8,Dot
```

to the existing ones, giving:

```text
Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,Ctl,RightSpace,RightPeriod,InvalidUtf8,Dot,Del,RightSpace
```

This can be specified using the `--ftp-encoding` flag or using an `encoding`
parameter in the config file.

##### Encoding example: Windows

As another example, take a Windows system where there is a file with
name `Test：1.jpg`, where `：` is the Unicode fullwidth colon symbol.
When using rclone to copy this to a remote which supports `:`,
the regular (halfwidth) colon (such as Google Drive), you will notice
that the file gets renamed to `Test:1.jpg`.

To avoid this you can change the set of characters rclone should convert
for the local filesystem, using command-line argument `--local-encoding`.
Rclone's default behavior on Windows corresponds to

```text
--local-encoding "Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,Ctl,RightSpace,RightPeriod,InvalidUtf8,Dot"
```

If you want to use fullwidth characters `：`, `＊` and `？` in your filenames
without rclone changing them when uploading to a remote, then set the same as
the default value but without `Colon,Question,Asterisk`:

```text
--local-encoding "Slash,LtGt,DoubleQuote,Pipe,BackSlash,Ctl,RightSpace,RightPeriod,InvalidUtf8,Dot"
```

Alternatively, you can disable the conversion of any characters with
`--local-encoding Raw`.

Instead of using command-line argument `--local-encoding`, you may also set it
as [environment variable](/docs/#environment-variables) `RCLONE_LOCAL_ENCODING`,
or [configure](/docs/#configure) a remote of type `local` in your config,
and set the `encoding` option there.

The risk by doing this is that if you have a filename with the regular (halfwidth)
`:`, `*` and `?` in your cloud storage, and you try to download
it to your Windows filesystem, this will fail. These characters are not
valid in filenames on Windows, and you have told rclone not to work around
this by converting them to valid fullwidth variants.

### MIME Type

MIME types (also known as media types) classify types of documents
using a simple text classification, e.g. `text/html` or
`application/pdf`.

Some cloud storage systems support reading (`R`) the MIME type of
objects and some support writing (`W`) the MIME type of objects.

The MIME type can be important if you are serving files directly to
HTTP from the storage system.

If you are copying from a remote which supports reading (`R`) to a
remote which supports writing (`W`) then rclone will preserve the MIME
types.  Otherwise they will be guessed from the extension, or the
remote itself may assign the MIME type.

### Metadata

Backends may or may support reading or writing metadata. They may
support reading and writing system metadata (metadata intrinsic to
that backend) and/or user metadata (general purpose metadata).

The levels of metadata support are

| Key | Explanation |
|-----|-------------|
| `R` | Read only System Metadata on files only|
| `RW` | Read and write System Metadata on files only|
| `RWU` | Read and write System Metadata and read and write User Metadata on files only|
| `DR` | Read only System Metadata on files and directories |
| `DRW` | Read and write System Metadata on files and directories|
| `DRWU` | Read and write System Metadata and read and write User Metadata on files and directories |

See [the metadata docs](/docs/#metadata) for more info.

## Optional Features

All rclone remotes support a base command set. Other features depend
upon backend-specific capabilities.

{{< optional-features-table >}}

### Purge

This deletes a directory quicker than just deleting all the files in
the directory.

### Copy

Used when copying an object to and from the same remote.  This known
as a server-side copy so you can copy a file without downloading it
and uploading it again.  It is used if you use `rclone copy` or
`rclone move` if the remote doesn't support `Move` directly.

If the server doesn't support `Copy` directly then for copy operations
the file is downloaded then re-uploaded.

### Move

Used when moving/renaming an object on the same remote.  This is known
as a server-side move of a file.  This is used in `rclone move` if the
server doesn't support `DirMove`.

If the server isn't capable of `Move` then rclone simulates it with
`Copy` then delete.  If the server doesn't support `Copy` then rclone
will download the file and re-upload it.

### DirMove

This is used to implement `rclone move` to move a directory if
possible.  If it isn't then it will use `Move` on each file (which
falls back to `Copy` then download and upload - see `Move` section).

### CleanUp

This is used for emptying the trash for a remote by `rclone cleanup`.

If the server can't do `CleanUp` then `rclone cleanup` will return an
error.

‡‡ Note that while Box implements this it has to delete every file
individually so it will be slower than emptying the trash via the WebUI

### ListR

The remote supports a recursive list to list all the contents beneath
a directory quickly.  This enables the `--fast-list` flag to work.
See the [rclone docs](/docs/#fast-list) for more details.

### StreamUpload

Some remotes allow files to be uploaded without knowing the file size
in advance. This allows certain operations to work without spooling the
file to local disk first, e.g. `rclone rcat`.

### MultithreadUpload

Some remotes allow transfers to the remote to be sent as chunks in
parallel. If this is supported then rclone will use multi-thread
copying to transfer files much faster.

### LinkSharing

Sets the necessary permissions on a file or folder and prints a link
that allows others to access them, even if they don't have an account
on the particular cloud provider.

### About

Rclone `about` prints quota information for a remote. Typical output
includes bytes used, free, quota and in trash.

If a remote lacks about capability `rclone about remote:`returns
an error.

Backends without about capability cannot determine free space for an
rclone mount, or use policy `mfs` (most free space) as a member of an
rclone union remote.

See [rclone about command](https://rclone.org/commands/rclone_about/)

### EmptyDir

The remote supports empty directories. See [Limitations](/bugs/#limitations)
 for details. Most Object/Bucket-based remotes do not support this.
