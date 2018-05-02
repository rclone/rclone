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

package pubsub

// This file provides a fake/mock in-memory pubsub server.

import (
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/internal/testutil"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	durpb "github.com/golang/protobuf/ptypes/duration"
	emptypb "github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	pb "google.golang.org/genproto/googleapis/pubsub/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeServer struct {
	pb.PublisherServer
	pb.SubscriberServer

	Addr string

	mu            sync.Mutex
	Acked         map[string]bool  // acked message IDs
	Deadlines     map[string]int32 // deadlines by message ID
	pullResponses []*pullResponse
	wg            sync.WaitGroup
	subs          map[string]*pb.Subscription
	topics        map[string]*pb.Topic
}

type pullResponse struct {
	msgs []*pb.ReceivedMessage
	err  error
}

func newFakeServer() (*fakeServer, error) {
	srv, err := testutil.NewServer()
	if err != nil {
		return nil, err
	}
	fake := &fakeServer{
		Addr:      srv.Addr,
		Acked:     map[string]bool{},
		Deadlines: map[string]int32{},
		subs:      map[string]*pb.Subscription{},
		topics:    map[string]*pb.Topic{},
	}
	pb.RegisterPublisherServer(srv.Gsrv, fake)
	pb.RegisterSubscriberServer(srv.Gsrv, fake)
	srv.Start()
	return fake, nil
}

// Each call to addStreamingPullMessages results in one StreamingPullResponse.
func (s *fakeServer) addStreamingPullMessages(msgs []*pb.ReceivedMessage) {
	s.pullResponses = append(s.pullResponses, &pullResponse{msgs, nil})
}

func (s *fakeServer) addStreamingPullError(err error) {
	s.pullResponses = append(s.pullResponses, &pullResponse{nil, err})
}

func (s *fakeServer) wait() {
	s.wg.Wait()
}

func (s *fakeServer) StreamingPull(stream pb.Subscriber_StreamingPullServer) error {
	s.wg.Add(1)
	defer s.wg.Done()
	errc := make(chan error, 1)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			req, err := stream.Recv()
			if err != nil {
				errc <- err
				return
			}
			s.mu.Lock()
			for _, id := range req.AckIds {
				s.Acked[id] = true
			}
			for i, id := range req.ModifyDeadlineAckIds {
				s.Deadlines[id] = req.ModifyDeadlineSeconds[i]
			}
			s.mu.Unlock()
		}
	}()
	// Send responses.
	for {
		s.mu.Lock()
		if len(s.pullResponses) == 0 {
			s.mu.Unlock()
			// Nothing to send, so wait for the client to shut down the stream.
			err := <-errc // a real error, or at least EOF
			if err == io.EOF {
				return nil
			}
			return err
		}
		pr := s.pullResponses[0]
		s.pullResponses = s.pullResponses[1:]
		s.mu.Unlock()
		if pr.err != nil {
			// Add a slight delay to ensure the server receives any
			// messages en route from the client before shutting down the stream.
			// This reduces flakiness of tests involving retry.
			time.Sleep(200 * time.Millisecond)
		}
		if pr.err == io.EOF {
			return nil
		}
		if pr.err != nil {
			return pr.err
		}
		// Return any error from Recv.
		select {
		case err := <-errc:
			return err
		default:
		}
		res := &pb.StreamingPullResponse{ReceivedMessages: pr.msgs}
		if err := stream.Send(res); err != nil {
			return err
		}
	}
}

const (
	minMessageRetentionDuration = 10 * time.Minute
	maxMessageRetentionDuration = 168 * time.Hour
)

var defaultMessageRetentionDuration = ptypes.DurationProto(maxMessageRetentionDuration)

func checkMRD(pmrd *durpb.Duration) error {
	mrd, err := ptypes.Duration(pmrd)
	if err != nil || mrd < minMessageRetentionDuration || mrd > maxMessageRetentionDuration {
		return status.Errorf(codes.InvalidArgument, "bad message_retention_duration %+v", pmrd)
	}
	return nil
}

func checkAckDeadline(ads int32) error {
	if ads < 10 || ads > 600 {
		// PubSub service returns Unknown.
		return status.Errorf(codes.Unknown, "bad ack_deadline_seconds: %d", ads)
	}
	return nil
}

func (s *fakeServer) CreateSubscription(ctx context.Context, sub *pb.Subscription) (*pb.Subscription, error) {
	if s.subs[sub.Name] != nil {
		return nil, status.Errorf(codes.AlreadyExists, "subscription %q", sub.Name)
	}
	sub2 := proto.Clone(sub).(*pb.Subscription)
	if err := checkAckDeadline(sub.AckDeadlineSeconds); err != nil {
		return nil, err
	}
	if sub.MessageRetentionDuration == nil {
		sub2.MessageRetentionDuration = defaultMessageRetentionDuration
	}
	if err := checkMRD(sub2.MessageRetentionDuration); err != nil {
		return nil, err
	}
	if sub.PushConfig == nil {
		sub2.PushConfig = &pb.PushConfig{}
	}
	s.subs[sub.Name] = sub2
	return sub2, nil
}

