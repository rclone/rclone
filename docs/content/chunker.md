---
title: "Chunker"
description: "Split-chunking overlay remote"
date: "2019-08-30"
---

<i class="fa fa-cut"></i>Chunker
----------------------------------------

The `chunker` overlay transparently splits large files into smaller chunks
during the upload to wrapped remote and transparently assembles them back
when the file is downloaded. This allows to effectively overcome size limits
imposed by storage providers.

To use it, first set up the underlying remote following the configuration
instructions for that remote. You can also use a local pathname instead of
a remote.

First check your chosen remote is working - we'll call it `remote:path` here.
Note that anything inside `remote:path` will be chunked and anything outside
won't. This means that if you are using a bucket based remote (eg S3, B2, swift)
then you should probably put the bucket in the remote `s3:bucket`.

Now configure `chunker` using `rclone config`. We will call this one `overlay`
to separate it from the `remote`.

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> overlay
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Transparently chunk/split large files
   \ "chunker"
[snip]
Storage> chunker
Remote to chunk/unchunk.
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).
remote> remote:path
Files larger than chunk_size will be split in chunks. By default 2 Gb.
Enter a size with suffix k,M,G,T. Press Enter for the default ("2G").
chunk_size> 1G
Edit advanced config? (y/n)
y) Yes
n) No
y/n> n
Remote config
--------------------
[overlay]
type = chunker
remote = TestLocal:
chunk_size = 2G
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Specifying the remote

In normal use, make sure the remote has a `:` in. If you specify the remote
without a `:` then rclone will use a local directory of that name.
So if you use a remote of `/path/to/secret/files` then rclone will
chunk stuff in that directory. If you use a remote of `name` then rclone
will put files in a directory called `name` in the current directory.


### Chunking

When rclone starts a file upload, chunker checks the file size.
If it doesn't exceed the configured chunk size, chunker will just pass it
to the wrapped remote. If a file is large, chunker will transparently cut
data in pieces with temporary names and stream them one by one, on the fly.
Each chunk will contain the specified number of data byts, except for the
last one which may have less data. If file size is unknown in advance
(this is called a streaming upload), chunker will internally create
a temporary copy, record its size and repeat the above process.
When upload completes, temporary chunk files are finally renamed.
This scheme guarantees that operations look from outside as atomic.
A similar method with hidden temporary chunks is used for other operations
(copy/move/rename etc). If operation fails, hidden chunks are normally
destroyed, and the destination composite file stays intact.

#### Chunk names

By default chunk names are `BIG_FILE_NAME.rclone-chunk.001`,
`BIG_FILE_NAME.rclone-chunk.002` etc, because the default chunk name
format is `*.rclone-chunk.###`. You can configure another name format
using the `--chunker-name-format` option. The format uses asterisk
`*` as a placeholder for the base file name and one or more consecutive
hash characters `#` as a placeholder for the chunk number. There must be
one and only one asterisk. The number of consecutive hashes defines the
minimum length of a string representing a chunk number. If a chunk number
has less digits than the number of hashes, it is left-padded by zeros.
If there are more digits in the number, they are left as is.
By default numbering starts from 1 but there is another option that allows
user to start from 0, eg. for compatibility with legacy software.

For example, if name format is `big_*-##.part`, and original file was
named `data.txt` and numbering starts from 0, then the first chunk will be
named `big_data.txt-00.part`, the 99th chunk will be `big_data.txt-98.part`
and the 302nd chunk will be `big_data.txt-301.part`.

Would-be chunk files are ignored if their name does not match given format.
The list command might encounter composite files with missinng or invalid
chunks. By default, if chunker detects a missing chunk it will silently
ignore the whole group. Use the `--chunker-fail-on-bad-chunks` flag
to make it fail with an error message.


### Metadata

By default when a file is large enough, chunker will create a metadata
object besides data chunks. The object is named after the original file.
Chunker allows to choose between few metadata formats. Please note that
currently metadata is not created for files smaller than configured
chunk size. This may change in future as new formats are developed.

#### Simple JSON metadata format

This is the default format. It supports hash sums and chunk validation
for composite files. Meta objects carry the following fields:

