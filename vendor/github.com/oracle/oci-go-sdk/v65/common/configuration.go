// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package common

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// AuthenticationType for auth
type AuthenticationType string

const (
	// UserPrincipal is default auth type
	UserPrincipal AuthenticationType = "user_principal"
	// InstancePrincipal is used for instance principle auth type
	InstancePrincipal AuthenticationType = "instance_principal"
	// InstancePrincipalDelegationToken is used for instance principle delegation token auth type
	InstancePrincipalDelegationToken AuthenticationType = "instance_principle_delegation_token"
	// UnknownAuthenticationType is used for none meaningful auth type
	UnknownAuthenticationType AuthenticationType = "unknown_auth_type"
)

// AuthConfig is used for getting auth related paras in config file
type AuthConfig struct {
	AuthType AuthenticationType
	// IsFromConfigFile is used to point out if the authConfig is from configuration file
	IsFromConfigFile bool
	OboToken         *string
}

// ConfigurationProvider wraps information about the account owner
type ConfigurationProvider interface {
	KeyProvider
	TenancyOCID() (string, error)
	UserOCID() (string, error)
	KeyFingerprint() (string, error)
	Region() (string, error)
	// AuthType() is used for specify the needed auth type, like UserPrincipal, InstancePrincipal, etc.
	AuthType() (AuthConfig, error)
}

var fileMutex = sync.Mutex{}
var fileCache = make(map[string][]byte)

func readFile(filename string) ([]byte, error) {
	fileMutex.Lock()
	defer fileMutex.Unlock()
	val, ok := fileCache[filename]
	if ok {
		return val, nil
	}
	val, err := ioutil.ReadFile(filename)
	if err == nil {
		fileCache[filename] = val
	}
	return val, err
}

// IsConfigurationProviderValid Tests all parts of the configuration provider do not return an error, this method will
// not check AuthType(), since authType() is not required to be there.
func IsConfigurationProviderValid(conf ConfigurationProvider) (ok bool, err error) {
	baseFn := []func() (string, error){conf.TenancyOCID, conf.UserOCID, conf.KeyFingerprint, conf.Region, conf.KeyID}
	for _, fn := range baseFn {
		_, err = fn()
		ok = err == nil
		if err != nil {
			return
		}
	}

	_, err = conf.PrivateRSAKey()
	ok = err == nil
	if err != nil {
		return
	}
	return true, nil
}

// rawConfigurationProvider allows a user to simply construct a configuration provider from raw values.
type rawConfigurationProvider struct {
	tenancy              string
	user                 string
	region               string
	fingerprint          string
	privateKey           string
	privateKeyPassphrase *string
}

// NewRawConfigurationProvider will create a ConfigurationProvider with the arguments of the function
func NewRawConfigurationProvider(tenancy, user, region, fingerprint, privateKey string, privateKeyPassphrase *string) ConfigurationProvider {
	return rawConfigurationProvider{tenancy, user, region, fingerprint, privateKey, privateKeyPassphrase}
}

func (p rawConfigurationProvider) PrivateRSAKey() (key *rsa.PrivateKey, err error) {
	return PrivateKeyFromBytes([]byte(p.privateKey), p.privateKeyPassphrase)
}

func (p rawConfigurationProvider) KeyID() (keyID string, err error) {
	tenancy, err := p.TenancyOCID()
	if err != nil {
		return
	}

	user, err := p.UserOCID()
	if err != nil {
		return
	}

	fingerprint, err := p.KeyFingerprint()
	if err != nil {
		return
	}

	return fmt.Sprintf("%s/%s/%s", tenancy, user, fingerprint), nil
}

func (p rawConfigurationProvider) TenancyOCID() (string, error) {
	if p.tenancy == "" {
		return "", fmt.Errorf("tenancy OCID can not be empty")
	}
	return p.tenancy, nil
}

func (p rawConfigurationProvider) UserOCID() (string, error) {
	if p.user == "" {
		return "", fmt.Errorf("user OCID can not be empty")
	}
	return p.user, nil
}

func (p rawConfigurationProvider) KeyFingerprint() (string, error) {
	if p.fingerprint == "" {
		return "", fmt.Errorf("fingerprint can not be empty")
	}
	return p.fingerprint, nil
}

