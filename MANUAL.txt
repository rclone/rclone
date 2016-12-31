rclone(1) User Manual
Nick Craig-Wood
Jan 02, 2017



RCLONE


[Logo]

Rclone is a command line program to sync files and directories to and
from

-   Google Drive
-   Amazon S3
-   Openstack Swift / Rackspace cloud files / Memset Memstore
-   Dropbox
-   Google Cloud Storage
-   Amazon Drive
-   Microsoft One Drive
-   Hubic
-   Backblaze B2
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
-   Optional encryption (Crypt)
-   Optional FUSE mount (rclone mount)

Links

-   Home page
-   Github project page for source and bug tracker
-   Rclone Forum
-   Google+ page
-   Downloads



INSTALL


Rclone is a Go program and comes as a single binary file.


Quickstart

-   Download the relevant binary.
-   Unpack and the rclone binary.
-   Run rclone config to setup. See rclone config docs for more details.

See below for some expanded Linux / macOS instructions.

See the Usage section of the docs for how to use rclone, or run
rclone -h.


Linux installation from precompiled binary

Fetch and unpack

    curl -O http://downloads.rclone.org/rclone-current-linux-amd64.zip
    unzip rclone-current-linux-amd64.zip
    cd rclone-*-linux-amd64

Copy binary file

    sudo cp rclone /usr/sbin/
    sudo chown root:root /usr/sbin/rclone
    sudo chmod 755 /usr/sbin/rclone

Install manpage

    sudo mkdir -p /usr/local/share/man/man1
    sudo cp rclone.1 /usr/local/share/man/man1/
    sudo mandb 

Run rclone config to setup. See rclone config docs for more details.

    rclone config


macOS installation from precompiled binary

Download the latest version of rclone.

    cd && curl -O http://downloads.rclone.org/rclone-current-osx-amd64.zip

Unzip the download and cd to the extracted folder.

    unzip -a rclone-current-osx-amd64.zip && cd rclone-*-osx-amd64

Move rclone to your $PATH. You will be prompted for your password.

    sudo mv rclone /usr/local/bin/

Remove the leftover files.

    cd .. && rm -rf rclone-*-osx-amd64 rclone-current-osx-amd64.zip

Run rclone config to setup. See rclone config docs for more details.

    rclone config


Install from source

Make sure you have at least Go 1.5 installed. Make sure your GOPATH is
set, then:

    go get -u -v github.com/ncw/rclone

and this will build the binary in $GOPATH/bin. If you have built rclone
before then you will want to update its dependencies first with this

    go get -u -v github.com/ncw/rclone/...


Installation with Ansible

This can be done with Stefan Weichinger's ansible role.

Instructions

1.  git clone https://github.com/stefangweichinger/ansible-rclone.git
    into your local roles-directory
2.  add the role to the hosts you want rclone installed to:

        - hosts: rclone-hosts
          roles:
              - rclone


Configure

First you'll need to configure rclone. As the object storage systems
have quite complicated authentication these are kept in a config file
.rclone.conf in your home directory by default. (You can use the
--config option to choose a different config file.)

The easiest way to make the config is to run rclone with the config
option:

    rclone config

See the following for detailed instructions for

-   Google drive
-   Amazon S3
-   Swift / Rackspace Cloudfiles / Memset Memstore
-   Dropbox
-   Google Cloud Storage
-   Local filesystem
-   Amazon Drive
-   Backblaze B2
-   Hubic
-   Microsoft One Drive
-   Yandex Disk
-   Crypt - to encrypt other remotes


Usage

Rclone syncs a directory tree from one storage system to another.

Its syntax is like this

    Syntax: [options] subcommand <parameters> <parameters...>

Source and destination paths are specified by the name you gave the
storage system in the config file then the sub path, eg "drive:myfolder"
to look at "myfolder" in Google drive.

You can define as many storage paths as you like in the config file.


Subcommands

rclone uses a system of subcommands. For example

    rclone ls remote:path # lists a re
    rclone copy /local/path remote:path # copies /local/path to the remote
    rclone sync /local/path remote:path # syncs /local/path to the remote


rclone config

Enter an interactive configuration session.

Synopsis

Enter an interactive configuration session.

    rclone config


rclone copy

Copy files from source to dest, skipping already copied

Synopsis

Copy the source to the destination. Doesn't transfer unchanged files,
testing by size and modification time or MD5SUM. Doesn't delete files
from the destination.

Note that it is always the contents of the directory that is synced, not
the directory so when source:path is a directory, it's the contents of
source:path that are copied, not the directory name and contents.

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

If you are familiar with rsync, rclone always works as if you had
written a trailing / - meaning "copy the contents of this directory".
This applies to all commands and whether you are talking about the
source or destination.

See the --no-traverse option for controlling whether rclone lists the
destination directory or not.

    rclone copy source:path dest:path


rclone sync

Make source and dest identical, modifying destination only.

Synopsis

Sync the source to the destination, changing the destination only.
Doesn't transfer unchanged files, testing by size and modification time
or MD5SUM. Destination is updated to match source, including deleting
files if necessary.

IMPORTANT: Since this can cause data loss, test first with the --dry-run
flag to see exactly what would be copied and deleted.

Note that files in the destination won't be deleted if there were any
errors at any point.

It is always the contents of the directory that is synced, not the
directory so when source:path is a directory, it's the contents of
source:path that are copied, not the directory name and contents. See
extended explanation in the copy command above if unsure.

If dest:path doesn't exist, it is created and the source:path contents
go there.

    rclone sync source:path dest:path


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

IMPORTANT: Since this can cause data loss, test first with the --dry-run
flag.

    rclone move source:path dest:path


rclone delete

Remove the contents of path.

Synopsis

Remove the contents of path. Unlike purge it obeys include/exclude
filters so can be used to selectively delete files.

Eg delete all files bigger than 100MBytes

Check what would be deleted first (use either)

    rclone --min-size 100M lsl remote:path
    rclone --dry-run --min-size 100M delete remote:path

Then delete

    rclone --min-size 100M delete remote:path

That reads "delete everything with a minimum size of 100 MB", hence
delete all files bigger than 100MBytes.

    rclone delete remote:path


rclone purge

Remove the path and all of its contents.

Synopsis

Remove the path and all of its contents. Note that this does not obey
include/exclude filters - everything will be removed. Use delete if you
want to selectively delete files.

    rclone purge remote:path


rclone mkdir

Make the path if it doesn't already exist.

Synopsis

Make the path if it doesn't already exist.

    rclone mkdir remote:path


rclone rmdir

Remove the path if empty.

Synopsis

Remove the path. Note that you can't remove a path with objects in it,
use purge for that.

    rclone rmdir remote:path


rclone check

Checks the files in the source and destination match.

Synopsis

Checks the files in the source and destination match. It compares sizes
and MD5SUMs and prints a report of files which don't match. It doesn't
alter the source or destination.

--size-only may be used to only compare the sizes, not the MD5SUMs.

    rclone check source:path dest:path


rclone ls

List all the objects in the path with size and path.

Synopsis

List all the objects in the path with size and path.

    rclone ls remote:path


rclone lsd

List all directories/containers/buckets in the path.

Synopsis

List all directories/containers/buckets in the path.

    rclone lsd remote:path


rclone lsl

List all the objects path with modification time, size and path.

Synopsis

List all the objects path with modification time, size and path.

    rclone lsl remote:path


rclone md5sum

Produces an md5sum file for all the objects in the path.

Synopsis

Produces an md5sum file for all the objects in the path. This is in the
same format as the standard md5sum tool produces.

    rclone md5sum remote:path


rclone sha1sum

Produces an sha1sum file for all the objects in the path.

Synopsis

Produces an sha1sum file for all the objects in the path. This is in the
same format as the standard sha1sum tool produces.

    rclone sha1sum remote:path


rclone size

Prints the total size and number of objects in remote:path.

Synopsis

Prints the total size and number of objects in remote:path.

    rclone size remote:path


rclone version

Show the version number.

Synopsis

Show the version number.

    rclone version


rclone cleanup

Clean up the remote if possible

Synopsis

Clean up the remote if possible. Empty the trash or delete old file
versions. Not supported by all remotes.

    rclone cleanup remote:path


rclone dedupe

Interactively find duplicate files delete/rename them.

Synopsis

By default dedup interactively finds duplicate files and offers to
delete all but one or rename them to be different. Only useful with
Google Drive which can have duplicate file names.

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
-   --dedupe-mode skip - removes identical files then skips
    anything left.
-   --dedupe-mode first - removes identical files then keeps the
    first one.
-   --dedupe-mode newest - removes identical files then keeps the
    newest one.
-   --dedupe-mode oldest - removes identical files then keeps the
    oldest one.
-   --dedupe-mode rename - removes identical files then renames the rest
    to be different.

For example to rename all the identically named photos in your Google
Photos directory, do

    rclone dedupe --dedupe-mode rename "drive:Google Photos"

Or

    rclone dedupe rename "drive:Google Photos"

    rclone dedupe [mode] remote:path

Options

          --dedupe-mode string   Dedupe mode interactive|skip|first|newest|oldest|rename. (default "interactive")


rclone authorize

Remote authorization.

Synopsis

Remote authorization. Used to authorize a remote or headless rclone from
a machine with a browser - use as instructed by rclone config.

    rclone authorize


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

    rclone cat remote:path


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

This doesn't transfer unchanged files, testing by size and modification
time or MD5SUM. It doesn't delete files from the destination.

    rclone copyto source:path dest:path


rclone genautocomplete

Output bash completion script for rclone.

Synopsis

Generates a bash shell autocompletion script for rclone.

This writes to /etc/bash_completion.d/rclone by default so will probably
need to be run with sudo or as root, eg

    sudo rclone genautocomplete

Logout and login again to use the autocompletion scripts, or source them
directly

    . /etc/bash_completion

If you supply a command line argument the script will be written there.

    rclone genautocomplete [output_file]


rclone gendocs

Output markdown docs for rclone to the directory supplied.

Synopsis

This produces markdown docs for the rclone commands to the directory
supplied. These are in a format suitable for hugo to render into the
rclone.org website.

    rclone gendocs output_directory


rclone listremotes

List all the remotes in the config file.

Synopsis

rclone listremotes lists all the available remotes from the config file.

When uses with the -l flag it lists the types too.

    rclone listremotes

Options

      -l, --long   Show the type as well as names.


rclone mount

Mount the remote as a mountpoint. EXPERIMENTAL

Synopsis

rclone mount allows Linux, FreeBSD and macOS to mount any of Rclone's
cloud storage systems as a file system with FUSE.

This is EXPERIMENTAL - use with care.

First set up your remote using rclone config. Check it works with
rclone ls etc.

Start the mount like this

    rclone mount remote:path/to/files /path/to/local/mount &

Stop the mount with

    fusermount -u /path/to/local/mount

Or with OS X

    umount -u /path/to/local/mount

Limitations

This can only write files seqentially, it can only seek when reading.

Rclone mount inherits rclone's directory handling. In rclone's world
directories don't really exist. This means that empty directories will
have a tendency to disappear once they fall out of the directory cache.

The bucket based FSes (eg swift, s3, google compute storage, b2) won't
work from the root - you will need to specify a bucket, or a path within
the bucket. So swift: won't work whereas swift:bucket will as will
swift:bucket/path.

Only supported on Linux, FreeBSD and OS X at the moment.

rclone mount vs rclone sync/copy

