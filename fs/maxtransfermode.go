package fs

// MaxTransferMode describes the possible delete modes in the config
type MaxTransferMode byte

// MaxTransferMode constants
const (
	MaxTransferModeHard MaxTransferMode = iota
	MaxTransferModeSoft
	MaxTransferModeCautious
	MaxTransferModeDefault = MaxTransferModeHard
)
