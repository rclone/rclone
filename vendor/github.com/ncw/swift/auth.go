package swift

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// Auth defines the operations needed to authenticate with swift
//
// This encapsulates the different authentication schemes in use
type Authenticator interface {
	// Request creates an http.Request for the auth - return nil if not needed
	Request(*Connection) (*http.Request, error)
	// Response parses the http.Response
	Response(resp *http.Response) error
	// The public storage URL - set Internal to true to read
	// internal/service net URL
	StorageUrl(Internal bool) string
	// The access token
	Token() string
	// The CDN url if available
	CdnUrl() string
}

type CustomEndpointAuthenticator interface {
	StorageUrlForEndpoint(endpointType EndpointType) string
}

type EndpointType string

const (
	// Use public URL as storage URL
	EndpointTypePublic = EndpointType("public")

	// Use internal URL as storage URL
	EndpointTypeInternal = EndpointType("internal")

	// Use admin URL as storage URL
	EndpointTypeAdmin = EndpointType("admin")
)

// newAuth - create a new Authenticator from the AuthUrl
//
// A hint for AuthVersion can be provided
func newAuth(c *Connection) (Authenticator, error) {
	AuthVersion := c.AuthVersion
	if AuthVersion == 0 {
		if strings.Contains(c.AuthUrl, "v3") {
			AuthVersion = 3
		} else if strings.Contains(c.AuthUrl, "v2") {
			AuthVersion = 2
		} else if strings.Contains(c.AuthUrl, "v1") {
			AuthVersion = 1
		} else {
			return nil, newErrorf(500, "Can't find AuthVersion in AuthUrl - set explicitly")
		}
	}
	switch AuthVersion {
	case 1:
		return &v1Auth{}, nil
	case 2:
		return &v2Auth{
			// Guess as to whether using API key or
			// password it will try both eventually so
			// this is just an optimization.
			useApiKey: len(c.ApiKey) >= 32,
		}, nil
	case 3:
		return &v3Auth{}, nil
	}
	return nil, newErrorf(500, "Auth Version %d not supported", AuthVersion)
}

// ------------------------------------------------------------

// v1 auth
type v1Auth struct {
	Headers http.Header // V1 auth: the authentication headers so extensions can access them
}

