package config

// Default config.Storage which panics with a useful error when used
type defaultStorage struct{}

var noConfigStorage = "internal error: no config file system found. Did you call configfile.Install()?"

// GetSectionList returns a slice of strings with names for all the
// sections
func (defaultStorage) GetSectionList() []string {
	panic(noConfigStorage)
}

// HasSection returns true if section exists in the config file
func (defaultStorage) HasSection(section string) bool {
	panic(noConfigStorage)
}

// DeleteSection removes the named section and all config from the
// config file
func (defaultStorage) DeleteSection(section string) {
	panic(noConfigStorage)
}

// GetKeyList returns the keys in this section
func (defaultStorage) GetKeyList(section string) []string {
	panic(noConfigStorage)
}

// GetValue returns the key in section with a found flag
func (defaultStorage) GetValue(section string, key string) (value string, found bool) {
	panic(noConfigStorage)
}

// SetValue sets the value under key in section
func (defaultStorage) SetValue(section string, key string, value string) {
	panic(noConfigStorage)
}

// DeleteKey removes the key under section
func (defaultStorage) DeleteKey(section string, key string) bool {
	panic(noConfigStorage)
}

// Load the config from permanent storage
func (defaultStorage) Load() error {
	panic(noConfigStorage)
}

// Save the config to permanent storage
func (defaultStorage) Save() error {
	panic(noConfigStorage)
}

// Serialize the config into a string
func (defaultStorage) Serialize() (string, error) {
	panic(noConfigStorage)
}

// Check the interface is satisfied
var _ Storage = defaultStorage{}
