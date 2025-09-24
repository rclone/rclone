### Logger Flags

The `--differ`, `--missing-on-dst`, `--missing-on-src`, `--match` and `--error`
flags write paths, one per line, to the file name (or stdout if it is `-`)
supplied. What they write is described in the help below. For example
`--differ` will write all paths which are present on both the source and
destination but different.

The `--combined` flag will write a file (or stdout) which contains all
file paths with a symbol and then a space and then the path to tell
you what happened to it. These are reminiscent of diff files.

- `= path` means path was found in source and destination and was identical
- `- path` means path was missing on the source, so only in the destination
- `+ path` means path was missing on the destination, so only in the source
- `* path` means path was present in source and destination but different.
- `! path` means there was an error reading or hashing the source or dest.

The `--dest-after` flag writes a list file using the same format flags
as [`lsf`](/commands/rclone_lsf/#synopsis) (including [customizable options
for hash, modtime, etc.](/commands/rclone_lsf/#synopsis))
Conceptually it is similar to rsync's `--itemize-changes`, but not identical
-- it should output an accurate list of what will be on the destination
after the command is finished.

When the `--no-traverse` flag is set, all logs involving files that exist only
on the destination will be incomplete or completely missing.

Note that these logger flags have a few limitations, and certain scenarios
are not currently supported:

- `--max-duration` / `CutoffModeHard`
- `--compare-dest` / `--copy-dest`
- server-side moves of an entire dir at once
- High-level retries, because there would be duplicates (use `--retries 1` to disable)
- Possibly some unusual error scenarios

Note also that each file is logged during execution, as opposed to after, so it
is most useful as a predictor of what SHOULD happen to each file
(which may or may not match what actually DID).
