package fs

// UseSnapshotMode describes when backend-specific point-in-time snapshots should be used when copying
type UseSnapshotMode = Enum[useSnapshotModeChoices]

// UseSnapshotMode constants
const (
	UseSnapshotModeNever UseSnapshotMode = iota
	UseSnapshotModeAttempt
	UseSnapshotModeAlways
)

type useSnapshotModeChoices struct{}

func (useSnapshotModeChoices) Choices() []string {
	return []string{
		UseSnapshotModeNever:   "NEVER",
		UseSnapshotModeAttempt: "ATTEMPT",
		UseSnapshotModeAlways:  "ALWAYS",
	}
}
