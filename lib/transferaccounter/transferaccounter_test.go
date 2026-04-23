package transferaccounter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	// Dummy add function
	var totalBytes int64
	addFn := func(n int64) {
		totalBytes += n
	}

	// Create the accounter
	ctx := context.Background()
	_, ta := New(ctx, addFn)

	// Verify object creation
	require.NotNil(t, ta)
	assert.False(t, ta.Started(), "New accounter should not be started by default")

	// Test Start()
	ta.Start()
	assert.True(t, ta.Started(), "Accounter should be started after calling Start()")

	// Test Add() logic
	ta.Add(100)
	ta.Add(50)
	assert.Equal(t, int64(150), totalBytes, "The add function should have been called with cumulative values")
	assert.Equal(t, int64(150), ta.total.Load(), "Internal counter did not count")

	// Test Reset() logic
	ta.Reset()
	assert.Equal(t, int64(0), totalBytes, "The Reset function failed")
	assert.Equal(t, int64(0), ta.total.Load(), "Internal counter did not reset")

}

func TestGet(t *testing.T) {
	t.Run("Retrieve existing accounter", func(t *testing.T) {
		// Create a specific accounter to identify later
		expectedTotal := int64(0)
		ctx, originalTa := New(context.Background(), func(n int64) { expectedTotal += n })

		// Retrieve it
		retrievedTa := Get(ctx)

		// Assert it is the exact same pointer
		assert.Equal(t, originalTa, retrievedTa)

		// Verify functionality passes through
		retrievedTa.Add(10)
		assert.Equal(t, int64(10), expectedTotal)
	})

	t.Run("Context does not contain accounter", func(t *testing.T) {
		ctx := context.Background()
		ta := Get(ctx)

		assert.NotNil(t, ta, "Get should never return nil")
		assert.Equal(t, nullAccounter, ta, "Should return the global nullAccounter")
	})

	t.Run("Context is nil", func(t *testing.T) {
		ta := Get(nil) //nolint:staticcheck // we want to test this

		assert.NotNil(t, ta, "Get should never return nil")
		assert.Equal(t, nullAccounter, ta, "Should return the global nullAccounter")
	})
}

func TestNullAccounterBehavior(t *testing.T) {
	// Ensure the null accounter (returned when context is missing/nil)
	// can be called without panicking.
	ta := Get(nil) //nolint:staticcheck // we want to test this

	assert.NotPanics(t, func() {
		ta.Start()
	})

	// Even after start, it acts as a valid object
	assert.True(t, ta.Started())

	assert.NotPanics(t, func() {
		ta.Add(1000)
	})
}
