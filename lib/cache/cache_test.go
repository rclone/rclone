package cache

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	called      = 0
	errSentinel = errors.New("an error")
	errCached   = errors.New("a cached error")
)

func setup(t *testing.T) (*Cache, CreateFunc) {
	called = 0
	create := func(path string) (interface{}, bool, error) {
		assert.Equal(t, 0, called)
		called++
		switch path {
		case "/":
			return "/", true, nil
		case "/file.txt":
			return "/file.txt", true, errCached
		case "/error":
			return nil, false, errSentinel
		case "/err":
			return nil, false, errSentinel
		}
		panic(fmt.Sprintf("Unknown path %q", path))
	}
	c := New()
	return c, create
}

func TestGet(t *testing.T) {
	c, create := setup(t)

	assert.Equal(t, 0, len(c.cache))

	f, err := c.Get("/", create)
	require.NoError(t, err)

	assert.Equal(t, 1, len(c.cache))

	f2, err := c.Get("/", create)
	require.NoError(t, err)

	assert.Equal(t, f, f2)
}

func TestGetFile(t *testing.T) {
	c, create := setup(t)

	assert.Equal(t, 0, len(c.cache))

	f, err := c.Get("/file.txt", create)
	require.Equal(t, errCached, err)

	assert.Equal(t, 1, len(c.cache))

	f2, err := c.Get("/file.txt", create)
	require.Equal(t, errCached, err)

	assert.Equal(t, f, f2)
}

func TestGetError(t *testing.T) {
	c, create := setup(t)

	assert.Equal(t, 0, len(c.cache))

	f, err := c.Get("/error", create)
	require.Equal(t, errSentinel, err)
	require.Equal(t, nil, f)

	assert.Equal(t, 0, len(c.cache))
}

func TestPut(t *testing.T) {
	c, create := setup(t)

	assert.Equal(t, 0, len(c.cache))

	c.Put("/alien", "slime")

	assert.Equal(t, 1, len(c.cache))

	fNew, err := c.Get("/alien", create)
	require.NoError(t, err)
	require.Equal(t, "slime", fNew)

	assert.Equal(t, 1, len(c.cache))
}

func TestCacheExpire(t *testing.T) {
	c, create := setup(t)

	c.SetExpireInterval(time.Millisecond)
	assert.Equal(t, false, c.expireRunning)

	_, err := c.Get("/", create)
	require.NoError(t, err)

	c.mu.Lock()
	entry := c.cache["/"]
	assert.Equal(t, 1, len(c.cache))
	c.mu.Unlock()

	c.cacheExpire()

	c.mu.Lock()
	assert.Equal(t, 1, len(c.cache))
	entry.lastUsed = time.Now().Add(-c.expireDuration - 60*time.Second)
	assert.Equal(t, true, c.expireRunning)
	c.mu.Unlock()

	time.Sleep(250 * time.Millisecond)

	c.mu.Lock()
	assert.Equal(t, false, c.expireRunning)
	assert.Equal(t, 0, len(c.cache))
	c.mu.Unlock()
}

func TestCacheNoExpire(t *testing.T) {
	c, create := setup(t)

	assert.False(t, c.noCache())

	c.SetExpireDuration(0)
	assert.Equal(t, false, c.expireRunning)

	assert.True(t, c.noCache())

	f, err := c.Get("/", create)
	require.NoError(t, err)
	require.NotNil(t, f)

	c.mu.Lock()
	assert.Equal(t, 0, len(c.cache))
	c.mu.Unlock()

	c.Put("/alien", "slime")

	c.mu.Lock()
	assert.Equal(t, 0, len(c.cache))
	c.mu.Unlock()
}

