// Package cloudfunctions provides access to the Google Cloud Functions API.
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

func New(client *http.Client) (*Service, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}
	s := &Service{client: client, BasePath: basePath}
	return s, nil
}

type Service struct {
	client    *http.Client
	BasePath  string // API endpoint base URL
	UserAgent string // optional additional User-Agent fragment
}

func (s *Service) userAgent() string {
	if s.UserAgent == "" {
		return googleapi.UserAgent
	}
	return googleapi.UserAgent + " " + s.UserAgent
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
	type noMethod OperationMetadataV1Beta2
	raw := noMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}
