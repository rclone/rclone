package touch

import (
	"bytes"
	"time"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/spf13/cobra"
)

var (
	notCreateNewFile bool
	timeAsArgument   string
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
	flags := commandDefintion.Flags()
	flags.BoolVarP(&notCreateNewFile, "not-create", "C", false, "Do not create the file if it does not exist.")
	flags.StringVarP(&timeAsArgument, "time", "t", "", "Change the modification times to the specified time instead of the current time of day. The argument is of the form 'YYMMDD' (ex. 17.10.30) or 'YYYY-MM-DDTHH:MM:SS' (ex. 2006-01-02T15:04:05)")
}

var commandDefintion = &cobra.Command{
	Use:   "touch remote:path",
	Short: `Create new file or change file modification time.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc, srcFileName := cmd.NewFsDstFile(args)
		cmd.Run(true, false, command, func() error {
			return Touch(fsrc, srcFileName)
		})
	},
}

//Touch create new file or change file modification time.
func Touch(fsrc fs.Fs, srcFileName string) error {
	timeAtr := time.Now()
	if timeAsArgument != "" {
		layout := "060102"
		if len(timeAsArgument) == len("2006-01-02T15:04:05") {
			layout = "2006-01-02T15:04:05"
		}
		timeAtrFromFlags, err := time.Parse(layout, timeAsArgument)
		if err == nil {
			timeAtr = timeAtrFromFlags
		}
	}
	file, err := fsrc.NewObject(srcFileName)
	if err != nil {
		_, err = fsrc.List(srcFileName)
		if err != nil && !notCreateNewFile {
			var buffer []byte
			src := fs.NewStaticObjectInfo(srcFileName, timeAtr, int64(len("")), true, nil, fsrc)
			_, err = fsrc.Put(bytes.NewBuffer(buffer), src)
			if err != nil {
				return err
			}
		}
		return nil
	}
	err = file.SetModTime(timeAtr)
	if err == fs.ErrorCantSetModTime {
		return fs.ErrorCantSetModTime
	} else if err == fs.ErrorCantSetModTimeWithoutDelete {
		return fs.ErrorCantSetModTimeWithoutDelete
	}
	return nil
}
