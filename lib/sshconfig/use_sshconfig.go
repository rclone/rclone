package sshconfig

import (
	"fmt"
	"github.com/kevinburke/ssh_config"
	"io"
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

func LoadSshConfigIntoEnv() error {
	if c, err := LoadSshConfig(); err != nil {
		return fmt.Errorf("error loading ssh config: %v", err)
	} else {
		if err := EnvLoadSshConfig(*c); err != nil {
			return fmt.Errorf("error setting Env with ssh config: %v", err)
		}
	}
	return nil
}

func LoadSshConfig() (*SshConfig, error) {
	path := filepath.Join(os.Getenv("HOME"), ".ssh", "config")
	f, err := os.Open(path) // returning file handler, so caller needs close

	if err != nil {
		return nil, fmt.Errorf("error opening ssh config file: %w", err)
	}
	return MapSshToRcloneConfig(f)
}

func MapSshToRcloneConfig(r io.Reader) (*SshConfig, error) {
	cfg, err := ssh_config.Decode(r)
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
				key := strings.ToLower(strings.TrimSpace(keyval[0]))
				value := strings.TrimSpace(strings.Join(keyval[1:], " "))

				//oldValue, containsKey := sections[pattern][key]
				//if containsKey {
				//	sections[pattern][key] = oldValue + "," + value
				//}

				sections[pattern][key] = value
			}
		}

		//keyFile, err := cfg.GetAll(pattern, "identityfile")
		//fmt.Println("Identi", pattern)
		//if err != nil {
		//	fmt.Println("Error", err)
		//} else {
		//	if len(keyFile) == 2 {
		//		//keyFile = []string{keyFile[1], keyFile[0]}
		//	}
		//	sections[pattern]["identityfile"] = strings.Join(keyFile, ",")
		//}
	}
	return &sections, nil
}

func EnvLoadSshConfig(sshCfg SshConfig) error {
	for sectionName, section := range sshCfg {
		//fmt.Println(sectionName, section)
		// check if config already exist
		//typeValue, _ := s.gc.GetValue(sectionName, "type")

		_, ok := section["identityfile"]
		if ok {
			//if _, ok := section["pubkey_file"]; !ok {
			//	section["pubkey_file"] = keyFile + ".pub"
			//}
			section["key_use_agent"] = "true"
		}

		section["type"] = "sftp"
		for key, value := range section {
			s := fmt.Sprintf("RCLONE_CONFIG_%s_%s", strings.ToUpper(sectionName), strings.ToUpper(ConvertToIniKey(key)))
			if err := os.Setenv(s, value); err != nil {
				return err
			}
		}
	}
	return nil
}
