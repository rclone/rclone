package proton

// Volume is a Proton Drive volume.
type Volume struct {
	VolumeID string // Encrypted volume ID

	CreationTime    int64  // Creation time of the volume in Unix time
	ModifyTime      int64  // Last modification time of the volume in Unix time
	MaxSpace        *int64 // Space limit for the volume in bytes, null if unlimited.
	UsedSpace       int64  // Space used by files in the volume in bytes
	DownloadedBytes int64  // The amount of downloaded data since last reset
	UploadedBytes   int64  // The amount of uploaded data since the last reset

	State         VolumeState          // The state of the volume (active, locked, maybe more in the future)
	Share         VolumeShare          // The main share of the volume
	RestoreStatus *VolumeRestoreStatus // The status of the restore task. Null if not applicable
}

// VolumeShare is the main share of a volume.
type VolumeShare struct {
	ShareID string // Encrypted share ID
	LinkID  string // Encrypted link ID
}

// VolumeState is the state of a volume.
type VolumeState int

const (
	VolumeStateActive VolumeState = 1
	VolumeStateLocked VolumeState = 3
)

// VolumeRestoreStatus is the status of the restore task.
type VolumeRestoreStatus int

const (
	RestoreStatusDone       VolumeRestoreStatus = 0
	RestoreStatusInProgress VolumeRestoreStatus = 1
	RestoreStatusFailed     VolumeRestoreStatus = -1
)
