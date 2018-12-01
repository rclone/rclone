package plugdeps

import "encoding/json"

// Command is the plugin command you want to control further
type Command struct {
	Name     string    `toml:"name" json:"name"`
	Flags    []string  `toml:"flags,omitempty" json:"flags,omitempty"`
	Commands []Command `toml:"command,omitempty" json:"commands,omitempty"`
}

// String implementation of fmt.Stringer
func (p Command) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}
