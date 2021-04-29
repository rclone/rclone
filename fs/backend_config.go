// Structures and utilities for backend config
//
//

package fs

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs/config/configmap"
)

const (
	// ConfigToken is the key used to store the token under
	ConfigToken = "token"
)

// ConfigOAuth should be called to do the OAuth
//
// set in lib/oauthutil to avoid a circular import
var ConfigOAuth func(ctx context.Context, name string, m configmap.Mapper, ri *RegInfo, in ConfigIn) (*ConfigOut, error)

// ConfigIn is passed to the Config function for an Fs
//
// The interactive config system for backends is state based. This is so that different frontends to the config can be attached, eg over the API or web page.
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

// ConfigInput asks the user for a string
//
// state should be the next state required
// help should be the help shown to the user
func ConfigInput(state string, help string) (*ConfigOut, error) {
	return &ConfigOut{
		State: state,
		Option: &Option{
			Help:    help,
			Default: "",
		},
	}, nil
}

// ConfigPassword asks the user for a password
//
// state should be the next state required
// help should be the help shown to the user
func ConfigPassword(state string, help string) (*ConfigOut, error) {
	return &ConfigOut{
		State: state,
		Option: &Option{
			Help:       help,
			Default:    "",
			IsPassword: true,
		},
	}, nil
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
// help should be the help shown to the user
func ConfigConfirm(state string, Default bool, help string) (*ConfigOut, error) {
	return &ConfigOut{
		State: state,
		Option: &Option{
			Help:    help,
			Default: Default,
			Examples: []OptionExample{{
				Value: "true",
				Help:  "Yes",
			}, {
				Value: "false",
				Help:  "No",
			}},
		},
	}, nil
}

// ConfigChooseFixed returns a ConfigOut structure which has a list of items to choose from.
//
// state should be the next state required
// help should be the help shown to the user
// items should be the items in the list
//
// It chooses the first item to be the default.
// If there are no items then it will return an error.
// If there is only one item it will short cut to the next state
func ConfigChooseFixed(state string, help string, items []OptionExample) (*ConfigOut, error) {
	if len(items) == 0 {
		return nil, errors.Errorf("no items found in: %s", help)
	}
	choose := &ConfigOut{
		State: state,
		Option: &Option{
			Help:     help,
			Examples: items,
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
// help should be the help shown to the user
// n should be the number of items in the list
// getItem should return the items (value, help)
//
// It chooses the first item to be the default.
// If there are no items then it will return an error.
// If there is only one item it will short cut to the next state
func ConfigChoose(state string, help string, n int, getItem func(i int) (itemValue string, itemHelp string)) (*ConfigOut, error) {
	items := make(OptionExamples, n)
	for i := range items {
		items[i].Value, items[i].Help = getItem(i)
	}
	return ConfigChooseFixed(state, help, items)
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
// It wraps any OAuth transactions as necessary so only straight forward config questions are emitted
func BackendConfig(ctx context.Context, name string, m configmap.Mapper, ri *RegInfo, in ConfigIn) (*ConfigOut, error) {
	ci := GetConfig(ctx)
	if ri.Config == nil {
		return nil, nil
	}
	// Do internal states here
	if strings.HasPrefix(in.State, "*") {
		switch {
		case strings.HasPrefix(in.State, "*oauth"):
			return ConfigOAuth(ctx, name, m, ri, in)
		default:
			return nil, errors.Errorf("unknown internal state %q", in.State)
		}
	}
	out, err := ri.Config(ctx, name, m, in)
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
	case out.Option != nil && ci.AutoConfirm:
		// If AutoConfirm is set, choose the default value
		result := fmt.Sprint(out.Option.Default)
		Debugf(nil, "Auto confirm is set, choosing default %q for state %q", result, out.State)
		return ConfigResult(out.State, result)
	}
	return out, nil
}
