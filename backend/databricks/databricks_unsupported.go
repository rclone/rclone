// Build for databricks for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9

// Package databricks provides a filesystem interface using github.com/databricks/databricks-sdk-go
package databricks
