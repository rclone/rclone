package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func init() {
	// This block is run super-early, before configuration harness kick in
	if IsMountHelper() {
		if args, err := convertMountHelperArgs(os.Args); err == nil {
			os.Args = args
		} else {
			Fatalf(nil, "Failed to parse command line: %v", err)
		}
	}
}

// PassDaemonArgsAsEnviron tells how CLI arguments are passed to the daemon
// When false, arguments are passed as is, visible in the `ps` output.
// When true, arguments are converted into environment variables (more secure).
var PassDaemonArgsAsEnviron bool

// Comma-separated list of mount options to ignore.
// Leading and trailing commas are required.
const helperIgnoredOpts = ",rw,_netdev,nofail,user,dev,nodev,suid,nosuid,exec,noexec,auto,noauto,"

// Valid option name characters
const helperValidOptChars = "-_0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Parser errors
var (
	errHelperBadOption    = errors.New("option names may only contain `0-9`, `A-Z`, `a-z`, `-` and `_`")
	errHelperOptionName   = errors.New("option name can't start with `-` or `_`")
	errHelperEmptyOption  = errors.New("option name can't be empty")
	errHelperQuotedValue  = errors.New("unterminated quoted value")
	errHelperAfterQuote   = errors.New("expecting `,` or another quote after a quote")
	errHelperSyntax       = errors.New("syntax error in option string")
	errHelperEmptyCommand = errors.New("command name can't be empty")
	errHelperEnvSyntax    = errors.New("environment variable must have syntax env.NAME=[VALUE]")
)

// IsMountHelper returns true if rclone was invoked as mount helper:
// as /sbin/mount.rlone (by /bin/mount)
// or /usr/bin/rclonefs (by fusermount or directly)
func IsMountHelper() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	me := filepath.Base(os.Args[0])
	return me == "mount.rclone" || me == "rclonefs"
}

// convertMountHelperArgs converts "-o" styled mount helper arguments
// into usual rclone flags
func convertMountHelperArgs(origArgs []string) ([]string, error) {
	if IsDaemon() {
		// The arguments have already been converted by the parent
		return origArgs, nil
	}

	args := []string{}
	command := "mount"
	parseOpts := false
	gotDaemon := false
	gotVerbose := false
	vCount := 0

	for _, arg := range origArgs[1:] {
		if !parseOpts {
			switch arg {
			case "-o", "--opt":
				parseOpts = true
			case "-v", "-vv", "-vvv", "-vvvv":
				vCount += len(arg) - 1
			case "-h", "--help":
				args = append(args, "--help")
			default:
				if strings.HasPrefix(arg, "-") {
					return nil, fmt.Errorf("flag %q is not supported in mount mode", arg)
				}
				args = append(args, arg)
			}
			continue
		}

		opts, err := parseHelperOptionString(arg)
		if err != nil {
			return nil, err
		}
		parseOpts = false

		for _, opt := range opts {
			if strings.Contains(helperIgnoredOpts, ","+opt+",") || strings.HasPrefix(opt, "x-systemd") {
				continue
			}

			param, value, _ := strings.Cut(opt, "=")

			// Set environment variables
			if strings.HasPrefix(param, "env.") {
				if param = param[4:]; param == "" {
					return nil, errHelperEnvSyntax
				}
				_ = os.Setenv(param, value)
				continue
			}

			switch param {
			// Change command to run
			case "command":
				if value == "" {
					return nil, errHelperEmptyCommand
				}
				command = value
				continue
			// Flag StartDaemon to pass arguments as environment
			case "args2env":
				PassDaemonArgsAsEnviron = true
				continue
			// Handle verbosity options
			case "v", "vv", "vvv", "vvvv":
				vCount += len(param)
				continue
			case "verbose":
				gotVerbose = true
			// Don't add --daemon if it was explicitly included
			case "daemon":
				gotDaemon = true
			// Alias for the standard mount option "ro"
			case "ro":
				param = "read-only"
			}

			arg = "--" + strings.ToLower(strings.ReplaceAll(param, "_", "-"))
			if value != "" {
				arg += "=" + value
			}
			args = append(args, arg)
		}
	}
	if parseOpts {
		return nil, fmt.Errorf("dangling -o without argument")
	}

	if vCount > 0 && !gotVerbose {
		args = append(args, fmt.Sprintf("--verbose=%d", vCount))
	}
	if strings.Contains(command, "mount") && !gotDaemon {
		// Default to daemonized mount
		args = append(args, "--daemon")
	}
	if len(args) > 0 && args[0] == command {
		// Remove artefact of repeated conversion
		args = args[1:]
	}
	prepend := []string{origArgs[0], command}
	return append(prepend, args...), nil
}

