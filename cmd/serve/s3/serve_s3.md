`serve s3` implements a basic s3 server that serves a remote via s3.
This can be viewed with an s3 client, or you can make an [s3 type
remote](/s3/) to read and write to it with rclone.

`serve s3` is considered **Experimental** so use with care.

S3 server supports Signature Version 4 authentication. Just use
`--auth-key accessKey,secretKey` and set the `Authorization`
header correctly in the request. (See the [AWS
docs](https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html)).

`--auth-key` can be repeated for multiple auth pairs. If
`--auth-key` is not provided then `serve s3` will allow anonymous
access.

Alternatively `--auth-proxy` can be used to look up the secret for each
access key ID and choose the backend it maps to (see [Auth
Proxy](#auth-proxy) below). When an auth proxy is in use `--auth-key`
is ignored and every request must be signed with the secret the proxy
returns for its access key ID.

Like all rclone flags `--auth-key` can be set via environment
variables, in this case `RCLONE_AUTH_KEY`. Since this flag can be
repeated, the input to `RCLONE_AUTH_KEY` is CSV encoded. Because the
`accessKey,secretKey` has a comma in, this means it needs to be in
quotes.

```console
export RCLONE_AUTH_KEY='"user,pass"'
rclone serve s3 ...
```

Or to supply multiple identities:

```console
export RCLONE_AUTH_KEY='"user1,pass1","user2,pass2"'
rclone serve s3 ...
```

Setting this variable without quotes will produce an error.

Please note that some clients may require HTTPS endpoints. See [the
SSL docs](#tls-ssl) for more information.

This command uses the [VFS directory cache](#vfs-virtual-file-system).
All the functionality will work with `--vfs-cache-mode off`. Using
`--vfs-cache-mode full` (or `writes`) can be used to cache objects
locally to improve performance.

Use `--force-path-style=false` if you want to use the bucket name as a
part of the hostname (such as mybucket.local)

Use `--etag-hash` if you want to change the hash uses for the `ETag`.
Note that using anything other than `MD5` (the default) is likely to
cause problems for S3 clients which rely on the Etag being the MD5.

### Quickstart

For a simple set up, to serve `remote:path` over s3, run the server
like this:

```console
rclone serve s3 --auth-key ACCESS_KEY_ID,SECRET_ACCESS_KEY remote:path
```

For example, to use a simple folder in the filesystem, run the server
with a command like this:

```console
rclone serve s3 --auth-key ACCESS_KEY_ID,SECRET_ACCESS_KEY local:/path/to/folder
```

The `rclone.conf` for the server could look like this:

```ini
[local]
type = local
```

The `local` configuration is optional though. If you run the server with a
`remote:path` like `/path/to/folder` (without the `local:` prefix and without an
`rclone.conf` file), rclone will fall back to a default configuration, which
will be visible as a warning in the logs. But it will run nonetheless.

This will be compatible with an rclone (client) remote configuration which
is defined like this:

```ini
[serves3]
type = s3
provider = Rclone
endpoint = http://127.0.0.1:8080/
access_key_id = ACCESS_KEY_ID
secret_access_key = SECRET_ACCESS_KEY
```

### Object uploads (PUT)

A `PutObject` upload only ever changes the object at its key atomically, on
success, a failed or interrupted PUT neither removes nor overwrites the
object already stored at the key, and never leaves a partial object visible
at it.

Remotes that upload atomically (e.g. object stores such as `s3`) are streamed
straight to the destination. On remotes where a partial upload would
otherwise be visible (e.g. `local`), and whenever `--vfs-cache-mode` is
`writes` or above, the upload is written to a temporary object that is
renamed into place on success; these remotes need to support a server-side
move or copy for this (nearly all do - without move or copy the upload is
written directly and a failed PUT may leave a partial object at the key). If
`serve s3` is killed part-way through an upload the temporary object (named
with a leading `.rclone_temp_put_`) may be left behind; it is hidden from
S3 listings but must be removed manually.

### Multipart uploads

Multipart uploads are written, in part-number order, to a temporary
object which is renamed into place, server-side, on completion, so the
upload is atomic. The object at the key only ever changes on a
successful completion. A failed or aborted upload never affects any
object already stored under that name and a partly-uploaded object
never becomes visible under it.

With the default `--vfs-cache-mode off` `serve s3` **streams** each
multipart upload, in part-number order, into a single streaming upload
to the underlying remote, so the whole file is never buffered in
memory. Memory use stays bounded by the parts in flight. The remote
then performs its own internal upload (for example its own multipart
upload, still with bounded memory). Remotes that don't support
streaming uploads (those that must know the file size before the
upload starts, such as `onedrive`, `pcloud`, `jottacloud`, `mailru`,
`opendrive`, `putio`, `protondrive` and `zoho`) have the parts spooled
to a temporary file on **local disk** instead, and uploaded with the
size then known on completion, so they need local disk space for the
largest objects in flight rather than memory.

With `--vfs-cache-mode writes` (or `full`) the parts are written to a
temporary file in the VFS cache and uploaded by the VFS write-back -
see [Multipart uploads and the VFS
cache](#multipart-uploads-and-the-vfs-cache) below.

The rename into place needs the remote to support a server-side move
or copy, which nearly all do. It is a cheap rename on most remotes,
but on object stores without a real rename (such as `s3` itself) the
move is performed as a server-side copy and delete of the whole
object, which can take time and API calls for large objects.
Concurrent multipart uploads of the same key (which S3 permits) are
safe. Each writes its own temporary object and the last to complete
wins.

On the few remotes that support neither server side move nor copy, the
parts are written straight to the destination object instead and never
buffered in memory. This is at some cost in atomicity - the incomplete
object is visible under its final name while the upload is in flight,
as it also is for a plain object PUT on such remotes, and concurrent
multipart uploads of the same key write to the same object and can
interleave. A failed or aborted upload still leaves any pre-existing
object untouched provided the remote uploads atomically and the VFS
cache is off; on a remote where partial uploads are visible it may
leave partial data at the key (like a plain PUT there), and with
`--vfs-cache-mode writes` (or `full`) a write to the cache cannot be
abandoned, so an aborted upload's partial data is written back to the
remote as if it had completed.

**Features**

- The whole object is never buffered in memory; memory use is bounded by
  the parts in flight, not the upload size.
- Parts can be any size. Clients that don't produce uniform-sized parts
  work fine - for example PostgreSQL backup tools such as **pgBarman**
  and **pgBackRest**, which flush an upload buffer once it grows past
  the chunk size, so each part is the chunk size plus a variable
  overshoot.
- Works through `crypt` for any part size, since the object is encrypted
  as one continuous stream.
- The destination object only ever changes atomically, on completion: an
  aborted or failed upload leaves any pre-existing object of the same
  name untouched, and a partly-uploaded object never becomes visible
  (except on the few remotes with no server-side move or copy, as
  above).
- Multipart uploads go through the VFS like any other upload, so they
  show in rclone's transfer stats and obey `--bwlimit`.
- Backend-agnostic - it only needs the remote to support a server-side
  move or copy for the rename into place, which nearly all do; a remote
  without streaming upload support spools to local disk as above.

**Limitations**

- Parts must arrive in ascending, contiguous part-number order
  (1, 2, 3, ...). Parts the client uploads concurrently or out of order
  are buffered until their turn. The memory used for this buffering is
  capped, per upload, by `--multipart-streaming-buffer-limit` (default
  `256M`, `0` for no limit): a part that would take the buffer over the
  limit is stalled until the stream drains, so a client that uploads
  faster than the remote can accept sees backpressure rather than
  unbounded server memory use. Since a stalled part holds its HTTP
  request open, clients whose upload concurrency times chunk size
  exceeds the limit may need a longer read timeout when the remote is
  slow. Non-contiguous part numbers are rejected on completion.
  Configure the client to upload in part order, ideally with low
  concurrency, for the lowest memory use.
- A part uploaded again before completion - typically a client retrying
  after a timeout - is accepted: if the earlier copy is still buffered
  it is replaced, and if it has already been streamed an identical
  re-upload is a no-op. What isn't possible is replacing a part that has
  already been streamed with *different* content - that is rejected. A
  failure in the stream to the remote itself still aborts the whole
  upload and the client must start it again. (The remote's own upload
  still retries its internal chunks.)
- Parts are serialised into one stream, so ingest from the client is
  effectively single-threaded. When streaming, the remote's own upload
  runs concurrently with the parts arriving; with the local disk spool
  or the VFS cache the upload to the remote only starts on completion.
- If `serve s3` is killed part-way through an upload the temporary
  object (named with a leading `.rclone_temp_multipart_`) may be left
  behind; it is hidden from S3 listings but must be removed manually.

#### Multipart uploads and the VFS cache

With `--vfs-cache-mode writes` (or `full`) multipart uploads do not
stream to the remote at all. The parts are written, in part-number
order, to a temporary file in the VFS cache. On completion the file is
renamed into place and uploaded by the VFS write-back, exactly like a
plain object PUT. This needs no streaming upload support from the
remote. The rename normally happens in the cache before the upload has
started, but the VFS requires the remote to support a server-side move
or copy to rename files at all (and uses one if the temporary file has
already been written back, e.g. with `--vfs-write-back 0`). On remotes
without either, the parts are written to the cache directly under the
final key instead: the upload still never touches memory, but it loses
its atomicity - the in-flight upload is visible at the key, and an
aborted upload cannot be abandoned once in the cache, so its partial
data is written back to the remote as if it were a completed object.

Remotes that benefit from `--vfs-cache-mode writes`:

- **Remotes over slow or unreliable links.** A failure in a streamed
  upload aborts the whole multipart upload and the client must start
  again from the first part; a failed write-back upload is retried by
  the VFS (see `--vfs-cache-max-age` and friends) without the client
  being involved. Ingest from the client also runs at local disk speed
  rather than being throttled to the remote's pace.
- **Workloads that read back or overwrite what they just wrote.** The
  completed object stays in the cache, so subsequent `GET`/`HEAD`
  requests are served locally, and plain PUTs and multipart uploads to
  the same key go through the same cache entry so the last write wins
  regardless of upload style.

The trade-offs of the VFS cache:

- The whole object lands on local disk, so the cache (`--cache-dir`)
  needs space for the largest objects in flight; `--vfs-cache-max-size`
  cannot evict files which are still being uploaded.
- The `200 OK` for `CompleteMultipartUpload` means the data is safely
  in the **local cache**, not yet on the remote - the same durability
  the cache gives plain PUTs. If an acknowledgement must mean the data
  has reached the remote (for example WAL archiving), use the default
  `--vfs-cache-mode off`.
- The upload to the remote only starts on completion, rather than
  overlapping with the parts arriving, so the data reaches the remote
  later than with streaming.
- If `serve s3` is killed part-way through an upload, the temporary
  file survives in the cache and the VFS cache recovery uploads it to
  the remote on restart as a temporary object (named with a leading
  `.rclone_temp_multipart_`); as with the streaming path, it is
  hidden from S3 listings but must be removed manually.

#### Cleaning up temporary objects

If `serve s3` is killed part-way through an upload it can leave a
temporary object behind, named with a leading `.rclone_temp_`. This
whole prefix is reserved: any object whose name (the last
`/`-separated segment of its key) starts with `.rclone_temp_` is
hidden from S3 listings, so don't give real objects such names - an
existing object with such a name disappears from listings (though it
stays accessible directly by its key: only listings hide reserved
names, `GET`, `HEAD` and `DELETE` of the exact key still work). A
temporary object never holds acknowledged data - uploads whose
temporary object survived were never confirmed to the client - so old
ones are safe to delete:

    rclone delete --min-age 24h --include ".rclone_temp_*" remote:path

The `--min-age` protects uploads which are still in progress: make sure
it is longer than your longest upload, especially if several `serve s3`
instances share the same remote.

rclone v1.75 named its temporary multipart objects
`.rclone_multipart_upload_*`; leftovers from an older server are also
hidden from listings and can be cleaned up the same way.

#### Abandoned uploads

A client which starts a multipart upload and vanishes without either
completing or aborting it would otherwise hold on to its resources
forever.

An incomplete multipart upload which has had no activity for
`--multipart-expiry` (default `24h`) is therefore aborted and cleaned
up, exactly as if the client had called `AbortMultipartUpload`, and a
`NOTICE` is logged.

An upload with a part still being received is never expired, however
slowly the part is arriving, and each completed part restarts the
clock, so the expiry only needs to outlast the client's pauses
*between* parts, not the whole upload.

Late operations on an expired upload fail with `NoSuchUpload`, as they
do on real S3 when a lifecycle rule has aborted the upload. Set
`--multipart-expiry 0` to keep incomplete uploads forever.

#### Disabling streaming

If you pass `--disable-multipart-streaming`, multipart uploads are
instead **buffered in memory** by the underlying S3 library: every
part is held in memory and the whole object is written out in one go
when the upload completes. This removes the in-order/contiguous-part
restriction above, so parts can be uploaded in any order, but **memory
use grows with the size of the upload**, so it is only suitable for
small objects. A one-off `NOTICE` is logged the first time this
happens. This flag is the only thing that makes multipart uploads
buffer in memory - it is never done because of missing remote
capabilities. Consider `--vfs-cache-mode writes` instead, which
buffers the upload in the VFS cache on disk and takes precedence over
`--disable-multipart-streaming`.

### Bugs

Multipart server side copies do not work (see
[#7454](https://github.com/rclone/rclone/issues/7454)). These take a
very long time and eventually fail. The default threshold for
multipart server side copies is 5G which is the maximum it can be, so
files above this side will fail to be server side copied.

For a current list of `serve s3` bugs see the [serve
s3](https://github.com/rclone/rclone/labels/serve%20s3) bug category
on GitHub.

### Limitations

`serve s3` will treat all directories in the root as buckets and
ignore all files in the root. You can use `CreateBucket` to create
folders under the root, but you can't create empty folders under other
folders not in the root.

When using `PutObject` or `DeleteObject`, rclone will automatically
create or clean up empty folders. If you don't want to clean up empty
folders automatically, use `--no-cleanup`.

When using `ListObjects`, rclone will use `/` when the delimiter is
empty. This reduces backend requests with no effect on most
operations, but if the delimiter is something other than `/` and
empty, rclone will do a full recursive search of the backend, which
can take some time.

Versioning is not currently supported.

Metadata will only be saved in memory other than the rclone `mtime`
metadata which will be set as the modification time of the file.

### Object names

`serve s3` stores objects as files in the backend, so object keys are
mapped to file paths rather than treated as the opaque strings AWS S3
allows. Keys must be in canonical path form: keys that contain `..` or
`.` path segments, repeated slashes (`//`), or a leading or trailing
slash are rejected with a `400 Bad Request` (`InvalidArgument`)
instead of being normalised, since normalising them could alias two
distinct keys to the same file or resolve a key outside its bucket.
This matches the behaviour of other S3 servers such as MinIO.

### Supported operations

`serve s3` currently supports the following operations.

- Bucket
  - `ListBuckets`
  - `CreateBucket`
  - `DeleteBucket`
- Object
  - `HeadObject`
  - `ListObjects`
  - `GetObject`
  - `PutObject`
  - `DeleteObject`
  - `DeleteObjects`
  - `CreateMultipartUpload`
  - `CompleteMultipartUpload`
  - `AbortMultipartUpload`
  - `CopyObject`
  - `UploadPart`

Other operations will return error `Unimplemented`.
