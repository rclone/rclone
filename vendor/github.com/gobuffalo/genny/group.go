package genny

import "sync"

type Group struct {
	Generators []*Generator
	moot       sync.RWMutex
}

func (gg *Group) Add(g *Generator) {
	m := &gg.moot
	m.Lock()
	defer m.Unlock()
	gg.Generators = append(gg.Generators, g)
}

func (gg *Group) Merge(g2 *Group) {
	for _, g := range g2.Generators {
		gg.Add(g)
	}
}

func (gg *Group) With(r *Runner) {
	m := &gg.moot
	m.RLock()
	defer m.RUnlock()
	for _, g := range gg.Generators {
		r.With(g)
	}
}
