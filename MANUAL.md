% rclone(1) User Manual
% Nick Craig-Wood
% Jun 24, 2020

# Rclone syncs your files to cloud storage

<img width="50%" src="https://rclone.org/img/logo_on_light__horizontal_color.svg" alt="rclone logo" style="float:right; padding: 5px;" >

- [About rclone](#about)
- [What can rclone do for you?](#what)
- [What features does rclone have?](#features)
- [What providers does rclone support?](#providers)
- [Download](https://rclone.org/downloads/)
- [Install](https://rclone.org/install/)
- [Donate.](https://rclone.org/donate/)

## About rclone {#about}

Rclone is a command line program to manage files on cloud storage. It
is a feature rich alternative to cloud vendors' web storage
interfaces. [Over 40 cloud storage products](#providers) support
rclone including S3 object stores, business & consumer file storage
services, as well as standard transfer protocols.

Rclone has powerful cloud equivalents to the unix commands rsync, cp,
mv, mount, ls, ncdu, tree, rm, and cat. Rclone's familiar syntax
includes shell pipeline support, and `--dry-run` protection. It is
used at the command line, in scripts or via its [API](/rc).

Users call rclone *"The Swiss army knife of cloud storage"*, and
*"Technology indistinguishable from magic"*.

Rclone really looks after your data. It preserves timestamps and
verifies checksums at all times. Transfers over limited bandwidth;
intermittent connections, or subject to quota can be restarted, from
the last good file transferred. You can
[check](https://rclone.org/commands/rclone_check/) the integrity of your files. Where
possible, rclone employs server side transfers to minimise local
bandwidth use and transfers from one provider to another without
using local disk.

Virtual backends wrap local and cloud file systems to apply
[encryption](https://rclone.org/crypt/), 
[caching](https://rclone.org/cache/),
[chunking](https://rclone.org/chunker/) and
[joining](https://rclone.org/union/).

Rclone [mounts](https://rclone.org/commands/rclone_mount/) any local, cloud or
virtual filesystem as a disk on Windows,
macOS, linux and FreeBSD, and also serves these over
[SFTP](https://rclone.org/commands/rclone_serve_sftp/),
[HTTP](https://rclone.org/commands/rclone_serve_http/),
[WebDAV](https://rclone.org/commands/rclone_serve_webdav/),
[FTP](https://rclone.org/commands/rclone_serve_ftp/) and
[DLNA](https://rclone.org/commands/rclone_serve_dlna/).

Rclone is mature, open source software originally inspired by rsync
and written in [Go](https://golang.org). The friendly support
community are familiar with varied use cases. Official Ubuntu, Debian,
Fedora, Brew and Chocolatey repos. include rclone. For the latest
version [downloading from rclone.org](https://rclone.org/downloads/) is recommended.

Rclone is widely used on Linux, Windows and Mac. Third party
developers create innovative backup, restore, GUI and business
process solutions using the rclone command line or API.

Rclone does the heavy lifting of communicating with cloud storage.

## What can rclone do for you? {#what}

Rclone helps you:

- Backup (and encrypt) files to cloud storage
- Restore (and decrypt) files from cloud storage
- Mirror cloud data to other cloud services or locally
- Migrate data to cloud, or between cloud storage vendors
- Mount multiple, encrypted, cached or diverse cloud storage as a disk
- Analyse and account for data held on cloud storage using [lsf](https://rclone.org/commands/rclone_lsf/), [ljson](https://rclone.org/commands/rclone_lsjson/), [size](https://rclone.org/commands/rclone_size/), [ncdu](https://rclone.org/commands/rclone_ncdu/)
- [Union](https://rclone.org/union/) file systems together to present multiple local and/or cloud file systems as one

## Features {#features}

- Transfers
    - MD5, SHA1 hashes are checked at all times for file integrity
    - Timestamps are preserved on files
    - Operations can be restarted at any time
    - Can be to and from network, eg two different cloud providers
    - Can use multi-threaded downloads to local disk
- [Copy](https://rclone.org/commands/rclone_copy/) new or changed files to cloud storage
- [Sync](https://rclone.org/commands/rclone_sync/) (one way) to make a directory identical
- [Move](https://rclone.org/commands/rclone_move/) files to cloud storage deleting the local after verification
- [Check](https://rclone.org/commands/rclone_check/) hashes and for missing/extra files
- [Mount](https://rclone.org/commands/rclone_mount/) your cloud storage as a network disk
- [Serve](https://rclone.org/commands/rclone_serve/) local or remote files over [HTTP](https://rclone.org/commands/rclone_serve_http/)/[WebDav](https://rclone.org/commands/rclone_serve_webdav/)/[FTP](https://rclone.org/commands/rclone_serve_ftp/)/[SFTP](https://rclone.org/commands/rclone_serve_sftp/)/[dlna](https://rclone.org/commands/rclone_serve_dlna/)
- Experimental [Web based GUI](https://rclone.org/gui/)

## Supported providers {#providers}

(There are many others, built on standard protocols such as
WebDAV or S3, that work out of the box.)


- 1Fichier
- Alibaba Cloud (Aliyun) Object Storage System (OSS)
- Amazon Drive
- Amazon S3
- Backblaze B2
- Box
- Ceph
- Citrix ShareFile
- C14
- DigitalOcean Spaces
- Dreamhost
- Dropbox
- FTP
- Google Cloud Storage
- Google Drive
- Google Photos
- HTTP
- Hubic
- Jottacloud
- IBM COS S3
- Koofr
- Mail.ru Cloud
- Memset Memstore
- Mega
- Memory
- Microsoft Azure Blob Storage
- Microsoft OneDrive
- Minio
- Nextcloud
- OVH
- OpenDrive
- OpenStack Swift
- Oracle Cloud Storage
- ownCloud
- pCloud
- premiumize.me
- put.io
- QingStor
- Rackspace Cloud Files
- rsync.net
- Scaleway
- Seafile
- SFTP
- StackPath
- SugarSync
- Tardigrade
- Wasabi
- WebDAV
- Yandex Disk
- The local filesystem


Links

  *  [Home page](https://rclone.org/)
  *  [GitHub project page for source and bug tracker](https://github.com/rclone/rclone)
  *  [Rclone Forum](https://forum.rclone.org)
  * [Downloads](https://rclone.org/downloads/)

# Install #

Rclone is a Go program and comes as a single binary file.

## Quickstart ##

  * [Download](https://rclone.org/downloads/) the relevant binary.
  * Extract the `rclone` or `rclone.exe` binary from the archive
  * Run `rclone config` to setup. See [rclone config docs](https://rclone.org/docs/) for more details.

See below for some expanded Linux / macOS instructions.

See the [Usage section](https://rclone.org/docs/#usage) of the docs for how to use rclone, or
run `rclone -h`.

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

Run `rclone config` to setup. See [rclone config docs](https://rclone.org/docs/) for more details.

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

Run `rclone config` to setup. See [rclone config docs](https://rclone.org/docs/) for more details.

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
also use version tags, eg `:1.49.1`, `:1.49` or `:1`.

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

Make sure you have at least [Go](https://golang.org/) 1.7
installed.  [Download go](https://golang.org/dl/) if necessary.  The
latest release is recommended. Then

    git clone https://github.com/rclone/rclone.git
    cd rclone
    go build
    ./rclone version

You can also build and install rclone in the
[GOPATH](https://github.com/golang/go/wiki/GOPATH) (which defaults to
`~/go`) with:

    go get -u -v github.com/rclone/rclone

and this will build the binary in `$GOPATH/bin` (`~/go/bin/rclone` by
default) after downloading the source to
`$GOPATH/src/github.com/rclone/rclone` (`~/go/src/github.com/rclone/rclone`
by default).

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

Configure
---------

First, you'll need to configure rclone.  As the object storage systems
have quite complicated authentication these are kept in a config file.
(See the `--config` entry for how to find the config file and choose
its location.)

The easiest way to make the config is to run rclone with the config
option:

    rclone config

See the following for detailed instructions for

  * [1Fichier](https://rclone.org/fichier/)
  * [Alias](https://rclone.org/alias/)
  * [Amazon Drive](https://rclone.org/amazonclouddrive/)
  * [Amazon S3](https://rclone.org/s3/)
  * [Backblaze B2](https://rclone.org/b2/)
  * [Box](https://rclone.org/box/)
  * [Cache](https://rclone.org/cache/)
  * [Chunker](https://rclone.org/chunker/) - transparently splits large files for other remotes
  * [Citrix ShareFile](https://rclone.org/sharefile/)
  * [Crypt](https://rclone.org/crypt/) - to encrypt other remotes
  * [DigitalOcean Spaces](https://rclone.org/s3/#digitalocean-spaces)
  * [Dropbox](https://rclone.org/dropbox/)
  * [FTP](https://rclone.org/ftp/)
  * [Google Cloud Storage](https://rclone.org/googlecloudstorage/)
  * [Google Drive](https://rclone.org/drive/)
  * [Google Photos](https://rclone.org/googlephotos/)
  * [HTTP](https://rclone.org/http/)
  * [Hubic](https://rclone.org/hubic/)
  * [Jottacloud / GetSky.no](https://rclone.org/jottacloud/)
  * [Koofr](https://rclone.org/koofr/)
  * [Mail.ru Cloud](https://rclone.org/mailru/)
  * [Mega](https://rclone.org/mega/)
  * [Memory](https://rclone.org/memory/)
  * [Microsoft Azure Blob Storage](https://rclone.org/azureblob/)
  * [Microsoft OneDrive](https://rclone.org/onedrive/)
  * [OpenStack Swift / Rackspace Cloudfiles / Memset Memstore](https://rclone.org/swift/)
  * [OpenDrive](https://rclone.org/opendrive/)
  * [Pcloud](https://rclone.org/pcloud/)
  * [premiumize.me](https://rclone.org/premiumizeme/)
  * [put.io](https://rclone.org/putio/)
  * [QingStor](https://rclone.org/qingstor/)
  * [Seafile](https://rclone.org/seafile/)
  * [SFTP](https://rclone.org/sftp/)
  * [SugarSync](https://rclone.org/sugarsync/)
  * [Tardigrade](https://rclone.org/tardigrade/)
  * [Union](https://rclone.org/union/)
  * [WebDAV](https://rclone.org/webdav/)
  * [Yandex Disk](https://rclone.org/yandex/)
  * [The local filesystem](https://rclone.org/local/)

Usage
-----

Rclone syncs a directory tree from one storage system to another.

Its syntax is like this

    Syntax: [options] subcommand <parameters> <parameters...>

Source and destination paths are specified by the name you gave the
storage system in the config file then the sub path, eg
"drive:myfolder" to look at "myfolder" in Google drive.

You can define as many storage paths as you like in the config file.

Subcommands
-----------

rclone uses a system of subcommands.  For example

    rclone ls remote:path # lists a remote
    rclone copy /local/path remote:path # copies /local/path to the remote
    rclone sync /local/path remote:path # syncs /local/path to the remote

# rclone config

Enter an interactive configuration session.

## Synopsis

Enter an interactive configuration session where you can setup new
remotes and manage existing ones. You may also set or remove a
password to protect your configuration.


```
rclone config [flags]
```

## Options

```
  -h, --help   help for config
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.
* [rclone config create](https://rclone.org/commands/rclone_config_create/)	 - Create a new remote with name, type and options.
* [rclone config delete](https://rclone.org/commands/rclone_config_delete/)	 - Delete an existing remote `name`.
* [rclone config disconnect](https://rclone.org/commands/rclone_config_disconnect/)	 - Disconnects user from remote
* [rclone config dump](https://rclone.org/commands/rclone_config_dump/)	 - Dump the config file as JSON.
* [rclone config edit](https://rclone.org/commands/rclone_config_edit/)	 - Enter an interactive configuration session.
* [rclone config file](https://rclone.org/commands/rclone_config_file/)	 - Show path of configuration file in use.
* [rclone config password](https://rclone.org/commands/rclone_config_password/)	 - Update password in an existing remote.
* [rclone config providers](https://rclone.org/commands/rclone_config_providers/)	 - List in JSON format all the providers and options.
* [rclone config reconnect](https://rclone.org/commands/rclone_config_reconnect/)	 - Re-authenticates user with remote.
* [rclone config show](https://rclone.org/commands/rclone_config_show/)	 - Print (decrypted) config file, or the config for a single remote.
* [rclone config update](https://rclone.org/commands/rclone_config_update/)	 - Update options in an existing remote.
* [rclone config userinfo](https://rclone.org/commands/rclone_config_userinfo/)	 - Prints info about logged in user of remote.

# rclone copy

Copy files from source to dest, skipping already copied

## Synopsis


Copy the source to the destination.  Doesn't transfer
unchanged files, testing by size and modification time or
MD5SUM.  Doesn't delete files from the destination.

Note that it is always the contents of the directory that is synced,
not the directory so when source:path is a directory, it's the
contents of source:path that are copied, not the directory name and
contents.

If dest:path doesn't exist, it is created and the source:path contents
go there.

For example

    rclone copy source:sourcepath dest:destpath

Let's say there are two files in sourcepath

    sourcepath/one.txt
    sourcepath/two.txt

This copies them to

    destpath/one.txt
    destpath/two.txt

Not to

    destpath/sourcepath/one.txt
    destpath/sourcepath/two.txt

If you are familiar with `rsync`, rclone always works as if you had
written a trailing / - meaning "copy the contents of this directory".
This applies to all commands and whether you are talking about the
source or destination.

See the [--no-traverse](https://rclone.org/docs/#no-traverse) option for controlling
whether rclone lists the destination directory or not.  Supplying this
option when copying a small number of files into a large destination
can speed transfers up greatly.

For example, if you have many files in /path/to/src but only a few of
them change every day, you can copy all the files which have changed
recently very efficiently like this:

    rclone copy --max-age 24h --no-traverse /path/to/src remote:

**Note**: Use the `-P`/`--progress` flag to view real-time transfer statistics


```
rclone copy source:path dest:path [flags]
```

## Options

```
      --create-empty-src-dirs   Create empty source dirs on destination after copy
  -h, --help                    help for copy
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone sync

Make source and dest identical, modifying destination only.

## Synopsis


Sync the source to the destination, changing the destination
only.  Doesn't transfer unchanged files, testing by size and
modification time or MD5SUM.  Destination is updated to match
source, including deleting files if necessary.

**Important**: Since this can cause data loss, test first with the
`--dry-run` flag to see exactly what would be copied and deleted.

Note that files in the destination won't be deleted if there were any
errors at any point.

It is always the contents of the directory that is synced, not the
directory so when source:path is a directory, it's the contents of
source:path that are copied, not the directory name and contents.  See
extended explanation in the `copy` command above if unsure.

If dest:path doesn't exist, it is created and the source:path contents
go there.

**Note**: Use the `-P`/`--progress` flag to view real-time transfer statistics


```
rclone sync source:path dest:path [flags]
```

## Options

```
      --create-empty-src-dirs   Create empty source dirs on destination after sync
  -h, --help                    help for sync
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone move

Move files from source to dest.

## Synopsis


Moves the contents of the source directory to the destination
directory. Rclone will error if the source and destination overlap and
the remote does not support a server side directory move operation.

If no filters are in use and if possible this will server side move
`source:path` into `dest:path`. After this `source:path` will no
longer exist.

Otherwise for each file in `source:path` selected by the filters (if
any) this will move it into `dest:path`.  If possible a server side
move will be used, otherwise it will copy it (server side if possible)
into `dest:path` then delete the original (if no errors on copy) in
`source:path`.

If you want to delete empty source directories after move, use the --delete-empty-src-dirs flag.

See the [--no-traverse](https://rclone.org/docs/#no-traverse) option for controlling
whether rclone lists the destination directory or not.  Supplying this
option when moving a small number of files into a large destination
can speed transfers up greatly.

**Important**: Since this can cause data loss, test first with the
--dry-run flag.

**Note**: Use the `-P`/`--progress` flag to view real-time transfer statistics.


```
rclone move source:path dest:path [flags]
```

## Options

```
      --create-empty-src-dirs   Create empty source dirs on destination after move
      --delete-empty-src-dirs   Delete empty source dirs after move
  -h, --help                    help for move
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone delete

Remove the contents of path.

## Synopsis


Remove the files in path.  Unlike `purge` it obeys include/exclude
filters so can be used to selectively delete files.

`rclone delete` only deletes objects but leaves the directory structure
alone. If you want to delete a directory and all of its contents use
`rclone purge`

If you supply the --rmdirs flag, it will remove all empty directories along with it.

Eg delete all files bigger than 100MBytes

Check what would be deleted first (use either)

    rclone --min-size 100M lsl remote:path
    rclone --dry-run --min-size 100M delete remote:path

Then delete

    rclone --min-size 100M delete remote:path

That reads "delete everything with a minimum size of 100 MB", hence
delete all files bigger than 100MBytes.


```
rclone delete remote:path [flags]
```

## Options

```
  -h, --help     help for delete
      --rmdirs   rmdirs removes empty directories but leaves root intact
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone purge

Remove the path and all of its contents.

## Synopsis


Remove the path and all of its contents.  Note that this does not obey
include/exclude filters - everything will be removed.  Use `delete` if
you want to selectively delete files.


```
rclone purge remote:path [flags]
```

## Options

```
  -h, --help   help for purge
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone mkdir

Make the path if it doesn't already exist.

## Synopsis

Make the path if it doesn't already exist.

```
rclone mkdir remote:path [flags]
```

## Options

```
  -h, --help   help for mkdir
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone rmdir

Remove the path if empty.

## Synopsis


Remove the path.  Note that you can't remove a path with
objects in it, use purge for that.

```
rclone rmdir remote:path [flags]
```

## Options

```
  -h, --help   help for rmdir
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone check

Checks the files in the source and destination match.

## Synopsis


Checks the files in the source and destination match.  It compares
sizes and hashes (MD5 or SHA1) and logs a report of files which don't
match.  It doesn't alter the source or destination.

If you supply the --size-only flag, it will only compare the sizes not
the hashes as well.  Use this for a quick check.

If you supply the --download flag, it will download the data from
both remotes and check them against each other on the fly.  This can
be useful for remotes that don't support hashes or if you really want
to check all the data.

If you supply the --one-way flag, it will only check that files in source
match the files in destination, not the other way around. Meaning extra files in
destination that are not in the source will not trigger an error.


```
rclone check source:path dest:path [flags]
```

## Options

```
      --download   Check by downloading rather than with hash.
  -h, --help       help for check
      --one-way    Check one way only, source files must exist on remote
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone ls

List the objects in the path with size and path.

## Synopsis


Lists the objects in the source path to standard output in a human
readable format with size and path. Recurses by default.

Eg

    $ rclone ls swift:bucket
        60295 bevajer5jef
        90613 canole
        94467 diwogej7
        37600 fubuwic


Any of the filtering options can be applied to this command.

There are several related list commands

  * `ls` to list size and path of objects only
  * `lsl` to list modification time, size and path of objects only
  * `lsd` to list directories only
  * `lsf` to list objects and directories in easy to parse format
  * `lsjson` to list objects and directories in JSON format

`ls`,`lsl`,`lsd` are designed to be human readable.
`lsf` is designed to be human and machine readable.
`lsjson` is designed to be machine readable.

Note that `ls` and `lsl` recurse by default - use "--max-depth 1" to stop the recursion.

The other list commands `lsd`,`lsf`,`lsjson` do not recurse by default - use "-R" to make them recurse.

Listing a non existent directory will produce an error except for
remotes which can't have empty directories (eg s3, swift, gcs, etc -
the bucket based remotes).


```
rclone ls remote:path [flags]
```

## Options

```
  -h, --help   help for ls
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone lsd

List all directories/containers/buckets in the path.

## Synopsis


Lists the directories in the source path to standard output. Does not
recurse by default.  Use the -R flag to recurse.

This command lists the total size of the directory (if known, -1 if
not), the modification time (if known, the current time if not), the
number of objects in the directory (if known, -1 if not) and the name
of the directory, Eg

    $ rclone lsd swift:
          494000 2018-04-26 08:43:20     10000 10000files
              65 2018-04-26 08:43:20         1 1File

Or

    $ rclone lsd drive:test
              -1 2016-10-17 17:41:53        -1 1000files
              -1 2017-01-03 14:40:54        -1 2500files
              -1 2017-07-08 14:39:28        -1 4000files

If you just want the directory names use "rclone lsf --dirs-only".


Any of the filtering options can be applied to this command.

There are several related list commands

  * `ls` to list size and path of objects only
  * `lsl` to list modification time, size and path of objects only
  * `lsd` to list directories only
  * `lsf` to list objects and directories in easy to parse format
  * `lsjson` to list objects and directories in JSON format

`ls`,`lsl`,`lsd` are designed to be human readable.
`lsf` is designed to be human and machine readable.
`lsjson` is designed to be machine readable.

Note that `ls` and `lsl` recurse by default - use "--max-depth 1" to stop the recursion.

The other list commands `lsd`,`lsf`,`lsjson` do not recurse by default - use "-R" to make them recurse.

Listing a non existent directory will produce an error except for
remotes which can't have empty directories (eg s3, swift, gcs, etc -
the bucket based remotes).


```
rclone lsd remote:path [flags]
```

## Options

```
  -h, --help        help for lsd
  -R, --recursive   Recurse into the listing.
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone lsl

List the objects in path with modification time, size and path.

## Synopsis


Lists the objects in the source path to standard output in a human
readable format with modification time, size and path. Recurses by default.

Eg

    $ rclone lsl swift:bucket
        60295 2016-06-25 18:55:41.062626927 bevajer5jef
        90613 2016-06-25 18:55:43.302607074 canole
        94467 2016-06-25 18:55:43.046609333 diwogej7
        37600 2016-06-25 18:55:40.814629136 fubuwic


Any of the filtering options can be applied to this command.

There are several related list commands

  * `ls` to list size and path of objects only
  * `lsl` to list modification time, size and path of objects only
  * `lsd` to list directories only
  * `lsf` to list objects and directories in easy to parse format
  * `lsjson` to list objects and directories in JSON format

`ls`,`lsl`,`lsd` are designed to be human readable.
`lsf` is designed to be human and machine readable.
`lsjson` is designed to be machine readable.

Note that `ls` and `lsl` recurse by default - use "--max-depth 1" to stop the recursion.

The other list commands `lsd`,`lsf`,`lsjson` do not recurse by default - use "-R" to make them recurse.

Listing a non existent directory will produce an error except for
remotes which can't have empty directories (eg s3, swift, gcs, etc -
the bucket based remotes).


```
rclone lsl remote:path [flags]
```

## Options

```
  -h, --help   help for lsl
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone md5sum

Produces an md5sum file for all the objects in the path.

## Synopsis


Produces an md5sum file for all the objects in the path.  This
is in the same format as the standard md5sum tool produces.


```
rclone md5sum remote:path [flags]
```

## Options

```
  -h, --help   help for md5sum
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone sha1sum

Produces an sha1sum file for all the objects in the path.

## Synopsis


Produces an sha1sum file for all the objects in the path.  This
is in the same format as the standard sha1sum tool produces.


```
rclone sha1sum remote:path [flags]
```

## Options

```
  -h, --help   help for sha1sum
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone size

Prints the total size and number of objects in remote:path.

## Synopsis

Prints the total size and number of objects in remote:path.

```
rclone size remote:path [flags]
```

## Options

```
  -h, --help   help for size
      --json   format output as JSON
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone version

Show the version number.

## Synopsis


Show the version number, the go version and the architecture.

Eg

    $ rclone version
    rclone v1.41
    - os/arch: linux/amd64
    - go version: go1.10

If you supply the --check flag, then it will do an online check to
compare your version with the latest release and the latest beta.

    $ rclone version --check
    yours:  1.42.0.6
    latest: 1.42          (released 2018-06-16)
    beta:   1.42.0.5      (released 2018-06-17)

Or

    $ rclone version --check
    yours:  1.41
    latest: 1.42          (released 2018-06-16)
      upgrade: https://downloads.rclone.org/v1.42
    beta:   1.42.0.5      (released 2018-06-17)
      upgrade: https://beta.rclone.org/v1.42-005-g56e1e820



```
rclone version [flags]
```

## Options

```
      --check   Check for new version.
  -h, --help    help for version
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone cleanup

Clean up the remote if possible

## Synopsis


Clean up the remote if possible.  Empty the trash or delete old file
versions. Not supported by all remotes.


```
rclone cleanup remote:path [flags]
```

## Options

```
  -h, --help   help for cleanup
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone dedupe

Interactively find duplicate files and delete/rename them.

## Synopsis


By default `dedupe` interactively finds duplicate files and offers to
delete all but one or rename them to be different. Only useful with
Google Drive which can have duplicate file names.

In the first pass it will merge directories with the same name.  It
will do this iteratively until all the identical directories have been
merged.

The `dedupe` command will delete all but one of any identical (same
md5sum) files it finds without confirmation.  This means that for most
duplicated files the `dedupe` command will not be interactive.  You
can use `--dry-run` to see what would happen without doing anything.

Here is an example run.

Before - with duplicates

    $ rclone lsl drive:dupes
      6048320 2016-03-05 16:23:16.798000000 one.txt
      6048320 2016-03-05 16:23:11.775000000 one.txt
       564374 2016-03-05 16:23:06.731000000 one.txt
      6048320 2016-03-05 16:18:26.092000000 one.txt
      6048320 2016-03-05 16:22:46.185000000 two.txt
      1744073 2016-03-05 16:22:38.104000000 two.txt
       564374 2016-03-05 16:22:52.118000000 two.txt

Now the `dedupe` session

    $ rclone dedupe drive:dupes
    2016/03/05 16:24:37 Google drive root 'dupes': Looking for duplicates using interactive mode.
    one.txt: Found 4 duplicates - deleting identical copies
    one.txt: Deleting 2/3 identical duplicates (md5sum "1eedaa9fe86fd4b8632e2ac549403b36")
    one.txt: 2 duplicates remain
      1:      6048320 bytes, 2016-03-05 16:23:16.798000000, md5sum 1eedaa9fe86fd4b8632e2ac549403b36
      2:       564374 bytes, 2016-03-05 16:23:06.731000000, md5sum 7594e7dc9fc28f727c42ee3e0749de81
    s) Skip and do nothing
    k) Keep just one (choose which in next step)
    r) Rename all to be different (by changing file.jpg to file-1.jpg)
    s/k/r> k
    Enter the number of the file to keep> 1
    one.txt: Deleted 1 extra copies
    two.txt: Found 3 duplicates - deleting identical copies
    two.txt: 3 duplicates remain
      1:       564374 bytes, 2016-03-05 16:22:52.118000000, md5sum 7594e7dc9fc28f727c42ee3e0749de81
      2:      6048320 bytes, 2016-03-05 16:22:46.185000000, md5sum 1eedaa9fe86fd4b8632e2ac549403b36
      3:      1744073 bytes, 2016-03-05 16:22:38.104000000, md5sum 851957f7fb6f0bc4ce76be966d336802
    s) Skip and do nothing
    k) Keep just one (choose which in next step)
    r) Rename all to be different (by changing file.jpg to file-1.jpg)
    s/k/r> r
    two-1.txt: renamed from: two.txt
    two-2.txt: renamed from: two.txt
    two-3.txt: renamed from: two.txt

The result being

    $ rclone lsl drive:dupes
      6048320 2016-03-05 16:23:16.798000000 one.txt
       564374 2016-03-05 16:22:52.118000000 two-1.txt
      6048320 2016-03-05 16:22:46.185000000 two-2.txt
      1744073 2016-03-05 16:22:38.104000000 two-3.txt

Dedupe can be run non interactively using the `--dedupe-mode` flag or by using an extra parameter with the same value

  * `--dedupe-mode interactive` - interactive as above.
  * `--dedupe-mode skip` - removes identical files then skips anything left.
  * `--dedupe-mode first` - removes identical files then keeps the first one.
  * `--dedupe-mode newest` - removes identical files then keeps the newest one.
  * `--dedupe-mode oldest` - removes identical files then keeps the oldest one.
  * `--dedupe-mode largest` - removes identical files then keeps the largest one.
  * `--dedupe-mode smallest` - removes identical files then keeps the smallest one.
  * `--dedupe-mode rename` - removes identical files then renames the rest to be different.

For example to rename all the identically named photos in your Google Photos directory, do

    rclone dedupe --dedupe-mode rename "drive:Google Photos"

Or

    rclone dedupe rename "drive:Google Photos"


```
rclone dedupe [mode] remote:path [flags]
```

## Options

```
      --dedupe-mode string   Dedupe mode interactive|skip|first|newest|oldest|largest|smallest|rename. (default "interactive")
  -h, --help                 help for dedupe
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone about

Get quota information from the remote.

## Synopsis


Get quota information from the remote, like bytes used/free/quota and bytes
used in the trash. Not supported by all remotes.

This will print to stdout something like this:

    Total:   17G
    Used:    7.444G
    Free:    1.315G
    Trashed: 100.000M
    Other:   8.241G

Where the fields are:

  * Total: total size available.
  * Used: total size used
  * Free: total amount this user could upload.
  * Trashed: total amount in the trash
  * Other: total amount in other storage (eg Gmail, Google Photos)
  * Objects: total number of objects in the storage

Note that not all the backends provide all the fields - they will be
missing if they are not known for that backend.  Where it is known
that the value is unlimited the value will also be omitted.

Use the --full flag to see the numbers written out in full, eg

    Total:   18253611008
    Used:    7993453766
    Free:    1411001220
    Trashed: 104857602
    Other:   8849156022

Use the --json flag for a computer readable output, eg

    {
        "total": 18253611008,
        "used": 7993453766,
        "trashed": 104857602,
        "other": 8849156022,
        "free": 1411001220
    }


```
rclone about remote: [flags]
```

## Options

```
      --full   Full numbers instead of SI units
  -h, --help   help for about
      --json   Format output as JSON
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone authorize

Remote authorization.

## Synopsis


Remote authorization. Used to authorize a remote or headless
rclone from a machine with a browser - use as instructed by
rclone config.

Use the --auth-no-open-browser to prevent rclone to open auth
link in default browser automatically.

```
rclone authorize [flags]
```

## Options

```
      --auth-no-open-browser   Do not automatically open auth link in default browser
  -h, --help                   help for authorize
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone backend

Run a backend specific command.

## Synopsis


This runs a backend specific command. The commands themselves (except
for "help" and "features") are defined by the backends and you should
see the backend docs for definitions.

You can discover what commands a backend implements by using

    rclone backend help remote:
    rclone backend help <backendname>

You can also discover information about the backend using (see
[operations/fsinfo](https://rclone.org/rc/#operations/fsinfo) in the remote control docs
for more info).

    rclone backend features remote:

Pass options to the backend command with -o. This should be key=value or key, eg:

    rclone backend stats remote:path stats -o format=json -o long

Pass arguments to the backend by placing them on the end of the line

    rclone backend cleanup remote:path file1 file2 file3

Note to run these commands on a running backend then see
[backend/command](https://rclone.org/rc/#backend/command) in the rc docs.


```
rclone backend <command> remote:path [opts] <args> [flags]
```

## Options

```
  -h, --help                 help for backend
      --json                 Always output in JSON format.
  -o, --option stringArray   Option in the form name=value or name.
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone cat

Concatenates any files and sends them to stdout.

## Synopsis


rclone cat sends any files to standard output.

You can use it like this to output a single file

    rclone cat remote:path/to/file

Or like this to output any file in dir or its subdirectories.

    rclone cat remote:path/to/dir

Or like this to output any .txt files in dir or its subdirectories.

    rclone --include "*.txt" cat remote:path/to/dir

Use the --head flag to print characters only at the start, --tail for
the end and --offset and --count to print a section in the middle.
Note that if offset is negative it will count from the end, so
--offset -1 --count 1 is equivalent to --tail 1.


```
rclone cat remote:path [flags]
```

## Options

```
      --count int    Only print N characters. (default -1)
      --discard      Discard the output instead of printing.
      --head int     Only print the first N characters.
  -h, --help         help for cat
      --offset int   Start printing at offset N (or from end if -ve).
      --tail int     Only print the last N characters.
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone config create

Create a new remote with name, type and options.

## Synopsis


Create a new remote of `name` with `type` and options.  The options
should be passed in pairs of `key` `value`.

For example to make a swift remote of name myremote using auto config
you would do:

    rclone config create myremote swift env_auth true

Note that if the config process would normally ask a question the
default is taken.  Each time that happens rclone will print a message
saying how to affect the value taken.

If any of the parameters passed is a password field, then rclone will
automatically obscure them if they aren't already obscured before
putting them in the config file.

**NB** If the password parameter is 22 characters or longer and
consists only of base64 characters then rclone can get confused about
whether the password is already obscured or not and put unobscured
passwords into the config file. If you want to be 100% certain that
the passwords get obscured then use the "--obscure" flag, or if you
are 100% certain you are already passing obscured passwords then use
"--no-obscure".  You can also set osbscured passwords using the
"rclone config password" command.

So for example if you wanted to configure a Google Drive remote but
using remote authorization you would do this:

    rclone config create mydrive drive config_is_local false


```
rclone config create `name` `type` [`key` `value`]* [flags]
```

## Options

```
  -h, --help         help for create
      --no-obscure   Force any passwords not to be obscured.
      --obscure      Force any passwords to be obscured.
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone config](https://rclone.org/commands/rclone_config/)	 - Enter an interactive configuration session.

# rclone config delete

Delete an existing remote `name`.

## Synopsis

Delete an existing remote `name`.

```
rclone config delete `name` [flags]
```

## Options

```
  -h, --help   help for delete
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone config](https://rclone.org/commands/rclone_config/)	 - Enter an interactive configuration session.

# rclone config disconnect

Disconnects user from remote

## Synopsis


This disconnects the remote: passed in to the cloud storage system.

This normally means revoking the oauth token.

To reconnect use "rclone config reconnect".


```
rclone config disconnect remote: [flags]
```

## Options

```
  -h, --help   help for disconnect
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone config](https://rclone.org/commands/rclone_config/)	 - Enter an interactive configuration session.

# rclone config dump

Dump the config file as JSON.

## Synopsis

Dump the config file as JSON.

```
rclone config dump [flags]
```

## Options

```
  -h, --help   help for dump
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone config](https://rclone.org/commands/rclone_config/)	 - Enter an interactive configuration session.

# rclone config edit

Enter an interactive configuration session.

## Synopsis

Enter an interactive configuration session where you can setup new
remotes and manage existing ones. You may also set or remove a
password to protect your configuration.


```
rclone config edit [flags]
```

## Options

```
  -h, --help   help for edit
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone config](https://rclone.org/commands/rclone_config/)	 - Enter an interactive configuration session.

# rclone config file

Show path of configuration file in use.

## Synopsis

Show path of configuration file in use.

```
rclone config file [flags]
```

## Options

```
  -h, --help   help for file
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone config](https://rclone.org/commands/rclone_config/)	 - Enter an interactive configuration session.

# rclone config password

Update password in an existing remote.

## Synopsis


Update an existing remote's password. The password
should be passed in pairs of `key` `value`.

For example to set password of a remote of name myremote you would do:

    rclone config password myremote fieldname mypassword

This command is obsolete now that "config update" and "config create"
both support obscuring passwords directly.


```
rclone config password `name` [`key` `value`]+ [flags]
```

## Options

```
  -h, --help   help for password
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone config](https://rclone.org/commands/rclone_config/)	 - Enter an interactive configuration session.

# rclone config providers

List in JSON format all the providers and options.

## Synopsis

List in JSON format all the providers and options.

```
rclone config providers [flags]
```

## Options

```
  -h, --help   help for providers
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone config](https://rclone.org/commands/rclone_config/)	 - Enter an interactive configuration session.

# rclone config reconnect

Re-authenticates user with remote.

## Synopsis


This reconnects remote: passed in to the cloud storage system.

To disconnect the remote use "rclone config disconnect".

This normally means going through the interactive oauth flow again.


```
rclone config reconnect remote: [flags]
```

## Options

```
  -h, --help   help for reconnect
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone config](https://rclone.org/commands/rclone_config/)	 - Enter an interactive configuration session.

# rclone config show

Print (decrypted) config file, or the config for a single remote.

## Synopsis

Print (decrypted) config file, or the config for a single remote.

```
rclone config show [<remote>] [flags]
```

## Options

```
  -h, --help   help for show
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone config](https://rclone.org/commands/rclone_config/)	 - Enter an interactive configuration session.

# rclone config update

Update options in an existing remote.

## Synopsis


Update an existing remote's options. The options should be passed in
in pairs of `key` `value`.

For example to update the env_auth field of a remote of name myremote
you would do:

    rclone config update myremote swift env_auth true

If any of the parameters passed is a password field, then rclone will
automatically obscure them if they aren't already obscured before
putting them in the config file.

**NB** If the password parameter is 22 characters or longer and
consists only of base64 characters then rclone can get confused about
whether the password is already obscured or not and put unobscured
passwords into the config file. If you want to be 100% certain that
the passwords get obscured then use the "--obscure" flag, or if you
are 100% certain you are already passing obscured passwords then use
"--no-obscure".  You can also set osbscured passwords using the
"rclone config password" command.

If the remote uses OAuth the token will be updated, if you don't
require this add an extra parameter thus:

    rclone config update myremote swift env_auth true config_refresh_token false


```
rclone config update `name` [`key` `value`]+ [flags]
```

## Options

```
  -h, --help         help for update
      --no-obscure   Force any passwords not to be obscured.
      --obscure      Force any passwords to be obscured.
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone config](https://rclone.org/commands/rclone_config/)	 - Enter an interactive configuration session.

# rclone config userinfo

Prints info about logged in user of remote.

## Synopsis


This prints the details of the person logged in to the cloud storage
system.


```
rclone config userinfo remote: [flags]
```

## Options

```
  -h, --help   help for userinfo
      --json   Format output as JSON
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone config](https://rclone.org/commands/rclone_config/)	 - Enter an interactive configuration session.

# rclone copyto

Copy files from source to dest, skipping already copied

## Synopsis


If source:path is a file or directory then it copies it to a file or
directory named dest:path.

This can be used to upload single files to other than their current
name.  If the source is a directory then it acts exactly like the copy
command.

So

    rclone copyto src dst

where src and dst are rclone paths, either remote:path or
/path/to/local or C:\windows\path\if\on\windows.

This will:

    if src is file
        copy it to dst, overwriting an existing file if it exists
    if src is directory
        copy it to dst, overwriting existing files if they exist
        see copy command for full details

This doesn't transfer unchanged files, testing by size and
modification time or MD5SUM.  It doesn't delete files from the
destination.

**Note**: Use the `-P`/`--progress` flag to view real-time transfer statistics


```
rclone copyto source:path dest:path [flags]
```

## Options

```
  -h, --help   help for copyto
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone copyurl

Copy url content to dest.

## Synopsis


Download a URL's content and copy it to the destination without saving
it in temporary storage.

Setting --auto-filename will cause the file name to be retrieved from
the from URL (after any redirections) and used in the destination
path.

Setting --no-clobber will prevent overwriting file on the 
destination if there is one with the same name.

Setting --stdout or making the output file name "-" will cause the
output to be written to standard output.


```
rclone copyurl https://example.com dest:path [flags]
```

## Options

```
  -a, --auto-filename   Get the file name from the URL and use it for destination file path
  -h, --help            help for copyurl
      --no-clobber      Prevent overwriting file with same name
      --stdout          Write the output to stdout rather than a file
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone cryptcheck

Cryptcheck checks the integrity of a crypted remote.

## Synopsis


rclone cryptcheck checks a remote against a crypted remote.  This is
the equivalent of running rclone check, but able to check the
checksums of the crypted remote.

For it to work the underlying remote of the cryptedremote must support
some kind of checksum.

It works by reading the nonce from each file on the cryptedremote: and
using that to encrypt each file on the remote:.  It then checks the
checksum of the underlying file on the cryptedremote: against the
checksum of the file it has just encrypted.

Use it like this

    rclone cryptcheck /path/to/files encryptedremote:path

You can use it like this also, but that will involve downloading all
the files in remote:path.

    rclone cryptcheck remote:path encryptedremote:path

After it has run it will log the status of the encryptedremote:.

If you supply the --one-way flag, it will only check that files in source
match the files in destination, not the other way around. Meaning extra files in
destination that are not in the source will not trigger an error.


```
rclone cryptcheck remote:path cryptedremote:path [flags]
```

## Options

```
  -h, --help      help for cryptcheck
      --one-way   Check one way only, source files must exist on destination
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone cryptdecode

Cryptdecode returns unencrypted file names.

## Synopsis


rclone cryptdecode returns unencrypted file names when provided with
a list of encrypted file names. List limit is 10 items.

If you supply the --reverse flag, it will return encrypted file names.

use it like this

	rclone cryptdecode encryptedremote: encryptedfilename1 encryptedfilename2

	rclone cryptdecode --reverse encryptedremote: filename1 filename2


```
rclone cryptdecode encryptedremote: encryptedfilename [flags]
```

## Options

```
  -h, --help      help for cryptdecode
      --reverse   Reverse cryptdecode, encrypts filenames
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone deletefile

Remove a single file from remote.

## Synopsis


Remove a single file from remote.  Unlike `delete` it cannot be used to
remove a directory and it doesn't obey include/exclude filters - if the specified file exists,
it will always be removed.


```
rclone deletefile remote:path [flags]
```

## Options

```
  -h, --help   help for deletefile
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone genautocomplete

Output completion script for a given shell.

## Synopsis


Generates a shell completion script for rclone.
Run with --help to list the supported shells.


## Options

```
  -h, --help   help for genautocomplete
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.
* [rclone genautocomplete bash](https://rclone.org/commands/rclone_genautocomplete_bash/)	 - Output bash completion script for rclone.
* [rclone genautocomplete fish](https://rclone.org/commands/rclone_genautocomplete_fish/)	 - Output fish completion script for rclone.
* [rclone genautocomplete zsh](https://rclone.org/commands/rclone_genautocomplete_zsh/)	 - Output zsh completion script for rclone.

# rclone genautocomplete bash

Output bash completion script for rclone.

## Synopsis


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

## Options

```
  -h, --help   help for bash
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone genautocomplete](https://rclone.org/commands/rclone_genautocomplete/)	 - Output completion script for a given shell.

# rclone genautocomplete fish

Output fish completion script for rclone.

## Synopsis


Generates a fish autocompletion script for rclone.

This writes to /etc/fish/completions/rclone.fish by default so will
probably need to be run with sudo or as root, eg

    sudo rclone genautocomplete fish

Logout and login again to use the autocompletion scripts, or source
them directly

    . /etc/fish/completions/rclone.fish

If you supply a command line argument the script will be written
there.


```
rclone genautocomplete fish [output_file] [flags]
```

## Options

```
  -h, --help   help for fish
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone genautocomplete](https://rclone.org/commands/rclone_genautocomplete/)	 - Output completion script for a given shell.

# rclone genautocomplete zsh

Output zsh completion script for rclone.

## Synopsis


Generates a zsh autocompletion script for rclone.

This writes to /usr/share/zsh/vendor-completions/_rclone by default so will
probably need to be run with sudo or as root, eg

    sudo rclone genautocomplete zsh

Logout and login again to use the autocompletion scripts, or source
them directly

    autoload -U compinit && compinit

If you supply a command line argument the script will be written
there.


```
rclone genautocomplete zsh [output_file] [flags]
```

## Options

```
  -h, --help   help for zsh
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone genautocomplete](https://rclone.org/commands/rclone_genautocomplete/)	 - Output completion script for a given shell.

# rclone gendocs

Output markdown docs for rclone to the directory supplied.

## Synopsis


This produces markdown docs for the rclone commands to the directory
supplied.  These are in a format suitable for hugo to render into the
rclone.org website.

```
rclone gendocs output_directory [flags]
```

## Options

```
  -h, --help   help for gendocs
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone hashsum

Produces a hashsum file for all the objects in the path.

## Synopsis


Produces a hash file for all the objects in the path using the hash
named.  The output is in the same format as the standard
md5sum/sha1sum tool.

Run without a hash to see the list of supported hashes, eg

    $ rclone hashsum
    Supported hashes are:
      * MD5
      * SHA-1
      * DropboxHash
      * QuickXorHash

Then

    $ rclone hashsum MD5 remote:path


```
rclone hashsum <hash> remote:path [flags]
```

## Options

```
      --base64   Output base64 encoded hashsum
  -h, --help     help for hashsum
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone link

Generate public link to file/folder.

## Synopsis


rclone link will create or retrieve a public link to the given file or folder.

    rclone link remote:path/to/file
    rclone link remote:path/to/folder/

If successful, the last line of the output will contain the link. Exact
capabilities depend on the remote, but the link will always be created with
the least constraints – e.g. no expiry, no password protection, accessible
without account.


```
rclone link remote:path [flags]
```

## Options

```
  -h, --help   help for link
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone listremotes

List all the remotes in the config file.

## Synopsis


rclone listremotes lists all the available remotes from the config file.

When uses with the -l flag it lists the types too.


```
rclone listremotes [flags]
```

## Options

```
  -h, --help   help for listremotes
      --long   Show the type as well as names.
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone lsf

List directories and objects in remote:path formatted for parsing

## Synopsis


List the contents of the source path (directories and objects) to
standard output in a form which is easy to parse by scripts.  By
default this will just be the names of the objects and directories,
one per line.  The directories will have a / suffix.

Eg

    $ rclone lsf swift:bucket
    bevajer5jef
    canole
    diwogej7
    ferejej3gux/
    fubuwic

Use the --format option to control what gets listed.  By default this
is just the path, but you can use these parameters to control the
output:

    p - path
    s - size
    t - modification time
    h - hash
    i - ID of object
    o - Original ID of underlying object
    m - MimeType of object if known
    e - encrypted name
    T - tier of storage if known, eg "Hot" or "Cool"

So if you wanted the path, size and modification time, you would use
--format "pst", or maybe --format "tsp" to put the path last.

Eg

    $ rclone lsf  --format "tsp" swift:bucket
    2016-06-25 18:55:41;60295;bevajer5jef
    2016-06-25 18:55:43;90613;canole
    2016-06-25 18:55:43;94467;diwogej7
    2018-04-26 08:50:45;0;ferejej3gux/
    2016-06-25 18:55:40;37600;fubuwic

If you specify "h" in the format you will get the MD5 hash by default,
use the "--hash" flag to change which hash you want.  Note that this
can be returned as an empty string if it isn't available on the object
(and for directories), "ERROR" if there was an error reading it from
the object and "UNSUPPORTED" if that object does not support that hash
type.

For example to emulate the md5sum command you can use

    rclone lsf -R --hash MD5 --format hp --separator "  " --files-only .

Eg

    $ rclone lsf -R --hash MD5 --format hp --separator "  " --files-only swift:bucket 
    7908e352297f0f530b84a756f188baa3  bevajer5jef
    cd65ac234e6fea5925974a51cdd865cc  canole
    03b5341b4f234b9d984d03ad076bae91  diwogej7
    8fd37c3810dd660778137ac3a66cc06d  fubuwic
    99713e14a4c4ff553acaf1930fad985b  gixacuh7ku

(Though "rclone md5sum ." is an easier way of typing this.)

By default the separator is ";" this can be changed with the
--separator flag.  Note that separators aren't escaped in the path so
putting it last is a good strategy.

Eg

    $ rclone lsf  --separator "," --format "tshp" swift:bucket
    2016-06-25 18:55:41,60295,7908e352297f0f530b84a756f188baa3,bevajer5jef
    2016-06-25 18:55:43,90613,cd65ac234e6fea5925974a51cdd865cc,canole
    2016-06-25 18:55:43,94467,03b5341b4f234b9d984d03ad076bae91,diwogej7
    2018-04-26 08:52:53,0,,ferejej3gux/
    2016-06-25 18:55:40,37600,8fd37c3810dd660778137ac3a66cc06d,fubuwic

You can output in CSV standard format.  This will escape things in "
if they contain ,

Eg

    $ rclone lsf --csv --files-only --format ps remote:path
    test.log,22355
    test.sh,449
    "this file contains a comma, in the file name.txt",6

Note that the --absolute parameter is useful for making lists of files
to pass to an rclone copy with the --files-from-raw flag.

For example to find all the files modified within one day and copy
those only (without traversing the whole directory structure):

    rclone lsf --absolute --files-only --max-age 1d /path/to/local > new_files
    rclone copy --files-from-raw new_files /path/to/local remote:path


Any of the filtering options can be applied to this command.

There are several related list commands

  * `ls` to list size and path of objects only
  * `lsl` to list modification time, size and path of objects only
  * `lsd` to list directories only
  * `lsf` to list objects and directories in easy to parse format
  * `lsjson` to list objects and directories in JSON format

`ls`,`lsl`,`lsd` are designed to be human readable.
`lsf` is designed to be human and machine readable.
`lsjson` is designed to be machine readable.

Note that `ls` and `lsl` recurse by default - use "--max-depth 1" to stop the recursion.

The other list commands `lsd`,`lsf`,`lsjson` do not recurse by default - use "-R" to make them recurse.

Listing a non existent directory will produce an error except for
remotes which can't have empty directories (eg s3, swift, gcs, etc -
the bucket based remotes).


```
rclone lsf remote:path [flags]
```

## Options

```
      --absolute           Put a leading / in front of path names.
      --csv                Output in CSV format.
  -d, --dir-slash          Append a slash to directory names. (default true)
      --dirs-only          Only list directories.
      --files-only         Only list files.
  -F, --format string      Output format - see  help for details (default "p")
      --hash h             Use this hash when h is used in the format MD5|SHA-1|DropboxHash (default "MD5")
  -h, --help               help for lsf
  -R, --recursive          Recurse into the listing.
  -s, --separator string   Separator for the items in the format. (default ";")
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone lsjson

List directories and objects in the path in JSON format.

## Synopsis

List directories and objects in the path in JSON format.

The output is an array of Items, where each Item looks like this

   {
      "Hashes" : {
         "SHA-1" : "f572d396fae9206628714fb2ce00f72e94f2258f",
         "MD5" : "b1946ac92492d2347c6235b4d2611184",
         "DropboxHash" : "ecb65bb98f9d905b70458986c39fcbad7715e5f2fcc3b1f07767d7c83e2438cc"
      },
      "ID": "y2djkhiujf83u33",
      "OrigID": "UYOJVTUW00Q1RzTDA",
      "IsBucket" : false,
      "IsDir" : false,
      "MimeType" : "application/octet-stream",
      "ModTime" : "2017-05-31T16:15:57.034468261+01:00",
      "Name" : "file.txt",
      "Encrypted" : "v0qpsdq8anpci8n929v3uu9338",
      "EncryptedPath" : "kja9098349023498/v0qpsdq8anpci8n929v3uu9338",
      "Path" : "full/path/goes/here/file.txt",
      "Size" : 6,
      "Tier" : "hot",
   }

If --hash is not specified the Hashes property won't be emitted. The
types of hash can be specified with the --hash-type parameter (which
may be repeated). If --hash-type is set then it implies --hash.

If --no-modtime is specified then ModTime will be blank. This can
speed things up on remotes where reading the ModTime takes an extra
request (eg s3, swift).

If --no-mimetype is specified then MimeType will be blank. This can
speed things up on remotes where reading the MimeType takes an extra
request (eg s3, swift).

If --encrypted is not specified the Encrypted won't be emitted.

If --dirs-only is not specified files in addition to directories are
returned

If --files-only is not specified directories in addition to the files
will be returned.

The Path field will only show folders below the remote path being listed.
If "remote:path" contains the file "subfolder/file.txt", the Path for "file.txt"
will be "subfolder/file.txt", not "remote:path/subfolder/file.txt".
When used without --recursive the Path will always be the same as Name.

If the directory is a bucket in a bucket based backend, then
"IsBucket" will be set to true. This key won't be present unless it is
"true".

The time is in RFC3339 format with up to nanosecond precision.  The
number of decimal digits in the seconds will depend on the precision
that the remote can hold the times, so if times are accurate to the
nearest millisecond (eg Google Drive) then 3 digits will always be
shown ("2017-05-31T16:15:57.034+01:00") whereas if the times are
accurate to the nearest second (Dropbox, Box, WebDav etc) no digits
will be shown ("2017-05-31T16:15:57+01:00").

The whole output can be processed as a JSON blob, or alternatively it
can be processed line by line as each item is written one to a line.

Any of the filtering options can be applied to this command.

There are several related list commands

  * `ls` to list size and path of objects only
  * `lsl` to list modification time, size and path of objects only
  * `lsd` to list directories only
  * `lsf` to list objects and directories in easy to parse format
  * `lsjson` to list objects and directories in JSON format

`ls`,`lsl`,`lsd` are designed to be human readable.
`lsf` is designed to be human and machine readable.
`lsjson` is designed to be machine readable.

Note that `ls` and `lsl` recurse by default - use "--max-depth 1" to stop the recursion.

The other list commands `lsd`,`lsf`,`lsjson` do not recurse by default - use "-R" to make them recurse.

Listing a non existent directory will produce an error except for
remotes which can't have empty directories (eg s3, swift, gcs, etc -
the bucket based remotes).


```
rclone lsjson remote:path [flags]
```

## Options

```
      --dirs-only               Show only directories in the listing.
  -M, --encrypted               Show the encrypted names.
      --files-only              Show only files in the listing.
      --hash                    Include hashes in the output (may take longer).
      --hash-type stringArray   Show only this hash type (may be repeated).
  -h, --help                    help for lsjson
      --no-mimetype             Don't read the mime type (can speed things up).
      --no-modtime              Don't read the modification time (can speed things up).
      --original                Show the ID of the underlying Object.
  -R, --recursive               Recurse into the listing.
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone mount

Mount the remote as file system on a mountpoint.

## Synopsis


rclone mount allows Linux, FreeBSD, macOS and Windows to
mount any of Rclone's cloud storage systems as a file system with
FUSE.

First set up your remote using `rclone config`.  Check it works with `rclone ls` etc.

You can either run mount in foreground mode or background (daemon) mode. Mount runs in
foreground mode by default, use the --daemon flag to specify background mode mode.
Background mode is only supported on Linux and OSX, you can only run mount in
foreground mode on Windows.

On Linux/macOS/FreeBSD Start the mount like this where `/path/to/local/mount`
is an **empty** **existing** directory.

    rclone mount remote:path/to/files /path/to/local/mount

Or on Windows like this where `X:` is an unused drive letter
or use a path to **non-existent** directory.

    rclone mount remote:path/to/files X:
    rclone mount remote:path/to/files C:\path\to\nonexistent\directory

When running in background mode the user will have to stop the mount manually (specified below).

When the program ends while in foreground mode, either via Ctrl+C or receiving
a SIGINT or SIGTERM signal, the mount is automatically stopped.

The umount operation can fail, for example when the mountpoint is busy.
When that happens, it is the user's responsibility to stop the mount manually.

Stopping the mount manually:

    # Linux
    fusermount -u /path/to/local/mount
    # OS X
    umount /path/to/local/mount

## Installing on Windows

To run rclone mount on Windows, you will need to
download and install [WinFsp](http://www.secfs.net/winfsp/).

[WinFsp](https://github.com/billziss-gh/winfsp) is an open source
Windows File System Proxy which makes it easy to write user space file
systems for Windows.  It provides a FUSE emulation layer which rclone
uses combination with
[cgofuse](https://github.com/billziss-gh/cgofuse).  Both of these
packages are by Bill Zissimopoulos who was very helpful during the
implementation of rclone mount for Windows.

### Windows caveats

Note that drives created as Administrator are not visible by other
accounts (including the account that was elevated as
Administrator). So if you start a Windows drive from an Administrative
Command Prompt and then try to access the same drive from Explorer
(which does not run as Administrator), you will not be able to see the
new drive.

The easiest way around this is to start the drive from a normal
command prompt. It is also possible to start a drive from the SYSTEM
account (using [the WinFsp.Launcher
infrastructure](https://github.com/billziss-gh/winfsp/wiki/WinFsp-Service-Architecture))
which creates drives accessible for everyone on the system or
alternatively using [the nssm service manager](https://nssm.cc/usage).

### Mount as a network drive

By default, rclone will mount the remote as a normal drive. However,
you can also mount it as a **Network Drive** (or **Network Share**, as
mentioned in some places)

Unlike other systems, Windows provides a different filesystem type for
network drives.  Windows and other programs treat the network drives
and fixed/removable drives differently: In network drives, many I/O
operations are optimized, as the high latency and low reliability
(compared to a normal drive) of a network is expected.

Although many people prefer network shares to be mounted as normal
system drives, this might cause some issues, such as programs not
working as expected or freezes and errors while operating with the
mounted remote in Windows Explorer. If you experience any of those,
consider mounting rclone remotes as network shares, as Windows expects
normal drives to be fast and reliable, while cloud storage is far from
that.  See also [Limitations](#limitations) section below for more
info

Add "--fuse-flag --VolumePrefix=\server\share" to your "mount"
command, **replacing "share" with any other name of your choice if you
are mounting more than one remote**. Otherwise, the mountpoints will
conflict and your mounted filesystems will overlap.

[Read more about drive mapping](https://en.wikipedia.org/wiki/Drive_mapping)

## Limitations

Without the use of "--vfs-cache-mode" this can only write files
sequentially, it can only seek when reading.  This means that many
applications won't work with their files on an rclone mount without
"--vfs-cache-mode writes" or "--vfs-cache-mode full".  See the [File
Caching](#file-caching) section for more info.

The bucket based remotes (eg Swift, S3, Google Compute Storage, B2,
Hubic) do not support the concept of empty directories, so empty
directories will have a tendency to disappear once they fall out of
the directory cache.

Only supported on Linux, FreeBSD, OS X and Windows at the moment.

## rclone mount vs rclone sync/copy

File systems expect things to be 100% reliable, whereas cloud storage
systems are a long way from 100% reliable. The rclone sync/copy
commands cope with this with lots of retries.  However rclone mount
can't use retries in the same way without making local copies of the
uploads. Look at the [file caching](#file-caching)
for solutions to make mount more reliable.

## Attribute caching

You can use the flag --attr-timeout to set the time the kernel caches
the attributes (size, modification time etc) for directory entries.

The default is "1s" which caches files just long enough to avoid
too many callbacks to rclone from the kernel.

In theory 0s should be the correct value for filesystems which can
change outside the control of the kernel. However this causes quite a
few problems such as
[rclone using too much memory](https://github.com/rclone/rclone/issues/2157),
[rclone not serving files to samba](https://forum.rclone.org/t/rclone-1-39-vs-1-40-mount-issue/5112)
and [excessive time listing directories](https://github.com/rclone/rclone/issues/2095#issuecomment-371141147).

The kernel can cache the info about a file for the time given by
"--attr-timeout". You may see corruption if the remote file changes
length during this window.  It will show up as either a truncated file
or a file with garbage on the end.  With "--attr-timeout 1s" this is
very unlikely but not impossible.  The higher you set "--attr-timeout"
the more likely it is.  The default setting of "1s" is the lowest
setting which mitigates the problems above.

If you set it higher ('10s' or '1m' say) then the kernel will call
back to rclone less often making it more efficient, however there is
more chance of the corruption issue above.

If files don't change on the remote outside of the control of rclone
then there is no chance of corruption.

This is the same as setting the attr_timeout option in mount.fuse.

## Filters

Note that all the rclone filters can be used to select a subset of the
files to be visible in the mount.

## systemd

When running rclone mount as a systemd service, it is possible
to use Type=notify. In this case the service will enter the started state
after the mountpoint has been successfully set up.
Units having the rclone mount service specified as a requirement
will see all files and folders immediately in this mode.

## chunked reading ###

--vfs-read-chunk-size will enable reading the source objects in parts.
This can reduce the used download quota for some remotes by requesting only chunks
from the remote that are actually read at the cost of an increased number of requests.

When --vfs-read-chunk-size-limit is also specified and greater than --vfs-read-chunk-size,
the chunk size for each open file will get doubled for each chunk read, until the
specified value is reached. A value of -1 will disable the limit and the chunk size will
grow indefinitely.

With --vfs-read-chunk-size 100M and --vfs-read-chunk-size-limit 0 the following
parts will be downloaded: 0-100M, 100M-200M, 200M-300M, 300M-400M and so on.
When --vfs-read-chunk-size-limit 500M is specified, the result would be
0-100M, 100M-300M, 300M-700M, 700M-1200M, 1200M-1700M and so on.

Chunked reading will only work with --vfs-cache-mode < full, as the file will always
be copied to the vfs cache before opening with --vfs-cache-mode full.

## Directory Cache

Using the `--dir-cache-time` flag, you can set how long a
directory should be considered up to date and not refreshed from the
backend. Changes made locally in the mount may appear immediately or
invalidate the cache. However, changes done on the remote will only
be picked up once the cache expires if the backend configured does not
support polling for changes. If the backend supports polling, changes
will be picked up on within the polling interval.

Alternatively, you can send a `SIGHUP` signal to rclone for
it to flush all directory caches, regardless of how old they are.
Assuming only one rclone instance is running, you can reset the cache
like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a [remote control](/rc) then you can use
rclone rc to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

## File Buffering

The `--buffer-size` flag determines the amount of memory,
that will be used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of
data in memory at all times. The buffered data is bound to one file
descriptor and won't be shared between multiple open file descriptors
of the same file.

This flag is a upper limit for the used memory per file descriptor.
The buffer will only use memory for data that is downloaded but not
not yet read. If the buffer is empty, only a small amount of memory
will be used.
The maximum memory used by rclone for buffering can be up to
`--buffer-size * open files`.

## File Caching

These flags control the VFS file caching options.  The VFS layer is
used by rclone mount to make a cloud storage system work more like a
normal file system.

You'll need to enable VFS caching if you want, for example, to read
and write simultaneously to a file.  See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

If run with `-vv` rclone will print the location of the file cache.  The
files are stored in the user cache file area which is OS dependent but
can be controlled with `--cache-dir` or setting the appropriate
environment variable.

The cache has 4 different modes selected by `--vfs-cache-mode`.
The higher the cache mode the more compatible rclone becomes at the
cost of using disk space.

Note that files are written back to the remote only when they are
closed so if rclone is quit or dies with open files then these won't
get written back to the remote.  However they will still be in the on
disk cache.

If using --vfs-cache-max-size note that the cache may exceed this size
for two reasons.  Firstly because it is only checked every
--vfs-cache-poll-interval.  Secondly because open files cannot be
evicted from the cache.

### --vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

  * Files can't be opened for both read AND write
  * Files opened for write can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files open for read with O_TRUNC will be opened write only
  * Files open for write only will behave as if O_TRUNC was supplied
  * Open modes O_APPEND, O_TRUNC are ignored
  * If an upload fails it can't be retried

### --vfs-cache-mode minimal

This is very similar to "off" except that files opened for read AND
write will be buffered to disks.  This means that files opened for
write will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

  * Files opened for write only can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files opened for write only will ignore O_APPEND, O_TRUNC
  * If an upload fails it can't be retried

### --vfs-cache-mode writes

In this mode files opened for read only are still read directly from
the remote, write only and read/write files are buffered to disk
first.

This mode should support all normal file system operations.

If an upload fails it will be retried up to --low-level-retries times.

### --vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk.  When
a file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at
the cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk,
it will be kept on the disk after it is written to the remote.  It
will be purged on a schedule according to `--vfs-cache-max-age`.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
--low-level-retries times.

## Case Sensitivity

Linux file systems are case-sensitive: two files can differ only
by case, and the exact case must be used when opening a file.

Windows is not like most other operating systems supported by rclone.
File systems in modern Windows are case-insensitive but case-preserving:
although existing files can be opened using any case, the exact case used
to create the file is preserved and available for programs to query.
It is not allowed for two files in the same directory to differ only by case.

Usually file systems on macOS are case-insensitive. It is possible to make macOS
file systems case-sensitive but that is not the default

The "--vfs-case-insensitive" mount flag controls how rclone handles these
two cases. If its value is "false", rclone passes file names to the mounted
file system as is. If the flag is "true" (or appears without a value on
command line), rclone may perform a "fixup" as explained below.

The user may specify a file name to open/delete/rename/etc with a case
different than what is stored on mounted file system. If an argument refers
to an existing file with exactly the same name, then the case of the existing
file on the disk will be used. However, if a file name with exactly the same
name is not found but a name differing only by case exists, rclone will
transparently fixup the name. This fixup happens only when an existing file
is requested. Case sensitivity of file names created anew by rclone is
controlled by an underlying mounted file system.

Note that case sensitivity of the operating system running rclone (the target)
may differ from case sensitivity of a file system mounted by rclone (the source).
The flag controls whether "fixup" is performed to satisfy the target.

If the flag is not provided on command line, then its default value depends
on the operating system where rclone runs: "true" on Windows and macOS, "false"
otherwise. If the flag is provided without a value, then it is "true".


```
rclone mount remote:path /path/to/mountpoint [flags]
```

## Options

```
      --allow-non-empty                        Allow mounting over a non-empty directory (not Windows).
      --allow-other                            Allow access to other users.
      --allow-root                             Allow access to root user.
      --async-read                             Use asynchronous reads. (default true)
      --attr-timeout duration                  Time for which file/directory attributes are cached. (default 1s)
      --daemon                                 Run mount as a daemon (background mode).
      --daemon-timeout duration                Time limit for rclone to respond to kernel (not supported by all OSes).
      --debug-fuse                             Debug the FUSE internals - needs -v.
      --default-permissions                    Makes kernel enforce access control based on the file mode.
      --dir-cache-time duration                Time to cache directory entries for. (default 5m0s)
      --dir-perms FileMode                     Directory permissions (default 0777)
      --file-perms FileMode                    File permissions (default 0666)
      --fuse-flag stringArray                  Flags or arguments to be passed direct to libfuse/WinFsp. Repeat if required.
      --gid uint32                             Override the gid field set by the filesystem. (default 1000)
  -h, --help                                   help for mount
      --max-read-ahead SizeSuffix              The number of bytes that can be prefetched for sequential reads. (default 128k)
      --no-checksum                            Don't compare checksums on up/download.
      --no-modtime                             Don't read/write the modification time (can speed things up).
      --no-seek                                Don't allow seeking in files.
  -o, --option stringArray                     Option for libfuse/WinFsp. Repeat if required.
      --poll-interval duration                 Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable. (default 1m0s)
      --read-only                              Mount read-only.
      --uid uint32                             Override the uid field set by the filesystem. (default 1000)
      --umask int                              Override the permission bits set by the filesystem.
      --vfs-cache-max-age duration             Max age of objects in the cache. (default 1h0m0s)
      --vfs-cache-max-size SizeSuffix          Max total size of objects in the cache. (default off)
      --vfs-cache-mode CacheMode               Cache mode off|minimal|writes|full (default off)
      --vfs-cache-poll-interval duration       Interval to poll the cache for stale objects. (default 1m0s)
      --vfs-case-insensitive                   If a file name not found, find a case insensitive match.
      --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
      --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)
      --vfs-read-wait duration                 Time to wait for in-sequence read before seeking. (default 20ms)
      --vfs-write-wait duration                Time to wait for in-sequence write before giving error. (default 1s)
      --volname string                         Set the volume name (not supported by all OSes).
      --write-back-cache                       Makes kernel buffer writes before sending them to rclone. Without this, writethrough caching is used.
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone moveto

Move file or directory from source to dest.

## Synopsis


If source:path is a file or directory then it moves it to a file or
directory named dest:path.

This can be used to rename files or upload single files to other than
their existing name.  If the source is a directory then it acts exactly
like the move command.

So

    rclone moveto src dst

where src and dst are rclone paths, either remote:path or
/path/to/local or C:\windows\path\if\on\windows.

This will:

    if src is file
        move it to dst, overwriting an existing file if it exists
    if src is directory
        move it to dst, overwriting existing files if they exist
        see move command for full details

This doesn't transfer unchanged files, testing by size and
modification time or MD5SUM.  src will be deleted on successful
transfer.

**Important**: Since this can cause data loss, test first with the
--dry-run flag.

**Note**: Use the `-P`/`--progress` flag to view real-time transfer statistics.


```
rclone moveto source:path dest:path [flags]
```

## Options

```
  -h, --help   help for moveto
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone ncdu

Explore a remote with a text based user interface.

## Synopsis


This displays a text based user interface allowing the navigation of a
remote. It is most useful for answering the question - "What is using
all my disk space?".



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
     y copy current path to clipbard
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

## Options

```
  -h, --help   help for ncdu
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone obscure

Obscure password for use in the rclone config file

## Synopsis

In the rclone config file, human readable passwords are
obscured. Obscuring them is done by encrypting them and writing them
out in base64. This is **not** a secure way of encrypting these
passwords as rclone can decrypt them - it is to prevent "eyedropping"
- namely someone seeing a password in the rclone config file by
accident.

Many equally important things (like access tokens) are not obscured in
the config file. However it is very hard to shoulder surf a 64
character hex token.

If you want to encrypt the config file then please use config file
encryption - see [rclone config](https://rclone.org/commands/rclone_config/) for more
info.

```
rclone obscure password [flags]
```

## Options

```
  -h, --help   help for obscure
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone rc

Run a command against a running rclone.

## Synopsis



This runs a command against a running rclone.  Use the --url flag to
specify an non default URL to connect on.  This can be either a
":port" which is taken to mean "http://localhost:port" or a
"host:port" which is taken to mean "http://host:port"

A username and password can be passed in with --user and --pass.

Note that --rc-addr, --rc-user, --rc-pass will be read also for --url,
--user, --pass.

Arguments should be passed in as parameter=value.

The result will be returned as a JSON object by default.

The --json parameter can be used to pass in a JSON blob as an input
instead of key=value arguments.  This is the only way of passing in
more complicated values.

The -o/--opt option can be used to set a key "opt" with key, value
options in the form "-o key=value" or "-o key". It can be repeated as
many times as required. This is useful for rc commands which take the
"opt" parameter which by convention is a dictionary of strings.

    -o key=value -o key2

Will place this in the "opt" value

    {"key":"value", "key2","")


The -a/--arg option can be used to set strings in the "arg" value. It
can be repeated as many times as required. This is useful for rc
commands which take the "arg" parameter which by convention is a list
of strings.

    -a value -a value2

Will place this in the "arg" value

    ["value", "value2"]

Use --loopback to connect to the rclone instance running "rclone rc".
This is very useful for testing commands without having to run an
rclone rc server, eg:

    rclone rc --loopback operations/about fs=/

Use "rclone rc" to see a list of all possible commands.

```
rclone rc commands parameter [flags]
```

## Options

```
  -a, --arg stringArray   Argument placed in the "arg" array.
  -h, --help              help for rc
      --json string       Input JSON - use instead of key=value args.
      --loopback          If set connect to this rclone instance not via HTTP.
      --no-output         If set don't output the JSON result.
  -o, --opt stringArray   Option in the form name=value or name placed in the "opt" array.
      --pass string       Password to use to connect to rclone remote control.
      --url string        URL to connect to rclone remote control. (default "http://localhost:5572/")
      --user string       Username to use to rclone remote control.
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone rcat

Copies standard input to file on remote.

## Synopsis


rclone rcat reads from standard input (stdin) and copies it to a
single remote file.

    echo "hello world" | rclone rcat remote:path/to/file
    ffmpeg - | rclone rcat remote:path/to/file

If the remote file already exists, it will be overwritten.

rcat will try to upload small files in a single request, which is
usually more efficient than the streaming/chunked upload endpoints,
which use multiple requests. Exact behaviour depends on the remote.
What is considered a small file may be set through
`--streaming-upload-cutoff`. Uploading only starts after
the cutoff is reached or if the file ends before that. The data
must fit into RAM. The cutoff needs to be small enough to adhere
the limits of your remote, please see there. Generally speaking,
setting this cutoff too high will decrease your performance.

Note that the upload can also not be retried because the data is
not kept around until the upload succeeds. If you need to transfer
a lot of data, you're better off caching locally and then
`rclone move` it to the destination.

```
rclone rcat remote:path [flags]
```

## Options

```
  -h, --help   help for rcat
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone rcd

Run rclone listening to remote control commands only.

## Synopsis


This runs rclone so that it only listens to remote control commands.

This is useful if you are controlling rclone via the rc API.

If you pass in a path to a directory, rclone will serve that directory
for GET requests on the URL passed in.  It will also open the URL in
the browser when rclone is run.

See the [rc documentation](https://rclone.org/rc/) for more info on the rc flags.


```
rclone rcd <path to files to serve>* [flags]
```

## Options

```
  -h, --help   help for rcd
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone rmdirs

Remove empty directories under the path.

## Synopsis

This removes any empty directories (or directories that only contain
empty directories) under the path that it finds, including the path if
it has nothing in.

If you supply the --leave-root flag, it will not remove the root directory.

This is useful for tidying up remotes that rclone has left a lot of
empty directories in.



```
rclone rmdirs remote:path [flags]
```

## Options

```
  -h, --help         help for rmdirs
      --leave-root   Do not remove root directory if empty
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone serve

Serve a remote over a protocol.

## Synopsis

rclone serve is used to serve a remote over a given protocol. This
command requires the use of a subcommand to specify the protocol, eg

    rclone serve http remote:

Each subcommand has its own options which you can see in their help.


```
rclone serve <protocol> [opts] <remote> [flags]
```

## Options

```
  -h, --help   help for serve
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.
* [rclone serve dlna](https://rclone.org/commands/rclone_serve_dlna/)	 - Serve remote:path over DLNA
* [rclone serve ftp](https://rclone.org/commands/rclone_serve_ftp/)	 - Serve remote:path over FTP.
* [rclone serve http](https://rclone.org/commands/rclone_serve_http/)	 - Serve the remote over HTTP.
* [rclone serve restic](https://rclone.org/commands/rclone_serve_restic/)	 - Serve the remote for restic's REST API.
* [rclone serve sftp](https://rclone.org/commands/rclone_serve_sftp/)	 - Serve the remote over SFTP.
* [rclone serve webdav](https://rclone.org/commands/rclone_serve_webdav/)	 - Serve remote:path over webdav.

# rclone serve dlna

Serve remote:path over DLNA

## Synopsis

rclone serve dlna is a DLNA media server for media stored in an rclone remote. Many
devices, such as the Xbox and PlayStation, can automatically discover this server in the LAN
and play audio/video from it. VLC is also supported. Service discovery uses UDP multicast
packets (SSDP) and will thus only work on LANs.

Rclone will list all files present in the remote, without filtering based on media formats or
file extensions. Additionally, there is no media transcoding support. This means that some
players might show files that they are not able to play back correctly.


## Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.

Use --name to choose the friendly server name, which is by
default "rclone (hostname)".

Use --log-trace in conjunction with -vv to enable additional debug
logging of all UPNP traffic.

## Directory Cache

Using the `--dir-cache-time` flag, you can set how long a
directory should be considered up to date and not refreshed from the
backend. Changes made locally in the mount may appear immediately or
invalidate the cache. However, changes done on the remote will only
be picked up once the cache expires if the backend configured does not
support polling for changes. If the backend supports polling, changes
will be picked up on within the polling interval.

Alternatively, you can send a `SIGHUP` signal to rclone for
it to flush all directory caches, regardless of how old they are.
Assuming only one rclone instance is running, you can reset the cache
like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a [remote control](/rc) then you can use
rclone rc to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

## File Buffering

The `--buffer-size` flag determines the amount of memory,
that will be used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of
data in memory at all times. The buffered data is bound to one file
descriptor and won't be shared between multiple open file descriptors
of the same file.

This flag is a upper limit for the used memory per file descriptor.
The buffer will only use memory for data that is downloaded but not
not yet read. If the buffer is empty, only a small amount of memory
will be used.
The maximum memory used by rclone for buffering can be up to
`--buffer-size * open files`.

## File Caching

These flags control the VFS file caching options.  The VFS layer is
used by rclone mount to make a cloud storage system work more like a
normal file system.

You'll need to enable VFS caching if you want, for example, to read
and write simultaneously to a file.  See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

If run with `-vv` rclone will print the location of the file cache.  The
files are stored in the user cache file area which is OS dependent but
can be controlled with `--cache-dir` or setting the appropriate
environment variable.

The cache has 4 different modes selected by `--vfs-cache-mode`.
The higher the cache mode the more compatible rclone becomes at the
cost of using disk space.

Note that files are written back to the remote only when they are
closed so if rclone is quit or dies with open files then these won't
get written back to the remote.  However they will still be in the on
disk cache.

If using --vfs-cache-max-size note that the cache may exceed this size
for two reasons.  Firstly because it is only checked every
--vfs-cache-poll-interval.  Secondly because open files cannot be
evicted from the cache.

### --vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

  * Files can't be opened for both read AND write
  * Files opened for write can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files open for read with O_TRUNC will be opened write only
  * Files open for write only will behave as if O_TRUNC was supplied
  * Open modes O_APPEND, O_TRUNC are ignored
  * If an upload fails it can't be retried

### --vfs-cache-mode minimal

This is very similar to "off" except that files opened for read AND
write will be buffered to disks.  This means that files opened for
write will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

  * Files opened for write only can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files opened for write only will ignore O_APPEND, O_TRUNC
  * If an upload fails it can't be retried

### --vfs-cache-mode writes

In this mode files opened for read only are still read directly from
the remote, write only and read/write files are buffered to disk
first.

This mode should support all normal file system operations.

If an upload fails it will be retried up to --low-level-retries times.

### --vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk.  When
a file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at
the cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk,
it will be kept on the disk after it is written to the remote.  It
will be purged on a schedule according to `--vfs-cache-max-age`.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
--low-level-retries times.

## Case Sensitivity

Linux file systems are case-sensitive: two files can differ only
by case, and the exact case must be used when opening a file.

Windows is not like most other operating systems supported by rclone.
File systems in modern Windows are case-insensitive but case-preserving:
although existing files can be opened using any case, the exact case used
to create the file is preserved and available for programs to query.
It is not allowed for two files in the same directory to differ only by case.

Usually file systems on macOS are case-insensitive. It is possible to make macOS
file systems case-sensitive but that is not the default

The "--vfs-case-insensitive" mount flag controls how rclone handles these
two cases. If its value is "false", rclone passes file names to the mounted
file system as is. If the flag is "true" (or appears without a value on
command line), rclone may perform a "fixup" as explained below.

The user may specify a file name to open/delete/rename/etc with a case
different than what is stored on mounted file system. If an argument refers
to an existing file with exactly the same name, then the case of the existing
file on the disk will be used. However, if a file name with exactly the same
name is not found but a name differing only by case exists, rclone will
transparently fixup the name. This fixup happens only when an existing file
is requested. Case sensitivity of file names created anew by rclone is
controlled by an underlying mounted file system.

Note that case sensitivity of the operating system running rclone (the target)
may differ from case sensitivity of a file system mounted by rclone (the source).
The flag controls whether "fixup" is performed to satisfy the target.

If the flag is not provided on command line, then its default value depends
on the operating system where rclone runs: "true" on Windows and macOS, "false"
otherwise. If the flag is provided without a value, then it is "true".


```
rclone serve dlna remote:path [flags]
```

## Options

```
      --addr string                            ip:port or :port to bind the DLNA http server to. (default ":7879")
      --dir-cache-time duration                Time to cache directory entries for. (default 5m0s)
      --dir-perms FileMode                     Directory permissions (default 0777)
      --file-perms FileMode                    File permissions (default 0666)
      --gid uint32                             Override the gid field set by the filesystem. (default 1000)
  -h, --help                                   help for dlna
      --log-trace                              enable trace logging of SOAP traffic
      --name string                            name of DLNA server
      --no-checksum                            Don't compare checksums on up/download.
      --no-modtime                             Don't read/write the modification time (can speed things up).
      --no-seek                                Don't allow seeking in files.
      --poll-interval duration                 Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable. (default 1m0s)
      --read-only                              Mount read-only.
      --uid uint32                             Override the uid field set by the filesystem. (default 1000)
      --umask int                              Override the permission bits set by the filesystem. (default 2)
      --vfs-cache-max-age duration             Max age of objects in the cache. (default 1h0m0s)
      --vfs-cache-max-size SizeSuffix          Max total size of objects in the cache. (default off)
      --vfs-cache-mode CacheMode               Cache mode off|minimal|writes|full (default off)
      --vfs-cache-poll-interval duration       Interval to poll the cache for stale objects. (default 1m0s)
      --vfs-case-insensitive                   If a file name not found, find a case insensitive match.
      --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
      --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)
      --vfs-read-wait duration                 Time to wait for in-sequence read before seeking. (default 20ms)
      --vfs-write-wait duration                Time to wait for in-sequence write before giving error. (default 1s)
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone serve](https://rclone.org/commands/rclone_serve/)	 - Serve a remote over a protocol.

# rclone serve ftp

Serve remote:path over FTP.

## Synopsis


rclone serve ftp implements a basic ftp server to serve the
remote over FTP protocol. This can be viewed with a ftp client
or you can make a remote of type ftp to read and write it.

## Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.  You can use port
:0 to let the OS choose an available port.

If you set --addr to listen on a public or LAN accessible IP address
then using Authentication is advised - see the next section for info.

### Authentication

By default this will serve files without needing a login.

You can set a single username and password with the --user and --pass flags.

## Directory Cache

Using the `--dir-cache-time` flag, you can set how long a
directory should be considered up to date and not refreshed from the
backend. Changes made locally in the mount may appear immediately or
invalidate the cache. However, changes done on the remote will only
be picked up once the cache expires if the backend configured does not
support polling for changes. If the backend supports polling, changes
will be picked up on within the polling interval.

Alternatively, you can send a `SIGHUP` signal to rclone for
it to flush all directory caches, regardless of how old they are.
Assuming only one rclone instance is running, you can reset the cache
like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a [remote control](/rc) then you can use
rclone rc to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

## File Buffering

The `--buffer-size` flag determines the amount of memory,
that will be used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of
data in memory at all times. The buffered data is bound to one file
descriptor and won't be shared between multiple open file descriptors
of the same file.

This flag is a upper limit for the used memory per file descriptor.
The buffer will only use memory for data that is downloaded but not
not yet read. If the buffer is empty, only a small amount of memory
will be used.
The maximum memory used by rclone for buffering can be up to
`--buffer-size * open files`.

## File Caching

These flags control the VFS file caching options.  The VFS layer is
used by rclone mount to make a cloud storage system work more like a
normal file system.

You'll need to enable VFS caching if you want, for example, to read
and write simultaneously to a file.  See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

If run with `-vv` rclone will print the location of the file cache.  The
files are stored in the user cache file area which is OS dependent but
can be controlled with `--cache-dir` or setting the appropriate
environment variable.

The cache has 4 different modes selected by `--vfs-cache-mode`.
The higher the cache mode the more compatible rclone becomes at the
cost of using disk space.

Note that files are written back to the remote only when they are
closed so if rclone is quit or dies with open files then these won't
get written back to the remote.  However they will still be in the on
disk cache.

If using --vfs-cache-max-size note that the cache may exceed this size
for two reasons.  Firstly because it is only checked every
--vfs-cache-poll-interval.  Secondly because open files cannot be
evicted from the cache.

### --vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

  * Files can't be opened for both read AND write
  * Files opened for write can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files open for read with O_TRUNC will be opened write only
  * Files open for write only will behave as if O_TRUNC was supplied
  * Open modes O_APPEND, O_TRUNC are ignored
  * If an upload fails it can't be retried

### --vfs-cache-mode minimal

This is very similar to "off" except that files opened for read AND
write will be buffered to disks.  This means that files opened for
write will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

  * Files opened for write only can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files opened for write only will ignore O_APPEND, O_TRUNC
  * If an upload fails it can't be retried

### --vfs-cache-mode writes

In this mode files opened for read only are still read directly from
the remote, write only and read/write files are buffered to disk
first.

This mode should support all normal file system operations.

If an upload fails it will be retried up to --low-level-retries times.

### --vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk.  When
a file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at
the cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk,
it will be kept on the disk after it is written to the remote.  It
will be purged on a schedule according to `--vfs-cache-max-age`.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
--low-level-retries times.

## Case Sensitivity

Linux file systems are case-sensitive: two files can differ only
by case, and the exact case must be used when opening a file.

Windows is not like most other operating systems supported by rclone.
File systems in modern Windows are case-insensitive but case-preserving:
although existing files can be opened using any case, the exact case used
to create the file is preserved and available for programs to query.
It is not allowed for two files in the same directory to differ only by case.

Usually file systems on macOS are case-insensitive. It is possible to make macOS
file systems case-sensitive but that is not the default

The "--vfs-case-insensitive" mount flag controls how rclone handles these
two cases. If its value is "false", rclone passes file names to the mounted
file system as is. If the flag is "true" (or appears without a value on
command line), rclone may perform a "fixup" as explained below.

The user may specify a file name to open/delete/rename/etc with a case
different than what is stored on mounted file system. If an argument refers
to an existing file with exactly the same name, then the case of the existing
file on the disk will be used. However, if a file name with exactly the same
name is not found but a name differing only by case exists, rclone will
transparently fixup the name. This fixup happens only when an existing file
is requested. Case sensitivity of file names created anew by rclone is
controlled by an underlying mounted file system.

Note that case sensitivity of the operating system running rclone (the target)
may differ from case sensitivity of a file system mounted by rclone (the source).
The flag controls whether "fixup" is performed to satisfy the target.

If the flag is not provided on command line, then its default value depends
on the operating system where rclone runs: "true" on Windows and macOS, "false"
otherwise. If the flag is provided without a value, then it is "true".

## Auth Proxy

If you supply the parameter `--auth-proxy /path/to/program` then
rclone will use that program to generate backends on the fly which
then are used to authenticate incoming requests.  This uses a simple
JSON based protocl with input on STDIN and output on STDOUT.

**PLEASE NOTE:** `--auth-proxy` and `--authorized-keys` cannot be used
together, if `--auth-proxy` is set the authorized keys option will be
ignored.

There is an example program
[bin/test_proxy.py](https://github.com/rclone/rclone/blob/master/test_proxy.py)
in the rclone source code.

The program's job is to take a `user` and `pass` on the input and turn
those into the config for a backend on STDOUT in JSON format.  This
config will have any default parameters for the backend added, but it
won't use configuration from environment variables or command line
options - it is the job of the proxy program to make a complete
config.

This config generated must have this extra parameter
- `_root` - root to use for the backend

And it may have this parameter
- `_obscure` - comma separated strings for parameters to obscure

If password authentication was used by the client, input to the proxy
process (on STDIN) would look similar to this:

```
{
	"user": "me",
	"pass": "mypassword"
}
```

If public-key authentication was used by the client, input to the
proxy process (on STDIN) would look similar to this:

```
{
	"user": "me",
	"public_key": "AAAAB3NzaC1yc2EAAAADAQABAAABAQDuwESFdAe14hVS6omeyX7edc...JQdf"
}
```

And as an example return this on STDOUT

```
{
	"type": "sftp",
	"_root": "",
	"_obscure": "pass",
	"user": "me",
	"pass": "mypassword",
	"host": "sftp.example.com"
}
```

This would mean that an SFTP backend would be created on the fly for
the `user` and `pass`/`public_key` returned in the output to the host given.  Note
that since `_obscure` is set to `pass`, rclone will obscure the `pass`
parameter before creating the backend (which is required for sftp
backends).

The program can manipulate the supplied `user` in any way, for example
to make proxy to many different sftp backends, you could make the
`user` be `user@example.com` and then set the `host` to `example.com`
in the output and the user to `user`. For security you'd probably want
to restrict the `host` to a limited list.

Note that an internal cache is keyed on `user` so only use that for
configuration, don't use `pass` or `public_key`.  This also means that if a user's
password or public-key is changed the cache will need to expire (which takes 5 mins)
before it takes effect.

This can be used to build general purpose proxies to any kind of
backend that rclone supports.  


```
rclone serve ftp remote:path [flags]
```

## Options

```
      --addr string                            IPaddress:Port or :Port to bind server to. (default "localhost:2121")
      --auth-proxy string                      A program to use to create the backend from the auth.
      --dir-cache-time duration                Time to cache directory entries for. (default 5m0s)
      --dir-perms FileMode                     Directory permissions (default 0777)
      --file-perms FileMode                    File permissions (default 0666)
      --gid uint32                             Override the gid field set by the filesystem. (default 1000)
  -h, --help                                   help for ftp
      --no-checksum                            Don't compare checksums on up/download.
      --no-modtime                             Don't read/write the modification time (can speed things up).
      --no-seek                                Don't allow seeking in files.
      --pass string                            Password for authentication. (empty value allow every password)
      --passive-port string                    Passive port range to use. (default "30000-32000")
      --poll-interval duration                 Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable. (default 1m0s)
      --public-ip string                       Public IP address to advertise for passive connections.
      --read-only                              Mount read-only.
      --uid uint32                             Override the uid field set by the filesystem. (default 1000)
      --umask int                              Override the permission bits set by the filesystem. (default 2)
      --user string                            User name for authentication. (default "anonymous")
      --vfs-cache-max-age duration             Max age of objects in the cache. (default 1h0m0s)
      --vfs-cache-max-size SizeSuffix          Max total size of objects in the cache. (default off)
      --vfs-cache-mode CacheMode               Cache mode off|minimal|writes|full (default off)
      --vfs-cache-poll-interval duration       Interval to poll the cache for stale objects. (default 1m0s)
      --vfs-case-insensitive                   If a file name not found, find a case insensitive match.
      --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
      --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)
      --vfs-read-wait duration                 Time to wait for in-sequence read before seeking. (default 20ms)
      --vfs-write-wait duration                Time to wait for in-sequence write before giving error. (default 1s)
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone serve](https://rclone.org/commands/rclone_serve/)	 - Serve a remote over a protocol.

# rclone serve http

Serve the remote over HTTP.

## Synopsis

rclone serve http implements a basic web server to serve the remote
over HTTP.  This can be viewed in a web browser or you can make a
remote of type http read from it.

You can use the filter flags (eg --include, --exclude) to control what
is served.

The server will log errors.  Use -v to see access logs.

--bwlimit will be respected for file transfers.  Use --stats to
control the stats printing.

## Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.  You can use port
:0 to let the OS choose an available port.

If you set --addr to listen on a public or LAN accessible IP address
then using Authentication is advised - see the next section for info.

--server-read-timeout and --server-write-timeout can be used to
control the timeouts on the server.  Note that this is the total time
for a transfer.

--max-header-bytes controls the maximum number of bytes the server will
accept in the HTTP header.

--baseurl controls the URL prefix that rclone serves from.  By default
rclone will serve from the root.  If you used --baseurl "/rclone" then
rclone would serve from a URL starting with "/rclone/".  This is
useful if you wish to proxy rclone serve.  Rclone automatically
inserts leading and trailing "/" on --baseurl, so --baseurl "rclone",
--baseurl "/rclone" and --baseurl "/rclone/" are all treated
identically.

--template allows a user to specify a custom markup template for http
and webdav serve functions.  The server exports the following markup
to be used within the template to server pages:

| Parameter   | Description |
| :---------- | :---------- |
| .Name       | The full path of a file/directory. |
| .Title      | Directory listing of .Name |
| .Sort       | The current sort used.  This is changeable via ?sort= parameter |
|             | Sort Options: namedirfist,name,size,time (default namedirfirst) |
| .Order      | The current ordering used.  This is changeable via ?order= parameter |
|             | Order Options: asc,desc (default asc) |
| .Query      | Currently unused. |
| .Breadcrumb | Allows for creating a relative navigation |
|-- .Link     | The relative to the root link of the Text. |
|-- .Text     | The Name of the directory. |
| .Entries    | Information about a specific file/directory. |
|-- .URL      | The 'url' of an entry.  |
|-- .Leaf     | Currently same as 'URL' but intended to be 'just' the name. |
|-- .IsDir    | Boolean for if an entry is a directory or not. |
|-- .Size     | Size in Bytes of the entry. |
|-- .ModTime  | The UTC timestamp of an entry. |

### Authentication

By default this will serve files without needing a login.

You can either use an htpasswd file which can take lots of users, or
set a single username and password with the --user and --pass flags.

Use --htpasswd /path/to/htpasswd to provide an htpasswd file.  This is
in standard apache format and supports MD5, SHA1 and BCrypt for basic
authentication.  Bcrypt is recommended.

To create an htpasswd file:

    touch htpasswd
    htpasswd -B htpasswd user
    htpasswd -B htpasswd anotherUser

The password file can be updated while rclone is running.

Use --realm to set the authentication realm.

### SSL/TLS

By default this will serve over http.  If you want you can serve over
https.  You will need to supply the --cert and --key flags.  If you
wish to do client side certificate validation then you will need to
supply --client-ca also.

--cert should be either a PEM encoded certificate or a concatenation
of that with the CA certificate.  --key should be the PEM encoded
private key and --client-ca should be the PEM encoded client
certificate authority certificate.

## Directory Cache

Using the `--dir-cache-time` flag, you can set how long a
directory should be considered up to date and not refreshed from the
backend. Changes made locally in the mount may appear immediately or
invalidate the cache. However, changes done on the remote will only
be picked up once the cache expires if the backend configured does not
support polling for changes. If the backend supports polling, changes
will be picked up on within the polling interval.

Alternatively, you can send a `SIGHUP` signal to rclone for
it to flush all directory caches, regardless of how old they are.
Assuming only one rclone instance is running, you can reset the cache
like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a [remote control](/rc) then you can use
rclone rc to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

## File Buffering

The `--buffer-size` flag determines the amount of memory,
that will be used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of
data in memory at all times. The buffered data is bound to one file
descriptor and won't be shared between multiple open file descriptors
of the same file.

This flag is a upper limit for the used memory per file descriptor.
The buffer will only use memory for data that is downloaded but not
not yet read. If the buffer is empty, only a small amount of memory
will be used.
The maximum memory used by rclone for buffering can be up to
`--buffer-size * open files`.

## File Caching

These flags control the VFS file caching options.  The VFS layer is
used by rclone mount to make a cloud storage system work more like a
normal file system.

You'll need to enable VFS caching if you want, for example, to read
and write simultaneously to a file.  See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

If run with `-vv` rclone will print the location of the file cache.  The
files are stored in the user cache file area which is OS dependent but
can be controlled with `--cache-dir` or setting the appropriate
environment variable.

The cache has 4 different modes selected by `--vfs-cache-mode`.
The higher the cache mode the more compatible rclone becomes at the
cost of using disk space.

Note that files are written back to the remote only when they are
closed so if rclone is quit or dies with open files then these won't
get written back to the remote.  However they will still be in the on
disk cache.

If using --vfs-cache-max-size note that the cache may exceed this size
for two reasons.  Firstly because it is only checked every
--vfs-cache-poll-interval.  Secondly because open files cannot be
evicted from the cache.

### --vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

  * Files can't be opened for both read AND write
  * Files opened for write can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files open for read with O_TRUNC will be opened write only
  * Files open for write only will behave as if O_TRUNC was supplied
  * Open modes O_APPEND, O_TRUNC are ignored
  * If an upload fails it can't be retried

### --vfs-cache-mode minimal

This is very similar to "off" except that files opened for read AND
write will be buffered to disks.  This means that files opened for
write will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

  * Files opened for write only can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files opened for write only will ignore O_APPEND, O_TRUNC
  * If an upload fails it can't be retried

### --vfs-cache-mode writes

In this mode files opened for read only are still read directly from
the remote, write only and read/write files are buffered to disk
first.

This mode should support all normal file system operations.

If an upload fails it will be retried up to --low-level-retries times.

### --vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk.  When
a file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at
the cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk,
it will be kept on the disk after it is written to the remote.  It
will be purged on a schedule according to `--vfs-cache-max-age`.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
--low-level-retries times.

## Case Sensitivity

Linux file systems are case-sensitive: two files can differ only
by case, and the exact case must be used when opening a file.

Windows is not like most other operating systems supported by rclone.
File systems in modern Windows are case-insensitive but case-preserving:
although existing files can be opened using any case, the exact case used
to create the file is preserved and available for programs to query.
It is not allowed for two files in the same directory to differ only by case.

Usually file systems on macOS are case-insensitive. It is possible to make macOS
file systems case-sensitive but that is not the default

The "--vfs-case-insensitive" mount flag controls how rclone handles these
two cases. If its value is "false", rclone passes file names to the mounted
file system as is. If the flag is "true" (or appears without a value on
command line), rclone may perform a "fixup" as explained below.

The user may specify a file name to open/delete/rename/etc with a case
different than what is stored on mounted file system. If an argument refers
to an existing file with exactly the same name, then the case of the existing
file on the disk will be used. However, if a file name with exactly the same
name is not found but a name differing only by case exists, rclone will
transparently fixup the name. This fixup happens only when an existing file
is requested. Case sensitivity of file names created anew by rclone is
controlled by an underlying mounted file system.

Note that case sensitivity of the operating system running rclone (the target)
may differ from case sensitivity of a file system mounted by rclone (the source).
The flag controls whether "fixup" is performed to satisfy the target.

If the flag is not provided on command line, then its default value depends
on the operating system where rclone runs: "true" on Windows and macOS, "false"
otherwise. If the flag is provided without a value, then it is "true".


```
rclone serve http remote:path [flags]
```

## Options

```
      --addr string                            IPaddress:Port or :Port to bind server to. (default "localhost:8080")
      --baseurl string                         Prefix for URLs - leave blank for root.
      --cert string                            SSL PEM key (concatenation of certificate and CA certificate)
      --client-ca string                       Client certificate authority to verify clients with
      --dir-cache-time duration                Time to cache directory entries for. (default 5m0s)
      --dir-perms FileMode                     Directory permissions (default 0777)
      --file-perms FileMode                    File permissions (default 0666)
      --gid uint32                             Override the gid field set by the filesystem. (default 1000)
  -h, --help                                   help for http
      --htpasswd string                        htpasswd file - if not provided no authentication is done
      --key string                             SSL PEM Private key
      --max-header-bytes int                   Maximum size of request header (default 4096)
      --no-checksum                            Don't compare checksums on up/download.
      --no-modtime                             Don't read/write the modification time (can speed things up).
      --no-seek                                Don't allow seeking in files.
      --pass string                            Password for authentication.
      --poll-interval duration                 Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable. (default 1m0s)
      --read-only                              Mount read-only.
      --realm string                           realm for authentication (default "rclone")
      --server-read-timeout duration           Timeout for server reading data (default 1h0m0s)
      --server-write-timeout duration          Timeout for server writing data (default 1h0m0s)
      --template string                        User Specified Template.
      --uid uint32                             Override the uid field set by the filesystem. (default 1000)
      --umask int                              Override the permission bits set by the filesystem. (default 2)
      --user string                            User name for authentication.
      --vfs-cache-max-age duration             Max age of objects in the cache. (default 1h0m0s)
      --vfs-cache-max-size SizeSuffix          Max total size of objects in the cache. (default off)
      --vfs-cache-mode CacheMode               Cache mode off|minimal|writes|full (default off)
      --vfs-cache-poll-interval duration       Interval to poll the cache for stale objects. (default 1m0s)
      --vfs-case-insensitive                   If a file name not found, find a case insensitive match.
      --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
      --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)
      --vfs-read-wait duration                 Time to wait for in-sequence read before seeking. (default 20ms)
      --vfs-write-wait duration                Time to wait for in-sequence write before giving error. (default 1s)
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone serve](https://rclone.org/commands/rclone_serve/)	 - Serve a remote over a protocol.

# rclone serve restic

Serve the remote for restic's REST API.

## Synopsis

rclone serve restic implements restic's REST backend API
over HTTP.  This allows restic to use rclone as a data storage
mechanism for cloud providers that restic does not support directly.

[Restic](https://restic.net/) is a command line program for doing
backups.

The server will log errors.  Use -v to see access logs.

--bwlimit will be respected for file transfers.  Use --stats to
control the stats printing.

## Setting up rclone for use by restic ###

First [set up a remote for your chosen cloud provider](https://rclone.org/docs/#configure).

Once you have set up the remote, check it is working with, for example
"rclone lsd remote:".  You may have called the remote something other
than "remote:" - just substitute whatever you called it in the
following instructions.

Now start the rclone restic server

    rclone serve restic -v remote:backup

Where you can replace "backup" in the above by whatever path in the
remote you wish to use.

By default this will serve on "localhost:8080" you can change this
with use of the "--addr" flag.

You might wish to start this server on boot.

## Setting up restic to use rclone ###

Now you can [follow the restic
instructions](http://restic.readthedocs.io/en/latest/030_preparing_a_new_repo.html#rest-server)
on setting up restic.

Note that you will need restic 0.8.2 or later to interoperate with
rclone.

For the example above you will want to use "http://localhost:8080/" as
the URL for the REST server.

For example:

    $ export RESTIC_REPOSITORY=rest:http://localhost:8080/
    $ export RESTIC_PASSWORD=yourpassword
    $ restic init
    created restic backend 8b1a4b56ae at rest:http://localhost:8080/

    Please note that knowledge of your password is required to access
    the repository. Losing your password means that your data is
    irrecoverably lost.
    $ restic backup /path/to/files/to/backup
    scan [/path/to/files/to/backup]
    scanned 189 directories, 312 files in 0:00
    [0:00] 100.00%  38.128 MiB / 38.128 MiB  501 / 501 items  0 errors  ETA 0:00
    duration: 0:00
    snapshot 45c8fdd8 saved

### Multiple repositories ####

Note that you can use the endpoint to host multiple repositories.  Do
this by adding a directory name or path after the URL.  Note that
these **must** end with /.  Eg

    $ export RESTIC_REPOSITORY=rest:http://localhost:8080/user1repo/
    # backup user1 stuff
    $ export RESTIC_REPOSITORY=rest:http://localhost:8080/user2repo/
    # backup user2 stuff

### Private repositories ####

The "--private-repos" flag can be used to limit users to repositories starting
with a path of `/<username>/`.

## Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.  You can use port
:0 to let the OS choose an available port.

If you set --addr to listen on a public or LAN accessible IP address
then using Authentication is advised - see the next section for info.

--server-read-timeout and --server-write-timeout can be used to
control the timeouts on the server.  Note that this is the total time
for a transfer.

--max-header-bytes controls the maximum number of bytes the server will
accept in the HTTP header.

--baseurl controls the URL prefix that rclone serves from.  By default
rclone will serve from the root.  If you used --baseurl "/rclone" then
rclone would serve from a URL starting with "/rclone/".  This is
useful if you wish to proxy rclone serve.  Rclone automatically
inserts leading and trailing "/" on --baseurl, so --baseurl "rclone",
--baseurl "/rclone" and --baseurl "/rclone/" are all treated
identically.

--template allows a user to specify a custom markup template for http
and webdav serve functions.  The server exports the following markup
to be used within the template to server pages:

| Parameter   | Description |
| :---------- | :---------- |
| .Name       | The full path of a file/directory. |
| .Title      | Directory listing of .Name |
| .Sort       | The current sort used.  This is changeable via ?sort= parameter |
|             | Sort Options: namedirfist,name,size,time (default namedirfirst) |
| .Order      | The current ordering used.  This is changeable via ?order= parameter |
|             | Order Options: asc,desc (default asc) |
| .Query      | Currently unused. |
| .Breadcrumb | Allows for creating a relative navigation |
|-- .Link     | The relative to the root link of the Text. |
|-- .Text     | The Name of the directory. |
| .Entries    | Information about a specific file/directory. |
|-- .URL      | The 'url' of an entry.  |
|-- .Leaf     | Currently same as 'URL' but intended to be 'just' the name. |
|-- .IsDir    | Boolean for if an entry is a directory or not. |
|-- .Size     | Size in Bytes of the entry. |
|-- .ModTime  | The UTC timestamp of an entry. |

### Authentication

By default this will serve files without needing a login.

You can either use an htpasswd file which can take lots of users, or
set a single username and password with the --user and --pass flags.

Use --htpasswd /path/to/htpasswd to provide an htpasswd file.  This is
in standard apache format and supports MD5, SHA1 and BCrypt for basic
authentication.  Bcrypt is recommended.

To create an htpasswd file:

    touch htpasswd
    htpasswd -B htpasswd user
    htpasswd -B htpasswd anotherUser

The password file can be updated while rclone is running.

Use --realm to set the authentication realm.

### SSL/TLS

By default this will serve over http.  If you want you can serve over
https.  You will need to supply the --cert and --key flags.  If you
wish to do client side certificate validation then you will need to
supply --client-ca also.

--cert should be either a PEM encoded certificate or a concatenation
of that with the CA certificate.  --key should be the PEM encoded
private key and --client-ca should be the PEM encoded client
certificate authority certificate.


```
rclone serve restic remote:path [flags]
```

## Options

```
      --addr string                     IPaddress:Port or :Port to bind server to. (default "localhost:8080")
      --append-only                     disallow deletion of repository data
      --baseurl string                  Prefix for URLs - leave blank for root.
      --cert string                     SSL PEM key (concatenation of certificate and CA certificate)
      --client-ca string                Client certificate authority to verify clients with
  -h, --help                            help for restic
      --htpasswd string                 htpasswd file - if not provided no authentication is done
      --key string                      SSL PEM Private key
      --max-header-bytes int            Maximum size of request header (default 4096)
      --pass string                     Password for authentication.
      --private-repos                   users can only access their private repo
      --realm string                    realm for authentication (default "rclone")
      --server-read-timeout duration    Timeout for server reading data (default 1h0m0s)
      --server-write-timeout duration   Timeout for server writing data (default 1h0m0s)
      --stdio                           run an HTTP2 server on stdin/stdout
      --template string                 User Specified Template.
      --user string                     User name for authentication.
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone serve](https://rclone.org/commands/rclone_serve/)	 - Serve a remote over a protocol.

# rclone serve sftp

Serve the remote over SFTP.

## Synopsis

rclone serve sftp implements an SFTP server to serve the remote
over SFTP.  This can be used with an SFTP client or you can make a
remote of type sftp to use with it.

You can use the filter flags (eg --include, --exclude) to control what
is served.

The server will log errors.  Use -v to see access logs.

--bwlimit will be respected for file transfers.  Use --stats to
control the stats printing.

You must provide some means of authentication, either with --user/--pass,
an authorized keys file (specify location with --authorized-keys - the
default is the same as ssh), an --auth-proxy, or set the --no-auth flag for no
authentication when logging in.

Note that this also implements a small number of shell commands so
that it can provide md5sum/sha1sum/df information for the rclone sftp
backend.  This means that is can support SHA1SUMs, MD5SUMs and the
about command when paired with the rclone sftp backend.

If you don't supply a --key then rclone will generate one and cache it
for later use.

By default the server binds to localhost:2022 - if you want it to be
reachable externally then supply "--addr :2022" for example.

Note that the default of "--vfs-cache-mode off" is fine for the rclone
sftp backend, but it may not be with other SFTP clients.


## Directory Cache

Using the `--dir-cache-time` flag, you can set how long a
directory should be considered up to date and not refreshed from the
backend. Changes made locally in the mount may appear immediately or
invalidate the cache. However, changes done on the remote will only
be picked up once the cache expires if the backend configured does not
support polling for changes. If the backend supports polling, changes
will be picked up on within the polling interval.

Alternatively, you can send a `SIGHUP` signal to rclone for
it to flush all directory caches, regardless of how old they are.
Assuming only one rclone instance is running, you can reset the cache
like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a [remote control](/rc) then you can use
rclone rc to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

## File Buffering

The `--buffer-size` flag determines the amount of memory,
that will be used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of
data in memory at all times. The buffered data is bound to one file
descriptor and won't be shared between multiple open file descriptors
of the same file.

This flag is a upper limit for the used memory per file descriptor.
The buffer will only use memory for data that is downloaded but not
not yet read. If the buffer is empty, only a small amount of memory
will be used.
The maximum memory used by rclone for buffering can be up to
`--buffer-size * open files`.

## File Caching

These flags control the VFS file caching options.  The VFS layer is
used by rclone mount to make a cloud storage system work more like a
normal file system.

You'll need to enable VFS caching if you want, for example, to read
and write simultaneously to a file.  See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

If run with `-vv` rclone will print the location of the file cache.  The
files are stored in the user cache file area which is OS dependent but
can be controlled with `--cache-dir` or setting the appropriate
environment variable.

The cache has 4 different modes selected by `--vfs-cache-mode`.
The higher the cache mode the more compatible rclone becomes at the
cost of using disk space.

Note that files are written back to the remote only when they are
closed so if rclone is quit or dies with open files then these won't
get written back to the remote.  However they will still be in the on
disk cache.

If using --vfs-cache-max-size note that the cache may exceed this size
for two reasons.  Firstly because it is only checked every
--vfs-cache-poll-interval.  Secondly because open files cannot be
evicted from the cache.

### --vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

  * Files can't be opened for both read AND write
  * Files opened for write can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files open for read with O_TRUNC will be opened write only
  * Files open for write only will behave as if O_TRUNC was supplied
  * Open modes O_APPEND, O_TRUNC are ignored
  * If an upload fails it can't be retried

### --vfs-cache-mode minimal

This is very similar to "off" except that files opened for read AND
write will be buffered to disks.  This means that files opened for
write will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

  * Files opened for write only can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files opened for write only will ignore O_APPEND, O_TRUNC
  * If an upload fails it can't be retried

### --vfs-cache-mode writes

In this mode files opened for read only are still read directly from
the remote, write only and read/write files are buffered to disk
first.

This mode should support all normal file system operations.

If an upload fails it will be retried up to --low-level-retries times.

### --vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk.  When
a file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at
the cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk,
it will be kept on the disk after it is written to the remote.  It
will be purged on a schedule according to `--vfs-cache-max-age`.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
--low-level-retries times.

## Case Sensitivity

Linux file systems are case-sensitive: two files can differ only
by case, and the exact case must be used when opening a file.

Windows is not like most other operating systems supported by rclone.
File systems in modern Windows are case-insensitive but case-preserving:
although existing files can be opened using any case, the exact case used
to create the file is preserved and available for programs to query.
It is not allowed for two files in the same directory to differ only by case.

Usually file systems on macOS are case-insensitive. It is possible to make macOS
file systems case-sensitive but that is not the default

The "--vfs-case-insensitive" mount flag controls how rclone handles these
two cases. If its value is "false", rclone passes file names to the mounted
file system as is. If the flag is "true" (or appears without a value on
command line), rclone may perform a "fixup" as explained below.

The user may specify a file name to open/delete/rename/etc with a case
different than what is stored on mounted file system. If an argument refers
to an existing file with exactly the same name, then the case of the existing
file on the disk will be used. However, if a file name with exactly the same
name is not found but a name differing only by case exists, rclone will
transparently fixup the name. This fixup happens only when an existing file
is requested. Case sensitivity of file names created anew by rclone is
controlled by an underlying mounted file system.

Note that case sensitivity of the operating system running rclone (the target)
may differ from case sensitivity of a file system mounted by rclone (the source).
The flag controls whether "fixup" is performed to satisfy the target.

If the flag is not provided on command line, then its default value depends
on the operating system where rclone runs: "true" on Windows and macOS, "false"
otherwise. If the flag is provided without a value, then it is "true".

## Auth Proxy

If you supply the parameter `--auth-proxy /path/to/program` then
rclone will use that program to generate backends on the fly which
then are used to authenticate incoming requests.  This uses a simple
JSON based protocl with input on STDIN and output on STDOUT.

**PLEASE NOTE:** `--auth-proxy` and `--authorized-keys` cannot be used
together, if `--auth-proxy` is set the authorized keys option will be
ignored.

There is an example program
[bin/test_proxy.py](https://github.com/rclone/rclone/blob/master/test_proxy.py)
in the rclone source code.

The program's job is to take a `user` and `pass` on the input and turn
those into the config for a backend on STDOUT in JSON format.  This
config will have any default parameters for the backend added, but it
won't use configuration from environment variables or command line
options - it is the job of the proxy program to make a complete
config.

This config generated must have this extra parameter
- `_root` - root to use for the backend

And it may have this parameter
- `_obscure` - comma separated strings for parameters to obscure

If password authentication was used by the client, input to the proxy
process (on STDIN) would look similar to this:

```
{
	"user": "me",
	"pass": "mypassword"
}
```

If public-key authentication was used by the client, input to the
proxy process (on STDIN) would look similar to this:

```
{
	"user": "me",
	"public_key": "AAAAB3NzaC1yc2EAAAADAQABAAABAQDuwESFdAe14hVS6omeyX7edc...JQdf"
}
```

And as an example return this on STDOUT

```
{
	"type": "sftp",
	"_root": "",
	"_obscure": "pass",
	"user": "me",
	"pass": "mypassword",
	"host": "sftp.example.com"
}
```

This would mean that an SFTP backend would be created on the fly for
the `user` and `pass`/`public_key` returned in the output to the host given.  Note
that since `_obscure` is set to `pass`, rclone will obscure the `pass`
parameter before creating the backend (which is required for sftp
backends).

The program can manipulate the supplied `user` in any way, for example
to make proxy to many different sftp backends, you could make the
`user` be `user@example.com` and then set the `host` to `example.com`
in the output and the user to `user`. For security you'd probably want
to restrict the `host` to a limited list.

Note that an internal cache is keyed on `user` so only use that for
configuration, don't use `pass` or `public_key`.  This also means that if a user's
password or public-key is changed the cache will need to expire (which takes 5 mins)
before it takes effect.

This can be used to build general purpose proxies to any kind of
backend that rclone supports.  


```
rclone serve sftp remote:path [flags]
```

## Options

```
      --addr string                            IPaddress:Port or :Port to bind server to. (default "localhost:2022")
      --auth-proxy string                      A program to use to create the backend from the auth.
      --authorized-keys string                 Authorized keys file (default "~/.ssh/authorized_keys")
      --dir-cache-time duration                Time to cache directory entries for. (default 5m0s)
      --dir-perms FileMode                     Directory permissions (default 0777)
      --file-perms FileMode                    File permissions (default 0666)
      --gid uint32                             Override the gid field set by the filesystem. (default 1000)
  -h, --help                                   help for sftp
      --key stringArray                        SSH private host key file (Can be multi-valued, leave blank to auto generate)
      --no-auth                                Allow connections with no authentication if set.
      --no-checksum                            Don't compare checksums on up/download.
      --no-modtime                             Don't read/write the modification time (can speed things up).
      --no-seek                                Don't allow seeking in files.
      --pass string                            Password for authentication.
      --poll-interval duration                 Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable. (default 1m0s)
      --read-only                              Mount read-only.
      --uid uint32                             Override the uid field set by the filesystem. (default 1000)
      --umask int                              Override the permission bits set by the filesystem. (default 2)
      --user string                            User name for authentication.
      --vfs-cache-max-age duration             Max age of objects in the cache. (default 1h0m0s)
      --vfs-cache-max-size SizeSuffix          Max total size of objects in the cache. (default off)
      --vfs-cache-mode CacheMode               Cache mode off|minimal|writes|full (default off)
      --vfs-cache-poll-interval duration       Interval to poll the cache for stale objects. (default 1m0s)
      --vfs-case-insensitive                   If a file name not found, find a case insensitive match.
      --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
      --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)
      --vfs-read-wait duration                 Time to wait for in-sequence read before seeking. (default 20ms)
      --vfs-write-wait duration                Time to wait for in-sequence write before giving error. (default 1s)
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone serve](https://rclone.org/commands/rclone_serve/)	 - Serve a remote over a protocol.

# rclone serve webdav

Serve remote:path over webdav.

## Synopsis


rclone serve webdav implements a basic webdav server to serve the
remote over HTTP via the webdav protocol. This can be viewed with a
webdav client, through a web browser, or you can make a remote of
type webdav to read and write it.

## Webdav options

### --etag-hash 

This controls the ETag header.  Without this flag the ETag will be
based on the ModTime and Size of the object.

If this flag is set to "auto" then rclone will choose the first
supported hash on the backend or you can use a named hash such as
"MD5" or "SHA-1".

Use "rclone hashsum" to see the full list.


## Server options

Use --addr to specify which IP address and port the server should
listen on, eg --addr 1.2.3.4:8000 or --addr :8080 to listen to all
IPs.  By default it only listens on localhost.  You can use port
:0 to let the OS choose an available port.

If you set --addr to listen on a public or LAN accessible IP address
then using Authentication is advised - see the next section for info.

--server-read-timeout and --server-write-timeout can be used to
control the timeouts on the server.  Note that this is the total time
for a transfer.

--max-header-bytes controls the maximum number of bytes the server will
accept in the HTTP header.

--baseurl controls the URL prefix that rclone serves from.  By default
rclone will serve from the root.  If you used --baseurl "/rclone" then
rclone would serve from a URL starting with "/rclone/".  This is
useful if you wish to proxy rclone serve.  Rclone automatically
inserts leading and trailing "/" on --baseurl, so --baseurl "rclone",
--baseurl "/rclone" and --baseurl "/rclone/" are all treated
identically.

--template allows a user to specify a custom markup template for http
and webdav serve functions.  The server exports the following markup
to be used within the template to server pages:

| Parameter   | Description |
| :---------- | :---------- |
| .Name       | The full path of a file/directory. |
| .Title      | Directory listing of .Name |
| .Sort       | The current sort used.  This is changeable via ?sort= parameter |
|             | Sort Options: namedirfist,name,size,time (default namedirfirst) |
| .Order      | The current ordering used.  This is changeable via ?order= parameter |
|             | Order Options: asc,desc (default asc) |
| .Query      | Currently unused. |
| .Breadcrumb | Allows for creating a relative navigation |
|-- .Link     | The relative to the root link of the Text. |
|-- .Text     | The Name of the directory. |
| .Entries    | Information about a specific file/directory. |
|-- .URL      | The 'url' of an entry.  |
|-- .Leaf     | Currently same as 'URL' but intended to be 'just' the name. |
|-- .IsDir    | Boolean for if an entry is a directory or not. |
|-- .Size     | Size in Bytes of the entry. |
|-- .ModTime  | The UTC timestamp of an entry. |

### Authentication

By default this will serve files without needing a login.

You can either use an htpasswd file which can take lots of users, or
set a single username and password with the --user and --pass flags.

Use --htpasswd /path/to/htpasswd to provide an htpasswd file.  This is
in standard apache format and supports MD5, SHA1 and BCrypt for basic
authentication.  Bcrypt is recommended.

To create an htpasswd file:

    touch htpasswd
    htpasswd -B htpasswd user
    htpasswd -B htpasswd anotherUser

The password file can be updated while rclone is running.

Use --realm to set the authentication realm.

### SSL/TLS

By default this will serve over http.  If you want you can serve over
https.  You will need to supply the --cert and --key flags.  If you
wish to do client side certificate validation then you will need to
supply --client-ca also.

--cert should be either a PEM encoded certificate or a concatenation
of that with the CA certificate.  --key should be the PEM encoded
private key and --client-ca should be the PEM encoded client
certificate authority certificate.

## Directory Cache

Using the `--dir-cache-time` flag, you can set how long a
directory should be considered up to date and not refreshed from the
backend. Changes made locally in the mount may appear immediately or
invalidate the cache. However, changes done on the remote will only
be picked up once the cache expires if the backend configured does not
support polling for changes. If the backend supports polling, changes
will be picked up on within the polling interval.

Alternatively, you can send a `SIGHUP` signal to rclone for
it to flush all directory caches, regardless of how old they are.
Assuming only one rclone instance is running, you can reset the cache
like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a [remote control](/rc) then you can use
rclone rc to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

## File Buffering

The `--buffer-size` flag determines the amount of memory,
that will be used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of
data in memory at all times. The buffered data is bound to one file
descriptor and won't be shared between multiple open file descriptors
of the same file.

This flag is a upper limit for the used memory per file descriptor.
The buffer will only use memory for data that is downloaded but not
not yet read. If the buffer is empty, only a small amount of memory
will be used.
The maximum memory used by rclone for buffering can be up to
`--buffer-size * open files`.

## File Caching

These flags control the VFS file caching options.  The VFS layer is
used by rclone mount to make a cloud storage system work more like a
normal file system.

You'll need to enable VFS caching if you want, for example, to read
and write simultaneously to a file.  See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

If run with `-vv` rclone will print the location of the file cache.  The
files are stored in the user cache file area which is OS dependent but
can be controlled with `--cache-dir` or setting the appropriate
environment variable.

The cache has 4 different modes selected by `--vfs-cache-mode`.
The higher the cache mode the more compatible rclone becomes at the
cost of using disk space.

Note that files are written back to the remote only when they are
closed so if rclone is quit or dies with open files then these won't
get written back to the remote.  However they will still be in the on
disk cache.

If using --vfs-cache-max-size note that the cache may exceed this size
for two reasons.  Firstly because it is only checked every
--vfs-cache-poll-interval.  Secondly because open files cannot be
evicted from the cache.

### --vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

  * Files can't be opened for both read AND write
  * Files opened for write can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files open for read with O_TRUNC will be opened write only
  * Files open for write only will behave as if O_TRUNC was supplied
  * Open modes O_APPEND, O_TRUNC are ignored
  * If an upload fails it can't be retried

### --vfs-cache-mode minimal

This is very similar to "off" except that files opened for read AND
write will be buffered to disks.  This means that files opened for
write will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

  * Files opened for write only can't be seeked
  * Existing files opened for write must have O_TRUNC set
  * Files opened for write only will ignore O_APPEND, O_TRUNC
  * If an upload fails it can't be retried

### --vfs-cache-mode writes

In this mode files opened for read only are still read directly from
the remote, write only and read/write files are buffered to disk
first.

This mode should support all normal file system operations.

If an upload fails it will be retried up to --low-level-retries times.

### --vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk.  When
a file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at
the cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk,
it will be kept on the disk after it is written to the remote.  It
will be purged on a schedule according to `--vfs-cache-max-age`.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
--low-level-retries times.

## Case Sensitivity

Linux file systems are case-sensitive: two files can differ only
by case, and the exact case must be used when opening a file.

Windows is not like most other operating systems supported by rclone.
File systems in modern Windows are case-insensitive but case-preserving:
although existing files can be opened using any case, the exact case used
to create the file is preserved and available for programs to query.
It is not allowed for two files in the same directory to differ only by case.

Usually file systems on macOS are case-insensitive. It is possible to make macOS
file systems case-sensitive but that is not the default

The "--vfs-case-insensitive" mount flag controls how rclone handles these
two cases. If its value is "false", rclone passes file names to the mounted
file system as is. If the flag is "true" (or appears without a value on
command line), rclone may perform a "fixup" as explained below.

The user may specify a file name to open/delete/rename/etc with a case
different than what is stored on mounted file system. If an argument refers
to an existing file with exactly the same name, then the case of the existing
file on the disk will be used. However, if a file name with exactly the same
name is not found but a name differing only by case exists, rclone will
transparently fixup the name. This fixup happens only when an existing file
is requested. Case sensitivity of file names created anew by rclone is
controlled by an underlying mounted file system.

Note that case sensitivity of the operating system running rclone (the target)
may differ from case sensitivity of a file system mounted by rclone (the source).
The flag controls whether "fixup" is performed to satisfy the target.

If the flag is not provided on command line, then its default value depends
on the operating system where rclone runs: "true" on Windows and macOS, "false"
otherwise. If the flag is provided without a value, then it is "true".

## Auth Proxy

If you supply the parameter `--auth-proxy /path/to/program` then
rclone will use that program to generate backends on the fly which
then are used to authenticate incoming requests.  This uses a simple
JSON based protocl with input on STDIN and output on STDOUT.

**PLEASE NOTE:** `--auth-proxy` and `--authorized-keys` cannot be used
together, if `--auth-proxy` is set the authorized keys option will be
ignored.

There is an example program
[bin/test_proxy.py](https://github.com/rclone/rclone/blob/master/test_proxy.py)
in the rclone source code.

The program's job is to take a `user` and `pass` on the input and turn
those into the config for a backend on STDOUT in JSON format.  This
config will have any default parameters for the backend added, but it
won't use configuration from environment variables or command line
options - it is the job of the proxy program to make a complete
config.

This config generated must have this extra parameter
- `_root` - root to use for the backend

And it may have this parameter
- `_obscure` - comma separated strings for parameters to obscure

If password authentication was used by the client, input to the proxy
process (on STDIN) would look similar to this:

```
{
	"user": "me",
	"pass": "mypassword"
}
```

If public-key authentication was used by the client, input to the
proxy process (on STDIN) would look similar to this:

```
{
	"user": "me",
	"public_key": "AAAAB3NzaC1yc2EAAAADAQABAAABAQDuwESFdAe14hVS6omeyX7edc...JQdf"
}
```

And as an example return this on STDOUT

```
{
	"type": "sftp",
	"_root": "",
	"_obscure": "pass",
	"user": "me",
	"pass": "mypassword",
	"host": "sftp.example.com"
}
```

This would mean that an SFTP backend would be created on the fly for
the `user` and `pass`/`public_key` returned in the output to the host given.  Note
that since `_obscure` is set to `pass`, rclone will obscure the `pass`
parameter before creating the backend (which is required for sftp
backends).

The program can manipulate the supplied `user` in any way, for example
to make proxy to many different sftp backends, you could make the
`user` be `user@example.com` and then set the `host` to `example.com`
in the output and the user to `user`. For security you'd probably want
to restrict the `host` to a limited list.

Note that an internal cache is keyed on `user` so only use that for
configuration, don't use `pass` or `public_key`.  This also means that if a user's
password or public-key is changed the cache will need to expire (which takes 5 mins)
before it takes effect.

This can be used to build general purpose proxies to any kind of
backend that rclone supports.  


```
rclone serve webdav remote:path [flags]
```

## Options

```
      --addr string                            IPaddress:Port or :Port to bind server to. (default "localhost:8080")
      --auth-proxy string                      A program to use to create the backend from the auth.
      --baseurl string                         Prefix for URLs - leave blank for root.
      --cert string                            SSL PEM key (concatenation of certificate and CA certificate)
      --client-ca string                       Client certificate authority to verify clients with
      --dir-cache-time duration                Time to cache directory entries for. (default 5m0s)
      --dir-perms FileMode                     Directory permissions (default 0777)
      --disable-dir-list                       Disable HTML directory list on GET request for a directory
      --etag-hash string                       Which hash to use for the ETag, or auto or blank for off
      --file-perms FileMode                    File permissions (default 0666)
      --gid uint32                             Override the gid field set by the filesystem. (default 1000)
  -h, --help                                   help for webdav
      --htpasswd string                        htpasswd file - if not provided no authentication is done
      --key string                             SSL PEM Private key
      --max-header-bytes int                   Maximum size of request header (default 4096)
      --no-checksum                            Don't compare checksums on up/download.
      --no-modtime                             Don't read/write the modification time (can speed things up).
      --no-seek                                Don't allow seeking in files.
      --pass string                            Password for authentication.
      --poll-interval duration                 Time to wait between polling for changes. Must be smaller than dir-cache-time. Only on supported remotes. Set to 0 to disable. (default 1m0s)
      --read-only                              Mount read-only.
      --realm string                           realm for authentication (default "rclone")
      --server-read-timeout duration           Timeout for server reading data (default 1h0m0s)
      --server-write-timeout duration          Timeout for server writing data (default 1h0m0s)
      --template string                        User Specified Template.
      --uid uint32                             Override the uid field set by the filesystem. (default 1000)
      --umask int                              Override the permission bits set by the filesystem. (default 2)
      --user string                            User name for authentication.
      --vfs-cache-max-age duration             Max age of objects in the cache. (default 1h0m0s)
      --vfs-cache-max-size SizeSuffix          Max total size of objects in the cache. (default off)
      --vfs-cache-mode CacheMode               Cache mode off|minimal|writes|full (default off)
      --vfs-cache-poll-interval duration       Interval to poll the cache for stale objects. (default 1m0s)
      --vfs-case-insensitive                   If a file name not found, find a case insensitive match.
      --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
      --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)
      --vfs-read-wait duration                 Time to wait for in-sequence read before seeking. (default 20ms)
      --vfs-write-wait duration                Time to wait for in-sequence write before giving error. (default 1s)
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone serve](https://rclone.org/commands/rclone_serve/)	 - Serve a remote over a protocol.

# rclone settier

Changes storage class/tier of objects in remote.

## Synopsis


rclone settier changes storage tier or class at remote if supported.
Few cloud storage services provides different storage classes on objects,
for example AWS S3 and Glacier, Azure Blob storage - Hot, Cool and Archive,
Google Cloud Storage, Regional Storage, Nearline, Coldline etc.

Note that, certain tier changes make objects not available to access immediately.
For example tiering to archive in azure blob storage makes objects in frozen state,
user can restore by setting tier to Hot/Cool, similarly S3 to Glacier makes object
inaccessible.true

You can use it to tier single object

    rclone settier Cool remote:path/file

Or use rclone filters to set tier on only specific files

	rclone --include "*.txt" settier Hot remote:path/dir

Or just provide remote directory and all files in directory will be tiered

    rclone settier tier remote:path/dir


```
rclone settier tier remote:path [flags]
```

## Options

```
  -h, --help   help for settier
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone touch

Create new file or change file modification time.

## Synopsis


Set the modification time on object(s) as specified by remote:path to
have the current time.

If remote:path does not exist then a zero sized object will be created
unless the --no-create flag is provided.

If --timestamp is used then it will set the modification time to that
time instead of the current time. Times may be specified as one of:

- 'YYMMDD' - eg. 17.10.30
- 'YYYY-MM-DDTHH:MM:SS' - eg. 2006-01-02T15:04:05

Note that --timestamp is in UTC if you want local time then add the
--localtime flag.


```
rclone touch remote:path [flags]
```

## Options

```
  -h, --help               help for touch
      --localtime          Use localtime for timestamp, not UTC.
  -C, --no-create          Do not create the file if it does not exist.
  -t, --timestamp string   Use specified time instead of the current time of day.
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.

# rclone tree

List the contents of the remote in a tree like fashion.

## Synopsis


rclone tree lists the contents of a remote in a similar way to the
unix tree command.

For example

    $ rclone tree remote:path
    /
    ├── file1
    ├── file2
    ├── file3
    └── subdir
        ├── file4
        └── file5
    
    1 directories, 5 files

You can use any of the filtering options with the tree command (eg
--include and --exclude).  You can also use --fast-list.

The tree command has many options for controlling the listing which
are compatible with the tree command.  Note that not all of them have
short options as they conflict with rclone's short options.


```
rclone tree remote:path [flags]
```

## Options

```
  -a, --all             All files are listed (list . files too).
  -C, --color           Turn colorization on always.
  -d, --dirs-only       List directories only.
      --dirsfirst       List directories before files (-U disables).
      --full-path       Print the full path prefix for each file.
  -h, --help            help for tree
      --human           Print the size in a more human readable way.
      --level int       Descend only level directories deep.
  -D, --modtime         Print the date of last modification.
  -i, --noindent        Don't print indentation lines.
      --noreport        Turn off file/directory count at end of tree listing.
  -o, --output string   Output to file instead of stdout.
  -p, --protections     Print the protections for each file.
  -Q, --quote           Quote filenames with double quotes.
  -s, --size            Print the size in bytes of each file.
      --sort string     Select sort: name,version,size,mtime,ctime.
      --sort-ctime      Sort files by last status change time.
  -t, --sort-modtime    Sort files by last modification time.
  -r, --sort-reverse    Reverse the order of the sort.
  -U, --unsorted        Leave files unsorted.
      --version         Sort files alphanumerically by version.
```

See the [global flags page](https://rclone.org/flags/) for global options not listed here.

## SEE ALSO

* [rclone](https://rclone.org/commands/rclone/)	 - Show help for rclone commands, flags and backends.


Copying single files
--------------------

rclone normally syncs or copies directories.  However, if the source
remote points to a file, rclone will just copy that file.  The
destination remote must point to a directory - rclone will give the
error `Failed to create file system for "remote:file": is a file not a
directory` if it isn't.

For example, suppose you have a remote with a file in called
`test.jpg`, then you could copy just that file like this

    rclone copy remote:test.jpg /tmp/download

The file `test.jpg` will be placed inside `/tmp/download`.

This is equivalent to specifying

    rclone copy --files-from /tmp/files remote: /tmp/download

Where `/tmp/files` contains the single line

    test.jpg

It is recommended to use `copy` when copying individual files, not `sync`.
They have pretty much the same effect but `copy` will use a lot less
memory.

Syntax of remote paths
----------------------

The syntax of the paths passed to the rclone command are as follows.

### /path/to/dir

This refers to the local file system.

On Windows only `\` may be used instead of `/` in local paths
**only**, non local paths must use `/`.

These paths needn't start with a leading `/` - if they don't then they
will be relative to the current directory.

### remote:path/to/dir

This refers to a directory `path/to/dir` on `remote:` as defined in
the config file (configured with `rclone config`).

### remote:/path/to/dir

On most backends this is refers to the same directory as
`remote:path/to/dir` and that format should be preferred.  On a very
small number of remotes (FTP, SFTP, Dropbox for business) this will
refer to a different directory.  On these, paths without a leading `/`
will refer to your "home" directory and paths with a leading `/` will
refer to the root.

### :backend:path/to/dir

This is an advanced form for creating remotes on the fly.  `backend`
should be the name or prefix of a backend (the `type` in the config
file) and all the configuration for the backend should be provided on
the command line (or in environment variables).

Here are some examples:

    rclone lsd --http-url https://pub.rclone.org :http:

To list all the directories in the root of `https://pub.rclone.org/`.

    rclone lsf --http-url https://example.com :http:path/to/dir

To list files and directories in `https://example.com/path/to/dir/`

    rclone copy --http-url https://example.com :http:path/to/dir /tmp/dir

To copy files and directories in `https://example.com/path/to/dir` to `/tmp/dir`.

    rclone copy --sftp-host example.com :sftp:path/to/dir /tmp/dir

To copy files and directories from `example.com` in the relative
directory `path/to/dir` to `/tmp/dir` using sftp.

Quoting and the shell
---------------------

When you are typing commands to your computer you are using something
called the command line shell.  This interprets various characters in
an OS specific way.

Here are some gotchas which may help users unfamiliar with the shell rules

### Linux / OSX ###

If your names have spaces or shell metacharacters (eg `*`, `?`, `$`,
`'`, `"` etc) then you must quote them.  Use single quotes `'` by default.

    rclone copy 'Important files?' remote:backup

If you want to send a `'` you will need to use `"`, eg

    rclone copy "O'Reilly Reviews" remote:backup

The rules for quoting metacharacters are complicated and if you want
the full details you'll have to consult the manual page for your
shell.

### Windows ###

If your names have spaces in you need to put them in `"`, eg

    rclone copy "E:\folder name\folder name\folder name" remote:backup

If you are using the root directory on its own then don't quote it
(see [#464](https://github.com/rclone/rclone/issues/464) for why), eg

    rclone copy E:\ remote:backup

Copying files or directories with `:` in the names
--------------------------------------------------

rclone uses `:` to mark a remote name.  This is, however, a valid
filename component in non-Windows OSes.  The remote name parser will
only search for a `:` up to the first `/` so if you need to act on a
file or directory like this then use the full path starting with a
`/`, or use `./` as a current directory prefix.

So to sync a directory called `sync:me` to a remote called `remote:` use

    rclone sync ./sync:me remote:path

or

    rclone sync /full/path/to/sync:me remote:path

Server Side Copy
----------------

Most remotes (but not all - see [the
overview](https://rclone.org/overview/#optional-features)) support server side copy.

This means if you want to copy one folder to another then rclone won't
download all the files and re-upload them; it will instruct the server
to copy them in place.

Eg

    rclone copy s3:oldbucket s3:newbucket

Will copy the contents of `oldbucket` to `newbucket` without
downloading and re-uploading.

Remotes which don't support server side copy **will** download and
re-upload in this case.

Server side copies are used with `sync` and `copy` and will be
identified in the log when using the `-v` flag.  The `move` command
may also use them if remote doesn't support server side move directly.
This is done by issuing a server side copy then a delete which is much
quicker than a download and re-upload.

Server side copies will only be attempted if the remote names are the
same.

This can be used when scripting to make aged backups efficiently, eg

    rclone sync remote:current-backup remote:previous-backup
    rclone sync /path/to/files remote:current-backup

Options
-------

Rclone has a number of options to control its behaviour.

Options that take parameters can have the values passed in two ways,
`--option=value` or `--option value`. However boolean (true/false)
options behave slightly differently to the other options in that
`--boolean` sets the option to `true` and the absence of the flag sets
it to `false`.  It is also possible to specify `--boolean=false` or
`--boolean=true`.  Note that `--boolean false` is not valid - this is
parsed as `--boolean` and the `false` is parsed as an extra command
line argument for rclone.

Options which use TIME use the go time parser.  A duration string is a
possibly signed sequence of decimal numbers, each with optional
fraction and a unit suffix, such as "300ms", "-1.5h" or "2h45m". Valid
time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

Options which use SIZE use kByte by default.  However, a suffix of `b`
for bytes, `k` for kBytes, `M` for MBytes, `G` for GBytes, `T` for
TBytes and `P` for PBytes may be used.  These are the binary units, eg
1, 2\*\*10, 2\*\*20, 2\*\*30 respectively.

### --backup-dir=DIR ###

When using `sync`, `copy` or `move` any files which would have been
overwritten or deleted are moved in their original hierarchy into this
directory.

If `--suffix` is set, then the moved files will have the suffix added
to them.  If there is a file with the same path (after the suffix has
been added) in DIR, then it will be overwritten.

The remote in use must support server side move or copy and you must
use the same remote as the destination of the sync.  The backup
directory must not overlap the destination directory.

For example

    rclone sync /path/to/local remote:current --backup-dir remote:old

will sync `/path/to/local` to `remote:current`, but for any files
which would have been updated or deleted will be stored in
`remote:old`.

If running rclone from a script you might want to use today's date as
the directory name passed to `--backup-dir` to store the old files, or
you might want to pass `--suffix` with today's date.

See `--compare-dest` and `--copy-dest`.

### --bind string ###

Local address to bind to for outgoing connections.  This can be an
IPv4 address (1.2.3.4), an IPv6 address (1234::789A) or host name.  If
the host name doesn't resolve or resolves to more than one IP address
it will give an error.

### --bwlimit=BANDWIDTH_SPEC ###

This option controls the bandwidth limit. Limits can be specified
in two ways: As a single limit, or as a timetable.

Single limits last for the duration of the session. To use a single limit,
specify the desired bandwidth in kBytes/s, or use a suffix b|k|M|G.  The
default is `0` which means to not limit bandwidth.

For example, to limit bandwidth usage to 10 MBytes/s use `--bwlimit 10M`

It is also possible to specify a "timetable" of limits, which will cause
certain limits to be applied at certain times. To specify a timetable, format your
entries as `WEEKDAY-HH:MM,BANDWIDTH WEEKDAY-HH:MM,BANDWIDTH...` where:
`WEEKDAY` is optional element.
It could be written as whole world or only using 3 first characters.
`HH:MM` is an hour from 00:00 to 23:59.

An example of a typical timetable to avoid link saturation during daytime
working hours could be:

`--bwlimit "08:00,512 12:00,10M 13:00,512 18:00,30M 23:00,off"`

In this example, the transfer bandwidth will be every day set to 512kBytes/sec at 8am.
At noon, it will raise to 10Mbytes/s, and drop back to 512kBytes/sec at 1pm.
At 6pm, the bandwidth limit will be set to 30MBytes/s, and at 11pm it will be
completely disabled (full speed). Anything between 11pm and 8am will remain
unlimited.

An example of timetable with `WEEKDAY` could be:

`--bwlimit "Mon-00:00,512 Fri-23:59,10M Sat-10:00,1M Sun-20:00,off"`

It mean that, the transfer bandwidth will be set to 512kBytes/sec on Monday.
It will raise to 10Mbytes/s before the end of Friday. 
At 10:00 on Sunday it will be set to 1Mbyte/s.
From 20:00 at Sunday will be unlimited.

Timeslots without weekday are extended to whole week.
So this one example:

`--bwlimit "Mon-00:00,512 12:00,1M Sun-20:00,off"`

Is equal to this:

`--bwlimit "Mon-00:00,512Mon-12:00,1M Tue-12:00,1M Wed-12:00,1M Thu-12:00,1M Fri-12:00,1M Sat-12:00,1M Sun-12:00,1M Sun-20:00,off"`

Bandwidth limits only apply to the data transfer. They don't apply to the
bandwidth of the directory listings etc.

Note that the units are Bytes/s, not Bits/s.  Typically connections are
measured in Bits/s - to convert divide by 8.  For example, let's say
you have a 10 Mbit/s connection and you wish rclone to use half of it
- 5 Mbit/s.  This is 5/8 = 0.625MByte/s so you would use a `--bwlimit
0.625M` parameter for rclone.

On Unix systems (Linux, macOS, …) the bandwidth limiter can be toggled by
sending a `SIGUSR2` signal to rclone. This allows to remove the limitations
of a long running rclone transfer and to restore it back to the value specified
with `--bwlimit` quickly when needed. Assuming there is only one rclone instance
running, you can toggle the limiter like this:

    kill -SIGUSR2 $(pidof rclone)

If you configure rclone with a [remote control](/rc) then you can use
change the bwlimit dynamically:

    rclone rc core/bwlimit rate=1M

### --buffer-size=SIZE ###

Use this sized buffer to speed up file transfers.  Each `--transfer`
will use this much memory for buffering.

When using `mount` or `cmount` each open file descriptor will use this much
memory for buffering.
See the [mount](https://rclone.org/commands/rclone_mount/#file-buffering) documentation for more details.

Set to `0` to disable the buffering for the minimum memory usage.

Note that the memory allocation of the buffers is influenced by the
[--use-mmap](#use-mmap) flag.

### --check-first ###

If this flag is set then in a `sync`, `copy` or `move`, rclone will do
all the checks to see whether files need to be transferred before
doing any of the transfers. Normally rclone would start running
transfers as soon as possible.

This flag can be useful on IO limited systems where transfers
interfere with checking.

Using this flag can use more memory as it effectively sets
`--max-backlog` to infinite. This means that all the info on the
objects to transfer is held in memory before the transfers start.

### --checkers=N ###

The number of checkers to run in parallel.  Checkers do the equality
checking of files during a sync.  For some storage systems (eg S3,
Swift, Dropbox) this can take a significant amount of time so they are
run in parallel.

The default is to run 8 checkers in parallel.

### -c, --checksum ###

Normally rclone will look at modification time and size of files to
see if they are equal.  If you set this flag then rclone will check
the file hash and size to determine if files are equal.

This is useful when the remote doesn't support setting modified time
and a more accurate sync is desired than just checking the file size.

This is very useful when transferring between remotes which store the
same hash type on the object, eg Drive and Swift. For details of which
remotes support which hash type see the table in the [overview
section](https://rclone.org/overview/).

Eg `rclone --checksum sync s3:/bucket swift:/bucket` would run much
quicker than without the `--checksum` flag.

When using this flag, rclone won't update mtimes of remote files if
they are incorrect as it would normally.

### --compare-dest=DIR ###

When using `sync`, `copy` or `move` DIR is checked in addition to the 
destination for files. If a file identical to the source is found that 
file is NOT copied from source. This is useful to copy just files that 
have changed since the last backup.

You must use the same remote as the destination of the sync.  The 
compare directory must not overlap the destination directory.

See `--copy-dest` and `--backup-dir`.

### --config=CONFIG_FILE ###

Specify the location of the rclone config file.

Normally the config file is in your home directory as a file called
`.config/rclone/rclone.conf` (or `.rclone.conf` if created with an
older version). If `$XDG_CONFIG_HOME` is set it will be at
`$XDG_CONFIG_HOME/rclone/rclone.conf`.

If there is a file `rclone.conf` in the same directory as the rclone
executable it will be preferred. This file must be created manually
for Rclone to use it, it will never be created automatically.

If you run `rclone config file` you will see where the default
location is for you.

Use this flag to override the config location, eg `rclone
--config=".myconfig" .config`.

### --contimeout=TIME ###

Set the connection timeout. This should be in go time format which
looks like `5s` for 5 seconds, `10m` for 10 minutes, or `3h30m`.

The connection timeout is the amount of time rclone will wait for a
connection to go through to a remote object storage system.  It is
`1m` by default.

### --copy-dest=DIR ###

When using `sync`, `copy` or `move` DIR is checked in addition to the 
destination for files. If a file identical to the source is found that 
file is server side copied from DIR to the destination. This is useful 
for incremental backup.

The remote in use must support server side copy and you must
use the same remote as the destination of the sync.  The compare
directory must not overlap the destination directory.

See `--compare-dest` and `--backup-dir`.

### --dedupe-mode MODE ###

Mode to run dedupe command in.  One of `interactive`, `skip`, `first`, `newest`, `oldest`, `rename`.  The default is `interactive`.  See the dedupe command for more information as to what these options mean.

### --disable FEATURE,FEATURE,... ###

This disables a comma separated list of optional features. For example
to disable server side move and server side copy use:

    --disable move,copy

The features can be put in any case.

To see a list of which features can be disabled use:

    --disable help

See the overview [features](https://rclone.org/overview/#features) and
[optional features](https://rclone.org/overview/#optional-features) to get an idea of
which feature does what.

This flag can be useful for debugging and in exceptional circumstances
(eg Google Drive limiting the total volume of Server Side Copies to
100GB/day).

### -n, --dry-run ###

Do a trial run with no permanent changes.  Use this to see what rclone
would do without actually doing it.  Useful when setting up the `sync`
command which deletes files in the destination.

### --expect-continue-timeout=TIME ###

This specifies the amount of time to wait for a server's first
response headers after fully writing the request headers if the
request has an "Expect: 100-continue" header. Not all backends support
using this.

Zero means no timeout and causes the body to be sent immediately,
without waiting for the server to approve.  This time does not include
the time to send the request header.

The default is `1s`.  Set to `0` to disable.

### --error-on-no-transfer ###

By default, rclone will exit with return code 0 if there were no errors.

This option allows rclone to return exit code 9 if no files were transferred
between the source and destination. This allows using rclone in scripts, and
triggering follow-on actions if data was copied, or skipping if not.

NB: Enabling this option turns a usually non-fatal error into a potentially
fatal one - please check and adjust your scripts accordingly!

### --header ###

Add an HTTP header for all transactions. The flag can be repeated to
add multiple headers.

If you want to add headers only for uploads use `--header-upload` and
if you want to add headers only for downloads use `--header-download`.

This flag is supported for all HTTP based backends even those not
supported by `--header-upload` and `--header-download` so may be used
as a workaround for those with care.

```
rclone ls remote:test --header "X-Rclone: Foo" --header "X-LetMeIn: Yes"
```

### --header-download ###

Add an HTTP header for all download transactions. The flag can be repeated to
add multiple headers.

```
rclone sync s3:test/src ~/dst --header-download "X-Amz-Meta-Test: Foo" --header-download "X-Amz-Meta-Test2: Bar"
```

See the GitHub issue [here](https://github.com/rclone/rclone/issues/59) for
currently supported backends.

### --header-upload ###

Add an HTTP header for all upload transactions. The flag can be repeated to add
multiple headers.

```
rclone sync ~/src s3:test/dst --header-upload "Content-Disposition: attachment; filename='cool.html'" --header-upload "X-Amz-Meta-Test: FooBar"
```

See the GitHub issue [here](https://github.com/rclone/rclone/issues/59) for
currently supported backends.

### --ignore-case-sync ###

Using this option will cause rclone to ignore the case of the files 
when synchronizing so files will not be copied/synced when the
existing filenames are the same, even if the casing is different.

### --ignore-checksum ###

Normally rclone will check that the checksums of transferred files
match, and give an error "corrupted on transfer" if they don't.

You can use this option to skip that check.  You should only use it if
you have had the "corrupted on transfer" error message and you are
sure you might want to transfer potentially corrupted data.

### --ignore-existing ###

Using this option will make rclone unconditionally skip all files
that exist on the destination, no matter the content of these files.

While this isn't a generally recommended option, it can be useful
in cases where your files change due to encryption. However, it cannot
correct partial transfers in case a transfer was interrupted.

### --ignore-size ###

Normally rclone will look at modification time and size of files to
see if they are equal.  If you set this flag then rclone will check
only the modification time.  If `--checksum` is set then it only
checks the checksum.

It will also cause rclone to skip verifying the sizes are the same
after transfer.

This can be useful for transferring files to and from OneDrive which
occasionally misreports the size of image files (see
[#399](https://github.com/rclone/rclone/issues/399) for more info).

### -I, --ignore-times ###

Using this option will cause rclone to unconditionally upload all
files regardless of the state of files on the destination.

Normally rclone would skip any files that have the same
modification time and are the same size (or have the same checksum if
using `--checksum`).

### --immutable ###

Treat source and destination files as immutable and disallow
modification.

With this option set, files will be created and deleted as requested,
but existing files will never be updated.  If an existing file does
not match between the source and destination, rclone will give the error
`Source and destination exist but do not match: immutable file modified`.

Note that only commands which transfer files (e.g. `sync`, `copy`,
`move`) are affected by this behavior, and only modification is
disallowed.  Files may still be deleted explicitly (e.g. `delete`,
`purge`) or implicitly (e.g. `sync`, `move`).  Use `copy --immutable`
if it is desired to avoid deletion as well as modification.

This can be useful as an additional layer of protection for immutable
or append-only data sets (notably backup archives), where modification
implies corruption and should not be propagated.

## --leave-root ###

During rmdirs it will not remove root directory, even if it's empty.

### --log-file=FILE ###

Log all of rclone's output to FILE.  This is not active by default.
This can be useful for tracking down problems with syncs in
combination with the `-v` flag.  See the [Logging section](#logging)
for more info.

Note that if you are using the `logrotate` program to manage rclone's
logs, then you should use the `copytruncate` option as rclone doesn't
have a signal to rotate logs.

### --log-format LIST ###

Comma separated list of log format options. `date`, `time`, `microseconds`, `longfile`, `shortfile`, `UTC`.  The default is "`date`,`time`". 

### --log-level LEVEL ###

This sets the log level for rclone.  The default log level is `NOTICE`.

`DEBUG` is equivalent to `-vv`. It outputs lots of debug info - useful
for bug reports and really finding out what rclone is doing.

`INFO` is equivalent to `-v`. It outputs information about each transfer
and prints stats once a minute by default.

`NOTICE` is the default log level if no logging flags are supplied. It
outputs very little when things are working normally. It outputs
warnings and significant events.

`ERROR` is equivalent to `-q`. It only outputs error messages.

### --use-json-log ###

This switches the log format to JSON for rclone. The fields of json log 
are level, msg, source, time.

### --low-level-retries NUMBER ###

This controls the number of low level retries rclone does.

A low level retry is used to retry a failing operation - typically one
HTTP request.  This might be uploading a chunk of a big file for
example.  You will see low level retries in the log with the `-v`
flag.

This shouldn't need to be changed from the default in normal operations.
However, if you get a lot of low level retries you may wish
to reduce the value so rclone moves on to a high level retry (see the
`--retries` flag) quicker.

Disable low level retries with `--low-level-retries 1`.

### --max-backlog=N ###

This is the maximum allowable backlog of files in a sync/copy/move
queued for being checked or transferred.

This can be set arbitrarily large.  It will only use memory when the
queue is in use.  Note that it will use in the order of N kB of memory
when the backlog is in use.

Setting this large allows rclone to calculate how many files are
pending more accurately, give a more accurate estimated finish
time and make `--order-by` work more accurately.

Setting this small will make rclone more synchronous to the listings
of the remote which may be desirable.

Setting this to a negative number will make the backlog as large as
possible.

### --max-delete=N ###

This tells rclone not to delete more than N files.  If that limit is
exceeded then a fatal error will be generated and rclone will stop the
operation in progress.

### --max-depth=N ###

This modifies the recursion depth for all the commands except purge.

So if you do `rclone --max-depth 1 ls remote:path` you will see only
the files in the top level directory.  Using `--max-depth 2` means you
will see all the files in first two directory levels and so on.

For historical reasons the `lsd` command defaults to using a
`--max-depth` of 1 - you can override this with the command line flag.

You can use this command to disable recursion (with `--max-depth 1`).

Note that if you use this with `sync` and `--delete-excluded` the
files not recursed through are considered excluded and will be deleted
on the destination.  Test first with `--dry-run` if you are not sure
what will happen.

### --max-duration=TIME ###

Rclone will stop scheduling new transfers when it has run for the
duration specified.

Defaults to off.

When the limit is reached any existing transfers will complete.

Rclone won't exit with an error if the transfer limit is reached.

### --max-transfer=SIZE ###

Rclone will stop transferring when it has reached the size specified.
Defaults to off.

When the limit is reached all transfers will stop immediately.

Rclone will exit with exit code 8 if the transfer limit is reached.

### --cutoff-mode=hard|soft|cautious ###

This modifies the behavior of `--max-transfer`
Defaults to `--cutoff-mode=hard`.

Specifying `--cutoff-mode=hard` will stop transferring immediately
when Rclone reaches the limit.

Specifying `--cutoff-mode=soft` will stop starting new transfers
when Rclone reaches the limit.

Specifying `--cutoff-mode=cautious` will try to prevent Rclone
from reaching the limit.

### --modify-window=TIME ###

When checking whether a file has been modified, this is the maximum
allowed time difference that a file can have and still be considered
equivalent.

The default is `1ns` unless this is overridden by a remote.  For
example OS X only stores modification times to the nearest second so
if you are reading and writing to an OS X filing system this will be
`1s` by default.

This command line flag allows you to override that computed default.

### --multi-thread-cutoff=SIZE ###

When downloading files to the local backend above this size, rclone
will use multiple threads to download the file (default 250M).

Rclone preallocates the file (using `fallocate(FALLOC_FL_KEEP_SIZE)`
on unix or `NTSetInformationFile` on Windows both of which takes no
time) then each thread writes directly into the file at the correct
place.  This means that rclone won't create fragmented or sparse files
and there won't be any assembly time at the end of the transfer.

The number of threads used to download is controlled by
`--multi-thread-streams`.

Use `-vv` if you wish to see info about the threads.

This will work with the `sync`/`copy`/`move` commands and friends
`copyto`/`moveto`.  Multi thread downloads will be used with `rclone
mount` and `rclone serve` if `--vfs-cache-mode` is set to `writes` or
above.

**NB** that this **only** works for a local destination but will work
with any source.

**NB** that multi thread copies are disabled for local to local copies
as they are faster without unless `--multi-thread-streams` is set
explicitly.

**NB** on Windows using multi-thread downloads will cause the
resulting files to be [sparse](https://en.wikipedia.org/wiki/Sparse_file).
Use `--local-no-sparse` to disable sparse files (which may cause long
delays at the start of downloads) or disable multi-thread downloads
with `--multi-thread-streams 0`

### --multi-thread-streams=N ###

When using multi thread downloads (see above `--multi-thread-cutoff`)
this sets the maximum number of streams to use.  Set to `0` to disable
multi thread downloads (Default 4).

Exactly how many streams rclone uses for the download depends on the
size of the file. To calculate the number of download streams Rclone
divides the size of the file by the `--multi-thread-cutoff` and rounds
up, up to the maximum set with `--multi-thread-streams`.

So if `--multi-thread-cutoff 250MB` and `--multi-thread-streams 4` are
in effect (the defaults):

- 0MB..250MB files will be downloaded with 1 stream
- 250MB..500MB files will be downloaded with 2 streams
- 500MB..750MB files will be downloaded with 3 streams
- 750MB+ files will be downloaded with 4 streams

### --no-check-dest ###

The `--no-check-dest` can be used with `move` or `copy` and it causes
rclone not to check the destination at all when copying files.

This means that:

- the destination is not listed minimising the API calls
- files are always transferred
- this can cause duplicates on remotes which allow it (eg Google Drive)
- `--retries 1` is recommended otherwise you'll transfer everything again on a retry

This flag is useful to minimise the transactions if you know that none
of the files are on the destination.

This is a specialized flag which should be ignored by most users!

### --no-gzip-encoding ###

Don't set `Accept-Encoding: gzip`.  This means that rclone won't ask
the server for compressed files automatically. Useful if you've set
the server to return files with `Content-Encoding: gzip` but you
uploaded compressed files.

There is no need to set this in normal operation, and doing so will
decrease the network transfer efficiency of rclone.

### --no-traverse ###

The `--no-traverse` flag controls whether the destination file system
is traversed when using the `copy` or `move` commands.
`--no-traverse` is not compatible with `sync` and will be ignored if
you supply it with `sync`.

If you are only copying a small number of files (or are filtering most
of the files) and/or have a large number of files on the destination
then `--no-traverse` will stop rclone listing the destination and save
time.

However, if you are copying a large number of files, especially if you
are doing a copy where lots of the files under consideration haven't
changed and won't need copying then you shouldn't use `--no-traverse`.

See [rclone copy](https://rclone.org/commands/rclone_copy/) for an example of how to use it.

### --no-unicode-normalization ###

Don't normalize unicode characters in filenames during the sync routine.

Sometimes, an operating system will store filenames containing unicode
parts in their decomposed form (particularly macOS). Some cloud storage
systems will then recompose the unicode, resulting in duplicate files if
the data is ever copied back to a local filesystem.

Using this flag will disable that functionality, treating each unicode
character as unique. For example, by default é and é will be normalized
into the same character. With `--no-unicode-normalization` they will be
treated as unique characters.

### --no-update-modtime ###

When using this flag, rclone won't update modification times of remote
files if they are incorrect as it would normally.

This can be used if the remote is being synced with another tool also
(eg the Google Drive client).

### --order-by string ###

The `--order-by` flag controls the order in which files in the backlog
are processed in `rclone sync`, `rclone copy` and `rclone move`.

The order by string is constructed like this.  The first part
describes what aspect is being measured:

- `size` - order by the size of the files
- `name` - order by the full path of the files
- `modtime` - order by the modification date of the files

This can have a modifier appended with a comma:

- `ascending` or `asc` - order so that the smallest (or oldest) is processed first
- `descending` or `desc` - order so that the largest (or newest) is processed first
- `mixed` - order so that the smallest is processed first for some threads and the largest for others

If the modifier is `mixed` then it can have an optional percentage
(which defaults to `50`), eg `size,mixed,25` which means that 25% of
the threads should be taking the smallest items and 75% the
largest. The threads which take the smallest first will always take
the smallest first and likewise the largest first threads. The `mixed`
mode can be useful to minimise the transfer time when you are
transferring a mixture of large and small files - the large files are
guaranteed upload threads and bandwidth and the small files will be
processed continuously.

If no modifier is supplied then the order is `ascending`.

For example

- `--order-by size,desc` - send the largest files first
- `--order-by modtime,ascending` - send the oldest files first
- `--order-by name` - send the files with alphabetically by path first

If the `--order-by` flag is not supplied or it is supplied with an
empty string then the default ordering will be used which is as
scanned.  With `--checkers 1` this is mostly alphabetical, however
with the default `--checkers 8` it is somewhat random.

#### Limitations

The `--order-by` flag does not do a separate pass over the data.  This
means that it may transfer some files out of the order specified if

- there are no files in the backlog or the source has not been fully scanned yet
- there are more than [--max-backlog](#max-backlog-n) files in the backlog

Rclone will do its best to transfer the best file it has so in
practice this should not cause a problem.  Think of `--order-by` as
being more of a best efforts flag rather than a perfect ordering.

### --password-command SpaceSepList ###

This flag supplies a program which should supply the config password
when run. This is an alternative to rclone prompting for the password
or setting the `RCLONE_CONFIG_PASS` variable.

The argument to this should be a command with a space separated list
of arguments. If one of the arguments has a space in then enclose it
in `"`, if you want a literal `"` in an argument then enclose the
argument in `"` and double the `"`. See [CSV encoding](https://godoc.org/encoding/csv)
for more info.

Eg

    --password-command echo hello
    --password-command echo "hello with space"
    --password-command echo "hello with ""quotes"" and space"

See the [Configuration Encryption](#configuration-encryption) for more info.

See a [Windows PowerShell example on the Wiki](https://github.com/rclone/rclone/wiki/Windows-Powershell-use-rclone-password-command-for-Config-file-password).

### -P, --progress ###

This flag makes rclone update the stats in a static block in the
terminal providing a realtime overview of the transfer.

Any log messages will scroll above the static block.  Log messages
will push the static block down to the bottom of the terminal where it
will stay.

Normally this is updated every 500mS but this period can be overridden
with the `--stats` flag.

This can be used with the `--stats-one-line` flag for a simpler
display.

Note: On Windows until [this bug](https://github.com/Azure/go-ansiterm/issues/26)
is fixed all non-ASCII characters will be replaced with `.` when
`--progress` is in use.

### -q, --quiet ###

This flag will limit rclone's output to error messages only.

### --retries int ###

Retry the entire sync if it fails this many times it fails (default 3).

Some remotes can be unreliable and a few retries help pick up the
files which didn't get transferred because of errors.

Disable retries with `--retries 1`.

### --retries-sleep=TIME ###

This sets the interval between each retry specified by `--retries` 

The default is `0`. Use `0` to disable.

### --size-only ###

Normally rclone will look at modification time and size of files to
see if they are equal.  If you set this flag then rclone will check
only the size.

This can be useful transferring files from Dropbox which have been
modified by the desktop sync client which doesn't set checksums of
modification times in the same way as rclone.

### --stats=TIME ###

Commands which transfer data (`sync`, `copy`, `copyto`, `move`,
`moveto`) will print data transfer stats at regular intervals to show
their progress.

This sets the interval.

The default is `1m`. Use `0` to disable.

If you set the stats interval then all commands can show stats.  This
can be useful when running other commands, `check` or `mount` for
example.

Stats are logged at `INFO` level by default which means they won't
show at default log level `NOTICE`.  Use `--stats-log-level NOTICE` or
`-v` to make them show.  See the [Logging section](#logging) for more
info on log levels.

Note that on macOS you can send a SIGINFO (which is normally ctrl-T in
the terminal) to make the stats print immediately.

### --stats-file-name-length integer ###
By default, the `--stats` output will truncate file names and paths longer 
than 40 characters.  This is equivalent to providing 
`--stats-file-name-length 40`. Use `--stats-file-name-length 0` to disable 
any truncation of file names printed by stats.

### --stats-log-level string ###

Log level to show `--stats` output at.  This can be `DEBUG`, `INFO`,
`NOTICE`, or `ERROR`.  The default is `INFO`.  This means at the
default level of logging which is `NOTICE` the stats won't show - if
you want them to then use `--stats-log-level NOTICE`.  See the [Logging
section](#logging) for more info on log levels.

### --stats-one-line ###

When this is specified, rclone condenses the stats into a single line
showing the most important stats only.

### --stats-one-line-date ###

When this is specified, rclone enables the single-line stats and prepends
the display with a date string. The default is `2006/01/02 15:04:05 - `

### --stats-one-line-date-format ###

When this is specified, rclone enables the single-line stats and prepends
the display with a user-supplied date string. The date string MUST be
enclosed in quotes. Follow [golang specs](https://golang.org/pkg/time/#Time.Format) for
date formatting syntax.

### --stats-unit=bits|bytes ###

By default, data transfer rates will be printed in bytes/second.

This option allows the data rate to be printed in bits/second.

Data transfer volume will still be reported in bytes.

The rate is reported as a binary unit, not SI unit. So 1 Mbit/s
equals 1,048,576 bits/s and not 1,000,000 bits/s.

The default is `bytes`.

### --suffix=SUFFIX ###

When using `sync`, `copy` or `move` any files which would have been
overwritten or deleted will have the suffix added to them.  If there 
is a file with the same path (after the suffix has been added), then 
it will be overwritten.

The remote in use must support server side move or copy and you must
use the same remote as the destination of the sync.

This is for use with files to add the suffix in the current directory 
or with `--backup-dir`. See `--backup-dir` for more info.

For example

    rclone sync /path/to/local/file remote:current --suffix .bak

will sync `/path/to/local` to `remote:current`, but for any files
which would have been updated or deleted have .bak added.

### --suffix-keep-extension ###

When using `--suffix`, setting this causes rclone put the SUFFIX
before the extension of the files that it backs up rather than after.

So let's say we had `--suffix -2019-01-01`, without the flag `file.txt`
would be backed up to `file.txt-2019-01-01` and with the flag it would
be backed up to `file-2019-01-01.txt`.  This can be helpful to make
sure the suffixed files can still be opened.

### --syslog ###

On capable OSes (not Windows or Plan9) send all log output to syslog.

This can be useful for running rclone in a script or `rclone mount`.

### --syslog-facility string ###

If using `--syslog` this sets the syslog facility (eg `KERN`, `USER`).
See `man syslog` for a list of possible facilities.  The default
facility is `DAEMON`.

### --tpslimit float ###

Limit HTTP transactions per second to this. Default is 0 which is used
to mean unlimited transactions per second.

For example to limit rclone to 10 HTTP transactions per second use
`--tpslimit 10`, or to 1 transaction every 2 seconds use `--tpslimit
0.5`.

Use this when the number of transactions per second from rclone is
causing a problem with the cloud storage provider (eg getting you
banned or rate limited).

This can be very useful for `rclone mount` to control the behaviour of
applications using it.

See also `--tpslimit-burst`.

### --tpslimit-burst int ###

Max burst of transactions for `--tpslimit` (default `1`).

Normally `--tpslimit` will do exactly the number of transaction per
second specified.  However if you supply `--tps-burst` then rclone can
save up some transactions from when it was idle giving a burst of up
to the parameter supplied.

For example if you provide `--tpslimit-burst 10` then if rclone has
been idle for more than 10*`--tpslimit` then it can do 10 transactions
very quickly before they are limited again.

This may be used to increase performance of `--tpslimit` without
changing the long term average number of transactions per second.

### --track-renames ###

By default, rclone doesn't keep track of renamed files, so if you
rename a file locally then sync it to a remote, rclone will delete the
old file on the remote and upload a new copy.

If you use this flag, and the remote supports server side copy or
server side move, and the source and destination have a compatible
hash, then this will track renames during `sync`
operations and perform renaming server-side.

Files will be matched by size and hash - if both match then a rename
will be considered.

If the destination does not support server-side copy or move, rclone
will fall back to the default behaviour and log an error level message
to the console. Note: Encrypted destinations are not supported
by `--track-renames`.

Note that `--track-renames` is incompatible with `--no-traverse` and
that it uses extra memory to keep track of all the rename candidates.

Note also that `--track-renames` is incompatible with
`--delete-before` and will select `--delete-after` instead of
`--delete-during`.

### --track-renames-strategy (hash,modtime) ###

This option changes the matching criteria for `--track-renames` to match
by any combination of modtime, hash, size. Matching by size is always enabled
no matter what option is selected here. This also means
that it enables `--track-renames` support for encrypted destinations.
If nothing is specified, the default option is matching by hashes.

### --delete-(before,during,after) ###

This option allows you to specify when files on your destination are
deleted when you sync folders.

Specifying the value `--delete-before` will delete all files present
on the destination, but not on the source *before* starting the
transfer of any new or updated files. This uses two passes through the
file systems, one for the deletions and one for the copies.

Specifying `--delete-during` will delete files while checking and
uploading files. This is the fastest option and uses the least memory.

Specifying `--delete-after` (the default value) will delay deletion of
files until all new/updated files have been successfully transferred.
The files to be deleted are collected in the copy pass then deleted
after the copy pass has completed successfully.  The files to be
deleted are held in memory so this mode may use more memory.  This is
the safest mode as it will only delete files if there have been no
errors subsequent to that.  If there have been errors before the
deletions start then you will get the message `not deleting files as
there were IO errors`.

### --fast-list ###

When doing anything which involves a directory listing (eg `sync`,
`copy`, `ls` - in fact nearly every command), rclone normally lists a
directory and processes it before using more directory lists to
process any subdirectories.  This can be parallelised and works very
quickly using the least amount of memory.

However, some remotes have a way of listing all files beneath a
directory in one (or a small number) of transactions.  These tend to
be the bucket based remotes (eg S3, B2, GCS, Swift, Hubic).

If you use the `--fast-list` flag then rclone will use this method for
listing directories.  This will have the following consequences for
the listing:

  * It **will** use fewer transactions (important if you pay for them)
  * It **will** use more memory.  Rclone has to load the whole listing into memory.
  * It *may* be faster because it uses fewer transactions
  * It *may* be slower because it can't be parallelized

rclone should always give identical results with and without
`--fast-list`.

If you pay for transactions and can fit your entire sync listing into
memory then `--fast-list` is recommended.  If you have a very big sync
to do then don't use `--fast-list` otherwise you will run out of
memory.

If you use `--fast-list` on a remote which doesn't support it, then
rclone will just ignore it.

### --timeout=TIME ###

This sets the IO idle timeout.  If a transfer has started but then
becomes idle for this long it is considered broken and disconnected.

The default is `5m`.  Set to `0` to disable.

### --transfers=N ###

The number of file transfers to run in parallel.  It can sometimes be
useful to set this to a smaller number if the remote is giving a lot
of timeouts or bigger if you have lots of bandwidth and a fast remote.

The default is to run 4 file transfers in parallel.

### -u, --update ###

This forces rclone to skip any files which exist on the destination
and have a modified time that is newer than the source file.

This can be useful when transferring to a remote which doesn't support
mod times directly (or when using `--use-server-modtime` to avoid extra
API calls) as it is more accurate than a `--size-only` check and faster
than using `--checksum`.

If an existing destination file has a modification time equal (within
the computed modify window precision) to the source file's, it will be
updated if the sizes are different.  If `--checksum` is set then
rclone will update the destination if the checksums differ too.

If an existing destination file is older than the source file then
it will be updated if the size or checksum differs from the source file.

On remotes which don't support mod time directly (or when using
`--use-server-modtime`) the time checked will be the uploaded time.
This means that if uploading to one of these remotes, rclone will skip
any files which exist on the destination and have an uploaded time that
is newer than the modification time of the source file.

### --use-mmap ###

If this flag is set then rclone will use anonymous memory allocated by
mmap on Unix based platforms and VirtualAlloc on Windows for its
transfer buffers (size controlled by `--buffer-size`).  Memory
allocated like this does not go on the Go heap and can be returned to
the OS immediately when it is finished with.

If this flag is not set then rclone will allocate and free the buffers
using the Go memory allocator which may use more memory as memory
pages are returned less aggressively to the OS.

It is possible this does not work well on all platforms so it is
disabled by default; in the future it may be enabled by default.

### --use-server-modtime ###

Some object-store backends (e.g, Swift, S3) do not preserve file modification
times (modtime). On these backends, rclone stores the original modtime as
additional metadata on the object. By default it will make an API call to
retrieve the metadata when the modtime is needed by an operation.

Use this flag to disable the extra API call and rely instead on the server's
modified time. In cases such as a local to remote sync using `--update`,
knowing the local file is newer than the time it was last uploaded to the
remote is sufficient. In those cases, this flag can speed up the process and
reduce the number of API calls necessary.

Using this flag on a sync operation without also using `--update` would cause
all files modified at any time other than the last upload time to be uploaded
again, which is probably not what you want.

### -v, -vv, --verbose ###

With `-v` rclone will tell you about each file that is transferred and
a small number of significant events.

With `-vv` rclone will become very verbose telling you about every
file it considers and transfers.  Please send bug reports with a log
with this setting.

### -V, --version ###

Prints the version number

SSL/TLS options
---------------

The outgoing SSL/TLS connections rclone makes can be controlled with
these options.  For example this can be very useful with the HTTP or
WebDAV backends. Rclone HTTP servers have their own set of
configuration for SSL/TLS which you can find in their documentation.

### --ca-cert string

This loads the PEM encoded certificate authority certificate and uses
it to verify the certificates of the servers rclone connects to.

If you have generated certificates signed with a local CA then you
will need this flag to connect to servers using those certificates.

### --client-cert string

This loads the PEM encoded client side certificate.

This is used for [mutual TLS authentication](https://en.wikipedia.org/wiki/Mutual_authentication).

The `--client-key` flag is required too when using this.

### --client-key string

This loads the PEM encoded client side private key used for mutual TLS
authentication.  Used in conjunction with `--client-cert`.

### --no-check-certificate=true/false ###

`--no-check-certificate` controls whether a client verifies the
server's certificate chain and host name.
If `--no-check-certificate` is true, TLS accepts any certificate
presented by the server and any host name in that certificate.
In this mode, TLS is susceptible to man-in-the-middle attacks.

This option defaults to `false`.

**This should be used only for testing.**

Configuration Encryption
------------------------
Your configuration file contains information for logging in to 
your cloud services. This means that you should keep your 
`.rclone.conf` file in a secure location.

If you are in an environment where that isn't possible, you can
add a password to your configuration. This means that you will
have to supply the password every time you start rclone.

To add a password to your rclone configuration, execute `rclone config`.

```
>rclone config
Current remotes:

e) Edit existing remote
n) New remote
d) Delete remote
s) Set configuration password
q) Quit config
e/n/d/s/q>
```

Go into `s`, Set configuration password:
```
e/n/d/s/q> s
Your configuration is not encrypted.
If you add a password, you will protect your login information to cloud services.
a) Add Password
q) Quit to main menu
a/q> a
Enter NEW configuration password:
password:
Confirm NEW password:
password:
Password set
Your configuration is encrypted.
c) Change Password
u) Unencrypt configuration
q) Quit to main menu
c/u/q>
```

Your configuration is now encrypted, and every time you start rclone
you will have to supply the password. See below for details.
In the same menu, you can change the password or completely remove
encryption from your configuration.

There is no way to recover the configuration if you lose your password.

rclone uses [nacl secretbox](https://godoc.org/golang.org/x/crypto/nacl/secretbox) 
which in turn uses XSalsa20 and Poly1305 to encrypt and authenticate 
your configuration with secret-key cryptography.
The password is SHA-256 hashed, which produces the key for secretbox.
The hashed password is not stored.

While this provides very good security, we do not recommend storing
your encrypted rclone configuration in public if it contains sensitive
information, maybe except if you use a very strong password.

If it is safe in your environment, you can set the `RCLONE_CONFIG_PASS`
environment variable to contain your password, in which case it will be
used for decrypting the configuration.

You can set this for a session from a script.  For unix like systems
save this to a file called `set-rclone-password`:

```
#!/bin/echo Source this file don't run it

read -s RCLONE_CONFIG_PASS
export RCLONE_CONFIG_PASS
```

Then source the file when you want to use it.  From the shell you
would do `source set-rclone-password`.  It will then ask you for the
password and set it in the environment variable.

An alternate means of supplying the password is to provide a script
which will retrieve the password and print on standard output.  This
script should have a fully specified path name and not rely on any
environment variables.  The script is supplied either via
`--password-command="..."` command line argument or via the
`RCLONE_PASSWORD_COMMAND` environment variable.

One useful example of this is using the `passwordstore` application
to retrieve the password:

```
export RCLONE_PASSWORD_COMMAND="pass rclone/config"
```

If the `passwordstore` password manager holds the password for the
rclone configuration, using the script method means the password
is primarily protected by the `passwordstore` system, and is never
embedded in the clear in scripts, nor available for examination
using the standard commands available.  It is quite possible with
long running rclone sessions for copies of passwords to be innocently
captured in log files or terminal scroll buffers, etc.  Using the
script method of supplying the password enhances the security of
the config password considerably.

If you are running rclone inside a script, unless you are using the
`--password-command` method, you might want to disable 
password prompts. To do that, pass the parameter 
`--ask-password=false` to rclone. This will make rclone fail instead
of asking for a password if `RCLONE_CONFIG_PASS` doesn't contain
a valid password, and `--password-command` has not been supplied.


Developer options
-----------------

These options are useful when developing or debugging rclone.  There
are also some more remote specific options which aren't documented
here which are used for testing.  These start with remote name eg
`--drive-test-option` - see the docs for the remote in question.

### --cpuprofile=FILE ###

Write CPU profile to file.  This can be analysed with `go tool pprof`.

#### --dump flag,flag,flag ####

The `--dump` flag takes a comma separated list of flags to dump info
about.

Note that some headers including `Accept-Encoding` as shown may not 
be correct in the request and the response may not show `Content-Encoding`
if the go standard libraries auto gzip encoding was in effect. In this case 
the body of the request will be gunzipped before showing it.

The available flags are:

#### --dump headers ####

Dump HTTP headers with `Authorization:` lines removed. May still
contain sensitive info.  Can be very verbose.  Useful for debugging
only.

Use `--dump auth` if you do want the `Authorization:` headers.

#### --dump bodies ####

Dump HTTP headers and bodies - may contain sensitive info.  Can be
very verbose.  Useful for debugging only.

Note that the bodies are buffered in memory so don't use this for
enormous files.

#### --dump requests ####

Like `--dump bodies` but dumps the request bodies and the response
headers.  Useful for debugging download problems.

#### --dump responses ####

Like `--dump bodies` but dumps the response bodies and the request
headers. Useful for debugging upload problems.

#### --dump auth ####

Dump HTTP headers - will contain sensitive info such as
`Authorization:` headers - use `--dump headers` to dump without
`Authorization:` headers.  Can be very verbose.  Useful for debugging
only.

#### --dump filters ####

Dump the filters to the output.  Useful to see exactly what include
and exclude options are filtering on.

#### --dump goroutines ####

This dumps a list of the running go-routines at the end of the command
to standard output.

#### --dump openfiles ####

This dumps a list of the open files at the end of the command.  It
uses the `lsof` command to do that so you'll need that installed to
use it.

### --memprofile=FILE ###

Write memory profile to file. This can be analysed with `go tool pprof`.

Filtering
---------

For the filtering options

  * `--delete-excluded`
  * `--filter`
  * `--filter-from`
  * `--exclude`
  * `--exclude-from`
  * `--include`
  * `--include-from`
  * `--files-from`
  * `--files-from-raw`
  * `--min-size`
  * `--max-size`
  * `--min-age`
  * `--max-age`
  * `--dump filters`

See the [filtering section](https://rclone.org/filtering/).

Remote control
--------------

For the remote control options and for instructions on how to remote control rclone

  * `--rc`
  * and anything starting with `--rc-`

See [the remote control section](https://rclone.org/rc/).

Logging
-------

rclone has 4 levels of logging, `ERROR`, `NOTICE`, `INFO` and `DEBUG`.

By default, rclone logs to standard error.  This means you can redirect
standard error and still see the normal output of rclone commands (eg
`rclone ls`).

By default, rclone will produce `Error` and `Notice` level messages.

If you use the `-q` flag, rclone will only produce `Error` messages.

If you use the `-v` flag, rclone will produce `Error`, `Notice` and
`Info` messages.

If you use the `-vv` flag, rclone will produce `Error`, `Notice`,
`Info` and `Debug` messages.

You can also control the log levels with the `--log-level` flag.

If you use the `--log-file=FILE` option, rclone will redirect `Error`,
`Info` and `Debug` messages along with standard error to FILE.

If you use the `--syslog` flag then rclone will log to syslog and the
`--syslog-facility` control which facility it uses.

Rclone prefixes all log messages with their level in capitals, eg INFO
which makes it easy to grep the log file for different kinds of
information.

Exit Code
---------

If any errors occur during the command execution, rclone will exit with a
non-zero exit code.  This allows scripts to detect when rclone
operations have failed.

During the startup phase, rclone will exit immediately if an error is
detected in the configuration.  There will always be a log message
immediately before exiting.

When rclone is running it will accumulate errors as it goes along, and
only exit with a non-zero exit code if (after retries) there were
still failed transfers.  For every error counted there will be a high
priority log message (visible with `-q`) showing the message and
which file caused the problem. A high priority message is also shown
when starting a retry so the user can see that any previous error
messages may not be valid after the retry. If rclone has done a retry
it will log a high priority message if the retry was successful.

### List of exit codes ###
  * `0` - success
  * `1` - Syntax or usage error
  * `2` - Error not otherwise categorised
  * `3` - Directory not found
  * `4` - File not found
  * `5` - Temporary error (one that more retries might fix) (Retry errors)
  * `6` - Less serious errors (like 461 errors from dropbox) (NoRetry errors)
  * `7` - Fatal error (one that more retries won't fix, like account suspended) (Fatal errors)
  * `8` - Transfer exceeded - limit set by --max-transfer reached
  * `9` - Operation successful, but no files transferred

Environment Variables
---------------------

Rclone can be configured entirely using environment variables.  These
can be used to set defaults for options or config file entries.

### Options ###

Every option in rclone can have its default set by environment
variable.

To find the name of the environment variable, first, take the long
option name, strip the leading `--`, change `-` to `_`, make
upper case and prepend `RCLONE_`.

For example, to always set `--stats 5s`, set the environment variable
`RCLONE_STATS=5s`.  If you set stats on the command line this will
override the environment variable setting.

Or to always use the trash in drive `--drive-use-trash`, set
`RCLONE_DRIVE_USE_TRASH=true`.

The same parser is used for the options and the environment variables
so they take exactly the same form.

### Config file ###

You can set defaults for values in the config file on an individual
remote basis.  If you want to use this feature, you will need to
discover the name of the config items that you want.  The easiest way
is to run through `rclone config` by hand, then look in the config
file to see what the values are (the config file can be found by
looking at the help for `--config` in `rclone help`).

To find the name of the environment variable, you need to set, take
`RCLONE_CONFIG_` + name of remote + `_` + name of config file option
and make it all uppercase.

For example, to configure an S3 remote named `mys3:` without a config
file (using unix ways of setting environment variables):

```
$ export RCLONE_CONFIG_MYS3_TYPE=s3
$ export RCLONE_CONFIG_MYS3_ACCESS_KEY_ID=XXX
$ export RCLONE_CONFIG_MYS3_SECRET_ACCESS_KEY=XXX
$ rclone lsd MYS3:
          -1 2016-09-21 12:54:21        -1 my-bucket
$ rclone listremotes | grep mys3
mys3:
```

Note that if you want to create a remote using environment variables
you must create the `..._TYPE` variable as above.

### Other environment variables ###

  * `RCLONE_CONFIG_PASS` set to contain your config file password (see [Configuration Encryption](#configuration-encryption) section)
  * `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` (or the lowercase versions thereof).
    * `HTTPS_PROXY` takes precedence over `HTTP_PROXY` for https requests.
    * The environment values may be either a complete URL or a "host[:port]" for, in which case the "http" scheme is assumed.

# Configuring rclone on a remote / headless machine #

Some of the configurations (those involving oauth2) require an
Internet connected web browser.

If you are trying to set rclone up on a remote or headless box with no
browser available on it (eg a NAS or a server in a datacenter) then
you will need to use an alternative means of configuration.  There are
two ways of doing it, described below.

## Configuring using rclone authorize ##

On the headless box run `rclone` config but answer `N` to the `Use
auto config?` question.

```
...
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes (default)
n) No
y/n> n
For this to work, you will need rclone available on a machine that has
a web browser available.

For more help and alternate methods see: https://rclone.org/remote_setup/

Execute the following on the machine with the web browser (same rclone
version recommended):

	rclone authorize "amazon cloud drive"

Then paste the result below:
result>
```

Then on your main desktop machine

```
rclone authorize "amazon cloud drive"
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
Paste the following into your remote machine --->
SECRET_TOKEN
<---End paste
```

Then back to the headless box, paste in the code

```
result> SECRET_TOKEN
--------------------
[acd12]
client_id = 
client_secret = 
token = SECRET_TOKEN
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d>
```

## Configuring by copying the config file ##

Rclone stores all of its config in a single configuration file.  This
can easily be copied to configure a remote rclone.

So first configure rclone on your desktop machine with

    rclone config

to set up the config file.

Find the config file by running `rclone config file`, for example

```
$ rclone config file
Configuration file is stored at:
/home/user/.rclone.conf
```

Now transfer it to the remote box (scp, cut paste, ftp, sftp etc) and
place it in the correct place (use `rclone config file` on the remote
box to find out where).

# Filtering, includes and excludes #

Rclone has a sophisticated set of include and exclude rules. Some of
these are based on patterns and some on other things like file size.

The filters are applied for the `copy`, `sync`, `move`, `ls`, `lsl`,
`md5sum`, `sha1sum`, `size`, `delete` and `check` operations.
Note that `purge` does not obey the filters.

Each path as it passes through rclone is matched against the include
and exclude rules like `--include`, `--exclude`, `--include-from`,
`--exclude-from`, `--filter`, or `--filter-from`. The simplest way to
try them out is using the `ls` command, or `--dry-run` together with
`-v`. `--filter-from`, `--exclude-from`, `--include-from`, `--files-from`,
`--files-from-raw` understand `-` as a file name to mean read from standard
input.

## Patterns ##

The patterns used to match files for inclusion or exclusion are based
on "file globs" as used by the unix shell.

If the pattern starts with a `/` then it only matches at the top level
of the directory tree, **relative to the root of the remote** (not
necessarily the root of the local drive). If it doesn't start with `/`
then it is matched starting at the **end of the path**, but it will
only match a complete path element:

    file.jpg  - matches "file.jpg"
              - matches "directory/file.jpg"
              - doesn't match "afile.jpg"
              - doesn't match "directory/afile.jpg"
    /file.jpg - matches "file.jpg" in the root directory of the remote
              - doesn't match "afile.jpg"
              - doesn't match "directory/file.jpg"

**Important** Note that you must use `/` in patterns and not `\` even
if running on Windows.

A `*` matches anything but not a `/`.

    *.jpg  - matches "file.jpg"
           - matches "directory/file.jpg"
           - doesn't match "file.jpg/something"

Use `**` to match anything, including slashes (`/`).

    dir/** - matches "dir/file.jpg"
           - matches "dir/dir1/dir2/file.jpg"
           - doesn't match "directory/file.jpg"
           - doesn't match "adir/file.jpg"

A `?` matches any character except a slash `/`.

    l?ss  - matches "less"
          - matches "lass"
          - doesn't match "floss"

A `[` and `]` together make a character class, such as `[a-z]` or
`[aeiou]` or `[[:alpha:]]`.  See the [go regexp
docs](https://golang.org/pkg/regexp/syntax/) for more info on these.

    h[ae]llo - matches "hello"
             - matches "hallo"
             - doesn't match "hullo"

A `{` and `}` define a choice between elements.  It should contain a
comma separated list of patterns, any of which might match.  These
patterns can contain wildcards.

    {one,two}_potato - matches "one_potato"
                     - matches "two_potato"
                     - doesn't match "three_potato"
                     - doesn't match "_potato"

Special characters can be escaped with a `\` before them.

    \*.jpg       - matches "*.jpg"
    \\.jpg       - matches "\.jpg"
    \[one\].jpg  - matches "[one].jpg"

Patterns are case sensitive unless the `--ignore-case` flag is used.

Without `--ignore-case` (default)

    potato - matches "potato"
           - doesn't match "POTATO"

With `--ignore-case`

    potato - matches "potato"
           - matches "POTATO"

Note also that rclone filter globs can only be used in one of the
filter command line flags, not in the specification of the remote, so
`rclone copy "remote:dir*.jpg" /path/to/dir` won't work - what is
required is `rclone --include "*.jpg" copy remote:dir /path/to/dir`

### Directories ###

Rclone keeps track of directories that could match any file patterns.

Eg if you add the include rule

    /a/*.jpg

Rclone will synthesize the directory include rule

    /a/

If you put any rules which end in `/` then it will only match
directories.

Directory matches are **only** used to optimise directory access
patterns - you must still match the files that you want to match.
Directory matches won't optimise anything on bucket based remotes (eg
s3, swift, google compute storage, b2) which don't have a concept of
directory.

### Differences between rsync and rclone patterns ###

Rclone implements bash style `{a,b,c}` glob matching which rsync doesn't.

Rclone always does a wildcard match so `\` must always escape a `\`.

## How the rules are used ##

Rclone maintains a combined list of include rules and exclude rules.

Each file is matched in order, starting from the top, against the rule
in the list until it finds a match.  The file is then included or
excluded according to the rule type.

If the matcher fails to find a match after testing against all the
entries in the list then the path is included.

For example given the following rules, `+` being include, `-` being
exclude,

    - secret*.jpg
    + *.jpg
    + *.png
    + file2.avi
    - *

This would include

  * `file1.jpg`
  * `file3.png`
  * `file2.avi`

This would exclude

  * `secret17.jpg`
  * non `*.jpg` and `*.png`

A similar process is done on directory entries before recursing into
them.  This only works on remotes which have a concept of directory
(Eg local, google drive, onedrive, amazon drive) and not on bucket
based remotes (eg s3, swift, google compute storage, b2).

## Adding filtering rules ##

Filtering rules are added with the following command line flags.

### Repeating options ##

You can repeat the following options to add more than one rule of that
type.

  * `--include`
  * `--include-from`
  * `--exclude`
  * `--exclude-from`
  * `--filter`
  * `--filter-from`
  * `--filter-from-raw`

**Important** You should not use `--include*` together with `--exclude*`. 
It may produce different results than you expected. In that case try to use: `--filter*`.

Note that all the options of the same type are processed together in
the order above, regardless of what order they were placed on the
command line.

So all `--include` options are processed first in the order they
appeared on the command line, then all `--include-from` options etc.

To mix up the order includes and excludes, the `--filter` flag can be
used.

### `--exclude` - Exclude files matching pattern ###

Add a single exclude rule with `--exclude`.

This flag can be repeated.  See above for the order the flags are
processed in.

Eg `--exclude *.bak` to exclude all bak files from the sync.

### `--exclude-from` - Read exclude patterns from file ###

Add exclude rules from a file.

This flag can be repeated.  See above for the order the flags are
processed in.

Prepare a file like this `exclude-file.txt`

    # a sample exclude rule file
    *.bak
    file2.jpg

Then use as `--exclude-from exclude-file.txt`.  This will sync all
files except those ending in `bak` and `file2.jpg`.

This is useful if you have a lot of rules.

### `--include` - Include files matching pattern ###

Add a single include rule with `--include`.

This flag can be repeated.  See above for the order the flags are
processed in.

Eg `--include *.{png,jpg}` to include all `png` and `jpg` files in the
backup and no others.

This adds an implicit `--exclude *` at the very end of the filter
list. This means you can mix `--include` and `--include-from` with the
other filters (eg `--exclude`) but you must include all the files you
want in the include statement.  If this doesn't provide enough
flexibility then you must use `--filter-from`.

### `--include-from` - Read include patterns from file ###

Add include rules from a file.

This flag can be repeated.  See above for the order the flags are
processed in.

Prepare a file like this `include-file.txt`

    # a sample include rule file
    *.jpg
    *.png
    file2.avi

Then use as `--include-from include-file.txt`.  This will sync all
`jpg`, `png` files and `file2.avi`.

This is useful if you have a lot of rules.

This adds an implicit `--exclude *` at the very end of the filter
list. This means you can mix `--include` and `--include-from` with the
other filters (eg `--exclude`) but you must include all the files you
want in the include statement.  If this doesn't provide enough
flexibility then you must use `--filter-from`.

### `--filter` - Add a file-filtering rule ###

This can be used to add a single include or exclude rule.  Include
rules start with `+ ` and exclude rules start with `- `.  A special
rule called `!` can be used to clear the existing rules.

This flag can be repeated.  See above for the order the flags are
processed in.

Eg `--filter "- *.bak"` to exclude all bak files from the sync.

### `--filter-from` - Read filtering patterns from a file ###

Add include/exclude rules from a file.

This flag can be repeated.  See above for the order the flags are
processed in.

Prepare a file like this `filter-file.txt`

    # a sample filter rule file
    - secret*.jpg
    + *.jpg
    + *.png
    + file2.avi
    - /dir/Trash/**
    + /dir/**
    # exclude everything else
    - *

Then use as `--filter-from filter-file.txt`.  The rules are processed
in the order that they are defined.

This example will include all `jpg` and `png` files, exclude any files
matching `secret*.jpg` and include `file2.avi`.  It will also include
everything in the directory `dir` at the root of the sync, except
`dir/Trash` which it will exclude.  Everything else will be excluded
from the sync.

### `--files-from` - Read list of source-file names ###

This reads a list of file names from the file passed in and **only**
these files are transferred.  The **filtering rules are ignored**
completely if you use this option.

`--files-from` expects a list of files as its input. Leading / trailing
whitespace is stripped from the input lines and lines starting with `#`
and `;` are ignored.

Rclone will traverse the file system if you use `--files-from`,
effectively using the files in `--files-from` as a set of filters.
Rclone will not error if any of the files are missing.

If you use `--no-traverse` as well as `--files-from` then rclone will
not traverse the destination file system, it will find each file
individually using approximately 1 API call. This can be more
efficient for small lists of files.

This option can be repeated to read from more than one file.  These
are read in the order that they are placed on the command line.

Paths within the `--files-from` file will be interpreted as starting
with the root specified in the command.  Leading `/` characters are
ignored. See [--files-from-raw](#files-from-raw-read-list-of-source-file-names-without-any-processing)
if you need the input to be processed in a raw manner.

For example, suppose you had `files-from.txt` with this content:

    # comment
    file1.jpg
    subdir/file2.jpg

You could then use it like this:

    rclone copy --files-from files-from.txt /home/me/pics remote:pics

This will transfer these files only (if they exist)

    /home/me/pics/file1.jpg        → remote:pics/file1.jpg
    /home/me/pics/subdir/file2.jpg → remote:pics/subdir/file2.jpg

To take a more complicated example, let's say you had a few files you
want to back up regularly with these absolute paths:

    /home/user1/important
    /home/user1/dir/file
    /home/user2/stuff

To copy these you'd find a common subdirectory - in this case `/home`
and put the remaining files in `files-from.txt` with or without
leading `/`, eg

    user1/important
    user1/dir/file
    user2/stuff

You could then copy these to a remote like this

    rclone copy --files-from files-from.txt /home remote:backup

The 3 files will arrive in `remote:backup` with the paths as in the
`files-from.txt` like this:

    /home/user1/important → remote:backup/user1/important
    /home/user1/dir/file  → remote:backup/user1/dir/file
    /home/user2/stuff     → remote:backup/user2/stuff

You could of course choose `/` as the root too in which case your
`files-from.txt` might look like this.

    /home/user1/important
    /home/user1/dir/file
    /home/user2/stuff

And you would transfer it like this

    rclone copy --files-from files-from.txt / remote:backup

In this case there will be an extra `home` directory on the remote:

    /home/user1/important → remote:backup/home/user1/important
    /home/user1/dir/file  → remote:backup/home/user1/dir/file
    /home/user2/stuff     → remote:backup/home/user2/stuff

### `--files-from-raw` - Read list of source-file names without any processing ###
This option is same as `--files-from` with the only difference being that the input
is read in a raw manner. This means that lines with leading/trailing whitespace and
lines starting with `;` or `#` are read without any processing. [rclone lsf](https://rclone.org/commands/rclone_lsf/)
has a compatible format that can be used to export file lists from remotes, which
can then be used as an input to `--files-from-raw`.

### `--min-size` - Don't transfer any file smaller than this ###

This option controls the minimum size file which will be transferred.
This defaults to `kBytes` but a suffix of `k`, `M`, or `G` can be
used.

For example `--min-size 50k` means no files smaller than 50kByte will be
transferred.

### `--max-size` - Don't transfer any file larger than this ###

This option controls the maximum size file which will be transferred.
This defaults to `kBytes` but a suffix of `k`, `M`, or `G` can be
used.

For example `--max-size 1G` means no files larger than 1GByte will be
transferred.

### `--max-age` - Don't transfer any file older than this ###

This option controls the maximum age of files to transfer.  Give in
seconds or with a suffix of:

  * `ms` - Milliseconds
  * `s` - Seconds
  * `m` - Minutes
  * `h` - Hours
  * `d` - Days
  * `w` - Weeks
  * `M` - Months
  * `y` - Years

For example `--max-age 2d` means no files older than 2 days will be
transferred.

This can also be an absolute time in one of these formats

- RFC3339 - eg "2006-01-02T15:04:05Z07:00"
- ISO8601 Date and time, local timezone - "2006-01-02T15:04:05"
- ISO8601 Date and time, local timezone - "2006-01-02 15:04:05"
- ISO8601 Date - "2006-01-02" (YYYY-MM-DD)

### `--min-age` - Don't transfer any file younger than this ###

This option controls the minimum age of files to transfer.  Give in
seconds or with a suffix (see `--max-age` for list of suffixes)

For example `--min-age 2d` means no files younger than 2 days will be
transferred.

### `--delete-excluded` - Delete files on dest excluded from sync ###

**Important** this flag is dangerous - use with `--dry-run` and `-v` first.

When doing `rclone sync` this will delete any files which are excluded
from the sync on the destination.

If for example you did a sync from `A` to `B` without the `--min-size 50k` flag

    rclone sync A: B:

Then you repeated it like this with the `--delete-excluded`

    rclone --min-size 50k --delete-excluded sync A: B:

This would delete all files on `B` which are less than 50 kBytes as
these are now excluded from the sync.

Always test first with `--dry-run` and `-v` before using this flag.

### `--dump filters` - dump the filters to the output ###

This dumps the defined filters to the output as regular expressions.

Useful for debugging.

### `--ignore-case` - make searches case insensitive ###

Normally filter patterns are case sensitive.  If this flag is supplied
then filter patterns become case insensitive.

Normally a `--include "file.txt"` will not match a file called
`FILE.txt`.  However if you use the `--ignore-case` flag then
`--include "file.txt"` this will match a file called `FILE.txt`.

## Quoting shell metacharacters ##

The examples above may not work verbatim in your shell as they have
shell metacharacters in them (eg `*`), and may require quoting.

Eg linux, OSX

  * `--include \*.jpg`
  * `--include '*.jpg'`
  * `--include='*.jpg'`

In Windows the expansion is done by the command not the shell so this
should work fine

  * `--include *.jpg`

## Exclude directory based on a file ##

It is possible to exclude a directory based on a file, which is
present in this directory. Filename should be specified using the
`--exclude-if-present` flag. This flag has a priority over the other
filtering flags.

Imagine, you have the following directory structure:

    dir1/file1
    dir1/dir2/file2
    dir1/dir2/dir3/file3
    dir1/dir2/dir3/.ignore

You can exclude `dir3` from sync by running the following command:

    rclone sync --exclude-if-present .ignore dir1 remote:backup

Currently only one filename is supported, i.e. `--exclude-if-present`
should not be used multiple times.

# GUI (Experimental)

Rclone can serve a web based GUI (graphical user interface).  This is
somewhat experimental at the moment so things may be subject to
change.

Run this command in a terminal and rclone will download and then
display the GUI in a web browser.

```
rclone rcd --rc-web-gui
```

This will produce logs like this and rclone needs to continue to run to serve the GUI:
    
```
2019/08/25 11:40:14 NOTICE: A new release for gui is present at https://github.com/rclone/rclone-webui-react/releases/download/v0.0.6/currentbuild.zip
2019/08/25 11:40:14 NOTICE: Downloading webgui binary. Please wait. [Size: 3813937, Path :  /home/USER/.cache/rclone/webgui/v0.0.6.zip]
2019/08/25 11:40:16 NOTICE: Unzipping
2019/08/25 11:40:16 NOTICE: Serving remote control on http://127.0.0.1:5572/
```

This assumes you are running rclone locally on your machine.  It is
possible to separate the rclone and the GUI - see below for details.

If you wish to check for updates then you can add `--rc-web-gui-update` 
to the command line.

If you find your GUI broken, you may force it to update by add `--rc-web-gui-force-update`.

By default, rclone will open your browser. Add `--rc-web-gui-no-open-browser` 
to disable this feature.

## Using the GUI

Once the GUI opens, you will be looking at the dashboard which has an overall overview.

On the left hand side you will see a series of view buttons you can click on:

- Dashboard - main overview
- Configs - examine and create new configurations
- Explorer - view, download and upload files to the cloud storage systems
- Backend - view or alter the backend config
- Log out

(More docs and walkthrough video to come!)

## How it works

When you run the `rclone rcd --rc-web-gui` this is what happens

- Rclone starts but only runs the remote control API ("rc").
- The API is bound to localhost with an auto generated username and password.
- If the API bundle is missing then rclone will download it.
- rclone will start serving the files from the API bundle over the same port as the API
- rclone will open the browser with a `login_token` so it can log straight in.

## Advanced use

The `rclone rcd` may use any of the [flags documented on the rc page](https://rclone.org/rc/#supported-parameters).

The flag `--rc-web-gui` is shorthand for

- Download the web GUI if necessary
- Check we are using some authentication
- `--rc-user gui`
- `--rc-pass <random password>`
- `--rc-serve`

These flags can be overridden as desired.

See also the [rclone rcd documentation](https://rclone.org/commands/rclone_rcd/).

### Example: Running a public GUI

For example the GUI could be served on a public port over SSL using an htpasswd file using the following flags:

- `--rc-web-gui`
- `--rc-addr :443`
- `--rc-htpasswd /path/to/htpasswd`
- `--rc-cert /path/to/ssl.crt`
- `--rc-key /path/to/ssl.key`

### Example: Running a GUI behind a proxy

If you want to run the GUI behind a proxy at `/rclone` you could use these flags:

- `--rc-web-gui`
- `--rc-baseurl rclone`
- `--rc-htpasswd /path/to/htpasswd`

Or instead of htpasswd if you just want a single user and password:

- `--rc-user me`
- `--rc-pass mypassword`

## Project

The GUI is being developed in the: [rclone/rclone-webui-react repository](https://github.com/rclone/rclone-webui-react).

Bug reports and contributions are very welcome :-)

If you have questions then please ask them on the [rclone forum](https://forum.rclone.org/).

# Remote controlling rclone with its API

If rclone is run with the `--rc` flag then it starts an http server
which can be used to remote control rclone using its API.

If you just want to run a remote control then see the [rcd command](https://rclone.org/commands/rclone_rcd/).

## Supported parameters

### --rc

Flag to start the http server listen on remote requests
      
### --rc-addr=IP

IPaddress:Port or :Port to bind server to. (default "localhost:5572")

### --rc-cert=KEY
SSL PEM key (concatenation of certificate and CA certificate)

### --rc-client-ca=PATH
Client certificate authority to verify clients with

### --rc-htpasswd=PATH

htpasswd file - if not provided no authentication is done

### --rc-key=PATH

SSL PEM Private key

### --rc-max-header-bytes=VALUE

Maximum size of request header (default 4096)

### --rc-user=VALUE

User name for authentication.

### --rc-pass=VALUE

Password for authentication.

### --rc-realm=VALUE

Realm for authentication (default "rclone")

### --rc-server-read-timeout=DURATION

Timeout for server reading data (default 1h0m0s)

### --rc-server-write-timeout=DURATION

Timeout for server writing data (default 1h0m0s)

### --rc-serve

Enable the serving of remote objects via the HTTP interface.  This
means objects will be accessible at http://127.0.0.1:5572/ by default,
so you can browse to http://127.0.0.1:5572/ or http://127.0.0.1:5572/*
to see a listing of the remotes.  Objects may be requested from
remotes using this syntax http://127.0.0.1:5572/[remote:path]/path/to/object

Default Off.

### --rc-files /path/to/directory

Path to local files to serve on the HTTP server.

If this is set then rclone will serve the files in that directory.  It
will also open the root in the web browser if specified.  This is for
implementing browser based GUIs for rclone functions.

If `--rc-user` or `--rc-pass` is set then the URL that is opened will
have the authorization in the URL in the `http://user:pass@localhost/`
style.

Default Off.

### --rc-enable-metrics

Enable OpenMetrics/Prometheus compatible endpoint at `/metrics`.

Default Off.

### --rc-web-gui

Set this flag to serve the default web gui on the same port as rclone.

Default Off.

### --rc-allow-origin

Set the allowed Access-Control-Allow-Origin for rc requests.

Can be used with --rc-web-gui if the rclone is running on different IP than the web-gui.

Default is IP address on which rc is running.

### --rc-web-fetch-url

Set the URL to fetch the rclone-web-gui files from.

Default https://api.github.com/repos/rclone/rclone-webui-react/releases/latest.

### --rc-web-gui-update

Set this flag to check and update rclone-webui-react from the rc-web-fetch-url.

Default Off.

### --rc-web-gui-force-update

Set this flag to force update rclone-webui-react from the rc-web-fetch-url.

Default Off.

### --rc-web-gui-no-open-browser

Set this flag to disable opening browser automatically when using web-gui.

Default Off.

### --rc-job-expire-duration=DURATION

Expire finished async jobs older than DURATION (default 60s).

### --rc-job-expire-interval=DURATION

Interval duration to check for expired async jobs (default 10s).

### --rc-no-auth

By default rclone will require authorisation to have been set up on
the rc interface in order to use any methods which access any rclone
remotes.  Eg `operations/list` is denied as it involved creating a
remote as is `sync/copy`.

If this is set then no authorisation will be required on the server to
use these methods.  The alternative is to use `--rc-user` and
`--rc-pass` and use these credentials in the request.

Default Off.

## Accessing the remote control via the rclone rc command

Rclone itself implements the remote control protocol in its `rclone
rc` command.

You can use it like this

```
$ rclone rc rc/noop param1=one param2=two
{
	"param1": "one",
	"param2": "two"
}
```

Run `rclone rc` on its own to see the help for the installed remote
control commands.

`rclone rc` also supports a `--json` flag which can be used to send
more complicated input parameters.

```
$ rclone rc --json '{ "p1": [1,"2",null,4], "p2": { "a":1, "b":2 } }' rc/noop
{
	"p1": [
		1,
		"2",
		null,
		4
	],
	"p2": {
		"a": 1,
		"b": 2
	}
}
```

## Special parameters

The rc interface supports some special parameters which apply to
**all** commands.  These start with `_` to show they are different.

### Running asynchronous jobs with _async = true

Each rc call is classified as a job and it is assigned its own id. By default
jobs are executed immediately as they are created or synchronously.

If `_async` has a true value when supplied to an rc call then it will
return immediately with a job id and the task will be run in the
background.  The `job/status` call can be used to get information of
the background job.  The job can be queried for up to 1 minute after
it has finished.

It is recommended that potentially long running jobs, eg `sync/sync`,
`sync/copy`, `sync/move`, `operations/purge` are run with the `_async`
flag to avoid any potential problems with the HTTP request and
response timing out.

Starting a job with the `_async` flag:

```
$ rclone rc --json '{ "p1": [1,"2",null,4], "p2": { "a":1, "b":2 }, "_async": true }' rc/noop
{
	"jobid": 2
}
```

Query the status to see if the job has finished.  For more information
on the meaning of these return parameters see the `job/status` call.

```
$ rclone rc --json '{ "jobid":2 }' job/status
{
	"duration": 0.000124163,
	"endTime": "2018-10-27T11:38:07.911245881+01:00",
	"error": "",
	"finished": true,
	"id": 2,
	"output": {
		"_async": true,
		"p1": [
			1,
			"2",
			null,
			4
		],
		"p2": {
			"a": 1,
			"b": 2
		}
	},
	"startTime": "2018-10-27T11:38:07.911121728+01:00",
	"success": true
}
```

`job/list` can be used to show the running or recently completed jobs

```
$ rclone rc job/list
{
	"jobids": [
		2
	]
}
```

### Assigning operations to groups with _group = value

Each rc call has its own stats group for tracking its metrics. By default
grouping is done by the composite group name from prefix `job/` and  id of the
job like so `job/1`.

If `_group` has a value then stats for that request will be grouped under that
value. This allows caller to group stats under their own name.

Stats for specific group can be accessed by passing `group` to `core/stats`:

```
$ rclone rc --json '{ "group": "job/1" }' core/stats
{
	"speed": 12345
	...
}
```

## Supported commands

### backend/command: Runs a backend command. {#backend-command}

This takes the following parameters

- command - a string with the command name
- fs - a remote name string eg "drive:"
- arg - a list of arguments for the backend command
- opt - a map of string to string of options

Returns

- result - result from the backend command

For example

    rclone rc backend/command command=noop fs=. -o echo=yes -o blue -a path1 -a path2

Returns

```
{
	"result": {
		"arg": [
			"path1",
			"path2"
		],
		"name": "noop",
		"opt": {
			"blue": "",
			"echo": "yes"
		}
	}
}
```

Note that this is the direct equivalent of using this "backend"
command:

    rclone backend noop . -o echo=yes -o blue path1 path2

Note that arguments must be preceded by the "-a" flag

See the [backend](https://rclone.org/commands/rclone_backend/) command for more information.

**Authentication is required for this call.**

### cache/expire: Purge a remote from cache {#cache-expire}

Purge a remote from the cache backend. Supports either a directory or a file.
Params:
  - remote = path to remote (required)
  - withData = true/false to delete cached data (chunks) as well (optional)

Eg

    rclone rc cache/expire remote=path/to/sub/folder/
    rclone rc cache/expire remote=/ withData=true

### cache/fetch: Fetch file chunks {#cache-fetch}

Ensure the specified file chunks are cached on disk.

The chunks= parameter specifies the file chunks to check.
It takes a comma separated list of array slice indices.
The slice indices are similar to Python slices: start[:end]

start is the 0 based chunk number from the beginning of the file
to fetch inclusive. end is 0 based chunk number from the beginning
of the file to fetch exclusive.
Both values can be negative, in which case they count from the back
of the file. The value "-5:" represents the last 5 chunks of a file.

Some valid examples are:
":5,-5:" -> the first and last five chunks
"0,-2" -> the first and the second last chunk
"0:10" -> the first ten chunks

Any parameter with a key that starts with "file" can be used to
specify files to fetch, eg

    rclone rc cache/fetch chunks=0 file=hello file2=home/goodbye

File names will automatically be encrypted when the a crypt remote
is used on top of the cache.

### cache/stats: Get cache stats {#cache-stats}

Show statistics for the cache remote.

### config/create: create the config for a remote. {#config-create}

This takes the following parameters

- name - name of remote
- parameters - a map of \{ "key": "value" \} pairs
- type - type of the new remote
- obscure - optional bool - forces obscuring of passwords
- noObscure - optional bool - forces passwords not to be obscured


See the [config create command](https://rclone.org/commands/rclone_config_create/) command for more information on the above.

**Authentication is required for this call.**

### config/delete: Delete a remote in the config file. {#config-delete}

Parameters:

- name - name of remote to delete

See the [config delete command](https://rclone.org/commands/rclone_config_delete/) command for more information on the above.

**Authentication is required for this call.**

### config/dump: Dumps the config file. {#config-dump}

Returns a JSON object:
- key: value

Where keys are remote names and values are the config parameters.

See the [config dump command](https://rclone.org/commands/rclone_config_dump/) command for more information on the above.

**Authentication is required for this call.**

### config/get: Get a remote in the config file. {#config-get}

Parameters:

- name - name of remote to get

See the [config dump command](https://rclone.org/commands/rclone_config_dump/) command for more information on the above.

**Authentication is required for this call.**

### config/listremotes: Lists the remotes in the config file. {#config-listremotes}

Returns
- remotes - array of remote names

See the [listremotes command](https://rclone.org/commands/rclone_listremotes/) command for more information on the above.

**Authentication is required for this call.**

### config/password: password the config for a remote. {#config-password}

This takes the following parameters

- name - name of remote
- parameters - a map of \{ "key": "value" \} pairs


See the [config password command](https://rclone.org/commands/rclone_config_password/) command for more information on the above.

**Authentication is required for this call.**

### config/providers: Shows how providers are configured in the config file. {#config-providers}

Returns a JSON object:
- providers - array of objects

See the [config providers command](https://rclone.org/commands/rclone_config_providers/) command for more information on the above.

**Authentication is required for this call.**

### config/update: update the config for a remote. {#config-update}

This takes the following parameters

- name - name of remote
- parameters - a map of \{ "key": "value" \} pairs
- obscure - optional bool - forces obscuring of passwords
- noObscure - optional bool - forces passwords not to be obscured


See the [config update command](https://rclone.org/commands/rclone_config_update/) command for more information on the above.

**Authentication is required for this call.**

### core/bwlimit: Set the bandwidth limit. {#core-bwlimit}

This sets the bandwidth limit to that passed in.

Eg

    rclone rc core/bwlimit rate=off
    {
        "bytesPerSecond": -1,
        "rate": "off"
    }
    rclone rc core/bwlimit rate=1M
    {
        "bytesPerSecond": 1048576,
        "rate": "1M"
    }


If the rate parameter is not supplied then the bandwidth is queried

    rclone rc core/bwlimit
    {
        "bytesPerSecond": 1048576,
        "rate": "1M"
    }

The format of the parameter is exactly the same as passed to --bwlimit
except only one bandwidth may be specified.

In either case "rate" is returned as a human readable string, and
"bytesPerSecond" is returned as a number.

### core/gc: Runs a garbage collection. {#core-gc}

This tells the go runtime to do a garbage collection run.  It isn't
necessary to call this normally, but it can be useful for debugging
memory problems.

### core/group-list: Returns list of stats. {#core-group-list}

This returns list of stats groups currently in memory. 

Returns the following values:
```
{
	"groups":  an array of group names:
		[
			"group1",
			"group2",
			...
		]
}
```

### core/memstats: Returns the memory statistics {#core-memstats}

This returns the memory statistics of the running program.  What the values mean
are explained in the go docs: https://golang.org/pkg/runtime/#MemStats

The most interesting values for most people are:

* HeapAlloc: This is the amount of memory rclone is actually using
* HeapSys: This is the amount of memory rclone has obtained from the OS
* Sys: this is the total amount of memory requested from the OS
  * It is virtual memory so may include unused memory

### core/obscure: Obscures a string passed in. {#core-obscure}

Pass a clear string and rclone will obscure it for the config file:
- clear - string

Returns
- obscured - string

### core/pid: Return PID of current process {#core-pid}

This returns PID of current process.
Useful for stopping rclone process.

### core/quit: Terminates the app. {#core-quit}

(optional) Pass an exit code to be used for terminating the app:
- exitCode - int

### core/stats: Returns stats about current transfers. {#core-stats}

This returns all available stats:

	rclone rc core/stats

If group is not provided then summed up stats for all groups will be
returned.

Parameters

- group - name of the stats group (string)

Returns the following values:

```
{
	"speed": average speed in bytes/sec since start of the process,
	"bytes": total transferred bytes since the start of the process,
	"errors": number of errors,
	"fatalError": whether there has been at least one FatalError,
	"retryError": whether there has been at least one non-NoRetryError,
	"checks": number of checked files,
	"transfers": number of transferred files,
	"deletes" : number of deleted files,
	"renames" : number of renamed files,
	"elapsedTime": time in seconds since the start of the process,
	"lastError": last occurred error,
	"transferring": an array of currently active file transfers:
		[
			{
				"bytes": total transferred bytes for this file,
				"eta": estimated time in seconds until file transfer completion
				"name": name of the file,
				"percentage": progress of the file transfer in percent,
				"speed": speed in bytes/sec,
				"speedAvg": speed in bytes/sec as an exponentially weighted moving average,
				"size": size of the file in bytes
			}
		],
	"checking": an array of names of currently active file checks
		[]
}
```
Values for "transferring", "checking" and "lastError" are only assigned if data is available.
The value for "eta" is null if an eta cannot be determined.

### core/stats-delete: Delete stats group. {#core-stats-delete}

This deletes entire stats group

Parameters

- group - name of the stats group (string)

### core/stats-reset: Reset stats. {#core-stats-reset}

This clears counters, errors and finished transfers for all stats or specific 
stats group if group is provided.

Parameters

- group - name of the stats group (string)

### core/transferred: Returns stats about completed transfers. {#core-transferred}

This returns stats about completed transfers:

	rclone rc core/transferred

If group is not provided then completed transfers for all groups will be
returned.

Note only the last 100 completed transfers are returned.

Parameters

- group - name of the stats group (string)

Returns the following values:
```
{
	"transferred":  an array of completed transfers (including failed ones):
		[
			{
				"name": name of the file,
				"size": size of the file in bytes,
				"bytes": total transferred bytes for this file,
				"checked": if the transfer is only checked (skipped, deleted),
				"timestamp": integer representing millisecond unix epoch,
				"error": string description of the error (empty if successful),
				"jobid": id of the job that this transfer belongs to
			}
		]
}
```

### core/version: Shows the current version of rclone and the go runtime. {#core-version}

This shows the current version of go and the go runtime

- version - rclone version, eg "v1.44"
- decomposed - version number as [major, minor, patch, subpatch]
    - note patch and subpatch will be 999 for a git compiled version
- isGit - boolean - true if this was compiled from the git version
- os - OS in use as according to Go
- arch - cpu architecture in use according to Go
- goVersion - version of Go runtime in use

### debug/set-block-profile-rate: Set runtime.SetBlockProfileRate for blocking profiling. {#debug-set-block-profile-rate}

SetBlockProfileRate controls the fraction of goroutine blocking events
that are reported in the blocking profile. The profiler aims to sample
an average of one blocking event per rate nanoseconds spent blocked.

To include every blocking event in the profile, pass rate = 1. To turn
off profiling entirely, pass rate <= 0.

After calling this you can use this to see the blocking profile:

    go tool pprof http://localhost:5572/debug/pprof/block

Parameters

- rate - int

### debug/set-mutex-profile-fraction: Set runtime.SetMutexProfileFraction for mutex profiling. {#debug-set-mutex-profile-fraction}

SetMutexProfileFraction controls the fraction of mutex contention
events that are reported in the mutex profile. On average 1/rate
events are reported. The previous rate is returned.

To turn off profiling entirely, pass rate 0. To just read the current
rate, pass rate < 0. (For n>1 the details of sampling may change.)

Once this is set you can look use this to profile the mutex contention:

    go tool pprof http://localhost:5572/debug/pprof/mutex

Parameters

- rate - int

Results

- previousRate - int

### job/list: Lists the IDs of the running jobs {#job-list}

Parameters - None

Results

- jobids - array of integer job ids

### job/status: Reads the status of the job ID {#job-status}

Parameters

- jobid - id of the job (integer)

Results

- finished - boolean
- duration - time in seconds that the job ran for
- endTime - time the job finished (eg "2018-10-26T18:50:20.528746884+01:00")
- error - error from the job or empty string for no error
- finished - boolean whether the job has finished or not
- id - as passed in above
- startTime - time the job started (eg "2018-10-26T18:50:20.528336039+01:00")
- success - boolean - true for success false otherwise
- output - output of the job as would have been returned if called synchronously
- progress - output of the progress related to the underlying job

### job/stop: Stop the running job {#job-stop}

Parameters

- jobid - id of the job (integer)

### mount/mount: Create a new mount point {#mount-mount}

rclone allows Linux, FreeBSD, macOS and Windows to mount any of
Rclone's cloud storage systems as a file system with FUSE.

If no mountType is provided, the priority is given as follows: 1. mount 2.cmount 3.mount2

This takes the following parameters

- fs - a remote path to be mounted (required)
- mountPoint: valid path on the local machine (required)
- mountType: One of the values (mount, cmount, mount2) specifies the mount implementation to use

Eg

    rclone rc mount/mount fs=mydrive: mountPoint=/home/<user>/mountPoint
    rclone rc mount/mount fs=mydrive: mountPoint=/home/<user>/mountPoint mountType=mount

**Authentication is required for this call.**

### mount/types: Show all possible mount types {#mount-types}

This shows all possible mount types and returns them as a list.

This takes no parameters and returns

- mountTypes: list of mount types

The mount types are strings like "mount", "mount2", "cmount" and can
be passed to mount/mount as the mountType parameter.

Eg

    rclone rc mount/types

**Authentication is required for this call.**

### mount/unmount: Unmount all active mounts {#mount-unmount}

rclone allows Linux, FreeBSD, macOS and Windows to
mount any of Rclone's cloud storage systems as a file system with
FUSE.

This takes the following parameters

- mountPoint: valid path on the local machine where the mount was created (required)

Eg

    rclone rc mount/unmount mountPoint=/home/<user>/mountPoint

**Authentication is required for this call.**

### operations/about: Return the space used on the remote {#operations-about}

This takes the following parameters

- fs - a remote name string eg "drive:"

The result is as returned from rclone about --json

See the [about command](https://rclone.org/commands/rclone_size/) command for more information on the above.

**Authentication is required for this call.**

### operations/cleanup: Remove trashed files in the remote or path {#operations-cleanup}

This takes the following parameters

- fs - a remote name string eg "drive:"

See the [cleanup command](https://rclone.org/commands/rclone_cleanup/) command for more information on the above.

**Authentication is required for this call.**

### operations/copyfile: Copy a file from source remote to destination remote {#operations-copyfile}

This takes the following parameters

- srcFs - a remote name string eg "drive:" for the source
- srcRemote - a path within that remote eg "file.txt" for the source
- dstFs - a remote name string eg "drive2:" for the destination
- dstRemote - a path within that remote eg "file2.txt" for the destination

**Authentication is required for this call.**

### operations/copyurl: Copy the URL to the object {#operations-copyurl}

This takes the following parameters

- fs - a remote name string eg "drive:"
- remote - a path within that remote eg "dir"
- url - string, URL to read from
 - autoFilename - boolean, set to true to retrieve destination file name from url
See the [copyurl command](https://rclone.org/commands/rclone_copyurl/) command for more information on the above.

**Authentication is required for this call.**

### operations/delete: Remove files in the path {#operations-delete}

This takes the following parameters

- fs - a remote name string eg "drive:"

See the [delete command](https://rclone.org/commands/rclone_delete/) command for more information on the above.

**Authentication is required for this call.**

### operations/deletefile: Remove the single file pointed to {#operations-deletefile}

This takes the following parameters

- fs - a remote name string eg "drive:"
- remote - a path within that remote eg "dir"

See the [deletefile command](https://rclone.org/commands/rclone_deletefile/) command for more information on the above.

**Authentication is required for this call.**

### operations/fsinfo: Return information about the remote {#operations-fsinfo}

This takes the following parameters

- fs - a remote name string eg "drive:"

This returns info about the remote passed in;

```
{
	// optional features and whether they are available or not
	"Features": {
		"About": true,
		"BucketBased": false,
		"CanHaveEmptyDirectories": true,
		"CaseInsensitive": false,
		"ChangeNotify": false,
		"CleanUp": false,
		"Copy": false,
		"DirCacheFlush": false,
		"DirMove": true,
		"DuplicateFiles": false,
		"GetTier": false,
		"ListR": false,
		"MergeDirs": false,
		"Move": true,
		"OpenWriterAt": true,
		"PublicLink": false,
		"Purge": true,
		"PutStream": true,
		"PutUnchecked": false,
		"ReadMimeType": false,
		"ServerSideAcrossConfigs": false,
		"SetTier": false,
		"SetWrapper": false,
		"UnWrap": false,
		"WrapFs": false,
		"WriteMimeType": false
	},
	// Names of hashes available
	"Hashes": [
		"MD5",
		"SHA-1",
		"DropboxHash",
		"QuickXorHash"
	],
	"Name": "local",	// Name as created
	"Precision": 1,		// Precision of timestamps in ns
	"Root": "/",		// Path as created
	"String": "Local file system at /" // how the remote will appear in logs
}
```

This command does not have a command line equivalent so use this instead:

    rclone rc --loopback operations/fsinfo fs=remote:

### operations/list: List the given remote and path in JSON format {#operations-list}

This takes the following parameters

- fs - a remote name string eg "drive:"
- remote - a path within that remote eg "dir"
- opt - a dictionary of options to control the listing (optional)
    - recurse - If set recurse directories
    - noModTime - If set return modification time
    - showEncrypted -  If set show decrypted names
    - showOrigIDs - If set show the IDs for each item if known
    - showHash - If set return a dictionary of hashes

The result is

- list
    - This is an array of objects as described in the lsjson command

See the [lsjson command](https://rclone.org/commands/rclone_lsjson/) for more information on the above and examples.

**Authentication is required for this call.**

### operations/mkdir: Make a destination directory or container {#operations-mkdir}

This takes the following parameters

- fs - a remote name string eg "drive:"
- remote - a path within that remote eg "dir"

See the [mkdir command](https://rclone.org/commands/rclone_mkdir/) command for more information on the above.

**Authentication is required for this call.**

### operations/movefile: Move a file from source remote to destination remote {#operations-movefile}

This takes the following parameters

- srcFs - a remote name string eg "drive:" for the source
- srcRemote - a path within that remote eg "file.txt" for the source
- dstFs - a remote name string eg "drive2:" for the destination
- dstRemote - a path within that remote eg "file2.txt" for the destination

**Authentication is required for this call.**

### operations/publiclink: Create or retrieve a public link to the given file or folder. {#operations-publiclink}

This takes the following parameters

- fs - a remote name string eg "drive:"
- remote - a path within that remote eg "dir"

Returns

- url - URL of the resource

See the [link command](https://rclone.org/commands/rclone_link/) command for more information on the above.

**Authentication is required for this call.**

### operations/purge: Remove a directory or container and all of its contents {#operations-purge}

This takes the following parameters

- fs - a remote name string eg "drive:"
- remote - a path within that remote eg "dir"

See the [purge command](https://rclone.org/commands/rclone_purge/) command for more information on the above.

**Authentication is required for this call.**

### operations/rmdir: Remove an empty directory or container {#operations-rmdir}

This takes the following parameters

- fs - a remote name string eg "drive:"
- remote - a path within that remote eg "dir"

See the [rmdir command](https://rclone.org/commands/rclone_rmdir/) command for more information on the above.

**Authentication is required for this call.**

### operations/rmdirs: Remove all the empty directories in the path {#operations-rmdirs}

This takes the following parameters

- fs - a remote name string eg "drive:"
- remote - a path within that remote eg "dir"
- leaveRoot - boolean, set to true not to delete the root

See the [rmdirs command](https://rclone.org/commands/rclone_rmdirs/) command for more information on the above.

**Authentication is required for this call.**

### operations/size: Count the number of bytes and files in remote {#operations-size}

This takes the following parameters

- fs - a remote name string eg "drive:path/to/dir"

Returns

- count - number of files
- bytes - number of bytes in those files

See the [size command](https://rclone.org/commands/rclone_size/) command for more information on the above.

**Authentication is required for this call.**

### options/blocks: List all the option blocks {#options-blocks}

Returns
- options - a list of the options block names

### options/get: Get all the options {#options-get}

Returns an object where keys are option block names and values are an
object with the current option values in.

This shows the internal names of the option within rclone which should
map to the external options very easily with a few exceptions.

### options/set: Set an option {#options-set}

Parameters

- option block name containing an object with
  - key: value

Repeated as often as required.

Only supply the options you wish to change.  If an option is unknown
it will be silently ignored.  Not all options will have an effect when
changed like this.

For example:

This sets DEBUG level logs (-vv)

    rclone rc options/set --json '{"main": {"LogLevel": 8}}'

And this sets INFO level logs (-v)

    rclone rc options/set --json '{"main": {"LogLevel": 7}}'

And this sets NOTICE level logs (normal without -v)

    rclone rc options/set --json '{"main": {"LogLevel": 6}}'

### rc/error: This returns an error {#rc-error}

This returns an error with the input as part of its error string.
Useful for testing error handling.

### rc/list: List all the registered remote control commands {#rc-list}

This lists all the registered remote control commands as a JSON map in
the commands response.

### rc/noop: Echo the input to the output parameters {#rc-noop}

This echoes the input parameters to the output parameters for testing
purposes.  It can be used to check that rclone is still alive and to
check that parameter passing is working properly.

### rc/noopauth: Echo the input to the output parameters requiring auth {#rc-noopauth}

This echoes the input parameters to the output parameters for testing
purposes.  It can be used to check that rclone is still alive and to
check that parameter passing is working properly.

**Authentication is required for this call.**

### sync/copy: copy a directory from source remote to destination remote {#sync-copy}

This takes the following parameters

- srcFs - a remote name string eg "drive:src" for the source
- dstFs - a remote name string eg "drive:dst" for the destination


See the [copy command](https://rclone.org/commands/rclone_copy/) command for more information on the above.

**Authentication is required for this call.**

### sync/move: move a directory from source remote to destination remote {#sync-move}

This takes the following parameters

- srcFs - a remote name string eg "drive:src" for the source
- dstFs - a remote name string eg "drive:dst" for the destination
- deleteEmptySrcDirs - delete empty src directories if set


See the [move command](https://rclone.org/commands/rclone_move/) command for more information on the above.

**Authentication is required for this call.**

### sync/sync: sync a directory from source remote to destination remote {#sync-sync}

This takes the following parameters

- srcFs - a remote name string eg "drive:src" for the source
- dstFs - a remote name string eg "drive:dst" for the destination


See the [sync command](https://rclone.org/commands/rclone_sync/) command for more information on the above.

**Authentication is required for this call.**

### vfs/forget: Forget files or directories in the directory cache. {#vfs-forget}

This forgets the paths in the directory cache causing them to be
re-read from the remote when needed.

If no paths are passed in then it will forget all the paths in the
directory cache.

    rclone rc vfs/forget

Otherwise pass files or dirs in as file=path or dir=path.  Any
parameter key starting with file will forget that file and any
starting with dir will forget that dir, eg

    rclone rc vfs/forget file=hello file2=goodbye dir=home/junk

### vfs/poll-interval: Get the status or update the value of the poll-interval option. {#vfs-poll-interval}

Without any parameter given this returns the current status of the
poll-interval setting.

When the interval=duration parameter is set, the poll-interval value
is updated and the polling function is notified.
Setting interval=0 disables poll-interval.

    rclone rc vfs/poll-interval interval=5m

The timeout=duration parameter can be used to specify a time to wait
for the current poll function to apply the new value.
If timeout is less or equal 0, which is the default, wait indefinitely.

The new poll-interval value will only be active when the timeout is
not reached.

If poll-interval is updated or disabled temporarily, some changes
might not get picked up by the polling function, depending on the
used remote.

### vfs/refresh: Refresh the directory cache. {#vfs-refresh}

This reads the directories for the specified paths and freshens the
directory cache.

If no paths are passed in then it will refresh the root directory.

    rclone rc vfs/refresh

Otherwise pass directories in as dir=path. Any parameter key
starting with dir will refresh that directory, eg

    rclone rc vfs/refresh dir=home/junk dir2=data/misc

If the parameter recursive=true is given the whole directory tree
will get refreshed. This refresh will use --fast-list if enabled.



## Accessing the remote control via HTTP

Rclone implements a simple HTTP based protocol.

Each endpoint takes an JSON object and returns a JSON object or an
error.  The JSON objects are essentially a map of string names to
values.

All calls must made using POST.

The input objects can be supplied using URL parameters, POST
parameters or by supplying "Content-Type: application/json" and a JSON
blob in the body.  There are examples of these below using `curl`.

The response will be a JSON blob in the body of the response.  This is
formatted to be reasonably human readable.

### Error returns

If an error occurs then there will be an HTTP error status (eg 500)
and the body of the response will contain a JSON encoded error object,
eg

```
{
    "error": "Expecting string value for key \"remote\" (was float64)",
    "input": {
        "fs": "/tmp",
        "remote": 3
    },
    "status": 400
    "path": "operations/rmdir",
}
```

The keys in the error response are
- error - error string
- input - the input parameters to the call
- status - the HTTP status code
- path - the path of the call

### CORS

The sever implements basic CORS support and allows all origins for that.
The response to a preflight OPTIONS request will echo the requested "Access-Control-Request-Headers" back.

### Using POST with URL parameters only

```
curl -X POST 'http://localhost:5572/rc/noop?potato=1&sausage=2'
```

Response

```
{
	"potato": "1",
	"sausage": "2"
}
```

Here is what an error response looks like:

```
curl -X POST 'http://localhost:5572/rc/error?potato=1&sausage=2'
```

```
{
	"error": "arbitrary error on input map[potato:1 sausage:2]",
	"input": {
		"potato": "1",
		"sausage": "2"
	}
}
```

Note that curl doesn't return errors to the shell unless you use the `-f` option

```
$ curl -f -X POST 'http://localhost:5572/rc/error?potato=1&sausage=2'
curl: (22) The requested URL returned error: 400 Bad Request
$ echo $?
22
```

### Using POST with a form

```
curl --data "potato=1" --data "sausage=2" http://localhost:5572/rc/noop
```

Response

```
{
	"potato": "1",
	"sausage": "2"
}
```

Note that you can combine these with URL parameters too with the POST
parameters taking precedence.

```
curl --data "potato=1" --data "sausage=2" "http://localhost:5572/rc/noop?rutabaga=3&sausage=4"
```

Response

```
{
	"potato": "1",
	"rutabaga": "3",
	"sausage": "4"
}

```

### Using POST with a JSON blob

```
curl -H "Content-Type: application/json" -X POST -d '{"potato":2,"sausage":1}' http://localhost:5572/rc/noop
```

response

```
{
	"password": "xyz",
	"username": "xyz"
}
```

This can be combined with URL parameters too if required.  The JSON
blob takes precedence.

```
curl -H "Content-Type: application/json" -X POST -d '{"potato":2,"sausage":1}' 'http://localhost:5572/rc/noop?rutabaga=3&potato=4'
```

```
{
	"potato": 2,
	"rutabaga": "3",
	"sausage": 1
}
```

## Debugging rclone with pprof ##

If you use the `--rc` flag this will also enable the use of the go
profiling tools on the same port.

To use these, first [install go](https://golang.org/doc/install).

### Debugging memory use

To profile rclone's memory use you can run:

    go tool pprof -web http://localhost:5572/debug/pprof/heap

This should open a page in your browser showing what is using what
memory.

You can also use the `-text` flag to produce a textual summary

```
$ go tool pprof -text http://localhost:5572/debug/pprof/heap
Showing nodes accounting for 1537.03kB, 100% of 1537.03kB total
      flat  flat%   sum%        cum   cum%
 1024.03kB 66.62% 66.62%  1024.03kB 66.62%  github.com/rclone/rclone/vendor/golang.org/x/net/http2/hpack.addDecoderNode
     513kB 33.38%   100%      513kB 33.38%  net/http.newBufioWriterSize
         0     0%   100%  1024.03kB 66.62%  github.com/rclone/rclone/cmd/all.init
         0     0%   100%  1024.03kB 66.62%  github.com/rclone/rclone/cmd/serve.init
         0     0%   100%  1024.03kB 66.62%  github.com/rclone/rclone/cmd/serve/restic.init
         0     0%   100%  1024.03kB 66.62%  github.com/rclone/rclone/vendor/golang.org/x/net/http2.init
         0     0%   100%  1024.03kB 66.62%  github.com/rclone/rclone/vendor/golang.org/x/net/http2/hpack.init
         0     0%   100%  1024.03kB 66.62%  github.com/rclone/rclone/vendor/golang.org/x/net/http2/hpack.init.0
         0     0%   100%  1024.03kB 66.62%  main.init
         0     0%   100%      513kB 33.38%  net/http.(*conn).readRequest
         0     0%   100%      513kB 33.38%  net/http.(*conn).serve
         0     0%   100%  1024.03kB 66.62%  runtime.main
```

### Debugging go routine leaks

Memory leaks are most often caused by go routine leaks keeping memory
alive which should have been garbage collected.

See all active go routines using

    curl http://localhost:5572/debug/pprof/goroutine?debug=1

Or go to http://localhost:5572/debug/pprof/goroutine?debug=1 in your browser.

### Other profiles to look at

You can see a summary of profiles available at http://localhost:5572/debug/pprof/

Here is how to use some of them:

- Memory: `go tool pprof http://localhost:5572/debug/pprof/heap`
- Go routines: `curl http://localhost:5572/debug/pprof/goroutine?debug=1`
- 30-second CPU profile: `go tool pprof http://localhost:5572/debug/pprof/profile`
- 5-second execution trace: `wget http://localhost:5572/debug/pprof/trace?seconds=5`
- Goroutine blocking profile
    - Enable first with: `rclone rc debug/set-block-profile-rate rate=1` ([docs](#debug/set-block-profile-rate))
    - `go tool pprof http://localhost:5572/debug/pprof/block`
- Contended mutexes:
    - Enable first with: `rclone rc debug/set-mutex-profile-fraction rate=1` ([docs](#debug/set-mutex-profile-fraction))
    - `go tool pprof http://localhost:5572/debug/pprof/mutex`

See the [net/http/pprof docs](https://golang.org/pkg/net/http/pprof/)
for more info on how to use the profiling and for a general overview
see [the Go team's blog post on profiling go programs](https://blog.golang.org/profiling-go-programs).

The profiling hook is [zero overhead unless it is used](https://stackoverflow.com/q/26545159/164234).

# Overview of cloud storage systems #

Each cloud storage system is slightly different.  Rclone attempts to
provide a unified interface to them, but some underlying differences
show through.

## Features ##

Here is an overview of the major features of each cloud storage system.

| Name                         | Hash        | ModTime | Case Insensitive | Duplicate Files | MIME Type |
| ---------------------------- |:-----------:|:-------:|:----------------:|:---------------:|:---------:|
| 1Fichier                     | Whirlpool   | No      | No               | Yes             | R         |
| Amazon Drive                 | MD5         | No      | Yes              | No              | R         |
| Amazon S3                    | MD5         | Yes     | No               | No              | R/W       |
| Backblaze B2                 | SHA1        | Yes     | No               | No              | R/W       |
| Box                          | SHA1        | Yes     | Yes              | No              | -         |
| Citrix ShareFile             | MD5         | Yes     | Yes              | No              | -         |
| Dropbox                      | DBHASH †    | Yes     | Yes              | No              | -         |
| FTP                          | -           | No      | No               | No              | -         |
| Google Cloud Storage         | MD5         | Yes     | No               | No              | R/W       |
| Google Drive                 | MD5         | Yes     | No               | Yes             | R/W       |
| Google Photos                | -           | No      | No               | Yes             | R         |
| HTTP                         | -           | No      | No               | No              | R         |
| Hubic                        | MD5         | Yes     | No               | No              | R/W       |
| Jottacloud                   | MD5         | Yes     | Yes              | No              | R/W       |
| Koofr                        | MD5         | No      | Yes              | No              | -         |
| Mail.ru Cloud                | Mailru ‡‡‡  | Yes     | Yes              | No              | -         |
| Mega                         | -           | No      | No               | Yes             | -         |
| Memory                       | MD5         | Yes     | No               | No              | -         |
| Microsoft Azure Blob Storage | MD5         | Yes     | No               | No              | R/W       |
| Microsoft OneDrive           | SHA1 ‡‡     | Yes     | Yes              | No              | R         |
| OpenDrive                    | MD5         | Yes     | Yes              | No              | -         |
| OpenStack Swift              | MD5         | Yes     | No               | No              | R/W       |
| pCloud                       | MD5, SHA1   | Yes     | No               | No              | W         |
| premiumize.me                | -           | No      | Yes              | No              | R         |
| put.io                       | CRC-32      | Yes     | No               | Yes             | R         |
| QingStor                     | MD5         | No      | No               | No              | R/W       |
| Seafile                      | -           | No      | No               | No              | -         |
| SFTP                         | MD5, SHA1 ‡ | Yes     | Depends          | No              | -         |
| SugarSync                    | -           | No      | No               | No              | -         |
| Tardigrade                   | -           | Yes     | No               | No              | -         |
| WebDAV                       | MD5, SHA1 ††| Yes ††† | Depends          | No              | -         |
| Yandex Disk                  | MD5         | Yes     | No               | No              | R/W       |
| The local filesystem         | All         | Yes     | Depends          | No              | -         |

### Hash ###

The cloud storage system supports various hash types of the objects.
The hashes are used when transferring data as an integrity check and
can be specifically used with the `--checksum` flag in syncs and in
the `check` command.

To use the verify checksums when transferring between cloud storage
systems they must support a common hash type.

† Note that Dropbox supports [its own custom
hash](https://www.dropbox.com/developers/reference/content-hash).
This is an SHA256 sum of all the 4MB block SHA256s.

‡ SFTP supports checksums if the same login has shell access and `md5sum`
or `sha1sum` as well as `echo` are in the remote's PATH.

†† WebDAV supports hashes when used with Owncloud and Nextcloud only.

††† WebDAV supports modtimes when used with Owncloud and Nextcloud only.

‡‡ Microsoft OneDrive Personal supports SHA1 hashes, whereas OneDrive
for business and SharePoint server support Microsoft's own
[QuickXorHash](https://docs.microsoft.com/en-us/onedrive/developer/code-snippets/quickxorhash).

‡‡‡ Mail.ru uses its own modified SHA1 hash

### ModTime ###

The cloud storage system supports setting modification times on
objects.  If it does then this enables a using the modification times
as part of the sync.  If not then only the size will be checked by
default, though the MD5SUM can be checked with the `--checksum` flag.

All cloud storage systems support some kind of date on the object and
these will be set when transferring from the cloud storage system.

### Case Insensitive ###

If a cloud storage systems is case sensitive then it is possible to
have two files which differ only in case, eg `file.txt` and
`FILE.txt`.  If a cloud storage system is case insensitive then that
isn't possible.

This can cause problems when syncing between a case insensitive
system and a case sensitive system.  The symptom of this is that no
matter how many times you run the sync it never completes fully.

The local filesystem and SFTP may or may not be case sensitive
depending on OS.

  * Windows - usually case insensitive, though case is preserved
  * OSX - usually case insensitive, though it is possible to format case sensitive
  * Linux - usually case sensitive, but there are case insensitive file systems (eg FAT formatted USB keys)

Most of the time this doesn't cause any problems as people tend to
avoid files whose name differs only by case even on case sensitive
systems.

### Duplicate files ###

If a cloud storage system allows duplicate files then it can have two
objects with the same name.

This confuses rclone greatly when syncing - use the `rclone dedupe`
command to rename or remove duplicates.

### Restricted filenames ###

Some cloud storage systems might have restrictions on the characters
that are usable in file or directory names.
When `rclone` detects such a name during a file upload, it will
transparently replace the restricted characters with similar looking
Unicode characters.

This process is designed to avoid ambiguous file names as much as
possible and allow to move files between many cloud storage systems
transparently.

The name shown by `rclone` to the user or during log output will only
contain a minimal set of [replaced characters](#restricted-characters)
to ensure correct formatting and not necessarily the actual name used
on the cloud storage.

This transformation is reversed when downloading a file or parsing
`rclone` arguments.
For example, when uploading a file named `my file?.txt` to Onedrive
will be displayed as `my file?.txt` on the console, but stored as
`my file？.txt` (the `?` gets replaced by the similar looking `？`
character) to Onedrive.
The reverse transformation allows to read a file`unusual/name.txt`
from Google Drive, by passing the name `unusual／name.txt` (the `/` needs
to be replaced by the similar looking `／` character) on the command line.

#### Default restricted characters {#restricted-characters}

The table below shows the characters that are replaced by default.

When a replacement character is found in a filename, this character
will be escaped with the `‛` character to avoid ambiguous file names.
(e.g. a file named `␀.txt` would shown as `‛␀.txt`)

Each cloud storage backend can use a different set of characters,
which will be specified in the documentation for each backend.

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| SOH       | 0x01  | ␁           |
| STX       | 0x02  | ␂           |
| ETX       | 0x03  | ␃           |
| EOT       | 0x04  | ␄           |
| ENQ       | 0x05  | ␅           |
| ACK       | 0x06  | ␆           |
| BEL       | 0x07  | ␇           |
| BS        | 0x08  | ␈           |
| HT        | 0x09  | ␉           |
| LF        | 0x0A  | ␊           |
| VT        | 0x0B  | ␋           |
| FF        | 0x0C  | ␌           |
| CR        | 0x0D  | ␍           |
| SO        | 0x0E  | ␎           |
| SI        | 0x0F  | ␏           |
| DLE       | 0x10  | ␐           |
| DC1       | 0x11  | ␑           |
| DC2       | 0x12  | ␒           |
| DC3       | 0x13  | ␓           |
| DC4       | 0x14  | ␔           |
| NAK       | 0x15  | ␕           |
| SYN       | 0x16  | ␖           |
| ETB       | 0x17  | ␗           |
| CAN       | 0x18  | ␘           |
| EM        | 0x19  | ␙           |
| SUB       | 0x1A  | ␚           |
| ESC       | 0x1B  | ␛           |
| FS        | 0x1C  | ␜           |
| GS        | 0x1D  | ␝           |
| RS        | 0x1E  | ␞           |
| US        | 0x1F  | ␟           |
| /         | 0x2F  | ／           |
| DEL       | 0x7F  | ␡           |

The default encoding will also encode these file names as they are
problematic with many cloud storage systems.

| File name | Replacement |
| --------- |:-----------:|
| .         | ．          |
| ..        | ．．         |

#### Invalid UTF-8 bytes {#invalid-utf8}

Some backends only support a sequence of well formed UTF-8 bytes
as file or directory names.

In this case all invalid UTF-8 bytes will be replaced with a quoted
representation of the byte value to allow uploading a file to such a
backend. For example, the invalid byte `0xFE` will be encoded as `‛FE`.

A common source of invalid UTF-8 bytes are local filesystems, that store
names in a different encoding than UTF-8 or UTF-16, like latin1. See the
[local filenames](https://rclone.org/local/#filenames) section for details.

#### Encoding option {#encoding}

Most backends have an encoding options, specified as a flag
`--backend-encoding` where `backend` is the name of the backend, or as
a config parameter `encoding` (you'll need to select the Advanced
config in `rclone config` to see it).

This will have default value which encodes and decodes characters in
such a way as to preserve the maximum number of characters (see
above).

However this can be incorrect in some scenarios, for example if you
have a Windows file system with characters such as `＊` and `？` that
you want to remain as those characters on the remote rather than being
translated to `*` and `?`.

The `--backend-encoding` flags allow you to change that. You can
disable the encoding completely with `--backend-encoding None` or set
`encoding = None` in the config file.

Encoding takes a comma separated list of encodings. You can see the
list of all available characters by passing an invalid value to this
flag, eg `--local-encoding "help"` and `rclone help flags encoding`
will show you the defaults for the backends.

| Encoding  | Characters |
| --------- | ---------- |
| Asterisk | `*` |
| BackQuote | `` ` `` |
| BackSlash | `\` |
| Colon | `:` |
| CrLf | CR 0x0D, LF 0x0A |
| Ctl | All control characters 0x00-0x1F |
| Del | DEL 0x7F |
| Dollar | `$` |
| Dot | `.` |
| DoubleQuote | `"` |
| Hash | `#` |
| InvalidUtf8 | An invalid UTF-8 character (eg latin1) |
| LeftCrLfHtVt | CR 0x0D, LF 0x0A,HT 0x09, VT 0x0B on the left of a string |
| LeftPeriod | `.` on the left of a string |
| LeftSpace | SPACE on the left of a string |
| LeftTilde | `~` on the left of a string |
| LtGt | `<`, `>` |
| None | No characters are encoded |
| Percent | `%` |
| Pipe | \| |
| Question | `?` |
| RightCrLfHtVt | CR 0x0D, LF 0x0A, HT 0x09, VT 0x0B on the right of a string |
| RightPeriod | `.` on the right of a string |
| RightSpace | SPACE on the right of a string |
| SingleQuote | `'` |
| Slash | `/` |

To take a specific example, the FTP backend's default encoding is

    --ftp-encoding "Slash,Del,Ctl,RightSpace,Dot"

However, let's say the FTP server is running on Windows and can't have
any of the invalid Windows characters in file names. You are backing
up Linux servers to this FTP server which do have those characters in
file names. So you would add the Windows set which are

    Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,Ctl,RightSpace,RightPeriod,InvalidUtf8,Dot

to the existing ones, giving:

    Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,Ctl,RightSpace,RightPeriod,InvalidUtf8,Dot,Del,RightSpace

This can be specified using the `--ftp-encoding` flag or using an `encoding` parameter in the config file.

Or let's say you have a Windows server but you want to preserve `＊`
and `？`, you would then have this as the encoding (the Windows
encoding minus `Asterisk` and `Question`).

    Slash,LtGt,DoubleQuote,Colon,Pipe,BackSlash,Ctl,RightSpace,RightPeriod,InvalidUtf8,Dot

This can be specified using the `--local-encoding` flag or using an
`encoding` parameter in the config file.

### MIME Type ###

MIME types (also known as media types) classify types of documents
using a simple text classification, eg `text/html` or
`application/pdf`.

Some cloud storage systems support reading (`R`) the MIME type of
objects and some support writing (`W`) the MIME type of objects.

The MIME type can be important if you are serving files directly to
HTTP from the storage system.

If you are copying from a remote which supports reading (`R`) to a
remote which supports writing (`W`) then rclone will preserve the MIME
types.  Otherwise they will be guessed from the extension, or the
remote itself may assign the MIME type.

## Optional Features ##

All the remotes support a basic set of features, but there are some
optional features supported by some remotes used to make some
operations more efficient.

| Name                         | Purge | Copy | Move | DirMove | CleanUp | ListR | StreamUpload | LinkSharing | About | EmptyDir |
| ---------------------------- |:-----:|:----:|:----:|:-------:|:-------:|:-----:|:------------:|:------------:|:-----:| :------: |
| 1Fichier                     | No    | No   | No   | No      | No      | No    | No           | No           |   No  |  Yes |
| Amazon Drive                 | Yes   | No   | Yes  | Yes     | No [#575](https://github.com/rclone/rclone/issues/575) | No  | No  | No [#2178](https://github.com/rclone/rclone/issues/2178) | No  | Yes |
| Amazon S3                    | No    | Yes  | No   | No      | No      | Yes   | Yes          | No [#2178](https://github.com/rclone/rclone/issues/2178) | No  | No |
| Backblaze B2                 | No    | Yes  | No   | No      | Yes     | Yes   | Yes          | Yes | No  | No |
| Box                          | Yes   | Yes  | Yes  | Yes     | No [#575](https://github.com/rclone/rclone/issues/575) | No  | Yes | Yes | No  | Yes |
| Citrix ShareFile             | Yes   | Yes  | Yes  | Yes     | No      | No    | Yes          | No          | No  | Yes |
| Dropbox                      | Yes   | Yes  | Yes  | Yes     | No [#575](https://github.com/rclone/rclone/issues/575) | No  | Yes | Yes | Yes | Yes |
| FTP                          | No    | No   | Yes  | Yes     | No      | No    | Yes          | No [#2178](https://github.com/rclone/rclone/issues/2178) | No  | Yes |
| Google Cloud Storage         | Yes   | Yes  | No   | No      | No      | Yes   | Yes          | No [#2178](https://github.com/rclone/rclone/issues/2178) | No  | No |
| Google Drive                 | Yes   | Yes  | Yes  | Yes     | Yes     | Yes   | Yes          | Yes         | Yes | Yes |
| Google Photos                | No    | No   | No   | No      | No      | No    | No           | No          | No | No |
| HTTP                         | No    | No   | No   | No      | No      | No    | No           | No [#2178](https://github.com/rclone/rclone/issues/2178) | No  | Yes |
| Hubic                        | Yes † | Yes  | No   | No      | No      | Yes   | Yes          | No [#2178](https://github.com/rclone/rclone/issues/2178) | Yes | No |
| Jottacloud                   | Yes   | Yes  | Yes  | Yes     | Yes     | Yes   | No           | Yes                                                   | Yes | Yes |
| Mail.ru Cloud                | Yes   | Yes  | Yes  | Yes     | Yes     | No    | No           | Yes                                                   | Yes | Yes |
| Mega                         | Yes   | No   | Yes  | Yes     | Yes     | No    | No           | No [#2178](https://github.com/rclone/rclone/issues/2178) | Yes | Yes |
| Memory                       | No    | Yes  | No   | No      | No      | Yes   | Yes          | No          | No | No |
| Microsoft Azure Blob Storage | Yes   | Yes  | No   | No      | No      | Yes   | Yes          | No [#2178](https://github.com/rclone/rclone/issues/2178) | No  | No |
| Microsoft OneDrive           | Yes   | Yes  | Yes  | Yes     | No [#575](https://github.com/rclone/rclone/issues/575) | No | No | Yes | Yes | Yes |
| OpenDrive                    | Yes   | Yes  | Yes  | Yes     | No      | No    | No           | No                                                    | No  | Yes |
| OpenStack Swift              | Yes † | Yes  | No   | No      | No      | Yes   | Yes          | No [#2178](https://github.com/rclone/rclone/issues/2178) | Yes | No |
| pCloud                       | Yes   | Yes  | Yes  | Yes     | Yes     | No    | No           | No [#2178](https://github.com/rclone/rclone/issues/2178) | Yes | Yes |
| premiumize.me                | Yes   | No   | Yes  | Yes     | No      | No    | No           | Yes         | Yes | Yes |
| put.io                       | Yes   | No   | Yes  | Yes     | Yes     | No    | Yes          | No [#2178](https://github.com/rclone/rclone/issues/2178) | Yes | Yes |
| QingStor                     | No    | Yes  | No   | No      | Yes     | Yes   | No           | No [#2178](https://github.com/rclone/rclone/issues/2178) | No  | No |
| Seafile                      | Yes   | Yes  | Yes  | Yes     | Yes     | Yes   | Yes          | Yes         | Yes | Yes |
| SFTP                         | No    | No   | Yes  | Yes     | No      | No    | Yes          | No [#2178](https://github.com/rclone/rclone/issues/2178) | Yes  | Yes |
| SugarSync                    | Yes   | Yes  | Yes  | Yes     | No      | No    | Yes          | Yes         | No  | Yes |
| Tardigrade                   | Yes † | No   | No   | No      | No      | Yes   | Yes          | No          | No  | No  |
| WebDAV                       | Yes   | Yes  | Yes  | Yes     | No      | No    | Yes ‡        | No [#2178](https://github.com/rclone/rclone/issues/2178) | Yes  | Yes |
| Yandex Disk                  | Yes   | Yes  | Yes  | Yes     | Yes     | No    | Yes          | Yes         | Yes | Yes |
| The local filesystem         | Yes   | No   | Yes  | Yes     | No      | No    | Yes          | No          | Yes | Yes |

### Purge ###

This deletes a directory quicker than just deleting all the files in
the directory.

† Note Swift, Hubic, and Tardigrade implement this in order to delete
directory markers but they don't actually have a quicker way of deleting
files other than deleting them individually.

‡ StreamUpload is not supported with Nextcloud

### Copy ###

Used when copying an object to and from the same remote.  This known
as a server side copy so you can copy a file without downloading it
and uploading it again.  It is used if you use `rclone copy` or
`rclone move` if the remote doesn't support `Move` directly.

If the server doesn't support `Copy` directly then for copy operations
the file is downloaded then re-uploaded.

### Move ###

Used when moving/renaming an object on the same remote.  This is known
as a server side move of a file.  This is used in `rclone move` if the
server doesn't support `DirMove`.

If the server isn't capable of `Move` then rclone simulates it with
`Copy` then delete.  If the server doesn't support `Copy` then rclone
will download the file and re-upload it.

### DirMove ###

This is used to implement `rclone move` to move a directory if
possible.  If it isn't then it will use `Move` on each file (which
falls back to `Copy` then download and upload - see `Move` section).

### CleanUp ###

This is used for emptying the trash for a remote by `rclone cleanup`.

If the server can't do `CleanUp` then `rclone cleanup` will return an
error.

### ListR ###

The remote supports a recursive list to list all the contents beneath
a directory quickly.  This enables the `--fast-list` flag to work.
See the [rclone docs](https://rclone.org/docs/#fast-list) for more details.

### StreamUpload ###

Some remotes allow files to be uploaded without knowing the file size
in advance. This allows certain operations to work without spooling the
file to local disk first, e.g. `rclone rcat`.

### LinkSharing ###

Sets the necessary permissions on a file or folder and prints a link
that allows others to access them, even if they don't have an account
on the particular cloud provider.

### About ###

This is used to fetch quota information from the remote, like bytes
used/free/quota and bytes used in the trash.

This is also used to return the space used, available for `rclone mount`.

If the server can't do `About` then `rclone about` will return an
error.

### EmptyDir ###

The remote supports empty directories. See [Limitations](https://rclone.org/bugs/#limitations)
 for details. Most Object/Bucket based remotes do not support this.

# Global Flags

This describes the global flags available to every rclone command
split into two groups, non backend and backend flags.

## Non Backend Flags

These flags are available for every command.

```
      --ask-password                         Allow prompt for password for encrypted configuration. (default true)
      --auto-confirm                         If enabled, do not request console confirmation.
      --backup-dir string                    Make backups into hierarchy based in DIR.
      --bind string                          Local address to bind to for outgoing connections, IPv4, IPv6 or name.
      --buffer-size SizeSuffix               In memory buffer size when reading files for each --transfer. (default 16M)
      --bwlimit BwTimetable                  Bandwidth limit in kBytes/s, or use suffix b|k|M|G or a full timetable.
      --ca-cert string                       CA certificate used to verify servers
      --cache-dir string                     Directory rclone will use for caching. (default "$HOME/.cache/rclone")
      --check-first                          Do all the checks before starting transfers.
      --checkers int                         Number of checkers to run in parallel. (default 8)
  -c, --checksum                             Skip based on checksum (if available) & size, not mod-time & size
      --client-cert string                   Client SSL certificate (PEM) for mutual TLS auth
      --client-key string                    Client SSL private key (PEM) for mutual TLS auth
      --compare-dest string                  Include additional server-side path during comparison.
      --config string                        Config file. (default "$HOME/.config/rclone/rclone.conf")
      --contimeout duration                  Connect timeout (default 1m0s)
      --copy-dest string                     Implies --compare-dest but also copies files from path into destination.
      --cpuprofile string                    Write cpu profile to file
      --cutoff-mode string                   Mode to stop transfers when reaching the max transfer limit HARD|SOFT|CAUTIOUS (default "HARD")
      --delete-after                         When synchronizing, delete files on destination after transferring (default)
      --delete-before                        When synchronizing, delete files on destination before transferring
      --delete-during                        When synchronizing, delete files during transfer
      --delete-excluded                      Delete files on dest excluded from sync
      --disable string                       Disable a comma separated list of features.  Use help to see a list.
  -n, --dry-run                              Do a trial run with no permanent changes
      --dump DumpFlags                       List of items to dump from: headers,bodies,requests,responses,auth,filters,goroutines,openfiles
      --dump-bodies                          Dump HTTP headers and bodies - may contain sensitive info
      --dump-headers                         Dump HTTP headers - may contain sensitive info
      --error-on-no-transfer                 Sets exit code 9 if no files are transferred, useful in scripts
      --exclude stringArray                  Exclude files matching pattern
      --exclude-from stringArray             Read exclude patterns from file (use - to read from stdin)
      --exclude-if-present string            Exclude directories if filename is present
      --expect-continue-timeout duration     Timeout when using expect / 100-continue in HTTP (default 1s)
      --fast-list                            Use recursive list if available. Uses more memory but fewer transactions.
      --files-from stringArray               Read list of source-file names from file (use - to read from stdin)
      --files-from-raw stringArray           Read list of source-file names from file without any processing of lines (use - to read from stdin)
  -f, --filter stringArray                   Add a file-filtering rule
      --filter-from stringArray              Read filtering patterns from a file (use - to read from stdin)
      --header stringArray                   Set HTTP header for all transactions
      --header-download stringArray          Set HTTP header for download transactions
      --header-upload stringArray            Set HTTP header for upload transactions
      --ignore-case                          Ignore case in filters (case insensitive)
      --ignore-case-sync                     Ignore case when synchronizing
      --ignore-checksum                      Skip post copy check of checksums.
      --ignore-errors                        delete even if there are I/O errors
      --ignore-existing                      Skip all files that exist on destination
      --ignore-size                          Ignore size when skipping use mod-time or checksum.
  -I, --ignore-times                         Don't skip files that match size and time - transfer all files
      --immutable                            Do not modify files. Fail if existing files have been modified.
      --include stringArray                  Include files matching pattern
      --include-from stringArray             Read include patterns from file (use - to read from stdin)
      --log-file string                      Log everything to this file
      --log-format string                    Comma separated list of log format options (default "date,time")
      --log-level string                     Log level DEBUG|INFO|NOTICE|ERROR (default "NOTICE")
      --low-level-retries int                Number of low level retries to do. (default 10)
      --max-age Duration                     Only transfer files younger than this in s or suffix ms|s|m|h|d|w|M|y (default off)
      --max-backlog int                      Maximum number of objects in sync or check backlog. (default 10000)
      --max-delete int                       When synchronizing, limit the number of deletes (default -1)
      --max-depth int                        If set limits the recursion depth to this. (default -1)
      --max-duration duration                Maximum duration rclone will transfer data for.
      --max-size SizeSuffix                  Only transfer files smaller than this in k or suffix b|k|M|G (default off)
      --max-stats-groups int                 Maximum number of stats groups to keep in memory. On max oldest is discarded. (default 1000)
      --max-transfer SizeSuffix              Maximum size of data to transfer. (default off)
      --memprofile string                    Write memory profile to file
      --min-age Duration                     Only transfer files older than this in s or suffix ms|s|m|h|d|w|M|y (default off)
      --min-size SizeSuffix                  Only transfer files bigger than this in k or suffix b|k|M|G (default off)
      --modify-window duration               Max time diff to be considered the same (default 1ns)
      --multi-thread-cutoff SizeSuffix       Use multi-thread downloads for files above this size. (default 250M)
      --multi-thread-streams int             Max number of streams to use for multi-thread downloads. (default 4)
      --no-check-certificate                 Do not verify the server SSL certificate. Insecure.
      --no-check-dest                        Don't check the destination, copy regardless.
      --no-gzip-encoding                     Don't set Accept-Encoding: gzip.
      --no-traverse                          Don't traverse destination file system on copy.
      --no-unicode-normalization             Don't normalize unicode characters in filenames.
      --no-update-modtime                    Don't update destination mod-time if files identical.
      --order-by string                      Instructions on how to order the transfers, eg 'size,descending'
      --password-command SpaceSepList        Command for supplying password for encrypted configuration.
  -P, --progress                             Show progress during transfer.
  -q, --quiet                                Print as little stuff as possible
      --rc                                   Enable the remote control server.
      --rc-addr string                       IPaddress:Port or :Port to bind server to. (default "localhost:5572")
      --rc-allow-origin string               Set the allowed origin for CORS.
      --rc-baseurl string                    Prefix for URLs - leave blank for root.
      --rc-cert string                       SSL PEM key (concatenation of certificate and CA certificate)
      --rc-client-ca string                  Client certificate authority to verify clients with
      --rc-enable-metrics                    Enable prometheus metrics on /metrics
      --rc-files string                      Path to local files to serve on the HTTP server.
      --rc-htpasswd string                   htpasswd file - if not provided no authentication is done
      --rc-job-expire-duration duration      expire finished async jobs older than this value (default 1m0s)
      --rc-job-expire-interval duration      interval to check for expired async jobs (default 10s)
      --rc-key string                        SSL PEM Private key
      --rc-max-header-bytes int              Maximum size of request header (default 4096)
      --rc-no-auth                           Don't require auth for certain methods.
      --rc-pass string                       Password for authentication.
      --rc-realm string                      realm for authentication (default "rclone")
      --rc-serve                             Enable the serving of remote objects.
      --rc-server-read-timeout duration      Timeout for server reading data (default 1h0m0s)
      --rc-server-write-timeout duration     Timeout for server writing data (default 1h0m0s)
      --rc-template string                   User Specified Template.
      --rc-user string                       User name for authentication.
      --rc-web-fetch-url string              URL to fetch the releases for webgui. (default "https://api.github.com/repos/rclone/rclone-webui-react/releases/latest")
      --rc-web-gui                           Launch WebGUI on localhost
      --rc-web-gui-force-update              Force update to latest version of web gui
      --rc-web-gui-no-open-browser           Don't open the browser automatically
      --rc-web-gui-update                    Check and update to latest version of web gui
      --retries int                          Retry operations this many times if they fail (default 3)
      --retries-sleep duration               Interval between retrying operations if they fail, e.g 500ms, 60s, 5m. (0 to disable)
      --size-only                            Skip based on size only, not mod-time or checksum
      --stats duration                       Interval between printing stats, e.g 500ms, 60s, 5m. (0 to disable) (default 1m0s)
      --stats-file-name-length int           Max file name length in stats. 0 for no limit (default 45)
      --stats-log-level string               Log level to show --stats output DEBUG|INFO|NOTICE|ERROR (default "INFO")
      --stats-one-line                       Make the stats fit on one line.
      --stats-one-line-date                  Enables --stats-one-line and add current date/time prefix.
      --stats-one-line-date-format string    Enables --stats-one-line-date and uses custom formatted date. Enclose date string in double quotes ("). See https://golang.org/pkg/time/#Time.Format
      --stats-unit string                    Show data rate in stats as either 'bits' or 'bytes'/s (default "bytes")
      --streaming-upload-cutoff SizeSuffix   Cutoff for switching to chunked upload if file size is unknown. Upload starts after reaching cutoff or when file ends. (default 100k)
      --suffix string                        Suffix to add to changed files.
      --suffix-keep-extension                Preserve the extension when using --suffix.
      --syslog                               Use Syslog for logging
      --syslog-facility string               Facility for syslog, eg KERN,USER,... (default "DAEMON")
      --timeout duration                     IO idle timeout (default 5m0s)
      --tpslimit float                       Limit HTTP transactions per second to this.
      --tpslimit-burst int                   Max burst of transactions for --tpslimit. (default 1)
      --track-renames                        When synchronizing, track file renames and do a server side move if possible
      --track-renames-strategy string        Strategies to use when synchronizing using track-renames hash|modtime (default "hash")
      --transfers int                        Number of file transfers to run in parallel. (default 4)
  -u, --update                               Skip files that are newer on the destination.
      --use-cookies                          Enable session cookiejar.
      --use-json-log                         Use json log format.
      --use-mmap                             Use mmap allocator (see docs).
      --use-server-modtime                   Use server modified time instead of object metadata
      --user-agent string                    Set the user-agent to a specified string. The default is rclone/ version (default "rclone/v1.52.2")
  -v, --verbose count                        Print lots more stuff (repeat for more)
```

## Backend Flags

These flags are available for every command. They control the backends
and may be set in the config file.

```
      --acd-auth-url string                                      Auth server URL.
      --acd-client-id string                                     Amazon Application Client ID.
      --acd-client-secret string                                 Amazon Application Client Secret.
      --acd-encoding MultiEncoder                                This sets the encoding for the backend. (default Slash,InvalidUtf8,Dot)
      --acd-templink-threshold SizeSuffix                        Files >= this size will be downloaded via their tempLink. (default 9G)
      --acd-token-url string                                     Token server url.
      --acd-upload-wait-per-gb Duration                          Additional time per GB to wait after a failed complete upload to see if it appears. (default 3m0s)
      --alias-remote string                                      Remote or path to alias.
      --azureblob-access-tier string                             Access tier of blob: hot, cool or archive.
      --azureblob-account string                                 Storage Account Name (leave blank to use SAS URL or Emulator)
      --azureblob-chunk-size SizeSuffix                          Upload chunk size (<= 100MB). (default 4M)
      --azureblob-disable-checksum                               Don't store MD5 checksum with object metadata.
      --azureblob-encoding MultiEncoder                          This sets the encoding for the backend. (default Slash,BackSlash,Del,Ctl,RightPeriod,InvalidUtf8)
      --azureblob-endpoint string                                Endpoint for the service
      --azureblob-key string                                     Storage Account Key (leave blank to use SAS URL or Emulator)
      --azureblob-list-chunk int                                 Size of blob list. (default 5000)
      --azureblob-memory-pool-flush-time Duration                How often internal memory buffer pools will be flushed. (default 1m0s)
      --azureblob-memory-pool-use-mmap                           Whether to use mmap buffers in internal memory pool.
      --azureblob-sas-url string                                 SAS URL for container level access only
      --azureblob-upload-cutoff SizeSuffix                       Cutoff for switching to chunked upload (<= 256MB). (default 256M)
      --azureblob-use-emulator                                   Uses local storage emulator if provided as 'true' (leave blank if using real azure storage endpoint)
      --b2-account string                                        Account ID or Application Key ID
      --b2-chunk-size SizeSuffix                                 Upload chunk size. Must fit in memory. (default 96M)
      --b2-disable-checksum                                      Disable checksums for large (> upload cutoff) files
      --b2-download-auth-duration Duration                       Time before the authorization token will expire in s or suffix ms|s|m|h|d. (default 1w)
      --b2-download-url string                                   Custom endpoint for downloads.
      --b2-encoding MultiEncoder                                 This sets the encoding for the backend. (default Slash,BackSlash,Del,Ctl,InvalidUtf8,Dot)
      --b2-endpoint string                                       Endpoint for the service.
      --b2-hard-delete                                           Permanently delete files on remote removal, otherwise hide files.
      --b2-key string                                            Application Key
      --b2-test-mode string                                      A flag string for X-Bz-Test-Mode header for debugging.
      --b2-upload-cutoff SizeSuffix                              Cutoff for switching to chunked upload. (default 200M)
      --b2-versions                                              Include old versions in directory listings.
      --box-box-config-file string                               Box App config.json location
      --box-box-sub-type string                                   (default "user")
      --box-client-id string                                     Box App Client Id.
      --box-client-secret string                                 Box App Client Secret
      --box-commit-retries int                                   Max number of times to try committing a multipart file. (default 100)
      --box-encoding MultiEncoder                                This sets the encoding for the backend. (default Slash,BackSlash,Del,Ctl,RightSpace,InvalidUtf8,Dot)
      --box-root-folder-id string                                Fill in for rclone to use a non root folder as its starting point.
      --box-upload-cutoff SizeSuffix                             Cutoff for switching to multipart upload (>= 50MB). (default 50M)
      --cache-chunk-clean-interval Duration                      How often should the cache perform cleanups of the chunk storage. (default 1m0s)
      --cache-chunk-no-memory                                    Disable the in-memory cache for storing chunks during streaming.
      --cache-chunk-path string                                  Directory to cache chunk files. (default "$HOME/.cache/rclone/cache-backend")
      --cache-chunk-size SizeSuffix                              The size of a chunk (partial file data). (default 5M)
      --cache-chunk-total-size SizeSuffix                        The total size that the chunks can take up on the local disk. (default 10G)
      --cache-db-path string                                     Directory to store file structure metadata DB. (default "$HOME/.cache/rclone/cache-backend")
      --cache-db-purge                                           Clear all the cached data for this remote on start.
      --cache-db-wait-time Duration                              How long to wait for the DB to be available - 0 is unlimited (default 1s)
      --cache-info-age Duration                                  How long to cache file structure information (directory listings, file size, times etc). (default 6h0m0s)
      --cache-plex-insecure string                               Skip all certificate verification when connecting to the Plex server
      --cache-plex-password string                               The password of the Plex user (obscured)
      --cache-plex-url string                                    The URL of the Plex server
      --cache-plex-username string                               The username of the Plex user
      --cache-read-retries int                                   How many times to retry a read from a cache storage. (default 10)
      --cache-remote string                                      Remote to cache.
      --cache-rps int                                            Limits the number of requests per second to the source FS (-1 to disable) (default -1)
      --cache-tmp-upload-path string                             Directory to keep temporary files until they are uploaded.
      --cache-tmp-wait-time Duration                             How long should files be stored in local cache before being uploaded (default 15s)
      --cache-workers int                                        How many workers should run in parallel to download chunks. (default 4)
      --cache-writes                                             Cache file data on writes through the FS
      --chunker-chunk-size SizeSuffix                            Files larger than chunk size will be split in chunks. (default 2G)
      --chunker-fail-hard                                        Choose how chunker should handle files with missing or invalid chunks.
      --chunker-hash-type string                                 Choose how chunker handles hash sums. All modes but "none" require metadata. (default "md5")
      --chunker-meta-format string                               Format of the metadata object or "none". By default "simplejson". (default "simplejson")
      --chunker-name-format string                               String format of chunk file names. (default "*.rclone_chunk.###")
      --chunker-remote string                                    Remote to chunk/unchunk.
      --chunker-start-from int                                   Minimum valid chunk number. Usually 0 or 1. (default 1)
  -L, --copy-links                                               Follow symlinks and copy the pointed to item.
      --crypt-directory-name-encryption                          Option to either encrypt directory names or leave them intact. (default true)
      --crypt-filename-encryption string                         How to encrypt the filenames. (default "standard")
      --crypt-password string                                    Password or pass phrase for encryption. (obscured)
      --crypt-password2 string                                   Password or pass phrase for salt. Optional but recommended. (obscured)
      --crypt-remote string                                      Remote to encrypt/decrypt.
      --crypt-show-mapping                                       For all files listed show how the names encrypt.
      --drive-acknowledge-abuse                                  Set to allow files which return cannotDownloadAbusiveFile to be downloaded.
      --drive-allow-import-name-change                           Allow the filetype to change when uploading Google docs (e.g. file.doc to file.docx). This will confuse sync and reupload every time.
      --drive-alternate-export                                   Use alternate export URLs for google documents export.,
      --drive-auth-owner-only                                    Only consider files owned by the authenticated user.
      --drive-chunk-size SizeSuffix                              Upload chunk size. Must a power of 2 >= 256k. (default 8M)
      --drive-client-id string                                   Google Application Client Id
      --drive-client-secret string                               Google Application Client Secret
      --drive-disable-http2                                      Disable drive using http2 (default true)
      --drive-encoding MultiEncoder                              This sets the encoding for the backend. (default InvalidUtf8)
      --drive-export-formats string                              Comma separated list of preferred formats for downloading Google docs. (default "docx,xlsx,pptx,svg")
      --drive-formats string                                     Deprecated: see export_formats
      --drive-impersonate string                                 Impersonate this user when using a service account.
      --drive-import-formats string                              Comma separated list of preferred formats for uploading Google docs.
      --drive-keep-revision-forever                              Keep new head revision of each file forever.
      --drive-list-chunk int                                     Size of listing chunk 100-1000. 0 to disable. (default 1000)
      --drive-pacer-burst int                                    Number of API calls to allow without sleeping. (default 100)
      --drive-pacer-min-sleep Duration                           Minimum time to sleep between API calls. (default 100ms)
      --drive-root-folder-id string                              ID of the root folder
      --drive-scope string                                       Scope that rclone should use when requesting access from drive.
      --drive-server-side-across-configs                         Allow server side operations (eg copy) to work across different drive configs.
      --drive-service-account-credentials string                 Service Account Credentials JSON blob
      --drive-service-account-file string                        Service Account Credentials JSON file path
      --drive-shared-with-me                                     Only show files that are shared with me.
      --drive-size-as-quota                                      Show sizes as storage quota usage, not actual size.
      --drive-skip-checksum-gphotos                              Skip MD5 checksum on Google photos and videos only.
      --drive-skip-gdocs                                         Skip google documents in all listings.
      --drive-skip-shortcuts                                     If set skip shortcut files
      --drive-stop-on-upload-limit                               Make upload limit errors be fatal
      --drive-team-drive string                                  ID of the Team Drive
      --drive-trashed-only                                       Only show files that are in the trash.
      --drive-upload-cutoff SizeSuffix                           Cutoff for switching to chunked upload (default 8M)
      --drive-use-created-date                                   Use file created date instead of modified date.,
      --drive-use-shared-date                                    Use date file was shared instead of modified date.
      --drive-use-trash                                          Send files to the trash instead of deleting permanently. (default true)
      --drive-v2-download-min-size SizeSuffix                    If Object's are greater, use drive v2 API to download. (default off)
      --dropbox-chunk-size SizeSuffix                            Upload chunk size. (< 150M). (default 48M)
      --dropbox-client-id string                                 Dropbox App Client Id
      --dropbox-client-secret string                             Dropbox App Client Secret
      --dropbox-encoding MultiEncoder                            This sets the encoding for the backend. (default Slash,BackSlash,Del,RightSpace,InvalidUtf8,Dot)
      --dropbox-impersonate string                               Impersonate this user when using a business account.
      --fichier-api-key string                                   Your API Key, get it from https://1fichier.com/console/params.pl
      --fichier-encoding MultiEncoder                            This sets the encoding for the backend. (default Slash,LtGt,DoubleQuote,SingleQuote,BackQuote,Dollar,BackSlash,Del,Ctl,LeftSpace,RightSpace,InvalidUtf8,Dot)
      --fichier-shared-folder string                             If you want to download a shared folder, add this parameter
      --ftp-concurrency int                                      Maximum number of FTP simultaneous connections, 0 for unlimited
      --ftp-disable-epsv                                         Disable using EPSV even if server advertises support
      --ftp-encoding MultiEncoder                                This sets the encoding for the backend. (default Slash,Del,Ctl,RightSpace,Dot)
      --ftp-host string                                          FTP host to connect to
      --ftp-no-check-certificate                                 Do not verify the TLS certificate of the server
      --ftp-pass string                                          FTP password (obscured)
      --ftp-port string                                          FTP port, leave blank to use default (21)
      --ftp-tls                                                  Use FTP over TLS (Implicit)
      --ftp-user string                                          FTP username, leave blank for current username, $USER
      --gcs-bucket-acl string                                    Access Control List for new buckets.
      --gcs-bucket-policy-only                                   Access checks should use bucket-level IAM policies.
      --gcs-client-id string                                     Google Application Client Id
      --gcs-client-secret string                                 Google Application Client Secret
      --gcs-encoding MultiEncoder                                This sets the encoding for the backend. (default Slash,CrLf,InvalidUtf8,Dot)
      --gcs-location string                                      Location for the newly created buckets.
      --gcs-object-acl string                                    Access Control List for new objects.
      --gcs-project-number string                                Project number.
      --gcs-service-account-file string                          Service Account Credentials JSON file path
      --gcs-storage-class string                                 The storage class to use when storing objects in Google Cloud Storage.
      --gphotos-client-id string                                 Google Application Client Id
      --gphotos-client-secret string                             Google Application Client Secret
      --gphotos-read-only                                        Set to make the Google Photos backend read only.
      --gphotos-read-size                                        Set to read the size of media items.
      --gphotos-start-year int                                   Year limits the photos to be downloaded to those which are uploaded after the given year (default 2000)
      --http-headers CommaSepList                                Set HTTP headers for all transactions
      --http-no-head                                             Don't use HEAD requests to find file sizes in dir listing
      --http-no-slash                                            Set this if the site doesn't end directories with /
      --http-url string                                          URL of http host to connect to
      --hubic-chunk-size SizeSuffix                              Above this size files will be chunked into a _segments container. (default 5G)
      --hubic-client-id string                                   Hubic Client Id
      --hubic-client-secret string                               Hubic Client Secret
      --hubic-encoding MultiEncoder                              This sets the encoding for the backend. (default Slash,InvalidUtf8)
      --hubic-no-chunk                                           Don't chunk files during streaming upload.
      --jottacloud-encoding MultiEncoder                         This sets the encoding for the backend. (default Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,Del,Ctl,InvalidUtf8,Dot)
      --jottacloud-hard-delete                                   Delete files permanently rather than putting them into the trash.
      --jottacloud-md5-memory-limit SizeSuffix                   Files bigger than this will be cached on disk to calculate the MD5 if required. (default 10M)
      --jottacloud-trashed-only                                  Only show files that are in the trash.
      --jottacloud-unlink                                        Remove existing public link to file/folder with link command rather than creating.
      --jottacloud-upload-resume-limit SizeSuffix                Files bigger than this can be resumed if the upload fail's. (default 10M)
      --koofr-encoding MultiEncoder                              This sets the encoding for the backend. (default Slash,BackSlash,Del,Ctl,InvalidUtf8,Dot)
      --koofr-endpoint string                                    The Koofr API endpoint to use (default "https://app.koofr.net")
      --koofr-mountid string                                     Mount ID of the mount to use. If omitted, the primary mount is used.
      --koofr-password string                                    Your Koofr password for rclone (generate one at https://app.koofr.net/app/admin/preferences/password) (obscured)
      --koofr-setmtime                                           Does the backend support setting modification time. Set this to false if you use a mount ID that points to a Dropbox or Amazon Drive backend. (default true)
      --koofr-user string                                        Your Koofr user name
  -l, --links                                                    Translate symlinks to/from regular files with a '.rclonelink' extension
      --local-case-insensitive                                   Force the filesystem to report itself as case insensitive
      --local-case-sensitive                                     Force the filesystem to report itself as case sensitive.
      --local-encoding MultiEncoder                              This sets the encoding for the backend. (default Slash,Dot)
      --local-no-check-updated                                   Don't check to see if the files change during upload
      --local-no-sparse                                          Disable sparse files for multi-thread downloads
      --local-no-unicode-normalization                           Don't apply unicode normalization to paths and filenames (Deprecated)
      --local-nounc string                                       Disable UNC (long path names) conversion on Windows
      --mailru-check-hash                                        What should copy do if file checksum is mismatched or invalid (default true)
      --mailru-encoding MultiEncoder                             This sets the encoding for the backend. (default Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,Del,Ctl,InvalidUtf8,Dot)
      --mailru-pass string                                       Password (obscured)
      --mailru-speedup-enable                                    Skip full upload if there is another file with same data hash. (default true)
      --mailru-speedup-file-patterns string                      Comma separated list of file name patterns eligible for speedup (put by hash). (default "*.mkv,*.avi,*.mp4,*.mp3,*.zip,*.gz,*.rar,*.pdf")
      --mailru-speedup-max-disk SizeSuffix                       This option allows you to disable speedup (put by hash) for large files (default 3G)
      --mailru-speedup-max-memory SizeSuffix                     Files larger than the size given below will always be hashed on disk. (default 32M)
      --mailru-user string                                       User name (usually email)
      --mega-debug                                               Output more debug from Mega.
      --mega-encoding MultiEncoder                               This sets the encoding for the backend. (default Slash,InvalidUtf8,Dot)
      --mega-hard-delete                                         Delete files permanently rather than putting them into the trash.
      --mega-pass string                                         Password. (obscured)
      --mega-user string                                         User name
  -x, --one-file-system                                          Don't cross filesystem boundaries (unix/macOS only).
      --onedrive-chunk-size SizeSuffix                           Chunk size to upload files with - must be multiple of 320k (327,680 bytes). (default 10M)
      --onedrive-client-id string                                Microsoft App Client Id
      --onedrive-client-secret string                            Microsoft App Client Secret
      --onedrive-drive-id string                                 The ID of the drive to use
      --onedrive-drive-type string                               The type of the drive ( personal | business | documentLibrary )
      --onedrive-encoding MultiEncoder                           This sets the encoding for the backend. (default Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,Hash,Percent,BackSlash,Del,Ctl,LeftSpace,LeftTilde,RightSpace,RightPeriod,InvalidUtf8,Dot)
      --onedrive-expose-onenote-files                            Set to make OneNote files show up in directory listings.
      --onedrive-server-side-across-configs                      Allow server side operations (eg copy) to work across different onedrive configs.
      --opendrive-chunk-size SizeSuffix                          Files will be uploaded in chunks this size. (default 10M)
      --opendrive-encoding MultiEncoder                          This sets the encoding for the backend. (default Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,LeftSpace,LeftCrLfHtVt,RightSpace,RightCrLfHtVt,InvalidUtf8,Dot)
      --opendrive-password string                                Password. (obscured)
      --opendrive-username string                                Username
      --pcloud-client-id string                                  Pcloud App Client Id
      --pcloud-client-secret string                              Pcloud App Client Secret
      --pcloud-encoding MultiEncoder                             This sets the encoding for the backend. (default Slash,BackSlash,Del,Ctl,InvalidUtf8,Dot)
      --pcloud-root-folder-id string                             Fill in for rclone to use a non root folder as its starting point. (default "d0")
      --premiumizeme-encoding MultiEncoder                       This sets the encoding for the backend. (default Slash,DoubleQuote,BackSlash,Del,Ctl,InvalidUtf8,Dot)
      --putio-encoding MultiEncoder                              This sets the encoding for the backend. (default Slash,BackSlash,Del,Ctl,InvalidUtf8,Dot)
      --qingstor-access-key-id string                            QingStor Access Key ID
      --qingstor-chunk-size SizeSuffix                           Chunk size to use for uploading. (default 4M)
      --qingstor-connection-retries int                          Number of connection retries. (default 3)
      --qingstor-encoding MultiEncoder                           This sets the encoding for the backend. (default Slash,Ctl,InvalidUtf8)
      --qingstor-endpoint string                                 Enter an endpoint URL to connection QingStor API.
      --qingstor-env-auth                                        Get QingStor credentials from runtime. Only applies if access_key_id and secret_access_key is blank.
      --qingstor-secret-access-key string                        QingStor Secret Access Key (password)
      --qingstor-upload-concurrency int                          Concurrency for multipart uploads. (default 1)
      --qingstor-upload-cutoff SizeSuffix                        Cutoff for switching to chunked upload (default 200M)
      --qingstor-zone string                                     Zone to connect to.
      --s3-access-key-id string                                  AWS Access Key ID.
      --s3-acl string                                            Canned ACL used when creating buckets and storing or copying objects.
      --s3-bucket-acl string                                     Canned ACL used when creating buckets.
      --s3-chunk-size SizeSuffix                                 Chunk size to use for uploading. (default 5M)
      --s3-copy-cutoff SizeSuffix                                Cutoff for switching to multipart copy (default 5G)
      --s3-disable-checksum                                      Don't store MD5 checksum with object metadata
      --s3-encoding MultiEncoder                                 This sets the encoding for the backend. (default Slash,InvalidUtf8,Dot)
      --s3-endpoint string                                       Endpoint for S3 API.
      --s3-env-auth                                              Get AWS credentials from runtime (environment variables or EC2/ECS meta data if no env vars).
      --s3-force-path-style                                      If true use path style access if false use virtual hosted style. (default true)
      --s3-leave-parts-on-error                                  If true avoid calling abort upload on a failure, leaving all successfully uploaded parts on S3 for manual recovery.
      --s3-list-chunk int                                        Size of listing chunk (response list for each ListObject S3 request). (default 1000)
      --s3-location-constraint string                            Location constraint - must be set to match the Region.
      --s3-memory-pool-flush-time Duration                       How often internal memory buffer pools will be flushed. (default 1m0s)
      --s3-memory-pool-use-mmap                                  Whether to use mmap buffers in internal memory pool.
      --s3-provider string                                       Choose your S3 provider.
      --s3-region string                                         Region to connect to.
      --s3-secret-access-key string                              AWS Secret Access Key (password)
      --s3-server-side-encryption string                         The server-side encryption algorithm used when storing this object in S3.
      --s3-session-token string                                  An AWS session token
      --s3-sse-customer-algorithm string                         If using SSE-C, the server-side encryption algorithm used when storing this object in S3.
      --s3-sse-customer-key string                               If using SSE-C you must provide the secret encryption key used to encrypt/decrypt your data.
      --s3-sse-customer-key-md5 string                           If using SSE-C you must provide the secret encryption key MD5 checksum.
      --s3-sse-kms-key-id string                                 If using KMS ID you must provide the ARN of Key.
      --s3-storage-class string                                  The storage class to use when storing new objects in S3.
      --s3-upload-concurrency int                                Concurrency for multipart uploads. (default 4)
      --s3-upload-cutoff SizeSuffix                              Cutoff for switching to chunked upload (default 200M)
      --s3-use-accelerate-endpoint                               If true use the AWS S3 accelerated endpoint.
      --s3-v2-auth                                               If true use v2 authentication.
      --seafile-2fa                                              Two-factor authentication ('true' if the account has 2FA enabled)
      --seafile-create-library                                   Should rclone create a library if it doesn't exist
      --seafile-encoding MultiEncoder                            This sets the encoding for the backend. (default Slash,DoubleQuote,BackSlash,Ctl,InvalidUtf8)
      --seafile-library string                                   Name of the library. Leave blank to access all non-encrypted libraries.
      --seafile-library-key string                               Library password (for encrypted libraries only). Leave blank if you pass it through the command line. (obscured)
      --seafile-pass string                                      Password (obscured)
      --seafile-url string                                       URL of seafile host to connect to
      --seafile-user string                                      User name (usually email address)
      --sftp-ask-password                                        Allow asking for SFTP password when needed.
      --sftp-disable-hashcheck                                   Disable the execution of SSH commands to determine if remote file hashing is available.
      --sftp-host string                                         SSH host to connect to
      --sftp-key-file string                                     Path to PEM-encoded private key file, leave blank or set key-use-agent to use ssh-agent.
      --sftp-key-file-pass string                                The passphrase to decrypt the PEM-encoded private key file. (obscured)
      --sftp-key-pem string                                      Raw PEM-encoded private key, If specified, will override key_file parameter.
      --sftp-key-use-agent                                       When set forces the usage of the ssh-agent.
      --sftp-md5sum-command string                               The command used to read md5 hashes. Leave blank for autodetect.
      --sftp-pass string                                         SSH password, leave blank to use ssh-agent. (obscured)
      --sftp-path-override string                                Override path used by SSH connection.
      --sftp-port string                                         SSH port, leave blank to use default (22)
      --sftp-set-modtime                                         Set the modified time on the remote if set. (default true)
      --sftp-sha1sum-command string                              The command used to read sha1 hashes. Leave blank for autodetect.
      --sftp-skip-links                                          Set to skip any symlinks and any other non regular files.
      --sftp-use-insecure-cipher                                 Enable the use of insecure ciphers and key exchange methods.
      --sftp-user string                                         SSH username, leave blank for current username, ncw
      --sharefile-chunk-size SizeSuffix                          Upload chunk size. Must a power of 2 >= 256k. (default 64M)
      --sharefile-encoding MultiEncoder                          This sets the encoding for the backend. (default Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,Ctl,LeftSpace,LeftPeriod,RightSpace,RightPeriod,InvalidUtf8,Dot)
      --sharefile-endpoint string                                Endpoint for API calls.
      --sharefile-root-folder-id string                          ID of the root folder
      --sharefile-upload-cutoff SizeSuffix                       Cutoff for switching to multipart upload. (default 128M)
      --skip-links                                               Don't warn about skipped symlinks.
      --sugarsync-access-key-id string                           Sugarsync Access Key ID.
      --sugarsync-app-id string                                  Sugarsync App ID.
      --sugarsync-authorization string                           Sugarsync authorization
      --sugarsync-authorization-expiry string                    Sugarsync authorization expiry
      --sugarsync-deleted-id string                              Sugarsync deleted folder id
      --sugarsync-encoding MultiEncoder                          This sets the encoding for the backend. (default Slash,Ctl,InvalidUtf8,Dot)
      --sugarsync-hard-delete                                    Permanently delete files if true
      --sugarsync-private-access-key string                      Sugarsync Private Access Key
      --sugarsync-refresh-token string                           Sugarsync refresh token
      --sugarsync-root-id string                                 Sugarsync root id
      --sugarsync-user string                                    Sugarsync user
      --swift-application-credential-id string                   Application Credential ID (OS_APPLICATION_CREDENTIAL_ID)
      --swift-application-credential-name string                 Application Credential Name (OS_APPLICATION_CREDENTIAL_NAME)
      --swift-application-credential-secret string               Application Credential Secret (OS_APPLICATION_CREDENTIAL_SECRET)
      --swift-auth string                                        Authentication URL for server (OS_AUTH_URL).
      --swift-auth-token string                                  Auth Token from alternate authentication - optional (OS_AUTH_TOKEN)
      --swift-auth-version int                                   AuthVersion - optional - set to (1,2,3) if your auth URL has no version (ST_AUTH_VERSION)
      --swift-chunk-size SizeSuffix                              Above this size files will be chunked into a _segments container. (default 5G)
      --swift-domain string                                      User domain - optional (v3 auth) (OS_USER_DOMAIN_NAME)
      --swift-encoding MultiEncoder                              This sets the encoding for the backend. (default Slash,InvalidUtf8)
      --swift-endpoint-type string                               Endpoint type to choose from the service catalogue (OS_ENDPOINT_TYPE) (default "public")
      --swift-env-auth                                           Get swift credentials from environment variables in standard OpenStack form.
      --swift-key string                                         API key or password (OS_PASSWORD).
      --swift-no-chunk                                           Don't chunk files during streaming upload.
      --swift-region string                                      Region name - optional (OS_REGION_NAME)
      --swift-storage-policy string                              The storage policy to use when creating a new container
      --swift-storage-url string                                 Storage URL - optional (OS_STORAGE_URL)
      --swift-tenant string                                      Tenant name - optional for v1 auth, this or tenant_id required otherwise (OS_TENANT_NAME or OS_PROJECT_NAME)
      --swift-tenant-domain string                               Tenant domain - optional (v3 auth) (OS_PROJECT_DOMAIN_NAME)
      --swift-tenant-id string                                   Tenant ID - optional for v1 auth, this or tenant required otherwise (OS_TENANT_ID)
      --swift-user string                                        User name to log in (OS_USERNAME).
      --swift-user-id string                                     User ID to log in - optional - most swift systems use user and leave this blank (v3 auth) (OS_USER_ID).
      --tardigrade-access-grant string                           Access Grant.
      --tardigrade-api-key string                                API Key.
      --tardigrade-passphrase string                             Encryption Passphrase. To access existing objects enter passphrase used for uploading.
      --tardigrade-provider string                               Choose an authentication method. (default "existing")
      --tardigrade-satellite-address <nodeid>@<address>:<port>   Satellite Address. Custom satellite address should match the format: <nodeid>@<address>:<port>. (default "us-central-1.tardigrade.io")
      --union-action-policy string                               Policy to choose upstream on ACTION category. (default "epall")
      --union-cache-time int                                     Cache time of usage and free space (in seconds). This option is only useful when a path preserving policy is used. (default 120)
      --union-create-policy string                               Policy to choose upstream on CREATE category. (default "epmfs")
      --union-search-policy string                               Policy to choose upstream on SEARCH category. (default "ff")
      --union-upstreams string                                   List of space separated upstreams.
      --webdav-bearer-token string                               Bearer token instead of user/pass (eg a Macaroon)
      --webdav-bearer-token-command string                       Command to run to get a bearer token
      --webdav-pass string                                       Password. (obscured)
      --webdav-url string                                        URL of http host to connect to
      --webdav-user string                                       User name
      --webdav-vendor string                                     Name of the Webdav site/service/software you are using
      --yandex-client-id string                                  Yandex Client Id
      --yandex-client-secret string                              Yandex Client Secret
      --yandex-encoding MultiEncoder                             This sets the encoding for the backend. (default Slash,Del,Ctl,InvalidUtf8,Dot)
      --yandex-unlink                                            Remove existing public link to file/folder with link command rather than creating.
```

 1Fichier
-----------------------------------------

This is a backend for the [1fichier](https://1fichier.com) cloud
storage service. Note that a Premium subscription is required to use
the API.

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

The initial setup for 1Fichier involves getting the API key from the website which you
need to do in your browser.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / 1Fichier
   \ "fichier"
[snip]
Storage> fichier
** See help for fichier backend at: https://rclone.org/fichier/ **

Your API Key, get it from https://1fichier.com/console/params.pl
Enter a string value. Press Enter for the default ("").
api_key> example_key

Edit advanced config? (y/n)
y) Yes
n) No
y/n> 
Remote config
--------------------
[remote]
type = fichier
api_key = example_key
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Once configured you can then use `rclone` like this,

List directories in top level of your 1Fichier account

    rclone lsd remote:

List all the files in your 1Fichier account

    rclone ls remote:

To copy a local directory to a 1Fichier directory called backup

    rclone copy /home/source remote:backup

### Modified time and hashes ###

1Fichier does not support modification times. It supports the Whirlpool hash algorithm.

### Duplicated files ###

1Fichier can have two files with exactly the same name and path (unlike a
normal file system).

Duplicated files cause problems with the syncing and you will see
messages in the log about duplicates.

#### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| \         | 0x5C  | ＼           |
| <         | 0x3C  | ＜           |
| >         | 0x3E  | ＞           |
| "         | 0x22  | ＂           |
| $         | 0x24  | ＄           |
| `         | 0x60  | ｀           |
| '         | 0x27  | ＇           |

File names can also not start or end with the following characters.
These only get replaced if they are the first or last character in the
name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.


### Standard Options

Here are the standard options specific to fichier (1Fichier).

#### --fichier-api-key

Your API Key, get it from https://1fichier.com/console/params.pl

- Config:      api_key
- Env Var:     RCLONE_FICHIER_API_KEY
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to fichier (1Fichier).

#### --fichier-shared-folder

If you want to download a shared folder, add this parameter

- Config:      shared_folder
- Env Var:     RCLONE_FICHIER_SHARED_FOLDER
- Type:        string
- Default:     ""

#### --fichier-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_FICHIER_ENCODING
- Type:        MultiEncoder
- Default:     Slash,LtGt,DoubleQuote,SingleQuote,BackQuote,Dollar,BackSlash,Del,Ctl,LeftSpace,RightSpace,InvalidUtf8,Dot



 Alias
-----------------------------------------

The `alias` remote provides a new name for another remote.

Paths may be as deep as required or a local path, 
eg `remote:directory/subdirectory` or `/directory/subdirectory`.

During the initial setup with `rclone config` you will specify the target
remote. The target remote can either be a local path or another remote.

Subfolders can be used in target remote. Assume an alias remote named `backup`
with the target `mydrive:private/backup`. Invoking `rclone mkdir backup:desktop`
is exactly the same as invoking `rclone mkdir mydrive:private/backup/desktop`.

There will be no special handling of paths containing `..` segments.
Invoking `rclone mkdir backup:../desktop` is exactly the same as invoking
`rclone mkdir mydrive:private/backup/../desktop`.
The empty path is not allowed as a remote. To alias the current directory
use `.` instead.

Here is an example of how to make an alias called `remote` for local folder.
First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Alias for an existing remote
   \ "alias"
[snip]
Storage> alias
Remote or path to alias.
Can be "myremote:path/to/dir", "myremote:bucket", "myremote:" or "/local/path".
remote> /mnt/storage/backup
Remote config
--------------------
[remote]
remote = /mnt/storage/backup
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
Current remotes:

Name                 Type
====                 ====
remote               alias

e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> q
```

Once configured you can then use `rclone` like this,

List directories in top level in `/mnt/storage/backup`

    rclone lsd remote:

List all the files in `/mnt/storage/backup`

    rclone ls remote:

Copy another local directory to the alias directory called source

    rclone copy /home/source remote:source


### Standard Options

Here are the standard options specific to alias (Alias for an existing remote).

#### --alias-remote

Remote or path to alias.
Can be "myremote:path/to/dir", "myremote:bucket", "myremote:" or "/local/path".

- Config:      remote
- Env Var:     RCLONE_ALIAS_REMOTE
- Type:        string
- Default:     ""



 Amazon Drive
-----------------------------------------

Amazon Drive, formerly known as Amazon Cloud Drive, is a cloud storage
service run by Amazon for consumers.

## Status

**Important:** rclone supports Amazon Drive only if you have your own
set of API keys. Unfortunately the [Amazon Drive developer
program](https://developer.amazon.com/amazon-drive) is now closed to
new entries so if you don't already have your own set of keys you will
not be able to use rclone with Amazon Drive.

For the history on why rclone no longer has a set of Amazon Drive API
keys see [the forum](https://forum.rclone.org/t/rclone-has-been-banned-from-amazon-drive/2314).

If you happen to know anyone who works at Amazon then please ask them
to re-instate rclone into the Amazon Drive developer program - thanks!

## Setup

The initial setup for Amazon Drive involves getting a token from
Amazon which you need to do in your browser.  `rclone config` walks
you through it.

The configuration process for Amazon Drive may involve using an [oauth
proxy](https://github.com/ncw/oauthproxy). This is used to keep the
Amazon credentials out of the source code.  The proxy runs in Google's
very secure App Engine environment and doesn't store any credentials
which pass through it.

Since rclone doesn't currently have its own Amazon Drive credentials
so you will either need to have your own `client_id` and
`client_secret` with Amazon Drive, or use a third party oauth proxy
in which case you will need to enter `client_id`, `client_secret`,
`auth_url` and `token_url`.

Note also if you are not using Amazon's `auth_url` and `token_url`,
(ie you filled in something for those) then if setting up on a remote
machine you can only use the [copying the config method of
configuration](https://rclone.org/remote_setup/#configuring-by-copying-the-config-file)
- `rclone authorize` will not work.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
n/r/c/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Amazon Drive
   \ "amazon cloud drive"
[snip]
Storage> amazon cloud drive
Amazon Application Client Id - required.
client_id> your client ID goes here
Amazon Application Client Secret - required.
client_secret> your client secret goes here
Auth server URL - leave blank to use Amazon's.
auth_url> Optional auth URL
Token server url - leave blank to use Amazon's.
token_url> Optional token URL
Remote config
Make sure your Redirect URL is set to "http://127.0.0.1:53682/" in your custom config.
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
client_id = your client ID goes here
client_secret = your client secret goes here
auth_url = Optional auth URL
token_url = Optional token URL
token = {"access_token":"xxxxxxxxxxxxxxxxxxxxxxx","token_type":"bearer","refresh_token":"xxxxxxxxxxxxxxxxxx","expiry":"2015-09-06T16:07:39.658438471+01:00"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](https://rclone.org/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Amazon. This only runs from the moment it
opens your browser to the moment you get back the verification
code.  This is on `http://127.0.0.1:53682/` and this it may require
you to unblock it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List directories in top level of your Amazon Drive

    rclone lsd remote:

List all the files in your Amazon Drive

    rclone ls remote:

To copy a local directory to an Amazon Drive directory called backup

    rclone copy /home/source remote:backup

### Modified time and MD5SUMs ###

Amazon Drive doesn't allow modification times to be changed via
the API so these won't be accurate or used for syncing.

It does store MD5SUMs so for a more accurate sync, you can use the
`--checksum` flag.

#### Restricted filename characters

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| /         | 0x2F  | ／          |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Deleting files ###

Any files you delete with rclone will end up in the trash.  Amazon
don't provide an API to permanently delete files, nor to empty the
trash, so you will have to do that with one of Amazon's apps or via
the Amazon Drive website. As of November 17, 2016, files are 
automatically deleted by Amazon from the trash after 30 days.

### Using with non `.com` Amazon accounts ###

Let's say you usually use `amazon.co.uk`. When you authenticate with
rclone it will take you to an `amazon.com` page to log in.  Your
`amazon.co.uk` email and password should work here just fine.


### Standard Options

Here are the standard options specific to amazon cloud drive (Amazon Drive).

#### --acd-client-id

Amazon Application Client ID.

- Config:      client_id
- Env Var:     RCLONE_ACD_CLIENT_ID
- Type:        string
- Default:     ""

#### --acd-client-secret

Amazon Application Client Secret.

- Config:      client_secret
- Env Var:     RCLONE_ACD_CLIENT_SECRET
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to amazon cloud drive (Amazon Drive).

#### --acd-auth-url

Auth server URL.
Leave blank to use Amazon's.

- Config:      auth_url
- Env Var:     RCLONE_ACD_AUTH_URL
- Type:        string
- Default:     ""

#### --acd-token-url

Token server url.
leave blank to use Amazon's.

- Config:      token_url
- Env Var:     RCLONE_ACD_TOKEN_URL
- Type:        string
- Default:     ""

#### --acd-checkpoint

Checkpoint for internal polling (debug).

- Config:      checkpoint
- Env Var:     RCLONE_ACD_CHECKPOINT
- Type:        string
- Default:     ""

#### --acd-upload-wait-per-gb

Additional time per GB to wait after a failed complete upload to see if it appears.

Sometimes Amazon Drive gives an error when a file has been fully
uploaded but the file appears anyway after a little while.  This
happens sometimes for files over 1GB in size and nearly every time for
files bigger than 10GB. This parameter controls the time rclone waits
for the file to appear.

The default value for this parameter is 3 minutes per GB, so by
default it will wait 3 minutes for every GB uploaded to see if the
file appears.

You can disable this feature by setting it to 0. This may cause
conflict errors as rclone retries the failed upload but the file will
most likely appear correctly eventually.

These values were determined empirically by observing lots of uploads
of big files for a range of file sizes.

Upload with the "-v" flag to see more info about what rclone is doing
in this situation.

- Config:      upload_wait_per_gb
- Env Var:     RCLONE_ACD_UPLOAD_WAIT_PER_GB
- Type:        Duration
- Default:     3m0s

#### --acd-templink-threshold

Files >= this size will be downloaded via their tempLink.

Files this size or more will be downloaded via their "tempLink". This
is to work around a problem with Amazon Drive which blocks downloads
of files bigger than about 10GB.  The default for this is 9GB which
shouldn't need to be changed.

To download files above this threshold, rclone requests a "tempLink"
which downloads the file through a temporary URL directly from the
underlying S3 storage.

- Config:      templink_threshold
- Env Var:     RCLONE_ACD_TEMPLINK_THRESHOLD
- Type:        SizeSuffix
- Default:     9G

#### --acd-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_ACD_ENCODING
- Type:        MultiEncoder
- Default:     Slash,InvalidUtf8,Dot



### Limitations ###

Note that Amazon Drive is case insensitive so you can't have a
file called "Hello.doc" and one called "hello.doc".

Amazon Drive has rate limiting so you may notice errors in the
sync (429 errors).  rclone will automatically retry the sync up to 3
times by default (see `--retries` flag) which should hopefully work
around this problem.

Amazon Drive has an internal limit of file sizes that can be uploaded
to the service. This limit is not officially published, but all files
larger than this will fail.

At the time of writing (Jan 2016) is in the area of 50GB per file.
This means that larger files are likely to fail.

Unfortunately there is no way for rclone to see that this failure is
because of file size, so it will retry the operation, as any other
failure. To avoid this problem, use `--max-size 50000M` option to limit
the maximum size of uploaded files. Note that `--max-size` does not split
files into segments, it only ignores files over this size.

 Amazon S3 Storage Providers
--------------------------------------------------------

The S3 backend can be used with a number of different providers:


- AWS S3
- Alibaba Cloud (Aliyun) Object Storage System (OSS)
- Ceph
- DigitalOcean Spaces
- Dreamhost
- IBM COS S3
- Minio
- Scaleway
- StackPath
- Wasabi


Paths are specified as `remote:bucket` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, eg `remote:bucket/path/to/dir`.

Once you have made a remote (see the provider specific section above)
you can use it like this:

See all buckets

    rclone lsd remote:

Make a new bucket

    rclone mkdir remote:bucket

List the contents of a bucket

    rclone ls remote:bucket

Sync `/home/local/directory` to the remote bucket, deleting any excess
files in the bucket.

    rclone sync /home/local/directory remote:bucket

## AWS S3 {#amazon-s3}

Here is an example of making an s3 configuration.  First run

    rclone config

This will guide you through an interactive setup process.

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Amazon S3 Compliant Storage Providers (AWS, Ceph, Dreamhost, IBM COS, Minio)
   \ "s3"
[snip]
Storage> s3
Choose your S3 provider.
Choose a number from below, or type in your own value
 1 / Amazon Web Services (AWS) S3
   \ "AWS"
 2 / Ceph Object Storage
   \ "Ceph"
 3 / Digital Ocean Spaces
   \ "DigitalOcean"
 4 / Dreamhost DreamObjects
   \ "Dreamhost"
 5 / IBM COS S3
   \ "IBMCOS"
 6 / Minio Object Storage
   \ "Minio"
 7 / Wasabi Object Storage
   \ "Wasabi"
 8 / Any other S3 compatible provider
   \ "Other"
provider> 1
Get AWS credentials from runtime (environment variables or EC2/ECS meta data if no env vars). Only applies if access_key_id and secret_access_key is blank.
Choose a number from below, or type in your own value
 1 / Enter AWS credentials in the next step
   \ "false"
 2 / Get AWS credentials from the environment (env vars or IAM)
   \ "true"
env_auth> 1
AWS Access Key ID - leave blank for anonymous access or runtime credentials.
access_key_id> XXX
AWS Secret Access Key (password) - leave blank for anonymous access or runtime credentials.
secret_access_key> YYY
Region to connect to.
Choose a number from below, or type in your own value
   / The default endpoint - a good choice if you are unsure.
 1 | US Region, Northern Virginia or Pacific Northwest.
   | Leave location constraint empty.
   \ "us-east-1"
   / US East (Ohio) Region
 2 | Needs location constraint us-east-2.
   \ "us-east-2"
   / US West (Oregon) Region
 3 | Needs location constraint us-west-2.
   \ "us-west-2"
   / US West (Northern California) Region
 4 | Needs location constraint us-west-1.
   \ "us-west-1"
   / Canada (Central) Region
 5 | Needs location constraint ca-central-1.
   \ "ca-central-1"
   / EU (Ireland) Region
 6 | Needs location constraint EU or eu-west-1.
   \ "eu-west-1"
   / EU (London) Region
 7 | Needs location constraint eu-west-2.
   \ "eu-west-2"
   / EU (Frankfurt) Region
 8 | Needs location constraint eu-central-1.
   \ "eu-central-1"
   / Asia Pacific (Singapore) Region
 9 | Needs location constraint ap-southeast-1.
   \ "ap-southeast-1"
   / Asia Pacific (Sydney) Region
10 | Needs location constraint ap-southeast-2.
   \ "ap-southeast-2"
   / Asia Pacific (Tokyo) Region
11 | Needs location constraint ap-northeast-1.
   \ "ap-northeast-1"
   / Asia Pacific (Seoul)
12 | Needs location constraint ap-northeast-2.
   \ "ap-northeast-2"
   / Asia Pacific (Mumbai)
13 | Needs location constraint ap-south-1.
   \ "ap-south-1"
   / Asia Patific (Hong Kong) Region
14 | Needs location constraint ap-east-1.
   \ "ap-east-1"
   / South America (Sao Paulo) Region
15 | Needs location constraint sa-east-1.
   \ "sa-east-1"
region> 1
Endpoint for S3 API.
Leave blank if using AWS to use the default endpoint for the region.
endpoint> 
Location constraint - must be set to match the Region. Used when creating buckets only.
Choose a number from below, or type in your own value
 1 / Empty for US Region, Northern Virginia or Pacific Northwest.
   \ ""
 2 / US East (Ohio) Region.
   \ "us-east-2"
 3 / US West (Oregon) Region.
   \ "us-west-2"
 4 / US West (Northern California) Region.
   \ "us-west-1"
 5 / Canada (Central) Region.
   \ "ca-central-1"
 6 / EU (Ireland) Region.
   \ "eu-west-1"
 7 / EU (London) Region.
   \ "eu-west-2"
 8 / EU Region.
   \ "EU"
 9 / Asia Pacific (Singapore) Region.
   \ "ap-southeast-1"
10 / Asia Pacific (Sydney) Region.
   \ "ap-southeast-2"
11 / Asia Pacific (Tokyo) Region.
   \ "ap-northeast-1"
12 / Asia Pacific (Seoul)
   \ "ap-northeast-2"
13 / Asia Pacific (Mumbai)
   \ "ap-south-1"
14 / Asia Pacific (Hong Kong)
   \ "ap-east-1"
15 / South America (Sao Paulo) Region.
   \ "sa-east-1"
location_constraint> 1
Canned ACL used when creating buckets and/or storing objects in S3.
For more info visit https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
Choose a number from below, or type in your own value
 1 / Owner gets FULL_CONTROL. No one else has access rights (default).
   \ "private"
 2 / Owner gets FULL_CONTROL. The AllUsers group gets READ access.
   \ "public-read"
   / Owner gets FULL_CONTROL. The AllUsers group gets READ and WRITE access.
 3 | Granting this on a bucket is generally not recommended.
   \ "public-read-write"
 4 / Owner gets FULL_CONTROL. The AuthenticatedUsers group gets READ access.
   \ "authenticated-read"
   / Object owner gets FULL_CONTROL. Bucket owner gets READ access.
 5 | If you specify this canned ACL when creating a bucket, Amazon S3 ignores it.
   \ "bucket-owner-read"
   / Both the object owner and the bucket owner get FULL_CONTROL over the object.
 6 | If you specify this canned ACL when creating a bucket, Amazon S3 ignores it.
   \ "bucket-owner-full-control"
acl> 1
The server-side encryption algorithm used when storing this object in S3.
Choose a number from below, or type in your own value
 1 / None
   \ ""
 2 / AES256
   \ "AES256"
server_side_encryption> 1
The storage class to use when storing objects in S3.
Choose a number from below, or type in your own value
 1 / Default
   \ ""
 2 / Standard storage class
   \ "STANDARD"
 3 / Reduced redundancy storage class
   \ "REDUCED_REDUNDANCY"
 4 / Standard Infrequent Access storage class
   \ "STANDARD_IA"
 5 / One Zone Infrequent Access storage class
   \ "ONEZONE_IA"
 6 / Glacier storage class
   \ "GLACIER"
 7 / Glacier Deep Archive storage class
   \ "DEEP_ARCHIVE"
 8 / Intelligent-Tiering storage class
   \ "INTELLIGENT_TIERING"
storage_class> 1
Remote config
--------------------
[remote]
type = s3
provider = AWS
env_auth = false
access_key_id = XXX
secret_access_key = YYY
region = us-east-1
endpoint = 
location_constraint = 
acl = private
server_side_encryption = 
storage_class = 
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> 
```

### --fast-list ###

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](https://rclone.org/docs/#fast-list) for more details.

### --update and --use-server-modtime ###

As noted below, the modified time is stored on metadata on the object. It is
used by default for all operations that require checking the time a file was
last updated. It allows rclone to treat the remote more like a true filesystem,
but it is inefficient because it requires an extra API call to retrieve the
metadata.

For many operations, the time the object was last uploaded to the remote is
sufficient to determine if it is "dirty". By using `--update` along with
`--use-server-modtime`, you can avoid the extra API call and simply upload
files whose local modtime is newer than the time it was last uploaded.

### Modified time ###

The modified time is stored as metadata on the object as
`X-Amz-Meta-Mtime` as floating point since the epoch accurate to 1 ns.

If the modification time needs to be updated rclone will attempt to perform a server
side copy to update the modification if the object can be copied in a single part.
In the case the object is larger than 5Gb or is in Glacier or Glacier Deep Archive
storage the object will be uploaded rather than copied.

#### Restricted filename characters

S3 allows any valid UTF-8 string as a key.

Invalid UTF-8 bytes will be [replaced](https://rclone.org/overview/#invalid-utf8), as
they can't be used in XML.

The following characters are replaced since these are problematic when
dealing with the REST API:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| /         | 0x2F  | ／           |

The encoding will also encode these file names as they don't seem to
work with the SDK properly:

| File name | Replacement |
| --------- |:-----------:|
| .         | ．          |
| ..        | ．．         |

### Multipart uploads ###

rclone supports multipart uploads with S3 which means that it can
upload files bigger than 5GB.

Note that files uploaded *both* with multipart upload *and* through
crypt remotes do not have MD5 sums.

rclone switches from single part uploads to multipart uploads at the
point specified by `--s3-upload-cutoff`.  This can be a maximum of 5GB
and a minimum of 0 (ie always upload multipart files).

The chunk sizes used in the multipart upload are specified by
`--s3-chunk-size` and the number of chunks uploaded concurrently is
specified by `--s3-upload-concurrency`.

Multipart uploads will use `--transfers` * `--s3-upload-concurrency` *
`--s3-chunk-size` extra memory.  Single part uploads to not use extra
memory.

Single part transfers can be faster than multipart transfers or slower
depending on your latency from S3 - the more latency, the more likely
single part transfers will be faster.

Increasing `--s3-upload-concurrency` will increase throughput (8 would
be a sensible value) and increasing `--s3-chunk-size` also increases
throughput (16M would be sensible).  Increasing either of these will
use more memory.  The default values are high enough to gain most of
the possible performance without using too much memory.


### Buckets and Regions ###

With Amazon S3 you can list buckets (`rclone lsd`) using any region,
but you can only access the content of a bucket from the region it was
created in.  If you attempt to access a bucket from the wrong region,
you will get an error, `incorrect region, the bucket is not in 'XXX'
region`.

### Authentication ###

There are a number of ways to supply `rclone` with a set of AWS
credentials, with and without using the environment.

The different authentication methods are tried in this order:

 - Directly in the rclone configuration file (`env_auth = false` in the config file):
   - `access_key_id` and `secret_access_key` are required.
   - `session_token` can be optionally set when using AWS STS.
 - Runtime configuration (`env_auth = true` in the config file):
   - Export the following environment variables before running `rclone`:
     - Access Key ID: `AWS_ACCESS_KEY_ID` or `AWS_ACCESS_KEY`
     - Secret Access Key: `AWS_SECRET_ACCESS_KEY` or `AWS_SECRET_KEY`
     - Session Token: `AWS_SESSION_TOKEN` (optional)
   - Or, use a [named profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-multiple-profiles.html):
     - Profile files are standard files used by AWS CLI tools
     - By default it will use the profile in your home directory (eg `~/.aws/credentials` on unix based systems) file and the "default" profile, to change set these environment variables:
         - `AWS_SHARED_CREDENTIALS_FILE` to control which file.
         - `AWS_PROFILE` to control which profile to use.
   - Or, run `rclone` in an ECS task with an IAM role (AWS only).
   - Or, run `rclone` on an EC2 instance with an IAM role (AWS only).
   - Or, run `rclone` in an EKS pod with an IAM role that is associated with a service account (AWS only).

If none of these option actually end up providing `rclone` with AWS
credentials then S3 interaction will be non-authenticated (see below).

### S3 Permissions ###

When using the `sync` subcommand of `rclone` the following minimum
permissions are required to be available on the bucket being written to:

* `ListBucket`
* `DeleteObject`
* `GetObject`
* `PutObject`
* `PutObjectACL`

When using the `lsd` subcommand, the `ListAllMyBuckets` permission is required.

Example policy:

```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::USER_SID:user/USER_NAME"
            },
            "Action": [
                "s3:ListBucket",
                "s3:DeleteObject",
                "s3:GetObject",
                "s3:PutObject",
                "s3:PutObjectAcl"
            ],
            "Resource": [
              "arn:aws:s3:::BUCKET_NAME/*",
              "arn:aws:s3:::BUCKET_NAME"
            ]
        },
        {
            "Effect": "Allow",
            "Action": "s3:ListAllMyBuckets",
            "Resource": "arn:aws:s3:::*"
        }	
    ]
}
```

Notes on above:

1. This is a policy that can be used when creating bucket. It assumes
   that `USER_NAME` has been created.
2. The Resource entry must include both resource ARNs, as one implies
   the bucket and the other implies the bucket's objects.

For reference, [here's an Ansible script](https://gist.github.com/ebridges/ebfc9042dd7c756cd101cfa807b7ae2b)
that will generate one or more buckets that will work with `rclone sync`.

### Key Management System (KMS) ###

If you are using server side encryption with KMS then you will find
you can't transfer small objects.  As a work-around you can use the
`--ignore-checksum` flag.

A proper fix is being worked on in [issue #1824](https://github.com/rclone/rclone/issues/1824).

### Glacier and Glacier Deep Archive ###

You can upload objects using the glacier storage class or transition them to glacier using a [lifecycle policy](http://docs.aws.amazon.com/AmazonS3/latest/user-guide/create-lifecycle.html).
The bucket can still be synced or copied into normally, but if rclone
tries to access data from the glacier storage class you will see an error like below.

    2017/09/11 19:07:43 Failed to sync: failed to open source object: Object in GLACIER, restore first: path/to/file

In this case you need to [restore](http://docs.aws.amazon.com/AmazonS3/latest/user-guide/restore-archived-objects.html)
the object(s) in question before using rclone.

Note that rclone only speaks the S3 API it does not speak the Glacier
Vault API, so rclone cannot directly access Glacier Vaults.


### Standard Options

Here are the standard options specific to s3 (Amazon S3 Compliant Storage Provider (AWS, Alibaba, Ceph, Digital Ocean, Dreamhost, IBM COS, Minio, etc)).

#### --s3-provider

Choose your S3 provider.

- Config:      provider
- Env Var:     RCLONE_S3_PROVIDER
- Type:        string
- Default:     ""
- Examples:
    - "AWS"
        - Amazon Web Services (AWS) S3
    - "Alibaba"
        - Alibaba Cloud Object Storage System (OSS) formerly Aliyun
    - "Ceph"
        - Ceph Object Storage
    - "DigitalOcean"
        - Digital Ocean Spaces
    - "Dreamhost"
        - Dreamhost DreamObjects
    - "IBMCOS"
        - IBM COS S3
    - "Minio"
        - Minio Object Storage
    - "Netease"
        - Netease Object Storage (NOS)
    - "StackPath"
        - StackPath Object Storage
    - "Wasabi"
        - Wasabi Object Storage
    - "Other"
        - Any other S3 compatible provider

#### --s3-env-auth

Get AWS credentials from runtime (environment variables or EC2/ECS meta data if no env vars).
Only applies if access_key_id and secret_access_key is blank.

- Config:      env_auth
- Env Var:     RCLONE_S3_ENV_AUTH
- Type:        bool
- Default:     false
- Examples:
    - "false"
        - Enter AWS credentials in the next step
    - "true"
        - Get AWS credentials from the environment (env vars or IAM)

#### --s3-access-key-id

AWS Access Key ID.
Leave blank for anonymous access or runtime credentials.

- Config:      access_key_id
- Env Var:     RCLONE_S3_ACCESS_KEY_ID
- Type:        string
- Default:     ""

#### --s3-secret-access-key

AWS Secret Access Key (password)
Leave blank for anonymous access or runtime credentials.

- Config:      secret_access_key
- Env Var:     RCLONE_S3_SECRET_ACCESS_KEY
- Type:        string
- Default:     ""

#### --s3-region

Region to connect to.

- Config:      region
- Env Var:     RCLONE_S3_REGION
- Type:        string
- Default:     ""
- Examples:
    - "us-east-1"
        - The default endpoint - a good choice if you are unsure.
        - US Region, Northern Virginia or Pacific Northwest.
        - Leave location constraint empty.
    - "us-east-2"
        - US East (Ohio) Region
        - Needs location constraint us-east-2.
    - "us-west-2"
        - US West (Oregon) Region
        - Needs location constraint us-west-2.
    - "us-west-1"
        - US West (Northern California) Region
        - Needs location constraint us-west-1.
    - "ca-central-1"
        - Canada (Central) Region
        - Needs location constraint ca-central-1.
    - "eu-west-1"
        - EU (Ireland) Region
        - Needs location constraint EU or eu-west-1.
    - "eu-west-2"
        - EU (London) Region
        - Needs location constraint eu-west-2.
    - "eu-north-1"
        - EU (Stockholm) Region
        - Needs location constraint eu-north-1.
    - "eu-central-1"
        - EU (Frankfurt) Region
        - Needs location constraint eu-central-1.
    - "ap-southeast-1"
        - Asia Pacific (Singapore) Region
        - Needs location constraint ap-southeast-1.
    - "ap-southeast-2"
        - Asia Pacific (Sydney) Region
        - Needs location constraint ap-southeast-2.
    - "ap-northeast-1"
        - Asia Pacific (Tokyo) Region
        - Needs location constraint ap-northeast-1.
    - "ap-northeast-2"
        - Asia Pacific (Seoul)
        - Needs location constraint ap-northeast-2.
    - "ap-south-1"
        - Asia Pacific (Mumbai)
        - Needs location constraint ap-south-1.
    - "ap-east-1"
        - Asia Patific (Hong Kong) Region
        - Needs location constraint ap-east-1.
    - "sa-east-1"
        - South America (Sao Paulo) Region
        - Needs location constraint sa-east-1.

#### --s3-region

Region to connect to.
Leave blank if you are using an S3 clone and you don't have a region.

- Config:      region
- Env Var:     RCLONE_S3_REGION
- Type:        string
- Default:     ""
- Examples:
    - ""
        - Use this if unsure. Will use v4 signatures and an empty region.
    - "other-v2-signature"
        - Use this only if v4 signatures don't work, eg pre Jewel/v10 CEPH.

#### --s3-endpoint

Endpoint for S3 API.
Leave blank if using AWS to use the default endpoint for the region.

- Config:      endpoint
- Env Var:     RCLONE_S3_ENDPOINT
- Type:        string
- Default:     ""

#### --s3-endpoint

Endpoint for IBM COS S3 API.
Specify if using an IBM COS On Premise.

- Config:      endpoint
- Env Var:     RCLONE_S3_ENDPOINT
- Type:        string
- Default:     ""
- Examples:
    - "s3-api.us-geo.objectstorage.softlayer.net"
        - US Cross Region Endpoint
    - "s3-api.dal.us-geo.objectstorage.softlayer.net"
        - US Cross Region Dallas Endpoint
    - "s3-api.wdc-us-geo.objectstorage.softlayer.net"
        - US Cross Region Washington DC Endpoint
    - "s3-api.sjc-us-geo.objectstorage.softlayer.net"
        - US Cross Region San Jose Endpoint
    - "s3-api.us-geo.objectstorage.service.networklayer.com"
        - US Cross Region Private Endpoint
    - "s3-api.dal-us-geo.objectstorage.service.networklayer.com"
        - US Cross Region Dallas Private Endpoint
    - "s3-api.wdc-us-geo.objectstorage.service.networklayer.com"
        - US Cross Region Washington DC Private Endpoint
    - "s3-api.sjc-us-geo.objectstorage.service.networklayer.com"
        - US Cross Region San Jose Private Endpoint
    - "s3.us-east.objectstorage.softlayer.net"
        - US Region East Endpoint
    - "s3.us-east.objectstorage.service.networklayer.com"
        - US Region East Private Endpoint
    - "s3.us-south.objectstorage.softlayer.net"
        - US Region South Endpoint
    - "s3.us-south.objectstorage.service.networklayer.com"
        - US Region South Private Endpoint
    - "s3.eu-geo.objectstorage.softlayer.net"
        - EU Cross Region Endpoint
    - "s3.fra-eu-geo.objectstorage.softlayer.net"
        - EU Cross Region Frankfurt Endpoint
    - "s3.mil-eu-geo.objectstorage.softlayer.net"
        - EU Cross Region Milan Endpoint
    - "s3.ams-eu-geo.objectstorage.softlayer.net"
        - EU Cross Region Amsterdam Endpoint
    - "s3.eu-geo.objectstorage.service.networklayer.com"
        - EU Cross Region Private Endpoint
    - "s3.fra-eu-geo.objectstorage.service.networklayer.com"
        - EU Cross Region Frankfurt Private Endpoint
    - "s3.mil-eu-geo.objectstorage.service.networklayer.com"
        - EU Cross Region Milan Private Endpoint
    - "s3.ams-eu-geo.objectstorage.service.networklayer.com"
        - EU Cross Region Amsterdam Private Endpoint
    - "s3.eu-gb.objectstorage.softlayer.net"
        - Great Britain Endpoint
    - "s3.eu-gb.objectstorage.service.networklayer.com"
        - Great Britain Private Endpoint
    - "s3.ap-geo.objectstorage.softlayer.net"
        - APAC Cross Regional Endpoint
    - "s3.tok-ap-geo.objectstorage.softlayer.net"
        - APAC Cross Regional Tokyo Endpoint
    - "s3.hkg-ap-geo.objectstorage.softlayer.net"
        - APAC Cross Regional HongKong Endpoint
    - "s3.seo-ap-geo.objectstorage.softlayer.net"
        - APAC Cross Regional Seoul Endpoint
    - "s3.ap-geo.objectstorage.service.networklayer.com"
        - APAC Cross Regional Private Endpoint
    - "s3.tok-ap-geo.objectstorage.service.networklayer.com"
        - APAC Cross Regional Tokyo Private Endpoint
    - "s3.hkg-ap-geo.objectstorage.service.networklayer.com"
        - APAC Cross Regional HongKong Private Endpoint
    - "s3.seo-ap-geo.objectstorage.service.networklayer.com"
        - APAC Cross Regional Seoul Private Endpoint
    - "s3.mel01.objectstorage.softlayer.net"
        - Melbourne Single Site Endpoint
    - "s3.mel01.objectstorage.service.networklayer.com"
        - Melbourne Single Site Private Endpoint
    - "s3.tor01.objectstorage.softlayer.net"
        - Toronto Single Site Endpoint
    - "s3.tor01.objectstorage.service.networklayer.com"
        - Toronto Single Site Private Endpoint

#### --s3-endpoint

Endpoint for OSS API.

- Config:      endpoint
- Env Var:     RCLONE_S3_ENDPOINT
- Type:        string
- Default:     ""
- Examples:
    - "oss-cn-hangzhou.aliyuncs.com"
        - East China 1 (Hangzhou)
    - "oss-cn-shanghai.aliyuncs.com"
        - East China 2 (Shanghai)
    - "oss-cn-qingdao.aliyuncs.com"
        - North China 1 (Qingdao)
    - "oss-cn-beijing.aliyuncs.com"
        - North China 2 (Beijing)
    - "oss-cn-zhangjiakou.aliyuncs.com"
        - North China 3 (Zhangjiakou)
    - "oss-cn-huhehaote.aliyuncs.com"
        - North China 5 (Huhehaote)
    - "oss-cn-shenzhen.aliyuncs.com"
        - South China 1 (Shenzhen)
    - "oss-cn-hongkong.aliyuncs.com"
        - Hong Kong (Hong Kong)
    - "oss-us-west-1.aliyuncs.com"
        - US West 1 (Silicon Valley)
    - "oss-us-east-1.aliyuncs.com"
        - US East 1 (Virginia)
    - "oss-ap-southeast-1.aliyuncs.com"
        - Southeast Asia Southeast 1 (Singapore)
    - "oss-ap-southeast-2.aliyuncs.com"
        - Asia Pacific Southeast 2 (Sydney)
    - "oss-ap-southeast-3.aliyuncs.com"
        - Southeast Asia Southeast 3 (Kuala Lumpur)
    - "oss-ap-southeast-5.aliyuncs.com"
        - Asia Pacific Southeast 5 (Jakarta)
    - "oss-ap-northeast-1.aliyuncs.com"
        - Asia Pacific Northeast 1 (Japan)
    - "oss-ap-south-1.aliyuncs.com"
        - Asia Pacific South 1 (Mumbai)
    - "oss-eu-central-1.aliyuncs.com"
        - Central Europe 1 (Frankfurt)
    - "oss-eu-west-1.aliyuncs.com"
        - West Europe (London)
    - "oss-me-east-1.aliyuncs.com"
        - Middle East 1 (Dubai)

#### --s3-endpoint

Endpoint for StackPath Object Storage.

- Config:      endpoint
- Env Var:     RCLONE_S3_ENDPOINT
- Type:        string
- Default:     ""
- Examples:
    - "s3.us-east-2.stackpathstorage.com"
        - US East Endpoint
    - "s3.us-west-1.stackpathstorage.com"
        - US West Endpoint
    - "s3.eu-central-1.stackpathstorage.com"
        - EU Endpoint

#### --s3-endpoint

Endpoint for S3 API.
Required when using an S3 clone.

- Config:      endpoint
- Env Var:     RCLONE_S3_ENDPOINT
- Type:        string
- Default:     ""
- Examples:
    - "objects-us-east-1.dream.io"
        - Dream Objects endpoint
    - "nyc3.digitaloceanspaces.com"
        - Digital Ocean Spaces New York 3
    - "ams3.digitaloceanspaces.com"
        - Digital Ocean Spaces Amsterdam 3
    - "sgp1.digitaloceanspaces.com"
        - Digital Ocean Spaces Singapore 1
    - "s3.wasabisys.com"
        - Wasabi US East endpoint
    - "s3.us-west-1.wasabisys.com"
        - Wasabi US West endpoint
    - "s3.eu-central-1.wasabisys.com"
        - Wasabi EU Central endpoint

#### --s3-location-constraint

Location constraint - must be set to match the Region.
Used when creating buckets only.

- Config:      location_constraint
- Env Var:     RCLONE_S3_LOCATION_CONSTRAINT
- Type:        string
- Default:     ""
- Examples:
    - ""
        - Empty for US Region, Northern Virginia or Pacific Northwest.
    - "us-east-2"
        - US East (Ohio) Region.
    - "us-west-2"
        - US West (Oregon) Region.
    - "us-west-1"
        - US West (Northern California) Region.
    - "ca-central-1"
        - Canada (Central) Region.
    - "eu-west-1"
        - EU (Ireland) Region.
    - "eu-west-2"
        - EU (London) Region.
    - "eu-north-1"
        - EU (Stockholm) Region.
    - "EU"
        - EU Region.
    - "ap-southeast-1"
        - Asia Pacific (Singapore) Region.
    - "ap-southeast-2"
        - Asia Pacific (Sydney) Region.
    - "ap-northeast-1"
        - Asia Pacific (Tokyo) Region.
    - "ap-northeast-2"
        - Asia Pacific (Seoul)
    - "ap-south-1"
        - Asia Pacific (Mumbai)
    - "ap-east-1"
        - Asia Pacific (Hong Kong)
    - "sa-east-1"
        - South America (Sao Paulo) Region.

#### --s3-location-constraint

Location constraint - must match endpoint when using IBM Cloud Public.
For on-prem COS, do not make a selection from this list, hit enter

- Config:      location_constraint
- Env Var:     RCLONE_S3_LOCATION_CONSTRAINT
- Type:        string
- Default:     ""
- Examples:
    - "us-standard"
        - US Cross Region Standard
    - "us-vault"
        - US Cross Region Vault
    - "us-cold"
        - US Cross Region Cold
    - "us-flex"
        - US Cross Region Flex
    - "us-east-standard"
        - US East Region Standard
    - "us-east-vault"
        - US East Region Vault
    - "us-east-cold"
        - US East Region Cold
    - "us-east-flex"
        - US East Region Flex
    - "us-south-standard"
        - US South Region Standard
    - "us-south-vault"
        - US South Region Vault
    - "us-south-cold"
        - US South Region Cold
    - "us-south-flex"
        - US South Region Flex
    - "eu-standard"
        - EU Cross Region Standard
    - "eu-vault"
        - EU Cross Region Vault
    - "eu-cold"
        - EU Cross Region Cold
    - "eu-flex"
        - EU Cross Region Flex
    - "eu-gb-standard"
        - Great Britain Standard
    - "eu-gb-vault"
        - Great Britain Vault
    - "eu-gb-cold"
        - Great Britain Cold
    - "eu-gb-flex"
        - Great Britain Flex
    - "ap-standard"
        - APAC Standard
    - "ap-vault"
        - APAC Vault
    - "ap-cold"
        - APAC Cold
    - "ap-flex"
        - APAC Flex
    - "mel01-standard"
        - Melbourne Standard
    - "mel01-vault"
        - Melbourne Vault
    - "mel01-cold"
        - Melbourne Cold
    - "mel01-flex"
        - Melbourne Flex
    - "tor01-standard"
        - Toronto Standard
    - "tor01-vault"
        - Toronto Vault
    - "tor01-cold"
        - Toronto Cold
    - "tor01-flex"
        - Toronto Flex

#### --s3-location-constraint

Location constraint - must be set to match the Region.
Leave blank if not sure. Used when creating buckets only.

- Config:      location_constraint
- Env Var:     RCLONE_S3_LOCATION_CONSTRAINT
- Type:        string
- Default:     ""

#### --s3-acl

Canned ACL used when creating buckets and storing or copying objects.

This ACL is used for creating objects and if bucket_acl isn't set, for creating buckets too.

For more info visit https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl

Note that this ACL is applied when server side copying objects as S3
doesn't copy the ACL from the source but rather writes a fresh one.

- Config:      acl
- Env Var:     RCLONE_S3_ACL
- Type:        string
- Default:     ""
- Examples:
    - "private"
        - Owner gets FULL_CONTROL. No one else has access rights (default).
    - "public-read"
        - Owner gets FULL_CONTROL. The AllUsers group gets READ access.
    - "public-read-write"
        - Owner gets FULL_CONTROL. The AllUsers group gets READ and WRITE access.
        - Granting this on a bucket is generally not recommended.
    - "authenticated-read"
        - Owner gets FULL_CONTROL. The AuthenticatedUsers group gets READ access.
    - "bucket-owner-read"
        - Object owner gets FULL_CONTROL. Bucket owner gets READ access.
        - If you specify this canned ACL when creating a bucket, Amazon S3 ignores it.
    - "bucket-owner-full-control"
        - Both the object owner and the bucket owner get FULL_CONTROL over the object.
        - If you specify this canned ACL when creating a bucket, Amazon S3 ignores it.
    - "private"
        - Owner gets FULL_CONTROL. No one else has access rights (default). This acl is available on IBM Cloud (Infra), IBM Cloud (Storage), On-Premise COS
    - "public-read"
        - Owner gets FULL_CONTROL. The AllUsers group gets READ access. This acl is available on IBM Cloud (Infra), IBM Cloud (Storage), On-Premise IBM COS
    - "public-read-write"
        - Owner gets FULL_CONTROL. The AllUsers group gets READ and WRITE access. This acl is available on IBM Cloud (Infra), On-Premise IBM COS
    - "authenticated-read"
        - Owner gets FULL_CONTROL. The AuthenticatedUsers group gets READ access. Not supported on Buckets. This acl is available on IBM Cloud (Infra) and On-Premise IBM COS

#### --s3-server-side-encryption

The server-side encryption algorithm used when storing this object in S3.

- Config:      server_side_encryption
- Env Var:     RCLONE_S3_SERVER_SIDE_ENCRYPTION
- Type:        string
- Default:     ""
- Examples:
    - ""
        - None
    - "AES256"
        - AES256
    - "aws:kms"
        - aws:kms

#### --s3-sse-kms-key-id

If using KMS ID you must provide the ARN of Key.

- Config:      sse_kms_key_id
- Env Var:     RCLONE_S3_SSE_KMS_KEY_ID
- Type:        string
- Default:     ""
- Examples:
    - ""
        - None
    - "arn:aws:kms:us-east-1:*"
        - arn:aws:kms:*

#### --s3-storage-class

The storage class to use when storing new objects in S3.

- Config:      storage_class
- Env Var:     RCLONE_S3_STORAGE_CLASS
- Type:        string
- Default:     ""
- Examples:
    - ""
        - Default
    - "STANDARD"
        - Standard storage class
    - "REDUCED_REDUNDANCY"
        - Reduced redundancy storage class
    - "STANDARD_IA"
        - Standard Infrequent Access storage class
    - "ONEZONE_IA"
        - One Zone Infrequent Access storage class
    - "GLACIER"
        - Glacier storage class
    - "DEEP_ARCHIVE"
        - Glacier Deep Archive storage class
    - "INTELLIGENT_TIERING"
        - Intelligent-Tiering storage class

#### --s3-storage-class

The storage class to use when storing new objects in OSS.

- Config:      storage_class
- Env Var:     RCLONE_S3_STORAGE_CLASS
- Type:        string
- Default:     ""
- Examples:
    - ""
        - Default
    - "STANDARD"
        - Standard storage class
    - "GLACIER"
        - Archive storage mode.
    - "STANDARD_IA"
        - Infrequent access storage mode.

### Advanced Options

Here are the advanced options specific to s3 (Amazon S3 Compliant Storage Provider (AWS, Alibaba, Ceph, Digital Ocean, Dreamhost, IBM COS, Minio, etc)).

#### --s3-bucket-acl

Canned ACL used when creating buckets.

For more info visit https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl

Note that this ACL is applied when only when creating buckets.  If it
isn't set then "acl" is used instead.

- Config:      bucket_acl
- Env Var:     RCLONE_S3_BUCKET_ACL
- Type:        string
- Default:     ""
- Examples:
    - "private"
        - Owner gets FULL_CONTROL. No one else has access rights (default).
    - "public-read"
        - Owner gets FULL_CONTROL. The AllUsers group gets READ access.
    - "public-read-write"
        - Owner gets FULL_CONTROL. The AllUsers group gets READ and WRITE access.
        - Granting this on a bucket is generally not recommended.
    - "authenticated-read"
        - Owner gets FULL_CONTROL. The AuthenticatedUsers group gets READ access.

#### --s3-sse-customer-algorithm

If using SSE-C, the server-side encryption algorithm used when storing this object in S3.

- Config:      sse_customer_algorithm
- Env Var:     RCLONE_S3_SSE_CUSTOMER_ALGORITHM
- Type:        string
- Default:     ""
- Examples:
    - ""
        - None
    - "AES256"
        - AES256

#### --s3-sse-customer-key

If using SSE-C you must provide the secret encryption key used to encrypt/decrypt your data.

- Config:      sse_customer_key
- Env Var:     RCLONE_S3_SSE_CUSTOMER_KEY
- Type:        string
- Default:     ""
- Examples:
    - ""
        - None

#### --s3-sse-customer-key-md5

If using SSE-C you must provide the secret encryption key MD5 checksum.

- Config:      sse_customer_key_md5
- Env Var:     RCLONE_S3_SSE_CUSTOMER_KEY_MD5
- Type:        string
- Default:     ""
- Examples:
    - ""
        - None

#### --s3-upload-cutoff

Cutoff for switching to chunked upload

Any files larger than this will be uploaded in chunks of chunk_size.
The minimum is 0 and the maximum is 5GB.

- Config:      upload_cutoff
- Env Var:     RCLONE_S3_UPLOAD_CUTOFF
- Type:        SizeSuffix
- Default:     200M

#### --s3-chunk-size

Chunk size to use for uploading.

When uploading files larger than upload_cutoff or files with unknown
size (eg from "rclone rcat" or uploaded with "rclone mount" or google
photos or google docs) they will be uploaded as multipart uploads
using this chunk size.

Note that "--s3-upload-concurrency" chunks of this size are buffered
in memory per transfer.

If you are transferring large files over high speed links and you have
enough memory, then increasing this will speed up the transfers.

Rclone will automatically increase the chunk size when uploading a
large file of known size to stay below the 10,000 chunks limit.

Files of unknown size are uploaded with the configured
chunk_size. Since the default chunk size is 5MB and there can be at
most 10,000 chunks, this means that by default the maximum size of
file you can stream upload is 48GB.  If you wish to stream upload
larger files then you will need to increase chunk_size.

- Config:      chunk_size
- Env Var:     RCLONE_S3_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     5M

#### --s3-copy-cutoff

Cutoff for switching to multipart copy

Any files larger than this that need to be server side copied will be
copied in chunks of this size.

The minimum is 0 and the maximum is 5GB.

- Config:      copy_cutoff
- Env Var:     RCLONE_S3_COPY_CUTOFF
- Type:        SizeSuffix
- Default:     5G

#### --s3-disable-checksum

Don't store MD5 checksum with object metadata

Normally rclone will calculate the MD5 checksum of the input before
uploading it so it can add it to metadata on the object. This is great
for data integrity checking but can cause long delays for large files
to start uploading.

- Config:      disable_checksum
- Env Var:     RCLONE_S3_DISABLE_CHECKSUM
- Type:        bool
- Default:     false

#### --s3-session-token

An AWS session token

- Config:      session_token
- Env Var:     RCLONE_S3_SESSION_TOKEN
- Type:        string
- Default:     ""

#### --s3-upload-concurrency

Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently.

If you are uploading small numbers of large file over high speed link
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.

- Config:      upload_concurrency
- Env Var:     RCLONE_S3_UPLOAD_CONCURRENCY
- Type:        int
- Default:     4

#### --s3-force-path-style

If true use path style access if false use virtual hosted style.

If this is true (the default) then rclone will use path style access,
if false then rclone will use virtual path style. See [the AWS S3
docs](https://docs.aws.amazon.com/AmazonS3/latest/dev/UsingBucket.html#access-bucket-intro)
for more info.

Some providers (eg AWS, Aliyun OSS or Netease COS) require this set to
false - rclone will do this automatically based on the provider
setting.

- Config:      force_path_style
- Env Var:     RCLONE_S3_FORCE_PATH_STYLE
- Type:        bool
- Default:     true

#### --s3-v2-auth

If true use v2 authentication.

If this is false (the default) then rclone will use v4 authentication.
If it is set then rclone will use v2 authentication.

Use this only if v4 signatures don't work, eg pre Jewel/v10 CEPH.

- Config:      v2_auth
- Env Var:     RCLONE_S3_V2_AUTH
- Type:        bool
- Default:     false

#### --s3-use-accelerate-endpoint

If true use the AWS S3 accelerated endpoint.

See: [AWS S3 Transfer acceleration](https://docs.aws.amazon.com/AmazonS3/latest/dev/transfer-acceleration-examples.html)

- Config:      use_accelerate_endpoint
- Env Var:     RCLONE_S3_USE_ACCELERATE_ENDPOINT
- Type:        bool
- Default:     false

#### --s3-leave-parts-on-error

If true avoid calling abort upload on a failure, leaving all successfully uploaded parts on S3 for manual recovery.

It should be set to true for resuming uploads across different sessions.

WARNING: Storing parts of an incomplete multipart upload counts towards space usage on S3 and will add additional costs if not cleaned up.


- Config:      leave_parts_on_error
- Env Var:     RCLONE_S3_LEAVE_PARTS_ON_ERROR
- Type:        bool
- Default:     false

#### --s3-list-chunk

Size of listing chunk (response list for each ListObject S3 request).

This option is also known as "MaxKeys", "max-items", or "page-size" from the AWS S3 specification.
Most services truncate the response list to 1000 objects even if requested more than that.
In AWS S3 this is a global maximum and cannot be changed, see [AWS S3](https://docs.aws.amazon.com/cli/latest/reference/s3/ls.html).
In Ceph, this can be increased with the "rgw list buckets max chunk" option.


- Config:      list_chunk
- Env Var:     RCLONE_S3_LIST_CHUNK
- Type:        int
- Default:     1000

#### --s3-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_S3_ENCODING
- Type:        MultiEncoder
- Default:     Slash,InvalidUtf8,Dot

#### --s3-memory-pool-flush-time

How often internal memory buffer pools will be flushed.
Uploads which requires additional buffers (f.e multipart) will use memory pool for allocations.
This option controls how often unused buffers will be removed from the pool.

- Config:      memory_pool_flush_time
- Env Var:     RCLONE_S3_MEMORY_POOL_FLUSH_TIME
- Type:        Duration
- Default:     1m0s

#### --s3-memory-pool-use-mmap

Whether to use mmap buffers in internal memory pool.

- Config:      memory_pool_use_mmap
- Env Var:     RCLONE_S3_MEMORY_POOL_USE_MMAP
- Type:        bool
- Default:     false



### Anonymous access to public buckets ###

If you want to use rclone to access a public bucket, configure with a
blank `access_key_id` and `secret_access_key`.  Your config should end
up looking like this:

```
[anons3]
type = s3
provider = AWS
env_auth = false
access_key_id = 
secret_access_key = 
region = us-east-1
endpoint = 
location_constraint = 
acl = private
server_side_encryption = 
storage_class = 
```

Then use it as normal with the name of the public bucket, eg

    rclone lsd anons3:1000genomes

You will be able to list and copy data but not upload it.

### Ceph ###

[Ceph](https://ceph.com/) is an open source unified, distributed
storage system designed for excellent performance, reliability and
scalability.  It has an S3 compatible object storage interface.

To use rclone with Ceph, configure as above but leave the region blank
and set the endpoint.  You should end up with something like this in
your config:


```
[ceph]
type = s3
provider = Ceph
env_auth = false
access_key_id = XXX
secret_access_key = YYY
region =
endpoint = https://ceph.endpoint.example.com
location_constraint =
acl =
server_side_encryption =
storage_class =
```

If you are using an older version of CEPH, eg 10.2.x Jewel, then you
may need to supply the parameter `--s3-upload-cutoff 0` or put this in
the config file as `upload_cutoff 0` to work around a bug which causes
uploading of small files to fail.

Note also that Ceph sometimes puts `/` in the passwords it gives
users.  If you read the secret access key using the command line tools
you will get a JSON blob with the `/` escaped as `\/`.  Make sure you
only write `/` in the secret access key.

Eg the dump from Ceph looks something like this (irrelevant keys
removed).

```
{
    "user_id": "xxx",
    "display_name": "xxxx",
    "keys": [
        {
            "user": "xxx",
            "access_key": "xxxxxx",
            "secret_key": "xxxxxx\/xxxx"
        }
    ],
}
```

Because this is a json dump, it is encoding the `/` as `\/`, so if you
use the secret key as `xxxxxx/xxxx`  it will work fine.

### Dreamhost ###

Dreamhost [DreamObjects](https://www.dreamhost.com/cloud/storage/) is
an object storage system based on CEPH.

To use rclone with Dreamhost, configure as above but leave the region blank
and set the endpoint.  You should end up with something like this in
your config:

```
[dreamobjects]
type = s3
provider = DreamHost
env_auth = false
access_key_id = your_access_key
secret_access_key = your_secret_key
region =
endpoint = objects-us-west-1.dream.io
location_constraint =
acl = private
server_side_encryption =
storage_class =
```

### DigitalOcean Spaces ###

[Spaces](https://www.digitalocean.com/products/object-storage/) is an [S3-interoperable](https://developers.digitalocean.com/documentation/spaces/) object storage service from cloud provider DigitalOcean.

To connect to DigitalOcean Spaces you will need an access key and secret key. These can be retrieved on the "[Applications & API](https://cloud.digitalocean.com/settings/api/tokens)" page of the DigitalOcean control panel. They will be needed when prompted by `rclone config` for your `access_key_id` and `secret_access_key`.

When prompted for a `region` or `location_constraint`, press enter to use the default value. The region must be included in the `endpoint` setting (e.g. `nyc3.digitaloceanspaces.com`). The default values can be used for other settings.

Going through the whole process of creating a new remote by running `rclone config`, each prompt should be answered as shown below:

```
Storage> s3
env_auth> 1
access_key_id> YOUR_ACCESS_KEY
secret_access_key> YOUR_SECRET_KEY
region>
endpoint> nyc3.digitaloceanspaces.com
location_constraint>
acl>
storage_class>
```

The resulting configuration file should look like:

```
[spaces]
type = s3
provider = DigitalOcean
env_auth = false
access_key_id = YOUR_ACCESS_KEY
secret_access_key = YOUR_SECRET_KEY
region =
endpoint = nyc3.digitaloceanspaces.com
location_constraint =
acl =
server_side_encryption =
storage_class =
```

Once configured, you can create a new Space and begin copying files. For example:

```
rclone mkdir spaces:my-new-space
rclone copy /path/to/files spaces:my-new-space
```

### IBM COS (S3) ###

Information stored with IBM Cloud Object Storage is encrypted and dispersed across multiple geographic locations, and accessed through an implementation of the S3 API. This service makes use of the distributed storage technologies provided by IBM’s Cloud Object Storage System (formerly Cleversafe). For more information visit: (http://www.ibm.com/cloud/object-storage)

To configure access to IBM COS S3, follow the steps below:

1. Run rclone config and select n for a new remote.
```
	2018/02/14 14:13:11 NOTICE: Config file "C:\\Users\\a\\.config\\rclone\\rclone.conf" not found - using defaults
	No remotes found - make a new one
	n) New remote
	s) Set configuration password
	q) Quit config
	n/s/q> n
```

2. Enter the name for the configuration
```
	name> <YOUR NAME>
```

3. Select "s3" storage.
```
Choose a number from below, or type in your own value
 	1 / Alias for an existing remote
   	\ "alias"
 	2 / Amazon Drive
   	\ "amazon cloud drive"
 	3 / Amazon S3 Complaint Storage Providers (Dreamhost, Ceph, Minio, IBM COS)
   	\ "s3"
 	4 / Backblaze B2
   	\ "b2"
[snip]
	23 / http Connection
    \ "http"
Storage> 3
```

4. Select IBM COS as the S3 Storage Provider.
```
Choose the S3 provider.
Choose a number from below, or type in your own value
	 1 / Choose this option to configure Storage to AWS S3
	   \ "AWS"
 	 2 / Choose this option to configure Storage to Ceph Systems
  	 \ "Ceph"
	 3 /  Choose this option to configure Storage to Dreamhost
     \ "Dreamhost"
   4 / Choose this option to the configure Storage to IBM COS S3
   	 \ "IBMCOS"
 	 5 / Choose this option to the configure Storage to Minio
     \ "Minio"
	 Provider>4
```

5. Enter the Access Key and Secret.
```
	AWS Access Key ID - leave blank for anonymous access or runtime credentials.
	access_key_id> <>
	AWS Secret Access Key (password) - leave blank for anonymous access or runtime credentials.
	secret_access_key> <>
```

6. Specify the endpoint for IBM COS. For Public IBM COS, choose from the option below. For On Premise IBM COS, enter an enpoint address.
```
	Endpoint for IBM COS S3 API.
	Specify if using an IBM COS On Premise.
	Choose a number from below, or type in your own value
	 1 / US Cross Region Endpoint
   	   \ "s3-api.us-geo.objectstorage.softlayer.net"
	 2 / US Cross Region Dallas Endpoint
   	   \ "s3-api.dal.us-geo.objectstorage.softlayer.net"
 	 3 / US Cross Region Washington DC Endpoint
   	   \ "s3-api.wdc-us-geo.objectstorage.softlayer.net"
	 4 / US Cross Region San Jose Endpoint
	   \ "s3-api.sjc-us-geo.objectstorage.softlayer.net"
	 5 / US Cross Region Private Endpoint
	   \ "s3-api.us-geo.objectstorage.service.networklayer.com"
	 6 / US Cross Region Dallas Private Endpoint
	   \ "s3-api.dal-us-geo.objectstorage.service.networklayer.com"
	 7 / US Cross Region Washington DC Private Endpoint
	   \ "s3-api.wdc-us-geo.objectstorage.service.networklayer.com"
	 8 / US Cross Region San Jose Private Endpoint
	   \ "s3-api.sjc-us-geo.objectstorage.service.networklayer.com"
	 9 / US Region East Endpoint
	   \ "s3.us-east.objectstorage.softlayer.net"
	10 / US Region East Private Endpoint
	   \ "s3.us-east.objectstorage.service.networklayer.com"
	11 / US Region South Endpoint
[snip]
	34 / Toronto Single Site Private Endpoint
	   \ "s3.tor01.objectstorage.service.networklayer.com"
	endpoint>1
```


7. Specify a IBM COS Location Constraint. The location constraint must match endpoint when using IBM Cloud Public. For on-prem COS, do not make a selection from this list, hit enter
```
	 1 / US Cross Region Standard
	   \ "us-standard"
	 2 / US Cross Region Vault
	   \ "us-vault"
	 3 / US Cross Region Cold
	   \ "us-cold"
	 4 / US Cross Region Flex
	   \ "us-flex"
	 5 / US East Region Standard
	   \ "us-east-standard"
	 6 / US East Region Vault
	   \ "us-east-vault"
	 7 / US East Region Cold
	   \ "us-east-cold"
	 8 / US East Region Flex
	   \ "us-east-flex"
	 9 / US South Region Standard
	   \ "us-south-standard"
	10 / US South Region Vault
	   \ "us-south-vault"
[snip]
	32 / Toronto Flex
	   \ "tor01-flex"
location_constraint>1
```

9. Specify a canned ACL. IBM Cloud (Strorage) supports "public-read" and "private". IBM Cloud(Infra) supports all the canned ACLs. On-Premise COS supports all the canned ACLs.
```
Canned ACL used when creating buckets and/or storing objects in S3.
For more info visit https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
Choose a number from below, or type in your own value
      1 / Owner gets FULL_CONTROL. No one else has access rights (default). This acl is available on IBM Cloud (Infra), IBM Cloud (Storage), On-Premise COS
      \ "private"
      2  / Owner gets FULL_CONTROL. The AllUsers group gets READ access. This acl is available on IBM Cloud (Infra), IBM Cloud (Storage), On-Premise IBM COS
      \ "public-read"
      3 / Owner gets FULL_CONTROL. The AllUsers group gets READ and WRITE access. This acl is available on IBM Cloud (Infra), On-Premise IBM COS
      \ "public-read-write"
      4  / Owner gets FULL_CONTROL. The AuthenticatedUsers group gets READ access. Not supported on Buckets. This acl is available on IBM Cloud (Infra) and On-Premise IBM COS
      \ "authenticated-read"
acl> 1
```


12. Review the displayed configuration and accept to save the "remote" then quit. The config file should look like this
```
	[xxx]
	type = s3
	Provider = IBMCOS
	access_key_id = xxx
	secret_access_key = yyy
	endpoint = s3-api.us-geo.objectstorage.softlayer.net
	location_constraint = us-standard
	acl = private
```

13. Execute rclone commands
```
	1)	Create a bucket.
		rclone mkdir IBM-COS-XREGION:newbucket
	2)	List available buckets.
		rclone lsd IBM-COS-XREGION:
		-1 2017-11-08 21:16:22        -1 test
		-1 2018-02-14 20:16:39        -1 newbucket
	3)	List contents of a bucket.
		rclone ls IBM-COS-XREGION:newbucket
		18685952 test.exe
	4)	Copy a file from local to remote.
		rclone copy /Users/file.txt IBM-COS-XREGION:newbucket
	5)	Copy a file from remote to local.
		rclone copy IBM-COS-XREGION:newbucket/file.txt .
	6)	Delete a file on remote.
		rclone delete IBM-COS-XREGION:newbucket/file.txt
```

### Minio ###

[Minio](https://minio.io/) is an object storage server built for cloud application developers and devops.

It is very easy to install and provides an S3 compatible server which can be used by rclone.

To use it, install Minio following the instructions [here](https://docs.minio.io/docs/minio-quickstart-guide).

When it configures itself Minio will print something like this

```
Endpoint:  http://192.168.1.106:9000  http://172.23.0.1:9000
AccessKey: USWUXHGYZQYFYFFIT3RE
SecretKey: MOJRH0mkL1IPauahWITSVvyDrQbEEIwljvmxdq03
Region:    us-east-1
SQS ARNs:  arn:minio:sqs:us-east-1:1:redis arn:minio:sqs:us-east-1:2:redis

Browser Access:
   http://192.168.1.106:9000  http://172.23.0.1:9000

Command-line Access: https://docs.minio.io/docs/minio-client-quickstart-guide
   $ mc config host add myminio http://192.168.1.106:9000 USWUXHGYZQYFYFFIT3RE MOJRH0mkL1IPauahWITSVvyDrQbEEIwljvmxdq03

Object API (Amazon S3 compatible):
   Go:         https://docs.minio.io/docs/golang-client-quickstart-guide
   Java:       https://docs.minio.io/docs/java-client-quickstart-guide
   Python:     https://docs.minio.io/docs/python-client-quickstart-guide
   JavaScript: https://docs.minio.io/docs/javascript-client-quickstart-guide
   .NET:       https://docs.minio.io/docs/dotnet-client-quickstart-guide

Drive Capacity: 26 GiB Free, 165 GiB Total
```

These details need to go into `rclone config` like this.  Note that it
is important to put the region in as stated above.

```
env_auth> 1
access_key_id> USWUXHGYZQYFYFFIT3RE
secret_access_key> MOJRH0mkL1IPauahWITSVvyDrQbEEIwljvmxdq03
region> us-east-1
endpoint> http://192.168.1.106:9000
location_constraint>
server_side_encryption>
```

Which makes the config file look like this

```
[minio]
type = s3
provider = Minio
env_auth = false
access_key_id = USWUXHGYZQYFYFFIT3RE
secret_access_key = MOJRH0mkL1IPauahWITSVvyDrQbEEIwljvmxdq03
region = us-east-1
endpoint = http://192.168.1.106:9000
location_constraint =
server_side_encryption =
```

So once set up, for example to copy files into a bucket

```
rclone copy /path/to/files minio:bucket
```

### Scaleway {#scaleway}

[Scaleway](https://www.scaleway.com/object-storage/) The Object Storage platform allows you to store anything from backups, logs and web assets to documents and photos.
Files can be dropped from the Scaleway console or transferred through our API and CLI or using any S3-compatible tool.

Scaleway provides an S3 interface which can be configured for use with rclone like this:

```
[scaleway]
type = s3
env_auth = false
endpoint = s3.nl-ams.scw.cloud
access_key_id = SCWXXXXXXXXXXXXXX
secret_access_key = 1111111-2222-3333-44444-55555555555555
region = nl-ams
location_constraint =
acl = private
force_path_style = false
server_side_encryption =
storage_class =
```

### Wasabi ###

[Wasabi](https://wasabi.com) is a cloud-based object storage service for a
broad range of applications and use cases. Wasabi is designed for
individuals and organizations that require a high-performance,
reliable, and secure data storage infrastructure at minimal cost.

Wasabi provides an S3 interface which can be configured for use with
rclone like this.

```
No remotes found - make a new one
n) New remote
s) Set configuration password
n/s> n
name> wasabi
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Amazon S3 (also Dreamhost, Ceph, Minio)
   \ "s3"
[snip]
Storage> s3
Get AWS credentials from runtime (environment variables or EC2/ECS meta data if no env vars). Only applies if access_key_id and secret_access_key is blank.
Choose a number from below, or type in your own value
 1 / Enter AWS credentials in the next step
   \ "false"
 2 / Get AWS credentials from the environment (env vars or IAM)
   \ "true"
env_auth> 1
AWS Access Key ID - leave blank for anonymous access or runtime credentials.
access_key_id> YOURACCESSKEY
AWS Secret Access Key (password) - leave blank for anonymous access or runtime credentials.
secret_access_key> YOURSECRETACCESSKEY
Region to connect to.
Choose a number from below, or type in your own value
   / The default endpoint - a good choice if you are unsure.
 1 | US Region, Northern Virginia or Pacific Northwest.
   | Leave location constraint empty.
   \ "us-east-1"
[snip]
region> us-east-1
Endpoint for S3 API.
Leave blank if using AWS to use the default endpoint for the region.
Specify if using an S3 clone such as Ceph.
endpoint> s3.wasabisys.com
Location constraint - must be set to match the Region. Used when creating buckets only.
Choose a number from below, or type in your own value
 1 / Empty for US Region, Northern Virginia or Pacific Northwest.
   \ ""
[snip]
location_constraint>
Canned ACL used when creating buckets and/or storing objects in S3.
For more info visit https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
Choose a number from below, or type in your own value
 1 / Owner gets FULL_CONTROL. No one else has access rights (default).
   \ "private"
[snip]
acl>
The server-side encryption algorithm used when storing this object in S3.
Choose a number from below, or type in your own value
 1 / None
   \ ""
 2 / AES256
   \ "AES256"
server_side_encryption>
The storage class to use when storing objects in S3.
Choose a number from below, or type in your own value
 1 / Default
   \ ""
 2 / Standard storage class
   \ "STANDARD"
 3 / Reduced redundancy storage class
   \ "REDUCED_REDUNDANCY"
 4 / Standard Infrequent Access storage class
   \ "STANDARD_IA"
storage_class>
Remote config
--------------------
[wasabi]
env_auth = false
access_key_id = YOURACCESSKEY
secret_access_key = YOURSECRETACCESSKEY
region = us-east-1
endpoint = s3.wasabisys.com
location_constraint =
acl =
server_side_encryption =
storage_class =
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This will leave the config file looking like this.

```
[wasabi]
type = s3
provider = Wasabi
env_auth = false
access_key_id = YOURACCESSKEY
secret_access_key = YOURSECRETACCESSKEY
region =
endpoint = s3.wasabisys.com
location_constraint =
acl =
server_side_encryption =
storage_class =
```

### Alibaba OSS {#alibaba-oss}

Here is an example of making an [Alibaba Cloud (Aliyun) OSS](https://www.alibabacloud.com/product/oss/)
configuration.  First run:

    rclone config

This will guide you through an interactive setup process.

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> oss
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
 4 / Amazon S3 Compliant Storage Provider (AWS, Alibaba, Ceph, Digital Ocean, Dreamhost, IBM COS, Minio, etc)
   \ "s3"
[snip]
Storage> s3
Choose your S3 provider.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / Amazon Web Services (AWS) S3
   \ "AWS"
 2 / Alibaba Cloud Object Storage System (OSS) formerly Aliyun
   \ "Alibaba"
 3 / Ceph Object Storage
   \ "Ceph"
[snip]
provider> Alibaba
Get AWS credentials from runtime (environment variables or EC2/ECS meta data if no env vars).
Only applies if access_key_id and secret_access_key is blank.
Enter a boolean value (true or false). Press Enter for the default ("false").
Choose a number from below, or type in your own value
 1 / Enter AWS credentials in the next step
   \ "false"
 2 / Get AWS credentials from the environment (env vars or IAM)
   \ "true"
env_auth> 1
AWS Access Key ID.
Leave blank for anonymous access or runtime credentials.
Enter a string value. Press Enter for the default ("").
access_key_id> accesskeyid
AWS Secret Access Key (password)
Leave blank for anonymous access or runtime credentials.
Enter a string value. Press Enter for the default ("").
secret_access_key> secretaccesskey
Endpoint for OSS API.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / East China 1 (Hangzhou)
   \ "oss-cn-hangzhou.aliyuncs.com"
 2 / East China 2 (Shanghai)
   \ "oss-cn-shanghai.aliyuncs.com"
 3 / North China 1 (Qingdao)
   \ "oss-cn-qingdao.aliyuncs.com"
[snip]
endpoint> 1
Canned ACL used when creating buckets and storing or copying objects.

Note that this ACL is applied when server side copying objects as S3
doesn't copy the ACL from the source but rather writes a fresh one.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / Owner gets FULL_CONTROL. No one else has access rights (default).
   \ "private"
 2 / Owner gets FULL_CONTROL. The AllUsers group gets READ access.
   \ "public-read"
   / Owner gets FULL_CONTROL. The AllUsers group gets READ and WRITE access.
[snip]
acl> 1
The storage class to use when storing new objects in OSS.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / Default
   \ ""
 2 / Standard storage class
   \ "STANDARD"
 3 / Archive storage mode.
   \ "GLACIER"
 4 / Infrequent access storage mode.
   \ "STANDARD_IA"
storage_class> 1
Edit advanced config? (y/n)
y) Yes
n) No
y/n> n
Remote config
--------------------
[oss]
type = s3
provider = Alibaba
env_auth = false
access_key_id = accesskeyid
secret_access_key = secretaccesskey
endpoint = oss-cn-hangzhou.aliyuncs.com
acl = private
storage_class = Standard
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Netease NOS  ###

For Netease NOS configure as per the configurator `rclone config`
setting the provider `Netease`.  This will automatically set
`force_path_style = false` which is necessary for it to run properly.

 Backblaze B2
----------------------------------------

B2 is [Backblaze's cloud storage system](https://www.backblaze.com/b2/).

Paths are specified as `remote:bucket` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, eg `remote:bucket/path/to/dir`.

Here is an example of making a b2 configuration.  First run

    rclone config

This will guide you through an interactive setup process.  To authenticate
you will either need your Account ID (a short hex number) and Master
Application Key (a long hex number) OR an Application Key, which is the
recommended method. See below for further details on generating and using
an Application Key.

```
No remotes found - make a new one
n) New remote
q) Quit config
n/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Backblaze B2
   \ "b2"
[snip]
Storage> b2
Account ID or Application Key ID
account> 123456789abc
Application Key
key> 0123456789abcdef0123456789abcdef0123456789
Endpoint for the service - leave blank normally.
endpoint>
Remote config
--------------------
[remote]
account = 123456789abc
key = 0123456789abcdef0123456789abcdef0123456789
endpoint =
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `remote` and can now be used like this

See all buckets

    rclone lsd remote:

Create a new bucket

    rclone mkdir remote:bucket

List the contents of a bucket

    rclone ls remote:bucket

Sync `/home/local/directory` to the remote bucket, deleting any
excess files in the bucket.

    rclone sync /home/local/directory remote:bucket

### Application Keys ###

B2 supports multiple [Application Keys for different access permission
to B2 Buckets](https://www.backblaze.com/b2/docs/application_keys.html).

You can use these with rclone too; you will need to use rclone version 1.43
or later.

Follow Backblaze's docs to create an Application Key with the required
permission and add the `applicationKeyId` as the `account` and the
`Application Key` itself as the `key`.

Note that you must put the _applicationKeyId_ as the `account` – you
can't use the master Account ID.  If you try then B2 will return 401
errors.

### --fast-list ###

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](https://rclone.org/docs/#fast-list) for more details.

### Modified time ###

The modified time is stored as metadata on the object as
`X-Bz-Info-src_last_modified_millis` as milliseconds since 1970-01-01
in the Backblaze standard.  Other tools should be able to use this as
a modified time.

Modified times are used in syncing and are fully supported. Note that
if a modification time needs to be updated on an object then it will
create a new version of the object.

#### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| \         | 0x5C  | ＼           |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.

### SHA1 checksums ###

The SHA1 checksums of the files are checked on upload and download and
will be used in the syncing process.

Large files (bigger than the limit in `--b2-upload-cutoff`) which are
uploaded in chunks will store their SHA1 on the object as
`X-Bz-Info-large_file_sha1` as recommended by Backblaze.

For a large file to be uploaded with an SHA1 checksum, the source
needs to support SHA1 checksums. The local disk supports SHA1
checksums so large file transfers from local disk will have an SHA1.
See [the overview](https://rclone.org/overview/#features) for exactly which remotes
support SHA1.

Sources which don't support SHA1, in particular `crypt` will upload
large files without SHA1 checksums.  This may be fixed in the future
(see [#1767](https://github.com/rclone/rclone/issues/1767)).

Files sizes below `--b2-upload-cutoff` will always have an SHA1
regardless of the source.

### Transfers ###

Backblaze recommends that you do lots of transfers simultaneously for
maximum speed.  In tests from my SSD equipped laptop the optimum
setting is about `--transfers 32` though higher numbers may be used
for a slight speed improvement. The optimum number for you may vary
depending on your hardware, how big the files are, how much you want
to load your computer, etc.  The default of `--transfers 4` is
definitely too low for Backblaze B2 though.

Note that uploading big files (bigger than 200 MB by default) will use
a 96 MB RAM buffer by default.  There can be at most `--transfers` of
these in use at any moment, so this sets the upper limit on the memory
used.

### Versions ###

When rclone uploads a new version of a file it creates a [new version
of it](https://www.backblaze.com/b2/docs/file_versions.html).
Likewise when you delete a file, the old version will be marked hidden
and still be available.  Conversely, you may opt in to a "hard delete"
of files with the `--b2-hard-delete` flag which would permanently remove
the file instead of hiding it.

Old versions of files, where available, are visible using the 
`--b2-versions` flag.

**NB** Note that `--b2-versions` does not work with crypt at the
moment [#1627](https://github.com/rclone/rclone/issues/1627). Using
[--backup-dir](https://rclone.org/docs/#backup-dir-dir) with rclone is the recommended
way of working around this.

If you wish to remove all the old versions then you can use the
`rclone cleanup remote:bucket` command which will delete all the old
versions of files, leaving the current ones intact.  You can also
supply a path and only old versions under that path will be deleted,
eg `rclone cleanup remote:bucket/path/to/stuff`.

Note that `cleanup` will remove partially uploaded files from the bucket
if they are more than a day old.

When you `purge` a bucket, the current and the old versions will be
deleted then the bucket will be deleted.

However `delete` will cause the current versions of the files to
become hidden old versions.

Here is a session showing the listing and retrieval of an old
version followed by a `cleanup` of the old versions.

Show current version and all the versions with `--b2-versions` flag.

```
$ rclone -q ls b2:cleanup-test
        9 one.txt

$ rclone -q --b2-versions ls b2:cleanup-test
        9 one.txt
        8 one-v2016-07-04-141032-000.txt
       16 one-v2016-07-04-141003-000.txt
       15 one-v2016-07-02-155621-000.txt
```

Retrieve an old version

```
$ rclone -q --b2-versions copy b2:cleanup-test/one-v2016-07-04-141003-000.txt /tmp

$ ls -l /tmp/one-v2016-07-04-141003-000.txt
-rw-rw-r-- 1 ncw ncw 16 Jul  2 17:46 /tmp/one-v2016-07-04-141003-000.txt
```

Clean up all the old versions and show that they've gone.

```
$ rclone -q cleanup b2:cleanup-test

$ rclone -q ls b2:cleanup-test
        9 one.txt

$ rclone -q --b2-versions ls b2:cleanup-test
        9 one.txt
```

### Data usage ###

It is useful to know how many requests are sent to the server in different scenarios.

All copy commands send the following 4 requests:

```
/b2api/v1/b2_authorize_account
/b2api/v1/b2_create_bucket
/b2api/v1/b2_list_buckets
/b2api/v1/b2_list_file_names
```

The `b2_list_file_names` request will be sent once for every 1k files
in the remote path, providing the checksum and modification time of
the listed files. As of version 1.33 issue
[#818](https://github.com/rclone/rclone/issues/818) causes extra requests
to be sent when using B2 with Crypt. When a copy operation does not
require any files to be uploaded, no more requests will be sent.

Uploading files that do not require chunking, will send 2 requests per
file upload:

```
/b2api/v1/b2_get_upload_url
/b2api/v1/b2_upload_file/
```

Uploading files requiring chunking, will send 2 requests (one each to
start and finish the upload) and another 2 requests for each chunk:

```
/b2api/v1/b2_start_large_file
/b2api/v1/b2_get_upload_part_url
/b2api/v1/b2_upload_part/
/b2api/v1/b2_finish_large_file
```

#### Versions ####

Versions can be viewed with the `--b2-versions` flag. When it is set
rclone will show and act on older versions of files.  For example

Listing without `--b2-versions`

```
$ rclone -q ls b2:cleanup-test
        9 one.txt
```

And with

```
$ rclone -q --b2-versions ls b2:cleanup-test
        9 one.txt
        8 one-v2016-07-04-141032-000.txt
       16 one-v2016-07-04-141003-000.txt
       15 one-v2016-07-02-155621-000.txt
```

Showing that the current version is unchanged but older versions can
be seen.  These have the UTC date that they were uploaded to the
server to the nearest millisecond appended to them.

Note that when using `--b2-versions` no file write operations are
permitted, so you can't upload files or delete them.

### B2 and rclone link ###

Rclone supports generating file share links for private B2 buckets.
They can either be for a file for example:

```
./rclone link B2:bucket/path/to/file.txt
https://f002.backblazeb2.com/file/bucket/path/to/file.txt?Authorization=xxxxxxxx

```

or if run on a directory you will get:

```
./rclone link B2:bucket/path
https://f002.backblazeb2.com/file/bucket/path?Authorization=xxxxxxxx
```

you can then use the authorization token (the part of the url from the
 `?Authorization=` on) on any file path under that directory. For example:

```
https://f002.backblazeb2.com/file/bucket/path/to/file1?Authorization=xxxxxxxx
https://f002.backblazeb2.com/file/bucket/path/file2?Authorization=xxxxxxxx
https://f002.backblazeb2.com/file/bucket/path/folder/file3?Authorization=xxxxxxxx

```


### Standard Options

Here are the standard options specific to b2 (Backblaze B2).

#### --b2-account

Account ID or Application Key ID

- Config:      account
- Env Var:     RCLONE_B2_ACCOUNT
- Type:        string
- Default:     ""

#### --b2-key

Application Key

- Config:      key
- Env Var:     RCLONE_B2_KEY
- Type:        string
- Default:     ""

#### --b2-hard-delete

Permanently delete files on remote removal, otherwise hide files.

- Config:      hard_delete
- Env Var:     RCLONE_B2_HARD_DELETE
- Type:        bool
- Default:     false

### Advanced Options

Here are the advanced options specific to b2 (Backblaze B2).

#### --b2-endpoint

Endpoint for the service.
Leave blank normally.

- Config:      endpoint
- Env Var:     RCLONE_B2_ENDPOINT
- Type:        string
- Default:     ""

#### --b2-test-mode

A flag string for X-Bz-Test-Mode header for debugging.

This is for debugging purposes only. Setting it to one of the strings
below will cause b2 to return specific errors:

  * "fail_some_uploads"
  * "expire_some_account_authorization_tokens"
  * "force_cap_exceeded"

These will be set in the "X-Bz-Test-Mode" header which is documented
in the [b2 integrations checklist](https://www.backblaze.com/b2/docs/integration_checklist.html).

- Config:      test_mode
- Env Var:     RCLONE_B2_TEST_MODE
- Type:        string
- Default:     ""

#### --b2-versions

Include old versions in directory listings.
Note that when using this no file write operations are permitted,
so you can't upload files or delete them.

- Config:      versions
- Env Var:     RCLONE_B2_VERSIONS
- Type:        bool
- Default:     false

#### --b2-upload-cutoff

Cutoff for switching to chunked upload.

Files above this size will be uploaded in chunks of "--b2-chunk-size".

This value should be set no larger than 4.657GiB (== 5GB).

- Config:      upload_cutoff
- Env Var:     RCLONE_B2_UPLOAD_CUTOFF
- Type:        SizeSuffix
- Default:     200M

#### --b2-chunk-size

Upload chunk size. Must fit in memory.

When uploading large files, chunk the file into this size.  Note that
these chunks are buffered in memory and there might a maximum of
"--transfers" chunks in progress at once.  5,000,000 Bytes is the
minimum size.

- Config:      chunk_size
- Env Var:     RCLONE_B2_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     96M

#### --b2-disable-checksum

Disable checksums for large (> upload cutoff) files

Normally rclone will calculate the SHA1 checksum of the input before
uploading it so it can add it to metadata on the object. This is great
for data integrity checking but can cause long delays for large files
to start uploading.

- Config:      disable_checksum
- Env Var:     RCLONE_B2_DISABLE_CHECKSUM
- Type:        bool
- Default:     false

#### --b2-download-url

Custom endpoint for downloads.

This is usually set to a Cloudflare CDN URL as Backblaze offers
free egress for data downloaded through the Cloudflare network.
This is probably only useful for a public bucket.
Leave blank if you want to use the endpoint provided by Backblaze.

- Config:      download_url
- Env Var:     RCLONE_B2_DOWNLOAD_URL
- Type:        string
- Default:     ""

#### --b2-download-auth-duration

Time before the authorization token will expire in s or suffix ms|s|m|h|d.

The duration before the download authorization token will expire.
The minimum value is 1 second. The maximum value is one week.

- Config:      download_auth_duration
- Env Var:     RCLONE_B2_DOWNLOAD_AUTH_DURATION
- Type:        Duration
- Default:     1w

#### --b2-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_B2_ENCODING
- Type:        MultiEncoder
- Default:     Slash,BackSlash,Del,Ctl,InvalidUtf8,Dot



 Box
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

The initial setup for Box involves getting a token from Box which you
can do either in your browser, or with a config.json downloaded from Box
to use JWT authentication.  `rclone config` walks you through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Box
   \ "box"
[snip]
Storage> box
Box App Client Id - leave blank normally.
client_id> 
Box App Client Secret - leave blank normally.
client_secret>
Box App config.json location
Leave blank normally.
Enter a string value. Press Enter for the default ("").
config_json>
'enterprise' or 'user' depending on the type of token being requested.
Enter a string value. Press Enter for the default ("user").
box_sub_type>
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
client_id = 
client_secret = 
token = {"access_token":"XXX","token_type":"bearer","refresh_token":"XXX","expiry":"XXX"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](https://rclone.org/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Box. This only runs from the moment it opens
your browser to the moment you get back the verification code.  This
is on `http://127.0.0.1:53682/` and this it may require you to unblock
it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List directories in top level of your Box

    rclone lsd remote:

List all the files in your Box

    rclone ls remote:

To copy a local directory to an Box directory called backup

    rclone copy /home/source remote:backup

### Using rclone with an Enterprise account with SSO ###

If you have an "Enterprise" account type with Box with single sign on
(SSO), you need to create a password to use Box with rclone. This can
be done at your Enterprise Box account by going to Settings, "Account"
Tab, and then set the password in the "Authentication" field.

Once you have done this, you can setup your Enterprise Box account
using the same procedure detailed above in the, using the password you
have just set.

### Invalid refresh token ###

According to the [box docs](https://developer.box.com/v2.0/docs/oauth-20#section-6-using-the-access-and-refresh-tokens):

> Each refresh_token is valid for one use in 60 days.

This means that if you

  * Don't use the box remote for 60 days
  * Copy the config file with a box refresh token in and use it in two places
  * Get an error on a token refresh

then rclone will return an error which includes the text `Invalid
refresh token`.

To fix this you will need to use oauth2 again to update the refresh
token.  You can use the methods in [the remote setup
docs](https://rclone.org/remote_setup/), bearing in mind that if you use the copy the
config file method, you should not use that remote on the computer you
did the authentication on.

Here is how to do it.

```
$ rclone config
Current remotes:

Name                 Type
====                 ====
remote               box

e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> e
Choose a number from below, or type in an existing value
 1 > remote
remote> remote
--------------------
[remote]
type = box
token = {"access_token":"XXX","token_type":"bearer","refresh_token":"XXX","expiry":"2017-07-08T23:40:08.059167677+01:00"}
--------------------
Edit remote
Value "client_id" = ""
Edit? (y/n)>
y) Yes
n) No
y/n> n
Value "client_secret" = ""
Edit? (y/n)>
y) Yes
n) No
y/n> n
Remote config
Already have a token - refresh?
y) Yes
n) No
y/n> y
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
type = box
token = {"access_token":"YYY","token_type":"bearer","refresh_token":"YYY","expiry":"2017-07-23T12:22:29.259137901+01:00"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Modified time and hashes ###

Box allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.

Box supports SHA1 type hashes, so you can use the `--checksum`
flag.

#### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| \         | 0x5C  | ＼           |

File names can also not end with the following characters.
These only get replaced if they are the last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Transfers ###

For files above 50MB rclone will use a chunked transfer.  Rclone will
upload up to `--transfers` chunks at the same time (shared among all
the multipart uploads).  Chunks are buffered in memory and are
normally 8MB so increasing `--transfers` will increase memory use.

### Deleting files ###

Depending on the enterprise settings for your user, the item will
either be actually deleted from Box or moved to the trash.

### Root folder ID ###

You can set the `root_folder_id` for rclone.  This is the directory
(identified by its `Folder ID`) that rclone considers to be the root
of your Box drive.

Normally you will leave this blank and rclone will determine the
correct root to use itself.

However you can set this to restrict rclone to a specific folder
hierarchy.

In order to do this you will have to find the `Folder ID` of the
directory you wish rclone to display.  This will be the last segment
of the URL when you open the relevant folder in the Box web
interface.

So if the folder you want rclone to use has a URL which looks like
`https://app.box.com/folder/11xxxxxxxxx8`
in the browser, then you use `11xxxxxxxxx8` as
the `root_folder_id` in the config.


### Standard Options

Here are the standard options specific to box (Box).

#### --box-client-id

Box App Client Id.
Leave blank normally.

- Config:      client_id
- Env Var:     RCLONE_BOX_CLIENT_ID
- Type:        string
- Default:     ""

#### --box-client-secret

Box App Client Secret
Leave blank normally.

- Config:      client_secret
- Env Var:     RCLONE_BOX_CLIENT_SECRET
- Type:        string
- Default:     ""

#### --box-box-config-file

Box App config.json location
Leave blank normally.

- Config:      box_config_file
- Env Var:     RCLONE_BOX_BOX_CONFIG_FILE
- Type:        string
- Default:     ""

#### --box-box-sub-type



- Config:      box_sub_type
- Env Var:     RCLONE_BOX_BOX_SUB_TYPE
- Type:        string
- Default:     "user"
- Examples:
    - "user"
        - Rclone should act on behalf of a user
    - "enterprise"
        - Rclone should act on behalf of a service account

### Advanced Options

Here are the advanced options specific to box (Box).

#### --box-root-folder-id

Fill in for rclone to use a non root folder as its starting point.

- Config:      root_folder_id
- Env Var:     RCLONE_BOX_ROOT_FOLDER_ID
- Type:        string
- Default:     "0"

#### --box-upload-cutoff

Cutoff for switching to multipart upload (>= 50MB).

- Config:      upload_cutoff
- Env Var:     RCLONE_BOX_UPLOAD_CUTOFF
- Type:        SizeSuffix
- Default:     50M

#### --box-commit-retries

Max number of times to try committing a multipart file.

- Config:      commit_retries
- Env Var:     RCLONE_BOX_COMMIT_RETRIES
- Type:        int
- Default:     100

#### --box-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_BOX_ENCODING
- Type:        MultiEncoder
- Default:     Slash,BackSlash,Del,Ctl,RightSpace,InvalidUtf8,Dot



### Limitations ###

Note that Box is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

Box file names can't have the `\` character in.  rclone maps this to
and from an identical looking unicode equivalent `＼`.

Box only supports filenames up to 255 characters in length.

 Cache (BETA)
-----------------------------------------

The `cache` remote wraps another existing remote and stores file structure
and its data for long running tasks like `rclone mount`.

## Status

The cache backend code is working but it currently doesn't
have a maintainer so there are [outstanding bugs](https://github.com/rclone/rclone/issues?q=is%3Aopen+is%3Aissue+label%3Abug+label%3A%22Remote%3A+Cache%22) which aren't getting fixed.

The cache backend is due to be phased out in favour of the VFS caching
layer eventually which is more tightly integrated into rclone.

Until this happens we recommend only using the cache backend if you
find you can't work without it. There are many docs online describing
the use of the cache backend to minimize API hits and by-and-large
these are out of date and the cache backend isn't needed in those
scenarios any more.

## Setup

To get started you just need to have an existing remote which can be configured
with `cache`.

Here is an example of how to make a remote called `test-cache`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
n/r/c/s/q> n
name> test-cache
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Cache a remote
   \ "cache"
[snip]
Storage> cache
Remote to cache.
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).
remote> local:/test
Optional: The URL of the Plex server
plex_url> http://127.0.0.1:32400
Optional: The username of the Plex user
plex_username> dummyusername
Optional: The password of the Plex user
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank
y/g/n> y
Enter the password:
password:
Confirm the password:
password:
The size of a chunk. Lower value good for slow connections but can affect seamless reading.
Default: 5M
Choose a number from below, or type in your own value
 1 / 1MB
   \ "1m"
 2 / 5 MB
   \ "5M"
 3 / 10 MB
   \ "10M"
chunk_size> 2
How much time should object info (file size, file hashes etc) be stored in cache. Use a very high value if you don't plan on changing the source FS from outside the cache.
Accepted units are: "s", "m", "h".
Default: 5m
Choose a number from below, or type in your own value
 1 / 1 hour
   \ "1h"
 2 / 24 hours
   \ "24h"
 3 / 24 hours
   \ "48h"
info_age> 2
The maximum size of stored chunks. When the storage grows beyond this size, the oldest chunks will be deleted.
Default: 10G
Choose a number from below, or type in your own value
 1 / 500 MB
   \ "500M"
 2 / 1 GB
   \ "1G"
 3 / 10 GB
   \ "10G"
chunk_total_size> 3
Remote config
--------------------
[test-cache]
remote = local:/test
plex_url = http://127.0.0.1:32400
plex_username = dummyusername
plex_password = *** ENCRYPTED ***
chunk_size = 5M
info_age = 48h
chunk_total_size = 10G
```

You can then use it like this,

List directories in top level of your drive

    rclone lsd test-cache:

List all the files in your drive

    rclone ls test-cache:

To start a cached mount

    rclone mount --allow-other test-cache: /var/tmp/test-cache

### Write Features ###

### Offline uploading ###

In an effort to make writing through cache more reliable, the backend 
now supports this feature which can be activated by specifying a
`cache-tmp-upload-path`.

A files goes through these states when using this feature:

1. An upload is started (usually by copying a file on the cache remote)
2. When the copy to the temporary location is complete the file is part 
of the cached remote and looks and behaves like any other file (reading included)
3. After `cache-tmp-wait-time` passes and the file is next in line, `rclone move` 
is used to move the file to the cloud provider
4. Reading the file still works during the upload but most modifications on it will be prohibited
5. Once the move is complete the file is unlocked for modifications as it
becomes as any other regular file
6. If the file is being read through `cache` when it's actually
deleted from the temporary path then `cache` will simply swap the source
to the cloud provider without interrupting the reading (small blip can happen though)

Files are uploaded in sequence and only one file is uploaded at a time.
Uploads will be stored in a queue and be processed based on the order they were added.
The queue and the temporary storage is persistent across restarts but
can be cleared on startup with the `--cache-db-purge` flag.

### Write Support ###

Writes are supported through `cache`.
One caveat is that a mounted cache remote does not add any retry or fallback
mechanism to the upload operation. This will depend on the implementation
of the wrapped remote. Consider using `Offline uploading` for reliable writes.

One special case is covered with `cache-writes` which will cache the file
data at the same time as the upload when it is enabled making it available
from the cache store immediately once the upload is finished.

### Read Features ###

#### Multiple connections ####

To counter the high latency between a local PC where rclone is running
and cloud providers, the cache remote can split multiple requests to the
cloud provider for smaller file chunks and combines them together locally
where they can be available almost immediately before the reader usually
needs them.

This is similar to buffering when media files are played online. Rclone
will stay around the current marker but always try its best to stay ahead
and prepare the data before.

#### Plex Integration ####

There is a direct integration with Plex which allows cache to detect during reading
if the file is in playback or not. This helps cache to adapt how it queries
the cloud provider depending on what is needed for.

Scans will have a minimum amount of workers (1) while in a confirmed playback cache
will deploy the configured number of workers.

This integration opens the doorway to additional performance improvements
which will be explored in the near future.

**Note:** If Plex options are not configured, `cache` will function with its
configured options without adapting any of its settings.

How to enable? Run `rclone config` and add all the Plex options (endpoint, username
and password) in your remote and it will be automatically enabled.

Affected settings:
- `cache-workers`: _Configured value_ during confirmed playback or _1_ all the other times

##### Certificate Validation #####

When the Plex server is configured to only accept secure connections, it is
possible to use `.plex.direct` URLs to ensure certificate validation succeeds.
These URLs are used by Plex internally to connect to the Plex server securely.

The format for these URLs is the following:

https://ip-with-dots-replaced.server-hash.plex.direct:32400/

The `ip-with-dots-replaced` part can be any IPv4 address, where the dots
have been replaced with dashes, e.g. `127.0.0.1` becomes `127-0-0-1`.

To get the `server-hash` part, the easiest way is to visit

https://plex.tv/api/resources?includeHttps=1&X-Plex-Token=your-plex-token

This page will list all the available Plex servers for your account
with at least one `.plex.direct` link for each. Copy one URL and replace
the IP address with the desired address. This can be used as the
`plex_url` value.

### Known issues ###

#### Mount and --dir-cache-time ####

--dir-cache-time controls the first layer of directory caching which works at the mount layer.
Being an independent caching mechanism from the `cache` backend, it will manage its own entries
based on the configured time.

To avoid getting in a scenario where dir cache has obsolete data and cache would have the correct
one, try to set `--dir-cache-time` to a lower time than `--cache-info-age`. Default values are
already configured in this way. 

#### Windows support - Experimental ####

There are a couple of issues with Windows `mount` functionality that still require some investigations.
It should be considered as experimental thus far as fixes come in for this OS.

Most of the issues seem to be related to the difference between filesystems
on Linux flavors and Windows as cache is heavily dependent on them.

Any reports or feedback on how cache behaves on this OS is greatly appreciated.
 
- https://github.com/rclone/rclone/issues/1935
- https://github.com/rclone/rclone/issues/1907
- https://github.com/rclone/rclone/issues/1834 

#### Risk of throttling ####

Future iterations of the cache backend will make use of the pooling functionality
of the cloud provider to synchronize and at the same time make writing through it
more tolerant to failures. 

There are a couple of enhancements in track to add these but in the meantime
there is a valid concern that the expiring cache listings can lead to cloud provider
throttles or bans due to repeated queries on it for very large mounts.

Some recommendations:
- don't use a very small interval for entry information (`--cache-info-age`)
- while writes aren't yet optimised, you can still write through `cache` which gives you the advantage
of adding the file in the cache at the same time if configured to do so.

Future enhancements:

- https://github.com/rclone/rclone/issues/1937
- https://github.com/rclone/rclone/issues/1936 

#### cache and crypt ####

One common scenario is to keep your data encrypted in the cloud provider
using the `crypt` remote. `crypt` uses a similar technique to wrap around
an existing remote and handles this translation in a seamless way.

There is an issue with wrapping the remotes in this order:
**cloud remote** -> **crypt** -> **cache**

During testing, I experienced a lot of bans with the remotes in this order.
I suspect it might be related to how crypt opens files on the cloud provider
which makes it think we're downloading the full file instead of small chunks.
Organizing the remotes in this order yields better results:
**cloud remote** -> **cache** -> **crypt**

#### absolute remote paths ####

`cache` can not differentiate between relative and absolute paths for the wrapped remote.
Any path given in the `remote` config setting and on the command line will be passed to
the wrapped remote as is, but for storing the chunks on disk the path will be made
relative by removing any leading `/` character.

This behavior is irrelevant for most backend types, but there are backends where a leading `/`
changes the effective directory, e.g. in the `sftp` backend paths starting with a `/` are
relative to the root of the SSH server and paths without are relative to the user home directory.
As a result `sftp:bin` and `sftp:/bin` will share the same cache folder, even if they represent
a different directory on the SSH server.

### Cache and Remote Control (--rc) ###
Cache supports the new `--rc` mode in rclone and can be remote controlled through the following end points:
By default, the listener is disabled if you do not add the flag.

### rc cache/expire
Purge a remote from the cache backend. Supports either a directory or a file.
It supports both encrypted and unencrypted file names if cache is wrapped by crypt.

Params:
  - **remote** = path to remote **(required)**
  - **withData** = true/false to delete cached data (chunks) as well _(optional, false by default)_


### Standard Options

Here are the standard options specific to cache (Cache a remote).

#### --cache-remote

Remote to cache.
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).

- Config:      remote
- Env Var:     RCLONE_CACHE_REMOTE
- Type:        string
- Default:     ""

#### --cache-plex-url

The URL of the Plex server

- Config:      plex_url
- Env Var:     RCLONE_CACHE_PLEX_URL
- Type:        string
- Default:     ""

#### --cache-plex-username

The username of the Plex user

- Config:      plex_username
- Env Var:     RCLONE_CACHE_PLEX_USERNAME
- Type:        string
- Default:     ""

#### --cache-plex-password

The password of the Plex user

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      plex_password
- Env Var:     RCLONE_CACHE_PLEX_PASSWORD
- Type:        string
- Default:     ""

#### --cache-chunk-size

The size of a chunk (partial file data).

Use lower numbers for slower connections. If the chunk size is
changed, any downloaded chunks will be invalid and cache-chunk-path
will need to be cleared or unexpected EOF errors will occur.

- Config:      chunk_size
- Env Var:     RCLONE_CACHE_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     5M
- Examples:
    - "1m"
        - 1MB
    - "5M"
        - 5 MB
    - "10M"
        - 10 MB

#### --cache-info-age

How long to cache file structure information (directory listings, file size, times etc). 
If all write operations are done through the cache then you can safely make
this value very large as the cache store will also be updated in real time.

- Config:      info_age
- Env Var:     RCLONE_CACHE_INFO_AGE
- Type:        Duration
- Default:     6h0m0s
- Examples:
    - "1h"
        - 1 hour
    - "24h"
        - 24 hours
    - "48h"
        - 48 hours

#### --cache-chunk-total-size

The total size that the chunks can take up on the local disk.

If the cache exceeds this value then it will start to delete the
oldest chunks until it goes under this value.

- Config:      chunk_total_size
- Env Var:     RCLONE_CACHE_CHUNK_TOTAL_SIZE
- Type:        SizeSuffix
- Default:     10G
- Examples:
    - "500M"
        - 500 MB
    - "1G"
        - 1 GB
    - "10G"
        - 10 GB

### Advanced Options

Here are the advanced options specific to cache (Cache a remote).

#### --cache-plex-token

The plex token for authentication - auto set normally

- Config:      plex_token
- Env Var:     RCLONE_CACHE_PLEX_TOKEN
- Type:        string
- Default:     ""

#### --cache-plex-insecure

Skip all certificate verification when connecting to the Plex server

- Config:      plex_insecure
- Env Var:     RCLONE_CACHE_PLEX_INSECURE
- Type:        string
- Default:     ""

#### --cache-db-path

Directory to store file structure metadata DB.
The remote name is used as the DB file name.

- Config:      db_path
- Env Var:     RCLONE_CACHE_DB_PATH
- Type:        string
- Default:     "$HOME/.cache/rclone/cache-backend"

#### --cache-chunk-path

Directory to cache chunk files.

Path to where partial file data (chunks) are stored locally. The remote
name is appended to the final path.

This config follows the "--cache-db-path". If you specify a custom
location for "--cache-db-path" and don't specify one for "--cache-chunk-path"
then "--cache-chunk-path" will use the same path as "--cache-db-path".

- Config:      chunk_path
- Env Var:     RCLONE_CACHE_CHUNK_PATH
- Type:        string
- Default:     "$HOME/.cache/rclone/cache-backend"

#### --cache-db-purge

Clear all the cached data for this remote on start.

- Config:      db_purge
- Env Var:     RCLONE_CACHE_DB_PURGE
- Type:        bool
- Default:     false

#### --cache-chunk-clean-interval

How often should the cache perform cleanups of the chunk storage.
The default value should be ok for most people. If you find that the
cache goes over "cache-chunk-total-size" too often then try to lower
this value to force it to perform cleanups more often.

- Config:      chunk_clean_interval
- Env Var:     RCLONE_CACHE_CHUNK_CLEAN_INTERVAL
- Type:        Duration
- Default:     1m0s

#### --cache-read-retries

How many times to retry a read from a cache storage.

Since reading from a cache stream is independent from downloading file
data, readers can get to a point where there's no more data in the
cache.  Most of the times this can indicate a connectivity issue if
cache isn't able to provide file data anymore.

For really slow connections, increase this to a point where the stream is
able to provide data but your experience will be very stuttering.

- Config:      read_retries
- Env Var:     RCLONE_CACHE_READ_RETRIES
- Type:        int
- Default:     10

#### --cache-workers

How many workers should run in parallel to download chunks.

Higher values will mean more parallel processing (better CPU needed)
and more concurrent requests on the cloud provider.  This impacts
several aspects like the cloud provider API limits, more stress on the
hardware that rclone runs on but it also means that streams will be
more fluid and data will be available much more faster to readers.

**Note**: If the optional Plex integration is enabled then this
setting will adapt to the type of reading performed and the value
specified here will be used as a maximum number of workers to use.

- Config:      workers
- Env Var:     RCLONE_CACHE_WORKERS
- Type:        int
- Default:     4

#### --cache-chunk-no-memory

Disable the in-memory cache for storing chunks during streaming.

By default, cache will keep file data during streaming in RAM as well
to provide it to readers as fast as possible.

This transient data is evicted as soon as it is read and the number of
chunks stored doesn't exceed the number of workers. However, depending
on other settings like "cache-chunk-size" and "cache-workers" this footprint
can increase if there are parallel streams too (multiple files being read
at the same time).

If the hardware permits it, use this feature to provide an overall better
performance during streaming but it can also be disabled if RAM is not
available on the local machine.

- Config:      chunk_no_memory
- Env Var:     RCLONE_CACHE_CHUNK_NO_MEMORY
- Type:        bool
- Default:     false

#### --cache-rps

Limits the number of requests per second to the source FS (-1 to disable)

This setting places a hard limit on the number of requests per second
that cache will be doing to the cloud provider remote and try to
respect that value by setting waits between reads.

If you find that you're getting banned or limited on the cloud
provider through cache and know that a smaller number of requests per
second will allow you to work with it then you can use this setting
for that.

A good balance of all the other settings should make this setting
useless but it is available to set for more special cases.

**NOTE**: This will limit the number of requests during streams but
other API calls to the cloud provider like directory listings will
still pass.

- Config:      rps
- Env Var:     RCLONE_CACHE_RPS
- Type:        int
- Default:     -1

#### --cache-writes

Cache file data on writes through the FS

If you need to read files immediately after you upload them through
cache you can enable this flag to have their data stored in the
cache store at the same time during upload.

- Config:      writes
- Env Var:     RCLONE_CACHE_WRITES
- Type:        bool
- Default:     false

#### --cache-tmp-upload-path

Directory to keep temporary files until they are uploaded.

This is the path where cache will use as a temporary storage for new
files that need to be uploaded to the cloud provider.

Specifying a value will enable this feature. Without it, it is
completely disabled and files will be uploaded directly to the cloud
provider

- Config:      tmp_upload_path
- Env Var:     RCLONE_CACHE_TMP_UPLOAD_PATH
- Type:        string
- Default:     ""

#### --cache-tmp-wait-time

How long should files be stored in local cache before being uploaded

This is the duration that a file must wait in the temporary location
_cache-tmp-upload-path_ before it is selected for upload.

Note that only one file is uploaded at a time and it can take longer
to start the upload if a queue formed for this purpose.

- Config:      tmp_wait_time
- Env Var:     RCLONE_CACHE_TMP_WAIT_TIME
- Type:        Duration
- Default:     15s

#### --cache-db-wait-time

How long to wait for the DB to be available - 0 is unlimited

Only one process can have the DB open at any one time, so rclone waits
for this duration for the DB to become available before it gives an
error.

If you set it to 0 then it will wait forever.

- Config:      db_wait_time
- Env Var:     RCLONE_CACHE_DB_WAIT_TIME
- Type:        Duration
- Default:     1s

### Backend commands

Here are the commands specific to the cache backend.

Run them with

    rclone backend COMMAND remote:

The help below will explain what arguments each command takes.

See [the "rclone backend" command](https://rclone.org/commands/rclone_backend/) for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](https://rclone.org/rc/#backend/command).

#### stats

Print stats on the cache backend in JSON format.

    rclone backend stats remote: [options] [<arguments>+]



Chunker (BETA)
----------------------------------------

The `chunker` overlay transparently splits large files into smaller chunks
during upload to wrapped remote and transparently assembles them back
when the file is downloaded. This allows to effectively overcome size limits
imposed by storage providers.

To use it, first set up the underlying remote following the configuration
instructions for that remote. You can also use a local pathname instead of
a remote.

First check your chosen remote is working - we'll call it `remote:path` here.
Note that anything inside `remote:path` will be chunked and anything outside
won't. This means that if you are using a bucket based remote (eg S3, B2, swift)
then you should probably put the bucket in the remote `s3:bucket`.

Now configure `chunker` using `rclone config`. We will call this one `overlay`
to separate it from the `remote` itself.

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> overlay
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Transparently chunk/split large files
   \ "chunker"
[snip]
Storage> chunker
Remote to chunk/unchunk.
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).
Enter a string value. Press Enter for the default ("").
remote> remote:path
Files larger than chunk size will be split in chunks.
Enter a size with suffix k,M,G,T. Press Enter for the default ("2G").
chunk_size> 100M
Choose how chunker handles hash sums. All modes but "none" require metadata.
Enter a string value. Press Enter for the default ("md5").
Choose a number from below, or type in your own value
 1 / Pass any hash supported by wrapped remote for non-chunked files, return nothing otherwise
   \ "none"
 2 / MD5 for composite files
   \ "md5"
 3 / SHA1 for composite files
   \ "sha1"
 4 / MD5 for all files
   \ "md5all"
 5 / SHA1 for all files
   \ "sha1all"
 6 / Copying a file to chunker will request MD5 from the source falling back to SHA1 if unsupported
   \ "md5quick"
 7 / Similar to "md5quick" but prefers SHA1 over MD5
   \ "sha1quick"
hash_type> md5
Edit advanced config? (y/n)
y) Yes
n) No
y/n> n
Remote config
--------------------
[overlay]
type = chunker
remote = remote:bucket
chunk_size = 100M
hash_type = md5
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Specifying the remote

In normal use, make sure the remote has a `:` in. If you specify the remote
without a `:` then rclone will use a local directory of that name.
So if you use a remote of `/path/to/secret/files` then rclone will
chunk stuff in that directory. If you use a remote of `name` then rclone
will put files in a directory called `name` in the current directory.


### Chunking

When rclone starts a file upload, chunker checks the file size. If it
doesn't exceed the configured chunk size, chunker will just pass the file
to the wrapped remote. If a file is large, chunker will transparently cut
data in pieces with temporary names and stream them one by one, on the fly.
Each data chunk will contain the specified number of bytes, except for the
last one which may have less data. If file size is unknown in advance
(this is called a streaming upload), chunker will internally create
a temporary copy, record its size and repeat the above process.

When upload completes, temporary chunk files are finally renamed.
This scheme guarantees that operations can be run in parallel and look
from outside as atomic.
A similar method with hidden temporary chunks is used for other operations
(copy/move/rename etc). If an operation fails, hidden chunks are normally
destroyed, and the target composite file stays intact.

When a composite file download is requested, chunker transparently
assembles it by concatenating data chunks in order. As the split is trivial
one could even manually concatenate data chunks together to obtain the
original content.

When the `list` rclone command scans a directory on wrapped remote,
the potential chunk files are accounted for, grouped and assembled into
composite directory entries. Any temporary chunks are hidden.

List and other commands can sometimes come across composite files with
missing or invalid chunks, eg. shadowed by like-named directory or
another file. This usually means that wrapped file system has been directly
tampered with or damaged. If chunker detects a missing chunk it will
by default print warning, skip the whole incomplete group of chunks but
proceed with current command.
You can set the `--chunker-fail-hard` flag to have commands abort with
error message in such cases.


#### Chunk names

The default chunk name format is `*.rclone_chunk.###`, hence by default
chunk names are `BIG_FILE_NAME.rclone_chunk.001`,
`BIG_FILE_NAME.rclone_chunk.002` etc. You can configure another name format
using the `name_format` configuration file option. The format uses asterisk
`*` as a placeholder for the base file name and one or more consecutive
hash characters `#` as a placeholder for sequential chunk number.
There must be one and only one asterisk. The number of consecutive hash
characters defines the minimum length of a string representing a chunk number.
If decimal chunk number has less digits than the number of hashes, it is
left-padded by zeros. If the decimal string is longer, it is left intact.
By default numbering starts from 1 but there is another option that allows
user to start from 0, eg. for compatibility with legacy software.

For example, if name format is `big_*-##.part` and original file name is
`data.txt` and numbering starts from 0, then the first chunk will be named
`big_data.txt-00.part`, the 99th chunk will be `big_data.txt-98.part`
and the 302nd chunk will become `big_data.txt-301.part`.

Note that `list` assembles composite directory entries only when chunk names
match the configured format and treats non-conforming file names as normal
non-chunked files.


### Metadata

Besides data chunks chunker will by default create metadata object for
a composite file. The object is named after the original file.
Chunker allows user to disable metadata completely (the `none` format).
Note that metadata is normally not created for files smaller than the
configured chunk size. This may change in future rclone releases.

#### Simple JSON metadata format

This is the default format. It supports hash sums and chunk validation
for composite files. Meta objects carry the following fields:

- `ver`     - version of format, currently `1`
- `size`    - total size of composite file
- `nchunks` - number of data chunks in file
- `md5`     - MD5 hashsum of composite file (if present)
- `sha1`    - SHA1 hashsum (if present)

There is no field for composite file name as it's simply equal to the name
of meta object on the wrapped remote. Please refer to respective sections
for details on hashsums and modified time handling.

#### No metadata

You can disable meta objects by setting the meta format option to `none`.
In this mode chunker will scan directory for all files that follow
configured chunk name format, group them by detecting chunks with the same
base name and show group names as virtual composite files.
This method is more prone to missing chunk errors (especially missing
last chunk) than format with metadata enabled.


### Hashsums

Chunker supports hashsums only when a compatible metadata is present.
Hence, if you choose metadata format of `none`, chunker will report hashsum
as `UNSUPPORTED`.

Please note that by default metadata is stored only for composite files.
If a file is smaller than configured chunk size, chunker will transparently
redirect hash requests to wrapped remote, so support depends on that.
You will see the empty string as a hashsum of requested type for small
files if the wrapped remote doesn't support it.

Many storage backends support MD5 and SHA1 hash types, so does chunker.
With chunker you can choose one or another but not both.
MD5 is set by default as the most supported type.
Since chunker keeps hashes for composite files and falls back to the
wrapped remote hash for non-chunked ones, we advise you to choose the same
hash type as supported by wrapped remote so that your file listings
look coherent.

If your storage backend does not support MD5 or SHA1 but you need consistent
file hashing, configure chunker with `md5all` or `sha1all`. These two modes
guarantee given hash for all files. If wrapped remote doesn't support it,
chunker will then add metadata to all files, even small. However, this can
double the amount of small files in storage and incur additional service charges.
You can even use chunker to force md5/sha1 support in any other remote
at expense of sidecar meta objects by setting eg. `chunk_type=sha1all`
to force hashsums and `chunk_size=1P` to effectively disable chunking.

Normally, when a file is copied to chunker controlled remote, chunker
will ask the file source for compatible file hash and revert to on-the-fly
calculation if none is found. This involves some CPU overhead but provides
a guarantee that given hashsum is available. Also, chunker will reject
a server-side copy or move operation if source and destination hashsum
types are different resulting in the extra network bandwidth, too.
In some rare cases this may be undesired, so chunker provides two optional
choices: `sha1quick` and `md5quick`. If the source does not support primary
hash type and the quick mode is enabled, chunker will try to fall back to
the secondary type. This will save CPU and bandwidth but can result in empty
hashsums at destination. Beware of consequences: the `sync` command will
revert (sometimes silently) to time/size comparison if compatible hashsums
between source and target are not found.


### Modified time

Chunker stores modification times using the wrapped remote so support
depends on that. For a small non-chunked file the chunker overlay simply
manipulates modification time of the wrapped remote file.
For a composite file with metadata chunker will get and set
modification time of the metadata object on the wrapped remote.
If file is chunked but metadata format is `none` then chunker will
use modification time of the first data chunk.


### Migrations

The idiomatic way to migrate to a different chunk size, hash type or
chunk naming scheme is to:

- Collect all your chunked files under a directory and have your
  chunker remote point to it.
- Create another directory (most probably on the same cloud storage)
  and configure a new remote with desired metadata format,
  hash type, chunk naming etc.
- Now run `rclone sync oldchunks: newchunks:` and all your data
  will be transparently converted in transfer.
  This may take some time, yet chunker will try server-side
  copy if possible.
- After checking data integrity you may remove configuration section
  of the old remote.

If rclone gets killed during a long operation on a big composite file,
hidden temporary chunks may stay in the directory. They will not be
shown by the `list` command but will eat up your account quota.
Please note that the `deletefile` command deletes only active
chunks of a file. As a workaround, you can use remote of the wrapped
file system to see them.
An easy way to get rid of hidden garbage is to copy littered directory
somewhere using the chunker remote and purge the original directory.
The `copy` command will copy only active chunks while the `purge` will
remove everything including garbage.


### Caveats and Limitations

Chunker requires wrapped remote to support server side `move` (or `copy` +
`delete`) operations, otherwise it will explicitly refuse to start.
This is because it internally renames temporary chunk files to their final
names when an operation completes successfully.

Chunker encodes chunk number in file name, so with default `name_format`
setting it adds 17 characters. Also chunker adds 7 characters of temporary
suffix during operations. Many file systems limit base file name without path
by 255 characters. Using rclone's crypt remote as a base file system limits
file name by 143 characters. Thus, maximum name length is 231 for most files
and 119 for chunker-over-crypt. A user in need can change name format to
eg. `*.rcc##` and save 10 characters (provided at most 99 chunks per file).

Note that a move implemented using the copy-and-delete method may incur
double charging with some cloud storage providers.

Chunker will not automatically rename existing chunks when you run
`rclone config` on a live remote and change the chunk name format.
Beware that in result of this some files which have been treated as chunks
before the change can pop up in directory listings as normal files
and vice versa. The same warning holds for the chunk size.
If you desperately need to change critical chunking settings, you should
run data migration as described above.

If wrapped remote is case insensitive, the chunker overlay will inherit
that property (so you can't have a file called "Hello.doc" and "hello.doc"
in the same directory).



### Standard Options

Here are the standard options specific to chunker (Transparently chunk/split large files).

#### --chunker-remote

Remote to chunk/unchunk.
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).

- Config:      remote
- Env Var:     RCLONE_CHUNKER_REMOTE
- Type:        string
- Default:     ""

#### --chunker-chunk-size

Files larger than chunk size will be split in chunks.

- Config:      chunk_size
- Env Var:     RCLONE_CHUNKER_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     2G

#### --chunker-hash-type

Choose how chunker handles hash sums. All modes but "none" require metadata.

- Config:      hash_type
- Env Var:     RCLONE_CHUNKER_HASH_TYPE
- Type:        string
- Default:     "md5"
- Examples:
    - "none"
        - Pass any hash supported by wrapped remote for non-chunked files, return nothing otherwise
    - "md5"
        - MD5 for composite files
    - "sha1"
        - SHA1 for composite files
    - "md5all"
        - MD5 for all files
    - "sha1all"
        - SHA1 for all files
    - "md5quick"
        - Copying a file to chunker will request MD5 from the source falling back to SHA1 if unsupported
    - "sha1quick"
        - Similar to "md5quick" but prefers SHA1 over MD5

### Advanced Options

Here are the advanced options specific to chunker (Transparently chunk/split large files).

#### --chunker-name-format

String format of chunk file names.
The two placeholders are: base file name (*) and chunk number (#...).
There must be one and only one asterisk and one or more consecutive hash characters.
If chunk number has less digits than the number of hashes, it is left-padded by zeros.
If there are more digits in the number, they are left as is.
Possible chunk files are ignored if their name does not match given format.

- Config:      name_format
- Env Var:     RCLONE_CHUNKER_NAME_FORMAT
- Type:        string
- Default:     "*.rclone_chunk.###"

#### --chunker-start-from

Minimum valid chunk number. Usually 0 or 1.
By default chunk numbers start from 1.

- Config:      start_from
- Env Var:     RCLONE_CHUNKER_START_FROM
- Type:        int
- Default:     1

#### --chunker-meta-format

Format of the metadata object or "none". By default "simplejson".
Metadata is a small JSON file named after the composite file.

- Config:      meta_format
- Env Var:     RCLONE_CHUNKER_META_FORMAT
- Type:        string
- Default:     "simplejson"
- Examples:
    - "none"
        - Do not use metadata files at all. Requires hash type "none".
    - "simplejson"
        - Simple JSON supports hash sums and chunk validation.
        - It has the following fields: ver, size, nchunks, md5, sha1.

#### --chunker-fail-hard

Choose how chunker should handle files with missing or invalid chunks.

- Config:      fail_hard
- Env Var:     RCLONE_CHUNKER_FAIL_HARD
- Type:        bool
- Default:     false
- Examples:
    - "true"
        - Report errors and abort current command.
    - "false"
        - Warn user, skip incomplete file and proceed.



##  Citrix ShareFile

[Citrix ShareFile](https://sharefile.com) is a secure file sharing and transfer service aimed as business.

The initial setup for Citrix ShareFile involves getting a token from
Citrix ShareFile which you can in your browser.  `rclone config` walks you
through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
XX / Citrix Sharefile
   \ "sharefile"
Storage> sharefile
** See help for sharefile backend at: https://rclone.org/sharefile/ **

ID of the root folder

Leave blank to access "Personal Folders".  You can use one of the
standard values here or any folder ID (long hex number ID).
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / Access the Personal Folders. (Default)
   \ ""
 2 / Access the Favorites folder.
   \ "favorites"
 3 / Access all the shared folders.
   \ "allshared"
 4 / Access all the individual connectors.
   \ "connectors"
 5 / Access the home, favorites, and shared folders as well as the connectors.
   \ "top"
root_folder_id> 
Edit advanced config? (y/n)
y) Yes
n) No
y/n> n
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth?state=XXX
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
type = sharefile
endpoint = https://XXX.sharefile.com
token = {"access_token":"XXX","token_type":"bearer","refresh_token":"XXX","expiry":"2019-09-30T19:41:45.878561877+01:00"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](https://rclone.org/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Citrix ShareFile. This only runs from the moment it opens
your browser to the moment you get back the verification code.  This
is on `http://127.0.0.1:53682/` and this it may require you to unblock
it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List directories in top level of your ShareFile

    rclone lsd remote:

List all the files in your ShareFile

    rclone ls remote:

To copy a local directory to an ShareFile directory called backup

    rclone copy /home/source remote:backup

Paths may be as deep as required, eg `remote:directory/subdirectory`.

### Modified time and hashes ###

ShareFile allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.

ShareFile supports MD5 type hashes, so you can use the `--checksum`
flag.

### Transfers ###

For files above 128MB rclone will use a chunked transfer.  Rclone will
upload up to `--transfers` chunks at the same time (shared among all
the multipart uploads).  Chunks are buffered in memory and are
normally 64MB so increasing `--transfers` will increase memory use.

### Limitations ###

Note that ShareFile is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

ShareFile only supports filenames up to 256 characters in length.

#### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| \\        | 0x5C  | ＼           |
| *         | 0x2A  | ＊           |
| <         | 0x3C  | ＜           |
| >         | 0x3E  | ＞           |
| ?         | 0x3F  | ？           |
| :         | 0x3A  | ：           |
| \|        | 0x7C  | ｜           |
| "         | 0x22  | ＂           |

File names can also not start or end with the following characters.
These only get replaced if they are the first or last character in the
name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |
| .         | 0x2E  | ．           |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.


### Standard Options

Here are the standard options specific to sharefile (Citrix Sharefile).

#### --sharefile-root-folder-id

ID of the root folder

Leave blank to access "Personal Folders".  You can use one of the
standard values here or any folder ID (long hex number ID).

- Config:      root_folder_id
- Env Var:     RCLONE_SHAREFILE_ROOT_FOLDER_ID
- Type:        string
- Default:     ""
- Examples:
    - ""
        - Access the Personal Folders. (Default)
    - "favorites"
        - Access the Favorites folder.
    - "allshared"
        - Access all the shared folders.
    - "connectors"
        - Access all the individual connectors.
    - "top"
        - Access the home, favorites, and shared folders as well as the connectors.

### Advanced Options

Here are the advanced options specific to sharefile (Citrix Sharefile).

#### --sharefile-upload-cutoff

Cutoff for switching to multipart upload.

- Config:      upload_cutoff
- Env Var:     RCLONE_SHAREFILE_UPLOAD_CUTOFF
- Type:        SizeSuffix
- Default:     128M

#### --sharefile-chunk-size

Upload chunk size. Must a power of 2 >= 256k.

Making this larger will improve performance, but note that each chunk
is buffered in memory one per transfer.

Reducing this will reduce memory usage but decrease performance.

- Config:      chunk_size
- Env Var:     RCLONE_SHAREFILE_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     64M

#### --sharefile-endpoint

Endpoint for API calls.

This is usually auto discovered as part of the oauth process, but can
be set manually to something like: https://XXX.sharefile.com


- Config:      endpoint
- Env Var:     RCLONE_SHAREFILE_ENDPOINT
- Type:        string
- Default:     ""

#### --sharefile-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_SHAREFILE_ENCODING
- Type:        MultiEncoder
- Default:     Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,Ctl,LeftSpace,LeftPeriod,RightSpace,RightPeriod,InvalidUtf8,Dot



Crypt
----------------------------------------

The `crypt` remote encrypts and decrypts another remote.

To use it first set up the underlying remote following the config
instructions for that remote.  You can also use a local pathname
instead of a remote which will encrypt and decrypt from that directory
which might be useful for encrypting onto a USB stick for example.

First check your chosen remote is working - we'll call it
`remote:path` in these docs.  Note that anything inside `remote:path`
will be encrypted and anything outside won't.  This means that if you
are using a bucket based remote (eg S3, B2, swift) then you should
probably put the bucket in the remote `s3:bucket`. If you just use
`s3:` then rclone will make encrypted bucket names too (if using file
name encryption) which may or may not be what you want.

Now configure `crypt` using `rclone config`. We will call this one
`secret` to differentiate it from the `remote`.

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n   
name> secret
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Encrypt/Decrypt a remote
   \ "crypt"
[snip]
Storage> crypt
Remote to encrypt/decrypt.
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).
remote> remote:path
How to encrypt the filenames.
Choose a number from below, or type in your own value
 1 / Don't encrypt the file names.  Adds a ".bin" extension only.
   \ "off"
 2 / Encrypt the filenames see the docs for the details.
   \ "standard"
 3 / Very simple filename obfuscation.
   \ "obfuscate"
filename_encryption> 2
Option to either encrypt directory names or leave them intact.
Choose a number from below, or type in your own value
 1 / Encrypt directory names.
   \ "true"
 2 / Don't encrypt directory names, leave them intact.
   \ "false"
filename_encryption> 1
Password or pass phrase for encryption.
y) Yes type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
Password or pass phrase for salt. Optional but recommended.
Should be different to the previous password.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank
y/g/n> g
Password strength in bits.
64 is just about memorable
128 is secure
1024 is the maximum
Bits> 128
Your password is: JAsJvRcgR-_veXNfy_sGmQ
Use this password?
y) Yes
n) No
y/n> y
Remote config
--------------------
[secret]
remote = remote:path
filename_encryption = standard
password = *** ENCRYPTED ***
password2 = *** ENCRYPTED ***
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

**Important** The password is stored in the config file is lightly
obscured so it isn't immediately obvious what it is.  It is in no way
secure unless you use config file encryption.

A long passphrase is recommended, or you can use a random one.

The obscured password is created by using AES-CTR with a static key, with
the salt stored verbatim at the beginning of the obscured password. This
static key is shared by between all versions of rclone.

If you reconfigure rclone with the same passwords/passphrases
elsewhere it will be compatible, but the obscured version will be different
due to the different salt.

Note that rclone does not encrypt

  * file length - this can be calculated within 16 bytes
  * modification time - used for syncing

## Specifying the remote ##

In normal use, make sure the remote has a `:` in. If you specify the
remote without a `:` then rclone will use a local directory of that
name.  So if you use a remote of `/path/to/secret/files` then rclone
will encrypt stuff to that directory.  If you use a remote of `name`
then rclone will put files in a directory called `name` in the current
directory.

If you specify the remote as `remote:path/to/dir` then rclone will
store encrypted files in `path/to/dir` on the remote. If you are using
file name encryption, then when you save files to
`secret:subdir/subfile` this will store them in the unencrypted path
`path/to/dir` but the `subdir/subpath` bit will be encrypted.

Note that unless you want encrypted bucket names (which are difficult
to manage because you won't know what directory they represent in web
interfaces etc), you should probably specify a bucket, eg
`remote:secretbucket` when using bucket based remotes such as S3,
Swift, Hubic, B2, GCS.

## Example ##

To test I made a little directory of files using "standard" file name
encryption.

```
plaintext/
├── file0.txt
├── file1.txt
└── subdir
    ├── file2.txt
    ├── file3.txt
    └── subsubdir
        └── file4.txt
```

Copy these to the remote and list them back

```
$ rclone -q copy plaintext secret:
$ rclone -q ls secret:
        7 file1.txt
        6 file0.txt
        8 subdir/file2.txt
       10 subdir/subsubdir/file4.txt
        9 subdir/file3.txt
```

Now see what that looked like when encrypted

```
$ rclone -q ls remote:path
       55 hagjclgavj2mbiqm6u6cnjjqcg
       54 v05749mltvv1tf4onltun46gls
       57 86vhrsv86mpbtd3a0akjuqslj8/dlj7fkq4kdq72emafg7a7s41uo
       58 86vhrsv86mpbtd3a0akjuqslj8/7uu829995du6o42n32otfhjqp4/b9pausrfansjth5ob3jkdqd4lc
       56 86vhrsv86mpbtd3a0akjuqslj8/8njh1sk437gttmep3p70g81aps
```

Note that this retains the directory structure which means you can do this

```
$ rclone -q ls secret:subdir
        8 file2.txt
        9 file3.txt
       10 subsubdir/file4.txt
```

If don't use file name encryption then the remote will look like this
- note the `.bin` extensions added to prevent the cloud provider
attempting to interpret the data.

```
$ rclone -q ls remote:path
       54 file0.txt.bin
       57 subdir/file3.txt.bin
       56 subdir/file2.txt.bin
       58 subdir/subsubdir/file4.txt.bin
       55 file1.txt.bin
```

### File name encryption modes ###

Here are some of the features of the file name encryption modes

Off

  * doesn't hide file names or directory structure
  * allows for longer file names (~246 characters)
  * can use sub paths and copy single files

Standard

  * file names encrypted
  * file names can't be as long (~143 characters)
  * can use sub paths and copy single files
  * directory structure visible
  * identical files names will have identical uploaded names
  * can use shortcuts to shorten the directory recursion

Obfuscation

This is a simple "rotate" of the filename, with each file having a rot
distance based on the filename. We store the distance at the beginning
of the filename. So a file called "hello" may become "53.jgnnq".

This is not a strong encryption of filenames, but it may stop automated
scanning tools from picking up on filename patterns. As such it's an
intermediate between "off" and "standard". The advantage is that it
allows for longer path segment names.

There is a possibility with some unicode based filenames that the
obfuscation is weak and may map lower case characters to upper case
equivalents.  You can not rely on this for strong protection.

  * file names very lightly obfuscated
  * file names can be longer than standard encryption
  * can use sub paths and copy single files
  * directory structure visible
  * identical files names will have identical uploaded names

Cloud storage systems have various limits on file name length and
total path length which you are more likely to hit using "Standard"
file name encryption.  If you keep your file names to below 156
characters in length then you should be OK on all providers.

There may be an even more secure file name encryption mode in the
future which will address the long file name problem.

### Directory name encryption ###
Crypt offers the option of encrypting dir names or leaving them intact.
There are two options:

True

Encrypts the whole file path including directory names
Example:
`1/12/123.txt` is encrypted to
`p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng/qgm4avr35m5loi1th53ato71v0`

False

Only encrypts file names, skips directory names
Example:
`1/12/123.txt` is encrypted to
`1/12/qgm4avr35m5loi1th53ato71v0`


### Modified time and hashes ###

Crypt stores modification times using the underlying remote so support
depends on that.

Hashes are not stored for crypt.  However the data integrity is
protected by an extremely strong crypto authenticator.

Note that you should use the `rclone cryptcheck` command to check the
integrity of a crypted remote instead of `rclone check` which can't
check the checksums properly.


### Standard Options

Here are the standard options specific to crypt (Encrypt/Decrypt a remote).

#### --crypt-remote

Remote to encrypt/decrypt.
Normally should contain a ':' and a path, eg "myremote:path/to/dir",
"myremote:bucket" or maybe "myremote:" (not recommended).

- Config:      remote
- Env Var:     RCLONE_CRYPT_REMOTE
- Type:        string
- Default:     ""

#### --crypt-filename-encryption

How to encrypt the filenames.

- Config:      filename_encryption
- Env Var:     RCLONE_CRYPT_FILENAME_ENCRYPTION
- Type:        string
- Default:     "standard"
- Examples:
    - "standard"
        - Encrypt the filenames see the docs for the details.
    - "obfuscate"
        - Very simple filename obfuscation.
    - "off"
        - Don't encrypt the file names.  Adds a ".bin" extension only.

#### --crypt-directory-name-encryption

Option to either encrypt directory names or leave them intact.

NB If filename_encryption is "off" then this option will do nothing.

- Config:      directory_name_encryption
- Env Var:     RCLONE_CRYPT_DIRECTORY_NAME_ENCRYPTION
- Type:        bool
- Default:     true
- Examples:
    - "true"
        - Encrypt directory names.
    - "false"
        - Don't encrypt directory names, leave them intact.

#### --crypt-password

Password or pass phrase for encryption.

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      password
- Env Var:     RCLONE_CRYPT_PASSWORD
- Type:        string
- Default:     ""

#### --crypt-password2

Password or pass phrase for salt. Optional but recommended.
Should be different to the previous password.

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      password2
- Env Var:     RCLONE_CRYPT_PASSWORD2
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to crypt (Encrypt/Decrypt a remote).

#### --crypt-show-mapping

For all files listed show how the names encrypt.

If this flag is set then for each file that the remote is asked to
list, it will log (at level INFO) a line stating the decrypted file
name and the encrypted file name.

This is so you can work out which encrypted names are which decrypted
names just in case you need to do something with the encrypted file
names, or for debugging purposes.

- Config:      show_mapping
- Env Var:     RCLONE_CRYPT_SHOW_MAPPING
- Type:        bool
- Default:     false

### Backend commands

Here are the commands specific to the crypt backend.

Run them with

    rclone backend COMMAND remote:

The help below will explain what arguments each command takes.

See [the "rclone backend" command](https://rclone.org/commands/rclone_backend/) for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](https://rclone.org/rc/#backend/command).

#### encode

Encode the given filename(s)

    rclone backend encode remote: [options] [<arguments>+]

This encodes the filenames given as arguments returning a list of
strings of the encoded results.

Usage Example:

    rclone backend encode crypt: file1 [file2...]
    rclone rc backend/command command=encode fs=crypt: file1 [file2...]


#### decode

Decode the given filename(s)

    rclone backend decode remote: [options] [<arguments>+]

This decodes the filenames given as arguments returning a list of
strings of the decoded results. It will return an error if any of the
inputs are invalid.

Usage Example:

    rclone backend decode crypt: encryptedfile1 [encryptedfile2...]
    rclone rc backend/command command=decode fs=crypt: encryptedfile1 [encryptedfile2...]




## Backing up a crypted remote ##

If you wish to backup a crypted remote, it is recommended that you use
`rclone sync` on the encrypted files, and make sure the passwords are
the same in the new encrypted remote.

This will have the following advantages

  * `rclone sync` will check the checksums while copying
  * you can use `rclone check` between the encrypted remotes
  * you don't decrypt and encrypt unnecessarily

For example, let's say you have your original remote at `remote:` with
the encrypted version at `eremote:` with path `remote:crypt`.  You
would then set up the new remote `remote2:` and then the encrypted
version `eremote2:` with path `remote2:crypt` using the same passwords
as `eremote:`.

To sync the two remotes you would do

    rclone sync remote:crypt remote2:crypt

And to check the integrity you would do

    rclone check remote:crypt remote2:crypt

## File formats ##

### File encryption ###

Files are encrypted 1:1 source file to destination object.  The file
has a header and is divided into chunks.

#### Header ####

  * 8 bytes magic string `RCLONE\x00\x00`
  * 24 bytes Nonce (IV)

The initial nonce is generated from the operating systems crypto
strong random number generator.  The nonce is incremented for each
chunk read making sure each nonce is unique for each block written.
The chance of a nonce being re-used is minuscule.  If you wrote an
exabyte of data (10¹⁸ bytes) you would have a probability of
approximately 2×10⁻³² of re-using a nonce.

#### Chunk ####

Each chunk will contain 64kB of data, except for the last one which
may have less data.  The data chunk is in standard NACL secretbox
format. Secretbox uses XSalsa20 and Poly1305 to encrypt and
authenticate messages.

Each chunk contains:

  * 16 Bytes of Poly1305 authenticator
  * 1 - 65536 bytes XSalsa20 encrypted data

64k chunk size was chosen as the best performing chunk size (the
authenticator takes too much time below this and the performance drops
off due to cache effects above this).  Note that these chunks are
buffered in memory so they can't be too big.

This uses a 32 byte (256 bit key) key derived from the user password.

#### Examples ####

1 byte file will encrypt to

  * 32 bytes header
  * 17 bytes data chunk

49 bytes total

1MB (1048576 bytes) file will encrypt to

  * 32 bytes header
  * 16 chunks of 65568 bytes

1049120 bytes total (a 0.05% overhead).  This is the overhead for big
files.

### Name encryption ###

File names are encrypted segment by segment - the path is broken up
into `/` separated strings and these are encrypted individually.

File segments are padded using PKCS#7 to a multiple of 16 bytes
before encryption.

They are then encrypted with EME using AES with 256 bit key. EME
(ECB-Mix-ECB) is a wide-block encryption mode presented in the 2003
paper "A Parallelizable Enciphering Mode" by Halevi and Rogaway.

This makes for deterministic encryption which is what we want - the
same filename must encrypt to the same thing otherwise we can't find
it on the cloud storage system.

This means that

  * filenames with the same name will encrypt the same
  * filenames which start the same won't have a common prefix

This uses a 32 byte key (256 bits) and a 16 byte (128 bits) IV both of
which are derived from the user password.

After encryption they are written out using a modified version of
standard `base32` encoding as described in RFC4648.  The standard
encoding is modified in two ways:

  * it becomes lower case (no-one likes upper case filenames!)
  * we strip the padding character `=`

`base32` is used rather than the more efficient `base64` so rclone can be
used on case insensitive remotes (eg Windows, Amazon Drive).

### Key derivation ###

Rclone uses `scrypt` with parameters `N=16384, r=8, p=1` with an
optional user supplied salt (password2) to derive the 32+32+16 = 80
bytes of key material required.  If the user doesn't supply a salt
then rclone uses an internal one.

`scrypt` makes it impractical to mount a dictionary attack on rclone
encrypted data.  For full protection against this you should always use
a salt.

 Dropbox
---------------------------------

Paths are specified as `remote:path`

Dropbox paths may be as deep as required, eg
`remote:directory/subdirectory`.

The initial setup for dropbox involves getting a token from Dropbox
which you need to do in your browser.  `rclone config` walks you
through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
n) New remote
d) Delete remote
q) Quit config
e/n/d/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Dropbox
   \ "dropbox"
[snip]
Storage> dropbox
Dropbox App Key - leave blank normally.
app_key>
Dropbox App Secret - leave blank normally.
app_secret>
Remote config
Please visit:
https://www.dropbox.com/1/oauth2/authorize?client_id=XXXXXXXXXXXXXXX&response_type=code
Enter the code: XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX_XXXXXXXXXX
--------------------
[remote]
app_key =
app_secret =
token = XXXXXXXXXXXXXXXXXXXXXXXXXXXXX_XXXX_XXXXXXXXXXXXXXXXXXXXXXXXXXXXX
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

You can then use it like this,

List directories in top level of your dropbox

    rclone lsd remote:

List all the files in your dropbox

    rclone ls remote:

To copy a local directory to a dropbox directory called backup

    rclone copy /home/source remote:backup

### Dropbox for business ###

Rclone supports Dropbox for business and Team Folders.

When using Dropbox for business `remote:` and `remote:path/to/file`
will refer to your personal folder.

If you wish to see Team Folders you must use a leading `/` in the
path, so `rclone lsd remote:/` will refer to the root and show you all
Team Folders and your User Folder.

You can then use team folders like this `remote:/TeamFolder` and
`remote:/TeamFolder/path/to/file`.

A leading `/` for a Dropbox personal account will do nothing, but it
will take an extra HTTP transaction so it should be avoided.

### Modified time and Hashes ###

Dropbox supports modified times, but the only way to set a
modification time is to re-upload the file.

This means that if you uploaded your data with an older version of
rclone which didn't support the v2 API and modified times, rclone will
decide to upload all your old data to fix the modification times.  If
you don't want this to happen use `--size-only` or `--checksum` flag
to stop it.

Dropbox supports [its own hash
type](https://www.dropbox.com/developers/reference/content-hash) which
is checked for all transfers.

#### Restricted filename characters

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| /         | 0x2F  | ／           |
| DEL       | 0x7F  | ␡           |
| \         | 0x5C  | ＼           |

File names can also not end with the following characters.
These only get replaced if they are the last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.


### Standard Options

Here are the standard options specific to dropbox (Dropbox).

#### --dropbox-client-id

Dropbox App Client Id
Leave blank normally.

- Config:      client_id
- Env Var:     RCLONE_DROPBOX_CLIENT_ID
- Type:        string
- Default:     ""

#### --dropbox-client-secret

Dropbox App Client Secret
Leave blank normally.

- Config:      client_secret
- Env Var:     RCLONE_DROPBOX_CLIENT_SECRET
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to dropbox (Dropbox).

#### --dropbox-chunk-size

Upload chunk size. (< 150M).

Any files larger than this will be uploaded in chunks of this size.

Note that chunks are buffered in memory (one at a time) so rclone can
deal with retries.  Setting this larger will increase the speed
slightly (at most 10% for 128MB in tests) at the cost of using more
memory.  It can be set smaller if you are tight on memory.

- Config:      chunk_size
- Env Var:     RCLONE_DROPBOX_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     48M

#### --dropbox-impersonate

Impersonate this user when using a business account.

- Config:      impersonate
- Env Var:     RCLONE_DROPBOX_IMPERSONATE
- Type:        string
- Default:     ""

#### --dropbox-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_DROPBOX_ENCODING
- Type:        MultiEncoder
- Default:     Slash,BackSlash,Del,RightSpace,InvalidUtf8,Dot



### Limitations ###

Note that Dropbox is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

There are some file names such as `thumbs.db` which Dropbox can't
store.  There is a full list of them in the ["Ignored Files" section
of this document](https://www.dropbox.com/en/help/145).  Rclone will
issue an error message `File name disallowed - not uploading` if it
attempts to upload one of those file names, but the sync won't fail.

If you have more than 10,000 files in a directory then `rclone purge
dropbox:dir` will return the error `Failed to purge: There are too
many files involved in this operation`.  As a work-around do an
`rclone delete dropbox:dir` followed by an `rclone rmdir dropbox:dir`.

### Get your own Dropbox App ID ###

When you use rclone with Dropbox in its default configuration you are using rclone's App ID. This is shared between all the rclone users.

Here is how to create your own Dropbox App ID for rclone:

1. Log into the [Dropbox App console](https://www.dropbox.com/developers/apps/create) with your Dropbox Account (It need not
to be the same account as the Dropbox you want to access)

2. Choose an API => Usually this should be `Dropbox API`

3. Choose the type of access you want to use => `Full Dropbox` or `App Folder`

4. Name your App. The app name is global, so you can't use `rclone` for example

5. Click the button `Create App`

5. Fill `Redirect URIs` as `http://localhost:53682/`

6. Find the `App key` and `App secret` Use these values in rclone config to add a new remote or edit an existing remote.

 FTP
------------------------------

FTP is the File Transfer Protocol. FTP support is provided using the
[github.com/jlaffaye/ftp](https://godoc.org/github.com/jlaffaye/ftp)
package.

Here is an example of making an FTP configuration.  First run

    rclone config

This will guide you through an interactive setup process. An FTP remote only
needs a host together with and a username and a password. With anonymous FTP
server, you will need to use `anonymous` as username and your email address as
the password.

```
No remotes found - make a new one
n) New remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
n/r/c/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / FTP Connection
   \ "ftp"
[snip]
Storage> ftp
** See help for ftp backend at: https://rclone.org/ftp/ **

FTP host to connect to
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / Connect to ftp.example.com
   \ "ftp.example.com"
host> ftp.example.com
FTP username, leave blank for current username, ncw
Enter a string value. Press Enter for the default ("").
user> 
FTP port, leave blank to use default (21)
Enter a string value. Press Enter for the default ("").
port> 
FTP password
y) Yes type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
Use FTP over TLS (Implicit)
Enter a boolean value (true or false). Press Enter for the default ("false").
tls> 
Remote config
--------------------
[remote]
type = ftp
host = ftp.example.com
pass = *** ENCRYPTED ***
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `remote` and can now be used like this

See all directories in the home directory

    rclone lsd remote:

Make a new directory

    rclone mkdir remote:path/to/directory

List the contents of a directory

    rclone ls remote:path/to/directory

Sync `/home/local/directory` to the remote directory, deleting any
excess files in the directory.

    rclone sync /home/local/directory remote:directory

### Modified time ###

FTP does not support modified times.  Any times you see on the server
will be time of upload.

### Checksums ###

FTP does not support any checksums.

### Usage without a config file ###

An example how to use the ftp remote without a config file:

    rclone lsf :ftp: --ftp-host=speedtest.tele2.net --ftp-user=anonymous --ftp-pass=`rclone obscure dummy`

#### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

File names can also not end with the following characters.
These only get replaced if they are the last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |

Note that not all FTP servers can have all characters in file names, for example:

| FTP Server| Forbidden characters |
| --------- |:--------------------:|
| proftpd   | `*`                  |
| pureftpd  | `\ [ ]`              |

### Implicit TLS ###

FTP supports implicit FTP over TLS servers (FTPS). This has to be enabled
in the config for the remote. The default FTPS port is `990` so the
port will likely have to be explicitly set in the config for the remote.


### Standard Options

Here are the standard options specific to ftp (FTP Connection).

#### --ftp-host

FTP host to connect to

- Config:      host
- Env Var:     RCLONE_FTP_HOST
- Type:        string
- Default:     ""
- Examples:
    - "ftp.example.com"
        - Connect to ftp.example.com

#### --ftp-user

FTP username, leave blank for current username, $USER

- Config:      user
- Env Var:     RCLONE_FTP_USER
- Type:        string
- Default:     ""

#### --ftp-port

FTP port, leave blank to use default (21)

- Config:      port
- Env Var:     RCLONE_FTP_PORT
- Type:        string
- Default:     ""

#### --ftp-pass

FTP password

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      pass
- Env Var:     RCLONE_FTP_PASS
- Type:        string
- Default:     ""

#### --ftp-tls

Use FTP over TLS (Implicit)

- Config:      tls
- Env Var:     RCLONE_FTP_TLS
- Type:        bool
- Default:     false

### Advanced Options

Here are the advanced options specific to ftp (FTP Connection).

#### --ftp-concurrency

Maximum number of FTP simultaneous connections, 0 for unlimited

- Config:      concurrency
- Env Var:     RCLONE_FTP_CONCURRENCY
- Type:        int
- Default:     0

#### --ftp-no-check-certificate

Do not verify the TLS certificate of the server

- Config:      no_check_certificate
- Env Var:     RCLONE_FTP_NO_CHECK_CERTIFICATE
- Type:        bool
- Default:     false

#### --ftp-disable-epsv

Disable using EPSV even if server advertises support

- Config:      disable_epsv
- Env Var:     RCLONE_FTP_DISABLE_EPSV
- Type:        bool
- Default:     false

#### --ftp-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_FTP_ENCODING
- Type:        MultiEncoder
- Default:     Slash,Del,Ctl,RightSpace,Dot



### Limitations ###

Note that since FTP isn't HTTP based the following flags don't work
with it: `--dump-headers`, `--dump-bodies`, `--dump-auth`

Note that `--timeout` isn't supported (but `--contimeout` is).

Note that `--bind` isn't supported.

FTP could support server side move but doesn't yet.

Note that the ftp backend does not support the `ftp_proxy` environment
variable yet.

Note that while implicit FTP over TLS is supported,
explicit FTP over TLS is not.

 Google Cloud Storage
-------------------------------------------------

Paths are specified as `remote:bucket` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, eg `remote:bucket/path/to/dir`.

The initial setup for google cloud storage involves getting a token from Google Cloud Storage
which you need to do in your browser.  `rclone config` walks you
through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
n) New remote
d) Delete remote
q) Quit config
e/n/d/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Google Cloud Storage (this is not Google Drive)
   \ "google cloud storage"
[snip]
Storage> google cloud storage
Google Application Client Id - leave blank normally.
client_id>
Google Application Client Secret - leave blank normally.
client_secret>
Project number optional - needed only for list/create/delete buckets - see your developer console.
project_number> 12345678
Service Account Credentials JSON file path - needed only if you want use SA instead of interactive login.
service_account_file>
Access Control List for new objects.
Choose a number from below, or type in your own value
 1 / Object owner gets OWNER access, and all Authenticated Users get READER access.
   \ "authenticatedRead"
 2 / Object owner gets OWNER access, and project team owners get OWNER access.
   \ "bucketOwnerFullControl"
 3 / Object owner gets OWNER access, and project team owners get READER access.
   \ "bucketOwnerRead"
 4 / Object owner gets OWNER access [default if left blank].
   \ "private"
 5 / Object owner gets OWNER access, and project team members get access according to their roles.
   \ "projectPrivate"
 6 / Object owner gets OWNER access, and all Users get READER access.
   \ "publicRead"
object_acl> 4
Access Control List for new buckets.
Choose a number from below, or type in your own value
 1 / Project team owners get OWNER access, and all Authenticated Users get READER access.
   \ "authenticatedRead"
 2 / Project team owners get OWNER access [default if left blank].
   \ "private"
 3 / Project team members get access according to their roles.
   \ "projectPrivate"
 4 / Project team owners get OWNER access, and all Users get READER access.
   \ "publicRead"
 5 / Project team owners get OWNER access, and all Users get WRITER access.
   \ "publicReadWrite"
bucket_acl> 2
Location for the newly created buckets.
Choose a number from below, or type in your own value
 1 / Empty for default location (US).
   \ ""
 2 / Multi-regional location for Asia.
   \ "asia"
 3 / Multi-regional location for Europe.
   \ "eu"
 4 / Multi-regional location for United States.
   \ "us"
 5 / Taiwan.
   \ "asia-east1"
 6 / Tokyo.
   \ "asia-northeast1"
 7 / Singapore.
   \ "asia-southeast1"
 8 / Sydney.
   \ "australia-southeast1"
 9 / Belgium.
   \ "europe-west1"
10 / London.
   \ "europe-west2"
11 / Iowa.
   \ "us-central1"
12 / South Carolina.
   \ "us-east1"
13 / Northern Virginia.
   \ "us-east4"
14 / Oregon.
   \ "us-west1"
location> 12
The storage class to use when storing objects in Google Cloud Storage.
Choose a number from below, or type in your own value
 1 / Default
   \ ""
 2 / Multi-regional storage class
   \ "MULTI_REGIONAL"
 3 / Regional storage class
   \ "REGIONAL"
 4 / Nearline storage class
   \ "NEARLINE"
 5 / Coldline storage class
   \ "COLDLINE"
 6 / Durable reduced availability storage class
   \ "DURABLE_REDUCED_AVAILABILITY"
storage_class> 5
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine or Y didn't work
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
type = google cloud storage
client_id =
client_secret =
token = {"AccessToken":"xxxx.xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","RefreshToken":"x/xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx_xxxxxxxxx","Expiry":"2014-07-17T20:49:14.929208288+01:00","Extra":null}
project_number = 12345678
object_acl = private
bucket_acl = private
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Note that rclone runs a webserver on your local machine to collect the
token as returned from Google if you use auto config mode. This only
runs from the moment it opens your browser to the moment you get back
the verification code.  This is on `http://127.0.0.1:53682/` and this
it may require you to unblock it temporarily if you are running a host
firewall, or use manual mode.

This remote is called `remote` and can now be used like this

See all the buckets in your project

    rclone lsd remote:

Make a new bucket

    rclone mkdir remote:bucket

List the contents of a bucket

    rclone ls remote:bucket

Sync `/home/local/directory` to the remote bucket, deleting any excess
files in the bucket.

    rclone sync /home/local/directory remote:bucket

### Service Account support ###

You can set up rclone with Google Cloud Storage in an unattended mode,
i.e. not tied to a specific end-user Google account. This is useful
when you want to synchronise files onto machines that don't have
actively logged-in users, for example build machines.

To get credentials for Google Cloud Platform
[IAM Service Accounts](https://cloud.google.com/iam/docs/service-accounts),
please head to the
[Service Account](https://console.cloud.google.com/permissions/serviceaccounts)
section of the Google Developer Console. Service Accounts behave just
like normal `User` permissions in
[Google Cloud Storage ACLs](https://cloud.google.com/storage/docs/access-control),
so you can limit their access (e.g. make them read only). After
creating an account, a JSON file containing the Service Account's
credentials will be downloaded onto your machines. These credentials
are what rclone will use for authentication.

To use a Service Account instead of OAuth2 token flow, enter the path
to your Service Account credentials at the `service_account_file`
prompt and rclone won't use the browser based authentication
flow. If you'd rather stuff the contents of the credentials file into
the rclone config file, you can set `service_account_credentials` with
the actual contents of the file instead, or set the equivalent
environment variable.

### Application Default Credentials ###

If no other source of credentials is provided, rclone will fall back
to
[Application Default Credentials](https://cloud.google.com/video-intelligence/docs/common/auth#authenticating_with_application_default_credentials)
this is useful both when you already have configured authentication
for your developer account, or in production when running on a google
compute host. Note that if running in docker, you may need to run
additional commands on your google compute machine -
[see this page](https://cloud.google.com/container-registry/docs/advanced-authentication#gcloud_as_a_docker_credential_helper).

Note that in the case application default credentials are used, there
is no need to explicitly configure a project number.

### --fast-list ###

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](https://rclone.org/docs/#fast-list) for more details.

### Custom upload headers ###

You can set custom upload headers with the `--header-upload`
flag. Google Cloud Storage supports the headers as described in the
[working with metadata documentation](https://cloud.google.com/storage/docs/gsutil/addlhelp/WorkingWithObjectMetadata)

- Cache-Control
- Content-Disposition
- Content-Encoding
- Content-Language
- Content-Type
- X-Goog-Meta-

Eg `--header-upload "Content-Type text/potato"`

Note that the last of these is for setting custom metadata in the form
`--header-upload "x-goog-meta-key: value"`

### Modified time ###

Google google cloud storage stores md5sums natively and rclone stores
modification times as metadata on the object, under the "mtime" key in
RFC3339 format accurate to 1ns.

#### Restricted filename characters

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| LF        | 0x0A  | ␊           |
| CR        | 0x0D  | ␍           |
| /         | 0x2F  | ／          |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.


### Standard Options

Here are the standard options specific to google cloud storage (Google Cloud Storage (this is not Google Drive)).

#### --gcs-client-id

Google Application Client Id
Leave blank normally.

- Config:      client_id
- Env Var:     RCLONE_GCS_CLIENT_ID
- Type:        string
- Default:     ""

#### --gcs-client-secret

Google Application Client Secret
Leave blank normally.

- Config:      client_secret
- Env Var:     RCLONE_GCS_CLIENT_SECRET
- Type:        string
- Default:     ""

#### --gcs-project-number

Project number.
Optional - needed only for list/create/delete buckets - see your developer console.

- Config:      project_number
- Env Var:     RCLONE_GCS_PROJECT_NUMBER
- Type:        string
- Default:     ""

#### --gcs-service-account-file

Service Account Credentials JSON file path
Leave blank normally.
Needed only if you want use SA instead of interactive login.

- Config:      service_account_file
- Env Var:     RCLONE_GCS_SERVICE_ACCOUNT_FILE
- Type:        string
- Default:     ""

#### --gcs-service-account-credentials

Service Account Credentials JSON blob
Leave blank normally.
Needed only if you want use SA instead of interactive login.

- Config:      service_account_credentials
- Env Var:     RCLONE_GCS_SERVICE_ACCOUNT_CREDENTIALS
- Type:        string
- Default:     ""

#### --gcs-object-acl

Access Control List for new objects.

- Config:      object_acl
- Env Var:     RCLONE_GCS_OBJECT_ACL
- Type:        string
- Default:     ""
- Examples:
    - "authenticatedRead"
        - Object owner gets OWNER access, and all Authenticated Users get READER access.
    - "bucketOwnerFullControl"
        - Object owner gets OWNER access, and project team owners get OWNER access.
    - "bucketOwnerRead"
        - Object owner gets OWNER access, and project team owners get READER access.
    - "private"
        - Object owner gets OWNER access [default if left blank].
    - "projectPrivate"
        - Object owner gets OWNER access, and project team members get access according to their roles.
    - "publicRead"
        - Object owner gets OWNER access, and all Users get READER access.

#### --gcs-bucket-acl

Access Control List for new buckets.

- Config:      bucket_acl
- Env Var:     RCLONE_GCS_BUCKET_ACL
- Type:        string
- Default:     ""
- Examples:
    - "authenticatedRead"
        - Project team owners get OWNER access, and all Authenticated Users get READER access.
    - "private"
        - Project team owners get OWNER access [default if left blank].
    - "projectPrivate"
        - Project team members get access according to their roles.
    - "publicRead"
        - Project team owners get OWNER access, and all Users get READER access.
    - "publicReadWrite"
        - Project team owners get OWNER access, and all Users get WRITER access.

#### --gcs-bucket-policy-only

Access checks should use bucket-level IAM policies.

If you want to upload objects to a bucket with Bucket Policy Only set
then you will need to set this.

When it is set, rclone:

- ignores ACLs set on buckets
- ignores ACLs set on objects
- creates buckets with Bucket Policy Only set

Docs: https://cloud.google.com/storage/docs/bucket-policy-only


- Config:      bucket_policy_only
- Env Var:     RCLONE_GCS_BUCKET_POLICY_ONLY
- Type:        bool
- Default:     false

#### --gcs-location

Location for the newly created buckets.

- Config:      location
- Env Var:     RCLONE_GCS_LOCATION
- Type:        string
- Default:     ""
- Examples:
    - ""
        - Empty for default location (US).
    - "asia"
        - Multi-regional location for Asia.
    - "eu"
        - Multi-regional location for Europe.
    - "us"
        - Multi-regional location for United States.
    - "asia-east1"
        - Taiwan.
    - "asia-east2"
        - Hong Kong.
    - "asia-northeast1"
        - Tokyo.
    - "asia-south1"
        - Mumbai.
    - "asia-southeast1"
        - Singapore.
    - "australia-southeast1"
        - Sydney.
    - "europe-north1"
        - Finland.
    - "europe-west1"
        - Belgium.
    - "europe-west2"
        - London.
    - "europe-west3"
        - Frankfurt.
    - "europe-west4"
        - Netherlands.
    - "us-central1"
        - Iowa.
    - "us-east1"
        - South Carolina.
    - "us-east4"
        - Northern Virginia.
    - "us-west1"
        - Oregon.
    - "us-west2"
        - California.

#### --gcs-storage-class

The storage class to use when storing objects in Google Cloud Storage.

- Config:      storage_class
- Env Var:     RCLONE_GCS_STORAGE_CLASS
- Type:        string
- Default:     ""
- Examples:
    - ""
        - Default
    - "MULTI_REGIONAL"
        - Multi-regional storage class
    - "REGIONAL"
        - Regional storage class
    - "NEARLINE"
        - Nearline storage class
    - "COLDLINE"
        - Coldline storage class
    - "ARCHIVE"
        - Archive storage class
    - "DURABLE_REDUCED_AVAILABILITY"
        - Durable reduced availability storage class

### Advanced Options

Here are the advanced options specific to google cloud storage (Google Cloud Storage (this is not Google Drive)).

#### --gcs-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_GCS_ENCODING
- Type:        MultiEncoder
- Default:     Slash,CrLf,InvalidUtf8,Dot



 Google Drive
-----------------------------------------

Paths are specified as `drive:path`

Drive paths may be as deep as required, eg `drive:directory/subdirectory`.

The initial setup for drive involves getting a token from Google drive
which you need to do in your browser.  `rclone config` walks you
through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
n/r/c/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Google Drive
   \ "drive"
[snip]
Storage> drive
Google Application Client Id - leave blank normally.
client_id>
Google Application Client Secret - leave blank normally.
client_secret>
Scope that rclone should use when requesting access from drive.
Choose a number from below, or type in your own value
 1 / Full access all files, excluding Application Data Folder.
   \ "drive"
 2 / Read-only access to file metadata and file contents.
   \ "drive.readonly"
   / Access to files created by rclone only.
 3 | These are visible in the drive website.
   | File authorization is revoked when the user deauthorizes the app.
   \ "drive.file"
   / Allows read and write access to the Application Data folder.
 4 | This is not visible in the drive website.
   \ "drive.appfolder"
   / Allows read-only access to file metadata but
 5 | does not allow any access to read or download file content.
   \ "drive.metadata.readonly"
scope> 1
ID of the root folder - leave blank normally.  Fill in to access "Computers" folders. (see docs).
root_folder_id> 
Service Account Credentials JSON file path - needed only if you want use SA instead of interactive login.
service_account_file>
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine or Y didn't work
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
Configure this as a team drive?
y) Yes
n) No
y/n> n
--------------------
[remote]
client_id = 
client_secret = 
scope = drive
root_folder_id = 
service_account_file =
token = {"access_token":"XXX","token_type":"Bearer","refresh_token":"XXX","expiry":"2014-03-16T13:57:58.955387075Z"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Note that rclone runs a webserver on your local machine to collect the
token as returned from Google if you use auto config mode. This only
runs from the moment it opens your browser to the moment you get back
the verification code.  This is on `http://127.0.0.1:53682/` and this
it may require you to unblock it temporarily if you are running a host
firewall, or use manual mode.

You can then use it like this,

List directories in top level of your drive

    rclone lsd remote:

List all the files in your drive

    rclone ls remote:

To copy a local directory to a drive directory called backup

    rclone copy /home/source remote:backup

### Scopes ###

Rclone allows you to select which scope you would like for rclone to
use.  This changes what type of token is granted to rclone.  [The
scopes are defined
here](https://developers.google.com/drive/v3/web/about-auth).

The scope are

#### drive ####

This is the default scope and allows full access to all files, except
for the Application Data Folder (see below).

Choose this one if you aren't sure.

#### drive.readonly ####

This allows read only access to all files.  Files may be listed and
downloaded but not uploaded, renamed or deleted.

#### drive.file ####

With this scope rclone can read/view/modify only those files and
folders it creates.

So if you uploaded files to drive via the web interface (or any other
means) they will not be visible to rclone.

This can be useful if you are using rclone to backup data and you want
to be sure confidential data on your drive is not visible to rclone.

Files created with this scope are visible in the web interface.

#### drive.appfolder ####

This gives rclone its own private area to store files.  Rclone will
not be able to see any other files on your drive and you won't be able
to see rclone's files from the web interface either.

#### drive.metadata.readonly ####

This allows read only access to file names only.  It does not allow
rclone to download or upload data, or rename or delete files or
directories.

### Root folder ID ###

You can set the `root_folder_id` for rclone.  This is the directory
(identified by its `Folder ID`) that rclone considers to be the root
of your drive.

Normally you will leave this blank and rclone will determine the
correct root to use itself.

However you can set this to restrict rclone to a specific folder
hierarchy or to access data within the "Computers" tab on the drive
web interface (where files from Google's Backup and Sync desktop
program go).

In order to do this you will have to find the `Folder ID` of the
directory you wish rclone to display.  This will be the last segment
of the URL when you open the relevant folder in the drive web
interface.

So if the folder you want rclone to use has a URL which looks like
`https://drive.google.com/drive/folders/1XyfxxxxxxxxxxxxxxxxxxxxxxxxxKHCh`
in the browser, then you use `1XyfxxxxxxxxxxxxxxxxxxxxxxxxxKHCh` as
the `root_folder_id` in the config.

**NB** folders under the "Computers" tab seem to be read only (drive
gives a 500 error) when using rclone.

There doesn't appear to be an API to discover the folder IDs of the
"Computers" tab - please contact us if you know otherwise!

Note also that rclone can't access any data under the "Backups" tab on
the google drive web interface yet.

### Service Account support ###

You can set up rclone with Google Drive in an unattended mode,
i.e. not tied to a specific end-user Google account. This is useful
when you want to synchronise files onto machines that don't have
actively logged-in users, for example build machines.

To use a Service Account instead of OAuth2 token flow, enter the path
to your Service Account credentials at the `service_account_file`
prompt during `rclone config` and rclone won't use the browser based
authentication flow. If you'd rather stuff the contents of the
credentials file into the rclone config file, you can set
`service_account_credentials` with the actual contents of the file
instead, or set the equivalent environment variable.

#### Use case - Google Apps/G-suite account and individual Drive ####

Let's say that you are the administrator of a Google Apps (old) or
G-suite account.
The goal is to store data on an individual's Drive account, who IS
a member of the domain.
We'll call the domain **example.com**, and the user
**foo@example.com**.

There's a few steps we need to go through to accomplish this:

##### 1. Create a service account for example.com #####
  - To create a service account and obtain its credentials, go to the
[Google Developer Console](https://console.developers.google.com).
  - You must have a project - create one if you don't.
  - Then go to "IAM & admin" -> "Service Accounts".
  - Use the "Create Credentials" button. Fill in "Service account name"
with something that identifies your client. "Role" can be empty.
  - Tick "Furnish a new private key" - select "Key type JSON".
  - Tick "Enable G Suite Domain-wide Delegation". This option makes
"impersonation" possible, as documented here:
[Delegating domain-wide authority to the service account](https://developers.google.com/identity/protocols/OAuth2ServiceAccount#delegatingauthority)
  - These credentials are what rclone will use for authentication.
If you ever need to remove access, press the "Delete service
account key" button.

##### 2. Allowing API access to example.com Google Drive #####
  - Go to example.com's admin console
  - Go into "Security" (or use the search bar)
  - Select "Show more" and then "Advanced settings"
  - Select "Manage API client access" in the "Authentication" section
  - In the "Client Name" field enter the service account's
"Client ID" - this can be found in the Developer Console under
"IAM & Admin" -> "Service Accounts", then "View Client ID" for
the newly created service account.
It is a ~21 character numerical string.
  - In the next field, "One or More API Scopes", enter
`https://www.googleapis.com/auth/drive`
to grant access to Google Drive specifically.

##### 3. Configure rclone, assuming a new install #####

```
rclone config

n/s/q> n         # New
name>gdrive      # Gdrive is an example name
Storage>         # Select the number shown for Google Drive
client_id>       # Can be left blank
client_secret>   # Can be left blank
scope>           # Select your scope, 1 for example
root_folder_id>  # Can be left blank
service_account_file> /home/foo/myJSONfile.json # This is where the JSON file goes!
y/n>             # Auto config, y

```

##### 4. Verify that it's working #####
  - `rclone -v --drive-impersonate foo@example.com lsf gdrive:backup`
  - The arguments do:
    - `-v` - verbose logging
    - `--drive-impersonate foo@example.com` - this is what does
the magic, pretending to be user foo.
    - `lsf` - list files in a parsing friendly way
    - `gdrive:backup` - use the remote called gdrive, work in
the folder named backup.

### Team drives ###

If you want to configure the remote to point to a Google Team Drive
then answer `y` to the question `Configure this as a team drive?`.

This will fetch the list of Team Drives from google and allow you to
configure which one you want to use.  You can also type in a team
drive ID if you prefer.

For example:

```
Configure this as a team drive?
y) Yes
n) No
y/n> y
Fetching team drive list...
Choose a number from below, or type in your own value
 1 / Rclone Test
   \ "xxxxxxxxxxxxxxxxxxxx"
 2 / Rclone Test 2
   \ "yyyyyyyyyyyyyyyyyyyy"
 3 / Rclone Test 3
   \ "zzzzzzzzzzzzzzzzzzzz"
Enter a Team Drive ID> 1
--------------------
[remote]
client_id =
client_secret =
token = {"AccessToken":"xxxx.x.xxxxx_xxxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","RefreshToken":"1/xxxxxxxxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxx","Expiry":"2014-03-16T13:57:58.955387075Z","Extra":null}
team_drive = xxxxxxxxxxxxxxxxxxxx
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### --fast-list ###

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](https://rclone.org/docs/#fast-list) for more details.

It does this by combining multiple `list` calls into a single API request.

This works by combining many `'%s' in parents` filters into one expression.
To list the contents of directories a, b and c, the following requests will be send by the regular `List` function:
```
trashed=false and 'a' in parents
trashed=false and 'b' in parents
trashed=false and 'c' in parents
```
These can now be combined into a single request:
```
trashed=false and ('a' in parents or 'b' in parents or 'c' in parents)
```

The implementation of `ListR` will put up to 50 `parents` filters into one request.
It will  use the `--checkers` value to specify the number of requests to run in parallel.

In tests, these batch requests were up to 20x faster than the regular method.
Running the following command against different sized folders gives:
```
rclone lsjson -vv -R --checkers=6 gdrive:folder
```

small folder (220 directories, 700 files):

- without `--fast-list`: 38s
- with `--fast-list`: 10s

large folder (10600 directories, 39000 files):

- without `--fast-list`: 22:05 min
- with `--fast-list`: 58s

### Modified time ###

Google drive stores modification times accurate to 1 ms.

#### Restricted filename characters

Only Invalid UTF-8 bytes will be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.

In contrast to other backends, `/` can also be used in names and `.`
or `..` are valid names.

### Revisions ###

Google drive stores revisions of files.  When you upload a change to
an existing file to google drive using rclone it will create a new
revision of that file.

Revisions follow the standard google policy which at time of writing
was

  * They are deleted after 30 days or 100 revisions (whatever comes first).
  * They do not count towards a user storage quota.

### Deleting files ###

By default rclone will send all files to the trash when deleting
files.  If deleting them permanently is required then use the
`--drive-use-trash=false` flag, or set the equivalent environment
variable.

### Shortcuts ###

In March 2020 Google introduced a new feature in Google Drive called
[drive shortcuts](https://support.google.com/drive/answer/9700156)
([API](https://developers.google.com/drive/api/v3/shortcuts)). These
will (by September 2020) [replace the ability for files or folders to
be in multiple folders at once](https://cloud.google.com/blog/products/g-suite/simplifying-google-drives-folder-structure-and-sharing-models).

Shortcuts are files that link to other files on Google Drive somewhat
like a symlink in unix, except they point to the underlying file data
(eg the inode in unix terms) so they don't break if the source is
renamed or moved about.

Be default rclone treats these as follows.

For shortcuts pointing to files:

- When listing a file shortcut appears as the destination file.
- When downloading the contents of the destination file is downloaded.
- When updating shortcut file with a non shortcut file, the shortcut is removed then a new file is uploaded in place of the shortcut.
- When server side moving (renaming) the shortcut is renamed, not the destination file.
- When server side copying the shortcut is copied, not the contents of the shortcut.
- When deleting the shortcut is deleted not the linked file.
- When setting the modification time, the modification time of the linked file will be set.

For shortcuts pointing to folders:

- When listing the shortcut appears as a folder and that folder will contain the contents of the linked folder appear (including any sub folders)
- When downloading the contents of the linked folder and sub contents are downloaded
- When uploading to a shortcut folder the file will be placed in the linked folder
- When server side moving (renaming) the shortcut is renamed, not the destination folder
- When server side copying the contents of the linked folder is copied, not the shortcut.
- When deleting with `rclone rmdir` or `rclone purge` the shortcut is deleted not the linked folder.
- **NB** When deleting with `rclone remove` or `rclone mount` the contents of the linked folder will be deleted.

The [rclone backend](https://rclone.org/commands/rclone_backend/) command can be used to create shortcuts.  

Shortcuts can be completely ignored with the `--drive-skip-shortcuts` flag
or the corresponding `skip_shortcuts` configuration setting.

### Emptying trash ###

If you wish to empty your trash you can use the `rclone cleanup remote:`
command which will permanently delete all your trashed files. This command
does not take any path arguments.

Note that Google Drive takes some time (minutes to days) to empty the
trash even though the command returns within a few seconds.  No output
is echoed, so there will be no confirmation even using -v or -vv.

### Quota information ###

To view your current quota you can use the `rclone about remote:`
command which will display your usage limit (quota), the usage in Google
Drive, the size of all files in the Trash and the space used by other
Google services such as Gmail. This command does not take any path
arguments.

#### Import/Export of google documents ####

Google documents can be exported from and uploaded to Google Drive.

When rclone downloads a Google doc it chooses a format to download
depending upon the `--drive-export-formats` setting.
By default the export formats are `docx,xlsx,pptx,svg` which are a
sensible default for an editable document.

When choosing a format, rclone runs down the list provided in order
and chooses the first file format the doc can be exported as from the
list. If the file can't be exported to a format on the formats list,
then rclone will choose a format from the default list.

If you prefer an archive copy then you might use `--drive-export-formats
pdf`, or if you prefer openoffice/libreoffice formats you might use
`--drive-export-formats ods,odt,odp`.

Note that rclone adds the extension to the google doc, so if it is
called `My Spreadsheet` on google docs, it will be exported as `My
Spreadsheet.xlsx` or `My Spreadsheet.pdf` etc.

When importing files into Google Drive, rclone will convert all
files with an extension in `--drive-import-formats` to their
associated document type.
rclone will not convert any files by default, since the conversion
is lossy process.

The conversion must result in a file with the same extension when
the `--drive-export-formats` rules are applied to the uploaded document.

Here are some examples for allowed and prohibited conversions.

| export-formats | import-formats | Upload Ext | Document Ext | Allowed |
| -------------- | -------------- | ---------- | ------------ | ------- |
| odt | odt | odt | odt | Yes |
| odt | docx,odt | odt | odt | Yes |
|  | docx | docx | docx | Yes |
|  | odt | odt | docx | No |
| odt,docx | docx,odt | docx | odt | No |
| docx,odt | docx,odt | docx | docx | Yes |
| docx,odt | docx,odt | odt | docx | No |

This limitation can be disabled by specifying `--drive-allow-import-name-change`.
When using this flag, rclone can convert multiple files types resulting
in the same document type at once, eg with `--drive-import-formats docx,odt,txt`,
all files having these extension would result in a document represented as a docx file.
This brings the additional risk of overwriting a document, if multiple files
have the same stem. Many rclone operations will not handle this name change
in any way. They assume an equal name when copying files and might copy the
file again or delete them when the name changes. 

Here are the possible export extensions with their corresponding mime types.
Most of these can also be used for importing, but there more that are not
listed here. Some of these additional ones might only be available when
the operating system provides the correct MIME type entries.

This list can be changed by Google Drive at any time and might not
represent the currently available conversions.

| Extension | Mime Type | Description |
| --------- |-----------| ------------|
| csv  | text/csv | Standard CSV format for Spreadsheets |
| docx | application/vnd.openxmlformats-officedocument.wordprocessingml.document | Microsoft Office Document |
| epub | application/epub+zip | E-book format |
| html | text/html | An HTML Document |
| jpg  | image/jpeg | A JPEG Image File |
| json | application/vnd.google-apps.script+json | JSON Text Format |
| odp  | application/vnd.oasis.opendocument.presentation | Openoffice Presentation |
| ods  | application/vnd.oasis.opendocument.spreadsheet | Openoffice Spreadsheet |
| ods  | application/x-vnd.oasis.opendocument.spreadsheet | Openoffice Spreadsheet |
| odt  | application/vnd.oasis.opendocument.text | Openoffice Document |
| pdf  | application/pdf | Adobe PDF Format |
| png  | image/png | PNG Image Format|
| pptx | application/vnd.openxmlformats-officedocument.presentationml.presentation | Microsoft Office Powerpoint |
| rtf  | application/rtf | Rich Text Format |
| svg  | image/svg+xml | Scalable Vector Graphics Format |
| tsv  | text/tab-separated-values | Standard TSV format for spreadsheets |
| txt  | text/plain | Plain Text |
| xlsx | application/vnd.openxmlformats-officedocument.spreadsheetml.sheet | Microsoft Office Spreadsheet |
| zip  | application/zip | A ZIP file of HTML, Images CSS |

Google documents can also be exported as link files. These files will
open a browser window for the Google Docs website of that document
when opened. The link file extension has to be specified as a
`--drive-export-formats` parameter. They will match all available
Google Documents.

| Extension | Description | OS Support |
| --------- | ----------- | ---------- |
| desktop | freedesktop.org specified desktop entry | Linux |
| link.html | An HTML Document with a redirect | All |
| url | INI style link file | macOS, Windows |
| webloc | macOS specific XML format | macOS |


### Standard Options

Here are the standard options specific to drive (Google Drive).

#### --drive-client-id

Google Application Client Id
Setting your own is recommended.
See https://rclone.org/drive/#making-your-own-client-id for how to create your own.
If you leave this blank, it will use an internal key which is low performance.

- Config:      client_id
- Env Var:     RCLONE_DRIVE_CLIENT_ID
- Type:        string
- Default:     ""

#### --drive-client-secret

Google Application Client Secret
Setting your own is recommended.

- Config:      client_secret
- Env Var:     RCLONE_DRIVE_CLIENT_SECRET
- Type:        string
- Default:     ""

#### --drive-scope

Scope that rclone should use when requesting access from drive.

- Config:      scope
- Env Var:     RCLONE_DRIVE_SCOPE
- Type:        string
- Default:     ""
- Examples:
    - "drive"
        - Full access all files, excluding Application Data Folder.
    - "drive.readonly"
        - Read-only access to file metadata and file contents.
    - "drive.file"
        - Access to files created by rclone only.
        - These are visible in the drive website.
        - File authorization is revoked when the user deauthorizes the app.
    - "drive.appfolder"
        - Allows read and write access to the Application Data folder.
        - This is not visible in the drive website.
    - "drive.metadata.readonly"
        - Allows read-only access to file metadata but
        - does not allow any access to read or download file content.

#### --drive-root-folder-id

ID of the root folder
Leave blank normally.

Fill in to access "Computers" folders (see docs), or for rclone to use
a non root folder as its starting point.

Note that if this is blank, the first time rclone runs it will fill it
in with the ID of the root folder.


- Config:      root_folder_id
- Env Var:     RCLONE_DRIVE_ROOT_FOLDER_ID
- Type:        string
- Default:     ""

#### --drive-service-account-file

Service Account Credentials JSON file path 
Leave blank normally.
Needed only if you want use SA instead of interactive login.

- Config:      service_account_file
- Env Var:     RCLONE_DRIVE_SERVICE_ACCOUNT_FILE
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to drive (Google Drive).

#### --drive-service-account-credentials

Service Account Credentials JSON blob
Leave blank normally.
Needed only if you want use SA instead of interactive login.

- Config:      service_account_credentials
- Env Var:     RCLONE_DRIVE_SERVICE_ACCOUNT_CREDENTIALS
- Type:        string
- Default:     ""

#### --drive-team-drive

ID of the Team Drive

- Config:      team_drive
- Env Var:     RCLONE_DRIVE_TEAM_DRIVE
- Type:        string
- Default:     ""

#### --drive-auth-owner-only

Only consider files owned by the authenticated user.

- Config:      auth_owner_only
- Env Var:     RCLONE_DRIVE_AUTH_OWNER_ONLY
- Type:        bool
- Default:     false

#### --drive-use-trash

Send files to the trash instead of deleting permanently.
Defaults to true, namely sending files to the trash.
Use `--drive-use-trash=false` to delete files permanently instead.

- Config:      use_trash
- Env Var:     RCLONE_DRIVE_USE_TRASH
- Type:        bool
- Default:     true

#### --drive-skip-gdocs

Skip google documents in all listings.
If given, gdocs practically become invisible to rclone.

- Config:      skip_gdocs
- Env Var:     RCLONE_DRIVE_SKIP_GDOCS
- Type:        bool
- Default:     false

#### --drive-skip-checksum-gphotos

Skip MD5 checksum on Google photos and videos only.

Use this if you get checksum errors when transferring Google photos or
videos.

Setting this flag will cause Google photos and videos to return a
blank MD5 checksum.

Google photos are identified by being in the "photos" space.

Corrupted checksums are caused by Google modifying the image/video but
not updating the checksum.

- Config:      skip_checksum_gphotos
- Env Var:     RCLONE_DRIVE_SKIP_CHECKSUM_GPHOTOS
- Type:        bool
- Default:     false

#### --drive-shared-with-me

Only show files that are shared with me.

Instructs rclone to operate on your "Shared with me" folder (where
Google Drive lets you access the files and folders others have shared
with you).

This works both with the "list" (lsd, lsl, etc) and the "copy"
commands (copy, sync, etc), and with all other commands too.

- Config:      shared_with_me
- Env Var:     RCLONE_DRIVE_SHARED_WITH_ME
- Type:        bool
- Default:     false

#### --drive-trashed-only

Only show files that are in the trash.
This will show trashed files in their original directory structure.

- Config:      trashed_only
- Env Var:     RCLONE_DRIVE_TRASHED_ONLY
- Type:        bool
- Default:     false

#### --drive-formats

Deprecated: see export_formats

- Config:      formats
- Env Var:     RCLONE_DRIVE_FORMATS
- Type:        string
- Default:     ""

#### --drive-export-formats

Comma separated list of preferred formats for downloading Google docs.

- Config:      export_formats
- Env Var:     RCLONE_DRIVE_EXPORT_FORMATS
- Type:        string
- Default:     "docx,xlsx,pptx,svg"

#### --drive-import-formats

Comma separated list of preferred formats for uploading Google docs.

- Config:      import_formats
- Env Var:     RCLONE_DRIVE_IMPORT_FORMATS
- Type:        string
- Default:     ""

#### --drive-allow-import-name-change

Allow the filetype to change when uploading Google docs (e.g. file.doc to file.docx). This will confuse sync and reupload every time.

- Config:      allow_import_name_change
- Env Var:     RCLONE_DRIVE_ALLOW_IMPORT_NAME_CHANGE
- Type:        bool
- Default:     false

#### --drive-use-created-date

Use file created date instead of modified date.,

Useful when downloading data and you want the creation date used in
place of the last modified date.

**WARNING**: This flag may have some unexpected consequences.

When uploading to your drive all files will be overwritten unless they
haven't been modified since their creation. And the inverse will occur
while downloading.  This side effect can be avoided by using the
"--checksum" flag.

This feature was implemented to retain photos capture date as recorded
by google photos. You will first need to check the "Create a Google
Photos folder" option in your google drive settings. You can then copy
or move the photos locally and use the date the image was taken
(created) set as the modification date.

- Config:      use_created_date
- Env Var:     RCLONE_DRIVE_USE_CREATED_DATE
- Type:        bool
- Default:     false

#### --drive-use-shared-date

Use date file was shared instead of modified date.

Note that, as with "--drive-use-created-date", this flag may have
unexpected consequences when uploading/downloading files.

If both this flag and "--drive-use-created-date" are set, the created
date is used.

- Config:      use_shared_date
- Env Var:     RCLONE_DRIVE_USE_SHARED_DATE
- Type:        bool
- Default:     false

#### --drive-list-chunk

Size of listing chunk 100-1000. 0 to disable.

- Config:      list_chunk
- Env Var:     RCLONE_DRIVE_LIST_CHUNK
- Type:        int
- Default:     1000

#### --drive-impersonate

Impersonate this user when using a service account.

Note that if this is used then "root_folder_id" will be ignored.


- Config:      impersonate
- Env Var:     RCLONE_DRIVE_IMPERSONATE
- Type:        string
- Default:     ""

#### --drive-alternate-export

Use alternate export URLs for google documents export.,

If this option is set this instructs rclone to use an alternate set of
export URLs for drive documents.  Users have reported that the
official export URLs can't export large documents, whereas these
unofficial ones can.

See rclone issue [#2243](https://github.com/rclone/rclone/issues/2243) for background,
[this google drive issue](https://issuetracker.google.com/issues/36761333) and
[this helpful post](https://www.labnol.org/internet/direct-links-for-google-drive/28356/).

- Config:      alternate_export
- Env Var:     RCLONE_DRIVE_ALTERNATE_EXPORT
- Type:        bool
- Default:     false

#### --drive-upload-cutoff

Cutoff for switching to chunked upload

- Config:      upload_cutoff
- Env Var:     RCLONE_DRIVE_UPLOAD_CUTOFF
- Type:        SizeSuffix
- Default:     8M

#### --drive-chunk-size

Upload chunk size. Must a power of 2 >= 256k.

Making this larger will improve performance, but note that each chunk
is buffered in memory one per transfer.

Reducing this will reduce memory usage but decrease performance.

- Config:      chunk_size
- Env Var:     RCLONE_DRIVE_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     8M

#### --drive-acknowledge-abuse

Set to allow files which return cannotDownloadAbusiveFile to be downloaded.

If downloading a file returns the error "This file has been identified
as malware or spam and cannot be downloaded" with the error code
"cannotDownloadAbusiveFile" then supply this flag to rclone to
indicate you acknowledge the risks of downloading the file and rclone
will download it anyway.

- Config:      acknowledge_abuse
- Env Var:     RCLONE_DRIVE_ACKNOWLEDGE_ABUSE
- Type:        bool
- Default:     false

#### --drive-keep-revision-forever

Keep new head revision of each file forever.

- Config:      keep_revision_forever
- Env Var:     RCLONE_DRIVE_KEEP_REVISION_FOREVER
- Type:        bool
- Default:     false

#### --drive-size-as-quota

Show sizes as storage quota usage, not actual size.

Show the size of a file as the storage quota used. This is the
current version plus any older versions that have been set to keep
forever.

**WARNING**: This flag may have some unexpected consequences.

It is not recommended to set this flag in your config - the
recommended usage is using the flag form --drive-size-as-quota when
doing rclone ls/lsl/lsf/lsjson/etc only.

If you do use this flag for syncing (not recommended) then you will
need to use --ignore size also.

- Config:      size_as_quota
- Env Var:     RCLONE_DRIVE_SIZE_AS_QUOTA
- Type:        bool
- Default:     false

#### --drive-v2-download-min-size

If Object's are greater, use drive v2 API to download.

- Config:      v2_download_min_size
- Env Var:     RCLONE_DRIVE_V2_DOWNLOAD_MIN_SIZE
- Type:        SizeSuffix
- Default:     off

#### --drive-pacer-min-sleep

Minimum time to sleep between API calls.

- Config:      pacer_min_sleep
- Env Var:     RCLONE_DRIVE_PACER_MIN_SLEEP
- Type:        Duration
- Default:     100ms

#### --drive-pacer-burst

Number of API calls to allow without sleeping.

- Config:      pacer_burst
- Env Var:     RCLONE_DRIVE_PACER_BURST
- Type:        int
- Default:     100

#### --drive-server-side-across-configs

Allow server side operations (eg copy) to work across different drive configs.

This can be useful if you wish to do a server side copy between two
different Google drives.  Note that this isn't enabled by default
because it isn't easy to tell if it will work between any two
configurations.

- Config:      server_side_across_configs
- Env Var:     RCLONE_DRIVE_SERVER_SIDE_ACROSS_CONFIGS
- Type:        bool
- Default:     false

#### --drive-disable-http2

Disable drive using http2

There is currently an unsolved issue with the google drive backend and
HTTP/2.  HTTP/2 is therefore disabled by default for the drive backend
but can be re-enabled here.  When the issue is solved this flag will
be removed.

See: https://github.com/rclone/rclone/issues/3631



- Config:      disable_http2
- Env Var:     RCLONE_DRIVE_DISABLE_HTTP2
- Type:        bool
- Default:     true

#### --drive-stop-on-upload-limit

Make upload limit errors be fatal

At the time of writing it is only possible to upload 750GB of data to
Google Drive a day (this is an undocumented limit). When this limit is
reached Google Drive produces a slightly different error message. When
this flag is set it causes these errors to be fatal.  These will stop
the in-progress sync.

Note that this detection is relying on error message strings which
Google don't document so it may break in the future.

See: https://github.com/rclone/rclone/issues/3857


- Config:      stop_on_upload_limit
- Env Var:     RCLONE_DRIVE_STOP_ON_UPLOAD_LIMIT
- Type:        bool
- Default:     false

#### --drive-skip-shortcuts

If set skip shortcut files

Normally rclone dereferences shortcut files making them appear as if
they are the original file (see [the shortcuts section](#shortcuts)).
If this flag is set then rclone will ignore shortcut files completely.


- Config:      skip_shortcuts
- Env Var:     RCLONE_DRIVE_SKIP_SHORTCUTS
- Type:        bool
- Default:     false

#### --drive-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_DRIVE_ENCODING
- Type:        MultiEncoder
- Default:     InvalidUtf8

### Backend commands

Here are the commands specific to the drive backend.

Run them with

    rclone backend COMMAND remote:

The help below will explain what arguments each command takes.

See [the "rclone backend" command](https://rclone.org/commands/rclone_backend/) for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](https://rclone.org/rc/#backend/command).

#### get

Get command for fetching the drive config parameters

    rclone backend get remote: [options] [<arguments>+]

This is a get command which will be used to fetch the various drive config parameters

Usage Examples:

    rclone backend get drive: [-o service_account_file] [-o chunk_size]
    rclone rc backend/command command=get fs=drive: [-o service_account_file] [-o chunk_size]


Options:

- "chunk_size": show the current upload chunk size
- "service_account_file": show the current service account file

#### set

Set command for updating the drive config parameters

    rclone backend set remote: [options] [<arguments>+]

This is a set command which will be used to update the various drive config parameters

Usage Examples:

    rclone backend set drive: [-o service_account_file=sa.json] [-o chunk_size=67108864]
    rclone rc backend/command command=set fs=drive: [-o service_account_file=sa.json] [-o chunk_size=67108864]


Options:

- "chunk_size": update the current upload chunk size
- "service_account_file": update the current service account file

#### shortcut

Create shortcuts from files or directories

    rclone backend shortcut remote: [options] [<arguments>+]

This command creates shortcuts from files or directories.

Usage:

    rclone backend shortcut drive: source_item destination_shortcut
    rclone backend shortcut drive: source_item -o target=drive2: destination_shortcut

In the first example this creates a shortcut from the "source_item"
which can be a file or a directory to the "destination_shortcut". The
"source_item" and the "destination_shortcut" should be relative paths
from "drive:"

In the second example this creates a shortcut from the "source_item"
relative to "drive:" to the "destination_shortcut" relative to
"drive2:". This may fail with a permission error if the user
authenticated with "drive2:" can't read files from "drive:".


Options:

- "target": optional target remote for the shortcut destination



### Limitations ###

Drive has quite a lot of rate limiting.  This causes rclone to be
limited to transferring about 2 files per second only.  Individual
files may be transferred much faster at 100s of MBytes/s but lots of
small files can take a long time.

Server side copies are also subject to a separate rate limit. If you
see User rate limit exceeded errors, wait at least 24 hours and retry.
You can disable server side copies with `--disable copy` to download
and upload the files if you prefer.

#### Limitations of Google Docs ####

Google docs will appear as size -1 in `rclone ls` and as size 0 in
anything which uses the VFS layer, eg `rclone mount`, `rclone serve`.

This is because rclone can't find out the size of the Google docs
without downloading them.

Google docs will transfer correctly with `rclone sync`, `rclone copy`
etc as rclone knows to ignore the size when doing the transfer.

However an unfortunate consequence of this is that you may not be able
to download Google docs using `rclone mount`. If it doesn't work you
will get a 0 sized file.  If you try again the doc may gain its
correct size and be downloadable. Whether it will work on not depends
on the application accessing the mount and the OS you are running -
experiment to find out if it does work for you!

### Duplicated files ###

Sometimes, for no reason I've been able to track down, drive will
duplicate a file that rclone uploads.  Drive unlike all the other
remotes can have duplicated files.

Duplicated files cause problems with the syncing and you will see
messages in the log about duplicates.

Use `rclone dedupe` to fix duplicated files.

Note that this isn't just a problem with rclone, even Google Photos on
Android duplicates files on drive sometimes.

### Rclone appears to be re-copying files it shouldn't ###

The most likely cause of this is the duplicated file issue above - run
`rclone dedupe` and check your logs for duplicate object or directory
messages.

This can also be caused by a delay/caching on google drive's end when
comparing directory listings. Specifically with team drives used in
combination with --fast-list. Files that were uploaded recently may
not appear on the directory list sent to rclone when using --fast-list.

Waiting a moderate period of time between attempts (estimated to be
approximately 1 hour) and/or not using --fast-list both seem to be
effective in preventing the problem.

### Making your own client_id ###

When you use rclone with Google drive in its default configuration you
are using rclone's client_id.  This is shared between all the rclone
users.  There is a global rate limit on the number of queries per
second that each client_id can do set by Google.  rclone already has a
high quota and I will continue to make sure it is high enough by
contacting Google.

It is strongly recommended to use your own client ID as the default rclone ID is heavily used. If you have multiple services running, it is recommended to use an API key for each service. The default Google quota is 10 transactions per second so it is recommended to stay under that number as if you use more than that, it will cause rclone to rate limit and make things slower.

Here is how to create your own Google Drive client ID for rclone:

1. Log into the [Google API
Console](https://console.developers.google.com/) with your Google
account. It doesn't matter what Google account you use. (It need not
be the same account as the Google Drive you want to access)

2. Select a project or create a new project.

3. Under "ENABLE APIS AND SERVICES" search for "Drive", and enable the
"Google Drive API".

4. Click "Credentials" in the left-side panel (not "Create
credentials", which opens the wizard), then "Create credentials"

5. If you already configured an "Oauth Consent Screen", then skip
to the next step; if not, click on "CONFIGURE CONSENT SCREEN" button 
(near the top right corner of the right panel), then select "External"
and click on "CREATE"; on the next screen, enter an "Application name"
("rclone" is OK) then click on "Save" (all other data is optional). 
Click again on "Credentials" on the left panel to go back to the 
"Credentials" screen.

(PS: if you are a GSuite user, you could also select "Internal" instead
of "External" above, but this has not been tested/documented so far). 

6.  Click on the "+ CREATE CREDENTIALS" button at the top of the screen,
then select "OAuth client ID".

7. Choose an application type of "Desktop app" if you using a Google account or "Other" if 
you using a GSuite account and click "Create". (the default name is fine)

8. It will show you a client ID and client secret.  Use these values
in rclone config to add a new remote or edit an existing remote.

Be aware that, due to the "enhanced security" recently introduced by
Google, you are theoretically expected to "submit your app for verification"
and then wait a few weeks(!) for their response; in practice, you can go right
ahead and use the client ID and client secret with rclone, the only issue will
be a very scary confirmation screen shown when you connect via your browser 
for rclone to be able to get its token-id (but as this only happens during 
the remote configuration, it's not such a big deal). 

(Thanks to @balazer on github for these instructions.)

 Google Photos
-------------------------------------------------

The rclone backend for [Google Photos](https://www.google.com/photos/about/) is
a specialized backend for transferring photos and videos to and from
Google Photos.

**NB** The Google Photos API which rclone uses has quite a few
limitations, so please read the [limitations section](#limitations)
carefully to make sure it is suitable for your use.

## Configuring Google Photos

The initial setup for google cloud storage involves getting a token from Google Photos
which you need to do in your browser.  `rclone config` walks you
through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Google Photos
   \ "google photos"
[snip]
Storage> google photos
** See help for google photos backend at: https://rclone.org/googlephotos/ **

Google Application Client Id
Leave blank normally.
Enter a string value. Press Enter for the default ("").
client_id> 
Google Application Client Secret
Leave blank normally.
Enter a string value. Press Enter for the default ("").
client_secret> 
Set to make the Google Photos backend read only.

If you choose read only then rclone will only request read only access
to your photos, otherwise rclone will request full access.
Enter a boolean value (true or false). Press Enter for the default ("false").
read_only> 
Edit advanced config? (y/n)
y) Yes
n) No
y/n> n
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code

*** IMPORTANT: All media items uploaded to Google Photos with rclone
*** are stored in full resolution at original quality.  These uploads
*** will count towards storage in your Google Account.

--------------------
[remote]
type = google photos
token = {"access_token":"XXX","token_type":"Bearer","refresh_token":"XXX","expiry":"2019-06-28T17:38:04.644930156+01:00"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Note that rclone runs a webserver on your local machine to collect the
token as returned from Google if you use auto config mode. This only
runs from the moment it opens your browser to the moment you get back
the verification code.  This is on `http://127.0.0.1:53682/` and this
may require you to unblock it temporarily if you are running a host
firewall, or use manual mode.

This remote is called `remote` and can now be used like this

See all the albums in your photos

    rclone lsd remote:album

Make a new album

    rclone mkdir remote:album/newAlbum

List the contents of an album

    rclone ls remote:album/newAlbum

Sync `/home/local/images` to the Google Photos, removing any excess
files in the album.

    rclone sync /home/local/image remote:album/newAlbum

## Layout

As Google Photos is not a general purpose cloud storage system the
backend is laid out to help you navigate it.

The directories under `media` show different ways of categorizing the
media.  Each file will appear multiple times.  So if you want to make
a backup of your google photos you might choose to backup
`remote:media/by-month`.  (**NB** `remote:media/by-day` is rather slow
at the moment so avoid for syncing.)

Note that all your photos and videos will appear somewhere under
`media`, but they may not appear under `album` unless you've put them
into albums.

```
/
- upload
    - file1.jpg
    - file2.jpg
    - ...
- media
    - all
        - file1.jpg
        - file2.jpg
        - ...
    - by-year
        - 2000
            - file1.jpg
            - ...
        - 2001
            - file2.jpg
            - ...
        - ...
    - by-month
        - 2000
            - 2000-01
                - file1.jpg
                - ...
            - 2000-02
                - file2.jpg
                - ...
        - ...
    - by-day
        - 2000
            - 2000-01-01
                - file1.jpg
                - ...
            - 2000-01-02
                - file2.jpg
                - ...
        - ...
- album
    - album name
    - album name/sub
- shared-album
    - album name
    - album name/sub
- feature
    - favorites
        - file1.jpg
        - file2.jpg
```

There are two writable parts of the tree, the `upload` directory and
sub directories of the `album` directory.

The `upload` directory is for uploading files you don't want to put
into albums. This will be empty to start with and will contain the
files you've uploaded for one rclone session only, becoming empty
again when you restart rclone. The use case for this would be if you
have a load of files you just want to once off dump into Google
Photos. For repeated syncing, uploading to `album` will work better.

Directories within the `album` directory are also writeable and you
may create new directories (albums) under `album`.  If you copy files
with a directory hierarchy in there then rclone will create albums
with the `/` character in them.  For example if you do

    rclone copy /path/to/images remote:album/images

and the images directory contains

```
images
    - file1.jpg
    dir
        file2.jpg
    dir2
        dir3
            file3.jpg
```

Then rclone will create the following albums with the following files in

- images
    - file1.jpg
- images/dir
    - file2.jpg
- images/dir2/dir3
    - file3.jpg

This means that you can use the `album` path pretty much like a normal
filesystem and it is a good target for repeated syncing.

The `shared-album` directory shows albums shared with you or by you.
This is similar to the Sharing tab in the Google Photos web interface.

## Limitations

Only images and videos can be uploaded.  If you attempt to upload non
videos or images or formats that Google Photos doesn't understand,
rclone will upload the file, then Google Photos will give an error
when it is put turned into a media item.

Note that all media items uploaded to Google Photos through the API
are stored in full resolution at "original quality" and **will** count
towards your storage quota in your Google Account.  The API does
**not** offer a way to upload in "high quality" mode..

### Downloading Images

When Images are downloaded this strips EXIF location (according to the
docs and my tests).  This is a limitation of the Google Photos API and
is covered by [bug #112096115](https://issuetracker.google.com/issues/112096115).

**The current google API does not allow photos to be downloaded at original resolution.  This is very important if you are, for example, relying on "Google Photos" as a backup of your photos.  You will not be able to use rclone to redownload original images.  You could use 'google takeout' to recover the original photos as a last resort**

### Downloading Videos

When videos are downloaded they are downloaded in a really compressed
version of the video compared to downloading it via the Google Photos
web interface. This is covered by [bug #113672044](https://issuetracker.google.com/issues/113672044).

### Duplicates

If a file name is duplicated in a directory then rclone will add the
file ID into its name.  So two files called `file.jpg` would then
appear as `file {123456}.jpg` and `file {ABCDEF}.jpg` (the actual IDs
are a lot longer alas!).

If you upload the same image (with the same binary data) twice then
Google Photos will deduplicate it.  However it will retain the
filename from the first upload which may confuse rclone.  For example
if you uploaded an image to `upload` then uploaded the same image to
`album/my_album` the filename of the image in `album/my_album` will be
what it was uploaded with initially, not what you uploaded it with to
`album`.  In practise this shouldn't cause too many problems.

### Modified time

The date shown of media in Google Photos is the creation date as
determined by the EXIF information, or the upload date if that is not
known.

This is not changeable by rclone and is not the modification date of
the media on local disk.  This means that rclone cannot use the dates
from Google Photos for syncing purposes.

### Size

The Google Photos API does not return the size of media.  This means
that when syncing to Google Photos, rclone can only do a file
existence check.

It is possible to read the size of the media, but this needs an extra
HTTP HEAD request per media item so is **very slow** and uses up a lot of
transactions.  This can be enabled with the `--gphotos-read-size`
option or the `read_size = true` config parameter.

If you want to use the backend with `rclone mount` you may need to
enable this flag (depending on your OS and application using the
photos) otherwise you may not be able to read media off the mount.
You'll need to experiment to see if it works for you without the flag.

### Albums

Rclone can only upload files to albums it created. This is a
[limitation of the Google Photos API](https://developers.google.com/photos/library/guides/manage-albums).

Rclone can remove files it uploaded from albums it created only.

### Deleting files

Rclone can remove files from albums it created, but note that the
Google Photos API does not allow media to be deleted permanently so
this media will still remain. See [bug #109759781](https://issuetracker.google.com/issues/109759781).

Rclone cannot delete files anywhere except under `album`.

### Deleting albums

The Google Photos API does not support deleting albums - see [bug #135714733](https://issuetracker.google.com/issues/135714733).


### Standard Options

Here are the standard options specific to google photos (Google Photos).

#### --gphotos-client-id

Google Application Client Id
Leave blank normally.

- Config:      client_id
- Env Var:     RCLONE_GPHOTOS_CLIENT_ID
- Type:        string
- Default:     ""

#### --gphotos-client-secret

Google Application Client Secret
Leave blank normally.

- Config:      client_secret
- Env Var:     RCLONE_GPHOTOS_CLIENT_SECRET
- Type:        string
- Default:     ""

#### --gphotos-read-only

Set to make the Google Photos backend read only.

If you choose read only then rclone will only request read only access
to your photos, otherwise rclone will request full access.

- Config:      read_only
- Env Var:     RCLONE_GPHOTOS_READ_ONLY
- Type:        bool
- Default:     false

### Advanced Options

Here are the advanced options specific to google photos (Google Photos).

#### --gphotos-read-size

Set to read the size of media items.

Normally rclone does not read the size of media items since this takes
another transaction.  This isn't necessary for syncing.  However
rclone mount needs to know the size of files in advance of reading
them, so setting this flag when using rclone mount is recommended if
you want to read the media.

- Config:      read_size
- Env Var:     RCLONE_GPHOTOS_READ_SIZE
- Type:        bool
- Default:     false

#### --gphotos-start-year

Year limits the photos to be downloaded to those which are uploaded after the given year

- Config:      start_year
- Env Var:     RCLONE_GPHOTOS_START_YEAR
- Type:        int
- Default:     2000



 HTTP
-------------------------------------------------

The HTTP remote is a read only remote for reading files of a
webserver.  The webserver should provide file listings which rclone
will read and turn into a remote.  This has been tested with common
webservers such as Apache/Nginx/Caddy and will likely work with file
listings from most web servers.  (If it doesn't then please file an
issue, or send a pull request!)

Paths are specified as `remote:` or `remote:path/to/dir`.

Here is an example of how to make a remote called `remote`.  First
run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / http Connection
   \ "http"
[snip]
Storage> http
URL of http host to connect to
Choose a number from below, or type in your own value
 1 / Connect to example.com
   \ "https://example.com"
url> https://beta.rclone.org
Remote config
--------------------
[remote]
url = https://beta.rclone.org
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
Current remotes:

Name                 Type
====                 ====
remote               http

e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> q
```

This remote is called `remote` and can now be used like this

See all the top level directories

    rclone lsd remote:

List the contents of a directory

    rclone ls remote:directory

Sync the remote `directory` to `/home/local/directory`, deleting any excess files.

    rclone sync remote:directory /home/local/directory

### Read only ###

This remote is read only - you can't upload files to an HTTP server.

### Modified time ###

Most HTTP servers store time accurate to 1 second.

### Checksum ###

No checksums are stored.

### Usage without a config file ###

Since the http remote only has one config parameter it is easy to use
without a config file:

    rclone lsd --http-url https://beta.rclone.org :http:


### Standard Options

Here are the standard options specific to http (http Connection).

#### --http-url

URL of http host to connect to

- Config:      url
- Env Var:     RCLONE_HTTP_URL
- Type:        string
- Default:     ""
- Examples:
    - "https://example.com"
        - Connect to example.com
    - "https://user:pass@example.com"
        - Connect to example.com using a username and password

### Advanced Options

Here are the advanced options specific to http (http Connection).

#### --http-headers

Set HTTP headers for all transactions

Use this to set additional HTTP headers for all transactions

The input format is comma separated list of key,value pairs.  Standard
[CSV encoding](https://godoc.org/encoding/csv) may be used.

For example to set a Cookie use 'Cookie,name=value', or '"Cookie","name=value"'.

You can set multiple headers, eg '"Cookie","name=value","Authorization","xxx"'.


- Config:      headers
- Env Var:     RCLONE_HTTP_HEADERS
- Type:        CommaSepList
- Default:     

#### --http-no-slash

Set this if the site doesn't end directories with /

Use this if your target website does not use / on the end of
directories.

A / on the end of a path is how rclone normally tells the difference
between files and directories.  If this flag is set, then rclone will
treat all files with Content-Type: text/html as directories and read
URLs from them rather than downloading them.

Note that this may cause rclone to confuse genuine HTML files with
directories.

- Config:      no_slash
- Env Var:     RCLONE_HTTP_NO_SLASH
- Type:        bool
- Default:     false

#### --http-no-head

Don't use HEAD requests to find file sizes in dir listing

If your site is being very slow to load then you can try this option.
Normally rclone does a HEAD request for each potential file in a
directory listing to:

- find its size
- check it really exists
- check to see if it is a directory

If you set this option, rclone will not do the HEAD request.  This will mean

- directory listings are much quicker
- rclone won't have the times or sizes of any files
- some files that don't exist may be in the listing


- Config:      no_head
- Env Var:     RCLONE_HTTP_NO_HEAD
- Type:        bool
- Default:     false



 Hubic
-----------------------------------------

Paths are specified as `remote:path`

Paths are specified as `remote:container` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, eg `remote:container/path/to/dir`.

The initial setup for Hubic involves getting a token from Hubic which
you need to do in your browser.  `rclone config` walks you through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
n) New remote
s) Set configuration password
n/s> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Hubic
   \ "hubic"
[snip]
Storage> hubic
Hubic Client Id - leave blank normally.
client_id>
Hubic Client Secret - leave blank normally.
client_secret>
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
client_id =
client_secret =
token = {"access_token":"XXXXXX"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](https://rclone.org/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Hubic. This only runs from the moment it opens
your browser to the moment you get back the verification code.  This
is on `http://127.0.0.1:53682/` and this it may require you to unblock
it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List containers in the top level of your Hubic

    rclone lsd remote:

List all the files in your Hubic

    rclone ls remote:

To copy a local directory to an Hubic directory called backup

    rclone copy /home/source remote:backup

If you want the directory to be visible in the official *Hubic
browser*, you need to copy your files to the `default` directory

    rclone copy /home/source remote:default/backup

### --fast-list ###

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](https://rclone.org/docs/#fast-list) for more details.

### Modified time ###

The modified time is stored as metadata on the object as
`X-Object-Meta-Mtime` as floating point since the epoch accurate to 1
ns.

This is a de facto standard (used in the official python-swiftclient
amongst others) for storing the modification time for an object.

Note that Hubic wraps the Swift backend, so most of the properties of
are the same.


### Standard Options

Here are the standard options specific to hubic (Hubic).

#### --hubic-client-id

Hubic Client Id
Leave blank normally.

- Config:      client_id
- Env Var:     RCLONE_HUBIC_CLIENT_ID
- Type:        string
- Default:     ""

#### --hubic-client-secret

Hubic Client Secret
Leave blank normally.

- Config:      client_secret
- Env Var:     RCLONE_HUBIC_CLIENT_SECRET
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to hubic (Hubic).

#### --hubic-chunk-size

Above this size files will be chunked into a _segments container.

Above this size files will be chunked into a _segments container.  The
default for this is 5GB which is its maximum value.

- Config:      chunk_size
- Env Var:     RCLONE_HUBIC_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     5G

#### --hubic-no-chunk

Don't chunk files during streaming upload.

When doing streaming uploads (eg using rcat or mount) setting this
flag will cause the swift backend to not upload chunked files.

This will limit the maximum upload size to 5GB. However non chunked
files are easier to deal with and have an MD5SUM.

Rclone will still chunk files bigger than chunk_size when doing normal
copy operations.

- Config:      no_chunk
- Env Var:     RCLONE_HUBIC_NO_CHUNK
- Type:        bool
- Default:     false

#### --hubic-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_HUBIC_ENCODING
- Type:        MultiEncoder
- Default:     Slash,InvalidUtf8



### Limitations ###

This uses the normal OpenStack Swift mechanism to refresh the Swift
API credentials and ignores the expires field returned by the Hubic
API.

The Swift API doesn't return a correct MD5SUM for segmented files
(Dynamic or Static Large Objects) so rclone won't check or use the
MD5SUM for these.

 Jottacloud
-----------------------------------------

Jottacloud is a cloud storage service provider from a Norwegian company, using its own datacenters in Norway.

In addition to the official service at [jottacloud.com](https://www.jottacloud.com/), there are
also several whitelabel versions which should work with this backend.

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

## Setup

To configure Jottacloud you will need to generate a personal security token in the Jottacloud web interface.
You will the option to do in your [account security settings](https://www.jottacloud.com/web/secure)
(for whitelabel version you need to find this page in its web interface).
Note that the web interface may refer to this token as a JottaCli token.

Here is an example of how to make a remote called `remote`.  First run:

    rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> jotta
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Jottacloud
   \ "jottacloud"
[snip]
Storage> jottacloud
** See help for jottacloud backend at: https://rclone.org/jottacloud/ **

Edit advanced config? (y/n)
y) Yes
n) No
y/n> n
Remote config

Generate a personal login token here: https://www.jottacloud.com/web/secure
Login Token> <your token here>

Do you want to use a non standard device/mountpoint e.g. for accessing files uploaded using the official Jottacloud client?

y) Yes
n) No
y/n> y
Please select the device to use. Normally this will be Jotta
Choose a number from below, or type in an existing value
 1 > DESKTOP-3H31129
 2 > fla1
 3 > Jotta
Devices> 3
Please select the mountpoint to user. Normally this will be Archive
Choose a number from below, or type in an existing value
 1 > Archive
 2 > Links
 3 > Sync
 
Mountpoints> 1
--------------------
[jotta]
type = jottacloud
user = 0xC4KE@gmail.com
token = {........}
device = Jotta
mountpoint = Archive
configVersion = 1
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```
Once configured you can then use `rclone` like this,

List directories in top level of your Jottacloud

    rclone lsd remote:

List all the files in your Jottacloud

    rclone ls remote:

To copy a local directory to an Jottacloud directory called backup

    rclone copy /home/source remote:backup

### Devices and Mountpoints

The official Jottacloud client registers a device for each computer you install it on,
and then creates a mountpoint for each folder you select for Backup.
The web interface uses a special device called Jotta for the Archive and Sync mountpoints.
In most cases you'll want to use the Jotta/Archive device/mountpoint, however if you want to access
files uploaded by any of the official clients rclone provides the option to select other devices
and mountpoints during config.

The built-in Jotta device may also contain several other mountpoints, such as: Latest, Links, Shared and Trash.
These are special mountpoints with a different internal representation than the "regular" mountpoints.
Rclone will only to a very limited degree support them. Generally you should avoid these, unless you know what you
are doing.

### --fast-list

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](https://rclone.org/docs/#fast-list) for more details.

Note that the implementation in Jottacloud always uses only a single
API request to get the entire list, so for large folders this could
lead to long wait time before the first results are shown.

### Modified time and hashes

Jottacloud allows modification times to be set on objects accurate to 1
second. These will be used to detect whether objects need syncing or
not.

Jottacloud supports MD5 type hashes, so you can use the `--checksum`
flag.

Note that Jottacloud requires the MD5 hash before upload so if the
source does not have an MD5 checksum then the file will be cached
temporarily on disk (wherever the `TMPDIR` environment variable points
to) before it is uploaded.  Small files will be cached in memory - see
the [--jottacloud-md5-memory-limit](#jottacloud-md5-memory-limit) flag.

#### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| "         | 0x22  | ＂          |
| *         | 0x2A  | ＊          |
| :         | 0x3A  | ：          |
| <         | 0x3C  | ＜          |
| >         | 0x3E  | ＞          |
| ?         | 0x3F  | ？          |
| \|        | 0x7C  | ｜          |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in XML strings.

### Deleting files

By default rclone will send all files to the trash when deleting files. They will be permanently
deleted automatically after 30 days. You may bypass the trash and permanently delete files immediately
by using the [--jottacloud-hard-delete](#jottacloud-hard-delete) flag, or set the equivalent environment variable.
Emptying the trash is supported by the [cleanup](https://rclone.org/commands/rclone_cleanup/) command.

### Versions

Jottacloud supports file versioning. When rclone uploads a new version of a file it creates a new version of it.
Currently rclone only supports retrieving the current version but older versions can be accessed via the Jottacloud Website.

### Quota information

To view your current quota you can use the `rclone about remote:`
command which will display your usage limit (unless it is unlimited)
and the current usage.


### Advanced Options

Here are the advanced options specific to jottacloud (Jottacloud).

#### --jottacloud-md5-memory-limit

Files bigger than this will be cached on disk to calculate the MD5 if required.

- Config:      md5_memory_limit
- Env Var:     RCLONE_JOTTACLOUD_MD5_MEMORY_LIMIT
- Type:        SizeSuffix
- Default:     10M

#### --jottacloud-trashed-only

Only show files that are in the trash.
This will show trashed files in their original directory structure.

- Config:      trashed_only
- Env Var:     RCLONE_JOTTACLOUD_TRASHED_ONLY
- Type:        bool
- Default:     false

#### --jottacloud-hard-delete

Delete files permanently rather than putting them into the trash.

- Config:      hard_delete
- Env Var:     RCLONE_JOTTACLOUD_HARD_DELETE
- Type:        bool
- Default:     false

#### --jottacloud-unlink

Remove existing public link to file/folder with link command rather than creating.
Default is false, meaning link command will create or retrieve public link.

- Config:      unlink
- Env Var:     RCLONE_JOTTACLOUD_UNLINK
- Type:        bool
- Default:     false

#### --jottacloud-upload-resume-limit

Files bigger than this can be resumed if the upload fail's.

- Config:      upload_resume_limit
- Env Var:     RCLONE_JOTTACLOUD_UPLOAD_RESUME_LIMIT
- Type:        SizeSuffix
- Default:     10M

#### --jottacloud-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_JOTTACLOUD_ENCODING
- Type:        MultiEncoder
- Default:     Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,Del,Ctl,InvalidUtf8,Dot



### Limitations

Note that Jottacloud is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

There are quite a few characters that can't be in Jottacloud file names. Rclone will map these names to and from an identical
looking unicode equivalent. For example if a file has a ? in it will be mapped to ？ instead.

Jottacloud only supports filenames up to 255 characters in length.

### Troubleshooting

Jottacloud exhibits some inconsistent behaviours regarding deleted files and folders which may cause Copy, Move and DirMove
operations to previously deleted paths to fail. Emptying the trash should help in such cases.

 Koofr
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

The initial setup for Koofr involves creating an application password for
rclone. You can do that by opening the Koofr
[web application](https://app.koofr.net/app/admin/preferences/password),
giving the password a nice name like `rclone` and clicking on generate.

Here is an example of how to make a remote called `koofr`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> koofr 
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Koofr
   \ "koofr"
[snip]
Storage> koofr
** See help for koofr backend at: https://rclone.org/koofr/ **

Your Koofr user name
Enter a string value. Press Enter for the default ("").
user> USER@NAME
Your Koofr password for rclone (generate one at https://app.koofr.net/app/admin/preferences/password)
y) Yes type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
Edit advanced config? (y/n)
y) Yes
n) No
y/n> n
Remote config
--------------------
[koofr]
type = koofr
baseurl = https://app.koofr.net
user = USER@NAME
password = *** ENCRYPTED ***
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

You can choose to edit advanced config in order to enter your own service URL
if you use an on-premise or white label Koofr instance, or choose an alternative
mount instead of your primary storage.

Once configured you can then use `rclone` like this,

List directories in top level of your Koofr

    rclone lsd koofr:

List all the files in your Koofr

    rclone ls koofr:

To copy a local directory to an Koofr directory called backup

    rclone copy /home/source remote:backup

#### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| \         | 0x5C  | ＼           |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in XML strings.


### Standard Options

Here are the standard options specific to koofr (Koofr).

#### --koofr-user

Your Koofr user name

- Config:      user
- Env Var:     RCLONE_KOOFR_USER
- Type:        string
- Default:     ""

#### --koofr-password

Your Koofr password for rclone (generate one at https://app.koofr.net/app/admin/preferences/password)

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      password
- Env Var:     RCLONE_KOOFR_PASSWORD
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to koofr (Koofr).

#### --koofr-endpoint

The Koofr API endpoint to use

- Config:      endpoint
- Env Var:     RCLONE_KOOFR_ENDPOINT
- Type:        string
- Default:     "https://app.koofr.net"

#### --koofr-mountid

Mount ID of the mount to use. If omitted, the primary mount is used.

- Config:      mountid
- Env Var:     RCLONE_KOOFR_MOUNTID
- Type:        string
- Default:     ""

#### --koofr-setmtime

Does the backend support setting modification time. Set this to false if you use a mount ID that points to a Dropbox or Amazon Drive backend.

- Config:      setmtime
- Env Var:     RCLONE_KOOFR_SETMTIME
- Type:        bool
- Default:     true

#### --koofr-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_KOOFR_ENCODING
- Type:        MultiEncoder
- Default:     Slash,BackSlash,Del,Ctl,InvalidUtf8,Dot



### Limitations ###

Note that Koofr is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

 Mail.ru Cloud
----------------------------------------

[Mail.ru Cloud](https://cloud.mail.ru/) is a cloud storage provided by a Russian internet company [Mail.Ru Group](https://mail.ru). The official desktop client is [Disk-O:](https://disk-o.cloud/), available only on Windows. (Please note that official sites are in Russian)

Currently it is recommended to disable 2FA on Mail.ru accounts intended for rclone until it gets eventually implemented.

### Features highlights ###

- Paths may be as deep as required, eg `remote:directory/subdirectory`
- Files have a `last modified time` property, directories don't
- Deleted files are by default moved to the trash
- Files and directories can be shared via public links
- Partial uploads or streaming are not supported, file size must be known before upload
- Maximum file size is limited to 2G for a free account, unlimited for paid accounts
- Storage keeps hash for all files and performs transparent deduplication,
  the hash algorithm is a modified SHA1
- If a particular file is already present in storage, one can quickly submit file hash
  instead of long file upload (this optimization is supported by rclone)

### Configuration ###

Here is an example of making a mailru configuration. First create a Mail.ru Cloud
account and choose a tariff, then run

    rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Mail.ru Cloud
   \ "mailru"
[snip]
Storage> mailru
User name (usually email)
Enter a string value. Press Enter for the default ("").
user> username@mail.ru
Password
y) Yes type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
Skip full upload if there is another file with same data hash.
This feature is called "speedup" or "put by hash". It is especially efficient
in case of generally available files like popular books, video or audio clips
[snip]
Enter a boolean value (true or false). Press Enter for the default ("true").
Choose a number from below, or type in your own value
 1 / Enable
   \ "true"
 2 / Disable
   \ "false"
speedup_enable> 1
Edit advanced config? (y/n)
y) Yes
n) No
y/n> n
Remote config
--------------------
[remote]
type = mailru
user = username@mail.ru
pass = *** ENCRYPTED ***
speedup_enable = true
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Configuration of this backend does not require a local web browser.
You can use the configured backend as shown below:

See top level directories

    rclone lsd remote:

Make a new directory

    rclone mkdir remote:directory

List the contents of a directory

    rclone ls remote:directory

Sync `/home/local/directory` to the remote path, deleting any
excess files in the path.

    rclone sync /home/local/directory remote:directory

### Modified time ###

Files support a modification time attribute with up to 1 second precision.
Directories do not have a modification time, which is shown as "Jan 1 1970".

### Hash checksums ###

Hash sums use a custom Mail.ru algorithm based on SHA1.
If file size is less than or equal to the SHA1 block size (20 bytes),
its hash is simply its data right-padded with zero bytes.
Hash sum of a larger file is computed as a SHA1 sum of the file data
bytes concatenated with a decimal representation of the data length.

### Emptying Trash ###

Removing a file or directory actually moves it to the trash, which is not
visible to rclone but can be seen in a web browser. The trashed file
still occupies part of total quota. If you wish to empty your trash
and free some quota, you can use the `rclone cleanup remote:` command,
which will permanently delete all your trashed files.
This command does not take any path arguments.

### Quota information ###

To view your current quota you can use the `rclone about remote:`
command which will display your usage limit (quota) and the current usage.

#### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| "         | 0x22  | ＂          |
| *         | 0x2A  | ＊          |
| :         | 0x3A  | ：          |
| <         | 0x3C  | ＜          |
| >         | 0x3E  | ＞          |
| ?         | 0x3F  | ？          |
| \         | 0x5C  | ＼          |
| \|        | 0x7C  | ｜          |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Limitations ###

File size limits depend on your account. A single file size is limited by 2G
for a free account and unlimited for paid tariffs. Please refer to the Mail.ru
site for the total uploaded size limits.

Note that Mailru is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".


### Standard Options

Here are the standard options specific to mailru (Mail.ru Cloud).

#### --mailru-user

User name (usually email)

- Config:      user
- Env Var:     RCLONE_MAILRU_USER
- Type:        string
- Default:     ""

#### --mailru-pass

Password

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      pass
- Env Var:     RCLONE_MAILRU_PASS
- Type:        string
- Default:     ""

#### --mailru-speedup-enable

Skip full upload if there is another file with same data hash.
This feature is called "speedup" or "put by hash". It is especially efficient
in case of generally available files like popular books, video or audio clips,
because files are searched by hash in all accounts of all mailru users.
Please note that rclone may need local memory and disk space to calculate
content hash in advance and decide whether full upload is required.
Also, if rclone does not know file size in advance (e.g. in case of
streaming or partial uploads), it will not even try this optimization.

- Config:      speedup_enable
- Env Var:     RCLONE_MAILRU_SPEEDUP_ENABLE
- Type:        bool
- Default:     true
- Examples:
    - "true"
        - Enable
    - "false"
        - Disable

### Advanced Options

Here are the advanced options specific to mailru (Mail.ru Cloud).

#### --mailru-speedup-file-patterns

Comma separated list of file name patterns eligible for speedup (put by hash).
Patterns are case insensitive and can contain '*' or '?' meta characters.

- Config:      speedup_file_patterns
- Env Var:     RCLONE_MAILRU_SPEEDUP_FILE_PATTERNS
- Type:        string
- Default:     "*.mkv,*.avi,*.mp4,*.mp3,*.zip,*.gz,*.rar,*.pdf"
- Examples:
    - ""
        - Empty list completely disables speedup (put by hash).
    - "*"
        - All files will be attempted for speedup.
    - "*.mkv,*.avi,*.mp4,*.mp3"
        - Only common audio/video files will be tried for put by hash.
    - "*.zip,*.gz,*.rar,*.pdf"
        - Only common archives or PDF books will be tried for speedup.

#### --mailru-speedup-max-disk

This option allows you to disable speedup (put by hash) for large files
(because preliminary hashing can exhaust you RAM or disk space)

- Config:      speedup_max_disk
- Env Var:     RCLONE_MAILRU_SPEEDUP_MAX_DISK
- Type:        SizeSuffix
- Default:     3G
- Examples:
    - "0"
        - Completely disable speedup (put by hash).
    - "1G"
        - Files larger than 1Gb will be uploaded directly.
    - "3G"
        - Choose this option if you have less than 3Gb free on local disk.

#### --mailru-speedup-max-memory

Files larger than the size given below will always be hashed on disk.

- Config:      speedup_max_memory
- Env Var:     RCLONE_MAILRU_SPEEDUP_MAX_MEMORY
- Type:        SizeSuffix
- Default:     32M
- Examples:
    - "0"
        - Preliminary hashing will always be done in a temporary disk location.
    - "32M"
        - Do not dedicate more than 32Mb RAM for preliminary hashing.
    - "256M"
        - You have at most 256Mb RAM free for hash calculations.

#### --mailru-check-hash

What should copy do if file checksum is mismatched or invalid

- Config:      check_hash
- Env Var:     RCLONE_MAILRU_CHECK_HASH
- Type:        bool
- Default:     true
- Examples:
    - "true"
        - Fail with error.
    - "false"
        - Ignore and continue.

#### --mailru-user-agent

HTTP user agent used internally by client.
Defaults to "rclone/VERSION" or "--user-agent" provided on command line.

- Config:      user_agent
- Env Var:     RCLONE_MAILRU_USER_AGENT
- Type:        string
- Default:     ""

#### --mailru-quirks

Comma separated list of internal maintenance flags.
This option must not be used by an ordinary user. It is intended only to
facilitate remote troubleshooting of backend issues. Strict meaning of
flags is not documented and not guaranteed to persist between releases.
Quirks will be removed when the backend grows stable.
Supported quirks: atomicmkdir binlist gzip insecure retry400

- Config:      quirks
- Env Var:     RCLONE_MAILRU_QUIRKS
- Type:        string
- Default:     ""

#### --mailru-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_MAILRU_ENCODING
- Type:        MultiEncoder
- Default:     Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,Del,Ctl,InvalidUtf8,Dot



 Mega
-----------------------------------------

[Mega](https://mega.nz/) is a cloud storage and file hosting service
known for its security feature where all files are encrypted locally
before they are uploaded. This prevents anyone (including employees of
Mega) from accessing the files without knowledge of the key used for
encryption.

This is an rclone backend for Mega which supports the file transfer
features of Mega using the same client side encryption.

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Mega
   \ "mega"
[snip]
Storage> mega
User name
user> you@example.com
Password.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank
y/g/n> y
Enter the password:
password:
Confirm the password:
password:
Remote config
--------------------
[remote]
type = mega
user = you@example.com
pass = *** ENCRYPTED ***
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

**NOTE:** The encryption keys need to have been already generated after a regular login
via the browser, otherwise attempting to use the credentials in `rclone` will fail.

Once configured you can then use `rclone` like this,

List directories in top level of your Mega

    rclone lsd remote:

List all the files in your Mega

    rclone ls remote:

To copy a local directory to an Mega directory called backup

    rclone copy /home/source remote:backup

### Modified time and hashes ###

Mega does not support modification times or hashes yet.

#### Restricted filename characters

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| /         | 0x2F  | ／          |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Duplicated files ###

Mega can have two files with exactly the same name and path (unlike a
normal file system).

Duplicated files cause problems with the syncing and you will see
messages in the log about duplicates.

Use `rclone dedupe` to fix duplicated files.

### Failure to log-in ###

Mega remotes seem to get blocked (reject logins) under "heavy use".
We haven't worked out the exact blocking rules but it seems to be
related to fast paced, successive rclone commands.

For example, executing this command 90 times in a row `rclone link
remote:file` will cause the remote to become "blocked". This is not an
abnormal situation, for example if you wish to get the public links of
a directory with hundred of files...  After more or less a week, the
remote will remote accept rclone logins normally again.

You can mitigate this issue by mounting the remote it with `rclone
mount`. This will log-in when mounting and a log-out when unmounting
only. You can also run `rclone rcd` and then use `rclone rc` to run
the commands over the API to avoid logging in each time.

Rclone does not currently close mega sessions (you can see them in the
web interface), however closing the sessions does not solve the issue.

If you space rclone commands by 3 seconds it will avoid blocking the
remote. We haven't identified the exact blocking rules, so perhaps one
could execute the command 80 times without waiting and avoid blocking
by waiting 3 seconds, then continuing...

Note that this has been observed by trial and error and might not be
set in stone.

Other tools seem not to produce this blocking effect, as they use a
different working approach (state-based, using sessionIDs instead of
log-in) which isn't compatible with the current stateless rclone
approach.

Note that once blocked, the use of other tools (such as megacmd) is
not a sure workaround: following megacmd login times have been
observed in succession for blocked remote: 7 minutes, 20 min, 30min, 30
min, 30min. Web access looks unaffected though.

Investigation is continuing in relation to workarounds based on
timeouts, pacers, retrials and tpslimits - if you discover something
relevant, please post on the forum.

So, if rclone was working nicely and suddenly you are unable to log-in
and you are sure the user and the password are correct, likely you
have got the remote blocked for a while.


### Standard Options

Here are the standard options specific to mega (Mega).

#### --mega-user

User name

- Config:      user
- Env Var:     RCLONE_MEGA_USER
- Type:        string
- Default:     ""

#### --mega-pass

Password.

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      pass
- Env Var:     RCLONE_MEGA_PASS
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to mega (Mega).

#### --mega-debug

Output more debug from Mega.

If this flag is set (along with -vv) it will print further debugging
information from the mega backend.

- Config:      debug
- Env Var:     RCLONE_MEGA_DEBUG
- Type:        bool
- Default:     false

#### --mega-hard-delete

Delete files permanently rather than putting them into the trash.

Normally the mega backend will put all deletions into the trash rather
than permanently deleting them.  If you specify this then rclone will
permanently delete objects instead.

- Config:      hard_delete
- Env Var:     RCLONE_MEGA_HARD_DELETE
- Type:        bool
- Default:     false

#### --mega-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_MEGA_ENCODING
- Type:        MultiEncoder
- Default:     Slash,InvalidUtf8,Dot



### Limitations ###

This backend uses the [go-mega go library](https://github.com/t3rm1n4l/go-mega) which is an opensource
go library implementing the Mega API. There doesn't appear to be any
documentation for the mega protocol beyond the [mega C++ SDK](https://github.com/meganz/sdk) source code
so there are likely quite a few errors still remaining in this library.

Mega allows duplicate files which may confuse rclone.

 Memory
-----------------------------------------

The memory backend is an in RAM backend. It does not persist its
data - use the local backend for that.

The memory backend behaves like a bucket based remote (eg like
s3). Because it has no parameters you can just use it with the
`:memory:` remote name.

You can configure it as a remote like this with `rclone config` too if
you want to:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Memory
   \ "memory"
[snip]
Storage> memory
** See help for memory backend at: https://rclone.org/memory/ **

Remote config

--------------------
[remote]
type = memory
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Because the memory backend isn't persistent it is most useful for
testing or with an rclone server or rclone mount, eg

    rclone mount :memory: /mnt/tmp
    rclone serve webdav :memory:
    rclone serve sftp :memory:

### Modified time and hashes ###

The memory backend supports MD5 hashes and modification times accurate to 1 nS.

#### Restricted filename characters

The memory backend replaces the [default restricted characters
set](https://rclone.org/overview/#restricted-characters).




 Microsoft Azure Blob Storage
-----------------------------------------

Paths are specified as `remote:container` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, eg
`remote:container/path/to/dir`.

Here is an example of making a Microsoft Azure Blob Storage
configuration.  For a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Microsoft Azure Blob Storage
   \ "azureblob"
[snip]
Storage> azureblob
Storage Account Name
account> account_name
Storage Account Key
key> base64encodedkey==
Endpoint for the service - leave blank normally.
endpoint> 
Remote config
--------------------
[remote]
account = account_name
key = base64encodedkey==
endpoint = 
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See all containers

    rclone lsd remote:

Make a new container

    rclone mkdir remote:container

List the contents of a container

    rclone ls remote:container

Sync `/home/local/directory` to the remote container, deleting any excess
files in the container.

    rclone sync /home/local/directory remote:container

### --fast-list ###

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](https://rclone.org/docs/#fast-list) for more details.

### Modified time ###

The modified time is stored as metadata on the object with the `mtime`
key.  It is stored using RFC3339 Format time with nanosecond
precision.  The metadata is supplied during directory listings so
there is no overhead to using it.

### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| /         | 0x2F  | ／           |
| \         | 0x5C  | ＼           |

File names can also not end with the following characters.
These only get replaced if they are the last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| .         | 0x2E  | ．          |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Hashes ###

MD5 hashes are stored with blobs.  However blobs that were uploaded in
chunks only have an MD5 if the source remote was capable of MD5
hashes, eg the local disk.

### Authenticating with Azure Blob Storage

Rclone has 3 ways of authenticating with Azure Blob Storage:

#### Account and Key

This is the most straight forward and least flexible way.  Just fill
in the `account` and `key` lines and leave the rest blank.

#### SAS URL

This can be an account level SAS URL or container level SAS URL.

To use it leave `account`, `key` blank and fill in `sas_url`.

An account level SAS URL or container level SAS URL can be obtained
from the Azure portal or the Azure Storage Explorer.  To get a
container level SAS URL right click on a container in the Azure Blob
explorer in the Azure portal.

If you use a container level SAS URL, rclone operations are permitted
only on a particular container, eg

    rclone ls azureblob:container

You can also list the single container from the root. This will only
show the container specified by the SAS URL.

    $ rclone lsd azureblob:
    container/

Note that you can't see or access any other containers - this will
fail

    rclone ls azureblob:othercontainer

Container level SAS URLs are useful for temporarily allowing third
parties access to a single container or putting credentials into an
untrusted environment such as a CI build server.

### Multipart uploads ###

Rclone supports multipart uploads with Azure Blob storage.  Files
bigger than 256MB will be uploaded using chunked upload by default.

The files will be uploaded in parallel in 4MB chunks (by default).
Note that these chunks are buffered in memory and there may be up to
`--transfers` of them being uploaded at once.

Files can't be split into more than 50,000 chunks so by default, so
the largest file that can be uploaded with 4MB chunk size is 195GB.
Above this rclone will double the chunk size until it creates less
than 50,000 chunks.  By default this will mean a maximum file size of
3.2TB can be uploaded.  This can be raised to 5TB using
`--azureblob-chunk-size 100M`.

Note that rclone doesn't commit the block list until the end of the
upload which means that there is a limit of 9.5TB of multipart uploads
in progress as Azure won't allow more than that amount of uncommitted
blocks.


### Standard Options

Here are the standard options specific to azureblob (Microsoft Azure Blob Storage).

#### --azureblob-account

Storage Account Name (leave blank to use SAS URL or Emulator)

- Config:      account
- Env Var:     RCLONE_AZUREBLOB_ACCOUNT
- Type:        string
- Default:     ""

#### --azureblob-key

Storage Account Key (leave blank to use SAS URL or Emulator)

- Config:      key
- Env Var:     RCLONE_AZUREBLOB_KEY
- Type:        string
- Default:     ""

#### --azureblob-sas-url

SAS URL for container level access only
(leave blank if using account/key or Emulator)

- Config:      sas_url
- Env Var:     RCLONE_AZUREBLOB_SAS_URL
- Type:        string
- Default:     ""

#### --azureblob-use-emulator

Uses local storage emulator if provided as 'true' (leave blank if using real azure storage endpoint)

- Config:      use_emulator
- Env Var:     RCLONE_AZUREBLOB_USE_EMULATOR
- Type:        bool
- Default:     false

### Advanced Options

Here are the advanced options specific to azureblob (Microsoft Azure Blob Storage).

#### --azureblob-endpoint

Endpoint for the service
Leave blank normally.

- Config:      endpoint
- Env Var:     RCLONE_AZUREBLOB_ENDPOINT
- Type:        string
- Default:     ""

#### --azureblob-upload-cutoff

Cutoff for switching to chunked upload (<= 256MB).

- Config:      upload_cutoff
- Env Var:     RCLONE_AZUREBLOB_UPLOAD_CUTOFF
- Type:        SizeSuffix
- Default:     256M

#### --azureblob-chunk-size

Upload chunk size (<= 100MB).

Note that this is stored in memory and there may be up to
"--transfers" chunks stored at once in memory.

- Config:      chunk_size
- Env Var:     RCLONE_AZUREBLOB_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     4M

#### --azureblob-list-chunk

Size of blob list.

This sets the number of blobs requested in each listing chunk. Default
is the maximum, 5000. "List blobs" requests are permitted 2 minutes
per megabyte to complete. If an operation is taking longer than 2
minutes per megabyte on average, it will time out (
[source](https://docs.microsoft.com/en-us/rest/api/storageservices/setting-timeouts-for-blob-service-operations#exceptions-to-default-timeout-interval)
). This can be used to limit the number of blobs items to return, to
avoid the time out.

- Config:      list_chunk
- Env Var:     RCLONE_AZUREBLOB_LIST_CHUNK
- Type:        int
- Default:     5000

#### --azureblob-access-tier

Access tier of blob: hot, cool or archive.

Archived blobs can be restored by setting access tier to hot or
cool. Leave blank if you intend to use default access tier, which is
set at account level

If there is no "access tier" specified, rclone doesn't apply any tier.
rclone performs "Set Tier" operation on blobs while uploading, if objects
are not modified, specifying "access tier" to new one will have no effect.
If blobs are in "archive tier" at remote, trying to perform data transfer
operations from remote will not be allowed. User should first restore by
tiering blob to "Hot" or "Cool".

- Config:      access_tier
- Env Var:     RCLONE_AZUREBLOB_ACCESS_TIER
- Type:        string
- Default:     ""

#### --azureblob-disable-checksum

Don't store MD5 checksum with object metadata.

Normally rclone will calculate the MD5 checksum of the input before
uploading it so it can add it to metadata on the object. This is great
for data integrity checking but can cause long delays for large files
to start uploading.

- Config:      disable_checksum
- Env Var:     RCLONE_AZUREBLOB_DISABLE_CHECKSUM
- Type:        bool
- Default:     false

#### --azureblob-memory-pool-flush-time

How often internal memory buffer pools will be flushed.
Uploads which requires additional buffers (f.e multipart) will use memory pool for allocations.
This option controls how often unused buffers will be removed from the pool.

- Config:      memory_pool_flush_time
- Env Var:     RCLONE_AZUREBLOB_MEMORY_POOL_FLUSH_TIME
- Type:        Duration
- Default:     1m0s

#### --azureblob-memory-pool-use-mmap

Whether to use mmap buffers in internal memory pool.

- Config:      memory_pool_use_mmap
- Env Var:     RCLONE_AZUREBLOB_MEMORY_POOL_USE_MMAP
- Type:        bool
- Default:     false

#### --azureblob-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_AZUREBLOB_ENCODING
- Type:        MultiEncoder
- Default:     Slash,BackSlash,Del,Ctl,RightPeriod,InvalidUtf8



### Limitations ###

MD5 sums are only uploaded with chunked files if the source has an MD5
sum.  This will always be the case for a local to azure copy.

### Azure Storage Emulator Support ###
You can test rclone with storage emulator locally, to do this make sure azure storage emulator
installed locally and set up a new remote with `rclone config` follow instructions described in
introduction, set `use_emulator` config as `true`, you do not need to provide default account name
or key if using emulator.

 Microsoft OneDrive
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

The initial setup for OneDrive involves getting a token from
Microsoft which you need to do in your browser.  `rclone config` walks
you through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Microsoft OneDrive
   \ "onedrive"
[snip]
Storage> onedrive
Microsoft App Client Id
Leave blank normally.
Enter a string value. Press Enter for the default ("").
client_id>
Microsoft App Client Secret
Leave blank normally.
Enter a string value. Press Enter for the default ("").
client_secret>
Edit advanced config? (y/n)
y) Yes
n) No
y/n> n
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
Choose a number from below, or type in an existing value
 1 / OneDrive Personal or Business
   \ "onedrive"
 2 / Sharepoint site
   \ "sharepoint"
 3 / Type in driveID
   \ "driveid"
 4 / Type in SiteID
   \ "siteid"
 5 / Search a Sharepoint site
   \ "search"
Your choice> 1
Found 1 drives, please select the one you want to use:
0: OneDrive (business) id=b!Eqwertyuiopasdfghjklzxcvbnm-7mnbvcxzlkjhgfdsapoiuytrewqk
Chose drive to use:> 0
Found drive 'root' of type 'business', URL: https://org-my.sharepoint.com/personal/you/Documents
Is that okay?
y) Yes
n) No
y/n> y
--------------------
[remote]
type = onedrive
token = {"access_token":"youraccesstoken","token_type":"Bearer","refresh_token":"yourrefreshtoken","expiry":"2018-08-26T22:39:52.486512262+08:00"}
drive_id = b!Eqwertyuiopasdfghjklzxcvbnm-7mnbvcxzlkjhgfdsapoiuytrewqk
drive_type = business
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](https://rclone.org/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Microsoft. This only runs from the moment it
opens your browser to the moment you get back the verification
code.  This is on `http://127.0.0.1:53682/` and this it may require
you to unblock it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List directories in top level of your OneDrive

    rclone lsd remote:

List all the files in your OneDrive

    rclone ls remote:

To copy a local directory to an OneDrive directory called backup

    rclone copy /home/source remote:backup

### Getting your own Client ID and Key ###

You can use your own Client ID if the default (`client_id` left blank)
one doesn't work for you or you see lots of throttling. The default
Client ID and Key is shared by all rclone users when performing
requests.

If you are having problems with them (E.g., seeing a lot of throttling), you can get your own
Client ID and Key by following the steps below:

1. Open https://portal.azure.com/#blade/Microsoft_AAD_RegisteredApps/ApplicationsListBlade, then click `New registration`.
2. Enter a name for your app, choose account type `Accounts in any organizational directory (Any Azure AD directory - Multitenant) and personal Microsoft accounts (e.g. Skype, Xbox)`, select `Web` in `Redirect URI` Enter `http://localhost:53682/` and click Register. Copy and keep the `Application (client) ID` under the app name for later use.
3. Under `manage` select `Certificates & secrets`, click `New client secret`. Copy and keep that secret for later use.
4. Under `manage` select `API permissions`, click `Add a permission` and select `Microsoft Graph` then select `delegated permissions`.
5. Search and select the following permissions: `Files.Read`, `Files.ReadWrite`, `Files.Read.All`, `Files.ReadWrite.All`, `offline_access`, `User.Read`. Once selected click `Add permissions` at the bottom.

Now the application is complete. Run `rclone config` to create or edit a OneDrive remote.
Supply the app ID and password as Client ID and Secret, respectively. rclone will walk you through the remaining steps.

### Modification time and hashes ###

OneDrive allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.

OneDrive personal supports SHA1 type hashes. OneDrive for business and
Sharepoint Server support
[QuickXorHash](https://docs.microsoft.com/en-us/onedrive/developer/code-snippets/quickxorhash).

For all types of OneDrive you can use the `--checksum` flag.

### Restricted filename characters ###

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| "         | 0x22  | ＂          |
| *         | 0x2A  | ＊          |
| :         | 0x3A  | ：          |
| <         | 0x3C  | ＜          |
| >         | 0x3E  | ＞          |
| ?         | 0x3F  | ？          |
| \         | 0x5C  | ＼          |
| \|        | 0x7C  | ｜          |
| #         | 0x23  | ＃          |
| %         | 0x25  | ％          |

File names can also not end with the following characters.
These only get replaced if they are the last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |
| .         | 0x2E  | ．          |

File names can also not begin with the following characters.
These only get replaced if they are the first character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |
| ~         | 0x7E  | ～          |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Deleting files ###

Any files you delete with rclone will end up in the trash.  Microsoft
doesn't provide an API to permanently delete files, nor to empty the
trash, so you will have to do that with one of Microsoft's apps or via
the OneDrive website.


### Standard Options

Here are the standard options specific to onedrive (Microsoft OneDrive).

#### --onedrive-client-id

Microsoft App Client Id
Leave blank normally.

- Config:      client_id
- Env Var:     RCLONE_ONEDRIVE_CLIENT_ID
- Type:        string
- Default:     ""

#### --onedrive-client-secret

Microsoft App Client Secret
Leave blank normally.

- Config:      client_secret
- Env Var:     RCLONE_ONEDRIVE_CLIENT_SECRET
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to onedrive (Microsoft OneDrive).

#### --onedrive-chunk-size

Chunk size to upload files with - must be multiple of 320k (327,680 bytes).

Above this size files will be chunked - must be multiple of 320k (327,680 bytes) and
should not exceed 250M (262,144,000 bytes) else you may encounter \"Microsoft.SharePoint.Client.InvalidClientQueryException: The request message is too big.\"
Note that the chunks will be buffered into memory.

- Config:      chunk_size
- Env Var:     RCLONE_ONEDRIVE_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     10M

#### --onedrive-drive-id

The ID of the drive to use

- Config:      drive_id
- Env Var:     RCLONE_ONEDRIVE_DRIVE_ID
- Type:        string
- Default:     ""

#### --onedrive-drive-type

The type of the drive ( personal | business | documentLibrary )

- Config:      drive_type
- Env Var:     RCLONE_ONEDRIVE_DRIVE_TYPE
- Type:        string
- Default:     ""

#### --onedrive-expose-onenote-files

Set to make OneNote files show up in directory listings.

By default rclone will hide OneNote files in directory listings because
operations like "Open" and "Update" won't work on them.  But this
behaviour may also prevent you from deleting them.  If you want to
delete OneNote files or otherwise want them to show up in directory
listing, set this option.

- Config:      expose_onenote_files
- Env Var:     RCLONE_ONEDRIVE_EXPOSE_ONENOTE_FILES
- Type:        bool
- Default:     false

#### --onedrive-server-side-across-configs

Allow server side operations (eg copy) to work across different onedrive configs.

This can be useful if you wish to do a server side copy between two
different Onedrives.  Note that this isn't enabled by default
because it isn't easy to tell if it will work between any two
configurations.

- Config:      server_side_across_configs
- Env Var:     RCLONE_ONEDRIVE_SERVER_SIDE_ACROSS_CONFIGS
- Type:        bool
- Default:     false

#### --onedrive-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_ONEDRIVE_ENCODING
- Type:        MultiEncoder
- Default:     Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,Hash,Percent,BackSlash,Del,Ctl,LeftSpace,LeftTilde,RightSpace,RightPeriod,InvalidUtf8,Dot



### Limitations ###

#### Naming ####

Note that OneDrive is case insensitive so you can't have a
file called "Hello.doc" and one called "hello.doc".

There are quite a few characters that can't be in OneDrive file
names.  These can't occur on Windows platforms, but on non-Windows
platforms they are common.  Rclone will map these names to and from an
identical looking unicode equivalent.  For example if a file has a `?`
in it will be mapped to `？` instead.

#### File sizes ####

The largest allowed file sizes are 15GB for OneDrive for Business and 100GB for OneDrive Personal (Updated 19 May 2020).
Source:
https://support.office.com/en-us/article/upload-photos-and-files-to-onedrive-b00ad3fe-6643-4b16-9212-de00ef02b586

#### Path length ####

The entire path, including the file name, must contain fewer than 400 characters for OneDrive, OneDrive for Business and SharePoint Online. If you are encrypting file and folder names with rclone, you may want to pay attention to this limitation because the encrypted names are typically longer than the original ones.

#### Number of files ####

OneDrive seems to be OK with at least 50,000 files in a folder, but at
100,000 rclone will get errors listing the directory like `couldn’t
list files: UnknownError:`.  See
[#2707](https://github.com/rclone/rclone/issues/2707) for more info.

An official document about the limitations for different types of OneDrive can be found [here](https://support.office.com/en-us/article/invalid-file-names-and-file-types-in-onedrive-onedrive-for-business-and-sharepoint-64883a5d-228e-48f5-b3d2-eb39e07630fa).

### Versioning issue ###

Every change in OneDrive causes the service to create a new version.
This counts against a users quota.
For example changing the modification time of a file creates a second
version, so the file is using twice the space.

The `copy` is the only rclone command affected by this as we copy
the file and then afterwards set the modification time to match the
source file.

**Note**: Starting October 2018, users will no longer be able to disable versioning by default. This is because Microsoft has brought an [update](https://techcommunity.microsoft.com/t5/Microsoft-OneDrive-Blog/New-Updates-to-OneDrive-and-SharePoint-Team-Site-Versioning/ba-p/204390) to the mechanism. To change this new default setting, a PowerShell command is required to be run by a SharePoint admin. If you are an admin, you can run these commands in PowerShell to change that setting:

1. `Install-Module -Name Microsoft.Online.SharePoint.PowerShell` (in case you haven't installed this already)
1. `Import-Module Microsoft.Online.SharePoint.PowerShell -DisableNameChecking`
1. `Connect-SPOService -Url https://YOURSITE-admin.sharepoint.com -Credential YOU@YOURSITE.COM` (replacing `YOURSITE`, `YOU`, `YOURSITE.COM` with the actual values; this will prompt for your credentials)
1. `Set-SPOTenant -EnableMinimumVersionRequirement $False`
1. `Disconnect-SPOService` (to disconnect from the server)

*Below are the steps for normal users to disable versioning. If you don't see the "No Versioning" option, make sure the above requirements are met.*  

User [Weropol](https://github.com/Weropol) has found a method to disable
versioning on OneDrive

1. Open the settings menu by clicking on the gear symbol at the top of the OneDrive Business page.
2. Click Site settings.
3. Once on the Site settings page, navigate to Site Administration > Site libraries and lists.
4. Click Customize "Documents".
5. Click General Settings > Versioning Settings.
6. Under Document Version History select the option No versioning.
Note: This will disable the creation of new file versions, but will not remove any previous versions. Your documents are safe.
7. Apply the changes by clicking OK.
8. Use rclone to upload or modify files. (I also use the --no-update-modtime flag)
9. Restore the versioning settings after using rclone. (Optional)

### Troubleshooting ###

#### Unexpected file size/hash differences on Sharepoint ####

It is a
[known](https://github.com/OneDrive/onedrive-api-docs/issues/935#issuecomment-441741631)
issue that Sharepoint (not OneDrive or OneDrive for Business) silently modifies
uploaded files, mainly Office files (.docx, .xlsx, etc.), causing file size and
hash checks to fail. To use rclone with such affected files on Sharepoint, you
may disable these checks with the following command line arguments:

```
--ignore-checksum --ignore-size
```

#### Replacing/deleting existing files on Sharepoint gets "item not found" ####

It is a [known](https://github.com/OneDrive/onedrive-api-docs/issues/1068) issue
that Sharepoint (not OneDrive or OneDrive for Business) may return "item not
found" errors when users try to replace or delete uploaded files; this seems to
mainly affect Office files (.docx, .xlsx, etc.). As a workaround, you may use
the `--backup-dir <BACKUP_DIR>` command line argument so rclone moves the
files to be replaced/deleted into a given backup directory (instead of directly
replacing/deleting them). For example, to instruct rclone to move the files into
the directory `rclone-backup-dir` on backend `mysharepoint`, you may use:

```
--backup-dir mysharepoint:rclone-backup-dir
```

#### access\_denied (AADSTS65005) ####

```
Error: access_denied
Code: AADSTS65005
Description: Using application 'rclone' is currently not supported for your organization [YOUR_ORGANIZATION] because it is in an unmanaged state. An administrator needs to claim ownership of the company by DNS validation of [YOUR_ORGANIZATION] before the application rclone can be provisioned.
```

This means that rclone can't use the OneDrive for Business API with your account. You can't do much about it, maybe write an email to your admins.

However, there are other ways to interact with your OneDrive account. Have a look at the webdav backend: https://rclone.org/webdav/#sharepoint

#### invalid\_grant (AADSTS50076) ####

```
Error: invalid_grant
Code: AADSTS50076
Description: Due to a configuration change made by your administrator, or because you moved to a new location, you must use multi-factor authentication to access '...'.
```

If you see the error above after enabling multi-factor authentication for your account, you can fix it by refreshing your OAuth refresh token. To do that, run `rclone config`, and choose to edit your OneDrive backend. Then, you don't need to actually make any changes until you reach this question: `Already have a token - refresh?`. For this question, answer `y` and go through the process to refresh your token, just like the first time the backend is configured. After this, rclone should work again for this backend.

 OpenDrive
------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
n) New remote
d) Delete remote
q) Quit config
e/n/d/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / OpenDrive
   \ "opendrive"
[snip]
Storage> opendrive
Username
username>
Password
y) Yes type in my own password
g) Generate random password
y/g> y
Enter the password:
password:
Confirm the password:
password:
--------------------
[remote]
username =
password = *** ENCRYPTED ***
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

List directories in top level of your OpenDrive

    rclone lsd remote:

List all the files in your OpenDrive

    rclone ls remote:

To copy a local directory to an OpenDrive directory called backup

    rclone copy /home/source remote:backup

### Modified time and MD5SUMs ###

OpenDrive allows modification times to be set on objects accurate to 1
second. These will be used to detect whether objects need syncing or
not.

#### Restricted filename characters

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| /         | 0x2F  | ／          |
| "         | 0x22  | ＂          |
| *         | 0x2A  | ＊          |
| :         | 0x3A  | ：          |
| <         | 0x3C  | ＜          |
| >         | 0x3E  | ＞          |
| ?         | 0x3F  | ？          |
| \         | 0x5C  | ＼          |
| \|        | 0x7C  | ｜          |

File names can also not begin or end with the following characters.
These only get replaced if they are the first or last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |
| HT        | 0x09  | ␉           |
| LF        | 0x0A  | ␊           |
| VT        | 0x0B  | ␋           |
| CR        | 0x0D  | ␍           |


Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.


### Standard Options

Here are the standard options specific to opendrive (OpenDrive).

#### --opendrive-username

Username

- Config:      username
- Env Var:     RCLONE_OPENDRIVE_USERNAME
- Type:        string
- Default:     ""

#### --opendrive-password

Password.

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      password
- Env Var:     RCLONE_OPENDRIVE_PASSWORD
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to opendrive (OpenDrive).

#### --opendrive-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_OPENDRIVE_ENCODING
- Type:        MultiEncoder
- Default:     Slash,LtGt,DoubleQuote,Colon,Question,Asterisk,Pipe,BackSlash,LeftSpace,LeftCrLfHtVt,RightSpace,RightCrLfHtVt,InvalidUtf8,Dot

#### --opendrive-chunk-size

Files will be uploaded in chunks this size.

Note that these chunks are buffered in memory so increasing them will
increase memory use.

- Config:      chunk_size
- Env Var:     RCLONE_OPENDRIVE_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     10M



### Limitations ###

Note that OpenDrive is case insensitive so you can't have a
file called "Hello.doc" and one called "hello.doc".

There are quite a few characters that can't be in OpenDrive file
names.  These can't occur on Windows platforms, but on non-Windows
platforms they are common.  Rclone will map these names to and from an
identical looking unicode equivalent.  For example if a file has a `?`
in it will be mapped to `？` instead.

 QingStor
---------------------------------------

Paths are specified as `remote:bucket` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, eg `remote:bucket/path/to/dir`.

Here is an example of making an QingStor configuration.  First run

    rclone config

This will guide you through an interactive setup process.

```
No remotes found - make a new one
n) New remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
n/r/c/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / QingStor Object Storage
   \ "qingstor"
[snip]
Storage> qingstor
Get QingStor credentials from runtime. Only applies if access_key_id and secret_access_key is blank.
Choose a number from below, or type in your own value
 1 / Enter QingStor credentials in the next step
   \ "false"
 2 / Get QingStor credentials from the environment (env vars or IAM)
   \ "true"
env_auth> 1
QingStor Access Key ID - leave blank for anonymous access or runtime credentials.
access_key_id> access_key
QingStor Secret Access Key (password) - leave blank for anonymous access or runtime credentials.
secret_access_key> secret_key
Enter an endpoint URL to connection QingStor API.
Leave blank will use the default value "https://qingstor.com:443"
endpoint>
Zone connect to. Default is "pek3a".
Choose a number from below, or type in your own value
   / The Beijing (China) Three Zone
 1 | Needs location constraint pek3a.
   \ "pek3a"
   / The Shanghai (China) First Zone
 2 | Needs location constraint sh1a.
   \ "sh1a"
zone> 1
Number of connection retry.
Leave blank will use the default value "3".
connection_retries>
Remote config
--------------------
[remote]
env_auth = false
access_key_id = access_key
secret_access_key = secret_key
endpoint =
zone = pek3a
connection_retries =
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `remote` and can now be used like this

See all buckets

    rclone lsd remote:

Make a new bucket

    rclone mkdir remote:bucket

List the contents of a bucket

    rclone ls remote:bucket

Sync `/home/local/directory` to the remote bucket, deleting any excess
files in the bucket.

    rclone sync /home/local/directory remote:bucket

### --fast-list ###

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](https://rclone.org/docs/#fast-list) for more details.

### Multipart uploads ###

rclone supports multipart uploads with QingStor which means that it can
upload files bigger than 5GB. Note that files uploaded with multipart
upload don't have an MD5SUM.

Note that incomplete multipart uploads older than 24 hours can be
removed with `rclone cleanup remote:bucket` just for one bucket
`rclone cleanup remote:` for all buckets. QingStor does not ever
remove incomplete multipart uploads so it may be necessary to run this
from time to time.

### Buckets and Zone ###

With QingStor you can list buckets (`rclone lsd`) using any zone,
but you can only access the content of a bucket from the zone it was
created in.  If you attempt to access a bucket from the wrong zone,
you will get an error, `incorrect zone, the bucket is not in 'XXX'
zone`.

### Authentication ###

There are two ways to supply `rclone` with a set of QingStor
credentials. In order of precedence:

 - Directly in the rclone configuration file (as configured by `rclone config`)
   - set `access_key_id` and `secret_access_key`
 - Runtime configuration:
   - set `env_auth` to `true` in the config file
   - Exporting the following environment variables before running `rclone`
     - Access Key ID: `QS_ACCESS_KEY_ID` or `QS_ACCESS_KEY`
     - Secret Access Key: `QS_SECRET_ACCESS_KEY` or `QS_SECRET_KEY`

### Restricted filename characters

The control characters 0x00-0x1F and / are replaced as in the [default
restricted characters set](https://rclone.org/overview/#restricted-characters).  Note
that 0x7F is not replaced.

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.


### Standard Options

Here are the standard options specific to qingstor (QingCloud Object Storage).

#### --qingstor-env-auth

Get QingStor credentials from runtime. Only applies if access_key_id and secret_access_key is blank.

- Config:      env_auth
- Env Var:     RCLONE_QINGSTOR_ENV_AUTH
- Type:        bool
- Default:     false
- Examples:
    - "false"
        - Enter QingStor credentials in the next step
    - "true"
        - Get QingStor credentials from the environment (env vars or IAM)

#### --qingstor-access-key-id

QingStor Access Key ID
Leave blank for anonymous access or runtime credentials.

- Config:      access_key_id
- Env Var:     RCLONE_QINGSTOR_ACCESS_KEY_ID
- Type:        string
- Default:     ""

#### --qingstor-secret-access-key

QingStor Secret Access Key (password)
Leave blank for anonymous access or runtime credentials.

- Config:      secret_access_key
- Env Var:     RCLONE_QINGSTOR_SECRET_ACCESS_KEY
- Type:        string
- Default:     ""

#### --qingstor-endpoint

Enter an endpoint URL to connection QingStor API.
Leave blank will use the default value "https://qingstor.com:443"

- Config:      endpoint
- Env Var:     RCLONE_QINGSTOR_ENDPOINT
- Type:        string
- Default:     ""

#### --qingstor-zone

Zone to connect to.
Default is "pek3a".

- Config:      zone
- Env Var:     RCLONE_QINGSTOR_ZONE
- Type:        string
- Default:     ""
- Examples:
    - "pek3a"
        - The Beijing (China) Three Zone
        - Needs location constraint pek3a.
    - "sh1a"
        - The Shanghai (China) First Zone
        - Needs location constraint sh1a.
    - "gd2a"
        - The Guangdong (China) Second Zone
        - Needs location constraint gd2a.

### Advanced Options

Here are the advanced options specific to qingstor (QingCloud Object Storage).

#### --qingstor-connection-retries

Number of connection retries.

- Config:      connection_retries
- Env Var:     RCLONE_QINGSTOR_CONNECTION_RETRIES
- Type:        int
- Default:     3

#### --qingstor-upload-cutoff

Cutoff for switching to chunked upload

Any files larger than this will be uploaded in chunks of chunk_size.
The minimum is 0 and the maximum is 5GB.

- Config:      upload_cutoff
- Env Var:     RCLONE_QINGSTOR_UPLOAD_CUTOFF
- Type:        SizeSuffix
- Default:     200M

#### --qingstor-chunk-size

Chunk size to use for uploading.

When uploading files larger than upload_cutoff they will be uploaded
as multipart uploads using this chunk size.

Note that "--qingstor-upload-concurrency" chunks of this size are buffered
in memory per transfer.

If you are transferring large files over high speed links and you have
enough memory, then increasing this will speed up the transfers.

- Config:      chunk_size
- Env Var:     RCLONE_QINGSTOR_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     4M

#### --qingstor-upload-concurrency

Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently.

NB if you set this to > 1 then the checksums of multpart uploads
become corrupted (the uploads themselves are not corrupted though).

If you are uploading small numbers of large file over high speed link
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.

- Config:      upload_concurrency
- Env Var:     RCLONE_QINGSTOR_UPLOAD_CONCURRENCY
- Type:        int
- Default:     1

#### --qingstor-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_QINGSTOR_ENCODING
- Type:        MultiEncoder
- Default:     Slash,Ctl,InvalidUtf8



Swift
----------------------------------------

Swift refers to [OpenStack Object Storage](https://docs.openstack.org/swift/latest/).
Commercial implementations of that being:

  * [Rackspace Cloud Files](https://www.rackspace.com/cloud/files/)
  * [Memset Memstore](https://www.memset.com/cloud/storage/)
  * [OVH Object Storage](https://www.ovh.co.uk/public-cloud/storage/object-storage/)
  * [Oracle Cloud Storage](https://cloud.oracle.com/storage-opc)
  * [IBM Bluemix Cloud ObjectStorage Swift](https://console.bluemix.net/docs/infrastructure/objectstorage-swift/index.html)

Paths are specified as `remote:container` (or `remote:` for the `lsd`
command.)  You may put subdirectories in too, eg `remote:container/path/to/dir`.

Here is an example of making a swift configuration.  First run

    rclone config

This will guide you through an interactive setup process.

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / OpenStack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
   \ "swift"
[snip]
Storage> swift
Get swift credentials from environment variables in standard OpenStack form.
Choose a number from below, or type in your own value
 1 / Enter swift credentials in the next step
   \ "false"
 2 / Get swift credentials from environment vars. Leave other fields blank if using this.
   \ "true"
env_auth> true
User name to log in (OS_USERNAME).
user> 
API key or password (OS_PASSWORD).
key> 
Authentication URL for server (OS_AUTH_URL).
Choose a number from below, or type in your own value
 1 / Rackspace US
   \ "https://auth.api.rackspacecloud.com/v1.0"
 2 / Rackspace UK
   \ "https://lon.auth.api.rackspacecloud.com/v1.0"
 3 / Rackspace v2
   \ "https://identity.api.rackspacecloud.com/v2.0"
 4 / Memset Memstore UK
   \ "https://auth.storage.memset.com/v1.0"
 5 / Memset Memstore UK v2
   \ "https://auth.storage.memset.com/v2.0"
 6 / OVH
   \ "https://auth.cloud.ovh.net/v3"
auth> 
User ID to log in - optional - most swift systems use user and leave this blank (v3 auth) (OS_USER_ID).
user_id> 
User domain - optional (v3 auth) (OS_USER_DOMAIN_NAME)
domain> 
Tenant name - optional for v1 auth, this or tenant_id required otherwise (OS_TENANT_NAME or OS_PROJECT_NAME)
tenant> 
Tenant ID - optional for v1 auth, this or tenant required otherwise (OS_TENANT_ID)
tenant_id> 
Tenant domain - optional (v3 auth) (OS_PROJECT_DOMAIN_NAME)
tenant_domain> 
Region name - optional (OS_REGION_NAME)
region> 
Storage URL - optional (OS_STORAGE_URL)
storage_url> 
Auth Token from alternate authentication - optional (OS_AUTH_TOKEN)
auth_token> 
AuthVersion - optional - set to (1,2,3) if your auth URL has no version (ST_AUTH_VERSION)
auth_version> 
Endpoint type to choose from the service catalogue (OS_ENDPOINT_TYPE)
Choose a number from below, or type in your own value
 1 / Public (default, choose this if not sure)
   \ "public"
 2 / Internal (use internal service net)
   \ "internal"
 3 / Admin
   \ "admin"
endpoint_type> 
Remote config
--------------------
[test]
env_auth = true
user = 
key = 
auth = 
user_id = 
domain = 
tenant = 
tenant_id = 
tenant_domain = 
region = 
storage_url = 
auth_token = 
auth_version = 
endpoint_type = 
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `remote` and can now be used like this

See all containers

    rclone lsd remote:

Make a new container

    rclone mkdir remote:container

List the contents of a container

    rclone ls remote:container

Sync `/home/local/directory` to the remote container, deleting any
excess files in the container.

    rclone sync /home/local/directory remote:container

### Configuration from an OpenStack credentials file ###

An OpenStack credentials file typically looks something something
like this (without the comments)

```
export OS_AUTH_URL=https://a.provider.net/v2.0
export OS_TENANT_ID=ffffffffffffffffffffffffffffffff
export OS_TENANT_NAME="1234567890123456"
export OS_USERNAME="123abc567xy"
echo "Please enter your OpenStack Password: "
read -sr OS_PASSWORD_INPUT
export OS_PASSWORD=$OS_PASSWORD_INPUT
export OS_REGION_NAME="SBG1"
if [ -z "$OS_REGION_NAME" ]; then unset OS_REGION_NAME; fi
```

The config file needs to look something like this where `$OS_USERNAME`
represents the value of the `OS_USERNAME` variable - `123abc567xy` in
the example above.

```
[remote]
type = swift
user = $OS_USERNAME
key = $OS_PASSWORD
auth = $OS_AUTH_URL
tenant = $OS_TENANT_NAME
```

Note that you may (or may not) need to set `region` too - try without first.

### Configuration from the environment ###

If you prefer you can configure rclone to use swift using a standard
set of OpenStack environment variables.

When you run through the config, make sure you choose `true` for
`env_auth` and leave everything else blank.

rclone will then set any empty config parameters from the environment
using standard OpenStack environment variables.  There is [a list of
the
variables](https://godoc.org/github.com/ncw/swift#Connection.ApplyEnvironment)
in the docs for the swift library.

### Using an alternate authentication method ###

If your OpenStack installation uses a non-standard authentication method
that might not be yet supported by rclone or the underlying swift library, 
you can authenticate externally (e.g. calling manually the `openstack` 
commands to get a token). Then, you just need to pass the two 
configuration variables ``auth_token`` and ``storage_url``. 
If they are both provided, the other variables are ignored. rclone will 
not try to authenticate but instead assume it is already authenticated 
and use these two variables to access the OpenStack installation.

#### Using rclone without a config file ####

You can use rclone with swift without a config file, if desired, like
this:

```
source openstack-credentials-file
export RCLONE_CONFIG_MYREMOTE_TYPE=swift
export RCLONE_CONFIG_MYREMOTE_ENV_AUTH=true
rclone lsd myremote:
```

### --fast-list ###

This remote supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](https://rclone.org/docs/#fast-list) for more details.

### --update and --use-server-modtime ###

As noted below, the modified time is stored on metadata on the object. It is
used by default for all operations that require checking the time a file was
last updated. It allows rclone to treat the remote more like a true filesystem,
but it is inefficient because it requires an extra API call to retrieve the
metadata.

For many operations, the time the object was last uploaded to the remote is
sufficient to determine if it is "dirty". By using `--update` along with
`--use-server-modtime`, you can avoid the extra API call and simply upload
files whose local modtime is newer than the time it was last uploaded.


### Standard Options

Here are the standard options specific to swift (OpenStack Swift (Rackspace Cloud Files, Memset Memstore, OVH)).

#### --swift-env-auth

Get swift credentials from environment variables in standard OpenStack form.

- Config:      env_auth
- Env Var:     RCLONE_SWIFT_ENV_AUTH
- Type:        bool
- Default:     false
- Examples:
    - "false"
        - Enter swift credentials in the next step
    - "true"
        - Get swift credentials from environment vars. Leave other fields blank if using this.

#### --swift-user

User name to log in (OS_USERNAME).

- Config:      user
- Env Var:     RCLONE_SWIFT_USER
- Type:        string
- Default:     ""

#### --swift-key

API key or password (OS_PASSWORD).

- Config:      key
- Env Var:     RCLONE_SWIFT_KEY
- Type:        string
- Default:     ""

#### --swift-auth

Authentication URL for server (OS_AUTH_URL).

- Config:      auth
- Env Var:     RCLONE_SWIFT_AUTH
- Type:        string
- Default:     ""
- Examples:
    - "https://auth.api.rackspacecloud.com/v1.0"
        - Rackspace US
    - "https://lon.auth.api.rackspacecloud.com/v1.0"
        - Rackspace UK
    - "https://identity.api.rackspacecloud.com/v2.0"
        - Rackspace v2
    - "https://auth.storage.memset.com/v1.0"
        - Memset Memstore UK
    - "https://auth.storage.memset.com/v2.0"
        - Memset Memstore UK v2
    - "https://auth.cloud.ovh.net/v3"
        - OVH

#### --swift-user-id

User ID to log in - optional - most swift systems use user and leave this blank (v3 auth) (OS_USER_ID).

- Config:      user_id
- Env Var:     RCLONE_SWIFT_USER_ID
- Type:        string
- Default:     ""

#### --swift-domain

User domain - optional (v3 auth) (OS_USER_DOMAIN_NAME)

- Config:      domain
- Env Var:     RCLONE_SWIFT_DOMAIN
- Type:        string
- Default:     ""

#### --swift-tenant

Tenant name - optional for v1 auth, this or tenant_id required otherwise (OS_TENANT_NAME or OS_PROJECT_NAME)

- Config:      tenant
- Env Var:     RCLONE_SWIFT_TENANT
- Type:        string
- Default:     ""

#### --swift-tenant-id

Tenant ID - optional for v1 auth, this or tenant required otherwise (OS_TENANT_ID)

- Config:      tenant_id
- Env Var:     RCLONE_SWIFT_TENANT_ID
- Type:        string
- Default:     ""

#### --swift-tenant-domain

Tenant domain - optional (v3 auth) (OS_PROJECT_DOMAIN_NAME)

- Config:      tenant_domain
- Env Var:     RCLONE_SWIFT_TENANT_DOMAIN
- Type:        string
- Default:     ""

#### --swift-region

Region name - optional (OS_REGION_NAME)

- Config:      region
- Env Var:     RCLONE_SWIFT_REGION
- Type:        string
- Default:     ""

#### --swift-storage-url

Storage URL - optional (OS_STORAGE_URL)

- Config:      storage_url
- Env Var:     RCLONE_SWIFT_STORAGE_URL
- Type:        string
- Default:     ""

#### --swift-auth-token

Auth Token from alternate authentication - optional (OS_AUTH_TOKEN)

- Config:      auth_token
- Env Var:     RCLONE_SWIFT_AUTH_TOKEN
- Type:        string
- Default:     ""

#### --swift-application-credential-id

Application Credential ID (OS_APPLICATION_CREDENTIAL_ID)

- Config:      application_credential_id
- Env Var:     RCLONE_SWIFT_APPLICATION_CREDENTIAL_ID
- Type:        string
- Default:     ""

#### --swift-application-credential-name

Application Credential Name (OS_APPLICATION_CREDENTIAL_NAME)

- Config:      application_credential_name
- Env Var:     RCLONE_SWIFT_APPLICATION_CREDENTIAL_NAME
- Type:        string
- Default:     ""

#### --swift-application-credential-secret

Application Credential Secret (OS_APPLICATION_CREDENTIAL_SECRET)

- Config:      application_credential_secret
- Env Var:     RCLONE_SWIFT_APPLICATION_CREDENTIAL_SECRET
- Type:        string
- Default:     ""

#### --swift-auth-version

AuthVersion - optional - set to (1,2,3) if your auth URL has no version (ST_AUTH_VERSION)

- Config:      auth_version
- Env Var:     RCLONE_SWIFT_AUTH_VERSION
- Type:        int
- Default:     0

#### --swift-endpoint-type

Endpoint type to choose from the service catalogue (OS_ENDPOINT_TYPE)

- Config:      endpoint_type
- Env Var:     RCLONE_SWIFT_ENDPOINT_TYPE
- Type:        string
- Default:     "public"
- Examples:
    - "public"
        - Public (default, choose this if not sure)
    - "internal"
        - Internal (use internal service net)
    - "admin"
        - Admin

#### --swift-storage-policy

The storage policy to use when creating a new container

This applies the specified storage policy when creating a new
container. The policy cannot be changed afterwards. The allowed
configuration values and their meaning depend on your Swift storage
provider.

- Config:      storage_policy
- Env Var:     RCLONE_SWIFT_STORAGE_POLICY
- Type:        string
- Default:     ""
- Examples:
    - ""
        - Default
    - "pcs"
        - OVH Public Cloud Storage
    - "pca"
        - OVH Public Cloud Archive

### Advanced Options

Here are the advanced options specific to swift (OpenStack Swift (Rackspace Cloud Files, Memset Memstore, OVH)).

#### --swift-chunk-size

Above this size files will be chunked into a _segments container.

Above this size files will be chunked into a _segments container.  The
default for this is 5GB which is its maximum value.

- Config:      chunk_size
- Env Var:     RCLONE_SWIFT_CHUNK_SIZE
- Type:        SizeSuffix
- Default:     5G

#### --swift-no-chunk

Don't chunk files during streaming upload.

When doing streaming uploads (eg using rcat or mount) setting this
flag will cause the swift backend to not upload chunked files.

This will limit the maximum upload size to 5GB. However non chunked
files are easier to deal with and have an MD5SUM.

Rclone will still chunk files bigger than chunk_size when doing normal
copy operations.

- Config:      no_chunk
- Env Var:     RCLONE_SWIFT_NO_CHUNK
- Type:        bool
- Default:     false

#### --swift-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_SWIFT_ENCODING
- Type:        MultiEncoder
- Default:     Slash,InvalidUtf8



### Modified time ###

The modified time is stored as metadata on the object as
`X-Object-Meta-Mtime` as floating point since the epoch accurate to 1
ns.

This is a de facto standard (used in the official python-swiftclient
amongst others) for storing the modification time for an object.

### Restricted filename characters

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| /         | 0x2F  | ／          |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Limitations ###

The Swift API doesn't return a correct MD5SUM for segmented files
(Dynamic or Static Large Objects) so rclone won't check or use the
MD5SUM for these.

### Troubleshooting ###

#### Rclone gives Failed to create file system for "remote:": Bad Request ####

Due to an oddity of the underlying swift library, it gives a "Bad
Request" error rather than a more sensible error when the
authentication fails for Swift.

So this most likely means your username / password is wrong.  You can
investigate further with the `--dump-bodies` flag.

This may also be caused by specifying the region when you shouldn't
have (eg OVH).

#### Rclone gives Failed to create file system: Response didn't have storage url and auth token ####

This is most likely caused by forgetting to specify your tenant when
setting up a swift remote.

 pCloud
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

The initial setup for pCloud involves getting a token from pCloud which you
need to do in your browser.  `rclone config` walks you through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Pcloud
   \ "pcloud"
[snip]
Storage> pcloud
Pcloud App Client Id - leave blank normally.
client_id> 
Pcloud App Client Secret - leave blank normally.
client_secret> 
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
client_id = 
client_secret = 
token = {"access_token":"XXX","token_type":"bearer","expiry":"0001-01-01T00:00:00Z"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](https://rclone.org/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from pCloud. This only runs from the moment it opens
your browser to the moment you get back the verification code.  This
is on `http://127.0.0.1:53682/` and this it may require you to unblock
it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List directories in top level of your pCloud

    rclone lsd remote:

List all the files in your pCloud

    rclone ls remote:

To copy a local directory to an pCloud directory called backup

    rclone copy /home/source remote:backup

### Modified time and hashes ###

pCloud allows modification times to be set on objects accurate to 1
second.  These will be used to detect whether objects need syncing or
not.  In order to set a Modification time pCloud requires the object
be re-uploaded.

pCloud supports MD5 and SHA1 type hashes, so you can use the
`--checksum` flag.

#### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| \         | 0x5C  | ＼          |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Deleting files ###

Deleted files will be moved to the trash.  Your subscription level
will determine how long items stay in the trash.  `rclone cleanup` can
be used to empty the trash.

### Root folder ID ###

You can set the `root_folder_id` for rclone.  This is the directory
(identified by its `Folder ID`) that rclone considers to be the root
of your pCloud drive.

Normally you will leave this blank and rclone will determine the
correct root to use itself.

However you can set this to restrict rclone to a specific folder
hierarchy.

In order to do this you will have to find the `Folder ID` of the
directory you wish rclone to display.  This will be the `folder` field
of the URL when you open the relevant folder in the pCloud web
interface.

So if the folder you want rclone to use has a URL which looks like
`https://my.pcloud.com/#page=filemanager&folder=5xxxxxxxx8&tpl=foldergrid`
in the browser, then you use `5xxxxxxxx8` as
the `root_folder_id` in the config.


### Standard Options

Here are the standard options specific to pcloud (Pcloud).

#### --pcloud-client-id

Pcloud App Client Id
Leave blank normally.

- Config:      client_id
- Env Var:     RCLONE_PCLOUD_CLIENT_ID
- Type:        string
- Default:     ""

#### --pcloud-client-secret

Pcloud App Client Secret
Leave blank normally.

- Config:      client_secret
- Env Var:     RCLONE_PCLOUD_CLIENT_SECRET
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to pcloud (Pcloud).

#### --pcloud-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_PCLOUD_ENCODING
- Type:        MultiEncoder
- Default:     Slash,BackSlash,Del,Ctl,InvalidUtf8,Dot

#### --pcloud-root-folder-id

Fill in for rclone to use a non root folder as its starting point.

- Config:      root_folder_id
- Env Var:     RCLONE_PCLOUD_ROOT_FOLDER_ID
- Type:        string
- Default:     "d0"



 premiumize.me
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

The initial setup for [premiumize.me](https://premiumize.me/) involves getting a token from premiumize.me which you
need to do in your browser.  `rclone config` walks you through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / premiumize.me
   \ "premiumizeme"
[snip]
Storage> premiumizeme
** See help for premiumizeme backend at: https://rclone.org/premiumizeme/ **

Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
type = premiumizeme
token = {"access_token":"XXX","token_type":"Bearer","refresh_token":"XXX","expiry":"2029-08-07T18:44:15.548915378+01:00"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> 
```

See the [remote setup docs](https://rclone.org/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from premiumize.me. This only runs from the moment it opens
your browser to the moment you get back the verification code.  This
is on `http://127.0.0.1:53682/` and this it may require you to unblock
it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

List directories in top level of your premiumize.me

    rclone lsd remote:

List all the files in your premiumize.me

    rclone ls remote:

To copy a local directory to an premiumize.me directory called backup

    rclone copy /home/source remote:backup

### Modified time and hashes ###

premiumize.me does not support modification times or hashes, therefore
syncing will default to `--size-only` checking.  Note that using
`--update` will work.

#### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| \         | 0x5C  | ＼           |
| "         | 0x22  | ＂           |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.


### Standard Options

Here are the standard options specific to premiumizeme (premiumize.me).

#### --premiumizeme-api-key

API Key.

This is not normally used - use oauth instead.


- Config:      api_key
- Env Var:     RCLONE_PREMIUMIZEME_API_KEY
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to premiumizeme (premiumize.me).

#### --premiumizeme-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_PREMIUMIZEME_ENCODING
- Type:        MultiEncoder
- Default:     Slash,DoubleQuote,BackSlash,Del,Ctl,InvalidUtf8,Dot



### Limitations ###

Note that premiumize.me is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

premiumize.me file names can't have the `\` or `"` characters in.
rclone maps these to and from an identical looking unicode equivalents
`＼` and `＂`

premiumize.me only supports filenames up to 255 characters in length.

 put.io
---------------------------------

Paths are specified as `remote:path`

put.io paths may be as deep as required, eg
`remote:directory/subdirectory`.

The initial setup for put.io involves getting a token from put.io
which you need to do in your browser.  `rclone config` walks you
through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> putio
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Put.io
   \ "putio"
[snip]
Storage> putio
** See help for putio backend at: https://rclone.org/putio/ **

Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[putio]
type = putio
token = {"access_token":"XXXXXXXX","expiry":"0001-01-01T00:00:00Z"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
Current remotes:

Name                 Type
====                 ====
putio                putio

e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> q
```

Note that rclone runs a webserver on your local machine to collect the
token as returned from Google if you use auto config mode. This only
runs from the moment it opens your browser to the moment you get back
the verification code.  This is on `http://127.0.0.1:53682/` and this
it may require you to unblock it temporarily if you are running a host
firewall, or use manual mode.

You can then use it like this,

List directories in top level of your put.io

    rclone lsd remote:

List all the files in your put.io

    rclone ls remote:

To copy a local directory to a put.io directory called backup

    rclone copy /home/source remote:backup

#### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| \         | 0x5C  | ＼           |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.


### Advanced Options

Here are the advanced options specific to putio (Put.io).

#### --putio-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_PUTIO_ENCODING
- Type:        MultiEncoder
- Default:     Slash,BackSlash,Del,Ctl,InvalidUtf8,Dot



Seafile
----------------------------------------

This is a backend for the [Seafile](https://www.seafile.com/) storage service:
- It works with both the free community edition or the professional edition.
- Seafile versions 6.x and 7.x are all supported.
- Encrypted libraries are also supported.
- It supports 2FA enabled users

### Root mode vs Library mode ###

There are two distinct modes you can setup your remote:
- you point your remote to the **root of the server**, meaning you don't specify a library during the configuration:
Paths are specified as `remote:library`. You may put subdirectories in too, eg `remote:library/path/to/dir`.
- you point your remote to a specific library during the configuration:
Paths are specified as `remote:path/to/dir`. **This is the recommended mode when using encrypted libraries**. (_This mode is possibly slightly faster than the root mode_)

### Configuration in root mode ###

Here is an example of making a seafile configuration for a user with **no** two-factor authentication.  First run

    rclone config

This will guide you through an interactive setup process. To authenticate
you will need the URL of your server, your email (or username) and your password.

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> seafile
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Seafile
   \ "seafile"
[snip]
Storage> seafile
** See help for seafile backend at: https://rclone.org/seafile/ **

URL of seafile host to connect to
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / Connect to cloud.seafile.com
   \ "https://cloud.seafile.com/"
url> http://my.seafile.server/
User name (usually email address)
Enter a string value. Press Enter for the default ("").
user> me@example.com
Password
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank (default)
y/g> y
Enter the password:
password:
Confirm the password:
password:
Two-factor authentication ('true' if the account has 2FA enabled)
Enter a boolean value (true or false). Press Enter for the default ("false").
2fa> false
Name of the library. Leave blank to access all non-encrypted libraries.
Enter a string value. Press Enter for the default ("").
library>
Library password (for encrypted libraries only). Leave blank if you pass it through the command line.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank (default)
y/g/n> n
Edit advanced config? (y/n)
y) Yes
n) No (default)
y/n> n
Remote config
Two-factor authentication is not enabled on this account.
--------------------
[seafile]
type = seafile
url = http://my.seafile.server/
user = me@example.com
pass = *** ENCRYPTED ***
2fa = false
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `seafile`. It's pointing to the root of your seafile server and can now be used like this:

See all libraries

    rclone lsd seafile:

Create a new library

    rclone mkdir seafile:library

List the contents of a library

    rclone ls seafile:library

Sync `/home/local/directory` to the remote library, deleting any
excess files in the library.

    rclone sync /home/local/directory seafile:library

### Configuration in library mode ###

Here's an example of a configuration in library mode with a user that has the two-factor authentication enabled. Your 2FA code will be asked at the end of the configuration, and will attempt to authenticate you:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> seafile
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Seafile
   \ "seafile"
[snip]
Storage> seafile
** See help for seafile backend at: https://rclone.org/seafile/ **

URL of seafile host to connect to
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
 1 / Connect to cloud.seafile.com
   \ "https://cloud.seafile.com/"
url> http://my.seafile.server/
User name (usually email address)
Enter a string value. Press Enter for the default ("").
user> me@example.com
Password
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank (default)
y/g> y
Enter the password:
password:
Confirm the password:
password:
Two-factor authentication ('true' if the account has 2FA enabled)
Enter a boolean value (true or false). Press Enter for the default ("false").
2fa> true
Name of the library. Leave blank to access all non-encrypted libraries.
Enter a string value. Press Enter for the default ("").
library> My Library
Library password (for encrypted libraries only). Leave blank if you pass it through the command line.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank (default)
y/g/n> n
Edit advanced config? (y/n)
y) Yes
n) No (default)
y/n> n
Remote config
Two-factor authentication: please enter your 2FA code
2fa code> 123456
Authenticating...
Success!
--------------------
[seafile]
type = seafile
url = http://my.seafile.server/
user = me@example.com
pass = 
2fa = true
library = My Library
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

You'll notice your password is blank in the configuration. It's because we only need the password to authenticate you once.

You specified `My Library` during the configuration. The root of the remote is pointing at the
root of the library `My Library`:

See all files in the library:

    rclone lsd seafile:

Create a new directory inside the library

    rclone mkdir seafile:directory

List the contents of a directory

    rclone ls seafile:directory

Sync `/home/local/directory` to the remote library, deleting any
excess files in the library.

    rclone sync /home/local/directory seafile:


### --fast-list ###

Seafile version 7+ supports `--fast-list` which allows you to use fewer
transactions in exchange for more memory. See the [rclone
docs](https://rclone.org/docs/#fast-list) for more details.
Please note this is not supported on seafile server version 6.x


#### Restricted filename characters

In addition to the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
the following characters are also replaced:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| /         | 0x2F  | ／          |
| "         | 0x22  | ＂          |
| \         | 0x5C  | ＼           |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Seafile and rclone link ###

Rclone supports generating share links for non-encrypted libraries only.
They can either be for a file or a directory:

```
rclone link seafile:seafile-tutorial.doc
http://my.seafile.server/f/fdcd8a2f93f84b8b90f4/

```

or if run on a directory you will get:

```
rclone link seafile:dir
http://my.seafile.server/d/9ea2455f6f55478bbb0d/
```

Please note a share link is unique for each file or directory. If you run a link command on a file/dir
that has already been shared, you will get the exact same link.

### Compatibility ###

It has been actively tested using the [seafile docker image](https://github.com/haiwen/seafile-docker) of these versions:
- 6.3.4 community edition
- 7.0.5 community edition
- 7.1.3 community edition

Versions below 6.0 are not supported.
Versions between 6.0 and 6.3 haven't been tested and might not work properly.


### Standard Options

Here are the standard options specific to seafile (seafile).

#### --seafile-url

URL of seafile host to connect to

- Config:      url
- Env Var:     RCLONE_SEAFILE_URL
- Type:        string
- Default:     ""
- Examples:
    - "https://cloud.seafile.com/"
        - Connect to cloud.seafile.com

#### --seafile-user

User name (usually email address)

- Config:      user
- Env Var:     RCLONE_SEAFILE_USER
- Type:        string
- Default:     ""

#### --seafile-pass

Password

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      pass
- Env Var:     RCLONE_SEAFILE_PASS
- Type:        string
- Default:     ""

#### --seafile-2fa

Two-factor authentication ('true' if the account has 2FA enabled)

- Config:      2fa
- Env Var:     RCLONE_SEAFILE_2FA
- Type:        bool
- Default:     false

#### --seafile-library

Name of the library. Leave blank to access all non-encrypted libraries.

- Config:      library
- Env Var:     RCLONE_SEAFILE_LIBRARY
- Type:        string
- Default:     ""

#### --seafile-library-key

Library password (for encrypted libraries only). Leave blank if you pass it through the command line.

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      library_key
- Env Var:     RCLONE_SEAFILE_LIBRARY_KEY
- Type:        string
- Default:     ""

#### --seafile-auth-token

Authentication token

- Config:      auth_token
- Env Var:     RCLONE_SEAFILE_AUTH_TOKEN
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to seafile (seafile).

#### --seafile-create-library

Should rclone create a library if it doesn't exist

- Config:      create_library
- Env Var:     RCLONE_SEAFILE_CREATE_LIBRARY
- Type:        bool
- Default:     false

#### --seafile-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_SEAFILE_ENCODING
- Type:        MultiEncoder
- Default:     Slash,DoubleQuote,BackSlash,Ctl,InvalidUtf8



 SFTP
----------------------------------------

SFTP is the [Secure (or SSH) File Transfer
Protocol](https://en.wikipedia.org/wiki/SSH_File_Transfer_Protocol).

The SFTP backend can be used with a number of different providers:


- C14
- rsync.net


SFTP runs over SSH v2 and is installed as standard with most modern
SSH installations.

Paths are specified as `remote:path`. If the path does not begin with
a `/` it is relative to the home directory of the user.  An empty path
`remote:` refers to the user's home directory.

"Note that some SFTP servers will need the leading / - Synology is a
good example of this. rsync.net, on the other hand, requires users to
OMIT the leading /.

Here is an example of making an SFTP configuration.  First run

    rclone config

This will guide you through an interactive setup process.

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / SSH/SFTP Connection
   \ "sftp"
[snip]
Storage> sftp
SSH host to connect to
Choose a number from below, or type in your own value
 1 / Connect to example.com
   \ "example.com"
host> example.com
SSH username, leave blank for current username, ncw
user> sftpuser
SSH port, leave blank to use default (22)
port>
SSH password, leave blank to use ssh-agent.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank
y/g/n> n
Path to unencrypted PEM-encoded private key file, leave blank to use ssh-agent.
key_file>
Remote config
--------------------
[remote]
host = example.com
user = sftpuser
port =
pass =
key_file =
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

This remote is called `remote` and can now be used like this:

See all directories in the home directory

    rclone lsd remote:

Make a new directory

    rclone mkdir remote:path/to/directory

List the contents of a directory

    rclone ls remote:path/to/directory

Sync `/home/local/directory` to the remote directory, deleting any
excess files in the directory.

    rclone sync /home/local/directory remote:directory

### SSH Authentication ###

The SFTP remote supports three authentication methods:

  * Password
  * Key file
  * ssh-agent

Key files should be PEM-encoded private key files. For instance `/home/$USER/.ssh/id_rsa`.
Only unencrypted OpenSSH or PEM encrypted files are supported.

The key file can be specified in either an external file (key_file) or contained within the 
rclone config file (key_pem).  If using key_pem in the config file, the entry should be on a
single line with new line ('\n' or '\r\n') separating lines.  i.e. 

key_pem = -----BEGIN RSA PRIVATE KEY-----\nMaMbaIXtE\n0gAMbMbaSsd\nMbaass\n-----END RSA PRIVATE KEY-----

This will generate it correctly for key_pem for use in the config:  

    awk '{printf "%s\\n", $0}' < ~/.ssh/id_rsa

If you don't specify `pass`, `key_file`, or `key_pem` then rclone will attempt to contact an ssh-agent.

You can also specify `key_use_agent` to force the usage of an ssh-agent. In this case
`key_file` or `key_pem` can also be specified to force the usage of a specific key in the ssh-agent.

Using an ssh-agent is the only way to load encrypted OpenSSH keys at the moment.

If you set the `--sftp-ask-password` option, rclone will prompt for a
password when needed and no password has been configured.

### ssh-agent on macOS ###

Note that there seem to be various problems with using an ssh-agent on
macOS due to recent changes in the OS.  The most effective work-around
seems to be to start an ssh-agent in each session, eg

    eval `ssh-agent -s` && ssh-add -A

And then at the end of the session

    eval `ssh-agent -k`

These commands can be used in scripts of course.

### Modified time ###

Modified times are stored on the server to 1 second precision.

Modified times are used in syncing and are fully supported.

Some SFTP servers disable setting/modifying the file modification time after
upload (for example, certain configurations of ProFTPd with mod_sftp). If you
are using one of these servers, you can set the option `set_modtime = false` in
your RClone backend configuration to disable this behaviour.


### Standard Options

Here are the standard options specific to sftp (SSH/SFTP Connection).

#### --sftp-host

SSH host to connect to

- Config:      host
- Env Var:     RCLONE_SFTP_HOST
- Type:        string
- Default:     ""
- Examples:
    - "example.com"
        - Connect to example.com

#### --sftp-user

SSH username, leave blank for current username, ncw

- Config:      user
- Env Var:     RCLONE_SFTP_USER
- Type:        string
- Default:     ""

#### --sftp-port

SSH port, leave blank to use default (22)

- Config:      port
- Env Var:     RCLONE_SFTP_PORT
- Type:        string
- Default:     ""

#### --sftp-pass

SSH password, leave blank to use ssh-agent.

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      pass
- Env Var:     RCLONE_SFTP_PASS
- Type:        string
- Default:     ""

#### --sftp-key-pem

Raw PEM-encoded private key, If specified, will override key_file parameter.

- Config:      key_pem
- Env Var:     RCLONE_SFTP_KEY_PEM
- Type:        string
- Default:     ""

#### --sftp-key-file

Path to PEM-encoded private key file, leave blank or set key-use-agent to use ssh-agent.

- Config:      key_file
- Env Var:     RCLONE_SFTP_KEY_FILE
- Type:        string
- Default:     ""

#### --sftp-key-file-pass

The passphrase to decrypt the PEM-encoded private key file.

Only PEM encrypted key files (old OpenSSH format) are supported. Encrypted keys
in the new OpenSSH format can't be used.

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      key_file_pass
- Env Var:     RCLONE_SFTP_KEY_FILE_PASS
- Type:        string
- Default:     ""

#### --sftp-key-use-agent

When set forces the usage of the ssh-agent.

When key-file is also set, the ".pub" file of the specified key-file is read and only the associated key is
requested from the ssh-agent. This allows to avoid `Too many authentication failures for *username*` errors
when the ssh-agent contains many keys.

- Config:      key_use_agent
- Env Var:     RCLONE_SFTP_KEY_USE_AGENT
- Type:        bool
- Default:     false

#### --sftp-use-insecure-cipher

Enable the use of insecure ciphers and key exchange methods. 

This enables the use of the following insecure ciphers and key exchange methods:

- aes128-cbc
- aes192-cbc
- aes256-cbc
- 3des-cbc
- diffie-hellman-group-exchange-sha256
- diffie-hellman-group-exchange-sha1

Those algorithms are insecure and may allow plaintext data to be recovered by an attacker.

- Config:      use_insecure_cipher
- Env Var:     RCLONE_SFTP_USE_INSECURE_CIPHER
- Type:        bool
- Default:     false
- Examples:
    - "false"
        - Use default Cipher list.
    - "true"
        - Enables the use of the aes128-cbc cipher and diffie-hellman-group-exchange-sha256, diffie-hellman-group-exchange-sha1 key exchange.

#### --sftp-disable-hashcheck

Disable the execution of SSH commands to determine if remote file hashing is available.
Leave blank or set to false to enable hashing (recommended), set to true to disable hashing.

- Config:      disable_hashcheck
- Env Var:     RCLONE_SFTP_DISABLE_HASHCHECK
- Type:        bool
- Default:     false

### Advanced Options

Here are the advanced options specific to sftp (SSH/SFTP Connection).

#### --sftp-ask-password

Allow asking for SFTP password when needed.

If this is set and no password is supplied then rclone will:
- ask for a password
- not contact the ssh agent


- Config:      ask_password
- Env Var:     RCLONE_SFTP_ASK_PASSWORD
- Type:        bool
- Default:     false

#### --sftp-path-override

Override path used by SSH connection.

This allows checksum calculation when SFTP and SSH paths are
different. This issue affects among others Synology NAS boxes.

Shared folders can be found in directories representing volumes

    rclone sync /home/local/directory remote:/directory --ssh-path-override /volume2/directory

Home directory can be found in a shared folder called "home"

    rclone sync /home/local/directory remote:/home/directory --ssh-path-override /volume1/homes/USER/directory

- Config:      path_override
- Env Var:     RCLONE_SFTP_PATH_OVERRIDE
- Type:        string
- Default:     ""

#### --sftp-set-modtime

Set the modified time on the remote if set.

- Config:      set_modtime
- Env Var:     RCLONE_SFTP_SET_MODTIME
- Type:        bool
- Default:     true

#### --sftp-md5sum-command

The command used to read md5 hashes. Leave blank for autodetect.

- Config:      md5sum_command
- Env Var:     RCLONE_SFTP_MD5SUM_COMMAND
- Type:        string
- Default:     ""

#### --sftp-sha1sum-command

The command used to read sha1 hashes. Leave blank for autodetect.

- Config:      sha1sum_command
- Env Var:     RCLONE_SFTP_SHA1SUM_COMMAND
- Type:        string
- Default:     ""

#### --sftp-skip-links

Set to skip any symlinks and any other non regular files.

- Config:      skip_links
- Env Var:     RCLONE_SFTP_SKIP_LINKS
- Type:        bool
- Default:     false



### Limitations ###

SFTP supports checksums if the same login has shell access and `md5sum`
or `sha1sum` as well as `echo` are in the remote's PATH.
This remote checksumming (file hashing) is recommended and enabled by default.
Disabling the checksumming may be required if you are connecting to SFTP servers
which are not under your control, and to which the execution of remote commands
is prohibited.  Set the configuration option `disable_hashcheck` to `true` to
disable checksumming.

SFTP also supports `about` if the same login has shell
access and `df` are in the remote's PATH. `about` will
return the total space, free space, and used space on the remote
for the disk of the specified path on the remote or, if not set,
the disk of the root on the remote.
`about` will fail if it does not have shell
access or if `df` is not in the remote's PATH.

Note that some SFTP servers (eg Synology) the paths are different for
SSH and SFTP so the hashes can't be calculated properly.  For them
using `disable_hashcheck` is a good idea.

The only ssh agent supported under Windows is Putty's pageant.

The Go SSH library disables the use of the aes128-cbc cipher by
default, due to security concerns. This can be re-enabled on a
per-connection basis by setting the `use_insecure_cipher` setting in
the configuration file to `true`. Further details on the insecurity of
this cipher can be found [in this paper]
(http://www.isg.rhul.ac.uk/~kp/SandPfinal.pdf).

SFTP isn't supported under plan9 until [this
issue](https://github.com/pkg/sftp/issues/156) is fixed.

Note that since SFTP isn't HTTP based the following flags don't work
with it: `--dump-headers`, `--dump-bodies`, `--dump-auth`

Note that `--timeout` isn't supported (but `--contimeout` is).


## C14 {#c14}

C14 is supported through the SFTP backend.

See [C14's documentation](https://www.online.net/en/storage/c14-cold-storage)

## rsync.net {#rsync-net}

rsync.net is supported through the SFTP backend.

See [rsync.net's documentation of rclone examples](https://www.rsync.net/products/rclone.html).

 SugarSync
-----------------------------------------

[SugarSync](https://sugarsync.com) is a cloud service that enables
active synchronization of files across computers and other devices for
file backup, access, syncing, and sharing.

The initial setup for SugarSync involves getting a token from SugarSync which you
can do with rclone. `rclone config` walks you through it.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Sugarsync
   \ "sugarsync"
[snip]
Storage> sugarsync
** See help for sugarsync backend at: https://rclone.org/sugarsync/ **

Sugarsync App ID.
Leave blank to use rclone's.
Enter a string value. Press Enter for the default ("").
app_id> 
Sugarsync Access Key ID.
Leave blank to use rclone's.
Enter a string value. Press Enter for the default ("").
access_key_id> 
Sugarsync Private Access Key
Leave blank to use rclone's.
Enter a string value. Press Enter for the default ("").
private_access_key> 
Permanently delete files if true
otherwise put them in the deleted files.
Enter a boolean value (true or false). Press Enter for the default ("false").
hard_delete> 
Edit advanced config? (y/n)
y) Yes
n) No (default)
y/n> n
Remote config
Username (email address)> nick@craig-wood.com
Your Sugarsync password is only required during setup and will not be stored.
password:
--------------------
[remote]
type = sugarsync
refresh_token = https://api.sugarsync.com/app-authorization/XXXXXXXXXXXXXXXXXX
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Note that the config asks for your email and password but doesn't
store them, it only uses them to get the initial token.

Once configured you can then use `rclone` like this,

List directories (sync folders) in top level of your SugarSync

    rclone lsd remote:

List all the files in your SugarSync folder "Test"

    rclone ls remote:Test

To copy a local directory to an SugarSync folder called backup

    rclone copy /home/source remote:backup

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

**NB** you can't create files in the top level folder you have to
create a folder, which rclone will create as a "Sync Folder" with
SugarSync.

### Modified time and hashes ###

SugarSync does not support modification times or hashes, therefore
syncing will default to `--size-only` checking.  Note that using
`--update` will work as rclone can read the time files were uploaded.

#### Restricted filename characters

SugarSync replaces the [default restricted characters set](https://rclone.org/overview/#restricted-characters)
except for DEL.

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in XML strings.

### Deleting files ###

Deleted files will be moved to the "Deleted items" folder by default.

However you can supply the flag `--sugarsync-hard-delete` or set the
config parameter `hard_delete = true` if you would like files to be
deleted straight away.



### Standard Options

Here are the standard options specific to sugarsync (Sugarsync).

#### --sugarsync-app-id

Sugarsync App ID.

Leave blank to use rclone's.

- Config:      app_id
- Env Var:     RCLONE_SUGARSYNC_APP_ID
- Type:        string
- Default:     ""

#### --sugarsync-access-key-id

Sugarsync Access Key ID.

Leave blank to use rclone's.

- Config:      access_key_id
- Env Var:     RCLONE_SUGARSYNC_ACCESS_KEY_ID
- Type:        string
- Default:     ""

#### --sugarsync-private-access-key

Sugarsync Private Access Key

Leave blank to use rclone's.

- Config:      private_access_key
- Env Var:     RCLONE_SUGARSYNC_PRIVATE_ACCESS_KEY
- Type:        string
- Default:     ""

#### --sugarsync-hard-delete

Permanently delete files if true
otherwise put them in the deleted files.

- Config:      hard_delete
- Env Var:     RCLONE_SUGARSYNC_HARD_DELETE
- Type:        bool
- Default:     false

### Advanced Options

Here are the advanced options specific to sugarsync (Sugarsync).

#### --sugarsync-refresh-token

Sugarsync refresh token

Leave blank normally, will be auto configured by rclone.

- Config:      refresh_token
- Env Var:     RCLONE_SUGARSYNC_REFRESH_TOKEN
- Type:        string
- Default:     ""

#### --sugarsync-authorization

Sugarsync authorization

Leave blank normally, will be auto configured by rclone.

- Config:      authorization
- Env Var:     RCLONE_SUGARSYNC_AUTHORIZATION
- Type:        string
- Default:     ""

#### --sugarsync-authorization-expiry

Sugarsync authorization expiry

Leave blank normally, will be auto configured by rclone.

- Config:      authorization_expiry
- Env Var:     RCLONE_SUGARSYNC_AUTHORIZATION_EXPIRY
- Type:        string
- Default:     ""

#### --sugarsync-user

Sugarsync user

Leave blank normally, will be auto configured by rclone.

- Config:      user
- Env Var:     RCLONE_SUGARSYNC_USER
- Type:        string
- Default:     ""

#### --sugarsync-root-id

Sugarsync root id

Leave blank normally, will be auto configured by rclone.

- Config:      root_id
- Env Var:     RCLONE_SUGARSYNC_ROOT_ID
- Type:        string
- Default:     ""

#### --sugarsync-deleted-id

Sugarsync deleted folder id

Leave blank normally, will be auto configured by rclone.

- Config:      deleted_id
- Env Var:     RCLONE_SUGARSYNC_DELETED_ID
- Type:        string
- Default:     ""

#### --sugarsync-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_SUGARSYNC_ENCODING
- Type:        MultiEncoder
- Default:     Slash,Ctl,InvalidUtf8,Dot



 Tardigrade
-----------------------------------------

[Tardigrade](https://tardigrade.io) is an encrypted, secure, and 
cost-effective object storage service that enables you to store, back up, and 
archive large amounts of data in a decentralized manner.

## Setup

To make a new Tardigrade configuration you need one of the following:
* Access Grant that someone else shared with you.
* [API Key](https://documentation.tardigrade.io/getting-started/uploading-your-first-object/create-an-api-key)
of a Tardigrade project you are a member of.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

### Setup with access grant

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Tardigrade Decentralized Cloud Storage
   \ "tardigrade"
[snip]
Storage> tardigrade
** See help for tardigrade backend at: https://rclone.org/tardigrade/ **

Choose an authentication method.
Enter a string value. Press Enter for the default ("existing").
Choose a number from below, or type in your own value
 1 / Use an existing access grant.
   \ "existing"
 2 / Create a new access grant from satellite address, API key, and passphrase.
   \ "new"
provider> existing
Access Grant.
Enter a string value. Press Enter for the default ("").
access_grant> your-access-grant-received-by-someone-else
Remote config
--------------------
[remote]
type = tardigrade
access_grant = your-access-grant-received-by-someone-else
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

### Setup with API key and passhprase

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Enter a string value. Press Enter for the default ("").
Choose a number from below, or type in your own value
[snip]
XX / Tardigrade Decentralized Cloud Storage
   \ "tardigrade"
[snip]
Storage> tardigrade
** See help for tardigrade backend at: https://rclone.org/tardigrade/ **

Choose an authentication method.
Enter a string value. Press Enter for the default ("existing").
Choose a number from below, or type in your own value
 1 / Use an existing access grant.
   \ "existing"
 2 / Create a new access grant from satellite address, API key, and passphrase.
   \ "new"
provider> new
Satellite Address. Custom satellite address should match the format: `<nodeid>@<address>:<port>`.
Enter a string value. Press Enter for the default ("us-central-1.tardigrade.io").
Choose a number from below, or type in your own value
 1 / US Central 1
   \ "us-central-1.tardigrade.io"
 2 / Europe West 1
   \ "europe-west-1.tardigrade.io"
 3 / Asia East 1
   \ "asia-east-1.tardigrade.io"
satellite_address> 1
API Key.
Enter a string value. Press Enter for the default ("").
api_key> your-api-key-for-your-tardigrade-project
Encryption Passphrase. To access existing objects enter passphrase used for uploading.
Enter a string value. Press Enter for the default ("").
passphrase> your-human-readable-encryption-passphrase
Remote config
--------------------
[remote]
type = tardigrade
satellite_address = 12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777
api_key = your-api-key-for-your-tardigrade-project
passphrase = your-human-readable-encryption-passphrase
access_grant = the-access-grant-generated-from-the-api-key-and-passphrase
--------------------
y) Yes this is OK (default)
e) Edit this remote
d) Delete this remote
y/e/d> y
```

## Usage

Paths are specified as `remote:bucket` (or `remote:` for the `lsf`
command.)  You may put subdirectories in too, eg `remote:bucket/path/to/dir`.

Once configured you can then use `rclone` like this.

### Create a new bucket

Use the `mkdir` command to create new bucket, e.g. `bucket`.

    rclone mkdir remote:bucket

### List all buckets

Use the `lsf` command to list all buckets.

    rclone lsf remote:

Note the colon (`:`) character at the end of the command line.

### Delete a bucket

Use the `rmdir` command to delete an empty bucket.

    rclone rmdir remote:bucket

Use the `purge` command to delete a non-empty bucket with all its content.

    rclone purge remote:bucket

### Upload objects

Use the `copy` command to upload an object.

    rclone copy --progress /home/local/directory/file.ext remote:bucket/path/to/dir/

The `--progress` flag is for displaying progress information.
Remove it if you don't need this information.

Use a folder in the local path to upload all its objects.

    rclone copy --progress /home/local/directory/ remote:bucket/path/to/dir/

Only modified files will be copied.

### List objects

Use the `ls` command to list recursively all objects in a bucket.

    rclone ls remote:bucket

Add the folder to the remote path to list recursively all objects in this folder.

    rclone ls remote:bucket/path/to/dir/

Use the `lsf` command to list non-recursively all objects in a bucket or a folder.

    rclone lsf remote:bucket/path/to/dir/

### Download objects

Use the `copy` command to download an object.

    rclone copy --progress remote:bucket/path/to/dir/file.ext /home/local/directory/

The `--progress` flag is for displaying progress information.
Remove it if you don't need this information.

Use a folder in the remote path to download all its objects.

    rclone copy --progress remote:bucket/path/to/dir/ /home/local/directory/

### Delete objects

Use the `deletefile` command to delete a single object.

    rclone deletefile remote:bucket/path/to/dir/file.ext

Use the `delete` command to delete all object in a folder.

    rclone delete remote:bucket/path/to/dir/

### Print the total size of objects

Use the `size` command to print the total size of objects in a bucket or a folder.

    rclone size remote:bucket/path/to/dir/

### Sync two Locations

Use the `sync` command to sync the source to the destination,
changing the destination only, deleting any excess files.

    rclone sync --progress /home/local/directory/ remote:bucket/path/to/dir/

The `--progress` flag is for displaying progress information.
Remove it if you don't need this information.

Since this can cause data loss, test first with the `--dry-run` flag
to see exactly what would be copied and deleted.

The sync can be done also from Tardigrade to the local file system.

    rclone sync --progress remote:bucket/path/to/dir/ /home/local/directory/

Or between two Tardigrade buckets.

    rclone sync --progress remote-us:bucket/path/to/dir/ remote-europe:bucket/path/to/dir/

Or even between another cloud storage and Tardigrade.

    rclone sync --progress s3:bucket/path/to/dir/ tardigrade:bucket/path/to/dir/


### Standard Options

Here are the standard options specific to tardigrade (Tardigrade Decentralized Cloud Storage).

#### --tardigrade-provider

Choose an authentication method.

- Config:      provider
- Env Var:     RCLONE_TARDIGRADE_PROVIDER
- Type:        string
- Default:     "existing"
- Examples:
    - "existing"
        - Use an existing access grant.
    - "new"
        - Create a new access grant from satellite address, API key, and passphrase.

#### --tardigrade-access-grant

Access Grant.

- Config:      access_grant
- Env Var:     RCLONE_TARDIGRADE_ACCESS_GRANT
- Type:        string
- Default:     ""

#### --tardigrade-satellite-address

Satellite Address. Custom satellite address should match the format: `<nodeid>@<address>:<port>`.

- Config:      satellite_address
- Env Var:     RCLONE_TARDIGRADE_SATELLITE_ADDRESS
- Type:        string
- Default:     "us-central-1.tardigrade.io"
- Examples:
    - "us-central-1.tardigrade.io"
        - US Central 1
    - "europe-west-1.tardigrade.io"
        - Europe West 1
    - "asia-east-1.tardigrade.io"
        - Asia East 1

#### --tardigrade-api-key

API Key.

- Config:      api_key
- Env Var:     RCLONE_TARDIGRADE_API_KEY
- Type:        string
- Default:     ""

#### --tardigrade-passphrase

Encryption Passphrase. To access existing objects enter passphrase used for uploading.

- Config:      passphrase
- Env Var:     RCLONE_TARDIGRADE_PASSPHRASE
- Type:        string
- Default:     ""



 Union
-----------------------------------------

The `union` remote provides a unification similar to UnionFS using other remotes.

Paths may be as deep as required or a local path, 
eg `remote:directory/subdirectory` or `/directory/subdirectory`.

During the initial setup with `rclone config` you will specify the upstream
remotes as a space separated list. The upstream remotes can either be a local paths or other remotes.

Attribute `:ro` and `:nc` can be attach to the end of path to tag the remote as **read only** or **no create**,
eg `remote:directory/subdirectory:ro` or `remote:directory/subdirectory:nc`.

Subfolders can be used in upstream remotes. Assume a union remote named `backup`
with the remotes `mydrive:private/backup`. Invoking `rclone mkdir backup:desktop`
is exactly the same as invoking `rclone mkdir mydrive2:/backup/desktop`.

There will be no special handling of paths containing `..` segments.
Invoking `rclone mkdir backup:../desktop` is exactly the same as invoking
`rclone mkdir mydrive2:/backup/../desktop`.

### Behavior / Policies

The behavior of union backend is inspired by [trapexit/mergerfs](https://github.com/trapexit/mergerfs). All functions are grouped into 3 categories: **action**, **create** and **search**. These functions and categories can be assigned a policy which dictates what file or directory is chosen when performing that behavior. Any policy can be assigned to a function or category though some may not be very useful in practice. For instance: **rand** (random) may be useful for file creation (create) but could lead to very odd behavior if used for `delete` if there were more than one copy of the file.

#### Function / Category classifications

| Category | Description              | Functions                                                                           |
|----------|--------------------------|-------------------------------------------------------------------------------------|
| action   | Writing Existing file    | move, rmdir, rmdirs, delete, purge and copy, sync (as destination when file exist)  |
| create   | Create non-existing file | copy, sync (as destination when file not exist)                                     |
| search   | Reading and listing file | ls, lsd, lsl, cat, md5sum, sha1sum and copy, sync (as source)                       |
| N/A      |                          | size, about                                                                         |

#### Path Preservation

Policies, as described below, are of two basic types. `path preserving` and `non-path preserving`.

All policies which start with `ep` (**epff**, **eplfs**, **eplus**, **epmfs**, **eprand**) are `path preserving`. `ep` stands for `existing path`.

A path preserving policy will only consider upstreams where the relative path being accessed already exists.

When using non-path preserving policies paths will be created in target upstreams as necessary.

#### Quota Relevant Policies

Some policies rely on quota information. These policies should be used only if your upstreams support the respective quota fields.

| Policy     | Required Field |
|------------|----------------|
| lfs, eplfs | Free           |
| mfs, epmfs | Free           |
| lus, eplus | Used           |
| lno, eplno | Objects        |

To check if your upstream supports the field, run `rclone about remote: [flags]` and see if the required field exists.

#### Filters

Policies basically search upstream remotes and create a list of files / paths for functions to work on. The policy is responsible for filtering and sorting. The policy type defines the sorting but filtering is mostly uniform as described below.

* No **search** policies filter.
* All **action** policies will filter out remotes which are tagged as **read-only**.
* All **create** policies will filter out remotes which are tagged **read-only** or **no-create**.

If all remotes are filtered an error will be returned.

#### Policy descriptions

The policies definition are inspired by [trapexit/mergerfs](https://github.com/trapexit/mergerfs) but not exactly the same. Some policy definition could be different due to the much larger latency of remote file systems.

| Policy           | Description                                                |
|------------------|------------------------------------------------------------|
| all | Search category: same as **epall**. Action category: same as **epall**. Create category: act on all upstreams. |
| epall (existing path, all) | Search category: Given this order configured, act on the first one found where the relative path exists. Action category: apply to all found. Create category: act on all upstreams where the relative path exists. |
| epff (existing path, first found) | Act on the first one found, by the time upstreams reply, where the relative path exists. |
| eplfs (existing path, least free space) | Of all the upstreams on which the relative path exists choose the one with the least free space. |
| eplus (existing path, least used space) | Of all the upstreams on which the relative path exists choose the one with the least used space. |
| eplno (existing path, least number of objects) | Of all the upstreams on which the relative path exists choose the one with the least number of objects. |
| epmfs (existing path, most free space) | Of all the upstreams on which the relative path exists choose the one with the most free space. |
| eprand (existing path, random) | Calls **epall** and then randomizes. Returns only one upstream. |
| ff (first found) | Search category: same as **epff**. Action category: same as **epff**. Create category: Act on the first one found by the time upstreams reply. |
| lfs (least free space) | Search category: same as **eplfs**. Action category: same as **eplfs**. Create category: Pick the upstream with the least available free space. |
| lus (least used space) | Search category: same as **eplus**. Action category: same as **eplus**. Create category: Pick the upstream with the least used space. |
| lno (least number of objects) | Search category: same as **eplno**. Action category: same as **eplno**. Create category: Pick the upstream with the least number of objects. |
| mfs (most free space) | Search category: same as **epmfs**. Action category: same as **epmfs**. Create category: Pick the upstream with the most available free space. |
| newest | Pick the file / directory with the largest mtime. |
| rand (random) | Calls **all** and then randomizes. Returns only one upstream. |

### Setup

Here is an example of how to make a union called `remote` for local folders.
First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Union merges the contents of several remotes
   \ "union"
[snip]
Storage> union
List of space separated upstreams.
Can be 'upstreama:test/dir upstreamb:', '\"upstreama:test/space:ro dir\" upstreamb:', etc.
Enter a string value. Press Enter for the default ("").
upstreams>
Policy to choose upstream on ACTION class.
Enter a string value. Press Enter for the default ("epall").
action_policy>
Policy to choose upstream on CREATE class.
Enter a string value. Press Enter for the default ("epmfs").
create_policy>
Policy to choose upstream on SEARCH class.
Enter a string value. Press Enter for the default ("ff").
search_policy>
Cache time of usage and free space (in seconds). This option is only useful when a path preserving policy is used.
Enter a signed integer. Press Enter for the default ("120").
cache_time>
Remote config
--------------------
[remote]
type = union
upstreams = C:\dir1 C:\dir2 C:\dir3
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
Current remotes:

Name                 Type
====                 ====
remote               union

e) Edit existing remote
n) New remote
d) Delete remote
r) Rename remote
c) Copy remote
s) Set configuration password
q) Quit config
e/n/d/r/c/s/q> q
```

Once configured you can then use `rclone` like this,

List directories in top level in `C:\dir1`, `C:\dir2` and `C:\dir3`

    rclone lsd remote:

List all the files in `C:\dir1`, `C:\dir2` and `C:\dir3`

    rclone ls remote:

Copy another local directory to the union directory called source, which will be placed into `C:\dir3`

    rclone copy C:\source remote:source


### Standard Options

Here are the standard options specific to union (Union merges the contents of several upstream fs).

#### --union-upstreams

List of space separated upstreams.
Can be 'upstreama:test/dir upstreamb:', '"upstreama:test/space:ro dir" upstreamb:', etc.


- Config:      upstreams
- Env Var:     RCLONE_UNION_UPSTREAMS
- Type:        string
- Default:     ""

#### --union-action-policy

Policy to choose upstream on ACTION category.

- Config:      action_policy
- Env Var:     RCLONE_UNION_ACTION_POLICY
- Type:        string
- Default:     "epall"

#### --union-create-policy

Policy to choose upstream on CREATE category.

- Config:      create_policy
- Env Var:     RCLONE_UNION_CREATE_POLICY
- Type:        string
- Default:     "epmfs"

#### --union-search-policy

Policy to choose upstream on SEARCH category.

- Config:      search_policy
- Env Var:     RCLONE_UNION_SEARCH_POLICY
- Type:        string
- Default:     "ff"

#### --union-cache-time

Cache time of usage and free space (in seconds). This option is only useful when a path preserving policy is used.

- Config:      cache_time
- Env Var:     RCLONE_UNION_CACHE_TIME
- Type:        int
- Default:     120



 WebDAV
-----------------------------------------

Paths are specified as `remote:path`

Paths may be as deep as required, eg `remote:directory/subdirectory`.

To configure the WebDAV remote you will need to have a URL for it, and
a username and password.  If you know what kind of system you are
connecting to then rclone can enable extra features.

Here is an example of how to make a remote called `remote`.  First run:

     rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
q) Quit config
n/s/q> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Webdav
   \ "webdav"
[snip]
Storage> webdav
URL of http host to connect to
Choose a number from below, or type in your own value
 1 / Connect to example.com
   \ "https://example.com"
url> https://example.com/remote.php/webdav/
Name of the Webdav site/service/software you are using
Choose a number from below, or type in your own value
 1 / Nextcloud
   \ "nextcloud"
 2 / Owncloud
   \ "owncloud"
 3 / Sharepoint
   \ "sharepoint"
 4 / Other site/service or software
   \ "other"
vendor> 1
User name
user> user
Password.
y) Yes type in my own password
g) Generate random password
n) No leave this optional password blank
y/g/n> y
Enter the password:
password:
Confirm the password:
password:
Bearer token instead of user/pass (eg a Macaroon)
bearer_token>
Remote config
--------------------
[remote]
type = webdav
url = https://example.com/remote.php/webdav/
vendor = nextcloud
user = user
pass = *** ENCRYPTED ***
bearer_token =
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

Once configured you can then use `rclone` like this,

List directories in top level of your WebDAV

    rclone lsd remote:

List all the files in your WebDAV

    rclone ls remote:

To copy a local directory to an WebDAV directory called backup

    rclone copy /home/source remote:backup

### Modified time and hashes ###

Plain WebDAV does not support modified times.  However when used with
Owncloud or Nextcloud rclone will support modified times.

Likewise plain WebDAV does not support hashes, however when used with
Owncloud or Nextcloud rclone will support SHA1 and MD5 hashes.
Depending on the exact version of Owncloud or Nextcloud hashes may
appear on all objects, or only on objects which had a hash uploaded
with them.


### Standard Options

Here are the standard options specific to webdav (Webdav).

#### --webdav-url

URL of http host to connect to

- Config:      url
- Env Var:     RCLONE_WEBDAV_URL
- Type:        string
- Default:     ""
- Examples:
    - "https://example.com"
        - Connect to example.com

#### --webdav-vendor

Name of the Webdav site/service/software you are using

- Config:      vendor
- Env Var:     RCLONE_WEBDAV_VENDOR
- Type:        string
- Default:     ""
- Examples:
    - "nextcloud"
        - Nextcloud
    - "owncloud"
        - Owncloud
    - "sharepoint"
        - Sharepoint
    - "other"
        - Other site/service or software

#### --webdav-user

User name

- Config:      user
- Env Var:     RCLONE_WEBDAV_USER
- Type:        string
- Default:     ""

#### --webdav-pass

Password.

**NB** Input to this must be obscured - see [rclone obscure](https://rclone.org/commands/rclone_obscure/).

- Config:      pass
- Env Var:     RCLONE_WEBDAV_PASS
- Type:        string
- Default:     ""

#### --webdav-bearer-token

Bearer token instead of user/pass (eg a Macaroon)

- Config:      bearer_token
- Env Var:     RCLONE_WEBDAV_BEARER_TOKEN
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to webdav (Webdav).

#### --webdav-bearer-token-command

Command to run to get a bearer token

- Config:      bearer_token_command
- Env Var:     RCLONE_WEBDAV_BEARER_TOKEN_COMMAND
- Type:        string
- Default:     ""



## Provider notes ##

See below for notes on specific providers.

### Owncloud ###

Click on the settings cog in the bottom right of the page and this
will show the WebDAV URL that rclone needs in the config step.  It
will look something like `https://example.com/remote.php/webdav/`.

Owncloud supports modified times using the `X-OC-Mtime` header.

### Nextcloud ###

This is configured in an identical way to Owncloud.  Note that
Nextcloud does not support streaming of files (`rcat`) whereas
Owncloud does. This [may be
fixed](https://github.com/nextcloud/nextcloud-snap/issues/365) in the
future.

### Sharepoint ###

Rclone can be used with Sharepoint provided by OneDrive for Business
or Office365 Education Accounts.
This feature is only needed for a few of these Accounts,
mostly Office365 Education ones. These accounts are sometimes not
verified by the domain owner [github#1975](https://github.com/rclone/rclone/issues/1975)

This means that these accounts can't be added using the official
API (other Accounts should work with the "onedrive" option). However,
it is possible to access them using webdav.

To use a sharepoint remote with rclone, add it like this:
First, you need to get your remote's URL:

- Go [here](https://onedrive.live.com/about/en-us/signin/)
  to open your OneDrive or to sign in
- Now take a look at your address bar, the URL should look like this:
  `https://[YOUR-DOMAIN]-my.sharepoint.com/personal/[YOUR-EMAIL]/_layouts/15/onedrive.aspx`

You'll only need this URL up to the email address. After that, you'll
most likely want to add "/Documents". That subdirectory contains
the actual data stored on your OneDrive.

Add the remote to rclone like this:
Configure the `url` as `https://[YOUR-DOMAIN]-my.sharepoint.com/personal/[YOUR-EMAIL]/Documents`
and use your normal account email and password for `user` and `pass`.
If you have 2FA enabled, you have to generate an app password.
Set the `vendor` to `sharepoint`.

Your config file should look like this:

```
[sharepoint]
type = webdav
url = https://[YOUR-DOMAIN]-my.sharepoint.com/personal/[YOUR-EMAIL]/Documents
vendor = other
user = YourEmailAddress
pass = encryptedpassword
```

#### Required Flags for SharePoint ####
As SharePoint does some special things with uploaded documents, you won't be able to use the documents size or the documents hash to compare if a file has been changed since the upload / which file is newer.

For Rclone calls copying files (especially Office files such as .docx, .xlsx, etc.) from/to SharePoint (like copy, sync, etc.), you should append these flags to ensure Rclone uses the "Last Modified" datetime property to compare your documents:

```
--ignore-size --ignore-checksum --update
```

### dCache ###

dCache is a storage system that supports many protocols and
authentication/authorisation schemes.  For WebDAV clients, it allows
users to authenticate with username and password (BASIC), X.509,
Kerberos, and various bearer tokens, including
[Macaroons](https://www.dcache.org/manuals/workshop-2017-05-29-Umea/000-Final/anupam_macaroons_v02.pdf)
and [OpenID-Connect](https://en.wikipedia.org/wiki/OpenID_Connect)
access tokens.

Configure as normal using the `other` type.  Don't enter a username or
password, instead enter your Macaroon as the `bearer_token`.

The config will end up looking something like this.

```
[dcache]
type = webdav
url = https://dcache...
vendor = other
user =
pass =
bearer_token = your-macaroon
```

There is a [script](https://github.com/sara-nl/GridScripts/blob/master/get-macaroon) that
obtains a Macaroon from a dCache WebDAV endpoint, and creates an rclone config file.

Macaroons may also be obtained from the dCacheView
web-browser/JavaScript client that comes with dCache.

### OpenID-Connect ###

dCache also supports authenticating with OpenID-Connect access tokens.
OpenID-Connect is a protocol (based on OAuth 2.0) that allows services
to identify users who have authenticated with some central service.

Support for OpenID-Connect in rclone is currently achieved using
another software package called
[oidc-agent](https://github.com/indigo-dc/oidc-agent).  This is a
command-line tool that facilitates obtaining an access token.  Once
installed and configured, an access token is obtained by running the
`oidc-token` command.  The following example shows a (shortened)
access token obtained from the *XDC* OIDC Provider.

```
paul@celebrimbor:~$ oidc-token XDC
eyJraWQ[...]QFXDt0
paul@celebrimbor:~$
```

**Note** Before the `oidc-token` command will work, the refresh token
must be loaded into the oidc agent.  This is done with the `oidc-add`
command (e.g., `oidc-add XDC`).  This is typically done once per login
session.  Full details on this and how to register oidc-agent with
your OIDC Provider are provided in the [oidc-agent
documentation](https://indigo-dc.gitbooks.io/oidc-agent/).

The rclone `bearer_token_command` configuration option is used to
fetch the access token from oidc-agent.

Configure as a normal WebDAV endpoint, using the 'other' vendor,
leaving the username and password empty.  When prompted, choose to
edit the advanced config and enter the command to get a bearer token
(e.g., `oidc-agent XDC`).

The following example config shows a WebDAV endpoint that uses
oidc-agent to supply an access token from the *XDC* OIDC Provider.

```
[dcache]
type = webdav
url = https://dcache.example.org/
vendor = other
bearer_token_command = oidc-token XDC
```

Yandex Disk
----------------------------------------

[Yandex Disk](https://disk.yandex.com) is a cloud storage solution created by [Yandex](https://yandex.com).

Here is an example of making a yandex configuration.  First run

    rclone config

This will guide you through an interactive setup process:

```
No remotes found - make a new one
n) New remote
s) Set configuration password
n/s> n
name> remote
Type of storage to configure.
Choose a number from below, or type in your own value
[snip]
XX / Yandex Disk
   \ "yandex"
[snip]
Storage> yandex
Yandex Client Id - leave blank normally.
client_id>
Yandex Client Secret - leave blank normally.
client_secret>
Remote config
Use auto config?
 * Say Y if not sure
 * Say N if you are working on a remote or headless machine
y) Yes
n) No
y/n> y
If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
Log in and authorize rclone for access
Waiting for code...
Got code
--------------------
[remote]
client_id =
client_secret =
token = {"access_token":"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","token_type":"bearer","expiry":"2016-12-29T12:27:11.362788025Z"}
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
```

See the [remote setup docs](https://rclone.org/remote_setup/) for how to set it up on a
machine with no Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Yandex Disk. This only runs from the moment it
opens your browser to the moment you get back the verification code.
This is on `http://127.0.0.1:53682/` and this it may require you to
unblock it temporarily if you are running a host firewall.

Once configured you can then use `rclone` like this,

See top level directories

    rclone lsd remote:

Make a new directory

    rclone mkdir remote:directory

List the contents of a directory

    rclone ls remote:directory

Sync `/home/local/directory` to the remote path, deleting any
excess files in the path.

    rclone sync /home/local/directory remote:directory

Yandex paths may be as deep as required, eg `remote:directory/subdirectory`.

### Modified time ###

Modified times are supported and are stored accurate to 1 ns in custom
metadata called `rclone_modified` in RFC3339 with nanoseconds format.

### MD5 checksums ###

MD5 checksums are natively supported by Yandex Disk.

### Emptying Trash ###

If you wish to empty your trash you can use the `rclone cleanup remote:`
command which will permanently delete all your trashed files. This command
does not take any path arguments.

### Quota information ###

To view your current quota you can use the `rclone about remote:`
command which will display your usage limit (quota) and the current usage.

#### Restricted filename characters

The [default restricted characters set](https://rclone.org/overview/#restricted-characters)
are replaced.

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be used in JSON strings.

### Limitations ###

When uploading very large files (bigger than about 5GB) you will need
to increase the `--timeout` parameter.  This is because Yandex pauses
(perhaps to calculate the MD5SUM for the entire file) before returning
confirmation that the file has been uploaded.  The default handling of
timeouts in rclone is to assume a 5 minute pause is an error and close
the connection - you'll see `net/http: timeout awaiting response
headers` errors in the logs if this is happening.  Setting the timeout
to twice the max size of file in GB should be enough, so if you want
to upload a 30GB file set a timeout of `2 * 30 = 60m`, that is
`--timeout 60m`.


### Standard Options

Here are the standard options specific to yandex (Yandex Disk).

#### --yandex-client-id

Yandex Client Id
Leave blank normally.

- Config:      client_id
- Env Var:     RCLONE_YANDEX_CLIENT_ID
- Type:        string
- Default:     ""

#### --yandex-client-secret

Yandex Client Secret
Leave blank normally.

- Config:      client_secret
- Env Var:     RCLONE_YANDEX_CLIENT_SECRET
- Type:        string
- Default:     ""

### Advanced Options

Here are the advanced options specific to yandex (Yandex Disk).

#### --yandex-unlink

Remove existing public link to file/folder with link command rather than creating.
Default is false, meaning link command will create or retrieve public link.

- Config:      unlink
- Env Var:     RCLONE_YANDEX_UNLINK
- Type:        bool
- Default:     false

#### --yandex-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_YANDEX_ENCODING
- Type:        MultiEncoder
- Default:     Slash,Del,Ctl,InvalidUtf8,Dot



 Local Filesystem
-------------------------------------------

Local paths are specified as normal filesystem paths, eg `/path/to/wherever`, so

    rclone sync /home/source /tmp/destination

Will sync `/home/source` to `/tmp/destination`

These can be configured into the config file for consistencies sake,
but it is probably easier not to.

### Modified time ###

Rclone reads and writes the modified time using an accuracy determined by
the OS.  Typically this is 1ns on Linux, 10 ns on Windows and 1 Second
on OS X.

### Filenames ###

Filenames should be encoded in UTF-8 on disk. This is the normal case
for Windows and OS X.

There is a bit more uncertainty in the Linux world, but new
distributions will have UTF-8 encoded files names. If you are using an
old Linux filesystem with non UTF-8 file names (eg latin1) then you
can use the `convmv` tool to convert the filesystem to UTF-8. This
tool is available in most distributions' package managers.

If an invalid (non-UTF8) filename is read, the invalid characters will
be replaced with a quoted representation of the invalid bytes. The name
`gro\xdf` will be transferred as `gro‛DF`. `rclone` will emit a debug
message in this case (use `-v` to see), eg

```
Local file system at .: Replacing invalid UTF-8 characters in "gro\xdf"
```

#### Restricted characters

On non Windows platforms the following characters are replaced when
handling file names.

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| /         | 0x2F  | ／           |

When running on Windows the following characters are replaced. This
list is based on the [Windows file naming conventions](https://docs.microsoft.com/de-de/windows/desktop/FileIO/naming-a-file#naming-conventions).

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| NUL       | 0x00  | ␀           |
| SOH       | 0x01  | ␁           |
| STX       | 0x02  | ␂           |
| ETX       | 0x03  | ␃           |
| EOT       | 0x04  | ␄           |
| ENQ       | 0x05  | ␅           |
| ACK       | 0x06  | ␆           |
| BEL       | 0x07  | ␇           |
| BS        | 0x08  | ␈           |
| HT        | 0x09  | ␉           |
| LF        | 0x0A  | ␊           |
| VT        | 0x0B  | ␋           |
| FF        | 0x0C  | ␌           |
| CR        | 0x0D  | ␍           |
| SO        | 0x0E  | ␎           |
| SI        | 0x0F  | ␏           |
| DLE       | 0x10  | ␐           |
| DC1       | 0x11  | ␑           |
| DC2       | 0x12  | ␒           |
| DC3       | 0x13  | ␓           |
| DC4       | 0x14  | ␔           |
| NAK       | 0x15  | ␕           |
| SYN       | 0x16  | ␖           |
| ETB       | 0x17  | ␗           |
| CAN       | 0x18  | ␘           |
| EM        | 0x19  | ␙           |
| SUB       | 0x1A  | ␚           |
| ESC       | 0x1B  | ␛           |
| FS        | 0x1C  | ␜           |
| GS        | 0x1D  | ␝           |
| RS        | 0x1E  | ␞           |
| US        | 0x1F  | ␟           |
| /         | 0x2F  | ／           |
| "         | 0x22  | ＂           |
| *         | 0x2A  | ＊           |
| :         | 0x3A  | ：           |
| <         | 0x3C  | ＜           |
| >         | 0x3E  | ＞           |
| ?         | 0x3F  | ？           |
| \         | 0x5C  | ＼           |
| \|        | 0x7C  | ｜           |

File names on Windows can also not end with the following characters.
These only get replaced if they are the last character in the name:

| Character | Value | Replacement |
| --------- |:-----:|:-----------:|
| SP        | 0x20  | ␠           |
| .         | 0x2E  | ．           |

Invalid UTF-8 bytes will also be [replaced](https://rclone.org/overview/#invalid-utf8),
as they can't be converted to UTF-16.

### Long paths on Windows ###

Rclone handles long paths automatically, by converting all paths to long
[UNC paths](https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath)
which allows paths up to 32,767 characters.

This is why you will see that your paths, for instance `c:\files` is
converted to the UNC path `\\?\c:\files` in the output,
and `\\server\share` is converted to `\\?\UNC\server\share`.

However, in rare cases this may cause problems with buggy file
system drivers like [EncFS](https://github.com/rclone/rclone/issues/261).
To disable UNC conversion globally, add this to your `.rclone.conf` file:

```
[local]
nounc = true
```

If you want to selectively disable UNC, you can add it to a separate entry like this:

```
[nounc]
type = local
nounc = true
```
And use rclone like this:

`rclone copy c:\src nounc:z:\dst`

This will use UNC paths on `c:\src` but not on `z:\dst`.
Of course this will cause problems if the absolute path length of a
file exceeds 258 characters on z, so only use this option if you have to.

### Symlinks / Junction points

Normally rclone will ignore symlinks or junction points (which behave
like symlinks under Windows).

If you supply `--copy-links` or `-L` then rclone will follow the
symlink and copy the pointed to file or directory.  Note that this
flag is incompatible with `-links` / `-l`.

This flag applies to all commands.

For example, supposing you have a directory structure like this

```
$ tree /tmp/a
/tmp/a
├── b -> ../b
├── expected -> ../expected
├── one
└── two
    └── three
```

Then you can see the difference with and without the flag like this

```
$ rclone ls /tmp/a
        6 one
        6 two/three
```

and

```
$ rclone -L ls /tmp/a
     4174 expected
        6 one
        6 two/three
        6 b/two
        6 b/one
```

#### --links, -l 

Normally rclone will ignore symlinks or junction points (which behave
like symlinks under Windows).

If you supply this flag then rclone will copy symbolic links from the local storage,
and store them as text files, with a '.rclonelink' suffix in the remote storage.

The text file will contain the target of the symbolic link (see example).

This flag applies to all commands.

For example, supposing you have a directory structure like this

```
$ tree /tmp/a
/tmp/a
├── file1 -> ./file4
└── file2 -> /home/user/file3
```

Copying the entire directory with '-l'

```
$ rclone copyto -l /tmp/a/file1 remote:/tmp/a/
```

The remote files are created with a '.rclonelink' suffix

```
$ rclone ls remote:/tmp/a
       5 file1.rclonelink
      14 file2.rclonelink
```

The remote files will contain the target of the symbolic links

```
$ rclone cat remote:/tmp/a/file1.rclonelink
./file4

$ rclone cat remote:/tmp/a/file2.rclonelink
/home/user/file3
```

Copying them back with '-l'

```
$ rclone copyto -l remote:/tmp/a/ /tmp/b/

$ tree /tmp/b
/tmp/b
├── file1 -> ./file4
└── file2 -> /home/user/file3
```

However, if copied back without '-l'

```
$ rclone copyto remote:/tmp/a/ /tmp/b/

$ tree /tmp/b
/tmp/b
├── file1.rclonelink
└── file2.rclonelink
````

Note that this flag is incompatible with `-copy-links` / `-L`.

### Restricting filesystems with --one-file-system

Normally rclone will recurse through filesystems as mounted.

However if you set `--one-file-system` or `-x` this tells rclone to
stay in the filesystem specified by the root and not to recurse into
different file systems.

For example if you have a directory hierarchy like this

```
root
├── disk1     - disk1 mounted on the root
│   └── file3 - stored on disk1
├── disk2     - disk2 mounted on the root
│   └── file4 - stored on disk12
├── file1     - stored on the root disk
└── file2     - stored on the root disk
```

Using `rclone --one-file-system copy root remote:` will only copy `file1` and `file2`.  Eg

```
$ rclone -q --one-file-system ls root
        0 file1
        0 file2
```

```
$ rclone -q ls root
        0 disk1/file3
        0 disk2/file4
        0 file1
        0 file2
```

**NB** Rclone (like most unix tools such as `du`, `rsync` and `tar`)
treats a bind mount to the same device as being on the same
filesystem.

**NB** This flag is only available on Unix based systems.  On systems
where it isn't supported (eg Windows) it will be ignored.


### Standard Options

Here are the standard options specific to local (Local Disk).

#### --local-nounc

Disable UNC (long path names) conversion on Windows

- Config:      nounc
- Env Var:     RCLONE_LOCAL_NOUNC
- Type:        string
- Default:     ""
- Examples:
    - "true"
        - Disables long file names

### Advanced Options

Here are the advanced options specific to local (Local Disk).

#### --copy-links / -L

Follow symlinks and copy the pointed to item.

- Config:      copy_links
- Env Var:     RCLONE_LOCAL_COPY_LINKS
- Type:        bool
- Default:     false

#### --links / -l

Translate symlinks to/from regular files with a '.rclonelink' extension

- Config:      links
- Env Var:     RCLONE_LOCAL_LINKS
- Type:        bool
- Default:     false

#### --skip-links

Don't warn about skipped symlinks.
This flag disables warning messages on skipped symlinks or junction
points, as you explicitly acknowledge that they should be skipped.

- Config:      skip_links
- Env Var:     RCLONE_LOCAL_SKIP_LINKS
- Type:        bool
- Default:     false

#### --local-no-unicode-normalization

Don't apply unicode normalization to paths and filenames (Deprecated)

This flag is deprecated now.  Rclone no longer normalizes unicode file
names, but it compares them with unicode normalization in the sync
routine instead.

- Config:      no_unicode_normalization
- Env Var:     RCLONE_LOCAL_NO_UNICODE_NORMALIZATION
- Type:        bool
- Default:     false

#### --local-no-check-updated

Don't check to see if the files change during upload

Normally rclone checks the size and modification time of files as they
are being uploaded and aborts with a message which starts "can't copy
- source file is being updated" if the file changes during upload.

However on some file systems this modification time check may fail (eg
[Glusterfs #2206](https://github.com/rclone/rclone/issues/2206)) so this
check can be disabled with this flag.

- Config:      no_check_updated
- Env Var:     RCLONE_LOCAL_NO_CHECK_UPDATED
- Type:        bool
- Default:     false

#### --one-file-system / -x

Don't cross filesystem boundaries (unix/macOS only).

- Config:      one_file_system
- Env Var:     RCLONE_LOCAL_ONE_FILE_SYSTEM
- Type:        bool
- Default:     false

#### --local-case-sensitive

Force the filesystem to report itself as case sensitive.

Normally the local backend declares itself as case insensitive on
Windows/macOS and case sensitive for everything else.  Use this flag
to override the default choice.

- Config:      case_sensitive
- Env Var:     RCLONE_LOCAL_CASE_SENSITIVE
- Type:        bool
- Default:     false

#### --local-case-insensitive

Force the filesystem to report itself as case insensitive

Normally the local backend declares itself as case insensitive on
Windows/macOS and case sensitive for everything else.  Use this flag
to override the default choice.

- Config:      case_insensitive
- Env Var:     RCLONE_LOCAL_CASE_INSENSITIVE
- Type:        bool
- Default:     false

#### --local-no-sparse

Disable sparse files for multi-thread downloads

On Windows platforms rclone will make sparse files when doing
multi-thread downloads. This avoids long pauses on large files where
the OS zeros the file. However sparse files may be undesirable as they
cause disk fragmentation and can be slow to work with.

- Config:      no_sparse
- Env Var:     RCLONE_LOCAL_NO_SPARSE
- Type:        bool
- Default:     false

#### --local-encoding

This sets the encoding for the backend.

See: the [encoding section in the overview](https://rclone.org/overview/#encoding) for more info.

- Config:      encoding
- Env Var:     RCLONE_LOCAL_ENCODING
- Type:        MultiEncoder
- Default:     Slash,Dot

### Backend commands

Here are the commands specific to the local backend.

Run them with

    rclone backend COMMAND remote:

The help below will explain what arguments each command takes.

See [the "rclone backend" command](https://rclone.org/commands/rclone_backend/) for more
info on how to pass options and arguments.

These can be run on a running backend using the rc command
[backend/command](https://rclone.org/rc/#backend/command).

#### noop

A null operation for testing backend commands

    rclone backend noop remote: [options] [<arguments>+]

This is a test command which has some options
you can try to change the output.

Options:

- "echo": echo the input arguments
- "error": return an error based on option value



# Changelog

## v1.52.2 - 2020-06-24

[See commits](https://github.com/rclone/rclone/compare/v1.52.1...v1.52.2)

* Bug Fixes
    * build
        * Fix docker release build action (Nick Craig-Wood)
        * Fix custom timezone in Docker image (NoLooseEnds)
    * check: Fix misleading message which printed errors instead of differences (Nick Craig-Wood)
    * errors: Add WSAECONNREFUSED and more to the list of retriable Windows errors (Nick Craig-Wood)
    * rcd: Fix incorrect prometheus metrics (Gary Kim)
    * serve restic: Fix flags so they use environment variables (Nick Craig-Wood)
    * serve webdav: Fix flags so they use environment variables (Nick Craig-Wood)
    * sync: Fix --track-renames-strategy modtime (Nick Craig-Wood)
* Drive
    * Fix not being able to delete a directory with a trashed shortcut (Nick Craig-Wood)
    * Fix creating a directory inside a shortcut (Nick Craig-Wood)
    * Fix --drive-impersonate with cached root_folder_id (Nick Craig-Wood)
* SFTP
    * Fix SSH key PEM loading (Zac Rubin)
* Swift
    * Speed up deletes by not retrying segment container deletes (Nick Craig-Wood)
* Tardigrade
    * Upgrade to uplink v1.1.1 (Caleb Case)
* WebDAV
    * Fix free/used display for rclone about/df for certain backends (Nick Craig-Wood)

## v1.52.1 - 2020-06-10

[See commits](https://github.com/rclone/rclone/compare/v1.52.0...v1.52.1)

* Bug Fixes
    * lib/file: Fix SetSparse on Windows 7 which fixes downloads of files > 250MB (Nick Craig-Wood)
    * build
        * Update go.mod to go1.14 to enable -mod=vendor build (Nick Craig-Wood)
        * Remove quicktest from Dockerfile (Nick Craig-Wood)
        * Build Docker images with GitHub actions (Matteo Pietro Dazzi)
        * Update Docker build workflows (Nick Craig-Wood)
        * Set user_allow_other in /etc/fuse.conf in the Docker image (Nick Craig-Wood)
        * Fix xgo build after go1.14 go.mod update (Nick Craig-Wood)
    * docs
        * Add link to source and modified time to footer of every page (Nick Craig-Wood)
        * Remove manually set dates and use git dates instead (Nick Craig-Wood)
        * Minor tense, punctuation, brevity and positivity changes for the home page (edwardxml)
        * Remove leading slash in page reference in footer when present (Nick Craig-Wood)
        * Note commands which need obscured input in the docs (Nick Craig-Wood)
        * obscure: Write more help as we are referencing it elsewhere (Nick Craig-Wood)
* VFS
    * Fix OS vs Unix path confusion - fixes ChangeNotify on Windows (Nick Craig-Wood)
* Drive
    * Fix missing items when listing using --fast-list / ListR (Nick Craig-Wood)
* Putio
    * Fix panic on Object.Open (Cenk Alti)
* S3
    * Fix upload of single files into buckets without create permission (Nick Craig-Wood)
    * Fix --header-upload (Nick Craig-Wood)
* Tardigrade
    * Fix listing bug by upgrading to v1.0.7
    * Set UserAgent to rclone (Caleb Case)

## v1.52.0 - 2020-05-27

Special thanks to Martin Michlmayr for proof reading and correcting
all the docs and Edward Barker for helping re-write the front page.

[See commits](https://github.com/rclone/rclone/compare/v1.51.0...v1.52.0)

* New backends
    * [Tardigrade](https://rclone.org/tardigrade/) backend for use with storj.io (Caleb Case)
    * [Union](https://rclone.org/union/) re-write to have multiple writable remotes (Max Sum)
    * [Seafile](/seafile) for Seafile server (Fred @creativeprojects)
* New commands
    * backend: command for backend specific commands (see backends) (Nick Craig-Wood)
    * cachestats: Deprecate in favour of `rclone backend stats cache:` (Nick Craig-Wood)
    * dbhashsum: Deprecate in favour of `rclone hashsum DropboxHash` (Nick Craig-Wood)
* New Features
    * Add `--header-download` and `--header-upload` flags for setting HTTP headers when uploading/downloading (Tim Gallant)
    * Add `--header` flag to add HTTP headers to every HTTP transaction (Nick Craig-Wood)
    * Add `--check-first` to do all checking before starting transfers (Nick Craig-Wood)
    * Add `--track-renames-strategy` for configurable matching criteria for `--track-renames` (Bernd Schoolmann)
    * Add `--cutoff-mode` hard,soft,catious (Shing Kit Chan & Franklyn Tackitt)
    * Filter flags (eg `--files-from -`) can read from stdin (fishbullet)
    * Add `--error-on-no-transfer` option (Jon Fautley)
    * Implement `--order-by xxx,mixed` for copying some small and some big files (Nick Craig-Wood)
    * Allow `--max-backlog` to be negative meaning as large as possible (Nick Craig-Wood)
    * Added `--no-unicode-normalization` flag to allow Unicode filenames to remain unique (Ben Zenker)
    * Allow `--min-age`/`--max-age` to take a date as well as a duration (Nick Craig-Wood)
    * Add rename statistics for file and directory renames (Nick Craig-Wood)
    * Add statistics output to JSON log (reddi)
    * Make stats be printed on non-zero exit code (Nick Craig-Wood)
    * When running `--password-command` allow use of stdin (Sébastien Gross)
    * Stop empty strings being a valid remote path (Nick Craig-Wood)
    * accounting: support WriterTo for less memory copying (Nick Craig-Wood)
    * build
        * Update to use go1.14 for the build (Nick Craig-Wood)
        * Add `-trimpath` to release build for reproduceable builds (Nick Craig-Wood)
        * Remove GOOS and GOARCH from Dockerfile (Brandon Philips)
    * config
        * Fsync the config file after writing to save more reliably (Nick Craig-Wood)
        * Add `--obscure` and `--no-obscure` flags to `config create`/`update` (Nick Craig-Wood)
        * Make `config show` take `remote:` as well as `remote` (Nick Craig-Wood)
    * copyurl: Add `--no-clobber` flag (Denis)
    * delete: Added `--rmdirs` flag to delete directories as well (Kush)
    * filter: Added `--files-from-raw` flag (Ankur Gupta)
    * genautocomplete: Add support for fish shell (Matan Rosenberg)
    * log: Add support for syslog LOCAL facilities (Patryk Jakuszew)
    * lsjson: Add `--hash-type` parameter and use it in lsf to speed up hashing (Nick Craig-Wood)
    * rc
        * Add `-o`/`--opt` and `-a`/`--arg` for more structured input (Nick Craig-Wood)
        * Implement `backend/command` for running backend specific commands remotely (Nick Craig-Wood)
        * Add `mount/mount` command for starting `rclone mount` via the API (Chaitanya)
    * rcd: Add Prometheus metrics support (Gary Kim)
    * serve http
        * Added a `--template` flag for user defined markup (calistri)
        * Add Last-Modified headers to files and directories (Nick Craig-Wood)
    * serve sftp: Add support for multiple host keys by repeating `--key` flag (Maxime Suret)
    * touch: Add `--localtime` flag to make `--timestamp` localtime not UTC (Nick Craig-Wood)
* Bug Fixes
    * accounting
        * Restore "Max number of stats groups reached" log line (Michał Matczuk)
        * Correct exitcode on Transfer Limit Exceeded flag. (Anuar Serdaliyev)
        * Reset bytes read during copy retry (Ankur Gupta)
        * Fix race clearing stats (Nick Craig-Wood)
    * copy: Only create empty directories when they don't exist on the remote (Ishuah Kariuki)
    * dedupe: Stop dedupe deleting files with identical IDs (Nick Craig-Wood)
    * oauth
        * Use custom http client so that `--no-check-certificate` is honored by oauth token fetch (Mark Spieth)
        * Replace deprecated oauth2.NoContext (Lars Lehtonen)
    * operations
        * Fix setting the timestamp on Windows for multithread copy (Nick Craig-Wood)
        * Make rcat obey `--ignore-checksum` (Nick Craig-Wood)
        * Make `--max-transfer` more accurate (Nick Craig-Wood)
    * rc
        * Fix dropped error (Lars Lehtonen)
        * Fix misplaced http server config (Xiaoxing Ye)
        * Disable duplicate log (ElonH)
    * serve dlna
        * Cds: don't specify childCount at all when unknown (Dan Walters)
        * Cds: use modification time as date in dlna metadata (Dan Walters)
    * serve restic: Fix tests after restic project removed vendoring (Nick Craig-Wood)
    * sync
        * Fix incorrect "nothing to transfer" message using `--delete-before` (Nick Craig-Wood)
        * Only create empty directories when they don't exist on the remote (Ishuah Kariuki)
* Mount
    * Add `--async-read` flag to disable asynchronous reads (Nick Craig-Wood)
    * Ignore `--allow-root` flag with a warning as it has been removed upstream (Nick Craig-Wood)
    * Warn if `--allow-non-empty` used on Windows and clarify docs (Nick Craig-Wood)
    * Constrain to go1.13 or above otherwise bazil.org/fuse fails to compile (Nick Craig-Wood)
    * Fix fail because of too long volume name (evileye)
    * Report 1PB free for unknown disk sizes (Nick Craig-Wood)
    * Map more rclone errors into file systems errors (Nick Craig-Wood)
    * Fix disappearing cwd problem (Nick Craig-Wood)
    * Use ReaddirPlus on Windows to improve directory listing performance (Nick Craig-Wood)
    * Send a hint as to whether the filesystem is case insensitive or not (Nick Craig-Wood)
    * Add rc command `mount/types` (Nick Craig-Wood)
    * Change maximum leaf name length to 1024 bytes (Nick Craig-Wood)
* VFS
    * Add `--vfs-read-wait` and `--vfs-write-wait` flags to control time waiting for a sequential read/write (Nick Craig-Wood)
    * Change default `--vfs-read-wait` to 20ms (it was 5ms and not configurable) (Nick Craig-Wood)
    * Make `df` output more consistent on a rclone mount. (Yves G)
    * Report 1PB free for unknown disk sizes (Nick Craig-Wood)
    * Fix race condition caused by unlocked reading of Dir.path (Nick Craig-Wood)
    * Make File lock and Dir lock not overlap to avoid deadlock (Nick Craig-Wood)
    * Implement lock ordering between File and Dir to eliminate deadlocks (Nick Craig-Wood)
    * Factor the vfs cache into its own package (Nick Craig-Wood)
    * Pin the Fs in use in the Fs cache (Nick Craig-Wood)
    * Add SetSys() methods to Node to allow caching stuff on a node (Nick Craig-Wood)
    * Ignore file not found errors from Hash in Read.Release (Nick Craig-Wood)
    * Fix hang in read wait code (Nick Craig-Wood)
* Local
    * Speed up multi thread downloads by using sparse files on Windows (Nick Craig-Wood)
    * Implement `--local-no-sparse` flag for disabling sparse files (Nick Craig-Wood)
    * Implement `rclone backend noop` for testing purposes (Nick Craig-Wood)
    * Fix "file not found" errors on post transfer Hash calculation (Nick Craig-Wood)
* Cache
    * Implement `rclone backend stats` command (Nick Craig-Wood)
    * Fix Server Side Copy with Temp Upload (Brandon McNama)
    * Remove Unused Functions (Lars Lehtonen)
    * Disable race tests until bbolt is fixed (Nick Craig-Wood)
    * Move methods used for testing into test file (greatroar)
    * Add Pin and Unpin and canonicalised lookup (Nick Craig-Wood)
    * Use proper import path go.etcd.io/bbolt (Robert-André Mauchin)
* Crypt
    * Calculate hashes for uploads from local disk (Nick Craig-Wood)
        * This allows crypted Jottacloud uploads without using local disk
        * This means crypted s3/b2 uploads will now have hashes
    * Added `rclone backend decode`/`encode` commands to replicate functionality of `cryptdecode` (Anagh Kumar Baranwal)
    * Get rid of the unused Cipher interface as it obfuscated the code (Nick Craig-Wood)
* Azure Blob
    * Implement streaming of unknown sized files so `rcat` is now supported (Nick Craig-Wood)
    * Implement memory pooling to control memory use (Nick Craig-Wood)
    * Add `--azureblob-disable-checksum` flag (Nick Craig-Wood)
    * Retry `InvalidBlobOrBlock` error as it may indicate block concurrency problems (Nick Craig-Wood)
    * Remove unused `Object.parseTimeString()` (Lars Lehtonen)
    * Fix permission error on SAS URL limited to container (Nick Craig-Wood)
* B2
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Ignore directory markers at the root also (Nick Craig-Wood)
    * Force the case of the SHA1 to lowercase (Nick Craig-Wood)
    * Remove unused `largeUpload.clearUploadURL()` (Lars Lehtonen)
* Box
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Implement About to read size used (Nick Craig-Wood)
    * Add token renew function for jwt auth (David Bramwell)
    * Added support for interchangeable root folder for Box backend (Sunil Patra)
    * Remove unnecessary iat from jws claims (David)
* Drive
    * Follow shortcuts by default, skip with `--drive-skip-shortcuts` (Nick Craig-Wood)
    * Implement `rclone backend shortcut` command for creating shortcuts (Nick Craig-Wood)
    * Added `rclone backend` command to change `service_account_file` and `chunk_size` (Anagh Kumar Baranwal)
    * Fix missing files when using `--fast-list` and `--drive-shared-with-me` (Nick Craig-Wood)
    * Fix duplicate items when using `--drive-shared-with-me` (Nick Craig-Wood)
    * Extend `--drive-stop-on-upload-limit` to respond to `teamDriveFileLimitExceeded`. (harry)
    * Don't delete files with multiple parents to avoid data loss (Nick Craig-Wood)
    * Server side copy docs use default description if empty (Nick Craig-Wood)
* Dropbox
    * Make error insufficient space to be fatal (harry)
    * Add info about required redirect url (Elan Ruusamäe)
* Fichier
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Implement custom pacer to deal with the new rate limiting (buengese)
* FTP
    * Fix lockup when using concurrency limit on failed connections (Nick Craig-Wood)
    * Fix lockup on failed upload when using concurrency limit (Nick Craig-Wood)
    * Fix lockup on Close failures when using concurrency limit (Nick Craig-Wood)
    * Work around pureftp sending spurious 150 messages (Nick Craig-Wood)
* Google Cloud Storage
    * Add support for `--header-upload` and `--header-download` (Nick Craig-Wood)
    * Add `ARCHIVE` storage class to help (Adam Stroud)
    * Ignore directory markers at the root (Nick Craig-Wood)
* Googlephotos
    * Make the start year configurable (Daven)
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Create feature/favorites directory (Brandon Philips)
    * Fix "concurrent map write" error (Nick Craig-Wood)
    * Don't put an image in error message (Nick Craig-Wood)
* HTTP
    * Improved directory listing with new template from Caddy project (calisro)
* Jottacloud
    * Implement `--jottacloud-trashed-only` (buengese)
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Use `RawURLEncoding` when decoding base64 encoded login token (buengese)
    * Implement cleanup (buengese)
    * Update docs regarding cleanup, removed remains from old auth, and added warning about special mountpoints. (albertony)
* Mailru
    * Describe 2FA requirements (valery1707)
* Onedrive
    * Implement `--onedrive-server-side-across-configs` (Nick Craig-Wood)
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Fix occasional 416 errors on multipart uploads (Nick Craig-Wood)
    * Added maximum chunk size limit warning in the docs (Harry)
    * Fix missing drive on config (Nick Craig-Wood)
    * Make error `quotaLimitReached` to be fatal (harry)
* Opendrive
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
* Pcloud
    * Added support for interchangeable root folder for pCloud backend (Sunil Patra)
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Fix initial config "Auth state doesn't match" message (Nick Craig-Wood)
* Premiumizeme
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Prune unused functions (Lars Lehtonen)
* Putio
    * Add support for `--header-upload` and `--header-download` (Nick Craig-Wood)
    * Make downloading files use the rclone http Client (Nick Craig-Wood)
    * Fix parsing of remotes with leading and trailing / (Nick Craig-Wood)
* Qingstor
    * Make `rclone cleanup` remove pending multipart uploads older than 24h (Nick Craig-Wood)
    * Try harder to cancel failed multipart uploads (Nick Craig-Wood)
    * Prune `multiUploader.list()` (Lars Lehtonen)
    * Lint fix (Lars Lehtonen)
* S3
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Use memory pool for buffer allocations (Maciej Zimnoch)
    * Add SSE-C support for AWS, Ceph, and MinIO (Jack Anderson)
    * Fail fast multipart upload (Michał Matczuk)
    * Report errors on bucket creation (mkdir) correctly (Nick Craig-Wood)
    * Specify that Minio supports URL encoding in listings (Nick Craig-Wood)
    * Added 500 as retryErrorCode (Michał Matczuk)
    * Use `--low-level-retries` as the number of SDK retries (Aleksandar Janković)
    * Fix multipart abort context (Aleksandar Jankovic)
    * Replace deprecated `session.New()` with `session.NewSession()` (Lars Lehtonen)
    * Use the provided size parameter when allocating a new memory pool (Joachim Brandon LeBlanc)
    * Use rclone's low level retries instead of AWS SDK to fix listing retries (Nick Craig-Wood)
    * Ignore directory markers at the root also (Nick Craig-Wood)
    * Use single memory pool (Michał Matczuk)
    * Do not resize buf on put to memBuf (Michał Matczuk)
    * Improve docs for `--s3-disable-checksum` (Nick Craig-Wood)
    * Don't leak memory or tokens in edge cases for multipart upload (Nick Craig-Wood)
* Seafile
    * Implement 2FA (Fred)
* SFTP
    * Added `--sftp-pem-key` to support inline key files (calisro)
    * Fix post transfer copies failing with 0 size when using `set_modtime=false` (Nick Craig-Wood)
* Sharefile
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
* Sugarsync
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
* Swift
    * Add support for `--header-upload` and `--header-download` (Nick Craig-Wood)
    * Fix cosmetic issue in error message (Martin Michlmayr)
* Union
    * Implement multiple writable remotes (Max Sum)
    * Fix server-side copy (Max Sum)
    * Implement ListR (Max Sum)
    * Enable ListR when upstreams contain local (Max Sum)
* WebDAV
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)
    * Fix `X-OC-Mtime` header for Transip compatibility (Nick Craig-Wood)
    * Report full and consistent usage with `about` (Yves G)
* Yandex
    * Add support for `--header-upload` and `--header-download` (Tim Gallant)

## v1.51.0 - 2020-02-01

* New backends
    * [Memory](https://rclone.org/memory/) (Nick Craig-Wood)
    * [Sugarsync](https://rclone.org/sugarsync/) (Nick Craig-Wood)
* New Features
    * Adjust all backends to have `--backend-encoding` parameter (Nick Craig-Wood)
        * this enables the encoding for special characters to be adjusted or disabled
    * Add `--max-duration` flag to control the maximum duration of a transfer session (boosh)
    * Add `--expect-continue-timeout` flag, default 1s (Nick Craig-Wood)
    * Add `--no-check-dest` flag for copying without testing the destination (Nick Craig-Wood)
    * Implement `--order-by` flag to order transfers (Nick Craig-Wood)
    * accounting
        * Don't show entries in both transferring and checking (Nick Craig-Wood)
        * Add option to delete stats (Aleksandar Jankovic)
    * build
        * Compress the test builds with gzip (Nick Craig-Wood)
        * Implement a framework for starting test servers during tests (Nick Craig-Wood)
    * cmd: Always print elapsed time to tenth place seconds in progress (Gary Kim)
    * config
        * Add `--password-command` to allow dynamic config password (Damon Permezel)
        * Give config questions default values (Nick Craig-Wood)
        * Check a remote exists when creating a new one (Nick Craig-Wood)
    * copyurl: Add `--stdout` flag to write to stdout (Nick Craig-Wood)
    * dedupe: Implement keep smallest too (Nick Craig-Wood)
    * hashsum: Add flag `--base64` flag (landall)
    * lsf: Speed up on s3/swift/etc by not reading mimetype by default (Nick Craig-Wood)
    * lsjson: Add `--no-mimetype` flag (Nick Craig-Wood)
    * rc: Add methods to turn on blocking and mutex profiling (Nick Craig-Wood)
    * rcd
        * Adding group parameter to stats (Chaitanya)
        * Move webgui apart; option to disable browser (Xiaoxing Ye)
    * serve sftp: Add support for public key with auth proxy (Paul Tinsley)
    * stats: Show deletes in stats and hide zero stats (anuar45)
* Bug Fixes
    * accounting
        * Fix error counter counting multiple times (Ankur Gupta)
        * Fix error count shown as checks (Cnly)
        * Clear finished transfer in stats-reset (Maciej Zimnoch)
        * Added StatsInfo locking in statsGroups sum function (Michał Matczuk)
    * asyncreader: Fix EOF error (buengese)
    * check: Fix `--one-way` recursing more directories than it needs to (Nick Craig-Wood)
    * chunkedreader: Disable hash calculation for first segment (Nick Craig-Wood)
    * config
        * Do not open browser on headless on drive/gcs/google photos (Xiaoxing Ye)
        * SetValueAndSave ignore error if config section does not exist yet (buengese)
    * cmd: Fix completion with an encrypted config (Danil Semelenov)
    * dbhashsum: Stop it returning UNSUPPORTED on dropbox (Nick Craig-Wood)
    * dedupe: Add missing modes to help string (Nick Craig-Wood)
    * operations
        * Fix dedupe continuing on errors like insufficientFilePermisson (SezalAgrawal)
        * Clear accounting before low level retry (Maciej Zimnoch)
        * Write debug message when hashes could not be checked (Ole Schütt)
        * Move interface assertion to tests to remove pflag dependency (Nick Craig-Wood)
        * Make NewOverrideObjectInfo public and factor uses (Nick Craig-Wood)
    * proxy: Replace use of bcrypt with sha256 (Nick Craig-Wood)
    * vendor
        * Update bazil.org/fuse to fix FreeBSD 12.1 (Nick Craig-Wood)
        * Update github.com/t3rm1n4l/go-mega to fix mega "illegal base64 data at input byte 22" (Nick Craig-Wood)
        * Update termbox-go to fix ncdu command on FreeBSD (Kuang-che Wu)
        * Update t3rm1n4l/go-mega - fixes mega: couldn't login: crypto/aes: invalid key size 0 (Nick Craig-Wood)
* Mount
    * Enable async reads for a 20% speedup (Nick Craig-Wood)
    * Replace use of WriteAt with Write for cache mode >= writes and O_APPEND (Brett Dutro)
    * Make sure we call unmount when exiting (Nick Craig-Wood)
    * Don't build on go1.10 as bazil/fuse no longer supports it (Nick Craig-Wood)
    * When setting dates discard out of range dates (Nick Craig-Wood)
* VFS
    * Add a newly created file straight into the directory (Nick Craig-Wood)
    * Only calculate one hash for reads for a speedup (Nick Craig-Wood)
    * Make ReadAt for non cached files work better with non-sequential reads (Nick Craig-Wood)
    * Fix edge cases when reading ModTime from file (Nick Craig-Wood)
    * Make sure existing files opened for write show correct size (Nick Craig-Wood)
    * Don't cache the path in RW file objects to fix renaming (Nick Craig-Wood)
    * Fix rename of open files when using the VFS cache (Nick Craig-Wood)
    * When renaming files in the cache, rename the cache item in memory too (Nick Craig-Wood)
    * Fix open file renaming on drive when using `--vfs-cache-mode writes` (Nick Craig-Wood)
    * Fix incorrect modtime for mv into mount with `--vfs-cache-modes writes` (Nick Craig-Wood)
    * On rename, rename in cache too if the file exists (Anagh Kumar Baranwal)
* Local
    * Make source file being updated errors be NoLowLevelRetry errors (Nick Craig-Wood)
    * Fix update of hidden files on Windows (Nick Craig-Wood)
* Cache
    * Follow move of upstream library github.com/coreos/bbolt github.com/etcd-io/bbolt (Nick Craig-Wood)
    * Fix `fatal error: concurrent map writes` (Nick Craig-Wood)
* Crypt
    * Reorder the filename encryption options (Thomas Eales)
    * Correctly handle trailing dot (buengese)
* Chunker
    * Reduce length of temporary suffix (Ivan Andreev)
* Drive
    * Add `--drive-stop-on-upload-limit` flag to stop syncs when upload limit reached (Nick Craig-Wood)
    * Add `--drive-use-shared-date` to use date file was shared instead of modified date (Garry McNulty)
    * Make sure invalid auth for teamdrives always reports an error (Nick Craig-Wood)
    * Fix `--fast-list` when using appDataFolder (Nick Craig-Wood)
    * Use multipart resumable uploads for streaming and uploads in mount (Nick Craig-Wood)
    * Log an ERROR if an incomplete search is returned (Nick Craig-Wood)
    * Hide dangerous config from the configurator (Nick Craig-Wood)
* Dropbox
    * Treat `insufficient_space` errors as non retriable errors (Nick Craig-Wood)
* Jottacloud
    * Use new auth method used by official client (buengese)
    * Add URL to generate Login Token to config wizard (Nick Craig-Wood)
    * Add support whitelabel versions (buengese)
* Koofr
    * Use rclone HTTP client. (jaKa)
* Onedrive
    * Add Sites.Read.All permission (Benjamin Richter)
    * Add support "Retry-After" header (Motonori IWAMURO)
* Opendrive
    * Implement `--opendrive-chunk-size` (Nick Craig-Wood)
* S3
    * Re-implement multipart upload to fix memory issues (Nick Craig-Wood)
    * Add `--s3-copy-cutoff` for size to switch to multipart copy (Nick Craig-Wood)
    * Add new region Asia Patific (Hong Kong) (Outvi V)
    * Reduce memory usage streaming files by reducing max stream upload size (Nick Craig-Wood)
    * Add `--s3-list-chunk` option for bucket listing (Thomas Kriechbaumer)
    * Force path style bucket access to off for AWS deprecation (Nick Craig-Wood)
    * Use AWS web identity role provider if available (Tennix)
    * Add StackPath Object Storage Support (Dave Koston)
    * Fix ExpiryWindow value (Aleksandar Jankovic)
    * Fix DisableChecksum condition (Aleksandar Janković)
    * Fix URL decoding of NextMarker (Nick Craig-Wood)
* SFTP
    * Add `--sftp-skip-links` to skip symlinks and non regular files (Nick Craig-Wood)
    * Retry Creation of Connection (Sebastian Brandt)
    * Fix "failed to parse private key file: ssh: not an encrypted key" error (Nick Craig-Wood)
    * Open files for update write only to fix AWS SFTP interop (Nick Craig-Wood)
* Swift
    * Reserve segments of dynamic large object when delete objects in container what was enabled versioning. (Nguyễn Hữu Luân)
    * Fix parsing of X-Object-Manifest (Nick Craig-Wood)
    * Update OVH API endpoint (unbelauscht)
* WebDAV
    * Make nextcloud only upload SHA1 checksums (Nick Craig-Wood)
    * Fix case of "Bearer" in Authorization: header to agree with RFC (Nick Craig-Wood)
    * Add Referer header to fix problems with WAFs (Nick Craig-Wood)

## v1.50.2 - 2019-11-19

* Bug Fixes
    * accounting: Fix memory leak on retries operations (Nick Craig-Wood)
* Drive
    * Fix listing of the root directory with drive.files scope (Nick Craig-Wood)
    * Fix --drive-root-folder-id with team/shared drives (Nick Craig-Wood)

## v1.50.1 - 2019-11-02

* Bug Fixes
    * hash: Fix accidentally changed hash names for `DropboxHash` and `CRC-32` (Nick Craig-Wood)
    * fshttp: Fix error reporting on tpslimit token bucket errors (Nick Craig-Wood)
    * fshttp: Don't print token bucket errors on context cancelled (Nick Craig-Wood)
* Local
    * Fix listings of . on Windows (Nick Craig-Wood)
* Onedrive
    * Fix DirMove/Move after Onedrive change (Xiaoxing Ye)

## v1.50.0 - 2019-10-26

* New backends
    * [Citrix Sharefile](https://rclone.org/sharefile/) (Nick Craig-Wood)
    * [Chunker](https://rclone.org/chunker/) - an overlay backend to split files into smaller parts (Ivan Andreev)
    * [Mail.ru Cloud](https://rclone.org/mailru/) (Ivan Andreev)
* New Features
    * encodings (Fabian Möller & Nick Craig-Wood)
        * All backends now use file name encoding to ensure any file name can be written to any backend.
        * See the [restricted file name docs](https://rclone.org/overview/#restricted-filenames) for more info and the [local backend docs](/local/#filenames).
        * Some file names may look different in rclone if you are using any control characters in names or [unicode FULLWIDTH symbols](https://en.wikipedia.org/wiki/Halfwidth_and_Fullwidth_Forms_(Unicode_block)).
    * build
        * Update to use go1.13 for the build (Nick Craig-Wood)
        * Drop support for go1.9 (Nick Craig-Wood)
        * Build rclone with GitHub actions (Nick Craig-Wood)
        * Convert python scripts to python3 (Nick Craig-Wood)
        * Swap Azure/go-ansiterm for mattn/go-colorable (Nick Craig-Wood)
        * Dockerfile fixes (Matei David)
        * Add [plugin support](https://github.com/rclone/rclone/blob/master/CONTRIBUTING.md#writing-a-plugin) for backends and commands (Richard Patel)
    * config
        * Use alternating Red/Green in config to make more obvious (Nick Craig-Wood)
    * contrib
        * Add sample DLNA server Docker Compose manifest. (pataquets)
        * Add sample WebDAV server Docker Compose manifest. (pataquets)
    * copyurl
        * Add `--auto-filename` flag for using file name from URL in destination path (Denis)
    * serve dlna:
        * Many compatibility improvements (Dan Walters)
        * Support for external srt subtitles (Dan Walters)
    * rc
        * Added command core/quit (Saksham Khanna)
* Bug Fixes
    * sync
        * Make `--update`/`-u` not transfer files that haven't changed (Nick Craig-Wood)
        * Free objects after they come out of the transfer pipe to save memory (Nick Craig-Wood)
        * Fix `--files-from without --no-traverse` doing a recursive scan (Nick Craig-Wood)
    * operations
        * Fix accounting for server side copies (Nick Craig-Wood)
        * Display 'All duplicates removed' only if dedupe successful (Sezal Agrawal)
        * Display 'Deleted X extra copies' only if dedupe successful (Sezal Agrawal)
    * accounting
        * Only allow up to 100 completed transfers in the accounting list to save memory (Nick Craig-Wood)
        * Cull the old time ranges when possible to save memory (Nick Craig-Wood)
        * Fix panic due to server-side copy fallback (Ivan Andreev)
        * Fix memory leak noticeable for transfers of large numbers of objects (Nick Craig-Wood)
        * Fix total duration calculation (Nick Craig-Wood)
    * cmd
        * Fix environment variables not setting command line flags (Nick Craig-Wood)
        * Make autocomplete compatible with bash's posix mode for macOS (Danil Semelenov)
        * Make `--progress` work in git bash on Windows (Nick Craig-Wood)
        * Fix 'compopt: command not found' on autocomplete on macOS (Danil Semelenov)
    * config
        * Fix setting of non top level flags from environment variables (Nick Craig-Wood)
        * Check config names more carefully and report errors (Nick Craig-Wood)
        * Remove error: can't use `--size-only` and `--ignore-size` together. (Nick Craig-Wood)
    * filter: Prevent mixing options when `--files-from` is in use (Michele Caci)
    * serve sftp: Fix crash on unsupported operations (eg Readlink) (Nick Craig-Wood)
* Mount
    * Allow files of unknown size to be read properly (Nick Craig-Wood)
    * Skip tests on <= 2 CPUs to avoid lockup (Nick Craig-Wood)
    * Fix panic on File.Open (Nick Craig-Wood)
    * Fix "mount_fusefs: -o timeout=: option not supported" on FreeBSD (Nick Craig-Wood)
    * Don't pass huge filenames (>4k) to FUSE as it can't cope (Nick Craig-Wood)
* VFS
    * Add flag `--vfs-case-insensitive` for windows/macOS mounts (Ivan Andreev)
    * Make objects of unknown size readable through the VFS (Nick Craig-Wood)
    * Move writeback of dirty data out of close() method into its own method (FlushWrites) and remove close() call from Flush() (Brett Dutro)
    * Stop empty dirs disappearing when renamed on bucket based remotes (Nick Craig-Wood)
    * Stop change notify polling clearing so much of the directory cache (Nick Craig-Wood)
* Azure Blob
    * Disable logging to the Windows event log (Nick Craig-Wood)
* B2
    * Remove `unverified:` prefix on sha1 to improve interop (eg with CyberDuck) (Nick Craig-Wood)
* Box
    * Add options to get access token via JWT auth (David)
* Drive
    * Disable HTTP/2 by default to work around INTERNAL_ERROR problems (Nick Craig-Wood)
    * Make sure that drive root ID is always canonical (Nick Craig-Wood)
    * Fix `--drive-shared-with-me` from the root with lsand `--fast-list` (Nick Craig-Wood)
    * Fix ChangeNotify polling for shared drives (Nick Craig-Wood)
    * Fix change notify polling when using appDataFolder (Nick Craig-Wood)
* Dropbox
    * Make disallowed filenames errors not retry (Nick Craig-Wood)
    * Fix nil pointer exception on restricted files (Nick Craig-Wood)
* Fichier
    * Fix accessing files > 2GB on 32 bit systems (Nick Craig-Wood)
* FTP
    * Allow disabling EPSV mode (Jon Fautley)
* HTTP
    * HEAD directory entries in parallel to speedup (Nick Craig-Wood)
    * Add `--http-no-head` to stop rclone doing HEAD in listings (Nick Craig-Wood)
* Putio
    * Add ability to resume uploads (Cenk Alti)
* S3
    * Fix signature v2_auth headers (Anthony Rusdi)
    * Fix encoding for control characters (Nick Craig-Wood)
    * Only ask for URL encoded directory listings if we need them on Ceph (Nick Craig-Wood)
    * Add option for multipart failure behaviour (Aleksandar Jankovic)
    * Support for multipart copy (庄天翼)
    * Fix nil pointer reference if no metadata returned for object (Nick Craig-Wood)
* SFTP
    * Fix `--sftp-ask-password` trying to contact the ssh agent (Nick Craig-Wood)
    * Fix hashes of files with backslashes (Nick Craig-Wood)
    * Include more ciphers with `--sftp-use-insecure-cipher` (Carlos Ferreyra)
* WebDAV
    * Parse and return Sharepoint error response (Henning Surmeier)

## v1.49.5 - 2019-10-05

* Bug Fixes
    * Revert back to go1.12.x for the v1.49.x builds as go1.13.x was causing issues (Nick Craig-Wood)
    * Fix rpm packages by using master builds of nfpm (Nick Craig-Wood)
    * Fix macOS build after brew changes (Nick Craig-Wood)

## v1.49.4 - 2019-09-29

* Bug Fixes
    * cmd/rcd: Address ZipSlip vulnerability (Richard Patel)
    * accounting: Fix file handle leak on errors (Nick Craig-Wood)
    * oauthutil: Fix security problem when running with two users on the same machine (Nick Craig-Wood)
* FTP
    * Fix listing of an empty root returning: error dir not found (Nick Craig-Wood)
* S3
    * Fix SetModTime on GLACIER/ARCHIVE objects and implement set/get tier (Nick Craig-Wood)

## v1.49.3 - 2019-09-15

* Bug Fixes
    * accounting
        * Fix total duration calculation (Aleksandar Jankovic)
        * Fix "file already closed" on transfer retries (Nick Craig-Wood)

## v1.49.2 - 2019-09-08

* New Features
    * build: Add Docker workflow support (Alfonso Montero)
* Bug Fixes
    * accounting: Fix locking in Transfer to avoid deadlock with `--progress` (Nick Craig-Wood)
    * docs: Fix template argument for mktemp in install.sh (Cnly)
    * operations: Fix `-u`/`--update` with google photos / files of unknown size (Nick Craig-Wood)
    * rc: Fix docs for config/create /update /password (Nick Craig-Wood)
* Google Cloud Storage
    * Fix need for elevated permissions on SetModTime (Nick Craig-Wood)

## v1.49.1 - 2019-08-28

* Bug Fixes
    * config: Fix generated passwords being stored as empty password (Nick Craig-Wood)
    * rcd: Added missing parameter for web-gui info logs. (Chaitanya)
* Googlephotos
    * Fix crash on error response (Nick Craig-Wood)
* Onedrive
    * Fix crash on error response (Nick Craig-Wood)

## v1.49.0 - 2019-08-26

* New backends
    * [1fichier](https://rclone.org/fichier/) (Laura Hausmann)
    * [Google Photos](https://rclone.org/googlephotos/) (Nick Craig-Wood)
    * [Putio](https://rclone.org/putio/) (Cenk Alti)
    * [premiumize.me](https://rclone.org/premiumizeme/) (Nick Craig-Wood)
* New Features
    * Experimental [web GUI](https://rclone.org/gui/) (Chaitanya Bankanhal)
    * Implement `--compare-dest` & `--copy-dest` (yparitcher)
    * Implement `--suffix` without `--backup-dir` for backup to current dir (yparitcher)
    * `config reconnect` to re-login (re-run the oauth login) for the backend. (Nick Craig-Wood)
    * `config userinfo` to discover which user you are logged in as. (Nick Craig-Wood)
    * `config disconnect` to disconnect you (log out) from the backend. (Nick Craig-Wood)
    * Add `--use-json-log` for JSON logging (justinalin)
    * Add context propagation to rclone (Aleksandar Jankovic)
    * Reworking internal statistics interfaces so they work with rc jobs (Aleksandar Jankovic)
    * Add Higher units for ETA (AbelThar)
    * Update rclone logos to new design (Andreas Chlupka)
    * hash: Add CRC-32 support (Cenk Alti)
    * help showbackend: Fixed advanced option category when there are no standard options (buengese)
    * ncdu: Display/Copy to Clipboard Current Path (Gary Kim)
    * operations:
        * Run hashing operations in parallel (Nick Craig-Wood)
        * Don't calculate checksums when using `--ignore-checksum` (Nick Craig-Wood)
        * Check transfer hashes when using `--size-only` mode (Nick Craig-Wood)
        * Disable multi thread copy for local to local copies (Nick Craig-Wood)
        * Debug successful hashes as well as failures (Nick Craig-Wood)
    * rc
        * Add ability to stop async jobs (Aleksandar Jankovic)
        * Return current settings if core/bwlimit called without parameters (Nick Craig-Wood)
        * Rclone-WebUI integration with rclone (Chaitanya Bankanhal)
        * Added command line parameter to control the cross origin resource sharing (CORS) in the rcd. (Security Improvement) (Chaitanya Bankanhal)
        * Add anchor tags to the docs so links are consistent (Nick Craig-Wood)
        * Remove _async key from input parameters after parsing so later operations won't get confused (buengese)
        * Add call to clear stats (Aleksandar Jankovic)
    * rcd
        * Auto-login for web-gui (Chaitanya Bankanhal)
        * Implement `--baseurl` for rcd and web-gui (Chaitanya Bankanhal)
    * serve dlna
        * Only select interfaces which can multicast for SSDP (Nick Craig-Wood)
        * Add more builtin mime types to cover standard audio/video (Nick Craig-Wood)
        * Fix missing mime types on Android causing missing videos (Nick Craig-Wood)
    * serve ftp
        * Refactor to bring into line with other serve commands (Nick Craig-Wood)
        * Implement `--auth-proxy` (Nick Craig-Wood)
    * serve http: Implement `--baseurl` (Nick Craig-Wood)
    * serve restic: Implement `--baseurl` (Nick Craig-Wood)
    * serve sftp
        * Implement auth proxy (Nick Craig-Wood)
        * Fix detection of whether server is authorized (Nick Craig-Wood)
    * serve webdav
        * Implement `--baseurl` (Nick Craig-Wood)
        * Support `--auth-proxy` (Nick Craig-Wood)
* Bug Fixes
    * Make "bad record MAC" a retriable error (Nick Craig-Wood)
    * copyurl: Fix copying files that return HTTP errors (Nick Craig-Wood)
    * march: Fix checking sub-directories when using `--no-traverse` (buengese)
    * rc
        * Fix unmarshalable http.AuthFn in options and put in test for marshalability (Nick Craig-Wood)
        * Move job expire flags to rc to fix initialization problem (Nick Craig-Wood)
        * Fix `--loopback` with rc/list and others (Nick Craig-Wood)
    * rcat: Fix slowdown on systems with multiple hashes (Nick Craig-Wood)
    * rcd: Fix permissions problems on cache directory with web gui download (Nick Craig-Wood)
* Mount
    * Default `--deamon-timout` to 15 minutes on macOS and FreeBSD (Nick Craig-Wood)
    * Update docs to show mounting from root OK for bucket based (Nick Craig-Wood)
    * Remove nonseekable flag from write files (Nick Craig-Wood)
* VFS
    * Make write without cache more efficient (Nick Craig-Wood)
    * Fix `--vfs-cache-mode minimal` and `writes` ignoring cached files (Nick Craig-Wood)
* Local
    * Add `--local-case-sensitive` and `--local-case-insensitive` (Nick Craig-Wood)
    * Avoid polluting page cache when uploading local files to remote backends (Michał Matczuk)
    * Don't calculate any hashes by default (Nick Craig-Wood)
    * Fadvise run syscall on a dedicated go routine (Michał Matczuk)
* Azure Blob
    * Azure Storage Emulator support (Sandeep)
    * Updated config help details to remove connection string references (Sandeep)
    * Make all operations work from the root (Nick Craig-Wood)
* B2
    * Implement link sharing (yparitcher)
    * Enable server side copy to copy between buckets (Nick Craig-Wood)
    * Make all operations work from the root (Nick Craig-Wood)
* Drive
    * Fix server side copy of big files (Nick Craig-Wood)
    * Update API for teamdrive use (Nick Craig-Wood)
    * Add error for purge with `--drive-trashed-only` (ginvine)
* Fichier
    * Make FolderID int and adjust related code (buengese)
* Google Cloud Storage
    * Reduce oauth scope requested as suggested by Google (Nick Craig-Wood)
    * Make all operations work from the root (Nick Craig-Wood)
* HTTP
    * Add `--http-headers` flag for setting arbitrary headers (Nick Craig-Wood)
* Jottacloud
    * Use new api for retrieving internal username (buengese)
    * Refactor configuration and minor cleanup (buengese)
* Koofr
    * Support setting modification times on Koofr backend. (jaKa)
* Opendrive
    * Refactor to use existing lib/rest facilities for uploads (Nick Craig-Wood)
* Qingstor
    * Upgrade to v3 SDK and fix listing loop (Nick Craig-Wood)
    * Make all operations work from the root (Nick Craig-Wood)
* S3
    * Add INTELLIGENT_TIERING storage class (Matti Niemenmaa)
    * Make all operations work from the root (Nick Craig-Wood)
* SFTP
    * Add missing interface check and fix About (Nick Craig-Wood)
    * Completely ignore all modtime checks if SetModTime=false (Jon Fautley)
    * Support md5/sha1 with rsync.net (Nick Craig-Wood)
    * Save the md5/sha1 command in use to the config file for efficiency (Nick Craig-Wood)
    * Opt-in support for diffie-hellman-group-exchange-sha256 diffie-hellman-group-exchange-sha1 (Yi FU)
* Swift
    * Use FixRangeOption to fix 0 length files via the VFS (Nick Craig-Wood)
    * Fix upload when using no_chunk to return the correct size (Nick Craig-Wood)
    * Make all operations work from the root (Nick Craig-Wood)
    * Fix segments leak during failed large file uploads. (nguyenhuuluan434)
* WebDAV
    * Add `--webdav-bearer-token-command` (Nick Craig-Wood)
    * Refresh token when it expires with `--webdav-bearer-token-command` (Nick Craig-Wood)
    * Add docs for using bearer_token_command with oidc-agent (Paul Millar)

## v1.48.0 - 2019-06-15

* New commands
    * serve sftp: Serve an rclone remote over SFTP (Nick Craig-Wood)
* New Features
    * Multi threaded downloads to local storage (Nick Craig-Wood)
        * controlled with `--multi-thread-cutoff` and `--multi-thread-streams`
    * Use rclone.conf from rclone executable directory to enable portable use (albertony)
    * Allow sync of a file and a directory with the same name (forgems)
        * this is common on bucket based remotes, eg s3, gcs
    * Add `--ignore-case-sync` for forced case insensitivity (garry415)
    * Implement `--stats-one-line-date` and `--stats-one-line-date-format` (Peter Berbec)
    * Log an ERROR for all commands which exit with non-zero status (Nick Craig-Wood)
    * Use go-homedir to read the home directory more reliably (Nick Craig-Wood)
    * Enable creating encrypted config through external script invocation (Wojciech Smigielski)
    * build: Drop support for go1.8 (Nick Craig-Wood)
    * config: Make config create/update encrypt passwords where necessary (Nick Craig-Wood)
    * copyurl: Honor `--no-check-certificate` (Stefan Breunig)
    * install: Linux skip man pages if no mandb (didil)
    * lsf: Support showing the Tier of the object (Nick Craig-Wood)
    * lsjson
        * Added EncryptedPath to output (calisro)
        * Support showing the Tier of the object (Nick Craig-Wood)
        * Add IsBucket field for bucket based remote listing of the root (Nick Craig-Wood)
    * rc
        * Add `--loopback` flag to run commands directly without a server (Nick Craig-Wood)
        * Add operations/fsinfo: Return information about the remote (Nick Craig-Wood)
        * Skip auth for OPTIONS request (Nick Craig-Wood)
        * cmd/providers: Add DefaultStr, ValueStr and Type fields (Nick Craig-Wood)
        * jobs: Make job expiry timeouts configurable (Aleksandar Jankovic)
    * serve dlna reworked and improved (Dan Walters)
    * serve ftp: add `--ftp-public-ip` flag to specify public IP (calistri)
    * serve restic: Add support for `--private-repos` in `serve restic` (Florian Apolloner)
    * serve webdav: Combine serve webdav and serve http (Gary Kim)
    * size: Ignore negative sizes when calculating total (Garry McNulty)
* Bug Fixes
    * Make move and copy individual files obey `--backup-dir` (Nick Craig-Wood)
    * If `--ignore-checksum` is in effect, don't calculate checksum (Nick Craig-Wood)
    * moveto: Fix case-insensitive same remote move (Gary Kim)
    * rc: Fix serving bucket based objects with `--rc-serve` (Nick Craig-Wood)
    * serve webdav: Fix serveDir not being updated with changes from webdav (Gary Kim)
* Mount
    * Fix poll interval documentation (Animosity022)
* VFS
    * Make WriteAt for non cached files work with non-sequential writes (Nick Craig-Wood)
* Local
    * Only calculate the required hashes for big speedup (Nick Craig-Wood)
    * Log errors when listing instead of returning an error (Nick Craig-Wood)
    * Fix preallocate warning on Linux with ZFS (Nick Craig-Wood)
* Crypt
    * Make rclone dedupe work through crypt (Nick Craig-Wood)
    * Fix wrapping of ChangeNotify to decrypt directories properly (Nick Craig-Wood)
    * Support PublicLink (rclone link) of underlying backend (Nick Craig-Wood)
    * Implement Optional methods SetTier, GetTier (Nick Craig-Wood)
* B2
    * Implement server side copy (Nick Craig-Wood)
    * Implement SetModTime (Nick Craig-Wood)
* Drive
    * Fix move and copy from TeamDrive to GDrive (Fionera)
    * Add notes that cleanup works in the background on drive (Nick Craig-Wood)
    * Add `--drive-server-side-across-configs` to default back to old server side copy semantics by default (Nick Craig-Wood)
    * Add `--drive-size-as-quota` to show storage quota usage for file size (Garry McNulty)
* FTP
    * Add FTP List timeout (Jeff Quinn)
    * Add FTP over TLS support (Gary Kim)
    * Add `--ftp-no-check-certificate` option for FTPS (Gary Kim)
* Google Cloud Storage
    * Fix upload errors when uploading pre 1970 files (Nick Craig-Wood)
* Jottacloud
    * Add support for selecting device and mountpoint. (buengese)
* Mega
    * Add cleanup support (Gary Kim)
* Onedrive
    * More accurately check if root is found (Cnly)
* S3
    * Support S3 Accelerated endpoints with `--s3-use-accelerate-endpoint` (Nick Craig-Wood)
    * Add config info for Wasabi's EU Central endpoint (Robert Marko)
    * Make SetModTime work for GLACIER while syncing (Philip Harvey)
* SFTP
    * Add About support (Gary Kim)
    * Fix about parsing of `df` results so it can cope with -ve results (Nick Craig-Wood)
    * Send custom client version and debug server version (Nick Craig-Wood)
* WebDAV
    * Retry on 423 Locked errors (Nick Craig-Wood)

## v1.47.0 - 2019-04-13

* New backends
    * Backend for Koofr cloud storage service. (jaKa)
* New Features
    * Resume downloads if the reader fails in copy (Nick Craig-Wood)
        * this means rclone will restart transfers if the source has an error
        * this is most useful for downloads or cloud to cloud copies
    * Use `--fast-list` for listing operations where it won't use more memory (Nick Craig-Wood)
        * this should speed up the following operations on remotes which support `ListR`
        * `dedupe`, `serve restic` `lsf`, `ls`, `lsl`, `lsjson`, `lsd`, `md5sum`, `sha1sum`, `hashsum`, `size`, `delete`, `cat`, `settier`
        * use `--disable ListR` to get old behaviour if required
    * Make `--files-from` traverse the destination unless `--no-traverse` is set (Nick Craig-Wood)
        * this fixes `--files-from` with Google drive and excessive API use in general.
    * Make server side copy account bytes and obey `--max-transfer` (Nick Craig-Wood)
    * Add `--create-empty-src-dirs` flag and default to not creating empty dirs (ishuah)
    * Add client side TLS/SSL flags `--ca-cert`/`--client-cert`/`--client-key` (Nick Craig-Wood)
    * Implement `--suffix-keep-extension` for use with `--suffix` (Nick Craig-Wood)
    * build:
        * Switch to semvar compliant version tags to be go modules compliant (Nick Craig-Wood)
        * Update to use go1.12.x for the build (Nick Craig-Wood)
    * serve dlna: Add connection manager service description to improve compatibility (Dan Walters)
    * lsf: Add 'e' format to show encrypted names and 'o' for original IDs (Nick Craig-Wood)
    * lsjson: Added `--files-only` and `--dirs-only` flags (calistri)
    * rc: Implement operations/publiclink the equivalent of `rclone link` (Nick Craig-Wood)
* Bug Fixes
    * accounting: Fix total ETA when `--stats-unit bits` is in effect (Nick Craig-Wood)
    * Bash TAB completion
        * Use private custom func to fix clash between rclone and kubectl (Nick Craig-Wood)
        * Fix for remotes with underscores in their names (Six)
        * Fix completion of remotes (Florian Gamböck)
        * Fix autocompletion of remote paths with spaces (Danil Semelenov)
    * serve dlna: Fix root XML service descriptor (Dan Walters)
    * ncdu: Fix display corruption with Chinese characters (Nick Craig-Wood)
    * Add SIGTERM to signals which run the exit handlers on unix (Nick Craig-Wood)
    * rc: Reload filter when the options are set via the rc (Nick Craig-Wood)
* VFS / Mount
    * Fix FreeBSD: Ignore Truncate if called with no readers and already the correct size (Nick Craig-Wood)
    * Read directory and check for a file before mkdir (Nick Craig-Wood)
    * Shorten the locking window for vfs/refresh (Nick Craig-Wood)
* Azure Blob
    * Enable MD5 checksums when uploading files bigger than the "Cutoff" (Dr.Rx)
    * Fix SAS URL support (Nick Craig-Wood)
* B2
    * Allow manual configuration of backblaze downloadUrl (Vince)
    * Ignore already_hidden error on remove (Nick Craig-Wood)
    * Ignore malformed `src_last_modified_millis` (Nick Craig-Wood)
* Drive
    * Add `--skip-checksum-gphotos` to ignore incorrect checksums on Google Photos (Nick Craig-Wood)
    * Allow server side move/copy between different remotes. (Fionera)
    * Add docs on team drives and `--fast-list` eventual consistency (Nestar47)
    * Fix imports of text files (Nick Craig-Wood)
    * Fix range requests on 0 length files (Nick Craig-Wood)
    * Fix creation of duplicates with server side copy (Nick Craig-Wood)
* Dropbox
    * Retry blank errors to fix long listings (Nick Craig-Wood)
* FTP
    * Add `--ftp-concurrency` to limit maximum number of connections (Nick Craig-Wood)
* Google Cloud Storage
    * Fall back to default application credentials (marcintustin)
    * Allow bucket policy only buckets (Nick Craig-Wood)
* HTTP
    * Add `--http-no-slash` for websites with directories with no slashes (Nick Craig-Wood)
    * Remove duplicates from listings (Nick Craig-Wood)
    * Fix socket leak on 404 errors (Nick Craig-Wood)
* Jottacloud
    * Fix token refresh (Sebastian Bünger)
    * Add device registration (Oliver Heyme)
* Onedrive
    * Implement graceful cancel of multipart uploads if rclone is interrupted (Cnly)
    * Always add trailing colon to path when addressing items, (Cnly)
    * Return errors instead of panic for invalid uploads (Fabian Möller)
* S3
    * Add support for "Glacier Deep Archive" storage class (Manu)
    * Update Dreamhost endpoint (Nick Craig-Wood)
    * Note incompatibility with CEPH Jewel (Nick Craig-Wood)
* SFTP
    * Allow custom ssh client config (Alexandru Bumbacea)
* Swift
    * Obey Retry-After to enable OVH restore from cold storage (Nick Craig-Wood)
    * Work around token expiry on CEPH (Nick Craig-Wood)
* WebDAV
    * Allow IsCollection property to be integer or boolean (Nick Craig-Wood)
    * Fix race when creating directories (Nick Craig-Wood)
    * Fix About/df when reading the available/total returns 0 (Nick Craig-Wood)

## v1.46 - 2019-02-09

* New backends
    * Support Alibaba Cloud (Aliyun) OSS via the s3 backend (Nick Craig-Wood)
* New commands
    * serve dlna: serves a remove via DLNA for the local network (nicolov)
* New Features
    * copy, move: Restore deprecated `--no-traverse` flag (Nick Craig-Wood)
        * This is useful for when transferring a small number of files into a large destination
    * genautocomplete: Add remote path completion for bash completion (Christopher Peterson & Danil Semelenov)
    * Buffer memory handling reworked to return memory to the OS better (Nick Craig-Wood)
        * Buffer recycling library to replace sync.Pool
        * Optionally use memory mapped memory for better memory shrinking
        * Enable with `--use-mmap` if having memory problems - not default yet
    * Parallelise reading of files specified by `--files-from` (Nick Craig-Wood)
    * check: Add stats showing total files matched. (Dario Guzik)
    * Allow rename/delete open files under Windows (Nick Craig-Wood)
    * lsjson: Use exactly the correct number of decimal places in the seconds (Nick Craig-Wood)
    * Add cookie support with cmdline switch `--use-cookies` for all HTTP based remotes (qip)
    * Warn if `--checksum` is set but there are no hashes available (Nick Craig-Wood)
    * Rework rate limiting (pacer) to be more accurate and allow bursting (Nick Craig-Wood)
    * Improve error reporting for too many/few arguments in commands (Nick Craig-Wood)
    * listremotes: Remove `-l` short flag as it conflicts with the new global flag (weetmuts)
    * Make http serving with auth generate INFO messages on auth fail (Nick Craig-Wood)
* Bug Fixes
    * Fix layout of stats (Nick Craig-Wood)
    * Fix `--progress` crash under Windows Jenkins (Nick Craig-Wood)
    * Fix transfer of google/onedrive docs by calling Rcat in Copy when size is -1 (Cnly)
    * copyurl: Fix checking of `--dry-run` (Denis Skovpen)
* Mount
    * Check that mountpoint and local directory to mount don't overlap (Nick Craig-Wood)
    * Fix mount size under 32 bit Windows (Nick Craig-Wood)
* VFS
    * Implement renaming of directories for backends without DirMove (Nick Craig-Wood)
        * now all backends except b2 support renaming directories
    * Implement `--vfs-cache-max-size` to limit the total size of the cache (Nick Craig-Wood)
    * Add `--dir-perms` and `--file-perms` flags to set default permissions (Nick Craig-Wood)
    * Fix deadlock on concurrent operations on a directory (Nick Craig-Wood)
    * Fix deadlock between RWFileHandle.close and File.Remove (Nick Craig-Wood)
    * Fix renaming/deleting open files with cache mode "writes" under Windows (Nick Craig-Wood)
    * Fix panic on rename with `--dry-run` set (Nick Craig-Wood)
    * Fix vfs/refresh with recurse=true needing the `--fast-list` flag
* Local
    * Add support for `-l`/`--links` (symbolic link translation) (yair@unicorn)
        * this works by showing links as `link.rclonelink` - see local backend docs for more info
        * this errors if used with `-L`/`--copy-links`
    * Fix renaming/deleting open files on Windows (Nick Craig-Wood)
* Crypt
    * Check for maximum length before decrypting filename to fix panic (Garry McNulty)
* Azure Blob
    * Allow building azureblob backend on *BSD (themylogin)
    * Use the rclone HTTP client to support `--dump headers`, `--tpslimit` etc (Nick Craig-Wood)
    * Use the s3 pacer for 0 delay in non error conditions (Nick Craig-Wood)
    * Ignore directory markers (Nick Craig-Wood)
    * Stop Mkdir attempting to create existing containers (Nick Craig-Wood)
* B2
    * cleanup: will remove unfinished large files >24hrs old (Garry McNulty)
    * For a bucket limited application key check the bucket name (Nick Craig-Wood)
        * before this, rclone would use the authorised bucket regardless of what you put on the command line
    * Added `--b2-disable-checksum` flag (Wojciech Smigielski)
        * this enables large files to be uploaded without a SHA-1 hash for speed reasons
* Drive
    * Set default pacer to 100ms for 10 tps (Nick Craig-Wood)
        * This fits the Google defaults much better and reduces the 403 errors massively
        * Add `--drive-pacer-min-sleep` and `--drive-pacer-burst` to control the pacer
    * Improve ChangeNotify support for items with multiple parents (Fabian Möller)
    * Fix ListR for items with multiple parents - this fixes oddities with `vfs/refresh` (Fabian Möller)
    * Fix using `--drive-impersonate` and appfolders (Nick Craig-Wood)
    * Fix google docs in rclone mount for some (not all) applications (Nick Craig-Wood)
* Dropbox
    * Retry-After support for Dropbox backend (Mathieu Carbou)
* FTP
    * Wait for 60 seconds for a connection to Close then declare it dead (Nick Craig-Wood)
        * helps with indefinite hangs on some FTP servers
* Google Cloud Storage
    * Update google cloud storage endpoints (weetmuts)
* HTTP
    * Add an example with username and password which is supported but wasn't documented (Nick Craig-Wood)
    * Fix backend with `--files-from` and non-existent files (Nick Craig-Wood)
* Hubic
    * Make error message more informative if authentication fails (Nick Craig-Wood)
* Jottacloud
    * Resume and deduplication support (Oliver Heyme)
    * Use token auth for all API requests Don't store password anymore (Sebastian Bünger)
    * Add support for 2-factor authentication (Sebastian Bünger)
* Mega
    * Implement v2 account login which fixes logins for newer Mega accounts (Nick Craig-Wood)
    * Return error if an unknown length file is attempted to be uploaded (Nick Craig-Wood)
    * Add new error codes for better error reporting (Nick Craig-Wood)
* Onedrive
    * Fix broken support for "shared with me" folders (Alex Chen)
    * Fix root ID not normalised (Cnly)
    * Return err instead of panic on unknown-sized uploads (Cnly)
* Qingstor
    * Fix go routine leak on multipart upload errors (Nick Craig-Wood)
    * Add upload chunk size/concurrency/cutoff control (Nick Craig-Wood)
    * Default `--qingstor-upload-concurrency` to 1 to work around bug (Nick Craig-Wood)
* S3
    * Implement `--s3-upload-cutoff` for single part uploads below this (Nick Craig-Wood)
    * Change `--s3-upload-concurrency` default to 4 to increase performance (Nick Craig-Wood)
    * Add `--s3-bucket-acl` to control bucket ACL (Nick Craig-Wood)
    * Auto detect region for buckets on operation failure (Nick Craig-Wood)
    * Add GLACIER storage class (William Cocker)
    * Add Scaleway to s3 documentation (Rémy Léone)
    * Add AWS endpoint eu-north-1 (weetmuts)
* SFTP
    * Add support for PEM encrypted private keys (Fabian Möller)
    * Add option to force the usage of an ssh-agent (Fabian Möller)
    * Perform environment variable expansion on key-file (Fabian Möller)
    * Fix rmdir on Windows based servers (eg CrushFTP) (Nick Craig-Wood)
    * Fix rmdir deleting directory contents on some SFTP servers (Nick Craig-Wood)
    * Fix error on dangling symlinks (Nick Craig-Wood)
* Swift
    * Add `--swift-no-chunk` to disable segmented uploads in rcat/mount (Nick Craig-Wood)
    * Introduce application credential auth support (kayrus)
    * Fix memory usage by slimming Object (Nick Craig-Wood)
    * Fix extra requests on upload (Nick Craig-Wood)
    * Fix reauth on big files (Nick Craig-Wood)
* Union
    * Fix poll-interval not working (Nick Craig-Wood)
* WebDAV
    * Support About which means rclone mount will show the correct disk size (Nick Craig-Wood)
    * Support MD5 and SHA1 hashes with Owncloud and Nextcloud (Nick Craig-Wood)
    * Fail soft on time parsing errors (Nick Craig-Wood)
    * Fix infinite loop on failed directory creation (Nick Craig-Wood)
    * Fix identification of directories for Bitrix Site Manager (Nick Craig-Wood)
    * Fix upload of 0 length files on some servers (Nick Craig-Wood)
    * Fix if MKCOL fails with 423 Locked assume the directory exists (Nick Craig-Wood)

## v1.45 - 2018-11-24

* New backends
    * The Yandex backend was re-written - see below for details (Sebastian Bünger)
* New commands
    * rcd: New command just to serve the remote control API (Nick Craig-Wood)
* New Features
    * The remote control API (rc) was greatly expanded to allow full control over rclone (Nick Craig-Wood)
        * sensitive operations require authorization or the `--rc-no-auth` flag
        * config/* operations to configure rclone
        * options/* for reading/setting command line flags
        * operations/* for all low level operations, eg copy file, list directory
        * sync/* for sync, copy and move
        * `--rc-files` flag to serve files on the rc http server
          * this is for building web native GUIs for rclone
        * Optionally serving objects on the rc http server
        * Ensure rclone fails to start up if the `--rc` port is in use already
        * See [the rc docs](https://rclone.org/rc/) for more info
    * sync/copy/move
        * Make `--files-from` only read the objects specified and don't scan directories (Nick Craig-Wood)
            * This is a huge speed improvement for destinations with lots of files
    * filter: Add `--ignore-case` flag (Nick Craig-Wood)
    * ncdu: Add remove function ('d' key) (Henning Surmeier)
    * rc command
        * Add `--json` flag for structured JSON input (Nick Craig-Wood)
        * Add `--user` and `--pass` flags and interpret `--rc-user`, `--rc-pass`, `--rc-addr` (Nick Craig-Wood)
    * build
        * Require go1.8 or later for compilation (Nick Craig-Wood)
        * Enable softfloat on MIPS arch (Scott Edlund)
        * Integration test framework revamped with a better report and better retries (Nick Craig-Wood)
* Bug Fixes
    * cmd: Make `--progress` update the stats correctly at the end (Nick Craig-Wood)
    * config: Create config directory on save if it is missing (Nick Craig-Wood)
    * dedupe: Check for existing filename before renaming a dupe file (ssaqua)
    * move: Don't create directories with `--dry-run` (Nick Craig-Wood)
    * operations: Fix Purge and Rmdirs when dir is not the root (Nick Craig-Wood)
    * serve http/webdav/restic: Ensure rclone exits if the port is in use (Nick Craig-Wood)
* Mount
    * Make `--volname` work for Windows and macOS (Nick Craig-Wood)
* Azure Blob
    * Avoid context deadline exceeded error by setting a large TryTimeout value (brused27)
    * Fix erroneous Rmdir error "directory not empty" (Nick Craig-Wood)
    * Wait for up to 60s to create a just deleted container (Nick Craig-Wood)
* Dropbox
    * Add dropbox impersonate support (Jake Coggiano)
* Jottacloud
    * Fix bug in `--fast-list` handing of empty folders (albertony)
* Opendrive
    * Fix transfer of files with `+` and `&` in (Nick Craig-Wood)
    * Fix retries of upload chunks (Nick Craig-Wood)
* S3
    * Set ACL for server side copies to that provided by the user (Nick Craig-Wood)
    * Fix role_arn, credential_source, ... (Erik Swanson)
    * Add config info for Wasabi's US-West endpoint (Henry Ptasinski)
* SFTP
    * Ensure file hash checking is really disabled (Jon Fautley)
* Swift
    * Add pacer for retries to make swift more reliable (Nick Craig-Wood)
* WebDAV
    * Add Content-Type to PUT requests (Nick Craig-Wood)
    * Fix config parsing so `--webdav-user` and `--webdav-pass` flags work (Nick Craig-Wood)
    * Add RFC3339 date format (Ralf Hemberger)
* Yandex
    * The yandex backend was re-written (Sebastian Bünger)
        * This implements low level retries (Sebastian Bünger)
        * Copy, Move, DirMove, PublicLink and About optional interfaces (Sebastian Bünger)
        * Improved general error handling (Sebastian Bünger)
        * Removed ListR for now due to inconsistent behaviour (Sebastian Bünger)

## v1.44 - 2018-10-15

* New commands
    * serve ftp: Add ftp server (Antoine GIRARD)
    * settier: perform storage tier changes on supported remotes (sandeepkru)
* New Features
    * Reworked command line help
        * Make default help less verbose (Nick Craig-Wood)
        * Split flags up into global and backend flags (Nick Craig-Wood)
        * Implement specialised help for flags and backends (Nick Craig-Wood)
        * Show URL of backend help page when starting config (Nick Craig-Wood)
    * stats: Long names now split in center (Joanna Marek)
    * Add `--log-format` flag for more control over log output (dcpu)
    * rc: Add support for OPTIONS and basic CORS (frenos)
    * stats: show FatalErrors and NoRetryErrors in stats (Cédric Connes)
* Bug Fixes
    * Fix -P not ending with a new line (Nick Craig-Wood)
    * config: don't create default config dir when user supplies `--config` (albertony)
    * Don't print non-ASCII characters with `--progress` on windows (Nick Craig-Wood)
    * Correct logs for excluded items (ssaqua)
* Mount
    * Remove EXPERIMENTAL tags (Nick Craig-Wood)
* VFS
    * Fix race condition detected by serve ftp tests (Nick Craig-Wood)
    * Add vfs/poll-interval rc command (Fabian Möller)
    * Enable rename for nearly all remotes using server side Move or Copy (Nick Craig-Wood)
    * Reduce directory cache cleared by poll-interval (Fabian Möller)
    * Remove EXPERIMENTAL tags (Nick Craig-Wood)
* Local
    * Skip bad symlinks in dir listing with -L enabled (Cédric Connes)
    * Preallocate files on Windows to reduce fragmentation (Nick Craig-Wood)
    * Preallocate files on linux with fallocate(2) (Nick Craig-Wood)
* Cache
    * Add cache/fetch rc function (Fabian Möller)
    * Fix worker scale down (Fabian Möller)
    * Improve performance by not sending info requests for cached chunks (dcpu)
    * Fix error return value of cache/fetch rc method (Fabian Möller)
    * Documentation fix for cache-chunk-total-size (Anagh Kumar Baranwal)
    * Preserve leading / in wrapped remote path (Fabian Möller)
    * Add plex_insecure option to skip certificate validation (Fabian Möller)
    * Remove entries that no longer exist in the source (dcpu)
* Crypt
    * Preserve leading / in wrapped remote path (Fabian Möller)
* Alias
    * Fix handling of Windows network paths (Nick Craig-Wood)
* Azure Blob
    * Add `--azureblob-list-chunk` parameter (Santiago Rodríguez)
    * Implemented settier command support on azureblob remote. (sandeepkru)
    * Work around SDK bug which causes errors for chunk-sized files (Nick Craig-Wood)
* Box
    * Implement link sharing. (Sebastian Bünger)
* Drive
    * Add `--drive-import-formats` - google docs can now be imported (Fabian Möller)
        * Rewrite mime type and extension handling (Fabian Möller)
        * Add document links (Fabian Möller)
        * Add support for multipart document extensions (Fabian Möller)
        * Add support for apps-script to json export (Fabian Möller)
        * Fix escaped chars in documents during list (Fabian Möller)
    * Add `--drive-v2-download-min-size` a workaround for slow downloads (Fabian Möller)
    * Improve directory notifications in ChangeNotify (Fabian Möller)
    * When listing team drives in config, continue on failure (Nick Craig-Wood)
* FTP
    * Add a small pause after failed upload before deleting file (Nick Craig-Wood)
* Google Cloud Storage
    * Fix service_account_file being ignored (Fabian Möller)
* Jottacloud
    * Minor improvement in quota info (omit if unlimited) (albertony)
    * Add `--fast-list` support (albertony)
    * Add permanent delete support: `--jottacloud-hard-delete` (albertony)
    * Add link sharing support (albertony)
    * Fix handling of reserved characters. (Sebastian Bünger)
    * Fix socket leak on Object.Remove (Nick Craig-Wood)
* Onedrive
    * Rework to support Microsoft Graph (Cnly)
        * **NB** this will require re-authenticating the remote
    * Removed upload cutoff and always do session uploads (Oliver Heyme)
    * Use single-part upload for empty files (Cnly)
    * Fix new fields not saved when editing old config (Alex Chen)
    * Fix sometimes special chars in filenames not replaced (Alex Chen)
    * Ignore OneNote files by default (Alex Chen)
    * Add link sharing support (jackyzy823)
* S3
    * Use custom pacer, to retry operations when reasonable (Craig Miskell)
    * Use configured server-side-encryption and storage class options when calling CopyObject() (Paul Kohout)
    * Make `--s3-v2-auth` flag (Nick Craig-Wood)
    * Fix v2 auth on files with spaces (Nick Craig-Wood)
* Union
    * Implement union backend which reads from multiple backends (Felix Brucker)
    * Implement optional interfaces (Move, DirMove, Copy etc) (Nick Craig-Wood)
    * Fix ChangeNotify to support multiple remotes (Fabian Möller)
    * Fix `--backup-dir` on union backend (Nick Craig-Wood)
* WebDAV
    * Add another time format (Nick Craig-Wood)
    * Add a small pause after failed upload before deleting file (Nick Craig-Wood)
    * Add workaround for missing mtime (buergi)
    * Sharepoint: Renew cookies after 12hrs (Henning Surmeier)
* Yandex
    * Remove redundant nil checks (teresy)

## v1.43.1 - 2018-09-07

Point release to fix hubic and azureblob backends.

* Bug Fixes
    * ncdu: Return error instead of log.Fatal in Show (Fabian Möller)
    * cmd: Fix crash with `--progress` and `--stats 0` (Nick Craig-Wood)
    * docs: Tidy website display (Anagh Kumar Baranwal)
* Azure Blob:
    * Fix multi-part uploads. (sandeepkru)
* Hubic
    * Fix uploads (Nick Craig-Wood)
    * Retry auth fetching if it fails to make hubic more reliable (Nick Craig-Wood)

## v1.43 - 2018-09-01

* New backends
    * Jottacloud (Sebastian Bünger)
* New commands
    * copyurl: copies a URL to a remote (Denis)
* New Features
    * Reworked config for backends (Nick Craig-Wood)
        * All backend config can now be supplied by command line, env var or config file
        * Advanced section in the config wizard for the optional items
        * A large step towards rclone backends being usable in other go software
        * Allow on the fly remotes with :backend: syntax
    * Stats revamp
        * Add `--progress`/`-P` flag to show interactive progress (Nick Craig-Wood)
        * Show the total progress of the sync in the stats (Nick Craig-Wood)
        * Add `--stats-one-line` flag for single line stats (Nick Craig-Wood)
    * Added weekday schedule into `--bwlimit` (Mateusz)
    * lsjson: Add option to show the original object IDs (Fabian Möller)
    * serve webdav: Make Content-Type without reading the file and add `--etag-hash` (Nick Craig-Wood)
    * build
        * Build macOS with native compiler (Nick Craig-Wood)
        * Update to use go1.11 for the build (Nick Craig-Wood)
    * rc
        * Added core/stats to return the stats (reddi1)
    * `version --check`: Prints the current release and beta versions (Nick Craig-Wood)
* Bug Fixes
    * accounting
        * Fix time to completion estimates (Nick Craig-Wood)
        * Fix moving average speed for file stats (Nick Craig-Wood)
    * config: Fix error reading password from piped input (Nick Craig-Wood)
    * move: Fix `--delete-empty-src-dirs` flag to delete all empty dirs on move (ishuah)
* Mount
    * Implement `--daemon-timeout` flag for OSXFUSE (Nick Craig-Wood)
    * Fix mount `--daemon` not working with encrypted config (Alex Chen)
    * Clip the number of blocks to 2^32-1 on macOS - fixes borg backup (Nick Craig-Wood)
* VFS
    * Enable vfs-read-chunk-size by default (Fabian Möller)
    * Add the vfs/refresh rc command (Fabian Möller)
    * Add non recursive mode to vfs/refresh rc command (Fabian Möller)
    * Try to seek buffer on read only files (Fabian Möller)
* Local
    * Fix crash when deprecated `--local-no-unicode-normalization` is supplied (Nick Craig-Wood)
    * Fix mkdir error when trying to copy files to the root of a drive on windows (Nick Craig-Wood)
* Cache
    * Fix nil pointer deref when using lsjson on cached directory (Nick Craig-Wood)
    * Fix nil pointer deref for occasional crash on playback (Nick Craig-Wood)
* Crypt
    * Fix accounting when checking hashes on upload (Nick Craig-Wood)
* Amazon Cloud Drive
    * Make very clear in the docs that rclone has no ACD keys (Nick Craig-Wood)
* Azure Blob
    * Add connection string and SAS URL auth (Nick Craig-Wood)
    * List the container to see if it exists (Nick Craig-Wood)
    * Port new Azure Blob Storage SDK (sandeepkru)
    * Added blob tier, tier between Hot, Cool and Archive. (sandeepkru)
    * Remove leading / from paths (Nick Craig-Wood)
* B2
    * Support Application Keys (Nick Craig-Wood)
    * Remove leading / from paths (Nick Craig-Wood)
* Box
    * Fix upload of > 2GB files on 32 bit platforms (Nick Craig-Wood)
    * Make `--box-commit-retries` flag defaulting to 100 to fix large uploads (Nick Craig-Wood)
* Drive
    * Add `--drive-keep-revision-forever` flag (lewapm)
    * Handle gdocs when filtering file names in list (Fabian Möller)
    * Support using `--fast-list` for large speedups (Fabian Möller)
* FTP
    * Fix Put mkParentDir failed: 521 for BunnyCDN (Nick Craig-Wood)
* Google Cloud Storage
    * Fix index out of range error with `--fast-list` (Nick Craig-Wood)
* Jottacloud
    * Fix MD5 error check (Oliver Heyme)
    * Handle empty time values (Martin Polden)
    * Calculate missing MD5s (Oliver Heyme)
    * Docs, fixes and tests for MD5 calculation (Nick Craig-Wood)
    * Add optional MimeTyper interface. (Sebastian Bünger)
    * Implement optional About interface (for `df` support). (Sebastian Bünger)
* Mega
    * Wait for events instead of arbitrary sleeping (Nick Craig-Wood)
    * Add `--mega-hard-delete` flag (Nick Craig-Wood)
    * Fix failed logins with upper case chars in email (Nick Craig-Wood)
* Onedrive
    * Shared folder support (Yoni Jah)
    * Implement DirMove (Cnly)
    * Fix rmdir sometimes deleting directories with contents (Nick Craig-Wood)
* Pcloud
    * Delete half uploaded files on upload error (Nick Craig-Wood)
* Qingstor
    * Remove leading / from paths (Nick Craig-Wood)
* S3
    * Fix index out of range error with `--fast-list` (Nick Craig-Wood)
    * Add `--s3-force-path-style` (Nick Craig-Wood)
    * Add support for KMS Key ID (bsteiss)
    * Remove leading / from paths (Nick Craig-Wood)
* Swift
    * Add `storage_policy` (Ruben Vandamme)
    * Make it so just `storage_url` or `auth_token` can be overridden (Nick Craig-Wood)
    * Fix server side copy bug for unusual file names (Nick Craig-Wood)
    * Remove leading / from paths (Nick Craig-Wood)
* WebDAV
    * Ensure we call MKCOL with a URL with a trailing / for QNAP interop (Nick Craig-Wood)
    * If root ends with / then don't check if it is a file (Nick Craig-Wood)
    * Don't accept redirects when reading metadata (Nick Craig-Wood)
    * Add bearer token (Macaroon) support for dCache (Nick Craig-Wood)
    * Document dCache and Macaroons (Onno Zweers)
    * Sharepoint recursion with different depth (Henning)
    * Attempt to remove failed uploads (Nick Craig-Wood)
* Yandex
    * Fix listing/deleting files in the root (Nick Craig-Wood)

## v1.42 - 2018-06-16

* New backends
    * OpenDrive (Oliver Heyme, Jakub Karlicek, ncw)
* New commands
    * deletefile command (Filip Bartodziej)
* New Features
    * copy, move: Copy single files directly, don't use `--files-from` work-around
        * this makes them much more efficient
    * Implement `--max-transfer` flag to quit transferring at a limit
        * make exit code 8 for `--max-transfer` exceeded
    * copy: copy empty source directories to destination (Ishuah Kariuki)
    * check: Add `--one-way` flag (Kasper Byrdal Nielsen)
    * Add siginfo handler for macOS for ctrl-T stats (kubatasiemski)
    * rc
        * add core/gc to run a garbage collection on demand
        * enable go profiling by default on the `--rc` port
        * return error from remote on failure
    * lsf
        * Add `--absolute` flag to add a leading / onto path names
        * Add `--csv` flag for compliant CSV output
        * Add 'm' format specifier to show the MimeType
        * Implement 'i' format for showing object ID
    * lsjson
        * Add MimeType to the output
        * Add ID field to output to show Object ID
    * Add `--retries-sleep` flag (Benjamin Joseph Dag)
    * Oauth tidy up web page and error handling (Henning Surmeier)
* Bug Fixes
    * Password prompt output with `--log-file` fixed for unix (Filip Bartodziej)
    * Calculate ModifyWindow each time on the fly to fix various problems (Stefan Breunig)
* Mount
    * Only print "File.rename error" if there actually is an error (Stefan Breunig)
    * Delay rename if file has open writers instead of failing outright (Stefan Breunig)
    * Ensure atexit gets run on interrupt
    * macOS enhancements
        * Make `--noappledouble` `--noapplexattr`
        * Add `--volname` flag and remove special chars from it
        * Make Get/List/Set/Remove xattr return ENOSYS for efficiency
        * Make `--daemon` work for macOS without CGO
* VFS
    * Add `--vfs-read-chunk-size` and `--vfs-read-chunk-size-limit` (Fabian Möller)
    * Fix ChangeNotify for new or changed folders (Fabian Möller)
* Local
    * Fix symlink/junction point directory handling under Windows
        * **NB** you will need to add `-L` to your command line to copy files with reparse points
* Cache
    * Add non cached dirs on notifications (Remus Bunduc)
    * Allow root to be expired from rc (Remus Bunduc)
    * Clean remaining empty folders from temp upload path (Remus Bunduc)
    * Cache lists using batch writes (Remus Bunduc)
    * Use secure websockets for HTTPS Plex addresses (John Clayton)
    * Reconnect plex websocket on failures (Remus Bunduc)
    * Fix panic when running without plex configs (Remus Bunduc)
    * Fix root folder caching (Remus Bunduc)
* Crypt
    * Check the crypted hash of files when uploading for extra data security
* Dropbox
    * Make Dropbox for business folders accessible using an initial `/` in the path
* Google Cloud Storage
    * Low level retry all operations if necessary
* Google Drive
    * Add `--drive-acknowledge-abuse` to download flagged files
    * Add `--drive-alternate-export` to fix large doc export
    * Don't attempt to choose Team Drives when using rclone config create
    * Fix change list polling with team drives
    * Fix ChangeNotify for folders (Fabian Möller)
    * Fix about (and df on a mount) for team drives
* Onedrive
    * Errorhandler for onedrive for business requests (Henning Surmeier)
* S3
    * Adjust upload concurrency with `--s3-upload-concurrency` (themylogin)
    * Fix `--s3-chunk-size` which was always using the minimum
* SFTP
    * Add `--ssh-path-override` flag (Piotr Oleszczyk)
    * Fix slow downloads for long latency connections
* Webdav
    * Add workarounds for biz.mail.ru
    * Ignore Reason-Phrase in status line to fix 4shared (Rodrigo)
    * Better error message generation

## v1.41 - 2018-04-28

* New backends
    * Mega support added
    * Webdav now supports SharePoint cookie authentication (hensur)
* New commands
    * link: create public link to files and folders (Stefan Breunig)
    * about: gets quota info from a remote (a-roussos, ncw)
    * hashsum: a generic tool for any hash to produce md5sum like output
* New Features
    * lsd: Add -R flag and fix and update docs for all ls commands
    * ncdu: added a "refresh" key - CTRL-L (Keith Goldfarb)
    * serve restic: Add append-only mode (Steve Kriss)
    * serve restic: Disallow overwriting files in append-only mode (Alexander Neumann)
    * serve restic: Print actual listener address (Matt Holt)
    * size: Add --json flag (Matthew Holt)
    * sync: implement --ignore-errors (Mateusz Pabian)
    * dedupe: Add dedupe largest functionality (Richard Yang)
    * fs: Extend SizeSuffix to include TB and PB for rclone about
    * fs: add --dump goroutines and --dump openfiles for debugging
    * rc: implement core/memstats to print internal memory usage info
    * rc: new call rc/pid (Michael P. Dubner)
* Compile
    * Drop support for go1.6
* Release
    * Fix `make tarball` (Chih-Hsuan Yen)
* Bug Fixes
    * filter: fix --min-age and --max-age together check
    * fs: limit MaxIdleConns and MaxIdleConnsPerHost in transport
    * lsd,lsf: make sure all times we output are in local time
    * rc: fix setting bwlimit to unlimited
    * rc: take note of the --rc-addr flag too as per the docs
* Mount
    * Use About to return the correct disk total/used/free (eg in `df`)
    * Set `--attr-timeout default` to `1s` - fixes:
        * rclone using too much memory
        * rclone not serving files to samba
        * excessive time listing directories
    * Fix `df -i` (upstream fix)
* VFS
    * Filter files `.` and `..` from directory listing
    * Only make the VFS cache if --vfs-cache-mode > Off
* Local
    * Add --local-no-check-updated to disable updated file checks
    * Retry remove on Windows sharing violation error
* Cache
    * Flush the memory cache after close
    * Purge file data on notification
    * Always forget parent dir for notifications
    * Integrate with Plex websocket
    * Add rc cache/stats (seuffert)
    * Add info log on notification 
* Box
    * Fix failure reading large directories - parse file/directory size as float
* Dropbox
    * Fix crypt+obfuscate on dropbox
    * Fix repeatedly uploading the same files
* FTP
    * Work around strange response from box FTP server
    * More workarounds for FTP servers to fix mkParentDir error
    * Fix no error on listing non-existent directory
* Google Cloud Storage
    * Add service_account_credentials (Matt Holt)
    * Detect bucket presence by listing it - minimises permissions needed
    * Ignore zero length directory markers
* Google Drive
    * Add service_account_credentials (Matt Holt)
    * Fix directory move leaving a hardlinked directory behind
    * Return proper google errors when Opening files
    * When initialized with a filepath, optional features used incorrect root path (Stefan Breunig)
* HTTP
    * Fix sync for servers which don't return Content-Length in HEAD
* Onedrive
    * Add QuickXorHash support for OneDrive for business
    * Fix socket leak in multipart session upload
* S3
    * Look in S3 named profile files for credentials
    * Add `--s3-disable-checksum` to disable checksum uploading (Chris Redekop)
    * Hierarchical configuration support (Giri Badanahatti)
    * Add in config for all the supported S3 providers
    * Add One Zone Infrequent Access storage class (Craig Rachel)
    * Add --use-server-modtime support (Peter Baumgartner)
    * Add --s3-chunk-size option to control multipart uploads
    * Ignore zero length directory markers
* SFTP
    * Update docs to match code, fix typos and clarify disable_hashcheck prompt (Michael G. Noll)
    * Update docs with Synology quirks
    * Fail soft with a debug on hash failure
* Swift
    * Add --use-server-modtime support (Peter Baumgartner)
* Webdav
    * Support SharePoint cookie authentication (hensur)
    * Strip leading and trailing / off root

## v1.40 - 2018-03-19

* New backends
    * Alias backend to create aliases for existing remote names (Fabian Möller)
* New commands
    * `lsf`: list for parsing purposes (Jakub Tasiemski)
        * by default this is a simple non recursive list of files and directories
        * it can be configured to add more info in an easy to parse way
    * `serve restic`: for serving a remote as a Restic REST endpoint
        * This enables restic to use any backends that rclone can access
        * Thanks Alexander Neumann for help, patches and review
    * `rc`: enable the remote control of a running rclone
        * The running rclone must be started with --rc and related flags.
        * Currently there is support for bwlimit, and flushing for mount and cache.
* New Features
    * `--max-delete` flag to add a delete threshold (Bjørn Erik Pedersen)
    * All backends now support RangeOption for ranged Open
        * `cat`: Use RangeOption for limited fetches to make more efficient
        * `cryptcheck`: make reading of nonce more efficient with RangeOption
    * serve http/webdav/restic
        * support SSL/TLS
        * add `--user` `--pass` and `--htpasswd` for authentication
    * `copy`/`move`: detect file size change during copy/move and abort transfer (ishuah)
    * `cryptdecode`: added option to return encrypted file names. (ishuah)
    * `lsjson`: add `--encrypted` to show encrypted name (Jakub Tasiemski)
    * Add `--stats-file-name-length` to specify the printed file name length for stats (Will Gunn)
* Compile
    * Code base was shuffled and factored
        * backends moved into a backend directory
        * large packages split up
        * See the CONTRIBUTING.md doc for info as to what lives where now
    * Update to using go1.10 as the default go version
    * Implement daily [full integration tests](https://pub.rclone.org/integration-tests/)
* Release
    * Include a source tarball and sign it and the binaries
    * Sign the git tags as part of the release process
    * Add .deb and .rpm packages as part of the build
    * Make a beta release for all branches on the main repo (but not pull requests)
* Bug Fixes
    * config: fixes errors on non existing config by loading config file only on first access
    * config: retry saving the config after failure (Mateusz)
    * sync: when using `--backup-dir` don't delete files if we can't set their modtime
        * this fixes odd behaviour with Dropbox and `--backup-dir`
    * fshttp: fix idle timeouts for HTTP connections
    * `serve http`: fix serving files with : in - fixes
    * Fix `--exclude-if-present` to ignore directories which it doesn't have permission for (Iakov Davydov)
    * Make accounting work properly with crypt and b2
    * remove `--no-traverse` flag because it is obsolete
* Mount
    * Add `--attr-timeout` flag to control attribute caching in kernel
        * this now defaults to 0 which is correct but less efficient
        * see [the mount docs](https://rclone.org/commands/rclone_mount/#attribute-caching) for more info
    * Add `--daemon` flag to allow mount to run in the background (ishuah)
    * Fix: Return ENOSYS rather than EIO on attempted link
        * This fixes FileZilla accessing an rclone mount served over sftp.
    * Fix setting modtime twice
    * Mount tests now run on CI for Linux (mount & cmount)/Mac/Windows
    * Many bugs fixed in the VFS layer - see below
* VFS
    * Many fixes for `--vfs-cache-mode` writes and above
        * Update cached copy if we know it has changed (fixes stale data)
        * Clean path names before using them in the cache
        * Disable cache cleaner if `--vfs-cache-poll-interval=0`
        * Fill and clean the cache immediately on startup
    * Fix Windows opening every file when it stats the file
    * Fix applying modtime for an open Write Handle
    * Fix creation of files when truncating
    * Write 0 bytes when flushing unwritten handles to avoid race conditions in FUSE
    * Downgrade "poll-interval is not supported" message to Info
    * Make OpenFile and friends return EINVAL if O_RDONLY and O_TRUNC
* Local
    * Downgrade "invalid cross-device link: trying copy" to debug
    * Make DirMove return fs.ErrorCantDirMove to allow fallback to Copy for cross device
    * Fix race conditions updating the hashes
* Cache
    * Add support for polling - cache will update when remote changes on supported backends
    * Reduce log level for Plex api
    * Fix dir cache issue
    * Implement `--cache-db-wait-time` flag
    * Improve efficiency with RangeOption and RangeSeek
    * Fix dirmove with temp fs enabled
    * Notify vfs when using temp fs
    * Offline uploading
    * Remote control support for path flushing
* Amazon cloud drive
    * Rclone no longer has any working keys - disable integration tests
    * Implement DirChangeNotify to notify cache/vfs/mount of changes
* Azureblob
    * Don't check for bucket/container presence if listing was OK
        * this makes rclone do one less request per invocation
    * Improve accounting for chunked uploads
* Backblaze B2
    * Don't check for bucket/container presence if listing was OK
        * this makes rclone do one less request per invocation
* Box
    * Improve accounting for chunked uploads
* Dropbox
    * Fix custom oauth client parameters
* Google Cloud Storage
    * Don't check for bucket/container presence if listing was OK
        * this makes rclone do one less request per invocation
* Google Drive
    * Migrate to api v3 (Fabian Möller)
    * Add scope configuration and root folder selection
    * Add `--drive-impersonate` for service accounts
        * thanks to everyone who tested, explored and contributed docs
    * Add `--drive-use-created-date` to use created date as modified date (nbuchanan)
    * Request the export formats only when required
        * This makes rclone quicker when there are no google docs
    * Fix finding paths with latin1 chars (a workaround for a drive bug)
    * Fix copying of a single Google doc file
    * Fix `--drive-auth-owner-only` to look in all directories
* HTTP
    * Fix handling of directories with & in
* Onedrive
    * Removed upload cutoff and always do session uploads
        * this stops the creation of multiple versions on business onedrive
    * Overwrite object size value with real size when reading file. (Victor)
        * this fixes oddities when onedrive misreports the size of images
* Pcloud
    * Remove unused chunked upload flag and code
* Qingstor
    * Don't check for bucket/container presence if listing was OK
        * this makes rclone do one less request per invocation
* S3
    * Support hashes for multipart files (Chris Redekop)
    * Initial support for IBM COS (S3) (Giri Badanahatti)
    * Update docs to discourage use of v2 auth with CEPH and others
    * Don't check for bucket/container presence if listing was OK
        * this makes rclone do one less request per invocation
    * Fix server side copy and set modtime on files with + in
* SFTP
    * Add option to disable remote hash check command execution (Jon Fautley)
    * Add `--sftp-ask-password` flag to prompt for password when needed (Leo R. Lundgren)
    * Add `set_modtime` configuration option
    * Fix following of symlinks
    * Fix reading config file outside of Fs setup
    * Fix reading $USER in username fallback not $HOME
    * Fix running under crontab - Use correct OS way of reading username 
* Swift
    * Fix refresh of authentication token
        * in v1.39 a bug was introduced which ignored new tokens - this fixes it
    * Fix extra HEAD transaction when uploading a new file
    * Don't check for bucket/container presence if listing was OK
        * this makes rclone do one less request per invocation
* Webdav
    * Add new time formats to support mydrive.ch and others

## v1.39 - 2017-12-23

* New backends
    * WebDAV
        * tested with nextcloud, owncloud, put.io and others!
    * Pcloud
    * cache - wraps a cache around other backends (Remus Bunduc)
        * useful in combination with mount
        * NB this feature is in beta so use with care
* New commands
    * serve command with subcommands:
        * serve webdav: this implements a webdav server for any rclone remote.
        * serve http: command to serve a remote over HTTP
    * config: add sub commands for full config file management
        * create/delete/dump/edit/file/password/providers/show/update
    * touch: to create or update the timestamp of a file (Jakub Tasiemski)
* New Features
    * curl install for rclone (Filip Bartodziej)
    * --stats now shows percentage, size, rate and ETA in condensed form (Ishuah Kariuki)
    * --exclude-if-present to exclude a directory if a file is present (Iakov Davydov)
    * rmdirs: add --leave-root flag (lewapm)
    * move: add --delete-empty-src-dirs flag to remove dirs after move (Ishuah Kariuki)
    * Add --dump flag, introduce --dump requests, responses and remove --dump-auth, --dump-filters
        * Obscure X-Auth-Token: from headers when dumping too
    * Document and implement exit codes for different failure modes (Ishuah Kariuki)
* Compile
* Bug Fixes
    * Retry lots more different types of errors to make multipart transfers more reliable
    * Save the config before asking for a token, fixes disappearing oauth config
    * Warn the user if --include and --exclude are used together (Ernest Borowski)
    * Fix duplicate files (eg on Google drive) causing spurious copies
    * Allow trailing and leading whitespace for passwords (Jason Rose)
    * ncdu: fix crashes on empty directories
    * rcat: fix goroutine leak
    * moveto/copyto: Fix to allow copying to the same name
* Mount
    * --vfs-cache mode to make writes into mounts more reliable.
        * this requires caching files on the disk (see --cache-dir)
        * As this is a new feature, use with care
    * Use sdnotify to signal systemd the mount is ready (Fabian Möller)
    * Check if directory is not empty before mounting (Ernest Borowski)
* Local
    * Add error message for cross file system moves
    * Fix equality check for times
* Dropbox
    * Rework multipart upload
        * buffer the chunks when uploading large files so they can be retried
        * change default chunk size to 48MB now we are buffering them in memory
        * retry every error after the first chunk is done successfully
    * Fix error when renaming directories
* Swift
    * Fix crash on bad authentication
* Google Drive
    * Add service account support (Tim Cooijmans)
* S3
    * Make it work properly with Digital Ocean Spaces (Andrew Starr-Bochicchio)
    * Fix crash if a bad listing is received
    * Add support for ECS task IAM roles (David Minor)
* Backblaze B2
    * Fix multipart upload retries
    * Fix --hard-delete to make it work 100% of the time
* Swift
    * Allow authentication with storage URL and auth key (Giovanni Pizzi)
    * Add new fields for swift configuration to support IBM Bluemix Swift (Pierre Carlson)
    * Add OS_TENANT_ID and OS_USER_ID to config
    * Allow configs with user id instead of user name
    * Check if swift segments container exists before creating (John Leach)
    * Fix memory leak in swift transfers (upstream fix)
* SFTP
    * Add option to enable the use of aes128-cbc cipher (Jon Fautley)
* Amazon cloud drive
    * Fix download of large files failing with "Only one auth mechanism allowed"
* crypt
    * Option to encrypt directory names or leave them intact
    * Implement DirChangeNotify (Fabian Möller)
* onedrive
    * Add option to choose resourceURL during setup of OneDrive Business account if more than one is available for user

## v1.38 - 2017-09-30

* New backends
    * Azure Blob Storage (thanks Andrei Dragomir)
    * Box
    * Onedrive for Business (thanks Oliver Heyme)
    * QingStor from QingCloud (thanks wuyu)
* New commands
    * `rcat` - read from standard input and stream upload
    * `tree` - shows a nicely formatted recursive listing
    * `cryptdecode` - decode crypted file names (thanks ishuah)
    * `config show` - print the config file
    * `config file` - print the config file location
* New Features
    * Empty directories are deleted on `sync`
    * `dedupe` - implement merging of duplicate directories
    * `check` and `cryptcheck` made more consistent and use less memory
    * `cleanup` for remaining remotes (thanks ishuah)
    * `--immutable` for ensuring that files don't change (thanks Jacob McNamee)
    * `--user-agent` option (thanks Alex McGrath Kraak)
    * `--disable` flag to disable optional features
    * `--bind` flag for choosing the local addr on outgoing connections
    * Support for zsh auto-completion (thanks bpicode)
    * Stop normalizing file names but do a normalized compare in `sync`
* Compile
    * Update to using go1.9 as the default go version
    * Remove snapd build due to maintenance problems
* Bug Fixes
    * Improve retriable error detection which makes multipart uploads better
    * Make `check` obey `--ignore-size`
    * Fix bwlimit toggle in conjunction with schedules (thanks cbruegg)
    * `config` ensures newly written config is on the same mount
* Local
    * Revert to copy when moving file across file system boundaries
    * `--skip-links` to suppress symlink warnings (thanks Zhiming Wang)
* Mount
    * Re-use `rcat` internals to support uploads from all remotes
* Dropbox
    * Fix "entry doesn't belong in directory" error
    * Stop using deprecated API methods
* Swift
    * Fix server side copy to empty container with `--fast-list`
* Google Drive
    * Change the default for `--drive-use-trash` to `true`
* S3
    * Set session token when using STS (thanks Girish Ramakrishnan)
    * Glacier docs and error messages (thanks Jan Varho)
    * Read 1000 (not 1024) items in dir listings to fix Wasabi
* Backblaze B2
    * Fix SHA1 mismatch when downloading files with no SHA1
    * Calculate missing hashes on the fly instead of spooling
    * `--b2-hard-delete` to permanently delete (not hide) files (thanks John Papandriopoulos)
* Hubic
    * Fix creating containers - no longer have to use the `default` container
* Swift
    * Optionally configure from a standard set of OpenStack environment vars
    * Add `endpoint_type` config
* Google Cloud Storage
    * Fix bucket creation to work with limited permission users
* SFTP
    * Implement connection pooling for multiple ssh connections
    * Limit new connections per second
    * Add support for MD5 and SHA1 hashes where available (thanks Christian Brüggemann)
* HTTP
    * Fix URL encoding issues
    * Fix directories with `:` in
    * Fix panic with URL encoded content

## v1.37 - 2017-07-22

* New backends
    * FTP - thanks to Antonio Messina
    * HTTP - thanks to Vasiliy Tolstov
* New commands
    * rclone ncdu - for exploring a remote with a text based user interface.
    * rclone lsjson - for listing with a machine readable output
    * rclone dbhashsum - to show Dropbox style hashes of files (local or Dropbox)
* New Features
    * Implement --fast-list flag
        * This allows remotes to list recursively if they can
        * This uses less transactions (important if you pay for them)
        * This may or may not be quicker
        * This will use more memory as it has to hold the listing in memory
        * --old-sync-method deprecated - the remaining uses are covered by --fast-list
        * This involved a major re-write of all the listing code
    * Add --tpslimit and --tpslimit-burst to limit transactions per second
        * this is useful in conjunction with `rclone mount` to limit external apps
    * Add --stats-log-level so can see --stats without -v
    * Print password prompts to stderr - Hraban Luyat
    * Warn about duplicate files when syncing
    * Oauth improvements
        * allow auth_url and token_url to be set in the config file
        * Print redirection URI if using own credentials.
    * Don't Mkdir at the start of sync to save transactions
* Compile
    * Update build to go1.8.3
    * Require go1.6 for building rclone
    * Compile 386 builds with "GO386=387" for maximum compatibility
* Bug Fixes
    * Fix menu selection when no remotes
    * Config saving reworked to not kill the file if disk gets full
    * Don't delete remote if name does not change while renaming
    * moveto, copyto: report transfers and checks as per move and copy
* Local
    * Add --local-no-unicode-normalization flag - Bob Potter
* Mount
    * Now supported on Windows using cgofuse and WinFsp - thanks to Bill Zissimopoulos for much help
    * Compare checksums on upload/download via FUSE
    * Unmount when program ends with SIGINT (Ctrl+C) or SIGTERM - Jérôme Vizcaino
    * On read only open of file, make open pending until first read
    * Make --read-only reject modify operations
    * Implement ModTime via FUSE for remotes that support it
    * Allow modTime to be changed even before all writers are closed
    * Fix panic on renames
    * Fix hang on errored upload
* Crypt
    * Report the name:root as specified by the user
    * Add an "obfuscate" option for filename encryption - Stephen Harris
* Amazon Drive
    * Fix initialization order for token renewer
    * Remove revoked credentials, allow oauth proxy config and update docs
* B2
    * Reduce minimum chunk size to 5MB
* Drive
    * Add team drive support
    * Reduce bandwidth by adding fields for partial responses - Martin Kristensen
    * Implement --drive-shared-with-me flag to view shared with me files - Danny Tsai
    * Add --drive-trashed-only to read only the files in the trash
    * Remove obsolete --drive-full-list
    * Add missing seek to start on retries of chunked uploads
    * Fix stats accounting for upload
    * Convert / in names to a unicode equivalent (／)
    * Poll for Google Drive changes when mounted
* OneDrive
    * Fix the uploading of files with spaces
    * Fix initialization order for token renewer
    * Display speeds accurately when uploading - Yoni Jah
    * Swap to using http://localhost:53682/ as redirect URL - Michael Ledin
    * Retry on token expired error, reset upload body on retry - Yoni Jah
* Google Cloud Storage
    * Add ability to specify location and storage class via config and command line - thanks gdm85
    * Create container if necessary on server side copy
    * Increase directory listing chunk to 1000 to increase performance
    * Obtain a refresh token for GCS - Steven Lu
* Yandex
    * Fix the name reported in log messages (was empty)
    * Correct error return for listing empty directory
* Dropbox
    * Rewritten to use the v2 API
        * Now supports ModTime
            * Can only set by uploading the file again
            * If you uploaded with an old rclone, rclone may upload everything again
            * Use `--size-only` or `--checksum` to avoid this
        * Now supports the Dropbox content hashing scheme
        * Now supports low level retries
* S3
    * Work around eventual consistency in bucket creation
    * Create container if necessary on server side copy
    * Add us-east-2 (Ohio) and eu-west-2 (London) S3 regions - Zahiar Ahmed
* Swift, Hubic
    * Fix zero length directory markers showing in the subdirectory listing
        * this caused lots of duplicate transfers
    * Fix paged directory listings
        * this caused duplicate directory errors
    * Create container if necessary on server side copy
    * Increase directory listing chunk to 1000 to increase performance
    * Make sensible error if the user forgets the container
* SFTP
    * Add support for using ssh key files
    * Fix under Windows
    * Fix ssh agent on Windows
    * Adapt to latest version of library - Igor Kharin

## v1.36 - 2017-03-18

* New Features
    * SFTP remote (Jack Schmidt)
    * Re-implement sync routine to work a directory at a time reducing memory usage
    * Logging revamped to be more inline with rsync - now much quieter
            * -v only shows transfers
            * -vv is for full debug
            * --syslog to log to syslog on capable platforms
    * Implement --backup-dir and --suffix
    * Implement --track-renames (initial implementation by Bjørn Erik Pedersen)
    * Add time-based bandwidth limits (Lukas Loesche)
    * rclone cryptcheck: checks integrity of crypt remotes
    * Allow all config file variables and options to be set from environment variables
    * Add --buffer-size parameter to control buffer size for copy
    * Make --delete-after the default
    * Add --ignore-checksum flag (fixed by Hisham Zarka)
    * rclone check: Add --download flag to check all the data, not just hashes
    * rclone cat: add --head, --tail, --offset, --count and --discard
    * rclone config: when choosing from a list, allow the value to be entered too
    * rclone config: allow rename and copy of remotes
    * rclone obscure: for generating encrypted passwords for rclone's config (T.C. Ferguson)
    * Comply with XDG Base Directory specification (Dario Giovannetti)
        * this moves the default location of the config file in a backwards compatible way
    * Release changes
        * Ubuntu snap support (Dedsec1)
        * Compile with go 1.8
        * MIPS/Linux big and little endian support
* Bug Fixes
    * Fix copyto copying things to the wrong place if the destination dir didn't exist
    * Fix parsing of remotes in moveto and copyto
    * Fix --delete-before deleting files on copy
    * Fix --files-from with an empty file copying everything
    * Fix sync: don't update mod times if --dry-run set
    * Fix MimeType propagation
    * Fix filters to add ** rules to directory rules
* Local
    * Implement -L, --copy-links flag to allow rclone to follow symlinks
    * Open files in write only mode so rclone can write to an rclone mount
    * Fix unnormalised unicode causing problems reading directories
    * Fix interaction between -x flag and --max-depth
* Mount
    * Implement proper directory handling (mkdir, rmdir, renaming)
    * Make include and exclude filters apply to mount
    * Implement read and write async buffers - control with --buffer-size
    * Fix fsync on for directories
    * Fix retry on network failure when reading off crypt
* Crypt
    * Add --crypt-show-mapping to show encrypted file mapping
    * Fix crypt writer getting stuck in a loop
        * **IMPORTANT** this bug had the potential to cause data corruption when
            * reading data from a network based remote and
            * writing to a crypt on Google Drive
        * Use the cryptcheck command to validate your data if you are concerned
        * If syncing two crypt remotes, sync the unencrypted remote
* Amazon Drive
    * Fix panics on Move (rename)
    * Fix panic on token expiry
* B2
    * Fix inconsistent listings and rclone check
    * Fix uploading empty files with go1.8
    * Constrain memory usage when doing multipart uploads
    * Fix upload url not being refreshed properly
* Drive
    * Fix Rmdir on directories with trashed files
    * Fix "Ignoring unknown object" when downloading
    * Add --drive-list-chunk
    * Add --drive-skip-gdocs (Károly Oláh)
* OneDrive
    * Implement Move
    * Fix Copy
        * Fix overwrite detection in Copy
        * Fix waitForJob to parse errors correctly
    * Use token renewer to stop auth errors on long uploads
    * Fix uploading empty files with go1.8
* Google Cloud Storage
    * Fix depth 1 directory listings
* Yandex
    * Fix single level directory listing
* Dropbox
    * Normalise the case for single level directory listings
    * Fix depth 1 listing
* S3
    * Added ca-central-1 region (Jon Yergatian)

## v1.35 - 2017-01-02

* New Features
    * moveto and copyto commands for choosing a destination name on copy/move
    * rmdirs command to recursively delete empty directories
    * Allow repeated --include/--exclude/--filter options
    * Only show transfer stats on commands which transfer stuff
        * show stats on any command using the `--stats` flag
    * Allow overlapping directories in move when server side dir move is supported
    * Add --stats-unit option - thanks Scott McGillivray
* Bug Fixes
    * Fix the config file being overwritten when two rclone instances are running
    * Make rclone lsd obey the filters properly
    * Fix compilation on mips
    * Fix not transferring files that don't differ in size
    * Fix panic on nil retry/fatal error
* Mount
    * Retry reads on error - should help with reliability a lot
    * Report the modification times for directories from the remote
    * Add bandwidth accounting and limiting (fixes --bwlimit)
    * If --stats provided will show stats and which files are transferring
    * Support R/W files if truncate is set.
    * Implement statfs interface so df works
    * Note that write is now supported on Amazon Drive
    * Report number of blocks in a file - thanks Stefan Breunig
* Crypt
    * Prevent the user pointing crypt at itself
    * Fix failed to authenticate decrypted block errors
        * these will now return the underlying unexpected EOF instead
* Amazon Drive
    * Add support for server side move and directory move - thanks Stefan Breunig
    * Fix nil pointer deref on size attribute
* B2
    * Use new prefix and delimiter parameters in directory listings
        * This makes --max-depth 1 dir listings as used in mount much faster
    * Reauth the account while doing uploads too - should help with token expiry
* Drive
    * Make DirMove more efficient and complain about moving the root
    * Create destination directory on Move()

## v1.34 - 2016-11-06

* New Features
    * Stop single file and `--files-from` operations iterating through the source bucket.
    * Stop removing failed upload to cloud storage remotes
    * Make ContentType be preserved for cloud to cloud copies
    * Add support to toggle bandwidth limits via SIGUSR2 - thanks Marco Paganini
    * `rclone check` shows count of hashes that couldn't be checked
    * `rclone listremotes` command
    * Support linux/arm64 build - thanks Fredrik Fornwall
    * Remove `Authorization:` lines from `--dump-headers` output
* Bug Fixes
    * Ignore files with control characters in the names
    * Fix `rclone move` command
        * Delete src files which already existed in dst
        * Fix deletion of src file when dst file older
    * Fix `rclone check` on crypted file systems
    * Make failed uploads not count as "Transferred"
    * Make sure high level retries show with `-q`
    * Use a vendor directory with godep for repeatable builds
* `rclone mount` - FUSE
    * Implement FUSE mount options
        * `--no-modtime`, `--debug-fuse`, `--read-only`, `--allow-non-empty`, `--allow-root`, `--allow-other`
        * `--default-permissions`, `--write-back-cache`, `--max-read-ahead`, `--umask`, `--uid`, `--gid`
    * Add `--dir-cache-time` to control caching of directory entries
    * Implement seek for files opened for read (useful for video players)
        * with `-no-seek` flag to disable
    * Fix crash on 32 bit ARM (alignment of 64 bit counter)
    * ...and many more internal fixes and improvements!
* Crypt
    * Don't show encrypted password in configurator to stop confusion
* Amazon Drive
    * New wait for upload option `--acd-upload-wait-per-gb`
        * upload timeouts scale by file size and can be disabled
    * Add 502 Bad Gateway to list of errors we retry
    * Fix overwriting a file with a zero length file
    * Fix ACD file size warning limit - thanks Felix Bünemann
* Local
    * Unix: implement `-x`/`--one-file-system` to stay on a single file system
        * thanks Durval Menezes and Luiz Carlos Rumbelsperger Viana
    * Windows: ignore the symlink bit on files
    * Windows: Ignore directory based junction points
* B2
    * Make sure each upload has at least one upload slot - fixes strange upload stats
    * Fix uploads when using crypt
    * Fix download of large files (sha1 mismatch)
    * Return error when we try to create a bucket which someone else owns
    * Update B2 docs with Data usage, and Crypt section - thanks Tomasz Mazur
* S3
    * Command line and config file support for
        * Setting/overriding ACL  - thanks Radek Senfeld
        * Setting storage class - thanks Asko Tamm
* Drive
    * Make exponential backoff work exactly as per Google specification
    * add `.epub`, `.odp` and `.tsv` as export formats.
* Swift
    * Don't read metadata for directory marker objects

## v1.33 - 2016-08-24

* New Features
    * Implement encryption
        * data encrypted in NACL secretbox format
        * with optional file name encryption
    * New commands
        * rclone mount - implements FUSE mounting of remotes (EXPERIMENTAL)
            * works on Linux, FreeBSD and OS X (need testers for the last 2!)
        * rclone cat - outputs remote file or files to the terminal
        * rclone genautocomplete - command to make a bash completion script for rclone
    * Editing a remote using `rclone config` now goes through the wizard
    * Compile with go 1.7 - this fixes rclone on macOS Sierra and on 386 processors
    * Use cobra for sub commands and docs generation
* drive
    * Document how to make your own client_id
* s3
    * User-configurable Amazon S3 ACL (thanks Radek Šenfeld)
* b2
    * Fix stats accounting for upload - no more jumping to 100% done
    * On cleanup delete hide marker if it is the current file
    * New B2 API endpoint (thanks Per Cederberg)
    * Set maximum backoff to 5 Minutes
* onedrive
    * Fix URL escaping in file names - eg uploading files with `+` in them.
* amazon cloud drive
    * Fix token expiry during large uploads
    * Work around 408 REQUEST_TIMEOUT and 504 GATEWAY_TIMEOUT errors
* local
    * Fix filenames with invalid UTF-8 not being uploaded
    * Fix problem with some UTF-8 characters on OS X

## v1.32 - 2016-07-13

* Backblaze B2
    * Fix upload of files large files not in root

## v1.31 - 2016-07-13

* New Features
    * Reduce memory on sync by about 50%
    * Implement --no-traverse flag to stop copy traversing the destination remote.
        * This can be used to reduce memory usage down to the smallest possible.
        * Useful to copy a small number of files into a large destination folder.
    * Implement cleanup command for emptying trash / removing old versions of files
        * Currently B2 only
    * Single file handling improved
        * Now copied with --files-from
        * Automatically sets --no-traverse when copying a single file
    * Info on using installing with ansible - thanks Stefan Weichinger
    * Implement --no-update-modtime flag to stop rclone fixing the remote modified times.
* Bug Fixes
    * Fix move command - stop it running for overlapping Fses - this was causing data loss.
* Local
    * Fix incomplete hashes - this was causing problems for B2.
* Amazon Drive
    * Rename Amazon Cloud Drive to Amazon Drive - no changes to config file needed.
* Swift
    * Add support for non-default project domain - thanks Antonio Messina.
* S3
    * Add instructions on how to use rclone with minio.
    * Add ap-northeast-2 (Seoul) and ap-south-1 (Mumbai) regions.
    * Skip setting the modified time for objects > 5GB as it isn't possible.
* Backblaze B2
    * Add --b2-versions flag so old versions can be listed and retrieved.
    * Treat 403 errors (eg cap exceeded) as fatal.
    * Implement cleanup command for deleting old file versions.
    * Make error handling compliant with B2 integrations notes.
    * Fix handling of token expiry.
    * Implement --b2-test-mode to set `X-Bz-Test-Mode` header.
    * Set cutoff for chunked upload to 200MB as per B2 guidelines.
    * Make upload multi-threaded.
* Dropbox
    * Don't retry 461 errors.

## v1.30 - 2016-06-18

* New Features
    * Directory listing code reworked for more features and better error reporting (thanks to Klaus Post for help).  This enables
        * Directory include filtering for efficiency
        * --max-depth parameter
        * Better error reporting
        * More to come
    * Retry more errors
    * Add --ignore-size flag - for uploading images to onedrive
    * Log -v output to stdout by default
    * Display the transfer stats in more human readable form
    * Make 0 size files specifiable with `--max-size 0b`
    * Add `b` suffix so we can specify bytes in --bwlimit, --min-size etc
    * Use "password:" instead of "password>" prompt - thanks Klaus Post and Leigh Klotz
* Bug Fixes
    * Fix retry doing one too many retries
* Local
    * Fix problems with OS X and UTF-8 characters
* Amazon Drive
    * Check a file exists before uploading to help with 408 Conflict errors
    * Reauth on 401 errors - this has been causing a lot of problems
    * Work around spurious 403 errors
    * Restart directory listings on error
* Google Drive
    * Check a file exists before uploading to help with duplicates
    * Fix retry of multipart uploads
* Backblaze B2
    * Implement large file uploading
* S3
    * Add AES256 server-side encryption for - thanks Justin R. Wilson
* Google Cloud Storage
    * Make sure we don't use conflicting content types on upload
    * Add service account support - thanks Michal Witkowski
* Swift
    * Add auth version parameter
    * Add domain option for openstack (v3 auth) - thanks Fabian Ruff

## v1.29 - 2016-04-18

* New Features
    * Implement `-I, --ignore-times` for unconditional upload
    * Improve `dedupe`command
        * Now removes identical copies without asking
        * Now obeys `--dry-run`
        * Implement `--dedupe-mode` for non interactive running
            * `--dedupe-mode interactive` - interactive the default.
            * `--dedupe-mode skip` - removes identical files then skips anything left.
            * `--dedupe-mode first` - removes identical files then keeps the first one.
            * `--dedupe-mode newest` - removes identical files then keeps the newest one.
            * `--dedupe-mode oldest` - removes identical files then keeps the oldest one.
            * `--dedupe-mode rename` - removes identical files then renames the rest to be different.
* Bug fixes
    * Make rclone check obey the `--size-only` flag.
    * Use "application/octet-stream" if discovered mime type is invalid.
    * Fix missing "quit" option when there are no remotes.
* Google Drive
    * Increase default chunk size to 8 MB - increases upload speed of big files
    * Speed up directory listings and make more reliable
    * Add missing retries for Move and DirMove - increases reliability
    * Preserve mime type on file update
* Backblaze B2
    * Enable mod time syncing
        * This means that B2 will now check modification times
        * It will upload new files to update the modification times
        * (there isn't an API to just set the mod time.)
        * If you want the old behaviour use `--size-only`.
    * Update API to new version
    * Fix parsing of mod time when not in metadata
* Swift/Hubic
    * Don't return an MD5SUM for static large objects
* S3
    * Fix uploading files bigger than 50GB

## v1.28 - 2016-03-01

* New Features
    * Configuration file encryption - thanks Klaus Post
    * Improve `rclone config` adding more help and making it easier to understand
    * Implement `-u`/`--update` so creation times can be used on all remotes
    * Implement `--low-level-retries` flag
    * Optionally disable gzip compression on downloads with `--no-gzip-encoding`
* Bug fixes
    * Don't make directories if `--dry-run` set
    * Fix and document the `move` command
    * Fix redirecting stderr on unix-like OSes when using `--log-file`
    * Fix `delete` command to wait until all finished - fixes missing deletes.
* Backblaze B2
    * Use one upload URL per go routine fixes `more than one upload using auth token`
    * Add pacing, retries and reauthentication - fixes token expiry problems
    * Upload without using a temporary file from local (and remotes which support SHA1)
    * Fix reading metadata for all files when it shouldn't have been
* Drive
    * Fix listing drive documents at root
    * Disable copy and move for Google docs
* Swift
    * Fix uploading of chunked files with non ASCII characters
    * Allow setting of `storage_url` in the config - thanks Xavier Lucas
* S3
    * Allow IAM role and credentials from environment variables - thanks Brian Stengaard
    * Allow low privilege users to use S3 (check if directory exists during Mkdir) - thanks Jakub Gedeon
* Amazon Drive
    * Retry on more things to make directory listings more reliable

## v1.27 - 2016-01-31

* New Features
    * Easier headless configuration with `rclone authorize`
    * Add support for multiple hash types - we now check SHA1 as well as MD5 hashes.
    * `delete` command which does obey the filters (unlike `purge`)
    * `dedupe` command to deduplicate a remote.  Useful with Google Drive.
    * Add `--ignore-existing` flag to skip all files that exist on destination.
    * Add `--delete-before`, `--delete-during`, `--delete-after` flags.
    * Add `--memprofile` flag to debug memory use.
    * Warn the user about files with same name but different case
    * Make `--include` rules add their implicit exclude * at the end of the filter list
    * Deprecate compiling with go1.3
* Amazon Drive
    * Fix download of files > 10 GB
    * Fix directory traversal ("Next token is expired") for large directory listings
    * Remove 409 conflict from error codes we will retry - stops very long pauses
* Backblaze B2
    * SHA1 hashes now checked by rclone core
* Drive
    * Add `--drive-auth-owner-only` to only consider files owned by the user - thanks Björn Harrtell
    * Export Google documents
* Dropbox
    * Make file exclusion error controllable with -q
* Swift
    * Fix upload from unprivileged user.
* S3
    * Fix updating of mod times of files with `+` in.
* Local
    * Add local file system option to disable UNC on Windows.

## v1.26 - 2016-01-02

* New Features
    * Yandex storage backend - thank you Dmitry Burdeev ("dibu")
    * Implement Backblaze B2 storage backend
    * Add --min-age and --max-age flags - thank you Adriano Aurélio Meirelles
    * Make ls/lsl/md5sum/size/check obey includes and excludes
* Fixes
    * Fix crash in http logging
    * Upload releases to github too
* Swift
    * Fix sync for chunked files
* OneDrive
    * Re-enable server side copy
    * Don't mask HTTP error codes with JSON decode error
* S3
    * Fix corrupting Content-Type on mod time update (thanks Joseph Spurrier)

## v1.25 - 2015-11-14

* New features
    * Implement Hubic storage system
* Fixes
    * Fix deletion of some excluded files without --delete-excluded
        * This could have deleted files unexpectedly on sync
        * Always check first with `--dry-run`!
* Swift
    * Stop SetModTime losing metadata (eg X-Object-Manifest)
        * This could have caused data loss for files > 5GB in size
    * Use ContentType from Object to avoid lookups in listings
* OneDrive
    * disable server side copy as it seems to be broken at Microsoft

## v1.24 - 2015-11-07

* New features
    * Add support for Microsoft OneDrive
    * Add `--no-check-certificate` option to disable server certificate verification
    * Add async readahead buffer for faster transfer of big files
* Fixes
    * Allow spaces in remotes and check remote names for validity at creation time
    * Allow '&' and disallow ':' in Windows filenames.
* Swift
    * Ignore directory marker objects where appropriate - allows working with Hubic
    * Don't delete the container if fs wasn't at root
* S3
    * Don't delete the bucket if fs wasn't at root
* Google Cloud Storage
    * Don't delete the bucket if fs wasn't at root

## v1.23 - 2015-10-03

* New features
    * Implement `rclone size` for measuring remotes
* Fixes
    * Fix headless config for drive and gcs
    * Tell the user they should try again if the webserver method failed
    * Improve output of `--dump-headers`
* S3
    * Allow anonymous access to public buckets
* Swift
    * Stop chunked operations logging "Failed to read info: Object Not Found"
    * Use Content-Length on uploads for extra reliability

## v1.22 - 2015-09-28

* Implement rsync like include and exclude flags
* swift
    * Support files > 5GB - thanks Sergey Tolmachev

## v1.21 - 2015-09-22

* New features
    * Display individual transfer progress
    * Make lsl output times in localtime
* Fixes
    * Fix allowing user to override credentials again in Drive, GCS and ACD
* Amazon Drive
    * Implement compliant pacing scheme
* Google Drive
    * Make directory reads concurrent for increased speed.

## v1.20 - 2015-09-15

* New features
    * Amazon Drive support
    * Oauth support redone - fix many bugs and improve usability
        * Use "golang.org/x/oauth2" as oauth library of choice
        * Improve oauth usability for smoother initial signup
        * drive, googlecloudstorage: optionally use auto config for the oauth token
    * Implement --dump-headers and --dump-bodies debug flags
    * Show multiple matched commands if abbreviation too short
    * Implement server side move where possible
* local
    * Always use UNC paths internally on Windows - fixes a lot of bugs
* dropbox
    * force use of our custom transport which makes timeouts work
* Thanks to Klaus Post for lots of help with this release

## v1.19 - 2015-08-28

* New features
    * Server side copies for s3/swift/drive/dropbox/gcs
    * Move command - uses server side copies if it can
    * Implement --retries flag - tries 3 times by default
    * Build for plan9/amd64 and solaris/amd64 too
* Fixes
    * Make a current version download with a fixed URL for scripting
    * Ignore rmdir in limited fs rather than throwing error
* dropbox
    * Increase chunk size to improve upload speeds massively
    * Issue an error message when trying to upload bad file name

## v1.18 - 2015-08-17

* drive
    * Add `--drive-use-trash` flag so rclone trashes instead of deletes
    * Add "Forbidden to download" message for files with no downloadURL
* dropbox
    * Remove datastore
        * This was deprecated and it caused a lot of problems
        * Modification times and MD5SUMs no longer stored
    * Fix uploading files > 2GB
* s3
    * use official AWS SDK from github.com/aws/aws-sdk-go
    * **NB** will most likely require you to delete and recreate remote
    * enable multipart upload which enables files > 5GB
    * tested with Ceph / RadosGW / S3 emulation
    * many thanks to Sam Liston and Brian Haymore at the [Utah Center for High Performance Computing](https://www.chpc.utah.edu/) for a Ceph test account
* misc
    * Show errors when reading the config file
    * Do not print stats in quiet mode - thanks Leonid Shalupov
    * Add FAQ
    * Fix created directories not obeying umask
    * Linux installation instructions - thanks Shimon Doodkin

## v1.17 - 2015-06-14

* dropbox: fix case insensitivity issues - thanks Leonid Shalupov

## v1.16 - 2015-06-09

* Fix uploading big files which was causing timeouts or panics
* Don't check md5sum after download with --size-only

## v1.15 - 2015-06-06

* Add --checksum flag to only discard transfers by MD5SUM - thanks Alex Couper
* Implement --size-only flag to sync on size not checksum & modtime
* Expand docs and remove duplicated information
* Document rclone's limitations with directories
* dropbox: update docs about case insensitivity

## v1.14 - 2015-05-21

* local: fix encoding of non utf-8 file names - fixes a duplicate file problem
* drive: docs about rate limiting
* google cloud storage: Fix compile after API change in "google.golang.org/api/storage/v1"

## v1.13 - 2015-05-10

* Revise documentation (especially sync)
* Implement --timeout and --conntimeout
* s3: ignore etags from multipart uploads which aren't md5sums

## v1.12 - 2015-03-15

* drive: Use chunked upload for files above a certain size
* drive: add --drive-chunk-size and --drive-upload-cutoff parameters
* drive: switch to insert from update when a failed copy deletes the upload
* core: Log duplicate files if they are detected

## v1.11 - 2015-03-04

* swift: add region parameter
* drive: fix crash on failed to update remote mtime
* In remote paths, change native directory separators to /
* Add synchronization to ls/lsl/lsd output to stop corruptions
* Ensure all stats/log messages to go stderr
* Add --log-file flag to log everything (including panics) to file
* Make it possible to disable stats printing with --stats=0
* Implement --bwlimit to limit data transfer bandwidth

## v1.10 - 2015-02-12

* s3: list an unlimited number of items
* Fix getting stuck in the configurator

## v1.09 - 2015-02-07

* windows: Stop drive letters (eg C:) getting mixed up with remotes (eg drive:)
* local: Fix directory separators on Windows
* drive: fix rate limit exceeded errors

## v1.08 - 2015-02-04

* drive: fix subdirectory listing to not list entire drive
* drive: Fix SetModTime
* dropbox: adapt code to recent library changes

## v1.07 - 2014-12-23

* google cloud storage: fix memory leak

## v1.06 - 2014-12-12

* Fix "Couldn't find home directory" on OSX
* swift: Add tenant parameter
* Use new location of Google API packages

## v1.05 - 2014-08-09

* Improved tests and consequently lots of minor fixes
* core: Fix race detected by go race detector
* core: Fixes after running errcheck
* drive: reset root directory on Rmdir and Purge
* fs: Document that Purger returns error on empty directory, test and fix
* google cloud storage: fix ListDir on subdirectory
* google cloud storage: re-read metadata in SetModTime
* s3: make reading metadata more reliable to work around eventual consistency problems
* s3: strip trailing / from ListDir()
* swift: return directories without / in ListDir

## v1.04 - 2014-07-21

* google cloud storage: Fix crash on Update

## v1.03 - 2014-07-20

* swift, s3, dropbox: fix updated files being marked as corrupted
* Make compile with go 1.1 again

## v1.02 - 2014-07-19

* Implement Dropbox remote
* Implement Google Cloud Storage remote
* Verify Md5sums and Sizes after copies
* Remove times from "ls" command - lists sizes only
* Add add "lsl" - lists times and sizes
* Add "md5sum" command

## v1.01 - 2014-07-04

* drive: fix transfer of big files using up lots of memory

## v1.00 - 2014-07-03

* drive: fix whole second dates

## v0.99 - 2014-06-26

* Fix --dry-run not working
* Make compatible with go 1.1

## v0.98 - 2014-05-30

* s3: Treat missing Content-Length as 0 for some ceph installations
* rclonetest: add file with a space in

## v0.97 - 2014-05-05

* Implement copying of single files
* s3 & swift: support paths inside containers/buckets

## v0.96 - 2014-04-24

* drive: Fix multiple files of same name being created
* drive: Use o.Update and fs.Put to optimise transfers
* Add version number, -V and --version

## v0.95 - 2014-03-28

* rclone.org: website, docs and graphics
* drive: fix path parsing

## v0.94 - 2014-03-27

* Change remote format one last time
* GNU style flags

## v0.93 - 2014-03-16

* drive: store token in config file
* cross compile other versions
* set strict permissions on config file

## v0.92 - 2014-03-15

* Config fixes and --config option

## v0.91 - 2014-03-15

* Make config file

## v0.90 - 2013-06-27

* Project named rclone

## v0.00 - 2012-11-18

* Project started

# Bugs and Limitations

## Limitations

### Directory timestamps aren't preserved

Rclone doesn't currently preserve the timestamps of directories.  This
is because rclone only really considers objects when syncing.

### Rclone struggles with millions of files in a directory

Currently rclone loads each directory entirely into memory before
using it.  Since each Rclone object takes 0.5k-1k of memory this can
take a very long time and use an extremely large amount of memory.

Millions of files in a directory tend caused by software writing cloud
storage (eg S3 buckets).

### Bucket based remotes and folders

Bucket based remotes (eg S3/GCS/Swift/B2) do not have a concept of
directories.  Rclone therefore cannot create directories in them which
means that empty directories on a bucket based remote will tend to
disappear.

Some software creates empty keys ending in `/` as directory markers.
Rclone doesn't do this as it potentially creates more objects and
costs more.  It may do in future (probably with a flag).

## Bugs

Bugs are stored in rclone's GitHub project:

* [Reported bugs](https://github.com/rclone/rclone/issues?q=is%3Aopen+is%3Aissue+label%3Abug)
* [Known issues](https://github.com/rclone/rclone/issues?q=is%3Aopen+is%3Aissue+milestone%3A%22Known+Problem%22)

Frequently Asked Questions
--------------------------

### Do all cloud storage systems support all rclone commands ###

Yes they do.  All the rclone commands (eg `sync`, `copy` etc) will
work on all the remote storage systems.

### Can I copy the config from one machine to another ###

Sure!  Rclone stores all of its config in a single file.  If you want
to find this file, run `rclone config file` which will tell you where
it is.

See the [remote setup docs](https://rclone.org/remote_setup/) for more info.

### How do I configure rclone on a remote / headless box with no browser? ###

This has now been documented in its own [remote setup page](https://rclone.org/remote_setup/).

### Can rclone sync directly from drive to s3 ###

Rclone can sync between two remote cloud storage systems just fine.

Note that it effectively downloads the file and uploads it again, so
the node running rclone would need to have lots of bandwidth.

The syncs would be incremental (on a file by file basis).

Eg

    rclone sync drive:Folder s3:bucket


### Using rclone from multiple locations at the same time ###

You can use rclone from multiple places at the same time if you choose
different subdirectory for the output, eg

```
Server A> rclone sync /tmp/whatever remote:ServerA
Server B> rclone sync /tmp/whatever remote:ServerB
```

If you sync to the same directory then you should use rclone copy
otherwise the two instances of rclone may delete each other's files, eg

```
Server A> rclone copy /tmp/whatever remote:Backup
Server B> rclone copy /tmp/whatever remote:Backup
```

The file names you upload from Server A and Server B should be
different in this case, otherwise some file systems (eg Drive) may
make duplicates.

### Why doesn't rclone support partial transfers / binary diffs like rsync? ###

Rclone stores each file you transfer as a native object on the remote
cloud storage system.  This means that you can see the files you
upload as expected using alternative access methods (eg using the
Google Drive web interface).  There is a 1:1 mapping between files on
your hard disk and objects created in the cloud storage system.

Cloud storage systems (at least none I've come across yet) don't
support partially uploading an object. You can't take an existing
object, and change some bytes in the middle of it.

It would be possible to make a sync system which stored binary diffs
instead of whole objects like rclone does, but that would break the
1:1 mapping of files on your hard disk to objects in the remote cloud
storage system.

All the cloud storage systems support partial downloads of content, so
it would be possible to make partial downloads work.  However to make
this work efficiently this would require storing a significant amount
of metadata, which breaks the desired 1:1 mapping of files to objects.

### Can rclone do bi-directional sync? ###

No, not at present.  rclone only does uni-directional sync from A ->
B. It may do in the future though since it has all the primitives - it
just requires writing the algorithm to do it.

### Can I use rclone with an HTTP proxy? ###

Yes. rclone will follow the standard environment variables for
proxies, similar to cURL and other programs.

In general the variables are called `http_proxy` (for services reached
over `http`) and `https_proxy` (for services reached over `https`).  Most
public services will be using `https`, but you may wish to set both.

The content of the variable is `protocol://server:port`.  The protocol
value is the one used to talk to the proxy server, itself, and is commonly
either `http` or `socks5`.

Slightly annoyingly, there is no _standard_ for the name; some applications
may use `http_proxy` but another one `HTTP_PROXY`.  The `Go` libraries
used by `rclone` will try both variations, but you may wish to set all
possibilities.  So, on Linux, you may end up with code similar to

    export http_proxy=http://proxyserver:12345
    export https_proxy=$http_proxy
    export HTTP_PROXY=$http_proxy
    export HTTPS_PROXY=$http_proxy

The `NO_PROXY` allows you to disable the proxy for specific hosts.
Hosts must be comma separated, and can contain domains or parts.
For instance "foo.com" also matches "bar.foo.com".

e.g.

    export no_proxy=localhost,127.0.0.0/8,my.host.name
    export NO_PROXY=$no_proxy

Note that the ftp backend does not support `ftp_proxy` yet.

### Rclone gives x509: failed to load system roots and no roots provided error ###

This means that `rclone` can't file the SSL root certificates.  Likely
you are running `rclone` on a NAS with a cut-down Linux OS, or
possibly on Solaris.

Rclone (via the Go runtime) tries to load the root certificates from
these places on Linux.

    "/etc/ssl/certs/ca-certificates.crt", // Debian/Ubuntu/Gentoo etc.
    "/etc/pki/tls/certs/ca-bundle.crt",   // Fedora/RHEL
    "/etc/ssl/ca-bundle.pem",             // OpenSUSE
    "/etc/pki/tls/cacert.pem",            // OpenELEC

So doing something like this should fix the problem.  It also sets the
time which is important for SSL to work properly.

```
mkdir -p /etc/ssl/certs/
curl -o /etc/ssl/certs/ca-certificates.crt https://raw.githubusercontent.com/bagder/ca-bundle/master/ca-bundle.crt
ntpclient -s -h pool.ntp.org
```

The two environment variables `SSL_CERT_FILE` and `SSL_CERT_DIR`, mentioned in the [x509 package](https://godoc.org/crypto/x509),
provide an additional way to provide the SSL root certificates.

Note that you may need to add the `--insecure` option to the `curl` command line if it doesn't work without.

```
curl --insecure -o /etc/ssl/certs/ca-certificates.crt https://raw.githubusercontent.com/bagder/ca-bundle/master/ca-bundle.crt
```

### Rclone gives Failed to load config file: function not implemented error ###

Likely this means that you are running rclone on Linux version not
supported by the go runtime, ie earlier than version 2.6.23.

See the [system requirements section in the go install
docs](https://golang.org/doc/install) for full details.

### All my uploaded docx/xlsx/pptx files appear as archive/zip ###

This is caused by uploading these files from a Windows computer which
hasn't got the Microsoft Office suite installed.  The easiest way to
fix is to install the Word viewer and the Microsoft Office
Compatibility Pack for Word, Excel, and PowerPoint 2007 and later
versions' file formats

### tcp lookup some.domain.com no such host ###

This happens when rclone cannot resolve a domain. Please check that
your DNS setup is generally working, e.g.

```
# both should print a long list of possible IP addresses
dig www.googleapis.com          # resolve using your default DNS
dig www.googleapis.com @8.8.8.8 # resolve with Google's DNS server
```

If you are using `systemd-resolved` (default on Arch Linux), ensure it
is at version 233 or higher. Previous releases contain a bug which
causes not all domains to be resolved properly.

Additionally with the `GODEBUG=netdns=` environment variable the Go
resolver decision can be influenced. This also allows to resolve certain
issues with DNS resolution. See the [name resolution section in the go docs](https://golang.org/pkg/net/#hdr-Name_Resolution).

### The total size reported in the stats for a sync is wrong and keeps changing

It is likely you have more than 10,000 files that need to be
synced. By default rclone only gets 10,000 files ahead in a sync so as
not to use up too much memory. You can change this default with the
[--max-backlog](https://rclone.org/docs/#max-backlog-n) flag.

### Rclone is using too much memory or appears to have a memory leak

Rclone is written in Go which uses a garbage collector.  The default
settings for the garbage collector mean that it runs when the heap
size has doubled.

However it is possible to tune the garbage collector to use less
memory by [setting GOGC](https://dave.cheney.net/tag/gogc) to a lower
value, say `export GOGC=20`.  This will make the garbage collector
work harder, reducing memory size at the expense of CPU usage.

The most common cause of rclone using lots of memory is a single
directory with thousands or millions of files in.  Rclone has to load
this entirely into memory as rclone objects.  Each rclone object takes
0.5k-1k of memory.

License
-------

This is free software under the terms of MIT the license (check the
COPYING file included with the source code).

```
Copyright (C) 2019 by Nick Craig-Wood https://www.craig-wood.com/nick/

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
```

Authors
-------

  * Nick Craig-Wood <nick@craig-wood.com>

Contributors
------------

{{< rem `email addresses removed from here need to be addeed to
bin/.ignore-emails to make sure update-authors.py doesn't immediately
put them back in again.` >}}

  * Alex Couper <amcouper@gmail.com>
  * Leonid Shalupov <leonid@shalupov.com> <shalupov@diverse.org.ru>
  * Shimon Doodkin <helpmepro1@gmail.com>
  * Colin Nicholson <colin@colinn.com>
  * Klaus Post <klauspost@gmail.com>
  * Sergey Tolmachev <tolsi.ru@gmail.com>
  * Adriano Aurélio Meirelles <adriano@atinge.com>
  * C. Bess <cbess@users.noreply.github.com>
  * Dmitry Burdeev <dibu28@gmail.com>
  * Joseph Spurrier <github@josephspurrier.com>
  * Björn Harrtell <bjorn@wololo.org>
  * Xavier Lucas <xavier.lucas@corp.ovh.com>
  * Werner Beroux <werner@beroux.com>
  * Brian Stengaard <brian@stengaard.eu>
  * Jakub Gedeon <jgedeon@sofi.com>
  * Jim Tittsler <jwt@onjapan.net>
  * Michal Witkowski <michal@improbable.io>
  * Fabian Ruff <fabian.ruff@sap.com>
  * Leigh Klotz <klotz@quixey.com>
  * Romain Lapray <lapray.romain@gmail.com>
  * Justin R. Wilson <jrw972@gmail.com>
  * Antonio Messina <antonio.s.messina@gmail.com>
  * Stefan G. Weichinger <office@oops.co.at>
  * Per Cederberg <cederberg@gmail.com>
  * Radek Šenfeld <rush@logic.cz>
  * Fredrik Fornwall <fredrik@fornwall.net>
  * Asko Tamm <asko@deekit.net>
  * xor-zz <xor@gstocco.com>
  * Tomasz Mazur <tmazur90@gmail.com>
  * Marco Paganini <paganini@paganini.net>
  * Felix Bünemann <buenemann@louis.info>
  * Durval Menezes <jmrclone@durval.com>
  * Luiz Carlos Rumbelsperger Viana <maxd13_luiz_carlos@hotmail.com>
  * Stefan Breunig <stefan-github@yrden.de>
  * Alishan Ladhani <ali-l@users.noreply.github.com>
  * 0xJAKE <0xJAKE@users.noreply.github.com>
  * Thibault Molleman <thibaultmol@users.noreply.github.com>
  * Scott McGillivray <scott.mcgillivray@gmail.com>
  * Bjørn Erik Pedersen <bjorn.erik.pedersen@gmail.com>
  * Lukas Loesche <lukas@mesosphere.io>
  * emyarod <allllaboutyou@gmail.com>
  * T.C. Ferguson <tcf909@gmail.com>
  * Brandur <brandur@mutelight.org>
  * Dario Giovannetti <dev@dariogiovannetti.net>
  * Károly Oláh <okaresz@aol.com>
  * Jon Yergatian <jon@macfanatic.ca>
  * Jack Schmidt <github@mowsey.org>
  * Dedsec1 <Dedsec1@users.noreply.github.com>
  * Hisham Zarka <hzarka@gmail.com>
  * Jérôme Vizcaino <jerome.vizcaino@gmail.com>
  * Mike Tesch <mjt6129@rit.edu>
  * Marvin Watson <marvwatson@users.noreply.github.com>
  * Danny Tsai <danny8376@gmail.com>
  * Yoni Jah <yonjah+git@gmail.com> <yonjah+github@gmail.com>
  * Stephen Harris <github@spuddy.org> <sweharris@users.noreply.github.com>
  * Ihor Dvoretskyi <ihor.dvoretskyi@gmail.com>
  * Jon Craton <jncraton@gmail.com>
  * Hraban Luyat <hraban@0brg.net>
  * Michael Ledin <mledin89@gmail.com>
  * Martin Kristensen <me@azgul.com>
  * Too Much IO <toomuchio@users.noreply.github.com>
  * Anisse Astier <anisse@astier.eu>
  * Zahiar Ahmed <zahiar@live.com>
  * Igor Kharin <igorkharin@gmail.com>
  * Bill Zissimopoulos <billziss@navimatics.com>
  * Bob Potter <bobby.potter@gmail.com>
  * Steven Lu <tacticalazn@gmail.com>
  * Sjur Fredriksen <sjurtf@ifi.uio.no>
  * Ruwbin <hubus12345@gmail.com>
  * Fabian Möller <fabianm88@gmail.com> <f.moeller@nynex.de>
  * Edward Q. Bridges <github@eqbridges.com>
  * Vasiliy Tolstov <v.tolstov@selfip.ru>
  * Harshavardhana <harsha@minio.io>
  * sainaen <sainaen@gmail.com>
  * gdm85 <gdm85@users.noreply.github.com>
  * Yaroslav Halchenko <debian@onerussian.com>
  * John Papandriopoulos <jpap@users.noreply.github.com>
  * Zhiming Wang <zmwangx@gmail.com>
  * Andy Pilate <cubox@cubox.me>
  * Oliver Heyme <olihey@googlemail.com> <olihey@users.noreply.github.com> <de8olihe@lego.com>
  * wuyu <wuyu@yunify.com>
  * Andrei Dragomir <adragomi@adobe.com>
  * Christian Brüggemann <mail@cbruegg.com>
  * Alex McGrath Kraak <amkdude@gmail.com>
  * bpicode <bjoern.pirnay@googlemail.com>
  * Daniel Jagszent <daniel@jagszent.de>
  * Josiah White <thegenius2009@gmail.com>
  * Ishuah Kariuki <kariuki@ishuah.com> <ishuah91@gmail.com>
  * Jan Varho <jan@varho.org>
  * Girish Ramakrishnan <girish@cloudron.io>
  * LingMan <LingMan@users.noreply.github.com>
  * Jacob McNamee <jacobmcnamee@gmail.com>
  * jersou <jertux@gmail.com>
  * thierry <thierry@substantiel.fr>
  * Simon Leinen <simon.leinen@gmail.com> <ubuntu@s3-test.novalocal>
  * Dan Dascalescu <ddascalescu+github@gmail.com>
  * Jason Rose <jason@jro.io>
  * Andrew Starr-Bochicchio <a.starr.b@gmail.com>
  * John Leach <john@johnleach.co.uk>
  * Corban Raun <craun@instructure.com>
  * Pierre Carlson <mpcarl@us.ibm.com>
  * Ernest Borowski <er.borowski@gmail.com>
  * Remus Bunduc <remus.bunduc@gmail.com>
  * Iakov Davydov <iakov.davydov@unil.ch> <dav05.gith@myths.ru>
  * Jakub Tasiemski <tasiemski@gmail.com>
  * David Minor <dminor@saymedia.com>
  * Tim Cooijmans <cooijmans.tim@gmail.com>
  * Laurence <liuxy6@gmail.com>
  * Giovanni Pizzi <gio.piz@gmail.com>
  * Filip Bartodziej <filipbartodziej@gmail.com>
  * Jon Fautley <jon@dead.li>
  * lewapm <32110057+lewapm@users.noreply.github.com>
  * Yassine Imounachen <yassine256@gmail.com>
  * Chris Redekop <chris-redekop@users.noreply.github.com> <chris.redekop@gmail.com>
  * Jon Fautley <jon@adenoid.appstal.co.uk>
  * Will Gunn <WillGunn@users.noreply.github.com>
  * Lucas Bremgartner <lucas@bremis.ch>
  * Jody Frankowski <jody.frankowski@gmail.com>
  * Andreas Roussos <arouss1980@gmail.com>
  * nbuchanan <nbuchanan@utah.gov>
  * Durval Menezes <rclone@durval.com>
  * Victor <vb-github@viblo.se>
  * Mateusz <pabian.mateusz@gmail.com>
  * Daniel Loader <spicypixel@gmail.com>
  * David0rk <davidork@gmail.com>
  * Alexander Neumann <alexander@bumpern.de>
  * Giri Badanahatti <gbadanahatti@us.ibm.com@Giris-MacBook-Pro.local>
  * Leo R. Lundgren <leo@finalresort.org>
  * wolfv <wolfv6@users.noreply.github.com>
  * Dave Pedu <dave@davepedu.com>
  * Stefan Lindblom <lindblom@spotify.com>
  * seuffert <oliver@seuffert.biz>
  * gbadanahatti <37121690+gbadanahatti@users.noreply.github.com>
  * Keith Goldfarb <barkofdelight@gmail.com>
  * Steve Kriss <steve@heptio.com>
  * Chih-Hsuan Yen <yan12125@gmail.com>
  * Alexander Neumann <fd0@users.noreply.github.com>
  * Matt Holt <mholt@users.noreply.github.com>
  * Eri Bastos <bastos.eri@gmail.com>
  * Michael P. Dubner <pywebmail@list.ru>
  * Antoine GIRARD <sapk@users.noreply.github.com>
  * Mateusz Piotrowski <mpp302@gmail.com>
  * Animosity022 <animosity22@users.noreply.github.com> <earl.texter@gmail.com>
  * Peter Baumgartner <pete@lincolnloop.com>
  * Craig Rachel <craig@craigrachel.com>
  * Michael G. Noll <miguno@users.noreply.github.com>
  * hensur <me@hensur.de>
  * Oliver Heyme <de8olihe@lego.com>
  * Richard Yang <richard@yenforyang.com>
  * Piotr Oleszczyk <piotr.oleszczyk@gmail.com>
  * Rodrigo <rodarima@gmail.com>
  * NoLooseEnds <NoLooseEnds@users.noreply.github.com>
  * Jakub Karlicek <jakub@karlicek.me>
  * John Clayton <john@codemonkeylabs.com>
  * Kasper Byrdal Nielsen <byrdal76@gmail.com>
  * Benjamin Joseph Dag <bjdag1234@users.noreply.github.com>
  * themylogin <themylogin@gmail.com>
  * Onno Zweers <onno.zweers@surfsara.nl>
  * Jasper Lievisse Adriaanse <jasper@humppa.nl>
  * sandeepkru <sandeep.ummadi@gmail.com> <sandeepkru@users.noreply.github.com>
  * HerrH <atomtigerzoo@users.noreply.github.com>
  * Andrew <4030760+sparkyman215@users.noreply.github.com>
  * dan smith <XX1011@gmail.com>
  * Oleg Kovalov <iamolegkovalov@gmail.com>
  * Ruben Vandamme <github-com-00ff86@vandamme.email>
  * Cnly <minecnly@gmail.com>
  * Andres Alvarez <1671935+kir4h@users.noreply.github.com>
  * reddi1 <xreddi@gmail.com>
  * Matt Tucker <matthewtckr@gmail.com>
  * Sebastian Bünger <buengese@gmail.com>
  * Martin Polden <mpolden@mpolden.no>
  * Alex Chen <Cnly@users.noreply.github.com>
  * Denis <deniskovpen@gmail.com>
  * bsteiss <35940619+bsteiss@users.noreply.github.com>
  * Cédric Connes <cedric.connes@gmail.com>
  * Dr. Tobias Quathamer <toddy15@users.noreply.github.com>
  * dcpu <42736967+dcpu@users.noreply.github.com>
  * Sheldon Rupp <me@shel.io>
  * albertony <12441419+albertony@users.noreply.github.com>
  * cron410 <cron410@gmail.com>
  * Anagh Kumar Baranwal <6824881+darthShadow@users.noreply.github.com>
  * Felix Brucker <felix@felixbrucker.com>
  * Santiago Rodríguez <scollazo@users.noreply.github.com>
  * Craig Miskell <craig.miskell@fluxfederation.com>
  * Antoine GIRARD <sapk@sapk.fr>
  * Joanna Marek <joanna.marek@u2i.com>
  * frenos <frenos@users.noreply.github.com>
  * ssaqua <ssaqua@users.noreply.github.com>
  * xnaas <me@xnaas.info>
  * Frantisek Fuka <fuka@fuxoft.cz>
  * Paul Kohout <pauljkohout@yahoo.com>
  * dcpu <43330287+dcpu@users.noreply.github.com>
  * jackyzy823 <jackyzy823@gmail.com>
  * David Haguenauer <ml@kurokatta.org>
  * teresy <hi.teresy@gmail.com>
  * buergi <patbuergi@gmx.de>
  * Florian Gamboeck <mail@floga.de>
  * Ralf Hemberger <10364191+rhemberger@users.noreply.github.com>
  * Scott Edlund <sedlund@users.noreply.github.com>
  * Erik Swanson <erik@retailnext.net>
  * Jake Coggiano <jake@stripe.com>
  * brused27 <brused27@noemailaddress>
  * Peter Kaminski <kaminski@istori.com>
  * Henry Ptasinski <henry@logout.com>
  * Alexander <kharkovalexander@gmail.com>
  * Garry McNulty <garrmcnu@gmail.com>
  * Mathieu Carbou <mathieu.carbou@gmail.com>
  * Mark Otway <mark@otway.com>
  * William Cocker <37018962+WilliamCocker@users.noreply.github.com>
  * François Leurent <131.js@cloudyks.org>
  * Arkadius Stefanski <arkste@gmail.com>
  * Jay <dev@jaygoel.com>
  * andrea rota <a@xelera.eu>
  * nicolov <nicolov@users.noreply.github.com>
  * Dario Guzik <dario@guzik.com.ar>
  * qip <qip@users.noreply.github.com>
  * yair@unicorn <yair@unicorn>
  * Matt Robinson <brimstone@the.narro.ws>
  * kayrus <kay.diam@gmail.com>
  * Rémy Léone <remy.leone@gmail.com>
  * Wojciech Smigielski <wojciech.hieronim.smigielski@gmail.com>
  * weetmuts <oehrstroem@gmail.com>
  * Jonathan <vanillajonathan@users.noreply.github.com>
  * James Carpenter <orbsmiv@users.noreply.github.com>
  * Vince <vince0villamora@gmail.com>
  * Nestar47 <47841759+Nestar47@users.noreply.github.com>
  * Six <brbsix@gmail.com>
  * Alexandru Bumbacea <alexandru.bumbacea@booking.com>
  * calisro <robert.calistri@gmail.com>
  * Dr.Rx <david.rey@nventive.com>
  * marcintustin <marcintustin@users.noreply.github.com>
  * jaKa Močnik <jaka@koofr.net>
  * Fionera <fionera@fionera.de>
  * Dan Walters <dan@walters.io>
  * Danil Semelenov <sgtpep@users.noreply.github.com>
  * xopez <28950736+xopez@users.noreply.github.com>
  * Ben Boeckel <mathstuf@gmail.com>
  * Manu <manu@snapdragon.cc>
  * Kyle E. Mitchell <kyle@kemitchell.com>
  * Gary Kim <gary@garykim.dev>
  * Jon <jonathn@github.com>
  * Jeff Quinn <jeffrey.quinn@bluevoyant.com>
  * Peter Berbec <peter@berbec.com>
  * didil <1284255+didil@users.noreply.github.com>
  * id01 <gaviniboom@gmail.com>
  * Robert Marko <robimarko@gmail.com>
  * Philip Harvey <32467456+pharveybattelle@users.noreply.github.com>
  * JorisE <JorisE@users.noreply.github.com>
  * garry415 <garry.415@gmail.com>
  * forgems <forgems@gmail.com>
  * Florian Apolloner <florian@apolloner.eu>
  * Aleksandar Janković <office@ajankovic.com> <ajankovic@users.noreply.github.com>
  * Maran <maran@protonmail.com>
  * nguyenhuuluan434 <nguyenhuuluan434@gmail.com>
  * Laura Hausmann <zotan@zotan.pw> <laura@hausmann.dev>
  * yparitcher <y@paritcher.com>
  * AbelThar <abela.tharen@gmail.com>
  * Matti Niemenmaa <matti.niemenmaa+git@iki.fi>
  * Russell Davis <russelldavis@users.noreply.github.com>
  * Yi FU <yi.fu@tink.se>
  * Paul Millar <paul.millar@desy.de>
  * justinalin <justinalin@qnap.com>
  * EliEron <subanimehd@gmail.com>
  * justina777 <chiahuei.lin@gmail.com>
  * Chaitanya Bankanhal <bchaitanya15@gmail.com>
  * Michał Matczuk <michal@scylladb.com>
  * Macavirus <macavirus@zoho.com>
  * Abhinav Sharma <abhi18av@users.noreply.github.com>
  * ginvine <34869051+ginvine@users.noreply.github.com>
  * Patrick Wang <mail6543210@yahoo.com.tw>
  * Cenk Alti <cenkalti@gmail.com>
  * Andreas Chlupka <andy@chlupka.com>
  * Alfonso Montero <amontero@tinet.org>
  * Ivan Andreev <ivandeex@gmail.com>
  * David Baumgold <david@davidbaumgold.com>
  * Lars Lehtonen <lars.lehtonen@gmail.com>
  * Matei David <matei.david@gmail.com>
  * David <david.bramwell@endemolshine.com>
  * Anthony Rusdi <33247310+antrusd@users.noreply.github.com>
  * Richard Patel <me@terorie.dev>
  * 庄天翼 <zty0826@gmail.com>
  * SwitchJS <dev@switchjs.com>
  * Raphael <PowershellNinja@users.noreply.github.com>
  * Sezal Agrawal <sezalagrawal@gmail.com>
  * Tyler <TylerNakamura@users.noreply.github.com>
  * Brett Dutro <brett.dutro@gmail.com>
  * Vighnesh SK <booterror99@gmail.com>
  * Arijit Biswas <dibbyo456@gmail.com>
  * Michele Caci <michele.caci@gmail.com>
  * AlexandrBoltris <ua2fgb@gmail.com>
  * Bryce Larson <blarson@saltstack.com>
  * Carlos Ferreyra <crypticmind@gmail.com>
  * Saksham Khanna <sakshamkhanna@outlook.com>
  * dausruddin <5763466+dausruddin@users.noreply.github.com>
  * zero-24 <zero-24@users.noreply.github.com>
  * Xiaoxing Ye <ye@xiaoxing.us>
  * Barry Muldrey <barry@muldrey.net>
  * Sebastian Brandt <sebastian.brandt@friday.de>
  * Marco Molteni <marco.molteni@mailbox.org>
  * Ankur Gupta <ankur0493@gmail.com> <7876747+ankur0493@users.noreply.github.com>
  * Maciej Zimnoch <maciej@scylladb.com>
  * anuar45 <serdaliyev.anuar@gmail.com>
  * Fernando <ferferga@users.noreply.github.com>
  * David Cole <david.cole@sohonet.com>
  * Wei He <git@weispot.com>
  * Outvi V <19144373+outloudvi@users.noreply.github.com>
  * Thomas Kriechbaumer <thomas@kriechbaumer.name>
  * Tennix <tennix@users.noreply.github.com>
  * Ole Schütt <ole@schuett.name>
  * Kuang-che Wu <kcwu@csie.org>
  * Thomas Eales <wingsuit@users.noreply.github.com>
  * Paul Tinsley <paul.tinsley@vitalsource.com>
  * Felix Hungenberg <git@shiftgeist.com>
  * Benjamin Richter <github@dev.telepath.de>
  * landall <cst_zf@qq.com>
  * thestigma <thestigma@gmail.com>
  * jtagcat <38327267+jtagcat@users.noreply.github.com>
  * Damon Permezel <permezel@me.com>
  * boosh <boosh@users.noreply.github.com>
  * unbelauscht <58393353+unbelauscht@users.noreply.github.com>
  * Motonori IWAMURO <vmi@nifty.com>
  * Benjapol Worakan <benwrk@live.com>
  * Dave Koston <dave.koston@stackpath.com>
  * Durval Menezes <DurvalMenezes@users.noreply.github.com>
  * Tim Gallant <me@timgallant.us>
  * Frederick Zhang <frederick888@tsundere.moe>
  * valery1707 <valery1707@gmail.com>
  * Yves G <theYinYeti@yalis.fr>
  * Shing Kit Chan <chanshingkit@gmail.com>
  * Franklyn Tackitt <franklyn@tackitt.net>
  * Robert-André Mauchin <zebob.m@gmail.com>
  * evileye <48332831+ibiruai@users.noreply.github.com>
  * Joachim Brandon LeBlanc <brandon@leblanc.codes>
  * Patryk Jakuszew <patryk.jakuszew@gmail.com>
  * fishbullet <shindu666@gmail.com>
  * greatroar <@>
  * Bernd Schoolmann <mail@quexten.com>
  * Elan Ruusamäe <glen@pld-linux.org>
  * Max Sum <max@lolyculture.com>
  * Mark Spieth <mspieth@users.noreply.github.com>
  * harry <me@harry.plus>
  * Samantha McVey <samantham@posteo.net>
  * Jack Anderson <jack.anderson@metaswitch.com>
  * Michael G <draget@speciesm.net>
  * Brandon Philips <brandon@ifup.org>
  * Daven <dooven@users.noreply.github.com>
  * Martin Stone <martin@d7415.co.uk>
  * David Bramwell <13053834+dbramwell@users.noreply.github.com>
  * Sunil Patra <snl_su@live.com>
  * Adam Stroud <adam.stroud@gmail.com>
  * Kush <kushsharma@users.noreply.github.com>
  * Matan Rosenberg <matan129@gmail.com>
  * gitch1 <63495046+gitch1@users.noreply.github.com>
  * ElonH <elonhhuang@gmail.com>
  * Fred <fred@creativeprojects.tech>
  * Sébastien Gross <renard@users.noreply.github.com>
  * Maxime Suret <11944422+msuret@users.noreply.github.com>
  * Caleb Case <caleb@storj.io>
  * Ben Zenker <imbenzenker@gmail.com>
  * Martin Michlmayr <tbm@cyrius.com>
  * Brandon McNama <bmcnama@pagerduty.com>
  * Daniel Slyman <github@skylayer.eu>

# Contact the rclone project #

## Forum ##

Forum for questions and general discussion:

  * https://forum.rclone.org

## GitHub repository ##

The project's repository is located at:

  * https://github.com/rclone/rclone

There you can file bug reports or contribute with pull requests.

## Twitter ##

You can also follow me on twitter for rclone announcements:

  * [@njcw](https://twitter.com/njcw)

## Email ##

Or if all else fails or you want to ask something private or
confidential email [Nick Craig-Wood](mailto:nick@craig-wood.com).
Please don't email me requests for help - those are better directed to
the forum. Thanks!