func (p rawConfigurationProvider) Region() (string, error) {
	return canStringBeRegion(p.region)
}

func (p rawConfigurationProvider) AuthType() (AuthConfig, error) {
	return AuthConfig{UnknownAuthenticationType, false, nil}, nil
}

// environmentConfigurationProvider reads configuration from environment variables
type environmentConfigurationProvider struct {
	PrivateKeyPassword        string
	EnvironmentVariablePrefix string
}

// ConfigurationProviderEnvironmentVariables creates a ConfigurationProvider from a uniform set of environment variables starting with a prefix
// The env variables should look like: [prefix]_private_key_path, [prefix]_tenancy_ocid, [prefix]_user_ocid, [prefix]_fingerprint
// [prefix]_region
func ConfigurationProviderEnvironmentVariables(environmentVariablePrefix, privateKeyPassword string) ConfigurationProvider {
	return environmentConfigurationProvider{EnvironmentVariablePrefix: environmentVariablePrefix,
		PrivateKeyPassword: privateKeyPassword}
}

func (p environmentConfigurationProvider) String() string {
	return fmt.Sprintf("Configuration provided by environment variables prefixed with: %s", p.EnvironmentVariablePrefix)
}

func (p environmentConfigurationProvider) PrivateRSAKey() (key *rsa.PrivateKey, err error) {
	environmentVariable := fmt.Sprintf("%s_%s", p.EnvironmentVariablePrefix, "private_key_path")
	var ok bool
	var value string
	if value, ok = os.LookupEnv(environmentVariable); !ok {
		return nil, fmt.Errorf("can not read PrivateKey from env variable: %s", environmentVariable)
	}

	expandedPath := expandPath(value)
	pemFileContent, err := readFile(expandedPath)
	if err != nil {
		Debugln("Can not read PrivateKey location from environment variable: " + environmentVariable)
		return
	}

	key, err = PrivateKeyFromBytes(pemFileContent, &p.PrivateKeyPassword)
	return
}

func (p environmentConfigurationProvider) KeyID() (keyID string, err error) {
	ocid, err := p.TenancyOCID()
	if err != nil {
		return
	}

	userocid, err := p.UserOCID()
	if err != nil {
		return
	}

	fingerprint, err := p.KeyFingerprint()
	if err != nil {
		return
	}

	return fmt.Sprintf("%s/%s/%s", ocid, userocid, fingerprint), nil
}

func (p environmentConfigurationProvider) TenancyOCID() (value string, err error) {
	environmentVariable := fmt.Sprintf("%s_%s", p.EnvironmentVariablePrefix, "tenancy_ocid")
	var ok bool
	if value, ok = os.LookupEnv(environmentVariable); !ok {
		err = fmt.Errorf("can not read Tenancy from environment variable %s", environmentVariable)
	} else if value == "" {
		err = fmt.Errorf("tenancy OCID can not be empty when reading from environmental variable")
	}
	return
}

func (p environmentConfigurationProvider) UserOCID() (value string, err error) {
	environmentVariable := fmt.Sprintf("%s_%s", p.EnvironmentVariablePrefix, "user_ocid")
	var ok bool
	if value, ok = os.LookupEnv(environmentVariable); !ok {
		err = fmt.Errorf("can not read user id from environment variable %s", environmentVariable)
	} else if value == "" {
		err = fmt.Errorf("user OCID can not be empty when reading from environmental variable")
	}
	return
}

func (p environmentConfigurationProvider) KeyFingerprint() (value string, err error) {
	environmentVariable := fmt.Sprintf("%s_%s", p.EnvironmentVariablePrefix, "fingerprint")
	var ok bool
	if value, ok = os.LookupEnv(environmentVariable); !ok {
		err = fmt.Errorf("can not read fingerprint from environment variable %s", environmentVariable)
	} else if value == "" {
		err = fmt.Errorf("fingerprint can not be empty when reading from environmental variable")
	}
	return
}

func (p environmentConfigurationProvider) Region() (value string, err error) {
	environmentVariable := fmt.Sprintf("%s_%s", p.EnvironmentVariablePrefix, "region")
	var ok bool
	if value, ok = os.LookupEnv(environmentVariable); !ok {
		err = fmt.Errorf("can not read region from environment variable %s", environmentVariable)
		return value, err
	}

	return canStringBeRegion(value)
}

