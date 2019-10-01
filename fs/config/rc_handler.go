package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/rc"
)

// DumpRcRemote dumps the config for a single remote
func DumpRcRemote(name string) (dump rc.Params) {
	params := rc.Params{}
	for key, v := range GetRemoteConfig().GetRemote(name).GetConfig() {
		params[key] = v
	}
	return params
}

// DumpRcBlob dumps all the config as an unstructured blob suitable
// for the rc
func DumpRcBlob() (dump rc.Params) {
	dump = rc.Params{}
	for _, name := range GetRemoteConfig().GetRemotes() {
		dump[name] = DumpRcRemote(name)
	}
	return dump
}

// Dump dumps all the config as a JSON file
func Dump() error {
	dump := DumpRcBlob()
	b, err := json.MarshalIndent(dump, "", "    ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal config dump")
	}
	_, err = os.Stdout.Write(b)
	if err != nil {
		return errors.Wrap(err, "failed to write config dump")
	}
	return nil
}

// DeleteRemote gets the user to delete a remote
func DeleteRemote(name string) {
	GetRemoteConfig().DeleteRemote(name)
}

// UpdateRemote adds the keyValues passed in to the remote of name.
// keyValues should be key, value pairs.
func UpdateRemote(name string, keyValues rc.Params) error {

	// Work out which options need to be obscured
	needsObscure := map[string]struct{}{}
	if fsType := GetRemoteConfig().GetRemote(name).GetString("type"); fsType != "" {
		if ri, err := fs.Find(fsType); err != nil {
			fs.Debugf(nil, "Couldn't find fs for type %q", fsType)
		} else {
			for _, opt := range ri.Options {
				if opt.IsPassword {
					needsObscure[opt.Name] = struct{}{}
				}
			}
		}
	} else {
		fs.Debugf(nil, "UpdateRemote: Couldn't find fs type")
	}

	// Set the config
	for k, v := range keyValues {
		vStr := fmt.Sprint(v)
		// Obscure parameter if necessary
		if _, ok := needsObscure[k]; ok {
			_, err := obscure.Reveal(vStr)
			if err != nil {
				// If error => not already obscured, so obscure it
				vStr, err = obscure.Obscure(vStr)
				if err != nil {
					return errors.Wrap(err, "UpdateRemote: obscure failed")
				}
			}
		}
		GetRemoteConfig().GetRemote(name).SetString(k, vStr)
	}
	//RunRemoteConfig(name)
	return nil
}

// CreateRemote creates a new remote with name, provider and a list of
// parameters which are key, value pairs.  If update is set then it
// adds the new keys rather than replacing all of them.
func CreateRemote(name string, provider string, keyValues rc.Params) error {
	// Delete the old config if it exists
	remote := GetRemoteConfig().CreateRemote(name)
	// Set the type
	remote.SetString("type", provider)
	// Set the remaining values
	return UpdateRemote(name, keyValues)
}

// PasswordRemote adds the keyValues passed in to the remote of name.
// keyValues should be key, value pairs.
func PasswordRemote(name string, keyValues rc.Params) error {
	for k, v := range keyValues {
		keyValues[k] = obscure.MustObscure(fmt.Sprint(v))
	}
	return UpdateRemote(name, keyValues)
}
