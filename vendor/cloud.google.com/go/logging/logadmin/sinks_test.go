// Copyright 2016 Google Inc. All Rights Reserved.
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

// TODO(jba): document in CONTRIBUTING.md that service account must be given "Logs Configuration Writer" IAM role for sink tests to pass.
// TODO(jba): [cont] (1) From top left menu, go to IAM & Admin. (2) In Roles dropdown for acct, select Logging > Logs Configuration Writer. (3) Save.
// TODO(jba): Also, cloud-logs@google.com must have Owner permission on the GCS bucket named for the test project.

package logadmin

import (
	"log"
	"testing"
	"time"

	"cloud.google.com/go/internal/testutil"
	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var sinkIDs = testutil.NewUIDSpace("GO-CLIENT-TEST-SINK")

const testFilter = ""

var testSinkDestination string

// Called just before TestMain calls m.Run.
// Returns a cleanup function to be called after the tests finish.
func initSinks(ctx context.Context) func() {
	// Create a unique GCS bucket so concurrent tests don't interfere with each other.
	bucketIDs := testutil.NewUIDSpace(testProjectID + "-log-sink")
	testBucket := bucketIDs.New()
	testSinkDestination = "storage.googleapis.com/" + testBucket
	var storageClient *storage.Client
	if integrationTest {
		// Create a unique bucket as a sink destination, and give the cloud logging account
		// owner right.
		ts := testutil.TokenSource(ctx, storage.ScopeFullControl)
		var err error
		storageClient, err = storage.NewClient(ctx, option.WithTokenSource(ts))
		if err != nil {
			log.Fatalf("new storage client: %v", err)
		}
		bucket := storageClient.Bucket(testBucket)
		if err := bucket.Create(ctx, testProjectID, nil); err != nil {
			log.Fatalf("creating storage bucket %q: %v", testBucket, err)
		}
		if err := bucket.ACL().Set(ctx, "group-cloud-logs@google.com", storage.RoleOwner); err != nil {
			log.Fatalf("setting owner role: %v", err)
		}
	}
	// Clean up from aborted tests.
	it := client.Sinks(ctx)
	for {
		s, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("listing sinks: %v", err)
			break
		}
		if sinkIDs.Older(s.ID, 24*time.Hour) {
			client.DeleteSink(ctx, s.ID) // ignore error
		}
	}
	if integrationTest {
		for _, bn := range bucketNames(ctx, storageClient) {
			if bucketIDs.Older(bn, 24*time.Hour) {
				storageClient.Bucket(bn).Delete(ctx) // ignore error
			}
		}
		return func() {
			if err := storageClient.Bucket(testBucket).Delete(ctx); err != nil {
				log.Printf("deleting %q: %v", testBucket, err)
			}
			storageClient.Close()
		}
	}
	return func() {}
}

// Collect the name of all buckets for the test project.
func bucketNames(ctx context.Context, client *storage.Client) []string {
	var names []string
	it := client.Buckets(ctx, testProjectID)
loop:
	for {
		b, err := it.Next()
		switch err {
		case nil:
			names = append(names, b.Name)
		case iterator.Done:
			break loop
		default:
			log.Printf("listing buckets: %v", err)
			break loop
		}
	}
	return names
}

func TestCreateDeleteSink(t *testing.T) {
	ctx := context.Background()
	sink := &Sink{
		ID:          sinkIDs.New(),
		Destination: testSinkDestination,
		Filter:      testFilter,
	}
	got, err := client.CreateSink(ctx, sink)
	if err != nil {
		t.Fatal(err)
	}
	defer client.DeleteSink(ctx, sink.ID)
	if want := sink; !testutil.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	got, err = client.Sink(ctx, sink.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := sink; !testutil.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	if err := client.DeleteSink(ctx, sink.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := client.Sink(ctx, sink.ID); err == nil {
		t.Fatal("got no error, expected one")
	}
}

func TestUpdateSink(t *testing.T) {
	ctx := context.Background()
	sink := &Sink{
		ID:          sinkIDs.New(),
		Destination: testSinkDestination,
		Filter:      testFilter,
	}

	if _, err := client.CreateSink(ctx, sink); err != nil {
		t.Fatal(err)
	}
	got, err := client.UpdateSink(ctx, sink)
	if err != nil {
		t.Fatal(err)
	}
	defer client.DeleteSink(ctx, sink.ID)
	if want := sink; !testutil.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	got, err = client.Sink(ctx, sink.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := sink; !testutil.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}

	// Updating an existing sink changes it.
	sink.Filter = ""
	if _, err := client.UpdateSink(ctx, sink); err != nil {
		t.Fatal(err)
	}
	got, err = client.Sink(ctx, sink.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := sink; !testutil.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestListSinks(t *testing.T) {
	ctx := context.Background()
	var sinks []*Sink
	want := map[string]*Sink{}
	for i := 0; i < 4; i++ {
		s := &Sink{
			ID:          sinkIDs.New(),
			Destination: testSinkDestination,
			Filter:      testFilter,
		}
		sinks = append(sinks, s)
		want[s.ID] = s
	}
	for _, s := range sinks {
		if _, err := client.CreateSink(ctx, s); err != nil {
			t.Fatalf("Create(%q): %v", s.ID, err)
		}
		defer client.DeleteSink(ctx, s.ID)
	}

	got := map[string]*Sink{}
	it := client.Sinks(ctx)
	for {
		s, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		// If tests run simultaneously, we may have more sinks than we
		// created. So only check for our own.
		if _, ok := want[s.ID]; ok {
			got[s.ID] = s
		}
	}
	if !testutil.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
