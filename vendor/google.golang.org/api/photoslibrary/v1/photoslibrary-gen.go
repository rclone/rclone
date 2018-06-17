// Package photoslibrary provides access to the Photos Library API.
//
// See https://developers.google.com/photos/
//
// Usage example:
//
//   import "google.golang.org/api/photoslibrary/v1"
//   ...
//   photoslibraryService, err := photoslibrary.New(oauthHttpClient)
package photoslibrary // import "google.golang.org/api/photoslibrary/v1"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	context "golang.org/x/net/context"
	ctxhttp "golang.org/x/net/context/ctxhttp"
	gensupport "google.golang.org/api/gensupport"
	googleapi "google.golang.org/api/googleapi"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Always reference these packages, just in case the auto-generated code
// below doesn't.
var _ = bytes.NewBuffer
var _ = strconv.Itoa
var _ = fmt.Sprintf
var _ = json.NewDecoder
var _ = io.Copy
var _ = url.Parse
var _ = gensupport.MarshalJSON
var _ = googleapi.Version
var _ = errors.New
var _ = strings.Replace
var _ = context.Canceled
var _ = ctxhttp.Do

const apiId = "photoslibrary:v1"
const apiName = "photoslibrary"
const apiVersion = "v1"
const basePath = "https://photoslibrary.googleapis.com/"

// OAuth2 scopes used by this API.
const (
	// View the photos, videos and albums in your Google Photos
	DrivePhotosReadonlyScope = "https://www.googleapis.com/auth/drive.photos.readonly"

	// View and manage your Google Photos library
	PhotoslibraryScope = "https://www.googleapis.com/auth/photoslibrary"

	// Add to your Google Photos library
	PhotoslibraryAppendonlyScope = "https://www.googleapis.com/auth/photoslibrary.appendonly"

	// View your Google Photos library
	PhotoslibraryReadonlyScope = "https://www.googleapis.com/auth/photoslibrary.readonly"

	// Manage photos added by this app
	PhotoslibraryReadonlyAppcreateddataScope = "https://www.googleapis.com/auth/photoslibrary.readonly.appcreateddata"

	// Manage and add to shared albums on your behalf
	PhotoslibrarySharingScope = "https://www.googleapis.com/auth/photoslibrary.sharing"
)

func New(client *http.Client) (*Service, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}
	s := &Service{client: client, BasePath: basePath}
	s.Albums = NewAlbumsService(s)
	s.MediaItems = NewMediaItemsService(s)
	s.SharedAlbums = NewSharedAlbumsService(s)
	return s, nil
}

type Service struct {
	client    *http.Client
	BasePath  string // API endpoint base URL
	UserAgent string // optional additional User-Agent fragment

	Albums *AlbumsService

	MediaItems *MediaItemsService

	SharedAlbums *SharedAlbumsService
}

func (s *Service) userAgent() string {
	if s.UserAgent == "" {
		return googleapi.UserAgent
	}
	return googleapi.UserAgent + " " + s.UserAgent
}

func NewAlbumsService(s *Service) *AlbumsService {
	rs := &AlbumsService{s: s}
	return rs
}

type AlbumsService struct {
	s *Service
}

func NewMediaItemsService(s *Service) *MediaItemsService {
	rs := &MediaItemsService{s: s}
	return rs
}

type MediaItemsService struct {
	s *Service
}

func NewSharedAlbumsService(s *Service) *SharedAlbumsService {
	rs := &SharedAlbumsService{s: s}
	return rs
}

type SharedAlbumsService struct {
	s *Service
}

