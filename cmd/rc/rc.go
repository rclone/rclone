package rc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/jobs"
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
	options   []string
	arguments []string
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	cmdFlags := commandDefinition.Flags()
	flags.BoolVarP(cmdFlags, &noOutput, "no-output", "", noOutput, "If set, don't output the JSON result")
	flags.StringVarP(cmdFlags, &url, "url", "", url, "URL to connect to rclone remote control")
	flags.StringVarP(cmdFlags, &jsonInput, "json", "", jsonInput, "Input JSON - use instead of key=value args")
	flags.StringVarP(cmdFlags, &authUser, "user", "", "", "Username to use to rclone remote control")
	flags.StringVarP(cmdFlags, &authPass, "pass", "", "", "Password to use to connect to rclone remote control")
	flags.BoolVarP(cmdFlags, &loopback, "loopback", "", false, "If set connect to this rclone instance not via HTTP")
	flags.StringArrayVarP(cmdFlags, &options, "opt", "o", options, "Option in the form name=value or name placed in the \"opt\" array")
	flags.StringArrayVarP(cmdFlags, &arguments, "arg", "a", arguments, "Argument placed in the \"arg\" array")
}

var commandDefinition = &cobra.Command{
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

The -o/--opt option can be used to set a key "opt" with key, value
options in the form "-o key=value" or "-o key". It can be repeated as
many times as required. This is useful for rc commands which take the
"opt" parameter which by convention is a dictionary of strings.

    -o key=value -o key2

Will place this in the "opt" value

    {"key":"value", "key2","")


The -a/--arg option can be used to set strings in the "arg" value. It
can be repeated as many times as required. This is useful for rc
commands which take the "arg" parameter which by convention is a list
of strings.

    -a value -a value2

Will place this in the "arg" value

    ["value", "value2"]

Use --loopback to connect to the rclone instance running "rclone rc".
This is very useful for testing commands without having to run an
rclone rc server, e.g.:

    rclone rc --loopback operations/about fs=/

Use "rclone rc" to see a list of all possible commands.`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 1e9, command, args)
		cmd.Run(false, false, command, func() error {
			ctx := context.Background()
			parseFlags()
			if len(args) == 0 {
				return list(ctx)
			}
			return run(ctx, args)
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

// ParseOptions parses a slice of options in the form key=value or key
// into a map
func ParseOptions(options []string) (opt map[string]string) {
	opt = make(map[string]string, len(options))
	for _, option := range options {
		equals := strings.IndexRune(option, '=')
		key := option
		value := ""
		if equals >= 0 {
			key = option[:equals]
			value = option[equals+1:]
		}
		opt[key] = value
	}
	return opt
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
func doCall(ctx context.Context, path string, in rc.Params) (out rc.Params, err error) {
	// If loopback set, short circuit HTTP request
	if loopback {
		call := rc.Calls.Get(path)
		if call == nil {
			return nil, fmt.Errorf("method %q not found", path)
		}
		_, out, err := jobs.NewJob(ctx, call.Fn, in)
		if err != nil {
			return nil, fmt.Errorf("loopback call failed: %w", err)
		}
		// Reshape (serialize then deserialize) the data so it is in the form expected
		err = rc.Reshape(&out, out)
		if err != nil {
			return nil, fmt.Errorf("loopback reshape failed: %w", err)
		}
		return out, nil
	}

	// Do HTTP request
	client := fshttp.NewClient(ctx)
	url += path
	data, err := json.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("failed to encode JSON: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if authUser != "" || authPass != "" {
		req.SetBasicAuth(authUser, authPass)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
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
		return nil, fmt.Errorf("Failed to read rc response: %s: %s", resp.Status, bodyString)
	}

	// Parse output
	out = make(rc.Params)
	err = json.NewDecoder(resp.Body).Decode(&out)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	// Check we got 200 OK
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("operation %q failed: %v", path, out["error"])
	}

	return out, err
}

// Run the remote control command passed in
func run(ctx context.Context, args []string) (err error) {
	path := strings.Trim(args[0], "/")

	// parse input
	in := make(rc.Params)
	params := args[1:]
	if jsonInput == "" {
		for _, param := range params {
			equals := strings.IndexRune(param, '=')
			if equals < 0 {
				return fmt.Errorf("no '=' found in parameter %q", param)
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
			return fmt.Errorf("bad --json input: %w", err)
		}
	}
	if len(options) > 0 {
		in["opt"] = ParseOptions(options)
	}
	if len(arguments) > 0 {
		in["arg"] = arguments
	}

	// Do the call
	out, callErr := doCall(ctx, path, in)

	// Write the JSON blob to stdout if required
	if out != nil && !noOutput {
		err := rc.WriteJSON(os.Stdout, out)
		if err != nil {
			return fmt.Errorf("failed to output JSON: %w", err)
		}
	}

	return callErr
}

// List the available commands to stdout
func list(ctx context.Context) error {
	list, err := doCall(ctx, "rc/list", nil)
	if err != nil {
		return fmt.Errorf("failed to list: %w", err)
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
		fmt.Printf("### %s: %s {#%s}\n\n", info["Path"], info["Title"], strings.ReplaceAll(info["Path"].(string), "/", "-"))
		fmt.Printf("%s\n\n", info["Help"])
		if authRequired := info["AuthRequired"]; authRequired != nil {
			if authRequired.(bool) {
				fmt.Printf("**Authentication is required for this call.**\n\n")
			}
		}
	}
	return nil
}
