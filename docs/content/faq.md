---
title: "FAQ"
description: "Rclone Frequently Asked Questions"
date: "2015-08-27"
---

Frequently Asked Questions
--------------------------

### Do all cloud storage systems support all rclone commands ###

Yes they do.  All the rclone commands (eg `sync`, `copy` etc) will
work on all the remote storage systems.


### Can I copy the config from one machine to another ###

Sure!  Rclone stores all of its config in a single file.  If you want
to find this file, the simplest way is to run `rclone -h` and look at
the help for the `--config` flag which will tell you where it is. Eg,

```
$ rclone -h
Sync files and directories to and from local and remote object stores - v1.18.
[snip]
Options:
      --bwlimit=0: Bandwidth limit in kBytes/s, or use suffix k|M|G
      --checkers=8: Number of checkers to run in parallel.
  -c, --checksum=false: Skip based on checksum & size, not mod-time & size
      --config="/home/user/.rclone.conf": Config file.
[snip]
```

So in this config the config file can be found in
`/home/user/.rclone.conf`.

Just copy that to the equivalent place in the destination (run `rclone
-h` above again on the destination machine if not sure).


### Can rclone sync directly from drive to s3 ###

Rclone can sync between two remote cloud storage systems just fine.

Note that it effectively downloads the file and uploads it again, so
the node running rclone would need to have lots of bandwidth.

The syncs would be incremental (on a file by file basis).

Eg

    rclone sync drive:Folder s3:bucket


### Using rclone from multiple locations at the same time ###

You can use rclone from multiple places at the same time if you choose
different subdirectory for the output, eg

```
Server A> rclone sync /tmp/whatever remote:ServerA
Server B> rclone sync /tmp/whatever remote:ServerB
```

If you sync to the same directory then you should use rclone copy
otherwise the two rclones may delete each others files, eg

```
Server A> rclone copy /tmp/whatever remote:Backup
Server B> rclone copy /tmp/whatever remote:Backup
```

The file names you upload from Server A and Server B should be
different in this case, otherwise some file systems (eg Drive) may
make duplicates.

### Why doesn't rclone support partial transfers / binary diffs like rsync? ###

Rclone stores each file you transfer as a native object on the remote
cloud storage system.  This means that you can see the files you
upload as expected using alternative access methods (eg using the
Google Drive web interface).  There is a 1:1 mapping between files on
your hard disk and objects created in the cloud storage system.

Cloud storage systems (at least none I've come across yet) don't
support partially uploading an object. You can't take an existing
object, and change some bytes in the middle of it.

It would be possible to make a sync system which stored binary diffs
instead of whole objects like rclone does, but that would break the
1:1 mapping of files on your hard disk to objects in the remote cloud
storage system.

All the cloud storage systems support partial downloads of content, so
it would be possible to make partial downloads work.  However to make
this work efficiently this would require storing a significant amount
of metadata, which breaks the desired 1:1 mapping of files to objects.
