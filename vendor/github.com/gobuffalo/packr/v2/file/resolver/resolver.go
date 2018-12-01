package resolver

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/gobuffalo/packr/v2/file"
)

type Resolver interface {
	Resolve(string, string) (file.File, error)
}

func defaultResolver() Resolver {
	pwd, _ := os.Getwd()
	return &Disk{
		Root: pwd,
	}
}

var DefaultResolver = defaultResolver()

func String(r Resolver) string {
	m := map[string]interface{}{
		"name": fmt.Sprintf("%T", r),
	}
	if fm, ok := r.(file.FileMappable); ok {
		m["files"] = fm
	}
	b, _ := json.Marshal(m)
	return string(b)
}
