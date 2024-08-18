---
title: "Hasher"
description: "Better checksums for other remotes"
versionIntroduced: "v1.57"
status: Experimental
---

# {{< icon "fa fa-check-double" >}} Hasher

Hasher is a special overlay backend to create remotes which handle
checksums for other remotes. It's main functions include:
- Emulate hash types unimplemented by backends
- Cache checksums to help with slow hashing of large local or (S)FTP files
- Warm up checksum cache from external SUM files

## Getting started

To use Hasher, first set up the underlying remote following the configuration
instructions for that remote. You can also use a local pathname instead of
a remote. Check that your base remote is working.

Let's call the base remote `myRemote:path` here. Note that anything inside
`myRemote:path` will be handled by hasher and anything outside won't.
This means that if you are using a bucket based remote (S3, B2, Swift)
then you should put the bucket in the remote `s3:bucket`.

Now proceed to interactive or manual configuration.

### Interactive configuration

Run `rclone config`:
```
No remotes found, make a new one?
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> Hasher1
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Handle checksums for other remotes
   \ "hasher"
[snip]
Storage> hasher
Remote to cache checksums for, like myremote:mypath.
Enter a string value. Press Enter for the default ("").
remote> myRemote:path
Comma separated list of supported checksum types.
Enter a string value. Press Enter for the default ("md5,sha1").
hashsums> md5
Maximum time to keep checksums in cache. 0 = no cache, off = cache forever.
max_age> off
Edit advanced config? (y/n)
y) Yes
n) No
y/n> n
Remote config
--------------------
[Hasher1]
type = hasher
remote = myRemote:path
hashsums = md5
max_age = off
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Manual configuration

Run `rclone config path` to see the path of current active config file,
usually `YOURHOME/.config/rclone/rclone.conf`.
Open it in your favorite text editor, find section for the base remote
and create new section for hasher like in the following examples:

```
[Hasher1]
type = hasher
remote = myRemote:path
hashes = md5
max_age = off

[Hasher2]
type = hasher
remote = /local/path
hashes = dropbox,sha1
max_age = 24h
```

Hasher takes basically the following parameters:
- `remote` is required,
- `hashes` is a comma separated list of supported checksums
   (by default `md5,sha1`),
- `max_age` - maximum time to keep a checksum value in the cache,
   `0` will disable caching completely,
   `off` will cache "forever" (that is until the files get changed).

Make sure the `remote` has `:` (colon) in. If you specify the remote without
a colon then rclone will use a local directory of that name. So if you use
a remote of `/local/path` then rclone will handle hashes for that directory.
If you use `remote = name` literally then rclone will put files
**in a directory called `name` located under current directory**.

## Usage

### Basic operations

Now you can use it as `Hasher2:subdir/file` instead of base remote.
Hasher will transparently update cache with new checksums when a file
is fully read or overwritten, like:
```
rclone copy External:path/file Hasher:dest/path

rclone cat Hasher:path/to/file > /dev/null
```

The way to refresh **all** cached checksums (even unsupported by the base backend)
for a subtree is to **re-download** all files in the subtree. For example,
use `hashsum --download` using **any** supported hashsum on the command line
(we just care to re-read):
```
rclone hashsum MD5 --download Hasher:path/to/subtree > /dev/null

rclone backend dump Hasher:path/to/subtree
```

You can print or drop hashsum cache using custom backend commands:
```
rclone backend dump Hasher:dir/subdir

