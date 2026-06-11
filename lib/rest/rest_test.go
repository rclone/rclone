package rest

import (
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientGetter_ReturnsSameClient(t *testing.T) {
	httpClient := &http.Client{}
	api := NewClient(httpClient)

	assert.Same(t, httpClient, api.Client())
}

func TestClientGetter_Concurrent(t *testing.T) {
	api := NewClient(&http.Client{})

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = api.Client()
		}()
	}
	wg.Wait()
}

func TestClientGetter_ConcurrentWithWriter(t *testing.T) {
	api := NewClient(&http.Client{})
	noopHandler := func(resp *http.Response) error { return nil }

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = api.Client()
		}()
		go func() {
			defer wg.Done()
			api.SetErrorHandler(noopHandler)
		}()
	}
	wg.Wait()
}
