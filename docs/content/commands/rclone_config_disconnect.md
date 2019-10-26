---
date: 2019-10-26T11:04:03+01:00
title: "rclone config disconnect"
slug: rclone_config_disconnect
url: /commands/rclone_config_disconnect/
---
## rclone config disconnect

Disconnects user from remote

### Synopsis


This disconnects the remote: passed in to the cloud storage system.

This normally means revoking the oauth token.

To reconnect use "rclone config reconnect".


```
rclone config disconnect remote: [flags]
```

### Options

```
  -h, --help   help for disconnect
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone config](/commands/rclone_config/)	 - Enter an interactive configuration session.

