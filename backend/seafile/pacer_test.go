package seafile

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The pacer is shared by every request to a server, so its minimum sleep
// caps the request rate. Uploading a small file takes two requests, so a
// large minSleep makes many small files slow whatever --transfers is.
func TestPacerDoesNotThrottleSmallFiles(t *testing.T) {
	ctx := context.Background()
	p := getPacer(ctx, "https://pacer-test.invalid")

	const calls = 50
	start := time.Now()
	for i := 0; i < calls; i++ {
		err := p.Call(func() (bool, error) { return false, nil })
		require.NoError(t, err)
	}
	// 50 calls at minSleep=100ms take about 5s, at 10ms about 0.5s
	assert.Less(t, time.Since(start), 2*time.Second)
}
