package config

import (
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/rc"
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

See the [config dump command](/commands/rclone_config_dump/) command for more information on the above.
`,
	})
}

// Return the config file dump
func rcDump(in rc.Params) (out rc.Params, err error) {
	return DumpRcBlob(), nil
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

See the [config dump command](/commands/rclone_config_dump/) command for more information on the above.
`,
	})
}

// Return the config file get
func rcGet(in rc.Params) (out rc.Params, err error) {
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
		Title:        "Lists the remotes in the config file.",
		AuthRequired: true,
		Help: `
Returns
- remotes - array of remote names

See the [listremotes command](/commands/rclone_listremotes/) command for more information on the above.
`,
	})
}

// Return the a list of remotes in the config file
func rcListRemotes(in rc.Params) (out rc.Params, err error) {
	var remotes = []string{}
	for _, remote := range getConfigData().GetSectionList() {
		remotes = append(remotes, remote)
	}
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

See the [config providers command](/commands/rclone_config_providers/) command for more information on the above.
`,
	})
}

// Return the config file providers
func rcProviders(in rc.Params) (out rc.Params, err error) {
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
		rc.Add(rc.Call{
			Path:         "config/" + name,
			AuthRequired: true,
			Fn: func(in rc.Params) (rc.Params, error) {
				return rcConfig(in, name)
			},
			Title: name + " the config for a remote.",
			Help: `This takes the following parameters

- name - name of remote
- type - type of new remote
` + extraHelp + `

See the [config ` + name + ` command](/commands/rclone_config_` + name + `/) command for more information on the above.`,
		})
	}
}

// Manipulate the config file
func rcConfig(in rc.Params, what string) (out rc.Params, err error) {
	name, err := in.GetString("name")
	if err != nil {
		return nil, err
	}
	parameters := rc.Params{}
	err = in.GetStruct("parameters", &parameters)
	if err != nil {
		return nil, err
	}
	switch what {
	case "create":
		remoteType, err := in.GetString("type")
		if err != nil {
			return nil, err
		}
		return nil, CreateRemote(name, remoteType, parameters)
	case "update":
		return nil, UpdateRemote(name, parameters)
	case "password":
		return nil, PasswordRemote(name, parameters)
	}
	panic("unknown rcConfig type")
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

See the [config delete command](/commands/rclone_config_delete/) command for more information on the above.
`,
	})
}

// Return the config file delete
func rcDelete(in rc.Params) (out rc.Params, err error) {
	name, err := in.GetString("name")
	if err != nil {
		return nil, err
	}
	DeleteRemote(name)
	return nil, nil
}
