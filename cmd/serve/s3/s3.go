package s3

import (
	"context"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/hash"
	httplib "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/lib/http/auth"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
)

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	hostBucketMode: false,
	hashName:       "MD5",
	hashType:       hash.MD5,

	noCleanup: false,
}

// Opt is options set by command line flags
var Opt = DefaultOpt

func init() {
	flagSet := Command.Flags()
	httplib.AddFlags(flagSet)
	vfsflags.AddFlags(flagSet)
	flags.BoolVarP(flagSet, &Opt.hostBucketMode, "force-path-style", "", Opt.hostBucketMode, "If true use path style access if false use virtual hosted style (default true)")
	flags.StringVarP(flagSet, &Opt.hashName, "etag-hash", "", Opt.hashName, "Which hash to use for the ETag, or auto or blank for off")
	flags.StringArrayVarP(flagSet, &Opt.authPair, "authkey", "", Opt.authPair, "Set key pair for v4 authorization, split by comma")
	flags.BoolVarP(flagSet, &Opt.noCleanup, "no-cleanup", "", Opt.noCleanup, "Not to cleanup empty folder after object is deleted")
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "s3 remote:path",
	Short: `Serve remote:path over s3.`,
	Long:  strings.ReplaceAll(longHelp, "|", "`") + httplib.Help + auth.Help + vfs.Help,
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
			router, err := httplib.Router()
			if err != nil {
				return err
			}
			s.Bind(router)
			httplib.Wait()
			return nil
		})
		return nil
	},
}
