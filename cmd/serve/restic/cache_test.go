package restic

import (
	"sort"
	"strings"
	"testing"

	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
)

func (c *cache) String() string {
	keys := []string{}
	c.mu.Lock()
	for k := range c.items {
		keys = append(keys, k)
	}
	c.mu.Unlock()
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func TestCacheCRUD(t *testing.T) {
	c := newCache(true)
	assert.Equal(t, "", c.String())
	assert.Nil(t, c.find("potato"))
	o := mockobject.New("potato")
	c.add(o.Remote(), o)
	assert.Equal(t, "potato", c.String())
	assert.Equal(t, o, c.find("potato"))
	c.remove("potato")
	assert.Equal(t, "", c.String())
	assert.Nil(t, c.find("potato"))
	c.remove("notfound")
}

func TestCacheRemovePrefix(t *testing.T) {
	c := newCache(true)
	for _, remote := range []string{
		"a",
		"b",
		"b/1",
		"b/2/3",
		"b/2/4",
		"b/2",
		"c",
	} {
		c.add(remote, mockobject.New(remote))
	}
	assert.Equal(t, "a,b,b/1,b/2,b/2/3,b/2/4,c", c.String())
	c.removePrefix("b")
	assert.Equal(t, "a,b,c", c.String())
	c.removePrefix("/")
	assert.Equal(t, "", c.String())
}
