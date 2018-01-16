// Copyright 2017, Google Inc. All rights reserved.
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

// AUTO-GENERATED CODE. DO NOT EDIT.

package videointelligence_test

import (
	"cloud.google.com/go/videointelligence/apiv1"
	"golang.org/x/net/context"
	videointelligencepb "google.golang.org/genproto/googleapis/cloud/videointelligence/v1"
)

func ExampleNewClient() {
	ctx := context.Background()
	c, err := videointelligence.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use client.
	_ = c
}

func ExampleClient_AnnotateVideo() {
	ctx := context.Background()
	c, err := videointelligence.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &videointelligencepb.AnnotateVideoRequest{
	// TODO: Fill request struct fields.
	}
	op, err := c.AnnotateVideo(ctx, req)
	if err != nil {
		// TODO: Handle error.
	}

	resp, err := op.Wait(ctx)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use resp.
	_ = resp
}
