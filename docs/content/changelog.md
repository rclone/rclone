---
title: "Documentation"
description: "Rclone Changelog"
date: "2018-11-24"
---

# Changelog

## v1.45 - 2018-11-24

* New backends
    * The Yandex backend was re-written - see below for details (Sebastian Bünger)
* New commands
    * rcd: New command just to serve the remote control API (Nick Craig-Wood)
* New Features
    * The remote control API (rc) was greatly expanded to allow full control over rclone (Nick Craig-Wood)
        * sensitive operations require authorization or the `--rc-no-auth` flag
        * config/* operations to configure rclone
        * options/* for reading/setting command line flags
        * operations/* for all low level operations, eg copy file, list directory
        * sync/* for sync, copy and move
        * `--rc-files` flag to serve files on the rc http server
          * this is for building web native GUIs for rclone
        * Optionally serving objects on the rc http server
        * Ensure rclone fails to start up if the `--rc` port is in use already
        * See [the rc docs](https://rclone.org/rc/) for more info
    * sync/copy/move
        * Make `--files-from` only read the objects specified and don't scan directories (Nick Craig-Wood)
            * This is a huge speed improvement for destinations with lots of files
    * filter: Add `--ignore-case` flag (Nick Craig-Wood)
    * ncdu: Add remove function ('d' key) (Henning Surmeier)
    * rc command
        * Add `--json` flag for structured JSON input (Nick Craig-Wood)
        * Add `--user` and `--pass` flags and interpret `--rc-user`, `--rc-pass`, `--rc-addr` (Nick Craig-Wood)
    * build
        * Require go1.8 or later for compilation (Nick Craig-Wood)
        * Enable softfloat on MIPS arch (Scott Edlund)
        * Integration test framework revamped with a better report and better retries (Nick Craig-Wood)
* Bug Fixes
    * cmd: Make --progress update the stats correctly at the end (Nick Craig-Wood)
    * config: Create config directory on save if it is missing (Nick Craig-Wood)
    * dedupe: Check for existing filename before renaming a dupe file (ssaqua)
    * move: Don't create directories with --dry-run (Nick Craig-Wood)
    * operations: Fix Purge and Rmdirs when dir is not the root (Nick Craig-Wood)
    * serve http/webdav/restic: Ensure rclone exits if the port is in use (Nick Craig-Wood)
* Mount
    * Make `--volname` work for Windows and macOS (Nick Craig-Wood)
* Azure Blob
    * Avoid context deadline exceeded error by setting a large TryTimeout value (brused27)
    * Fix erroneous Rmdir error "directory not empty" (Nick Craig-Wood)
    * Wait for up to 60s to create a just deleted container (Nick Craig-Wood)
* Dropbox
    * Add dropbox impersonate support (Jake Coggiano)
* Jottacloud
    * Fix bug in `--fast-list` handing of empty folders (albertony)
* Opendrive
    * Fix transfer of files with `+` and `&` in (Nick Craig-Wood)
    * Fix retries of upload chunks (Nick Craig-Wood)
* S3
    * Set ACL for server side copies to that provided by the user (Nick Craig-Wood)
    * Fix role_arn, credential_source, ... (Erik Swanson)
    * Add config info for Wasabi's US-West endpoint (Henry Ptasinski)
* SFTP
    * Ensure file hash checking is really disabled (Jon Fautley)
* Swift
    * Add pacer for retries to make swift more reliable (Nick Craig-Wood)
* WebDAV
    * Add Content-Type to PUT requests (Nick Craig-Wood)
    * Fix config parsing so `--webdav-user` and `--webdav-pass` flags work (Nick Craig-Wood)
    * Add RFC3339 date format (Ralf Hemberger)
* Yandex
    * The yandex backend was re-written (Sebastian Bünger)
        * This implements low level retries (Sebastian Bünger)
        * Copy, Move, DirMove, PublicLink and About optional interfaces (Sebastian Bünger)
        * Improved general error handling (Sebastian Bünger)
        * Removed ListR for now due to inconsistent behaviour (Sebastian Bünger)

## v1.44 - 2018-10-15

* New commands
    * serve ftp: Add ftp server (Antoine GIRARD)
    * settier: perform storage tier changes on supported remotes (sandeepkru)
* New Features
    * Reworked command line help
        * Make default help less verbose (Nick Craig-Wood)
        * Split flags up into global and backend flags (Nick Craig-Wood)
        * Implement specialised help for flags and backends (Nick Craig-Wood)
        * Show URL of backend help page when starting config (Nick Craig-Wood)
    * stats: Long names now split in center (Joanna Marek)
    * Add --log-format flag for more control over log output (dcpu)
    * rc: Add support for OPTIONS and basic CORS (frenos)
    * stats: show FatalErrors and NoRetryErrors in stats (Cédric Connes)
* Bug Fixes
    * Fix -P not ending with a new line (Nick Craig-Wood)
    * config: don't create default config dir when user supplies --config (albertony)
    * Don't print non-ASCII characters with --progress on windows (Nick Craig-Wood)
    * Correct logs for excluded items (ssaqua)
* Mount
    * Remove EXPERIMENTAL tags (Nick Craig-Wood)
* VFS
    * Fix race condition detected by serve ftp tests (Nick Craig-Wood)
    * Add vfs/poll-interval rc command (Fabian Möller)
    * Enable rename for nearly all remotes using server side Move or Copy (Nick Craig-Wood)
    * Reduce directory cache cleared by poll-interval (Fabian Möller)
    * Remove EXPERIMENTAL tags (Nick Craig-Wood)
* Local
    * Skip bad symlinks in dir listing with -L enabled (Cédric Connes)
    * Preallocate files on Windows to reduce fragmentation (Nick Craig-Wood)
    * Preallocate files on linux with fallocate(2) (Nick Craig-Wood)
* Cache
    * Add cache/fetch rc function (Fabian Möller)
    * Fix worker scale down (Fabian Möller)
    * Improve performance by not sending info requests for cached chunks (dcpu)
    * Fix error return value of cache/fetch rc method (Fabian Möller)
    * Documentation fix for cache-chunk-total-size (Anagh Kumar Baranwal)
    * Preserve leading / in wrapped remote path (Fabian Möller)
    * Add plex_insecure option to skip certificate validation (Fabian Möller)
    * Remove entries that no longer exist in the source (dcpu)
* Crypt
    * Preserve leading / in wrapped remote path (Fabian Möller)
* Alias
    * Fix handling of Windows network paths (Nick Craig-Wood)
* Azure Blob
    * Add --azureblob-list-chunk parameter (Santiago Rodríguez)
    * Implemented settier command support on azureblob remote. (sandeepkru)
    * Work around SDK bug which causes errors for chunk-sized files (Nick Craig-Wood)
* Box
    * Implement link sharing. (Sebastian Bünger)
* Drive
    * Add --drive-import-formats - google docs can now be imported (Fabian Möller)
        * Rewrite mime type and extension handling (Fabian Möller)
        * Add document links (Fabian Möller)
        * Add support for multipart document extensions (Fabian Möller)
        * Add support for apps-script to json export (Fabian Möller)
        * Fix escaped chars in documents during list (Fabian Möller)
    * Add --drive-v2-download-min-size a workaround for slow downloads (Fabian Möller)
    * Improve directory notifications in ChangeNotify (Fabian Möller)
    * When listing team drives in config, continue on failure (Nick Craig-Wood)
* FTP
    * Add a small pause after failed upload before deleting file (Nick Craig-Wood)
* Google Cloud Storage
    * Fix service_account_file being ignored (Fabian Möller)
* Jottacloud
    * Minor improvement in quota info (omit if unlimited) (albertony)
    * Add --fast-list support (albertony)
    * Add permanent delete support: --jottacloud-hard-delete (albertony)
    * Add link sharing support (albertony)
    * Fix handling of reserved characters. (Sebastian Bünger)
    * Fix socket leak on Object.Remove (Nick Craig-Wood)
* Onedrive
    * Rework to support Microsoft Graph (Cnly)
        * **NB** this will require re-authenticating the remote
    * Removed upload cutoff and always do session uploads (Oliver Heyme)
    * Use single-part upload for empty files (Cnly)
    * Fix new fields not saved when editing old config (Alex Chen)
    * Fix sometimes special chars in filenames not replaced (Alex Chen)
    * Ignore OneNote files by default (Alex Chen)
    * Add link sharing support (jackyzy823)
* S3
    * Use custom pacer, to retry operations when reasonable (Craig Miskell)
    * Use configured server-side-encryption and storace class options when calling CopyObject() (Paul Kohout)
    * Make --s3-v2-auth flag (Nick Craig-Wood)
    * Fix v2 auth on files with spaces (Nick Craig-Wood)
* Union
    * Implement union backend which reads from multiple backends (Felix Brucker)
    * Implement optional interfaces (Move, DirMove, Copy etc) (Nick Craig-Wood)
    * Fix ChangeNotify to support multiple remotes (Fabian Möller)
    * Fix --backup-dir on union backend (Nick Craig-Wood)
* WebDAV
    * Add another time format (Nick Craig-Wood)
    * Add a small pause after failed upload before deleting file (Nick Craig-Wood)
    * Add workaround for missing mtime (buergi)
    * Sharepoint: Renew cookies after 12hrs (Henning Surmeier)
* Yandex
    * Remove redundant nil checks (teresy)

## v1.43.1 - 2018-09-07

Point release to fix hubic and azureblob backends.

* Bug Fixes
    * ncdu: Return error instead of log.Fatal in Show (Fabian Möller)
    * cmd: Fix crash with --progress and --stats 0 (Nick Craig-Wood)
    * docs: Tidy website display (Anagh Kumar Baranwal)
* Azure Blob:
    * Fix multi-part uploads. (sandeepkru)
* Hubic
    * Fix uploads (Nick Craig-Wood)
    * Retry auth fetching if it fails to make hubic more reliable (Nick Craig-Wood)

## v1.43 - 2018-09-01

* New backends
    * Jottacloud (Sebastian Bünger)
* New commands
    * copyurl: copies a URL to a remote (Denis)
* New Features
    * Reworked config for backends (Nick Craig-Wood)
        * All backend config can now be supplied by command line, env var or config file
        * Advanced section in the config wizard for the optional items
        * A large step towards rclone backends being usable in other go software
        * Allow on the fly remotes with :backend: syntax
    * Stats revamp
        * Add `--progress`/`-P` flag to show interactive progress (Nick Craig-Wood)
        * Show the total progress of the sync in the stats (Nick Craig-Wood)
        * Add `--stats-one-line` flag for single line stats (Nick Craig-Wood)
    * Added weekday schedule into `--bwlimit` (Mateusz)
    * lsjson: Add option to show the original object IDs (Fabian Möller)
    * serve webdav: Make Content-Type without reading the file and add `--etag-hash` (Nick Craig-Wood)
    * build
        * Build macOS with native compiler (Nick Craig-Wood)
        * Update to use go1.11 for the build (Nick Craig-Wood)
    * rc
        * Added core/stats to return the stats (reddi1)
    * `version --check`: Prints the current release and beta versions (Nick Craig-Wood)
* Bug Fixes
    * accounting
        * Fix time to completion estimates (Nick Craig-Wood)
        * Fix moving average speed for file stats (Nick Craig-Wood)
    * config: Fix error reading password from piped input (Nick Craig-Wood)
    * move: Fix `--delete-empty-src-dirs` flag to delete all empty dirs on move (ishuah)
* Mount
    * Implement `--daemon-timeout` flag for OSXFUSE (Nick Craig-Wood)
    * Fix mount `--daemon` not working with encrypted config (Alex Chen)
    * Clip the number of blocks to 2^32-1 on macOS - fixes borg backup (Nick Craig-Wood)
* VFS
    * Enable vfs-read-chunk-size by default (Fabian Möller)
    * Add the vfs/refresh rc command (Fabian Möller)
    * Add non recursive mode to vfs/refresh rc command (Fabian Möller)
    * Try to seek buffer on read only files (Fabian Möller)
* Local
    * Fix crash when deprecated `--local-no-unicode-normalization` is supplied (Nick Craig-Wood)
    * Fix mkdir error when trying to copy files to the root of a drive on windows (Nick Craig-Wood)
* Cache
    * Fix nil pointer deref when using lsjson on cached directory (Nick Craig-Wood)
    * Fix nil pointer deref for occasional crash on playback (Nick Craig-Wood)
* Crypt
    * Fix accounting when checking hashes on upload (Nick Craig-Wood)
* Amazon Cloud Drive
    * Make very clear in the docs that rclone has no ACD keys (Nick Craig-Wood)
* Azure Blob
    * Add connection string and SAS URL auth (Nick Craig-Wood)
    * List the container to see if it exists (Nick Craig-Wood)
    * Port new Azure Blob Storage SDK (sandeepkru)
    * Added blob tier, tier between Hot, Cool and Archive. (sandeepkru)
    * Remove leading / from paths (Nick Craig-Wood)
* B2
    * Support Application Keys (Nick Craig-Wood)
    * Remove leading / from paths (Nick Craig-Wood)
* Box
    * Fix upload of > 2GB files on 32 bit platforms (Nick Craig-Wood)
    * Make `--box-commit-retries` flag defaulting to 100 to fix large uploads (Nick Craig-Wood)
* Drive
    * Add `--drive-keep-revision-forever` flag (lewapm)
    * Handle gdocs when filtering file names in list (Fabian Möller)
    * Support using `--fast-list` for large speedups (Fabian Möller)
* FTP
    * Fix Put mkParentDir failed: 521 for BunnyCDN (Nick Craig-Wood)
* Google Cloud Storage
    * Fix index out of range error with `--fast-list` (Nick Craig-Wood)
* Jottacloud
    * Fix MD5 error check (Oliver Heyme)
    * Handle empty time values (Martin Polden)
    * Calculate missing MD5s (Oliver Heyme)
    * Docs, fixes and tests for MD5 calculation (Nick Craig-Wood)
    * Add optional MimeTyper interface. (Sebastian Bünger)
    * Implement optional About interface (for `df` support). (Sebastian Bünger)
* Mega
    * Wait for events instead of arbitrary sleeping (Nick Craig-Wood)
    * Add `--mega-hard-delete` flag (Nick Craig-Wood)
    * Fix failed logins with upper case chars in email (Nick Craig-Wood)
* Onedrive
    * Shared folder support (Yoni Jah)
    * Implement DirMove (Cnly)
    * Fix rmdir sometimes deleting directories with contents (Nick Craig-Wood)
* Pcloud
    * Delete half uploaded files on upload error (Nick Craig-Wood)
* Qingstor
    * Remove leading / from paths (Nick Craig-Wood)
* S3
    * Fix index out of range error with `--fast-list` (Nick Craig-Wood)
    * Add `--s3-force-path-style` (Nick Craig-Wood)
    * Add support for KMS Key ID (bsteiss)
    * Remove leading / from paths (Nick Craig-Wood)
* Swift
    * Add `storage_policy` (Ruben Vandamme)
    * Make it so just `storage_url` or `auth_token` can be overidden (Nick Craig-Wood)
    * Fix server side copy bug for unusal file names (Nick Craig-Wood)
    * Remove leading / from paths (Nick Craig-Wood)
* WebDAV
    * Ensure we call MKCOL with a URL with a trailing / for QNAP interop (Nick Craig-Wood)
    * If root ends with / then don't check if it is a file (Nick Craig-Wood)
    * Don't accept redirects when reading metadata (Nick Craig-Wood)
    * Add bearer token (Macaroon) support for dCache (Nick Craig-Wood)
    * Document dCache and Macaroons (Onno Zweers)
    * Sharepoint recursion with different depth (Henning)
    * Attempt to remove failed uploads (Nick Craig-Wood)
* Yandex
    * Fix listing/deleting files in the root (Nick Craig-Wood)

## v1.42 - 2018-06-16

* New backends
    * OpenDrive (Oliver Heyme, Jakub Karlicek, ncw)
* New commands
    * deletefile command (Filip Bartodziej)
* New Features
    * copy, move: Copy single files directly, don't use `--files-from` work-around
        * this makes them much more efficient
    * Implement `--max-transfer` flag to quit transferring at a limit
        * make exit code 8 for `--max-transfer` exceeded
    * copy: copy empty source directories to destination (Ishuah Kariuki)
    * check: Add `--one-way` flag (Kasper Byrdal Nielsen)
    * Add siginfo handler for macOS for ctrl-T stats (kubatasiemski)
    * rc
        * add core/gc to run a garbage collection on demand
        * enable go profiling by default on the `--rc` port
        * return error from remote on failure
    * lsf
        * Add `--absolute` flag to add a leading / onto path names
        * Add `--csv` flag for compliant CSV output
        * Add 'm' format specifier to show the MimeType
        * Implement 'i' format for showing object ID
    * lsjson
        * Add MimeType to the output
        * Add ID field to output to show Object ID
    * Add `--retries-sleep` flag (Benjamin Joseph Dag)
    * Oauth tidy up web page and error handling (Henning Surmeier)
* Bug Fixes
    * Password prompt output with `--log-file` fixed for unix (Filip Bartodziej)
    * Calculate ModifyWindow each time on the fly to fix various problems (Stefan Breunig)
* Mount
    * Only print "File.rename error" if there actually is an error (Stefan Breunig)
    * Delay rename if file has open writers instead of failing outright (Stefan Breunig)
    * Ensure atexit gets run on interrupt
    * macOS enhancements
        * Make `--noappledouble` `--noapplexattr`
        * Add `--volname` flag and remove special chars from it
        * Make Get/List/Set/Remove xattr return ENOSYS for efficiency
        * Make `--daemon` work for macOS without CGO
* VFS
    * Add `--vfs-read-chunk-size` and `--vfs-read-chunk-size-limit` (Fabian Möller)
    * Fix ChangeNotify for new or changed folders (Fabian Möller)
* Local
    * Fix symlink/junction point directory handling under Windows
        * **NB** you will need to add `-L` to your command line to copy files with reparse points
* Cache
    * Add non cached dirs on notifications (Remus Bunduc)
    * Allow root to be expired from rc (Remus Bunduc)
    * Clean remaining empty folders from temp upload path (Remus Bunduc)
    * Cache lists using batch writes (Remus Bunduc)
    * Use secure websockets for HTTPS Plex addresses (John Clayton)
    * Reconnect plex websocket on failures (Remus Bunduc)
    * Fix panic when running without plex configs (Remus Bunduc)
    * Fix root folder caching (Remus Bunduc)
* Crypt
    * Check the crypted hash of files when uploading for extra data security
* Dropbox
    * Make Dropbox for business folders accessible using an initial `/` in the path
* Google Cloud Storage
    * Low level retry all operations if necessary
* Google Drive
    * Add `--drive-acknowledge-abuse` to download flagged files
    * Add `--drive-alternate-export` to fix large doc export
    * Don't attempt to choose Team Drives when using rclone config create
    * Fix change list polling with team drives
    * Fix ChangeNotify for folders (Fabian Möller)
    * Fix about (and df on a mount) for team drives
* Onedrive
    * Errorhandler for onedrive for business requests (Henning Surmeier)
* S3
    * Adjust upload concurrency with `--s3-upload-concurrency` (themylogin)
    * Fix `--s3-chunk-size` which was always using the minimum
* SFTP
    * Add `--ssh-path-override` flag (Piotr Oleszczyk)
    * Fix slow downloads for long latency connections
* Webdav
    * Add workarounds for biz.mail.ru
    * Ignore Reason-Phrase in status line to fix 4shared (Rodrigo)
    * Better error message generation

## v1.41 - 2018-04-28

* New backends
    * Mega support added
    * Webdav now supports SharePoint cookie authentication (hensur)
* New commands
    * link: create public link to files and folders (Stefan Breunig)
    * about: gets quota info from a remote (a-roussos, ncw)
    * hashsum: a generic tool for any hash to produce md5sum like output
* New Features
    * lsd: Add -R flag and fix and update docs for all ls commands
    * ncdu: added a "refresh" key - CTRL-L (Keith Goldfarb)
    * serve restic: Add append-only mode (Steve Kriss)
    * serve restic: Disallow overwriting files in append-only mode (Alexander Neumann)
    * serve restic: Print actual listener address (Matt Holt)
    * size: Add --json flag (Matthew Holt)
    * sync: implement --ignore-errors (Mateusz Pabian)
    * dedupe: Add dedupe largest functionality (Richard Yang)
    * fs: Extend SizeSuffix to include TB and PB for rclone about
    * fs: add --dump goroutines and --dump openfiles for debugging
    * rc: implement core/memstats to print internal memory usage info
    * rc: new call rc/pid (Michael P. Dubner)
* Compile
    * Drop support for go1.6
* Release
    * Fix `make tarball` (Chih-Hsuan Yen)
* Bug Fixes
    * filter: fix --min-age and --max-age together check
    * fs: limit MaxIdleConns and MaxIdleConnsPerHost in transport
    * lsd,lsf: make sure all times we output are in local time
    * rc: fix setting bwlimit to unlimited
    * rc: take note of the --rc-addr flag too as per the docs
* Mount
    * Use About to return the correct disk total/used/free (eg in `df`)
    * Set `--attr-timeout default` to `1s` - fixes:
        * rclone using too much memory
        * rclone not serving files to samba
        * excessive time listing directories
    * Fix `df -i` (upstream fix)
* VFS
    * Filter files `.` and `..` from directory listing
    * Only make the VFS cache if --vfs-cache-mode > Off
* Local
    * Add --local-no-check-updated to disable updated file checks
    * Retry remove on Windows sharing violation error
* Cache
    * Flush the memory cache after close
    * Purge file data on notification
    * Always forget parent dir for notifications
    * Integrate with Plex websocket
    * Add rc cache/stats (seuffert)
    * Add info log on notification 
* Box
    * Fix failure reading large directories - parse file/directory size as float
* Dropbox
    * Fix crypt+obfuscate on dropbox
    * Fix repeatedly uploading the same files
* FTP
    * Work around strange response from box FTP server
    * More workarounds for FTP servers to fix mkParentDir error
    * Fix no error on listing non-existent directory
* Google Cloud Storage
    * Add service_account_credentials (Matt Holt)
    * Detect bucket presence by listing it - minimises permissions needed
    * Ignore zero length directory markers
* Google Drive
    * Add service_account_credentials (Matt Holt)
    * Fix directory move leaving a hardlinked directory behind
    * Return proper google errors when Opening files
    * When initialized with a filepath, optional features used incorrect root path (Stefan Breunig)
* HTTP
    * Fix sync for servers which don't return Content-Length in HEAD
* Onedrive
    * Add QuickXorHash support for OneDrive for business
    * Fix socket leak in multipart session upload
* S3
    * Look in S3 named profile files for credentials
    * Add `--s3-disable-checksum` to disable checksum uploading (Chris Redekop)
    * Hierarchical configuration support (Giri Badanahatti)
    * Add in config for all the supported S3 providers
    * Add One Zone Infrequent Access storage class (Craig Rachel)
    * Add --use-server-modtime support (Peter Baumgartner)
    * Add --s3-chunk-size option to control multipart uploads
    * Ignore zero length directory markers
* SFTP
    * Update docs to match code, fix typos and clarify disable_hashcheck prompt (Michael G. Noll)
    * Update docs with Synology quirks
    * Fail soft with a debug on hash failure
* Swift
    * Add --use-server-modtime support (Peter Baumgartner)
* Webdav
    * Support SharePoint cookie authentication (hensur)
    * Strip leading and trailing / off root

## v1.40 - 2018-03-19

* New backends
    * Alias backend to create aliases for existing remote names (Fabian Möller)
* New commands
    * `lsf`: list for parsing purposes (Jakub Tasiemski)
        * by default this is a simple non recursive list of files and directories
        * it can be configured to add more info in an easy to parse way
    * `serve restic`: for serving a remote as a Restic REST endpoint
        * This enables restic to use any backends that rclone can access
        * Thanks Alexander Neumann for help, patches and review
    * `rc`: enable the remote control of a running rclone
        * The running rclone must be started with --rc and related flags.
        * Currently there is support for bwlimit, and flushing for mount and cache.
* New Features
    * `--max-delete` flag to add a delete threshold (Bjørn Erik Pedersen)
    * All backends now support RangeOption for ranged Open
        * `cat`: Use RangeOption for limited fetches to make more efficient
        * `cryptcheck`: make reading of nonce more efficient with RangeOption
    * serve http/webdav/restic
        * support SSL/TLS
        * add `--user` `--pass` and `--htpasswd` for authentication
    * `copy`/`move`: detect file size change during copy/move and abort transfer (ishuah)
    * `cryptdecode`: added option to return encrypted file names. (ishuah)
    * `lsjson`: add `--encrypted` to show encrypted name (Jakub Tasiemski)
    * Add `--stats-file-name-length` to specify the printed file name length for stats (Will Gunn)
* Compile
    * Code base was shuffled and factored
        * backends moved into a backend directory
        * large packages split up
        * See the CONTRIBUTING.md doc for info as to what lives where now
    * Update to using go1.10 as the default go version
    * Implement daily [full integration tests](https://pub.rclone.org/integration-tests/)
* Release
    * Include a source tarball and sign it and the binaries
    * Sign the git tags as part of the release process
    * Add .deb and .rpm packages as part of the build
    * Make a beta release for all branches on the main repo (but not pull requests)
* Bug Fixes
    * config: fixes errors on non existing config by loading config file only on first access
    * config: retry saving the config after failure (Mateusz)
    * sync: when using `--backup-dir` don't delete files if we can't set their modtime
        * this fixes odd behaviour with Dropbox and `--backup-dir`
    * fshttp: fix idle timeouts for HTTP connections
    * `serve http`: fix serving files with : in - fixes
    * Fix `--exclude-if-present` to ignore directories which it doesn't have permission for (Iakov Davydov)
    * Make accounting work properly with crypt and b2
    * remove `--no-traverse` flag because it is obsolete
* Mount
    * Add `--attr-timeout` flag to control attribute caching in kernel
        * this now defaults to 0 which is correct but less efficient
        * see [the mount docs](/commands/rclone_mount/#attribute-caching) for more info
    * Add `--daemon` flag to allow mount to run in the background (ishuah)
    * Fix: Return ENOSYS rather than EIO on attempted link
        * This fixes FileZilla accessing an rclone mount served over sftp.
    * Fix setting modtime twice
    * Mount tests now run on CI for Linux (mount & cmount)/Mac/Windows
    * Many bugs fixed in the VFS layer - see below
* VFS
    * Many fixes for `--vfs-cache-mode` writes and above
        * Update cached copy if we know it has changed (fixes stale data)
        * Clean path names before using them in the cache
        * Disable cache cleaner if `--vfs-cache-poll-interval=0`
        * Fill and clean the cache immediately on startup
    * Fix Windows opening every file when it stats the file
    * Fix applying modtime for an open Write Handle
    * Fix creation of files when truncating
    * Write 0 bytes when flushing unwritten handles to avoid race conditions in FUSE
    * Downgrade "poll-interval is not supported" message to Info
    * Make OpenFile and friends return EINVAL if O_RDONLY and O_TRUNC
* Local
    * Downgrade "invalid cross-device link: trying copy" to debug
    * Make DirMove return fs.ErrorCantDirMove to allow fallback to Copy for cross device
    * Fix race conditions updating the hashes
* Cache
    * Add support for polling - cache will update when remote changes on supported backends
    * Reduce log level for Plex api
    * Fix dir cache issue
    * Implement `--cache-db-wait-time` flag
    * Improve efficiency with RangeOption and RangeSeek
    * Fix dirmove with temp fs enabled
    * Notify vfs when using temp fs
    * Offline uploading
    * Remote control support for path flushing
* Amazon cloud drive
    * Rclone no longer has any working keys - disable integration tests
    * Implement DirChangeNotify to notify cache/vfs/mount of changes
* Azureblob
    * Don't check for bucket/container presense if listing was OK
        * this makes rclone do one less request per invocation
    * Improve accounting for chunked uploads
* Backblaze B2
    * Don't check for bucket/container presense if listing was OK
        * this makes rclone do one less request per invocation
* Box
    * Improve accounting for chunked uploads
* Dropbox
    * Fix custom oauth client parameters
* Google Cloud Storage
    * Don't check for bucket/container presense if listing was OK
        * this makes rclone do one less request per invocation
* Google Drive
    * Migrate to api v3 (Fabian Möller)
    * Add scope configuration and root folder selection
    * Add `--drive-impersonate` for service accounts
        * thanks to everyone who tested, explored and contributed docs
    * Add `--drive-use-created-date` to use created date as modified date (nbuchanan)
    * Request the export formats only when required
        * This makes rclone quicker when there are no google docs
    * Fix finding paths with latin1 chars (a workaround for a drive bug)
    * Fix copying of a single Google doc file
    * Fix `--drive-auth-owner-only` to look in all directories
* HTTP
    * Fix handling of directories with & in
* Onedrive
    * Removed upload cutoff and always do session uploads
        * this stops the creation of multiple versions on business onedrive
    * Overwrite object size value with real size when reading file. (Victor)
        * this fixes oddities when onedrive misreports the size of images
* Pcloud
    * Remove unused chunked upload flag and code
* Qingstor
    * Don't check for bucket/container presense if listing was OK
        * this makes rclone do one less request per invocation
* S3
    * Support hashes for multipart files (Chris Redekop)
    * Initial support for IBM COS (S3) (Giri Badanahatti)
    * Update docs to discourage use of v2 auth with CEPH and others
    * Don't check for bucket/container presense if listing was OK
        * this makes rclone do one less request per invocation
    * Fix server side copy and set modtime on files with + in
* SFTP
    * Add option to disable remote hash check command execution (Jon Fautley)
    * Add `--sftp-ask-password` flag to prompt for password when needed (Leo R. Lundgren)
    * Add `set_modtime` configuration option
    * Fix following of symlinks
    * Fix reading config file outside of Fs setup
    * Fix reading $USER in username fallback not $HOME
    * Fix running under crontab - Use correct OS way of reading username 
* Swift
    * Fix refresh of authentication token
        * in v1.39 a bug was introduced which ignored new tokens - this fixes it
    * Fix extra HEAD transaction when uploading a new file
    * Don't check for bucket/container presense if listing was OK
        * this makes rclone do one less request per invocation
* Webdav
    * Add new time formats to support mydrive.ch and others

## v1.39 - 2017-12-23

* New backends
    * WebDAV
        * tested with nextcloud, owncloud, put.io and others!
    * Pcloud
    * cache - wraps a cache around other backends (Remus Bunduc)
        * useful in combination with mount
        * NB this feature is in beta so use with care
* New commands
    * serve command with subcommands:
        * serve webdav: this implements a webdav server for any rclone remote.
        * serve http: command to serve a remote over HTTP
    * config: add sub commands for full config file management
        * create/delete/dump/edit/file/password/providers/show/update
    * touch: to create or update the timestamp of a file (Jakub Tasiemski)
* New Features
    * curl install for rclone (Filip Bartodziej)
    * --stats now shows percentage, size, rate and ETA in condensed form (Ishuah Kariuki)
    * --exclude-if-present to exclude a directory if a file is present (Iakov Davydov)
    * rmdirs: add --leave-root flag (lewpam)
    * move: add --delete-empty-src-dirs flag to remove dirs after move (Ishuah Kariuki)
    * Add --dump flag, introduce --dump requests, responses and remove --dump-auth, --dump-filters
        * Obscure X-Auth-Token: from headers when dumping too
    * Document and implement exit codes for different failure modes (Ishuah Kariuki)
* Compile
* Bug Fixes
    * Retry lots more different types of errors to make multipart transfers more reliable
    * Save the config before asking for a token, fixes disappearing oauth config
    * Warn the user if --include and --exclude are used together (Ernest Borowski)
    * Fix duplicate files (eg on Google drive) causing spurious copies
    * Allow trailing and leading whitespace for passwords (Jason Rose)
    * ncdu: fix crashes on empty directories
    * rcat: fix goroutine leak
    * moveto/copyto: Fix to allow copying to the same name
* Mount
    * --vfs-cache mode to make writes into mounts more reliable.
        * this requires caching files on the disk (see --cache-dir)
        * As this is a new feature, use with care
    * Use sdnotify to signal systemd the mount is ready (Fabian Möller)
    * Check if directory is not empty before mounting (Ernest Borowski)
* Local
    * Add error message for cross file system moves
    * Fix equality check for times
* Dropbox
    * Rework multipart upload
        * buffer the chunks when uploading large files so they can be retried
        * change default chunk size to 48MB now we are buffering them in memory
        * retry every error after the first chunk is done successfully
    * Fix error when renaming directories
* Swift
    * Fix crash on bad authentication
* Google Drive
    * Add service account support (Tim Cooijmans)
* S3
    * Make it work properly with Digital Ocean Spaces (Andrew Starr-Bochicchio)
    * Fix crash if a bad listing is received
    * Add support for ECS task IAM roles (David Minor)
* Backblaze B2
    * Fix multipart upload retries
    * Fix --hard-delete to make it work 100% of the time
* Swift
    * Allow authentication with storage URL and auth key (Giovanni Pizzi)
    * Add new fields for swift configuration to support IBM Bluemix Swift (Pierre Carlson)
    * Add OS_TENANT_ID and OS_USER_ID to config
    * Allow configs with user id instead of user name
    * Check if swift segments container exists before creating (John Leach)
    * Fix memory leak in swift transfers (upstream fix)
* SFTP
    * Add option to enable the use of aes128-cbc cipher (Jon Fautley)
* Amazon cloud drive
    * Fix download of large files failing with "Only one auth mechanism allowed"
* crypt
    * Option to encrypt directory names or leave them intact
    * Implement DirChangeNotify (Fabian Möller)
* onedrive
    * Add option to choose resourceURL during setup of OneDrive Business account if more than one is available for user

## v1.38 - 2017-09-30

* New backends
    * Azure Blob Storage (thanks Andrei Dragomir)
    * Box
    * Onedrive for Business (thanks Oliver Heyme)
    * QingStor from QingCloud (thanks wuyu)
* New commands
    * `rcat` - read from standard input and stream upload
    * `tree` - shows a nicely formatted recursive listing
    * `cryptdecode` - decode crypted file names (thanks ishuah)
    * `config show` - print the config file
    * `config file` - print the config file location
* New Features
    * Empty directories are deleted on `sync`
    * `dedupe` - implement merging of duplicate directories
    * `check` and `cryptcheck` made more consistent and use less memory
    * `cleanup` for remaining remotes (thanks ishuah)
    * `--immutable` for ensuring that files don't change (thanks Jacob McNamee)
    * `--user-agent` option (thanks Alex McGrath Kraak)
    * `--disable` flag to disable optional features
    * `--bind` flag for choosing the local addr on outgoing connections
    * Support for zsh auto-completion (thanks bpicode)
    * Stop normalizing file names but do a normalized compare in `sync`
* Compile
    * Update to using go1.9 as the default go version
    * Remove snapd build due to maintenance problems
* Bug Fixes
    * Improve retriable error detection which makes multipart uploads better
    * Make `check` obey `--ignore-size`
    * Fix bwlimit toggle in conjunction with schedules (thanks cbruegg)
    * `config` ensures newly written config is on the same mount
* Local
    * Revert to copy when moving file across file system boundaries
    * `--skip-links` to suppress symlink warnings (thanks Zhiming Wang)
* Mount
    * Re-use `rcat` internals to support uploads from all remotes
* Dropbox
    * Fix "entry doesn't belong in directory" error
    * Stop using deprecated API methods
* Swift
    * Fix server side copy to empty container with `--fast-list`
* Google Drive
    * Change the default for `--drive-use-trash` to `true`
* S3
    * Set session token when using STS (thanks Girish Ramakrishnan)
    * Glacier docs and error messages (thanks Jan Varho)
    * Read 1000 (not 1024) items in dir listings to fix Wasabi
* Backblaze B2
    * Fix SHA1 mismatch when downloading files with no SHA1
    * Calculate missing hashes on the fly instead of spooling
    * `--b2-hard-delete` to permanently delete (not hide) files (thanks John Papandriopoulos)
* Hubic
    * Fix creating containers - no longer have to use the `default` container
* Swift
    * Optionally configure from a standard set of OpenStack environment vars
    * Add `endpoint_type` config
* Google Cloud Storage
    * Fix bucket creation to work with limited permission users
* SFTP
    * Implement connection pooling for multiple ssh connections
    * Limit new connections per second
    * Add support for MD5 and SHA1 hashes where available (thanks Christian Brüggemann)
* HTTP
    * Fix URL encoding issues
    * Fix directories with `:` in
    * Fix panic with URL encoded content

## v1.37 - 2017-07-22

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
        * This will use more memory as it has to hold the listing in memory
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

## v1.36 - 2017-03-18

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

## v1.35 - 2017-01-02

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

## v1.34 - 2016-11-06

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

## v1.33 - 2016-08-24

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

## v1.32 - 2016-07-13

* Backblaze B2
    * Fix upload of files large files not in root

## v1.31 - 2016-07-13

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

## v1.30 - 2016-06-18

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

## v1.29 - 2016-04-18

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

## v1.28 - 2016-03-01

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

## v1.27 - 2016-01-31

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

## v1.26 - 2016-01-02

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

## v1.25 - 2015-11-14

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

## v1.24 - 2015-11-07

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

## v1.23 - 2015-10-03

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

## v1.22 - 2015-09-28

* Implement rsync like include and exclude flags
* swift
    * Support files > 5GB - thanks Sergey Tolmachev

## v1.21 - 2015-09-22

* New features
    * Display individual transfer progress
    * Make lsl output times in localtime
* Fixes
    * Fix allowing user to override credentials again in Drive, GCS and ACD
* Amazon Drive
    * Implement compliant pacing scheme
* Google Drive
    * Make directory reads concurrent for increased speed.

## v1.20 - 2015-09-15

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

## v1.19 - 2015-08-28

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

## v1.18 - 2015-08-17

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
    * many thanks to Sam Liston and Brian Haymore at the [Utah Center for High Performance Computing](https://www.chpc.utah.edu/) for a Ceph test account
* misc
    * Show errors when reading the config file
    * Do not print stats in quiet mode - thanks Leonid Shalupov
    * Add FAQ
    * Fix created directories not obeying umask
    * Linux installation instructions - thanks Shimon Doodkin

## v1.17 - 2015-06-14

* dropbox: fix case insensitivity issues - thanks Leonid Shalupov

## v1.16 - 2015-06-09

* Fix uploading big files which was causing timeouts or panics
* Don't check md5sum after download with --size-only

## v1.15 - 2015-06-06

* Add --checksum flag to only discard transfers by MD5SUM - thanks Alex Couper
* Implement --size-only flag to sync on size not checksum & modtime
* Expand docs and remove duplicated information
* Document rclone's limitations with directories
* dropbox: update docs about case insensitivity

## v1.14 - 2015-05-21

* local: fix encoding of non utf-8 file names - fixes a duplicate file problem
* drive: docs about rate limiting
* google cloud storage: Fix compile after API change in "google.golang.org/api/storage/v1"

## v1.13 - 2015-05-10

* Revise documentation (especially sync)
* Implement --timeout and --conntimeout
* s3: ignore etags from multipart uploads which aren't md5sums

## v1.12 - 2015-03-15

* drive: Use chunked upload for files above a certain size
* drive: add --drive-chunk-size and --drive-upload-cutoff parameters
* drive: switch to insert from update when a failed copy deletes the upload
* core: Log duplicate files if they are detected

## v1.11 - 2015-03-04

* swift: add region parameter
* drive: fix crash on failed to update remote mtime
* In remote paths, change native directory separators to /
* Add synchronization to ls/lsl/lsd output to stop corruptions
* Ensure all stats/log messages to go stderr
* Add --log-file flag to log everything (including panics) to file
* Make it possible to disable stats printing with --stats=0
* Implement --bwlimit to limit data transfer bandwidth

## v1.10 - 2015-02-12

* s3: list an unlimited number of items
* Fix getting stuck in the configurator

## v1.09 - 2015-02-07

* windows: Stop drive letters (eg C:) getting mixed up with remotes (eg drive:)
* local: Fix directory separators on Windows
* drive: fix rate limit exceeded errors

## v1.08 - 2015-02-04

* drive: fix subdirectory listing to not list entire drive
* drive: Fix SetModTime
* dropbox: adapt code to recent library changes

## v1.07 - 2014-12-23

* google cloud storage: fix memory leak

## v1.06 - 2014-12-12

* Fix "Couldn't find home directory" on OSX
* swift: Add tenant parameter
* Use new location of Google API packages

## v1.05 - 2014-08-09

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

## v1.04 - 2014-07-21

* google cloud storage: Fix crash on Update

## v1.03 - 2014-07-20

* swift, s3, dropbox: fix updated files being marked as corrupted
* Make compile with go 1.1 again

## v1.02 - 2014-07-19

* Implement Dropbox remote
* Implement Google Cloud Storage remote
* Verify Md5sums and Sizes after copies
* Remove times from "ls" command - lists sizes only
* Add add "lsl" - lists times and sizes
* Add "md5sum" command

## v1.01 - 2014-07-04

* drive: fix transfer of big files using up lots of memory

## v1.00 - 2014-07-03

* drive: fix whole second dates

## v0.99 - 2014-06-26

* Fix --dry-run not working
* Make compatible with go 1.1

## v0.98 - 2014-05-30

* s3: Treat missing Content-Length as 0 for some ceph installations
* rclonetest: add file with a space in

## v0.97 - 2014-05-05

* Implement copying of single files
* s3 & swift: support paths inside containers/buckets

## v0.96 - 2014-04-24

* drive: Fix multiple files of same name being created
* drive: Use o.Update and fs.Put to optimise transfers
* Add version number, -V and --version

## v0.95 - 2014-03-28

* rclone.org: website, docs and graphics
* drive: fix path parsing

## v0.94 - 2014-03-27

* Change remote format one last time
* GNU style flags

## v0.93 - 2014-03-16

* drive: store token in config file
* cross compile other versions
* set strict permissions on config file

## v0.92 - 2014-03-15

* Config fixes and --config option

## v0.91 - 2014-03-15

* Make config file

## v0.90 - 2013-06-27

* Project named rclone

## v0.00 - 2012-11-18

* Project started

