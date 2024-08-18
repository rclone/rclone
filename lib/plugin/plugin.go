//go:build (darwin || linux) && !gccgo

package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"
)

func init() {
	dir := os.Getenv("RCLONE_PLUGIN_PATH")
	if dir == "" {
		return
	}
	// Get file names of plugin dir
	listing, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to open plugin directory:", err)
	}
	// Enumerate file names, load valid plugins
	for _, file := range listing {
		// Match name
		fileName := file.Name()
		if !strings.HasPrefix(fileName, "librcloneplugin_") {
			continue
		}
		if !strings.HasSuffix(fileName, ".so") {
			continue
		}
		// Try to load plugin
		_, err := plugin.Open(filepath.Join(dir, fileName))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load plugin %s: %s\n",
				fileName, err)
		}
	}
}
