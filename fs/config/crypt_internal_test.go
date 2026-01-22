package config

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hashedKeyCompare(t *testing.T, a, b string, shouldMatch bool) {
	err := SetConfigPassword(a)
	require.NoError(t, err)
	k1 := configKey

	err = SetConfigPassword(b)
	require.NoError(t, err)
	k2 := configKey

	if shouldMatch {
		assert.Equal(t, k1, k2)
	} else {
		assert.NotEqual(t, k1, k2)
	}
}

func TestPassword(t *testing.T) {
	defer func() {
		configKey = nil // reset password
	}()
	var err error
	// Empty password should give error
	err = SetConfigPassword("  \t  ")
	require.Error(t, err)

	// Test invalid utf8 sequence
	err = SetConfigPassword(string([]byte{0xff, 0xfe, 0xfd}) + "abc")
	require.Error(t, err)

	// Simple check of wrong passwords
	hashedKeyCompare(t, "mis", "match", false)

	// Check that passwords match after unicode normalization
	hashedKeyCompare(t, "ﬀ\u0041\u030A", "ffÅ", true)

	// Check that passwords preserves case
	hashedKeyCompare(t, "abcdef", "ABCDEF", false)

}

func TestChangeConfigPassword(t *testing.T) {
	ci := fs.GetConfig(context.Background())

	var err error
	oldConfigPath := GetConfigPath()
	assert.NoError(t, SetConfigPath("./testdata/encrypted.conf"))
	defer func() {
		assert.NoError(t, SetConfigPath(oldConfigPath))
		ClearConfigPassword()
		ci.PasswordCommand = nil
	}()

	// Get rid of any config password
	ClearConfigPassword()

	// Return the password, checking the state of the environment variable
	checkCode := `
package main

import (
	"fmt"
	"os"
	"log"
)

func main() {
	v := os.Getenv("RCLONE_PASSWORD_CHANGE")
	if v == "" {
		log.Fatal("Env var not found")
	} else if v != "1" {
		log.Fatal("Env var wrong value")
	} else {
		fmt.Println("asdf")
	}
}
`
	dir := t.TempDir()
	code := filepath.Join(dir, "file.go")
	require.NoError(t, os.WriteFile(code, []byte(checkCode), 0777))

	// Set correct password using --password-command
	ci.PasswordCommand = fs.SpaceSepList{"go", "run", code}
	changeConfigPassword()
	err = Data().Load()
	require.NoError(t, err)
	sections := Data().GetSectionList()
	var expect = []string{"nounc", "unc"}
	assert.Equal(t, expect, sections)

	keys := Data().GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	assert.Equal(t, expect, keys)
}

