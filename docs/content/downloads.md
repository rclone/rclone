---
title: "Rclone downloads"
description: "Download rclone binaries for your OS."
type: page
---

Rclone Download {{< version >}}
=====================

| Arch-OS | Windows | macOS | Linux | .deb | .rpm | FreeBSD | NetBSD | OpenBSD | Plan9 | Solaris |
|:-------:|:-------:|:-----:|:-----:|:----:|:----:|:-------:|:------:|:-------:|:-----:|:-------:|
| Intel/AMD - 64 Bit | {{< download windows amd64 >}} | {{< download osx amd64 >}} | {{< download linux amd64 >}} | {{< download linux amd64 deb >}} | {{< download linux amd64 rpm >}} | {{< download freebsd amd64 >}} | {{< download netbsd amd64 >}} | {{< download openbsd amd64 >}} | {{< download plan9 amd64 >}} | {{< download solaris amd64 >}} |
| Intel/AMD - 32 Bit | {{< download windows 386 >}} | - | {{< download linux 386 >}} | {{< download linux 386 deb >}} | {{< download linux 386 rpm >}} | {{< download freebsd 386 >}} | {{< download netbsd 386 >}} | {{< download openbsd 386 >}} | {{< download plan9 386 >}} | - |
| ARMv6 - 32 Bit | - | - | {{< download linux arm >}} | {{< download linux arm deb >}} | {{< download linux arm rpm >}} | {{< download freebsd arm >}} | {{< download netbsd arm >}} | - | - | - |
| ARMv7 - 32 Bit | - | - | {{< download linux arm-v7 >}} | {{< download linux arm-v7 deb >}} | {{< download linux arm-v7 rpm >}} | {{< download freebsd arm-v7 >}} | {{< download netbsd arm-v7 >}} | - | - | - |
| ARM - 64 Bit | - | - | {{< download linux arm64 >}} | {{< download linux arm64 deb >}} | {{< download linux arm64 rpm >}} | - | - | - | - | - |
| MIPS - Big Endian | - | - | {{< download linux mips >}} | {{< download linux mips deb >}} | {{< download linux mips rpm >}} | - | - | - | - | - |
| MIPS - Little Endian | - | - | {{< download linux mipsle >}} | {{< download linux mipsle deb >}} | {{< download linux mipsle rpm >}} | - | - | - | - | - |

You can also find a [mirror of the downloads on GitHub](https://github.com/rclone/rclone/releases/tag/{{< version >}}).

## Script download and install ##

To install rclone on Linux/macOS/BSD systems, run:

    curl https://rclone.org/install.sh | sudo bash

For beta installation, run:

    curl https://rclone.org/install.sh | sudo bash -s beta

Note that this script checks the version of rclone installed first and
won't re-download if not needed.

Beta releases
=============

[Beta releases](https://beta.rclone.org) are generated from each commit
to master.  Note these are named like

    {Version Tag}-{Commit Number}-g{Git Commit Hash}

You can match the `Git Commit Hash` up with the [git
log](https://github.com/rclone/rclone/commits/master).  The most recent
release will have the largest `Version Tag` and `Commit Number` and
will normally be at the end of the list.

Some beta releases may have a branch name also:

    {Version Tag}-{Commit Number}-g{Git Commit Hash}-{Branch Name}

The presence of `Branch Name` indicates that this is a feature under
development which will at some point be merged into the normal betas
and then into a normal release.

The beta releases haven't been through the [full integration test
suite](https://pub.rclone.org/integration-tests/) like the releases.
However it is useful to try the latest beta before reporting an issue.

Note that [rclone.org](https://rclone.org/) is only updated on
releases - to see the documentation for the latest beta go to
[tip.rclone.org](https://tip.rclone.org/).

Downloads for scripting
=======================

If you would like to download the current version (maybe from a
script) from a URL which doesn't change then you can use these links.

| Arch-OS | Windows | macOS | Linux | .deb | .rpm | FreeBSD | NetBSD | OpenBSD | Plan9 | Solaris |
|:-------:|:-------:|:-----:|:-----:|:----:|:----:|:-------:|:------:|:-------:|:-----:|:-------:|
| Intel/AMD - 64 Bit | {{< cdownload windows amd64 >}} | {{< cdownload osx amd64 >}} | {{< cdownload linux amd64 >}} | {{< cdownload linux amd64 deb >}} | {{< cdownload linux amd64 rpm >}} | {{< cdownload freebsd amd64 >}} | {{< cdownload netbsd amd64 >}} | {{< cdownload openbsd amd64 >}} | {{< cdownload plan9 amd64 >}} | {{< cdownload solaris amd64 >}} |
| Intel/AMD - 32 Bit | {{< cdownload windows 386 >}} | - | {{< cdownload linux 386 >}} | {{< cdownload linux 386 deb >}} | {{< cdownload linux 386 rpm >}} | {{< cdownload freebsd 386 >}} | {{< cdownload netbsd 386 >}} | {{< cdownload openbsd 386 >}} | {{< cdownload plan9 386 >}} | - |
| ARMv6 - 32 Bit | - | - | {{< cdownload linux arm >}} | {{< cdownload linux arm deb >}} | {{< cdownload linux arm rpm >}} | {{< cdownload freebsd arm >}} | {{< cdownload netbsd arm >}} | - | - | - |
| ARMv7 - 32 Bit | - | - | {{< cdownload linux arm-v7 >}} | {{< cdownload linux arm-v7 deb >}} | {{< cdownload linux arm-v7 rpm >}} | {{< cdownload freebsd arm-v7 >}} | {{< cdownload netbsd arm-v7 >}} | - | - | - |
| ARM - 64 Bit | - | - | {{< cdownload linux arm64 >}} | {{< cdownload linux arm64 deb >}} | {{< cdownload linux arm64 rpm >}} | - | - | - | - | - |
| MIPS - Big Endian | - | - | {{< cdownload linux mips >}} | {{< cdownload linux mips deb >}} | {{< cdownload linux mips rpm >}} | - | - | - | - | - |
| MIPS - Little Endian | - | - | {{< cdownload linux mipsle >}} | {{< cdownload linux mipsle deb >}} | {{< cdownload linux mipsle rpm >}} | - | - | - | - | - |

Older Downloads
==============

Older downloads can be found [here](https://downloads.rclone.org/).
