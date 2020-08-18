package version

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/spf13/cobra"
)

var (
	check = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &check, "check", "", false, "Check for new version.")
}

var commandDefinition = &cobra.Command{
	Use:   "version",
	Short: `Show the version number.`,
	Long: `
Show the version number, the go version and the architecture.

Eg

    $ rclone version
    rclone v1.41
    - os/arch: linux/amd64
    - go version: go1.10

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
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 0, command, args)
		if check {
			checkVersion()
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

// getVersion gets the version by checking the download repository passed in
func getVersion(url string) (v *semver.Version, vs string, date time.Time, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return v, vs, date, err
	}
	defer fs.CheckClose(resp.Body, &err)
	if resp.StatusCode != http.StatusOK {
		return v, vs, date, errors.New(resp.Status)
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return v, vs, date, err
	}
	vs = strings.TrimSpace(string(bodyBytes))
	if strings.HasPrefix(vs, "rclone ") {
		vs = vs[7:]
	}
	vs = strings.TrimRight(vs, "β")
	date, err = http.ParseTime(resp.Header.Get("Last-Modified"))
	if err != nil {
		return v, vs, date, err
	}
	v, err = semver.NewVersion(stripV(vs))
	return v, vs, date, err
}

// check the current version against available versions
func checkVersion() {
	// Get Current version
	vCurrent, err := semver.NewVersion(stripV(fs.Version))
	if err != nil {
		fs.Errorf(nil, "Failed to parse version: %v", err)
	}
	const timeFormat = "2006-01-02"

	printVersion := func(what, url string) {
		v, vs, t, err := getVersion(url + "version.txt")
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
