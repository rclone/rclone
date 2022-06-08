// Types passed and returned to and from the API

package api

import (
	"strings"
	"time"
)

const (
	timeFormat = `"` + time.RFC3339 + `"`

	// PackageTypeOneNote is the package type value for OneNote files
	PackageTypeOneNote = "oneNote"
)

// Error is returned from one drive when things go wrong
type Error struct {
	ErrorInfo struct {
		Code       string `json:"code"`
		Message    string `json:"message"`
		InnerError struct {
			Code string `json:"code"`
		} `json:"innererror"`
	} `json:"error"`
}

// Error returns a string for the error and satisfies the error interface
func (e *Error) Error() string {
	out := e.ErrorInfo.Code
	if e.ErrorInfo.InnerError.Code != "" {
		out += ": " + e.ErrorInfo.InnerError.Code
	}
	out += ": " + e.ErrorInfo.Message
	return out
}

// Check Error satisfies the error interface
var _ error = (*Error)(nil)

// Identity represents an identity of an actor. For example, and actor
// can be a user, device, or application.
type Identity struct {
	DisplayName string `json:"displayName"`
	ID          string `json:"id"`
}

// IdentitySet is a keyed collection of Identity objects. It is used
// to represent a set of identities associated with various events for
// an item, such as created by or last modified by.
type IdentitySet struct {
	User        Identity `json:"user"`
	Application Identity `json:"application"`
	Device      Identity `json:"device"`
}

// Quota groups storage space quota-related information on OneDrive into a single structure.
type Quota struct {
	Total     int64  `json:"total"`
	Used      int64  `json:"used"`
	Remaining int64  `json:"remaining"`
	Deleted   int64  `json:"deleted"`
	State     string `json:"state"` // normal | nearing | critical | exceeded
}

// Drive is a representation of a drive resource
type Drive struct {
	ID        string      `json:"id"`
	DriveType string      `json:"driveType"`
	Owner     IdentitySet `json:"owner"`
	Quota     Quota       `json:"quota"`
}

// Timestamp represents represents date and time information for the
// OneDrive API, by using ISO 8601 and is always in UTC time.
type Timestamp time.Time

// MarshalJSON turns a Timestamp into JSON (in UTC)
func (t *Timestamp) MarshalJSON() (out []byte, err error) {
	timeString := (*time.Time)(t).UTC().Format(timeFormat)
	return []byte(timeString), nil
}

// UnmarshalJSON turns JSON into a Timestamp
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	newT, err := time.Parse(timeFormat, string(data))
	if err != nil {
		return err
	}
	*t = Timestamp(newT)
	return nil
}

// ItemReference groups data needed to reference a OneDrive item
// across the service into a single structure.
type ItemReference struct {
	DriveID   string `json:"driveId"`   // Unique identifier for the Drive that contains the item.	Read-only.
	ID        string `json:"id"`        // Unique identifier for the item.	Read/Write.
	Path      string `json:"path"`      // Path that used to navigate to the item.	Read/Write.
	DriveType string `json:"driveType"` // Type of the drive,	Read-Only
}

// RemoteItemFacet groups data needed to reference a OneDrive remote item
type RemoteItemFacet struct {
	ID                   string               `json:"id"`                   // The unique identifier of the item within the remote Drive. Read-only.
	Name                 string               `json:"name"`                 // The name of the item (filename and extension). Read-write.
	CreatedBy            IdentitySet          `json:"createdBy"`            // Identity of the user, device, and application which created the item. Read-only.
	LastModifiedBy       IdentitySet          `json:"lastModifiedBy"`       // Identity of the user, device, and application which last modified the item. Read-only.
	CreatedDateTime      Timestamp            `json:"createdDateTime"`      // Date and time of item creation. Read-only.
	LastModifiedDateTime Timestamp            `json:"lastModifiedDateTime"` // Date and time the item was last modified. Read-only.
	Folder               *FolderFacet         `json:"folder"`               // Folder metadata, if the item is a folder. Read-only.
	File                 *FileFacet           `json:"file"`                 // File metadata, if the item is a file. Read-only.
	Package              *PackageFacet        `json:"package"`              // If present, indicates that this item is a package instead of a folder or file. Packages are treated like files in some contexts and folders in others. Read-only.
	FileSystemInfo       *FileSystemInfoFacet `json:"fileSystemInfo"`       // File system information on client. Read-write.
	ParentReference      *ItemReference       `json:"parentReference"`      // Parent information, if the item has a parent. Read-write.
	Size                 int64                `json:"size"`                 // Size of the item in bytes. Read-only.
	WebURL               string               `json:"webUrl"`               // URL that displays the resource in the browser. Read-only.
}

// FolderFacet groups folder-related data on OneDrive into a single structure
type FolderFacet struct {
	ChildCount int64 `json:"childCount"` // Number of children contained immediately within this container.
}

