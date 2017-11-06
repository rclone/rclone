package vfs

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/djherbis/times"
	"github.com/ncw/rclone/fstest"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context" // switch to "context" when we stop supporting go1.6
)

// Check CacheMode it satisfies the pflag interface
var _ pflag.Value = (*CacheMode)(nil)

func TestCacheModeString(t *testing.T) {
	assert.Equal(t, "off", CacheModeOff.String())
	assert.Equal(t, "full", CacheModeFull.String())
	assert.Equal(t, "CacheMode(17)", CacheMode(17).String())
}

func TestCacheModeSet(t *testing.T) {
	var m CacheMode

	err := m.Set("full")
	assert.NoError(t, err)
	assert.Equal(t, CacheModeFull, m)

	err = m.Set("potato")
	assert.Error(t, err, "Unknown cache mode level")

	err = m.Set("")
	assert.Error(t, err, "Unknown cache mode level")
}

func TestCacheModeType(t *testing.T) {
	var m CacheMode
	assert.Equal(t, "string", m.Type())
}

func TestCacheNew(t *testing.T) {
	r := fstest.NewRun(t)
	defer r.Finalise()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := newCache(ctx, r.Fremote, &DefaultOpt)
	require.NoError(t, err)

	assert.Contains(t, c.root, "vfs")
	assert.Contains(t, c.f.Root(), filepath.Base(r.Fremote.Root()))

	// mkdir
	p, err := c.mkdir("potato")
	require.NoError(t, err)
	assert.Equal(t, "potato", filepath.Base(p))

	fi, err := os.Stat(filepath.Dir(p))
	require.NoError(t, err)
	assert.True(t, fi.IsDir())

	// get
	item := c.get("potato")
	item2 := c.get("potato")
	assert.Equal(t, item, item2)
	assert.WithinDuration(t, time.Now(), item.atime, time.Second)

	// updateTime
	//.. before
	t1 := time.Now().Add(-60 * time.Minute)
	c.updateTime("potato", t1)
	item = c.get("potato")
	assert.NotEqual(t, t1, item.atime)
	assert.Equal(t, 0, item.opens)
	//..after
	t2 := time.Now().Add(60 * time.Minute)
	c.updateTime("potato", t2)
	item = c.get("potato")
	assert.Equal(t, t2, item.atime)
	assert.Equal(t, 0, item.opens)

	// open
	c.open("potato")
	item = c.get("potato")
	assert.WithinDuration(t, time.Now(), item.atime, time.Second)
	assert.Equal(t, 1, item.opens)

	// write the file
	err = ioutil.WriteFile(p, []byte("hello"), 0600)
	require.NoError(t, err)

	// read its atime
	fi, err = os.Stat(p)
	assert.NoError(t, err)
	atime := times.Get(fi).AccessTime()

	// updateAtimes
	log.Printf("updateAtimes")
	item = c.get("potato")
	item.atime = time.Now().Add(-24 * time.Hour)
	err = c.updateAtimes()
	require.NoError(t, err)
	item = c.get("potato")
	assert.Equal(t, atime, item.atime)

	// try purging with file open
	c.purgeOld(10 * time.Second)
	_, err = os.Stat(p)
	assert.NoError(t, err)

	// close
	c.updateTime("potato", t2)
	c.close("potato")
	item = c.get("potato")
	assert.WithinDuration(t, time.Now(), item.atime, time.Second)
	assert.Equal(t, 0, item.opens)

	// try purging with file closed
	c.purgeOld(10 * time.Second)
	// ...nothing should happend
	_, err = os.Stat(p)
	assert.NoError(t, err)

	//.. purge again with -ve age
	c.purgeOld(-10 * time.Second)
	_, err = os.Stat(p)
	assert.True(t, os.IsNotExist(err))

	// clean - have tested the internals already
	c.clean()

	// cleanup
	err = c.cleanUp()
	require.NoError(t, err)
	_, err = os.Stat(c.root)
	assert.True(t, os.IsNotExist(err))
}
