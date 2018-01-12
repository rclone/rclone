package fs

import "github.com/spf13/pflag"

// Check it satisfies the interface
var _ pflag.Value = (*LogLevel)(nil)
