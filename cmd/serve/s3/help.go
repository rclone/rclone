package s3

var longHelp = `
Serve s3 implements a basic s3 server that serves a remote 
via s3. This can be viewed with an s3 client, or you can make
an s3 type remote to read and write to it.

S3 server supports Signature Version 4 authentication. Just
 use ` + `--s3-authkey accessKey1,secretKey1` + ` and
 set  Authorization Header correctly in the request. (See 
https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html)

Please note that some clients may require HTTPS endpoints. 
See [#SSL](#ssl-tls) for SSL configuration.

Use ` + `--force-path-style=false` + ` if you want to use bucket name as a part of 
hostname (such as mybucket.local)

Use ` + `--etag-hash` + ` if you want to change hash provider.

Limitations

serve s3 will treat all depth=1 directories in root as buckets and
 ignore files in that depth. You might use CreateBucket to create 
folders under root, but you can't create empty folder under other folders.

When using PutObject or DeleteObject, rclone will automatically create 
or clean up empty folders by the prefix. If you don't want to clean up 
empty folders automatically, use ` + `--no-cleanup` + `.

When using ListObjects, rclone will use ` + `/` + ` when the delimiter is empty. 
This reduces backend requests with no effect on most operations, but if 
the delimiter is something other than slash and nil, rclone will do a 
full recursive search to the backend, which may take some time.

serve s3 currently supports the following operations.
Bucket-level operations
ListBuckets, CreateBucket, DeleteBucket

Object-level operations
HeadObject, ListObjects, GetObject, PutObject, DeleteObject, DeleteObjects, 
CreateMultipartUpload, CompleteMultipartUpload, AbortMultipartUpload, 
CopyObject, UploadPart
Other operations will encounter error Unimplemented.
`
