---
title: "Documentation"
description: "Rclone Usage"
date: "2015-06-06"
---

Configure
---------

First you'll need to configure rclone.  As the object storage systems
have quite complicated authentication these are kept in a config file
`.rclone.conf` in your home directory by default.  (You can use the
`--config` option to choose a different config file.)

The easiest way to make the config is to run rclone with the config
option:

    rclone config

See the following for detailed instructions for

  * [Google drive](/drive/)
  * [Amazon S3](/s3/)
  * [Swift / Rackspace Cloudfiles / Memset Memstore](/swift/)
  * [Dropbox](/dropbox/)
  * [Google Cloud Storage](/googlecloudstorage/)
  * [Local filesystem](/local/)
  * [Amazon Drive](/amazonclouddrive/)
  * [Backblaze B2](/b2/)
  * [Hubic](/hubic/)
  * [Microsoft One Drive](/onedrive/)
  * [Yandex Disk](/yandex/)
  * [Crypt](/crypt/) - to encrypt other remotes

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

    rclone ls remote:path # lists a re
    rclone copy /local/path remote:path # copies /local/path to the remote
    rclone sync /local/path remote:path # syncs /local/path to the remote

The main rclone commands with most used first

* [rclone config](/commands/rclone_config/)	 - Enter an interactive configuration session.
* [rclone copy](/commands/rclone_copy/)	 - Copy files from source to dest, skipping already copied
* [rclone sync](/commands/rclone_sync/)	 - Make source and dest identical, modifying destination only.
* [rclone move](/commands/rclone_move/)	 - Move files from source to dest.
* [rclone delete](/commands/rclone_delete/)	 - Remove the contents of path.
* [rclone purge](/commands/rclone_purge/)	 - Remove the path and all of its contents.
* [rclone mkdir](/commands/rclone_mkdir/)	 - Make the path if it doesn't already exist.
* [rclone rmdir](/commands/rclone_rmdir/)	 - Remove the path.
* [rclone check](/commands/rclone_check/)	 - Checks the files in the source and destination match.
* [rclone ls](/commands/rclone_ls/)	 - List all the objects in the the path with size and path.
* [rclone lsd](/commands/rclone_lsd/)	 - List all directories/containers/buckets in the the path.
* [rclone lsl](/commands/rclone_lsl/)	 - List all the objects path with modification time, size and path.
* [rclone md5sum](/commands/rclone_md5sum/)	 - Produces an md5sum file for all the objects in the path.
* [rclone sha1sum](/commands/rclone_sha1sum/)	 - Produces an sha1sum file for all the objects in the path.
* [rclone size](/commands/rclone_size/)	 - Returns the total size and number of objects in remote:path.
* [rclone version](/commands/rclone_version/)	 - Show the version number.
* [rclone cleanup](/commands/rclone_cleanup/)	 - Clean up the remote if possible
* [rclone dedupe](/commands/rclone_dedupe/)	 - Interactively find duplicate files delete/rename them.

See the [commands index](/commands/) for the full list.

Copying single files
--------------------

rclone normally syncs or copies directories.  However if the source
remote points to a file, rclone will just copy that file.  The
destination remote must point to a directory - rclone will give the
error `Failed to create file system for "remote:file": is a file not a
directory` if it isn't.

For example, suppose you have a remote with a file in called
`test.jpg`, then you could copy just that file like this

    rclone copy remote:test.jpg /tmp/download

The file `test.jpg` will be placed inside `/tmp/download`.

This is equivalent to specifying

    rclone copy --no-traverse --files-from /tmp/files remote: /tmp/download

Where `/tmp/files` contains the single line

    test.jpg

