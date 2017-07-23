// Package dlp provides access to the DLP API.
//
// See https://cloud.google.com/dlp/docs/
//
// Usage example:
//
//   import "google.golang.org/api/dlp/v2beta1"
//   ...
//   dlpService, err := dlp.New(oauthHttpClient)
package dlp // import "google.golang.org/api/dlp/v2beta1"

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

const apiId = "dlp:v2beta1"
const apiName = "dlp"
const apiVersion = "v2beta1"
const basePath = "https://dlp.googleapis.com/"

// OAuth2 scopes used by this API.
const (
	// View and manage your data across Google Cloud Platform services
	CloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"
)

func New(client *http.Client) (*Service, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}
	s := &Service{client: client, BasePath: basePath}
	s.Content = NewContentService(s)
	s.Inspect = NewInspectService(s)
	s.RootCategories = NewRootCategoriesService(s)
	return s, nil
}

type Service struct {
	client    *http.Client
	BasePath  string // API endpoint base URL
	UserAgent string // optional additional User-Agent fragment

	Content *ContentService

	Inspect *InspectService

	RootCategories *RootCategoriesService
}

func (s *Service) userAgent() string {
	if s.UserAgent == "" {
		return googleapi.UserAgent
	}
	return googleapi.UserAgent + " " + s.UserAgent
}

func NewContentService(s *Service) *ContentService {
	rs := &ContentService{s: s}
	return rs
}

type ContentService struct {
	s *Service
}

func NewInspectService(s *Service) *InspectService {
	rs := &InspectService{s: s}
	rs.Operations = NewInspectOperationsService(s)
	rs.Results = NewInspectResultsService(s)
	return rs
}

type InspectService struct {
	s *Service

	Operations *InspectOperationsService

	Results *InspectResultsService
}

func NewInspectOperationsService(s *Service) *InspectOperationsService {
	rs := &InspectOperationsService{s: s}
	return rs
}

type InspectOperationsService struct {
	s *Service
}

func NewInspectResultsService(s *Service) *InspectResultsService {
	rs := &InspectResultsService{s: s}
	rs.Findings = NewInspectResultsFindingsService(s)
	return rs
}

type InspectResultsService struct {
	s *Service

	Findings *InspectResultsFindingsService
}

func NewInspectResultsFindingsService(s *Service) *InspectResultsFindingsService {
	rs := &InspectResultsFindingsService{s: s}
	return rs
}

type InspectResultsFindingsService struct {
	s *Service
}

func NewRootCategoriesService(s *Service) *RootCategoriesService {
	rs := &RootCategoriesService{s: s}
	rs.InfoTypes = NewRootCategoriesInfoTypesService(s)
	return rs
}

type RootCategoriesService struct {
	s *Service

	InfoTypes *RootCategoriesInfoTypesService
}

func NewRootCategoriesInfoTypesService(s *Service) *RootCategoriesInfoTypesService {
	rs := &RootCategoriesInfoTypesService{s: s}
	return rs
}

type RootCategoriesInfoTypesService struct {
	s *Service
}

// GoogleLongrunningCancelOperationRequest: The request message for
// Operations.CancelOperation.
type GoogleLongrunningCancelOperationRequest struct {
}

