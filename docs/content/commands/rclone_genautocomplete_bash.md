---
date: 2019-06-20T16:09:42+01:00
title: "rclone genautocomplete bash"
slug: rclone_genautocomplete_bash
url: /commands/rclone_genautocomplete_bash/
---
## rclone genautocomplete bash

Output bash completion script for rclone.

### Synopsis


Generates a bash shell autocompletion script for rclone.

This writes to /etc/bash_completion.d/rclone by default so will
probably need to be run with sudo or as root, eg

    sudo rclone genautocomplete bash

Logout and login again to use the autocompletion scripts, or source
them directly

    . /etc/bash_completion

If you supply a command line argument the script will be written
there.


```
rclone genautocomplete bash [output_file] [flags]
```

### Options

```
  -h, --help   help for bash
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone genautocomplete](/commands/rclone_genautocomplete/)	 - Output completion script for a given shell.

