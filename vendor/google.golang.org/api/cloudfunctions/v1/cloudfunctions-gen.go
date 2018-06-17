// Package cloudfunctions provides access to the Cloud Functions API.
//
// See https://cloud.google.com/functions
//
// Usage example:
//
//   import "google.golang.org/api/cloudfunctions/v1"
//   ...
//   cloudfunctionsService, err := cloudfunctions.New(oauthHttpClient)
package cloudfunctions // import "google.golang.org/api/cloudfunctions/v1"

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

const apiId = "cloudfunctions:v1"
const apiName = "cloudfunctions"
const apiVersion = "v1"
const basePath = "https://cloudfunctions.googleapis.com/"

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
	s.Operations = NewOperationsService(s)
	s.Projects = NewProjectsService(s)
	return s, nil
}

type Service struct {
	client    *http.Client
	BasePath  string // API endpoint base URL
	UserAgent string // optional additional User-Agent fragment

	Operations *OperationsService

	Projects *ProjectsService
}

func (s *Service) userAgent() string {
	if s.UserAgent == "" {
		return googleapi.UserAgent
	}
	return googleapi.UserAgent + " " + s.UserAgent
}

func NewOperationsService(s *Service) *OperationsService {
	rs := &OperationsService{s: s}
	return rs
}

type OperationsService struct {
	s *Service
}

func NewProjectsService(s *Service) *ProjectsService {
	rs := &ProjectsService{s: s}
	rs.Locations = NewProjectsLocationsService(s)
	return rs
}

type ProjectsService struct {
	s *Service

	Locations *ProjectsLocationsService
}

func NewProjectsLocationsService(s *Service) *ProjectsLocationsService {
	rs := &ProjectsLocationsService{s: s}
	rs.Functions = NewProjectsLocationsFunctionsService(s)
	return rs
}

type ProjectsLocationsService struct {
	s *Service

	Functions *ProjectsLocationsFunctionsService
}

func NewProjectsLocationsFunctionsService(s *Service) *ProjectsLocationsFunctionsService {
	rs := &ProjectsLocationsFunctionsService{s: s}
	return rs
}

type ProjectsLocationsFunctionsService struct {
	s *Service
}

