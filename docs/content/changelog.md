---
title: "Documentation"
description: "Rclone Changelog"
---

# Changelog

## v1.68.0 - 2024-09-08

[See commits](https://github.com/rclone/rclone/compare/v1.67.0...v1.68.0)

* New backends
    * [Files.com](/filescom) (Sam Harrison)
    * [Gofile](/gofile/) (Nick Craig-Wood)
    * [Pixeldrain](/pixeldrain/) (Fornax)
* Changed backends
    * [S3](/s3/) backend updated to use [AWS SDKv2](https://github.com/aws/aws-sdk-go-v2) as v1 is now unsupported.
        * The matrix of providers and auth methods is huge and there could be problems with obscure combinations.
        * Please report problems in a [new issue](https://github.com/rclone/rclone/issues/new/choose) on Github. 
* New commands
    * [config encryption](/commands/rclone_config_encryption/): set, remove and check to manage config file encryption (Nick Craig-Wood)
* New Features
    * build
        * Update to go1.23 and make go1.21 the minimum required version (Nick Craig-Wood)
        * Update all dependencies (Nick Craig-Wood)
        * Disable wasm/js build due to [go bug #64856](https://github.com/golang/go/issues/64856) (Nick Craig-Wood)
        * Enable custom linting rules with ruleguard via gocritic (albertony)
        * Update logging statements to make `--use-json-log` work always (albertony)
        * Adding new code quality tests and fixing the fallout (albertony)
    * config
        * Internal config re-organised to be more consistent and make it available from the rc (Nick Craig-Wood)
        * Avoid remotes with empty names from the environment (albertony)
        * Make listing of remotes more consistent (albertony)
        * Make getting config values more consistent (albertony)
        * Use `--password-command` to set config file password if supplied (Nick Craig-Wood)
    * doc fixes (albertony, crystalstall, David Seifert, Eng Zer Jun, Ernie Hershey, Florian Klink, John Oxley, kapitainsky, Mathieu Moreau, Nick Craig-Wood, nipil, Pétr Bozsó, Russ Bubley, Sam Harrison, Thearas, URenko, Will Miles, yuval-cloudinary)
    * fs: Allow semicolons as well as spaces in `--bwlimit` timetable parsing (Kyle Reynolds)
    * help
        * Global flags help command now takes glob filter (albertony)
        * Make help command output less distracting (albertony)
    * lib/encoder: Add Raw encoding for use where no encoding at all is required, eg `--local-encoding Raw` (URenko)
    * listremotes: Added options for filtering, ordering and json output (albertony)
    * nfsmount
        * Make the `--sudo` flag work for umount as well as mount (Nick Craig-Wood)
        * Add `-o tcp` option to NFS mount options to fix mounting under Linux (Nick Craig-Wood)
    * operations: copy: generate stable partial suffix (Georg Welzel)
    * rc
        * Add [options/info](/rc/#options-info) call to enumerate options (Nick Craig-Wood)
        * Add option blocks parameter to [options/get](/rc/#options-get) and [options/info](/rc/#options-info) (Nick Craig-Wood)
        * Add [vfs/queue](/rc/#vfs-queue) to show the status of the upload queue (Nick Craig-Wood)
        * Add [vfs/queue-set-expiry](/rc/#vfs-queue-set-expiry) to adjust expiry of items in the VFS queue (Nick Craig-Wood)
        * Add `--unix-socket` option to `rc` command (Florian Klink)
        * Prevent unmount rc command from sending a `STOPPING=1` sd-notify message (AThePeanut4)
    * rcserver: Implement [prometheus metrics](/docs/#metrics) on a dedicated port (Oleg Kunitsyn)
    * serve dlna
        * Also look at "Subs" subdirectory (Florian Klink)
        * Don't swallow `video.{idx,sub}` (Florian Klink)
        * Set more correct mime type (Florian Klink)
    * serve nfs
        * Implement on disk cache for file handles selected with `--nfs-cache-type` (Nick Craig-Wood)
        * Add tracing to filesystem calls (Nick Craig-Wood)
        * Mask unimplemented error from chmod (Nick Craig-Wood)
        * Unify the nfs library logging with rclone's logging better (Nick Craig-Wood)
        * Fix incorrect user id and group id exported to NFS (Nick Craig-Wood)
    * serve s3
        * Implement `--auth-proxy` (Sawjan Gurung)
        * Update to AWS SDKv2 by updating `github.com/rclone/gofakes3` (Nick Craig-Wood)
* Bug Fixes
    * bisync: Fix sync time problems with backends that round time (eg Dropbox) (nielash)
    * serve dlna: Fix panic: invalid argument to Int63n (Nick Craig-Wood)
* VFS
    * Add [--vfs-read-chunk-streams](/commands/rclone_mount/#vfs-read-chunk-streams-0-1) to parallel read chunks from files (Nick Craig-Wood)
        * This can increase mount performance on high bandwidth or large latency links
    * Fix cache encoding with special characters (URenko)
* Local
    * Fix encoding of root path fix (URenko)
    * Add server-side copy (using clone) with xattrs on macOS (nielash)
        * `--local-no-clone` flag to disable cloning for server-side copies (nielash)
    * Support setting custom `--metadata` during server-side Copy (nielash)
* Azure Blob
    * Allow anonymous access for public resources (Nick Craig-Wood)
* B2
    * Include custom upload headers in large file info (Pat Patterson)
* Drive
    * Fix copying Google Docs to a backend which only supports SHA1 (Nick Craig-Wood)
* Fichier
    * Fix detection of Flood Detected error (Nick Craig-Wood)
    * Fix server side move (Nick Craig-Wood)
* HTTP
    * Reload client certificates on expiry (Saleh Dindar)
    * Support listening on passed FDs (Florian Klink)
* Jottacloud
    * Fix setting of metadata on server side move (albertony)
* Onedrive
    * Fix nil pointer error when uploading small files (Nick Craig-Wood)
* Pcloud
    * Implement `SetModTime` (Georg Welzel)
    * Implement `OpenWriterAt` feature to enable multipart uploads (Georg Welzel)
* Pikpak
    * Improve data consistency by ensuring async tasks complete (wiserain)
    * Implement custom hash to replace wrong sha1 (wiserain)
    * Fix error with `copyto` command (wiserain)
    * Optimize file move by removing unnecessary `readMetaData()` call (wiserain)
    * Non-buffered hash calculation for local source files (wiserain)
    * Optimize upload by pre-fetching gcid from API (wiserain)
    * Correct file transfer progress for uploads by hash (wiserain)
    * Update to using AWS SDK v2 (wiserain)
* S3
    * Update to using AWS SDK v2 (Nick Craig-Wood)
        * Add `--s3-sdk-log-mode` to control SDKv2 debugging (Nick Craig-Wood)
    * Fix incorrect region for Magalu provider (Filipe Herculano)
    * Allow restoring from intelligent-tiering storage class (Pawel Palucha)
* SFTP
    * Use `uint32` for mtime to save memory (Tomasz Melcer)
    * Ignore useless errors when closing the connection pool (Nick Craig-Wood)
    * Support listening on passed FDs (Florian Klink)
* Swift
    * Add workarounds for bad listings in Ceph RGW (Paul Collins)
    * Add total/free space info in `about` command. (fsantagostinobietti)
* Ulozto
    * Fix upload of > 2GB files on 32 bit platforms (Tobias Markus)
* WebDAV
    * Add `--webdav-unix-socket-path` to connect to a unix socket (Florian Klink)
* Yandex
    * Implement custom user agent to help with upload speeds (Sebastian Bünger)
* Zoho
    * Fix inefficiencies uploading with new API to avoid throttling (Nick Craig-Wood)

## v1.67.0 - 2024-06-14

[See commits](https://github.com/rclone/rclone/compare/v1.66.0...v1.67.0)

* New backends
    * [uloz.to](/ulozto/) (iotmaestro)
    * New S3 providers
        * [Magalu Object Storage](/s3/#magalu) (Bruno Fernandes)
* New commands
    * [gitannex](/commands/rclone_gitannex/): Enables git-annex to store and retrieve content from an rclone remote (Dan McArdle)
* New Features
    * accounting: Add deleted files total size to status summary line (Kyle Reynolds)
    * build
        * Fix `CVE-2023-45288` by upgrading `golang.org/x/net` (Nick Craig-Wood)
        * Fix `CVE-2024-35255` by upgrading `github.com/Azure/azure-sdk-for-go/sdk/azidentity` to 1.6.0 (dependabot)
        * Convert source files with CRLF to LF (albertony)
        * Update all dependencies (Nick Craig-Wood)
    * doc updates (albertony, Alex Garel, Dave Nicolson, Dominik Joe Pantůček, Eric Wolf, Erisa A, Evan Harris, Evan McBeth, Gachoud Philippe, hidewrong, jakzoe, jumbi77, kapitainsky, Kyle Reynolds, Lewis Hook, Nick Craig-Wood, overallteach, pawsey-kbuckley, Pieter van Oostrum, psychopatt, racerole, static-moonlight, Warrentheo, yudrywet, yumeiyin )
    * ncdu: Do not quit on Esc to aid usability (Katia Esposito)
    * rcserver: Set `ModTime` for dirs and files served by `--rc-serve` (Nikita Shoshin)
* Bug Fixes
    * bisync: Add integration tests against all backends and fix many many problems (nielash)
    * config: Fix default value for `description` (Nick Craig-Wood)
    * copy: Fix `nil` pointer dereference when corrupted on transfer with `nil` dst (nielash)
    * fs
        * Improve JSON Unmarshalling for `Duration` types (Kyle Reynolds)
        * Close the CPU profile on exit (guangwu)
        * Replace `/bin/bash` with `/usr/bin/env bash` (Florian Klink)
    * oauthutil: Clear client secret if client ID is set (Michael Terry)
    * operations
        * Rework `rcat` so that it doesn't call the `--metadata-mapper` twice (Nick Craig-Wood)
        * Ensure `SrcFsType` is set correctly when using `--metadata-mapper` (Nick Craig-Wood)
        * Fix "optional feature not implemented" error with a crypted sftp bug (Nick Craig-Wood)
        * Fix very long file names when using copy with `--partial` (Nick Craig-Wood)
        * Fix retries downloading too much data with certain backends (Nick Craig-Wood)
        * Fix move when dst is nil and fdst is case-insensitive (nielash)
        * Fix lsjson `--encrypted` when using `--crypt-XXX` parameters (Nick Craig-Wood)
        * Fix missing metadata for multipart transfers to local disk (Nick Craig-Wood)
        * Fix incorrect modtime on some multipart transfers (Nick Craig-Wood)
        * Fix hashing problem in integration tests (Nick Craig-Wood)
    * rc
        * Fix stats groups being ignored in `operations/check` (Nick Craig-Wood)
        * Fix incorrect `Content-Type` in HTTP API (Kyle Reynolds)
    * serve s3
        * Fix `Last-Modified` header format (Butanediol)
        * Fix in-memory metadata storing wrong modtime (nielash)
        * Fix XML of error message (Nick Craig-Wood)
    * serve webdav: Fix webdav with `--baseurl` under Windows (Nick Craig-Wood)
    * serve dlna: Make `BrowseMetadata` more compliant (albertony)
    * serve http: Added `Content-Length` header when HTML directory is served (Sunny)
    * sync
        * Don't sync directories if they haven't been modified (Nick Craig-Wood)
        * Don't test reading metadata if we can't write it (Nick Craig-Wood)
        * Fix case normalisation (problem on on s3) (Nick Craig-Wood)
        * Fix management of empty directories to make it more accurate (Nick Craig-Wood)
        * Fix creation of empty directories when `--create-empty-src-dirs=false` (Nick Craig-Wood)
        * Fix directory modification times not being set (Nick Craig-Wood)
        * Fix "failed to update directory timestamp or metadata: directory not found" (Nick Craig-Wood)
        * Fix expecting SFTP to have MkdirMetadata method: optional feature not implemented (Nick Craig-Wood)
    * test info: Improve cleanup of temp files (Kyle Reynolds)
    * touch: Fix using `-R` on certain backends (Nick Craig-Wood)
* Mount
    * Add `--direct-io` flag to force uncached access (Nick Craig-Wood)
* VFS
    * Fix download loop when file size shrunk (Nick Craig-Wood)
    * Fix renaming a directory (nielash)
* Local
    * Add `--local-time-type` to use `mtime`/`atime`/`btime`/`ctime` as the time (Nick Craig-Wood)
    * Allow `SeBackupPrivilege` and/or `SeRestorePrivilege` to work on Windows (Charles Hamilton)
* Azure Blob
    * Fix encoding issue with dir path comparison (nielash)
* B2
    * Add new [cleanup](/b2/#cleanup) and [cleanup-hidden](/b2/#cleanup-hidden) backend commands. (Pat Patterson)
    * Update B2 URLs to new home (Nick Craig-Wood)
* Chunker
    * Fix startup when root points to composite multi-chunk file without metadata (nielash)
    * Fix case-insensitive comparison on local without metadata (nielash)
    * Fix "finalizer already set" error (nielash)
* Drive
    * Add [backend query](/drive/#query) command for general purpose querying of files (John-Paul Smith)
    * Stop sending notification emails when setting permissions (Nick Craig-Wood)
    * Fix server side copy with metadata from my drive to shared drive (Nick Craig-Wood)
    * Set all metadata permissions and return error summary instead of stopping on the first error (Nick Craig-Wood)
    * Make errors setting permissions into no retry errors (Nick Craig-Wood)
    * Fix description being overwritten on server side moves (Nick Craig-Wood)
    * Allow setting metadata to fail if `failok` flag is set (Nick Craig-Wood)
    * Fix panic when using `--metadata-mapper` on large google doc files (Nick Craig-Wood)
* Dropbox
    * Add `--dropbox-root-namespace` to override the root namespace (Bill Fraser)
* Google Cloud Storage
    * Fix encoding issue with dir path comparison (nielash)
* Hdfs
    * Fix f.String() not including subpath (nielash)
* Http
    * Add `--http-no-escape` to not escape URL metacharacters in path names (Kyle Reynolds)
* Jottacloud
    * Set metadata on server side copy and move (albertony)
* Linkbox
    * Fix working with names longer than 8-25 Unicode chars. (Vitaly)
    * Fix list paging and optimized synchronization. (gvitali)
* Mailru
    * Attempt to fix throttling by increasing min sleep to 100ms (Nick Craig-Wood)
* Memory
    * Fix dst mutating src after server-side copy (nielash)
    * Fix deadlock in operations.Purge (nielash)
    * Fix incorrect list entries when rooted at subdirectory (nielash)
* Onedrive
    * Add `--onedrive-hard-delete` to permanently delete files (Nick Craig-Wood)
    * Make server-side copy work in more scenarios (YukiUnHappy)
    * Fix "unauthenticated: Unauthenticated" errors when downloading (Nick Craig-Wood)
    * Fix `--metadata-mapper` being called twice if writing permissions (nielash)
    * Set all metadata permissions and return error summary instead of stopping on the first error (nielash)
    * Make errors setting permissions into no retry errors (Nick Craig-Wood)
    * Skip writing permissions with 'owner' role (nielash)
    * Fix references to deprecated permissions properties (nielash)
    * Add support for group permissions (nielash)
    * Allow setting permissions to fail if `failok` flag is set (Nick Craig-Wood)
* Pikpak
    * Make getFile() usage more efficient to avoid the download limit (wiserain)
    * Improve upload reliability and resolve potential file conflicts (wiserain)
    * Implement configurable chunk size for multipart upload (wiserain)
* Protondrive
    * Don't auth with an empty access token (Michał Dzienisiewicz)
* Qingstor
    * Disable integration tests as test account suspended (Nick Craig-Wood)
* Quatrix
    * Fix f.String() not including subpath (nielash)
* S3
    * Add new AWS region `il-central-1` Tel Aviv (yoelvini)
    * Update Scaleway's configuration options (Alexandre Lavigne)
    * Ceph: fix quirks when creating buckets to fix trying to create an existing bucket (Thomas Schneider)
    * Fix encoding issue with dir path comparison (nielash)
    * Fix 405 error on HEAD for delete marker with versionId (nielash)
    * Validate `--s3-copy-cutoff` size before copy (hoyho)
* SFTP
    * Add `--sftp-connections` to limit the maximum number of connections (Tomasz Melcer)
* Storj
    * Update `storj.io/uplink` to latest release (JT Olio)
    * Update bio on request (Nick Craig-Wood)
* Swift
    * Implement `--swift-use-segments-container` to allow >5G files on Blomp (Nick Craig-Wood)
* Union
    * Fix deleting dirs when all remotes can't have empty dirs (Nick Craig-Wood)
* WebDAV
    * Fix setting modification times erasing checksums on owncloud and nextcloud (nielash)
    * owncloud: Add `--webdav-owncloud-exclude-mounts` which allows excluding mounted folders when listing remote resources (Thomas Müller)
* Zoho
    * Fix throttling problem when uploading files (Nick Craig-Wood)
    * Use cursor listing for improved performance (Nick Craig-Wood)
    * Retry reading info after upload if size wasn't returned (Nick Craig-Wood)
    * Remove simple file names complication which is no longer needed (Nick Craig-Wood)
    * Sleep for 60 seconds if rate limit error received (Nick Craig-Wood)

## v1.66.0 - 2024-03-10

[See commits](https://github.com/rclone/rclone/compare/v1.65.0...v1.66.0)

* Major features
    * Rclone will now sync directory modification times if the backend supports it.
        * This can be disabled with [--no-update-dir-modtime](/docs/#no-update-dir-modtime)
        * See [the overview](/overview/#features) and look for the `D` flags in the `ModTime` column to see which backends support it.
    * Rclone will now sync directory metadata if the backend supports it when `-M`/`--metadata` is in use.
        * See [the overview](/overview/#features) and look for the `D` flags in the `Metadata` column to see which backends support it.
    * Bisync has received many updates see below for more details or [bisync's changelog](/bisync/#changelog)
* Removed backends
    * amazonclouddrive: Remove Amazon Drive backend code and docs (Nick Craig-Wood)
* New Features
    * backend
        * Add description field for all backends (Paul Stern)
    * build
        * Update to go1.22 and make go1.20 the minimum required version (Nick Craig-Wood)
        * Fix `CVE-2024-24786` by upgrading `google.golang.org/protobuf` (Nick Craig-Wood)
    * check: Respect `--no-unicode-normalization` and `--ignore-case-sync` for `--checkfile` (nielash)
    * cmd: Much improved shell auto completion which reduces the size of the completion file and works faster (Nick Craig-Wood)
    * doc updates (albertony, ben-ba, Eli, emyarod, huajin tong, Jack Provance, kapitainsky, keongalvin, Nick Craig-Wood, nielash, rarspace01, rzitzer, Tera, Vincent Murphy)
    * fs: Add more detailed logging for file includes/excludes (Kyle Reynolds)
    * lsf
        * Add `--time-format` flag (nielash)
        * Make metadata appear for directories (Nick Craig-Wood)
    * lsjson: Make metadata appear for directories (Nick Craig-Wood)
    * rc
        * Add `srcFs` and `dstFs` to `core/stats` and `core/transferred` stats (Nick Craig-Wood)
        * Add `operations/hashsum` to the rc as `rclone hashsum` equivalent (Nick Craig-Wood)
        * Add `config/paths` to the rc as `rclone config paths` equivalent (Nick Craig-Wood)
    * sync
        * Optionally report list of synced paths to file (nielash)
        * Implement directory sync for mod times and metadata (Nick Craig-Wood)
        * Don't set directory modtimes if already set (nielash)
        * Don't sync directory modtimes from backends which don't have directories (Nick Craig-Wood)
* Bug Fixes
    * backend
        * Make backends which use oauth implement the `Shutdown` and shutdown the oauth properly (rkonfj)
    * bisync
        * Handle unicode and case normalization consistently (nielash)
        * Partial uploads known issue on `local`/`ftp`/`sftp` has been resolved (unless using `--inplace`) (nielash)
        * Fixed handling of unicode normalization and case insensitivity, support for [`--fix-case`](/docs/#fix-case), [`--ignore-case-sync`](/docs/#ignore-case-sync), [`--no-unicode-normalization`](/docs/#no-unicode-normalization) (nielash)
        * Bisync no longer fails to find the correct listing file when configs are overridden with backend-specific flags. (nielash)
    * nfsmount
        * Fix exit after external unmount (nielash)
        * Fix `--volname` being ignored (nielash)
    * operations
        * Fix renaming a file on macOS (nielash)
        * Fix case-insensitive moves in operations.Move (nielash)
        * Fix TestCaseInsensitiveMoveFileDryRun on chunker integration tests (nielash)
        * Fix TestMkdirModTime test (Nick Craig-Wood)
        * Fix TestSetDirModTime for backends with SetDirModTime but not Metadata (Nick Craig-Wood)
        * Fix typo in log messages (nielash)
    * serve nfs: Fix writing files via Finder on macOS (nielash)
    * serve restic: Fix error handling (Michael Eischer)
    * serve webdav: Fix `--baseurl` without leading / (Nick Craig-Wood)
    * stats: Fix race between ResetCounters and stopAverageLoop called from time.AfterFunc (Nick Craig-Wood)
    * sync
        * `--fix-case` flag to rename case insensitive dest (nielash)
        * Use operations.DirMove instead of sync.MoveDir for `--fix-case` (nielash)
    * systemd: Fix detection and switch to the coreos package everywhere rather than having 2 separate libraries (Anagh Kumar Baranwal)
* Mount
    * Fix macOS not noticing errors with `--daemon` (Nick Craig-Wood)
    * Notice daemon dying much quicker (Nick Craig-Wood)
* VFS
    * Fix unicode normalization on macOS (nielash)
* Bisync
    * Copies and deletes are now handled in one operation instead of two (nielash)
    * `--track-renames` and `--backup-dir` are now supported (nielash)
    * Final listings are now generated from sync results, to avoid needing to re-list (nielash)
    * Bisync is now much more resilient to changes that happen during a bisync run, and far less prone to critical errors / undetected changes (nielash)
    * Bisync is now capable of rolling a file listing back in cases of uncertainty, essentially marking the file as needing to be rechecked next time. (nielash)
    * A few basic terminal colors are now supported, controllable with [`--color`](/docs/#color-when) (`AUTO`|`NEVER`|`ALWAYS`) (nielash)
    * Initial listing snapshots of Path1 and Path2 are now generated concurrently, using the same "march" infrastructure as `check` and `sync`, for performance improvements and less risk of error. (nielash)
    * `--resync` is now much more efficient (especially for users of `--create-empty-src-dirs`) (nielash)
    * Google Docs (and other files of unknown size) are now supported (with the same options as in `sync`) (nielash)
    * Equality checks before a sync conflict rename now fall back to `cryptcheck` (when possible) or `--download`, (nielash)
instead of of `--size-only`, when `check` is not available.
    * Bisync now fully supports comparing based on any combination of size, modtime, and checksum, lifting the prior restriction on backends without modtime support. (nielash)
    * Bisync now supports a "Graceful Shutdown" mode to cleanly cancel a run early without requiring `--resync`. (nielash)
    * New `--recover` flag allows robust recovery in the event of interruptions, without requiring `--resync`. (nielash)
    * A new `--max-lock` setting allows lock files to automatically renew and expire, for better automatic recovery when a run is interrupted. (nielash)
    * Bisync now supports auto-resolving sync conflicts and customizing rename behavior with new [`--conflict-resolve`](#conflict-resolve), [`--conflict-loser`](#conflict-loser), and [`--conflict-suffix`](#conflict-suffix) flags. (nielash)
    * A new [`--resync-mode`](#resync-mode) flag allows more control over which version of a file gets kept during a `--resync`. (nielash)
    * Bisync now supports [`--retries`](/docs/#retries-int) and [`--retries-sleep`](/docs/#retries-sleep-time) (when [`--resilient`](#resilient) is set.) (nielash)
    * Clarify file operation directions in dry-run logs (Kyle Reynolds)
* Local
    * Fix cleanRootPath on Windows after go1.21.4 stdlib update (nielash)
    * Implement setting modification time on directories (nielash)
    * Implement modtime and metadata for directories (Nick Craig-Wood)
    * Fix setting of btime on directories on Windows (Nick Craig-Wood)
    * Delete backend implementation of Purge to speed up and make stats (Nick Craig-Wood)
    * Support metadata setting and mapping on server side Move (Nick Craig-Wood)
* Cache
    * Implement setting modification time on directories (if supported by wrapped remote) (nielash)
    * Implement setting metadata on directories (Nick Craig-Wood)
* Crypt
    * Implement setting modification time on directories (if supported by wrapped remote) (nielash)
    * Implement setting metadata on directories (Nick Craig-Wood)
    * Improve handling of undecryptable file names (nielash)
    * Add missing error check spotted by linter (Nick Craig-Wood)
* Azure Blob
    * Implement `--azureblob-delete-snapshots` (Nick Craig-Wood)
* B2
    * Clarify exactly what `--b2-download-auth-duration` does in the docs (Nick Craig-Wood)
* Chunker
    * Implement setting modification time on directories (if supported by wrapped remote) (nielash)
    * Implement setting metadata on directories (Nick Craig-Wood)
* Combine
    * Implement setting modification time on directories (if supported by wrapped remote) (nielash)
    * Implement setting metadata on directories (Nick Craig-Wood)
    * Fix directory metadata error on upstream root (nielash)
    * Fix directory move across upstreams (nielash)
* Compress
    * Implement setting modification time on directories (if supported by wrapped remote) (nielash)
    * Implement setting metadata on directories (Nick Craig-Wood)
* Drive
    * Implement setting modification time on directories (nielash)
    * Implement modtime and metadata setting for directories (Nick Craig-Wood)
    * Support metadata setting and mapping on server side Move,Copy (Nick Craig-Wood)
* FTP
    * Fix mkdir with rsftp which is returning the wrong code (Nick Craig-Wood)
* Hasher
    * Implement setting modification time on directories (if supported by wrapped remote) (nielash)
    * Implement setting metadata on directories (Nick Craig-Wood)
    * Fix error from trying to stop an already-stopped db (nielash)
    * Look for cached hash if passed hash unexpectedly blank (nielash)
* Imagekit
    * Updated docs and web content (Harshit Budhraja)
    * Updated overview - supported operations (Harshit Budhraja)
* Mega
    * Fix panic with go1.22 (Nick Craig-Wood)
* Netstorage
    * Fix Root to return correct directory when pointing to a file (Nick Craig-Wood)
* Onedrive
    * Add metadata support (nielash)
* Opendrive
    * Fix moving file/folder within the same parent dir (nielash)
* Oracle Object Storage
    * Support `backend restore` command (Nikhil Ahuja)
    * Support workload identity authentication for OKE (Anders Swanson)
* Protondrive
    * Fix encoding of Root method (Nick Craig-Wood)
* Quatrix
    * Fix `Content-Range` header (Volodymyr)
    * Add option to skip project folders (Oksana Zhykina)
    * Fix Root to return correct directory when pointing to a file (Nick Craig-Wood)
* S3
    * Add `--s3-version-deleted` to show delete markers in listings when using versions. (Nick Craig-Wood)
    * Add IPv6 support with option `--s3-use-dual-stack` (Anthony Metzidis)
    * Copy parts in parallel when doing chunked server side copy (Nick Craig-Wood)
    * GCS provider: fix server side copy of files bigger than 5G (Nick Craig-Wood)
    * Support metadata setting and mapping on server side Copy (Nick Craig-Wood)
* Seafile
    * Fix download/upload error when `FILE_SERVER_ROOT` is relative (DanielEgbers)
    * Fix Root to return correct directory when pointing to a file (Nick Craig-Wood)
* SFTP
    * Implement setting modification time on directories (nielash)
    * Set directory modtimes update on write flag (Nick Craig-Wood)
    * Shorten wait delay for external ssh binaries now that we are using go1.20 (Nick Craig-Wood)
* Swift
    * Avoid unnecessary container versioning check (Joe Cai)
* Union
    * Implement setting modification time on directories (if supported by wrapped remote) (nielash)
    * Implement setting metadata on directories (Nick Craig-Wood)
* WebDAV
    * Reduce priority of chunks upload log (Gabriel Ramos)
    * owncloud: Add config `owncloud_exclude_shares` which allows to exclude shared files and folders when listing remote resources (Thomas Müller)

## v1.65.2 - 2024-01-24

[See commits](https://github.com/rclone/rclone/compare/v1.65.1...v1.65.2)

* Bug Fixes
    * build: bump github.com/cloudflare/circl from 1.3.6 to 1.3.7 (dependabot)
    * docs updates (Nick Craig-Wood, kapitainsky, nielash, Tera, Harshit Budhraja)
* VFS
    * Fix stale data when using `--vfs-cache-mode` full (Nick Craig-Wood)
* Azure Blob
    * **IMPORTANT** Fix data corruption bug - see [#7590](https://github.com/rclone/rclone/issues/7590) (Nick Craig-Wood)

## v1.65.1 - 2024-01-08

[See commits](https://github.com/rclone/rclone/compare/v1.65.0...v1.65.1)

* Bug Fixes
    * build
        * Bump golang.org/x/crypto to fix ssh terrapin CVE-2023-48795 (dependabot)
        * Update to go1.21.5 to fix Windows path problems (Nick Craig-Wood)
        * Fix docker build on arm/v6 (Nick Craig-Wood)
    * install.sh: fix harmless error message on install (Nick Craig-Wood)
    * accounting: fix stats to show server side transfers (Nick Craig-Wood)
    * doc fixes (albertony, ben-ba, Eli Orzitzer, emyarod, keongalvin, rarspace01)
    * nfsmount: Compile for all unix oses, add `--sudo` and fix error/option handling (Nick Craig-Wood)
    * operations: Fix files moved by rclone move not being counted as transfers (Nick Craig-Wood)
    * oauthutil: Avoid panic when `*token` and `*ts.token` are the same (rkonfj)
    * serve s3: Fix listing oddities (Nick Craig-Wood)
* VFS
    * Note that `--vfs-refresh` runs in the background (Nick Craig-Wood)
* Azurefiles
    * Fix storage base url (Oksana)
* Crypt
    * Fix rclone move a file over itself deleting the file (Nick Craig-Wood)
* Chunker
    * Fix rclone move a file over itself deleting the file (Nick Craig-Wood)
* Compress
    * Fix rclone move a file over itself deleting the file (Nick Craig-Wood)
* Dropbox
    * Fix used space on dropbox team accounts (Nick Craig-Wood)
* FTP
    * Fix multi-thread copy (WeidiDeng)
* Googlephotos
    * Fix nil pointer exception when batch failed (Nick Craig-Wood)
* Hasher
    * Fix rclone move a file over itself deleting the file (Nick Craig-Wood)
    * Fix invalid memory address error when MaxAge == 0 (nielash)
* Onedrive
    * Fix error listing: unknown object type `<nil>` (Nick Craig-Wood)
    * Fix "unauthenticated: Unauthenticated" errors when uploading (Nick Craig-Wood)
* Oracleobjectstorage
    * Fix object storage endpoint for custom endpoints (Manoj Ghosh)
    * Multipart copy create bucket if it doesn't exist. (Manoj Ghosh)
* Protondrive
    * Fix CVE-2023-45286 / GHSA-xwh9-gc39-5298 (Nick Craig-Wood)
* S3
    * Fix crash if no UploadId in multipart upload (Nick Craig-Wood)
* Smb
    * Fix shares not listed by updating go-smb2 (halms)
* Union
    * Fix rclone move a file over itself deleting the file (Nick Craig-Wood)

## v1.65.0 - 2023-11-26

[See commits](https://github.com/rclone/rclone/compare/v1.64.0...v1.65.0)

* New backends
    * Azure Files (karan, moongdal, Nick Craig-Wood)
    * ImageKit (Abhinav Dhiman)
    * Linkbox (viktor, Nick Craig-Wood)
* New commands
    * `serve s3`: Let rclone act as an S3 compatible server (Mikubill, Artur Neumann, Saw-jan, Nick Craig-Wood)
    * `nfsmount`: mount command to provide mount mechanism on macOS without FUSE (Saleh Dindar)
    * `serve nfs`: to serve a remote for use by `nfsmount` (Saleh Dindar)
* New Features
    * install.sh: Clean up temp files in install script (Jacob Hands)
    * build
        * Update all dependencies (Nick Craig-Wood)
        * Refactor version info and icon resource handling on windows (albertony)
    * doc updates (albertony, alfish2000, asdffdsazqqq, Dimitri Papadopoulos, Herby Gillot, Joda Stößer, Manoj Ghosh, Nick Craig-Wood)
    * Implement `--metadata-mapper` to transform metatadata with a user supplied program (Nick Craig-Wood)
    * Add `ChunkWriterDoesntSeek` feature flag and set it for b2 (Nick Craig-Wood)
    * lib/http: Export basic go string functions for use in `--template` (Gabriel Espinoza)
    * makefile: Use POSIX compatible install arguments (Mina Galić)
    * operations
        * Use less memory when doing multithread uploads (Nick Craig-Wood)
        * Implement `--partial-suffix` to control extension of temporary file names (Volodymyr)
    * rc
        * Add `operations/check` to the rc API (Nick Craig-Wood)
        * Always report an error as JSON (Nick Craig-Wood)
        * Set `Last-Modified` header for files served by `--rc-serve` (Nikita Shoshin)
    * size: Dont show duplicate object count when less than 1k (albertony)
* Bug Fixes
    * fshttp: Fix `--contimeout` being ignored (你知道未来吗)
    * march: Fix excessive parallelism when using `--no-traverse` (Nick Craig-Wood)
    * ncdu: Fix crash when re-entering changed directory after rescan (Nick Craig-Wood)
    * operations
        * Fix overwrite of destination when multi-thread transfer fails (Nick Craig-Wood)
        * Fix invalid UTF-8 when truncating file names when not using `--inplace` (Nick Craig-Wood)
    * serve dnla: Fix crash on graceful exit (wuxingzhong)
* Mount
    * Disable mount for freebsd and alias cmount as mount on that platform (Nick Craig-Wood)
* VFS
    * Add `--vfs-refresh` flag to read all the directories on start (Beyond Meat)
    * Implement Name() method in WriteFileHandle and ReadFileHandle (Saleh Dindar)
    * Add go-billy dependency and make sure vfs.Handle implements billy.File (Saleh Dindar)
    * Error out early if can't upload 0 length file (Nick Craig-Wood)
* Local
    * Fix copying from Windows Volume Shadows (Nick Craig-Wood)
* Azure Blob
    * Add support for cold tier (Ivan Yanitra)
* B2
    * Implement "rclone backend lifecycle" to read and set bucket lifecycles (Nick Craig-Wood)
    * Implement `--b2-lifecycle` to control lifecycle when creating buckets (Nick Craig-Wood)
    * Fix listing all buckets when not needed (Nick Craig-Wood)
    * Fix multi-thread upload with copyto going to wrong name (Nick Craig-Wood)
    * Fix server side chunked copy when file size was exactly `--b2-copy-cutoff` (Nick Craig-Wood)
    * Fix streaming chunked files an exact multiple of chunk size (Nick Craig-Wood)
* Box
    * Filter more EventIDs when polling (David Sze)
    * Add more logging for polling (David Sze)
    * Fix performance problem reading metadata for single files (Nick Craig-Wood)
* Drive
    * Add read/write metadata support (Nick Craig-Wood)
    * Add support for SHA-1 and SHA-256 checksums (rinsuki)
    * Add `--drive-show-all-gdocs` to allow unexportable gdocs to be server side copied (Nick Craig-Wood)
    * Add a note that `--drive-scope` accepts comma-separated list of scopes (Keigo Imai)
    * Fix error updating created time metadata on existing object (Nick Craig-Wood)
    * Fix integration tests by enabling metadata support from the context (Nick Craig-Wood)
* Dropbox
    * Factor batcher into lib/batcher (Nick Craig-Wood)
    * Fix missing encoding for rclone purge (Nick Craig-Wood)
* Google Cloud Storage
    * Fix 400 Bad request errors when using multi-thread copy (Nick Craig-Wood)
* Googlephotos
    * Implement batcher for uploads (Nick Craig-Wood)
* Hdfs
    * Added support for list of namenodes in hdfs remote config (Tayo-pasedaRJ)
* HTTP
    * Implement set backend command to update running backend (Nick Craig-Wood)
    * Enable methods used with WebDAV (Alen Šiljak)
* Jottacloud
    * Add support for reading and writing metadata (albertony)
* Onedrive
    * Implement ListR method which gives `--fast-list` support (Nick Craig-Wood)
        * This must be enabled with the `--onedrive-delta` flag
* Quatrix
    * Add partial upload support (Oksana Zhykina)
    * Overwrite files on conflict during server-side move (Oksana Zhykina)
* S3
    * Add Linode provider (Nick Craig-Wood)
    * Add docs on how to add a new provider (Nick Craig-Wood)
    * Fix no error being returned when creating a bucket we don't own (Nick Craig-Wood)
    * Emit a debug message if anonymous credentials are in use (Nick Craig-Wood)
    * Add `--s3-disable-multipart-uploads` flag (Nick Craig-Wood)
    * Detect looping when using gcs and versions (Nick Craig-Wood)
* SFTP
    * Implement `--sftp-copy-is-hardlink` to server side copy as hardlink (Nick Craig-Wood)
* Smb
    * Fix incorrect `about` size by switching to `github.com/cloudsoda/go-smb2` fork (Nick Craig-Wood)
    * Fix modtime of multithread uploads by setting PartialUploads (Nick Craig-Wood)
* WebDAV
    * Added an rclone vendor to work with `rclone serve webdav` (Adithya Kumar)

## v1.64.2 - 2023-10-19

[See commits](https://github.com/rclone/rclone/compare/v1.64.1...v1.64.2)

* Bug Fixes
    * selfupdate: Fix "invalid hashsum signature" error (Nick Craig-Wood)
    * build: Fix docker build running out of space (Nick Craig-Wood)

## v1.64.1 - 2023-10-17

[See commits](https://github.com/rclone/rclone/compare/v1.64.0...v1.64.1)

* Bug Fixes
    * cmd: Make `--progress` output logs in the same format as without (Nick Craig-Wood)
    * docs fixes (Dimitri Papadopoulos Orfanos, Herby Gillot, Manoj Ghosh, Nick Craig-Wood)
    * lsjson: Make sure we set the global metadata flag too (Nick Craig-Wood)
    * operations
        * Ensure concurrency is no greater than the number of chunks (Pat Patterson)
        * Fix OpenOptions ignored in copy if operation was a multiThreadCopy (Vitor Gomes)
        * Fix error message on delete to have file name (Nick Craig-Wood)
    * serve sftp: Return not supported error for not supported commands (Nick Craig-Wood)
    * build: Upgrade golang.org/x/net to v0.17.0 to fix HTTP/2 rapid reset (Nick Craig-Wood)
    * pacer: Fix b2 deadlock by defaulting max connections to unlimited (Nick Craig-Wood)
* Mount
    * Fix automount not detecting drive is ready (Nick Craig-Wood)
* VFS
    * Fix update dir modification time (Saleh Dindar)
* Azure Blob
    * Fix "fatal error: concurrent map writes" (Nick Craig-Wood)
* B2
    * Fix multipart upload: corrupted on transfer: sizes differ XXX vs 0 (Nick Craig-Wood)
    * Fix locking window when getting mutipart upload URL (Nick Craig-Wood)
    * Fix server side copies greater than 4GB (Nick Craig-Wood)
    * Fix chunked streaming uploads (Nick Craig-Wood)
    * Reduce default `--b2-upload-concurrency` to 4 to reduce memory usage (Nick Craig-Wood)
* Onedrive
    * Fix the configurator to allow `/teams/ID` in the config (Nick Craig-Wood)
* Oracleobjectstorage
    * Fix OpenOptions being ignored in uploadMultipart with chunkWriter (Nick Craig-Wood)
* S3
    * Fix slice bounds out of range error when listing (Nick Craig-Wood)
    * Fix OpenOptions being ignored in uploadMultipart with chunkWriter (Vitor Gomes)
* Storj
    * Update storj.io/uplink to v1.12.0 (Kaloyan Raev)

## v1.64.0 - 2023-09-11

[See commits](https://github.com/rclone/rclone/compare/v1.63.0...v1.64.0)

* New backends
    * [Proton Drive](/protondrive/) (Chun-Hung Tseng)
    * [Quatrix](/quatrix/) (Oksana, Volodymyr Kit)
    * New S3 providers
        * [Synology C2](/s3/#synology-c2) (BakaWang)
        * [Leviia](/s3/#leviia) (Benjamin)
    * New Jottacloud providers
        * [Onlime](/jottacloud/) (Fjodor42)
        * [Telia Sky](/jottacloud/) (NoLooseEnds)
* Major changes
    * Multi-thread transfers (Vitor Gomes, Nick Craig-Wood, Manoj Ghosh, Edwin Mackenzie-Owen)
        * Multi-thread transfers are now available when transferring to:
            * `local`, `s3`, `azureblob`, `b2`, `oracleobjectstorage` and `smb`
        * This greatly improves transfer speed between two network sources.
        * In memory buffering has been unified between all backends and should share memory better.
        * See [--multi-thread docs](/docs/#multi-thread-cutoff) for more info
* New commands
    * `rclone config redacted` support mechanism for showing redacted config (Nick Craig-Wood)
* New Features
    * accounting
        * Show server side stats in own lines and not as bytes transferred (Nick Craig-Wood)
    * bisync
        * Add new `--ignore-listing-checksum` flag to distinguish from `--ignore-checksum` (nielash)
        * Add experimental `--resilient` mode to allow recovery from self-correctable errors (nielash)
        * Add support for `--create-empty-src-dirs` (nielash)
        * Dry runs no longer commit filter changes (nielash)
        * Enforce `--check-access` during `--resync` (nielash)
        * Apply filters correctly during deletes (nielash)
        * Equality check before renaming (leave identical files alone) (nielash)
        * Fix `dryRun` rc parameter being ignored (nielash)
    * build
        * Update to `go1.21` and make `go1.19` the minimum required version (Anagh Kumar Baranwal, Nick Craig-Wood)
        * Update dependencies (Nick Craig-Wood)
        * Add snap installation (hideo aoyama)
        * Change Winget Releaser job to `ubuntu-latest` (sitiom)
    * cmd: Refactor and use sysdnotify in more commands (eNV25)
    * config: Add `--multi-thread-chunk-size` flag (Vitor Gomes)
    * doc updates (antoinetran, Benjamin, Bjørn Smith, Dean Attali, gabriel-suela, James Braza, Justin Hellings, kapitainsky, Mahad, Masamune3210, Nick Craig-Wood, Nihaal Sangha, Niklas Hambüchen, Raymond Berger, r-ricci, Sawada Tsunayoshi, Tiago Boeing, Vladislav Vorobev)
    * fs
        * Use atomic types everywhere (Roberto Ricci)
        * When `--max-transfer` limit is reached exit with code (10) (kapitainsky)
        * Add rclone completion powershell - basic implementation only (Nick Craig-Wood)
    * http servers: Allow CORS to be set with `--allow-origin` flag (yuudi)
    * lib/rest: Remove unnecessary `nil` check (Eng Zer Jun)
    * ncdu: Add keybinding to rescan filesystem (eNV25)
    * rc
        * Add `executeId` to job listings (yuudi)
        * Add `core/du` to measure local disk usage (Nick Craig-Wood)
        * Add `operations/settier` to API (Drew Stinnett)
    * rclone test info: Add `--check-base32768` flag to check can store all base32768 characters (Nick Craig-Wood)
    * rmdirs: Remove directories concurrently controlled by `--checkers` (Nick Craig-Wood)
* Bug Fixes
    * accounting: Don't stop calculating average transfer speed until the operation is complete (Jacob Hands)
    * fs: Fix `transferTime` not being set in JSON logs (Jacob Hands)
    * fshttp: Fix `--bind 0.0.0.0` allowing IPv6 and `--bind ::0` allowing IPv4 (Nick Craig-Wood)
    * operations: Fix overlapping check on case insensitive file systems (Nick Craig-Wood)
    * serve dlna: Fix MIME type if backend can't identify it (Nick Craig-Wood)
    * serve ftp: Fix race condition when using the auth proxy (Nick Craig-Wood)
    * serve sftp: Fix hash calculations with `--vfs-cache-mode full` (Nick Craig-Wood)
    * serve webdav: Fix error: Expecting fs.Object or fs.Directory, got `nil` (Nick Craig-Wood)
    * sync: Fix lockup with `--cutoff-mode=soft` and `--max-duration` (Nick Craig-Wood)
* Mount
    * fix: Mount parsing for linux (Anagh Kumar Baranwal)
* VFS
    * Add `--vfs-cache-min-free-space` to control minimum free space on the disk containing the cache (Nick Craig-Wood)
    * Added cache cleaner for directories to reduce memory usage (Anagh Kumar Baranwal)
    * Update parent directory modtimes on vfs actions (David Pedersen)
    * Keep virtual directory status accurate and reduce deadlock potential (Anagh Kumar Baranwal)
    * Make sure struct field is aligned for atomic access (Roberto Ricci)
* Local
    * Rmdir return an error if the path is not a dir (zjx20)
* Azure Blob
    * Implement `OpenChunkWriter` and multi-thread uploads (Nick Craig-Wood)
    * Fix creation of directory markers (Nick Craig-Wood)
    * Fix purging with directory markers (Nick Craig-Wood)
* B2
    * Implement `OpenChunkWriter` and multi-thread uploads (Nick Craig-Wood)
    * Fix rclone link when object path contains special characters (Alishan Ladhani)
* Box
    * Add polling support (David Sze)
    * Add `--box-impersonate` to impersonate a user ID (Nick Craig-Wood)
    * Fix unhelpful decoding of error messages into decimal numbers (Nick Craig-Wood)
* Chunker
    * Update documentation to mention issue with small files (Ricardo D'O. Albanus)
* Compress
    * Fix ChangeNotify (Nick Craig-Wood)
* Drive
    * Add `--drive-fast-list-bug-fix` to control ListR bug workaround (Nick Craig-Wood)
* Fichier
    * Implement `DirMove` (Nick Craig-Wood)
    * Fix error code parsing (alexia)
* FTP
    * Add socks_proxy support for SOCKS5 proxies (Zach)
    * Fix 425 "TLS session of data connection not resumed" errors (Nick Craig-Wood)
* Hdfs
    * Retry "replication in progress" errors when uploading (Nick Craig-Wood)
    * Fix uploading to the wrong object on Update with overridden remote name (Nick Craig-Wood)
* HTTP
    * CORS should not be sent if not set (yuudi)
    * Fix webdav OPTIONS response (yuudi)
* Opendrive
    * Fix List on a just deleted and remade directory (Nick Craig-Wood)
* Oracleobjectstorage
    * Use rclone's rate limiter in multipart transfers (Manoj Ghosh)
    * Implement `OpenChunkWriter` and multi-thread uploads (Manoj Ghosh)
* S3
    * Refactor multipart upload to use `OpenChunkWriter` and `ChunkWriter` (Vitor Gomes)
    * Factor generic multipart upload into `lib/multipart` (Nick Craig-Wood)
    * Fix purging of root directory with `--s3-directory-markers` (Nick Craig-Wood)
    * Add `rclone backend set` command to update the running config (Nick Craig-Wood)
    * Add `rclone backend restore-status` command (Nick Craig-Wood)
* SFTP
    * Stop uploads re-using the same ssh connection to improve performance (Nick Craig-Wood)
    * Add `--sftp-ssh` to specify an external ssh binary to use (Nick Craig-Wood)
    * Add socks_proxy support for SOCKS5 proxies (Zach)
    * Support dynamic `--sftp-path-override` (nielash)
    * Fix spurious warning when using `--sftp-ssh` (Nick Craig-Wood)
* Smb
    * Implement multi-threaded writes for copies to smb (Edwin Mackenzie-Owen)
* Storj
    * Performance improvement for large file uploads (Kaloyan Raev)
* Swift
    * Fix HEADing 0-length objects when `--swift-no-large-objects` set (Julian Lepinski)
* Union
    * Add `:writback` to act as a simple cache (Nick Craig-Wood)
* WebDAV
    * Nextcloud: fix segment violation in low-level retry (Paul)
* Zoho
    * Remove Range requests workarounds to fix integration tests (Nick Craig-Wood)

## v1.63.1 - 2023-07-17

[See commits](https://github.com/rclone/rclone/compare/v1.63.0...v1.63.1)

* Bug Fixes
    * build: Fix macos builds for versions < 12 (Anagh Kumar Baranwal)
    * dirtree: Fix performance with large directories of directories and `--fast-list` (Nick Craig-Wood)
    * operations
        * Fix deadlock when using `lsd`/`ls` with `--progress` (Nick Craig-Wood)
        * Fix `.rclonelink` files not being converted back to symlinks (Nick Craig-Wood)
    * doc fixes (Dean Attali, Mahad, Nick Craig-Wood, Sawada Tsunayoshi, Vladislav Vorobev)
* Local
    * Fix partial directory read for corrupted filesystem (Nick Craig-Wood)
* Box
    * Fix reconnect failing with HTTP 400 Bad Request (albertony)
* Smb
    * Fix "Statfs failed: bucket or container name is needed" when mounting (Nick Craig-Wood)
* WebDAV
    * Nextcloud: fix must use /dav/files/USER endpoint not /webdav error (Paul)
    * Nextcloud chunking: add more guidance for the user to check the config (darix)

## v1.63.0 - 2023-06-30

[See commits](https://github.com/rclone/rclone/compare/v1.62.0...v1.63.0)

* New backends
    * [Pikpak](/pikpak/) (wiserain)
    * New S3 providers
        * [petabox.io](/s3/#petabox) (Andrei Smirnov)
        * [Google Cloud Storage](/s3/#google-cloud-storage) (Anthony Pessy)
    * New WebDAV providers
        * [Fastmail](/webdav/#fastmail-files) (Arnavion)
* Major changes
    * Files will be copied to a temporary name ending in `.partial` when copying to `local`,`ftp`,`sftp` then renamed at the end of the transfer. (Janne Hellsten, Nick Craig-Wood)
        * This helps with data integrity as we don't delete the existing file until the new one is complete.
        * It can be disabled with the [--inplace](/docs/#inplace) flag.
        * This behaviour will also happen if the backend is wrapped, for example `sftp` wrapped with `crypt`.
    * The [s3](/s3/#s3-directory-markers), [azureblob](/azureblob/#azureblob-directory-markers) and [gcs](/googlecloudstorage/#gcs-directory-markers) backends now support directory markers so empty directories are supported (Jānis Bebrītis, Nick Craig-Wood)
    * The [--default-time](/docs/#default-time-time) flag now controls the unknown modification time of files/dirs (Nick Craig-Wood)
        * If a file or directory does not have a modification time rclone can read then rclone will display this fixed time instead.
        * For the old behaviour use `--default-time 0s` which will set this time to the time rclone started up.
* New Features
    * build
        * Modernise linters in use and fixup all affected code (albertony)
        * Push docker beta to GHCR (GitHub container registry) (Richard Tweed)
    * cat: Add `--separator` option to cat command (Loren Gordon)
    * config
        * Do not remove/overwrite other files during config file save (albertony)
        * Do not overwrite config file symbolic link (albertony)
        * Stop `config create` making invalid config files (Nick Craig-Wood)
    * doc updates (Adam K, Aditya Basu, albertony, asdffdsazqqq, Damo, danielkrajnik, Dimitri Papadopoulos, dlitster, Drew Parsons, jumbi77, kapitainsky, mac-15, Mariusz Suchodolski, Nick Craig-Wood, NickIAm, Rintze Zelle, Stanislav Gromov, Tareq Sharafy, URenko, yuudi, Zach Kipp)
    * fs
        * Add `size` to JSON logs when moving or copying an object (Nick Craig-Wood)
        * Allow boolean features to be enabled with `--disable !Feature` (Nick Craig-Wood)
    * genautocomplete: Rename to `completion` with alias to the old name (Nick Craig-Wood)
    * librclone: Added example on using `librclone` with Go (alankrit)
    * lsjson: Make `--stat` more efficient (Nick Craig-Wood)
    * operations
        * Implement `--multi-thread-write-buffer-size` for speed improvements on downloads (Paulo Schreiner)
        * Reopen downloads on error when using `check --download` and `cat` (Nick Craig-Wood)
    * rc: `config/listremotes` includes remotes defined with environment variables (kapitainsky)
    * selfupdate: Obey `--no-check-certificate` flag (Nick Craig-Wood)
    * serve restic: Trigger systemd notify (Shyim)
    * serve webdav: Implement owncloud checksum and modtime extensions (WeidiDeng)
    * sync: `--suffix-keep-extension` preserve 2 part extensions like .tar.gz (Nick Craig-Wood)
* Bug Fixes
    * accounting
        * Fix Prometheus metrics to be the same as `core/stats` (Nick Craig-Wood)
        * Bwlimit signal handler should always start (Sam Lai)
    * bisync: Fix `maxDelete` parameter being ignored via the rc (Nick Craig-Wood)
    * cmd/ncdu: Fix screen corruption when logging (eNV25)
    * filter: Fix deadlock with errors on `--files-from` (douchen)
    * fs
        * Fix interaction between `--progress` and `--interactive` (Nick Craig-Wood)
        * Fix infinite recursive call in pacer ModifyCalculator (fixes issue reported by the staticcheck linter) (albertony)
    * lib/atexit: Ensure OnError only calls cancel function once (Nick Craig-Wood)
    * lib/rest: Fix problems re-using HTTP connections (Nick Craig-Wood)
    * rc
        * Fix `operations/stat` with trailing `/` (Nick Craig-Wood)
        * Fix missing `--rc` flags (Nick Craig-Wood)
        * Fix output of Time values in `options/get` (Nick Craig-Wood)
    * serve dlna: Fix potential data race (Nick Craig-Wood)
    * version: Fix reported os/kernel version for windows (albertony)
* Mount
    * Add `--mount-case-insensitive` to force the mount to be case insensitive (Nick Craig-Wood)
    * Removed unnecessary byte slice allocation for reads (Anagh Kumar Baranwal)
    * Clarify rclone mount error when installed via homebrew (Nick Craig-Wood)
    * Added _netdev to the example mount so it gets treated as a remote-fs rather than local-fs (Anagh Kumar Baranwal)
* Mount2
    * Updated go-fuse version (Anagh Kumar Baranwal)
    * Fixed statfs (Anagh Kumar Baranwal)
    * Disable xattrs (Anagh Kumar Baranwal)
* VFS
    * Add MkdirAll function to make a directory and all beneath (Nick Craig-Wood)
    * Fix reload: failed to add virtual dir entry: file does not exist (Nick Craig-Wood)
    * Fix writing to a read only directory creating spurious directory entries (WeidiDeng)
    * Fix potential data race (Nick Craig-Wood)
    * Fix backends being Shutdown too early when startup takes a long time (Nick Craig-Wood)
* Local
    * Fix filtering of symlinks with `-l`/`--links` flag (Nick Craig-Wood)
    * Fix /path/to/file.rclonelink when `-l`/`--links` is in use (Nick Craig-Wood)
    * Fix crash with `--metadata` on Android (Nick Craig-Wood)
* Cache
    * Fix backends shutting down when in use when used via the rc (Nick Craig-Wood)
* Crypt
    * Add `--crypt-suffix` option to set a custom suffix for encrypted files (jladbrook)
    * Add `--crypt-pass-bad-blocks` to allow corrupted file output (Nick Craig-Wood)
    * Fix reading 0 length files (Nick Craig-Wood)
    * Try not to return "unexpected EOF" error (Nick Craig-Wood)
    * Reduce allocations (albertony)
    * Recommend Dropbox for `base32768` encoding (Nick Craig-Wood)
* Azure Blob
    * Empty directory markers (Nick Craig-Wood)
    * Support azure workload identities (Tareq Sharafy)
    * Fix azure blob uploads with multiple bits of metadata (Nick Craig-Wood)
    * Fix azurite compatibility by sending nil tier if set to empty string (Roel Arents)
* Combine
    * Implement missing methods (Nick Craig-Wood)
    * Fix goroutine stack overflow on bad object (Nick Craig-Wood)
* Drive
    * Add `--drive-env-auth` to get IAM credentials from runtime (Peter Brunner)
    * Update drive service account guide (Juang, Yi-Lin)
    * Fix change notify picking up files outside the root (Nick Craig-Wood)
    * Fix trailing slash mis-identificaton of folder as file (Nick Craig-Wood)
    * Fix incorrect remote after Update on object (Nick Craig-Wood)
* Dropbox
    * Implement `--dropbox-pacer-min-sleep` flag (Nick Craig-Wood)
    * Fix the dropbox batcher stalling (Misty)
* Fichier
    * Add `--ficicher-cdn` option to use the CDN for download (Nick Craig-Wood)
* FTP
    * Lower log message priority when `SetModTime` is not supported to debug (Tobias Gion)
    * Fix "unsupported LIST line" errors on startup (Nick Craig-Wood)
    * Fix "501 Not a valid pathname." errors when creating directories (Nick Craig-Wood)
* Google Cloud Storage
    * Empty directory markers (Jānis Bebrītis, Nick Craig-Wood)
    * Added `--gcs-user-project` needed for requester pays (Christopher Merry)
* HTTP
    * Add client certificate user auth middleware. This can auth `serve restic` from the username in the client cert. (Peter Fern)
* Jottacloud
    * Fix vfs writeback stuck in a failed upload loop with file versioning disabled (albertony)
* Onedrive
    * Add `--onedrive-av-override` flag to download files flagged as virus (Nick Craig-Wood)
    * Fix quickxorhash on 32 bit architectures (Nick Craig-Wood)
    * Report any list errors during `rclone cleanup` (albertony)
* Putio
    * Fix uploading to the wrong object on Update with overridden remote name (Nick Craig-Wood)
    * Fix modification times not being preserved for server side copy and move (Nick Craig-Wood)
    * Fix server side copy failures (400 errors) (Nick Craig-Wood)
* S3
    * Empty directory markers (Jānis Bebrītis, Nick Craig-Wood)
    * Update Scaleway storage classes (Brian Starkey)
    * Fix `--s3-versions` on individual objects (Nick Craig-Wood)
    * Fix hang on aborting multipart upload with iDrive e2 (Nick Craig-Wood)
    * Fix missing "tier" metadata (Nick Craig-Wood)
    * Fix V3sign: add missing subresource delete (cc)
    * Fix Arvancloud Domain and region changes and alphabetise the provider (Ehsan Tadayon)
    * Fix Qiniu KODO quirks virtualHostStyle is false (zzq)
* SFTP
    * Add `--sftp-host-key-algorithms ` to allow specifying SSH host key algorithms (Joel)
    * Fix using `--sftp-key-use-agent` and `--sftp-key-file` together needing private key file (Arnav Singh)
    * Fix move to allow overwriting existing files (Nick Craig-Wood)
    * Don't stat directories before listing them (Nick Craig-Wood)
    * Don't check remote points to a file if it ends with / (Nick Craig-Wood)
* Sharefile
    * Disable streamed transfers as they no longer work (Nick Craig-Wood)
* Smb
    * Code cleanup to avoid overwriting ctx before first use (fixes issue reported by the staticcheck linter) (albertony)
* Storj
    * Fix "uplink: too many requests" errors when uploading to the same file (Nick Craig-Wood)
    * Fix uploading to the wrong object on Update with overridden remote name (Nick Craig-Wood)
* Swift
    * Ignore 404 error when deleting an object (Nick Craig-Wood)
* Union
    * Implement missing methods (Nick Craig-Wood)
    * Allow errors to be unwrapped for inspection (Nick Craig-Wood)
* Uptobox
    * Add `--uptobox-private` flag to make all uploaded files private (Nick Craig-Wood)
    * Fix improper regex (Aaron Gokaslan)
    * Fix Update returning the wrong object (Nick Craig-Wood)
    * Fix rmdir declaring that directories weren't empty (Nick Craig-Wood)
* WebDAV
    * nextcloud: Add support for chunked uploads (Paul)
    * Set modtime using propset for owncloud and nextcloud (WeidiDeng)
    * Make pacer minSleep configurable with `--webdav-pacer-min-sleep` (ed)
    * Fix server side copy/move not overwriting (WeidiDeng)
    * Fix modtime on server side copy for owncloud and nextcloud (Nick Craig-Wood)
* Yandex
    * Fix 400 Bad Request on transfer failure (Nick Craig-Wood)
* Zoho
    * Fix downloads with `Range:` header returning the wrong data (Nick Craig-Wood)

## v1.62.2 - 2023-03-16

[See commits](https://github.com/rclone/rclone/compare/v1.62.1...v1.62.2)

* Bug Fixes
    * docker volume plugin: Add missing fuse3 dependency (Nick Craig-Wood)
    * docs: Fix size documentation (asdffdsazqqq)
* FTP
    * Fix 426 errors on downloads with vsftpd (Lesmiscore)

## v1.62.1 - 2023-03-15

[See commits](https://github.com/rclone/rclone/compare/v1.62.0...v1.62.1)

* Bug Fixes
    * docker: Add missing fuse3 dependency (cycneuramus)
    * build: Update release docs to be more careful with the tag (Nick Craig-Wood)
    * build: Set Github release to draft while uploading binaries (Nick Craig-Wood)

## v1.62.0 - 2023-03-14

[See commits](https://github.com/rclone/rclone/compare/v1.61.0...v1.62.0)

* New Features
    * accounting: Make checkers show what they are doing (Nick Craig-Wood)
    * authorize: Add support for custom templates (Hunter Wittenborn)
    * build
        * Update to go1.20 (Nick Craig-Wood, Anagh Kumar Baranwal)
        * Add winget releaser workflow (Ryan Caezar Itang)
        * Add dependabot (Ryan Caezar Itang)
    * doc updates (albertony, Bryan Kaplan, Gerard Bosch, IMTheNachoMan, Justin Winokur, Manoj Ghosh, Nick Craig-Wood, Ole Frost, Peter Brunner, piyushgarg, Ryan Caezar Itang, Simmon Li, ToBeFree)
    * filter: Emit INFO message when can't work out directory filters (Nick Craig-Wood)
    * fs
        * Added multiple ca certificate support. (alankrit)
        * Add `--max-delete-size` a delete size threshold (Leandro Sacchet)
    * fspath: Allow the symbols `@` and `+` in remote names (albertony)
    * lib/terminal: Enable windows console virtual terminal sequences processing (ANSI/VT100 colors) (albertony)
    * move: If `--check-first` and `--order-by` are set then delete with perfect ordering (Nick Craig-Wood)
    * serve http: Support `--auth-proxy` (Matthias Baur)
* Bug Fixes
    * accounting
        * Avoid negative ETA values for very slow speeds (albertony)
        * Limit length of ETA string (albertony)
        * Show human readable elapsed time when longer than a day (albertony)
    * all: Apply codeql fixes (Aaron Gokaslan)
    * build
        * Fix condition for manual workflow run (albertony)
        * Fix building for ARMv5 and ARMv6 (albertony)
            * selfupdate: Consider ARM version
            * install.sh: fix ARMv6 download
            * version: Report ARM version
    * deletefile: Return error code 4 if file does not exist (Nick Craig-Wood)
    * docker: Fix volume plugin does not remount volume on docker restart (logopk)
    * fs: Fix race conditions in `--max-delete` and `--max-delete-size` (Nick Craig-Wood)
    * lib/oauthutil: Handle fatal errors better (Alex Chen)
    * mount2: Fix `--allow-non-empty` (Nick Craig-Wood)
    * operations: Fix concurrency: use `--checkers` unless transferring files (Nick Craig-Wood)
    * serve ftp: Fix timestamps older than 1 year in listings (Nick Craig-Wood)
    * sync: Fix concurrency: use `--checkers` unless transferring files (Nick Craig-Wood)
    * tree
        * Fix nil pointer exception on stat failure (Nick Craig-Wood)
        * Fix colored output on windows (albertony)
        * Fix display of files with illegal Windows file system names (Nick Craig-Wood)
* Mount
    * Fix creating and renaming files on case insensitive backends (Nick Craig-Wood)
    * Do not treat `\\?\` prefixed paths as network share paths on windows (albertony)
    * Fix check for empty mount point on Linux (Nick Craig-Wood)
    * Fix `--allow-non-empty` (Nick Craig-Wood)
    * Avoid incorrect or premature overlap check on windows (albertony)
    * Update to fuse3 after bazil.org/fuse update (Nick Craig-Wood)
* VFS
    * Make uploaded files retain modtime with non-modtime backends (Nick Craig-Wood)
    * Fix incorrect modtime on fs which don't support setting modtime (Nick Craig-Wood)
    * Fix rename of directory containing files to be uploaded (Nick Craig-Wood)
* Local
    * Fix `%!w(<nil>)` in "failed to read directory" error (Marks Polakovs)
    * Fix exclusion of dangling symlinks with -L/--copy-links (Nick Craig-Wood)
* Crypt
    * Obey `--ignore-checksum` (Nick Craig-Wood)
    * Fix for unencrypted directory names on case insensitive remotes (Ole Frost)
* Azure Blob
    * Remove workarounds for SDK bugs after v0.6.1 update (Nick Craig-Wood)
* B2
    * Fix uploading files bigger than 1TiB (Nick Craig-Wood)
* Drive
    * Note that `--drive-acknowledge-abuse` needs SA Manager permission (Nick Craig-Wood)
    * Make `--drive-stop-on-upload-limit` to respond to storageQuotaExceeded (Ninh Pham)
* FTP
    * Retry 426 errors (Nick Craig-Wood)
    * Retry errors when initiating downloads (Nick Craig-Wood)
    * Revert to upstream `github.com/jlaffaye/ftp` now fix is merged (Nick Craig-Wood)
* Google Cloud Storage
    * Add `--gcs-env-auth` to pick up IAM credentials from env/instance (Peter Brunner)
* Mega
    * Add `--mega-use-https` flag (NodudeWasTaken)
* Onedrive
    * Default onedrive personal to QuickXorHash as Microsoft is removing SHA1 (Nick Craig-Wood)
    * Add `--onedrive-hash-type` to change the hash in use (Nick Craig-Wood)
    * Improve speed of QuickXorHash (LXY)
* Oracle Object Storage
    * Speed up operations by using S3 pacer and setting minsleep to 10ms (Manoj Ghosh)
    * Expose the `storage_tier` option in config (Manoj Ghosh)
    * Bring your own encryption keys (Manoj Ghosh)
* S3
    * Check multipart upload ETag when `--s3-no-head` is in use (Nick Craig-Wood)
    * Add `--s3-sts-endpoint` to specify STS endpoint (Nick Craig-Wood)
    * Fix incorrect tier support for StorJ and IDrive when pointing at a file (Ole Frost)
    * Fix AWS STS failing if `--s3-endpoint` is set (Nick Craig-Wood)
    * Make purge remove directory markers too (Nick Craig-Wood)
* Seafile
    * Renew library password (Fred)
* SFTP
    * Fix uploads being 65% slower than they should be with crypt (Nick Craig-Wood)
* Smb
    * Allow SPN (service principal name) to be configured (Nick Craig-Wood)
    * Check smb connection is closed (happyxhw)
* Storj
    * Implement `rclone link` (Kaloyan Raev)
    * Implement `rclone purge` (Kaloyan Raev)
    * Update satellite urls and labels (Kaloyan Raev)
* WebDAV
    * Fix interop with davrods server (Nick Craig-Wood)

## v1.61.1 - 2022-12-23

[See commits](https://github.com/rclone/rclone/compare/v1.61.0...v1.61.1)

* Bug Fixes
    * docs:
        * Show only significant parts of version number in version introduced label (albertony)
        * Fix unescaped HTML (Nick Craig-Wood)
    * lib/http: Shutdown all servers on exit to remove unix socket (Nick Craig-Wood)
    * rc: Fix `--rc-addr` flag (which is an alternate for `--url`) (Anagh Kumar Baranwal)
    * serve restic
        * Don't serve via http if serving via `--stdio` (Nick Craig-Wood)
        * Fix immediate exit when not using stdio (Nick Craig-Wood)
    * serve webdav
        * Fix `--baseurl` handling after `lib/http` refactor (Nick Craig-Wood)
        * Fix running duplicate Serve call (Nick Craig-Wood)
* Azure Blob
    * Fix "409 Public access is not permitted on this storage account" (Nick Craig-Wood)
* S3
    * storj: Update endpoints (Kaloyan Raev)

## v1.61.0 - 2022-12-20

[See commits](https://github.com/rclone/rclone/compare/v1.60.0...v1.61.0)

* New backends
    * New S3 providers
        * [Liara LOS](/s3/#liara-cloud) (MohammadReza)
* New Features
    * build: Add vulnerability testing using govulncheck (albertony)
    * cmd: Enable `SIGINFO` (Ctrl-T) handler on FreeBSD, NetBSD, OpenBSD and Dragonfly BSD (x3-apptech)
    * config: Add [config/setpath](/rc/#config-setpath) for setting config path via rc/librclone (Nick Craig-Wood)
    * dedupe
        * Count Checks in the stats while scanning for duplicates (Nick Craig-Wood)
        * Make dedupe obey the filters (Nick Craig-Wood)
    * dlna: Properly attribute code used from https://github.com/anacrolix/dms (Nick Craig-Wood)
    * docs
        * Add minimum versions and status badges to backend and command docs (Nick Craig-Wood, albertony)
        * Remote names may not start or end with space (albertony)
    * filter: Add metadata filters [--metadata-include/exclude/filter](/filtering/#metadata) and friends (Nick Craig-Wood)
    * fs
        * Make all duration flags take `y`, `M`, `w`, `d` etc suffixes (Nick Craig-Wood)
        * Add global flag `--color` to control terminal colors (Kevin Verstaen)
    * fspath: Allow unicode numbers and letters in remote names (albertony)
    * lib/file: Improve error message for creating dir on non-existent network host on windows (albertony)
    * lib/http: Finish port of rclone servers to `lib/http` (Tom Mombourquette, Nick Craig-Wood)
    * lib/oauthutil: Improved usability of config flows needing web browser (Ole Frost)
    * ncdu
        * Add support for modification time (albertony)
        * Fallback to sort by name also for sort by average size (albertony)
        * Rework to use tcell directly instead of the termbox wrapper (eNV25)
    * rc: Add commands to set [GC Percent](/rc/#debug-set-gc-percent) & [Memory Limit](/rc/#debug-set-soft-memory-limit) (go 1.19+) (Anagh Kumar Baranwal)
    * rcat: Preserve metadata when Copy falls back to Rcat (Nick Craig-Wood)
    * rcd: Refactor rclone rc server to use `lib/http` (Nick Craig-Wood)
    * rcserver: Avoid generating default credentials with htpasswd (Kamui)
    * restic: Refactor to use `lib/http` (Nolan Woods)
    * serve http: Support unix sockets and multiple listeners (Tom Mombourquette)
    * serve webdav: Refactor to use `lib/http` (Nick Craig-Wood)
    * test: Replace defer cleanup with `t.Cleanup` (Eng Zer Jun)
    * test memory: Read metadata if `-M` flag is specified (Nick Craig-Wood)
    * wasm: Comply with `wasm_exec.js` licence terms (Matthew Vernon)
* Bug Fixes
    * build: Update `golang.org/x/net/http2` to fix GO-2022-1144 (Nick Craig-Wood)
    * restic: Fix typo in docs 'remove' should be 'remote' (asdffdsazqqq)
    * serve dlna: Fix panic: Logger uninitialized. (Nick Craig-Wood)
* Mount
    * Update cgofuse for FUSE-T support for mounting volumes on Mac (Nick Craig-Wood)
* VFS
    * Windows: fix slow opening of exe files by not truncating files when not necessary (Nick Craig-Wood)
    * Fix IO Error opening a file with `O_CREATE|O_RDONLY` in `--vfs-cache-mode` not full (Nick Craig-Wood)
* Crypt
    * Fix compress wrapping crypt giving upload errors (Nick Craig-Wood)
* Azure Blob
    * Port to new SDK (Nick Craig-Wood)
        * Revamp authentication to include all methods and docs (Nick Craig-Wood)
        * Port old authentication methods to new SDK (Nick Craig-Wood, Brad Ackerman)
        * Thanks to [Stonebranch](https://www.stonebranch.com/) for sponsoring this work.
    * Add `--azureblob-no-check-container` to assume container exists (Nick Craig-Wood)
    * Add `--use-server-modtime` support (Abdullah Saglam)
    * Add support for custom upload headers (rkettelerij)
    * Allow emulator account/key override (Roel Arents)
    * Support simple "environment credentials" (Nathaniel Wesley Filardo)
    * Ignore `AuthorizationFailure` when trying to create a create a container (Nick Craig-Wood)
* Box
    * Added note on Box API rate limits (Ole Frost)
* Drive
    * Handle shared drives with leading/trailing space in name (related to) (albertony)
* FTP
    * Update help text of implicit/explicit TLS options to refer to FTPS instead of FTP (ycdtosa)
    * Improve performance to speed up `--files-from` and `NewObject` (Anthony Pessy)
* HTTP
    * Parse GET responses when `no_head` is set (Arnie97)
    * Do not update object size based on `Range` requests (Arnie97)
    * Support `Content-Range` response header (Arnie97)
* Onedrive
    * Document workaround for shared with me files (vanplus)
* S3
    * Add Liara LOS to provider list (MohammadReza)
    * Add DigitalOcean Spaces regions `sfo3`, `fra1`, `syd1` (Jack)
    * Avoid privileged `GetBucketLocation` to resolve s3 region (Anthony Pessy)
    * Stop setting object and bucket ACL to `private` if it is an empty string (Philip Harvey)
    * If bucket or object ACL is empty string then don't add `X-Amz-Acl:` header (Nick Craig-Wood)
    * Reduce memory consumption for s3 objects (Erik Agterdenbos)
    * Fix listing loop when using v2 listing on v1 server (Nick Craig-Wood)
    * Fix nil pointer exception when using Versions (Nick Craig-Wood)
    * Fix excess memory usage when using versions (Nick Craig-Wood)
    * Ignore versionIDs from uploads unless using `--s3-versions` or `--s3-versions-at` (Nick Craig-Wood)
* SFTP
    * Add configuration options to set ssh Ciphers / MACs / KeyExchange (dgouju)
    * Auto-detect shell type for fish (albertony)
    * Fix NewObject with leading / (Nick Craig-Wood)
* Smb
    * Fix issue where spurious dot directory is created (albertony)
* Storj
    * Implement server side Copy (Kaloyan Raev)

## v1.60.1 - 2022-11-17

[See commits](https://github.com/rclone/rclone/compare/v1.60.0...v1.60.1)

* Bug Fixes
    * lib/cache: Fix alias backend shutting down too soon (Nick Craig-Wood)
    * wasm: Fix walltime link error by adding up-to-date wasm_exec.js (João Henrique Franco)
    * docs
        * Update faq.md with bisync (Samuel Johnson)
        * Corrected download links in windows install docs (coultonluke)
        * Add direct download link for windows arm64 (albertony)
        * Remove link to rclone slack as it is no longer supported (Nick Craig-Wood)
        * Faq: how to use a proxy server that requires a username and password (asdffdsazqqq)
        * Oracle-object-storage: doc fix (Manoj Ghosh)
        * Fix typo `remove` in rclone_serve_restic command (Joda Stößer)
        * Fix character that was incorrectly interpreted as markdown (Clément Notin)
* VFS
    * Fix deadlock caused by cache cleaner and upload finishing (Nick Craig-Wood)
* Local
    * Clean absolute paths (albertony)
    * Fix -L/--copy-links with filters missing directories (Nick Craig-Wood)
* Mailru
    * Note that an app password is now needed (Nick Craig-Wood)
    * Allow timestamps to be before the epoch 1970-01-01 (Nick Craig-Wood)
* S3
    * Add provider quirk `--s3-might-gzip` to fix corrupted on transfer: sizes differ (Nick Craig-Wood)
    * Allow Storj to server side copy since it seems to work now (Nick Craig-Wood)
    * Fix for unchecked err value in s3 listv2 (Aaron Gokaslan)
    * Add additional Wasabi locations (techknowlogick)
* Smb
    * Fix `Failed to sync: context canceled` at the end of syncs (Nick Craig-Wood)
* WebDAV
    * Fix Move/Copy/DirMove when using -server-side-across-configs (Nick Craig-Wood)

## v1.60.0 - 2022-10-21

[See commits](https://github.com/rclone/rclone/compare/v1.59.0...v1.60.0)

* New backends
    * [Oracle object storage](/oracleobjectstorage/) (Manoj Ghosh)
    * [SMB](/smb/) / CIFS (Windows file sharing) (Lesmiscore)
    * New S3 providers
        * [IONOS Cloud Storage](/s3/#ionos) (Dmitry Deniskin)
        * [Qiniu KODO](/s3/#qiniu) (Bachue Zhou)
* New Features
    * build
        * Update to go1.19 and make go1.17 the minimum required version (Nick Craig-Wood)
        * Install.sh: fix arm-v7 download (Ole Frost)
    * fs: Warn the user when using an existing remote name without a colon (Nick Craig-Wood)
    * httplib: Add `--xxx-min-tls-version` option to select minimum TLS version for HTTP servers (Robert Newson)
    * librclone: Add PHP bindings and test program (Jordi Gonzalez Muñoz)
    * operations
        * Add `--server-side-across-configs` global flag for any backend (Nick Craig-Wood)
        * Optimise `--copy-dest` and `--compare-dest` (Nick Craig-Wood)
    * rc: add `job/stopgroup` to stop group (Evan Spensley)
    * serve dlna
        * Add `--announce-interval` to control SSDP Announce Interval (YanceyChiew)
        * Add `--interface` to Specify SSDP interface names line (Simon Bos)
        * Add support for more external subtitles (YanceyChiew)
        * Add verification of addresses (YanceyChiew)
    * sync: Optimise `--copy-dest` and `--compare-dest` (Nick Craig-Wood)
    * doc updates (albertony, Alexander Knorr, anonion, João Henrique Franco, Josh Soref, Lorenzo Milesi, Marco Molteni, Mark Trolley, Ole Frost, partev, Ryan Morey, Tom Mombourquette, YFdyh000)
* Bug Fixes
    * filter
        * Fix incorrect filtering with `UseFilter` context flag and wrapping backends (Nick Craig-Wood)
        * Make sure we check `--files-from` when looking for a single file (Nick Craig-Wood)
    * rc
        * Fix `mount/listmounts` not returning the full Fs entered in `mount/mount` (Tom Mombourquette)
        * Handle external unmount when mounting (Isaac Aymerich)
        * Validate Daemon option is not set when mounting a volume via RC (Isaac Aymerich)
    * sync: Update docs and error messages to reflect fixes to overlap checks (Nick Naumann)
* VFS
    * Reduce memory use by embedding `sync.Cond` (Nick Craig-Wood)
    * Reduce memory usage by re-ordering commonly used structures (Nick Craig-Wood)
    * Fix excess CPU used by VFS cache cleaner looping (Nick Craig-Wood)
* Local
    * Obey file filters in listing to fix errors on excluded files (Nick Craig-Wood)
    * Fix "Failed to read metadata: function not implemented" on old Linux kernels (Nick Craig-Wood)
* Compress
    * Fix crash due to nil metadata (Nick Craig-Wood)
    * Fix error handling to not use or return nil objects (Nick Craig-Wood)
* Drive
    * Make `--drive-stop-on-upload-limit` obey quota exceeded error (Steve Kowalik)
* FTP
    * Add `--ftp-force-list-hidden` option to show hidden items (Øyvind Heddeland Instefjord)
    * Fix hang when using ExplicitTLS to certain servers. (Nick Craig-Wood)
* Google Cloud Storage
    * Add `--gcs-endpoint` flag and config parameter (Nick Craig-Wood)
* Hubic
    * Remove backend as service has now shut down (Nick Craig-Wood)
* Onedrive
    * Rename Onedrive(cn) 21Vianet to Vnet Group (Yen Hu)
    * Disable change notify in China region since it is not supported (Nick Craig-Wood)
* S3
    * Implement `--s3-versions` flag to show old versions of objects if enabled (Nick Craig-Wood)
    * Implement `--s3-version-at` flag to show versions of objects at a particular time (Nick Craig-Wood)
    * Implement `backend versioning` command to get/set bucket versioning (Nick Craig-Wood)
    * Implement `Purge` to purge versions and `backend cleanup-hidden` (Nick Craig-Wood)
    * Add `--s3-decompress` flag to decompress gzip-encoded files (Nick Craig-Wood)
    * Add `--s3-sse-customer-key-base64` to supply keys with binary data (Richard Bateman)
    * Try to keep the maximum precision in ModTime with `--user-server-modtime` (Nick Craig-Wood)
    * Drop binary metadata with an ERROR message as it can't be stored (Nick Craig-Wood)
    * Add `--s3-no-system-metadata` to suppress read and write of system metadata (Nick Craig-Wood)
* SFTP
    * Fix directory creation races (Lesmiscore)
* Swift
    * Add `--swift-no-large-objects` to reduce HEAD requests (Nick Craig-Wood)
* Union
    * Propagate SlowHash feature to fix hasher interaction (Lesmiscore)

## v1.59.2 - 2022-09-15

[See commits](https://github.com/rclone/rclone/compare/v1.59.1...v1.59.2)

* Bug Fixes
    * config: Move locking to fix fatal error: concurrent map read and map write (Nick Craig-Wood)
* Local
    * Disable xattr support if the filesystems indicates it is not supported (Nick Craig-Wood)
* Azure Blob
    * Fix chunksize calculations producing too many parts (Nick Craig-Wood)
* B2
    * Fix chunksize calculations producing too many parts (Nick Craig-Wood)
* S3
    * Fix chunksize calculations producing too many parts (Nick Craig-Wood)

## v1.59.1 - 2022-08-08

[See commits](https://github.com/rclone/rclone/compare/v1.59.0...v1.59.1)

* Bug Fixes
    * accounting: Fix panic in core/stats-reset with unknown group (Nick Craig-Wood)
    * build: Fix android build after GitHub actions change (Nick Craig-Wood)
    * dlna: Fix SOAP action header parsing (Joram Schrijver)
    * docs: Fix links to mount command from install docs (albertony)
    * dropbox: Fix ChangeNotify was unable to decrypt errors (Nick Craig-Wood)
    * fs: Fix parsing of times and durations of the form "YYYY-MM-DD HH:MM:SS" (Nick Craig-Wood)
    * serve sftp: Fix checksum detection (Nick Craig-Wood)
    * sync: Add accidentally missed filter-sensitivity to --backup-dir option (Nick Naumann)
* Combine
    * Fix docs showing `remote=` instead of `upstreams=` (Nick Craig-Wood)
    * Throw error if duplicate directory name is specified (Nick Craig-Wood)
    * Fix errors with backends shutting down while in use (Nick Craig-Wood)
* Dropbox
    * Fix hang on quit with --dropbox-batch-mode off (Nick Craig-Wood)
    * Fix infinite loop on uploading a corrupted file (Nick Craig-Wood)
* Internetarchive
    * Ignore checksums for files using the different method (Lesmiscore)
    * Handle hash symbol in the middle of filename (Lesmiscore)
* Jottacloud
    * Fix working with whitelabel Elgiganten Cloud
    * Do not store username in config when using standard auth (albertony)
* Mega
    * Fix nil pointer exception when bad node received (Nick Craig-Wood)
* S3
    * Fix --s3-no-head panic: reflect: Elem of invalid type s3.PutObjectInput (Nick Craig-Wood)
* SFTP
    * Fix issue with WS_FTP by working around failing RealPath (albertony)
* Union
    * Fix duplicated files when using directories with leading / (Nick Craig-Wood)
    * Fix multiple files being uploaded when roots don't exist (Nick Craig-Wood)
    * Fix panic due to misalignment of struct field in 32 bit architectures (r-ricci)

## v1.59.0 - 2022-07-09

[See commits](https://github.com/rclone/rclone/compare/v1.58.0...v1.59.0)

* New backends
    * [Combine](/combine) multiple remotes in one directory tree (Nick Craig-Wood)
    * [Hidrive](/hidrive/) (Ovidiu Victor Tatar)
    * [Internet Archive](/internetarchive/) (Lesmiscore (Naoya Ozaki))
    * New S3 providers
        * [ArvanCloud AOS](/s3/#arvan-cloud) (ehsantdy)
        * [Cloudflare R2](/s3/#cloudflare-r2) (Nick Craig-Wood)
        * [Huawei OBS](/s3/#huawei-obs) (m00594701)
        * [IDrive e2](/s3/#idrive-e2) (vyloy)
* New commands
    * [test makefile](/commands/rclone_test_makefile/): Create a single file for testing (Nick Craig-Wood)
* New Features
    * [Metadata framework](/docs/#metadata) to read and write system and user metadata on backends (Nick Craig-Wood)
        * Implemented initially for `local`, `s3` and `internetarchive` backends
        * `--metadata`/`-M` flag to control whether metadata is copied
        * `--metadata-set` flag to specify metadata for uploads
        * Thanks to [Manz Solutions](https://manz-solutions.at/) for sponsoring this work.
    * build
        * Update to go1.18 and make go1.16 the minimum required version (Nick Craig-Wood)
        * Update android go build to 1.18.x and NDK to 23.1.7779620 (Nick Craig-Wood)
        * All windows binaries now no longer CGO (Nick Craig-Wood)
        * Add `linux/arm/v6` to docker images (Nick Craig-Wood)
        * A huge number of fixes found with [staticcheck](https://staticcheck.io/) (albertony)
        * Configurable version suffix independent of version number (albertony)
    * check: Implement `--no-traverse` and `--no-unicode-normalization` (Nick Craig-Wood)
    * config: Readability improvements (albertony)
    * copyurl: Add `--header-filename` to honor the HTTP header filename directive (J-P Treen)
    * filter: Allow multiple `--exclude-if-present` flags (albertony)
    * fshttp: Add `--disable-http-keep-alives` to disable HTTP Keep Alives (Nick Craig-Wood)
    * install.sh
        * Set the modes on the files and/or directories on macOS (Michael C Tiernan - MIT-Research Computing Project)
        * Pre verify sudo authorization `-v` before calling curl. (Michael C Tiernan - MIT-Research Computing Project)
    * lib/encoder: Add Semicolon encoding (Nick Craig-Wood)
    * lsf: Add metadata support with `M` flag (Nick Craig-Wood)
    * lsjson: Add `--metadata`/`-M` flag (Nick Craig-Wood)
    * ncdu
        * Implement multi selection (CrossR)
        * Replace termbox with tcell's termbox wrapper (eNV25)
        * Display correct path in delete confirmation dialog (Roberto Ricci)
    * operations
        * Speed up hash checking by aborting the other hash if first returns nothing (Nick Craig-Wood)
        * Use correct src/dst in some log messages (zzr93)
    * rcat: Check checksums by default like copy does (Nick Craig-Wood)
    * selfupdate: Replace deprecated `x/crypto/openpgp` package with `ProtonMail/go-crypto` (albertony)
    * serve ftp: Check `--passive-port` arguments are correct (Nick Craig-Wood)
    * size: Warn about inaccurate results when objects with unknown size (albertony)
    * sync: Overlap check is now filter-sensitive so `--backup-dir` can be in the root provided it is filtered (Nick)
    * test info: Check file name lengths using 1,2,3,4 byte unicode characters (Nick Craig-Wood)
    * test makefile(s): `--sparse`, `--zero`, `--pattern`, `--ascii`, `--chargen` flags to control file contents (Nick Craig-Wood)
    * Make sure we call the `Shutdown` method on backends (Martin Czygan)
* Bug Fixes
    * accounting: Fix unknown length file transfers counting 3 transfers each (buda)
    * ncdu: Fix issue where dir size is summed when file sizes are -1 (albertony)
    * sync/copy/move
        * Fix `--fast-list` `--create-empty-src-dirs` and `--exclude` (Nick Craig-Wood)
        * Fix `--max-duration` and `--cutoff-mode soft` (Nick Craig-Wood)
    * Fix fs cache unpin (Martin Czygan)
    * Set proper exit code for errors that are not low-level retried (e.g. size/timestamp changing) (albertony)
* Mount
    * Support `windows/arm64` (may still be problems - see [#5828](https://github.com/rclone/rclone/issues/5828)) (Nick Craig-Wood)
    * Log IO errors at ERROR level (Nick Craig-Wood)
    * Ignore `_netdev` mount argument (Hugal31)
* VFS
    * Add `--vfs-fast-fingerprint` for less accurate but faster fingerprints (Nick Craig-Wood)
    * Add `--vfs-disk-space-total-size` option to manually set the total disk space (Claudio Maradonna)
    * vfscache: Fix fatal error: sync: unlock of unlocked mutex error (Nick Craig-Wood)
* Local
    * Fix parsing of `--local-nounc` flag (Nick Craig-Wood)
    * Add Metadata support (Nick Craig-Wood)
* Crypt
    * Support metadata (Nick Craig-Wood)
* Azure Blob
    * Calculate Chunksize/blocksize to stay below maxUploadParts (Leroy van Logchem)
    * Use chunksize lib to determine chunksize dynamically (Derek Battams)
    * Case insensitive access tier (Rob Pickerill)
    * Allow remote emulator (azurite) (Lorenzo Maiorfi)
* B2
    * Add `--b2-version-at` flag to show file versions at time specified (SwazRGB)
    * Use chunksize lib to determine chunksize dynamically (Derek Battams)
* Chunker
    * Mark as not supporting metadata (Nick Craig-Wood)
* Compress
    * Support metadata (Nick Craig-Wood)
* Drive
    * Make `backend config -o config` add a combined `AllDrives:` remote (Nick Craig-Wood)
    * Make `--drive-shared-with-me` work with shared drives (Nick Craig-Wood)
    * Add `--drive-resource-key` for accessing link-shared files (Nick Craig-Wood)
    * Add backend commands `exportformats` and `importformats` for debugging (Nick Craig-Wood)
    * Fix 404 errors on copy/server side copy objects from public folder (Nick Craig-Wood)
    * Update Internal OAuth consent screen docs (Phil Shackleton)
    * Moved `root_folder_id` to advanced section (Abhiraj)
* Dropbox
    * Migrate from deprecated api (m8rge)
    * Add logs to show when poll interval limits are exceeded (Nick Craig-Wood)
    * Fix nil pointer exception on dropbox impersonate user not found (Nick Craig-Wood)
* Fichier
    * Parse api error codes and them accordingly (buengese)
* FTP
    * Add support for `disable_utf8` option (Jason Zheng)
    * Revert to upstream `github.com/jlaffaye/ftp` from our fork (Nick Craig-Wood)
* Google Cloud Storage
    * Add `--gcs-no-check-bucket` to minimise transactions and perms (Nick Gooding)
    * Add `--gcs-decompress` flag to decompress gzip-encoded files (Nick Craig-Wood)
        * by default these will be downloaded compressed (which previously failed)
* Hasher
    * Support metadata (Nick Craig-Wood)
* HTTP
    * Fix missing response when using custom auth handler (albertony)
* Jottacloud
    * Add support for upload to custom device and mountpoint (albertony)
    * Always store username in config and use it to avoid initial API request (albertony)
    * Fix issue with server-side copy when destination is in trash (albertony)
    * Fix listing output of remote with special characters (albertony)
* Mailru
    * Fix timeout by using int instead of time.Duration for keeping number of seconds (albertony)
* Mega
    * Document using MEGAcmd to help with login failures (Art M. Gallagher)
* Onedrive
    * Implement `--poll-interval` for onedrive (Hugo Laloge)
    * Add access scopes option (Sven Gerber)
* Opendrive
    * Resolve lag and truncate bugs (Scott Grimes)
* Pcloud
    * Fix about with no free space left (buengese)
    * Fix cleanup (buengese)
* S3
    * Use PUT Object instead of presigned URLs to upload single part objects (Nick Craig-Wood)
    * Backend restore command to skip non-GLACIER objects (Vincent Murphy)
    * Use chunksize lib to determine chunksize dynamically (Derek Battams)
    * Retry RequestTimeout errors (Nick Craig-Wood)
    * Implement reading and writing of metadata (Nick Craig-Wood)
* SFTP
    * Add support for about and hashsum on windows server (albertony)
    * Use vendor-specific VFS statistics extension for about if available (albertony)
    * Add `--sftp-chunk-size` to control packets sizes for high latency links (Nick Craig-Wood)
    * Add `--sftp-concurrency` to improve high latency transfers (Nick Craig-Wood)
    * Add `--sftp-set-env` option to set environment variables (Nick Craig-Wood)
    * Add Hetzner Storage Boxes to supported sftp backends (Anthrazz)
* Storj
    * Fix put which lead to the file being unreadable when using mount (Erik van Velzen)
* Union
    * Add `min_free_space` option for `lfs`/`eplfs` policies (Nick Craig-Wood)
    * Fix uploading files to union of all bucket based remotes (Nick Craig-Wood)
    * Fix get free space for remotes which don't support it (Nick Craig-Wood)
    * Fix `eplus` policy to select correct entry for existing files (Nick Craig-Wood)
    * Support metadata (Nick Craig-Wood)
* Uptobox
    * Fix root path handling (buengese)
* WebDAV
    * Add SharePoint in other specific regions support (Noah Hsu)
* Yandex
    * Handle api error on server-side move (albertony)
* Zoho
    * Add Japan and China regions (buengese)

## v1.58.1 - 2022-04-29

[See commits](https://github.com/rclone/rclone/compare/v1.58.0...v1.58.1)

* Bug Fixes
    * build: Update github.com/billziss-gh to github.com/winfsp (Nick Craig-Wood)
    * filter: Fix timezone of `--min-age`/`-max-age` from UTC to local as documented (Nick Craig-Wood)
    * rc/js: Correct RC method names (Sơn Trần-Nguyễn)
    * docs
        * Fix some links to command pages (albertony)
        * Add `--multi-thread-streams` note to `--transfers`. (Zsolt Ero)
* Mount
    * Fix `--devname` and fusermount: unknown option 'fsname' when mounting via rc (Nick Craig-Wood)
* VFS
    * Remove wording which suggests VFS is only for mounting (Nick Craig-Wood)
* Dropbox
    * Fix retries of multipart uploads with incorrect_offset error (Nick Craig-Wood)
* Google Cloud Storage
    * Use the s3 pacer to speed up transactions (Nick Craig-Wood)
    * pacer: Default the Google pacer to a burst of 100 to fix gcs pacing (Nick Craig-Wood)
* Jottacloud
    * Fix scope in token request (albertony)
* Netstorage
    * Fix unescaped HTML in documentation (Nick Craig-Wood)
    * Make levels of headings consistent (Nick Craig-Wood)
    * Add support contacts to netstorage doc (Nil Alexandrov)
* Onedrive
    * Note that sharepoint also changes web files (.html, .aspx) (GH)
* Putio
    * Handle rate limit errors (Berkan Teber)
    * Fix multithread download and other ranged requests (rafma0)
* S3
    * Add ChinaMobile EOS to provider list (GuoXingbin)
    * Sync providers in config description with providers (Nick Craig-Wood)
* SFTP
    * Fix OpenSSH 8.8+ RSA keys incompatibility (KARBOWSKI Piotr)
    * Note that Scaleway C14 is deprecating SFTP in favor of S3 (Adrien Rey-Jarthon)
* Storj
    * Fix bucket creation on Move (Nick Craig-Wood)
* WebDAV
    * Don't override Referer if user sets it (Nick Craig-Wood)

## v1.58.0 - 2022-03-18

[See commits](https://github.com/rclone/rclone/compare/v1.57.0...v1.58.0)

* New backends
    * [Akamai Netstorage](/netstorage) (Nil Alexandrov)
    * [Seagate Lyve](/s3/#lyve), [SeaweedFS](/s3/#seaweedfs), [Storj](/s3/#storj), [RackCorp](/s3/#RackCorp) via s3 backend
    * [Storj](/storj/) (renamed from Tardigrade - your old config files will continue working)
* New commands
    * [bisync](/bisync/) - experimental bidirectional cloud sync (Ivan Andreev, Chris Nelson)
* New Features
    * build
        * Add `windows/arm64` build (`rclone mount` not supported yet) (Nick Craig-Wood)
        * Raise minimum go version to go1.15 (Nick Craig-Wood)
    * config: Allow dot in remote names and improve config editing (albertony)
    * dedupe: Add quit as a choice in interactive mode (albertony)
    * dlna: Change icons to the newest ones. (Alain Nussbaumer)
    * filter: Add [`{{ regexp }}` syntax](/filtering/#regexp) to pattern matches (Nick Craig-Wood)
    * fshttp: Add prometheus metrics for HTTP status code (Michał Matczuk)
    * hashsum: Support creating hash from data received on stdin (albertony)
    * librclone
        * Allow empty string or null input instead of empty json object (albertony)
        * Add support for mount commands (albertony)
    * operations: Add server-side moves to stats (Ole Frost)
    * rc: Allow user to disable authentication for web gui (negative0)
    * tree: Remove obsolete `--human` replaced by global `--human-readable` (albertony)
    * version: Report correct friendly-name for newer Windows 10/11 versions (albertony)
* Bug Fixes
    * build
        * Fix ARM architecture version in .deb packages after nfpm change (Nick Craig-Wood)
        * Hard fork `github.com/jlaffaye/ftp` to fix `go get github.com/rclone/rclone` (Nick Craig-Wood)
    * oauthutil: Fix crash when webbrowser requests `/robots.txt` (Nick Craig-Wood)
    * operations: Fix goroutine leak in case of copy retry (Ankur Gupta)
    * rc:
        * Fix `operations/publiclink` default for `expires` parameter (Nick Craig-Wood)
        * Fix missing computation of `transferQueueSize` when summing up statistics group (Carlo Mion)
        * Fix missing `StatsInfo` fields in the computation of the group sum (Carlo Mion)
    * sync: Fix `--max-duration` so it doesn't retry when the duration is exceeded (Nick Craig-Wood)
    * touch: Fix issue where a directory is created instead of a file (albertony)
* Mount
    * Add `--devname` to set the device name sent to FUSE for mount display (Nick Craig-Wood)
* VFS
    * Add `vfs/stats` remote control to show statistics (Nick Craig-Wood)
    * Fix `failed to _ensure cache internal error: downloaders is nil error` (Nick Craig-Wood)
    * Fix handling of special characters in file names (Bumsu Hyeon)
* Local
    * Fix hash invalidation which caused errors with local crypt mount (Nick Craig-Wood)
* Crypt
    * Add `base64` and `base32768` filename encoding options (Max Sum, Sinan Tan)
* Azure Blob
    * Implement `--azureblob-upload-concurrency` parameter to speed uploads (Nick Craig-Wood)
    * Remove 100MB upper limit on `chunk_size` as it is no longer needed (Nick Craig-Wood)
    * Raise `--azureblob-upload-concurrency` to 16 by default (Nick Craig-Wood)
    * Fix crash with SAS URL and no container (Nick Craig-Wood)
* Compress
    * Fix crash if metadata upload failed (Nick Craig-Wood)
    * Fix memory leak (Nick Craig-Wood)
* Drive
    * Added `--drive-copy-shortcut-content` (Abhiraj)
    * Disable OAuth OOB flow (copy a token) due to Google deprecation (Nick Craig-Wood)
        * See [the deprecation note](https://developers.googleblog.com/2022/02/making-oauth-flows-safer.html#disallowed-oob).
    * Add `--drive-skip-dangling-shortcuts` flag (Nick Craig-Wood)
    * When using a link type `--drive-export-formats` shows all doc types (Nick Craig-Wood)
* Dropbox
    * Speed up directory listings by specifying 1000 items in a chunk (Nick Craig-Wood)
    * Save an API request when at the root (Nick Craig-Wood)
* Fichier
    * Implemented About functionality (Gourav T)
* FTP
    * Add `--ftp-ask-password` to prompt for password when needed (Borna Butkovic)
* Google Cloud Storage
    * Add missing regions (Nick Craig-Wood)
    * Disable OAuth OOB flow (copy a token) due to Google deprecation (Nick Craig-Wood)
        * See [the deprecation note](https://developers.googleblog.com/2022/02/making-oauth-flows-safer.html#disallowed-oob).
* Googlephotos
    * Disable OAuth OOB flow (copy a token) due to Google deprecation (Nick Craig-Wood)
        * See [the deprecation note](https://developers.googleblog.com/2022/02/making-oauth-flows-safer.html#disallowed-oob).
* Hasher
    * Fix crash on object not found (Nick Craig-Wood)
* Hdfs
    * Add file (Move) and directory move (DirMove) support (Andy Jackson)
* HTTP
    * Improved recognition of URL pointing to a single file (albertony)
* Jottacloud
    * Change API used by recursive list (ListR) (Kim)
    * Add support for Tele2 Cloud (Fredric Arklid)
* Koofr
    * Add Digistorage service as a Koofr provider. (jaKa)
* Mailru
    * Fix int32 overflow on arm32 (Ivan Andreev)
* Onedrive
    * Add config option for oauth scope `Sites.Read.All` (Charlie Jiang)
    * Minor optimization of quickxorhash (Isaac Levy)
    * Add `--onedrive-root-folder-id` flag (Nick Craig-Wood)
    * Do not retry on `400 pathIsTooLong` error (ctrl-q)
* Pcloud
    * Add support for recursive list (ListR) (Niels van de Weem)
    * Fix pre-1970 time stamps (Nick Craig-Wood)
* S3
    * Use `ListObjectsV2` for faster listings (Felix Bünemann)
        * Fallback to `ListObject` v1 on unsupported providers (Nick Craig-Wood)
    * Use the `ETag` on multipart transfers to verify the transfer was OK (Nick Craig-Wood)
        * Add `--s3-use-multipart-etag` provider quirk to disable this on unsupported providers (Nick Craig-Wood)
    * New Providers
        * RackCorp object storage (bbabich)
        * Seagate Lyve Cloud storage (Nick Craig-Wood)
        * SeaweedFS (Chris Lu)
        * Storj Shared gateways (Márton Elek, Nick Craig-Wood)
    * Add Wasabi AP Northeast 2 endpoint info (lindwurm)
    * Add `GLACIER_IR` storage class (Yunhai Luo)
    * Document `Content-MD5` workaround for object-lock enabled buckets (Paulo Martins)
    * Fix multipart upload with `--no-head` flag (Nick Craig-Wood)
    * Simplify content length processing in s3 with download url (Logeshwaran Murugesan)
* SFTP
    * Add rclone to list of supported `md5sum`/`sha1sum` commands to look for (albertony)
    * Refactor so we only have one way of running remote commands (Nick Craig-Wood)
    * Fix timeout on hashing large files by sending keepalives (Nick Craig-Wood)
    * Fix unnecessary seeking when uploading and downloading files (Nick Craig-Wood)
    * Update docs on how to create `known_hosts` file (Nick Craig-Wood)
* Storj
    * Rename tardigrade backend to storj backend (Nick Craig-Wood)
    * Implement server side Move for files (Nick Craig-Wood)
    * Update docs to explain differences between s3 and this backend (Elek, Márton)
* Swift
    * Fix About so it shows info about the current container only (Nick Craig-Wood)
* Union
    * Fix treatment of remotes with `//` in (Nick Craig-Wood)
    * Fix deadlock when one part of a multi-upload fails (Nick Craig-Wood)
    * Fix eplus policy returned nil (Vitor Arruda)
* Yandex
    * Add permanent deletion support (deinferno)

## v1.57.0 - 2021-11-01

[See commits](https://github.com/rclone/rclone/compare/v1.56.0...v1.57.0)

* New backends
    * Sia: for Sia decentralized cloud (Ian Levesque, Matthew Sevey, Ivan Andreev)
    * Hasher: caches hashes and enable hashes for backends that don't support them (Ivan Andreev)
* New commands
    * lsjson --stat: to get info about a single file/dir and `operations/stat` api (Nick Craig-Wood)
    * config paths: show configured paths (albertony)
* New Features
    * about: Make human-readable output more consistent with other commands (albertony)
    * build
        * Use go1.17 for building and make go1.14 the minimum supported  (Nick Craig-Wood)
        * Update Go to 1.16 and NDK to 22b for Android builds (x0b)
    * config
        * Support hyphen in remote name from environment variable (albertony)
        * Make temporary directory user-configurable (albertony)
        * Convert `--cache-dir` value to an absolute path (albertony)
        * Do not override MIME types from OS defaults (albertony)
    * docs
        * Toc styling and header levels cleanup (albertony)
        * Extend documentation on valid remote names (albertony)
        * Mention make for building and cmount tag for macos (Alex Chen)
        * ...and many more contributions to numerous to mention!
    * fs: Move with `--ignore-existing` will not delete skipped files (Nathan Collins)
    * hashsum
        * Treat hash values in sum file as case insensitive (Ivan Andreev)
        * Don't put `ERROR` or `UNSUPPORTED` in output (Ivan Andreev)
    * lib/encoder: Add encoding of square brackets (Ivan Andreev)
    * lib/file: Improve error message when attempting to create dir on nonexistent drive on windows (albertony)
    * lib/http: Factor password hash salt into options with default (Nolan Woods)
    * lib/kv: Add key-value database api (Ivan Andreev)
    * librclone
        * Add `RcloneFreeString` function (albertony)
        * Free strings in python example (albertony)
    * log: Optionally print pid in logs (Ivan Andreev)
    * ls: Introduce `--human-readable` global option to print human-readable sizes (albertony)
    * ncdu: Introduce key `u` to toggle human-readable (albertony)
    * operations: Add `rmdirs -v` output (Justin Winokur)
    * serve sftp
        * Generate an ECDSA server key as well as RSA (Nick Craig-Wood)
        * Generate an Ed25519 server key as well as ECDSA and RSA (albertony)
    * serve docker
        * Allow to customize proxy settings of docker plugin (Ivan Andreev)
        * Build docker plugin for multiple platforms (Thomas Stachl)
    * size: Include human-readable count (albertony)
    * touch: Add support for touching files in directory, with recursive option, filtering and `--dry-run`/`-i` (albertony)
    * tree: Option to print human-readable sizes removed in favor of global option (albertony)
* Bug Fixes
    * lib/http
        * Fix bad username check in single auth secret provider (Nolan Woods)
        * Fix handling of SSL credentials (Nolan Woods)
    * serve ftp: Ensure modtime is passed as UTC always to fix timezone oddities (Nick Craig-Wood)
    * serve sftp: Fix generation of server keys on windows (albertony)
    * serve docker: Fix octal umask (Ivan Andreev)
* Mount
    * Enable rclone to be run as mount helper direct from the fstab (Ivan Andreev)
    * Use procfs to validate mount on linux (Ivan Andreev)
    * Correctly daemonize for compatibility with automount (Ivan Andreev)
* VFS
    * Ensure names used in cache path are legal on current OS (albertony)
    * Ignore `ECLOSED` when truncating file handles to fix intermittent bad file descriptor error (Nick Craig-Wood)
* Local
    * Refactor default OS encoding out from local backend into shared encoder lib (albertony)
* Crypt
    * Return wrapped object even with `--crypt-no-data-encryption` (Ivan Andreev)
    * Fix uploads with `--crypt-no-data-encryption` (Nick Craig-Wood)
* Azure Blob
    * Add `--azureblob-no-head-object` (Tatsuya Noyori)
* Box
    * Make listings of heavily used directories more reliable (Nick Craig-Wood)
    * When doing cleanup delete as much as possible (Nick Craig-Wood)
    * Add `--box-list-chunk` to control listing chunk size (Nick Craig-Wood)
    * Delete items in parallel in cleanup using `--checkers` threads (Nick Craig-Wood)
    * Add `--box-owned-by` to only show items owned by the login passed (Nick Craig-Wood)
    * Retry `operation_blocked_temporary` errors (Nick Craig-Wood)
* Chunker
    * Md5all must create metadata if base hash is slow (Ivan Andreev)
* Drive
    * Speed up directory listings by constraining the API listing using the current filters (fotile96, Ivan Andreev)
    * Fix buffering for single request upload for files smaller than `--drive-upload-cutoff` (YenForYang)
    * Add `-o config` option to `backend drives` to make config for all shared drives (Nick Craig-Wood)
* Dropbox
    * Add `--dropbox-batch-commit-timeout` to control batch timeout (Nick Craig-Wood)
* Filefabric
    * Make backoff exponential for error_background to fix errors (Nick Craig-Wood)
    * Fix directory move after API change (Nick Craig-Wood)
* FTP
    * Enable tls session cache by default (Ivan Andreev)
    * Add option to disable tls13 (Ivan Andreev)
    * Fix timeout after long uploads (Ivan Andreev)
    * Add support for precise time (Ivan Andreev)
    * Enable CI for ProFtpd, PureFtpd, VsFtpd (Ivan Andreev)
* Googlephotos
    * Use encoder for album names to fix albums with control characters (Parth Shukla)
* Jottacloud
    * Implement `SetModTime` to support modtime-only changes (albertony)
    * Improved error handling with `SetModTime` and corrupt files in general (albertony)
    * Add support for `UserInfo` (`rclone config userinfo`) feature (albertony)
    * Return direct download link from `rclone link` command (albertony)
* Koofr
    * Create direct share link (Dmitry Bogatov)
* Pcloud
    * Add sha256 support (Ken Enrique Morel)
* Premiumizeme
    * Fix directory listing after API changes (Nick Craig-Wood)
    * Fix server side move after API change (Nick Craig-Wood)
    * Fix server side directory move after API changes (Nick Craig-Wood)
* S3
    * Add support to use CDN URL to download the file (Logeshwaran)
    * Add AWS Snowball Edge to providers examples (r0kk3rz)
    * Use a combination of SDK retries and rclone retries (Nick Craig-Wood)
    * Fix IAM Role for Service Account not working and other auth problems (Nick Craig-Wood)
    * Fix `shared_credentials_file` auth after reverting incorrect fix (Nick Craig-Wood)
    * Fix corrupted on transfer: sizes differ 0 vs xxxx with Ceph (Nick Craig-Wood)
* Seafile
    * Fix error when not configured for 2fa (Fred)
* SFTP
    * Fix timeout when doing MD5SUM of large file (Nick Craig-Wood)
* Swift
    * Update OCI URL (David Liu)
    * Document OVH Cloud Archive (HNGamingUK)
* Union
    * Fix rename not working with union of local disk and bucket based remote (Nick Craig-Wood)

## v1.56.2 - 2021-10-01

[See commits](https://github.com/rclone/rclone/compare/v1.56.1...v1.56.2)

* Bug Fixes
    * serve http: Re-add missing auth to http service (Nolan Woods)
    * build: Update golang.org/x/sys to fix crash on macOS when compiled with go1.17 (Herby Gillot)
* FTP
    * Fix deadlock after failed update when concurrency=1 (Ivan Andreev)

## v1.56.1 - 2021-09-19

[See commits](https://github.com/rclone/rclone/compare/v1.56.0...v1.56.1)

* Bug Fixes
    * accounting: Fix maximum bwlimit by scaling scale max token bucket size (Nick Craig-Wood)
    * rc: Fix speed does not update in core/stats (negative0)
    * selfupdate: Fix `--quiet` option, not quite quiet (yedamo)
    * serve http: Fix `serve http` exiting directly after starting (Cnly)
    * build
        * Apply gofmt from golang 1.17 (Ivan Andreev)
        * Update Go to 1.16 and NDK to 22b for android/any (x0b)
* Mount
    * Fix `--daemon` mode (Ivan Andreev)
* VFS
    * Fix duplicates on rename (Nick Craig-Wood)
    * Fix crash when truncating a just uploaded object (Nick Craig-Wood)
    * Fix issue where empty dirs would build up in cache meta dir (albertony)
* Drive
    * Fix instructions for auto config (Greg Sadetsky)
    * Fix lsf example without drive-impersonate (Greg Sadetsky)
* Onedrive
    * Handle HTTP 400 better in PublicLink (Alex Chen)
    * Clarification of the process for creating custom client_id (Mariano Absatz)
* Pcloud
    * Return an early error when Put is called with an unknown size (Nick Craig-Wood)
    * Try harder to delete a failed upload (Nick Craig-Wood)
* S3
    * Add Wasabi's AP-Northeast endpoint info (hota)
    * Fix typo in s3 documentation (Greg Sadetsky)
* Seafile
    * Fix 2fa config state machine (Fred)
* SFTP
    * Remove spurious error message on `--sftp-disable-concurrent-reads` (Nick Craig-Wood)
* Sugarsync
    * Fix initial connection after config re-arrangement (Nick Craig-Wood)

## v1.56.0 - 2021-07-20

[See commits](https://github.com/rclone/rclone/compare/v1.55.0...v1.56.0)

* New backends
    * [Uptobox](/uptobox/) (buengese)
* New commands
    * [serve docker](/commands/rclone_serve_docker/) (Antoine GIRARD) (Ivan Andreev)
        * and accompanying [docker volume plugin](/docker/)
    * [checksum](/commands/rclone_checksum/) to check files against a file of checksums (Ivan Andreev)
        * this is also available as `rclone md5sum -C` etc
    * [config touch](/commands/rclone_config_touch/): ensure config exists at configured location (albertony)
    * [test changenotify](/commands/rclone_test_changenotify/): command to help debugging changenotify (Nick Craig-Wood)
* Deprecations
    * `dbhashsum`: Remove command deprecated a year ago (Ivan Andreev)
    * `cache`: Deprecate cache backend (Ivan Andreev)
* New Features
    * rework config system so it can be used non-interactively via cli and rc API.
        * See docs in [config create](/commands/rclone_config_create/)
        * This is a very big change to all the backends so may cause breakages - please file bugs!
    * librclone - export the rclone RC as a C library (lewisxy) (Nick Craig-Wood)
        * Link a C-API rclone shared object into your project
        * Use the RC as an in memory interface
        * Python example supplied
        * Also supports Android and gomobile
    * fs
        * Add `--disable-http2` for global http2 disable (Nick Craig-Wood)
        * Make `--dump` imply `-vv` (Alex Chen)
        * Use binary prefixes for size and rate units (albertony)
        * Use decimal prefixes for counts (albertony)
        * Add google search widget to rclone.org (Ivan Andreev)
    * accounting: Calculate rolling average speed (Haochen Tong)
    * atexit: Terminate with non-zero status after receiving signal (Michael Hanselmann)
    * build
        * Only run event-based workflow scripts under rclone repo with manual override (Mathieu Carbou)
        * Add Android build with gomobile (x0b)
    * check: Log the hash in use like cryptcheck does (Nick Craig-Wood)
    * version: Print os/version, kernel and bitness (Ivan Andreev)
    * config
        * Prevent use of Windows reserved names in config file name (albertony)
        * Create config file in windows appdata directory by default (albertony)
        * Treat any config file paths with filename notfound as memory-only config (albertony)
        * Delay load config file (albertony)
        * Replace defaultConfig with a thread-safe in-memory implementation (Chris Macklin)
        * Allow `config create` and friends to take `key=value` parameters (Nick Craig-Wood)
        * Fixed issues with flags/options set by environment vars. (Ole Frost)
    * fshttp: Implement graceful DSCP error handling (Tyson Moore)
    * lib/http - provides an abstraction for a central http server that services can bind routes to (Nolan Woods)
        * Add `--template` config and flags to serve/data (Nolan Woods)
        * Add default 404 handler (Nolan Woods)
    * link: Use "off" value for unset expiry (Nick Craig-Wood)
    * oauthutil: Raise fatal error if token expired without refresh token (Alex Chen)
    * rcat: Add `--size` flag for more efficient uploads of known size (Nazar Mishturak)
    * serve sftp: Add `--stdio` flag to serve via stdio (Tom)
    * sync: Don't warn about `--no-traverse` when `--files-from` is set (Nick Gaya)
    * `test makefiles`
        * Add `--seed` flag and make data generated repeatable (Nick Craig-Wood)
        * Add log levels and speed summary (Nick Craig-Wood)
* Bug Fixes
    * accounting: Fix startTime of statsGroups.sum (Haochen Tong)
    * cmd/ncdu: Fix out of range panic in delete (buengese)
    * config
        * Fix issues with memory-only config file paths (albertony)
        * Fix in memory config not saving on the fly backend config (Nick Craig-Wood)
    * fshttp: Fix address parsing for DSCP (Tyson Moore)
    * ncdu: Update termbox-go library to fix crash (Nick Craig-Wood)
    * oauthutil: Fix old authorize result not recognised (Cnly)
    * operations: Don't update timestamps of files in `--compare-dest` (Nick Gaya)
    * selfupdate: fix archive name on macos (Ivan Andreev)
* Mount
    * Refactor before adding serve docker (Antoine GIRARD)
* VFS
    * Add cache reset for `--vfs-cache-max-size` handling at cache poll interval (Leo Luan)
    * Fix modtime changing when reading file into cache (Nick Craig-Wood)
    * Avoid unnecessary subdir in cache path (albertony)
    * Fix that umask option cannot be set as environment variable (albertony)
    * Do not print notice about missing poll-interval support when set to 0 (albertony)
* Local
    * Always use readlink to read symlink size for better compatibility (Nick Craig-Wood)
    * Add `--local-unicode-normalization` (and remove `--local-no-unicode-normalization`) (Nick Craig-Wood)
    * Skip entries removed concurrently with List() (Ivan Andreev)
* Crypt
    * Support timestamped filenames from `--b2-versions` (Dominik Mydlil)
* B2
    * Don't include the bucket name in public link file prefixes (Jeffrey Tolar)
    * Fix versions and .files with no extension (Nick Craig-Wood)
    * Factor version handling into lib/version (Dominik Mydlil)
* Box
    * Use upload preflight check to avoid listings in file uploads (Nick Craig-Wood)
    * Return errors instead of calling log.Fatal with them (Nick Craig-Wood)
* Drive
    * Switch to the Drives API for looking up shared drives (Nick Craig-Wood)
    * Fix some google docs being treated as files (Nick Craig-Wood)
* Dropbox
    * Add `--dropbox-batch-mode` flag to speed up uploading (Nick Craig-Wood)
        * Read the [batch mode](/dropbox/#batch-mode) docs for more info
    * Set visibility in link sharing when `--expire` is set (Nick Craig-Wood)
    * Simplify chunked uploads (Alexey Ivanov)
    * Improve "own App IP" instructions (Ivan Andreev)
* Fichier
    * Check if more than one upload link is returned (Nick Craig-Wood)
    * Support downloading password protected files and folders (Florian Penzkofer)
    * Make error messages report text from the API (Nick Craig-Wood)
    * Fix move of files in the same directory (Nick Craig-Wood)
    * Check that we actually got a download token and retry if we didn't (buengese)
* Filefabric
    * Fix listing after change of from field from "int" to int. (Nick Craig-Wood)
* FTP
    * Make upload error 250 indicate success (Nick Craig-Wood)
* GCS
  * Make compatible with gsutil's mtime metadata (database64128)
  * Clean up time format constants (database64128)
* Google Photos
  * Fix read only scope not being used properly (Nick Craig-Wood)
* HTTP
    * Replace httplib with lib/http (Nolan Woods)
    * Clean up Bind to better use middleware (Nolan Woods)
* Jottacloud
    * Fix legacy auth with state based config system (buengese)
    * Fix invalid url in output from link command (albertony)
    * Add no versions option (buengese)
* Onedrive
    * Add `list_chunk option` (Nick Gaya)
    * Also report root error if unable to cancel multipart upload (Cnly)
    * Fix  failed to configure: empty token found error (Nick Craig-Wood)
    * Make link return direct download link (Xuanchen Wu)
* S3
    * Add `--s3-no-head-object` (Tatsuya Noyori)
    * Remove WebIdentityRoleProvider to fix crash on auth (Nick Craig-Wood)
    * Don't check to see if remote is object if it ends with / (Nick Craig-Wood)
    * Add SeaweedFS (Chris Lu)
    * Update Alibaba OSS endpoints (Chuan Zh)
* SFTP
    * Fix performance regression by re-enabling concurrent writes (Nick Craig-Wood)
    * Expand tilde and environment variables in configured `known_hosts_file` (albertony)
* Tardigrade
    * Upgrade to uplink v1.4.6 (Caleb Case)
    * Use negative offset (Caleb Case)
    * Add warning about `too many open files` (acsfer)
* WebDAV
    * Fix sharepoint auth over http (Nick Craig-Wood)
    * Add headers option (Antoon Prins)

## v1.55.1 - 2021-04-26

[See commits](https://github.com/rclone/rclone/compare/v1.55.0...v1.55.1)

* Bug Fixes
    * selfupdate
        * Dont detect FUSE if build is static (Ivan Andreev)
        * Add build tag noselfupdate (Ivan Andreev)
    * sync: Fix incorrect error reported by graceful cutoff (Nick Craig-Wood)
    * install.sh: fix macOS arm64 download (Nick Craig-Wood)
    * build: Fix version numbers in android branch builds (Nick Craig-Wood)
    * docs
        * Contributing.md: update setup instructions for go1.16 (Nick Gaya)
        * WinFsp 2021 is out of beta (albertony)
        * Minor cleanup of space around code section (albertony)
        * Fixed some typos (albertony)
* VFS
    * Fix a code path which allows dirty data to be removed causing data loss (Nick Craig-Wood)
* Compress
    * Fix compressed name regexp (buengese)
* Drive
    * Fix backend copyid of google doc to directory (Nick Craig-Wood)
    * Don't open browser when service account... (Ansh Mittal)
* Dropbox
    * Add missing team_data.member scope for use with --impersonate (Nick Craig-Wood)
    * Fix About after scopes changes - rclone config reconnect needed (Nick Craig-Wood)
    * Fix Unable to decrypt returned paths from changeNotify (Nick Craig-Wood)
* FTP
    * Fix implicit TLS (Ivan Andreev)
* Onedrive
    * Work around for random "Unable to initialize RPS" errors (OleFrost)
* SFTP
    * Revert sftp library to v1.12.0 from v1.13.0 to fix performance regression (Nick Craig-Wood)
    * Fix Update ReadFrom failed: failed to send packet: EOF errors (Nick Craig-Wood)
* Zoho
    * Fix error when region isn't set (buengese)
    * Do not ask for mountpoint twice when using headless setup (buengese)

## v1.55.0 - 2021-03-31

[See commits](https://github.com/rclone/rclone/compare/v1.54.0...v1.55.0)

* New commands
    * [selfupdate](/commands/rclone_selfupdate/) (Ivan Andreev)
        * Allows rclone to update itself in-place or via a package (using `--package` flag)
        * Reads cryptographically signed signatures for non beta releases
        * Works on all OSes.
    * [test](/commands/rclone_test/) - these are test commands - use with care!
        * `histogram` - Makes a histogram of file name characters.
        * `info` - Discovers file name or other limitations for paths.
        * `makefiles` - Make a random file hierarchy for testing.
        * `memory` - Load all the objects at remote:path into memory and report memory stats.
* New Features
    * [Connection strings](/docs/#connection-strings)
        * Config parameters can now be passed as part of the remote name as a connection string.
        * For example, to do the equivalent of `--drive-shared-with-me` use `drive,shared_with_me:`
        * Make sure we don't save on the fly remote config to the config file (Nick Craig-Wood)
        * Make sure backends with additional config have a different name for caching (Nick Craig-Wood)
        * This work was sponsored by CERN, through the [CS3MESH4EOSC Project](https://cs3mesh4eosc.eu/).
            * CS3MESH4EOSC has received funding from the European Union’s Horizon 2020
            * research and innovation programme under Grant Agreement no. 863353.
    * build
        * Update go build version to go1.16 and raise minimum go version to go1.13 (Nick Craig-Wood)
        * Make a macOS ARM64 build to support Apple Silicon (Nick Craig-Wood)
        * Install macfuse 4.x instead of osxfuse 3.x (Nick Craig-Wood)
        * Use `GO386=softfloat` instead of deprecated `GO386=387` for 386 builds (Nick Craig-Wood)
        * Disable IOS builds for the time being (Nick Craig-Wood)
        * Androids builds made with up to date NDK (x0b)
        * Add an rclone user to the Docker image but don't use it by default (cynthia kwok)
    * dedupe: Make largest directory primary to minimize data moved (Saksham Khanna)
    * config
        * Wrap config library in an interface (Fionera)
        * Make config file system pluggable (Nick Craig-Wood)
        * `--config ""` or `"/notfound"` for in memory config only (Nick Craig-Wood)
        * Clear fs cache of stale entries when altering config (Nick Craig-Wood)
    * copyurl: Add option to print resulting auto-filename (albertony)
    * delete: Make `--rmdirs` obey the filters (Nick Craig-Wood)
    * docs - many fixes and reworks from edwardxml, albertony, pvalls, Ivan Andreev, Evan Harris, buengese, Alexey Tabakman
    * encoder/filename - add SCSU as tables (Klaus Post)
    * Add multiple paths support to `--compare-dest` and `--copy-dest` flag (K265)
    * filter: Make `--exclude "dir/"` equivalent to `--exclude "dir/**"` (Nick Craig-Wood)
    * fshttp: Add DSCP support with `--dscp` for QoS with differentiated services (Max Sum)
    * lib/cache: Add Delete and DeletePrefix methods (Nick Craig-Wood)
    * lib/file
        * Make pre-allocate detect disk full errors and return them (Nick Craig-Wood)
        * Don't run preallocate concurrently (Nick Craig-Wood)
        * Retry preallocate on EINTR (Nick Craig-Wood)
    * operations: Made copy and sync operations obey a RetryAfterError (Ankur Gupta)
    * rc
        * Add string alternatives for setting options over the rc (Nick Craig-Wood)
        * Add `options/local` to see the options configured in the context (Nick Craig-Wood)
        * Add `_config` parameter to set global config for just this rc call (Nick Craig-Wood)
        * Implement passing filter config with `_filter` parameter (Nick Craig-Wood)
        * Add `fscache/clear` and `fscache/entries` to control the fs cache (Nick Craig-Wood)
        * Avoid +Inf value for speed in `core/stats` (albertony)
        * Add a full set of stats to `core/stats` (Nick Craig-Wood)
        * Allow `fs=` params to be a JSON blob (Nick Craig-Wood)
    * rcd: Added systemd notification during the `rclone rcd` command. (Naveen Honest Raj)
    * rmdirs: Make `--rmdirs` obey the filters (Nick Craig-Wood)
    * version: Show build tags and type of executable (Ivan Andreev)
* Bug Fixes
    * install.sh: make it fail on download errors (Ivan Andreev)
    * Fix excessive retries missing `--max-duration` timeout (Nick Craig-Wood)
    * Fix crash when `--low-level-retries=0` (Nick Craig-Wood)
    * Fix failed token refresh on mounts created via the rc (Nick Craig-Wood)
    * fshttp: Fix bandwidth limiting after bad merge (Nick Craig-Wood)
    * lib/atexit
        * Unregister interrupt handler once it has fired so users can interrupt again (Nick Craig-Wood)
        * Fix occasional failure to unmount with CTRL-C (Nick Craig-Wood)
        * Fix deadlock calling Finalise while Run is running (Nick Craig-Wood)
    * lib/rest: Fix multipart uploads not stopping on context cancel (Nick Craig-Wood)
* Mount
    * Allow mounting to root directory on windows (albertony)
    * Improved handling of relative paths on windows (albertony)
    * Fix unicode issues with accented characters on macOS (Nick Craig-Wood)
    * Docs: document the new FileSecurity option in WinFsp 2021 (albertony)
    * Docs: add note about volume path syntax on windows (albertony)
    * Fix caching of old directories after renaming them (Nick Craig-Wood)
    * Update cgofuse to the latest version to bring in macfuse 4 fix (Nick Craig-Wood)
* VFS
    * `--vfs-used-is-size` to report used space using recursive scan (tYYGH)
    * Don't set modification time if it was already correct (Nick Craig-Wood)
    * Fix Create causing windows explorer to truncate files on CTRL-C CTRL-V (Nick Craig-Wood)
    * Fix modtimes not updating when writing via cache (Nick Craig-Wood)
    * Fix modtimes changing by fractional seconds after upload (Nick Craig-Wood)
    * Fix modtime set if `--vfs-cache-mode writes`/`full` and no write (Nick Craig-Wood)
    * Rename files in cache and cancel uploads on directory rename (Nick Craig-Wood)
    * Fix directory renaming by renaming dirs cached in memory (Nick Craig-Wood)
* Local
    * Add flag `--local-no-preallocate` (David Sze)
    * Make `nounc` an advanced option except on Windows (albertony)
    * Don't ignore preallocate disk full errors (Nick Craig-Wood)
* Cache
    * Add `--fs-cache-expire-duration` to control the fs cache (Nick Craig-Wood)
* Crypt
    * Add option to not encrypt data (Vesnyx)
    * Log hash ok on upload (albertony)
* Azure Blob
    * Add container public access level support. (Manish Kumar)
* B2
    * Fix HTML files downloaded via cloudflare (Nick Craig-Wood)
* Box
    * Fix transfers getting stuck on token expiry after API change (Nick Craig-Wood)
* Chunker
    * Partially implement no-rename transactions (Maxwell Calman)
* Drive
    * Don't stop server side copy if couldn't read description (Nick Craig-Wood)
    * Pass context on to drive SDK - to help with cancellation (Nick Craig-Wood)
* Dropbox
    * Add polling for changes support (Robert Thomas)
    * Make `--timeout 0` work properly (Nick Craig-Wood)
    * Raise priority of rate limited message to INFO to make it more noticeable (Nick Craig-Wood)
* Fichier
    * Implement copy & move (buengese)
    * Implement public link (buengese)
* FTP
    * Implement Shutdown method (Nick Craig-Wood)
    * Close idle connections after `--ftp-idle-timeout` (1m by default) (Nick Craig-Wood)
    * Make `--timeout 0` work properly (Nick Craig-Wood)
    * Add `--ftp-close-timeout` flag for use with awkward ftp servers (Nick Craig-Wood)
    * Retry connections and logins on 421 errors (Nick Craig-Wood)
* Hdfs
    * Fix permissions for when directory is created (Lucas Messenger)
* Onedrive
    * Make `--timeout 0` work properly (Nick Craig-Wood)
* S3
    * Fix `--s3-profile` which wasn't working (Nick Craig-Wood)
* SFTP
    * Close idle connections after `--sftp-idle-timeout` (1m by default) (Nick Craig-Wood)
    * Fix "file not found" errors for read once servers (Nick Craig-Wood)
    * Fix SetModTime stat failed: object not found with `--sftp-set-modtime=false` (Nick Craig-Wood)
* Swift
    * Update github.com/ncw/swift to v2.0.0 (Nick Craig-Wood)
    * Implement copying large objects (nguyenhuuluan434)
* Union
    * Fix crash when using epff policy (Nick Craig-Wood)
    * Fix union attempting to update files on a read only file system (Nick Craig-Wood)
    * Refactor to use fspath.SplitFs instead of fs.ParseRemote (Nick Craig-Wood)
    * Fix initialisation broken in refactor (Nick Craig-Wood)
* WebDAV
    * Add support for sharepoint with NTLM authentication (Rauno Ots)
    * Make sharepoint-ntlm docs more consistent (Alex Chen)
    * Improve terminology in sharepoint-ntlm docs (Ivan Andreev)
    * Disable HTTP/2 for NTLM authentication (georne)
    * Fix sharepoint-ntlm error 401 for parallel actions (Ivan Andreev)
    * Check that purged directory really exists (Ivan Andreev)
* Yandex
    * Make `--timeout 0` work properly (Nick Craig-Wood)
* Zoho
    * Replace client id - you will need to `rclone config reconnect` after this (buengese)
    * Add forgotten setupRegion() to NewFs - this finally fixes regions other than EU (buengese)

## v1.54.1 - 2021-03-08

[See commits](https://github.com/rclone/rclone/compare/v1.54.0...v1.54.1)

* Bug Fixes
    * accounting: Fix --bwlimit when up or down is off (Nick Craig-Wood)
    * docs
        * Fix nesting of brackets and backticks in ftp docs (edwardxml)
        * Fix broken link in sftp page (edwardxml)
        * Fix typo in crypt.md (Romeo Kienzler)
        * Changelog: Correct link to digitalis.io (Alex JOST)
        * Replace #file-caching with #vfs-file-caching (Miron Veryanskiy)
        * Convert bogus example link to code (edwardxml)
        * Remove dead link from rc.md (edwardxml)
    * rc: Sync,copy,move: document createEmptySrcDirs parameter (Nick Craig-Wood)
    * lsjson: Fix unterminated JSON in the presence of errors (Nick Craig-Wood)
* Mount
    * Fix mount dropping on macOS by setting --daemon-timeout 10m (Nick Craig-Wood)
* VFS
    * Document simultaneous usage with the same cache shouldn't be used (Nick Craig-Wood)
* B2
    * Automatically raise upload cutoff to avoid spurious error (Nick Craig-Wood)
    * Fix failed to create file system with application key limited to a prefix (Nick Craig-Wood)
* Drive
    * Refer to Shared Drives instead of Team Drives (Nick Craig-Wood)
* Dropbox
    * Add scopes to oauth request and optionally "members.read" (Nick Craig-Wood)
* S3
    * Fix failed to create file system with folder level permissions policy (Nick Craig-Wood)
    * Fix Wasabi HEAD requests returning stale data by using only 1 transport (Nick Craig-Wood)
    * Fix shared_credentials_file auth (Dmitry Chepurovskiy)
    * Add --s3-no-head to reducing costs docs (Nick Craig-Wood)
* Union
    * Fix mkdir at root with remote:/ (Nick Craig-Wood)
* Zoho
    * Fix custom client id's (buengese)

## v1.54.0 - 2021-02-02

[See commits](https://github.com/rclone/rclone/compare/v1.53.0...v1.54.0)

* New backends
    * Compression remote (experimental) (buengese)
    * Enterprise File Fabric (Nick Craig-Wood)
        * This work was sponsored by [Storage Made Easy](https://storagemadeeasy.com/)
    * HDFS (Hadoop Distributed File System) (Yury Stankevich)
    * Zoho workdrive (buengese)
* New Features
    * Deglobalise the config (Nick Craig-Wood)
        * Global config now read from the context
        * This will enable passing of global config via the rc
        * This work was sponsored by [Digitalis](https://digitalis.io/)
    * Add `--bwlimit` for upload and download (Nick Craig-Wood)
        * Obey bwlimit in http Transport for better limiting
    * Enhance systemd integration (Hekmon)
        * log level identification, manual activation with flag, automatic systemd launch detection
        * Don't compile systemd log integration for non unix systems (Benjamin Gustin)
    * Add a `--download` flag to md5sum/sha1sum/hashsum to force rclone to download and hash files locally (lostheli)
    * Add `--progress-terminal-title` to print ETA to terminal title (LaSombra)
    * Make backend env vars show in help as the defaults for backend flags (Nick Craig-Wood)
    * build
        * Raise minimum go version to go1.12 (Nick Craig-Wood)
    * dedupe
        * Add `--by-hash` to dedupe on content hash not file name (Nick Craig-Wood)
        * Add `--dedupe-mode list` to just list dupes, changing nothing (Nick Craig-Wood)
        * Add warning if used on a remote which can't have duplicate names (Nick Craig-Wood)
    * fs
        * Add Shutdown optional method for backends (Nick Craig-Wood)
        * When using `--files-from` check files concurrently (zhucan)
        * Accumulate stats when using `--dry-run` (Ingo Weiss)
        * Always show stats when using `--dry-run` or `--interactive` (Nick Craig-Wood)
        * Add support for flag `--no-console` on windows to hide the console window (albertony)
    * genautocomplete: Add support to output to stdout (Ingo)
    * ncdu
        * Highlight read errors instead of aborting (Claudio Bantaloukas)
        * Add sort by average size in directory (Adam Plánský)
        * Add toggle option for average s3ize in directory - key 'a' (Adam Plánský)
        * Add empty folder flag into ncdu browser (Adam Plánský)
        * Add `!` (error) and `.` (unreadable) file flags to go with `e` (empty) (Nick Craig-Wood)
    * obscure: Make `rclone obscure -` ignore newline at end of line (Nick Craig-Wood)
    * operations
        * Add logs when need to upload files to set mod times (Nick Craig-Wood)
        * Move and copy log name of the destination object in verbose (Adam Plánský)
        * Add size if known to skipped items and JSON log (Nick Craig-Wood)
    * rc
        * Prefer actual listener address if using ":port" or "addr:0" only (Nick Craig-Wood)
        * Add listener for finished jobs (Aleksandar Jankovic)
    * serve ftp: Add options to enable TLS (Deepak Sah)
    * serve http/webdav: Redirect requests to the base url without the / (Nick Craig-Wood)
    * serve restic: Implement object cache (Nick Craig-Wood)
    * stats: Add counter for deleted directories (Nick Craig-Wood)
    * sync: Only print "There was nothing to transfer" if no errors (Nick Craig-Wood)
    * webui
        * Prompt user for updating webui if an update is available (Chaitanya Bankanhal)
        * Fix plugins initialization (negative0)
* Bug Fixes
    * fs
        * Fix nil pointer on copy & move operations directly to remote (Anagh Kumar Baranwal)
        * Fix parsing of .. when joining remotes (Nick Craig-Wood)
    * log: Fix enabling systemd logging when using `--log-file` (Nick Craig-Wood)
    * check
        * Make the error count match up in the log message (Nick Craig-Wood)
    * move: Fix data loss when source and destination are the same object (Nick Craig-Wood)
    * operations
        * Fix `--cutoff-mode` hard not cutting off immediately (Nick Craig-Wood)
        * Fix `--immutable` error message (Nick Craig-Wood)
    * sync
        * Fix `--cutoff-mode` soft & cautious so it doesn't end the transfer early (Nick Craig-Wood)
        * Fix `--immutable` errors retrying many times (Nick Craig-Wood)
* Docs
    * Many fixes and a rewrite of the filtering docs (edwardxml)
    * Many spelling and grammar fixes (Josh Soref)
    * Doc fixes for commands delete, purge, rmdir, rmdirs and mount (albertony)
    * And thanks to these people for many doc fixes too numerous to list
        * Ameer Dawood, Antoine GIRARD, Bob Bagwill, Christopher Stewart
        * CokeMine, David, Dov Murik, Durval Menezes, Evan Harris, gtorelly
        * Ilyess Bachiri, Janne Johansson, Kerry Su, Marcin Zelent,
        * Martin Michlmayr, Milly, Sơn Trần-Nguyễn
* Mount
    * Update systemd status with cache stats (Hekmon)
    * Disable bazil/fuse based mount on macOS (Nick Craig-Wood)
        * Make `rclone mount` actually run `rclone cmount` under macOS (Nick Craig-Wood)
    * Implement mknod to make NFS file creation work (Nick Craig-Wood)
    * Make sure we don't call umount more than once (Nick Craig-Wood)
    * More user friendly mounting as network drive on windows (albertony)
    * Detect if uid or gid are set in same option string: -o uid=123,gid=456 (albertony)
    * Don't attempt to unmount if fs has been destroyed already (Nick Craig-Wood)
* VFS
    * Fix virtual entries causing deleted files to still appear (Nick Craig-Wood)
    * Fix "file already exists" error for stale cache files (Nick Craig-Wood)
    * Fix file leaks with `--vfs-cache-mode` full and `--buffer-size 0` (Nick Craig-Wood)
    * Fix invalid cache path on windows when using :backend: as remote (albertony)
* Local
    * Continue listing files/folders when a circular symlink is detected (Manish Gupta)
    * New flag `--local-zero-size-links` to fix sync on some virtual filesystems (Riccardo Iaconelli)
* Azure Blob
    * Add support for service principals (James Lim)
    * Add support for managed identities (Brad Ackerman)
    * Add examples for access tier (Bob Pusateri)
    * Utilize the streaming capabilities from the SDK for multipart uploads (Denis Neuling)
    * Fix setting of mime types (Nick Craig-Wood)
    * Fix crash when listing outside a SAS URL's root (Nick Craig-Wood)
    * Delete archive tier blobs before update if `--azureblob-archive-tier-delete` (Nick Craig-Wood)
    * Fix crash on startup (Nick Craig-Wood)
    * Fix memory usage by upgrading the SDK to v0.13.0 and implementing a TransferManager (Nick Craig-Wood)
    * Require go1.14+ to compile due to SDK changes (Nick Craig-Wood)
* B2
    * Make NewObject use less expensive API calls (Nick Craig-Wood)
        * This will improve `--files-from` and `restic serve` in particular
    * Fixed crash on an empty file name (lluuaapp)
* Box
    * Fix NewObject for files that differ in case (Nick Craig-Wood)
    * Fix finding directories in a case insensitive way (Nick Craig-Wood)
* Chunker
    * Skip long local hashing, hash in-transit (fixes) (Ivan Andreev)
    * Set Features ReadMimeType to false as Object.MimeType not supported (Nick Craig-Wood)
    * Fix case-insensitive NewObject, test metadata detection (Ivan Andreev)
* Drive
    * Implement `rclone backend copyid` command for copying files by ID (Nick Craig-Wood)
    * Added flag `--drive-stop-on-download-limit` to stop transfers when the download limit is exceeded (Anagh Kumar Baranwal)
    * Implement CleanUp workaround for team drives (buengese)
    * Allow shortcut resolution and creation to be retried (Nick Craig-Wood)
    * Log that emptying the trash can take some time (Nick Craig-Wood)
    * Add xdg office icons to xdg desktop files (Pau Rodriguez-Estivill)
* Dropbox
    * Add support for viewing shared files and folders (buengese)
    * Enable short lived access tokens (Nick Craig-Wood)
    * Implement IDer on Objects so `rclone lsf` etc can read the IDs (buengese)
    * Set Features ReadMimeType to false as Object.MimeType not supported (Nick Craig-Wood)
    * Make malformed_path errors from too long files not retriable (Nick Craig-Wood)
    * Test file name length before upload to fix upload loop (Nick Craig-Wood)
* Fichier
    * Set Features ReadMimeType to true as Object.MimeType is supported (Nick Craig-Wood)
* FTP
    * Add `--ftp-disable-msld` option to ignore MLSD for really old servers (Nick Craig-Wood)
    * Make `--tpslimit apply` (Nick Craig-Wood)
* Google Cloud Storage
    * Storage class object header support (Laurens Janssen)
    * Fix anonymous client to use rclone's HTTP client (Nick Craig-Wood)
    * Fix `Entry doesn't belong in directory "" (same as directory) - ignoring` (Nick Craig-Wood)
* Googlephotos
    * New flag `--gphotos-include-archived` to show archived photos as well (Nicolas Rueff)
* Jottacloud
    * Don't erroneously report support for writing mime types (buengese)
    * Add support for Telia Cloud (Patrik Nordlén)
* Mailru
    * Accept special folders eg camera-upload (Ivan Andreev)
    * Avoid prehashing of large local files (Ivan Andreev)
    * Fix uploads after recent changes on server (Ivan Andreev)
    * Fix range requests after June 2020 changes on server (Ivan Andreev)
    * Fix invalid timestamp on corrupted files (fixes) (Ivan Andreev)
    * Remove deprecated protocol quirks (Ivan Andreev)
* Memory
    * Fix setting of mime types (Nick Craig-Wood)
* Onedrive
    * Add support for China region operated by 21vianet and other regional suppliers (NyaMisty)
    * Warn on gateway timeout errors (Nick Craig-Wood)
    * Fall back to normal copy if server-side copy unavailable (Alex Chen)
    * Fix server-side copy completely disabled on OneDrive for Business (Cnly)
    * (business only) workaround to replace existing file on server-side copy (Alex Chen)
    * Enhance link creation with expiry, scope, type and password (Nick Craig-Wood)
    * Remove % and # from the set of encoded characters (Alex Chen)
    * Support addressing site by server-relative URL (kice)
* Opendrive
    * Fix finding directories in a case insensitive way (Nick Craig-Wood)
* Pcloud
    * Fix setting of mime types (Nick Craig-Wood)
* Premiumizeme
    * Fix finding directories in a case insensitive way (Nick Craig-Wood)
* Qingstor
    * Fix error propagation in CleanUp (Nick Craig-Wood)
    * Fix rclone cleanup (Nick Craig-Wood)
* S3
    * Added `--s3-disable-http2` to disable http/2 (Anagh Kumar Baranwal)
    * Complete SSE-C implementation (Nick Craig-Wood)
        * Fix hashes on small files with AWS:KMS and SSE-C (Nick Craig-Wood)
        * Add MD5 metadata to objects uploaded with SSE-AWS/SSE-C (Nick Craig-Wood)
    * Add `--s3-no-head parameter` to minimise transactions on upload (Nick Craig-Wood)
    * Update docs with a Reducing Costs section (Nick Craig-Wood)
    * Added error handling for error code 429 indicating too many requests (Anagh Kumar Baranwal)
    * Add requester pays option (kelv)
    * Fix copy multipart with v2 auth failing with 'SignatureDoesNotMatch' (Louis Koo)
* SFTP
    * Allow cert based auth via optional pubkey (Stephen Harris)
    * Allow user to optionally check server hosts key to add security (Stephen Harris)
    * Defer asking for user passwords until the SSH connection succeeds (Stephen Harris)
    * Remember entered password in AskPass mode (Stephen Harris)
    * Implement Shutdown method (Nick Craig-Wood)
    * Implement keyboard interactive authentication (Nick Craig-Wood)
    * Make `--tpslimit` apply (Nick Craig-Wood)
    * Implement `--sftp-use-fstat` for unusual SFTP servers (Nick Craig-Wood)
* Sugarsync
    * Fix NewObject for files that differ in case (Nick Craig-Wood)
    * Fix finding directories in a case insensitive way (Nick Craig-Wood)
* Swift
    * Fix deletion of parts of Static Large Object (SLO) (Nguyễn Hữu Luân)
    * Ensure partially uploaded large files are uploaded unless `--swift-leave-parts-on-error` (Nguyễn Hữu Luân)
* Tardigrade
    * Upgrade to uplink v1.4.1 (Caleb Case)
* WebDAV
    * Updated docs to show streaming to nextcloud is working (Durval Menezes)
* Yandex
    * Set Features WriteMimeType to false as Yandex ignores mime types (Nick Craig-Wood)

## v1.53.4 - 2021-01-20

[See commits](https://github.com/rclone/rclone/compare/v1.53.3...v1.53.4)

* Bug Fixes
    * accounting: Fix data race in Transferred() (Maciej Zimnoch)
    * build
        * Stop tagged releases making a current beta (Nick Craig-Wood)
        * Upgrade docker buildx action (Matteo Pietro Dazzi)
        * Add -buildmode to cross-compile.go (Nick Craig-Wood)
        * Fix docker build by upgrading ilteoood/docker_buildx (Nick Craig-Wood)
        * Revert GitHub actions brew fix since this is now fixed (Nick Craig-Wood)
        * Fix brew install --cask syntax for macOS build (Nick Craig-Wood)
        * Update nfpm syntax to fix build of .deb/.rpm packages (Nick Craig-Wood)
        * Fix for Windows build errors (Ivan Andreev)
    * fs: Parseduration: fixed tests to use UTC time (Ankur Gupta)
    * fshttp: Prevent overlap of HTTP headers in logs (Nathan Collins)
    * rc
        * Fix core/command giving 500 internal error (Nick Craig-Wood)
        * Add Copy method to rc.Params (Nick Craig-Wood)
        * Fix 500 error when marshalling errors from core/command (Nick Craig-Wood)
        * plugins: Create plugins files only if webui is enabled. (negative0)
    * serve http: Fix serving files of unknown length (Nick Craig-Wood)
    * serve sftp: Fix authentication on one connection blocking others (Nick Craig-Wood)
* Mount
    * Add optional `brew` tag to throw an error when using mount in the binaries installed via Homebrew (Anagh Kumar Baranwal)
    * Add "." and ".." to directories to match cmount and expectations (Nick Craig-Wood)
* VFS
    * Make cache dir absolute before using it to fix path too long errors (Nick Craig-Wood)
* Chunker
    * Improve detection of incompatible metadata (Ivan Andreev)
* Google Cloud Storage
    * Fix server side copy of large objects (Nick Craig-Wood)
* Jottacloud
    * Fix token renewer to fix long uploads (Nick Craig-Wood)
    * Fix token refresh failed: is not a regular file error (Nick Craig-Wood)
* Pcloud
    * Only use SHA1 hashes in EU region (Nick Craig-Wood)
* Sharefile
    * Undo Fix backend due to API swapping integers for strings (Nick Craig-Wood)
* WebDAV
    * Fix Open Range requests to fix 4shared mount (Nick Craig-Wood)
    * Add "Depth: 0" to GET requests to fix bitrix (Nick Craig-Wood)

## v1.53.3 - 2020-11-19

[See commits](https://github.com/rclone/rclone/compare/v1.53.2...v1.53.3)

* Bug Fixes
    * random: Fix incorrect use of math/rand instead of crypto/rand CVE-2020-28924 (Nick Craig-Wood)
        * Passwords you have generated with `rclone config` may be insecure
        * See [issue #4783](https://github.com/rclone/rclone/issues/4783) for more details and a checking tool
    * random: Seed math/rand in one place with crypto strong seed (Nick Craig-Wood)
* VFS
    * Fix vfs/refresh calls with fs= parameter (Nick Craig-Wood)
* Sharefile
    * Fix backend due to API swapping integers for strings (Nick Craig-Wood)

## v1.53.2 - 2020-10-26

[See commits](https://github.com/rclone/rclone/compare/v1.53.1...v1.53.2)

* Bug Fixes
    * accounting
        * Fix incorrect speed and transferTime in core/stats (Nick Craig-Wood)
        * Stabilize display order of transfers on Windows (Nick Craig-Wood)
    * operations
        * Fix use of --suffix without --backup-dir (Nick Craig-Wood)
        * Fix spurious "--checksum is in use but the source and destination have no hashes in common" (Nick Craig-Wood)
    * build
        * Work around GitHub actions brew problem (Nick Craig-Wood)
        * Stop using set-env and set-path in the GitHub actions (Nick Craig-Wood)
* Mount
    * mount2: Fix the swapped UID / GID values (Russell Cattelan)
* VFS
    * Detect and recover from a file being removed externally from the cache (Nick Craig-Wood)
    * Fix a deadlock vulnerability in downloaders.Close (Leo Luan)
    * Fix a race condition in retryFailedResets (Leo Luan)
    * Fix missed concurrency control between some item operations and reset (Leo Luan)
    * Add exponential backoff during ENOSPC retries (Leo Luan)
    * Add a missed update of used cache space (Leo Luan)
    * Fix --no-modtime to not attempt to set modtimes (as documented) (Nick Craig-Wood)
* Local
    * Fix sizes and syncing with --links option on Windows (Nick Craig-Wood)
* Chunker
    * Disable ListR to fix missing files on GDrive (workaround) (Ivan Andreev)
    * Fix upload over crypt (Ivan Andreev)
* Fichier
    * Increase maximum file size from 100GB to 300GB (gyutw)
* Jottacloud
    * Remove clientSecret from config when upgrading to token based authentication (buengese)
    * Avoid double url escaping of device/mountpoint (albertony)
    * Remove DirMove workaround as it's not required anymore - also (buengese)
* Mailru
    * Fix uploads after recent changes on server (Ivan Andreev)
    * Fix range requests after june changes on server (Ivan Andreev)
    * Fix invalid timestamp on corrupted files (fixes) (Ivan Andreev)
* Onedrive
    * Fix disk usage for sharepoint (Nick Craig-Wood)
* S3
    * Add missing regions for AWS (Anagh Kumar Baranwal)
* Seafile
    * Fix accessing libraries > 2GB on 32 bit systems (Muffin King)
* SFTP
    * Always convert the checksum to lower case (buengese)
* Union
    * Create root directories if none exist (Nick Craig-Wood)

## v1.53.1 - 2020-09-13

[See commits](https://github.com/rclone/rclone/compare/v1.53.0...v1.53.1)

* Bug Fixes
    * accounting: Remove new line from end of --stats-one-line display (Nick Craig-Wood)
    * check
        * Add back missing --download flag (Nick Craig-Wood)
        * Fix docs (Nick Craig-Wood)
    * docs
        * Note --log-file does append (Nick Craig-Wood)
        * Add full stops for consistency in rclone --help (edwardxml)
        * Add Tencent COS to s3 provider list (wjielai)
        * Updated mount command to reflect that it requires Go 1.13 or newer (Evan Harris)
        * jottacloud: Mention that uploads from local disk will not need to cache files to disk for md5 calculation (albertony)
        * Fix formatting of rc docs page (Nick Craig-Wood)
    * build
        * Include vendor tar ball in release and fix startdev (Nick Craig-Wood)
        * Fix "Illegal instruction" error for ARMv6 builds (Nick Craig-Wood)
        * Fix architecture name in ARMv7 build (Nick Craig-Wood)
* VFS
    * Fix spurious error "vfs cache: failed to _ensure cache EOF" (Nick Craig-Wood)
    * Log an ERROR if we fail to set the file to be sparse (Nick Craig-Wood)
* Local
    * Log an ERROR if we fail to set the file to be sparse (Nick Craig-Wood)
* Drive
    * Re-adds special oauth help text (Tim Gallant)
* Opendrive
    * Do not retry 400 errors (Evan Harris)

## v1.53.0 - 2020-09-02

[See commits](https://github.com/rclone/rclone/compare/v1.52.0...v1.53.0)

* New Features
    * The [VFS layer](/commands/rclone_mount/#vfs-virtual-file-system) was heavily reworked for this release - see below for more details
    * Interactive mode [-i/--interactive](/docs/#interactive) for destructive operations (fishbullet)
    * Add [--bwlimit-file](/docs/#bwlimit-file-bandwidth-spec) flag to limit speeds of individual file transfers (Nick Craig-Wood)
    * Transfers are sorted by start time in the stats and progress output (Max Sum)
    * Make sure backends expand `~` and environment vars in file names they use (Nick Craig-Wood)
    * Add [--refresh-times](/docs/#refresh-times) flag to set modtimes on hashless backends (Nick Craig-Wood)
    * build
        * Remove vendor directory in favour of Go modules (Nick Craig-Wood)
        * Build with go1.15.x by default (Nick Craig-Wood)
        * Drop macOS 386 build as it is no longer supported by go1.15 (Nick Craig-Wood)
        * Add ARMv7 to the supported builds (Nick Craig-Wood)
        * Enable `rclone cmount` on macOS (Nick Craig-Wood)
        * Make rclone build with gccgo (Nick Craig-Wood)
        * Make rclone build with wasm (Nick Craig-Wood)
        * Change beta numbering to be semver compatible (Nick Craig-Wood)
        * Add file properties and icon to Windows executable (albertony)
        * Add experimental interface for integrating rclone into browsers (Nick Craig-Wood)
    * lib: Add file name compression (Klaus Post)
    * rc
        * Allow installation and use of plugins and test plugins with rclone-webui (Chaitanya Bankanhal)
        * Add reverse proxy pluginsHandler for serving plugins (Chaitanya Bankanhal)
        * Add `mount/listmounts` option for listing current mounts (Chaitanya Bankanhal)
        * Add `operations/uploadfile` to upload a file through rc using encoding multipart/form-data (Chaitanya Bankanhal)
        * Add `core/command` to execute rclone terminal commands. (Chaitanya Bankanhal)
    * `rclone check`
        * Add reporting of filenames for same/missing/changed (Nick Craig-Wood)
        * Make check command obey `--dry-run`/`-i`/`--interactive` (Nick Craig-Wood)
        * Make check do `--checkers` files concurrently (Nick Craig-Wood)
        * Retry downloads if they fail when using the `--download` flag (Nick Craig-Wood)
        * Make it show stats by default (Nick Craig-Wood)
    * `rclone obscure`: Allow obscure command to accept password on STDIN (David Ibarra)
    * `rclone config`
        * Set RCLONE_CONFIG_DIR for use in config files and subprocesses (Nick Craig-Wood)
        * Reject remote names starting with a dash. (jtagcat)
    * `rclone cryptcheck`: Add reporting of filenames for same/missing/changed (Nick Craig-Wood)
    * `rclone dedupe`: Make it obey the `--size-only` flag for duplicate detection (Nick Craig-Wood)
    * `rclone link`: Add `--expire` and `--unlink` flags (Roman Kredentser)
    * `rclone mkdir`: Warn when using mkdir on remotes which can't have empty directories (Nick Craig-Wood)
    * `rclone rc`: Allow JSON parameters to simplify command line usage (Nick Craig-Wood)
    * `rclone serve ftp`
        * Don't compile on < go1.13 after dependency update (Nick Craig-Wood)
        * Add error message if auth proxy fails (Nick Craig-Wood)
        * Use refactored goftp.io/server library for binary shrink (Nick Craig-Wood)
    * `rclone serve restic`: Expose interfaces so that rclone can be used as a library from within restic (Jack)
    * `rclone sync`: Add `--track-renames-strategy leaf` (Nick Craig-Wood)
    * `rclone touch`: Add ability to set nanosecond resolution times (Nick Craig-Wood)
    * `rclone tree`: Remove `-i` shorthand for `--noindent` as it conflicts with `-i`/`--interactive` (Nick Craig-Wood)
* Bug Fixes
    * accounting
        * Fix documentation for `speed`/`speedAvg` (Nick Craig-Wood)
        * Fix elapsed time not show actual time since beginning (Chaitanya Bankanhal)
        * Fix deadlock in stats printing (Nick Craig-Wood)
    * build
        * Fix file handle leak in GitHub release tool (Garrett Squire)
    * `rclone check`: Fix successful retries with `--download` counting errors (Nick Craig-Wood)
    * `rclone dedupe`: Fix logging to be easier to understand (Nick Craig-Wood)
* Mount
    * Warn macOS users that mount implementation is changing (Nick Craig-Wood)
        * to test the new implementation use `rclone cmount` instead of `rclone mount`
        * this is because the library rclone uses has dropped macOS support
    * rc interface
        * Add call for unmount all (Chaitanya Bankanhal)
        * Make `mount/mount` remote control take vfsOpt option (Nick Craig-Wood)
        * Add mountOpt to `mount/mount` (Nick Craig-Wood)
        * Add VFS and Mount options to `mount/listmounts` (Nick Craig-Wood)
    * Catch panics in cgofuse initialization and turn into error messages (Nick Craig-Wood)
    * Always supply stat information in Readdir (Nick Craig-Wood)
    * Add support for reading unknown length files using direct IO (Windows) (Nick Craig-Wood)
    * Fix On Windows don't add `-o uid/gid=-1` if user supplies `-o uid/gid`. (Nick Craig-Wood)
    * Fix macOS losing directory contents in cmount (Nick Craig-Wood)
    * Fix volume name broken in recent refactor (Nick Craig-Wood)
* VFS
    * Implement partial reads for `--vfs-cache-mode full` (Nick Craig-Wood)
    * Add `--vfs-writeback` option to delay writes back to cloud storage (Nick Craig-Wood)
    * Add `--vfs-read-ahead` parameter for use with `--vfs-cache-mode full` (Nick Craig-Wood)
    * Restart pending uploads on restart of the cache (Nick Craig-Wood)
    * Support synchronous cache space recovery upon ENOSPC (Leo Luan)
    * Allow ReadAt and WriteAt to run concurrently with themselves (Nick Craig-Wood)
    * Change modtime of file before upload to current (Rob Calistri)
    * Recommend `--vfs-cache-modes writes` on backends which can't stream (Nick Craig-Wood)
    * Add an optional `fs` parameter to vfs rc methods (Nick Craig-Wood)
    * Fix errors when using > 260 char files in the cache in Windows (Nick Craig-Wood)
    * Fix renaming of items while they are being uploaded (Nick Craig-Wood)
    * Fix very high load caused by slow directory listings (Nick Craig-Wood)
    * Fix renamed files not being uploaded with `--vfs-cache-mode minimal` (Nick Craig-Wood)
    * Fix directory locking caused by slow directory listings (Nick Craig-Wood)
    * Fix saving from chrome without `--vfs-cache-mode writes` (Nick Craig-Wood)
* Local
    * Add `--local-no-updated` to provide a consistent view of changing objects (Nick Craig-Wood)
    * Add `--local-no-set-modtime` option to prevent modtime changes (tyhuber1)
    * Fix race conditions updating and reading Object metadata (Nick Craig-Wood)
* Cache
    * Make any created backends be cached to fix rc problems (Nick Craig-Wood)
    * Fix dedupe on caches wrapping drives (Nick Craig-Wood)
* Crypt
    * Add `--crypt-server-side-across-configs` flag (Nick Craig-Wood)
    * Make any created backends be cached to fix rc problems (Nick Craig-Wood)
* Alias
    * Make any created backends be cached to fix rc problems (Nick Craig-Wood)
* Azure Blob
    * Don't compile on < go1.13 after dependency update (Nick Craig-Wood)
* B2
    * Implement server-side copy for files > 5GB (Nick Craig-Wood)
    * Cancel in progress multipart uploads and copies on rclone exit (Nick Craig-Wood)
    * Note that b2's encoding now allows \ but rclone's hasn't changed (Nick Craig-Wood)
    * Fix transfers when using download_url (Nick Craig-Wood)
* Box
    * Implement rclone cleanup (buengese)
    * Cancel in progress multipart uploads and copies on rclone exit (Nick Craig-Wood)
    * Allow authentication with access token (David)
* Chunker
    * Make any created backends be cached to fix rc problems (Nick Craig-Wood)
* Drive
    * Add `rclone backend drives` to list shared drives (teamdrives) (Nick Craig-Wood)
    * Implement `rclone backend untrash` (Nick Craig-Wood)
    * Work around drive bug which didn't set modtime of copied docs (Nick Craig-Wood)
    * Added `--drive-starred-only` to only show starred files (Jay McEntire)
    * Deprecate `--drive-alternate-export` as it is no longer needed (themylogin)
    * Fix duplication of Google docs on server-side copy (Nick Craig-Wood)
    * Fix "panic: send on closed channel" when recycling dir entries (Nick Craig-Wood)
* Dropbox
    * Add copyright detector info in limitations section in the docs (Alex Guerrero)
    * Fix `rclone link` by removing expires parameter (Nick Craig-Wood)
* Fichier
    * Detect Flood detected: IP Locked error and sleep for 30s (Nick Craig-Wood)
* FTP
    * Add explicit TLS support (Heiko Bornholdt)
    * Add support for `--dump bodies` and `--dump auth` for debugging (Nick Craig-Wood)
    * Fix interoperation with pure-ftpd (Nick Craig-Wood)
* Google Cloud Storage
    * Add support for anonymous access (Kai Lüke)
* Jottacloud
    * Bring back legacy authentication for use with whitelabel versions (buengese)
    * Switch to new api root - also implement a very ugly workaround for the DirMove failures (buengese)
* Onedrive
    * Rework cancel of multipart uploads on rclone exit (Nick Craig-Wood)
    * Implement rclone cleanup (Nick Craig-Wood)
    * Add `--onedrive-no-versions` flag to remove old versions (Nick Craig-Wood)
* Pcloud
    * Implement `rclone link` for public link creation (buengese)
* Qingstor
    * Cancel in progress multipart uploads on rclone exit (Nick Craig-Wood)
* S3
    * Preserve metadata when doing multipart copy (Nick Craig-Wood)
    * Cancel in progress multipart uploads and copies on rclone exit (Nick Craig-Wood)
    * Add `rclone link` for public link sharing (Roman Kredentser)
    * Add `rclone backend restore` command to restore objects from GLACIER (Nick Craig-Wood)
    * Add `rclone cleanup` and `rclone backend cleanup` to clean unfinished multipart uploads (Nick Craig-Wood)
    * Add `rclone backend list-multipart-uploads` to list unfinished multipart uploads (Nick Craig-Wood)
    * Add `--s3-max-upload-parts` support (Kamil Trzciński)
    * Add `--s3-no-check-bucket` for minimising rclone transactions and perms (Nick Craig-Wood)
    * Add `--s3-profile` and `--s3-shared-credentials-file` options (Nick Craig-Wood)
    * Use regional s3 us-east-1 endpoint (David)
    * Add Scaleway provider (Vincent Feltz)
    * Update IBM COS endpoints (Egor Margineanu)
    * Reduce the default `--s3-copy-cutoff` to < 5GB for Backblaze S3 compatibility (Nick Craig-Wood)
    * Fix detection of bucket existing (Nick Craig-Wood)
* SFTP
    * Use the absolute path instead of the relative path for listing for improved compatibility (Nick Craig-Wood)
    * Add `--sftp-subsystem` and `--sftp-server-command` options (aus)
* Swift
    * Fix dangling large objects breaking the listing (Nick Craig-Wood)
    * Fix purge not deleting directory markers (Nick Craig-Wood)
    * Fix update multipart object removing all of its own parts (Nick Craig-Wood)
    * Fix missing hash from object returned from upload (Nick Craig-Wood)
* Tardigrade
    * Upgrade to uplink v1.2.0 (Kaloyan Raev)
* Union
    * Fix writing with the all policy (Nick Craig-Wood)
* WebDAV
    * Fix directory creation with 4shared (Nick Craig-Wood)

## v1.52.3 - 2020-08-07

[See commits](https://github.com/rclone/rclone/compare/v1.52.2...v1.52.3)

* Bug Fixes
    * docs
        * Disable smart typography (e.g. en-dash) in MANUAL.* and man page (Nick Craig-Wood)
        * Update install.md to reflect minimum Go version (Evan Harris)
        * Update install from source instructions (Nick Craig-Wood)
        * make_manual: Support SOURCE_DATE_EPOCH (Morten Linderud)
    * log: Fix --use-json-log going to stderr not --log-file on Windows (Nick Craig-Wood)
    * serve dlna: Fix file list on Samsung Series 6+ TVs (Matteo Pietro Dazzi)
    * sync: Fix deadlock with --track-renames-strategy modtime (Nick Craig-Wood)
* Cache
    * Fix moveto/copyto remote:file remote:file2 (Nick Craig-Wood)
* Drive
    * Stop using root_folder_id as a cache (Nick Craig-Wood)
    * Make dangling shortcuts appear in listings (Nick Craig-Wood)
    * Drop "Disabling ListR" messages down to debug (Nick Craig-Wood)
    * Workaround and policy for Google Drive API (Dmitry Ustalov)
* FTP
    * Add note to docs about home vs root directory selection (Nick Craig-Wood)
* Onedrive
    * Fix reverting to Copy when Move would have worked (Nick Craig-Wood)
    * Avoid comma rendered in URL in onedrive.md (Kevin)
* Pcloud
    * Fix oauth on European region "eapi.pcloud.com" (Nick Craig-Wood)
* S3
    * Fix bucket Region auto detection when Region unset in config (Nick Craig-Wood)

## v1.52.2 - 2020-06-24

[See commits](https://github.com/rclone/rclone/compare/v1.52.1...v1.52.2)

* Bug Fixes
    * build
        * Fix docker release build action (Nick Craig-Wood)
        * Fix custom timezone in Docker image (NoLooseEnds)
    * check: Fix misleading message which printed errors instead of differences (Nick Craig-Wood)
    * errors: Add WSAECONNREFUSED and more to the list of retriable Windows errors (Nick Craig-Wood)
    * rcd: Fix incorrect prometheus metrics (Gary Kim)
    * serve restic: Fix flags so they use environment variables (Nick Craig-Wood)
    * serve webdav: Fix flags so they use environment variables (Nick Craig-Wood)
    * sync: Fix --track-renames-strategy modtime (Nick Craig-Wood)
* Drive
    * Fix not being able to delete a directory with a trashed shortcut (Nick Craig-Wood)
    * Fix creating a directory inside a shortcut (Nick Craig-Wood)
    * Fix --drive-impersonate with cached root_folder_id (Nick Craig-Wood)
* SFTP
    * Fix SSH key PEM loading (Zac Rubin)
* Swift
    * Speed up deletes by not retrying segment container deletes (Nick Craig-Wood)
* Tardigrade
    * Upgrade to uplink v1.1.1 (Caleb Case)
* WebDAV
    * Fix free/used display for rclone about/df for certain backends (Nick Craig-Wood)

## v1.52.1 - 2020-06-10

[See commits](https://github.com/rclone/rclone/compare/v1.52.0...v1.52.1)

* Bug Fixes
    * lib/file: Fix SetSparse on Windows 7 which fixes downloads of files > 250MB (Nick Craig-Wood)
    * build
        * Update go.mod to go1.14 to enable -mod=vendor build (Nick Craig-Wood)
        * Remove quicktest from Dockerfile (Nick Craig-Wood)
        * Build Docker images with GitHub actions (Matteo Pietro Dazzi)
        * Update Docker build workflows (Nick Craig-Wood)
        * Set user_allow_other in /etc/fuse.conf in the Docker image (Nick Craig-Wood)
        * Fix xgo build after go1.14 go.mod update (Nick Craig-Wood)
    * docs
        * Add link to source and modified time to footer of every page (Nick Craig-Wood)
        * Remove manually set dates and use git dates instead (Nick Craig-Wood)
        * Minor tense, punctuation, brevity and positivity changes for the home page (edwardxml)
        * Remove leading slash in page reference in footer when present (Nick Craig-Wood)
        * Note commands which need obscured input in the docs (Nick Craig-Wood)
        * obscure: Write more help as we are referencing it elsewhere (Nick Craig-Wood)
* VFS
    * Fix OS vs Unix path confusion - fixes ChangeNotify on Windows (Nick Craig-Wood)
* Drive
    * Fix missing items when listing using --fast-list / ListR (Nick Craig-Wood)
* Putio
    * Fix panic on Object.Open (Cenk Alti)
* S3
    * Fix upload of single files into buckets without create permission (Nick Craig-Wood)
    * Fix --header-upload (Nick Craig-Wood)
* Tardigrade
    * Fix listing bug by upgrading to v1.0.7
    * Set UserAgent to rclone (Caleb Case)

## v1.52.0 - 2020-05-27

Special thanks to Martin Michlmayr for proof reading and correcting
all the docs and Edward Barker for helping re-write the front page.

[See commits](https://github.com/rclone/rclone/compare/v1.51.0...v1.52.0)

* New backends
    * [Tardigrade](/tardigrade/) backend for use with storj.io (Caleb Case)
    * [Union](/union/) re-write to have multiple writable remotes (Max Sum)
    * [Seafile](/seafile) for Seafile server (Fred @creativeprojects)
* New commands
    * backend: command for backend-specific commands (see backends) (Nick Craig-Wood)
    * cachestats: Deprecate in favour of `rclone backend stats cache:` (Nick Craig-Wood)
    * dbhashsum: Deprecate in favour of `rclone hashsum DropboxHash` (Nick Craig-Wood)
* New Features
    * Add `--header-download` and `--header-upload` flags for setting HTTP headers when uploading/downloading (Tim Gallant)
    * Add `--header` flag to add HTTP headers to every HTTP transaction (Nick Craig-Wood)
    * Add `--check-first` to do all checking before starting transfers (Nick Craig-Wood)
    * Add `--track-renames-strategy` for configurable matching criteria for `--track-renames` (Bernd Schoolmann)
    * Add `--cutoff-mode` hard,soft,cautious (Shing Kit Chan & Franklyn Tackitt)
    * Filter flags (e.g. `--files-from -`) can read from stdin (fishbullet)
    * Add `--error-on-no-transfer` option (Jon Fautley)
    * Implement `--order-by xxx,mixed` for copying some small and some big files (Nick Craig-Wood)
    * Allow `--max-backlog` to be negative meaning as large as possible (Nick Craig-Wood)
    * Added `--no-unicode-normalization` flag to allow Unicode filenames to remain unique (Ben Zenker)
    * Allow `--min-age`/`--max-age` to take a date as well as a duration (Nick Craig-Wood)
    * Add rename statistics for file and directory renames (Nick Craig-Wood)
    * Add statistics output to JSON log (reddi)
    * Make stats be printed on non-zero exit code (Nick Craig-Wood)
    * When running `--password-command` allow use of stdin (Sébastien Gross)
    * Stop empty strings being a valid remote path (Nick Craig-Wood)
    * accounting: support WriterTo for less memory copying (Nick Craig-Wood)
    * build
        * Update to use go1.14 for the build (Nick Craig-Wood)
        * Add `-trimpath` to release build for reproduceable builds (Nick Craig-Wood)
        * Remove GOOS and GOARCH from Dockerfile (Brandon Philips)
    * config
        * Fsync the config file after writing to save more reliably (Nick Craig-Wood)
        * Add `--obscure` and `--no-obscure` flags to `config create`/`update` (Nick Craig-Wood)
        * Make `config show` take `remote:` as well as `remote` (Nick Craig-Wood)
    * copyurl: Add `--no-clobber` flag (Denis)
    * delete: Added `--rmdirs` flag to delete directories as well (Kush)
    * filter: Added `--files-from-raw` flag (Ankur Gupta)
    * genautocomplete: Add support for fish shell (Matan Rosenberg)
    * log: Add support for syslog LOCAL facilities (Patryk Jakuszew)
    * lsjson: Add `--hash-type` parameter and use it in lsf to speed up hashing (Nick Craig-Wood)
    * rc
        * Add `-o`/`--opt` and `-a`/`--arg` for more structured input (Nick Craig-Wood)
        * Implement `backend/command` for running backend-specific commands remotely (Nick Craig-Wood)
        * Add `mount/mount` command for starting `rclone mount` via the API (Chaitanya)
    * rcd: Add Prometheus metrics support (Gary Kim)
    * serve http
        * Added a `--template` flag for user defined markup (calistri)
        * Add Last-Modified headers to files and directories (Nick Craig-Wood)
    * serve sftp: Add support for multiple host keys by repeating `--key` flag (Maxime Suret)
    * touch: Add `--localtime` flag to make `--timestamp` localtime not UTC (Nick Craig-Wood)
* Bug Fixes
    * accounting
        * Restore "Max number of stats groups reached" log line (Michał Matczuk)
        * Correct exitcode on Transfer Limit Exceeded flag. (Anuar Serdaliyev)
        * Reset bytes read during copy retry (Ankur Gupta)
        * Fix race clearing stats (Nick Craig-Wood)
    * copy: Only create empty directories when they don't exist on the remote (Ishuah Kariuki)
    * dedupe: Stop dedupe deleting files with identical IDs (Nick Craig-Wood)
    * oauth
        * Use custom http client so that `--no-check-certificate` is honored by oauth token fetch (Mark Spieth)
        * Replace deprecated oauth2.NoContext (Lars Lehtonen)
    * operations
        * Fix setting the timestamp on Windows for multithread copy (Nick Craig-Wood)
        * Make rcat obey `--ignore-checksum` (Nick Craig-Wood)
        * Make `--max-transfer` more accurate (Nick Craig-Wood)
    * rc
        * Fix dropped error (Lars Lehtonen)
        * Fix misplaced http server config (Xiaoxing Ye)
        * Disable duplicate log (ElonH)
    * serve dlna
        * Cds: don't specify childCount at all when unknown (Dan Walters)
        * Cds: use modification time as date in dlna metadata (Dan Walters)
    * serve restic: Fix tests after restic project removed vendoring (Nick Craig-Wood)
    * sync
        * Fix incorrect "nothing to transfer" message using `--delete-before` (Nick Craig-Wood)
        * Only create empty directories when they don't exist on the remote (Ishuah Kariuki)
* Mount
    * Add `--async-read` flag to disable asynchronous reads (Nick Craig-Wood)
    * Ignore `--allow-root` flag with a warning as it has been removed upstream (Nick Craig-Wood)
    * Warn if `--allow-non-empty` used on Windows and clarify docs (Nick Craig-Wood)
    * Constrain to go1.13 or above otherwise bazil.org/fuse fails to compile (Nick Craig-Wood)
    * Fix fail because of too long volume name (evileye)
    * Report 1PB free for unknown disk sizes (Nick Craig-Wood)
    * Map more rclone errors into file systems errors (Nick Craig-Wood)
    * Fix disappearing cwd problem (Nick Craig-Wood)
    * Use ReaddirPlus on Windows to improve directory listing performance (Nick Craig-Wood)
    * Send a hint as to whether the filesystem is case insensitive or not (Nick Craig-Wood)
    * Add rc command `mount/types` (Nick Craig-Wood)
    * Change maximum leaf name length to 1024 bytes (Nick Craig-Wood)
* VFS
    * Add `--vfs-read-wait` and `--vfs-write-wait` flags to control time waiting for a sequential read/write (Nick Craig-Wood)
    * Change default `--vfs-read-wait` to 20ms (it was 5ms and not configurable) (Nick Craig-Wood)
    * Make `df` output more consistent on a rclone mount. (Yves G)
    * Report 1PB free for unknown disk sizes (Nick Craig-Wood)
    * Fix race condition caused by unlocked reading of Dir.path (Nick Craig-Wood)
    * Make File lock and Dir lock not overlap to avoid deadlock (Nick Craig-Wood)
    * Implement lock ordering between File and Dir to eliminate deadlocks (Nick Craig-Wood)
    * Factor the vfs cache into its own package (Nick Craig-Wood)
    * Pin the Fs in use in the Fs cache (Nick Craig-Wood)
    * Add SetSys() methods to Node to allow caching stuff on a node (Nick Craig-Wood)
    * Ignore file not found errors from Hash in Read.Release (Nick Craig-Wood)
    * Fix hang in read wait code (Nick Craig-Wood)
* Local
    * Speed up multi thread downloads by using sparse files on Windows (Nick Craig-Wood)
    * Implement `--local-no-sparse` flag for disabling sparse files (Nick Craig-Wood)
    * Implement `rclone backend noop` for testing purposes (Nick Craig-Wood)
    * Fix "file not found" errors on post transfer Hash calculation (Nick Craig-Wood)
* Cache
    * Implement `rclone backend stats` command (Nick Craig-Wood)
    * Fix Server Side Copy with Temp Upload (Brandon McNama)
    * Remove Unused Functions (Lars Lehtonen)
    * Disable race tests until bbolt is fixed (Nick Craig-Wood)
    * Move methods used for testing into test file (greatroar)
    * Add Pin and Unpin and canonicalised lookup (Nick Craig-Wood)
    * Use proper import path go.etcd.io/bbolt (Robert-André Mauchin)
* Crypt
    * Calculate hashes for uploads from local disk (Nick Craig-Wood)
        * This allows encrypted Jottacloud uploads without using local disk
        * This means encrypted s3/b2 uploads will now have hashes
    * Added `rclone backend decode`/`encode` commands to replicate functionality of `cryptdecode` (Anagh Kumar Baranwal)
    * Get rid of the unused Cipher interface as it obfuscated the code (Nick Craig-Wood)
* Azure Blob
    * Implement streaming of unknown sized files so `rcat` is now supported (Nick Craig-Wood)
    * Implement memory pooling to control memory use (Nick Craig-Wood)
    * Add `--azureblob-disable-checksum` flag (Nick Craig-Wood)
    * Retry `InvalidBlobOrBlock` error as it may indicate block concurrency problems (Nick Craig-Wood)
    * Remove unused `Object.parseTimeString()` (Lars Lehtonen)
    * Fix permission error on SAS URL limited to container (Nick Craig-Wood)
* B2
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Ignore directory markers at the root also (Nick Craig-Wood)
    * Force the case of the SHA1 to lowercase (Nick Craig-Wood)
    * Remove unused `largeUpload.clearUploadURL()` (Lars Lehtonen)
* Box
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Implement About to read size used (Nick Craig-Wood)
    * Add token renew function for jwt auth (David Bramwell)
    * Added support for interchangeable root folder for Box backend (Sunil Patra)
    * Remove unnecessary iat from jws claims (David)
* Drive
    * Follow shortcuts by default, skip with `--drive-skip-shortcuts` (Nick Craig-Wood)
    * Implement `rclone backend shortcut` command for creating shortcuts (Nick Craig-Wood)
    * Added `rclone backend` command to change `service_account_file` and `chunk_size` (Anagh Kumar Baranwal)
    * Fix missing files when using `--fast-list` and `--drive-shared-with-me` (Nick Craig-Wood)
    * Fix duplicate items when using `--drive-shared-with-me` (Nick Craig-Wood)
    * Extend `--drive-stop-on-upload-limit` to respond to `teamDriveFileLimitExceeded`. (harry)
    * Don't delete files with multiple parents to avoid data loss (Nick Craig-Wood)
    * Server side copy docs use default description if empty (Nick Craig-Wood)
* Dropbox
    * Make error insufficient space to be fatal (harry)
    * Add info about required redirect url (Elan Ruusamäe)
* Fichier
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Implement custom pacer to deal with the new rate limiting (buengese)
* FTP
    * Fix lockup when using concurrency limit on failed connections (Nick Craig-Wood)
    * Fix lockup on failed upload when using concurrency limit (Nick Craig-Wood)
    * Fix lockup on Close failures when using concurrency limit (Nick Craig-Wood)
    * Work around pureftp sending spurious 150 messages (Nick Craig-Wood)
* Google Cloud Storage
    * Add support for `--header-upload` and `--header-download` (Nick Craig-Wood)
    * Add `ARCHIVE` storage class to help (Adam Stroud)
    * Ignore directory markers at the root (Nick Craig-Wood)
* Googlephotos
    * Make the start year configurable (Daven)
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Create feature/favorites directory (Brandon Philips)
    * Fix "concurrent map write" error (Nick Craig-Wood)
    * Don't put an image in error message (Nick Craig-Wood)
* HTTP
    * Improved directory listing with new template from Caddy project (calisro)
* Jottacloud
    * Implement `--jottacloud-trashed-only` (buengese)
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Use `RawURLEncoding` when decoding base64 encoded login token (buengese)
    * Implement cleanup (buengese)
    * Update docs regarding cleanup, removed remains from old auth, and added warning about special mountpoints. (albertony)
* Mailru
    * Describe 2FA requirements (valery1707)
* Onedrive
    * Implement `--onedrive-server-side-across-configs` (Nick Craig-Wood)
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Fix occasional 416 errors on multipart uploads (Nick Craig-Wood)
    * Added maximum chunk size limit warning in the docs (Harry)
    * Fix missing drive on config (Nick Craig-Wood)
    * Make error `quotaLimitReached` to be fatal (harry)
* Opendrive
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
* Pcloud
    * Added support for interchangeable root folder for pCloud backend (Sunil Patra)
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Fix initial config "Auth state doesn't match" message (Nick Craig-Wood)
* Premiumizeme
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Prune unused functions (Lars Lehtonen)
* Putio
    * Add support for `--header-upload` and `--header-download` (Nick Craig-Wood)
    * Make downloading files use the rclone http Client (Nick Craig-Wood)
    * Fix parsing of remotes with leading and trailing / (Nick Craig-Wood)
* Qingstor
    * Make `rclone cleanup` remove pending multipart uploads older than 24h (Nick Craig-Wood)
    * Try harder to cancel failed multipart uploads (Nick Craig-Wood)
    * Prune `multiUploader.list()` (Lars Lehtonen)
    * Lint fix (Lars Lehtonen)
* S3
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Use memory pool for buffer allocations (Maciej Zimnoch)
    * Add SSE-C support for AWS, Ceph, and MinIO (Jack Anderson)
    * Fail fast multipart upload (Michał Matczuk)
    * Report errors on bucket creation (mkdir) correctly (Nick Craig-Wood)
    * Specify that Minio supports URL encoding in listings (Nick Craig-Wood)
    * Added 500 as retryErrorCode (Michał Matczuk)
    * Use `--low-level-retries` as the number of SDK retries (Aleksandar Janković)
    * Fix multipart abort context (Aleksandar Jankovic)
    * Replace deprecated `session.New()` with `session.NewSession()` (Lars Lehtonen)
    * Use the provided size parameter when allocating a new memory pool (Joachim Brandon LeBlanc)
    * Use rclone's low level retries instead of AWS SDK to fix listing retries (Nick Craig-Wood)
    * Ignore directory markers at the root also (Nick Craig-Wood)
    * Use single memory pool (Michał Matczuk)
    * Do not resize buf on put to memBuf (Michał Matczuk)
    * Improve docs for `--s3-disable-checksum` (Nick Craig-Wood)
    * Don't leak memory or tokens in edge cases for multipart upload (Nick Craig-Wood)
* Seafile
    * Implement 2FA (Fred)
* SFTP
    * Added `--sftp-pem-key` to support inline key files (calisro)
    * Fix post transfer copies failing with 0 size when using `set_modtime=false` (Nick Craig-Wood)
* Sharefile
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
* Sugarsync
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
* Swift
    * Add support for `--header-upload` and `--header-download` (Nick Craig-Wood)
    * Fix cosmetic issue in error message (Martin Michlmayr)
* Union
    * Implement multiple writable remotes (Max Sum)
    * Fix server-side copy (Max Sum)
    * Implement ListR (Max Sum)
    * Enable ListR when upstreams contain local (Max Sum)
* WebDAV
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Fix `X-OC-Mtime` header for Transip compatibility (Nick Craig-Wood)
    * Report full and consistent usage with `about` (Yves G)
* Yandex
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)

## v1.51.0 - 2020-02-01

* New backends
    * [Memory](/memory/) (Nick Craig-Wood)
    * [Sugarsync](/sugarsync/) (Nick Craig-Wood)
* New Features
    * Adjust all backends to have `--backend-encoding` parameter (Nick Craig-Wood)
        * this enables the encoding for special characters to be adjusted or disabled
    * Add `--max-duration` flag to control the maximum duration of a transfer session (boosh)
    * Add `--expect-continue-timeout` flag, default 1s (Nick Craig-Wood)
    * Add `--no-check-dest` flag for copying without testing the destination (Nick Craig-Wood)
    * Implement `--order-by` flag to order transfers (Nick Craig-Wood)
    * accounting
        * Don't show entries in both transferring and checking (Nick Craig-Wood)
        * Add option to delete stats (Aleksandar Jankovic)
    * build
        * Compress the test builds with gzip (Nick Craig-Wood)
        * Implement a framework for starting test servers during tests (Nick Craig-Wood)
    * cmd: Always print elapsed time to tenth place seconds in progress (Gary Kim)
    * config
        * Add `--password-command` to allow dynamic config password (Damon Permezel)
        * Give config questions default values (Nick Craig-Wood)
        * Check a remote exists when creating a new one (Nick Craig-Wood)
    * copyurl: Add `--stdout` flag to write to stdout (Nick Craig-Wood)
    * dedupe: Implement keep smallest too (Nick Craig-Wood)
    * hashsum: Add flag `--base64` flag (landall)
    * lsf: Speed up on s3/swift/etc by not reading mimetype by default (Nick Craig-Wood)
    * lsjson: Add `--no-mimetype` flag (Nick Craig-Wood)
    * rc: Add methods to turn on blocking and mutex profiling (Nick Craig-Wood)
    * rcd
        * Adding group parameter to stats (Chaitanya)
        * Move webgui apart; option to disable browser (Xiaoxing Ye)
    * serve sftp: Add support for public key with auth proxy (Paul Tinsley)
    * stats: Show deletes in stats and hide zero stats (anuar45)
* Bug Fixes
    * accounting
        * Fix error counter counting multiple times (Ankur Gupta)
        * Fix error count shown as checks (Cnly)
        * Clear finished transfer in stats-reset (Maciej Zimnoch)
        * Added StatsInfo locking in statsGroups sum function (Michał Matczuk)
    * asyncreader: Fix EOF error (buengese)
    * check: Fix `--one-way` recursing more directories than it needs to (Nick Craig-Wood)
    * chunkedreader: Disable hash calculation for first segment (Nick Craig-Wood)
    * config
        * Do not open browser on headless on drive/gcs/google photos (Xiaoxing Ye)
        * SetValueAndSave ignore error if config section does not exist yet (buengese)
    * cmd: Fix completion with an encrypted config (Danil Semelenov)
    * dbhashsum: Stop it returning UNSUPPORTED on dropbox (Nick Craig-Wood)
    * dedupe: Add missing modes to help string (Nick Craig-Wood)
    * operations
        * Fix dedupe continuing on errors like insufficientFilePersimmon (SezalAgrawal)
        * Clear accounting before low level retry (Maciej Zimnoch)
        * Write debug message when hashes could not be checked (Ole Schütt)
        * Move interface assertion to tests to remove pflag dependency (Nick Craig-Wood)
        * Make NewOverrideObjectInfo public and factor uses (Nick Craig-Wood)
    * proxy: Replace use of bcrypt with sha256 (Nick Craig-Wood)
    * vendor
        * Update bazil.org/fuse to fix FreeBSD 12.1 (Nick Craig-Wood)
        * Update github.com/t3rm1n4l/go-mega to fix mega "illegal base64 data at input byte 22" (Nick Craig-Wood)
        * Update termbox-go to fix ncdu command on FreeBSD (Kuang-che Wu)
        * Update t3rm1n4l/go-mega - fixes mega: couldn't login: crypto/aes: invalid key size 0 (Nick Craig-Wood)
* Mount
    * Enable async reads for a 20% speedup (Nick Craig-Wood)
    * Replace use of WriteAt with Write for cache mode >= writes and O_APPEND (Brett Dutro)
    * Make sure we call unmount when exiting (Nick Craig-Wood)
    * Don't build on go1.10 as bazil/fuse no longer supports it (Nick Craig-Wood)
    * When setting dates discard out of range dates (Nick Craig-Wood)
* VFS
    * Add a newly created file straight into the directory (Nick Craig-Wood)
    * Only calculate one hash for reads for a speedup (Nick Craig-Wood)
    * Make ReadAt for non cached files work better with non-sequential reads (Nick Craig-Wood)
    * Fix edge cases when reading ModTime from file (Nick Craig-Wood)
    * Make sure existing files opened for write show correct size (Nick Craig-Wood)
    * Don't cache the path in RW file objects to fix renaming (Nick Craig-Wood)
    * Fix rename of open files when using the VFS cache (Nick Craig-Wood)
    * When renaming files in the cache, rename the cache item in memory too (Nick Craig-Wood)
    * Fix open file renaming on drive when using `--vfs-cache-mode writes` (Nick Craig-Wood)
    * Fix incorrect modtime for mv into mount with `--vfs-cache-modes writes` (Nick Craig-Wood)
    * On rename, rename in cache too if the file exists (Anagh Kumar Baranwal)
* Local
    * Make source file being updated errors be NoLowLevelRetry errors (Nick Craig-Wood)
    * Fix update of hidden files on Windows (Nick Craig-Wood)
* Cache
    * Follow move of upstream library github.com/coreos/bbolt github.com/etcd-io/bbolt (Nick Craig-Wood)
    * Fix `fatal error: concurrent map writes` (Nick Craig-Wood)
* Crypt
    * Reorder the filename encryption options (Thomas Eales)
    * Correctly handle trailing dot (buengese)
* Chunker
    * Reduce length of temporary suffix (Ivan Andreev)
* Drive
    * Add `--drive-stop-on-upload-limit` flag to stop syncs when upload limit reached (Nick Craig-Wood)
    * Add `--drive-use-shared-date` to use date file was shared instead of modified date (Garry McNulty)
    * Make sure invalid auth for teamdrives always reports an error (Nick Craig-Wood)
    * Fix `--fast-list` when using appDataFolder (Nick Craig-Wood)
    * Use multipart resumable uploads for streaming and uploads in mount (Nick Craig-Wood)
    * Log an ERROR if an incomplete search is returned (Nick Craig-Wood)
    * Hide dangerous config from the configurator (Nick Craig-Wood)
* Dropbox
    * Treat `insufficient_space` errors as non retriable errors (Nick Craig-Wood)
* Jottacloud
    * Use new auth method used by official client (buengese)
    * Add URL to generate Login Token to config wizard (Nick Craig-Wood)
    * Add support whitelabel versions (buengese)
* Koofr
    * Use rclone HTTP client. (jaKa)
* Onedrive
    * Add Sites.Read.All permission (Benjamin Richter)
    * Add support "Retry-After" header (Motonori IWAMURO)
* Opendrive
    * Implement `--opendrive-chunk-size` (Nick Craig-Wood)
* S3
    * Re-implement multipart upload to fix memory issues (Nick Craig-Wood)
    * Add `--s3-copy-cutoff` for size to switch to multipart copy (Nick Craig-Wood)
    * Add new region Asia Pacific (Hong Kong) (Outvi V)
    * Reduce memory usage streaming files by reducing max stream upload size (Nick Craig-Wood)
    * Add `--s3-list-chunk` option for bucket listing (Thomas Kriechbaumer)
    * Force path style bucket access to off for AWS deprecation (Nick Craig-Wood)
    * Use AWS web identity role provider if available (Tennix)
    * Add StackPath Object Storage Support (Dave Koston)
    * Fix ExpiryWindow value (Aleksandar Jankovic)
    * Fix DisableChecksum condition (Aleksandar Janković)
    * Fix URL decoding of NextMarker (Nick Craig-Wood)
* SFTP
    * Add `--sftp-skip-links` to skip symlinks and non regular files (Nick Craig-Wood)
    * Retry Creation of Connection (Sebastian Brandt)
    * Fix "failed to parse private key file: ssh: not an encrypted key" error (Nick Craig-Wood)
    * Open files for update write only to fix AWS SFTP interop (Nick Craig-Wood)
* Swift
    * Reserve segments of dynamic large object when delete objects in container what was enabled versioning. (Nguyễn Hữu Luân)
    * Fix parsing of X-Object-Manifest (Nick Craig-Wood)
    * Update OVH API endpoint (unbelauscht)
* WebDAV
    * Make nextcloud only upload SHA1 checksums (Nick Craig-Wood)
    * Fix case of "Bearer" in Authorization: header to agree with RFC (Nick Craig-Wood)
    * Add Referer header to fix problems with WAFs (Nick Craig-Wood)

## v1.50.2 - 2019-11-19

* Bug Fixes
    * accounting: Fix memory leak on retries operations (Nick Craig-Wood)
* Drive
    * Fix listing of the root directory with drive.files scope (Nick Craig-Wood)
    * Fix --drive-root-folder-id with team/shared drives (Nick Craig-Wood)

## v1.50.1 - 2019-11-02

* Bug Fixes
    * hash: Fix accidentally changed hash names for `DropboxHash` and `CRC-32` (Nick Craig-Wood)
    * fshttp: Fix error reporting on tpslimit token bucket errors (Nick Craig-Wood)
    * fshttp: Don't print token bucket errors on context cancelled (Nick Craig-Wood)
* Local
    * Fix listings of . on Windows (Nick Craig-Wood)
* Onedrive
    * Fix DirMove/Move after Onedrive change (Xiaoxing Ye)

## v1.50.0 - 2019-10-26

* New backends
    * [Citrix Sharefile](/sharefile/) (Nick Craig-Wood)
    * [Chunker](/chunker/) - an overlay backend to split files into smaller parts (Ivan Andreev)
    * [Mail.ru Cloud](/mailru/) (Ivan Andreev)
* New Features
    * encodings (Fabian Möller & Nick Craig-Wood)
        * All backends now use file name encoding to ensure any file name can be written to any backend.
        * See the [restricted file name docs](/overview/#restricted-filenames) for more info and the [local backend docs](/local/#filenames).
        * Some file names may look different in rclone if you are using any control characters in names or [unicode FULLWIDTH symbols](https://en.wikipedia.org/wiki/Halfwidth_and_Fullwidth_Forms_(Unicode_block)).
    * build
        * Update to use go1.13 for the build (Nick Craig-Wood)
        * Drop support for go1.9 (Nick Craig-Wood)
        * Build rclone with GitHub actions (Nick Craig-Wood)
        * Convert python scripts to python3 (Nick Craig-Wood)
        * Swap Azure/go-ansiterm for mattn/go-colorable (Nick Craig-Wood)
        * Dockerfile fixes (Matei David)
        * Add [plugin support](https://github.com/rclone/rclone/blob/master/CONTRIBUTING.md#writing-a-plugin) for backends and commands (Richard Patel)
    * config
        * Use alternating Red/Green in config to make more obvious (Nick Craig-Wood)
    * contrib
        * Add sample DLNA server Docker Compose manifest. (pataquets)
        * Add sample WebDAV server Docker Compose manifest. (pataquets)
    * copyurl
        * Add `--auto-filename` flag for using file name from URL in destination path (Denis)
    * serve dlna:
        * Many compatibility improvements (Dan Walters)
        * Support for external srt subtitles (Dan Walters)
    * rc
        * Added command core/quit (Saksham Khanna)
* Bug Fixes
    * sync
        * Make `--update`/`-u` not transfer files that haven't changed (Nick Craig-Wood)
        * Free objects after they come out of the transfer pipe to save memory (Nick Craig-Wood)
        * Fix `--files-from without --no-traverse` doing a recursive scan (Nick Craig-Wood)
    * operations
        * Fix accounting for server-side copies (Nick Craig-Wood)
        * Display 'All duplicates removed' only if dedupe successful (Sezal Agrawal)
        * Display 'Deleted X extra copies' only if dedupe successful (Sezal Agrawal)
    * accounting
        * Only allow up to 100 completed transfers in the accounting list to save memory (Nick Craig-Wood)
        * Cull the old time ranges when possible to save memory (Nick Craig-Wood)
        * Fix panic due to server-side copy fallback (Ivan Andreev)
        * Fix memory leak noticeable for transfers of large numbers of objects (Nick Craig-Wood)
        * Fix total duration calculation (Nick Craig-Wood)
    * cmd
        * Fix environment variables not setting command line flags (Nick Craig-Wood)
        * Make autocomplete compatible with bash's posix mode for macOS (Danil Semelenov)
        * Make `--progress` work in git bash on Windows (Nick Craig-Wood)
        * Fix 'compopt: command not found' on autocomplete on macOS (Danil Semelenov)
    * config
        * Fix setting of non top level flags from environment variables (Nick Craig-Wood)
        * Check config names more carefully and report errors (Nick Craig-Wood)
        * Remove error: can't use `--size-only` and `--ignore-size` together. (Nick Craig-Wood)
    * filter: Prevent mixing options when `--files-from` is in use (Michele Caci)
    * serve sftp: Fix crash on unsupported operations (e.g. Readlink) (Nick Craig-Wood)
* Mount
    * Allow files of unknown size to be read properly (Nick Craig-Wood)
    * Skip tests on <= 2 CPUs to avoid lockup (Nick Craig-Wood)
    * Fix panic on File.Open (Nick Craig-Wood)
    * Fix "mount_fusefs: -o timeout=: option not supported" on FreeBSD (Nick Craig-Wood)
    * Don't pass huge filenames (>4k) to FUSE as it can't cope (Nick Craig-Wood)
* VFS
    * Add flag `--vfs-case-insensitive` for windows/macOS mounts (Ivan Andreev)
    * Make objects of unknown size readable through the VFS (Nick Craig-Wood)
    * Move writeback of dirty data out of close() method into its own method (FlushWrites) and remove close() call from Flush() (Brett Dutro)
    * Stop empty dirs disappearing when renamed on bucket-based remotes (Nick Craig-Wood)
    * Stop change notify polling clearing so much of the directory cache (Nick Craig-Wood)
* Azure Blob
    * Disable logging to the Windows event log (Nick Craig-Wood)
* B2
    * Remove `unverified:` prefix on sha1 to improve interop (e.g. with CyberDuck) (Nick Craig-Wood)
* Box
    * Add options to get access token via JWT auth (David)
* Drive
    * Disable HTTP/2 by default to work around INTERNAL_ERROR problems (Nick Craig-Wood)
    * Make sure that drive root ID is always canonical (Nick Craig-Wood)
    * Fix `--drive-shared-with-me` from the root with lsand `--fast-list` (Nick Craig-Wood)
    * Fix ChangeNotify polling for shared drives (Nick Craig-Wood)
    * Fix change notify polling when using appDataFolder (Nick Craig-Wood)
* Dropbox
    * Make disallowed filenames errors not retry (Nick Craig-Wood)
    * Fix nil pointer exception on restricted files (Nick Craig-Wood)
* Fichier
    * Fix accessing files > 2GB on 32 bit systems (Nick Craig-Wood)
* FTP
    * Allow disabling EPSV mode (Jon Fautley)
* HTTP
    * HEAD directory entries in parallel to speedup (Nick Craig-Wood)
    * Add `--http-no-head` to stop rclone doing HEAD in listings (Nick Craig-Wood)
* Putio
    * Add ability to resume uploads (Cenk Alti)
* S3
    * Fix signature v2_auth headers (Anthony Rusdi)
    * Fix encoding for control characters (Nick Craig-Wood)
    * Only ask for URL encoded directory listings if we need them on Ceph (Nick Craig-Wood)
    * Add option for multipart failure behaviour (Aleksandar Jankovic)
    * Support for multipart copy (庄天翼)
    * Fix nil pointer reference if no metadata returned for object (Nick Craig-Wood)
* SFTP
    * Fix `--sftp-ask-password` trying to contact the ssh agent (Nick Craig-Wood)
    * Fix hashes of files with backslashes (Nick Craig-Wood)
    * Include more ciphers with `--sftp-use-insecure-cipher` (Carlos Ferreyra)
* WebDAV
    * Parse and return Sharepoint error response (Henning Surmeier)

## v1.49.5 - 2019-10-05

* Bug Fixes
    * Revert back to go1.12.x for the v1.49.x builds as go1.13.x was causing issues (Nick Craig-Wood)
    * Fix rpm packages by using master builds of nfpm (Nick Craig-Wood)
    * Fix macOS build after brew changes (Nick Craig-Wood)

## v1.49.4 - 2019-09-29

* Bug Fixes
    * cmd/rcd: Address ZipSlip vulnerability (Richard Patel)
    * accounting: Fix file handle leak on errors (Nick Craig-Wood)
    * oauthutil: Fix security problem when running with two users on the same machine (Nick Craig-Wood)
* FTP
    * Fix listing of an empty root returning: error dir not found (Nick Craig-Wood)
* S3
    * Fix SetModTime on GLACIER/ARCHIVE objects and implement set/get tier (Nick Craig-Wood)

## v1.49.3 - 2019-09-15

* Bug Fixes
    * accounting
        * Fix total duration calculation (Aleksandar Jankovic)
        * Fix "file already closed" on transfer retries (Nick Craig-Wood)

## v1.49.2 - 2019-09-08

* New Features
    * build: Add Docker workflow support (Alfonso Montero)
* Bug Fixes
    * accounting: Fix locking in Transfer to avoid deadlock with `--progress` (Nick Craig-Wood)
    * docs: Fix template argument for mktemp in install.sh (Cnly)
    * operations: Fix `-u`/`--update` with google photos / files of unknown size (Nick Craig-Wood)
    * rc: Fix docs for config/create /update /password (Nick Craig-Wood)
* Google Cloud Storage
    * Fix need for elevated permissions on SetModTime (Nick Craig-Wood)

## v1.49.1 - 2019-08-28

* Bug Fixes
    * config: Fix generated passwords being stored as empty password (Nick Craig-Wood)
    * rcd: Added missing parameter for web-gui info logs. (Chaitanya)
* Googlephotos
    * Fix crash on error response (Nick Craig-Wood)
* Onedrive
    * Fix crash on error response (Nick Craig-Wood)

## v1.49.0 - 2019-08-26

* New backends
    * [1fichier](/fichier/) (Laura Hausmann)
    * [Google Photos](/googlephotos/) (Nick Craig-Wood)
    * [Putio](/putio/) (Cenk Alti)
    * [premiumize.me](/premiumizeme/) (Nick Craig-Wood)
* New Features
    * Experimental [web GUI](/gui/) (Chaitanya Bankanhal)
    * Implement `--compare-dest` & `--copy-dest` (yparitcher)
    * Implement `--suffix` without `--backup-dir` for backup to current dir (yparitcher)
    * `config reconnect` to re-login (re-run the oauth login) for the backend. (Nick Craig-Wood)
    * `config userinfo` to discover which user you are logged in as. (Nick Craig-Wood)
    * `config disconnect` to disconnect you (log out) from the backend. (Nick Craig-Wood)
    * Add `--use-json-log` for JSON logging (justinalin)
    * Add context propagation to rclone (Aleksandar Jankovic)
    * Reworking internal statistics interfaces so they work with rc jobs (Aleksandar Jankovic)
    * Add Higher units for ETA (AbelThar)
    * Update rclone logos to new design (Andreas Chlupka)
    * hash: Add CRC-32 support (Cenk Alti)
    * help showbackend: Fixed advanced option category when there are no standard options (buengese)
    * ncdu: Display/Copy to Clipboard Current Path (Gary Kim)
    * operations:
        * Run hashing operations in parallel (Nick Craig-Wood)
        * Don't calculate checksums when using `--ignore-checksum` (Nick Craig-Wood)
        * Check transfer hashes when using `--size-only` mode (Nick Craig-Wood)
        * Disable multi thread copy for local to local copies (Nick Craig-Wood)
        * Debug successful hashes as well as failures (Nick Craig-Wood)
    * rc
        * Add ability to stop async jobs (Aleksandar Jankovic)
        * Return current settings if core/bwlimit called without parameters (Nick Craig-Wood)
        * Rclone-WebUI integration with rclone (Chaitanya Bankanhal)
        * Added command line parameter to control the cross origin resource sharing (CORS) in the rcd. (Security Improvement) (Chaitanya Bankanhal)
        * Add anchor tags to the docs so links are consistent (Nick Craig-Wood)
        * Remove _async key from input parameters after parsing so later operations won't get confused (buengese)
        * Add call to clear stats (Aleksandar Jankovic)
    * rcd
        * Auto-login for web-gui (Chaitanya Bankanhal)
        * Implement `--baseurl` for rcd and web-gui (Chaitanya Bankanhal)
    * serve dlna
        * Only select interfaces which can multicast for SSDP (Nick Craig-Wood)
        * Add more builtin mime types to cover standard audio/video (Nick Craig-Wood)
        * Fix missing mime types on Android causing missing videos (Nick Craig-Wood)
    * serve ftp
        * Refactor to bring into line with other serve commands (Nick Craig-Wood)
        * Implement `--auth-proxy` (Nick Craig-Wood)
    * serve http: Implement `--baseurl` (Nick Craig-Wood)
    * serve restic: Implement `--baseurl` (Nick Craig-Wood)
    * serve sftp
        * Implement auth proxy (Nick Craig-Wood)
        * Fix detection of whether server is authorized (Nick Craig-Wood)
    * serve webdav
        * Implement `--baseurl` (Nick Craig-Wood)
        * Support `--auth-proxy` (Nick Craig-Wood)
* Bug Fixes
    * Make "bad record MAC" a retriable error (Nick Craig-Wood)
    * copyurl: Fix copying files that return HTTP errors (Nick Craig-Wood)
    * march: Fix checking sub-directories when using `--no-traverse` (buengese)
    * rc
        * Fix unmarshalable http.AuthFn in options and put in test for marshalability (Nick Craig-Wood)
        * Move job expire flags to rc to fix initialization problem (Nick Craig-Wood)
        * Fix `--loopback` with rc/list and others (Nick Craig-Wood)
    * rcat: Fix slowdown on systems with multiple hashes (Nick Craig-Wood)
    * rcd: Fix permissions problems on cache directory with web gui download (Nick Craig-Wood)
* Mount
    * Default `--daemon-timeout` to 15 minutes on macOS and FreeBSD (Nick Craig-Wood)
    * Update docs to show mounting from root OK for bucket-based (Nick Craig-Wood)
    * Remove nonseekable flag from write files (Nick Craig-Wood)
* VFS
    * Make write without cache more efficient (Nick Craig-Wood)
    * Fix `--vfs-cache-mode minimal` and `writes` ignoring cached files (Nick Craig-Wood)
* Local
    * Add `--local-case-sensitive` and `--local-case-insensitive` (Nick Craig-Wood)
    * Avoid polluting page cache when uploading local files to remote backends (Michał Matczuk)
    * Don't calculate any hashes by default (Nick Craig-Wood)
    * Fadvise run syscall on a dedicated go routine (Michał Matczuk)
* Azure Blob
    * Azure Storage Emulator support (Sandeep)
    * Updated config help details to remove connection string references (Sandeep)
    * Make all operations work from the root (Nick Craig-Wood)
* B2
    * Implement link sharing (yparitcher)
    * Enable server-side copy to copy between buckets (Nick Craig-Wood)
    * Make all operations work from the root (Nick Craig-Wood)
* Drive
    * Fix server-side copy of big files (Nick Craig-Wood)
    * Update API for teamdrive use (Nick Craig-Wood)
    * Add error for purge with `--drive-trashed-only` (ginvine)
* Fichier
    * Make FolderID int and adjust related code (buengese)
* Google Cloud Storage
    * Reduce oauth scope requested as suggested by Google (Nick Craig-Wood)
    * Make all operations work from the root (Nick Craig-Wood)
* HTTP
    * Add `--http-headers` flag for setting arbitrary headers (Nick Craig-Wood)
* Jottacloud
    * Use new api for retrieving internal username (buengese)
    * Refactor configuration and minor cleanup (buengese)
* Koofr
    * Support setting modification times on Koofr backend. (jaKa)
* Opendrive
    * Refactor to use existing lib/rest facilities for uploads (Nick Craig-Wood)
* Qingstor
    * Upgrade to v3 SDK and fix listing loop (Nick Craig-Wood)
    * Make all operations work from the root (Nick Craig-Wood)
* S3
    * Add INTELLIGENT_TIERING storage class (Matti Niemenmaa)
    * Make all operations work from the root (Nick Craig-Wood)
* SFTP
    * Add missing interface check and fix About (Nick Craig-Wood)
    * Completely ignore all modtime checks if SetModTime=false (Jon Fautley)
    * Support md5/sha1 with rsync.net (Nick Craig-Wood)
    * Save the md5/sha1 command in use to the config file for efficiency (Nick Craig-Wood)
    * Opt-in support for diffie-hellman-group-exchange-sha256 diffie-hellman-group-exchange-sha1 (Yi FU)
* Swift
    * Use FixRangeOption to fix 0 length files via the VFS (Nick Craig-Wood)
    * Fix upload when using no_chunk to return the correct size (Nick Craig-Wood)
    * Make all operations work from the root (Nick Craig-Wood)
    * Fix segments leak during failed large file uploads. (nguyenhuuluan434)
* WebDAV
    * Add `--webdav-bearer-token-command` (Nick Craig-Wood)
    * Refresh token when it expires with `--webdav-bearer-token-command` (Nick Craig-Wood)
    * Add docs for using bearer_token_command with oidc-agent (Paul Millar)

## v1.48.0 - 2019-06-15

* New commands
    * serve sftp: Serve an rclone remote over SFTP (Nick Craig-Wood)
* New Features
    * Multi threaded downloads to local storage (Nick Craig-Wood)
        * controlled with `--multi-thread-cutoff` and `--multi-thread-streams`
    * Use rclone.conf from rclone executable directory to enable portable use (albertony)
    * Allow sync of a file and a directory with the same name (forgems)
        * this is common on bucket-based remotes, e.g. s3, gcs
    * Add `--ignore-case-sync` for forced case insensitivity (garry415)
    * Implement `--stats-one-line-date` and `--stats-one-line-date-format` (Peter Berbec)
    * Log an ERROR for all commands which exit with non-zero status (Nick Craig-Wood)
    * Use go-homedir to read the home directory more reliably (Nick Craig-Wood)
    * Enable creating encrypted config through external script invocation (Wojciech Smigielski)
    * build: Drop support for go1.8 (Nick Craig-Wood)
    * config: Make config create/update encrypt passwords where necessary (Nick Craig-Wood)
    * copyurl: Honor `--no-check-certificate` (Stefan Breunig)
    * install: Linux skip man pages if no mandb (didil)
    * lsf: Support showing the Tier of the object (Nick Craig-Wood)
    * lsjson
        * Added EncryptedPath to output (calisro)
        * Support showing the Tier of the object (Nick Craig-Wood)
        * Add IsBucket field for bucket-based remote listing of the root (Nick Craig-Wood)
    * rc
        * Add `--loopback` flag to run commands directly without a server (Nick Craig-Wood)
        * Add operations/fsinfo: Return information about the remote (Nick Craig-Wood)
        * Skip auth for OPTIONS request (Nick Craig-Wood)
        * cmd/providers: Add DefaultStr, ValueStr and Type fields (Nick Craig-Wood)
        * jobs: Make job expiry timeouts configurable (Aleksandar Jankovic)
    * serve dlna reworked and improved (Dan Walters)
    * serve ftp: add `--ftp-public-ip` flag to specify public IP (calistri)
    * serve restic: Add support for `--private-repos` in `serve restic` (Florian Apolloner)
    * serve webdav: Combine serve webdav and serve http (Gary Kim)
    * size: Ignore negative sizes when calculating total (Garry McNulty)
* Bug Fixes
    * Make move and copy individual files obey `--backup-dir` (Nick Craig-Wood)
    * If `--ignore-checksum` is in effect, don't calculate checksum (Nick Craig-Wood)
    * moveto: Fix case-insensitive same remote move (Gary Kim)
    * rc: Fix serving bucket-based objects with `--rc-serve` (Nick Craig-Wood)
    * serve webdav: Fix serveDir not being updated with changes from webdav (Gary Kim)
* Mount
    * Fix poll interval documentation (Animosity022)
* VFS
    * Make WriteAt for non cached files work with non-sequential writes (Nick Craig-Wood)
* Local
    * Only calculate the required hashes for big speedup (Nick Craig-Wood)
    * Log errors when listing instead of returning an error (Nick Craig-Wood)
    * Fix preallocate warning on Linux with ZFS (Nick Craig-Wood)
* Crypt
    * Make rclone dedupe work through crypt (Nick Craig-Wood)
    * Fix wrapping of ChangeNotify to decrypt directories properly (Nick Craig-Wood)
    * Support PublicLink (rclone link) of underlying backend (Nick Craig-Wood)
    * Implement Optional methods SetTier, GetTier (Nick Craig-Wood)
* B2
    * Implement server-side copy (Nick Craig-Wood)
    * Implement SetModTime (Nick Craig-Wood)
* Drive
    * Fix move and copy from TeamDrive to GDrive (Fionera)
    * Add notes that cleanup works in the background on drive (Nick Craig-Wood)
    * Add `--drive-server-side-across-configs` to default back to old server-side copy semantics by default (Nick Craig-Wood)
    * Add `--drive-size-as-quota` to show storage quota usage for file size (Garry McNulty)
* FTP
    * Add FTP List timeout (Jeff Quinn)
    * Add FTP over TLS support (Gary Kim)
    * Add `--ftp-no-check-certificate` option for FTPS (Gary Kim)
* Google Cloud Storage
    * Fix upload errors when uploading pre 1970 files (Nick Craig-Wood)
* Jottacloud
    * Add support for selecting device and mountpoint. (buengese)
* Mega
    * Add cleanup support (Gary Kim)
* Onedrive
    * More accurately check if root is found (Cnly)
* S3
    * Support S3 Accelerated endpoints with `--s3-use-accelerate-endpoint` (Nick Craig-Wood)
    * Add config info for Wasabi's EU Central endpoint (Robert Marko)
    * Make SetModTime work for GLACIER while syncing (Philip Harvey)
* SFTP
    * Add About support (Gary Kim)
    * Fix about parsing of `df` results so it can cope with -ve results (Nick Craig-Wood)
    * Send custom client version and debug server version (Nick Craig-Wood)
* WebDAV
    * Retry on 423 Locked errors (Nick Craig-Wood)

## v1.47.0 - 2019-04-13

* New backends
    * Backend for Koofr cloud storage service. (jaKa)
* New Features
    * Resume downloads if the reader fails in copy (Nick Craig-Wood)
        * this means rclone will restart transfers if the source has an error
        * this is most useful for downloads or cloud to cloud copies
    * Use `--fast-list` for listing operations where it won't use more memory (Nick Craig-Wood)
        * this should speed up the following operations on remotes which support `ListR`
        * `dedupe`, `serve restic` `lsf`, `ls`, `lsl`, `lsjson`, `lsd`, `md5sum`, `sha1sum`, `hashsum`, `size`, `delete`, `cat`, `settier`
        * use `--disable ListR` to get old behaviour if required
    * Make `--files-from` traverse the destination unless `--no-traverse` is set (Nick Craig-Wood)
        * this fixes `--files-from` with Google drive and excessive API use in general.
    * Make server-side copy account bytes and obey `--max-transfer` (Nick Craig-Wood)
    * Add `--create-empty-src-dirs` flag and default to not creating empty dirs (ishuah)
    * Add client side TLS/SSL flags `--ca-cert`/`--client-cert`/`--client-key` (Nick Craig-Wood)
    * Implement `--suffix-keep-extension` for use with `--suffix` (Nick Craig-Wood)
    * build:
        * Switch to semver compliant version tags to be go modules compliant (Nick Craig-Wood)
        * Update to use go1.12.x for the build (Nick Craig-Wood)
    * serve dlna: Add connection manager service description to improve compatibility (Dan Walters)
    * lsf: Add 'e' format to show encrypted names and 'o' for original IDs (Nick Craig-Wood)
    * lsjson: Added `--files-only` and `--dirs-only` flags (calistri)
    * rc: Implement operations/publiclink the equivalent of `rclone link` (Nick Craig-Wood)
* Bug Fixes
    * accounting: Fix total ETA when `--stats-unit bits` is in effect (Nick Craig-Wood)
    * Bash TAB completion
        * Use private custom func to fix clash between rclone and kubectl (Nick Craig-Wood)
        * Fix for remotes with underscores in their names (Six)
        * Fix completion of remotes (Florian Gamböck)
        * Fix autocompletion of remote paths with spaces (Danil Semelenov)
    * serve dlna: Fix root XML service descriptor (Dan Walters)
    * ncdu: Fix display corruption with Chinese characters (Nick Craig-Wood)
    * Add SIGTERM to signals which run the exit handlers on unix (Nick Craig-Wood)
    * rc: Reload filter when the options are set via the rc (Nick Craig-Wood)
* VFS / Mount
    * Fix FreeBSD: Ignore Truncate if called with no readers and already the correct size (Nick Craig-Wood)
    * Read directory and check for a file before mkdir (Nick Craig-Wood)
    * Shorten the locking window for vfs/refresh (Nick Craig-Wood)
* Azure Blob
    * Enable MD5 checksums when uploading files bigger than the "Cutoff" (Dr.Rx)
    * Fix SAS URL support (Nick Craig-Wood)
* B2
    * Allow manual configuration of backblaze downloadUrl (Vince)
    * Ignore already_hidden error on remove (Nick Craig-Wood)
    * Ignore malformed `src_last_modified_millis` (Nick Craig-Wood)
* Drive
    * Add `--skip-checksum-gphotos` to ignore incorrect checksums on Google Photos (Nick Craig-Wood)
    * Allow server-side move/copy between different remotes. (Fionera)
    * Add docs on team drives and `--fast-list` eventual consistency (Nestar47)
    * Fix imports of text files (Nick Craig-Wood)
    * Fix range requests on 0 length files (Nick Craig-Wood)
    * Fix creation of duplicates with server-side copy (Nick Craig-Wood)
* Dropbox
    * Retry blank errors to fix long listings (Nick Craig-Wood)
* FTP
    * Add `--ftp-concurrency` to limit maximum number of connections (Nick Craig-Wood)
* Google Cloud Storage
    * Fall back to default application credentials (marcintustin)
    * Allow bucket policy only buckets (Nick Craig-Wood)
* HTTP
    * Add `--http-no-slash` for websites with directories with no slashes (Nick Craig-Wood)
    * Remove duplicates from listings (Nick Craig-Wood)
    * Fix socket leak on 404 errors (Nick Craig-Wood)
* Jottacloud
    * Fix token refresh (Sebastian Bünger)
    * Add device registration (Oliver Heyme)
* Onedrive
    * Implement graceful cancel of multipart uploads if rclone is interrupted (Cnly)
    * Always add trailing colon to path when addressing items, (Cnly)
    * Return errors instead of panic for invalid uploads (Fabian Möller)
* S3
    * Add support for "Glacier Deep Archive" storage class (Manu)
    * Update Dreamhost endpoint (Nick Craig-Wood)
    * Note incompatibility with CEPH Jewel (Nick Craig-Wood)
* SFTP
    * Allow custom ssh client config (Alexandru Bumbacea)
* Swift
    * Obey Retry-After to enable OVH restore from cold storage (Nick Craig-Wood)
    * Work around token expiry on CEPH (Nick Craig-Wood)
* WebDAV
    * Allow IsCollection property to be integer or boolean (Nick Craig-Wood)
    * Fix race when creating directories (Nick Craig-Wood)
    * Fix About/df when reading the available/total returns 0 (Nick Craig-Wood)

## v1.46 - 2019-02-09

* New backends
    * Support Alibaba Cloud (Aliyun) OSS via the s3 backend (Nick Craig-Wood)
* New commands
    * serve dlna: serves a remove via DLNA for the local network (nicolov)
* New Features
    * copy, move: Restore deprecated `--no-traverse` flag (Nick Craig-Wood)
        * This is useful for when transferring a small number of files into a large destination
    * genautocomplete: Add remote path completion for bash completion (Christopher Peterson & Danil Semelenov)
    * Buffer memory handling reworked to return memory to the OS better (Nick Craig-Wood)
        * Buffer recycling library to replace sync.Pool
        * Optionally use memory mapped memory for better memory shrinking
        * Enable with `--use-mmap` if having memory problems - not default yet
    * Parallelise reading of files specified by `--files-from` (Nick Craig-Wood)
    * check: Add stats showing total files matched. (Dario Guzik)
    * Allow rename/delete open files under Windows (Nick Craig-Wood)
    * lsjson: Use exactly the correct number of decimal places in the seconds (Nick Craig-Wood)
    * Add cookie support with cmdline switch `--use-cookies` for all HTTP based remotes (qip)
    * Warn if `--checksum` is set but there are no hashes available (Nick Craig-Wood)
    * Rework rate limiting (pacer) to be more accurate and allow bursting (Nick Craig-Wood)
    * Improve error reporting for too many/few arguments in commands (Nick Craig-Wood)
    * listremotes: Remove `-l` short flag as it conflicts with the new global flag (weetmuts)
    * Make http serving with auth generate INFO messages on auth fail (Nick Craig-Wood)
* Bug Fixes
    * Fix layout of stats (Nick Craig-Wood)
    * Fix `--progress` crash under Windows Jenkins (Nick Craig-Wood)
    * Fix transfer of google/onedrive docs by calling Rcat in Copy when size is -1 (Cnly)
    * copyurl: Fix checking of `--dry-run` (Denis Skovpen)
* Mount
    * Check that mountpoint and local directory to mount don't overlap (Nick Craig-Wood)
    * Fix mount size under 32 bit Windows (Nick Craig-Wood)
* VFS
    * Implement renaming of directories for backends without DirMove (Nick Craig-Wood)
        * now all backends except b2 support renaming directories
    * Implement `--vfs-cache-max-size` to limit the total size of the cache (Nick Craig-Wood)
    * Add `--dir-perms` and `--file-perms` flags to set default permissions (Nick Craig-Wood)
    * Fix deadlock on concurrent operations on a directory (Nick Craig-Wood)
    * Fix deadlock between RWFileHandle.close and File.Remove (Nick Craig-Wood)
    * Fix renaming/deleting open files with cache mode "writes" under Windows (Nick Craig-Wood)
    * Fix panic on rename with `--dry-run` set (Nick Craig-Wood)
    * Fix vfs/refresh with recurse=true needing the `--fast-list` flag
* Local
    * Add support for `-l`/`--links` (symbolic link translation) (yair@unicorn)
        * this works by showing links as `link.rclonelink` - see local backend docs for more info
        * this errors if used with `-L`/`--copy-links`
    * Fix renaming/deleting open files on Windows (Nick Craig-Wood)
* Crypt
    * Check for maximum length before decrypting filename to fix panic (Garry McNulty)
* Azure Blob
    * Allow building azureblob backend on *BSD (themylogin)
    * Use the rclone HTTP client to support `--dump headers`, `--tpslimit`, etc. (Nick Craig-Wood)
    * Use the s3 pacer for 0 delay in non error conditions (Nick Craig-Wood)
    * Ignore directory markers (Nick Craig-Wood)
    * Stop Mkdir attempting to create existing containers (Nick Craig-Wood)
* B2
    * cleanup: will remove unfinished large files >24hrs old (Garry McNulty)
    * For a bucket limited application key check the bucket name (Nick Craig-Wood)
        * before this, rclone would use the authorised bucket regardless of what you put on the command line
    * Added `--b2-disable-checksum` flag (Wojciech Smigielski)
        * this enables large files to be uploaded without a SHA-1 hash for speed reasons
* Drive
    * Set default pacer to 100ms for 10 tps (Nick Craig-Wood)
        * This fits the Google defaults much better and reduces the 403 errors massively
        * Add `--drive-pacer-min-sleep` and `--drive-pacer-burst` to control the pacer
    * Improve ChangeNotify support for items with multiple parents (Fabian Möller)
    * Fix ListR for items with multiple parents - this fixes oddities with `vfs/refresh` (Fabian Möller)
    * Fix using `--drive-impersonate` and appfolders (Nick Craig-Wood)
    * Fix google docs in rclone mount for some (not all) applications (Nick Craig-Wood)
* Dropbox
    * Retry-After support for Dropbox backend (Mathieu Carbou)
* FTP
    * Wait for 60 seconds for a connection to Close then declare it dead (Nick Craig-Wood)
        * helps with indefinite hangs on some FTP servers
* Google Cloud Storage
    * Update google cloud storage endpoints (weetmuts)
* HTTP
    * Add an example with username and password which is supported but wasn't documented (Nick Craig-Wood)
    * Fix backend with `--files-from` and nonexistent files (Nick Craig-Wood)
* Hubic
    * Make error message more informative if authentication fails (Nick Craig-Wood)
* Jottacloud
    * Resume and deduplication support (Oliver Heyme)
    * Use token auth for all API requests Don't store password anymore (Sebastian Bünger)
    * Add support for 2-factor authentication (Sebastian Bünger)
* Mega
    * Implement v2 account login which fixes logins for newer Mega accounts (Nick Craig-Wood)
    * Return error if an unknown length file is attempted to be uploaded (Nick Craig-Wood)
    * Add new error codes for better error reporting (Nick Craig-Wood)
* Onedrive
    * Fix broken support for "shared with me" folders (Alex Chen)
    * Fix root ID not normalised (Cnly)
    * Return err instead of panic on unknown-sized uploads (Cnly)
* Qingstor
    * Fix go routine leak on multipart upload errors (Nick Craig-Wood)
    * Add upload chunk size/concurrency/cutoff control (Nick Craig-Wood)
    * Default `--qingstor-upload-concurrency` to 1 to work around bug (Nick Craig-Wood)
* S3
    * Implement `--s3-upload-cutoff` for single part uploads below this (Nick Craig-Wood)
    * Change `--s3-upload-concurrency` default to 4 to increase performance (Nick Craig-Wood)
    * Add `--s3-bucket-acl` to control bucket ACL (Nick Craig-Wood)
    * Auto detect region for buckets on operation failure (Nick Craig-Wood)
    * Add GLACIER storage class (William Cocker)
    * Add Scaleway to s3 documentation (Rémy Léone)
    * Add AWS endpoint eu-north-1 (weetmuts)
* SFTP
    * Add support for PEM encrypted private keys (Fabian Möller)
    * Add option to force the usage of an ssh-agent (Fabian Möller)
    * Perform environment variable expansion on key-file (Fabian Möller)
    * Fix rmdir on Windows based servers (e.g. CrushFTP) (Nick Craig-Wood)
    * Fix rmdir deleting directory contents on some SFTP servers (Nick Craig-Wood)
    * Fix error on dangling symlinks (Nick Craig-Wood)
* Swift
    * Add `--swift-no-chunk` to disable segmented uploads in rcat/mount (Nick Craig-Wood)
    * Introduce application credential auth support (kayrus)
    * Fix memory usage by slimming Object (Nick Craig-Wood)
    * Fix extra requests on upload (Nick Craig-Wood)
    * Fix reauth on big files (Nick Craig-Wood)
* Union
    * Fix poll-interval not working (Nick Craig-Wood)
* WebDAV
    * Support About which means rclone mount will show the correct disk size (Nick Craig-Wood)
    * Support MD5 and SHA1 hashes with Owncloud and Nextcloud (Nick Craig-Wood)
    * Fail soft on time parsing errors (Nick Craig-Wood)
    * Fix infinite loop on failed directory creation (Nick Craig-Wood)
    * Fix identification of directories for Bitrix Site Manager (Nick Craig-Wood)
    * Fix upload of 0 length files on some servers (Nick Craig-Wood)
    * Fix if MKCOL fails with 423 Locked assume the directory exists (Nick Craig-Wood)

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
        * operations/* for all low level operations, e.g. copy file, list directory
        * sync/* for sync, copy and move
        * `--rc-files` flag to serve files on the rc http server
          * this is for building web native GUIs for rclone
        * Optionally serving objects on the rc http server
        * Ensure rclone fails to start up if the `--rc` port is in use already
        * See [the rc docs](/rc/) for more info
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
    * cmd: Make `--progress` update the stats correctly at the end (Nick Craig-Wood)
    * config: Create config directory on save if it is missing (Nick Craig-Wood)
    * dedupe: Check for existing filename before renaming a dupe file (ssaqua)
    * move: Don't create directories with `--dry-run` (Nick Craig-Wood)
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
    * Set ACL for server-side copies to that provided by the user (Nick Craig-Wood)
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
    * Add `--log-format` flag for more control over log output (dcpu)
    * rc: Add support for OPTIONS and basic CORS (frenos)
    * stats: show FatalErrors and NoRetryErrors in stats (Cédric Connes)
* Bug Fixes
    * Fix -P not ending with a new line (Nick Craig-Wood)
    * config: don't create default config dir when user supplies `--config` (albertony)
    * Don't print non-ASCII characters with `--progress` on windows (Nick Craig-Wood)
    * Correct logs for excluded items (ssaqua)
* Mount
    * Remove EXPERIMENTAL tags (Nick Craig-Wood)
* VFS
    * Fix race condition detected by serve ftp tests (Nick Craig-Wood)
    * Add vfs/poll-interval rc command (Fabian Möller)
    * Enable rename for nearly all remotes using server-side Move or Copy (Nick Craig-Wood)
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
    * Add `--azureblob-list-chunk` parameter (Santiago Rodríguez)
    * Implemented settier command support on azureblob remote. (sandeepkru)
    * Work around SDK bug which causes errors for chunk-sized files (Nick Craig-Wood)
* Box
    * Implement link sharing. (Sebastian Bünger)
* Drive
    * Add `--drive-import-formats` - google docs can now be imported (Fabian Möller)
        * Rewrite mime type and extension handling (Fabian Möller)
        * Add document links (Fabian Möller)
        * Add support for multipart document extensions (Fabian Möller)
        * Add support for apps-script to json export (Fabian Möller)
        * Fix escaped chars in documents during list (Fabian Möller)
    * Add `--drive-v2-download-min-size` a workaround for slow downloads (Fabian Möller)
    * Improve directory notifications in ChangeNotify (Fabian Möller)
    * When listing team drives in config, continue on failure (Nick Craig-Wood)
* FTP
    * Add a small pause after failed upload before deleting file (Nick Craig-Wood)
* Google Cloud Storage
    * Fix service_account_file being ignored (Fabian Möller)
* Jottacloud
    * Minor improvement in quota info (omit if unlimited) (albertony)
    * Add `--fast-list` support (albertony)
    * Add permanent delete support: `--jottacloud-hard-delete` (albertony)
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
    * Use configured server-side-encryption and storage class options when calling CopyObject() (Paul Kohout)
    * Make `--s3-v2-auth` flag (Nick Craig-Wood)
    * Fix v2 auth on files with spaces (Nick Craig-Wood)
* Union
    * Implement union backend which reads from multiple backends (Felix Brucker)
    * Implement optional interfaces (Move, DirMove, Copy, etc.) (Nick Craig-Wood)
    * Fix ChangeNotify to support multiple remotes (Fabian Möller)
    * Fix `--backup-dir` on union backend (Nick Craig-Wood)
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
    * cmd: Fix crash with `--progress` and `--stats 0` (Nick Craig-Wood)
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
    * Make it so just `storage_url` or `auth_token` can be overridden (Nick Craig-Wood)
    * Fix server-side copy bug for unusual file names (Nick Craig-Wood)
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
    * Check the encrypted hash of files when uploading for extra data security
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
    * Use About to return the correct disk total/used/free (e.g. in `df`)
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
    * Fix no error on listing nonexistent directory
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
    * config: fixes errors on nonexistent config by loading config file only on first access
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
    * Don't check for bucket/container presence if listing was OK
        * this makes rclone do one less request per invocation
    * Improve accounting for chunked uploads
* Backblaze B2
    * Don't check for bucket/container presence if listing was OK
        * this makes rclone do one less request per invocation
* Box
    * Improve accounting for chunked uploads
* Dropbox
    * Fix custom oauth client parameters
* Google Cloud Storage
    * Don't check for bucket/container presence if listing was OK
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
    * Don't check for bucket/container presence if listing was OK
        * this makes rclone do one less request per invocation
* S3
    * Support hashes for multipart files (Chris Redekop)
    * Initial support for IBM COS (S3) (Giri Badanahatti)
    * Update docs to discourage use of v2 auth with CEPH and others
    * Don't check for bucket/container presence if listing was OK
        * this makes rclone do one less request per invocation
    * Fix server-side copy and set modtime on files with + in
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
    * Don't check for bucket/container presence if listing was OK
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
    * rmdirs: add --leave-root flag (lewapm)
    * move: add --delete-empty-src-dirs flag to remove dirs after move (Ishuah Kariuki)
    * Add --dump flag, introduce --dump requests, responses and remove --dump-auth, --dump-filters
        * Obscure X-Auth-Token: from headers when dumping too
    * Document and implement exit codes for different failure modes (Ishuah Kariuki)
* Compile
* Bug Fixes
    * Retry lots more different types of errors to make multipart transfers more reliable
    * Save the config before asking for a token, fixes disappearing oauth config
    * Warn the user if --include and --exclude are used together (Ernest Borowski)
    * Fix duplicate files (e.g. on Google drive) causing spurious copies
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
    * `cryptdecode` - decode encrypted file names (thanks ishuah)
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
    * Reuse `rcat` internals to support uploads from all remotes
* Dropbox
    * Fix "entry doesn't belong in directory" error
    * Stop using deprecated API methods
* Swift
    * Fix server-side copy to empty container with `--fast-list`
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
    * rclone lsjson - for listing with a machine-readable output
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
        * this is useful in conjunction with `rclone mount` to limit external apps
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
    * Create container if necessary on server-side copy
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
    * Create container if necessary on server-side copy
    * Add us-east-2 (Ohio) and eu-west-2 (London) S3 regions - Zahiar Ahmed
* Swift, Hubic
    * Fix zero length directory markers showing in the subdirectory listing
        * this caused lots of duplicate transfers
    * Fix paged directory listings
        * this caused duplicate directory errors
    * Create container if necessary on server-side copy
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
    * Allow overlapping directories in move when server-side dir move is supported
    * Add --stats-unit option - thanks Scott McGillivray
* Bug Fixes
    * Fix the config file being overwritten when two rclone instances are running
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
    * Add support for server-side move and directory move - thanks Stefan Breunig
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
    * Fix `rclone check` on encrypted file systems
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
    * Windows: Ignore directory-based junction points
* B2
    * Make sure each upload has at least one upload slot - fixes strange upload stats
    * Fix uploads when using crypt
    * Fix download of large files (sha1 mismatch)
    * Return error when we try to create a bucket which someone else owns
    * Update B2 docs with Data usage, and Crypt section - thanks Tomasz Mazur
* S3
    * Command line and config file support for
        * Setting/overriding ACL  - thanks Radek Šenfeld
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
    * Fix URL escaping in file names - e.g. uploading files with `+` in them.
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
    * Add --b2-versions flag so old versions can be listed and retrieved.
    * Treat 403 errors (e.g. cap exceeded) as fatal.
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
    * Display the transfer stats in more human-readable form
    * Make 0 size files specifiable with `--max-size 0b`
    * Add `b` suffix so we can specify bytes in --bwlimit, --min-size, etc.
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
    * Make `--include` rules add their implicit exclude * at the end of the filter list
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
    * Re-enable server-side copy
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
    * Stop SetModTime losing metadata (e.g. X-Object-Manifest)
        * This could have caused data loss for files > 5GB in size
    * Use ContentType from Object to avoid lookups in listings
* OneDrive
    * disable server-side copy as it seems to be broken at Microsoft

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
        * Use "golang.org/x/oauth2" as oauth library of choice
        * Improve oauth usability for smoother initial signup
        * drive, googlecloudstorage: optionally use auto config for the oauth token
    * Implement --dump-headers and --dump-bodies debug flags
    * Show multiple matched commands if abbreviation too short
    * Implement server-side move where possible
* local
    * Always use UNC paths internally on Windows - fixes a lot of bugs
* dropbox
    * force use of our custom transport which makes timeouts work
* Thanks to Klaus Post for lots of help with this release

## v1.19 - 2015-08-28

* New features
    * Server side copies for s3/swift/drive/dropbox/gcs
    * Move command - uses server-side copies if it can
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

* windows: Stop drive letters (e.g. C:) getting mixed up with remotes (e.g. drive:)
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

