package adb

import (
	"log"
	"math/rand"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/thinkhy/go-adb/internal/errors"
	"github.com/thinkhy/go-adb/wire"
)

/*
DeviceWatcher publishes device status change events.
If the server dies while listening for events, it restarts the server.
*/
type DeviceWatcher struct {
	*deviceWatcherImpl
}

// DeviceStateChangedEvent represents a device state transition.
// Contains the device’s old and new states, but also provides methods to query the
// type of state transition.
type DeviceStateChangedEvent struct {
	Serial   string
	OldState DeviceState
	NewState DeviceState
}

// CameOnline returns true if this event represents a device coming online.
func (s DeviceStateChangedEvent) CameOnline() bool {
	return s.OldState != StateOnline && s.NewState == StateOnline
}

// WentOffline returns true if this event represents a device going offline.
func (s DeviceStateChangedEvent) WentOffline() bool {
	return s.OldState == StateOnline && s.NewState != StateOnline
}

type deviceWatcherImpl struct {
	server server

	// If an error occurs, it is stored here and eventChan is close immediately after.
	err atomic.Value

	eventChan chan DeviceStateChangedEvent
}

func newDeviceWatcher(server server) *DeviceWatcher {
	watcher := &DeviceWatcher{&deviceWatcherImpl{
		server:    server,
		eventChan: make(chan DeviceStateChangedEvent),
	}}

	runtime.SetFinalizer(watcher, func(watcher *DeviceWatcher) {
		watcher.Shutdown()
	})

	go publishDevices(watcher.deviceWatcherImpl)

	return watcher
}

/*
C returns a channel than can be received on to get events.
If an unrecoverable error occurs, or Shutdown is called, the channel will be closed.
*/
func (w *DeviceWatcher) C() <-chan DeviceStateChangedEvent {
	return w.eventChan
}

// Err returns the error that caused the channel returned by C to be closed, if C is closed.
// If C is not closed, its return value is undefined.
func (w *DeviceWatcher) Err() error {
	if err, ok := w.err.Load().(error); ok {
		return err
	}
	return nil
}

// Shutdown stops the watcher from listening for events and closes the channel returned
// from C.
func (w *DeviceWatcher) Shutdown() {
	// TODO(z): Implement.
}

func (w *deviceWatcherImpl) reportErr(err error) {
	w.err.Store(err)
}

/*
publishDevices reads device lists from scanner, calculates diffs, and publishes events on
eventChan.
Returns when scanner returns an error.
Doesn't refer directly to a *DeviceWatcher so it can be GCed (which will,
in turn, close Scanner and stop this goroutine).

TODO: to support shutdown, spawn a new goroutine each time a server connection is established.
This goroutine should read messages and send them to a message channel. Can write errors directly
to errVal. publishDevicesUntilError should take the msg chan and the scanner and select on the msg chan and stop chan, and if the stop
chan sends, close the scanner and return true. If the msg chan closes, just return false.
publishDevices can look at ret val: if false and err == EOF, reconnect. If false and other error, report err
and abort. If true, report no error and stop.
*/
func publishDevices(watcher *deviceWatcherImpl) {
	defer close(watcher.eventChan)

	var lastKnownStates map[string]DeviceState
	finished := false

	for {
		scanner, err := connectToTrackDevices(watcher.server)
		if err != nil {
			watcher.reportErr(err)
			return
		}

		finished, err = publishDevicesUntilError(scanner, watcher.eventChan, &lastKnownStates)

		if finished {
			scanner.Close()
			return
		}

		if HasErrCode(err, ConnectionResetError) {
			// The server died, restart and reconnect.

			// Delay by a random [0ms, 500ms) in case multiple DeviceWatchers are trying to
			// start the same server.
			delay := time.Duration(rand.Intn(500)) * time.Millisecond

			log.Printf("[DeviceWatcher] server died, restarting in %s…", delay)
			time.Sleep(delay)
			if err := watcher.server.Start(); err != nil {
				log.Println("[DeviceWatcher] error restarting server, giving up")
				watcher.reportErr(err)
				return
			} // Else server should be running, continue listening.
		} else {
			// Unknown error, don't retry.
			watcher.reportErr(err)
			return
		}
	}
}

func connectToTrackDevices(server server) (wire.Scanner, error) {
	conn, err := server.Dial()
	if err != nil {
		return nil, err
	}

	if err := wire.SendMessageString(conn, "host:track-devices"); err != nil {
		conn.Close()
		return nil, err
	}

	if _, err := conn.ReadStatus("host:track-devices"); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func publishDevicesUntilError(scanner wire.Scanner, eventChan chan<- DeviceStateChangedEvent, lastKnownStates *map[string]DeviceState) (finished bool, err error) {
	for {
		msg, err := scanner.ReadMessage()
		if err != nil {
			return false, err
		}

		deviceStates, err := parseDeviceStates(string(msg))
		if err != nil {
			return false, err
		}

		for _, event := range calculateStateDiffs(*lastKnownStates, deviceStates) {
			eventChan <- event
		}
		*lastKnownStates = deviceStates
	}
}

func parseDeviceStates(msg string) (states map[string]DeviceState, err error) {
	states = make(map[string]DeviceState)

	for lineNum, line := range strings.Split(msg, "\n") {
		if len(line) == 0 {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) != 2 {
			err = errors.Errorf(errors.ParseError, "invalid device state line %d: %s", lineNum, line)
			return
		}

		serial, stateString := fields[0], fields[1]
		var state DeviceState
		state, err = parseDeviceState(stateString)
		states[serial] = state
	}

	return
}

func calculateStateDiffs(oldStates, newStates map[string]DeviceState) (events []DeviceStateChangedEvent) {
	for serial, oldState := range oldStates {
		newState, ok := newStates[serial]

		if oldState != newState {
			if ok {
				// Device present in both lists: state changed.
				events = append(events, DeviceStateChangedEvent{serial, oldState, newState})
			} else {
				// Device only present in old list: device removed.
				events = append(events, DeviceStateChangedEvent{serial, oldState, StateDisconnected})
			}
		}
	}

	for serial, newState := range newStates {
		if _, ok := oldStates[serial]; !ok {
			// Device only present in new list: device added.
			events = append(events, DeviceStateChangedEvent{serial, StateDisconnected, newState})
		}
	}

	return events
}
