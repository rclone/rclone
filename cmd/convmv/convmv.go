// Package convmv provides the convmv command.
package convmv

import (
	"context"
	"errors"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/sync"
	"github.com/rclone/rclone/lib/transform"
	"github.com/spf13/cobra"
)

// Globals
var (
	deleteEmptySrcDirs = false
	createEmptySrcDirs = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &deleteEmptySrcDirs, "delete-empty-src-dirs", "", deleteEmptySrcDirs, "Delete empty source dirs after move", "")
	flags.BoolVarP(cmdFlags, &createEmptySrcDirs, "create-empty-src-dirs", "", createEmptySrcDirs, "Create empty source dirs on destination after move", "")
}

var commandDefinition = &cobra.Command{
	Use:   "convmv dest:path --name-transform XXX",
	Short: `Convert file and directory names in place.`,
	// Warning¡ "¡" will be replaced by backticks below
	Long: strings.ReplaceAll(`convmv supports advanced path name transformations for converting and renaming
files and directories by applying prefixes, suffixes, and other alterations.

`+transform.Help()+`The regex command generally accepts Perl-style regular expressions, the exact
syntax is defined in the [Go regular expression reference](https://golang.org/pkg/regexp/syntax/).
The replacement string may contain capturing group variables, referencing
capturing groups using the syntax ¡$name¡ or ¡${name}¡, where the name can
refer to a named capturing group or it can simply be the index as a number.
To insert a literal $, use $$.

Multiple transformations can be used in sequence, applied
in the order they are specified on the command line.

The ¡--name-transform¡ flag is also available in ¡sync¡, ¡copy¡, and ¡move¡.

### Files vs Directories

By default ¡--name-transform¡ will only apply to file names. The means only the
leaf file name will be transformed. However some of the transforms would be
better applied to the whole path or just directories. To choose which which
part of the file path is affected some tags can be added to the ¡--name-transform¡.

| Tag | Effect |
|------|------|
| ¡file¡ | Only transform the leaf name of files (DEFAULT) |
| ¡dir¡ | Only transform name of directories - these may appear anywhere in the path |
| ¡all¡ | Transform the entire path for files and directories |

This is used by adding the tag into the transform name like this:
¡--name-transform file,prefix=ABC¡ or ¡--name-transform dir,prefix=DEF¡.

For some conversions using all is more likely to be useful, for example
¡--name-transform all,nfc¡.

Note that ¡--name-transform¡ may not add path separators ¡/¡ to the name.
This will cause an error.

### Ordering and Conflicts

- Transformations will be applied in the order specified by the user.
  - If the ¡file¡ tag is in use (the default) then only the leaf name of files
    will be transformed.
  - If the ¡dir¡ tag is in use then directories anywhere in the path will be
    transformed
  - If the ¡all¡ tag is in use then directories and files anywhere in the path
    will be transformed
  - Each transformation will be run one path segment at a time.
  - If a transformation adds a ¡/¡ or ends up with an empty path segment then
    that will be an error.
- It is up to the user to put the transformations in a sensible order.
  - Conflicting transformations, such as ¡prefix¡ followed by ¡trimprefix¡ or
    ¡nfc¡ followed by ¡nfd¡, are possible.
  - Instead of enforcing mutual exclusivity, transformations are applied in
    sequence as specified by the user, allowing for intentional use cases
    (e.g., trimming one prefix before adding another).
  - Users should be aware that certain combinations may lead to unexpected
    results and should verify transformations using ¡--dry-run¡ before execution.

### Race Conditions and Non-Deterministic Behavior

Some transformations, such as ¡replace=old:new¡, may introduce conflicts where
multiple source files map to the same destination name. This can lead to race
conditions when performing concurrent transfers. It is up to the user to
anticipate these.

- If two files from the source are transformed into the same name at the
  destination, the final state may be non-deterministic.
- Running rclone check after a sync using such transformations may erroneously
  report missing or differing files due to overwritten results.

To minimize risks, users should:

- Carefully review transformations that may introduce conflicts.
- Use ¡--dry-run¡ to inspect changes before executing a sync (but keep in mind
  that it won't show the effect of non-deterministic transformations).
- Avoid transformations that cause multiple distinct source files to map to the
  same destination name.
- Consider disabling concurrency with ¡--transfers=1¡ if necessary.
- Certain transformations (e.g. ¡prefix¡) will have a multiplying effect every
  time they are used. Avoid these when using ¡bisync¡.`, "¡", "`"),
	Annotations: map[string]string{
		"versionIntroduced": "v1.70",
		"groups":            "Filter,Listing,Important,Copy",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fdst, srcFileName := cmd.NewFsFile(args[0])
		cmd.Run(false, true, command, func() error {
			if !transform.Transforming(context.Background()) {
				return errors.New("--name-transform must be set")
			}
			if srcFileName == "" {
				return sync.Transform(context.Background(), fdst, deleteEmptySrcDirs, createEmptySrcDirs)
			}
			return operations.TransformFile(context.Background(), fdst, srcFileName)
		})
	},
}
