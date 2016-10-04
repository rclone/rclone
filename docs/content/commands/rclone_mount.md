---
date: 2016-08-24T23:47:55+01:00
title: "rclone mount"
slug: rclone_mount
url: /commands/rclone_mount/
---
## rclone mount

Mount the remote as a mountpoint. **EXPERIMENTAL**

### Synopsis



rclone mount allows Linux, FreeBSD and macOS to mount any of Rclone's
cloud storage systems as a file system with FUSE.

This is **EXPERIMENTAL** - use with care.

First set up your remote using `rclone config`.  Check it works with `rclone ls` etc.

Start the mount like this

    rclone mount remote:path/to/files /path/to/local/mount &

Stop the mount with

    fusermount -u /path/to/local/mount

Or with OS X

    umount -u /path/to/local/mount

### Limitations ###

This can only read files seqentially, or write files sequentially.  It
can't read and write or seek in files.

rclonefs inherits rclone's directory handling.  In rclone's world
directories don't really exist.  This means that empty directories
will have a tendency to disappear once they fall out of the directory
cache.

The bucket based FSes (eg swift, s3, google compute storage, b2) won't
work from the root - you will need to specify a bucket, or a path
within the bucket.  So `swift:` won't work whereas `swift:bucket` will
as will `swift:bucket/path`.

Only supported on Linux, FreeBSD and OS X at the moment.

### rclone mount vs rclone sync/copy ##

File systems expect things to be 100% reliable, whereas cloud storage
systems are a long way from 100% reliable. The rclone sync/copy
commands cope with this with lots of retries.  However rclone mount
can't use retries in the same way without making local copies of the
uploads.  This might happen in the future, but for the moment rclone
mount won't do that, so will be less reliable than the rclone command.

### Bugs ###

  * All the remotes should work for read, but some may not for write
    * those which need to know the size in advance won't - eg B2
    * maybe should pass in size as -1 to mean work it out

### TODO ###

  * Check hashes on upload/download
  * Preserve timestamps
  * Move directories


```
rclone mount remote:path /path/to/mountpoint
```

### Options

```
      --debug-fuse   Debug the FUSE internals - needs -v.
      --no-modtime   Don't read the modification time (can speed things up).
```

### Options inherited from parent commands

```
      --acd-templink-threshold int      Files >= this size will be downloaded via their tempLink.
      --acd-upload-wait-time duration   Time to wait after a failed complete upload to see if it appears. (default 2m0s)
      --ask-password                    Allow prompt for password for encrypted configuration. (default true)
      --b2-chunk-size int               Upload chunk size. Must fit in memory.
      --b2-test-mode string             A flag string for X-Bz-Test-Mode header.
      --b2-upload-cutoff int            Cutoff for switching to chunked upload
      --b2-versions                     Include old versions in directory listings.
      --bwlimit int                     Bandwidth limit in kBytes/s, or use suffix b|k|M|G
      --checkers int                    Number of checkers to run in parallel. (default 8)
  -c, --checksum                        Skip based on checksum & size, not mod-time & size
      --config string                   Config file. (default "/home/ncw/.rclone.conf")
      --contimeout duration             Connect timeout (default 1m0s)
      --cpuprofile string               Write cpu profile to file
      --delete-after                    When synchronizing, delete files on destination after transfering
      --delete-before                   When synchronizing, delete files on destination before transfering
      --delete-during                   When synchronizing, delete files during transfer (default)
      --delete-excluded                 Delete files on dest excluded from sync
      --drive-auth-owner-only           Only consider files owned by the authenticated user. Requires drive-full-list.
      --drive-chunk-size int            Upload chunk size. Must a power of 2 >= 256k.
      --drive-formats string            Comma separated list of preferred formats for downloading Google docs. (default "docx,xlsx,pptx,svg")
      --drive-full-list                 Use a full listing for directory list. More data but usually quicker. (obsolete)
      --drive-upload-cutoff int         Cutoff for switching to chunked upload
      --drive-use-trash                 Send files to the trash instead of deleting permanently.
      --dropbox-chunk-size int          Upload chunk size. Max 150M.
  -n, --dry-run                         Do a trial run with no permanent changes
      --dump-bodies                     Dump HTTP headers and bodies - may contain sensitive info
      --dump-filters                    Dump the filters to the output
      --dump-headers                    Dump HTTP headers - may contain sensitive info
      --exclude string                  Exclude files matching pattern
      --exclude-from string             Read exclude patterns from file
      --files-from string               Read list of source-file names from file
  -f, --filter string                   Add a file-filtering rule
      --filter-from string              Read filtering patterns from a file
      --ignore-existing                 Skip all files that exist on destination
      --ignore-size                     Ignore size when skipping use mod-time or checksum.
  -I, --ignore-times                    Don't skip files that match size and time - transfer all files
      --include string                  Include files matching pattern
      --include-from string             Read include patterns from file
      --log-file string                 Log everything to this file
      --low-level-retries int           Number of low level retries to do. (default 10)
      --max-age string                  Don't transfer any file older than this in s or suffix ms|s|m|h|d|w|M|y
      --max-depth int                   If set limits the recursion depth to this. (default -1)
      --max-size int                    Don't transfer any file larger than this in k or suffix b|k|M|G
      --memprofile string               Write memory profile to file
      --min-age string                  Don't transfer any file younger than this in s or suffix ms|s|m|h|d|w|M|y
      --min-size int                    Don't transfer any file smaller than this in k or suffix b|k|M|G
      --modify-window duration          Max time diff to be considered the same (default 1ns)
      --no-check-certificate            Do not verify the server SSL certificate. Insecure.
      --no-gzip-encoding                Don't set Accept-Encoding: gzip.
      --no-traverse                     Don't traverse destination file system on copy.
      --no-update-modtime               Don't update destination mod-time if files identical.
      --onedrive-chunk-size int         Above this size files will be chunked - must be multiple of 320k.
      --onedrive-upload-cutoff int      Cutoff for switching to chunked upload - must be <= 100MB
  -q, --quiet                           Print as little stuff as possible
      --retries int                     Retry operations this many times if they fail (default 3)
      --size-only                       Skip based on size only, not mod-time or checksum
      --stats duration                  Interval to print stats (0 to disable) (default 1m0s)
      --swift-chunk-size int            Above this size files will be chunked into a _segments container.
      --timeout duration                IO idle timeout (default 5m0s)
      --transfers int                   Number of file transfers to run in parallel. (default 4)
  -u, --update                          Skip files that are newer on the destination.
  -v, --verbose                         Print lots more stuff
```

### SEE ALSO
* [rclone](/commands/rclone/)	 - Sync files and directories to and from local and remote object stores - v1.33-DEV

###### Auto generated by spf13/cobra on 24-Aug-2016
