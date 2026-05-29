// Type definitions specific to Dataverse

package api

// DataverseDatasetResponse is returned by the Dataverse dataset API
type DataverseDatasetResponse struct {
	Status string           `json:"status"`
	Data   DataverseDataset `json:"data"`
}

// DataverseDataset is the representation of a dataset
type DataverseDataset struct {
	LatestVersion DataverseDatasetVersion `json:"latestVersion"`
}

// DataverseDatasetVersion is the representation of a dataset version
type DataverseDatasetVersion struct {
	LastUpdateTime string          `json:"lastUpdateTime"`
	Files          []DataverseFile `json:"files"`
}

// DataverseFile is the representation of a file found in a dataset
type DataverseFile struct {
	DirectoryLabel string            `json:"directoryLabel"`
	Restricted     bool              `json:"restricted,omitempty"`
	DataFile       DataverseDataFile `json:"dataFile"`
}

// DataverseDataFile represents file metadata details
type DataverseDataFile struct {
	ID                 int64     `json:"id"`
	Filename           string    `json:"filename"`
	ContentType        string    `json:"contentType"`
	FileSize           int64     `json:"filesize"`
	OriginalFileFormat string    `json:"originalFileFormat"`
	OriginalFileSize   int64     `json:"originalFileSize"`
	OriginalFileName   string    `json:"originalFileName"`
	MD5                string    `json:"md5"`
	Checksum           *Checksum `json:"checksum,omitempty"` // some endpoints return a {type,value} block instead of md5
}

// IsIngested reports whether Dataverse stored an "original" form
// alongside the archival form (true for tabular ingest: CSV/SPSS/Stata
// uploads parsed into a normalised .tab).
func (d DataverseDataFile) IsIngested() bool {
	return d.OriginalFileName != "" && d.OriginalFileName != d.Filename
}

// StoredMD5 returns the MD5 Dataverse persisted for this file, from
// either the legacy `md5` field or the newer `checksum` envelope. For
// tabular-ingest files this is the MD5 of the ORIGINAL upload, not the
// archival form Dataverse computes on demand.
func (d DataverseDataFile) StoredMD5() string {
	if d.MD5 != "" {
		return d.MD5
	}
	if d.Checksum != nil && (d.Checksum.Type == "MD5" || d.Checksum.Type == "md5") {
		return d.Checksum.Value
	}
	return ""
}

// Checksum captures the `{type, value}` envelope Dataverse returns for
// hashes on some endpoints.
type Checksum struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// DataverseVersionResponse is returned by the dataset version API
// (/api/datasets/:persistentId/versions/{version}). Its data payload is
// a single version (unlike DataverseDatasetResponse, whose data wraps
// latestVersion), so it reuses DataverseDatasetVersion directly.
type DataverseVersionResponse struct {
	Status  string                  `json:"status"`
	Data    DataverseDatasetVersion `json:"data"`
	Message string                  `json:"message,omitempty"` // populated on "status":"ERROR"
}

// AuthHeader is the request header Dataverse uses for token-based API
// authentication. The byte-stream reads attach it directly (and strip
// it on the cross-host redirect to S3).
const AuthHeader = "X-Dataverse-Key"

// LatestVersion asks Dataverse for the latest version the caller can
// see (draft for owners, latest published for everyone else).
const LatestVersion = ":latest"

// The /tree endpoint is a lazy, paginated view of one directory level
// inside a dataset version:
//
//	GET {host}/api/datasets/:persistentId/versions/{version}/tree
//	    ?persistentId={pid}&path={dir}&limit={n}[&cursor={c}][&originals=true]
//
// Unlike the whole-version file list it returns ONE directory level at a
// time and pages through an opaque cursor, turning mount-time listing of
// a large dataset from "fetch everything" into "fetch the root level".
// It is newer than the backend's minimum Dataverse, so callers
// feature-detect it and fall back to the whole-version list.

// TreeResponse is the envelope returned by the /tree endpoint.
type TreeResponse struct {
	Status  string   `json:"status"`
	Data    TreePage `json:"data"`
	Message string   `json:"message,omitempty"` // populated on "status":"ERROR"
}

// TreePage is one page of a single directory level.
type TreePage struct {
	Path  string     `json:"path"`
	Items []TreeItem `json:"items"`
	// NextCursor is nil/absent on the last page; echo it back verbatim to
	// fetch the next page.
	NextCursor       *string `json:"nextCursor"`
	Limit            int     `json:"limit,omitempty"`
	Order            string  `json:"order,omitempty"`
	ApproximateCount int64   `json:"approximateCount,omitempty"`
}

// TreeItem is one entry in a directory level. Folders carry `counts`;
// files carry the wire identity (`id`) and the metadata needed to build
// an Object the shared Open path can consume.
type TreeItem struct {
	Type string `json:"type"` // "folder" | "file"
	Name string `json:"name"`
	Path string `json:"path"`

	// Folder-only.
	Counts *TreeCounts `json:"counts,omitempty"`

	// File-only. Size/Checksum/DownloadURL reflect the original-upload
	// form when the page was requested with originals=true, otherwise the
	// served (archival) form.
	ID          int64         `json:"id,omitempty"`
	Size        int64         `json:"size,omitempty"`
	ContentType string        `json:"contentType,omitempty"`
	Access      string        `json:"access,omitempty"` // "public"|"restricted"|"embargoed"
	Checksum    *TreeChecksum `json:"checksum,omitempty"`
	DownloadURL string        `json:"downloadUrl,omitempty"`
}

// TreeChecksum is the {type,value} hash envelope on file items.
type TreeChecksum struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// TreeCounts is the per-folder recursive summary (informational only).
type TreeCounts struct {
	Files      int64 `json:"files"`
	Folders    int64 `json:"folders"`
	Bytes      int64 `json:"bytes"`
	Restricted int64 `json:"restricted"`
	Embargoed  int64 `json:"embargoed"`
}

// IsFolder reports whether the item is a directory.
func (i *TreeItem) IsFolder() bool { return i.Type == "folder" }

// ChecksumValue returns the file's checksum value, or "" if absent.
func (i *TreeItem) ChecksumValue() string {
	if i.Checksum == nil {
		return ""
	}
	return i.Checksum.Value
}
