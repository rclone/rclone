// Package api provides types used by the GoPro Media Library API.
//
// GoPro does not publish this API. These types are reverse engineered from
// the gopro.com web app and cross-checked against community clients
// (github.com/dustin/gopro-plus, github.com/mvisonneau/gpcd,
// github.com/aricha/GoProcure) and a live account probe on 2026-08-31.
package api

import (
	"fmt"
	"time"
)

// Medium is a single photo, video or other media item in the library
type Medium struct {
	ID            string    `json:"id"`
	GoProUserID   string    `json:"gopro_user_id,omitempty"`
	Filename      string    `json:"filename"`
	FileExtension string    `json:"file_extension"`
	Type          string    `json:"type"`
	CapturedAt    time.Time `json:"captured_at"`
	CreatedAt     time.Time `json:"created_at"`
	FileSize      *int64    `json:"file_size"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	CameraModel   string    `json:"camera_model"`
	ItemCount     int       `json:"item_count"`
	MomentsCount  int       `json:"moments_count"`
	ReadyToView   string    `json:"ready_to_view"`
	Token         string    `json:"token"`
	ContentTitle  string    `json:"content_title"`
	Resolution    string    `json:"resolution"`
	// ReprocessedAt is set once GoPro has reprocessed a medium after its
	// initial upload (null if it never has been). A live account probe
	// found this uniquely set on the one medium (out of hundreds checked)
	// whose cached FileSize was wrong - see "Size verification" in the
	// gopro backend docs.
	ReprocessedAt *time.Time `json:"reprocessed_at"`
}

// MediumUpdate is the request body for PUT /media/{id}, which updates a
// medium in place - only the fields set here are changed. Confirmed live:
// this can rename a medium (Filename/ContentTitle) and change its
// CapturedAt, both normally fixed at upload time.
type MediumUpdate struct {
	Filename     *string    `json:"filename,omitempty"`
	ContentTitle *string    `json:"content_title,omitempty"`
	CapturedAt   *time.Time `json:"captured_at,omitempty"`
}

// PageInfo describes pagination state returned alongside a media listing
type PageInfo struct {
	CurrentPage int `json:"current_page"`
	PerPage     int `json:"per_page"`
	TotalItems  int `json:"total_items"`
	TotalPages  int `json:"total_pages"`
}

// SearchResponse is returned from GET /media/search
type SearchResponse struct {
	Embedded struct {
		Media  []Medium   `json:"media"`
		Errors []APIError `json:"errors"`
	} `json:"_embedded"`
	Pages PageInfo `json:"_pages"`
}

// DeletedMediaResponse is returned from GET /media/deleted
//
// Confirmed live: each item here carries every field Medium does (plus
// several deletion-specific ones this backend has no use for, such as
// delete_scheduled_at and associations) under the same names, so it
// decodes directly into Medium.
type DeletedMediaResponse struct {
	DeletedMedia []Medium `json:"deleted_media"`
	Pages        PageInfo `json:"_pages"`
}

// RestoreRequest is the request body for POST /media/restore, which moves
// media back out of GoPro's trash to the active library - confirmed live,
// undocumented.
type RestoreRequest struct {
	IDs []string `json:"ids"`
}

// CollectionCreate is the request body for POST /collections, which creates
// a public share link (GoPro calls it a "collection" internally, though it
// always holds exactly one medium as used by this backend). Cloneable is
// GoPro's own field name for what its web UI labels "Allow Download" -
// confirmed live via that UI, enabling it also shares any GPS data embedded
// in the file, not just download access - there's one field for both.
type CollectionCreate struct {
	Title     string `json:"title,omitempty"`
	Cloneable bool   `json:"cloneable"`
}

// Collection is returned from POST /collections and PUT /collections/{id} -
// id is a UUID (a distinct id space from a medium's 24-hex-character id),
// and is also the path segment of the collection's public share URL,
// https://gopro.com/v/{id} - confirmed live, readable with no authentication.
type Collection struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Cloneable bool   `json:"cloneable"`
}

// CollectionMediaUpdate is the request body for PUT /collections/{id},
// which adds the given media to the share.
type CollectionMediaUpdate struct {
	MediaIDs []string `json:"media_ids"`
}

// File is a downloadable rendition of a medium, as returned in
// _embedded.files or _embedded.variations from GET /media/{id}/download.
//
// files[] and variations[] share this shape on the wire; Label and Quality
// are only populated on variations.
type File struct {
	URL            string `json:"url"`
	Head           string `json:"head"`
	CameraPosition string `json:"camera_position,omitempty"`
	ItemNumber     int    `json:"item_number"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Orientation    int    `json:"orientation,omitempty"`
	VideoCodec     string `json:"video_codec,omitempty"`
	Label          string `json:"label,omitempty"`
	Type           string `json:"type,omitempty"`
	Quality        string `json:"quality,omitempty"`
	Available      bool   `json:"available"`
}