It is recommended to use `copy` when copying single files not `sync`.
They have pretty much the same effect but `copy` will use a lot less
memory.

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
(see [#464](https://github.com/ncw/rclone/issues/464) for why), eg

    rclone copy E:\ remote:backup

Server Side Copy
----------------

Drive, S3, Dropbox, Swift and Google Cloud Storage support server side
copy.

This means if you want to copy one folder to another then rclone won't
download all the files and re-upload them; it will instruct the server
to copy them in place.

Eg

    rclone copy s3:oldbucket s3:newbucket

Will copy the contents of `oldbucket` to `newbucket` without
downloading and re-uploading.

Remotes which don't support server side copy (eg local) **will**
download and re-upload in this case.

Server side copies are used with `sync` and `copy` and will be
identified in the log when using the `-v` flag.

Server side copies will only be attempted if the remote names are the
same.

This can be used when scripting to make aged backups efficiently, eg

    rclone sync remote:current-backup remote:previous-backup
    rclone sync /path/to/files remote:current-backup

Options
-------

Rclone has a number of options to control its behaviour.

Options which use TIME use the go time parser.  A duration string is a
possibly signed sequence of decimal numbers, each with optional
fraction and a unit suffix, such as "300ms", "-1.5h" or "2h45m". Valid
time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

Options which use SIZE use kByte by default.  However a suffix of `b`
for bytes, `k` for kBytes, `M` for MBytes and `G` for GBytes may be
used.  These are the binary units, eg 1, 2\*\*10, 2\*\*20, 2\*\*30
respectively.

### --bwlimit=SIZE ###

Bandwidth limit in kBytes/s, or use suffix b|k|M|G.  The default is `0`
which means to not limit bandwidth.

For example to limit bandwidth usage to 10 MBytes/s use `--bwlimit 10M`

This only limits the bandwidth of the data transfer, it doesn't limit
the bandwith of the directory listings etc.

Note that the units are Bytes/s not Bits/s.  Typically connections are
measured in Bits/s - to convert divide by 8.  For example let's say
you have a 10 Mbit/s connection and you wish rclone to use half of it
- 5 Mbit/s.  This is 5/8 = 0.625MByte/s so you would use a `--bwlimit
0.625M` parameter for rclone.

### --checkers=N ###

The number of checkers to run in parallel.  Checkers do the equality
checking of files during a sync.  For some storage systems (eg s3,
swift, dropbox) this can take a significant amount of time so they are
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
section](/overview/).

Eg `rclone --checksum sync s3:/bucket swift:/bucket` would run much
quicker than without the `--checksum` flag.

When using this flag, rclone won't update mtimes of remote files if
they are incorrect as it would normally.

### --config=CONFIG_FILE ###

Specify the location of the rclone config file.  Normally this is in
your home directory as a file called `.rclone.conf`.  If you run
`rclone -h` and look at the help for the `--config` option you will
see where the default location is for you.  Use this flag to override
the config location, eg `rclone --config=".myconfig" .config`.

### --contimeout=TIME ###

Set the connection timeout. This should be in go time format which
looks like `5s` for 5 seconds, `10m` for 10 minutes, or `3h30m`.

The connection timeout is the amount of time rclone will wait for a
connection to go through to a remote object storage system.  It is
`1m` by default.

### --dedupe-mode MODE ###

Mode to run dedupe command in.  One of `interactive`, `skip`, `first`, `newest`, `oldest`, `rename`.  The default is `interactive`.  See the dedupe command for more information as to what these options mean.

### -n, --dry-run ###

Do a trial run with no permanent changes.  Use this to see what rclone
would do without actually doing it.  Useful when setting up the `sync`
command which deletes files in the destination.

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

This can be useful for transferring files to and from onedrive which
occasionally misreports the size of image files (see
[#399](https://github.com/ncw/rclone/issues/399) for more info).

### -I, --ignore-times ###

Using this option will cause rclone to unconditionally upload all
files regardless of the state of files on the destination.

Normally rclone would skip any files that have the same
modification time and are the same size (or have the same checksum if
using `--checksum`).

### --log-file=FILE ###

Log all of rclone's output to FILE.  This is not active by default.
This can be useful for tracking down problems with syncs in
combination with the `-v` flag.  See the Logging section for more
info.

### --low-level-retries NUMBER ###

This controls the number of low level retries rclone does.

A low level retry is used to retry a failing operation - typically one
HTTP request.  This might be uploading a chunk of a big file for
example.  You will see low level retries in the log with the `-v`
flag.

This shouldn't need to be changed from the default in normal
operations, however if you get a lot of low level retries you may wish
to reduce the value so rclone moves on to a high level retry (see the
`--retries` flag) quicker.

Disable low level retries with `--low-level-retries 1`.

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

### --modify-window=TIME ###

When checking whether a file has been modified, this is the maximum
allowed time difference that a file can have and still be considered
equivalent.

The default is `1ns` unless this is overridden by a remote.  For
example OS X only stores modification times to the nearest second so
if you are reading and writing to an OS X filing system this will be
`1s` by default.

This command line flag allows you to override that computed default.

### --no-gzip-encoding ###

Don't set `Accept-Encoding: gzip`.  This means that rclone won't ask
the server for compressed files automatically. Useful if you've set
the server to return files with `Content-Encoding: gzip` but you
uploaded compressed files.

There is no need to set this in normal operation, and doing so will
decrease the network transfer efficiency of rclone.

### --no-update-modtime ###

When using this flag, rclone won't update modification times of remote
files if they are incorrect as it would normally.

This can be used if the remote is being synced with another tool also
(eg the Google Drive client).

### -q, --quiet ###

Normally rclone outputs stats and a completion message.  If you set
this flag it will make as little output as possible.

### --retries int ###

Retry the entire sync if it fails this many times it fails (default 3).

Some remotes can be unreliable and a few retries helps pick up the
files which didn't get transferred because of errors.

Disable retries with `--retries 1`.

### --size-only ###

Normally rclone will look at modification time and size of files to
see if they are equal.  If you set this flag then rclone will check
only the size.

This can be useful transferring files from dropbox which have been
modified by the desktop sync client which doesn't set checksums of
modification times in the same way as rclone.

### --stats=TIME ###

Commands which transfer data (`sync`, `copy`, `copyto`, `move`,
`moveto`) will print data transfer stats at regular intervals to show
their progress.

This sets the interval.

The default is `1m`. Use 0 to disable.

If you set the stats interval then all command can show stats.  This
can be useful when running other commands, `check` or `mount` for
example.

### --stats-unit=bits|bytes ###

By default data transfer rates will be printed in bytes/second.

This option allows the data rate to be printed in bits/second.

Data transfer volume will still be reported in bytes.

The rate is reported as a binary unit, not SI unit. So 1 Mbit/s
equals 1,048,576 bits/s and not 1,000,000 bits/s.

The default is `bytes`.

### --track-renames ###

By default renames will be performed as a delete and a copy, which is ineffective for remotes that supports server-side move operations.

Setting this option will track renames during `sync` and perform renaming server-side. If the destination does not support server-side move, `sync` will fall back to the default behaviour and log an `error` to the console.  

### --delete-(before,during,after) ###

This option allows you to specify when files on your destination are
deleted when you sync folders.

Specifying the value `--delete-before` will delete all files present
on the destination, but not on the source *before* starting the
transfer of any new or updated files.  This uses extra memory as it
has to store the source listing before proceeding.

Specifying `--delete-during` (default value) will delete files while
checking and uploading files. This is usually the fastest option.
Currently this works the same as `--delete-after` but it may change in
the future.

Specifying `--delete-after` will delay deletion of files until all new/updated
files have been successfully transfered.

### --timeout=TIME ###

This sets the IO idle timeout.  If a transfer has started but then
becomes idle for this long it is considered broken and disconnected.

The default is `5m`.  Set to 0 to disable.

### --transfers=N ###

The number of file transfers to run in parallel.  It can sometimes be
useful to set this to a smaller number if the remote is giving a lot
of timeouts or bigger if you have lots of bandwidth and a fast remote.

The default is to run 4 file transfers in parallel.

### -u, --update ###

This forces rclone to skip any files which exist on the destination
and have a modified time that is newer than the source file.

If an existing destination file has a modification time equal (within
the computed modify window precision) to the source file's, it will be
updated if the sizes are different.

On remotes which don't support mod time directly the time checked will
be the uploaded time.  This means that if uploading to one of these
remoes, rclone will skip any files which exist on the destination and
have an uploaded time that is newer than the modification time of the
source file.

This can be useful when transferring to a remote which doesn't support
mod times directly as it is more accurate than a `--size-only` check
and faster than using `--checksum`.

### -v, --verbose ###

If you set this flag, rclone will become very verbose telling you
about every file it considers and transfers.

Very useful for debugging.

### -V, --version ###

Prints the version number

Configuration Encryption
------------------------
Your configuration file contains information for logging in to 
your cloud services. This means that you should keep your 
`.rclone.conf` file in a secure location.

If you are in an environment where that isn't possible, you can
add a password to your configuration. This means that you will
have to enter the password every time you start rclone.

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
you will now be asked for the password. In the same menu you can 
change the password or completely remove encryption from your
configuration.

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
password and set it in the envonment variable.

If you are running rclone inside a script, you might want to disable 
password prompts. To do that, pass the parameter 
`--ask-password=false` to rclone. This will make rclone fail instead
of asking for a password if `RCLONE_CONFIG_PASS` doesn't contain
a valid password.


Developer options
-----------------

These options are useful when developing or debugging rclone.  There
are also some more remote specific options which aren't documented
here which are used for testing.  These start with remote name eg
`--drive-test-option` - see the docs for the remote in question.

### --cpuprofile=FILE ###

Write CPU profile to file.  This can be analysed with `go tool pprof`.

### --dump-auth ###

Dump HTTP headers - will contain sensitive info such as
`Authorization:` headers - use `--dump-headers` to dump without
`Authorization:` headers.  Can be very verbose.  Useful for debugging
only.

### --dump-bodies ###

Dump HTTP headers and bodies - may contain sensitive info.  Can be
very verbose.  Useful for debugging only.

### --dump-filters ###

Dump the filters to the output.  Useful to see exactly what include
and exclude options are filtering on.

### --dump-headers ###

Dump HTTP headers with `Authorization:` lines removed. May still
contain sensitive info.  Can be very verbose.  Useful for debugging
only.

Use `--dump-auth` if you do want the `Authorization:` headers.

### --memprofile=FILE ###

Write memory profile to file. This can be analysed with `go tool pprof`.

### --no-check-certificate=true/false ###

`--no-check-certificate` controls whether a client verifies the
server's certificate chain and host name.
If `--no-check-certificate` is true, TLS accepts any certificate
presented by the server and any host name in that certificate.
In this mode, TLS is susceptible to man-in-the-middle attacks.

This option defaults to `false`.

**This should be used only for testing.**

### --no-traverse ###

The `--no-traverse` flag controls whether the destination file system
is traversed when using the `copy` or `move` commands.

If you are only copying a small number of files and/or have a large
number of files on the destination then `--no-traverse` will stop
rclone listing the destination and save time.

However if you are copying a large number of files, escpecially if you
are doing a copy where lots of the files haven't changed and won't
need copying then you shouldn't use `--no-traverse`.

It can also be used to reduce the memory usage of rclone when copying
- `rclone --no-traverse copy src dst` won't load either the source or
destination listings into memory so will use the minimum amount of
memory.

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
  * `--min-size`
  * `--max-size`
  * `--min-age`
  * `--max-age`
  * `--dump-filters`

See the [filtering section](/filtering/).

Logging
-------

rclone has 3 levels of logging, `Error`, `Info` and `Debug`.

By default rclone logs `Error` and `Info` to standard error and `Debug`
to standard output.  This means you can redirect standard output and
standard error to different places.

By default rclone will produce `Error` and `Info` level messages.

If you use the `-q` flag, rclone will only produce `Error` messages.

If you use the `-v` flag, rclone will produce `Error`, `Info` and
`Debug` messages.

If you use the `--log-file=FILE` option, rclone will redirect `Error`,
`Info` and `Debug` messages along with standard error to FILE.

Exit Code
---------

If any errors occurred during the command, rclone with an exit code of
`1`.  This allows scripts to detect when rclone operations have failed.

During the startup phase rclone will exit immediately if an error is
detected in the configuration.  There will always be a log message
immediately before exiting.

When rclone is running it will accumulate errors as it goes along, and
only exit with an non-zero exit code if (after retries) there were no
transfers with errors remaining.  For every error counted there will
be a high priority log message (visibile with `-q`) showing the
message and which file caused the problem. A high priority message is
also shown when starting a retry so the user can see that any previous
error messages may not be valid after the retry. If rclone has done a
retry it will log a high priority message if the retry was successful.
