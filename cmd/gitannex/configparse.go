package gitannex

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/fspath"
)

type configID int

const (
	configRemoteName configID = iota
	configPrefix
	configLayout
)

// configDefinition describes a configuration value required by this command. We
// use "GETCONFIG" messages to query git-annex for these values at runtime.
type configDefinition struct {
	id           configID
	names        []string
	description  string
	defaultValue string
}

const (
	defaultRclonePrefix = "git-annex-rclone"
	defaultRcloneLayout = "nodir"
)

var requiredConfigs = []configDefinition{
	{
		id:    configRemoteName,
		names: []string{"rcloneremotename", "target"},
		description: "Name of the rclone remote to use. " +
			"Must match a remote known to rclone. " +
			"(Note that rclone remotes are a distinct concept from git-annex remotes.)",
	},
	{
		id:    configPrefix,
		names: []string{"rcloneprefix", "prefix"},
		description: "Directory where rclone will write git-annex content. " +
			fmt.Sprintf("If not specified, defaults to %q. ", defaultRclonePrefix) +
			"This directory will be created on init if it does not exist.",
		defaultValue: defaultRclonePrefix,
	},
	{
		id:    configLayout,
		names: []string{"rclonelayout", "rclone_layout"},
		description: "Defines where, within the rcloneprefix directory, rclone will write git-annex content. " +
			fmt.Sprintf("Must be one of %v. ", allLayoutModes()) +
			fmt.Sprintf("If empty, defaults to %q.", defaultRcloneLayout),
		defaultValue: defaultRcloneLayout,
	},
}

func (c *configDefinition) getCanonicalName() string {
	if len(c.names) < 1 {
		panic(fmt.Errorf("configDefinition must have at least one name: %v", c))
	}
	return c.names[0]
}

// fullDescription returns a single-line, human-readable description for this
// config. The returned string begins with a list of synonyms and ends with
// `c.description`.
func (c *configDefinition) fullDescription() string {
	if len(c.names) <= 1 {
		return c.description
	}
	// Exclude the canonical name from the list of synonyms.
	synonyms := c.names[1:len(c.names)]
	commaSeparatedSynonyms := strings.Join(synonyms, ", ")
	return fmt.Sprintf("(synonyms: %s) %s", commaSeparatedSynonyms, c.description)
}

// validateRemoteName validates the "rcloneremotename" config that we receive
// from git-annex. It returns nil iff `value` is valid. Otherwise, it returns a
// descriptive error suitable for sending back to git-annex via stdout.
//
// The value is only valid when:
//  1. It is the exact name of an existing remote.
//  2. It is an fspath string that names an existing remote or a backend. The
//     string may include options, but it must not include a path. (That's what
//     the "rcloneprefix" config is for.)
//
// While backends are not remote names, per se, they are permitted for
// compatibility with [fstest]. We could guard this behavior behind
// [testing.Testing] to prevent users from specifying backend strings, but
// there's no obvious harm in permitting it.
func validateRemoteName(value string) error {
	remoteNames := config.GetRemoteNames()
	// Check whether `value` is an exact match for an existing remote.
	//
	// If we checked whether [cache.Get] returns [fs.ErrorNotFoundInConfigFile],
	// we would incorrectly identify file names as valid remote names. We also
	// avoid [config.FileSections] because it will miss remotes that are defined
	// by environment variables.
	if slices.Contains(remoteNames, value) {
		return nil
	}
	parsed, err := fspath.Parse(value)
	if err != nil {
		return fmt.Errorf("remote could not be parsed: %s", value)
	}
	if parsed.Path != "" {
		return fmt.Errorf("remote does not exist or incorrectly contains a path: %s", value)
	}
	// Now that we've established `value` is an fspath string that does not
	// include a path component, we only need to check whether it names an
	// existing remote or backend.
	if slices.Contains(remoteNames, parsed.Name) {
		return nil
	}
	maybeBackend := strings.HasPrefix(value, ":")
	if !maybeBackend {
		return fmt.Errorf("remote does not exist: %s", value)
	}
	// Strip the leading colon before searching for the backend. For instance,
	// search for "local" instead of ":local". Note that `parsed.Name` already
	// omits any config options baked into the string.
	trimmedBackendName := strings.TrimPrefix(parsed.Name, ":")
	if _, err = fs.Find(trimmedBackendName); err != nil {
		return fmt.Errorf("backend does not exist: %s", trimmedBackendName)
	}
	return nil
}
