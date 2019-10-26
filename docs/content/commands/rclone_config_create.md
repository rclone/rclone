---
date: 2019-10-26T11:04:03+01:00
title: "rclone config create"
slug: rclone_config_create
url: /commands/rclone_config_create/
---
## rclone config create

Create a new remote with name, type and options.

### Synopsis


Create a new remote of <name> with <type> and options.  The options
should be passed in in pairs of <key> <value>.

For example to make a swift remote of name myremote using auto config
you would do:

    rclone config create myremote swift env_auth true

Note that if the config process would normally ask a question the
default is taken.  Each time that happens rclone will print a message
saying how to affect the value taken.

If any of the parameters passed is a password field, then rclone will
automatically obscure them before putting them in the config file.

So for example if you wanted to configure a Google Drive remote but
using remote authorization you would do this:

    rclone config create mydrive drive config_is_local false


```
rclone config create <name> <type> [<key> <value>]* [flags]
```

### Options

```
  -h, --help   help for create
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone config](/commands/rclone_config/)	 - Enter an interactive configuration session.