// GoogleLongrunningListOperationsResponse: The response message for
// Operations.ListOperations.
type GoogleLongrunningListOperationsResponse struct {
	// NextPageToken: The standard List next-page token.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// Operations: A list of operations that matches the specified filter in
	// the request.
	Operations []*GoogleLongrunningOperation `json:"operations,omitempty"`

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

func (s *GoogleLongrunningListOperationsResponse) MarshalJSON() ([]byte, error) {
	type noMethod GoogleLongrunningListOperationsResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleLongrunningOperation: This resource represents a long-running
// operation that is the result of a
// network API call.
type GoogleLongrunningOperation struct {
	// Done: If the value is `false`, it means the operation is still in
	// progress.
	// If true, the operation is completed, and either `error` or `response`
	// is
	// available.
	Done bool `json:"done,omitempty"`

	// Error: The error result of the operation in case of failure or
	// cancellation.
	Error *GoogleRpcStatus `json:"error,omitempty"`

	// Metadata: This field will contain an InspectOperationMetadata object.
	// This will always be returned with the Operation.
	Metadata googleapi.RawMessage `json:"metadata,omitempty"`

	// Name: The server-assigned name, The `name` should have the format of
	// `inspect/operations/<identifier>`.
	Name string `json:"name,omitempty"`

	// Response: This field will contain an InspectOperationResult object.
	Response googleapi.RawMessage `json:"response,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Done") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Done") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleLongrunningOperation) MarshalJSON() ([]byte, error) {
	type noMethod GoogleLongrunningOperation
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1CategoryDescription: Info Type Category
// description.
type GooglePrivacyDlpV2beta1CategoryDescription struct {
	// DisplayName: Human readable form of the category name.
	DisplayName string `json:"displayName,omitempty"`

	// Name: Internal name of the category.
	Name string `json:"name,omitempty"`

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

func (s *GooglePrivacyDlpV2beta1CategoryDescription) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1CategoryDescription
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1CloudStorageKey: Record key for a finding in a
// Cloud Storage file.
type GooglePrivacyDlpV2beta1CloudStorageKey struct {
	// FilePath: Path to the file.
	FilePath string `json:"filePath,omitempty"`

	// StartOffset: Byte offset of the referenced data in the file.
	StartOffset int64 `json:"startOffset,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "FilePath") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "FilePath") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1CloudStorageKey) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1CloudStorageKey
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1CloudStorageOptions: Options defining a file
// or a set of files (path ending with *) within
// a Google Cloud Storage bucket.
type GooglePrivacyDlpV2beta1CloudStorageOptions struct {
	FileSet *GooglePrivacyDlpV2beta1FileSet `json:"fileSet,omitempty"`

	// ForceSendFields is a list of field names (e.g. "FileSet") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "FileSet") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1CloudStorageOptions) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1CloudStorageOptions
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1CloudStoragePath: A location in Cloud Storage.
type GooglePrivacyDlpV2beta1CloudStoragePath struct {
	// Path: The url, in the format of `gs://bucket/<path>`.
	Path string `json:"path,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Path") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Path") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1CloudStoragePath) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1CloudStoragePath
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1Color: Represents a color in the RGB color
// space.
type GooglePrivacyDlpV2beta1Color struct {
	// Blue: The amount of blue in the color as a value in the interval [0,
	// 1].
	Blue float64 `json:"blue,omitempty"`

	// Green: The amount of green in the color as a value in the interval
	// [0, 1].
	Green float64 `json:"green,omitempty"`

	// Red: The amount of red in the color as a value in the interval [0,
	// 1].
	Red float64 `json:"red,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Blue") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Blue") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1Color) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1Color
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GooglePrivacyDlpV2beta1Color) UnmarshalJSON(data []byte) error {
	type noMethod GooglePrivacyDlpV2beta1Color
	var s1 struct {
		Blue  gensupport.JSONFloat64 `json:"blue"`
		Green gensupport.JSONFloat64 `json:"green"`
		Red   gensupport.JSONFloat64 `json:"red"`
		*noMethod
	}
	s1.noMethod = (*noMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Blue = float64(s1.Blue)
	s.Green = float64(s1.Green)
	s.Red = float64(s1.Red)
	return nil
}

// GooglePrivacyDlpV2beta1ContentItem: Container structure for the
// content to inspect.
type GooglePrivacyDlpV2beta1ContentItem struct {
	// Data: Content data to inspect or redact.
	Data string `json:"data,omitempty"`

	// Type: Type of the content, as defined in Content-Type HTTP
	// header.
	// Supported types are: all "text" types, octet streams, PNG
	// images,
	// JPEG images.
	Type string `json:"type,omitempty"`

	// Value: String data to inspect or redact.
	Value string `json:"value,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Data") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Data") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1ContentItem) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1ContentItem
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1CreateInspectOperationRequest: Request for
// scheduling a scan of a data subset from a Google Platform
// data
// repository.
type GooglePrivacyDlpV2beta1CreateInspectOperationRequest struct {
	// InspectConfig: Configuration for the inspector.
	InspectConfig *GooglePrivacyDlpV2beta1InspectConfig `json:"inspectConfig,omitempty"`

	// OutputConfig: Optional location to store findings. The bucket must
	// already exist and
	// the Google APIs service account for DLP must have write permission
	// to
	// write to the given bucket.
	// <p>Results are split over multiple csv files with each file name
	// matching
	// the pattern "[operation_id]_[count].csv", for
	// example
	// `3094877188788974909_1.csv`. The `operation_id` matches
	// the
	// identifier for the Operation, and the `count` is a counter used
	// for
	// tracking the number of files written. <p>The CSV file(s) contain
	// the
	// following columns regardless of storage type scanned: <li>id
	// <li>info_type
	// <li>likelihood <li>byte size of finding <li>quote
	// <li>time_stamp<br/>
	// <p>For Cloud Storage the next columns are:
	// <li>file_path
	// <li>start_offset<br/>
	// <p>For Cloud Datastore the next columns are:
	// <li>project_id
	// <li>namespace_id <li>path <li>column_name <li>offset
	OutputConfig *GooglePrivacyDlpV2beta1OutputStorageConfig `json:"outputConfig,omitempty"`

	// StorageConfig: Specification of the data set to process.
	StorageConfig *GooglePrivacyDlpV2beta1StorageConfig `json:"storageConfig,omitempty"`

	// ForceSendFields is a list of field names (e.g. "InspectConfig") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "InspectConfig") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1CreateInspectOperationRequest) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1CreateInspectOperationRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1DatastoreKey: Record key for a finding in
// Cloud Datastore.
type GooglePrivacyDlpV2beta1DatastoreKey struct {
	// EntityKey: Datastore entity key.
	EntityKey *GooglePrivacyDlpV2beta1Key `json:"entityKey,omitempty"`

	// ForceSendFields is a list of field names (e.g. "EntityKey") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "EntityKey") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1DatastoreKey) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1DatastoreKey
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1DatastoreOptions: Options defining a data set
// within Google Cloud Datastore.
type GooglePrivacyDlpV2beta1DatastoreOptions struct {
	// Kind: The kind to process.
	Kind *GooglePrivacyDlpV2beta1KindExpression `json:"kind,omitempty"`

	// PartitionId: A partition ID identifies a grouping of entities. The
	// grouping is always
	// by project and namespace, however the namespace ID may be empty.
	PartitionId *GooglePrivacyDlpV2beta1PartitionId `json:"partitionId,omitempty"`

	// Projection: Properties to scan. If none are specified, all properties
	// will be scanned
	// by default.
	Projection []*GooglePrivacyDlpV2beta1Projection `json:"projection,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Kind") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Kind") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1DatastoreOptions) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1DatastoreOptions
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1FieldId: General identifier of a data field in
// a storage service.
type GooglePrivacyDlpV2beta1FieldId struct {
	// ColumnName: Column name describing the field.
	ColumnName string `json:"columnName,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ColumnName") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ColumnName") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1FieldId) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1FieldId
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1FileSet: Set of files to scan.
type GooglePrivacyDlpV2beta1FileSet struct {
	// Url: The url, in the format `gs://<bucket>/<path>`. Trailing wildcard
	// in the
	// path is allowed.
	Url string `json:"url,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Url") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Url") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1FileSet) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1FileSet
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1Finding: Container structure describing a
// single finding within a string or image.
type GooglePrivacyDlpV2beta1Finding struct {
	// CreateTime: Timestamp when finding was detected.
	CreateTime string `json:"createTime,omitempty"`

	// InfoType: The specific type of info the string might be.
	InfoType *GooglePrivacyDlpV2beta1InfoType `json:"infoType,omitempty"`

	// Likelihood: Estimate of how likely it is that the info_type is
	// correct.
	//
	// Possible values:
	//   "LIKELIHOOD_UNSPECIFIED" - Default value; information with all
	// likelihoods is included.
	//   "VERY_UNLIKELY" - Few matching elements.
	//   "UNLIKELY"
	//   "POSSIBLE" - Some matching elements.
	//   "LIKELY"
	//   "VERY_LIKELY" - Many matching elements.
	Likelihood string `json:"likelihood,omitempty"`

	// Location: Location of the info found.
	Location *GooglePrivacyDlpV2beta1Location `json:"location,omitempty"`

	// Quote: The specific string that may be potentially sensitive info.
	Quote string `json:"quote,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CreateTime") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CreateTime") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1Finding) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1Finding
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1ImageLocation: Bounding box encompassing
// detected text within an image.
type GooglePrivacyDlpV2beta1ImageLocation struct {
	// Height: Height of the bounding box in pixels.
	Height int64 `json:"height,omitempty"`

	// Left: Left coordinate of the bounding box. (0,0) is upper left.
	Left int64 `json:"left,omitempty"`

	// Top: Top coordinate of the bounding box. (0,0) is upper left.
	Top int64 `json:"top,omitempty"`

	// Width: Width of the bounding box in pixels.
	Width int64 `json:"width,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Height") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Height") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1ImageLocation) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1ImageLocation
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1ImageRedactionConfig: Configuration for
// determing how redaction of images should occur.
type GooglePrivacyDlpV2beta1ImageRedactionConfig struct {
	// InfoType: Only one per info_type should be provided per request. If
	// not
	// specified, and redact_all_text is false, the DLP API will redacts
	// all
	// text that it matches against all info_types that are found, but
	// not
	// specified in another ImageRedactionConfig.
	InfoType *GooglePrivacyDlpV2beta1InfoType `json:"infoType,omitempty"`

	// RedactAllText: If true, all text found in the image, regardless if it
	// matches an
	// info_type, is redacted.
	RedactAllText bool `json:"redactAllText,omitempty"`

	// RedactionColor: The color to use when redacting content from an
	// image. If not specified,
	// the default is black.
	RedactionColor *GooglePrivacyDlpV2beta1Color `json:"redactionColor,omitempty"`

	// ForceSendFields is a list of field names (e.g. "InfoType") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "InfoType") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1ImageRedactionConfig) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1ImageRedactionConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1InfoType: Type of information detected by the
// API.
type GooglePrivacyDlpV2beta1InfoType struct {
	// Name: Name of the information type. For built-in info types, this is
	// provided by
	// the API call ListInfoTypes. For user-defined info types, this
	// is
	// provided by the user. All user-defined info types must have unique
	// names,
	// and cannot conflict with built-in info type names.
	Name string `json:"name,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Name") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Name") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1InfoType) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1InfoType
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1InfoTypeDescription: Info type description.
type GooglePrivacyDlpV2beta1InfoTypeDescription struct {
	// Categories: List of categories this info type belongs to.
	Categories []*GooglePrivacyDlpV2beta1CategoryDescription `json:"categories,omitempty"`

	// DisplayName: Human readable form of the info type name.
	DisplayName string `json:"displayName,omitempty"`

	// Name: Internal name of the info type.
	Name string `json:"name,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Categories") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Categories") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1InfoTypeDescription) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1InfoTypeDescription
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1InfoTypeStatistics: Statistics regarding a
// specific InfoType.
type GooglePrivacyDlpV2beta1InfoTypeStatistics struct {
	// Count: Number of findings for this info type.
	Count int64 `json:"count,omitempty,string"`

	// InfoType: The type of finding this stat is for.
	InfoType *GooglePrivacyDlpV2beta1InfoType `json:"infoType,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Count") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Count") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1InfoTypeStatistics) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1InfoTypeStatistics
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1InspectConfig: Configuration description of
// the scanning process.
// When used with redactContent only info_types and min_likelihood are
// currently
// used.
type GooglePrivacyDlpV2beta1InspectConfig struct {
	// ExcludeTypes: When true, excludes type information of the findings.
	ExcludeTypes bool `json:"excludeTypes,omitempty"`

	// IncludeQuote: When true, a contextual quote from the data that
	// triggered a finding is
	// included in the response; see Finding.quote.
	IncludeQuote bool `json:"includeQuote,omitempty"`

	// InfoTypes: Restricts what info_types to look for. The values must
	// correspond to
	// InfoType values returned by ListInfoTypes or found in
	// documentation.
	// Empty info_types runs all enabled detectors.
	InfoTypes []*GooglePrivacyDlpV2beta1InfoType `json:"infoTypes,omitempty"`

	// MaxFindings: Limits the number of findings per content item or long
	// running operation.
	MaxFindings int64 `json:"maxFindings,omitempty"`

	// MinLikelihood: Only returns findings equal or above this threshold.
	//
	// Possible values:
	//   "LIKELIHOOD_UNSPECIFIED" - Default value; information with all
	// likelihoods is included.
	//   "VERY_UNLIKELY" - Few matching elements.
	//   "UNLIKELY"
	//   "POSSIBLE" - Some matching elements.
	//   "LIKELY"
	//   "VERY_LIKELY" - Many matching elements.
	MinLikelihood string `json:"minLikelihood,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ExcludeTypes") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ExcludeTypes") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1InspectConfig) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1InspectConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1InspectContentRequest: Request to search for
// potentially sensitive info in a list of items.
type GooglePrivacyDlpV2beta1InspectContentRequest struct {
	// InspectConfig: Configuration for the inspector.
	InspectConfig *GooglePrivacyDlpV2beta1InspectConfig `json:"inspectConfig,omitempty"`

	// Items: The list of items to inspect. Items in a single request
	// are
	// considered "related" unless inspect_config.independent_inputs is
	// true.
	// Up to 100 are allowed per request.
	Items []*GooglePrivacyDlpV2beta1ContentItem `json:"items,omitempty"`

	// ForceSendFields is a list of field names (e.g. "InspectConfig") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "InspectConfig") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1InspectContentRequest) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1InspectContentRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1InspectContentResponse: Results of inspecting
// a list of items.
type GooglePrivacyDlpV2beta1InspectContentResponse struct {
	// Results: Each content_item from the request has a result in this
	// list, in the
	// same order as the request.
	Results []*GooglePrivacyDlpV2beta1InspectResult `json:"results,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Results") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Results") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1InspectContentResponse) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1InspectContentResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1InspectOperationMetadata: Metadata returned
// within GetOperation for an inspect request.
type GooglePrivacyDlpV2beta1InspectOperationMetadata struct {
	// CreateTime: The time which this request was started.
	CreateTime string `json:"createTime,omitempty"`

	InfoTypeStats []*GooglePrivacyDlpV2beta1InfoTypeStatistics `json:"infoTypeStats,omitempty"`

	// ProcessedBytes: Total size in bytes that were processed.
	ProcessedBytes int64 `json:"processedBytes,omitempty,string"`

	// RequestInspectConfig: The inspect config used to create the
	// Operation.
	RequestInspectConfig *GooglePrivacyDlpV2beta1InspectConfig `json:"requestInspectConfig,omitempty"`

	// RequestOutputConfig: Optional location to store findings.
	RequestOutputConfig *GooglePrivacyDlpV2beta1OutputStorageConfig `json:"requestOutputConfig,omitempty"`

	// RequestStorageConfig: The storage config used to create the
	// Operation.
	RequestStorageConfig *GooglePrivacyDlpV2beta1StorageConfig `json:"requestStorageConfig,omitempty"`

	// TotalEstimatedBytes: Estimate of the number of bytes to process.
	TotalEstimatedBytes int64 `json:"totalEstimatedBytes,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "CreateTime") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CreateTime") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1InspectOperationMetadata) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1InspectOperationMetadata
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1InspectOperationResult: The operational data.
type GooglePrivacyDlpV2beta1InspectOperationResult struct {
	// Name: The server-assigned name, which is only unique within the same
	// service that
	// originally returns it. If you use the default HTTP mapping,
	// the
	// `name` should have the format of `inspect/results/{id}`.
	Name string `json:"name,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Name") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Name") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1InspectOperationResult) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1InspectOperationResult
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1InspectResult: All the findings for a single
// scanned item.
type GooglePrivacyDlpV2beta1InspectResult struct {
	// Findings: List of findings for an item.
	Findings []*GooglePrivacyDlpV2beta1Finding `json:"findings,omitempty"`

	// FindingsTruncated: If true, then this item might have more findings
	// than were returned,
	// and the findings returned are an arbitrary subset of all
	// findings.
	// The findings list might be truncated because the input items were
	// too
	// large, or because the server reached the maximum amount of
	// resources
	// allowed for a single API call. For best results, divide the input
	// into
	// smaller batches.
	FindingsTruncated bool `json:"findingsTruncated,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Findings") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Findings") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1InspectResult) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1InspectResult
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1Key: A unique identifier for a Datastore
// entity.
// If a key's partition ID or any of its path kinds or names
// are
// reserved/read-only, the key is reserved/read-only.
// A reserved/read-only key is forbidden in certain documented contexts.
type GooglePrivacyDlpV2beta1Key struct {
	// PartitionId: Entities are partitioned into subsets, currently
	// identified by a project
	// ID and namespace ID.
	// Queries are scoped to a single partition.
	PartitionId *GooglePrivacyDlpV2beta1PartitionId `json:"partitionId,omitempty"`

	// Path: The entity path.
	// An entity path consists of one or more elements composed of a kind
	// and a
	// string or numerical identifier, which identify entities. The
	// first
	// element identifies a _root entity_, the second element identifies
	// a _child_ of the root entity, the third element identifies a child of
	// the
	// second entity, and so forth. The entities identified by all prefixes
	// of
	// the path are called the element's _ancestors_.
	//
	// A path can never be empty, and a path can have at most 100 elements.
	Path []*GooglePrivacyDlpV2beta1PathElement `json:"path,omitempty"`

	// ForceSendFields is a list of field names (e.g. "PartitionId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "PartitionId") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1Key) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1Key
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1KindExpression: A representation of a
// Datastore kind.
type GooglePrivacyDlpV2beta1KindExpression struct {
	// Name: The name of the kind.
	Name string `json:"name,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Name") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Name") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1KindExpression) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1KindExpression
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1ListInfoTypesResponse: Response to the
// ListInfoTypes request.
type GooglePrivacyDlpV2beta1ListInfoTypesResponse struct {
	// InfoTypes: Set of sensitive info types belonging to a category.
	InfoTypes []*GooglePrivacyDlpV2beta1InfoTypeDescription `json:"infoTypes,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "InfoTypes") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "InfoTypes") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1ListInfoTypesResponse) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1ListInfoTypesResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1ListInspectFindingsResponse: Response to the
// ListInspectFindings request.
type GooglePrivacyDlpV2beta1ListInspectFindingsResponse struct {
	// NextPageToken: If not empty, indicates that there may be more results
	// that match the
	// request; this value should be passed in a new
	// `ListInspectFindingsRequest`.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// Result: The results.
	Result *GooglePrivacyDlpV2beta1InspectResult `json:"result,omitempty"`

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

func (s *GooglePrivacyDlpV2beta1ListInspectFindingsResponse) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1ListInspectFindingsResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1ListRootCategoriesResponse: Response for
// ListRootCategories request.
type GooglePrivacyDlpV2beta1ListRootCategoriesResponse struct {
	// Categories: List of all into type categories supported by the API.
	Categories []*GooglePrivacyDlpV2beta1CategoryDescription `json:"categories,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Categories") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Categories") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1ListRootCategoriesResponse) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1ListRootCategoriesResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1Location: Specifies the location of a finding
// within its source item.
type GooglePrivacyDlpV2beta1Location struct {
	// ByteRange: Zero-based byte offsets within a content item.
	ByteRange *GooglePrivacyDlpV2beta1Range `json:"byteRange,omitempty"`

	// CodepointRange: Character offsets within a content item, included
	// when content type
	// is a text. Default charset assumed to be UTF-8.
	CodepointRange *GooglePrivacyDlpV2beta1Range `json:"codepointRange,omitempty"`

	// FieldId: Field id of the field containing the finding.
	FieldId *GooglePrivacyDlpV2beta1FieldId `json:"fieldId,omitempty"`

	// ImageBoxes: Location within an image's pixels.
	ImageBoxes []*GooglePrivacyDlpV2beta1ImageLocation `json:"imageBoxes,omitempty"`

	// RecordKey: Key of the finding.
	RecordKey *GooglePrivacyDlpV2beta1RecordKey `json:"recordKey,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ByteRange") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ByteRange") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1Location) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1Location
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1OutputStorageConfig: Cloud repository for
// storing output.
type GooglePrivacyDlpV2beta1OutputStorageConfig struct {
	// StoragePath: The path to a Google Cloud Storage location to store
	// output.
	StoragePath *GooglePrivacyDlpV2beta1CloudStoragePath `json:"storagePath,omitempty"`

	// ForceSendFields is a list of field names (e.g. "StoragePath") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "StoragePath") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1OutputStorageConfig) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1OutputStorageConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1PartitionId: Datastore partition ID.
// A partition ID identifies a grouping of entities. The grouping is
// always
// by project and namespace, however the namespace ID may be empty.
//
// A partition ID contains several dimensions:
// project ID and namespace ID.
type GooglePrivacyDlpV2beta1PartitionId struct {
	// NamespaceId: If not empty, the ID of the namespace to which the
	// entities belong.
	NamespaceId string `json:"namespaceId,omitempty"`

	// ProjectId: The ID of the project to which the entities belong.
	ProjectId string `json:"projectId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "NamespaceId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "NamespaceId") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1PartitionId) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1PartitionId
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1PathElement: A (kind, ID/name) pair used to
// construct a key path.
//
// If either name or ID is set, the element is complete.
// If neither is set, the element is incomplete.
type GooglePrivacyDlpV2beta1PathElement struct {
	// Id: The auto-allocated ID of the entity.
	// Never equal to zero. Values less than zero are discouraged and may
	// not
	// be supported in the future.
	Id int64 `json:"id,omitempty,string"`

	// Kind: The kind of the entity.
	// A kind matching regex `__.*__` is reserved/read-only.
	// A kind must not contain more than 1500 bytes when UTF-8
	// encoded.
	// Cannot be "".
	Kind string `json:"kind,omitempty"`

	// Name: The name of the entity.
	// A name matching regex `__.*__` is reserved/read-only.
	// A name must not be more than 1500 bytes when UTF-8 encoded.
	// Cannot be "".
	Name string `json:"name,omitempty"`

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

func (s *GooglePrivacyDlpV2beta1PathElement) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1PathElement
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1Projection: A representation of a Datastore
// property in a projection.
type GooglePrivacyDlpV2beta1Projection struct {
	// Property: The property to project.
	Property *GooglePrivacyDlpV2beta1PropertyReference `json:"property,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Property") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Property") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1Projection) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1Projection
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1PropertyReference: A reference to a property
// relative to the Datastore kind expressions.
type GooglePrivacyDlpV2beta1PropertyReference struct {
	// Name: The name of the property.
	// If name includes "."s, it may be interpreted as a property name path.
	Name string `json:"name,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Name") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Name") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1PropertyReference) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1PropertyReference
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1Range: Generic half-open interval [start, end)
type GooglePrivacyDlpV2beta1Range struct {
	// End: Index of the last character of the range (exclusive).
	End int64 `json:"end,omitempty,string"`

	// Start: Index of the first character of the range (inclusive).
	Start int64 `json:"start,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "End") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "End") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1Range) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1Range
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1RecordKey: Message for a unique key indicating
// a record that contains a finding.
type GooglePrivacyDlpV2beta1RecordKey struct {
	CloudStorageKey *GooglePrivacyDlpV2beta1CloudStorageKey `json:"cloudStorageKey,omitempty"`

	DatastoreKey *GooglePrivacyDlpV2beta1DatastoreKey `json:"datastoreKey,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CloudStorageKey") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CloudStorageKey") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1RecordKey) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1RecordKey
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1RedactContentRequest: Request to search for
// potentially sensitive info in a list of items
// and replace it with a default or provided content.
type GooglePrivacyDlpV2beta1RedactContentRequest struct {
	// ImageRedactionConfigs: The configuration for specifying what content
	// to redact from images.
	ImageRedactionConfigs []*GooglePrivacyDlpV2beta1ImageRedactionConfig `json:"imageRedactionConfigs,omitempty"`

	// InspectConfig: Configuration for the inspector.
	InspectConfig *GooglePrivacyDlpV2beta1InspectConfig `json:"inspectConfig,omitempty"`

	// Items: The list of items to inspect. Up to 100 are allowed per
	// request.
	Items []*GooglePrivacyDlpV2beta1ContentItem `json:"items,omitempty"`

	// ReplaceConfigs: The strings to replace findings text findings with.
	// Must specify at least
	// one of these or one ImageRedactionConfig if redacting images.
	ReplaceConfigs []*GooglePrivacyDlpV2beta1ReplaceConfig `json:"replaceConfigs,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "ImageRedactionConfigs") to unconditionally include in API requests.
	// By default, fields with empty values are omitted from API requests.
	// However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ImageRedactionConfigs") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1RedactContentRequest) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1RedactContentRequest
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1RedactContentResponse: Results of redacting a
// list of items.
type GooglePrivacyDlpV2beta1RedactContentResponse struct {
	// Items: The redacted content.
	Items []*GooglePrivacyDlpV2beta1ContentItem `json:"items,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Items") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Items") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1RedactContentResponse) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1RedactContentResponse
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type GooglePrivacyDlpV2beta1ReplaceConfig struct {
	// InfoType: Type of information to replace. Only one ReplaceConfig per
	// info_type
	// should be provided. If ReplaceConfig does not have an info_type, the
	// DLP
	// API matches it against all info_types that are found but not
	// specified in
	// another ReplaceConfig.
	InfoType *GooglePrivacyDlpV2beta1InfoType `json:"infoType,omitempty"`

	// ReplaceWith: Content replacing sensitive information of given type.
	// Max 256 chars.
	ReplaceWith string `json:"replaceWith,omitempty"`

	// ForceSendFields is a list of field names (e.g. "InfoType") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "InfoType") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1ReplaceConfig) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1ReplaceConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1StorageConfig: Shared message indicating Cloud
// storage type.
type GooglePrivacyDlpV2beta1StorageConfig struct {
	// CloudStorageOptions: Google Cloud Storage options specification.
	CloudStorageOptions *GooglePrivacyDlpV2beta1CloudStorageOptions `json:"cloudStorageOptions,omitempty"`

	// DatastoreOptions: Google Cloud Datastore options specification.
	DatastoreOptions *GooglePrivacyDlpV2beta1DatastoreOptions `json:"datastoreOptions,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CloudStorageOptions")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CloudStorageOptions") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1StorageConfig) MarshalJSON() ([]byte, error) {
	type noMethod GooglePrivacyDlpV2beta1StorageConfig
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleProtobufEmpty: A generic empty message that you can re-use to
// avoid defining duplicated
// empty messages in your APIs. A typical example is to use it as the
// request
// or the response type of an API method. For instance:
//
//     service Foo {
//       rpc Bar(google.protobuf.Empty) returns
// (google.protobuf.Empty);
//     }
//
// The JSON representation for `Empty` is empty JSON object `{}`.
type GoogleProtobufEmpty struct {
	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`
}

// GoogleRpcStatus: The `Status` type defines a logical error model that
// is suitable for different
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
type GoogleRpcStatus struct {
	// Code: The status code, which should be an enum value of
	// google.rpc.Code.
	Code int64 `json:"code,omitempty"`

	// Details: A list of messages that carry the error details.  There will
	// be a
	// common set of message types for APIs to use.
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

func (s *GoogleRpcStatus) MarshalJSON() ([]byte, error) {
	type noMethod GoogleRpcStatus
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// method id "dlp.content.inspect":

type ContentInspectCall struct {
	s                                            *Service
	googleprivacydlpv2beta1inspectcontentrequest *GooglePrivacyDlpV2beta1InspectContentRequest
	urlParams_                                   gensupport.URLParams
	ctx_                                         context.Context
	header_                                      http.Header
}

// Inspect: Finds potentially sensitive info in a list of strings.
// This method has limits on input size, processing time, and output
// size.
func (r *ContentService) Inspect(googleprivacydlpv2beta1inspectcontentrequest *GooglePrivacyDlpV2beta1InspectContentRequest) *ContentInspectCall {
	c := &ContentInspectCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.googleprivacydlpv2beta1inspectcontentrequest = googleprivacydlpv2beta1inspectcontentrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ContentInspectCall) Fields(s ...googleapi.Field) *ContentInspectCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ContentInspectCall) Context(ctx context.Context) *ContentInspectCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ContentInspectCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ContentInspectCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta1inspectcontentrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta1/content:inspect")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.content.inspect" call.
// Exactly one of *GooglePrivacyDlpV2beta1InspectContentResponse or
// error will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta1InspectContentResponse.ServerResponse.Header
// or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ContentInspectCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta1InspectContentResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta1InspectContentResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Finds potentially sensitive info in a list of strings.\nThis method has limits on input size, processing time, and output size.",
	//   "flatPath": "v2beta1/content:inspect",
	//   "httpMethod": "POST",
	//   "id": "dlp.content.inspect",
	//   "parameterOrder": [],
	//   "parameters": {},
	//   "path": "v2beta1/content:inspect",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta1InspectContentRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta1InspectContentResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.content.redact":

type ContentRedactCall struct {
	s                                           *Service
	googleprivacydlpv2beta1redactcontentrequest *GooglePrivacyDlpV2beta1RedactContentRequest
	urlParams_                                  gensupport.URLParams
	ctx_                                        context.Context
	header_                                     http.Header
}

// Redact: Redacts potentially sensitive info from a list of
// strings.
// This method has limits on input size, processing time, and output
// size.
func (r *ContentService) Redact(googleprivacydlpv2beta1redactcontentrequest *GooglePrivacyDlpV2beta1RedactContentRequest) *ContentRedactCall {
	c := &ContentRedactCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.googleprivacydlpv2beta1redactcontentrequest = googleprivacydlpv2beta1redactcontentrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ContentRedactCall) Fields(s ...googleapi.Field) *ContentRedactCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ContentRedactCall) Context(ctx context.Context) *ContentRedactCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ContentRedactCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ContentRedactCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta1redactcontentrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta1/content:redact")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.content.redact" call.
// Exactly one of *GooglePrivacyDlpV2beta1RedactContentResponse or error
// will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta1RedactContentResponse.ServerResponse.Header
// or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ContentRedactCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta1RedactContentResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta1RedactContentResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Redacts potentially sensitive info from a list of strings.\nThis method has limits on input size, processing time, and output size.",
	//   "flatPath": "v2beta1/content:redact",
	//   "httpMethod": "POST",
	//   "id": "dlp.content.redact",
	//   "parameterOrder": [],
	//   "parameters": {},
	//   "path": "v2beta1/content:redact",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta1RedactContentRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta1RedactContentResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.inspect.operations.cancel":

type InspectOperationsCancelCall struct {
	s                                       *Service
	name                                    string
	googlelongrunningcanceloperationrequest *GoogleLongrunningCancelOperationRequest
	urlParams_                              gensupport.URLParams
	ctx_                                    context.Context
	header_                                 http.Header
}

// Cancel: Cancels an operation. Use the get method to check whether the
// cancellation succeeded or whether the operation completed despite
// cancellation.
func (r *InspectOperationsService) Cancel(name string, googlelongrunningcanceloperationrequest *GoogleLongrunningCancelOperationRequest) *InspectOperationsCancelCall {
	c := &InspectOperationsCancelCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.googlelongrunningcanceloperationrequest = googlelongrunningcanceloperationrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *InspectOperationsCancelCall) Fields(s ...googleapi.Field) *InspectOperationsCancelCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *InspectOperationsCancelCall) Context(ctx context.Context) *InspectOperationsCancelCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *InspectOperationsCancelCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *InspectOperationsCancelCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googlelongrunningcanceloperationrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta1/{+name}:cancel")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.inspect.operations.cancel" call.
// Exactly one of *GoogleProtobufEmpty or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *GoogleProtobufEmpty.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *InspectOperationsCancelCall) Do(opts ...googleapi.CallOption) (*GoogleProtobufEmpty, error) {
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
	ret := &GoogleProtobufEmpty{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Cancels an operation. Use the get method to check whether the cancellation succeeded or whether the operation completed despite cancellation.",
	//   "flatPath": "v2beta1/inspect/operations/{operationsId}:cancel",
	//   "httpMethod": "POST",
	//   "id": "dlp.inspect.operations.cancel",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the operation resource to be cancelled.",
	//       "location": "path",
	//       "pattern": "^inspect/operations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta1/{+name}:cancel",
	//   "request": {
	//     "$ref": "GoogleLongrunningCancelOperationRequest"
	//   },
	//   "response": {
	//     "$ref": "GoogleProtobufEmpty"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.inspect.operations.create":

type InspectOperationsCreateCall struct {
	s                                                    *Service
	googleprivacydlpv2beta1createinspectoperationrequest *GooglePrivacyDlpV2beta1CreateInspectOperationRequest
	urlParams_                                           gensupport.URLParams
	ctx_                                                 context.Context
	header_                                              http.Header
}

// Create: Schedules a job scanning content in a Google Cloud Platform
// data
// repository.
func (r *InspectOperationsService) Create(googleprivacydlpv2beta1createinspectoperationrequest *GooglePrivacyDlpV2beta1CreateInspectOperationRequest) *InspectOperationsCreateCall {
	c := &InspectOperationsCreateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.googleprivacydlpv2beta1createinspectoperationrequest = googleprivacydlpv2beta1createinspectoperationrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *InspectOperationsCreateCall) Fields(s ...googleapi.Field) *InspectOperationsCreateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *InspectOperationsCreateCall) Context(ctx context.Context) *InspectOperationsCreateCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *InspectOperationsCreateCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *InspectOperationsCreateCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta1createinspectoperationrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta1/inspect/operations")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.inspect.operations.create" call.
// Exactly one of *GoogleLongrunningOperation or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *GoogleLongrunningOperation.ServerResponse.Header or (if a response
// was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *InspectOperationsCreateCall) Do(opts ...googleapi.CallOption) (*GoogleLongrunningOperation, error) {
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
	ret := &GoogleLongrunningOperation{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Schedules a job scanning content in a Google Cloud Platform data\nrepository.",
	//   "flatPath": "v2beta1/inspect/operations",
	//   "httpMethod": "POST",
	//   "id": "dlp.inspect.operations.create",
	//   "parameterOrder": [],
	//   "parameters": {},
	//   "path": "v2beta1/inspect/operations",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta1CreateInspectOperationRequest"
	//   },
	//   "response": {
	//     "$ref": "GoogleLongrunningOperation"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.inspect.operations.delete":

type InspectOperationsDeleteCall struct {
	s          *Service
	name       string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
	header_    http.Header
}

// Delete: This method is not supported and the server returns
// `UNIMPLEMENTED`.
func (r *InspectOperationsService) Delete(name string) *InspectOperationsDeleteCall {
	c := &InspectOperationsDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *InspectOperationsDeleteCall) Fields(s ...googleapi.Field) *InspectOperationsDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *InspectOperationsDeleteCall) Context(ctx context.Context) *InspectOperationsDeleteCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *InspectOperationsDeleteCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *InspectOperationsDeleteCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta1/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("DELETE", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.inspect.operations.delete" call.
// Exactly one of *GoogleProtobufEmpty or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *GoogleProtobufEmpty.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *InspectOperationsDeleteCall) Do(opts ...googleapi.CallOption) (*GoogleProtobufEmpty, error) {
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
	ret := &GoogleProtobufEmpty{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "This method is not supported and the server returns `UNIMPLEMENTED`.",
	//   "flatPath": "v2beta1/inspect/operations/{operationsId}",
	//   "httpMethod": "DELETE",
	//   "id": "dlp.inspect.operations.delete",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the operation resource to be deleted.",
	//       "location": "path",
	//       "pattern": "^inspect/operations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta1/{+name}",
	//   "response": {
	//     "$ref": "GoogleProtobufEmpty"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.inspect.operations.get":

type InspectOperationsGetCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// Get: Gets the latest state of a long-running operation.  Clients can
// use this
// method to poll the operation result at intervals as recommended by
// the API
// service.
func (r *InspectOperationsService) Get(name string) *InspectOperationsGetCall {
	c := &InspectOperationsGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *InspectOperationsGetCall) Fields(s ...googleapi.Field) *InspectOperationsGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *InspectOperationsGetCall) IfNoneMatch(entityTag string) *InspectOperationsGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *InspectOperationsGetCall) Context(ctx context.Context) *InspectOperationsGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *InspectOperationsGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *InspectOperationsGetCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta1/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.inspect.operations.get" call.
// Exactly one of *GoogleLongrunningOperation or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *GoogleLongrunningOperation.ServerResponse.Header or (if a response
// was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *InspectOperationsGetCall) Do(opts ...googleapi.CallOption) (*GoogleLongrunningOperation, error) {
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
	ret := &GoogleLongrunningOperation{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Gets the latest state of a long-running operation.  Clients can use this\nmethod to poll the operation result at intervals as recommended by the API\nservice.",
	//   "flatPath": "v2beta1/inspect/operations/{operationsId}",
	//   "httpMethod": "GET",
	//   "id": "dlp.inspect.operations.get",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the operation resource.",
	//       "location": "path",
	//       "pattern": "^inspect/operations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta1/{+name}",
	//   "response": {
	//     "$ref": "GoogleLongrunningOperation"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.inspect.operations.list":

type InspectOperationsListCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Fetch the list of long running operations.
func (r *InspectOperationsService) List(name string) *InspectOperationsListCall {
	c := &InspectOperationsListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Filter sets the optional parameter "filter": This parameter supports
// filtering by done, ie done=true or done=false.
func (c *InspectOperationsListCall) Filter(filter string) *InspectOperationsListCall {
	c.urlParams_.Set("filter", filter)
	return c
}

// PageSize sets the optional parameter "pageSize": The list page size.
// The max allowed value is 256 and default is 100.
func (c *InspectOperationsListCall) PageSize(pageSize int64) *InspectOperationsListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": The standard list
// page token.
func (c *InspectOperationsListCall) PageToken(pageToken string) *InspectOperationsListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *InspectOperationsListCall) Fields(s ...googleapi.Field) *InspectOperationsListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *InspectOperationsListCall) IfNoneMatch(entityTag string) *InspectOperationsListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *InspectOperationsListCall) Context(ctx context.Context) *InspectOperationsListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *InspectOperationsListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *InspectOperationsListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta1/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.inspect.operations.list" call.
// Exactly one of *GoogleLongrunningListOperationsResponse or error will
// be non-nil. Any non-2xx status code is an error. Response headers are
// in either
// *GoogleLongrunningListOperationsResponse.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *InspectOperationsListCall) Do(opts ...googleapi.CallOption) (*GoogleLongrunningListOperationsResponse, error) {
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
	ret := &GoogleLongrunningListOperationsResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Fetch the list of long running operations.",
	//   "flatPath": "v2beta1/inspect/operations",
	//   "httpMethod": "GET",
	//   "id": "dlp.inspect.operations.list",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "filter": {
	//       "description": "This parameter supports filtering by done, ie done=true or done=false.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "name": {
	//       "description": "The name of the operation's parent resource.",
	//       "location": "path",
	//       "pattern": "^inspect/operations$",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "pageSize": {
	//       "description": "The list page size. The max allowed value is 256 and default is 100.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "description": "The standard list page token.",
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta1/{+name}",
	//   "response": {
	//     "$ref": "GoogleLongrunningListOperationsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *InspectOperationsListCall) Pages(ctx context.Context, f func(*GoogleLongrunningListOperationsResponse) error) error {
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

// method id "dlp.inspect.results.findings.list":

type InspectResultsFindingsListCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Returns list of results for given inspect operation result set
// id.
func (r *InspectResultsFindingsService) List(name string) *InspectResultsFindingsListCall {
	c := &InspectResultsFindingsListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Filter sets the optional parameter "filter": Restricts findings to
// items that match. Supports info_type and
// likelihood.
// <p>Examples:<br/>
// <li>info_type=EMAIL_ADDRESS
// <li>info_typ
// e=PHONE_NUMBER,EMAIL_ADDRESS
// <li>likelihood=VERY_LIKELY
// <li>likelihood
// =VERY_LIKELY,LIKELY
// <li>info_type=EMAIL_ADDRESS,likelihood=VERY_LIKELY
// ,LIKELY
func (c *InspectResultsFindingsListCall) Filter(filter string) *InspectResultsFindingsListCall {
	c.urlParams_.Set("filter", filter)
	return c
}

// PageSize sets the optional parameter "pageSize": Maximum number of
// results to return.
// If 0, the implementation selects a reasonable value.
func (c *InspectResultsFindingsListCall) PageSize(pageSize int64) *InspectResultsFindingsListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": The value returned
// by the last `ListInspectFindingsResponse`; indicates
// that this is a continuation of a prior `ListInspectFindings` call,
// and that
// the system should return the next page of data.
func (c *InspectResultsFindingsListCall) PageToken(pageToken string) *InspectResultsFindingsListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *InspectResultsFindingsListCall) Fields(s ...googleapi.Field) *InspectResultsFindingsListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *InspectResultsFindingsListCall) IfNoneMatch(entityTag string) *InspectResultsFindingsListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *InspectResultsFindingsListCall) Context(ctx context.Context) *InspectResultsFindingsListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *InspectResultsFindingsListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *InspectResultsFindingsListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta1/{+name}/findings")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.inspect.results.findings.list" call.
// Exactly one of *GooglePrivacyDlpV2beta1ListInspectFindingsResponse or
// error will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta1ListInspectFindingsResponse.ServerResponse.Hea
// der or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *InspectResultsFindingsListCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta1ListInspectFindingsResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta1ListInspectFindingsResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Returns list of results for given inspect operation result set id.",
	//   "flatPath": "v2beta1/inspect/results/{resultsId}/findings",
	//   "httpMethod": "GET",
	//   "id": "dlp.inspect.results.findings.list",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "filter": {
	//       "description": "Restricts findings to items that match. Supports info_type and likelihood.\n\u003cp\u003eExamples:\u003cbr/\u003e\n\u003cli\u003einfo_type=EMAIL_ADDRESS\n\u003cli\u003einfo_type=PHONE_NUMBER,EMAIL_ADDRESS\n\u003cli\u003elikelihood=VERY_LIKELY\n\u003cli\u003elikelihood=VERY_LIKELY,LIKELY\n\u003cli\u003einfo_type=EMAIL_ADDRESS,likelihood=VERY_LIKELY,LIKELY",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "name": {
	//       "description": "Identifier of the results set returned as metadata of\nthe longrunning operation created by a call to CreateInspectOperation.\nShould be in the format of `inspect/results/{id}.",
	//       "location": "path",
	//       "pattern": "^inspect/results/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "pageSize": {
	//       "description": "Maximum number of results to return.\nIf 0, the implementation selects a reasonable value.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "description": "The value returned by the last `ListInspectFindingsResponse`; indicates\nthat this is a continuation of a prior `ListInspectFindings` call, and that\nthe system should return the next page of data.",
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta1/{+name}/findings",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta1ListInspectFindingsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *InspectResultsFindingsListCall) Pages(ctx context.Context, f func(*GooglePrivacyDlpV2beta1ListInspectFindingsResponse) error) error {
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

// method id "dlp.rootCategories.list":

type RootCategoriesListCall struct {
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Returns the list of root categories of sensitive information.
func (r *RootCategoriesService) List() *RootCategoriesListCall {
	c := &RootCategoriesListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// LanguageCode sets the optional parameter "languageCode": Optional
// language code for localized friendly category names.
// If omitted or if localized strings are not available,
// en-US strings will be returned.
func (c *RootCategoriesListCall) LanguageCode(languageCode string) *RootCategoriesListCall {
	c.urlParams_.Set("languageCode", languageCode)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *RootCategoriesListCall) Fields(s ...googleapi.Field) *RootCategoriesListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *RootCategoriesListCall) IfNoneMatch(entityTag string) *RootCategoriesListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *RootCategoriesListCall) Context(ctx context.Context) *RootCategoriesListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *RootCategoriesListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *RootCategoriesListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta1/rootCategories")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.rootCategories.list" call.
// Exactly one of *GooglePrivacyDlpV2beta1ListRootCategoriesResponse or
// error will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta1ListRootCategoriesResponse.ServerResponse.Head
// er or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *RootCategoriesListCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta1ListRootCategoriesResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta1ListRootCategoriesResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Returns the list of root categories of sensitive information.",
	//   "flatPath": "v2beta1/rootCategories",
	//   "httpMethod": "GET",
	//   "id": "dlp.rootCategories.list",
	//   "parameterOrder": [],
	//   "parameters": {
	//     "languageCode": {
	//       "description": "Optional language code for localized friendly category names.\nIf omitted or if localized strings are not available,\nen-US strings will be returned.",
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta1/rootCategories",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta1ListRootCategoriesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.rootCategories.infoTypes.list":

type RootCategoriesInfoTypesListCall struct {
	s            *Service
	category     string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Returns sensitive information types for given category.
func (r *RootCategoriesInfoTypesService) List(category string) *RootCategoriesInfoTypesListCall {
	c := &RootCategoriesInfoTypesListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.category = category
	return c
}

// LanguageCode sets the optional parameter "languageCode": Optional
// BCP-47 language code for localized info type friendly
// names. If omitted, or if localized strings are not available,
// en-US strings will be returned.
func (c *RootCategoriesInfoTypesListCall) LanguageCode(languageCode string) *RootCategoriesInfoTypesListCall {
	c.urlParams_.Set("languageCode", languageCode)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *RootCategoriesInfoTypesListCall) Fields(s ...googleapi.Field) *RootCategoriesInfoTypesListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *RootCategoriesInfoTypesListCall) IfNoneMatch(entityTag string) *RootCategoriesInfoTypesListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *RootCategoriesInfoTypesListCall) Context(ctx context.Context) *RootCategoriesInfoTypesListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *RootCategoriesInfoTypesListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *RootCategoriesInfoTypesListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta1/rootCategories/{+category}/infoTypes")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"category": c.category,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.rootCategories.infoTypes.list" call.
// Exactly one of *GooglePrivacyDlpV2beta1ListInfoTypesResponse or error
// will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta1ListInfoTypesResponse.ServerResponse.Header
// or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *RootCategoriesInfoTypesListCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta1ListInfoTypesResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta1ListInfoTypesResponse{
		ServerResponse: googleapi.ServerResponse{
			Header:         res.Header,
			HTTPStatusCode: res.StatusCode,
		},
	}
	target := &ret
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Returns sensitive information types for given category.",
	//   "flatPath": "v2beta1/rootCategories/{rootCategoriesId}/infoTypes",
	//   "httpMethod": "GET",
	//   "id": "dlp.rootCategories.infoTypes.list",
	//   "parameterOrder": [
	//     "category"
	//   ],
	//   "parameters": {
	//     "category": {
	//       "description": "Category name as returned by ListRootCategories.",
	//       "location": "path",
	//       "pattern": "^[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "languageCode": {
	//       "description": "Optional BCP-47 language code for localized info type friendly\nnames. If omitted, or if localized strings are not available,\nen-US strings will be returned.",
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta1/rootCategories/{+category}/infoTypes",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta1ListInfoTypesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}
