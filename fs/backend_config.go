// Structures and utilities for backend config
//
//

package fs

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs/config/configmap"
)

const (
	// ConfigToken is the key used to store the token under
	ConfigToken = "token"

	// ConfigKeyEphemeralPrefix marks config keys which shouldn't be stored in the config file
	ConfigKeyEphemeralPrefix = "config_"
)

// ConfigOAuth should be called to do the OAuth
//
// set in lib/oauthutil to avoid a circular import
var ConfigOAuth func(ctx context.Context, name string, m configmap.Mapper, ri *RegInfo, in ConfigIn) (*ConfigOut, error)

// ConfigIn is passed to the Config function for an Fs
//
// The interactive config system for backends is state based. This is
// so that different frontends to the config can be attached, eg over
// the API or web page.
//
// Each call to the config system supplies ConfigIn which tells the
// system what to do. Each will return a ConfigOut which gives a
// question to ask the user and a state to return to. There is one
// special question which allows the backends to do OAuth.
//
// The ConfigIn contains a State which the backend should act upon and
// a Result from the previous question to the user.
//
// If ConfigOut is nil or ConfigOut.State == "" then the process is
// deemed to have finished. If there is no Option in ConfigOut then
// the next state will be called immediately. This is wrapped in
// ConfigGoto and ConfigResult.
//
// Backends should keep no state in memory - if they need to persist
// things between calls it should be persisted in the config file.
// Things can also be persisted in the state using the StatePush and
// StatePop utilities here.
//
// The utilities here are convenience methods for different kinds of
// questions and responses.
//
// Where the questions ask for a name then this should start with
// "config_" to show it is an ephemeral config input rather than the
// actual value stored in the config file. Names beginning with
// "config_fs_" are reserved for internal use.
//
// State names starting with "*" are reserved for internal use.
//
// Note that in the bin directory there is a python program called
// "config.py" which shows how this interface should be used.
type ConfigIn struct {
	State  string // State to run
	Result string // Result from previous Option
}

// ConfigOut is returned from Config function for an Fs
//
// State is the state for the next call to Config
// OAuth is a special value set by oauthutil.ConfigOAuth
// Error is displayed to the user before asking a question
// Result is passed to the next call to Config if Option/OAuth isn't set
type ConfigOut struct {
	State  string      // State to jump to after this
	Option *Option     // Option to query user about
	OAuth  interface{} `json:"-"` // Do OAuth if set
	Error  string      // error to be displayed to the user
	Result string      // if Option/OAuth not set then this is passed to the next state
}

// ConfigInputOptional asks the user for a string which may be empty
//
// state should be the next state required
// name is the config name for this item
// help should be the help shown to the user
func ConfigInputOptional(state string, name string, help string) (*ConfigOut, error) {
	return &ConfigOut{
		State: state,
		Option: &Option{
			Name:    name,
			Help:    help,
			Default: "",
		},
	}, nil
}

// ConfigInput asks the user for a non-empty string
//
// state should be the next state required
// name is the config name for this item
// help should be the help shown to the user
func ConfigInput(state string, name string, help string) (*ConfigOut, error) {
	out, _ := ConfigInputOptional(state, name, help)
	out.Option.Required = true
	return out, nil
}

// ConfigPassword asks the user for a password
//
// state should be the next state required
// name is the config name for this item
// help should be the help shown to the user
func ConfigPassword(state string, name string, help string) (*ConfigOut, error) {
	out, _ := ConfigInputOptional(state, name, help)
	out.Option.IsPassword = true
	return out, nil
}

// ConfigGoto goes to the next state with empty Result
//
// state should be the next state required
func ConfigGoto(state string) (*ConfigOut, error) {
	return &ConfigOut{
		State: state,
	}, nil
}

// ConfigResult goes to the next state with result given
//
// state should be the next state required
// result should be the result for the next state
func ConfigResult(state, result string) (*ConfigOut, error) {
	return &ConfigOut{
		State:  state,
		Result: result,
	}, nil
}

// ConfigError shows the error to the user and goes to the state passed in
//
// state should be the next state required
// Error should be the error shown to the user
func ConfigError(state string, Error string) (*ConfigOut, error) {
	return &ConfigOut{
		State: state,
		Error: Error,
	}, nil
}

// ConfigConfirm returns a ConfigOut structure which asks a Yes/No question
//
// state should be the next state required
// Default should be the default state
// name is the config name for this item
// help should be the help shown to the user
func ConfigConfirm(state string, Default bool, name string, help string) (*ConfigOut, error) {
	return &ConfigOut{
		State: state,
		Option: &Option{
			Name:    name,
			Help:    help,
			Default: Default,
			Examples: []OptionExample{{
				Value: "true",
				Help:  "Yes",
			}, {
				Value: "false",
				Help:  "No",
			}},
			Exclusive: true,
		},
	}, nil
}

