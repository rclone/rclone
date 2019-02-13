package adb

import "github.com/thinkhy/go-adb/internal/errors"

// DeviceState represents one of the 3 possible states adb will report devices.
// A device can be communicated with when it's in StateOnline.
// A USB device will make the following state transitions:
// 	Plugged in: StateDisconnected->StateOffline->StateOnline
// 	Unplugged:  StateOnline->StateDisconnected
//go:generate stringer -type=DeviceState
type DeviceState int8

const (
	StateInvalid DeviceState = iota
	StateUnauthorized
	StateDisconnected
	StateOffline
	StateConnecting
	StateOnline
)

var deviceStateStrings = map[string]DeviceState{
	"":             StateDisconnected,
	"offline":      StateOffline,
	"connecting":   StateConnecting,
	"device":       StateOnline,
	"unauthorized": StateUnauthorized,
}

func parseDeviceState(str string) (DeviceState, error) {
	state, ok := deviceStateStrings[str]
	if !ok {
		return StateInvalid, errors.Errorf(errors.ParseError, "invalid device state: %q", state)
	}
	return state, nil
}
