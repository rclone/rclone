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

package builder

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/pengsrc/go-shared/convert"

	"github.com/yunify/qingstor-sdk-go"
	"github.com/yunify/qingstor-sdk-go/logger"
	"github.com/yunify/qingstor-sdk-go/request/data"
	"github.com/yunify/qingstor-sdk-go/utils"
)

// QingStorBuilder is the request builder for QingStor service.
type QingStorBuilder struct {
	baseBuilder *BaseBuilder
}

// BuildHTTPRequest builds http request with an operation and an input.
func (qb *QingStorBuilder) BuildHTTPRequest(o *data.Operation, i *reflect.Value) (*http.Request, error) {
	qb.baseBuilder = &BaseBuilder{}
	qb.baseBuilder.operation = o
	qb.baseBuilder.input = i

	_, err := qb.baseBuilder.parse()
	if err != nil {
		return nil, err
	}
	err = qb.parseURL()
	if err != nil {
		return nil, err
	}

	httpRequest, err := http.NewRequest(qb.baseBuilder.operation.RequestMethod,
		qb.baseBuilder.parsedURL, qb.baseBuilder.parsedBody)
	if err != nil {
		return nil, err
	}

	err = qb.baseBuilder.setupHeaders(httpRequest)
	if err != nil {
		return nil, err
	}
	err = qb.setupHeaders(httpRequest)
	if err != nil {
		return nil, err
	}

	logger.Infof(nil, fmt.Sprintf(
		"Built QingStor request: [%d] %s",
		convert.StringToTimestamp(httpRequest.Header.Get("Date"), convert.RFC822),
		httpRequest.URL.String(),
	))

	logger.Infof(nil, fmt.Sprintf(
		"QingStor request headers: [%d] %s",
		convert.StringToTimestamp(httpRequest.Header.Get("Date"), convert.RFC822),
		fmt.Sprint(httpRequest.Header),
	))

	if qb.baseBuilder.parsedBodyString != "" {
		logger.Infof(nil, fmt.Sprintf(
			"QingStor request body string: [%d] %s",
			convert.StringToTimestamp(httpRequest.Header.Get("Date"), convert.RFC822),
			qb.baseBuilder.parsedBodyString,
		))
	}

	return httpRequest, nil
}

func (qb *QingStorBuilder) parseURL() error {
	config := qb.baseBuilder.operation.Config

	zone := (*qb.baseBuilder.parsedProperties)["zone"]
	port := strconv.Itoa(config.Port)
	endpoint := config.Protocol + "://" + config.Host + ":" + port
	if zone != "" {
		endpoint = config.Protocol + "://" + zone + "." + config.Host + ":" + port
	}

	requestURI := qb.baseBuilder.operation.RequestURI
	for key, value := range *qb.baseBuilder.parsedProperties {
		endpoint = strings.Replace(endpoint, "<"+key+">", utils.URLQueryEscape(value), -1)
		requestURI = strings.Replace(requestURI, "<"+key+">", utils.URLQueryEscape(value), -1)
	}
	requestURI = regexp.MustCompile(`/+`).ReplaceAllString(requestURI, "/")

	requestURL, err := url.Parse(endpoint + requestURI)
	if err != nil {
		return err
	}

	if qb.baseBuilder.parsedQuery != nil {
		queryValue := requestURL.Query()
		for key, value := range *qb.baseBuilder.parsedQuery {
			queryValue.Set(key, value)
		}
		requestURL.RawQuery = queryValue.Encode()
	}

	qb.baseBuilder.parsedURL = requestURL.String()

	return nil
}

func (qb *QingStorBuilder) setupHeaders(httpRequest *http.Request) error {
	if httpRequest.Header.Get("User-Agent") == "" {
		version := fmt.Sprintf(`Go v%s`, strings.Replace(runtime.Version(), "go", "", -1))
		system := fmt.Sprintf(`%s_%s_%s`, runtime.GOOS, runtime.GOARCH, runtime.Compiler)
		ua := fmt.Sprintf(`qingstor-sdk-go/%s (%s; %s)`, sdk.Version, version, system)
		if qb.baseBuilder.operation.Config.AdditionalUserAgent != "" {
			ua = fmt.Sprintf(`%s %s`, ua, qb.baseBuilder.operation.Config.AdditionalUserAgent)
		}
		httpRequest.Header.Set("User-Agent", ua)
	}

	if s := httpRequest.Header.Get("X-QS-Fetch-Source"); s != "" {
		u, err := url.Parse(s)
		if err != nil {
			return fmt.Errorf("invalid HTTP header value: %s", s)
		}
		httpRequest.Header.Set("X-QS-Fetch-Source", u.String())
	}

	if qb.baseBuilder.operation.APIName == "Delete Multiple Objects" {
		buffer := &bytes.Buffer{}
		buffer.ReadFrom(httpRequest.Body)
		httpRequest.Body = ioutil.NopCloser(bytes.NewReader(buffer.Bytes()))

		md5Value := md5.Sum(buffer.Bytes())
		httpRequest.Header.Set("Content-MD5", base64.StdEncoding.EncodeToString(md5Value[:]))
	}

	return nil
}
