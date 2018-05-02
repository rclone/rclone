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

	"cloud.google.com/go/internal/testutil"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/api/googleapi"
	raw "google.golang.org/api/storage/v1"
)

func TestBucketAttrsToRawBucket(t *testing.T) {
	t.Parallel()
	attrs := &BucketAttrs{
		Name:             "name",
		ACL:              []ACLRule{{Entity: "bob@example.com", Role: RoleOwner}},
		DefaultObjectACL: []ACLRule{{Entity: AllUsers, Role: RoleReader}},
		Location:         "loc",
		StorageClass:     "class",
		RetentionPolicy: &RetentionPolicy{
			RetentionPeriod: 3 * time.Second,
		},
		VersioningEnabled: false,
		// should be ignored:
		MetaGeneration: 39,
		Created:        time.Now(),
		Labels:         map[string]string{"label": "value"},
		CORS: []CORS{
			{
				MaxAge:          time.Hour,
				Methods:         []string{"GET", "POST"},
				Origins:         []string{"*"},
				ResponseHeaders: []string{"FOO"},
			},
		},
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
		RetentionPolicy: &raw.BucketRetentionPolicy{
			RetentionPeriod: 3,
		},
		Versioning: nil, // ignore VersioningEnabled if false
		Labels:     map[string]string{"label": "value"},
		Cors: []*raw.BucketCors{
			{
				MaxAgeSeconds:  3600,
				Method:         []string{"GET", "POST"},
				Origin:         []string{"*"},
				ResponseHeader: []string{"FOO"},
			},
		},
	}
	if msg := testutil.Diff(got, want); msg != "" {
		t.Error(msg)
	}

	attrs.VersioningEnabled = true
	attrs.RequesterPays = true
	got = attrs.toRawBucket()
	want.Versioning = &raw.BucketVersioning{Enabled: true}
	want.Billing = &raw.BucketBilling{RequesterPays: true}
	if msg := testutil.Diff(got, want); msg != "" {
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
	if msg := testutil.Diff(got, want); msg != "" {
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

	if msg := testutil.Diff(got, want); msg != "" {
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
				return b.newPatchCall(&BucketAttrsToUpdate{
					VersioningEnabled: false,
					RequesterPays:     false,
				})
			},
			rc.Buckets.Patch("name", &raw.Bucket{
				Versioning: &raw.BucketVersioning{
					Enabled:         false,
					ForceSendFields: []string{"Enabled"},
				},
				Billing: &raw.BucketBilling{
					RequesterPays:   false,
					ForceSendFields: []string{"RequesterPays"},
				},
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

func TestNewBucket(t *testing.T) {
	labels := map[string]string{"a": "b"}
	matchClasses := []string{"MULTI_REGIONAL", "REGIONAL", "STANDARD"}
	rb := &raw.Bucket{
		Name:           "name",
		Location:       "loc",
		Metageneration: 3,
		StorageClass:   "sc",
		TimeCreated:    "2017-10-23T04:05:06Z",
		Versioning:     &raw.BucketVersioning{Enabled: true},
		Labels:         labels,
		Billing:        &raw.BucketBilling{RequesterPays: true},
		Lifecycle: &raw.BucketLifecycle{
			Rule: []*raw.BucketLifecycleRule{{
				Action: &raw.BucketLifecycleRuleAction{
					Type:         "SetStorageClass",
					StorageClass: "NEARLINE",
				},
				Condition: &raw.BucketLifecycleRuleCondition{
					Age:                 10,
					IsLive:              googleapi.Bool(true),
					CreatedBefore:       "2017-01-02",
					MatchesStorageClass: matchClasses,
					NumNewerVersions:    3,
				},
			}},
		},
		RetentionPolicy: &raw.BucketRetentionPolicy{
			RetentionPeriod: 3,
			EffectiveTime:   time.Now().Format(time.RFC3339),
		},
		Cors: []*raw.BucketCors{
			{
				MaxAgeSeconds:  3600,
				Method:         []string{"GET", "POST"},
				Origin:         []string{"*"},
				ResponseHeader: []string{"FOO"},
			},
		},
		Acl: []*raw.BucketAccessControl{
			{Bucket: "name", Role: "READER", Email: "joe@example.com", Entity: "allUsers"},
		},
	}
	want := &BucketAttrs{
		Name:              "name",
		Location:          "loc",
		MetaGeneration:    3,
		StorageClass:      "sc",
		Created:           time.Date(2017, 10, 23, 4, 5, 6, 0, time.UTC),
		VersioningEnabled: true,
		Labels:            labels,
		RequesterPays:     true,
		Lifecycle: Lifecycle{
			Rules: []LifecycleRule{
				{
					Action: LifecycleAction{
						Type:         SetStorageClassAction,
						StorageClass: "NEARLINE",
					},
					Condition: LifecycleCondition{
						AgeInDays:             10,
						Liveness:              Live,
						CreatedBefore:         time.Date(2017, 1, 2, 0, 0, 0, 0, time.UTC),
						MatchesStorageClasses: matchClasses,
						NumNewerVersions:      3,
					},
				},
			},
		},
		RetentionPolicy: &RetentionPolicy{
			RetentionPeriod: 3 * time.Second,
		},
		CORS: []CORS{
			{
				MaxAge:          time.Hour,
				Methods:         []string{"GET", "POST"},
				Origins:         []string{"*"},
				ResponseHeaders: []string{"FOO"},
			},
		},
		ACL:              []ACLRule{{Entity: "allUsers", Role: RoleReader}},
		DefaultObjectACL: []ACLRule{},
	}
	got, err := newBucket(rb)
	if err != nil {
		t.Fatal(err)
	}
	if diff := testutil.Diff(got, want, cmpopts.IgnoreTypes(time.Time{})); diff != "" {
		t.Errorf("got=-, want=+:\n%s", diff)
	}
}
