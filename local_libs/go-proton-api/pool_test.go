package proton

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ProtonMail/gluon/async"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPool_NewJob(t *testing.T) {
	doubler := newDoubler(runtime.NumCPU())
	defer doubler.Done()

	job1, done1, err := doubler.newJob(context.Background(), 1)
	require.NoError(t, err)
	defer done1()

	job2, done2, err := doubler.newJob(context.Background(), 2)
	require.NoError(t, err)
	defer done2()

	res2, err := job2.result()
	require.NoError(t, err)

	res1, err := job1.result()
	require.NoError(t, err)

	assert.Equal(t, 2, res1)
	assert.Equal(t, 4, res2)
}

func TestPool_NewJob_Done(t *testing.T) {
	// Create a doubler pool with 2 workers.
	doubler := newDoubler(2)
	defer doubler.Done()

	// Start two jobs. Don't mark the jobs as done yet.
	job1, done1, err := doubler.newJob(context.Background(), 1)
	require.NoError(t, err)
	job2, done2, err := doubler.newJob(context.Background(), 2)
	require.NoError(t, err)

	// Get the first result.
	res1, _ := job1.result()
	assert.Equal(t, 2, res1)

	// Get the first result.
	res2, _ := job2.result()
	assert.Equal(t, 4, res2)

	// Additional jobs will wait.
	job3, done3, err := doubler.newJob(context.Background(), 3)
	require.NoError(t, err)
	job4, done4, err := doubler.newJob(context.Background(), 4)
	require.NoError(t, err)

	// Channel to collect results from jobs 3 and 4.
	resCh := make(chan int, 2)

	go func() {
		res, _ := job3.result()
		resCh <- res
	}()

	go func() {
		res, _ := job4.result()
		resCh <- res
	}()

	// Mark jobs 1 and 2 as done, freeing up the workers.
	done1()
	done2()

	assert.ElementsMatch(t, []int{6, 8}, []int{<-resCh, <-resCh})

	// Mark jobs 3 and 4 as done, freeing up the workers.
	done3()
	done4()
}

func TestPool_Process(t *testing.T) {
	doubler := newDoubler(runtime.NumCPU())
	defer doubler.Done()

	res := make([]int, 5)

	require.NoError(t, doubler.Process(context.Background(), []int{1, 2, 3, 4, 5}, func(index, reqVal, resVal int, err error) error {
		require.NoError(t, err)

		res[index] = resVal

		return nil
	}))

	assert.Equal(t, []int{
		2,
		4,
		6,
		8,
		10,
	}, res)
}

func TestPool_Process_Error(t *testing.T) {
	doubler := newDoublerWithError(runtime.NumCPU())
	defer doubler.Done()

	assert.Error(t, doubler.Process(context.Background(), []int{1, 2, 3, 4, 5}, func(_int, _ int, _ int, err error) error {
		return err
	}))
}

func TestPool_Process_Parallel(t *testing.T) {
	doubler := newDoubler(runtime.NumCPU(), 100*time.Millisecond)
	defer doubler.Done()

	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			require.NoError(t, doubler.Process(context.Background(), []int{1, 2, 3, 4}, func(_ int, _ int, _ int, err error) error {
				return nil
			}))
		}()
	}

	wg.Wait()
}

func TestPool_ProcessAll(t *testing.T) {
	doubler := newDoubler(runtime.NumCPU())
	defer doubler.Done()

	res, err := doubler.ProcessAll(context.Background(), []int{1, 2, 3, 4, 5})
	require.NoError(t, err)

	assert.Equal(t, []int{
		2,
		4,
		6,
		8,
		10,
	}, res)
}

func newDoubler(workers int, delay ...time.Duration) *Pool[int, int] {
	return NewPool(workers, async.NoopPanicHandler{}, func(ctx context.Context, req int) (int, error) {
		if len(delay) > 0 {
			time.Sleep(delay[0])
		}

		return 2 * req, nil
	})
}

func newDoublerWithError(workers int) *Pool[int, int] {
	return NewPool(workers, async.NoopPanicHandler{}, func(ctx context.Context, req int) (int, error) {
		if req%2 == 0 {
			return 0, errors.New("oops")
		}

		return 2 * req, nil
	})
}
