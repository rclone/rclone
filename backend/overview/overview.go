// Package overview provides info about a backend
package overview

import (
	"fmt"
	"strings"

	"github.com/rclone/rclone/docs/data/backends"
	"gopkg.in/yaml.v3"
)

// BackendConfig defines information about the backend
type BackendConfig struct {
	Backend          string   `yaml:"backend"`
	Name             string   `yaml:"name"`
	Tier             string   `yaml:"tier"`
	Maintainers      string   `yaml:"maintainers"`
	FeaturesScore    int      `yaml:"features_score"`
	IntegrationTests string   `yaml:"integration_tests"`
	DataIntegrity    string   `yaml:"data_integrity"`
	Performance      string   `yaml:"performance"`
	Adoption         string   `yaml:"adoption"`
	Docs             string   `yaml:"docs"`
	Security         string   `yaml:"security"`
	Virtual          bool     `yaml:"virtual"`
	Remote           string   `yaml:"remote"`
	Features         []string `yaml:"features"`
	Hashes           []string `yaml:"hashes"`
	Precision        int      `yaml:"precision"`
}

// GetBackendConfig from docs/data/backends
func GetBackendConfig(name string) (*BackendConfig, error) {
	fileName := fmt.Sprintf("%s.yaml", strings.ToLower(name))

	data, err := backends.BackendFS.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("could not find backend file %s: %w", fileName, err)
	}

	var config BackendConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML in %s: %w", fileName, err)
	}

	return &config, nil
}
