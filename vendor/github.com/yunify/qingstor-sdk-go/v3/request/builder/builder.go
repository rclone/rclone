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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/pengsrc/go-shared/convert"

	"github.com/yunify/qingstor-sdk-go/v3"
	"github.com/yunify/qingstor-sdk-go/v3/logger"
	"github.com/yunify/qingstor-sdk-go/v3/request/data"
	"github.com/yunify/qingstor-sdk-go/v3/utils"
)

// Builder is the request builder for QingStor service.
type Builder struct {
	parsedURL        string
	parsedProperties *map[string]string
	parsedQuery      *map[string]string
	parsedHeaders    *map[string]string
	parsedBodyString string
	parsedBody       io.Reader

	operation *data.Operation
	input     *reflect.Value
}

// BuildHTTPRequest builds http request with an operation and an input.
func (qb *Builder) BuildHTTPRequest(o *data.Operation, i *reflect.Value) (*http.Request, error) {
	qb.operation = o
	qb.input = i

	err := qb.parse()
	if err != nil {
		return nil, err
	}

	httpRequest, err := http.NewRequest(qb.operation.RequestMethod,
		qb.parsedURL, qb.parsedBody)
	if err != nil {
		return nil, err
	}

	err = qb.setupHeaders(httpRequest)
	if err != nil {
		return nil, err
	}

	timestamp := convert.StringToTimestamp(httpRequest.Header.Get("Date"), convert.RFC822)

	logger.Infof(nil, fmt.Sprintf(
		"Built QingStor request: [%d] %s",
		timestamp,
		httpRequest.URL.String(),
	))

	logger.Infof(nil, fmt.Sprintf(
		"QingStor request headers: [%d] %s",
		timestamp,
		fmt.Sprint(httpRequest.Header),
	))

	if qb.parsedBodyString != "" {
		logger.Infof(nil, fmt.Sprintf(
			"QingStor request body string: [%d] %s",
			timestamp,
			qb.parsedBodyString,
		))
	}

	return httpRequest, nil
}

func (qb *Builder) parse() error {
	err := qb.parseRequestQueryAndHeaders()
	if err != nil {
		return err
	}
	err = qb.parseRequestBody()
	if err != nil {
		return err
	}
	err = qb.parseRequestProperties()
	if err != nil {
		return err
	}
	err = qb.parseRequestURL()
	if err != nil {
		return err
	}

	return nil
}

func (qb *Builder) parseRequestBody() error {
	requestData := map[string]interface{}{}

	if !qb.input.IsValid() {
		return nil
	}

	fields := qb.input.Elem()
	if !fields.IsValid() {
		return nil
	}

	for i := 0; i < fields.NumField(); i++ {
		location := fields.Type().Field(i).Tag.Get("location")
		if location == "elements" {
			name := fields.Type().Field(i).Tag.Get("name")
			requestData[name] = fields.Field(i).Interface()
		}
	}

	if len(requestData) != 0 {
		dataValue, err := json.Marshal(requestData)
		if err != nil {
			return err
		}

		qb.parsedBodyString = string(dataValue)
		qb.parsedBody = strings.NewReader(qb.parsedBodyString)
		(*qb.parsedHeaders)["Content-Type"] = "application/json"
	} else {
		value := fields.FieldByName("Body")
		if value.IsValid() {
			switch value.Interface().(type) {
			case string:
				if value.String() != "" {
					qb.parsedBodyString = value.String()
					qb.parsedBody = strings.NewReader(value.String())
				}
			case io.Reader:
				if value.Interface().(io.Reader) != nil {
					qb.parsedBody = value.Interface().(io.Reader)
				}
			}
		}
	}

	return nil
}

func (qb *Builder) parseRequestProperties() error {
	propertiesMap := map[string]string{}
	qb.parsedProperties = &propertiesMap

	if qb.operation.Properties != nil {
		fields := reflect.ValueOf(qb.operation.Properties).Elem()
		if fields.IsValid() {
			for i := 0; i < fields.NumField(); i++ {
				switch value := fields.Field(i).Interface().(type) {
				case *string:
					if value != nil {
						propertiesMap[fields.Type().Field(i).Tag.Get("name")] = *value
					}
				case *int:
					if value != nil {
						numberString := strconv.Itoa(int(*value))
						propertiesMap[fields.Type().Field(i).Tag.Get("name")] = numberString
					}
				}
			}
		}
	}

	return nil
}

