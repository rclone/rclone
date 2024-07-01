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

Please note that some clients may require HTTPS endpoints. See [the
SSL docs](#ssl-tls) for more information.

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

```
rclone serve s3 --auth-key ACCESS_KEY_ID,SECRET_ACCESS_KEY remote:path
```

For example, to use a simple folder in the filesystem, run the server
with a command like this:

```
rclone serve s3 --auth-key ACCESS_KEY_ID,SECRET_ACCESS_KEY local:/path/to/folder
```

The `rclone.conf` for the server could look like this:

```
[local]
type = local
```

The `local` configuration is optional though. If you run the server with a
`remote:path` like `/path/to/folder` (without the `local:` prefix and without an
`rclone.conf` file), rclone will fall back to a default configuration, which
will be visible as a warning in the logs. But it will run nonetheless.

This will be compatible with an rclone (client) remote configuration which
is defined like this:

```
[serves3]
type = s3
provider = Rclone
endpoint = http://127.0.0.1:8080/
access_key_id = ACCESS_KEY_ID
secret_access_key = SECRET_ACCESS_KEY
use_multipart_uploads = false
```

Note that setting `disable_multipart_uploads = true` is to work around
[a bug](#bugs) which will be fixed in due course.

### Bugs

When uploading multipart files `serve s3` holds all the parts in
memory (see [#7453](https://github.com/rclone/rclone/issues/7453)).
This is a limitaton of the library rclone uses for serving S3 and will
hopefully be fixed at some point.

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
