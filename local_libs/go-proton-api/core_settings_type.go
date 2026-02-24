package proton

type UserSettings struct {
	Telemetry    SettingsBool
	CrashReports SettingsBool
}

type SetTelemetryReq struct {
	Telemetry SettingsBool
}

type SetCrashReportReq struct {
	CrashReports SettingsBool
}
type SettingsBool int

const (
	SettingDisabled SettingsBool = iota
	SettingEnabled
)
