---
date: 2019-10-26T11:04:03+01:00
title: "rclone touch"
slug: rclone_touch
url: /commands/rclone_touch/
---
## rclone touch

Create new file or change file modification time.

### Synopsis

Create new file or change file modification time.

```
rclone touch remote:path [flags]
```

### Options

```
  -h, --help               help for touch
  -C, --no-create          Do not create the file if it does not exist.
  -t, --timestamp string   Change the modification times to the specified time instead of the current time of day. The argument is of the form 'YYMMDD' (ex. 17.10.30) or 'YYYY-MM-DDTHH:MM:SS' (ex. 2006-01-02T15:04:05)
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone](/commands/rclone/)	 - Show help for rclone commands, flags and backends.