// AddEnrichmentToAlbumRequest: Request to add an enrichment to a
// specific album at a specific position.
type AddEnrichmentToAlbumRequest struct {
	// AlbumPosition: The position where the enrichment will be inserted.
	AlbumPosition *AlbumPosition `json:"albumPosition,omitempty"`

	// NewEnrichmentItem: The enrichment to be added.
	NewEnrichmentItem *NewEnrichmentItem `json:"newEnrichmentItem,omitempty"`

	// ForceSendFields is a list of field names (e.g. "AlbumPosition") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "AlbumPosition") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *AddEnrichmentToAlbumRequest) MarshalJSON() ([]byte, error) {
	type NoMethod AddEnrichmentToAlbumRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type AddEnrichmentToAlbumResponse struct {
	// EnrichmentItem: [Output only] Enrichment which was added.
	EnrichmentItem *EnrichmentItem `json:"enrichmentItem,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "EnrichmentItem") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "EnrichmentItem") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *AddEnrichmentToAlbumResponse) MarshalJSON() ([]byte, error) {
	type NoMethod AddEnrichmentToAlbumResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Album: Representation of an album in Google Photos.
// Albums are a container for media items. They contain an
// additional
// shareInfo property if they have been shared by the application.
type Album struct {
	// CoverPhotoBaseUrl: [Output only] A URL to the cover photo's bytes.
	// This should not be used as
	// is. Parameters should be appended to this URL before use. For
	// example,
	// '=w2048-h1024' will set the dimensions of the cover photo to have a
	// width
	// of 2048 px and height of 1024 px.
	CoverPhotoBaseUrl string `json:"coverPhotoBaseUrl,omitempty"`

	// Id: [Ouput only] Identifier for the album. This is a persistent
	// identifier that
	// can be used between sessions to identify this album.
	Id string `json:"id,omitempty"`

	// IsWriteable: [Output only] True if media items can be created in the
	// album.
	// This field is based on the scopes granted and permissions of the
	// album. If
	// the scopes are changed or permissions of the album are changed, this
	// field
	// will be updated.
	IsWriteable bool `json:"isWriteable,omitempty"`

	// ProductUrl: [Output only] Google Photos URL for the album. The user
	// needs to be signed
	// in to their Google Photos account to access this link.
	ProductUrl string `json:"productUrl,omitempty"`

	// ShareInfo: [Output only] Information related to shared albums.
	// This field is only populated if the album is a shared album,
	// the
	// developer created the album and the user has granted
	// photoslibrary.sharing
	// scope.
	ShareInfo *ShareInfo `json:"shareInfo,omitempty"`

	// Title: Name of the album displayed to the user in their Google Photos
	// account.
	// This string should not be more than 500 characters.
	Title string `json:"title,omitempty"`

	// TotalMediaItems: [Output only] The number of media items in the album
	TotalMediaItems int64 `json:"totalMediaItems,omitempty,string"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "CoverPhotoBaseUrl")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CoverPhotoBaseUrl") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *Album) MarshalJSON() ([]byte, error) {
	type NoMethod Album
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// AlbumPosition: Specifies a position in an album.
type AlbumPosition struct {
	// Position: Type of position, for a media or enrichment item.
	//
	// Possible values:
	//   "POSITION_TYPE_UNSPECIFIED" - Default value if this enum is not
	// set.
	//   "FIRST_IN_ALBUM" - At the beginning of the album.
	//   "LAST_IN_ALBUM" - At the end of the album.
	//   "AFTER_MEDIA_ITEM" - After a media item.
	//   "AFTER_ENRICHMENT_ITEM" - After an enrichment item.
	Position string `json:"position,omitempty"`

	// RelativeEnrichmentItemId: The enrichment item to which the position
	// is relative to.
	// Only used when position type is AFTER_ENRICHMENT_ITEM.
	RelativeEnrichmentItemId string `json:"relativeEnrichmentItemId,omitempty"`

	// RelativeMediaItemId: The media item to which the position is relative
	// to.
	// Only used when position type is AFTER_MEDIA_ITEM.
	RelativeMediaItemId string `json:"relativeMediaItemId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Position") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Position") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *AlbumPosition) MarshalJSON() ([]byte, error) {
	type NoMethod AlbumPosition
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// BatchCreateMediaItemsRequest: Request to create one or more media
// items in a user's Google Photos library.
// If an <code>albumid</code> is specified, the media items are also
// added to
// that album. <code>albumPosition</code> is optional and can only be
// specified
// if an <code>albumId</code> is set.
type BatchCreateMediaItemsRequest struct {
	// AlbumId: Identifier of the album where the media item(s) will be
	// added. They will
	// also be added to the user's library. This is an optional field.
	AlbumId string `json:"albumId,omitempty"`

	// AlbumPosition: Position in the album where the media item(s) will be
	// added. If not
	// specified, the media item(s) will be added to the end of the album
	// (as per
	// the default value which is LAST_IN_ALBUM).
	// The request will fail if this field is present but no album_id
	// is
	// specified.
	AlbumPosition *AlbumPosition `json:"albumPosition,omitempty"`

	// NewMediaItems: List of media items to be created.
	NewMediaItems []*NewMediaItem `json:"newMediaItems,omitempty"`

	// ForceSendFields is a list of field names (e.g. "AlbumId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "AlbumId") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *BatchCreateMediaItemsRequest) MarshalJSON() ([]byte, error) {
	type NoMethod BatchCreateMediaItemsRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type BatchCreateMediaItemsResponse struct {
	// NewMediaItemResults: [Output only] List of media items which were
	// created.
	NewMediaItemResults []*NewMediaItemResult `json:"newMediaItemResults,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "NewMediaItemResults")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "NewMediaItemResults") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *BatchCreateMediaItemsResponse) MarshalJSON() ([]byte, error) {
	type NoMethod BatchCreateMediaItemsResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// ContentFilter: This filter is used to define which results to return
// based on the contents
// of the media item.
//
// It is possible to specify a list of categories to include, and/or a
// list of
// categories to exclude. Within each list, the categories are combined
// with an
// OR. For example, if the content filter looks like:
//
//     included_content_categories: [c1, c2, c3]
//
// It would get media items that contain (c1 OR c2 OR c3).
//
// And if the content filter looks like:
//
//     excluded_content_categories: [c1, c2, c3]
//
// It would get media items that contain NOT (c1 OR c2 OR c3).
// You can also include some categories while excluding others, as in
// this
// proto:
//
//     included_content_categories: [c1, c2],
//     excluded_content_category: [c3, c4]
//
// It would get media items that contain (c1 OR c2) AND NOT (c3 OR
// c4).
//
// A category that appears in <code>includedContentategories</code> must
// not
// appear in <code>excludedContentCategories</code>.
type ContentFilter struct {
	// ExcludedContentCategories: The set of categories that must NOT be
	// present in the media items in the
	// result. The items in the set are ORed. There is a maximum of
	// 10
	// excludedContentCategories per request.
	//
	// Possible values:
	//   "NONE" - Default content category. This category is ignored if any
	// other category is
	// also listed.
	//   "LANDSCAPES" - Media items containing landscapes.
	//   "RECEIPTS" - Media items containing receipts.
	//   "CITYSCAPES" - Media items containing cityscapes.
	//   "LANDMARKS" - Media items containing landmarks.
	//   "SELFIES" - Media items that are selfies.
	//   "PEOPLE" - Media items containing people.
	//   "PETS" - Media items containing pets.
	//   "WEDDINGS" - Media items from weddings.
	//   "BIRTHDAYS" - Media items from birthdays.
	//   "DOCUMENTS" - Media items containing documents.
	//   "TRAVEL" - Media items taken during travel.
	//   "ANIMALS" - Media items containing animals.
	//   "FOOD" - Media items containing food.
	//   "SPORT" - Media items from sporting events.
	//   "NIGHT" - Media items taken at night.
	//   "PERFORMANCES" - Media items from performances.
	//   "WHITEBOARDS" - Media items containing whiteboards.
	//   "SCREENSHOTS" - Media items that are screenshots.
	//   "UTILITY" - Media items that are considered to be 'utility.
	// Including, but not limited
	// to documents, screenshots, whiteboards etc.
	ExcludedContentCategories []string `json:"excludedContentCategories,omitempty"`

	// IncludedContentCategories: The set of categories that must be present
	// in the media items in the
	// result. The items in the set are ORed. There is a maximum of
	// 10
	// includedContentCategories per request.
	//
	// Possible values:
	//   "NONE" - Default content category. This category is ignored if any
	// other category is
	// also listed.
	//   "LANDSCAPES" - Media items containing landscapes.
	//   "RECEIPTS" - Media items containing receipts.
	//   "CITYSCAPES" - Media items containing cityscapes.
	//   "LANDMARKS" - Media items containing landmarks.
	//   "SELFIES" - Media items that are selfies.
	//   "PEOPLE" - Media items containing people.
	//   "PETS" - Media items containing pets.
	//   "WEDDINGS" - Media items from weddings.
	//   "BIRTHDAYS" - Media items from birthdays.
	//   "DOCUMENTS" - Media items containing documents.
	//   "TRAVEL" - Media items taken during travel.
	//   "ANIMALS" - Media items containing animals.
	//   "FOOD" - Media items containing food.
	//   "SPORT" - Media items from sporting events.
	//   "NIGHT" - Media items taken at night.
	//   "PERFORMANCES" - Media items from performances.
	//   "WHITEBOARDS" - Media items containing whiteboards.
	//   "SCREENSHOTS" - Media items that are screenshots.
	//   "UTILITY" - Media items that are considered to be 'utility.
	// Including, but not limited
	// to documents, screenshots, whiteboards etc.
	IncludedContentCategories []string `json:"includedContentCategories,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "ExcludedContentCategories") to unconditionally include in API
	// requests. By default, fields with empty values are omitted from API
	// requests. However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g.
	// "ExcludedContentCategories") to include in API requests with the JSON
	// null value. By default, fields with empty values are omitted from API
	// requests. However, any field with an empty value appearing in
	// NullFields will be sent to the server as null. It is an error if a
	// field in this list has a non-empty value. This may be used to include
	// null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *ContentFilter) MarshalJSON() ([]byte, error) {
	type NoMethod ContentFilter
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// ContributorInfo: Information about a user who contributed the media
// item. Note that this
// information is only included if the album containing the media item
// is
// shared, was created by you and you have the sharing scope.
type ContributorInfo struct {
	// DisplayName: Display name of the contributor.
	DisplayName string `json:"displayName,omitempty"`

	// ProfilePictureBaseUrl: URL to the profile picture of the contributor.
	ProfilePictureBaseUrl string `json:"profilePictureBaseUrl,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DisplayName") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DisplayName") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *ContributorInfo) MarshalJSON() ([]byte, error) {
	type NoMethod ContributorInfo
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// CreateAlbumRequest: Request to create an album in Google Photos.
type CreateAlbumRequest struct {
	// Album: The album to be created.
	Album *Album `json:"album,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Album") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Album") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *CreateAlbumRequest) MarshalJSON() ([]byte, error) {
	type NoMethod CreateAlbumRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Date: Represents a whole calendar date. The day may be 0 to represent
// a year and month where the day is not significant, e.g. a whole
// calendar month. The month may be 0 to represent a a day and a year
// where the month is not signficant, e.g. when you want to specify the
// same day in every month of a year or a specific year. The year may be
// 0 to represent a month and day independent of year, e.g. anniversary
// date.
type Date struct {
	// Day: Day of month. Must be from 1 to 31 and valid for the year and
	// month, or 0
	// if specifying a year/month where the day is not significant.
	Day int64 `json:"day,omitempty"`

	// Month: Month of year. Must be from 1 to 12, or 0 if specifying a date
	// without a
	// month.
	Month int64 `json:"month,omitempty"`

	// Year: Year of date. Must be from 1 to 9999, or 0 if specifying a date
	// without
	// a year.
	Year int64 `json:"year,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Day") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Day") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *Date) MarshalJSON() ([]byte, error) {
	type NoMethod Date
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// DateFilter: This filter defines the allowed dates or date ranges for
// the media returned.
// It is possible to pick a set of specific dates and a set of date
// ranges.
type DateFilter struct {
	// Dates: List of dates that the media items must have been created on.
	// There is a
	// maximum of 5 dates that can be included per request.
	Dates []*Date `json:"dates,omitempty"`

	// Ranges: List of dates ranges that the media items must have been
	// created in. There
	// is a maximum of 5 dates ranges that can be included per request.
	Ranges []*DateRange `json:"ranges,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Dates") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Dates") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *DateFilter) MarshalJSON() ([]byte, error) {
	type NoMethod DateFilter
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// DateRange: Defines a range of dates. Both dates must be of the same
// format (see Date
// definition for more).
type DateRange struct {
	// EndDate: The end date (included as part of the range) in the same
	// format as the
	// start date.
	EndDate *Date `json:"endDate,omitempty"`

	// StartDate: The start date (included as part of the range) in one of
	// the formats
	// described.
	StartDate *Date `json:"startDate,omitempty"`

	// ForceSendFields is a list of field names (e.g. "EndDate") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "EndDate") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *DateRange) MarshalJSON() ([]byte, error) {
	type NoMethod DateRange
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// EnrichmentItem: An enrichment item.
type EnrichmentItem struct {
	// Id: Identifier of the enrichment item.
	Id string `json:"id,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Id") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Id") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *EnrichmentItem) MarshalJSON() ([]byte, error) {
	type NoMethod EnrichmentItem
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Filters: Filters that can be applied to a media item search.
// If multiple filter options are specified, they are treated as AND
// with each
// other.
type Filters struct {
	// ContentFilter: Filters the media items based on their content.
	ContentFilter *ContentFilter `json:"contentFilter,omitempty"`

	// DateFilter: Filters the media items based on their creation date.
	DateFilter *DateFilter `json:"dateFilter,omitempty"`

	// IncludeArchivedMedia: If set, the results will include media items
	// that the user has archived.
	// Defaults to false (archived media items are not included).
	IncludeArchivedMedia bool `json:"includeArchivedMedia,omitempty"`

	// MediaTypeFilter: Filters the media items based on the type of media.
	MediaTypeFilter *MediaTypeFilter `json:"mediaTypeFilter,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ContentFilter") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ContentFilter") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *Filters) MarshalJSON() ([]byte, error) {
	type NoMethod Filters
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// JoinSharedAlbumRequest: Request to join a shared album on behalf of
// the user. This uses a shareToken
// which can be acquired via the shareAlbum or listSharedAlbums calls.
type JoinSharedAlbumRequest struct {
	// ShareToken: Token indicating the shared album to join on behalf of
	// the user.
	ShareToken string `json:"shareToken,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ShareToken") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ShareToken") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *JoinSharedAlbumRequest) MarshalJSON() ([]byte, error) {
	type NoMethod JoinSharedAlbumRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// JoinSharedAlbumResponse: Response to successfully joining the shared
// album on behalf of the user.
type JoinSharedAlbumResponse struct {
	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`
}

// LatLng: An object representing a latitude/longitude pair. This is
// expressed as a pair
// of doubles representing degrees latitude and degrees longitude.
// Unless
// specified otherwise, this must conform to the
// <a
// href="http://www.unoosa.org/pdf/icg/2012/template/WGS_84.pdf">WGS84
// st
// andard</a>. Values must be within normalized ranges.
type LatLng struct {
	// Latitude: The latitude in degrees. It must be in the range [-90.0,
	// +90.0].
	Latitude float64 `json:"latitude,omitempty"`

	// Longitude: The longitude in degrees. It must be in the range [-180.0,
	// +180.0].
	Longitude float64 `json:"longitude,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Latitude") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Latitude") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *LatLng) MarshalJSON() ([]byte, error) {
	type NoMethod LatLng
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *LatLng) UnmarshalJSON(data []byte) error {
	type NoMethod LatLng
	var s1 struct {
		Latitude  gensupport.JSONFloat64 `json:"latitude"`
		Longitude gensupport.JSONFloat64 `json:"longitude"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Latitude = float64(s1.Latitude)
	s.Longitude = float64(s1.Longitude)
	return nil
}

type ListAlbumsResponse struct {
	// Albums: [Output only] List of albums that were created by the user.
	Albums []*Album `json:"albums,omitempty"`

	// NextPageToken: [Output only] Token to use to get the next set of
	// albums. Populated if
	// there are more albums to retrieve for this request.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Albums") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Albums") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *ListAlbumsResponse) MarshalJSON() ([]byte, error) {
	type NoMethod ListAlbumsResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type ListSharedAlbumsResponse struct {
	// NextPageToken: [Output only] Token to use to get the next set of
	// shared albums. Populated
	// if there are more shared albums to retrieve for this request.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// SharedAlbums: [Output only] List of shared albums that were
	// requested.
	SharedAlbums []*Album `json:"sharedAlbums,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "NextPageToken") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "NextPageToken") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *ListSharedAlbumsResponse) MarshalJSON() ([]byte, error) {
	type NoMethod ListSharedAlbumsResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Location: Represents a physical location.
type Location struct {
	// Latlng: Position of the location on the map.
	Latlng *LatLng `json:"latlng,omitempty"`

	// LocationName: Name of the location to be displayed.
	LocationName string `json:"locationName,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Latlng") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Latlng") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *Location) MarshalJSON() ([]byte, error) {
	type NoMethod Location
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// LocationEnrichment: An enrichment containing a single location.
type LocationEnrichment struct {
	// Location: Location for this enrichment item.
	Location *Location `json:"location,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Location") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Location") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *LocationEnrichment) MarshalJSON() ([]byte, error) {
	type NoMethod LocationEnrichment
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// MapEnrichment: An enrichment containing a map, showing origin and
// destination locations.
type MapEnrichment struct {
	// Destination: Destination location for this enrichemt item.
	Destination *Location `json:"destination,omitempty"`

	// Origin: Origin location for this enrichment item.
	Origin *Location `json:"origin,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Destination") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Destination") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *MapEnrichment) MarshalJSON() ([]byte, error) {
	type NoMethod MapEnrichment
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// MediaItem: Representation of a media item (e.g. photo, video etc.) in
// Google Photos.
type MediaItem struct {
	// BaseUrl: A URL to the media item's bytes. This should not be used as
	// is.
	// For example, '=w2048-h1024' will set the dimensions of a media item
	// of type
	// photo to have a width of 2048 px and height of 1024 px.
	BaseUrl string `json:"baseUrl,omitempty"`

	// ContributorInfo: Information about the user who created this media
	// item.
	ContributorInfo *ContributorInfo `json:"contributorInfo,omitempty"`

	// Description: Description of the media item. This is shown to the user
	// in the item's
	// info section in the Google Photos app.
	Description string `json:"description,omitempty"`

	// Id: Identifier for the media item. This is a persistent identifier
	// that can be
	// used between sessions to identify this media item.
	Id string `json:"id,omitempty"`

	// MediaMetadata: Metadata related to the media item, for example the
	// height, width or
	// creation time.
	MediaMetadata *MediaMetadata `json:"mediaMetadata,omitempty"`

	// MimeType: MIME type of the media item.
	MimeType string `json:"mimeType,omitempty"`

	// ProductUrl: Google Photos URL for the media item. This link will only
	// be available to
	// the user if they're signed in.
	ProductUrl string `json:"productUrl,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "BaseUrl") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BaseUrl") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *MediaItem) MarshalJSON() ([]byte, error) {
	type NoMethod MediaItem
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// MediaMetadata: Metadata for a media item.
type MediaMetadata struct {
	// CreationTime: Time when the media item was first created (not when it
	// was uploaded to
	// Google Photos).
	CreationTime string `json:"creationTime,omitempty"`

	// Height: Original height (in pixels) of the media item.
	Height int64 `json:"height,omitempty,string"`

	// Photo: Metadata for a photo media type.
	Photo *Photo `json:"photo,omitempty"`

	// Video: Metadata for a video media type.
	Video *Video `json:"video,omitempty"`

	// Width: Original width (in pixels) of the media item.
	Width int64 `json:"width,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "CreationTime") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CreationTime") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *MediaMetadata) MarshalJSON() ([]byte, error) {
	type NoMethod MediaMetadata
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// MediaTypeFilter: This filter defines the type of media items to be
// returned, for example
// videos or photos. All the specified media types are treated as an OR
// with
// each other.
type MediaTypeFilter struct {
	// MediaTypes: The types of media items to be included. This field
	// should only be
	// populated with one media type, multiple media types will result in an
	// error
	// response.
	//
	// Possible values:
	//   "ALL_MEDIA" - Treated as if no filters are applied. All media types
	// are included.
	//   "VIDEO" - All media items that are considered videos.
	// This also includes movies the user has created using the Google
	// Photos app.
	//   "PHOTO" - All media items that are considered photos. This includes
	// .bmp, .gif, .ico,
	// .jpg (and other spellings), .tiff, .webp as well as special photo
	// types
	// such as iOS live photos, Android motion photos, panoramas,
	// photospheres.
	MediaTypes []string `json:"mediaTypes,omitempty"`

	// ForceSendFields is a list of field names (e.g. "MediaTypes") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "MediaTypes") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *MediaTypeFilter) MarshalJSON() ([]byte, error) {
	type NoMethod MediaTypeFilter
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// NewEnrichmentItem: A new enrichment item to be added to an album,
// used by the
// AddEnrichmentToAlbum call.
type NewEnrichmentItem struct {
	// LocationEnrichment: Location to be added to the album.
	LocationEnrichment *LocationEnrichment `json:"locationEnrichment,omitempty"`

	// MapEnrichment: Map to be added to the album.
	MapEnrichment *MapEnrichment `json:"mapEnrichment,omitempty"`

	// TextEnrichment: Text to be added to the album.
	TextEnrichment *TextEnrichment `json:"textEnrichment,omitempty"`

	// ForceSendFields is a list of field names (e.g. "LocationEnrichment")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "LocationEnrichment") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *NewEnrichmentItem) MarshalJSON() ([]byte, error) {
	type NoMethod NewEnrichmentItem
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// NewMediaItem: New media item that will be created in a user's Google
// Photos account.
type NewMediaItem struct {
	// Description: Description of the media item. This will be shown to the
	// user in the item's
	// info section in the Google Photos app.
	// This string should not be more than 1000 characters.
	Description string `json:"description,omitempty"`

	// SimpleMediaItem: A new media item that has been uploaded via the
	// included uploadToken.
	SimpleMediaItem *SimpleMediaItem `json:"simpleMediaItem,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Description") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Description") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *NewMediaItem) MarshalJSON() ([]byte, error) {
	type NoMethod NewMediaItem
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// NewMediaItemResult: Result of creating a new media item.
type NewMediaItemResult struct {
	// MediaItem: Media item created with the upload token. It is populated
	// if no errors
	// occurred and the media item was created successfully.
	MediaItem *MediaItem `json:"mediaItem,omitempty"`

	// Status: If an error occurred during the creation of this media item,
	// this field
	// will be populated with information related to the error. Details of
	// this
	// status can be found down below.
	Status *Status `json:"status,omitempty"`

	// UploadToken: The upload token used to create this new media item.
	UploadToken string `json:"uploadToken,omitempty"`

	// ForceSendFields is a list of field names (e.g. "MediaItem") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "MediaItem") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *NewMediaItemResult) MarshalJSON() ([]byte, error) {
	type NoMethod NewMediaItemResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Photo: Metadata that is specific to a photo, for example, ISO, focal
// length and
// exposure time. Some of these fields may be null or not included.
type Photo struct {
	// ApertureFNumber: Apeture f number of the photo.
	ApertureFNumber float64 `json:"apertureFNumber,omitempty"`

	// CameraMake: Brand of the camera which took the photo.
	CameraMake string `json:"cameraMake,omitempty"`

	// CameraModel: Model of the camera which took the photo.
	CameraModel string `json:"cameraModel,omitempty"`

	// ExposureTime: Exposure time of the photo.
	ExposureTime string `json:"exposureTime,omitempty"`

	// FocalLength: Focal length of the photo.
	FocalLength float64 `json:"focalLength,omitempty"`

	// IsoEquivalent: ISO of the photo.
	IsoEquivalent int64 `json:"isoEquivalent,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ApertureFNumber") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ApertureFNumber") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *Photo) MarshalJSON() ([]byte, error) {
	type NoMethod Photo
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *Photo) UnmarshalJSON(data []byte) error {
	type NoMethod Photo
	var s1 struct {
		ApertureFNumber gensupport.JSONFloat64 `json:"apertureFNumber"`
		FocalLength     gensupport.JSONFloat64 `json:"focalLength"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.ApertureFNumber = float64(s1.ApertureFNumber)
	s.FocalLength = float64(s1.FocalLength)
	return nil
}

// SearchMediaItemsRequest: Request to search for media items in a
// user's library.
//
// If the album id is specified, this call will return the list of media
// items
// in the album. If neither filters nor album id are
// specified, this call will return all media items in a user's Google
// Photos
// library.
//
// If filters are specified, this call will return all media items
// in
// the user's library which fulfills the criteria based upon the
// filters.
//
// Filters and album id must not both be set, as this will result in
// an
// invalid request.
type SearchMediaItemsRequest struct {
	// AlbumId: Identifier of an album. If populated will list all media
	// items in
	// specified album. Cannot be set in conjunction with any filters.
	AlbumId string `json:"albumId,omitempty"`

	// Filters: Filters to apply to the request. Cannot be set in conjuction
	// with an
	// albumId.
	Filters *Filters `json:"filters,omitempty"`

	// PageSize: Maximum number of media items to return in the response.
	// The default number
	// of media items to return at a time is 100. The maximum page size is
	// 500.
	PageSize int64 `json:"pageSize,omitempty"`

	// PageToken: A continuation token to get the next page of the results.
	// Adding this to
	// the request will return the rows after the pageToken. The pageToken
	// should
	// be the value returned in the nextPageToken parameter in the response
	// to the
	// searchMediaItems request.
	PageToken string `json:"pageToken,omitempty"`

	// ForceSendFields is a list of field names (e.g. "AlbumId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "AlbumId") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *SearchMediaItemsRequest) MarshalJSON() ([]byte, error) {
	type NoMethod SearchMediaItemsRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type SearchMediaItemsResponse struct {
	// MediaItems: [Output only] List of media items that match the search
	// parameters.
	MediaItems []*MediaItem `json:"mediaItems,omitempty"`

	// NextPageToken: [Output only] Token to use to get the next set of
	// media items. Its presence
	// is the only reliable indicator of more media items being available in
	// the
	// next request.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "MediaItems") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "MediaItems") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *SearchMediaItemsResponse) MarshalJSON() ([]byte, error) {
	type NoMethod SearchMediaItemsResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// ShareAlbumRequest: Request to make an album shared in Google Photos.
type ShareAlbumRequest struct {
	// SharedAlbumOptions: Options to be set when converting the album to a
	// shared album.
	SharedAlbumOptions *SharedAlbumOptions `json:"sharedAlbumOptions,omitempty"`

	// ForceSendFields is a list of field names (e.g. "SharedAlbumOptions")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "SharedAlbumOptions") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *ShareAlbumRequest) MarshalJSON() ([]byte, error) {
	type NoMethod ShareAlbumRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type ShareAlbumResponse struct {
	// ShareInfo: [Output only] Information about the shared album.
	ShareInfo *ShareInfo `json:"shareInfo,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "ShareInfo") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ShareInfo") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *ShareAlbumResponse) MarshalJSON() ([]byte, error) {
	type NoMethod ShareAlbumResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// ShareInfo: Information about albums that are shared. Note that
// this
// information is only included if the album was created by you and you
// have the
// sharing scope.
type ShareInfo struct {
	// ShareToken: A token which can be used to join this shared album on
	// behalf of other
	// users via the API.
	ShareToken string `json:"shareToken,omitempty"`

	// ShareableUrl: A link to the album that's now shared on the Google
	// Photos website and app.
	// Anyone with the link can access this shared album and see all of the
	// items
	// present in the album.
	ShareableUrl string `json:"shareableUrl,omitempty"`

	// SharedAlbumOptions: Options set for the shared album.
	SharedAlbumOptions *SharedAlbumOptions `json:"sharedAlbumOptions,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ShareToken") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ShareToken") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *ShareInfo) MarshalJSON() ([]byte, error) {
	type NoMethod ShareInfo
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// SharedAlbumOptions: Options that control the sharing of an album.
type SharedAlbumOptions struct {
	// IsCollaborative: True if the shared album allows collaborators (users
	// who have joined
	// the album) to add media items to it. Defaults to false.
	IsCollaborative bool `json:"isCollaborative,omitempty"`

	// IsCommentable: True if the shared album allows the owner and the
	// collaborators (users
	// who have joined the album) to add comments to the album. Defaults to
	// false.
	IsCommentable bool `json:"isCommentable,omitempty"`

	// ForceSendFields is a list of field names (e.g. "IsCollaborative") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "IsCollaborative") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *SharedAlbumOptions) MarshalJSON() ([]byte, error) {
	type NoMethod SharedAlbumOptions
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// SimpleMediaItem: A simple media item to be created in Google Photos
// via an upload token.
type SimpleMediaItem struct {
	// UploadToken: Token identifying the media bytes which have been
	// uploaded to Google.
	UploadToken string `json:"uploadToken,omitempty"`

	// ForceSendFields is a list of field names (e.g. "UploadToken") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "UploadToken") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *SimpleMediaItem) MarshalJSON() ([]byte, error) {
	type NoMethod SimpleMediaItem
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Status: The `Status` type defines a logical error model that is
// suitable for different
// programming environments, including REST APIs and RPC APIs. It is
// used by
// [gRPC](https://github.com/grpc). The error model is designed to
// be:
//
// - Simple to use and understand for most users
// - Flexible enough to meet unexpected needs
//
// # Overview
//
// The `Status` message contains three pieces of data: error code, error
// message,
// and error details. The error code should be an enum value
// of
// google.rpc.Code, but it may accept additional error codes if needed.
// The
// error message should be a developer-facing English message that
// helps
// developers *understand* and *resolve* the error. If a localized
// user-facing
// error message is needed, put the localized message in the error
// details or
// localize it in the client. The optional error details may contain
// arbitrary
// information about the error. There is a predefined set of error
// detail types
// in the package `google.rpc` that can be used for common error
// conditions.
//
// # Language mapping
//
// The `Status` message is the logical representation of the error
// model, but it
// is not necessarily the actual wire format. When the `Status` message
// is
// exposed in different client libraries and different wire protocols,
// it can be
// mapped differently. For example, it will likely be mapped to some
// exceptions
// in Java, but more likely mapped to some error codes in C.
//
// # Other uses
//
// The error model and the `Status` message can be used in a variety
// of
// environments, either with or without APIs, to provide a
// consistent developer experience across different
// environments.
//
// Example uses of this error model include:
//
// - Partial errors. If a service needs to return partial errors to the
// client,
//     it may embed the `Status` in the normal response to indicate the
// partial
//     errors.
//
// - Workflow errors. A typical workflow has multiple steps. Each step
// may
//     have a `Status` message for error reporting.
//
// - Batch operations. If a client uses batch request and batch
// response, the
//     `Status` message should be used directly inside batch response,
// one for
//     each error sub-response.
//
// - Asynchronous operations. If an API call embeds asynchronous
// operation
//     results in its response, the status of those operations should
// be
//     represented directly using the `Status` message.
//
// - Logging. If some API errors are stored in logs, the message
// `Status` could
//     be used directly after any stripping needed for security/privacy
// reasons.
type Status struct {
	// Code: The status code, which should be an enum value of
	// google.rpc.Code.
	Code int64 `json:"code,omitempty"`

	// Details: A list of messages that carry the error details.  There is a
	// common set of
	// message types for APIs to use.
	Details []googleapi.RawMessage `json:"details,omitempty"`

	// Message: A developer-facing error message, which should be in
	// English. Any
	// user-facing error message should be localized and sent in
	// the
	// google.rpc.Status.details field, or localized by the client.
	Message string `json:"message,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Code") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Code") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *Status) MarshalJSON() ([]byte, error) {
	type NoMethod Status
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// TextEnrichment: An enrichment containing text.
type TextEnrichment struct {
	// Text: Text for this text enrichment item.
	Text string `json:"text,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Text") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Text") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *TextEnrichment) MarshalJSON() ([]byte, error) {
	type NoMethod TextEnrichment
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Video: Metadata that is specific to a video, for example, fps and
// processing status.
// Some of these fields may be null or not included.
type Video struct {
	// CameraMake: Brand of the camera which took the video.
	CameraMake string `json:"cameraMake,omitempty"`

	// CameraModel: Model of the camera which took the video.
	CameraModel string `json:"cameraModel,omitempty"`

	// Fps: Frame rate of the video.
	Fps float64 `json:"fps,omitempty"`

	// Status: Processing status of the video.
	//
	// Possible values:
	//   "UNSPECIFIED" - Video processing status is unknown.
	//   "PROCESSING" - Video is currently being processed. The user will
	// see an icon for this
	// video in the Google Photos app, however, it will not be playable yet.
	//   "READY" - Video is now ready for viewing.
	//   "FAILED" - Something has gone wrong and the video has failed to
	// process.
	Status string `json:"status,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CameraMake") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CameraMake") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *Video) MarshalJSON() ([]byte, error) {
	type NoMethod Video
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *Video) UnmarshalJSON(data []byte) error {
	type NoMethod Video
	var s1 struct {
		Fps gensupport.JSONFloat64 `json:"fps"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Fps = float64(s1.Fps)
	return nil
}

// method id "photoslibrary.albums.addEnrichment":

type AlbumsAddEnrichmentCall struct {
	s                           *Service
	albumId                     string
	addenrichmenttoalbumrequest *AddEnrichmentToAlbumRequest
	urlParams_                  gensupport.URLParams
	ctx_                        context.Context
	header_                     http.Header
}

// AddEnrichment: Adds an enrichment to a specified position in a
// defined album.
func (r *AlbumsService) AddEnrichment(albumId string, addenrichmenttoalbumrequest *AddEnrichmentToAlbumRequest) *AlbumsAddEnrichmentCall {
	c := &AlbumsAddEnrichmentCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.albumId = albumId
	c.addenrichmenttoalbumrequest = addenrichmenttoalbumrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *AlbumsAddEnrichmentCall) Fields(s ...googleapi.Field) *AlbumsAddEnrichmentCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *AlbumsAddEnrichmentCall) Context(ctx context.Context) *AlbumsAddEnrichmentCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *AlbumsAddEnrichmentCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *AlbumsAddEnrichmentCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.addenrichmenttoalbumrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/albums/{+albumId}:addEnrichment")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"albumId": c.albumId,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "photoslibrary.albums.addEnrichment" call.
// Exactly one of *AddEnrichmentToAlbumResponse or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *AddEnrichmentToAlbumResponse.ServerResponse.Header or (if a
// response was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *AlbumsAddEnrichmentCall) Do(opts ...googleapi.CallOption) (*AddEnrichmentToAlbumResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &AddEnrichmentToAlbumResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Adds an enrichment to a specified position in a defined album.",
	//   "flatPath": "v1/albums/{albumsId}:addEnrichment",
	//   "httpMethod": "POST",
	//   "id": "photoslibrary.albums.addEnrichment",
	//   "parameterOrder": [
	//     "albumId"
	//   ],
	//   "parameters": {
	//     "albumId": {
	//       "description": "Identifier of the album where the enrichment will be added.",
	//       "location": "path",
	//       "pattern": "^[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/albums/{+albumId}:addEnrichment",
	//   "request": {
	//     "$ref": "AddEnrichmentToAlbumRequest"
	//   },
	//   "response": {
	//     "$ref": "AddEnrichmentToAlbumResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/photoslibrary",
	//     "https://www.googleapis.com/auth/photoslibrary.appendonly",
	//     "https://www.googleapis.com/auth/photoslibrary.sharing"
	//   ]
	// }

}

// method id "photoslibrary.albums.create":

type AlbumsCreateCall struct {
	s                  *Service
	createalbumrequest *CreateAlbumRequest
	urlParams_         gensupport.URLParams
	ctx_               context.Context
	header_            http.Header
}

// Create: Creates an album in a user's Google Photos library.
func (r *AlbumsService) Create(createalbumrequest *CreateAlbumRequest) *AlbumsCreateCall {
	c := &AlbumsCreateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.createalbumrequest = createalbumrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *AlbumsCreateCall) Fields(s ...googleapi.Field) *AlbumsCreateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *AlbumsCreateCall) Context(ctx context.Context) *AlbumsCreateCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *AlbumsCreateCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *AlbumsCreateCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.createalbumrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/albums")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "photoslibrary.albums.create" call.
// Exactly one of *Album or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Album.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *AlbumsCreateCall) Do(opts ...googleapi.CallOption) (*Album, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &Album{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Creates an album in a user's Google Photos library.",
	//   "flatPath": "v1/albums",
	//   "httpMethod": "POST",
	//   "id": "photoslibrary.albums.create",
	//   "parameterOrder": [],
	//   "parameters": {},
	//   "path": "v1/albums",
	//   "request": {
	//     "$ref": "CreateAlbumRequest"
	//   },
	//   "response": {
	//     "$ref": "Album"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/photoslibrary",
	//     "https://www.googleapis.com/auth/photoslibrary.appendonly",
	//     "https://www.googleapis.com/auth/photoslibrary.sharing"
	//   ]
	// }

}

// method id "photoslibrary.albums.get":

type AlbumsGetCall struct {
	s            *Service
	albumId      string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// Get: Returns the album specified by the given album id.
func (r *AlbumsService) Get(albumId string) *AlbumsGetCall {
	c := &AlbumsGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.albumId = albumId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *AlbumsGetCall) Fields(s ...googleapi.Field) *AlbumsGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *AlbumsGetCall) IfNoneMatch(entityTag string) *AlbumsGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *AlbumsGetCall) Context(ctx context.Context) *AlbumsGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *AlbumsGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *AlbumsGetCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		reqHeaders.Set("If-None-Match", c.ifNoneMatch_)
	}
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/albums/{+albumId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"albumId": c.albumId,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "photoslibrary.albums.get" call.
// Exactly one of *Album or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Album.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *AlbumsGetCall) Do(opts ...googleapi.CallOption) (*Album, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &Album{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Returns the album specified by the given album id.",
	//   "flatPath": "v1/albums/{albumsId}",
	//   "httpMethod": "GET",
	//   "id": "photoslibrary.albums.get",
	//   "parameterOrder": [
	//     "albumId"
	//   ],
	//   "parameters": {
	//     "albumId": {
	//       "description": "Identifier of the album to be requested.",
	//       "location": "path",
	//       "pattern": "^[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/albums/{+albumId}",
	//   "response": {
	//     "$ref": "Album"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/drive.photos.readonly",
	//     "https://www.googleapis.com/auth/photoslibrary",
	//     "https://www.googleapis.com/auth/photoslibrary.readonly",
	//     "https://www.googleapis.com/auth/photoslibrary.readonly.appcreateddata"
	//   ]
	// }

}

// method id "photoslibrary.albums.list":

type AlbumsListCall struct {
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists all albums shown to a user in the 'Albums' tab of the
// Google
// Photos app.
func (r *AlbumsService) List() *AlbumsListCall {
	c := &AlbumsListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// PageSize sets the optional parameter "pageSize": Maximum number of
// albums to return in the response. The default number of
// albums to return at a time is 20. The maximum page size is 50.
func (c *AlbumsListCall) PageSize(pageSize int64) *AlbumsListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": A continuation
// token to get the next page of the results. Adding this to
// the request will return the rows after the pageToken. The pageToken
// should
// be the value returned in the nextPageToken parameter in the response
// to the
// listAlbums request.
func (c *AlbumsListCall) PageToken(pageToken string) *AlbumsListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *AlbumsListCall) Fields(s ...googleapi.Field) *AlbumsListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *AlbumsListCall) IfNoneMatch(entityTag string) *AlbumsListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *AlbumsListCall) Context(ctx context.Context) *AlbumsListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *AlbumsListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *AlbumsListCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		reqHeaders.Set("If-None-Match", c.ifNoneMatch_)
	}
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/albums")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "photoslibrary.albums.list" call.
// Exactly one of *ListAlbumsResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *ListAlbumsResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *AlbumsListCall) Do(opts ...googleapi.CallOption) (*ListAlbumsResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ListAlbumsResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Lists all albums shown to a user in the 'Albums' tab of the Google\nPhotos app.",
	//   "flatPath": "v1/albums",
	//   "httpMethod": "GET",
	//   "id": "photoslibrary.albums.list",
	//   "parameterOrder": [],
	//   "parameters": {
	//     "pageSize": {
	//       "description": "Maximum number of albums to return in the response. The default number of\nalbums to return at a time is 20. The maximum page size is 50.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "description": "A continuation token to get the next page of the results. Adding this to\nthe request will return the rows after the pageToken. The pageToken should\nbe the value returned in the nextPageToken parameter in the response to the\nlistAlbums request.",
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/albums",
	//   "response": {
	//     "$ref": "ListAlbumsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/drive.photos.readonly",
	//     "https://www.googleapis.com/auth/photoslibrary",
	//     "https://www.googleapis.com/auth/photoslibrary.readonly",
	//     "https://www.googleapis.com/auth/photoslibrary.readonly.appcreateddata"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *AlbumsListCall) Pages(ctx context.Context, f func(*ListAlbumsResponse) error) error {
	c.ctx_ = ctx
	defer c.PageToken(c.urlParams_.Get("pageToken")) // reset paging to original point
	for {
		x, err := c.Do()
		if err != nil {
			return err
		}
		if err := f(x); err != nil {
			return err
		}
		if x.NextPageToken == "" {
			return nil
		}
		c.PageToken(x.NextPageToken)
	}
}

