package configfile

import (
	"fmt"
	"github.com/kevinburke/ssh_config"
	"os"
	"path/filepath"
	"strings"
)

var keyMapping = map[string]string{
	"identityfile": "key_file",
	"pubkeyfile":   "pubkey_file",
	"user":         "user",
	"hostname":     "host",
	"port":         "port",
	"password":     "pass",
}

func ConvertToIniKey(customKey string) string {
	if iniKey, found := keyMapping[strings.ToLower(customKey)]; found {
		return iniKey
	}

	return customKey
}

type SshConfig map[string]map[string]string

func LoadSshConfig() (*SshConfig, error) {
	path := filepath.Join(os.Getenv("HOME"), ".ssh", "config")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening ssh config file: %w", err)
	}

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("error deocing ssh config file: %w", err)
	}
	sections := SshConfig(map[string]map[string]string{})

	for _, host := range cfg.Hosts {
		pattern := host.Patterns[0].String()
		sections[pattern] = make(map[string]string)
		for _, node := range host.Nodes {

			keyval := strings.Fields(strings.TrimSpace(node.String()))
			if len(keyval) > 1 {
				key := strings.TrimSpace(keyval[0])
				value := strings.TrimSpace(strings.Join(keyval[1:], " "))

				sections[pattern][key] = value
			}
		}
	}

	return &sections, nil
}

func (s *Storage) MergeSshConfig(sshCfg SshConfig) error {
	//s.mu.Lock()
	//defer s.mu.Unlock()

	fmt.Println("Merging....")

	for sectionName, section := range sshCfg {
		// check if config already exist
		typeValue, _ := s.gc.GetValue(sectionName, "type")
		if typeValue == "" { // if not add ssh config

			// TODO: check if automatically guessing for ssh key file names is desired
			_, ok := section["identityfile"]
			if ok {
				//if _, ok := section["pubkey_file"]; !ok {
				//	section["pubkey_file"] = keyFile + ".pub"
				//}

				section["key_use_agent"] = "true"
				//section["md5sum_command"] = "md5sum"
				//section["sha1sum_command"] = "sha1sum"
			}

			section["type"] = "sftp"
			for key, value := range section {
				s.gc.SetValue(sectionName, ConvertToIniKey(key), value)
			}
		}
	}
	return nil
}
