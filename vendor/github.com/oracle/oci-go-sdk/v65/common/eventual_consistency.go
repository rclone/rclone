// Copyright (c) 2016, 2018, 2023, Oracle and/or its affiliates.  All rights reserved.
// This software is dual-licensed to you under the Universal Permissive License (UPL) 1.0 as shown at https://oss.oracle.com/licenses/upl or Apache License 2.0 as shown at http://www.apache.org/licenses/LICENSE-2.0. You may choose either license.

package common

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofrs/flock"
)

const (
	// OciGoSdkEcConfigEnvVarName contains the name of the environment variable that can be used to configure the eventual consistency (EC) communication mode.
	// Allowed values for environment variable:
	// 1. OCI_GO_SDK_EC_CONFIG = "file,/path/to/shared/timestamp/file"
	// 2. OCI_GO_SDK_EC_CONFIG = "inprocess"
	// 3. absent -- same as OCI_GO_SDK_EC_CONFIG = "inprocess"
	OciGoSdkEcConfigEnvVarName string = "OCI_GO_SDK_EC_CONFIG"
)

//
// Eventual consistency communication mode
//

// EcMode is the eventual consistency (EC) communication mode used.
type EcMode int64

const (
	// Uninitialized means the EC communication mode has not been set yet.
	Uninitialized EcMode = iota // 0

	// InProcess is the default EC communication mode which only communicates the end-of-window timestamp inside the same process.
	InProcess

	// File is the EC communication mode that uses a file to communicate the end-of-window timestamp using a file visible across processes.
	// Locking is performed using a lock file.
	File
)

var (
	affectedByEventualConsistencyRetryStatusCodeMap = map[StatErrCode]bool{
		{400, "RelatedResourceNotAuthorizedOrNotFound"}: true,
		{404, "NotAuthorizedOrNotFound"}:                true,
		{409, "NotAuthorizedOrResourceAlreadyExists"}:   true,
		{400, "InsufficientServicePermissions"}:         true,
		{400, "ResourceDisabled"}:                       true,
	}
)

// IsErrorAffectedByEventualConsistency returns true if the error is affected by eventual consistency.
func IsErrorAffectedByEventualConsistency(Error error) bool {
	if err, ok := IsServiceError(Error); ok {
		return affectedByEventualConsistencyRetryStatusCodeMap[StatErrCode{err.GetHTTPStatusCode(), err.GetCode()}]
	}
	return false
}

func getEcMode(mode string) EcMode {
	var lmode = strings.ToLower(mode)
	switch lmode {
	case "file":
		return File
	case "inprocess":
		return InProcess
	}
	ecLogf("%s: Unknown ec mode '%s', assuming 'inprocess'", OciGoSdkEcConfigEnvVarName, mode)
	return InProcess
}

// EventuallyConsistentContext contains the information about the end of the eventually consistent window.
type EventuallyConsistentContext struct {
	// memory-based
	endOfWindow     atomic.Value
	lock            sync.RWMutex
	timeNowProvider func() time.Time

	// mode selector
	ecMode EcMode

	// file-based

	// timestampFileName and timestampLockFile should be set to files that
	// are accessible by all processes that need to share information about
	// eventual consistency.
	// A sensible choice are files inside the temp directory, as returned by os.TempDir()
	timestampFileName *string
	timestampFileLock *flock.Flock

	// lock and unlock functions
	readLock    func(e *EventuallyConsistentContext) error
	readUnlock  func(e *EventuallyConsistentContext) error
	writeLock   func(e *EventuallyConsistentContext) error
	writeUnlock func(e *EventuallyConsistentContext) error

	// get/set functions
	getEndOfWindowUnsynchronized func(e *EventuallyConsistentContext) (*time.Time, error)
	setEndOfWindowUnsynchronized func(e *EventuallyConsistentContext, newEndOfWindowTime *time.Time) error
}