// method id "photoslibrary.albums.share":

type AlbumsShareCall struct {
	s                 *Service
	albumId           string
	sharealbumrequest *ShareAlbumRequest
	urlParams_        gensupport.URLParams
	ctx_              context.Context
	header_           http.Header
}

// Share: Marks an album as 'shared' and accessible to other users. This
// action can
// only be performed on albums which were created by the developer via
// the
// API.
func (r *AlbumsService) Share(albumId string, sharealbumrequest *ShareAlbumRequest) *AlbumsShareCall {
	c := &AlbumsShareCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.albumId = albumId
	c.sharealbumrequest = sharealbumrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *AlbumsShareCall) Fields(s ...googleapi.Field) *AlbumsShareCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *AlbumsShareCall) Context(ctx context.Context) *AlbumsShareCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *AlbumsShareCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *AlbumsShareCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.sharealbumrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/albums/{+albumId}:share")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"albumId": c.albumId,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "photoslibrary.albums.share" call.
// Exactly one of *ShareAlbumResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *ShareAlbumResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *AlbumsShareCall) Do(opts ...googleapi.CallOption) (*ShareAlbumResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ShareAlbumResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Marks an album as 'shared' and accessible to other users. This action can\nonly be performed on albums which were created by the developer via the\nAPI.",
	//   "flatPath": "v1/albums/{albumsId}:share",
	//   "httpMethod": "POST",
	//   "id": "photoslibrary.albums.share",
	//   "parameterOrder": [
	//     "albumId"
	//   ],
	//   "parameters": {
	//     "albumId": {
	//       "description": "Identifier of the album to be shared. This album id must belong to an album\ncreated by the developer.\n.",
	//       "location": "path",
	//       "pattern": "^[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/albums/{+albumId}:share",
	//   "request": {
	//     "$ref": "ShareAlbumRequest"
	//   },
	//   "response": {
	//     "$ref": "ShareAlbumResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/photoslibrary.sharing"
	//   ]
	// }

}

