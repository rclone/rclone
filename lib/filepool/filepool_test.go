package filepool

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// handle is a fake pooled handle used by the tests.
type handle struct {
	id       int
	released bool
	relErr   error
}

// harness wires a Pool up to counters so the tests can assert on the open and
// release calls without a real backend.
type harness struct {
	mu       sync.Mutex
	next     int
	opens    int
	openErr  error
	releases int
	closeErr error
}

func (h *harness) open(context.Context) (*handle, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.opens++
	if h.openErr != nil {
		return nil, h.openErr
	}
	h.next++
	return &handle{id: h.next}, nil
}

func (h *harness) release(hd *handle, err error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.releases++
	hd.released = true
	hd.relErr = err
	return h.closeErr
}

func newPool(h *harness) *Pool[*handle] {
	return New(context.Background(), h.open, h.release)
}

func TestGetOpensWhenEmpty(t *testing.T) {
	h := &harness{}
	p := newPool(h)

	hd, err := p.Get()
	require.NoError(t, err)
	assert.Equal(t, 1, hd.id)
	assert.Equal(t, 1, h.opens)
	assert.Empty(t, p.free)
}

func TestGetReusesFreeHandle(t *testing.T) {
	h := &harness{}
	p := newPool(h)

	hd, err := p.Get()
	require.NoError(t, err)
	p.Put(hd, nil)
	assert.Len(t, p.free, 1)

	got, err := p.Get()
	require.NoError(t, err)
	assert.Same(t, hd, got, "a free handle should be reused instead of opening a new one")
	assert.Equal(t, 1, h.opens)
}

func TestGetOpenError(t *testing.T) {
	h := &harness{openErr: errors.New("connection failed")}
	p := newPool(h)

	hd, err := p.Get()
	assert.Error(t, err)
	assert.Nil(t, hd)
	assert.EqualError(t, err, "connection failed")
}

func TestPutSuccessKeepsHandle(t *testing.T) {
	h := &harness{}
	p := newPool(h)

	hd, err := p.Get()
	require.NoError(t, err)
	p.Put(hd, nil)

	assert.Len(t, p.free, 1)
	assert.Zero(t, h.releases, "a healthy handle must not be released")
}

func TestPutErrorReleasesHandle(t *testing.T) {
	h := &harness{}
	p := newPool(h)

	hd, err := p.Get()
	require.NoError(t, err)
	writeErr := errors.New("write error")
	p.Put(hd, writeErr)

	assert.Empty(t, p.free, "a handle put back with an error must not be reused")
	assert.Equal(t, 1, h.releases)
	assert.True(t, hd.released)
	assert.Equal(t, writeErr, hd.relErr, "release must receive the write error")
}

func TestDrainReleasesEveryHandle(t *testing.T) {
	h := &harness{}
	p := newPool(h)

	var handles []*handle
	for range 3 {
		hd, err := p.Get()
		require.NoError(t, err)
		handles = append(handles, hd)
	}
	for _, hd := range handles {
		p.Put(hd, nil)
	}

	require.NoError(t, p.Drain())
	assert.Empty(t, p.free)
	assert.Equal(t, 3, h.releases)
	for _, hd := range handles {
		assert.True(t, hd.released)
		assert.NoError(t, hd.relErr, "draining passes a nil error to release")
	}
}

func TestDrainReturnsCloseError(t *testing.T) {
	h := &harness{closeErr: errors.New("close failed")}
	p := newPool(h)

	hd, err := p.Get()
	require.NoError(t, err)
	p.Put(hd, nil)

	assert.EqualError(t, p.Drain(), "close failed")
}

func TestConcurrentGetPut(t *testing.T) {
	h := &harness{}
	p := newPool(h)

	const workers = 10
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for range 100 {
				hd, err := p.Get()
				if err != nil {
					return
				}
				p.Put(hd, nil)
			}
		}()
	}
	wg.Wait()

	// Every handle handed out was returned, so draining must release them all
	// with no leaks.
	require.NoError(t, p.Drain())
	assert.Empty(t, p.free)
	assert.Equal(t, h.opens, h.releases)
}
