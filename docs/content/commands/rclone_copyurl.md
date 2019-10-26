---
date: 2019-10-26T11:04:03+01:00
title: "rclone copyurl"
slug: rclone_copyurl
url: /commands/rclone_copyurl/
---
## rclone copyurl

Copy url content to dest.

### Synopsis


Download urls content and copy it to destination 
without saving it in tmp storage.

Setting --auto-filename flag will cause retrieving file name from url and using it in destination path. 


```
rclone copyurl https://example.com dest:path [flags]
```

### Options

```
  -a, --auto-filename   Get the file name from the url and use it for destination file path
  -h, --help            help for copyurl
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone](/commands/rclone/)	 - Show help for rclone commands, flags and backends.