// method id "photoslibrary.mediaItems.batchCreate":

type MediaItemsBatchCreateCall struct {
	s                            *Service
	batchcreatemediaitemsrequest *BatchCreateMediaItemsRequest
	urlParams_                   gensupport.URLParams
	ctx_                         context.Context
	header_                      http.Header
}

// BatchCreate: Creates one or more media items in a user's Google
// Photos library.
// If an album id is specified, the media item(s) are also added to the
// album.
// By default the media item(s) will be added to the end of the library
// or
// album.
//
// If an album id and position are both defined, then the media items
// will
// be added to the album at the specified position.
//
// If multiple media items are given, they will be inserted at the
// specified
// position.
func (r *MediaItemsService) BatchCreate(batchcreatemediaitemsrequest *BatchCreateMediaItemsRequest) *MediaItemsBatchCreateCall {
	c := &MediaItemsBatchCreateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.batchcreatemediaitemsrequest = batchcreatemediaitemsrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *MediaItemsBatchCreateCall) Fields(s ...googleapi.Field) *MediaItemsBatchCreateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *MediaItemsBatchCreateCall) Context(ctx context.Context) *MediaItemsBatchCreateCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *MediaItemsBatchCreateCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *MediaItemsBatchCreateCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.batchcreatemediaitemsrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/mediaItems:batchCreate")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "photoslibrary.mediaItems.batchCreate" call.
