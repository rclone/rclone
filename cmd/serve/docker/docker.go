// Package docker serves a remote suitable for use with docker volume api
package docker

import (
	"context"
	_ "embed"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/mountlib"
	"github.com/rclone/rclone/cmd/serve"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
)

var (
	pluginName  = "rclone"
	pluginScope = "local"
	baseDir     = "/var/lib/docker-volumes/rclone"
	sockDir     = "/run/docker/plugins" //lint:ignore U1000 unused when not building linux
	defSpecDir  = "/etc/docker/plugins"
	stateFile   = "docker-plugin.state"
	socketAddr  = "" // TCP listening address or empty string for Unix socket
	socketGid   = syscall.Getgid()
	canPersist  = false // allows writing to config file
	forgetState = false
	noSpec      = false
)

//go:embed docker.md
var longHelp string

// help returns the help string cleaned up to simplify appending
func help() string {
	return strings.TrimSpace(longHelp) + "\n\n"
}

func init() {
	cmdFlags := Command.Flags()
	// Add command specific flags
	flags.StringVarP(cmdFlags, &baseDir, "base-dir", "", baseDir, "Base directory for volumes", "")
	flags.StringVarP(cmdFlags, &socketAddr, "socket-addr", "", socketAddr, "Address <host:port> or absolute path (default: /run/docker/plugins/rclone.sock)", "")
	flags.IntVarP(cmdFlags, &socketGid, "socket-gid", "", socketGid, "GID for unix socket (default: current process GID)", "")
	flags.BoolVarP(cmdFlags, &forgetState, "forget-state", "", forgetState, "Skip restoring previous state", "")
	flags.BoolVarP(cmdFlags, &noSpec, "no-spec", "", noSpec, "Do not write spec file", "")
	// Add common mount/vfs flags
	mountlib.AddFlags(cmdFlags)
	vfsflags.AddFlags(cmdFlags)
	// Register with parent command
	serve.Command.AddCommand(Command)
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "docker",
	Short: `Serve any remote on docker's volume plugin API.`,
	Long:  help() + vfs.Help(),
	Annotations: map[string]string{
		"versionIntroduced": "v1.56",
		"groups":            "Filter",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 0, command, args)
		cmd.Run(false, false, command, func() error {
			ctx := context.Background()
			drv, err := NewDriver(ctx, baseDir, nil, nil, false, forgetState)
			if err != nil {
				return err
			}
			srv := NewServer(drv)
			if socketAddr == "" {
				// Listen on unix socket at /run/docker/plugins/<pluginName>.sock
				return srv.ServeUnix(pluginName, socketGid)
			}
			if filepath.IsAbs(socketAddr) {
				// Listen on unix socket at given path
				return srv.ServeUnix(socketAddr, socketGid)
			}
			return srv.ServeTCP(socketAddr, "", nil, noSpec)
		})
	},
}
