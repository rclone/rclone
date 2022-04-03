// Package common defines code common to the union and the policies
//
// These need to be defined in a separate package to avoid import loops
package common

import "github.com/rclone/rclone/fs"

// Options defines the configuration for this backend
type Options struct {
	Upstreams    fs.SpaceSepList `config:"upstreams"`
	Remotes      fs.SpaceSepList `config:"remotes"` // Deprecated
	ActionPolicy string          `config:"action_policy"`
	CreatePolicy string          `config:"create_policy"`
	SearchPolicy string          `config:"search_policy"`
	CacheTime    int             `config:"cache_time"`
}
