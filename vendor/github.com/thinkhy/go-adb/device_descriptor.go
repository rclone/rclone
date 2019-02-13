package adb

import "fmt"

//go:generate stringer -type=deviceDescriptorType
type deviceDescriptorType int

const (
	// host:transport-any and host:<request>
	DeviceAny deviceDescriptorType = iota
	// host:transport:<serial> and host-serial:<serial>:<request>
	DeviceSerial
	// host:transport-usb and host-usb:<request>
	DeviceUsb
	// host:transport-local and host-local:<request>
	DeviceLocal
)

type DeviceDescriptor struct {
	descriptorType deviceDescriptorType

	// Only used if Type is DeviceSerial.
	serial string
}

func AnyDevice() DeviceDescriptor {
	return DeviceDescriptor{descriptorType: DeviceAny}
}

func AnyUsbDevice() DeviceDescriptor {
	return DeviceDescriptor{descriptorType: DeviceUsb}
}

func AnyLocalDevice() DeviceDescriptor {
	return DeviceDescriptor{descriptorType: DeviceLocal}
}

func DeviceWithSerial(serial string) DeviceDescriptor {
	return DeviceDescriptor{
		descriptorType: DeviceSerial,
		serial:         serial,
	}
}

func (d DeviceDescriptor) String() string {
	if d.descriptorType == DeviceSerial {
		return fmt.Sprintf("%s[%s]", d.descriptorType, d.serial)
	}
	return d.descriptorType.String()
}

func (d DeviceDescriptor) getHostPrefix() string {
	switch d.descriptorType {
	case DeviceAny:
		return "host"
	case DeviceUsb:
		return "host-usb"
	case DeviceLocal:
		return "host-local"
	case DeviceSerial:
		return fmt.Sprintf("host-serial:%s", d.serial)
	default:
		panic(fmt.Sprintf("invalid DeviceDescriptorType: %v", d.descriptorType))
	}
}

func (d DeviceDescriptor) getTransportDescriptor() string {
	switch d.descriptorType {
	case DeviceAny:
		return "transport-any"
	case DeviceUsb:
		return "transport-usb"
	case DeviceLocal:
		return "transport-local"
	case DeviceSerial:
		return fmt.Sprintf("transport:%s", d.serial)
	default:
		panic(fmt.Sprintf("invalid DeviceDescriptorType: %v", d.descriptorType))
	}
}
