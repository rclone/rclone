rclone @ allows Linux, FreeBSD, macOS and Windows to
mount any of Rclone's cloud storage systems as a file system with
FUSE.

First set up your remote using `rclone config`.  Check it works with `rclone ls` etc.

On Linux and macOS, you can run mount in either foreground or background (aka
daemon) mode. Mount runs in foreground mode by default. Use the `--daemon` flag
to force background mode. On Windows you can run mount in foreground only,
the flag is ignored.

In background mode rclone acts as a generic Unix mount program: the main
program starts, spawns background rclone process to setup and maintain the
mount, waits until success or timeout and exits with appropriate code
(killing the child process if it fails).

On Linux/macOS/FreeBSD start the mount like this, where `/path/to/local/mount`
is an **empty** **existing** directory:

    rclone @ remote:path/to/files /path/to/local/mount

On Windows you can start a mount in different ways. See [below](#mounting-modes-on-windows)
for details. If foreground mount is used interactively from a console window,
rclone will serve the mount and occupy the console so another window should be
used to work with the mount until rclone is interrupted e.g. by pressing Ctrl-C.

The following examples will mount to an automatically assigned drive,
to specific drive letter `X:`, to path `C:\path\parent\mount`
(where parent directory or drive must exist, and mount must **not** exist,
and is not supported when [mounting as a network drive](#mounting-modes-on-windows)), and
the last example will mount as network share `\\cloud\remote` and map it to an
automatically assigned drive:

    rclone @ remote:path/to/files *
    rclone @ remote:path/to/files X:
    rclone @ remote:path/to/files C:\path\parent\mount
    rclone @ remote:path/to/files \\cloud\remote

When the program ends while in foreground mode, either via Ctrl+C or receiving
a SIGINT or SIGTERM signal, the mount should be automatically stopped.

When running in background mode the user will have to stop the mount manually:

    # Linux
    fusermount -u /path/to/local/mount
    # OS X
    umount /path/to/local/mount

The umount operation can fail, for example when the mountpoint is busy.
When that happens, it is the user's responsibility to stop the mount manually.

The size of the mounted file system will be set according to information retrieved
from the remote, the same as returned by the [rclone about](https://rclone.org/commands/rclone_about/)
command. Remotes with unlimited storage may report the used size only,
then an additional 1 PiB of free space is assumed. If the remote does not
[support](https://rclone.org/overview/#optional-features) the about feature
at all, then 1 PiB is set as both the total and the free size.

### Installing on Windows

To run rclone @ on Windows, you will need to
download and install [WinFsp](http://www.secfs.net/winfsp/).

[WinFsp](https://github.com/winfsp/winfsp) is an open-source
Windows File System Proxy which makes it easy to write user space file
systems for Windows.  It provides a FUSE emulation layer which rclone
uses combination with [cgofuse](https://github.com/winfsp/cgofuse).
Both of these packages are by Bill Zissimopoulos who was very helpful
during the implementation of rclone @ for Windows.

#### Mounting modes on windows

Unlike other operating systems, Microsoft Windows provides a different filesystem
type for network and fixed drives. It optimises access on the assumption fixed
disk drives are fast and reliable, while network drives have relatively high latency
and less reliability. Some settings can also be differentiated between the two types,
for example that Windows Explorer should just display icons and not create preview
thumbnails for image and video files on network drives.

In most cases, rclone will mount the remote as a normal, fixed disk drive by default.
However, you can also choose to mount it as a remote network drive, often described
as a network share. If you mount an rclone remote using the default, fixed drive mode
and experience unexpected program errors, freezes or other issues, consider mounting
as a network drive instead.

When mounting as a fixed disk drive you can either mount to an unused drive letter,
or to a path representing a **nonexistent** subdirectory of an **existing** parent
directory or drive. Using the special value `*` will tell rclone to
automatically assign the next available drive letter, starting with Z: and moving backward.
Examples:

    rclone @ remote:path/to/files *
    rclone @ remote:path/to/files X:
    rclone @ remote:path/to/files C:\path\parent\mount
    rclone @ remote:path/to/files X:

Option `--volname` can be used to set a custom volume name for the mounted
file system. The default is to use the remote name and path.

To mount as network drive, you can add option `--network-mode`
to your @ command. Mounting to a directory path is not supported in
this mode, it is a limitation Windows imposes on junctions, so the remote must always
be mounted to a drive letter.

    rclone @ remote:path/to/files X: --network-mode

A volume name specified with `--volname` will be used to create the network share path.
A complete UNC path, such as `\\cloud\remote`, optionally with path
`\\cloud\remote\madeup\path`, will be used as is. Any other
string will be used as the share part, after a default prefix `\\server\`.
If no volume name is specified then `\\server\share` will be used.
You must make sure the volume name is unique when you are mounting more than one drive,
or else the mount command will fail. The share name will treated as the volume label for
the mapped drive, shown in Windows Explorer etc, while the complete
`\\server\share` will be reported as the remote UNC path by
`net use` etc, just like a normal network drive mapping.

If you specify a full network share UNC path with `--volname`, this will implicitly
set the `--network-mode` option, so the following two examples have same result:

    rclone @ remote:path/to/files X: --network-mode
    rclone @ remote:path/to/files X: --volname \\server\share

You may also specify the network share UNC path as the mountpoint itself. Then rclone
will automatically assign a drive letter, same as with `*` and use that as
mountpoint, and instead use the UNC path specified as the volume name, as if it were
specified with the `--volname` option. This will also implicitly set
the `--network-mode` option. This means the following two examples have same result:

    rclone @ remote:path/to/files \\cloud\remote
    rclone @ remote:path/to/files * --volname \\cloud\remote

There is yet another way to enable network mode, and to set the share path,
and that is to pass the "native" libfuse/WinFsp option directly:
`--fuse-flag --VolumePrefix=\server\share`. Note that the path
must be with just a single backslash prefix in this case.


*Note:* In previous versions of rclone this was the only supported method.

[Read more about drive mapping](https://en.wikipedia.org/wiki/Drive_mapping)

See also [Limitations](#limitations) section below.

#### Windows filesystem permissions

The FUSE emulation layer on Windows must convert between the POSIX-based
permission model used in FUSE, and the permission model used in Windows,
based on access-control lists (ACL).

The mounted filesystem will normally get three entries in its access-control list (ACL),
representing permissions for the POSIX permission scopes: Owner, group and others.
By default, the owner and group will be taken from the current user, and the built-in
group "Everyone" will be used to represent others. The user/group can be customized
with FUSE options "UserName" and "GroupName",
e.g. `-o UserName=user123 -o GroupName="Authenticated Users"`.
The permissions on each entry will be set according to [options](#options)
`--dir-perms` and `--file-perms`, which takes a value in traditional Unix
[numeric notation](https://en.wikipedia.org/wiki/File-system_permissions#Numeric_notation).

The default permissions corresponds to `--file-perms 0666 --dir-perms 0777`,
i.e. read and write permissions to everyone. This means you will not be able
to start any programs from the mount. To be able to do that you must add
execute permissions, e.g. `--file-perms 0777 --dir-perms 0777` to add it
to everyone. If the program needs to write files, chances are you will
have to enable [VFS File Caching](#vfs-file-caching) as well (see also
[limitations](#limitations)). Note that the default write permission have
some restrictions for accounts other than the owner, specifically it lacks
the "write extended attributes", as explained next.

The mapping of permissions is not always trivial, and the result you see in
Windows Explorer may not be exactly like you expected. For example, when setting
a value that includes write access for the group or others scope, this will be
mapped to individual permissions "write attributes", "write data" and
"append data", but not "write extended attributes". Windows will then show this
as basic permission "Special" instead of "Write", because "Write" also covers
the "write extended attributes" permission. When setting digit 0 for group or
others, to indicate no permissions, they will still get individual permissions
"read attributes", "read extended attributes" and "read permissions". This is
done for compatibility reasons, e.g. to allow users without additional
permissions to be able to read basic metadata about files like in Unix.

WinFsp 2021 (version 1.9) introduced a new FUSE option "FileSecurity",
that allows the complete specification of file security descriptors using
[SDDL](https://docs.microsoft.com/en-us/windows/win32/secauthz/security-descriptor-string-format).
With this you get detailed control of the resulting permissions, compared
to use of the POSIX permissions described above, and no additional permissions
will be added automatically for compatibility with Unix. Some example use
cases will following.

If you set POSIX permissions for only allowing access to the owner,
using `--file-perms 0600 --dir-perms 0700`, the user group and the built-in
"Everyone" group will still be given some special permissions, as described
above. Some programs may then (incorrectly) interpret this as the file being
accessible by everyone, for example an SSH client may warn about "unprotected
private key file". You can work around this by specifying
`-o FileSecurity="D:P(A;;FA;;;OW)"`, which sets file all access (FA) to the
owner (OW), and nothing else.

When setting write permissions then, except for the owner, this does not
include the "write extended attributes" permission, as mentioned above.
This may prevent applications from writing to files, giving permission denied
error instead. To set working write permissions for the built-in "Everyone"
group, similar to what it gets by default but with the addition of the
"write extended attributes", you can specify
`-o FileSecurity="D:P(A;;FRFW;;;WD)"`, which sets file read (FR) and file
write (FW) to everyone (WD). If file execute (FX) is also needed, then change
to `-o FileSecurity="D:P(A;;FRFWFX;;;WD)"`, or set file all access (FA) to
get full access permissions, including delete, with
`-o FileSecurity="D:P(A;;FA;;;WD)"`.

#### Windows caveats

Drives created as Administrator are not visible to other accounts,
not even an account that was elevated to Administrator with the
User Account Control (UAC) feature. A result of this is that if you mount
to a drive letter from a Command Prompt run as Administrator, and then try
to access the same drive from Windows Explorer (which does not run as
Administrator), you will not be able to see the mounted drive.

If you don't need to access the drive from applications running with
administrative privileges, the easiest way around this is to always
create the mount from a non-elevated command prompt.

To make mapped drives available to the user account that created them
regardless if elevated or not, there is a special Windows setting called
[linked connections](https://docs.microsoft.com/en-us/troubleshoot/windows-client/networking/mapped-drives-not-available-from-elevated-command#detail-to-configure-the-enablelinkedconnections-registry-entry)
that can be enabled.

It is also possible to make a drive mount available to everyone on the system,
by running the process creating it as the built-in SYSTEM account.
There are several ways to do this: One is to use the command-line
utility [PsExec](https://docs.microsoft.com/en-us/sysinternals/downloads/psexec),
from Microsoft's Sysinternals suite, which has option `-s` to start
processes as the SYSTEM account. Another alternative is to run the mount
command from a Windows Scheduled Task, or a Windows Service, configured
to run as the SYSTEM account. A third alternative is to use the
[WinFsp.Launcher infrastructure](https://github.com/winfsp/winfsp/wiki/WinFsp-Service-Architecture)).
Read more in the [install documentation](https://rclone.org/install/).
Note that when running rclone as another user, it will not use
the configuration file from your profile unless you tell it to
with the [`--config`](https://rclone.org/docs/#config-config-file) option.
Note also that it is now the SYSTEM account that will have the owner
permissions, and other accounts will have permissions according to the
group or others scopes. As mentioned above, these will then not get the
"write extended attributes" permission, and this may prevent writing to
files. You can work around this with the FileSecurity option, see
example above.

Note that mapping to a directory path, instead of a drive letter,
does not suffer from the same limitations.

### Mounting on macOS

Mounting on macOS can be done either via [built-in NFS server](/commands/rclone_serve_nfs/), [macFUSE](https://osxfuse.github.io/) 
(also known as osxfuse) or [FUSE-T](https://www.fuse-t.org/). macFUSE is a traditional
FUSE driver utilizing a macOS kernel extension (kext). FUSE-T is an alternative FUSE system
which "mounts" via an NFSv4 local server.

##### Unicode Normalization

It is highly recommended to keep the default of `--no-unicode-normalization=false`
for all `mount` and `serve` commands on macOS. For details, see [vfs-case-sensitivity](https://rclone.org/commands/rclone_mount/#vfs-case-sensitivity).

#### NFS mount

This method spins up an NFS server using [serve nfs](/commands/rclone_serve_nfs/) command and mounts
it to the specified mountpoint. If you run this in background mode using |--daemon|, you will need to
send SIGTERM signal to the rclone process using |kill| command to stop the mount.

Note that `--nfs-cache-handle-limit` controls the maximum number of cached file handles stored by the `nfsmount` caching handler.
This should not be set too low or you may experience errors when trying to access files. The default is 1000000,
but consider lowering this limit if the server's system resource usage causes problems.

#### macFUSE Notes

If installing macFUSE using [dmg packages](https://github.com/osxfuse/osxfuse/releases) from
the website, rclone will locate the macFUSE libraries without any further intervention.
If however, macFUSE is installed using the [macports](https://www.macports.org/) package manager,
the following addition steps are required.

    sudo mkdir /usr/local/lib
    cd /usr/local/lib
    sudo ln -s /opt/local/lib/libfuse.2.dylib

#### FUSE-T Limitations, Caveats, and Notes

There are some limitations, caveats, and notes about how it works. These are current as 
of FUSE-T version 1.0.14.

##### ModTime update on read

As per the [FUSE-T wiki](https://github.com/macos-fuse-t/fuse-t/wiki#caveats):

> File access and modification times cannot be set separately as it seems to be an 
> issue with the NFS client which always modifies both. Can be reproduced with 
> 'touch -m' and 'touch -a' commands

This means that viewing files with various tools, notably macOS Finder, will cause rlcone
to update the modification time of the file. This may make rclone upload a full new copy
of the file.
    
##### Read Only mounts

When mounting with `--read-only`, attempts to write to files will fail *silently* as
opposed to with a clear warning as in macFUSE.

### Limitations

Without the use of `--vfs-cache-mode` this can only write files
sequentially, it can only seek when reading.  This means that many
applications won't work with their files on an rclone mount without
`--vfs-cache-mode writes` or `--vfs-cache-mode full`.
See the [VFS File Caching](#vfs-file-caching) section for more info.
When using NFS mount on macOS, if you don't specify |--vfs-cache-mode|
the mount point will be read-only.

The bucket-based remotes (e.g. Swift, S3, Google Compute Storage, B2)
do not support the concept of empty directories, so empty
directories will have a tendency to disappear once they fall out of
the directory cache.

When `rclone mount` is invoked on Unix with `--daemon` flag, the main rclone
program will wait for the background mount to become ready or until the timeout
specified by the `--daemon-wait` flag. On Linux it can check mount status using
ProcFS so the flag in fact sets **maximum** time to wait, while the real wait
can be less. On macOS / BSD the time to wait is constant and the check is
performed only at the end. We advise you to set wait time on macOS reasonably.

Only supported on Linux, FreeBSD, OS X and Windows at the moment.

### rclone @ vs rclone sync/copy

File systems expect things to be 100% reliable, whereas cloud storage
systems are a long way from 100% reliable. The rclone sync/copy
commands cope with this with lots of retries.  However rclone @
can't use retries in the same way without making local copies of the
uploads. Look at the [VFS File Caching](#vfs-file-caching)
for solutions to make @ more reliable.

### Attribute caching

You can use the flag `--attr-timeout` to set the time the kernel caches
the attributes (size, modification time, etc.) for directory entries.

The default is `1s` which caches files just long enough to avoid
too many callbacks to rclone from the kernel.

In theory 0s should be the correct value for filesystems which can
change outside the control of the kernel. However this causes quite a
few problems such as
[rclone using too much memory](https://github.com/rclone/rclone/issues/2157),
[rclone not serving files to samba](https://forum.rclone.org/t/rclone-1-39-vs-1-40-mount-issue/5112)
and [excessive time listing directories](https://github.com/rclone/rclone/issues/2095#issuecomment-371141147).

The kernel can cache the info about a file for the time given by
`--attr-timeout`. You may see corruption if the remote file changes
length during this window.  It will show up as either a truncated file
or a file with garbage on the end.  With `--attr-timeout 1s` this is
very unlikely but not impossible.  The higher you set `--attr-timeout`
the more likely it is.  The default setting of "1s" is the lowest
setting which mitigates the problems above.

If you set it higher (`10s` or `1m` say) then the kernel will call
back to rclone less often making it more efficient, however there is
more chance of the corruption issue above.

If files don't change on the remote outside of the control of rclone
then there is no chance of corruption.

This is the same as setting the attr_timeout option in mount.fuse.

### Filters

Note that all the rclone filters can be used to select a subset of the
files to be visible in the mount.

### systemd

When running rclone @ as a systemd service, it is possible
to use Type=notify. In this case the service will enter the started state
after the mountpoint has been successfully set up.
Units having the rclone @ service specified as a requirement
will see all files and folders immediately in this mode.

Note that systemd runs mount units without any environment variables including
`PATH` or `HOME`. This means that tilde (`~`) expansion will not work
and you should provide `--config` and `--cache-dir` explicitly as absolute
paths via rclone arguments.
Since mounting requires the `fusermount` program, rclone will use the fallback
PATH of `/bin:/usr/bin` in this scenario. Please ensure that `fusermount`
is present on this PATH.

### Rclone as Unix mount helper

The core Unix program `/bin/mount` normally takes the `-t FSTYPE` argument
then runs the `/sbin/mount.FSTYPE` helper program passing it mount options
as `-o key=val,...` or `--opt=...`. Automount (classic or systemd) behaves
in a similar way.

rclone by default expects GNU-style flags `--key val`. To run it as a mount
helper you should symlink rclone binary to `/sbin/mount.rclone` and optionally
`/usr/bin/rclonefs`, e.g. `ln -s /usr/bin/rclone /sbin/mount.rclone`.
rclone will detect it and translate command-line arguments appropriately.

Now you can run classic mounts like this:
```
mount sftp1:subdir /mnt/data -t rclone -o vfs_cache_mode=writes,sftp_key_file=/path/to/pem
```

or create systemd mount units:
```
# /etc/systemd/system/mnt-data.mount
[Unit]
Description=Mount for /mnt/data
[Mount]
Type=rclone
What=sftp1:subdir
Where=/mnt/data
Options=rw,_netdev,allow_other,args2env,vfs-cache-mode=writes,config=/etc/rclone.conf,cache-dir=/var/rclone
```

optionally accompanied by systemd automount unit
```
# /etc/systemd/system/mnt-data.automount
[Unit]
Description=AutoMount for /mnt/data
[Automount]
Where=/mnt/data
TimeoutIdleSec=600
[Install]
WantedBy=multi-user.target
```

or add in `/etc/fstab` a line like
```
sftp1:subdir /mnt/data rclone rw,noauto,nofail,_netdev,x-systemd.automount,args2env,vfs_cache_mode=writes,config=/etc/rclone.conf,cache_dir=/var/cache/rclone 0 0
```

or use classic Automountd.
Remember to provide explicit `config=...,cache-dir=...` as a workaround for
mount units being run without `HOME`.

Rclone in the mount helper mode will split `-o` argument(s) by comma, replace `_`
by `-` and prepend `--` to get the command-line flags. Options containing commas
or spaces can be wrapped in single or double quotes. Any inner quotes inside outer
quotes of the same type should be doubled.

Mount option syntax includes a few extra options treated specially:

- `env.NAME=VALUE` will set an environment variable for the mount process.
  This helps with Automountd and Systemd.mount which don't allow setting
  custom environment for mount helpers.
  Typically you will use `env.HTTPS_PROXY=proxy.host:3128` or `env.HOME=/root`
- `command=cmount` can be used to run `cmount` or any other rclone command
  rather than the default `mount`.
- `args2env` will pass mount options to the mount helper running in background
  via environment variables instead of command line arguments. This allows to
  hide secrets from such commands as `ps` or `pgrep`.
- `vv...` will be transformed into appropriate `--verbose=N`
- standard mount options like `x-systemd.automount`, `_netdev`, `nosuid` and alike
  are intended only for Automountd and ignored by rclone.