// HashesType groups different types of hashes into a single structure, for an item on OneDrive.
type HashesType struct {
	Sha1Hash     string `json:"sha1Hash"`     // hex encoded SHA1 hash for the contents of the file (if available)
	Crc32Hash    string `json:"crc32Hash"`    // hex encoded CRC32 value of the file (if available)
	QuickXorHash string `json:"quickXorHash"` // base64 encoded QuickXorHash value of the file (if available)
}

// FileFacet groups file-related data on OneDrive into a single structure.
type FileFacet struct {
	MimeType string     `json:"mimeType"` // The MIME type for the file. This is determined by logic on the server and might not be the value provided when the file was uploaded.
	Hashes   HashesType `json:"hashes"`   // Hashes of the file's binary content, if available.
}

// FileSystemInfoFacet contains properties that are reported by the
// device's local file system for the local version of an item. This
// facet can be used to specify the last modified date or created date
// of the item as it was on the local device.
type FileSystemInfoFacet struct {
	CreatedDateTime      Timestamp `json:"createdDateTime"`      // The UTC date and time the file was created on a client.
	LastModifiedDateTime Timestamp `json:"lastModifiedDateTime"` // The UTC date and time the file was last modified on a client.
}

// DeletedFacet indicates that the item on OneDrive has been
// deleted. In this version of the API, the presence (non-null) of the
// facet value indicates that the file was deleted. A null (or
// missing) value indicates that the file is not deleted.
type DeletedFacet struct {
}

// PackageFacet indicates that a DriveItem is the top level item
// in a "package" or a collection of items that should be treated as a collection instead of individual items.
// `oneNote` is the only currently defined value.
type PackageFacet struct {
	Type string `json:"type"`
}

// Item represents metadata for an item in OneDrive
type Item struct {
	ID                   string               `json:"id"`                   // The unique identifier of the item within the Drive. Read-only.
	Name                 string               `json:"name"`                 // The name of the item (filename and extension). Read-write.
	ETag                 string               `json:"eTag"`                 // eTag for the entire item (metadata + content). Read-only.
	CTag                 string               `json:"cTag"`                 // An eTag for the content of the item. This eTag is not changed if only the metadata is changed. Read-only.
	CreatedBy            IdentitySet          `json:"createdBy"`            // Identity of the user, device, and application which created the item. Read-only.
	LastModifiedBy       IdentitySet          `json:"lastModifiedBy"`       // Identity of the user, device, and application which last modified the item. Read-only.
	CreatedDateTime      Timestamp            `json:"createdDateTime"`      // Date and time of item creation. Read-only.
	LastModifiedDateTime Timestamp            `json:"lastModifiedDateTime"` // Date and time the item was last modified. Read-only.
	Size                 int64                `json:"size"`                 // Size of the item in bytes. Read-only.
	ParentReference      *ItemReference       `json:"parentReference"`      // Parent information, if the item has a parent. Read-write.
	WebURL               string               `json:"webUrl"`               // URL that displays the resource in the browser. Read-only.
	Description          string               `json:"description"`          // Provide a user-visible description of the item. Read-write.
	Folder               *FolderFacet         `json:"folder"`               // Folder metadata, if the item is a folder. Read-only.
	File                 *FileFacet           `json:"file"`                 // File metadata, if the item is a file. Read-only.
	RemoteItem           *RemoteItemFacet     `json:"remoteItem"`           // Remote Item metadata, if the item is a remote shared item. Read-only.
	FileSystemInfo       *FileSystemInfoFacet `json:"fileSystemInfo"`       // File system information on client. Read-write.
	//	Image                *ImageFacet          `json:"image"`                // Image metadata, if the item is an image. Read-only.
	//	Photo                *PhotoFacet          `json:"photo"`                // Photo metadata, if the item is a photo. Read-only.
	//	Audio                *AudioFacet          `json:"audio"`                // Audio metadata, if the item is an audio file. Read-only.
	//	Video                *VideoFacet          `json:"video"`                // Video metadata, if the item is a video. Read-only.
	//	Location             *LocationFacet       `json:"location"`             // Location metadata, if the item has location data. Read-only.
	Package *PackageFacet `json:"package"` // If present, indicates that this item is a package instead of a folder or file. Packages are treated like files in some contexts and folders in others. Read-only.
	Deleted *DeletedFacet `json:"deleted"` // Information about the deleted state of the item. Read-only.
}

// ViewDeltaResponse is the response to the view delta method
type ViewDeltaResponse struct {
	Value      []Item `json:"value"`            // An array of Item objects which have been created, modified, or deleted.
	NextLink   string `json:"@odata.nextLink"`  // A URL to retrieve the next available page of changes.
	DeltaLink  string `json:"@odata.deltaLink"` // A URL returned instead of @odata.nextLink after all current changes have been returned. Used to read the next set of changes in the future.
	DeltaToken string `json:"@delta.token"`     // A token value that can be used in the query string on manually-crafted calls to view.delta. Not needed if you're using nextLink and deltaLink.
}

