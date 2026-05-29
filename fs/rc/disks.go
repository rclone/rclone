//go:build !(netbsd && 386)

package rc

import "github.com/shirou/gopsutil/v4/disk"

// getMounts returns a slice of disk mount points
func getMounts() (mounts []string) {
	partitions, _ := disk.Partitions(false)
	for _, partition := range partitions {
		mounts = append(mounts, partition.Mountpoint)
	}
	return mounts
}
