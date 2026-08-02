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

### Multipart uploads

By default `serve s3` **streams** each multipart upload, in part-number
order, into a single `PutStream` upload to the underlying remote, so the
whole file is never buffered in memory - memory use stays bounded by the
parts in flight. The remote then performs its own internal upload (for
example its own multipart upload, still with bounded memory). This works
for any remote that supports `PutStream`, which is nearly all of them,
including through `crypt`.

The upload is atomic so the destination object only ever changes on a
successful completion. A failed or aborted upload never affects any
object already stored under that name. Remotes that upload atomically
already (object stores such as `s3`) are streamed straight to the
destination. On remotes where a partial upload would otherwise be visible
(such as `local`), the parts are streamed to a temporary object that is
moved into place, server-side, on completion; these remotes therefore
also need to support a server-side move or copy.

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
  name untouched, and a partly-uploaded object never becomes visible.
- Backend-agnostic - it only needs the remote to support `PutStream`
  (plus a server-side move or copy on remotes that don't upload
  atomically).

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
  effectively single-threaded, although the remote's own upload still
  runs concurrently.
- On remotes that don't upload atomically (such as `local`), the
  completed object is moved into place with a server-side operation.
  This is a cheap rename on most such remotes. On these remotes, if
  `serve s3` is killed part-way through an upload the temporary object
  (named with a leading `.rclone_multipart_upload_`) may be left behind;
  it is hidden from S3 listings but must be removed manually.

#### Disabling streaming

If you pass `--disable-multipart-streaming`, or the remote doesn't
support `PutStream` (or doesn't upload atomically and can't move or copy
server-side), multipart uploads are instead **buffered in memory**
by the underlying S3 library: every part is held in memory and the whole
object is written out in one go when the upload completes (the previous
behaviour). This removes the in-order/contiguous-part restriction above,
so parts can be uploaded in any order, but **memory use grows with the
size of the upload**, so it is only suitable for small objects. A one-off
`NOTICE` is logged the first time this happens.

Alternatively, if the client is an rclone `s3` remote (like the
`[serves3]` example above), you can set `use_multipart_uploads = false`
on it so it uploads each object as a single stream and skips multipart
uploads altogether.

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
