---
date: 2017-07-22T18:15:25+01:00
title: "rclone move"
slug: rclone_move
url: /commands/rclone_move/
---
## rclone move

Move files from source to dest.

### Synopsis



Moves the contents of the source directory to the destination
directory. Rclone will error if the source and destination overlap and
the remote does not support a server side directory move operation.

If no filters are in use and if possible this will server side move
`source:path` into `dest:path`. After this `source:path` will no
longer longer exist.

Otherwise for each file in `source:path` selected by the filters (if
any) this will move it into `dest:path`.  If possible a server side
move will be used, otherwise it will copy it (server side if possible)
into `dest:path` then delete the original (if no errors on copy) in
`source:path`.

**Important**: Since this can cause data loss, test first with the
--dry-run flag.


```
rclone move source:path dest:path
```

### Options inherited from parent commands

```
      --acd-templink-threshold int        Files >= this size will be downloaded via their tempLink. (default 9G)
      --acd-upload-wait-per-gb duration   Additional time per GB to wait after a failed complete upload to see if it appears. (default 3m0s)
      --ask-password                      Allow prompt for password for encrypted configuration. (default true)
      --b2-chunk-size int                 Upload chunk size. Must fit in memory. (default 96M)
      --b2-test-mode string               A flag string for X-Bz-Test-Mode header.
      --b2-upload-cutoff int              Cutoff for switching to chunked upload (default 190.735M)
      --b2-versions                       Include old versions in directory listings.
      --backup-dir string                 Make backups into hierarchy based in DIR.
      --buffer-size int                   Buffer size when copying files. (default 16M)
      --bwlimit BwTimetable               Bandwidth limit in kBytes/s, or use suffix b|k|M|G or a full timetable.
      --checkers int                      Number of checkers to run in parallel. (default 8)
  -c, --checksum                          Skip based on checksum & size, not mod-time & size
      --config string                     Config file. (default "/home/ncw/.rclone.conf")
      --contimeout duration               Connect timeout (default 1m0s)
  -L, --copy-links                        Follow symlinks and copy the pointed to item.
      --cpuprofile string                 Write cpu profile to file
      --crypt-show-mapping                For all files listed show how the names encrypt.
      --delete-after                      When synchronizing, delete files on destination after transfering
      --delete-before                     When synchronizing, delete files on destination before transfering
      --delete-during                     When synchronizing, delete files during transfer (default)
      --delete-excluded                   Delete files on dest excluded from sync
      --drive-auth-owner-only             Only consider files owned by the authenticated user.
      --drive-chunk-size int              Upload chunk size. Must a power of 2 >= 256k. (default 8M)
      --drive-formats string              Comma separated list of preferred formats for downloading Google docs. (default "docx,xlsx,pptx,svg")
      --drive-list-chunk int              Size of listing chunk 100-1000. 0 to disable. (default 1000)
      --drive-shared-with-me              Only show files that are shared with me
      --drive-skip-gdocs                  Skip google documents in all listings.
      --drive-trashed-only                Only show files that are in the trash
      --drive-upload-cutoff int           Cutoff for switching to chunked upload (default 8M)
      --drive-use-trash                   Send files to the trash instead of deleting permanently.
      --dropbox-chunk-size int            Upload chunk size. Max 150M. (default 128M)
  -n, --dry-run                           Do a trial run with no permanent changes
      --dump-auth                         Dump HTTP headers with auth info
      --dump-bodies                       Dump HTTP headers and bodies - may contain sensitive info
      --dump-filters                      Dump the filters to the output
      --dump-headers                      Dump HTTP headers - may contain sensitive info
      --exclude stringArray               Exclude files matching pattern
      --exclude-from stringArray          Read exclude patterns from file
      --fast-list                         Use recursive list if available. Uses more memory but fewer transactions.
      --files-from stringArray            Read list of source-file names from file
  -f, --filter stringArray                Add a file-filtering rule
      --filter-from stringArray           Read filtering patterns from a file
      --gcs-location string               Default location for buckets (us|eu|asia|us-central1|us-east1|us-east4|us-west1|asia-east1|asia-noetheast1|asia-southeast1|australia-southeast1|europe-west1|europe-west2).
      --gcs-storage-class string          Default storage class for buckets (MULTI_REGIONAL|REGIONAL|STANDARD|NEARLINE|COLDLINE|DURABLE_REDUCED_AVAILABILITY).
      --ignore-checksum                   Skip post copy check of checksums.
      --ignore-existing                   Skip all files that exist on destination
      --ignore-size                       Ignore size when skipping use mod-time or checksum.
  -I, --ignore-times                      Don't skip files that match size and time - transfer all files
      --include stringArray               Include files matching pattern
      --include-from stringArray          Read include patterns from file
      --local-no-unicode-normalization    Don't apply unicode normalization to paths and filenames
      --log-file string                   Log everything to this file
      --log-level string                  Log level DEBUG|INFO|NOTICE|ERROR (default "NOTICE")
      --low-level-retries int             Number of low level retries to do. (default 10)
      --max-age string                    Don't transfer any file older than this in s or suffix ms|s|m|h|d|w|M|y
      --max-depth int                     If set limits the recursion depth to this. (default -1)
      --max-size int                      Don't transfer any file larger than this in k or suffix b|k|M|G (default off)
      --memprofile string                 Write memory profile to file
      --min-age string                    Don't transfer any file younger than this in s or suffix ms|s|m|h|d|w|M|y
      --min-size int                      Don't transfer any file smaller than this in k or suffix b|k|M|G (default off)
      --modify-window duration            Max time diff to be considered the same (default 1ns)
      --no-check-certificate              Do not verify the server SSL certificate. Insecure.
      --no-gzip-encoding                  Don't set Accept-Encoding: gzip.
      --no-traverse                       Don't traverse destination file system on copy.
      --no-update-modtime                 Don't update destination mod-time if files identical.
      --old-sync-method                   Deprecated - use --fast-list instead
  -x, --one-file-system                   Don't cross filesystem boundaries.
      --onedrive-chunk-size int           Above this size files will be chunked - must be multiple of 320k. (default 10M)
      --onedrive-upload-cutoff int        Cutoff for switching to chunked upload - must be <= 100MB (default 10M)
  -q, --quiet                             Print as little stuff as possible
      --retries int                       Retry operations this many times if they fail (default 3)
      --s3-acl string                     Canned ACL used when creating buckets and/or storing objects in S3
      --s3-storage-class string           Storage class to use when uploading S3 objects (STANDARD|REDUCED_REDUNDANCY|STANDARD_IA)
      --size-only                         Skip based on size only, not mod-time or checksum
      --stats duration                    Interval between printing stats, e.g 500ms, 60s, 5m. (0 to disable) (default 1m0s)
      --stats-log-level string            Log level to show --stats output DEBUG|INFO|NOTICE|ERROR (default "INFO")
      --stats-unit string                 Show data rate in stats as either 'bits' or 'bytes'/s (default "bytes")
      --suffix string                     Suffix for use with --backup-dir.
      --swift-chunk-size int              Above this size files will be chunked into a _segments container. (default 5G)
      --syslog                            Use Syslog for logging
      --syslog-facility string            Facility for syslog, eg KERN,USER,... (default "DAEMON")
      --timeout duration                  IO idle timeout (default 5m0s)
      --tpslimit float                    Limit HTTP transactions per second to this.
      --tpslimit-burst int                Max burst of transactions for --tpslimit. (default 1)
      --track-renames                     When synchronizing, track file renames and do a server side move if possible
      --transfers int                     Number of file transfers to run in parallel. (default 4)
  -u, --update                            Skip files that are newer on the destination.
  -v, --verbose count[=-1]                Print lots more stuff (repeat for more)
```

### SEE ALSO
* [rclone](/commands/rclone/)	 - Sync files and directories to and from local and remote object stores - v1.37

###### Auto generated by spf13/cobra on 22-Jul-2017