// ListChildrenResponse is the response to the list children method
type ListChildrenResponse struct {
	Value    []Item `json:"value"`           // An array of Item objects
	NextLink string `json:"@odata.nextLink"` // A URL to retrieve the next available page of items.
}

// CreateItemRequest is the request to create an item object
type CreateItemRequest struct {
	Name             string      `json:"name"`                   // Name of the folder to be created.
	Folder           FolderFacet `json:"folder"`                 // Empty Folder facet to indicate that folder is the type of resource to be created.
	ConflictBehavior string      `json:"@name.conflictBehavior"` // Determines what to do if an item with a matching name already exists in this folder. Accepted values are: rename, replace, and fail (the default).
}

// SetFileSystemInfo is used to Update an object's FileSystemInfo.
type SetFileSystemInfo struct {
	FileSystemInfo FileSystemInfoFacet `json:"fileSystemInfo"` // File system information on client. Read-write.
}

// CreateUploadRequest is used by CreateUploadSession to set the dates correctly
type CreateUploadRequest struct {
	Item SetFileSystemInfo `json:"item"`
}

// CreateUploadResponse is the response from creating an upload session
type CreateUploadResponse struct {
	UploadURL          string    `json:"uploadUrl"`          // "https://sn3302.up.1drv.com/up/fe6987415ace7X4e1eF866337",
	ExpirationDateTime Timestamp `json:"expirationDateTime"` // "2015-01-29T09:21:55.523Z",
	NextExpectedRanges []string  `json:"nextExpectedRanges"` // ["0-"]
}

// UploadFragmentResponse is the response from uploading a fragment
type UploadFragmentResponse struct {
	ExpirationDateTime Timestamp `json:"expirationDateTime"` // "2015-01-29T09:21:55.523Z",
	NextExpectedRanges []string  `json:"nextExpectedRanges"` // ["0-"]
}

// CopyItemRequest is the request to copy an item object
//
// Note: The parentReference should include either an id or path but
// not both. If both are included, they need to reference the same
// item or an error will occur.
type CopyItemRequest struct {
	ParentReference ItemReference `json:"parentReference"` // Reference to the parent item the copy will be created in.
	Name            *string       `json:"name"`            // Optional The new name for the copy. If this isn't provided, the same name will be used as the original.
}

// MoveItemRequest is the request to copy an item object
//
// Note: The parentReference should include either an id or path but
// not both. If both are included, they need to reference the same
// item or an error will occur.
type MoveItemRequest struct {
	ParentReference *ItemReference       `json:"parentReference,omitempty"` // Reference to the destination parent directory
	Name            string               `json:"name,omitempty"`            // Optional The new name for the file. If this isn't provided, the same name will be used as the original.
	FileSystemInfo  *FileSystemInfoFacet `json:"fileSystemInfo,omitempty"`  // File system information on client. Read-write.
}

//CreateShareLinkRequest is the request to create a sharing link
//Always Type:view and Scope:anonymous for public sharing
type CreateShareLinkRequest struct {
	Type     string     `json:"type"`                         // Link type in View, Edit or Embed
	Scope    string     `json:"scope,omitempty"`              // Scope in anonymous, organization
	Password string     `json:"password,omitempty"`           // The password of the sharing link that is set by the creator. Optional and OneDrive Personal only.
	Expiry   *time.Time `json:"expirationDateTime,omitempty"` // A String with format of yyyy-MM-ddTHH:mm:ssZ of DateTime indicates the expiration time of the permission.
}

