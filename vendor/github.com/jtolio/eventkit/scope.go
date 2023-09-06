package eventkit

import (
	"time"
)

type Scope struct {
	r    *Registry
	name []string
}

func (s *Scope) Subscope(name string) *Scope {
	return &Scope{r: s.r, name: append(append([]string(nil), s.name...), name)}
}

func (s *Scope) Event(name string, tags ...Tag) {
	s.r.Submit(&Event{
		Name:      name,
		Scope:     s.name,
		Timestamp: time.Now(),
		Tags:      tags,
	})
}
