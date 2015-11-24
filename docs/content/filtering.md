---
title: "Filtering"
description: "Filtering, includes and excludes"
date: "2015-09-27"
---

# Filtering, includes and excludes #

Rclone has a sophisticated set of include and exclude rules. Some of
these are based on patterns and some on other things like file size.

The filters are applied for the `copy`, `sync`, `move`, `ls`, `lsl`,
`md5sum`, `size` and `check` operations.  Note that `purge` does not
obey the filters.

Each path as it passes through rclone is matched against the include
and exclude rules.  The paths are matched without a leading `/`.

For example the files might be passed to the matching engine like this

  * `file1.jpg`
  * `file2.jpg`
  * `directory/file3.jpg`

## Patterns ##

The patterns used to match files for inclusion or exclusion are based
on "file globs" as used by the unix shell.

If the pattern starts with a `/` then it only matches at the top level
of the directory tree.  If it doesn't start with `/` then it is
matched starting at the end of the path, but it will only match a
complete path element.

    file.jpg  - matches "file.jpg"
              - matches "directory/file.jpg"
              - doesn't match "afile.jpg"
              - doesn't match "directory/afile.jpg"
    /file.jpg - matches "file.jpg"
              - doesn't match "afile.jpg"
              - doesn't match "directory/file.jpg"

A `*` matches anything but not a `/`.

    *.jpg  - matches "file.jpg"
           - matches "directory/file.jpg"
           - doesn't match "file.jpg/anotherfile.png"

Use `**` to match anything, including slashes.

    dir/** - matches "dir/file.jpg"
           - matches "dir/dir1/dir2/file.jpg"
           - doesn't match "directory/file.jpg"
           - doesn't match "adir/file.jpg"

A `?` matches any character except a slash `/`.

    l?ss  - matches "less"
          - matches "lass"
          - doesn't match "floss"

A `[` and `]` together make a a character class, such as `[a-z]` or
`[aeiou]` or `[[:alpha:]]`.  See the [go regexp
docs](https://golang.org/pkg/regexp/syntax/) for more info on these.

    h[ae]llo - matches "hello"
             - matches "hallo"
             - doesn't match "hullo"

A `{` and `}` define a choice between elements.  It should contain a
comma seperated list of patterns, any of which might match.  These
patterns can contain wildcards.

    {one,two}_potato - matches "one_potato"
                     - matches "two_potato"
                     - doesn't match "three_potato"
                     - doesn't match "_potato"

Special characters can be escaped with a `\` before them.

    \*.jpg       - matches "*.jpg"
    \\.jpg       - matches "\.jpg"
    \[one\].jpg  - matches "[one].jpg"
  
### Differences between rsync and rclone patterns ###

Rclone implements bash style `{a,b,c}` glob matching which rsync doesn't.

Rclone ignores `/` at the end of a pattern.

Rclone always does a wildcard match so `\` must always escape a `\`.

## How the rules are used ##

Rclone maintains a list of include rules and exclude rules.

Each file is matched in order against the list until it finds a match.
The file is then included or excluded according to the rule type.

If the matcher falls off the bottom of the list then the path is
included.

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

## Adding filtering rules ##

Filtering rules are added with the following command line flags.

### `--exclude` - Exclude files matching pattern ###

Add a single exclude rule with `--exclude`.

Eg `--exclude *.bak` to exclude all bak files from the sync.

### `--exclude-from` - Read exclude patterns from file ###

Add exclude rules from a file.

Prepare a file like this `exclude-file.txt`

    # a sample exclude rule file
    *.bak
    file2.jpg

Then use as `--exclude-from exclude-file.txt`.  This will sync all
files except those ending in `bak` and `file2.jpg`.

This is useful if you have a lot of rules.

### `--include` - Include files matching pattern ###

Add a single include rule with `--include`.

Eg `--include *.{png,jpg}` to include all `png` and `jpg` files in the
backup and no others.

This adds an implicit `--exclude *` at the end of the filter list.

### `--include-from` - Read include patterns from file ###

Add include rules from a file.

Prepare a file like this `include-file.txt`

    # a sample include rule file
    *.jpg
    *.png
    file2.avi

Then use as `--include-from include-file.txt`.  This will sync all
`jpg`, `png` files and `file2.avi`.

This is useful if you have a lot of rules.

This adds an implicit `--exclude *` at the end of the filter list.

### `--filter` - Add a file-filtering rule ###

This can be used to add a single include or exclude rule.  Include
rules start with `+ ` and exclude rules start with `- `.  A special
rule called `!` can be used to clear the existing rules.

Eg `--filter "- *.bak"` to exclude all bak files from the sync.

### `--filter-from` - Read filtering patterns from a file ###

Add include/exclude rules from a file.

Prepare a file like this `filter-file.txt`

    # a sample exclude rule file
    - secret*.jpg
    + *.jpg
    + *.png
    + file2.avi
    # exclude everything else
    - *

Then use as `--filter-from filter-file.txt`.  The rules are processed
in the order that they are defined.

This example will include all `jpg` and `png` files, exclude any files
matching `secret*.jpg` and include `file2.avi`.  Everything else will
be excluded from the sync.

### `--files-from` - Read list of source-file names ###

This reads a list of file names from the file passed in and **only**
these files are transferred.  The filtering rules are ignored
completely if you use this option.

Prepare a file like this `files-from.txt`

    # comment
    file1.jpg
    file2.jpg

Then use as `--files-from files-from.txt`.  This will only transfer
`file1.jpg` and `file2.jpg` providing they exist.

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

### `--dump-filters` - dump the filters to the output ###

This dumps the defined filters to the output as regular expressions.

Useful for debugging.

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
