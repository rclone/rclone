---
title: "Bisync"
description: "Bidirectional cloud sync solution in rclone"
---

## Getting started {#getting-started}

- [Install rclone](/install/) and setup your remotes.
- Bisync will create its working directory
  at `~/.cache/rclone/bisync` on Linux
  or `C:\Users\MyLogin\AppData\Local\rclone\bisync` on Windows.
  Make sure that this location is writable.
- Run bisync with the `--resync` flag, specifying the paths
  to the local and remote sync directory roots.
- For successive sync runs, leave off the `--resync` flag.
- Consider using a [filters file](#filtering) for excluding
  unnecessary files and directories from the sync.
- Consider setting up the [--check-access](#check-access-option) feature
  for safety.
- On Linux, consider setting up a [crontab entry](#cron). bisync can
  safely run in concurrent cron jobs thanks to lock files it maintains.

Here is a typical run log (with timestamps removed for clarity):

```
rclone bisync /testdir/path1/ /testdir/path2/ --verbose
INFO  : Synching Path1 "/testdir/path1/" with Path2 "/testdir/path2/"
INFO  : Path1 checking for diffs
INFO  : - Path1    File is new                         - file11.txt
INFO  : - Path1    File is newer                       - file2.txt
INFO  : - Path1    File is newer                       - file5.txt
INFO  : - Path1    File is newer                       - file7.txt
INFO  : - Path1    File was deleted                    - file4.txt
INFO  : - Path1    File was deleted                    - file6.txt
INFO  : - Path1    File was deleted                    - file8.txt
INFO  : Path1:    7 changes:    1 new,    3 newer,    0 older,    3 deleted
INFO  : Path2 checking for diffs
INFO  : - Path2    File is new                         - file10.txt
INFO  : - Path2    File is newer                       - file1.txt
INFO  : - Path2    File is newer                       - file5.txt
INFO  : - Path2    File is newer                       - file6.txt
INFO  : - Path2    File was deleted                    - file3.txt
INFO  : - Path2    File was deleted                    - file7.txt
INFO  : - Path2    File was deleted                    - file8.txt
INFO  : Path2:    7 changes:    1 new,    3 newer,    0 older,    3 deleted
INFO  : Applying changes
INFO  : - Path1    Queue copy to Path2                 - /testdir/path2/file11.txt
INFO  : - Path1    Queue copy to Path2                 - /testdir/path2/file2.txt
INFO  : - Path2    Queue delete                        - /testdir/path2/file4.txt
NOTICE: - WARNING  New or changed in both paths        - file5.txt
NOTICE: - Path1    Renaming Path1 copy                 - /testdir/path1/file5.txt..path1
NOTICE: - Path1    Queue copy to Path2                 - /testdir/path2/file5.txt..path1
NOTICE: - Path2    Renaming Path2 copy                 - /testdir/path2/file5.txt..path2
NOTICE: - Path2    Queue copy to Path1                 - /testdir/path1/file5.txt..path2
INFO  : - Path2    Queue copy to Path1                 - /testdir/path1/file6.txt
INFO  : - Path1    Queue copy to Path2                 - /testdir/path2/file7.txt
INFO  : - Path2    Queue copy to Path1                 - /testdir/path1/file1.txt
INFO  : - Path2    Queue copy to Path1                 - /testdir/path1/file10.txt
INFO  : - Path1    Queue delete                        - /testdir/path1/file3.txt
INFO  : - Path2    Do queued copies to                 - Path1
INFO  : - Path1    Do queued copies to                 - Path2
INFO  : -          Do queued deletes on                - Path1
INFO  : -          Do queued deletes on                - Path2
INFO  : Updating listings
INFO  : Validating listings for Path1 "/testdir/path1/" vs Path2 "/testdir/path2/"
INFO  : Bisync successful
```

## Command line syntax

```
$ rclone bisync --help
Usage:
  rclone bisync remote1:path1 remote2:path2 [flags]

Positional arguments:
  Path1, Path2  Local path, or remote storage with ':' plus optional path.
                Type 'rclone listremotes' for list of configured remotes.

Optional Flags:
      --check-access            Ensure expected `RCLONE_TEST` files are found on
                                both Path1 and Path2 filesystems, else abort.
      --check-filename FILENAME Filename for `--check-access` (default: `RCLONE_TEST`)
      --check-sync CHOICE       Controls comparison of final listings:
                                `true | false | only` (default: true)
                                If set to `only`, bisync will only compare listings
                                from the last run but skip actual sync.
      --filters-file PATH       Read filtering patterns from a file
      --max-delete PERCENT      Safety check on maximum percentage of deleted files allowed.
                                If exceeded, the bisync run will abort. (default: 50%)
      --force                   Bypass `--max-delete` safety check and run the sync.
                                Consider using with `--verbose`
      --remove-empty-dirs       Remove empty directories at the final cleanup step.
  -1, --resync                  Performs the resync run.
                                Warning: Path1 files may overwrite Path2 versions.
                                Consider using `--verbose` or `--dry-run` first.
      --localtime               Use local time in listings (default: UTC)
      --no-cleanup              Retain working files (useful for troubleshooting and testing).
      --workdir PATH            Use custom working directory (useful for testing).
                                (default: `~/.cache/rclone/bisync`)
  -n, --dry-run                 Go through the motions - No files are copied/deleted.
  -v, --verbose                 Increases logging verbosity.
                                May be specified more than once for more details.
  -h, --help                    help for bisync
```

Arbitrary rclone flags may be specified on the
[bisync command line](/commands/rclone_bisync/), for example
`rclone bsync ./testdir/path1/ gdrive:testdir/path2/ --drive-skip-gdocs -v -v --timeout 10s`
Note that interactions of various rclone flags with bisync process flow
has not been fully tested yet.

### Paths

Path1 and Path2 arguments may be references to any mix of local directory
paths (absolute or relative), UNC paths (`//server/share/path`),
Windows drive paths (with a drive letter and `:`) or configured
[remotes](/docs/#syntax-of-remote-paths) with optional subdirectory paths.
Cloud references are distinguished by having a `:` in the argument
(see [Windows support](#windows) below).

Path1 and Path2 are treated equally, in that neither has priority for
file changes, and access efficiency does not change whether a remote
is on Path1 or Path2.

The listings in bisync working directory (default: `~/.cache/rclone/bisync`)
are named based on the Path1 and Path2 arguments so that separate syncs
to individual directories within the tree may be set up, e.g.:
`path_to_local_tree..dropbox_subdir.lst`.

Any empty directories after the sync on both the Path1 and Path2
filesystems are not deleted by default. If the `--remove-empty-dirs`
flag is specified, then both paths will have any empty directories purged
as the last step in the process.

## Command-line flags

#### --resync

This will effectively make both Path1 and Path2 filesystems contain a
matching superset of all files. Path2 files that do not exist in Path1 will
be copied to Path1, and the process will then sync the Path1 tree to Path2.

The base directories on the both Path1 and Path2 filesystems must exist
or bisync will fail. This is required for safety - that bisync can verify
that both paths are valid.

When using `--resync` a newer version of a file on the Path2 filesystem
will be overwritten by the Path1 filesystem version.
Carefully evaluate deltas using [--dry-run](/flags/#non-backend-flags).

For a resync run, one of the paths may be empty (no files in the path tree).
The resync run should result in files on both paths, else a normal non-resync
run will fail.

For a non-resync run, either path being empty (no files in the tree) fails with
`Empty current PathN listing. Cannot sync to an empty directory: X.pathN.lst`
This is a safety check that an unexpected empty path does not result in
deleting **everything** in the other path.

#### --check-access

Access check files are an additional safety measure against data loss.
bisync will ensure it can find matching `RCLONE_TEST` files in the same places
in the Path1 and Path2 filesystems.
Time stamps and file contents are not important, just the names and locations.
Place one or more `RCLONE_TEST` files in the Path1 or Path2 filesystem and
then do either a run without `--check-access` or a `--resync` to set
matching files on both filesystems.
If you have symbolic links in your sync tree it is recommended to place
`RCLONE_TEST` files in the linked-to directory tree to protect against
bisync assuming a bunch of deleted files if the linked-to tree should not be
accessible. Also see the `--check-filename` flag.

#### --max-delete

As a safety check, if greater than the `--max-delete` percent of files were
deleted on either the Path1 or Path2 filesystem, then bisync will abort with
a warning message, without making any changes.
The default `--max-delete` is `50%`.
One way to trigger this limit is to rename a directory that contains more
than half of your files. This will appear to bisync as a bunch of deleted
files and a bunch of new files.
This safety check is intended to block bisync from deleting all of the
files on both filesystems due to a temporary network access issue, or if
the user had inadvertently deleted the files on one side or the other.
To force the sync either set a different delete percentage limit,
e.g. `--max-delete 75` (allows up to 75% deletion), or use `--force`
to bypass the check.

Also see the [all files changed](#all-files-changed) check.

#### --filters-file {#filters-file}

By using rclone filter features you can exclude file types or directory
sub-trees from the sync.
See the [bisync filters](#filtering) section and generic
[--filter-from](/filtering/#filter-from-read-filtering-patterns-from-a-file)
documentation.
An [example filters file](#example-filters-file) contains filters for
non-allowed files for synching with Dropbox.

If you make changes to your filters file then bisync requires a run
with `--resync`. This is a safety feature, which avoids existing files
on the Path1 and/or Path2 side from seeming to disappear from view
(since they are excluded in the new listings), which would fool bisync
into seeing them as deleted (as compared to the prior run listings),
and then bisync would proceed to delete them for real.

To block this from happening bisync calculates an MD5 hash of the filters file
and stores the hash in a `.md5` file in the same place as your filters file.
On the next runs with `--filters-file` set, bisync re-calculates the MD5 hash
of the current filters file and compares it to the hash stored in `.md5` file.
If they don't match the run aborts with a critical error and thus forces you
to do a `--resync`, likely avoiding a disaster.

#### --check-sync

Enabled by default, the check-sync function checks that all of the same
files exist in both the Path1 and Path2 history listings. This _check-sync_
integrity check is performed at the end of the sync run by default.
Any untrapped failing copy/deletes between the two paths might result
in differences between the two listings and in the untracked file content
differences between the two paths. A resync run would correct the error.

Note that the default-enabled integrity check locally executes a load of both
the final Path1 and Path2 listings, and thus adds to the run time of a sync.
Using `--check-sync=false` will disable it and may significantly reduce the
sync run times for very large numbers of files.

The check may be run manually with `--check-sync=only`. It runs only the
integrity check and terminates without actually synching.

## Operation

### Runtime flow details

bisync retains the listings of the `Path1` and `Path2` filesystems
from the prior run.
On each successive run it will:

- list files on `path1` and `path2`, and check for changes on each side.
  Changes include `New`, `Newer`, `Older`, and `Deleted` files.
- Propagate changes on `path1` to `path2`, and vice-versa.

### Safety measures

- Lock file prevents multiple simultaneous runs when taking a while.
  This can be particularly useful if bisync is run by cron scheduler.
- Handle change conflicts non-destructively by creating
  `..path1` and `..path2` file versions.
- File system access health check using `RCLONE_TEST` files
  (see the `--check-access` flag).
- Abort on excessive deletes - protects against a failed listing
  being interpreted as all the files were deleted.
  See the `--max-delete` and `--force` flags.
- If something evil happens, bisync goes into a safe state to block
  damage by later runs. (See [Error Handling](#error-handling))

### Normal sync checks

 Type         | Description                                   | Result                   | Implementation
--------------|-----------------------------------------------|--------------------------|-----------------------------
Path2 new     | File is new on Path2, does not exist on Path1 | Path2 version survives   | `rclone copy` Path2 to Path1
Path2 newer   | File is newer on Path2, unchanged on Path1    | Path2 version survives   | `rclone copy` Path2 to Path1
Path2 deleted | File is deleted on Path2, unchanged on Path1  | File is deleted          | `rclone delete` Path1
Path1 new     | File is new on Path1, does not exist on Path2 | Path1 version survives   | `rclone copy` Path1 to Path2
Path1 newer   | File is newer on Path1, unchanged on Path2    | Path1 version survives   | `rclone copy` Path1 to Path2
Path1 older   | File is older on Path1, unchanged on Path2    | _Path1 version survives_ | `rclone copy` Path1 to Path2
Path2 older   | File is older on Path2, unchanged on Path1    | _Path2 version survives_ | `rclone copy` Path2 to Path1
Path1 deleted | File no longer exists on Path1                | File is deleted          | `rclone delete` Path2

### Unusual sync checks

 Type                           | Description                           | Result                             | Implementation
--------------------------------|---------------------------------------|------------------------------------|-----------------------
Path1 new AND Path2 new         | File is new on Path1 AND new on Path2 | Files renamed to _Path1 and _Path2 | `rclone copy` _Path2 file to Path1, `rclone copy` _Path1 file to Path2
Path2 newer AND Path1 changed   | File is newer on Path2 AND also changed (newer/older/size) on Path1 | Files renamed to _Path1 and _Path2 | `rclone copy` _Path2 file to Path1, `rclone copy` _Path1 file to Path2
Path2 newer AND Path1 deleted   | File is newer on Path2 AND also deleted on Path1 | Path2 version survives  | `rclone copy` Path2 to Path1
Path2 deleted AND Path1 changed | File is deleted on Path2 AND changed (newer/older/size) on Path1 | Path1 version survives |`rclone copy` Path1 to Path2
Path1 deleted AND Path2 changed | File is deleted on Path1 AND changed (newer/older/size) on Path2 | Path2 version survives  | `rclone copy` Path2 to Path1

### All files changed check {#all-files-changed}

if _all_ prior existing files on either of the filesystems have changed
(e.g. timestamps have changed due to changing the system's timezone)
then bisync will abort without making any changes.
Any new files are not considered for this check. You could use `--force`
to force the sync (whichever side has the changed timestamp files wins).
Alternately, a `--resync` may be used (Path1 versions will be pushed
to Path2). Consider the situation carefully and perhaps use `--dry-run`
before you commit to the changes.

### Modification time

Bisync relies on file timestamps to identify changed files and will
_refuse_ to operate if backend lacks the modification time support.

If you or your application should change the content of a file
without changing the modification time then bisync will _not_
notice the change, and thus will not copy it to the other side.

Note that on some cloud storage systems it is not possible to have file
timestamps that match _precisely_ between the local and other filesystems.

Bisync's approach to this problem is by tracking the changes on each side
_separately_ over time with a local database of files in that side then
applying the resulting changes on the other side.

### Error handling {#error-handling}

Certain bisync critical errors, such as file copy/move failing, will result in
a bisync lockout of following runs. The lockout is asserted because the sync
status and history of the Path1 and Path2 filesystems cannot be trusted,
so it is safer to block any further changes until someone checks things out.
The recovery is to do a `--resync` again.

It is recommended to use `--resync --dry-run --verbose` initially and
_carefully_ review what changes will be made before running the `--resync`
without `--dry-run`.

Most of these events come up due to a error status from an internal call.
On such a critical error the `{...}.path1.lst` and `{...}.path2.lst`
listing files are renamed to extension `.lst-err`, which blocks any future
bisync runs (since the normal `.lst` files are not found).
Bisync keeps them under `bisync` subdirectory of the rclone cache direcory,
typically at `${HOME}/.cache/rclone/bisync/` on Linux.

Some errors are considered temporary and re-running the bisync is not blocked.
The _critical return_ blocks further bisync runs.

### Lock file

When bisync is running, a lock file is created in the bisync working directory,
typically at `~/.cache/rclone/bisync/PATH1..PATH2.lck` on Linux.
If bisync should crash or hang, the lock file will remain in place and block
any further runs of bisync _for the same paths_.
Delete the lock file as part of debugging the situation.
The lock file effectively blocks follow-on (e.g., scheduled by _cron_) runs
when the prior invocation is taking a long time.
The lock file contains _PID_ of the blocking process, which may help in debug.

**Note**
that while concurrent bisync runs are allowed, _be very cautious_
that there is no overlap in the trees being synched between concurrent runs,
lest there be replicated files, deleted files and general mayhem.

### Return codes

`rclone bisync` returns the following codes to calling program:
- `0` on a successful run,
- `1` for a non-critical failing run (a rerun may be successful),
- `2` for a critically aborted run (requires a `--resync` to recover).

## Limitations

### Supported backends

Bisync is considered _BETA_ and has been tested with the following backends:
- Local filesystem
- Google Drive
- Dropbox
- OneDrive
- S3
- SFTP

It has not been fully tested with other services yet.
If it works, or sorta works, please let us know and we'll update the list.
Run the test suite to check for proper operation as described below.

First release of `rclone bisync` requires that underlying backend supported
the modification time feature and will refuse to run otherwise.
This limitation will be lifted in a future `rclone bisync` release.

### Concurrent modifications

When using **Local, FTP or SFTP** remotes rclone does not create _temporary_
files at the destination when copying, and thus if the connection is lost
the created file may be corrupt, which will likely propagate back to the
original path on the next sync, resulting in data loss.
This will be solved in a future release, there is no workaround at the moment.

Files that **change during** a bisync run may result in data loss.
This has been seen in a highly dynamic environment, where the filesystem
is getting hammered by running processes during the sync.
The solution is to sync at quiet times or [filter out](#filtering)
unnecessary directories and files.

### Empty directories

New empty directories on one path are _not_ propagated to the other side.
This is because bisync (and rclone) natively works on files not directories.
The following sequence is a workaround but will not propagate the delete
of an empty directory to the other side:

```
rclone bisync PATH1 PATH2
rclone copy PATH1 PATH2 --filter "+ */" --filter "- **" --create-empty-src-dirs
rclone copy PATH2 PATH2 --filter "+ */" --filter "- **" --create-empty-src-dirs
```

### Renamed directories

Renaming a folder on the Path1 side results is deleting all files on
the Path2 side and then copying all files again from Path1 to Path2.
Bisync sees this as all files in the old directory name as deleted and all
files in the new directory name as new. Similarly, renaming a directory on
both sides to the same name will result in creating `..path1` and `..path2`
files on both sides.
Currently the most effective and efficient method of renaming a directory
is to rename it on both sides, then do a `--resync`.

### Case sensitivity

Synching with **case-insensitive** filesystems, such as Windows or `Box`,
can result in file name conflicts. This will be fixed in a future release.
The near term workaround is to make sure that files on both sides
don't have spelling case differences (`Smile.jpg` vs. `smile.jpg`).

## Windows support {#windows}

Bisync has been tested on Windows 8.1, Windows 10 Pro 64-bit and on Windows
Github runners.

Drive letters are allowed, including drive letters mapped to network drives
(`rclone bisync J:\localsync GDrive:`).
If a drive letter is omitted, the shell current drive is the default.
Drive letters are a single character follows by `:`, so cloud names
must be more than one character long.

Absolute paths (with or without a drive letter), and relative paths
(with or without a drive letter) are supported.

Working directory is created at `C:\Users\MyLogin\AppData\Local\rclone\bisync`.

Note that bisync output may show a mix of forward `/` and back `\` slashes.

Be careful of case independent directory and file naming on Windows
vs. case dependent Linux

## Filtering {#filtering}

See [filtering documentation](/filtering/)
for how filter rules are written and interpreted.

Bisync's [`--filters-file`](#filters-file) flag slightly extends the rclone's
[--filter-from](/filtering/#filter-from-read-filtering-patterns-from-a-file)
filtering mechanism.
For a given bisync run you may provide _only one_ `--filters-file`.
The `--include*`, `--exclude*`, and `--filter` flags are also supported.

### How to filter directories

Filtering portions of the directory tree is a critical feature for synching.

Examples of directory trees (always beneath the Path1/Path2 root level)
you may want to exclude from your sync:
- Directory trees containing only software build intermediate files.
- Directory trees containing application temporary files and data
  such as the Windows `C:\Users\MyLogin\AppData\` tree.
- Directory trees containing files that are large, less important,
  or are getting thrashed continuously by ongoing processes.

On the other hand, there may be only select directories that you
actually want to sync, and exclude all others. See the
[Example include-style filters for Windows user directories](#include-filters)
below.

### Filters file writing guidelines

1. Begin with excluding directory trees:
    - e.g. `- /AppData/`
    - `**` on the end is not necessary. Once a given directory level
      is excluded then everything beneath it won't be looked at by rclone.
    - Exclude such directories that are unneeded, are big, dynamically thrashed,
      or where there may be access permission issues.
    - Excluding such dirs first will make rclone operations (much) faster.
    - Specific files may also be excluded, as with the Dropbox exclusions
      example below.
2. Decide if its easier (or cleaner) to:
    - Include select directories and therefore _exclude everything else_ -- or --
    - Exclude select directories and therefore _include everything else_
3. Include select directories:
    - Add lines like: `+ /Documents/PersonalFiles/**` to select which
      directories to include in the sync.
    - `**` on the end specifies to include the full depth of the specified tree.
    - With Include-style filters, files at the Path1/Path2 root are not included.
      They may be included with `+ /*`.
    - Place RCLONE_TEST files within these included directory trees.
      They will only be looked for in these directory trees.
    - Finish by excluding everything else by adding `- **` at the end
      of the filters file.
    - Disregard step 4.
4. Exclude select directories:
    - Add more lines like in step 1.
      For example: `-/Desktop/tempfiles/`, or `- /testdir/`.
      Again, a `**` on the end is not necessary.
    - Do _not_ add a `- **` in the file. Without this line, everything
      will be included that has not be explicitly excluded.
    - Disregard step 3.

A few rules for the syntax of a filter file expanding on
[filtering documentation](/filtering/):

- Lines may start with spaces and tabs - rclone strips leading whitespace.
- If the first non-whitespace character is a `#` then the line is a comment
  and will be ignored.
- Blank lines are ignored.
- The first non-whitespace character on a filter line must be a `+` or `-`.
- Exactly 1 space is allowed between the `+/-` and the path term.
- Only forward slashes (`/`) are used in path terms, even on Windows.
- The rest of the line is taken as the path term.
  Trailing whitespace is taken literally, and probably is an error.

### Example include-style filters for Windows user directories {#include-filters}

This Windows _include-style_ example is based on the sync root (Path1)
set to `C:\Users\MyLogin`. The strategy is to select specific directories
to be synched with a network drive (Path2).

- `- /AppData/` excludes an entire tree of Windows stored stuff
  that need not be synched.
  In my case, AppData has >11 GB of stuff I don't care about, and there are
  some subdirectories beneath AppData that are not accessible to my
  user login, resulting in bisync critical aborts.
- Windows creates cache files starting with both upper and
  lowercase `NTUSER` at `C:\Users\MyLogin`. These files may be dynamic,
  locked, and are generally _don't care_.
- There are just a few directories with _my_ data that I do want synched,
  in the form of `+ /<path>`. By selecting only the directory trees I
  want to avoid the dozen plus directories that various apps make
  at `C:\Users\MyLogin\Documents`.
- Include files in the root of the sync point, `C:\Users\MyLogin`,
  by adding the `+ /*` line.
- This is an Include-style filters file, therefore it ends with `- **`
  which excludes everything not explicitly included.

```
- /AppData/
- NTUSER*
- ntuser*
+ /Documents/Family/**
+ /Documents/Sketchup/**
+ /Documents/Microcapture_Photo/**
+ /Documents/Microcapture_Video/**
+ /Desktop/**
+ /Pictures/**
+ /*
- **
```

Note also that Windows implements several "library" links such as
`C:\Users\MyLogin\My Documents\My Music` pointing to `C:\Users\MyLogin\Music`.
rclone sees these as links, so you must add `--links` to the
bisync command line if you which to follow these links. I find that I get
permission errors in trying to follow the links, so I don't include the
rclone `--links` flag, but then you get lots of `Can't follow symlink…`
noise from rclone about not following the links. This noise can be
quashed by adding `--quiet` to the bisync command line.

## Example exclude-style filters files for use with Dropbox {#exclude-filters}

- Dropbox disallows synching the listed temporary and configuration/data files.
  The `- <filename>` filters exclude these files where ever they may occur
  in the sync tree. Consider adding similar exclusions for file types
  you don't need to sync, such as core dump and software build files.
- bisync testing creates `/testdir/` at the top level of the sync tree,
  and usually deletes the tree after the test. If a normal sync should run
  while the `/testdir/` tree exists the `--check-access` phase may fail
  due to unbalanced RCLONE_TEST files.
  The `- /testdir/` filter blocks this tree from being synched.
  You don't need this exclusion if you are not doing bisync development testing.
- Everything else beneath the Path1/Path2 root will be synched.
- RCLONE_TEST files may be placed anywhere within the tree, including the root.

### Example filters file for Dropbox {#example-filters-file}

```
# Filter file for use with bisync
# See https://rclone.org/filtering/ for filtering rules
# NOTICE: If you make changes to this file you MUST do a --resync run.
#         Run with --dry-run to see what changes will be made.

# Dropbox wont sync some files so filter them away here.
# See https://help.dropbox.com/installs-integrations/sync-uploads/files-not-syncing
- .dropbox.attr
- ~*.tmp
- ~$*
- .~*
- desktop.ini
- .dropbox

# Used for bisync testing, so excluded from normal runs
- /testdir/

# Other example filters
#- /TiBU/
#- /Photos/
```

### How --check-access handles filters

At the start of a bisync run, listings are gathered for Path1 and Path2
while using the user's `--filters-file`. During the check access phase,
bisync scans these listings for `RCLONE_TEST` files.
Any `RCLONE_TEST` files hidden by the `--filters-file` are _not_ in the
listings and thus not checked during the check access phase.

## Troubleshooting {#troubleshooting}

### Reading bisync logs

Here are two normal runs. The first one has a newer file on the remote.
The second has no deltas between local and remote.

```
2021/05/16 00:24:38 INFO  : Synching Path1 "/path/to/local/tree/" with Path2 "dropbox:/"
2021/05/16 00:24:38 INFO  : Path1 checking for diffs
2021/05/16 00:24:38 INFO  : - Path1    File is new                         - file.txt
2021/05/16 00:24:38 INFO  : Path1:    1 changes:    1 new,    0 newer,    0 older,    0 deleted
2021/05/16 00:24:38 INFO  : Path2 checking for diffs
2021/05/16 00:24:38 INFO  : Applying changes
2021/05/16 00:24:38 INFO  : - Path1    Queue copy to Path2                 - dropbox:/file.txt
2021/05/16 00:24:38 INFO  : - Path1    Do queued copies to                 - Path2
2021/05/16 00:24:38 INFO  : Updating listings
2021/05/16 00:24:38 INFO  : Validating listings for Path1 "/path/to/local/tree/" vs Path2 "dropbox:/"
2021/05/16 00:24:38 INFO  : Bisync successful

2021/05/16 00:36:52 INFO  : Synching Path1 "/path/to/local/tree/" with Path2 "dropbox:/"
2021/05/16 00:36:52 INFO  : Path1 checking for diffs
2021/05/16 00:36:52 INFO  : Path2 checking for diffs
2021/05/16 00:36:52 INFO  : No changes found
2021/05/16 00:36:52 INFO  : Updating listings
2021/05/16 00:36:52 INFO  : Validating listings for Path1 "/path/to/local/tree/" vs Path2 "dropbox:/"
2021/05/16 00:36:52 INFO  : Bisync successful
```

### Dry run oddity

The `--dry-run` messages may indicate that it would try to delete some files.
For example, if a file is new on Path2 and does not exist on Path1 then
it would normally be copied to Path1, but with `--dry-run` enabled those
copies don't happen, which leads to the attempted delete on the Path2,
blocked again by --dry-run: `... Not deleting as --dry-run`.

This whole confusing situation is an artifact of the `--dry-run` flag.
Scrutinize the proposed deletes carefully, and if the files would have been
copied to Path1 then the threatened deletes on Path2 may be disregarded.

### Retries

Rclone has built in retries. If you run with `--verbose` you'll see
error and retry messages such as shown below. This is usually not a bug.
If at the end of the run you see `Bisync successful` and not
`Bisync critical error` or `Bisync aborted` then the run was successful,
and you can ignore the error messages.

The following run shows an intermittent fail. Lines _5_ and _6- are
low level messages. Line _6_ is a bubbled-up _warning_ message, conveying
the error. Rclone normally retries failing commands, so there may be
numerous such messages in the log.

Since there are no final error/warning messages on line _7_, rclone has
recovered from failure after a retry, and the overall sync was successful.

```
1: 2021/05/14 00:44:12 INFO  : Synching Path1 "/path/to/local/tree" with Path2 "dropbox:"
2: 2021/05/14 00:44:12 INFO  : Path1 checking for diffs
3: 2021/05/14 00:44:12 INFO  : Path2 checking for diffs
4: 2021/05/14 00:44:12 INFO  : Path2:  113 changes:   22 new,    0 newer,    0 older,   91 deleted
5: 2021/05/14 00:44:12 ERROR : /path/to/local/tree/objects/af: error listing: unexpected end of JSON input
6: 2021/05/14 00:44:12 NOTICE: WARNING  listing try 1 failed.                 - dropbox:
7: 2021/05/14 00:44:12 INFO  : Bisync successful
```

This log shows a _Critical failure_ which requires a `--resync` to recover from.
See the [Runtime Error Handling](#error-handling) section.

```
2021/05/12 00:49:40 INFO  : Google drive root '': Waiting for checks to finish
2021/05/12 00:49:40 INFO  : Google drive root '': Waiting for transfers to finish
2021/05/12 00:49:40 INFO  : Google drive root '': not deleting files as there were IO errors
2021/05/12 00:49:40 ERROR : Attempt 3/3 failed with 3 errors and: not deleting files as there were IO errors
2021/05/12 00:49:40 ERROR : Failed to sync: not deleting files as there were IO errors
2021/05/12 00:49:40 NOTICE: WARNING  rclone sync try 3 failed.           - /path/to/local/tree/
2021/05/12 00:49:40 ERROR : Bisync aborted. Must run --resync to recover.
```

### Denied downloads of "infected" or "abusive" files

Google Drive has a filter for certain file types (`.exe`, `.apk`, et cetera)
that by default cannot be copied from Google Drive to the local filesystem.
If you are having problems, run with `--verbose` to see specifically which
files are generating complaints. If the error is
`This file has been identified as malware or spam and cannot be downloaded`,
consider using the flag
[--drive-acknowledge-abuse](/drive/#drive-acknowledge-abuse).

### Google Doc files

Google docs exist as virtual files on Google Drive and cannot be transferred
to other filesystems natively. While it is possible to export a Google doc to
a normal file (with `.xlsx` extension, for example), it's not possible
to import a normal file back into a Google document.

Bisync's handling of Google Doc files is to flag them in the run log output
for user's attention and ignore them for any file transfers, deletes, or syncs.
They will show up with a length of `-1` in the listings.
This bisync run is otherwise successful:

```
2021/05/11 08:23:15 INFO  : Synching Path1 "/path/to/local/tree/base/" with Path2 "GDrive:"
2021/05/11 08:23:15 INFO  : ...path2.lst-new: Ignoring incorrect line: "- -1 - - 2018-07-29T08:49:30.136000000+0000 GoogleDoc.docx"
2021/05/11 08:23:15 INFO  : Bisync successful
```

## Usage examples

### Cron {#cron}

Rclone does not yet have a built-in capability to monitor the local file
system for changes and must be blindly run periodically.
On Windows this can be done using a _Task Scheduler_,
on Linux you can use _Cron_ which is described below.

The 1st example runs a sync every 5 minutes between a local directory
and an OwnCloud server, with output logged to a runlog file:

```
# Minute (0-59)
#      Hour (0-23)
#           Day of Month (1-31)
#                Month (1-12 or Jan-Dec)
#                     Day of Week (0-6 or Sun-Sat)
#                         Command
  */5  *    *    *    *   /path/to/rclone bisync /local/files MyCloud: --check-access --filters-file /path/to/bysync-filters.txt --log-file /path/to//bisync.log
```

See [crontab syntax](https://www.man7.org/linux/man-pages/man1/crontab.1p.html#INPUT_FILES)).
for the details of crontab time interval expressions.

If you run `rclone bisync` as a cron job, redirect stdout/stderr to a file.
The 2nd example runs a sync to Dropbox every hour and logs all stdout (via the `>>`)
and stderr (via `2>&1`) to a log file.

```
0 * * * * /path/to/rclone bisync /path/to/local/dropbox Dropbox: --check-access --filters-file /home/user/filters.txt >> /path/to/logs/dropbox-run.log 2>&1
```

### Sharing an encrypted folder tree between hosts

bisync can keep a local folder in sync with a cloud service,
but what if you have some highly sensitive files to be synched?

Usage of a cloud service is for exchanging both routine and sensitive
personal files between one's home network, one's personal notebook when on the
road, and with one's work computer. The routine data is not sensitive.
For the sensitive data, configure an rclone [crypt remote](/crypt/) to point to
a subdirectory within the local disk tree that is bisync'd to Dropbox,
and then set up an bisync for this local crypt directory to a directory
outside of the main sync tree.

### Linux server setup

- `/path/to/DBoxroot` is the root of my local sync tree.
  There are numerous subdirectories.
- `/path/to/DBoxroot/crypt` is the root subdirectory for files
  that are encrypted. This local directory target is setup as an
  rclone crypt remote named `Dropcrypt:`.
  See [rclone.conf](#rclone-conf-snippet) snippet below.
- `/path/to/my/unencrypted/files` is the root of my sensitive
  files - not encrypted, not within the tree synched to Dropbox.
- To sync my local unencrypted files with the encrypted Dropbox versions
  I manually run `bisync /path/to/my/unencrypted/files DropCrypt:`.
  This step could be bundled into a script to run before and after
  the full Dropbox tree sync in the last step,
  thus actively keeping the sensitive files in sync.
- `bisync /path/to/DBoxroot Dropbox:` runs periodically via cron,
  keeping my full local sync tree in sync with Dropbox.

### Windows notebook setup

- The Dropbox client runs keeping the local tree `C:\Users\MyLogin\Dropbox`
  always in sync with Dropbox. I could have used `rclone bisync` instead.
- A separate directory tree at `C:\Users\MyLogin\Documents\DropLocal`
  hosts the tree of unencrypted files/folders.
- To sync my local unencrypted files with the encrypted
  Dropbox versions I manually run the following command:
  `rclone bisync C:\Users\MyLogin\Documents\DropLocal Dropcrypt:`.
- The Dropbox client then syncs the changes with Dropbox.

### rclone.conf snippet {#rclone-conf-snippet}

```
[Dropbox]
type = dropbox
...

[Dropcrypt]
type = crypt
remote = /path/to/DBoxroot/crypt          # on the Linux server
remote = C:\Users\MyLogin\Dropbox\crypt   # on the Windows notebook
filename_encryption = standard
directory_name_encryption = true
password = ...
...
```

## Testing {#testing}

You should read this section only if you are developing for rclone.
You need to have rclone source code locally to work with bisync tests.

Bisync has a dedicated test framework implemented in the `bisync_test.go`
file located in the rclone source tree. The test suite is based on the
`go test` command. Series of tests are stored in subdirectories below the
`cmd/bisync/testdata` directory. Individual tests can be invoked by their
directory name, e.g.
`go test . -case basic -remote local -remote2 gdrive: -v`

Tests will make a temporary folder on remote and purge it afterwards.
If during test run there are intermittent errors and rclone retries,
these errors will be captured and flagged as invalid MISCOMPAREs.
Rerunning the test will let it pass. Consider such failures as noise.

### Test command syntax

```
usage: go test ./cmd/bisync [options...]

Options:
  -case NAME        Name(s) of the test case(s) to run. Multiple names should
                    be separated by commas. You can remove the `test_` prefix
                    and replace `_` by `-` in test name for convenience.
                    If not `all`, the name(s) should map to a directory under
                    `./cmd/bisync/testdata`.
                    Use `all` to run all tests (default: all)
  -remote PATH1     `local` or name of cloud service with `:` (default: local)
  -remote2 PATH2    `local` or name of cloud service with `:` (default: local)
  -no-compare       Disable comparing test results with the golden directory
                    (default: compare)
  -no-cleanup       Disable cleanup of Path1 and Path2 testdirs.
                    Useful for troubleshooting. (default: cleanup)
  -golden           Store results in the golden directory (default: false)
                    This flag can be used with multiple tests.
  -debug            Print debug messages
  -stop-at NUM      Stop test after given step number. (default: run to the end)
                    Implies `-no-compare` and `-no-cleanup`, if the test really
                    ends prematurely. Only meaningful for a single test case.
  -refresh-times    Force refreshing the target modtime, useful for Dropbox
                    (default: false)
  -verbose          Run tests verbosely
```

Note: unlike rclone flags which must be prefixed by double dash (`--`), the
test command flags can be equally prefixed by a single `-` or double dash.

### Running tests

- `go test . -case basic -remote local -remote2 local`
  runs the `test_basic` test case using only the local filesystem,
  synching one local directory with another local directory.
  Test script output is to the console, while commands within scenario.txt
  have their output sent to the `.../workdir/test.log` file,
  which is finally compared to the golden copy.
- The first argument after `go test` should be a relative name of the
  directory containing bisync source code. If you run tests right from there,
  the argument will be `.` (current directory) as in most examples below.
  If you run bisync tests from the rclone source directory, the command
  should be `go test ./cmd/bisync ...`.
- The test engine will mangle rclone output to ensure comparability
  with golden listings and logs.
- Test scenarios are located in `./cmd/bisync/testdata`. The test `-case`
  argument should match the full name of a subdirectory under that
  directory. Every test subdirectory name on disk must start with `test_`,
  this prefix can be omitted on command line for brevity. Also, underscores
  in the name can be replaced by dashes for convenience.
- `go test . -remote local -remote2 local -case all` runs all tests.
- Path1 and Path2 may either be the keyword `local`
  or may be names of configured cloud services.
  `go test . -remote gdrive: -remote2 dropbox: -case basic`
  will run the test between these two services, without transferring
  any files to the local filesystem.
- Test run stdout and stderr console output may be directed to a file, e.g.
  `go test . -remote gdrive: -remote2 local -case all > runlog.txt 2>&1`

### Test execution flow

1. The base setup in the `initial` directory of the testcase is applied
   on the Path1 and Path2 filesystems (via rclone copy the initial directory
   to Path1, then rclone sync Path1 to Path2).
2. The commands in the scenario.txt file are applied, with output directed
   to the `test.log` file in the test working directory.
   Typically, the first actual command in the `scenario.txt` file is
   to do a `--resync`, which establishes the baseline
   `{...}.path1.lst` and `{...}.path2.lst` files in the test working
   directory (`.../workdir/` relative to the temporary test directory).
   Various commands and listing snapshots are done within the test.
3. Finally, the contents of the test working directory are compared
   to the contents of the testcase's golden directory.

### Notes about testing

- Test cases are in individual directories beneath `./cmd/bisync/testdata`.
  A command line reference to a test is understood to reference a directory
  beneath `testdata`. For example,
  `go test ./cmd/bisync -case dry-run -remote gdrive: -remote2 local`
  refers to the test case in `./cmd/bisync/testdata/test_dry_run`.
- The test working directory is located at `.../workdir` relative to a
  temporary test directory, usually under `/tmp` on Linux.
- The local test sync tree is created at a temporary directory named
  like `bisync.XXX` under system temporary directory.
- The remote test sync tree is located at a temporary directory
  under `<remote:>/bisync.XXX/`.
- `path1` and/or `path2` subdirectories are created in a temporary
  directory under the respective local or cloud test remote.
- By default, the Path1 and Path2 test dirs and workdir will be deleted
  after each test run. The `-no-cleanup` flag disables purging these
  directories when validating and debugging a given test.
  These directories will be flushed before running another test,
  independent of the `-no-cleanup` usage.
- You will likely want to add `- /testdir/` to your normal
  bisync `--filters-file` so that normal syncs do not attempt to sync
  the test temporary directories, which may have `RCLONE_TEST` miscompares
  in some testcases which would otherwise trip the `--check-access` system.
  The `--check-access` mechanism is hard-coded to ignore `RCLONE_TEST`
  files beneath `bisync/testdata`, so the test cases may reside on the
  synched tree even if there are check file mismatches in the test tree.
- Some Dropbox tests can fail, notably printing the following message:
  `src and dst identical but can't set mod time without deleting and re-uploading`
  This is expected and happens due a way Dropbox handles modificaion times.
  You should use the `-refresh-times` test flag to make up for this.
- If Dropbox tests hit request limit for you and print error message
  `too_many_requests/...: Too many requests or write operations.`
  then follow the
  [Dropbox App ID instructions](/dropbox/#get-your-own-dropbox-app-id).

### Updating golden results

Sometimes even a slight change in the bisync source can cause little changes
spread around many log files. Updating them manually would be a nighmare.

The `-golden` flag will store the `test.log` and `*.lst` listings from each
test case into respective golden directories. Golden results will
automatically contain generic strings instead of local or cloud paths which
means that they should match when run with a different cloud service.

Your normal workflow might be as follows:
1. Git-clone the rclone sources locally
2. Modify bisync source and check that it builds
3. Run the whole test suite `go test ./cmd/bisync -remote local`
4. If some tests show log difference, recheck them individually, e.g.:
   `go test ./cmd/bisync -remote local -case basic`
5. If you are convinced with the difference, goldenize all tests at once:
   `go test ./cmd/bisync -remote local -golden`
6. Use word diff: `git diff --word-diff ./cmd/bisync/testdata/`.
   Please note that normal line-level diff is generally useless here.
7. Check the difference _carefully_!
8. Commit the change (`git commit`) _only_ if you are sure.
   If unsure, save your code changes then wipe the log diffs from git:
   `git reset [--hard]`.

### Structure of test scenarios

- `<testname>/initial/` contains a tree of files that will be set
  as the initial condition on both Path1 and Path2 testdirs.
- `<testname>/modfiles/` contains files that will be used to
  modify the Path1 and/or Path2 filesystems.
- `<testname>/golden/` contains the expected content of the test
  working directory (`workdir`) at the completion of the testcase.
- `<testname>/scenario.txt` contains the body of the test, in the form of
  various commands to modify files, run bisync, and snapshot listings.
  Output from these commands is captured to `.../workdir/test.log`
  for comparison to the golden files.

### Supported test commands

- `test <some message>`
  Print the line to the console and to the `test.log`:
  `test sync is working correctly with options x, y, z`
- `copy-listings <prefix>`
  Save a copy of all `.lst` listings in the test working directory
  with the specified prefix:
  `save-listings exclude-pass-run`
- `move-listings <prefix>`
  Similar to `copy-listings` but removes the source
- `purge-children <dir>`
  This will delete all child files and purge all child subdirs under given
  directory but keep the parent intact. This behavior is important for tests
  with Google Drive because removing and re-creating the parent would change
  its ID.
- `delete-file <file>`
  Delete a single file.
- `delete-glob <dir> <pattern>`
  Delete a group of files located one level deep in the given directory
  with names maching a given glob pattern.
- `touch-glob YYYY-MM-DD <dir> <pattern>`
  Change modification time on a group of files.
- `touch-copy YYYY-MM-DD <source-file> <dest-dir>`
  Change file modification time then copy it to destination.
- `copy-file <source-file> <dest-dir>`
  Copy a single file to given directory.
- `copy-as <source-file> <dest-file>`
  Similar to above but destination must include both directory
  and the new file name at destination.
- `copy-dir <src> <dst>` and `sync-dir <src> <dst>`
  Copy/sync a directory. Equivalent of `rclone copy` and `rclone sync`.
- `list-dirs <dir>`
  Equivalent to `rclone lsf -R --dirs-only <dir>`
- `bisync [options]`
  Runs bisync against `-remote` and `-remote2`.

### Supported substitution terms

- `{testdir/}` - the root dir of the testcase
- `{datadir/}` - the `modfiles` dir under the testcase root
- `{workdir/}` - the temporary test working directory
- `{path1/}` - the root of the Path1 test directory tree
- `{path2/}` - the root of the Path2 test directory tree
- `{session}` - base name of the test listings
- `{/}` - OS-specific path separator
- `{spc}`, `{tab}`, `{eol}` - whitespace
- `{chr:HH}` - raw byte with given hexadecimal code

Substitution results of the terms named like `{dir/}` will end with
`/` (or backslash on Windows), so it is not necessary to include
slash in the usage, for example `delete-file {path1/}file1.txt`.

## Benchmarks

_This section is work in progress._

Here are a few data points for scale, execution times, and memory usage.

The first set of data was taken between a local disk to Dropbox.
The [speedtest.net](https://speedtest.net) download speed was ~170 Mbps,
and upload speed was ~10 Mbps. 500 files (~9.5 MB each) had been already
synched. 50 files were added in a new directory, each ~9.5 MB, ~475 MB total.

Change                                | Operations and times                                   | Overall run time
--------------------------------------|--------------------------------------------------------|------------------
500 files synched (nothing to move)   | 1x listings for Path1 & Path2                          | 1.5 sec
500 files synched with --check-access | 1x listings for Path1 & Path2                          | 1.5 sec
50 new files on remote                | Queued 50 copies down: 27 sec                          |  29 sec
Moved local dir                       | Queued 50 copies up: 410 sec, 50 deletes up: 9 sec     | 421 sec
Moved remote dir                      | Queued 50 copies down: 31 sec, 50 deletes down: <1 sec |  33 sec
Delete local dir                      | Queued 50 deletes up: 9 sec                            |  13 sec

This next data is from a user's application. They had ~400GB of data
over 1.96 million files being sync'ed between a Windows local disk and some
remote cloud. The file full path length was on average 35 characters
(which factors into load time and RAM required).

- Loading the prior listing into memory (1.96 million files, listing file
  size 140 MB) took ~30 sec and occupied about 1 GB of RAM.
- Getting a fresh listing of the local file system (producing the
  140 MB output file) took about XXX sec.
- Getting a fresh listing of the remote file system (producing the 140 MB
  output file) took about XXX sec. The network download speed was measured
  at XXX Mb/s.
- Once the prior and current Path1 and Path2 listings were loaded (a total
  of four to be loaded, two at a time), determining the deltas was pretty
  quick (a few seconds for this test case), and the transfer time for any
  files to be copied was dominated by the network bandwidth.

## References

rclone's bisync implementation was derived from
the [rclonesync-V2](https://github.com/cjnaz/rclonesync-V2) project,
including documentation and test mechanisms,
with [@cjnaz](https://github.com/cjnaz)'s full support and encouragement.

`rclone bisync` is similar in nature to a range of other projects:

- [unison](https://github.com/bcpierce00/unison)
- [syncthing](https://github.com/syncthing/syncthing)
- [cjnaz/rclonesync](https://github.com/cjnaz/rclonesync-V2)
- [ConorWilliams/rsinc](https://github.com/ConorWilliams/rsinc)
- [jwink3101/syncrclone](https://github.com/Jwink3101/syncrclone)
- [DavideRossi/upback](https://github.com/DavideRossi/upback)

Bisync adopts the differential synchronization technique, which is
based on keeping history of changes performed by both synchronizing sides.
See the _Dual Shadow Method_ section in the
[Neil Fraser's article](https://neil.fraser.name/writing/sync/).

Also note a number of academic publications by
[Benjamin Pierce](http://www.cis.upenn.edu/%7Ebcpierce/papers/index.shtml#File%20Synchronization)
about _Unison_ and synchronization in general.
