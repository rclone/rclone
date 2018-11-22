package lsjson

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/cmd/ls/lshelp"
	"github.com/ncw/rclone/fs/operations"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	opt operations.ListJSONOpt
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
	commandDefintion.Flags().BoolVarP(&opt.Recurse, "recursive", "R", false, "Recurse into the listing.")
	commandDefintion.Flags().BoolVarP(&opt.ShowHash, "hash", "", false, "Include hashes in the output (may take longer).")
	commandDefintion.Flags().BoolVarP(&opt.NoModTime, "no-modtime", "", false, "Don't read the modification time (can speed things up).")
	commandDefintion.Flags().BoolVarP(&opt.ShowEncrypted, "encrypted", "M", false, "Show the encrypted names.")
	commandDefintion.Flags().BoolVarP(&opt.ShowOrigIDs, "original", "", false, "Show the ID of the underlying Object.")
}

var commandDefintion = &cobra.Command{
	Use:   "lsjson remote:path",
	Short: `List directories and objects in the path in JSON format.`,
	Long: `List directories and objects in the path in JSON format.

The output is an array of Items, where each Item looks like this

   {
      "Hashes" : {
         "SHA-1" : "f572d396fae9206628714fb2ce00f72e94f2258f",
         "MD5" : "b1946ac92492d2347c6235b4d2611184",
         "DropboxHash" : "ecb65bb98f9d905b70458986c39fcbad7715e5f2fcc3b1f07767d7c83e2438cc"
      },
      "ID": "y2djkhiujf83u33",
      "OrigID": "UYOJVTUW00Q1RzTDA",
      "IsDir" : false,
      "MimeType" : "application/octet-stream",
      "ModTime" : "2017-05-31T16:15:57.034468261+01:00",
      "Name" : "file.txt",
      "Encrypted" : "v0qpsdq8anpci8n929v3uu9338",
      "Path" : "full/path/goes/here/file.txt",
      "Size" : 6
   }

If --hash is not specified the Hashes property won't be emitted.

If --no-modtime is specified then ModTime will be blank.

If --encrypted is not specified the Encrypted won't be emitted.

The Path field will only show folders below the remote path being listed.
If "remote:path" contains the file "subfolder/file.txt", the Path for "file.txt"
will be "subfolder/file.txt", not "remote:path/subfolder/file.txt".
When used without --recursive the Path will always be the same as Name.

The time is in RFC3339 format with nanosecond precision.

The whole output can be processed as a JSON blob, or alternatively it
can be processed line by line as each item is written one to a line.
` + lshelp.Help,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			fmt.Println("[")
			first := true
			err := operations.ListJSON(fsrc, "", &opt, func(item *operations.ListJSONItem) error {
				out, err := json.Marshal(item)
				if err != nil {
					return errors.Wrap(err, "failed to marshal list object")
				}
				if first {
					first = false
				} else {
					fmt.Print(",\n")
				}
				_, err = os.Stdout.Write(out)
				if err != nil {
					return errors.Wrap(err, "failed to write to output")
				}
				return nil
			})
			if err != nil {
				return err
			}
			if !first {
				fmt.Println()
			}
			fmt.Println("]")
			return nil
		})
	},
}
