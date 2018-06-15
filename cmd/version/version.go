package version

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	check = false
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	flags := commandDefinition.Flags()
	flags.BoolVarP(&check, "check", "", false, "Check for new version.")
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

var parseVersion = regexp.MustCompile(`^(?:rclone )?v(\d+)\.(\d+)(?:\.(\d+))?(?:-(\d+)(?:-(g[\wβ-]+))?)?$`)

type version []int

func newVersion(in string) (v version, err error) {
	r := parseVersion.FindStringSubmatch(in)
	if r == nil {
		return v, errors.Errorf("failed to match version string %q", in)
	}
	atoi := func(s string) int {
		i, err := strconv.Atoi(s)
		if err != nil {
			fs.Errorf(nil, "Failed to parse %q as int from %q: %v", s, in, err)
		}
		return i
	}
	v = version{
		atoi(r[1]), // major
		atoi(r[2]), // minor
	}
	if r[3] != "" {
		v = append(v, atoi(r[3])) // patch
	} else if r[4] != "" {
		v = append(v, 0) // patch
	}
	if r[4] != "" {
		v = append(v, atoi(r[4])) // dev
	}
	return v, nil
}

// String converts v to a string
func (v version) String() string {
	var out []string
	for _, vv := range v {
		out = append(out, fmt.Sprint(vv))
	}
	return strings.Join(out, ".")
}

// cmp compares two versions returning >0, <0 or 0
func (v version) cmp(o version) (d int) {
	n := len(v)
	if n > len(o) {
		n = len(o)
	}
	for i := 0; i < n; i++ {
		d = v[i] - o[i]
		if d != 0 {
			return d
		}
	}
	return len(v) - len(o)
}

// getVersion gets the version by checking the download repository passed in
func getVersion(url string) (v version, vs string, date time.Time, err error) {
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
	v, err = newVersion(vs)
	return v, vs, date, err
}

// check the current version against available versions
func checkVersion() {
	// Get Current version
	currentVersion := fs.Version
	currentIsGit := strings.HasSuffix(currentVersion, "-DEV")
	if currentIsGit {
		currentVersion = currentVersion[:len(currentVersion)-4]
	}
	vCurrent, err := newVersion(currentVersion)
	if err != nil {
		fs.Errorf(nil, "Failed to get parse version: %v", err)
	}
	if currentIsGit {
		vCurrent = append(vCurrent, 999, 999)
	}

	const timeFormat = "2006-01-02"

	printVersion := func(what, url string) {
		v, vs, t, err := getVersion(url + "version.txt")
		if err != nil {
			fs.Errorf(nil, "Failed to get rclone %s version: %v", what, err)
			return
		}
		fmt.Printf("%-8s%-13v %20s\n",
			what+":",
			v,
			"(released "+t.Format(timeFormat)+")",
		)
		if v.cmp(vCurrent) > 0 {
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
	if currentIsGit {
		fmt.Println("Your version is compiled from git so comparisons may be wrong.")
	}
}
