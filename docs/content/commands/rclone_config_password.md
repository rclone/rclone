---
date: 2019-10-26T11:04:03+01:00
title: "rclone config password"
slug: rclone_config_password
url: /commands/rclone_config_password/
---
## rclone config password

Update password in an existing remote.

### Synopsis


Update an existing remote's password. The password
should be passed in in pairs of <key> <value>.

For example to set password of a remote of name myremote you would do:

    rclone config password myremote fieldname mypassword

This command is obsolete now that "config update" and "config create"
both support obscuring passwords directly.


```
rclone config password <name> [<key> <value>]+ [flags]
```

### Options

```
  -h, --help   help for password
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone config](/commands/rclone_config/)	 - Enter an interactive configuration session.

