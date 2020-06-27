/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2019 MinIO, Inc.
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
	"encoding/xml"
	"errors"
	"net/http"
	"net/url"
	"time"
)

// AssumeRoleWithLDAPResponse contains the result of successful
// AssumeRoleWithLDAPIdentity request
type AssumeRoleWithLDAPResponse struct {
	XMLName          xml.Name           `xml:"https://sts.amazonaws.com/doc/2011-06-15/ AssumeRoleWithLDAPIdentityResponse" json:"-"`
	Result           LDAPIdentityResult `xml:"AssumeRoleWithLDAPIdentityResult"`
	ResponseMetadata struct {
		RequestID string `xml:"RequestId,omitempty"`
	} `xml:"ResponseMetadata,omitempty"`
}

// LDAPIdentityResult - contains credentials for a successful
// AssumeRoleWithLDAPIdentity request.
type LDAPIdentityResult struct {
	Credentials struct {
		AccessKey    string    `xml:"AccessKeyId" json:"accessKey,omitempty"`
		SecretKey    string    `xml:"SecretAccessKey" json:"secretKey,omitempty"`
		Expiration   time.Time `xml:"Expiration" json:"expiration,omitempty"`
		SessionToken string    `xml:"SessionToken" json:"sessionToken,omitempty"`
	} `xml:",omitempty"`

	SubjectFromToken string `xml:",omitempty"`
}

// LDAPIdentity retrieves credentials from MinIO
type LDAPIdentity struct {
	Expiry

	stsEndpoint string

	ldapUsername, ldapPassword string
}

// NewLDAPIdentity returns new credentials object that uses LDAP
// Identity.
func NewLDAPIdentity(stsEndpoint, ldapUsername, ldapPassword string) (*Credentials, error) {
	return New(&LDAPIdentity{
		stsEndpoint:  stsEndpoint,
		ldapUsername: ldapUsername,
		ldapPassword: ldapPassword,
	}), nil
}

// Retrieve gets the credential by calling the MinIO STS API for
// LDAP on the configured stsEndpoint.
func (k *LDAPIdentity) Retrieve() (value Value, err error) {
	u, kerr := url.Parse(k.stsEndpoint)
	if kerr != nil {
		err = kerr
		return
	}

	clnt := &http.Client{Transport: http.DefaultTransport}
	v := url.Values{}
	v.Set("Action", "AssumeRoleWithLDAPIdentity")
	v.Set("Version", "2011-06-15")
	v.Set("LDAPUsername", k.ldapUsername)
	v.Set("LDAPPassword", k.ldapPassword)

	u.RawQuery = v.Encode()

	req, kerr := http.NewRequest("POST", u.String(), nil)
	if kerr != nil {
		err = kerr
		return
	}

	resp, kerr := clnt.Do(req)
	if kerr != nil {
		err = kerr
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = errors.New(resp.Status)
		return
	}

	r := AssumeRoleWithLDAPResponse{}
	if err = xml.NewDecoder(resp.Body).Decode(&r); err != nil {
		return
	}

	cr := r.Result.Credentials
	k.SetExpiration(cr.Expiration, DefaultExpiryWindow)
	return Value{
		AccessKeyID:     cr.AccessKey,
		SecretAccessKey: cr.SecretKey,
		SessionToken:    cr.SessionToken,
		SignerType:      SignatureV4,
	}, nil
}
