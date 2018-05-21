// Package configmap provides an abstraction for reading and writing config
package configmap

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
	getters []Getter
}

// New returns an empty Map
func New() *Map {
	return &Map{}
}

// AddGetter appends a getter onto the end of the getters
func (c *Map) AddGetter(getter Getter) *Map {
	c.getters = append(c.getters, getter)
	return c
}

// AddGetters appends multiple getters onto the end of the getters
func (c *Map) AddGetters(getters ...Getter) *Map {
	c.getters = append(c.getters, getters...)
	return c
}

// AddSetter appends a setter onto the end of the setters
func (c *Map) AddSetter(setter Setter) *Map {
	c.setters = append(c.setters, setter)
	return c
}

// Get gets an item with the key passed in and return the value from
// the first getter. If the item is found then it returns true,
// otherwise false.
func (c *Map) Get(key string) (value string, ok bool) {
	for _, do := range c.getters {
		value, ok = do.Get(key)
		if ok {
			return value, ok
		}
	}
	return "", false
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