// SidecarFile is a non-media file associated with a medium (telemetry, GPS
// track, metadata) as returned in _embedded.sidecar_files.
type SidecarFile struct {
	URL         string  `json:"url"`
	Head        string  `json:"head"`
	Label       string  `json:"label"`
	Type        string  `json:"type"`
	FPS         float64 `json:"fps,omitempty"`
	ItemNumber  int     `json:"item_number,omitempty"`
	TaskVersion string  `json:"task_version,omitempty"`
	Available   bool    `json:"available"`
}

// DownloadResponse is returned from GET /media/{id}/download
//
// _embedded.files is a proxy rendition, not the camera original. The
// original lives in _embedded.variations under label:"source". Callers
// should prefer that and fall back to files[0].
type DownloadResponse struct {
	Filename string `json:"filename"`
	Embedded struct {
		Files        []File        `json:"files"`
		Variations   []File        `json:"variations"`
		SidecarFiles []SidecarFile `json:"sidecar_files"`
	} `json:"_embedded"`
}

// StorageBucket is one of UserInfo's two storage breakdowns
type StorageBucket struct {
	TotalCount   int64 `json:"total_count"`
	TotalStorage int64 `json:"total_storage"`
}

// UserInfo is returned from GET /media/user
//
// GoPro Media Library has two storage pools: media from a GoPro-branded
// camera is "exempt" and doesn't count against any limit ("Unlimited
// Storage" in the account dashboard); every other upload is "non_exempt"
// and is capped at NonExemptStorageLimit ("Additional Storage: x/100GB" in
// the dashboard).
// TotalStorage/TotalCount are the sum of both and are not, on their own,
// comparable to NonExemptStorageLimit.
type UserInfo struct {
	ID                    string        `json:"id"`
	NonExemptStorageLimit int64         `json:"non_exempt_storage_limit"`
	TotalCount            int64         `json:"total_count"`
	TotalStorage          int64         `json:"total_storage"`
	Exempt                StorageBucket `json:"exempt"`
	NonExempt             StorageBucket `json:"non_exempt"`
	CreatedAt             time.Time     `json:"created_at"`
	UpdatedAt             time.Time     `json:"updated_at"`
}

// APIError is a single error as returned in an _embedded.errors array, for
// example from DELETE /media
type APIError struct {
	Reason      string `json:"reason"`
	Code        int    `json:"code"`
	Description string `json:"description"`
	ID          string `json:"id"`
}

// DeleteResponse is returned from DELETE /media
type DeleteResponse struct {
	Embedded struct {
		Errors []APIError `json:"errors"`
	} `json:"_embedded"`
}

// UploadAuthorization describes one chunk upload target, as returned in
// _embedded.authorizations from GET /user-uploads/{derivativeID}
type UploadAuthorization struct {
	URL           string `json:"url"`
	Part          int    `json:"part"`
	ContentLength string `json:"Content-Length"`
}

// UserUploadsResponse is returned from GET /user-uploads/{derivativeID}
type UserUploadsResponse struct {
	Embedded struct {
		Authorizations []UploadAuthorization `json:"authorizations"`
	} `json:"_embedded"`
}

// Error is returned for OAuth token endpoint failures, which use the
// standard OAuth2 error shape ({"error": "...", "error_description": "..."})
// rather than GoPro's own error shape.
type Error struct {
	ErrorCode        string `json:"error"`
	ErrorDescription string `json:"error_description"`
	Status           int    `json:"-"`
	Body             string `json:"-"`
}

// Error satisfies the error interface
func (e *Error) Error() string {
	if e.ErrorCode != "" {
		return fmt.Sprintf("gopro: %s: %s (%d)", e.ErrorCode, e.ErrorDescription, e.Status)
	}
	if e.Body != "" {
		return fmt.Sprintf("gopro: HTTP error %d: %s", e.Status, e.Body)
	}
	return fmt.Sprintf("gopro: HTTP error %d", e.Status)
}
