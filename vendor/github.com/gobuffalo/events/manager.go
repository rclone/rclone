package events

import (
	"sort"
	"strings"
	"sync"

	"github.com/markbates/safe"
	"github.com/pkg/errors"
)

type DeleteFn func()

// Manager can be implemented to replace the default
// events manager
type Manager interface {
	Listen(string, Listener) (DeleteFn, error)
	Emit(Event) error
}

// DefaultManager implements a map backed Manager
func DefaultManager() Manager {
	return &manager{
		moot:      &sync.RWMutex{},
		listeners: map[string]Listener{},
	}
}

// SetManager allows you to replace the default
// event manager with a custom one
func SetManager(m Manager) {
	boss = m
}

var boss Manager = DefaultManager()
var _ listable = &manager{}

type manager struct {
	moot      *sync.RWMutex
	listeners map[string]Listener
}

func (m *manager) Listen(name string, l Listener) (DeleteFn, error) {
	m.moot.RLock()
	_, ok := m.listeners[name]
	m.moot.RUnlock()
	if ok {
		return nil, errors.Errorf("listener named %s is already listening", name)
	}

	m.moot.Lock()
	m.listeners[name] = l
	m.moot.Unlock()

	df := func() {
		m.moot.Lock()
		delete(m.listeners, name)
		m.moot.Unlock()
	}

	return df, nil
}

func (m *manager) Emit(e Event) error {
	if err := e.Validate(); err != nil {
		return errors.WithStack(err)
	}
	e.Kind = strings.ToLower(e.Kind)
	if e.IsError() && e.Error == nil {
		e.Error = errors.New(e.Kind)
	}
	go func(e Event) {
		m.moot.Lock()
		defer m.moot.Unlock()
		for _, l := range m.listeners {
			ex := Event{
				Kind:    e.Kind,
				Error:   e.Error,
				Message: e.Message,
				Payload: Payload{},
			}
			for k, v := range e.Payload {
				ex.Payload[k] = v
			}
			go func(e Event, l Listener) {
				safe.Run(func() {
					l(e)
				})
			}(ex, l)
		}
	}(e)
	return nil
}

func (m *manager) List() ([]string, error) {
	var names []string
	m.moot.RLock()
	for k := range m.listeners {
		names = append(names, k)
	}
	m.moot.RUnlock()
	sort.Strings(names)
	return names, nil
}
