// Package signature verify request for any AWS service.
package signature

import (
	"strings"
	"sync"
	"time"
)

var credStore sync.Map

// Credentials holds access and secret keys.
type Credentials struct {
	AccessKey    string                 `xml:"AccessKeyId" json:"accessKey,omitempty"`
	SecretKey    string                 `xml:"SecretAccessKey" json:"secretKey,omitempty"`
	Expiration   time.Time              `xml:"Expiration" json:"expiration,omitempty"`
	SessionToken string                 `xml:"SessionToken" json:"sessionToken,omitempty"`
	Status       string                 `xml:"-" json:"status,omitempty"`
	ParentUser   string                 `xml:"-" json:"parentUser,omitempty"`
	Groups       []string               `xml:"-" json:"groups,omitempty"`
	Claims       map[string]interface{} `xml:"-" json:"claims,omitempty"`
}

// signValues data type represents structured form of AWS Signature V4 header.
type signValues struct {
	Credential    credentialHeader
	SignedHeaders []string
	Signature     string
}

// credentialHeader data type represents structured form of Credential
// string from authorization header.
type credentialHeader struct {
	accessKey string
	scope     signScope
}

type signScope struct {
	date    time.Time
	region  string
	service string
	request string
}

// Return scope string.
func (c credentialHeader) getScope() string {
	return strings.Join([]string{
		c.scope.date.Format(yyyymmdd),
		c.scope.region,
		c.scope.service,
		c.scope.request,
	}, slashSeparator)
}