// newEcContext creates a new EC context based on the OCI_GO_SDK_EC_CONFIG environment variable.
func newEcContext() *EventuallyConsistentContext {
	ecConfig, ecConfigProvided := os.LookupEnv(OciGoSdkEcConfigEnvVarName)
	if !ecConfigProvided {
		ecConfig = ""
	}

	commaIndex := strings.Index(ecConfig, ",")
	var ecConfigMode = ecConfig
	var ecConfigRest = ""
	if commaIndex >= 0 {
		ecConfigMode = ecConfig[:commaIndex]
		ecConfigRest = ecConfig[commaIndex+1:]
	}
	ecMode := getEcMode(ecConfigMode)

	switch ecMode {
	case File:
		if len(ecConfigRest) < 1 {
			ecLogf("%s: Expected file name after comma for 'File' mode ('file,/path/to/file'), was: '%s'", OciGoSdkEcConfigEnvVarName, ecConfig)
			return nil
		}
		return newEcContextFile(ecConfigRest)
	}

	return newEcContextInProcess()
}

// newEcContextInProcess creates a new in-process EC context.
func newEcContextInProcess() *EventuallyConsistentContext {
	ecContext := EventuallyConsistentContext{
		ecMode:                       InProcess,
		readLock:                     ecInProcessReadLock,
		readUnlock:                   ecInProcessReadUnlock,
		writeLock:                    ecInProcessWriteLock,
		writeUnlock:                  ecInProcessWriteUnlock,
		getEndOfWindowUnsynchronized: ecInProcessGetEndOfWindowUnsynchronized,
		setEndOfWindowUnsynchronized: ecInProcessSetEndOfWindowUnsynchronized,
		timeNowProvider:              func() time.Time { return time.Now() },
	}
	return &ecContext
}

// newEcContextFile creates a new EC context kept in a file.
// timestampFileName should be set to a file accessible by all processes that
// need to share information about eventual consistency.
// A sensible choice are files inside the temp directory, as returned by os.TempDir()
// The lock file will use the same name, with the suffix ".lock" added.
func newEcContextFile(timestampFileName string) *EventuallyConsistentContext {
	timestampLockFileName := timestampFileName + ".lock"
	ecContext := EventuallyConsistentContext{
		ecMode:                       File,
		readLock:                     ecFileReadLock,
		readUnlock:                   ecFileReadUnlock,
		writeLock:                    ecFileWriteLock,
		writeUnlock:                  ecFileWriteUnlock,
		getEndOfWindowUnsynchronized: ecFileGetEndOfWindowUnsynchronized,
		setEndOfWindowUnsynchronized: ecFileSetEndOfWindowUnsynchronized,
		timeNowProvider:              func() time.Time { return time.Now() },
		timestampFileName:            &timestampFileName,
		timestampFileLock:            flock.New(timestampLockFileName),
	}
	ecDebugf("%s: Using file modification time of file '%s' and lock file '%s'", OciGoSdkEcConfigEnvVarName, *ecContext.timestampFileName, timestampLockFileName)
	return &ecContext
}

// InitializeEcContextFromEnvVar initializes the EcContext variable as configured
// in the OCI_GO_SDK_EC_CONFIG environment variable.
func InitializeEcContextFromEnvVar() {
	EcContext = newEcContext()
}

// InitializeEcContextInProcess initializes the EcContext variable to be in-process only.
func InitializeEcContextInProcess() {
	EcContext = newEcContextInProcess()
}

// InitializeEcContextFile initializes the EcContext variable to be kept in a timestamp file,
// protected by a lock file.
// timestampFileName should be set to a file accessible by all processes that
// need to share information about eventual consistency.
// A sensible choice are files inside the temp directory, as returned by os.TempDir()
// The lock file will use the same name, with the suffix ".lock" added.
func InitializeEcContextFile(timestampFileName string) {
	EcContext = newEcContextFile(timestampFileName)
}

//
// InProcess functions
//

func ecInProcessReadLock(e *EventuallyConsistentContext) error {
	e.lock.RLock()
	return nil
}

func ecInProcessReadUnlock(e *EventuallyConsistentContext) error {
	e.lock.RUnlock()
	return nil
}

func ecInProcessWriteLock(e *EventuallyConsistentContext) error {
	e.lock.Lock()
	return nil
}

func ecInProcessWriteUnlock(e *EventuallyConsistentContext) error {
	e.lock.Unlock()
	return nil
}

// ecInProcessGetEndOfWindowUnsynchronized returns the end time of an eventually consistent window,
// or nil if no eventually consistent requests were made.
// There is no mutex synchronization.
func ecInProcessGetEndOfWindowUnsynchronized(e *EventuallyConsistentContext) (*time.Time, error) {
	untyped := e.endOfWindow.Load() // returns nil if there has been no call to Store for this Value
	if untyped == nil {
		return (*time.Time)(nil), nil
	}
	t := untyped.(*time.Time)

	return t, nil
}

