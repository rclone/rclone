//go:build go1.18
// +build go1.18

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.

package generated

import "github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"

func (client *PageBlobClient) Endpoint() string {
	return client.endpoint
}

func (client *PageBlobClient) Pipeline() runtime.Pipeline {
	return client.pl
}
