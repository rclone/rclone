// Filesystem registry and backend options

package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/lib/errcount"
)

// Registry of filesystems
var Registry []*RegInfo

// optDescription is a basic description option
var optDescription = Option{
	Name:     "description",
	Help:     "Description of the remote.",
	Default:  "",
	Advanced: true,
}

// RegInfo provides information about a filesystem
type RegInfo struct {
	// Name of this fs
	Name string
	// Description of this fs - defaults to Name
	Description string
	// Prefix for command line flags for this fs - defaults to Name if not set
	Prefix string
	// Create a new file system.  If root refers to an existing
	// object, then it should return an Fs which points to
	// the parent of that object and ErrorIsFile.
	NewFs func(ctx context.Context, name string, root string, config configmap.Mapper) (Fs, error) `json:"-"`
	// Function to call to help with config - see docs for ConfigIn for more info
	Config func(ctx context.Context, name string, m configmap.Mapper, configIn ConfigIn) (*ConfigOut, error) `json:"-"`
	// Options for the Fs configuration
	Options Options
	// The command help, if any
	CommandHelp []CommandHelp
	// Aliases - other names this backend is known by
	Aliases []string
	// Hide - if set don't show in the configurator
	Hide bool
	// MetadataInfo help about the metadata in use in this backend
	MetadataInfo *MetadataInfo
}

// FileName returns the on disk file name for this backend
func (ri *RegInfo) FileName() string {
	return strings.ReplaceAll(ri.Name, " ", "")
}

// Options is a slice of configuration Option for a backend
type Options []Option

// Add more options returning a new options slice
func (os Options) Add(newOptions Options) Options {
	return append(os, newOptions...)
}

// AddPrefix adds more options with a prefix returning a new options slice
func (os Options) AddPrefix(newOptions Options, prefix string, groups string) Options {
	for _, opt := range newOptions {
		// opt is a copy so can modify
		opt.Name = prefix + "_" + opt.Name
		opt.Groups = groups
		os = append(os, opt)
	}
	return os
}

