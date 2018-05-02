// Package dlp provides access to the Cloud Data Loss Prevention (DLP) API.
//
// See https://cloud.google.com/dlp/docs/
//
// Usage example:
//
//   import "google.golang.org/api/dlp/v2beta2"
//   ...
//   dlpService, err := dlp.New(oauthHttpClient)
package dlp // import "google.golang.org/api/dlp/v2beta2"

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

const apiId = "dlp:v2beta2"
const apiName = "dlp"
const apiVersion = "v2beta2"
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
	s.InfoTypes = NewInfoTypesService(s)
	s.Organizations = NewOrganizationsService(s)
	s.Projects = NewProjectsService(s)
	return s, nil
}

type Service struct {
	client    *http.Client
	BasePath  string // API endpoint base URL
	UserAgent string // optional additional User-Agent fragment

	InfoTypes *InfoTypesService

	Organizations *OrganizationsService

	Projects *ProjectsService
}

func (s *Service) userAgent() string {
	if s.UserAgent == "" {
		return googleapi.UserAgent
	}
	return googleapi.UserAgent + " " + s.UserAgent
}

func NewInfoTypesService(s *Service) *InfoTypesService {
	rs := &InfoTypesService{s: s}
	return rs
}

type InfoTypesService struct {
	s *Service
}

func NewOrganizationsService(s *Service) *OrganizationsService {
	rs := &OrganizationsService{s: s}
	rs.DeidentifyTemplates = NewOrganizationsDeidentifyTemplatesService(s)
	rs.InspectTemplates = NewOrganizationsInspectTemplatesService(s)
	return rs
}

type OrganizationsService struct {
	s *Service

	DeidentifyTemplates *OrganizationsDeidentifyTemplatesService

	InspectTemplates *OrganizationsInspectTemplatesService
}

func NewOrganizationsDeidentifyTemplatesService(s *Service) *OrganizationsDeidentifyTemplatesService {
	rs := &OrganizationsDeidentifyTemplatesService{s: s}
	return rs
}

type OrganizationsDeidentifyTemplatesService struct {
	s *Service
}

func NewOrganizationsInspectTemplatesService(s *Service) *OrganizationsInspectTemplatesService {
	rs := &OrganizationsInspectTemplatesService{s: s}
	return rs
}

type OrganizationsInspectTemplatesService struct {
	s *Service
}

func NewProjectsService(s *Service) *ProjectsService {
	rs := &ProjectsService{s: s}
	rs.Content = NewProjectsContentService(s)
	rs.DataSource = NewProjectsDataSourceService(s)
	rs.DeidentifyTemplates = NewProjectsDeidentifyTemplatesService(s)
	rs.DlpJobs = NewProjectsDlpJobsService(s)
	rs.Image = NewProjectsImageService(s)
	rs.InspectTemplates = NewProjectsInspectTemplatesService(s)
	rs.JobTriggers = NewProjectsJobTriggersService(s)
	return rs
}

type ProjectsService struct {
	s *Service

	Content *ProjectsContentService

	DataSource *ProjectsDataSourceService

	DeidentifyTemplates *ProjectsDeidentifyTemplatesService

	DlpJobs *ProjectsDlpJobsService

	Image *ProjectsImageService

	InspectTemplates *ProjectsInspectTemplatesService

	JobTriggers *ProjectsJobTriggersService
}

func NewProjectsContentService(s *Service) *ProjectsContentService {
	rs := &ProjectsContentService{s: s}
	return rs
}

type ProjectsContentService struct {
	s *Service
}

func NewProjectsDataSourceService(s *Service) *ProjectsDataSourceService {
	rs := &ProjectsDataSourceService{s: s}
	return rs
}

type ProjectsDataSourceService struct {
	s *Service
}

func NewProjectsDeidentifyTemplatesService(s *Service) *ProjectsDeidentifyTemplatesService {
	rs := &ProjectsDeidentifyTemplatesService{s: s}
	return rs
}

type ProjectsDeidentifyTemplatesService struct {
	s *Service
}

func NewProjectsDlpJobsService(s *Service) *ProjectsDlpJobsService {
	rs := &ProjectsDlpJobsService{s: s}
	return rs
}

type ProjectsDlpJobsService struct {
	s *Service
}

func NewProjectsImageService(s *Service) *ProjectsImageService {
	rs := &ProjectsImageService{s: s}
	return rs
}

type ProjectsImageService struct {
	s *Service
}

func NewProjectsInspectTemplatesService(s *Service) *ProjectsInspectTemplatesService {
	rs := &ProjectsInspectTemplatesService{s: s}
	return rs
}

type ProjectsInspectTemplatesService struct {
	s *Service
}

func NewProjectsJobTriggersService(s *Service) *ProjectsJobTriggersService {
	rs := &ProjectsJobTriggersService{s: s}
	return rs
}

type ProjectsJobTriggersService struct {
	s *Service
}