- `size`    - total size of chunks
- `nchunks` - number of chunks
- `md5`     - MD5 hashsum (if present)
- `sha1`    - SHA1 hashsum (if present)

There is no field for composite file name as it's simply equal to the name
of meta object on the wrapped remote. Please refer to respective sections
for detils on hashsums and modified time handling.

#### WedDavMailRu compatible metadata format

The `wdmrcompat` metadata format is only useful to support historical files
created by [WebDriveMailru](https://github.com/yar229/WebDavMailRuCloud).
It keeps the following fields (most are ignored, though):

- `Name`         - name of the composite file (always equal to the meta file name)
- `Size`         - total size of chunks
- `PublicKey`    - ignored, always "null"
- `CreationDate` - last modification (sic!) time, ignored.

#### No metadata

You can disable meta objects by setting the meta format option to `none`.
In this mode chunker will scan directory for all files that follow
configured chunk name format, group them by detecting chunks with the same
base name and show group names as virtual composite files.
When a download is requested, chunker will transparently assemble compound
files by merging chunks in order. This method is more prone to missing chunk
errors (especially missing last chunk) than metadata-enabled formats.


### Hashsums

Chunker supports hashsums only when a compatible metadata is present.
Thus, if you choose metadata format of `none` or `wdmrcompat`, chunker
will return `UNSUPPORTED` as hashsum.

Please note that metadata is stored only for composite files. If a file
is small (smaller than configured chunk size), chunker will transparently
redirect hash requests to wrapped remote, so support depends on that.
You will see the empty string as a hashsum of requested type for small
files if the wrapped remote doesn't support it.

Many storage backends support MD5 and SHA1 hash types, so does chunker.
Currently you can choose one or another but not both.
MD5 is set by default as the most supported type.
Since chunker keeps hashes for composite files and falls back to the
wrapped remote hash for small ones, we advise you to choose the same
hash type as wrapped remote, so your file listings look coherent.

Normally, when a file is copied to chunker controlled remote, chunker
will ask its source for compatible file hash and revert to on-the-fly
calculation if none is found. This involves some CPU overhead but provides
a guarantee that given hashsum is available. Also, chunker will reject
a server-side copy or move operation if source and destination hashsum
types are different, resulting in the extra network bandwidth, too.
In some rare cases this may be undesired, so chunker provides two optional
choices: `sha1quick` and `md5quick`. If source does not have the primary
hash type and the quick mode is enabled, chunker will try to fall back to
the secondary type. This will save CPU and bandwidth but can result in empty
hashsums at destination. Beware of consequences: the `sync` command will
revert (sometimes silently) to time/size comparison if compatible hashsums
between source and target are not found.


### Modified time

Chunker stores modification times using the wrapped remote so support
depends on that. For a small non-chunked file the chunker overlay simply
manipulates modification time of the wrapped remote file.
If file is large and metadata is present, then chunker will get and set
modification time of the metadata object on the wrapped remote.
If file is chunked but metadata format is `none` then chunker will
use modification time of the first chunk.


### Migrations

The idiomatic way to migrate to a different chunk size, hash type or
chunk naming scheme is to:

- Collect all your chunked files under a directory and have your
  chunker remote point to it.
- Create another directory (possibly on the same cloud storage)
  and configure a new remote with desired metadata format,
  hash type, chunk naming etc.
- Now run `rclone sync oldchunks: newchunks:` and all your data
  will be transparently converted at transfer.
  This may take some time.
- After checking data integrity you may remove configuration section
  of the old remote.

If rclone gets killed during a long operation on a big composite file,
hidden temporary chunks may stay in the directory. They will not be
shown by the list command but will eat up your account quota.
Please note that the `deletefile` rclone command deletes only active
chunks of a file. As a workaround, you can use remote of the wrapped
file system to see them.
An easy way to get rid of hidden garbage is to copy littered directory
somewhere using the chunker remote and purge original directory.
The `copy` command will copy only active chunks while the `purge` will
remove everything including garbage.


### Caveats and Limitations

Chunker requires wrapped remote to support server side `move` (or `copy` +
delete) operations, otherwise it will explicitly refuse to start.
This is because it internally renames temporary chunk files to their final
names when an operation completes successfully.

Note that moves done using the copy-and-delete method may incur double
charging with some cloud storage providers.

Chunker will not automatically rename existing chunks when you change the
chunk name format. Beware that in result of this some files which have been
treated as chunks before the change can pop up in directory listings as
normal files and vice versa. The same warning holds for the chunk size.
If you desperately need to change critical chunking setings, you should
run data migration as described in a dedicated section.

If wrapped remote is case insensitive, the chunker overlay will inherit
that property (so you can't have a file called "Hello.doc" and "hello.doc"
in the same directory).


<!--- autogenerated options start - DO NOT EDIT, instead edit fs.RegInfo in backend/chunker/chunker.go then run make backenddocs -->
### Standard Options

Here are the standard options specific to chunker.

#### --chunker-remote

Remote to chunk/unchunk.
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).

