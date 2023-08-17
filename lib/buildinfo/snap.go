//go:build snap
// +build snap

package buildinfo

func init() {
	Tags = append(Tags, "snap")
}
