//go:build solaris
// +build solaris

package net

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/internal/common"
)

// NetIOCounters returnes network I/O statistics for every network
// interface installed on the system.  If pernic argument is false,
// return only sum of all information (which name is 'all'). If true,
// every network interface installed on the system is returned
// separately.
func IOCounters(pernic bool) ([]IOCountersStat, error) {
	return IOCountersWithContext(context.Background(), pernic)
}

func IOCountersWithContext(ctx context.Context, pernic bool) ([]IOCountersStat, error) {
	// collect all the net class's links with below statistics
	filterstr := "/^(?!vnic)/::phys:/^rbytes64$|^ipackets64$|^idrops64$|^ierrors$|^obytes64$|^opackets64$|^odrops64$|^oerrors$/"
	if runtime.GOOS == "illumos" {
		filterstr = "/[^vnic]/::mac:/^rbytes64$|^ipackets64$|^idrops64$|^ierrors$|^obytes64$|^opackets64$|^odrops64$|^oerrors$/"
	}
	kstatSysOut, err := invoke.CommandWithContext(ctx, "kstat", "-c", "net", "-p", filterstr)
	if err != nil {
		return nil, fmt.Errorf("cannot execute kstat: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(kstatSysOut)), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("no interface found")
	}
	rbytes64arr := make(map[string]uint64)
	ipackets64arr := make(map[string]uint64)
	idrops64arr := make(map[string]uint64)
	ierrorsarr := make(map[string]uint64)
	obytes64arr := make(map[string]uint64)
	opackets64arr := make(map[string]uint64)
	odrops64arr := make(map[string]uint64)
	oerrorsarr := make(map[string]uint64)

	re := regexp.MustCompile(`[:\s]+`)
	for _, line := range lines {
		fields := re.Split(line, -1)
		interfaceName := fields[0]
		instance := fields[1]
		switch fields[3] {
		case "rbytes64":
			rbytes64arr[interfaceName+instance], err = strconv.ParseUint(fields[4], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse rbytes64: %w", err)
			}
		case "ipackets64":
			ipackets64arr[interfaceName+instance], err = strconv.ParseUint(fields[4], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse ipackets64: %w", err)
			}
		case "idrops64":
			idrops64arr[interfaceName+instance], err = strconv.ParseUint(fields[4], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse idrops64: %w", err)
			}
		case "ierrors":
			ierrorsarr[interfaceName+instance], err = strconv.ParseUint(fields[4], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse ierrors: %w", err)
			}
		case "obytes64":
			obytes64arr[interfaceName+instance], err = strconv.ParseUint(fields[4], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse obytes64: %w", err)
			}
		case "opackets64":
			opackets64arr[interfaceName+instance], err = strconv.ParseUint(fields[4], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse opackets64: %w", err)
			}
		case "odrops64":
			odrops64arr[interfaceName+instance], err = strconv.ParseUint(fields[4], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse odrops64: %w", err)
			}
		case "oerrors":
			oerrorsarr[interfaceName+instance], err = strconv.ParseUint(fields[4], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse oerrors: %w", err)
			}
		}
	}
	ret := make([]IOCountersStat, 0)
	for k := range rbytes64arr {
		nic := IOCountersStat{
			Name:        k,
			BytesRecv:   rbytes64arr[k],
			PacketsRecv: ipackets64arr[k],
			Errin:       ierrorsarr[k],
			Dropin:      idrops64arr[k],
			BytesSent:   obytes64arr[k],
			PacketsSent: opackets64arr[k],
			Errout:      oerrorsarr[k],
			Dropout:     odrops64arr[k],
		}
		ret = append(ret, nic)
	}

	if !pernic {
		return getIOCountersAll(ret)
	}

	return ret, nil
}

func Connections(kind string) ([]ConnectionStat, error) {
	return ConnectionsWithContext(context.Background(), kind)
}

func ConnectionsWithContext(ctx context.Context, kind string) ([]ConnectionStat, error) {
	return []ConnectionStat{}, common.ErrNotImplementedError
}

func FilterCounters() ([]FilterStat, error) {
	return FilterCountersWithContext(context.Background())
}

func FilterCountersWithContext(ctx context.Context) ([]FilterStat, error) {
	return []FilterStat{}, common.ErrNotImplementedError
}

func ProtoCounters(protocols []string) ([]ProtoCountersStat, error) {
	return ProtoCountersWithContext(context.Background(), protocols)
}

func ProtoCountersWithContext(ctx context.Context, protocols []string) ([]ProtoCountersStat, error) {
	return []ProtoCountersStat{}, common.ErrNotImplementedError
}