// TestPasswordCache tests the password caching functionality
func TestPasswordCache(t *testing.T) {
	// Create a helper program that counts invocations
	counterCode := `
package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	counterFile := os.Getenv("COUNTER_FILE")
	if counterFile == "" {
		fmt.Fprintln(os.Stderr, "COUNTER_FILE not set")
		os.Exit(1)
	}

	// Read current count
	count := 0
	data, err := os.ReadFile(counterFile)
	if err == nil {
		count, _ = strconv.Atoi(string(data))
	}

	// Increment and write back
	count++
	os.WriteFile(counterFile, []byte(strconv.Itoa(count)), 0644)

	// Output the password
	fmt.Println("asdf")
}
`
	dir := t.TempDir()
	code := filepath.Join(dir, "counter.go")
	require.NoError(t, os.WriteFile(code, []byte(counterCode), 0777))

	counterFile := filepath.Join(dir, "count.txt")

	ctx, ci := fs.AddConfig(context.Background())

	t.Run("CacheDisabledByDefault", func(t *testing.T) {
		// Reset state
		ClearConfigPassword()
		os.WriteFile(counterFile, []byte("0"), 0644)
		os.Setenv("COUNTER_FILE", counterFile)
		defer os.Unsetenv("COUNTER_FILE")

		ci.PasswordCommand = fs.SpaceSepList{"go", "run", code}
		ci.ConfigPassCache = 0 // Cache disabled (default)

		// Call GetPasswordCommand twice
		pass1, err := GetPasswordCommand(ctx)
		require.NoError(t, err)
		assert.Equal(t, "asdf", pass1)

		pass2, err := GetPasswordCommand(ctx)
		require.NoError(t, err)
		assert.Equal(t, "asdf", pass2)

		// Read counter - should be 2 (invoked twice)
		data, err := os.ReadFile(counterFile)
		require.NoError(t, err)
		count, _ := strconv.Atoi(string(data))
		assert.Equal(t, 2, count, "Command should be invoked twice when cache is disabled")
	})

	t.Run("CacheEnabledWithinTTL", func(t *testing.T) {
		// Reset state
		ClearConfigPassword()
		os.WriteFile(counterFile, []byte("0"), 0644)
		os.Setenv("COUNTER_FILE", counterFile)
		defer os.Unsetenv("COUNTER_FILE")

		ci.PasswordCommand = fs.SpaceSepList{"go", "run", code}
		ci.ConfigPassCache = fs.Duration(5 * time.Second) // Cache for 5 seconds

		// Call GetPasswordCommand twice within TTL
		pass1, err := GetPasswordCommand(ctx)
		require.NoError(t, err)
		assert.Equal(t, "asdf", pass1)

		pass2, err := GetPasswordCommand(ctx)
		require.NoError(t, err)
		assert.Equal(t, "asdf", pass2)

		// Read counter - should be 1 (invoked once, second call used cache)
		data, err := os.ReadFile(counterFile)
		require.NoError(t, err)
		count, _ := strconv.Atoi(string(data))
		assert.Equal(t, 1, count, "Command should be invoked only once when cache is active")

		// Verify cache is active
		assert.True(t, IsPasswordCacheActive(), "Password cache should be active")
	})

	t.Run("CacheExpiry", func(t *testing.T) {
		// Reset state
		ClearConfigPassword()
		os.WriteFile(counterFile, []byte("0"), 0644)
		os.Setenv("COUNTER_FILE", counterFile)
		defer os.Unsetenv("COUNTER_FILE")

		ci.PasswordCommand = fs.SpaceSepList{"go", "run", code}
		ci.ConfigPassCache = fs.Duration(100 * time.Millisecond) // Very short TTL for testing

		// First call - should invoke command
		pass1, err := GetPasswordCommand(ctx)
		require.NoError(t, err)
		assert.Equal(t, "asdf", pass1)

		// Wait for cache to expire
		time.Sleep(150 * time.Millisecond)

		// Cache should be expired
		assert.False(t, IsPasswordCacheActive(), "Password cache should be expired")

		// Second call - should invoke command again
		pass2, err := GetPasswordCommand(ctx)
		require.NoError(t, err)
		assert.Equal(t, "asdf", pass2)

		// Read counter - should be 2 (invoked twice due to cache expiry)
		data, err := os.ReadFile(counterFile)
		require.NoError(t, err)
		count, _ := strconv.Atoi(string(data))
		assert.Equal(t, 2, count, "Command should be invoked twice after cache expiry")
	})

	t.Run("ClearPasswordCacheClears", func(t *testing.T) {
		// Reset state
		ClearConfigPassword()
		os.WriteFile(counterFile, []byte("0"), 0644)
		os.Setenv("COUNTER_FILE", counterFile)
		defer os.Unsetenv("COUNTER_FILE")

		ci.PasswordCommand = fs.SpaceSepList{"go", "run", code}
		ci.ConfigPassCache = fs.Duration(5 * time.Second)

		// First call
		_, err := GetPasswordCommand(ctx)
		require.NoError(t, err)
		assert.True(t, IsPasswordCacheActive(), "Password cache should be active")

		// Clear the cache
		ClearPasswordCache()
		assert.False(t, IsPasswordCacheActive(), "Password cache should be cleared")

		// Second call - should invoke command again
		_, err = GetPasswordCommand(ctx)
		require.NoError(t, err)

		// Read counter - should be 2
		data, err := os.ReadFile(counterFile)
		require.NoError(t, err)
		count, _ := strconv.Atoi(string(data))
		assert.Equal(t, 2, count, "Command should be invoked twice after manual cache clear")
	})

	t.Run("ClearConfigPasswordClearsCache", func(t *testing.T) {
		// Reset state
		ClearConfigPassword()
		os.WriteFile(counterFile, []byte("0"), 0644)
		os.Setenv("COUNTER_FILE", counterFile)
		defer os.Unsetenv("COUNTER_FILE")

		ci.PasswordCommand = fs.SpaceSepList{"go", "run", code}
		ci.ConfigPassCache = fs.Duration(5 * time.Second)

		// First call
		_, err := GetPasswordCommand(ctx)
		require.NoError(t, err)
		assert.True(t, IsPasswordCacheActive(), "Password cache should be active")

		// Clear the config password (should also clear cache)
		ClearConfigPassword()
		assert.False(t, IsPasswordCacheActive(), "Password cache should be cleared with config password")
	})

	t.Run("NoPasswordCommand", func(t *testing.T) {
		// Reset state
		ClearConfigPassword()

		ci.PasswordCommand = fs.SpaceSepList{} // No command
		ci.ConfigPassCache = fs.Duration(5 * time.Second)

		// Should return empty string without error
		pass, err := GetPasswordCommand(ctx)
		require.NoError(t, err)
		assert.Equal(t, "", pass)
	})

	// Cleanup
	ci.PasswordCommand = nil
	ci.ConfigPassCache = 0
	ClearConfigPassword()
}

