package touch

import (
	"bytes"
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/object"
	"github.com/spf13/cobra"
)

var (
	notCreateNewFile bool
	timeAsArgument   string
	localTime        bool
)

const defaultLayout string = "060102"
const layoutDateWithTime = "2006-01-02T15:04:05"

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &notCreateNewFile, "no-create", "C", false, "Do not create the file if it does not exist.")
	flags.StringVarP(cmdFlags, &timeAsArgument, "timestamp", "t", "", "Use specified time instead of the current time of day.")
	flags.BoolVarP(cmdFlags, &localTime, "localtime", "", false, "Use localtime for timestamp, not UTC.")
}

var commandDefinition = &cobra.Command{
	Use:   "touch remote:path",
	Short: `Create new file or change file modification time.`,
	Long: `
Set the modification time on object(s) as specified by remote:path to
have the current time.

If remote:path does not exist then a zero sized object will be created
unless the --no-create flag is provided.

If --timestamp is used then it will set the modification time to that
time instead of the current time. Times may be specified as one of:

- 'YYMMDD' - eg. 17.10.30
- 'YYYY-MM-DDTHH:MM:SS' - eg. 2006-01-02T15:04:05

Note that --timestamp is in UTC if you want local time then add the
--localtime flag.
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 1, command, args)
		fsrc, srcFileName := cmd.NewFsDstFile(args)
		cmd.Run(true, false, command, func() error {
			return Touch(context.Background(), fsrc, srcFileName)
		})
	},
}

//Touch create new file or change file modification time.
func Touch(ctx context.Context, fsrc fs.Fs, srcFileName string) (err error) {
	timeAtr := time.Now()
	if timeAsArgument != "" {
		layout := defaultLayout
		if len(timeAsArgument) == len(layoutDateWithTime) {
			layout = layoutDateWithTime
		}
		var timeAtrFromFlags time.Time
		if localTime {
			timeAtrFromFlags, err = time.ParseInLocation(layout, timeAsArgument, time.Local)
		} else {
			timeAtrFromFlags, err = time.Parse(layout, timeAsArgument)
		}
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
