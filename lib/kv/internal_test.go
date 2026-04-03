//go:build !plan9 && !js

package kv

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKvConcurrency(t *testing.T) {
	require.Equal(t, 0, len(dbMap), "no databases can be started initially")

	const threadNum = 5
	var wg sync.WaitGroup
	ctx := context.Background()
	results := make([]*DB, threadNum)
	wg.Add(threadNum)
	for i := range threadNum {
		go func(i int) {
			db, err := Start(ctx, "test", nil)
			require.NoError(t, err)
			require.NotNil(t, db)
			results[i] = db
			wg.Done()
		}(i)
	}
	wg.Wait()

	// must have a single multi-referenced db
	db := results[0]
	assert.Equal(t, 1, len(dbMap))
	assert.Equal(t, threadNum, db.refs)
	for i := range threadNum {
		assert.Equal(t, db, results[i])
	}

	for i := range threadNum {
		assert.Equal(t, 1, len(dbMap))
		err := db.Stop(false)
		assert.NoError(t, err, "unexpected error %v at retry %d", err, i)
	}

	assert.Equal(t, 0, len(dbMap), "must be closed in the end")
	err := db.Stop(false)
	assert.ErrorIs(t, err, ErrInactive, "missing expected stop indication")
}

type testWriteOp struct {
	key string
	val string
}

func (op *testWriteOp) Do(_ context.Context, b Bucket) error {
	return b.Put([]byte(op.key), []byte(op.val))
}

type testReadOp struct {
	key string
	val string
}

func (op *testReadOp) Do(_ context.Context, b Bucket) error {
	op.val = string(b.Get([]byte(op.key)))
	return nil
}

func TestKvReadOnly(t *testing.T) {
	require.Equal(t, 0, len(dbMap), "no databases can be started initially")

	ctx := context.Background()
	db, err := Start(ctx, "test", nil)
	require.NoError(t, err)
	defer func() { _ = db.Stop(true) }()

	// Write succeeds when not read-only
	err = db.Do(true, &testWriteOp{key: "k1", val: "v1"})
	assert.NoError(t, err)

	db.SetReadOnly(true)

	// Write returns ErrReadOnly
	err = db.Do(true, &testWriteOp{key: "k2", val: "v2"})
	assert.ErrorIs(t, err, ErrReadOnly)

	// Read still works
	readOp := &testReadOp{key: "k1"}
	err = db.Do(false, readOp)
	assert.NoError(t, err)
	assert.Equal(t, "v1", readOp.val)

	// Unset read-only, write succeeds again
	db.SetReadOnly(false)
	err = db.Do(true, &testWriteOp{key: "k3", val: "v3"})
	assert.NoError(t, err)
}

func TestKvExit(t *testing.T) {
	require.Equal(t, 0, len(dbMap), "no databases can be started initially")
	const dbNum = 5
	ctx := context.Background()
	for i := range dbNum {
		facility := fmt.Sprintf("test-%d", i)
		for j := 0; j <= i; j++ {
			db, err := Start(ctx, facility, nil)
			require.NoError(t, err)
			require.NotNil(t, db)
		}
	}
	assert.Equal(t, dbNum, len(dbMap))
	Exit()
	assert.Equal(t, 0, len(dbMap))
}