// Exactly one of *BatchCreateMediaItemsResponse or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *BatchCreateMediaItemsResponse.ServerResponse.Header or (if a
// response was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *MediaItemsBatchCreateCall) Do(opts ...googleapi.CallOption) (*BatchCreateMediaItemsResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &BatchCreateMediaItemsResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Creates one or more media items in a user's Google Photos library.\nIf an album id is specified, the media item(s) are also added to the album.\nBy default the media item(s) will be added to the end of the library or\nalbum.\n\nIf an album id and position are both defined, then the media items will\nbe added to the album at the specified position.\n\nIf multiple media items are given, they will be inserted at the specified\nposition.",
	//   "flatPath": "v1/mediaItems:batchCreate",
	//   "httpMethod": "POST",
	//   "id": "photoslibrary.mediaItems.batchCreate",
	//   "parameterOrder": [],
	//   "parameters": {},
	//   "path": "v1/mediaItems:batchCreate",
	//   "request": {
	//     "$ref": "BatchCreateMediaItemsRequest"
	//   },
	//   "response": {
	//     "$ref": "BatchCreateMediaItemsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/photoslibrary",
	//     "https://www.googleapis.com/auth/photoslibrary.appendonly",
	//     "https://www.googleapis.com/auth/photoslibrary.sharing"
	//   ]
	// }

}

