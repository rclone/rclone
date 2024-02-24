// Package lsf provides the lsf command.
package lsf

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/ls/lshelp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/operations"
	"github.com/spf13/cobra"
)

var (
	format     string
	timeFormat string
	separator  string
	dirSlash   bool
	recurse    bool
	hashType   = hash.MD5
	filesOnly  bool
	dirsOnly   bool
	csv        bool
	absolute   bool
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.StringVarP(cmdFlags, &format, "format", "F", "p", "Output format - see  help for details", "")
	flags.StringVarP(cmdFlags, &timeFormat, "time-format", "t", "", "Specify a custom time format, or 'max' for max precision supported by remote (default: 2006-01-02 15:04:05)", "")
	flags.StringVarP(cmdFlags, &separator, "separator", "s", ";", "Separator for the items in the format", "")
	flags.BoolVarP(cmdFlags, &dirSlash, "dir-slash", "d", true, "Append a slash to directory names", "")
	flags.FVarP(cmdFlags, &hashType, "hash", "", "Use this hash when `h` is used in the format MD5|SHA-1|DropboxHash", "")
	flags.BoolVarP(cmdFlags, &filesOnly, "files-only", "", false, "Only list files", "")
	flags.BoolVarP(cmdFlags, &dirsOnly, "dirs-only", "", false, "Only list directories", "")
	flags.BoolVarP(cmdFlags, &csv, "csv", "", false, "Output in CSV format", "")
	flags.BoolVarP(cmdFlags, &absolute, "absolute", "", false, "Put a leading / in front of path names", "")
	flags.BoolVarP(cmdFlags, &recurse, "recursive", "R", false, "Recurse into the listing", "")
}

var commandDefinition = &cobra.Command{
	Use:   "lsf remote:path",
	Short: `List directories and objects in remote:path formatted for parsing.`,
	Long: `
List the contents of the source path (directories and objects) to
standard output in a form which is easy to parse by scripts.  By
default this will just be the names of the objects and directories,
one per line.  The directories will have a / suffix.

Eg

    $ rclone lsf swift:bucket
    bevajer5jef
    canole
    diwogej7
    ferejej3gux/
    fubuwic

Use the ` + "`--format`" + ` option to control what gets listed.  By default this
is just the path, but you can use these parameters to control the
output:

    p - path
    s - size
    t - modification time
    h - hash
    i - ID of object
    o - Original ID of underlying object
    m - MimeType of object if known
    e - encrypted name
    T - tier of storage if known, e.g. "Hot" or "Cool"
    M - Metadata of object in JSON blob format, eg {"key":"value"}

So if you wanted the path, size and modification time, you would use
` + "`--format \"pst\"`, or maybe `--format \"tsp\"`" + ` to put the path last.

Eg

    $ rclone lsf  --format "tsp" swift:bucket
    2016-06-25 18:55:41;60295;bevajer5jef
    2016-06-25 18:55:43;90613;canole
    2016-06-25 18:55:43;94467;diwogej7
    2018-04-26 08:50:45;0;ferejej3gux/
    2016-06-25 18:55:40;37600;fubuwic

If you specify "h" in the format you will get the MD5 hash by default,
use the ` + "`--hash`" + ` flag to change which hash you want.  Note that this
can be returned as an empty string if it isn't available on the object
(and for directories), "ERROR" if there was an error reading it from
the object and "UNSUPPORTED" if that object does not support that hash
type.

For example, to emulate the md5sum command you can use

    rclone lsf -R --hash MD5 --format hp --separator "  " --files-only .

Eg

    $ rclone lsf -R --hash MD5 --format hp --separator "  " --files-only swift:bucket
    7908e352297f0f530b84a756f188baa3  bevajer5jef
    cd65ac234e6fea5925974a51cdd865cc  canole
    03b5341b4f234b9d984d03ad076bae91  diwogej7
    8fd37c3810dd660778137ac3a66cc06d  fubuwic
    99713e14a4c4ff553acaf1930fad985b  gixacuh7ku

(Though "rclone md5sum ." is an easier way of typing this.)

By default the separator is ";" this can be changed with the
` + "`--separator`" + ` flag.  Note that separators aren't escaped in the path so
putting it last is a good strategy.

Eg

    $ rclone lsf  --separator "," --format "tshp" swift:bucket
    2016-06-25 18:55:41,60295,7908e352297f0f530b84a756f188baa3,bevajer5jef
    2016-06-25 18:55:43,90613,cd65ac234e6fea5925974a51cdd865cc,canole
    2016-06-25 18:55:43,94467,03b5341b4f234b9d984d03ad076bae91,diwogej7
    2018-04-26 08:52:53,0,,ferejej3gux/
    2016-06-25 18:55:40,37600,8fd37c3810dd660778137ac3a66cc06d,fubuwic

You can output in CSV standard format.  This will escape things in "
if they contain ,

Eg

    $ rclone lsf --csv --files-only --format ps remote:path
    test.log,22355
    test.sh,449
    "this file contains a comma, in the file name.txt",6

Note that the ` + "`--absolute`" + ` parameter is useful for making lists of files
to pass to an rclone copy with the ` + "`--files-from-raw`" + ` flag.

For example, to find all the files modified within one day and copy
those only (without traversing the whole directory structure):

    rclone lsf --absolute --files-only --max-age 1d /path/to/local > new_files
    rclone copy --files-from-raw new_files /path/to/local remote:path

The default time format is ` + "`'2006-01-02 15:04:05'`" + `.
[Other formats](https://pkg.go.dev/time#pkg-constants) can be specified with the ` + "`--time-format`" + ` flag.
Examples:

	rclone lsf remote:path --format pt --time-format 'Jan 2, 2006 at 3:04pm (MST)'
	rclone lsf remote:path --format pt --time-format '2006-01-02 15:04:05.000000000'
	rclone lsf remote:path --format pt --time-format '2006-01-02T15:04:05.999999999Z07:00'
	rclone lsf remote:path --format pt --time-format RFC3339
	rclone lsf remote:path --format pt --time-format DateOnly
	rclone lsf remote:path --format pt --time-format max
` + "`--time-format max`" + ` will automatically truncate ` + "'`2006-01-02 15:04:05.000000000`'" + `
to the maximum precision supported by the remote.

` + lshelp.Help,
	Annotations: map[string]string{
		"versionIntroduced": "v1.40",
		"groups":            "Filter,Listing",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			// Work out if the separatorFlag was supplied or not
			separatorFlag := command.Flags().Lookup("separator")
			separatorFlagSupplied := separatorFlag != nil && separatorFlag.Changed
			// Default the separator to , if using CSV
			if csv && !separatorFlagSupplied {
				separator = ","
			}
			return Lsf(context.Background(), fsrc, os.Stdout)
		})
	},
}

