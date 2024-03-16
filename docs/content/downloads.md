---
title: "Rclone downloads"
description: "Download rclone binaries for your OS."
type: page
---

# Downloads

Rclone is single executable (`rclone`, or `rclone.exe` on Windows) that you can
simply download as a zip archive and extract into a location of your choosing.
See the [install](https://rclone.org/install/) documentation for more details.

## Release {{% version %}} OS requirements {#osrequirements}

| OS | Minimum Version | 
|:-------:|:-------:|
| Linux | kernel 2.6.32 |
| macOS (Intel) | 10.15 (Catalina) |
| macOS (ARM64) | 11 (Big Sur) |
| Windows | 10, Server 2016 |
| FreeBSD | 12.2 |
| OpenBSD | 6.9 |

## Release {{% version %}} {#release}

| Arch-OS | Windows | macOS | Linux | .deb | .rpm | FreeBSD | NetBSD | OpenBSD | Plan9 | Solaris |
|:-------:|:-------:|:-----:|:-----:|:----:|:----:|:-------:|:------:|:-------:|:-----:|:-------:|
| Intel/AMD - 64 Bit | {{< download windows amd64 >}} | {{< download osx amd64 >}} | {{< download linux amd64 >}} | {{< download linux amd64 deb >}} | {{< download linux amd64 rpm >}} | {{< download freebsd amd64 >}} | {{< download netbsd amd64 >}} | {{< download openbsd amd64 >}} | {{< download plan9 amd64 >}} | {{< download solaris amd64 >}} |
| Intel/AMD - 32 Bit | {{< download windows 386 >}} | - | {{< download linux 386 >}} | {{< download linux 386 deb >}} | {{< download linux 386 rpm >}} | {{< download freebsd 386 >}} | {{< download netbsd 386 >}} | {{< download openbsd 386 >}} | {{< download plan9 386 >}} | - |
| ARMv5 - 32 Bit NOHF | - | - | {{< download linux arm >}} | {{< download linux arm deb >}} | {{< download linux arm rpm >}} | {{< download freebsd arm >}} | {{< download netbsd arm >}} | - | - | - |
| ARMv6 - 32 Bit | - | - | {{< download linux arm-v6 >}} | {{< download linux arm-v6 deb >}} | {{< download linux arm-v6 rpm >}} | {{< download freebsd arm-v6 >}} | {{< download netbsd arm-v6 >}} | - | - | - |
| ARMv7 - 32 Bit | - | - | {{< download linux arm-v7 >}} | {{< download linux arm-v7 deb >}} | {{< download linux arm-v7 rpm >}} | {{< download freebsd arm-v7 >}} | {{< download netbsd arm-v7 >}} | - | - | - |
| ARM - 64 Bit | {{< download windows arm64 >}} | {{< download osx arm64 >}} | {{< download linux arm64 >}} | {{< download linux arm64 deb >}} | {{< download linux arm64 rpm >}} | - | - | - | - | - |
| MIPS - Big Endian | - | - | {{< download linux mips >}} | {{< download linux mips deb >}} | {{< download linux mips rpm >}} | - | - | - | - | - |
| MIPS - Little Endian | - | - | {{< download linux mipsle >}} | {{< download linux mipsle deb >}} | {{< download linux mipsle rpm >}} | - | - | - | - | - |

You can also find a [mirror of the downloads on GitHub](https://github.com/rclone/rclone/releases/tag/{{< version >}}).

See also [Android builds](https://beta.rclone.org/{{% version %}}/testbuilds/).
These are built as part of the official release, but haven't been
adopted as first class builds yet.

See [the release signing docs](/release_signing/) for how to verify
signatures on the release.

## Script download and install ##

To install rclone on Linux/macOS/BSD systems, run:

    sudo -v ; curl https://rclone.org/install.sh | sudo bash

For beta installation, run:

    sudo -v ; curl https://rclone.org/install.sh | sudo bash -s beta

Note that this script checks the version of rclone installed first and
won't re-download if not needed.

## Beta releases

[Beta releases](https://beta.rclone.org) are generated from each commit
to master.  Note these are named like

    {Version Tag}.beta.{Commit Number}.{Git Commit Hash}

e.g.

    v1.53.0-beta.4677.b657a2204

The `Version Tag` is the version that the beta release will become
when it is released. You can match the `Git Commit Hash` up with the
[git log](https://github.com/rclone/rclone/commits/master).  The most
recent release will have the largest `Version Tag` and `Commit Number`
and will normally be at the end of the list.

Some beta releases may have a branch name also:

    {Version Tag}-beta.{Commit Number}.{Git Commit Hash}.{Branch Name}

e.g.

    v1.53.0-beta.4677.b657a2204.semver

The presence of `Branch Name` indicates that this is a feature under
development which will at some point be merged into the normal betas
and then into a normal release.

The beta releases haven't been through the [full integration test
suite](https://pub.rclone.org/integration-tests/) like the releases.
However it is useful to try the latest beta before reporting an issue.

Note that [rclone.org](https://rclone.org/) is only updated on
releases - to see the documentation for the latest beta go to
[tip.rclone.org](https://tip.rclone.org/).

## Downloads for scripting

If you would like to download the current version (maybe from a
script) from a URL which doesn't change then you can use these links.

| Arch-OS | Windows | macOS | Linux | .deb | .rpm | FreeBSD | NetBSD | OpenBSD | Plan9 | Solaris |
|:-------:|:-------:|:-----:|:-----:|:----:|:----:|:-------:|:------:|:-------:|:-----:|:-------:|
| Intel/AMD - 64 Bit | {{< cdownload windows amd64 >}} | {{< cdownload osx amd64 >}} | {{< cdownload linux amd64 >}} | {{< cdownload linux amd64 deb >}} | {{< cdownload linux amd64 rpm >}} | {{< cdownload freebsd amd64 >}} | {{< cdownload netbsd amd64 >}} | {{< cdownload openbsd amd64 >}} | {{< cdownload plan9 amd64 >}} | {{< cdownload solaris amd64 >}} |
| Intel/AMD - 32 Bit | {{< cdownload windows 386 >}} | - | {{< cdownload linux 386 >}} | {{< cdownload linux 386 deb >}} | {{< cdownload linux 386 rpm >}} | {{< cdownload freebsd 386 >}} | {{< cdownload netbsd 386 >}} | {{< cdownload openbsd 386 >}} | {{< cdownload plan9 386 >}} | - |
| ARMv5 - 32 Bit NOHF | - | - | {{< cdownload linux arm >}} | {{< cdownload linux arm deb >}} | {{< cdownload linux arm rpm >}} | {{< cdownload freebsd arm >}} | {{< cdownload netbsd arm >}} | - | - | - |
| ARMv6 - 32 Bit | - | - | {{< cdownload linux arm-v6 >}} | {{< cdownload linux arm-v6 deb >}} | {{< cdownload linux arm-v6 rpm >}} | {{< cdownload freebsd arm-v6 >}} | {{< cdownload netbsd arm-v6 >}} | - | - | - |
| ARMv7 - 32 Bit | - | - | {{< cdownload linux arm-v7 >}} | {{< cdownload linux arm-v7 deb >}} | {{< cdownload linux arm-v7 rpm >}} | {{< cdownload freebsd arm-v7 >}} | {{< cdownload netbsd arm-v7 >}} | - | - | - |
| ARM - 64 Bit | {{< cdownload windows arm64 >}} | {{< cdownload osx arm64 >}} | {{< cdownload linux arm64 >}} | {{< cdownload linux arm64 deb >}} | {{< cdownload linux arm64 rpm >}} | - | - | - | - | - |
| MIPS - Big Endian | - | - | {{< cdownload linux mips >}} | {{< cdownload linux mips deb >}} | {{< cdownload linux mips rpm >}} | - | - | - | - | - |
| MIPS - Little Endian | - | - | {{< cdownload linux mipsle >}} | {{< cdownload linux mipsle deb >}} | {{< cdownload linux mipsle rpm >}} | - | - | - | - | - |

## Older Downloads

Older downloads can be found [here](https://downloads.rclone.org/).

The latest `rclone` version working for:
| OS | Maximum rclone version | 
|:-------:|:-------:|
| Windows 7 | v1.63.1 |
| Windows Server 2008 | v1.63.1 |
| Windows Server 2012 | v1.63.1 |
| Windows XP | v1.42 |
| Windows Vista | v1.42 |
| macOS 10.14 (Mojave) | v1.63.1 |
| macOS 10.13 (High Sierra) | v1.63.1 |
| macOS 10.12 (Sierra) | v1.56.0 |
| macOS 10.11 (El Capitan) | v1.52.0 |
| macOS 10.10 (Yosemite) | v1.49.0 |
| OS X 10.9 (Mavericks) | v1.42 |
| OS X 10.8 (Mountain Lion) | v1.42 |