// method id "photoslibrary.mediaItems.get":

type MediaItemsGetCall struct {
	s            *Service
	mediaItemId  string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// Get: Returns the media item specified based on a given media item id.
func (r *MediaItemsService) Get(mediaItemId string) *MediaItemsGetCall {
	c := &MediaItemsGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.mediaItemId = mediaItemId
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *MediaItemsGetCall) Fields(s ...googleapi.Field) *MediaItemsGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *MediaItemsGetCall) IfNoneMatch(entityTag string) *MediaItemsGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *MediaItemsGetCall) Context(ctx context.Context) *MediaItemsGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *MediaItemsGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *MediaItemsGetCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		reqHeaders.Set("If-None-Match", c.ifNoneMatch_)
	}
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/mediaItems/{+mediaItemId}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"mediaItemId": c.mediaItemId,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "photoslibrary.mediaItems.get" call.
// Exactly one of *MediaItem or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *MediaItem.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *MediaItemsGetCall) Do(opts ...googleapi.CallOption) (*MediaItem, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &MediaItem{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Returns the media item specified based on a given media item id.",
	//   "flatPath": "v1/mediaItems/{mediaItemsId}",
	//   "httpMethod": "GET",
	//   "id": "photoslibrary.mediaItems.get",
	//   "parameterOrder": [
	//     "mediaItemId"
	//   ],
	//   "parameters": {
	//     "mediaItemId": {
	//       "description": "Identifier of media item to be requested.",
	//       "location": "path",
	//       "pattern": "^[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/mediaItems/{+mediaItemId}",
	//   "response": {
	//     "$ref": "MediaItem"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/drive.photos.readonly",
	//     "https://www.googleapis.com/auth/photoslibrary",
	//     "https://www.googleapis.com/auth/photoslibrary.readonly",
	//     "https://www.googleapis.com/auth/photoslibrary.readonly.appcreateddata"
	//   ]
	// }

}