File systems expect things to be 100% reliable, whereas cloud storage
systems are a long way from 100% reliable. The rclone sync/copy commands
cope with this with lots of retries. However rclone mount can't use
retries in the same way without making local copies of the uploads. This
might happen in the future, but for the moment rclone mount won't do
that, so will be less reliable than the rclone command.

Bugs

-   All the remotes should work for read, but some may not for write
    -   those which need to know the size in advance won't - eg B2
    -   maybe should pass in size as -1 to mean work it out
    -   Or put in an an upload cache to cache the files on disk first

TODO

-   Check hashes on upload/download
-   Preserve timestamps
-   Move directories

    rclone mount remote:path /path/to/mountpoint

Options

          --allow-non-empty           Allow mounting over a non-empty directory.
          --allow-other               Allow access to other users.
          --allow-root                Allow access to root user.
          --debug-fuse                Debug the FUSE internals - needs -v.
          --default-permissions       Makes kernel enforce access control based on the file mode.
          --dir-cache-time duration   Time to cache directory entries for. (default 5m0s)
          --gid uint32                Override the gid field set by the filesystem. (default 502)
          --max-read-ahead int        The number of bytes that can be prefetched for sequential reads. (default 128k)
          --no-modtime                Don't read the modification time (can speed things up).
          --no-seek                   Don't allow seeking in files.
          --read-only                 Mount read-only.
          --uid uint32                Override the uid field set by the filesystem. (default 502)
          --umask int                 Override the permission bits set by the filesystem. (default 2)
          --write-back-cache          Makes kernel buffer writes before sending them to rclone. Without this, writethrough caching is used.


rclone moveto

Move file or directory from source to dest.

Synopsis

If source:path is a file or directory then it moves it to a file or
directory named dest:path.

This can be used to rename files or upload single files to other than
their existing name. If the source is a directory then it acts exacty
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

This doesn't transfer unchanged files, testing by size and modification
time or MD5SUM. src will be deleted on successful transfer.

IMPORTANT: Since this can cause data loss, test first with the --dry-run
flag.

    rclone moveto source:path dest:path


rclone rmdirs

Remove any empty directoryies under the path.

Synopsis

This removes any empty directories (or directories that only contain
empty directories) under the path that it finds, including the path if
it has nothing in.

This is useful for tidying up remotes that rclone has left a lot of
empty directories in.

    rclone rmdirs remote:path


Copying single files

rclone normally syncs or copies directories. However if the source
remote points to a file, rclone will just copy that file. The
destination remote must point to a directory - rclone will give the
error
Failed to create file system for "remote:file": is a file not a directory
if it isn't.

For example, suppose you have a remote with a file in called test.jpg,
then you could copy just that file like this

    rclone copy remote:test.jpg /tmp/download

The file test.jpg will be placed inside /tmp/download.

This is equivalent to specifying

    rclone copy --no-traverse --files-from /tmp/files remote: /tmp/download

Where /tmp/files contains the single line

    test.jpg

It is recommended to use copy when copying single files not sync. They
have pretty much the same effect but copy will use a lot less memory.


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
full details you'll have to consult the manual page for your shell.

Windows

If your names have spaces in you need to put them in ", eg

    rclone copy "E:\folder name\folder name\folder name" remote:backup

If you are using the root directory on its own then don't quote it (see
#464 for why), eg

    rclone copy E:\ remote:backup


Server Side Copy

Drive, S3, Dropbox, Swift and Google Cloud Storage support server side
copy.

This means if you want to copy one folder to another then rclone won't
download all the files and re-upload them; it will instruct the server
to copy them in place.

Eg

    rclone copy s3:oldbucket s3:newbucket

Will copy the contents of oldbucket to newbucket without downloading and
re-uploading.

Remotes which don't support server side copy (eg local) WILL download
and re-upload in this case.

Server side copies are used with sync and copy and will be identified in
the log when using the -v flag.

Server side copies will only be attempted if the remote names are the
same.

This can be used when scripting to make aged backups efficiently, eg

    rclone sync remote:current-backup remote:previous-backup
    rclone sync /path/to/files remote:current-backup


Options

Rclone has a number of options to control its behaviour.

Options which use TIME use the go time parser. A duration string is a
possibly signed sequence of decimal numbers, each with optional fraction
and a unit suffix, such as "300ms", "-1.5h" or "2h45m". Valid time units
are "ns", "us" (or "µs"), "ms", "s", "m", "h".

Options which use SIZE use kByte by default. However a suffix of b for
bytes, k for kBytes, M for MBytes and G for GBytes may be used. These
are the binary units, eg 1, 2**10, 2**20, 2**30 respectively.

--bwlimit=SIZE

Bandwidth limit in kBytes/s, or use suffix b|k|M|G. The default is 0
which means to not limit bandwidth.

For example to limit bandwidth usage to 10 MBytes/s use --bwlimit 10M

This only limits the bandwidth of the data transfer, it doesn't limit
the bandwith of the directory listings etc.

Note that the units are Bytes/s not Bits/s. Typically connections are
measured in Bits/s - to convert divide by 8. For example let's say you
have a 10 Mbit/s connection and you wish rclone to use half of it - 5
Mbit/s. This is 5/8 = 0.625MByte/s so you would use a --bwlimit 0.625M
parameter for rclone.

--checkers=N

The number of checkers to run in parallel. Checkers do the equality
checking of files during a sync. For some storage systems (eg s3, swift,
dropbox) this can take a significant amount of time so they are run in
parallel.

The default is to run 8 checkers in parallel.

-c, --checksum

Normally rclone will look at modification time and size of files to see
if they are equal. If you set this flag then rclone will check the file
hash and size to determine if files are equal.

This is useful when the remote doesn't support setting modified time and
a more accurate sync is desired than just checking the file size.

This is very useful when transferring between remotes which store the
same hash type on the object, eg Drive and Swift. For details of which
remotes support which hash type see the table in the overview section.

Eg rclone --checksum sync s3:/bucket swift:/bucket would run much
quicker than without the --checksum flag.

When using this flag, rclone won't update mtimes of remote files if they
are incorrect as it would normally.

--config=CONFIG_FILE

Specify the location of the rclone config file. Normally this is in your
home directory as a file called .rclone.conf. If you run rclone -h and
look at the help for the --config option you will see where the default
location is for you. Use this flag to override the config location, eg
rclone --config=".myconfig" .config.

--contimeout=TIME

Set the connection timeout. This should be in go time format which looks
like 5s for 5 seconds, 10m for 10 minutes, or 3h30m.

The connection timeout is the amount of time rclone will wait for a
connection to go through to a remote object storage system. It is 1m by
default.

--dedupe-mode MODE

Mode to run dedupe command in. One of interactive, skip, first, newest,
oldest, rename. The default is interactive. See the dedupe command for
more information as to what these options mean.

-n, --dry-run

Do a trial run with no permanent changes. Use this to see what rclone
would do without actually doing it. Useful when setting up the sync
command which deletes files in the destination.

--ignore-existing

Using this option will make rclone unconditionally skip all files that
exist on the destination, no matter the content of these files.

While this isn't a generally recommended option, it can be useful in
cases where your files change due to encryption. However, it cannot
correct partial transfers in case a transfer was interrupted.

--ignore-size

Normally rclone will look at modification time and size of files to see
if they are equal. If you set this flag then rclone will check only the
modification time. If --checksum is set then it only checks the
checksum.

It will also cause rclone to skip verifying the sizes are the same after
transfer.

This can be useful for transferring files to and from onedrive which
occasionally misreports the size of image files (see #399 for more
info).

-I, --ignore-times

Using this option will cause rclone to unconditionally upload all files
regardless of the state of files on the destination.

Normally rclone would skip any files that have the same modification
time and are the same size (or have the same checksum if using
--checksum).

--log-file=FILE

Log all of rclone's output to FILE. This is not active by default. This
can be useful for tracking down problems with syncs in combination with
the -v flag. See the Logging section for more info.

--low-level-retries NUMBER

This controls the number of low level retries rclone does.

A low level retry is used to retry a failing operation - typically one
HTTP request. This might be uploading a chunk of a big file for example.
You will see low level retries in the log with the -v flag.

This shouldn't need to be changed from the default in normal operations,
however if you get a lot of low level retries you may wish to reduce the
value so rclone moves on to a high level retry (see the --retries flag)
quicker.

Disable low level retries with --low-level-retries 1.

--max-depth=N

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

--modify-window=TIME

When checking whether a file has been modified, this is the maximum
allowed time difference that a file can have and still be considered
equivalent.

The default is 1ns unless this is overridden by a remote. For example OS
X only stores modification times to the nearest second so if you are
reading and writing to an OS X filing system this will be 1s by default.

This command line flag allows you to override that computed default.

--no-gzip-encoding

Don't set Accept-Encoding: gzip. This means that rclone won't ask the
server for compressed files automatically. Useful if you've set the
server to return files with Content-Encoding: gzip but you uploaded
compressed files.

There is no need to set this in normal operation, and doing so will
decrease the network transfer efficiency of rclone.

--no-update-modtime

When using this flag, rclone won't update modification times of remote
files if they are incorrect as it would normally.

This can be used if the remote is being synced with another tool also
(eg the Google Drive client).

-q, --quiet

Normally rclone outputs stats and a completion message. If you set this
flag it will make as little output as possible.

--retries int

Retry the entire sync if it fails this many times it fails (default 3).

Some remotes can be unreliable and a few retries helps pick up the files
which didn't get transferred because of errors.

Disable retries with --retries 1.

--size-only

Normally rclone will look at modification time and size of files to see
if they are equal. If you set this flag then rclone will check only the
size.

This can be useful transferring files from dropbox which have been
modified by the desktop sync client which doesn't set checksums of
modification times in the same way as rclone.

--stats=TIME

Commands which transfer data (sync, copy, copyto, move, moveto) will
print data transfer stats at regular intervals to show their progress.

This sets the interval.

The default is 1m. Use 0 to disable.

If you set the stats interval then all command can show stats. This can
be useful when running other commands, check or mount for example.

--stats-unit=bits|bytes

By default data transfer rates will be printed in bytes/second.

This option allows the data rate to be printed in bits/second.

Data transfer volume will still be reported in bytes.

The rate is reported as a binary unit, not SI unit. So 1 Mbit/s equals
1,048,576 bits/s and not 1,000,000 bits/s.

The default is bytes.

--delete-(before,during,after)

This option allows you to specify when files on your destination are
deleted when you sync folders.

Specifying the value --delete-before will delete all files present on
the destination, but not on the source _before_ starting the transfer of
any new or updated files. This uses extra memory as it has to store the
source listing before proceeding.

Specifying --delete-during (default value) will delete files while
checking and uploading files. This is usually the fastest option.
Currently this works the same as --delete-after but it may change in the
future.

Specifying --delete-after will delay deletion of files until all
new/updated files have been successfully transfered.

--timeout=TIME

This sets the IO idle timeout. If a transfer has started but then
becomes idle for this long it is considered broken and disconnected.

The default is 5m. Set to 0 to disable.

--transfers=N

The number of file transfers to run in parallel. It can sometimes be
useful to set this to a smaller number if the remote is giving a lot of
timeouts or bigger if you have lots of bandwidth and a fast remote.

The default is to run 4 file transfers in parallel.

-u, --update

This forces rclone to skip any files which exist on the destination and
have a modified time that is newer than the source file.

If an existing destination file has a modification time equal (within
the computed modify window precision) to the source file's, it will be
updated if the sizes are different.

