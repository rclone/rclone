package oncer

import (
	"fmt"
	"io"
	"os"
)

const deprecated = "DEPRECATED"

var deprecationWriter io.Writer = os.Stdout

func Deprecate(depth int, name string, msg string) {
	Do(deprecated+name, func() {
		fmt.Fprintf(deprecationWriter, "[%s] %s has been deprecated.\n", deprecated, name)
		if len(msg) > 0 {
			fmt.Fprintf(deprecationWriter, "\t%s\n", msg)
		}
	})
}
