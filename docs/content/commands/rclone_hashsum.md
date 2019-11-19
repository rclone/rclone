---
date: 2019-11-19T16:02:36Z
title: "rclone hashsum"
slug: rclone_hashsum
url: /commands/rclone_hashsum/
---
## rclone hashsum

Produces an hashsum file for all the objects in the path.

### Synopsis


Produces a hash file for all the objects in the path using the hash
named.  The output is in the same format as the standard
md5sum/sha1sum tool.

Run without a hash to see the list of supported hashes, eg

    $ rclone hashsum
    Supported hashes are:
      * MD5
      * SHA-1
      * DropboxHash
      * QuickXorHash

Then

    $ rclone hashsum MD5 remote:path


```
rclone hashsum <hash> remote:path [flags]
```

### Options

```
  -h, --help   help for hashsum
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone](/commands/rclone/)	 - Show help for rclone commands, flags and backends.

