package config

import (
	"context"
	"errors"
	"os"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
)

func init() {
	rc.Add(rc.Call{
		Path:         "config/dump",
		Fn:           rcDump,
		Title:        "Dumps the config file.",
		AuthRequired: true,
		Help: `
Returns a JSON object:
- key: value

Where keys are remote names and values are the config parameters.

See the [config dump](/commands/rclone_config_dump/) command for more information on the above.
`,
	})
}

// Return the config file dump
func rcDump(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	return DumpRcBlob(), nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "config/decrypt",
		Fn:           rcDecrypt,
		Title:        "Decrypts the Rclone config file.",
		AuthRequired: true,
		Help:         `Decrypt the Rclone configuration file. Requires login.`,
	})
}

// Decrypt the Rclone config file
func rcDecrypt(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	ClearConfigPassword()
	SaveConfig()
	return nil, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "config/encrypt",
		Fn:           rcEncrypt,
		Title:        "Encrypts the Rclone config file.",
		AuthRequired: false,
		Help: `
Parameters:
- password: password to use for encryption

Returns nil for success or a JSON object on error:
- error: error_message

Encrypts the config file. After encryption you will need to be logged in to use Rclone. 
Passwords are not recoverable.
`,
	})
}

// Encrypts the Rclone config file
func rcEncrypt(ctx context.Context, in rc.Params) (out rc.Params, err error) {

	passwd, err := in.GetString("password")
	if err != nil {
		out = rc.Params{
			"error": err,
		}
	} else if passwd == "" {
		out = rc.Params{
			"error": "password paramater not supplied",
		}
	} else {
		err := SetConfigPassword(passwd)
		if err != nil {
			return nil, err
		}
		SaveConfig()
		out = nil
	}
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "config/get",
		Fn:           rcGet,
		Title:        "Get a remote in the config file.",
		AuthRequired: true,
		Help: `
Parameters:

- name - name of remote to get

See the [config dump](/commands/rclone_config_dump/) command for more information on the above.
`,
	})
}

// Return the config file get
func rcGet(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	name, err := in.GetString("name")
	if err != nil {
		return nil, err
	}
	return DumpRcRemote(name), nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "config/listremotes",
		Fn:           rcListRemotes,
		Title:        "Lists the remotes in the config file and defined in environment variables.",
		AuthRequired: true,
		Help: `
Returns
- remotes - array of remote names

See the [listremotes](/commands/rclone_listremotes/) command for more information on the above.
`,
	})
}

// Return the a list of remotes in the config file
// including any defined by environment variables.
func rcListRemotes(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	remotes := FileSections()
	out = rc.Params{
		"remotes": remotes,
	}
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "config/providers",
		Fn:           rcProviders,
		Title:        "Shows how providers are configured in the config file.",
		AuthRequired: true,
		Help: `
Returns a JSON object:
- providers - array of objects

See the [config providers](/commands/rclone_config_providers/) command for more information on the above.
`,
	})
}

// Return the config file providers
func rcProviders(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	out = rc.Params{
		"providers": fs.Registry,
	}
	return out, nil
}

func init() {
	for _, name := range []string{"create", "update", "password"} {
		name := name
		extraHelp := ""
		if name == "create" {
			extraHelp = "- type - type of the new remote\n"
		}
		if name == "create" || name == "update" {
			extraHelp += `- opt - a dictionary of options to control the configuration
    - obscure - declare passwords are plain and need obscuring
    - noObscure - declare passwords are already obscured and don't need obscuring
    - nonInteractive - don't interact with a user, return questions
    - continue - continue the config process with an answer
    - all - ask all the config questions not just the post config ones
    - state - state to restart with - used with continue
    - result - result to restart with - used with continue
`
		}
		rc.Add(rc.Call{
			Path:         "config/" + name,
			AuthRequired: true,
			Fn: func(ctx context.Context, in rc.Params) (rc.Params, error) {
				return rcConfig(ctx, in, name)
			},
			Title: name + " the config for a remote.",
			Help: `This takes the following parameters:

- name - name of remote
- parameters - a map of \{ "key": "value" \} pairs
` + extraHelp + `

See the [config ` + name + `](/commands/rclone_config_` + name + `/) command for more information on the above.`,
		})
	}
}

// Manipulate the config file
func rcConfig(ctx context.Context, in rc.Params, what string) (out rc.Params, err error) {
	name, err := in.GetString("name")
	if err != nil {
		return nil, err
	}
	parameters := rc.Params{}
	err = in.GetStruct("parameters", &parameters)
	if err != nil {
		return nil, err
	}
	var opt UpdateRemoteOpt
	err = in.GetStruct("opt", &opt)
	if err != nil && !rc.IsErrParamNotFound(err) {
		return nil, err
	}
	// Backwards compatibility
	if value, err := in.GetBool("obscure"); err == nil {
		opt.Obscure = value
	}
	if value, err := in.GetBool("noObscure"); err == nil {
		opt.NoObscure = value
	}
	var configOut *fs.ConfigOut
	switch what {
	case "create":
		remoteType, typeErr := in.GetString("type")
		if typeErr != nil {
			return nil, typeErr
		}
		configOut, err = CreateRemote(ctx, name, remoteType, parameters, opt)
	case "update":
		configOut, err = UpdateRemote(ctx, name, parameters, opt)
	case "password":
		err = PasswordRemote(ctx, name, parameters)
	default:
		err = errors.New("unknown rcConfig type")
	}
	if err != nil {
		return nil, err
	}
	if !opt.NonInteractive {
		return nil, nil
	}
	if configOut == nil {
		configOut = &fs.ConfigOut{}
	}
	err = rc.Reshape(&out, configOut)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "config/delete",
		Fn:           rcDelete,
		Title:        "Delete a remote in the config file.",
		AuthRequired: true,
		Help: `
Parameters:

- name - name of remote to delete

See the [config delete](/commands/rclone_config_delete/) command for more information on the above.
`,
	})
}

// Return the config file delete
func rcDelete(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	name, err := in.GetString("name")
	if err != nil {
		return nil, err
	}
	DeleteRemote(name)
	return nil, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "config/setpath",
		Fn:           rcSetPath,
		Title:        "Set the path of the config file",
		AuthRequired: true,
		Help: `
Parameters:

- path - path to the config file to use
`,
	})
}

// Set the config file path
func rcSetPath(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	path, err := in.GetString("path")
	if err != nil {
		return nil, err
	}
	err = SetConfigPath(path)
	return nil, err
}

func init() {
	rc.Add(rc.Call{
		Path:         "config/paths",
		Fn:           rcPaths,
		Title:        "Reads the config file path and other important paths.",
		AuthRequired: true,
		Help: `
Returns a JSON object with the following keys:

- config: path to config file
- cache: path to root of cache directory
- temp: path to root of temporary directory

Eg

    {
        "cache": "/home/USER/.cache/rclone",
        "config": "/home/USER/.rclone.conf",
        "temp": "/tmp"
    }

See the [config paths](/commands/rclone_config_paths/) command for more information on the above.
`,
	})
}

// Set the config file path
func rcPaths(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	return rc.Params{
		"config": GetConfigPath(),
		"cache":  GetCacheDir(),
		"temp":   os.TempDir(),
	}, nil
}
