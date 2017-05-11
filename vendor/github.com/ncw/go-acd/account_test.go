// Copyright (c) 2015 Serge Gebhardt. All rights reserved.
//
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package acd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccount_getInfo(t *testing.T) {
	r := *NewMockResponseOkString(`{ "termsOfUse": "1.0.0", "status": "ACTIVE" }`)
	c := NewMockClient(r)

	info, _, err := c.Account.GetInfo()

	assert.NoError(t, err)
	assert.Equal(t, "ACTIVE", *info.Status)
	assert.Equal(t, "1.0.0", *info.TermsOfUse)
}

func TestAccount_getQuota(t *testing.T) {
	r := *NewMockResponseOkString(`
{
	"quota": 5368709120,
	"lastCalculated": "2014-08-13T23:01:47.479Z",
	"available": 4069088896
}
	`)
	c := NewMockClient(r)

	quota, _, err := c.Account.GetQuota()

	assert.NoError(t, err)
	assert.Equal(t, "2014-08-13 23:01:47.479 +0000 UTC", quota.LastCalculated.String())
	assert.Equal(t, uint64(5368709120), *quota.Quota)
	assert.Equal(t, uint64(4069088896), *quota.Available)
}

func TestAccount_getUsage(t *testing.T) {
	r := *NewMockResponseOkString(`
{
	"lastCalculated":"2014-08-13T23:17:41.365Z",
	"other":{
		"total":{
			"bytes":29999771,
			"count":871
		},
		"billable":{
			"bytes":29999771,
			"count":871
		}
	},
	"doc":{
		"total":{
			"bytes":807170,
			"count":10
		},
		"billable":{
			"bytes":807170,
			"count":10
		}
	},
	"photo":{
		"total":{
			"bytes":9477988,
			"count":25
		},
		"billable":{
			"bytes":9477988,
			"count":25
		}
	},
	"video":{
		"total":{
			"bytes":23524252,
			"count":22
		},
		"billable":{
			"bytes":23524252,
			"count":22
		}
	}
}
	`)
	c := NewMockClient(r)

	usage, _, err := c.Account.GetUsage()

	assert.NoError(t, err)
	assert.Equal(t, "2014-08-13 23:17:41.365 +0000 UTC", usage.LastCalculated.String())

	assert.Equal(t, uint64(29999771), *usage.Other.Total.Bytes)
	assert.Equal(t, uint64(871), *usage.Other.Total.Count)
	assert.Equal(t, uint64(29999771), *usage.Other.Billable.Bytes)
	assert.Equal(t, uint64(871), *usage.Other.Billable.Count)

	assert.Equal(t, uint64(807170), *usage.Doc.Total.Bytes)
	assert.Equal(t, uint64(10), *usage.Doc.Total.Count)
	assert.Equal(t, uint64(807170), *usage.Doc.Billable.Bytes)
	assert.Equal(t, uint64(10), *usage.Doc.Billable.Count)

	assert.Equal(t, uint64(9477988), *usage.Photo.Total.Bytes)
	assert.Equal(t, uint64(25), *usage.Photo.Total.Count)
	assert.Equal(t, uint64(9477988), *usage.Photo.Billable.Bytes)
	assert.Equal(t, uint64(25), *usage.Photo.Billable.Count)

	assert.Equal(t, uint64(23524252), *usage.Video.Total.Bytes)
	assert.Equal(t, uint64(22), *usage.Video.Total.Count)
	assert.Equal(t, uint64(23524252), *usage.Video.Billable.Bytes)
	assert.Equal(t, uint64(22), *usage.Video.Billable.Count)
}
