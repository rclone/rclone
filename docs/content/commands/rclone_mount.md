---
date: 2018-06-16T18:20:28+01:00
title: "rclone mount"
slug: rclone_mount
url: /commands/rclone_mount/
---
## rclone mount

Mount the remote as a mountpoint. **EXPERIMENTAL**

### Synopsis


rclone mount allows Linux, FreeBSD, macOS and Windows to
mount any of Rclone's cloud storage systems as a file system with
FUSE.

This is **EXPERIMENTAL** - use with care.

First set up your remote using `rclone config`.  Check it works with `rclone ls` etc.

Start the mount like this

    rclone mount remote:path/to/files /path/to/local/mount

Or on Windows like this where X: is an unused drive letter

    rclone mount remote:path/to/files X:

When the program ends, either via Ctrl+C or receiving a SIGINT or SIGTERM signal,
the mount is automatically stopped.

The umount operation can fail, for example when the mountpoint is busy.
When that happens, it is the user's responsibility to stop the mount manually with

    # Linux
    fusermount -u /path/to/local/mount
    # OS X
    umount /path/to/local/mount

### Installing on Windows

To run rclone mount on Windows, you will need to
download and install [WinFsp](http://www.secfs.net/winfsp/).

WinFsp is an [open source](https://github.com/billziss-gh/winfsp)
Windows File System Proxy which makes it easy to write user space file
systems for Windows.  It provides a FUSE emulation layer which rclone
uses combination with
[cgofuse](https://github.com/billziss-gh/cgofuse).  Both of these
packages are by Bill Zissimopoulos who was very helpful during the
implementation of rclone mount for Windows.

#### Windows caveats

Note that drives created as Administrator are not visible by other
accounts (including the account that was elevated as
Administrator). So if you start a Windows drive from an Administrative
Command Prompt and then try to access the same drive from Explorer
(which does not run as Administrator), you will not be able to see the
new drive.

The easiest way around this is to start the drive from a normal
command prompt. It is also possible to start a drive from the SYSTEM
account (using [the WinFsp.Launcher
infrastructure](https://github.com/billziss-gh/winfsp/wiki/WinFsp-Service-Architecture))
which creates drives accessible for everyone on the system or
alternatively using [the nssm service manager](https://nssm.cc/usage).

### Limitations

Without the use of "--vfs-cache-mode" this can only write files
sequentially, it can only seek when reading.  This means that many
applications won't work with their files on an rclone mount without
"--vfs-cache-mode writes" or "--vfs-cache-mode full".  See the [File
Caching](#file-caching) section for more info.

The bucket based remotes (eg Swift, S3, Google Compute Storage, B2,
Hubic) won't work from the root - you will need to specify a bucket,
or a path within the bucket.  So `swift:` won't work whereas
`swift:bucket` will as will `swift:bucket/path`.
None of these support the concept of directories, so empty
directories will have a tendency to disappear once they fall out of
the directory cache.

Only supported on Linux, FreeBSD, OS X and Windows at the moment.

### rclone mount vs rclone sync/copy

File systems expect things to be 100% reliable, whereas cloud storage
systems are a long way from 100% reliable. The rclone sync/copy
commands cope with this with lots of retries.  However rclone mount
can't use retries in the same way without making local copies of the
uploads. Look at the **EXPERIMENTAL** [file caching](#file-caching)
for solutions to make mount mount more reliable.

### Attribute caching

You can use the flag --attr-timeout to set the time the kernel caches
the attributes (size, modification time etc) for directory entries.

The default is "1s" which caches files just long enough to avoid
too many callbacks to rclone from the kernel.

In theory 0s should be the correct value for filesystems which can
change outside the control of the kernel. However this causes quite a
few problems such as
[rclone using too much memory](https://github.com/ncw/rclone/issues/2157),
[rclone not serving files to samba](https://forum.rclone.org/t/rclone-1-39-vs-1-40-mount-issue/5112)
and [excessive time listing directories](https://github.com/ncw/rclone/issues/2095#issuecomment-371141147).

The kernel can cache the info about a file for the time given by
"--attr-timeout". You may see corruption if the remote file changes
length during this window.  It will show up as either a truncated file
or a file with garbage on the end.  With "--attr-timeout 1s" this is
very unlikely but not impossible.  The higher you set "--attr-timeout"
the more likely it is.  The default setting of "1s" is the lowest
setting which mitigates the problems above.

If you set it higher ('10s' or '1m' say) then the kernel will call
back to rclone less often making it more efficient, however there is
more chance of the corruption issue above.

If files don't change on the remote outside of the control of rclone
then there is no chance of corruption.

This is the same as setting the attr_timeout option in mount.fuse.

### Filters

Note that all the rclone filters can be used to select a subset of the
files to be visible in the mount.

### systemd

When running rclone mount as a systemd service, it is possible
to use Type=notify. In this case the service will enter the started state
after the mountpoint has been successfully set up.
Units having the rclone mount service specified as a requirement
will see all files and folders immediately in this mode.

### chunked reading ###

--vfs-read-chunk-size will enable reading the source objects in parts.
This can reduce the used download quota for some remotes by requesting only chunks
from the remote that are actually read at the cost of an increased number of requests.

When --vfs-read-chunk-size-limit is also specified and greater than --vfs-read-chunk-size,
the chunk size for each open file will get doubled for each chunk read, until the
specified value is reached. A value of -1 will disable the limit and the chunk size will
grow indefinitely.

With --vfs-read-chunk-size 100M and --vfs-read-chunk-size-limit 0 the following
parts will be downloaded: 0-100M, 100M-200M, 200M-300M, 300M-400M and so on.
When --vfs-read-chunk-size-limit 500M is specified, the result would be
0-100M, 100M-300M, 300M-700M, 700M-1200M, 1200M-1700M and so on.

Chunked reading will only work with --vfs-cache-mode < full, as the file will always
be copied to the vfs cache before opening with --vfs-cache-mode full.

### Directory Cache

Using the `--dir-cache-time` flag, you can set how long a
directory should be considered up to date and not refreshed from the
backend. Changes made locally in the mount may appear immediately or
invalidate the cache. However, changes done on the remote will only
be picked up once the cache expires.

Alternatively, you can send a `SIGHUP` signal to rclone for
it to flush all directory caches, regardless of how old they are.
Assuming only one rclone instance is running, you can reset the cache
like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a [remote control](/rc) then you can use
rclone rc to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

### File Caching

**NB** File caching is **EXPERIMENTAL** - use with care!

These flags control the VFS file caching options.  The VFS layer is
used by rclone mount to make a cloud storage system work more like a
normal file system.

You'll need to enable VFS caching if you want, for example, to read
and write simultaneously to a file.  See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)

If run with `-vv` rclone will print the location of the file cache.  The
files are stored in the user cache file area which is OS dependent but
can be controlled with `--cache-dir` or setting the appropriate
environment variable.

The cache has 4 different modes selected by `--vfs-cache-mode`.
The higher the cache mode the more compatible rclone becomes at the
cost of using disk space.

Note that files are written back to the remote only when they are
closed so if rclone is quit or dies with open files then these won't
get written back to the remote.  However they will still be in the on
disk cache.

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

If an upload fails it will be retried up to --low-level-retries times.

#### --vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk.  When
a file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at
the cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk,
it will be kept on the disk after it is written to the remote.  It
will be purged on a schedule according to `--vfs-cache-max-age`.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
--low-level-retries times.


```
rclone mount remote:path /path/to/mountpoint [flags]
```

### Options

```
      --allow-non-empty                    Allow mounting over a non-empty directory.
      --allow-other                        Allow access to other users.
      --allow-root                         Allow access to root user.
      --attr-timeout duration              Time for which file/directory attributes are cached. (default 1s)
      --daemon                             Run mount as a daemon (background mode).
      --debug-fuse                         Debug the FUSE internals - needs -v.
      --default-permissions                Makes kernel enforce access control based on the file mode.
      --dir-cache-time duration            Time to cache directory entries for. (default 5m0s)
      --fuse-flag stringArray              Flags or arguments to be passed direct to libfuse/WinFsp. Repeat if required.
      --gid uint32                         Override the gid field set by the filesystem. (default 502)
  -h, --help                               help for mount
      --max-read-ahead int                 The number of bytes that can be prefetched for sequential reads. (default 128k)
      --no-checksum                        Don't compare checksums on up/download.
      --no-modtime                         Don't read/write the modification time (can speed things up).
      --no-seek                            Don't allow seeking in files.
  -o, --option stringArray                 Option for libfuse/WinFsp. Repeat if required.
      --poll-interval duration             Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable. (default 1m0s)
      --read-only                          Mount read-only.
      --uid uint32                         Override the uid field set by the filesystem. (default 502)
      --umask int                          Override the permission bits set by the filesystem.
      --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
      --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
      --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
      --vfs-read-chunk-size int            Read the source objects in chunks.
      --vfs-read-chunk-size-limit int      If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. -1 is unlimited.
      --volname string                     Set the volume name (not supported by all OSes).
      --write-back-cache                   Makes kernel buffer writes before sending them to rclone. Without this, writethrough caching is used.
```

### Options inherited from parent commands

```
      --acd-templink-threshold int          Files >= this size will be downloaded via their tempLink. (default 9G)
      --acd-upload-wait-per-gb duration     Additional time per GB to wait after a failed complete upload to see if it appears. (default 3m0s)
      --ask-password                        Allow prompt for password for encrypted configuration. (default true)
      --auto-confirm                        If enabled, do not request console confirmation.
      --azureblob-chunk-size int            Upload chunk size. Must fit in memory. (default 4M)
      --azureblob-upload-cutoff int         Cutoff for switching to chunked upload (default 256M)
      --b2-chunk-size int                   Upload chunk size. Must fit in memory. (default 96M)
      --b2-hard-delete                      Permanently delete files on remote removal, otherwise hide files.
      --b2-test-mode string                 A flag string for X-Bz-Test-Mode header.
      --b2-upload-cutoff int                Cutoff for switching to chunked upload (default 190.735M)
      --b2-versions                         Include old versions in directory listings.
      --backup-dir string                   Make backups into hierarchy based in DIR.
      --bind string                         Local address to bind to for outgoing connections, IPv4, IPv6 or name.
      --box-upload-cutoff int               Cutoff for switching to multipart upload (default 50M)
      --buffer-size int                     Buffer size when copying files. (default 16M)
      --bwlimit BwTimetable                 Bandwidth limit in kBytes/s, or use suffix b|k|M|G or a full timetable.
      --cache-chunk-clean-interval string   Interval at which chunk cleanup runs (default "1m")
      --cache-chunk-no-memory               Disable the in-memory cache for storing chunks during streaming
      --cache-chunk-path string             Directory to cached chunk files (default "/home/ncw/.cache/rclone/cache-backend")
      --cache-chunk-size string             The size of a chunk (default "5M")
      --cache-db-path string                Directory to cache DB (default "/home/ncw/.cache/rclone/cache-backend")
      --cache-db-purge                      Purge the cache DB before
      --cache-db-wait-time duration         How long to wait for the DB to be available - 0 is unlimited (default 1s)
      --cache-dir string                    Directory rclone will use for caching. (default "/home/ncw/.cache/rclone")
      --cache-info-age string               How much time should object info be stored in cache (default "6h")
      --cache-read-retries int              How many times to retry a read from a cache storage (default 10)
      --cache-rps int                       Limits the number of requests per second to the source FS. -1 disables the rate limiter (default -1)
      --cache-tmp-upload-path string        Directory to keep temporary files until they are uploaded to the cloud storage
      --cache-tmp-wait-time string          How long should files be stored in local cache before being uploaded (default "15m")
      --cache-total-chunk-size string       The total size which the chunks can take up from the disk (default "10G")
      --cache-workers int                   How many workers should run in parallel to download chunks (default 4)
      --cache-writes                        Will cache file data on writes through the FS
      --checkers int                        Number of checkers to run in parallel. (default 8)
  -c, --checksum                            Skip based on checksum & size, not mod-time & size
      --config string                       Config file. (default "/home/ncw/.rclone.conf")
      --contimeout duration                 Connect timeout (default 1m0s)
  -L, --copy-links                          Follow symlinks and copy the pointed to item.
      --cpuprofile string                   Write cpu profile to file
      --crypt-show-mapping                  For all files listed show how the names encrypt.
      --delete-after                        When synchronizing, delete files on destination after transfering
      --delete-before                       When synchronizing, delete files on destination before transfering
      --delete-during                       When synchronizing, delete files during transfer (default)
      --delete-excluded                     Delete files on dest excluded from sync
      --disable string                      Disable a comma separated list of features.  Use help to see a list.
      --drive-acknowledge-abuse             Set to allow files which return cannotDownloadAbusiveFile to be downloaded.
      --drive-alternate-export              Use alternate export URLs for google documents export.
      --drive-auth-owner-only               Only consider files owned by the authenticated user.
      --drive-chunk-size int                Upload chunk size. Must a power of 2 >= 256k. (default 8M)
      --drive-formats string                Comma separated list of preferred formats for downloading Google docs. (default "docx,xlsx,pptx,svg")
      --drive-impersonate string            Impersonate this user when using a service account.
      --drive-list-chunk int                Size of listing chunk 100-1000. 0 to disable. (default 1000)
      --drive-shared-with-me                Only show files that are shared with me
      --drive-skip-gdocs                    Skip google documents in all listings.
      --drive-trashed-only                  Only show files that are in the trash
      --drive-upload-cutoff int             Cutoff for switching to chunked upload (default 8M)
      --drive-use-created-date              Use created date instead of modified date.
      --drive-use-trash                     Send files to the trash instead of deleting permanently. (default true)
      --dropbox-chunk-size int              Upload chunk size. Max 150M. (default 48M)
  -n, --dry-run                             Do a trial run with no permanent changes
      --dump string                         List of items to dump from: headers,bodies,requests,responses,auth,filters,goroutines,openfiles
      --dump-bodies                         Dump HTTP headers and bodies - may contain sensitive info
      --dump-headers                        Dump HTTP bodies - may contain sensitive info
      --exclude stringArray                 Exclude files matching pattern
      --exclude-from stringArray            Read exclude patterns from file
      --exclude-if-present string           Exclude directories if filename is present
      --fast-list                           Use recursive list if available. Uses more memory but fewer transactions.
      --files-from stringArray              Read list of source-file names from file
  -f, --filter stringArray                  Add a file-filtering rule
      --filter-from stringArray             Read filtering patterns from a file
      --gcs-location string                 Default location for buckets (us|eu|asia|us-central1|us-east1|us-east4|us-west1|asia-east1|asia-noetheast1|asia-southeast1|australia-southeast1|europe-west1|europe-west2).
      --gcs-storage-class string            Default storage class for buckets (MULTI_REGIONAL|REGIONAL|STANDARD|NEARLINE|COLDLINE|DURABLE_REDUCED_AVAILABILITY).
      --ignore-checksum                     Skip post copy check of checksums.
      --ignore-errors                       delete even if there are I/O errors
      --ignore-existing                     Skip all files that exist on destination
      --ignore-size                         Ignore size when skipping use mod-time or checksum.
  -I, --ignore-times                        Don't skip files that match size and time - transfer all files
      --immutable                           Do not modify files. Fail if existing files have been modified.
      --include stringArray                 Include files matching pattern
      --include-from stringArray            Read include patterns from file
      --local-no-check-updated              Don't check to see if the files change during upload
      --local-no-unicode-normalization      Don't apply unicode normalization to paths and filenames
      --log-file string                     Log everything to this file
      --log-level string                    Log level DEBUG|INFO|NOTICE|ERROR (default "NOTICE")
      --low-level-retries int               Number of low level retries to do. (default 10)
      --max-age duration                    Only transfer files younger than this in s or suffix ms|s|m|h|d|w|M|y (default off)
      --max-delete int                      When synchronizing, limit the number of deletes (default -1)
      --max-depth int                       If set limits the recursion depth to this. (default -1)
      --max-size int                        Only transfer files smaller than this in k or suffix b|k|M|G (default off)
      --max-transfer int                    Maximum size of data to transfer. (default off)
      --mega-debug                          If set then output more debug from mega.
      --memprofile string                   Write memory profile to file
      --min-age duration                    Only transfer files older than this in s or suffix ms|s|m|h|d|w|M|y (default off)
      --min-size int                        Only transfer files bigger than this in k or suffix b|k|M|G (default off)
      --modify-window duration              Max time diff to be considered the same (default 1ns)
      --no-check-certificate                Do not verify the server SSL certificate. Insecure.
      --no-gzip-encoding                    Don't set Accept-Encoding: gzip.
      --no-traverse                         Obsolete - does nothing.
      --no-update-modtime                   Don't update destination mod-time if files identical.
  -x, --one-file-system                     Don't cross filesystem boundaries.
      --onedrive-chunk-size int             Above this size files will be chunked - must be multiple of 320k. (default 10M)
  -q, --quiet                               Print as little stuff as possible
      --rc                                  Enable the remote control server.
      --rc-addr string                      IPaddress:Port or :Port to bind server to. (default "localhost:5572")
      --rc-cert string                      SSL PEM key (concatenation of certificate and CA certificate)
      --rc-client-ca string                 Client certificate authority to verify clients with
      --rc-htpasswd string                  htpasswd file - if not provided no authentication is done
      --rc-key string                       SSL PEM Private key
      --rc-max-header-bytes int             Maximum size of request header (default 4096)
      --rc-pass string                      Password for authentication.
      --rc-realm string                     realm for authentication (default "rclone")
      --rc-server-read-timeout duration     Timeout for server reading data (default 1h0m0s)
      --rc-server-write-timeout duration    Timeout for server writing data (default 1h0m0s)
      --rc-user string                      User name for authentication.
      --retries int                         Retry operations this many times if they fail (default 3)
      --retries-sleep duration              Interval between retrying operations if they fail, e.g 500ms, 60s, 5m. (0 to disable)
      --s3-acl string                       Canned ACL used when creating buckets and/or storing objects in S3
      --s3-chunk-size int                   Chunk size to use for uploading (default 5M)
      --s3-disable-checksum                 Don't store MD5 checksum with object metadata
      --s3-storage-class string             Storage class to use when uploading S3 objects (STANDARD|REDUCED_REDUNDANCY|STANDARD_IA|ONEZONE_IA)
      --s3-upload-concurrency int           Concurrency for multipart uploads (default 2)
      --sftp-ask-password                   Allow asking for SFTP password when needed.
      --size-only                           Skip based on size only, not mod-time or checksum
      --skip-links                          Don't warn about skipped symlinks.
      --ssh-path-override string            Override path used by SSH connection.
      --stats duration                      Interval between printing stats, e.g 500ms, 60s, 5m. (0 to disable) (default 1m0s)
      --stats-file-name-length int          Max file name length in stats. 0 for no limit (default 40)
      --stats-log-level string              Log level to show --stats output DEBUG|INFO|NOTICE|ERROR (default "INFO")
      --stats-unit string                   Show data rate in stats as either 'bits' or 'bytes'/s (default "bytes")
      --streaming-upload-cutoff int         Cutoff for switching to chunked upload if file size is unknown. Upload starts after reaching cutoff or when file ends. (default 100k)
      --suffix string                       Suffix for use with --backup-dir.
      --swift-chunk-size int                Above this size files will be chunked into a _segments container. (default 5G)
      --syslog                              Use Syslog for logging
      --syslog-facility string              Facility for syslog, eg KERN,USER,... (default "DAEMON")
      --timeout duration                    IO idle timeout (default 5m0s)
      --tpslimit float                      Limit HTTP transactions per second to this.
      --tpslimit-burst int                  Max burst of transactions for --tpslimit. (default 1)
      --track-renames                       When synchronizing, track file renames and do a server side move if possible
      --transfers int                       Number of file transfers to run in parallel. (default 4)
  -u, --update                              Skip files that are newer on the destination.
      --use-server-modtime                  Use server modified time instead of object metadata
      --user-agent string                   Set the user-agent to a specified string. The default is rclone/ version (default "rclone/v1.42")
  -v, --verbose count                       Print lots more stuff (repeat for more)
```

### SEE ALSO

* [rclone](/commands/rclone/)	 - Sync files and directories to and from local and remote object stores - v1.42

###### Auto generated by spf13/cobra on 16-Jun-2018
