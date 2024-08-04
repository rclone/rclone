// Getters and Setters for ConfigMap

package fs

import (
	"os"

	"github.com/rclone/rclone/fs/config/configmap"
)

// A configmap.Getter to read from the environment RCLONE_CONFIG_backend_option_name
type configEnvVars string

// Get a config item from the environment variables if possible
func (configName configEnvVars) Get(key string) (value string, ok bool) {
	envKey := ConfigToEnv(string(configName), key)
	value, ok = os.LookupEnv(envKey)
	if ok {
		Debugf(nil, "Setting %s=%q for %q from environment variable %s", key, value, configName, envKey)
	}
	return value, ok
}

// A configmap.Getter to read from the environment RCLONE_option_name
type optionEnvVars struct {
	prefix  string
	options Options
}

// Get a config item from the option environment variables if possible
func (oev optionEnvVars) Get(key string) (value string, ok bool) {
	opt := oev.options.Get(key)
	if opt == nil {
		return "", false
	}
	var envKey string
	if oev.prefix == "" {
		envKey = OptionToEnv(key)
	} else {
		envKey = OptionToEnv(oev.prefix + "-" + key)
	}
	value, ok = os.LookupEnv(envKey)
	if ok {
		Debugf(nil, "Setting %s %s=%q from environment variable %s", oev.prefix, key, value, envKey)
	} else if opt.NoPrefix {
		// For options with NoPrefix set, check without prefix too
		envKey := OptionToEnv(key)
		value, ok = os.LookupEnv(envKey)
		if ok {
			Debugf(nil, "Setting %s=%q for %s from environment variable %s", key, value, oev.prefix, envKey)
		}
	}
	return value, ok
}

// A configmap.Getter to read either the default value or the set
// value from the RegInfo.Options
type regInfoValues struct {
	options    Options
	useDefault bool
}

// override the values in configMap with the either the flag values or
// the default values
func (r *regInfoValues) Get(key string) (value string, ok bool) {
	opt := r.options.Get(key)
	if opt != nil && (r.useDefault || !opt.IsDefault()) {
		return opt.String(), true
	}
	return "", false
}

// A configmap.Setter to read from the config file
type setConfigFile string

// Set a config item into the config file
func (section setConfigFile) Set(key, value string) {
	Debugf(nil, "Saving config %q in section %q of the config file", key, section)
	err := ConfigFileSet(string(section), key, value)
	if err != nil {
		Errorf(nil, "Failed saving config %q in section %q of the config file: %v", key, section, err)
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

// ConfigMap creates a configmap.Map from the Options, prefix and the
// configName passed in. If connectionStringConfig has any entries (it may be nil),
// then it will be added to the lookup with the highest priority.
//
// If options is nil then the returned configmap.Map should only be
// used for reading non backend specific parameters, such as "type".
//
// This can be used for global settings if prefix is "" and configName is ""
func ConfigMap(prefix string, options Options, configName string, connectionStringConfig configmap.Simple) (config *configmap.Map) {
	// Create the config
	config = configmap.New()

	// Read the config, more specific to least specific

	// Config from connection string
	if len(connectionStringConfig) > 0 {
		config.AddGetter(connectionStringConfig, configmap.PriorityNormal)
	}

	// flag values
	if options != nil {
		config.AddGetter(&regInfoValues{options, false}, configmap.PriorityNormal)
	}

	// remote specific environment vars
	if configName != "" {
		config.AddGetter(configEnvVars(configName), configmap.PriorityNormal)
	}

	// backend specific environment vars
	if options != nil {
		config.AddGetter(optionEnvVars{prefix: prefix, options: options}, configmap.PriorityNormal)
	}

	// config file
	if configName != "" {
		config.AddGetter(getConfigFile(configName), configmap.PriorityConfig)
	}

	// default values
	if options != nil {
		config.AddGetter(&regInfoValues{options, true}, configmap.PriorityDefault)
	}

	// Set Config
	config.AddSetter(setConfigFile(configName))
	return config
}