func TestCachePin(t *testing.T) {
	c, create := setup(t)

	_, err := c.Get("/", create)
	require.NoError(t, err)

	// Pin a non existent item to show nothing happens
	c.Pin("notfound")

	c.mu.Lock()
	entry := c.cache["/"]
	assert.Equal(t, 1, len(c.cache))
	c.mu.Unlock()

	c.cacheExpire()

	c.mu.Lock()
	assert.Equal(t, 1, len(c.cache))
	c.mu.Unlock()

	// Pin the entry and check it does not get expired
	c.Pin("/")

	// Reset last used to make the item expirable
	c.mu.Lock()
	entry.lastUsed = time.Now().Add(-c.expireDuration - 60*time.Second)
	c.mu.Unlock()

	c.cacheExpire()

	c.mu.Lock()
	assert.Equal(t, 1, len(c.cache))
	c.mu.Unlock()

	// Unpin the entry and check it does get expired now
	c.Unpin("/")

	// Reset last used
	c.mu.Lock()
	entry.lastUsed = time.Now().Add(-c.expireDuration - 60*time.Second)
	c.mu.Unlock()

	c.cacheExpire()

	c.mu.Lock()
	assert.Equal(t, 0, len(c.cache))
	c.mu.Unlock()
}

func TestClear(t *testing.T) {
	c, create := setup(t)

	assert.Equal(t, 0, len(c.cache))

	_, err := c.Get("/", create)
	require.NoError(t, err)

	assert.Equal(t, 1, len(c.cache))

	c.Clear()

	assert.Equal(t, 0, len(c.cache))
}

func TestEntries(t *testing.T) {
	c, create := setup(t)

	assert.Equal(t, 0, c.Entries())

	_, err := c.Get("/", create)
	require.NoError(t, err)

	assert.Equal(t, 1, c.Entries())

	c.Clear()

	assert.Equal(t, 0, c.Entries())
}

func TestGetMaybe(t *testing.T) {
	c, create := setup(t)

	value, found := c.GetMaybe("/")
	assert.Equal(t, false, found)
	assert.Nil(t, value)

	f, err := c.Get("/", create)
	require.NoError(t, err)

	value, found = c.GetMaybe("/")
	assert.Equal(t, true, found)
	assert.Equal(t, f, value)

	c.Clear()

	value, found = c.GetMaybe("/")
	assert.Equal(t, false, found)
	assert.Nil(t, value)
}

func TestDelete(t *testing.T) {
	c, create := setup(t)

	assert.Equal(t, 0, len(c.cache))

	_, err := c.Get("/", create)
	require.NoError(t, err)

	assert.Equal(t, 1, len(c.cache))

	assert.Equal(t, false, c.Delete("notfound"))
	assert.Equal(t, 1, len(c.cache))

	assert.Equal(t, true, c.Delete("/"))
	assert.Equal(t, 0, len(c.cache))

	assert.Equal(t, false, c.Delete("/"))
	assert.Equal(t, 0, len(c.cache))
}

func TestDeletePrefix(t *testing.T) {
	create := func(path string) (interface{}, bool, error) {
		return path, true, nil
	}
	c := New()

	_, err := c.Get("remote:path", create)
	require.NoError(t, err)
	_, err = c.Get("remote:path2", create)
	require.NoError(t, err)
	_, err = c.Get("remote:", create)
	require.NoError(t, err)
	_, err = c.Get("remote", create)
	require.NoError(t, err)

	assert.Equal(t, 4, len(c.cache))

	assert.Equal(t, 3, c.DeletePrefix("remote:"))
	assert.Equal(t, 1, len(c.cache))

	assert.Equal(t, 1, c.DeletePrefix(""))
	assert.Equal(t, 0, len(c.cache))

	assert.Equal(t, 0, c.DeletePrefix(""))
	assert.Equal(t, 0, len(c.cache))
}

func TestCacheRename(t *testing.T) {
	c := New()
	create := func(path string) (interface{}, bool, error) {
		return path, true, nil
	}

	existing1, err := c.Get("existing1", create)
	require.NoError(t, err)
	_, err = c.Get("existing2", create)
	require.NoError(t, err)

	assert.Equal(t, 2, c.Entries())

	// rename to non existent
	value, found := c.Rename("existing1", "EXISTING1")
	assert.Equal(t, true, found)
	assert.Equal(t, existing1, value)

	assert.Equal(t, 2, c.Entries())

	// rename to existent and check existing value is returned
	value, found = c.Rename("existing2", "EXISTING1")
	assert.Equal(t, true, found)
	assert.Equal(t, existing1, value)

	assert.Equal(t, 1, c.Entries())

	// rename non existent
	value, found = c.Rename("notfound", "NOTFOUND")
	assert.Equal(t, false, found)
	assert.Nil(t, value)

	assert.Equal(t, 1, c.Entries())
}
