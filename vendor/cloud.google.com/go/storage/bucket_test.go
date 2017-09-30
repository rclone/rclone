// Copyright 2017 Google Inc. All Rights Reserved.
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
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"cloud.google.com/go/internal/pretty"
	"cloud.google.com/go/internal/testutil"
	raw "google.golang.org/api/storage/v1"
)

func TestBucketAttrsToRawBucket(t *testing.T) {
	t.Parallel()
	attrs := &BucketAttrs{
		Name:              "name",
		ACL:               []ACLRule{{Entity: "bob@example.com", Role: RoleOwner}},
		DefaultObjectACL:  []ACLRule{{Entity: AllUsers, Role: RoleReader}},
		Location:          "loc",
		StorageClass:      "class",
		VersioningEnabled: false,
		// should be ignored:
		MetaGeneration: 39,
		Created:        time.Now(),
		Labels:         map[string]string{"label": "value"},
	}
	got := attrs.toRawBucket()
	want := &raw.Bucket{
		Name: "name",
		Acl: []*raw.BucketAccessControl{
			{Entity: "bob@example.com", Role: "OWNER"},
		},
		DefaultObjectAcl: []*raw.ObjectAccessControl{
			{Entity: "allUsers", Role: "READER"},
		},
		Location:     "loc",
		StorageClass: "class",
		Versioning:   nil, // ignore VersioningEnabled if flase
		Labels:       map[string]string{"label": "value"},
	}
	msg, ok, err := pretty.Diff(want, got)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error(msg)
	}

	attrs.VersioningEnabled = true
	attrs.RequesterPays = true
	got = attrs.toRawBucket()
	want.Versioning = &raw.BucketVersioning{Enabled: true}
	want.Billing = &raw.BucketBilling{RequesterPays: true}
	msg, ok, err = pretty.Diff(want, got)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error(msg)
	}
}

func TestBucketAttrsToUpdateToRawBucket(t *testing.T) {
	t.Parallel()
	au := &BucketAttrsToUpdate{
		VersioningEnabled: false,
		RequesterPays:     false,
	}
	au.SetLabel("a", "foo")
	au.DeleteLabel("b")
	au.SetLabel("c", "")
	got := au.toRawBucket()
	want := &raw.Bucket{
		Versioning: &raw.BucketVersioning{
			Enabled:         false,
			ForceSendFields: []string{"Enabled"},
		},
		Labels: map[string]string{
			"a": "foo",
			"c": "",
		},
		Billing: &raw.BucketBilling{
			RequesterPays:   false,
			ForceSendFields: []string{"RequesterPays"},
		},
		NullFields: []string{"Labels.b"},
	}
	msg, ok, err := pretty.Diff(want, got)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error(msg)
	}

	var au2 BucketAttrsToUpdate
	au2.DeleteLabel("b")
	got = au2.toRawBucket()
	want = &raw.Bucket{
		Labels:          map[string]string{},
		ForceSendFields: []string{"Labels"},
		NullFields:      []string{"Labels.b"},
	}
	msg, ok, err = pretty.Diff(want, got)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error(msg)
	}

}

func TestCallBuilders(t *testing.T) {
	rc, err := raw.New(&http.Client{})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{raw: rc}
	const metagen = 17

	b := c.Bucket("name")
	bm := b.If(BucketConditions{MetagenerationMatch: metagen}).UserProject("p")

	equal := func(x, y interface{}) bool {
		return testutil.Equal(x, y,
			cmp.AllowUnexported(
				raw.BucketsGetCall{},
				raw.BucketsDeleteCall{},
				raw.BucketsPatchCall{},
			),
			cmp.FilterPath(func(p cmp.Path) bool {
				return p[len(p)-1].Type() == reflect.TypeOf(&raw.Service{})
			}, cmp.Ignore()),
		)
	}

	for i, test := range []struct {
		callFunc func(*BucketHandle) (interface{}, error)
		want     interface {
			Header() http.Header
		}
		metagenFunc func(interface{})
	}{
		{
			func(b *BucketHandle) (interface{}, error) { return b.newGetCall() },
			rc.Buckets.Get("name").Projection("full"),
			func(req interface{}) { req.(*raw.BucketsGetCall).IfMetagenerationMatch(metagen).UserProject("p") },
		},
		{
			func(b *BucketHandle) (interface{}, error) { return b.newDeleteCall() },
			rc.Buckets.Delete("name"),
			func(req interface{}) { req.(*raw.BucketsDeleteCall).IfMetagenerationMatch(metagen).UserProject("p") },
		},
		{
			func(b *BucketHandle) (interface{}, error) {
				return b.newPatchCall(&BucketAttrsToUpdate{VersioningEnabled: false})
			},
			rc.Buckets.Patch("name", &raw.Bucket{
				Versioning: &raw.BucketVersioning{Enabled: false, ForceSendFields: []string{"Enabled"}},
			}).Projection("full"),
			func(req interface{}) { req.(*raw.BucketsPatchCall).IfMetagenerationMatch(metagen).UserProject("p") },
		},
	} {
		got, err := test.callFunc(b)
		if err != nil {
			t.Fatal(err)
		}
		setClientHeader(test.want.Header())
		if !equal(got, test.want) {
			t.Errorf("#%d: got %#v, want %#v", i, got, test.want)
		}
		got, err = test.callFunc(bm)
		if err != nil {
			t.Fatal(err)
		}
		test.metagenFunc(test.want)
		if !equal(got, test.want) {
			t.Errorf("#%d:\ngot  %#v\nwant %#v", i, got, test.want)
		}
	}

	// Error.
	bm = b.If(BucketConditions{MetagenerationMatch: 1, MetagenerationNotMatch: 2})
	if _, err := bm.newGetCall(); err == nil {
		t.Errorf("got nil, want error")
	}
	if _, err := bm.newDeleteCall(); err == nil {
		t.Errorf("got nil, want error")
	}
	if _, err := bm.newPatchCall(&BucketAttrsToUpdate{}); err == nil {
		t.Errorf("got nil, want error")
	}
}
