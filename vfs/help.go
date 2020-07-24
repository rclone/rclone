package vfs

// Help contains text describing file and directory caching to add to
// the command help.
var Help = `
### VFS - Virtual File System

This command uses the VFS layer. This adapts the cloud storage objects
that rclone uses into something which looks much more like a disk
filing system.

Cloud storage objects have lots of properties which aren't like disk
files - you can't extend them or write to the middle of them, so the
VFS layer has to deal with that. Because there is no one right way of
doing this there are various options explained below.

The VFS layer also implements a directory cache - this caches info
about files and directories (but not the data) in memory.

### VFS Directory Cache

Using the ` + "`--dir-cache-time`" + ` flag, you can control long a
directory should be considered up to date and not refreshed from the
backend. Changes made through the mount will appear immediately or
invalidate the cache.

    --dir-cache-time duration   Time to cache directory entries for. (default 5m0s)
    --poll-interval duration    Time to wait between polling for changes.

However, changes made directoy on the cloud storage by the web
interface or a different copy of rclone will only be picked up once
the directory cache expires if the backend configured does not support
polling for changes. If the backend supports polling, changes will be
picked up on within the polling interval.

You can send a ` + "`SIGHUP`" + ` signal to rclone for it to flush all
directory caches, regardless of how old they are.  Assuming only one
rclone instance is running, you can reset the cache like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a [remote control](/rc) then you can use
rclone rc to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

### VFS File Buffering

The ` + "`--buffer-size`" + ` flag determines the amount of memory,
that will be used to buffer data in advance.

Each open file will try to keep the specified amount of data in memory
at all times. The buffered data is bound to one open file and won't be
shared.

This flag is a upper limit for the used memory per open file.  The
buffer will only use memory for data that is downloaded but not not
yet read. If the buffer is empty, only a small amount of memory will
be used.

The maximum memory used by rclone for buffering can be up to
` + "`--buffer-size * open files`" + `.

### VFS File Caching

These flags control the VFS file caching options. File caching is
necessary to make the VFS layer appear compatible with a normal file
system. It can be disabled at the cost of some compatibility.

For example you'll need to enable VFS caching if you want to read and
write simultaneously to a file.  See below for more details.

Note that the VFS cache is separate from the cache backend and you may
find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-mode CacheMode           Cache mode off|minimal|writes|full (default off)
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-max-size SizeSuffix      Max total size of objects in the cache. (default off)
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-write-back duration            Time to writeback files after last use when using cache. (default 5s)

If run with ` + "`-vv`" + ` rclone will print the location of the file cache.  The
files are stored in the user cache file area which is OS dependent but
can be controlled with ` + "`--cache-dir`" + ` or setting the appropriate
environment variable.

The cache has 4 different modes selected by ` + "`--vfs-cache-mode`" + `.
The higher the cache mode the more compatible rclone becomes at the
cost of using disk space.

Note that files are written back to the remote only when they are
closed and if they haven't been accessed for --vfs-write-back
second. If rclone is quit or dies with files that haven't been
uploaded, these will be uploaded next time rclone is run with the same
flags.

If using --vfs-cache-max-size note that the cache may exceed this size
for two reasons.  Firstly because it is only checked every
--vfs-cache-poll-interval.  Secondly because open files cannot be
evicted from the cache.

#### --vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

  * Files can't be opened for both read AND write
  * Files opened for write can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files open for read with O_TRUNC will be opened write only
  * Files open for write only will behave as if O_TRUNC was supplied
  * Open modes O_APPEND, O_TRUNC are ignored
  * If an upload fails it can't be retried

#### --vfs-cache-mode minimal

This is very similar to "off" except that files opened for read AND
write will be buffered to disks.  This means that files opened for
write will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

  * Files opened for write only can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files opened for write only will ignore O_APPEND, O_TRUNC
  * If an upload fails it can't be retried

#### --vfs-cache-mode writes

In this mode files opened for read only are still read directly from
the remote, write only and read/write files are buffered to disk
first.

This mode should support all normal file system operations.

If an upload fails it will be retried at exponentially increasing
intervals up to 1 minute.

#### --vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk. When
data is read from the remote this is buffered to disk as well.

In this mode the files in the cache will be sparse files and rclone
will keep track of which bits of the files it has dowloaded.

So if an application only reads the starts of each file, then rclone
will only buffer the start of the file. These files will appear to be
their full size in the cache, but they will be sparse files with only
the data that has been downloaded present in them.

This mode should support all normal file system operations and is
otherwise identical to --vfs-cache-mode writes.

### VFS Performance

These flags may be used to enable/disable features of the VFS for
performance or other reasons.

In particular S3 and Swift benefit hugely from the --no-modtime flag
(or use --use-server-modtime for a slightly different effect) as each
read of the modification time takes a transaction.

    --no-checksum     Don't compare checksums on up/download.
    --no-modtime      Don't read/write the modification time (can speed things up).
    --no-seek         Don't allow seeking in files.
    --read-only       Mount read-only.

When rclone reads files from a remote it reads them in chunks. This
means that rather than requesting the whole file rclone reads the
chunk specified. This is advantageous because some cloud providers
account for reads being all the data requested, not all the data
delivered.

Rclone will keep doubling the chunk size requested starting at
--vfs-read-chunk-size with a maximum of --vfs-read-chunk-size-limit
unless it is set to "off" in which case there will be no limit.

    --vfs-read-chunk-size SizeSuffix        Read the source objects in chunks. (default 128M)
    --vfs-read-chunk-size-limit SizeSuffix  Max chunk doubling size (default "off")

Sometimes rclone is delivered reads or writes out of order. Rather
than seeking rclone will wait a short time for the in sequence read or
write to come in. These flags only come into effect when not using an
on disk cach file.

    --vfs-read-wait duration   Time to wait for in-sequence read before seeking. (default 20ms)
    --vfs-write-wait duration  Time to wait for in-sequence write before giving error. (default 1s)

### VFS Case Sensitivity

Linux file systems are case-sensitive: two files can differ only
by case, and the exact case must be used when opening a file.

File systems in modern Windows are case-insensitive but case-preserving:
although existing files can be opened using any case, the exact case used
to create the file is preserved and available for programs to query.
It is not allowed for two files in the same directory to differ only by case.

Usually file systems on macOS are case-insensitive. It is possible to make macOS
file systems case-sensitive but that is not the default

The "--vfs-case-insensitive" mount flag controls how rclone handles these
two cases. If its value is "false", rclone passes file names to the mounted
file system as-is. If the flag is "true" (or appears without a value on
command line), rclone may perform a "fixup" as explained below.

The user may specify a file name to open/delete/rename/etc with a case
different than what is stored on mounted file system. If an argument refers
to an existing file with exactly the same name, then the case of the existing
file on the disk will be used. However, if a file name with exactly the same
name is not found but a name differing only by case exists, rclone will
transparently fixup the name. This fixup happens only when an existing file
is requested. Case sensitivity of file names created anew by rclone is
controlled by an underlying mounted file system.

Note that case sensitivity of the operating system running rclone (the target)
may differ from case sensitivity of a file system mounted by rclone (the source).
The flag controls whether "fixup" is performed to satisfy the target.

If the flag is not provided on command line, then its default value depends
on the operating system where rclone runs: "true" on Windows and macOS, "false"
otherwise. If the flag is provided without a value, then it is "true".
`
