//go:build snap

package buildinfo

func init() {
	Tags = append(Tags, "snap")
}