- Config:      remote
- Env Var:     RCLONE_CHUNKER_REMOTE
- Type:        string
- Default:     ""

#### --chunker-chunk-size

Files larger than chunk size will be split in chunks.

- Config:      chunk_size
- Env Var:     RCLONE_CHUNKER_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     2G

### Advanced Options

Here are the advanced options specific to chunker (Transparently chunk/split large files).

#### --chunker-name-format

String format of chunk file names.
The two placeholders are: base file name (*) and chunk number (#...).
There must be one and only one asterisk and one or more consecutive hash characters.
If chunk number has less digits than the number of hashes, it is left-padded by zeros.
If there are more digits in the number, they are left as is.
Possible chunk files are ignored if their name does not match given format.

- Config:      name_format
- Env Var:     RCLONE_CHUNKER_NAME_FORMAT
- Type:        string
- Default:     "*.rclone_chunk.###"

#### --chunker-start-from

Minimum valid chunk number. Usually 0 or 1.
By default chunk numbers start from 1.

- Config:      start_from
- Env Var:     RCLONE_CHUNKER_START_FROM
- Type:        int
- Default:     1

#### --chunker-meta-format

Format of the metadata object or "none". By default "simplejson".
Metadata is a small JSON file named after the composite file.

- Config:      meta_format
- Env Var:     RCLONE_CHUNKER_META_FORMAT
- Type:        string
- Default:     "simplejson"
- Examples:
    - "none"
        - Do not use metadata files at all. Requires hash type "none".
    - "simplejson"
        - Simple JSON supports hash sums and chunk validation.
        - It has the following fields: size, nchunks, md5, sha1.
    - "wdmrcompat"
        - This format brings compatibility with WebDavMailRuCloud.
        - It does not support hash sums or validation, most fields are ignored.
        - It has the following fields: Name, Size, PublicKey, CreationDate.
        - Requires hash type "none".

#### --chunker-hash-type

Choose how chunker handles hash sums.

- Config:      hash_type
- Env Var:     RCLONE_CHUNKER_HASH_TYPE
- Type:        string
- Default:     "md5"
- Examples:
    - "none"
        - Chunker can pass any hash supported by wrapped remote
        - for a single-chunk file but returns nothing otherwise.
    - "md5"
        - MD5 for multi-chunk files. Requires "simplejson".
    - "sha1"
        - SHA1 for multi-chunk files. Requires "simplejson".
    - "md5quick"
        - When a file is copied on to chunker, MD5 is taken from its source
        - falling back to SHA1 if the source doesn't support it. Requires "simplejson".
    - "sha1quick"
        - Similar to "md5quick" but prefers SHA1 over MD5. Requires "simplejson".

#### --chunker-fail-on-bad-chunks

The list command might encounter files with missinng or invalid chunks.
This boolean flag tells what rclone should do in such cases.

- Config:      fail_on_bad_chunks
- Env Var:     RCLONE_CHUNKER_FAIL_ON_BAD_CHUNKS
- Type:        bool
- Default:     false
- Examples:
    - "true"
        - Fail with error.
    - "false"
        - Silently ignore invalid object.

<!--- autogenerated options stop -->