func (p environmentConfigurationProvider) AuthType() (AuthConfig, error) {
	return AuthConfig{UnknownAuthenticationType, false, nil},
		fmt.Errorf("unsupported, keep the interface")
}

// fileConfigurationProvider. reads configuration information from a file
type fileConfigurationProvider struct {
	//The path to the configuration file
	ConfigPath string

	//The password for the private key
	PrivateKeyPassword string

	//The profile for the configuration
	Profile string

	//ConfigFileInfo
	FileInfo *configFileInfo

	//Mutex to protect the config file
	configMux sync.Mutex
}

type fileConfigurationProviderError struct {
	err error
}

func (fpe fileConfigurationProviderError) Error() string {
	return fmt.Sprintf("%s\nFor more info about config file and how to get required information, see https://docs.oracle.com/en-us/iaas/Content/API/Concepts/sdkconfig.htm", fpe.err)
}

// ConfigurationProviderFromFile creates a configuration provider from a configuration file
// by reading the "DEFAULT" profile
func ConfigurationProviderFromFile(configFilePath, privateKeyPassword string) (ConfigurationProvider, error) {
	if configFilePath == "" {
		return nil, fmt.Errorf("config file path can not be empty")
	}

	return fileConfigurationProvider{
		ConfigPath:         configFilePath,
		PrivateKeyPassword: privateKeyPassword,
		Profile:            "DEFAULT",
		configMux:          sync.Mutex{}}, nil
}

// ConfigurationProviderFromFileWithProfile creates a configuration provider from a configuration file
// and the given profile
func ConfigurationProviderFromFileWithProfile(configFilePath, profile, privateKeyPassword string) (ConfigurationProvider, error) {
	if configFilePath == "" {
		return nil, fileConfigurationProviderError{err: fmt.Errorf("config file path can not be empty")}
	}

	return fileConfigurationProvider{
		ConfigPath:         configFilePath,
		PrivateKeyPassword: privateKeyPassword,
		Profile:            profile,
		configMux:          sync.Mutex{}}, nil
}

type configFileInfo struct {
	UserOcid, Fingerprint, KeyFilePath, TenancyOcid, Region, Passphrase, SecurityTokenFilePath, DelegationTokenFilePath,
	AuthenticationType string
	PresentConfiguration rune
}

const (
	hasTenancy = 1 << iota
	hasUser
	hasFingerprint
	hasRegion
	hasKeyFile
	hasPassphrase
	hasSecurityTokenFile
	hasDelegationTokenFile
	hasAuthenticationType
	none
)

var profileRegex = regexp.MustCompile(`^\[(.*)\]`)

func parseConfigFile(data []byte, profile string) (info *configFileInfo, err error) {

	if len(data) == 0 {
		return nil, fileConfigurationProviderError{err: fmt.Errorf("configuration file content is empty")}
	}

	content := string(data)
	splitContent := strings.Split(content, "\n")

	//Look for profile
	for i, line := range splitContent {
		if match := profileRegex.FindStringSubmatch(line); match != nil && len(match) > 1 && match[1] == profile {
			start := i + 1
			return parseConfigAtLine(start, splitContent)
		}
	}

	return nil, fileConfigurationProviderError{err: fmt.Errorf("configuration file did not contain profile: %s", profile)}
}

