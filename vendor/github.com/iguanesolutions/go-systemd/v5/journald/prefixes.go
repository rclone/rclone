package sysdjournald

const (
	// EmergPrefix is the string to prefix in Emergency for systemd-journald
	EmergPrefix = "<0>"
	// AlertPrefix is the string to prefix in Alert for systemd-journald
	AlertPrefix = "<1>"
	// CritPrefix is the string to prefix in Critical for systemd-journald
	CritPrefix = "<2>"
	// ErrPrefix is the string to prefix in Error for systemd-journald
	ErrPrefix = "<3>"
	// WarningPrefix is the string to prefix in Warning for systemd-journald
	WarningPrefix = "<4>"
	// NoticePrefix is the string to prefix in Notice for systemd-journald
	NoticePrefix = "<5>"
	// InfoPrefix is the string to prefix in Info for systemd-journald
	InfoPrefix = "<6>"
	// DebugPrefix is the string to prefix in Debug for systemd-journald
	DebugPrefix = "<7>"
)
