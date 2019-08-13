package rc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/rc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	noOutput  = false
	url       = "http://localhost:5572/"
	jsonInput = ""
	authUser  = ""
	authPass  = ""
	loopback  = false
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
	commandDefintion.Flags().BoolVarP(&noOutput, "no-output", "", noOutput, "If set don't output the JSON result.")
	commandDefintion.Flags().StringVarP(&url, "url", "", url, "URL to connect to rclone remote control.")
	commandDefintion.Flags().StringVarP(&jsonInput, "json", "", jsonInput, "Input JSON - use instead of key=value args.")
	commandDefintion.Flags().StringVarP(&authUser, "user", "", "", "Username to use to rclone remote control.")
	commandDefintion.Flags().StringVarP(&authPass, "pass", "", "", "Password to use to connect to rclone remote control.")
	commandDefintion.Flags().BoolVarP(&loopback, "loopback", "", false, "If set connect to this rclone instance not via HTTP.")
}

var commandDefintion = &cobra.Command{
	Use:   "rc commands parameter",
	Short: `Run a command against a running rclone.`,
	Long: `

This runs a command against a running rclone.  Use the --url flag to
specify an non default URL to connect on.  This can be either a
":port" which is taken to mean "http://localhost:port" or a
"host:port" which is taken to mean "http://host:port"

A username and password can be passed in with --user and --pass.

Note that --rc-addr, --rc-user, --rc-pass will be read also for --url,
--user, --pass.

Arguments should be passed in as parameter=value.

The result will be returned as a JSON object by default.

The --json parameter can be used to pass in a JSON blob as an input
instead of key=value arguments.  This is the only way of passing in
more complicated values.

Use --loopback to connect to the rclone instance running "rclone rc".
This is very useful for testing commands without having to run an
rclone rc server, eg:

    rclone rc --loopback operations/about fs=/

Use "rclone rc" to see a list of all possible commands.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1E9, command, args)
		cmd.Run(false, false, command, func() error {
			parseFlags()
			if len(args) == 0 {
				return list()
			}
			return run(args)
		})
	},
}

// Parse the flags
func parseFlags() {
	// set alternates from alternate flags
	setAlternateFlag("rc-addr", &url)
	setAlternateFlag("rc-user", &authUser)
	setAlternateFlag("rc-pass", &authPass)
	// If url is just :port then fix it up
	if strings.HasPrefix(url, ":") {
		url = "localhost" + url
	}
	// if url is just host:port add http://
	if !strings.HasPrefix(url, "http:") && !strings.HasPrefix(url, "https:") {
		url = "http://" + url
	}
	// if url doesn't end with / add it
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
}

// If the user set flagName set the output to its value
func setAlternateFlag(flagName string, output *string) {
	if rcFlag := pflag.Lookup(flagName); rcFlag != nil && rcFlag.Changed {
		*output = rcFlag.Value.String()
	}
}

// do a call from (path, in) to (out, err).
//
// if err is set, out may be a valid error return or it may be nil
func doCall(path string, in rc.Params) (out rc.Params, err error) {
	// If loopback set, short circuit HTTP request
	if loopback {
		call := rc.Calls.Get(path)
		if call == nil {
			return nil, errors.Errorf("method %q not found", path)
		}
		out, err = call.Fn(context.Background(), in)
		if err != nil {
			return nil, errors.Wrap(err, "loopback call failed")
		}
		// Reshape (serialize then deserialize) the data so it is in the form expected
		err = rc.Reshape(&out, out)
		if err != nil {
			return nil, errors.Wrap(err, "loopback reshape failed")
		}
		return out, nil
	}

	// Do HTTP request
	client := fshttp.NewClient(fs.Config)
	url += path
	data, err := json.Marshal(in)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode JSON")
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, errors.Wrap(err, "failed to make request")
	}

	req.Header.Set("Content-Type", "application/json")
	if authUser != "" || authPass != "" {
		req.SetBasicAuth(authUser, authPass)
	}

	resp, err := client.Do(req)
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
	params := args[1:]
	if jsonInput == "" {
		for _, param := range params {
			equals := strings.IndexRune(param, '=')
			if equals < 0 {
				return errors.Errorf("no '=' found in parameter %q", param)
			}
			key, value := param[:equals], param[equals+1:]
			in[key] = value
		}
	} else {
		if len(params) > 0 {
			return errors.New("can't use --json and parameters together")
		}
		err = json.Unmarshal([]byte(jsonInput), &in)
		if err != nil {
			return errors.Wrap(err, "bad --json input")
		}
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
		fmt.Printf("### %s: %s {#%s}\n\n", info["Path"], info["Title"], info["Path"])
		fmt.Printf("%s\n\n", info["Help"])
		if authRequired := info["AuthRequired"]; authRequired != nil {
			if authRequired.(bool) {
				fmt.Printf("Authentication is required for this call.\n\n")
			}
		}
	}
	return nil
}