func parseConfigAtLine(start int, content []string) (info *configFileInfo, err error) {
	var configurationPresent rune
	info = &configFileInfo{}
	for i := start; i < len(content); i++ {
		line := content[i]
		if profileRegex.MatchString(line) {
			break
		}

		if !strings.Contains(line, "=") {
			continue
		}

		splits := strings.Split(line, "=")
		switch key, value := strings.TrimSpace(splits[0]), strings.TrimSpace(splits[1]); strings.ToLower(key) {
		case "passphrase", "pass_phrase":
			configurationPresent = configurationPresent | hasPassphrase
			info.Passphrase = value
		case "user":
			configurationPresent = configurationPresent | hasUser
			info.UserOcid = value
		case "fingerprint":
			configurationPresent = configurationPresent | hasFingerprint
			info.Fingerprint = value
		case "key_file":
			configurationPresent = configurationPresent | hasKeyFile
			info.KeyFilePath = value
		case "tenancy":
			configurationPresent = configurationPresent | hasTenancy
			info.TenancyOcid = value
		case "region":
			configurationPresent = configurationPresent | hasRegion
			info.Region = value
		case "security_token_file":
			configurationPresent = configurationPresent | hasSecurityTokenFile
			info.SecurityTokenFilePath = value
		case "delegation_token_file":
			configurationPresent = configurationPresent | hasDelegationTokenFile
			info.DelegationTokenFilePath = value
		case "authentication_type":
			configurationPresent = configurationPresent | hasAuthenticationType
			info.AuthenticationType = value
		}
	}
	info.PresentConfiguration = configurationPresent
	return

}

// cleans and expands the path if it contains a tilde , returns the expanded path or the input path as is if not expansion
// was performed
func expandPath(filename string) (expandedPath string) {
	cleanedPath := filepath.Clean(filename)
	expandedPath = cleanedPath
	if strings.HasPrefix(cleanedPath, "~") {
		rest := cleanedPath[2:]
		expandedPath = filepath.Join(getHomeFolder(), rest)
	}
	return
}

func openConfigFile(configFilePath string) (data []byte, err error) {
	expandedPath := expandPath(configFilePath)
	data, err = readFile(expandedPath)
	if err != nil {
		err = fmt.Errorf("can not read config file: %s due to: %s", configFilePath, err.Error())
	}

	return
}

func (p fileConfigurationProvider) String() string {
	return fmt.Sprintf("Configuration provided by file: %s", p.ConfigPath)
}

func (p fileConfigurationProvider) readAndParseConfigFile() (info *configFileInfo, err error) {
	p.configMux.Lock()
	defer p.configMux.Unlock()
	if p.FileInfo != nil {
		return p.FileInfo, nil
	}

	if p.ConfigPath == "" {
		return nil, fileConfigurationProviderError{err: fmt.Errorf("configuration path can not be empty")}
	}

	data, err := openConfigFile(p.ConfigPath)
	if err != nil {
		err = fileConfigurationProviderError{err: fmt.Errorf("error while parsing config file: %s. Due to: %s", p.ConfigPath, err.Error())}
		return
	}

	p.FileInfo, err = parseConfigFile(data, p.Profile)
	return p.FileInfo, err
}

func presentOrError(value string, expectedConf, presentConf rune, confMissing string) (string, error) {
	if presentConf&expectedConf == expectedConf {
		return value, nil
	}
	return "", fileConfigurationProviderError{err: errors.New(confMissing + " configuration is missing from file")}
}

func (p fileConfigurationProvider) TenancyOCID() (value string, err error) {
	info, err := p.readAndParseConfigFile()
	if err != nil {
		err = fileConfigurationProviderError{err: fmt.Errorf("can not read tenancy configuration due to: %s", err.Error())}
		return
	}

	value, err = presentOrError(info.TenancyOcid, hasTenancy, info.PresentConfiguration, "tenancy")
	if err == nil && value == "" {
		err = fileConfigurationProviderError{err: fmt.Errorf("tenancy OCID can not be empty when reading from config file")}
	}
	return
}

func (p fileConfigurationProvider) UserOCID() (value string, err error) {
	info, err := p.readAndParseConfigFile()
	if err != nil {
		err = fileConfigurationProviderError{err: fmt.Errorf("can not read tenancy configuration due to: %s", err.Error())}
		return
	}

	if value, err = presentOrError(info.UserOcid, hasUser, info.PresentConfiguration, "user"); err != nil {
		// need to check if securityTokenPath is provided, if security token is provided, userOCID can be "".
		if _, stErr := presentOrError(info.SecurityTokenFilePath, hasSecurityTokenFile, info.PresentConfiguration,
			"securityTokenPath"); stErr == nil {
			err = nil
		}
	}
	return
}

