// Package cloudiot provides access to the Cloud IoT API.
//
// See https://cloud.google.com/iot
//
// Usage example:
//
//   import "google.golang.org/api/cloudiot/v1"
//   ...
//   cloudiotService, err := cloudiot.New(oauthHttpClient)
package cloudiot // import "google.golang.org/api/cloudiot/v1"

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

const apiId = "cloudiot:v1"
const apiName = "cloudiot"
const apiVersion = "v1"
const basePath = "https://cloudiot.googleapis.com/"

// OAuth2 scopes used by this API.
const (
	// View and manage your data across Google Cloud Platform services
	CloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

	// Register and manage devices in the Google Cloud IoT service
	CloudiotScope = "https://www.googleapis.com/auth/cloudiot"
)

func New(client *http.Client) (*Service, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}
	s := &Service{client: client, BasePath: basePath}
	s.Projects = NewProjectsService(s)
	return s, nil
}

type Service struct {
	client    *http.Client
	BasePath  string // API endpoint base URL
	UserAgent string // optional additional User-Agent fragment

	Projects *ProjectsService
}

func (s *Service) userAgent() string {
	if s.UserAgent == "" {
		return googleapi.UserAgent
	}
	return googleapi.UserAgent + " " + s.UserAgent
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
	rs.Registries = NewProjectsLocationsRegistriesService(s)
	return rs
}

type ProjectsLocationsService struct {
	s *Service

	Registries *ProjectsLocationsRegistriesService
}

func NewProjectsLocationsRegistriesService(s *Service) *ProjectsLocationsRegistriesService {
	rs := &ProjectsLocationsRegistriesService{s: s}
	rs.Devices = NewProjectsLocationsRegistriesDevicesService(s)
	return rs
}

type ProjectsLocationsRegistriesService struct {
	s *Service

	Devices *ProjectsLocationsRegistriesDevicesService
}

func NewProjectsLocationsRegistriesDevicesService(s *Service) *ProjectsLocationsRegistriesDevicesService {
	rs := &ProjectsLocationsRegistriesDevicesService{s: s}
	rs.ConfigVersions = NewProjectsLocationsRegistriesDevicesConfigVersionsService(s)
	rs.States = NewProjectsLocationsRegistriesDevicesStatesService(s)
	return rs
}

type ProjectsLocationsRegistriesDevicesService struct {
	s *Service

	ConfigVersions *ProjectsLocationsRegistriesDevicesConfigVersionsService

	States *ProjectsLocationsRegistriesDevicesStatesService
}

func NewProjectsLocationsRegistriesDevicesConfigVersionsService(s *Service) *ProjectsLocationsRegistriesDevicesConfigVersionsService {
	rs := &ProjectsLocationsRegistriesDevicesConfigVersionsService{s: s}
	return rs
}

type ProjectsLocationsRegistriesDevicesConfigVersionsService struct {
	s *Service
}

func NewProjectsLocationsRegistriesDevicesStatesService(s *Service) *ProjectsLocationsRegistriesDevicesStatesService {
	rs := &ProjectsLocationsRegistriesDevicesStatesService{s: s}
	return rs
}

type ProjectsLocationsRegistriesDevicesStatesService struct {
	s *Service
}

// Binding: Associates `members` with a `role`.
type Binding struct {
	// Members: Specifies the identities requesting access for a Cloud
	// Platform resource.
	// `members` can have the following values:
	//
	// * `allUsers`: A special identifier that represents anyone who is
	//    on the internet; with or without a Google account.
	//
	// * `allAuthenticatedUsers`: A special identifier that represents
	// anyone
	//    who is authenticated with a Google account or a service
	// account.
	//
	// * `user:{emailid}`: An email address that represents a specific
	// Google
	//    account. For example, `alice@gmail.com` .
	//
	//
	// * `serviceAccount:{emailid}`: An email address that represents a
	// service
	//    account. For example,
	// `my-other-app@appspot.gserviceaccount.com`.
	//
	// * `group:{emailid}`: An email address that represents a Google
	// group.
	//    For example, `admins@example.com`.
	//
	//
	// * `domain:{domain}`: A Google Apps domain name that represents all
	// the
	//    users of that domain. For example, `google.com` or
	// `example.com`.
	//
	//
	Members []string `json:"members,omitempty"`

	// Role: Role that is assigned to `members`.
	// For example, `roles/viewer`, `roles/editor`, or
	// `roles/owner`.
	// Required
	Role string `json:"role,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Members") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Members") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *Binding) MarshalJSON() ([]byte, error) {
	type NoMethod Binding
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Device: The device resource.
type Device struct {
	// Blocked: If a device is blocked, connections or requests from this
	// device will fail.
	// Can be used to temporarily prevent the device from connecting if,
	// for
	// example, the sensor is generating bad data and needs maintenance.
	Blocked bool `json:"blocked,omitempty"`

	// Config: The most recent device configuration, which is eventually
	// sent from
	// Cloud IoT Core to the device. If not present on creation,
	// the
	// configuration will be initialized with an empty payload and version
	// value
	// of `1`. To update this field after creation, use
	// the
	// `DeviceManager.ModifyCloudToDeviceConfig` method.
	Config *DeviceConfig `json:"config,omitempty"`

	// Credentials: The credentials used to authenticate this device. To
	// allow credential
	// rotation without interruption, multiple device credentials can be
	// bound to
	// this device. No more than 3 credentials can be bound to a single
	// device at
	// a time. When new credentials are added to a device, they are
	// verified
	// against the registry credentials. For details, see the description of
	// the
	// `DeviceRegistry.credentials` field.
	Credentials []*DeviceCredential `json:"credentials,omitempty"`

	// Id: The user-defined device identifier. The device ID must be
	// unique
	// within a device registry.
	Id string `json:"id,omitempty"`

	// LastConfigAckTime: [Output only] The last time a cloud-to-device
	// config version acknowledgment
	// was received from the device. This field is only for
	// configurations
	// sent through MQTT.
	LastConfigAckTime string `json:"lastConfigAckTime,omitempty"`

	// LastConfigSendTime: [Output only] The last time a cloud-to-device
	// config version was sent to
	// the device.
	LastConfigSendTime string `json:"lastConfigSendTime,omitempty"`

	// LastErrorStatus: [Output only] The error message of the most recent
	// error, such as a failure
	// to publish to Cloud Pub/Sub. 'last_error_time' is the timestamp of
	// this
	// field. If no errors have occurred, this field has an empty
	// message
	// and the status code 0 == OK. Otherwise, this field is expected to
	// have a
	// status code other than OK.
	LastErrorStatus *Status `json:"lastErrorStatus,omitempty"`

	// LastErrorTime: [Output only] The time the most recent error occurred,
	// such as a failure to
	// publish to Cloud Pub/Sub. This field is the timestamp
	// of
	// 'last_error_status'.
	LastErrorTime string `json:"lastErrorTime,omitempty"`

	// LastEventTime: [Output only] The last time a telemetry event was
	// received. Timestamps are
	// periodically collected and written to storage; they may be stale by a
	// few
	// minutes.
	LastEventTime string `json:"lastEventTime,omitempty"`

	// LastHeartbeatTime: [Output only] The last time an MQTT `PINGREQ` was
	// received. This field
	// applies only to devices connecting through MQTT. MQTT clients usually
	// only
	// send `PINGREQ` messages if the connection is idle, and no other
	// messages
	// have been sent. Timestamps are periodically collected and written
	// to
	// storage; they may be stale by a few minutes.
	LastHeartbeatTime string `json:"lastHeartbeatTime,omitempty"`

	// LastStateTime: [Output only] The last time a state event was
	// received. Timestamps are
	// periodically collected and written to storage; they may be stale by a
	// few
	// minutes.
	LastStateTime string `json:"lastStateTime,omitempty"`

	// Metadata: The metadata key-value pairs assigned to the device. This
	// metadata is not
	// interpreted or indexed by Cloud IoT Core. It can be used to add
	// contextual
	// information for the device.
	//
	// Keys must conform to the regular expression a-zA-Z+ and
	// be less than 128 bytes in length.
	//
	// Values are free-form strings. Each value must be less than or equal
	// to 32
	// KB in size.
	//
	// The total size of all keys and values must be less than 256 KB, and
	// the
	// maximum number of key-value pairs is 500.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Name: The resource path name. For
	// example,
	// `projects/p1/locations/us-central1/registries/registry0/devic
	// es/dev0`
	// or
	// `projects/p1/locations/us-central1/registries/registry0/devices/{nu
	// m_id}`.
	// When `name` is populated as a response from the service, it always
	// ends
	// in the device numeric ID.
	Name string `json:"name,omitempty"`

	// NumId: [Output only] A server-defined unique numeric ID for the
	// device. This is a
	// more compact way to identify devices, and it is globally unique.
	NumId uint64 `json:"numId,omitempty,string"`

	// State: [Output only] The state most recently received from the
	// device. If no state
	// has been reported, this field is not present.
	State *DeviceState `json:"state,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Blocked") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Blocked") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *Device) MarshalJSON() ([]byte, error) {
	type NoMethod Device
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// DeviceConfig: The device configuration. Eventually delivered to
// devices.
type DeviceConfig struct {
	// BinaryData: The device configuration data.
	BinaryData string `json:"binaryData,omitempty"`

	// CloudUpdateTime: [Output only] The time at which this configuration
	// version was updated in
	// Cloud IoT Core. This timestamp is set by the server.
	CloudUpdateTime string `json:"cloudUpdateTime,omitempty"`

	// DeviceAckTime: [Output only] The time at which Cloud IoT Core
	// received the
	// acknowledgment from the device, indicating that the device has
	// received
	// this configuration version. If this field is not present, the device
	// has
	// not yet acknowledged that it received this version. Note that
	// when
	// the config was sent to the device, many config versions may have
	// been
	// available in Cloud IoT Core while the device was disconnected, and
	// on
	// connection, only the latest version is sent to the device.
	// Some
	// versions may never be sent to the device, and therefore are
	// never
	// acknowledged. This timestamp is set by Cloud IoT Core.
	DeviceAckTime string `json:"deviceAckTime,omitempty"`

	// Version: [Output only] The version of this update. The version number
	// is assigned by
	// the server, and is always greater than 0 after device creation.
	// The
	// version must be 0 on the `CreateDevice` request if a `config`
	// is
	// specified; the response of `CreateDevice` will always have a value of
	// 1.
	Version int64 `json:"version,omitempty,string"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "BinaryData") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BinaryData") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *DeviceConfig) MarshalJSON() ([]byte, error) {
	type NoMethod DeviceConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// DeviceCredential: A server-stored device credential used for