On remotes which don't support mod time directly the time checked will
be the uploaded time. This means that if uploading to one of these
remoes, rclone will skip any files which exist on the destination and
have an uploaded time that is newer than the modification time of the
source file.

This can be useful when transferring to a remote which doesn't support
mod times directly as it is more accurate than a --size-only check and
faster than using --checksum.

-v, --verbose

If you set this flag, rclone will become very verbose telling you about
every file it considers and transfers.

Very useful for debugging.

-V, --version

Prints the version number


Configuration Encryption

Your configuration file contains information for logging in to your
cloud services. This means that you should keep your .rclone.conf file
in a secure location.

If you are in an environment where that isn't possible, you can add a
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
will now be asked for the password. In the same menu you can change the
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
set it in the envonment variable.

If you are running rclone inside a script, you might want to disable
password prompts. To do that, pass the parameter --ask-password=false to
rclone. This will make rclone fail instead of asking for a password if
RCLONE_CONFIG_PASS doesn't contain a valid password.


Developer options

These options are useful when developing or debugging rclone. There are
also some more remote specific options which aren't documented here
which are used for testing. These start with remote name eg
--drive-test-option - see the docs for the remote in question.

--cpuprofile=FILE

Write CPU profile to file. This can be analysed with go tool pprof.

--dump-auth

Dump HTTP headers - will contain sensitive info such as Authorization:
headers - use --dump-headers to dump without Authorization: headers. Can
be very verbose. Useful for debugging only.

--dump-bodies

Dump HTTP headers and bodies - may contain sensitive info. Can be very
verbose. Useful for debugging only.

--dump-filters

Dump the filters to the output. Useful to see exactly what include and
exclude options are filtering on.

--dump-headers

Dump HTTP headers with Authorization: lines removed. May still contain
sensitive info. Can be very verbose. Useful for debugging only.

Use --dump-auth if you do want the Authorization: headers.

--memprofile=FILE

Write memory profile to file. This can be analysed with go tool pprof.

--no-check-certificate=true/false

--no-check-certificate controls whether a client verifies the server's
certificate chain and host name. If --no-check-certificate is true, TLS
accepts any certificate presented by the server and any host name in
that certificate. In this mode, TLS is susceptible to man-in-the-middle
attacks.

This option defaults to false.

THIS SHOULD BE USED ONLY FOR TESTING.

--no-traverse

The --no-traverse flag controls whether the destination file system is
traversed when using the copy or move commands.

If you are only copying a small number of files and/or have a large
number of files on the destination then --no-traverse will stop rclone
listing the destination and save time.

However if you are copying a large number of files, escpecially if you
are doing a copy where lots of the files haven't changed and won't need
copying then you shouldn't use --no-traverse.

It can also be used to reduce the memory usage of rclone when copying -
rclone --no-traverse copy src dst won't load either the source or
destination listings into memory so will use the minimum amount of
memory.


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
-   --dump-filters

See the filtering section.


Logging

rclone has 3 levels of logging, Error, Info and Debug.

By default rclone logs Error and Info to standard error and Debug to
standard output. This means you can redirect standard output and
standard error to different places.

By default rclone will produce Error and Info level messages.

If you use the -q flag, rclone will only produce Error messages.

If you use the -v flag, rclone will produce Error, Info and Debug
messages.

If you use the --log-file=FILE option, rclone will redirect Error, Info
and Debug messages along with standard error to FILE.


Exit Code

If any errors occurred during the command, rclone with an exit code of
1. This allows scripts to detect when rclone operations have failed.

During the startup phase rclone will exit immediately if an error is
detected in the configuration. There will always be a log message
immediately before exiting.

When rclone is running it will accumulate errors as it goes along, and
only exit with an non-zero exit code if (after retries) there were no
transfers with errors remaining. For every error counted there will be a
high priority log message (visibile with -q) showing the message and
which file caused the problem. A high priority message is also shown
when starting a retry so the user can see that any previous error
messages may not be valid after the retry. If rclone has done a retry it
will log a high priority message if the retry was successful.



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

Find the config file by running rclone -h and looking for the help for
the --config option

    $ rclone -h
    [snip]
          --config="/home/user/.rclone.conf": Config file.
    [snip]

Now transfer it to the remote box (scp, cut paste, ftp, sftp etc) and
place it in the correct place (use rclone -h on the remote box to find
out where).



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
"file globs" as used by the unix shell.

If the pattern starts with a / then it only matches at the top level of
the directory tree, RELATIVE TO THE ROOT OF THE REMOTE (not necessarily
the root of the local drive). If it doesn't start with / then it is
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

A [ and ] together make a a character class, such as [a-z] or [aeiou] or
[[:alpha:]]. See the go regexp docs for more info on these.

    h[ae]llo - matches "hello"
             - matches "hallo"
             - doesn't match "hullo"

A { and } define a choice between elements. It should contain a comma
seperated list of patterns, any of which might match. These patterns can
contain wildcards.

    {one,two}_potato - matches "one_potato"
                     - matches "two_potato"
                     - doesn't match "three_potato"
                     - doesn't match "_potato"

Special characters can be escaped with a \ before them.

    \*.jpg       - matches "*.jpg"
    \\.jpg       - matches "\.jpg"
    \[one\].jpg  - matches "[one].jpg"

Note also that rclone filter globs can only be used in one of the filter
command line flags, not in the specification of the remote, so
rclone copy "remote:dir*.jpg" /path/to/dir won't work - what is required
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
won't optimise anything on bucket based remotes (eg s3, swift, google
compute storage, b2) which don't have a concept of directory.

Differences between rsync and rclone patterns

Rclone implements bash style {a,b,c} glob matching which rsync doesn't.

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
the include statement. If this doesn't provide enough flexibility then
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
the include statement. If this doesn't provide enough flexibility then
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

    # a sample exclude rule file
    - secret*.jpg
    + *.jpg
    + *.png
    + file2.avi
    # exclude everything else
    - *

Then use as --filter-from filter-file.txt. The rules are processed in
the order that they are defined.

This example will include all jpg and png files, exclude any files
matching secret*.jpg and include file2.avi. Everything else will be
excluded from the sync.

--files-from - Read list of source-file names

This reads a list of file names from the file passed in and ONLY these
files are transferred. The filtering rules are ignored completely if you
use this option.

This option can be repeated to read from more than one file. These are
read in the order that they are placed on the command line.

Prepare a file like this files-from.txt

    # comment
    file1.jpg
    file2.jpg

Then use as --files-from files-from.txt. This will only transfer
file1.jpg and file2.jpg providing they exist.

For example, let's say you had a few files you want to back up regularly
with these absolute paths:

    /home/user1/important
    /home/user1/dir/file
    /home/user2/stuff

To copy these you'd find a common subdirectory - in this case /home and
put the remaining files in files-from.txt with or without leading /, eg

    user1/important
    user1/dir/file
    user2/stuff

You could then copy these to a remote like this

    rclone copy --files-from files-from.txt /home remote:backup

The 3 files will arrive in remote:backup with the paths as in the
files-from.txt.

You could of course choose / as the root too in which case your
files-from.txt might look like this.

    /home/user1/important
    /home/user1/dir/file
    /home/user2/stuff

And you would transfer it like this

    rclone copy --files-from files-from.txt / remote:backup

In this case there will be an extra home directory on the remote.

--min-size - Don't transfer any file smaller than this

This option controls the minimum size file which will be transferred.
This defaults to kBytes but a suffix of k, M, or G can be used.

For example --min-size 50k means no files smaller than 50kByte will be
transferred.

--max-size - Don't transfer any file larger than this

This option controls the maximum size file which will be transferred.
This defaults to kBytes but a suffix of k, M, or G can be used.

For example --max-size 1G means no files larger than 1GByte will be
transferred.

--max-age - Don't transfer any file older than this

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

--min-age - Don't transfer any file younger than this

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

--dump-filters - dump the filters to the output

This dumps the defined filters to the output as regular expressions.

Useful for debugging.


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



OVERVIEW OF CLOUD STORAGE SYSTEMS


Each cloud storage system is slighly different. Rclone attempts to
provide a unified interface to them, but some underlying differences
show through.


Features

Here is an overview of the major features of each cloud storage system.

  Name                    Hash   ModTime   Case Insensitive   Duplicate Files   MIME Type
  ---------------------- ------ --------- ------------------ ----------------- -----------
  Google Drive            MD5      Yes            No                Yes            R/W
  Amazon S3               MD5      Yes            No                No             R/W
  Openstack Swift         MD5      Yes            No                No             R/W
  Dropbox                  -       No            Yes                No              R
  Google Cloud Storage    MD5      Yes            No                No             R/W
  Amazon Drive            MD5      No            Yes                No              R
  Microsoft One Drive     SHA1     Yes           Yes                No              R
  Hubic                   MD5      Yes            No                No             R/W
  Backblaze B2            SHA1     Yes            No                No             R/W
  Yandex Disk             MD5      Yes            No                No             R/W
  The local filesystem    All      Yes         Depends              No              -

Hash

The cloud storage system supports various hash types of the objects.
The hashes are used when transferring data as an integrity check and can
be specifically used with the --checksum flag in syncs and in the check
command.

To use the checksum checks between filesystems they must support a
common hash type.

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
cloud storage system is case insensitive then that isn't possible.

This can cause problems when syncing between a case insensitive system
and a case sensitive system. The symptom of this is that no matter how
many times you run the sync it never completes fully.

The local filesystem may or may not be case sensitive depending on OS.

-   Windows - usually case insensitive, though case is preserved
-   OSX - usually case insensitive, though it is possible to format case
    sensitive
-   Linux - usually case sensitive, but there are case insensitive file
    systems (eg FAT formatted USB keys)

Most of the time this doesn't cause any problems as people tend to avoid
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

  Name                    Purge   Copy    Move     DirMove   CleanUp
  ---------------------- ------- ------ --------- --------- ---------
  Google Drive             Yes    Yes      Yes       Yes     No #575
  Amazon S3                No     Yes      No        No        No
  Openstack Swift         Yes †   Yes      No        No        No
  Dropbox                  Yes    Yes      Yes       Yes     No #575
  Google Cloud Storage     Yes    Yes      No        No        No
  Amazon Drive             Yes     No      Yes       Yes     No #575
  Microsoft One Drive      Yes    Yes    No #197   No #197   No #575
  Hubic                   Yes †   Yes      No        No        No
  Backblaze B2             No      No      No        No        Yes
  Yandex Disk              Yes     No      No        No      No #575
  The local filesystem     Yes     No      Yes       Yes       No

Purge

This deletes a directory quicker than just deleting all the files in the
directory.

† Note Swift and Hubic implement this in order to delete directory
markers but they don't actually have a quicker way of deleting files
other than deleting them individually.

Copy

Used when copying an object to and from the same remote. This known as a
server side copy so you can copy a file without downloading it and
uploading it again. It is used if you use rclone copy or rclone move if
the remote doesn't support Move directly.

If the server doesn't support Copy directly then for copy operations the
file is downloaded then re-uploaded.

Move

Used when moving/renaming an object on the same remote. This is known as
a server side move of a file. This is used in rclone move if the server
doesn't support DirMove.

If the server isn't capable of Move then rclone simulates it with Copy
then delete. If the server doesn't support Copy then rclone will
download the file and re-upload it.

DirMove

This is used to implement rclone move to move a directory if possible.
If it isn't then it will use Move on each file (which falls back to Copy
then download and upload - see Move section).

CleanUp

This is used for emptying the trash for a remote by rclone cleanup.