// ConfigChooseFixed returns a ConfigOut structure which has a list of items to choose from.
//
// state should be the next state required
// name is the config name for this item
// help should be the help shown to the user
// items should be the items in the list
//
// It chooses the first item to be the default.
// If there are no items then it will return an error.
// If there is only one item it will short cut to the next state
func ConfigChooseFixed(state string, name string, help string, items []OptionExample) (*ConfigOut, error) {
	if len(items) == 0 {
		return nil, errors.Errorf("no items found in: %s", help)
	}
	choose := &ConfigOut{
		State: state,
		Option: &Option{
			Name:      name,
			Help:      help,
			Examples:  items,
			Exclusive: true,
		},
	}
	choose.Option.Default = choose.Option.Examples[0].Value
	if len(items) == 1 {
		// short circuit asking the question if only one entry
		choose.Result = choose.Option.Examples[0].Value
		choose.Option = nil
	}
	return choose, nil
}

// ConfigChoose returns a ConfigOut structure which has a list of items to choose from.
//
// state should be the next state required
// name is the config name for this item
// help should be the help shown to the user
// n should be the number of items in the list
// getItem should return the items (value, help)
//
// It chooses the first item to be the default.
// If there are no items then it will return an error.
// If there is only one item it will short cut to the next state
func ConfigChoose(state string, name string, help string, n int, getItem func(i int) (itemValue string, itemHelp string)) (*ConfigOut, error) {
	items := make(OptionExamples, n)
	for i := range items {
		items[i].Value, items[i].Help = getItem(i)
	}
	return ConfigChooseFixed(state, name, help, items)
}

// StatePush pushes a new values onto the front of the config string
func StatePush(state string, values ...string) string {
	for i := range values {
		values[i] = strings.Replace(values[i], ",", "，", -1) // replace comma with unicode wide version
	}
	if state != "" {
		values = append(values[:len(values):len(values)], state)
	}
	return strings.Join(values, ",")
}

type configOAuthKeyType struct{}

// OAuth key for config
var configOAuthKey = configOAuthKeyType{}

// ConfigOAuthOnly marks the ctx so that the Config will stop after
// finding an OAuth
func ConfigOAuthOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, configOAuthKey, struct{}{})
}

// Return true if ctx is marked as ConfigOAuthOnly
func isConfigOAuthOnly(ctx context.Context) bool {
	return ctx.Value(configOAuthKey) != nil
}

// StatePop pops a state from the front of the config string
// It returns the new state and the value popped
func StatePop(state string) (newState string, value string) {
	comma := strings.IndexRune(state, ',')
	if comma < 0 {
		return "", state
	}
	value, newState = state[:comma], state[comma+1:]
	value = strings.Replace(value, "，", ",", -1) // replace unicode wide comma with comma
	return newState, value
}

// BackendConfig calls the config for the backend in ri
//
// It wraps any OAuth transactions as necessary so only straight
// forward config questions are emitted
func BackendConfig(ctx context.Context, name string, m configmap.Mapper, ri *RegInfo, choices configmap.Getter, in ConfigIn) (out *ConfigOut, err error) {
	for {
		out, err = backendConfigStep(ctx, name, m, ri, choices, in)
		if err != nil {
			break
		}
		if out == nil || out.State == "" {
			// finished
			break
		}
		if out.Option != nil {
			// question to ask user
			break
		}
		if out.Error != "" {
			// error to show user
			break
		}
		// non terminal state, but no question to ask or error to show - loop here
		in = ConfigIn{
			State:  out.State,
			Result: out.Result,
		}
	}
	return out, err
}

// ConfigAll should be passed in as the initial state to run the
// entire config
const ConfigAll = "*all"

// Run the config state machine for the normal config
func configAll(ctx context.Context, name string, m configmap.Mapper, ri *RegInfo, in ConfigIn) (out *ConfigOut, err error) {
	if len(ri.Options) == 0 {
		return ConfigGoto("*postconfig")
	}

	// States are encoded
	//
	//     *all-ACTION,NUMBER,ADVANCED
	//
	// Where NUMBER is the curent state, ADVANCED is a flag true or false
	// to say whether we are asking about advanced config and
	// ACTION is what the state should be doing next.
	stateParams, state := StatePop(in.State)
	stateParams, stateNumber := StatePop(stateParams)
	_, stateAdvanced := StatePop(stateParams)

	optionNumber := 0
	advanced := stateAdvanced == "true"
	if stateNumber != "" {
		optionNumber, err = strconv.Atoi(stateNumber)
		if err != nil {
			return nil, errors.Wrap(err, "internal error: bad state number")
		}
	}

	// Detect if reached the end of the questions
	if optionNumber == len(ri.Options) {
		if ri.Options.HasAdvanced() {
			return ConfigConfirm("*all-advanced", false, "config_fs_advanced", "Edit advanced config?")
		}
		return ConfigGoto("*postconfig")
	} else if optionNumber < 0 || optionNumber > len(ri.Options) {
		return nil, errors.New("internal error: option out of range")
	}

	// Make the next state
	newState := func(state string, i int, advanced bool) string {
		return StatePush("", state, fmt.Sprint(i), fmt.Sprint(advanced))
	}

	// Find the current option
	option := &ri.Options[optionNumber]

	switch state {
	case "*all":
		// If option is hidden or doesn't match advanced setting then skip it
		if option.Hide&OptionHideConfigurator != 0 || option.Advanced != advanced {
			return ConfigGoto(newState("*all", optionNumber+1, advanced))
		}

		// Skip this question if it isn't the correct provider
		provider, _ := m.Get(ConfigProvider)
		if !MatchProvider(option.Provider, provider) {
			return ConfigGoto(newState("*all", optionNumber+1, advanced))
		}

		out = &ConfigOut{
			State:  newState("*all-set", optionNumber, advanced),
			Option: option,
		}

		// Filter examples by provider if necessary
		if provider != "" && len(option.Examples) > 0 {
			optionCopy := option.Copy()
			optionCopy.Examples = OptionExamples{}
			for _, example := range option.Examples {
				if MatchProvider(example.Provider, provider) {
					optionCopy.Examples = append(optionCopy.Examples, example)
				}
			}
			out.Option = optionCopy
		}

		return out, nil
	case "*all-set":
		// Set the value if not different to current
		// Note this won't set blank values in the config file
		// if the default is blank
		currentValue, _ := m.Get(option.Name)
		if currentValue != in.Result {
			m.Set(option.Name, in.Result)
		}
		// Find the next question
		return ConfigGoto(newState("*all", optionNumber+1, advanced))
	case "*all-advanced":
		// Reply to edit advanced question
		if in.Result == "true" {
			return ConfigGoto(newState("*all", 0, true))
		}
		return ConfigGoto("*postconfig")
	}
	return nil, errors.Errorf("internal error: bad state %q", state)
}

