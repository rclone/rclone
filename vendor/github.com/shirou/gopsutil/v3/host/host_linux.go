//go:build linux
// +build linux

package host

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/shirou/gopsutil/v3/internal/common"
)

type lsbStruct struct {
	ID          string
	Release     string
	Codename    string
	Description string
}

// from utmp.h
const (
	user_PROCESS = 7

	hostTemperatureScale = 1000.0
)

func HostIDWithContext(ctx context.Context) (string, error) {
	sysProductUUID := common.HostSysWithContext(ctx, "class/dmi/id/product_uuid")
	machineID := common.HostEtcWithContext(ctx, "machine-id")
	procSysKernelRandomBootID := common.HostProcWithContext(ctx, "sys/kernel/random/boot_id")
	switch {
	// In order to read this file, needs to be supported by kernel/arch and run as root
	// so having fallback is important
	case common.PathExists(sysProductUUID):
		lines, err := common.ReadLines(sysProductUUID)
		if err == nil && len(lines) > 0 && lines[0] != "" {
			return strings.ToLower(lines[0]), nil
		}
		fallthrough
	// Fallback on GNU Linux systems with systemd, readable by everyone
	case common.PathExists(machineID):
		lines, err := common.ReadLines(machineID)
		if err == nil && len(lines) > 0 && len(lines[0]) == 32 {
			st := lines[0]
			return fmt.Sprintf("%s-%s-%s-%s-%s", st[0:8], st[8:12], st[12:16], st[16:20], st[20:32]), nil
		}
		fallthrough
	// Not stable between reboot, but better than nothing
	default:
		lines, err := common.ReadLines(procSysKernelRandomBootID)
		if err == nil && len(lines) > 0 && lines[0] != "" {
			return strings.ToLower(lines[0]), nil
		}
	}

	return "", nil
}

func numProcs(ctx context.Context) (uint64, error) {
	return common.NumProcsWithContext(ctx)
}

func BootTimeWithContext(ctx context.Context) (uint64, error) {
	return common.BootTimeWithContext(ctx)
}

func UptimeWithContext(ctx context.Context) (uint64, error) {
	sysinfo := &unix.Sysinfo_t{}
	if err := unix.Sysinfo(sysinfo); err != nil {
		return 0, err
	}
	return uint64(sysinfo.Uptime), nil
}

