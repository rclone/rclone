// Package sshconfig functions to convert ssh config file to rclone config and to add to temp env vars
package sshconfig

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kevinburke/ssh_config"
)

// keyMapping defines the mapping between SSH configuration keys and Rclone configuration keys.
var keyMapping = map[string]string{
	"identityfile": "key_file",
	"pubkeyfile":   "pubkey_file",
	"user":         "user",
	"hostname":     "host",
	"port":         "port",
	"password":     "pass",
}

// sshConfig represents the SSH configuration structure
type sshConfig map[string]map[string]string

// LoadSSHConfigIntoEnv loads SSH configuration into environment variables by mapping SSH settings to
// rclone configuration and setting the environment accordingly.
// Returns an error if any step fails during the process.
// Note: type=sftp is added for each host (section), also key_use_agent=true is set, when if key_file was given.
func LoadSSHConfigIntoEnv() error {
	path := filepath.Join(os.Getenv("HOME"), ".ssh", "config")
	f, err := os.Open(path) // returning file handler, so caller needs close

	if err != nil {
		return fmt.Errorf("error opening ssh config file: %w", err)
	}

	c, err := mapSSHToRcloneConfig(f)
	if err != nil {
		return fmt.Errorf("error mapping ssh config to rclone config: %w", err)
	}

	if err := EnvLoadSSHConfig(c); err != nil {
		return fmt.Errorf("error setting Env with ssh config: %v", err)
	}

	return nil
}

// EnvLoadSSHConfig sets the environment variables based on the Ssh configuration.
func EnvLoadSSHConfig(sshCfg sshConfig) error {
	for sectionName, section := range sshCfg {

		for key, value := range section {
			s := fmt.Sprintf("RCLONE_CONFIG_%s_%s", strings.ToUpper(sectionName), strings.ToUpper(convertToIniKey(key)))
			if err := os.Setenv(s, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// convertToIniKey converts custom SSH configuration keys to Rclone configuration keys using keyMapping.
func convertToIniKey(customKey string) string {
	if iniKey, found := keyMapping[strings.ToLower(customKey)]; found {
		return iniKey
	}

	return customKey
}

// mapSSHToRcloneConfig maps Ssh configuration to rclone configuration.
func mapSSHToRcloneConfig(r io.Reader) (sshConfig, error) {
	cfg, err := ssh_config.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("error deocing ssh config file: %w", err)
	}
	sections := sshConfig(map[string]map[string]string{})

	for _, host := range cfg.Hosts {
		pattern := host.Patterns[0].String()
		sections[pattern] = make(map[string]string)

		// ssh configs are always type sftp
		sections[pattern]["type"] = "sftp"

		for _, node := range host.Nodes {
			keyval := strings.Fields(strings.TrimSpace(node.String()))
			if len(keyval) > 1 {
				key := strings.ToLower(strings.TrimSpace(keyval[0]))
				value := strings.TrimSpace(strings.Join(keyval[1:], " "))

				sections[pattern][convertToIniKey(key)] = value
			}
		}

		// add missing key_use_agent if there is identityfile or here mapped key_file
		_, ok := sections[pattern]["key_file"]
		if ok {
			sections[pattern]["key_use_agent"] = "true"
		}
	}
	return sections, nil
}