rclone backend drop Hasher:
```

### Pre-Seed from a SUM File

Hasher supports two backend commands: generic SUM file `import` and faster
but less consistent `stickyimport`.

```
rclone backend import Hasher:dir/subdir SHA1 /path/to/SHA1SUM [--checkers 4]
```

Instead of SHA1 it can be any hash supported by the remote. The last argument
can point to either a local or an `other-remote:path` text file in SUM format.
The command will parse the SUM file, then walk down the path given by the
first argument, snapshot current fingerprints and fill in the cache entries
correspondingly.
- Paths in the SUM file are treated as relative to `hasher:dir/subdir`.
- The command will **not** check that supplied values are correct.
  You **must know** what you are doing.
- This is a one-time action. The SUM file will not get "attached" to the
  remote. Cache entries can still be overwritten later, should the object's
  fingerprint change.
- The tree walk can take long depending on the tree size. You can increase
  `--checkers` to make it faster. Or use `stickyimport` if you don't care
  about fingerprints and consistency.

```
rclone backend stickyimport hasher:path/to/data sha1 remote:/path/to/sum.sha1
```

`stickyimport` is similar to `import` but works much faster because it
does not need to stat existing files and skips initial tree walk.
Instead of binding cache entries to file fingerprints it creates _sticky_
entries bound to the file name alone ignoring size, modification time etc.
Such hash entries can be replaced only by `purge`, `delete`, `backend drop`
or by full re-read/re-write of the files.

## Configuration reference

{{< rem autogenerated options start" - DO NOT EDIT - instead edit fs.RegInfo in backend/hasher/hasher.go then run make backenddocs" >}}
### Standard options

Here are the Standard options specific to hasher (Better checksums for other remotes).

#### --hasher-remote

Remote to cache checksums for (e.g. myRemote:path).

Properties:

- Config:      remote
- Env Var:     RCLONE_HASHER_REMOTE
- Type:        string
- Required:    true

#### --hasher-hashes

Comma separated list of supported checksum types.

Properties:

- Config:      hashes
- Env Var:     RCLONE_HASHER_HASHES
- Type:        CommaSepList
- Default:     md5,sha1

#### --hasher-max-age

Maximum time to keep checksums in cache (0 = no cache, off = cache forever).

Properties:

- Config:      max_age
- Env Var:     RCLONE_HASHER_MAX_AGE
- Type:        Duration
- Default:     off

### Advanced options

Here are the Advanced options specific to hasher (Better checksums for other remotes).

#### --hasher-auto-size

Auto-update checksum for files smaller than this size (disabled by default).

Properties:

- Config:      auto_size
- Env Var:     RCLONE_HASHER_AUTO_SIZE
- Type:        SizeSuffix
- Default:     0

#### --hasher-description

Description of the remote.

Properties:

- Config:      description
- Env Var:     RCLONE_HASHER_DESCRIPTION
- Type:        string
- Required:    false

### Metadata

Any metadata supported by the underlying remote is read and written.

See the [metadata](/docs/#metadata) docs for more info.

## Backend commands

Here are the commands specific to the hasher backend.

Run them with

    rclone backend COMMAND remote:

The help below will explain what arguments each command takes.

See the [backend](/commands/rclone_backend/) command for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](/rc/#backend-command).

### drop

Drop cache

    rclone backend drop remote: [options] [<arguments>+]

Completely drop checksum cache.
Usage Example:
    rclone backend drop hasher:


### dump

Dump the database

    rclone backend dump remote: [options] [<arguments>+]

Dump cache records covered by the current remote

### fulldump

Full dump of the database

    rclone backend fulldump remote: [options] [<arguments>+]

Dump all cache records in the database

### import

Import a SUM file

    rclone backend import remote: [options] [<arguments>+]

Amend hash cache from a SUM file and bind checksums to files by size/time.
Usage Example:
    rclone backend import hasher:subdir md5 /path/to/sum.md5


### stickyimport

Perform fast import of a SUM file

    rclone backend stickyimport remote: [options] [<arguments>+]

Fill hash cache from a SUM file without verifying file fingerprints.
Usage Example:
    rclone backend stickyimport hasher:subdir md5 remote:path/to/sum.md5


{{< rem autogenerated options stop >}}

## Implementation details (advanced)

This section explains how various rclone operations work on a hasher remote.

**Disclaimer. This section describes current implementation which can
change in future rclone versions!.**

### Hashsum command

The `rclone hashsum` (or `md5sum` or `sha1sum`) command will:

1. if requested hash is supported by lower level, just pass it.
2. if object size is below `auto_size` then download object and calculate
   _requested_ hashes on the fly.
3. if unsupported and the size is big enough, build object `fingerprint`
   (including size, modtime if supported, first-found _other_ hash if any).
4. if the strict match is found in cache for the requested remote, return
   the stored hash.
5. if remote found but fingerprint mismatched, then purge the entry and
   proceed to step 6.
6. if remote not found or had no requested hash type or after step 5:
   download object, calculate all _supported_ hashes on the fly and store
   in cache; return requested hash.

### Other operations

- any time a hash is requested, follow the logic from 1-4 from `hashsum` above
- whenever a file is uploaded or downloaded **in full**, capture the stream
  to calculate all supported hashes on the fly and update database
- server-side `move`  will update keys of existing cache entries
- `deletefile` will remove a single cache entry
- `purge` will remove all cache entries under the purged path

Note that setting `max_age = 0` will disable checksum caching completely.

If you set `max_age = off`, checksums in cache will never age, unless you
fully rewrite or delete the file.

### Cache storage

Cached checksums are stored as `bolt` database files under rclone cache
directory, usually `~/.cache/rclone/kv/`. Databases are maintained
one per _base_ backend, named like `BaseRemote~hasher.bolt`.
Checksums for multiple `alias`-es into a single base backend
will be stored in the single database. All local paths are treated as
aliases into the `local` backend (unless encrypted or chunked) and stored
in `~/.cache/rclone/kv/local~hasher.bolt`.
Databases can be shared between multiple rclone processes.