// Lsf lists all the objects in the path with modification time, size
// and path in specific format.
func Lsf(ctx context.Context, fsrc fs.Fs, out io.Writer) error {
	var list operations.ListFormat
	list.SetSeparator(separator)
	list.SetCSV(csv)
	list.SetDirSlash(dirSlash)
	list.SetAbsolute(absolute)
	var opt = operations.ListJSONOpt{
		NoModTime:  true,
		NoMimeType: true,
		DirsOnly:   dirsOnly,
		FilesOnly:  filesOnly,
		Recurse:    recurse,
	}

	for _, char := range format {
		switch char {
		case 'p':
			list.AddPath()
		case 't':
			if timeFormat == "max" {
				timeFormat = operations.FormatForLSFPrecision(fsrc.Precision())
			}
			list.AddModTime(timeFormat)
			opt.NoModTime = false
		case 's':
			list.AddSize()
		case 'h':
			list.AddHash(hashType)
			opt.ShowHash = true
			opt.HashTypes = []string{hashType.String()}
		case 'i':
			list.AddID()
		case 'm':
			list.AddMimeType()
			opt.NoMimeType = false
		case 'e':
			list.AddEncrypted()
			opt.ShowEncrypted = true
		case 'o':
			list.AddOrigID()
			opt.ShowOrigIDs = true
		case 'T':
			list.AddTier()
		case 'M':
			list.AddMetadata()
			opt.Metadata = true
		default:
			return fmt.Errorf("unknown format character %q", char)
		}
	}

	return operations.ListJSON(ctx, fsrc, "", &opt, func(item *operations.ListJSONItem) error {
		_, _ = fmt.Fprintln(out, list.Format(item))
		return nil
	})
}