// parseHelperOptionString deconstructs the -o value into slice of options
// in a way similar to connection strings.
// Example:
//
//	param1=value,param2="qvalue",param3='item1,item2',param4="a ""b"" 'c'"
//
// An error may be returned if the remote name has invalid characters
// or the parameters are invalid or the path is empty.
//
// The algorithm was adapted from fspath.Parse with some modifications:
// - allow `-` in option names
// - handle special options `x-systemd.X` and `env.X`
// - drop support for :backend: and /path
func parseHelperOptionString(optString string) (opts []string, err error) {
	if optString = strings.TrimSpace(optString); optString == "" {
		return nil, nil
	}
	// States for parser
	const (
		stateParam = uint8(iota)
		stateValue
		stateQuotedValue
		stateAfterQuote
		stateDone
	)
	var (
		state   = stateParam // current state of parser
		i       int          // position in path
		prev    int          // previous position in path
		c       rune         // current rune under consideration
		quote   rune         // kind of quote to end this quoted string
		param   string       // current parameter value
		doubled bool         // set if had doubled quotes
	)
	for i, c = range optString + "," {
		switch state {
		// Parses param= and param2=
		case stateParam:
			switch c {
			case ',', '=':
				param = optString[prev:i]
				if len(param) == 0 {
					return nil, errHelperEmptyOption
				}
				if param[0] == '-' {
					return nil, errHelperOptionName
				}
				prev = i + 1
				if c == '=' {
					state = stateValue
					break
				}
				opts = append(opts, param)
			case '.':
				if pref := optString[prev:i]; pref != "env" && pref != "x-systemd" {
					return nil, errHelperBadOption
				}
			default:
				if !strings.ContainsRune(helperValidOptChars, c) {
					return nil, errHelperBadOption
				}
			}
		case stateValue:
			switch c {
			case '\'', '"':
				if i == prev {
					quote = c
					prev = i + 1
					doubled = false
					state = stateQuotedValue
				}
			case ',':
				value := optString[prev:i]
				prev = i + 1
				opts = append(opts, param+"="+value)
				state = stateParam
			}
		case stateQuotedValue:
			if c == quote {
				state = stateAfterQuote
			}
		case stateAfterQuote:
			switch c {
			case ',':
				value := optString[prev : i-1]
				// replace any doubled quotes if there were any
				if doubled {
					value = strings.ReplaceAll(value, string(quote)+string(quote), string(quote))
				}
				prev = i + 1
				opts = append(opts, param+"="+value)
				state = stateParam
			case quote:
				// Here is a doubled quote to indicate a literal quote
				state = stateQuotedValue
				doubled = true
			default:
				return nil, errHelperAfterQuote
			}
		}
	}

	// Depending on which state we were in when we fell off the
	// end of the state machine we can return a sensible error.
	if state == stateParam && prev > len(optString) {
		state = stateDone
	}
	switch state {
	case stateQuotedValue:
		return nil, errHelperQuotedValue
	case stateAfterQuote:
		return nil, errHelperAfterQuote
	case stateDone:
		break
	default:
		return nil, errHelperSyntax
	}
	return opts, nil
}
