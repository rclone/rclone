// Package version provides the version command.
package version

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/spf13/cobra"
)

var (
	check = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &check, "check", "", false, "Check for new version", "")
}

var commandDefinition = &cobra.Command{
	Use:   "version",
	Short: `Show the version number.`,
	Long: `Show the rclone version number, the go version, the build target
OS and architecture, the runtime OS and kernel version and bitness,
build tags and the type of executable (static or dynamic).

For example:

    $ rclone version
    rclone v1.55.0
    - os/version: ubuntu 18.04 (64 bit)
    - os/kernel: 4.15.0-136-generic (x86_64)
    - os/type: linux
    - os/arch: amd64
    - go/version: go1.16
    - go/linking: static
    - go/tags: none

Note: before rclone version 1.55 the os/type and os/arch lines were merged,
      and the "go/version" line was tagged as "go version".

If you supply the --check flag, then it will do an online check to
compare your version with the latest release and the latest beta.

    $ rclone version --check
    yours:  1.42.0.6
    latest: 1.42          (released 2018-06-16)
    beta:   1.42.0.5      (released 2018-06-17)

Or

    $ rclone version --check
    yours:  1.41
    latest: 1.42          (released 2018-06-16)
      upgrade: https://downloads.rclone.org/v1.42
    beta:   1.42.0.5      (released 2018-06-17)
      upgrade: https://beta.rclone.org/v1.42-005-g56e1e820

`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.33",
	},
	Run: func(command *cobra.Command, args []string) {
		ctx := context.Background()
		cmd.CheckArgs(0, 0, command, args)
		if check {
			CheckVersion(ctx)
		} else {
			cmd.ShowVersion()
		}
	},
}

// strip a leading v off the string
func stripV(s string) string {
	if len(s) > 0 && s[0] == 'v' {
		return s[1:]
	}
	return s
}

// GetVersion gets the version available for download
func GetVersion(ctx context.Context, url string) (v *semver.Version, vs string, date time.Time, err error) {
	resp, err := fshttp.NewClient(ctx).Get(url)
	if err != nil {
		return v, vs, date, err
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode != http.StatusOK {
		return v, vs, date, errors.New(resp.Status)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return v, vs, date, err
	}
	vs = strings.TrimSpace(string(bodyBytes))
	vs = strings.TrimPrefix(vs, "rclone ")
	vs = strings.TrimRight(vs, "β")
	date, err = http.ParseTime(resp.Header.Get("Last-Modified"))
	if err != nil {
		return v, vs, date, err
	}
	v, err = semver.NewVersion(stripV(vs))
	return v, vs, date, err
}

// CheckVersion checks the installed version against available downloads
func CheckVersion(ctx context.Context) {
	vCurrent, err := semver.NewVersion(stripV(fs.Version))
	if err != nil {
		fs.Errorf(nil, "Failed to parse version: %v", err)
	}
	const timeFormat = "2006-01-02"

	printVersion := func(what, url string) {
		v, vs, t, err := GetVersion(ctx, url+"version.txt")
		if err != nil {
			fs.Errorf(nil, "Failed to get rclone %s version: %v", what, err)
			return
		}
		fmt.Printf("%-8s%-40v %20s\n",
			what+":",
			v,
			"(released "+t.Format(timeFormat)+")",
		)
		if v.Compare(*vCurrent) > 0 {
			fmt.Printf("  upgrade: %s\n", url+vs)
		}
	}
	fmt.Printf("yours:  %-13s\n", vCurrent)
	printVersion(
		"latest",
		"https://downloads.rclone.org/",
	)
	printVersion(
		"beta",
		"https://beta.rclone.org/",
	)
	if strings.HasSuffix(fs.Version, "-DEV") {
		fmt.Println("Your version is compiled from git so comparisons may be wrong.")
	}
}
