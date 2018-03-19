/*
Copyright 2017 Google Inc. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package spanner

import (
	"errors"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/spanner/internal/testutil"

	"golang.org/x/net/context"
	sppb "google.golang.org/genproto/googleapis/spanner/v1"
	"google.golang.org/grpc/codes"
)

var (
	errAbrt = spannerErrorf(codes.Aborted, "")
	errUsr  = errors.New("error")
)

// setup sets up a Client using mockclient
func mockClient(t *testing.T) (*sessionPool, *testutil.MockCloudSpannerClient, *Client) {
	var (
		mc       = testutil.NewMockCloudSpannerClient(t)
		spc      = SessionPoolConfig{}
		database = "mockdb"
	)
	spc.getRPCClient = func() (sppb.SpannerClient, error) {
		return mc, nil
	}
	sp, err := newSessionPool(database, spc, nil)
	if err != nil {
		t.Fatalf("cannot create session pool: %v", err)
	}
	return sp, mc, &Client{
		database:     database,
		idleSessions: sp,
	}
}

// TestReadOnlyAcquire tests acquire for ReadOnlyTransaction.
func TestReadOnlyAcquire(t *testing.T) {
	t.Parallel()
	_, mc, client := mockClient(t)
	defer client.Close()
	mc.SetActions(
		testutil.Action{"BeginTransaction", errUsr},
		testutil.Action{"BeginTransaction", nil},
		testutil.Action{"BeginTransaction", nil},
	)

	// Singleuse should only be used once.
	txn := client.Single()
	defer txn.Close()
	_, _, e := txn.acquire(context.Background())
	if e != nil {
		t.Errorf("Acquire for single use, got %v, want nil.", e)
	}
	_, _, e = txn.acquire(context.Background())
	if wantErr := errTxClosed(); !testEqual(e, wantErr) {
		t.Errorf("Second acquire for single use, got %v, want %v.", e, wantErr)
	}
	// Multiuse can recover from acquire failure.
	txn = client.ReadOnlyTransaction()
	_, _, e = txn.acquire(context.Background())
	if wantErr := toSpannerError(errUsr); !testEqual(e, wantErr) {
		t.Errorf("Acquire for multi use, got %v, want %v.", e, wantErr)
	}
	_, _, e = txn.acquire(context.Background())
	if e != nil {
		t.Errorf("Acquire for multi use, got %v, want nil.", e)
	}
	txn.Close()
	// Multiuse can not be used after close.
	_, _, e = txn.acquire(context.Background())
	if wantErr := errTxClosed(); !testEqual(e, wantErr) {
		t.Errorf("Second acquire for multi use, got %v, want %v.", e, wantErr)
	}
	// Multiuse can be acquired concurrently.
	txn = client.ReadOnlyTransaction()
	defer txn.Close()
	mc.Freeze()
	var (
		sh1 *sessionHandle
		sh2 *sessionHandle
		ts1 *sppb.TransactionSelector
		ts2 *sppb.TransactionSelector
		wg  = sync.WaitGroup{}
	)
	acquire := func(sh **sessionHandle, ts **sppb.TransactionSelector) {
		defer wg.Done()
		var e error
		*sh, *ts, e = txn.acquire(context.Background())
		if e != nil {
			t.Errorf("Concurrent acquire for multiuse, got %v, expect nil.", e)
		}
	}
	wg.Add(2)
	go acquire(&sh1, &ts1)
	go acquire(&sh2, &ts2)
	<-time.After(100 * time.Millisecond)
	mc.Unfreeze()
	wg.Wait()
	if !testEqual(sh1.session, sh2.session) {
		t.Errorf("Expect acquire to get same session handle, got %v and %v.", sh1, sh2)
	}
	if !testEqual(ts1, ts2) {
		t.Errorf("Expect acquire to get same transaction selector, got %v and %v.", ts1, ts2)
	}
}

// TestRetryOnAbort tests transaction retries on abort.
func TestRetryOnAbort(t *testing.T) {
	t.Parallel()
	_, mc, client := mockClient(t)
	defer client.Close()
	// commit in writeOnlyTransaction
	mc.SetActions(
		testutil.Action{"Commit", errAbrt}, // abort on first commit
		testutil.Action{"Commit", nil},
	)

	ms := []*Mutation{
		Insert("Accounts", []string{"AccountId", "Nickname", "Balance"}, []interface{}{int64(1), "Foo", int64(50)}),
		Insert("Accounts", []string{"AccountId", "Nickname", "Balance"}, []interface{}{int64(2), "Bar", int64(1)}),
	}
	if _, e := client.Apply(context.Background(), ms, ApplyAtLeastOnce()); e != nil {
		t.Errorf("applyAtLeastOnce retry on abort, got %v, want nil.", e)
	}
	// begin and commit in ReadWriteTransaction
	mc.SetActions(
		testutil.Action{"BeginTransaction", nil},     // let takeWriteSession succeed and get a session handle
		testutil.Action{"Commit", errAbrt},           // let first commit fail and retry will begin new transaction
		testutil.Action{"BeginTransaction", errAbrt}, // this time we can fail the begin attempt
		testutil.Action{"BeginTransaction", nil},
		testutil.Action{"Commit", nil},
	)

	if _, e := client.Apply(context.Background(), ms); e != nil {
		t.Errorf("ReadWriteTransaction retry on abort, got %v, want nil.", e)
	}
}

// TestBadSession tests bad session (session not found error).
// TODO: session closed from transaction close
func TestBadSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sp, mc, client := mockClient(t)
	defer client.Close()
	var sid string
	// Prepare a session, get the session id for use in testing.
	if s, e := sp.take(ctx); e != nil {
		t.Fatal("Prepare session failed.")
	} else {
		sid = s.getID()
		s.recycle()
	}

	wantErr := spannerErrorf(codes.NotFound, "Session not found: %v", sid)
	// ReadOnlyTransaction
	mc.SetActions(
		testutil.Action{"BeginTransaction", wantErr},
		testutil.Action{"BeginTransaction", wantErr},
		testutil.Action{"BeginTransaction", wantErr},
	)
	txn := client.ReadOnlyTransaction()
	defer txn.Close()
	if _, _, got := txn.acquire(ctx); !testEqual(wantErr, got) {
		t.Errorf("Expect acquire to fail, got %v, want %v.", got, wantErr)
	}
	// The failure should recycle the session, we expect it to be used in following requests.
	if got := txn.Query(ctx, NewStatement("SELECT 1")); !testEqual(wantErr, got.err) {
		t.Errorf("Expect Query to fail, got %v, want %v.", got.err, wantErr)
	}
	if got := txn.Read(ctx, "Users", KeySets(Key{"alice"}, Key{"bob"}), []string{"name", "email"}); !testEqual(wantErr, got.err) {
		t.Errorf("Expect Read to fail, got %v, want %v.", got.err, wantErr)
	}
	// writeOnlyTransaction
	ms := []*Mutation{
		Insert("Accounts", []string{"AccountId", "Nickname", "Balance"}, []interface{}{int64(1), "Foo", int64(50)}),
		Insert("Accounts", []string{"AccountId", "Nickname", "Balance"}, []interface{}{int64(2), "Bar", int64(1)}),
	}
	mc.SetActions(testutil.Action{"Commit", wantErr})
	if _, got := client.Apply(context.Background(), ms, ApplyAtLeastOnce()); !testEqual(wantErr, got) {
		t.Errorf("Expect applyAtLeastOnce to fail, got %v, want %v.", got, wantErr)
	}
}

func TestFunctionErrorReturned(t *testing.T) {
	t.Parallel()
	_, mc, client := mockClient(t)
	defer client.Close()
	mc.SetActions(
		testutil.Action{"BeginTransaction", nil},
		testutil.Action{"Rollback", nil},
	)

	want := errors.New("an error")
	_, got := client.ReadWriteTransaction(context.Background(),
		func(context.Context, *ReadWriteTransaction) error { return want })
	if got != want {
		t.Errorf("got <%v>, want <%v>", got, want)
	}
	mc.CheckActionsConsumed()
}
