package fstestutil

import (
	"errors"
	"io/ioutil"
	"strings"
)

// Linux /proc/mounts shows current mounts.
// Same format as /etc/fstab. Quoting getmntent(3):
//
// Since fields in the mtab and fstab files are separated by whitespace,
// octal escapes are used to represent the four characters space (\040),
// tab (\011), newline (\012) and backslash (\134) in those files when
// they occur in one of the four strings in a mntent structure.
//
// http://linux.die.net/man/3/getmntent

var fstabUnescape = strings.NewReplacer(
	`\040`, "\040",
	`\011`, "\011",
	`\012`, "\012",
	`\134`, "\134",
)

var errNotFound = errors.New("mount not found")

func getMountInfo(mnt string) (*MountInfo, error) {
	data, err := ioutil.ReadFile("/proc/mounts")
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		// Fields are: fsname dir type opts freq passno
		fsname := fstabUnescape.Replace(fields[0])
		dir := fstabUnescape.Replace(fields[1])
		fstype := fstabUnescape.Replace(fields[2])
		if mnt == dir {
			info := &MountInfo{
				FSName: fsname,
				Type:   fstype,
			}
			return info, nil
		}
	}
	return nil, errNotFound
}
