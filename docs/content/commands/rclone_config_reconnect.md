---
date: 2019-10-26T11:04:03+01:00
title: "rclone config reconnect"
slug: rclone_config_reconnect
url: /commands/rclone_config_reconnect/
---
## rclone config reconnect

Re-authenticates user with remote.

### Synopsis


This reconnects remote: passed in to the cloud storage system.

To disconnect the remote use "rclone config disconnect".

This normally means going through the interactive oauth flow again.


```
rclone config reconnect remote: [flags]
```

### Options

```
  -h, --help   help for reconnect
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone config](/commands/rclone_config/)	 - Enter an interactive configuration session.

