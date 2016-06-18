---
title: "Documentation"
description: "Rclone Changelog"
date: "2016-06-18"
---

Changelog
---------

  * v1.30 - 2016-06-18
    * New Features
      * Directory listing code reworked for more features and better error reporting (thanks to Klaus Post for help).  This enables
        * Directory include filtering for efficiency
        * --max-depth parameter
        * Better error reporting
        * More to come
      * Retry more errors
      * Add --ignore-size flag - for uploading images to onedrive
      * Log -v output to stdout by default
      * Display the transfer stats in more human readable form
      * Make 0 size files specifiable with `--max-size 0b`
      * Add `b` suffix so we can specify bytes in --bwlimit, --min-size etc
      * Use "password:" instead of "password>" prompt - thanks Klaus Post and Leigh Klotz
    * Bug Fixes
      * Fix retry doing one too many retries
    * Local
      * Fix problems with OS X and UTF-8 characters
    * Amazon Cloud Drive
      * Check a file exists before uploading to help with 408 Conflict errors
      * Reauth on 401 errors - this has been causing a lot of problems
      * Work around spurious 403 errors
      * Restart directory listings on error
    * Google Drive
      * Check a file exists before uploading to help with duplicates
      * Fix retry of multipart uploads
    * Backblaze B2
      * Implement large file uploading
    * S3
      * Add AES256 server-side encryption for - thanks Justin R. Wilson
    * Google Cloud Storage
      * Make sure we don't use conflicting content types on upload
      * Add service account support - thanks Michal Witkowski
    * Swift
      * Add auth version parameter
      * Add domain option for openstack (v3 auth) - thanks Fabian Ruff
  * v1.29 - 2016-04-18
    * New Features
      * Implement `-I, --ignore-times` for unconditional upload
      * Improve `dedupe`command
        * Now removes identical copies without asking
        * Now obeys `--dry-run`
        * Implement `--dedupe-mode` for non interactive running
          * `--dedupe-mode interactive` - interactive the default.
          * `--dedupe-mode skip` - removes identical files then skips anything left.
          * `--dedupe-mode first` - removes identical files then keeps the first one.
          * `--dedupe-mode newest` - removes identical files then keeps the newest one.
          * `--dedupe-mode oldest` - removes identical files then keeps the oldest one.
          * `--dedupe-mode rename` - removes identical files then renames the rest to be different.
    * Bug fixes
      * Make rclone check obey the `--size-only` flag.
      * Use "application/octet-stream" if discovered mime type is invalid.
      * Fix missing "quit" option when there are no remotes.
    * Google Drive
      * Increase default chunk size to 8 MB - increases upload speed of big files
      * Speed up directory listings and make more reliable
      * Add missing retries for Move and DirMove - increases reliability
      * Preserve mime type on file update
    * Backblaze B2
      * Enable mod time syncing
        * This means that B2 will now check modification times
        * It will upload new files to update the modification times
        * (there isn't an API to just set the mod time.)
        * If you want the old behaviour use `--size-only`.
      * Update API to new version
      * Fix parsing of mod time when not in metadata
    * Swift/Hubic
      * Don't return an MD5SUM for static large objects
    * S3
      * Fix uploading files bigger than 50GB
  * v1.28 - 2016-03-01
    * New Features
      * Configuration file encryption - thanks Klaus Post
      * Improve `rclone config` adding more help and making it easier to understand
      * Implement `-u`/`--update` so creation times can be used on all remotes
      * Implement `--low-level-retries` flag
      * Optionally disable gzip compression on downloads with `--no-gzip-encoding`
    * Bug fixes
      * Don't make directories if `--dry-run` set
      * Fix and document the `move` command
      * Fix redirecting stderr on unix-like OSes when using `--log-file`
      * Fix `delete` command to wait until all finished - fixes missing deletes.
    * Backblaze B2
      * Use one upload URL per go routine fixes `more than one upload using auth token`
      * Add pacing, retries and reauthentication - fixes token expiry problems
      * Upload without using a temporary file from local (and remotes which support SHA1)
      * Fix reading metadata for all files when it shouldn't have been
    * Drive
      * Fix listing drive documents at root
      * Disable copy and move for Google docs
    * Swift
      * Fix uploading of chunked files with non ASCII characters
      * Allow setting of `storage_url` in the config - thanks Xavier Lucas
    * S3
      * Allow IAM role and credentials from environment variables - thanks Brian Stengaard
      * Allow low privilege users to use S3 (check if directory exists during Mkdir) - thanks Jakub Gedeon
    * Amazon Cloud Drive
      * Retry on more things to make directory listings more reliable
  * v1.27 - 2016-01-31
    * New Features
      * Easier headless configuration with `rclone authorize`
      * Add support for multiple hash types - we now check SHA1 as well as MD5 hashes.
      * `delete` command which does obey the filters (unlike `purge`)
      * `dedupe` command to deduplicate a remote.  Useful with Google Drive.
      * Add `--ignore-existing` flag to skip all files that exist on destination.
      * Add `--delete-before`, `--delete-during`, `--delete-after` flags.
      * Add `--memprofile` flag to debug memory use.
      * Warn the user about files with same name but different case
      * Make `--include` rules add their implict exclude * at the end of the filter list
      * Deprecate compiling with go1.3
    * Amazon Cloud Drive
      * Fix download of files > 10 GB
      * Fix directory traversal ("Next token is expired") for large directory listings
      * Remove 409 conflict from error codes we will retry - stops very long pauses
    * Backblaze B2
      * SHA1 hashes now checked by rclone core
    * Drive
      * Add `--drive-auth-owner-only` to only consider files owned by the user - thanks Björn Harrtell
      * Export Google documents
    * Dropbox
      * Make file exclusion error controllable with -q
    * Swift
      * Fix upload from unprivileged user.
    * S3
      * Fix updating of mod times of files with `+` in.
    * Local
      * Add local file system option to disable UNC on Windows.
  * v1.26 - 2016-01-02
    * New Features
      * Yandex storage backend - thank you Dmitry Burdeev ("dibu")
      * Implement Backblaze B2 storage backend
      * Add --min-age and --max-age flags - thank you Adriano Aurélio Meirelles
      * Make ls/lsl/md5sum/size/check obey includes and excludes
    * Fixes
      * Fix crash in http logging
      * Upload releases to github too
    * Swift
      * Fix sync for chunked files
    * One Drive
      * Re-enable server side copy
      * Don't mask HTTP error codes with JSON decode error
    * S3
      * Fix corrupting Content-Type on mod time update (thanks Joseph Spurrier)
  * v1.25 - 2015-11-14
    * New features
      * Implement Hubic storage system
    * Fixes
      * Fix deletion of some excluded files without --delete-excluded
        * This could have deleted files unexpectedly on sync
        * Always check first with `--dry-run`!
    * Swift
      * Stop SetModTime losing metadata (eg X-Object-Manifest)
        * This could have caused data loss for files > 5GB in size
      * Use ContentType from Object to avoid lookups in listings
    * One Drive
      * disable server side copy as it seems to be broken at Microsoft
  * v1.24 - 2015-11-07
    * New features
      * Add support for Microsoft One Drive
      * Add `--no-check-certificate` option to disable server certificate verification
      * Add async readahead buffer for faster transfer of big files
    * Fixes
      * Allow spaces in remotes and check remote names for validity at creation time
      * Allow '&' and disallow ':' in Windows filenames.
    * Swift
      * Ignore directory marker objects where appropriate - allows working with Hubic
      * Don't delete the container if fs wasn't at root
    * S3
      * Don't delete the bucket if fs wasn't at root
    * Google Cloud Storage
      * Don't delete the bucket if fs wasn't at root
  * v1.23 - 2015-10-03
    * New features
      * Implement `rclone size` for measuring remotes
    * Fixes
      * Fix headless config for drive and gcs
      * Tell the user they should try again if the webserver method failed
      * Improve output of `--dump-headers`
    * S3
      * Allow anonymous access to public buckets
    * Swift
      * Stop chunked operations logging "Failed to read info: Object Not Found"
      * Use Content-Length on uploads for extra reliability
  * v1.22 - 2015-09-28
    * Implement rsync like include and exclude flags
    * swift
      * Support files > 5GB - thanks Sergey Tolmachev
  * v1.21 - 2015-09-22
    * New features
      * Display individual transfer progress
      * Make lsl output times in localtime
    * Fixes
      * Fix allowing user to override credentials again in Drive, GCS and ACD
    * Amazon Cloud Drive
      * Implement compliant pacing scheme
    * Google Drive
      * Make directory reads concurrent for increased speed.
  * v1.20 - 2015-09-15
    * New features
      * Amazon Cloud Drive support
      * Oauth support redone - fix many bugs and improve usability
        * Use "golang.org/x/oauth2" as oauth libary of choice
        * Improve oauth usability for smoother initial signup
        * drive, googlecloudstorage: optionally use auto config for the oauth token
      * Implement --dump-headers and --dump-bodies debug flags
      * Show multiple matched commands if abbreviation too short
      * Implement server side move where possible
    * local
      * Always use UNC paths internally on Windows - fixes a lot of bugs
    * dropbox
      * force use of our custom transport which makes timeouts work
    * Thanks to Klaus Post for lots of help with this release
  * v1.19 - 2015-08-28
    * New features
      * Server side copies for s3/swift/drive/dropbox/gcs
      * Move command - uses server side copies if it can
      * Implement --retries flag - tries 3 times by default
      * Build for plan9/amd64 and solaris/amd64 too
    * Fixes
      * Make a current version download with a fixed URL for scripting
      * Ignore rmdir in limited fs rather than throwing error
    * dropbox
      * Increase chunk size to improve upload speeds massively
      * Issue an error message when trying to upload bad file name
  * v1.18 - 2015-08-17
    * drive
      * Add `--drive-use-trash` flag so rclone trashes instead of deletes
      * Add "Forbidden to download" message for files with no downloadURL
    * dropbox
      * Remove datastore
        * This was deprecated and it caused a lot of problems
        * Modification times and MD5SUMs no longer stored
      * Fix uploading files > 2GB
    * s3
      * use official AWS SDK from github.com/aws/aws-sdk-go
      * **NB** will most likely require you to delete and recreate remote
      * enable multipart upload which enables files > 5GB
      * tested with Ceph / RadosGW / S3 emulation
      * many thanks to Sam Liston and Brian Haymore at the [Utah
        Center for High Performance Computing](https://www.chpc.utah.edu/) for a Ceph test account
    * misc
      * Show errors when reading the config file
      * Do not print stats in quiet mode - thanks Leonid Shalupov
      * Add FAQ
      * Fix created directories not obeying umask
      * Linux installation instructions - thanks Shimon Doodkin
  * v1.17 - 2015-06-14
    * dropbox: fix case insensitivity issues - thanks Leonid Shalupov
  * v1.16 - 2015-06-09
    * Fix uploading big files which was causing timeouts or panics
    * Don't check md5sum after download with --size-only
  * v1.15 - 2015-06-06
    * Add --checksum flag to only discard transfers by MD5SUM - thanks Alex Couper
    * Implement --size-only flag to sync on size not checksum & modtime
    * Expand docs and remove duplicated information
    * Document rclone's limitations with directories
    * dropbox: update docs about case insensitivity
  * v1.14 - 2015-05-21
    * local: fix encoding of non utf-8 file names - fixes a duplicate file problem
    * drive: docs about rate limiting
    * google cloud storage: Fix compile after API change in "google.golang.org/api/storage/v1"
  * v1.13 - 2015-05-10
    * Revise documentation (especially sync)
    * Implement --timeout and --conntimeout
    * s3: ignore etags from multipart uploads which aren't md5sums
  * v1.12 - 2015-03-15
    * drive: Use chunked upload for files above a certain size
    * drive: add --drive-chunk-size and --drive-upload-cutoff parameters
    * drive: switch to insert from update when a failed copy deletes the upload
    * core: Log duplicate files if they are detected
  * v1.11 - 2015-03-04
    * swift: add region parameter
    * drive: fix crash on failed to update remote mtime
    * In remote paths, change native directory separators to /
    * Add synchronization to ls/lsl/lsd output to stop corruptions
    * Ensure all stats/log messages to go stderr
    * Add --log-file flag to log everything (including panics) to file
    * Make it possible to disable stats printing with --stats=0
    * Implement --bwlimit to limit data transfer bandwidth
  * v1.10 - 2015-02-12
    * s3: list an unlimited number of items
    * Fix getting stuck in the configurator
  * v1.09 - 2015-02-07
    * windows: Stop drive letters (eg C:) getting mixed up with remotes (eg drive:)
    * local: Fix directory separators on Windows
    * drive: fix rate limit exceeded errors
  * v1.08 - 2015-02-04
    * drive: fix subdirectory listing to not list entire drive
    * drive: Fix SetModTime
    * dropbox: adapt code to recent library changes
  * v1.07 - 2014-12-23
    * google cloud storage: fix memory leak
  * v1.06 - 2014-12-12
    * Fix "Couldn't find home directory" on OSX
    * swift: Add tenant parameter
    * Use new location of Google API packages
  * v1.05 - 2014-08-09
    * Improved tests and consequently lots of minor fixes
    * core: Fix race detected by go race detector
    * core: Fixes after running errcheck
    * drive: reset root directory on Rmdir and Purge
    * fs: Document that Purger returns error on empty directory, test and fix
    * google cloud storage: fix ListDir on subdirectory
    * google cloud storage: re-read metadata in SetModTime
    * s3: make reading metadata more reliable to work around eventual consistency problems
    * s3: strip trailing / from ListDir()
    * swift: return directories without / in ListDir
  * v1.04 - 2014-07-21
    * google cloud storage: Fix crash on Update
  * v1.03 - 2014-07-20
    * swift, s3, dropbox: fix updated files being marked as corrupted
    * Make compile with go 1.1 again
  * v1.02 - 2014-07-19
    * Implement Dropbox remote
    * Implement Google Cloud Storage remote
    * Verify Md5sums and Sizes after copies
    * Remove times from "ls" command - lists sizes only
    * Add add "lsl" - lists times and sizes
    * Add "md5sum" command
  * v1.01 - 2014-07-04
    * drive: fix transfer of big files using up lots of memory
  * v1.00 - 2014-07-03
    * drive: fix whole second dates
  * v0.99 - 2014-06-26
    * Fix --dry-run not working
    * Make compatible with go 1.1
  * v0.98 - 2014-05-30
    * s3: Treat missing Content-Length as 0 for some ceph installations
    * rclonetest: add file with a space in
  * v0.97 - 2014-05-05
    * Implement copying of single files
    * s3 & swift: support paths inside containers/buckets
  * v0.96 - 2014-04-24
    * drive: Fix multiple files of same name being created
    * drive: Use o.Update and fs.Put to optimise transfers
    * Add version number, -V and --version
  * v0.95 - 2014-03-28
    * rclone.org: website, docs and graphics
    * drive: fix path parsing
  * v0.94 - 2014-03-27
    * Change remote format one last time
    * GNU style flags
  * v0.93 - 2014-03-16
    * drive: store token in config file
    * cross compile other versions
    * set strict permissions on config file
  * v0.92 - 2014-03-15
    * Config fixes and --config option
  * v0.91 - 2014-03-15
    * Make config file
  * v0.90 - 2013-06-27
    * Project named rclone
  * v0.00 - 2012-11-18
    * Project started
