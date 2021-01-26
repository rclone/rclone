---
title: "Install"
description: "Rclone Installation"
---

# Install #

Rclone is a Go program and comes as a single binary file.

## Quickstart ##

  * [Download](/downloads/) the relevant binary.
  * Extract the `rclone` or `rclone.exe` binary from the archive
  * Run `rclone config` to setup. See [rclone config docs](/docs/) for more details.
  * Optionally configure [automatic execution](#autostart).

See below for some expanded Linux / macOS instructions.

See the [Usage section](/docs/#usage) of the docs for how to use rclone, or
run `rclone -h`.

Already installed rclone can be easily updated to the latest version
using the [rclone selfupdate](/commands/rclone_selfupdate/) command.

## Script installation ##

To install rclone on Linux/macOS/BSD systems, run:

    curl https://rclone.org/install.sh | sudo bash

For beta installation, run:

    curl https://rclone.org/install.sh | sudo bash -s beta

Note that this script checks the version of rclone installed first and
won't re-download if not needed.

## Linux installation from precompiled binary ##

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

## macOS installation with brew ##

    brew install rclone

## macOS installation from precompiled binary, using curl ##

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

## macOS installation from precompiled binary, using a web browser ##

When downloading a binary with a web browser, the browser will set the macOS
gatekeeper quarantine attribute. Starting from Catalina, when attempting to run
`rclone`, a pop-up will appear saying:

    “rclone” cannot be opened because the developer cannot be verified.
    macOS cannot verify that this app is free from malware.

The simplest fix is to run

    xattr -d com.apple.quarantine rclone

## Install with docker ##

The rclone maintains a [docker image for rclone](https://hub.docker.com/r/rclone/rclone).
These images are autobuilt by docker hub from the rclone source based
on a minimal Alpine linux image.

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

# make sure the config is ok by listing the remotes
docker run --rm \
    --volume ~/.config/rclone:/config/rclone \
    --volume ~/data:/data:shared \
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

## Install from source ##

Make sure you have at least [Go](https://golang.org/) go1.13
installed.  [Download go](https://golang.org/dl/) if necessary.  The
latest release is recommended. Then

    git clone https://github.com/rclone/rclone.git
    cd rclone
    go build
    ./rclone version

This will leave you a checked out version of rclone you can modify and
send pull requests with. If you use `make` instead of `go build` then
the rclone build will have the correct version information in it.

You can also build the latest stable rclone with:

    go get github.com/rclone/rclone

or the latest version (equivalent to the beta) with

    go get github.com/rclone/rclone@master

These will build the binary in `$(go env GOPATH)/bin`
(`~/go/bin/rclone` by default) after downloading the source to the go
module cache. Note - do **not** use the `-u` flag here. This causes go
to try to update the dependencies that rclone uses and sometimes these
don't work with the current version of rclone.

## Installation with Ansible ##

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

# Autostart #

After installing and configuring rclone, as described above, you are ready to use rclone
as an interactive command line utility. If your goal is to perform *periodic* operations,
such as a regular [sync](https://rclone.org/commands/rclone_sync/), you will probably want
to configure your rclone command in your operating system's scheduler. If you need to
expose *service*-like features, such as [remote control](https://rclone.org/rc/),
[GUI](https://rclone.org/gui/), [serve](https://rclone.org/commands/rclone_serve/)
or [mount](https://rclone.org/commands/rclone_move/), you will often want an rclone
command always running in the background, and configuring it to run in a service infrastructure
may be a better option. Below are some alternatives on how to achieve this on
different operating systems.

NOTE: Before setting up autorun it is highly recommended that you have tested your command
manually from a Command Prompt first.

## Autostart on Windows ##

The most relevant alternatives for autostart on Windows are:
- Run at user log on using the Startup folder
- Run at user log on, at system startup or at schedule using Task Scheduler
- Run at system startup using Windows service

### Running in background

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

### User account

As mentioned in the [mount](https://rclone.org/commands/rclone_move/) documentation,
mounted drives created as Administrator are not visible to other accounts, not even the
account that was elevated as Administrator. By running the mount command as the
built-in `SYSTEM` user account, it will create drives accessible for everyone on
the system. Both scheduled task and Windows service can be used to achieve this.

NOTE: Remember that when rclone runs as the `SYSTEM` user, the user profile
that it sees will not be yours. This means that if you normally run rclone with
configuration file in the default location, to be able to use the same configuration
when running as the system user you must explicitely tell rclone where to find
it with the [`--config`](https://rclone.org/docs/#config-config-file) option,
or else it will look in the system users profile path (`C:\Windows\System32\config\systemprofile`).
To test your command manually from a Command Prompt, you can run it with
the [PsExec](https://docs.microsoft.com/en-us/sysinternals/downloads/psexec)
utility from Microsoft's Sysinternals suite, which takes option `-s` to
execute commands as the `SYSTEM` user.

### Start from Startup folder ###

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

### Start from Task Scheduler ###

Task Scheduler is an administrative tool built into Windows, and it can be used to
configure rclone to be started automatically in a highly configurable way, e.g.
periodically on a schedule, on user log on, or at system startup. It can run
be configured to run as the current user, or for a mount command that needs to
be available to all users it can run as the `SYSTEM` user.
For technical information, see
https://docs.microsoft.com/windows/win32/taskschd/task-scheduler-start-page.

### Run as service ###

For running rclone at system startup, you can create a Windows service that executes
your rclone command, as an alternative to scheduled task configured to run at startup.

#### Mount command built-in service integration ####

For mount commands, Rclone has a built-in Windows service integration via the third party
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

#### Third party service integration ####

To Windows service running any rclone command, the excellent third party utility
[NSSM](http://nssm.cc), the "Non-Sucking Service Manager", can be used.
It includes some advanced features such as adjusting process periority, defining
process environment variables, redirect to file anything written to stdout, and
customized response to different exit codes, with a GUI to configure everything from
(although it can also be used from command line ).

There are also several other alternatives. To mention one more,
[WinSW](https://github.com/winsw/winsw), "Windows Service Wrapper", is worth checking out.
It requires .NET Framework, but it is preinstalled on newer versions of Windows, and it
also provides alternative standalone distributions which includes necessary runtime (.NET 5).
WinSW is a command-line only utility, where you have to manually create an XML file with
service configuration. This may be a drawback for some, but it can also be an advantage
as it is easy to back up and re-use the configuration
settings, without having go through manual steps in a GUI. One thing to note is that
by default it does not restart the service on error, one have to explicit enable this
in the configuration file (via the "onfailure" parameter).

## Autostart on Linux

### Start as a service

To always run rclone in background, relevant for mount commands etc,
you can use systemd to set up rclone as a system or user service. Running as a
system service ensures that it is run at startup even if the user it is running as
has no active session. Running rclone as a user service ensures that it only
starts after the configured user has logged into the system.

### Run periodically from cron

To run a periodic command, such as a copy/sync, you can set up a cron job.
