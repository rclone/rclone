package utils

import (
	"context"
	"io/ioutil"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pengsrc/go-shared/log"
)

func TestRecover(t *testing.T) {
	discardLogger, err := log.NewLogger(ioutil.Discard)
	assert.NoError(t, err)
	log.SetGlobalLogger(discardLogger)
	defer log.SetGlobalLogger(nil)

	ctx := context.Background()

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer Recover(ctx)
		wg.Done()
		panic("fear")
	}()
	wg.Wait()
}
