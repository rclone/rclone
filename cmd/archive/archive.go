// Package archive implements 'rclone archive'.
package archive

import (
	"context"
	"errors"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/cobra"

	"github.com/rclone/rclone/cmd/archive/create"
	"github.com/rclone/rclone/cmd/archive/extract"
	"github.com/rclone/rclone/cmd/archive/list"
)

var (
	longList = false
	fullpath = false
	prefix   = ""
	format   = ""
)

func init() {
	// create flags
	createFlags := createCommand.Flags()
	flags.BoolVarP(createFlags, &fullpath, "fullpath", "", fullpath, "Set prefix for files in archive to source path", "")
	flags.StringVarP(createFlags, &prefix, "prefix", "", prefix, "Set prefix for files in archive to entered value or source path", "")
	flags.StringVarP(createFlags, &format, "format", "", format, "Compress the archive using the selected format. If not set will try and guess from extension. Use 'rclone archive create --help' for the supported formats", "")
	// list flags
	listFlags := listCommand.Flags()
	flags.BoolVarP(listFlags, &longList, "long", "", longList, "List extra attributtes", "")
	//
	Command.AddCommand(createCommand)
	Command.AddCommand(listCommand)
	Command.AddCommand(extractCommand)
	cmd.Root.AddCommand(Command)
}

// Command - archive command
var Command = &cobra.Command{
	Use:   "archive <action> [opts] <source> [<destination>]",
	Short: `Perform an action on an archive.`,
	Long: `Perform an action on an archive. Requires the use of a
subcommand to specify the protocol, e.g.

    rclone archive list remote:

Each subcommand has its own options which you can see in their help.
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.70",
	},
	RunE: func(command *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errors.New("archive requires an action, e.g. 'rclone archive list remote:'")
		}
		return errors.New("unknown action")
	},
}

var listCommand = &cobra.Command{
	Use:   "list [flags] <source>",
	Short: `List archive contents from source.`,
	// Warning! "|" will be replaced by backticks below
	Long: `List contents of an archive to the console, will autodetect format`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.70",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		//
		src, srcFile := cmd.NewFsFile(args[0])
		//
		cmd.Run(false, false, command, func() error {
			return list.ArchiveList(context.Background(), src, srcFile, longList)
		})
		return nil
	},
}

var extractCommand = &cobra.Command{
	Use:   "extract [flags] <source> <destination>",
	Short: `Extract archives from source to destination.`,
	// Warning! "|" will be replaced by backticks below
	Long: `Extract archive contents to destination directory, will autodetect format`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.70",
	},
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(2, 2, command, args)
		//
		src, srcFile := cmd.NewFsFile(args[0])
		dst, dstFile := cmd.NewFsFile(args[1])
		//
		cmd.Run(false, false, command, func() error {
			return extract.ArchiveExtract(context.Background(), src, srcFile, dst, dstFile)
		})
		return nil
	},
}

// Command - create
var createCommand = &cobra.Command{
	Use:   "create [flags] <source> [<destination>]",
	Short: `Archive source file(s) to destination.`,
	// Warning! "|" will be replaced by backticks below
	Long: strings.ReplaceAll(`Creates an archive from the files source:path and saves the archive
to dest:path. If dest:path is missing, it will write to the console.

Valid formats for the --format flag. If format is not set it will
guess it from the extension.

	Format	  Extensions
	------	  -----------
	zip 	  .zip
	tar 	  .tar
	tar.gz 	  .tar.gz, .tgz, .taz
	tar.bz2   .tar.bz2, .tb2, .tbz, .tbz2, .tz2
	tar.lz	  .tar.lz
	tar.lz4	  .tar.lz4
	tar.xz	  .tar.xz, .txz
	tar.zst	  .tar.zst, .tzst
	tar.br	  .tar.br
	tar.sz	  .tar.sz
	tar.mz	  .tar.mz

The --prefix and --fullpath flags will add a prefix for the files in
the archive. If the flag |--fullpath| is set then the files will have
the source path as prefix. If the flag |--prefix=<value>| is set then
the files will have <value> as prefix. It's possible to create invalid
file names with |--prefix=<value>| so use caution. Flag |--prefix|
always has priority.

If we have a directory |/sourcedir| with the following:

    file1.txt
    dir1/file2.txt

If we run the command |rclone archive create /sourcedir /dest.tar.gz| the
contents of the archive will be:

    file1.txt
    dir1/
    dir1/file2.txt

If we run the command |rclone archive create --fullpath /sourcedir /dest.tar.gz|
the contents of the archive will be:

    sourcedir/file1.txt
    sourcedir/dir1/
    sourcedir/dir1/file2.txt

If we run the command |rclone archive create --prefix=my_new_path /sourcedir /dest.tar.gz|
the contents of the archive will be:

    my_new_path/file1.txt
    my_new_path/dir1/
    my_new_path/dir1/file2.txt

`, "|", "`"),
	Annotations: map[string]string{
		"versionIntroduced": "v1.70",
	},
	RunE: func(command *cobra.Command, args []string) error {
		var src, dst fs.Fs
		var dstFile string
		if len(args) == 1 { // source only, archive to stdout
			src = cmd.NewFsSrc(args)
		} else if len(args) == 2 {
			src = cmd.NewFsSrc(args)
			dst, dstFile = cmd.NewFsFile(args[1])
		} else {
			cmd.CheckArgs(1, 2, command, args)
		}
		//
		cmd.Run(false, false, command, func() error {
			if prefix != "" {
				return create.ArchiveCreate(context.Background(), src, dst, dstFile, format, prefix)
			} else if fullpath {
				return create.ArchiveCreate(context.Background(), src, dst, dstFile, format, src.Root())
			}
			return create.ArchiveCreate(context.Background(), src, dst, dstFile, format, "")
		})
		return nil
	},
}
