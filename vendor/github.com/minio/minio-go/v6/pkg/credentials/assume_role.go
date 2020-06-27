/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2020 MinIO, Inc.
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
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v6/pkg/signer"
	sha256 "github.com/minio/sha256-simd"
)

// AssumeRoleResponse contains the result of successful AssumeRole request.
type AssumeRoleResponse struct {
	XMLName xml.Name `xml:"https://sts.amazonaws.com/doc/2011-06-15/ AssumeRoleResponse" json:"-"`

	Result           AssumeRoleResult `xml:"AssumeRoleResult"`
	ResponseMetadata struct {
		RequestID string `xml:"RequestId,omitempty"`
	} `xml:"ResponseMetadata,omitempty"`
}

// AssumeRoleResult - Contains the response to a successful AssumeRole
// request, including temporary credentials that can be used to make
// MinIO API requests.
type AssumeRoleResult struct {
	// The identifiers for the temporary security credentials that the operation
	// returns.
	AssumedRoleUser AssumedRoleUser `xml:",omitempty"`

	// The temporary security credentials, which include an access key ID, a secret
	// access key, and a security (or session) token.
	//
	// Note: The size of the security token that STS APIs return is not fixed. We
	// strongly recommend that you make no assumptions about the maximum size. As
	// of this writing, the typical size is less than 4096 bytes, but that can vary.
	// Also, future updates to AWS might require larger sizes.
	Credentials struct {
		AccessKey    string    `xml:"AccessKeyId" json:"accessKey,omitempty"`
		SecretKey    string    `xml:"SecretAccessKey" json:"secretKey,omitempty"`
		Expiration   time.Time `xml:"Expiration" json:"expiration,omitempty"`
		SessionToken string    `xml:"SessionToken" json:"sessionToken,omitempty"`
	} `xml:",omitempty"`

	// A percentage value that indicates the size of the policy in packed form.
	// The service rejects any policy with a packed size greater than 100 percent,
	// which means the policy exceeded the allowed space.
	PackedPolicySize int `xml:",omitempty"`
}

// A STSAssumeRole retrieves credentials from MinIO service, and keeps track if
// those credentials are expired.
type STSAssumeRole struct {
	Expiry

	// Required http Client to use when connecting to MinIO STS service.
	Client *http.Client

	// STS endpoint to fetch STS credentials.
	STSEndpoint string

	// various options for this request.
	Options STSAssumeRoleOptions
}

// STSAssumeRoleOptions collection of various input options
// to obtain AssumeRole credentials.
type STSAssumeRoleOptions struct {
	// Mandatory inputs.
	AccessKey string
	SecretKey string

	Location        string // Optional commonly needed with AWS STS.
	DurationSeconds int    // Optional defaults to 1 hour.

	// Optional only valid if using with AWS STS
	RoleARN         string
	RoleSessionName string
}

// NewSTSAssumeRole returns a pointer to a new
// Credentials object wrapping the STSAssumeRole.
func NewSTSAssumeRole(stsEndpoint string, opts STSAssumeRoleOptions) (*Credentials, error) {
	if stsEndpoint == "" {
		return nil, errors.New("STS endpoint cannot be empty")
	}
	if opts.AccessKey == "" || opts.SecretKey == "" {
		return nil, errors.New("AssumeRole credentials access/secretkey is mandatory")
	}
	return New(&STSAssumeRole{
		Client: &http.Client{
			Transport: http.DefaultTransport,
		},
		STSEndpoint: stsEndpoint,
		Options:     opts,
	}), nil
}

const defaultDurationSeconds = 3600

// closeResponse close non nil response with any response Body.
// convenient wrapper to drain any remaining data on response body.
//
// Subsequently this allows golang http RoundTripper
// to re-use the same connection for future requests.
func closeResponse(resp *http.Response) {
	// Callers should close resp.Body when done reading from it.
	// If resp.Body is not closed, the Client's underlying RoundTripper
	// (typically Transport) may not be able to re-use a persistent TCP
	// connection to the server for a subsequent "keep-alive" request.
	if resp != nil && resp.Body != nil {
		// Drain any remaining Body and then close the connection.
		// Without this closing connection would disallow re-using
		// the same connection for future uses.
		//  - http://stackoverflow.com/a/17961593/4465767
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
}

func getAssumeRoleCredentials(clnt *http.Client, endpoint string, opts STSAssumeRoleOptions) (AssumeRoleResponse, error) {
	v := url.Values{}
	v.Set("Action", "AssumeRole")
	v.Set("Version", "2011-06-15")
	if opts.RoleARN != "" {
		v.Set("RoleArn", opts.RoleARN)
	}
	if opts.RoleSessionName != "" {
		v.Set("RoleSessionName", opts.RoleSessionName)
	}
	if opts.DurationSeconds > defaultDurationSeconds {
		v.Set("DurationSeconds", strconv.Itoa(opts.DurationSeconds))
	} else {
		v.Set("DurationSeconds", strconv.Itoa(defaultDurationSeconds))
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return AssumeRoleResponse{}, err
	}
	u.Path = "/"

	postBody := strings.NewReader(v.Encode())
	hash := sha256.New()
	if _, err = io.Copy(hash, postBody); err != nil {
		return AssumeRoleResponse{}, err
	}
	postBody.Seek(0, 0)

	req, err := http.NewRequest("POST", u.String(), postBody)
	if err != nil {
		return AssumeRoleResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(hash.Sum(nil)))
	req = signer.SignV4STS(*req, opts.AccessKey, opts.SecretKey, opts.Location)

	resp, err := clnt.Do(req)
	if err != nil {
		return AssumeRoleResponse{}, err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return AssumeRoleResponse{}, errors.New(resp.Status)
	}

	a := AssumeRoleResponse{}
	if err = xml.NewDecoder(resp.Body).Decode(&a); err != nil {
		return AssumeRoleResponse{}, err
	}
	return a, nil
}

// Retrieve retrieves credentials from the MinIO service.
// Error will be returned if the request fails.
func (m *STSAssumeRole) Retrieve() (Value, error) {
	a, err := getAssumeRoleCredentials(m.Client, m.STSEndpoint, m.Options)
	if err != nil {
		return Value{}, err
	}

	// Expiry window is set to 10secs.
	m.SetExpiration(a.Result.Credentials.Expiration, DefaultExpiryWindow)

	return Value{
		AccessKeyID:     a.Result.Credentials.AccessKey,
		SecretAccessKey: a.Result.Credentials.SecretKey,
		SessionToken:    a.Result.Credentials.SessionToken,
		SignerType:      SignatureV4,
	}, nil
}
