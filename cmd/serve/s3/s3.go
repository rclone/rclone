package s3

import (
	"context"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve/httplib"
	"github.com/rclone/rclone/cmd/serve/httplib/httpflags"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
)

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	hostBucketMode: false,
	hashName:       "MD5",
	hashType:       hash.MD5,
	authPair:       "",
	noCleanup:      false,
}

// Opt is options set by command line flags
var Opt = DefaultOpt

func init() {
	flagSet := Command.Flags()
	httpflags.AddFlags(flagSet)
	vfsflags.AddFlags(flagSet)
	flags.BoolVarP(flagSet, &Opt.hostBucketMode, "host-bucket", "", Opt.hostBucketMode, "Whether to use bucket name in hostname (such as mybucket.local)")
	flags.StringVarP(flagSet, &Opt.hashName, "etag-hash", "", Opt.hashName, "Which hash to use for the ETag, or auto or blank for off")
	flags.StringVarP(flagSet, &Opt.authPair, "s3-auth", "", Opt.authPair, "Set key pairs for authorization, split by comma. example: ak-sk,ak2-sk2")
	flags.BoolVarP(flagSet, &Opt.noCleanup, "no-cleanup", "", Opt.noCleanup, "Not to cleanup empty folder after object is deleted")

}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "s3 remote:path",
	Short: `Serve remote:path over s3.`,
	Long:  strings.ReplaceAll(longHelp, "|", "`") + httplib.Help + vfs.Help,
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(1, 1, command, args)
		f := cmd.NewFsSrc(args)

		if Opt.hashName == "auto" {
			Opt.hashType = f.Hashes().GetOne()
		} else if Opt.hashName != "" {
			err := Opt.hashType.Set(Opt.hashName)
			if err != nil {
				return err
			}
		}
		cmd.Run(false, false, command, func() error {
			s := newServer(context.Background(), f, &Opt)
			err := s.Serve()
			if err != nil {
				return err
			}
			s.Wait()
			return nil
		})
		return nil
	},
}
