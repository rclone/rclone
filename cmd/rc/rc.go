package rc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/ncw/rclone/cmd"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/fshttp"
	"github.com/ncw/rclone/fs/rc"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	noOutput = false
	url      = "http://localhost:5572/"
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
	commandDefintion.Flags().BoolVarP(&noOutput, "no-output", "", noOutput, "If set don't output the JSON result.")
	commandDefintion.Flags().StringVarP(&url, "url", "", url, "URL to connect to rclone remote control.")
}

var commandDefintion = &cobra.Command{
	Use:   "rc commands parameter",
	Short: `Run a command against a running rclone.`,
	Long: `
This runs a command against a running rclone.  By default it will use
that specified in the --rc-addr command.

Arguments should be passed in as parameter=value.

The result will be returned as a JSON object by default.

Use "rclone rc" to see a list of all possible commands.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1E9, command, args)
		cmd.Run(false, false, command, func() error {
			if len(args) == 0 {
				return list()
			}
			return run(args)
		})
	},
}

// do a call from (path, in) to (out, err).
//
// if err is set, out may be a valid error return or it may be nil
func doCall(path string, in rc.Params) (out rc.Params, err error) {
	// Do HTTP request
	client := fshttp.NewClient(fs.Config)
	url := url
	// set the user use --rc-addr as well as --url
	if rcAddrFlag := pflag.Lookup("rc-addr"); rcAddrFlag != nil && rcAddrFlag.Changed {
		url = rcAddrFlag.Value.String()
		if strings.HasPrefix(url, ":") {
			url = "localhost" + url
		}
		url = "http://" + url + "/"
	}
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	url += path
	data, err := json.Marshal(in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode JSON")
	}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, errors.Wrap(err, "connection failed")
	}
	defer fs.CheckClose(resp.Body, &err)

	if resp.StatusCode != http.StatusOK {
		var body []byte
		body, err = ioutil.ReadAll(resp.Body)
		var bodyString string
		if err == nil {
			bodyString = string(body)
		} else {
			bodyString = err.Error()
		}
		bodyString = strings.TrimSpace(bodyString)
		return nil, errors.Errorf("Failed to read rc response: %s: %s", resp.Status, bodyString)
	}

	// Parse output
	out = make(rc.Params)
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode JSON")
	}

	// Check we got 200 OK
	if resp.StatusCode != http.StatusOK {
		err = errors.Errorf("operation %q failed: %v", path, out["error"])
	}

	return out, err
}

// Run the remote control command passed in
func run(args []string) (err error) {
	path := strings.Trim(args[0], "/")

	// parse input
	in := make(rc.Params)
	for _, param := range args[1:] {
		equals := strings.IndexRune(param, '=')
		if equals < 0 {
			return errors.Errorf("No '=' found in parameter %q", param)
		}
		key, value := param[:equals], param[equals+1:]
		in[key] = value
	}

	// Do the call
	out, callErr := doCall(path, in)

	// Write the JSON blob to stdout if required
	if out != nil && !noOutput {
		err := rc.WriteJSON(os.Stdout, out)
		if err != nil {
			return errors.Wrap(err, "failed to output JSON")
		}
	}

	return callErr
}

// List the available commands to stdout
func list() error {
	list, err := doCall("rc/list", nil)
	if err != nil {
		return errors.Wrap(err, "failed to list")
	}
	commands, ok := list["commands"].([]interface{})
	if !ok {
		return errors.New("bad JSON")
	}
	for _, command := range commands {
		info, ok := command.(map[string]interface{})
		if !ok {
			return errors.New("bad JSON")
		}
		fmt.Printf("### %s: %s\n\n", info["Path"], info["Title"])
		fmt.Printf("%s\n\n", info["Help"])
	}
	return nil
}
