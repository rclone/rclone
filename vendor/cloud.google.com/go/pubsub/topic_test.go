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

package pubsub

import (
	"net"
	"reflect"
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	pubsubpb "google.golang.org/genproto/googleapis/pubsub/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type topicListService struct {
	service
	topics []string
	err    error
	t      *testing.T // for error logging.
}

func (s *topicListService) newNextStringFunc() nextStringFunc {
	return func() (string, error) {
		if len(s.topics) == 0 {
			return "", iterator.Done
		}
		tn := s.topics[0]
		s.topics = s.topics[1:]
		return tn, s.err
	}
}

func (s *topicListService) listProjectTopics(ctx context.Context, projName string) nextStringFunc {
	if projName != "projects/projid" {
		s.t.Fatalf("unexpected call: projName: %q", projName)
		return nil
	}
	return s.newNextStringFunc()
}

func checkTopicListing(t *testing.T, want []string) {
	s := &topicListService{topics: want, t: t}
	c := &Client{projectID: "projid", s: s}
	topics, err := slurpTopics(c.Topics(context.Background()))
	if err != nil {
		t.Errorf("error listing topics: %v", err)
	}
	got := topicNames(topics)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("topic list: got: %v, want: %v", got, want)
	}
	if len(s.topics) != 0 {
		t.Errorf("outstanding topics: %v", s.topics)
	}
}

// All returns the remaining topics from this iterator.
func slurpTopics(it *TopicIterator) ([]*Topic, error) {
	var topics []*Topic
	for {
		switch topic, err := it.Next(); err {
		case nil:
			topics = append(topics, topic)
		case iterator.Done:
			return topics, nil
		default:
			return nil, err
		}
	}
}

func TestTopicID(t *testing.T) {
	const id = "id"
	serv := &topicListService{
		topics: []string{"projects/projid/topics/t1", "projects/projid/topics/t2"},
		t:      t,
	}
	c := &Client{projectID: "projid", s: serv}
	s := c.Topic(id)
	if got, want := s.ID(), id; got != want {
		t.Errorf("Token.ID() = %q; want %q", got, want)
	}
	want := []string{"t1", "t2"}
	topics, err := slurpTopics(c.Topics(context.Background()))
	if err != nil {
		t.Errorf("error listing topics: %v", err)
	}
	for i, topic := range topics {
		if got, want := topic.ID(), want[i]; got != want {
			t.Errorf("Token.ID() = %q; want %q", got, want)
		}
	}
}

func TestListTopics(t *testing.T) {
	checkTopicListing(t, []string{
		"projects/projid/topics/t1",
		"projects/projid/topics/t2",
		"projects/projid/topics/t3",
		"projects/projid/topics/t4"})
}

func TestListCompletelyEmptyTopics(t *testing.T) {
	var want []string
	checkTopicListing(t, want)
}

func TestStopPublishOrder(t *testing.T) {
	// Check that Stop doesn't panic if called before Publish.
	// Also that Publish after Stop returns the right error.
	ctx := context.Background()
	c := &Client{projectID: "projid"}
	topic := c.Topic("t")
	topic.Stop()
	r := topic.Publish(ctx, &Message{})
	_, err := r.Get(ctx)
	if err != errTopicStopped {
		t.Errorf("got %v, want errTopicStopped", err)
	}
}

func TestPublishTimeout(t *testing.T) {
	ctx := context.Background()
	serv := grpc.NewServer()
	pubsubpb.RegisterPublisherServer(serv, &alwaysFailPublish{})
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	go serv.Serve(lis)
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	s, err := newPubSubService(context.Background(), []option.ClientOption{option.WithGRPCConn(conn)})
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{s: s}
	topic := c.Topic("t")
	topic.PublishSettings.Timeout = 3 * time.Second
	r := topic.Publish(ctx, &Message{})
	defer topic.Stop()
	select {
	case <-r.Ready():
		_, err = r.Get(ctx)
		if err != context.DeadlineExceeded {
			t.Fatalf("got %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(2 * topic.PublishSettings.Timeout):
		t.Fatal("timed out")
	}
}

type alwaysFailPublish struct {
	pubsubpb.PublisherServer
}

func (s *alwaysFailPublish) Publish(ctx context.Context, req *pubsubpb.PublishRequest) (*pubsubpb.PublishResponse, error) {
	return nil, grpc.Errorf(codes.Unavailable, "try again")
}

func topicNames(topics []*Topic) []string {
	var names []string

	for _, topic := range topics {
		names = append(names, topic.name)

	}
	return names
}
