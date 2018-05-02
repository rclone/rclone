// Copyright 2018 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/context"
	"google.golang.org/api/option"
)

const readData = "0123456789"

func TestRangeReader(t *testing.T) {
	hc, close := newTestServer(handleRangeRead)
	defer close()
	ctx := context.Background()
	c, err := NewClient(ctx, option.WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}
	obj := c.Bucket("b").Object("o")
	for _, test := range []struct {
		offset, length int64
		want           string
	}{
		{0, -1, readData},
		{0, 10, readData},
		{0, 5, readData[:5]},
		{1, 3, readData[1:4]},
		{6, -1, readData[6:]},
		{4, 20, readData[4:]},
	} {
		r, err := obj.NewRangeReader(ctx, test.offset, test.length)
		if err != nil {
			t.Errorf("%d/%d: %v", test.offset, test.length, err)
			continue
		}
		gotb, err := ioutil.ReadAll(r)
		if err != nil {
			t.Errorf("%d/%d: %v", test.offset, test.length, err)
			continue
		}
		if got := string(gotb); got != test.want {
			t.Errorf("%d/%d: got %q, want %q", test.offset, test.length, got, test.want)
		}
	}
}

func handleRangeRead(w http.ResponseWriter, r *http.Request) {
	rh := strings.TrimSpace(r.Header.Get("Range"))
	data := readData
	var from, to int
	if rh == "" {
		from = 0
		to = len(data)
	} else {
		// assume "bytes=N-" or "bytes=N-M"
		var err error
		i := strings.IndexRune(rh, '=')
		j := strings.IndexRune(rh, '-')
		from, err = strconv.Atoi(rh[i+1 : j])
		if err != nil {
			w.WriteHeader(500)
			return
		}
		to = len(data)
		if j+1 < len(rh) {
			to, err = strconv.Atoi(rh[j+1:])
			if err != nil {
				w.WriteHeader(500)
				return
			}
			to++ // Range header is inclusive, Go slice is exclusive
		}
		if from >= len(data) && to != from {
			w.WriteHeader(416)
			return
		}
		if from > len(data) {
			from = len(data)
		}
		if to > len(data) {
			to = len(data)
		}
	}
	data = data[from:to]
	if data != readData {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", from, to-1, len(readData)))
		w.WriteHeader(http.StatusPartialContent)
	}
	if _, err := w.Write([]byte(data)); err != nil {
		panic(err)
	}
}