func (p fileConfigurationProvider) KeyFingerprint() (value string, err error) {
	info, err := p.readAndParseConfigFile()
	if err != nil {
		err = fileConfigurationProviderError{err: fmt.Errorf("can not read tenancy configuration due to: %s", err.Error())}
		return
	}
	value, err = presentOrError(info.Fingerprint, hasFingerprint, info.PresentConfiguration, "fingerprint")
	if err == nil && value == "" {
		return "", fmt.Errorf("fingerprint can not be empty when reading from config file")
	}
	return
}

func (p fileConfigurationProvider) KeyID() (keyID string, err error) {
	tenancy, err := p.TenancyOCID()
	if err != nil {
		return
	}

	fingerprint, err := p.KeyFingerprint()
	if err != nil {
		return
	}

	info, err := p.readAndParseConfigFile()
	if err != nil {
		err = fileConfigurationProviderError{err: fmt.Errorf("can not read tenancy configuration due to: %s", err.Error())}
		return
	}
	if info.PresentConfiguration&hasUser == hasUser {
		if info.UserOcid == "" {
			err = fileConfigurationProviderError{err: fmt.Errorf("user cannot be empty in the config file")}
			return
		}
		return fmt.Sprintf("%s/%s/%s", tenancy, info.UserOcid, fingerprint), nil
	}
	filePath, pathErr := presentOrError(info.SecurityTokenFilePath, hasSecurityTokenFile, info.PresentConfiguration, "securityTokenFilePath")
	if pathErr == nil {
		rawString, err := getTokenContent(filePath)
		if err != nil {
			return "", fileConfigurationProviderError{err: err}
		}
		return "ST$" + rawString, nil
	}
	err = fileConfigurationProviderError{err: fmt.Errorf("can not read SecurityTokenFilePath from configuration file due to: %s", pathErr.Error())}
	return
}

func (p fileConfigurationProvider) PrivateRSAKey() (key *rsa.PrivateKey, err error) {
	info, err := p.readAndParseConfigFile()
	if err != nil {
		err = fileConfigurationProviderError{err: fmt.Errorf("can not read tenancy configuration due to: %s", err.Error())}
		return
	}

	filePath, err := presentOrError(info.KeyFilePath, hasKeyFile, info.PresentConfiguration, "key file path")
	if err != nil {
		return
	}

	expandedPath := expandPath(filePath)
	pemFileContent, err := readFile(expandedPath)
	if err != nil {
		err = fileConfigurationProviderError{err: fmt.Errorf("can not read PrivateKey  from configuration file due to: %s", err.Error())}
		return
	}

	password := p.PrivateKeyPassword

	if password == "" && ((info.PresentConfiguration & hasPassphrase) == hasPassphrase) {
		password = info.Passphrase
	}

	key, err = PrivateKeyFromBytes(pemFileContent, &password)
	return
}

func (p fileConfigurationProvider) Region() (value string, err error) {
	info, err := p.readAndParseConfigFile()
	if err != nil {
		err = fileConfigurationProviderError{err: fmt.Errorf("can not read region configuration due to: %s", err.Error())}
		return
	}

	value, err = presentOrError(info.Region, hasRegion, info.PresentConfiguration, "region")
	if err != nil {
		val, error := getRegionFromEnvVar()
		if error != nil {
			err = fileConfigurationProviderError{err: fmt.Errorf("region configuration is missing from file, nor for OCI_REGION env var")}
			return
		}
		value = val
	}

	return canStringBeRegion(value)
}

func (p fileConfigurationProvider) AuthType() (AuthConfig, error) {
	info, err := p.readAndParseConfigFile()
	if err != nil {
		err = fmt.Errorf("can not read tenancy configuration due to: %s", err.Error())
		return AuthConfig{UnknownAuthenticationType, true, nil}, err
	}
	val, err := presentOrError(info.AuthenticationType, hasAuthenticationType, info.PresentConfiguration, "authentication_type")

	if val == "instance_principal" {
		if filePath, err := presentOrError(info.DelegationTokenFilePath, hasDelegationTokenFile, info.PresentConfiguration, "delegationTokenFilePath"); err == nil {
			if delegationToken, err := getTokenContent(filePath); err == nil && delegationToken != "" {
				Debugf("delegation token content is %s, and error is %s ", delegationToken, err)
				return AuthConfig{InstancePrincipalDelegationToken, true, &delegationToken}, nil
			}
			return AuthConfig{UnknownAuthenticationType, true, nil}, err

		}
		// normal instance principle
		return AuthConfig{InstancePrincipal, true, nil}, nil
	}

	// by default, if no "authentication_type" is provided, just treated as user principle type, and will not return error
	return AuthConfig{UserPrincipal, true, nil}, nil
}

