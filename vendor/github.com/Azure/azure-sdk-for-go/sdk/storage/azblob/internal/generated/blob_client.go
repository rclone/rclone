//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.

package generated

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"time"
)

// used to convert times from UTC to GMT before sending across the wire
var gmt = time.FixedZone("GMT", 0)

func (client *BlobClient) Endpoint() string {
	return client.endpoint
}

func (client *BlobClient) Pipeline() runtime.Pipeline {
	return client.pl
}

func (client *BlobClient) DeleteCreateRequest(ctx context.Context, options *BlobClientDeleteOptions, leaseAccessConditions *LeaseAccessConditions, modifiedAccessConditions *ModifiedAccessConditions) (*policy.Request, error) {
	return client.deleteCreateRequest(ctx, options, leaseAccessConditions, modifiedAccessConditions)
}

func (client *BlobClient) SetTierCreateRequest(ctx context.Context, tier AccessTier, options *BlobClientSetTierOptions, leaseAccessConditions *LeaseAccessConditions, modifiedAccessConditions *ModifiedAccessConditions) (*policy.Request, error) {
	return client.setTierCreateRequest(ctx, tier, options, leaseAccessConditions, modifiedAccessConditions)
}
