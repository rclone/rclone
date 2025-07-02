---
title: "Rclone Filtering"
description: "Rclone filtering, includes and excludes"
versionIntroduced: "v1.22"
---

# Filtering, includes and excludes

Filter flags determine which files rclone `sync`, `move`, `ls`, `lsl`,
`md5sum`, `sha1sum`, `size`, `delete`, `check` and similar commands
apply to.

They are specified in terms of path/file name patterns; path/file
lists; file age and size, or presence of a file in a directory. Bucket
based remotes without the concept of directory apply filters to object
key, age and size in an analogous way.

Rclone `purge` does not obey filters.

To test filters without risk of damage to data, apply them to `rclone
ls`, or with the `--dry-run` and `-vv` flags.

Rclone filter patterns can only be used in filter command line options, not
in the specification of a remote.

E.g. `rclone copy "remote:dir*.jpg" /path/to/dir` does not have a filter effect.
`rclone copy remote:dir /path/to/dir --include "*.jpg"` does.

**Important** Avoid mixing any two of `--include...`, `--exclude...` or
`--filter...` flags in an rclone command. The results might not be what
you expect. Instead use a `--filter...` flag.

## Patterns for matching path/file names

### Pattern syntax {#patterns}

Here is a formal definition of the pattern syntax,
[examples](#examples) are below.

Rclone matching rules follow a glob style:

    *         matches any sequence of non-separator (/) characters
    **        matches any sequence of characters including / separators
    ?         matches any single non-separator (/) character
    [ [ ! ] { character-range } ]
              character class (must be non-empty)
    { pattern-list }
              pattern alternatives
    {{ regexp }}
              regular expression to match
    c         matches character c (c != *, **, ?, \, [, {, })
    \c        matches reserved character c (c = *, **, ?, \, [, {, }) or character class

character-range:

    c         matches character c (c != \, -, ])
    \c        matches reserved character c (c = \, -, ])
    lo - hi   matches character c for lo <= c <= hi

pattern-list:

    pattern { , pattern }
              comma-separated (without spaces) patterns

character classes (see [Go regular expression reference](https://golang.org/pkg/regexp/syntax/)) include:

    Named character classes (e.g. [\d], [^\d], [\D], [^\D])
    Perl character classes (e.g. \s, \S, \w, \W)
    ASCII character classes (e.g. [[:alnum:]], [[:alpha:]], [[:punct:]], [[:xdigit:]])

regexp for advanced users to insert a regular expression - see [below](#regexp) for more info:

    Any re2 regular expression not containing `}}`

If the filter pattern starts with a `/` then it only matches
at the top level of the directory tree,
**relative to the root of the remote** (not necessarily the root
of the drive). If it does not start with `/` then it is matched
starting at the **end of the path/file name** but it only matches
a complete path element - it must match from a `/`
separator or the beginning of the path/file.

    file.jpg   - matches "file.jpg"
               - matches "directory/file.jpg"
               - doesn't match "afile.jpg"
               - doesn't match "directory/afile.jpg"
    /file.jpg  - matches "file.jpg" in the root directory of the remote
               - doesn't match "afile.jpg"
               - doesn't match "directory/file.jpg"

The top level of the remote might not be the top level of the drive.

E.g. for a Microsoft Windows local directory structure

    F:
    ├── bkp
    ├── data
    │   ├── excl
    │   │   ├── 123.jpg
    │   │   └── 456.jpg
    │   ├── incl
    │   │   └── document.pdf

To copy the contents of folder `data` into folder `bkp` excluding the contents of subfolder
`excl`the following command treats `F:\data` and `F:\bkp` as top level for filtering.

`rclone copy F:\data\ F:\bkp\ --exclude=/excl/**`

**Important** Use `/` in path/file name patterns and not `\` even if
running on Microsoft Windows.

Simple patterns are case sensitive unless the `--ignore-case` flag is used.

Without `--ignore-case` (default)

    potato - matches "potato"
           - doesn't match "POTATO"

With `--ignore-case`

    potato - matches "potato"
           - matches "POTATO"

## Using regular expressions in filter patterns {#regexp}

The syntax of filter patterns is glob style matching (like `bash`
uses) to make things easy for users. However this does not provide
absolute control over the matching, so for advanced users rclone also
provides a regular expression syntax.

The regular expressions used are as defined in the [Go regular
expression reference](https://golang.org/pkg/regexp/syntax/). Regular
expressions should be enclosed in `{{` `}}`. They will match only the
last path segment if the glob doesn't start with `/` or the whole path
name if it does. Note that rclone does not attempt to parse the
supplied regular expression, meaning that using any regular expression
filter will prevent rclone from using [directory filter rules](#directory_filter),
as it will instead check every path against
the supplied regular expression(s).

Here is how the `{{regexp}}` is transformed into an full regular
expression to match the entire path:

    {{regexp}}  becomes (^|/)(regexp)$
    /{{regexp}} becomes ^(regexp)$

Regexp syntax can be mixed with glob syntax, for example

    *.{{jpe?g}} to match file.jpg, file.jpeg but not file.png

You can also use regexp flags - to set case insensitive, for example

    *.{{(?i)jpg}} to match file.jpg, file.JPG but not file.png

Be careful with wildcards in regular expressions - you don't want them
to match path separators normally. To match any file name starting
with `start` and ending with `end` write

    {{start[^/]*end\.jpg}}

Not

    {{start.*end\.jpg}}

Which will match a directory called `start` with a file called
`end.jpg` in it as the `.*` will match `/` characters.

Note that you can use `-vv --dump filters` to show the filter patterns
in regexp format - rclone implements the glob patterns by transforming
them into regular expressions.

## Filter pattern examples {#examples}

| Description | Pattern | Matches | Does not match |
| ----------- |-------- | ------- | -------------- |
| Wildcard    | `*.jpg` | `/file.jpg`     | `/file.png`    |
|             |         | `/dir/file.jpg` | `/dir/file.png` |
| Rooted      | `/*.jpg` | `/file.jpg`    | `/file.png`    |
|             |          | `/file2.jpg`    | `/dir/file.jpg` |
| Alternates  | `*.{jpg,png}` | `/file.jpg`     | `/file.gif`    |
|             |         | `/dir/file.png` | `/dir/file.gif` |
| Path Wildcard | `dir/**` | `/dir/anyfile`     | `file.png`    |
|             |          | `/subdir/dir/subsubdir/anyfile` | `/subdir/file.png` |
| Any Char    | `*.t?t` | `/file.txt`     | `/file.qxt`    |
|             |         | `/dir/file.tzt` | `/dir/file.png` |
| Range       | `*.[a-z]` | `/file.a`     | `/file.0`    |
|             |         | `/dir/file.b` | `/dir/file.1` |
| Escape      | `*.\?\?\?` | `/file.???`     | `/file.abc`    |
|             |         | `/dir/file.???` | `/dir/file.def` |
| Class       | `*.\d\d\d` | `/file.012`     | `/file.abc`    |
|             |         | `/dir/file.345` | `/dir/file.def` |
| Regexp      | `*.{{jpe?g}}` | `/file.jpeg`     | `/file.png`    |
|             |         | `/dir/file.jpg` | `/dir/file.jpeeg` |
| Rooted Regexp | `/{{.*\.jpe?g}}` | `/file.jpeg`  | `/file.png`    |
|             |                  | `/file.jpg`   | `/dir/file.jpg` |

## How filter rules are applied to files {#how-filter-rules-work}

Rclone path/file name filters are made up of one or more of the following flags:

  * `--include`
  * `--include-from`
  * `--exclude`
  * `--exclude-from`
  * `--filter`
  * `--filter-from`

There can be more than one instance of individual flags.

Rclone internally uses a combined list of all the include and exclude
rules. The order in which rules are processed can influence the result
of the filter.

All flags of the same type are processed together in the order
above, regardless of what order the different types of flags are
included on the command line.

Multiple instances of the same flag are processed from left
to right according to their position in the command line.

To mix up the order of processing includes and excludes use `--filter...`
flags.

Within `--include-from`, `--exclude-from` and `--filter-from` flags
rules are processed from top to bottom of the referenced file.

If there is an `--include` or `--include-from` flag specified, rclone
implies a `- **` rule which it adds to the bottom of the internal rule
list. Specifying a `+` rule with a `--filter...` flag does not imply
that rule.

Each path/file name passed through rclone is matched against the
combined filter list. At first match to a rule the path/file name
is included or excluded and no further filter rules are processed for
that path/file.

If rclone does not find a match, after testing against all rules
(including the implied rule if appropriate), the path/file name
is included.

Any path/file included at that stage is processed by the rclone
command.

`--files-from` and `--files-from-raw` flags over-ride and cannot be
combined with other filter options.

To see the internal combined rule list, in regular expression form,
for a command add the `--dump filters` flag. Running an rclone command
with `--dump filters` and `-vv` flags lists the internal filter elements
and shows how they are applied to each source path/file. There is not
currently a means provided to pass regular expression filter options into
rclone directly though character class filter rules contain character
classes. [Go regular expression reference](https://golang.org/pkg/regexp/syntax/)

### How filter rules are applied to directories {#directory_filter}

Rclone commands are applied to path/file names not
directories. The entire contents of a directory can be matched
to a filter by the pattern `directory/*` or recursively by
`directory/**`.

Directory filter rules are defined with a closing `/` separator.

E.g. `/directory/subdirectory/` is an rclone directory filter rule.

Rclone commands can use directory filter rules to determine whether they
recurse into subdirectories. This potentially optimises access to a remote
by avoiding listing unnecessary directories. Whether optimisation is
desirable depends on the specific filter rules and source remote content.

If any [regular expression filters](#regexp) are in use, then no
directory recursion optimisation is possible, as rclone must check
every path against the supplied regular expression(s).

Directory recursion optimisation occurs if either:

* A source remote does not support the rclone `ListR` primitive. local,
sftp, Microsoft OneDrive and WebDAV do not support `ListR`. Google
Drive and most bucket type storage do. [Full list](https://rclone.org/overview/#optional-features)

* On other remotes (those that support `ListR`), if the rclone command is not naturally recursive, and
provided it is not run with the `--fast-list` flag. `ls`, `lsf -R` and
`size` are naturally recursive but `sync`, `copy` and `move` are not.

* Whenever the `--disable ListR` flag is applied to an rclone command.

Rclone commands imply directory filter rules from path/file filter
rules. To view the directory filter rules rclone has implied for a
command specify the `--dump filters` flag.

E.g. for an include rule

    /a/*.jpg

Rclone implies the directory include rule

    /a/

Directory filter rules specified in an rclone command can limit
the scope of an rclone command but path/file filters still have
to be specified.

E.g. `rclone ls remote: --include /directory/` will not match any
files. Because it is an `--include` option the `--exclude **` rule
is implied, and the `/directory/` pattern serves only to optimise
access to the remote by ignoring everything outside of that directory.

E.g. `rclone ls remote: --filter-from filter-list.txt` with a file
`filter-list.txt`:

    - /dir1/
    - /dir2/
    + *.pdf
    - **

All files in directories `dir1` or `dir2` or their subdirectories
are completely excluded from the listing. Only files of suffix
`pdf` in the root of `remote:` or its subdirectories are listed.
The `- **` rule prevents listing of any path/files not previously
matched by the rules above.

Option `exclude-if-present` creates a directory exclude rule based
on the presence of a file in a directory and takes precedence over
other rclone directory filter rules.

When using pattern list syntax, if a pattern item contains either
`/` or `**`, then rclone will not able to imply a directory filter rule
from this pattern list.

E.g. for an include rule

    {dir1/**,dir2/**}

Rclone will match files below directories `dir1` or `dir2` only,
but will not be able to use this filter to exclude a directory `dir3`
from being traversed.

Directory recursion optimisation may affect performance, but normally
not the result. One exception to this is sync operations with option
`--create-empty-src-dirs`, where any traversed empty directories will
be created. With the pattern list example `{dir1/**,dir2/**}` above,
this would create an empty directory `dir3` on destination (when it exists
on source). Changing the filter to `{dir1,dir2}/**`, or splitting it into
two include rules `--include dir1/** --include dir2/**`, will match the
same files while also filtering directories, with the result that an empty
directory `dir3` will no longer be created.

### `--exclude` - Exclude files matching pattern

Excludes path/file names from an rclone command based on a single exclude
rule.

This flag can be repeated. See above for the order filter flags are
processed in.

`--exclude` should not be used with `--include`, `--include-from`,
`--filter` or `--filter-from` flags.

`--exclude` has no effect when combined with `--files-from` or
`--files-from-raw` flags.

E.g. `rclone ls remote: --exclude *.bak` excludes all .bak files
from listing.

E.g. `rclone size remote: --exclude "/dir/**"` returns the total size of
all files on `remote:` excluding those in root directory `dir` and sub
directories.

E.g. on Microsoft Windows `rclone ls remote: --exclude "*\[{JP,KR,HK}\]*"`
lists the files in `remote:` without `[JP]` or `[KR]` or `[HK]` in
their name. Quotes prevent the shell from interpreting the `\`
characters.`\` characters escape the `[` and `]` so an rclone filter
treats them literally rather than as a character-range. The `{` and `}`
define an rclone pattern list. For other operating systems single quotes are
required ie `rclone ls remote: --exclude '*\[{JP,KR,HK}\]*'`

### `--exclude-from` - Read exclude patterns from file

Excludes path/file names from an rclone command based on rules in a
named file. The file contains a list of remarks and pattern rules.

For an example `exclude-file.txt`:

    # a sample exclude rule file
    *.bak
    file2.jpg

`rclone ls remote: --exclude-from exclude-file.txt` lists the files on
`remote:` except those named `file2.jpg` or with a suffix `.bak`. That is
equivalent to `rclone ls remote: --exclude file2.jpg --exclude "*.bak"`.

This flag can be repeated. See above for the order filter flags are
processed in.

The `--exclude-from` flag is useful where multiple exclude filter rules
are applied to an rclone command.

`--exclude-from` should not be used with `--include`, `--include-from`,
`--filter` or `--filter-from` flags.

`--exclude-from` has no effect when combined with `--files-from` or
`--files-from-raw` flags.

`--exclude-from` followed by `-` reads filter rules from standard input.

### `--include` - Include files matching pattern

Adds a single include rule based on path/file names to an rclone
command.

This flag can be repeated. See above for the order filter flags are
processed in.

`--include` has no effect when combined with `--files-from` or
`--files-from-raw` flags.

`--include` implies `--exclude **` at the end of an rclone internal
filter list. Therefore if you mix `--include` and `--include-from`
flags with `--exclude`, `--exclude-from`, `--filter` or `--filter-from`,
you must use include rules for all the files you want in the include
statement. For more flexibility use the `--filter-from` flag.

E.g. `rclone ls remote: --include "*.{png,jpg}"` lists the files on
`remote:` with suffix `.png` and `.jpg`. All other files are excluded.

E.g. multiple rclone copy commands can be combined with `--include` and a
pattern-list.

    rclone copy /vol1/A remote:A
    rclone copy /vol1/B remote:B

is equivalent to:

    rclone copy /vol1 remote: --include "{A,B}/**"

E.g. `rclone ls remote:/wheat --include "??[^[:punct:]]*"` lists the
files `remote:` directory `wheat` (and subdirectories) whose third
character is not punctuation. This example uses
an [ASCII character class](https://golang.org/pkg/regexp/syntax/).

### `--include-from` - Read include patterns from file

Adds path/file names to an rclone command based on rules in a
named file. The file contains a list of remarks and pattern rules.

For an example `include-file.txt`:

    # a sample include rule file
    *.jpg
    file2.avi

`rclone ls remote: --include-from include-file.txt` lists the files on
`remote:` with name `file2.avi` or suffix `.jpg`. That is equivalent to
`rclone ls remote: --include file2.avi --include "*.jpg"`.

This flag can be repeated. See above for the order filter flags are
processed in.

The `--include-from` flag is useful where multiple include filter rules
are applied to an rclone command.

`--include-from` implies `--exclude **` at the end of an rclone internal
filter list. Therefore if you mix `--include` and `--include-from`
flags with `--exclude`, `--exclude-from`, `--filter` or `--filter-from`,
you must use include rules for all the files you want in the include
statement. For more flexibility use the `--filter-from` flag.

`--include-from` has no effect when combined with `--files-from` or
`--files-from-raw` flags.

`--include-from` followed by `-` reads filter rules from standard input.

### `--filter` - Add a file-filtering rule

Specifies path/file names to an rclone command, based on a single
include or exclude rule, in `+` or `-` format.

This flag can be repeated. See above for the order filter flags are
processed in.

`--filter +` differs from `--include`. In the case of `--include` rclone
implies an `--exclude *` rule which it adds to the bottom of the internal rule
list. `--filter...+` does not imply
that rule.

`--filter` has no effect when combined with `--files-from` or
`--files-from-raw` flags.

`--filter` should not be used with `--include`, `--include-from`,
`--exclude` or `--exclude-from` flags.

E.g. `rclone ls remote: --filter "- *.bak"` excludes all `.bak` files
from a list of `remote:`.

### `--filter-from` - Read filtering patterns from a file

Adds path/file names to an rclone command based on rules in a
named file. The file contains a list of remarks and pattern rules. Include
rules start with `+ ` and exclude rules with `- `. `!` clears existing
rules. Rules are processed in the order they are defined.

This flag can be repeated. See above for the order filter flags are
processed in.

Arrange the order of filter rules with the most restrictive first and
work down.

Lines starting with # or ; are ignored, and can be used to write comments. Inline comments are not supported. _Use `-vv --dump filters` to see how they appear in the final regexp._

E.g. for `filter-file.txt`:

    # a sample filter rule file
    - secret*.jpg
    + *.jpg
    + *.png
    + file2.avi
    - /dir/tmp/** # WARNING! This text will be treated as part of the path.
    - /dir/Trash/**
    + /dir/**
    # exclude everything else
    - *

`rclone ls remote: --filter-from filter-file.txt` lists the path/files on
`remote:` including all `jpg` and `png` files, excluding any
matching `secret*.jpg` and including `file2.avi`.  It also includes
everything in the directory `dir` at the root of `remote`, except
`remote:dir/Trash` which it excludes.  Everything else is excluded.


E.g. for an alternative `filter-file.txt`:

    - secret*.jpg
    + *.jpg
    + *.png
    + file2.avi
    - *

Files `file1.jpg`, `file3.png` and `file2.avi` are listed whilst
`secret17.jpg` and files without the suffix `.jpg` or `.png` are excluded.

E.g. for an alternative `filter-file.txt`:

    + *.jpg
    + *.gif
    !
    + 42.doc
    - *

Only file 42.doc is listed. Prior rules are cleared by the `!`.

### `--files-from` - Read list of source-file names

Adds path/files to an rclone command from a list in a named file.
Rclone processes the path/file names in the order of the list, and
no others.

Other filter flags (`--include`, `--include-from`, `--exclude`,
`--exclude-from`, `--filter` and `--filter-from`) are ignored when
`--files-from` is used.

`--files-from` expects a list of files as its input. Leading or
trailing whitespace is stripped from the input lines. Lines starting
with `#` or `;` are ignored.

`--files-from` followed by `-` reads the list of files from standard input.

Rclone commands with a `--files-from` flag traverse the remote,
treating the names in `--files-from` as a set of filters.

If the `--no-traverse` and `--files-from` flags are used together
an rclone command does not traverse the remote. Instead it addresses
each path/file named in the file individually. For each path/file name, that
requires typically 1 API call. This can be efficient for a short `--files-from`
list and a remote containing many files.

Rclone commands do not error if any names in the `--files-from` file are
missing from the source remote.

The `--files-from` flag can be repeated in a single rclone command to
read path/file names from more than one file. The files are read from left
to right along the command line.

Paths within the `--files-from` file are interpreted as starting
with the root specified in the rclone command.  Leading `/` separators are
ignored. See [--files-from-raw](#files-from-raw-read-list-of-source-file-names-without-any-processing) if
you need the input to be processed in a raw manner.

E.g. for a file `files-from.txt`:

    # comment
    file1.jpg
    subdir/file2.jpg

`rclone copy --files-from files-from.txt /home/me/pics remote:pics`
copies the following, if they exist, and only those files.

    /home/me/pics/file1.jpg        → remote:pics/file1.jpg
    /home/me/pics/subdir/file2.jpg → remote:pics/subdir/file2.jpg

E.g. to copy the following files referenced by their absolute paths:

    /home/user1/42
    /home/user1/dir/ford
    /home/user2/prefect

First find a common subdirectory - in this case `/home`
and put the remaining files in `files-from.txt` with or without
leading `/`, e.g.

    user1/42
    user1/dir/ford
    user2/prefect

Then copy these to a remote:

    rclone copy --files-from files-from.txt /home remote:backup

The three files are transferred as follows:

    /home/user1/42       → remote:backup/user1/important
    /home/user1/dir/ford → remote:backup/user1/dir/file
    /home/user2/prefect  → remote:backup/user2/stuff

Alternatively if `/` is chosen as root `files-from.txt` will be:

    /home/user1/42
    /home/user1/dir/ford
    /home/user2/prefect

The copy command will be:

    rclone copy --files-from files-from.txt / remote:backup

Then there will be an extra `home` directory on the remote:

    /home/user1/42       → remote:backup/home/user1/42
    /home/user1/dir/ford → remote:backup/home/user1/dir/ford
    /home/user2/prefect  → remote:backup/home/user2/prefect

### `--files-from-raw` - Read list of source-file names without any processing

This flag is the same as `--files-from` except that input is read in a
raw manner. Lines with leading / trailing whitespace, and lines starting
with `;` or `#` are read without any processing. [rclone lsf](/commands/rclone_lsf/) has
a compatible format that can be used to export file lists from remotes for
input to `--files-from-raw`.

### `--ignore-case` - make searches case insensitive

By default, rclone filter patterns are case sensitive. The `--ignore-case`
flag makes all of the filters patterns on the command line case
insensitive.

E.g. `--include "zaphod.txt"` does not match a file `Zaphod.txt`. With
`--ignore-case` a match is made.

## Quoting shell metacharacters

Rclone commands with filter patterns containing shell metacharacters may
not as work as expected in your shell and may require quoting.

E.g. linux, OSX (`*` metacharacter)

  * `--include \*.jpg`
  * `--include '*.jpg'`
  * `--include='*.jpg'`

Microsoft Windows expansion is done by the command, not shell, so
`--include *.jpg` does not require quoting.

If the rclone error
`Command .... needs .... arguments maximum: you provided .... non flag arguments:`
is encountered, the cause is commonly spaces within the name of a
remote or flag value. The fix then is to quote values containing spaces.

## Other filters

### `--min-size` - Don't transfer any file smaller than this

Controls the minimum size file within the scope of an rclone command.
Default units are `KiB` but abbreviations `B`, `K`, `M`, `G`, `T` or `P` are valid.

E.g. `rclone ls remote: --min-size 50k` lists files on `remote:` of 50 KiB
size or larger.

See [the size option docs](/docs/#size-option) for more info.

### `--max-size` - Don't transfer any file larger than this

Controls the maximum size file within the scope of an rclone command.
Default units are `KiB` but abbreviations `B`, `K`, `M`, `G`, `T` or `P` are valid.

E.g. `rclone ls remote: --max-size 1G` lists files on `remote:` of 1 GiB
size or smaller.

See [the size option docs](/docs/#size-option) for more info.

### `--max-age` - Don't transfer any file older than this

Controls the maximum age of files within the scope of an rclone command.

`--max-age` applies only to files and not to directories.

E.g. `rclone ls remote: --max-age 2d` lists files on `remote:` of 2 days
old or less.

See [the time option docs](/docs/#time-option) for valid formats.

### `--min-age` - Don't transfer any file younger than this

Controls the minimum age of files within the scope of an rclone command.
(see `--max-age` for valid formats)

`--min-age` applies only to files and not to directories.

E.g. `rclone ls remote: --min-age 2d` lists files on `remote:` of 2 days
old or more.

See [the time option docs](/docs/#time-option) for valid formats.

### `--hash-filter` - Deterministically select a subset of files {#hash-filter}

The `--hash-filter` flag enables selecting a deterministic subset of files, useful for:

1. Running large sync operations across multiple machines.
2. Checking a subset of files for bitrot.
3. Any other operations where a sample of files is required.

#### Syntax

The flag takes two parameters expressed as a fraction:

```
--hash-filter K/N
```

- `N`: The total number of partitions (must be a positive integer).
- `K`: The specific partition to select (an integer from `0` to `N`).

For example:
- `--hash-filter 1/3`: Selects the first third of the files.
- `--hash-filter 2/3` and `--hash-filter 3/3`: Select the second and third partitions, respectively.

Each partition is non-overlapping, ensuring all files are covered without duplication.

#### Random Partition Selection

Use `@` as `K` to randomly select a partition:

```
--hash-filter @/M
```

For example, `--hash-filter @/3` will randomly select a number between 0 and 2. This will stay constant across retries.

#### How It Works

- Rclone takes each file's full path, normalizes it to lowercase, and applies Unicode normalization.
- It then hashes the normalized path into a 64 bit number.
- The hash result is reduced modulo `N` to assign the file to a partition.
- If the calculated partition does not match `K` the file is excluded.
- Other filters may apply if the file is not excluded.

**Important:** Rclone will traverse all directories to apply the filter.

#### Usage Notes

- Safe to use with `rclone sync`; source and destination selections will match.
- **Do not** use with `--delete-excluded`, as this could delete unselected files.
- Ignored if `--files-from` is used.

#### Examples

##### Dividing files into 4 partitions

Assuming the current directory contains `file1.jpg` through `file9.jpg`:

```
$ rclone lsf --hash-filter 0/4 .
file1.jpg
file5.jpg

$ rclone lsf --hash-filter 1/4 .
file3.jpg
file6.jpg
file9.jpg

$ rclone lsf --hash-filter 2/4 .
file2.jpg
file4.jpg

$ rclone lsf --hash-filter 3/4 .
file7.jpg
file8.jpg

$ rclone lsf --hash-filter 4/4 . # the same as --hash-filter 0/4
file1.jpg
file5.jpg
```

##### Syncing the first quarter of files

```
rclone sync --hash-filter 1/4 source:path destination:path
```

##### Checking a random 1% of files for integrity

```
rclone check --download --hash-filter @/100 source:path destination:path
```

## Other flags

### `--delete-excluded` - Delete files on dest excluded from sync

**Important** this flag is dangerous to your data - use with `--dry-run`
and `-v` first.

In conjunction with `rclone sync`, `--delete-excluded` deletes any files
on the destination which are excluded from the command.

E.g. the scope of `rclone sync --interactive A: B:` can be restricted:

    rclone --min-size 50k --delete-excluded sync A: B:

All files on `B:` which are less than 50 KiB are deleted
because they are excluded from the rclone sync command.

### `--dump filters` - dump the filters to the output

Dumps the defined filters to standard output in regular expression
format.

Useful for debugging.

## Exclude directory based on a file

The `--exclude-if-present` flag controls whether a directory is
within the scope of an rclone command based on the presence of a
named file within it. The flag can be repeated to check for
multiple file names, presence of any of them will exclude the
directory.

This flag has a priority over other filter flags.

E.g. for the following directory structure:

    dir1/file1
    dir1/dir2/file2
    dir1/dir2/dir3/file3
    dir1/dir2/dir3/.ignore

The command `rclone ls --exclude-if-present .ignore dir1` does
not list `dir3`, `file3` or `.ignore`.

## Metadata filters {#metadata}

The metadata filters work in a very similar way to the normal file
name filters, except they match [metadata](/docs/#metadata) on the
object.

The metadata should be specified as `key=value` patterns. This may be
wildcarded using the normal [filter patterns](#patterns) or [regular
expressions](#regexp).

For example if you wished to list only local files with a mode of
`100664` you could do that with:

    rclone lsf -M --files-only --metadata-include "mode=100664" .

Or if you wished to show files with an `atime`, `mtime` or `btime` at a given date:

    rclone lsf -M --files-only --metadata-include "[abm]time=2022-12-16*" .

Like file filtering, metadata filtering only applies to files not to
directories.

The filters can be applied using these flags.

- `--metadata-include`      - Include metadatas matching pattern
- `--metadata-include-from` - Read metadata include patterns from file (use - to read from stdin)
- `--metadata-exclude`      - Exclude metadatas matching pattern
- `--metadata-exclude-from` - Read metadata exclude patterns from file (use - to read from stdin)
- `--metadata-filter`       - Add a metadata filtering rule
- `--metadata-filter-from`  - Read metadata filtering patterns from a file (use - to read from stdin)

Each flag can be repeated. See the section on [how filter rules are
applied](#how-filter-rules-work) for more details - these flags work
in an identical way to the file name filtering flags, but instead of
file name patterns have metadata patterns.


## Common pitfalls

The most frequent filter support issues on
the [rclone forum](https://forum.rclone.org/) are:

* Not using paths relative to the root of the remote
* Not using `/` to match from the root of a remote
* Not using `**` to match the contents of a directory

