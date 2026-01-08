package api

import (
	"fmt"
)

var (
	errorsDescription = map[int]string{
		// from API
		2:      "Required parameters are missing.",
		105:    "External link error",
		100001: "The client_id or client_secret parameter is invalid.",
		100002: "The code is invalid (invalid or expired code).",
		200001: "Unsupported authorization type grant_type.",
		200002: "Invalid access_token.",
		200003: "The access_token has expired.",
		200004: "Invalid refresh_token.",
		200005: "The refresh_token has expired.",
		300001: "The frequency of exchanging the code for the access_token is too high.",
		400001: "The user has not yet completed the authorization operation for the device_code (the error code of the device code mode).",
		500001: "Internal service exception.",
		-7:     "Invalid file name",
		-8:     "The file already exists",
		-9:     "The file doesn't exist or request parameter spd is incorrect.",
		-12:    "Error in extraction code",

		// from web client
		-1: "User name or password verification failed",
		-2: "Back up",
		-4: "Unknown error",
		-5: "Unknown error",
		-6: "Failed to login, please try again later",
		// -7: "Unable to access or the name is wrong",
		// -8: "The file already exists in this directory ",
		// -9:      "The file doesn't exist or The request parameter spd is incorrect.",
		-10: "Your space is insufficient",
		-11: "The parent directory does not exist",
		// -12:     "Error in extraction code",
		-14: "The account has been initialized",
		-13: "The device has been bonded",
		-19: "Please enter the verification code",
		-21: "Cannot operate preference files",
		-22: "Shared files cannot be renamed or moved",
		-23: "Failed to operate database, please contact the administrator",
		-24: "The files to cancel contain some that are not allowed to cancel",
		-25: "Not beta user",
		-26: "Invalid invitation code",
		-32: "Your space is insufficient",
		1:   "System Error",
		// 2: "Server Error, please try again later",
		3:     "No more than 100 files at a time",
		4:     "New file name error ",
		5:     "Illegal target directory",
		7:     "Illegal NS or no access",
		8:     "Illegal ID or no access",
		9:     "Failed to apply for the key",
		10:    "Unsuccessful superfile",
		11:    "Illegal user ID (or user name) or no access ",
		12:    "Some files already exist in target directory",
		15:    "Operation failed",
		58:    "Size of upload file more than allowed free plan limit (4GB)", // original message Upload single file vip limit
		102:   "Unable to access the directory",
		103:   "Incorrect password",
		104:   "Invalid cookie",
		111:   "You currently have unfinished tasks, please operate after completion",
		121:   "The number of files exceeds the limit, please delete your files until the number is below 5 million",
		132:   "Verify your identity to operate the files",
		141:   "Internal Error",
		142:   "You have been removed from this Shared Folder, thus you cannot continue",
		301:   "Other request error",
		501:   "Illegal format of the LIST",
		618:   "Failed request",
		619:   "PCS returns an error code",
		600:   "json error",
		601:   "Incorrect exception",
		617:   "Other error",
		211:   "Unable to access or being banned",
		407:   "Internal error",
		31080: "Server error, please try again later",
		31021: "Unable to access network, please check the network or try again later",
		31075: "No more than 999 files at a time, please reduce the number",
		31116: "Your space is insufficient",
		31401: "The selected file will be canceled from the shared folder after it is moved, and the members cannot view it. Are you sure to continue?",
		31034: "The frequency of operation is too soon, please try again later",
		36009: "Insufficient user space",
		36010: "The file doesn't exist",
		36012: "Operation timed out, please try again later",
		36013: "Cannot download, too many tasks are downloaded at the same time",
		36014: "The storage path has been used",
		36016: "Task deleted",
		36017: "Task completed",
		36019: "The task is being processed",
		36018: "Failed to resolve，the torrent file is corrupted",
		36020: "The task address doesn't exist",
		36021: "A normal user can download 1 task at a time! Y You can download more by subscribing the offline download package",
		36023: "A normal user can complete 5 offline downloading tasks only per month! You can download more by subscribing the offline download package",
		36022: "Cannot download, too many tasks are downloaded at the same time",
		36024: "The number of downloads in this month has reached the limit",
		36025: "Expired link",
		36026: "Link format error",
		36028: "Unable to access relevant information",
		36027: "Link format error",
		36031: "Network busy, please try again later",
		36001: "Network busy, please try again later",
		36032: "Cannot download the offline files because they contain illegal contents",
		9000:  "TeraBox is not yet available in vm area",
		36038: "Download is unavailable as requested by the copyright owner",
		9001:  "A Server Error has occurred, please try again later",
		9002:  "The frequency of complaint is too fast, please try again later",
		10019: "Format Error",
		10013: "You currently have unfinished tasks, please operate after completion",
		10014: "No importable credentials detected",

		// gotted with message
		31208: "content_type is not exists",
		31299: "Size of superfile2 first part should not be smaller than 4MB",

		// third party [unofficial]
		4000023: "need verify (jsToken expired)",
		450016:  "need verify",
	}
)

// Num2Err convert error number to error presence
func Num2Err(number int) error {
	if number == 0 {
		return nil
	}

	return ErrorAPI{number}
}

// ErrIsNum checking is provided error is Terabox error. Can be a multiple error codes for one error
func ErrIsNum(err error, numbers ...int) bool {
	if e, ok := err.(ErrorInterface); ok {
		for _, number := range numbers {
			if e.ErrorNumber() == number {
				return true
			}
		}
	}
	return false
}

var _ ErrorInterface = ResponseDefault{}

// ErrorInterface universal Error Interface
type ErrorInterface interface {
	ErrorNumber() int
	Error() string
	Err() error
}

// ErrorAPI - Terabox API error
type ErrorAPI struct {
	ErrNumber int `json:"errno"`
}

// ErrorNumber return Terabox error number
func (err ErrorAPI) ErrorNumber() int {
	return err.ErrNumber
}

// Error is Error Interface implementation
func (err ErrorAPI) Error() string {
	if _, ok := errorsDescription[err.ErrNumber]; ok {
		return errorsDescription[err.ErrNumber]
	}

	return fmt.Sprintf("Unknown error %d", err.ErrNumber)
}

// Err return the error if number of error not 0, otherwise nil
func (err ErrorAPI) Err() error {
	if err.ErrNumber == 0 {
		return nil
	}

	return err
}