// authentication.
type DeviceCredential struct {
	// ExpirationTime: [Optional] The time at which this credential becomes
	// invalid. This
	// credential will be ignored for new client authentication requests
	// after
	// this timestamp; however, it will not be automatically deleted.
	ExpirationTime string `json:"expirationTime,omitempty"`

	// PublicKey: A public key used to verify the signature of JSON Web
	// Tokens (JWTs).
	// When adding a new device credential, either via device creation or
	// via
	// modifications, this public key credential may be required to be
	// signed by
	// one of the registry level certificates. More specifically, if
	// the
	// registry contains at least one certificate, any new device
	// credential
	// must be signed by one of the registry certificates. As a result,
	// when the registry contains certificates, only X.509 certificates
	// are
	// accepted as device credentials. However, if the registry does
	// not contain a certificate, self-signed certificates and public keys
	// will
	// be accepted. New device credentials must be different from
	// every
	// registry-level certificate.
	PublicKey *PublicKeyCredential `json:"publicKey,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ExpirationTime") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ExpirationTime") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *DeviceCredential) MarshalJSON() ([]byte, error) {
	type NoMethod DeviceCredential
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// DeviceRegistry: A container for a group of devices.
type DeviceRegistry struct {
	// Credentials: The credentials used to verify the device credentials.
	// No more than 10
	// credentials can be bound to a single registry at a time. The
	// verification
	// process occurs at the time of device creation or update. If this
	// field is
	// empty, no verification is performed. Otherwise, the credentials of a
	// newly
	// created device or added credentials of an updated device should be
	// signed
	// with one of these registry credentials.
	//
	// Note, however, that existing devices will never be affected
	// by
	// modifications to this list of credentials: after a device has
	// been
	// successfully created in a registry, it should be able to connect even
	// if
	// its registry credentials are revoked, deleted, or modified.
	Credentials []*RegistryCredential `json:"credentials,omitempty"`

	// EventNotificationConfigs: The configuration for notification of
	// telemetry events received from the
	// device. All telemetry events that were successfully published by
	// the
	// device and acknowledged by Cloud IoT Core are guaranteed to
	// be
	// delivered to Cloud Pub/Sub. If multiple configurations match a
	// message,
	// only the first matching configuration is used. If you try to publish
	// a
	// device telemetry event using MQTT without specifying a Cloud Pub/Sub
	// topic
	// for the device's registry, the connection closes automatically. If
	// you try
	// to do so using an HTTP connection, an error is returned. Up to
	// 10
	// configurations may be provided.
	EventNotificationConfigs []*EventNotificationConfig `json:"eventNotificationConfigs,omitempty"`

	// HttpConfig: The DeviceService (HTTP) configuration for this device
	// registry.
	HttpConfig *HttpConfig `json:"httpConfig,omitempty"`

	// Id: The identifier of this device registry. For example,
	// `myRegistry`.
	Id string `json:"id,omitempty"`

	// MqttConfig: The MQTT configuration for this device registry.
	MqttConfig *MqttConfig `json:"mqttConfig,omitempty"`

	// Name: The resource path name. For
	// example,
	// `projects/example-project/locations/us-central1/registries/my
	// -registry`.
	Name string `json:"name,omitempty"`

	// StateNotificationConfig: The configuration for notification of new
	// states received from the device.
	// State updates are guaranteed to be stored in the state history,
	// but
	// notifications to Cloud Pub/Sub are not guaranteed. For example,
	// if
	// permissions are misconfigured or the specified topic doesn't exist,
	// no
	// notification will be published but the state will still be stored in
	// Cloud
	// IoT Core.
	StateNotificationConfig *StateNotificationConfig `json:"stateNotificationConfig,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Credentials") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Credentials") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *DeviceRegistry) MarshalJSON() ([]byte, error) {
	type NoMethod DeviceRegistry
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// DeviceState: The device state, as reported by the device.
type DeviceState struct {
	// BinaryData: The device state data.
	BinaryData string `json:"binaryData,omitempty"`

	// UpdateTime: [Output only] The time at which this state version was
	// updated in Cloud
	// IoT Core.
	UpdateTime string `json:"updateTime,omitempty"`

	// ForceSendFields is a list of field names (e.g. "BinaryData") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BinaryData") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *DeviceState) MarshalJSON() ([]byte, error) {
	type NoMethod DeviceState
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Empty: A generic empty message that you can re-use to avoid defining
// duplicated
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
type Empty struct {
	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`
}

