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

package cloud_test

import (
	"time"

	"cloud.google.com/go/bigquery"
	"golang.org/x/net/context"
)

// To set a timeout for an RPC, use context.WithTimeout.
func Example_timeout() {
	ctx := context.Background()
	// Do not set a timeout on the context passed to NewClient: dialing happens
	// asynchronously, and the context is used to refresh credentials in the
	// background.
	client, err := bigquery.NewClient(ctx, "project-id")
	if err != nil {
		// TODO: handle error.
	}
	// Time out if it takes more than 10 seconds to create a dataset.
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel() // Always call cancel.

	if err := client.Dataset("new-dataset").Create(tctx, nil); err != nil {
		// TODO: handle error.
	}
}

// To arrange for an RPC to be canceled, use context.WithCancel.
func Example_cancellation() {
	ctx := context.Background()
	// Do not cancel the context passed to NewClient: dialing happens asynchronously,
	// and the context is used to refresh credentials in the background.
	client, err := bigquery.NewClient(ctx, "project-id")
	if err != nil {
		// TODO: handle error.
	}
	cctx, cancel := context.WithCancel(ctx)
	defer cancel() // Always call cancel.

	// TODO: Make the cancel function available to whatever might want to cancel the
	// call--perhaps a GUI button.
	if err := client.Dataset("new-dataset").Create(cctx, nil); err != nil {
		// TODO: handle error.
	}
}