// v1 Authentication - make request
func (auth *v1Auth) Request(c *Connection) (*http.Request, error) {
	req, err := http.NewRequest("GET", c.AuthUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("X-Auth-Key", c.ApiKey)
	req.Header.Set("X-Auth-User", c.UserName)
	return req, nil
}

// v1 Authentication - read response
func (auth *v1Auth) Response(resp *http.Response) error {
	auth.Headers = resp.Header
	return nil
}

// v1 Authentication - read storage url
func (auth *v1Auth) StorageUrl(Internal bool) string {
	storageUrl := auth.Headers.Get("X-Storage-Url")
	if Internal {
		newUrl, err := url.Parse(storageUrl)
		if err != nil {
			return storageUrl
		}
		newUrl.Host = "snet-" + newUrl.Host
		storageUrl = newUrl.String()
	}
	return storageUrl
}

// v1 Authentication - read auth token
func (auth *v1Auth) Token() string {
	return auth.Headers.Get("X-Auth-Token")
}

// v1 Authentication - read cdn url
func (auth *v1Auth) CdnUrl() string {
	return auth.Headers.Get("X-CDN-Management-Url")
}

// ------------------------------------------------------------

// v2 Authentication
type v2Auth struct {
	Auth        *v2AuthResponse
	Region      string
	useApiKey   bool // if set will use API key not Password
	useApiKeyOk bool // if set won't change useApiKey any more
	notFirst    bool // set after first run
}

// v2 Authentication - make request
func (auth *v2Auth) Request(c *Connection) (*http.Request, error) {
	auth.Region = c.Region
	// Toggle useApiKey if not first run and not OK yet
	if auth.notFirst && !auth.useApiKeyOk {
		auth.useApiKey = !auth.useApiKey
	}
	auth.notFirst = true
	// Create a V2 auth request for the body of the connection
	var v2i interface{}
	if !auth.useApiKey {
		// Normal swift authentication
		v2 := v2AuthRequest{}
		v2.Auth.PasswordCredentials.UserName = c.UserName
		v2.Auth.PasswordCredentials.Password = c.ApiKey
		v2.Auth.Tenant = c.Tenant
		v2.Auth.TenantId = c.TenantId
		v2i = v2
	} else {
		// Rackspace special with API Key
		v2 := v2AuthRequestRackspace{}
		v2.Auth.ApiKeyCredentials.UserName = c.UserName
		v2.Auth.ApiKeyCredentials.ApiKey = c.ApiKey
		v2.Auth.Tenant = c.Tenant
		v2.Auth.TenantId = c.TenantId
		v2i = v2
	}
	body, err := json.Marshal(v2i)
	if err != nil {
		return nil, err
	}
	url := c.AuthUrl
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	url += "tokens"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	return req, nil
}

// v2 Authentication - read response
func (auth *v2Auth) Response(resp *http.Response) error {
	auth.Auth = new(v2AuthResponse)
	err := readJson(resp, auth.Auth)
	// If successfully read Auth then no need to toggle useApiKey any more
	if err == nil {
		auth.useApiKeyOk = true
	}
	return err
}

// Finds the Endpoint Url of "type" from the v2AuthResponse using the
// Region if set or defaulting to the first one if not
//
// Returns "" if not found
func (auth *v2Auth) endpointUrl(Type string, endpointType EndpointType) string {
	for _, catalog := range auth.Auth.Access.ServiceCatalog {
		if catalog.Type == Type {
			for _, endpoint := range catalog.Endpoints {
				if auth.Region == "" || (auth.Region == endpoint.Region) {
					switch endpointType {
					case EndpointTypeInternal:
						return endpoint.InternalUrl
					case EndpointTypePublic:
						return endpoint.PublicUrl
					case EndpointTypeAdmin:
						return endpoint.AdminUrl
					default:
						return ""
					}
				}
			}
		}
	}
	return ""
}

// v2 Authentication - read storage url
//
// If Internal is true then it reads the private (internal / service
// net) URL.
func (auth *v2Auth) StorageUrl(Internal bool) string {
	endpointType := EndpointTypePublic
	if Internal {
		endpointType = EndpointTypeInternal
	}
	return auth.StorageUrlForEndpoint(endpointType)
}

// v2 Authentication - read storage url
//
// Use the indicated endpointType to choose a URL.
func (auth *v2Auth) StorageUrlForEndpoint(endpointType EndpointType) string {
	return auth.endpointUrl("object-store", endpointType)
}

// v2 Authentication - read auth token
func (auth *v2Auth) Token() string {
	return auth.Auth.Access.Token.Id
}

// v2 Authentication - read cdn url
func (auth *v2Auth) CdnUrl() string {
	return auth.endpointUrl("rax:object-cdn", EndpointTypePublic)
}

// ------------------------------------------------------------

// V2 Authentication request
//
// http://docs.openstack.org/developer/keystone/api_curl_examples.html
// http://docs.rackspace.com/servers/api/v2/cs-gettingstarted/content/curl_auth.html
// http://docs.openstack.org/api/openstack-identity-service/2.0/content/POST_authenticate_v2.0_tokens_.html
type v2AuthRequest struct {
	Auth struct {
		PasswordCredentials struct {
			UserName string `json:"username"`
			Password string `json:"password"`
		} `json:"passwordCredentials"`
		Tenant   string `json:"tenantName,omitempty"`
		TenantId string `json:"tenantId,omitempty"`
	} `json:"auth"`
}

// V2 Authentication request - Rackspace variant
//
// http://docs.openstack.org/developer/keystone/api_curl_examples.html
// http://docs.rackspace.com/servers/api/v2/cs-gettingstarted/content/curl_auth.html
// http://docs.openstack.org/api/openstack-identity-service/2.0/content/POST_authenticate_v2.0_tokens_.html
type v2AuthRequestRackspace struct {
	Auth struct {
		ApiKeyCredentials struct {
			UserName string `json:"username"`
			ApiKey   string `json:"apiKey"`
		} `json:"RAX-KSKEY:apiKeyCredentials"`
		Tenant   string `json:"tenantName,omitempty"`
		TenantId string `json:"tenantId,omitempty"`
	} `json:"auth"`
}

// V2 Authentication reply
//
// http://docs.openstack.org/developer/keystone/api_curl_examples.html
// http://docs.rackspace.com/servers/api/v2/cs-gettingstarted/content/curl_auth.html
// http://docs.openstack.org/api/openstack-identity-service/2.0/content/POST_authenticate_v2.0_tokens_.html
type v2AuthResponse struct {
	Access struct {
		ServiceCatalog []struct {
			Endpoints []struct {
				InternalUrl string
				PublicUrl   string
				AdminUrl    string
				Region      string
				TenantId    string
			}
			Name string
			Type string
		}
		Token struct {
			Expires string
			Id      string
			Tenant  struct {
				Id   string
				Name string
			}
		}
		User struct {
			DefaultRegion string `json:"RAX-AUTH:defaultRegion"`
			Id            string
			Name          string
			Roles         []struct {
				Description string
				Id          string
				Name        string
				TenantId    string
			}
		}
	}
}
