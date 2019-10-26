---
date: 2019-10-26T11:04:03+01:00
title: "rclone config update"
slug: rclone_config_update
url: /commands/rclone_config_update/
---
## rclone config update

Update options in an existing remote.

### Synopsis


Update an existing remote's options. The options should be passed in
in pairs of <key> <value>.

For example to update the env_auth field of a remote of name myremote
you would do:

    rclone config update myremote swift env_auth true

If any of the parameters passed is a password field, then rclone will
automatically obscure them before putting them in the config file.

If the remote uses oauth the token will be updated, if you don't
require this add an extra parameter thus:

    rclone config update myremote swift env_auth true config_refresh_token false


```
rclone config update <name> [<key> <value>]+ [flags]
```

### Options

```
  -h, --help   help for update
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone config](/commands/rclone_config/)	 - Enter an interactive configuration session.

