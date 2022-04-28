---
title: "Bugs"
description: "Rclone Bugs and Limitations"
---

# Bugs and Limitations

## Limitations

### Directory timestamps aren't preserved

Rclone doesn't currently preserve the timestamps of directories.  This
is because rclone only really considers objects when syncing.

### Rclone struggles with millions of files in a directory/bucket

Currently rclone loads each directory/bucket entirely into memory before
using it.  Since each rclone object takes 0.5k-1k of memory this can take
a very long time and use a large amount of memory.

Millions of files in a directory tends to occur on bucket-based remotes
(e.g. S3 buckets) since those remotes do not segregate subdirectories within
the bucket.

### Bucket-based remotes and folders

Bucket-based remotes (e.g. S3/GCS/Swift/B2) do not have a concept of
directories.  Rclone therefore cannot create directories in them which
means that empty directories on a bucket-based remote will tend to
disappear.

Some software creates empty keys ending in `/` as directory markers.
Rclone doesn't do this as it potentially creates more objects and
costs more.  This ability may be added in the future (probably via a
flag/option).

## Bugs

Bugs are stored in rclone's GitHub project:

* [Reported bugs](https://github.com/rclone/rclone/issues?q=is%3Aopen+is%3Aissue+label%3Abug)
* [Known issues](https://github.com/rclone/rclone/issues?q=is%3Aopen+is%3Aissue+milestone%3A%22Known+Problem%22)

