package configmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	_ Mapper = Simple(nil)
	_ Getter = Simple(nil)
	_ Setter = Simple(nil)
)

func TestConfigMapGet(t *testing.T) {
	m := New()

	value, found := m.Get("config1")
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	value, found = m.Get("config2")
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	m1 := Simple{
		"config1": "one",
	}

	m.AddGetter(m1)

	value, found = m.Get("config1")
	assert.Equal(t, "one", value)
	assert.Equal(t, true, found)

	value, found = m.Get("config2")
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	m2 := Simple{
		"config1": "one2",
		"config2": "two2",
	}

	m.AddGetter(m2)

	value, found = m.Get("config1")
	assert.Equal(t, "one", value)
	assert.Equal(t, true, found)

	value, found = m.Get("config2")
	assert.Equal(t, "two2", value)
	assert.Equal(t, true, found)

}

func TestConfigMapSet(t *testing.T) {
	m := New()

	m1 := Simple{
		"config1": "one",
	}
	m2 := Simple{
		"config1": "one2",
		"config2": "two2",
	}

	m.AddSetter(m1).AddSetter(m2)

	m.Set("config2", "potato")

	assert.Equal(t, Simple{
		"config1": "one",
		"config2": "potato",
	}, m1)
	assert.Equal(t, Simple{
		"config1": "one2",
		"config2": "potato",
	}, m2)

	m.Set("config1", "beetroot")

	assert.Equal(t, Simple{
		"config1": "beetroot",
		"config2": "potato",
	}, m1)
	assert.Equal(t, Simple{
		"config1": "beetroot",
		"config2": "potato",
	}, m2)
}

func TestConfigMapGetOverride(t *testing.T) {
	m := New()

	value, found := m.GetOverride("config1")
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	value, found = m.GetOverride("config2")
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	m1 := Simple{
		"config1": "one",
	}

	m.AddOverrideGetter(m1)

	value, found = m.GetOverride("config1")
	assert.Equal(t, "one", value)
	assert.Equal(t, true, found)

	value, found = m.GetOverride("config2")
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	m2 := Simple{
		"config1": "one2",
		"config2": "two2",
	}

	m.AddGetter(m2)

	value, found = m.GetOverride("config1")
	assert.Equal(t, "one", value)
	assert.Equal(t, true, found)

	value, found = m.GetOverride("config2")
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	value, found = m.Get("config1")
	assert.Equal(t, "one", value)
	assert.Equal(t, true, found)

	value, found = m.Get("config2")
	assert.Equal(t, "two2", value)
	assert.Equal(t, true, found)

}

func TestSimpleString(t *testing.T) {
	// Basic
	assert.Equal(t, "", Simple(nil).String())
	assert.Equal(t, "", Simple{}.String())
	assert.Equal(t, "config1='one'", Simple{
		"config1": "one",
	}.String())

	// Check ordering
	assert.Equal(t, "config1='one',config2='two',config3='three',config4='four',config5='five'", Simple{
		"config5": "five",
		"config4": "four",
		"config3": "three",
		"config2": "two",
		"config1": "one",
	}.String())

	// Check escaping
	assert.Equal(t, "apple='',config1='o''n''e'", Simple{
		"config1": "o'n'e",
		"apple":   "",
	}.String())
}
