---
date: 2019-10-26T11:04:03+01:00
title: "rclone dbhashsum"
slug: rclone_dbhashsum
url: /commands/rclone_dbhashsum/
---
## rclone dbhashsum

Produces a Dropbox hash file for all the objects in the path.

### Synopsis


Produces a Dropbox hash file for all the objects in the path.  The
hashes are calculated according to [Dropbox content hash
rules](https://www.dropbox.com/developers/reference/content-hash).
The output is in the same format as md5sum and sha1sum.


```
rclone dbhashsum remote:path [flags]
```

### Options

```
  -h, --help   help for dbhashsum
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone](/commands/rclone/)	 - Show help for rclone commands, flags and backends.