If the server can't do CleanUp then rclone cleanup will return an error.


Google Drive

Paths are specified as drive:path

Drive paths may be as deep as required, eg drive:directory/subdirectory.

The initial setup for drive involves getting a token from Google drive
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
     2 / Amazon S3 (also Dreamhost, Ceph)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     6 / Google Drive
       \ "drive"
     7 / Hubic
       \ "hubic"
     8 / Local Disk
       \ "local"
     9 / Microsoft OneDrive
       \ "onedrive"
    10 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    11 / Yandex Disk
       \ "yandex"
    Storage> 6
    Google Application Client Id - leave blank normally.
    client_id> 
    Google Application Client Secret - leave blank normally.
    client_secret> 
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
    client_id = 
    client_secret = 
    token = {"AccessToken":"xxxx.x.xxxxx_xxxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx","RefreshToken":"1/xxxxxxxxxxxxxxxx_xxxxxxxxxxxxxxxxxxxxxxxxxx","Expiry":"2014-03-16T13:57:58.955387075Z","Extra":null}
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

Modified time

Google drive stores modification times accurate to 1 ms.

Revisions

Google drive stores revisions of files. When you upload a change to an
existing file to google drive using rclone it will create a new revision
of that file.

Revisions follow the standard google policy which at time of writing was

-   They are deleted after 30 days or 100 revisions (whatever
    comes first).
-   They do not count towards a user storage quota.

Deleting files

By default rclone will delete files permanently when requested. If
sending them to the trash is required instead then use the
--drive-use-trash flag.

Specific options

Here are the command line options specific to this cloud storage system.

--drive-chunk-size=SIZE

Upload chunk size. Must a power of 2 >= 256k. Default value is 8 MB.

Making this larger will improve performance, but note that each chunk is
buffered in memory one per transfer.

Reducing this will reduce memory usage but decrease performance.

--drive-full-list

No longer does anything - kept for backwards compatibility.

--drive-upload-cutoff=SIZE

File size cutoff for switching to chunked upload. Default is 8 MB.

--drive-use-trash

Send files to the trash instead of deleting permanently. Defaults to
off, namely deleting files permanently.

--drive-auth-owner-only

Only consider files owned by the authenticated user. Requires that
--drive-full-list=true (default).

--drive-formats

Google documents can only be exported from Google drive. When rclone
downloads a Google doc it chooses a format to download depending upon
this setting.

By default the formats are docx,xlsx,pptx,svg which are a sensible
default for an editable document.

When choosing a format, rclone runs down the list provided in order and
chooses the first file format the doc can be exported as from the list.
If the file can't be exported to a format on the formats list, then
rclone will choose a format from the default list.

If you prefer an archive copy then you might use --drive-formats pdf, or
if you prefer openoffice/libreoffice formats you might use
--drive-formats ods,odt,odp.

Note that rclone adds the extension to the google doc, so if it is
calles My Spreadsheet on google docs, it will be exported as
My Spreadsheet.xlsx or My Spreadsheet.pdf etc.

Here are the possible extensions with their corresponding mime types.

  -------------------------------------
  Extension  Mime Type    Description
  ---------- ------------ -------------
  csv        text/csv     Standard CSV
                          format for
                          Spreadsheets

  doc        application/ Micosoft
             msword       Office
                          Document

  docx       application/ Microsoft
             vnd.openxmlf Office
             ormats-offic Document
             edocument.wo 
             rdprocessing 
             ml.document  

  epub       application/ E-book format
             epub+zip     

  html       text/html    An HTML
                          Document

  jpg        image/jpeg   A JPEG Image
                          File

  odp        application/ Openoffice
             vnd.oasis.op Presentation
             endocument.p 
             resentation  

  ods        application/ Openoffice
             vnd.oasis.op Spreadsheet
             endocument.s 
             preadsheet   

  ods        application/ Openoffice
             x-vnd.oasis. Spreadsheet
             opendocument 
             .spreadsheet 

  odt        application/ Openoffice
             vnd.oasis.op Document
             endocument.t 
             ext          

  pdf        application/ Adobe PDF
             pdf          Format

  png        image/png    PNG Image
                          Format

  pptx       application/ Microsoft
             vnd.openxmlf Office
             ormats-offic Powerpoint
             edocument.pr 
             esentationml 
             .presentatio 
             n            

  rtf        application/ Rich Text
             rtf          Format

  svg        image/svg+xm Scalable
             l            Vector
                          Graphics
                          Format

  tsv        text/tab-sep Standard TSV
             arated-value format for
             s            spreadsheets

  txt        text/plain   Plain Text

  xls        application/ Microsoft
             vnd.ms-excel Office
                          Spreadsheet

  xlsx       application/ Microsoft
             vnd.openxmlf Office
             ormats-offic Spreadsheet
             edocument.sp 
             readsheetml. 
             sheet        

  zip        application/ A ZIP file of
             zip          HTML, Images
                          CSS
  -------------------------------------

Limitations

Drive has quite a lot of rate limiting. This causes rclone to be limited
to transferring about 2 files per second only. Individual files may be
transferred much faster at 100s of MBytes/s but lots of small files can
take a long time.

Making your own client_id

When you use rclone with Google drive in its default configuration you
are using rclone's client_id. This is shared between all the rclone
users. There is a global rate limit on the number of queries per second
that each client_id can do set by Google. rclone already has a high
quota and I will continue to make sure it is high enough by contacting
Google.

However you might find you get better performance making your own
client_id if you are a heavy user. Or you may not depending on exactly
how Google have been raising rclone's rate limit.

Here is how to create your own Google Drive client ID for rclone:

1.  Log into the Google API Console with your Google account. It doesn't
    matter what Google account you use. (It need not be the same account
    as the Google Drive you want to access)

2.  Select a project or create a new project.

3.  Under Overview, Google APIs, Google Apps APIs, click "Drive API",
    then "Enable".

4.  Click "Credentials" in the left-side panel (not "Go to credentials",
    which opens the wizard), then "Create credentials", then "OAuth
    client ID". It will prompt you to set the OAuth consent screen
    product name, if you haven't set one already.

5.  Choose an application type of "other", and click "Create". (the
    default name is fine)

6.  It will show you a client ID and client secret. Use these values in
    rclone config to add a new remote or edit an existing remote.

(Thanks to @balazer on github for these instructions.)


Amazon S3

Paths are specified as remote:bucket (or remote: for the lsd command.)
You may put subdirectories in too, eg remote:bucket/path/to/dir.

Here is an example of making an s3 configuration. First run

    rclone config

This will guide you through an interactive setup process.

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    n/s> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     6 / Google Drive
       \ "drive"
     7 / Hubic
       \ "hubic"
     8 / Local Disk
       \ "local"
     9 / Microsoft OneDrive
       \ "onedrive"
    10 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    11 / Yandex Disk
       \ "yandex"
    Storage> 2
    Get AWS credentials from runtime (environment variables or EC2 meta data if no env vars). Only applies if access_key_id and secret_access_key is blank.
    Choose a number from below, or type in your own value
     1 / Enter AWS credentials in the next step
       \ "false"
     2 / Get AWS credentials from the environment (env vars or IAM)
       \ "true"
    env_auth> 1
    AWS Access Key ID - leave blank for anonymous access or runtime credentials.
    access_key_id> access_key
    AWS Secret Access Key (password) - leave blank for anonymous access or runtime credentials.
    secret_access_key> secret_key
    Region to connect to.
    Choose a number from below, or type in your own value
       / The default endpoint - a good choice if you are unsure.
     1 | US Region, Northern Virginia or Pacific Northwest.
       | Leave location constraint empty.
       \ "us-east-1"
       / US West (Oregon) Region
     2 | Needs location constraint us-west-2.
       \ "us-west-2"
       / US West (Northern California) Region
     3 | Needs location constraint us-west-1.
       \ "us-west-1"
       / EU (Ireland) Region Region
     4 | Needs location constraint EU or eu-west-1.
       \ "eu-west-1"
       / EU (Frankfurt) Region
     5 | Needs location constraint eu-central-1.
       \ "eu-central-1"
       / Asia Pacific (Singapore) Region
     6 | Needs location constraint ap-southeast-1.
       \ "ap-southeast-1"
       / Asia Pacific (Sydney) Region
     7 | Needs location constraint ap-southeast-2.
       \ "ap-southeast-2"
       / Asia Pacific (Tokyo) Region
     8 | Needs location constraint ap-northeast-1.
       \ "ap-northeast-1"
       / South America (Sao Paulo) Region
     9 | Needs location constraint sa-east-1.
       \ "sa-east-1"
       / If using an S3 clone that only understands v2 signatures
    10 | eg Ceph/Dreamhost
       | set this and make sure you set the endpoint.
       \ "other-v2-signature"
       / If using an S3 clone that understands v4 signatures set this
    11 | and make sure you set the endpoint.
       \ "other-v4-signature"
    region> 1
    Endpoint for S3 API.
    Leave blank if using AWS to use the default endpoint for the region.
    Specify if using an S3 clone such as Ceph.
    endpoint> 
    Location constraint - must be set to match the Region. Used when creating buckets only.
    Choose a number from below, or type in your own value
     1 / Empty for US Region, Northern Virginia or Pacific Northwest.
       \ ""
     2 / US West (Oregon) Region.
       \ "us-west-2"
     3 / US West (Northern California) Region.
       \ "us-west-1"
     4 / EU (Ireland) Region.
       \ "eu-west-1"
     5 / EU Region.
       \ "EU"
     6 / Asia Pacific (Singapore) Region.
       \ "ap-southeast-1"
     7 / Asia Pacific (Sydney) Region.
       \ "ap-southeast-2"
     8 / Asia Pacific (Tokyo) Region.
       \ "ap-northeast-1"
     9 / South America (Sao Paulo) Region.
       \ "sa-east-1"
    location_constraint> 1
    Canned ACL used when creating buckets and/or storing objects in S3.
    For more info visit http://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
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
    acl> private
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
    [remote]
    env_auth = false
    access_key_id = access_key
    secret_access_key = secret_key
    region = us-east-1
    endpoint = 
    location_constraint = 
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

Modified time

The modified time is stored as metadata on the object as
X-Amz-Meta-Mtime as floating point since the epoch accurate to 1 ns.

Multipart uploads

rclone supports multipart uploads with S3 which means that it can upload
files bigger than 5GB. Note that files uploaded with multipart upload
don't have an MD5SUM.

Buckets and Regions

With Amazon S3 you can list buckets (rclone lsd) using any region, but
you can only access the content of a bucket from the region it was
created in. If you attempt to access a bucket from the wrong region, you
will get an error, incorrect region, the bucket is not in 'XXX' region.

Authentication

There are two ways to supply rclone with a set of AWS credentials. In
order of precedence:

-   Directly in the rclone configuration file (as configured by
    rclone config)
-   set access_key_id and secret_access_key
-   Runtime configuration:
-   set env_auth to true in the config file
-   Exporting the following environment variables before running rclone
    -   Access Key ID: AWS_ACCESS_KEY_ID or AWS_ACCESS_KEY
    -   Secret Access Key: AWS_SECRET_ACCESS_KEY or AWS_SECRET_KEY
-   Running rclone on an EC2 instance with an IAM role

If none of these option actually end up providing rclone with AWS
credentials then S3 interaction will be non-authenticated (see below).

Specific options

Here are the command line options specific to this cloud storage system.

--s3-acl=STRING

Canned ACL used when creating buckets and/or storing objects in S3.