func backendConfigStep(ctx context.Context, name string, m configmap.Mapper, ri *RegInfo, choices configmap.Getter, in ConfigIn) (out *ConfigOut, err error) {
	ci := GetConfig(ctx)
	Debugf(name, "config in: state=%q, result=%q", in.State, in.Result)
	defer func() {
		Debugf(name, "config out: out=%+v, err=%v", out, err)
	}()

	switch {
	case strings.HasPrefix(in.State, ConfigAll):
		// Do all config
		out, err = configAll(ctx, name, m, ri, in)
	case strings.HasPrefix(in.State, "*oauth"):
		// Do internal oauth states
		out, err = ConfigOAuth(ctx, name, m, ri, in)
	case strings.HasPrefix(in.State, "*postconfig"):
		// Do the post config starting from state ""
		in.State = ""
		return backendConfigStep(ctx, name, m, ri, choices, in)
	case strings.HasPrefix(in.State, "*"):
		err = errors.Errorf("unknown internal state %q", in.State)
	default:
		// Otherwise pass to backend
		if ri.Config == nil {
			return nil, nil
		}
		out, err = ri.Config(ctx, name, m, in)
	}
	if err != nil {
		return nil, err
	}
	switch {
	case out == nil:
	case out.OAuth != nil:
		// If this is an OAuth state the deal with it here
		returnState := out.State
		// If rclone authorize, stop after doing oauth
		if isConfigOAuthOnly(ctx) {
			Debugf(nil, "OAuth only is set - overriding return state")
			returnState = ""
		}
		// Run internal state, saving the input so we can recall the state
		return ConfigGoto(StatePush("", "*oauth", returnState, in.State, in.Result))
	case out.Option != nil:
		if out.Option.Name == "" {
			return nil, errors.New("internal error: no name set in Option")
		}
		// If override value is set in the choices then use that
		if result, ok := choices.Get(out.Option.Name); ok {
			Debugf(nil, "Override value found, choosing value %q for state %q", result, out.State)
			return ConfigResult(out.State, result)
		}
		// If AutoConfirm is set, choose the default value
		if ci.AutoConfirm {
			result := fmt.Sprint(out.Option.Default)
			Debugf(nil, "Auto confirm is set, choosing default %q for state %q, override by setting config parameter %q", result, out.State, out.Option.Name)
			return ConfigResult(out.State, result)
		}
		// If fs.ConfigEdit is set then make the default value
		// in the config the current value.
		if result, ok := choices.Get(ConfigEdit); ok && result == "true" {
			if value, ok := m.Get(out.Option.Name); ok {
				newOption := out.Option.Copy()
				oldValue := newOption.Value
				err = newOption.Set(value)
				if err != nil {
					Errorf(nil, "Failed to set %q from %q - using default: %v", out.Option.Name, value, err)
				} else {
					newOption.Default = newOption.Value
					newOption.Value = oldValue
					out.Option = newOption
				}
			}
		}
	}
	return out, nil
}

// MatchProvider returns true if provider matches the providerConfig string.
//
// The providerConfig string can either be a list of providers to
// match, or if it starts with "!" it will be a list of providers not
// to match.
//
// If either providerConfig or provider is blank then it will return true
func MatchProvider(providerConfig, provider string) bool {
	if providerConfig == "" || provider == "" {
		return true
	}
	negate := false
	if strings.HasPrefix(providerConfig, "!") {
		providerConfig = providerConfig[1:]
		negate = true
	}
	providers := strings.Split(providerConfig, ",")
	matched := false
	for _, p := range providers {
		if p == provider {
			matched = true
			break
		}
	}
	if negate {
		return !matched
	}
	return matched
}
