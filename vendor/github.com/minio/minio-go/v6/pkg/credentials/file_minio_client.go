/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2017 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package credentials

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	jsoniter "github.com/json-iterator/go"
	homedir "github.com/mitchellh/go-homedir"
)

// A FileMinioClient retrieves credentials from the current user's home
// directory, and keeps track if those credentials are expired.
//
// Configuration file example: $HOME/.mc/config.json
type FileMinioClient struct {
	// Path to the shared credentials file.
	//
	// If empty will look for "MINIO_SHARED_CREDENTIALS_FILE" env variable. If the
	// env value is empty will default to current user's home directory.
	// Linux/OSX: "$HOME/.mc/config.json"
	// Windows:   "%USERALIAS%\mc\config.json"
	Filename string

	// MinIO Alias to extract credentials from the shared credentials file. If empty
	// will default to environment variable "MINIO_ALIAS" or "default" if
	// environment variable is also not set.
	Alias string

	// retrieved states if the credentials have been successfully retrieved.
	retrieved bool
}

// NewFileMinioClient returns a pointer to a new Credentials object
// wrapping the Alias file provider.
func NewFileMinioClient(filename string, alias string) *Credentials {
	return New(&FileMinioClient{
		Filename: filename,
		Alias:    alias,
	})
}

// Retrieve reads and extracts the shared credentials from the current
// users home directory.
func (p *FileMinioClient) Retrieve() (Value, error) {
	if p.Filename == "" {
		if value, ok := os.LookupEnv("MINIO_SHARED_CREDENTIALS_FILE"); ok {
			p.Filename = value
		} else {
			homeDir, err := homedir.Dir()
			if err != nil {
				return Value{}, err
			}
			p.Filename = filepath.Join(homeDir, ".mc", "config.json")
			if runtime.GOOS == "windows" {
				p.Filename = filepath.Join(homeDir, "mc", "config.json")
			}
		}
	}

	if p.Alias == "" {
		p.Alias = os.Getenv("MINIO_ALIAS")
		if p.Alias == "" {
			p.Alias = "s3"
		}
	}

	p.retrieved = false

	hostCfg, err := loadAlias(p.Filename, p.Alias)
	if err != nil {
		return Value{}, err
	}

	p.retrieved = true
	return Value{
		AccessKeyID:     hostCfg.AccessKey,
		SecretAccessKey: hostCfg.SecretKey,
		SignerType:      parseSignatureType(hostCfg.API),
	}, nil
}

// IsExpired returns if the shared credentials have expired.
func (p *FileMinioClient) IsExpired() bool {
	return !p.retrieved
}

// hostConfig configuration of a host.
type hostConfig struct {
	URL       string `json:"url"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	API       string `json:"api"`
}

// config config version.
type config struct {
	Version string                `json:"version"`
	Hosts   map[string]hostConfig `json:"hosts"`
}

// loadAliass loads from the file pointed to by shared credentials filename for alias.
// The credentials retrieved from the alias will be returned or error. Error will be
// returned if it fails to read from the file.
func loadAlias(filename, alias string) (hostConfig, error) {
	cfg := &config{}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	configBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return hostConfig{}, err
	}
	if err = json.Unmarshal(configBytes, cfg); err != nil {
		return hostConfig{}, err
	}
	return cfg.Hosts[alias], nil
}
