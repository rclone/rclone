package host

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/internal/common"
)

func HostIDWithContext(ctx context.Context) (string, error) {
	platform, err := parseReleaseFile()
	if err != nil {
		return "", err
	}

	if platform == "SmartOS" {
		// If everything works, use the current zone ID as the HostID if present.
		out, err := invoke.CommandWithContext(ctx, "zonename")
		if err == nil {
			sc := bufio.NewScanner(bytes.NewReader(out))
			for sc.Scan() {
				line := sc.Text()

				// If we're in the global zone, rely on the hostname.
				if line == "global" {
					hostname, err := os.Hostname()
					if err == nil {
						return hostname, nil
					}
				} else {
					return strings.TrimSpace(line), nil
				}
			}
		}
	}

	// If HostID is still unknown, use hostid(1), which can lie to callers but at
	// this point there are no hardware facilities available.  This behavior
	// matches that of other supported OSes.
	out, err := invoke.CommandWithContext(ctx, "hostid")
	if err == nil {
		sc := bufio.NewScanner(bytes.NewReader(out))
		for sc.Scan() {
			line := sc.Text()
			return strings.TrimSpace(line), nil
		}
	}

	return "", nil
}

// Count number of processes based on the number of entries in /proc
func numProcs(ctx context.Context) (uint64, error) {
	dirs, err := ioutil.ReadDir("/proc")
	if err != nil {
		return 0, err
	}
	return uint64(len(dirs)), nil
}

var kstatMatch = regexp.MustCompile(`([^\s]+)[\s]+([^\s]*)`)

func BootTimeWithContext(ctx context.Context) (uint64, error) {
	out, err := invoke.CommandWithContext(ctx, "kstat", "-p", "unix:0:system_misc:boot_time")
	if err != nil {
		return 0, err
	}

	kstats := kstatMatch.FindAllStringSubmatch(string(out), -1)
	if len(kstats) != 1 {
		return 0, fmt.Errorf("expected 1 kstat, found %d", len(kstats))
	}

	return strconv.ParseUint(kstats[0][2], 10, 64)
}

func UptimeWithContext(ctx context.Context) (uint64, error) {
	bootTime, err := BootTime()
	if err != nil {
		return 0, err
	}
	return timeSince(bootTime), nil
}

func UsersWithContext(ctx context.Context) ([]UserStat, error) {
	return []UserStat{}, common.ErrNotImplementedError
}

func SensorsTemperaturesWithContext(ctx context.Context) ([]TemperatureStat, error) {
	var ret []TemperatureStat

	out, err := invoke.CommandWithContext(ctx, "ipmitool", "-c", "sdr", "list")
	if err != nil {
		return ret, err
	}

	r := csv.NewReader(strings.NewReader(string(out)))
	// Output may contain errors, e.g. "bmc_send_cmd: Permission denied", don't expect a consistent number of records
	r.FieldsPerRecord = -1
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ret, err
		}
		// CPU1 Temp,40,degrees C,ok
		if len(record) < 3 || record[1] == "" || record[2] != "degrees C" {
			continue
		}
		v, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			return ret, err
		}
		ts := TemperatureStat{
			SensorKey:   strings.TrimSuffix(record[0], " Temp"),
			Temperature: v,
		}
		ret = append(ret, ts)
	}

	return ret, nil
}

func VirtualizationWithContext(ctx context.Context) (string, string, error) {
	return "", "", common.ErrNotImplementedError
}

// Find distribution name from /etc/release
func parseReleaseFile() (string, error) {
	b, err := ioutil.ReadFile("/etc/release")
	if err != nil {
		return "", err
	}
	s := string(b)
	s = strings.TrimSpace(s)

	var platform string

	switch {
	case strings.HasPrefix(s, "SmartOS"):
		platform = "SmartOS"
	case strings.HasPrefix(s, "OpenIndiana"):
		platform = "OpenIndiana"
	case strings.HasPrefix(s, "OmniOS"):
		platform = "OmniOS"
	case strings.HasPrefix(s, "Open Storage"):
		platform = "NexentaStor"
	case strings.HasPrefix(s, "Solaris"):
		platform = "Solaris"
	case strings.HasPrefix(s, "Oracle Solaris"):
		platform = "Solaris"
	default:
		platform = strings.Fields(s)[0]
	}

	return platform, nil
}

// parseUnameOutput returns platformFamily, kernelVersion and platformVersion
func parseUnameOutput(ctx context.Context) (string, string, string, error) {
	out, err := invoke.CommandWithContext(ctx, "uname", "-srv")
	if err != nil {
		return "", "", "", err
	}

	fields := strings.Fields(string(out))
	if len(fields) < 3 {
		return "", "", "", fmt.Errorf("malformed `uname` output")
	}

	return fields[0], fields[1], fields[2], nil
}

func KernelVersionWithContext(ctx context.Context) (string, error) {
	_, kernelVersion, _, err := parseUnameOutput(ctx)
	return kernelVersion, err
}

func PlatformInformationWithContext(ctx context.Context) (string, string, string, error) {
	platform, err := parseReleaseFile()
	if err != nil {
		return "", "", "", err
	}

	platformFamily, _, platformVersion, err := parseUnameOutput(ctx)
	if err != nil {
		return "", "", "", err
	}

	return platform, platformFamily, platformVersion, nil
}
