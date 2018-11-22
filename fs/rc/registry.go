// Define the registry

package rc

import (
	"sort"
	"strings"
	"sync"

	"github.com/ncw/rclone/fs"
)

// Func defines a type for a remote control function
type Func func(in Params) (out Params, err error)

// Call defines info about a remote control function and is used in
// the Add function to create new entry points.
type Call struct {
	Path         string // path to activate this RC
	Fn           Func   `json:"-"` // function to call
	Title        string // help for the function
	AuthRequired bool   // if set then this call requires authorisation to be set
	Help         string // multi-line markdown formatted help
}

// Registry holds the list of all the registered remote control functions
type Registry struct {
	mu   sync.RWMutex
	call map[string]*Call
}

// NewRegistry makes a new registry for remote control functions
func NewRegistry() *Registry {
	return &Registry{
		call: make(map[string]*Call),
	}
}

// Add a call to the registry
func (r *Registry) Add(call Call) {
	r.mu.Lock()
	defer r.mu.Unlock()
	call.Path = strings.Trim(call.Path, "/")
	call.Help = strings.TrimSpace(call.Help)
	fs.Debugf(nil, "Adding path %q to remote control registry", call.Path)
	r.call[call.Path] = &call
}

// Get a Call from a path or nil
func (r *Registry) Get(path string) *Call {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.call[path]
}

// List of all calls in alphabetical order
func (r *Registry) List() (out []*Call) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var keys []string
	for key := range r.call {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = append(out, r.call[key])
	}
	return out
}

// Calls is the global registry of Call objects
var Calls = NewRegistry()

// Add a function to the global registry
func Add(call Call) {
	Calls.Add(call)
}
