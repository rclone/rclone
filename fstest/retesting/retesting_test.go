package retesting_test

import (
	"testing"

	"github.com/rclone/rclone/fstest/retesting"
	"github.com/stretchr/testify/assert"
)

var savedRetries int

func setRetries(newRetries int) {
	savedRetries = *retesting.FlakeyRetries
	*retesting.FlakeyRetries = newRetries
}

func restoreRetries() {
	*retesting.FlakeyRetries = savedRetries
}

func TestRetriesOneDeep(t *testing.T) {
	setRetries(2)
	defer restoreRetries()

	boyCount := 0

	retesting.Run(t, "boy", func(t retesting.T) {
		boyCount++
		pass := boyCount > 1 // fail at #1 then pass at #2
		t.Logf("boy run #%d - %v pass", boyCount, pass)
		assert.True(t, pass)
	})

	assert.Equal(t, 2, boyCount, "boy must retry 2 times")
}

func TestRetriesTwoDeep(t *testing.T) {
	setRetries(2)
	defer restoreRetries()

	dadCount := 0
	sonCount := 0

	retesting.Run(t, "dad", func(t retesting.T) {
		dadCount++
		t.Logf("dad run #%d", dadCount)
		retesting.Run(t, "son", func(t retesting.T) {
			sonCount++
			pass := sonCount > 1 // fail at #1 then pass at #2
			t.Logf("son run #%d - %v pass", sonCount, pass)
			assert.True(t, pass)
		})
	})

	assert.Equal(t, 2, dadCount, "dad must retry 2 times")
	assert.Equal(t, 2, sonCount, "son must retry 2 times")
}
