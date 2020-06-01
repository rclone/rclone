---
title: "Filtering"
description: "Filtering, includes and excludes"
---

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
lines starting with `;` or `#` are read without any processing. [rclone lsf](/commands/rclone_lsf/)
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
