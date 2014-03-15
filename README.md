Rclone
======

Sync files and directories to and from

  * Openstack Swift / Rackspace cloud files / Memset Memstore
  * Amazon S3
  * Google Drive
  * The local filesystem

Features

  * MD5SUMs checked at all times for file integrity
  * Timestamps preserved on files
  * Partial syncs supported on a whole file basis
  * Copy mode to just copy new/changed files
  * Sync mode to make a directory identical
  * Check mode to check all MD5SUMs

Install
-------

Rclone is a Go program and comes as a single binary file.

Download the relevant binary from

  * http://www.craig-wood.com/nick/pub/rclone/

Or alternatively if you have Go installed use

    go get github.com/ncw/rclone

and this will build the binary in `$GOPATH/bin`.  You can then modify
the source and submit patches.

Configure
---------

First you'll need to configure rclone.  As the object storage systems
have quite complicated authentication these are kept in a config file
`.rclone.conf` in your home directory by default.  (You can use the
-config option to choose a different config file.)

The easiest way to make the config is to run rclone with the config
option, Eg

rclone config

Here is an example of making an s3 configuration

```
No remotes found - make a new one
n) New remote
q) Quit config
n/q> n
name> remote
What type of source is it?
Choose a number from below
 1) swift
 2) s3
 3) local
 4) drive
type> 2
AWS Access Key ID.
access_key_id> accesskey
AWS Secret Access Key (password). 
secret_access_key> secretaccesskey
Endpoint for S3 API.
Choose a number from below, or type in your own value
 * The default endpoint - a good choice if you are unsure.
 * US Region, Northern Virginia or Pacific Northwest.
 * Leave location constraint empty.
 1) https://s3.amazonaws.com/
 * US Region, Northern Virginia only.
 * Leave location constraint empty.
 2) https://s3-external-1.amazonaws.com
 * US West (Oregon) Region
 * Needs location constraint us-west-2.
 3) https://s3-us-west-2.amazonaws.com
[snip]
 * South America (Sao Paulo) Region
 * Needs location constraint sa-east-1.
 9) https://s3-sa-east-1.amazonaws.com
endpoint> 1
Location constraint - must be set to match the Endpoint.
Choose a number from below, or type in your own value
 * Empty for US Region, Northern Virginia or Pacific Northwest.
 1) 
 * US West (Oregon) Region.
 2) us-west-2
 * US West (Northern California) Region.
 3) us-west-1
[snip]
 * South America (Sao Paulo) Region.
 9) sa-east-1
location_constraint> 1
--------------------
[remote]
access_key_id = accesskey
secret_access_key = secretaccesskey
endpoint = https://s3.amazonaws.com/
location_constraint = 
--------------------
y) Yes this is OK
e) Edit this remote
d) Delete this remote
y/e/d> y
Current remotes:

Name                 Type
====                 ====
remote               s3

e) Edit existing remote
n) New remote
d) Delete remote
q) Quit config
e/n/d/q> q
```

This can now be used like this

```
rclone lsd remote:// - see all buckets/containers
rclone ls remote:// - list a bucket
rclone sync /home/local/directory remote://bucket
```

Usage
-----

Rclone syncs a directory tree from local to remote.

Its basic syntax is like this

    Syntax: [options] subcommand <parameters> <parameters...>

Each subcommand looks like this.  See below for how to specify the
source and destination paths.

rclone copy source://path dest://path

Copy the source to the destination.  Doesn't transfer
unchanged files, testing first by modification time then by
MD5SUM.  Doesn't delete files from the destination.

rclone sync source://path dest://path

Sync the source to the destination.  Doesn't transfer
unchanged files, testing first by modification time then by
MD5SUM.  Deletes any files that exist in source that don't
exist in destination. Since this can cause data loss, test
first with the -dry-run flag.

rclone ls [remote://path]

List all the objects in the the path.

rclone lsd [remote://path]

List all directoryes/objects/buckets in the the path.

rclone mkdir remote://path

Make the path if it doesn't already exist

rclone rmdir remote://path

Remove the path.  Note that you can't remove a path with
objects in it, use purge for that.

rclone purge remote://path

Remove the path and all of its contents.

rclone check source://path dest://path

Checks the files in the source and destination match.  It
compares sizes and MD5SUMs and prints a report of files which
don't match.  It doesn't alter the source or destination.

General options:
  -config Location of the config file
  -transfers=4: Number of file transfers to run in parallel.
  -checkers=8: Number of MD5SUM checkers to run in parallel.
  -dry-run=false: Do a trial run with no permanent changes
  -modify-window=1ns: Max time difference to be considered the same - this is automatically set usually
  -quiet=false: Print as little stuff as possible
  -stats=1m0s: Interval to print stats
  -verbose=false: Print lots more stuff

Developer options:
  -cpuprofile="": Write cpu profile to file

Local Filesystem
----------------

Paths are specified as normal filesystem paths, so

    rclone sync /home/source /tmp/destination

Will sync source to destination

Swift / Rackspace cloudfiles / Memset Memstore
----------------------------------------------

Paths are specified as remote://container or remote:// for the lsd
command.

So to copy a local directory to a swift container called backup

rclone sync /home/source swift://backup

The modified time is stored as metadata on the object as
'X-Object-Meta-Mtime' as floating point since the epoch.

This is a defacto standard (used in the official python-swiftclient
amongst others) for storing the modification time (as read using
os.Stat) for an object.

Amazon S3
---------

Paths are specified as remote://bucket

So to copy a local directory to a s3 container called backup

rclone sync /home/source s3://backup

The modified time is stored as metadata on the object as
"X-Amz-Meta-Mtime" as floating point since the epoch.

Google drive
------------

Paths are specified as drive://path  Drive paths may be as deep as required.

FIXME describe how to set up initially

So to copy a local directory to a drive directory called backup

rclone sync /home/source s3://backup

License
-------

This is free software under the terms of MIT the license (check the
COPYING file included in this package).

Bugs
----

Save the google drive auth in this config file too!

Describe how to do the google auth.

Contact and support
-------------------

The project website is at:

- https://github.com/ncw/rclone

There you can file bug reports, ask for help or contribute patches.

Authors
-------

- Nick Craig-Wood <nick@craig-wood.com>

Contributors
------------

- Your name goes here!
