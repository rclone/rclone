// Copyright (c) 2015 Serge Gebhardt. All rights reserved.
//
// Use of this source code is governed by the ISC
// license that can be found in the LICENSE file.

package acd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNode_getRoot(t *testing.T) {
	r := *NewMockResponseOkString(`
{
	"count":1,
	"data":[
		{
			"isRoot":true,
			"eTagResponse":"LMel82iwbNk",
			"id":"3ohaT2SSQWOecmP0GSWv6g",
			"kind":"FOLDER",
			"version":156,
			"labels":[

			],
			"createdDate":"2014-04-08T20:58:58.011Z",
			"createdBy":"CloudDriveFiles",
			"restricted":false,
			"modifiedDate":"2015-05-03T16:12:35.394Z",
			"isShared":false,
			"parents":[

			],
			"status":"AVAILABLE"
		}
	]
}
`)
	c := NewMockClient(r)

	root, _, err := c.Nodes.GetRoot()

	assert.NoError(t, err)
	assert.Equal(t, "3ohaT2SSQWOecmP0GSWv6g", *root.Id)
	assert.Nil(t, root.Name)
}

func TestNode_getNodes(t *testing.T) {
	r := *NewMockResponseOkString(`
{
	"count":2,
	"nextToken":"kgkbpodpt6",
	"data":[
		{
			"eTagResponse":"eodh1-sfNbMI",
			"id":"eRkZ6YMuX5W3VqV3Ia7_lf",
			"name":"fooNew.jpg",
			"kind":"FILE",
			"metadataVersion":1,
			"modifiedDate":"2014-03-07T22:31:12.173Z",
			"creationDate":"2014-03-07T22:31:12.173Z",
			"labels":[
				"PHOTO"
			],
			"description":"My Awesome Photo",
			"createdBy":"ApplicationId1",
			"parents":[
				"foo1",
				"123"
			],
			"status":"Available",
			"restricted":false,
			"size":56654,
			"contentType":"image/jpeg",
			"md5":"6df23dc03f9b54cc38a0fc1483df6e21",
			"fileExtension":"jpeg",
			"contentProperties":{
				"image":{
					"make":"SAMSUNG",
					"model":"SAMSUNG-SGH-I747",
					"exposureTime":"1/1780",
					"dateTimeOriginal":"2012-08-25T14:23:24.000Z",
					"flash":"No",
					"focalLength":"37/10",
					"dateTime":"2012-08-25T14:23:24.000Z",
					"dateTimeDigitized":"2012-08-25T14:23:24.000Z",
					"software":"I747UCALG1",
					"orientation":"1",
					"colorSpace":"sRGB",
					"meteringMode":"CenterWeightedAverage",
					"exposureProgram":"Aperture Priority",
					"exposureMode":"Auto Exposure",
					"whiteBalance":"Auto",
					"sensingMethod":"One-chip color area",
					"xResolution":"72",
					"yResolution":"72",
					"resolutionUnit":"Pixels/Inch"
				}
			}
		},
		{
			"eTagResponse":"sdgrrtbbfdd",
			"id":"fooo1",
			"name":"foo.zip",
			"kind":"FILE",
			"metadataVersion":1,
			"modifiedDate":"2014-03-07T22:31:12.173Z",
			"creationDate":"2014-03-07T22:31:12.173Z",
			"labels":[
				"ZIP File"
			],
			"description":"All My Data",
			"createdBy":"ApplicationId2",
			"status":"Available",
			"restricted":false,
			"size":5665423,
			"contentType":"application/octet-stream",
			"md5":"6df23dc03f9b54cc38a0fc1483df6e23",
			"fileExtension":"zip"
		}
	]
}
`)
	c := NewMockClient(r)
	opts := &NodeListOptions{}

	nodes, _, err := c.Nodes.GetNodes(opts)

	assert.NoError(t, err)
	assert.Equal(t, "kgkbpodpt6", opts.StartToken)
	assert.Equal(t, 2, len(nodes))

	assert.Equal(t, "eRkZ6YMuX5W3VqV3Ia7_lf", *nodes[0].Id)
	assert.Equal(t, "fooNew.jpg", *nodes[0].Name)

	assert.Equal(t, "fooo1", *nodes[1].Id)
	assert.Equal(t, "foo.zip", *nodes[1].Name)
}

func TestEscapeForFilter(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"potato", "potato"},
		{`potato+sausage`, `potato\+sausage`},
		{`+ - & | ! ( ) { } [ ] ^ ' " ~ * ? : \`, `\+\ \-\ \&\ \|\ \!\ \(\ \)\ \{\ \}\ \[\ \]\ \^\ \'\ \"\ \~\ \*\ \?\ \:\ \\`},
	} {
		got := EscapeForFilter(test.in)
		if test.want != got {
			t.Errorf("in(%q): want '%s' got '%s'", test.in, test.want, got)
		}
	}
}
