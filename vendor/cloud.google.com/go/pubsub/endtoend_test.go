// Copyright 2014 Google Inc. All Rights Reserved.
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
	"bytes"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"

	"cloud.google.com/go/internal/testutil"
	"google.golang.org/api/option"
)

const (
	timeout                 = time.Minute * 10
	ackDeadline             = time.Second * 10
	nMessages               = 1e4
	acceptableDupPercentage = .05
	numAcceptableDups       = int(nMessages * acceptableDupPercentage / 100)
)

// Buffer log messages to debug failures.
var logBuf bytes.Buffer

// TestEndToEnd pumps many messages into a topic and tests that they are all
// delivered to each subscription for the topic. It also tests that messages
// are not unexpectedly redelivered.
func TestEndToEnd(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Integration tests skipped in short mode")
	}
	log.SetOutput(&logBuf)
	ctx := context.Background()
	ts := testutil.TokenSource(ctx, ScopePubSub, ScopeCloudPlatform)
	if ts == nil {
		t.Skip("Integration tests skipped. See CONTRIBUTING.md for details")
	}

	now := time.Now()
	topicName := fmt.Sprintf("endtoend-%d", now.UnixNano())
	subPrefix := fmt.Sprintf("endtoend-%d", now.UnixNano())

	client, err := NewClient(ctx, testutil.ProjID(), option.WithTokenSource(ts))
	if err != nil {
		t.Fatalf("Creating client error: %v", err)
	}

	var topic *Topic
	if topic, err = client.CreateTopic(ctx, topicName); err != nil {
		t.Fatalf("CreateTopic error: %v", err)
	}
	defer topic.Delete(ctx)

	// Two subscriptions to the same topic.
	var subs [2]*Subscription
	for i := 0; i < len(subs); i++ {
		subs[i], err = client.CreateSubscription(ctx, fmt.Sprintf("%s-%d", subPrefix, i), SubscriptionConfig{
			Topic:       topic,
			AckDeadline: ackDeadline,
		})
		if err != nil {
			t.Fatalf("CreateSub error: %v", err)
		}
		defer subs[i].Delete(ctx)
	}

	err = publish(ctx, topic, nMessages)
	topic.Stop()
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	// recv provides an indication that messages are still arriving.
	recv := make(chan struct{})
	// We have two subscriptions to our topic.
	// Each subscription will get a copy of each published message.
	var wg sync.WaitGroup
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	consumers := []*consumer{
		{counts: make(map[string]int), recv: recv, durations: []time.Duration{time.Hour}},
		{counts: make(map[string]int), recv: recv,
			durations: []time.Duration{ackDeadline, ackDeadline, ackDeadline / 2, ackDeadline / 2, time.Hour}},
	}
	for i, con := range consumers {
		con := con
		sub := subs[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			con.consume(t, cctx, sub)
		}()
	}
	// Wait for a while after the last message before declaring quiescence.
	// We wait a multiple of the ack deadline, for two reasons:
	// 1. To detect if messages are redelivered after having their ack
	//    deadline extended.
	// 2. To wait for redelivery of messages that were en route when a Receive
	//    is canceled. This can take considerably longer than the ack deadline.
	quiescenceDur := ackDeadline * 6
	quiescenceTimer := time.NewTimer(quiescenceDur)

loop:
	for {
		select {
		case <-recv:
			// Reset timer so we wait quiescenceDur after the last message.
			// See https://godoc.org/time#Timer.Reset for why the Stop
			// and channel drain are necessary.
			if !quiescenceTimer.Stop() {
				<-quiescenceTimer.C
			}
			quiescenceTimer.Reset(quiescenceDur)

		case <-quiescenceTimer.C:
			cancel()
			log.Println("quiesced")
			break loop

		case <-cctx.Done():
			t.Fatal("timed out")
		}
	}
	wg.Wait()
	ok := true
	for i, con := range consumers {
		var numDups int
		var zeroes int
		for _, v := range con.counts {
			if v == 0 {
				zeroes += 1
			}
			numDups += v - 1
		}

		if zeroes > 0 {
			t.Errorf("Consumer %d: %d messages never arrived", i, zeroes)
			ok = false
		} else if numDups > numAcceptableDups {
			t.Errorf("Consumer %d: Willing to accept %d dups (%f%% duplicated of %d messages), but got %d", i, numAcceptableDups, acceptableDupPercentage, int(nMessages), numDups)
			ok = false
		}
	}
	if !ok {
		logBuf.WriteTo(os.Stdout)
	}
}

// publish publishes n messages to topic, and returns the published message IDs.
func publish(ctx context.Context, topic *Topic, n int) error {
	var rs []*PublishResult
	for i := 0; i < n; i++ {
		m := &Message{Data: []byte(fmt.Sprintf("msg %d", i))}
		rs = append(rs, topic.Publish(ctx, m))
	}
	var ids []string
	for _, r := range rs {
		id, err := r.Get(ctx)
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}
	return nil
}

// consumer consumes messages according to its configuration.
type consumer struct {
	durations []time.Duration

	// A value is sent to recv each time Inc is called.
	recv chan struct{}

	mu     sync.Mutex
	counts map[string]int
	total  int
}

// consume reads messages from a subscription, and keeps track of what it receives in mc.
// After consume returns, the caller should wait on wg to ensure that no more updates to mc will be made.
func (c *consumer) consume(t *testing.T, ctx context.Context, sub *Subscription) {
	for _, dur := range c.durations {
		ctx2, cancel := context.WithTimeout(ctx, dur)
		defer cancel()
		id := sub.name[len(sub.name)-2:]
		log.Printf("%s: start receive", id)
		prev := c.total
		err := sub.Receive(ctx2, c.process)
		log.Printf("%s: end receive; read %d", id, c.total-prev)
		if err != nil {
			t.Errorf("error from Receive: %v", err)
			return
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

// process handles a message and records it in mc.
func (c *consumer) process(_ context.Context, m *Message) {
	c.mu.Lock()
	c.counts[m.ID] += 1
	c.total++
	c.mu.Unlock()
	c.recv <- struct{}{}
	// Simulate time taken to process m, while continuing to process more messages.
	// Some messages will need to have their ack deadline extended due to this delay.
	delay := rand.Intn(int(ackDeadline * 3))
	time.AfterFunc(time.Duration(delay), m.Ack)
}
