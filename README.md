% rclone(1) User Manual
% Nick Craig-Wood
% Jul 7, 2014

Rclone
======

[![Logo](http://rclone.org/img/rclone-120x120.png)](http://rclone.org/)

Rclone is a command line program to sync files and directories to and from

  * Google Drive
  * Amazon S3
  * Openstack Swift / Rackspace cloud files / Memset Memstore
  * Dropbox
  * Google Cloud Storage
  * The local filesystem

Features

  * MD5SUMs checked at all times for file integrity
  * Timestamps preserved on files
  * Partial syncs supported on a whole file basis
  * Copy mode to just copy new/changed files
  * Sync mode to make a directory identical
  * Check mode to check all MD5SUMs
  * Can sync to and from network, eg two different Drive accounts

See the Home page for more documentation and configuration walkthroughs.

  * http://rclone.org/

Install
-------

Rclone is a Go program and comes as a single binary file.

Download the binary for your OS from

  * http://rclone.org/downloads/

Or alternatively if you have Go installed use

    go install github.com/ncw/rclone

and this will build the binary in `$GOPATH/bin`.

Configure
---------

First you'll need to configure rclone.  As the object storage systems
have quite complicated authentication these are kept in a config file
`.rclone.conf` in your home directory by default.  (You can use the
`--config` option to choose a different config file.)

The easiest way to make the config is to run rclone with the config
option, Eg

    rclone config

Usage
-----

Rclone syncs a directory tree from local to remote.

Its basic syntax is

    Syntax: [options] subcommand <parameters> <parameters...>

See below for how to specify the source and destination paths.

Subcommands
-----------

    rclone copy source:path dest:path

Copy the source to the destination.  Doesn't transfer
unchanged files, testing first by modification time then by
MD5SUM.  Doesn't delete files from the destination.

    rclone sync source:path dest:path

Sync the source to the destination.  Doesn't transfer
unchanged files, testing first by modification time then by
size.  Deletes any files that exist in source that don't
exist in destination. Since this can cause data loss, test
first with the `--dry-run` flag.

    rclone ls [remote:path]

List all the objects in the the path with size and path.

    rclone lsd [remote:path]

List all directories/containers/buckets in the the path.

    rclone lsl [remote:path]

List all the objects in the the path with modification time, size and path.

    rclone md5sum [remote:path]

Produces an md5sum file for all the objects in the path.  This
is in the same format as the standard md5sum tool produces.

    rclone mkdir remote:path

Make the path if it doesn't already exist

    rclone rmdir remote:path

Remove the path.  Note that you can't remove a path with
objects in it, use purge for that.

    rclone purge remote:path

Remove the path and all of its contents.

    rclone check source:path dest:path

Checks the files in the source and destination match.  It
compares sizes and MD5SUMs and prints a report of files which
don't match.  It doesn't alter the source or destination.

    rclone config 

Enter an interactive configuration session.

    rclone help 

This help.

General options:

```
      --bwlimit=0: Bandwidth limit in kBytes/s, or use suffix K|M|G
      --checkers=8: Number of checkers to run in parallel.
      --config="~/.rclone.conf": Config file.
  -n, --dry-run=false: Do a trial run with no permanent changes
      --log-file="": Log everything to this file
      --modify-window=1ns: Max time diff to be considered the same
  -q, --quiet=false: Print as little stuff as possible
      --stats=1m0s: Interval to print stats (0 to disable)
      --transfers=4: Number of file transfers to run in parallel.
  -v, --verbose=false: Print lots more stuff
  -V, --version=false: Print the version number
```

Developer options:

```
      --cpuprofile="": Write cpu profile to file
```

Local Filesystem
----------------

Paths are specified as normal filesystem paths, so

    rclone sync /home/source /tmp/destination

Will sync `/home/source` to `/tmp/destination`

Swift / Rackspace cloudfiles / Memset Memstore
----------------------------------------------

Paths are specified as remote:container (or remote: for the `lsd`
command.)  You may put subdirectories in too, eg
`remote:container/path/to/dir`.

So to copy a local directory to a swift container called backup:

    rclone sync /home/source swift:backup

The modified time is stored as metadata on the object as
`X-Object-Meta-Mtime` as floating point since the epoch.

This is a defacto standard (used in the official python-swiftclient
amongst others) for storing the modification time (as read using
os.Stat) for an object.

Amazon S3
---------

Paths are specified as remote:bucket.  You may put subdirectories in
too, eg `remote:bucket/path/to/dir`.

So to copy a local directory to a s3 container called backup

    rclone sync /home/source s3:backup

The modified time is stored as metadata on the object as
`X-Amz-Meta-Mtime` as floating point since the epoch.

Google drive
------------

Paths are specified as remote:path  Drive paths may be as deep as required.

The initial setup for drive involves getting a token from Google drive
which you need to do in your browser.  `rclone config` walks you
through it.

To copy a local directory to a drive directory called backup

    rclone copy /home/source remote:backup

Google drive stores modification times accurate to 1 ms natively.

Dropbox
-------

Paths are specified as remote:path  Dropbox paths may be as deep as required.

The initial setup for dropbox involves getting a token from Dropbox
which you need to do in your browser.  `rclone config` walks you
through it.

To copy a local directory to a drive directory called backup

    rclone copy /home/source dropbox:backup

Md5sums and timestamps in RFC3339 format accurate to 1ns are stored in
a Dropbox datastore called "rclone".  Dropbox datastores are limited
to 100,000 rows so this is the maximum number of files rclone can
manage on Dropbox.

Google Cloud Storage
--------------------

Paths are specified as remote:path Google Cloud Storage paths may be
as deep as required.

The initial setup for Google Cloud Storage involves getting a token
from Google which you need to do in your browser.  `rclone config`
walks you through it.

To copy a local directory to a google cloud storage directory called backup

    rclone copy /home/source remote:backup

Google google cloud storage stores md5sums natively and rclone stores
modification times as metadata on the object, under the "mtime" key in
RFC3339 format accurate to 1ns.

Single file copies
------------------

Rclone can copy single files

    rclone src:path/to/file dst:path/dir

Or

    rclone src:path/to/file dst:path/to/file

Note that you can't rename the file if you are copying from one file to another.

License
-------

This is free software under the terms of MIT the license (check the
COPYING file included in this package).

Bugs
----

  * Empty directories left behind with Local and Drive
    * eg purging a local directory with subdirectories doesn't work

Changelog
---------
  * v1.11 - 2015-03-04
    * swift: add region parameter
    * drive: fix crash on failed to update remote mtime
    * In remote paths, change native directory separators to /
    * Add synchronization to ls/lsl/lsd output to stop corruptions
    * Ensure all stats/log messages to go stderr
    * Add --log-file flag to log everything (including panics) to file
    * Make it possible to disable stats printing with --stats=0
    * Implement --bwlimit to limit data transfer bandwidth
  * v1.10 - 2015-02-12
    * s3: list an unlimited number of items
    * Fix getting stuck in the configurator
  * v1.09 - 2015-02-07
    * windows: Stop drive letters (eg C:) getting mixed up with remotes (eg drive:)
    * local: Fix directory separators on Windows
    * drive: fix rate limit exceeded errors
  * v1.08 - 2015-02-04
    * drive: fix subdirectory listing to not list entire drive
    * drive: Fix SetModTime
    * dropbox: adapt code to recent library changes
  * v1.07 - 2014-12-23
    * google cloud storage: fix memory leak
  * v1.06 - 2014-12-12
    * Fix "Couldn't find home directory" on OSX
    * swift: Add tenant parameter
    * Use new location of Google API packages
  * v1.05 - 2014-08-09
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
  * v1.04 - 2014-07-21
    * google cloud storage: Fix crash on Update
  * v1.03 - 2014-07-20
    * swift, s3, dropbox: fix updated files being marked as corrupted
    * Make compile with go 1.1 again
  * v1.02 - 2014-07-19
    * Implement Dropbox remote
    * Implement Google Cloud Storage remote
    * Verify Md5sums and Sizes after copies
    * Remove times from "ls" command - lists sizes only
    * Add add "lsl" - lists times and sizes
    * Add "md5sum" command
  * v1.01 - 2014-07-04
    * drive: fix transfer of big files using up lots of memory
  * v1.00 - 2014-07-03
    * drive: fix whole second dates
  * v0.99 - 2014-06-26
    * Fix --dry-run not working
    * Make compatible with go 1.1
  * v0.98 - 2014-05-30
    * s3: Treat missing Content-Length as 0 for some ceph installations
    * rclonetest: add file with a space in
  * v0.97 - 2014-05-05
    * Implement copying of single files
    * s3 & swift: support paths inside containers/buckets
  * v0.96 - 2014-04-24
    * drive: Fix multiple files of same name being created
    * drive: Use o.Update and fs.Put to optimise transfers
    * Add version number, -V and --version
  * v0.95 - 2014-03-28
    * rclone.org: website, docs and graphics
    * drive: fix path parsing
  * v0.94 - 2014-03-27
    * Change remote format one last time
    * GNU style flags
  * v0.93 - 2014-03-16
    * drive: store token in config file
    * cross compile other versions
    * set strict permissions on config file
  * v0.92 - 2014-03-15
    * Config fixes and --config option
  * v0.91 - 2014-03-15
    * Make config file
  * v0.90 - 2013-06-27
    * Project named rclone
  * v0.00 - 2012-11-18
    * Project started


Contact and support
-------------------

The project website is at:

  * https://github.com/ncw/rclone

There you can file bug reports, ask for help or send pull requests.

Authors
-------

  * Nick Craig-Wood <nick@craig-wood.com>

Contributors
------------

  * Your name goes here!
