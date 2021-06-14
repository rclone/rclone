// Filesystem registry and backend options

package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
)

// Registry of filesystems
var Registry []*RegInfo

// RegInfo provides information about a filesystem
type RegInfo struct {
	// Name of this fs
	Name string
	// Description of this fs - defaults to Name
	Description string
	// Prefix for command line flags for this fs - defaults to Name if not set
	Prefix string
	// Create a new file system.  If root refers to an existing
	// object, then it should return an Fs which which points to
	// the parent of that object and ErrorIsFile.
	NewFs func(ctx context.Context, name string, root string, config configmap.Mapper) (Fs, error) `json:"-"`
	// Function to call to help with config - see docs for ConfigIn for more info
	Config func(ctx context.Context, name string, m configmap.Mapper, configIn ConfigIn) (*ConfigOut, error) `json:"-"`
	// Options for the Fs configuration
	Options Options
	// The command help, if any
	CommandHelp []CommandHelp
}

// FileName returns the on disk file name for this backend
func (ri *RegInfo) FileName() string {
	return strings.Replace(ri.Name, " ", "", -1)
}

// Options is a slice of configuration Option for a backend
type Options []Option

// Set the default values for the options
func (os Options) setValues() {
	for i := range os {
		o := &os[i]
		if o.Default == nil {
			o.Default = ""
		}
	}
}

// Get the Option corresponding to name or return nil if not found
func (os Options) Get(name string) *Option {
	for i := range os {
		opt := &os[i]
		if opt.Name == name {
			return opt
		}
	}
	return nil
}

// Overridden discovers which config items have been overridden in the
// configmap passed in, either by the config string, command line
// flags or environment variables
func (os Options) Overridden(m *configmap.Map) configmap.Simple {
	var overridden = configmap.Simple{}
	for i := range os {
		opt := &os[i]
		value, isSet := m.GetPriority(opt.Name, configmap.PriorityNormal)
		if isSet {
			overridden.Set(opt.Name, value)
		}
	}
	return overridden
}

// NonDefault discovers which config values aren't at their default
func (os Options) NonDefault(m configmap.Getter) configmap.Simple {
	var nonDefault = configmap.Simple{}
	for i := range os {
		opt := &os[i]
		value, isSet := m.Get(opt.Name)
		if !isSet {
			continue
		}
		defaultValue := fmt.Sprint(opt.Default)
		if value != defaultValue {
			nonDefault.Set(opt.Name, value)
		}
	}
	return nonDefault
}

// HasAdvanced discovers if any options have an Advanced setting
func (os Options) HasAdvanced() bool {
	for i := range os {
		opt := &os[i]
		if opt.Advanced {
			return true
		}
	}
	return false
}

// OptionVisibility controls whether the options are visible in the
// configurator or the command line.
type OptionVisibility byte

// Constants Option.Hide
const (
	OptionHideCommandLine OptionVisibility = 1 << iota
	OptionHideConfigurator
	OptionHideBoth = OptionHideCommandLine | OptionHideConfigurator
)

// Option is describes an option for the config wizard
//
// This also describes command line options and environment variables
type Option struct {
	Name       string           // name of the option in snake_case
	Help       string           // Help, the first line only is used for the command line help
	Provider   string           // Set to filter on provider
	Default    interface{}      // default value, nil => ""
	Value      interface{}      // value to be set by flags
	Examples   OptionExamples   `json:",omitempty"` // config examples
	ShortOpt   string           // the short option for this if required
	Hide       OptionVisibility // set this to hide the config from the configurator or the command line
	Required   bool             // this option is required
	IsPassword bool             // set if the option is a password
	NoPrefix   bool             // set if the option for this should not use the backend prefix
	Advanced   bool             // set if this is an advanced config option
	Exclusive  bool             // set if the answer can only be one of the examples
}

// BaseOption is an alias for Option used internally
type BaseOption Option

// MarshalJSON turns an Option into JSON
//
// It adds some generated fields for ease of use
// - DefaultStr - a string rendering of Default
// - ValueStr - a string rendering of Value
// - Type - the type of the option
func (o *Option) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		BaseOption
		DefaultStr string
		ValueStr   string
		Type       string
	}{
		BaseOption: BaseOption(*o),
		DefaultStr: fmt.Sprint(o.Default),
		ValueStr:   o.String(),
		Type:       o.Type(),
	})
}

// GetValue gets the current current value which is the default if not set
func (o *Option) GetValue() interface{} {
	val := o.Value
	if val == nil {
		val = o.Default
		if val == nil {
			val = ""
		}
	}
	return val
}

// String turns Option into a string
func (o *Option) String() string {
	return fmt.Sprint(o.GetValue())
}

// Set an Option from a string
func (o *Option) Set(s string) (err error) {
	newValue, err := configstruct.StringToInterface(o.GetValue(), s)
	if err != nil {
		return err
	}
	o.Value = newValue
	return nil
}

// Type of the value
func (o *Option) Type() string {
	return reflect.TypeOf(o.GetValue()).Name()
}

// FlagName for the option
func (o *Option) FlagName(prefix string) string {
	name := strings.Replace(o.Name, "_", "-", -1) // convert snake_case to kebab-case
	if !o.NoPrefix {
		name = prefix + "-" + name
	}
	return name
}

// EnvVarName for the option
func (o *Option) EnvVarName(prefix string) string {
	return OptionToEnv(prefix + "-" + o.Name)
}

// Copy makes a shallow copy of the option
func (o *Option) Copy() *Option {
	copy := new(Option)
	*copy = *o
	return copy
}

// OptionExamples is a slice of examples
type OptionExamples []OptionExample

// Len is part of sort.Interface.
func (os OptionExamples) Len() int { return len(os) }

// Swap is part of sort.Interface.
func (os OptionExamples) Swap(i, j int) { os[i], os[j] = os[j], os[i] }

// Less is part of sort.Interface.
func (os OptionExamples) Less(i, j int) bool { return os[i].Help < os[j].Help }

// Sort sorts an OptionExamples
func (os OptionExamples) Sort() { sort.Sort(os) }

// OptionExample describes an example for an Option
type OptionExample struct {
	Value    string
	Help     string
	Provider string
}

// Register a filesystem
//
// Fs modules  should use this in an init() function
func Register(info *RegInfo) {
	info.Options.setValues()
	if info.Prefix == "" {
		info.Prefix = info.Name
	}
	Registry = append(Registry, info)
}

// Find looks for a RegInfo object for the name passed in.  The name
// can be either the Name or the Prefix.
//
// Services are looked up in the config file
func Find(name string) (*RegInfo, error) {
	for _, item := range Registry {
		if item.Name == name || item.Prefix == name || item.FileName() == name {
			return item, nil
		}
	}
	return nil, errors.Errorf("didn't find backend called %q", name)
}

// MustFind looks for an Info object for the type name passed in
//
// Services are looked up in the config file
//
// Exits with a fatal error if not found
func MustFind(name string) *RegInfo {
	fs, err := Find(name)
	if err != nil {
		log.Fatalf("Failed to find remote: %v", err)
	}
	return fs
}
