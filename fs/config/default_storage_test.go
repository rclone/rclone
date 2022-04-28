package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultStorage(t *testing.T) {
	a := assert.New(t)

	ds := newDefaultStorage()

	section := "test"
	key := "key"
	val := "something"
	ds.SetValue(section, key, val)
	ds.SetValue("some other section", key, val)

	v, hasVal := ds.GetValue(section, key)
	a.True(hasVal)
	a.Equal(val, v)

	a.ElementsMatch([]string{section, "some other section"}, ds.GetSectionList())
	a.True(ds.HasSection(section))
	a.False(ds.HasSection("nope"))

	a.Equal([]string{key}, ds.GetKeyList(section))

	_, err := ds.Serialize()
	a.NoError(err)

	a.True(ds.DeleteKey(section, key))
	a.False(ds.DeleteKey(section, key))
	a.False(ds.DeleteKey("not there", key))

	_, hasVal = ds.GetValue(section, key)
	a.False(hasVal)

	ds.DeleteSection(section)
	a.False(ds.HasSection(section))
}