For more info visit the canned ACL docs.

--s3-storage-class=STRING

Storage class to upload new objects with.

Available options include:

-   STANDARD - default storage class
-   STANDARD_IA - for less frequently accessed data (e.g backups)
-   REDUCED_REDUNDANCY (only for noncritical, reproducible data, has
    lower redundancy)

Anonymous access to public buckets

If you want to use rclone to access a public bucket, configure with a
blank access_key_id and secret_access_key. Eg

    No remotes found - make a new one
    n) New remote
    q) Quit config
    n/q> n
    name> anons3
    What type of source is it?
    Choose a number from below
     1) amazon cloud drive
     2) b2
     3) drive
     4) dropbox
     5) google cloud storage
     6) swift
     7) hubic
     8) local
     9) onedrive
    10) s3
    11) yandex
    type> 10
    Get AWS credentials from runtime (environment variables or EC2 meta data if no env vars). Only applies if access_key_id and secret_access_key is blank.
    Choose a number from below, or type in your own value
     * Enter AWS credentials in the next step
     1) false
     * Get AWS credentials from the environment (env vars or IAM)
     2) true
    env_auth> 1
    AWS Access Key ID - leave blank for anonymous access or runtime credentials.
    access_key_id>
    AWS Secret Access Key (password) - leave blank for anonymous access or runtime credentials.
    secret_access_key>
    ...

Then use it as normal with the name of the public bucket, eg

    rclone lsd anons3:1000genomes

You will be able to list and copy data but not upload it.

Ceph

Ceph is an object storage system which presents an Amazon S3 interface.

To use rclone with ceph, you need to set the following parameters in the
config.

    access_key_id = Whatever
    secret_access_key = Whatever
    endpoint = https://ceph.endpoint.goes.here/
    region = other-v2-signature

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

Minio

Minio is an object storage server built for cloud application developers
and devops.

It is very easy to install and provides an S3 compatible server which
can be used by rclone.

To use it, install Minio following the instructions from the web site.

When it configures itself Minio will print something like this

    AccessKey: WLGDGYAQYIGI833EV05A  SecretKey: BYvgJM101sHngl2uzjXS/OBF/aMxAN06JrJ3qJlF Region: us-east-1

    Minio Object Storage:
         http://127.0.0.1:9000
         http://10.0.0.3:9000

    Minio Browser:
         http://127.0.0.1:9000
         http://10.0.0.3:9000

These details need to go into rclone config like this. Note that it is
important to put the region in as stated above.

    env_auth> 1
    access_key_id> WLGDGYAQYIGI833EV05A
    secret_access_key> BYvgJM101sHngl2uzjXS/OBF/aMxAN06JrJ3qJlF   
    region> us-east-1
    endpoint> http://10.0.0.3:9000
    location_constraint> 
    server_side_encryption>

Which makes the config file look like this

    [minio]
    env_auth = false
    access_key_id = WLGDGYAQYIGI833EV05A
    secret_access_key = BYvgJM101sHngl2uzjXS/OBF/aMxAN06JrJ3qJlF
    region = us-east-1
    endpoint = http://10.0.0.3:9000
    location_constraint = 
    server_side_encryption = 

Minio doesn't support all the features of S3 yet. In particular it
doesn't support MD5 checksums (ETags) or metadata. This means rclone
can't check MD5SUMs or store the modified date. However you can work
around this with the --size-only flag of rclone.

So once set up, for example to copy files into a bucket

    rclone --size-only copy /path/to/files minio:bucket


Swift

Swift refers to Openstack Object Storage. Commercial implementations of
that being:

-   Rackspace Cloud Files
-   Memset Memstore

Paths are specified as remote:container (or remote: for the lsd
command.) You may put subdirectories in too, eg
remote:container/path/to/dir.

Here is an example of making a swift configuration. First run

    rclone config

This will guide you through an interactive setup process.

    No remotes found - make a new one
    n) New remote
    s) Set configuration password
    n/s> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     6 / Google Drive
       \ "drive"
     7 / Hubic
       \ "hubic"
     8 / Local Disk
       \ "local"
     9 / Microsoft OneDrive
       \ "onedrive"
    10 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    11 / Yandex Disk
       \ "yandex"
    Storage> 10
    User name to log in.
    user> user_name
    API key or password.
    key> password_or_api_key
    Authentication URL for server.
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
    auth> 1
    User domain - optional (v3 auth)
    domain> Default
    Tenant name - optional
    tenant> 
    Tenant domain - optional (v3 auth)
    tenant_domain>
    Region name - optional
    region> 
    Storage URL - optional
    storage_url> 
    Remote config
    AuthVersion - optional - set to (1,2,3) if your auth URL has no version
    auth_version> 
    --------------------
    [remote]
    user = user_name
    key = password_or_api_key
    auth = https://auth.api.rackspacecloud.com/v1.0
    tenant = 
    region = 
    storage_url = 
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

Configuration from an Openstack credentials file

An Opentstack credentials file typically looks something something like
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

Specific options

Here are the command line options specific to this cloud storage system.

--swift-chunk-size=SIZE

Above this size files will be chunked into a _segments container. The
default for this is 5GB which is its maximum value.

Modified time

The modified time is stored as metadata on the object as
X-Object-Meta-Mtime as floating point since the epoch accurate to 1 ns.

This is a defacto standard (used in the official python-swiftclient
amongst others) for storing the modification time for an object.

Limitations

The Swift API doesn't return a correct MD5SUM for segmented files
(Dynamic or Static Large Objects) so rclone won't check or use the
MD5SUM for these.

Troubleshooting

Rclone gives Failed to create file system for "remote:": Bad Request

Due to an oddity of the underlying swift library, it gives a "Bad
Request" error rather than a more sensible error when the authentication
fails for Swift.

So this most likely means your username / password is wrong. You can
investigate further with the --dump-bodies flag.

This may also be caused by specifying the region when you shouldn't have
(eg OVH).

Rclone gives Failed to create file system: Response didn't have storage storage url and auth token

This is most likely caused by forgetting to specify your tenant when
setting up a swift remote.


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
     2 / Amazon S3 (also Dreamhost, Ceph)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     6 / Google Drive
       \ "drive"
     7 / Hubic
       \ "hubic"
     8 / Local Disk
       \ "local"
     9 / Microsoft OneDrive
       \ "onedrive"
    10 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    11 / Yandex Disk
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

Modified time and MD5SUMs

Dropbox doesn't provide the ability to set modification times in the V1
public API, so rclone can't support modified time with Dropbox.

This may change in the future - see these issues for details:

-   Dropbox V2 API
-   Allow syncs for remotes that can't set modtime on existing objects

Dropbox doesn't return any sort of checksum (MD5 or SHA1).

Together that means that syncs to dropbox will effectively have the
--size-only flag set.

Specific options

Here are the command line options specific to this cloud storage system.

--dropbox-chunk-size=SIZE

Upload chunk size. Max 150M. The default is 128MB. Note that this isn't
buffered into memory.

Limitations

Note that Dropbox is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

There are some file names such as thumbs.db which Dropbox can't store.
There is a full list of them in the "Ignored Files" section of this
document. Rclone will issue an error message
File name disallowed - not uploading if it attempt to upload one of
those file names, but the sync won't fail.

If you have more than 10,000 files in a directory then
rclone purge dropbox:dir will return the error
Failed to purge: There are too many files involved in this operation. As
a work-around do an rclone delete dropbix:dir followed by an
rclone rmdir dropbox:dir.


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
     2 / Amazon S3 (also Dreamhost, Ceph)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     6 / Google Drive
       \ "drive"
     7 / Hubic
       \ "hubic"
     8 / Local Disk
       \ "local"
     9 / Microsoft OneDrive
       \ "onedrive"
    10 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    11 / Yandex Disk
       \ "yandex"
    Storage> 5
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
     * Object owner gets OWNER access, and all Authenticated Users get READER access.
     1) authenticatedRead
     * Object owner gets OWNER access, and project team owners get OWNER access.
     2) bucketOwnerFullControl
     * Object owner gets OWNER access, and project team owners get READER access.
     3) bucketOwnerRead
     * Object owner gets OWNER access [default if left blank].
     4) private
     * Object owner gets OWNER access, and project team members get access according to their roles.
     5) projectPrivate
     * Object owner gets OWNER access, and all Users get READER access.
     6) publicRead
    object_acl> 4
    Access Control List for new buckets.
    Choose a number from below, or type in your own value
     * Project team owners get OWNER access, and all Authenticated Users get READER access.
     1) authenticatedRead
     * Project team owners get OWNER access [default if left blank].
     2) private
     * Project team members get access according to their roles.
     3) projectPrivate
     * Project team owners get OWNER access, and all Users get READER access.
     4) publicRead
     * Project team owners get OWNER access, and all Users get WRITER access.
     5) publicReadWrite
    bucket_acl> 2
    Remote config
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
i.e. not tied to a specific end-user Google account. This is useful when
you want to synchronise files onto machines that don't have actively
logged-in users, for example build machines.

To get credentials for Google Cloud Platform IAM Service Accounts,
please head to the Service Account section of the Google Developer
Console. Service Accounts behave just like normal User permissions in
Google Cloud Storage ACLs, so you can limit their access (e.g. make them
read only). After creating an account, a JSON file containing the
Service Account's credentials will be downloaded onto your machines.
These credentials are what rclone will use for authentication.

To use a Service Account instead of OAuth2 token flow, enter the path to
your Service Account credentials at the service_account_file prompt and
rclone won't use the browser based authentication flow.

Modified time

Google google cloud storage stores md5sums natively and rclone stores
modification times as metadata on the object, under the "mtime" key in
RFC3339 format accurate to 1ns.


Amazon Drive

Paths are specified as remote:path

Paths may be as deep as required, eg remote:directory/subdirectory.

The initial setup for Amazon Drive involves getting a token from Amazon
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
     2 / Amazon S3 (also Dreamhost, Ceph)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     6 / Google Drive
       \ "drive"
     7 / Hubic
       \ "hubic"
     8 / Local Disk
       \ "local"
     9 / Microsoft OneDrive
       \ "onedrive"
    10 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    11 / Yandex Disk
       \ "yandex"
    Storage> 1
    Amazon Application Client Id - leave blank normally.
    client_id> 
    Amazon Application Client Secret - leave blank normally.
    client_secret> 
    Remote config
    If your browser doesn't open automatically go to the following link: http://127.0.0.1:53682/auth
    Log in and authorize rclone for access
    Waiting for code...
    Got code
    --------------------
    [remote]
    client_id = 
    client_secret = 
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

Amazon Drive doesn't allow modification times to be changed via the API
so these won't be accurate or used for syncing.

It does store MD5SUMs so for a more accurate sync, you can use the
--checksum flag.

Deleting files

Any files you delete with rclone will end up in the trash. Amazon don't
provide an API to permanently delete files, nor to empty the trash, so
you will have to do that with one of Amazon's apps or via the Amazon
Drive website.

Using with non .com Amazon accounts

Let's say you usually use amazon.co.uk. When you authenticate with
rclone it will take you to an amazon.com page to log in. Your
amazon.co.uk email and password should work here just fine.

Specific options

Here are the command line options specific to this cloud storage system.

--acd-templink-threshold=SIZE

Files this size or more will be downloaded via their tempLink. This is
to work around a problem with Amazon Drive which blocks downloads of
files bigger than about 10GB. The default for this is 9GB which
shouldn't need to be changed.

To download files above this threshold, rclone requests a tempLink which
downloads the file through a temporary URL directly from the underlying
S3 storage.

