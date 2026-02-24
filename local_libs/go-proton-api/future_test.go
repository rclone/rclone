package proton

import (
	"math/rand"
	"testing"
	"time"

	"github.com/ProtonMail/gluon/async"
	"github.com/stretchr/testify/require"
)

func TestFuture(t *testing.T) {
	resCh := make(chan int)

	NewFuture(async.NoopPanicHandler{}, func() (int, error) {
		return 42, nil
	}).Then(func(res int, err error) {
		resCh <- res
	})

	require.Equal(t, 42, <-resCh)
}

func TestGroup(t *testing.T) {
	group := NewGroup[int](async.NoopPanicHandler{})

	for i := 0; i < 10; i++ {
		i := i

		group.Add(func() (int, error) {
			// Sleep a random amount of time so that results are returned in a random order.
			time.Sleep(time.Duration(rand.Int()%10) * time.Millisecond) //nolint:gosec

			// Return the job index [0, 10].
			return i, nil
		})
	}

	resCh := make(chan int)

	go func() {
		require.Equal(t, group.ForEach(func(res int) error { resCh <- res; return nil }), nil)
	}()

	// Results should be returned in the original order.
	for i := 0; i < 10; i++ {
		require.Equal(t, i, <-resCh)
	}
}
