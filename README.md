Rclone
======

[![Logo](http://rclone.org/img/rclone-120x120.png)](http://rclone.org/)

Sync files and directories to and from

  * Google Drive
  * Amazon S3
  * Openstack Swift / Rackspace cloud files / Memset Memstore
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

Download the relevant binary from

  * http://www.craig-wood.com/nick/pub/rclone/

Or alternatively if you have Go installed use

    go get github.com/ncw/rclone

and this will build the binary in `$GOPATH/bin`.

You can then modify the source and submit patches.

Configure
---------

First you'll need to configure rclone.  As the object storage systems
have quite complicated authentication these are kept in a config file
`.rclone.conf` in your home directory by default.  (You can use the
-config option to choose a different config file.)

The easiest way to make the config is to run rclone with the config
option, Eg

    rclone config

Usage
-----

Rclone syncs a directory tree from local to remote.

Its basic syntax is like this

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
MD5SUM.  Deletes any files that exist in source that don't
exist in destination. Since this can cause data loss, test
first with the -dry-run flag.

    rclone ls [remote:path]

List all the objects in the the path.

    rclone lsd [remote:path]

List all directoryes/objects/buckets in the the path.

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

General options:
  * `-config` Location of the config file
  * `-transfers=4`: Number of file transfers to run in parallel.
  * `-checkers=8`: Number of MD5SUM checkers to run in parallel.
  * `-dry-run=false`: Do a trial run with no permanent changes
  * `-modify-window=1ns`: Max time difference to be considered the same - this is automatically set usually
  * `-quiet=false`: Print as little stuff as possible
  * `-stats=1m0s`: Interval to print stats
  * `-verbose=false`: Print lots more stuff

Developer options:
  * `-cpuprofile=""`: Write cpu profile to file

Local Filesystem
----------------

Paths are specified as normal filesystem paths, so

    rclone sync /home/source /tmp/destination

Will sync source to destination

Swift / Rackspace cloudfiles / Memset Memstore
----------------------------------------------

Paths are specified as remote:container (or remote: for the `lsd`
command.)

So to copy a local directory to a swift container called backup:

    rclone sync /home/source swift:backup

The modified time is stored as metadata on the object as
`X-Object-Meta-Mtime` as floating point since the epoch.

This is a defacto standard (used in the official python-swiftclient
amongst others) for storing the modification time (as read using
os.Stat) for an object.

Amazon S3
---------

Paths are specified as remote:bucket

So to copy a local directory to a s3 container called backup

    rclone sync /home/source s3:backup

The modified time is stored as metadata on the object as
`X-Amz-Meta-Mtime` as floating point since the epoch.

Google drive
------------

Paths are specified as drive:path  Drive paths may be as deep as required.

The initial setup for drive involves getting a token from Google drive
which you need to do in your browser.  The `rclone config` walks you
through it.

To copy a local directory to a drive directory called backup

    rclone copy /home/source drv:backup

Google drive stores modification times accurate to 1 ms.

License
-------

This is free software under the terms of MIT the license (check the
COPYING file included in this package).

Bugs
----

  * Doesn't sync individual files yet, only directories.
  * Drive: Sometimes get: Failed to copy: Upload failed: googleapi: Error 403: Rate Limit Exceeded
    * quota is 100.0 requests/second/user
  * Empty directories left behind with Local and Drive
    * eg purging a local directory with subdirectories doesn't work

Contact and support
-------------------

The project website is at:

  * https://github.com/ncw/rclone

There you can file bug reports, ask for help or contribute patches.

Authors
-------

  * Nick Craig-Wood <nick@craig-wood.com>

Contributors
------------

  * Your name goes here!
