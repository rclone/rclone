package estuary

import (
	"github.com/ipfs/go-cid"
	"time"
)

type UserSettings struct {
	Replication           int           `json:"replication"`
	Verified              bool          `json:"verified"`
	DealDuration          int           `json:"dealDuration"`
	MaxStagingWait        time.Duration `json:"maxStagingWait"`
	FileStagingThreshold  int64         `json:"fileStagingThreshold"`
	ContentAddingDisabled bool          `json:"contentAddingDisabled"`
	DealMakingDisabled    bool          `json:"dealMakingDisabled"`
	UploadEndpoints       []string      `json:"uploadEndpoints"`
	Flags                 int           `json:"flags"`
}

type ViewerResponse struct {
	Username   string       `json:"username"`
	Perms      int          `json:"perms"`
	ID         uint         `json:"id"`
	Address    string       `json:"address,omitempty"`
	Miners     []string     `json:"miners,omitempty"`
	AuthExpiry time.Time    `json:"auth_expiry,omitempty"`
	Settings   UserSettings `json:"settings"`
}

type DbCID struct {
	CID cid.Cid
}
