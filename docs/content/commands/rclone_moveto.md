---
title: "rclone moveto"
description: "Move file or directory from source to dest."
versionIntroduced: v1.35
# autogenerated - DO NOT EDIT, instead edit the source code in cmd/moveto/ and as part of making a release run "make commanddocs"
---
# rclone moveto

Move file or directory from source to dest.

## Synopsis

If source:path is a file or directory then it moves it to a file or
directory named dest:path.

This can be used to rename files or upload single files to other than
their existing name.  If the source is a directory then it acts exactly
like the [move](/commands/rclone_move/) command.

So

    rclone moveto src dst

where src and dst are rclone paths, either remote:path or
/path/to/local or C:\windows\path\if\on\windows.

This will:

    if src is file
        move it to dst, overwriting an existing file if it exists
    if src is directory
        move it to dst, overwriting existing files if they exist
        see move command for full details

This doesn't transfer files that are identical on src and dst, testing
by size and modification time or MD5SUM.  src will be deleted on
successful transfer.

**Important**: Since this can cause data loss, test first with the
`--dry-run` or the `--interactive`/`-i` flag.

**Note**: Use the `-P`/`--progress` flag to view real-time transfer statistics.


```
rclone moveto source:path dest:path [flags]
```

## Options

```
  -h, --help   help for moveto
```

Options shared with other commands are described next.
See the [global flags page](/flags/) for global options not listed here.

### Copy Options

Flags for anything which can copy a file

```
      --check-first                                 Do all the checks before starting transfers
  -c, --checksum                                    Check for changes with size & checksum (if available, or fallback to size only)
      --compare-dest stringArray                    Include additional server-side paths during comparison
      --copy-dest stringArray                       Implies --compare-dest but also copies files from paths into destination
      --cutoff-mode HARD|SOFT|CAUTIOUS              Mode to stop transfers when reaching the max transfer limit HARD|SOFT|CAUTIOUS (default HARD)
      --ignore-case-sync                            Ignore case when synchronizing
      --ignore-checksum                             Skip post copy check of checksums
      --ignore-existing                             Skip all files that exist on destination
      --ignore-size                                 Ignore size when skipping use modtime or checksum
  -I, --ignore-times                                Don't skip items that match size and time - transfer all unconditionally
      --immutable                                   Do not modify files, fail if existing files have been modified
      --inplace                                     Download directly to destination file instead of atomic download to temp/rename
  -l, --links                                       Translate symlinks to/from regular files with a '.rclonelink' extension
      --max-backlog int                             Maximum number of objects in sync or check backlog (default 10000)
      --max-duration Duration                       Maximum duration rclone will transfer data for (default 0s)
      --max-transfer SizeSuffix                     Maximum size of data to transfer (default off)
  -M, --metadata                                    If set, preserve metadata when copying objects
      --modify-window Duration                      Max time diff to be considered the same (default 1ns)
      --multi-thread-chunk-size SizeSuffix          Chunk size for multi-thread downloads / uploads, if not set by filesystem (default 64Mi)
      --multi-thread-cutoff SizeSuffix              Use multi-thread downloads for files above this size (default 256Mi)
      --multi-thread-streams int                    Number of streams to use for multi-thread downloads (default 4)
      --multi-thread-write-buffer-size SizeSuffix   In memory buffer size for writing when in multi-thread mode (default 128Ki)
      --name-transform stringArray                  Transform paths during the copy process
      --no-check-dest                               Don't check the destination, copy regardless
      --no-traverse                                 Don't traverse destination file system on copy
      --no-update-dir-modtime                       Don't update directory modification times
      --no-update-modtime                           Don't update destination modtime if files identical
      --order-by string                             Instructions on how to order the transfers, e.g. 'size,descending'
      --partial-suffix string                       Add partial-suffix to temporary file name when --inplace is not used (default ".partial")
      --refresh-times                               Refresh the modtime of remote files
      --server-side-across-configs                  Allow server-side operations (e.g. copy) to work across different configs
      --size-only                                   Skip based on size only, not modtime or checksum
      --streaming-upload-cutoff SizeSuffix          Cutoff for switching to chunked upload if file size is unknown, upload starts after reaching cutoff or when file ends (default 100Ki)
  -u, --update                                      Skip files that are newer on the destination
```

### Important Options

Important flags useful for most commands

```
  -n, --dry-run         Do a trial run with no permanent changes
  -i, --interactive     Enable interactive mode
  -v, --verbose count   Print lots more stuff (repeat for more)
```

### Filter Options

Flags for filtering directory listings

```
      --delete-excluded                     Delete files on dest excluded from sync
      --exclude stringArray                 Exclude files matching pattern
      --exclude-from stringArray            Read file exclude patterns from file (use - to read from stdin)
      --exclude-if-present stringArray      Exclude directories if filename is present
      --files-from stringArray              Read list of source-file names from file (use - to read from stdin)
      --files-from-raw stringArray          Read list of source-file names from file without any processing of lines (use - to read from stdin)
  -f, --filter stringArray                  Add a file filtering rule
      --filter-from stringArray             Read file filtering patterns from a file (use - to read from stdin)
      --hash-filter string                  Partition filenames by hash k/n or randomly @/n
      --ignore-case                         Ignore case in filters (case insensitive)
      --include stringArray                 Include files matching pattern
      --include-from stringArray            Read file include patterns from file (use - to read from stdin)
      --max-age Duration                    Only transfer files younger than this in s or suffix ms|s|m|h|d|w|M|y (default off)
      --max-depth int                       If set limits the recursion depth to this (default -1)
      --max-size SizeSuffix                 Only transfer files smaller than this in KiB or suffix B|K|M|G|T|P (default off)
      --metadata-exclude stringArray        Exclude metadatas matching pattern
      --metadata-exclude-from stringArray   Read metadata exclude patterns from file (use - to read from stdin)
      --metadata-filter stringArray         Add a metadata filtering rule
      --metadata-filter-from stringArray    Read metadata filtering patterns from a file (use - to read from stdin)
      --metadata-include stringArray        Include metadatas matching pattern
      --metadata-include-from stringArray   Read metadata include patterns from file (use - to read from stdin)
      --min-age Duration                    Only transfer files older than this in s or suffix ms|s|m|h|d|w|M|y (default off)
      --min-size SizeSuffix                 Only transfer files bigger than this in KiB or suffix B|K|M|G|T|P (default off)
```

### Listing Options

Flags for listing directories

```
      --default-time Time   Time to show if modtime is unknown for files and directories (default 2000-01-01T00:00:00Z)
      --fast-list           Use recursive list if available; uses more memory but fewer transactions
```

## See Also

* [rclone](/commands/rclone/)	 - Show help for rclone commands, flags and backends.

