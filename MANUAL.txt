rclone(1) User Manual
Nick Craig-Wood
Jun 15, 2019



RCLONE


[Logo]

Rclone is a command line program to sync files and directories to and
from:

-   Alibaba Cloud (Aliyun) Object Storage System (OSS)
-   Amazon Drive (See note)
-   Amazon S3
-   Backblaze B2
-   Box
-   Ceph
-   DigitalOcean Spaces
-   Dreamhost
-   Dropbox
-   FTP
-   Google Cloud Storage
-   Google Drive
-   HTTP
-   Hubic
-   Jottacloud
-   IBM COS S3
-   Koofr
-   Memset Memstore
-   Mega
-   Microsoft Azure Blob Storage
-   Microsoft OneDrive
-   Minio
-   Nextcloud
-   OVH
-   OpenDrive
-   Openstack Swift
-   Oracle Cloud Storage
-   ownCloud
-   pCloud
-   put.io
-   QingStor
-   Rackspace Cloud Files
-   rsync.net
-   Scaleway
-   SFTP
-   Wasabi
-   WebDAV
-   Yandex Disk
-   The local filesystem

Features

-   MD5/SHA1 hashes checked at all times for file integrity
-   Timestamps preserved on files
-   Partial syncs supported on a whole file basis
-   Copy mode to just copy new/changed files
-   Sync (one way) mode to make a directory identical
-   Check mode to check for file hash equality
-   Can sync to and from network, eg two different cloud accounts
-   Encryption backend
-   Cache backend
-   Union backend
-   Optional FUSE mount (rclone mount)
-   Multi-threaded downloads to local disk
-   Can serve local or remote files over HTTP/WebDav/FTP/SFTP/dlna

Links

-   Home page
-   GitHub project page for source and bug tracker
-   Rclone Forum
-   Downloads



INSTALL


Rclone is a Go program and comes as a single binary file.


Quickstart

-   Download the relevant binary.
-   Extract the rclone or rclone.exe binary from the archive
-   Run rclone config to setup. See rclone config docs for more details.

See below for some expanded Linux / macOS instructions.

See the Usage section of the docs for how to use rclone, or run
rclone -h.


Script installation

To install rclone on Linux/macOS/BSD systems, run:

    curl https://rclone.org/install.sh | sudo bash

For beta installation, run:

    curl https://rclone.org/install.sh | sudo bash -s beta

Note that this script checks the version of rclone installed first and
won’t re-download if not needed.


Linux installation from precompiled binary

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

Run rclone config to setup. See rclone config docs for more details.

    rclone config


macOS installation from precompiled binary

Download the latest version of rclone.

    cd && curl -O https://downloads.rclone.org/rclone-current-osx-amd64.zip

Unzip the download and cd to the extracted folder.

    unzip -a rclone-current-osx-amd64.zip && cd rclone-*-osx-amd64

Move rclone to your $PATH. You will be prompted for your password.

    sudo mkdir -p /usr/local/bin
    sudo mv rclone /usr/local/bin/

(the mkdir command is safe to run, even if the directory already
exists).

Remove the leftover files.

    cd .. && rm -rf rclone-*-osx-amd64 rclone-current-osx-amd64.zip

Run rclone config to setup. See rclone config docs for more details.

    rclone config


Install from source

Make sure you have at least Go 1.7 installed. Download go if necessary.
The latest release is recommended. Then

    git clone https://github.com/ncw/rclone.git
    cd rclone
    go build
    ./rclone version

You can also build and install rclone in the GOPATH (which defaults to
~/go) with:

    go get -u -v github.com/ncw/rclone

and this will build the binary in $GOPATH/bin (~/go/bin/rclone by
default) after downloading the source to
$GOPATH/src/github.com/ncw/rclone (~/go/src/github.com/ncw/rclone by
default).


Installation with Ansible

This can be done with Stefan Weichinger’s ansible role.

Instructions

1.  git clone https://github.com/stefangweichinger/ansible-rclone.git
    into your local roles-directory
2.  add the role to the hosts you want rclone installed to:

        - hosts: rclone-hosts
          roles:
              - rclone


Configure

First, you’ll need to configure rclone. As the object storage systems
have quite complicated authentication these are kept in a config file.
(See the --config entry for how to find the config file and choose its
location.)

The easiest way to make the config is to run rclone with the config
option:

    rclone config

See the following for detailed instructions for

-   Alias
-   Amazon Drive
-   Amazon S3
-   Backblaze B2
-   Box
-   Cache
-   Crypt - to encrypt other remotes
-   DigitalOcean Spaces
-   Dropbox
-   FTP
-   Google Cloud Storage
-   Google Drive
-   HTTP
-   Hubic
-   Jottacloud
-   Koofr
-   Mega
-   Microsoft Azure Blob Storage
-   Microsoft OneDrive
-   Openstack Swift / Rackspace Cloudfiles / Memset Memstore
-   OpenDrive
-   Pcloud
-   QingStor
-   SFTP
-   Union
-   WebDAV
-   Yandex Disk
-   The local filesystem


Usage

Rclone syncs a directory tree from one storage system to another.

Its syntax is like this

    Syntax: [options] subcommand <parameters> <parameters...>

Source and destination paths are specified by the name you gave the
storage system in the config file then the sub path, eg “drive:myfolder”
to look at “myfolder” in Google drive.

You can define as many storage paths as you like in the config file.


Subcommands

rclone uses a system of subcommands. For example

    rclone ls remote:path # lists a remote
    rclone copy /local/path remote:path # copies /local/path to the remote
    rclone sync /local/path remote:path # syncs /local/path to the remote


rclone config

Enter an interactive configuration session.

Synopsis

Enter an interactive configuration session where you can setup new
remotes and manage existing ones. You may also set or remove a password
to protect your configuration.

    rclone config [flags]

Options

      -h, --help   help for config

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.
-   rclone config create - Create a new remote with name, type and
    options.
-   rclone config delete - Delete an existing remote .
-   rclone config dump - Dump the config file as JSON.
-   rclone config edit - Enter an interactive configuration session.
-   rclone config file - Show path of configuration file in use.
-   rclone config password - Update password in an existing remote.
-   rclone config providers - List in JSON format all the providers and
    options.
-   rclone config show - Print (decrypted) config file, or the config
    for a single remote.
-   rclone config update - Update options in an existing remote.

Auto generated by spf13/cobra on 15-Jun-2019


rclone copy

Copy files from source to dest, skipping already copied

Synopsis

Copy the source to the destination. Doesn’t transfer unchanged files,
testing by size and modification time or MD5SUM. Doesn’t delete files
from the destination.

Note that it is always the contents of the directory that is synced, not
the directory so when source:path is a directory, it’s the contents of
source:path that are copied, not the directory name and contents.

If dest:path doesn’t exist, it is created and the source:path contents
go there.

For example

    rclone copy source:sourcepath dest:destpath

Let’s say there are two files in sourcepath

    sourcepath/one.txt
    sourcepath/two.txt

This copies them to

    destpath/one.txt
    destpath/two.txt

Not to

    destpath/sourcepath/one.txt
    destpath/sourcepath/two.txt

If you are familiar with rsync, rclone always works as if you had
written a trailing / - meaning “copy the contents of this directory”.
This applies to all commands and whether you are talking about the
source or destination.

See the –no-traverse option for controlling whether rclone lists the
destination directory or not. Supplying this option when copying a small
number of files into a large destination can speed transfers up greatly.

For example, if you have many files in /path/to/src but only a few of
them change every day, you can to copy all the files which have changed
recently very efficiently like this:

    rclone copy --max-age 24h --no-traverse /path/to/src remote:

NOTE: Use the -P/--progress flag to view real-time transfer statistics

    rclone copy source:path dest:path [flags]

Options

          --create-empty-src-dirs   Create empty source dirs on destination after copy
      -h, --help                    help for copy

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone sync

Make source and dest identical, modifying destination only.

Synopsis

Sync the source to the destination, changing the destination only.
Doesn’t transfer unchanged files, testing by size and modification time
or MD5SUM. Destination is updated to match source, including deleting
files if necessary.

IMPORTANT: Since this can cause data loss, test first with the --dry-run
flag to see exactly what would be copied and deleted.

Note that files in the destination won’t be deleted if there were any
errors at any point.

It is always the contents of the directory that is synced, not the
directory so when source:path is a directory, it’s the contents of
source:path that are copied, not the directory name and contents. See
extended explanation in the copy command above if unsure.

If dest:path doesn’t exist, it is created and the source:path contents
go there.

NOTE: Use the -P/--progress flag to view real-time transfer statistics

    rclone sync source:path dest:path [flags]

Options

          --create-empty-src-dirs   Create empty source dirs on destination after sync
      -h, --help                    help for sync

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone move

Move files from source to dest.

Synopsis

Moves the contents of the source directory to the destination directory.
Rclone will error if the source and destination overlap and the remote
does not support a server side directory move operation.

If no filters are in use and if possible this will server side move
source:path into dest:path. After this source:path will no longer longer
exist.

Otherwise for each file in source:path selected by the filters (if any)
this will move it into dest:path. If possible a server side move will be
used, otherwise it will copy it (server side if possible) into dest:path
then delete the original (if no errors on copy) in source:path.

If you want to delete empty source directories after move, use the
–delete-empty-src-dirs flag.

See the –no-traverse option for controlling whether rclone lists the
destination directory or not. Supplying this option when moving a small
number of files into a large destination can speed transfers up greatly.

IMPORTANT: Since this can cause data loss, test first with the –dry-run
flag.

NOTE: Use the -P/--progress flag to view real-time transfer statistics.

    rclone move source:path dest:path [flags]

Options

          --create-empty-src-dirs   Create empty source dirs on destination after move
          --delete-empty-src-dirs   Delete empty source dirs after move
      -h, --help                    help for move

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone delete

Remove the contents of path.

Synopsis

Remove the files in path. Unlike purge it obeys include/exclude filters
so can be used to selectively delete files.

rclone delete only deletes objects but leaves the directory structure
alone. If you want to delete a directory and all of its contents use
rclone purge

Eg delete all files bigger than 100MBytes

Check what would be deleted first (use either)

    rclone --min-size 100M lsl remote:path
    rclone --dry-run --min-size 100M delete remote:path

Then delete

    rclone --min-size 100M delete remote:path

That reads “delete everything with a minimum size of 100 MB”, hence
delete all files bigger than 100MBytes.

    rclone delete remote:path [flags]

Options

      -h, --help   help for delete

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone purge

Remove the path and all of its contents.

Synopsis

Remove the path and all of its contents. Note that this does not obey
include/exclude filters - everything will be removed. Use delete if you
want to selectively delete files.

    rclone purge remote:path [flags]

Options

      -h, --help   help for purge

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone mkdir

Make the path if it doesn’t already exist.

Synopsis

Make the path if it doesn’t already exist.

    rclone mkdir remote:path [flags]

Options

      -h, --help   help for mkdir

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone rmdir

Remove the path if empty.

Synopsis

Remove the path. Note that you can’t remove a path with objects in it,
use purge for that.

    rclone rmdir remote:path [flags]

Options

      -h, --help   help for rmdir

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone check

Checks the files in the source and destination match.

Synopsis

Checks the files in the source and destination match. It compares sizes
and hashes (MD5 or SHA1) and logs a report of files which don’t match.
It doesn’t alter the source or destination.

If you supply the –size-only flag, it will only compare the sizes not
the hashes as well. Use this for a quick check.

If you supply the –download flag, it will download the data from both
remotes and check them against each other on the fly. This can be useful
for remotes that don’t support hashes or if you really want to check all
the data.

If you supply the –one-way flag, it will only check that files in source
match the files in destination, not the other way around. Meaning extra
files in destination that are not in the source will not trigger an
error.

    rclone check source:path dest:path [flags]

Options

          --download   Check by downloading rather than with hash.
      -h, --help       help for check
          --one-way    Check one way only, source files must exist on remote

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone ls

List the objects in the path with size and path.

Synopsis

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

-   ls to list size and path of objects only
-   lsl to list modification time, size and path of objects only
-   lsd to list directories only
-   lsf to list objects and directories in easy to parse format
-   lsjson to list objects and directories in JSON format

ls,lsl,lsd are designed to be human readable. lsf is designed to be
human and machine readable. lsjson is designed to be machine readable.

Note that ls and lsl recurse by default - use “–max-depth 1” to stop the
recursion.

The other list commands lsd,lsf,lsjson do not recurse by default - use
“-R” to make them recurse.

Listing a non existent directory will produce an error except for
remotes which can’t have empty directories (eg s3, swift, gcs, etc - the
bucket based remotes).

    rclone ls remote:path [flags]

Options

      -h, --help   help for ls

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone lsd

List all directories/containers/buckets in the path.

Synopsis

Lists the directories in the source path to standard output. Does not
recurse by default. Use the -R flag to recurse.

This command lists the total size of the directory (if known, -1 if
not), the modification time (if known, the current time if not), the
number of objects in the directory (if known, -1 if not) and the name of
the directory, Eg

    $ rclone lsd swift:
          494000 2018-04-26 08:43:20     10000 10000files
              65 2018-04-26 08:43:20         1 1File

Or

    $ rclone lsd drive:test
              -1 2016-10-17 17:41:53        -1 1000files
              -1 2017-01-03 14:40:54        -1 2500files
              -1 2017-07-08 14:39:28        -1 4000files

If you just want the directory names use “rclone lsf –dirs-only”.

Any of the filtering options can be applied to this command.

There are several related list commands

-   ls to list size and path of objects only
-   lsl to list modification time, size and path of objects only
-   lsd to list directories only
-   lsf to list objects and directories in easy to parse format
-   lsjson to list objects and directories in JSON format

ls,lsl,lsd are designed to be human readable. lsf is designed to be
human and machine readable. lsjson is designed to be machine readable.

Note that ls and lsl recurse by default - use “–max-depth 1” to stop the
recursion.

The other list commands lsd,lsf,lsjson do not recurse by default - use
“-R” to make them recurse.

Listing a non existent directory will produce an error except for
remotes which can’t have empty directories (eg s3, swift, gcs, etc - the
bucket based remotes).

    rclone lsd remote:path [flags]

Options

      -h, --help        help for lsd
      -R, --recursive   Recurse into the listing.

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone lsl

List the objects in path with modification time, size and path.

Synopsis

Lists the objects in the source path to standard output in a human
readable format with modification time, size and path. Recurses by
default.

Eg

    $ rclone lsl swift:bucket
        60295 2016-06-25 18:55:41.062626927 bevajer5jef
        90613 2016-06-25 18:55:43.302607074 canole
        94467 2016-06-25 18:55:43.046609333 diwogej7
        37600 2016-06-25 18:55:40.814629136 fubuwic

Any of the filtering options can be applied to this command.

There are several related list commands

-   ls to list size and path of objects only
-   lsl to list modification time, size and path of objects only
-   lsd to list directories only
-   lsf to list objects and directories in easy to parse format
-   lsjson to list objects and directories in JSON format

ls,lsl,lsd are designed to be human readable. lsf is designed to be
human and machine readable. lsjson is designed to be machine readable.

Note that ls and lsl recurse by default - use “–max-depth 1” to stop the
recursion.

The other list commands lsd,lsf,lsjson do not recurse by default - use
“-R” to make them recurse.

Listing a non existent directory will produce an error except for
remotes which can’t have empty directories (eg s3, swift, gcs, etc - the
bucket based remotes).

    rclone lsl remote:path [flags]

Options

      -h, --help   help for lsl

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone md5sum

Produces an md5sum file for all the objects in the path.

Synopsis

Produces an md5sum file for all the objects in the path. This is in the
same format as the standard md5sum tool produces.

    rclone md5sum remote:path [flags]

Options

      -h, --help   help for md5sum

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone sha1sum

Produces an sha1sum file for all the objects in the path.

Synopsis

Produces an sha1sum file for all the objects in the path. This is in the
same format as the standard sha1sum tool produces.

    rclone sha1sum remote:path [flags]

Options

      -h, --help   help for sha1sum

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone size

Prints the total size and number of objects in remote:path.

Synopsis

Prints the total size and number of objects in remote:path.

    rclone size remote:path [flags]

Options

      -h, --help   help for size
          --json   format output as JSON

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone version

Show the version number.

Synopsis

Show the version number, the go version and the architecture.

Eg

    $ rclone version
    rclone v1.41
    - os/arch: linux/amd64
    - go version: go1.10

If you supply the –check flag, then it will do an online check to
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

    rclone version [flags]

Options

          --check   Check for new version.
      -h, --help    help for version

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone cleanup

Clean up the remote if possible

Synopsis

Clean up the remote if possible. Empty the trash or delete old file
versions. Not supported by all remotes.

    rclone cleanup remote:path [flags]

Options

      -h, --help   help for cleanup

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone dedupe

Interactively find duplicate files and delete/rename them.

Synopsis

By default dedupe interactively finds duplicate files and offers to
delete all but one or rename them to be different. Only useful with
Google Drive which can have duplicate file names.

In the first pass it will merge directories with the same name. It will
do this iteratively until all the identical directories have been
merged.

The dedupe command will delete all but one of any identical (same
md5sum) files it finds without confirmation. This means that for most
duplicated files the dedupe command will not be interactive. You can use
--dry-run to see what would happen without doing anything.

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

Now the dedupe session

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

Dedupe can be run non interactively using the --dedupe-mode flag or by
using an extra parameter with the same value

-   --dedupe-mode interactive - interactive as above.
-   --dedupe-mode skip - removes identical files then skips anything
    left.
-   --dedupe-mode first - removes identical files then keeps the first
    one.
-   --dedupe-mode newest - removes identical files then keeps the newest
    one.
-   --dedupe-mode oldest - removes identical files then keeps the oldest
    one.
-   --dedupe-mode largest - removes identical files then keeps the
    largest one.
-   --dedupe-mode rename - removes identical files then renames the rest
    to be different.

For example to rename all the identically named photos in your Google
Photos directory, do

    rclone dedupe --dedupe-mode rename "drive:Google Photos"

Or

    rclone dedupe rename "drive:Google Photos"

    rclone dedupe [mode] remote:path [flags]

Options

          --dedupe-mode string   Dedupe mode interactive|skip|first|newest|oldest|rename. (default "interactive")
      -h, --help                 help for dedupe

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone about

Get quota information from the remote.

Synopsis

Get quota information from the remote, like bytes used/free/quota and
bytes used in the trash. Not supported by all remotes.

This will print to stdout something like this:

    Total:   17G
    Used:    7.444G
    Free:    1.315G
    Trashed: 100.000M
    Other:   8.241G

Where the fields are:

-   Total: total size available.
-   Used: total size used
-   Free: total amount this user could upload.
-   Trashed: total amount in the trash
-   Other: total amount in other storage (eg Gmail, Google Photos)
-   Objects: total number of objects in the storage

Note that not all the backends provide all the fields - they will be
missing if they are not known for that backend. Where it is known that
the value is unlimited the value will also be omitted.

Use the –full flag to see the numbers written out in full, eg

    Total:   18253611008
    Used:    7993453766
    Free:    1411001220
    Trashed: 104857602
    Other:   8849156022

Use the –json flag for a computer readable output, eg

    {
        "total": 18253611008,
        "used": 7993453766,
        "trashed": 104857602,
        "other": 8849156022,
        "free": 1411001220
    }

    rclone about remote: [flags]

Options

          --full   Full numbers instead of SI units
      -h, --help   help for about
          --json   Format output as JSON

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone authorize

Remote authorization.

Synopsis

Remote authorization. Used to authorize a remote or headless rclone from
a machine with a browser - use as instructed by rclone config.

    rclone authorize [flags]

Options

      -h, --help   help for authorize

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone cachestats

Print cache stats for a remote

Synopsis

Print cache stats for a remote in JSON format

    rclone cachestats source: [flags]

Options

      -h, --help   help for cachestats

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone cat

Concatenates any files and sends them to stdout.

Synopsis

rclone cat sends any files to standard output.

You can use it like this to output a single file

    rclone cat remote:path/to/file

Or like this to output any file in dir or subdirectories.

    rclone cat remote:path/to/dir

Or like this to output any .txt files in dir or subdirectories.

    rclone --include "*.txt" cat remote:path/to/dir

Use the –head flag to print characters only at the start, –tail for the
end and –offset and –count to print a section in the middle. Note that
if offset is negative it will count from the end, so –offset -1 –count 1
is equivalent to –tail 1.

    rclone cat remote:path [flags]

Options

          --count int    Only print N characters. (default -1)
          --discard      Discard the output instead of printing.
          --head int     Only print the first N characters.
      -h, --help         help for cat
          --offset int   Start printing at offset N (or from end if -ve).
          --tail int     Only print the last N characters.

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone config create

Create a new remote with name, type and options.

Synopsis

Create a new remote of with and options. The options should be passed in
in pairs of .

For example to make a swift remote of name myremote using auto config
you would do:

    rclone config create myremote swift env_auth true

Note that if the config process would normally ask a question the
default is taken. Each time that happens rclone will print a message
saying how to affect the value taken.

If any of the parameters passed is a password field, then rclone will
automatically obscure them before putting them in the config file.

So for example if you wanted to configure a Google Drive remote but
using remote authorization you would do this:

    rclone config create mydrive drive config_is_local false

    rclone config create <name> <type> [<key> <value>]* [flags]

Options

      -h, --help   help for create

SEE ALSO

-   rclone config - Enter an interactive configuration session.

Auto generated by spf13/cobra on 15-Jun-2019


rclone config delete

Delete an existing remote .

Synopsis

Delete an existing remote .

    rclone config delete <name> [flags]

Options

      -h, --help   help for delete

SEE ALSO

-   rclone config - Enter an interactive configuration session.

Auto generated by spf13/cobra on 15-Jun-2019


rclone config dump

Dump the config file as JSON.

Synopsis

Dump the config file as JSON.

    rclone config dump [flags]

Options

      -h, --help   help for dump

SEE ALSO

-   rclone config - Enter an interactive configuration session.

Auto generated by spf13/cobra on 15-Jun-2019


rclone config edit

Enter an interactive configuration session.

Synopsis

Enter an interactive configuration session where you can setup new
remotes and manage existing ones. You may also set or remove a password
to protect your configuration.

    rclone config edit [flags]

Options

      -h, --help   help for edit

SEE ALSO

-   rclone config - Enter an interactive configuration session.

Auto generated by spf13/cobra on 15-Jun-2019


rclone config file

Show path of configuration file in use.

Synopsis

Show path of configuration file in use.

    rclone config file [flags]

Options

      -h, --help   help for file

SEE ALSO

-   rclone config - Enter an interactive configuration session.

Auto generated by spf13/cobra on 15-Jun-2019


rclone config password

Update password in an existing remote.

Synopsis

Update an existing remote’s password. The password should be passed in
in pairs of .

For example to set password of a remote of name myremote you would do:

    rclone config password myremote fieldname mypassword

This command is obsolete now that “config update” and “config create”
both support obscuring passwords directly.

    rclone config password <name> [<key> <value>]+ [flags]

Options

      -h, --help   help for password

SEE ALSO

-   rclone config - Enter an interactive configuration session.

Auto generated by spf13/cobra on 15-Jun-2019


rclone config providers

List in JSON format all the providers and options.

Synopsis

List in JSON format all the providers and options.

    rclone config providers [flags]

Options

      -h, --help   help for providers

SEE ALSO

-   rclone config - Enter an interactive configuration session.

Auto generated by spf13/cobra on 15-Jun-2019


rclone config show

Print (decrypted) config file, or the config for a single remote.

Synopsis

Print (decrypted) config file, or the config for a single remote.

    rclone config show [<remote>] [flags]

Options

      -h, --help   help for show

SEE ALSO

-   rclone config - Enter an interactive configuration session.

Auto generated by spf13/cobra on 15-Jun-2019


rclone config update

Update options in an existing remote.

Synopsis

Update an existing remote’s options. The options should be passed in in
pairs of .

For example to update the env_auth field of a remote of name myremote
you would do:

    rclone config update myremote swift env_auth true

If any of the parameters passed is a password field, then rclone will
automatically obscure them before putting them in the config file.

If the remote uses oauth the token will be updated, if you don’t require
this add an extra parameter thus:

    rclone config update myremote swift env_auth true config_refresh_token false

    rclone config update <name> [<key> <value>]+ [flags]

Options

      -h, --help   help for update

SEE ALSO

-   rclone config - Enter an interactive configuration session.

Auto generated by spf13/cobra on 15-Jun-2019


rclone copyto

Copy files from source to dest, skipping already copied

Synopsis

If source:path is a file or directory then it copies it to a file or
directory named dest:path.

This can be used to upload single files to other than their current
name. If the source is a directory then it acts exactly like the copy
command.

So

    rclone copyto src dst

where src and dst are rclone paths, either remote:path or /path/to/local
or C:.

This will:

    if src is file
        copy it to dst, overwriting an existing file if it exists
    if src is directory
        copy it to dst, overwriting existing files if they exist
        see copy command for full details

This doesn’t transfer unchanged files, testing by size and modification
time or MD5SUM. It doesn’t delete files from the destination.

NOTE: Use the -P/--progress flag to view real-time transfer statistics

    rclone copyto source:path dest:path [flags]

Options

      -h, --help   help for copyto

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone copyurl

Copy url content to dest.

Synopsis

Download urls content and copy it to destination without saving it in
tmp storage.

    rclone copyurl https://example.com dest:path [flags]

Options

      -h, --help   help for copyurl

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone cryptcheck

Cryptcheck checks the integrity of a crypted remote.

Synopsis

rclone cryptcheck checks a remote against a crypted remote. This is the
equivalent of running rclone check, but able to check the checksums of
the crypted remote.

For it to work the underlying remote of the cryptedremote must support
some kind of checksum.

It works by reading the nonce from each file on the cryptedremote: and
using that to encrypt each file on the remote:. It then checks the
checksum of the underlying file on the cryptedremote: against the
checksum of the file it has just encrypted.

Use it like this

    rclone cryptcheck /path/to/files encryptedremote:path

You can use it like this also, but that will involve downloading all the
files in remote:path.

    rclone cryptcheck remote:path encryptedremote:path

After it has run it will log the status of the encryptedremote:.

If you supply the –one-way flag, it will only check that files in source
match the files in destination, not the other way around. Meaning extra
files in destination that are not in the source will not trigger an
error.

    rclone cryptcheck remote:path cryptedremote:path [flags]

Options

      -h, --help      help for cryptcheck
          --one-way   Check one way only, source files must exist on destination

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone cryptdecode

Cryptdecode returns unencrypted file names.

Synopsis

rclone cryptdecode returns unencrypted file names when provided with a
list of encrypted file names. List limit is 10 items.

If you supply the –reverse flag, it will return encrypted file names.

use it like this

    rclone cryptdecode encryptedremote: encryptedfilename1 encryptedfilename2

    rclone cryptdecode --reverse encryptedremote: filename1 filename2

    rclone cryptdecode encryptedremote: encryptedfilename [flags]

Options

      -h, --help      help for cryptdecode
          --reverse   Reverse cryptdecode, encrypts filenames

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone dbhashsum

Produces a Dropbox hash file for all the objects in the path.

Synopsis

Produces a Dropbox hash file for all the objects in the path. The hashes
are calculated according to Dropbox content hash rules. The output is in
the same format as md5sum and sha1sum.

    rclone dbhashsum remote:path [flags]

Options

      -h, --help   help for dbhashsum

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone deletefile

Remove a single file from remote.

Synopsis

Remove a single file from remote. Unlike delete it cannot be used to
remove a directory and it doesn’t obey include/exclude filters - if the
specified file exists, it will always be removed.

    rclone deletefile remote:path [flags]

Options

      -h, --help   help for deletefile

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone genautocomplete

Output completion script for a given shell.

Synopsis

Generates a shell completion script for rclone. Run with –help to list
the supported shells.

Options

      -h, --help   help for genautocomplete

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.
-   rclone genautocomplete bash - Output bash completion script for
    rclone.
-   rclone genautocomplete zsh - Output zsh completion script for
    rclone.

Auto generated by spf13/cobra on 15-Jun-2019


rclone genautocomplete bash

Output bash completion script for rclone.

Synopsis

Generates a bash shell autocompletion script for rclone.

This writes to /etc/bash_completion.d/rclone by default so will probably
need to be run with sudo or as root, eg

    sudo rclone genautocomplete bash

Logout and login again to use the autocompletion scripts, or source them
directly

    . /etc/bash_completion

If you supply a command line argument the script will be written there.

    rclone genautocomplete bash [output_file] [flags]

Options

      -h, --help   help for bash

SEE ALSO

-   rclone genautocomplete - Output completion script for a given shell.

Auto generated by spf13/cobra on 15-Jun-2019


rclone genautocomplete zsh

Output zsh completion script for rclone.

Synopsis

Generates a zsh autocompletion script for rclone.

This writes to /usr/share/zsh/vendor-completions/_rclone by default so
will probably need to be run with sudo or as root, eg

    sudo rclone genautocomplete zsh

Logout and login again to use the autocompletion scripts, or source them
directly

    autoload -U compinit && compinit

If you supply a command line argument the script will be written there.

    rclone genautocomplete zsh [output_file] [flags]

Options

      -h, --help   help for zsh

SEE ALSO

-   rclone genautocomplete - Output completion script for a given shell.

Auto generated by spf13/cobra on 15-Jun-2019


rclone gendocs

Output markdown docs for rclone to the directory supplied.

Synopsis

This produces markdown docs for the rclone commands to the directory
supplied. These are in a format suitable for hugo to render into the
rclone.org website.

    rclone gendocs output_directory [flags]

Options

      -h, --help   help for gendocs

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone hashsum

Produces an hashsum file for all the objects in the path.

Synopsis

Produces a hash file for all the objects in the path using the hash
named. The output is in the same format as the standard md5sum/sha1sum
tool.

Run without a hash to see the list of supported hashes, eg

    $ rclone hashsum
    Supported hashes are:
      * MD5
      * SHA-1
      * DropboxHash
      * QuickXorHash

Then

    $ rclone hashsum MD5 remote:path

    rclone hashsum <hash> remote:path [flags]

Options

      -h, --help   help for hashsum

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone link

Generate public link to file/folder.

Synopsis

rclone link will create or retrieve a public link to the given file or
folder.

    rclone link remote:path/to/file
    rclone link remote:path/to/folder/

If successful, the last line of the output will contain the link. Exact
capabilities depend on the remote, but the link will always be created
with the least constraints – e.g. no expiry, no password protection,
accessible without account.

    rclone link remote:path [flags]

Options

      -h, --help   help for link

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone listremotes

List all the remotes in the config file.

Synopsis

rclone listremotes lists all the available remotes from the config file.

When uses with the -l flag it lists the types too.

    rclone listremotes [flags]

Options

      -h, --help   help for listremotes
          --long   Show the type as well as names.

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone lsf

List directories and objects in remote:path formatted for parsing

Synopsis

List the contents of the source path (directories and objects) to
standard output in a form which is easy to parse by scripts. By default
this will just be the names of the objects and directories, one per
line. The directories will have a / suffix.

Eg

    $ rclone lsf swift:bucket
    bevajer5jef
    canole
    diwogej7
    ferejej3gux/
    fubuwic

Use the –format option to control what gets listed. By default this is
just the path, but you can use these parameters to control the output:

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
–format “pst”, or maybe –format “tsp” to put the path last.

Eg

    $ rclone lsf  --format "tsp" swift:bucket
    2016-06-25 18:55:41;60295;bevajer5jef
    2016-06-25 18:55:43;90613;canole
    2016-06-25 18:55:43;94467;diwogej7
    2018-04-26 08:50:45;0;ferejej3gux/
    2016-06-25 18:55:40;37600;fubuwic

If you specify “h” in the format you will get the MD5 hash by default,
use the “–hash” flag to change which hash you want. Note that this can
be returned as an empty string if it isn’t available on the object (and
for directories), “ERROR” if there was an error reading it from the
object and “UNSUPPORTED” if that object does not support that hash type.

For example to emulate the md5sum command you can use

    rclone lsf -R --hash MD5 --format hp --separator "  " --files-only .

Eg

    $ rclone lsf -R --hash MD5 --format hp --separator "  " --files-only swift:bucket 
    7908e352297f0f530b84a756f188baa3  bevajer5jef
    cd65ac234e6fea5925974a51cdd865cc  canole
    03b5341b4f234b9d984d03ad076bae91  diwogej7
    8fd37c3810dd660778137ac3a66cc06d  fubuwic
    99713e14a4c4ff553acaf1930fad985b  gixacuh7ku

(Though “rclone md5sum .” is an easier way of typing this.)

By default the separator is “;” this can be changed with the –separator
flag. Note that separators aren’t escaped in the path so putting it last
is a good strategy.

Eg

    $ rclone lsf  --separator "," --format "tshp" swift:bucket
    2016-06-25 18:55:41,60295,7908e352297f0f530b84a756f188baa3,bevajer5jef
    2016-06-25 18:55:43,90613,cd65ac234e6fea5925974a51cdd865cc,canole
    2016-06-25 18:55:43,94467,03b5341b4f234b9d984d03ad076bae91,diwogej7
    2018-04-26 08:52:53,0,,ferejej3gux/
    2016-06-25 18:55:40,37600,8fd37c3810dd660778137ac3a66cc06d,fubuwic

You can output in CSV standard format. This will escape things in " if
they contain ,

Eg

    $ rclone lsf --csv --files-only --format ps remote:path
    test.log,22355
    test.sh,449
    "this file contains a comma, in the file name.txt",6

Note that the –absolute parameter is useful for making lists of files to
pass to an rclone copy with the –files-from flag.

For example to find all the files modified within one day and copy those
only (without traversing the whole directory structure):

    rclone lsf --absolute --files-only --max-age 1d /path/to/local > new_files
    rclone copy --files-from new_files /path/to/local remote:path

Any of the filtering options can be applied to this command.

There are several related list commands

-   ls to list size and path of objects only
-   lsl to list modification time, size and path of objects only
-   lsd to list directories only
-   lsf to list objects and directories in easy to parse format
-   lsjson to list objects and directories in JSON format

ls,lsl,lsd are designed to be human readable. lsf is designed to be
human and machine readable. lsjson is designed to be machine readable.

Note that ls and lsl recurse by default - use “–max-depth 1” to stop the
recursion.

The other list commands lsd,lsf,lsjson do not recurse by default - use
“-R” to make them recurse.

Listing a non existent directory will produce an error except for
remotes which can’t have empty directories (eg s3, swift, gcs, etc - the
bucket based remotes).

    rclone lsf remote:path [flags]

Options

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

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone lsjson

List directories and objects in the path in JSON format.

Synopsis

List directories and objects in the path in JSON format.

The output is an array of Items, where each Item looks like this

{ “Hashes” : { “SHA-1” : “f572d396fae9206628714fb2ce00f72e94f2258f”,
“MD5” : “b1946ac92492d2347c6235b4d2611184”, “DropboxHash” :
“ecb65bb98f9d905b70458986c39fcbad7715e5f2fcc3b1f07767d7c83e2438cc” },
“ID”: “y2djkhiujf83u33”, “OrigID”: “UYOJVTUW00Q1RzTDA”, “IsBucket” :
false, “IsDir” : false, “MimeType” : “application/octet-stream”,
“ModTime” : “2017-05-31T16:15:57.034468261+01:00”, “Name” : “file.txt”,
“Encrypted” : “v0qpsdq8anpci8n929v3uu9338”, “EncryptedPath” :
“kja9098349023498/v0qpsdq8anpci8n929v3uu9338”, “Path” :
“full/path/goes/here/file.txt”, “Size” : 6, “Tier” : “hot”, }

If –hash is not specified the Hashes property won’t be emitted.

If –no-modtime is specified then ModTime will be blank.

If –encrypted is not specified the Encrypted won’t be emitted.

If –dirs-only is not specified files in addition to directories are
returned

If –files-only is not specified directories in addition to the files
will be returned.

The Path field will only show folders below the remote path being
listed. If “remote:path” contains the file “subfolder/file.txt”, the
Path for “file.txt” will be “subfolder/file.txt”, not
“remote:path/subfolder/file.txt”. When used without –recursive the Path
will always be the same as Name.

If the directory is a bucket in a bucket based backend, then “IsBucket”
will be set to true. This key won’t be present unless it is “true”.

The time is in RFC3339 format with up to nanosecond precision. The
number of decimal digits in the seconds will depend on the precision
that the remote can hold the times, so if times are accurate to the
nearest millisecond (eg Google Drive) then 3 digits will always be shown
(“2017-05-31T16:15:57.034+01:00”) whereas if the times are accurate to
the nearest second (Dropbox, Box, WebDav etc) no digits will be shown
(“2017-05-31T16:15:57+01:00”).

The whole output can be processed as a JSON blob, or alternatively it
can be processed line by line as each item is written one to a line.

Any of the filtering options can be applied to this command.

There are several related list commands

-   ls to list size and path of objects only
-   lsl to list modification time, size and path of objects only
-   lsd to list directories only
-   lsf to list objects and directories in easy to parse format
-   lsjson to list objects and directories in JSON format

ls,lsl,lsd are designed to be human readable. lsf is designed to be
human and machine readable. lsjson is designed to be machine readable.

Note that ls and lsl recurse by default - use “–max-depth 1” to stop the
recursion.

The other list commands lsd,lsf,lsjson do not recurse by default - use
“-R” to make them recurse.

Listing a non existent directory will produce an error except for
remotes which can’t have empty directories (eg s3, swift, gcs, etc - the
bucket based remotes).

    rclone lsjson remote:path [flags]

Options

          --dirs-only    Show only directories in the listing.
      -M, --encrypted    Show the encrypted names.
          --files-only   Show only files in the listing.
          --hash         Include hashes in the output (may take longer).
      -h, --help         help for lsjson
          --no-modtime   Don't read the modification time (can speed things up).
          --original     Show the ID of the underlying Object.
      -R, --recursive    Recurse into the listing.

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone mount

Mount the remote as file system on a mountpoint.

Synopsis

rclone mount allows Linux, FreeBSD, macOS and Windows to mount any of
Rclone’s cloud storage systems as a file system with FUSE.

First set up your remote using rclone config. Check it works with
rclone ls etc.

Start the mount like this

    rclone mount remote:path/to/files /path/to/local/mount

Or on Windows like this where X: is an unused drive letter

    rclone mount remote:path/to/files X:

When the program ends, either via Ctrl+C or receiving a SIGINT or
SIGTERM signal, the mount is automatically stopped.

The umount operation can fail, for example when the mountpoint is busy.
When that happens, it is the user’s responsibility to stop the mount
manually with

    # Linux
    fusermount -u /path/to/local/mount
    # OS X
    umount /path/to/local/mount

Installing on Windows

To run rclone mount on Windows, you will need to download and install
WinFsp.

WinFsp is an open source Windows File System Proxy which makes it easy
to write user space file systems for Windows. It provides a FUSE
emulation layer which rclone uses combination with cgofuse. Both of
these packages are by Bill Zissimopoulos who was very helpful during the
implementation of rclone mount for Windows.

Windows caveats

Note that drives created as Administrator are not visible by other
accounts (including the account that was elevated as Administrator). So
if you start a Windows drive from an Administrative Command Prompt and
then try to access the same drive from Explorer (which does not run as
Administrator), you will not be able to see the new drive.

The easiest way around this is to start the drive from a normal command
prompt. It is also possible to start a drive from the SYSTEM account
(using the WinFsp.Launcher infrastructure) which creates drives
accessible for everyone on the system or alternatively using the nssm
service manager.

Limitations

Without the use of “–vfs-cache-mode” this can only write files
sequentially, it can only seek when reading. This means that many
applications won’t work with their files on an rclone mount without
“–vfs-cache-mode writes” or “–vfs-cache-mode full”. See the File Caching
section for more info.

The bucket based remotes (eg Swift, S3, Google Compute Storage, B2,
Hubic) won’t work from the root - you will need to specify a bucket, or
a path within the bucket. So swift: won’t work whereas swift:bucket will
as will swift:bucket/path. None of these support the concept of
directories, so empty directories will have a tendency to disappear once
they fall out of the directory cache.

Only supported on Linux, FreeBSD, OS X and Windows at the moment.

rclone mount vs rclone sync/copy

File systems expect things to be 100% reliable, whereas cloud storage
systems are a long way from 100% reliable. The rclone sync/copy commands
cope with this with lots of retries. However rclone mount can’t use
retries in the same way without making local copies of the uploads. Look
at the file caching for solutions to make mount more reliable.

Attribute caching

You can use the flag –attr-timeout to set the time the kernel caches the
attributes (size, modification time etc) for directory entries.

The default is “1s” which caches files just long enough to avoid too
many callbacks to rclone from the kernel.

In theory 0s should be the correct value for filesystems which can
change outside the control of the kernel. However this causes quite a
few problems such as rclone using too much memory, rclone not serving
files to samba and excessive time listing directories.

The kernel can cache the info about a file for the time given by
“–attr-timeout”. You may see corruption if the remote file changes
length during this window. It will show up as either a truncated file or
a file with garbage on the end. With “–attr-timeout 1s” this is very
unlikely but not impossible. The higher you set “–attr-timeout” the more
likely it is. The default setting of “1s” is the lowest setting which
mitigates the problems above.

If you set it higher (‘10s’ or ‘1m’ say) then the kernel will call back
to rclone less often making it more efficient, however there is more
chance of the corruption issue above.

If files don’t change on the remote outside of the control of rclone
then there is no chance of corruption.

This is the same as setting the attr_timeout option in mount.fuse.

Filters

Note that all the rclone filters can be used to select a subset of the
files to be visible in the mount.

systemd

When running rclone mount as a systemd service, it is possible to use
Type=notify. In this case the service will enter the started state after
the mountpoint has been successfully set up. Units having the rclone
mount service specified as a requirement will see all files and folders
immediately in this mode.

chunked reading

–vfs-read-chunk-size will enable reading the source objects in parts.
This can reduce the used download quota for some remotes by requesting
only chunks from the remote that are actually read at the cost of an
increased number of requests.

When –vfs-read-chunk-size-limit is also specified and greater than
–vfs-read-chunk-size, the chunk size for each open file will get doubled
for each chunk read, until the specified value is reached. A value of -1
will disable the limit and the chunk size will grow indefinitely.

With –vfs-read-chunk-size 100M and –vfs-read-chunk-size-limit 0 the
following parts will be downloaded: 0-100M, 100M-200M, 200M-300M,
300M-400M and so on. When –vfs-read-chunk-size-limit 500M is specified,
the result would be 0-100M, 100M-300M, 300M-700M, 700M-1200M,
1200M-1700M and so on.

Chunked reading will only work with –vfs-cache-mode < full, as the file
will always be copied to the vfs cache before opening with
–vfs-cache-mode full.

Directory Cache

Using the --dir-cache-time flag, you can set how long a directory should
be considered up to date and not refreshed from the backend. Changes
made locally in the mount may appear immediately or invalidate the
cache. However, changes done on the remote will only be picked up once
the cache expires.

Alternatively, you can send a SIGHUP signal to rclone for it to flush
all directory caches, regardless of how old they are. Assuming only one
rclone instance is running, you can reset the cache like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a remote control then you can use rclone rc
to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

File Buffering

The --buffer-size flag determines the amount of memory, that will be
used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of data
in memory at all times. The buffered data is bound to one file
descriptor and won’t be shared between multiple open file descriptors of
the same file.

This flag is a upper limit for the used memory per file descriptor. The
buffer will only use memory for data that is downloaded but not not yet
read. If the buffer is empty, only a small amount of memory will be
used. The maximum memory used by rclone for buffering can be up to
--buffer-size * open files.

File Caching

These flags control the VFS file caching options. The VFS layer is used
by rclone mount to make a cloud storage system work more like a normal
file system.

You’ll need to enable VFS caching if you want, for example, to read and
write simultaneously to a file. See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

If run with -vv rclone will print the location of the file cache. The
files are stored in the user cache file area which is OS dependent but
can be controlled with --cache-dir or setting the appropriate
environment variable.

The cache has 4 different modes selected by --vfs-cache-mode. The higher
the cache mode the more compatible rclone becomes at the cost of using
disk space.

Note that files are written back to the remote only when they are closed
so if rclone is quit or dies with open files then these won’t get
written back to the remote. However they will still be in the on disk
cache.

If using –vfs-cache-max-size note that the cache may exceed this size
for two reasons. Firstly because it is only checked every
–vfs-cache-poll-interval. Secondly because open files cannot be evicted
from the cache.

–vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

-   Files can’t be opened for both read AND write
-   Files opened for write can’t be seeked
-   Existing files opened for write must have O_TRUNC set
-   Files open for read with O_TRUNC will be opened write only
-   Files open for write only will behave as if O_TRUNC was supplied
-   Open modes O_APPEND, O_TRUNC are ignored
-   If an upload fails it can’t be retried

–vfs-cache-mode minimal

This is very similar to “off” except that files opened for read AND
write will be buffered to disks. This means that files opened for write
will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

-   Files opened for write only can’t be seeked
-   Existing files opened for write must have O_TRUNC set
-   Files opened for write only will ignore O_APPEND, O_TRUNC
-   If an upload fails it can’t be retried

–vfs-cache-mode writes

In this mode files opened for read only are still read directly from the
remote, write only and read/write files are buffered to disk first.

This mode should support all normal file system operations.

If an upload fails it will be retried up to –low-level-retries times.

–vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk. When a
file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at the
cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk, it
will be kept on the disk after it is written to the remote. It will be
purged on a schedule according to --vfs-cache-max-age.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
–low-level-retries times.

    rclone mount remote:path /path/to/mountpoint [flags]

Options

          --allow-non-empty                        Allow mounting over a non-empty directory.
          --allow-other                            Allow access to other users.
          --allow-root                             Allow access to root user.
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
          --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
          --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)
          --volname string                         Set the volume name (not supported by all OSes).
          --write-back-cache                       Makes kernel buffer writes before sending them to rclone. Without this, writethrough caching is used.

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone moveto

Move file or directory from source to dest.

Synopsis

If source:path is a file or directory then it moves it to a file or
directory named dest:path.

This can be used to rename files or upload single files to other than
their existing name. If the source is a directory then it acts exactly
like the move command.

So

    rclone moveto src dst

where src and dst are rclone paths, either remote:path or /path/to/local
or C:.

This will:

    if src is file
        move it to dst, overwriting an existing file if it exists
    if src is directory
        move it to dst, overwriting existing files if they exist
        see move command for full details

This doesn’t transfer unchanged files, testing by size and modification
time or MD5SUM. src will be deleted on successful transfer.

IMPORTANT: Since this can cause data loss, test first with the –dry-run
flag.

NOTE: Use the -P/--progress flag to view real-time transfer statistics.

    rclone moveto source:path dest:path [flags]

Options

      -h, --help   help for moveto

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone ncdu

Explore a remote with a text based user interface.

Synopsis

This displays a text based user interface allowing the navigation of a
remote. It is most useful for answering the question - “What is using
all my disk space?”.

To make the user interface it first scans the entire remote given and
builds an in memory representation. rclone ncdu can be used during this
scanning phase and you will see it building up the directory structure
as it goes along.

Here are the keys - press ‘?’ to toggle the help on and off

     ↑,↓ or k,j to Move
     →,l to enter
     ←,h to return
     c toggle counts
     g toggle graph
     n,s,C sort by name,size,count
     d delete file/directory
     ^L refresh screen
     ? to toggle help on and off
     q/ESC/c-C to quit

This an homage to the ncdu tool but for rclone remotes. It is missing
lots of features at the moment but is useful as it stands.

Note that it might take some time to delete big files/folders. The UI
won’t respond in the meantime since the deletion is done synchronously.

    rclone ncdu remote:path [flags]

Options

      -h, --help   help for ncdu

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone obscure

Obscure password for use in the rclone.conf

Synopsis

Obscure password for use in the rclone.conf

    rclone obscure password [flags]

Options

      -h, --help   help for obscure

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone rc

Run a command against a running rclone.

Synopsis

This runs a command against a running rclone. Use the –url flag to
specify an non default URL to connect on. This can be either a “:port”
which is taken to mean “http://localhost:port” or a “host:port” which is
taken to mean “http://host:port”

A username and password can be passed in with –user and –pass.

Note that –rc-addr, –rc-user, –rc-pass will be read also for –url,
–user, –pass.

Arguments should be passed in as parameter=value.

The result will be returned as a JSON object by default.

The –json parameter can be used to pass in a JSON blob as an input
instead of key=value arguments. This is the only way of passing in more
complicated values.

Use –loopback to connect to the rclone instance running “rclone rc”.
This is very useful for testing commands without having to run an rclone
rc server, eg:

    rclone rc --loopback operations/about fs=/

Use “rclone rc” to see a list of all possible commands.

    rclone rc commands parameter [flags]

Options

      -h, --help          help for rc
          --json string   Input JSON - use instead of key=value args.
          --loopback      If set connect to this rclone instance not via HTTP.
          --no-output     If set don't output the JSON result.
          --pass string   Password to use to connect to rclone remote control.
          --url string    URL to connect to rclone remote control. (default "http://localhost:5572/")
          --user string   Username to use to rclone remote control.

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone rcat

Copies standard input to file on remote.

Synopsis

rclone rcat reads from standard input (stdin) and copies it to a single
remote file.

    echo "hello world" | rclone rcat remote:path/to/file
    ffmpeg - | rclone rcat remote:path/to/file

If the remote file already exists, it will be overwritten.

rcat will try to upload small files in a single request, which is
usually more efficient than the streaming/chunked upload endpoints,
which use multiple requests. Exact behaviour depends on the remote. What
is considered a small file may be set through --streaming-upload-cutoff.
Uploading only starts after the cutoff is reached or if the file ends
before that. The data must fit into RAM. The cutoff needs to be small
enough to adhere the limits of your remote, please see there. Generally
speaking, setting this cutoff too high will decrease your performance.

Note that the upload can also not be retried because the data is not
kept around until the upload succeeds. If you need to transfer a lot of
data, you’re better off caching locally and then rclone move it to the
destination.

    rclone rcat remote:path [flags]

Options

      -h, --help   help for rcat

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone rcd

Run rclone listening to remote control commands only.

Synopsis

This runs rclone so that it only listens to remote control commands.

This is useful if you are controlling rclone via the rc API.

If you pass in a path to a directory, rclone will serve that directory
for GET requests on the URL passed in. It will also open the URL in the
browser when rclone is run.

See the rc documentation for more info on the rc flags.

    rclone rcd <path to files to serve>* [flags]

Options

      -h, --help   help for rcd

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone rmdirs

Remove empty directories under the path.

Synopsis

This removes any empty directories (or directories that only contain
empty directories) under the path that it finds, including the path if
it has nothing in.

If you supply the –leave-root flag, it will not remove the root
directory.

This is useful for tidying up remotes that rclone has left a lot of
empty directories in.

    rclone rmdirs remote:path [flags]

Options

      -h, --help         help for rmdirs
          --leave-root   Do not remove root directory if empty

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone serve

Serve a remote over a protocol.

Synopsis

rclone serve is used to serve a remote over a given protocol. This
command requires the use of a subcommand to specify the protocol, eg

    rclone serve http remote:

Each subcommand has its own options which you can see in their help.

    rclone serve <protocol> [opts] <remote> [flags]

Options

      -h, --help   help for serve

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.
-   rclone serve dlna - Serve remote:path over DLNA
-   rclone serve ftp - Serve remote:path over FTP.
-   rclone serve http - Serve the remote over HTTP.
-   rclone serve restic - Serve the remote for restic’s REST API.
-   rclone serve sftp - Serve the remote over SFTP.
-   rclone serve webdav - Serve remote:path over webdav.

Auto generated by spf13/cobra on 15-Jun-2019


rclone serve dlna

Serve remote:path over DLNA

Synopsis

rclone serve dlna is a DLNA media server for media stored in a rclone
remote. Many devices, such as the Xbox and PlayStation, can
automatically discover this server in the LAN and play audio/video from
it. VLC is also supported. Service discovery uses UDP multicast packets
(SSDP) and will thus only work on LANs.

Rclone will list all files present in the remote, without filtering
based on media formats or file extensions. Additionally, there is no
media transcoding support. This means that some players might show files
that they are not able to play back correctly.

Server options

Use –addr to specify which IP address and port the server should listen
on, eg –addr 1.2.3.4:8000 or –addr :8080 to listen to all IPs.

Use –name to choose the friendly server name, which is by default
“rclone (hostname)”.

Use –log-trace in conjunction with -vv to enable additional debug
logging of all UPNP traffic.

Directory Cache

Using the --dir-cache-time flag, you can set how long a directory should
be considered up to date and not refreshed from the backend. Changes
made locally in the mount may appear immediately or invalidate the
cache. However, changes done on the remote will only be picked up once
the cache expires.

Alternatively, you can send a SIGHUP signal to rclone for it to flush
all directory caches, regardless of how old they are. Assuming only one
rclone instance is running, you can reset the cache like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a remote control then you can use rclone rc
to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

File Buffering

The --buffer-size flag determines the amount of memory, that will be
used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of data
in memory at all times. The buffered data is bound to one file
descriptor and won’t be shared between multiple open file descriptors of
the same file.

This flag is a upper limit for the used memory per file descriptor. The
buffer will only use memory for data that is downloaded but not not yet
read. If the buffer is empty, only a small amount of memory will be
used. The maximum memory used by rclone for buffering can be up to
--buffer-size * open files.

File Caching

These flags control the VFS file caching options. The VFS layer is used
by rclone mount to make a cloud storage system work more like a normal
file system.

You’ll need to enable VFS caching if you want, for example, to read and
write simultaneously to a file. See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

If run with -vv rclone will print the location of the file cache. The
files are stored in the user cache file area which is OS dependent but
can be controlled with --cache-dir or setting the appropriate
environment variable.

The cache has 4 different modes selected by --vfs-cache-mode. The higher
the cache mode the more compatible rclone becomes at the cost of using
disk space.

Note that files are written back to the remote only when they are closed
so if rclone is quit or dies with open files then these won’t get
written back to the remote. However they will still be in the on disk
cache.

If using –vfs-cache-max-size note that the cache may exceed this size
for two reasons. Firstly because it is only checked every
–vfs-cache-poll-interval. Secondly because open files cannot be evicted
from the cache.

–vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

-   Files can’t be opened for both read AND write
-   Files opened for write can’t be seeked
-   Existing files opened for write must have O_TRUNC set
-   Files open for read with O_TRUNC will be opened write only
-   Files open for write only will behave as if O_TRUNC was supplied
-   Open modes O_APPEND, O_TRUNC are ignored
-   If an upload fails it can’t be retried

–vfs-cache-mode minimal

This is very similar to “off” except that files opened for read AND
write will be buffered to disks. This means that files opened for write
will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

-   Files opened for write only can’t be seeked
-   Existing files opened for write must have O_TRUNC set
-   Files opened for write only will ignore O_APPEND, O_TRUNC
-   If an upload fails it can’t be retried

–vfs-cache-mode writes

In this mode files opened for read only are still read directly from the
remote, write only and read/write files are buffered to disk first.

This mode should support all normal file system operations.

If an upload fails it will be retried up to –low-level-retries times.

–vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk. When a
file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at the
cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk, it
will be kept on the disk after it is written to the remote. It will be
purged on a schedule according to --vfs-cache-max-age.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
–low-level-retries times.

    rclone serve dlna remote:path [flags]

Options

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
          --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
          --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)

SEE ALSO

-   rclone serve - Serve a remote over a protocol.

Auto generated by spf13/cobra on 15-Jun-2019


rclone serve ftp

Serve remote:path over FTP.

Synopsis

rclone serve ftp implements a basic ftp server to serve the remote over
FTP protocol. This can be viewed with a ftp client or you can make a
remote of type ftp to read and write it.

Server options

Use –addr to specify which IP address and port the server should listen
on, eg –addr 1.2.3.4:8000 or –addr :8080 to listen to all IPs. By
default it only listens on localhost. You can use port :0 to let the OS
choose an available port.

If you set –addr to listen on a public or LAN accessible IP address then
using Authentication is advised - see the next section for info.

Authentication

By default this will serve files without needing a login.

You can set a single username and password with the –user and –pass
flags.

Directory Cache

Using the --dir-cache-time flag, you can set how long a directory should
be considered up to date and not refreshed from the backend. Changes
made locally in the mount may appear immediately or invalidate the
cache. However, changes done on the remote will only be picked up once
the cache expires.

Alternatively, you can send a SIGHUP signal to rclone for it to flush
all directory caches, regardless of how old they are. Assuming only one
rclone instance is running, you can reset the cache like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a remote control then you can use rclone rc
to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

File Buffering

The --buffer-size flag determines the amount of memory, that will be
used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of data
in memory at all times. The buffered data is bound to one file
descriptor and won’t be shared between multiple open file descriptors of
the same file.

This flag is a upper limit for the used memory per file descriptor. The
buffer will only use memory for data that is downloaded but not not yet
read. If the buffer is empty, only a small amount of memory will be
used. The maximum memory used by rclone for buffering can be up to
--buffer-size * open files.

File Caching

These flags control the VFS file caching options. The VFS layer is used
by rclone mount to make a cloud storage system work more like a normal
file system.

You’ll need to enable VFS caching if you want, for example, to read and
write simultaneously to a file. See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

If run with -vv rclone will print the location of the file cache. The
files are stored in the user cache file area which is OS dependent but
can be controlled with --cache-dir or setting the appropriate
environment variable.

The cache has 4 different modes selected by --vfs-cache-mode. The higher
the cache mode the more compatible rclone becomes at the cost of using
disk space.

Note that files are written back to the remote only when they are closed
so if rclone is quit or dies with open files then these won’t get
written back to the remote. However they will still be in the on disk
cache.

If using –vfs-cache-max-size note that the cache may exceed this size
for two reasons. Firstly because it is only checked every
–vfs-cache-poll-interval. Secondly because open files cannot be evicted
from the cache.

–vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

-   Files can’t be opened for both read AND write
-   Files opened for write can’t be seeked
-   Existing files opened for write must have O_TRUNC set
-   Files open for read with O_TRUNC will be opened write only
-   Files open for write only will behave as if O_TRUNC was supplied
-   Open modes O_APPEND, O_TRUNC are ignored
-   If an upload fails it can’t be retried

–vfs-cache-mode minimal

This is very similar to “off” except that files opened for read AND
write will be buffered to disks. This means that files opened for write
will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

-   Files opened for write only can’t be seeked
-   Existing files opened for write must have O_TRUNC set
-   Files opened for write only will ignore O_APPEND, O_TRUNC
-   If an upload fails it can’t be retried

–vfs-cache-mode writes

In this mode files opened for read only are still read directly from the
remote, write only and read/write files are buffered to disk first.

This mode should support all normal file system operations.

If an upload fails it will be retried up to –low-level-retries times.

–vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk. When a
file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at the
cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk, it
will be kept on the disk after it is written to the remote. It will be
purged on a schedule according to --vfs-cache-max-age.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
–low-level-retries times.

    rclone serve ftp remote:path [flags]

Options

          --addr string                            IPaddress:Port or :Port to bind server to. (default "localhost:2121")
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
          --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
          --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)

SEE ALSO

-   rclone serve - Serve a remote over a protocol.

Auto generated by spf13/cobra on 15-Jun-2019


rclone serve http

Serve the remote over HTTP.

Synopsis

rclone serve http implements a basic web server to serve the remote over
HTTP. This can be viewed in a web browser or you can make a remote of
type http read from it.

You can use the filter flags (eg –include, –exclude) to control what is
served.

The server will log errors. Use -v to see access logs.

–bwlimit will be respected for file transfers. Use –stats to control the
stats printing.

Server options

Use –addr to specify which IP address and port the server should listen
on, eg –addr 1.2.3.4:8000 or –addr :8080 to listen to all IPs. By
default it only listens on localhost. You can use port :0 to let the OS
choose an available port.

If you set –addr to listen on a public or LAN accessible IP address then
using Authentication is advised - see the next section for info.

–server-read-timeout and –server-write-timeout can be used to control
the timeouts on the server. Note that this is the total time for a
transfer.

–max-header-bytes controls the maximum number of bytes the server will
accept in the HTTP header.

Authentication

By default this will serve files without needing a login.

You can either use an htpasswd file which can take lots of users, or set
a single username and password with the –user and –pass flags.

Use –htpasswd /path/to/htpasswd to provide an htpasswd file. This is in
standard apache format and supports MD5, SHA1 and BCrypt for basic
authentication. Bcrypt is recommended.

To create an htpasswd file:

    touch htpasswd
    htpasswd -B htpasswd user
    htpasswd -B htpasswd anotherUser

The password file can be updated while rclone is running.

Use –realm to set the authentication realm.

SSL/TLS

By default this will serve over http. If you want you can serve over
https. You will need to supply the –cert and –key flags. If you wish to
do client side certificate validation then you will need to supply
–client-ca also.

–cert should be a either a PEM encoded certificate or a concatenation of
that with the CA certificate. –key should be the PEM encoded private key
and –client-ca should be the PEM encoded client certificate authority
certificate.

Directory Cache

Using the --dir-cache-time flag, you can set how long a directory should
be considered up to date and not refreshed from the backend. Changes
made locally in the mount may appear immediately or invalidate the
cache. However, changes done on the remote will only be picked up once
the cache expires.

Alternatively, you can send a SIGHUP signal to rclone for it to flush
all directory caches, regardless of how old they are. Assuming only one
rclone instance is running, you can reset the cache like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a remote control then you can use rclone rc
to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

File Buffering

The --buffer-size flag determines the amount of memory, that will be
used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of data
in memory at all times. The buffered data is bound to one file
descriptor and won’t be shared between multiple open file descriptors of
the same file.

This flag is a upper limit for the used memory per file descriptor. The
buffer will only use memory for data that is downloaded but not not yet
read. If the buffer is empty, only a small amount of memory will be
used. The maximum memory used by rclone for buffering can be up to
--buffer-size * open files.

File Caching

These flags control the VFS file caching options. The VFS layer is used
by rclone mount to make a cloud storage system work more like a normal
file system.

You’ll need to enable VFS caching if you want, for example, to read and
write simultaneously to a file. See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

If run with -vv rclone will print the location of the file cache. The
files are stored in the user cache file area which is OS dependent but
can be controlled with --cache-dir or setting the appropriate
environment variable.

The cache has 4 different modes selected by --vfs-cache-mode. The higher
the cache mode the more compatible rclone becomes at the cost of using
disk space.

Note that files are written back to the remote only when they are closed
so if rclone is quit or dies with open files then these won’t get
written back to the remote. However they will still be in the on disk
cache.

If using –vfs-cache-max-size note that the cache may exceed this size
for two reasons. Firstly because it is only checked every
–vfs-cache-poll-interval. Secondly because open files cannot be evicted
from the cache.

–vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

-   Files can’t be opened for both read AND write
-   Files opened for write can’t be seeked
-   Existing files opened for write must have O_TRUNC set
-   Files open for read with O_TRUNC will be opened write only
-   Files open for write only will behave as if O_TRUNC was supplied
-   Open modes O_APPEND, O_TRUNC are ignored
-   If an upload fails it can’t be retried

–vfs-cache-mode minimal

This is very similar to “off” except that files opened for read AND
write will be buffered to disks. This means that files opened for write
will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

-   Files opened for write only can’t be seeked
-   Existing files opened for write must have O_TRUNC set
-   Files opened for write only will ignore O_APPEND, O_TRUNC
-   If an upload fails it can’t be retried

–vfs-cache-mode writes

In this mode files opened for read only are still read directly from the
remote, write only and read/write files are buffered to disk first.

This mode should support all normal file system operations.

If an upload fails it will be retried up to –low-level-retries times.

–vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk. When a
file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at the
cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk, it
will be kept on the disk after it is written to the remote. It will be
purged on a schedule according to --vfs-cache-max-age.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
–low-level-retries times.

    rclone serve http remote:path [flags]

Options

          --addr string                            IPaddress:Port or :Port to bind server to. (default "localhost:8080")
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
          --uid uint32                             Override the uid field set by the filesystem. (default 1000)
          --umask int                              Override the permission bits set by the filesystem. (default 2)
          --user string                            User name for authentication.
          --vfs-cache-max-age duration             Max age of objects in the cache. (default 1h0m0s)
          --vfs-cache-max-size SizeSuffix          Max total size of objects in the cache. (default off)
          --vfs-cache-mode CacheMode               Cache mode off|minimal|writes|full (default off)
          --vfs-cache-poll-interval duration       Interval to poll the cache for stale objects. (default 1m0s)
          --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
          --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)

SEE ALSO

-   rclone serve - Serve a remote over a protocol.

Auto generated by spf13/cobra on 15-Jun-2019


rclone serve restic

Serve the remote for restic’s REST API.

Synopsis

rclone serve restic implements restic’s REST backend API over HTTP. This
allows restic to use rclone as a data storage mechanism for cloud
providers that restic does not support directly.

Restic is a command line program for doing backups.

The server will log errors. Use -v to see access logs.

–bwlimit will be respected for file transfers. Use –stats to control the
stats printing.

Setting up rclone for use by restic

First set up a remote for your chosen cloud provider.

Once you have set up the remote, check it is working with, for example
“rclone lsd remote:”. You may have called the remote something other
than “remote:” - just substitute whatever you called it in the following
instructions.

Now start the rclone restic server

    rclone serve restic -v remote:backup

Where you can replace “backup” in the above by whatever path in the
remote you wish to use.

By default this will serve on “localhost:8080” you can change this with
use of the “–addr” flag.

You might wish to start this server on boot.

Setting up restic to use rclone

Now you can follow the restic instructions on setting up restic.

Note that you will need restic 0.8.2 or later to interoperate with
rclone.

For the example above you will want to use “http://localhost:8080/” as
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

Multiple repositories

Note that you can use the endpoint to host multiple repositories. Do
this by adding a directory name or path after the URL. Note that these
MUST end with /. Eg

    $ export RESTIC_REPOSITORY=rest:http://localhost:8080/user1repo/
    # backup user1 stuff
    $ export RESTIC_REPOSITORY=rest:http://localhost:8080/user2repo/
    # backup user2 stuff

Private repositories

The “–private-repos” flag can be used to limit users to repositories
starting with a path of “//”.

Server options

Use –addr to specify which IP address and port the server should listen
on, eg –addr 1.2.3.4:8000 or –addr :8080 to listen to all IPs. By
default it only listens on localhost. You can use port :0 to let the OS
choose an available port.

If you set –addr to listen on a public or LAN accessible IP address then
using Authentication is advised - see the next section for info.

–server-read-timeout and –server-write-timeout can be used to control
the timeouts on the server. Note that this is the total time for a
transfer.

–max-header-bytes controls the maximum number of bytes the server will
accept in the HTTP header.

Authentication

By default this will serve files without needing a login.

You can either use an htpasswd file which can take lots of users, or set
a single username and password with the –user and –pass flags.

Use –htpasswd /path/to/htpasswd to provide an htpasswd file. This is in
standard apache format and supports MD5, SHA1 and BCrypt for basic
authentication. Bcrypt is recommended.

To create an htpasswd file:

    touch htpasswd
    htpasswd -B htpasswd user
    htpasswd -B htpasswd anotherUser

The password file can be updated while rclone is running.

Use –realm to set the authentication realm.

SSL/TLS

By default this will serve over http. If you want you can serve over
https. You will need to supply the –cert and –key flags. If you wish to
do client side certificate validation then you will need to supply
–client-ca also.

–cert should be a either a PEM encoded certificate or a concatenation of
that with the CA certificate. –key should be the PEM encoded private key
and –client-ca should be the PEM encoded client certificate authority
certificate.

    rclone serve restic remote:path [flags]

Options

          --addr string                     IPaddress:Port or :Port to bind server to. (default "localhost:8080")
          --append-only                     disallow deletion of repository data
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
          --user string                     User name for authentication.

SEE ALSO

-   rclone serve - Serve a remote over a protocol.

Auto generated by spf13/cobra on 15-Jun-2019


rclone serve sftp

Serve the remote over SFTP.

Synopsis

rclone serve sftp implements an SFTP server to serve the remote over
SFTP. This can be used with an SFTP client or you can make a remote of
type sftp to use with it.

You can use the filter flags (eg –include, –exclude) to control what is
served.

The server will log errors. Use -v to see access logs.

–bwlimit will be respected for file transfers. Use –stats to control the
stats printing.

You must provide some means of authentication, either with –user/–pass,
an authorized keys file (specify location with –authorized-keys - the
default is the same as ssh) or set the –no-auth flag for no
authentication when logging in.

Note that this also implements a small number of shell commands so that
it can provide md5sum/sha1sum/df information for the rclone sftp
backend. This means that is can support SHA1SUMs, MD5SUMs and the about
command when paired with the rclone sftp backend.

If you don’t supply a –key then rclone will generate one and cache it
for later use.

By default the server binds to localhost:2022 - if you want it to be
reachable externally then supply “–addr :2022” for example.

Note that the default of “–vfs-cache-mode off” is fine for the rclone
sftp backend, but it may not be with other SFTP clients.

Directory Cache

Using the --dir-cache-time flag, you can set how long a directory should
be considered up to date and not refreshed from the backend. Changes
made locally in the mount may appear immediately or invalidate the
cache. However, changes done on the remote will only be picked up once
the cache expires.

Alternatively, you can send a SIGHUP signal to rclone for it to flush
all directory caches, regardless of how old they are. Assuming only one
rclone instance is running, you can reset the cache like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a remote control then you can use rclone rc
to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

File Buffering

The --buffer-size flag determines the amount of memory, that will be
used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of data
in memory at all times. The buffered data is bound to one file
descriptor and won’t be shared between multiple open file descriptors of
the same file.

This flag is a upper limit for the used memory per file descriptor. The
buffer will only use memory for data that is downloaded but not not yet
read. If the buffer is empty, only a small amount of memory will be
used. The maximum memory used by rclone for buffering can be up to
--buffer-size * open files.

File Caching

These flags control the VFS file caching options. The VFS layer is used
by rclone mount to make a cloud storage system work more like a normal
file system.

You’ll need to enable VFS caching if you want, for example, to read and
write simultaneously to a file. See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

If run with -vv rclone will print the location of the file cache. The
files are stored in the user cache file area which is OS dependent but
can be controlled with --cache-dir or setting the appropriate
environment variable.

The cache has 4 different modes selected by --vfs-cache-mode. The higher
the cache mode the more compatible rclone becomes at the cost of using
disk space.

Note that files are written back to the remote only when they are closed
so if rclone is quit or dies with open files then these won’t get
written back to the remote. However they will still be in the on disk
cache.

If using –vfs-cache-max-size note that the cache may exceed this size
for two reasons. Firstly because it is only checked every
–vfs-cache-poll-interval. Secondly because open files cannot be evicted
from the cache.

–vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

-   Files can’t be opened for both read AND write
-   Files opened for write can’t be seeked
-   Existing files opened for write must have O_TRUNC set
-   Files open for read with O_TRUNC will be opened write only
-   Files open for write only will behave as if O_TRUNC was supplied
-   Open modes O_APPEND, O_TRUNC are ignored
-   If an upload fails it can’t be retried

–vfs-cache-mode minimal

This is very similar to “off” except that files opened for read AND
write will be buffered to disks. This means that files opened for write
will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

-   Files opened for write only can’t be seeked
-   Existing files opened for write must have O_TRUNC set
-   Files opened for write only will ignore O_APPEND, O_TRUNC
-   If an upload fails it can’t be retried

–vfs-cache-mode writes

In this mode files opened for read only are still read directly from the
remote, write only and read/write files are buffered to disk first.

This mode should support all normal file system operations.

If an upload fails it will be retried up to –low-level-retries times.

–vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk. When a
file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at the
cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk, it
will be kept on the disk after it is written to the remote. It will be
purged on a schedule according to --vfs-cache-max-age.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
–low-level-retries times.

    rclone serve sftp remote:path [flags]

Options

          --addr string                            IPaddress:Port or :Port to bind server to. (default "localhost:2022")
          --authorized-keys string                 Authorized keys file (default "~/.ssh/authorized_keys")
          --dir-cache-time duration                Time to cache directory entries for. (default 5m0s)
          --dir-perms FileMode                     Directory permissions (default 0777)
          --file-perms FileMode                    File permissions (default 0666)
          --gid uint32                             Override the gid field set by the filesystem. (default 1000)
      -h, --help                                   help for sftp
          --key string                             SSH private key file (leave blank to auto generate)
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
          --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
          --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)

SEE ALSO

-   rclone serve - Serve a remote over a protocol.

Auto generated by spf13/cobra on 15-Jun-2019


rclone serve webdav

Serve remote:path over webdav.

Synopsis

rclone serve webdav implements a basic webdav server to serve the remote
over HTTP via the webdav protocol. This can be viewed with a webdav
client, through a web browser, or you can make a remote of type webdav
to read and write it.

Webdav options

–etag-hash

This controls the ETag header. Without this flag the ETag will be based
on the ModTime and Size of the object.

If this flag is set to “auto” then rclone will choose the first
supported hash on the backend or you can use a named hash such as “MD5”
or “SHA-1”.

Use “rclone hashsum” to see the full list.

Server options

Use –addr to specify which IP address and port the server should listen
on, eg –addr 1.2.3.4:8000 or –addr :8080 to listen to all IPs. By
default it only listens on localhost. You can use port :0 to let the OS
choose an available port.

If you set –addr to listen on a public or LAN accessible IP address then
using Authentication is advised - see the next section for info.

–server-read-timeout and –server-write-timeout can be used to control
the timeouts on the server. Note that this is the total time for a
transfer.

–max-header-bytes controls the maximum number of bytes the server will
accept in the HTTP header.

Authentication

By default this will serve files without needing a login.

You can either use an htpasswd file which can take lots of users, or set
a single username and password with the –user and –pass flags.

Use –htpasswd /path/to/htpasswd to provide an htpasswd file. This is in
standard apache format and supports MD5, SHA1 and BCrypt for basic
authentication. Bcrypt is recommended.

To create an htpasswd file:

    touch htpasswd
    htpasswd -B htpasswd user
    htpasswd -B htpasswd anotherUser

The password file can be updated while rclone is running.

Use –realm to set the authentication realm.

SSL/TLS

By default this will serve over http. If you want you can serve over
https. You will need to supply the –cert and –key flags. If you wish to
do client side certificate validation then you will need to supply
–client-ca also.

–cert should be a either a PEM encoded certificate or a concatenation of
that with the CA certificate. –key should be the PEM encoded private key
and –client-ca should be the PEM encoded client certificate authority
certificate.

Directory Cache

Using the --dir-cache-time flag, you can set how long a directory should
be considered up to date and not refreshed from the backend. Changes
made locally in the mount may appear immediately or invalidate the
cache. However, changes done on the remote will only be picked up once
the cache expires.

Alternatively, you can send a SIGHUP signal to rclone for it to flush
all directory caches, regardless of how old they are. Assuming only one
rclone instance is running, you can reset the cache like this:

    kill -SIGHUP $(pidof rclone)

If you configure rclone with a remote control then you can use rclone rc
to flush the whole directory cache:

    rclone rc vfs/forget

Or individual files or directories:

    rclone rc vfs/forget file=path/to/file dir=path/to/dir

File Buffering

The --buffer-size flag determines the amount of memory, that will be
used to buffer data in advance.

Each open file descriptor will try to keep the specified amount of data
in memory at all times. The buffered data is bound to one file
descriptor and won’t be shared between multiple open file descriptors of
the same file.

This flag is a upper limit for the used memory per file descriptor. The
buffer will only use memory for data that is downloaded but not not yet
read. If the buffer is empty, only a small amount of memory will be
used. The maximum memory used by rclone for buffering can be up to
--buffer-size * open files.

File Caching

These flags control the VFS file caching options. The VFS layer is used
by rclone mount to make a cloud storage system work more like a normal
file system.

You’ll need to enable VFS caching if you want, for example, to read and
write simultaneously to a file. See below for more details.

Note that the VFS cache works in addition to the cache backend and you
may find that you need one or the other or both.

    --cache-dir string                   Directory rclone will use for caching.
    --vfs-cache-max-age duration         Max age of objects in the cache. (default 1h0m0s)
    --vfs-cache-mode string              Cache mode off|minimal|writes|full (default "off")
    --vfs-cache-poll-interval duration   Interval to poll the cache for stale objects. (default 1m0s)
    --vfs-cache-max-size int             Max total size of objects in the cache. (default off)

If run with -vv rclone will print the location of the file cache. The
files are stored in the user cache file area which is OS dependent but
can be controlled with --cache-dir or setting the appropriate
environment variable.

The cache has 4 different modes selected by --vfs-cache-mode. The higher
the cache mode the more compatible rclone becomes at the cost of using
disk space.

Note that files are written back to the remote only when they are closed
so if rclone is quit or dies with open files then these won’t get
written back to the remote. However they will still be in the on disk
cache.

If using –vfs-cache-max-size note that the cache may exceed this size
for two reasons. Firstly because it is only checked every
–vfs-cache-poll-interval. Secondly because open files cannot be evicted
from the cache.

–vfs-cache-mode off

In this mode the cache will read directly from the remote and write
directly to the remote without caching anything on disk.

This will mean some operations are not possible

-   Files can’t be opened for both read AND write
-   Files opened for write can’t be seeked
-   Existing files opened for write must have O_TRUNC set
-   Files open for read with O_TRUNC will be opened write only
-   Files open for write only will behave as if O_TRUNC was supplied
-   Open modes O_APPEND, O_TRUNC are ignored
-   If an upload fails it can’t be retried

–vfs-cache-mode minimal

This is very similar to “off” except that files opened for read AND
write will be buffered to disks. This means that files opened for write
will be a lot more compatible, but uses the minimal disk space.

These operations are not possible

-   Files opened for write only can’t be seeked
-   Existing files opened for write must have O_TRUNC set
-   Files opened for write only will ignore O_APPEND, O_TRUNC
-   If an upload fails it can’t be retried

–vfs-cache-mode writes

In this mode files opened for read only are still read directly from the
remote, write only and read/write files are buffered to disk first.

This mode should support all normal file system operations.

If an upload fails it will be retried up to –low-level-retries times.

–vfs-cache-mode full

In this mode all reads and writes are buffered to and from disk. When a
file is opened for read it will be downloaded in its entirety first.

This may be appropriate for your needs, or you may prefer to look at the
cache backend which does a much more sophisticated job of caching,
including caching directory hierarchies and chunks of files.

In this mode, unlike the others, when a file is written to the disk, it
will be kept on the disk after it is written to the remote. It will be
purged on a schedule according to --vfs-cache-max-age.

This mode should support all normal file system operations.

If an upload or download fails it will be retried up to
–low-level-retries times.

    rclone serve webdav remote:path [flags]

Options

          --addr string                            IPaddress:Port or :Port to bind server to. (default "localhost:8080")
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
          --uid uint32                             Override the uid field set by the filesystem. (default 1000)
          --umask int                              Override the permission bits set by the filesystem. (default 2)
          --user string                            User name for authentication.
          --vfs-cache-max-age duration             Max age of objects in the cache. (default 1h0m0s)
          --vfs-cache-max-size SizeSuffix          Max total size of objects in the cache. (default off)
          --vfs-cache-mode CacheMode               Cache mode off|minimal|writes|full (default off)
          --vfs-cache-poll-interval duration       Interval to poll the cache for stale objects. (default 1m0s)
          --vfs-read-chunk-size SizeSuffix         Read the source objects in chunks. (default 128M)
          --vfs-read-chunk-size-limit SizeSuffix   If greater than --vfs-read-chunk-size, double the chunk size after each chunk read, until the limit is reached. 'off' is unlimited. (default off)

SEE ALSO

-   rclone serve - Serve a remote over a protocol.

Auto generated by spf13/cobra on 15-Jun-2019


rclone settier

Changes storage class/tier of objects in remote.

Synopsis

rclone settier changes storage tier or class at remote if supported. Few
cloud storage services provides different storage classes on objects,
for example AWS S3 and Glacier, Azure Blob storage - Hot, Cool and
Archive, Google Cloud Storage, Regional Storage, Nearline, Coldline etc.

Note that, certain tier changes make objects not available to access
immediately. For example tiering to archive in azure blob storage makes
objects in frozen state, user can restore by setting tier to Hot/Cool,
similarly S3 to Glacier makes object inaccessible.true

You can use it to tier single object

    rclone settier Cool remote:path/file

Or use rclone filters to set tier on only specific files

    rclone --include "*.txt" settier Hot remote:path/dir

Or just provide remote directory and all files in directory will be
tiered

    rclone settier tier remote:path/dir

    rclone settier tier remote:path [flags]

Options

      -h, --help   help for settier

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone touch

Create new file or change file modification time.

Synopsis

Create new file or change file modification time.

    rclone touch remote:path [flags]

Options

      -h, --help               help for touch
      -C, --no-create          Do not create the file if it does not exist.
      -t, --timestamp string   Change the modification times to the specified time instead of the current time of day. The argument is of the form 'YYMMDD' (ex. 17.10.30) or 'YYYY-MM-DDTHH:MM:SS' (ex. 2006-01-02T15:04:05)

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


rclone tree

List the contents of the remote in a tree like fashion.

Synopsis

rclone tree lists the contents of a remote in a similar way to the unix
tree command.

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
–include and –exclude). You can also use –fast-list.

The tree command has many options for controlling the listing which are
compatible with the tree command. Note that not all of them have short
options as they conflict with rclone’s short options.

    rclone tree remote:path [flags]

Options

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

SEE ALSO

-   rclone - Show help for rclone commands, flags and backends.

Auto generated by spf13/cobra on 15-Jun-2019


Copying single files

rclone normally syncs or copies directories. However, if the source
remote points to a file, rclone will just copy that file. The
destination remote must point to a directory - rclone will give the
error
Failed to create file system for "remote:file": is a file not a directory
if it isn’t.

For example, suppose you have a remote with a file in called test.jpg,
then you could copy just that file like this

    rclone copy remote:test.jpg /tmp/download

The file test.jpg will be placed inside /tmp/download.

This is equivalent to specifying

    rclone copy --files-from /tmp/files remote: /tmp/download

Where /tmp/files contains the single line

    test.jpg

It is recommended to use copy when copying individual files, not sync.
They have pretty much the same effect but copy will use a lot less
memory.


Syntax of remote paths

The syntax of the paths passed to the rclone command are as follows.

/path/to/dir

This refers to the local file system.

On Windows only \ may be used instead of / in local paths ONLY, non
local paths must use /.

These paths needn’t start with a leading / - if they don’t then they
will be relative to the current directory.

remote:path/to/dir

This refers to a directory path/to/dir on remote: as defined in the
config file (configured with rclone config).

remote:/path/to/dir

On most backends this is refers to the same directory as
remote:path/to/dir and that format should be preferred. On a very small
number of remotes (FTP, SFTP, Dropbox for business) this will refer to a
different directory. On these, paths without a leading / will refer to
your “home” directory and paths with a leading / will refer to the root.

:backend:path/to/dir

This is an advanced form for creating remotes on the fly. backend should
be the name or prefix of a backend (the type in the config file) and all
the configuration for the backend should be provided on the command line
(or in environment variables).

Here are some examples:

    rclone lsd --http-url https://pub.rclone.org :http:

To list all the directories in the root of https://pub.rclone.org/.

    rclone lsf --http-url https://example.com :http:path/to/dir

To list files and directories in https://example.com/path/to/dir/

    rclone copy --http-url https://example.com :http:path/to/dir /tmp/dir

To copy files and directories in https://example.com/path/to/dir to
/tmp/dir.

    rclone copy --sftp-host example.com :sftp:path/to/dir /tmp/dir

To copy files and directories from example.com in the relative directory
path/to/dir to /tmp/dir using sftp.


Quoting and the shell

When you are typing commands to your computer you are using something
called the command line shell. This interprets various characters in an
OS specific way.

Here are some gotchas which may help users unfamiliar with the shell
rules

Linux / OSX

If your names have spaces or shell metacharacters (eg *, ?, $, ', " etc)
then you must quote them. Use single quotes ' by default.

    rclone copy 'Important files?' remote:backup

If you want to send a ' you will need to use ", eg

    rclone copy "O'Reilly Reviews" remote:backup

The rules for quoting metacharacters are complicated and if you want the
full details you’ll have to consult the manual page for your shell.

Windows

If your names have spaces in you need to put them in ", eg

    rclone copy "E:\folder name\folder name\folder name" remote:backup

If you are using the root directory on its own then don’t quote it (see
#464 for why), eg

    rclone copy E:\ remote:backup


Copying files or directories with : in the names

rclone uses : to mark a remote name. This is, however, a valid filename
component in non-Windows OSes. The remote name parser will only search
for a : up to the first / so if you need to act on a file or directory
like this then use the full path starting with a /, or use ./ as a
current directory prefix.

So to sync a directory called sync:me to a remote called remote: use

    rclone sync ./sync:me remote:path

or

    rclone sync /full/path/to/sync:me remote:path


Server Side Copy

Most remotes (but not all - see the overview) support server side copy.

This means if you want to copy one folder to another then rclone won’t
download all the files and re-upload them; it will instruct the server
to copy them in place.

Eg

    rclone copy s3:oldbucket s3:newbucket

Will copy the contents of oldbucket to newbucket without downloading and
re-uploading.

Remotes which don’t support server side copy WILL download and re-upload
in this case.

Server side copies are used with sync and copy and will be identified in
the log when using the -v flag. The move command may also use them if
remote doesn’t support server side move directly. This is done by
issuing a server side copy then a delete which is much quicker than a
download and re-upload.

Server side copies will only be attempted if the remote names are the
same.

This can be used when scripting to make aged backups efficiently, eg

    rclone sync remote:current-backup remote:previous-backup
    rclone sync /path/to/files remote:current-backup


Options

Rclone has a number of options to control its behaviour.

Options that take parameters can have the values passed in two ways,
--option=value or --option value. However boolean (true/false) options
behave slightly differently to the other options in that --boolean sets
the option to true and the absence of the flag sets it to false. It is
also possible to specify --boolean=false or --boolean=true. Note that
--boolean false is not valid - this is parsed as --boolean and the false
is parsed as an extra command line argument for rclone.

Options which use TIME use the go time parser. A duration string is a
possibly signed sequence of decimal numbers, each with optional fraction
and a unit suffix, such as “300ms”, “-1.5h” or “2h45m”. Valid time units
are “ns”, “us” (or “µs”), “ms”, “s”, “m”, “h”.

Options which use SIZE use kByte by default. However, a suffix of b for
bytes, k for kBytes, M for MBytes, G for GBytes, T for TBytes and P for
PBytes may be used. These are the binary units, eg 1, 2**10, 2**20,
2**30 respectively.

–backup-dir=DIR

When using sync, copy or move any files which would have been
overwritten or deleted are moved in their original hierarchy into this
directory.

If --suffix is set, then the moved files will have the suffix added to
them. If there is a file with the same path (after the suffix has been
added) in DIR, then it will be overwritten.

The remote in use must support server side move or copy and you must use
the same remote as the destination of the sync. The backup directory
must not overlap the destination directory.

For example

    rclone sync /path/to/local remote:current --backup-dir remote:old

will sync /path/to/local to remote:current, but for any files which
would have been updated or deleted will be stored in remote:old.

If running rclone from a script you might want to use today’s date as
the directory name passed to --backup-dir to store the old files, or you
might want to pass --suffix with today’s date.

–bind string

Local address to bind to for outgoing connections. This can be an IPv4
address (1.2.3.4), an IPv6 address (1234::789A) or host name. If the
host name doesn’t resolve or resolves to more than one IP address it
will give an error.

–bwlimit=BANDWIDTH_SPEC

This option controls the bandwidth limit. Limits can be specified in two
ways: As a single limit, or as a timetable.

Single limits last for the duration of the session. To use a single
limit, specify the desired bandwidth in kBytes/s, or use a suffix
b|k|M|G. The default is 0 which means to not limit bandwidth.

For example, to limit bandwidth usage to 10 MBytes/s use --bwlimit 10M

It is also possible to specify a “timetable” of limits, which will cause
certain limits to be applied at certain times. To specify a timetable,
format your entries as “WEEKDAY-HH:MM,BANDWIDTH
WEEKDAY-HH:MM,BANDWIDTH…” where: WEEKDAY is optional element. It could
be written as whole world or only using 3 first characters. HH:MM is an
hour from 00:00 to 23:59.

An example of a typical timetable to avoid link saturation during
daytime working hours could be:

--bwlimit "08:00,512 12:00,10M 13:00,512 18:00,30M 23:00,off"

In this example, the transfer bandwidth will be every day set to
512kBytes/sec at 8am. At noon, it will raise to 10Mbytes/s, and drop
back to 512kBytes/sec at 1pm. At 6pm, the bandwidth limit will be set to
30MBytes/s, and at 11pm it will be completely disabled (full speed).
Anything between 11pm and 8am will remain unlimited.

An example of timetable with WEEKDAY could be:

--bwlimit "Mon-00:00,512 Fri-23:59,10M Sat-10:00,1M Sun-20:00,off"

It mean that, the transfer bandwidth will be set to 512kBytes/sec on
Monday. It will raise to 10Mbytes/s before the end of Friday. At 10:00
on Sunday it will be set to 1Mbyte/s. From 20:00 at Sunday will be
unlimited.

Timeslots without weekday are extended to whole week. So this one
example:

--bwlimit "Mon-00:00,512 12:00,1M Sun-20:00,off"

Is equal to this:

--bwlimit "Mon-00:00,512Mon-12:00,1M Tue-12:00,1M Wed-12:00,1M Thu-12:00,1M Fri-12:00,1M Sat-12:00,1M Sun-12:00,1M Sun-20:00,off"

Bandwidth limits only apply to the data transfer. They don’t apply to
the bandwidth of the directory listings etc.

Note that the units are Bytes/s, not Bits/s. Typically connections are
measured in Bits/s - to convert divide by 8. For example, let’s say you
have a 10 Mbit/s connection and you wish rclone to use half of it - 5
Mbit/s. This is 5/8 = 0.625MByte/s so you would use a --bwlimit 0.625M
parameter for rclone.

On Unix systems (Linux, MacOS, …) the bandwidth limiter can be toggled
by sending a SIGUSR2 signal to rclone. This allows to remove the
limitations of a long running rclone transfer and to restore it back to
the value specified with --bwlimit quickly when needed. Assuming there
is only one rclone instance running, you can toggle the limiter like
this:

    kill -SIGUSR2 $(pidof rclone)

If you configure rclone with a remote control then you can use change
the bwlimit dynamically:

    rclone rc core/bwlimit rate=1M

–buffer-size=SIZE

Use this sized buffer to speed up file transfers. Each --transfer will
use this much memory for buffering.

When using mount or cmount each open file descriptor will use this much
memory for buffering. See the mount documentation for more details.

Set to 0 to disable the buffering for the minimum memory usage.

Note that the memory allocation of the buffers is influenced by the
–use-mmap flag.

–checkers=N

The number of checkers to run in parallel. Checkers do the equality
checking of files during a sync. For some storage systems (eg S3, Swift,
Dropbox) this can take a significant amount of time so they are run in
parallel.

The default is to run 8 checkers in parallel.

-c, –checksum

Normally rclone will look at modification time and size of files to see
if they are equal. If you set this flag then rclone will check the file
hash and size to determine if files are equal.

This is useful when the remote doesn’t support setting modified time and
a more accurate sync is desired than just checking the file size.

This is very useful when transferring between remotes which store the
same hash type on the object, eg Drive and Swift. For details of which
remotes support which hash type see the table in the overview section.

Eg rclone --checksum sync s3:/bucket swift:/bucket would run much
quicker than without the --checksum flag.

When using this flag, rclone won’t update mtimes of remote files if they
are incorrect as it would normally.

–config=CONFIG_FILE

Specify the location of the rclone config file.

Normally the config file is in your home directory as a file called
.config/rclone/rclone.conf (or .rclone.conf if created with an older
version). If $XDG_CONFIG_HOME is set it will be at
$XDG_CONFIG_HOME/rclone/rclone.conf.

If there is a file rclone.conf in the same directory as the rclone
executable it will be preferred. This file must be created manually for
Rclone to use it, it will never be created automatically.

If you run rclone config file you will see where the default location is
for you.

Use this flag to override the config location, eg
rclone --config=".myconfig" .config.

–contimeout=TIME

Set the connection timeout. This should be in go time format which looks
like 5s for 5 seconds, 10m for 10 minutes, or 3h30m.

The connection timeout is the amount of time rclone will wait for a
connection to go through to a remote object storage system. It is 1m by
default.

–dedupe-mode MODE

Mode to run dedupe command in. One of interactive, skip, first, newest,
oldest, rename. The default is interactive. See the dedupe command for
more information as to what these options mean.

–disable FEATURE,FEATURE,…

This disables a comma separated list of optional features. For example
to disable server side move and server side copy use:

    --disable move,copy

The features can be put in in any case.

To see a list of which features can be disabled use:

    --disable help

See the overview features and optional features to get an idea of which
feature does what.

This flag can be useful for debugging and in exceptional circumstances
(eg Google Drive limiting the total volume of Server Side Copies to
100GB/day).

-n, –dry-run

Do a trial run with no permanent changes. Use this to see what rclone
would do without actually doing it. Useful when setting up the sync
command which deletes files in the destination.

–ignore-case-sync

Using this option will cause rclone to ignore the case of the files when
synchronizing so files will not be copied/synced when the existing
filenames are the same, even if the casing is different.

–ignore-checksum

Normally rclone will check that the checksums of transferred files
match, and give an error “corrupted on transfer” if they don’t.

You can use this option to skip that check. You should only use it if
you have had the “corrupted on transfer” error message and you are sure
you might want to transfer potentially corrupted data.

–ignore-existing

Using this option will make rclone unconditionally skip all files that
exist on the destination, no matter the content of these files.

While this isn’t a generally recommended option, it can be useful in
cases where your files change due to encryption. However, it cannot
correct partial transfers in case a transfer was interrupted.

–ignore-size

Normally rclone will look at modification time and size of files to see
if they are equal. If you set this flag then rclone will check only the
modification time. If --checksum is set then it only checks the
checksum.

It will also cause rclone to skip verifying the sizes are the same after
transfer.

This can be useful for transferring files to and from OneDrive which
occasionally misreports the size of image files (see #399 for more
info).

-I, –ignore-times

Using this option will cause rclone to unconditionally upload all files
regardless of the state of files on the destination.

Normally rclone would skip any files that have the same modification
time and are the same size (or have the same checksum if using
--checksum).

–immutable

Treat source and destination files as immutable and disallow
modification.

With this option set, files will be created and deleted as requested,
but existing files will never be updated. If an existing file does not
match between the source and destination, rclone will give the error
Source and destination exist but do not match: immutable file modified.

Note that only commands which transfer files (e.g. sync, copy, move) are
affected by this behavior, and only modification is disallowed. Files
may still be deleted explicitly (e.g. delete, purge) or implicitly (e.g.
sync, move). Use copy --immutable if it is desired to avoid deletion as
well as modification.

This can be useful as an additional layer of protection for immutable or
append-only data sets (notably backup archives), where modification
implies corruption and should not be propagated.


–leave-root

During rmdirs it will not remove root directory, even if it’s empty.

–log-file=FILE

Log all of rclone’s output to FILE. This is not active by default. This
can be useful for tracking down problems with syncs in combination with
the -v flag. See the Logging section for more info.

Note that if you are using the logrotate program to manage rclone’s
logs, then you should use the copytruncate option as rclone doesn’t have
a signal to rotate logs.

–log-format LIST

Comma separated list of log format options. date, time, microseconds,
longfile, shortfile, UTC. The default is “date,time”.

–log-level LEVEL

This sets the log level for rclone. The default log level is NOTICE.

DEBUG is equivalent to -vv. It outputs lots of debug info - useful for
bug reports and really finding out what rclone is doing.

INFO is equivalent to -v. It outputs information about each transfer and
prints stats once a minute by default.

NOTICE is the default log level if no logging flags are supplied. It
outputs very little when things are working normally. It outputs
warnings and significant events.

ERROR is equivalent to -q. It only outputs error messages.

–low-level-retries NUMBER

This controls the number of low level retries rclone does.

A low level retry is used to retry a failing operation - typically one
HTTP request. This might be uploading a chunk of a big file for example.
You will see low level retries in the log with the -v flag.

This shouldn’t need to be changed from the default in normal operations.
However, if you get a lot of low level retries you may wish to reduce
the value so rclone moves on to a high level retry (see the --retries
flag) quicker.

Disable low level retries with --low-level-retries 1.

–max-backlog=N

This is the maximum allowable backlog of files in a sync/copy/move
queued for being checked or transferred.

This can be set arbitrarily large. It will only use memory when the
queue is in use. Note that it will use in the order of N kB of memory
when the backlog is in use.

Setting this large allows rclone to calculate how many files are pending
more accurately and give a more accurate estimated finish time.

Setting this small will make rclone more synchronous to the listings of
the remote which may be desirable.

–max-delete=N

This tells rclone not to delete more than N files. If that limit is
exceeded then a fatal error will be generated and rclone will stop the
operation in progress.

–max-depth=N

This modifies the recursion depth for all the commands except purge.

So if you do rclone --max-depth 1 ls remote:path you will see only the
files in the top level directory. Using --max-depth 2 means you will see
all the files in first two directory levels and so on.

For historical reasons the lsd command defaults to using a --max-depth
of 1 - you can override this with the command line flag.

You can use this command to disable recursion (with --max-depth 1).

Note that if you use this with sync and --delete-excluded the files not
recursed through are considered excluded and will be deleted on the
destination. Test first with --dry-run if you are not sure what will
happen.

–max-transfer=SIZE

Rclone will stop transferring when it has reached the size specified.
Defaults to off.

When the limit is reached all transfers will stop immediately.

Rclone will exit with exit code 8 if the transfer limit is reached.

–modify-window=TIME

When checking whether a file has been modified, this is the maximum
allowed time difference that a file can have and still be considered
equivalent.

The default is 1ns unless this is overridden by a remote. For example OS
X only stores modification times to the nearest second so if you are
reading and writing to an OS X filing system this will be 1s by default.

This command line flag allows you to override that computed default.

–multi-thread-cutoff=SIZE

When downloading files to the local backend above this size, rclone will
use multiple threads to download the file. (default 250M)

Rclone preallocates the file (using fallocate(FALLOC_FL_KEEP_SIZE) on
unix or NTSetInformationFile on Windows both of which takes no time)
then each thread writes directly into the file at the correct place.
This means that rclone won’t create fragmented or sparse files and there
won’t be any assembly time at the end of the transfer.

The number of threads used to dowload is controlled by
--multi-thread-streams.

Use -vv if you wish to see info about the threads.

This will work with the sync/copy/move commands and friends
copyto/moveto. Multi thread downloads will be used with rclone mount and
rclone serve if --vfs-cache-mode is set to writes or above.

NB that this ONLY works for a local destination but will work with any
source.

–multi-thread-streams=N

When using multi thread downloads (see above --multi-thread-cutoff) this
sets the maximum number of streams to use. Set to 0 to disable multi
thread downloads. (Default 4)

Exactly how many streams rclone uses for the download depends on the
size of the file. To calculate the number of download streams Rclone
divides the size of the file by the --multi-thread-cutoff and rounds up,
up to the maximum set with --multi-thread-streams.

So if --multi-thread-cutoff 250MB and --multi-thread-streams 4 are in
effect (the defaults):

-   0MB.250MB files will be downloaded with 1 stream
-   250MB..500MB files will be downloaded with 2 streams
-   500MB..750MB files will be downloaded with 3 streams
-   750MB+ files will be downloaded with 4 streams

–no-gzip-encoding

Don’t set Accept-Encoding: gzip. This means that rclone won’t ask the
server for compressed files automatically. Useful if you’ve set the
server to return files with Content-Encoding: gzip but you uploaded
compressed files.

There is no need to set this in normal operation, and doing so will
decrease the network transfer efficiency of rclone.

–no-traverse

The --no-traverse flag controls whether the destination file system is
traversed when using the copy or move commands. --no-traverse is not
compatible with sync and will be ignored if you supply it with sync.

If you are only copying a small number of files (or are filtering most
of the files) and/or have a large number of files on the destination
then --no-traverse will stop rclone listing the destination and save
time.

However, if you are copying a large number of files, especially if you
are doing a copy where lots of the files under consideration haven’t
changed and won’t need copying then you shouldn’t use --no-traverse.

See rclone copy for an example of how to use it.

–no-update-modtime

When using this flag, rclone won’t update modification times of remote
files if they are incorrect as it would normally.

This can be used if the remote is being synced with another tool also
(eg the Google Drive client).

-P, –progress

This flag makes rclone update the stats in a static block in the
terminal providing a realtime overview of the transfer.

Any log messages will scroll above the static block. Log messages will
push the static block down to the bottom of the terminal where it will
stay.

Normally this is updated every 500mS but this period can be overridden
with the --stats flag.

This can be used with the --stats-one-line flag for a simpler display.

Note: On Windows untilthis bug is fixed all non-ASCII characters will be
replaced with . when --progress is in use.

-q, –quiet

Normally rclone outputs stats and a completion message. If you set this
flag it will make as little output as possible.

–retries int

Retry the entire sync if it fails this many times it fails (default 3).

Some remotes can be unreliable and a few retries help pick up the files
which didn’t get transferred because of errors.

Disable retries with --retries 1.

–retries-sleep=TIME

This sets the interval between each retry specified by --retries

The default is 0. Use 0 to disable.

–size-only

Normally rclone will look at modification time and size of files to see
if they are equal. If you set this flag then rclone will check only the
size.

This can be useful transferring files from Dropbox which have been
modified by the desktop sync client which doesn’t set checksums of
modification times in the same way as rclone.

–stats=TIME

Commands which transfer data (sync, copy, copyto, move, moveto) will
print data transfer stats at regular intervals to show their progress.

This sets the interval.

The default is 1m. Use 0 to disable.

If you set the stats interval then all commands can show stats. This can
be useful when running other commands, check or mount for example.

Stats are logged at INFO level by default which means they won’t show at
default log level NOTICE. Use --stats-log-level NOTICE or -v to make
them show. See the Logging section for more info on log levels.

Note that on macOS you can send a SIGINFO (which is normally ctrl-T in
the terminal) to make the stats print immediately.

–stats-file-name-length integer

By default, the --stats output will truncate file names and paths longer
than 40 characters. This is equivalent to providing
--stats-file-name-length 40. Use --stats-file-name-length 0 to disable
any truncation of file names printed by stats.

–stats-log-level string

Log level to show --stats output at. This can be DEBUG, INFO, NOTICE, or
ERROR. The default is INFO. This means at the default level of logging
which is NOTICE the stats won’t show - if you want them to then use
--stats-log-level NOTICE. See the Logging section for more info on log
levels.

–stats-one-line

When this is specified, rclone condenses the stats into a single line
showing the most important stats only.

–stats-one-line-date

When this is specified, rclone enables the single-line stats and
prepends the display with a date string. The default is
2006/01/02 15:04:05 -

–stats-one-line-date-format

When this is specified, rclone enables the single-line stats and
prepends the display with a user-supplied date string. The date string
MUST be enclosed in quotes. Follow golang specs for date formatting
syntax.

–stats-unit=bits|bytes

By default, data transfer rates will be printed in bytes/second.

This option allows the data rate to be printed in bits/second.

Data transfer volume will still be reported in bytes.

The rate is reported as a binary unit, not SI unit. So 1 Mbit/s equals
1,048,576 bits/s and not 1,000,000 bits/s.

The default is bytes.

–suffix=SUFFIX

This is for use with --backup-dir only. If this isn’t set then
--backup-dir will move files with their original name. If it is set then
the files will have SUFFIX added on to them.

See --backup-dir for more info.

–suffix-keep-extension

When using --suffix, setting this causes rclone put the SUFFIX before
the extension of the files that it backs up rather than after.

So let’s say we had --suffix -2019-01-01, without the flag file.txt
would be backed up to file.txt-2019-01-01 and with the flag it would be
backed up to file-2019-01-01.txt. This can be helpful to make sure the
suffixed files can still be opened.

–syslog

On capable OSes (not Windows or Plan9) send all log output to syslog.

This can be useful for running rclone in a script or rclone mount.

–syslog-facility string

If using --syslog this sets the syslog facility (eg KERN, USER). See
man syslog for a list of possible facilities. The default facility is
DAEMON.

–tpslimit float

Limit HTTP transactions per second to this. Default is 0 which is used
to mean unlimited transactions per second.

For example to limit rclone to 10 HTTP transactions per second use
--tpslimit 10, or to 1 transaction every 2 seconds use --tpslimit 0.5.

Use this when the number of transactions per second from rclone is
causing a problem with the cloud storage provider (eg getting you banned
or rate limited).

This can be very useful for rclone mount to control the behaviour of
applications using it.

See also --tpslimit-burst.

–tpslimit-burst int

Max burst of transactions for --tpslimit. (default 1)

Normally --tpslimit will do exactly the number of transaction per second
specified. However if you supply --tps-burst then rclone can save up
some transactions from when it was idle giving a burst of up to the
parameter supplied.

For example if you provide --tpslimit-burst 10 then if rclone has been
idle for more than 10*--tpslimit then it can do 10 transactions very
quickly before they are limited again.

This may be used to increase performance of --tpslimit without changing
the long term average number of transactions per second.

–track-renames

By default, rclone doesn’t keep track of renamed files, so if you rename
a file locally then sync it to a remote, rclone will delete the old file
on the remote and upload a new copy.

If you use this flag, and the remote supports server side copy or server
side move, and the source and destination have a compatible hash, then
this will track renames during sync operations and perform renaming
server-side.

Files will be matched by size and hash - if both match then a rename
will be considered.

If the destination does not support server-side copy or move, rclone
will fall back to the default behaviour and log an error level message
to the console. Note: Encrypted destinations are not supported by
--track-renames.

Note that --track-renames is incompatible with --no-traverse and that it
uses extra memory to keep track of all the rename candidates.

Note also that --track-renames is incompatible with --delete-before and
will select --delete-after instead of --delete-during.

–delete-(before,during,after)

This option allows you to specify when files on your destination are
deleted when you sync folders.

Specifying the value --delete-before will delete all files present on
the destination, but not on the source _before_ starting the transfer of
any new or updated files. This uses two passes through the file systems,
one for the deletions and one for the copies.

Specifying --delete-during will delete files while checking and
uploading files. This is the fastest option and uses the least memory.

Specifying --delete-after (the default value) will delay deletion of
files until all new/updated files have been successfully transferred.
The files to be deleted are collected in the copy pass then deleted
after the copy pass has completed successfully. The files to be deleted
are held in memory so this mode may use more memory. This is the safest
mode as it will only delete files if there have been no errors
subsequent to that. If there have been errors before the deletions start
then you will get the message
not deleting files as there were IO errors.

–fast-list

When doing anything which involves a directory listing (eg sync, copy,
ls - in fact nearly every command), rclone normally lists a directory
and processes it before using more directory lists to process any
subdirectories. This can be parallelised and works very quickly using
the least amount of memory.

However, some remotes have a way of listing all files beneath a
directory in one (or a small number) of transactions. These tend to be
the bucket based remotes (eg S3, B2, GCS, Swift, Hubic).

If you use the --fast-list flag then rclone will use this method for
listing directories. This will have the following consequences for the
listing:

-   It WILL use fewer transactions (important if you pay for them)
-   It WILL use more memory. Rclone has to load the whole listing into
    memory.
-   It _may_ be faster because it uses fewer transactions
-   It _may_ be slower because it can’t be parallelized

rclone should always give identical results with and without
--fast-list.

If you pay for transactions and can fit your entire sync listing into
memory then --fast-list is recommended. If you have a very big sync to
do then don’t use --fast-list otherwise you will run out of memory.

If you use --fast-list on a remote which doesn’t support it, then rclone
will just ignore it.

–timeout=TIME

This sets the IO idle timeout. If a transfer has started but then
becomes idle for this long it is considered broken and disconnected.

The default is 5m. Set to 0 to disable.

–transfers=N

The number of file transfers to run in parallel. It can sometimes be
useful to set this to a smaller number if the remote is giving a lot of
timeouts or bigger if you have lots of bandwidth and a fast remote.

The default is to run 4 file transfers in parallel.

-u, –update

This forces rclone to skip any files which exist on the destination and
have a modified time that is newer than the source file.

If an existing destination file has a modification time equal (within
the computed modify window precision) to the source file’s, it will be
updated if the sizes are different.

On remotes which don’t support mod time directly the time checked will
be the uploaded time. This means that if uploading to one of these
remotes, rclone will skip any files which exist on the destination and
have an uploaded time that is newer than the modification time of the
source file.

This can be useful when transferring to a remote which doesn’t support
mod times directly as it is more accurate than a --size-only check and
faster than using --checksum.

–use-mmap

If this flag is set then rclone will use anonymous memory allocated by
mmap on Unix based platforms and VirtualAlloc on Windows for its
transfer buffers (size controlled by --buffer-size). Memory allocated
like this does not go on the Go heap and can be returned to the OS
immediately when it is finished with.

If this flag is not set then rclone will allocate and free the buffers
using the Go memory allocator which may use more memory as memory pages
are returned less aggressively to the OS.

It is possible this does not work well on all platforms so it is
disabled by default; in the future it may be enabled by default.

–use-server-modtime

Some object-store backends (e.g, Swift, S3) do not preserve file
modification times (modtime). On these backends, rclone stores the
original modtime as additional metadata on the object. By default it
will make an API call to retrieve the metadata when the modtime is
needed by an operation.

Use this flag to disable the extra API call and rely instead on the
server’s modified time. In cases such as a local to remote sync, knowing
the local file is newer than the time it was last uploaded to the remote
is sufficient. In those cases, this flag can speed up the process and
reduce the number of API calls necessary.

-v, -vv, –verbose

With -v rclone will tell you about each file that is transferred and a
small number of significant events.

With -vv rclone will become very verbose telling you about every file it
considers and transfers. Please send bug reports with a log with this
setting.

-V, –version

Prints the version number


SSL/TLS options

The outoing SSL/TLS connections rclone makes can be controlled with
these options. For example this can be very useful with the HTTP or
WebDAV backends. Rclone HTTP servers have their own set of configuration
for SSL/TLS which you can find in their documentation.

–ca-cert string

This loads the PEM encoded certificate authority certificate and uses it
to verify the certificates of the servers rclone connects to.

If you have generated certificates signed with a local CA then you will
need this flag to connect to servers using those certificates.

–client-cert string

This loads the PEM encoded client side certificate.

This is used for mutual TLS authentication.

The --client-key flag is required too when using this.

–client-key string

This loads the PEM encoded client side private key used for mutual TLS
authentication. Used in conjunction with --client-cert.

–no-check-certificate=true/false

--no-check-certificate controls whether a client verifies the server’s
certificate chain and host name. If --no-check-certificate is true, TLS
accepts any certificate presented by the server and any host name in
that certificate. In this mode, TLS is susceptible to man-in-the-middle
attacks.

This option defaults to false.

THIS SHOULD BE USED ONLY FOR TESTING.


Configuration Encryption

Your configuration file contains information for logging in to your
cloud services. This means that you should keep your .rclone.conf file
in a secure location.

If you are in an environment where that isn’t possible, you can add a
password to your configuration. This means that you will have to enter
the password every time you start rclone.

To add a password to your rclone configuration, execute rclone config.

    >rclone config
    Current remotes:

    e) Edit existing remote
    n) New remote
    d) Delete remote
    s) Set configuration password
    q) Quit config
    e/n/d/s/q>

Go into s, Set configuration password:

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

Your configuration is now encrypted, and every time you start rclone you
will now be asked for the password. In the same menu, you can change the
password or completely remove encryption from your configuration.

There is no way to recover the configuration if you lose your password.

rclone uses nacl secretbox which in turn uses XSalsa20 and Poly1305 to
encrypt and authenticate your configuration with secret-key
cryptography. The password is SHA-256 hashed, which produces the key for
secretbox. The hashed password is not stored.

While this provides very good security, we do not recommend storing your
encrypted rclone configuration in public if it contains sensitive
information, maybe except if you use a very strong password.

If it is safe in your environment, you can set the RCLONE_CONFIG_PASS
environment variable to contain your password, in which case it will be
used for decrypting the configuration.

You can set this for a session from a script. For unix like systems save
this to a file called set-rclone-password:

    #!/bin/echo Source this file don't run it

    read -s RCLONE_CONFIG_PASS
    export RCLONE_CONFIG_PASS

Then source the file when you want to use it. From the shell you would
do source set-rclone-password. It will then ask you for the password and
set it in the environment variable.

If you are running rclone inside a script, you might want to disable
password prompts. To do that, pass the parameter --ask-password=false to
rclone. This will make rclone fail instead of asking for a password if
RCLONE_CONFIG_PASS doesn’t contain a valid password.


Developer options

These options are useful when developing or debugging rclone. There are
also some more remote specific options which aren’t documented here
which are used for testing. These start with remote name eg
--drive-test-option - see the docs for the remote in question.

–cpuprofile=FILE

Write CPU profile to file. This can be analysed with go tool pprof.

–dump flag,flag,flag

The --dump flag takes a comma separated list of flags to dump info
about. These are:

–dump headers

Dump HTTP headers with Authorization: lines removed. May still contain
sensitive info. Can be very verbose. Useful for debugging only.

Use --dump auth if you do want the Authorization: headers.

–dump bodies

Dump HTTP headers and bodies - may contain sensitive info. Can be very
verbose. Useful for debugging only.

Note that the bodies are buffered in memory so don’t use this for
enormous files.

–dump requests

Like --dump bodies but dumps the request bodies and the response
headers. Useful for debugging download problems.

–dump responses

Like --dump bodies but dumps the response bodies and the request
headers. Useful for debugging upload problems.

–dump auth

Dump HTTP headers - will contain sensitive info such as Authorization:
headers - use --dump headers to dump without Authorization: headers. Can
be very verbose. Useful for debugging only.

–dump filters

Dump the filters to the output. Useful to see exactly what include and
exclude options are filtering on.

–dump goroutines

This dumps a list of the running go-routines at the end of the command
to standard output.

–dump openfiles

This dumps a list of the open files at the end of the command. It uses
the lsof command to do that so you’ll need that installed to use it.

–memprofile=FILE

Write memory profile to file. This can be analysed with go tool pprof.


Filtering

For the filtering options

-   --delete-excluded
-   --filter
-   --filter-from
-   --exclude
-   --exclude-from
-   --include
-   --include-from
-   --files-from
-   --min-size
-   --max-size
-   --min-age
-   --max-age
-   --dump filters

See the filtering section.


Remote control

For the remote control options and for instructions on how to remote
control rclone

-   --rc
-   and anything starting with --rc-

See the remote control section.


Logging

rclone has 4 levels of logging, ERROR, NOTICE, INFO and DEBUG.

By default, rclone logs to standard error. This means you can redirect
standard error and still see the normal output of rclone commands (eg
rclone ls).

By default, rclone will produce Error and Notice level messages.

If you use the -q flag, rclone will only produce Error messages.

If you use the -v flag, rclone will produce Error, Notice and Info
messages.

If you use the -vv flag, rclone will produce Error, Notice, Info and
Debug messages.

You can also control the log levels with the --log-level flag.

If you use the --log-file=FILE option, rclone will redirect Error, Info
and Debug messages along with standard error to FILE.

If you use the --syslog flag then rclone will log to syslog and the
--syslog-facility control which facility it uses.

Rclone prefixes all log messages with their level in capitals, eg INFO
which makes it easy to grep the log file for different kinds of
information.


Exit Code

If any errors occur during the command execution, rclone will exit with
a non-zero exit code. This allows scripts to detect when rclone
operations have failed.

During the startup phase, rclone will exit immediately if an error is
detected in the configuration. There will always be a log message
immediately before exiting.

When rclone is running it will accumulate errors as it goes along, and
only exit with a non-zero exit code if (after retries) there were still
failed transfers. For every error counted there will be a high priority
log message (visible with -q) showing the message and which file caused
the problem. A high priority message is also shown when starting a retry
so the user can see that any previous error messages may not be valid
after the retry. If rclone has done a retry it will log a high priority
message if the retry was successful.

List of exit codes

-   0 - success
-   1 - Syntax or usage error
-   2 - Error not otherwise categorised
-   3 - Directory not found
-   4 - File not found
-   5 - Temporary error (one that more retries might fix) (Retry errors)
-   6 - Less serious errors (like 461 errors from dropbox) (NoRetry
    errors)
-   7 - Fatal error (one that more retries won’t fix, like account
    suspended) (Fatal errors)
-   8 - Transfer exceeded - limit set by –max-transfer reached


Environment Variables

Rclone can be configured entirely using environment variables. These can
be used to set defaults for options or config file entries.

Options

Every option in rclone can have its default set by environment variable.

To find the name of the environment variable, first, take the long
option name, strip the leading --, change - to _, make upper case and
prepend RCLONE_.

For example, to always set --stats 5s, set the environment variable
RCLONE_STATS=5s. If you set stats on the command line this will override
the environment variable setting.

Or to always use the trash in drive --drive-use-trash, set
RCLONE_DRIVE_USE_TRASH=true.

The same parser is used for the options and the environment variables so
they take exactly the same form.

Config file

You can set defaults for values in the config file on an individual
remote basis. If you want to use this feature, you will need to discover
the name of the config items that you want. The easiest way is to run
through rclone config by hand, then look in the config file to see what
the values are (the config file can be found by looking at the help for
--config in rclone help).

To find the name of the environment variable, you need to set, take
RCLONE_CONFIG_ + name of remote + _ + name of config file option and
make it all uppercase.

For example, to configure an S3 remote named mys3: without a config file
(using unix ways of setting environment variables):

    $ export RCLONE_CONFIG_MYS3_TYPE=s3
    $ export RCLONE_CONFIG_MYS3_ACCESS_KEY_ID=XXX
    $ export RCLONE_CONFIG_MYS3_SECRET_ACCESS_KEY=XXX
    $ rclone lsd MYS3:
              -1 2016-09-21 12:54:21        -1 my-bucket
    $ rclone listremotes | grep mys3
    mys3:

Note that if you want to create a remote using environment variables you
must create the ..._TYPE variable as above.

Other environment variables

-   RCLONE_CONFIG_PASS` set to contain your config file password (see
    Configuration Encryption section)
-   HTTP_PROXY, HTTPS_PROXY and NO_PROXY (or the lowercase versions
    thereof).
    -   HTTPS_PROXY takes precedence over HTTP_PROXY for https requests.
    -   The environment values may be either a complete URL or a
        “host[:port]” for, in which case the “http” scheme is assumed.



CONFIGURING RCLONE ON A REMOTE / HEADLESS MACHINE


Some of the configurations (those involving oauth2) require an Internet
connected web browser.

If you are trying to set rclone up on a remote or headless box with no
browser available on it (eg a NAS or a server in a datacenter) then you
will need to use an alternative means of configuration. There are two
ways of doing it, described below.


Configuring using rclone authorize

On the headless box

    ...
    Remote config
    Use auto config?
     * Say Y if not sure
     * Say N if you are working on a remote or headless machine
    y) Yes
    n) No
    y/n> n
    For this to work, you will need rclone available on a machine that has a web browser available.
    Execute the following on your machine:
        rclone authorize "amazon cloud drive"
    Then paste the result below:
    result>

Then on your main desktop machine

    rclone authorize "amazon cloud drive"
    If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
    Log in and authorize rclone for access
    Waiting for code...
    Got code
    Paste the following into your remote machine --->
    SECRET_TOKEN
    <---End paste

Then back to the headless box, paste in the code

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


Configuring by copying the config file

Rclone stores all of its config in a single configuration file. This can
easily be copied to configure a remote rclone.

So first configure rclone on your desktop machine

    rclone config

to set up the config file.

Find the config file by running rclone config file, for example

    $ rclone config file
    Configuration file is stored at:
    /home/user/.rclone.conf

Now transfer it to the remote box (scp, cut paste, ftp, sftp etc) and
place it in the correct place (use rclone config file on the remote box
to find out where).



FILTERING, INCLUDES AND EXCLUDES


Rclone has a sophisticated set of include and exclude rules. Some of
these are based on patterns and some on other things like file size.

The filters are applied for the copy, sync, move, ls, lsl, md5sum,
sha1sum, size, delete and check operations. Note that purge does not
obey the filters.

Each path as it passes through rclone is matched against the include and
exclude rules like --include, --exclude, --include-from, --exclude-from,
--filter, or --filter-from. The simplest way to try them out is using
the ls command, or --dry-run together with -v.


Patterns

The patterns used to match files for inclusion or exclusion are based on
“file globs” as used by the unix shell.

If the pattern starts with a / then it only matches at the top level of
the directory tree, RELATIVE TO THE ROOT OF THE REMOTE (not necessarily
the root of the local drive). If it doesn’t start with / then it is
matched starting at the END OF THE PATH, but it will only match a
complete path element:

    file.jpg  - matches "file.jpg"
              - matches "directory/file.jpg"
              - doesn't match "afile.jpg"
              - doesn't match "directory/afile.jpg"
    /file.jpg - matches "file.jpg" in the root directory of the remote
              - doesn't match "afile.jpg"
              - doesn't match "directory/file.jpg"

IMPORTANT Note that you must use / in patterns and not \ even if running
on Windows.

A * matches anything but not a /.

    *.jpg  - matches "file.jpg"
           - matches "directory/file.jpg"
           - doesn't match "file.jpg/something"

Use ** to match anything, including slashes (/).

    dir/** - matches "dir/file.jpg"
           - matches "dir/dir1/dir2/file.jpg"
           - doesn't match "directory/file.jpg"
           - doesn't match "adir/file.jpg"

A ? matches any character except a slash /.

    l?ss  - matches "less"
          - matches "lass"
          - doesn't match "floss"

A [ and ] together make a character class, such as [a-z] or [aeiou] or
[[:alpha:]]. See the go regexp docs for more info on these.

    h[ae]llo - matches "hello"
             - matches "hallo"
             - doesn't match "hullo"

A { and } define a choice between elements. It should contain a comma
separated list of patterns, any of which might match. These patterns can
contain wildcards.

    {one,two}_potato - matches "one_potato"
                     - matches "two_potato"
                     - doesn't match "three_potato"
                     - doesn't match "_potato"

Special characters can be escaped with a \ before them.

    \*.jpg       - matches "*.jpg"
    \\.jpg       - matches "\.jpg"
    \[one\].jpg  - matches "[one].jpg"

Patterns are case sensitive unless the --ignore-case flag is used.

Without --ignore-case (default)

    potato - matches "potato"
           - doesn't match "POTATO"

With --ignore-case

    potato - matches "potato"
           - matches "POTATO"

Note also that rclone filter globs can only be used in one of the filter
command line flags, not in the specification of the remote, so
rclone copy "remote:dir*.jpg" /path/to/dir won’t work - what is required
is rclone --include "*.jpg" copy remote:dir /path/to/dir

Directories

Rclone keeps track of directories that could match any file patterns.

Eg if you add the include rule

    /a/*.jpg

Rclone will synthesize the directory include rule

    /a/

If you put any rules which end in / then it will only match directories.

Directory matches are ONLY used to optimise directory access patterns -
you must still match the files that you want to match. Directory matches
won’t optimise anything on bucket based remotes (eg s3, swift, google
compute storage, b2) which don’t have a concept of directory.

Differences between rsync and rclone patterns

Rclone implements bash style {a,b,c} glob matching which rsync doesn’t.

Rclone always does a wildcard match so \ must always escape a \.


How the rules are used

Rclone maintains a combined list of include rules and exclude rules.

Each file is matched in order, starting from the top, against the rule
in the list until it finds a match. The file is then included or
excluded according to the rule type.

If the matcher fails to find a match after testing against all the
entries in the list then the path is included.

For example given the following rules, + being include, - being exclude,

    - secret*.jpg
    + *.jpg
    + *.png
    + file2.avi
    - *

This would include

-   file1.jpg
-   file3.png
-   file2.avi

This would exclude

-   secret17.jpg
-   non *.jpg and *.png

A similar process is done on directory entries before recursing into
them. This only works on remotes which have a concept of directory (Eg
local, google drive, onedrive, amazon drive) and not on bucket based
remotes (eg s3, swift, google compute storage, b2).


Adding filtering rules

Filtering rules are added with the following command line flags.

Repeating options

You can repeat the following options to add more than one rule of that
type.

-   --include
-   --include-from
-   --exclude
-   --exclude-from
-   --filter
-   --filter-from

IMPORTANT You should not use --include* together with --exclude*. It may
produce different results than you expected. In that case try to use:
--filter*.

Note that all the options of the same type are processed together in the
order above, regardless of what order they were placed on the command
line.

So all --include options are processed first in the order they appeared
on the command line, then all --include-from options etc.

To mix up the order includes and excludes, the --filter flag can be
used.

--exclude - Exclude files matching pattern

Add a single exclude rule with --exclude.

This flag can be repeated. See above for the order the flags are
processed in.

Eg --exclude *.bak to exclude all bak files from the sync.

--exclude-from - Read exclude patterns from file

Add exclude rules from a file.

This flag can be repeated. See above for the order the flags are
processed in.

Prepare a file like this exclude-file.txt

    # a sample exclude rule file
    *.bak
    file2.jpg

Then use as --exclude-from exclude-file.txt. This will sync all files
except those ending in bak and file2.jpg.

This is useful if you have a lot of rules.

--include - Include files matching pattern

Add a single include rule with --include.

This flag can be repeated. See above for the order the flags are
processed in.

Eg --include *.{png,jpg} to include all png and jpg files in the backup
and no others.

This adds an implicit --exclude * at the very end of the filter list.
This means you can mix --include and --include-from with the other
filters (eg --exclude) but you must include all the files you want in
the include statement. If this doesn’t provide enough flexibility then
you must use --filter-from.

--include-from - Read include patterns from file

Add include rules from a file.

This flag can be repeated. See above for the order the flags are
processed in.

Prepare a file like this include-file.txt

    # a sample include rule file
    *.jpg
    *.png
    file2.avi

Then use as --include-from include-file.txt. This will sync all jpg, png
files and file2.avi.

This is useful if you have a lot of rules.

This adds an implicit --exclude * at the very end of the filter list.
This means you can mix --include and --include-from with the other
filters (eg --exclude) but you must include all the files you want in
the include statement. If this doesn’t provide enough flexibility then
you must use --filter-from.

--filter - Add a file-filtering rule

This can be used to add a single include or exclude rule. Include rules
start with + and exclude rules start with -. A special rule called ! can
be used to clear the existing rules.

This flag can be repeated. See above for the order the flags are
processed in.

Eg --filter "- *.bak" to exclude all bak files from the sync.

--filter-from - Read filtering patterns from a file

Add include/exclude rules from a file.

This flag can be repeated. See above for the order the flags are
processed in.

Prepare a file like this filter-file.txt

    # a sample filter rule file
    - secret*.jpg
    + *.jpg
    + *.png
    + file2.avi
    - /dir/Trash/**
    + /dir/**
    # exclude everything else
    - *

Then use as --filter-from filter-file.txt. The rules are processed in
the order that they are defined.

This example will include all jpg and png files, exclude any files
matching secret*.jpg and include file2.avi. It will also include
everything in the directory dir at the root of the sync, except
dir/Trash which it will exclude. Everything else will be excluded from
the sync.

--files-from - Read list of source-file names

This reads a list of file names from the file passed in and ONLY these
files are transferred. The FILTERING RULES ARE IGNORED completely if you
use this option.

Rclone will traverse the file system if you use --files-from,
effectively using the files in --files-from as a set of filters. Rclone
will not error if any of the files are missing.

If you use --no-traverse as well as --files-from then rclone will not
traverse the destination file system, it will find each file
individually using approximately 1 API call. This can be more efficient
for small lists of files.

This option can be repeated to read from more than one file. These are
read in the order that they are placed on the command line.

Paths within the --files-from file will be interpreted as starting with
the root specified in the command. Leading / characters are ignored.

For example, suppose you had files-from.txt with this content:

    # comment
    file1.jpg
    subdir/file2.jpg

You could then use it like this:

    rclone copy --files-from files-from.txt /home/me/pics remote:pics

This will transfer these files only (if they exist)

    /home/me/pics/file1.jpg        → remote:pics/file1.jpg
    /home/me/pics/subdir/file2.jpg → remote:pics/subdirfile1.jpg

To take a more complicated example, let’s say you had a few files you
want to back up regularly with these absolute paths:

    /home/user1/important
    /home/user1/dir/file
    /home/user2/stuff

To copy these you’d find a common subdirectory - in this case /home and
put the remaining files in files-from.txt with or without leading /, eg

    user1/important
    user1/dir/file
    user2/stuff

You could then copy these to a remote like this

    rclone copy --files-from files-from.txt /home remote:backup

The 3 files will arrive in remote:backup with the paths as in the
files-from.txt like this:

    /home/user1/important → remote:backup/user1/important
    /home/user1/dir/file  → remote:backup/user1/dir/file
    /home/user2/stuff     → remote:backup/stuff

You could of course choose / as the root too in which case your
files-from.txt might look like this.

    /home/user1/important
    /home/user1/dir/file
    /home/user2/stuff

And you would transfer it like this

    rclone copy --files-from files-from.txt / remote:backup

In this case there will be an extra home directory on the remote:

    /home/user1/important → remote:home/backup/user1/important
    /home/user1/dir/file  → remote:home/backup/user1/dir/file
    /home/user2/stuff     → remote:home/backup/stuff

--min-size - Don’t transfer any file smaller than this

This option controls the minimum size file which will be transferred.
This defaults to kBytes but a suffix of k, M, or G can be used.

For example --min-size 50k means no files smaller than 50kByte will be
transferred.

--max-size - Don’t transfer any file larger than this

This option controls the maximum size file which will be transferred.
This defaults to kBytes but a suffix of k, M, or G can be used.

For example --max-size 1G means no files larger than 1GByte will be
transferred.

--max-age - Don’t transfer any file older than this

This option controls the maximum age of files to transfer. Give in
seconds or with a suffix of:

-   ms - Milliseconds
-   s - Seconds
-   m - Minutes
-   h - Hours
-   d - Days
-   w - Weeks
-   M - Months
-   y - Years

For example --max-age 2d means no files older than 2 days will be
transferred.

--min-age - Don’t transfer any file younger than this

This option controls the minimum age of files to transfer. Give in
seconds or with a suffix (see --max-age for list of suffixes)

For example --min-age 2d means no files younger than 2 days will be
transferred.

--delete-excluded - Delete files on dest excluded from sync

IMPORTANT this flag is dangerous - use with --dry-run and -v first.

When doing rclone sync this will delete any files which are excluded
from the sync on the destination.

If for example you did a sync from A to B without the --min-size 50k
flag

    rclone sync A: B:

Then you repeated it like this with the --delete-excluded

    rclone --min-size 50k --delete-excluded sync A: B:

This would delete all files on B which are less than 50 kBytes as these
are now excluded from the sync.

Always test first with --dry-run and -v before using this flag.

--dump filters - dump the filters to the output

This dumps the defined filters to the output as regular expressions.

Useful for debugging.

--ignore-case - make searches case insensitive

Normally filter patterns are case sensitive. If this flag is supplied
then filter patterns become case insensitive.

Normally a --include "file.txt" will not match a file called FILE.txt.
However if you use the --ignore-case flag then --include "file.txt" this
will match a file called FILE.txt.


Quoting shell metacharacters

The examples above may not work verbatim in your shell as they have
shell metacharacters in them (eg *), and may require quoting.

Eg linux, OSX

-   --include \*.jpg
-   --include '*.jpg'
-   --include='*.jpg'

In Windows the expansion is done by the command not the shell so this
should work fine

-   --include *.jpg


Exclude directory based on a file

It is possible to exclude a directory based on a file, which is present
in this directory. Filename should be specified using the
--exclude-if-present flag. This flag has a priority over the other
filtering flags.

Imagine, you have the following directory structure:

    dir1/file1
    dir1/dir2/file2
    dir1/dir2/dir3/file3
    dir1/dir2/dir3/.ignore

You can exclude dir3 from sync by running the following command:

    rclone sync --exclude-if-present .ignore dir1 remote:backup

Currently only one filename is supported, i.e. --exclude-if-present
should not be used multiple times.



REMOTE CONTROLLING RCLONE


If rclone is run with the --rc flag then it starts an http server which
can be used to remote control rclone.

If you just want to run a remote control then see the rcd command.

NB this is experimental and everything here is subject to change!


Supported parameters

–rc

Flag to start the http server listen on remote requests

–rc-addr=IP

IPaddress:Port or :Port to bind server to. (default “localhost:5572”)

–rc-cert=KEY

SSL PEM key (concatenation of certificate and CA certificate)

–rc-client-ca=PATH

Client certificate authority to verify clients with

–rc-htpasswd=PATH

htpasswd file - if not provided no authentication is done

–rc-key=PATH

SSL PEM Private key

–rc-max-header-bytes=VALUE

Maximum size of request header (default 4096)

–rc-user=VALUE

User name for authentication.

–rc-pass=VALUE

Password for authentication.

–rc-realm=VALUE

Realm for authentication (default “rclone”)

–rc-server-read-timeout=DURATION

Timeout for server reading data (default 1h0m0s)

–rc-server-write-timeout=DURATION

Timeout for server writing data (default 1h0m0s)

–rc-serve

Enable the serving of remote objects via the HTTP interface. This means
objects will be accessible at http://127.0.0.1:5572/ by default, so you
can browse to http://127.0.0.1:5572/ or http://127.0.0.1:5572/* to see a
listing of the remotes. Objects may be requested from remotes using this
syntax http://127.0.0.1:5572/[remote:path]/path/to/object

Default Off.

–rc-files /path/to/directory

Path to local files to serve on the HTTP server.

If this is set then rclone will serve the files in that directory. It
will also open the root in the web browser if specified. This is for
implementing browser based GUIs for rclone functions.

If --rc-user or --rc-pass is set then the URL that is opened will have
the authorization in the URL in the http://user:pass@localhost/ style.

Default Off.

–rc-job-expire-duration=DURATION

Expire finished async jobs older than DURATION (default 60s).

–rc-job-expire-interval=DURATION

Interval duration to check for expired async jobs (default 10s).

–rc-no-auth

By default rclone will require authorisation to have been set up on the
rc interface in order to use any methods which access any rclone
remotes. Eg operations/list is denied as it involved creating a remote
as is sync/copy.

If this is set then no authorisation will be required on the server to
use these methods. The alternative is to use --rc-user and --rc-pass and
use these credentials in the request.

Default Off.


Accessing the remote control via the rclone rc command

Rclone itself implements the remote control protocol in its rclone rc
command.

You can use it like this

    $ rclone rc rc/noop param1=one param2=two
    {
        "param1": "one",
        "param2": "two"
    }

Run rclone rc on its own to see the help for the installed remote
control commands.

rclone rc also supports a --json flag which can be used to send more
complicated input parameters.

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


Special parameters

The rc interface supports some special parameters which apply to ALL
commands. These start with _ to show they are different.

Running asynchronous jobs with _async = true

If _async has a true value when supplied to an rc call then it will
return immediately with a job id and the task will be run in the
background. The job/status call can be used to get information of the
background job. The job can be queried for up to 1 minute after it has
finished.

It is recommended that potentially long running jobs, eg sync/sync,
sync/copy, sync/move, operations/purge are run with the _async flag to
avoid any potential problems with the HTTP request and response timing
out.

Starting a job with the _async flag:

    $ rclone rc --json '{ "p1": [1,"2",null,4], "p2": { "a":1, "b":2 }, "_async": true }' rc/noop
    {
        "jobid": 2
    }

Query the status to see if the job has finished. For more information on
the meaning of these return parameters see the job/status call.

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

job/list can be used to show the running or recently completed jobs

    $ rclone rc job/list
    {
        "jobids": [
            2
        ]
    }


Supported commands

cache/expire: Purge a remote from cache

Purge a remote from the cache backend. Supports either a directory or a
file. Params: - remote = path to remote (required) - withData =
true/false to delete cached data (chunks) as well (optional)

Eg

    rclone rc cache/expire remote=path/to/sub/folder/
    rclone rc cache/expire remote=/ withData=true

cache/fetch: Fetch file chunks

Ensure the specified file chunks are cached on disk.

The chunks= parameter specifies the file chunks to check. It takes a
comma separated list of array slice indices. The slice indices are
similar to Python slices: start[:end]

start is the 0 based chunk number from the beginning of the file to
fetch inclusive. end is 0 based chunk number from the beginning of the
file to fetch exclusive. Both values can be negative, in which case they
count from the back of the file. The value “-5:” represents the last 5
chunks of a file.

Some valid examples are: “:5,-5:” -> the first and last five chunks
“0,-2” -> the first and the second last chunk “0:10” -> the first ten
chunks

Any parameter with a key that starts with “file” can be used to specify
files to fetch, eg

    rclone rc cache/fetch chunks=0 file=hello file2=home/goodbye

File names will automatically be encrypted when the a crypt remote is
used on top of the cache.

cache/stats: Get cache stats

Show statistics for the cache remote.

config/create: create the config for a remote.

This takes the following parameters

-   name - name of remote
-   type - type of the new remote

See the config create command command for more information on the above.

Authentication is required for this call.

config/delete: Delete a remote in the config file.

Parameters: - name - name of remote to delete

See the config delete command command for more information on the above.

Authentication is required for this call.

config/dump: Dumps the config file.

Returns a JSON object: - key: value

Where keys are remote names and values are the config parameters.

See the config dump command command for more information on the above.

Authentication is required for this call.

config/get: Get a remote in the config file.

Parameters: - name - name of remote to get

See the config dump command command for more information on the above.

Authentication is required for this call.

config/listremotes: Lists the remotes in the config file.

Returns - remotes - array of remote names

See the listremotes command command for more information on the above.

Authentication is required for this call.

config/password: password the config for a remote.

This takes the following parameters

-   name - name of remote

See the config password command command for more information on the
above.

Authentication is required for this call.

config/providers: Shows how providers are configured in the config file.

Returns a JSON object: - providers - array of objects

See the config providers command command for more information on the
above.

Authentication is required for this call.

config/update: update the config for a remote.

This takes the following parameters

-   name - name of remote

See the config update command command for more information on the above.

Authentication is required for this call.

core/bwlimit: Set the bandwidth limit.

This sets the bandwidth limit to that passed in.

Eg

    rclone rc core/bwlimit rate=1M
    rclone rc core/bwlimit rate=off

The format of the parameter is exactly the same as passed to –bwlimit
except only one bandwidth may be specified.

core/gc: Runs a garbage collection.

This tells the go runtime to do a garbage collection run. It isn’t
necessary to call this normally, but it can be useful for debugging
memory problems.

core/memstats: Returns the memory statistics

This returns the memory statistics of the running program. What the
values mean are explained in the go docs:
https://golang.org/pkg/runtime/#MemStats

The most interesting values for most people are:

-   HeapAlloc: This is the amount of memory rclone is actually using
-   HeapSys: This is the amount of memory rclone has obtained from the
    OS
-   Sys: this is the total amount of memory requested from the OS
    -   It is virtual memory so may include unused memory

core/obscure: Obscures a string passed in.

Pass a clear string and rclone will obscure it for the config file: -
clear - string

Returns - obscured - string

core/pid: Return PID of current process

This returns PID of current process. Useful for stopping rclone process.

core/stats: Returns stats about current transfers.

This returns all available stats

    rclone rc core/stats

Returns the following values:

    {
        "speed": average speed in bytes/sec since start of the process,
        "bytes": total transferred bytes since the start of the process,
        "errors": number of errors,
        "fatalError": whether there has been at least one FatalError,
        "retryError": whether there has been at least one non-NoRetryError,
        "checks": number of checked files,
        "transfers": number of transferred files,
        "deletes" : number of deleted files,
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

Values for “transferring”, “checking” and “lastError” are only assigned
if data is available. The value for “eta” is null if an eta cannot be
determined.

core/version: Shows the current version of rclone and the go runtime.

This shows the current version of go and the go runtime - version -
rclone version, eg “v1.44” - decomposed - version number as [major,
minor, patch, subpatch] - note patch and subpatch will be 999 for a git
compiled version - isGit - boolean - true if this was compiled from the
git version - os - OS in use as according to Go - arch - cpu
architecture in use according to Go - goVersion - version of Go runtime
in use

job/list: Lists the IDs of the running jobs

Parameters - None

Results - jobids - array of integer job ids

job/status: Reads the status of the job ID

Parameters - jobid - id of the job (integer)

Results - finished - boolean - duration - time in seconds that the job
ran for - endTime - time the job finished (eg
“2018-10-26T18:50:20.528746884+01:00”) - error - error from the job or
empty string for no error - finished - boolean whether the job has
finished or not - id - as passed in above - startTime - time the job
started (eg “2018-10-26T18:50:20.528336039+01:00”) - success - boolean -
true for success false otherwise - output - output of the job as would
have been returned if called synchronously

operations/about: Return the space used on the remote

This takes the following parameters

-   fs - a remote name string eg “drive:”

The result is as returned from rclone about –json

See the about command command for more information on the above.

Authentication is required for this call.

operations/cleanup: Remove trashed files in the remote or path

This takes the following parameters

-   fs - a remote name string eg “drive:”

See the cleanup command command for more information on the above.

Authentication is required for this call.

operations/copyfile: Copy a file from source remote to destination remote

This takes the following parameters

-   srcFs - a remote name string eg “drive:” for the source
-   srcRemote - a path within that remote eg “file.txt” for the source
-   dstFs - a remote name string eg “drive2:” for the destination
-   dstRemote - a path within that remote eg “file2.txt” for the
    destination

Authentication is required for this call.

operations/copyurl: Copy the URL to the object

This takes the following parameters

-   fs - a remote name string eg “drive:”
-   remote - a path within that remote eg “dir”
-   url - string, URL to read from

See the copyurl command command for more information on the above.

Authentication is required for this call.

operations/delete: Remove files in the path

This takes the following parameters

-   fs - a remote name string eg “drive:”

See the delete command command for more information on the above.

Authentication is required for this call.

operations/deletefile: Remove the single file pointed to

This takes the following parameters

-   fs - a remote name string eg “drive:”
-   remote - a path within that remote eg “dir”

See the deletefile command command for more information on the above.

Authentication is required for this call.

operations/fsinfo: Return information about the remote

This takes the following parameters

-   fs - a remote name string eg “drive:”

This returns info about the remote passed in;

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
        "Name": "local",    // Name as created
        "Precision": 1,     // Precision of timestamps in ns
        "Root": "/",        // Path as created
        "String": "Local file system at /" // how the remote will appear in logs
    }

This command does not have a command line equivalent so use this
instead:

    rclone rc --loopback operations/fsinfo fs=remote:

operations/list: List the given remote and path in JSON format

This takes the following parameters

-   fs - a remote name string eg “drive:”
-   remote - a path within that remote eg “dir”
-   opt - a dictionary of options to control the listing (optional)
    -   recurse - If set recurse directories
    -   noModTime - If set return modification time
    -   showEncrypted - If set show decrypted names
    -   showOrigIDs - If set show the IDs for each item if known
    -   showHash - If set return a dictionary of hashes

The result is

-   list
    -   This is an array of objects as described in the lsjson command

See the lsjson command for more information on the above and examples.

Authentication is required for this call.

operations/mkdir: Make a destination directory or container

This takes the following parameters

-   fs - a remote name string eg “drive:”
-   remote - a path within that remote eg “dir”

See the mkdir command command for more information on the above.

Authentication is required for this call.

operations/movefile: Move a file from source remote to destination remote

This takes the following parameters

-   srcFs - a remote name string eg “drive:” for the source
-   srcRemote - a path within that remote eg “file.txt” for the source
-   dstFs - a remote name string eg “drive2:” for the destination
-   dstRemote - a path within that remote eg “file2.txt” for the
    destination

Authentication is required for this call.

operations/publiclink: Create or retrieve a public link to the given file or folder.

This takes the following parameters

-   fs - a remote name string eg “drive:”
-   remote - a path within that remote eg “dir”

Returns

-   url - URL of the resource

See the link command command for more information on the above.

Authentication is required for this call.

operations/purge: Remove a directory or container and all of its contents

This takes the following parameters

-   fs - a remote name string eg “drive:”
-   remote - a path within that remote eg “dir”

See the purge command command for more information on the above.

Authentication is required for this call.

operations/rmdir: Remove an empty directory or container

This takes the following parameters

-   fs - a remote name string eg “drive:”
-   remote - a path within that remote eg “dir”

See the rmdir command command for more information on the above.

Authentication is required for this call.

operations/rmdirs: Remove all the empty directories in the path

This takes the following parameters

-   fs - a remote name string eg “drive:”
-   remote - a path within that remote eg “dir”
-   leaveRoot - boolean, set to true not to delete the root

See the rmdirs command command for more information on the above.

Authentication is required for this call.

operations/size: Count the number of bytes and files in remote

This takes the following parameters

-   fs - a remote name string eg “drive:path/to/dir”

Returns

-   count - number of files
-   bytes - number of bytes in those files

See the size command command for more information on the above.

Authentication is required for this call.

options/blocks: List all the option blocks

Returns - options - a list of the options block names

options/get: Get all the options

Returns an object where keys are option block names and values are an
object with the current option values in.

This shows the internal names of the option within rclone which should
map to the external options very easily with a few exceptions.

options/set: Set an option

Parameters

-   option block name containing an object with
    -   key: value

Repeated as often as required.

Only supply the options you wish to change. If an option is unknown it
will be silently ignored. Not all options will have an effect when
changed like this.

For example:

This sets DEBUG level logs (-vv)

    rclone rc options/set --json '{"main": {"LogLevel": 8}}'

And this sets INFO level logs (-v)

    rclone rc options/set --json '{"main": {"LogLevel": 7}}'

And this sets NOTICE level logs (normal without -v)

    rclone rc options/set --json '{"main": {"LogLevel": 6}}'

rc/error: This returns an error

This returns an error with the input as part of its error string. Useful
for testing error handling.

rc/list: List all the registered remote control commands

This lists all the registered remote control commands as a JSON map in
the commands response.

rc/noop: Echo the input to the output parameters

This echoes the input parameters to the output parameters for testing
purposes. It can be used to check that rclone is still alive and to
check that parameter passing is working properly.

rc/noopauth: Echo the input to the output parameters requiring auth

This echoes the input parameters to the output parameters for testing
purposes. It can be used to check that rclone is still alive and to
check that parameter passing is working properly.

Authentication is required for this call.

sync/copy: copy a directory from source remote to destination remote

This takes the following parameters

-   srcFs - a remote name string eg “drive:src” for the source
-   dstFs - a remote name string eg “drive:dst” for the destination

See the copy command command for more information on the above.

Authentication is required for this call.

sync/move: move a directory from source remote to destination remote

This takes the following parameters

-   srcFs - a remote name string eg “drive:src” for the source
-   dstFs - a remote name string eg “drive:dst” for the destination
-   deleteEmptySrcDirs - delete empty src directories if set

See the move command command for more information on the above.

Authentication is required for this call.

sync/sync: sync a directory from source remote to destination remote

This takes the following parameters

-   srcFs - a remote name string eg “drive:src” for the source
-   dstFs - a remote name string eg “drive:dst” for the destination

See the sync command command for more information on the above.

Authentication is required for this call.

vfs/forget: Forget files or directories in the directory cache.

This forgets the paths in the directory cache causing them to be re-read
from the remote when needed.

If no paths are passed in then it will forget all the paths in the
directory cache.

    rclone rc vfs/forget

Otherwise pass files or dirs in as file=path or dir=path. Any parameter
key starting with file will forget that file and any starting with dir
will forget that dir, eg

    rclone rc vfs/forget file=hello file2=goodbye dir=home/junk

vfs/poll-interval: Get the status or update the value of the poll-interval option.

Without any parameter given this returns the current status of the
poll-interval setting.

When the interval=duration parameter is set, the poll-interval value is
updated and the polling function is notified. Setting interval=0
disables poll-interval.

    rclone rc vfs/poll-interval interval=5m

The timeout=duration parameter can be used to specify a time to wait for
the current poll function to apply the new value. If timeout is less or
equal 0, which is the default, wait indefinitely.

The new poll-interval value will only be active when the timeout is not
reached.

If poll-interval is updated or disabled temporarily, some changes might
not get picked up by the polling function, depending on the used remote.

vfs/refresh: Refresh the directory cache.

This reads the directories for the specified paths and freshens the
directory cache.

If no paths are passed in then it will refresh the root directory.

    rclone rc vfs/refresh

Otherwise pass directories in as dir=path. Any parameter key starting
with dir will refresh that directory, eg

    rclone rc vfs/refresh dir=home/junk dir2=data/misc

If the parameter recursive=true is given the whole directory tree will
get refreshed. This refresh will use –fast-list if enabled.


Accessing the remote control via HTTP

Rclone implements a simple HTTP based protocol.

Each endpoint takes an JSON object and returns a JSON object or an
error. The JSON objects are essentially a map of string names to values.

All calls must made using POST.

The input objects can be supplied using URL parameters, POST parameters
or by supplying “Content-Type: application/json” and a JSON blob in the
body. There are examples of these below using curl.

The response will be a JSON blob in the body of the response. This is
formatted to be reasonably human readable.

Error returns

If an error occurs then there will be an HTTP error status (eg 500) and
the body of the response will contain a JSON encoded error object, eg

    {
        "error": "Expecting string value for key \"remote\" (was float64)",
        "input": {
            "fs": "/tmp",
            "remote": 3
        },
        "status": 400
        "path": "operations/rmdir",
    }

The keys in the error response are - error - error string - input - the
input parameters to the call - status - the HTTP status code - path -
the path of the call

CORS

The sever implements basic CORS support and allows all origins for that.
The response to a preflight OPTIONS request will echo the requested
“Access-Control-Request-Headers” back.

Using POST with URL parameters only

    curl -X POST 'http://localhost:5572/rc/noop?potato=1&sausage=2'

Response

    {
        "potato": "1",
        "sausage": "2"
    }

Here is what an error response looks like:

    curl -X POST 'http://localhost:5572/rc/error?potato=1&sausage=2'

    {
        "error": "arbitrary error on input map[potato:1 sausage:2]",
        "input": {
            "potato": "1",
            "sausage": "2"
        }
    }

Note that curl doesn’t return errors to the shell unless you use the -f
option

    $ curl -f -X POST 'http://localhost:5572/rc/error?potato=1&sausage=2'
    curl: (22) The requested URL returned error: 400 Bad Request
    $ echo $?
    22

Using POST with a form

    curl --data "potato=1" --data "sausage=2" http://localhost:5572/rc/noop

Response

    {
        "potato": "1",
        "sausage": "2"
    }

Note that you can combine these with URL parameters too with the POST
parameters taking precedence.

    curl --data "potato=1" --data "sausage=2" "http://localhost:5572/rc/noop?rutabaga=3&sausage=4"

Response

    {
        "potato": "1",
        "rutabaga": "3",
        "sausage": "4"
    }

Using POST with a JSON blob

    curl -H "Content-Type: application/json" -X POST -d '{"potato":2,"sausage":1}' http://localhost:5572/rc/noop

response

    {
        "password": "xyz",
        "username": "xyz"
    }

This can be combined with URL parameters too if required. The JSON blob
takes precedence.

    curl -H "Content-Type: application/json" -X POST -d '{"potato":2,"sausage":1}' 'http://localhost:5572/rc/noop?rutabaga=3&potato=4'

    {
        "potato": 2,
        "rutabaga": "3",
        "sausage": 1
    }


Debugging rclone with pprof

If you use the --rc flag this will also enable the use of the go
profiling tools on the same port.

To use these, first install go.

Debugging memory use

To profile rclone’s memory use you can run:

    go tool pprof -web http://localhost:5572/debug/pprof/heap

This should open a page in your browser showing what is using what
memory.

You can also use the -text flag to produce a textual summary

    $ go tool pprof -text http://localhost:5572/debug/pprof/heap
    Showing nodes accounting for 1537.03kB, 100% of 1537.03kB total
          flat  flat%   sum%        cum   cum%
     1024.03kB 66.62% 66.62%  1024.03kB 66.62%  github.com/ncw/rclone/vendor/golang.org/x/net/http2/hpack.addDecoderNode
         513kB 33.38%   100%      513kB 33.38%  net/http.newBufioWriterSize
             0     0%   100%  1024.03kB 66.62%  github.com/ncw/rclone/cmd/all.init
             0     0%   100%  1024.03kB 66.62%  github.com/ncw/rclone/cmd/serve.init
             0     0%   100%  1024.03kB 66.62%  github.com/ncw/rclone/cmd/serve/restic.init
             0     0%   100%  1024.03kB 66.62%  github.com/ncw/rclone/vendor/golang.org/x/net/http2.init
             0     0%   100%  1024.03kB 66.62%  github.com/ncw/rclone/vendor/golang.org/x/net/http2/hpack.init
             0     0%   100%  1024.03kB 66.62%  github.com/ncw/rclone/vendor/golang.org/x/net/http2/hpack.init.0
             0     0%   100%  1024.03kB 66.62%  main.init
             0     0%   100%      513kB 33.38%  net/http.(*conn).readRequest
             0     0%   100%      513kB 33.38%  net/http.(*conn).serve
             0     0%   100%  1024.03kB 66.62%  runtime.main

Debugging go routine leaks

Memory leaks are most often caused by go routine leaks keeping memory
alive which should have been garbage collected.

See all active go routines using

    curl http://localhost:5572/debug/pprof/goroutine?debug=1

Or go to http://localhost:5572/debug/pprof/goroutine?debug=1 in your
browser.

Other profiles to look at

You can see a summary of profiles available at
http://localhost:5572/debug/pprof/

Here is how to use some of them:

-   Memory: go tool pprof http://localhost:5572/debug/pprof/heap
-   Go routines:
    curl http://localhost:5572/debug/pprof/goroutine?debug=1
-   30-second CPU profile:
    go tool pprof http://localhost:5572/debug/pprof/profile
-   5-second execution trace:
    wget http://localhost:5572/debug/pprof/trace?seconds=5

See the net/http/pprof docs for more info on how to use the profiling
and for a general overview see the Go team’s blog post on profiling go
programs.

The profiling hook is zero overhead unless it is used.



OVERVIEW OF CLOUD STORAGE SYSTEMS


Each cloud storage system is slightly different. Rclone attempts to
provide a unified interface to them, but some underlying differences
show through.


Features

Here is an overview of the major features of each cloud storage system.

  Name                                Hash       ModTime   Case Insensitive   Duplicate Files   MIME Type
  ------------------------------ -------------- --------- ------------------ ----------------- -----------
  Amazon Drive                        MD5          No            Yes                No              R
  Amazon S3                           MD5          Yes            No                No             R/W
  Backblaze B2                        SHA1         Yes            No                No             R/W
  Box                                 SHA1         Yes           Yes                No              -
  Dropbox                           DBHASH †       Yes           Yes                No              -
  FTP                                  -           No             No                No              -
  Google Cloud Storage                MD5          Yes            No                No             R/W
  Google Drive                        MD5          Yes            No                Yes            R/W
  HTTP                                 -           No             No                No              R
  Hubic                               MD5          Yes            No                No             R/W
  Jottacloud                          MD5          Yes           Yes                No             R/W
  Koofr                               MD5          No            Yes                No              -
  Mega                                 -           No             No                Yes             -
  Microsoft Azure Blob Storage        MD5          Yes            No                No             R/W
  Microsoft OneDrive                SHA1 ‡‡        Yes           Yes                No              R
  OpenDrive                           MD5          Yes           Yes                No              -
  Openstack Swift                     MD5          Yes            No                No             R/W
  pCloud                           MD5, SHA1       Yes            No                No              W
  QingStor                            MD5          No             No                No             R/W
  SFTP                            MD5, SHA1 ‡      Yes         Depends              No              -
  WebDAV                          MD5, SHA1 ††   Yes †††       Depends              No              -
  Yandex Disk                         MD5          Yes            No                No             R/W
  The local filesystem                All          Yes         Depends              No              -

Hash

The cloud storage system supports various hash types of the objects. The
hashes are used when transferring data as an integrity check and can be
specifically used with the --checksum flag in syncs and in the check
command.

To use the verify checksums when transferring between cloud storage
systems they must support a common hash type.

† Note that Dropbox supports its own custom hash. This is an SHA256 sum
of all the 4MB block SHA256s.

‡ SFTP supports checksums if the same login has shell access and md5sum
or sha1sum as well as echo are in the remote’s PATH.

†† WebDAV supports hashes when used with Owncloud and Nextcloud only.

††† WebDAV supports modtimes when used with Owncloud and Nextcloud only.

‡‡ Microsoft OneDrive Personal supports SHA1 hashes, whereas OneDrive
for business and SharePoint server support Microsoft’s own QuickXorHash.

ModTime

The cloud storage system supports setting modification times on objects.
If it does then this enables a using the modification times as part of
the sync. If not then only the size will be checked by default, though
the MD5SUM can be checked with the --checksum flag.

All cloud storage systems support some kind of date on the object and
these will be set when transferring from the cloud storage system.

Case Insensitive

If a cloud storage systems is case sensitive then it is possible to have
two files which differ only in case, eg file.txt and FILE.txt. If a
cloud storage system is case insensitive then that isn’t possible.

This can cause problems when syncing between a case insensitive system
and a case sensitive system. The symptom of this is that no matter how
many times you run the sync it never completes fully.

The local filesystem and SFTP may or may not be case sensitive depending
on OS.

-   Windows - usually case insensitive, though case is preserved
-   OSX - usually case insensitive, though it is possible to format case
    sensitive
-   Linux - usually case sensitive, but there are case insensitive file
    systems (eg FAT formatted USB keys)

Most of the time this doesn’t cause any problems as people tend to avoid
files whose name differs only by case even on case sensitive systems.

Duplicate files

If a cloud storage system allows duplicate files then it can have two
objects with the same name.

This confuses rclone greatly when syncing - use the rclone dedupe
command to rename or remove duplicates.

MIME Type

MIME types (also known as media types) classify types of documents using
a simple text classification, eg text/html or application/pdf.

Some cloud storage systems support reading (R) the MIME type of objects
and some support writing (W) the MIME type of objects.

The MIME type can be important if you are serving files directly to HTTP
from the storage system.

If you are copying from a remote which supports reading (R) to a remote
which supports writing (W) then rclone will preserve the MIME types.
Otherwise they will be guessed from the extension, or the remote itself
may assign the MIME type.


Optional Features

All the remotes support a basic set of features, but there are some
optional features supported by some remotes used to make some operations
more efficient.

  Name                            Purge   Copy   Move   DirMove   CleanUp   ListR   StreamUpload   LinkSharing   About
  ------------------------------ ------- ------ ------ --------- --------- ------- -------------- ------------- -------
  Amazon Drive                     Yes     No    Yes      Yes     No #575    No          No         No #2178      No
  Amazon S3                        No     Yes     No      No        No       Yes        Yes         No #2178      No
  Backblaze B2                     No     Yes     No      No        Yes      Yes        Yes         No #2178      No
  Box                              Yes    Yes    Yes      Yes     No #575    No         Yes            Yes        No
  Dropbox                          Yes    Yes    Yes      Yes     No #575    No         Yes            Yes        Yes
  FTP                              No      No    Yes      Yes       No       No         Yes         No #2178      No
  Google Cloud Storage             Yes    Yes     No      No        No       Yes        Yes         No #2178      No
  Google Drive                     Yes    Yes    Yes      Yes       Yes      Yes        Yes            Yes        Yes
  HTTP                             No      No     No      No        No       No          No         No #2178      No
  Hubic                           Yes †   Yes     No      No        No       Yes        Yes         No #2178      Yes
  Jottacloud                       Yes    Yes    Yes      Yes       No       Yes         No            Yes        Yes
  Mega                             Yes     No    Yes      Yes       Yes      No          No         No #2178      Yes
  Microsoft Azure Blob Storage     Yes    Yes     No      No        No       Yes         No         No #2178      No
  Microsoft OneDrive               Yes    Yes    Yes      Yes     No #575    No          No            Yes        Yes
  OpenDrive                        Yes    Yes    Yes      Yes       No       No          No            No         No
  Openstack Swift                 Yes †   Yes     No      No        No       Yes        Yes         No #2178      Yes
  pCloud                           Yes    Yes    Yes      Yes       Yes      No          No         No #2178      Yes
  QingStor                         No     Yes     No      No        No       Yes         No         No #2178      No
  SFTP                             No      No    Yes      Yes       No       No         Yes         No #2178      Yes
  WebDAV                           Yes    Yes    Yes      Yes       No       No        Yes ‡        No #2178      Yes
  Yandex Disk                      Yes    Yes    Yes      Yes       Yes      No         Yes            Yes        Yes
  The local filesystem             Yes     No    Yes      Yes       No       No         Yes            No         Yes

Purge

This deletes a directory quicker than just deleting all the files in the
directory.

† Note Swift and Hubic implement this in order to delete directory
markers but they don’t actually have a quicker way of deleting files
other than deleting them individually.

‡ StreamUpload is not supported with Nextcloud

Copy

Used when copying an object to and from the same remote. This known as a
server side copy so you can copy a file without downloading it and
uploading it again. It is used if you use rclone copy or rclone move if
the remote doesn’t support Move directly.

If the server doesn’t support Copy directly then for copy operations the
file is downloaded then re-uploaded.

Move

Used when moving/renaming an object on the same remote. This is known as
a server side move of a file. This is used in rclone move if the server
doesn’t support DirMove.

If the server isn’t capable of Move then rclone simulates it with Copy
then delete. If the server doesn’t support Copy then rclone will
download the file and re-upload it.

DirMove

This is used to implement rclone move to move a directory if possible.
If it isn’t then it will use Move on each file (which falls back to Copy
then download and upload - see Move section).

CleanUp

This is used for emptying the trash for a remote by rclone cleanup.

If the server can’t do CleanUp then rclone cleanup will return an error.

ListR

The remote supports a recursive list to list all the contents beneath a
directory quickly. This enables the --fast-list flag to work. See the
rclone docs for more details.

StreamUpload

Some remotes allow files to be uploaded without knowing the file size in
advance. This allows certain operations to work without spooling the
file to local disk first, e.g. rclone rcat.

LinkSharing

Sets the necessary permissions on a file or folder and prints a link
that allows others to access them, even if they don’t have an account on
the particular cloud provider.

About

This is used to fetch quota information from the remote, like bytes
used/free/quota and bytes used in the trash.

This is also used to return the space used, available for rclone mount.

If the server can’t do About then rclone about will return an error.


Alias

The alias remote provides a new name for another remote.

Paths may be as deep as required or a local path, eg
remote:directory/subdirectory or /directory/subdirectory.

During the initial setup with rclone config you will specify the target
remote. The target remote can either be a local path or another remote.

Subfolders can be used in target remote. Assume a alias remote named
backup with the target mydrive:private/backup. Invoking
rclone mkdir backup:desktop is exactly the same as invoking
rclone mkdir mydrive:private/backup/desktop.

There will be no special handling of paths containing .. segments.
Invoking rclone mkdir backup:../desktop is exactly the same as invoking
rclone mkdir mydrive:private/backup/../desktop. The empty path is not
allowed as a remote. To alias the current directory use . instead.

Here is an example of how to make a alias called remote for local
folder. First run:

     rclone config

This will guide you through an interactive setup process:

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Alias for an existing remote
       \ "alias"
     2 / Amazon Drive
       \ "amazon cloud drive"
     3 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     4 / Backblaze B2
       \ "b2"
     5 / Box
       \ "box"
     6 / Cache a remote
       \ "cache"
     7 / Dropbox
       \ "dropbox"
     8 / Encrypt/Decrypt a remote
       \ "crypt"
     9 / FTP Connection
       \ "ftp"
    10 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
    11 / Google Drive
       \ "drive"
    12 / Hubic
       \ "hubic"
    13 / Local Disk
       \ "local"
    14 / Microsoft Azure Blob Storage
       \ "azureblob"
    15 / Microsoft OneDrive
       \ "onedrive"
    16 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    17 / Pcloud
       \ "pcloud"
    18 / QingCloud Object Storage
       \ "qingstor"
    19 / SSH/SFTP Connection
       \ "sftp"
    20 / Webdav
       \ "webdav"
    21 / Yandex Disk
       \ "yandex"
    22 / http Connection
       \ "http"
    Storage> 1
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

Once configured you can then use rclone like this,

List directories in top level in /mnt/storage/backup

    rclone lsd remote:

List all the files in /mnt/storage/backup

    rclone ls remote:

Copy another local directory to the alias directory called source

    rclone copy /home/source remote:source

Standard Options

Here are the standard options specific to alias (Alias for an existing
remote).

–alias-remote

Remote or path to alias. Can be “myremote:path/to/dir”,
“myremote:bucket”, “myremote:” or “/local/path”.

-   Config: remote
-   Env Var: RCLONE_ALIAS_REMOTE
-   Type: string
-   Default: ""


Amazon Drive

Amazon Drive, formerly known as Amazon Cloud Drive, is a cloud storage
service run by Amazon for consumers.


Status

IMPORTANT: rclone supports Amazon Drive only if you have your own set of
API keys. Unfortunately the Amazon Drive developer program is now closed
to new entries so if you don’t already have your own set of keys you
will not be able to use rclone with Amazon Drive.

For the history on why rclone no longer has a set of Amazon Drive API
keys see the forum.

If you happen to know anyone who works at Amazon then please ask them to
re-instate rclone into the Amazon Drive developer program - thanks!


Setup

The initial setup for Amazon Drive involves getting a token from Amazon
which you need to do in your browser. rclone config walks you through
it.

The configuration process for Amazon Drive may involve using an oauth
proxy. This is used to keep the Amazon credentials out of the source
code. The proxy runs in Google’s very secure App Engine environment and
doesn’t store any credentials which pass through it.

Since rclone doesn’t currently have its own Amazon Drive credentials so
you will either need to have your own client_id and client_secret with
Amazon Drive, or use a a third party ouath proxy in which case you will
need to enter client_id, client_secret, auth_url and token_url.

Note also if you are not using Amazon’s auth_url and token_url, (ie you
filled in something for those) then if setting up on a remote machine
you can only use the copying the config method of configuration -
rclone authorize will not work.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

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
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Encrypt/Decrypt a remote
       \ "crypt"
     6 / FTP Connection
       \ "ftp"
     7 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     8 / Google Drive
       \ "drive"
     9 / Hubic
       \ "hubic"
    10 / Local Disk
       \ "local"
    11 / Microsoft OneDrive
       \ "onedrive"
    12 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    13 / SSH/SFTP Connection
       \ "sftp"
    14 / Yandex Disk
       \ "yandex"
    Storage> 1
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

See the remote setup docs for how to set it up on a machine with no
Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Amazon. This only runs from the moment it opens
your browser to the moment you get back the verification code. This is
on http://127.0.0.1:53682/ and this it may require you to unblock it
temporarily if you are running a host firewall.

Once configured you can then use rclone like this,

List directories in top level of your Amazon Drive

    rclone lsd remote:

List all the files in your Amazon Drive

    rclone ls remote:

To copy a local directory to an Amazon Drive directory called backup

    rclone copy /home/source remote:backup

Modified time and MD5SUMs

Amazon Drive doesn’t allow modification times to be changed via the API
so these won’t be accurate or used for syncing.

It does store MD5SUMs so for a more accurate sync, you can use the
--checksum flag.

Deleting files

Any files you delete with rclone will end up in the trash. Amazon don’t
provide an API to permanently delete files, nor to empty the trash, so
you will have to do that with one of Amazon’s apps or via the Amazon
Drive website. As of November 17, 2016, files are automatically deleted
by Amazon from the trash after 30 days.

Using with non .com Amazon accounts

Let’s say you usually use amazon.co.uk. When you authenticate with
rclone it will take you to an amazon.com page to log in. Your
amazon.co.uk email and password should work here just fine.

Standard Options

Here are the standard options specific to amazon cloud drive (Amazon
Drive).

–acd-client-id

Amazon Application Client ID.

-   Config: client_id
-   Env Var: RCLONE_ACD_CLIENT_ID
-   Type: string
-   Default: ""

–acd-client-secret

Amazon Application Client Secret.

-   Config: client_secret
-   Env Var: RCLONE_ACD_CLIENT_SECRET
-   Type: string
-   Default: ""

Advanced Options

Here are the advanced options specific to amazon cloud drive (Amazon
Drive).

–acd-auth-url

Auth server URL. Leave blank to use Amazon’s.

-   Config: auth_url
-   Env Var: RCLONE_ACD_AUTH_URL
-   Type: string
-   Default: ""

–acd-token-url

Token server url. leave blank to use Amazon’s.

-   Config: token_url
-   Env Var: RCLONE_ACD_TOKEN_URL
-   Type: string
-   Default: ""

–acd-checkpoint

Checkpoint for internal polling (debug).

-   Config: checkpoint
-   Env Var: RCLONE_ACD_CHECKPOINT
-   Type: string
-   Default: ""

–acd-upload-wait-per-gb

Additional time per GB to wait after a failed complete upload to see if
it appears.

Sometimes Amazon Drive gives an error when a file has been fully
uploaded but the file appears anyway after a little while. This happens
sometimes for files over 1GB in size and nearly every time for files
bigger than 10GB. This parameter controls the time rclone waits for the
file to appear.

The default value for this parameter is 3 minutes per GB, so by default
it will wait 3 minutes for every GB uploaded to see if the file appears.

You can disable this feature by setting it to 0. This may cause conflict
errors as rclone retries the failed upload but the file will most likely
appear correctly eventually.

These values were determined empirically by observing lots of uploads of
big files for a range of file sizes.

Upload with the “-v” flag to see more info about what rclone is doing in
this situation.

-   Config: upload_wait_per_gb
-   Env Var: RCLONE_ACD_UPLOAD_WAIT_PER_GB
-   Type: Duration
-   Default: 3m0s

–acd-templink-threshold

Files >= this size will be downloaded via their tempLink.

Files this size or more will be downloaded via their “tempLink”. This is
to work around a problem with Amazon Drive which blocks downloads of
files bigger than about 10GB. The default for this is 9GB which
shouldn’t need to be changed.

To download files above this threshold, rclone requests a “tempLink”
which downloads the file through a temporary URL directly from the
underlying S3 storage.

-   Config: templink_threshold
-   Env Var: RCLONE_ACD_TEMPLINK_THRESHOLD
-   Type: SizeSuffix
-   Default: 9G

Limitations

Note that Amazon Drive is case insensitive so you can’t have a file
called “Hello.doc” and one called “hello.doc”.

Amazon Drive has rate limiting so you may notice errors in the sync (429
errors). rclone will automatically retry the sync up to 3 times by
default (see --retries flag) which should hopefully work around this
problem.

Amazon Drive has an internal limit of file sizes that can be uploaded to
the service. This limit is not officially published, but all files
larger than this will fail.

At the time of writing (Jan 2016) is in the area of 50GB per file. This
means that larger files are likely to fail.

Unfortunately there is no way for rclone to see that this failure is
because of file size, so it will retry the operation, as any other
failure. To avoid this problem, use --max-size 50000M option to limit
the maximum size of uploaded files. Note that --max-size does not split
files into segments, it only ignores files over this size.


Amazon S3 Storage Providers

The S3 backend can be used with a number of different providers:

-   AWS S3
-   Alibaba Cloud (Aliyun) Object Storage System (OSS)
-   Ceph
-   DigitalOcean Spaces
-   Dreamhost
-   IBM COS S3
-   Minio
-   Wasabi

Paths are specified as remote:bucket (or remote: for the lsd command.)
You may put subdirectories in too, eg remote:bucket/path/to/dir.

Once you have made a remote (see the provider specific section above)
you can use it like this:

See all buckets

    rclone lsd remote:

Make a new bucket

    rclone mkdir remote:bucket

List the contents of a bucket

    rclone ls remote:bucket

Sync /home/local/directory to the remote bucket, deleting any excess
files in the bucket.

    rclone sync /home/local/directory remote:bucket


AWS S3

Here is an example of making an s3 configuration. First run

    rclone config

This will guide you through an interactive setup process.

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Alias for an existing remote
       \ "alias"
     2 / Amazon Drive
       \ "amazon cloud drive"
     3 / Amazon S3 Compliant Storage Providers (AWS, Ceph, Dreamhost, IBM COS, Minio)
       \ "s3"
     4 / Backblaze B2
       \ "b2"
    [snip]
    23 / http Connection
       \ "http"
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
       / South America (Sao Paulo) Region
    14 | Needs location constraint sa-east-1.
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
    14 / South America (Sao Paulo) Region.
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

–fast-list

This remote supports --fast-list which allows you to use fewer
transactions in exchange for more memory. See the rclone docs for more
details.

–update and –use-server-modtime

As noted below, the modified time is stored on metadata on the object.
It is used by default for all operations that require checking the time
a file was last updated. It allows rclone to treat the remote more like
a true filesystem, but it is inefficient because it requires an extra
API call to retrieve the metadata.

For many operations, the time the object was last uploaded to the remote
is sufficient to determine if it is “dirty”. By using --update along
with --use-server-modtime, you can avoid the extra API call and simply
upload files whose local modtime is newer than the time it was last
uploaded.

Modified time

The modified time is stored as metadata on the object as
X-Amz-Meta-Mtime as floating point since the epoch accurate to 1 ns.

If the modification time needs to be updated rclone will attempt to
perform a server side copy to update the modification if the object can
be copied in a single part.
In the case the object is larger than 5Gb or is in Glacier or Glacier
Deep Archive storage the object will be uploaded rather than copied.

Multipart uploads

rclone supports multipart uploads with S3 which means that it can upload
files bigger than 5GB.

Note that files uploaded _both_ with multipart upload _and_ through
crypt remotes do not have MD5 sums.

rclone switches from single part uploads to multipart uploads at the
point specified by --s3-upload-cutoff. This can be a maximum of 5GB and
a minimum of 0 (ie always upload multipart files).

The chunk sizes used in the multipart upload are specified by
--s3-chunk-size and the number of chunks uploaded concurrently is
specified by --s3-upload-concurrency.

Multipart uploads will use --transfers * --s3-upload-concurrency *
--s3-chunk-size extra memory. Single part uploads to not use extra
memory.

Single part transfers can be faster than multipart transfers or slower
depending on your latency from S3 - the more latency, the more likely
single part transfers will be faster.

Increasing --s3-upload-concurrency will increase throughput (8 would be
a sensible value) and increasing --s3-chunk-size also increases
throughput (16M would be sensible). Increasing either of these will use
more memory. The default values are high enough to gain most of the
possible performance without using too much memory.

Buckets and Regions

With Amazon S3 you can list buckets (rclone lsd) using any region, but
you can only access the content of a bucket from the region it was
created in. If you attempt to access a bucket from the wrong region, you
will get an error, incorrect region, the bucket is not in 'XXX' region.

Authentication

There are a number of ways to supply rclone with a set of AWS
credentials, with and without using the environment.

The different authentication methods are tried in this order:

-   Directly in the rclone configuration file (env_auth = false in the
    config file):
    -   access_key_id and secret_access_key are required.
    -   session_token can be optionally set when using AWS STS.
-   Runtime configuration (env_auth = true in the config file):
    -   Export the following environment variables before running
        rclone:
        -   Access Key ID: AWS_ACCESS_KEY_ID or AWS_ACCESS_KEY
        -   Secret Access Key: AWS_SECRET_ACCESS_KEY or AWS_SECRET_KEY
        -   Session Token: AWS_SESSION_TOKEN (optional)
    -   Or, use a named profile:
        -   Profile files are standard files used by AWS CLI tools
        -   By default it will use the profile in your home directory
            (eg ~/.aws/credentials on unix based systems) file and the
            “default” profile, to change set these environment
            variables:
            -   AWS_SHARED_CREDENTIALS_FILE to control which file.
            -   AWS_PROFILE to control which profile to use.
    -   Or, run rclone in an ECS task with an IAM role (AWS only).
    -   Or, run rclone on an EC2 instance with an IAM role (AWS only).

If none of these option actually end up providing rclone with AWS
credentials then S3 interaction will be non-authenticated (see below).

S3 Permissions

When using the sync subcommand of rclone the following minimum
permissions are required to be available on the bucket being written to:

-   ListBucket
-   DeleteObject
-   GetObject
-   PutObject
-   PutObjectACL

Example policy:

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
            }
        ]
    }

Notes on above:

1.  This is a policy that can be used when creating bucket. It assumes
    that USER_NAME has been created.
2.  The Resource entry must include both resource ARNs, as one implies
    the bucket and the other implies the bucket’s objects.

For reference, here’s an Ansible script that will generate one or more
buckets that will work with rclone sync.

Key Management System (KMS)

If you are using server side encryption with KMS then you will find you
can’t transfer small objects. As a work-around you can use the
--ignore-checksum flag.

A proper fix is being worked on in issue #1824.

Glacier and Glacier Deep Archive

You can upload objects using the glacier storage class or transition
them to glacier using a lifecycle policy. The bucket can still be synced
or copied into normally, but if rclone tries to access data from the
glacier storage class you will see an error like below.

    2017/09/11 19:07:43 Failed to sync: failed to open source object: Object in GLACIER, restore first: path/to/file

In this case you need to restore the object(s) in question before using
rclone.

Note that rclone only speaks the S3 API it does not speak the Glacier
Vault API, so rclone cannot directly access Glacier Vaults.

Standard Options

Here are the standard options specific to s3 (Amazon S3 Compliant
Storage Provider (AWS, Alibaba, Ceph, Digital Ocean, Dreamhost, IBM COS,
Minio, etc)).

–s3-provider

Choose your S3 provider.

-   Config: provider
-   Env Var: RCLONE_S3_PROVIDER
-   Type: string
-   Default: ""
-   Examples:
    -   “AWS”
        -   Amazon Web Services (AWS) S3
    -   “Alibaba”
        -   Alibaba Cloud Object Storage System (OSS) formerly Aliyun
    -   “Ceph”
        -   Ceph Object Storage
    -   “DigitalOcean”
        -   Digital Ocean Spaces
    -   “Dreamhost”
        -   Dreamhost DreamObjects
    -   “IBMCOS”
        -   IBM COS S3
    -   “Minio”
        -   Minio Object Storage
    -   “Netease”
        -   Netease Object Storage (NOS)
    -   “Wasabi”
        -   Wasabi Object Storage
    -   “Other”
        -   Any other S3 compatible provider

–s3-env-auth

Get AWS credentials from runtime (environment variables or EC2/ECS meta
data if no env vars). Only applies if access_key_id and
secret_access_key is blank.

-   Config: env_auth
-   Env Var: RCLONE_S3_ENV_AUTH
-   Type: bool
-   Default: false
-   Examples:
    -   “false”
        -   Enter AWS credentials in the next step
    -   “true”
        -   Get AWS credentials from the environment (env vars or IAM)

–s3-access-key-id

AWS Access Key ID. Leave blank for anonymous access or runtime
credentials.

-   Config: access_key_id
-   Env Var: RCLONE_S3_ACCESS_KEY_ID
-   Type: string
-   Default: ""

–s3-secret-access-key

AWS Secret Access Key (password) Leave blank for anonymous access or
runtime credentials.

-   Config: secret_access_key
-   Env Var: RCLONE_S3_SECRET_ACCESS_KEY
-   Type: string
-   Default: ""

–s3-region

Region to connect to.

-   Config: region
-   Env Var: RCLONE_S3_REGION
-   Type: string
-   Default: ""
-   Examples:
    -   “us-east-1”
        -   The default endpoint - a good choice if you are unsure.
        -   US Region, Northern Virginia or Pacific Northwest.
        -   Leave location constraint empty.
    -   “us-east-2”
        -   US East (Ohio) Region
        -   Needs location constraint us-east-2.
    -   “us-west-2”
        -   US West (Oregon) Region
        -   Needs location constraint us-west-2.
    -   “us-west-1”
        -   US West (Northern California) Region
        -   Needs location constraint us-west-1.
    -   “ca-central-1”
        -   Canada (Central) Region
        -   Needs location constraint ca-central-1.
    -   “eu-west-1”
        -   EU (Ireland) Region
        -   Needs location constraint EU or eu-west-1.
    -   “eu-west-2”
        -   EU (London) Region
        -   Needs location constraint eu-west-2.
    -   “eu-north-1”
        -   EU (Stockholm) Region
        -   Needs location constraint eu-north-1.
    -   “eu-central-1”
        -   EU (Frankfurt) Region
        -   Needs location constraint eu-central-1.
    -   “ap-southeast-1”
        -   Asia Pacific (Singapore) Region
        -   Needs location constraint ap-southeast-1.
    -   “ap-southeast-2”
        -   Asia Pacific (Sydney) Region
        -   Needs location constraint ap-southeast-2.
    -   “ap-northeast-1”
        -   Asia Pacific (Tokyo) Region
        -   Needs location constraint ap-northeast-1.
    -   “ap-northeast-2”
        -   Asia Pacific (Seoul)
        -   Needs location constraint ap-northeast-2.
    -   “ap-south-1”
        -   Asia Pacific (Mumbai)
        -   Needs location constraint ap-south-1.
    -   “sa-east-1”
        -   South America (Sao Paulo) Region
        -   Needs location constraint sa-east-1.

–s3-region

Region to connect to. Leave blank if you are using an S3 clone and you
don’t have a region.

-   Config: region
-   Env Var: RCLONE_S3_REGION
-   Type: string
-   Default: ""
-   Examples:
    -   ""
        -   Use this if unsure. Will use v4 signatures and an empty
            region.
    -   “other-v2-signature”
        -   Use this only if v4 signatures don’t work, eg pre Jewel/v10
            CEPH.

–s3-endpoint

Endpoint for S3 API. Leave blank if using AWS to use the default
endpoint for the region.

-   Config: endpoint
-   Env Var: RCLONE_S3_ENDPOINT
-   Type: string
-   Default: ""

–s3-endpoint

Endpoint for IBM COS S3 API. Specify if using an IBM COS On Premise.

-   Config: endpoint
-   Env Var: RCLONE_S3_ENDPOINT
-   Type: string
-   Default: ""
-   Examples:
    -   “s3-api.us-geo.objectstorage.softlayer.net”
        -   US Cross Region Endpoint
    -   “s3-api.dal.us-geo.objectstorage.softlayer.net”
        -   US Cross Region Dallas Endpoint
    -   “s3-api.wdc-us-geo.objectstorage.softlayer.net”
        -   US Cross Region Washington DC Endpoint
    -   “s3-api.sjc-us-geo.objectstorage.softlayer.net”
        -   US Cross Region San Jose Endpoint
    -   “s3-api.us-geo.objectstorage.service.networklayer.com”
        -   US Cross Region Private Endpoint
    -   “s3-api.dal-us-geo.objectstorage.service.networklayer.com”
        -   US Cross Region Dallas Private Endpoint
    -   “s3-api.wdc-us-geo.objectstorage.service.networklayer.com”
        -   US Cross Region Washington DC Private Endpoint
    -   “s3-api.sjc-us-geo.objectstorage.service.networklayer.com”
        -   US Cross Region San Jose Private Endpoint
    -   “s3.us-east.objectstorage.softlayer.net”
        -   US Region East Endpoint
    -   “s3.us-east.objectstorage.service.networklayer.com”
        -   US Region East Private Endpoint
    -   “s3.us-south.objectstorage.softlayer.net”
        -   US Region South Endpoint
    -   “s3.us-south.objectstorage.service.networklayer.com”
        -   US Region South Private Endpoint
    -   “s3.eu-geo.objectstorage.softlayer.net”
        -   EU Cross Region Endpoint
    -   “s3.fra-eu-geo.objectstorage.softlayer.net”
        -   EU Cross Region Frankfurt Endpoint
    -   “s3.mil-eu-geo.objectstorage.softlayer.net”
        -   EU Cross Region Milan Endpoint
    -   “s3.ams-eu-geo.objectstorage.softlayer.net”
        -   EU Cross Region Amsterdam Endpoint
    -   “s3.eu-geo.objectstorage.service.networklayer.com”
        -   EU Cross Region Private Endpoint
    -   “s3.fra-eu-geo.objectstorage.service.networklayer.com”
        -   EU Cross Region Frankfurt Private Endpoint
    -   “s3.mil-eu-geo.objectstorage.service.networklayer.com”
        -   EU Cross Region Milan Private Endpoint
    -   “s3.ams-eu-geo.objectstorage.service.networklayer.com”
        -   EU Cross Region Amsterdam Private Endpoint
    -   “s3.eu-gb.objectstorage.softlayer.net”
        -   Great Britain Endpoint
    -   “s3.eu-gb.objectstorage.service.networklayer.com”
        -   Great Britain Private Endpoint
    -   “s3.ap-geo.objectstorage.softlayer.net”
        -   APAC Cross Regional Endpoint
    -   “s3.tok-ap-geo.objectstorage.softlayer.net”
        -   APAC Cross Regional Tokyo Endpoint
    -   “s3.hkg-ap-geo.objectstorage.softlayer.net”
        -   APAC Cross Regional HongKong Endpoint
    -   “s3.seo-ap-geo.objectstorage.softlayer.net”
        -   APAC Cross Regional Seoul Endpoint
    -   “s3.ap-geo.objectstorage.service.networklayer.com”
        -   APAC Cross Regional Private Endpoint
    -   “s3.tok-ap-geo.objectstorage.service.networklayer.com”
        -   APAC Cross Regional Tokyo Private Endpoint
    -   “s3.hkg-ap-geo.objectstorage.service.networklayer.com”
        -   APAC Cross Regional HongKong Private Endpoint
    -   “s3.seo-ap-geo.objectstorage.service.networklayer.com”
        -   APAC Cross Regional Seoul Private Endpoint
    -   “s3.mel01.objectstorage.softlayer.net”
        -   Melbourne Single Site Endpoint
    -   “s3.mel01.objectstorage.service.networklayer.com”
        -   Melbourne Single Site Private Endpoint
    -   “s3.tor01.objectstorage.softlayer.net”
        -   Toronto Single Site Endpoint
    -   “s3.tor01.objectstorage.service.networklayer.com”
        -   Toronto Single Site Private Endpoint

–s3-endpoint

Endpoint for OSS API.

-   Config: endpoint
-   Env Var: RCLONE_S3_ENDPOINT
-   Type: string
-   Default: ""
-   Examples:
    -   “oss-cn-hangzhou.aliyuncs.com”
        -   East China 1 (Hangzhou)
    -   “oss-cn-shanghai.aliyuncs.com”
        -   East China 2 (Shanghai)
    -   “oss-cn-qingdao.aliyuncs.com”
        -   North China 1 (Qingdao)
    -   “oss-cn-beijing.aliyuncs.com”
        -   North China 2 (Beijing)
    -   “oss-cn-zhangjiakou.aliyuncs.com”
        -   North China 3 (Zhangjiakou)
    -   “oss-cn-huhehaote.aliyuncs.com”
        -   North China 5 (Huhehaote)
    -   “oss-cn-shenzhen.aliyuncs.com”
        -   South China 1 (Shenzhen)
    -   “oss-cn-hongkong.aliyuncs.com”
        -   Hong Kong (Hong Kong)
    -   “oss-us-west-1.aliyuncs.com”
        -   US West 1 (Silicon Valley)
    -   “oss-us-east-1.aliyuncs.com”
        -   US East 1 (Virginia)
    -   “oss-ap-southeast-1.aliyuncs.com”
        -   Southeast Asia Southeast 1 (Singapore)
    -   “oss-ap-southeast-2.aliyuncs.com”
        -   Asia Pacific Southeast 2 (Sydney)
    -   “oss-ap-southeast-3.aliyuncs.com”
        -   Southeast Asia Southeast 3 (Kuala Lumpur)
    -   “oss-ap-southeast-5.aliyuncs.com”
        -   Asia Pacific Southeast 5 (Jakarta)
    -   “oss-ap-northeast-1.aliyuncs.com”
        -   Asia Pacific Northeast 1 (Japan)
    -   “oss-ap-south-1.aliyuncs.com”
        -   Asia Pacific South 1 (Mumbai)
    -   “oss-eu-central-1.aliyuncs.com”
        -   Central Europe 1 (Frankfurt)
    -   “oss-eu-west-1.aliyuncs.com”
        -   West Europe (London)
    -   “oss-me-east-1.aliyuncs.com”
        -   Middle East 1 (Dubai)

–s3-endpoint

Endpoint for S3 API. Required when using an S3 clone.

-   Config: endpoint
-   Env Var: RCLONE_S3_ENDPOINT
-   Type: string
-   Default: ""
-   Examples:
    -   “objects-us-east-1.dream.io”
        -   Dream Objects endpoint
    -   “nyc3.digitaloceanspaces.com”
        -   Digital Ocean Spaces New York 3
    -   “ams3.digitaloceanspaces.com”
        -   Digital Ocean Spaces Amsterdam 3
    -   “sgp1.digitaloceanspaces.com”
        -   Digital Ocean Spaces Singapore 1
    -   “s3.wasabisys.com”
        -   Wasabi US East endpoint
    -   “s3.us-west-1.wasabisys.com”
        -   Wasabi US West endpoint
    -   “s3.eu-central-1.wasabisys.com”
        -   Wasabi EU Central endpoint

–s3-location-constraint

Location constraint - must be set to match the Region. Used when
creating buckets only.

-   Config: location_constraint
-   Env Var: RCLONE_S3_LOCATION_CONSTRAINT
-   Type: string
-   Default: ""
-   Examples:
    -   ""
        -   Empty for US Region, Northern Virginia or Pacific Northwest.
    -   “us-east-2”
        -   US East (Ohio) Region.
    -   “us-west-2”
        -   US West (Oregon) Region.
    -   “us-west-1”
        -   US West (Northern California) Region.
    -   “ca-central-1”
        -   Canada (Central) Region.
    -   “eu-west-1”
        -   EU (Ireland) Region.
    -   “eu-west-2”
        -   EU (London) Region.
    -   “eu-north-1”
        -   EU (Stockholm) Region.
    -   “EU”
        -   EU Region.
    -   “ap-southeast-1”
        -   Asia Pacific (Singapore) Region.
    -   “ap-southeast-2”
        -   Asia Pacific (Sydney) Region.
    -   “ap-northeast-1”
        -   Asia Pacific (Tokyo) Region.
    -   “ap-northeast-2”
        -   Asia Pacific (Seoul)
    -   “ap-south-1”
        -   Asia Pacific (Mumbai)
    -   “sa-east-1”
        -   South America (Sao Paulo) Region.

–s3-location-constraint

Location constraint - must match endpoint when using IBM Cloud Public.
For on-prem COS, do not make a selection from this list, hit enter

-   Config: location_constraint
-   Env Var: RCLONE_S3_LOCATION_CONSTRAINT
-   Type: string
-   Default: ""
-   Examples:
    -   “us-standard”
        -   US Cross Region Standard
    -   “us-vault”
        -   US Cross Region Vault
    -   “us-cold”
        -   US Cross Region Cold
    -   “us-flex”
        -   US Cross Region Flex
    -   “us-east-standard”
        -   US East Region Standard
    -   “us-east-vault”
        -   US East Region Vault
    -   “us-east-cold”
        -   US East Region Cold
    -   “us-east-flex”
        -   US East Region Flex
    -   “us-south-standard”
        -   US South Region Standard
    -   “us-south-vault”
        -   US South Region Vault
    -   “us-south-cold”
        -   US South Region Cold
    -   “us-south-flex”
        -   US South Region Flex
    -   “eu-standard”
        -   EU Cross Region Standard
    -   “eu-vault”
        -   EU Cross Region Vault
    -   “eu-cold”
        -   EU Cross Region Cold
    -   “eu-flex”
        -   EU Cross Region Flex
    -   “eu-gb-standard”
        -   Great Britain Standard
    -   “eu-gb-vault”
        -   Great Britain Vault
    -   “eu-gb-cold”
        -   Great Britain Cold
    -   “eu-gb-flex”
        -   Great Britain Flex
    -   “ap-standard”
        -   APAC Standard
    -   “ap-vault”
        -   APAC Vault
    -   “ap-cold”
        -   APAC Cold
    -   “ap-flex”
        -   APAC Flex
    -   “mel01-standard”
        -   Melbourne Standard
    -   “mel01-vault”
        -   Melbourne Vault
    -   “mel01-cold”
        -   Melbourne Cold
    -   “mel01-flex”
        -   Melbourne Flex
    -   “tor01-standard”
        -   Toronto Standard
    -   “tor01-vault”
        -   Toronto Vault
    -   “tor01-cold”
        -   Toronto Cold
    -   “tor01-flex”
        -   Toronto Flex

–s3-location-constraint

Location constraint - must be set to match the Region. Leave blank if
not sure. Used when creating buckets only.

-   Config: location_constraint
-   Env Var: RCLONE_S3_LOCATION_CONSTRAINT
-   Type: string
-   Default: ""

–s3-acl

Canned ACL used when creating buckets and storing or copying objects.

This ACL is used for creating objects and if bucket_acl isn’t set, for
creating buckets too.

For more info visit
https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl

Note that this ACL is applied when server side copying objects as S3
doesn’t copy the ACL from the source but rather writes a fresh one.

-   Config: acl
-   Env Var: RCLONE_S3_ACL
-   Type: string
-   Default: ""
-   Examples:
    -   “private”
        -   Owner gets FULL_CONTROL. No one else has access rights
            (default).
    -   “public-read”
        -   Owner gets FULL_CONTROL. The AllUsers group gets READ
            access.
    -   “public-read-write”
        -   Owner gets FULL_CONTROL. The AllUsers group gets READ and
            WRITE access.
        -   Granting this on a bucket is generally not recommended.
    -   “authenticated-read”
        -   Owner gets FULL_CONTROL. The AuthenticatedUsers group gets
            READ access.
    -   “bucket-owner-read”
        -   Object owner gets FULL_CONTROL. Bucket owner gets READ
            access.
        -   If you specify this canned ACL when creating a bucket,
            Amazon S3 ignores it.
    -   “bucket-owner-full-control”
        -   Both the object owner and the bucket owner get FULL_CONTROL
            over the object.
        -   If you specify this canned ACL when creating a bucket,
            Amazon S3 ignores it.
    -   “private”
        -   Owner gets FULL_CONTROL. No one else has access rights
            (default). This acl is available on IBM Cloud (Infra), IBM
            Cloud (Storage), On-Premise COS
    -   “public-read”
        -   Owner gets FULL_CONTROL. The AllUsers group gets READ
            access. This acl is available on IBM Cloud (Infra), IBM
            Cloud (Storage), On-Premise IBM COS
    -   “public-read-write”
        -   Owner gets FULL_CONTROL. The AllUsers group gets READ and
            WRITE access. This acl is available on IBM Cloud (Infra),
            On-Premise IBM COS
    -   “authenticated-read”
        -   Owner gets FULL_CONTROL. The AuthenticatedUsers group gets
            READ access. Not supported on Buckets. This acl is available
            on IBM Cloud (Infra) and On-Premise IBM COS

–s3-server-side-encryption

The server-side encryption algorithm used when storing this object in
S3.

-   Config: server_side_encryption
-   Env Var: RCLONE_S3_SERVER_SIDE_ENCRYPTION
-   Type: string
-   Default: ""
-   Examples:
    -   ""
        -   None
    -   “AES256”
        -   AES256
    -   “aws:kms”
        -   aws:kms

–s3-sse-kms-key-id

If using KMS ID you must provide the ARN of Key.

-   Config: sse_kms_key_id
-   Env Var: RCLONE_S3_SSE_KMS_KEY_ID
-   Type: string
-   Default: ""
-   Examples:
    -   ""
        -   None
    -   "arn:aws:kms:us-east-1:*"
        -   arn:aws:kms:*

–s3-storage-class

The storage class to use when storing new objects in S3.

-   Config: storage_class
-   Env Var: RCLONE_S3_STORAGE_CLASS
-   Type: string
-   Default: ""
-   Examples:
    -   ""
        -   Default
    -   “STANDARD”
        -   Standard storage class
    -   “REDUCED_REDUNDANCY”
        -   Reduced redundancy storage class
    -   “STANDARD_IA”
        -   Standard Infrequent Access storage class
    -   “ONEZONE_IA”
        -   One Zone Infrequent Access storage class
    -   “GLACIER”
        -   Glacier storage class
    -   “DEEP_ARCHIVE”
        -   Glacier Deep Archive storage class

–s3-storage-class

The storage class to use when storing new objects in OSS.

-   Config: storage_class
-   Env Var: RCLONE_S3_STORAGE_CLASS
-   Type: string
-   Default: ""
-   Examples:
    -   ""
        -   Default
    -   “STANDARD”
        -   Standard storage class
    -   “GLACIER”
        -   Archive storage mode.
    -   “STANDARD_IA”
        -   Infrequent access storage mode.

Advanced Options

Here are the advanced options specific to s3 (Amazon S3 Compliant
Storage Provider (AWS, Alibaba, Ceph, Digital Ocean, Dreamhost, IBM COS,
Minio, etc)).

–s3-bucket-acl

Canned ACL used when creating buckets.

For more info visit
https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl

Note that this ACL is applied when only when creating buckets. If it
isn’t set then “acl” is used instead.

-   Config: bucket_acl
-   Env Var: RCLONE_S3_BUCKET_ACL
-   Type: string
-   Default: ""
-   Examples:
    -   “private”
        -   Owner gets FULL_CONTROL. No one else has access rights
            (default).
    -   “public-read”
        -   Owner gets FULL_CONTROL. The AllUsers group gets READ
            access.
    -   “public-read-write”
        -   Owner gets FULL_CONTROL. The AllUsers group gets READ and
            WRITE access.
        -   Granting this on a bucket is generally not recommended.
    -   “authenticated-read”
        -   Owner gets FULL_CONTROL. The AuthenticatedUsers group gets
            READ access.

–s3-upload-cutoff

Cutoff for switching to chunked upload

Any files larger than this will be uploaded in chunks of chunk_size. The
minimum is 0 and the maximum is 5GB.

-   Config: upload_cutoff
-   Env Var: RCLONE_S3_UPLOAD_CUTOFF
-   Type: SizeSuffix
-   Default: 200M

–s3-chunk-size

Chunk size to use for uploading.

When uploading files larger than upload_cutoff they will be uploaded as
multipart uploads using this chunk size.

Note that “–s3-upload-concurrency” chunks of this size are buffered in
memory per transfer.

If you are transferring large files over high speed links and you have
enough memory, then increasing this will speed up the transfers.

-   Config: chunk_size
-   Env Var: RCLONE_S3_CHUNK_SIZE
-   Type: SizeSuffix
-   Default: 5M

–s3-disable-checksum

Don’t store MD5 checksum with object metadata

-   Config: disable_checksum
-   Env Var: RCLONE_S3_DISABLE_CHECKSUM
-   Type: bool
-   Default: false

–s3-session-token

An AWS session token

-   Config: session_token
-   Env Var: RCLONE_S3_SESSION_TOKEN
-   Type: string
-   Default: ""

–s3-upload-concurrency

Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently.

If you are uploading small numbers of large file over high speed link
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.

-   Config: upload_concurrency
-   Env Var: RCLONE_S3_UPLOAD_CONCURRENCY
-   Type: int
-   Default: 4

–s3-force-path-style

If true use path style access if false use virtual hosted style.

If this is true (the default) then rclone will use path style access, if
false then rclone will use virtual path style. See the AWS S3 docs for
more info.

Some providers (eg Aliyun OSS or Netease COS) require this set to false.

-   Config: force_path_style
-   Env Var: RCLONE_S3_FORCE_PATH_STYLE
-   Type: bool
-   Default: true

–s3-v2-auth

If true use v2 authentication.

If this is false (the default) then rclone will use v4 authentication.
If it is set then rclone will use v2 authentication.

Use this only if v4 signatures don’t work, eg pre Jewel/v10 CEPH.

-   Config: v2_auth
-   Env Var: RCLONE_S3_V2_AUTH
-   Type: bool
-   Default: false

–s3-use-accelerate-endpoint

If true use the AWS S3 accelerated endpoint.

See: AWS S3 Transfer acceleration

-   Config: use_accelerate_endpoint
-   Env Var: RCLONE_S3_USE_ACCELERATE_ENDPOINT
-   Type: bool
-   Default: false

Anonymous access to public buckets

If you want to use rclone to access a public bucket, configure with a
blank access_key_id and secret_access_key. Your config should end up
looking like this:

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

Then use it as normal with the name of the public bucket, eg

    rclone lsd anons3:1000genomes

You will be able to list and copy data but not upload it.

Ceph

Ceph is an open source unified, distributed storage system designed for
excellent performance, reliability and scalability. It has an S3
compatible object storage interface.

To use rclone with Ceph, configure as above but leave the region blank
and set the endpoint. You should end up with something like this in your
config:

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

If you are using an older version of CEPH, eg 10.2.x Jewel, then you may
need to supply the parameter --s3-upload-cutoff 0 or put this in the
config file as upload_cutoff 0 to work around a bug which causes
uploading of small files to fail.

Note also that Ceph sometimes puts / in the passwords it gives users. If
you read the secret access key using the command line tools you will get
a JSON blob with the / escaped as \/. Make sure you only write / in the
secret access key.

Eg the dump from Ceph looks something like this (irrelevant keys
removed).

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

Because this is a json dump, it is encoding the / as \/, so if you use
the secret key as xxxxxx/xxxx it will work fine.

Dreamhost

Dreamhost DreamObjects is an object storage system based on CEPH.

To use rclone with Dreamhost, configure as above but leave the region
blank and set the endpoint. You should end up with something like this
in your config:

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

DigitalOcean Spaces

Spaces is an S3-interoperable object storage service from cloud provider
DigitalOcean.

To connect to DigitalOcean Spaces you will need an access key and secret
key. These can be retrieved on the “Applications & API” page of the
DigitalOcean control panel. They will be needed when promted by
rclone config for your access_key_id and secret_access_key.

When prompted for a region or location_constraint, press enter to use
the default value. The region must be included in the endpoint setting
(e.g. nyc3.digitaloceanspaces.com). The default values can be used for
other settings.

Going through the whole process of creating a new remote by running
rclone config, each prompt should be answered as shown below:

    Storage> s3
    env_auth> 1
    access_key_id> YOUR_ACCESS_KEY
    secret_access_key> YOUR_SECRET_KEY
    region>
    endpoint> nyc3.digitaloceanspaces.com
    location_constraint>
    acl>
    storage_class>

The resulting configuration file should look like:

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

Once configured, you can create a new Space and begin copying files. For
example:

    rclone mkdir spaces:my-new-space
    rclone copy /path/to/files spaces:my-new-space

IBM COS (S3)

Information stored with IBM Cloud Object Storage is encrypted and
dispersed across multiple geographic locations, and accessed through an
implementation of the S3 API. This service makes use of the distributed
storage technologies provided by IBM’s Cloud Object Storage System
(formerly Cleversafe). For more information visit:
(http://www.ibm.com/cloud/object-storage)

To configure access to IBM COS S3, follow the steps below:

1.  Run rclone config and select n for a new remote.

        2018/02/14 14:13:11 NOTICE: Config file "C:\\Users\\a\\.config\\rclone\\rclone.conf" not found - using defaults
        No remotes found - make a new one
        n) New remote
        s) Set configuration password
        q) Quit config
        n/s/q> n

2.  Enter the name for the configuration

        name> <YOUR NAME>

3.  Select “s3” storage.

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

4.  Select IBM COS as the S3 Storage Provider.

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

5.  Enter the Access Key and Secret.

        AWS Access Key ID - leave blank for anonymous access or runtime credentials.
        access_key_id> <>
        AWS Secret Access Key (password) - leave blank for anonymous access or runtime credentials.
        secret_access_key> <>

6.  Specify the endpoint for IBM COS. For Public IBM COS, choose from
    the option below. For On Premise IBM COS, enter an enpoint address.

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

7.  Specify a IBM COS Location Constraint. The location constraint must
    match endpoint when using IBM Cloud Public. For on-prem COS, do not
    make a selection from this list, hit enter

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

9.  Specify a canned ACL. IBM Cloud (Strorage) supports “public-read”
    and “private”. IBM Cloud(Infra) supports all the canned ACLs.
    On-Premise COS supports all the canned ACLs.

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

12. Review the displayed configuration and accept to save the “remote”
    then quit. The config file should look like this

        [xxx]
        type = s3
        Provider = IBMCOS
        access_key_id = xxx
        secret_access_key = yyy
        endpoint = s3-api.us-geo.objectstorage.softlayer.net
        location_constraint = us-standard
        acl = private

13. Execute rclone commands

        1)  Create a bucket.
            rclone mkdir IBM-COS-XREGION:newbucket
        2)  List available buckets.
            rclone lsd IBM-COS-XREGION:
            -1 2017-11-08 21:16:22        -1 test
            -1 2018-02-14 20:16:39        -1 newbucket
        3)  List contents of a bucket.
            rclone ls IBM-COS-XREGION:newbucket
            18685952 test.exe
        4)  Copy a file from local to remote.
            rclone copy /Users/file.txt IBM-COS-XREGION:newbucket
        5)  Copy a file from remote to local.
            rclone copy IBM-COS-XREGION:newbucket/file.txt .
        6)  Delete a file on remote.
            rclone delete IBM-COS-XREGION:newbucket/file.txt

Minio

Minio is an object storage server built for cloud application developers
and devops.

It is very easy to install and provides an S3 compatible server which
can be used by rclone.

To use it, install Minio following the instructions here.

When it configures itself Minio will print something like this

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

These details need to go into rclone config like this. Note that it is
important to put the region in as stated above.

    env_auth> 1
    access_key_id> USWUXHGYZQYFYFFIT3RE
    secret_access_key> MOJRH0mkL1IPauahWITSVvyDrQbEEIwljvmxdq03
    region> us-east-1
    endpoint> http://192.168.1.106:9000
    location_constraint>
    server_side_encryption>

Which makes the config file look like this

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

So once set up, for example to copy files into a bucket

    rclone copy /path/to/files minio:bucket

Scaleway

Scaleway The Object Storage platform allows you to store anything from
backups, logs and web assets to documents and photos. Files can be
dropped from the Scaleway console or transferred through our API and CLI
or using any S3-compatible tool.

Scaleway provides an S3 interface which can be configured for use with
rclone like this:

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

Wasabi

Wasabi is a cloud-based object storage service for a broad range of
applications and use cases. Wasabi is designed for individuals and
organizations that require a high-performance, reliable, and secure data
storage infrastructure at minimal cost.

Wasabi provides an S3 interface which can be configured for use with
rclone like this.

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    n/s> n
    name> wasabi
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
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

This will leave the config file looking like this.

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

Alibaba OSS

Here is an example of making an Alibaba Cloud (Aliyun) OSS
configuration. First run:

    rclone config

This will guide you through an interactive setup process.

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

Netease NOS

For Netease NOS configure as per the configurator rclone config setting
the provider Netease. This will automatically set
force_path_style = false which is necessary for it to run properly.


Backblaze B2

B2 is Backblaze’s cloud storage system.

Paths are specified as remote:bucket (or remote: for the lsd command.)
You may put subdirectories in too, eg remote:bucket/path/to/dir.

Here is an example of making a b2 configuration. First run

    rclone config

This will guide you through an interactive setup process. To
authenticate you will either need your Account ID (a short hex number)
and Master Application Key (a long hex number) OR an Application Key,
which is the recommended method. See below for further details on
generating and using an Application Key.

    No remotes found - make a new one
    n) New remote
    q) Quit config
    n/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Encrypt/Decrypt a remote
       \ "crypt"
     6 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     7 / Google Drive
       \ "drive"
     8 / Hubic
       \ "hubic"
     9 / Local Disk
       \ "local"
    10 / Microsoft OneDrive
       \ "onedrive"
    11 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    12 / SSH/SFTP Connection
       \ "sftp"
    13 / Yandex Disk
       \ "yandex"
    Storage> 3
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

This remote is called remote and can now be used like this

See all buckets

    rclone lsd remote:

Create a new bucket

    rclone mkdir remote:bucket

List the contents of a bucket

    rclone ls remote:bucket

Sync /home/local/directory to the remote bucket, deleting any excess
files in the bucket.

    rclone sync /home/local/directory remote:bucket

Application Keys

B2 supports multiple Application Keys for different access permission to
B2 Buckets.

You can use these with rclone too; you will need to use rclone version
1.43 or later.

Follow Backblaze’s docs to create an Application Key with the required
permission and add the applicationKeyId as the account and the
Application Key itself as the key.

Note that you must put the _applicationKeyId_ as the account – you can’t
use the master Account ID. If you try then B2 will return 401 errors.

–fast-list

This remote supports --fast-list which allows you to use fewer
transactions in exchange for more memory. See the rclone docs for more
details.

Modified time

The modified time is stored as metadata on the object as
X-Bz-Info-src_last_modified_millis as milliseconds since 1970-01-01 in
the Backblaze standard. Other tools should be able to use this as a
modified time.

Modified times are used in syncing and are fully supported. Note that if
a modification time needs to be updated on an object then it will create
a new version of the object.

SHA1 checksums

The SHA1 checksums of the files are checked on upload and download and
will be used in the syncing process.

Large files (bigger than the limit in --b2-upload-cutoff) which are
uploaded in chunks will store their SHA1 on the object as
X-Bz-Info-large_file_sha1 as recommended by Backblaze.

For a large file to be uploaded with an SHA1 checksum, the source needs
to support SHA1 checksums. The local disk supports SHA1 checksums so
large file transfers from local disk will have an SHA1. See the overview
for exactly which remotes support SHA1.

Sources which don’t support SHA1, in particular crypt will upload large
files without SHA1 checksums. This may be fixed in the future (see
#1767).

Files sizes below --b2-upload-cutoff will always have an SHA1 regardless
of the source.

Transfers

Backblaze recommends that you do lots of transfers simultaneously for
maximum speed. In tests from my SSD equipped laptop the optimum setting
is about --transfers 32 though higher numbers may be used for a slight
speed improvement. The optimum number for you may vary depending on your
hardware, how big the files are, how much you want to load your
computer, etc. The default of --transfers 4 is definitely too low for
Backblaze B2 though.

Note that uploading big files (bigger than 200 MB by default) will use a
96 MB RAM buffer by default. There can be at most --transfers of these
in use at any moment, so this sets the upper limit on the memory used.

Versions

When rclone uploads a new version of a file it creates a new version of
it. Likewise when you delete a file, the old version will be marked
hidden and still be available. Conversely, you may opt in to a “hard
delete” of files with the --b2-hard-delete flag which would permanently
remove the file instead of hiding it.

Old versions of files, where available, are visible using the
--b2-versions flag.

NB Note that --b2-versions does not work with crypt at the moment #1627.
Using –backup-dir with rclone is the recommended way of working around
this.

If you wish to remove all the old versions then you can use the
rclone cleanup remote:bucket command which will delete all the old
versions of files, leaving the current ones intact. You can also supply
a path and only old versions under that path will be deleted, eg
rclone cleanup remote:bucket/path/to/stuff.

Note that cleanup will remove partially uploaded files from the bucket
if they are more than a day old.

When you purge a bucket, the current and the old versions will be
deleted then the bucket will be deleted.

However delete will cause the current versions of the files to become
hidden old versions.

Here is a session showing the listing and retrieval of an old version
followed by a cleanup of the old versions.

Show current version and all the versions with --b2-versions flag.

    $ rclone -q ls b2:cleanup-test
            9 one.txt

    $ rclone -q --b2-versions ls b2:cleanup-test
            9 one.txt
            8 one-v2016-07-04-141032-000.txt
           16 one-v2016-07-04-141003-000.txt
           15 one-v2016-07-02-155621-000.txt

Retrieve an old version

    $ rclone -q --b2-versions copy b2:cleanup-test/one-v2016-07-04-141003-000.txt /tmp

    $ ls -l /tmp/one-v2016-07-04-141003-000.txt
    -rw-rw-r-- 1 ncw ncw 16 Jul  2 17:46 /tmp/one-v2016-07-04-141003-000.txt

Clean up all the old versions and show that they’ve gone.

    $ rclone -q cleanup b2:cleanup-test

    $ rclone -q ls b2:cleanup-test
            9 one.txt

    $ rclone -q --b2-versions ls b2:cleanup-test
            9 one.txt

Data usage

It is useful to know how many requests are sent to the server in
different scenarios.

All copy commands send the following 4 requests:

    /b2api/v1/b2_authorize_account
    /b2api/v1/b2_create_bucket
    /b2api/v1/b2_list_buckets
    /b2api/v1/b2_list_file_names

The b2_list_file_names request will be sent once for every 1k files in
the remote path, providing the checksum and modification time of the
listed files. As of version 1.33 issue #818 causes extra requests to be
sent when using B2 with Crypt. When a copy operation does not require
any files to be uploaded, no more requests will be sent.

Uploading files that do not require chunking, will send 2 requests per
file upload:

    /b2api/v1/b2_get_upload_url
    /b2api/v1/b2_upload_file/

Uploading files requiring chunking, will send 2 requests (one each to
start and finish the upload) and another 2 requests for each chunk:

    /b2api/v1/b2_start_large_file
    /b2api/v1/b2_get_upload_part_url
    /b2api/v1/b2_upload_part/
    /b2api/v1/b2_finish_large_file

Versions

Versions can be viewed with the --b2-versions flag. When it is set
rclone will show and act on older versions of files. For example

Listing without --b2-versions

    $ rclone -q ls b2:cleanup-test
            9 one.txt

And with

    $ rclone -q --b2-versions ls b2:cleanup-test
            9 one.txt
            8 one-v2016-07-04-141032-000.txt
           16 one-v2016-07-04-141003-000.txt
           15 one-v2016-07-02-155621-000.txt

Showing that the current version is unchanged but older versions can be
seen. These have the UTC date that they were uploaded to the server to
the nearest millisecond appended to them.

Note that when using --b2-versions no file write operations are
permitted, so you can’t upload files or delete them.

Standard Options

Here are the standard options specific to b2 (Backblaze B2).

–b2-account

Account ID or Application Key ID

-   Config: account
-   Env Var: RCLONE_B2_ACCOUNT
-   Type: string
-   Default: ""

–b2-key

Application Key

-   Config: key
-   Env Var: RCLONE_B2_KEY
-   Type: string
-   Default: ""

–b2-hard-delete

Permanently delete files on remote removal, otherwise hide files.

-   Config: hard_delete
-   Env Var: RCLONE_B2_HARD_DELETE
-   Type: bool
-   Default: false

Advanced Options

Here are the advanced options specific to b2 (Backblaze B2).

–b2-endpoint

Endpoint for the service. Leave blank normally.

-   Config: endpoint
-   Env Var: RCLONE_B2_ENDPOINT
-   Type: string
-   Default: ""

–b2-test-mode

A flag string for X-Bz-Test-Mode header for debugging.

This is for debugging purposes only. Setting it to one of the strings
below will cause b2 to return specific errors:

-   “fail_some_uploads”
-   “expire_some_account_authorization_tokens”
-   “force_cap_exceeded”

These will be set in the “X-Bz-Test-Mode” header which is documented in
the b2 integrations checklist.

-   Config: test_mode
-   Env Var: RCLONE_B2_TEST_MODE
-   Type: string
-   Default: ""

–b2-versions

Include old versions in directory listings. Note that when using this no
file write operations are permitted, so you can’t upload files or delete
them.

-   Config: versions
-   Env Var: RCLONE_B2_VERSIONS
-   Type: bool
-   Default: false

–b2-upload-cutoff

Cutoff for switching to chunked upload.

Files above this size will be uploaded in chunks of “–b2-chunk-size”.

This value should be set no larger than 4.657GiB (== 5GB).

-   Config: upload_cutoff
-   Env Var: RCLONE_B2_UPLOAD_CUTOFF
-   Type: SizeSuffix
-   Default: 200M

–b2-chunk-size

Upload chunk size. Must fit in memory.

When uploading large files, chunk the file into this size. Note that
these chunks are buffered in memory and there might a maximum of
“–transfers” chunks in progress at once. 5,000,000 Bytes is the minimum
size.

-   Config: chunk_size
-   Env Var: RCLONE_B2_CHUNK_SIZE
-   Type: SizeSuffix
-   Default: 96M

–b2-disable-checksum

Disable checksums for large (> upload cutoff) files

-   Config: disable_checksum
-   Env Var: RCLONE_B2_DISABLE_CHECKSUM
-   Type: bool
-   Default: false

–b2-download-url

Custom endpoint for downloads.

This is usually set to a Cloudflare CDN URL as Backblaze offers free
egress for data downloaded through the Cloudflare network. Leave blank
if you want to use the endpoint provided by Backblaze.

-   Config: download_url
-   Env Var: RCLONE_B2_DOWNLOAD_URL
-   Type: string
-   Default: ""


Box

Paths are specified as remote:path

Paths may be as deep as required, eg remote:directory/subdirectory.

The initial setup for Box involves getting a token from Box which you
need to do in your browser. rclone config walks you through it.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Box
       \ "box"
     5 / Dropbox
       \ "dropbox"
     6 / Encrypt/Decrypt a remote
       \ "crypt"
     7 / FTP Connection
       \ "ftp"
     8 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     9 / Google Drive
       \ "drive"
    10 / Hubic
       \ "hubic"
    11 / Local Disk
       \ "local"
    12 / Microsoft OneDrive
       \ "onedrive"
    13 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    14 / SSH/SFTP Connection
       \ "sftp"
    15 / Yandex Disk
       \ "yandex"
    16 / http Connection
       \ "http"
    Storage> box
    Box App Client Id - leave blank normally.
    client_id> 
    Box App Client Secret - leave blank normally.
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
    token = {"access_token":"XXX","token_type":"bearer","refresh_token":"XXX","expiry":"XXX"}
    --------------------
    y) Yes this is OK
    e) Edit this remote
    d) Delete this remote
    y/e/d> y

See the remote setup docs for how to set it up on a machine with no
Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Box. This only runs from the moment it opens your
browser to the moment you get back the verification code. This is on
http://127.0.0.1:53682/ and this it may require you to unblock it
temporarily if you are running a host firewall.

Once configured you can then use rclone like this,

List directories in top level of your Box

    rclone lsd remote:

List all the files in your Box

    rclone ls remote:

To copy a local directory to an Box directory called backup

    rclone copy /home/source remote:backup

Using rclone with an Enterprise account with SSO

If you have an “Enterprise” account type with Box with single sign on
(SSO), you need to create a password to use Box with rclone. This can be
done at your Enterprise Box account by going to Settings, “Account” Tab,
and then set the password in the “Authentication” field.

Once you have done this, you can setup your Enterprise Box account using
the same procedure detailed above in the, using the password you have
just set.

Invalid refresh token

According to the box docs:

  Each refresh_token is valid for one use in 60 days.

This means that if you

-   Don’t use the box remote for 60 days
-   Copy the config file with a box refresh token in and use it in two
    places
-   Get an error on a token refresh

then rclone will return an error which includes the text
Invalid refresh token.

To fix this you will need to use oauth2 again to update the refresh
token. You can use the methods in the remote setup docs, bearing in mind
that if you use the copy the config file method, you should not use that
remote on the computer you did the authentication on.

Here is how to do it.

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

Modified time and hashes

Box allows modification times to be set on objects accurate to 1 second.
These will be used to detect whether objects need syncing or not.

Box supports SHA1 type hashes, so you can use the --checksum flag.

Transfers

For files above 50MB rclone will use a chunked transfer. Rclone will
upload up to --transfers chunks at the same time (shared among all the
multipart uploads). Chunks are buffered in memory and are normally 8MB
so increasing --transfers will increase memory use.

Deleting files

Depending on the enterprise settings for your user, the item will either
be actually deleted from Box or moved to the trash.

Standard Options

Here are the standard options specific to box (Box).

–box-client-id

Box App Client Id. Leave blank normally.

-   Config: client_id
-   Env Var: RCLONE_BOX_CLIENT_ID
-   Type: string
-   Default: ""

–box-client-secret

Box App Client Secret Leave blank normally.

-   Config: client_secret
-   Env Var: RCLONE_BOX_CLIENT_SECRET
-   Type: string
-   Default: ""

Advanced Options

Here are the advanced options specific to box (Box).

–box-upload-cutoff

Cutoff for switching to multipart upload (>= 50MB).

-   Config: upload_cutoff
-   Env Var: RCLONE_BOX_UPLOAD_CUTOFF
-   Type: SizeSuffix
-   Default: 50M

–box-commit-retries

Max number of times to try committing a multipart file.

-   Config: commit_retries
-   Env Var: RCLONE_BOX_COMMIT_RETRIES
-   Type: int
-   Default: 100

Limitations

Note that Box is case insensitive so you can’t have a file called
“Hello.doc” and one called “hello.doc”.

Box file names can’t have the \ character in. rclone maps this to and
from an identical looking unicode equivalent ＼.

Box only supports filenames up to 255 characters in length.


Cache (BETA)

The cache remote wraps another existing remote and stores file structure
and its data for long running tasks like rclone mount.

To get started you just need to have an existing remote which can be
configured with cache.

Here is an example of how to make a remote called test-cache. First run:

     rclone config

This will guide you through an interactive setup process:

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
    ...
     5 / Cache a remote
       \ "cache"
    ...
    Storage> 5
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

You can then use it like this,

List directories in top level of your drive

    rclone lsd test-cache:

List all the files in your drive

    rclone ls test-cache:

To start a cached mount

    rclone mount --allow-other test-cache: /var/tmp/test-cache

Write Features

Offline uploading

In an effort to make writing through cache more reliable, the backend
now supports this feature which can be activated by specifying a
cache-tmp-upload-path.

A files goes through these states when using this feature:

1.  An upload is started (usually by copying a file on the cache remote)
2.  When the copy to the temporary location is complete the file is part
    of the cached remote and looks and behaves like any other file
    (reading included)
3.  After cache-tmp-wait-time passes and the file is next in line,
    rclone move is used to move the file to the cloud provider
4.  Reading the file still works during the upload but most
    modifications on it will be prohibited
5.  Once the move is complete the file is unlocked for modifications as
    it becomes as any other regular file
6.  If the file is being read through cache when it’s actually deleted
    from the temporary path then cache will simply swap the source to
    the cloud provider without interrupting the reading (small blip can
    happen though)

Files are uploaded in sequence and only one file is uploaded at a time.
Uploads will be stored in a queue and be processed based on the order
they were added. The queue and the temporary storage is persistent
across restarts but can be cleared on startup with the --cache-db-purge
flag.

Write Support

Writes are supported through cache. One caveat is that a mounted cache
remote does not add any retry or fallback mechanism to the upload
operation. This will depend on the implementation of the wrapped remote.
Consider using Offline uploading for reliable writes.

One special case is covered with cache-writes which will cache the file
data at the same time as the upload when it is enabled making it
available from the cache store immediately once the upload is finished.

Read Features

Multiple connections

To counter the high latency between a local PC where rclone is running
and cloud providers, the cache remote can split multiple requests to the
cloud provider for smaller file chunks and combines them together
locally where they can be available almost immediately before the reader
usually needs them.

This is similar to buffering when media files are played online. Rclone
will stay around the current marker but always try its best to stay
ahead and prepare the data before.

Plex Integration

There is a direct integration with Plex which allows cache to detect
during reading if the file is in playback or not. This helps cache to
adapt how it queries the cloud provider depending on what is needed for.

Scans will have a minimum amount of workers (1) while in a confirmed
playback cache will deploy the configured number of workers.

This integration opens the doorway to additional performance
improvements which will be explored in the near future.

NOTE: If Plex options are not configured, cache will function with its
configured options without adapting any of its settings.

How to enable? Run rclone config and add all the Plex options (endpoint,
username and password) in your remote and it will be automatically
enabled.

Affected settings: - cache-workers: _Configured value_ during confirmed
playback or _1_ all the other times

Certificate Validation

When the Plex server is configured to only accept secure connections, it
is possible to use .plex.direct URL’s to ensure certificate validation
succeeds. These URL’s are used by Plex internally to connect to the Plex
server securely.

The format for this URL’s is the following:

https://ip-with-dots-replaced.server-hash.plex.direct:32400/

The ip-with-dots-replaced part can be any IPv4 address, where the dots
have been replaced with dashes, e.g. 127.0.0.1 becomes 127-0-0-1.

To get the server-hash part, the easiest way is to visit

https://plex.tv/api/resources?includeHttps=1&X-Plex-Token=your-plex-token

This page will list all the available Plex servers for your account with
at least one .plex.direct link for each. Copy one URL and replace the IP
address with the desired address. This can be used as the plex_url
value.

Known issues

Mount and –dir-cache-time

–dir-cache-time controls the first layer of directory caching which
works at the mount layer. Being an independent caching mechanism from
the cache backend, it will manage its own entries based on the
configured time.

To avoid getting in a scenario where dir cache has obsolete data and
cache would have the correct one, try to set --dir-cache-time to a lower
time than --cache-info-age. Default values are already configured in
this way.

Windows support - Experimental

There are a couple of issues with Windows mount functionality that still
require some investigations. It should be considered as experimental
thus far as fixes come in for this OS.

Most of the issues seem to be related to the difference between
filesystems on Linux flavors and Windows as cache is heavily dependant
on them.

Any reports or feedback on how cache behaves on this OS is greatly
appreciated.

-   https://github.com/ncw/rclone/issues/1935
-   https://github.com/ncw/rclone/issues/1907
-   https://github.com/ncw/rclone/issues/1834

Risk of throttling

Future iterations of the cache backend will make use of the pooling
functionality of the cloud provider to synchronize and at the same time
make writing through it more tolerant to failures.

There are a couple of enhancements in track to add these but in the
meantime there is a valid concern that the expiring cache listings can
lead to cloud provider throttles or bans due to repeated queries on it
for very large mounts.

Some recommendations: - don’t use a very small interval for entry
informations (--cache-info-age) - while writes aren’t yet optimised, you
can still write through cache which gives you the advantage of adding
the file in the cache at the same time if configured to do so.

Future enhancements:

-   https://github.com/ncw/rclone/issues/1937
-   https://github.com/ncw/rclone/issues/1936

cache and crypt

One common scenario is to keep your data encrypted in the cloud provider
using the crypt remote. crypt uses a similar technique to wrap around an
existing remote and handles this translation in a seamless way.

There is an issue with wrapping the remotes in this order: CLOUD REMOTE
-> CRYPT -> CACHE

During testing, I experienced a lot of bans with the remotes in this
order. I suspect it might be related to how crypt opens files on the
cloud provider which makes it think we’re downloading the full file
instead of small chunks. Organizing the remotes in this order yields
better results: CLOUD REMOTE -> CACHE -> CRYPT

absolute remote paths

cache can not differentiate between relative and absolute paths for the
wrapped remote. Any path given in the remote config setting and on the
command line will be passed to the wrapped remote as is, but for storing
the chunks on disk the path will be made relative by removing any
leading / character.

This behavior is irrelevant for most backend types, but there are
backends where a leading / changes the effective directory, e.g. in the
sftp backend paths starting with a / are relative to the root of the SSH
server and paths without are relative to the user home directory. As a
result sftp:bin and sftp:/bin will share the same cache folder, even if
they represent a different directory on the SSH server.

Cache and Remote Control (–rc)

Cache supports the new --rc mode in rclone and can be remote controlled
through the following end points: By default, the listener is disabled
if you do not add the flag.

rc cache/expire

Purge a remote from the cache backend. Supports either a directory or a
file. It supports both encrypted and unencrypted file names if cache is
wrapped by crypt.

Params: - REMOTE = path to remote (REQUIRED) - WITHDATA = true/false to
delete cached data (chunks) as well _(optional, false by default)_

Standard Options

Here are the standard options specific to cache (Cache a remote).

–cache-remote

Remote to cache. Normally should contain a ‘:’ and a path, eg
“myremote:path/to/dir”, “myremote:bucket” or maybe “myremote:” (not
recommended).

-   Config: remote
-   Env Var: RCLONE_CACHE_REMOTE
-   Type: string
-   Default: ""

–cache-plex-url

The URL of the Plex server

-   Config: plex_url
-   Env Var: RCLONE_CACHE_PLEX_URL
-   Type: string
-   Default: ""

–cache-plex-username

The username of the Plex user

-   Config: plex_username
-   Env Var: RCLONE_CACHE_PLEX_USERNAME
-   Type: string
-   Default: ""

–cache-plex-password

The password of the Plex user

-   Config: plex_password
-   Env Var: RCLONE_CACHE_PLEX_PASSWORD
-   Type: string
-   Default: ""

–cache-chunk-size

The size of a chunk (partial file data).

Use lower numbers for slower connections. If the chunk size is changed,
any downloaded chunks will be invalid and cache-chunk-path will need to
be cleared or unexpected EOF errors will occur.

-   Config: chunk_size
-   Env Var: RCLONE_CACHE_CHUNK_SIZE
-   Type: SizeSuffix
-   Default: 5M
-   Examples:
    -   “1m”
        -   1MB
    -   “5M”
        -   5 MB
    -   “10M”
        -   10 MB

–cache-info-age

How long to cache file structure information (directory listings, file
size, times etc). If all write operations are done through the cache
then you can safely make this value very large as the cache store will
also be updated in real time.

-   Config: info_age
-   Env Var: RCLONE_CACHE_INFO_AGE
-   Type: Duration
-   Default: 6h0m0s
-   Examples:
    -   “1h”
        -   1 hour
    -   “24h”
        -   24 hours
    -   “48h”
        -   48 hours

–cache-chunk-total-size

The total size that the chunks can take up on the local disk.

If the cache exceeds this value then it will start to delete the oldest
chunks until it goes under this value.

-   Config: chunk_total_size
-   Env Var: RCLONE_CACHE_CHUNK_TOTAL_SIZE
-   Type: SizeSuffix
-   Default: 10G
-   Examples:
    -   “500M”
        -   500 MB
    -   “1G”
        -   1 GB
    -   “10G”
        -   10 GB

Advanced Options

Here are the advanced options specific to cache (Cache a remote).

–cache-plex-token

The plex token for authentication - auto set normally

-   Config: plex_token
-   Env Var: RCLONE_CACHE_PLEX_TOKEN
-   Type: string
-   Default: ""

–cache-plex-insecure

Skip all certificate verifications when connecting to the Plex server

-   Config: plex_insecure
-   Env Var: RCLONE_CACHE_PLEX_INSECURE
-   Type: string
-   Default: ""

–cache-db-path

Directory to store file structure metadata DB. The remote name is used
as the DB file name.

-   Config: db_path
-   Env Var: RCLONE_CACHE_DB_PATH
-   Type: string
-   Default: “$HOME/.cache/rclone/cache-backend”

–cache-chunk-path

Directory to cache chunk files.

Path to where partial file data (chunks) are stored locally. The remote
name is appended to the final path.

This config follows the “–cache-db-path”. If you specify a custom
location for “–cache-db-path” and don’t specify one for
“–cache-chunk-path” then “–cache-chunk-path” will use the same path as
“–cache-db-path”.

-   Config: chunk_path
-   Env Var: RCLONE_CACHE_CHUNK_PATH
-   Type: string
-   Default: “$HOME/.cache/rclone/cache-backend”

–cache-db-purge

Clear all the cached data for this remote on start.

-   Config: db_purge
-   Env Var: RCLONE_CACHE_DB_PURGE
-   Type: bool
-   Default: false

–cache-chunk-clean-interval

How often should the cache perform cleanups of the chunk storage. The
default value should be ok for most people. If you find that the cache
goes over “cache-chunk-total-size” too often then try to lower this
value to force it to perform cleanups more often.

-   Config: chunk_clean_interval
-   Env Var: RCLONE_CACHE_CHUNK_CLEAN_INTERVAL
-   Type: Duration
-   Default: 1m0s

–cache-read-retries

How many times to retry a read from a cache storage.

Since reading from a cache stream is independent from downloading file
data, readers can get to a point where there’s no more data in the
cache. Most of the times this can indicate a connectivity issue if cache
isn’t able to provide file data anymore.

For really slow connections, increase this to a point where the stream
is able to provide data but your experience will be very stuttering.

-   Config: read_retries
-   Env Var: RCLONE_CACHE_READ_RETRIES
-   Type: int
-   Default: 10

–cache-workers

How many workers should run in parallel to download chunks.

Higher values will mean more parallel processing (better CPU needed) and
more concurrent requests on the cloud provider. This impacts several
aspects like the cloud provider API limits, more stress on the hardware
that rclone runs on but it also means that streams will be more fluid
and data will be available much more faster to readers.

NOTE: If the optional Plex integration is enabled then this setting will
adapt to the type of reading performed and the value specified here will
be used as a maximum number of workers to use.

-   Config: workers
-   Env Var: RCLONE_CACHE_WORKERS
-   Type: int
-   Default: 4

–cache-chunk-no-memory

Disable the in-memory cache for storing chunks during streaming.

By default, cache will keep file data during streaming in RAM as well to
provide it to readers as fast as possible.

This transient data is evicted as soon as it is read and the number of
chunks stored doesn’t exceed the number of workers. However, depending
on other settings like “cache-chunk-size” and “cache-workers” this
footprint can increase if there are parallel streams too (multiple files
being read at the same time).

If the hardware permits it, use this feature to provide an overall
better performance during streaming but it can also be disabled if RAM
is not available on the local machine.

-   Config: chunk_no_memory
-   Env Var: RCLONE_CACHE_CHUNK_NO_MEMORY
-   Type: bool
-   Default: false

–cache-rps

Limits the number of requests per second to the source FS (-1 to
disable)

This setting places a hard limit on the number of requests per second
that cache will be doing to the cloud provider remote and try to respect
that value by setting waits between reads.

If you find that you’re getting banned or limited on the cloud provider
through cache and know that a smaller number of requests per second will
allow you to work with it then you can use this setting for that.

A good balance of all the other settings should make this setting
useless but it is available to set for more special cases.

NOTE: This will limit the number of requests during streams but other
API calls to the cloud provider like directory listings will still pass.

-   Config: rps
-   Env Var: RCLONE_CACHE_RPS
-   Type: int
-   Default: -1

–cache-writes

Cache file data on writes through the FS

If you need to read files immediately after you upload them through
cache you can enable this flag to have their data stored in the cache
store at the same time during upload.

-   Config: writes
-   Env Var: RCLONE_CACHE_WRITES
-   Type: bool
-   Default: false

–cache-tmp-upload-path

Directory to keep temporary files until they are uploaded.

This is the path where cache will use as a temporary storage for new
files that need to be uploaded to the cloud provider.

Specifying a value will enable this feature. Without it, it is
completely disabled and files will be uploaded directly to the cloud
provider

-   Config: tmp_upload_path
-   Env Var: RCLONE_CACHE_TMP_UPLOAD_PATH
-   Type: string
-   Default: ""

–cache-tmp-wait-time

How long should files be stored in local cache before being uploaded

This is the duration that a file must wait in the temporary location
_cache-tmp-upload-path_ before it is selected for upload.

Note that only one file is uploaded at a time and it can take longer to
start the upload if a queue formed for this purpose.

-   Config: tmp_wait_time
-   Env Var: RCLONE_CACHE_TMP_WAIT_TIME
-   Type: Duration
-   Default: 15s

–cache-db-wait-time

How long to wait for the DB to be available - 0 is unlimited

Only one process can have the DB open at any one time, so rclone waits
for this duration for the DB to become available before it gives an
error.

If you set it to 0 then it will wait forever.

-   Config: db_wait_time
-   Env Var: RCLONE_CACHE_DB_WAIT_TIME
-   Type: Duration
-   Default: 1s


Crypt

The crypt remote encrypts and decrypts another remote.

To use it first set up the underlying remote following the config
instructions for that remote. You can also use a local pathname instead
of a remote which will encrypt and decrypt from that directory which
might be useful for encrypting onto a USB stick for example.

First check your chosen remote is working - we’ll call it remote:path in
these docs. Note that anything inside remote:path will be encrypted and
anything outside won’t. This means that if you are using a bucket based
remote (eg S3, B2, swift) then you should probably put the bucket in the
remote s3:bucket. If you just use s3: then rclone will make encrypted
bucket names too (if using file name encryption) which may or may not be
what you want.

Now configure crypt using rclone config. We will call this one secret to
differentiate it from the remote.

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n   
    name> secret
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Encrypt/Decrypt a remote
       \ "crypt"
     6 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     7 / Google Drive
       \ "drive"
     8 / Hubic
       \ "hubic"
     9 / Local Disk
       \ "local"
    10 / Microsoft OneDrive
       \ "onedrive"
    11 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    12 / SSH/SFTP Connection
       \ "sftp"
    13 / Yandex Disk
       \ "yandex"
    Storage> 5
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

IMPORTANT The password is stored in the config file is lightly obscured
so it isn’t immediately obvious what it is. It is in no way secure
unless you use config file encryption.

A long passphrase is recommended, or you can use a random one. Note that
if you reconfigure rclone with the same passwords/passphrases elsewhere
it will be compatible - all the secrets used are derived from those two
passwords/passphrases.

Note that rclone does not encrypt

-   file length - this can be calcuated within 16 bytes
-   modification time - used for syncing


Specifying the remote

In normal use, make sure the remote has a : in. If you specify the
remote without a : then rclone will use a local directory of that name.
So if you use a remote of /path/to/secret/files then rclone will encrypt
stuff to that directory. If you use a remote of name then rclone will
put files in a directory called name in the current directory.

If you specify the remote as remote:path/to/dir then rclone will store
encrypted files in path/to/dir on the remote. If you are using file name
encryption, then when you save files to secret:subdir/subfile this will
store them in the unencrypted path path/to/dir but the subdir/subpath
bit will be encrypted.

Note that unless you want encrypted bucket names (which are difficult to
manage because you won’t know what directory they represent in web
interfaces etc), you should probably specify a bucket, eg
remote:secretbucket when using bucket based remotes such as S3, Swift,
Hubic, B2, GCS.


Example

To test I made a little directory of files using “standard” file name
encryption.

    plaintext/
    ├── file0.txt
    ├── file1.txt
    └── subdir
        ├── file2.txt
        ├── file3.txt
        └── subsubdir
            └── file4.txt

Copy these to the remote and list them back

    $ rclone -q copy plaintext secret:
    $ rclone -q ls secret:
            7 file1.txt
            6 file0.txt
            8 subdir/file2.txt
           10 subdir/subsubdir/file4.txt
            9 subdir/file3.txt

Now see what that looked like when encrypted

    $ rclone -q ls remote:path
           55 hagjclgavj2mbiqm6u6cnjjqcg
           54 v05749mltvv1tf4onltun46gls
           57 86vhrsv86mpbtd3a0akjuqslj8/dlj7fkq4kdq72emafg7a7s41uo
           58 86vhrsv86mpbtd3a0akjuqslj8/7uu829995du6o42n32otfhjqp4/b9pausrfansjth5ob3jkdqd4lc
           56 86vhrsv86mpbtd3a0akjuqslj8/8njh1sk437gttmep3p70g81aps

Note that this retains the directory structure which means you can do
this

    $ rclone -q ls secret:subdir
            8 file2.txt
            9 file3.txt
           10 subsubdir/file4.txt

If don’t use file name encryption then the remote will look like this -
note the .bin extensions added to prevent the cloud provider attempting
to interpret the data.

    $ rclone -q ls remote:path
           54 file0.txt.bin
           57 subdir/file3.txt.bin
           56 subdir/file2.txt.bin
           58 subdir/subsubdir/file4.txt.bin
           55 file1.txt.bin

File name encryption modes

Here are some of the features of the file name encryption modes

Off

-   doesn’t hide file names or directory structure
-   allows for longer file names (~246 characters)
-   can use sub paths and copy single files

Standard

-   file names encrypted
-   file names can’t be as long (~143 characters)
-   can use sub paths and copy single files
-   directory structure visible
-   identical files names will have identical uploaded names
-   can use shortcuts to shorten the directory recursion

Obfuscation

This is a simple “rotate” of the filename, with each file having a rot
distance based on the filename. We store the distance at the beginning
of the filename. So a file called “hello” may become “53.jgnnq”

This is not a strong encryption of filenames, but it may stop automated
scanning tools from picking up on filename patterns. As such it’s an
intermediate between “off” and “standard”. The advantage is that it
allows for longer path segment names.

There is a possibility with some unicode based filenames that the
obfuscation is weak and may map lower case characters to upper case
equivalents. You can not rely on this for strong protection.

-   file names very lightly obfuscated
-   file names can be longer than standard encryption
-   can use sub paths and copy single files
-   directory structure visible
-   identical files names will have identical uploaded names

Cloud storage systems have various limits on file name length and total
path length which you are more likely to hit using “Standard” file name
encryption. If you keep your file names to below 156 characters in
length then you should be OK on all providers.

There may be an even more secure file name encryption mode in the future
which will address the long file name problem.

Directory name encryption

Crypt offers the option of encrypting dir names or leaving them intact.
There are two options:

True

Encrypts the whole file path including directory names Example:
1/12/123.txt is encrypted to
p0e52nreeaj0a5ea7s64m4j72s/l42g6771hnv3an9cgc8cr2n1ng/qgm4avr35m5loi1th53ato71v0

False

Only encrypts file names, skips directory names Example: 1/12/123.txt is
encrypted to 1/12/qgm4avr35m5loi1th53ato71v0

Modified time and hashes

Crypt stores modification times using the underlying remote so support
depends on that.

Hashes are not stored for crypt. However the data integrity is protected
by an extremely strong crypto authenticator.

Note that you should use the rclone cryptcheck command to check the
integrity of a crypted remote instead of rclone check which can’t check
the checksums properly.

Standard Options

Here are the standard options specific to crypt (Encrypt/Decrypt a
remote).

–crypt-remote

Remote to encrypt/decrypt. Normally should contain a ‘:’ and a path, eg
“myremote:path/to/dir”, “myremote:bucket” or maybe “myremote:” (not
recommended).

-   Config: remote
-   Env Var: RCLONE_CRYPT_REMOTE
-   Type: string
-   Default: ""

–crypt-filename-encryption

How to encrypt the filenames.

-   Config: filename_encryption
-   Env Var: RCLONE_CRYPT_FILENAME_ENCRYPTION
-   Type: string
-   Default: “standard”
-   Examples:
    -   “off”
        -   Don’t encrypt the file names. Adds a “.bin” extension only.
    -   “standard”
        -   Encrypt the filenames see the docs for the details.
    -   “obfuscate”
        -   Very simple filename obfuscation.

–crypt-directory-name-encryption

Option to either encrypt directory names or leave them intact.

-   Config: directory_name_encryption
-   Env Var: RCLONE_CRYPT_DIRECTORY_NAME_ENCRYPTION
-   Type: bool
-   Default: true
-   Examples:
    -   “true”
        -   Encrypt directory names.
    -   “false”
        -   Don’t encrypt directory names, leave them intact.

–crypt-password

Password or pass phrase for encryption.

-   Config: password
-   Env Var: RCLONE_CRYPT_PASSWORD
-   Type: string
-   Default: ""

–crypt-password2

Password or pass phrase for salt. Optional but recommended. Should be
different to the previous password.

-   Config: password2
-   Env Var: RCLONE_CRYPT_PASSWORD2
-   Type: string
-   Default: ""

Advanced Options

Here are the advanced options specific to crypt (Encrypt/Decrypt a
remote).

–crypt-show-mapping

For all files listed show how the names encrypt.

If this flag is set then for each file that the remote is asked to list,
it will log (at level INFO) a line stating the decrypted file name and
the encrypted file name.

This is so you can work out which encrypted names are which decrypted
names just in case you need to do something with the encrypted file
names, or for debugging purposes.

-   Config: show_mapping
-   Env Var: RCLONE_CRYPT_SHOW_MAPPING
-   Type: bool
-   Default: false


Backing up a crypted remote

If you wish to backup a crypted remote, it it recommended that you use
rclone sync on the encrypted files, and make sure the passwords are the
same in the new encrypted remote.

This will have the following advantages

-   rclone sync will check the checksums while copying
-   you can use rclone check between the encrypted remotes
-   you don’t decrypt and encrypt unnecessarily

For example, let’s say you have your original remote at remote: with the
encrypted version at eremote: with path remote:crypt. You would then set
up the new remote remote2: and then the encrypted version eremote2: with
path remote2:crypt using the same passwords as eremote:.

To sync the two remotes you would do

    rclone sync remote:crypt remote2:crypt

And to check the integrity you would do

    rclone check remote:crypt remote2:crypt


File formats

File encryption

Files are encrypted 1:1 source file to destination object. The file has
a header and is divided into chunks.

Header

-   8 bytes magic string RCLONE\x00\x00
-   24 bytes Nonce (IV)

The initial nonce is generated from the operating systems crypto strong
random number generator. The nonce is incremented for each chunk read
making sure each nonce is unique for each block written. The chance of a
nonce being re-used is minuscule. If you wrote an exabyte of data (10¹⁸
bytes) you would have a probability of approximately 2×10⁻³² of re-using
a nonce.

Chunk

Each chunk will contain 64kB of data, except for the last one which may
have less data. The data chunk is in standard NACL secretbox format.
Secretbox uses XSalsa20 and Poly1305 to encrypt and authenticate
messages.

Each chunk contains:

-   16 Bytes of Poly1305 authenticator
-   1 - 65536 bytes XSalsa20 encrypted data

64k chunk size was chosen as the best performing chunk size (the
authenticator takes too much time below this and the performance drops
off due to cache effects above this). Note that these chunks are
buffered in memory so they can’t be too big.

This uses a 32 byte (256 bit key) key derived from the user password.

Examples

1 byte file will encrypt to

-   32 bytes header
-   17 bytes data chunk

49 bytes total

1MB (1048576 bytes) file will encrypt to

-   32 bytes header
-   16 chunks of 65568 bytes

1049120 bytes total (a 0.05% overhead). This is the overhead for big
files.

Name encryption

File names are encrypted segment by segment - the path is broken up into
/ separated strings and these are encrypted individually.

File segments are padded using using PKCS#7 to a multiple of 16 bytes
before encryption.

They are then encrypted with EME using AES with 256 bit key. EME
(ECB-Mix-ECB) is a wide-block encryption mode presented in the 2003
paper “A Parallelizable Enciphering Mode” by Halevi and Rogaway.

This makes for deterministic encryption which is what we want - the same
filename must encrypt to the same thing otherwise we can’t find it on
the cloud storage system.

This means that

-   filenames with the same name will encrypt the same
-   filenames which start the same won’t have a common prefix

This uses a 32 byte key (256 bits) and a 16 byte (128 bits) IV both of
which are derived from the user password.

After encryption they are written out using a modified version of
standard base32 encoding as described in RFC4648. The standard encoding
is modified in two ways:

-   it becomes lower case (no-one likes upper case filenames!)
-   we strip the padding character =

base32 is used rather than the more efficient base64 so rclone can be
used on case insensitive remotes (eg Windows, Amazon Drive).

Key derivation

Rclone uses scrypt with parameters N=16384, r=8, p=1 with an optional
user supplied salt (password2) to derive the 32+32+16 = 80 bytes of key
material required. If the user doesn’t supply a salt then rclone uses an
internal one.

scrypt makes it impractical to mount a dictionary attack on rclone
encrypted data. For full protection against this you should always use a
salt.


Dropbox

Paths are specified as remote:path

Dropbox paths may be as deep as required, eg
remote:directory/subdirectory.

The initial setup for dropbox involves getting a token from Dropbox
which you need to do in your browser. rclone config walks you through
it.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

    n) New remote
    d) Delete remote
    q) Quit config
    e/n/d/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Encrypt/Decrypt a remote
       \ "crypt"
     6 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     7 / Google Drive
       \ "drive"
     8 / Hubic
       \ "hubic"
     9 / Local Disk
       \ "local"
    10 / Microsoft OneDrive
       \ "onedrive"
    11 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    12 / SSH/SFTP Connection
       \ "sftp"
    13 / Yandex Disk
       \ "yandex"
    Storage> 4
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

You can then use it like this,

List directories in top level of your dropbox

    rclone lsd remote:

List all the files in your dropbox

    rclone ls remote:

To copy a local directory to a dropbox directory called backup

    rclone copy /home/source remote:backup

Dropbox for business

Rclone supports Dropbox for business and Team Folders.

When using Dropbox for business remote: and remote:path/to/file will
refer to your personal folder.

If you wish to see Team Folders you must use a leading / in the path, so
rclone lsd remote:/ will refer to the root and show you all Team Folders
and your User Folder.

You can then use team folders like this remote:/TeamFolder and
remote:/TeamFolder/path/to/file.

A leading / for a Dropbox personal account will do nothing, but it will
take an extra HTTP transaction so it should be avoided.

Modified time and Hashes

Dropbox supports modified times, but the only way to set a modification
time is to re-upload the file.

This means that if you uploaded your data with an older version of
rclone which didn’t support the v2 API and modified times, rclone will
decide to upload all your old data to fix the modification times. If you
don’t want this to happen use --size-only or --checksum flag to stop it.

Dropbox supports its own hash type which is checked for all transfers.

Standard Options

Here are the standard options specific to dropbox (Dropbox).

–dropbox-client-id

Dropbox App Client Id Leave blank normally.

-   Config: client_id
-   Env Var: RCLONE_DROPBOX_CLIENT_ID
-   Type: string
-   Default: ""

–dropbox-client-secret

Dropbox App Client Secret Leave blank normally.

-   Config: client_secret
-   Env Var: RCLONE_DROPBOX_CLIENT_SECRET
-   Type: string
-   Default: ""

Advanced Options

Here are the advanced options specific to dropbox (Dropbox).

–dropbox-chunk-size

Upload chunk size. (< 150M).

Any files larger than this will be uploaded in chunks of this size.

Note that chunks are buffered in memory (one at a time) so rclone can
deal with retries. Setting this larger will increase the speed slightly
(at most 10% for 128MB in tests) at the cost of using more memory. It
can be set smaller if you are tight on memory.

-   Config: chunk_size
-   Env Var: RCLONE_DROPBOX_CHUNK_SIZE
-   Type: SizeSuffix
-   Default: 48M

–dropbox-impersonate

Impersonate this user when using a business account.

-   Config: impersonate
-   Env Var: RCLONE_DROPBOX_IMPERSONATE
-   Type: string
-   Default: ""

Limitations

Note that Dropbox is case insensitive so you can’t have a file called
“Hello.doc” and one called “hello.doc”.

There are some file names such as thumbs.db which Dropbox can’t store.
There is a full list of them in the “Ignored Files” section of this
document. Rclone will issue an error message
File name disallowed - not uploading if it attempts to upload one of
those file names, but the sync won’t fail.

If you have more than 10,000 files in a directory then
rclone purge dropbox:dir will return the error
Failed to purge: There are too many files involved in this operation. As
a work-around do an rclone delete dropbox:dir followed by an
rclone rmdir dropbox:dir.


FTP

FTP is the File Transfer Protocol. FTP support is provided using the
github.com/jlaffaye/ftp package.

Here is an example of making an FTP configuration. First run

    rclone config

This will guide you through an interactive setup process. An FTP remote
only needs a host together with and a username and a password. With
anonymous FTP server, you will need to use anonymous as username and
your email address as the password.

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
    10 / FTP Connection
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

This remote is called remote and can now be used like this

See all directories in the home directory

    rclone lsd remote:

Make a new directory

    rclone mkdir remote:path/to/directory

List the contents of a directory

    rclone ls remote:path/to/directory

Sync /home/local/directory to the remote directory, deleting any excess
files in the directory.

    rclone sync /home/local/directory remote:directory

Modified time

FTP does not support modified times. Any times you see on the server
will be time of upload.

Checksums

FTP does not support any checksums.

Implicit TLS

FTP supports implicit FTP over TLS servers (FTPS). This has to be
enabled in the config for the remote. The default FTPS port is 990 so
the port will likely have to be explictly set in the config for the
remote.

Standard Options

Here are the standard options specific to ftp (FTP Connection).

–ftp-host

FTP host to connect to

-   Config: host
-   Env Var: RCLONE_FTP_HOST
-   Type: string
-   Default: ""
-   Examples:
    -   “ftp.example.com”
        -   Connect to ftp.example.com

–ftp-user

FTP username, leave blank for current username, $USER

-   Config: user
-   Env Var: RCLONE_FTP_USER
-   Type: string
-   Default: ""

–ftp-port

FTP port, leave blank to use default (21)

-   Config: port
-   Env Var: RCLONE_FTP_PORT
-   Type: string
-   Default: ""

–ftp-pass

FTP password

-   Config: pass
-   Env Var: RCLONE_FTP_PASS
-   Type: string
-   Default: ""

–ftp-tls

Use FTP over TLS (Implicit)

-   Config: tls
-   Env Var: RCLONE_FTP_TLS
-   Type: bool
-   Default: false

Advanced Options

Here are the advanced options specific to ftp (FTP Connection).

–ftp-concurrency

Maximum number of FTP simultaneous connections, 0 for unlimited

-   Config: concurrency
-   Env Var: RCLONE_FTP_CONCURRENCY
-   Type: int
-   Default: 0

–ftp-no-check-certificate

Do not verify the TLS certificate of the server

-   Config: no_check_certificate
-   Env Var: RCLONE_FTP_NO_CHECK_CERTIFICATE
-   Type: bool
-   Default: false

Limitations

Note that since FTP isn’t HTTP based the following flags don’t work with
it: --dump-headers, --dump-bodies, --dump-auth

Note that --timeout isn’t supported (but --contimeout is).

Note that --bind isn’t supported.

FTP could support server side move but doesn’t yet.

Note that the ftp backend does not support the ftp_proxy environment
variable yet.

Note that while implicit FTP over TLS is supported, explicit FTP over
TLS is not.


Google Cloud Storage

Paths are specified as remote:bucket (or remote: for the lsd command.)
You may put subdirectories in too, eg remote:bucket/path/to/dir.

The initial setup for google cloud storage involves getting a token from
Google Cloud Storage which you need to do in your browser. rclone config
walks you through it.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

    n) New remote
    d) Delete remote
    q) Quit config
    e/n/d/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Encrypt/Decrypt a remote
       \ "crypt"
     6 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     7 / Google Drive
       \ "drive"
     8 / Hubic
       \ "hubic"
     9 / Local Disk
       \ "local"
    10 / Microsoft OneDrive
       \ "onedrive"
    11 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    12 / SSH/SFTP Connection
       \ "sftp"
    13 / Yandex Disk
       \ "yandex"
    Storage> 6
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

Note that rclone runs a webserver on your local machine to collect the
token as returned from Google if you use auto config mode. This only
runs from the moment it opens your browser to the moment you get back
the verification code. This is on http://127.0.0.1:53682/ and this it
may require you to unblock it temporarily if you are running a host
firewall, or use manual mode.

This remote is called remote and can now be used like this

See all the buckets in your project

    rclone lsd remote:

Make a new bucket

    rclone mkdir remote:bucket

List the contents of a bucket

    rclone ls remote:bucket

Sync /home/local/directory to the remote bucket, deleting any excess
files in the bucket.

    rclone sync /home/local/directory remote:bucket

Service Account support

You can set up rclone with Google Cloud Storage in an unattended mode,
i.e. not tied to a specific end-user Google account. This is useful when
you want to synchronise files onto machines that don’t have actively
logged-in users, for example build machines.

To get credentials for Google Cloud Platform IAM Service Accounts,
please head to the Service Account section of the Google Developer
Console. Service Accounts behave just like normal User permissions in
Google Cloud Storage ACLs, so you can limit their access (e.g. make them
read only). After creating an account, a JSON file containing the
Service Account’s credentials will be downloaded onto your machines.
These credentials are what rclone will use for authentication.

To use a Service Account instead of OAuth2 token flow, enter the path to
your Service Account credentials at the service_account_file prompt and
rclone won’t use the browser based authentication flow. If you’d rather
stuff the contents of the credentials file into the rclone config file,
you can set service_account_credentials with the actual contents of the
file instead, or set the equivalent environment variable.

Application Default Credentials

If no other source of credentials is provided, rclone will fall back to
Application Default Credentials this is useful both when you already
have configured authentication for your developer account, or in
production when running on a google compute host. Note that if running
in docker, you may need to run additional commands on your google
compute machine - see this page.

Note that in the case application default credentials are used, there is
no need to explicitly configure a project number.

–fast-list

This remote supports --fast-list which allows you to use fewer
transactions in exchange for more memory. See the rclone docs for more
details.

Modified time

Google google cloud storage stores md5sums natively and rclone stores
modification times as metadata on the object, under the “mtime” key in
RFC3339 format accurate to 1ns.

Standard Options

Here are the standard options specific to google cloud storage (Google
Cloud Storage (this is not Google Drive)).

–gcs-client-id

Google Application Client Id Leave blank normally.

-   Config: client_id
-   Env Var: RCLONE_GCS_CLIENT_ID
-   Type: string
-   Default: ""

–gcs-client-secret

Google Application Client Secret Leave blank normally.

-   Config: client_secret
-   Env Var: RCLONE_GCS_CLIENT_SECRET
-   Type: string
-   Default: ""

–gcs-project-number

Project number. Optional - needed only for list/create/delete buckets -
see your developer console.

-   Config: project_number
-   Env Var: RCLONE_GCS_PROJECT_NUMBER
-   Type: string
-   Default: ""

–gcs-service-account-file

Service Account Credentials JSON file path Leave blank normally. Needed
only if you want use SA instead of interactive login.

-   Config: service_account_file
-   Env Var: RCLONE_GCS_SERVICE_ACCOUNT_FILE
-   Type: string
-   Default: ""

–gcs-service-account-credentials

Service Account Credentials JSON blob Leave blank normally. Needed only
if you want use SA instead of interactive login.

-   Config: service_account_credentials
-   Env Var: RCLONE_GCS_SERVICE_ACCOUNT_CREDENTIALS
-   Type: string
-   Default: ""

–gcs-object-acl

Access Control List for new objects.

-   Config: object_acl
-   Env Var: RCLONE_GCS_OBJECT_ACL
-   Type: string
-   Default: ""
-   Examples:
    -   “authenticatedRead”
        -   Object owner gets OWNER access, and all Authenticated Users
            get READER access.
    -   “bucketOwnerFullControl”
        -   Object owner gets OWNER access, and project team owners get
            OWNER access.
    -   “bucketOwnerRead”
        -   Object owner gets OWNER access, and project team owners get
            READER access.
    -   “private”
        -   Object owner gets OWNER access [default if left blank].
    -   “projectPrivate”
        -   Object owner gets OWNER access, and project team members get
            access according to their roles.
    -   “publicRead”
        -   Object owner gets OWNER access, and all Users get READER
            access.

–gcs-bucket-acl

Access Control List for new buckets.

-   Config: bucket_acl
-   Env Var: RCLONE_GCS_BUCKET_ACL
-   Type: string
-   Default: ""
-   Examples:
    -   “authenticatedRead”
        -   Project team owners get OWNER access, and all Authenticated
            Users get READER access.
    -   “private”
        -   Project team owners get OWNER access [default if left
            blank].
    -   “projectPrivate”
        -   Project team members get access according to their roles.
    -   “publicRead”
        -   Project team owners get OWNER access, and all Users get
            READER access.
    -   “publicReadWrite”
        -   Project team owners get OWNER access, and all Users get
            WRITER access.

–gcs-bucket-policy-only

Access checks should use bucket-level IAM policies.

If you want to upload objects to a bucket with Bucket Policy Only set
then you will need to set this.

When it is set, rclone:

-   ignores ACLs set on buckets
-   ignores ACLs set on objects
-   creates buckets with Bucket Policy Only set

Docs: https://cloud.google.com/storage/docs/bucket-policy-only

-   Config: bucket_policy_only
-   Env Var: RCLONE_GCS_BUCKET_POLICY_ONLY
-   Type: bool
-   Default: false

–gcs-location

Location for the newly created buckets.

-   Config: location
-   Env Var: RCLONE_GCS_LOCATION
-   Type: string
-   Default: ""
-   Examples:
    -   ""
        -   Empty for default location (US).
    -   “asia”
        -   Multi-regional location for Asia.
    -   “eu”
        -   Multi-regional location for Europe.
    -   “us”
        -   Multi-regional location for United States.
    -   “asia-east1”
        -   Taiwan.
    -   “asia-east2”
        -   Hong Kong.
    -   “asia-northeast1”
        -   Tokyo.
    -   “asia-south1”
        -   Mumbai.
    -   “asia-southeast1”
        -   Singapore.
    -   “australia-southeast1”
        -   Sydney.
    -   “europe-north1”
        -   Finland.
    -   “europe-west1”
        -   Belgium.
    -   “europe-west2”
        -   London.
    -   “europe-west3”
        -   Frankfurt.
    -   “europe-west4”
        -   Netherlands.
    -   “us-central1”
        -   Iowa.
    -   “us-east1”
        -   South Carolina.
    -   “us-east4”
        -   Northern Virginia.
    -   “us-west1”
        -   Oregon.
    -   “us-west2”
        -   California.

–gcs-storage-class

The storage class to use when storing objects in Google Cloud Storage.

-   Config: storage_class
-   Env Var: RCLONE_GCS_STORAGE_CLASS
-   Type: string
-   Default: ""
-   Examples:
    -   ""
        -   Default
    -   “MULTI_REGIONAL”
        -   Multi-regional storage class
    -   “REGIONAL”
        -   Regional storage class
    -   “NEARLINE”
        -   Nearline storage class
    -   “COLDLINE”
        -   Coldline storage class
    -   “DURABLE_REDUCED_AVAILABILITY”
        -   Durable reduced availability storage class


Google Drive

Paths are specified as drive:path

Drive paths may be as deep as required, eg drive:directory/subdirectory.

The initial setup for drive involves getting a token from Google drive
which you need to do in your browser. rclone config walks you through
it.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

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
    10 / Google Drive
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

Note that rclone runs a webserver on your local machine to collect the
token as returned from Google if you use auto config mode. This only
runs from the moment it opens your browser to the moment you get back
the verification code. This is on http://127.0.0.1:53682/ and this it
may require you to unblock it temporarily if you are running a host
firewall, or use manual mode.

You can then use it like this,

List directories in top level of your drive

    rclone lsd remote:

List all the files in your drive

    rclone ls remote:

To copy a local directory to a drive directory called backup

    rclone copy /home/source remote:backup

Scopes

Rclone allows you to select which scope you would like for rclone to
use. This changes what type of token is granted to rclone. The scopes
are defined here..

The scope are

drive

This is the default scope and allows full access to all files, except
for the Application Data Folder (see below).

Choose this one if you aren’t sure.

drive.readonly

This allows read only access to all files. Files may be listed and
downloaded but not uploaded, renamed or deleted.

drive.file

With this scope rclone can read/view/modify only those files and folders
it creates.

So if you uploaded files to drive via the web interface (or any other
means) they will not be visible to rclone.

This can be useful if you are using rclone to backup data and you want
to be sure confidential data on your drive is not visible to rclone.

Files created with this scope are visible in the web interface.

drive.appfolder

This gives rclone its own private area to store files. Rclone will not
be able to see any other files on your drive and you won’t be able to
see rclone’s files from the web interface either.

drive.metadata.readonly

This allows read only access to file names only. It does not allow
rclone to download or upload data, or rename or delete files or
directories.

Root folder ID

You can set the root_folder_id for rclone. This is the directory
(identified by its Folder ID) that rclone considers to be the root of
your drive.

Normally you will leave this blank and rclone will determine the correct
root to use itself.

However you can set this to restrict rclone to a specific folder
hierarchy or to access data within the “Computers” tab on the drive web
interface (where files from Google’s Backup and Sync desktop program
go).

In order to do this you will have to find the Folder ID of the directory
you wish rclone to display. This will be the last segment of the URL
when you open the relevant folder in the drive web interface.

So if the folder you want rclone to use has a URL which looks like
https://drive.google.com/drive/folders/1XyfxxxxxxxxxxxxxxxxxxxxxxxxxKHCh
in the browser, then you use 1XyfxxxxxxxxxxxxxxxxxxxxxxxxxKHCh as the
root_folder_id in the config.

NB folders under the “Computers” tab seem to be read only (drive gives a
500 error) when using rclone.

There doesn’t appear to be an API to discover the folder IDs of the
“Computers” tab - please contact us if you know otherwise!

Note also that rclone can’t access any data under the “Backups” tab on
the google drive web interface yet.

Service Account support

You can set up rclone with Google Drive in an unattended mode, i.e. not
tied to a specific end-user Google account. This is useful when you want
to synchronise files onto machines that don’t have actively logged-in
users, for example build machines.

To use a Service Account instead of OAuth2 token flow, enter the path to
your Service Account credentials at the service_account_file prompt
during rclone config and rclone won’t use the browser based
authentication flow. If you’d rather stuff the contents of the
credentials file into the rclone config file, you can set
service_account_credentials with the actual contents of the file
instead, or set the equivalent environment variable.

Use case - Google Apps/G-suite account and individual Drive

Let’s say that you are the administrator of a Google Apps (old) or
G-suite account. The goal is to store data on an individual’s Drive
account, who IS a member of the domain. We’ll call the domain
EXAMPLE.COM, and the user FOO@EXAMPLE.COM.

There’s a few steps we need to go through to accomplish this:

1. Create a service account for example.com

-   To create a service account and obtain its credentials, go to the
    Google Developer Console.
-   You must have a project - create one if you don’t.
-   Then go to “IAM & admin” -> “Service Accounts”.
-   Use the “Create Credentials” button. Fill in “Service account name”
    with something that identifies your client. “Role” can be empty.
-   Tick “Furnish a new private key” - select “Key type JSON”.
-   Tick “Enable G Suite Domain-wide Delegation”. This option makes
    “impersonation” possible, as documented here: Delegating domain-wide
    authority to the service account
-   These credentials are what rclone will use for authentication. If
    you ever need to remove access, press the “Delete service account
    key” button.

2. Allowing API access to example.com Google Drive

-   Go to example.com’s admin console
-   Go into “Security” (or use the search bar)
-   Select “Show more” and then “Advanced settings”
-   Select “Manage API client access” in the “Authentication” section
-   In the “Client Name” field enter the service account’s “Client ID” -
    this can be found in the Developer Console under “IAM & Admin” ->
    “Service Accounts”, then “View Client ID” for the newly created
    service account. It is a ~21 character numerical string.
-   In the next field, “One or More API Scopes”, enter
    https://www.googleapis.com/auth/drive to grant access to Google
    Drive specifically.

3. Configure rclone, assuming a new install

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

4. Verify that it’s working

-   rclone -v --drive-impersonate foo@example.com lsf gdrive:backup
-   The arguments do:
    -   -v - verbose logging
    -   --drive-impersonate foo@example.com - this is what does the
        magic, pretending to be user foo.
    -   lsf - list files in a parsing friendly way
    -   gdrive:backup - use the remote called gdrive, work in the folder
        named backup.

Team drives

If you want to configure the remote to point to a Google Team Drive then
answer y to the question Configure this as a team drive?.

This will fetch the list of Team Drives from google and allow you to
configure which one you want to use. You can also type in a team drive
ID if you prefer.

For example:

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

–fast-list

This remote supports --fast-list which allows you to use fewer
transactions in exchange for more memory. See the rclone docs for more
details.

It does this by combining multiple list calls into a single API request.

This works by combining many '%s' in parents filters into one
expression. To list the contents of directories a, b and c, the
following requests will be send by the regular List function:

    trashed=false and 'a' in parents
    trashed=false and 'b' in parents
    trashed=false and 'c' in parents

These can now be combined into a single request:

    trashed=false and ('a' in parents or 'b' in parents or 'c' in parents)

The implementation of ListR will put up to 50 parents filters into one
request. It will use the --checkers value to specify the number of
requests to run in parallel.

In tests, these batch requests were up to 20x faster than the regular
method. Running the following command against different sized folders
gives:

    rclone lsjson -vv -R --checkers=6 gdrive:folder

small folder (220 directories, 700 files):

-   without --fast-list: 38s
-   with --fast-list: 10s

large folder (10600 directories, 39000 files):

-   without --fast-list: 22:05 min
-   with --fast-list: 58s

Modified time

Google drive stores modification times accurate to 1 ms.

Revisions

Google drive stores revisions of files. When you upload a change to an
existing file to google drive using rclone it will create a new revision
of that file.

Revisions follow the standard google policy which at time of writing was

-   They are deleted after 30 days or 100 revisions (whatever comes
    first).
-   They do not count towards a user storage quota.

Deleting files

By default rclone will send all files to the trash when deleting files.
If deleting them permanently is required then use the
--drive-use-trash=false flag, or set the equivalent environment
variable.

Emptying trash

If you wish to empty your trash you can use the rclone cleanup remote:
command which will permanently delete all your trashed files. This
command does not take any path arguments.

Note that Google Drive takes some time (minutes to days) to empty the
trash even though the command returns within a few seconds. No output is
echoed, so there will be no confirmation even using -v or -vv.

Quota information

To view your current quota you can use the rclone about remote: command
which will display your usage limit (quota), the usage in Google Drive,
the size of all files in the Trash and the space used by other Google
services such as Gmail. This command does not take any path arguments.

Import/Export of google documents

Google documents can be exported from and uploaded to Google Drive.

When rclone downloads a Google doc it chooses a format to download
depending upon the --drive-export-formats setting. By default the export
formats are docx,xlsx,pptx,svg which are a sensible default for an
editable document.

When choosing a format, rclone runs down the list provided in order and
chooses the first file format the doc can be exported as from the list.
If the file can’t be exported to a format on the formats list, then
rclone will choose a format from the default list.

If you prefer an archive copy then you might use
--drive-export-formats pdf, or if you prefer openoffice/libreoffice
formats you might use --drive-export-formats ods,odt,odp.

Note that rclone adds the extension to the google doc, so if it is
called My Spreadsheet on google docs, it will be exported as
My Spreadsheet.xlsx or My Spreadsheet.pdf etc.

When importing files into Google Drive, rclone will convert all files
with an extension in --drive-import-formats to their associated document
type. rclone will not convert any files by default, since the conversion
is lossy process.

The conversion must result in a file with the same extension when the
--drive-export-formats rules are applied to the uploaded document.

Here are some examples for allowed and prohibited conversions.

  export-formats   import-formats   Upload Ext   Document Ext   Allowed
  ---------------- ---------------- ------------ -------------- ---------
  odt              odt              odt          odt            Yes
  odt              docx,odt         odt          odt            Yes
                   docx             docx         docx           Yes
                   odt              odt          docx           No
  odt,docx         docx,odt         docx         odt            No
  docx,odt         docx,odt         docx         docx           Yes
  docx,odt         docx,odt         odt          docx           No

This limitation can be disabled by specifying
--drive-allow-import-name-change. When using this flag, rclone can
convert multiple files types resulting in the same document type at
once, eg with --drive-import-formats docx,odt,txt, all files having
these extension would result in a document represented as a docx file.
This brings the additional risk of overwriting a document, if multiple
files have the same stem. Many rclone operations will not handle this
name change in any way. They assume an equal name when copying files and
might copy the file again or delete them when the name changes.

Here are the possible export extensions with their corresponding mime
types. Most of these can also be used for importing, but there more that
are not listed here. Some of these additional ones might only be
available when the operating system provides the correct MIME type
entries.

This list can be changed by Google Drive at any time and might not
represent the currently available conversions.

  --------------------------------------------------------------------------------------------------------------------------
  Extension           Mime Type                                                                   Description
  ------------------- --------------------------------------------------------------------------- --------------------------
  csv                 text/csv                                                                    Standard CSV format for
                                                                                                  Spreadsheets

  docx                application/vnd.openxmlformats-officedocument.wordprocessingml.document     Microsoft Office Document

  epub                application/epub+zip                                                        E-book format

  html                text/html                                                                   An HTML Document

  jpg                 image/jpeg                                                                  A JPEG Image File

  json                application/vnd.google-apps.script+json                                     JSON Text Format

  odp                 application/vnd.oasis.opendocument.presentation                             Openoffice Presentation

  ods                 application/vnd.oasis.opendocument.spreadsheet                              Openoffice Spreadsheet

  ods                 application/x-vnd.oasis.opendocument.spreadsheet                            Openoffice Spreadsheet

  odt                 application/vnd.oasis.opendocument.text                                     Openoffice Document

  pdf                 application/pdf                                                             Adobe PDF Format

  png                 image/png                                                                   PNG Image Format

  pptx                application/vnd.openxmlformats-officedocument.presentationml.presentation   Microsoft Office
                                                                                                  Powerpoint

  rtf                 application/rtf                                                             Rich Text Format

  svg                 image/svg+xml                                                               Scalable Vector Graphics
                                                                                                  Format

  tsv                 text/tab-separated-values                                                   Standard TSV format for
                                                                                                  spreadsheets

  txt                 text/plain                                                                  Plain Text

  xlsx                application/vnd.openxmlformats-officedocument.spreadsheetml.sheet           Microsoft Office
                                                                                                  Spreadsheet

  zip                 application/zip                                                             A ZIP file of HTML, Images
                                                                                                  CSS
  --------------------------------------------------------------------------------------------------------------------------

Google documents can also be exported as link files. These files will
open a browser window for the Google Docs website of that document when
opened. The link file extension has to be specified as a
--drive-export-formats parameter. They will match all available Google
Documents.

  Extension   Description                               OS Support
  ----------- ----------------------------------------- ----------------
  desktop     freedesktop.org specified desktop entry   Linux
  link.html   An HTML Document with a redirect          All
  url         INI style link file                       macOS, Windows
  webloc      macOS specific XML format                 macOS

Standard Options

Here are the standard options specific to drive (Google Drive).

–drive-client-id

Google Application Client Id Setting your own is recommended. See
https://rclone.org/drive/#making-your-own-client-id for how to create
your own. If you leave this blank, it will use an internal key which is
low performance.

-   Config: client_id
-   Env Var: RCLONE_DRIVE_CLIENT_ID
-   Type: string
-   Default: ""

–drive-client-secret

Google Application Client Secret Setting your own is recommended.

-   Config: client_secret
-   Env Var: RCLONE_DRIVE_CLIENT_SECRET
-   Type: string
-   Default: ""

–drive-scope

Scope that rclone should use when requesting access from drive.

-   Config: scope
-   Env Var: RCLONE_DRIVE_SCOPE
-   Type: string
-   Default: ""
-   Examples:
    -   “drive”
        -   Full access all files, excluding Application Data Folder.
    -   “drive.readonly”
        -   Read-only access to file metadata and file contents.
    -   “drive.file”
        -   Access to files created by rclone only.
        -   These are visible in the drive website.
        -   File authorization is revoked when the user deauthorizes the
            app.
    -   “drive.appfolder”
        -   Allows read and write access to the Application Data folder.
        -   This is not visible in the drive website.
    -   “drive.metadata.readonly”
        -   Allows read-only access to file metadata but
        -   does not allow any access to read or download file content.

–drive-root-folder-id

ID of the root folder Leave blank normally. Fill in to access
“Computers” folders. (see docs).

-   Config: root_folder_id
-   Env Var: RCLONE_DRIVE_ROOT_FOLDER_ID
-   Type: string
-   Default: ""

–drive-service-account-file

Service Account Credentials JSON file path Leave blank normally. Needed
only if you want use SA instead of interactive login.

-   Config: service_account_file
-   Env Var: RCLONE_DRIVE_SERVICE_ACCOUNT_FILE
-   Type: string
-   Default: ""

Advanced Options

Here are the advanced options specific to drive (Google Drive).

–drive-service-account-credentials

Service Account Credentials JSON blob Leave blank normally. Needed only
if you want use SA instead of interactive login.

-   Config: service_account_credentials
-   Env Var: RCLONE_DRIVE_SERVICE_ACCOUNT_CREDENTIALS
-   Type: string
-   Default: ""

–drive-team-drive

ID of the Team Drive

-   Config: team_drive
-   Env Var: RCLONE_DRIVE_TEAM_DRIVE
-   Type: string
-   Default: ""

–drive-auth-owner-only

Only consider files owned by the authenticated user.

-   Config: auth_owner_only
-   Env Var: RCLONE_DRIVE_AUTH_OWNER_ONLY
-   Type: bool
-   Default: false

–drive-use-trash

Send files to the trash instead of deleting permanently. Defaults to
true, namely sending files to the trash. Use --drive-use-trash=false to
delete files permanently instead.

-   Config: use_trash
-   Env Var: RCLONE_DRIVE_USE_TRASH
-   Type: bool
-   Default: true

–drive-skip-gdocs

Skip google documents in all listings. If given, gdocs practically
become invisible to rclone.

-   Config: skip_gdocs
-   Env Var: RCLONE_DRIVE_SKIP_GDOCS
-   Type: bool
-   Default: false

–drive-skip-checksum-gphotos

Skip MD5 checksum on Google photos and videos only.

Use this if you get checksum errors when transferring Google photos or
videos.

Setting this flag will cause Google photos and videos to return a blank
MD5 checksum.

Google photos are identifed by being in the “photos” space.

Corrupted checksums are caused by Google modifying the image/video but
not updating the checksum.

-   Config: skip_checksum_gphotos
-   Env Var: RCLONE_DRIVE_SKIP_CHECKSUM_GPHOTOS
-   Type: bool
-   Default: false

–drive-shared-with-me

Only show files that are shared with me.

Instructs rclone to operate on your “Shared with me” folder (where
Google Drive lets you access the files and folders others have shared
with you).

This works both with the “list” (lsd, lsl, etc) and the “copy” commands
(copy, sync, etc), and with all other commands too.

-   Config: shared_with_me
-   Env Var: RCLONE_DRIVE_SHARED_WITH_ME
-   Type: bool
-   Default: false

–drive-trashed-only

Only show files that are in the trash. This will show trashed files in
their original directory structure.

-   Config: trashed_only
-   Env Var: RCLONE_DRIVE_TRASHED_ONLY
-   Type: bool
-   Default: false

–drive-formats

Deprecated: see export_formats

-   Config: formats
-   Env Var: RCLONE_DRIVE_FORMATS
-   Type: string
-   Default: ""

–drive-export-formats

Comma separated list of preferred formats for downloading Google docs.

-   Config: export_formats
-   Env Var: RCLONE_DRIVE_EXPORT_FORMATS
-   Type: string
-   Default: “docx,xlsx,pptx,svg”

–drive-import-formats

Comma separated list of preferred formats for uploading Google docs.

-   Config: import_formats
-   Env Var: RCLONE_DRIVE_IMPORT_FORMATS
-   Type: string
-   Default: ""

–drive-allow-import-name-change

Allow the filetype to change when uploading Google docs (e.g. file.doc
to file.docx). This will confuse sync and reupload every time.

-   Config: allow_import_name_change
-   Env Var: RCLONE_DRIVE_ALLOW_IMPORT_NAME_CHANGE
-   Type: bool
-   Default: false

–drive-use-created-date

Use file created date instead of modified date.,

Useful when downloading data and you want the creation date used in
place of the last modified date.

WARNING: This flag may have some unexpected consequences.

When uploading to your drive all files will be overwritten unless they
haven’t been modified since their creation. And the inverse will occur
while downloading. This side effect can be avoided by using the
“–checksum” flag.

This feature was implemented to retain photos capture date as recorded
by google photos. You will first need to check the “Create a Google
Photos folder” option in your google drive settings. You can then copy
or move the photos locally and use the date the image was taken
(created) set as the modification date.

-   Config: use_created_date
-   Env Var: RCLONE_DRIVE_USE_CREATED_DATE
-   Type: bool
-   Default: false

–drive-list-chunk

Size of listing chunk 100-1000. 0 to disable.

-   Config: list_chunk
-   Env Var: RCLONE_DRIVE_LIST_CHUNK
-   Type: int
-   Default: 1000

–drive-impersonate

Impersonate this user when using a service account.

-   Config: impersonate
-   Env Var: RCLONE_DRIVE_IMPERSONATE
-   Type: string
-   Default: ""

–drive-alternate-export

Use alternate export URLs for google documents export.,

If this option is set this instructs rclone to use an alternate set of
export URLs for drive documents. Users have reported that the official
export URLs can’t export large documents, whereas these unofficial ones
can.

See rclone issue #2243 for background, this google drive issue and this
helpful post.

-   Config: alternate_export
-   Env Var: RCLONE_DRIVE_ALTERNATE_EXPORT
-   Type: bool
-   Default: false

–drive-upload-cutoff

Cutoff for switching to chunked upload

-   Config: upload_cutoff
-   Env Var: RCLONE_DRIVE_UPLOAD_CUTOFF
-   Type: SizeSuffix
-   Default: 8M

–drive-chunk-size

Upload chunk size. Must a power of 2 >= 256k.

Making this larger will improve performance, but note that each chunk is
buffered in memory one per transfer.

Reducing this will reduce memory usage but decrease performance.

-   Config: chunk_size
-   Env Var: RCLONE_DRIVE_CHUNK_SIZE
-   Type: SizeSuffix
-   Default: 8M

–drive-acknowledge-abuse

Set to allow files which return cannotDownloadAbusiveFile to be
downloaded.

If downloading a file returns the error “This file has been identified
as malware or spam and cannot be downloaded” with the error code
“cannotDownloadAbusiveFile” then supply this flag to rclone to indicate
you acknowledge the risks of downloading the file and rclone will
download it anyway.

-   Config: acknowledge_abuse
-   Env Var: RCLONE_DRIVE_ACKNOWLEDGE_ABUSE
-   Type: bool
-   Default: false

–drive-keep-revision-forever

Keep new head revision of each file forever.

-   Config: keep_revision_forever
-   Env Var: RCLONE_DRIVE_KEEP_REVISION_FOREVER
-   Type: bool
-   Default: false

–drive-size-as-quota

Show storage quota usage for file size.

The storage used by a file is the size of the current version plus any
older versions that have been set to keep forever.

-   Config: size_as_quota
-   Env Var: RCLONE_DRIVE_SIZE_AS_QUOTA
-   Type: bool
-   Default: false

–drive-v2-download-min-size

If Object’s are greater, use drive v2 API to download.

-   Config: v2_download_min_size
-   Env Var: RCLONE_DRIVE_V2_DOWNLOAD_MIN_SIZE
-   Type: SizeSuffix
-   Default: off

–drive-pacer-min-sleep

Minimum time to sleep between API calls.

-   Config: pacer_min_sleep
-   Env Var: RCLONE_DRIVE_PACER_MIN_SLEEP
-   Type: Duration
-   Default: 100ms

–drive-pacer-burst

Number of API calls to allow without sleeping.

-   Config: pacer_burst
-   Env Var: RCLONE_DRIVE_PACER_BURST
-   Type: int
-   Default: 100

–drive-server-side-across-configs

Allow server side operations (eg copy) to work across different drive
configs.

This can be useful if you wish to do a server side copy between two
different Google drives. Note that this isn’t enabled by default because
it isn’t easy to tell if it will work beween any two configurations.

-   Config: server_side_across_configs
-   Env Var: RCLONE_DRIVE_SERVER_SIDE_ACROSS_CONFIGS
-   Type: bool
-   Default: false

Limitations

Drive has quite a lot of rate limiting. This causes rclone to be limited
to transferring about 2 files per second only. Individual files may be
transferred much faster at 100s of MBytes/s but lots of small files can
take a long time.

Server side copies are also subject to a separate rate limit. If you see
User rate limit exceeded errors, wait at least 24 hours and retry. You
can disable server side copies with --disable copy to download and
upload the files if you prefer.

Limitations of Google Docs

Google docs will appear as size -1 in rclone ls and as size 0 in
anything which uses the VFS layer, eg rclone mount, rclone serve.

This is because rclone can’t find out the size of the Google docs
without downloading them.

Google docs will transfer correctly with rclone sync, rclone copy etc as
rclone knows to ignore the size when doing the transfer.

However an unfortunate consequence of this is that you can’t download
Google docs using rclone mount - you will get a 0 sized file. If you try
again the doc may gain its correct size and be downloadable.

Duplicated files

Sometimes, for no reason I’ve been able to track down, drive will
duplicate a file that rclone uploads. Drive unlike all the other remotes
can have duplicated files.

Duplicated files cause problems with the syncing and you will see
messages in the log about duplicates.

Use rclone dedupe to fix duplicated files.

Note that this isn’t just a problem with rclone, even Google Photos on
Android duplicates files on drive sometimes.

Rclone appears to be re-copying files it shouldn’t

The most likely cause of this is the duplicated file issue above - run
rclone dedupe and check your logs for duplicate object or directory
messages.

This can also be caused by a delay/caching on google drive’s end when
comparing directory listings. Specifically with team drives used in
combination with –fast-list. Files that were uploaded recently may not
appear on the directory list sent to rclone when using –fast-list.

Waiting a moderate period of time between attempts (estimated to be
approximately 1 hour) and/or not using –fast-list both seem to be
effective in preventing the problem.

Making your own client_id

When you use rclone with Google drive in its default configuration you
are using rclone’s client_id. This is shared between all the rclone
users. There is a global rate limit on the number of queries per second
that each client_id can do set by Google. rclone already has a high
quota and I will continue to make sure it is high enough by contacting
Google.

It is strongly recommended to use your own client ID as the default
rclone ID is heavily used. If you have multiple services running, it is
recommended to use an API key for each service. The default Google quota
is 10 transactions per second so it is recommended to stay under that
number as if you use more than that, it will cause rclone to rate limit
and make things slower.

Here is how to create your own Google Drive client ID for rclone:

1.  Log into the Google API Console with your Google account. It doesn’t
    matter what Google account you use. (It need not be the same account
    as the Google Drive you want to access)

2.  Select a project or create a new project.

3.  Under “ENABLE APIS AND SERVICES” search for “Drive”, and enable the
    then “Google Drive API”.

4.  Click “Credentials” in the left-side panel (not “Create
    credentials”, which opens the wizard), then “Create credentials”,
    then “OAuth client ID”. It will prompt you to set the OAuth consent
    screen product name, if you haven’t set one already.

5.  Choose an application type of “other”, and click “Create”. (the
    default name is fine)

6.  It will show you a client ID and client secret. Use these values in
    rclone config to add a new remote or edit an existing remote.

(Thanks to @balazer on github for these instructions.)


HTTP

The HTTP remote is a read only remote for reading files of a webserver.
The webserver should provide file listings which rclone will read and
turn into a remote. This has been tested with common webservers such as
Apache/Nginx/Caddy and will likely work with file listings from most web
servers. (If it doesn’t then please file an issue, or send a pull
request!)

Paths are specified as remote: or remote:path/to/dir.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Encrypt/Decrypt a remote
       \ "crypt"
     6 / FTP Connection
       \ "ftp"
     7 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     8 / Google Drive
       \ "drive"
     9 / Hubic
       \ "hubic"
    10 / Local Disk
       \ "local"
    11 / Microsoft OneDrive
       \ "onedrive"
    12 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    13 / SSH/SFTP Connection
       \ "sftp"
    14 / Yandex Disk
       \ "yandex"
    15 / http Connection
       \ "http"
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

This remote is called remote and can now be used like this

See all the top level directories

    rclone lsd remote:

List the contents of a directory

    rclone ls remote:directory

Sync the remote directory to /home/local/directory, deleting any excess
files.

    rclone sync remote:directory /home/local/directory

Read only

This remote is read only - you can’t upload files to an HTTP server.

Modified time

Most HTTP servers store time accurate to 1 second.

Checksum

No checksums are stored.

Usage without a config file

Since the http remote only has one config parameter it is easy to use
without a config file:

    rclone lsd --http-url https://beta.rclone.org :http:

Standard Options

Here are the standard options specific to http (http Connection).

–http-url

URL of http host to connect to

-   Config: url
-   Env Var: RCLONE_HTTP_URL
-   Type: string
-   Default: ""
-   Examples:
    -   “https://example.com”
        -   Connect to example.com
    -   “https://user:pass@example.com”
        -   Connect to example.com using a username and password

Advanced Options

Here are the advanced options specific to http (http Connection).

–http-no-slash

Set this if the site doesn’t end directories with /

Use this if your target website does not use / on the end of
directories.

A / on the end of a path is how rclone normally tells the difference
between files and directories. If this flag is set, then rclone will
treat all files with Content-Type: text/html as directories and read
URLs from them rather than downloading them.

Note that this may cause rclone to confuse genuine HTML files with
directories.

-   Config: no_slash
-   Env Var: RCLONE_HTTP_NO_SLASH
-   Type: bool
-   Default: false


Hubic

Paths are specified as remote:path

Paths are specified as remote:container (or remote: for the lsd
command.) You may put subdirectories in too, eg
remote:container/path/to/dir.

The initial setup for Hubic involves getting a token from Hubic which
you need to do in your browser. rclone config walks you through it.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

    n) New remote
    s) Set configuration password
    n/s> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Encrypt/Decrypt a remote
       \ "crypt"
     6 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     7 / Google Drive
       \ "drive"
     8 / Hubic
       \ "hubic"
     9 / Local Disk
       \ "local"
    10 / Microsoft OneDrive
       \ "onedrive"
    11 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    12 / SSH/SFTP Connection
       \ "sftp"
    13 / Yandex Disk
       \ "yandex"
    Storage> 8
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

See the remote setup docs for how to set it up on a machine with no
Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Hubic. This only runs from the moment it opens
your browser to the moment you get back the verification code. This is
on http://127.0.0.1:53682/ and this it may require you to unblock it
temporarily if you are running a host firewall.

Once configured you can then use rclone like this,

List containers in the top level of your Hubic

    rclone lsd remote:

List all the files in your Hubic

    rclone ls remote:

To copy a local directory to an Hubic directory called backup

    rclone copy /home/source remote:backup

If you want the directory to be visible in the official _Hubic browser_,
you need to copy your files to the default directory

    rclone copy /home/source remote:default/backup

–fast-list

This remote supports --fast-list which allows you to use fewer
transactions in exchange for more memory. See the rclone docs for more
details.

Modified time

The modified time is stored as metadata on the object as
X-Object-Meta-Mtime as floating point since the epoch accurate to 1 ns.

This is a de facto standard (used in the official python-swiftclient
amongst others) for storing the modification time for an object.

Note that Hubic wraps the Swift backend, so most of the properties of
are the same.

Standard Options

Here are the standard options specific to hubic (Hubic).

–hubic-client-id

Hubic Client Id Leave blank normally.

-   Config: client_id
-   Env Var: RCLONE_HUBIC_CLIENT_ID
-   Type: string
-   Default: ""

–hubic-client-secret

Hubic Client Secret Leave blank normally.

-   Config: client_secret
-   Env Var: RCLONE_HUBIC_CLIENT_SECRET
-   Type: string
-   Default: ""

Advanced Options

Here are the advanced options specific to hubic (Hubic).

–hubic-chunk-size

Above this size files will be chunked into a _segments container.

Above this size files will be chunked into a _segments container. The
default for this is 5GB which is its maximum value.

-   Config: chunk_size
-   Env Var: RCLONE_HUBIC_CHUNK_SIZE
-   Type: SizeSuffix
-   Default: 5G

–hubic-no-chunk

Don’t chunk files during streaming upload.

When doing streaming uploads (eg using rcat or mount) setting this flag
will cause the swift backend to not upload chunked files.

This will limit the maximum upload size to 5GB. However non chunked
files are easier to deal with and have an MD5SUM.

Rclone will still chunk files bigger than chunk_size when doing normal
copy operations.

-   Config: no_chunk
-   Env Var: RCLONE_HUBIC_NO_CHUNK
-   Type: bool
-   Default: false

Limitations

This uses the normal OpenStack Swift mechanism to refresh the Swift API
credentials and ignores the expires field returned by the Hubic API.

The Swift API doesn’t return a correct MD5SUM for segmented files
(Dynamic or Static Large Objects) so rclone won’t check or use the
MD5SUM for these.


Jottacloud

Paths are specified as remote:path

Paths may be as deep as required, eg remote:directory/subdirectory.

To configure Jottacloud you will need to enter your username and
password and select a mountpoint.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

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
    14 / JottaCloud
       \ "jottacloud"
    [snip]
    Storage> jottacloud
    ** See help for jottacloud backend at: https://rclone.org/jottacloud/ **

    User Name:
    Enter a string value. Press Enter for the default ("").
    user> user@email.tld
    Edit advanced config? (y/n)
    y) Yes
    n) No
    y/n> n
    Remote config

    Do you want to create a machine specific API key?

    Rclone has it's own Jottacloud API KEY which works fine as long as one only uses rclone on a single machine. When you want to use rclone with this account on more than one machine it's recommended to create a machine specific API key. These keys can NOT be shared between machines.

    y) Yes
    n) No
    y/n> y
    Your Jottacloud password is only required during setup and will not be stored.
    password:

    Do you want to use a non standard device/mountpoint e.g. for accessing files uploaded using the official Jottacloud client?

    y) Yes
    n) No
    y/n> y
    Please select the device to use. Normally this will be Jotta
    Choose a number from below, or type in an existing value
     1 > DESKTOP-3H31129
     2 > test1
     3 > Jotta
    Devices> 3
    Please select the mountpoint to user. Normally this will be Archive
    Choose a number from below, or type in an existing value
     1 > Archive
     2 > Shared
     3 > Sync
    Mountpoints> 1
    --------------------
    [jotta]
    type = jottacloud
    user = 0xC4KE@gmail.com
    client_id = .....
    client_secret = ........
    token = {........}
    device = Jotta
    mountpoint = Archive
    --------------------
    y) Yes this is OK
    e) Edit this remote
    d) Delete this remote
    y/e/d> y

Once configured you can then use rclone like this,

List directories in top level of your Jottacloud

    rclone lsd remote:

List all the files in your Jottacloud

    rclone ls remote:

To copy a local directory to an Jottacloud directory called backup

    rclone copy /home/source remote:backup

Devices and Mountpoints

The official Jottacloud client registers a device for each computer you
install it on and then creates a mountpoint for each folder you select
for Backup. The web interface uses a special device called Jotta for the
Archive, Sync and Shared mountpoints. In most cases you’ll want to use
the Jotta/Archive device/mounpoint however if you want to access files
uploaded by the official rclone provides the option to select other
devices and mountpoints during config.

–fast-list

This remote supports --fast-list which allows you to use fewer
transactions in exchange for more memory. See the rclone docs for more
details.

Note that the implementation in Jottacloud always uses only a single API
request to get the entire list, so for large folders this could lead to
long wait time before the first results are shown.

Modified time and hashes

Jottacloud allows modification times to be set on objects accurate to 1
second. These will be used to detect whether objects need syncing or
not.

Jottacloud supports MD5 type hashes, so you can use the --checksum flag.

Note that Jottacloud requires the MD5 hash before upload so if the
source does not have an MD5 checksum then the file will be cached
temporarily on disk (wherever the TMPDIR environment variable points to)
before it is uploaded. Small files will be cached in memory - see the
--jottacloud-md5-memory-limit flag.

Deleting files

By default rclone will send all files to the trash when deleting files.
Due to a lack of API documentation emptying the trash is currently only
possible via the Jottacloud website. If deleting permanently is required
then use the --jottacloud-hard-delete flag, or set the equivalent
environment variable.

Versions

Jottacloud supports file versioning. When rclone uploads a new version
of a file it creates a new version of it. Currently rclone only supports
retrieving the current version but older versions can be accessed via
the Jottacloud Website.

Quota information

To view your current quota you can use the rclone about remote: command
which will display your usage limit (unless it is unlimited) and the
current usage.

Device IDs

Jottacloud requires each ‘device’ to be registered. Rclone brings such a
registration to easily access your account but if you want to use
Jottacloud together with rclone on multiple machines you NEED to create
a seperate deviceID/deviceSecrect on each machine. You will asked during
setting up the remote. Please be aware that this also means that copying
the rclone config from one machine to another does NOT work with
Jottacloud accounts. You have to create it on each machine.

Standard Options

Here are the standard options specific to jottacloud (JottaCloud).

–jottacloud-user

User Name:

-   Config: user
-   Env Var: RCLONE_JOTTACLOUD_USER
-   Type: string
-   Default: ""

Advanced Options

Here are the advanced options specific to jottacloud (JottaCloud).

–jottacloud-md5-memory-limit

Files bigger than this will be cached on disk to calculate the MD5 if
required.

-   Config: md5_memory_limit
-   Env Var: RCLONE_JOTTACLOUD_MD5_MEMORY_LIMIT
-   Type: SizeSuffix
-   Default: 10M

–jottacloud-hard-delete

Delete files permanently rather than putting them into the trash.

-   Config: hard_delete
-   Env Var: RCLONE_JOTTACLOUD_HARD_DELETE
-   Type: bool
-   Default: false

–jottacloud-unlink

Remove existing public link to file/folder with link command rather than
creating. Default is false, meaning link command will create or retrieve
public link.

-   Config: unlink
-   Env Var: RCLONE_JOTTACLOUD_UNLINK
-   Type: bool
-   Default: false

–jottacloud-upload-resume-limit

Files bigger than this can be resumed if the upload fail’s.

-   Config: upload_resume_limit
-   Env Var: RCLONE_JOTTACLOUD_UPLOAD_RESUME_LIMIT
-   Type: SizeSuffix
-   Default: 10M

Limitations

Note that Jottacloud is case insensitive so you can’t have a file called
“Hello.doc” and one called “hello.doc”.

There are quite a few characters that can’t be in Jottacloud file names.
Rclone will map these names to and from an identical looking unicode
equivalent. For example if a file has a ? in it will be mapped to ？
instead.

Jottacloud only supports filenames up to 255 characters in length.

Troubleshooting

Jottacloud exhibits some inconsistent behaviours regarding deleted files
and folders which may cause Copy, Move and DirMove operations to
previously deleted paths to fail. Emptying the trash should help in such
cases.


Koofr

Paths are specified as remote:path

Paths may be as deep as required, eg remote:directory/subdirectory.

The initial setup for Koofr involves creating an application password
for rclone. You can do that by opening the Koofr web application, giving
the password a nice name like rclone and clicking on generate.

Here is an example of how to make a remote called koofr. First run:

     rclone config

This will guide you through an interactive setup process:

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n
    name> koofr 
    Type of storage to configure.
    Enter a string value. Press Enter for the default ("").
    Choose a number from below, or type in your own value
     1 / A stackable unification remote, which can appear to merge the contents of several remotes
       \ "union"
     2 / Alias for an existing remote
       \ "alias"
     3 / Amazon Drive
       \ "amazon cloud drive"
     4 / Amazon S3 Compliant Storage Provider (AWS, Alibaba, Ceph, Digital Ocean, Dreamhost, IBM COS, Minio, etc)
       \ "s3"
     5 / Backblaze B2
       \ "b2"
     6 / Box
       \ "box"
     7 / Cache a remote
       \ "cache"
     8 / Dropbox
       \ "dropbox"
     9 / Encrypt/Decrypt a remote
       \ "crypt"
    10 / FTP Connection
       \ "ftp"
    11 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
    12 / Google Drive
       \ "drive"
    13 / Hubic
       \ "hubic"
    14 / JottaCloud
       \ "jottacloud"
    15 / Koofr
       \ "koofr"
    16 / Local Disk
       \ "local"
    17 / Mega
       \ "mega"
    18 / Microsoft Azure Blob Storage
       \ "azureblob"
    19 / Microsoft OneDrive
       \ "onedrive"
    20 / OpenDrive
       \ "opendrive"
    21 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    22 / Pcloud
       \ "pcloud"
    23 / QingCloud Object Storage
       \ "qingstor"
    24 / SSH/SFTP Connection
       \ "sftp"
    25 / Webdav
       \ "webdav"
    26 / Yandex Disk
       \ "yandex"
    27 / http Connection
       \ "http"
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

You can choose to edit advanced config in order to enter your own
service URL if you use an on-premise or white label Koofr instance, or
choose an alternative mount instead of your primary storage.

Once configured you can then use rclone like this,

List directories in top level of your Koofr

    rclone lsd koofr:

List all the files in your Koofr

    rclone ls koofr:

To copy a local directory to an Koofr directory called backup

    rclone copy /home/source remote:backup

Standard Options

Here are the standard options specific to koofr (Koofr).

–koofr-user

Your Koofr user name

-   Config: user
-   Env Var: RCLONE_KOOFR_USER
-   Type: string
-   Default: ""

–koofr-password

Your Koofr password for rclone (generate one at
https://app.koofr.net/app/admin/preferences/password)

-   Config: password
-   Env Var: RCLONE_KOOFR_PASSWORD
-   Type: string
-   Default: ""

Advanced Options

Here are the advanced options specific to koofr (Koofr).

–koofr-endpoint

The Koofr API endpoint to use

-   Config: endpoint
-   Env Var: RCLONE_KOOFR_ENDPOINT
-   Type: string
-   Default: “https://app.koofr.net”

–koofr-mountid

Mount ID of the mount to use. If omitted, the primary mount is used.

-   Config: mountid
-   Env Var: RCLONE_KOOFR_MOUNTID
-   Type: string
-   Default: ""

Limitations

Note that Koofr is case insensitive so you can’t have a file called
“Hello.doc” and one called “hello.doc”.


Mega

Mega is a cloud storage and file hosting service known for its security
feature where all files are encrypted locally before they are uploaded.
This prevents anyone (including employees of Mega) from accessing the
files without knowledge of the key used for encryption.

This is an rclone backend for Mega which supports the file transfer
features of Mega using the same client side encryption.

Paths are specified as remote:path

Paths may be as deep as required, eg remote:directory/subdirectory.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Alias for an existing remote
       \ "alias"
    [snip]
    14 / Mega
       \ "mega"
    [snip]
    23 / http Connection
       \ "http"
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

NOTE: The encryption keys need to have been already generated after a
regular login via the browser, otherwise attempting to use the
credentials in rclone will fail.

Once configured you can then use rclone like this,

List directories in top level of your Mega

    rclone lsd remote:

List all the files in your Mega

    rclone ls remote:

To copy a local directory to an Mega directory called backup

    rclone copy /home/source remote:backup

Modified time and hashes

Mega does not support modification times or hashes yet.

Duplicated files

Mega can have two files with exactly the same name and path (unlike a
normal file system).

Duplicated files cause problems with the syncing and you will see
messages in the log about duplicates.

Use rclone dedupe to fix duplicated files.

Failure to log-in

Mega remotes seem to get blocked (reject logins) under “heavy use”. We
haven’t worked out the exact blocking rules but it seems to be related
to fast paced, sucessive rclone commands.

For example, executing this command 90 times in a row
rclone link remote:file will cause the remote to become “blocked”. This
is not an abnormal situation, for example if you wish to get the public
links of a directory with hundred of files… After more or less a week,
the remote will remote accept rclone logins normally again.

You can mitigate this issue by mounting the remote it with rclone mount.
This will log-in when mounting and a log-out when unmounting only. You
can also run rclone rcd and then use rclone rc to run the commands over
the API to avoid logging in each time.

Rclone does not currently close mega sessions (you can see them in the
web interface), however closing the sessions does not solve the issue.

If you space rclone commands by 3 seconds it will avoid blocking the
remote. We haven’t identified the exact blocking rules, so perhaps one
could execute the command 80 times without waiting and avoid blocking by
waiting 3 seconds, then continuing…

Note that this has been observed by trial and error and might not be set
in stone.

Other tools seem not to produce this blocking effect, as they use a
different working approach (state-based, using sessionIDs instead of
log-in) which isn’t compatible with the current stateless rclone
approach.

Note that once blocked, the use of other tools (such as megacmd) is not
a sure workaround: following megacmd login times have been observed in
sucession for blocked remote: 7 minutes, 20 min, 30min, 30 min, 30min.
Web access looks unaffected though.

Investigation is continuing in relation to workarounds based on
timeouts, pacers, retrials and tpslimits - if you discover something
relevant, please post on the forum.

So, if rclone was working nicely and suddenly you are unable to log-in
and you are sure the user and the password are correct, likely you have
got the remote blocked for a while.

Standard Options

Here are the standard options specific to mega (Mega).

–mega-user

User name

-   Config: user
-   Env Var: RCLONE_MEGA_USER
-   Type: string
-   Default: ""

–mega-pass

Password.

-   Config: pass
-   Env Var: RCLONE_MEGA_PASS
-   Type: string
-   Default: ""

Advanced Options

Here are the advanced options specific to mega (Mega).

–mega-debug

Output more debug from Mega.

If this flag is set (along with -vv) it will print further debugging
information from the mega backend.

-   Config: debug
-   Env Var: RCLONE_MEGA_DEBUG
-   Type: bool
-   Default: false

–mega-hard-delete

Delete files permanently rather than putting them into the trash.

Normally the mega backend will put all deletions into the trash rather
than permanently deleting them. If you specify this then rclone will
permanently delete objects instead.

-   Config: hard_delete
-   Env Var: RCLONE_MEGA_HARD_DELETE
-   Type: bool
-   Default: false

Limitations

This backend uses the go-mega go library which is an opensource go
library implementing the Mega API. There doesn’t appear to be any
documentation for the mega protocol beyond the mega C++ SDK source code
so there are likely quite a few errors still remaining in this library.

Mega allows duplicate files which may confuse rclone.


Microsoft Azure Blob Storage

Paths are specified as remote:container (or remote: for the lsd
command.) You may put subdirectories in too, eg
remote:container/path/to/dir.

Here is an example of making a Microsoft Azure Blob Storage
configuration. For a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Box
       \ "box"
     5 / Dropbox
       \ "dropbox"
     6 / Encrypt/Decrypt a remote
       \ "crypt"
     7 / FTP Connection
       \ "ftp"
     8 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     9 / Google Drive
       \ "drive"
    10 / Hubic
       \ "hubic"
    11 / Local Disk
       \ "local"
    12 / Microsoft Azure Blob Storage
       \ "azureblob"
    13 / Microsoft OneDrive
       \ "onedrive"
    14 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    15 / SSH/SFTP Connection
       \ "sftp"
    16 / Yandex Disk
       \ "yandex"
    17 / http Connection
       \ "http"
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

See all containers

    rclone lsd remote:

Make a new container

    rclone mkdir remote:container

List the contents of a container

    rclone ls remote:container

Sync /home/local/directory to the remote container, deleting any excess
files in the container.

    rclone sync /home/local/directory remote:container

–fast-list

This remote supports --fast-list which allows you to use fewer
transactions in exchange for more memory. See the rclone docs for more
details.

Modified time

The modified time is stored as metadata on the object with the mtime
key. It is stored using RFC3339 Format time with nanosecond precision.
The metadata is supplied during directory listings so there is no
overhead to using it.

Hashes

MD5 hashes are stored with blobs. However blobs that were uploaded in
chunks only have an MD5 if the source remote was capable of MD5 hashes,
eg the local disk.

Authenticating with Azure Blob Storage

Rclone has 3 ways of authenticating with Azure Blob Storage:

Account and Key

This is the most straight forward and least flexible way. Just fill in
the account and key lines and leave the rest blank.

SAS URL

This can be an account level SAS URL or container level SAS URL

To use it leave account, key blank and fill in sas_url.

Account level SAS URL or container level SAS URL can be obtained from
Azure portal or Azure Storage Explorer. To get a container level SAS URL
right click on a container in the Azure Blob explorer in the Azure
portal.

If You use container level SAS URL, rclone operations are permitted only
on particular container, eg

    rclone ls azureblob:container or rclone ls azureblob:

Since container name already exists in SAS URL, you can leave it empty
as well.

However these will not work

    rclone lsd azureblob:
    rclone ls azureblob:othercontainer

This would be useful for temporarily allowing third parties access to a
single container or putting credentials into an untrusted environment.

Multipart uploads

Rclone supports multipart uploads with Azure Blob storage. Files bigger
than 256MB will be uploaded using chunked upload by default.

The files will be uploaded in parallel in 4MB chunks (by default). Note
that these chunks are buffered in memory and there may be up to
--transfers of them being uploaded at once.

Files can’t be split into more than 50,000 chunks so by default, so the
largest file that can be uploaded with 4MB chunk size is 195GB. Above
this rclone will double the chunk size until it creates less than 50,000
chunks. By default this will mean a maximum file size of 3.2TB can be
uploaded. This can be raised to 5TB using --azureblob-chunk-size 100M.

Note that rclone doesn’t commit the block list until the end of the
upload which means that there is a limit of 9.5TB of multipart uploads
in progress as Azure won’t allow more than that amount of uncommitted
blocks.

Standard Options

Here are the standard options specific to azureblob (Microsoft Azure
Blob Storage).

–azureblob-account

Storage Account Name (leave blank to use connection string or SAS URL)

-   Config: account
-   Env Var: RCLONE_AZUREBLOB_ACCOUNT
-   Type: string
-   Default: ""

–azureblob-key

Storage Account Key (leave blank to use connection string or SAS URL)

-   Config: key
-   Env Var: RCLONE_AZUREBLOB_KEY
-   Type: string
-   Default: ""

–azureblob-sas-url

SAS URL for container level access only (leave blank if using
account/key or connection string)

-   Config: sas_url
-   Env Var: RCLONE_AZUREBLOB_SAS_URL
-   Type: string
-   Default: ""

Advanced Options

Here are the advanced options specific to azureblob (Microsoft Azure
Blob Storage).

–azureblob-endpoint

Endpoint for the service Leave blank normally.

-   Config: endpoint
-   Env Var: RCLONE_AZUREBLOB_ENDPOINT
-   Type: string
-   Default: ""

–azureblob-upload-cutoff

Cutoff for switching to chunked upload (<= 256MB).

-   Config: upload_cutoff
-   Env Var: RCLONE_AZUREBLOB_UPLOAD_CUTOFF
-   Type: SizeSuffix
-   Default: 256M

–azureblob-chunk-size

Upload chunk size (<= 100MB).

Note that this is stored in memory and there may be up to “–transfers”
chunks stored at once in memory.

-   Config: chunk_size
-   Env Var: RCLONE_AZUREBLOB_CHUNK_SIZE
-   Type: SizeSuffix
-   Default: 4M

–azureblob-list-chunk

Size of blob list.

This sets the number of blobs requested in each listing chunk. Default
is the maximum, 5000. “List blobs” requests are permitted 2 minutes per
megabyte to complete. If an operation is taking longer than 2 minutes
per megabyte on average, it will time out ( source ). This can be used
to limit the number of blobs items to return, to avoid the time out.

-   Config: list_chunk
-   Env Var: RCLONE_AZUREBLOB_LIST_CHUNK
-   Type: int
-   Default: 5000

–azureblob-access-tier

Access tier of blob: hot, cool or archive.

Archived blobs can be restored by setting access tier to hot or cool.
Leave blank if you intend to use default access tier, which is set at
account level

If there is no “access tier” specified, rclone doesn’t apply any tier.
rclone performs “Set Tier” operation on blobs while uploading, if
objects are not modified, specifying “access tier” to new one will have
no effect. If blobs are in “archive tier” at remote, trying to perform
data transfer operations from remote will not be allowed. User should
first restore by tiering blob to “Hot” or “Cool”.

-   Config: access_tier
-   Env Var: RCLONE_AZUREBLOB_ACCESS_TIER
-   Type: string
-   Default: ""

Limitations

MD5 sums are only uploaded with chunked files if the source has an MD5
sum. This will always be the case for a local to azure copy.


Microsoft OneDrive

Paths are specified as remote:path

Paths may be as deep as required, eg remote:directory/subdirectory.

The initial setup for OneDrive involves getting a token from Microsoft
which you need to do in your browser. rclone config walks you through
it.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

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
    ...
    18 / Microsoft OneDrive
       \ "onedrive"
    ...
    Storage> 18
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

See the remote setup docs for how to set it up on a machine with no
Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Microsoft. This only runs from the moment it
opens your browser to the moment you get back the verification code.
This is on http://127.0.0.1:53682/ and this it may require you to
unblock it temporarily if you are running a host firewall.

Once configured you can then use rclone like this,

List directories in top level of your OneDrive

    rclone lsd remote:

List all the files in your OneDrive

    rclone ls remote:

To copy a local directory to an OneDrive directory called backup

    rclone copy /home/source remote:backup

Getting your own Client ID and Key

rclone uses a pair of Client ID and Key shared by all rclone users when
performing requests by default. If you are having problems with them
(E.g., seeing a lot of throttling), you can get your own Client ID and
Key by following the steps below:

1.  Open https://apps.dev.microsoft.com/#/appList, then click Add an app
    (Choose Converged applications if applicable)
2.  Enter a name for your app, and click continue. Copy and keep the
    Application Id under the app name for later use.
3.  Under section Application Secrets, click Generate New Password. Copy
    and keep that password for later use.
4.  Under section Platforms, click Add platform, then Web. Enter
    http://localhost:53682/ in Redirect URLs.
5.  Under section Microsoft Graph Permissions, Add these
    delegated permissions: Files.Read, Files.ReadWrite, Files.Read.All,
    Files.ReadWrite.All, offline_access, User.Read.
6.  Scroll to the bottom and click Save.

Now the application is complete. Run rclone config to create or edit a
OneDrive remote. Supply the app ID and password as Client ID and Secret,
respectively. rclone will walk you through the remaining steps.

Modified time and hashes

OneDrive allows modification times to be set on objects accurate to 1
second. These will be used to detect whether objects need syncing or
not.

OneDrive personal supports SHA1 type hashes. OneDrive for business and
Sharepoint Server support QuickXorHash.

For all types of OneDrive you can use the --checksum flag.

Deleting files

Any files you delete with rclone will end up in the trash. Microsoft
doesn’t provide an API to permanently delete files, nor to empty the
trash, so you will have to do that with one of Microsoft’s apps or via
the OneDrive website.

Standard Options

Here are the standard options specific to onedrive (Microsoft OneDrive).

–onedrive-client-id

Microsoft App Client Id Leave blank normally.

-   Config: client_id
-   Env Var: RCLONE_ONEDRIVE_CLIENT_ID
-   Type: string
-   Default: ""

–onedrive-client-secret

Microsoft App Client Secret Leave blank normally.

-   Config: client_secret
-   Env Var: RCLONE_ONEDRIVE_CLIENT_SECRET
-   Type: string
-   Default: ""

Advanced Options

Here are the advanced options specific to onedrive (Microsoft OneDrive).

–onedrive-chunk-size

Chunk size to upload files with - must be multiple of 320k.

Above this size files will be chunked - must be multiple of 320k. Note
that the chunks will be buffered into memory.

-   Config: chunk_size
-   Env Var: RCLONE_ONEDRIVE_CHUNK_SIZE
-   Type: SizeSuffix
-   Default: 10M

–onedrive-drive-id

The ID of the drive to use

-   Config: drive_id
-   Env Var: RCLONE_ONEDRIVE_DRIVE_ID
-   Type: string
-   Default: ""

–onedrive-drive-type

The type of the drive ( personal | business | documentLibrary )

-   Config: drive_type
-   Env Var: RCLONE_ONEDRIVE_DRIVE_TYPE
-   Type: string
-   Default: ""

–onedrive-expose-onenote-files

Set to make OneNote files show up in directory listings.

By default rclone will hide OneNote files in directory listings because
operations like “Open” and “Update” won’t work on them. But this
behaviour may also prevent you from deleting them. If you want to delete
OneNote files or otherwise want them to show up in directory listing,
set this option.

-   Config: expose_onenote_files
-   Env Var: RCLONE_ONEDRIVE_EXPOSE_ONENOTE_FILES
-   Type: bool
-   Default: false

Limitations

Note that OneDrive is case insensitive so you can’t have a file called
“Hello.doc” and one called “hello.doc”.

There are quite a few characters that can’t be in OneDrive file names.
These can’t occur on Windows platforms, but on non-Windows platforms
they are common. Rclone will map these names to and from an identical
looking unicode equivalent. For example if a file has a ? in it will be
mapped to ？ instead.

The largest allowed file sizes are 15GB for OneDrive for Business and
35GB for OneDrive Personal (Updated 4 Jan 2019).

The entire path, including the file name, must contain fewer than 400
characters for OneDrive, OneDrive for Business and SharePoint Online. If
you are encrypting file and folder names with rclone, you may want to
pay attention to this limitation because the encrypted names are
typically longer than the original ones.

OneDrive seems to be OK with at least 50,000 files in a folder, but at
100,000 rclone will get errors listing the directory like
couldn’t list files: UnknownError:. See #2707 for more info.

An official document about the limitations for different types of
OneDrive can be found here.

Versioning issue

Every change in OneDrive causes the service to create a new version.
This counts against a users quota. For example changing the modification
time of a file creates a second version, so the file is using twice the
space.

The copy is the only rclone command affected by this as we copy the file
and then afterwards set the modification time to match the source file.

NOTE: Starting October 2018, users will no longer be able to disable
versioning by default. This is because Microsoft has brought an update
to the mechanism. To change this new default setting, a PowerShell
command is required to be run by a SharePoint admin. If you are an
admin, you can run these commands in PowerShell to change that setting:

1.  Install-Module -Name Microsoft.Online.SharePoint.PowerShell (in case
    you haven’t installed this already)
2.  Import-Module Microsoft.Online.SharePoint.PowerShell -DisableNameChecking
3.  Connect-SPOService -Url https://YOURSITE-admin.sharepoint.com -Credential YOU@YOURSITE.COM
    (replacing YOURSITE, YOU, YOURSITE.COM with the actual values; this
    will prompt for your credentials)
4.  Set-SPOTenant -EnableMinimumVersionRequirement $False
5.  Disconnect-SPOService (to disconnect from the server)

_Below are the steps for normal users to disable versioning. If you
don’t see the “No Versioning” option, make sure the above requirements
are met._

User Weropol has found a method to disable versioning on OneDrive

1.  Open the settings menu by clicking on the gear symbol at the top of
    the OneDrive Business page.
2.  Click Site settings.
3.  Once on the Site settings page, navigate to Site Administration >
    Site libraries and lists.
4.  Click Customize “Documents”.
5.  Click General Settings > Versioning Settings.
6.  Under Document Version History select the option No versioning.
    Note: This will disable the creation of new file versions, but will
    not remove any previous versions. Your documents are safe.
7.  Apply the changes by clicking OK.
8.  Use rclone to upload or modify files. (I also use the
    –no-update-modtime flag)
9.  Restore the versioning settings after using rclone. (Optional)

Troubleshooting

    Error: access_denied
    Code: AADSTS65005
    Description: Using application 'rclone' is currently not supported for your organization [YOUR_ORGANIZATION] because it is in an unmanaged state. An administrator needs to claim ownership of the company by DNS validation of [YOUR_ORGANIZATION] before the application rclone can be provisioned.

This means that rclone can’t use the OneDrive for Business API with your
account. You can’t do much about it, maybe write an email to your
admins.

However, there are other ways to interact with your OneDrive account.
Have a look at the webdav backend: https://rclone.org/webdav/#sharepoint

    Error: invalid_grant
    Code: AADSTS50076
    Description: Due to a configuration change made by your administrator, or because you moved to a new location, you must use multi-factor authentication to access '...'.

If you see the error above after enabling multi-factor authentication
for your account, you can fix it by refreshing your OAuth refresh token.
To do that, run rclone config, and choose to edit your OneDrive backend.
Then, you don’t need to actually make any changes until you reach this
question: Already have a token - refresh?. For this question, answer y
and go through the process to refresh your token, just like the first
time the backend is configured. After this, rclone should work again for
this backend.


OpenDrive

Paths are specified as remote:path

Paths may be as deep as required, eg remote:directory/subdirectory.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

    n) New remote
    d) Delete remote
    q) Quit config
    e/n/d/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Encrypt/Decrypt a remote
       \ "crypt"
     6 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     7 / Google Drive
       \ "drive"
     8 / Hubic
       \ "hubic"
     9 / Local Disk
       \ "local"
    10 / OpenDrive
       \ "opendrive"
    11 / Microsoft OneDrive
       \ "onedrive"
    12 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    13 / SSH/SFTP Connection
       \ "sftp"
    14 / Yandex Disk
       \ "yandex"
    Storage> 10
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

List directories in top level of your OpenDrive

    rclone lsd remote:

List all the files in your OpenDrive

    rclone ls remote:

To copy a local directory to an OpenDrive directory called backup

    rclone copy /home/source remote:backup

Modified time and MD5SUMs

OpenDrive allows modification times to be set on objects accurate to 1
second. These will be used to detect whether objects need syncing or
not.

Standard Options

Here are the standard options specific to opendrive (OpenDrive).

–opendrive-username

Username

-   Config: username
-   Env Var: RCLONE_OPENDRIVE_USERNAME
-   Type: string
-   Default: ""

–opendrive-password

Password.

-   Config: password
-   Env Var: RCLONE_OPENDRIVE_PASSWORD
-   Type: string
-   Default: ""

Limitations

Note that OpenDrive is case insensitive so you can’t have a file called
“Hello.doc” and one called “hello.doc”.

There are quite a few characters that can’t be in OpenDrive file names.
These can’t occur on Windows platforms, but on non-Windows platforms
they are common. Rclone will map these names to and from an identical
looking unicode equivalent. For example if a file has a ? in it will be
mapped to ？ instead.


QingStor

Paths are specified as remote:bucket (or remote: for the lsd command.)
You may put subdirectories in too, eg remote:bucket/path/to/dir.

Here is an example of making an QingStor configuration. First run

    rclone config

This will guide you through an interactive setup process.

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
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Encrypt/Decrypt a remote
       \ "crypt"
     6 / FTP Connection
       \ "ftp"
     7 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     8 / Google Drive
       \ "drive"
     9 / Hubic
       \ "hubic"
    10 / Local Disk
       \ "local"
    11 / Microsoft OneDrive
       \ "onedrive"
    12 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    13 / QingStor Object Storage
       \ "qingstor"
    14 / SSH/SFTP Connection
       \ "sftp"
    15 / Yandex Disk
       \ "yandex"
    Storage> 13
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
    Enter a endpoint URL to connection QingStor API.
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
    Number of connnection retry.
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

This remote is called remote and can now be used like this

See all buckets

    rclone lsd remote:

Make a new bucket

    rclone mkdir remote:bucket

List the contents of a bucket

    rclone ls remote:bucket

Sync /home/local/directory to the remote bucket, deleting any excess
files in the bucket.

    rclone sync /home/local/directory remote:bucket

–fast-list

This remote supports --fast-list which allows you to use fewer
transactions in exchange for more memory. See the rclone docs for more
details.

Multipart uploads

rclone supports multipart uploads with QingStor which means that it can
upload files bigger than 5GB. Note that files uploaded with multipart
upload don’t have an MD5SUM.

Buckets and Zone

With QingStor you can list buckets (rclone lsd) using any zone, but you
can only access the content of a bucket from the zone it was created in.
If you attempt to access a bucket from the wrong zone, you will get an
error, incorrect zone, the bucket is not in 'XXX' zone.

Authentication

There are two ways to supply rclone with a set of QingStor credentials.
In order of precedence:

-   Directly in the rclone configuration file (as configured by
    rclone config)
    -   set access_key_id and secret_access_key
-   Runtime configuration:
    -   set env_auth to true in the config file
    -   Exporting the following environment variables before running
        rclone
        -   Access Key ID: QS_ACCESS_KEY_ID or QS_ACCESS_KEY
        -   Secret Access Key: QS_SECRET_ACCESS_KEY or QS_SECRET_KEY

Standard Options

Here are the standard options specific to qingstor (QingCloud Object
Storage).

–qingstor-env-auth

Get QingStor credentials from runtime. Only applies if access_key_id and
secret_access_key is blank.

-   Config: env_auth
-   Env Var: RCLONE_QINGSTOR_ENV_AUTH
-   Type: bool
-   Default: false
-   Examples:
    -   “false”
        -   Enter QingStor credentials in the next step
    -   “true”
        -   Get QingStor credentials from the environment (env vars or
            IAM)

–qingstor-access-key-id

QingStor Access Key ID Leave blank for anonymous access or runtime
credentials.

-   Config: access_key_id
-   Env Var: RCLONE_QINGSTOR_ACCESS_KEY_ID
-   Type: string
-   Default: ""

–qingstor-secret-access-key

QingStor Secret Access Key (password) Leave blank for anonymous access
or runtime credentials.

-   Config: secret_access_key
-   Env Var: RCLONE_QINGSTOR_SECRET_ACCESS_KEY
-   Type: string
-   Default: ""

–qingstor-endpoint

Enter a endpoint URL to connection QingStor API. Leave blank will use
the default value “https://qingstor.com:443”

-   Config: endpoint
-   Env Var: RCLONE_QINGSTOR_ENDPOINT
-   Type: string
-   Default: ""

–qingstor-zone

Zone to connect to. Default is “pek3a”.

-   Config: zone
-   Env Var: RCLONE_QINGSTOR_ZONE
-   Type: string
-   Default: ""
-   Examples:
    -   “pek3a”
        -   The Beijing (China) Three Zone
        -   Needs location constraint pek3a.
    -   “sh1a”
        -   The Shanghai (China) First Zone
        -   Needs location constraint sh1a.
    -   “gd2a”
        -   The Guangdong (China) Second Zone
        -   Needs location constraint gd2a.

Advanced Options

Here are the advanced options specific to qingstor (QingCloud Object
Storage).

–qingstor-connection-retries

Number of connection retries.

-   Config: connection_retries
-   Env Var: RCLONE_QINGSTOR_CONNECTION_RETRIES
-   Type: int
-   Default: 3

–qingstor-upload-cutoff

Cutoff for switching to chunked upload

Any files larger than this will be uploaded in chunks of chunk_size. The
minimum is 0 and the maximum is 5GB.

-   Config: upload_cutoff
-   Env Var: RCLONE_QINGSTOR_UPLOAD_CUTOFF
-   Type: SizeSuffix
-   Default: 200M

–qingstor-chunk-size

Chunk size to use for uploading.

When uploading files larger than upload_cutoff they will be uploaded as
multipart uploads using this chunk size.

Note that “–qingstor-upload-concurrency” chunks of this size are
buffered in memory per transfer.

If you are transferring large files over high speed links and you have
enough memory, then increasing this will speed up the transfers.

-   Config: chunk_size
-   Env Var: RCLONE_QINGSTOR_CHUNK_SIZE
-   Type: SizeSuffix
-   Default: 4M

–qingstor-upload-concurrency

Concurrency for multipart uploads.

This is the number of chunks of the same file that are uploaded
concurrently.

NB if you set this to > 1 then the checksums of multpart uploads become
corrupted (the uploads themselves are not corrupted though).

If you are uploading small numbers of large file over high speed link
and these uploads do not fully utilize your bandwidth, then increasing
this may help to speed up the transfers.

-   Config: upload_concurrency
-   Env Var: RCLONE_QINGSTOR_UPLOAD_CONCURRENCY
-   Type: int
-   Default: 1


Swift

Swift refers to Openstack Object Storage. Commercial implementations of
that being:

-   Rackspace Cloud Files
-   Memset Memstore
-   OVH Object Storage
-   Oracle Cloud Storage
-   IBM Bluemix Cloud ObjectStorage Swift

Paths are specified as remote:container (or remote: for the lsd
command.) You may put subdirectories in too, eg
remote:container/path/to/dir.

Here is an example of making a swift configuration. First run

    rclone config

This will guide you through an interactive setup process.

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Box
       \ "box"
     5 / Cache a remote
       \ "cache"
     6 / Dropbox
       \ "dropbox"
     7 / Encrypt/Decrypt a remote
       \ "crypt"
     8 / FTP Connection
       \ "ftp"
     9 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
    10 / Google Drive
       \ "drive"
    11 / Hubic
       \ "hubic"
    12 / Local Disk
       \ "local"
    13 / Microsoft Azure Blob Storage
       \ "azureblob"
    14 / Microsoft OneDrive
       \ "onedrive"
    15 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    16 / Pcloud
       \ "pcloud"
    17 / QingCloud Object Storage
       \ "qingstor"
    18 / SSH/SFTP Connection
       \ "sftp"
    19 / Webdav
       \ "webdav"
    20 / Yandex Disk
       \ "yandex"
    21 / http Connection
       \ "http"
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
       \ "https://auth.cloud.ovh.net/v2.0"
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

This remote is called remote and can now be used like this

See all containers

    rclone lsd remote:

Make a new container

    rclone mkdir remote:container

List the contents of a container

    rclone ls remote:container

Sync /home/local/directory to the remote container, deleting any excess
files in the container.

    rclone sync /home/local/directory remote:container

Configuration from an OpenStack credentials file

An OpenStack credentials file typically looks something something like
this (without the comments)

    export OS_AUTH_URL=https://a.provider.net/v2.0
    export OS_TENANT_ID=ffffffffffffffffffffffffffffffff
    export OS_TENANT_NAME="1234567890123456"
    export OS_USERNAME="123abc567xy"
    echo "Please enter your OpenStack Password: "
    read -sr OS_PASSWORD_INPUT
    export OS_PASSWORD=$OS_PASSWORD_INPUT
    export OS_REGION_NAME="SBG1"
    if [ -z "$OS_REGION_NAME" ]; then unset OS_REGION_NAME; fi

The config file needs to look something like this where $OS_USERNAME
represents the value of the OS_USERNAME variable - 123abc567xy in the
example above.

    [remote]
    type = swift
    user = $OS_USERNAME
    key = $OS_PASSWORD
    auth = $OS_AUTH_URL
    tenant = $OS_TENANT_NAME

Note that you may (or may not) need to set region too - try without
first.

Configuration from the environment

If you prefer you can configure rclone to use swift using a standard set
of OpenStack environment variables.

When you run through the config, make sure you choose true for env_auth
and leave everything else blank.

rclone will then set any empty config parameters from the environment
using standard OpenStack environment variables. There is a list of the
variables in the docs for the swift library.

Using an alternate authentication method

If your OpenStack installation uses a non-standard authentication method
that might not be yet supported by rclone or the underlying swift
library, you can authenticate externally (e.g. calling manually the
openstack commands to get a token). Then, you just need to pass the two
configuration variables auth_token and storage_url. If they are both
provided, the other variables are ignored. rclone will not try to
authenticate but instead assume it is already authenticated and use
these two variables to access the OpenStack installation.

Using rclone without a config file

You can use rclone with swift without a config file, if desired, like
this:

    source openstack-credentials-file
    export RCLONE_CONFIG_MYREMOTE_TYPE=swift
    export RCLONE_CONFIG_MYREMOTE_ENV_AUTH=true
    rclone lsd myremote:

–fast-list

This remote supports --fast-list which allows you to use fewer
transactions in exchange for more memory. See the rclone docs for more
details.

–update and –use-server-modtime

As noted below, the modified time is stored on metadata on the object.
It is used by default for all operations that require checking the time
a file was last updated. It allows rclone to treat the remote more like
a true filesystem, but it is inefficient because it requires an extra
API call to retrieve the metadata.

For many operations, the time the object was last uploaded to the remote
is sufficient to determine if it is “dirty”. By using --update along
with --use-server-modtime, you can avoid the extra API call and simply
upload files whose local modtime is newer than the time it was last
uploaded.

Standard Options

Here are the standard options specific to swift (Openstack Swift
(Rackspace Cloud Files, Memset Memstore, OVH)).

–swift-env-auth

Get swift credentials from environment variables in standard OpenStack
form.

-   Config: env_auth
-   Env Var: RCLONE_SWIFT_ENV_AUTH
-   Type: bool
-   Default: false
-   Examples:
    -   “false”
        -   Enter swift credentials in the next step
    -   “true”
        -   Get swift credentials from environment vars. Leave other
            fields blank if using this.

–swift-user

User name to log in (OS_USERNAME).

-   Config: user
-   Env Var: RCLONE_SWIFT_USER
-   Type: string
-   Default: ""

–swift-key

API key or password (OS_PASSWORD).

-   Config: key
-   Env Var: RCLONE_SWIFT_KEY
-   Type: string
-   Default: ""

–swift-auth

Authentication URL for server (OS_AUTH_URL).

-   Config: auth
-   Env Var: RCLONE_SWIFT_AUTH
-   Type: string
-   Default: ""
-   Examples:
    -   “https://auth.api.rackspacecloud.com/v1.0”
        -   Rackspace US
    -   “https://lon.auth.api.rackspacecloud.com/v1.0”
        -   Rackspace UK
    -   “https://identity.api.rackspacecloud.com/v2.0”
        -   Rackspace v2
    -   “https://auth.storage.memset.com/v1.0”
        -   Memset Memstore UK
    -   “https://auth.storage.memset.com/v2.0”
        -   Memset Memstore UK v2
    -   “https://auth.cloud.ovh.net/v2.0”
        -   OVH

–swift-user-id

User ID to log in - optional - most swift systems use user and leave
this blank (v3 auth) (OS_USER_ID).

-   Config: user_id
-   Env Var: RCLONE_SWIFT_USER_ID
-   Type: string
-   Default: ""

–swift-domain

User domain - optional (v3 auth) (OS_USER_DOMAIN_NAME)

-   Config: domain
-   Env Var: RCLONE_SWIFT_DOMAIN
-   Type: string
-   Default: ""

–swift-tenant

Tenant name - optional for v1 auth, this or tenant_id required otherwise
(OS_TENANT_NAME or OS_PROJECT_NAME)

-   Config: tenant
-   Env Var: RCLONE_SWIFT_TENANT
-   Type: string
-   Default: ""

–swift-tenant-id

Tenant ID - optional for v1 auth, this or tenant required otherwise
(OS_TENANT_ID)

-   Config: tenant_id
-   Env Var: RCLONE_SWIFT_TENANT_ID
-   Type: string
-   Default: ""

–swift-tenant-domain

Tenant domain - optional (v3 auth) (OS_PROJECT_DOMAIN_NAME)

-   Config: tenant_domain
-   Env Var: RCLONE_SWIFT_TENANT_DOMAIN
-   Type: string
-   Default: ""

–swift-region

Region name - optional (OS_REGION_NAME)

-   Config: region
-   Env Var: RCLONE_SWIFT_REGION
-   Type: string
-   Default: ""

–swift-storage-url

Storage URL - optional (OS_STORAGE_URL)

-   Config: storage_url
-   Env Var: RCLONE_SWIFT_STORAGE_URL
-   Type: string
-   Default: ""

–swift-auth-token

Auth Token from alternate authentication - optional (OS_AUTH_TOKEN)

-   Config: auth_token
-   Env Var: RCLONE_SWIFT_AUTH_TOKEN
-   Type: string
-   Default: ""

–swift-application-credential-id

Application Credential ID (OS_APPLICATION_CREDENTIAL_ID)

-   Config: application_credential_id
-   Env Var: RCLONE_SWIFT_APPLICATION_CREDENTIAL_ID
-   Type: string
-   Default: ""

–swift-application-credential-name

Application Credential Name (OS_APPLICATION_CREDENTIAL_NAME)

-   Config: application_credential_name
-   Env Var: RCLONE_SWIFT_APPLICATION_CREDENTIAL_NAME
-   Type: string
-   Default: ""

–swift-application-credential-secret

Application Credential Secret (OS_APPLICATION_CREDENTIAL_SECRET)

-   Config: application_credential_secret
-   Env Var: RCLONE_SWIFT_APPLICATION_CREDENTIAL_SECRET
-   Type: string
-   Default: ""

–swift-auth-version

AuthVersion - optional - set to (1,2,3) if your auth URL has no version
(ST_AUTH_VERSION)

-   Config: auth_version
-   Env Var: RCLONE_SWIFT_AUTH_VERSION
-   Type: int
-   Default: 0

–swift-endpoint-type

Endpoint type to choose from the service catalogue (OS_ENDPOINT_TYPE)

-   Config: endpoint_type
-   Env Var: RCLONE_SWIFT_ENDPOINT_TYPE
-   Type: string
-   Default: “public”
-   Examples:
    -   “public”
        -   Public (default, choose this if not sure)
    -   “internal”
        -   Internal (use internal service net)
    -   “admin”
        -   Admin

–swift-storage-policy

The storage policy to use when creating a new container

This applies the specified storage policy when creating a new container.
The policy cannot be changed afterwards. The allowed configuration
values and their meaning depend on your Swift storage provider.

-   Config: storage_policy
-   Env Var: RCLONE_SWIFT_STORAGE_POLICY
-   Type: string
-   Default: ""
-   Examples:
    -   ""
        -   Default
    -   “pcs”
        -   OVH Public Cloud Storage
    -   “pca”
        -   OVH Public Cloud Archive

Advanced Options

Here are the advanced options specific to swift (Openstack Swift
(Rackspace Cloud Files, Memset Memstore, OVH)).

–swift-chunk-size

Above this size files will be chunked into a _segments container.

Above this size files will be chunked into a _segments container. The
default for this is 5GB which is its maximum value.

-   Config: chunk_size
-   Env Var: RCLONE_SWIFT_CHUNK_SIZE
-   Type: SizeSuffix
-   Default: 5G

–swift-no-chunk

Don’t chunk files during streaming upload.

When doing streaming uploads (eg using rcat or mount) setting this flag
will cause the swift backend to not upload chunked files.

This will limit the maximum upload size to 5GB. However non chunked
files are easier to deal with and have an MD5SUM.

Rclone will still chunk files bigger than chunk_size when doing normal
copy operations.

-   Config: no_chunk
-   Env Var: RCLONE_SWIFT_NO_CHUNK
-   Type: bool
-   Default: false

Modified time

The modified time is stored as metadata on the object as
X-Object-Meta-Mtime as floating point since the epoch accurate to 1 ns.

This is a defacto standard (used in the official python-swiftclient
amongst others) for storing the modification time for an object.

Limitations

The Swift API doesn’t return a correct MD5SUM for segmented files
(Dynamic or Static Large Objects) so rclone won’t check or use the
MD5SUM for these.

Troubleshooting

Rclone gives Failed to create file system for “remote:”: Bad Request

Due to an oddity of the underlying swift library, it gives a “Bad
Request” error rather than a more sensible error when the authentication
fails for Swift.

So this most likely means your username / password is wrong. You can
investigate further with the --dump-bodies flag.

This may also be caused by specifying the region when you shouldn’t have
(eg OVH).

Rclone gives Failed to create file system: Response didn’t have storage storage url and auth token

This is most likely caused by forgetting to specify your tenant when
setting up a swift remote.


pCloud

Paths are specified as remote:path

Paths may be as deep as required, eg remote:directory/subdirectory.

The initial setup for pCloud involves getting a token from pCloud which
you need to do in your browser. rclone config walks you through it.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Box
       \ "box"
     5 / Dropbox
       \ "dropbox"
     6 / Encrypt/Decrypt a remote
       \ "crypt"
     7 / FTP Connection
       \ "ftp"
     8 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     9 / Google Drive
       \ "drive"
    10 / Hubic
       \ "hubic"
    11 / Local Disk
       \ "local"
    12 / Microsoft Azure Blob Storage
       \ "azureblob"
    13 / Microsoft OneDrive
       \ "onedrive"
    14 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    15 / Pcloud
       \ "pcloud"
    16 / QingCloud Object Storage
       \ "qingstor"
    17 / SSH/SFTP Connection
       \ "sftp"
    18 / Yandex Disk
       \ "yandex"
    19 / http Connection
       \ "http"
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

See the remote setup docs for how to set it up on a machine with no
Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from pCloud. This only runs from the moment it opens
your browser to the moment you get back the verification code. This is
on http://127.0.0.1:53682/ and this it may require you to unblock it
temporarily if you are running a host firewall.

Once configured you can then use rclone like this,

List directories in top level of your pCloud

    rclone lsd remote:

List all the files in your pCloud

    rclone ls remote:

To copy a local directory to an pCloud directory called backup

    rclone copy /home/source remote:backup

Modified time and hashes

pCloud allows modification times to be set on objects accurate to 1
second. These will be used to detect whether objects need syncing or
not. In order to set a Modification time pCloud requires the object be
re-uploaded.

pCloud supports MD5 and SHA1 type hashes, so you can use the --checksum
flag.

Deleting files

Deleted files will be moved to the trash. Your subscription level will
determine how long items stay in the trash. rclone cleanup can be used
to empty the trash.

Standard Options

Here are the standard options specific to pcloud (Pcloud).

–pcloud-client-id

Pcloud App Client Id Leave blank normally.

-   Config: client_id
-   Env Var: RCLONE_PCLOUD_CLIENT_ID
-   Type: string
-   Default: ""

–pcloud-client-secret

Pcloud App Client Secret Leave blank normally.

-   Config: client_secret
-   Env Var: RCLONE_PCLOUD_CLIENT_SECRET
-   Type: string
-   Default: ""


SFTP

SFTP is the Secure (or SSH) File Transfer Protocol.

SFTP runs over SSH v2 and is installed as standard with most modern SSH
installations.

Paths are specified as remote:path. If the path does not begin with a /
it is relative to the home directory of the user. An empty path remote:
refers to the user’s home directory.

"Note that some SFTP servers will need the leading / - Synology is a
good example of this. rsync.net, on the other hand, requires users to
OMIT the leading /.

Here is an example of making an SFTP configuration. First run

    rclone config

This will guide you through an interactive setup process.

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Encrypt/Decrypt a remote
       \ "crypt"
     6 / FTP Connection
       \ "ftp"
     7 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     8 / Google Drive
       \ "drive"
     9 / Hubic
       \ "hubic"
    10 / Local Disk
       \ "local"
    11 / Microsoft OneDrive
       \ "onedrive"
    12 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    13 / SSH/SFTP Connection
       \ "sftp"
    14 / Yandex Disk
       \ "yandex"
    15 / http Connection
       \ "http"
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

This remote is called remote and can now be used like this:

See all directories in the home directory

    rclone lsd remote:

Make a new directory

    rclone mkdir remote:path/to/directory

List the contents of a directory

    rclone ls remote:path/to/directory

Sync /home/local/directory to the remote directory, deleting any excess
files in the directory.

    rclone sync /home/local/directory remote:directory

SSH Authentication

The SFTP remote supports three authentication methods:

-   Password
-   Key file
-   ssh-agent

Key files should be PEM-encoded private key files. For instance
/home/$USER/.ssh/id_rsa. Only unencrypted OpenSSH or PEM encrypted files
are supported.

If you don’t specify pass or key_file then rclone will attempt to
contact an ssh-agent.

You can also specify key_use_agent to force the usage of an ssh-agent.
In this case key_file can also be specified to force the usage of a
specific key in the ssh-agent.

Using an ssh-agent is the only way to load encrypted OpenSSH keys at the
moment.

If you set the --sftp-ask-password option, rclone will prompt for a
password when needed and no password has been configured.

ssh-agent on macOS

Note that there seem to be various problems with using an ssh-agent on
macOS due to recent changes in the OS. The most effective work-around
seems to be to start an ssh-agent in each session, eg

    eval `ssh-agent -s` && ssh-add -A

And then at the end of the session

    eval `ssh-agent -k`

These commands can be used in scripts of course.

Modified time

Modified times are stored on the server to 1 second precision.

Modified times are used in syncing and are fully supported.

Some SFTP servers disable setting/modifying the file modification time
after upload (for example, certain configurations of ProFTPd with
mod_sftp). If you are using one of these servers, you can set the option
set_modtime = false in your RClone backend configuration to disable this
behaviour.

Standard Options

Here are the standard options specific to sftp (SSH/SFTP Connection).

–sftp-host

SSH host to connect to

-   Config: host
-   Env Var: RCLONE_SFTP_HOST
-   Type: string
-   Default: ""
-   Examples:
    -   “example.com”
        -   Connect to example.com

–sftp-user

SSH username, leave blank for current username, ncw

-   Config: user
-   Env Var: RCLONE_SFTP_USER
-   Type: string
-   Default: ""

–sftp-port

SSH port, leave blank to use default (22)

-   Config: port
-   Env Var: RCLONE_SFTP_PORT
-   Type: string
-   Default: ""

–sftp-pass

SSH password, leave blank to use ssh-agent.

-   Config: pass
-   Env Var: RCLONE_SFTP_PASS
-   Type: string
-   Default: ""

–sftp-key-file

Path to PEM-encoded private key file, leave blank or set key-use-agent
to use ssh-agent.

-   Config: key_file
-   Env Var: RCLONE_SFTP_KEY_FILE
-   Type: string
-   Default: ""

–sftp-key-file-pass

The passphrase to decrypt the PEM-encoded private key file.

Only PEM encrypted key files (old OpenSSH format) are supported.
Encrypted keys in the new OpenSSH format can’t be used.

-   Config: key_file_pass
-   Env Var: RCLONE_SFTP_KEY_FILE_PASS
-   Type: string
-   Default: ""

–sftp-key-use-agent

When set forces the usage of the ssh-agent.

When key-file is also set, the “.pub” file of the specified key-file is
read and only the associated key is requested from the ssh-agent. This
allows to avoid Too many authentication failures for *username* errors
when the ssh-agent contains many keys.

-   Config: key_use_agent
-   Env Var: RCLONE_SFTP_KEY_USE_AGENT
-   Type: bool
-   Default: false

–sftp-use-insecure-cipher

Enable the use of the aes128-cbc cipher. This cipher is insecure and may
allow plaintext data to be recovered by an attacker.

-   Config: use_insecure_cipher
-   Env Var: RCLONE_SFTP_USE_INSECURE_CIPHER
-   Type: bool
-   Default: false
-   Examples:
    -   “false”
        -   Use default Cipher list.
    -   “true”
        -   Enables the use of the aes128-cbc cipher.

–sftp-disable-hashcheck

Disable the execution of SSH commands to determine if remote file
hashing is available. Leave blank or set to false to enable hashing
(recommended), set to true to disable hashing.

-   Config: disable_hashcheck
-   Env Var: RCLONE_SFTP_DISABLE_HASHCHECK
-   Type: bool
-   Default: false

Advanced Options

Here are the advanced options specific to sftp (SSH/SFTP Connection).

–sftp-ask-password

Allow asking for SFTP password when needed.

-   Config: ask_password
-   Env Var: RCLONE_SFTP_ASK_PASSWORD
-   Type: bool
-   Default: false

–sftp-path-override

Override path used by SSH connection.

This allows checksum calculation when SFTP and SSH paths are different.
This issue affects among others Synology NAS boxes.

Shared folders can be found in directories representing volumes

    rclone sync /home/local/directory remote:/directory --ssh-path-override /volume2/directory

Home directory can be found in a shared folder called “home”

    rclone sync /home/local/directory remote:/home/directory --ssh-path-override /volume1/homes/USER/directory

-   Config: path_override
-   Env Var: RCLONE_SFTP_PATH_OVERRIDE
-   Type: string
-   Default: ""

–sftp-set-modtime

Set the modified time on the remote if set.

-   Config: set_modtime
-   Env Var: RCLONE_SFTP_SET_MODTIME
-   Type: bool
-   Default: true

Limitations

SFTP supports checksums if the same login has shell access and md5sum or
sha1sum as well as echo are in the remote’s PATH. This remote
checksumming (file hashing) is recommended and enabled by default.
Disabling the checksumming may be required if you are connecting to SFTP
servers which are not under your control, and to which the execution of
remote commands is prohibited. Set the configuration option
disable_hashcheck to true to disable checksumming.

SFTP also supports about if the same login has shell access and df are
in the remote’s PATH. about will return the total space, free space, and
used space on the remote for the disk of the specified path on the
remote or, if not set, the disk of the root on the remote. about will
fail if it does not have shell access or if df is not in the remote’s
PATH.

Note that some SFTP servers (eg Synology) the paths are different for
SSH and SFTP so the hashes can’t be calculated properly. For them using
disable_hashcheck is a good idea.

The only ssh agent supported under Windows is Putty’s pageant.

The Go SSH library disables the use of the aes128-cbc cipher by default,
due to security concerns. This can be re-enabled on a per-connection
basis by setting the use_insecure_cipher setting in the configuration
file to true. Further details on the insecurity of this cipher can be
found [in this paper] (http://www.isg.rhul.ac.uk/~kp/SandPfinal.pdf).

SFTP isn’t supported under plan9 until this issue is fixed.

Note that since SFTP isn’t HTTP based the following flags don’t work
with it: --dump-headers, --dump-bodies, --dump-auth

Note that --timeout isn’t supported (but --contimeout is).


Union

The union remote provides a unification similar to UnionFS using other
remotes.

Paths may be as deep as required or a local path, eg
remote:directory/subdirectory or /directory/subdirectory.

During the initial setup with rclone config you will specify the target
remotes as a space separated list. The target remotes can either be a
local paths or other remotes.

The order of the remotes is important as it defines which remotes take
precedence over others if there are files with the same name in the same
logical path. The last remote is the topmost remote and replaces files
with the same name from previous remotes.

Only the last remote is used to write to and delete from, all other
remotes are read-only.

Subfolders can be used in target remote. Assume a union remote named
backup with the remotes mydrive:private/backup mydrive2:/backup.
Invoking rclone mkdir backup:desktop is exactly the same as invoking
rclone mkdir mydrive2:/backup/desktop.

There will be no special handling of paths containing .. segments.
Invoking rclone mkdir backup:../desktop is exactly the same as invoking
rclone mkdir mydrive2:/backup/../desktop.

Here is an example of how to make a union called remote for local
folders. First run:

     rclone config

This will guide you through an interactive setup process:

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Alias for an existing remote
       \ "alias"
     2 / Amazon Drive
       \ "amazon cloud drive"
     3 / Amazon S3 Compliant Storage Providers (AWS, Ceph, Dreamhost, IBM COS, Minio)
       \ "s3"
     4 / Backblaze B2
       \ "b2"
     5 / Box
       \ "box"
     6 / Builds a stackable unification remote, which can appear to merge the contents of several remotes
       \ "union"
     7 / Cache a remote
       \ "cache"
     8 / Dropbox
       \ "dropbox"
     9 / Encrypt/Decrypt a remote
       \ "crypt"
    10 / FTP Connection
       \ "ftp"
    11 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
    12 / Google Drive
       \ "drive"
    13 / Hubic
       \ "hubic"
    14 / JottaCloud
       \ "jottacloud"
    15 / Local Disk
       \ "local"
    16 / Mega
       \ "mega"
    17 / Microsoft Azure Blob Storage
       \ "azureblob"
    18 / Microsoft OneDrive
       \ "onedrive"
    19 / OpenDrive
       \ "opendrive"
    20 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    21 / Pcloud
       \ "pcloud"
    22 / QingCloud Object Storage
       \ "qingstor"
    23 / SSH/SFTP Connection
       \ "sftp"
    24 / Webdav
       \ "webdav"
    25 / Yandex Disk
       \ "yandex"
    26 / http Connection
       \ "http"
    Storage> union
    List of space separated remotes.
    Can be 'remotea:test/dir remoteb:', '"remotea:test/space dir" remoteb:', etc.
    The last remote is used to write to.
    Enter a string value. Press Enter for the default ("").
    remotes>
    Remote config
    --------------------
    [remote]
    type = union
    remotes = C:\dir1 C:\dir2 C:\dir3
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

Once configured you can then use rclone like this,

List directories in top level in C:\dir1, C:\dir2 and C:\dir3

    rclone lsd remote:

List all the files in C:\dir1, C:\dir2 and C:\dir3

    rclone ls remote:

Copy another local directory to the union directory called source, which
will be placed into C:\dir3

    rclone copy C:\source remote:source

Standard Options

Here are the standard options specific to union (A stackable unification
remote, which can appear to merge the contents of several remotes).

–union-remotes

List of space separated remotes. Can be ‘remotea:test/dir remoteb:’,
‘“remotea:test/space dir” remoteb:’, etc. The last remote is used to
write to.

-   Config: remotes
-   Env Var: RCLONE_UNION_REMOTES
-   Type: string
-   Default: ""


WebDAV

Paths are specified as remote:path

Paths may be as deep as required, eg remote:directory/subdirectory.

To configure the WebDAV remote you will need to have a URL for it, and a
username and password. If you know what kind of system you are
connecting to then rclone can enable extra features.

Here is an example of how to make a remote called remote. First run:

     rclone config

This will guide you through an interactive setup process:

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    q) Quit config
    n/s/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
    [snip]
    22 / Webdav
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

Once configured you can then use rclone like this,

List directories in top level of your WebDAV

    rclone lsd remote:

List all the files in your WebDAV

    rclone ls remote:

To copy a local directory to an WebDAV directory called backup

    rclone copy /home/source remote:backup

Modified time and hashes

Plain WebDAV does not support modified times. However when used with
Owncloud or Nextcloud rclone will support modified times.

Likewise plain WebDAV does not support hashes, however when used with
Owncloud or Nextcloud rclone will support SHA1 and MD5 hashes. Depending
on the exact version of Owncloud or Nextcloud hashes may appear on all
objects, or only on objects which had a hash uploaded with them.

Standard Options

Here are the standard options specific to webdav (Webdav).

–webdav-url

URL of http host to connect to

-   Config: url
-   Env Var: RCLONE_WEBDAV_URL
-   Type: string
-   Default: ""
-   Examples:
    -   “https://example.com”
        -   Connect to example.com

–webdav-vendor

Name of the Webdav site/service/software you are using

-   Config: vendor
-   Env Var: RCLONE_WEBDAV_VENDOR
-   Type: string
-   Default: ""
-   Examples:
    -   “nextcloud”
        -   Nextcloud
    -   “owncloud”
        -   Owncloud
    -   “sharepoint”
        -   Sharepoint
    -   “other”
        -   Other site/service or software

–webdav-user

User name

-   Config: user
-   Env Var: RCLONE_WEBDAV_USER
-   Type: string
-   Default: ""

–webdav-pass

Password.

-   Config: pass
-   Env Var: RCLONE_WEBDAV_PASS
-   Type: string
-   Default: ""

–webdav-bearer-token

Bearer token instead of user/pass (eg a Macaroon)

-   Config: bearer_token
-   Env Var: RCLONE_WEBDAV_BEARER_TOKEN
-   Type: string
-   Default: ""


Provider notes

See below for notes on specific providers.

Owncloud

Click on the settings cog in the bottom right of the page and this will
show the WebDAV URL that rclone needs in the config step. It will look
something like https://example.com/remote.php/webdav/.

Owncloud supports modified times using the X-OC-Mtime header.

Nextcloud

This is configured in an identical way to Owncloud. Note that Nextcloud
does not support streaming of files (rcat) whereas Owncloud does. This
may be fixed in the future.

Put.io

put.io can be accessed in a read only way using webdav.

Configure the url as https://webdav.put.io and use your normal account
username and password for user and pass. Set the vendor to other.

Your config file should end up looking like this:

    [putio]
    type = webdav
    url = https://webdav.put.io
    vendor = other
    user = YourUserName
    pass = encryptedpassword

If you are using put.io with rclone mount then use the --read-only flag
to signal to the OS that it can’t write to the mount.

For more help see the put.io webdav docs.

Sharepoint

Rclone can be used with Sharepoint provided by OneDrive for Business or
Office365 Education Accounts. This feature is only needed for a few of
these Accounts, mostly Office365 Education ones. These accounts are
sometimes not verified by the domain owner github#1975

This means that these accounts can’t be added using the official API
(other Accounts should work with the “onedrive” option). However, it is
possible to access them using webdav.

To use a sharepoint remote with rclone, add it like this: First, you
need to get your remote’s URL:

-   Go here to open your OneDrive or to sign in
-   Now take a look at your address bar, the URL should look like this:
    https://[YOUR-DOMAIN]-my.sharepoint.com/personal/[YOUR-EMAIL]/_layouts/15/onedrive.aspx

You’ll only need this URL upto the email address. After that, you’ll
most likely want to add “/Documents”. That subdirectory contains the
actual data stored on your OneDrive.

Add the remote to rclone like this: Configure the url as
https://[YOUR-DOMAIN]-my.sharepoint.com/personal/[YOUR-EMAIL]/Documents
and use your normal account email and password for user and pass. If you
have 2FA enabled, you have to generate an app password. Set the vendor
to sharepoint.

Your config file should look like this:

    [sharepoint]
    type = webdav
    url = https://[YOUR-DOMAIN]-my.sharepoint.com/personal/[YOUR-EMAIL]/Documents
    vendor = other
    user = YourEmailAddress
    pass = encryptedpassword

dCache

dCache is a storage system with WebDAV doors that support, beside basic
and x509, authentication with Macaroons (bearer tokens).

Configure as normal using the other type. Don’t enter a username or
password, instead enter your Macaroon as the bearer_token.

The config will end up looking something like this.

    [dcache]
    type = webdav
    url = https://dcache...
    vendor = other
    user =
    pass =
    bearer_token = your-macaroon

There is a script that obtains a Macaroon from a dCache WebDAV endpoint,
and creates an rclone config file.


Yandex Disk

Yandex Disk is a cloud storage solution created by Yandex.

Yandex paths may be as deep as required, eg
remote:directory/subdirectory.

Here is an example of making a yandex configuration. First run

    rclone config

This will guide you through an interactive setup process:

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    n/s> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph, Minio)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Encrypt/Decrypt a remote
       \ "crypt"
     6 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     7 / Google Drive
       \ "drive"
     8 / Hubic
       \ "hubic"
     9 / Local Disk
       \ "local"
    10 / Microsoft OneDrive
       \ "onedrive"
    11 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    12 / SSH/SFTP Connection
       \ "sftp"
    13 / Yandex Disk
       \ "yandex"
    Storage> 13
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

See the remote setup docs for how to set it up on a machine with no
Internet browser available.

Note that rclone runs a webserver on your local machine to collect the
token as returned from Yandex Disk. This only runs from the moment it
opens your browser to the moment you get back the verification code.
This is on http://127.0.0.1:53682/ and this it may require you to
unblock it temporarily if you are running a host firewall.

Once configured you can then use rclone like this,

See top level directories

    rclone lsd remote:

Make a new directory

    rclone mkdir remote:directory

List the contents of a directory

    rclone ls remote:directory

Sync /home/local/directory to the remote path, deleting any excess files
in the path.

    rclone sync /home/local/directory remote:directory

Modified time

Modified times are supported and are stored accurate to 1 ns in custom
metadata called rclone_modified in RFC3339 with nanoseconds format.

MD5 checksums

MD5 checksums are natively supported by Yandex Disk.

Emptying Trash

If you wish to empty your trash you can use the rclone cleanup remote:
command which will permanently delete all your trashed files. This
command does not take any path arguments.

Quota information

To view your current quota you can use the rclone about remote: command
which will display your usage limit (quota) and the current usage.

Limitations

When uploading very large files (bigger than about 5GB) you will need to
increase the --timeout parameter. This is because Yandex pauses (perhaps
to calculate the MD5SUM for the entire file) before returning
confirmation that the file has been uploaded. The default handling of
timeouts in rclone is to assume a 5 minute pause is an error and close
the connection - you’ll see net/http: timeout awaiting response headers
errors in the logs if this is happening. Setting the timeout to twice
the max size of file in GB should be enough, so if you want to upload a
30GB file set a timeout of 2 * 30 = 60m, that is --timeout 60m.

Standard Options

Here are the standard options specific to yandex (Yandex Disk).

–yandex-client-id

Yandex Client Id Leave blank normally.

-   Config: client_id
-   Env Var: RCLONE_YANDEX_CLIENT_ID
-   Type: string
-   Default: ""

–yandex-client-secret

Yandex Client Secret Leave blank normally.

-   Config: client_secret
-   Env Var: RCLONE_YANDEX_CLIENT_SECRET
-   Type: string
-   Default: ""

Advanced Options

Here are the advanced options specific to yandex (Yandex Disk).

–yandex-unlink

Remove existing public link to file/folder with link command rather than
creating. Default is false, meaning link command will create or retrieve
public link.

-   Config: unlink
-   Env Var: RCLONE_YANDEX_UNLINK
-   Type: bool
-   Default: false


Local Filesystem

Local paths are specified as normal filesystem paths, eg
/path/to/wherever, so

    rclone sync /home/source /tmp/destination

Will sync /home/source to /tmp/destination

These can be configured into the config file for consistencies sake, but
it is probably easier not to.

Modified time

Rclone reads and writes the modified time using an accuracy determined
by the OS. Typically this is 1ns on Linux, 10 ns on Windows and 1 Second
on OS X.

Filenames

Filenames are expected to be encoded in UTF-8 on disk. This is the
normal case for Windows and OS X.

There is a bit more uncertainty in the Linux world, but new
distributions will have UTF-8 encoded files names. If you are using an
old Linux filesystem with non UTF-8 file names (eg latin1) then you can
use the convmv tool to convert the filesystem to UTF-8. This tool is
available in most distributions’ package managers.

If an invalid (non-UTF8) filename is read, the invalid characters will
be replaced with the unicode replacement character, ‘�’. rclone will
emit a debug message in this case (use -v to see), eg

    Local file system at .: Replacing invalid UTF-8 characters in "gro\xdf"

Long paths on Windows

Rclone handles long paths automatically, by converting all paths to long
UNC paths which allows paths up to 32,767 characters.

This is why you will see that your paths, for instance c:\files is
converted to the UNC path \\?\c:\files in the output, and \\server\share
is converted to \\?\UNC\server\share.

However, in rare cases this may cause problems with buggy file system
drivers like EncFS. To disable UNC conversion globally, add this to your
.rclone.conf file:

    [local]
    nounc = true

If you want to selectively disable UNC, you can add it to a separate
entry like this:

    [nounc]
    type = local
    nounc = true

And use rclone like this:

rclone copy c:\src nounc:z:\dst

This will use UNC paths on c:\src but not on z:\dst. Of course this will
cause problems if the absolute path length of a file exceeds 258
characters on z, so only use this option if you have to.

Symlinks / Junction points

Normally rclone will ignore symlinks or junction points (which behave
like symlinks under Windows).

If you supply --copy-links or -L then rclone will follow the symlink and
copy the pointed to file or directory. Note that this flag is
incompatible with -links / -l.

This flag applies to all commands.

For example, supposing you have a directory structure like this

    $ tree /tmp/a
    /tmp/a
    ├── b -> ../b
    ├── expected -> ../expected
    ├── one
    └── two
        └── three

Then you can see the difference with and without the flag like this

    $ rclone ls /tmp/a
            6 one
            6 two/three

and

    $ rclone -L ls /tmp/a
         4174 expected
            6 one
            6 two/three
            6 b/two
            6 b/one

–links, -l

Normally rclone will ignore symlinks or junction points (which behave
like symlinks under Windows).

If you supply this flag then rclone will copy symbolic links from the
local storage, and store them as text files, with a ‘.rclonelink’ suffix
in the remote storage.

The text file will contain the target of the symbolic link (see
example).

This flag applies to all commands.

For example, supposing you have a directory structure like this

    $ tree /tmp/a
    /tmp/a
    ├── file1 -> ./file4
    └── file2 -> /home/user/file3

Copying the entire directory with ‘-l’

    $ rclone copyto -l /tmp/a/file1 remote:/tmp/a/

The remote files are created with a ‘.rclonelink’ suffix

    $ rclone ls remote:/tmp/a
           5 file1.rclonelink
          14 file2.rclonelink

The remote files will contain the target of the symbolic links

    $ rclone cat remote:/tmp/a/file1.rclonelink
    ./file4

    $ rclone cat remote:/tmp/a/file2.rclonelink
    /home/user/file3

Copying them back with ‘-l’

    $ rclone copyto -l remote:/tmp/a/ /tmp/b/

    $ tree /tmp/b
    /tmp/b
    ├── file1 -> ./file4
    └── file2 -> /home/user/file3

However, if copied back without ‘-l’

    $ rclone copyto remote:/tmp/a/ /tmp/b/

    $ tree /tmp/b
    /tmp/b
    ├── file1.rclonelink
    └── file2.rclonelink

Note that this flag is incompatible with -copy-links / -L.

Restricting filesystems with –one-file-system

Normally rclone will recurse through filesystems as mounted.

However if you set --one-file-system or -x this tells rclone to stay in
the filesystem specified by the root and not to recurse into different
file systems.

For example if you have a directory hierarchy like this

    root
    ├── disk1     - disk1 mounted on the root
    │   └── file3 - stored on disk1
    ├── disk2     - disk2 mounted on the root
    │   └── file4 - stored on disk12
    ├── file1     - stored on the root disk
    └── file2     - stored on the root disk

Using rclone --one-file-system copy root remote: will only copy file1
and file2. Eg

    $ rclone -q --one-file-system ls root
            0 file1
            0 file2

    $ rclone -q ls root
            0 disk1/file3
            0 disk2/file4
            0 file1
            0 file2

NB Rclone (like most unix tools such as du, rsync and tar) treats a bind
mount to the same device as being on the same filesystem.

NB This flag is only available on Unix based systems. On systems where
it isn’t supported (eg Windows) it will be ignored.

Standard Options

Here are the standard options specific to local (Local Disk).

–local-nounc

Disable UNC (long path names) conversion on Windows

-   Config: nounc
-   Env Var: RCLONE_LOCAL_NOUNC
-   Type: string
-   Default: ""
-   Examples:
    -   “true”
        -   Disables long file names

Advanced Options

Here are the advanced options specific to local (Local Disk).

–copy-links

Follow symlinks and copy the pointed to item.

-   Config: copy_links
-   Env Var: RCLONE_LOCAL_COPY_LINKS
-   Type: bool
-   Default: false

–links

Translate symlinks to/from regular files with a ‘.rclonelink’ extension

-   Config: links
-   Env Var: RCLONE_LOCAL_LINKS
-   Type: bool
-   Default: false

–skip-links

Don’t warn about skipped symlinks. This flag disables warning messages
on skipped symlinks or junction points, as you explicitly acknowledge
that they should be skipped.

-   Config: skip_links
-   Env Var: RCLONE_LOCAL_SKIP_LINKS
-   Type: bool
-   Default: false

–local-no-unicode-normalization

Don’t apply unicode normalization to paths and filenames (Deprecated)

This flag is deprecated now. Rclone no longer normalizes unicode file
names, but it compares them with unicode normalization in the sync
routine instead.

-   Config: no_unicode_normalization
-   Env Var: RCLONE_LOCAL_NO_UNICODE_NORMALIZATION
-   Type: bool
-   Default: false

–local-no-check-updated

Don’t check to see if the files change during upload

Normally rclone checks the size and modification time of files as they
are being uploaded and aborts with a message which starts “can’t copy -
source file is being updated” if the file changes during upload.

However on some file systems this modification time check may fail (eg
Glusterfs #2206) so this check can be disabled with this flag.

-   Config: no_check_updated
-   Env Var: RCLONE_LOCAL_NO_CHECK_UPDATED
-   Type: bool
-   Default: false

–one-file-system

Don’t cross filesystem boundaries (unix/macOS only).

-   Config: one_file_system
-   Env Var: RCLONE_LOCAL_ONE_FILE_SYSTEM
-   Type: bool
-   Default: false



CHANGELOG


v1.48.0 - 2019-06-15

-   New commands
    -   serve sftp: Serve an rclone remote over SFTP (Nick Craig-Wood)
-   New Features
    -   Multi threaded downloads to local storage (Nick Craig-Wood)
        -   controlled with --multi-thread-cutoff and
            --multi-thread-streams
    -   Use rclone.conf from rclone executable directory to enable
        portable use (albertony)
    -   Allow sync of a file and a directory with the same name
        (forgems)
        -   this is common on bucket based remotes, eg s3, gcs
    -   Add --ignore-case-sync for forced case insensitivity (garry415)
    -   Implement --stats-one-line-date and --stats-one-line-date-format
        (Peter Berbec)
    -   Log an ERROR for all commands which exit with non-zero status
        (Nick Craig-Wood)
    -   Use go-homedir to read the home directory more reliably (Nick
        Craig-Wood)
    -   Enable creating encrypted config through external script
        invocation (Wojciech Smigielski)
    -   build: Drop support for go1.8 (Nick Craig-Wood)
    -   config: Make config create/update encrypt passwords where
        necessary (Nick Craig-Wood)
    -   copyurl: Honor --no-check-certificate (Stefan Breunig)
    -   install: Linux skip man pages if no mandb (didil)
    -   lsf: Support showing the Tier of the object (Nick Craig-Wood)
    -   lsjson
        -   Added EncryptedPath to output (calisro)
        -   Support showing the Tier of the object (Nick Craig-Wood)
        -   Add IsBucket field for bucket based remote listing of the
            root (Nick Craig-Wood)
    -   rc
        -   Add --loopback flag to run commands directly without a
            server (Nick Craig-Wood)
        -   Add operations/fsinfo: Return information about the remote
            (Nick Craig-Wood)
        -   Skip auth for OPTIONS request (Nick Craig-Wood)
        -   cmd/providers: Add DefaultStr, ValueStr and Type fields
            (Nick Craig-Wood)
        -   jobs: Make job expiry timeouts configurable (Aleksandar
            Jankovic)
    -   serve dlna reworked and improved (Dan Walters)
    -   serve ftp: add --ftp-public-ip flag to specify public IP
        (calistri)
    -   serve restic: Add support for --private-repos in serve restic
        (Florian Apolloner)
    -   serve webdav: Combine serve webdav and serve http (Gary Kim)
    -   size: Ignore negative sizes when calculating total (Garry
        McNulty)
-   Bug Fixes
    -   Make move and copy individual files obey --backup-dir (Nick
        Craig-Wood)
    -   If --ignore-checksum is in effect, don’t calculate checksum
        (Nick Craig-Wood)
    -   moveto: Fix case-insensitive same remote move (Gary Kim)
    -   rc: Fix serving bucket based objects with --rc-serve (Nick
        Craig-Wood)
    -   serve webdav: Fix serveDir not being updated with changes from
        webdav (Gary Kim)
-   Mount
    -   Fix poll interval documentation (Animosity022)
-   VFS
    -   Make WriteAt for non cached files work with non-sequential
        writes (Nick Craig-Wood)
-   Local
    -   Only calculate the required hashes for big speedup (Nick
        Craig-Wood)
    -   Log errors when listing instead of returning an error (Nick
        Craig-Wood)
    -   Fix preallocate warning on Linux with ZFS (Nick Craig-Wood)
-   Crypt
    -   Make rclone dedupe work through crypt (Nick Craig-Wood)
    -   Fix wrapping of ChangeNotify to decrypt directories properly
        (Nick Craig-Wood)
    -   Support PublicLink (rclone link) of underlying backend (Nick
        Craig-Wood)
    -   Implement Optional methods SetTier, GetTier (Nick Craig-Wood)
-   B2
    -   Implement server side copy (Nick Craig-Wood)
    -   Implement SetModTime (Nick Craig-Wood)
-   Drive
    -   Fix move and copy from TeamDrive to GDrive (Fionera)
    -   Add notes that cleanup works in the background on drive (Nick
        Craig-Wood)
    -   Add --drive-server-side-across-configs to default back to old
        server side copy semantics by default (Nick Craig-Wood)
    -   Add --drive-size-as-quota to show storage quota usage for file
        size (Garry McNulty)
-   FTP
    -   Add FTP List timeout (Jeff Quinn)
    -   Add FTP over TLS support (Gary Kim)
    -   Add --ftp-no-check-certificate option for FTPS (Gary Kim)
-   Google Cloud Storage
    -   Fix upload errors when uploading pre 1970 files (Nick
        Craig-Wood)
-   Jottacloud
    -   Add support for selecting device and mountpoint. (buengese)
-   Mega
    -   Add cleanup support (Gary Kim)
-   Onedrive
    -   More accurately check if root is found (Cnly)
-   S3
    -   Suppport S3 Accelerated endpoints with
        --s3-use-accelerate-endpoint (Nick Craig-Wood)
    -   Add config info for Wasabi’s EU Central endpoint (Robert Marko)
    -   Make SetModTime work for GLACIER while syncing (Philip Harvey)
-   SFTP
    -   Add About support (Gary Kim)
    -   Fix about parsing of df results so it can cope with -ve results
        (Nick Craig-Wood)
    -   Send custom client version and debug server version (Nick
        Craig-Wood)
-   WebDAV
    -   Retry on 423 Locked errors (Nick Craig-Wood)


v1.47.0 - 2019-04-13

-   New backends
    -   Backend for Koofr cloud storage service. (jaKa)
-   New Features
    -   Resume downloads if the reader fails in copy (Nick Craig-Wood)
        -   this means rclone will restart transfers if the source has
            an error
        -   this is most useful for downloads or cloud to cloud copies
    -   Use --fast-list for listing operations where it won’t use more
        memory (Nick Craig-Wood)
        -   this should speed up the following operations on remotes
            which support ListR
        -   dedupe, serve restic lsf, ls, lsl, lsjson, lsd, md5sum,
            sha1sum, hashsum, size, delete, cat, settier
        -   use --disable ListR to get old behaviour if required
    -   Make --files-from traverse the destination unless --no-traverse
        is set (Nick Craig-Wood)
        -   this fixes --files-from with Google drive and excessive API
            use in general.
    -   Make server side copy account bytes and obey --max-transfer
        (Nick Craig-Wood)
    -   Add --create-empty-src-dirs flag and default to not creating
        empty dirs (ishuah)
    -   Add client side TLS/SSL flags
        --ca-cert/--client-cert/--client-key (Nick Craig-Wood)
    -   Implement --suffix-keep-extension for use with --suffix (Nick
        Craig-Wood)
    -   build:
        -   Switch to semvar compliant version tags to be go modules
            compliant (Nick Craig-Wood)
        -   Update to use go1.12.x for the build (Nick Craig-Wood)
    -   serve dlna: Add connection manager service description to
        improve compatibility (Dan Walters)
    -   lsf: Add ‘e’ format to show encrypted names and ‘o’ for original
        IDs (Nick Craig-Wood)
    -   lsjson: Added --files-only and --dirs-only flags (calistri)
    -   rc: Implement operations/publiclink the equivalent of
        rclone link (Nick Craig-Wood)
-   Bug Fixes
    -   accounting: Fix total ETA when --stats-unit bits is in effect
        (Nick Craig-Wood)
    -   Bash TAB completion
        -   Use private custom func to fix clash between rclone and
            kubectl (Nick Craig-Wood)
        -   Fix for remotes with underscores in their names (Six)
        -   Fix completion of remotes (Florian Gamböck)
        -   Fix autocompletion of remote paths with spaces (Danil
            Semelenov)
    -   serve dlna: Fix root XML service descriptor (Dan Walters)
    -   ncdu: Fix display corruption with Chinese characters (Nick
        Craig-Wood)
    -   Add SIGTERM to signals which run the exit handlers on unix (Nick
        Craig-Wood)
    -   rc: Reload filter when the options are set via the rc (Nick
        Craig-Wood)
-   VFS / Mount
    -   Fix FreeBSD: Ignore Truncate if called with no readers and
        already the correct size (Nick Craig-Wood)
    -   Read directory and check for a file before mkdir (Nick
        Craig-Wood)
    -   Shorten the locking window for vfs/refresh (Nick Craig-Wood)
-   Azure Blob
    -   Enable MD5 checksums when uploading files bigger than the
        “Cutoff” (Dr.Rx)
    -   Fix SAS URL support (Nick Craig-Wood)
-   B2
    -   Allow manual configuration of backblaze downloadUrl (Vince)
    -   Ignore already_hidden error on remove (Nick Craig-Wood)
    -   Ignore malformed src_last_modified_millis (Nick Craig-Wood)
-   Drive
    -   Add --skip-checksum-gphotos to ignore incorrect checksums on
        Google Photos (Nick Craig-Wood)
    -   Allow server side move/copy between different remotes. (Fionera)
    -   Add docs on team drives and --fast-list eventual consistency
        (Nestar47)
    -   Fix imports of text files (Nick Craig-Wood)
    -   Fix range requests on 0 length files (Nick Craig-Wood)
    -   Fix creation of duplicates with server side copy (Nick
        Craig-Wood)
-   Dropbox
    -   Retry blank errors to fix long listings (Nick Craig-Wood)
-   FTP
    -   Add --ftp-concurrency to limit maximum number of connections
        (Nick Craig-Wood)
-   Google Cloud Storage
    -   Fall back to default application credentials (marcintustin)
    -   Allow bucket policy only buckets (Nick Craig-Wood)
-   HTTP
    -   Add --http-no-slash for websites with directories with no
        slashes (Nick Craig-Wood)
    -   Remove duplicates from listings (Nick Craig-Wood)
    -   Fix socket leak on 404 errors (Nick Craig-Wood)
-   Jottacloud
    -   Fix token refresh (Sebastian Bünger)
    -   Add device registration (Oliver Heyme)
-   Onedrive
    -   Implement graceful cancel of multipart uploads if rclone is
        interrupted (Cnly)
    -   Always add trailing colon to path when addressing items, (Cnly)
    -   Return errors instead of panic for invalid uploads (Fabian
        Möller)
-   S3
    -   Add support for “Glacier Deep Archive” storage class (Manu)
    -   Update Dreamhost endpoint (Nick Craig-Wood)
    -   Note incompatibility with CEPH Jewel (Nick Craig-Wood)
-   SFTP
    -   Allow custom ssh client config (Alexandru Bumbacea)
-   Swift
    -   Obey Retry-After to enable OVH restore from cold storage (Nick
        Craig-Wood)
    -   Work around token expiry on CEPH (Nick Craig-Wood)
-   WebDAV
    -   Allow IsCollection property to be integer or boolean (Nick
        Craig-Wood)
    -   Fix race when creating directories (Nick Craig-Wood)
    -   Fix About/df when reading the available/total returns 0 (Nick
        Craig-Wood)


v1.46 - 2019-02-09

-   New backends
    -   Support Alibaba Cloud (Aliyun) OSS via the s3 backend (Nick
        Craig-Wood)
-   New commands
    -   serve dlna: serves a remove via DLNA for the local network
        (nicolov)
-   New Features
    -   copy, move: Restore deprecated --no-traverse flag (Nick
        Craig-Wood)
        -   This is useful for when transferring a small number of files
            into a large destination
    -   genautocomplete: Add remote path completion for bash completion
        (Christopher Peterson & Danil Semelenov)
    -   Buffer memory handling reworked to return memory to the OS
        better (Nick Craig-Wood)
        -   Buffer recycling library to replace sync.Pool
        -   Optionally use memory mapped memory for better memory
            shrinking
        -   Enable with --use-mmap if having memory problems - not
            default yet
    -   Parallelise reading of files specified by --files-from (Nick
        Craig-Wood)
    -   check: Add stats showing total files matched. (Dario Guzik)
    -   Allow rename/delete open files under Windows (Nick Craig-Wood)
    -   lsjson: Use exactly the correct number of decimal places in the
        seconds (Nick Craig-Wood)
    -   Add cookie support with cmdline switch --use-cookies for all
        HTTP based remotes (qip)
    -   Warn if --checksum is set but there are no hashes available
        (Nick Craig-Wood)
    -   Rework rate limiting (pacer) to be more accurate and allow
        bursting (Nick Craig-Wood)
    -   Improve error reporting for too many/few arguments in commands
        (Nick Craig-Wood)
    -   listremotes: Remove -l short flag as it conflicts with the new
        global flag (weetmuts)
    -   Make http serving with auth generate INFO messages on auth fail
        (Nick Craig-Wood)
-   Bug Fixes
    -   Fix layout of stats (Nick Craig-Wood)
    -   Fix --progress crash under Windows Jenkins (Nick Craig-Wood)
    -   Fix transfer of google/onedrive docs by calling Rcat in Copy
        when size is -1 (Cnly)
    -   copyurl: Fix checking of --dry-run (Denis Skovpen)
-   Mount
    -   Check that mountpoint and local directory to mount don’t overlap
        (Nick Craig-Wood)
    -   Fix mount size under 32 bit Windows (Nick Craig-Wood)
-   VFS
    -   Implement renaming of directories for backends without DirMove
        (Nick Craig-Wood)
        -   now all backends except b2 support renaming directories
    -   Implement --vfs-cache-max-size to limit the total size of the
        cache (Nick Craig-Wood)
    -   Add --dir-perms and --file-perms flags to set default
        permissions (Nick Craig-Wood)
    -   Fix deadlock on concurrent operations on a directory (Nick
        Craig-Wood)
    -   Fix deadlock between RWFileHandle.close and File.Remove (Nick
        Craig-Wood)
    -   Fix renaming/deleting open files with cache mode “writes” under
        Windows (Nick Craig-Wood)
    -   Fix panic on rename with --dry-run set (Nick Craig-Wood)
    -   Fix vfs/refresh with recurse=true needing the --fast-list flag
-   Local
    -   Add support for -l/--links (symbolic link translation)
        (yair@unicorn)
        -   this works by showing links as link.rclonelink - see local
            backend docs for more info
        -   this errors if used with -L/--copy-links
    -   Fix renaming/deleting open files on Windows (Nick Craig-Wood)
-   Crypt
    -   Check for maximum length before decrypting filename to fix panic
        (Garry McNulty)
-   Azure Blob
    -   Allow building azureblob backend on *BSD (themylogin)
    -   Use the rclone HTTP client to support --dump headers, --tpslimit
        etc (Nick Craig-Wood)
    -   Use the s3 pacer for 0 delay in non error conditions (Nick
        Craig-Wood)
    -   Ignore directory markers (Nick Craig-Wood)
    -   Stop Mkdir attempting to create existing containers (Nick
        Craig-Wood)
-   B2
    -   cleanup: will remove unfinished large files >24hrs old (Garry
        McNulty)
    -   For a bucket limited application key check the bucket name (Nick
        Craig-Wood)
        -   before this, rclone would use the authorised bucket
            regardless of what you put on the command line
    -   Added --b2-disable-checksum flag (Wojciech Smigielski)
        -   this enables large files to be uploaded without a SHA-1 hash
            for speed reasons
-   Drive
    -   Set default pacer to 100ms for 10 tps (Nick Craig-Wood)
        -   This fits the Google defaults much better and reduces the
            403 errors massively
        -   Add --drive-pacer-min-sleep and --drive-pacer-burst to
            control the pacer
    -   Improve ChangeNotify support for items with multiple parents
        (Fabian Möller)
    -   Fix ListR for items with multiple parents - this fixes oddities
        with vfs/refresh (Fabian Möller)
    -   Fix using --drive-impersonate and appfolders (Nick Craig-Wood)
    -   Fix google docs in rclone mount for some (not all) applications
        (Nick Craig-Wood)
-   Dropbox
    -   Retry-After support for Dropbox backend (Mathieu Carbou)
-   FTP
    -   Wait for 60 seconds for a connection to Close then declare it
        dead (Nick Craig-Wood)
        -   helps with indefinite hangs on some FTP servers
-   Google Cloud Storage
    -   Update google cloud storage endpoints (weetmuts)
-   HTTP
    -   Add an example with username and password which is supported but
        wasn’t documented (Nick Craig-Wood)
    -   Fix backend with --files-from and non-existent files (Nick
        Craig-Wood)
-   Hubic
    -   Make error message more informative if authentication fails
        (Nick Craig-Wood)
-   Jottacloud
    -   Resume and deduplication support (Oliver Heyme)
    -   Use token auth for all API requests Don’t store password anymore
        (Sebastian Bünger)
    -   Add support for 2-factor authentification (Sebastian Bünger)
-   Mega
    -   Implement v2 account login which fixes logins for newer Mega
        accounts (Nick Craig-Wood)
    -   Return error if an unknown length file is attempted to be
        uploaded (Nick Craig-Wood)
    -   Add new error codes for better error reporting (Nick Craig-Wood)
-   Onedrive
    -   Fix broken support for “shared with me” folders (Alex Chen)
    -   Fix root ID not normalised (Cnly)
    -   Return err instead of panic on unknown-sized uploads (Cnly)
-   Qingstor
    -   Fix go routine leak on multipart upload errors (Nick Craig-Wood)
    -   Add upload chunk size/concurrency/cutoff control (Nick
        Craig-Wood)
    -   Default --qingstor-upload-concurrency to 1 to work around bug
        (Nick Craig-Wood)
-   S3
    -   Implement --s3-upload-cutoff for single part uploads below this
        (Nick Craig-Wood)
    -   Change --s3-upload-concurrency default to 4 to increase
        perfomance (Nick Craig-Wood)
    -   Add --s3-bucket-acl to control bucket ACL (Nick Craig-Wood)
    -   Auto detect region for buckets on operation failure (Nick
        Craig-Wood)
    -   Add GLACIER storage class (William Cocker)
    -   Add Scaleway to s3 documentation (Rémy Léone)
    -   Add AWS endpoint eu-north-1 (weetmuts)
-   SFTP
    -   Add support for PEM encrypted private keys (Fabian Möller)
    -   Add option to force the usage of an ssh-agent (Fabian Möller)
    -   Perform environment variable expansion on key-file (Fabian
        Möller)
    -   Fix rmdir on Windows based servers (eg CrushFTP) (Nick
        Craig-Wood)
    -   Fix rmdir deleting directory contents on some SFTP servers (Nick
        Craig-Wood)
    -   Fix error on dangling symlinks (Nick Craig-Wood)
-   Swift
    -   Add --swift-no-chunk to disable segmented uploads in rcat/mount
        (Nick Craig-Wood)
    -   Introduce application credential auth support (kayrus)
    -   Fix memory usage by slimming Object (Nick Craig-Wood)
    -   Fix extra requests on upload (Nick Craig-Wood)
    -   Fix reauth on big files (Nick Craig-Wood)
-   Union
    -   Fix poll-interval not working (Nick Craig-Wood)
-   WebDAV
    -   Support About which means rclone mount will show the correct
        disk size (Nick Craig-Wood)
    -   Support MD5 and SHA1 hashes with Owncloud and Nextcloud (Nick
        Craig-Wood)
    -   Fail soft on time parsing errors (Nick Craig-Wood)
    -   Fix infinite loop on failed directory creation (Nick Craig-Wood)
    -   Fix identification of directories for Bitrix Site Manager (Nick
        Craig-Wood)
    -   Fix upload of 0 length files on some servers (Nick Craig-Wood)
    -   Fix if MKCOL fails with 423 Locked assume the directory exists
        (Nick Craig-Wood)


v1.45 - 2018-11-24

-   New backends
    -   The Yandex backend was re-written - see below for details
        (Sebastian Bünger)
-   New commands
    -   rcd: New command just to serve the remote control API (Nick
        Craig-Wood)
-   New Features
    -   The remote control API (rc) was greatly expanded to allow full
        control over rclone (Nick Craig-Wood)
        -   sensitive operations require authorization or the
            --rc-no-auth flag
        -   config/* operations to configure rclone
        -   options/* for reading/setting command line flags
        -   operations/* for all low level operations, eg copy file,
            list directory
        -   sync/* for sync, copy and move
        -   --rc-files flag to serve files on the rc http server
            -   this is for building web native GUIs for rclone
        -   Optionally serving objects on the rc http server
        -   Ensure rclone fails to start up if the --rc port is in use
            already
        -   See the rc docs for more info
    -   sync/copy/move
        -   Make --files-from only read the objects specified and don’t
            scan directories (Nick Craig-Wood)
            -   This is a huge speed improvement for destinations with
                lots of files
    -   filter: Add --ignore-case flag (Nick Craig-Wood)
    -   ncdu: Add remove function (‘d’ key) (Henning Surmeier)
    -   rc command
        -   Add --json flag for structured JSON input (Nick Craig-Wood)
        -   Add --user and --pass flags and interpret --rc-user,
            --rc-pass, --rc-addr (Nick Craig-Wood)
    -   build
        -   Require go1.8 or later for compilation (Nick Craig-Wood)
        -   Enable softfloat on MIPS arch (Scott Edlund)
        -   Integration test framework revamped with a better report and
            better retries (Nick Craig-Wood)
-   Bug Fixes
    -   cmd: Make –progress update the stats correctly at the end (Nick
        Craig-Wood)
    -   config: Create config directory on save if it is missing (Nick
        Craig-Wood)
    -   dedupe: Check for existing filename before renaming a dupe file
        (ssaqua)
    -   move: Don’t create directories with –dry-run (Nick Craig-Wood)
    -   operations: Fix Purge and Rmdirs when dir is not the root (Nick
        Craig-Wood)
    -   serve http/webdav/restic: Ensure rclone exits if the port is in
        use (Nick Craig-Wood)
-   Mount
    -   Make --volname work for Windows and macOS (Nick Craig-Wood)
-   Azure Blob
    -   Avoid context deadline exceeded error by setting a large
        TryTimeout value (brused27)
    -   Fix erroneous Rmdir error “directory not empty” (Nick
        Craig-Wood)
    -   Wait for up to 60s to create a just deleted container (Nick
        Craig-Wood)
-   Dropbox
    -   Add dropbox impersonate support (Jake Coggiano)
-   Jottacloud
    -   Fix bug in --fast-list handing of empty folders (albertony)
-   Opendrive
    -   Fix transfer of files with + and & in (Nick Craig-Wood)
    -   Fix retries of upload chunks (Nick Craig-Wood)
-   S3
    -   Set ACL for server side copies to that provided by the user
        (Nick Craig-Wood)
    -   Fix role_arn, credential_source, … (Erik Swanson)
    -   Add config info for Wasabi’s US-West endpoint (Henry Ptasinski)
-   SFTP
    -   Ensure file hash checking is really disabled (Jon Fautley)
-   Swift
    -   Add pacer for retries to make swift more reliable (Nick
        Craig-Wood)
-   WebDAV
    -   Add Content-Type to PUT requests (Nick Craig-Wood)
    -   Fix config parsing so --webdav-user and --webdav-pass flags work
        (Nick Craig-Wood)
    -   Add RFC3339 date format (Ralf Hemberger)
-   Yandex
    -   The yandex backend was re-written (Sebastian Bünger)
        -   This implements low level retries (Sebastian Bünger)
        -   Copy, Move, DirMove, PublicLink and About optional
            interfaces (Sebastian Bünger)
        -   Improved general error handling (Sebastian Bünger)
        -   Removed ListR for now due to inconsistent behaviour
            (Sebastian Bünger)


v1.44 - 2018-10-15

-   New commands
    -   serve ftp: Add ftp server (Antoine GIRARD)
    -   settier: perform storage tier changes on supported remotes
        (sandeepkru)
-   New Features
    -   Reworked command line help
        -   Make default help less verbose (Nick Craig-Wood)
        -   Split flags up into global and backend flags (Nick
            Craig-Wood)
        -   Implement specialised help for flags and backends (Nick
            Craig-Wood)
        -   Show URL of backend help page when starting config (Nick
            Craig-Wood)
    -   stats: Long names now split in center (Joanna Marek)
    -   Add –log-format flag for more control over log output (dcpu)
    -   rc: Add support for OPTIONS and basic CORS (frenos)
    -   stats: show FatalErrors and NoRetryErrors in stats (Cédric
        Connes)
-   Bug Fixes
    -   Fix -P not ending with a new line (Nick Craig-Wood)
    -   config: don’t create default config dir when user supplies
        –config (albertony)
    -   Don’t print non-ASCII characters with –progress on windows (Nick
        Craig-Wood)
    -   Correct logs for excluded items (ssaqua)
-   Mount
    -   Remove EXPERIMENTAL tags (Nick Craig-Wood)
-   VFS
    -   Fix race condition detected by serve ftp tests (Nick Craig-Wood)
    -   Add vfs/poll-interval rc command (Fabian Möller)
    -   Enable rename for nearly all remotes using server side Move or
        Copy (Nick Craig-Wood)
    -   Reduce directory cache cleared by poll-interval (Fabian Möller)
    -   Remove EXPERIMENTAL tags (Nick Craig-Wood)
-   Local
    -   Skip bad symlinks in dir listing with -L enabled (Cédric Connes)
    -   Preallocate files on Windows to reduce fragmentation (Nick
        Craig-Wood)
    -   Preallocate files on linux with fallocate(2) (Nick Craig-Wood)
-   Cache
    -   Add cache/fetch rc function (Fabian Möller)
    -   Fix worker scale down (Fabian Möller)
    -   Improve performance by not sending info requests for cached
        chunks (dcpu)
    -   Fix error return value of cache/fetch rc method (Fabian Möller)
    -   Documentation fix for cache-chunk-total-size (Anagh Kumar
        Baranwal)
    -   Preserve leading / in wrapped remote path (Fabian Möller)
    -   Add plex_insecure option to skip certificate validation (Fabian
        Möller)
    -   Remove entries that no longer exist in the source (dcpu)
-   Crypt
    -   Preserve leading / in wrapped remote path (Fabian Möller)
-   Alias
    -   Fix handling of Windows network paths (Nick Craig-Wood)
-   Azure Blob
    -   Add –azureblob-list-chunk parameter (Santiago Rodríguez)
    -   Implemented settier command support on azureblob remote.
        (sandeepkru)
    -   Work around SDK bug which causes errors for chunk-sized files
        (Nick Craig-Wood)
-   Box
    -   Implement link sharing. (Sebastian Bünger)
-   Drive
    -   Add –drive-import-formats - google docs can now be imported
        (Fabian Möller)
        -   Rewrite mime type and extension handling (Fabian Möller)
        -   Add document links (Fabian Möller)
        -   Add support for multipart document extensions (Fabian
            Möller)
        -   Add support for apps-script to json export (Fabian Möller)
        -   Fix escaped chars in documents during list (Fabian Möller)
    -   Add –drive-v2-download-min-size a workaround for slow downloads
        (Fabian Möller)
    -   Improve directory notifications in ChangeNotify (Fabian Möller)
    -   When listing team drives in config, continue on failure (Nick
        Craig-Wood)
-   FTP
    -   Add a small pause after failed upload before deleting file (Nick
        Craig-Wood)
-   Google Cloud Storage
    -   Fix service_account_file being ignored (Fabian Möller)
-   Jottacloud
    -   Minor improvement in quota info (omit if unlimited) (albertony)
    -   Add –fast-list support (albertony)
    -   Add permanent delete support: –jottacloud-hard-delete
        (albertony)
    -   Add link sharing support (albertony)
    -   Fix handling of reserved characters. (Sebastian Bünger)
    -   Fix socket leak on Object.Remove (Nick Craig-Wood)
-   Onedrive
    -   Rework to support Microsoft Graph (Cnly)
        -   NB this will require re-authenticating the remote
    -   Removed upload cutoff and always do session uploads (Oliver
        Heyme)
    -   Use single-part upload for empty files (Cnly)
    -   Fix new fields not saved when editing old config (Alex Chen)
    -   Fix sometimes special chars in filenames not replaced (Alex
        Chen)
    -   Ignore OneNote files by default (Alex Chen)
    -   Add link sharing support (jackyzy823)
-   S3
    -   Use custom pacer, to retry operations when reasonable (Craig
        Miskell)
    -   Use configured server-side-encryption and storace class options
        when calling CopyObject() (Paul Kohout)
    -   Make –s3-v2-auth flag (Nick Craig-Wood)
    -   Fix v2 auth on files with spaces (Nick Craig-Wood)
-   Union
    -   Implement union backend which reads from multiple backends
        (Felix Brucker)
    -   Implement optional interfaces (Move, DirMove, Copy etc) (Nick
        Craig-Wood)
    -   Fix ChangeNotify to support multiple remotes (Fabian Möller)
    -   Fix –backup-dir on union backend (Nick Craig-Wood)
-   WebDAV
    -   Add another time format (Nick Craig-Wood)
    -   Add a small pause after failed upload before deleting file (Nick
        Craig-Wood)
    -   Add workaround for missing mtime (buergi)
    -   Sharepoint: Renew cookies after 12hrs (Henning Surmeier)
-   Yandex
    -   Remove redundant nil checks (teresy)


v1.43.1 - 2018-09-07

Point release to fix hubic and azureblob backends.

-   Bug Fixes
    -   ncdu: Return error instead of log.Fatal in Show (Fabian Möller)
    -   cmd: Fix crash with –progress and –stats 0 (Nick Craig-Wood)
    -   docs: Tidy website display (Anagh Kumar Baranwal)
-   Azure Blob:
    -   Fix multi-part uploads. (sandeepkru)
-   Hubic
    -   Fix uploads (Nick Craig-Wood)
    -   Retry auth fetching if it fails to make hubic more reliable
        (Nick Craig-Wood)


v1.43 - 2018-09-01

-   New backends
    -   Jottacloud (Sebastian Bünger)
-   New commands
    -   copyurl: copies a URL to a remote (Denis)
-   New Features
    -   Reworked config for backends (Nick Craig-Wood)
        -   All backend config can now be supplied by command line, env
            var or config file
        -   Advanced section in the config wizard for the optional items
        -   A large step towards rclone backends being usable in other
            go software
        -   Allow on the fly remotes with :backend: syntax
    -   Stats revamp
        -   Add --progress/-P flag to show interactive progress (Nick
            Craig-Wood)
        -   Show the total progress of the sync in the stats (Nick
            Craig-Wood)
        -   Add --stats-one-line flag for single line stats (Nick
            Craig-Wood)
    -   Added weekday schedule into --bwlimit (Mateusz)
    -   lsjson: Add option to show the original object IDs (Fabian
        Möller)
    -   serve webdav: Make Content-Type without reading the file and add
        --etag-hash (Nick Craig-Wood)
    -   build
        -   Build macOS with native compiler (Nick Craig-Wood)
        -   Update to use go1.11 for the build (Nick Craig-Wood)
    -   rc
        -   Added core/stats to return the stats (reddi1)
    -   version --check: Prints the current release and beta versions
        (Nick Craig-Wood)
-   Bug Fixes
    -   accounting
        -   Fix time to completion estimates (Nick Craig-Wood)
        -   Fix moving average speed for file stats (Nick Craig-Wood)
    -   config: Fix error reading password from piped input (Nick
        Craig-Wood)
    -   move: Fix --delete-empty-src-dirs flag to delete all empty dirs
        on move (ishuah)
-   Mount
    -   Implement --daemon-timeout flag for OSXFUSE (Nick Craig-Wood)
    -   Fix mount --daemon not working with encrypted config (Alex Chen)
    -   Clip the number of blocks to 2^32-1 on macOS - fixes borg backup
        (Nick Craig-Wood)
-   VFS
    -   Enable vfs-read-chunk-size by default (Fabian Möller)
    -   Add the vfs/refresh rc command (Fabian Möller)
    -   Add non recursive mode to vfs/refresh rc command (Fabian Möller)
    -   Try to seek buffer on read only files (Fabian Möller)
-   Local
    -   Fix crash when deprecated --local-no-unicode-normalization is
        supplied (Nick Craig-Wood)
    -   Fix mkdir error when trying to copy files to the root of a drive
        on windows (Nick Craig-Wood)
-   Cache
    -   Fix nil pointer deref when using lsjson on cached directory
        (Nick Craig-Wood)
    -   Fix nil pointer deref for occasional crash on playback (Nick
        Craig-Wood)
-   Crypt
    -   Fix accounting when checking hashes on upload (Nick Craig-Wood)
-   Amazon Cloud Drive
    -   Make very clear in the docs that rclone has no ACD keys (Nick
        Craig-Wood)
-   Azure Blob
    -   Add connection string and SAS URL auth (Nick Craig-Wood)
    -   List the container to see if it exists (Nick Craig-Wood)
    -   Port new Azure Blob Storage SDK (sandeepkru)
    -   Added blob tier, tier between Hot, Cool and Archive.
        (sandeepkru)
    -   Remove leading / from paths (Nick Craig-Wood)
-   B2
    -   Support Application Keys (Nick Craig-Wood)
    -   Remove leading / from paths (Nick Craig-Wood)
-   Box
    -   Fix upload of > 2GB files on 32 bit platforms (Nick Craig-Wood)
    -   Make --box-commit-retries flag defaulting to 100 to fix large
        uploads (Nick Craig-Wood)
-   Drive
    -   Add --drive-keep-revision-forever flag (lewapm)
    -   Handle gdocs when filtering file names in list (Fabian Möller)
    -   Support using --fast-list for large speedups (Fabian Möller)
-   FTP
    -   Fix Put mkParentDir failed: 521 for BunnyCDN (Nick Craig-Wood)
-   Google Cloud Storage
    -   Fix index out of range error with --fast-list (Nick Craig-Wood)
-   Jottacloud
    -   Fix MD5 error check (Oliver Heyme)
    -   Handle empty time values (Martin Polden)
    -   Calculate missing MD5s (Oliver Heyme)
    -   Docs, fixes and tests for MD5 calculation (Nick Craig-Wood)
    -   Add optional MimeTyper interface. (Sebastian Bünger)
    -   Implement optional About interface (for df support). (Sebastian
        Bünger)
-   Mega
    -   Wait for events instead of arbitrary sleeping (Nick Craig-Wood)
    -   Add --mega-hard-delete flag (Nick Craig-Wood)
    -   Fix failed logins with upper case chars in email (Nick
        Craig-Wood)
-   Onedrive
    -   Shared folder support (Yoni Jah)
    -   Implement DirMove (Cnly)
    -   Fix rmdir sometimes deleting directories with contents (Nick
        Craig-Wood)
-   Pcloud
    -   Delete half uploaded files on upload error (Nick Craig-Wood)
-   Qingstor
    -   Remove leading / from paths (Nick Craig-Wood)
-   S3
    -   Fix index out of range error with --fast-list (Nick Craig-Wood)
    -   Add --s3-force-path-style (Nick Craig-Wood)
    -   Add support for KMS Key ID (bsteiss)
    -   Remove leading / from paths (Nick Craig-Wood)
-   Swift
    -   Add storage_policy (Ruben Vandamme)
    -   Make it so just storage_url or auth_token can be overidden (Nick
        Craig-Wood)
    -   Fix server side copy bug for unusal file names (Nick Craig-Wood)
    -   Remove leading / from paths (Nick Craig-Wood)
-   WebDAV
    -   Ensure we call MKCOL with a URL with a trailing / for QNAP
        interop (Nick Craig-Wood)
    -   If root ends with / then don’t check if it is a file (Nick
        Craig-Wood)
    -   Don’t accept redirects when reading metadata (Nick Craig-Wood)
    -   Add bearer token (Macaroon) support for dCache (Nick Craig-Wood)
    -   Document dCache and Macaroons (Onno Zweers)
    -   Sharepoint recursion with different depth (Henning)
    -   Attempt to remove failed uploads (Nick Craig-Wood)
-   Yandex
    -   Fix listing/deleting files in the root (Nick Craig-Wood)


v1.42 - 2018-06-16

-   New backends
    -   OpenDrive (Oliver Heyme, Jakub Karlicek, ncw)
-   New commands
    -   deletefile command (Filip Bartodziej)
-   New Features
    -   copy, move: Copy single files directly, don’t use --files-from
        work-around
        -   this makes them much more efficient
    -   Implement --max-transfer flag to quit transferring at a limit
        -   make exit code 8 for --max-transfer exceeded
    -   copy: copy empty source directories to destination (Ishuah
        Kariuki)
    -   check: Add --one-way flag (Kasper Byrdal Nielsen)
    -   Add siginfo handler for macOS for ctrl-T stats (kubatasiemski)
    -   rc
        -   add core/gc to run a garbage collection on demand
        -   enable go profiling by default on the --rc port
        -   return error from remote on failure
    -   lsf
        -   Add --absolute flag to add a leading / onto path names
        -   Add --csv flag for compliant CSV output
        -   Add ‘m’ format specifier to show the MimeType
        -   Implement ‘i’ format for showing object ID
    -   lsjson
        -   Add MimeType to the output
        -   Add ID field to output to show Object ID
    -   Add --retries-sleep flag (Benjamin Joseph Dag)
    -   Oauth tidy up web page and error handling (Henning Surmeier)
-   Bug Fixes
    -   Password prompt output with --log-file fixed for unix (Filip
        Bartodziej)
    -   Calculate ModifyWindow each time on the fly to fix various
        problems (Stefan Breunig)
-   Mount
    -   Only print “File.rename error” if there actually is an error
        (Stefan Breunig)
    -   Delay rename if file has open writers instead of failing
        outright (Stefan Breunig)
    -   Ensure atexit gets run on interrupt
    -   macOS enhancements
        -   Make --noappledouble --noapplexattr
        -   Add --volname flag and remove special chars from it
        -   Make Get/List/Set/Remove xattr return ENOSYS for efficiency
        -   Make --daemon work for macOS without CGO
-   VFS
    -   Add --vfs-read-chunk-size and --vfs-read-chunk-size-limit
        (Fabian Möller)
    -   Fix ChangeNotify for new or changed folders (Fabian Möller)
-   Local
    -   Fix symlink/junction point directory handling under Windows
        -   NB you will need to add -L to your command line to copy
            files with reparse points
-   Cache
    -   Add non cached dirs on notifications (Remus Bunduc)
    -   Allow root to be expired from rc (Remus Bunduc)
    -   Clean remaining empty folders from temp upload path (Remus
        Bunduc)
    -   Cache lists using batch writes (Remus Bunduc)
    -   Use secure websockets for HTTPS Plex addresses (John Clayton)
    -   Reconnect plex websocket on failures (Remus Bunduc)
    -   Fix panic when running without plex configs (Remus Bunduc)
    -   Fix root folder caching (Remus Bunduc)
-   Crypt
    -   Check the crypted hash of files when uploading for extra data
        security
-   Dropbox
    -   Make Dropbox for business folders accessible using an initial /
        in the path
-   Google Cloud Storage
    -   Low level retry all operations if necessary
-   Google Drive
    -   Add --drive-acknowledge-abuse to download flagged files
    -   Add --drive-alternate-export to fix large doc export
    -   Don’t attempt to choose Team Drives when using rclone config
        create
    -   Fix change list polling with team drives
    -   Fix ChangeNotify for folders (Fabian Möller)
    -   Fix about (and df on a mount) for team drives
-   Onedrive
    -   Errorhandler for onedrive for business requests (Henning
        Surmeier)
-   S3
    -   Adjust upload concurrency with --s3-upload-concurrency
        (themylogin)
    -   Fix --s3-chunk-size which was always using the minimum
-   SFTP
    -   Add --ssh-path-override flag (Piotr Oleszczyk)
    -   Fix slow downloads for long latency connections
-   Webdav
    -   Add workarounds for biz.mail.ru
    -   Ignore Reason-Phrase in status line to fix 4shared (Rodrigo)
    -   Better error message generation


v1.41 - 2018-04-28

-   New backends
    -   Mega support added
    -   Webdav now supports SharePoint cookie authentication (hensur)
-   New commands
    -   link: create public link to files and folders (Stefan Breunig)
    -   about: gets quota info from a remote (a-roussos, ncw)
    -   hashsum: a generic tool for any hash to produce md5sum like
        output
-   New Features
    -   lsd: Add -R flag and fix and update docs for all ls commands
    -   ncdu: added a “refresh” key - CTRL-L (Keith Goldfarb)
    -   serve restic: Add append-only mode (Steve Kriss)
    -   serve restic: Disallow overwriting files in append-only mode
        (Alexander Neumann)
    -   serve restic: Print actual listener address (Matt Holt)
    -   size: Add –json flag (Matthew Holt)
    -   sync: implement –ignore-errors (Mateusz Pabian)
    -   dedupe: Add dedupe largest functionality (Richard Yang)
    -   fs: Extend SizeSuffix to include TB and PB for rclone about
    -   fs: add –dump goroutines and –dump openfiles for debugging
    -   rc: implement core/memstats to print internal memory usage info
    -   rc: new call rc/pid (Michael P. Dubner)
-   Compile
    -   Drop support for go1.6
-   Release
    -   Fix make tarball (Chih-Hsuan Yen)
-   Bug Fixes
    -   filter: fix –min-age and –max-age together check
    -   fs: limit MaxIdleConns and MaxIdleConnsPerHost in transport
    -   lsd,lsf: make sure all times we output are in local time
    -   rc: fix setting bwlimit to unlimited
    -   rc: take note of the –rc-addr flag too as per the docs
-   Mount
    -   Use About to return the correct disk total/used/free (eg in df)
    -   Set --attr-timeout default to 1s - fixes:
        -   rclone using too much memory
        -   rclone not serving files to samba
        -   excessive time listing directories
    -   Fix df -i (upstream fix)
-   VFS
    -   Filter files . and .. from directory listing
    -   Only make the VFS cache if –vfs-cache-mode > Off
-   Local
    -   Add –local-no-check-updated to disable updated file checks
    -   Retry remove on Windows sharing violation error
-   Cache
    -   Flush the memory cache after close
    -   Purge file data on notification
    -   Always forget parent dir for notifications
    -   Integrate with Plex websocket
    -   Add rc cache/stats (seuffert)
    -   Add info log on notification
-   Box
    -   Fix failure reading large directories - parse file/directory
        size as float
-   Dropbox
    -   Fix crypt+obfuscate on dropbox
    -   Fix repeatedly uploading the same files
-   FTP
    -   Work around strange response from box FTP server
    -   More workarounds for FTP servers to fix mkParentDir error
    -   Fix no error on listing non-existent directory
-   Google Cloud Storage
    -   Add service_account_credentials (Matt Holt)
    -   Detect bucket presence by listing it - minimises permissions
        needed
    -   Ignore zero length directory markers
-   Google Drive
    -   Add service_account_credentials (Matt Holt)
    -   Fix directory move leaving a hardlinked directory behind
    -   Return proper google errors when Opening files
    -   When initialized with a filepath, optional features used
        incorrect root path (Stefan Breunig)
-   HTTP
    -   Fix sync for servers which don’t return Content-Length in HEAD
-   Onedrive
    -   Add QuickXorHash support for OneDrive for business
    -   Fix socket leak in multipart session upload
-   S3
    -   Look in S3 named profile files for credentials
    -   Add --s3-disable-checksum to disable checksum uploading (Chris
        Redekop)
    -   Hierarchical configuration support (Giri Badanahatti)
    -   Add in config for all the supported S3 providers
    -   Add One Zone Infrequent Access storage class (Craig Rachel)
    -   Add –use-server-modtime support (Peter Baumgartner)
    -   Add –s3-chunk-size option to control multipart uploads
    -   Ignore zero length directory markers
-   SFTP
    -   Update docs to match code, fix typos and clarify
        disable_hashcheck prompt (Michael G. Noll)
    -   Update docs with Synology quirks
    -   Fail soft with a debug on hash failure
-   Swift
    -   Add –use-server-modtime support (Peter Baumgartner)
-   Webdav
    -   Support SharePoint cookie authentication (hensur)
    -   Strip leading and trailing / off root


v1.40 - 2018-03-19

-   New backends
    -   Alias backend to create aliases for existing remote names
        (Fabian Möller)
-   New commands
    -   lsf: list for parsing purposes (Jakub Tasiemski)
        -   by default this is a simple non recursive list of files and
            directories
        -   it can be configured to add more info in an easy to parse
            way
    -   serve restic: for serving a remote as a Restic REST endpoint
        -   This enables restic to use any backends that rclone can
            access
        -   Thanks Alexander Neumann for help, patches and review
    -   rc: enable the remote control of a running rclone
        -   The running rclone must be started with –rc and related
            flags.
        -   Currently there is support for bwlimit, and flushing for
            mount and cache.
-   New Features
    -   --max-delete flag to add a delete threshold (Bjørn Erik
        Pedersen)
    -   All backends now support RangeOption for ranged Open
        -   cat: Use RangeOption for limited fetches to make more
            efficient
        -   cryptcheck: make reading of nonce more efficient with
            RangeOption
    -   serve http/webdav/restic
        -   support SSL/TLS
        -   add --user --pass and --htpasswd for authentication
    -   copy/move: detect file size change during copy/move and abort
        transfer (ishuah)
    -   cryptdecode: added option to return encrypted file names.
        (ishuah)
    -   lsjson: add --encrypted to show encrypted name (Jakub Tasiemski)
    -   Add --stats-file-name-length to specify the printed file name
        length for stats (Will Gunn)
-   Compile
    -   Code base was shuffled and factored
        -   backends moved into a backend directory
        -   large packages split up
        -   See the CONTRIBUTING.md doc for info as to what lives where
            now
    -   Update to using go1.10 as the default go version
    -   Implement daily full integration tests
-   Release
    -   Include a source tarball and sign it and the binaries
    -   Sign the git tags as part of the release process
    -   Add .deb and .rpm packages as part of the build
    -   Make a beta release for all branches on the main repo (but not
        pull requests)
-   Bug Fixes
    -   config: fixes errors on non existing config by loading config
        file only on first access
    -   config: retry saving the config after failure (Mateusz)
    -   sync: when using --backup-dir don’t delete files if we can’t set
        their modtime
        -   this fixes odd behaviour with Dropbox and --backup-dir
    -   fshttp: fix idle timeouts for HTTP connections
    -   serve http: fix serving files with : in - fixes
    -   Fix --exclude-if-present to ignore directories which it doesn’t
        have permission for (Iakov Davydov)
    -   Make accounting work properly with crypt and b2
    -   remove --no-traverse flag because it is obsolete
-   Mount
    -   Add --attr-timeout flag to control attribute caching in kernel
        -   this now defaults to 0 which is correct but less efficient
        -   see the mount docs for more info
    -   Add --daemon flag to allow mount to run in the background
        (ishuah)
    -   Fix: Return ENOSYS rather than EIO on attempted link
        -   This fixes FileZilla accessing an rclone mount served over
            sftp.
    -   Fix setting modtime twice
    -   Mount tests now run on CI for Linux (mount & cmount)/Mac/Windows
    -   Many bugs fixed in the VFS layer - see below
-   VFS
    -   Many fixes for --vfs-cache-mode writes and above
        -   Update cached copy if we know it has changed (fixes stale
            data)
        -   Clean path names before using them in the cache
        -   Disable cache cleaner if --vfs-cache-poll-interval=0
        -   Fill and clean the cache immediately on startup
    -   Fix Windows opening every file when it stats the file
    -   Fix applying modtime for an open Write Handle
    -   Fix creation of files when truncating
    -   Write 0 bytes when flushing unwritten handles to avoid race
        conditions in FUSE
    -   Downgrade “poll-interval is not supported” message to Info
    -   Make OpenFile and friends return EINVAL if O_RDONLY and O_TRUNC
-   Local
    -   Downgrade “invalid cross-device link: trying copy” to debug
    -   Make DirMove return fs.ErrorCantDirMove to allow fallback to
        Copy for cross device
    -   Fix race conditions updating the hashes
-   Cache
    -   Add support for polling - cache will update when remote changes
        on supported backends
    -   Reduce log level for Plex api
    -   Fix dir cache issue
    -   Implement --cache-db-wait-time flag
    -   Improve efficiency with RangeOption and RangeSeek
    -   Fix dirmove with temp fs enabled
    -   Notify vfs when using temp fs
    -   Offline uploading
    -   Remote control support for path flushing
-   Amazon cloud drive
    -   Rclone no longer has any working keys - disable integration
        tests
    -   Implement DirChangeNotify to notify cache/vfs/mount of changes
-   Azureblob
    -   Don’t check for bucket/container presense if listing was OK
        -   this makes rclone do one less request per invocation
    -   Improve accounting for chunked uploads
-   Backblaze B2
    -   Don’t check for bucket/container presense if listing was OK
        -   this makes rclone do one less request per invocation
-   Box
    -   Improve accounting for chunked uploads
-   Dropbox
    -   Fix custom oauth client parameters
-   Google Cloud Storage
    -   Don’t check for bucket/container presense if listing was OK
        -   this makes rclone do one less request per invocation
-   Google Drive
    -   Migrate to api v3 (Fabian Möller)
    -   Add scope configuration and root folder selection
    -   Add --drive-impersonate for service accounts
        -   thanks to everyone who tested, explored and contributed docs
    -   Add --drive-use-created-date to use created date as modified
        date (nbuchanan)
    -   Request the export formats only when required
        -   This makes rclone quicker when there are no google docs
    -   Fix finding paths with latin1 chars (a workaround for a drive
        bug)
    -   Fix copying of a single Google doc file
    -   Fix --drive-auth-owner-only to look in all directories
-   HTTP
    -   Fix handling of directories with & in
-   Onedrive
    -   Removed upload cutoff and always do session uploads
        -   this stops the creation of multiple versions on business
            onedrive
    -   Overwrite object size value with real size when reading file.
        (Victor)
        -   this fixes oddities when onedrive misreports the size of
            images
-   Pcloud
    -   Remove unused chunked upload flag and code
-   Qingstor
    -   Don’t check for bucket/container presense if listing was OK
        -   this makes rclone do one less request per invocation
-   S3
    -   Support hashes for multipart files (Chris Redekop)
    -   Initial support for IBM COS (S3) (Giri Badanahatti)
    -   Update docs to discourage use of v2 auth with CEPH and others
    -   Don’t check for bucket/container presense if listing was OK
        -   this makes rclone do one less request per invocation
    -   Fix server side copy and set modtime on files with + in
-   SFTP
    -   Add option to disable remote hash check command execution (Jon
        Fautley)
    -   Add --sftp-ask-password flag to prompt for password when needed
        (Leo R. Lundgren)
    -   Add set_modtime configuration option
    -   Fix following of symlinks
    -   Fix reading config file outside of Fs setup
    -   Fix reading $USER in username fallback not $HOME
    -   Fix running under crontab - Use correct OS way of reading
        username
-   Swift
    -   Fix refresh of authentication token
        -   in v1.39 a bug was introduced which ignored new tokens -
            this fixes it
    -   Fix extra HEAD transaction when uploading a new file
    -   Don’t check for bucket/container presense if listing was OK
        -   this makes rclone do one less request per invocation
-   Webdav
    -   Add new time formats to support mydrive.ch and others


v1.39 - 2017-12-23

-   New backends
    -   WebDAV
        -   tested with nextcloud, owncloud, put.io and others!
    -   Pcloud
    -   cache - wraps a cache around other backends (Remus Bunduc)
        -   useful in combination with mount
        -   NB this feature is in beta so use with care
-   New commands
    -   serve command with subcommands:
        -   serve webdav: this implements a webdav server for any rclone
            remote.
        -   serve http: command to serve a remote over HTTP
    -   config: add sub commands for full config file management
        -   create/delete/dump/edit/file/password/providers/show/update
    -   touch: to create or update the timestamp of a file (Jakub
        Tasiemski)
-   New Features
    -   curl install for rclone (Filip Bartodziej)
    -   –stats now shows percentage, size, rate and ETA in condensed
        form (Ishuah Kariuki)
    -   –exclude-if-present to exclude a directory if a file is present
        (Iakov Davydov)
    -   rmdirs: add –leave-root flag (lewpam)
    -   move: add –delete-empty-src-dirs flag to remove dirs after move
        (Ishuah Kariuki)
    -   Add –dump flag, introduce –dump requests, responses and remove
        –dump-auth, –dump-filters
        -   Obscure X-Auth-Token: from headers when dumping too
    -   Document and implement exit codes for different failure modes
        (Ishuah Kariuki)
-   Compile
-   Bug Fixes
    -   Retry lots more different types of errors to make multipart
        transfers more reliable
    -   Save the config before asking for a token, fixes disappearing
        oauth config
    -   Warn the user if –include and –exclude are used together (Ernest
        Borowski)
    -   Fix duplicate files (eg on Google drive) causing spurious copies
    -   Allow trailing and leading whitespace for passwords (Jason Rose)
    -   ncdu: fix crashes on empty directories
    -   rcat: fix goroutine leak
    -   moveto/copyto: Fix to allow copying to the same name
-   Mount
    -   –vfs-cache mode to make writes into mounts more reliable.
        -   this requires caching files on the disk (see –cache-dir)
        -   As this is a new feature, use with care
    -   Use sdnotify to signal systemd the mount is ready (Fabian
        Möller)
    -   Check if directory is not empty before mounting (Ernest
        Borowski)
-   Local
    -   Add error message for cross file system moves
    -   Fix equality check for times
-   Dropbox
    -   Rework multipart upload
        -   buffer the chunks when uploading large files so they can be
            retried
        -   change default chunk size to 48MB now we are buffering them
            in memory
        -   retry every error after the first chunk is done successfully
    -   Fix error when renaming directories
-   Swift
    -   Fix crash on bad authentication
-   Google Drive
    -   Add service account support (Tim Cooijmans)
-   S3
    -   Make it work properly with Digital Ocean Spaces (Andrew
        Starr-Bochicchio)
    -   Fix crash if a bad listing is received
    -   Add support for ECS task IAM roles (David Minor)
-   Backblaze B2
    -   Fix multipart upload retries
    -   Fix –hard-delete to make it work 100% of the time
-   Swift
    -   Allow authentication with storage URL and auth key (Giovanni
        Pizzi)
    -   Add new fields for swift configuration to support IBM Bluemix
        Swift (Pierre Carlson)
    -   Add OS_TENANT_ID and OS_USER_ID to config
    -   Allow configs with user id instead of user name
    -   Check if swift segments container exists before creating (John
        Leach)
    -   Fix memory leak in swift transfers (upstream fix)
-   SFTP
    -   Add option to enable the use of aes128-cbc cipher (Jon Fautley)
-   Amazon cloud drive
    -   Fix download of large files failing with “Only one auth
        mechanism allowed”
-   crypt
    -   Option to encrypt directory names or leave them intact
    -   Implement DirChangeNotify (Fabian Möller)
-   onedrive
    -   Add option to choose resourceURL during setup of OneDrive
        Business account if more than one is available for user


v1.38 - 2017-09-30

-   New backends
    -   Azure Blob Storage (thanks Andrei Dragomir)
    -   Box
    -   Onedrive for Business (thanks Oliver Heyme)
    -   QingStor from QingCloud (thanks wuyu)
-   New commands
    -   rcat - read from standard input and stream upload
    -   tree - shows a nicely formatted recursive listing
    -   cryptdecode - decode crypted file names (thanks ishuah)
    -   config show - print the config file
    -   config file - print the config file location
-   New Features
    -   Empty directories are deleted on sync
    -   dedupe - implement merging of duplicate directories
    -   check and cryptcheck made more consistent and use less memory
    -   cleanup for remaining remotes (thanks ishuah)
    -   --immutable for ensuring that files don’t change (thanks Jacob
        McNamee)
    -   --user-agent option (thanks Alex McGrath Kraak)
    -   --disable flag to disable optional features
    -   --bind flag for choosing the local addr on outgoing connections
    -   Support for zsh auto-completion (thanks bpicode)
    -   Stop normalizing file names but do a normalized compare in sync
-   Compile
    -   Update to using go1.9 as the default go version
    -   Remove snapd build due to maintenance problems
-   Bug Fixes
    -   Improve retriable error detection which makes multipart uploads
        better
    -   Make check obey --ignore-size
    -   Fix bwlimit toggle in conjunction with schedules (thanks
        cbruegg)
    -   config ensures newly written config is on the same mount
-   Local
    -   Revert to copy when moving file across file system boundaries
    -   --skip-links to suppress symlink warnings (thanks Zhiming Wang)
-   Mount
    -   Re-use rcat internals to support uploads from all remotes
-   Dropbox
    -   Fix “entry doesn’t belong in directory” error
    -   Stop using deprecated API methods
-   Swift
    -   Fix server side copy to empty container with --fast-list
-   Google Drive
    -   Change the default for --drive-use-trash to true
-   S3
    -   Set session token when using STS (thanks Girish Ramakrishnan)
    -   Glacier docs and error messages (thanks Jan Varho)
    -   Read 1000 (not 1024) items in dir listings to fix Wasabi
-   Backblaze B2
    -   Fix SHA1 mismatch when downloading files with no SHA1
    -   Calculate missing hashes on the fly instead of spooling
    -   --b2-hard-delete to permanently delete (not hide) files (thanks
        John Papandriopoulos)
-   Hubic
    -   Fix creating containers - no longer have to use the default
        container
-   Swift
    -   Optionally configure from a standard set of OpenStack
        environment vars
    -   Add endpoint_type config
-   Google Cloud Storage
    -   Fix bucket creation to work with limited permission users
-   SFTP
    -   Implement connection pooling for multiple ssh connections
    -   Limit new connections per second
    -   Add support for MD5 and SHA1 hashes where available (thanks
        Christian Brüggemann)
-   HTTP
    -   Fix URL encoding issues
    -   Fix directories with : in
    -   Fix panic with URL encoded content


v1.37 - 2017-07-22

-   New backends
    -   FTP - thanks to Antonio Messina
    -   HTTP - thanks to Vasiliy Tolstov
-   New commands
    -   rclone ncdu - for exploring a remote with a text based user
        interface.
    -   rclone lsjson - for listing with a machine readable output
    -   rclone dbhashsum - to show Dropbox style hashes of files (local
        or Dropbox)
-   New Features
    -   Implement –fast-list flag
        -   This allows remotes to list recursively if they can
        -   This uses less transactions (important if you pay for them)
        -   This may or may not be quicker
        -   This will use more memory as it has to hold the listing in
            memory
        -   –old-sync-method deprecated - the remaining uses are covered
            by –fast-list
        -   This involved a major re-write of all the listing code
    -   Add –tpslimit and –tpslimit-burst to limit transactions per
        second
        -   this is useful in conjuction with rclone mount to limit
            external apps
    -   Add –stats-log-level so can see –stats without -v
    -   Print password prompts to stderr - Hraban Luyat
    -   Warn about duplicate files when syncing
    -   Oauth improvements
        -   allow auth_url and token_url to be set in the config file
        -   Print redirection URI if using own credentials.
    -   Don’t Mkdir at the start of sync to save transactions
-   Compile
    -   Update build to go1.8.3
    -   Require go1.6 for building rclone
    -   Compile 386 builds with “GO386=387” for maximum compatibility
-   Bug Fixes
    -   Fix menu selection when no remotes
    -   Config saving reworked to not kill the file if disk gets full
    -   Don’t delete remote if name does not change while renaming
    -   moveto, copyto: report transfers and checks as per move and copy
-   Local
    -   Add –local-no-unicode-normalization flag - Bob Potter
-   Mount
    -   Now supported on Windows using cgofuse and WinFsp - thanks to
        Bill Zissimopoulos for much help
    -   Compare checksums on upload/download via FUSE
    -   Unmount when program ends with SIGINT (Ctrl+C) or SIGTERM -
        Jérôme Vizcaino
    -   On read only open of file, make open pending until first read
    -   Make –read-only reject modify operations
    -   Implement ModTime via FUSE for remotes that support it
    -   Allow modTime to be changed even before all writers are closed
    -   Fix panic on renames
    -   Fix hang on errored upload
-   Crypt
    -   Report the name:root as specified by the user
    -   Add an “obfuscate” option for filename encryption - Stephen
        Harris
-   Amazon Drive
    -   Fix initialization order for token renewer
    -   Remove revoked credentials, allow oauth proxy config and update
        docs
-   B2
    -   Reduce minimum chunk size to 5MB
-   Drive
    -   Add team drive support
    -   Reduce bandwidth by adding fields for partial responses - Martin
        Kristensen
    -   Implement –drive-shared-with-me flag to view shared with me
        files - Danny Tsai
    -   Add –drive-trashed-only to read only the files in the trash
    -   Remove obsolete –drive-full-list
    -   Add missing seek to start on retries of chunked uploads
    -   Fix stats accounting for upload
    -   Convert / in names to a unicode equivalent (／)
    -   Poll for Google Drive changes when mounted
-   OneDrive
    -   Fix the uploading of files with spaces
    -   Fix initialization order for token renewer
    -   Display speeds accurately when uploading - Yoni Jah
    -   Swap to using http://localhost:53682/ as redirect URL - Michael
        Ledin
    -   Retry on token expired error, reset upload body on retry - Yoni
        Jah
-   Google Cloud Storage
    -   Add ability to specify location and storage class via config and
        command line - thanks gdm85
    -   Create container if necessary on server side copy
    -   Increase directory listing chunk to 1000 to increase performance
    -   Obtain a refresh token for GCS - Steven Lu
-   Yandex
    -   Fix the name reported in log messages (was empty)
    -   Correct error return for listing empty directory
-   Dropbox
    -   Rewritten to use the v2 API
        -   Now supports ModTime
            -   Can only set by uploading the file again
            -   If you uploaded with an old rclone, rclone may upload
                everything again
            -   Use --size-only or --checksum to avoid this
        -   Now supports the Dropbox content hashing scheme
        -   Now supports low level retries
-   S3
    -   Work around eventual consistency in bucket creation
    -   Create container if necessary on server side copy
    -   Add us-east-2 (Ohio) and eu-west-2 (London) S3 regions - Zahiar
        Ahmed
-   Swift, Hubic
    -   Fix zero length directory markers showing in the subdirectory
        listing
        -   this caused lots of duplicate transfers
    -   Fix paged directory listings
        -   this caused duplicate directory errors
    -   Create container if necessary on server side copy
    -   Increase directory listing chunk to 1000 to increase performance
    -   Make sensible error if the user forgets the container
-   SFTP
    -   Add support for using ssh key files
    -   Fix under Windows
    -   Fix ssh agent on Windows
    -   Adapt to latest version of library - Igor Kharin


v1.36 - 2017-03-18

-   New Features
    -   SFTP remote (Jack Schmidt)
    -   Re-implement sync routine to work a directory at a time reducing
        memory usage
    -   Logging revamped to be more inline with rsync - now much
        quieter * -v only shows transfers * -vv is for full debug *
        –syslog to log to syslog on capable platforms
    -   Implement –backup-dir and –suffix
    -   Implement –track-renames (initial implementation by Bjørn Erik
        Pedersen)
    -   Add time-based bandwidth limits (Lukas Loesche)
    -   rclone cryptcheck: checks integrity of crypt remotes
    -   Allow all config file variables and options to be set from
        environment variables
    -   Add –buffer-size parameter to control buffer size for copy
    -   Make –delete-after the default
    -   Add –ignore-checksum flag (fixed by Hisham Zarka)
    -   rclone check: Add –download flag to check all the data, not just
        hashes
    -   rclone cat: add –head, –tail, –offset, –count and –discard
    -   rclone config: when choosing from a list, allow the value to be
        entered too
    -   rclone config: allow rename and copy of remotes
    -   rclone obscure: for generating encrypted passwords for rclone’s
        config (T.C. Ferguson)
    -   Comply with XDG Base Directory specification (Dario Giovannetti)
        -   this moves the default location of the config file in a
            backwards compatible way
    -   Release changes
        -   Ubuntu snap support (Dedsec1)
        -   Compile with go 1.8
        -   MIPS/Linux big and little endian support
-   Bug Fixes
    -   Fix copyto copying things to the wrong place if the destination
        dir didn’t exist
    -   Fix parsing of remotes in moveto and copyto
    -   Fix –delete-before deleting files on copy
    -   Fix –files-from with an empty file copying everything
    -   Fix sync: don’t update mod times if –dry-run set
    -   Fix MimeType propagation
    -   Fix filters to add ** rules to directory rules
-   Local
    -   Implement -L, –copy-links flag to allow rclone to follow
        symlinks
    -   Open files in write only mode so rclone can write to an rclone
        mount
    -   Fix unnormalised unicode causing problems reading directories
    -   Fix interaction between -x flag and –max-depth
-   Mount
    -   Implement proper directory handling (mkdir, rmdir, renaming)
    -   Make include and exclude filters apply to mount
    -   Implement read and write async buffers - control with
        –buffer-size
    -   Fix fsync on for directories
    -   Fix retry on network failure when reading off crypt
-   Crypt
    -   Add –crypt-show-mapping to show encrypted file mapping
    -   Fix crypt writer getting stuck in a loop
        -   IMPORTANT this bug had the potential to cause data
            corruption when
            -   reading data from a network based remote and
            -   writing to a crypt on Google Drive
        -   Use the cryptcheck command to validate your data if you are
            concerned
        -   If syncing two crypt remotes, sync the unencrypted remote
-   Amazon Drive
    -   Fix panics on Move (rename)
    -   Fix panic on token expiry
-   B2
    -   Fix inconsistent listings and rclone check
    -   Fix uploading empty files with go1.8
    -   Constrain memory usage when doing multipart uploads
    -   Fix upload url not being refreshed properly
-   Drive
    -   Fix Rmdir on directories with trashed files
    -   Fix “Ignoring unknown object” when downloading
    -   Add –drive-list-chunk
    -   Add –drive-skip-gdocs (Károly Oláh)
-   OneDrive
    -   Implement Move
    -   Fix Copy
        -   Fix overwrite detection in Copy
        -   Fix waitForJob to parse errors correctly
    -   Use token renewer to stop auth errors on long uploads
    -   Fix uploading empty files with go1.8
-   Google Cloud Storage
    -   Fix depth 1 directory listings
-   Yandex
    -   Fix single level directory listing
-   Dropbox
    -   Normalise the case for single level directory listings
    -   Fix depth 1 listing
-   S3
    -   Added ca-central-1 region (Jon Yergatian)


v1.35 - 2017-01-02

-   New Features
    -   moveto and copyto commands for choosing a destination name on
        copy/move
    -   rmdirs command to recursively delete empty directories
    -   Allow repeated –include/–exclude/–filter options
    -   Only show transfer stats on commands which transfer stuff
        -   show stats on any command using the --stats flag
    -   Allow overlapping directories in move when server side dir move
        is supported
    -   Add –stats-unit option - thanks Scott McGillivray
-   Bug Fixes
    -   Fix the config file being overwritten when two rclones are
        running
    -   Make rclone lsd obey the filters properly
    -   Fix compilation on mips
    -   Fix not transferring files that don’t differ in size
    -   Fix panic on nil retry/fatal error
-   Mount
    -   Retry reads on error - should help with reliability a lot
    -   Report the modification times for directories from the remote
    -   Add bandwidth accounting and limiting (fixes –bwlimit)
    -   If –stats provided will show stats and which files are
        transferring
    -   Support R/W files if truncate is set.
    -   Implement statfs interface so df works
    -   Note that write is now supported on Amazon Drive
    -   Report number of blocks in a file - thanks Stefan Breunig
-   Crypt
    -   Prevent the user pointing crypt at itself
    -   Fix failed to authenticate decrypted block errors
        -   these will now return the underlying unexpected EOF instead
-   Amazon Drive
    -   Add support for server side move and directory move - thanks
        Stefan Breunig
    -   Fix nil pointer deref on size attribute
-   B2
    -   Use new prefix and delimiter parameters in directory listings
        -   This makes –max-depth 1 dir listings as used in mount much
            faster
    -   Reauth the account while doing uploads too - should help with
        token expiry
-   Drive
    -   Make DirMove more efficient and complain about moving the root
    -   Create destination directory on Move()


v1.34 - 2016-11-06

-   New Features
    -   Stop single file and --files-from operations iterating through
        the source bucket.
    -   Stop removing failed upload to cloud storage remotes
    -   Make ContentType be preserved for cloud to cloud copies
    -   Add support to toggle bandwidth limits via SIGUSR2 - thanks
        Marco Paganini
    -   rclone check shows count of hashes that couldn’t be checked
    -   rclone listremotes command
    -   Support linux/arm64 build - thanks Fredrik Fornwall
    -   Remove Authorization: lines from --dump-headers output
-   Bug Fixes
    -   Ignore files with control characters in the names
    -   Fix rclone move command
        -   Delete src files which already existed in dst
        -   Fix deletion of src file when dst file older
    -   Fix rclone check on crypted file systems
    -   Make failed uploads not count as “Transferred”
    -   Make sure high level retries show with -q
    -   Use a vendor directory with godep for repeatable builds
-   rclone mount - FUSE
    -   Implement FUSE mount options
        -   --no-modtime, --debug-fuse, --read-only, --allow-non-empty,
            --allow-root, --allow-other
        -   --default-permissions, --write-back-cache, --max-read-ahead,
            --umask, --uid, --gid
    -   Add --dir-cache-time to control caching of directory entries
    -   Implement seek for files opened for read (useful for video
        players)
        -   with -no-seek flag to disable
    -   Fix crash on 32 bit ARM (alignment of 64 bit counter)
    -   …and many more internal fixes and improvements!
-   Crypt
    -   Don’t show encrypted password in configurator to stop confusion
-   Amazon Drive
    -   New wait for upload option --acd-upload-wait-per-gb
        -   upload timeouts scale by file size and can be disabled
    -   Add 502 Bad Gateway to list of errors we retry
    -   Fix overwriting a file with a zero length file
    -   Fix ACD file size warning limit - thanks Felix Bünemann
-   Local
    -   Unix: implement -x/--one-file-system to stay on a single file
        system
        -   thanks Durval Menezes and Luiz Carlos Rumbelsperger Viana
    -   Windows: ignore the symlink bit on files
    -   Windows: Ignore directory based junction points
-   B2
    -   Make sure each upload has at least one upload slot - fixes
        strange upload stats
    -   Fix uploads when using crypt
    -   Fix download of large files (sha1 mismatch)
    -   Return error when we try to create a bucket which someone else
        owns
    -   Update B2 docs with Data usage, and Crypt section - thanks
        Tomasz Mazur
-   S3
    -   Command line and config file support for
        -   Setting/overriding ACL - thanks Radek Senfeld
        -   Setting storage class - thanks Asko Tamm
-   Drive
    -   Make exponential backoff work exactly as per Google
        specification
    -   add .epub, .odp and .tsv as export formats.
-   Swift
    -   Don’t read metadata for directory marker objects


v1.33 - 2016-08-24

-   New Features
    -   Implement encryption
        -   data encrypted in NACL secretbox format
        -   with optional file name encryption
    -   New commands
        -   rclone mount - implements FUSE mounting of remotes
            (EXPERIMENTAL)
            -   works on Linux, FreeBSD and OS X (need testers for the
                last 2!)
        -   rclone cat - outputs remote file or files to the terminal
        -   rclone genautocomplete - command to make a bash completion
            script for rclone
    -   Editing a remote using rclone config now goes through the wizard
    -   Compile with go 1.7 - this fixes rclone on macOS Sierra and on
        386 processors
    -   Use cobra for sub commands and docs generation
-   drive
    -   Document how to make your own client_id
-   s3
    -   User-configurable Amazon S3 ACL (thanks Radek Šenfeld)
-   b2
    -   Fix stats accounting for upload - no more jumping to 100% done
    -   On cleanup delete hide marker if it is the current file
    -   New B2 API endpoint (thanks Per Cederberg)
    -   Set maximum backoff to 5 Minutes
-   onedrive
    -   Fix URL escaping in file names - eg uploading files with + in
        them.
-   amazon cloud drive
    -   Fix token expiry during large uploads
    -   Work around 408 REQUEST_TIMEOUT and 504 GATEWAY_TIMEOUT errors
-   local
    -   Fix filenames with invalid UTF-8 not being uploaded
    -   Fix problem with some UTF-8 characters on OS X


v1.32 - 2016-07-13

-   Backblaze B2
    -   Fix upload of files large files not in root


v1.31 - 2016-07-13

-   New Features
    -   Reduce memory on sync by about 50%
    -   Implement –no-traverse flag to stop copy traversing the
        destination remote.
        -   This can be used to reduce memory usage down to the smallest
            possible.
        -   Useful to copy a small number of files into a large
            destination folder.
    -   Implement cleanup command for emptying trash / removing old
        versions of files
        -   Currently B2 only
    -   Single file handling improved
        -   Now copied with –files-from
        -   Automatically sets –no-traverse when copying a single file
    -   Info on using installing with ansible - thanks Stefan Weichinger
    -   Implement –no-update-modtime flag to stop rclone fixing the
        remote modified times.
-   Bug Fixes
    -   Fix move command - stop it running for overlapping Fses - this
        was causing data loss.
-   Local
    -   Fix incomplete hashes - this was causing problems for B2.
-   Amazon Drive
    -   Rename Amazon Cloud Drive to Amazon Drive - no changes to config
        file needed.
-   Swift
    -   Add support for non-default project domain - thanks Antonio
        Messina.
-   S3
    -   Add instructions on how to use rclone with minio.
    -   Add ap-northeast-2 (Seoul) and ap-south-1 (Mumbai) regions.
    -   Skip setting the modified time for objects > 5GB as it isn’t
        possible.
-   Backblaze B2
    -   Add –b2-versions flag so old versions can be listed and
        retreived.
    -   Treat 403 errors (eg cap exceeded) as fatal.
    -   Implement cleanup command for deleting old file versions.
    -   Make error handling compliant with B2 integrations notes.
    -   Fix handling of token expiry.
    -   Implement –b2-test-mode to set X-Bz-Test-Mode header.
    -   Set cutoff for chunked upload to 200MB as per B2 guidelines.
    -   Make upload multi-threaded.
-   Dropbox
    -   Don’t retry 461 errors.


v1.30 - 2016-06-18

-   New Features
    -   Directory listing code reworked for more features and better
        error reporting (thanks to Klaus Post for help). This enables
        -   Directory include filtering for efficiency
        -   –max-depth parameter
        -   Better error reporting
        -   More to come
    -   Retry more errors
    -   Add –ignore-size flag - for uploading images to onedrive
    -   Log -v output to stdout by default
    -   Display the transfer stats in more human readable form
    -   Make 0 size files specifiable with --max-size 0b
    -   Add b suffix so we can specify bytes in –bwlimit, –min-size etc
    -   Use “password:” instead of “password>” prompt - thanks Klaus
        Post and Leigh Klotz
-   Bug Fixes
    -   Fix retry doing one too many retries
-   Local
    -   Fix problems with OS X and UTF-8 characters
-   Amazon Drive
    -   Check a file exists before uploading to help with 408 Conflict
        errors
    -   Reauth on 401 errors - this has been causing a lot of problems
    -   Work around spurious 403 errors
    -   Restart directory listings on error
-   Google Drive
    -   Check a file exists before uploading to help with duplicates
    -   Fix retry of multipart uploads
-   Backblaze B2
    -   Implement large file uploading
-   S3
    -   Add AES256 server-side encryption for - thanks Justin R. Wilson
-   Google Cloud Storage
    -   Make sure we don’t use conflicting content types on upload
    -   Add service account support - thanks Michal Witkowski
-   Swift
    -   Add auth version parameter
    -   Add domain option for openstack (v3 auth) - thanks Fabian Ruff


v1.29 - 2016-04-18

-   New Features
    -   Implement -I, --ignore-times for unconditional upload
    -   Improve dedupecommand
        -   Now removes identical copies without asking
        -   Now obeys --dry-run
        -   Implement --dedupe-mode for non interactive running
            -   --dedupe-mode interactive - interactive the default.
            -   --dedupe-mode skip - removes identical files then skips
                anything left.
            -   --dedupe-mode first - removes identical files then keeps
                the first one.
            -   --dedupe-mode newest - removes identical files then
                keeps the newest one.
            -   --dedupe-mode oldest - removes identical files then
                keeps the oldest one.
            -   --dedupe-mode rename - removes identical files then
                renames the rest to be different.
-   Bug fixes
    -   Make rclone check obey the --size-only flag.
    -   Use “application/octet-stream” if discovered mime type is
        invalid.
    -   Fix missing “quit” option when there are no remotes.
-   Google Drive
    -   Increase default chunk size to 8 MB - increases upload speed of
        big files
    -   Speed up directory listings and make more reliable
    -   Add missing retries for Move and DirMove - increases reliability
    -   Preserve mime type on file update
-   Backblaze B2
    -   Enable mod time syncing
        -   This means that B2 will now check modification times
        -   It will upload new files to update the modification times
        -   (there isn’t an API to just set the mod time.)
        -   If you want the old behaviour use --size-only.
    -   Update API to new version
    -   Fix parsing of mod time when not in metadata
-   Swift/Hubic
    -   Don’t return an MD5SUM for static large objects
-   S3
    -   Fix uploading files bigger than 50GB


v1.28 - 2016-03-01

-   New Features
    -   Configuration file encryption - thanks Klaus Post
    -   Improve rclone config adding more help and making it easier to
        understand
    -   Implement -u/--update so creation times can be used on all
        remotes
    -   Implement --low-level-retries flag
    -   Optionally disable gzip compression on downloads with
        --no-gzip-encoding
-   Bug fixes
    -   Don’t make directories if --dry-run set
    -   Fix and document the move command
    -   Fix redirecting stderr on unix-like OSes when using --log-file
    -   Fix delete command to wait until all finished - fixes missing
        deletes.
-   Backblaze B2
    -   Use one upload URL per go routine fixes
        more than one upload using auth token
    -   Add pacing, retries and reauthentication - fixes token expiry
        problems
    -   Upload without using a temporary file from local (and remotes
        which support SHA1)
    -   Fix reading metadata for all files when it shouldn’t have been
-   Drive
    -   Fix listing drive documents at root
    -   Disable copy and move for Google docs
-   Swift
    -   Fix uploading of chunked files with non ASCII characters
    -   Allow setting of storage_url in the config - thanks Xavier Lucas
-   S3
    -   Allow IAM role and credentials from environment variables -
        thanks Brian Stengaard
    -   Allow low privilege users to use S3 (check if directory exists
        during Mkdir) - thanks Jakub Gedeon
-   Amazon Drive
    -   Retry on more things to make directory listings more reliable


v1.27 - 2016-01-31

-   New Features
    -   Easier headless configuration with rclone authorize
    -   Add support for multiple hash types - we now check SHA1 as well
        as MD5 hashes.
    -   delete command which does obey the filters (unlike purge)
    -   dedupe command to deduplicate a remote. Useful with Google
        Drive.
    -   Add --ignore-existing flag to skip all files that exist on
        destination.
    -   Add --delete-before, --delete-during, --delete-after flags.
    -   Add --memprofile flag to debug memory use.
    -   Warn the user about files with same name but different case
    -   Make --include rules add their implict exclude * at the end of
        the filter list
    -   Deprecate compiling with go1.3
-   Amazon Drive
    -   Fix download of files > 10 GB
    -   Fix directory traversal (“Next token is expired”) for large
        directory listings
    -   Remove 409 conflict from error codes we will retry - stops very
        long pauses
-   Backblaze B2
    -   SHA1 hashes now checked by rclone core
-   Drive
    -   Add --drive-auth-owner-only to only consider files owned by the
        user - thanks Björn Harrtell
    -   Export Google documents
-   Dropbox
    -   Make file exclusion error controllable with -q
-   Swift
    -   Fix upload from unprivileged user.
-   S3
    -   Fix updating of mod times of files with + in.
-   Local
    -   Add local file system option to disable UNC on Windows.


v1.26 - 2016-01-02

-   New Features
    -   Yandex storage backend - thank you Dmitry Burdeev (“dibu”)
    -   Implement Backblaze B2 storage backend
    -   Add –min-age and –max-age flags - thank you Adriano Aurélio
        Meirelles
    -   Make ls/lsl/md5sum/size/check obey includes and excludes
-   Fixes
    -   Fix crash in http logging
    -   Upload releases to github too
-   Swift
    -   Fix sync for chunked files
-   OneDrive
    -   Re-enable server side copy
    -   Don’t mask HTTP error codes with JSON decode error
-   S3
    -   Fix corrupting Content-Type on mod time update (thanks Joseph
        Spurrier)


v1.25 - 2015-11-14

-   New features
    -   Implement Hubic storage system
-   Fixes
    -   Fix deletion of some excluded files without –delete-excluded
        -   This could have deleted files unexpectedly on sync
        -   Always check first with --dry-run!
-   Swift
    -   Stop SetModTime losing metadata (eg X-Object-Manifest)
        -   This could have caused data loss for files > 5GB in size
    -   Use ContentType from Object to avoid lookups in listings
-   OneDrive
    -   disable server side copy as it seems to be broken at Microsoft


v1.24 - 2015-11-07

-   New features
    -   Add support for Microsoft OneDrive
    -   Add --no-check-certificate option to disable server certificate
        verification
    -   Add async readahead buffer for faster transfer of big files
-   Fixes
    -   Allow spaces in remotes and check remote names for validity at
        creation time
    -   Allow ‘&’ and disallow ‘:’ in Windows filenames.
-   Swift
    -   Ignore directory marker objects where appropriate - allows
        working with Hubic
    -   Don’t delete the container if fs wasn’t at root
-   S3
    -   Don’t delete the bucket if fs wasn’t at root
-   Google Cloud Storage
    -   Don’t delete the bucket if fs wasn’t at root


v1.23 - 2015-10-03

-   New features
    -   Implement rclone size for measuring remotes
-   Fixes
    -   Fix headless config for drive and gcs
    -   Tell the user they should try again if the webserver method
        failed
    -   Improve output of --dump-headers
-   S3
    -   Allow anonymous access to public buckets
-   Swift
    -   Stop chunked operations logging “Failed to read info: Object Not
        Found”
    -   Use Content-Length on uploads for extra reliability


v1.22 - 2015-09-28

-   Implement rsync like include and exclude flags
-   swift
    -   Support files > 5GB - thanks Sergey Tolmachev


v1.21 - 2015-09-22

-   New features
    -   Display individual transfer progress
    -   Make lsl output times in localtime
-   Fixes
    -   Fix allowing user to override credentials again in Drive, GCS
        and ACD
-   Amazon Drive
    -   Implement compliant pacing scheme
-   Google Drive
    -   Make directory reads concurrent for increased speed.


v1.20 - 2015-09-15

-   New features
    -   Amazon Drive support
    -   Oauth support redone - fix many bugs and improve usability
        -   Use “golang.org/x/oauth2” as oauth libary of choice
        -   Improve oauth usability for smoother initial signup
        -   drive, googlecloudstorage: optionally use auto config for
            the oauth token
    -   Implement –dump-headers and –dump-bodies debug flags
    -   Show multiple matched commands if abbreviation too short
    -   Implement server side move where possible
-   local
    -   Always use UNC paths internally on Windows - fixes a lot of bugs
-   dropbox
    -   force use of our custom transport which makes timeouts work
-   Thanks to Klaus Post for lots of help with this release


v1.19 - 2015-08-28

-   New features
    -   Server side copies for s3/swift/drive/dropbox/gcs
    -   Move command - uses server side copies if it can
    -   Implement –retries flag - tries 3 times by default
    -   Build for plan9/amd64 and solaris/amd64 too
-   Fixes
    -   Make a current version download with a fixed URL for scripting
    -   Ignore rmdir in limited fs rather than throwing error
-   dropbox
    -   Increase chunk size to improve upload speeds massively
    -   Issue an error message when trying to upload bad file name


v1.18 - 2015-08-17

-   drive
    -   Add --drive-use-trash flag so rclone trashes instead of deletes
    -   Add “Forbidden to download” message for files with no
        downloadURL
-   dropbox
    -   Remove datastore
        -   This was deprecated and it caused a lot of problems
        -   Modification times and MD5SUMs no longer stored
    -   Fix uploading files > 2GB
-   s3
    -   use official AWS SDK from github.com/aws/aws-sdk-go
    -   NB will most likely require you to delete and recreate remote
    -   enable multipart upload which enables files > 5GB
    -   tested with Ceph / RadosGW / S3 emulation
    -   many thanks to Sam Liston and Brian Haymore at the Utah Center
        for High Performance Computing for a Ceph test account
-   misc
    -   Show errors when reading the config file
    -   Do not print stats in quiet mode - thanks Leonid Shalupov
    -   Add FAQ
    -   Fix created directories not obeying umask
    -   Linux installation instructions - thanks Shimon Doodkin


v1.17 - 2015-06-14

-   dropbox: fix case insensitivity issues - thanks Leonid Shalupov


v1.16 - 2015-06-09

-   Fix uploading big files which was causing timeouts or panics
-   Don’t check md5sum after download with –size-only


v1.15 - 2015-06-06

-   Add –checksum flag to only discard transfers by MD5SUM - thanks Alex
    Couper
-   Implement –size-only flag to sync on size not checksum & modtime
-   Expand docs and remove duplicated information
-   Document rclone’s limitations with directories
-   dropbox: update docs about case insensitivity


v1.14 - 2015-05-21

-   local: fix encoding of non utf-8 file names - fixes a duplicate file
    problem
-   drive: docs about rate limiting
-   google cloud storage: Fix compile after API change in
    “google.golang.org/api/storage/v1”


v1.13 - 2015-05-10

-   Revise documentation (especially sync)
-   Implement –timeout and –conntimeout
-   s3: ignore etags from multipart uploads which aren’t md5sums


v1.12 - 2015-03-15

-   drive: Use chunked upload for files above a certain size
-   drive: add –drive-chunk-size and –drive-upload-cutoff parameters
-   drive: switch to insert from update when a failed copy deletes the
    upload
-   core: Log duplicate files if they are detected


v1.11 - 2015-03-04

-   swift: add region parameter
-   drive: fix crash on failed to update remote mtime
-   In remote paths, change native directory separators to /
-   Add synchronization to ls/lsl/lsd output to stop corruptions
-   Ensure all stats/log messages to go stderr
-   Add –log-file flag to log everything (including panics) to file
-   Make it possible to disable stats printing with –stats=0
-   Implement –bwlimit to limit data transfer bandwidth


v1.10 - 2015-02-12

-   s3: list an unlimited number of items
-   Fix getting stuck in the configurator


v1.09 - 2015-02-07

-   windows: Stop drive letters (eg C:) getting mixed up with remotes
    (eg drive:)
-   local: Fix directory separators on Windows
-   drive: fix rate limit exceeded errors


v1.08 - 2015-02-04

-   drive: fix subdirectory listing to not list entire drive
-   drive: Fix SetModTime
-   dropbox: adapt code to recent library changes


v1.07 - 2014-12-23

-   google cloud storage: fix memory leak


v1.06 - 2014-12-12

-   Fix “Couldn’t find home directory” on OSX
-   swift: Add tenant parameter
-   Use new location of Google API packages


v1.05 - 2014-08-09

-   Improved tests and consequently lots of minor fixes
-   core: Fix race detected by go race detector
-   core: Fixes after running errcheck
-   drive: reset root directory on Rmdir and Purge
-   fs: Document that Purger returns error on empty directory, test and
    fix
-   google cloud storage: fix ListDir on subdirectory
-   google cloud storage: re-read metadata in SetModTime
-   s3: make reading metadata more reliable to work around eventual
    consistency problems
-   s3: strip trailing / from ListDir()
-   swift: return directories without / in ListDir


v1.04 - 2014-07-21

-   google cloud storage: Fix crash on Update


v1.03 - 2014-07-20

-   swift, s3, dropbox: fix updated files being marked as corrupted
-   Make compile with go 1.1 again


v1.02 - 2014-07-19

-   Implement Dropbox remote
-   Implement Google Cloud Storage remote
-   Verify Md5sums and Sizes after copies
-   Remove times from “ls” command - lists sizes only
-   Add add “lsl” - lists times and sizes
-   Add “md5sum” command


v1.01 - 2014-07-04

-   drive: fix transfer of big files using up lots of memory


v1.00 - 2014-07-03

-   drive: fix whole second dates


v0.99 - 2014-06-26

-   Fix –dry-run not working
-   Make compatible with go 1.1


v0.98 - 2014-05-30

-   s3: Treat missing Content-Length as 0 for some ceph installations
-   rclonetest: add file with a space in


v0.97 - 2014-05-05

-   Implement copying of single files
-   s3 & swift: support paths inside containers/buckets


v0.96 - 2014-04-24

-   drive: Fix multiple files of same name being created
-   drive: Use o.Update and fs.Put to optimise transfers
-   Add version number, -V and –version


v0.95 - 2014-03-28

-   rclone.org: website, docs and graphics
-   drive: fix path parsing


v0.94 - 2014-03-27

-   Change remote format one last time
-   GNU style flags


v0.93 - 2014-03-16

-   drive: store token in config file
-   cross compile other versions
-   set strict permissions on config file


v0.92 - 2014-03-15

-   Config fixes and –config option


v0.91 - 2014-03-15

-   Make config file


v0.90 - 2013-06-27

-   Project named rclone


v0.00 - 2012-11-18

-   Project started


Bugs and Limitations

Empty directories are left behind / not created

With remotes that have a concept of directory, eg Local and Drive, empty
directories may be left behind, or not created when one was expected.

This is because rclone doesn’t have a concept of a directory - it only
works on objects. Most of the object storage systems can’t actually
store a directory so there is nowhere for rclone to store anything about
directories.

You can work round this to some extent with thepurge command which will
delete everything under the path, INLUDING empty directories.

This may be fixed at some point in Issue #100

Directory timestamps aren’t preserved

For the same reason as the above, rclone doesn’t have a concept of a
directory - it only works on objects, therefore it can’t preserve the
timestamps of directories.


Frequently Asked Questions

Do all cloud storage systems support all rclone commands

Yes they do. All the rclone commands (eg sync, copy etc) will work on
all the remote storage systems.

Can I copy the config from one machine to another

Sure! Rclone stores all of its config in a single file. If you want to
find this file, run rclone config file which will tell you where it is.

See the remote setup docs for more info.

How do I configure rclone on a remote / headless box with no browser?

This has now been documented in its own remote setup page.

Can rclone sync directly from drive to s3

Rclone can sync between two remote cloud storage systems just fine.

Note that it effectively downloads the file and uploads it again, so the
node running rclone would need to have lots of bandwidth.

The syncs would be incremental (on a file by file basis).

Eg

    rclone sync drive:Folder s3:bucket

Using rclone from multiple locations at the same time

You can use rclone from multiple places at the same time if you choose
different subdirectory for the output, eg

    Server A> rclone sync /tmp/whatever remote:ServerA
    Server B> rclone sync /tmp/whatever remote:ServerB

If you sync to the same directory then you should use rclone copy
otherwise the two rclones may delete each others files, eg

    Server A> rclone copy /tmp/whatever remote:Backup
    Server B> rclone copy /tmp/whatever remote:Backup

The file names you upload from Server A and Server B should be different
in this case, otherwise some file systems (eg Drive) may make
duplicates.

Why doesn’t rclone support partial transfers / binary diffs like rsync?

Rclone stores each file you transfer as a native object on the remote
cloud storage system. This means that you can see the files you upload
as expected using alternative access methods (eg using the Google Drive
web interface). There is a 1:1 mapping between files on your hard disk
and objects created in the cloud storage system.

Cloud storage systems (at least none I’ve come across yet) don’t support
partially uploading an object. You can’t take an existing object, and
change some bytes in the middle of it.

It would be possible to make a sync system which stored binary diffs
instead of whole objects like rclone does, but that would break the 1:1
mapping of files on your hard disk to objects in the remote cloud
storage system.

All the cloud storage systems support partial downloads of content, so
it would be possible to make partial downloads work. However to make
this work efficiently this would require storing a significant amount of
metadata, which breaks the desired 1:1 mapping of files to objects.

Can rclone do bi-directional sync?

No, not at present. rclone only does uni-directional sync from A -> B.
It may do in the future though since it has all the primitives - it just
requires writing the algorithm to do it.

Can I use rclone with an HTTP proxy?

Yes. rclone will follow the standard environment variables for proxies,
similar to cURL and other programs.

In general the variables are called http_proxy (for services reached
over http) and https_proxy (for services reached over https). Most
public services will be using https, but you may wish to set both.

The content of the variable is protocol://server:port. The protocol
value is the one used to talk to the proxy server, itself, and is
commonly either http or socks5.

Slightly annoyingly, there is no _standard_ for the name; some
applications may use http_proxy but another one HTTP_PROXY. The Go
libraries used by rclone will try both variations, but you may wish to
set all possibilities. So, on Linux, you may end up with code similar to

    export http_proxy=http://proxyserver:12345
    export https_proxy=$http_proxy
    export HTTP_PROXY=$http_proxy
    export HTTPS_PROXY=$http_proxy

The NO_PROXY allows you to disable the proxy for specific hosts. Hosts
must be comma separated, and can contain domains or parts. For instance
“foo.com” also matches “bar.foo.com”.

e.g.

    export no_proxy=localhost,127.0.0.0/8,my.host.name
    export NO_PROXY=$no_proxy

Note that the ftp backend does not support ftp_proxy yet.

Rclone gives x509: failed to load system roots and no roots provided error

This means that rclone can’t file the SSL root certificates. Likely you
are running rclone on a NAS with a cut-down Linux OS, or possibly on
Solaris.

Rclone (via the Go runtime) tries to load the root certificates from
these places on Linux.

    "/etc/ssl/certs/ca-certificates.crt", // Debian/Ubuntu/Gentoo etc.
    "/etc/pki/tls/certs/ca-bundle.crt",   // Fedora/RHEL
    "/etc/ssl/ca-bundle.pem",             // OpenSUSE
    "/etc/pki/tls/cacert.pem",            // OpenELEC

So doing something like this should fix the problem. It also sets the
time which is important for SSL to work properly.

    mkdir -p /etc/ssl/certs/
    curl -o /etc/ssl/certs/ca-certificates.crt https://raw.githubusercontent.com/bagder/ca-bundle/master/ca-bundle.crt
    ntpclient -s -h pool.ntp.org

The two environment variables SSL_CERT_FILE and SSL_CERT_DIR, mentioned
in the x509 pacakge, provide an additional way to provide the SSL root
certificates.

Note that you may need to add the --insecure option to the curl command
line if it doesn’t work without.

    curl --insecure -o /etc/ssl/certs/ca-certificates.crt https://raw.githubusercontent.com/bagder/ca-bundle/master/ca-bundle.crt

Rclone gives Failed to load config file: function not implemented error

Likely this means that you are running rclone on Linux version not
supported by the go runtime, ie earlier than version 2.6.23.

See the system requirements section in the go install docs for full
details.

All my uploaded docx/xlsx/pptx files appear as archive/zip

This is caused by uploading these files from a Windows computer which
hasn’t got the Microsoft Office suite installed. The easiest way to fix
is to install the Word viewer and the Microsoft Office Compatibility
Pack for Word, Excel, and PowerPoint 2007 and later versions’ file
formats

tcp lookup some.domain.com no such host

This happens when rclone cannot resolve a domain. Please check that your
DNS setup is generally working, e.g.

    # both should print a long list of possible IP addresses
    dig www.googleapis.com          # resolve using your default DNS
    dig www.googleapis.com @8.8.8.8 # resolve with Google's DNS server

If you are using systemd-resolved (default on Arch Linux), ensure it is
at version 233 or higher. Previous releases contain a bug which causes
not all domains to be resolved properly.

Additionally with the GODEBUG=netdns= environment variable the Go
resolver decision can be influenced. This also allows to resolve certain
issues with DNS resolution. See the name resolution section in the go
docs.

The total size reported in the stats for a sync is wrong and keeps changing

It is likely you have more than 10,000 files that need to be synced. By
default rclone only gets 10,000 files ahead in a sync so as not to use
up too much memory. You can change this default with the –max-backlog
flag.

Rclone is using too much memory or appears to have a memory leak

Rclone is written in Go which uses a garbage collector. The default
settings for the garbage collector mean that it runs when the heap size
has doubled.

However it is possible to tune the garbage collector to use less memory
by setting GOGC to a lower value, say export GOGC=20. This will make the
garbage collector work harder, reducing memory size at the expense of
CPU usage.

The most common cause of rclone using lots of memory is a single
directory with thousands or millions of files in. Rclone has to load
this entirely into memory as rclone objects. Each Rclone object takes
0.5k-1k of memory.


License

This is free software under the terms of MIT the license (check the
COPYING file included with the source code).

    Copyright (C) 2012 by Nick Craig-Wood https://www.craig-wood.com/nick/

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


Authors

-   Nick Craig-Wood nick@craig-wood.com


Contributors

-   Alex Couper amcouper@gmail.com
-   Leonid Shalupov leonid@shalupov.com shalupov@diverse.org.ru
-   Shimon Doodkin helpmepro1@gmail.com
-   Colin Nicholson colin@colinn.com
-   Klaus Post klauspost@gmail.com
-   Sergey Tolmachev tolsi.ru@gmail.com
-   Adriano Aurélio Meirelles adriano@atinge.com
-   C. Bess cbess@users.noreply.github.com
-   Dmitry Burdeev dibu28@gmail.com
-   Joseph Spurrier github@josephspurrier.com
-   Björn Harrtell bjorn@wololo.org
-   Xavier Lucas xavier.lucas@corp.ovh.com
-   Werner Beroux werner@beroux.com
-   Brian Stengaard brian@stengaard.eu
-   Jakub Gedeon jgedeon@sofi.com
-   Jim Tittsler jwt@onjapan.net
-   Michal Witkowski michal@improbable.io
-   Fabian Ruff fabian.ruff@sap.com
-   Leigh Klotz klotz@quixey.com
-   Romain Lapray lapray.romain@gmail.com
-   Justin R. Wilson jrw972@gmail.com
-   Antonio Messina antonio.s.messina@gmail.com
-   Stefan G. Weichinger office@oops.co.at
-   Per Cederberg cederberg@gmail.com
-   Radek Šenfeld rush@logic.cz
-   Fredrik Fornwall fredrik@fornwall.net
-   Asko Tamm asko@deekit.net
-   xor-zz xor@gstocco.com
-   Tomasz Mazur tmazur90@gmail.com
-   Marco Paganini paganini@paganini.net
-   Felix Bünemann buenemann@louis.info
-   Durval Menezes jmrclone@durval.com
-   Luiz Carlos Rumbelsperger Viana maxd13_luiz_carlos@hotmail.com
-   Stefan Breunig stefan-github@yrden.de
-   Alishan Ladhani ali-l@users.noreply.github.com
-   0xJAKE 0xJAKE@users.noreply.github.com
-   Thibault Molleman thibaultmol@users.noreply.github.com
-   Scott McGillivray scott.mcgillivray@gmail.com
-   Bjørn Erik Pedersen bjorn.erik.pedersen@gmail.com
-   Lukas Loesche lukas@mesosphere.io
-   emyarod allllaboutyou@gmail.com
-   T.C. Ferguson tcf909@gmail.com
-   Brandur brandur@mutelight.org
-   Dario Giovannetti dev@dariogiovannetti.net
-   Károly Oláh okaresz@aol.com
-   Jon Yergatian jon@macfanatic.ca
-   Jack Schmidt github@mowsey.org
-   Dedsec1 Dedsec1@users.noreply.github.com
-   Hisham Zarka hzarka@gmail.com
-   Jérôme Vizcaino jerome.vizcaino@gmail.com
-   Mike Tesch mjt6129@rit.edu
-   Marvin Watson marvwatson@users.noreply.github.com
-   Danny Tsai danny8376@gmail.com
-   Yoni Jah yonjah+git@gmail.com yonjah+github@gmail.com
-   Stephen Harris github@spuddy.org sweharris@users.noreply.github.com
-   Ihor Dvoretskyi ihor.dvoretskyi@gmail.com
-   Jon Craton jncraton@gmail.com
-   Hraban Luyat hraban@0brg.net
-   Michael Ledin mledin89@gmail.com
-   Martin Kristensen me@azgul.com
-   Too Much IO toomuchio@users.noreply.github.com
-   Anisse Astier anisse@astier.eu
-   Zahiar Ahmed zahiar@live.com
-   Igor Kharin igorkharin@gmail.com
-   Bill Zissimopoulos billziss@navimatics.com
-   Bob Potter bobby.potter@gmail.com
-   Steven Lu tacticalazn@gmail.com
-   Sjur Fredriksen sjurtf@ifi.uio.no
-   Ruwbin hubus12345@gmail.com
-   Fabian Möller fabianm88@gmail.com f.moeller@nynex.de
-   Edward Q. Bridges github@eqbridges.com
-   Vasiliy Tolstov v.tolstov@selfip.ru
-   Harshavardhana harsha@minio.io
-   sainaen sainaen@gmail.com
-   gdm85 gdm85@users.noreply.github.com
-   Yaroslav Halchenko debian@onerussian.com
-   John Papandriopoulos jpap@users.noreply.github.com
-   Zhiming Wang zmwangx@gmail.com
-   Andy Pilate cubox@cubox.me
-   Oliver Heyme olihey@googlemail.com olihey@users.noreply.github.com
    de8olihe@lego.com
-   wuyu wuyu@yunify.com
-   Andrei Dragomir adragomi@adobe.com
-   Christian Brüggemann mail@cbruegg.com
-   Alex McGrath Kraak amkdude@gmail.com
-   bpicode bjoern.pirnay@googlemail.com
-   Daniel Jagszent daniel@jagszent.de
-   Josiah White thegenius2009@gmail.com
-   Ishuah Kariuki kariuki@ishuah.com ishuah91@gmail.com
-   Jan Varho jan@varho.org
-   Girish Ramakrishnan girish@cloudron.io
-   LingMan LingMan@users.noreply.github.com
-   Jacob McNamee jacobmcnamee@gmail.com
-   jersou jertux@gmail.com
-   thierry thierry@substantiel.fr
-   Simon Leinen simon.leinen@gmail.com ubuntu@s3-test.novalocal
-   Dan Dascalescu ddascalescu+github@gmail.com
-   Jason Rose jason@jro.io
-   Andrew Starr-Bochicchio a.starr.b@gmail.com
-   John Leach john@johnleach.co.uk
-   Corban Raun craun@instructure.com
-   Pierre Carlson mpcarl@us.ibm.com
-   Ernest Borowski er.borowski@gmail.com
-   Remus Bunduc remus.bunduc@gmail.com
-   Iakov Davydov iakov.davydov@unil.ch dav05.gith@myths.ru
-   Jakub Tasiemski tasiemski@gmail.com
-   David Minor dminor@saymedia.com
-   Tim Cooijmans cooijmans.tim@gmail.com
-   Laurence liuxy6@gmail.com
-   Giovanni Pizzi gio.piz@gmail.com
-   Filip Bartodziej filipbartodziej@gmail.com
-   Jon Fautley jon@dead.li
-   lewapm 32110057+lewapm@users.noreply.github.com
-   Yassine Imounachen yassine256@gmail.com
-   Chris Redekop chris-redekop@users.noreply.github.com
    chris.redekop@gmail.com
-   Jon Fautley jon@adenoid.appstal.co.uk
-   Will Gunn WillGunn@users.noreply.github.com
-   Lucas Bremgartner lucas@bremis.ch
-   Jody Frankowski jody.frankowski@gmail.com
-   Andreas Roussos arouss1980@gmail.com
-   nbuchanan nbuchanan@utah.gov
-   Durval Menezes rclone@durval.com
-   Victor vb-github@viblo.se
-   Mateusz pabian.mateusz@gmail.com
-   Daniel Loader spicypixel@gmail.com
-   David0rk davidork@gmail.com
-   Alexander Neumann alexander@bumpern.de
-   Giri Badanahatti gbadanahatti@us.ibm.com@Giris-MacBook-Pro.local
-   Leo R. Lundgren leo@finalresort.org
-   wolfv wolfv6@users.noreply.github.com
-   Dave Pedu dave@davepedu.com
-   Stefan Lindblom lindblom@spotify.com
-   seuffert oliver@seuffert.biz
-   gbadanahatti 37121690+gbadanahatti@users.noreply.github.com
-   Keith Goldfarb barkofdelight@gmail.com
-   Steve Kriss steve@heptio.com
-   Chih-Hsuan Yen yan12125@gmail.com
-   Alexander Neumann fd0@users.noreply.github.com
-   Matt Holt mholt@users.noreply.github.com
-   Eri Bastos bastos.eri@gmail.com
-   Michael P. Dubner pywebmail@list.ru
-   Antoine GIRARD sapk@users.noreply.github.com
-   Mateusz Piotrowski mpp302@gmail.com
-   Animosity022 animosity22@users.noreply.github.com
    earl.texter@gmail.com
-   Peter Baumgartner pete@lincolnloop.com
-   Craig Rachel craig@craigrachel.com
-   Michael G. Noll miguno@users.noreply.github.com
-   hensur me@hensur.de
-   Oliver Heyme de8olihe@lego.com
-   Richard Yang richard@yenforyang.com
-   Piotr Oleszczyk piotr.oleszczyk@gmail.com
-   Rodrigo rodarima@gmail.com
-   NoLooseEnds NoLooseEnds@users.noreply.github.com
-   Jakub Karlicek jakub@karlicek.me
-   John Clayton john@codemonkeylabs.com
-   Kasper Byrdal Nielsen byrdal76@gmail.com
-   Benjamin Joseph Dag bjdag1234@users.noreply.github.com
-   themylogin themylogin@gmail.com
-   Onno Zweers onno.zweers@surfsara.nl
-   Jasper Lievisse Adriaanse jasper@humppa.nl
-   sandeepkru sandeep.ummadi@gmail.com
    sandeepkru@users.noreply.github.com
-   HerrH atomtigerzoo@users.noreply.github.com
-   Andrew 4030760+sparkyman215@users.noreply.github.com
-   dan smith XX1011@gmail.com
-   Oleg Kovalov iamolegkovalov@gmail.com
-   Ruben Vandamme github-com-00ff86@vandamme.email
-   Cnly minecnly@gmail.com
-   Andres Alvarez 1671935+kir4h@users.noreply.github.com
-   reddi1 xreddi@gmail.com
-   Matt Tucker matthewtckr@gmail.com
-   Sebastian Bünger buengese@gmail.com
-   Martin Polden mpolden@mpolden.no
-   Alex Chen Cnly@users.noreply.github.com
-   Denis deniskovpen@gmail.com
-   bsteiss 35940619+bsteiss@users.noreply.github.com
-   Cédric Connes cedric.connes@gmail.com
-   Dr. Tobias Quathamer toddy15@users.noreply.github.com
-   dcpu 42736967+dcpu@users.noreply.github.com
-   Sheldon Rupp me@shel.io
-   albertony 12441419+albertony@users.noreply.github.com
-   cron410 cron410@gmail.com
-   Anagh Kumar Baranwal anaghk.dos@gmail.com
-   Felix Brucker felix@felixbrucker.com
-   Santiago Rodríguez scollazo@users.noreply.github.com
-   Craig Miskell craig.miskell@fluxfederation.com
-   Antoine GIRARD sapk@sapk.fr
-   Joanna Marek joanna.marek@u2i.com
-   frenos frenos@users.noreply.github.com
-   ssaqua ssaqua@users.noreply.github.com
-   xnaas me@xnaas.info
-   Frantisek Fuka fuka@fuxoft.cz
-   Paul Kohout pauljkohout@yahoo.com
-   dcpu 43330287+dcpu@users.noreply.github.com
-   jackyzy823 jackyzy823@gmail.com
-   David Haguenauer ml@kurokatta.org
-   teresy hi.teresy@gmail.com
-   buergi patbuergi@gmx.de
-   Florian Gamboeck mail@floga.de
-   Ralf Hemberger 10364191+rhemberger@users.noreply.github.com
-   Scott Edlund sedlund@users.noreply.github.com
-   Erik Swanson erik@retailnext.net
-   Jake Coggiano jake@stripe.com
-   brused27 brused27@noemailaddress
-   Peter Kaminski kaminski@istori.com
-   Henry Ptasinski henry@logout.com
-   Alexander kharkovalexander@gmail.com
-   Garry McNulty garrmcnu@gmail.com
-   Mathieu Carbou mathieu.carbou@gmail.com
-   Mark Otway mark@otway.com
-   William Cocker 37018962+WilliamCocker@users.noreply.github.com
-   François Leurent 131.js@cloudyks.org
-   Arkadius Stefanski arkste@gmail.com
-   Jay dev@jaygoel.com
-   andrea rota a@xelera.eu
-   nicolov nicolov@users.noreply.github.com
-   Dario Guzik dario@guzik.com.ar
-   qip qip@users.noreply.github.com
-   yair@unicorn yair@unicorn
-   Matt Robinson brimstone@the.narro.ws
-   kayrus kay.diam@gmail.com
-   Rémy Léone remy.leone@gmail.com
-   Wojciech Smigielski wojciech.hieronim.smigielski@gmail.com
-   weetmuts oehrstroem@gmail.com
-   Jonathan vanillajonathan@users.noreply.github.com
-   James Carpenter orbsmiv@users.noreply.github.com
-   Vince vince0villamora@gmail.com
-   Nestar47 47841759+Nestar47@users.noreply.github.com
-   Six brbsix@gmail.com
-   Alexandru Bumbacea alexandru.bumbacea@booking.com
-   calisro robert.calistri@gmail.com
-   Dr.Rx david.rey@nventive.com
-   marcintustin marcintustin@users.noreply.github.com
-   jaKa Močnik jaka@koofr.net
-   Fionera fionera@fionera.de
-   Dan Walters dan@walters.io
-   Danil Semelenov sgtpep@users.noreply.github.com
-   xopez 28950736+xopez@users.noreply.github.com
-   Ben Boeckel mathstuf@gmail.com
-   Manu manu@snapdragon.cc
-   Kyle E. Mitchell kyle@kemitchell.com
-   Gary Kim gary@garykim.dev
-   Jon jonathn@github.com
-   Jeff Quinn jeffrey.quinn@bluevoyant.com
-   Peter Berbec peter@berbec.com
-   didil 1284255+didil@users.noreply.github.com
-   id01 gaviniboom@gmail.com
-   Robert Marko robimarko@gmail.com
-   Philip Harvey 32467456+pharveybattelle@users.noreply.github.com
-   JorisE JorisE@users.noreply.github.com
-   garry415 garry.415@gmail.com
-   forgems forgems@gmail.com
-   Florian Apolloner florian@apolloner.eu
-   Aleksandar Jankovic office@ajankovic.com



CONTACT THE RCLONE PROJECT


Forum

Forum for questions and general discussion:

-   https://forum.rclone.org


Gitub project

The project website is at:

-   https://github.com/ncw/rclone

There you can file bug reports or contribute pull requests.


Twitter

You can also follow me on twitter for rclone announcements:

-   [@njcw](https://twitter.com/njcw)


Email

Or if all else fails or you want to ask something private or
confidential email Nick Craig-Wood