func getTokenContent(filePath string) (string, error) {
	expandedPath := expandPath(filePath)
	tokenFileContent, err := readFile(expandedPath)
	if err != nil {
		err = fileConfigurationProviderError{err: fmt.Errorf("can not read token content from configuration file due to: %s", err.Error())}
		return "", err
	}
	return fmt.Sprintf("%s", tokenFileContent), nil
}

// A configuration provider that look for information in  multiple configuration providers
type composingConfigurationProvider struct {
	Providers []ConfigurationProvider
}

// ComposingConfigurationProvider creates a composing configuration provider with the given slice of configuration providers
// A composing provider will return the configuration of the first provider that has the required property
// if no provider has the property it will return an error.
func ComposingConfigurationProvider(providers []ConfigurationProvider) (ConfigurationProvider, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("providers can not be an empty slice")
	}

	for i, p := range providers {
		if p == nil {
			return nil, fmt.Errorf("provider in position: %d is nil. ComposingConfiurationProvider does not support nil values", i)
		}
	}
	return composingConfigurationProvider{Providers: providers}, nil
}

func (c composingConfigurationProvider) TenancyOCID() (string, error) {
	for _, p := range c.Providers {
		val, err := p.TenancyOCID()
		if err == nil {
			return val, nil
		}
		Debugf("did not find a proper configuration for tenancy, err: %v", err)
	}
	return "", fmt.Errorf("did not find a proper configuration for tenancy")
}

func (c composingConfigurationProvider) UserOCID() (string, error) {
	for _, p := range c.Providers {
		val, err := p.UserOCID()
		if err == nil {
			return val, nil
		}
		Debugf("did not find a proper configuration for keyFingerprint, err: %v", err)
	}
	return "", fmt.Errorf("did not find a proper configuration for user")
}

func (c composingConfigurationProvider) KeyFingerprint() (string, error) {
	for _, p := range c.Providers {
		val, err := p.KeyFingerprint()
		if err == nil {
			return val, nil
		}
	}
	return "", fmt.Errorf("did not find a proper configuration for keyFingerprint")
}
func (c composingConfigurationProvider) Region() (string, error) {
	for _, p := range c.Providers {
		val, err := p.Region()
		if err == nil {
			return val, nil
		}
	}
	if val, err := getRegionFromEnvVar(); err == nil {
		return val, nil
	}
	return "", fmt.Errorf("did not find a proper configuration for region, nor for OCI_REGION env var")
}

func (c composingConfigurationProvider) KeyID() (string, error) {
	for _, p := range c.Providers {
		val, err := p.KeyID()
		if err == nil {
			return val, nil
		}
	}
	return "", fmt.Errorf("did not find a proper configuration for key id")
}

func (c composingConfigurationProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	for _, p := range c.Providers {
		val, err := p.PrivateRSAKey()
		if err == nil {
			return val, nil
		}
	}
	return nil, fmt.Errorf("did not find a proper configuration for private key")
}

func (c composingConfigurationProvider) AuthType() (AuthConfig, error) {
	// only check the first default fileConfigProvider
	authConfig, err := c.Providers[0].AuthType()
	if err == nil && authConfig.AuthType != UnknownAuthenticationType {
		return authConfig, nil
	}
	return AuthConfig{UnknownAuthenticationType, false, nil}, fmt.Errorf("did not find a proper configuration for auth type")
}

func getRegionFromEnvVar() (string, error) {
	regionEnvVar := "OCI_REGION"
	if region, existed := os.LookupEnv(regionEnvVar); existed {
		return region, nil
	}
	return "", fmt.Errorf("did not find OCI_REGION env var")
}

// RefreshableConfigurationProvider the interface to identity if the config provider is refreshable
type RefreshableConfigurationProvider interface {
	Refreshable() bool
}
