---
date: 2019-06-15T12:00:42+01:00
title: "rclone serve http"
slug: rclone_serve_http
url: /commands/rclone_serve_http/
---
## rclone serve http

Serve the remote over HTTP.

### Synopsis

rclone serve http implements a basic web server to serve the remote
over HTTP.  This can be viewed in a web browser or you can make a
remote of type http read from it.

You can use the filter flags (eg --include, --exclude) to control what
is served.

The server will log errors.  Use -v to see access logs.

--bwlimit will be respected for file transfers.  Use --stats to
control the stats printing.

### Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.  You can use port
:0 to let the OS choose an available port.

If you set --addr to listen on a public or LAN accessible IP address
then using Authentication is advised - see the next section for info.

--server-read-timeout and --server-write-timeout can be used to
control the timeouts on the server.  Note that this is the total time
for a transfer.

--max-header-bytes controls the maximum number of bytes the server will
accept in the HTTP header.

#### Authentication

By default this will serve files without needing a login.

You can either use an htpasswd file which can take lots of users, or
set a single username and password with the --user and --pass flags.

Use --htpasswd /path/to/htpasswd to provide an htpasswd file.  This is
in standard apache format and supports MD5, SHA1 and BCrypt for basic
authentication.  Bcrypt is recommended.

To create an htpasswd file:

    touch htpasswd
    htpasswd -B htpasswd user
    htpasswd -B htpasswd anotherUser

The password file can be updated while rclone is running.

Use --realm to set the authentication realm.

#### SSL/TLS

By default this will serve over http.  If you want you can serve over
https.  You will need to supply the --cert and --key flags.  If you
wish to do client side certificate validation then you will need to
supply --client-ca also.

--cert should be a either a PEM encoded certificate or a concatenation
of that with the CA certificate.  --key should be the PEM encoded
private key and --client-ca should be the PEM encoded client
certificate authority certificate.

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

### File Buffering

The `--buffer-size` flag determines the amount of memory,
that will be used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of
data in memory at all times. The buffered data is bound to one file
descriptor and won't be shared between multiple open file descriptors
of the same file.

This flag is a upper limit for the used memory per file descriptor.
The buffer will only use memory for data that is downloaded but not
not yet read. If the buffer is empty, only a small amount of memory
will be used.
The maximum memory used by rclone for buffering can be up to
`--buffer-size * open files`.

### File Caching

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
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

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
rclone serve http remote:path [flags]
```

### Options

```
      --addr string                            IPaddress:Port or :Port to bind server to. (default "localhost:8080")
      --cert string                            SSL PEM key (concatenation of certificate and CA certificate)
      --client-ca string                       Client certificate authority to verify clients with
      --dir-cache-time duration                Time to cache directory entries for. (default 5m0s)
      --dir-perms FileMode                     Directory permissions (default 0777)
      --file-perms FileMode                    File permissions (default 0666)
      --gid uint32                             Override the gid field set by the filesystem. (default 1000)
  -h, --help                                   help for http
      --htpasswd string                        htpasswd file - if not provided no authentication is done
      --key string                             SSL PEM Private key
      --max-header-bytes int                   Maximum size of request header (default 4096)
      --no-checksum                            Don't compare checksums on up/download.
      --no-modtime                             Don't read/write the modification time (can speed things up).
      --no-seek                                Don't allow seeking in files.
      --pass string                            Password for authentication.
      --poll-interval duration                 Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable. (default 1m0s)
      --read-only                              Mount read-only.
      --realm string                           realm for authentication (default "rclone")
      --server-read-timeout duration           Timeout for server reading data (default 1h0m0s)
      --server-write-timeout duration          Timeout for server writing data (default 1h0m0s)
      --uid uint32                             Override the uid field set by the filesystem. (default 1000)
      --umask int                              Override the permission bits set by the filesystem. (default 2)
      --user string                            User name for authentication.
      --vfs-cache-max-age duration             Max age of objects in the cache. (default 1h0m0s)
      --vfs-cache-max-size SizeSuffix          Max total size of objects in the cache. (default off)
      --vfs-cache-mode CacheMode               Cache mode off|minimal|writes|full (default off)
      --vfs-cache-poll-interval duration       Interval to poll the cache for stale objects. (default 1m0s)
      --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
      --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)
```

### SEE ALSO

* [rclone serve](/commands/rclone_serve/)	 - Serve a remote over a protocol.

###### Auto generated by spf13/cobra on 15-Jun-2019
