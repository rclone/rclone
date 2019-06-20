---
date: 2019-06-20T16:09:42+01:00
title: "rclone rmdirs"
slug: rclone_rmdirs
url: /commands/rclone_rmdirs/
---
## rclone rmdirs

Remove empty directories under the path.

### Synopsis

This removes any empty directories (or directories that only contain
empty directories) under the path that it finds, including the path if
it has nothing in.

If you supply the --leave-root flag, it will not remove the root directory.

This is useful for tidying up remotes that rclone has left a lot of
empty directories in.



```
rclone rmdirs remote:path [flags]
```

### Options

```
  -h, --help         help for rmdirs
      --leave-root   Do not remove root directory if empty
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone](/commands/rclone/)	 - Show help for rclone commands, flags and backends.

