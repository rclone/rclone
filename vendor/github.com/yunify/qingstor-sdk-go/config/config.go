// +-------------------------------------------------------------------------
// | Copyright (C) 2016 Yunify, Inc.
// +-------------------------------------------------------------------------
// | Licensed under the Apache License, Version 2.0 (the "License");
// | you may not use this work except in compliance with the License.
// | You may obtain a copy of the License in the LICENSE file, or at:
// |
// | http://www.apache.org/licenses/LICENSE-2.0
// |
// | Unless required by applicable law or agreed to in writing, software
// | distributed under the License is distributed on an "AS IS" BASIS,
// | WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// | See the License for the specific language governing permissions and
// | limitations under the License.
// +-------------------------------------------------------------------------

package config

import (
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/pengsrc/go-shared/yaml"

	"github.com/yunify/qingstor-sdk-go/logger"
)

// A Config stores a configuration of this sdk.
type Config struct {
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`

	Host              string `yaml:"host"`
	Port              int    `yaml:"port"`
	Protocol          string `yaml:"protocol"`
	ConnectionRetries int    `yaml:"connection_retries"`

	AdditionalUserAgent string `yaml:"additional_user_agent"`

	LogLevel string `yaml:"log_level"`

	Connection *http.Client
}

// New create a Config with given AccessKeyID and SecretAccessKey.
func New(accessKeyID, secretAccessKey string) (*Config, error) {
	config, err := NewDefault()
	if err != nil {
		return nil, err
	}

	config.AccessKeyID = accessKeyID
	config.SecretAccessKey = secretAccessKey

	config.Connection = &http.Client{}

	return config, nil
}

// NewDefault create a Config with default configuration.
func NewDefault() (*Config, error) {
	config := &Config{}
	err := config.LoadDefaultConfig()
	if err != nil {
		return nil, err
	}
	config.Connection = &http.Client{}

	return config, nil
}

// Check checks the configuration.
func (c *Config) Check() error {
	if c.AccessKeyID == "" {
		return errors.New("access key ID not specified")
	}
	if c.SecretAccessKey == "" {
		return errors.New("secret access key not specified")
	}

	if c.Host == "" {
		return errors.New("server host not specified")
	}
	if c.Port <= 0 {
		return errors.New("server port not specified")
	}
	if c.Protocol == "" {
		return errors.New("server protocol not specified")
	}

	if c.AdditionalUserAgent != "" {
		for _, x := range c.AdditionalUserAgent {
			// Allow space(32) to ~(126) in ASCII Table, exclude "(34).
			if int(x) < 32 || int(x) > 126 || int(x) == 32 || int(x) == 34 {
				return errors.New("additional User-Agent contains characters that not allowed")
			}
		}
	}

	err := logger.CheckLevel(c.LogLevel)
	if err != nil {
		return err
	}

	return nil
}

// LoadDefaultConfig loads the default configuration for Config.
// It returns error if yaml decode failed.
func (c *Config) LoadDefaultConfig() error {
	_, err := yaml.Decode([]byte(DefaultConfigFileContent), c)
	if err != nil {
		logger.Errorf("Config parse error: " + err.Error())
		return err
	}

	logger.SetLevel(c.LogLevel)

	return nil
}

// LoadUserConfig loads user configuration in ~/.qingstor/config.yaml for Config.
// It returns error if file not found.
func (c *Config) LoadUserConfig() error {
	_, err := os.Stat(GetUserConfigFilePath())
	if err != nil {
		logger.Warnf("Installing default config file to \"" + GetUserConfigFilePath() + "\"")
		InstallDefaultUserConfig()
	}

	return c.LoadConfigFromFilePath(GetUserConfigFilePath())
}

// LoadConfigFromFilePath loads configuration from a specified local path.
// It returns error if file not found or yaml decode failed.
func (c *Config) LoadConfigFromFilePath(filepath string) error {
	if strings.Index(filepath, "~/") == 0 {
		filepath = strings.Replace(filepath, "~/", getHome()+"/", 1)
	}

	yamlString, err := ioutil.ReadFile(filepath)
	if err != nil {
		logger.Errorf("File not found: " + filepath)
		return err
	}

	return c.LoadConfigFromContent(yamlString)
}

// LoadConfigFromContent loads configuration from a given byte slice.
// It returns error if yaml decode failed.
func (c *Config) LoadConfigFromContent(content []byte) error {
	c.LoadDefaultConfig()

	_, err := yaml.Decode(content, c)
	if err != nil {
		logger.Errorf("Config parse error: " + err.Error())
		return err
	}

	err = c.Check()
	if err != nil {
		return err
	}

	logger.SetLevel(c.LogLevel)

	return nil
}
