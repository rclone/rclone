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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/pengsrc/go-shared/convert"
	"github.com/pengsrc/go-shared/json"

	"github.com/yunify/qingstor-sdk-go/request/data"
	"github.com/yunify/qingstor-sdk-go/utils"
)

// BaseBuilder is the base builder for all services.
type BaseBuilder struct {
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
func (b *BaseBuilder) BuildHTTPRequest(o *data.Operation, i *reflect.Value) (*http.Request, error) {
	b.operation = o
	b.input = i

	_, err := b.parse()
	if err != nil {
		return nil, err
	}

	return b.build()
}

func (b *BaseBuilder) build() (*http.Request, error) {
	httpRequest, err := http.NewRequest(b.operation.RequestMethod, b.parsedURL, b.parsedBody)
	if err != nil {
		return nil, err
	}

	err = b.setupHeaders(httpRequest)
	if err != nil {
		return nil, err
	}

	return httpRequest, nil
}

func (b *BaseBuilder) parse() (*BaseBuilder, error) {
	err := b.parseRequestQueryAndHeaders()
	if err != nil {
		return b, err
	}
	err = b.parseRequestBody()
	if err != nil {
		return b, err
	}
	err = b.parseRequestProperties()
	if err != nil {
		return b, err
	}
	err = b.parseRequestURL()
	if err != nil {
		return b, err
	}

	return b, nil
}

func (b *BaseBuilder) parseRequestQueryAndHeaders() error {
	requestQuery := map[string]string{}
	requestHeaders := map[string]string{}
	maps := map[string](map[string]string){
		"query":  requestQuery,
		"headers": requestHeaders,
	}

	b.parsedQuery = &requestQuery
	b.parsedHeaders = &requestHeaders

	if !b.input.IsValid() {
		return nil
	}

	fields := b.input.Elem()
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
			}
		}
	}

	return nil
}

func (b *BaseBuilder) parseRequestBody() error {
	requestData := map[string]interface{}{}

	if !b.input.IsValid() {
		return nil
	}

	fields := b.input.Elem()
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
		dataValue, err := json.Encode(requestData, true)
		if err != nil {
			return err
		}

		b.parsedBodyString = string(dataValue)
		b.parsedBody = strings.NewReader(b.parsedBodyString)
		(*b.parsedHeaders)["Content-Type"] = "application/json"
	} else {
		value := fields.FieldByName("Body")
		if value.IsValid() {
			switch value.Interface().(type) {
			case string:
				if value.String() != "" {
					b.parsedBodyString = value.String()
					b.parsedBody = strings.NewReader(value.String())
				}
			case io.Reader:
				if value.Interface().(io.Reader) != nil {
					b.parsedBody = value.Interface().(io.Reader)
				}
			}
		}
	}

	return nil
}

func (b *BaseBuilder) parseRequestProperties() error {
	propertiesMap := map[string]string{}
	b.parsedProperties = &propertiesMap

	if b.operation.Properties != nil {
		fields := reflect.ValueOf(b.operation.Properties).Elem()
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

func (b *BaseBuilder) parseRequestURL() error {
	return nil
}

func (b *BaseBuilder) setupHeaders(httpRequest *http.Request) error {
	if b.parsedHeaders != nil {

		for headerKey, headerValue := range *b.parsedHeaders {
			if headerKey == "X-QS-Fetch-Source" {
				// header X-QS-Fetch-Source is a URL to fetch.
				// We should first parse this URL.
				requestURL, err := url.Parse(headerValue)
				if err != nil {
					return fmt.Errorf("invalid HTTP header value: %s", headerValue)
				}
				headerValue = requestURL.String()
			} else {
				for _, r := range headerValue {
					if r > unicode.MaxASCII {
						headerValue = utils.URLQueryEscape(headerValue)
						break
					}
				}
			}

			httpRequest.Header.Set(headerKey, headerValue)
		}
	}

	if httpRequest.Header.Get("Content-Length") == "" {
		var length int64
		switch body := b.parsedBody.(type) {
		case nil:
			length = 0
		case io.Seeker:
			//start, err := body.Seek(0, io.SeekStart)
			start, err := body.Seek(0, 0)
			if err != nil {
				return err
			}
			//end, err := body.Seek(0, io.SeekEnd)
			end, err := body.Seek(0, 2)
			if err != nil {
				return err
			}
			//body.Seek(0, io.SeekStart)
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
	length, err := strconv.Atoi(httpRequest.Header.Get("Content-Length"))
	if err != nil {
		return err
	}
	httpRequest.ContentLength = int64(length)

	if httpRequest.Header.Get("Date") == "" {
		httpRequest.Header.Set("Date", convert.TimeToString(time.Now(), convert.RFC822))
	}

	return nil
}