// ecInProcessSetEndOfWindowUnsynchronized sets the end time of the eventually consistent window.
// There is no mutex synchronization.
func ecInProcessSetEndOfWindowUnsynchronized(e *EventuallyConsistentContext, newEndOfWindowTime *time.Time) error {
	e.endOfWindow.Store(newEndOfWindowTime) // atomically replace the current object with the new one
	return nil
}

//
// File functions
//

func ecFileReadLock(e *EventuallyConsistentContext) error {
	return e.timestampFileLock.RLock()
}

func ecFileReadUnlock(e *EventuallyConsistentContext) error {
	return e.timestampFileLock.Unlock()
}

func ecFileWriteLock(e *EventuallyConsistentContext) error {
	return e.timestampFileLock.Lock()
}

func ecFileWriteUnlock(e *EventuallyConsistentContext) error {
	return e.timestampFileLock.Unlock()
}

// ecFileGetEndOfWindowUnsynchronized returns the end time of an eventually consistent window,
// or nil if no eventually consistent requests were made.
// There is no mutex synchronization.
func ecFileGetEndOfWindowUnsynchronized(e *EventuallyConsistentContext) (*time.Time, error) {
	file, err := os.Stat(*e.timestampFileName)

	if errors.Is(err, os.ErrNotExist) {
		ecDebugf("%s: File '%s' does not exist, meaning no EC in effect", OciGoSdkEcConfigEnvVarName, *e.timestampFileName)
		return (*time.Time)(nil), nil
	}
	if err != nil {
		ecLogf("%s: Error getting modified time from file '%s', assuming no EC in effect: %s", OciGoSdkEcConfigEnvVarName, *e.timestampFileName, err)
		return (*time.Time)(nil), err
	}

	t := file.ModTime()
	ecDebugf("%s: Read modified time of file '%s' as '%s'", OciGoSdkEcConfigEnvVarName, *e.timestampFileName, t)

	return &t, nil
}

// ecFileSetEndOfWindowUnsynchronized sets the end time of the eventually consistent window.
// There is no mutex synchronization.
func ecFileSetEndOfWindowUnsynchronized(e *EventuallyConsistentContext, newEndOfWindowTime *time.Time) error {
	if newEndOfWindowTime != nil {
		ecDebugf("%s: Updating modified time of file '%s' to '%s'", OciGoSdkEcConfigEnvVarName, *e.timestampFileName, *newEndOfWindowTime)
	} else {
		ecDebugf("%s: Updating modified time of file '%s' to <nil>", OciGoSdkEcConfigEnvVarName, *e.timestampFileName)
	}

	if newEndOfWindowTime == nil {
		err := os.Remove(*e.timestampFileName)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			ecLogf("%s: Error removing file '%s', may draw wrong EC conflusions: %s", OciGoSdkEcConfigEnvVarName, *e.timestampFileName, err)
		}
		return err
	}

	atime := time.Now()
	var err = os.Chtimes(*e.timestampFileName, atime, *newEndOfWindowTime)
	if errors.Is(err, os.ErrNotExist) {
		_, createErr := os.Create(*e.timestampFileName)
		if createErr != nil {
			ecLogf("%s: Error creating file '%s', will have to assume no EC in effect: %s", OciGoSdkEcConfigEnvVarName, *e.timestampFileName, createErr)
			return createErr
		}
		err = os.Chtimes(*e.timestampFileName, atime, *newEndOfWindowTime)
	}
	if err != nil {
		ecLogf("%s: Error changing modified time for file '%s', will have to assume no EC in effect: %s", OciGoSdkEcConfigEnvVarName, *e.timestampFileName, err)
		return err
	}
	return nil
}

//
// General functions for EC window handling, for all EC communication modes
//

// GetEndOfWindow returns the end time an eventually consistent window,
// or nil if no eventually consistent requests were made
func (e *EventuallyConsistentContext) GetEndOfWindow() *time.Time {
	e.readLock(e) // synchronize with potential writers
	defer e.readUnlock(e)

	endOfWindowTime, _ := e.getEndOfWindowUnsynchronized(e)

	// TODO: this is noisy logging, consider removing
	if endOfWindowTime != nil {
		ecDebugln(fmt.Sprintf("EcContext.GetEndOfWindow returns %s", endOfWindowTime))
	} else {
		ecDebugln("EcContext.GetEndOfWindow returns <nil>")
	}

	return endOfWindowTime
}

