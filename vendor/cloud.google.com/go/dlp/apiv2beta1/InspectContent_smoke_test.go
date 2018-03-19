// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// AUTO-GENERATED CODE. DO NOT EDIT.

package dlp

import (
	dlppb "google.golang.org/genproto/googleapis/privacy/dlp/v2beta1"
)

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"cloud.google.com/go/internal/testutil"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var _ = fmt.Sprintf
var _ = iterator.Done
var _ = strconv.FormatUint
var _ = time.Now

func TestDlpServiceSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	ctx := context.Background()
	ts := testutil.TokenSource(ctx, DefaultAuthScopes()...)
	if ts == nil {
		t.Skip("Integration tests skipped. See CONTRIBUTING.md for details")
	}

	projectId := testutil.ProjID()
	_ = projectId

	c, err := NewClient(ctx, option.WithTokenSource(ts))
	if err != nil {
		t.Fatal(err)
	}

	var minLikelihood dlppb.Likelihood = dlppb.Likelihood_POSSIBLE
	var inspectConfig = &dlppb.InspectConfig{
		MinLikelihood: minLikelihood,
	}
	var type_ string = "text/plain"
	var value string = "my phone number is 215-512-1212"
	var itemsElement = &dlppb.ContentItem{
		Type: type_,
		DataItem: &dlppb.ContentItem_Value{
			Value: value,
		},
	}
	var items = []*dlppb.ContentItem{itemsElement}
	var request = &dlppb.InspectContentRequest{
		InspectConfig: inspectConfig,
		Items:         items,
	}

	if _, err := c.InspectContent(ctx, request); err != nil {
		t.Error(err)
	}
}