--acd-upload-wait-per-gb=TIME

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

Upload with the -v flag to see more info about what rclone is doing in
this situation.

Limitations

Note that Amazon Drive is case insensitive so you can't have a file
called "Hello.doc" and one called "hello.doc".

Amazon Drive has rate limiting so you may notice errors in the sync (429
errors). rclone will automatically retry the sync up to 3 times by
default (see --retries flag) which should hopefully work around this
problem.

Amazon Drive has an internal limit of file sizes that can be uploaded to
the service. This limit is not officially published, but all files
larger than this will fail.

At the time of writing (Jan 2016) is in the area of 50GB per file. This
means that larger files are likely to fail.

Unfortunatly there is no way for rclone to see that this failure is
because of file size, so it will retry the operation, as any other
failure. To avoid this problem, use --max-size 50000M option to limit
the maximum size of uploaded files. Note that --max-size does not split
files into segments, it only ignores files over this size.


Microsoft One Drive

Paths are specified as remote:path

Paths may be as deep as required, eg remote:directory/subdirectory.

The initial setup for One Drive involves getting a token from Microsoft
which you need to do in your browser. rclone config walks you through
it.

Here is an example of how to make a remote called remote. First run:

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
     2 / Amazon S3 (also Dreamhost, Ceph)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     6 / Google Drive
       \ "drive"
     7 / Hubic
       \ "hubic"
     8 / Local Disk
       \ "local"
     9 / Microsoft OneDrive
       \ "onedrive"
    10 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    11 / Yandex Disk
       \ "yandex"
    Storage> 9
    Microsoft App Client Id - leave blank normally.
    client_id> 
    Microsoft App Client Secret - leave blank normally.
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
token as returned from Microsoft. This only runs from the moment it
opens your browser to the moment you get back the verification code.
This is on http://127.0.0.1:53682/ and this it may require you to
unblock it temporarily if you are running a host firewall.

Once configured you can then use rclone like this,

List directories in top level of your One Drive

    rclone lsd remote:

List all the files in your One Drive

    rclone ls remote:

To copy a local directory to an One Drive directory called backup

    rclone copy /home/source remote:backup

Modified time and hashes

One Drive allows modification times to be set on objects accurate to 1
second. These will be used to detect whether objects need syncing or
not.

One drive supports SHA1 type hashes, so you can use --checksum flag.

Deleting files

Any files you delete with rclone will end up in the trash. Microsoft
doesn't provide an API to permanently delete files, nor to empty the
trash, so you will have to do that with one of Microsoft's apps or via
the One Drive website.

Specific options

Here are the command line options specific to this cloud storage system.

--onedrive-chunk-size=SIZE

Above this size files will be chunked - must be multiple of 320k. The
default is 10MB. Note that the chunks will be buffered into memory.

--onedrive-upload-cutoff=SIZE

Cutoff for switching to chunked upload - must be <= 100MB. The default
is 10MB.

Limitations

Note that One Drive is case insensitive so you can't have a file called
"Hello.doc" and one called "hello.doc".

Rclone only supports your default One Drive, and doesn't work with One
Drive for business. Both these issues may be fixed at some point
depending on user demand!

There are quite a few characters that can't be in One Drive file names.
These can't occur on Windows platforms, but on non-Windows platforms
they are common. Rclone will map these names to and from an identical
looking unicode equivalent. For example if a file has a ? in it will be
mapped to ？ instead.

The largest allowed file size is 10GiB (10,737,418,240 bytes).


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
     2 / Amazon S3 (also Dreamhost, Ceph)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     6 / Google Drive
       \ "drive"
     7 / Hubic
       \ "hubic"
     8 / Local Disk
       \ "local"
     9 / Microsoft OneDrive
       \ "onedrive"
    10 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    11 / Yandex Disk
       \ "yandex"
    Storage> 7
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

Modified time

The modified time is stored as metadata on the object as
X-Object-Meta-Mtime as floating point since the epoch accurate to 1 ns.

This is a defacto standard (used in the official python-swiftclient
amongst others) for storing the modification time for an object.

Note that Hubic wraps the Swift backend, so most of the properties of
are the same.

Limitations

This uses the normal OpenStack Swift mechanism to refresh the Swift API
credentials and ignores the expires field returned by the Hubic API.

The Swift API doesn't return a correct MD5SUM for segmented files
(Dynamic or Static Large Objects) so rclone won't check or use the
MD5SUM for these.


Backblaze B2

B2 is Backblaze's cloud storage system.

Paths are specified as remote:bucket (or remote: for the lsd command.)
You may put subdirectories in too, eg remote:bucket/path/to/dir.

Here is an example of making a b2 configuration. First run

    rclone config

This will guide you through an interactive setup process. You will need
your account number (a short hex number) and key (a long hex number)
which you can get from the b2 control panel.

    No remotes found - make a new one
    n) New remote
    q) Quit config
    n/q> n
    name> remote
    Type of storage to configure.
    Choose a number from below, or type in your own value
     1 / Amazon Drive
       \ "amazon cloud drive"
     2 / Amazon S3 (also Dreamhost, Ceph)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     6 / Google Drive
       \ "drive"
     7 / Hubic
       \ "hubic"
     8 / Local Disk
       \ "local"
     9 / Microsoft OneDrive
       \ "onedrive"
    10 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    11 / Yandex Disk
       \ "yandex"
    Storage> 3
    Account ID
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

Make a new bucket

    rclone mkdir remote:bucket

List the contents of a bucket

    rclone ls remote:bucket

Sync /home/local/directory to the remote bucket, deleting any excess
files in the bucket.

    rclone sync /home/local/directory remote:bucket

Modified time

The modified time is stored as metadata on the object as
X-Bz-Info-src_last_modified_millis as milliseconds since 1970-01-01 in
the Backblaze standard. Other tools should be able to use this as a
modified time.

Modified times are used in syncing and are fully supported except in the
case of updating a modification time on an existing object. In this case
the object will be uploaded again as B2 doesn't have an API method to
set the modification time independent of doing an upload.

SHA1 checksums

The SHA1 checksums of the files are checked on upload and download and
will be used in the syncing process.

Large files which are uploaded in chunks will store their SHA1 on the
object as X-Bz-Info-large_file_sha1 as recommended by Backblaze.

Transfers

Backblaze recommends that you do lots of transfers simultaneously for
maximum speed. In tests from my SSD equiped laptop the optimum setting
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
it. Likewise when you delete a file, the old version will still be
available.

Old versions of files are visible using the --b2-versions flag.

If you wish to remove all the old versions then you can use the
rclone cleanup remote:bucket command which will delete all the old
versions of files, leaving the current ones intact. You can also supply
a path and only old versions under that path will be deleted, eg
rclone cleanup remote:bucket/path/to/stuff.

When you purge a bucket, the current and the old versions will be
deleted then the bucket will be deleted.

However delete will cause the current versions of the files to become
hidden old versions.

Here is a session showing the listing and and retreival of an old
version followed by a cleanup of the old versions.

Show current version and all the versions with --b2-versions flag.

    $ rclone -q ls b2:cleanup-test
            9 one.txt

    $ rclone -q --b2-versions ls b2:cleanup-test
            9 one.txt
            8 one-v2016-07-04-141032-000.txt
           16 one-v2016-07-04-141003-000.txt
           15 one-v2016-07-02-155621-000.txt

Retreive an old verson

    $ rclone -q --b2-versions copy b2:cleanup-test/one-v2016-07-04-141003-000.txt /tmp

    $ ls -l /tmp/one-v2016-07-04-141003-000.txt
    -rw-rw-r-- 1 ncw ncw 16 Jul  2 17:46 /tmp/one-v2016-07-04-141003-000.txt

Clean up all the old versions and show that they've gone.

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

B2 with crypt

When using B2 with crypt files are encrypted into a temporary location
and streamed from there. This is required to calculate the encrypted
file's checksum before beginning the upload. On Windows the %TMPDIR%
environment variable is used as the temporary location. If the file
requires chunking, both the chunking and encryption will take place in
memory.

Specific options

Here are the command line options specific to this cloud storage system.

--b2-chunk-size valuee=SIZE

When uploading large files chunk the file into this size. Note that
these chunks are buffered in memory and there might a maximum of
--transfers chunks in progress at once. 100,000,000 Bytes is the minimim
size (default 96M).

--b2-upload-cutoff=SIZE

Cutoff for switching to chunked upload (default 190.735 MiB == 200 MB).
Files above this size will be uploaded in chunks of --b2-chunk-size.

This value should be set no larger than 4.657GiB (== 5GB) as this is the
largest file size that can be uploaded.

--b2-test-mode=FLAG

This is for debugging purposes only.

Setting FLAG to one of the strings below will cause b2 to return
specific errors for debugging purposes.

-   fail_some_uploads
-   expire_some_account_authorization_tokens
-   force_cap_exceeded

These will be set in the X-Bz-Test-Mode header which is documented in
the b2 integrations checklist.

--b2-versions

When set rclone will show and act on older versions of files. For
example

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
permitted, so you can't upload files or delete them.


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
     2 / Amazon S3 (also Dreamhost, Ceph)
       \ "s3"
     3 / Backblaze B2
       \ "b2"
     4 / Dropbox
       \ "dropbox"
     5 / Google Cloud Storage (this is not Google Drive)
       \ "google cloud storage"
     6 / Google Drive
       \ "drive"
     7 / Hubic
       \ "hubic"
     8 / Local Disk
       \ "local"
     9 / Microsoft OneDrive
       \ "onedrive"
    10 / Openstack Swift (Rackspace Cloud Files, Memset Memstore, OVH)
       \ "swift"
    11 / Yandex Disk
       \ "yandex"
    Storage> 11
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


Crypt

The crypt remote encrypts and decrypts another remote.

To use it first set up the underlying remote following the config
instructions for that remote. You can also use a local pathname instead
of a remote which will encrypt and decrypt from that directory which
might be useful for encrypting onto a USB stick for example.

First check your chosen remote is working - we'll call it remote:path in
these docs. Note that anything inside remote:path will be encrypted and
anything outside won't. This means that if you are using a bucket based
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
    12 / Yandex Disk
       \ "yandex"
    Storage> 5
    Remote to encrypt/decrypt.
    Normally should contain a ':' and a path, eg "myremote:path/to/dir",
    "myremote:bucket" or "myremote:"
    remote> remote:path
    How to encrypt the filenames.
    Choose a number from below, or type in your own value
     1 / Don't encrypt the file names.  Adds a ".bin" extension only.
       \ "off"
     2 / Encrypt the filenames see the docs for the details.
       \ "standard"
    filename_encryption> 2
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
so it isn't immediately obvious what it is. It is in no way secure
unless you use config file encryption.

A long passphrase is recommended, or you can use a random one. Note that
if you reconfigure rclone with the same passwords/passphrases elsewhere
it will be compatible - all the secrets used are derived from those two
passwords/passphrases.

Note that rclone does not encrypt * file length - this can be calcuated
within 16 bytes * modification time - used for syncing


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
manage because you won't know what directory they represent in web
interfaces etc), you should probably specify a bucket, eg
remote:secretbucket when using bucket based remotes such as S3, Swift,
Hubic, B2, GCS.


Example

To test I made a little directory of files using "standard" file name
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

If don't use file name encryption then the remote will look like this -
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

Off * doesn't hide file names or directory structure * allows for longer
file names (~246 characters) * can use sub paths and copy single files

