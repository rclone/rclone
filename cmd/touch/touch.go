package touch

import (
	"bytes"
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/spf13/cobra"
)

var (
	notCreateNewFile bool
	timeAsArgument   string
)

const defaultLayout string = "060102"
const layoutDateWithTime = "2006-01-02T15:04:05"

func init() {
	cmd.Root.AddCommand(commandDefintion)
	flags := commandDefintion.Flags()
	flags.BoolVarP(&notCreateNewFile, "no-create", "C", false, "Do not create the file if it does not exist.")
	flags.StringVarP(&timeAsArgument, "timestamp", "t", "", "Change the modification times to the specified time instead of the current time of day. The argument is of the form 'YYMMDD' (ex. 17.10.30) or 'YYYY-MM-DDTHH:MM:SS' (ex. 2006-01-02T15:04:05)")
}

var commandDefintion = &cobra.Command{
	Use:   "touch remote:path",
	Short: `Create new file or change file modification time.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc, srcFileName := cmd.NewFsDstFile(args)
		cmd.Run(true, false, command, func() error {
			return Touch(context.Background(), fsrc, srcFileName)
		})
	},
}

//Touch create new file or change file modification time.
func Touch(ctx context.Context, fsrc fs.Fs, srcFileName string) error {
	timeAtr := time.Now()
	if timeAsArgument != "" {
		layout := defaultLayout
		if len(timeAsArgument) == len(layoutDateWithTime) {
			layout = layoutDateWithTime
		}
		timeAtrFromFlags, err := time.Parse(layout, timeAsArgument)
		if err != nil {
			return errors.Wrap(err, "failed to parse date/time argument")
		}
		timeAtr = timeAtrFromFlags
	}
	file, err := fsrc.NewObject(ctx, srcFileName)
	if err != nil {
		if !notCreateNewFile {
			var buffer []byte
			src := object.NewStaticObjectInfo(srcFileName, timeAtr, int64(len(buffer)), true, nil, fsrc)
			_, err = fsrc.Put(ctx, bytes.NewBuffer(buffer), src)
			if err != nil {
				return err
			}
		}
		return nil
	}
	err = file.SetModTime(ctx, timeAtr)
	if err != nil {
		return errors.Wrap(err, "touch: couldn't set mod time")
	}
	return nil
}
