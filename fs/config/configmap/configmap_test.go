package configmap

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	m.AddGetter(m1, PriorityNormal)

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

	m.AddGetter(m2, PriorityNormal)

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

	m.ClearSetters()

	// Check that nothing gets set
	m.Set("config1", "BEETROOT")

	assert.Equal(t, Simple{
		"config1": "beetroot",
		"config2": "potato",
	}, m1)
	assert.Equal(t, Simple{
		"config1": "beetroot",
		"config2": "potato",
	}, m2)

}

func TestConfigMapGetPriority(t *testing.T) {
	m := New()

	value, found := m.GetPriority("config1", PriorityMax)
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	value, found = m.GetPriority("config2", PriorityMax)
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	m1 := Simple{
		"config1": "one",
		"config3": "three",
	}

	m.AddGetter(m1, PriorityConfig)

	value, found = m.GetPriority("config1", PriorityNormal)
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	value, found = m.GetPriority("config2", PriorityNormal)
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	value, found = m.GetPriority("config3", PriorityNormal)
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	value, found = m.GetPriority("config1", PriorityConfig)
	assert.Equal(t, "one", value)
	assert.Equal(t, true, found)

	value, found = m.GetPriority("config2", PriorityConfig)
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	value, found = m.GetPriority("config3", PriorityConfig)
	assert.Equal(t, "three", value)
	assert.Equal(t, true, found)

	value, found = m.GetPriority("config1", PriorityMax)
	assert.Equal(t, "one", value)
	assert.Equal(t, true, found)

	value, found = m.GetPriority("config2", PriorityMax)
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	value, found = m.GetPriority("config3", PriorityMax)
	assert.Equal(t, "three", value)
	assert.Equal(t, true, found)

	m2 := Simple{
		"config1": "one2",
		"config2": "two2",
	}

	m.AddGetter(m2, PriorityNormal)

	value, found = m.GetPriority("config1", PriorityNormal)
	assert.Equal(t, "one2", value)
	assert.Equal(t, true, found)

	value, found = m.GetPriority("config2", PriorityNormal)
	assert.Equal(t, "two2", value)
	assert.Equal(t, true, found)

	value, found = m.GetPriority("config3", PriorityNormal)
	assert.Equal(t, "", value)
	assert.Equal(t, false, found)

	value, found = m.GetPriority("config1", PriorityConfig)
	assert.Equal(t, "one2", value)
	assert.Equal(t, true, found)

	value, found = m.GetPriority("config2", PriorityConfig)
	assert.Equal(t, "two2", value)
	assert.Equal(t, true, found)

	value, found = m.GetPriority("config3", PriorityConfig)
	assert.Equal(t, "three", value)
	assert.Equal(t, true, found)

	value, found = m.GetPriority("config1", PriorityMax)
	assert.Equal(t, "one2", value)
	assert.Equal(t, true, found)

	value, found = m.GetPriority("config2", PriorityMax)
	assert.Equal(t, "two2", value)
	assert.Equal(t, true, found)

	value, found = m.GetPriority("config3", PriorityMax)
	assert.Equal(t, "three", value)
	assert.Equal(t, true, found)
}

func TestConfigMapClearGetters(t *testing.T) {
	m := New()
	m1 := Simple{}
	m2 := Simple{}
	m3 := Simple{}
	m.AddGetter(m1, PriorityNormal)
	m.AddGetter(m2, PriorityDefault)
	m.AddGetter(m3, PriorityConfig)
	assert.Equal(t, []getprio{
		{m1, PriorityNormal},
		{m3, PriorityConfig},
		{m2, PriorityDefault},
	}, m.getters)
	m.ClearGetters(PriorityConfig)
	assert.Equal(t, []getprio{
		{m1, PriorityNormal},
		{m2, PriorityDefault},
	}, m.getters)
	m.ClearGetters(PriorityNormal)
	assert.Equal(t, []getprio{
		{m2, PriorityDefault},
	}, m.getters)
	m.ClearGetters(PriorityDefault)
	assert.Equal(t, []getprio{}, m.getters)
	m.ClearGetters(PriorityDefault)
	assert.Equal(t, []getprio{}, m.getters)
}

func TestConfigMapClearSetters(t *testing.T) {
	m := New()
	m1 := Simple{}
	m2 := Simple{}
	m3 := Simple{}
	m.AddSetter(m1)
	m.AddSetter(m2)
	m.AddSetter(m3)
	assert.Equal(t, []Setter{m1, m2, m3}, m.setters)
	m.ClearSetters()
	assert.Equal(t, []Setter(nil), m.setters)
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

func TestSimpleEncode(t *testing.T) {
	for _, test := range []struct {
		in   Simple
		want string
	}{
		{
			in:   Simple{},
			want: "",
		},
		{
			in: Simple{
				"one": "potato",
			},
			want: "eyJvbmUiOiJwb3RhdG8ifQ",
		},
		{
			in: Simple{
				"one": "potato",
				"two": "",
			},
			want: "eyJvbmUiOiJwb3RhdG8iLCJ0d28iOiIifQ",
		},
	} {
		got, err := test.in.Encode()
		require.NoError(t, err)
		assert.Equal(t, test.want, got)
		gotM := Simple{}
		err = gotM.Decode(got)
		require.NoError(t, err)
		assert.Equal(t, test.in, gotM)
	}
}

func TestSimpleDecode(t *testing.T) {
	for _, test := range []struct {
		in      string
		want    Simple
		wantErr string
	}{
		{
			in:   "",
			want: Simple{},
		},
		{
			in: "eyJvbmUiOiJwb3RhdG8ifQ",
			want: Simple{
				"one": "potato",
			},
		},
		{
			in: "   e yJvbm  UiOiJwb\r\n 3Rhd\tG8ifQ\n\n ",
			want: Simple{
				"one": "potato",
			},
		},
		{
			in: "eyJvbmUiOiJwb3RhdG8iLCJ0d28iOiIifQ",
			want: Simple{
				"one": "potato",
				"two": "",
			},
		},
		{
			in:      "!!!!!",
			want:    Simple{},
			wantErr: "decode simple map",
		},
		{
			in:   base64.RawStdEncoding.EncodeToString([]byte(`null`)),
			want: Simple{},
		},
		{
			in:      base64.RawStdEncoding.EncodeToString([]byte(`rubbish`)),
			want:    Simple{},
			wantErr: "parse simple map",
		},
	} {
		got := Simple{}
		err := got.Decode(test.in)
		assert.Equal(t, test.want, got, test.in)
		if test.wantErr == "" {
			require.NoError(t, err, test.in)
		} else {
			assert.Contains(t, err.Error(), test.wantErr, test.in)
		}
	}
}
