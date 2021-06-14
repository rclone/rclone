// Getters and Setters for ConfigMap

package fs

import (
	"os"
	"strings"

	"github.com/rclone/rclone/fs/config/configmap"
)

// A configmap.Getter to read from the environment RCLONE_CONFIG_backend_option_name
type configEnvVars string

// Get a config item from the environment variables if possible
func (configName configEnvVars) Get(key string) (value string, ok bool) {
	return os.LookupEnv(ConfigToEnv(string(configName), key))
}

// A configmap.Getter to read from the environment RCLONE_option_name
type optionEnvVars struct {
	fsInfo *RegInfo
}

// Get a config item from the option environment variables if possible
func (oev optionEnvVars) Get(key string) (value string, ok bool) {
	opt := oev.fsInfo.Options.Get(key)
	if opt == nil {
		return "", false
	}
	// For options with NoPrefix set, check without prefix too
	if opt.NoPrefix {
		value, ok = os.LookupEnv(OptionToEnv(key))
		if ok {
			return value, ok
		}
	}
	return os.LookupEnv(OptionToEnv(oev.fsInfo.Prefix + "-" + key))
}

// A configmap.Getter to read either the default value or the set
// value from the RegInfo.Options
type regInfoValues struct {
	fsInfo     *RegInfo
	useDefault bool
}

// override the values in configMap with the either the flag values or
// the default values
func (r *regInfoValues) Get(key string) (value string, ok bool) {
	opt := r.fsInfo.Options.Get(key)
	if opt != nil && (r.useDefault || opt.Value != nil) {
		return opt.String(), true
	}
	return "", false
}

// A configmap.Setter to read from the config file
type setConfigFile string

// Set a config item into the config file
func (section setConfigFile) Set(key, value string) {
	if strings.HasPrefix(string(section), ":") {
		Logf(nil, "Can't save config %q = %q for on the fly backend %q", key, value, section)
		return
	}
	Debugf(nil, "Saving config %q = %q in section %q of the config file", key, value, section)
	err := ConfigFileSet(string(section), key, value)
	if err != nil {
		Errorf(nil, "Failed saving config %q = %q in section %q of the config file: %v", key, value, section, err)
	}
}

// A configmap.Getter to read from the config file
type getConfigFile string

// Get a config item from the config file
func (section getConfigFile) Get(key string) (value string, ok bool) {
	value, ok = ConfigFileGet(string(section), key)
	// Ignore empty lines in the config file
	if value == "" {
		ok = false
	}
	return value, ok
}

// ConfigMap creates a configmap.Map from the *RegInfo and the
// configName passed in. If connectionStringConfig has any entries (it may be nil),
// then it will be added to the lookup with the highest priority.
//
// If fsInfo is nil then the returned configmap.Map should only be
// used for reading non backend specific parameters, such as "type".
func ConfigMap(fsInfo *RegInfo, configName string, connectionStringConfig configmap.Simple) (config *configmap.Map) {
	// Create the config
	config = configmap.New()

	// Read the config, more specific to least specific

	// Config from connection string
	if len(connectionStringConfig) > 0 {
		config.AddGetter(connectionStringConfig, configmap.PriorityNormal)
	}

	// flag values
	if fsInfo != nil {
		config.AddGetter(&regInfoValues{fsInfo, false}, configmap.PriorityNormal)
	}

	// remote specific environment vars
	config.AddGetter(configEnvVars(configName), configmap.PriorityNormal)

	// backend specific environment vars
	if fsInfo != nil {
		config.AddGetter(optionEnvVars{fsInfo: fsInfo}, configmap.PriorityNormal)
	}

	// config file
	config.AddGetter(getConfigFile(configName), configmap.PriorityConfig)

	// default values
	if fsInfo != nil {
		config.AddGetter(&regInfoValues{fsInfo, true}, configmap.PriorityDefault)
	}

	// Set Config
	config.AddSetter(setConfigFile(configName))
	return config
}
