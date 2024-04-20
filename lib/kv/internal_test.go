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
	for i := 0; i < threadNum; i++ {
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
	for i := 0; i < threadNum; i++ {
		assert.Equal(t, db, results[i])
	}

	for i := 0; i < threadNum; i++ {
		assert.Equal(t, 1, len(dbMap))
		err := db.Stop(false)
		assert.NoError(t, err, "unexpected error %v at retry %d", err, i)
	}

	assert.Equal(t, 0, len(dbMap), "must be closed in the end")
	err := db.Stop(false)
	assert.ErrorIs(t, err, ErrInactive, "missing expected stop indication")
}

func TestKvExit(t *testing.T) {
	require.Equal(t, 0, len(dbMap), "no databases can be started initially")
	const dbNum = 5
	ctx := context.Background()
	for i := 0; i < dbNum; i++ {
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
