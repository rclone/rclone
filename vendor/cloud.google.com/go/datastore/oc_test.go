// Copyright 2018 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build go1.8

package datastore

import (
	"testing"

	"cloud.google.com/go/internal/testutil"
	"golang.org/x/net/context"
)

func TestOCTracing(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(ctx, t)
	defer client.Close()

	te := testutil.NewTestExporter()
	defer te.Unregister()

	type SomeValue struct {
		S string
	}
	_, err := client.Put(ctx, IncompleteKey("SomeKey", nil), &SomeValue{"foo"})
	if err != nil {
		t.Fatalf("client.Put: %v", err)
	}

	if len(te.Spans) == 0 {
		t.Fatalf("Expected some span to be created, but got %d", 0)
	}
}
