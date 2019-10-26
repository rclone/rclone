---
date: 2019-10-26T11:04:03+01:00
title: "rclone serve"
slug: rclone_serve
url: /commands/rclone_serve/
---
## rclone serve

Serve a remote over a protocol.

### Synopsis

rclone serve is used to serve a remote over a given protocol. This
command requires the use of a subcommand to specify the protocol, eg

    rclone serve http remote:

Each subcommand has its own options which you can see in their help.


```
rclone serve <protocol> [opts] <remote> [flags]
```

### Options

```
  -h, --help   help for serve
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone](/commands/rclone/)	 - Show help for rclone commands, flags and backends.
* [rclone serve dlna](/commands/rclone_serve_dlna/)	 - Serve remote:path over DLNA
* [rclone serve ftp](/commands/rclone_serve_ftp/)	 - Serve remote:path over FTP.
* [rclone serve http](/commands/rclone_serve_http/)	 - Serve the remote over HTTP.
* [rclone serve restic](/commands/rclone_serve_restic/)	 - Serve the remote for restic's REST API.
* [rclone serve sftp](/commands/rclone_serve_sftp/)	 - Serve the remote over SFTP.
* [rclone serve webdav](/commands/rclone_serve_webdav/)	 - Serve remote:path over webdav.

