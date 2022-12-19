// Package lshelp provides common help for list commands.
package lshelp

import (
	"strings"
)

// Help describes the common help for all the list commands
// Warning! "|" will be replaced by backticks below
var Help = strings.ReplaceAll(`
Any of the filtering options can be applied to this command.

There are several related list commands

  * |ls| to list size and path of objects only
  * |lsl| to list modification time, size and path of objects only
  * |lsd| to list directories only
  * |lsf| to list objects and directories in easy to parse format
  * |lsjson| to list objects and directories in JSON format

|ls|,|lsl|,|lsd| are designed to be human-readable.
|lsf| is designed to be human and machine-readable.
|lsjson| is designed to be machine-readable.

Note that |ls| and |lsl| recurse by default - use |--max-depth 1| to stop the recursion.

The other list commands |lsd|,|lsf|,|lsjson| do not recurse by default - use |-R| to make them recurse.

Listing a nonexistent directory will produce an error except for
remotes which can't have empty directories (e.g. s3, swift, or gcs -
the bucket-based remotes).
`, "|", "`")