// method id "photoslibrary.mediaItems.search":

type MediaItemsSearchCall struct {
	s                       *Service
	searchmediaitemsrequest *SearchMediaItemsRequest
	urlParams_              gensupport.URLParams
	ctx_                    context.Context
	header_                 http.Header
}

// Search: Searches for media items in a user's Google Photos
// library.
// If no filters are set, then all media items in the user's library
// will be
// returned.
//
// If an album is set, all media items in the specified album will
// be
// returned.
//
// If filters are specified, anything that matches the filters from the
// user's
// library will be listed.
//
// If an album and filters are set, then this will result in an error.
func (r *MediaItemsService) Search(searchmediaitemsrequest *SearchMediaItemsRequest) *MediaItemsSearchCall {
	c := &MediaItemsSearchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.searchmediaitemsrequest = searchmediaitemsrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *MediaItemsSearchCall) Fields(s ...googleapi.Field) *MediaItemsSearchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *MediaItemsSearchCall) Context(ctx context.Context) *MediaItemsSearchCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *MediaItemsSearchCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *MediaItemsSearchCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.searchmediaitemsrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/mediaItems:search")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "photoslibrary.mediaItems.search" call.
// Exactly one of *SearchMediaItemsResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *SearchMediaItemsResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *MediaItemsSearchCall) Do(opts ...googleapi.CallOption) (*SearchMediaItemsResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &SearchMediaItemsResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Searches for media items in a user's Google Photos library.\nIf no filters are set, then all media items in the user's library will be\nreturned.\n\nIf an album is set, all media items in the specified album will be\nreturned.\n\nIf filters are specified, anything that matches the filters from the user's\nlibrary will be listed.\n\nIf an album and filters are set, then this will result in an error.",
	//   "flatPath": "v1/mediaItems:search",
	//   "httpMethod": "POST",
	//   "id": "photoslibrary.mediaItems.search",
	//   "parameterOrder": [],
	//   "parameters": {},
	//   "path": "v1/mediaItems:search",
	//   "request": {
	//     "$ref": "SearchMediaItemsRequest"
	//   },
	//   "response": {
	//     "$ref": "SearchMediaItemsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/drive.photos.readonly",
	//     "https://www.googleapis.com/auth/photoslibrary",
	//     "https://www.googleapis.com/auth/photoslibrary.readonly",
	//     "https://www.googleapis.com/auth/photoslibrary.readonly.appcreateddata"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *MediaItemsSearchCall) Pages(ctx context.Context, f func(*SearchMediaItemsResponse) error) error {
	c.ctx_ = ctx
	defer func(pt string) { c.searchmediaitemsrequest.PageToken = pt }(c.searchmediaitemsrequest.PageToken) // reset paging to original point
	for {
		x, err := c.Do()
		if err != nil {
			return err
		}
		if err := f(x); err != nil {
			return err
		}
		if x.NextPageToken == "" {
			return nil
		}
		c.searchmediaitemsrequest.PageToken = x.NextPageToken
	}
}

