package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/gobuffalo/envy"
	"github.com/spf13/cobra"
)

func Decorate(c Command) *cobra.Command {
	var flags []string
	if len(c.Flags) > 0 {
		for _, f := range c.Flags {
			flags = append(flags, f)
		}
	}
	cc := &cobra.Command{
		Use:     c.Name,
		Short:   fmt.Sprintf("[PLUGIN] %s", c.Description),
		Aliases: c.Aliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			plugCmd := c.Name
			if c.UseCommand != "" {
				plugCmd = c.UseCommand
			}

			ax := []string{plugCmd}
			if plugCmd == "-" {
				ax = []string{}
			}

			ax = append(ax, args...)
			ax = append(ax, flags...)
			ex := exec.Command(c.Binary, ax...)
			if runtime.GOOS != "windows" {
				ex.Env = append(envy.Environ(), "BUFFALO_PLUGIN=1")
			}
			ex.Stdin = os.Stdin
			ex.Stdout = os.Stdout
			ex.Stderr = os.Stderr
			return ex.Run()
		},
	}
	cc.DisableFlagParsing = true
	return cc
}
