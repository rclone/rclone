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
	"io"
	"time"

	gax "github.com/googleapis/gax-go"
	"golang.org/x/net/context"
	pb "google.golang.org/genproto/googleapis/firestore/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Implementation of realtime updates (a.k.a. watch).
// This code is closely based on the Node.js implementation,
// https://github.com/googleapis/nodejs-firestore/blob/master/src/watch.js.

var defaultBackoff = gax.Backoff{
	// Values from https://github.com/googleapis/nodejs-firestore/blob/master/src/backoff.js.
	Initial:    1 * time.Second,
	Max:        60 * time.Second,
	Multiplier: 1.5,
}

type watchStream struct {
	ctx     context.Context
	c       *Client
	target  *pb.Target // document or query being watched
	lc      pb.Firestore_ListenClient
	backoff gax.Backoff
}

func newWatchStream(ctx context.Context, c *Client, target *pb.Target) *watchStream {
	return &watchStream{
		ctx:     ctx,
		c:       c,
		target:  target,
		backoff: defaultBackoff,
	}
}

// recv receives the next message from the stream. It also handles opening the stream
// initially, and reopening it on non-permanent errors.
// recv doesn't have to be goroutine-safe.
func (s *watchStream) recv() (*pb.ListenResponse, error) {
	var err error
	for {
		if s.lc == nil {
			s.lc, err = s.open()
			if err != nil {
				// Do not retry if open fails.
				return nil, err
			}
		}
		res, err := s.lc.Recv()
		if err == nil || isPermanentWatchError(err) {
			return res, err
		}
		// Non-permanent error. Sleep and retry.
		// TODO: from node:
		// request.addTarget.resumeToken = resumeToken;
		// changeMap.clear();
		dur := s.backoff.Pause()
		// If we're out of quota, wait a long time before retrying.
		if status.Code(err) == codes.ResourceExhausted {
			dur = s.backoff.Max
		}
		if err := gax.Sleep(s.ctx, dur); err != nil {
			return nil, err
		}
		s.lc = nil
	}
}

func (s *watchStream) open() (pb.Firestore_ListenClient, error) {
	lc, err := s.c.c.Listen(s.ctx)
	if err == nil {
		err = lc.Send(&pb.ListenRequest{
			Database:     s.c.path(),
			TargetChange: &pb.ListenRequest_AddTarget{AddTarget: s.target},
		})
	}
	if err != nil {
		return nil, err
	}
	return lc, nil
}

func isPermanentWatchError(err error) bool {
	if err == io.EOF {
		// Retry on normal end-of-stream.
		return false
	}
	switch status.Code(err) {
	case codes.Canceled, codes.Unknown, codes.DeadlineExceeded, codes.ResourceExhausted,
		codes.Internal, codes.Unavailable, codes.Unauthenticated:
		return false
	default:
		return true
	}
}
