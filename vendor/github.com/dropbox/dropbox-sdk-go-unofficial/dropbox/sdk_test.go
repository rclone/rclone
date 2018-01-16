// Copyright (c) Dropbox, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package dropbox_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/users"
)

func generateURL(base string, namespace string, route string) string {
	return fmt.Sprintf("%s/%s/%s", base, namespace, route)
}

func TestInternalError(t *testing.T) {
	eString := "internal server error"
	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, eString, http.StatusInternalServerError)
		}))
	defer ts.Close()

	config := dropbox.Config{Client: ts.Client(), LogLevel: dropbox.LogDebug,
		URLGenerator: func(hostType string, style string, namespace string, route string) string {
			return generateURL(ts.URL, namespace, route)
		}}
	client := users.New(config)
	v, e := client.GetCurrentAccount()
	if v != nil || strings.Trim(e.Error(), "\n") != eString {
		t.Errorf("v: %v e: '%s'\n", v, e.Error())
	}
}
