package config

import (
	"context"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
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
	newType := args[0]
	f := fs.MustFind(newType)
	if f.Config == nil {
		return errors.Errorf("can't authorize fs %q", newType)
	}
	// Name used for temporary fs
	name := "**temp-fs**"

	// Make sure we delete it
	defer DeleteRemote(name)

	// Indicate that we are running rclone authorize
	Data.SetValue(name, ConfigAuthorize, "true")
	if noAutoBrowser {
		Data.SetValue(name, ConfigAuthNoBrowser, "true")
	}

	if len(args) == 3 {
		Data.SetValue(name, ConfigClientID, args[1])
		Data.SetValue(name, ConfigClientSecret, args[2])
	}

	m := fs.ConfigMap(f, name, nil)
	f.Config(ctx, name, m)
	return nil
}
