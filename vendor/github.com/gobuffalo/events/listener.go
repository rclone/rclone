package events

import (
	"fmt"
	"runtime"

	"github.com/pkg/errors"
)

// Listener is a function capable of handling events
type Listener func(e Event)

// NamedListen for events. Name is the name of the
// listener NOT the events you want to listen for,
// so something like "my-listener", "kafka-listener", etc...
func NamedListen(name string, l Listener) (DeleteFn, error) {
	return boss.Listen(name, l)
}

// Listen for events.
func Listen(l Listener) (DeleteFn, error) {
	_, file, line, _ := runtime.Caller(1)
	return NamedListen(fmt.Sprintf("%s:%d", file, line), l)
}

type listable interface {
	List() ([]string, error)
}

// List all listeners
func List() ([]string, error) {
	if l, ok := boss.(listable); ok {
		return l.List()
	}
	return []string{}, errors.Errorf("manager %T does not implemented listable", boss)
}
