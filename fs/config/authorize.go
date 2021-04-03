package config

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
)

// Authorize is for remote authorization of headless machines.
//
// It expects 1 or 3 arguments
//
//   rclone authorize "fs name"
//   rclone authorize "fs name" "client id" "client secret"
func Authorize(ctx context.Context, args []string, noAutoBrowser bool) error {
	ctx = suppressConfirm(ctx)
	switch len(args) {
	case 1, 3:
	default:
		return errors.Errorf("invalid number of arguments: %d", len(args))
	}
	Type := args[0] // FIXME could read this from input
	ri, err := fs.Find(Type)
	if err != nil {
		return err
	}
	if ri.Config == nil {
		return errors.Errorf("can't authorize fs %q", Type)
	}

	// Config map for remote
	inM := configmap.Simple{}

	// Indicate that we are running rclone authorize
	inM[ConfigAuthorize] = "true"
	if noAutoBrowser {
		inM[ConfigAuthNoBrowser] = "true"
	}

	if len(args) == 3 {
		inM[ConfigClientID] = args[1]
		inM[ConfigClientSecret] = args[2]
	}

	// Name used for temporary remote
	name := "**temp-fs**"

	m := fs.ConfigMap(ri, name, inM)
	outM := configmap.Simple{}
	m.ClearSetters()
	m.AddSetter(outM)

	ri.Config(ctx, name, m)

	// Print code if we are doing a manual auth
	fmt.Printf("Paste the following into your remote machine --->\n%s\n<---End paste\n", outM["token"])

	fs.Debugf(nil, "Set parameters %q", outM)

	return nil
}
