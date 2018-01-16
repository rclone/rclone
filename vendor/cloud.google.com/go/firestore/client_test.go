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

package firestore

import (
	"testing"

	"golang.org/x/net/context"
	pb "google.golang.org/genproto/googleapis/firestore/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var testClient = &Client{
	projectID:  "projectID",
	databaseID: "(default)",
}

func TestClientCollectionAndDoc(t *testing.T) {
	coll1 := testClient.Collection("X")
	db := "projects/projectID/databases/(default)"
	wantc1 := &CollectionRef{
		c:          testClient,
		parentPath: db,
		Parent:     nil,
		ID:         "X",
		Path:       "projects/projectID/databases/(default)/documents/X",
		Query:      Query{c: testClient, collectionID: "X", parentPath: db},
	}
	if !testEqual(coll1, wantc1) {
		t.Fatalf("got\n%+v\nwant\n%+v", coll1, wantc1)
	}
	doc1 := testClient.Doc("X/a")
	wantd1 := &DocumentRef{
		Parent: coll1,
		ID:     "a",
		Path:   "projects/projectID/databases/(default)/documents/X/a",
	}

	if !testEqual(doc1, wantd1) {
		t.Fatalf("got %+v, want %+v", doc1, wantd1)
	}
	coll2 := testClient.Collection("X/a/Y")
	parentPath := "projects/projectID/databases/(default)/documents/X/a"
	wantc2 := &CollectionRef{
		c:          testClient,
		parentPath: parentPath,
		Parent:     doc1,
		ID:         "Y",
		Path:       "projects/projectID/databases/(default)/documents/X/a/Y",
		Query:      Query{c: testClient, collectionID: "Y", parentPath: parentPath},
	}
	if !testEqual(coll2, wantc2) {
		t.Fatalf("\ngot  %+v\nwant %+v", coll2, wantc2)
	}
	doc2 := testClient.Doc("X/a/Y/b")
	wantd2 := &DocumentRef{
		Parent: coll2,
		ID:     "b",
		Path:   "projects/projectID/databases/(default)/documents/X/a/Y/b",
	}
	if !testEqual(doc2, wantd2) {
		t.Fatalf("got %+v, want %+v", doc2, wantd2)
	}
}

func TestClientCollDocErrors(t *testing.T) {
	for _, badColl := range []string{"", "/", "/a/", "/a/b", "a/b/", "a//b"} {
		coll := testClient.Collection(badColl)
		if coll != nil {
			t.Errorf("coll path %q: got %+v, want nil", badColl, coll)
		}
	}
	for _, badDoc := range []string{"", "a", "/", "/a", "a/", "a/b/c", "a//b/c"} {
		doc := testClient.Doc(badDoc)
		if doc != nil {
			t.Errorf("doc path %q: got %+v, want nil", badDoc, doc)
		}
	}
}

func TestGetAll(t *testing.T) {
	ctx := context.Background()
	const dbPath = "projects/projectID/databases/(default)"
	c, srv := newMock(t)
	defer c.Close()
	wantPBDocs := []*pb.Document{
		{
			Name:       dbPath + "/documents/C/a",
			CreateTime: aTimestamp,
			UpdateTime: aTimestamp,
			Fields:     map[string]*pb.Value{"f": intval(2)},
		},
		nil,
		{
			Name:       dbPath + "/documents/C/c",
			CreateTime: aTimestamp,
			UpdateTime: aTimestamp,
			Fields:     map[string]*pb.Value{"f": intval(1)},
		},
	}
	srv.addRPC(
		&pb.BatchGetDocumentsRequest{
			Database: dbPath,
			Documents: []string{
				dbPath + "/documents/C/a",
				dbPath + "/documents/C/b",
				dbPath + "/documents/C/c",
			},
		},
		[]interface{}{
			// deliberately put these out of order
			&pb.BatchGetDocumentsResponse{
				Result: &pb.BatchGetDocumentsResponse_Found{wantPBDocs[2]},
			},
			&pb.BatchGetDocumentsResponse{
				Result: &pb.BatchGetDocumentsResponse_Found{wantPBDocs[0]},
			},
			&pb.BatchGetDocumentsResponse{
				Result: &pb.BatchGetDocumentsResponse_Missing{dbPath + "/documents/C/b"},
			},
		},
	)
	coll := c.Collection("C")
	var docRefs []*DocumentRef
	for _, name := range []string{"a", "b", "c"} {
		docRefs = append(docRefs, coll.Doc(name))
	}
	docs, err := c.GetAll(ctx, docRefs)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(docs), len(wantPBDocs); got != want {
		t.Errorf("got %d docs, wanted %d", got, want)
	}
	for i, got := range docs {
		var want *DocumentSnapshot
		if wantPBDocs[i] != nil {
			want, err = newDocumentSnapshot(docRefs[i], wantPBDocs[i], c)
			if err != nil {
				t.Fatal(err)
			}
		}
		if diff := testDiff(got, want); diff != "" {
			t.Errorf("#%d: got=--, want==++\n%s", i, diff)
		}
	}
}

func TestGetAllErrors(t *testing.T) {
	ctx := context.Background()
	const (
		dbPath  = "projects/projectID/databases/(default)"
		docPath = dbPath + "/documents/C/a"
	)
	c, srv := newMock(t)
	if _, err := c.GetAll(ctx, []*DocumentRef{nil}); err != errNilDocRef {
		t.Errorf("got %v, want errNilDocRef", err)
	}

	// Internal server error.
	srv.addRPC(
		&pb.BatchGetDocumentsRequest{
			Database:  dbPath,
			Documents: []string{docPath},
		},
		[]interface{}{grpc.Errorf(codes.Internal, "")},
	)
	_, err := c.GetAll(ctx, []*DocumentRef{c.Doc("C/a")})
	codeEq(t, "GetAll #1", codes.Internal, err)

	// Doc appears as both found and missing (server bug).
	srv.reset()
	srv.addRPC(
		&pb.BatchGetDocumentsRequest{
			Database:  dbPath,
			Documents: []string{docPath},
		},
		[]interface{}{
			&pb.BatchGetDocumentsResponse{
				Result: &pb.BatchGetDocumentsResponse_Found{&pb.Document{Name: docPath}},
			},
			&pb.BatchGetDocumentsResponse{
				Result: &pb.BatchGetDocumentsResponse_Missing{docPath},
			},
		},
	)
	if _, err := c.GetAll(ctx, []*DocumentRef{c.Doc("C/a")}); err == nil {
		t.Error("got nil, want error")
	}

	// Doc never appears (server bug).
	srv.reset()
	srv.addRPC(
		&pb.BatchGetDocumentsRequest{
			Database:  dbPath,
			Documents: []string{docPath},
		},
		[]interface{}{},
	)
	if _, err := c.GetAll(ctx, []*DocumentRef{c.Doc("C/a")}); err == nil {
		t.Error("got nil, want error")
	}
}
