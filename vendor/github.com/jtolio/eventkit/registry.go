package eventkit

import (
	"time"
)

type Event struct {
	Name      string
	Scope     []string
	Timestamp time.Time
	Tags      []Tag
}

type Destination interface {
	Submit(*Event)
}

type Registry struct {
	dests []Destination
}

func NewRegistry() *Registry { return &Registry{} }

func (r *Registry) Scope(name string) *Scope {
	return &Scope{
		r:    r,
		name: []string{name},
	}
}

// AddDestination adds an output to the registry. Do not call
// AddDestination if (*Registry).Submit might be called
// concurrently. It is expected that AddDestination will be
// called at initialization time before any events.
func (r *Registry) AddDestination(dest Destination) {
	r.dests = append(r.dests, dest)
}

// Submit submits an Event to all added Destinations.
func (r *Registry) Submit(e *Event) {
	for _, dest := range r.dests {
		dest.Submit(e)
	}
}
