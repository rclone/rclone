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

package dlp_test

import (
	"cloud.google.com/go/dlp/apiv2beta1"
	"golang.org/x/net/context"
	dlppb "google.golang.org/genproto/googleapis/privacy/dlp/v2beta1"
)

func ExampleNewClient() {
	ctx := context.Background()
	c, err := dlp.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use client.
	_ = c
}

func ExampleClient_InspectContent() {
	ctx := context.Background()
	c, err := dlp.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &dlppb.InspectContentRequest{
		// TODO: Fill request struct fields.
	}
	resp, err := c.InspectContent(ctx, req)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use resp.
	_ = resp
}

func ExampleClient_RedactContent() {
	ctx := context.Background()
	c, err := dlp.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &dlppb.RedactContentRequest{
		// TODO: Fill request struct fields.
	}
	resp, err := c.RedactContent(ctx, req)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use resp.
	_ = resp
}

func ExampleClient_DeidentifyContent() {
	ctx := context.Background()
	c, err := dlp.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &dlppb.DeidentifyContentRequest{
		// TODO: Fill request struct fields.
	}
	resp, err := c.DeidentifyContent(ctx, req)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use resp.
	_ = resp
}

func ExampleClient_AnalyzeDataSourceRisk() {
	ctx := context.Background()
	c, err := dlp.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &dlppb.AnalyzeDataSourceRiskRequest{
		// TODO: Fill request struct fields.
	}
	op, err := c.AnalyzeDataSourceRisk(ctx, req)
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

func ExampleClient_CreateInspectOperation() {
	ctx := context.Background()
	c, err := dlp.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &dlppb.CreateInspectOperationRequest{
		// TODO: Fill request struct fields.
	}
	op, err := c.CreateInspectOperation(ctx, req)
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

func ExampleClient_ListInspectFindings() {
	ctx := context.Background()
	c, err := dlp.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &dlppb.ListInspectFindingsRequest{
		// TODO: Fill request struct fields.
	}
	resp, err := c.ListInspectFindings(ctx, req)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use resp.
	_ = resp
}

func ExampleClient_ListInfoTypes() {
	ctx := context.Background()
	c, err := dlp.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &dlppb.ListInfoTypesRequest{
		// TODO: Fill request struct fields.
	}
	resp, err := c.ListInfoTypes(ctx, req)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use resp.
	_ = resp
}

func ExampleClient_ListRootCategories() {
	ctx := context.Background()
	c, err := dlp.NewClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &dlppb.ListRootCategoriesRequest{
		// TODO: Fill request struct fields.
	}
	resp, err := c.ListRootCategories(ctx, req)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use resp.
	_ = resp
}
