// Implement config options reading and writing
//
// This is done here rather than in fs/fs.go so we don't cause a circular dependency

package rc

import (
	"context"
	"fmt"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
)

func init() {
	Add(Call{
		Path:  "options/blocks",
		Fn:    rcOptionsBlocks,
		Title: "List all the option blocks",
		Help: `Returns:
- options - a list of the options block names`,
	})
}

// Show the list of all the option blocks
func rcOptionsBlocks(ctx context.Context, in Params) (out Params, err error) {
	options := []string{}
	for _, opt := range fs.OptionsRegistry {
		options = append(options, opt.Name)
	}
	out = make(Params)
	out["options"] = options
	return out, nil
}

func init() {
	Add(Call{
		Path:  "options/get",
		Fn:    rcOptionsGet,
		Title: "Get all the global options",
		Help: `Returns an object where keys are option block names and values are an
object with the current option values in.

Parameters:

- blocks: optional string of comma separated blocks to include
    - all are included if this is missing or ""

Note that these are the global options which are unaffected by use of
the _config and _filter parameters. If you wish to read the parameters
set in _config then use options/config and for _filter use options/filter.

This shows the internal names of the option within rclone which should
map to the external options very easily with a few exceptions.
`,
	})
}

// Filter the blocks according to name
func filterBlocks(in Params, f func(oi fs.OptionsInfo)) (err error) {
	blocksStr, err := in.GetString("blocks")
	if err != nil && !IsErrParamNotFound(err) {
		return err
	}
	blocks := map[string]struct{}{}
	for _, name := range strings.Split(blocksStr, ",") {
		if name != "" {
			blocks[name] = struct{}{}
		}
	}
	for _, oi := range fs.OptionsRegistry {
		if _, found := blocks[oi.Name]; found || len(blocks) == 0 {
			f(oi)
		}
	}
	return nil
}

// Show the list of all the option blocks
func rcOptionsGet(ctx context.Context, in Params) (out Params, err error) {
	out = make(Params)
	err = filterBlocks(in, func(oi fs.OptionsInfo) {
		out[oi.Name] = oi.Opt
	})
	return out, err
}

func init() {
	Add(Call{
		Path:  "options/info",
		Fn:    rcOptionsInfo,
		Title: "Get info about all the global options",
		Help: `Returns an object where keys are option block names and values are an
array of objects with info about each options.

Parameters:

- blocks: optional string of comma separated blocks to include
    - all are included if this is missing or ""

These objects are in the same format as returned by "config/providers". They are
described in the [option blocks](#option-blocks) section.
`,
	})
}

// Show the info of all the option blocks
func rcOptionsInfo(ctx context.Context, in Params) (out Params, err error) {
	out = make(Params)
	err = filterBlocks(in, func(oi fs.OptionsInfo) {
		out[oi.Name] = oi.Options
	})
	return out, err
}

func init() {
	Add(Call{
		Path:  "options/local",
		Fn:    rcOptionsLocal,
		Title: "Get the currently active config for this call",
		Help: `Returns an object with the keys "config" and "filter".
The "config" key contains the local config and the "filter" key contains
the local filters.

Note that these are the local options specific to this rc call. If
_config was not supplied then they will be the global options.
Likewise with "_filter".

This call is mostly useful for seeing if _config and _filter passing
is working.

This shows the internal names of the option within rclone which should
map to the external options very easily with a few exceptions.
`,
	})
}

// Show the current config
func rcOptionsLocal(ctx context.Context, in Params) (out Params, err error) {
	out = make(Params)
	out["config"] = fs.GetConfig(ctx)
	out["filter"] = filter.GetConfig(ctx).Opt
	return out, nil
}

func init() {
	Add(Call{
		Path:  "options/set",
		Fn:    rcOptionsSet,
		Title: "Set an option",
		Help: `Parameters:

- option block name containing an object with
  - key: value

Repeated as often as required.

Only supply the options you wish to change.  If an option is unknown
it will be silently ignored.  Not all options will have an effect when
changed like this.

For example:

This sets DEBUG level logs (-vv) (these can be set by number or string)

    rclone rc options/set --json '{"main": {"LogLevel": "DEBUG"}}'
    rclone rc options/set --json '{"main": {"LogLevel": 8}}'

And this sets INFO level logs (-v)

    rclone rc options/set --json '{"main": {"LogLevel": "INFO"}}'

And this sets NOTICE level logs (normal without -v)

    rclone rc options/set --json '{"main": {"LogLevel": "NOTICE"}}'
`,
	})
}

// Set an option in an option block
func rcOptionsSet(ctx context.Context, in Params) (out Params, err error) {
	for name, options := range in {
		opt, ok := fs.OptionsRegistry[name]
		if !ok {
			return nil, fmt.Errorf("unknown option block %q", name)
		}
		err := Reshape(opt.Opt, options)
		if err != nil {
			return nil, fmt.Errorf("failed to write options from block %q: %w", name, err)
		}
		if opt.Reload != nil {
			err = opt.Reload(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to reload options from block %q: %w", name, err)
			}
		}
	}
	return out, nil
}