// GooglePrivacyDlpV2beta1AuxiliaryTable: An auxiliary table contains
// statistical information on the relative
// frequency of different quasi-identifiers values. It has one or
// several
// quasi-identifiers columns, and one column that indicates the
// relative
// frequency of each quasi-identifier tuple.
// If a tuple is present in the data but not in the auxiliary table,
// the
// corresponding relative frequency is assumed to be zero (and thus,
// the
// tuple is highly reidentifiable).
type GooglePrivacyDlpV2beta1AuxiliaryTable struct {
	// QuasiIds: Quasi-identifier columns. [required]
	QuasiIds []*GooglePrivacyDlpV2beta1QuasiIdField `json:"quasiIds,omitempty"`

	// RelativeFrequency: The relative frequency column must contain a
	// floating-point number
	// between 0 and 1 (inclusive). Null values are assumed to be
	// zero.
	// [required]
	RelativeFrequency *GooglePrivacyDlpV2beta1FieldId `json:"relativeFrequency,omitempty"`

	// Table: Auxiliary table location. [required]
	Table *GooglePrivacyDlpV2beta1BigQueryTable `json:"table,omitempty"`

	// ForceSendFields is a list of field names (e.g. "QuasiIds") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "QuasiIds") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1AuxiliaryTable) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1AuxiliaryTable
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1BigQueryOptions: Options defining BigQuery
// table and row identifiers.
type GooglePrivacyDlpV2beta1BigQueryOptions struct {
	// IdentifyingFields: References to fields uniquely identifying rows
	// within the table.
	// Nested fields in the format, like `person.birthdate.year`, are
	// allowed.
	IdentifyingFields []*GooglePrivacyDlpV2beta1FieldId `json:"identifyingFields,omitempty"`

	// TableReference: Complete BigQuery table reference.
	TableReference *GooglePrivacyDlpV2beta1BigQueryTable `json:"tableReference,omitempty"`

	// ForceSendFields is a list of field names (e.g. "IdentifyingFields")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "IdentifyingFields") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1BigQueryOptions) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1BigQueryOptions
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1BigQueryTable: Message defining the location
// of a BigQuery table. A table is uniquely
// identified  by its project_id, dataset_id, and table_name. Within a
// query
// a table is often referenced with a string in the format
// of:
// `<project_id>:<dataset_id>.<table_id>`
// or
// `<project_id>.<dataset_id>.<table_id>`.
type GooglePrivacyDlpV2beta1BigQueryTable struct {
	// DatasetId: Dataset ID of the table.
	DatasetId string `json:"datasetId,omitempty"`

	// ProjectId: The Google Cloud Platform project ID of the project
	// containing the table.
	// If omitted, project ID is inferred from the API call.
	ProjectId string `json:"projectId,omitempty"`

	// TableId: Name of the table.
	TableId string `json:"tableId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DatasetId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DatasetId") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1BigQueryTable) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1BigQueryTable
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1CategoricalStatsConfig: Compute numerical
// stats over an individual column, including
// number of distinct values and value count distribution.
type GooglePrivacyDlpV2beta1CategoricalStatsConfig struct {
	// Field: Field to compute categorical stats on. All column types
	// are
	// supported except for arrays and structs. However, it may be
	// more
	// informative to use NumericalStats when the field type is
	// supported,
	// depending on the data.
	Field *GooglePrivacyDlpV2beta1FieldId `json:"field,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Field") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Field") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1CategoricalStatsConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1CategoricalStatsConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1CategoricalStatsHistogramBucket: Histogram
// bucket of value frequencies in the column.
type GooglePrivacyDlpV2beta1CategoricalStatsHistogramBucket struct {
	// BucketSize: Total number of records in this bucket.
	BucketSize int64 `json:"bucketSize,omitempty,string"`

	// BucketValues: Sample of value frequencies in this bucket. The total
	// number of
	// values returned per bucket is capped at 20.
	BucketValues []*GooglePrivacyDlpV2beta1ValueFrequency `json:"bucketValues,omitempty"`

	// ValueFrequencyLowerBound: Lower bound on the value frequency of the
	// values in this bucket.
	ValueFrequencyLowerBound int64 `json:"valueFrequencyLowerBound,omitempty,string"`

	// ValueFrequencyUpperBound: Upper bound on the value frequency of the
	// values in this bucket.
	ValueFrequencyUpperBound int64 `json:"valueFrequencyUpperBound,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "BucketSize") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BucketSize") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1CategoricalStatsHistogramBucket) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1CategoricalStatsHistogramBucket
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1CategoricalStatsResult: Result of the
// categorical stats computation.
type GooglePrivacyDlpV2beta1CategoricalStatsResult struct {
	// ValueFrequencyHistogramBuckets: Histogram of value frequencies in the
	// column.
	ValueFrequencyHistogramBuckets []*GooglePrivacyDlpV2beta1CategoricalStatsHistogramBucket `json:"valueFrequencyHistogramBuckets,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "ValueFrequencyHistogramBuckets") to unconditionally include in API
	// requests. By default, fields with empty values are omitted from API
	// requests. However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g.
	// "ValueFrequencyHistogramBuckets") to include in API requests with the
	// JSON null value. By default, fields with empty values are omitted
	// from API requests. However, any field with an empty value appearing
	// in NullFields will be sent to the server as null. It is an error if a
	// field in this list has a non-empty value. This may be used to include
	// null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1CategoricalStatsResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1CategoricalStatsResult
	raw := NoMethod(*s)
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
	type NoMethod GooglePrivacyDlpV2beta1CloudStorageOptions
	raw := NoMethod(*s)
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
	type NoMethod GooglePrivacyDlpV2beta1CloudStoragePath
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1CustomInfoType: Custom information type
// provided by the user. Used to find domain-specific
// sensitive information configurable to the data in question.
type GooglePrivacyDlpV2beta1CustomInfoType struct {
	// Dictionary: Dictionary-based custom info type.
	Dictionary *GooglePrivacyDlpV2beta1Dictionary `json:"dictionary,omitempty"`

	// InfoType: Info type configuration. All custom info types must have
	// configurations
	// that do not conflict with built-in info types or other custom info
	// types.
	InfoType *GooglePrivacyDlpV2beta1InfoType `json:"infoType,omitempty"`

	// SurrogateType: Surrogate info type.
	SurrogateType *GooglePrivacyDlpV2beta1SurrogateType `json:"surrogateType,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Dictionary") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Dictionary") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1CustomInfoType) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1CustomInfoType
	raw := NoMethod(*s)
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
	type NoMethod GooglePrivacyDlpV2beta1DatastoreOptions
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1Dictionary: Custom information type based on a
// dictionary of words or phrases. This can
// be used to match sensitive information specific to the data, such as
// a list
// of employee IDs or job titles.
//
// Dictionary words are case-insensitive and all characters other than
// letters
// and digits in the unicode [Basic
// Multilingual
// Plane](https://en.wikipedia.org/wiki/Plane_%28Unicode%29#
// Basic_Multilingual_Plane)
// will be replaced with whitespace when scanning for matches, so
// the
// dictionary phrase "Sam Johnson" will match all three phrases "sam
// johnson",
// "Sam, Johnson", and "Sam (Johnson)". Additionally, the
// characters
// surrounding any match must be of a different type than the
// adjacent
// characters within the word, so letters must be next to non-letters
// and
// digits next to non-digits. For example, the dictionary word "jen"
// will
// match the first three letters of the text "jen123" but will return
// no
// matches for "jennifer".
//
// Dictionary words containing a large number of characters that are
// not
// letters or digits may result in unexpected findings because such
// characters
// are treated as whitespace.
type GooglePrivacyDlpV2beta1Dictionary struct {
	// WordList: List of words or phrases to search for.
	WordList *GooglePrivacyDlpV2beta1WordList `json:"wordList,omitempty"`

	// ForceSendFields is a list of field names (e.g. "WordList") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "WordList") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1Dictionary) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1Dictionary
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1EntityId: An entity in a dataset is a field or
// set of fields that correspond to a
// single person. For example, in medical records the `EntityId` might
// be
// a patient identifier, or for financial records it might be an
// account
// identifier. This message is used when generalizations or analysis
// must be
// consistent across multiple rows pertaining to the same entity.
type GooglePrivacyDlpV2beta1EntityId struct {
	// Field: Composite key indicating which field contains the entity
	// identifier.
	Field *GooglePrivacyDlpV2beta1FieldId `json:"field,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Field") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Field") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1EntityId) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1EntityId
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1FieldId: General identifier of a data field in
// a storage service.
type GooglePrivacyDlpV2beta1FieldId struct {
	// ColumnName: Name describing the field.
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
	type NoMethod GooglePrivacyDlpV2beta1FieldId
	raw := NoMethod(*s)
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
	type NoMethod GooglePrivacyDlpV2beta1FileSet
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1InfoType: Type of information detected by the
// API.
type GooglePrivacyDlpV2beta1InfoType struct {
	// Name: Name of the information type.
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
	type NoMethod GooglePrivacyDlpV2beta1InfoType
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1InfoTypeLimit: Max findings configuration per
// info type, per content item or long running
// operation.
type GooglePrivacyDlpV2beta1InfoTypeLimit struct {
	// InfoType: Type of information the findings limit applies to. Only one
	// limit per
	// info_type should be provided. If InfoTypeLimit does not have
	// an
	// info_type, the DLP API applies the limit against all info_types that
	// are
	// found but not specified in another InfoTypeLimit.
	InfoType *GooglePrivacyDlpV2beta1InfoType `json:"infoType,omitempty"`

	// MaxFindings: Max findings limit for the given infoType.
	MaxFindings int64 `json:"maxFindings,omitempty"`

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

func (s *GooglePrivacyDlpV2beta1InfoTypeLimit) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1InfoTypeLimit
	raw := NoMethod(*s)
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
	type NoMethod GooglePrivacyDlpV2beta1InfoTypeStatistics
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1InspectConfig: Configuration description of
// the scanning process.
// When used with redactContent only info_types and min_likelihood are
// currently
// used.
type GooglePrivacyDlpV2beta1InspectConfig struct {
	// CustomInfoTypes: Custom info types provided by the user.
	CustomInfoTypes []*GooglePrivacyDlpV2beta1CustomInfoType `json:"customInfoTypes,omitempty"`

	// ExcludeTypes: When true, excludes type information of the findings.
	ExcludeTypes bool `json:"excludeTypes,omitempty"`

	// IncludeQuote: When true, a contextual quote from the data that
	// triggered a finding is
	// included in the response; see Finding.quote.
	IncludeQuote bool `json:"includeQuote,omitempty"`

	// InfoTypeLimits: Configuration of findings limit given for specified
	// info types.
	InfoTypeLimits []*GooglePrivacyDlpV2beta1InfoTypeLimit `json:"infoTypeLimits,omitempty"`

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

	// ForceSendFields is a list of field names (e.g. "CustomInfoTypes") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CustomInfoTypes") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1InspectConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1InspectConfig
	raw := NoMethod(*s)
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
	type NoMethod GooglePrivacyDlpV2beta1InspectOperationMetadata
	raw := NoMethod(*s)
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
	type NoMethod GooglePrivacyDlpV2beta1InspectOperationResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1KAnonymityConfig: k-anonymity metric, used for
// analysis of reidentification risk.
type GooglePrivacyDlpV2beta1KAnonymityConfig struct {
	// EntityId: Optional message indicating that each distinct entity_id
	// should not
	// contribute to the k-anonymity count more than once per equivalence
	// class.
	// If an entity_id appears on several rows with different
	// quasi-identifier
	// tuples, it will contribute to each count exactly once.
	//
	// This can lead to unexpected results. Consider a table where ID 1
	// is
	// associated to quasi-identifier "foo", ID 2 to "bar", and ID 3 to
	// *both*
	// quasi-identifiers "foo" and "bar" (on separate rows), and where this
	// ID
	// is used as entity_id. Then, the anonymity value associated to ID 3
	// will
	// be 2, even if it is the only ID to be associated to both values "foo"
	// and
	// "bar".
	EntityId *GooglePrivacyDlpV2beta1EntityId `json:"entityId,omitempty"`

	// QuasiIds: Set of fields to compute k-anonymity over. When multiple
	// fields are
	// specified, they are considered a single composite key. Structs
	// and
	// repeated data types are not supported; however, nested fields
	// are
	// supported so long as they are not structs themselves or nested
	// within
	// a repeated field.
	QuasiIds []*GooglePrivacyDlpV2beta1FieldId `json:"quasiIds,omitempty"`

	// ForceSendFields is a list of field names (e.g. "EntityId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "EntityId") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1KAnonymityConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1KAnonymityConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1KAnonymityEquivalenceClass: The set of
// columns' values that share the same k-anonymity value.
type GooglePrivacyDlpV2beta1KAnonymityEquivalenceClass struct {
	// EquivalenceClassSize: Size of the equivalence class, for example
	// number of rows with the
	// above set of values.
	EquivalenceClassSize int64 `json:"equivalenceClassSize,omitempty,string"`

	// QuasiIdsValues: Set of values defining the equivalence class. One
	// value per
	// quasi-identifier column in the original KAnonymity metric
	// message.
	// The order is always the same as the original request.
	QuasiIdsValues []*GooglePrivacyDlpV2beta1Value `json:"quasiIdsValues,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "EquivalenceClassSize") to unconditionally include in API requests.
	// By default, fields with empty values are omitted from API requests.
	// However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "EquivalenceClassSize") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1KAnonymityEquivalenceClass) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1KAnonymityEquivalenceClass
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1KAnonymityHistogramBucket: Histogram bucket of
// equivalence class sizes in the table.
type GooglePrivacyDlpV2beta1KAnonymityHistogramBucket struct {
	// BucketSize: Total number of records in this bucket.
	BucketSize int64 `json:"bucketSize,omitempty,string"`

	// BucketValues: Sample of equivalence classes in this bucket. The total
	// number of
	// classes returned per bucket is capped at 20.
	BucketValues []*GooglePrivacyDlpV2beta1KAnonymityEquivalenceClass `json:"bucketValues,omitempty"`

	// EquivalenceClassSizeLowerBound: Lower bound on the size of the
	// equivalence classes in this bucket.
	EquivalenceClassSizeLowerBound int64 `json:"equivalenceClassSizeLowerBound,omitempty,string"`

	// EquivalenceClassSizeUpperBound: Upper bound on the size of the
	// equivalence classes in this bucket.
	EquivalenceClassSizeUpperBound int64 `json:"equivalenceClassSizeUpperBound,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "BucketSize") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BucketSize") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1KAnonymityHistogramBucket) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1KAnonymityHistogramBucket
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1KAnonymityResult: Result of the k-anonymity
// computation.
type GooglePrivacyDlpV2beta1KAnonymityResult struct {
	// EquivalenceClassHistogramBuckets: Histogram of k-anonymity
	// equivalence classes.
	EquivalenceClassHistogramBuckets []*GooglePrivacyDlpV2beta1KAnonymityHistogramBucket `json:"equivalenceClassHistogramBuckets,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "EquivalenceClassHistogramBuckets") to unconditionally include in API
	// requests. By default, fields with empty values are omitted from API
	// requests. However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g.
	// "EquivalenceClassHistogramBuckets") to include in API requests with
	// the JSON null value. By default, fields with empty values are omitted
	// from API requests. However, any field with an empty value appearing
	// in NullFields will be sent to the server as null. It is an error if a
	// field in this list has a non-empty value. This may be used to include
	// null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1KAnonymityResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1KAnonymityResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1KMapEstimationConfig: Reidentifiability
// metric. This corresponds to a risk model similar to what
// is called "journalist risk" in the literature, except the attack
// dataset is
// statistically modeled instead of being perfectly known. This can be
// done
// using publicly available data (like the US Census), or using a
// custom
// statistical model (indicated as one or several BigQuery tables), or
// by
// extrapolating from the distribution of values in the input dataset.
type GooglePrivacyDlpV2beta1KMapEstimationConfig struct {
	// AuxiliaryTables: Several auxiliary tables can be used in the
	// analysis. Each custom_tag
	// used to tag a quasi-identifiers column must appear in exactly one
	// column
	// of one auxiliary table.
	AuxiliaryTables []*GooglePrivacyDlpV2beta1AuxiliaryTable `json:"auxiliaryTables,omitempty"`

	// QuasiIds: Fields considered to be quasi-identifiers. No two columns
	// can have the
	// same tag. [required]
	QuasiIds []*GooglePrivacyDlpV2beta1TaggedField `json:"quasiIds,omitempty"`

	// RegionCode: ISO 3166-1 alpha-2 region code to use in the statistical
	// modeling.
	// Required if no column is tagged with a region-specific InfoType
	// (like
	// US_ZIP_5) or a region code.
	RegionCode string `json:"regionCode,omitempty"`

	// ForceSendFields is a list of field names (e.g. "AuxiliaryTables") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "AuxiliaryTables") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1KMapEstimationConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1KMapEstimationConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1KMapEstimationHistogramBucket: A
// KMapEstimationHistogramBucket message with the following values:
//   min_anonymity: 3
//   max_anonymity: 5
//   frequency: 42
// means that there are 42 records whose quasi-identifier values
// correspond
// to 3, 4 or 5 people in the overlying population. An important
// particular
// case is when min_anonymity = max_anonymity = 1: the frequency field
// then
// corresponds to the number of uniquely identifiable records.
type GooglePrivacyDlpV2beta1KMapEstimationHistogramBucket struct {
	// BucketSize: Number of records within these anonymity bounds.
	BucketSize int64 `json:"bucketSize,omitempty,string"`

	// BucketValues: Sample of quasi-identifier tuple values in this bucket.
	// The total
	// number of classes returned per bucket is capped at 20.
	BucketValues []*GooglePrivacyDlpV2beta1KMapEstimationQuasiIdValues `json:"bucketValues,omitempty"`

	// MaxAnonymity: Always greater than or equal to min_anonymity.
	MaxAnonymity int64 `json:"maxAnonymity,omitempty,string"`

	// MinAnonymity: Always positive.
	MinAnonymity int64 `json:"minAnonymity,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "BucketSize") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BucketSize") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1KMapEstimationHistogramBucket) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1KMapEstimationHistogramBucket
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1KMapEstimationQuasiIdValues: A tuple of values
// for the quasi-identifier columns.
type GooglePrivacyDlpV2beta1KMapEstimationQuasiIdValues struct {
	// EstimatedAnonymity: The estimated anonymity for these
	// quasi-identifier values.
	EstimatedAnonymity int64 `json:"estimatedAnonymity,omitempty,string"`

	// QuasiIdsValues: The quasi-identifier values.
	QuasiIdsValues []*GooglePrivacyDlpV2beta1Value `json:"quasiIdsValues,omitempty"`

	// ForceSendFields is a list of field names (e.g. "EstimatedAnonymity")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "EstimatedAnonymity") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1KMapEstimationQuasiIdValues) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1KMapEstimationQuasiIdValues
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1KMapEstimationResult: Result of the
// reidentifiability analysis. Note that these results are
// an
// estimation, not exact values.
type GooglePrivacyDlpV2beta1KMapEstimationResult struct {
	// KMapEstimationHistogram: The intervals [min_anonymity, max_anonymity]
	// do not overlap. If a value
	// doesn't correspond to any such interval, the associated frequency
	// is
	// zero. For example, the following records:
	//   {min_anonymity: 1, max_anonymity: 1, frequency: 17}
	//   {min_anonymity: 2, max_anonymity: 3, frequency: 42}
	//   {min_anonymity: 5, max_anonymity: 10, frequency: 99}
	// mean that there are no record with an estimated anonymity of 4, 5,
	// or
	// larger than 10.
	KMapEstimationHistogram []*GooglePrivacyDlpV2beta1KMapEstimationHistogramBucket `json:"kMapEstimationHistogram,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "KMapEstimationHistogram") to unconditionally include in API
	// requests. By default, fields with empty values are omitted from API
	// requests. However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "KMapEstimationHistogram")
	// to include in API requests with the JSON null value. By default,
	// fields with empty values are omitted from API requests. However, any
	// field with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1KMapEstimationResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1KMapEstimationResult
	raw := NoMethod(*s)
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
	type NoMethod GooglePrivacyDlpV2beta1KindExpression
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1LDiversityConfig: l-diversity metric, used for
// analysis of reidentification risk.
type GooglePrivacyDlpV2beta1LDiversityConfig struct {
	// QuasiIds: Set of quasi-identifiers indicating how equivalence classes
	// are
	// defined for the l-diversity computation. When multiple fields
	// are
	// specified, they are considered a single composite key.
	QuasiIds []*GooglePrivacyDlpV2beta1FieldId `json:"quasiIds,omitempty"`

	// SensitiveAttribute: Sensitive field for computing the l-value.
	SensitiveAttribute *GooglePrivacyDlpV2beta1FieldId `json:"sensitiveAttribute,omitempty"`

	// ForceSendFields is a list of field names (e.g. "QuasiIds") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "QuasiIds") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1LDiversityConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1LDiversityConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1LDiversityEquivalenceClass: The set of
// columns' values that share the same l-diversity value.
type GooglePrivacyDlpV2beta1LDiversityEquivalenceClass struct {
	// EquivalenceClassSize: Size of the k-anonymity equivalence class.
	EquivalenceClassSize int64 `json:"equivalenceClassSize,omitempty,string"`

	// NumDistinctSensitiveValues: Number of distinct sensitive values in
	// this equivalence class.
	NumDistinctSensitiveValues int64 `json:"numDistinctSensitiveValues,omitempty,string"`

	// QuasiIdsValues: Quasi-identifier values defining the k-anonymity
	// equivalence
	// class. The order is always the same as the original request.
	QuasiIdsValues []*GooglePrivacyDlpV2beta1Value `json:"quasiIdsValues,omitempty"`

	// TopSensitiveValues: Estimated frequencies of top sensitive values.
	TopSensitiveValues []*GooglePrivacyDlpV2beta1ValueFrequency `json:"topSensitiveValues,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "EquivalenceClassSize") to unconditionally include in API requests.
	// By default, fields with empty values are omitted from API requests.
	// However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "EquivalenceClassSize") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1LDiversityEquivalenceClass) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1LDiversityEquivalenceClass
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1LDiversityHistogramBucket: Histogram bucket of
// sensitive value frequencies in the table.
type GooglePrivacyDlpV2beta1LDiversityHistogramBucket struct {
	// BucketSize: Total number of records in this bucket.
	BucketSize int64 `json:"bucketSize,omitempty,string"`

	// BucketValues: Sample of equivalence classes in this bucket. The total
	// number of
	// classes returned per bucket is capped at 20.
	BucketValues []*GooglePrivacyDlpV2beta1LDiversityEquivalenceClass `json:"bucketValues,omitempty"`

	// SensitiveValueFrequencyLowerBound: Lower bound on the sensitive value
	// frequencies of the equivalence
	// classes in this bucket.
	SensitiveValueFrequencyLowerBound int64 `json:"sensitiveValueFrequencyLowerBound,omitempty,string"`

	// SensitiveValueFrequencyUpperBound: Upper bound on the sensitive value
	// frequencies of the equivalence
	// classes in this bucket.
	SensitiveValueFrequencyUpperBound int64 `json:"sensitiveValueFrequencyUpperBound,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "BucketSize") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BucketSize") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1LDiversityHistogramBucket) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1LDiversityHistogramBucket
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1LDiversityResult: Result of the l-diversity
// computation.
type GooglePrivacyDlpV2beta1LDiversityResult struct {
	// SensitiveValueFrequencyHistogramBuckets: Histogram of l-diversity
	// equivalence class sensitive value frequencies.
	SensitiveValueFrequencyHistogramBuckets []*GooglePrivacyDlpV2beta1LDiversityHistogramBucket `json:"sensitiveValueFrequencyHistogramBuckets,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "SensitiveValueFrequencyHistogramBuckets") to unconditionally include
	// in API requests. By default, fields with empty values are omitted
	// from API requests. However, any non-pointer, non-interface field
	// appearing in ForceSendFields will be sent to the server regardless of
	// whether the field is empty or not. This may be used to include empty
	// fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g.
	// "SensitiveValueFrequencyHistogramBuckets") to include in API requests
	// with the JSON null value. By default, fields with empty values are
	// omitted from API requests. However, any field with an empty value
	// appearing in NullFields will be sent to the server as null. It is an
	// error if a field in this list has a non-empty value. This may be used
	// to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1LDiversityResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1LDiversityResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1NumericalStatsConfig: Compute numerical stats
// over an individual column, including
// min, max, and quantiles.
type GooglePrivacyDlpV2beta1NumericalStatsConfig struct {
	// Field: Field to compute numerical stats on. Supported types
	// are
	// integer, float, date, datetime, timestamp, time.
	Field *GooglePrivacyDlpV2beta1FieldId `json:"field,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Field") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Field") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1NumericalStatsConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1NumericalStatsConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1NumericalStatsResult: Result of the numerical
// stats computation.
type GooglePrivacyDlpV2beta1NumericalStatsResult struct {
	// MaxValue: Maximum value appearing in the column.
	MaxValue *GooglePrivacyDlpV2beta1Value `json:"maxValue,omitempty"`

	// MinValue: Minimum value appearing in the column.
	MinValue *GooglePrivacyDlpV2beta1Value `json:"minValue,omitempty"`

	// QuantileValues: List of 99 values that partition the set of field
	// values into 100 equal
	// sized buckets.
	QuantileValues []*GooglePrivacyDlpV2beta1Value `json:"quantileValues,omitempty"`

	// ForceSendFields is a list of field names (e.g. "MaxValue") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "MaxValue") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1NumericalStatsResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1NumericalStatsResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1OutputStorageConfig: Cloud repository for
// storing output.
type GooglePrivacyDlpV2beta1OutputStorageConfig struct {
	// StoragePath: The path to a Google Cloud Storage location to store
	// output.
	// The bucket must already exist and
	// the Google APIs service account for DLP must have write permission
	// to
	// write to the given bucket.
	// Results are split over multiple csv files with each file name
	// matching
	// the pattern "[operation_id]_[count].csv", for
	// example
	// `3094877188788974909_1.csv`. The `operation_id` matches
	// the
	// identifier for the Operation, and the `count` is a counter used
	// for
	// tracking the number of files written.
	//
	// The CSV file(s) contain the following columns regardless of storage
	// type
	// scanned:
	// - id
	// - info_type
	// - likelihood
	// - byte size of finding
	// - quote
	// - timestamp
	//
	// For Cloud Storage the next columns are:
	//
	// - file_path
	// - start_offset
	//
	// For Cloud Datastore the next columns are:
	//
	// - project_id
	// - namespace_id
	// - path
	// - column_name
	// - offset
	//
	// For BigQuery the next columns are:
	//
	// - row_number
	// - project_id
	// - dataset_id
	// - table_id
	StoragePath *GooglePrivacyDlpV2beta1CloudStoragePath `json:"storagePath,omitempty"`

	// Table: Store findings in a new table in the dataset.
	Table *GooglePrivacyDlpV2beta1BigQueryTable `json:"table,omitempty"`

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
	type NoMethod GooglePrivacyDlpV2beta1OutputStorageConfig
	raw := NoMethod(*s)
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
	type NoMethod GooglePrivacyDlpV2beta1PartitionId
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1PrivacyMetric: Privacy metric to compute for
// reidentification risk analysis.
type GooglePrivacyDlpV2beta1PrivacyMetric struct {
	CategoricalStatsConfig *GooglePrivacyDlpV2beta1CategoricalStatsConfig `json:"categoricalStatsConfig,omitempty"`

	KAnonymityConfig *GooglePrivacyDlpV2beta1KAnonymityConfig `json:"kAnonymityConfig,omitempty"`

	KMapEstimationConfig *GooglePrivacyDlpV2beta1KMapEstimationConfig `json:"kMapEstimationConfig,omitempty"`

	LDiversityConfig *GooglePrivacyDlpV2beta1LDiversityConfig `json:"lDiversityConfig,omitempty"`

	NumericalStatsConfig *GooglePrivacyDlpV2beta1NumericalStatsConfig `json:"numericalStatsConfig,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "CategoricalStatsConfig") to unconditionally include in API requests.
	// By default, fields with empty values are omitted from API requests.
	// However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CategoricalStatsConfig")
	// to include in API requests with the JSON null value. By default,
	// fields with empty values are omitted from API requests. However, any
	// field with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1PrivacyMetric) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1PrivacyMetric
	raw := NoMethod(*s)
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
	type NoMethod GooglePrivacyDlpV2beta1Projection
	raw := NoMethod(*s)
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
	type NoMethod GooglePrivacyDlpV2beta1PropertyReference
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1QuasiIdField: A quasi-identifier column has a
// custom_tag, used to know which column
// in the data corresponds to which column in the statistical model.
type GooglePrivacyDlpV2beta1QuasiIdField struct {
	CustomTag string `json:"customTag,omitempty"`

	Field *GooglePrivacyDlpV2beta1FieldId `json:"field,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CustomTag") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CustomTag") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1QuasiIdField) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1QuasiIdField
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1RiskAnalysisOperationMetadata: Metadata
// returned within
// the
// [`riskAnalysis.operations.get`](/dlp/docs/reference/rest/v2beta1/r
// iskAnalysis.operations/get)
// for risk analysis.
type GooglePrivacyDlpV2beta1RiskAnalysisOperationMetadata struct {
	// CreateTime: The time which this request was started.
	CreateTime string `json:"createTime,omitempty"`

	// RequestedPrivacyMetric: Privacy metric to compute.
	RequestedPrivacyMetric *GooglePrivacyDlpV2beta1PrivacyMetric `json:"requestedPrivacyMetric,omitempty"`

	// RequestedSourceTable: Input dataset to compute metrics over.
	RequestedSourceTable *GooglePrivacyDlpV2beta1BigQueryTable `json:"requestedSourceTable,omitempty"`

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

func (s *GooglePrivacyDlpV2beta1RiskAnalysisOperationMetadata) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1RiskAnalysisOperationMetadata
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1RiskAnalysisOperationResult: Result of a risk
// analysis
// [`Operation`](/dlp/docs/reference/rest/v2beta1/inspect.operat
// ions)
// request.
type GooglePrivacyDlpV2beta1RiskAnalysisOperationResult struct {
	CategoricalStatsResult *GooglePrivacyDlpV2beta1CategoricalStatsResult `json:"categoricalStatsResult,omitempty"`

	KAnonymityResult *GooglePrivacyDlpV2beta1KAnonymityResult `json:"kAnonymityResult,omitempty"`

	KMapEstimationResult *GooglePrivacyDlpV2beta1KMapEstimationResult `json:"kMapEstimationResult,omitempty"`

	LDiversityResult *GooglePrivacyDlpV2beta1LDiversityResult `json:"lDiversityResult,omitempty"`

	NumericalStatsResult *GooglePrivacyDlpV2beta1NumericalStatsResult `json:"numericalStatsResult,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "CategoricalStatsResult") to unconditionally include in API requests.
	// By default, fields with empty values are omitted from API requests.
	// However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CategoricalStatsResult")
	// to include in API requests with the JSON null value. By default,
	// fields with empty values are omitted from API requests. However, any
	// field with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1RiskAnalysisOperationResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1RiskAnalysisOperationResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1StorageConfig: Shared message indicating Cloud
// storage type.
type GooglePrivacyDlpV2beta1StorageConfig struct {
	// BigQueryOptions: BigQuery options specification.
	BigQueryOptions *GooglePrivacyDlpV2beta1BigQueryOptions `json:"bigQueryOptions,omitempty"`

	// CloudStorageOptions: Google Cloud Storage options specification.
	CloudStorageOptions *GooglePrivacyDlpV2beta1CloudStorageOptions `json:"cloudStorageOptions,omitempty"`

	// DatastoreOptions: Google Cloud Datastore options specification.
	DatastoreOptions *GooglePrivacyDlpV2beta1DatastoreOptions `json:"datastoreOptions,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BigQueryOptions") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BigQueryOptions") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1StorageConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1StorageConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1SurrogateType: Message for detecting output
// from deidentification transformations
// such
// as
// [`CryptoReplaceFfxFpeConfig`](/dlp/docs/reference/rest/v2beta1/cont
// ent/deidentify#CryptoReplaceFfxFpeConfig).
// These types of transformations are
// those that perform pseudonymization, thereby producing a "surrogate"
// as
// output. This should be used in conjunction with a field on
// the
// transformation such as `surrogate_info_type`. This custom info type
// does
// not support the use of `detection_rules`.
type GooglePrivacyDlpV2beta1SurrogateType struct {
}

// GooglePrivacyDlpV2beta1TaggedField: A column with a semantic tag
// attached.
type GooglePrivacyDlpV2beta1TaggedField struct {
	// CustomTag: A column can be tagged with a custom tag. In this case,
	// the user must
	// indicate an auxiliary table that contains statistical information
	// on
	// the possible values of this column (below).
	CustomTag string `json:"customTag,omitempty"`

	// Field: Identifies the column. [required]
	Field *GooglePrivacyDlpV2beta1FieldId `json:"field,omitempty"`

	// Inferred: If no semantic tag is indicated, we infer the statistical
	// model from
	// the distribution of values in the input data
	Inferred *GoogleProtobufEmpty `json:"inferred,omitempty"`

	// InfoType: A column can be tagged with a InfoType to use the relevant
	// public
	// dataset as a statistical model of population, if available.
	// We
	// currently support US ZIP codes, region codes, ages and genders.
	InfoType *GooglePrivacyDlpV2beta1InfoType `json:"infoType,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CustomTag") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CustomTag") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1TaggedField) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1TaggedField
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1Value: Set of primitive values supported by
// the system.
// Note that for the purposes of inspection or transformation, the
// number
// of bytes considered to comprise a 'Value' is based on its
// representation
// as a UTF-8 encoded string. For example, if 'integer_value' is set
// to
// 123456789, the number of bytes would be counted as 9, even though
// an
// int64 only holds up to 8 bytes of data.
type GooglePrivacyDlpV2beta1Value struct {
	BooleanValue bool `json:"booleanValue,omitempty"`

	DateValue *GoogleTypeDate `json:"dateValue,omitempty"`

	FloatValue float64 `json:"floatValue,omitempty"`

	IntegerValue int64 `json:"integerValue,omitempty,string"`

	StringValue string `json:"stringValue,omitempty"`

	TimeValue *GoogleTypeTimeOfDay `json:"timeValue,omitempty"`

	TimestampValue string `json:"timestampValue,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BooleanValue") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BooleanValue") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1Value) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1Value
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GooglePrivacyDlpV2beta1Value) UnmarshalJSON(data []byte) error {
	type NoMethod GooglePrivacyDlpV2beta1Value
	var s1 struct {
		FloatValue gensupport.JSONFloat64 `json:"floatValue"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.FloatValue = float64(s1.FloatValue)
	return nil
}

// GooglePrivacyDlpV2beta1ValueFrequency: A value of a field, including
// its frequency.
type GooglePrivacyDlpV2beta1ValueFrequency struct {
	// Count: How many times the value is contained in the field.
	Count int64 `json:"count,omitempty,string"`

	// Value: A value contained in the field in question.
	Value *GooglePrivacyDlpV2beta1Value `json:"value,omitempty"`

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

func (s *GooglePrivacyDlpV2beta1ValueFrequency) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1ValueFrequency
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta1WordList: Message defining a list of words or
// phrases to search for in the data.
type GooglePrivacyDlpV2beta1WordList struct {
	// Words: Words or phrases defining the dictionary. The dictionary must
	// contain
	// at least one phrase and every phrase must contain at least 2
	// characters
	// that are letters or digits. [required]
	Words []string `json:"words,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Words") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Words") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta1WordList) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta1WordList
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Action: A task to execute on the completion of
// a job.
type GooglePrivacyDlpV2beta2Action struct {
	// PubSub: Publish a notification to a pubsub topic.
	PubSub *GooglePrivacyDlpV2beta2PublishToPubSub `json:"pubSub,omitempty"`

	// SaveFindings: Save resulting findings in a provided location.
	SaveFindings *GooglePrivacyDlpV2beta2SaveFindings `json:"saveFindings,omitempty"`

	// ForceSendFields is a list of field names (e.g. "PubSub") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "PubSub") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Action) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Action
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2AnalyzeDataSourceRiskDetails: Result of a risk
// analysis operation request.
type GooglePrivacyDlpV2beta2AnalyzeDataSourceRiskDetails struct {
	CategoricalStatsResult *GooglePrivacyDlpV2beta2CategoricalStatsResult `json:"categoricalStatsResult,omitempty"`

	KAnonymityResult *GooglePrivacyDlpV2beta2KAnonymityResult `json:"kAnonymityResult,omitempty"`

	KMapEstimationResult *GooglePrivacyDlpV2beta2KMapEstimationResult `json:"kMapEstimationResult,omitempty"`

	LDiversityResult *GooglePrivacyDlpV2beta2LDiversityResult `json:"lDiversityResult,omitempty"`

	NumericalStatsResult *GooglePrivacyDlpV2beta2NumericalStatsResult `json:"numericalStatsResult,omitempty"`

	// RequestedPrivacyMetric: Privacy metric to compute.
	RequestedPrivacyMetric *GooglePrivacyDlpV2beta2PrivacyMetric `json:"requestedPrivacyMetric,omitempty"`

	// RequestedSourceTable: Input dataset to compute metrics over.
	RequestedSourceTable *GooglePrivacyDlpV2beta2BigQueryTable `json:"requestedSourceTable,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "CategoricalStatsResult") to unconditionally include in API requests.
	// By default, fields with empty values are omitted from API requests.
	// However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CategoricalStatsResult")
	// to include in API requests with the JSON null value. By default,
	// fields with empty values are omitted from API requests. However, any
	// field with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2AnalyzeDataSourceRiskDetails) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2AnalyzeDataSourceRiskDetails
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2AnalyzeDataSourceRiskRequest: Request for
// creating a risk analysis DlpJob.
type GooglePrivacyDlpV2beta2AnalyzeDataSourceRiskRequest struct {
	// JobConfig: Configuration for this risk analysis job.
	JobConfig *GooglePrivacyDlpV2beta2RiskAnalysisJobConfig `json:"jobConfig,omitempty"`

	// JobId: Optional job ID to use for the created job. If not provided, a
	// job ID will
	// automatically be generated. Must be unique within the project. The
	// job ID
	// can contain uppercase and lowercase letters, numbers, and hyphens;
	// that is,
	// it must match the regular expression: `[a-zA-Z\\d-]+`. The maximum
	// length
	// is 100 characters. Can be empty to allow the system to generate one.
	JobId string `json:"jobId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "JobConfig") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "JobConfig") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2AnalyzeDataSourceRiskRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2AnalyzeDataSourceRiskRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2AuxiliaryTable: An auxiliary table contains
// statistical information on the relative
// frequency of different quasi-identifiers values. It has one or
// several
// quasi-identifiers columns, and one column that indicates the
// relative
// frequency of each quasi-identifier tuple.
// If a tuple is present in the data but not in the auxiliary table,
// the
// corresponding relative frequency is assumed to be zero (and thus,
// the
// tuple is highly reidentifiable).
type GooglePrivacyDlpV2beta2AuxiliaryTable struct {
	// QuasiIds: Quasi-identifier columns. [required]
	QuasiIds []*GooglePrivacyDlpV2beta2QuasiIdField `json:"quasiIds,omitempty"`

	// RelativeFrequency: The relative frequency column must contain a
	// floating-point number
	// between 0 and 1 (inclusive). Null values are assumed to be
	// zero.
	// [required]
	RelativeFrequency *GooglePrivacyDlpV2beta2FieldId `json:"relativeFrequency,omitempty"`

	// Table: Auxiliary table location. [required]
	Table *GooglePrivacyDlpV2beta2BigQueryTable `json:"table,omitempty"`

	// ForceSendFields is a list of field names (e.g. "QuasiIds") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "QuasiIds") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2AuxiliaryTable) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2AuxiliaryTable
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2BigQueryKey: Row key for identifying a record
// in BigQuery table.
type GooglePrivacyDlpV2beta2BigQueryKey struct {
	// RowNumber: Absolute number of the row from the beginning of the table
	// at the time
	// of scanning.
	RowNumber int64 `json:"rowNumber,omitempty,string"`

	// TableReference: Complete BigQuery table reference.
	TableReference *GooglePrivacyDlpV2beta2BigQueryTable `json:"tableReference,omitempty"`

	// ForceSendFields is a list of field names (e.g. "RowNumber") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "RowNumber") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2BigQueryKey) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2BigQueryKey
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2BigQueryOptions: Options defining BigQuery
// table and row identifiers.
type GooglePrivacyDlpV2beta2BigQueryOptions struct {
	// IdentifyingFields: References to fields uniquely identifying rows
	// within the table.
	// Nested fields in the format, like `person.birthdate.year`, are
	// allowed.
	IdentifyingFields []*GooglePrivacyDlpV2beta2FieldId `json:"identifyingFields,omitempty"`

	// TableReference: Complete BigQuery table reference.
	TableReference *GooglePrivacyDlpV2beta2BigQueryTable `json:"tableReference,omitempty"`

	// ForceSendFields is a list of field names (e.g. "IdentifyingFields")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "IdentifyingFields") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2BigQueryOptions) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2BigQueryOptions
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2BigQueryTable: Message defining the location
// of a BigQuery table. A table is uniquely
// identified  by its project_id, dataset_id, and table_name. Within a
// query
// a table is often referenced with a string in the format
// of:
// `<project_id>:<dataset_id>.<table_id>`
// or
// `<project_id>.<dataset_id>.<table_id>`.
type GooglePrivacyDlpV2beta2BigQueryTable struct {
	// DatasetId: Dataset ID of the table.
	DatasetId string `json:"datasetId,omitempty"`

	// ProjectId: The Google Cloud Platform project ID of the project
	// containing the table.
	// If omitted, project ID is inferred from the API call.
	ProjectId string `json:"projectId,omitempty"`

	// TableId: Name of the table.
	TableId string `json:"tableId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DatasetId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DatasetId") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2BigQueryTable) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2BigQueryTable
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Bucket: Bucket is represented as a range,
// along with replacement values.
type GooglePrivacyDlpV2beta2Bucket struct {
	// Max: Upper bound of the range, exclusive; type must match min.
	Max *GooglePrivacyDlpV2beta2Value `json:"max,omitempty"`

	// Min: Lower bound of the range, inclusive. Type should be the same as
	// max if
	// used.
	Min *GooglePrivacyDlpV2beta2Value `json:"min,omitempty"`

	// ReplacementValue: Replacement value for this bucket. If not
	// provided
	// the default behavior will be to hyphenate the min-max range.
	ReplacementValue *GooglePrivacyDlpV2beta2Value `json:"replacementValue,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Max") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Max") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Bucket) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Bucket
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2BucketingConfig: Generalization function that
// buckets values based on ranges. The ranges and
// replacement values are dynamically provided by the user for custom
// behavior,
// such as 1-30 -> LOW 31-65 -> MEDIUM 66-100 -> HIGH
// This can be used on
// data of type: number, long, string, timestamp.
// If the bound `Value` type differs from the type of data being
// transformed, we
// will first attempt converting the type of the data to be transformed
// to match
// the type of the bound before comparing.
type GooglePrivacyDlpV2beta2BucketingConfig struct {
	// Buckets: Set of buckets. Ranges must be non-overlapping.
	Buckets []*GooglePrivacyDlpV2beta2Bucket `json:"buckets,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Buckets") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Buckets") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2BucketingConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2BucketingConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CancelDlpJobRequest: The request message for
// canceling a DLP job.
type GooglePrivacyDlpV2beta2CancelDlpJobRequest struct {
}

// GooglePrivacyDlpV2beta2CategoricalStatsConfig: Compute numerical
// stats over an individual column, including
// number of distinct values and value count distribution.
type GooglePrivacyDlpV2beta2CategoricalStatsConfig struct {
	// Field: Field to compute categorical stats on. All column types
	// are
	// supported except for arrays and structs. However, it may be
	// more
	// informative to use NumericalStats when the field type is
	// supported,
	// depending on the data.
	Field *GooglePrivacyDlpV2beta2FieldId `json:"field,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Field") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Field") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CategoricalStatsConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CategoricalStatsConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type GooglePrivacyDlpV2beta2CategoricalStatsHistogramBucket struct {
	// BucketSize: Total number of values in this bucket.
	BucketSize int64 `json:"bucketSize,omitempty,string"`

	// BucketValueCount: Total number of distinct values in this bucket.
	BucketValueCount int64 `json:"bucketValueCount,omitempty,string"`

	// BucketValues: Sample of value frequencies in this bucket. The total
	// number of
	// values returned per bucket is capped at 20.
	BucketValues []*GooglePrivacyDlpV2beta2ValueFrequency `json:"bucketValues,omitempty"`

	// ValueFrequencyLowerBound: Lower bound on the value frequency of the
	// values in this bucket.
	ValueFrequencyLowerBound int64 `json:"valueFrequencyLowerBound,omitempty,string"`

	// ValueFrequencyUpperBound: Upper bound on the value frequency of the
	// values in this bucket.
	ValueFrequencyUpperBound int64 `json:"valueFrequencyUpperBound,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "BucketSize") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BucketSize") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CategoricalStatsHistogramBucket) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CategoricalStatsHistogramBucket
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CategoricalStatsResult: Result of the
// categorical stats computation.
type GooglePrivacyDlpV2beta2CategoricalStatsResult struct {
	// ValueFrequencyHistogramBuckets: Histogram of value frequencies in the
	// column.
	ValueFrequencyHistogramBuckets []*GooglePrivacyDlpV2beta2CategoricalStatsHistogramBucket `json:"valueFrequencyHistogramBuckets,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "ValueFrequencyHistogramBuckets") to unconditionally include in API
	// requests. By default, fields with empty values are omitted from API
	// requests. However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g.
	// "ValueFrequencyHistogramBuckets") to include in API requests with the
	// JSON null value. By default, fields with empty values are omitted
	// from API requests. However, any field with an empty value appearing
	// in NullFields will be sent to the server as null. It is an error if a
	// field in this list has a non-empty value. This may be used to include
	// null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CategoricalStatsResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CategoricalStatsResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CharacterMaskConfig: Partially mask a string
// by replacing a given number of characters with a
// fixed character. Masking can start from the beginning or end of the
// string.
// This can be used on data of any type (numbers, longs, and so on) and
// when
// de-identifying structured data we'll attempt to preserve the original
// data's
// type. (This allows you to take a long like 123 and modify it to a
// string like
// **3.
type GooglePrivacyDlpV2beta2CharacterMaskConfig struct {
	// CharactersToIgnore: When masking a string, items in this list will be
	// skipped when replacing.
	// For example, if your string is 555-555-5555 and you ask us to skip
	// `-` and
	// mask 5 chars with * we would produce ***-*55-5555.
	CharactersToIgnore []*GooglePrivacyDlpV2beta2CharsToIgnore `json:"charactersToIgnore,omitempty"`

	// MaskingCharacter: Character to mask the sensitive values&mdash;for
	// example, "*" for an
	// alphabetic string such as name, or "0" for a numeric string such as
	// ZIP
	// code or credit card number. String must have length 1. If not
	// supplied, we
	// will default to "*" for strings, 0 for digits.
	MaskingCharacter string `json:"maskingCharacter,omitempty"`

	// NumberToMask: Number of characters to mask. If not set, all matching
	// chars will be
	// masked. Skipped characters do not count towards this tally.
	NumberToMask int64 `json:"numberToMask,omitempty"`

	// ReverseOrder: Mask characters in reverse order. For example, if
	// `masking_character` is
	// '0', number_to_mask is 14, and `reverse_order` is false,
	// then
	// 1234-5678-9012-3456 -> 00000000000000-3456
	// If `masking_character` is '*', `number_to_mask` is 3, and
	// `reverse_order`
	// is true, then 12345 -> 12***
	ReverseOrder bool `json:"reverseOrder,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CharactersToIgnore")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CharactersToIgnore") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CharacterMaskConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CharacterMaskConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CharsToIgnore: Characters to skip when doing
// deidentification of a value. These will be left
// alone and skipped.
type GooglePrivacyDlpV2beta2CharsToIgnore struct {
	CharactersToSkip string `json:"charactersToSkip,omitempty"`

	// Possible values:
	//   "COMMON_CHARS_TO_IGNORE_UNSPECIFIED"
	//   "NUMERIC" - 0-9
	//   "ALPHA_UPPER_CASE" - A-Z
	//   "ALPHA_LOWER_CASE" - a-z
	//   "PUNCTUATION" - US Punctuation, one of
	// !"#$%&'()*+,-./:;<=>?@[\]^_`{|}~
	//   "WHITESPACE" - Whitespace character, one of [ \t\n\x0B\f\r]
	CommonCharactersToIgnore string `json:"commonCharactersToIgnore,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CharactersToSkip") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CharactersToSkip") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CharsToIgnore) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CharsToIgnore
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CloudStorageKey: Record key for a finding in a
// Cloud Storage file.
type GooglePrivacyDlpV2beta2CloudStorageKey struct {
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

func (s *GooglePrivacyDlpV2beta2CloudStorageKey) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CloudStorageKey
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CloudStorageOptions: Options defining a file
// or a set of files (path ending with *) within
// a Google Cloud Storage bucket.
type GooglePrivacyDlpV2beta2CloudStorageOptions struct {
	// BytesLimitPerFile: Max number of bytes to scan from a file. If a
	// scanned file's size is bigger
	// than this value then the rest of the bytes are omitted.
	BytesLimitPerFile int64 `json:"bytesLimitPerFile,omitempty,string"`

	FileSet *GooglePrivacyDlpV2beta2FileSet `json:"fileSet,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BytesLimitPerFile")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BytesLimitPerFile") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CloudStorageOptions) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CloudStorageOptions
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Color: Represents a color in the RGB color
// space.
type GooglePrivacyDlpV2beta2Color struct {
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

func (s *GooglePrivacyDlpV2beta2Color) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Color
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GooglePrivacyDlpV2beta2Color) UnmarshalJSON(data []byte) error {
	type NoMethod GooglePrivacyDlpV2beta2Color
	var s1 struct {
		Blue  gensupport.JSONFloat64 `json:"blue"`
		Green gensupport.JSONFloat64 `json:"green"`
		Red   gensupport.JSONFloat64 `json:"red"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.Blue = float64(s1.Blue)
	s.Green = float64(s1.Green)
	s.Red = float64(s1.Red)
	return nil
}

// GooglePrivacyDlpV2beta2Condition: The field type of `value` and
// `field` do not need to match to be
// considered equal, but not all comparisons are possible.
//
// A `value` of type:
//
// - `string` can be compared against all other types
// - `boolean` can only be compared against other booleans
// - `integer` can be compared against doubles or a string if the string
// value
// can be parsed as an integer.
// - `double` can be compared against integers or a string if the string
// can
// be parsed as a double.
// - `Timestamp` can be compared against strings in RFC 3339 date
// string
// format.
// - `TimeOfDay` can be compared against timestamps and strings in the
// format
// of 'HH:mm:ss'.
//
// If we fail to compare do to type mismatch, a warning will be given
// and
// the condition will evaluate to false.
type GooglePrivacyDlpV2beta2Condition struct {
	// Field: Field within the record this condition is evaluated against.
	// [required]
	Field *GooglePrivacyDlpV2beta2FieldId `json:"field,omitempty"`

	// Operator: Operator used to compare the field or infoType to the
	// value. [required]
	//
	// Possible values:
	//   "RELATIONAL_OPERATOR_UNSPECIFIED"
	//   "EQUAL_TO" - Equal.
	//   "NOT_EQUAL_TO" - Not equal to.
	//   "GREATER_THAN" - Greater than.
	//   "LESS_THAN" - Less than.
	//   "GREATER_THAN_OR_EQUALS" - Greater than or equals.
	//   "LESS_THAN_OR_EQUALS" - Less than or equals.
	//   "EXISTS" - Exists
	Operator string `json:"operator,omitempty"`

	// Value: Value to compare against. [Required, except for `EXISTS`
	// tests.]
	Value *GooglePrivacyDlpV2beta2Value `json:"value,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Field") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Field") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Condition) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Condition
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Conditions: A collection of conditions.
type GooglePrivacyDlpV2beta2Conditions struct {
	Conditions []*GooglePrivacyDlpV2beta2Condition `json:"conditions,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Conditions") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Conditions") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Conditions) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Conditions
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2ContentItem: Container structure for the
// content to inspect.
type GooglePrivacyDlpV2beta2ContentItem struct {
	// Data: Content data to inspect or redact.
	Data string `json:"data,omitempty"`

	// Table: Structured content for inspection.
	Table *GooglePrivacyDlpV2beta2Table `json:"table,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2ContentItem) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2ContentItem
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CreateDeidentifyTemplateRequest: Request
// message for CreateDeidentifyTemplate.
type GooglePrivacyDlpV2beta2CreateDeidentifyTemplateRequest struct {
	// DeidentifyTemplate: The DeidentifyTemplate to create.
	DeidentifyTemplate *GooglePrivacyDlpV2beta2DeidentifyTemplate `json:"deidentifyTemplate,omitempty"`

	// TemplateId: The template id can contain uppercase and lowercase
	// letters,
	// numbers, and hyphens; that is, it must match the regular
	// expression: `[a-zA-Z\\d-]+`. The maximum length is 100
	// characters. Can be empty to allow the system to generate one.
	TemplateId string `json:"templateId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DeidentifyTemplate")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DeidentifyTemplate") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CreateDeidentifyTemplateRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CreateDeidentifyTemplateRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CreateInspectTemplateRequest: Request message
// for CreateInspectTemplate.
type GooglePrivacyDlpV2beta2CreateInspectTemplateRequest struct {
	// InspectTemplate: The InspectTemplate to create.
	InspectTemplate *GooglePrivacyDlpV2beta2InspectTemplate `json:"inspectTemplate,omitempty"`

	// TemplateId: The template id can contain uppercase and lowercase
	// letters,
	// numbers, and hyphens; that is, it must match the regular
	// expression: `[a-zA-Z\\d-]+`. The maximum length is 100
	// characters. Can be empty to allow the system to generate one.
	TemplateId string `json:"templateId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "InspectTemplate") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "InspectTemplate") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CreateInspectTemplateRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CreateInspectTemplateRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CreateJobTriggerRequest: Request message for
// CreateJobTrigger.
type GooglePrivacyDlpV2beta2CreateJobTriggerRequest struct {
	// JobTrigger: The JobTrigger to create.
	JobTrigger *GooglePrivacyDlpV2beta2JobTrigger `json:"jobTrigger,omitempty"`

	// TriggerId: The trigger id can contain uppercase and lowercase
	// letters,
	// numbers, and hyphens; that is, it must match the regular
	// expression: `[a-zA-Z\\d-]+`. The maximum length is 100
	// characters. Can be empty to allow the system to generate one.
	TriggerId string `json:"triggerId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "JobTrigger") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "JobTrigger") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CreateJobTriggerRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CreateJobTriggerRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CryptoHashConfig: Pseudonymization method that
// generates surrogates via cryptographic hashing.
// Uses SHA-256.
// The key size must be either 32 or 64 bytes.
// Outputs a 32 byte digest as an uppercase hex string
// (for example, 41D1567F7F99F1DC2A5FAB886DEE5BEE).
// Currently, only string and integer values can be hashed.
type GooglePrivacyDlpV2beta2CryptoHashConfig struct {
	// CryptoKey: The key used by the hash function.
	CryptoKey *GooglePrivacyDlpV2beta2CryptoKey `json:"cryptoKey,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CryptoKey") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CryptoKey") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CryptoHashConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CryptoHashConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CryptoKey: This is a data encryption key (DEK)
// (as opposed to
// a key encryption key (KEK) stored by KMS).
// When using KMS to wrap/unwrap DEKs, be sure to set an appropriate
// IAM policy on the KMS CryptoKey (KEK) to ensure an attacker
// cannot
// unwrap the data crypto key.
type GooglePrivacyDlpV2beta2CryptoKey struct {
	KmsWrapped *GooglePrivacyDlpV2beta2KmsWrappedCryptoKey `json:"kmsWrapped,omitempty"`

	Transient *GooglePrivacyDlpV2beta2TransientCryptoKey `json:"transient,omitempty"`

	Unwrapped *GooglePrivacyDlpV2beta2UnwrappedCryptoKey `json:"unwrapped,omitempty"`

	// ForceSendFields is a list of field names (e.g. "KmsWrapped") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "KmsWrapped") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CryptoKey) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CryptoKey
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CryptoReplaceFfxFpeConfig: Replaces an
// identifier with a surrogate using FPE with the FFX
// mode of operation; however when used in the `ReidentifyContent` API
// method,
// it serves the opposite function by reversing the surrogate back
// into
// the original identifier.
// The identifier must be encoded as ASCII.
// For a given crypto key and context, the same identifier will
// be
// replaced with the same surrogate.
// Identifiers must be at least two characters long.
// In the case that the identifier is the empty string, it will be
// skipped.
// See [Pseudonymization](/dlp/docs/pseudonymization) for example usage.
type GooglePrivacyDlpV2beta2CryptoReplaceFfxFpeConfig struct {
	// Possible values:
	//   "FFX_COMMON_NATIVE_ALPHABET_UNSPECIFIED"
	//   "NUMERIC" - [0-9] (radix of 10)
	//   "HEXADECIMAL" - [0-9A-F] (radix of 16)
	//   "UPPER_CASE_ALPHA_NUMERIC" - [0-9A-Z] (radix of 36)
	//   "ALPHA_NUMERIC" - [0-9A-Za-z] (radix of 62)
	CommonAlphabet string `json:"commonAlphabet,omitempty"`

	// Context: The 'tweak', a context may be used for higher security since
	// the same
	// identifier in two different contexts won't be given the same
	// surrogate. If
	// the context is not set, a default tweak will be used.
	//
	// If the context is set but:
	//
	// 1. there is no record present when transforming a given value or
	// 1. the field is not present when transforming a given value,
	//
	// a default tweak will be used.
	//
	// Note that case (1) is expected when an `InfoTypeTransformation`
	// is
	// applied to both structured and non-structured
	// `ContentItem`s.
	// Currently, the referenced field may be of value type integer or
	// string.
	//
	// The tweak is constructed as a sequence of bytes in big endian byte
	// order
	// such that:
	//
	// - a 64 bit integer is encoded followed by a single byte of value 1
	// - a string is encoded in UTF-8 format followed by a single byte of
	// value
	//  å 2
	Context *GooglePrivacyDlpV2beta2FieldId `json:"context,omitempty"`

	// CryptoKey: The key used by the encryption algorithm. [required]
	CryptoKey *GooglePrivacyDlpV2beta2CryptoKey `json:"cryptoKey,omitempty"`

	// CustomAlphabet: This is supported by mapping these to the
	// alphanumeric characters
	// that the FFX mode natively supports. This happens
	// before/after
	// encryption/decryption.
	// Each character listed must appear only once.
	// Number of characters must be in the range [2, 62].
	// This must be encoded as ASCII.
	// The order of characters does not matter.
	CustomAlphabet string `json:"customAlphabet,omitempty"`

	// Radix: The native way to select the alphabet. Must be in the range
	// [2, 62].
	Radix int64 `json:"radix,omitempty"`

	// SurrogateInfoType: The custom infoType to annotate the surrogate
	// with.
	// This annotation will be applied to the surrogate by prefixing it
	// with
	// the name of the custom infoType followed by the number of
	// characters comprising the surrogate. The following scheme defines
	// the
	// format: info_type_name(surrogate_character_count):surrogate
	//
	// For example, if the name of custom infoType is 'MY_TOKEN_INFO_TYPE'
	// and
	// the surrogate is 'abc', the full replacement value
	// will be: 'MY_TOKEN_INFO_TYPE(3):abc'
	//
	// This annotation identifies the surrogate when inspecting content
	// using the
	// custom
	// infoType
	// [`SurrogateType`](/dlp/docs/reference/rest/v2beta2/InspectCon
	// fig#surrogatetype).
	// This facilitates reversal of the surrogate when it occurs in free
	// text.
	//
	// In order for inspection to work properly, the name of this infoType
	// must
	// not occur naturally anywhere in your data; otherwise, inspection
	// may
	// find a surrogate that does not correspond to an actual
	// identifier.
	// Therefore, choose your custom infoType name carefully after
	// considering
	// what your data looks like. One way to select a name that has a high
	// chance
	// of yielding reliable detection is to include one or more unicode
	// characters
	// that are highly improbable to exist in your data.
	// For example, assuming your data is entered from a regular ASCII
	// keyboard,
	// the symbol with the hex code point 29DD might be used like
	// so:
	// ⧝MY_TOKEN_TYPE
	SurrogateInfoType *GooglePrivacyDlpV2beta2InfoType `json:"surrogateInfoType,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CommonAlphabet") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CommonAlphabet") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CryptoReplaceFfxFpeConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CryptoReplaceFfxFpeConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2CustomInfoType: Custom information type
// provided by the user. Used to find domain-specific
// sensitive information configurable to the data in question.
type GooglePrivacyDlpV2beta2CustomInfoType struct {
	// DetectionRules: Set of detection rules to apply to all findings of
	// this custom info type.
	// Rules are applied in order that they are specified. Not supported for
	// the
	// `surrogate_type` custom info type.
	DetectionRules []*GooglePrivacyDlpV2beta2DetectionRule `json:"detectionRules,omitempty"`

	// Dictionary: Dictionary-based custom info type.
	Dictionary *GooglePrivacyDlpV2beta2Dictionary `json:"dictionary,omitempty"`

	// InfoType: Info type configuration. All custom info types must have
	// configurations
	// that do not conflict with built-in info types or other custom info
	// types.
	InfoType *GooglePrivacyDlpV2beta2InfoType `json:"infoType,omitempty"`

	// Likelihood: Likelihood to return for this custom info type. This base
	// value can be
	// altered by a detection rule if the finding meets the criteria
	// specified by
	// the rule. Defaults to `VERY_LIKELY` if not specified.
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

	// Regex: Regex-based custom info type.
	Regex *GooglePrivacyDlpV2beta2Regex `json:"regex,omitempty"`

	// SurrogateType: Surrogate info type.
	SurrogateType *GooglePrivacyDlpV2beta2SurrogateType `json:"surrogateType,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DetectionRules") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DetectionRules") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2CustomInfoType) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2CustomInfoType
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2DatastoreKey: Record key for a finding in
// Cloud Datastore.
type GooglePrivacyDlpV2beta2DatastoreKey struct {
	// EntityKey: Datastore entity key.
	EntityKey *GooglePrivacyDlpV2beta2Key `json:"entityKey,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2DatastoreKey) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2DatastoreKey
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2DatastoreOptions: Options defining a data set
// within Google Cloud Datastore.
type GooglePrivacyDlpV2beta2DatastoreOptions struct {
	// Kind: The kind to process.
	Kind *GooglePrivacyDlpV2beta2KindExpression `json:"kind,omitempty"`

	// PartitionId: A partition ID identifies a grouping of entities. The
	// grouping is always
	// by project and namespace, however the namespace ID may be empty.
	PartitionId *GooglePrivacyDlpV2beta2PartitionId `json:"partitionId,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2DatastoreOptions) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2DatastoreOptions
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2DateShiftConfig: Shifts dates by random number
// of days, with option to be consistent for the
// same context.
type GooglePrivacyDlpV2beta2DateShiftConfig struct {
	// Context: Points to the field that contains the context, for example,
	// an entity id.
	// If set, must also set method. If set, shift will be consistent for
	// the
	// given context.
	Context *GooglePrivacyDlpV2beta2FieldId `json:"context,omitempty"`

	// CryptoKey: Causes the shift to be computed based on this key and the
	// context. This
	// results in the same shift for the same context and crypto_key.
	CryptoKey *GooglePrivacyDlpV2beta2CryptoKey `json:"cryptoKey,omitempty"`

	// LowerBoundDays: For example, -5 means shift date to at most 5 days
	// back in the past.
	// [Required]
	LowerBoundDays int64 `json:"lowerBoundDays,omitempty"`

	// UpperBoundDays: Range of shift in days. Actual shift will be selected
	// at random within this
	// range (inclusive ends). Negative means shift to earlier in time. Must
	// not
	// be more than 365250 days (1000 years) each direction.
	//
	// For example, 3 means shift date to at most 3 days into the
	// future.
	// [Required]
	UpperBoundDays int64 `json:"upperBoundDays,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Context") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Context") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2DateShiftConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2DateShiftConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2DateTime: Message for a date time object.
type GooglePrivacyDlpV2beta2DateTime struct {
	// Date: One or more of the following must be set. All fields are
	// optional, but
	// when set must be valid date or time values.
	Date *GoogleTypeDate `json:"date,omitempty"`

	// Possible values:
	//   "DAY_OF_WEEK_UNSPECIFIED" - The unspecified day-of-week.
	//   "MONDAY" - The day-of-week of Monday.
	//   "TUESDAY" - The day-of-week of Tuesday.
	//   "WEDNESDAY" - The day-of-week of Wednesday.
	//   "THURSDAY" - The day-of-week of Thursday.
	//   "FRIDAY" - The day-of-week of Friday.
	//   "SATURDAY" - The day-of-week of Saturday.
	//   "SUNDAY" - The day-of-week of Sunday.
	DayOfWeek string `json:"dayOfWeek,omitempty"`

	Time *GoogleTypeTimeOfDay `json:"time,omitempty"`

	TimeZone *GooglePrivacyDlpV2beta2TimeZone `json:"timeZone,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Date") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Date") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2DateTime) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2DateTime
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2DeidentifyConfig: The configuration that
// controls how the data will change.
type GooglePrivacyDlpV2beta2DeidentifyConfig struct {
	// InfoTypeTransformations: Treat the dataset as free-form text and
	// apply the same free text
	// transformation everywhere.
	InfoTypeTransformations *GooglePrivacyDlpV2beta2InfoTypeTransformations `json:"infoTypeTransformations,omitempty"`

	// RecordTransformations: Treat the dataset as structured.
	// Transformations can be applied to
	// specific locations within structured datasets, such as transforming
	// a column within a table.
	RecordTransformations *GooglePrivacyDlpV2beta2RecordTransformations `json:"recordTransformations,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "InfoTypeTransformations") to unconditionally include in API
	// requests. By default, fields with empty values are omitted from API
	// requests. However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "InfoTypeTransformations")
	// to include in API requests with the JSON null value. By default,
	// fields with empty values are omitted from API requests. However, any
	// field with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2DeidentifyConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2DeidentifyConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2DeidentifyContentRequest: Request to
// de-identify a list of items.
type GooglePrivacyDlpV2beta2DeidentifyContentRequest struct {
	// DeidentifyConfig: Configuration for the de-identification of the
	// content item.
	// Items specified here will override the template referenced by
	// the
	// deidentify_template_name argument.
	DeidentifyConfig *GooglePrivacyDlpV2beta2DeidentifyConfig `json:"deidentifyConfig,omitempty"`

	// DeidentifyTemplateName: Optional template to use. Any configuration
	// directly specified in
	// deidentify_config will override those set in the template. Singular
	// fields
	// that are set in this request will replace their corresponding fields
	// in the
	// template. Repeated fields are appended. Singular sub-messages and
	// groups
	// are recursively merged.
	DeidentifyTemplateName string `json:"deidentifyTemplateName,omitempty"`

	// InspectConfig: Configuration for the inspector.
	// Items specified here will override the template referenced by
	// the
	// inspect_template_name argument.
	InspectConfig *GooglePrivacyDlpV2beta2InspectConfig `json:"inspectConfig,omitempty"`

	// InspectTemplateName: Optional template to use. Any configuration
	// directly specified in
	// inspect_config will override those set in the template. Singular
	// fields
	// that are set in this request will replace their corresponding fields
	// in the
	// template. Repeated fields are appended. Singular sub-messages and
	// groups
	// are recursively merged.
	InspectTemplateName string `json:"inspectTemplateName,omitempty"`

	// Item: The item to de-identify. Will be treated as text.
	Item *GooglePrivacyDlpV2beta2ContentItem `json:"item,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DeidentifyConfig") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DeidentifyConfig") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2DeidentifyContentRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2DeidentifyContentRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2DeidentifyContentResponse: Results of
// de-identifying a ContentItem.
type GooglePrivacyDlpV2beta2DeidentifyContentResponse struct {
	// Item: The de-identified item.
	Item *GooglePrivacyDlpV2beta2ContentItem `json:"item,omitempty"`

	// Overview: An overview of the changes that were made on the `item`.
	Overview *GooglePrivacyDlpV2beta2TransformationOverview `json:"overview,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Item") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Item") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2DeidentifyContentResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2DeidentifyContentResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2DeidentifyTemplate: The DeidentifyTemplates
// contains instructions on how to deidentify content.
type GooglePrivacyDlpV2beta2DeidentifyTemplate struct {
	// CreateTime: The creation timestamp of a inspectTemplate, output only
	// field.
	CreateTime string `json:"createTime,omitempty"`

	// DeidentifyConfig: ///////////// // The core content of the template
	// // ///////////////
	DeidentifyConfig *GooglePrivacyDlpV2beta2DeidentifyConfig `json:"deidentifyConfig,omitempty"`

	// Description: Short description (max 256 chars).
	Description string `json:"description,omitempty"`

	// DisplayName: Display name (max 256 chars).
	DisplayName string `json:"displayName,omitempty"`

	// Name: The template name. Output only.
	//
	// The template will have one of the following
	// formats:
	// `projects/PROJECT_ID/deidentifyTemplates/TEMPLATE_ID`
	// OR
	// `organizations/ORGANIZATION_ID/deidentifyTemplates/TEMPLATE_ID`
	Name string `json:"name,omitempty"`

	// UpdateTime: The last update timestamp of a inspectTemplate, output
	// only field.
	UpdateTime string `json:"updateTime,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

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

func (s *GooglePrivacyDlpV2beta2DeidentifyTemplate) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2DeidentifyTemplate
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2DetectionRule: Rule for modifying a custom
// info type to alter behavior under certain
// circumstances, depending on the specific details of the rule. Not
// supported
// for the `surrogate_type` custom info type.
type GooglePrivacyDlpV2beta2DetectionRule struct {
	// HotwordRule: Hotword-based detection rule.
	HotwordRule *GooglePrivacyDlpV2beta2HotwordRule `json:"hotwordRule,omitempty"`

	// ForceSendFields is a list of field names (e.g. "HotwordRule") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "HotwordRule") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2DetectionRule) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2DetectionRule
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Dictionary: Custom information type based on a
// dictionary of words or phrases. This can
// be used to match sensitive information specific to the data, such as
// a list
// of employee IDs or job titles.
//
// Dictionary words are case-insensitive and all characters other than
// letters
// and digits in the unicode [Basic
// Multilingual
// Plane](https://en.wikipedia.org/wiki/Plane_%28Unicode%29#
// Basic_Multilingual_Plane)
// will be replaced with whitespace when scanning for matches, so
// the
// dictionary phrase "Sam Johnson" will match all three phrases "sam
// johnson",
// "Sam, Johnson", and "Sam (Johnson)". Additionally, the
// characters
// surrounding any match must be of a different type than the
// adjacent
// characters within the word, so letters must be next to non-letters
// and
// digits next to non-digits. For example, the dictionary word "jen"
// will
// match the first three letters of the text "jen123" but will return
// no
// matches for "jennifer".
//
// Dictionary words containing a large number of characters that are
// not
// letters or digits may result in unexpected findings because such
// characters
// are treated as whitespace.
type GooglePrivacyDlpV2beta2Dictionary struct {
	// WordList: List of words or phrases to search for.
	WordList *GooglePrivacyDlpV2beta2WordList `json:"wordList,omitempty"`

	// ForceSendFields is a list of field names (e.g. "WordList") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "WordList") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Dictionary) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Dictionary
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2DlpJob: Combines all of the information about
// a DLP job.
type GooglePrivacyDlpV2beta2DlpJob struct {
	// CreateTime: Time when the job was created.
	CreateTime string `json:"createTime,omitempty"`

	// EndTime: Time when the job finished.
	EndTime string `json:"endTime,omitempty"`

	// ErrorResults: A stream of errors encountered running the job.
	ErrorResults []*GoogleRpcStatus `json:"errorResults,omitempty"`

	// InspectDetails: Results from inspecting a data source.
	InspectDetails *GooglePrivacyDlpV2beta2InspectDataSourceDetails `json:"inspectDetails,omitempty"`

	// JobTriggerName: If created by a job trigger, the resource name of the
	// trigger that
	// instantiated the job.
	JobTriggerName string `json:"jobTriggerName,omitempty"`

	// Name: The server-assigned name.
	Name string `json:"name,omitempty"`

	// RiskDetails: Results from analyzing risk of a data source.
	RiskDetails *GooglePrivacyDlpV2beta2AnalyzeDataSourceRiskDetails `json:"riskDetails,omitempty"`

	// StartTime: Time when the job started.
	StartTime string `json:"startTime,omitempty"`

	// State: State of a job.
	//
	// Possible values:
	//   "JOB_STATE_UNSPECIFIED"
	//   "PENDING" - The job has not yet started.
	//   "RUNNING" - The job is currently running.
	//   "DONE" - The job is no longer running.
	//   "CANCELED" - The job was canceled before it could complete.
	//   "FAILED" - The job had an error and did not complete.
	State string `json:"state,omitempty"`

	// Type: The type of job.
	//
	// Possible values:
	//   "DLP_JOB_TYPE_UNSPECIFIED"
	//   "INSPECT_JOB" - The job inspected Google Cloud for sensitive data.
	//   "RISK_ANALYSIS_JOB" - The job executed a Risk Analysis computation.
	Type string `json:"type,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

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

func (s *GooglePrivacyDlpV2beta2DlpJob) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2DlpJob
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2EntityId: An entity in a dataset is a field or
// set of fields that correspond to a
// single person. For example, in medical records the `EntityId` might
// be
// a patient identifier, or for financial records it might be an
// account
// identifier. This message is used when generalizations or analysis
// must be
// consistent across multiple rows pertaining to the same entity.
type GooglePrivacyDlpV2beta2EntityId struct {
	// Field: Composite key indicating which field contains the entity
	// identifier.
	Field *GooglePrivacyDlpV2beta2FieldId `json:"field,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Field") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Field") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2EntityId) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2EntityId
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Error: The results of an unsuccessful
// activation of the JobTrigger.
type GooglePrivacyDlpV2beta2Error struct {
	Details *GoogleRpcStatus `json:"details,omitempty"`

	// Timestamps: The times the error occurred.
	Timestamps []string `json:"timestamps,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Details") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Details") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Error) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Error
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Expressions: An expression, consisting or an
// operator and conditions.
type GooglePrivacyDlpV2beta2Expressions struct {
	Conditions *GooglePrivacyDlpV2beta2Conditions `json:"conditions,omitempty"`

	// LogicalOperator: The operator to apply to the result of conditions.
	// Default and currently
	// only supported value is `AND`.
	//
	// Possible values:
	//   "LOGICAL_OPERATOR_UNSPECIFIED"
	//   "AND"
	LogicalOperator string `json:"logicalOperator,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Conditions") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Conditions") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Expressions) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Expressions
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2FieldId: General identifier of a data field in
// a storage service.
type GooglePrivacyDlpV2beta2FieldId struct {
	// Name: Name describing the field.
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

func (s *GooglePrivacyDlpV2beta2FieldId) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2FieldId
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2FieldTransformation: The transformation to
// apply to the field.
type GooglePrivacyDlpV2beta2FieldTransformation struct {
	// Condition: Only apply the transformation if the condition evaluates
	// to true for the
	// given `RecordCondition`. The conditions are allowed to reference
	// fields
	// that are not used in the actual transformation. [optional]
	//
	// Example Use Cases:
	//
	// - Apply a different bucket transformation to an age column if the zip
	// code
	// column for the same record is within a specific range.
	// - Redact a field if the date of birth field is greater than 85.
	Condition *GooglePrivacyDlpV2beta2RecordCondition `json:"condition,omitempty"`

	// Fields: Input field(s) to apply the transformation to. [required]
	Fields []*GooglePrivacyDlpV2beta2FieldId `json:"fields,omitempty"`

	// InfoTypeTransformations: Treat the contents of the field as free
	// text, and selectively
	// transform content that matches an `InfoType`.
	InfoTypeTransformations *GooglePrivacyDlpV2beta2InfoTypeTransformations `json:"infoTypeTransformations,omitempty"`

	// PrimitiveTransformation: Apply the transformation to the entire
	// field.
	PrimitiveTransformation *GooglePrivacyDlpV2beta2PrimitiveTransformation `json:"primitiveTransformation,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Condition") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Condition") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2FieldTransformation) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2FieldTransformation
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2FileSet: Set of files to scan.
type GooglePrivacyDlpV2beta2FileSet struct {
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

func (s *GooglePrivacyDlpV2beta2FileSet) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2FileSet
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Finding: Represents a piece of potentially
// sensitive content.
type GooglePrivacyDlpV2beta2Finding struct {
	// CreateTime: Timestamp when finding was detected.
	CreateTime string `json:"createTime,omitempty"`

	// InfoType: The type of content that might have been found.
	// Provided if requested by the `InspectConfig`.
	InfoType *GooglePrivacyDlpV2beta2InfoType `json:"infoType,omitempty"`

	// Likelihood: Estimate of how likely it is that the `info_type` is
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

	// Location: Where the content was found.
	Location *GooglePrivacyDlpV2beta2Location `json:"location,omitempty"`

	// Quote: The content that was found. Even if the content is not
	// textual, it
	// may be converted to a textual representation here.
	// Provided if requested by the `InspectConfig` and the finding is
	// less than or equal to 4096 bytes long. If the finding exceeds 4096
	// bytes
	// in length, the quote may be omitted.
	Quote string `json:"quote,omitempty"`

	// QuoteInfo: Contains data parsed from quotes. Only populated if
	// include_quote was set
	// to true and a supported infoType was requested. Currently
	// supported
	// infoTypes: DATE, DATE_OF_BIRTH and TIME.
	QuoteInfo *GooglePrivacyDlpV2beta2QuoteInfo `json:"quoteInfo,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2Finding) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Finding
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type GooglePrivacyDlpV2beta2FindingLimits struct {
	// MaxFindingsPerInfoType: Configuration of findings limit given for
	// specified infoTypes.
	MaxFindingsPerInfoType []*GooglePrivacyDlpV2beta2InfoTypeLimit `json:"maxFindingsPerInfoType,omitempty"`

	// MaxFindingsPerItem: Max number of findings that will be returned for
	// each item scanned.
	// When set within `InspectDataSourceRequest`,
	// the maximum returned is 1000 regardless if this is set higher.
	// When set within `InspectContentRequest`, this field is ignored.
	MaxFindingsPerItem int64 `json:"maxFindingsPerItem,omitempty"`

	// MaxFindingsPerRequest: Max number of findings that will be returned
	// per request/job.
	// When set within `InspectContentRequest`, the maximum returned is
	// 1000
	// regardless if this is set higher.
	MaxFindingsPerRequest int64 `json:"maxFindingsPerRequest,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "MaxFindingsPerInfoType") to unconditionally include in API requests.
	// By default, fields with empty values are omitted from API requests.
	// However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "MaxFindingsPerInfoType")
	// to include in API requests with the JSON null value. By default,
	// fields with empty values are omitted from API requests. However, any
	// field with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2FindingLimits) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2FindingLimits
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2FixedSizeBucketingConfig: Buckets values based
// on fixed size ranges. The
// Bucketing transformation can provide all of this functionality,
// but requires more configuration. This message is provided as a
// convenience to
// the user for simple bucketing strategies.
//
// The transformed value will be a hyphenated string
// of
// <lower_bound>-<upper_bound>, i.e if lower_bound = 10 and upper_bound
// = 20
// all values that are within this bucket will be replaced with
// "10-20".
//
// This can be used on data of type: double, long.
//
// If the bound Value type differs from the type of data
// being transformed, we will first attempt converting the type of the
// data to
// be transformed to match the type of the bound before comparing.
type GooglePrivacyDlpV2beta2FixedSizeBucketingConfig struct {
	// BucketSize: Size of each bucket (except for minimum and maximum
	// buckets). So if
	// `lower_bound` = 10, `upper_bound` = 89, and `bucket_size` = 10, then
	// the
	// following buckets would be used: -10, 10-20, 20-30, 30-40, 40-50,
	// 50-60,
	// 60-70, 70-80, 80-89, 89+. Precision up to 2 decimals works.
	// [Required].
	BucketSize float64 `json:"bucketSize,omitempty"`

	// LowerBound: Lower bound value of buckets. All values less than
	// `lower_bound` are
	// grouped together into a single bucket; for example if `lower_bound` =
	// 10,
	// then all values less than 10 are replaced with the value “-10”.
	// [Required].
	LowerBound *GooglePrivacyDlpV2beta2Value `json:"lowerBound,omitempty"`

	// UpperBound: Upper bound value of buckets. All values greater than
	// upper_bound are
	// grouped together into a single bucket; for example if `upper_bound` =
	// 89,
	// then all values greater than 89 are replaced with the value
	// “89+”.
	// [Required].
	UpperBound *GooglePrivacyDlpV2beta2Value `json:"upperBound,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BucketSize") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BucketSize") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2FixedSizeBucketingConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2FixedSizeBucketingConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GooglePrivacyDlpV2beta2FixedSizeBucketingConfig) UnmarshalJSON(data []byte) error {
	type NoMethod GooglePrivacyDlpV2beta2FixedSizeBucketingConfig
	var s1 struct {
		BucketSize gensupport.JSONFloat64 `json:"bucketSize"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.BucketSize = float64(s1.BucketSize)
	return nil
}

// GooglePrivacyDlpV2beta2HotwordRule: Detection rule that adjusts the
// likelihood of findings within a certain
// proximity of hotwords.
type GooglePrivacyDlpV2beta2HotwordRule struct {
	// HotwordRegex: Regex pattern defining what qualifies as a hotword.
	HotwordRegex *GooglePrivacyDlpV2beta2Regex `json:"hotwordRegex,omitempty"`

	// LikelihoodAdjustment: Likelihood adjustment to apply to all matching
	// findings.
	LikelihoodAdjustment *GooglePrivacyDlpV2beta2LikelihoodAdjustment `json:"likelihoodAdjustment,omitempty"`

	// Proximity: Proximity of the finding within which the entire hotword
	// must reside.
	// The total length of the window cannot exceed 1000 characters. Note
	// that
	// the finding itself will be included in the window, so that hotwords
	// may
	// be used to match substrings of the finding itself. For example,
	// the
	// certainty of a phone number regex "\(\d{3}\) \d{3}-\d{4}" could
	// be
	// adjusted upwards if the area code is known to be the local area code
	// of
	// a company office using the hotword regex "\(xxx\)", where "xxx"
	// is the area code in question.
	Proximity *GooglePrivacyDlpV2beta2Proximity `json:"proximity,omitempty"`

	// ForceSendFields is a list of field names (e.g. "HotwordRegex") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "HotwordRegex") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2HotwordRule) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2HotwordRule
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2ImageLocation: Bounding box encompassing
// detected text within an image.
type GooglePrivacyDlpV2beta2ImageLocation struct {
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

func (s *GooglePrivacyDlpV2beta2ImageLocation) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2ImageLocation
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2ImageRedactionConfig: Configuration for
// determining how redaction of images should occur.
type GooglePrivacyDlpV2beta2ImageRedactionConfig struct {
	// InfoType: Only one per info_type should be provided per request. If
	// not
	// specified, and redact_all_text is false, the DLP API will redact
	// all
	// text that it matches against all info_types that are found, but
	// not
	// specified in another ImageRedactionConfig.
	InfoType *GooglePrivacyDlpV2beta2InfoType `json:"infoType,omitempty"`

	// RedactAllText: If true, all text found in the image, regardless
	// whether it matches an
	// info_type, is redacted.
	RedactAllText bool `json:"redactAllText,omitempty"`

	// RedactionColor: The color to use when redacting content from an
	// image. If not specified,
	// the default is black.
	RedactionColor *GooglePrivacyDlpV2beta2Color `json:"redactionColor,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2ImageRedactionConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2ImageRedactionConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InfoType: Type of information detected by the
// API.
type GooglePrivacyDlpV2beta2InfoType struct {
	// Name: Name of the information type.
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

func (s *GooglePrivacyDlpV2beta2InfoType) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InfoType
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InfoTypeDescription: InfoType description.
type GooglePrivacyDlpV2beta2InfoTypeDescription struct {
	// DisplayName: Human readable form of the infoType name.
	DisplayName string `json:"displayName,omitempty"`

	// Name: Internal name of the infoType.
	Name string `json:"name,omitempty"`

	// SupportedBy: Which parts of the API supports this InfoType.
	//
	// Possible values:
	//   "ENUM_TYPE_UNSPECIFIED"
	//   "INSPECT" - Supported by the inspect operations.
	//   "RISK_ANALYSIS" - Supported by the risk analysis operations.
	SupportedBy []string `json:"supportedBy,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2InfoTypeDescription) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InfoTypeDescription
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InfoTypeLimit: Max findings configuration per
// infoType, per content item or long
// running DlpJob.
type GooglePrivacyDlpV2beta2InfoTypeLimit struct {
	// InfoType: Type of information the findings limit applies to. Only one
	// limit per
	// info_type should be provided. If InfoTypeLimit does not have
	// an
	// info_type, the DLP API applies the limit against all info_types
	// that
	// are found but not specified in another InfoTypeLimit.
	InfoType *GooglePrivacyDlpV2beta2InfoType `json:"infoType,omitempty"`

	// MaxFindings: Max findings limit for the given infoType.
	MaxFindings int64 `json:"maxFindings,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2InfoTypeLimit) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InfoTypeLimit
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InfoTypeStatistics: Statistics regarding a
// specific InfoType.
type GooglePrivacyDlpV2beta2InfoTypeStatistics struct {
	// Count: Number of findings for this infoType.
	Count int64 `json:"count,omitempty,string"`

	// InfoType: The type of finding this stat is for.
	InfoType *GooglePrivacyDlpV2beta2InfoType `json:"infoType,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2InfoTypeStatistics) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InfoTypeStatistics
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InfoTypeTransformation: A transformation to
// apply to text that is identified as a specific
// info_type.
type GooglePrivacyDlpV2beta2InfoTypeTransformation struct {
	// InfoTypes: InfoTypes to apply the transformation to. Empty list will
	// match all
	// available infoTypes for this transformation.
	InfoTypes []*GooglePrivacyDlpV2beta2InfoType `json:"infoTypes,omitempty"`

	// PrimitiveTransformation: Primitive transformation to apply to the
	// infoType. [required]
	PrimitiveTransformation *GooglePrivacyDlpV2beta2PrimitiveTransformation `json:"primitiveTransformation,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2InfoTypeTransformation) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InfoTypeTransformation
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InfoTypeTransformations: A type of
// transformation that will scan unstructured text and
// apply various `PrimitiveTransformation`s to each finding, where
// the
// transformation is applied to only values that were identified as a
// specific
// info_type.
type GooglePrivacyDlpV2beta2InfoTypeTransformations struct {
	// Transformations: Transformation for each infoType. Cannot specify
	// more than one
	// for a given infoType. [required]
	Transformations []*GooglePrivacyDlpV2beta2InfoTypeTransformation `json:"transformations,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Transformations") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Transformations") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2InfoTypeTransformations) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InfoTypeTransformations
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InspectConfig: Configuration description of
// the scanning process.
// When used with redactContent only info_types and min_likelihood are
// currently
// used.
type GooglePrivacyDlpV2beta2InspectConfig struct {
	// CustomInfoTypes: Custom infoTypes provided by the user.
	CustomInfoTypes []*GooglePrivacyDlpV2beta2CustomInfoType `json:"customInfoTypes,omitempty"`

	// ExcludeInfoTypes: When true, excludes type information of the
	// findings.
	ExcludeInfoTypes bool `json:"excludeInfoTypes,omitempty"`

	// IncludeQuote: When true, a contextual quote from the data that
	// triggered a finding is
	// included in the response; see Finding.quote.
	IncludeQuote bool `json:"includeQuote,omitempty"`

	// InfoTypes: Restricts what info_types to look for. The values must
	// correspond to
	// InfoType values returned by ListInfoTypes or found in
	// documentation.
	// Empty info_types runs all enabled detectors.
	InfoTypes []*GooglePrivacyDlpV2beta2InfoType `json:"infoTypes,omitempty"`

	Limits *GooglePrivacyDlpV2beta2FindingLimits `json:"limits,omitempty"`

	// MinLikelihood: Only returns findings equal or above this threshold.
	// The default is
	// POSSIBLE.
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

	// ForceSendFields is a list of field names (e.g. "CustomInfoTypes") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CustomInfoTypes") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2InspectConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InspectConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InspectContentRequest: Request to search for
// potentially sensitive info in a ContentItem.
type GooglePrivacyDlpV2beta2InspectContentRequest struct {
	// InspectConfig: Configuration for the inspector. What specified here
	// will override
	// the template referenced by the inspect_template_name argument.
	InspectConfig *GooglePrivacyDlpV2beta2InspectConfig `json:"inspectConfig,omitempty"`

	// InspectTemplateName: Optional template to use. Any configuration
	// directly specified in
	// inspect_config will override those set in the template. Singular
	// fields
	// that are set in this request will replace their corresponding fields
	// in the
	// template. Repeated fields are appended. Singular sub-messages and
	// groups
	// are recursively merged.
	InspectTemplateName string `json:"inspectTemplateName,omitempty"`

	// Item: The item to inspect.
	Item *GooglePrivacyDlpV2beta2ContentItem `json:"item,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2InspectContentRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InspectContentRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InspectContentResponse: Results of inspecting
// an item.
type GooglePrivacyDlpV2beta2InspectContentResponse struct {
	// Result: The findings.
	Result *GooglePrivacyDlpV2beta2InspectResult `json:"result,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Result") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Result") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2InspectContentResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InspectContentResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InspectDataSourceDetails: The results of an
// inspect DataSource job.
type GooglePrivacyDlpV2beta2InspectDataSourceDetails struct {
	// RequestedOptions: The configuration used for this job.
	RequestedOptions *GooglePrivacyDlpV2beta2RequestedOptions `json:"requestedOptions,omitempty"`

	// Result: A summary of the outcome of this inspect job.
	Result *GooglePrivacyDlpV2beta2Result `json:"result,omitempty"`

	// ForceSendFields is a list of field names (e.g. "RequestedOptions") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "RequestedOptions") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2InspectDataSourceDetails) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InspectDataSourceDetails
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InspectDataSourceRequest: Request for
// scheduling a scan of a data subset from a Google Platform
// data
// repository.
type GooglePrivacyDlpV2beta2InspectDataSourceRequest struct {
	// JobConfig: A configuration for the job.
	JobConfig *GooglePrivacyDlpV2beta2InspectJobConfig `json:"jobConfig,omitempty"`

	// JobId: Optional job ID to use for the created job. If not provided, a
	// job ID will
	// automatically be generated. Must be unique within the project. The
	// job ID
	// can contain uppercase and lowercase letters, numbers, and hyphens;
	// that is,
	// it must match the regular expression: `[a-zA-Z\\d-]+`. The maximum
	// length
	// is 100 characters. Can be empty to allow the system to generate one.
	JobId string `json:"jobId,omitempty"`

	// ForceSendFields is a list of field names (e.g. "JobConfig") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "JobConfig") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2InspectDataSourceRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InspectDataSourceRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type GooglePrivacyDlpV2beta2InspectJobConfig struct {
	// Actions: Actions to execute at the completion of the job. Are
	// executed in the order
	// provided.
	Actions []*GooglePrivacyDlpV2beta2Action `json:"actions,omitempty"`

	// InspectConfig: How and what to scan for.
	InspectConfig *GooglePrivacyDlpV2beta2InspectConfig `json:"inspectConfig,omitempty"`

	// InspectTemplateName: If provided, will be used as the default for all
	// values in InspectConfig.
	// `inspect_config` will be merged into the values persisted as part of
	// the
	// template.
	InspectTemplateName string `json:"inspectTemplateName,omitempty"`

	// OutputConfig: Where to put the findings.
	OutputConfig *GooglePrivacyDlpV2beta2OutputStorageConfig `json:"outputConfig,omitempty"`

	// StorageConfig: The data to scan.
	StorageConfig *GooglePrivacyDlpV2beta2StorageConfig `json:"storageConfig,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Actions") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Actions") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2InspectJobConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InspectJobConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InspectResult: All the findings for a single
// scanned item.
type GooglePrivacyDlpV2beta2InspectResult struct {
	// Findings: List of findings for an item.
	Findings []*GooglePrivacyDlpV2beta2Finding `json:"findings,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2InspectResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InspectResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2InspectTemplate: The inspectTemplate contains
// a configuration (set of types of sensitive data
// to be detected) to be used anywhere you otherwise would normally
// specify
// InspectConfig.
type GooglePrivacyDlpV2beta2InspectTemplate struct {
	// CreateTime: The creation timestamp of a inspectTemplate, output only
	// field.
	CreateTime string `json:"createTime,omitempty"`

	// Description: Short description (max 256 chars).
	Description string `json:"description,omitempty"`

	// DisplayName: Display name (max 256 chars).
	DisplayName string `json:"displayName,omitempty"`

	// InspectConfig: The core content of the template. Configuration of the
	// scanning process.
	InspectConfig *GooglePrivacyDlpV2beta2InspectConfig `json:"inspectConfig,omitempty"`

	// Name: The template name. Output only.
	//
	// The template will have one of the following
	// formats:
	// `projects/PROJECT_ID/inspectTemplates/TEMPLATE_ID`
	// OR
	// `organizations/ORGANIZATION_ID/inspectTemplates/TEMPLATE_ID`
	Name string `json:"name,omitempty"`

	// UpdateTime: The last update timestamp of a inspectTemplate, output
	// only field.
	UpdateTime string `json:"updateTime,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

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

func (s *GooglePrivacyDlpV2beta2InspectTemplate) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2InspectTemplate
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2JobTrigger: Contains a configuration to make
// dlp api calls on a repeating basis.
type GooglePrivacyDlpV2beta2JobTrigger struct {
	// CreateTime: The creation timestamp of a triggeredJob, output only
	// field.
	CreateTime string `json:"createTime,omitempty"`

	// Description: User provided description (max 256 chars)
	Description string `json:"description,omitempty"`

	// DisplayName: Display name (max 100 chars)
	DisplayName string `json:"displayName,omitempty"`

	// Errors: A stream of errors encountered when the trigger was
	// activated. Repeated
	// errors may result in the JobTrigger automaticaly being paused.
	// Will return the last 100 errors. Whenever the JobTrigger is
	// modified
	// this list will be cleared. Output only field.
	Errors []*GooglePrivacyDlpV2beta2Error `json:"errors,omitempty"`

	InspectJob *GooglePrivacyDlpV2beta2InspectJobConfig `json:"inspectJob,omitempty"`

	// LastRunTime: The timestamp of the last time this trigger executed.
	LastRunTime string `json:"lastRunTime,omitempty"`

	// Name: Unique resource name for the triggeredJob, assigned by the
	// service when the
	// triggeredJob is created, for
	// example
	// `projects/dlp-test-project/triggeredJobs/53234423`.
	Name string `json:"name,omitempty"`

	// Status: A status for this trigger. [required]
	//
	// Possible values:
	//   "STATUS_UNSPECIFIED"
	//   "HEALTHY" - Trigger is healthy.
	//   "PAUSED" - Trigger is temporarily paused.
	//   "CANCELLED" - Trigger is cancelled and can not be resumed.
	Status string `json:"status,omitempty"`

	// Triggers: A list of triggers which will be OR'ed together. Only one
	// in the list
	// needs to trigger for a job to be started. The list may contain only
	// a single Schedule trigger and must have at least one object.
	Triggers []*GooglePrivacyDlpV2beta2Trigger `json:"triggers,omitempty"`

	// UpdateTime: The last update timestamp of a triggeredJob, output only
	// field.
	UpdateTime string `json:"updateTime,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

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

func (s *GooglePrivacyDlpV2beta2JobTrigger) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2JobTrigger
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2KAnonymityConfig: k-anonymity metric, used for
// analysis of reidentification risk.
type GooglePrivacyDlpV2beta2KAnonymityConfig struct {
	// EntityId: Optional message indicating that each distinct entity_id
	// should not
	// contribute to the k-anonymity count more than once per equivalence
	// class.
	// If an entity_id appears on several rows with different
	// quasi-identifier
	// tuples, it will contribute to each count exactly once.
	//
	// This can lead to unexpected results. Consider a table where ID 1
	// is
	// associated to quasi-identifier "foo", ID 2 to "bar", and ID 3 to
	// *both*
	// quasi-identifiers "foo" and "bar" (on separate rows), and where this
	// ID
	// is used as entity_id. Then, the anonymity value associated to ID 3
	// will
	// be 2, even if it is the only ID to be associated to both values "foo"
	// and
	// "bar".
	EntityId *GooglePrivacyDlpV2beta2EntityId `json:"entityId,omitempty"`

	// QuasiIds: Set of fields to compute k-anonymity over. When multiple
	// fields are
	// specified, they are considered a single composite key. Structs
	// and
	// repeated data types are not supported; however, nested fields
	// are
	// supported so long as they are not structs themselves or nested
	// within
	// a repeated field.
	QuasiIds []*GooglePrivacyDlpV2beta2FieldId `json:"quasiIds,omitempty"`

	// ForceSendFields is a list of field names (e.g. "EntityId") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "EntityId") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2KAnonymityConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2KAnonymityConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2KAnonymityEquivalenceClass: The set of
// columns' values that share the same ldiversity value
type GooglePrivacyDlpV2beta2KAnonymityEquivalenceClass struct {
	// EquivalenceClassSize: Size of the equivalence class, for example
	// number of rows with the
	// above set of values.
	EquivalenceClassSize int64 `json:"equivalenceClassSize,omitempty,string"`

	// QuasiIdsValues: Set of values defining the equivalence class. One
	// value per
	// quasi-identifier column in the original KAnonymity metric
	// message.
	// The order is always the same as the original request.
	QuasiIdsValues []*GooglePrivacyDlpV2beta2Value `json:"quasiIdsValues,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "EquivalenceClassSize") to unconditionally include in API requests.
	// By default, fields with empty values are omitted from API requests.
	// However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "EquivalenceClassSize") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2KAnonymityEquivalenceClass) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2KAnonymityEquivalenceClass
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type GooglePrivacyDlpV2beta2KAnonymityHistogramBucket struct {
	// BucketSize: Total number of equivalence classes in this bucket.
	BucketSize int64 `json:"bucketSize,omitempty,string"`

	// BucketValueCount: Total number of distinct equivalence classes in
	// this bucket.
	BucketValueCount int64 `json:"bucketValueCount,omitempty,string"`

	// BucketValues: Sample of equivalence classes in this bucket. The total
	// number of
	// classes returned per bucket is capped at 20.
	BucketValues []*GooglePrivacyDlpV2beta2KAnonymityEquivalenceClass `json:"bucketValues,omitempty"`

	// EquivalenceClassSizeLowerBound: Lower bound on the size of the
	// equivalence classes in this bucket.
	EquivalenceClassSizeLowerBound int64 `json:"equivalenceClassSizeLowerBound,omitempty,string"`

	// EquivalenceClassSizeUpperBound: Upper bound on the size of the
	// equivalence classes in this bucket.
	EquivalenceClassSizeUpperBound int64 `json:"equivalenceClassSizeUpperBound,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "BucketSize") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BucketSize") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2KAnonymityHistogramBucket) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2KAnonymityHistogramBucket
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2KAnonymityResult: Result of the k-anonymity
// computation.
type GooglePrivacyDlpV2beta2KAnonymityResult struct {
	// EquivalenceClassHistogramBuckets: Histogram of k-anonymity
	// equivalence classes.
	EquivalenceClassHistogramBuckets []*GooglePrivacyDlpV2beta2KAnonymityHistogramBucket `json:"equivalenceClassHistogramBuckets,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "EquivalenceClassHistogramBuckets") to unconditionally include in API
	// requests. By default, fields with empty values are omitted from API
	// requests. However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g.
	// "EquivalenceClassHistogramBuckets") to include in API requests with
	// the JSON null value. By default, fields with empty values are omitted
	// from API requests. However, any field with an empty value appearing
	// in NullFields will be sent to the server as null. It is an error if a
	// field in this list has a non-empty value. This may be used to include
	// null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2KAnonymityResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2KAnonymityResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2KMapEstimationConfig: Reidentifiability
// metric. This corresponds to a risk model similar to what
// is called "journalist risk" in the literature, except the attack
// dataset is
// statistically modeled instead of being perfectly known. This can be
// done
// using publicly available data (like the US Census), or using a
// custom
// statistical model (indicated as one or several BigQuery tables), or
// by
// extrapolating from the distribution of values in the input dataset.
type GooglePrivacyDlpV2beta2KMapEstimationConfig struct {
	// AuxiliaryTables: Several auxiliary tables can be used in the
	// analysis. Each custom_tag
	// used to tag a quasi-identifiers column must appear in exactly one
	// column
	// of one auxiliary table.
	AuxiliaryTables []*GooglePrivacyDlpV2beta2AuxiliaryTable `json:"auxiliaryTables,omitempty"`

	// QuasiIds: Fields considered to be quasi-identifiers. No two columns
	// can have the
	// same tag. [required]
	QuasiIds []*GooglePrivacyDlpV2beta2TaggedField `json:"quasiIds,omitempty"`

	// RegionCode: ISO 3166-1 alpha-2 region code to use in the statistical
	// modeling.
	// Required if no column is tagged with a region-specific InfoType
	// (like
	// US_ZIP_5) or a region code.
	RegionCode string `json:"regionCode,omitempty"`

	// ForceSendFields is a list of field names (e.g. "AuxiliaryTables") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "AuxiliaryTables") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2KMapEstimationConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2KMapEstimationConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2KMapEstimationHistogramBucket: A
// KMapEstimationHistogramBucket message with the following values:
//   min_anonymity: 3
//   max_anonymity: 5
//   frequency: 42
// means that there are 42 records whose quasi-identifier values
// correspond
// to 3, 4 or 5 people in the overlying population. An important
// particular
// case is when min_anonymity = max_anonymity = 1: the frequency field
// then
// corresponds to the number of uniquely identifiable records.
type GooglePrivacyDlpV2beta2KMapEstimationHistogramBucket struct {
	// BucketSize: Number of records within these anonymity bounds.
	BucketSize int64 `json:"bucketSize,omitempty,string"`

	// BucketValueCount: Total number of distinct quasi-identifier tuple
	// values in this bucket.
	BucketValueCount int64 `json:"bucketValueCount,omitempty,string"`

	// BucketValues: Sample of quasi-identifier tuple values in this bucket.
	// The total
	// number of classes returned per bucket is capped at 20.
	BucketValues []*GooglePrivacyDlpV2beta2KMapEstimationQuasiIdValues `json:"bucketValues,omitempty"`

	// MaxAnonymity: Always greater than or equal to min_anonymity.
	MaxAnonymity int64 `json:"maxAnonymity,omitempty,string"`

	// MinAnonymity: Always positive.
	MinAnonymity int64 `json:"minAnonymity,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "BucketSize") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BucketSize") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2KMapEstimationHistogramBucket) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2KMapEstimationHistogramBucket
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2KMapEstimationQuasiIdValues: A tuple of values
// for the quasi-identifier columns.
type GooglePrivacyDlpV2beta2KMapEstimationQuasiIdValues struct {
	// EstimatedAnonymity: The estimated anonymity for these
	// quasi-identifier values.
	EstimatedAnonymity int64 `json:"estimatedAnonymity,omitempty,string"`

	// QuasiIdsValues: The quasi-identifier values.
	QuasiIdsValues []*GooglePrivacyDlpV2beta2Value `json:"quasiIdsValues,omitempty"`

	// ForceSendFields is a list of field names (e.g. "EstimatedAnonymity")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "EstimatedAnonymity") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2KMapEstimationQuasiIdValues) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2KMapEstimationQuasiIdValues
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2KMapEstimationResult: Result of the
// reidentifiability analysis. Note that these results are
// an
// estimation, not exact values.
type GooglePrivacyDlpV2beta2KMapEstimationResult struct {
	// KMapEstimationHistogram: The intervals [min_anonymity, max_anonymity]
	// do not overlap. If a value
	// doesn't correspond to any such interval, the associated frequency
	// is
	// zero. For example, the following records:
	//   {min_anonymity: 1, max_anonymity: 1, frequency: 17}
	//   {min_anonymity: 2, max_anonymity: 3, frequency: 42}
	//   {min_anonymity: 5, max_anonymity: 10, frequency: 99}
	// mean that there are no record with an estimated anonymity of 4, 5,
	// or
	// larger than 10.
	KMapEstimationHistogram []*GooglePrivacyDlpV2beta2KMapEstimationHistogramBucket `json:"kMapEstimationHistogram,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "KMapEstimationHistogram") to unconditionally include in API
	// requests. By default, fields with empty values are omitted from API
	// requests. However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "KMapEstimationHistogram")
	// to include in API requests with the JSON null value. By default,
	// fields with empty values are omitted from API requests. However, any
	// field with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2KMapEstimationResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2KMapEstimationResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Key: A unique identifier for a Datastore
// entity.
// If a key's partition ID or any of its path kinds or names
// are
// reserved/read-only, the key is reserved/read-only.
// A reserved/read-only key is forbidden in certain documented contexts.
type GooglePrivacyDlpV2beta2Key struct {
	// PartitionId: Entities are partitioned into subsets, currently
	// identified by a project
	// ID and namespace ID.
	// Queries are scoped to a single partition.
	PartitionId *GooglePrivacyDlpV2beta2PartitionId `json:"partitionId,omitempty"`

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
	Path []*GooglePrivacyDlpV2beta2PathElement `json:"path,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2Key) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Key
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2KindExpression: A representation of a
// Datastore kind.
type GooglePrivacyDlpV2beta2KindExpression struct {
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

func (s *GooglePrivacyDlpV2beta2KindExpression) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2KindExpression
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2KmsWrappedCryptoKey: Include to use an
// existing data crypto key wrapped by KMS.
// Authorization requires the following IAM permissions when sending a
// request
// to perform a crypto transformation using a kms-wrapped crypto
// key:
// dlp.kms.encrypt
type GooglePrivacyDlpV2beta2KmsWrappedCryptoKey struct {
	// CryptoKeyName: The resource name of the KMS CryptoKey to use for
	// unwrapping. [required]
	CryptoKeyName string `json:"cryptoKeyName,omitempty"`

	// WrappedKey: The wrapped data crypto key. [required]
	WrappedKey string `json:"wrappedKey,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CryptoKeyName") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CryptoKeyName") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2KmsWrappedCryptoKey) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2KmsWrappedCryptoKey
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2LDiversityConfig: l-diversity metric, used for
// analysis of reidentification risk.
type GooglePrivacyDlpV2beta2LDiversityConfig struct {
	// QuasiIds: Set of quasi-identifiers indicating how equivalence classes
	// are
	// defined for the l-diversity computation. When multiple fields
	// are
	// specified, they are considered a single composite key.
	QuasiIds []*GooglePrivacyDlpV2beta2FieldId `json:"quasiIds,omitempty"`

	// SensitiveAttribute: Sensitive field for computing the l-value.
	SensitiveAttribute *GooglePrivacyDlpV2beta2FieldId `json:"sensitiveAttribute,omitempty"`

	// ForceSendFields is a list of field names (e.g. "QuasiIds") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "QuasiIds") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2LDiversityConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2LDiversityConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2LDiversityEquivalenceClass: The set of
// columns' values that share the same ldiversity value.
type GooglePrivacyDlpV2beta2LDiversityEquivalenceClass struct {
	// EquivalenceClassSize: Size of the k-anonymity equivalence class.
	EquivalenceClassSize int64 `json:"equivalenceClassSize,omitempty,string"`

	// NumDistinctSensitiveValues: Number of distinct sensitive values in
	// this equivalence class.
	NumDistinctSensitiveValues int64 `json:"numDistinctSensitiveValues,omitempty,string"`

	// QuasiIdsValues: Quasi-identifier values defining the k-anonymity
	// equivalence
	// class. The order is always the same as the original request.
	QuasiIdsValues []*GooglePrivacyDlpV2beta2Value `json:"quasiIdsValues,omitempty"`

	// TopSensitiveValues: Estimated frequencies of top sensitive values.
	TopSensitiveValues []*GooglePrivacyDlpV2beta2ValueFrequency `json:"topSensitiveValues,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "EquivalenceClassSize") to unconditionally include in API requests.
	// By default, fields with empty values are omitted from API requests.
	// However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "EquivalenceClassSize") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2LDiversityEquivalenceClass) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2LDiversityEquivalenceClass
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type GooglePrivacyDlpV2beta2LDiversityHistogramBucket struct {
	// BucketSize: Total number of equivalence classes in this bucket.
	BucketSize int64 `json:"bucketSize,omitempty,string"`

	// BucketValueCount: Total number of distinct equivalence classes in
	// this bucket.
	BucketValueCount int64 `json:"bucketValueCount,omitempty,string"`

	// BucketValues: Sample of equivalence classes in this bucket. The total
	// number of
	// classes returned per bucket is capped at 20.
	BucketValues []*GooglePrivacyDlpV2beta2LDiversityEquivalenceClass `json:"bucketValues,omitempty"`

	// SensitiveValueFrequencyLowerBound: Lower bound on the sensitive value
	// frequencies of the equivalence
	// classes in this bucket.
	SensitiveValueFrequencyLowerBound int64 `json:"sensitiveValueFrequencyLowerBound,omitempty,string"`

	// SensitiveValueFrequencyUpperBound: Upper bound on the sensitive value
	// frequencies of the equivalence
	// classes in this bucket.
	SensitiveValueFrequencyUpperBound int64 `json:"sensitiveValueFrequencyUpperBound,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "BucketSize") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BucketSize") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2LDiversityHistogramBucket) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2LDiversityHistogramBucket
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2LDiversityResult: Result of the l-diversity
// computation.
type GooglePrivacyDlpV2beta2LDiversityResult struct {
	// SensitiveValueFrequencyHistogramBuckets: Histogram of l-diversity
	// equivalence class sensitive value frequencies.
	SensitiveValueFrequencyHistogramBuckets []*GooglePrivacyDlpV2beta2LDiversityHistogramBucket `json:"sensitiveValueFrequencyHistogramBuckets,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "SensitiveValueFrequencyHistogramBuckets") to unconditionally include
	// in API requests. By default, fields with empty values are omitted
	// from API requests. However, any non-pointer, non-interface field
	// appearing in ForceSendFields will be sent to the server regardless of
	// whether the field is empty or not. This may be used to include empty
	// fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g.
	// "SensitiveValueFrequencyHistogramBuckets") to include in API requests
	// with the JSON null value. By default, fields with empty values are
	// omitted from API requests. However, any field with an empty value
	// appearing in NullFields will be sent to the server as null. It is an
	// error if a field in this list has a non-empty value. This may be used
	// to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2LDiversityResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2LDiversityResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2LikelihoodAdjustment: Message for specifying
// an adjustment to the likelihood of a finding as
// part of a detection rule.
type GooglePrivacyDlpV2beta2LikelihoodAdjustment struct {
	// FixedLikelihood: Set the likelihood of a finding to a fixed value.
	//
	// Possible values:
	//   "LIKELIHOOD_UNSPECIFIED" - Default value; information with all
	// likelihoods is included.
	//   "VERY_UNLIKELY" - Few matching elements.
	//   "UNLIKELY"
	//   "POSSIBLE" - Some matching elements.
	//   "LIKELY"
	//   "VERY_LIKELY" - Many matching elements.
	FixedLikelihood string `json:"fixedLikelihood,omitempty"`

	// RelativeLikelihood: Increase or decrease the likelihood by the
	// specified number of
	// levels. For example, if a finding would be `POSSIBLE` without
	// the
	// detection rule and `relative_likelihood` is 1, then it is upgraded
	// to
	// `LIKELY`, while a value of -1 would downgrade it to
	// `UNLIKELY`.
	// Likelihood may never drop below `VERY_UNLIKELY` or
	// exceed
	// `VERY_LIKELY`, so applying an adjustment of 1 followed by
	// an
	// adjustment of -1 when base likelihood is `VERY_LIKELY` will result
	// in
	// a final likelihood of `LIKELY`.
	RelativeLikelihood int64 `json:"relativeLikelihood,omitempty"`

	// ForceSendFields is a list of field names (e.g. "FixedLikelihood") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "FixedLikelihood") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2LikelihoodAdjustment) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2LikelihoodAdjustment
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse: Response
// message for ListDeidentifyTemplates.
type GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse struct {
	// DeidentifyTemplates: List of deidentify templates, up to page_size
	// in
	// ListDeidentifyTemplatesRequest.
	DeidentifyTemplates []*GooglePrivacyDlpV2beta2DeidentifyTemplate `json:"deidentifyTemplates,omitempty"`

	// NextPageToken: If the next page is available then the next page token
	// to be used
	// in following ListDeidentifyTemplates request.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "DeidentifyTemplates")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DeidentifyTemplates") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2ListDlpJobsResponse: The response message for
// listing DLP jobs.
type GooglePrivacyDlpV2beta2ListDlpJobsResponse struct {
	// Jobs: A list of DlpJobs that matches the specified filter in the
	// request.
	Jobs []*GooglePrivacyDlpV2beta2DlpJob `json:"jobs,omitempty"`

	// NextPageToken: The standard List next-page token.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Jobs") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Jobs") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2ListDlpJobsResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2ListDlpJobsResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2ListInfoTypesResponse: Response to the
// ListInfoTypes request.
type GooglePrivacyDlpV2beta2ListInfoTypesResponse struct {
	// InfoTypes: Set of sensitive infoTypes.
	InfoTypes []*GooglePrivacyDlpV2beta2InfoTypeDescription `json:"infoTypes,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2ListInfoTypesResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2ListInfoTypesResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2ListInspectTemplatesResponse: Response message
// for ListInspectTemplates.
type GooglePrivacyDlpV2beta2ListInspectTemplatesResponse struct {
	// InspectTemplates: List of inspectTemplates, up to page_size in
	// ListInspectTemplatesRequest.
	InspectTemplates []*GooglePrivacyDlpV2beta2InspectTemplate `json:"inspectTemplates,omitempty"`

	// NextPageToken: If the next page is available then the next page token
	// to be used
	// in following ListInspectTemplates request.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "InspectTemplates") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "InspectTemplates") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2ListInspectTemplatesResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2ListInspectTemplatesResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2ListJobTriggersResponse: Response message for
// ListJobTriggers.
type GooglePrivacyDlpV2beta2ListJobTriggersResponse struct {
	// JobTriggers: List of triggeredJobs, up to page_size in
	// ListJobTriggersRequest.
	JobTriggers []*GooglePrivacyDlpV2beta2JobTrigger `json:"jobTriggers,omitempty"`

	// NextPageToken: If the next page is available then the next page token
	// to be used
	// in following ListJobTriggers request.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "JobTriggers") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "JobTriggers") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2ListJobTriggersResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2ListJobTriggersResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Location: Specifies the location of the
// finding.
type GooglePrivacyDlpV2beta2Location struct {
	// ByteRange: Zero-based byte offsets delimiting the finding.
	// These are relative to the finding's containing element.
	// Note that when the content is not textual, this references
	// the UTF-8 encoded textual representation of the content.
	// Omitted if content is an image.
	ByteRange *GooglePrivacyDlpV2beta2Range `json:"byteRange,omitempty"`

	// CodepointRange: Unicode character offsets delimiting the
	// finding.
	// These are relative to the finding's containing element.
	// Provided when the content is text.
	CodepointRange *GooglePrivacyDlpV2beta2Range `json:"codepointRange,omitempty"`

	// FieldId: The pointer to the property or cell that contained the
	// finding.
	// Provided when the finding's containing element is a cell in a
	// table
	// or a property of storage object.
	FieldId *GooglePrivacyDlpV2beta2FieldId `json:"fieldId,omitempty"`

	// ImageBoxes: The area within the image that contained the
	// finding.
	// Provided when the content is an image.
	ImageBoxes []*GooglePrivacyDlpV2beta2ImageLocation `json:"imageBoxes,omitempty"`

	// RecordKey: The pointer to the record in storage that contained the
	// field the
	// finding was found in.
	// Provided when the finding's containing element is a property
	// of a storage object.
	RecordKey *GooglePrivacyDlpV2beta2RecordKey `json:"recordKey,omitempty"`

	// TableLocation: The pointer to the row of the table that contained the
	// finding.
	// Provided when the finding's containing element is a cell of a table.
	TableLocation *GooglePrivacyDlpV2beta2TableLocation `json:"tableLocation,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2Location) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Location
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2NumericalStatsConfig: Compute numerical stats
// over an individual column, including
// min, max, and quantiles.
type GooglePrivacyDlpV2beta2NumericalStatsConfig struct {
	// Field: Field to compute numerical stats on. Supported types
	// are
	// integer, float, date, datetime, timestamp, time.
	Field *GooglePrivacyDlpV2beta2FieldId `json:"field,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Field") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Field") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2NumericalStatsConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2NumericalStatsConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2NumericalStatsResult: Result of the numerical
// stats computation.
type GooglePrivacyDlpV2beta2NumericalStatsResult struct {
	// MaxValue: Maximum value appearing in the column.
	MaxValue *GooglePrivacyDlpV2beta2Value `json:"maxValue,omitempty"`

	// MinValue: Minimum value appearing in the column.
	MinValue *GooglePrivacyDlpV2beta2Value `json:"minValue,omitempty"`

	// QuantileValues: List of 99 values that partition the set of field
	// values into 100 equal
	// sized buckets.
	QuantileValues []*GooglePrivacyDlpV2beta2Value `json:"quantileValues,omitempty"`

	// ForceSendFields is a list of field names (e.g. "MaxValue") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "MaxValue") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2NumericalStatsResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2NumericalStatsResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2OutputStorageConfig: Cloud repository for
// storing output.
type GooglePrivacyDlpV2beta2OutputStorageConfig struct {
	// OutputSchema: Schema used for writing the findings. Columns are
	// derived from the
	// `Finding` object. If appending to an existing table, any columns from
	// the
	// predefined schema that are missing will be added. No columns in
	// the
	// existing table will be deleted.
	//
	// If unspecified, then all available columns will be used for a new
	// table,
	// and no changes will be made to an existing table.
	//
	// Possible values:
	//   "OUTPUT_SCHEMA_UNSPECIFIED"
	//   "BASIC_COLUMNS" - Basic schema including only `info_type`, `quote`,
	// `certainty`, and
	// `timestamp`.
	//   "GCS_COLUMNS" - Schema tailored to findings from scanning Google
	// Cloud Storage.
	//   "DATASTORE_COLUMNS" - Schema tailored to findings from scanning
	// Google Datastore.
	//   "BIG_QUERY_COLUMNS" - Schema tailored to findings from scanning
	// Google BigQuery.
	//   "ALL_COLUMNS" - Schema containing all columns.
	OutputSchema string `json:"outputSchema,omitempty"`

	// Table: Store findings in an existing table or a new table in an
	// existing
	// dataset. Each column in an existing table must have the same name,
	// type,
	// and mode of a field in the `Finding` object. If table_id is not set a
	// new
	// one will be generated for you with the following
	// format:
	// dlp_googleapis_yyyy_mm_dd_[dlp_job_id]. Pacific timezone will be used
	// for
	// generating the date details.
	Table *GooglePrivacyDlpV2beta2BigQueryTable `json:"table,omitempty"`

	// ForceSendFields is a list of field names (e.g. "OutputSchema") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "OutputSchema") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2OutputStorageConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2OutputStorageConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2PartitionId: Datastore partition ID.
// A partition ID identifies a grouping of entities. The grouping is
// always
// by project and namespace, however the namespace ID may be empty.
//
// A partition ID contains several dimensions:
// project ID and namespace ID.
type GooglePrivacyDlpV2beta2PartitionId struct {
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

func (s *GooglePrivacyDlpV2beta2PartitionId) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2PartitionId
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2PathElement: A (kind, ID/name) pair used to
// construct a key path.
//
// If either name or ID is set, the element is complete.
// If neither is set, the element is incomplete.
type GooglePrivacyDlpV2beta2PathElement struct {
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

func (s *GooglePrivacyDlpV2beta2PathElement) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2PathElement
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2PrimitiveTransformation: A rule for
// transforming a value.
type GooglePrivacyDlpV2beta2PrimitiveTransformation struct {
	BucketingConfig *GooglePrivacyDlpV2beta2BucketingConfig `json:"bucketingConfig,omitempty"`

	CharacterMaskConfig *GooglePrivacyDlpV2beta2CharacterMaskConfig `json:"characterMaskConfig,omitempty"`

	CryptoHashConfig *GooglePrivacyDlpV2beta2CryptoHashConfig `json:"cryptoHashConfig,omitempty"`

	CryptoReplaceFfxFpeConfig *GooglePrivacyDlpV2beta2CryptoReplaceFfxFpeConfig `json:"cryptoReplaceFfxFpeConfig,omitempty"`

	DateShiftConfig *GooglePrivacyDlpV2beta2DateShiftConfig `json:"dateShiftConfig,omitempty"`

	FixedSizeBucketingConfig *GooglePrivacyDlpV2beta2FixedSizeBucketingConfig `json:"fixedSizeBucketingConfig,omitempty"`

	RedactConfig *GooglePrivacyDlpV2beta2RedactConfig `json:"redactConfig,omitempty"`

	ReplaceConfig *GooglePrivacyDlpV2beta2ReplaceValueConfig `json:"replaceConfig,omitempty"`

	ReplaceWithInfoTypeConfig *GooglePrivacyDlpV2beta2ReplaceWithInfoTypeConfig `json:"replaceWithInfoTypeConfig,omitempty"`

	TimePartConfig *GooglePrivacyDlpV2beta2TimePartConfig `json:"timePartConfig,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BucketingConfig") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BucketingConfig") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2PrimitiveTransformation) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2PrimitiveTransformation
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2PrivacyMetric: Privacy metric to compute for
// reidentification risk analysis.
type GooglePrivacyDlpV2beta2PrivacyMetric struct {
	CategoricalStatsConfig *GooglePrivacyDlpV2beta2CategoricalStatsConfig `json:"categoricalStatsConfig,omitempty"`

	KAnonymityConfig *GooglePrivacyDlpV2beta2KAnonymityConfig `json:"kAnonymityConfig,omitempty"`

	KMapEstimationConfig *GooglePrivacyDlpV2beta2KMapEstimationConfig `json:"kMapEstimationConfig,omitempty"`

	LDiversityConfig *GooglePrivacyDlpV2beta2LDiversityConfig `json:"lDiversityConfig,omitempty"`

	NumericalStatsConfig *GooglePrivacyDlpV2beta2NumericalStatsConfig `json:"numericalStatsConfig,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "CategoricalStatsConfig") to unconditionally include in API requests.
	// By default, fields with empty values are omitted from API requests.
	// However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CategoricalStatsConfig")
	// to include in API requests with the JSON null value. By default,
	// fields with empty values are omitted from API requests. However, any
	// field with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2PrivacyMetric) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2PrivacyMetric
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Proximity: Message for specifying a window
// around a finding to apply a detection
// rule.
type GooglePrivacyDlpV2beta2Proximity struct {
	// WindowAfter: Number of characters after the finding to consider.
	WindowAfter int64 `json:"windowAfter,omitempty"`

	// WindowBefore: Number of characters before the finding to consider.
	WindowBefore int64 `json:"windowBefore,omitempty"`

	// ForceSendFields is a list of field names (e.g. "WindowAfter") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "WindowAfter") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Proximity) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Proximity
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2PublishToPubSub: Publish the results of a
// DlpJob to a pub sub channel.
// Compatible with: Inpect, Risk
type GooglePrivacyDlpV2beta2PublishToPubSub struct {
	// Topic: Cloud Pub/Sub topic to send notifications to. The topic must
	// have given
	// publishing access rights to the DLP API service account executing
	// the long running DlpJob sending the notifications.
	// Format is projects/{project}/topics/{topic}.
	Topic string `json:"topic,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Topic") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Topic") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2PublishToPubSub) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2PublishToPubSub
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2QuasiIdField: A quasi-identifier column has a
// custom_tag, used to know which column
// in the data corresponds to which column in the statistical model.
type GooglePrivacyDlpV2beta2QuasiIdField struct {
	CustomTag string `json:"customTag,omitempty"`

	Field *GooglePrivacyDlpV2beta2FieldId `json:"field,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CustomTag") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CustomTag") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2QuasiIdField) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2QuasiIdField
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2QuoteInfo: Message for infoType-dependent
// details parsed from quote.
type GooglePrivacyDlpV2beta2QuoteInfo struct {
	DateTime *GooglePrivacyDlpV2beta2DateTime `json:"dateTime,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DateTime") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DateTime") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2QuoteInfo) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2QuoteInfo
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Range: Generic half-open interval [start, end)
type GooglePrivacyDlpV2beta2Range struct {
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

func (s *GooglePrivacyDlpV2beta2Range) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Range
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2RecordCondition: A condition for determining
// whether a transformation should be applied to
// a field.
type GooglePrivacyDlpV2beta2RecordCondition struct {
	// Expressions: An expression.
	Expressions *GooglePrivacyDlpV2beta2Expressions `json:"expressions,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Expressions") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Expressions") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2RecordCondition) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2RecordCondition
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2RecordKey: Message for a unique key indicating
// a record that contains a finding.
type GooglePrivacyDlpV2beta2RecordKey struct {
	BigQueryKey *GooglePrivacyDlpV2beta2BigQueryKey `json:"bigQueryKey,omitempty"`

	CloudStorageKey *GooglePrivacyDlpV2beta2CloudStorageKey `json:"cloudStorageKey,omitempty"`

	DatastoreKey *GooglePrivacyDlpV2beta2DatastoreKey `json:"datastoreKey,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BigQueryKey") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BigQueryKey") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2RecordKey) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2RecordKey
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2RecordSuppression: Configuration to suppress
// records whose suppression conditions evaluate to
// true.
type GooglePrivacyDlpV2beta2RecordSuppression struct {
	// Condition: A condition that when it evaluates to true will result in
	// the record being
	// evaluated to be suppressed from the transformed content.
	Condition *GooglePrivacyDlpV2beta2RecordCondition `json:"condition,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Condition") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Condition") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2RecordSuppression) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2RecordSuppression
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2RecordTransformations: A type of
// transformation that is applied over structured data such as a
// table.
type GooglePrivacyDlpV2beta2RecordTransformations struct {
	// FieldTransformations: Transform the record by applying various field
	// transformations.
	FieldTransformations []*GooglePrivacyDlpV2beta2FieldTransformation `json:"fieldTransformations,omitempty"`

	// RecordSuppressions: Configuration defining which records get
	// suppressed entirely. Records that
	// match any suppression rule are omitted from the output [optional].
	RecordSuppressions []*GooglePrivacyDlpV2beta2RecordSuppression `json:"recordSuppressions,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "FieldTransformations") to unconditionally include in API requests.
	// By default, fields with empty values are omitted from API requests.
	// However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "FieldTransformations") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2RecordTransformations) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2RecordTransformations
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2RedactConfig: Redact a given value. For
// example, if used with an `InfoTypeTransformation`
// transforming PHONE_NUMBER, and input 'My phone number is
// 206-555-0123', the
// output would be 'My phone number is '.
type GooglePrivacyDlpV2beta2RedactConfig struct {
}

// GooglePrivacyDlpV2beta2RedactImageRequest: Request to search for
// potentially sensitive info in a list of items
// and replace it with a default or provided content.
type GooglePrivacyDlpV2beta2RedactImageRequest struct {
	// ImageData: The bytes of the image to redact.
	ImageData string `json:"imageData,omitempty"`

	// ImageRedactionConfigs: The configuration for specifying what content
	// to redact from images.
	ImageRedactionConfigs []*GooglePrivacyDlpV2beta2ImageRedactionConfig `json:"imageRedactionConfigs,omitempty"`

	// ImageType: Type of the content, as defined in Content-Type HTTP
	// header.
	// Supported types are: PNG, JPEG, SVG, & BMP.
	ImageType string `json:"imageType,omitempty"`

	// InspectConfig: Configuration for the inspector.
	InspectConfig *GooglePrivacyDlpV2beta2InspectConfig `json:"inspectConfig,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ImageData") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ImageData") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2RedactImageRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2RedactImageRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2RedactImageResponse: Results of redacting an
// image.
type GooglePrivacyDlpV2beta2RedactImageResponse struct {
	// ExtractedText: If an image was being inspected and the
	// InspectConfig's include_quote was
	// set to true, then this field will include all text, if any, that was
	// found
	// in the image.
	ExtractedText string `json:"extractedText,omitempty"`

	// RedactedImage: The redacted image. The type will be the same as the
	// original image.
	RedactedImage string `json:"redactedImage,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "ExtractedText") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ExtractedText") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2RedactImageResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2RedactImageResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Regex: Message defining a custom regular
// expression.
type GooglePrivacyDlpV2beta2Regex struct {
	// Pattern: Pattern defining the regular expression.
	Pattern string `json:"pattern,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Pattern") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Pattern") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Regex) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Regex
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2ReidentifyContentRequest: Request to
// re-identify an item.
type GooglePrivacyDlpV2beta2ReidentifyContentRequest struct {
	// InspectConfig: Configuration for the inspector.
	InspectConfig *GooglePrivacyDlpV2beta2InspectConfig `json:"inspectConfig,omitempty"`

	// InspectTemplateName: Optional template to use. Any configuration
	// directly specified in
	// `inspect_config` will override those set in the template. Singular
	// fields
	// that are set in this request will replace their corresponding fields
	// in the
	// template. Repeated fields are appended. Singular sub-messages and
	// groups
	// are recursively merged.
	InspectTemplateName string `json:"inspectTemplateName,omitempty"`

	// Item: The item to re-identify. Will be treated as text.
	Item *GooglePrivacyDlpV2beta2ContentItem `json:"item,omitempty"`

	// ReidentifyConfig: Configuration for the re-identification of the
	// content item.
	// This field shares the same proto message type that is used
	// for
	// de-identification, however its usage here is for the reversal of
	// the
	// previous de-identification. Re-identification is performed by
	// examining
	// the transformations used to de-identify the items and executing
	// the
	// reverse. This requires that only reversible transformations
	// be provided here. The reversible transformations are:
	//
	//  - `CryptoReplaceFfxFpeConfig`
	ReidentifyConfig *GooglePrivacyDlpV2beta2DeidentifyConfig `json:"reidentifyConfig,omitempty"`

	// ReidentifyTemplateName: Optional template to use. References an
	// instance of `DeidentifyTemplate`.
	// Any configuration directly specified in `reidentify_config`
	// or
	// `inspect_config` will override those set in the template. Singular
	// fields
	// that are set in this request will replace their corresponding fields
	// in the
	// template. Repeated fields are appended. Singular sub-messages and
	// groups
	// are recursively merged.
	ReidentifyTemplateName string `json:"reidentifyTemplateName,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2ReidentifyContentRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2ReidentifyContentRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2ReidentifyContentResponse: Results of
// re-identifying a item.
type GooglePrivacyDlpV2beta2ReidentifyContentResponse struct {
	// Item: The re-identified item.
	Item *GooglePrivacyDlpV2beta2ContentItem `json:"item,omitempty"`

	// Overview: An overview of the changes that were made to the `item`.
	Overview *GooglePrivacyDlpV2beta2TransformationOverview `json:"overview,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Item") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Item") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2ReidentifyContentResponse) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2ReidentifyContentResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2ReplaceValueConfig: Replace each input value
// with a given `Value`.
type GooglePrivacyDlpV2beta2ReplaceValueConfig struct {
	// NewValue: Value to replace it with.
	NewValue *GooglePrivacyDlpV2beta2Value `json:"newValue,omitempty"`

	// ForceSendFields is a list of field names (e.g. "NewValue") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "NewValue") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2ReplaceValueConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2ReplaceValueConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2ReplaceWithInfoTypeConfig: Replace each
// matching finding with the name of the info_type.
type GooglePrivacyDlpV2beta2ReplaceWithInfoTypeConfig struct {
}

type GooglePrivacyDlpV2beta2RequestedOptions struct {
	JobConfig *GooglePrivacyDlpV2beta2InspectJobConfig `json:"jobConfig,omitempty"`

	// SnapshotInspectTemplate: If run with an inspect template, a snapshot
	// of it's state at the time of
	// this run.
	SnapshotInspectTemplate *GooglePrivacyDlpV2beta2InspectTemplate `json:"snapshotInspectTemplate,omitempty"`

	// ForceSendFields is a list of field names (e.g. "JobConfig") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "JobConfig") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2RequestedOptions) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2RequestedOptions
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type GooglePrivacyDlpV2beta2Result struct {
	// InfoTypeStats: Statistics of how many instances of each info type
	// were found during
	// inspect job.
	InfoTypeStats []*GooglePrivacyDlpV2beta2InfoTypeStatistics `json:"infoTypeStats,omitempty"`

	// ProcessedBytes: Total size in bytes that were processed.
	ProcessedBytes int64 `json:"processedBytes,omitempty,string"`

	// TotalEstimatedBytes: Estimate of the number of bytes to process.
	TotalEstimatedBytes int64 `json:"totalEstimatedBytes,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "InfoTypeStats") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "InfoTypeStats") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Result) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Result
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2RiskAnalysisJobConfig: Configuration for a
// risk analysis job.
type GooglePrivacyDlpV2beta2RiskAnalysisJobConfig struct {
	// Actions: Actions to execute at the completion of the job. Are
	// executed in the order
	// provided.
	Actions []*GooglePrivacyDlpV2beta2Action `json:"actions,omitempty"`

	// PrivacyMetric: Privacy metric to compute.
	PrivacyMetric *GooglePrivacyDlpV2beta2PrivacyMetric `json:"privacyMetric,omitempty"`

	// SourceTable: Input dataset to compute metrics over.
	SourceTable *GooglePrivacyDlpV2beta2BigQueryTable `json:"sourceTable,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Actions") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Actions") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2RiskAnalysisJobConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2RiskAnalysisJobConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type GooglePrivacyDlpV2beta2Row struct {
	Values []*GooglePrivacyDlpV2beta2Value `json:"values,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Values") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Values") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Row) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Row
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2SaveFindings: If set, the detailed findings
// will be persisted to the specified
// OutputStorageConfig. Compatible with: Inspect
type GooglePrivacyDlpV2beta2SaveFindings struct {
	OutputConfig *GooglePrivacyDlpV2beta2OutputStorageConfig `json:"outputConfig,omitempty"`

	// ForceSendFields is a list of field names (e.g. "OutputConfig") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "OutputConfig") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2SaveFindings) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2SaveFindings
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Schedule: Schedule for triggeredJobs.
type GooglePrivacyDlpV2beta2Schedule struct {
	// ReccurrencePeriodDuration: With this option a job is started a
	// regular periodic basis. For
	// example: every 10 minutes.
	//
	// A scheduled start time will be skipped if the previous
	// execution has not ended when its scheduled time occurs.
	//
	// This value must be set to a time duration greater than or equal
	// to 60 minutes and can be no longer than 60 days.
	ReccurrencePeriodDuration string `json:"reccurrencePeriodDuration,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "ReccurrencePeriodDuration") to unconditionally include in API
	// requests. By default, fields with empty values are omitted from API
	// requests. However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g.
	// "ReccurrencePeriodDuration") to include in API requests with the JSON
	// null value. By default, fields with empty values are omitted from API
	// requests. However, any field with an empty value appearing in
	// NullFields will be sent to the server as null. It is an error if a
	// field in this list has a non-empty value. This may be used to include
	// null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Schedule) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Schedule
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2StorageConfig: Shared message indicating Cloud
// storage type.
type GooglePrivacyDlpV2beta2StorageConfig struct {
	// BigQueryOptions: BigQuery options specification.
	BigQueryOptions *GooglePrivacyDlpV2beta2BigQueryOptions `json:"bigQueryOptions,omitempty"`

	// CloudStorageOptions: Google Cloud Storage options specification.
	CloudStorageOptions *GooglePrivacyDlpV2beta2CloudStorageOptions `json:"cloudStorageOptions,omitempty"`

	// DatastoreOptions: Google Cloud Datastore options specification.
	DatastoreOptions *GooglePrivacyDlpV2beta2DatastoreOptions `json:"datastoreOptions,omitempty"`

	TimespanConfig *GooglePrivacyDlpV2beta2TimespanConfig `json:"timespanConfig,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BigQueryOptions") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BigQueryOptions") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2StorageConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2StorageConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2SummaryResult: A collection that informs the
// user the number of times a particular
// `TransformationResultCode` and error details occurred.
type GooglePrivacyDlpV2beta2SummaryResult struct {
	// Possible values:
	//   "TRANSFORMATION_RESULT_CODE_UNSPECIFIED"
	//   "SUCCESS"
	//   "ERROR"
	Code string `json:"code,omitempty"`

	Count int64 `json:"count,omitempty,string"`

	// Details: A place for warnings or errors to show up if a
	// transformation didn't
	// work as expected.
	Details string `json:"details,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2SummaryResult) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2SummaryResult
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2SurrogateType: Message for detecting output
// from deidentification transformations
// such
// as
// [`CryptoReplaceFfxFpeConfig`](/dlp/docs/reference/rest/v2beta1/cont
// ent/deidentify#CryptoReplaceFfxFpeConfig).
// These types of transformations are
// those that perform pseudonymization, thereby producing a "surrogate"
// as
// output. This should be used in conjunction with a field on
// the
// transformation such as `surrogate_info_type`. This custom info type
// does
// not support the use of `detection_rules`.
type GooglePrivacyDlpV2beta2SurrogateType struct {
}

// GooglePrivacyDlpV2beta2Table: Structured content to inspect. Up to
// 50,000 `Value`s per request allowed.
type GooglePrivacyDlpV2beta2Table struct {
	Headers []*GooglePrivacyDlpV2beta2FieldId `json:"headers,omitempty"`

	Rows []*GooglePrivacyDlpV2beta2Row `json:"rows,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Headers") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Headers") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Table) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Table
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2TableLocation: Location of a finding within a
// table.
type GooglePrivacyDlpV2beta2TableLocation struct {
	// RowIndex: The zero-based index of the row where the finding is
	// located.
	RowIndex int64 `json:"rowIndex,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "RowIndex") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "RowIndex") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2TableLocation) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2TableLocation
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2TaggedField: A column with a semantic tag
// attached.
type GooglePrivacyDlpV2beta2TaggedField struct {
	// CustomTag: A column can be tagged with a custom tag. In this case,
	// the user must
	// indicate an auxiliary table that contains statistical information
	// on
	// the possible values of this column (below).
	CustomTag string `json:"customTag,omitempty"`

	// Field: Identifies the column. [required]
	Field *GooglePrivacyDlpV2beta2FieldId `json:"field,omitempty"`

	// Inferred: If no semantic tag is indicated, we infer the statistical
	// model from
	// the distribution of values in the input data
	Inferred *GoogleProtobufEmpty `json:"inferred,omitempty"`

	// InfoType: A column can be tagged with a InfoType to use the relevant
	// public
	// dataset as a statistical model of population, if available.
	// We
	// currently support US ZIP codes, region codes, ages and genders.
	// To programmatically obtain the list of supported InfoTypes,
	// use
	// ListInfoTypes with the supported_by=RISK_ANALYSIS filter.
	InfoType *GooglePrivacyDlpV2beta2InfoType `json:"infoType,omitempty"`

	// ForceSendFields is a list of field names (e.g. "CustomTag") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "CustomTag") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2TaggedField) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2TaggedField
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2TimePartConfig: For use with `Date`,
// `Timestamp`, and `TimeOfDay`, extract or preserve a
// portion of the value.
type GooglePrivacyDlpV2beta2TimePartConfig struct {
	// Possible values:
	//   "TIME_PART_UNSPECIFIED"
	//   "YEAR" - [0-9999]
	//   "MONTH" - [1-12]
	//   "DAY_OF_MONTH" - [1-31]
	//   "DAY_OF_WEEK" - [1-7]
	//   "WEEK_OF_YEAR" - [1-52]
	//   "HOUR_OF_DAY" - [0-23]
	PartToExtract string `json:"partToExtract,omitempty"`

	// ForceSendFields is a list of field names (e.g. "PartToExtract") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "PartToExtract") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2TimePartConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2TimePartConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

type GooglePrivacyDlpV2beta2TimeZone struct {
	// OffsetMinutes: Set only if the offset can be determined. Positive for
	// time ahead of UTC.
	// E.g. For "UTC-9", this value is -540.
	OffsetMinutes int64 `json:"offsetMinutes,omitempty"`

	// ForceSendFields is a list of field names (e.g. "OffsetMinutes") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "OffsetMinutes") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2TimeZone) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2TimeZone
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2TimespanConfig: Configuration of the timespan
// of the items to include in scanning.
// Currently only supported when inspecting Google Cloud Storage and
// BigQuery.
type GooglePrivacyDlpV2beta2TimespanConfig struct {
	// EnableAutoPopulationOfTimespanConfig: When the job is started by a
	// JobTrigger we will automatically figure out
	// a valid start_time to avoid scanning files that have not been
	// modified
	// since the last time the JobTrigger executed. This will be based on
	// the
	// time of the execution of the last run of the JobTrigger.
	EnableAutoPopulationOfTimespanConfig bool `json:"enableAutoPopulationOfTimespanConfig,omitempty"`

	// EndTime: Exclude files newer than this value.
	// If set to zero, no upper time limit is applied.
	EndTime string `json:"endTime,omitempty"`

	// StartTime: Exclude files older than this value.
	StartTime string `json:"startTime,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "EnableAutoPopulationOfTimespanConfig") to unconditionally include in
	// API requests. By default, fields with empty values are omitted from
	// API requests. However, any non-pointer, non-interface field appearing
	// in ForceSendFields will be sent to the server regardless of whether
	// the field is empty or not. This may be used to include empty fields
	// in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g.
	// "EnableAutoPopulationOfTimespanConfig") to include in API requests
	// with the JSON null value. By default, fields with empty values are
	// omitted from API requests. However, any field with an empty value
	// appearing in NullFields will be sent to the server as null. It is an
	// error if a field in this list has a non-empty value. This may be used
	// to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2TimespanConfig) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2TimespanConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2TransformationOverview: Overview of the
// modifications that occurred.
type GooglePrivacyDlpV2beta2TransformationOverview struct {
	// TransformationSummaries: Transformations applied to the dataset.
	TransformationSummaries []*GooglePrivacyDlpV2beta2TransformationSummary `json:"transformationSummaries,omitempty"`

	// TransformedBytes: Total size in bytes that were transformed in some
	// way.
	TransformedBytes int64 `json:"transformedBytes,omitempty,string"`

	// ForceSendFields is a list of field names (e.g.
	// "TransformationSummaries") to unconditionally include in API
	// requests. By default, fields with empty values are omitted from API
	// requests. However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "TransformationSummaries")
	// to include in API requests with the JSON null value. By default,
	// fields with empty values are omitted from API requests. However, any
	// field with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2TransformationOverview) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2TransformationOverview
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2TransformationSummary: Summary of a single
// tranformation.
// Only one of 'transformation', 'field_transformation', or
// 'record_suppress'
// will be set.
type GooglePrivacyDlpV2beta2TransformationSummary struct {
	// Field: Set if the transformation was limited to a specific FieldId.
	Field *GooglePrivacyDlpV2beta2FieldId `json:"field,omitempty"`

	// FieldTransformations: The field transformation that was applied.
	// If multiple field transformations are requested for a single
	// field,
	// this list will contain all of them; otherwise, only one is supplied.
	FieldTransformations []*GooglePrivacyDlpV2beta2FieldTransformation `json:"fieldTransformations,omitempty"`

	// InfoType: Set if the transformation was limited to a specific
	// info_type.
	InfoType *GooglePrivacyDlpV2beta2InfoType `json:"infoType,omitempty"`

	// RecordSuppress: The specific suppression option these stats apply to.
	RecordSuppress *GooglePrivacyDlpV2beta2RecordSuppression `json:"recordSuppress,omitempty"`

	Results []*GooglePrivacyDlpV2beta2SummaryResult `json:"results,omitempty"`

	// Transformation: The specific transformation these stats apply to.
	Transformation *GooglePrivacyDlpV2beta2PrimitiveTransformation `json:"transformation,omitempty"`

	// TransformedBytes: Total size in bytes that were transformed in some
	// way.
	TransformedBytes int64 `json:"transformedBytes,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "Field") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Field") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2TransformationSummary) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2TransformationSummary
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2TransientCryptoKey: Use this to have a random
// data crypto key generated.
// It will be discarded after the request finishes.
type GooglePrivacyDlpV2beta2TransientCryptoKey struct {
	// Name: Name of the key. [required]
	// This is an arbitrary string used to differentiate different keys.
	// A unique key is generated per name: two separate
	// `TransientCryptoKey`
	// protos share the same generated key if their names are the same.
	// When the data crypto key is generated, this name is not used in any
	// way
	// (repeating the api call will result in a different key being
	// generated).
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

func (s *GooglePrivacyDlpV2beta2TransientCryptoKey) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2TransientCryptoKey
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Trigger: What event needs to occur for a new
// job to be started.
type GooglePrivacyDlpV2beta2Trigger struct {
	// Schedule: Create a job on a repeating basis based on the elapse of
	// time.
	Schedule *GooglePrivacyDlpV2beta2Schedule `json:"schedule,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Schedule") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Schedule") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Trigger) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Trigger
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2UnwrappedCryptoKey: Using raw keys is prone to
// security risks due to accidentally
// leaking the key. Choose another type of key if possible.
type GooglePrivacyDlpV2beta2UnwrappedCryptoKey struct {
	// Key: The AES 128/192/256 bit key. [required]
	Key string `json:"key,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Key") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Key") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2UnwrappedCryptoKey) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2UnwrappedCryptoKey
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2UpdateDeidentifyTemplateRequest: Request
// message for UpdateDeidentifyTemplate.
type GooglePrivacyDlpV2beta2UpdateDeidentifyTemplateRequest struct {
	// DeidentifyTemplate: New DeidentifyTemplate value.
	DeidentifyTemplate *GooglePrivacyDlpV2beta2DeidentifyTemplate `json:"deidentifyTemplate,omitempty"`

	// UpdateMask: Mask to control which fields get updated.
	UpdateMask string `json:"updateMask,omitempty"`

	// ForceSendFields is a list of field names (e.g. "DeidentifyTemplate")
	// to unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DeidentifyTemplate") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2UpdateDeidentifyTemplateRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2UpdateDeidentifyTemplateRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2UpdateInspectTemplateRequest: Request message
// for UpdateInspectTemplate.
type GooglePrivacyDlpV2beta2UpdateInspectTemplateRequest struct {
	// InspectTemplate: New InspectTemplate value.
	InspectTemplate *GooglePrivacyDlpV2beta2InspectTemplate `json:"inspectTemplate,omitempty"`

	// UpdateMask: Mask to control which fields get updated.
	UpdateMask string `json:"updateMask,omitempty"`

	// ForceSendFields is a list of field names (e.g. "InspectTemplate") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "InspectTemplate") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2UpdateInspectTemplateRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2UpdateInspectTemplateRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2UpdateJobTriggerRequest: Request message for
// UpdateJobTrigger.
type GooglePrivacyDlpV2beta2UpdateJobTriggerRequest struct {
	// JobTrigger: New JobTrigger value.
	JobTrigger *GooglePrivacyDlpV2beta2JobTrigger `json:"jobTrigger,omitempty"`

	// UpdateMask: Mask to control which fields get updated.
	UpdateMask string `json:"updateMask,omitempty"`

	// ForceSendFields is a list of field names (e.g. "JobTrigger") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "JobTrigger") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2UpdateJobTriggerRequest) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2UpdateJobTriggerRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2Value: Set of primitive values supported by
// the system.
// Note that for the purposes of inspection or transformation, the
// number
// of bytes considered to comprise a 'Value' is based on its
// representation
// as a UTF-8 encoded string. For example, if 'integer_value' is set
// to
// 123456789, the number of bytes would be counted as 9, even though
// an
// int64 only holds up to 8 bytes of data.
type GooglePrivacyDlpV2beta2Value struct {
	BooleanValue bool `json:"booleanValue,omitempty"`

	DateValue *GoogleTypeDate `json:"dateValue,omitempty"`

	// Possible values:
	//   "DAY_OF_WEEK_UNSPECIFIED" - The unspecified day-of-week.
	//   "MONDAY" - The day-of-week of Monday.
	//   "TUESDAY" - The day-of-week of Tuesday.
	//   "WEDNESDAY" - The day-of-week of Wednesday.
	//   "THURSDAY" - The day-of-week of Thursday.
	//   "FRIDAY" - The day-of-week of Friday.
	//   "SATURDAY" - The day-of-week of Saturday.
	//   "SUNDAY" - The day-of-week of Sunday.
	DayOfWeekValue string `json:"dayOfWeekValue,omitempty"`

	FloatValue float64 `json:"floatValue,omitempty"`

	IntegerValue int64 `json:"integerValue,omitempty,string"`

	StringValue string `json:"stringValue,omitempty"`

	TimeValue *GoogleTypeTimeOfDay `json:"timeValue,omitempty"`

	TimestampValue string `json:"timestampValue,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BooleanValue") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BooleanValue") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2Value) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2Value
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

func (s *GooglePrivacyDlpV2beta2Value) UnmarshalJSON(data []byte) error {
	type NoMethod GooglePrivacyDlpV2beta2Value
	var s1 struct {
		FloatValue gensupport.JSONFloat64 `json:"floatValue"`
		*NoMethod
	}
	s1.NoMethod = (*NoMethod)(s)
	if err := json.Unmarshal(data, &s1); err != nil {
		return err
	}
	s.FloatValue = float64(s1.FloatValue)
	return nil
}

// GooglePrivacyDlpV2beta2ValueFrequency: A value of a field, including
// its frequency.
type GooglePrivacyDlpV2beta2ValueFrequency struct {
	// Count: How many times the value is contained in the field.
	Count int64 `json:"count,omitempty,string"`

	// Value: A value contained in the field in question.
	Value *GooglePrivacyDlpV2beta2Value `json:"value,omitempty"`

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

func (s *GooglePrivacyDlpV2beta2ValueFrequency) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2ValueFrequency
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GooglePrivacyDlpV2beta2WordList: Message defining a list of words or
// phrases to search for in the data.
type GooglePrivacyDlpV2beta2WordList struct {
	// Words: Words or phrases defining the dictionary. The dictionary must
	// contain
	// at least one phrase and every phrase must contain at least 2
	// characters
	// that are letters or digits. [required]
	Words []string `json:"words,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Words") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Words") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GooglePrivacyDlpV2beta2WordList) MarshalJSON() ([]byte, error) {
	type NoMethod GooglePrivacyDlpV2beta2WordList
	raw := NoMethod(*s)
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

func (s *GoogleRpcStatus) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleRpcStatus
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleTypeDate: Represents a whole calendar date, e.g. date of birth.
// The time of day and
// time zone are either specified elsewhere or are not significant. The
// date
// is relative to the Proleptic Gregorian Calendar. The day may be 0
// to
// represent a year and month where the day is not significant, e.g.
// credit card
// expiration date. The year may be 0 to represent a month and day
// independent
// of year, e.g. anniversary date. Related types are
// google.type.TimeOfDay
// and `google.protobuf.Timestamp`.
type GoogleTypeDate struct {
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

func (s *GoogleTypeDate) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleTypeDate
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GoogleTypeTimeOfDay: Represents a time of day. The date and time zone
// are either not significant
// or are specified elsewhere. An API may choose to allow leap seconds.
// Related
// types are google.type.Date and `google.protobuf.Timestamp`.
type GoogleTypeTimeOfDay struct {
	// Hours: Hours of day in 24 hour format. Should be from 0 to 23. An API
	// may choose
	// to allow the value "24:00:00" for scenarios like business closing
	// time.
	Hours int64 `json:"hours,omitempty"`

	// Minutes: Minutes of hour of day. Must be from 0 to 59.
	Minutes int64 `json:"minutes,omitempty"`

	// Nanos: Fractions of seconds in nanoseconds. Must be from 0 to
	// 999,999,999.
	Nanos int64 `json:"nanos,omitempty"`

	// Seconds: Seconds of minutes of the time. Must normally be from 0 to
	// 59. An API may
	// allow the value 60 if it allows leap-seconds.
	Seconds int64 `json:"seconds,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Hours") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Hours") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *GoogleTypeTimeOfDay) MarshalJSON() ([]byte, error) {
	type NoMethod GoogleTypeTimeOfDay
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// method id "dlp.infoTypes.list":

type InfoTypesListCall struct {
	s            *Service
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Returns sensitive information types DLP supports.
func (r *InfoTypesService) List() *InfoTypesListCall {
	c := &InfoTypesListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	return c
}

// Filter sets the optional parameter "filter": Optional filter to only
// return infoTypes supported by certain parts of the
// API. Defaults to supported_by=INSPECT.
func (c *InfoTypesListCall) Filter(filter string) *InfoTypesListCall {
	c.urlParams_.Set("filter", filter)
	return c
}

// LanguageCode sets the optional parameter "languageCode": Optional
// BCP-47 language code for localized infoType friendly
// names. If omitted, or if localized strings are not available,
// en-US strings will be returned.
func (c *InfoTypesListCall) LanguageCode(languageCode string) *InfoTypesListCall {
	c.urlParams_.Set("languageCode", languageCode)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *InfoTypesListCall) Fields(s ...googleapi.Field) *InfoTypesListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *InfoTypesListCall) IfNoneMatch(entityTag string) *InfoTypesListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *InfoTypesListCall) Context(ctx context.Context) *InfoTypesListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *InfoTypesListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *InfoTypesListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/infoTypes")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.infoTypes.list" call.
// Exactly one of *GooglePrivacyDlpV2beta2ListInfoTypesResponse or error
// will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2ListInfoTypesResponse.ServerResponse.Header
// or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *InfoTypesListCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2ListInfoTypesResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta2ListInfoTypesResponse{
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
	//   "description": "Returns sensitive information types DLP supports.",
	//   "flatPath": "v2beta2/infoTypes",
	//   "httpMethod": "GET",
	//   "id": "dlp.infoTypes.list",
	//   "parameterOrder": [],
	//   "parameters": {
	//     "filter": {
	//       "description": "Optional filter to only return infoTypes supported by certain parts of the\nAPI. Defaults to supported_by=INSPECT.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "languageCode": {
	//       "description": "Optional BCP-47 language code for localized infoType friendly\nnames. If omitted, or if localized strings are not available,\nen-US strings will be returned.",
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/infoTypes",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2ListInfoTypesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.organizations.deidentifyTemplates.create":

type OrganizationsDeidentifyTemplatesCreateCall struct {
	s                                                      *Service
	parent                                                 string
	googleprivacydlpv2beta2createdeidentifytemplaterequest *GooglePrivacyDlpV2beta2CreateDeidentifyTemplateRequest
	urlParams_                                             gensupport.URLParams
	ctx_                                                   context.Context
	header_                                                http.Header
}

// Create: Creates an Deidentify template for re-using frequently used
// configuration
// for Deidentifying content, images, and storage.
func (r *OrganizationsDeidentifyTemplatesService) Create(parent string, googleprivacydlpv2beta2createdeidentifytemplaterequest *GooglePrivacyDlpV2beta2CreateDeidentifyTemplateRequest) *OrganizationsDeidentifyTemplatesCreateCall {
	c := &OrganizationsDeidentifyTemplatesCreateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.googleprivacydlpv2beta2createdeidentifytemplaterequest = googleprivacydlpv2beta2createdeidentifytemplaterequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *OrganizationsDeidentifyTemplatesCreateCall) Fields(s ...googleapi.Field) *OrganizationsDeidentifyTemplatesCreateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *OrganizationsDeidentifyTemplatesCreateCall) Context(ctx context.Context) *OrganizationsDeidentifyTemplatesCreateCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *OrganizationsDeidentifyTemplatesCreateCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *OrganizationsDeidentifyTemplatesCreateCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2createdeidentifytemplaterequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/deidentifyTemplates")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.organizations.deidentifyTemplates.create" call.
// Exactly one of *GooglePrivacyDlpV2beta2DeidentifyTemplate or error
// will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2DeidentifyTemplate.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *OrganizationsDeidentifyTemplatesCreateCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2DeidentifyTemplate, error) {
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
	ret := &GooglePrivacyDlpV2beta2DeidentifyTemplate{
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
	//   "description": "Creates an Deidentify template for re-using frequently used configuration\nfor Deidentifying content, images, and storage.",
	//   "flatPath": "v2beta2/organizations/{organizationsId}/deidentifyTemplates",
	//   "httpMethod": "POST",
	//   "id": "dlp.organizations.deidentifyTemplates.create",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id or\norganizations/my-org-id.",
	//       "location": "path",
	//       "pattern": "^organizations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/deidentifyTemplates",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2CreateDeidentifyTemplateRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2DeidentifyTemplate"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.organizations.deidentifyTemplates.delete":

type OrganizationsDeidentifyTemplatesDeleteCall struct {
	s          *Service
	name       string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
	header_    http.Header
}

// Delete: Deletes inspect templates.
func (r *OrganizationsDeidentifyTemplatesService) Delete(name string) *OrganizationsDeidentifyTemplatesDeleteCall {
	c := &OrganizationsDeidentifyTemplatesDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *OrganizationsDeidentifyTemplatesDeleteCall) Fields(s ...googleapi.Field) *OrganizationsDeidentifyTemplatesDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *OrganizationsDeidentifyTemplatesDeleteCall) Context(ctx context.Context) *OrganizationsDeidentifyTemplatesDeleteCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *OrganizationsDeidentifyTemplatesDeleteCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *OrganizationsDeidentifyTemplatesDeleteCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("DELETE", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.organizations.deidentifyTemplates.delete" call.
// Exactly one of *GoogleProtobufEmpty or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *GoogleProtobufEmpty.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *OrganizationsDeidentifyTemplatesDeleteCall) Do(opts ...googleapi.CallOption) (*GoogleProtobufEmpty, error) {
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
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Deletes inspect templates.",
	//   "flatPath": "v2beta2/organizations/{organizationsId}/deidentifyTemplates/{deidentifyTemplatesId}",
	//   "httpMethod": "DELETE",
	//   "id": "dlp.organizations.deidentifyTemplates.delete",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of the organization and deidentify template to be deleted,\nfor example `organizations/433245324/deidentifyTemplates/432452342` or\nprojects/project-id/deidentifyTemplates/432452342.",
	//       "location": "path",
	//       "pattern": "^organizations/[^/]+/deidentifyTemplates/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "response": {
	//     "$ref": "GoogleProtobufEmpty"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.organizations.deidentifyTemplates.get":

type OrganizationsDeidentifyTemplatesGetCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// Get: Gets an inspect template.
func (r *OrganizationsDeidentifyTemplatesService) Get(name string) *OrganizationsDeidentifyTemplatesGetCall {
	c := &OrganizationsDeidentifyTemplatesGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *OrganizationsDeidentifyTemplatesGetCall) Fields(s ...googleapi.Field) *OrganizationsDeidentifyTemplatesGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *OrganizationsDeidentifyTemplatesGetCall) IfNoneMatch(entityTag string) *OrganizationsDeidentifyTemplatesGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *OrganizationsDeidentifyTemplatesGetCall) Context(ctx context.Context) *OrganizationsDeidentifyTemplatesGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *OrganizationsDeidentifyTemplatesGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *OrganizationsDeidentifyTemplatesGetCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.organizations.deidentifyTemplates.get" call.
// Exactly one of *GooglePrivacyDlpV2beta2DeidentifyTemplate or error
// will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2DeidentifyTemplate.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *OrganizationsDeidentifyTemplatesGetCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2DeidentifyTemplate, error) {
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
	ret := &GooglePrivacyDlpV2beta2DeidentifyTemplate{
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
	//   "description": "Gets an inspect template.",
	//   "flatPath": "v2beta2/organizations/{organizationsId}/deidentifyTemplates/{deidentifyTemplatesId}",
	//   "httpMethod": "GET",
	//   "id": "dlp.organizations.deidentifyTemplates.get",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of the organization and deidentify template to be read, for\nexample `organizations/433245324/deidentifyTemplates/432452342` or\nprojects/project-id/deidentifyTemplates/432452342.",
	//       "location": "path",
	//       "pattern": "^organizations/[^/]+/deidentifyTemplates/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2DeidentifyTemplate"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.organizations.deidentifyTemplates.list":

type OrganizationsDeidentifyTemplatesListCall struct {
	s            *Service
	parent       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists inspect templates.
func (r *OrganizationsDeidentifyTemplatesService) List(parent string) *OrganizationsDeidentifyTemplatesListCall {
	c := &OrganizationsDeidentifyTemplatesListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	return c
}

// PageSize sets the optional parameter "pageSize": Optional size of the
// page, can be limited by server. If zero server returns
// a page of max size 100.
func (c *OrganizationsDeidentifyTemplatesListCall) PageSize(pageSize int64) *OrganizationsDeidentifyTemplatesListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": Optional page
// token to continue retrieval. Comes from previous call
// to `ListDeidentifyTemplates`.
func (c *OrganizationsDeidentifyTemplatesListCall) PageToken(pageToken string) *OrganizationsDeidentifyTemplatesListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *OrganizationsDeidentifyTemplatesListCall) Fields(s ...googleapi.Field) *OrganizationsDeidentifyTemplatesListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *OrganizationsDeidentifyTemplatesListCall) IfNoneMatch(entityTag string) *OrganizationsDeidentifyTemplatesListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *OrganizationsDeidentifyTemplatesListCall) Context(ctx context.Context) *OrganizationsDeidentifyTemplatesListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *OrganizationsDeidentifyTemplatesListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *OrganizationsDeidentifyTemplatesListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/deidentifyTemplates")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.organizations.deidentifyTemplates.list" call.
// Exactly one of
// *GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse or error will
// be non-nil. Any non-2xx status code is an error. Response headers are
// in either
// *GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse.ServerResponse
// .Header or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *OrganizationsDeidentifyTemplatesListCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse{
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
	//   "description": "Lists inspect templates.",
	//   "flatPath": "v2beta2/organizations/{organizationsId}/deidentifyTemplates",
	//   "httpMethod": "GET",
	//   "id": "dlp.organizations.deidentifyTemplates.list",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "pageSize": {
	//       "description": "Optional size of the page, can be limited by server. If zero server returns\na page of max size 100.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "description": "Optional page token to continue retrieval. Comes from previous call\nto `ListDeidentifyTemplates`.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id or\norganizations/my-org-id.",
	//       "location": "path",
	//       "pattern": "^organizations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/deidentifyTemplates",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *OrganizationsDeidentifyTemplatesListCall) Pages(ctx context.Context, f func(*GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse) error) error {
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

// method id "dlp.organizations.deidentifyTemplates.patch":

type OrganizationsDeidentifyTemplatesPatchCall struct {
	s                                                      *Service
	name                                                   string
	googleprivacydlpv2beta2updatedeidentifytemplaterequest *GooglePrivacyDlpV2beta2UpdateDeidentifyTemplateRequest
	urlParams_                                             gensupport.URLParams
	ctx_                                                   context.Context
	header_                                                http.Header
}

// Patch: Updates the inspect template.
func (r *OrganizationsDeidentifyTemplatesService) Patch(name string, googleprivacydlpv2beta2updatedeidentifytemplaterequest *GooglePrivacyDlpV2beta2UpdateDeidentifyTemplateRequest) *OrganizationsDeidentifyTemplatesPatchCall {
	c := &OrganizationsDeidentifyTemplatesPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.googleprivacydlpv2beta2updatedeidentifytemplaterequest = googleprivacydlpv2beta2updatedeidentifytemplaterequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *OrganizationsDeidentifyTemplatesPatchCall) Fields(s ...googleapi.Field) *OrganizationsDeidentifyTemplatesPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *OrganizationsDeidentifyTemplatesPatchCall) Context(ctx context.Context) *OrganizationsDeidentifyTemplatesPatchCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *OrganizationsDeidentifyTemplatesPatchCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *OrganizationsDeidentifyTemplatesPatchCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2updatedeidentifytemplaterequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.organizations.deidentifyTemplates.patch" call.
// Exactly one of *GooglePrivacyDlpV2beta2DeidentifyTemplate or error
// will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2DeidentifyTemplate.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *OrganizationsDeidentifyTemplatesPatchCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2DeidentifyTemplate, error) {
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
	ret := &GooglePrivacyDlpV2beta2DeidentifyTemplate{
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
	//   "description": "Updates the inspect template.",
	//   "flatPath": "v2beta2/organizations/{organizationsId}/deidentifyTemplates/{deidentifyTemplatesId}",
	//   "httpMethod": "PATCH",
	//   "id": "dlp.organizations.deidentifyTemplates.patch",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of organization and deidentify template to be updated, for\nexample `organizations/433245324/deidentifyTemplates/432452342` or\nprojects/project-id/deidentifyTemplates/432452342.",
	//       "location": "path",
	//       "pattern": "^organizations/[^/]+/deidentifyTemplates/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2UpdateDeidentifyTemplateRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2DeidentifyTemplate"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.organizations.inspectTemplates.create":

type OrganizationsInspectTemplatesCreateCall struct {
	s                                                   *Service
	parent                                              string
	googleprivacydlpv2beta2createinspecttemplaterequest *GooglePrivacyDlpV2beta2CreateInspectTemplateRequest
	urlParams_                                          gensupport.URLParams
	ctx_                                                context.Context
	header_                                             http.Header
}

// Create: Creates an inspect template for re-using frequently used
// configuration
// for inspecting content, images, and storage.
func (r *OrganizationsInspectTemplatesService) Create(parent string, googleprivacydlpv2beta2createinspecttemplaterequest *GooglePrivacyDlpV2beta2CreateInspectTemplateRequest) *OrganizationsInspectTemplatesCreateCall {
	c := &OrganizationsInspectTemplatesCreateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.googleprivacydlpv2beta2createinspecttemplaterequest = googleprivacydlpv2beta2createinspecttemplaterequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *OrganizationsInspectTemplatesCreateCall) Fields(s ...googleapi.Field) *OrganizationsInspectTemplatesCreateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *OrganizationsInspectTemplatesCreateCall) Context(ctx context.Context) *OrganizationsInspectTemplatesCreateCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *OrganizationsInspectTemplatesCreateCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *OrganizationsInspectTemplatesCreateCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2createinspecttemplaterequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/inspectTemplates")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.organizations.inspectTemplates.create" call.
// Exactly one of *GooglePrivacyDlpV2beta2InspectTemplate or error will
// be non-nil. Any non-2xx status code is an error. Response headers are
// in either
// *GooglePrivacyDlpV2beta2InspectTemplate.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *OrganizationsInspectTemplatesCreateCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2InspectTemplate, error) {
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
	ret := &GooglePrivacyDlpV2beta2InspectTemplate{
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
	//   "description": "Creates an inspect template for re-using frequently used configuration\nfor inspecting content, images, and storage.",
	//   "flatPath": "v2beta2/organizations/{organizationsId}/inspectTemplates",
	//   "httpMethod": "POST",
	//   "id": "dlp.organizations.inspectTemplates.create",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id or\norganizations/my-org-id.",
	//       "location": "path",
	//       "pattern": "^organizations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/inspectTemplates",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2CreateInspectTemplateRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2InspectTemplate"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.organizations.inspectTemplates.delete":

type OrganizationsInspectTemplatesDeleteCall struct {
	s          *Service
	name       string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
	header_    http.Header
}

// Delete: Deletes inspect templates.
func (r *OrganizationsInspectTemplatesService) Delete(name string) *OrganizationsInspectTemplatesDeleteCall {
	c := &OrganizationsInspectTemplatesDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *OrganizationsInspectTemplatesDeleteCall) Fields(s ...googleapi.Field) *OrganizationsInspectTemplatesDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *OrganizationsInspectTemplatesDeleteCall) Context(ctx context.Context) *OrganizationsInspectTemplatesDeleteCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *OrganizationsInspectTemplatesDeleteCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *OrganizationsInspectTemplatesDeleteCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("DELETE", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.organizations.inspectTemplates.delete" call.
// Exactly one of *GoogleProtobufEmpty or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *GoogleProtobufEmpty.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *OrganizationsInspectTemplatesDeleteCall) Do(opts ...googleapi.CallOption) (*GoogleProtobufEmpty, error) {
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
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Deletes inspect templates.",
	//   "flatPath": "v2beta2/organizations/{organizationsId}/inspectTemplates/{inspectTemplatesId}",
	//   "httpMethod": "DELETE",
	//   "id": "dlp.organizations.inspectTemplates.delete",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of the organization and inspectTemplate to be deleted, for\nexample `organizations/433245324/inspectTemplates/432452342` or\nprojects/project-id/inspectTemplates/432452342.",
	//       "location": "path",
	//       "pattern": "^organizations/[^/]+/inspectTemplates/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "response": {
	//     "$ref": "GoogleProtobufEmpty"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.organizations.inspectTemplates.get":

type OrganizationsInspectTemplatesGetCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// Get: Gets an inspect template.
func (r *OrganizationsInspectTemplatesService) Get(name string) *OrganizationsInspectTemplatesGetCall {
	c := &OrganizationsInspectTemplatesGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *OrganizationsInspectTemplatesGetCall) Fields(s ...googleapi.Field) *OrganizationsInspectTemplatesGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *OrganizationsInspectTemplatesGetCall) IfNoneMatch(entityTag string) *OrganizationsInspectTemplatesGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *OrganizationsInspectTemplatesGetCall) Context(ctx context.Context) *OrganizationsInspectTemplatesGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *OrganizationsInspectTemplatesGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *OrganizationsInspectTemplatesGetCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.organizations.inspectTemplates.get" call.
// Exactly one of *GooglePrivacyDlpV2beta2InspectTemplate or error will
// be non-nil. Any non-2xx status code is an error. Response headers are
// in either
// *GooglePrivacyDlpV2beta2InspectTemplate.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *OrganizationsInspectTemplatesGetCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2InspectTemplate, error) {
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
	ret := &GooglePrivacyDlpV2beta2InspectTemplate{
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
	//   "description": "Gets an inspect template.",
	//   "flatPath": "v2beta2/organizations/{organizationsId}/inspectTemplates/{inspectTemplatesId}",
	//   "httpMethod": "GET",
	//   "id": "dlp.organizations.inspectTemplates.get",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of the organization and inspectTemplate to be read, for\nexample `organizations/433245324/inspectTemplates/432452342` or\nprojects/project-id/inspectTemplates/432452342.",
	//       "location": "path",
	//       "pattern": "^organizations/[^/]+/inspectTemplates/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2InspectTemplate"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.organizations.inspectTemplates.list":

type OrganizationsInspectTemplatesListCall struct {
	s            *Service
	parent       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists inspect templates.
func (r *OrganizationsInspectTemplatesService) List(parent string) *OrganizationsInspectTemplatesListCall {
	c := &OrganizationsInspectTemplatesListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	return c
}

// PageSize sets the optional parameter "pageSize": Optional size of the
// page, can be limited by server. If zero server returns
// a page of max size 100.
func (c *OrganizationsInspectTemplatesListCall) PageSize(pageSize int64) *OrganizationsInspectTemplatesListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": Optional page
// token to continue retrieval. Comes from previous call
// to `ListInspectTemplates`.
func (c *OrganizationsInspectTemplatesListCall) PageToken(pageToken string) *OrganizationsInspectTemplatesListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *OrganizationsInspectTemplatesListCall) Fields(s ...googleapi.Field) *OrganizationsInspectTemplatesListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *OrganizationsInspectTemplatesListCall) IfNoneMatch(entityTag string) *OrganizationsInspectTemplatesListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *OrganizationsInspectTemplatesListCall) Context(ctx context.Context) *OrganizationsInspectTemplatesListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *OrganizationsInspectTemplatesListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *OrganizationsInspectTemplatesListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/inspectTemplates")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.organizations.inspectTemplates.list" call.
// Exactly one of *GooglePrivacyDlpV2beta2ListInspectTemplatesResponse
// or error will be non-nil. Any non-2xx status code is an error.
// Response headers are in either
// *GooglePrivacyDlpV2beta2ListInspectTemplatesResponse.ServerResponse.He
// ader or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *OrganizationsInspectTemplatesListCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2ListInspectTemplatesResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta2ListInspectTemplatesResponse{
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
	//   "description": "Lists inspect templates.",
	//   "flatPath": "v2beta2/organizations/{organizationsId}/inspectTemplates",
	//   "httpMethod": "GET",
	//   "id": "dlp.organizations.inspectTemplates.list",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "pageSize": {
	//       "description": "Optional size of the page, can be limited by server. If zero server returns\na page of max size 100.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "description": "Optional page token to continue retrieval. Comes from previous call\nto `ListInspectTemplates`.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id or\norganizations/my-org-id.",
	//       "location": "path",
	//       "pattern": "^organizations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/inspectTemplates",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2ListInspectTemplatesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *OrganizationsInspectTemplatesListCall) Pages(ctx context.Context, f func(*GooglePrivacyDlpV2beta2ListInspectTemplatesResponse) error) error {
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

// method id "dlp.organizations.inspectTemplates.patch":

type OrganizationsInspectTemplatesPatchCall struct {
	s                                                   *Service
	name                                                string
	googleprivacydlpv2beta2updateinspecttemplaterequest *GooglePrivacyDlpV2beta2UpdateInspectTemplateRequest
	urlParams_                                          gensupport.URLParams
	ctx_                                                context.Context
	header_                                             http.Header
}

// Patch: Updates the inspect template.
func (r *OrganizationsInspectTemplatesService) Patch(name string, googleprivacydlpv2beta2updateinspecttemplaterequest *GooglePrivacyDlpV2beta2UpdateInspectTemplateRequest) *OrganizationsInspectTemplatesPatchCall {
	c := &OrganizationsInspectTemplatesPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.googleprivacydlpv2beta2updateinspecttemplaterequest = googleprivacydlpv2beta2updateinspecttemplaterequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *OrganizationsInspectTemplatesPatchCall) Fields(s ...googleapi.Field) *OrganizationsInspectTemplatesPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *OrganizationsInspectTemplatesPatchCall) Context(ctx context.Context) *OrganizationsInspectTemplatesPatchCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *OrganizationsInspectTemplatesPatchCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *OrganizationsInspectTemplatesPatchCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2updateinspecttemplaterequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.organizations.inspectTemplates.patch" call.
// Exactly one of *GooglePrivacyDlpV2beta2InspectTemplate or error will
// be non-nil. Any non-2xx status code is an error. Response headers are
// in either
// *GooglePrivacyDlpV2beta2InspectTemplate.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *OrganizationsInspectTemplatesPatchCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2InspectTemplate, error) {
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
	ret := &GooglePrivacyDlpV2beta2InspectTemplate{
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
	//   "description": "Updates the inspect template.",
	//   "flatPath": "v2beta2/organizations/{organizationsId}/inspectTemplates/{inspectTemplatesId}",
	//   "httpMethod": "PATCH",
	//   "id": "dlp.organizations.inspectTemplates.patch",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of organization and inspectTemplate to be updated, for\nexample `organizations/433245324/inspectTemplates/432452342` or\nprojects/project-id/inspectTemplates/432452342.",
	//       "location": "path",
	//       "pattern": "^organizations/[^/]+/inspectTemplates/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2UpdateInspectTemplateRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2InspectTemplate"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.content.deidentify":

type ProjectsContentDeidentifyCall struct {
	s                                               *Service
	parent                                          string
	googleprivacydlpv2beta2deidentifycontentrequest *GooglePrivacyDlpV2beta2DeidentifyContentRequest
	urlParams_                                      gensupport.URLParams
	ctx_                                            context.Context
	header_                                         http.Header
}

// Deidentify: De-identifies potentially sensitive info from a
// ContentItem.
// This method has limits on input size and output size.
// [How-to guide](/dlp/docs/deidentify-sensitive-data)
func (r *ProjectsContentService) Deidentify(parent string, googleprivacydlpv2beta2deidentifycontentrequest *GooglePrivacyDlpV2beta2DeidentifyContentRequest) *ProjectsContentDeidentifyCall {
	c := &ProjectsContentDeidentifyCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.googleprivacydlpv2beta2deidentifycontentrequest = googleprivacydlpv2beta2deidentifycontentrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsContentDeidentifyCall) Fields(s ...googleapi.Field) *ProjectsContentDeidentifyCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsContentDeidentifyCall) Context(ctx context.Context) *ProjectsContentDeidentifyCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsContentDeidentifyCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsContentDeidentifyCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2deidentifycontentrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/content:deidentify")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.content.deidentify" call.
// Exactly one of *GooglePrivacyDlpV2beta2DeidentifyContentResponse or
// error will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2DeidentifyContentResponse.ServerResponse.Heade
// r or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsContentDeidentifyCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2DeidentifyContentResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta2DeidentifyContentResponse{
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
	//   "description": "De-identifies potentially sensitive info from a ContentItem.\nThis method has limits on input size and output size.\n[How-to guide](/dlp/docs/deidentify-sensitive-data)",
	//   "flatPath": "v2beta2/projects/{projectsId}/content:deidentify",
	//   "httpMethod": "POST",
	//   "id": "dlp.projects.content.deidentify",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/content:deidentify",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2DeidentifyContentRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2DeidentifyContentResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.content.inspect":

type ProjectsContentInspectCall struct {
	s                                            *Service
	parent                                       string
	googleprivacydlpv2beta2inspectcontentrequest *GooglePrivacyDlpV2beta2InspectContentRequest
	urlParams_                                   gensupport.URLParams
	ctx_                                         context.Context
	header_                                      http.Header
}

// Inspect: Finds potentially sensitive info in content.
// This method has limits on input size, processing time, and output
// size.
// [How-to guide for text](/dlp/docs/inspecting-text), [How-to guide
// for
// images](/dlp/docs/inspecting-images)
func (r *ProjectsContentService) Inspect(parent string, googleprivacydlpv2beta2inspectcontentrequest *GooglePrivacyDlpV2beta2InspectContentRequest) *ProjectsContentInspectCall {
	c := &ProjectsContentInspectCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.googleprivacydlpv2beta2inspectcontentrequest = googleprivacydlpv2beta2inspectcontentrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsContentInspectCall) Fields(s ...googleapi.Field) *ProjectsContentInspectCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsContentInspectCall) Context(ctx context.Context) *ProjectsContentInspectCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsContentInspectCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsContentInspectCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2inspectcontentrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/content:inspect")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.content.inspect" call.
// Exactly one of *GooglePrivacyDlpV2beta2InspectContentResponse or
// error will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2InspectContentResponse.ServerResponse.Header
// or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsContentInspectCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2InspectContentResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta2InspectContentResponse{
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
	//   "description": "Finds potentially sensitive info in content.\nThis method has limits on input size, processing time, and output size.\n[How-to guide for text](/dlp/docs/inspecting-text), [How-to guide for\nimages](/dlp/docs/inspecting-images)",
	//   "flatPath": "v2beta2/projects/{projectsId}/content:inspect",
	//   "httpMethod": "POST",
	//   "id": "dlp.projects.content.inspect",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/content:inspect",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2InspectContentRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2InspectContentResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.content.reidentify":

type ProjectsContentReidentifyCall struct {
	s                                               *Service
	parent                                          string
	googleprivacydlpv2beta2reidentifycontentrequest *GooglePrivacyDlpV2beta2ReidentifyContentRequest
	urlParams_                                      gensupport.URLParams
	ctx_                                            context.Context
	header_                                         http.Header
}

// Reidentify: Re-identify content that has been de-identified.
func (r *ProjectsContentService) Reidentify(parent string, googleprivacydlpv2beta2reidentifycontentrequest *GooglePrivacyDlpV2beta2ReidentifyContentRequest) *ProjectsContentReidentifyCall {
	c := &ProjectsContentReidentifyCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.googleprivacydlpv2beta2reidentifycontentrequest = googleprivacydlpv2beta2reidentifycontentrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsContentReidentifyCall) Fields(s ...googleapi.Field) *ProjectsContentReidentifyCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsContentReidentifyCall) Context(ctx context.Context) *ProjectsContentReidentifyCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsContentReidentifyCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsContentReidentifyCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2reidentifycontentrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/content:reidentify")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.content.reidentify" call.
// Exactly one of *GooglePrivacyDlpV2beta2ReidentifyContentResponse or
// error will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2ReidentifyContentResponse.ServerResponse.Heade
// r or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsContentReidentifyCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2ReidentifyContentResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta2ReidentifyContentResponse{
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
	//   "description": "Re-identify content that has been de-identified.",
	//   "flatPath": "v2beta2/projects/{projectsId}/content:reidentify",
	//   "httpMethod": "POST",
	//   "id": "dlp.projects.content.reidentify",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The parent resource name.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/content:reidentify",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2ReidentifyContentRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2ReidentifyContentResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.dataSource.analyze":

type ProjectsDataSourceAnalyzeCall struct {
	s                                                   *Service
	parent                                              string
	googleprivacydlpv2beta2analyzedatasourceriskrequest *GooglePrivacyDlpV2beta2AnalyzeDataSourceRiskRequest
	urlParams_                                          gensupport.URLParams
	ctx_                                                context.Context
	header_                                             http.Header
}

// Analyze: Schedules a job to compute risk analysis metrics over
// content in a Google
// Cloud Platform repository. [How-to
// guide](/dlp/docs/compute-risk-analysis)
func (r *ProjectsDataSourceService) Analyze(parent string, googleprivacydlpv2beta2analyzedatasourceriskrequest *GooglePrivacyDlpV2beta2AnalyzeDataSourceRiskRequest) *ProjectsDataSourceAnalyzeCall {
	c := &ProjectsDataSourceAnalyzeCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.googleprivacydlpv2beta2analyzedatasourceriskrequest = googleprivacydlpv2beta2analyzedatasourceriskrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsDataSourceAnalyzeCall) Fields(s ...googleapi.Field) *ProjectsDataSourceAnalyzeCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsDataSourceAnalyzeCall) Context(ctx context.Context) *ProjectsDataSourceAnalyzeCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsDataSourceAnalyzeCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsDataSourceAnalyzeCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2analyzedatasourceriskrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/dataSource:analyze")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.dataSource.analyze" call.
// Exactly one of *GooglePrivacyDlpV2beta2DlpJob or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *GooglePrivacyDlpV2beta2DlpJob.ServerResponse.Header or (if a
// response was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsDataSourceAnalyzeCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2DlpJob, error) {
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
	ret := &GooglePrivacyDlpV2beta2DlpJob{
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
	//   "description": "Schedules a job to compute risk analysis metrics over content in a Google\nCloud Platform repository. [How-to guide](/dlp/docs/compute-risk-analysis)",
	//   "flatPath": "v2beta2/projects/{projectsId}/dataSource:analyze",
	//   "httpMethod": "POST",
	//   "id": "dlp.projects.dataSource.analyze",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/dataSource:analyze",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2AnalyzeDataSourceRiskRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2DlpJob"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.dataSource.inspect":

type ProjectsDataSourceInspectCall struct {
	s                                               *Service
	parent                                          string
	googleprivacydlpv2beta2inspectdatasourcerequest *GooglePrivacyDlpV2beta2InspectDataSourceRequest
	urlParams_                                      gensupport.URLParams
	ctx_                                            context.Context
	header_                                         http.Header
}

// Inspect: Schedules a job scanning content in a Google Cloud Platform
// data
// repository. [How-to guide](/dlp/docs/inspecting-storage)
func (r *ProjectsDataSourceService) Inspect(parent string, googleprivacydlpv2beta2inspectdatasourcerequest *GooglePrivacyDlpV2beta2InspectDataSourceRequest) *ProjectsDataSourceInspectCall {
	c := &ProjectsDataSourceInspectCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.googleprivacydlpv2beta2inspectdatasourcerequest = googleprivacydlpv2beta2inspectdatasourcerequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsDataSourceInspectCall) Fields(s ...googleapi.Field) *ProjectsDataSourceInspectCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsDataSourceInspectCall) Context(ctx context.Context) *ProjectsDataSourceInspectCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsDataSourceInspectCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsDataSourceInspectCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2inspectdatasourcerequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/dataSource:inspect")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.dataSource.inspect" call.
// Exactly one of *GooglePrivacyDlpV2beta2DlpJob or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *GooglePrivacyDlpV2beta2DlpJob.ServerResponse.Header or (if a
// response was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsDataSourceInspectCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2DlpJob, error) {
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
	ret := &GooglePrivacyDlpV2beta2DlpJob{
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
	//   "description": "Schedules a job scanning content in a Google Cloud Platform data\nrepository. [How-to guide](/dlp/docs/inspecting-storage)",
	//   "flatPath": "v2beta2/projects/{projectsId}/dataSource:inspect",
	//   "httpMethod": "POST",
	//   "id": "dlp.projects.dataSource.inspect",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/dataSource:inspect",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2InspectDataSourceRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2DlpJob"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.deidentifyTemplates.create":

type ProjectsDeidentifyTemplatesCreateCall struct {
	s                                                      *Service
	parent                                                 string
	googleprivacydlpv2beta2createdeidentifytemplaterequest *GooglePrivacyDlpV2beta2CreateDeidentifyTemplateRequest
	urlParams_                                             gensupport.URLParams
	ctx_                                                   context.Context
	header_                                                http.Header
}

// Create: Creates an Deidentify template for re-using frequently used
// configuration
// for Deidentifying content, images, and storage.
func (r *ProjectsDeidentifyTemplatesService) Create(parent string, googleprivacydlpv2beta2createdeidentifytemplaterequest *GooglePrivacyDlpV2beta2CreateDeidentifyTemplateRequest) *ProjectsDeidentifyTemplatesCreateCall {
	c := &ProjectsDeidentifyTemplatesCreateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.googleprivacydlpv2beta2createdeidentifytemplaterequest = googleprivacydlpv2beta2createdeidentifytemplaterequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsDeidentifyTemplatesCreateCall) Fields(s ...googleapi.Field) *ProjectsDeidentifyTemplatesCreateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsDeidentifyTemplatesCreateCall) Context(ctx context.Context) *ProjectsDeidentifyTemplatesCreateCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsDeidentifyTemplatesCreateCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsDeidentifyTemplatesCreateCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2createdeidentifytemplaterequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/deidentifyTemplates")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.deidentifyTemplates.create" call.
// Exactly one of *GooglePrivacyDlpV2beta2DeidentifyTemplate or error
// will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2DeidentifyTemplate.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsDeidentifyTemplatesCreateCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2DeidentifyTemplate, error) {
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
	ret := &GooglePrivacyDlpV2beta2DeidentifyTemplate{
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
	//   "description": "Creates an Deidentify template for re-using frequently used configuration\nfor Deidentifying content, images, and storage.",
	//   "flatPath": "v2beta2/projects/{projectsId}/deidentifyTemplates",
	//   "httpMethod": "POST",
	//   "id": "dlp.projects.deidentifyTemplates.create",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id or\norganizations/my-org-id.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/deidentifyTemplates",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2CreateDeidentifyTemplateRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2DeidentifyTemplate"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.deidentifyTemplates.delete":

type ProjectsDeidentifyTemplatesDeleteCall struct {
	s          *Service
	name       string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
	header_    http.Header
}

// Delete: Deletes inspect templates.
func (r *ProjectsDeidentifyTemplatesService) Delete(name string) *ProjectsDeidentifyTemplatesDeleteCall {
	c := &ProjectsDeidentifyTemplatesDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsDeidentifyTemplatesDeleteCall) Fields(s ...googleapi.Field) *ProjectsDeidentifyTemplatesDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsDeidentifyTemplatesDeleteCall) Context(ctx context.Context) *ProjectsDeidentifyTemplatesDeleteCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsDeidentifyTemplatesDeleteCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsDeidentifyTemplatesDeleteCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("DELETE", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.deidentifyTemplates.delete" call.
// Exactly one of *GoogleProtobufEmpty or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *GoogleProtobufEmpty.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsDeidentifyTemplatesDeleteCall) Do(opts ...googleapi.CallOption) (*GoogleProtobufEmpty, error) {
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
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Deletes inspect templates.",
	//   "flatPath": "v2beta2/projects/{projectsId}/deidentifyTemplates/{deidentifyTemplatesId}",
	//   "httpMethod": "DELETE",
	//   "id": "dlp.projects.deidentifyTemplates.delete",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of the organization and deidentify template to be deleted,\nfor example `organizations/433245324/deidentifyTemplates/432452342` or\nprojects/project-id/deidentifyTemplates/432452342.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/deidentifyTemplates/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "response": {
	//     "$ref": "GoogleProtobufEmpty"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.deidentifyTemplates.get":

type ProjectsDeidentifyTemplatesGetCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// Get: Gets an inspect template.
func (r *ProjectsDeidentifyTemplatesService) Get(name string) *ProjectsDeidentifyTemplatesGetCall {
	c := &ProjectsDeidentifyTemplatesGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsDeidentifyTemplatesGetCall) Fields(s ...googleapi.Field) *ProjectsDeidentifyTemplatesGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsDeidentifyTemplatesGetCall) IfNoneMatch(entityTag string) *ProjectsDeidentifyTemplatesGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsDeidentifyTemplatesGetCall) Context(ctx context.Context) *ProjectsDeidentifyTemplatesGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsDeidentifyTemplatesGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsDeidentifyTemplatesGetCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.deidentifyTemplates.get" call.
// Exactly one of *GooglePrivacyDlpV2beta2DeidentifyTemplate or error
// will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2DeidentifyTemplate.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsDeidentifyTemplatesGetCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2DeidentifyTemplate, error) {
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
	ret := &GooglePrivacyDlpV2beta2DeidentifyTemplate{
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
	//   "description": "Gets an inspect template.",
	//   "flatPath": "v2beta2/projects/{projectsId}/deidentifyTemplates/{deidentifyTemplatesId}",
	//   "httpMethod": "GET",
	//   "id": "dlp.projects.deidentifyTemplates.get",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of the organization and deidentify template to be read, for\nexample `organizations/433245324/deidentifyTemplates/432452342` or\nprojects/project-id/deidentifyTemplates/432452342.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/deidentifyTemplates/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2DeidentifyTemplate"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.deidentifyTemplates.list":

type ProjectsDeidentifyTemplatesListCall struct {
	s            *Service
	parent       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists inspect templates.
func (r *ProjectsDeidentifyTemplatesService) List(parent string) *ProjectsDeidentifyTemplatesListCall {
	c := &ProjectsDeidentifyTemplatesListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	return c
}

// PageSize sets the optional parameter "pageSize": Optional size of the
// page, can be limited by server. If zero server returns
// a page of max size 100.
func (c *ProjectsDeidentifyTemplatesListCall) PageSize(pageSize int64) *ProjectsDeidentifyTemplatesListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": Optional page
// token to continue retrieval. Comes from previous call
// to `ListDeidentifyTemplates`.
func (c *ProjectsDeidentifyTemplatesListCall) PageToken(pageToken string) *ProjectsDeidentifyTemplatesListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsDeidentifyTemplatesListCall) Fields(s ...googleapi.Field) *ProjectsDeidentifyTemplatesListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsDeidentifyTemplatesListCall) IfNoneMatch(entityTag string) *ProjectsDeidentifyTemplatesListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsDeidentifyTemplatesListCall) Context(ctx context.Context) *ProjectsDeidentifyTemplatesListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsDeidentifyTemplatesListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsDeidentifyTemplatesListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/deidentifyTemplates")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.deidentifyTemplates.list" call.
// Exactly one of
// *GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse or error will
// be non-nil. Any non-2xx status code is an error. Response headers are
// in either
// *GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse.ServerResponse
// .Header or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsDeidentifyTemplatesListCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse{
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
	//   "description": "Lists inspect templates.",
	//   "flatPath": "v2beta2/projects/{projectsId}/deidentifyTemplates",
	//   "httpMethod": "GET",
	//   "id": "dlp.projects.deidentifyTemplates.list",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "pageSize": {
	//       "description": "Optional size of the page, can be limited by server. If zero server returns\na page of max size 100.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "description": "Optional page token to continue retrieval. Comes from previous call\nto `ListDeidentifyTemplates`.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id or\norganizations/my-org-id.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/deidentifyTemplates",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *ProjectsDeidentifyTemplatesListCall) Pages(ctx context.Context, f func(*GooglePrivacyDlpV2beta2ListDeidentifyTemplatesResponse) error) error {
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

// method id "dlp.projects.deidentifyTemplates.patch":

type ProjectsDeidentifyTemplatesPatchCall struct {
	s                                                      *Service
	name                                                   string
	googleprivacydlpv2beta2updatedeidentifytemplaterequest *GooglePrivacyDlpV2beta2UpdateDeidentifyTemplateRequest
	urlParams_                                             gensupport.URLParams
	ctx_                                                   context.Context
	header_                                                http.Header
}

// Patch: Updates the inspect template.
func (r *ProjectsDeidentifyTemplatesService) Patch(name string, googleprivacydlpv2beta2updatedeidentifytemplaterequest *GooglePrivacyDlpV2beta2UpdateDeidentifyTemplateRequest) *ProjectsDeidentifyTemplatesPatchCall {
	c := &ProjectsDeidentifyTemplatesPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.googleprivacydlpv2beta2updatedeidentifytemplaterequest = googleprivacydlpv2beta2updatedeidentifytemplaterequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsDeidentifyTemplatesPatchCall) Fields(s ...googleapi.Field) *ProjectsDeidentifyTemplatesPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsDeidentifyTemplatesPatchCall) Context(ctx context.Context) *ProjectsDeidentifyTemplatesPatchCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsDeidentifyTemplatesPatchCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsDeidentifyTemplatesPatchCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2updatedeidentifytemplaterequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.deidentifyTemplates.patch" call.
// Exactly one of *GooglePrivacyDlpV2beta2DeidentifyTemplate or error
// will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2DeidentifyTemplate.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsDeidentifyTemplatesPatchCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2DeidentifyTemplate, error) {
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
	ret := &GooglePrivacyDlpV2beta2DeidentifyTemplate{
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
	//   "description": "Updates the inspect template.",
	//   "flatPath": "v2beta2/projects/{projectsId}/deidentifyTemplates/{deidentifyTemplatesId}",
	//   "httpMethod": "PATCH",
	//   "id": "dlp.projects.deidentifyTemplates.patch",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of organization and deidentify template to be updated, for\nexample `organizations/433245324/deidentifyTemplates/432452342` or\nprojects/project-id/deidentifyTemplates/432452342.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/deidentifyTemplates/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2UpdateDeidentifyTemplateRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2DeidentifyTemplate"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.dlpJobs.cancel":

type ProjectsDlpJobsCancelCall struct {
	s                                          *Service
	name                                       string
	googleprivacydlpv2beta2canceldlpjobrequest *GooglePrivacyDlpV2beta2CancelDlpJobRequest
	urlParams_                                 gensupport.URLParams
	ctx_                                       context.Context
	header_                                    http.Header
}

// Cancel: Starts asynchronous cancellation on a long-running DlpJob.
// The server
// makes a best effort to cancel the DlpJob, but success is
// not
// guaranteed.
func (r *ProjectsDlpJobsService) Cancel(name string, googleprivacydlpv2beta2canceldlpjobrequest *GooglePrivacyDlpV2beta2CancelDlpJobRequest) *ProjectsDlpJobsCancelCall {
	c := &ProjectsDlpJobsCancelCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.googleprivacydlpv2beta2canceldlpjobrequest = googleprivacydlpv2beta2canceldlpjobrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsDlpJobsCancelCall) Fields(s ...googleapi.Field) *ProjectsDlpJobsCancelCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsDlpJobsCancelCall) Context(ctx context.Context) *ProjectsDlpJobsCancelCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsDlpJobsCancelCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsDlpJobsCancelCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2canceldlpjobrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}:cancel")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.dlpJobs.cancel" call.
// Exactly one of *GoogleProtobufEmpty or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *GoogleProtobufEmpty.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsDlpJobsCancelCall) Do(opts ...googleapi.CallOption) (*GoogleProtobufEmpty, error) {
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
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Starts asynchronous cancellation on a long-running DlpJob.  The server\nmakes a best effort to cancel the DlpJob, but success is not\nguaranteed.",
	//   "flatPath": "v2beta2/projects/{projectsId}/dlpJobs/{dlpJobsId}:cancel",
	//   "httpMethod": "POST",
	//   "id": "dlp.projects.dlpJobs.cancel",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the DlpJob resource to be cancelled.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/dlpJobs/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}:cancel",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2CancelDlpJobRequest"
	//   },
	//   "response": {
	//     "$ref": "GoogleProtobufEmpty"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.dlpJobs.delete":

type ProjectsDlpJobsDeleteCall struct {
	s          *Service
	name       string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
	header_    http.Header
}

// Delete: Deletes a long-running DlpJob. This method indicates that the
// client is
// no longer interested in the DlpJob result. The job will be cancelled
// if
// possible.
func (r *ProjectsDlpJobsService) Delete(name string) *ProjectsDlpJobsDeleteCall {
	c := &ProjectsDlpJobsDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsDlpJobsDeleteCall) Fields(s ...googleapi.Field) *ProjectsDlpJobsDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsDlpJobsDeleteCall) Context(ctx context.Context) *ProjectsDlpJobsDeleteCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsDlpJobsDeleteCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsDlpJobsDeleteCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("DELETE", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.dlpJobs.delete" call.
// Exactly one of *GoogleProtobufEmpty or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *GoogleProtobufEmpty.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsDlpJobsDeleteCall) Do(opts ...googleapi.CallOption) (*GoogleProtobufEmpty, error) {
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
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Deletes a long-running DlpJob. This method indicates that the client is\nno longer interested in the DlpJob result. The job will be cancelled if\npossible.",
	//   "flatPath": "v2beta2/projects/{projectsId}/dlpJobs/{dlpJobsId}",
	//   "httpMethod": "DELETE",
	//   "id": "dlp.projects.dlpJobs.delete",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the DlpJob resource to be deleted.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/dlpJobs/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "response": {
	//     "$ref": "GoogleProtobufEmpty"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.dlpJobs.get":

type ProjectsDlpJobsGetCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// Get: Gets the latest state of a long-running DlpJob.
func (r *ProjectsDlpJobsService) Get(name string) *ProjectsDlpJobsGetCall {
	c := &ProjectsDlpJobsGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsDlpJobsGetCall) Fields(s ...googleapi.Field) *ProjectsDlpJobsGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsDlpJobsGetCall) IfNoneMatch(entityTag string) *ProjectsDlpJobsGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsDlpJobsGetCall) Context(ctx context.Context) *ProjectsDlpJobsGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsDlpJobsGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsDlpJobsGetCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.dlpJobs.get" call.
// Exactly one of *GooglePrivacyDlpV2beta2DlpJob or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *GooglePrivacyDlpV2beta2DlpJob.ServerResponse.Header or (if a
// response was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsDlpJobsGetCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2DlpJob, error) {
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
	ret := &GooglePrivacyDlpV2beta2DlpJob{
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
	//   "description": "Gets the latest state of a long-running DlpJob.",
	//   "flatPath": "v2beta2/projects/{projectsId}/dlpJobs/{dlpJobsId}",
	//   "httpMethod": "GET",
	//   "id": "dlp.projects.dlpJobs.get",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the DlpJob resource.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/dlpJobs/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2DlpJob"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.dlpJobs.list":

type ProjectsDlpJobsListCall struct {
	s            *Service
	parent       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists DlpJobs that match the specified filter in the request.
func (r *ProjectsDlpJobsService) List(parent string) *ProjectsDlpJobsListCall {
	c := &ProjectsDlpJobsListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	return c
}

// Filter sets the optional parameter "filter": Allows
// filtering.
//
// Supported syntax:
//
// * Filter expressions are made up of one or more restrictions.
// * Restrictions can be combined by `AND` or `OR` logical operators.
// A
// sequence of restrictions implicitly uses `AND`.
// * A restriction has the form of `<field> <operator> <value>`.
// * Supported fields/values for inspect jobs:
//     - `state` - PENDING|RUNNING|CANCELED|FINISHED|FAILED
//     - `inspected_storage` - DATASTORE|CLOUD_STORAGE|BIGQUERY
//     - `trigger_name` - The resource name of the trigger that created
// job.
// * Supported fields for risk analysis jobs:
//     - `state` - RUNNING|CANCELED|FINISHED|FAILED
// * The operator must be `=` or `!=`.
//
// Examples:
//
// * inspected_storage = cloud_storage AND state = done
// * inspected_storage = cloud_storage OR inspected_storage = bigquery
// * inspected_storage = cloud_storage AND (state = done OR state =
// canceled)
//
// The length of this field should be no more than 500 characters.
func (c *ProjectsDlpJobsListCall) Filter(filter string) *ProjectsDlpJobsListCall {
	c.urlParams_.Set("filter", filter)
	return c
}

// PageSize sets the optional parameter "pageSize": The standard list
// page size.
func (c *ProjectsDlpJobsListCall) PageSize(pageSize int64) *ProjectsDlpJobsListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": The standard list
// page token.
func (c *ProjectsDlpJobsListCall) PageToken(pageToken string) *ProjectsDlpJobsListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Type sets the optional parameter "type": The type of job. Defaults to
// `DlpJobType.INSPECT`
//
// Possible values:
//   "DLP_JOB_TYPE_UNSPECIFIED"
//   "INSPECT_JOB"
//   "RISK_ANALYSIS_JOB"
func (c *ProjectsDlpJobsListCall) Type(type_ string) *ProjectsDlpJobsListCall {
	c.urlParams_.Set("type", type_)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsDlpJobsListCall) Fields(s ...googleapi.Field) *ProjectsDlpJobsListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsDlpJobsListCall) IfNoneMatch(entityTag string) *ProjectsDlpJobsListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsDlpJobsListCall) Context(ctx context.Context) *ProjectsDlpJobsListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsDlpJobsListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsDlpJobsListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/dlpJobs")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.dlpJobs.list" call.
// Exactly one of *GooglePrivacyDlpV2beta2ListDlpJobsResponse or error
// will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2ListDlpJobsResponse.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsDlpJobsListCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2ListDlpJobsResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta2ListDlpJobsResponse{
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
	//   "description": "Lists DlpJobs that match the specified filter in the request.",
	//   "flatPath": "v2beta2/projects/{projectsId}/dlpJobs",
	//   "httpMethod": "GET",
	//   "id": "dlp.projects.dlpJobs.list",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "filter": {
	//       "description": "Optional. Allows filtering.\n\nSupported syntax:\n\n* Filter expressions are made up of one or more restrictions.\n* Restrictions can be combined by `AND` or `OR` logical operators. A\nsequence of restrictions implicitly uses `AND`.\n* A restriction has the form of `\u003cfield\u003e \u003coperator\u003e \u003cvalue\u003e`.\n* Supported fields/values for inspect jobs:\n    - `state` - PENDING|RUNNING|CANCELED|FINISHED|FAILED\n    - `inspected_storage` - DATASTORE|CLOUD_STORAGE|BIGQUERY\n    - `trigger_name` - The resource name of the trigger that created job.\n* Supported fields for risk analysis jobs:\n    - `state` - RUNNING|CANCELED|FINISHED|FAILED\n* The operator must be `=` or `!=`.\n\nExamples:\n\n* inspected_storage = cloud_storage AND state = done\n* inspected_storage = cloud_storage OR inspected_storage = bigquery\n* inspected_storage = cloud_storage AND (state = done OR state = canceled)\n\nThe length of this field should be no more than 500 characters.",
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
	//     },
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "type": {
	//       "description": "The type of job. Defaults to `DlpJobType.INSPECT`",
	//       "enum": [
	//         "DLP_JOB_TYPE_UNSPECIFIED",
	//         "INSPECT_JOB",
	//         "RISK_ANALYSIS_JOB"
	//       ],
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/dlpJobs",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2ListDlpJobsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *ProjectsDlpJobsListCall) Pages(ctx context.Context, f func(*GooglePrivacyDlpV2beta2ListDlpJobsResponse) error) error {
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

// method id "dlp.projects.image.redact":

type ProjectsImageRedactCall struct {
	s                                         *Service
	parent                                    string
	googleprivacydlpv2beta2redactimagerequest *GooglePrivacyDlpV2beta2RedactImageRequest
	urlParams_                                gensupport.URLParams
	ctx_                                      context.Context
	header_                                   http.Header
}

// Redact: Redacts potentially sensitive info from an image.
// This method has limits on input size, processing time, and output
// size.
// [How-to guide](/dlp/docs/redacting-sensitive-data-images)
func (r *ProjectsImageService) Redact(parent string, googleprivacydlpv2beta2redactimagerequest *GooglePrivacyDlpV2beta2RedactImageRequest) *ProjectsImageRedactCall {
	c := &ProjectsImageRedactCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.googleprivacydlpv2beta2redactimagerequest = googleprivacydlpv2beta2redactimagerequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsImageRedactCall) Fields(s ...googleapi.Field) *ProjectsImageRedactCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsImageRedactCall) Context(ctx context.Context) *ProjectsImageRedactCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsImageRedactCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsImageRedactCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2redactimagerequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/image:redact")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.image.redact" call.
// Exactly one of *GooglePrivacyDlpV2beta2RedactImageResponse or error
// will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2RedactImageResponse.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsImageRedactCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2RedactImageResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta2RedactImageResponse{
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
	//   "description": "Redacts potentially sensitive info from an image.\nThis method has limits on input size, processing time, and output size.\n[How-to guide](/dlp/docs/redacting-sensitive-data-images)",
	//   "flatPath": "v2beta2/projects/{projectsId}/image:redact",
	//   "httpMethod": "POST",
	//   "id": "dlp.projects.image.redact",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/image:redact",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2RedactImageRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2RedactImageResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.inspectTemplates.create":

type ProjectsInspectTemplatesCreateCall struct {
	s                                                   *Service
	parent                                              string
	googleprivacydlpv2beta2createinspecttemplaterequest *GooglePrivacyDlpV2beta2CreateInspectTemplateRequest
	urlParams_                                          gensupport.URLParams
	ctx_                                                context.Context
	header_                                             http.Header
}

// Create: Creates an inspect template for re-using frequently used
// configuration
// for inspecting content, images, and storage.
func (r *ProjectsInspectTemplatesService) Create(parent string, googleprivacydlpv2beta2createinspecttemplaterequest *GooglePrivacyDlpV2beta2CreateInspectTemplateRequest) *ProjectsInspectTemplatesCreateCall {
	c := &ProjectsInspectTemplatesCreateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.googleprivacydlpv2beta2createinspecttemplaterequest = googleprivacydlpv2beta2createinspecttemplaterequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsInspectTemplatesCreateCall) Fields(s ...googleapi.Field) *ProjectsInspectTemplatesCreateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsInspectTemplatesCreateCall) Context(ctx context.Context) *ProjectsInspectTemplatesCreateCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsInspectTemplatesCreateCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsInspectTemplatesCreateCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2createinspecttemplaterequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/inspectTemplates")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.inspectTemplates.create" call.
// Exactly one of *GooglePrivacyDlpV2beta2InspectTemplate or error will
// be non-nil. Any non-2xx status code is an error. Response headers are
// in either
// *GooglePrivacyDlpV2beta2InspectTemplate.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsInspectTemplatesCreateCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2InspectTemplate, error) {
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
	ret := &GooglePrivacyDlpV2beta2InspectTemplate{
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
	//   "description": "Creates an inspect template for re-using frequently used configuration\nfor inspecting content, images, and storage.",
	//   "flatPath": "v2beta2/projects/{projectsId}/inspectTemplates",
	//   "httpMethod": "POST",
	//   "id": "dlp.projects.inspectTemplates.create",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id or\norganizations/my-org-id.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/inspectTemplates",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2CreateInspectTemplateRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2InspectTemplate"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.inspectTemplates.delete":

type ProjectsInspectTemplatesDeleteCall struct {
	s          *Service
	name       string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
	header_    http.Header
}

// Delete: Deletes inspect templates.
func (r *ProjectsInspectTemplatesService) Delete(name string) *ProjectsInspectTemplatesDeleteCall {
	c := &ProjectsInspectTemplatesDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsInspectTemplatesDeleteCall) Fields(s ...googleapi.Field) *ProjectsInspectTemplatesDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsInspectTemplatesDeleteCall) Context(ctx context.Context) *ProjectsInspectTemplatesDeleteCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsInspectTemplatesDeleteCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsInspectTemplatesDeleteCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("DELETE", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.inspectTemplates.delete" call.
// Exactly one of *GoogleProtobufEmpty or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *GoogleProtobufEmpty.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsInspectTemplatesDeleteCall) Do(opts ...googleapi.CallOption) (*GoogleProtobufEmpty, error) {
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
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Deletes inspect templates.",
	//   "flatPath": "v2beta2/projects/{projectsId}/inspectTemplates/{inspectTemplatesId}",
	//   "httpMethod": "DELETE",
	//   "id": "dlp.projects.inspectTemplates.delete",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of the organization and inspectTemplate to be deleted, for\nexample `organizations/433245324/inspectTemplates/432452342` or\nprojects/project-id/inspectTemplates/432452342.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/inspectTemplates/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "response": {
	//     "$ref": "GoogleProtobufEmpty"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.inspectTemplates.get":

type ProjectsInspectTemplatesGetCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// Get: Gets an inspect template.
func (r *ProjectsInspectTemplatesService) Get(name string) *ProjectsInspectTemplatesGetCall {
	c := &ProjectsInspectTemplatesGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsInspectTemplatesGetCall) Fields(s ...googleapi.Field) *ProjectsInspectTemplatesGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsInspectTemplatesGetCall) IfNoneMatch(entityTag string) *ProjectsInspectTemplatesGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsInspectTemplatesGetCall) Context(ctx context.Context) *ProjectsInspectTemplatesGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsInspectTemplatesGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsInspectTemplatesGetCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.inspectTemplates.get" call.
// Exactly one of *GooglePrivacyDlpV2beta2InspectTemplate or error will
// be non-nil. Any non-2xx status code is an error. Response headers are
// in either
// *GooglePrivacyDlpV2beta2InspectTemplate.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsInspectTemplatesGetCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2InspectTemplate, error) {
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
	ret := &GooglePrivacyDlpV2beta2InspectTemplate{
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
	//   "description": "Gets an inspect template.",
	//   "flatPath": "v2beta2/projects/{projectsId}/inspectTemplates/{inspectTemplatesId}",
	//   "httpMethod": "GET",
	//   "id": "dlp.projects.inspectTemplates.get",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of the organization and inspectTemplate to be read, for\nexample `organizations/433245324/inspectTemplates/432452342` or\nprojects/project-id/inspectTemplates/432452342.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/inspectTemplates/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2InspectTemplate"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.inspectTemplates.list":

type ProjectsInspectTemplatesListCall struct {
	s            *Service
	parent       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists inspect templates.
func (r *ProjectsInspectTemplatesService) List(parent string) *ProjectsInspectTemplatesListCall {
	c := &ProjectsInspectTemplatesListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	return c
}

// PageSize sets the optional parameter "pageSize": Optional size of the
// page, can be limited by server. If zero server returns
// a page of max size 100.
func (c *ProjectsInspectTemplatesListCall) PageSize(pageSize int64) *ProjectsInspectTemplatesListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": Optional page
// token to continue retrieval. Comes from previous call
// to `ListInspectTemplates`.
func (c *ProjectsInspectTemplatesListCall) PageToken(pageToken string) *ProjectsInspectTemplatesListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsInspectTemplatesListCall) Fields(s ...googleapi.Field) *ProjectsInspectTemplatesListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsInspectTemplatesListCall) IfNoneMatch(entityTag string) *ProjectsInspectTemplatesListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsInspectTemplatesListCall) Context(ctx context.Context) *ProjectsInspectTemplatesListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsInspectTemplatesListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsInspectTemplatesListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/inspectTemplates")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.inspectTemplates.list" call.
// Exactly one of *GooglePrivacyDlpV2beta2ListInspectTemplatesResponse
// or error will be non-nil. Any non-2xx status code is an error.
// Response headers are in either
// *GooglePrivacyDlpV2beta2ListInspectTemplatesResponse.ServerResponse.He
// ader or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsInspectTemplatesListCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2ListInspectTemplatesResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta2ListInspectTemplatesResponse{
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
	//   "description": "Lists inspect templates.",
	//   "flatPath": "v2beta2/projects/{projectsId}/inspectTemplates",
	//   "httpMethod": "GET",
	//   "id": "dlp.projects.inspectTemplates.list",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "pageSize": {
	//       "description": "Optional size of the page, can be limited by server. If zero server returns\na page of max size 100.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "description": "Optional page token to continue retrieval. Comes from previous call\nto `ListInspectTemplates`.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id or\norganizations/my-org-id.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/inspectTemplates",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2ListInspectTemplatesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *ProjectsInspectTemplatesListCall) Pages(ctx context.Context, f func(*GooglePrivacyDlpV2beta2ListInspectTemplatesResponse) error) error {
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

// method id "dlp.projects.inspectTemplates.patch":

type ProjectsInspectTemplatesPatchCall struct {
	s                                                   *Service
	name                                                string
	googleprivacydlpv2beta2updateinspecttemplaterequest *GooglePrivacyDlpV2beta2UpdateInspectTemplateRequest
	urlParams_                                          gensupport.URLParams
	ctx_                                                context.Context
	header_                                             http.Header
}

// Patch: Updates the inspect template.
func (r *ProjectsInspectTemplatesService) Patch(name string, googleprivacydlpv2beta2updateinspecttemplaterequest *GooglePrivacyDlpV2beta2UpdateInspectTemplateRequest) *ProjectsInspectTemplatesPatchCall {
	c := &ProjectsInspectTemplatesPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.googleprivacydlpv2beta2updateinspecttemplaterequest = googleprivacydlpv2beta2updateinspecttemplaterequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsInspectTemplatesPatchCall) Fields(s ...googleapi.Field) *ProjectsInspectTemplatesPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsInspectTemplatesPatchCall) Context(ctx context.Context) *ProjectsInspectTemplatesPatchCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsInspectTemplatesPatchCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsInspectTemplatesPatchCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2updateinspecttemplaterequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.inspectTemplates.patch" call.
// Exactly one of *GooglePrivacyDlpV2beta2InspectTemplate or error will
// be non-nil. Any non-2xx status code is an error. Response headers are
// in either
// *GooglePrivacyDlpV2beta2InspectTemplate.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsInspectTemplatesPatchCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2InspectTemplate, error) {
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
	ret := &GooglePrivacyDlpV2beta2InspectTemplate{
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
	//   "description": "Updates the inspect template.",
	//   "flatPath": "v2beta2/projects/{projectsId}/inspectTemplates/{inspectTemplatesId}",
	//   "httpMethod": "PATCH",
	//   "id": "dlp.projects.inspectTemplates.patch",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of organization and inspectTemplate to be updated, for\nexample `organizations/433245324/inspectTemplates/432452342` or\nprojects/project-id/inspectTemplates/432452342.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/inspectTemplates/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2UpdateInspectTemplateRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2InspectTemplate"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.jobTriggers.create":

type ProjectsJobTriggersCreateCall struct {
	s                                              *Service
	parent                                         string
	googleprivacydlpv2beta2createjobtriggerrequest *GooglePrivacyDlpV2beta2CreateJobTriggerRequest
	urlParams_                                     gensupport.URLParams
	ctx_                                           context.Context
	header_                                        http.Header
}

// Create: Creates a job to run DLP actions such as scanning storage for
// sensitive
// information on a set schedule.
func (r *ProjectsJobTriggersService) Create(parent string, googleprivacydlpv2beta2createjobtriggerrequest *GooglePrivacyDlpV2beta2CreateJobTriggerRequest) *ProjectsJobTriggersCreateCall {
	c := &ProjectsJobTriggersCreateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.googleprivacydlpv2beta2createjobtriggerrequest = googleprivacydlpv2beta2createjobtriggerrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsJobTriggersCreateCall) Fields(s ...googleapi.Field) *ProjectsJobTriggersCreateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsJobTriggersCreateCall) Context(ctx context.Context) *ProjectsJobTriggersCreateCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsJobTriggersCreateCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsJobTriggersCreateCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2createjobtriggerrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/jobTriggers")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.jobTriggers.create" call.
// Exactly one of *GooglePrivacyDlpV2beta2JobTrigger or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *GooglePrivacyDlpV2beta2JobTrigger.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsJobTriggersCreateCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2JobTrigger, error) {
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
	ret := &GooglePrivacyDlpV2beta2JobTrigger{
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
	//   "description": "Creates a job to run DLP actions such as scanning storage for sensitive\ninformation on a set schedule.",
	//   "flatPath": "v2beta2/projects/{projectsId}/jobTriggers",
	//   "httpMethod": "POST",
	//   "id": "dlp.projects.jobTriggers.create",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/jobTriggers",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2CreateJobTriggerRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2JobTrigger"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.jobTriggers.delete":

type ProjectsJobTriggersDeleteCall struct {
	s          *Service
	name       string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
	header_    http.Header
}

// Delete: Deletes a job trigger.
func (r *ProjectsJobTriggersService) Delete(name string) *ProjectsJobTriggersDeleteCall {
	c := &ProjectsJobTriggersDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsJobTriggersDeleteCall) Fields(s ...googleapi.Field) *ProjectsJobTriggersDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsJobTriggersDeleteCall) Context(ctx context.Context) *ProjectsJobTriggersDeleteCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsJobTriggersDeleteCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsJobTriggersDeleteCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("DELETE", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.jobTriggers.delete" call.
// Exactly one of *GoogleProtobufEmpty or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *GoogleProtobufEmpty.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsJobTriggersDeleteCall) Do(opts ...googleapi.CallOption) (*GoogleProtobufEmpty, error) {
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
	if err := gensupport.DecodeResponse(target, res); err != nil {
		return nil, err
	}
	return ret, nil
	// {
	//   "description": "Deletes a job trigger.",
	//   "flatPath": "v2beta2/projects/{projectsId}/jobTriggers/{jobTriggersId}",
	//   "httpMethod": "DELETE",
	//   "id": "dlp.projects.jobTriggers.delete",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of the project and the triggeredJob, for example\n`projects/dlp-test-project/jobTriggers/53234423`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/jobTriggers/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "response": {
	//     "$ref": "GoogleProtobufEmpty"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.jobTriggers.get":

type ProjectsJobTriggersGetCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// Get: Gets a job trigger.
func (r *ProjectsJobTriggersService) Get(name string) *ProjectsJobTriggersGetCall {
	c := &ProjectsJobTriggersGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsJobTriggersGetCall) Fields(s ...googleapi.Field) *ProjectsJobTriggersGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsJobTriggersGetCall) IfNoneMatch(entityTag string) *ProjectsJobTriggersGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsJobTriggersGetCall) Context(ctx context.Context) *ProjectsJobTriggersGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsJobTriggersGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsJobTriggersGetCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.jobTriggers.get" call.
// Exactly one of *GooglePrivacyDlpV2beta2JobTrigger or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *GooglePrivacyDlpV2beta2JobTrigger.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsJobTriggersGetCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2JobTrigger, error) {
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
	ret := &GooglePrivacyDlpV2beta2JobTrigger{
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
	//   "description": "Gets a job trigger.",
	//   "flatPath": "v2beta2/projects/{projectsId}/jobTriggers/{jobTriggersId}",
	//   "httpMethod": "GET",
	//   "id": "dlp.projects.jobTriggers.get",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of the project and the triggeredJob, for example\n`projects/dlp-test-project/jobTriggers/53234423`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/jobTriggers/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2JobTrigger"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// method id "dlp.projects.jobTriggers.list":

type ProjectsJobTriggersListCall struct {
	s            *Service
	parent       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists job triggers.
func (r *ProjectsJobTriggersService) List(parent string) *ProjectsJobTriggersListCall {
	c := &ProjectsJobTriggersListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	return c
}

// OrderBy sets the optional parameter "orderBy": Optional comma
// separated list of triggeredJob fields to order by,
// followed by 'asc/desc' postfix, i.e.
// "create_time asc,name desc,schedule_mode asc". This list
// is
// case-insensitive.
//
// Example: "name asc,schedule_mode desc, status desc"
//
// Supported filters keys and values are:
//
// - `create_time`: corresponds to time the triggeredJob was created.
// - `update_time`: corresponds to time the triggeredJob was last
// updated.
// - `name`: corresponds to JobTrigger's display name.
// - `status`: corresponds to the triggeredJob status.
func (c *ProjectsJobTriggersListCall) OrderBy(orderBy string) *ProjectsJobTriggersListCall {
	c.urlParams_.Set("orderBy", orderBy)
	return c
}

// PageSize sets the optional parameter "pageSize": Optional size of the
// page, can be limited by a server.
func (c *ProjectsJobTriggersListCall) PageSize(pageSize int64) *ProjectsJobTriggersListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": Optional page
// token to continue retrieval. Comes from previous call
// to ListJobTriggers. `order_by` and `filter` should not change
// for
// subsequent calls, but can be omitted if token is specified.
func (c *ProjectsJobTriggersListCall) PageToken(pageToken string) *ProjectsJobTriggersListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsJobTriggersListCall) Fields(s ...googleapi.Field) *ProjectsJobTriggersListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsJobTriggersListCall) IfNoneMatch(entityTag string) *ProjectsJobTriggersListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsJobTriggersListCall) Context(ctx context.Context) *ProjectsJobTriggersListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsJobTriggersListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsJobTriggersListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+parent}/jobTriggers")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.jobTriggers.list" call.
// Exactly one of *GooglePrivacyDlpV2beta2ListJobTriggersResponse or
// error will be non-nil. Any non-2xx status code is an error. Response
// headers are in either
// *GooglePrivacyDlpV2beta2ListJobTriggersResponse.ServerResponse.Header
// or (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsJobTriggersListCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2ListJobTriggersResponse, error) {
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
	ret := &GooglePrivacyDlpV2beta2ListJobTriggersResponse{
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
	//   "description": "Lists job triggers.",
	//   "flatPath": "v2beta2/projects/{projectsId}/jobTriggers",
	//   "httpMethod": "GET",
	//   "id": "dlp.projects.jobTriggers.list",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "orderBy": {
	//       "description": "Optional comma separated list of triggeredJob fields to order by,\nfollowed by 'asc/desc' postfix, i.e.\n`\"create_time asc,name desc,schedule_mode asc\"`. This list is\ncase-insensitive.\n\nExample: `\"name asc,schedule_mode desc, status desc\"`\n\nSupported filters keys and values are:\n\n- `create_time`: corresponds to time the triggeredJob was created.\n- `update_time`: corresponds to time the triggeredJob was last updated.\n- `name`: corresponds to JobTrigger's display name.\n- `status`: corresponds to the triggeredJob status.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "pageSize": {
	//       "description": "Optional size of the page, can be limited by a server.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "description": "Optional page token to continue retrieval. Comes from previous call\nto ListJobTriggers. `order_by` and `filter` should not change for\nsubsequent calls, but can be omitted if token is specified.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "parent": {
	//       "description": "The parent resource name, for example projects/my-project-id.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+parent}/jobTriggers",
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2ListJobTriggersResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *ProjectsJobTriggersListCall) Pages(ctx context.Context, f func(*GooglePrivacyDlpV2beta2ListJobTriggersResponse) error) error {
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

// method id "dlp.projects.jobTriggers.patch":

type ProjectsJobTriggersPatchCall struct {
	s                                              *Service
	name                                           string
	googleprivacydlpv2beta2updatejobtriggerrequest *GooglePrivacyDlpV2beta2UpdateJobTriggerRequest
	urlParams_                                     gensupport.URLParams
	ctx_                                           context.Context
	header_                                        http.Header
}

// Patch: Updates a job trigger.
func (r *ProjectsJobTriggersService) Patch(name string, googleprivacydlpv2beta2updatejobtriggerrequest *GooglePrivacyDlpV2beta2UpdateJobTriggerRequest) *ProjectsJobTriggersPatchCall {
	c := &ProjectsJobTriggersPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.googleprivacydlpv2beta2updatejobtriggerrequest = googleprivacydlpv2beta2updatejobtriggerrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsJobTriggersPatchCall) Fields(s ...googleapi.Field) *ProjectsJobTriggersPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsJobTriggersPatchCall) Context(ctx context.Context) *ProjectsJobTriggersPatchCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsJobTriggersPatchCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsJobTriggersPatchCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.googleprivacydlpv2beta2updatejobtriggerrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v2beta2/{+name}")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("PATCH", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "dlp.projects.jobTriggers.patch" call.
// Exactly one of *GooglePrivacyDlpV2beta2JobTrigger or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *GooglePrivacyDlpV2beta2JobTrigger.ServerResponse.Header or
// (if a response was returned at all) in
// error.(*googleapi.Error).Header. Use googleapi.IsNotModified to check
// whether the returned error was because http.StatusNotModified was
// returned.
func (c *ProjectsJobTriggersPatchCall) Do(opts ...googleapi.CallOption) (*GooglePrivacyDlpV2beta2JobTrigger, error) {
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
	ret := &GooglePrivacyDlpV2beta2JobTrigger{
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
	//   "description": "Updates a job trigger.",
	//   "flatPath": "v2beta2/projects/{projectsId}/jobTriggers/{jobTriggersId}",
	//   "httpMethod": "PATCH",
	//   "id": "dlp.projects.jobTriggers.patch",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "Resource name of the project and the triggeredJob, for example\n`projects/dlp-test-project/jobTriggers/53234423`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/jobTriggers/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v2beta2/{+name}",
	//   "request": {
	//     "$ref": "GooglePrivacyDlpV2beta2UpdateJobTriggerRequest"
	//   },
	//   "response": {
	//     "$ref": "GooglePrivacyDlpV2beta2JobTrigger"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform"
	//   ]
	// }

}
