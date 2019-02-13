package adb

import (
	"bufio"
	"strings"

	"github.com/thinkhy/go-adb/internal/errors"
)

type DeviceInfo struct {
	// Always set.
	Serial string
	Status string

	// Product, device, and model are not set in the short form.
	Product    string
	Model      string
	DeviceInfo string

	// Only set for devices connected via USB.
	Usb string
}

// IsUsb returns true if the device is connected via USB.
func (d *DeviceInfo) IsUsb() bool {
	return d.Usb != ""
}

func newDevice(serial string, status string, attrs map[string]string) (*DeviceInfo, error) {
	if serial == "" {
		return nil, errors.AssertionErrorf("device serial cannot be blank")
	}

	return &DeviceInfo{
		Serial:     serial,
		Status:     status,
		Product:    attrs["product"],
		Model:      attrs["model"],
		DeviceInfo: attrs["device"],
		Usb:        attrs["usb"],
	}, nil
}

func parseDeviceList(list string, lineParseFunc func(string) (*DeviceInfo, error)) ([]*DeviceInfo, error) {
	devices := []*DeviceInfo{}
	scanner := bufio.NewScanner(strings.NewReader(list))

	for scanner.Scan() {
		device, err := lineParseFunc(scanner.Text())
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}

	return devices, nil
}

func parseDeviceShort(line string) (*DeviceInfo, error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return nil, errors.Errorf(errors.ParseError,
			"malformed device line, expected 2 fields but found %d", len(fields))
	}

	return newDevice(fields[0], fields[1], map[string]string{})
}

func parseDeviceLong(line string) (*DeviceInfo, error) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return nil, errors.Errorf(errors.ParseError,
			"malformed device line, expected at least 5 fields but found %d", len(fields))
	}

	attrs := parseDeviceAttributes(fields[2:])
	return newDevice(fields[0], fields[1], attrs)
}

func parseDeviceAttributes(fields []string) map[string]string {
	attrs := map[string]string{}
	for _, field := range fields {
		key, val := parseKeyVal(field)
		attrs[key] = val
	}
	return attrs
}

// Parses a key:val pair and returns key, val.
func parseKeyVal(pair string) (string, string) {
	split := strings.Split(pair, ":")
	return split[0], split[1]
}
