// Package configmap provides an abstraction for reading and writing config
package configmap

import (
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"
	"unicode"

	"github.com/pkg/errors"
)

// Priority of getters
type Priority int8

// Priority levels for AddGetter
const (
	PriorityNormal  Priority = iota
	PriorityConfig           // use for reading from the config
	PriorityDefault          // use for default values
	PriorityMax
)

// Getter provides an interface to get config items
type Getter interface {
	// Get should get an item with the key passed in and return
	// the value. If the item is found then it should return true,
	// otherwise false.
	Get(key string) (value string, ok bool)
}

// Setter provides an interface to set config items
type Setter interface {
	// Set should set an item into persistent config store.
	Set(key, value string)
}

// Mapper provides an interface to read and write config
type Mapper interface {
	Getter
	Setter
}

// Map provides a wrapper around multiple Setter and
// Getter interfaces.
type Map struct {
	setters []Setter
	getters []getprio
}

type getprio struct {
	getter   Getter
	priority Priority
}

// New returns an empty Map
func New() *Map {
	return &Map{}
}

// AddGetter appends a getter onto the end of the getters in priority order
func (c *Map) AddGetter(getter Getter, priority Priority) *Map {
	c.getters = append(c.getters, getprio{getter, priority})
	sort.SliceStable(c.getters, func(i, j int) bool {
		return c.getters[i].priority < c.getters[j].priority
	})
	return c
}

// AddSetter appends a setter onto the end of the setters
func (c *Map) AddSetter(setter Setter) *Map {
	c.setters = append(c.setters, setter)
	return c
}

// ClearSetters removes all the setters set so far
func (c *Map) ClearSetters() *Map {
	c.setters = nil
	return c
}

// ClearGetters removes all the getters with the priority given
func (c *Map) ClearGetters(priority Priority) *Map {
	getters := c.getters[:0]
	for _, item := range c.getters {
		if item.priority != priority {
			getters = append(getters, item)
		}
	}
	c.getters = getters
	return c
}

// GetPriority gets an item with the key passed in and return the
// value from the first getter to return a result with priority <=
// maxPriority. If the item is found then it returns true, otherwise
// false.
func (c *Map) GetPriority(key string, maxPriority Priority) (value string, ok bool) {
	for _, item := range c.getters {
		if item.priority > maxPriority {
			break
		}
		value, ok = item.getter.Get(key)
		if ok {
			return value, ok
		}
	}
	return "", false
}

// Get gets an item with the key passed in and return the value from
// the first getter. If the item is found then it returns true,
// otherwise false.
func (c *Map) Get(key string) (value string, ok bool) {
	return c.GetPriority(key, PriorityMax)
}

// Set sets an item into all the stored setters.
func (c *Map) Set(key, value string) {
	for _, do := range c.setters {
		do.Set(key, value)
	}
}

// Simple is a simple Mapper for testing
type Simple map[string]string

// Get the value
func (c Simple) Get(key string) (value string, ok bool) {
	value, ok = c[key]
	return value, ok
}

// Set the value
func (c Simple) Set(key, value string) {
	c[key] = value
}

// String the map value the same way the config parser does, but with
// sorted keys for reproducability.
func (c Simple) String() string {
	var ks = make([]string, 0, len(c))
	for k := range c {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	var out strings.Builder
	for _, k := range ks {
		if out.Len() > 0 {
			out.WriteRune(',')
		}
		out.WriteString(k)
		out.WriteRune('=')
		out.WriteRune('\'')
		for _, ch := range c[k] {
			out.WriteRune(ch)
			// Escape ' as ''
			if ch == '\'' {
				out.WriteRune(ch)
			}
		}
		out.WriteRune('\'')
	}
	return out.String()
}

// Encode from c into a string suitable for putting on the command line
func (c Simple) Encode() (string, error) {
	if len(c) == 0 {
		return "", nil
	}
	buf, err := json.Marshal(c)
	if err != nil {
		return "", errors.Wrap(err, "encode simple map")
	}
	return base64.RawStdEncoding.EncodeToString(buf), nil
}

// Decode an Encode~d string in into c
func (c Simple) Decode(in string) error {
	// Remove all whitespace from the input string
	in = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, in)
	if len(in) == 0 {
		return nil
	}
	decodedM, err := base64.RawStdEncoding.DecodeString(in)
	if err != nil {
		return errors.Wrap(err, "decode simple map")
	}
	err = json.Unmarshal(decodedM, &c)
	if err != nil {
		return errors.Wrap(err, "parse simple map")
	}
	return nil
}