//CreateShareLinkResponse is the response from CreateShareLinkRequest
type CreateShareLinkResponse struct {
	ID    string   `json:"id"`
	Roles []string `json:"roles"`
	Link  struct {
		Type        string `json:"type"`
		Scope       string `json:"scope"`
		WebURL      string `json:"webUrl"`
		Application struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"application"`
	} `json:"link"`
}

// AsyncOperationStatus provides information on the status of an asynchronous job progress.
//
// The following API calls return AsyncOperationStatus resources:
//
// Copy Item
// Upload From URL
type AsyncOperationStatus struct {
	PercentageComplete float64 `json:"percentageComplete"` // A float value between 0 and 100 that indicates the percentage complete.
	Status             string  `json:"status"`             // A string value that maps to an enumeration of possible values about the status of the job. "notStarted | inProgress | completed | updating | failed | deletePending | deleteFailed | waiting"
	ErrorCode          string  `json:"errorCode"`          // Not officially documented :(
}

// GetID returns a normalized ID of the item
// If DriveID is known it will be prefixed to the ID with # separator
// Can be parsed using onedrive.parseNormalizedID(normalizedID)
func (i *Item) GetID() string {
	if i.IsRemote() && i.RemoteItem.ID != "" {
		return i.RemoteItem.ParentReference.DriveID + "#" + i.RemoteItem.ID
	} else if i.ParentReference != nil && !strings.Contains(i.ID, "#") {
		return i.ParentReference.DriveID + "#" + i.ID
	}
	return i.ID
}

// GetDriveID returns a normalized ParentReference of the item
func (i *Item) GetDriveID() string {
	return i.GetParentReference().DriveID
}

// GetName returns a normalized Name of the item
func (i *Item) GetName() string {
	if i.IsRemote() && i.RemoteItem.Name != "" {
		return i.RemoteItem.Name
	}
	return i.Name
}

// GetFolder returns a normalized Folder of the item
func (i *Item) GetFolder() *FolderFacet {
	if i.IsRemote() && i.RemoteItem.Folder != nil {
		return i.RemoteItem.Folder
	}
	return i.Folder
}

// GetPackage returns a normalized Package of the item
func (i *Item) GetPackage() *PackageFacet {
	if i.IsRemote() && i.RemoteItem.Package != nil {
		return i.RemoteItem.Package
	}
	return i.Package
}

// GetPackageType returns the package type of the item if available,
// otherwise ""
func (i *Item) GetPackageType() string {
	pack := i.GetPackage()
	if pack == nil {
		return ""
	}
	return pack.Type
}

// GetFile returns a normalized File of the item
func (i *Item) GetFile() *FileFacet {
	if i.IsRemote() && i.RemoteItem.File != nil {
		return i.RemoteItem.File
	}
	return i.File
}

// GetFileSystemInfo returns a normalized FileSystemInfo of the item
func (i *Item) GetFileSystemInfo() *FileSystemInfoFacet {
	if i.IsRemote() && i.RemoteItem.FileSystemInfo != nil {
		return i.RemoteItem.FileSystemInfo
	}
	return i.FileSystemInfo
}

// GetSize returns a normalized Size of the item
func (i *Item) GetSize() int64 {
	if i.IsRemote() && i.RemoteItem.Size != 0 {
		return i.RemoteItem.Size
	}
	return i.Size
}

// GetWebURL returns a normalized WebURL of the item
func (i *Item) GetWebURL() string {
	if i.IsRemote() && i.RemoteItem.WebURL != "" {
		return i.RemoteItem.WebURL
	}
	return i.WebURL
}

// GetCreatedBy returns a normalized CreatedBy of the item
func (i *Item) GetCreatedBy() IdentitySet {
	if i.IsRemote() && i.RemoteItem.CreatedBy != (IdentitySet{}) {
		return i.RemoteItem.CreatedBy
	}
	return i.CreatedBy
}

// GetLastModifiedBy returns a normalized LastModifiedBy of the item
func (i *Item) GetLastModifiedBy() IdentitySet {
	if i.IsRemote() && i.RemoteItem.LastModifiedBy != (IdentitySet{}) {
		return i.RemoteItem.LastModifiedBy
	}
	return i.LastModifiedBy
}

// GetCreatedDateTime returns a normalized CreatedDateTime of the item
func (i *Item) GetCreatedDateTime() Timestamp {
	if i.IsRemote() && i.RemoteItem.CreatedDateTime != (Timestamp{}) {
		return i.RemoteItem.CreatedDateTime
	}
	return i.CreatedDateTime
}

// GetLastModifiedDateTime returns a normalized LastModifiedDateTime of the item
func (i *Item) GetLastModifiedDateTime() Timestamp {
	if i.IsRemote() && i.RemoteItem.LastModifiedDateTime != (Timestamp{}) {
		return i.RemoteItem.LastModifiedDateTime
	}
	return i.LastModifiedDateTime
}

// GetParentReference returns a normalized ParentReference of the item
func (i *Item) GetParentReference() *ItemReference {
	if i.IsRemote() && i.ParentReference == nil {
		return i.RemoteItem.ParentReference
	}
	return i.ParentReference
}

// IsRemote checks if item is a remote item
func (i *Item) IsRemote() bool {
	return i.RemoteItem != nil
}

// User details for each version
type User struct {
	Email       string `json:"email"`
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
}

// LastModifiedBy for each version
type LastModifiedBy struct {
	User User `json:"user"`
}

// Version info
type Version struct {
	ID                   string         `json:"id"`
	LastModifiedDateTime time.Time      `json:"lastModifiedDateTime"`
	Size                 int            `json:"size"`
	LastModifiedBy       LastModifiedBy `json:"lastModifiedBy"`
}

// VersionsResponse is returned from /versions
type VersionsResponse struct {
	Versions []Version `json:"value"`
}
