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

// TODO(jba): test keepalive
// TODO(jba): test that expired messages are not kept alive
// TODO(jba): test that when all messages expire, Stop returns.

import (
	"io"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/internal/testutil"
	"google.golang.org/grpc/status"

	tspb "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
	pb "google.golang.org/genproto/googleapis/pubsub/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

var (
	timestamp    = &tspb.Timestamp{}
	testMessages = []*pb.ReceivedMessage{
		{AckId: "0", Message: &pb.PubsubMessage{Data: []byte{1}, PublishTime: timestamp}},
		{AckId: "1", Message: &pb.PubsubMessage{Data: []byte{2}, PublishTime: timestamp}},
		{AckId: "2", Message: &pb.PubsubMessage{Data: []byte{3}, PublishTime: timestamp}},
	}
)

func TestStreamingPullBasic(t *testing.T) {
	client, server := newFake(t)
	server.addStreamingPullMessages(testMessages)
	testStreamingPullIteration(t, client, server, testMessages)
}

func TestStreamingPullMultipleFetches(t *testing.T) {
	client, server := newFake(t)
	server.addStreamingPullMessages(testMessages[:1])
	server.addStreamingPullMessages(testMessages[1:])
	testStreamingPullIteration(t, client, server, testMessages)
}

func newTestSubscription(t *testing.T, client *Client, name string) *Subscription {
	topic := client.Topic("t")
	sub, err := client.CreateSubscription(context.Background(), name,
		SubscriptionConfig{Topic: topic})
	if err != nil {
		t.Fatalf("CreateSubscription: %v", err)
	}
	return sub
}

func testStreamingPullIteration(t *testing.T, client *Client, server *fakeServer, msgs []*pb.ReceivedMessage) {
	sub := newTestSubscription(t, client, "s")
	gotMsgs, err := pullN(context.Background(), sub, len(msgs), func(_ context.Context, m *Message) {
		id, err := strconv.Atoi(m.ackID)
		if err != nil {
			panic(err)
		}
		// ack evens, nack odds
		if id%2 == 0 {
			m.Ack()
		} else {
			m.Nack()
		}
	})
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	gotMap := map[string]*Message{}
	for _, m := range gotMsgs {
		gotMap[m.ackID] = m
	}
	for i, msg := range msgs {
		want, err := toMessage(msg)
		if err != nil {
			t.Fatal(err)
		}
		want.calledDone = true
		got := gotMap[want.ackID]
		if got == nil {
			t.Errorf("%d: no message for ackID %q", i, want.ackID)
			continue
		}
		if !testutil.Equal(got, want, cmp.AllowUnexported(Message{}), cmpopts.IgnoreTypes(time.Time{}, func(string, bool, time.Time) {})) {
			t.Errorf("%d: got\n%#v\nwant\n%#v", i, got, want)
		}
	}
	server.wait()
	for i := 0; i < len(msgs); i++ {
		id := msgs[i].AckId
		if i%2 == 0 {
			if !server.Acked[id] {
				t.Errorf("msg %q should have been acked but wasn't", id)
			}
		} else {
			if dl, ok := server.Deadlines[id]; !ok || dl != 0 {
				t.Errorf("msg %q should have been nacked but wasn't", id)
			}
		}
	}
}

func TestStreamingPullError(t *testing.T) {
	// If an RPC to the service returns a non-retryable error, Pull should
	// return after all callbacks return, without waiting for messages to be
	// acked.
	client, server := newFake(t)
	server.addStreamingPullMessages(testMessages[:1])
	server.addStreamingPullError(status.Errorf(codes.Unknown, ""))
	sub := newTestSubscription(t, client, "s")
	// Use only one goroutine, since the fake server is configured to
	// return only one error.
	sub.ReceiveSettings.NumGoroutines = 1
	callbackDone := make(chan struct{})
	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	err := sub.Receive(ctx, func(ctx context.Context, m *Message) {
		defer close(callbackDone)
		select {
		case <-ctx.Done():
			return
		}
	})
	select {
	case <-callbackDone:
	default:
		t.Fatal("Receive returned but callback was not done")
	}
	if want := codes.Unknown; grpc.Code(err) != want {
		t.Fatalf("got <%v>, want code %v", err, want)
	}
}

func TestStreamingPullCancel(t *testing.T) {
	// If Receive's context is canceled, it should return after all callbacks
	// return and all messages have been acked.
	client, server := newFake(t)
	server.addStreamingPullMessages(testMessages)
	sub := newTestSubscription(t, client, "s")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	var n int32
	err := sub.Receive(ctx, func(ctx2 context.Context, m *Message) {
		atomic.AddInt32(&n, 1)
		defer atomic.AddInt32(&n, -1)
		cancel()
		m.Ack()
	})
	if got := atomic.LoadInt32(&n); got != 0 {
		t.Errorf("Receive returned with %d callbacks still running", got)
	}
	if err != nil {
		t.Fatalf("Receive got <%v>, want nil", err)
	}
}

func TestStreamingPullRetry(t *testing.T) {
	// Check that we retry on io.EOF or Unavailable.
	client, server := newFake(t)
	server.addStreamingPullMessages(testMessages[:1])
	server.addStreamingPullError(io.EOF)
	server.addStreamingPullError(io.EOF)
	server.addStreamingPullMessages(testMessages[1:2])
	server.addStreamingPullError(status.Errorf(codes.Unavailable, ""))
	server.addStreamingPullError(status.Errorf(codes.Unavailable, ""))
	server.addStreamingPullMessages(testMessages[2:])

	testStreamingPullIteration(t, client, server, testMessages)
}

func TestStreamingPullOneActive(t *testing.T) {
	// Only one call to Pull can be active at a time.
	client, srv := newFake(t)
	srv.addStreamingPullMessages(testMessages[:1])
	sub := newTestSubscription(t, client, "s")
	ctx, cancel := context.WithCancel(context.Background())
	err := sub.Receive(ctx, func(ctx context.Context, m *Message) {
		m.Ack()
		err := sub.Receive(ctx, func(context.Context, *Message) {})
		if err != errReceiveInProgress {
			t.Errorf("got <%v>, want <%v>", err, errReceiveInProgress)
		}
		cancel()
	})
	if err != nil {
		t.Fatalf("got <%v>, want nil", err)
	}
}

func TestStreamingPullConcurrent(t *testing.T) {
	newMsg := func(i int) *pb.ReceivedMessage {
		return &pb.ReceivedMessage{
			AckId:   strconv.Itoa(i),
			Message: &pb.PubsubMessage{Data: []byte{byte(i)}, PublishTime: timestamp},
		}
	}

	// Multiple goroutines should be able to read from the same iterator.
	client, server := newFake(t)
	// Add a lot of messages, a few at a time, to make sure both threads get a chance.
	nMessages := 100
	for i := 0; i < nMessages; i += 2 {
		server.addStreamingPullMessages([]*pb.ReceivedMessage{newMsg(i), newMsg(i + 1)})
	}
	sub := newTestSubscription(t, client, "s")
	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	gotMsgs, err := pullN(ctx, sub, nMessages, func(ctx context.Context, m *Message) {
		m.Ack()
	})
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	seen := map[string]bool{}
	for _, gm := range gotMsgs {
		if seen[gm.ackID] {
			t.Fatalf("duplicate ID %q", gm.ackID)
		}
		seen[gm.ackID] = true
	}
	if len(seen) != nMessages {
		t.Fatalf("got %d messages, want %d", len(seen), nMessages)
	}
}

func TestStreamingPullFlowControl(t *testing.T) {
	// Callback invocations should not occur if flow control limits are exceeded.
	client, server := newFake(t)
	server.addStreamingPullMessages(testMessages)
	sub := newTestSubscription(t, client, "s")
	sub.ReceiveSettings.MaxOutstandingMessages = 2
	ctx, cancel := context.WithCancel(context.Background())
	activec := make(chan int)
	waitc := make(chan int)
	errc := make(chan error)
	go func() {
		errc <- sub.Receive(ctx, func(_ context.Context, m *Message) {
			activec <- 1
			<-waitc
			m.Ack()
		})
	}()
	// Here, two callbacks are active. Receive should be blocked in the flow
	// control acquire method on the third message.
	<-activec
	<-activec
	select {
	case <-activec:
		t.Fatal("third callback in progress")
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	// Receive still has not returned, because both callbacks are still blocked on waitc.
	select {
	case err := <-errc:
		t.Fatalf("Receive returned early with error %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	// Let both callbacks proceed.
	waitc <- 1
	waitc <- 1
	// The third callback will never run, because acquire returned a non-nil
	// error, causing Receive to return. So now Receive should end.
	if err := <-errc; err != nil {
		t.Fatalf("got %v from Receive, want nil", err)
	}
}

func newFake(t *testing.T) (*Client, *fakeServer) {
	srv, err := newFakeServer()
	if err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.Dial(srv.Addr, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(context.Background(), "projectID", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatal(err)
	}
	return client, srv
}

// pullN calls sub.Receive until at least n messages are received.
func pullN(ctx context.Context, sub *Subscription, n int, f func(context.Context, *Message)) ([]*Message, error) {
	var (
		mu   sync.Mutex
		msgs []*Message
	)
	cctx, cancel := context.WithCancel(ctx)
	err := sub.Receive(cctx, func(ctx context.Context, m *Message) {
		mu.Lock()
		msgs = append(msgs, m)
		nSeen := len(msgs)
		mu.Unlock()
		f(ctx, m)
		if nSeen >= n {
			cancel()
		}
	})
	if err != nil {
		return nil, err
	}
	return msgs, nil
}