// UpdateEndOfWindow sets the end time of the eventually consistent window the specified
// duration into the future
func (e *EventuallyConsistentContext) UpdateEndOfWindow(windowSize time.Duration) *time.Time {
	e.writeLock(e) // synchronize with other potential writers
	defer e.writeUnlock(e)

	currentEndOfWindowTime, _ := e.getEndOfWindowUnsynchronized(e)
	var newEndOfWindowTime = e.timeNowProvider().Add(windowSize)
	if currentEndOfWindowTime == nil || newEndOfWindowTime.After(*currentEndOfWindowTime) {
		e.setEndOfWindowUnsynchronized(e, &newEndOfWindowTime)

		// TODO: this is noisy logging, consider removing
		ecDebugln(fmt.Sprintf("EcContext.UpdateEndOfWindow to %s", newEndOfWindowTime))

		return &newEndOfWindowTime
	}
	return currentEndOfWindowTime
}

// setEndTimeOfEventuallyConsistentWindow sets the last time an eventually consistent request was made
// to the specified time
func (e *EventuallyConsistentContext) setEndOfWindow(newTime *time.Time) *time.Time {
	e.writeLock(e) // synchronize with other potential writers
	defer e.writeUnlock(e)

	e.setEndOfWindowUnsynchronized(e, newTime)

	// TODO: this is noisy logging, consider removing
	if newTime != nil {
		ecDebugln(fmt.Sprintf("EcContext.setEndOfWindow to %s", *newTime))
	} else {
		ecDebugln("EcContext.setEndOfWindow to <nil>")
	}

	return newTime
}

// EcContext contains the information about the end of the eventually consistent window for this process.
var EcContext = newEcContext()

//
// Logging helpers
//

// getGID returns the Goroutine id. This is purely for logging and debugging.
// See https://blog.sgmansfield.com/2015/12/goroutine-ids/
func getGID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

// some of these errors happen so early, defaultLogger may not have been
// initialized yet.
func initLogIfNecessary() {
	if defaultLogger == nil {
		l, _ := NewSDKLogger()
		SetSDKLogger(l)
	}
}

// Debugf logs v with the provided format if debug mode is set.
// There is no mutex synchronization. You should have acquired e.lock first.
func ecDebugf(format string, v ...interface{}) {
	defer func() {
		// recover from panic if one occured.
		if recover() != nil {
			Debugln("ecDebugf failed")
		}
	}()

	str := fmt.Sprintf(format, v...)

	initLogIfNecessary()

	// prefix message with "(pid=25140, gid=5)"
	Debugf("(pid=%d, gid=%d) %s", os.Getpid(), getGID(), str)
}

// Debug logs v if debug mode is set.
// There is no mutex synchronization. You should have acquired e.lock first.
func ecDebug(v ...interface{}) {
	defer func() {
		// recover from panic if one occured.
		if recover() != nil {
			Debugln("ecDebug failed")
		}
	}()

	initLogIfNecessary()

	// prefix message with "(pid=25140, gid=5)"
	Debug(append([]interface{}{"(pid=", os.Getpid(), ", gid=", getGID(), ") "}, v...)...)
}

// Debugln logs v appending a new line if debug mode is set
// There is no mutex synchronization. You should have acquired e.lock first.
func ecDebugln(v ...interface{}) {
	defer func() {
		// recover from panic if one occured.
		if recover() != nil {
			Debugln("ecDebugln failed")
		}
	}()

	initLogIfNecessary()

	// prefix message with "(pid=25140, gid=5)"
	Debugln(append([]interface{}{"(pid=", os.Getpid(), ", gid=", getGID(), ") "}, v...)...)
}

// Logf logs v with the provided format if info mode is set.
// There is no mutex synchronization. You should have acquired e.lock first.
func ecLogf(format string, v ...interface{}) {
	defer func() {
		// recover from panic if one occured.
		if recover() != nil {
			Debugln("ecLogf failed")
		}
	}()

	initLogIfNecessary()

	str := fmt.Sprintf(format, v...)
	// prefix message with "(pid=25140, gid=5)"
	Logf("(pid=%d, gid=%d) %s", os.Getpid(), getGID(), str)
}