func (s *fakeServer) GetSubscription(ctx context.Context, req *pb.GetSubscriptionRequest) (*pb.Subscription, error) {
	if sub := s.subs[req.Subscription]; sub != nil {
		return sub, nil
	}
	return nil, status.Errorf(codes.NotFound, "subscription %q", req.Subscription)
}

func (s *fakeServer) UpdateSubscription(ctx context.Context, req *pb.UpdateSubscriptionRequest) (*pb.Subscription, error) {
	sub := s.subs[req.Subscription.Name]
	if sub == nil {
		return nil, status.Errorf(codes.NotFound, "subscription %q", req.Subscription.Name)
	}
	for _, path := range req.UpdateMask.Paths {
		switch path {
		case "push_config":
			sub.PushConfig = req.Subscription.PushConfig

		case "ack_deadline_seconds":
			a := req.Subscription.AckDeadlineSeconds
			if err := checkAckDeadline(a); err != nil {
				return nil, err
			}
			sub.AckDeadlineSeconds = a

		case "retain_acked_messages":
			sub.RetainAckedMessages = req.Subscription.RetainAckedMessages

		case "message_retention_duration":
			if err := checkMRD(req.Subscription.MessageRetentionDuration); err != nil {
				return nil, err
			}
			sub.MessageRetentionDuration = req.Subscription.MessageRetentionDuration

			// TODO(jba): labels
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unknown field name %q", path)
		}
	}
	return sub, nil
}

func (s *fakeServer) DeleteSubscription(_ context.Context, req *pb.DeleteSubscriptionRequest) (*emptypb.Empty, error) {
	if s.subs[req.Subscription] == nil {
		return nil, status.Errorf(codes.NotFound, "subscription %q", req.Subscription)
	}
	delete(s.subs, req.Subscription)
	return &emptypb.Empty{}, nil
}

func (s *fakeServer) CreateTopic(_ context.Context, t *pb.Topic) (*pb.Topic, error) {
	if s.topics[t.Name] != nil {
		return nil, status.Errorf(codes.AlreadyExists, "topic %q", t.Name)
	}
	t2 := proto.Clone(t).(*pb.Topic)
	s.topics[t.Name] = t2
	return t2, nil
}

func (s *fakeServer) GetTopic(_ context.Context, req *pb.GetTopicRequest) (*pb.Topic, error) {
	if t := s.topics[req.Topic]; t != nil {
		return t, nil
	}
	return nil, status.Errorf(codes.NotFound, "topic %q", req.Topic)
}

func (s *fakeServer) DeleteTopic(_ context.Context, req *pb.DeleteTopicRequest) (*emptypb.Empty, error) {
	if s.topics[req.Topic] == nil {
		return nil, status.Errorf(codes.NotFound, "topic %q", req.Topic)
	}
	delete(s.topics, req.Topic)
	return &emptypb.Empty{}, nil
}

func (s *fakeServer) ListTopics(_ context.Context, req *pb.ListTopicsRequest) (*pb.ListTopicsResponse, error) {
	var names []string
	for n := range s.topics {
		if strings.HasPrefix(n, req.Project) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	from, to, nextToken, err := testutil.PageBounds(int(req.PageSize), req.PageToken, len(names))
	if err != nil {
		return nil, err
	}
	res := &pb.ListTopicsResponse{NextPageToken: nextToken}
	for i := from; i < to; i++ {
		res.Topics = append(res.Topics, s.topics[names[i]])
	}
	return res, nil
}

func (s *fakeServer) ListSubscriptions(_ context.Context, req *pb.ListSubscriptionsRequest) (*pb.ListSubscriptionsResponse, error) {
	var names []string
	for _, sub := range s.subs {
		if strings.HasPrefix(sub.Name, req.Project) {
			names = append(names, sub.Name)
		}
	}
	sort.Strings(names)
	from, to, nextToken, err := testutil.PageBounds(int(req.PageSize), req.PageToken, len(names))
	if err != nil {
		return nil, err
	}
	res := &pb.ListSubscriptionsResponse{NextPageToken: nextToken}
	for i := from; i < to; i++ {
		res.Subscriptions = append(res.Subscriptions, s.subs[names[i]])
	}
	return res, nil
}

func (s *fakeServer) ListTopicSubscriptions(_ context.Context, req *pb.ListTopicSubscriptionsRequest) (*pb.ListTopicSubscriptionsResponse, error) {
	var names []string
	for _, sub := range s.subs {
		if sub.Topic == req.Topic {
			names = append(names, sub.Name)
		}
	}
	sort.Strings(names)
	from, to, nextToken, err := testutil.PageBounds(int(req.PageSize), req.PageToken, len(names))
	if err != nil {
		return nil, err
	}
	return &pb.ListTopicSubscriptionsResponse{
		Subscriptions: names[from:to],
		NextPageToken: nextToken,
	}, nil
}