func (qb *Builder) parseRequestQueryAndHeaders() error {
	requestQuery := map[string]string{}
	requestHeaders := map[string]string{}
	maps := map[string](map[string]string){
		"query":   requestQuery,
		"headers": requestHeaders,
	}

	qb.parsedQuery = &requestQuery
	qb.parsedHeaders = &requestHeaders

	if !qb.input.IsValid() {
		return nil
	}

	fields := qb.input.Elem()
	if !fields.IsValid() {
		return nil
	}

	for i := 0; i < fields.NumField(); i++ {
		tagName := fields.Type().Field(i).Tag.Get("name")
		tagLocation := fields.Type().Field(i).Tag.Get("location")
		if tagDefault := fields.Type().Field(i).Tag.Get("default"); tagDefault != "" {
			maps[tagLocation][tagName] = tagDefault
		}
		if tagName != "" && tagLocation != "" && maps[tagLocation] != nil {
			switch value := fields.Field(i).Interface().(type) {
			case *string:
				if value != nil {
					maps[tagLocation][tagName] = *value
				}
			case *int:
				if value != nil {
					maps[tagLocation][tagName] = strconv.Itoa(int(*value))
				}
			case *int64:
				if value != nil {
					maps[tagLocation][tagName] = strconv.FormatInt(int64(*value), 10)
				}
			case *bool:
			case *time.Time:
				if value != nil {
					formatString := fields.Type().Field(i).Tag.Get("format")
					format := ""
					switch formatString {
					case "RFC 822":
						format = convert.RFC822
					case "ISO 8601":
						format = convert.ISO8601
					}
					maps[tagLocation][tagName] = convert.TimeToString(*value, format)
				}
			case *map[string]string:
				if value != nil {
					meta := *(fields.Field(i).Interface().(*map[string]string))
					for k, v := range meta {
						maps[tagLocation][k] = v
					}
				}
			}
		}
	}

	return nil
}

func (qb *Builder) parseRequestURL() error {
	config := qb.operation.Config

	zone := (*qb.parsedProperties)["zone"]
	port := strconv.Itoa(config.Port)
	endpoint := config.Protocol + "://" + config.Host + ":" + port
	if zone != "" {
		endpoint = config.Protocol + "://" + zone + "." + config.Host + ":" + port
	}

	requestURI := qb.operation.RequestURI
	for key, value := range *qb.parsedProperties {
		endpoint = strings.Replace(endpoint, "<"+key+">", utils.URLQueryEscape(value), -1)
		requestURI = strings.Replace(requestURI, "<"+key+">", utils.URLQueryEscape(value), -1)
	}
	if !config.DisableURICleaning {
		requestURI = regexp.MustCompile(`/+`).ReplaceAllString(requestURI, "/")
	}

	requestURL, err := url.Parse(endpoint + requestURI)
	if err != nil {
		return err
	}

	if qb.parsedQuery != nil {
		queryValue := requestURL.Query()
		for key, value := range *qb.parsedQuery {
			queryValue.Set(key, value)
		}
		requestURL.RawQuery = queryValue.Encode()
	}

	qb.parsedURL = requestURL.String()
	return nil
}

func (qb *Builder) setupHeaders(httpRequest *http.Request) error {
	if qb.parsedHeaders != nil {

		for headerKey, headerValue := range *qb.parsedHeaders {
			for _, r := range headerValue {
				if r > unicode.MaxASCII {
					headerValue = utils.URLQueryEscape(headerValue)
					break
				}
			}

			httpRequest.Header.Set(headerKey, headerValue)
		}
	}

	if httpRequest.Header.Get("Content-Length") == "" {
		var length int64
		switch body := qb.parsedBody.(type) {
		case nil:
			length = 0
		case io.Seeker:
			// start, err := body.Seek(0, io.SeekStart)
			start, err := body.Seek(0, 0)
			if err != nil {
				return err
			}
			// end, err := body.Seek(0, io.SeekEnd)
			end, err := body.Seek(0, 2)
			if err != nil {
				return err
			}
			// body.Seek(0, io.SeekStart)
			body.Seek(0, 0)
			length = end - start
		default:
			return errors.New("can not get Content-Length")
		}
		if length > 0 {
			httpRequest.ContentLength = length
			httpRequest.Header.Set("Content-Length", strconv.Itoa(int(length)))
		} else {
			httpRequest.Header.Set("Content-Length", "0")
		}
	}
	length, err := strconv.ParseInt(httpRequest.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return err
	}
	httpRequest.ContentLength = int64(length)

	if httpRequest.Header.Get("Date") == "" {
		httpRequest.Header.Set("Date", convert.TimeToString(time.Now(), convert.RFC822))
	}

	if httpRequest.Header.Get("User-Agent") == "" {
		version := fmt.Sprintf(`Go v%s`, strings.Replace(runtime.Version(), "go", "", -1))
		system := fmt.Sprintf(`%s_%s_%s`, runtime.GOOS, runtime.GOARCH, runtime.Compiler)
		ua := fmt.Sprintf(`qingstor-sdk-go/%s (%s; %s)`, sdk.Version, version, system)
		if qb.operation.Config.AdditionalUserAgent != "" {
			ua = fmt.Sprintf(`%s %s`, ua, qb.operation.Config.AdditionalUserAgent)
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

	if qb.operation.APIName == "Delete Multiple Objects" {
		buffer := &bytes.Buffer{}
		buffer.ReadFrom(httpRequest.Body)
		httpRequest.Body = ioutil.NopCloser(bytes.NewReader(buffer.Bytes()))

		md5Value := md5.Sum(buffer.Bytes())
		httpRequest.Header.Set("Content-MD5", base64.StdEncoding.EncodeToString(md5Value[:]))
	}

	return nil
}
