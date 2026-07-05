package zoho

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTPSMinSleep(t *testing.T) {
	// A transactions-per-second rate maps to 1s divided by that rate.
	assert.Equal(t, time.Second, tpsMinSleep(1))
	assert.Equal(t, 500*time.Millisecond, tpsMinSleep(2))
	assert.Equal(t, time.Second/6, tpsMinSleep(6)) // the default of 6 tps
}
