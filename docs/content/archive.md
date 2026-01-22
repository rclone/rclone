---
title: "Archive"
description: "Archive Remote"
versionIntroduced: "v1.72"
---

# {{< icon "fas fa-archive" >}} Archive

The Archive backend allows read only access to the content of archive
files on cloud storage without downloading the complete archive. This
means you could mount a large archive file and use only the parts of
it your application requires, rather than having to extract it.

The archive files are recognised by their extension.

| Archive  | Extension |
| -------- | --------- |
| Zip      | `.zip`    |
| Squashfs | `.sqfs`   |

The supported archive file types are cloud friendly - a single file
can be found and downloaded without downloading the whole archive.

If you just want to create, list or extract archives and don't want to
mount them then you may find the `rclone archive` commands more
convenient.

- [rclone archive create](/commands/rclone_archive_create/)
- [rclone archive list](/commands/rclone_archive_list/)
- [rclone archive extract](/commands/rclone_archive_extract/)

These commands supports a wider range of non cloud friendly archives
(but not squashfs) but can't be used for `rclone mount` or any other
rclone commands (eg `rclone check`).

## Configuration

This backend is best used without configuration.

Use it by putting the string `:archive:` in front of another remote,
say `remote:dir` to make `:archive:remote:dir`.

Any archives in `remote:dir` will become directories and any files may
be read out of them individually.

For example

```
$ rclone lsf s3:rclone/dir
100files.sqfs
100files.zip
```

Note that `100files.zip` and `100files.sqfs` are now directories:

```
$ rclone lsf :archive:s3:rclone/dir
100files.sqfs/
100files.zip/
```

Which we can look inside:

```
$ rclone lsf :archive:s3:rclone/dir/100files.zip/
cofofiy5jun
gigi
hevupaz5z
kacak/
kozemof/
lamapaq4
qejahen
quhenen2rey
soboves8
vibat/
wose
xade
zilupot
```

Files not in an archive can be read and written as normal. Files in an archive can only be read.

The archive backend can also be used in a configuration file. Use the `remote` variable to point to the destination of the archive.

```
[remote]
type = archive
remote = s3:rclone/dir/100files.zip
```

Gives

```
$ rclone lsf remote:
cofofiy5jun
gigi
hevupaz5z
kacak/
...
```


## Modification times

Modification times are preserved with an accuracy depending on the archive type.

```
$ rclone lsl --max-depth 1 :archive:s3:rclone/dir/100files.zip
       12 2025-10-27 14:39:20.000000000 cofofiy5jun
       81 2025-10-27 14:39:20.000000000 gigi
       58 2025-10-27 14:39:20.000000000 hevupaz5z
        6 2025-10-27 14:39:20.000000000 lamapaq4
       43 2025-10-27 14:39:20.000000000 qejahen
       66 2025-10-27 14:39:20.000000000 quhenen2rey
       95 2025-10-27 14:39:20.000000000 soboves8
       71 2025-10-27 14:39:20.000000000 wose
       76 2025-10-27 14:39:20.000000000 xade
       15 2025-10-27 14:39:20.000000000 zilupot
```

For `zip` and `squashfs` files this is 1s.

## Hashes

Which hash is supported depends on the archive type. Zip files use
CRC32, Squashfs don't support any hashes. For example:

```
$ rclone hashsum crc32 :archive:s3:rclone/dir/100files.zip/
b2288554  cofofiy5jun
a87e62b6  wose
f90f630b  xade
c7d0ef29  gigi
f1c64740  soboves8
cb7b4a5d  quhenen2rey
5115242b  kozemof/fonaxo
afeabd9a  qejahen
71202402  kozemof/fijubey5di
bd99e512  kozemof/napux
...
```

Hashes will be checked when the file is read from the archive and used
as part of syncing if possible.

```
$ rclone copy -vv :archive:s3:rclone/dir/100files.zip /tmp/100files
...
2025/10/27 14:56:44 DEBUG : kacak/turovat5c/yuyuquk: crc32 = abd05cc8 OK
2025/10/27 14:56:44 DEBUG : kacak/turovat5c/yuyuquk.aeb661dc.partial: renamed to: kacak/turovat5c/yuyuquk
2025/10/27 14:56:44 INFO  : kacak/turovat5c/yuyuquk: Copied (new)
...
```

## Zip

