package oncer

import (
	"sync"
)

var onces = &sync.Map{}

func Do(name string, fn func()) {
	o, _ := onces.LoadOrStore(name, &sync.Once{})
	if once, ok := o.(*sync.Once); ok {
		once.Do(fn)
	}
}

func Reset(names ...string) {
	if len(names) == 0 {
		onces = &sync.Map{}
		return
	}

	for _, n := range names {
		onces.Delete(n)
		onces.Delete(deprecated + n)
	}
}