// method id "photoslibrary.sharedAlbums.join":

type SharedAlbumsJoinCall struct {
	s                      *Service
	joinsharedalbumrequest *JoinSharedAlbumRequest
	urlParams_             gensupport.URLParams
	ctx_                   context.Context
	header_                http.Header
}

// Join: Joins a shared album on behalf of the Google Photos user.
func (r *SharedAlbumsService) Join(joinsharedalbumrequest *JoinSharedAlbumRequest) *SharedAlbumsJoinCall {
	c := &SharedAlbumsJoinCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.joinsharedalbumrequest = joinsharedalbumrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *SharedAlbumsJoinCall) Fields(s ...googleapi.Field) *SharedAlbumsJoinCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *SharedAlbumsJoinCall) Context(ctx context.Context) *SharedAlbumsJoinCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *SharedAlbumsJoinCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *SharedAlbumsJoinCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.joinsharedalbumrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/sharedAlbums:join")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "photoslibrary.sharedAlbums.join" call.
// Exactly one of *JoinSharedAlbumResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *JoinSharedAlbumResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *SharedAlbumsJoinCall) Do(opts ...googleapi.CallOption) (*JoinSharedAlbumResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &JoinSharedAlbumResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Joins a shared album on behalf of the Google Photos user.",
	//   "flatPath": "v1/sharedAlbums:join",
	//   "httpMethod": "POST",
	//   "id": "photoslibrary.sharedAlbums.join",
	//   "parameterOrder": [],
	//   "parameters": {},
	//   "path": "v1/sharedAlbums:join",
	//   "request": {
	//     "$ref": "JoinSharedAlbumRequest"
	//   },
	//   "response": {
	//     "$ref": "JoinSharedAlbumResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/photoslibrary.sharing"
	//   ]
	// }

}

// method id "photoslibrary.sharedAlbums.list":

type SharedAlbumsListCall struct {
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists all shared albums shown to a user in the 'Sharing' tab of
// the
// Google Photos app.
func (r *SharedAlbumsService) List() *SharedAlbumsListCall {
	c := &SharedAlbumsListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// PageSize sets the optional parameter "pageSize": Maximum number of
// albums to return in the response. The default number of
// albums to return at a time is 20. The maximum page size is 50.
func (c *SharedAlbumsListCall) PageSize(pageSize int64) *SharedAlbumsListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": A continuation
// token to get the next page of the results. Adding this to
// the request will return the rows after the pageToken. The pageToken
// should
// be the value returned in the nextPageToken parameter in the response
// to the
// listSharedAlbums request.
func (c *SharedAlbumsListCall) PageToken(pageToken string) *SharedAlbumsListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *SharedAlbumsListCall) Fields(s ...googleapi.Field) *SharedAlbumsListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *SharedAlbumsListCall) IfNoneMatch(entityTag string) *SharedAlbumsListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *SharedAlbumsListCall) Context(ctx context.Context) *SharedAlbumsListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *SharedAlbumsListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *SharedAlbumsListCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	if c.ifNoneMatch_ != "" {
		reqHeaders.Set("If-None-Match", c.ifNoneMatch_)
	}
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/sharedAlbums")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "photoslibrary.sharedAlbums.list" call.
// Exactly one of *ListSharedAlbumsResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *ListSharedAlbumsResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *SharedAlbumsListCall) Do(opts ...googleapi.CallOption) (*ListSharedAlbumsResponse, error) {
	gensupport.SetOptions(c.urlParams_, opts...)
	res, err := c.doRequest("json")
	if res != nil && res.StatusCode == http.StatusNotModified {
		if res.Body != nil {
			res.Body.Close()
		}
		return nil, &googleapi.Error{
			Code:   res.StatusCode,
			Header: res.Header,
		}
	}
	if err != nil {
		return nil, err
	}
	defer googleapi.CloseBody(res)
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	ret := &ListSharedAlbumsResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Lists all shared albums shown to a user in the 'Sharing' tab of the\nGoogle Photos app.",
	//   "flatPath": "v1/sharedAlbums",
	//   "httpMethod": "GET",
	//   "id": "photoslibrary.sharedAlbums.list",
	//   "parameterOrder": [],
	//   "parameters": {
	//     "pageSize": {
	//       "description": "Maximum number of albums to return in the response. The default number of\nalbums to return at a time is 20. The maximum page size is 50.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "description": "A continuation token to get the next page of the results. Adding this to\nthe request will return the rows after the pageToken. The pageToken should\nbe the value returned in the nextPageToken parameter in the response to the\nlistSharedAlbums request.",
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/sharedAlbums",
	//   "response": {
	//     "$ref": "ListSharedAlbumsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/drive.photos.readonly",
	//     "https://www.googleapis.com/auth/photoslibrary",
	//     "https://www.googleapis.com/auth/photoslibrary.readonly",
	//     "https://www.googleapis.com/auth/photoslibrary.readonly.appcreateddata"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *SharedAlbumsListCall) Pages(ctx context.Context, f func(*ListSharedAlbumsResponse) error) error {
	c.ctx_ = ctx
	defer c.PageToken(c.urlParams_.Get("pageToken")) // reset paging to original point
	for {
		x, err := c.Do()
		if err != nil {
			return err
		}
		if err := f(x); err != nil {
			return err
		}
		if x.NextPageToken == "" {
			return nil
		}
		c.PageToken(x.NextPageToken)
	}
}