The [Zip file format](https://en.wikipedia.org/wiki/ZIP_(file_format))
is a widely used archive format that bundles one or more files and
folders into a single file, primarily for easier storage or
transmission. It typically uses compression (most commonly the DEFLATE
algorithm) to reduce the overall size of the archived content. Zip
files are supported natively by most modern operating systems.

Rclone does not support the following advanced features of Zip files:

- Splitting large archives into smaller parts
- Password protection
- Zstd compression

## Squashfs

Squashfs is a compressed, read-only file system format primarily used
in Linux-based systems. It's designed to compress entire file systems
(including files, directories, and metadata) into a single archive
file, which can then be mounted and read directly, appearing as a
normal directory structure. Because it's read-only and highly
compressed, Squashfs is ideal for live CDs/USBs, embedded devices with
limited storage, and software package distribution, as it saves space
and ensures the integrity of the original files.

Rclone supports the following squashfs compression formats:

- `Gzip`
- `Lzma`
- `Xz`
- `Zstd`

These are not yet working:

- `Lzo` - Not yet supported
- `Lz4` - Broken with "error decompressing: lz4: bad magic number"

Rclone works fastest with large squashfs block sizes. For example:

```
mksquashfs 100files 100files.sqfs -comp zstd -b 1M
```

## Limitations

Files in the archive backend are read only. It isn't possible to
create archives with the archive backend yet. However you **can** create
archives with [rclone archive create](/commands/rclone_archive_create/).

Only `.zip` and `.sqfs` archives are supported as these are the only
common archiving formats which make it easy to read directory listings
from the archive without downloading the whole archive.

Internally the archive backend uses the VFS to access files. It isn't
possible to configure the internal VFS yet which might be useful.

## Archive Formats

Here's a table rating common archive formats on their Cloud
Optimization which is based on their ability to access a single file
without reading the entire archive.

This capability depends on whether the format has a central **index**
(or "table of contents") that a program can read first to find the
exact location of a specific file.

| Format | Extensions | Cloud Optimized | Explanation |
| :--- | :--- | :--- | :--- |
| **ZIP** | `.zip` | **Excellent** | **Zip files have an index** (the "central directory") stored at the *end* of the file. A program can seek to the end, read the index to find a file's location and size, and then seek directly to that file's data to extract it. |
| **SquashFS** | `.squashfs`, `.sqfs`, `.sfs` | **Excellent** | This is a compressed read-only *filesystem image*, not just an archive. It is **specifically designed for random access**. It uses metadata and index tables to allow the system to find and decompress individual files or data blocks on demand. |
| **ISO Image** | `.iso` | **Excellent** | Like SquashFS, this is a *filesystem image* (for optical media). It contains a filesystem (like ISO 9660 or UDF) with a **table of contents at a known location**, allowing for direct access to any file without reading the whole disk. |
| **RAR** | `.rar` | **Good** | RAR supports "non-solid" and "solid" modes. In the common **non-solid** mode, files are compressed separately, and an index allows for easy single-file extraction (like ZIP). In "solid" mode, this rating would be "Very Poor." |
| **7z** | `.7z` | **Poor** | By default, 7z uses "solid" archives to maximize compression. This compresses files as one continuous stream. To extract a file from the middle, all preceding files must be decompressed first. (If explicitly created as "non-solid," its rating would be "Excellent"). |
| **tar** | `.tar` | **Poor** | "Tape Archive" is a *streaming* format with **no central index**. To find a file, you must read the archive from the beginning, checking each file header one by one until you find the one you want. This is slow but doesn't require decompressing data. |
| **Gzipped Tar** | `.tar.gz`, `.tgz` | **Very Poor** | This is a `tar` file (already "Poor") compressed with `gzip` as a **single, non-seekable stream**. You cannot seek. To get *any* file, you must decompress the *entire* archive from the beginning up to that file. |
| **Bzipped/XZ Tar** | `.tar.bz2`, `.tar.xz` | **Very Poor** | This is the same principle as `tar.gz`. The entire archive is one large compressed block, making random access impossible. |

## Ideas for improvements

It would be possible to add ISO support fairly easily as the library we use ([go-diskfs](https://github.com/diskfs/go-diskfs/)) supports it. We could also add `ext4` and `fat32` the same way, however in my experience these are not very common as files so probably not worth it. Go-diskfs can also read partitions which we could potentially take advantage of.

It would be possible to add write support, but this would only be for creating new archives, not for updating existing archives.

<!-- autogenerated options start - DO NOT EDIT - instead edit fs.RegInfo in backend/archive/archive.go and run make backenddocs to verify --> <!-- markdownlint-disable-line line-length -->
### Standard options

Here are the Standard options specific to archive (Read archives).

#### --archive-remote

Remote to wrap to read archives from.

Normally should contain a ':' and a path, e.g. "myremote:path/to/dir",
"myremote:bucket" or "myremote:".

If this is left empty, then the archive backend will use the root as
the remote.

This means that you can use :archive:remote:path and it will be
equivalent to setting remote="remote:path".


Properties:

- Config:      remote
- Env Var:     RCLONE_ARCHIVE_REMOTE
- Type:        string
- Required:    false

### Advanced options

Here are the Advanced options specific to archive (Read archives).

#### --archive-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_ARCHIVE_DESCRIPTION
- Type:        string
- Required:    false

### Metadata

Any metadata supported by the underlying remote is read and written.

See the [metadata](/docs/#metadata) docs for more info.

<!-- autogenerated options stop -->
