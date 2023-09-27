package fs

type cutoffModeChoices struct{}

func (cutoffModeChoices) Choices() []string {
	return []string{
		CutoffModeHard:     "HARD",
		CutoffModeSoft:     "SOFT",
		CutoffModeCautious: "CAUTIOUS",
	}
}

// CutoffMode describes the possible delete modes in the config
type CutoffMode = Enum[cutoffModeChoices]

// CutoffMode constants
const (
	CutoffModeHard CutoffMode = iota
	CutoffModeSoft
	CutoffModeCautious
	CutoffModeDefault = CutoffModeHard
)
