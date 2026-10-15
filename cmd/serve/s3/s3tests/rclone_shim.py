"""pytest plugin adapting the s3-tests fixtures to rclone serve s3.

It only changes how the tests clean up after themselves, never what
they test.
"""
from botocore.exceptions import ClientError
import s3tests.functional as f

_list_versions = f.list_versions
_nuke_bucket = f.nuke_bucket


def list_versions(client, bucket, batch_size):
    """List objects to delete, falling back to ListObjectsV2 if
    ListObjectVersions isn't implemented."""
    try:
        yield from _list_versions(client, bucket, batch_size)
        return
    except ClientError as e:
        if e.response['Error']['Code'] != 'NotImplemented':
            raise
    paginator = client.get_paginator('list_objects_v2')
    for page in paginator.paginate(Bucket=bucket, PaginationConfig={'PageSize': batch_size}):
        objs = page.get('Contents', [])
        if objs:
            yield [{'Key': o['Key']} for o in objs]


def nuke_bucket(client, bucket):
    """Delete bucket after aborting its incomplete multipart uploads,
    which the s3-tests cleanup leaves behind."""
    try:
        for page in client.get_paginator('list_multipart_uploads').paginate(Bucket=bucket):
            for u in page.get('Uploads', []):
                client.abort_multipart_upload(Bucket=bucket, Key=u['Key'], UploadId=u['UploadId'])
    except ClientError:
        pass
    _nuke_bucket(client, bucket)


# Buckets which couldn't be deleted. Each fails the teardown of the
# test which left it, once; later tests skip it rather than every one
# of them erroring in setup.
_stuck = set()


def nuke_prefixed_buckets(prefix, client=None):
    """Delete all the buckets starting with prefix."""
    if client is None:
        client = f.get_client()
    err = None
    for name in f.get_buckets_list(client, prefix):
        if name in _stuck:
            continue
        try:
            nuke_bucket(client, name)
        except Exception as e:
            _stuck.add(name)
            err = e
    if err:
        raise err


f.list_versions = list_versions
f.nuke_bucket = nuke_bucket
f.nuke_prefixed_buckets = nuke_prefixed_buckets
