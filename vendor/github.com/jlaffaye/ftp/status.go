package ftp

import "fmt"

// FTP status codes, defined in RFC 959
const (
	StatusInitiating    = 100
	StatusRestartMarker = 110
	StatusReadyMinute   = 120
	StatusAlreadyOpen   = 125
	StatusAboutToSend   = 150

	StatusCommandOK             = 200
	StatusCommandNotImplemented = 202
	StatusSystem                = 211
	StatusDirectory             = 212
	StatusFile                  = 213
	StatusHelp                  = 214
	StatusName                  = 215
	StatusReady                 = 220
	StatusClosing               = 221
	StatusDataConnectionOpen    = 225
	StatusClosingDataConnection = 226
	StatusPassiveMode           = 227
	StatusLongPassiveMode       = 228
	StatusExtendedPassiveMode   = 229
	StatusLoggedIn              = 230
	StatusLoggedOut             = 231
	StatusLogoutAck             = 232
	StatusAuthOK                = 234
	StatusRequestedFileActionOK = 250
	StatusPathCreated           = 257

	StatusUserOK             = 331
	StatusLoginNeedAccount   = 332
	StatusRequestFilePending = 350

	StatusNotAvailable             = 421
	StatusCanNotOpenDataConnection = 425
	StatusTransfertAborted         = 426
	StatusInvalidCredentials       = 430
	StatusHostUnavailable          = 434
	StatusFileActionIgnored        = 450
	StatusActionAborted            = 451
	Status452                      = 452

	StatusBadCommand              = 500
	StatusBadArguments            = 501
	StatusNotImplemented          = 502
	StatusBadSequence             = 503
	StatusNotImplementedParameter = 504
	StatusNotLoggedIn             = 530
	StatusStorNeedAccount         = 532
	StatusFileUnavailable         = 550
	StatusPageTypeUnknown         = 551
	StatusExceededStorage         = 552
	StatusBadFileName             = 553
)

var statusText = map[int]string{
	// 200
	StatusCommandOK:             "Command okay.",
	StatusCommandNotImplemented: "Command not implemented, superfluous at this site.",
	StatusSystem:                "System status, or system help reply.",
	StatusDirectory:             "Directory status.",
	StatusFile:                  "File status.",
	StatusHelp:                  "Help message.",
	StatusName:                  "",
	StatusReady:                 "Service ready for new user.",
	StatusClosing:               "Service closing control connection.",
	StatusDataConnectionOpen:    "Data connection open; no transfer in progress.",
	StatusClosingDataConnection: "Closing data connection. Requested file action successful.",
	StatusPassiveMode:           "Entering Passive Mode.",
	StatusLongPassiveMode:       "Entering Long Passive Mode.",
	StatusExtendedPassiveMode:   "Entering Extended Passive Mode.",
	StatusLoggedIn:              "User logged in, proceed.",
	StatusLoggedOut:             "User logged out; service terminated.",
	StatusLogoutAck:             "Logout command noted, will complete when transfer done.",
	StatusAuthOK:                "AUTH command OK",
	StatusRequestedFileActionOK: "Requested file action okay, completed.",
	StatusPathCreated:           "Path created.",

	// 300
	StatusUserOK:             "User name okay, need password.",
	StatusLoginNeedAccount:   "Need account for login.",
	StatusRequestFilePending: "Requested file action pending further information.",

	// 400
	StatusNotAvailable:             "Service not available, closing control connection.",
	StatusCanNotOpenDataConnection: "Can't open data connection.",
	StatusTransfertAborted:         "Connection closed; transfer aborted.",
	StatusInvalidCredentials:       "Invalid username or password.",
	StatusHostUnavailable:          "Requested host unavailable.",
	StatusFileActionIgnored:        "Requested file action not taken.",
	StatusActionAborted:            "Requested action aborted. Local error in processing.",
	Status452:                      "Insufficient storage space in system.",

	// 500
	StatusBadCommand:              "Command unrecognized.",
	StatusBadArguments:            "Syntax error in parameters or arguments.",
	StatusNotImplemented:          "Command not implemented.",
	StatusBadSequence:             "Bad sequence of commands.",
	StatusNotImplementedParameter: "Command not implemented for that parameter.",
	StatusNotLoggedIn:             "Not logged in.",
	StatusStorNeedAccount:         "Need account for storing files.",
	StatusFileUnavailable:         "File unavailable.",
	StatusPageTypeUnknown:         "Page type unknown.",
	StatusExceededStorage:         "Exceeded storage allocation.",
	StatusBadFileName:             "File name not allowed.",
}

// StatusText returns a text for the FTP status code. It returns the empty string if the code is unknown.
func StatusText(code int) string {
	str, ok := statusText[code]
	if !ok {
		str = fmt.Sprintf("Unknown status code: %d", code)
	}
	return str
}
