/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2017 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package credentials

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	jsoniter "github.com/json-iterator/go"
)

// DefaultExpiryWindow - Default expiry window.
// ExpiryWindow will allow the credentials to trigger refreshing
// prior to the credentials actually expiring. This is beneficial
// so race conditions with expiring credentials do not cause
// request to fail unexpectedly due to ExpiredTokenException exceptions.
const DefaultExpiryWindow = time.Second * 10 // 10 secs

// A IAM retrieves credentials from the EC2 service, and keeps track if
// those credentials are expired.
type IAM struct {
	Expiry

	// Required http Client to use when connecting to IAM metadata service.
	Client *http.Client

	// Custom endpoint to fetch IAM role credentials.
	endpoint string
}

// IAM Roles for Amazon EC2
// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
const (
	defaultIAMRoleEndpoint      = "http://169.254.169.254"
	defaultECSRoleEndpoint      = "http://169.254.170.2"
	defaultSTSRoleEndpoint      = "https://sts.amazonaws.com"
	defaultIAMSecurityCredsPath = "/latest/meta-data/iam/security-credentials/"
)

// NewIAM returns a pointer to a new Credentials object wrapping the IAM.
func NewIAM(endpoint string) *Credentials {
	p := &IAM{
		Client: &http.Client{
			Transport: http.DefaultTransport,
		},
		endpoint: endpoint,
	}
	return New(p)
}

// Retrieve retrieves credentials from the EC2 service.
// Error will be returned if the request fails, or unable to extract
// the desired
func (m *IAM) Retrieve() (Value, error) {
	var roleCreds ec2RoleCredRespBody
	var err error

	endpoint := m.endpoint
	switch {
	case len(os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")) > 0:
		if len(endpoint) == 0 {
			if len(os.Getenv("AWS_REGION")) > 0 {
				endpoint = "https://sts." + os.Getenv("AWS_REGION") + ".amazonaws.com"
			} else {
				endpoint = defaultSTSRoleEndpoint
			}
		}

		creds := &STSWebIdentity{
			Client:          m.Client,
			stsEndpoint:     endpoint,
			roleARN:         os.Getenv("AWS_ROLE_ARN"),
			roleSessionName: os.Getenv("AWS_ROLE_SESSION_NAME"),
			getWebIDTokenExpiry: func() (*WebIdentityToken, error) {
				token, err := ioutil.ReadFile(os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE"))
				if err != nil {
					return nil, err
				}

				return &WebIdentityToken{Token: string(token)}, nil
			},
		}

		return creds.Retrieve()

	case len(os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI")) > 0:
		if len(endpoint) == 0 {
			endpoint = fmt.Sprintf("%s%s", defaultECSRoleEndpoint,
				os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"))
		}

		roleCreds, err = getEcsTaskCredentials(m.Client, endpoint)

	case len(os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI")) > 0:
		if len(endpoint) == 0 {
			endpoint = os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI")

			var ok bool
			if ok, err = isLoopback(endpoint); !ok {
				if err == nil {
					err = fmt.Errorf("uri host is not a loopback address: %s", endpoint)
				}
				break
			}
		}

		roleCreds, err = getEcsTaskCredentials(m.Client, endpoint)

	default:
		roleCreds, err = getCredentials(m.Client, endpoint)
	}

	if err != nil {
		return Value{}, err
	}
	// Expiry window is set to 10secs.
	m.SetExpiration(roleCreds.Expiration, DefaultExpiryWindow)

	return Value{
		AccessKeyID:     roleCreds.AccessKeyID,
		SecretAccessKey: roleCreds.SecretAccessKey,
		SessionToken:    roleCreds.Token,
		SignerType:      SignatureV4,
	}, nil
}

// A ec2RoleCredRespBody provides the shape for unmarshaling credential
// request responses.
type ec2RoleCredRespBody struct {
	// Success State
	Expiration      time.Time
	AccessKeyID     string
	SecretAccessKey string
	Token           string

	// Error state
	Code    string
	Message string

	// Unused params.
	LastUpdated time.Time
	Type        string
}

// Get the final IAM role URL where the request will
// be sent to fetch the rolling access credentials.
// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
func getIAMRoleURL(endpoint string) (*url.URL, error) {
	if endpoint == "" {
		endpoint = defaultIAMRoleEndpoint
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	u.Path = defaultIAMSecurityCredsPath
	return u, nil
}

// listRoleNames lists of credential role names associated
// with the current EC2 service. If there are no credentials,
// or there is an error making or receiving the request.
// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
func listRoleNames(client *http.Client, u *url.URL) ([]string, error) {
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	credsList := []string{}
	s := bufio.NewScanner(resp.Body)
	for s.Scan() {
		credsList = append(credsList, s.Text())
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	return credsList, nil
}

func getEcsTaskCredentials(client *http.Client, endpoint string) (ec2RoleCredRespBody, error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return ec2RoleCredRespBody{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return ec2RoleCredRespBody{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ec2RoleCredRespBody{}, errors.New(resp.Status)
	}

	respCreds := ec2RoleCredRespBody{}
	if err := jsoniter.NewDecoder(resp.Body).Decode(&respCreds); err != nil {
		return ec2RoleCredRespBody{}, err
	}

	return respCreds, nil
}

// getCredentials - obtains the credentials from the IAM role name associated with
// the current EC2 service.
//
// If the credentials cannot be found, or there is an error
// reading the response an error will be returned.
func getCredentials(client *http.Client, endpoint string) (ec2RoleCredRespBody, error) {

	// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
	u, err := getIAMRoleURL(endpoint)
	if err != nil {
		return ec2RoleCredRespBody{}, err
	}

	// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
	roleNames, err := listRoleNames(client, u)
	if err != nil {
		return ec2RoleCredRespBody{}, err
	}

	if len(roleNames) == 0 {
		return ec2RoleCredRespBody{}, errors.New("No IAM roles attached to this EC2 service")
	}

	// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
	// - An instance profile can contain only one IAM role. This limit cannot be increased.
	roleName := roleNames[0]

	// http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
	// The following command retrieves the security credentials for an
	// IAM role named `s3access`.
	//
	//    $ curl http://169.254.169.254/latest/meta-data/iam/security-credentials/s3access
	//
	u.Path = path.Join(u.Path, roleName)
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return ec2RoleCredRespBody{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return ec2RoleCredRespBody{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ec2RoleCredRespBody{}, errors.New(resp.Status)
	}

	respCreds := ec2RoleCredRespBody{}
	if err := jsoniter.NewDecoder(resp.Body).Decode(&respCreds); err != nil {
		return ec2RoleCredRespBody{}, err
	}

	if respCreds.Code != "Success" {
		// If an error code was returned something failed requesting the role.
		return ec2RoleCredRespBody{}, errors.New(respCreds.Message)
	}

	return respCreds, nil
}

// isLoopback identifies if a uri's host is on a loopback address
func isLoopback(uri string) (bool, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return false, err
	}

	host := u.Hostname()
	if len(host) == 0 {
		return false, fmt.Errorf("can't parse host from uri: %s", uri)
	}

	ips, err := net.LookupHost(host)
	if err != nil {
		return false, err
	}
	for _, ip := range ips {
		if !net.ParseIP(ip).IsLoopback() {
			return false, nil
		}
	}

	return true, nil
}
