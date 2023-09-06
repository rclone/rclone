// +-------------------------------------------------------------------------
// | Copyright (C) 2016 Yunify, Inc.
// +-------------------------------------------------------------------------
// | Licensed under the Apache License, Version 2.0 (the "License");
// | you may not use this work except in compliance with the License.
// | You may obtain a copy of the License in the LICENSE file, or at:
// |
// | http://www.apache.org/licenses/LICENSE-2.0
// |
// | Unless required by applicable law or agreed to in writing, software
// | distributed under the License is distributed on an "AS IS" BASIS,
// | WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// | See the License for the specific language governing permissions and
// | limitations under the License.
// +-------------------------------------------------------------------------

package signer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/pengsrc/go-shared/convert"

	"github.com/yunify/qingstor-sdk-go/v3/logger"
	"github.com/yunify/qingstor-sdk-go/v3/utils"
)

// QingStorSigner is the http request signer for QingStor service.
type QingStorSigner struct {
	AccessKeyID     string
	SecretAccessKey string
}

// WriteSignature calculates signature and write it to http request header.
func (qss *QingStorSigner) WriteSignature(request *http.Request) error {
	authorization, err := qss.BuildSignature(request)
	if err != nil {
		return err
	}

	request.Header.Set("Authorization", authorization)

	return nil
}

// WriteQuerySignature calculates signature and write it to http request url.
func (qss *QingStorSigner) WriteQuerySignature(request *http.Request, expires int) error {
	query, err := qss.BuildQuerySignature(request, expires)
	if err != nil {
		return err
	}

	if request.URL.RawQuery != "" {
		query = "?" + request.URL.RawQuery + "&" + query
	} else {
		query = "?" + query
	}

	newRequest, err := http.NewRequest(request.Method,
		request.URL.Scheme+"://"+request.URL.Host+utils.URLQueryEscape(request.URL.Path)+query, nil)
	if err != nil {
		return err
	}
	request.URL = newRequest.URL

	return nil
}

// BuildSignature calculates the signature string.
func (qss *QingStorSigner) BuildSignature(request *http.Request) (string, error) {
	stringToSign, err := qss.BuildStringToSign(request)
	if err != nil {
		return "", err
	}

	h := hmac.New(sha256.New, []byte(qss.SecretAccessKey))
	h.Write([]byte(stringToSign))

	signature := strings.TrimSpace(base64.StdEncoding.EncodeToString(h.Sum(nil)))
	authorization := "QS " + qss.AccessKeyID + ":" + signature

	logger.Debugf(nil, fmt.Sprintf(
		"QingStor authorization: [%d] %s",
		convert.StringToTimestamp(request.Header.Get("Date"), convert.RFC822),
		authorization,
	))

	return authorization, nil
}

// BuildQuerySignature calculates the signature string for query.
func (qss *QingStorSigner) BuildQuerySignature(request *http.Request, expires int) (string, error) {
	stringToSign, err := qss.BuildQueryStringToSign(request, expires)
	if err != nil {
		return "", err
	}

	h := hmac.New(sha256.New, []byte(qss.SecretAccessKey))
	h.Write([]byte(stringToSign))

	signature := strings.TrimSpace(base64.StdEncoding.EncodeToString(h.Sum(nil)))
	signature = utils.URLQueryEscape(signature)
	query := fmt.Sprintf(
		"access_key_id=%s&expires=%d&signature=%s",
		qss.AccessKeyID, expires, signature,
	)

	logger.Debugf(nil, fmt.Sprintf(
		"QingStor query signature: [%d] %s",
		convert.StringToTimestamp(request.Header.Get("Date"), convert.RFC822),
		query,
	))

	return query, nil
}

// BuildStringToSign build the string to sign.
func (qss *QingStorSigner) BuildStringToSign(request *http.Request) (string, error) {
	date := request.Header.Get("Date")
	if request.Header.Get("X-QS-Date") != "" {
		date = ""
	}
	stringToSign := fmt.Sprintf(
		"%s\n%s\n%s\n%s\n",
		request.Method,
		request.Header.Get("Content-MD5"),
		request.Header.Get("Content-Type"),
		date,
	)

	stringToSign += qss.buildCanonicalizedHeaders(request)
	canonicalizedResource, err := qss.buildCanonicalizedResource(request)
	if err != nil {
		return "", err
	}
	stringToSign += canonicalizedResource

	logger.Debugf(nil, fmt.Sprintf(
		"QingStor string to sign: [%d] %s",
		convert.StringToTimestamp(request.Header.Get("Date"), convert.RFC822),
		stringToSign,
	))

	return stringToSign, nil
}

// BuildQueryStringToSign build the string to sign for query.
func (qss *QingStorSigner) BuildQueryStringToSign(request *http.Request, expires int) (string, error) {
	stringToSign := fmt.Sprintf(
		"%s\n%s\n%s\n%d\n",
		request.Method,
		request.Header.Get("Content-MD5"),
		request.Header.Get("Content-Type"),
		expires,
	)

	stringToSign += qss.buildCanonicalizedHeaders(request)
	canonicalizedResource, err := qss.buildCanonicalizedResource(request)
	if err != nil {
		return "", err
	}
	stringToSign += canonicalizedResource

	logger.Debugf(nil, fmt.Sprintf(
		"QingStor query string to sign: [%d] %s",
		convert.StringToTimestamp(request.Header.Get("Date"), convert.RFC822),
		stringToSign,
	))

	return stringToSign, nil
}

func (qss *QingStorSigner) buildCanonicalizedHeaders(request *http.Request) string {
	keys := []string{}
	for key := range request.Header {
		if strings.HasPrefix(strings.ToLower(key), "x-qs-") {
			keys = append(keys, strings.TrimSpace(strings.ToLower(key)))
		}
	}

	sort.Strings(keys)

	canonicalizedHeaders := ""
	for _, key := range keys {
		canonicalizedHeaders += key + ":" + strings.TrimSpace(request.Header.Get(key)) + "\n"
	}

	return canonicalizedHeaders
}

func (qss *QingStorSigner) buildCanonicalizedResource(request *http.Request) (string, error) {
	path := utils.URLQueryEscape(request.URL.Path)
	query := request.URL.Query()

	keys := []string{}
	for key := range query {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	parts := []string{}
	for _, key := range keys {
		values := query[key]
		if qss.queryToSign(key) {
			if len(values) > 0 {
				if values[0] != "" {
					value := strings.TrimSpace(strings.Join(values, ""))
					parts = append(parts, key+"="+value)
				} else {
					parts = append(parts, key)
				}
			} else {
				parts = append(parts, key)
			}
		}
	}

	joinedParts := strings.Join(parts, "&")
	if joinedParts != "" {
		path = path + "?" + joinedParts
	}

	logger.Debugf(nil, fmt.Sprintf(
		"QingStor canonicalized resource: [%d] %s",
		convert.StringToTimestamp(request.Header.Get("Date"), convert.RFC822),
		path,
	))

	return path, nil
}

func (qss *QingStorSigner) queryToSign(key string) bool {
	keysMap := map[string]bool{
		"acl":                          true,
		"cors":                         true,
		"delete":                       true,
		"mirror":                       true,
		"part_number":                  true,
		"policy":                       true,
		"stats":                        true,
		"upload_id":                    true,
		"uploads":                      true,
		"image":                        true,
		"notification":                 true,
		"lifecycle":                    true,
		"logging":                      true,
		"response-expires":             true,
		"response-cache-control":       true,
		"response-content-type":        true,
		"response-content-language":    true,
		"response-content-encoding":    true,
		"response-content-disposition": true,
	}

	return keysMap[key]
}
