// Package backends provides embedded backend config
package backends

import "embed"

//go:embed *.yaml
var BackendFS embed.FS