Standard * file names encrypted * file names can't be as long (~156
characters) * can use sub paths and copy single files * directory
structure visibile * identical files names will have identical uploaded
names * can use shortcuts to shorten the directory recursion

Cloud storage systems have various limits on file name length and total
path length which you are more likely to hit using "Standard" file name
encryption. If you keep your file names to below 156 characters in
length then you should be OK on all providers.

There may be an even more secure file name encryption mode in the future
which will address the long file name problem.

Modified time and hashes

Crypt stores modification times using the underlying remote so support
depends on that.

Hashes are not stored for crypt. However the data integrity is protected
by an extremely strong crypto authenticator.


File formats

File encryption

Files are encrypted 1:1 source file to destination object. The file has
a header and is divided into chunks.

Header

-   8 bytes magic string RCLONE\x00\x00
-   24 bytes Nonce (IV)

The initial nonce is generated from the operating systems crypto strong
random number genrator. The nonce is incremented for each chunk read
making sure each nonce is unique for each block written. The chance of a
nonce being re-used is miniscule. If you wrote an exabyte of data (10¹⁸
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
buffered in memory so they can't be too big.

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
paper "A Parallelizable Enciphering Mode" by Halevi and Rogaway.

This makes for determinstic encryption which is what we want - the same
filename must encrypt to the same thing otherwise we can't find it on
the cloud storage system.

This means that

-   filenames with the same name will encrypt the same
-   filenames which start the same won't have a common prefix

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

Rclone uses scrypt with parameters N=16384, r=8, p=1 with a an optional
user supplied salt (password2) to derive the 32+32+16 = 80 bytes of key
material required. If the user doesn't supply a salt then rclone uses an
internal one.

scrypt makes it impractical to mount a dictionary attack on rclone
encrypted data. For full protection agains this you should always use a
salt.


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
available in most distributions' package managers.

If an invalid (non-UTF8) filename is read, the invalid caracters will be
replaced with the unicode replacement character, '�'. rclone will emit a
debug message in this case (use -v to see), eg

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

Specific options

Here are the command line options specific to local storage

--one-file-system, -x

This tells rclone to stay in the filesystem specified by the root and
not to recurse into different file systems.

For example if you have a directory heirachy like this

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
it isn't supported (eg Windows) it will not appear as an valid flag.


Changelog

-   v1.35 - 2017-01-02
    -   New Features
    -   moveto and copyto commands for choosing a destination name on
        copy/move
    -   rmdirs command to recursively delete empty directories
    -   Allow repeated --include/--exclude/--filter options
    -   Only show transfer stats on commands which transfer stuff
        -   show stats on any command using the --stats flag
    -   Allow overlapping directories in move when server side dir move
        is supported
    -   Add --stats-unit option - thanks Scott McGillivray
    -   Bug Fixes
    -   Fix the config file being overwritten when two rclones are
        running
    -   Make rclone lsd obey the filters properly
    -   Fix compilation on mips
    -   Fix not transferring files that don't differ in size
    -   Fix panic on nil retry/fatal error
    -   Mount
    -   Retry reads on error - should help with reliability a lot
    -   Report the modification times for directories from the remote
    -   Add bandwidth accounting and limiting (fixes --bwlimit)
    -   If --stats provided will show stats and which files are
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
        -   This makes --max-depth 1 dir listings as used in mount much
            faster
    -   Reauth the account while doing uploads too - should help with
        token expiry
    -   Drive
    -   Make DirMove more efficient and complain about moving the root
    -   Create destination directory on Move()
-   v1.34 - 2016-11-06
    -   New Features
    -   Stop single file and --files-from operations iterating through
        the source bucket.
    -   Stop removing failed upload to cloud storage remotes
    -   Make ContentType be preserved for cloud to cloud copies
    -   Add support to toggle bandwidth limits via SIGUSR2 - thanks
        Marco Paganini
    -   rclone check shows count of hashes that couldn't be checked
    -   rclone listremotes command
    -   Support linux/arm64 build - thanks Fredrik Fornwall
    -   Remove Authorization: lines from --dump-headers output
    -   Bug Fixes
    -   Ignore files with control characters in the names
    -   Fix rclone move command
        -   Delete src files which already existed in dst
        -   Fix deletion of src file when dst file older
    -   Fix rclone check on crypted file systems
    -   Make failed uploads not count as "Transferred"
    -   Make sure high level retries show with -q
    -   Use a vendor directory with godep for repeatable builds
    -   rclone mount - FUSE
    -   Implement FUSE mount options
        -   --no-modtime, --debug-fuse, --read-only, --allow-non-empty,
            --allow-root, --allow-other
        -   --default-permissions, --write-back-cache, --max-read-ahead,
            --umask, --uid, --gid
    -   Add --dir-cache-time to control caching of directory entries
    -   Implement seek for files opened for read (useful for
        video players)
        -   with -no-seek flag to disable
    -   Fix crash on 32 bit ARM (alignment of 64 bit counter)
    -   ...and many more internal fixes and improvements!
    -   Crypt
    -   Don't show encrypted password in configurator to stop confusion
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
    -   Don't read metadata for directory marker objects
-   v1.33 - 2016-08-24
    -   New Features
    -   Implement encryption
        -   data encrypted in NACL secretbox format
        -   with optional file name encryption
    -   New commands
        -   rclone mount - implements FUSE mounting of
            remotes (EXPERIMENTAL)
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
    -   Fix URL escaping in file names - eg uploading files with +
        in them.
    -   amazon cloud drive
    -   Fix token expiry during large uploads
    -   Work around 408 REQUEST_TIMEOUT and 504 GATEWAY_TIMEOUT errors
    -   local
    -   Fix filenames with invalid UTF-8 not being uploaded
    -   Fix problem with some UTF-8 characters on OS X
-   v1.32 - 2016-07-13
    -   Backblaze B2
    -   Fix upload of files large files not in root
-   v1.31 - 2016-07-13
    -   New Features
    -   Reduce memory on sync by about 50%
    -   Implement --no-traverse flag to stop copy traversing the
        destination remote.
        -   This can be used to reduce memory usage down to the
            smallest possible.
        -   Useful to copy a small number of files into a large
            destination folder.
    -   Implement cleanup command for emptying trash / removing old
        versions of files
        -   Currently B2 only
    -   Single file handling improved
        -   Now copied with --files-from
        -   Automatically sets --no-traverse when copying a single file
    -   Info on using installing with ansible - thanks Stefan Weichinger
    -   Implement --no-update-modtime flag to stop rclone fixing the
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
    -   Add support for non-default project domain - thanks
        Antonio Messina.
    -   S3
    -   Add instructions on how to use rclone with minio.
    -   Add ap-northeast-2 (Seoul) and ap-south-1 (Mumbai) regions.
    -   Skip setting the modified time for objects > 5GB as it
        isn't possible.
    -   Backblaze B2
    -   Add --b2-versions flag so old versions can be listed
        and retreived.
    -   Treat 403 errors (eg cap exceeded) as fatal.
    -   Implement cleanup command for deleting old file versions.
    -   Make error handling compliant with B2 integrations notes.
    -   Fix handling of token expiry.
    -   Implement --b2-test-mode to set X-Bz-Test-Mode header.
    -   Set cutoff for chunked upload to 200MB as per B2 guidelines.
    -   Make upload multi-threaded.
    -   Dropbox
    -   Don't retry 461 errors.
-   v1.30 - 2016-06-18
    -   New Features
    -   Directory listing code reworked for more features and better
        error reporting (thanks to Klaus Post for help). This enables
        -   Directory include filtering for efficiency
        -   --max-depth parameter
        -   Better error reporting
        -   More to come
    -   Retry more errors
    -   Add --ignore-size flag - for uploading images to onedrive
    -   Log -v output to stdout by default
    -   Display the transfer stats in more human readable form
    -   Make 0 size files specifiable with --max-size 0b
    -   Add b suffix so we can specify bytes in --bwlimit, --min-size
        etc
    -   Use "password:" instead of "password>" prompt - thanks Klaus
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
    -   Make sure we don't use conflicting content types on upload
    -   Add service account support - thanks Michal Witkowski
    -   Swift
    -   Add auth version parameter
    -   Add domain option for openstack (v3 auth) - thanks Fabian Ruff
-   v1.29 - 2016-04-18
    -   New Features
    -   Implement -I, --ignore-times for unconditional upload
    -   Improve dedupecommand
        -   Now removes identical copies without asking
        -   Now obeys --dry-run
        -   Implement --dedupe-mode for non interactive running
        -   --dedupe-mode interactive - interactive the default.
        -   --dedupe-mode skip - removes identical files then skips
            anything left.
        -   --dedupe-mode first - removes identical files then keeps the
            first one.
        -   --dedupe-mode newest - removes identical files then keeps
            the newest one.
        -   --dedupe-mode oldest - removes identical files then keeps
            the oldest one.
        -   --dedupe-mode rename - removes identical files then renames
            the rest to be different.
    -   Bug fixes
    -   Make rclone check obey the --size-only flag.
    -   Use "application/octet-stream" if discovered mime type
        is invalid.
    -   Fix missing "quit" option when there are no remotes.
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
        -   (there isn't an API to just set the mod time.)
        -   If you want the old behaviour use --size-only.
    -   Update API to new version
    -   Fix parsing of mod time when not in metadata
    -   Swift/Hubic
    -   Don't return an MD5SUM for static large objects
    -   S3
    -   Fix uploading files bigger than 50GB
-   v1.28 - 2016-03-01
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
    -   Don't make directories if --dry-run set
    -   Fix and document the move command
    -   Fix redirecting stderr on unix-like OSes when using --log-file
    -   Fix delete command to wait until all finished - fixes
        missing deletes.
    -   Backblaze B2
    -   Use one upload URL per go routine fixes
        more than one upload using auth token
    -   Add pacing, retries and reauthentication - fixes token expiry
        problems
    -   Upload without using a temporary file from local (and remotes
        which support SHA1)
    -   Fix reading metadata for all files when it shouldn't have been
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
-   v1.27 - 2016-01-31
    -   New Features
    -   Easier headless configuration with rclone authorize
    -   Add support for multiple hash types - we now check SHA1 as well
        as MD5 hashes.
    -   delete command which does obey the filters (unlike purge)
    -   dedupe command to deduplicate a remote. Useful with
        Google Drive.
    -   Add --ignore-existing flag to skip all files that exist
        on destination.
    -   Add --delete-before, --delete-during, --delete-after flags.
    -   Add --memprofile flag to debug memory use.
    -   Warn the user about files with same name but different case
    -   Make --include rules add their implict exclude * at the end of
        the filter list
    -   Deprecate compiling with go1.3
    -   Amazon Drive
    -   Fix download of files > 10 GB
    -   Fix directory traversal ("Next token is expired") for large
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
-   v1.26 - 2016-01-02
    -   New Features
    -   Yandex storage backend - thank you Dmitry Burdeev ("dibu")
    -   Implement Backblaze B2 storage backend
    -   Add --min-age and --max-age flags - thank you Adriano Aurélio
        Meirelles
    -   Make ls/lsl/md5sum/size/check obey includes and excludes
    -   Fixes
    -   Fix crash in http logging
    -   Upload releases to github too
    -   Swift
    -   Fix sync for chunked files
    -   One Drive
    -   Re-enable server side copy
    -   Don't mask HTTP error codes with JSON decode error
    -   S3
    -   Fix corrupting Content-Type on mod time update (thanks
        Joseph Spurrier)
-   v1.25 - 2015-11-14
    -   New features
    -   Implement Hubic storage system
    -   Fixes
    -   Fix deletion of some excluded files without --delete-excluded
        -   This could have deleted files unexpectedly on sync
        -   Always check first with --dry-run!
    -   Swift
    -   Stop SetModTime losing metadata (eg X-Object-Manifest)
        -   This could have caused data loss for files > 5GB in size
    -   Use ContentType from Object to avoid lookups in listings
    -   One Drive
    -   disable server side copy as it seems to be broken at Microsoft
-   v1.24 - 2015-11-07
    -   New features
    -   Add support for Microsoft One Drive
    -   Add --no-check-certificate option to disable server certificate
        verification
    -   Add async readahead buffer for faster transfer of big files
    -   Fixes
    -   Allow spaces in remotes and check remote names for validity at
        creation time
    -   Allow '&' and disallow ':' in Windows filenames.
    -   Swift
    -   Ignore directory marker objects where appropriate - allows
        working with Hubic
    -   Don't delete the container if fs wasn't at root
    -   S3
    -   Don't delete the bucket if fs wasn't at root
    -   Google Cloud Storage
    -   Don't delete the bucket if fs wasn't at root
-   v1.23 - 2015-10-03
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
    -   Stop chunked operations logging "Failed to read info: Object Not
        Found"
    -   Use Content-Length on uploads for extra reliability
-   v1.22 - 2015-09-28
    -   Implement rsync like include and exclude flags
    -   swift
    -   Support files > 5GB - thanks Sergey Tolmachev
-   v1.21 - 2015-09-22
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
-   v1.20 - 2015-09-15
    -   New features
    -   Amazon Drive support
    -   Oauth support redone - fix many bugs and improve usability
        -   Use "golang.org/x/oauth2" as oauth libary of choice
        -   Improve oauth usability for smoother initial signup
        -   drive, googlecloudstorage: optionally use auto config for
            the oauth token
    -   Implement --dump-headers and --dump-bodies debug flags
    -   Show multiple matched commands if abbreviation too short
    -   Implement server side move where possible
    -   local
    -   Always use UNC paths internally on Windows - fixes a lot of bugs
    -   dropbox
    -   force use of our custom transport which makes timeouts work
    -   Thanks to Klaus Post for lots of help with this release
-   v1.19 - 2015-08-28
    -   New features
    -   Server side copies for s3/swift/drive/dropbox/gcs
    -   Move command - uses server side copies if it can
    -   Implement --retries flag - tries 3 times by default
    -   Build for plan9/amd64 and solaris/amd64 too
    -   Fixes
    -   Make a current version download with a fixed URL for scripting
    -   Ignore rmdir in limited fs rather than throwing error
    -   dropbox
    -   Increase chunk size to improve upload speeds massively
    -   Issue an error message when trying to upload bad file name
-   v1.18 - 2015-08-17
    -   drive
    -   Add --drive-use-trash flag so rclone trashes instead of deletes
    -   Add "Forbidden to download" message for files with no
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
-   v1.17 - 2015-06-14
    -   dropbox: fix case insensitivity issues - thanks Leonid Shalupov
-   v1.16 - 2015-06-09
    -   Fix uploading big files which was causing timeouts or panics
    -   Don't check md5sum after download with --size-only
-   v1.15 - 2015-06-06
    -   Add --checksum flag to only discard transfers by MD5SUM - thanks
        Alex Couper
    -   Implement --size-only flag to sync on size not checksum &
        modtime
    -   Expand docs and remove duplicated information
    -   Document rclone's limitations with directories
    -   dropbox: update docs about case insensitivity
-   v1.14 - 2015-05-21
    -   local: fix encoding of non utf-8 file names - fixes a duplicate
        file problem
    -   drive: docs about rate limiting
    -   google cloud storage: Fix compile after API change in
        "google.golang.org/api/storage/v1"
-   v1.13 - 2015-05-10
    -   Revise documentation (especially sync)
    -   Implement --timeout and --conntimeout
    -   s3: ignore etags from multipart uploads which aren't md5sums
-   v1.12 - 2015-03-15
    -   drive: Use chunked upload for files above a certain size
    -   drive: add --drive-chunk-size and --drive-upload-cutoff
        parameters
    -   drive: switch to insert from update when a failed copy deletes
        the upload
    -   core: Log duplicate files if they are detected
-   v1.11 - 2015-03-04
    -   swift: add region parameter
    -   drive: fix crash on failed to update remote mtime
    -   In remote paths, change native directory separators to /
    -   Add synchronization to ls/lsl/lsd output to stop corruptions
    -   Ensure all stats/log messages to go stderr
    -   Add --log-file flag to log everything (including panics) to file
    -   Make it possible to disable stats printing with --stats=0
    -   Implement --bwlimit to limit data transfer bandwidth
-   v1.10 - 2015-02-12
    -   s3: list an unlimited number of items
    -   Fix getting stuck in the configurator
-   v1.09 - 2015-02-07
    -   windows: Stop drive letters (eg C:) getting mixed up with
        remotes (eg drive:)
    -   local: Fix directory separators on Windows
    -   drive: fix rate limit exceeded errors
-   v1.08 - 2015-02-04
    -   drive: fix subdirectory listing to not list entire drive
    -   drive: Fix SetModTime
    -   dropbox: adapt code to recent library changes
-   v1.07 - 2014-12-23
    -   google cloud storage: fix memory leak
-   v1.06 - 2014-12-12
    -   Fix "Couldn't find home directory" on OSX
    -   swift: Add tenant parameter
    -   Use new location of Google API packages
-   v1.05 - 2014-08-09
    -   Improved tests and consequently lots of minor fixes
    -   core: Fix race detected by go race detector
    -   core: Fixes after running errcheck
    -   drive: reset root directory on Rmdir and Purge
    -   fs: Document that Purger returns error on empty directory, test
        and fix
    -   google cloud storage: fix ListDir on subdirectory
    -   google cloud storage: re-read metadata in SetModTime
    -   s3: make reading metadata more reliable to work around eventual
        consistency problems
    -   s3: strip trailing / from ListDir()
    -   swift: return directories without / in ListDir
-   v1.04 - 2014-07-21
    -   google cloud storage: Fix crash on Update
-   v1.03 - 2014-07-20
    -   swift, s3, dropbox: fix updated files being marked as corrupted
    -   Make compile with go 1.1 again
-   v1.02 - 2014-07-19
    -   Implement Dropbox remote
    -   Implement Google Cloud Storage remote
    -   Verify Md5sums and Sizes after copies
    -   Remove times from "ls" command - lists sizes only
    -   Add add "lsl" - lists times and sizes
    -   Add "md5sum" command
-   v1.01 - 2014-07-04
    -   drive: fix transfer of big files using up lots of memory
-   v1.00 - 2014-07-03
    -   drive: fix whole second dates
-   v0.99 - 2014-06-26
    -   Fix --dry-run not working
    -   Make compatible with go 1.1
-   v0.98 - 2014-05-30
    -   s3: Treat missing Content-Length as 0 for some ceph
        installations
    -   rclonetest: add file with a space in
-   v0.97 - 2014-05-05
    -   Implement copying of single files
    -   s3 & swift: support paths inside containers/buckets
-   v0.96 - 2014-04-24
    -   drive: Fix multiple files of same name being created
    -   drive: Use o.Update and fs.Put to optimise transfers
    -   Add version number, -V and --version
-   v0.95 - 2014-03-28
    -   rclone.org: website, docs and graphics
    -   drive: fix path parsing
-   v0.94 - 2014-03-27
    -   Change remote format one last time
    -   GNU style flags
-   v0.93 - 2014-03-16
    -   drive: store token in config file
    -   cross compile other versions
    -   set strict permissions on config file
-   v0.92 - 2014-03-15
    -   Config fixes and --config option
-   v0.91 - 2014-03-15
    -   Make config file
-   v0.90 - 2013-06-27
    -   Project named rclone
-   v0.00 - 2012-11-18
    -   Project started


Bugs and Limitations

Empty directories are left behind / not created

With remotes that have a concept of directory, eg Local and Drive, empty
directories may be left behind, or not created when one was expected.

This is because rclone doesn't have a concept of a directory - it only
works on objects. Most of the object storage systems can't actually
store a directory so there is nowhere for rclone to store anything about
directories.

You can work round this to some extent with thepurge command which will
delete everything under the path, INLUDING empty directories.

This may be fixed at some point in Issue #100

Directory timestamps aren't preserved

For the same reason as the above, rclone doesn't have a concept of a
directory - it only works on objects, therefore it can't preserve the
timestamps of directories.


Frequently Asked Questions

Do all cloud storage systems support all rclone commands

Yes they do. All the rclone commands (eg sync, copy etc) will work on
all the remote storage systems.

Can I copy the config from one machine to another

Sure! Rclone stores all of its config in a single file. If you want to
find this file, the simplest way is to run rclone -h and look at the
help for the --config flag which will tell you where it is.

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

Why doesn't rclone support partial transfers / binary diffs like rsync?

Rclone stores each file you transfer as a native object on the remote
cloud storage system. This means that you can see the files you upload
as expected using alternative access methods (eg using the Google Drive
web interface). There is a 1:1 mapping between files on your hard disk
and objects created in the cloud storage system.

Cloud storage systems (at least none I've come across yet) don't support
partially uploading an object. You can't take an existing object, and
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

Yes. rclone will use the environment variables HTTP_PROXY, HTTPS_PROXY
and NO_PROXY, similar to cURL and other programs.

HTTPS_PROXY takes precedence over HTTP_PROXY for https requests.

The environment values may be either a complete URL or a "host[:port]",
in which case the "http" scheme is assumed.

The NO_PROXY allows you to disable the proxy for specific hosts. Hosts
must be comma separated, and can contain domains or parts. For instance
"foo.com" also matches "bar.foo.com".

Rclone gives x509: failed to load system roots and no roots provided error

This means that rclone can't file the SSL root certificates. Likely you
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

Note that you may need to add the --insecure option to the curl command
line if it doesn't work without.

    curl --insecure -o /etc/ssl/certs/ca-certificates.crt https://raw.githubusercontent.com/bagder/ca-bundle/master/ca-bundle.crt

Rclone gives Failed to load config file: function not implemented error

Likely this means that you are running rclone on Linux version not
supported by the go runtime, ie earlier than version 2.6.23.

See the system requirements section in the go install docs for full
details.

All my uploaded docx/xlsx/pptx files appear as archive/zip

This is caused by uploading these files from a Windows computer which
hasn't got the Microsoft Office suite installed. The easiest way to fix
is to install the Word viewer and the Microsoft Office Compatibility
Pack for Word, Excel, and PowerPoint 2007 and later versions' file
formats


License

This is free software under the terms of MIT the license (check the
COPYING file included with the source code).

    Copyright (C) 2012 by Nick Craig-Wood http://www.craig-wood.com/nick/

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
-   Leonid Shalupov leonid@shalupov.com
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



CONTACT THE RCLONE PROJECT


Forum

Forum for general discussions and questions:

-   https://forum.rclone.org


Gitub project

The project website is at:

-   https://github.com/ncw/rclone

There you can file bug reports, ask for help or contribute pull
requests.


Google+

Rclone has a Google+ page which announcements are posted to

-   Google+ page for general comments


Twitter

You can also follow me on twitter for rclone announcments

-   [@njcw](https://twitter.com/njcw)


Email

Or if all else fails or you want to ask something private or
confidential email Nick Craig-Wood