// Set the default values for the options
func (os Options) setValues() {
	for i := range os {
		o := &os[i]
		if o.Default == nil {
			o.Default = ""
		}
		// Create options for Enums
		if do, ok := o.Default.(Choices); ok && len(o.Examples) == 0 {
			o.Exclusive = true
			o.Required = true
			o.Examples = make(OptionExamples, len(do.Choices()))
			for i, choice := range do.Choices() {
				o.Examples[i].Value = choice
			}
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

// SetDefault sets the default for the Option corresponding to name
//
// Writes an ERROR level log if the option is not found
func (os Options) SetDefault(name string, def any) Options {
	opt := os.Get(name)
	if opt == nil {
		Errorf(nil, "Couldn't find option %q to SetDefault on", name)
	} else {
		opt.Default = def
	}
	return os
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
// This also describes command line options and environment variables.
//
// It is also used to describe options for the API.
//
// To create a multiple-choice option, specify the possible values
// in the Examples property. Whether the option's value is required
// to be one of these depends on other properties:
//   - Default is to allow any value, either from specified examples,
//     or any other value. To restrict exclusively to the specified
//     examples, also set Exclusive=true.
//   - If empty string should not be allowed then set Required=true,
//     and do not set Default.
type Option struct {
	Name       string           // name of the option in snake_case
	FieldName  string           // name of the field used in the rc JSON - will be auto filled normally
	Help       string           // help, start with a single sentence on a single line that will be extracted for command line help
	Groups     string           `json:",omitempty"` // groups this option belongs to - comma separated string for options classification
	Provider   string           `json:",omitempty"` // set to filter on provider
	Default    interface{}      // default value, nil => "", if set (and not to nil or "") then Required does nothing
	Value      interface{}      // value to be set by flags
	Examples   OptionExamples   `json:",omitempty"` // predefined values that can be selected from list (multiple-choice option)
	ShortOpt   string           `json:",omitempty"` // the short option for this if required
	Hide       OptionVisibility // set this to hide the config from the configurator or the command line
	Required   bool             // this option is required, meaning value cannot be empty unless there is a default
	IsPassword bool             // set if the option is a password
	NoPrefix   bool             // set if the option for this should not use the backend prefix
	Advanced   bool             // set if this is an advanced config option
	Exclusive  bool             // set if the answer can only be one of the examples (empty string allowed unless Required or Default is set)
	Sensitive  bool             // set if this option should be redacted when using rclone config redacted
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

// GetValue gets the current value which is the default if not set
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

// IsDefault returns true if the value is the default value
func (o *Option) IsDefault() bool {
	if o.Value == nil {
		return true
	}
	Default := o.Default
	if Default == nil {
		Default = ""
	}
	return reflect.DeepEqual(o.Value, Default)
}

// String turns Option into a string
func (o *Option) String() string {
	v := o.GetValue()
	if stringArray, isStringArray := v.([]string); isStringArray {
		// Treat empty string array as empty string
		// This is to make the default value of the option help nice
		if len(stringArray) == 0 {
			return ""
		}
		// Encode string arrays as CSV
		// The default Go encoding can't be decoded uniquely
		return CommaSepList(stringArray).String()
	}
	return fmt.Sprint(v)
}

// Set an Option from a string
func (o *Option) Set(s string) (err error) {
	v := o.GetValue()
	if stringArray, isStringArray := v.([]string); isStringArray {
		if stringArray == nil {
			stringArray = []string{}
		}
		// If this is still the default value then overwrite the defaults
		if reflect.ValueOf(o.Default).Pointer() == reflect.ValueOf(v).Pointer() {
			stringArray = []string{}
		}
		o.Value = append(stringArray, s)
		return nil
	}
	newValue, err := configstruct.StringToInterface(v, s)
	if err != nil {
		return err
	}
	o.Value = newValue
	return nil
}

type typer interface {
	Type() string
}

// Type of the value
func (o *Option) Type() string {
	v := o.GetValue()

	// Try to call Type method on non-pointer
	if do, ok := v.(typer); ok {
		return do.Type()
	}

	// Special case []string
	if _, isStringArray := v.([]string); isStringArray {
		return "stringArray"
	}

	return reflect.TypeOf(v).Name()
}

// FlagName for the option
func (o *Option) FlagName(prefix string) string {
	name := strings.ReplaceAll(o.Name, "_", "-") // convert snake_case to kebab-case
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
	Provider string `json:",omitempty"`
}

// Register a filesystem
//
// Fs modules  should use this in an init() function
func Register(info *RegInfo) {
	info.Options.setValues()
	if info.Prefix == "" {
		info.Prefix = info.Name
	}
	info.Options = append(info.Options, optDescription)
	Registry = append(Registry, info)
	for _, alias := range info.Aliases {
		// Copy the info block and rename and hide the alias and options
		aliasInfo := *info
		aliasInfo.Name = alias
		aliasInfo.Prefix = alias
		aliasInfo.Hide = true
		aliasInfo.Options = append(Options(nil), info.Options...)
		for i := range aliasInfo.Options {
			aliasInfo.Options[i].Hide = OptionHideBoth
		}
		Registry = append(Registry, &aliasInfo)
	}
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
	return nil, fmt.Errorf("didn't find backend called %q", name)
}

// MustFind looks for an Info object for the type name passed in
//
// Services are looked up in the config file.
//
// Exits with a fatal error if not found
func MustFind(name string) *RegInfo {
	fs, err := Find(name)
	if err != nil {
		Fatalf(nil, "Failed to find remote: %v", err)
	}
	return fs
}

// OptionsInfo holds info about an block of options
type OptionsInfo struct {
	Name    string                      // name of this options block for the rc
	Opt     interface{}                 // pointer to a struct to set the options in
	Options Options                     // description of the options
	Reload  func(context.Context) error // if not nil, call when options changed and on init
}

// OptionsRegistry is a registry of global options
var OptionsRegistry = map[string]OptionsInfo{}

// RegisterGlobalOptions registers global options to be made into
// command line options, rc options and environment variable reading.
//
// Packages which need global options should use this in an init() function
func RegisterGlobalOptions(oi OptionsInfo) {
	oi.Options.setValues()
	OptionsRegistry[oi.Name] = oi
	if oi.Opt != nil && oi.Options != nil {
		err := oi.Check()
		if err != nil {
			Fatalf(nil, "%v", err)
		}
	}
	// Load the default values into the options.
	//
	// These will be from the ultimate defaults or environment
	// variables.
	//
	// The flags haven't been processed yet so this will be run
	// again when the flags are ready.
	err := oi.load()
	if err != nil {
		Fatalf(nil, "Failed to load %q default values: %v", oi.Name, err)
	}
}

var optionName = regexp.MustCompile(`^[a-z0-9_]+$`)

// Check ensures that for every element of oi.Options there is a field
// in oi.Opt that matches it.
//
// It also sets Option.FieldName to be the name of the field for use
// in JSON.
func (oi *OptionsInfo) Check() error {
	errCount := errcount.New()
	items, err := configstruct.Items(oi.Opt)
	if err != nil {
		return err
	}
	itemsByName := map[string]*configstruct.Item{}
	for i := range items {
		item := &items[i]
		itemsByName[item.Name] = item
		if !optionName.MatchString(item.Name) {
			err = fmt.Errorf("invalid name in `config:%q` in Options struct", item.Name)
			errCount.Add(err)
			Errorf(nil, "%s", err)
		}
	}
	for i := range oi.Options {
		option := &oi.Options[i]
		// Check name is correct
		if !optionName.MatchString(option.Name) {
			err = fmt.Errorf("invalid Name: %q", option.Name)
			errCount.Add(err)
			Errorf(nil, "%s", err)
			continue
		}
		// Check item exists
		item, found := itemsByName[option.Name]
		if !found {
			err = fmt.Errorf("key %q in OptionsInfo not found in Options struct", option.Name)
			errCount.Add(err)
			Errorf(nil, "%s", err)
			continue
		}
		// Check type
		optType := fmt.Sprintf("%T", option.Default)
		itemType := fmt.Sprintf("%T", item.Value)
		if optType != itemType {
			err = fmt.Errorf("key %q in has type %q in OptionsInfo.Default but type %q in Options struct", option.Name, optType, itemType)
			//errCount.Add(err)
			Errorf(nil, "%s", err)
		}
		// Set FieldName
		option.FieldName = item.Field
	}
	return errCount.Err(fmt.Sprintf("internal error: options block %q", oi.Name))
}

// load the defaults from the options
//
// Reload the options if required
func (oi *OptionsInfo) load() error {
	if oi.Options == nil {
		Errorf(nil, "No options defined for config block %q", oi.Name)
		return nil
	}

	m := ConfigMap("", oi.Options, "", nil)
	err := configstruct.Set(m, oi.Opt)
	if err != nil {
		return fmt.Errorf("failed to initialise %q options: %w", oi.Name, err)
	}

	if oi.Reload != nil {
		err = oi.Reload(context.Background())
		if err != nil {
			return fmt.Errorf("failed to reload %q options: %w", oi.Name, err)
		}
	}
	return nil
}

// GlobalOptionsInit initialises the defaults of global options to
// their values read from the options, environment variables and
// command line parameters.
func GlobalOptionsInit() error {
	var keys []string
	for key := range OptionsRegistry {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		// Sort alphabetically, but with "main" first
		if keys[i] == "main" {
			return true
		}
		if keys[j] == "main" {
			return false
		}
		return keys[i] < keys[j]
	})
	for _, key := range keys {
		opt := OptionsRegistry[key]
		err := opt.load()
		if err != nil {
			return err
		}
	}
	return nil
}

// Type returns a textual string to identify the type of the remote
func Type(f Fs) string {
	typeName := fmt.Sprintf("%T", f)
	typeName = strings.TrimPrefix(typeName, "*")
	typeName = strings.TrimSuffix(typeName, ".Fs")
	return typeName
}

var (
	typeToRegInfoMu sync.Mutex
	typeToRegInfo   = map[string]*RegInfo{}
)

// Add the RegInfo to the reverse map
func addReverse(f Fs, fsInfo *RegInfo) {
	typeToRegInfoMu.Lock()
	defer typeToRegInfoMu.Unlock()
	typeToRegInfo[Type(f)] = fsInfo
}

// FindFromFs finds the *RegInfo used to create this Fs, provided
// it was created by fs.NewFs or cache.Get
//
// It returns nil if not found
func FindFromFs(f Fs) *RegInfo {
	typeToRegInfoMu.Lock()
	defer typeToRegInfoMu.Unlock()
	return typeToRegInfo[Type(f)]
}
