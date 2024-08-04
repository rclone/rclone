package config

import (
	"context"
	"fmt"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
)

// Authorize is for remote authorization of headless machines.
//
// It expects 1, 2 or 3 arguments
//
//	rclone authorize "fs name"
//	rclone authorize "fs name" "base64 encoded JSON blob"
//	rclone authorize "fs name" "client id" "client secret"
func Authorize(ctx context.Context, args []string, noAutoBrowser bool, templateFile string) error {
	ctx = suppressConfirm(ctx)
	ctx = fs.ConfigOAuthOnly(ctx)
	switch len(args) {
	case 1, 2, 3:
	default:
		return fmt.Errorf("invalid number of arguments: %d", len(args))
	}
	Type := args[0] // FIXME could read this from input
	ri, err := fs.Find(Type)
	if err != nil {
		return err
	}
	if ri.Config == nil {
		return fmt.Errorf("can't authorize fs %q", Type)
	}

	// Config map for remote
	inM := configmap.Simple{}

	// Indicate that we are running rclone authorize
	inM[ConfigAuthorize] = "true"
	if noAutoBrowser {
		inM[ConfigAuthNoBrowser] = "true"
	}

	// Indicate if we specified a custom template via a file
	if templateFile != "" {
		inM[ConfigTemplateFile] = templateFile
	}

	// Add extra parameters if supplied
	if len(args) == 2 {
		err := inM.Decode(args[1])
		if err != nil {
			return err
		}
	} else if len(args) == 3 {
		inM[ConfigClientID] = args[1]
		inM[ConfigClientSecret] = args[2]
	}

	// Name used for temporary remote
	name := "**temp-fs**"

	m := fs.ConfigMap(ri.Prefix, ri.Options, name, inM)
	outM := configmap.Simple{}
	m.ClearSetters()
	m.AddSetter(outM)
	m.AddGetter(outM, configmap.PriorityNormal)

	err = PostConfig(ctx, name, m, ri)
	if err != nil {
		return err
	}

	// Print the code for the user to paste
	out := outM["token"]

	// If received a config blob, then return one
	if len(args) == 2 {
		out, err = outM.Encode()
		if err != nil {
			return err
		}
	}
	fmt.Printf("Paste the following into your remote machine --->\n%s\n<---End paste\n", out)

	return nil
}
