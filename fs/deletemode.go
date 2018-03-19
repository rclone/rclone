package fs

// DeleteMode describes the possible delete modes in the config
type DeleteMode byte

// DeleteMode constants
const (
	DeleteModeOff DeleteMode = iota
	DeleteModeBefore
	DeleteModeDuring
	DeleteModeAfter
	DeleteModeOnly
	DeleteModeDefault = DeleteModeAfter
)