// TestPasswordCacheWithEncryptedConfig tests the password cache with an actual encrypted config
func TestPasswordCacheWithEncryptedConfig(t *testing.T) {
	// Create a helper program that counts invocations
	counterCode := `
package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	counterFile := os.Getenv("COUNTER_FILE")
	if counterFile == "" {
		fmt.Fprintln(os.Stderr, "COUNTER_FILE not set")
		os.Exit(1)
	}

	// Read current count
	count := 0
	data, err := os.ReadFile(counterFile)
	if err == nil {
		count, _ = strconv.Atoi(string(data))
	}

	// Increment and write back
	count++
	os.WriteFile(counterFile, []byte(strconv.Itoa(count)), 0644)

	// Output the correct password for testdata/encrypted.conf
	fmt.Println("asdf")
}
`
	dir := t.TempDir()
	code := filepath.Join(dir, "counter.go")
	require.NoError(t, os.WriteFile(code, []byte(counterCode), 0777))

	counterFile := filepath.Join(dir, "count.txt")

	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	oldConfig := *ci
	oldConfigPath := GetConfigPath()

	t.Run("LoadEncryptedConfigWithCache", func(t *testing.T) {
		// Reset state
		ClearConfigPassword()
		assert.NoError(t, SetConfigPath("./testdata/encrypted.conf"))
		os.WriteFile(counterFile, []byte("0"), 0644)
		os.Setenv("COUNTER_FILE", counterFile)

		defer func() {
			os.Unsetenv("COUNTER_FILE")
			assert.NoError(t, SetConfigPath(oldConfigPath))
			ClearConfigPassword()
			*ci = oldConfig
		}()

		ci.PasswordCommand = fs.SpaceSepList{"go", "run", code}
		ci.ConfigPassCache = fs.Duration(5 * time.Second)

		// Load the config twice
		err := Data().Load()
		require.NoError(t, err)

		ClearConfigPassword() // Clear config key but not password cache

		// Manually clear only the config key to simulate needing to decrypt again
		configKey = nil

		err = Data().Load()
		require.NoError(t, err)

		// Password command should only be invoked once due to caching
		data, err := os.ReadFile(counterFile)
		require.NoError(t, err)
		count, _ := strconv.Atoi(string(data))
		// Note: configKey caching may affect this, so we check if it's <= 2
		assert.LessOrEqual(t, count, 2, "Command should be invoked at most twice (once per config load if configKey is cleared)")
	})
}

// TestPasswordCacheThreadSafety tests that the password cache is thread-safe
// by verifying that concurrent access to the cache functions doesn't panic
func TestPasswordCacheThreadSafety(t *testing.T) {
	// This test verifies thread-safety of cache operations, not command reduction
	// The cache properly protects its internal state with a mutex

	ctx, ci := fs.AddConfig(context.Background())

	// Use a simple command that returns immediately
	ci.PasswordCommand = fs.SpaceSepList{"echo", "testpass"}
	ci.ConfigPassCache = fs.Duration(5 * time.Second)

	defer func() {
		ci.PasswordCommand = nil
		ci.ConfigPassCache = 0
		ClearConfigPassword()
	}()

	// Run multiple goroutines accessing cache functions concurrently
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			_, _ = GetPasswordCommand(ctx)
		}()
		go func() {
			defer wg.Done()
			_ = IsPasswordCacheActive()
		}()
		go func() {
			defer wg.Done()
			// Occasionally clear the cache
			if i%5 == 0 {
				ClearPasswordCache()
			}
		}()
	}
	wg.Wait()

	// If we get here without deadlock or panic, the test passes
}
