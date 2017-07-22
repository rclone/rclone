---
title: "Documentation"
description: "Rclone Changelog"
date: "2017-07-22"
---

Changelog
---------

  * v1.37 - 2017-07-22
    * New backends
      * FTP - thanks to Antonio Messina
      * HTTP - thanks to Vasiliy Tolstov
    * New commands
      * rclone ncdu - for exploring a remote with a text based user interface.
      * rclone lsjson - for listing with a machine readable output
      * rclone dbhashsum - to show Dropbox style hashes of files (local or Dropbox)
    * New Features
      * Implement --fast-list flag
        * This allows remotes to list recursively if they can
        * This uses less transactions (important if you pay for them)
        * This may or may not be quicker
        * This will user more memory as it has to hold the listing in memory
        * --old-sync-method deprecated - the remaining uses are covered by --fast-list
        * This involved a major re-write of all the listing code
      * Add --tpslimit and --tpslimit-burst to limit transactions per second
        * this is useful in conjuction with `rclone mount` to limit external apps
      * Add --stats-log-level so can see --stats without -v
      * Print password prompts to stderr - Hraban Luyat
      * Warn about duplicate files when syncing
      * Oauth improvements
        * allow auth_url and token_url to be set in the config file
        * Print redirection URI if using own credentials.
      * Don't Mkdir at the start of sync to save transactions
    * Compile
      * Update build to go1.8.3
      * Require go1.6 for building rclone
      * Compile 386 builds with "GO386=387" for maximum compatibility
    * Bug Fixes
      * Fix menu selection when no remotes
      * Config saving reworked to not kill the file if disk gets full
      * Don't delete remote if name does not change while renaming
      * moveto, copyto: report transfers and checks as per move and copy
    * Local
      * Add --local-no-unicode-normalization flag - Bob Potter
    * Mount
      * Now supported on Windows using cgofuse and WinFsp - thanks to Bill Zissimopoulos for much help
      * Compare checksums on upload/download via FUSE
      * Unmount when program ends with SIGINT (Ctrl+C) or SIGTERM - Jérôme Vizcaino
      * On read only open of file, make open pending until first read
      * Make --read-only reject modify operations
      * Implement ModTime via FUSE for remotes that support it
      * Allow modTime to be changed even before all writers are closed
      * Fix panic on renames
      * Fix hang on errored upload
    * Crypt
      * Report the name:root as specified by the user
      * Add an "obfuscate" option for filename encryption - Stephen Harris
    * Amazon Drive
      * Fix initialization order for token renewer
      * Remove revoked credentials, allow oauth proxy config and update docs
    * B2
      * Reduce minimum chunk size to 5MB
    * Drive
      * Add team drive support
      * Reduce bandwidth by adding fields for partial responses - Martin Kristensen
      * Implement --drive-shared-with-me flag to view shared with me files - Danny Tsai
      * Add --drive-trashed-only to read only the files in the trash
      * Remove obsolete --drive-full-list
      * Add missing seek to start on retries of chunked uploads
      * Fix stats accounting for upload
      * Convert / in names to a unicode equivalent (／)
      * Poll for Google Drive changes when mounted
    * OneDrive
      * Fix the uploading of files with spaces
      * Fix initialization order for token renewer
      * Display speeds accurately when uploading - Yoni Jah
      * Swap to using http://localhost:53682/ as redirect URL - Michael Ledin
      * Retry on token expired error, reset upload body on retry - Yoni Jah
    * Google Cloud Storage
      * Add ability to specify location and storage class via config and command line - thanks gdm85
      * Create container if necessary on server side copy
      * Increase directory listing chunk to 1000 to increase performance
      * Obtain a refresh token for GCS - Steven Lu
    * Yandex
      * Fix the name reported in log messages (was empty)
      * Correct error return for listing empty directory
    * Dropbox
      * Rewritten to use the v2 API
        * Now supports ModTime
          * Can only set by uploading the file again
          * If you uploaded with an old rclone, rclone may upload everything again
          * Use `--size-only` or `--checksum` to avoid this
        * Now supports the Dropbox content hashing scheme
        * Now supports low level retries
    * S3
      * Work around eventual consistency in bucket creation
      * Create container if necessary on server side copy
      * Add us-east-2 (Ohio) and eu-west-2 (London) S3 regions - Zahiar Ahmed
    * Swift, Hubic
      * Fix zero length directory markers showing in the subdirectory listing
        * this caused lots of duplicate transfers
      * Fix paged directory listings
        * this caused duplicate directory errors
      * Create container if necessary on server side copy
      * Increase directory listing chunk to 1000 to increase performance
      * Make sensible error if the user forgets the container
    * SFTP
      * Add support for using ssh key files
      * Fix under Windows
      * Fix ssh agent on Windows
      * Adapt to latest version of library - Igor Kharin
  * v1.36 - 2017-03-18
    * New Features
      * SFTP remote (Jack Schmidt)
      * Re-implement sync routine to work a directory at a time reducing memory usage
      * Logging revamped to be more inline with rsync - now much quieter
          * -v only shows transfers
          * -vv is for full debug
          * --syslog to log to syslog on capable platforms
      * Implement --backup-dir and --suffix
      * Implement --track-renames (initial implementation by Bjørn Erik Pedersen)
      * Add time-based bandwidth limits (Lukas Loesche)
      * rclone cryptcheck: checks integrity of crypt remotes
      * Allow all config file variables and options to be set from environment variables
      * Add --buffer-size parameter to control buffer size for copy
      * Make --delete-after the default
      * Add --ignore-checksum flag (fixed by Hisham Zarka)
      * rclone check: Add --download flag to check all the data, not just hashes
      * rclone cat: add --head, --tail, --offset, --count and --discard
      * rclone config: when choosing from a list, allow the value to be entered too
      * rclone config: allow rename and copy of remotes
      * rclone obscure: for generating encrypted passwords for rclone's config (T.C. Ferguson)
      * Comply with XDG Base Directory specification (Dario Giovannetti)
        * this moves the default location of the config file in a backwards compatible way
      * Release changes
        * Ubuntu snap support (Dedsec1)
        * Compile with go 1.8
        * MIPS/Linux big and little endian support
    * Bug Fixes
      * Fix copyto copying things to the wrong place if the destination dir didn't exist
      * Fix parsing of remotes in moveto and copyto
      * Fix --delete-before deleting files on copy
      * Fix --files-from with an empty file copying everything
      * Fix sync: don't update mod times if --dry-run set
      * Fix MimeType propagation
      * Fix filters to add ** rules to directory rules
    * Local
      * Implement -L, --copy-links flag to allow rclone to follow symlinks
      * Open files in write only mode so rclone can write to an rclone mount
      * Fix unnormalised unicode causing problems reading directories
      * Fix interaction between -x flag and --max-depth
    * Mount
      * Implement proper directory handling (mkdir, rmdir, renaming)
      * Make include and exclude filters apply to mount
      * Implement read and write async buffers - control with --buffer-size
      * Fix fsync on for directories
      * Fix retry on network failure when reading off crypt
    * Crypt
      * Add --crypt-show-mapping to show encrypted file mapping
      * Fix crypt writer getting stuck in a loop
        * **IMPORTANT** this bug had the potential to cause data corruption when
          * reading data from a network based remote and
          * writing to a crypt on Google Drive
        * Use the cryptcheck command to validate your data if you are concerned
        * If syncing two crypt remotes, sync the unencrypted remote
    * Amazon Drive
      * Fix panics on Move (rename)
      * Fix panic on token expiry
    * B2
      * Fix inconsistent listings and rclone check
      * Fix uploading empty files with go1.8
      * Constrain memory usage when doing multipart uploads
      * Fix upload url not being refreshed properly
    * Drive
      * Fix Rmdir on directories with trashed files
      * Fix "Ignoring unknown object" when downloading
      * Add --drive-list-chunk
      * Add --drive-skip-gdocs (Károly Oláh)
    * OneDrive
      * Implement Move
      * Fix Copy
        * Fix overwrite detection in Copy
        * Fix waitForJob to parse errors correctly
      * Use token renewer to stop auth errors on long uploads
      * Fix uploading empty files with go1.8
    * Google Cloud Storage
      * Fix depth 1 directory listings
    * Yandex
      * Fix single level directory listing
    * Dropbox
      * Normalise the case for single level directory listings
      * Fix depth 1 listing
    * S3
      * Added ca-central-1 region (Jon Yergatian)
  * v1.35 - 2017-01-02
    * New Features
      * moveto and copyto commands for choosing a destination name on copy/move
      * rmdirs command to recursively delete empty directories
      * Allow repeated --include/--exclude/--filter options
      * Only show transfer stats on commands which transfer stuff
        * show stats on any command using the `--stats` flag
      * Allow overlapping directories in move when server side dir move is supported
      * Add --stats-unit option - thanks Scott McGillivray
    * Bug Fixes
      * Fix the config file being overwritten when two rclones are running
      * Make rclone lsd obey the filters properly
      * Fix compilation on mips
      * Fix not transferring files that don't differ in size
      * Fix panic on nil retry/fatal error
    * Mount
      * Retry reads on error - should help with reliability a lot
      * Report the modification times for directories from the remote
      * Add bandwidth accounting and limiting (fixes --bwlimit)
      * If --stats provided will show stats and which files are transferring
      * Support R/W files if truncate is set.
      * Implement statfs interface so df works
      * Note that write is now supported on Amazon Drive
      * Report number of blocks in a file - thanks Stefan Breunig
    * Crypt
      * Prevent the user pointing crypt at itself
      * Fix failed to authenticate decrypted block errors
        * these will now return the underlying unexpected EOF instead
    * Amazon Drive
      * Add support for server side move and directory move - thanks Stefan Breunig
      * Fix nil pointer deref on size attribute
    * B2
      * Use new prefix and delimiter parameters in directory listings
        * This makes --max-depth 1 dir listings as used in mount much faster
      * Reauth the account while doing uploads too - should help with token expiry
    * Drive
      * Make DirMove more efficient and complain about moving the root
      * Create destination directory on Move()
  * v1.34 - 2016-11-06
    * New Features
      * Stop single file and `--files-from` operations iterating through the source bucket.
      * Stop removing failed upload to cloud storage remotes
      * Make ContentType be preserved for cloud to cloud copies
      * Add support to toggle bandwidth limits via SIGUSR2 - thanks Marco Paganini
      * `rclone check` shows count of hashes that couldn't be checked
      * `rclone listremotes` command
      * Support linux/arm64 build - thanks Fredrik Fornwall
      * Remove `Authorization:` lines from `--dump-headers` output
    * Bug Fixes
      * Ignore files with control characters in the names
      * Fix `rclone move` command
        * Delete src files which already existed in dst
        * Fix deletion of src file when dst file older
      * Fix `rclone check` on crypted file systems
      * Make failed uploads not count as "Transferred"
      * Make sure high level retries show with `-q`
      * Use a vendor directory with godep for repeatable builds
    * `rclone mount` - FUSE
      * Implement FUSE mount options
        * `--no-modtime`, `--debug-fuse`, `--read-only`, `--allow-non-empty`, `--allow-root`, `--allow-other`
        * `--default-permissions`, `--write-back-cache`, `--max-read-ahead`, `--umask`, `--uid`, `--gid`
      * Add `--dir-cache-time` to control caching of directory entries
      * Implement seek for files opened for read (useful for video players)
        * with `-no-seek` flag to disable
      * Fix crash on 32 bit ARM (alignment of 64 bit counter)
      * ...and many more internal fixes and improvements!
    * Crypt
      * Don't show encrypted password in configurator to stop confusion
    * Amazon Drive
      * New wait for upload option `--acd-upload-wait-per-gb`
        * upload timeouts scale by file size and can be disabled
      * Add 502 Bad Gateway to list of errors we retry
      * Fix overwriting a file with a zero length file
      * Fix ACD file size warning limit - thanks Felix Bünemann
    * Local
      * Unix: implement `-x`/`--one-file-system` to stay on a single file system
        * thanks Durval Menezes and Luiz Carlos Rumbelsperger Viana
      * Windows: ignore the symlink bit on files
      * Windows: Ignore directory based junction points
    * B2
      * Make sure each upload has at least one upload slot - fixes strange upload stats
      * Fix uploads when using crypt
      * Fix download of large files (sha1 mismatch)
      * Return error when we try to create a bucket which someone else owns
      * Update B2 docs with Data usage, and Crypt section - thanks Tomasz Mazur
    * S3
      * Command line and config file support for
        * Setting/overriding ACL  - thanks Radek Senfeld
        * Setting storage class - thanks Asko Tamm
    * Drive
      * Make exponential backoff work exactly as per Google specification
      * add `.epub`, `.odp` and `.tsv` as export formats.
    * Swift
      * Don't read metadata for directory marker objects
  * v1.33 - 2016-08-24
    * New Features
      * Implement encryption
        * data encrypted in NACL secretbox format
        * with optional file name encryption
      * New commands
        * rclone mount - implements FUSE mounting of remotes (EXPERIMENTAL)
          * works on Linux, FreeBSD and OS X (need testers for the last 2!)
        * rclone cat - outputs remote file or files to the terminal
        * rclone genautocomplete - command to make a bash completion script for rclone
      * Editing a remote using `rclone config` now goes through the wizard
      * Compile with go 1.7 - this fixes rclone on macOS Sierra and on 386 processors
      * Use cobra for sub commands and docs generation
    * drive
      * Document how to make your own client_id
    * s3
      * User-configurable Amazon S3 ACL (thanks Radek Šenfeld)
    * b2
      * Fix stats accounting for upload - no more jumping to 100% done
      * On cleanup delete hide marker if it is the current file
      * New B2 API endpoint (thanks Per Cederberg)
      * Set maximum backoff to 5 Minutes
    * onedrive
      * Fix URL escaping in file names - eg uploading files with `+` in them.
    * amazon cloud drive
      * Fix token expiry during large uploads
      * Work around 408 REQUEST_TIMEOUT and 504 GATEWAY_TIMEOUT errors
    * local
      * Fix filenames with invalid UTF-8 not being uploaded
      * Fix problem with some UTF-8 characters on OS X
  * v1.32 - 2016-07-13
    * Backblaze B2
      * Fix upload of files large files not in root
  * v1.31 - 2016-07-13
    * New Features
      * Reduce memory on sync by about 50%
      * Implement --no-traverse flag to stop copy traversing the destination remote.
        * This can be used to reduce memory usage down to the smallest possible.
        * Useful to copy a small number of files into a large destination folder.
      * Implement cleanup command for emptying trash / removing old versions of files
        * Currently B2 only
      * Single file handling improved
        * Now copied with --files-from
        * Automatically sets --no-traverse when copying a single file
      * Info on using installing with ansible - thanks Stefan Weichinger
      * Implement --no-update-modtime flag to stop rclone fixing the remote modified times.
    * Bug Fixes
      * Fix move command - stop it running for overlapping Fses - this was causing data loss.
    * Local
      * Fix incomplete hashes - this was causing problems for B2.
    * Amazon Drive
      * Rename Amazon Cloud Drive to Amazon Drive - no changes to config file needed.
    * Swift
      * Add support for non-default project domain - thanks Antonio Messina.
    * S3
      * Add instructions on how to use rclone with minio.
      * Add ap-northeast-2 (Seoul) and ap-south-1 (Mumbai) regions.
      * Skip setting the modified time for objects > 5GB as it isn't possible.
    * Backblaze B2
      * Add --b2-versions flag so old versions can be listed and retreived.
      * Treat 403 errors (eg cap exceeded) as fatal.
      * Implement cleanup command for deleting old file versions.
      * Make error handling compliant with B2 integrations notes.
      * Fix handling of token expiry.
      * Implement --b2-test-mode to set `X-Bz-Test-Mode` header.
      * Set cutoff for chunked upload to 200MB as per B2 guidelines.
      * Make upload multi-threaded.
    * Dropbox
      * Don't retry 461 errors.
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
    * Amazon Drive
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
    * Amazon Drive
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
    * Amazon Drive
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
    * OneDrive
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
    * OneDrive
      * disable server side copy as it seems to be broken at Microsoft
  * v1.24 - 2015-11-07
    * New features
      * Add support for Microsoft OneDrive
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
    * Amazon Drive
      * Implement compliant pacing scheme
    * Google Drive
      * Make directory reads concurrent for increased speed.
  * v1.20 - 2015-09-15
    * New features
      * Amazon Drive support
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
