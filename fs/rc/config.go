// Implement config options reading and writing
//
// This is done here rather than in fs/fs.go so we don't cause a circular dependency

package rc

import (
	"github.com/pkg/errors"
)

var optionBlock = map[string]interface{}{}

// AddOption adds an option set
func AddOption(name string, option interface{}) {
	optionBlock[name] = option
}

func init() {
	Add(Call{
		Path:  "options/blocks",
		Fn:    rcOptionsBlocks,
		Title: "List all the option blocks",
		Help: `Returns
- options - a list of the options block names`,
	})
}

// Show the list of all the option blocks
func rcOptionsBlocks(in Params) (out Params, err error) {
	options := []string{}
	for name := range optionBlock {
		options = append(options, name)
	}
	out = make(Params)
	out["options"] = options
	return out, nil
}

func init() {
	Add(Call{
		Path:  "options/get",
		Fn:    rcOptionsGet,
		Title: "Get all the options",
		Help: `Returns an object where keys are option block names and values are an
object with the current option values in.

This shows the internal names of the option within rclone which should
map to the external options very easily with a few exceptions.
`,
	})
}

// Show the list of all the option blocks
func rcOptionsGet(in Params) (out Params, err error) {
	out = make(Params)
	for name, options := range optionBlock {
		out[name] = options
	}
	return out, nil
}

func init() {
	Add(Call{
		Path:  "options/set",
		Fn:    rcOptionsSet,
		Title: "Set an option",
		Help: `Parameters

- option block name containing an object with
  - key: value

Repeated as often as required.

Only supply the options you wish to change.  If an option is unknown
it will be silently ignored.  Not all options will have an effect when
changed like this.
`,
	})
}

// Set an option in an option block
func rcOptionsSet(in Params) (out Params, err error) {
	for name, options := range in {
		current := optionBlock[name]
		if current == nil {
			return nil, errors.Errorf("unknown option block %q", name)
		}
		err := Reshape(current, options)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to write options from block %q", name)
		}

	}
	return out, nil
}