// CallFunctionRequest: Request for the `CallFunction` method.
type CallFunctionRequest struct {
	// Data: Input to be passed to the function.
	Data string `json:"data,omitempty"`

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

func (s *CallFunctionRequest) MarshalJSON() ([]byte, error) {
	type NoMethod CallFunctionRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// CallFunctionResponse: Response of `CallFunction` method.
type CallFunctionResponse struct {
	// Error: Either system or user-function generated error. Set if
	// execution
	// was not successful.
	Error string `json:"error,omitempty"`

	// ExecutionId: Execution id of function invocation.
	ExecutionId string `json:"executionId,omitempty"`

	// Result: Result populated for successful execution of synchronous
	// function. Will
	// not be populated if function does not return a result through
	// context.
	Result string `json:"result,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Error") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Error") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *CallFunctionResponse) MarshalJSON() ([]byte, error) {
	type NoMethod CallFunctionResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// CloudFunction: Describes a Cloud Function that contains user
// computation executed in
// response to an event. It encapsulate function and triggers
// configurations.
type CloudFunction struct {
	// AvailableMemoryMb: The amount of memory in MB available for a
	// function.
	// Defaults to 256MB.
	AvailableMemoryMb int64 `json:"availableMemoryMb,omitempty"`

	// Description: User-provided description of a function.
	Description string `json:"description,omitempty"`

	// EntryPoint: The name of the function (as defined in source code) that
	// will be
	// executed. Defaults to the resource name suffix, if not specified.
	// For
	// backward compatibility, if function with given name is not found,
	// then the
	// system will try to use function named "function".
	// For Node.js this is name of a function exported by the module
	// specified
	// in `source_location`.
	EntryPoint string `json:"entryPoint,omitempty"`

	// EventTrigger: A source that fires events in response to a condition
	// in another service.
	EventTrigger *EventTrigger `json:"eventTrigger,omitempty"`

	// HttpsTrigger: An HTTPS endpoint type of source that can be triggered
	// via URL.
	HttpsTrigger *HttpsTrigger `json:"httpsTrigger,omitempty"`

	// Labels: Labels associated with this Cloud Function.
	Labels map[string]string `json:"labels,omitempty"`

	// Name: A user-defined name of the function. Function names must be
	// unique
	// globally and match pattern `projects/*/locations/*/functions/*`
	Name string `json:"name,omitempty"`

	// Runtime: The runtime in which the function is going to run. If empty,
	// defaults to
	// Node.js 6.
	Runtime string `json:"runtime,omitempty"`

	// ServiceAccountEmail: Output only. The email of the function's service
	// account.
	ServiceAccountEmail string `json:"serviceAccountEmail,omitempty"`

	// SourceArchiveUrl: The Google Cloud Storage URL, starting with gs://,
	// pointing to the zip
	// archive which contains the function.
	SourceArchiveUrl string `json:"sourceArchiveUrl,omitempty"`

	// SourceRepository: **Beta Feature**
	//
	// The source repository where a function is hosted.
	SourceRepository *SourceRepository `json:"sourceRepository,omitempty"`

	// SourceUploadUrl: The Google Cloud Storage signed URL used for source
	// uploading, generated
	// by google.cloud.functions.v1.GenerateUploadUrl
	SourceUploadUrl string `json:"sourceUploadUrl,omitempty"`

	// Status: Output only. Status of the function deployment.
	//
	// Possible values:
	//   "CLOUD_FUNCTION_STATUS_UNSPECIFIED" - Not specified. Invalid state.
	//   "ACTIVE" - Function has been succesfully deployed and is serving.
	//   "OFFLINE" - Function deployment failed and the function isn’t
	// serving.
	//   "DEPLOY_IN_PROGRESS" - Function is being created or updated.
	//   "DELETE_IN_PROGRESS" - Function is being deleted.
	//   "UNKNOWN" - Function deployment failed and the function serving
	// state is undefined.
	// The function should be updated or deleted to move it out of this
	// state.
	Status string `json:"status,omitempty"`

	// Timeout: The function execution timeout. Execution is considered
	// failed and
	// can be terminated if the function is not completed at the end of
	// the
	// timeout period. Defaults to 60 seconds.
	Timeout string `json:"timeout,omitempty"`

	// UpdateTime: Output only. The last update timestamp of a Cloud
	// Function.
	UpdateTime string `json:"updateTime,omitempty"`

	// VersionId: Output only.
	// The version identifier of the Cloud Function. Each deployment
	// attempt
	// results in a new version of a function being created.
	VersionId int64 `json:"versionId,omitempty,string"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "AvailableMemoryMb")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "AvailableMemoryMb") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *CloudFunction) MarshalJSON() ([]byte, error) {
	type NoMethod CloudFunction
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// EventTrigger: Describes EventTrigger, used to request events be sent
// from another
// service.
type EventTrigger struct {
	// EventType: Required. The type of event to observe. For
	// example:
	// `providers/cloud.storage/eventTypes/object.change`
	// and
	// `providers/cloud.pubsub/eventTypes/topic.publish`.
	//
	// Event types match pattern `providers/*/eventTypes/*.*`.
	// The pattern contains:
	//
	// 1. namespace: For example, `cloud.storage` and
	//    `google.firebase.analytics`.
	// 2. resource type: The type of resource on which event occurs. For
	//    example, the Google Cloud Storage API includes the type
	// `object`.
	// 3. action: The action that generates the event. For example, action
	// for
	//    a Google Cloud Storage Object is 'change'.
	// These parts are lower case.
	EventType string `json:"eventType,omitempty"`

	// FailurePolicy: Specifies policy for failed executions.
	FailurePolicy *FailurePolicy `json:"failurePolicy,omitempty"`

	// Resource: Required. The resource(s) from which to observe events, for
	// example,
	// `projects/_/buckets/myBucket`.
	//
	// Not all syntactically correct values are accepted by all services.
	// For
	// example:
	//
	// 1. The authorization model must support it. Google Cloud Functions
	//    only allows EventTriggers to be deployed that observe resources in
	// the
	//    same project as the `CloudFunction`.
	// 2. The resource type must match the pattern expected for an
	//    `event_type`. For example, an `EventTrigger` that has an
	//    `event_type` of "google.pubsub.topic.publish" should have a
	// resource
	//    that matches Google Cloud Pub/Sub topics.
	//
	// Additionally, some services may support short names when creating
	// an
	// `EventTrigger`. These will always be returned in the normalized
	// "long"
	// format.
	//
	// See each *service's* documentation for supported formats.
	Resource string `json:"resource,omitempty"`

	// Service: The hostname of the service that should be observed.
	//
	// If no string is provided, the default service implementing the API
	// will
	// be used. For example, `storage.googleapis.com` is the default for
	// all
	// event types in the `google.storage` namespace.
	Service string `json:"service,omitempty"`

	// ForceSendFields is a list of field names (e.g. "EventType") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "EventType") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *EventTrigger) MarshalJSON() ([]byte, error) {
	type NoMethod EventTrigger
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// FailurePolicy: Describes the policy in case of function's execution
// failure.
// If empty, then defaults to ignoring failures (i.e. not retrying
// them).
type FailurePolicy struct {
	// Retry: If specified, then the function will be retried in case of a
	// failure.
	Retry *Retry `json:"retry,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Retry") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Retry") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *FailurePolicy) MarshalJSON() ([]byte, error) {
	type NoMethod FailurePolicy
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GenerateDownloadUrlRequest: Request of `GenerateDownloadUrl` method.
type GenerateDownloadUrlRequest struct {
	// VersionId: The optional version of function. If not set, default,
	// current version
	// is used.
	VersionId uint64 `json:"versionId,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "VersionId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "VersionId") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GenerateDownloadUrlRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GenerateDownloadUrlRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GenerateDownloadUrlResponse: Response of `GenerateDownloadUrl`
// method.
type GenerateDownloadUrlResponse struct {
	// DownloadUrl: The generated Google Cloud Storage signed URL that
	// should be used for
	// function source code download.
	DownloadUrl string `json:"downloadUrl,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "DownloadUrl") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DownloadUrl") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GenerateDownloadUrlResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GenerateDownloadUrlResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GenerateUploadUrlRequest: Request of `GenerateSourceUploadUrl`
// method.
type GenerateUploadUrlRequest struct {
}

// GenerateUploadUrlResponse: Response of `GenerateSourceUploadUrl`
// method.
type GenerateUploadUrlResponse struct {
	// UploadUrl: The generated Google Cloud Storage signed URL that should
	// be used for a
	// function source code upload. The uploaded file should be a zip
	// archive
	// which contains a function.
	UploadUrl string `json:"uploadUrl,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "UploadUrl") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "UploadUrl") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GenerateUploadUrlResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GenerateUploadUrlResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// HttpsTrigger: Describes HttpsTrigger, could be used to connect web
// hooks to function.
type HttpsTrigger struct {
	// Url: Output only. The deployed url for the function.
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

func (s *HttpsTrigger) MarshalJSON() ([]byte, error) {
	type NoMethod HttpsTrigger
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// ListFunctionsResponse: Response for the `ListFunctions` method.
type ListFunctionsResponse struct {
	// Functions: The functions that match the request.
	Functions []*CloudFunction `json:"functions,omitempty"`

	// NextPageToken: If not empty, indicates that there may be more
	// functions that match
	// the request; this value should be passed in a
	// new
	// google.cloud.functions.v1.ListFunctionsRequest
	// to get more functions.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Functions") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Functions") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *ListFunctionsResponse) MarshalJSON() ([]byte, error) {
	type NoMethod ListFunctionsResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// ListLocationsResponse: The response message for
// Locations.ListLocations.
type ListLocationsResponse struct {
	// Locations: A list of locations that matches the specified filter in
	// the request.
	Locations []*Location `json:"locations,omitempty"`

	// NextPageToken: The standard List next-page token.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Locations") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Locations") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *ListLocationsResponse) MarshalJSON() ([]byte, error) {
	type NoMethod ListLocationsResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// ListOperationsResponse: The response message for
// Operations.ListOperations.
type ListOperationsResponse struct {
	// NextPageToken: The standard List next-page token.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// Operations: A list of operations that matches the specified filter in
	// the request.
	Operations []*Operation `json:"operations,omitempty"`

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

func (s *ListOperationsResponse) MarshalJSON() ([]byte, error) {
	type NoMethod ListOperationsResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Location: A resource that represents Google Cloud Platform location.
type Location struct {
	// DisplayName: The friendly name for this location, typically a nearby
	// city name.
	// For example, "Tokyo".
	DisplayName string `json:"displayName,omitempty"`

	// Labels: Cross-service attributes for the location. For example
	//
	//     {"cloud.googleapis.com/region": "us-east1"}
	Labels map[string]string `json:"labels,omitempty"`

	// LocationId: The canonical id for this location. For example:
	// "us-east1".
	LocationId string `json:"locationId,omitempty"`

	// Metadata: Service-specific metadata. For example the available
	// capacity at the given
	// location.
	Metadata googleapi.RawMessage `json:"metadata,omitempty"`

	// Name: Resource name for the location, which may vary between
	// implementations.
	// For example: "projects/example-project/locations/us-east1"
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

func (s *Location) MarshalJSON() ([]byte, error) {
	type NoMethod Location
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Operation: This resource represents a long-running operation that is
// the result of a
// network API call.
type Operation struct {
	// Done: If the value is `false`, it means the operation is still in
	// progress.
	// If `true`, the operation is completed, and either `error` or
	// `response` is
	// available.
	Done bool `json:"done,omitempty"`

	// Error: The error result of the operation in case of failure or
	// cancellation.
	Error *Status `json:"error,omitempty"`

	// Metadata: Service-specific metadata associated with the operation.
	// It typically
	// contains progress information and common metadata such as create
	// time.
	// Some services might not provide such metadata.  Any method that
	// returns a
	// long-running operation should document the metadata type, if any.
	Metadata googleapi.RawMessage `json:"metadata,omitempty"`

	// Name: The server-assigned name, which is only unique within the same
	// service that
	// originally returns it. If you use the default HTTP mapping,
	// the
	// `name` should have the format of `operations/some/unique/name`.
	Name string `json:"name,omitempty"`

	// Response: The normal response of the operation in case of success.
	// If the original
	// method returns no data on success, such as `Delete`, the response
	// is
	// `google.protobuf.Empty`.  If the original method is
	// standard
	// `Get`/`Create`/`Update`, the response should be the resource.  For
	// other
	// methods, the response should have the type `XxxResponse`, where
	// `Xxx`
	// is the original method name.  For example, if the original method
	// name
	// is `TakeSnapshot()`, the inferred response type
	// is
	// `TakeSnapshotResponse`.
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

func (s *Operation) MarshalJSON() ([]byte, error) {
	type NoMethod Operation
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// OperationMetadataV1: Metadata describing an Operation
type OperationMetadataV1 struct {
	// Request: The original request that started the operation.
	Request googleapi.RawMessage `json:"request,omitempty"`

	// Target: Target of the operation - for
	// example
	// projects/project-1/locations/region-1/functions/function-1
	Target string `json:"target,omitempty"`

	// Type: Type of operation.
	//
	// Possible values:
	//   "OPERATION_UNSPECIFIED" - Unknown operation type.
	//   "CREATE_FUNCTION" - Triggered by CreateFunction call
	//   "UPDATE_FUNCTION" - Triggered by UpdateFunction call
	//   "DELETE_FUNCTION" - Triggered by DeleteFunction call.
	Type string `json:"type,omitempty"`

	// UpdateTime: The last update timestamp of the operation.
	UpdateTime string `json:"updateTime,omitempty"`

	// VersionId: Version id of the function created or updated by an API
	// call.
	// This field is only pupulated for Create and Update operations.
	VersionId int64 `json:"versionId,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "Request") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Request") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *OperationMetadataV1) MarshalJSON() ([]byte, error) {
	type NoMethod OperationMetadataV1
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// OperationMetadataV1Beta2: Metadata describing an Operation
type OperationMetadataV1Beta2 struct {
	// Request: The original request that started the operation.
	Request googleapi.RawMessage `json:"request,omitempty"`

	// Target: Target of the operation - for
	// example
	// projects/project-1/locations/region-1/functions/function-1
	Target string `json:"target,omitempty"`

	// Type: Type of operation.
	//
	// Possible values:
	//   "OPERATION_UNSPECIFIED" - Unknown operation type.
	//   "CREATE_FUNCTION" - Triggered by CreateFunction call
	//   "UPDATE_FUNCTION" - Triggered by UpdateFunction call
	//   "DELETE_FUNCTION" - Triggered by DeleteFunction call.
	Type string `json:"type,omitempty"`

	// UpdateTime: The last update timestamp of the operation.
	UpdateTime string `json:"updateTime,omitempty"`

	// VersionId: Version id of the function created or updated by an API
	// call.
	// This field is only pupulated for Create and Update operations.
	VersionId int64 `json:"versionId,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "Request") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Request") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *OperationMetadataV1Beta2) MarshalJSON() ([]byte, error) {
	type NoMethod OperationMetadataV1Beta2
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Retry: Describes the retry policy in case of function's execution
// failure.
// A function execution will be retried on any failure.
// A failed execution will be retried up to 7 days with an exponential
// backoff
// (capped at 10 seconds).
// Retried execution is charged as any other execution.
type Retry struct {
}

// SourceRepository: Describes SourceRepository, used to represent
// parameters related to
// source repository where a function is hosted.
type SourceRepository struct {
	// DeployedUrl: Output only. The URL pointing to the hosted repository
	// where the function
	// were defined at the time of deployment. It always points to a
	// specific
	// commit in the format described above.
	DeployedUrl string `json:"deployedUrl,omitempty"`

	// Url: The URL pointing to the hosted repository where the function is
	// defined.
	// There are supported Cloud Source Repository URLs in the
	// following
	// formats:
	//
	// To refer to a specific
	// commit:
	// `https://source.developers.google.com/projects/*/repos/*/revis
	// ions/*/paths/*`
	// To refer to a moveable alias
	// (branch):
	// `https://source.developers.google.com/projects/*/repos/*/mov
	// eable-aliases/*/paths/*`
	// In particular, to refer to HEAD use `master` moveable alias.
	// To refer to a specific fixed alias
	// (tag):
	// `https://source.developers.google.com/projects/*/repos/*/fixed-
	// aliases/*/paths/*`
	//
	// You may omit `paths/*` if you want to use the main directory.
	Url string `json:"url,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DeployedUrl") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DeployedUrl") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *SourceRepository) MarshalJSON() ([]byte, error) {
	type NoMethod SourceRepository
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

// method id "cloudfunctions.operations.get":

type OperationsGetCall struct {
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
func (r *OperationsService) Get(name string) *OperationsGetCall {
	c := &OperationsGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *OperationsGetCall) Fields(s ...googleapi.Field) *OperationsGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *OperationsGetCall) IfNoneMatch(entityTag string) *OperationsGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *OperationsGetCall) Context(ctx context.Context) *OperationsGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *OperationsGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *OperationsGetCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudfunctions.operations.get" call.
// Exactly one of *Operation or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *Operation.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *OperationsGetCall) Do(opts ...googleapi.CallOption) (*Operation, error) {
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
	ret := &Operation{
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
	//   "description": "Gets the latest state of a long-running operation.  Clients can use this\nmethod to poll the operation result at intervals as recommended by the API\nservice.",
	//   "flatPath": "v1/operations/{operationsId}",
	//   "httpMethod": "GET",
	//   "id": "cloudfunctions.operations.get",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the operation resource.",
	//       "location": "path",
	//       "pattern": "^operations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}",
	//   "response": {
	//     "$ref": "Operation"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "cloudfunctions.operations.list":

type OperationsListCall struct {
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists operations that match the specified filter in the
// request. If the
// server doesn't support this method, it returns
// `UNIMPLEMENTED`.
//
// NOTE: the `name` binding allows API services to override the
// binding
// to use different resource name schemes, such as `users/*/operations`.
// To
// override the binding, API services can add a binding such
// as
// "/v1/{name=users/*}/operations" to their service configuration.
// For backwards compatibility, the default name includes the
// operations
// collection id, however overriding users must ensure the name
// binding
// is the parent resource, without the operations collection id.
func (r *OperationsService) List() *OperationsListCall {
	c := &OperationsListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// Filter sets the optional parameter "filter": The standard list
// filter.
func (c *OperationsListCall) Filter(filter string) *OperationsListCall {
	c.urlParams_.Set("filter", filter)
	return c
}

// Name sets the optional parameter "name": The name of the operation's
// parent resource.
func (c *OperationsListCall) Name(name string) *OperationsListCall {
	c.urlParams_.Set("name", name)
	return c
}

// PageSize sets the optional parameter "pageSize": The standard list
// page size.
func (c *OperationsListCall) PageSize(pageSize int64) *OperationsListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": The standard list
// page token.
func (c *OperationsListCall) PageToken(pageToken string) *OperationsListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *OperationsListCall) Fields(s ...googleapi.Field) *OperationsListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *OperationsListCall) IfNoneMatch(entityTag string) *OperationsListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *OperationsListCall) Context(ctx context.Context) *OperationsListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *OperationsListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *OperationsListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/operations")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudfunctions.operations.list" call.
// Exactly one of *ListOperationsResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *ListOperationsResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *OperationsListCall) Do(opts ...googleapi.CallOption) (*ListOperationsResponse, error) {
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
	ret := &ListOperationsResponse{
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
	//   "description": "Lists operations that match the specified filter in the request. If the\nserver doesn't support this method, it returns `UNIMPLEMENTED`.\n\nNOTE: the `name` binding allows API services to override the binding\nto use different resource name schemes, such as `users/*/operations`. To\noverride the binding, API services can add a binding such as\n`\"/v1/{name=users/*}/operations\"` to their service configuration.\nFor backwards compatibility, the default name includes the operations\ncollection id, however overriding users must ensure the name binding\nis the parent resource, without the operations collection id.",
	//   "flatPath": "v1/operations",
	//   "httpMethod": "GET",
	//   "id": "cloudfunctions.operations.list",
	//   "parameterOrder": [],
	//   "parameters": {
	//     "filter": {
	//       "description": "The standard list filter.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "name": {
	//       "description": "The name of the operation's parent resource.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "pageSize": {
	//       "description": "The standard list page size.",
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
	//   "path": "v1/operations",
	//   "response": {
	//     "$ref": "ListOperationsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *OperationsListCall) Pages(ctx context.Context, f func(*ListOperationsResponse) error) error {
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

// method id "cloudfunctions.projects.locations.list":

type ProjectsLocationsListCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists information about the supported locations for this
// service.
func (r *ProjectsLocationsService) List(name string) *ProjectsLocationsListCall {
	c := &ProjectsLocationsListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Filter sets the optional parameter "filter": The standard list
// filter.
func (c *ProjectsLocationsListCall) Filter(filter string) *ProjectsLocationsListCall {
	c.urlParams_.Set("filter", filter)
	return c
}

// PageSize sets the optional parameter "pageSize": The standard list
// page size.
func (c *ProjectsLocationsListCall) PageSize(pageSize int64) *ProjectsLocationsListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": The standard list
// page token.
func (c *ProjectsLocationsListCall) PageToken(pageToken string) *ProjectsLocationsListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsListCall) Fields(s ...googleapi.Field) *ProjectsLocationsListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsLocationsListCall) IfNoneMatch(entityTag string) *ProjectsLocationsListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsListCall) Context(ctx context.Context) *ProjectsLocationsListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+name}/locations")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudfunctions.projects.locations.list" call.
// Exactly one of *ListLocationsResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *ListLocationsResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsListCall) Do(opts ...googleapi.CallOption) (*ListLocationsResponse, error) {
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
	ret := &ListLocationsResponse{
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
	//   "description": "Lists information about the supported locations for this service.",
	//   "flatPath": "v1/projects/{projectsId}/locations",
	//   "httpMethod": "GET",
	//   "id": "cloudfunctions.projects.locations.list",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "filter": {
	//       "description": "The standard list filter.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "name": {
	//       "description": "The resource that owns the locations collection, if applicable.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "pageSize": {
	//       "description": "The standard list page size.",
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
	//   "path": "v1/{+name}/locations",
	//   "response": {
	//     "$ref": "ListLocationsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *ProjectsLocationsListCall) Pages(ctx context.Context, f func(*ListLocationsResponse) error) error {
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

// method id "cloudfunctions.projects.locations.functions.call":

type ProjectsLocationsFunctionsCallCall struct {
	s                   *Service
	name                string
	callfunctionrequest *CallFunctionRequest
	urlParams_          gensupport.URLParams
	ctx_                context.Context
	header_             http.Header
}

// Call: Invokes synchronously deployed function. To be used for
// testing, very
// limited traffic allowed.
func (r *ProjectsLocationsFunctionsService) Call(name string, callfunctionrequest *CallFunctionRequest) *ProjectsLocationsFunctionsCallCall {
	c := &ProjectsLocationsFunctionsCallCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.callfunctionrequest = callfunctionrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsFunctionsCallCall) Fields(s ...googleapi.Field) *ProjectsLocationsFunctionsCallCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsFunctionsCallCall) Context(ctx context.Context) *ProjectsLocationsFunctionsCallCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsFunctionsCallCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsFunctionsCallCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.callfunctionrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+name}:call")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudfunctions.projects.locations.functions.call" call.
// Exactly one of *CallFunctionResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *CallFunctionResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsFunctionsCallCall) Do(opts ...googleapi.CallOption) (*CallFunctionResponse, error) {
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
	ret := &CallFunctionResponse{
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
	//   "description": "Invokes synchronously deployed function. To be used for testing, very\nlimited traffic allowed.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/functions/{functionsId}:call",
	//   "httpMethod": "POST",
	//   "id": "cloudfunctions.projects.locations.functions.call",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the function to be called.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/functions/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}:call",
	//   "request": {
	//     "$ref": "CallFunctionRequest"
	//   },
	//   "response": {
	//     "$ref": "CallFunctionResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "cloudfunctions.projects.locations.functions.create":

type ProjectsLocationsFunctionsCreateCall struct {
	s             *Service
	location      string
	cloudfunction *CloudFunction
	urlParams_    gensupport.URLParams
	ctx_          context.Context
	header_       http.Header
}

// Create: Creates a new function. If a function with the given name
// already exists in
// the specified project, the long running operation will
// return
// `ALREADY_EXISTS` error.
func (r *ProjectsLocationsFunctionsService) Create(location string, cloudfunction *CloudFunction) *ProjectsLocationsFunctionsCreateCall {
	c := &ProjectsLocationsFunctionsCreateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.location = location
	c.cloudfunction = cloudfunction
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsFunctionsCreateCall) Fields(s ...googleapi.Field) *ProjectsLocationsFunctionsCreateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsFunctionsCreateCall) Context(ctx context.Context) *ProjectsLocationsFunctionsCreateCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsFunctionsCreateCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsFunctionsCreateCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.cloudfunction)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+location}/functions")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"location": c.location,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudfunctions.projects.locations.functions.create" call.
// Exactly one of *Operation or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *Operation.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *ProjectsLocationsFunctionsCreateCall) Do(opts ...googleapi.CallOption) (*Operation, error) {
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
	ret := &Operation{
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
	//   "description": "Creates a new function. If a function with the given name already exists in\nthe specified project, the long running operation will return\n`ALREADY_EXISTS` error.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/functions",
	//   "httpMethod": "POST",
	//   "id": "cloudfunctions.projects.locations.functions.create",
	//   "parameterOrder": [
	//     "location"
	//   ],
	//   "parameters": {
	//     "location": {
	//       "description": "The project and location in which the function should be created, specified\nin the format `projects/*/locations/*`",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+location}/functions",
	//   "request": {
	//     "$ref": "CloudFunction"
	//   },
	//   "response": {
	//     "$ref": "Operation"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "cloudfunctions.projects.locations.functions.delete":

type ProjectsLocationsFunctionsDeleteCall struct {
	s          *Service
	name       string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
	header_    http.Header
}

// Delete: Deletes a function with the given name from the specified
// project. If the
// given function is used by some trigger, the trigger will be updated
// to
// remove this function.
func (r *ProjectsLocationsFunctionsService) Delete(name string) *ProjectsLocationsFunctionsDeleteCall {
	c := &ProjectsLocationsFunctionsDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsFunctionsDeleteCall) Fields(s ...googleapi.Field) *ProjectsLocationsFunctionsDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsFunctionsDeleteCall) Context(ctx context.Context) *ProjectsLocationsFunctionsDeleteCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsFunctionsDeleteCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsFunctionsDeleteCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("DELETE", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudfunctions.projects.locations.functions.delete" call.
// Exactly one of *Operation or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *Operation.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *ProjectsLocationsFunctionsDeleteCall) Do(opts ...googleapi.CallOption) (*Operation, error) {
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
	ret := &Operation{
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
	//   "description": "Deletes a function with the given name from the specified project. If the\ngiven function is used by some trigger, the trigger will be updated to\nremove this function.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/functions/{functionsId}",
	//   "httpMethod": "DELETE",
	//   "id": "cloudfunctions.projects.locations.functions.delete",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the function which should be deleted.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/functions/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}",
	//   "response": {
	//     "$ref": "Operation"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "cloudfunctions.projects.locations.functions.generateDownloadUrl":

type ProjectsLocationsFunctionsGenerateDownloadUrlCall struct {
	s                          *Service
	name                       string
	generatedownloadurlrequest *GenerateDownloadUrlRequest
	urlParams_                 gensupport.URLParams
	ctx_                       context.Context
	header_                    http.Header
}

// GenerateDownloadUrl: Returns a signed URL for downloading deployed
// function source code.
// The URL is only valid for a limited period and should be used
// within
// minutes after generation.
// For more information about the signed URL usage
// see:
// https://cloud.google.com/storage/docs/access-control/signed-urls
func (r *ProjectsLocationsFunctionsService) GenerateDownloadUrl(name string, generatedownloadurlrequest *GenerateDownloadUrlRequest) *ProjectsLocationsFunctionsGenerateDownloadUrlCall {
	c := &ProjectsLocationsFunctionsGenerateDownloadUrlCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.generatedownloadurlrequest = generatedownloadurlrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsFunctionsGenerateDownloadUrlCall) Fields(s ...googleapi.Field) *ProjectsLocationsFunctionsGenerateDownloadUrlCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsFunctionsGenerateDownloadUrlCall) Context(ctx context.Context) *ProjectsLocationsFunctionsGenerateDownloadUrlCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsFunctionsGenerateDownloadUrlCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsFunctionsGenerateDownloadUrlCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.generatedownloadurlrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+name}:generateDownloadUrl")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudfunctions.projects.locations.functions.generateDownloadUrl" call.
// Exactly one of *GenerateDownloadUrlResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *GenerateDownloadUrlResponse.ServerResponse.Header or (if a response
// was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsFunctionsGenerateDownloadUrlCall) Do(opts ...googleapi.CallOption) (*GenerateDownloadUrlResponse, error) {
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
	ret := &GenerateDownloadUrlResponse{
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
	//   "description": "Returns a signed URL for downloading deployed function source code.\nThe URL is only valid for a limited period and should be used within\nminutes after generation.\nFor more information about the signed URL usage see:\nhttps://cloud.google.com/storage/docs/access-control/signed-urls",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/functions/{functionsId}:generateDownloadUrl",
	//   "httpMethod": "POST",
	//   "id": "cloudfunctions.projects.locations.functions.generateDownloadUrl",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of function for which source code Google Cloud Storage signed\nURL should be generated.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/functions/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}:generateDownloadUrl",
	//   "request": {
	//     "$ref": "GenerateDownloadUrlRequest"
	//   },
	//   "response": {
	//     "$ref": "GenerateDownloadUrlResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "cloudfunctions.projects.locations.functions.generateUploadUrl":

type ProjectsLocationsFunctionsGenerateUploadUrlCall struct {
	s                        *Service
	parent                   string
	generateuploadurlrequest *GenerateUploadUrlRequest
	urlParams_               gensupport.URLParams
	ctx_                     context.Context
	header_                  http.Header
}

// GenerateUploadUrl: Returns a signed URL for uploading a function
// source code.
// For more information about the signed URL usage
// see:
// https://cloud.google.com/storage/docs/access-control/signed-urls.
//
// Once the function source code upload is complete, the used signed
// URL should be provided in CreateFunction or UpdateFunction request
// as a reference to the function source code.
//
// When uploading source code to the generated signed URL, please
// follow
// these restrictions:
//
// * Source file type should be a zip file.
// * Source file size should not exceed 100MB limit.
//
// When making a HTTP PUT request, these two headers need to be
// specified:
//
// * `content-type: application/zip`
// * `x-goog-content-length-range: 0,104857600`
func (r *ProjectsLocationsFunctionsService) GenerateUploadUrl(parent string, generateuploadurlrequest *GenerateUploadUrlRequest) *ProjectsLocationsFunctionsGenerateUploadUrlCall {
	c := &ProjectsLocationsFunctionsGenerateUploadUrlCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.generateuploadurlrequest = generateuploadurlrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsFunctionsGenerateUploadUrlCall) Fields(s ...googleapi.Field) *ProjectsLocationsFunctionsGenerateUploadUrlCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsFunctionsGenerateUploadUrlCall) Context(ctx context.Context) *ProjectsLocationsFunctionsGenerateUploadUrlCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsFunctionsGenerateUploadUrlCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsFunctionsGenerateUploadUrlCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.generateuploadurlrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+parent}/functions:generateUploadUrl")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudfunctions.projects.locations.functions.generateUploadUrl" call.
// Exactly one of *GenerateUploadUrlResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *GenerateUploadUrlResponse.ServerResponse.Header or (if a response
// was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsFunctionsGenerateUploadUrlCall) Do(opts ...googleapi.CallOption) (*GenerateUploadUrlResponse, error) {
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
	ret := &GenerateUploadUrlResponse{
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
	//   "description": "Returns a signed URL for uploading a function source code.\nFor more information about the signed URL usage see:\nhttps://cloud.google.com/storage/docs/access-control/signed-urls.\nOnce the function source code upload is complete, the used signed\nURL should be provided in CreateFunction or UpdateFunction request\nas a reference to the function source code.\n\nWhen uploading source code to the generated signed URL, please follow\nthese restrictions:\n\n* Source file type should be a zip file.\n* Source file size should not exceed 100MB limit.\n\nWhen making a HTTP PUT request, these two headers need to be specified:\n\n* `content-type: application/zip`\n* `x-goog-content-length-range: 0,104857600`",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/functions:generateUploadUrl",
	//   "httpMethod": "POST",
	//   "id": "cloudfunctions.projects.locations.functions.generateUploadUrl",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The project and location in which the Google Cloud Storage signed URL\nshould be generated, specified in the format `projects/*/locations/*`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+parent}/functions:generateUploadUrl",
	//   "request": {
	//     "$ref": "GenerateUploadUrlRequest"
	//   },
	//   "response": {
	//     "$ref": "GenerateUploadUrlResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "cloudfunctions.projects.locations.functions.get":

type ProjectsLocationsFunctionsGetCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// Get: Returns a function with the given name from the requested
// project.
func (r *ProjectsLocationsFunctionsService) Get(name string) *ProjectsLocationsFunctionsGetCall {
	c := &ProjectsLocationsFunctionsGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsFunctionsGetCall) Fields(s ...googleapi.Field) *ProjectsLocationsFunctionsGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsLocationsFunctionsGetCall) IfNoneMatch(entityTag string) *ProjectsLocationsFunctionsGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsFunctionsGetCall) Context(ctx context.Context) *ProjectsLocationsFunctionsGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsFunctionsGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsFunctionsGetCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudfunctions.projects.locations.functions.get" call.
// Exactly one of *CloudFunction or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *CloudFunction.ServerResponse.Header or (if a response was returned
// at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsFunctionsGetCall) Do(opts ...googleapi.CallOption) (*CloudFunction, error) {
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
	ret := &CloudFunction{
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
	//   "description": "Returns a function with the given name from the requested project.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/functions/{functionsId}",
	//   "httpMethod": "GET",
	//   "id": "cloudfunctions.projects.locations.functions.get",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the function which details should be obtained.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/functions/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}",
	//   "response": {
	//     "$ref": "CloudFunction"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "cloudfunctions.projects.locations.functions.list":

type ProjectsLocationsFunctionsListCall struct {
	s            *Service
	parent       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Returns a list of functions that belong to the requested
// project.
func (r *ProjectsLocationsFunctionsService) List(parent string) *ProjectsLocationsFunctionsListCall {
	c := &ProjectsLocationsFunctionsListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	return c
}

// PageSize sets the optional parameter "pageSize": Maximum number of
// functions to return per call.
func (c *ProjectsLocationsFunctionsListCall) PageSize(pageSize int64) *ProjectsLocationsFunctionsListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": The value returned
// by the last
// `ListFunctionsResponse`; indicates that
// this is a continuation of a prior `ListFunctions` call, and that
// the
// system should return the next page of data.
func (c *ProjectsLocationsFunctionsListCall) PageToken(pageToken string) *ProjectsLocationsFunctionsListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsFunctionsListCall) Fields(s ...googleapi.Field) *ProjectsLocationsFunctionsListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsLocationsFunctionsListCall) IfNoneMatch(entityTag string) *ProjectsLocationsFunctionsListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsFunctionsListCall) Context(ctx context.Context) *ProjectsLocationsFunctionsListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsFunctionsListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsFunctionsListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+parent}/functions")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudfunctions.projects.locations.functions.list" call.
// Exactly one of *ListFunctionsResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *ListFunctionsResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsFunctionsListCall) Do(opts ...googleapi.CallOption) (*ListFunctionsResponse, error) {
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
	ret := &ListFunctionsResponse{
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
	//   "description": "Returns a list of functions that belong to the requested project.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/functions",
	//   "httpMethod": "GET",
	//   "id": "cloudfunctions.projects.locations.functions.list",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "pageSize": {
	//       "description": "Maximum number of functions to return per call.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "description": "The value returned by the last\n`ListFunctionsResponse`; indicates that\nthis is a continuation of a prior `ListFunctions` call, and that the\nsystem should return the next page of data.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "parent": {
	//       "description": "The project and location from which the function should be listed,\nspecified in the format `projects/*/locations/*`\nIf you want to list functions in all locations, use \"-\" in place of a\nlocation.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+parent}/functions",
	//   "response": {
	//     "$ref": "ListFunctionsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *ProjectsLocationsFunctionsListCall) Pages(ctx context.Context, f func(*ListFunctionsResponse) error) error {
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

// method id "cloudfunctions.projects.locations.functions.patch":

type ProjectsLocationsFunctionsPatchCall struct {
	s             *Service
	name          string
	cloudfunction *CloudFunction
	urlParams_    gensupport.URLParams
	ctx_          context.Context
	header_       http.Header
}

// Patch: Updates existing function.
func (r *ProjectsLocationsFunctionsService) Patch(name string, cloudfunction *CloudFunction) *ProjectsLocationsFunctionsPatchCall {
	c := &ProjectsLocationsFunctionsPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.cloudfunction = cloudfunction
	return c
}

// UpdateMask sets the optional parameter "updateMask": Required list of
// fields to be updated in this request.
func (c *ProjectsLocationsFunctionsPatchCall) UpdateMask(updateMask string) *ProjectsLocationsFunctionsPatchCall {
	c.urlParams_.Set("updateMask", updateMask)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsFunctionsPatchCall) Fields(s ...googleapi.Field) *ProjectsLocationsFunctionsPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsFunctionsPatchCall) Context(ctx context.Context) *ProjectsLocationsFunctionsPatchCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsFunctionsPatchCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsFunctionsPatchCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.cloudfunction)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudfunctions.projects.locations.functions.patch" call.
// Exactly one of *Operation or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *Operation.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *ProjectsLocationsFunctionsPatchCall) Do(opts ...googleapi.CallOption) (*Operation, error) {
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
	ret := &Operation{
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
	//   "description": "Updates existing function.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/functions/{functionsId}",
	//   "httpMethod": "PATCH",
	//   "id": "cloudfunctions.projects.locations.functions.patch",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "A user-defined name of the function. Function names must be unique\nglobally and match pattern `projects/*/locations/*/functions/*`",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/functions/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "updateMask": {
	//       "description": "Required list of fields to be updated in this request.",
	//       "format": "google-fieldmask",
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}",
	//   "request": {
	//     "$ref": "CloudFunction"
	//   },
	//   "response": {
	//     "$ref": "Operation"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}
