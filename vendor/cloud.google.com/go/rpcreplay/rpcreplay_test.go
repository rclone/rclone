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

package rpcreplay

import (
	"bytes"
	"io"
	"testing"

	"cloud.google.com/go/internal/testutil"
	ipb "cloud.google.com/go/rpcreplay/proto/intstore"
	rpb "cloud.google.com/go/rpcreplay/proto/rpcreplay"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRecordIO(t *testing.T) {
	buf := &bytes.Buffer{}
	want := []byte{1, 2, 3}
	if err := writeRecord(buf, want); err != nil {
		t.Fatal(err)
	}
	got, err := readRecord(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestHeaderIO(t *testing.T) {
	buf := &bytes.Buffer{}
	want := []byte{1, 2, 3}
	if err := writeHeader(buf, want); err != nil {
		t.Fatal(err)
	}
	got, err := readHeader(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !testutil.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// readHeader errors
	for _, contents := range []string{"", "badmagic", "gRPCReplay"} {
		if _, err := readHeader(bytes.NewBufferString(contents)); err == nil {
			t.Errorf("%q: got nil, want error", contents)
		}
	}
}

func TestEntryIO(t *testing.T) {
	for i, want := range []*entry{
		{
			kind:     rpb.Entry_REQUEST,
			method:   "method",
			msg:      message{msg: &rpb.Entry{}},
			refIndex: 7,
		},
		{
			kind:     rpb.Entry_RESPONSE,
			method:   "method",
			msg:      message{err: status.Error(codes.NotFound, "not found")},
			refIndex: 8,
		},
		{
			kind:     rpb.Entry_RECV,
			method:   "method",
			msg:      message{err: io.EOF},
			refIndex: 3,
		},
	} {
		buf := &bytes.Buffer{}
		if err := writeEntry(buf, want); err != nil {
			t.Fatal(err)
		}
		got, err := readEntry(buf)
		if err != nil {
			t.Fatal(err)
		}
		if !got.equal(want) {
			t.Errorf("#%d: got %v, want %v", i, got, want)
		}
	}
}

var initialState = []byte{1, 2, 3}

func TestRecord(t *testing.T) {
	srv := newIntStoreServer()
	defer srv.stop()
	buf := record(t, srv)

	gotIstate, err := readHeader(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !testutil.Equal(gotIstate, initialState) {
		t.Fatalf("got %v, want %v", gotIstate, initialState)
	}
	item := &ipb.Item{Name: "a", Value: 1}
	wantEntries := []*entry{
		// Set
		{
			kind:   rpb.Entry_REQUEST,
			method: "/intstore.IntStore/Set",
			msg:    message{msg: item},
		},
		{
			kind:     rpb.Entry_RESPONSE,
			msg:      message{msg: &ipb.SetResponse{PrevValue: 0}},
			refIndex: 1,
		},
		// Get
		{
			kind:   rpb.Entry_REQUEST,
			method: "/intstore.IntStore/Get",
			msg:    message{msg: &ipb.GetRequest{Name: "a"}},
		},
		{
			kind:     rpb.Entry_RESPONSE,
			msg:      message{msg: item},
			refIndex: 3,
		},
		{
			kind:   rpb.Entry_REQUEST,
			method: "/intstore.IntStore/Get",
			msg:    message{msg: &ipb.GetRequest{Name: "x"}},
		},
		{
			kind:     rpb.Entry_RESPONSE,
			msg:      message{err: status.Error(codes.NotFound, `"x"`)},
			refIndex: 5,
		},
		// ListItems
		{ // entry #7
			kind:   rpb.Entry_CREATE_STREAM,
			method: "/intstore.IntStore/ListItems",
		},
		{
			kind:     rpb.Entry_SEND,
			msg:      message{msg: &ipb.ListItemsRequest{}},
			refIndex: 7,
		},
		{
			kind:     rpb.Entry_RECV,
			msg:      message{msg: item},
			refIndex: 7,
		},
		{
			kind:     rpb.Entry_RECV,
			msg:      message{err: io.EOF},
			refIndex: 7,
		},
		// SetStream
		{ // entry #11
			kind:   rpb.Entry_CREATE_STREAM,
			method: "/intstore.IntStore/SetStream",
		},
		{
			kind:     rpb.Entry_SEND,
			msg:      message{msg: &ipb.Item{Name: "b", Value: 2}},
			refIndex: 11,
		},
		{
			kind:     rpb.Entry_SEND,
			msg:      message{msg: &ipb.Item{Name: "c", Value: 3}},
			refIndex: 11,
		},
		{
			kind:     rpb.Entry_RECV,
			msg:      message{msg: &ipb.Summary{Count: 2}},
			refIndex: 11,
		},

		// StreamChat
		{ // entry #15
			kind:   rpb.Entry_CREATE_STREAM,
			method: "/intstore.IntStore/StreamChat",
		},
		{
			kind:     rpb.Entry_SEND,
			msg:      message{msg: &ipb.Item{Name: "d", Value: 4}},
			refIndex: 15,
		},
		{
			kind:     rpb.Entry_RECV,
			msg:      message{msg: &ipb.Item{Name: "d", Value: 4}},
			refIndex: 15,
		},
		{
			kind:     rpb.Entry_SEND,
			msg:      message{msg: &ipb.Item{Name: "e", Value: 5}},
			refIndex: 15,
		},
		{
			kind:     rpb.Entry_RECV,
			msg:      message{msg: &ipb.Item{Name: "e", Value: 5}},
			refIndex: 15,
		},
		{
			kind:     rpb.Entry_RECV,
			msg:      message{err: io.EOF},
			refIndex: 15,
		},
	}
	for i, w := range wantEntries {
		g, err := readEntry(buf)
		if err != nil {
			t.Fatalf("#%d: %v", i+1, err)
		}
		if !g.equal(w) {
			t.Errorf("#%d:\ngot  %+v\nwant %+v", i+1, g, w)
		}
	}
	g, err := readEntry(buf)
	if err != nil {
		t.Fatal(err)
	}
	if g != nil {
		t.Errorf("\ngot  %+v\nwant nil", g)
	}
}

func TestReplay(t *testing.T) {
	srv := newIntStoreServer()
	defer srv.stop()

	buf := record(t, srv)
	rep, err := NewReplayerReader(buf)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := rep.Initial(), initialState; !testutil.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// Replay the test.
	testService(t, srv.Addr, rep.DialOptions())
}

func record(t *testing.T, srv *intStoreServer) *bytes.Buffer {
	buf := &bytes.Buffer{}
	rec, err := NewRecorderWriter(buf, initialState)
	if err != nil {
		t.Fatal(err)
	}
	testService(t, srv.Addr, rec.DialOptions())
	if err := rec.Close(); err != nil {
		t.Fatal(err)
	}
	return buf
}

func testService(t *testing.T, addr string, opts []grpc.DialOption) {
	conn, err := grpc.Dial(addr,
		append([]grpc.DialOption{grpc.WithInsecure()}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := ipb.NewIntStoreClient(conn)
	ctx := context.Background()
	item := &ipb.Item{Name: "a", Value: 1}
	res, err := client.Set(ctx, item)
	if err != nil {
		t.Fatal(err)
	}
	if res.PrevValue != 0 {
		t.Errorf("got %d, want 0", res.PrevValue)
	}
	got, err := client.Get(ctx, &ipb.GetRequest{Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(got, item) {
		t.Errorf("got %v, want %v", got, item)
	}
	_, err = client.Get(ctx, &ipb.GetRequest{Name: "x"})
	if err == nil {
		t.Fatal("got nil, want error")
	}
	if _, ok := status.FromError(err); !ok {
		t.Errorf("got error type %T, want a grpc/status.Status", err)
	}

	wantItems := []*ipb.Item{item}
	lic, err := client.ListItems(ctx, &ipb.ListItemsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; ; i++ {
		item, err := lic.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if i >= len(wantItems) || !proto.Equal(item, wantItems[i]) {
			t.Fatalf("%d: bad item", i)
		}
	}

	ssc, err := client.SetStream(ctx)
	if err != nil {
		t.Fatal(err)
	}

	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}

	for i, name := range []string{"b", "c"} {
		must(ssc.Send(&ipb.Item{Name: name, Value: int32(i + 2)}))
	}
	summary, err := ssc.CloseAndRecv()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := summary.Count, int32(2); got != want {
		t.Fatalf("got %d, want %d", got, want)
	}

	chatc, err := client.StreamChat(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"d", "e"} {
		item := &ipb.Item{Name: name, Value: int32(i + 4)}
		must(chatc.Send(item))
		got, err := chatc.Recv()
		if err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(got, item) {
			t.Errorf("got %v, want %v", got, item)
		}
	}
	must(chatc.CloseSend())
	if _, err := chatc.Recv(); err != io.EOF {
		t.Fatalf("got %v, want EOF", err)
	}
}
