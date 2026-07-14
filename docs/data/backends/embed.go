// Package backends provides embedded backend config
package backends

import "embed"

//go:embed *.yaml

// BackendFS contains the backend YAML files
var BackendFS embed.FS
