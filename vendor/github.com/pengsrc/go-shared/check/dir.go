package check

import (
	"fmt"
	"os"
)

// Dir checks the given path, will return error if path not exists or path
// is not directory.
func Dir(path string) error {
	if info, err := os.Stat(path); err != nil {
		return fmt.Errorf(`directory not exists: %s`, path)
	} else if !info.IsDir() {
		return fmt.Errorf(`path is not directory: %s`, path)
	}
	return nil
}
