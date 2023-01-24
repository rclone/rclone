package estuary

import (
	"time"
)

type userSettings struct {
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

type collectionCreate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type deleteContentFromCollectionBody struct {
	By    string `json:"by"`
	Value string `json:"value"`
}

type contentAdd struct {
	ID      uint   `json:"estuaryID"`
	Cid     string `json:"cid,omitempty"`
	Error   string `json:"error"`
	Details string `json:"details"`
}

type ipfsPin struct {
	CID     string                 `json:"cid"`
	Name    string                 `json:"name"`
	Origins []string               `json:"origins"`
	Meta    map[string]interface{} `json:"meta"`
}

type ipfsPinStatusResponse struct {
	RequestID string                 `json:"requestid"`
	Status    string                 `json:"status"`
	Created   time.Time              `json:"created"`
	Delegates []string               `json:"delegates"`
	Info      map[string]interface{} `json:"info"`
	Pin       ipfsPin                `json:"pin"`
}

type collection struct {
	UUID        string    `json:"uuid"`
	CreatedAt   time.Time `json:"createdAt"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UserID      uint      `json:"userId"`
}
