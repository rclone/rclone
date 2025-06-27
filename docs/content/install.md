---
title: "Install"
description: "Rclone Installation"
---

# Install

Rclone is a Go program and comes as a single binary file.

## Quickstart

  * [Download](/downloads/) the relevant binary.
  * Extract the `rclone` executable, `rclone.exe` on Windows, from the archive.
  * Run `rclone config` to setup. See [rclone config docs](/docs/) for more details.
  * Optionally configure [automatic execution](#autostart).

See below for some expanded Linux / macOS / Windows instructions.

See the [usage](/docs/) docs for how to use rclone, or
run `rclone -h`.

Already installed rclone can be easily updated to the latest version
using the [rclone selfupdate](/commands/rclone_selfupdate/) command.

See [the release signing docs](/release_signing/) for how to verify
signatures on the release.

## Script installation

To install rclone on Linux/macOS/BSD systems, run:

    sudo -v ; curl https://rclone.org/install.sh | sudo bash

For beta installation, run:

    sudo -v ; curl https://rclone.org/install.sh | sudo bash -s beta

Note that this script checks the version of rclone installed first and
won't re-download if not needed.

## Linux installation {#linux}

### Precompiled binary {#linux-precompiled}

Fetch and unpack

    curl -O https://downloads.rclone.org/rclone-current-linux-amd64.zip
    unzip rclone-current-linux-amd64.zip
    cd rclone-*-linux-amd64

Copy binary file

    sudo cp rclone /usr/bin/
    sudo chown root:root /usr/bin/rclone
    sudo chmod 755 /usr/bin/rclone

Install manpage

    sudo mkdir -p /usr/local/share/man/man1
    sudo cp rclone.1 /usr/local/share/man/man1/
    sudo mandb

Run `rclone config` to setup. See [rclone config docs](/docs/) for more details.

    rclone config

## macOS installation {#macos}

### Installation with brew {#macos-brew}

    brew install rclone

NOTE: This version of rclone will not support `mount` any more (see
[#5373](https://github.com/rclone/rclone/issues/5373)). If mounting is wanted
on macOS, either install a precompiled binary or enable the relevant option
when [installing from source](#source).

Note that this is a third party installer not controlled by the rclone
developers so it may be out of date. Its current version is as below.

[![Homebrew package](https://repology.org/badge/version-for-repo/homebrew/rclone.svg)](https://repology.org/project/rclone/versions)

### Installation with MacPorts {#macos-macports}

On macOS, rclone can also be installed via [MacPorts](https://www.macports.org):

    sudo port install rclone

Note that this is a third party installer not controlled by the rclone
developers so it may be out of date. Its current version is as below.

[![MacPorts port](https://repology.org/badge/version-for-repo/macports/rclone.svg)](https://repology.org/project/rclone/versions)

More information [here](https://ports.macports.org/port/rclone/).

### Precompiled binary, using curl {#macos-precompiled}

To avoid problems with macOS gatekeeper enforcing the binary to be signed and
notarized it is enough to download with `curl`.

Download the latest version of rclone.

    cd && curl -O https://downloads.rclone.org/rclone-current-osx-amd64.zip

Unzip the download and cd to the extracted folder.

    unzip -a rclone-current-osx-amd64.zip && cd rclone-*-osx-amd64

Move rclone to your $PATH. You will be prompted for your password.

    sudo mkdir -p /usr/local/bin
    sudo mv rclone /usr/local/bin/

(the `mkdir` command is safe to run, even if the directory already exists).

Remove the leftover files.

    cd .. && rm -rf rclone-*-osx-amd64 rclone-current-osx-amd64.zip

Run `rclone config` to setup. See [rclone config docs](/docs/) for more details.

    rclone config

### Precompiled binary, using a web browser  {#macos-precompiled-web}

When downloading a binary with a web browser, the browser will set the macOS
gatekeeper quarantine attribute. Starting from Catalina, when attempting to run
`rclone`, a pop-up will appear saying:

    "rclone" cannot be opened because the developer cannot be verified.
    macOS cannot verify that this app is free from malware.

The simplest fix is to run

    xattr -d com.apple.quarantine rclone

## Windows installation {#windows}

### Precompiled binary {#windows-precompiled}

Fetch the correct binary for your processor type by clicking on these
links. If not sure, use the first link.

- [Intel/AMD - 64 Bit](https://downloads.rclone.org/rclone-current-windows-amd64.zip)
- [Intel/AMD - 32 Bit](https://downloads.rclone.org/rclone-current-windows-386.zip)
- [ARM - 64 Bit](https://downloads.rclone.org/rclone-current-windows-arm64.zip)

Open this file in the Explorer and extract `rclone.exe`. Rclone is a
portable executable so you can place it wherever is convenient.

Open a CMD window (or powershell) and run the binary. Note that rclone
does not launch a GUI by default, it runs in the CMD Window.

- Run `rclone.exe config` to setup. See [rclone config docs](/docs/) for more details.
- Optionally configure [automatic execution](#autostart).

If you are planning to use the [rclone mount](/commands/rclone_mount/)
feature then you will need to install the third party utility
[WinFsp](https://winfsp.dev/) also.

### Windows package manager (Winget) {#windows-chocolatey}

[Winget](https://learn.microsoft.com/en-us/windows/package-manager/) comes pre-installed with the latest versions of Windows. If not, update the [App Installer](https://www.microsoft.com/p/app-installer/9nblggh4nns1) package from the Microsoft store.

To install rclone
```
winget install Rclone.Rclone
```
To uninstall rclone
```
winget uninstall Rclone.Rclone --force
```

### Chocolatey package manager {#windows-chocolatey}

Make sure you have [Choco](https://chocolatey.org/) installed

```
choco search rclone
choco install rclone
```

This will install rclone on your Windows machine. If you are planning
to use [rclone mount](/commands/rclone_mount/) then

```
choco install winfsp
```

will install that too.

Note that this is a third party installer not controlled by the rclone
developers so it may be out of date. Its current version is as below.

[![Chocolatey package](https://repology.org/badge/version-for-repo/chocolatey/rclone.svg)](https://repology.org/project/rclone/versions)

### Scoop package manager {#windows-scoop}

Make sure you have [Scoop](https://scoop.sh/) installed

```
scoop install rclone
```

Note that this is a third party installer not controlled by the rclone
developers so it may be out of date. Its current version is as below.

[![Scoop package](https://repology.org/badge/version-for-repo/scoop/rclone.svg)](https://repology.org/project/rclone/versions)

## Package manager installation {#package-manager}

Many Linux, Windows, macOS and other OS distributions package and
distribute rclone.

The distributed versions of rclone are often quite out of date and for
this reason we recommend one of the other installation methods if
possible.

You can get an idea of how up to date or not your OS distribution's
package is here.

[![Packaging status](https://repology.org/badge/vertical-allrepos/rclone.svg?columns=3)](https://repology.org/project/rclone/versions)

## Docker installation {#docker}

The rclone developers maintain a [docker image for rclone](https://hub.docker.com/r/rclone/rclone).

**Note:** We also now offer a paid version of rclone with
enterprise-grade security and zero CVEs through our partner
[SecureBuild](https://securebuild.com/blog/introducing-securebuild).
If you are interested, check out their website and the [Rclone
SecureBuild Image](https://securebuild.com/images/rclone).

These images are built as part of the release process based on a
minimal Alpine Linux.

The `:latest` tag will always point to the latest stable release.  You
can use the `:beta` tag to get the latest build from master.  You can
also use version tags, e.g. `:1.49.1`, `:1.49` or `:1`.

```
$ docker pull rclone/rclone:latest
latest: Pulling from rclone/rclone
Digest: sha256:0e0ced72671989bb837fea8e88578b3fc48371aa45d209663683e24cfdaa0e11
...
$ docker run --rm rclone/rclone:latest version
rclone v1.49.1
- os/arch: linux/amd64
- go version: go1.12.9
```

There are a few command line options to consider when starting an rclone Docker container
from the rclone image.

- You need to mount the host rclone config dir at `/config/rclone` into the Docker
  container. Due to the fact that rclone updates tokens inside its config file, and that
  the update process involves a file rename, you need to mount the whole host rclone
  config dir, not just the single host rclone config file.

- You need to mount a host data dir at `/data` into the Docker container.

- By default, the rclone binary inside a Docker container runs with UID=0 (root).
  As a result, all files created in a run will have UID=0. If your config and data files
  reside on the host with a non-root UID:GID, you need to pass these on the container
  start command line.

- If you want to access the RC interface (either via the API or the Web UI), it is
  required to set the `--rc-addr` to `:5572` in order to connect to it from outside
  the container. An explanation about why this is necessary is present [here](https://web.archive.org/web/20200808071950/https://pythonspeed.com/articles/docker-connection-refused/).
    * NOTE: Users running this container with the docker network set to `host` should
     probably set it to listen to localhost only, with `127.0.0.1:5572` as the value for
      `--rc-addr`

- It is possible to use `rclone mount` inside a userspace Docker container, and expose
  the resulting fuse mount to the host. The exact `docker run` options to do that might
  vary slightly between hosts. See, e.g. the discussion in this
  [thread](https://github.com/moby/moby/issues/9448).

  You also need to mount the host `/etc/passwd` and `/etc/group` for fuse to work inside
  the container.

Here are some commands tested on an Ubuntu 18.04.3 host:

```
# config on host at ~/.config/rclone/rclone.conf
# data on host at ~/data

# add a remote interactively
docker run --rm -it \
    --volume ~/.config/rclone:/config/rclone \
    --user $(id -u):$(id -g) \
    rclone/rclone \
    config

# make sure the config is ok by listing the remotes
docker run --rm \
    --volume ~/.config/rclone:/config/rclone \
    --user $(id -u):$(id -g) \
    rclone/rclone \
    listremotes

# perform mount inside Docker container, expose result to host
mkdir -p ~/data/mount
docker run --rm \
    --volume ~/.config/rclone:/config/rclone \
    --volume ~/data:/data:shared \
    --user $(id -u):$(id -g) \
    --volume /etc/passwd:/etc/passwd:ro --volume /etc/group:/etc/group:ro \
    --device /dev/fuse --cap-add SYS_ADMIN --security-opt apparmor:unconfined \
    rclone/rclone \
    mount dropbox:Photos /data/mount &
ls ~/data/mount
kill %1
```

## Snap installation {#snap}

[![Get it from the Snap Store](https://snapcraft.io/static/images/badges/en/snap-store-black.svg)](https://snapcraft.io/rclone)

Make sure you have [Snapd installed](https://snapcraft.io/docs/installing-snapd)

```bash
$ sudo snap install rclone
```
Due to the strict confinement of Snap, rclone snap cannot access real /home/$USER/.config/rclone directory, default config path is as below.

- Default config directory:
    - /home/$USER/snap/rclone/current/.config/rclone

Note: Due to the strict confinement of Snap, `rclone mount` feature is `not` supported.

If mounting is wanted, either install a precompiled binary or enable the relevant option when [installing from source](#source).

Note that this is controlled by [community maintainer](https://github.com/boukendesho/rclone-snap) not the rclone developers so it may be out of date. Its current version is as below.

[![rclone](https://snapcraft.io/rclone/badge.svg)](https://snapcraft.io/rclone)


## Source installation {#source}

Make sure you have git and [Go](https://golang.org/) installed.
Go version 1.22 or newer is required, the latest release is recommended.
You can get it from your package manager, or download it from
[golang.org/dl](https://golang.org/dl/). Then you can run the following:

```
git clone https://github.com/rclone/rclone.git
cd rclone
go build
```

This will check out the rclone source in subfolder rclone, which you can later
modify and send pull requests with. Then it will build the rclone executable
in the same folder. As an initial check you can now run `./rclone version`
(`.\rclone version` on Windows).

Note that on macOS and Windows the [mount](https://rclone.org/commands/rclone_mount/)
command will not be available unless you specify an additional build tag `cmount`.

```
go build -tags cmount
```

This assumes you have a GCC compatible C compiler (GCC or Clang) in your PATH,
as it uses [cgo](https://pkg.go.dev/cmd/cgo). But on Windows, the
[cgofuse](https://github.com/winfsp/cgofuse) library that the cmount
implementation is based on, also supports building
[without cgo](https://github.com/golang/go/wiki/WindowsDLLs), i.e. by setting
environment variable CGO_ENABLED to value 0 (static linking). This is how the
official Windows release of rclone is being built, starting with version 1.59.
It is still possible to build with cgo on Windows as well, by using the MinGW
port of GCC, e.g. by installing it in a [MSYS2](https://www.msys2.org)
distribution (make sure you install it in the classic mingw64 subsystem, the
ucrt64 version is not compatible).

Additionally, to build with mount on Windows, you must install the third party
utility [WinFsp](https://winfsp.dev/), with the "Developer" feature selected.
If building with cgo, you must also set environment variable CPATH pointing to
the fuse include directory within the WinFsp installation
(normally `C:\Program Files (x86)\WinFsp\inc\fuse`).

You may add arguments `-ldflags -s` to omit symbol table and debug information,
making the executable file smaller, and `-trimpath` to remove references to
local file system paths. The official rclone releases are built with both of these.

```
go build -trimpath -ldflags -s -tags cmount
```

If you want to customize the version string, as reported by
the `rclone version` command, you can set one of the variables `fs.Version`,
`fs.VersionTag` (to keep default suffix but customize the number),
or `fs.VersionSuffix` (to keep default number but customize the suffix).
This can be done from the build command, by adding to the `-ldflags`
argument value as shown below.

```
go build -trimpath -ldflags "-s -X github.com/rclone/rclone/fs.Version=v9.9.9-test" -tags cmount
```

On Windows, the official executables also have the version information,
as well as a file icon, embedded as binary resources. To get that with your
own build you need to run the following command **before** the build command.
It generates a Windows resource system object file, with extension .syso, e.g.
`resource_windows_amd64.syso`, that will be automatically picked up by
future build commands.

```
go run bin/resource_windows.go
```

The above command will generate a resource file containing version information
based on the fs.Version variable in source at the time you run the command,
which means if the value of this variable changes you need to re-run the
command for it to be reflected in the version information. Also, if you
override this version variable in the build command as described above, you
need to do that also when generating the resource file, or else it will still
use the value from the source.

```
go run bin/resource_windows.go -version v9.9.9-test
```

Instead of executing the `go build` command directly, you can run it via the
Makefile. The default target changes the version suffix from "-DEV" to "-beta"
followed by additional commit details, embeds version information binary resources
on Windows, and copies the resulting rclone executable into your GOPATH bin folder
(`$(go env GOPATH)/bin`, which corresponds to `~/go/bin/rclone` by default).

```
make
```

To include mount command on macOS and Windows with Makefile build:

```
make GOTAGS=cmount
```

There are other make targets that can be used for more advanced builds,
such as cross-compiling for all supported os/architectures, and packaging
results into release artifacts.
See [Makefile](https://github.com/rclone/rclone/blob/master/Makefile)
and [cross-compile.go](https://github.com/rclone/rclone/blob/master/bin/cross-compile.go)
for details.

Another alternative method for source installation is to download the source,
build and install rclone - all in one operation, as a regular Go package.
The source will be stored it in the Go module cache, and the resulting
executable will be in your GOPATH bin folder (`$(go env GOPATH)/bin`,
which corresponds to `~/go/bin/rclone` by default).

```
go install github.com/rclone/rclone@latest
```

In some situations, rclone executable size might be too big for deployment
in very restricted environments when all backends with large SDKs are included.
To limit binary size unused backends can be commented out in `backends/all/all.go`
and unused commands in `cmd/all/all.go` before building with `go build` or `make`

## Ansible installation {#ansible}

This can be done with [Stefan Weichinger's ansible
role](https://github.com/stefangweichinger/ansible-rclone).

Instructions

  1. `git clone https://github.com/stefangweichinger/ansible-rclone.git` into your local roles-directory
  2. add the role to the hosts you want rclone installed to:

```
    - hosts: rclone-hosts
      roles:
          - rclone
```

## Portable installation {#portable}

As mentioned [above](https://rclone.org/install/#quickstart), rclone is single
executable (`rclone`, or `rclone.exe` on Windows) that you can download as a
zip archive and extract into a location of your choosing. When executing different
commands, it may create files in different locations, such as a configuration file
and various temporary files. By default the locations for these are according to
your operating system, e.g. configuration file in your user profile directory and
temporary files in the standard temporary directory, but you can customize all of
them, e.g. to make a completely self-contained, portable installation.

Run the [config paths](/commands/rclone_config_paths/) command to see
the locations that rclone will use.

To override them set the corresponding options (as command-line arguments, or as
[environment variables](https://rclone.org/docs/#environment-variables)):
  - [--config](https://rclone.org/docs/#config-config-file)
  - [--cache-dir](https://rclone.org/docs/#cache-dir-dir)
  - [--temp-dir](https://rclone.org/docs/#temp-dir-dir)

## Autostart

After installing and configuring rclone, as described above, you are ready to use rclone
as an interactive command line utility. If your goal is to perform *periodic* operations,
such as a regular [sync](https://rclone.org/commands/rclone_sync/), you will probably want
to configure your rclone command in your operating system's scheduler. If you need to
expose *service*-like features, such as [remote control](https://rclone.org/rc/),
[GUI](https://rclone.org/gui/), [serve](https://rclone.org/commands/rclone_serve/)
or [mount](https://rclone.org/commands/rclone_mount/), you will often want an rclone
command always running in the background, and configuring it to run in a service infrastructure
may be a better option. Below are some alternatives on how to achieve this on
different operating systems.

NOTE: Before setting up autorun it is highly recommended that you have tested your command
manually from a Command Prompt first.

### Autostart on Windows

The most relevant alternatives for autostart on Windows are:
- Run at user log on using the Startup folder
- Run at user log on, at system startup or at schedule using Task Scheduler
- Run at system startup using Windows service

#### Running in background

Rclone is a console application, so if not starting from an existing Command Prompt,
e.g. when starting rclone.exe from a shortcut, it will open a Command Prompt window.
When configuring rclone to run from task scheduler and windows service you are able
to set it to run hidden in background. From rclone version 1.54 you can also make it
run hidden from anywhere by adding option `--no-console` (it may still flash briefly
when the program starts). Since rclone normally writes information and any error
messages to the console, you must redirect this to a file to be able to see it.
Rclone has a built-in option `--log-file` for that.

Example command to run a sync in background:
```
c:\rclone\rclone.exe sync c:\files remote:/files --no-console --log-file c:\rclone\logs\sync_files.txt
```

#### User account

As mentioned in the [mount](https://rclone.org/commands/rclone_mount/) documentation,
mounted drives created as Administrator are not visible to other accounts, not even the
account that was elevated as Administrator. By running the mount command as the
built-in `SYSTEM` user account, it will create drives accessible for everyone on
the system. Both scheduled task and Windows service can be used to achieve this.

NOTE: Remember that when rclone runs as the `SYSTEM` user, the user profile
that it sees will not be yours. This means that if you normally run rclone with
configuration file in the default location, to be able to use the same configuration
when running as the system user you must explicitly tell rclone where to find
it with the [`--config`](https://rclone.org/docs/#config-config-file) option,
or else it will look in the system users profile path (`C:\Windows\System32\config\systemprofile`).
To test your command manually from a Command Prompt, you can run it with
the [PsExec](https://docs.microsoft.com/en-us/sysinternals/downloads/psexec)
utility from Microsoft's Sysinternals suite, which takes option `-s` to
execute commands as the `SYSTEM` user.

#### Start from Startup folder

To quickly execute an rclone command you can simply create a standard
Windows Explorer shortcut for the complete rclone command you want to run. If you
store this shortcut in the special "Startup" start-menu folder, Windows will
automatically run it at login. To open this folder in Windows Explorer,
enter path `%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup`,
or `C:\ProgramData\Microsoft\Windows\Start Menu\Programs\StartUp` if you want
the command to start for *every* user that logs in.

This is the easiest approach to autostarting of rclone, but it offers no
functionality to set it to run as different user, or to set conditions or
actions on certain events. Setting up a scheduled task as described below
will often give you better results.

#### Start from Task Scheduler

Task Scheduler is an administrative tool built into Windows, and it can be used to
configure rclone to be started automatically in a highly configurable way, e.g.
periodically on a schedule, on user log on, or at system startup. It can run
be configured to run as the current user, or for a mount command that needs to
be available to all users it can run as the `SYSTEM` user.
For technical information, see
https://docs.microsoft.com/windows/win32/taskschd/task-scheduler-start-page.

#### Run as service

For running rclone at system startup, you can create a Windows service that executes
your rclone command, as an alternative to scheduled task configured to run at startup.

##### Mount command built-in service integration

For mount commands, rclone has a built-in Windows service integration via the third-party
WinFsp library it uses. Registering as a regular Windows service easy, as you just have to
execute the built-in PowerShell command `New-Service` (requires administrative privileges).

Example of a PowerShell command that creates a Windows service for mounting
some `remote:/files` as drive letter `X:`, for *all* users (service will be running as the
local system account):

```
New-Service -Name Rclone -BinaryPathName 'c:\rclone\rclone.exe mount remote:/files X: --config c:\rclone\config\rclone.conf --log-file c:\rclone\logs\mount.txt'
```

The [WinFsp service infrastructure](https://github.com/billziss-gh/winfsp/wiki/WinFsp-Service-Architecture)
supports incorporating services for file system implementations, such as rclone,
into its own launcher service, as kind of "child services". This has the additional
advantage that it also implements a network provider that integrates into
Windows standard methods for managing network drives. This is currently not
officially supported by Rclone, but with WinFsp version 2019.3 B2 / v1.5B2 or later
it should be possible through path rewriting as described [here](https://github.com/rclone/rclone/issues/3340).

##### Third-party service integration

To Windows service running any rclone command, the excellent third-party utility
[NSSM](http://nssm.cc), the "Non-Sucking Service Manager", can be used.
It includes some advanced features such as adjusting process priority, defining
process environment variables, redirect to file anything written to stdout, and
customized response to different exit codes, with a GUI to configure everything from
(although it can also be used from command line ).

There are also several other alternatives. To mention one more,
[WinSW](https://github.com/winsw/winsw), "Windows Service Wrapper", is worth checking out.
It requires .NET Framework, but it is preinstalled on newer versions of Windows, and it
also provides alternative standalone distributions which includes necessary runtime (.NET 5).
WinSW is a command-line only utility, where you have to manually create an XML file with
service configuration. This may be a drawback for some, but it can also be an advantage
as it is easy to back up and reuse the configuration
settings, without having go through manual steps in a GUI. One thing to note is that
by default it does not restart the service on error, one have to explicit enable this
in the configuration file (via the "onfailure" parameter).

### Autostart on Linux

#### Start as a service

To always run rclone in background, relevant for mount commands etc,
you can use systemd to set up rclone as a system or user service. Running as a
system service ensures that it is run at startup even if the user it is running as
has no active session. Running rclone as a user service ensures that it only
starts after the configured user has logged into the system.

#### Run periodically from cron

To run a periodic command, such as a copy/sync, you can set up a cron job.
