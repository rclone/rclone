// +build !windows

package times

const hasPlatformSpecificStat = false

// do not use, only here to prevent "undefined" method error.
func platformSpecficStat(name string) (Timespec, error) {
	return nil, nil
}
