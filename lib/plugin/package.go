// Package plugin implements loading out-of-tree storage backends
// using https://golang.org/pkg/plugin/ on Linux and macOS.
//
// If the $RCLONE_PLUGIN_PATH is present, any Go plugins in that dir
// named like librcloneplugin_NAME.so will be loaded.
//
// To create a plugin, write the backend package like it was in-tree
// but set the package name to "main". Then, build the plugin with
//
//	go build -buildmode=plugin -o librcloneplugin_NAME.so
//
// where NAME equals the plugin's fs.RegInfo.Name.
package plugin

// Build for plugin for unsupported platforms to stop go complaining
// about "no buildable Go source files".
