// Package api contains definitions for using the alldebrid API
package api

import "fmt"

// Response is returned by all messages and embedded in the
// structures below
type Response struct {
	Status string `json:"status"`
	Error  *Error `json:"error,omitempty"`
	Data   any    `json:"data,omitempty"`
}

// Error represents an API error response
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AsErr checks the status and returns an err if bad or nil if good
func (r *Response) AsErr() error {
	if r.Status != "success" && r.Error != nil {
		return r.Error
	}
	return nil
}

// Error satisfies the error interface
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// UserResponse represents the response from /v4/user
type UserResponse struct {
	Response
	Data struct {
		User User `json:"user"`
	} `json:"data"`
}

// User represents user information
type User struct {
	Username             string         `json:"username"`
	Email                string         `json:"email"`
	IsPremium            bool           `json:"isPremium"`
	IsSubscribed         bool           `json:"isSubscribed"`
	IsTrial              bool           `json:"isTrial"`
	PremiumUntil         int64          `json:"premiumUntil"`
	Lang                 string         `json:"lang"`
	PreferedDomain       string         `json:"preferedDomain"`
	FidelityPoints       int            `json:"fidelityPoints"`
	LimitedHostersQuotas map[string]int `json:"limitedHostersQuotas"`
	Notifications        []string       `json:"notifications"`
	RemainingTrialQuota  *int           `json:"remainingTrialQuota,omitempty"`
}

// LinksResponse represents the response from /v4/user/links
type LinksResponse struct {
	Response
	Data struct {
		Links []Link `json:"links"`
	} `json:"data"`
}

// HistoryResponse represents the response from /v4/user/history
type HistoryResponse struct {
	Response
	Data struct {
		Links []Link `json:"links"`
	} `json:"data"`
}

// Link represents a saved or history link
type Link struct {
	Link     string `json:"link"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Date     int64  `json:"date"`
	Host     string `json:"host"`
}

// MagnetStatusResponse represents the response from /v4.1/magnet/status
type MagnetStatusResponse struct {
	Response
	Data struct {
		Magnets  []Magnet `json:"magnets"`
		Counter  int      `json:"counter,omitempty"`
		Fullsync bool     `json:"fullsync,omitempty"`
	} `json:"data"`
}

// Magnet represents a magnet download
type Magnet struct {
	ID             int    `json:"id"`
	Filename       string `json:"filename"`
	Size           int64  `json:"size"`
	Status         string `json:"status"`
	StatusCode     int    `json:"statusCode"`
	Downloaded     int64  `json:"downloaded"`
	Uploaded       int64  `json:"uploaded"`
	Seeders        int    `json:"seeders"`
	DownloadSpeed  int64  `json:"downloadSpeed"`
	UploadSpeed    int64  `json:"uploadSpeed"`
	UploadDate     int64  `json:"uploadDate"`
	CompletionDate int64  `json:"completionDate"`
	NBLinks        int    `json:"nbLinks"`
}

// MagnetFile represents a file within a magnet
type MagnetFile struct {
	Name    string       `json:"n"`
	Size    int64        `json:"s,omitempty"`
	Link    string       `json:"l,omitempty"`
	Entries []MagnetFile `json:"e,omitempty"`
}

// MagnetFilesResponse represents the response from /v4/magnet/files
type MagnetFilesResponse struct {
	Response
	Data struct {
		Magnets []MagnetFiles `json:"magnets"`
	} `json:"data"`
}

// MagnetFiles represents files for a specific magnet
type MagnetFiles struct {
	ID    string       `json:"id"`
	Files []MagnetFile `json:"files"`
	Error *Error       `json:"error,omitempty"`
}

// MagnetUploadResponse represents the response from /v4/magnet/upload
type MagnetUploadResponse struct {
	Response
	Data struct {
		Magnets []MagnetUpload `json:"magnets"`
	} `json:"data"`
}

// MagnetUpload represents the result of uploading a magnet
type MagnetUpload struct {
	Magnet string `json:"magnet"`
	Hash   string `json:"hash,omitempty"`
	Name   string `json:"name,omitempty"`
	Size   int64  `json:"size,omitempty"`
	Ready  bool   `json:"ready,omitempty"`
	ID     int    `json:"id,omitempty"`
	Error  *Error `json:"error,omitempty"`
}

// MagnetUploadFileResponse represents the response from /v4/magnet/upload/file
type MagnetUploadFileResponse struct {
	Response
	Data struct {
		Files []MagnetUploadFile `json:"files"`
	} `json:"data"`
}

// MagnetUploadFile represents the result of uploading a torrent file
type MagnetUploadFile struct {
	File  string `json:"file"`
	Name  string `json:"name,omitempty"`
	Size  int64  `json:"size,omitempty"`
	Hash  string `json:"hash,omitempty"`
	Ready bool   `json:"ready,omitempty"`
	ID    int    `json:"id,omitempty"`
	Error *Error `json:"error,omitempty"`
}

// MagnetDeleteResponse represents the response from /v4/magnet/delete
type MagnetDeleteResponse struct {
	Response
	Data struct {
		Message string `json:"message"`
	} `json:"data"`
}

// MagnetRestartResponse represents the response from /v4/magnet/restart
type MagnetRestartResponse struct {
	Response
	Data struct {
		Message string          `json:"message,omitempty"`
		Magnets []MagnetRestart `json:"magnets,omitempty"`
	} `json:"data"`
}

// MagnetRestart represents the result of restarting a magnet
type MagnetRestart struct {
	Magnet  string `json:"magnet"`
	Message string `json:"message,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// LinkUnlockResponse represents the response from /v4/link/unlock
type LinkUnlockResponse struct {
	Response
	Data LinkUnlock `json:"data"`
}

// LinkUnlock represents an unlocked link
type LinkUnlock struct {
	Link       string   `json:"link"`
	Filename   string   `json:"filename"`
	Host       string   `json:"host"`
	Streams    []Stream `json:"streams,omitempty"`
	Paws       bool     `json:"paws"`
	Filesize   int64    `json:"filesize"`
	ID         string   `json:"id"`
	HostDomain string   `json:"hostDomain"`
	Delayed    int      `json:"delayed,omitempty"`
}

// Stream represents a streaming option
type Stream struct {
	ID       string `json:"id"`
	Ext      string `json:"ext"`
	Quality  string `json:"quality"`
	Filesize int64  `json:"filesize"`
	Proto    string `json:"proto"`
	Name     string `json:"name"`
}

// LinkDelayedResponse represents the response from /v4/link/delayed
type LinkDelayedResponse struct {
	Response
	Data LinkDelayed `json:"data"`
}

// LinkDelayed represents a delayed link status
type LinkDelayed struct {
	Status   int    `json:"status"`
	TimeLeft int    `json:"time_left"`
	Link     string `json:"link,omitempty"`
}

// LinkSaveResponse represents the response from /v4/user/links/save
type LinkSaveResponse struct {
	Response
	Data struct {
		Message string `json:"message"`
	} `json:"data"`
}

// LinkDeleteResponse represents the response from /v4/user/links/delete
type LinkDeleteResponse struct {
	Response
	Data struct {
		Message string `json:"message"`
	} `json:"data"`
}

// HistoryDeleteResponse represents the response from /v4/user/history/delete
type HistoryDeleteResponse struct {
	Response
	Data struct {
		Message string `json:"message"`
	} `json:"data"`
}