func UsersWithContext(ctx context.Context) ([]UserStat, error) {
	utmpfile := common.HostVarWithContext(ctx, "run/utmp")

	file, err := os.Open(utmpfile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	count := len(buf) / sizeOfUtmp

	ret := make([]UserStat, 0, count)

	for i := 0; i < count; i++ {
		b := buf[i*sizeOfUtmp : (i+1)*sizeOfUtmp]

		var u utmp
		br := bytes.NewReader(b)
		err := binary.Read(br, binary.LittleEndian, &u)
		if err != nil {
			continue
		}
		if u.Type != user_PROCESS {
			continue
		}
		user := UserStat{
			User:     common.IntToString(u.User[:]),
			Terminal: common.IntToString(u.Line[:]),
			Host:     common.IntToString(u.Host[:]),
			Started:  int(u.Tv.Sec),
		}
		ret = append(ret, user)
	}

	return ret, nil
}

func getlsbStruct(ctx context.Context) (*lsbStruct, error) {
	ret := &lsbStruct{}
	if common.PathExists(common.HostEtcWithContext(ctx, "lsb-release")) {
		contents, err := common.ReadLines(common.HostEtcWithContext(ctx, "lsb-release"))
		if err != nil {
			return ret, err // return empty
		}
		for _, line := range contents {
			field := strings.Split(line, "=")
			if len(field) < 2 {
				continue
			}
			switch field[0] {
			case "DISTRIB_ID":
				ret.ID = field[1]
			case "DISTRIB_RELEASE":
				ret.Release = field[1]
			case "DISTRIB_CODENAME":
				ret.Codename = field[1]
			case "DISTRIB_DESCRIPTION":
				ret.Description = field[1]
			}
		}
	} else if common.PathExists("/usr/bin/lsb_release") {
		out, err := invoke.Command("/usr/bin/lsb_release")
		if err != nil {
			return ret, err
		}
		for _, line := range strings.Split(string(out), "\n") {
			field := strings.Split(line, ":")
			if len(field) < 2 {
				continue
			}
			switch field[0] {
			case "Distributor ID":
				ret.ID = field[1]
			case "Release":
				ret.Release = field[1]
			case "Codename":
				ret.Codename = field[1]
			case "Description":
				ret.Description = field[1]
			}
		}

	}

	return ret, nil
}

func PlatformInformationWithContext(ctx context.Context) (platform string, family string, version string, err error) {
	lsb, err := getlsbStruct(ctx)
	if err != nil {
		lsb = &lsbStruct{}
	}

	if common.PathExistsWithContents(common.HostEtcWithContext(ctx, "oracle-release")) {
		platform = "oracle"
		contents, err := common.ReadLines(common.HostEtcWithContext(ctx, "oracle-release"))
		if err == nil {
			version = getRedhatishVersion(contents)
		}

	} else if common.PathExistsWithContents(common.HostEtcWithContext(ctx, "enterprise-release")) {
		platform = "oracle"
		contents, err := common.ReadLines(common.HostEtcWithContext(ctx, "enterprise-release"))
		if err == nil {
			version = getRedhatishVersion(contents)
		}
	} else if common.PathExistsWithContents(common.HostEtcWithContext(ctx, "slackware-version")) {
		platform = "slackware"
		contents, err := common.ReadLines(common.HostEtcWithContext(ctx, "slackware-version"))
		if err == nil {
			version = getSlackwareVersion(contents)
		}
	} else if common.PathExistsWithContents(common.HostEtcWithContext(ctx, "debian_version")) {
		if lsb.ID == "Ubuntu" {
			platform = "ubuntu"
			version = lsb.Release
		} else if lsb.ID == "LinuxMint" {
			platform = "linuxmint"
			version = lsb.Release
		} else if lsb.ID == "Kylin" {
			platform = "Kylin"
			version = lsb.Release
		} else if lsb.ID == `"Cumulus Linux"` {
			platform = "cumuluslinux"
			version = lsb.Release
		} else {
			if common.PathExistsWithContents("/usr/bin/raspi-config") {
				platform = "raspbian"
			} else {
				platform = "debian"
			}
			contents, err := common.ReadLines(common.HostEtcWithContext(ctx, "debian_version"))
			if err == nil && len(contents) > 0 && contents[0] != "" {
				version = contents[0]
			}
		}
	} else if common.PathExists(common.HostEtcWithContext(ctx, "neokylin-release")) {
		contents, err := common.ReadLines(common.HostEtcWithContext(ctx, "neokylin-release"))
		if err == nil {
			version = getRedhatishVersion(contents)
			platform = getRedhatishPlatform(contents)
		}
	} else if common.PathExists(common.HostEtcWithContext(ctx, "redhat-release")) {
		contents, err := common.ReadLines(common.HostEtcWithContext(ctx, "redhat-release"))
		if err == nil {
			version = getRedhatishVersion(contents)
			platform = getRedhatishPlatform(contents)
		}
	} else if common.PathExists(common.HostEtcWithContext(ctx, "system-release")) {
		contents, err := common.ReadLines(common.HostEtcWithContext(ctx, "system-release"))
		if err == nil {
			version = getRedhatishVersion(contents)
			platform = getRedhatishPlatform(contents)
		}
	} else if common.PathExists(common.HostEtcWithContext(ctx, "gentoo-release")) {
		platform = "gentoo"
		contents, err := common.ReadLines(common.HostEtcWithContext(ctx, "gentoo-release"))
		if err == nil {
			version = getRedhatishVersion(contents)
		}
	} else if common.PathExists(common.HostEtcWithContext(ctx, "SuSE-release")) {
		contents, err := common.ReadLines(common.HostEtcWithContext(ctx, "SuSE-release"))
		if err == nil {
			version = getSuseVersion(contents)
			platform = getSusePlatform(contents)
		}
		// TODO: slackware detecion
	} else if common.PathExists(common.HostEtcWithContext(ctx, "arch-release")) {
		platform = "arch"
		version = lsb.Release
	} else if common.PathExists(common.HostEtcWithContext(ctx, "alpine-release")) {
		platform = "alpine"
		contents, err := common.ReadLines(common.HostEtcWithContext(ctx, "alpine-release"))
		if err == nil && len(contents) > 0 && contents[0] != "" {
			version = contents[0]
		}
	} else if common.PathExists(common.HostEtcWithContext(ctx, "os-release")) {
		p, v, err := common.GetOSReleaseWithContext(ctx)
		if err == nil {
			platform = p
			version = v
		}
	} else if lsb.ID == "RedHat" {
		platform = "redhat"
		version = lsb.Release
	} else if lsb.ID == "Amazon" {
		platform = "amazon"
		version = lsb.Release
	} else if lsb.ID == "ScientificSL" {
		platform = "scientific"
		version = lsb.Release
	} else if lsb.ID == "XenServer" {
		platform = "xenserver"
		version = lsb.Release
	} else if lsb.ID != "" {
		platform = strings.ToLower(lsb.ID)
		version = lsb.Release
	}

	platform = strings.Trim(platform, `"`)

	switch platform {
	case "debian", "ubuntu", "linuxmint", "raspbian", "Kylin", "cumuluslinux":
		family = "debian"
	case "fedora":
		family = "fedora"
	case "oracle", "centos", "redhat", "scientific", "enterpriseenterprise", "amazon", "xenserver", "cloudlinux", "ibm_powerkvm", "rocky", "almalinux":
		family = "rhel"
	case "suse", "opensuse", "opensuse-leap", "opensuse-tumbleweed", "opensuse-tumbleweed-kubic", "sles", "sled", "caasp":
		family = "suse"
	case "gentoo":
		family = "gentoo"
	case "slackware":
		family = "slackware"
	case "arch":
		family = "arch"
	case "exherbo":
		family = "exherbo"
	case "alpine":
		family = "alpine"
	case "coreos":
		family = "coreos"
	case "solus":
		family = "solus"
	case "neokylin":
		family = "neokylin"
	}

	return platform, family, version, nil
}

func KernelVersionWithContext(ctx context.Context) (version string, err error) {
	var utsname unix.Utsname
	err = unix.Uname(&utsname)
	if err != nil {
		return "", err
	}
	return unix.ByteSliceToString(utsname.Release[:]), nil
}

func getSlackwareVersion(contents []string) string {
	c := strings.ToLower(strings.Join(contents, ""))
	c = strings.Replace(c, "slackware ", "", 1)
	return c
}

func getRedhatishVersion(contents []string) string {
	c := strings.ToLower(strings.Join(contents, ""))

	if strings.Contains(c, "rawhide") {
		return "rawhide"
	}
	if matches := regexp.MustCompile(`release (\w[\d.]*)`).FindStringSubmatch(c); matches != nil {
		return matches[1]
	}
	return ""
}

func getRedhatishPlatform(contents []string) string {
	c := strings.ToLower(strings.Join(contents, ""))

	if strings.Contains(c, "red hat") {
		return "redhat"
	}
	f := strings.Split(c, " ")

	return f[0]
}

func getSuseVersion(contents []string) string {
	version := ""
	for _, line := range contents {
		if matches := regexp.MustCompile(`VERSION = ([\d.]+)`).FindStringSubmatch(line); matches != nil {
			version = matches[1]
		} else if matches := regexp.MustCompile(`PATCHLEVEL = ([\d]+)`).FindStringSubmatch(line); matches != nil {
			version = version + "." + matches[1]
		}
	}
	return version
}

func getSusePlatform(contents []string) string {
	c := strings.ToLower(strings.Join(contents, ""))
	if strings.Contains(c, "opensuse") {
		return "opensuse"
	}
	return "suse"
}

func VirtualizationWithContext(ctx context.Context) (string, string, error) {
	return common.VirtualizationWithContext(ctx)
}

func SensorsTemperaturesWithContext(ctx context.Context) ([]TemperatureStat, error) {
	var err error

	var files []string

	temperatures := make([]TemperatureStat, 0)

	// Only the temp*_input file provides current temperature
	// value in millidegree Celsius as reported by the temperature to the device:
	// https://www.kernel.org/doc/Documentation/hwmon/sysfs-interface
	if files, err = filepath.Glob(common.HostSysWithContext(ctx, "/class/hwmon/hwmon*/temp*_input")); err != nil {
		return temperatures, err
	}

	if len(files) == 0 {
		// CentOS has an intermediate /device directory:
		// https://github.com/giampaolo/psutil/issues/971
		if files, err = filepath.Glob(common.HostSysWithContext(ctx, "/class/hwmon/hwmon*/device/temp*_input")); err != nil {
			return temperatures, err
		}
	}

	var warns Warnings

	if len(files) == 0 { // handle distributions without hwmon, like raspbian #391, parse legacy thermal_zone files
		files, err = filepath.Glob(common.HostSysWithContext(ctx, "/class/thermal/thermal_zone*/"))
		if err != nil {
			return temperatures, err
		}
		for _, file := range files {
			// Get the name of the temperature you are reading
			name, err := ioutil.ReadFile(filepath.Join(file, "type"))
			if err != nil {
				warns.Add(err)
				continue
			}
			// Get the temperature reading
			current, err := ioutil.ReadFile(filepath.Join(file, "temp"))
			if err != nil {
				warns.Add(err)
				continue
			}
			temperature, err := strconv.ParseInt(strings.TrimSpace(string(current)), 10, 64)
			if err != nil {
				warns.Add(err)
				continue
			}

			temperatures = append(temperatures, TemperatureStat{
				SensorKey:   strings.TrimSpace(string(name)),
				Temperature: float64(temperature) / 1000.0,
			})
		}
		return temperatures, warns.Reference()
	}

	temperatures = make([]TemperatureStat, 0, len(files))

	// example directory
	// device/           temp1_crit_alarm  temp2_crit_alarm  temp3_crit_alarm  temp4_crit_alarm  temp5_crit_alarm  temp6_crit_alarm  temp7_crit_alarm
	// name              temp1_input       temp2_input       temp3_input       temp4_input       temp5_input       temp6_input       temp7_input
	// power/            temp1_label       temp2_label       temp3_label       temp4_label       temp5_label       temp6_label       temp7_label
	// subsystem/        temp1_max         temp2_max         temp3_max         temp4_max         temp5_max         temp6_max         temp7_max
	// temp1_crit        temp2_crit        temp3_crit        temp4_crit        temp5_crit        temp6_crit        temp7_crit        uevent
	for _, file := range files {
		var raw []byte

		var temperature float64

		// Get the base directory location
		directory := filepath.Dir(file)

		// Get the base filename prefix like temp1
		basename := strings.Split(filepath.Base(file), "_")[0]

		// Get the base path like <dir>/temp1
		basepath := filepath.Join(directory, basename)

		// Get the label of the temperature you are reading
		label := ""

		if raw, _ = ioutil.ReadFile(basepath + "_label"); len(raw) != 0 {
			// Format the label from "Core 0" to "core_0"
			label = strings.Join(strings.Split(strings.TrimSpace(strings.ToLower(string(raw))), " "), "_")
		}

		// Get the name of the temperature you are reading
		if raw, err = ioutil.ReadFile(filepath.Join(directory, "name")); err != nil {
			warns.Add(err)
			continue
		}

		name := strings.TrimSpace(string(raw))

		if label != "" {
			name = name + "_" + label
		}

		// Get the temperature reading
		if raw, err = ioutil.ReadFile(file); err != nil {
			warns.Add(err)
			continue
		}

		if temperature, err = strconv.ParseFloat(strings.TrimSpace(string(raw)), 64); err != nil {
			warns.Add(err)
			continue
		}

		// Add discovered temperature sensor to the list
		temperatures = append(temperatures, TemperatureStat{
			SensorKey:   name,
			Temperature: temperature / hostTemperatureScale,
			High:        optionalValueReadFromFile(basepath+"_max") / hostTemperatureScale,
			Critical:    optionalValueReadFromFile(basepath+"_crit") / hostTemperatureScale,
		})
	}

	return temperatures, warns.Reference()
}

func optionalValueReadFromFile(filename string) float64 {
	var raw []byte

	var err error

	var value float64

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return 0
	}

	if raw, err = ioutil.ReadFile(filename); err != nil {
		return 0
	}

	if value, err = strconv.ParseFloat(strings.TrimSpace(string(raw)), 64); err != nil {
		return 0
	}

	return value
}
