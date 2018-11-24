---
date: 2018-11-24T13:43:29Z
title: "rclone rcat"
slug: rclone_rcat
url: /commands/rclone_rcat/
---
## rclone rcat

Copies standard input to file on remote.

### Synopsis


rclone rcat reads from standard input (stdin) and copies it to a
single remote file.

    echo "hello world" | rclone rcat remote:path/to/file
    ffmpeg - | rclone rcat remote:path/to/file

If the remote file already exists, it will be overwritten.

rcat will try to upload small files in a single request, which is
usually more efficient than the streaming/chunked upload endpoints,
which use multiple requests. Exact behaviour depends on the remote.
What is considered a small file may be set through
`--streaming-upload-cutoff`. Uploading only starts after
the cutoff is reached or if the file ends before that. The data
must fit into RAM. The cutoff needs to be small enough to adhere
the limits of your remote, please see there. Generally speaking,
setting this cutoff too high will decrease your performance.

Note that the upload can also not be retried because the data is
not kept around until the upload succeeds. If you need to transfer
a lot of data, you're better off caching locally and then
`rclone move` it to the destination.

```
rclone rcat remote:path [flags]
```

### Options

```
  -h, --help   help for rcat
```

### Options inherited from parent commands

```
      --acd-auth-url string                        Auth server URL.
      --acd-client-id string                       Amazon Application Client ID.
      --acd-client-secret string                   Amazon Application Client Secret.
      --acd-templink-threshold SizeSuffix          Files >= this size will be downloaded via their tempLink. (default 9G)
      --acd-token-url string                       Token server url.
      --acd-upload-wait-per-gb Duration            Additional time per GB to wait after a failed complete upload to see if it appears. (default 3m0s)
      --alias-remote string                        Remote or path to alias.
      --ask-password                               Allow prompt for password for encrypted configuration. (default true)
      --auto-confirm                               If enabled, do not request console confirmation.
      --azureblob-access-tier string               Access tier of blob: hot, cool or archive.
      --azureblob-account string                   Storage Account Name (leave blank to use connection string or SAS URL)
      --azureblob-chunk-size SizeSuffix            Upload chunk size (<= 100MB). (default 4M)
      --azureblob-endpoint string                  Endpoint for the service
      --azureblob-key string                       Storage Account Key (leave blank to use connection string or SAS URL)
      --azureblob-list-chunk int                   Size of blob list. (default 5000)
      --azureblob-sas-url string                   SAS URL for container level access only
      --azureblob-upload-cutoff SizeSuffix         Cutoff for switching to chunked upload (<= 256MB). (default 256M)
      --b2-account string                          Account ID or Application Key ID
      --b2-chunk-size SizeSuffix                   Upload chunk size. Must fit in memory. (default 96M)
      --b2-endpoint string                         Endpoint for the service.
      --b2-hard-delete                             Permanently delete files on remote removal, otherwise hide files.
      --b2-key string                              Application Key
      --b2-test-mode string                        A flag string for X-Bz-Test-Mode header for debugging.
      --b2-upload-cutoff SizeSuffix                Cutoff for switching to chunked upload. (default 200M)
      --b2-versions                                Include old versions in directory listings.
      --backup-dir string                          Make backups into hierarchy based in DIR.
      --bind string                                Local address to bind to for outgoing connections, IPv4, IPv6 or name.
      --box-client-id string                       Box App Client Id.
      --box-client-secret string                   Box App Client Secret
      --box-commit-retries int                     Max number of times to try committing a multipart file. (default 100)
      --box-upload-cutoff SizeSuffix               Cutoff for switching to multipart upload (>= 50MB). (default 50M)
      --buffer-size int                            In memory buffer size when reading files for each --transfer. (default 16M)
      --bwlimit BwTimetable                        Bandwidth limit in kBytes/s, or use suffix b|k|M|G or a full timetable.
      --cache-chunk-clean-interval Duration        How often should the cache perform cleanups of the chunk storage. (default 1m0s)
      --cache-chunk-no-memory                      Disable the in-memory cache for storing chunks during streaming.
      --cache-chunk-path string                    Directory to cache chunk files. (default "$HOME/.cache/rclone/cache-backend")
      --cache-chunk-size SizeSuffix                The size of a chunk (partial file data). (default 5M)
      --cache-chunk-total-size SizeSuffix          The total size that the chunks can take up on the local disk. (default 10G)
      --cache-db-path string                       Directory to store file structure metadata DB. (default "$HOME/.cache/rclone/cache-backend")
      --cache-db-purge                             Clear all the cached data for this remote on start.
      --cache-db-wait-time Duration                How long to wait for the DB to be available - 0 is unlimited (default 1s)
      --cache-dir string                           Directory rclone will use for caching. (default "$HOME/.cache/rclone")
      --cache-info-age Duration                    How long to cache file structure information (directory listings, file size, times etc). (default 6h0m0s)
      --cache-plex-insecure string                 Skip all certificate verifications when connecting to the Plex server
      --cache-plex-password string                 The password of the Plex user
      --cache-plex-url string                      The URL of the Plex server
      --cache-plex-username string                 The username of the Plex user
      --cache-read-retries int                     How many times to retry a read from a cache storage. (default 10)
      --cache-remote string                        Remote to cache.
      --cache-rps int                              Limits the number of requests per second to the source FS (-1 to disable) (default -1)
      --cache-tmp-upload-path string               Directory to keep temporary files until they are uploaded.
      --cache-tmp-wait-time Duration               How long should files be stored in local cache before being uploaded (default 15s)
      --cache-workers int                          How many workers should run in parallel to download chunks. (default 4)
      --cache-writes                               Cache file data on writes through the FS
      --checkers int                               Number of checkers to run in parallel. (default 8)
  -c, --checksum                                   Skip based on checksum & size, not mod-time & size
      --config string                              Config file. (default "/home/ncw/.rclone.conf")
      --contimeout duration                        Connect timeout (default 1m0s)
  -L, --copy-links                                 Follow symlinks and copy the pointed to item.
      --cpuprofile string                          Write cpu profile to file
      --crypt-directory-name-encryption            Option to either encrypt directory names or leave them intact. (default true)
      --crypt-filename-encryption string           How to encrypt the filenames. (default "standard")
      --crypt-password string                      Password or pass phrase for encryption.
      --crypt-password2 string                     Password or pass phrase for salt. Optional but recommended.
      --crypt-remote string                        Remote to encrypt/decrypt.
      --crypt-show-mapping                         For all files listed show how the names encrypt.
      --delete-after                               When synchronizing, delete files on destination after transferring (default)
      --delete-before                              When synchronizing, delete files on destination before transferring
      --delete-during                              When synchronizing, delete files during transfer
      --delete-excluded                            Delete files on dest excluded from sync
      --disable string                             Disable a comma separated list of features.  Use help to see a list.
      --drive-acknowledge-abuse                    Set to allow files which return cannotDownloadAbusiveFile to be downloaded.
      --drive-allow-import-name-change             Allow the filetype to change when uploading Google docs (e.g. file.doc to file.docx). This will confuse sync and reupload every time.
      --drive-alternate-export                     Use alternate export URLs for google documents export.,
      --drive-auth-owner-only                      Only consider files owned by the authenticated user.
      --drive-chunk-size SizeSuffix                Upload chunk size. Must a power of 2 >= 256k. (default 8M)
      --drive-client-id string                     Google Application Client Id
      --drive-client-secret string                 Google Application Client Secret
      --drive-export-formats string                Comma separated list of preferred formats for downloading Google docs. (default "docx,xlsx,pptx,svg")
      --drive-formats string                       Deprecated: see export_formats
      --drive-impersonate string                   Impersonate this user when using a service account.
      --drive-import-formats string                Comma separated list of preferred formats for uploading Google docs.
      --drive-keep-revision-forever                Keep new head revision of each file forever.
      --drive-list-chunk int                       Size of listing chunk 100-1000. 0 to disable. (default 1000)
      --drive-root-folder-id string                ID of the root folder
      --drive-scope string                         Scope that rclone should use when requesting access from drive.
      --drive-service-account-credentials string   Service Account Credentials JSON blob
      --drive-service-account-file string          Service Account Credentials JSON file path
      --drive-shared-with-me                       Only show files that are shared with me.
      --drive-skip-gdocs                           Skip google documents in all listings.
      --drive-team-drive string                    ID of the Team Drive
      --drive-trashed-only                         Only show files that are in the trash.
      --drive-upload-cutoff SizeSuffix             Cutoff for switching to chunked upload (default 8M)
      --drive-use-created-date                     Use file created date instead of modified date.,
      --drive-use-trash                            Send files to the trash instead of deleting permanently. (default true)
      --drive-v2-download-min-size SizeSuffix      If Object's are greater, use drive v2 API to download. (default off)
      --dropbox-chunk-size SizeSuffix              Upload chunk size. (< 150M). (default 48M)
      --dropbox-client-id string                   Dropbox App Client Id
      --dropbox-client-secret string               Dropbox App Client Secret
      --dropbox-impersonate string                 Impersonate this user when using a business account.
  -n, --dry-run                                    Do a trial run with no permanent changes
      --dump string                                List of items to dump from: headers,bodies,requests,responses,auth,filters,goroutines,openfiles
      --dump-bodies                                Dump HTTP headers and bodies - may contain sensitive info
      --dump-headers                               Dump HTTP bodies - may contain sensitive info
      --exclude stringArray                        Exclude files matching pattern
      --exclude-from stringArray                   Read exclude patterns from file
      --exclude-if-present string                  Exclude directories if filename is present
      --fast-list                                  Use recursive list if available. Uses more memory but fewer transactions.
      --files-from stringArray                     Read list of source-file names from file
  -f, --filter stringArray                         Add a file-filtering rule
      --filter-from stringArray                    Read filtering patterns from a file
      --ftp-host string                            FTP host to connect to
      --ftp-pass string                            FTP password
      --ftp-port string                            FTP port, leave blank to use default (21)
      --ftp-user string                            FTP username, leave blank for current username, $USER
      --gcs-bucket-acl string                      Access Control List for new buckets.
      --gcs-client-id string                       Google Application Client Id
      --gcs-client-secret string                   Google Application Client Secret
      --gcs-location string                        Location for the newly created buckets.
      --gcs-object-acl string                      Access Control List for new objects.
      --gcs-project-number string                  Project number.
      --gcs-service-account-file string            Service Account Credentials JSON file path
      --gcs-storage-class string                   The storage class to use when storing objects in Google Cloud Storage.
      --http-url string                            URL of http host to connect to
      --hubic-chunk-size SizeSuffix                Above this size files will be chunked into a _segments container. (default 5G)
      --hubic-client-id string                     Hubic Client Id
      --hubic-client-secret string                 Hubic Client Secret
      --ignore-case                                Ignore case in filters (case insensitive)
      --ignore-checksum                            Skip post copy check of checksums.
      --ignore-errors                              delete even if there are I/O errors
      --ignore-existing                            Skip all files that exist on destination
      --ignore-size                                Ignore size when skipping use mod-time or checksum.
  -I, --ignore-times                               Don't skip files that match size and time - transfer all files
      --immutable                                  Do not modify files. Fail if existing files have been modified.
      --include stringArray                        Include files matching pattern
      --include-from stringArray                   Read include patterns from file
      --jottacloud-hard-delete                     Delete files permanently rather than putting them into the trash.
      --jottacloud-md5-memory-limit SizeSuffix     Files bigger than this will be cached on disk to calculate the MD5 if required. (default 10M)
      --jottacloud-mountpoint string               The mountpoint to use.
      --jottacloud-pass string                     Password.
      --jottacloud-unlink                          Remove existing public link to file/folder with link command rather than creating.
      --jottacloud-user string                     User Name
      --local-no-check-updated                     Don't check to see if the files change during upload
      --local-no-unicode-normalization             Don't apply unicode normalization to paths and filenames (Deprecated)
      --local-nounc string                         Disable UNC (long path names) conversion on Windows
      --log-file string                            Log everything to this file
      --log-format string                          Comma separated list of log format options (default "date,time")
      --log-level string                           Log level DEBUG|INFO|NOTICE|ERROR (default "NOTICE")
      --low-level-retries int                      Number of low level retries to do. (default 10)
      --max-age duration                           Only transfer files younger than this in s or suffix ms|s|m|h|d|w|M|y (default off)
      --max-backlog int                            Maximum number of objects in sync or check backlog. (default 10000)
      --max-delete int                             When synchronizing, limit the number of deletes (default -1)
      --max-depth int                              If set limits the recursion depth to this. (default -1)
      --max-size int                               Only transfer files smaller than this in k or suffix b|k|M|G (default off)
      --max-transfer int                           Maximum size of data to transfer. (default off)
      --mega-debug                                 Output more debug from Mega.
      --mega-hard-delete                           Delete files permanently rather than putting them into the trash.
      --mega-pass string                           Password.
      --mega-user string                           User name
      --memprofile string                          Write memory profile to file
      --min-age duration                           Only transfer files older than this in s or suffix ms|s|m|h|d|w|M|y (default off)
      --min-size int                               Only transfer files bigger than this in k or suffix b|k|M|G (default off)
      --modify-window duration                     Max time diff to be considered the same (default 1ns)
      --no-check-certificate                       Do not verify the server SSL certificate. Insecure.
      --no-gzip-encoding                           Don't set Accept-Encoding: gzip.
      --no-traverse                                Obsolete - does nothing.
      --no-update-modtime                          Don't update destination mod-time if files identical.
  -x, --one-file-system                            Don't cross filesystem boundaries (unix/macOS only).
      --onedrive-chunk-size SizeSuffix             Chunk size to upload files with - must be multiple of 320k. (default 10M)
      --onedrive-client-id string                  Microsoft App Client Id
      --onedrive-client-secret string              Microsoft App Client Secret
      --onedrive-drive-id string                   The ID of the drive to use
      --onedrive-drive-type string                 The type of the drive ( personal | business | documentLibrary )
      --onedrive-expose-onenote-files              Set to make OneNote files show up in directory listings.
      --opendrive-password string                  Password.
      --opendrive-username string                  Username
      --pcloud-client-id string                    Pcloud App Client Id
      --pcloud-client-secret string                Pcloud App Client Secret
  -P, --progress                                   Show progress during transfer.
      --qingstor-access-key-id string              QingStor Access Key ID
      --qingstor-connection-retries int            Number of connection retries. (default 3)
      --qingstor-endpoint string                   Enter a endpoint URL to connection QingStor API.
      --qingstor-env-auth                          Get QingStor credentials from runtime. Only applies if access_key_id and secret_access_key is blank.
      --qingstor-secret-access-key string          QingStor Secret Access Key (password)
      --qingstor-zone string                       Zone to connect to.
  -q, --quiet                                      Print as little stuff as possible
      --rc                                         Enable the remote control server.
      --rc-addr string                             IPaddress:Port or :Port to bind server to. (default "localhost:5572")
      --rc-cert string                             SSL PEM key (concatenation of certificate and CA certificate)
      --rc-client-ca string                        Client certificate authority to verify clients with
      --rc-files string                            Path to local files to serve on the HTTP server.
      --rc-htpasswd string                         htpasswd file - if not provided no authentication is done
      --rc-key string                              SSL PEM Private key
      --rc-max-header-bytes int                    Maximum size of request header (default 4096)
      --rc-no-auth                                 Don't require auth for certain methods.
      --rc-pass string                             Password for authentication.
      --rc-realm string                            realm for authentication (default "rclone")
      --rc-serve                                   Enable the serving of remote objects.
      --rc-server-read-timeout duration            Timeout for server reading data (default 1h0m0s)
      --rc-server-write-timeout duration           Timeout for server writing data (default 1h0m0s)
      --rc-user string                             User name for authentication.
      --retries int                                Retry operations this many times if they fail (default 3)
      --retries-sleep duration                     Interval between retrying operations if they fail, e.g 500ms, 60s, 5m. (0 to disable)
      --s3-access-key-id string                    AWS Access Key ID.
      --s3-acl string                              Canned ACL used when creating buckets and storing or copying objects.
      --s3-chunk-size SizeSuffix                   Chunk size to use for uploading. (default 5M)
      --s3-disable-checksum                        Don't store MD5 checksum with object metadata
      --s3-endpoint string                         Endpoint for S3 API.
      --s3-env-auth                                Get AWS credentials from runtime (environment variables or EC2/ECS meta data if no env vars).
      --s3-force-path-style                        If true use path style access if false use virtual hosted style. (default true)
      --s3-location-constraint string              Location constraint - must be set to match the Region.
      --s3-provider string                         Choose your S3 provider.
      --s3-region string                           Region to connect to.
      --s3-secret-access-key string                AWS Secret Access Key (password)
      --s3-server-side-encryption string           The server-side encryption algorithm used when storing this object in S3.
      --s3-session-token string                    An AWS session token
      --s3-sse-kms-key-id string                   If using KMS ID you must provide the ARN of Key.
      --s3-storage-class string                    The storage class to use when storing new objects in S3.
      --s3-upload-concurrency int                  Concurrency for multipart uploads. (default 2)
      --s3-v2-auth                                 If true use v2 authentication.
      --sftp-ask-password                          Allow asking for SFTP password when needed.
      --sftp-disable-hashcheck                     Disable the execution of SSH commands to determine if remote file hashing is available.
      --sftp-host string                           SSH host to connect to
      --sftp-key-file string                       Path to unencrypted PEM-encoded private key file, leave blank to use ssh-agent.
      --sftp-pass string                           SSH password, leave blank to use ssh-agent.
      --sftp-path-override string                  Override path used by SSH connection.
      --sftp-port string                           SSH port, leave blank to use default (22)
      --sftp-set-modtime                           Set the modified time on the remote if set. (default true)
      --sftp-use-insecure-cipher                   Enable the use of the aes128-cbc cipher. This cipher is insecure and may allow plaintext data to be recovered by an attacker.
      --sftp-user string                           SSH username, leave blank for current username, ncw
      --size-only                                  Skip based on size only, not mod-time or checksum
      --skip-links                                 Don't warn about skipped symlinks.
      --stats duration                             Interval between printing stats, e.g 500ms, 60s, 5m. (0 to disable) (default 1m0s)
      --stats-file-name-length int                 Max file name length in stats. 0 for no limit (default 40)
      --stats-log-level string                     Log level to show --stats output DEBUG|INFO|NOTICE|ERROR (default "INFO")
      --stats-one-line                             Make the stats fit on one line.
      --stats-unit string                          Show data rate in stats as either 'bits' or 'bytes'/s (default "bytes")
      --streaming-upload-cutoff int                Cutoff for switching to chunked upload if file size is unknown. Upload starts after reaching cutoff or when file ends. (default 100k)
      --suffix string                              Suffix for use with --backup-dir.
      --swift-auth string                          Authentication URL for server (OS_AUTH_URL).
      --swift-auth-token string                    Auth Token from alternate authentication - optional (OS_AUTH_TOKEN)
      --swift-auth-version int                     AuthVersion - optional - set to (1,2,3) if your auth URL has no version (ST_AUTH_VERSION)
      --swift-chunk-size SizeSuffix                Above this size files will be chunked into a _segments container. (default 5G)
      --swift-domain string                        User domain - optional (v3 auth) (OS_USER_DOMAIN_NAME)
      --swift-endpoint-type string                 Endpoint type to choose from the service catalogue (OS_ENDPOINT_TYPE) (default "public")
      --swift-env-auth                             Get swift credentials from environment variables in standard OpenStack form.
      --swift-key string                           API key or password (OS_PASSWORD).
      --swift-region string                        Region name - optional (OS_REGION_NAME)
      --swift-storage-policy string                The storage policy to use when creating a new container
      --swift-storage-url string                   Storage URL - optional (OS_STORAGE_URL)
      --swift-tenant string                        Tenant name - optional for v1 auth, this or tenant_id required otherwise (OS_TENANT_NAME or OS_PROJECT_NAME)
      --swift-tenant-domain string                 Tenant domain - optional (v3 auth) (OS_PROJECT_DOMAIN_NAME)
      --swift-tenant-id string                     Tenant ID - optional for v1 auth, this or tenant required otherwise (OS_TENANT_ID)
      --swift-user string                          User name to log in (OS_USERNAME).
      --swift-user-id string                       User ID to log in - optional - most swift systems use user and leave this blank (v3 auth) (OS_USER_ID).
      --syslog                                     Use Syslog for logging
      --syslog-facility string                     Facility for syslog, eg KERN,USER,... (default "DAEMON")
      --timeout duration                           IO idle timeout (default 5m0s)
      --tpslimit float                             Limit HTTP transactions per second to this.
      --tpslimit-burst int                         Max burst of transactions for --tpslimit. (default 1)
      --track-renames                              When synchronizing, track file renames and do a server side move if possible
      --transfers int                              Number of file transfers to run in parallel. (default 4)
      --union-remotes string                       List of space separated remotes.
  -u, --update                                     Skip files that are newer on the destination.
      --use-server-modtime                         Use server modified time instead of object metadata
      --user-agent string                          Set the user-agent to a specified string. The default is rclone/ version (default "rclone/v1.45")
  -v, --verbose count                              Print lots more stuff (repeat for more)
      --webdav-bearer-token string                 Bearer token instead of user/pass (eg a Macaroon)
      --webdav-pass string                         Password.
      --webdav-url string                          URL of http host to connect to
      --webdav-user string                         User name
      --webdav-vendor string                       Name of the Webdav site/service/software you are using
      --yandex-client-id string                    Yandex Client Id
      --yandex-client-secret string                Yandex Client Secret
      --yandex-unlink                              Remove existing public link to file/folder with link command rather than creating.
```

### SEE ALSO

* [rclone](/commands/rclone/)	 - Show help for rclone commands, flags and backends.

###### Auto generated by spf13/cobra on 24-Nov-2018