// EventNotificationConfig: The configuration for forwarding telemetry
// events.
type EventNotificationConfig struct {
	// PubsubTopicName: A Cloud Pub/Sub topic name. For
	// example,
	// `projects/myProject/topics/deviceEvents`.
	PubsubTopicName string `json:"pubsubTopicName,omitempty"`

	// SubfolderMatches: If the subfolder name matches this string exactly,
	// this configuration will
	// be used. The string must not include the leading '/' character. If
	// empty,
	// all strings are matched. This field is used only for telemetry
	// events;
	// subfolders are not supported for state changes.
	SubfolderMatches string `json:"subfolderMatches,omitempty"`

	// ForceSendFields is a list of field names (e.g. "PubsubTopicName") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "PubsubTopicName") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *EventNotificationConfig) MarshalJSON() ([]byte, error) {
	type NoMethod EventNotificationConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// GetIamPolicyRequest: Request message for `GetIamPolicy` method.
type GetIamPolicyRequest struct {
}

// HttpConfig: The configuration of the HTTP bridge for a device
// registry.
type HttpConfig struct {
	// HttpEnabledState: If enabled, allows devices to use DeviceService via
	// the HTTP protocol.
	// Otherwise, any requests to DeviceService will fail for this registry.
	//
	// Possible values:
	//   "HTTP_STATE_UNSPECIFIED" - No HTTP state specified. If not
	// specified, DeviceService will be
	// enabled by default.
	//   "HTTP_ENABLED" - Enables DeviceService (HTTP) service for the
	// registry.
	//   "HTTP_DISABLED" - Disables DeviceService (HTTP) service for the
	// registry.
	HttpEnabledState string `json:"httpEnabledState,omitempty"`

	// ForceSendFields is a list of field names (e.g. "HttpEnabledState") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "HttpEnabledState") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *HttpConfig) MarshalJSON() ([]byte, error) {
	type NoMethod HttpConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// ListDeviceConfigVersionsResponse: Response for
// `ListDeviceConfigVersions`.
type ListDeviceConfigVersionsResponse struct {
	// DeviceConfigs: The device configuration for the last few versions.
	// Versions are listed
	// in decreasing order, starting from the most recent one.
	DeviceConfigs []*DeviceConfig `json:"deviceConfigs,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "DeviceConfigs") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DeviceConfigs") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *ListDeviceConfigVersionsResponse) MarshalJSON() ([]byte, error) {
	type NoMethod ListDeviceConfigVersionsResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// ListDeviceRegistriesResponse: Response for `ListDeviceRegistries`.
type ListDeviceRegistriesResponse struct {
	// DeviceRegistries: The registries that matched the query.
	DeviceRegistries []*DeviceRegistry `json:"deviceRegistries,omitempty"`

	// NextPageToken: If not empty, indicates that there may be more
	// registries that match the
	// request; this value should be passed in a
	// new
	// `ListDeviceRegistriesRequest`.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "DeviceRegistries") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DeviceRegistries") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *ListDeviceRegistriesResponse) MarshalJSON() ([]byte, error) {
	type NoMethod ListDeviceRegistriesResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// ListDeviceStatesResponse: Response for `ListDeviceStates`.
type ListDeviceStatesResponse struct {
	// DeviceStates: The last few device states. States are listed in
	// descending order of server
	// update time, starting from the most recent one.
	DeviceStates []*DeviceState `json:"deviceStates,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "DeviceStates") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "DeviceStates") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *ListDeviceStatesResponse) MarshalJSON() ([]byte, error) {
	type NoMethod ListDeviceStatesResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// ListDevicesResponse: Response for `ListDevices`.
type ListDevicesResponse struct {
	// Devices: The devices that match the request.
	Devices []*Device `json:"devices,omitempty"`

	// NextPageToken: If not empty, indicates that there may be more devices
	// that match the
	// request; this value should be passed in a new `ListDevicesRequest`.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Devices") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Devices") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *ListDevicesResponse) MarshalJSON() ([]byte, error) {
	type NoMethod ListDevicesResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// ModifyCloudToDeviceConfigRequest: Request for
// `ModifyCloudToDeviceConfig`.
type ModifyCloudToDeviceConfigRequest struct {
	// BinaryData: The configuration data for the device.
	BinaryData string `json:"binaryData,omitempty"`

	// VersionToUpdate: The version number to update. If this value is zero,
	// it will not check the
	// version number of the server and will always update the current
	// version;
	// otherwise, this update will fail if the version number found on the
	// server
	// does not match this version number. This is used to support
	// multiple
	// simultaneous updates without losing data.
	VersionToUpdate int64 `json:"versionToUpdate,omitempty,string"`

	// ForceSendFields is a list of field names (e.g. "BinaryData") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "BinaryData") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *ModifyCloudToDeviceConfigRequest) MarshalJSON() ([]byte, error) {
	type NoMethod ModifyCloudToDeviceConfigRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// MqttConfig: The configuration of MQTT for a device registry.
type MqttConfig struct {
	// MqttEnabledState: If enabled, allows connections using the MQTT
	// protocol. Otherwise, MQTT
	// connections to this registry will fail.
	//
	// Possible values:
	//   "MQTT_STATE_UNSPECIFIED" - No MQTT state specified. If not
	// specified, MQTT will be enabled by default.
	//   "MQTT_ENABLED" - Enables a MQTT connection.
	//   "MQTT_DISABLED" - Disables a MQTT connection.
	MqttEnabledState string `json:"mqttEnabledState,omitempty"`

	// ForceSendFields is a list of field names (e.g. "MqttEnabledState") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "MqttEnabledState") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *MqttConfig) MarshalJSON() ([]byte, error) {
	type NoMethod MqttConfig
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// Policy: Defines an Identity and Access Management (IAM) policy. It is
// used to
// specify access control policies for Cloud Platform resources.
//
//
// A `Policy` consists of a list of `bindings`. A `binding` binds a list
// of
// `members` to a `role`, where the members can be user accounts, Google
// groups,
// Google domains, and service accounts. A `role` is a named list of
// permissions
// defined by IAM.
//
// **JSON Example**
//
//     {
//       "bindings": [
//         {
//           "role": "roles/owner",
//           "members": [
//             "user:mike@example.com",
//             "group:admins@example.com",
//             "domain:google.com",
//
// "serviceAccount:my-other-app@appspot.gserviceaccount.com"
//           ]
//         },
//         {
//           "role": "roles/viewer",
//           "members": ["user:sean@example.com"]
//         }
//       ]
//     }
//
// **YAML Example**
//
//     bindings:
//     - members:
//       - user:mike@example.com
//       - group:admins@example.com
//       - domain:google.com
//       - serviceAccount:my-other-app@appspot.gserviceaccount.com
//       role: roles/owner
//     - members:
//       - user:sean@example.com
//       role: roles/viewer
//
//
// For a description of IAM and its features, see the
// [IAM developer's guide](https://cloud.google.com/iam/docs).
type Policy struct {
	// Bindings: Associates a list of `members` to a `role`.
	// `bindings` with no members will result in an error.
	Bindings []*Binding `json:"bindings,omitempty"`

	// Etag: `etag` is used for optimistic concurrency control as a way to
	// help
	// prevent simultaneous updates of a policy from overwriting each
	// other.
	// It is strongly suggested that systems make use of the `etag` in
	// the
	// read-modify-write cycle to perform policy updates in order to avoid
	// race
	// conditions: An `etag` is returned in the response to `getIamPolicy`,
	// and
	// systems are expected to put that etag in the request to
	// `setIamPolicy` to
	// ensure that their change will be applied to the same version of the
	// policy.
	//
	// If no `etag` is provided in the call to `setIamPolicy`, then the
	// existing
	// policy is overwritten blindly.
	Etag string `json:"etag,omitempty"`

	// Version: Deprecated.
	Version int64 `json:"version,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Bindings") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Bindings") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *Policy) MarshalJSON() ([]byte, error) {
	type NoMethod Policy
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// PublicKeyCertificate: A public key certificate format and data.
type PublicKeyCertificate struct {
	// Certificate: The certificate data.
	Certificate string `json:"certificate,omitempty"`

	// Format: The certificate format.
	//
	// Possible values:
	//   "UNSPECIFIED_PUBLIC_KEY_CERTIFICATE_FORMAT" - The format has not
	// been specified. This is an invalid default value and
	// must not be used.
	//   "X509_CERTIFICATE_PEM" - An X.509v3 certificate
	// ([RFC5280](https://www.ietf.org/rfc/rfc5280.txt)),
	// encoded in base64, and wrapped by `-----BEGIN CERTIFICATE-----`
	// and
	// `-----END CERTIFICATE-----`.
	Format string `json:"format,omitempty"`

	// X509Details: [Output only] The certificate details. Used only for
	// X.509 certificates.
	X509Details *X509CertificateDetails `json:"x509Details,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Certificate") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Certificate") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *PublicKeyCertificate) MarshalJSON() ([]byte, error) {
	type NoMethod PublicKeyCertificate
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// PublicKeyCredential: A public key format and data.
type PublicKeyCredential struct {
	// Format: The format of the key.
	//
	// Possible values:
	//   "UNSPECIFIED_PUBLIC_KEY_FORMAT" - The format has not been
	// specified. This is an invalid default value and
	// must not be used.
	//   "RSA_PEM" - An RSA public key encoded in base64, and wrapped
	// by
	// `-----BEGIN PUBLIC KEY-----` and `-----END PUBLIC KEY-----`. This can
	// be
	// used to verify `RS256` signatures in JWT tokens
	// ([RFC7518](
	// https://www.ietf.org/rfc/rfc7518.txt)).
	//   "RSA_X509_PEM" - As RSA_PEM, but wrapped in an X.509v3 certificate
	// ([RFC5280](
	// https://www.ietf.org/rfc/rfc5280.txt)), encoded in base64, and
	// wrapped by
	// `-----BEGIN CERTIFICATE-----` and `-----END CERTIFICATE-----`.
	//   "ES256_PEM" - Public key for the ECDSA algorithm using P-256 and
	// SHA-256, encoded in
	// base64, and wrapped by `-----BEGIN PUBLIC KEY-----` and
	// `-----END
	// PUBLIC KEY-----`. This can be used to verify JWT tokens with the
	// `ES256`
	// algorithm ([RFC7518](https://www.ietf.org/rfc/rfc7518.txt)). This
	// curve is
	// defined in [OpenSSL](https://www.openssl.org/) as the `prime256v1`
	// curve.
	//   "ES256_X509_PEM" - As ES256_PEM, but wrapped in an X.509v3
	// certificate ([RFC5280](
	// https://www.ietf.org/rfc/rfc5280.txt)), encoded in base64, and
	// wrapped by
	// `-----BEGIN CERTIFICATE-----` and `-----END CERTIFICATE-----`.
	Format string `json:"format,omitempty"`

	// Key: The key data.
	Key string `json:"key,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Format") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Format") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *PublicKeyCredential) MarshalJSON() ([]byte, error) {
	type NoMethod PublicKeyCredential
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// RegistryCredential: A server-stored registry credential used to
// validate device credentials.
type RegistryCredential struct {
	// PublicKeyCertificate: A public key certificate used to verify the
	// device credentials.
	PublicKeyCertificate *PublicKeyCertificate `json:"publicKeyCertificate,omitempty"`

	// ForceSendFields is a list of field names (e.g.
	// "PublicKeyCertificate") to unconditionally include in API requests.
	// By default, fields with empty values are omitted from API requests.
	// However, any non-pointer, non-interface field appearing in
	// ForceSendFields will be sent to the server regardless of whether the
	// field is empty or not. This may be used to include empty fields in
	// Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "PublicKeyCertificate") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *RegistryCredential) MarshalJSON() ([]byte, error) {
	type NoMethod RegistryCredential
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// SetIamPolicyRequest: Request message for `SetIamPolicy` method.
type SetIamPolicyRequest struct {
	// Policy: REQUIRED: The complete policy to be applied to the
	// `resource`. The size of
	// the policy is limited to a few 10s of KB. An empty policy is a
	// valid policy but certain Cloud Platform services (such as
	// Projects)
	// might reject them.
	Policy *Policy `json:"policy,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Policy") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Policy") to include in API
	// requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *SetIamPolicyRequest) MarshalJSON() ([]byte, error) {
	type NoMethod SetIamPolicyRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// StateNotificationConfig: The configuration for notification of new
// states received from the device.
type StateNotificationConfig struct {
	// PubsubTopicName: A Cloud Pub/Sub topic name. For
	// example,
	// `projects/myProject/topics/deviceEvents`.
	PubsubTopicName string `json:"pubsubTopicName,omitempty"`

	// ForceSendFields is a list of field names (e.g. "PubsubTopicName") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "PubsubTopicName") to
	// include in API requests with the JSON null value. By default, fields
	// with empty values are omitted from API requests. However, any field
	// with an empty value appearing in NullFields will be sent to the
	// server as null. It is an error if a field in this list has a
	// non-empty value. This may be used to include null fields in Patch
	// requests.
	NullFields []string `json:"-"`
}

func (s *StateNotificationConfig) MarshalJSON() ([]byte, error) {
	type NoMethod StateNotificationConfig
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

// TestIamPermissionsRequest: Request message for `TestIamPermissions`
// method.
type TestIamPermissionsRequest struct {
	// Permissions: The set of permissions to check for the `resource`.
	// Permissions with
	// wildcards (such as '*' or 'storage.*') are not allowed. For
	// more
	// information see
	// [IAM
	// Overview](https://cloud.google.com/iam/docs/overview#permissions).
	Permissions []string `json:"permissions,omitempty"`

	// ForceSendFields is a list of field names (e.g. "Permissions") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Permissions") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *TestIamPermissionsRequest) MarshalJSON() ([]byte, error) {
	type NoMethod TestIamPermissionsRequest
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// TestIamPermissionsResponse: Response message for `TestIamPermissions`
// method.
type TestIamPermissionsResponse struct {
	// Permissions: A subset of `TestPermissionsRequest.permissions` that
	// the caller is
	// allowed.
	Permissions []string `json:"permissions,omitempty"`

	// ServerResponse contains the HTTP response code and headers from the
	// server.
	googleapi.ServerResponse `json:"-"`

	// ForceSendFields is a list of field names (e.g. "Permissions") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "Permissions") to include
	// in API requests with the JSON null value. By default, fields with
	// empty values are omitted from API requests. However, any field with
	// an empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *TestIamPermissionsResponse) MarshalJSON() ([]byte, error) {
	type NoMethod TestIamPermissionsResponse
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// X509CertificateDetails: Details of an X.509 certificate. For
// informational purposes only.
type X509CertificateDetails struct {
	// ExpiryTime: The time the certificate becomes invalid.
	ExpiryTime string `json:"expiryTime,omitempty"`

	// Issuer: The entity that signed the certificate.
	Issuer string `json:"issuer,omitempty"`

	// PublicKeyType: The type of public key in the certificate.
	PublicKeyType string `json:"publicKeyType,omitempty"`

	// SignatureAlgorithm: The algorithm used to sign the certificate.
	SignatureAlgorithm string `json:"signatureAlgorithm,omitempty"`

	// StartTime: The time the certificate becomes valid.
	StartTime string `json:"startTime,omitempty"`

	// Subject: The entity the certificate and public key belong to.
	Subject string `json:"subject,omitempty"`

	// ForceSendFields is a list of field names (e.g. "ExpiryTime") to
	// unconditionally include in API requests. By default, fields with
	// empty values are omitted from API requests. However, any non-pointer,
	// non-interface field appearing in ForceSendFields will be sent to the
	// server regardless of whether the field is empty or not. This may be
	// used to include empty fields in Patch requests.
	ForceSendFields []string `json:"-"`

	// NullFields is a list of field names (e.g. "ExpiryTime") to include in
	// API requests with the JSON null value. By default, fields with empty
	// values are omitted from API requests. However, any field with an
	// empty value appearing in NullFields will be sent to the server as
	// null. It is an error if a field in this list has a non-empty value.
	// This may be used to include null fields in Patch requests.
	NullFields []string `json:"-"`
}

func (s *X509CertificateDetails) MarshalJSON() ([]byte, error) {
	type NoMethod X509CertificateDetails
	raw := NoMethod(*s)
	return gensupport.MarshalJSON(raw, s.ForceSendFields, s.NullFields)
}

// method id "cloudiot.projects.locations.registries.create":

type ProjectsLocationsRegistriesCreateCall struct {
	s              *Service
	parent         string
	deviceregistry *DeviceRegistry
	urlParams_     gensupport.URLParams
	ctx_           context.Context
	header_        http.Header
}

// Create: Creates a device registry that contains devices.
func (r *ProjectsLocationsRegistriesService) Create(parent string, deviceregistry *DeviceRegistry) *ProjectsLocationsRegistriesCreateCall {
	c := &ProjectsLocationsRegistriesCreateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.deviceregistry = deviceregistry
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesCreateCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesCreateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesCreateCall) Context(ctx context.Context) *ProjectsLocationsRegistriesCreateCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesCreateCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesCreateCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.deviceregistry)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+parent}/registries")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudiot.projects.locations.registries.create" call.
// Exactly one of *DeviceRegistry or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *DeviceRegistry.ServerResponse.Header or (if a response was returned
// at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsRegistriesCreateCall) Do(opts ...googleapi.CallOption) (*DeviceRegistry, error) {
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
	ret := &DeviceRegistry{
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
	//   "description": "Creates a device registry that contains devices.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries",
	//   "httpMethod": "POST",
	//   "id": "cloudiot.projects.locations.registries.create",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The project and cloud region where this device registry must be created.\nFor example, `projects/example-project/locations/us-central1`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+parent}/registries",
	//   "request": {
	//     "$ref": "DeviceRegistry"
	//   },
	//   "response": {
	//     "$ref": "DeviceRegistry"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.delete":

type ProjectsLocationsRegistriesDeleteCall struct {
	s          *Service
	name       string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
	header_    http.Header
}

// Delete: Deletes a device registry configuration.
func (r *ProjectsLocationsRegistriesService) Delete(name string) *ProjectsLocationsRegistriesDeleteCall {
	c := &ProjectsLocationsRegistriesDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesDeleteCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesDeleteCall) Context(ctx context.Context) *ProjectsLocationsRegistriesDeleteCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesDeleteCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesDeleteCall) doRequest(alt string) (*http.Response, error) {
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

// Do executes the "cloudiot.projects.locations.registries.delete" call.
// Exactly one of *Empty or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Empty.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *ProjectsLocationsRegistriesDeleteCall) Do(opts ...googleapi.CallOption) (*Empty, error) {
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
	ret := &Empty{
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
	//   "description": "Deletes a device registry configuration.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}",
	//   "httpMethod": "DELETE",
	//   "id": "cloudiot.projects.locations.registries.delete",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the device registry. For example,\n`projects/example-project/locations/us-central1/registries/my-registry`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}",
	//   "response": {
	//     "$ref": "Empty"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.get":

type ProjectsLocationsRegistriesGetCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// Get: Gets a device registry configuration.
func (r *ProjectsLocationsRegistriesService) Get(name string) *ProjectsLocationsRegistriesGetCall {
	c := &ProjectsLocationsRegistriesGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesGetCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsLocationsRegistriesGetCall) IfNoneMatch(entityTag string) *ProjectsLocationsRegistriesGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesGetCall) Context(ctx context.Context) *ProjectsLocationsRegistriesGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesGetCall) doRequest(alt string) (*http.Response, error) {
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

// Do executes the "cloudiot.projects.locations.registries.get" call.
// Exactly one of *DeviceRegistry or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *DeviceRegistry.ServerResponse.Header or (if a response was returned
// at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsRegistriesGetCall) Do(opts ...googleapi.CallOption) (*DeviceRegistry, error) {
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
	ret := &DeviceRegistry{
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
	//   "description": "Gets a device registry configuration.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}",
	//   "httpMethod": "GET",
	//   "id": "cloudiot.projects.locations.registries.get",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the device registry. For example,\n`projects/example-project/locations/us-central1/registries/my-registry`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}",
	//   "response": {
	//     "$ref": "DeviceRegistry"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.getIamPolicy":

type ProjectsLocationsRegistriesGetIamPolicyCall struct {
	s                   *Service
	resource            string
	getiampolicyrequest *GetIamPolicyRequest
	urlParams_          gensupport.URLParams
	ctx_                context.Context
	header_             http.Header
}

// GetIamPolicy: Gets the access control policy for a resource.
// Returns an empty policy if the resource exists and does not have a
// policy
// set.
func (r *ProjectsLocationsRegistriesService) GetIamPolicy(resource string, getiampolicyrequest *GetIamPolicyRequest) *ProjectsLocationsRegistriesGetIamPolicyCall {
	c := &ProjectsLocationsRegistriesGetIamPolicyCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resource = resource
	c.getiampolicyrequest = getiampolicyrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesGetIamPolicyCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesGetIamPolicyCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesGetIamPolicyCall) Context(ctx context.Context) *ProjectsLocationsRegistriesGetIamPolicyCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesGetIamPolicyCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesGetIamPolicyCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.getiampolicyrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+resource}:getIamPolicy")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"resource": c.resource,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudiot.projects.locations.registries.getIamPolicy" call.
// Exactly one of *Policy or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Policy.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *ProjectsLocationsRegistriesGetIamPolicyCall) Do(opts ...googleapi.CallOption) (*Policy, error) {
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
	ret := &Policy{
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
	//   "description": "Gets the access control policy for a resource.\nReturns an empty policy if the resource exists and does not have a policy\nset.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}:getIamPolicy",
	//   "httpMethod": "POST",
	//   "id": "cloudiot.projects.locations.registries.getIamPolicy",
	//   "parameterOrder": [
	//     "resource"
	//   ],
	//   "parameters": {
	//     "resource": {
	//       "description": "REQUIRED: The resource for which the policy is being requested.\nSee the operation documentation for the appropriate value for this field.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+resource}:getIamPolicy",
	//   "request": {
	//     "$ref": "GetIamPolicyRequest"
	//   },
	//   "response": {
	//     "$ref": "Policy"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.list":

type ProjectsLocationsRegistriesListCall struct {
	s            *Service
	parent       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists device registries.
func (r *ProjectsLocationsRegistriesService) List(parent string) *ProjectsLocationsRegistriesListCall {
	c := &ProjectsLocationsRegistriesListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	return c
}

// PageSize sets the optional parameter "pageSize": The maximum number
// of registries to return in the response. If this value
// is zero, the service will select a default size. A call may return
// fewer
// objects than requested, but if there is a non-empty `page_token`,
// it
// indicates that more entries are available.
func (c *ProjectsLocationsRegistriesListCall) PageSize(pageSize int64) *ProjectsLocationsRegistriesListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": The value returned
// by the last `ListDeviceRegistriesResponse`; indicates
// that this is a continuation of a prior `ListDeviceRegistries` call,
// and
// that the system should return the next page of data.
func (c *ProjectsLocationsRegistriesListCall) PageToken(pageToken string) *ProjectsLocationsRegistriesListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesListCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsLocationsRegistriesListCall) IfNoneMatch(entityTag string) *ProjectsLocationsRegistriesListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesListCall) Context(ctx context.Context) *ProjectsLocationsRegistriesListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+parent}/registries")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudiot.projects.locations.registries.list" call.
// Exactly one of *ListDeviceRegistriesResponse or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *ListDeviceRegistriesResponse.ServerResponse.Header or (if a
// response was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsRegistriesListCall) Do(opts ...googleapi.CallOption) (*ListDeviceRegistriesResponse, error) {
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
	ret := &ListDeviceRegistriesResponse{
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
	//   "description": "Lists device registries.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries",
	//   "httpMethod": "GET",
	//   "id": "cloudiot.projects.locations.registries.list",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "pageSize": {
	//       "description": "The maximum number of registries to return in the response. If this value\nis zero, the service will select a default size. A call may return fewer\nobjects than requested, but if there is a non-empty `page_token`, it\nindicates that more entries are available.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "description": "The value returned by the last `ListDeviceRegistriesResponse`; indicates\nthat this is a continuation of a prior `ListDeviceRegistries` call, and\nthat the system should return the next page of data.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "parent": {
	//       "description": "The project and cloud region path. For example,\n`projects/example-project/locations/us-central1`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+parent}/registries",
	//   "response": {
	//     "$ref": "ListDeviceRegistriesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *ProjectsLocationsRegistriesListCall) Pages(ctx context.Context, f func(*ListDeviceRegistriesResponse) error) error {
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

// method id "cloudiot.projects.locations.registries.patch":

type ProjectsLocationsRegistriesPatchCall struct {
	s              *Service
	name           string
	deviceregistry *DeviceRegistry
	urlParams_     gensupport.URLParams
	ctx_           context.Context
	header_        http.Header
}

// Patch: Updates a device registry configuration.
func (r *ProjectsLocationsRegistriesService) Patch(name string, deviceregistry *DeviceRegistry) *ProjectsLocationsRegistriesPatchCall {
	c := &ProjectsLocationsRegistriesPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.deviceregistry = deviceregistry
	return c
}

// UpdateMask sets the optional parameter "updateMask": Only updates the
// `device_registry` fields indicated by this mask.
// The field mask must not be empty, and it must not contain fields
// that
// are immutable or only set by the server.
// Mutable top-level fields: `event_notification_config`,
// `http_config`,
// `mqtt_config`, and `state_notification_config`.
func (c *ProjectsLocationsRegistriesPatchCall) UpdateMask(updateMask string) *ProjectsLocationsRegistriesPatchCall {
	c.urlParams_.Set("updateMask", updateMask)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesPatchCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesPatchCall) Context(ctx context.Context) *ProjectsLocationsRegistriesPatchCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesPatchCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesPatchCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.deviceregistry)
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

// Do executes the "cloudiot.projects.locations.registries.patch" call.
// Exactly one of *DeviceRegistry or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *DeviceRegistry.ServerResponse.Header or (if a response was returned
// at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsRegistriesPatchCall) Do(opts ...googleapi.CallOption) (*DeviceRegistry, error) {
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
	ret := &DeviceRegistry{
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
	//   "description": "Updates a device registry configuration.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}",
	//   "httpMethod": "PATCH",
	//   "id": "cloudiot.projects.locations.registries.patch",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The resource path name. For example,\n`projects/example-project/locations/us-central1/registries/my-registry`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "updateMask": {
	//       "description": "Only updates the `device_registry` fields indicated by this mask.\nThe field mask must not be empty, and it must not contain fields that\nare immutable or only set by the server.\nMutable top-level fields: `event_notification_config`, `http_config`,\n`mqtt_config`, and `state_notification_config`.",
	//       "format": "google-fieldmask",
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}",
	//   "request": {
	//     "$ref": "DeviceRegistry"
	//   },
	//   "response": {
	//     "$ref": "DeviceRegistry"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.setIamPolicy":

type ProjectsLocationsRegistriesSetIamPolicyCall struct {
	s                   *Service
	resource            string
	setiampolicyrequest *SetIamPolicyRequest
	urlParams_          gensupport.URLParams
	ctx_                context.Context
	header_             http.Header
}

// SetIamPolicy: Sets the access control policy on the specified
// resource. Replaces any
// existing policy.
func (r *ProjectsLocationsRegistriesService) SetIamPolicy(resource string, setiampolicyrequest *SetIamPolicyRequest) *ProjectsLocationsRegistriesSetIamPolicyCall {
	c := &ProjectsLocationsRegistriesSetIamPolicyCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resource = resource
	c.setiampolicyrequest = setiampolicyrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesSetIamPolicyCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesSetIamPolicyCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesSetIamPolicyCall) Context(ctx context.Context) *ProjectsLocationsRegistriesSetIamPolicyCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesSetIamPolicyCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesSetIamPolicyCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.setiampolicyrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+resource}:setIamPolicy")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"resource": c.resource,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudiot.projects.locations.registries.setIamPolicy" call.
// Exactly one of *Policy or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Policy.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *ProjectsLocationsRegistriesSetIamPolicyCall) Do(opts ...googleapi.CallOption) (*Policy, error) {
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
	ret := &Policy{
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
	//   "description": "Sets the access control policy on the specified resource. Replaces any\nexisting policy.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}:setIamPolicy",
	//   "httpMethod": "POST",
	//   "id": "cloudiot.projects.locations.registries.setIamPolicy",
	//   "parameterOrder": [
	//     "resource"
	//   ],
	//   "parameters": {
	//     "resource": {
	//       "description": "REQUIRED: The resource for which the policy is being specified.\nSee the operation documentation for the appropriate value for this field.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+resource}:setIamPolicy",
	//   "request": {
	//     "$ref": "SetIamPolicyRequest"
	//   },
	//   "response": {
	//     "$ref": "Policy"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.testIamPermissions":

type ProjectsLocationsRegistriesTestIamPermissionsCall struct {
	s                         *Service
	resource                  string
	testiampermissionsrequest *TestIamPermissionsRequest
	urlParams_                gensupport.URLParams
	ctx_                      context.Context
	header_                   http.Header
}

// TestIamPermissions: Returns permissions that a caller has on the
// specified resource.
// If the resource does not exist, this will return an empty set
// of
// permissions, not a NOT_FOUND error.
func (r *ProjectsLocationsRegistriesService) TestIamPermissions(resource string, testiampermissionsrequest *TestIamPermissionsRequest) *ProjectsLocationsRegistriesTestIamPermissionsCall {
	c := &ProjectsLocationsRegistriesTestIamPermissionsCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.resource = resource
	c.testiampermissionsrequest = testiampermissionsrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesTestIamPermissionsCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesTestIamPermissionsCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesTestIamPermissionsCall) Context(ctx context.Context) *ProjectsLocationsRegistriesTestIamPermissionsCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesTestIamPermissionsCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesTestIamPermissionsCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.testiampermissionsrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+resource}:testIamPermissions")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"resource": c.resource,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudiot.projects.locations.registries.testIamPermissions" call.
// Exactly one of *TestIamPermissionsResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *TestIamPermissionsResponse.ServerResponse.Header or (if a response
// was returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsRegistriesTestIamPermissionsCall) Do(opts ...googleapi.CallOption) (*TestIamPermissionsResponse, error) {
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
	ret := &TestIamPermissionsResponse{
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
	//   "description": "Returns permissions that a caller has on the specified resource.\nIf the resource does not exist, this will return an empty set of\npermissions, not a NOT_FOUND error.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}:testIamPermissions",
	//   "httpMethod": "POST",
	//   "id": "cloudiot.projects.locations.registries.testIamPermissions",
	//   "parameterOrder": [
	//     "resource"
	//   ],
	//   "parameters": {
	//     "resource": {
	//       "description": "REQUIRED: The resource for which the policy detail is being requested.\nSee the operation documentation for the appropriate value for this field.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+resource}:testIamPermissions",
	//   "request": {
	//     "$ref": "TestIamPermissionsRequest"
	//   },
	//   "response": {
	//     "$ref": "TestIamPermissionsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.devices.create":

type ProjectsLocationsRegistriesDevicesCreateCall struct {
	s          *Service
	parent     string
	device     *Device
	urlParams_ gensupport.URLParams
	ctx_       context.Context
	header_    http.Header
}

// Create: Creates a device in a device registry.
func (r *ProjectsLocationsRegistriesDevicesService) Create(parent string, device *Device) *ProjectsLocationsRegistriesDevicesCreateCall {
	c := &ProjectsLocationsRegistriesDevicesCreateCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	c.device = device
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesDevicesCreateCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesDevicesCreateCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesDevicesCreateCall) Context(ctx context.Context) *ProjectsLocationsRegistriesDevicesCreateCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesDevicesCreateCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesDevicesCreateCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.device)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+parent}/devices")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudiot.projects.locations.registries.devices.create" call.
// Exactly one of *Device or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Device.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *ProjectsLocationsRegistriesDevicesCreateCall) Do(opts ...googleapi.CallOption) (*Device, error) {
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
	ret := &Device{
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
	//   "description": "Creates a device in a device registry.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}/devices",
	//   "httpMethod": "POST",
	//   "id": "cloudiot.projects.locations.registries.devices.create",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "parent": {
	//       "description": "The name of the device registry where this device should be created.\nFor example,\n`projects/example-project/locations/us-central1/registries/my-registry`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+parent}/devices",
	//   "request": {
	//     "$ref": "Device"
	//   },
	//   "response": {
	//     "$ref": "Device"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.devices.delete":

type ProjectsLocationsRegistriesDevicesDeleteCall struct {
	s          *Service
	name       string
	urlParams_ gensupport.URLParams
	ctx_       context.Context
	header_    http.Header
}

// Delete: Deletes a device.
func (r *ProjectsLocationsRegistriesDevicesService) Delete(name string) *ProjectsLocationsRegistriesDevicesDeleteCall {
	c := &ProjectsLocationsRegistriesDevicesDeleteCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesDevicesDeleteCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesDevicesDeleteCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesDevicesDeleteCall) Context(ctx context.Context) *ProjectsLocationsRegistriesDevicesDeleteCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesDevicesDeleteCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesDevicesDeleteCall) doRequest(alt string) (*http.Response, error) {
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

// Do executes the "cloudiot.projects.locations.registries.devices.delete" call.
// Exactly one of *Empty or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Empty.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *ProjectsLocationsRegistriesDevicesDeleteCall) Do(opts ...googleapi.CallOption) (*Empty, error) {
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
	ret := &Empty{
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
	//   "description": "Deletes a device.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}/devices/{devicesId}",
	//   "httpMethod": "DELETE",
	//   "id": "cloudiot.projects.locations.registries.devices.delete",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the device. For example,\n`projects/p0/locations/us-central1/registries/registry0/devices/device0` or\n`projects/p0/locations/us-central1/registries/registry0/devices/{num_id}`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+/devices/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}",
	//   "response": {
	//     "$ref": "Empty"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.devices.get":

type ProjectsLocationsRegistriesDevicesGetCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// Get: Gets details about a device.
func (r *ProjectsLocationsRegistriesDevicesService) Get(name string) *ProjectsLocationsRegistriesDevicesGetCall {
	c := &ProjectsLocationsRegistriesDevicesGetCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// FieldMask sets the optional parameter "fieldMask": The fields of the
// `Device` resource to be returned in the response. If the
// field mask is unset or empty, all fields are returned.
func (c *ProjectsLocationsRegistriesDevicesGetCall) FieldMask(fieldMask string) *ProjectsLocationsRegistriesDevicesGetCall {
	c.urlParams_.Set("fieldMask", fieldMask)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesDevicesGetCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesDevicesGetCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsLocationsRegistriesDevicesGetCall) IfNoneMatch(entityTag string) *ProjectsLocationsRegistriesDevicesGetCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesDevicesGetCall) Context(ctx context.Context) *ProjectsLocationsRegistriesDevicesGetCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesDevicesGetCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesDevicesGetCall) doRequest(alt string) (*http.Response, error) {
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

// Do executes the "cloudiot.projects.locations.registries.devices.get" call.
// Exactly one of *Device or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Device.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *ProjectsLocationsRegistriesDevicesGetCall) Do(opts ...googleapi.CallOption) (*Device, error) {
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
	ret := &Device{
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
	//   "description": "Gets details about a device.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}/devices/{devicesId}",
	//   "httpMethod": "GET",
	//   "id": "cloudiot.projects.locations.registries.devices.get",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "fieldMask": {
	//       "description": "The fields of the `Device` resource to be returned in the response. If the\nfield mask is unset or empty, all fields are returned.",
	//       "format": "google-fieldmask",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "name": {
	//       "description": "The name of the device. For example,\n`projects/p0/locations/us-central1/registries/registry0/devices/device0` or\n`projects/p0/locations/us-central1/registries/registry0/devices/{num_id}`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+/devices/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}",
	//   "response": {
	//     "$ref": "Device"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.devices.list":

type ProjectsLocationsRegistriesDevicesListCall struct {
	s            *Service
	parent       string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: List devices in a device registry.
func (r *ProjectsLocationsRegistriesDevicesService) List(parent string) *ProjectsLocationsRegistriesDevicesListCall {
	c := &ProjectsLocationsRegistriesDevicesListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.parent = parent
	return c
}

// DeviceIds sets the optional parameter "deviceIds": A list of device
// string identifiers. If empty, it will ignore this field.
// For example, `['device0', 'device12']`. This field cannot hold more
// than
// 10,000 entries.
func (c *ProjectsLocationsRegistriesDevicesListCall) DeviceIds(deviceIds ...string) *ProjectsLocationsRegistriesDevicesListCall {
	c.urlParams_.SetMulti("deviceIds", append([]string{}, deviceIds...))
	return c
}

// DeviceNumIds sets the optional parameter "deviceNumIds": A list of
// device numerical ids. If empty, it will ignore this field. This
// field cannot hold more than 10,000 entries.
func (c *ProjectsLocationsRegistriesDevicesListCall) DeviceNumIds(deviceNumIds ...uint64) *ProjectsLocationsRegistriesDevicesListCall {
	var deviceNumIds_ []string
	for _, v := range deviceNumIds {
		deviceNumIds_ = append(deviceNumIds_, fmt.Sprint(v))
	}
	c.urlParams_.SetMulti("deviceNumIds", deviceNumIds_)
	return c
}

// FieldMask sets the optional parameter "fieldMask": The fields of the
// `Device` resource to be returned in the response. The
// fields `id`, and `num_id` are always returned by default, along with
// any
// other fields specified.
func (c *ProjectsLocationsRegistriesDevicesListCall) FieldMask(fieldMask string) *ProjectsLocationsRegistriesDevicesListCall {
	c.urlParams_.Set("fieldMask", fieldMask)
	return c
}

// PageSize sets the optional parameter "pageSize": The maximum number
// of devices to return in the response. If this value
// is zero, the service will select a default size. A call may return
// fewer
// objects than requested, but if there is a non-empty `page_token`,
// it
// indicates that more entries are available.
func (c *ProjectsLocationsRegistriesDevicesListCall) PageSize(pageSize int64) *ProjectsLocationsRegistriesDevicesListCall {
	c.urlParams_.Set("pageSize", fmt.Sprint(pageSize))
	return c
}

// PageToken sets the optional parameter "pageToken": The value returned
// by the last `ListDevicesResponse`; indicates
// that this is a continuation of a prior `ListDevices` call, and
// that the system should return the next page of data.
func (c *ProjectsLocationsRegistriesDevicesListCall) PageToken(pageToken string) *ProjectsLocationsRegistriesDevicesListCall {
	c.urlParams_.Set("pageToken", pageToken)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesDevicesListCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesDevicesListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsLocationsRegistriesDevicesListCall) IfNoneMatch(entityTag string) *ProjectsLocationsRegistriesDevicesListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesDevicesListCall) Context(ctx context.Context) *ProjectsLocationsRegistriesDevicesListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesDevicesListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesDevicesListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+parent}/devices")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"parent": c.parent,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudiot.projects.locations.registries.devices.list" call.
// Exactly one of *ListDevicesResponse or error will be non-nil. Any
// non-2xx status code is an error. Response headers are in either
// *ListDevicesResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsRegistriesDevicesListCall) Do(opts ...googleapi.CallOption) (*ListDevicesResponse, error) {
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
	ret := &ListDevicesResponse{
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
	//   "description": "List devices in a device registry.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}/devices",
	//   "httpMethod": "GET",
	//   "id": "cloudiot.projects.locations.registries.devices.list",
	//   "parameterOrder": [
	//     "parent"
	//   ],
	//   "parameters": {
	//     "deviceIds": {
	//       "description": "A list of device string identifiers. If empty, it will ignore this field.\nFor example, `['device0', 'device12']`. This field cannot hold more than\n10,000 entries.",
	//       "location": "query",
	//       "repeated": true,
	//       "type": "string"
	//     },
	//     "deviceNumIds": {
	//       "description": "A list of device numerical ids. If empty, it will ignore this field. This\nfield cannot hold more than 10,000 entries.",
	//       "format": "uint64",
	//       "location": "query",
	//       "repeated": true,
	//       "type": "string"
	//     },
	//     "fieldMask": {
	//       "description": "The fields of the `Device` resource to be returned in the response. The\nfields `id`, and `num_id` are always returned by default, along with any\nother fields specified.",
	//       "format": "google-fieldmask",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "pageSize": {
	//       "description": "The maximum number of devices to return in the response. If this value\nis zero, the service will select a default size. A call may return fewer\nobjects than requested, but if there is a non-empty `page_token`, it\nindicates that more entries are available.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     },
	//     "pageToken": {
	//       "description": "The value returned by the last `ListDevicesResponse`; indicates\nthat this is a continuation of a prior `ListDevices` call, and\nthat the system should return the next page of data.",
	//       "location": "query",
	//       "type": "string"
	//     },
	//     "parent": {
	//       "description": "The device registry path. Required. For example,\n`projects/my-project/locations/us-central1/registries/my-registry`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+parent}/devices",
	//   "response": {
	//     "$ref": "ListDevicesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// Pages invokes f for each page of results.
// A non-nil error returned from f will halt the iteration.
// The provided context supersedes any context provided to the Context method.
func (c *ProjectsLocationsRegistriesDevicesListCall) Pages(ctx context.Context, f func(*ListDevicesResponse) error) error {
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

// method id "cloudiot.projects.locations.registries.devices.modifyCloudToDeviceConfig":

type ProjectsLocationsRegistriesDevicesModifyCloudToDeviceConfigCall struct {
	s                                *Service
	name                             string
	modifycloudtodeviceconfigrequest *ModifyCloudToDeviceConfigRequest
	urlParams_                       gensupport.URLParams
	ctx_                             context.Context
	header_                          http.Header
}

// ModifyCloudToDeviceConfig: Modifies the configuration for the device,
// which is eventually sent from
// the Cloud IoT Core servers. Returns the modified configuration
// version and
// its metadata.
func (r *ProjectsLocationsRegistriesDevicesService) ModifyCloudToDeviceConfig(name string, modifycloudtodeviceconfigrequest *ModifyCloudToDeviceConfigRequest) *ProjectsLocationsRegistriesDevicesModifyCloudToDeviceConfigCall {
	c := &ProjectsLocationsRegistriesDevicesModifyCloudToDeviceConfigCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.modifycloudtodeviceconfigrequest = modifycloudtodeviceconfigrequest
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesDevicesModifyCloudToDeviceConfigCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesDevicesModifyCloudToDeviceConfigCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesDevicesModifyCloudToDeviceConfigCall) Context(ctx context.Context) *ProjectsLocationsRegistriesDevicesModifyCloudToDeviceConfigCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesDevicesModifyCloudToDeviceConfigCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesDevicesModifyCloudToDeviceConfigCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.modifycloudtodeviceconfigrequest)
	if err != nil {
		return nil, err
	}
	reqHeaders.Set("Content-Type", "application/json")
	c.urlParams_.Set("alt", alt)
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+name}:modifyCloudToDeviceConfig")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("POST", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudiot.projects.locations.registries.devices.modifyCloudToDeviceConfig" call.
// Exactly one of *DeviceConfig or error will be non-nil. Any non-2xx
// status code is an error. Response headers are in either
// *DeviceConfig.ServerResponse.Header or (if a response was returned at
// all) in error.(*googleapi.Error).Header. Use googleapi.IsNotModified
// to check whether the returned error was because
// http.StatusNotModified was returned.
func (c *ProjectsLocationsRegistriesDevicesModifyCloudToDeviceConfigCall) Do(opts ...googleapi.CallOption) (*DeviceConfig, error) {
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
	ret := &DeviceConfig{
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
	//   "description": "Modifies the configuration for the device, which is eventually sent from\nthe Cloud IoT Core servers. Returns the modified configuration version and\nits metadata.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}/devices/{devicesId}:modifyCloudToDeviceConfig",
	//   "httpMethod": "POST",
	//   "id": "cloudiot.projects.locations.registries.devices.modifyCloudToDeviceConfig",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the device. For example,\n`projects/p0/locations/us-central1/registries/registry0/devices/device0` or\n`projects/p0/locations/us-central1/registries/registry0/devices/{num_id}`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+/devices/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}:modifyCloudToDeviceConfig",
	//   "request": {
	//     "$ref": "ModifyCloudToDeviceConfigRequest"
	//   },
	//   "response": {
	//     "$ref": "DeviceConfig"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.devices.patch":

type ProjectsLocationsRegistriesDevicesPatchCall struct {
	s          *Service
	name       string
	device     *Device
	urlParams_ gensupport.URLParams
	ctx_       context.Context
	header_    http.Header
}

// Patch: Updates a device.
func (r *ProjectsLocationsRegistriesDevicesService) Patch(name string, device *Device) *ProjectsLocationsRegistriesDevicesPatchCall {
	c := &ProjectsLocationsRegistriesDevicesPatchCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	c.device = device
	return c
}

// UpdateMask sets the optional parameter "updateMask": Only updates the
// `device` fields indicated by this mask.
// The field mask must not be empty, and it must not contain fields
// that
// are immutable or only set by the server.
// Mutable top-level fields: `credentials`, `blocked`, and `metadata`
func (c *ProjectsLocationsRegistriesDevicesPatchCall) UpdateMask(updateMask string) *ProjectsLocationsRegistriesDevicesPatchCall {
	c.urlParams_.Set("updateMask", updateMask)
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesDevicesPatchCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesDevicesPatchCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesDevicesPatchCall) Context(ctx context.Context) *ProjectsLocationsRegistriesDevicesPatchCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesDevicesPatchCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesDevicesPatchCall) doRequest(alt string) (*http.Response, error) {
	reqHeaders := make(http.Header)
	for k, v := range c.header_ {
		reqHeaders[k] = v
	}
	reqHeaders.Set("User-Agent", c.s.userAgent())
	var body io.Reader = nil
	body, err := googleapi.WithoutDataWrapper.JSONReader(c.device)
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

// Do executes the "cloudiot.projects.locations.registries.devices.patch" call.
// Exactly one of *Device or error will be non-nil. Any non-2xx status
// code is an error. Response headers are in either
// *Device.ServerResponse.Header or (if a response was returned at all)
// in error.(*googleapi.Error).Header. Use googleapi.IsNotModified to
// check whether the returned error was because http.StatusNotModified
// was returned.
func (c *ProjectsLocationsRegistriesDevicesPatchCall) Do(opts ...googleapi.CallOption) (*Device, error) {
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
	ret := &Device{
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
	//   "description": "Updates a device.",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}/devices/{devicesId}",
	//   "httpMethod": "PATCH",
	//   "id": "cloudiot.projects.locations.registries.devices.patch",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The resource path name. For example,\n`projects/p1/locations/us-central1/registries/registry0/devices/dev0` or\n`projects/p1/locations/us-central1/registries/registry0/devices/{num_id}`.\nWhen `name` is populated as a response from the service, it always ends\nin the device numeric ID.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+/devices/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "updateMask": {
	//       "description": "Only updates the `device` fields indicated by this mask.\nThe field mask must not be empty, and it must not contain fields that\nare immutable or only set by the server.\nMutable top-level fields: `credentials`, `blocked`, and `metadata`",
	//       "format": "google-fieldmask",
	//       "location": "query",
	//       "type": "string"
	//     }
	//   },
	//   "path": "v1/{+name}",
	//   "request": {
	//     "$ref": "Device"
	//   },
	//   "response": {
	//     "$ref": "Device"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.devices.configVersions.list":

type ProjectsLocationsRegistriesDevicesConfigVersionsListCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists the last few versions of the device configuration in
// descending
// order (i.e.: newest first).
func (r *ProjectsLocationsRegistriesDevicesConfigVersionsService) List(name string) *ProjectsLocationsRegistriesDevicesConfigVersionsListCall {
	c := &ProjectsLocationsRegistriesDevicesConfigVersionsListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// NumVersions sets the optional parameter "numVersions": The number of
// versions to list. Versions are listed in decreasing order of
// the version number. The maximum number of versions retained is 10. If
// this
// value is zero, it will return all the versions available.
func (c *ProjectsLocationsRegistriesDevicesConfigVersionsListCall) NumVersions(numVersions int64) *ProjectsLocationsRegistriesDevicesConfigVersionsListCall {
	c.urlParams_.Set("numVersions", fmt.Sprint(numVersions))
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesDevicesConfigVersionsListCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesDevicesConfigVersionsListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsLocationsRegistriesDevicesConfigVersionsListCall) IfNoneMatch(entityTag string) *ProjectsLocationsRegistriesDevicesConfigVersionsListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesDevicesConfigVersionsListCall) Context(ctx context.Context) *ProjectsLocationsRegistriesDevicesConfigVersionsListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesDevicesConfigVersionsListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesDevicesConfigVersionsListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+name}/configVersions")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudiot.projects.locations.registries.devices.configVersions.list" call.
// Exactly one of *ListDeviceConfigVersionsResponse or error will be
// non-nil. Any non-2xx status code is an error. Response headers are in
// either *ListDeviceConfigVersionsResponse.ServerResponse.Header or (if
// a response was returned at all) in error.(*googleapi.Error).Header.
// Use googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsRegistriesDevicesConfigVersionsListCall) Do(opts ...googleapi.CallOption) (*ListDeviceConfigVersionsResponse, error) {
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
	ret := &ListDeviceConfigVersionsResponse{
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
	//   "description": "Lists the last few versions of the device configuration in descending\norder (i.e.: newest first).",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}/devices/{devicesId}/configVersions",
	//   "httpMethod": "GET",
	//   "id": "cloudiot.projects.locations.registries.devices.configVersions.list",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the device. For example,\n`projects/p0/locations/us-central1/registries/registry0/devices/device0` or\n`projects/p0/locations/us-central1/registries/registry0/devices/{num_id}`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+/devices/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "numVersions": {
	//       "description": "The number of versions to list. Versions are listed in decreasing order of\nthe version number. The maximum number of versions retained is 10. If this\nvalue is zero, it will return all the versions available.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     }
	//   },
	//   "path": "v1/{+name}/configVersions",
	//   "response": {
	//     "$ref": "ListDeviceConfigVersionsResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}

// method id "cloudiot.projects.locations.registries.devices.states.list":

type ProjectsLocationsRegistriesDevicesStatesListCall struct {
	s            *Service
	name         string
	urlParams_   gensupport.URLParams
	ifNoneMatch_ string
	ctx_         context.Context
	header_      http.Header
}

// List: Lists the last few versions of the device state in descending
// order (i.e.:
// newest first).
func (r *ProjectsLocationsRegistriesDevicesStatesService) List(name string) *ProjectsLocationsRegistriesDevicesStatesListCall {
	c := &ProjectsLocationsRegistriesDevicesStatesListCall{s: r.s, urlParams_: make(gensupport.URLParams)}
	c.name = name
	return c
}

// NumStates sets the optional parameter "numStates": The number of
// states to list. States are listed in descending order of
// update time. The maximum number of states retained is 10. If
// this
// value is zero, it will return all the states available.
func (c *ProjectsLocationsRegistriesDevicesStatesListCall) NumStates(numStates int64) *ProjectsLocationsRegistriesDevicesStatesListCall {
	c.urlParams_.Set("numStates", fmt.Sprint(numStates))
	return c
}

// Fields allows partial responses to be retrieved. See
// https://developers.google.com/gdata/docs/2.0/basics#PartialResponse
// for more information.
func (c *ProjectsLocationsRegistriesDevicesStatesListCall) Fields(s ...googleapi.Field) *ProjectsLocationsRegistriesDevicesStatesListCall {
	c.urlParams_.Set("fields", googleapi.CombineFields(s))
	return c
}

// IfNoneMatch sets the optional parameter which makes the operation
// fail if the object's ETag matches the given value. This is useful for
// getting updates only after the object has changed since the last
// request. Use googleapi.IsNotModified to check whether the response
// error from Do is the result of In-None-Match.
func (c *ProjectsLocationsRegistriesDevicesStatesListCall) IfNoneMatch(entityTag string) *ProjectsLocationsRegistriesDevicesStatesListCall {
	c.ifNoneMatch_ = entityTag
	return c
}

// Context sets the context to be used in this call's Do method. Any
// pending HTTP request will be aborted if the provided context is
// canceled.
func (c *ProjectsLocationsRegistriesDevicesStatesListCall) Context(ctx context.Context) *ProjectsLocationsRegistriesDevicesStatesListCall {
	c.ctx_ = ctx
	return c
}

// Header returns an http.Header that can be modified by the caller to
// add HTTP headers to the request.
func (c *ProjectsLocationsRegistriesDevicesStatesListCall) Header() http.Header {
	if c.header_ == nil {
		c.header_ = make(http.Header)
	}
	return c.header_
}

func (c *ProjectsLocationsRegistriesDevicesStatesListCall) doRequest(alt string) (*http.Response, error) {
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
	urls := googleapi.ResolveRelative(c.s.BasePath, "v1/{+name}/states")
	urls += "?" + c.urlParams_.Encode()
	req, _ := http.NewRequest("GET", urls, body)
	req.Header = reqHeaders
	googleapi.Expand(req.URL, map[string]string{
		"name": c.name,
	})
	return gensupport.SendRequest(c.ctx_, c.s.client, req)
}

// Do executes the "cloudiot.projects.locations.registries.devices.states.list" call.
// Exactly one of *ListDeviceStatesResponse or error will be non-nil.
// Any non-2xx status code is an error. Response headers are in either
// *ListDeviceStatesResponse.ServerResponse.Header or (if a response was
// returned at all) in error.(*googleapi.Error).Header. Use
// googleapi.IsNotModified to check whether the returned error was
// because http.StatusNotModified was returned.
func (c *ProjectsLocationsRegistriesDevicesStatesListCall) Do(opts ...googleapi.CallOption) (*ListDeviceStatesResponse, error) {
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
	ret := &ListDeviceStatesResponse{
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
	//   "description": "Lists the last few versions of the device state in descending order (i.e.:\nnewest first).",
	//   "flatPath": "v1/projects/{projectsId}/locations/{locationsId}/registries/{registriesId}/devices/{devicesId}/states",
	//   "httpMethod": "GET",
	//   "id": "cloudiot.projects.locations.registries.devices.states.list",
	//   "parameterOrder": [
	//     "name"
	//   ],
	//   "parameters": {
	//     "name": {
	//       "description": "The name of the device. For example,\n`projects/p0/locations/us-central1/registries/registry0/devices/device0` or\n`projects/p0/locations/us-central1/registries/registry0/devices/{num_id}`.",
	//       "location": "path",
	//       "pattern": "^projects/[^/]+/locations/[^/]+/registries/[^/]+/devices/[^/]+$",
	//       "required": true,
	//       "type": "string"
	//     },
	//     "numStates": {
	//       "description": "The number of states to list. States are listed in descending order of\nupdate time. The maximum number of states retained is 10. If this\nvalue is zero, it will return all the states available.",
	//       "format": "int32",
	//       "location": "query",
	//       "type": "integer"
	//     }
	//   },
	//   "path": "v1/{+name}/states",
	//   "response": {
	//     "$ref": "ListDeviceStatesResponse"
	//   },
	//   "scopes": [
	//     "https://www.googleapis.com/auth/cloud-platform",
	//     "https://www.googleapis.com/auth/cloudiot"
	//   ]
	// }

}
