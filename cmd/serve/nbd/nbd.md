Run a Network Block Device server using remote:path to store the object.

You can use a unix socket by setting the url to `unix:/path/to/socket`
or just by using an absolute path name.

`rclone serve nbd` will run on any OS, but the examples for using it
are Linux specific. There do exist Windows and macOS NBD clients but
these haven't been tested yet.

To see the packets on the wire use `--dump headers` or `--dump bodies`.

**NB** this has no authentication. It may in the future allow SSL
certificates. If you need access control then you will have to provide
it on the network layer, or use unix sockets.

### remote:path pointing to a file

If the `remote:path` points to a file then rclone will serve the file
directly as a network block device.

Using this with `--read-only` is recommended. You can use any
`--vfs-cache-mode` and only parts of the file that are read will be
cached locally if using `--vfs-cache-mode full`.

If you don't use `--read-only` then `--vfs-cache-mode full` is
required and the entire file will be cached locally and won't be
uploaded until the client has disconnected (`nbd-client -d`).

### remote:path pointing to a directory

If the `remote:path` points to a directory then rclone will treat the
directory as a place to store chunks of the exported network block device.

It will store an `info.json` file in the top level and store the
individual chunks in a hierarchical directory scheme with no more than
256 chunks or directories in any directory.

The first time you use this, you should use the `--create` flag
indicating how big you want the network block device to appear. Rclone
only allocates space you use so you can make this large. For example
`--create 1T`. You can also pass the `--chunk-size` flag at this
point. If you don't you will get the default of 64k chunks.

Rclone will then chunk the network block device into `--chunk-size`
chunks. Rclone has to download the entire chunk in order to change
only part of it and it will cache the chunk on disk so bear that in
mind when choosing `--chunk-size`.

If you wish to change the size of the network block device you can use
the `--resize` flag. This won't remove any data, it just changes the
size advertised. So if you have made a file system on the block device
you will need to resize it too.

If you are using `--read-only` then you can use any
`--vfs-cache-mode`.

If you are not using `--read-only` then you will need
`--vfs-cache-mode writes` or `--vfs-cache-mode full`.

Note that rclone will be acting as a writeback cache with
`--vfs-cache-mode writes` or `--vfs-cache-mode full`. Note that rclone
will only write `--transfers` files at once so the cache can get a
backlog of uploads. You can reduce the writeback caching slightly
setting `--vfs-write-back 0`, however due to the way the kernel works,
this will only reduce it slightly.

If using `--vfs-cache-mode writes` or `--vfs-cache-mode full` it is
recommended to set limits on the cache size using some or all of these
flags as the VFS can use a lot of disk space very quickly.

    --vfs-cache-max-age duration           Max time since last access of objects in the cache (default 1h0m0s)
    --vfs-cache-max-size SizeSuffix        Max total size of objects in the cache (default off)
    --vfs-cache-min-free-space SizeSuffix  Target minimum free space on the disk containing the cache (default off)

You might also need to set this smaller as the cache will only be
examined at this interval.

    --vfs-cache-poll-interval duration     Interval to poll the cache for stale objects (default 1m0s)

### Linux Examples

Install

    sudo apt install nbd-client

Start server on localhost:10809 by default.

    rclone -v --vfs-cache-mode full serve ndb remote:path

List devices

    sudo modprobe nbd
    sudo nbd-client --list localhost

Format the partition and mount read write

    sudo nbd-client -g localhost 10809 /dev/nbd0
    sudo mkfs.ext4 /dev/nbd0
    sudo mkdir -p /mnt/tmp
    sudo mount -t ext4 /dev/nbd0 /mnt/tmp

Mount read only

    rclone -v --vfs-cache-mode full --read-only serve ndb remote:path
    sudo nbd-client --readonly -g localhost 10809 /dev/nbd0
    sudo mount -t ext4 -o ro /dev/nbd0 /mnt/tmp

Disconnect

    sudo umount /mnt/tmp
    sudo nbd-client -d /dev/nbd0

### TODO

Experiment with `-connections` option. This is supported by the code.
Does it improve performance?

       -connections num

       -C     Use  num connections to the server, to allow speeding up request
              handling, at the cost of higher resource usage  on  the  server.
              Use  of this option requires kernel support available first with
              Linux 4.9.

Experiment with `-persist` option - is that a good idea?

       -persist

       -p     When  this  option is specified, nbd-client will immediately try
              to reconnect an nbd device if the connection  ever  drops  unex‐
              pectedly due to a lost server or something similar.

Need to implement Trim and see if Trim is being called.

Need to delete zero files before upload (do in VFS layer?)

FIXME need better back pressure from VFS cache to writers.

FIXME need Sync to actually work!
