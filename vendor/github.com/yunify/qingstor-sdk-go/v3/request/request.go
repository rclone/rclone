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

package request

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/pengsrc/go-shared/convert"

	"github.com/yunify/qingstor-sdk-go/v3/logger"
	"github.com/yunify/qingstor-sdk-go/v3/request/builder"
	"github.com/yunify/qingstor-sdk-go/v3/request/data"
	"github.com/yunify/qingstor-sdk-go/v3/request/signer"
	"github.com/yunify/qingstor-sdk-go/v3/request/unpacker"
)

// A Request can build, sign, send and unpack API request.
type Request struct {
	Operation *data.Operation
	Input     *reflect.Value
	Output    *reflect.Value

	HTTPRequest  *http.Request
	HTTPResponse *http.Response
}

// New create a Request from given Operation, Input and Output.
// It returns a Request.
func New(o *data.Operation, i data.Input, x interface{}) (*Request, error) {
	input := reflect.ValueOf(i)
	if input.IsValid() && input.Elem().IsValid() {
		err := i.Validate()
		if err != nil {
			return nil, err
		}
	}
	output := reflect.ValueOf(x)

	return &Request{
		Operation: o,
		Input:     &input,
		Output:    &output,
	}, nil
}

// Send sends API request.
// It returns error if error occurred.
func (r *Request) Send() error {
	err := r.Build()
	if err != nil {
		return err
	}

	err = r.Sign()
	if err != nil {
		return err
	}

	err = r.Do()
	if err != nil {
		return err
	}

	return nil
}

// Build checks and builds the API request.
// It returns error if error occurred.
func (r *Request) Build() error {
	err := r.check()
	if err != nil {
		return err
	}

	err = r.build()
	if err != nil {
		return err
	}

	return nil
}

// Do sends and unpacks the API request.
// It returns error if error occurred.
func (r *Request) Do() error {
	err := r.send()
	if err != nil {
		return err
	}

	err = r.unpack()
	if err != nil {
		return err
	}
	return nil
}

// Sign sign the API request by setting the authorization header.
// It returns error if error occurred.
func (r *Request) Sign() error {
	err := r.sign()
	if err != nil {
		return err
	}

	return nil
}

// SignQuery sign the API request by appending query string.
// It returns error if error occurred.
func (r *Request) SignQuery(timeoutSeconds int) error {
	err := r.signQuery(int(time.Now().Unix()) + timeoutSeconds)
	if err != nil {
		return err
	}

	return nil
}

// ApplySignature applies the Authorization header.
// It returns error if error occurred.
func (r *Request) ApplySignature(authorization string) error {
	r.HTTPRequest.Header.Set("Authorization", authorization)
	return nil
}

// ApplyQuerySignature applies the query signature.
// It returns error if error occurred.
func (r *Request) ApplyQuerySignature(accessKeyID string, expires int, signature string) error {
	queryValue := r.HTTPRequest.URL.Query()
	queryValue.Set("access_key_id", accessKeyID)
	queryValue.Set("expires", strconv.Itoa(expires))
	queryValue.Set("signature", signature)

	r.HTTPRequest.URL.RawQuery = queryValue.Encode()
	return nil
}

func (r *Request) check() error {
	if r.Operation.Config.AccessKeyID == "" {
		return errors.New("access key not provided")
	}

	if r.Operation.Config.SecretAccessKey == "" {
		return errors.New("secret access key not provided")
	}

	return nil
}

func (r *Request) build() error {
	b := &builder.Builder{}
	httpRequest, err := b.BuildHTTPRequest(r.Operation, r.Input)
	if err != nil {
		return err
	}

	r.HTTPRequest = httpRequest
	return nil
}

func (r *Request) sign() error {
	s := &signer.QingStorSigner{
		AccessKeyID:     r.Operation.Config.AccessKeyID,
		SecretAccessKey: r.Operation.Config.SecretAccessKey,
	}
	err := s.WriteSignature(r.HTTPRequest)
	if err != nil {
		return err
	}

	return nil
}

func (r *Request) signQuery(expires int) error {
	s := &signer.QingStorSigner{
		AccessKeyID:     r.Operation.Config.AccessKeyID,
		SecretAccessKey: r.Operation.Config.SecretAccessKey,
	}
	err := s.WriteQuerySignature(r.HTTPRequest, expires)
	if err != nil {
		return err
	}

	return nil
}

func (r *Request) send() error {
	var response *http.Response
	var err error

	if r.Operation.Config.Connection == nil {
		r.Operation.Config.InitHTTPClient()
	}

	logger.Infof(nil, fmt.Sprintf(
		"Sending request: [%d] %s %s",
		convert.StringToTimestamp(r.HTTPRequest.Header.Get("Date"), convert.RFC822),
		r.Operation.RequestMethod,
		r.HTTPRequest.Host,
	))

	response, err = r.Operation.Config.Connection.Do(r.HTTPRequest)
	if err != nil {
		return err
	}

	r.HTTPResponse = response

	return nil
}

func (r *Request) unpack() error {
	u := &unpacker.QingStorUnpacker{}
	err := u.UnpackHTTPRequest(r.Operation, r.HTTPResponse, r.Output)
	if err != nil {
		return err
	}

	return nil
}
