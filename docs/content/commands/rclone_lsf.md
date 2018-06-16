---
date: 2018-06-16T18:20:28+01:00
title: "rclone lsf"
slug: rclone_lsf
url: /commands/rclone_lsf/
---
## rclone lsf

List directories and objects in remote:path formatted for parsing

### Synopsis


List the contents of the source path (directories and objects) to
standard output in a form which is easy to parse by scripts.  By
default this will just be the names of the objects and directories,
one per line.  The directories will have a / suffix.

Eg

    $ rclone lsf swift:bucket
    bevajer5jef
    canole
    diwogej7
    ferejej3gux/
    fubuwic

Use the --format option to control what gets listed.  By default this
is just the path, but you can use these parameters to control the
output:

    p - path
    s - size
    t - modification time
    h - hash
    i - ID of object if known
    m - MimeType of object if known

So if you wanted the path, size and modification time, you would use
--format "pst", or maybe --format "tsp" to put the path last.

Eg

    $ rclone lsf  --format "tsp" swift:bucket
    2016-06-25 18:55:41;60295;bevajer5jef
    2016-06-25 18:55:43;90613;canole
    2016-06-25 18:55:43;94467;diwogej7
    2018-04-26 08:50:45;0;ferejej3gux/
    2016-06-25 18:55:40;37600;fubuwic

If you specify "h" in the format you will get the MD5 hash by default,
use the "--hash" flag to change which hash you want.  Note that this
can be returned as an empty string if it isn't available on the object
(and for directories), "ERROR" if there was an error reading it from
the object and "UNSUPPORTED" if that object does not support that hash
type.

For example to emulate the md5sum command you can use

    rclone lsf -R --hash MD5 --format hp --separator "  " --files-only .

Eg

    $ rclone lsf -R --hash MD5 --format hp --separator "  " --files-only swift:bucket 
    7908e352297f0f530b84a756f188baa3  bevajer5jef
    cd65ac234e6fea5925974a51cdd865cc  canole
    03b5341b4f234b9d984d03ad076bae91  diwogej7
    8fd37c3810dd660778137ac3a66cc06d  fubuwic
    99713e14a4c4ff553acaf1930fad985b  gixacuh7ku

(Though "rclone md5sum ." is an easier way of typing this.)

By default the separator is ";" this can be changed with the
--separator flag.  Note that separators aren't escaped in the path so
putting it last is a good strategy.

Eg

    $ rclone lsf  --separator "," --format "tshp" swift:bucket
    2016-06-25 18:55:41,60295,7908e352297f0f530b84a756f188baa3,bevajer5jef
    2016-06-25 18:55:43,90613,cd65ac234e6fea5925974a51cdd865cc,canole
    2016-06-25 18:55:43,94467,03b5341b4f234b9d984d03ad076bae91,diwogej7
    2018-04-26 08:52:53,0,,ferejej3gux/
    2016-06-25 18:55:40,37600,8fd37c3810dd660778137ac3a66cc06d,fubuwic

You can output in CSV standard format.  This will escape things in "
if they contain ,

Eg

    $ rclone lsf --csv --files-only --format ps remote:path
    test.log,22355
    test.sh,449
    "this file contains a comma, in the file name.txt",6

Note that the --absolute parameter is useful for making lists of files
to pass to an rclone copy with the --files-from flag.

For example to find all the files modified within one day and copy
those only (without traversing the whole directory structure):

    rclone lsf --absolute --files-only --max-age 1d /path/to/local > new_files
    rclone copy --files-from new_files /path/to/local remote:path


Any of the filtering options can be applied to this commmand.

There are several related list commands

  * `ls` to list size and path of objects only
  * `lsl` to list modification time, size and path of objects only
  * `lsd` to list directories only
  * `lsf` to list objects and directories in easy to parse format
  * `lsjson` to list objects and directories in JSON format

`ls`,`lsl`,`lsd` are designed to be human readable.
`lsf` is designed to be human and machine readable.
`lsjson` is designed to be machine readable.

Note that `ls` and `lsl` recurse by default - use "--max-depth 1" to stop the recursion.

The other list commands `lsd`,`lsf`,`lsjson` do not recurse by default - use "-R" to make them recurse.

Listing a non existent directory will produce an error except for
remotes which can't have empty directories (eg s3, swift, gcs, etc -
the bucket based remotes).


```
rclone lsf remote:path [flags]
```

### Options

```
      --absolute           Put a leading / in front of path names.
      --csv                Output in CSV format.
  -d, --dir-slash          Append a slash to directory names. (default true)
      --dirs-only          Only list directories.
      --files-only         Only list files.
  -F, --format string      Output format - see  help for details (default "p")
      --hash h             Use this hash when h is used in the format MD5|SHA-1|DropboxHash (default "MD5")
  -h, --help               help for lsf
  -R, --recursive          Recurse into the listing.
  -s, --separator string   Separator for the items in the format. (default ";")
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
