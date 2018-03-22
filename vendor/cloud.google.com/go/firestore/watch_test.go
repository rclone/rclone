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

package firestore

import (
	"testing"

	"github.com/golang/protobuf/proto"
	gax "github.com/googleapis/gax-go"
	"golang.org/x/net/context"
	pb "google.golang.org/genproto/googleapis/firestore/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWatchRecv(t *testing.T) {
	ctx := context.Background()
	c, srv := newMock(t)
	db := defaultBackoff
	defaultBackoff = gax.Backoff{Initial: 1, Max: 1, Multiplier: 1}
	defer func() { defaultBackoff = db }()

	ws := newWatchStream(ctx, c, &pb.Target{})
	request := &pb.ListenRequest{
		Database:     "projects/projectID/databases/(default)",
		TargetChange: &pb.ListenRequest_AddTarget{&pb.Target{}},
	}
	response := &pb.ListenResponse{ResponseType: &pb.ListenResponse_DocumentChange{&pb.DocumentChange{}}}
	// Stream should retry on non-permanent errors, returning only the responses.
	srv.addRPC(request, []interface{}{response, status.Error(codes.Unknown, "")})
	srv.addRPC(request, []interface{}{response}) // stream will return io.EOF
	srv.addRPC(request, []interface{}{response, status.Error(codes.DeadlineExceeded, "")})
	srv.addRPC(request, []interface{}{status.Error(codes.ResourceExhausted, "")})
	srv.addRPC(request, []interface{}{status.Error(codes.Internal, "")})
	srv.addRPC(request, []interface{}{status.Error(codes.Unavailable, "")})
	srv.addRPC(request, []interface{}{status.Error(codes.Unauthenticated, "")})
	srv.addRPC(request, []interface{}{response})
	for i := 0; i < 4; i++ {
		res, err := ws.recv()
		if err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(res, response) {
			t.Fatalf("got %v, want %v", res, response)
		}
	}

	// Stream should not retry on a permanent error.
	srv.addRPC(request, []interface{}{status.Error(codes.AlreadyExists, "")})
	_, err := ws.recv()
	if got, want := status.Code(err), codes.AlreadyExists; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}
