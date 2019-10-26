---
date: 2019-10-26T11:04:03+01:00
title: "rclone link"
slug: rclone_link
url: /commands/rclone_link/
---
## rclone link

Generate public link to file/folder.

### Synopsis


rclone link will create or retrieve a public link to the given file or folder.

    rclone link remote:path/to/file
    rclone link remote:path/to/folder/

If successful, the last line of the output will contain the link. Exact
capabilities depend on the remote, but the link will always be created with
the least constraints – e.g. no expiry, no password protection, accessible
without account.


```
rclone link remote:path [flags]
```

### Options

```
  -h, --help   help for link
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone](/commands/rclone/)	 - Show help for rclone commands, flags and backends.

