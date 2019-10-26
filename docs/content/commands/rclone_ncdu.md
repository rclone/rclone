---
date: 2019-10-26T11:04:03+01:00
title: "rclone ncdu"
slug: rclone_ncdu
url: /commands/rclone_ncdu/
---
## rclone ncdu

Explore a remote with a text based user interface.

### Synopsis


This displays a text based user interface allowing the navigation of a
remote. It is most useful for answering the question - "What is using
all my disk space?".

<script src="https://asciinema.org/a/157793.js" id="asciicast-157793" async></script>

To make the user interface it first scans the entire remote given and
builds an in memory representation.  rclone ncdu can be used during
this scanning phase and you will see it building up the directory
structure as it goes along.

Here are the keys - press '?' to toggle the help on and off

     ↑,↓ or k,j to Move
     →,l to enter
     ←,h to return
     c toggle counts
     g toggle graph
     n,s,C sort by name,size,count
     d delete file/directory
     Y display current path
     ^L refresh screen
     ? to toggle help on and off
     q/ESC/c-C to quit

This an homage to the [ncdu tool](https://dev.yorhel.nl/ncdu) but for
rclone remotes.  It is missing lots of features at the moment
but is useful as it stands.

Note that it might take some time to delete big files/folders. The
UI won't respond in the meantime since the deletion is done synchronously.


```
rclone ncdu remote:path [flags]
```

### Options

```
  -h, --help   help for ncdu
```

See the [global flags page](/flags/) for global options not listed here.

### SEE ALSO

* [rclone](/commands/rclone/)	 - Show help for rclone commands, flags and backends.

