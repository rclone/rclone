package rc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyOptions struct {
	StringOpt string   `config:"string_opt"`
	IntOpt    int      `config:"int_opt"`
	BoolOpt   bool     `config:"bool_opt"`
	SliceOpt  []string `config:"slice_opt"`
}

func TestParseOptions(t *testing.T) {
	t.Run("FlatOnly", func(t *testing.T) {
		in := Params{
			"string_opt": "hello",
			"int_opt":    42,
			"bool_opt":   true,
			"slice_opt":  []any{"a", "b"},
		}
		var opt dummyOptions
		err := ParseOptions(in, "dummy", &opt)
		require.NoError(t, err)
		assert.Equal(t, "hello", opt.StringOpt)
		assert.Equal(t, 42, opt.IntOpt)
		assert.True(t, opt.BoolOpt)
		assert.Equal(t, []string{"a", "b"}, opt.SliceOpt)
		assert.Empty(t, in)
	})

	t.Run("NestedOnly", func(t *testing.T) {
		in := Params{
			"dummy": Params{
				"StringOpt": "nested_hello",
				"IntOpt":    100,
				"BoolOpt":   false,
				"SliceOpt":  []string{"x", "y"},
			},
		}
		var opt dummyOptions
		err := ParseOptions(in, "dummy", &opt)
		require.NoError(t, err)
		assert.Equal(t, "nested_hello", opt.StringOpt)
		assert.Equal(t, 100, opt.IntOpt)
		assert.False(t, opt.BoolOpt)
		assert.Equal(t, []string{"x", "y"}, opt.SliceOpt)
		assert.Empty(t, in)
	})

	t.Run("BothPrecedence", func(t *testing.T) {
		in := Params{
			"string_opt": "flat_value",
			"int_opt":    42,
			"dummy": Params{
				"StringOpt": "nested_value",
			},
		}
		var opt dummyOptions
		err := ParseOptions(in, "dummy", &opt)
		require.NoError(t, err)
		assert.Equal(t, "nested_value", opt.StringOpt) // Nested takes precedence
		assert.Equal(t, 42, opt.IntOpt)                // Flat is still parsed
		assert.Empty(t, in)
	})

	t.Run("MapSkip", func(t *testing.T) {
		in := Params{
			"string_opt": Params{"nested_key": "some_value"}, // string_opt matches but value is a map, should be skipped
			"int_opt":    42,
		}
		var opt dummyOptions
		err := ParseOptions(in, "dummy", &opt)
		require.NoError(t, err)
		assert.Equal(t, "", opt.StringOpt) // skipped
		assert.Equal(t, 42, opt.IntOpt)
		assert.Len(t, in, 1)
		assert.Contains(t, in, "string_opt")
	})

	t.Run("NullErrors", func(t *testing.T) {
		in := Params{
			"string_opt": nil, // string_opt is nil, configstruct should return error
		}
		var opt dummyOptions
		err := ParseOptions(in, "dummy", &opt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "interpreting <nil> as string failed")
	})

	t.Run("UnknownLeftBehind", func(t *testing.T) {
		in := Params{
			"string_opt": "value",
			"unknown":    "leftover",
		}
		var opt dummyOptions
		err := ParseOptions(in, "dummy", &opt)
		require.NoError(t, err)
		assert.Equal(t, "value", opt.StringOpt)
		assert.Len(t, in, 1)
		assert.Equal(t, "leftover", in["unknown"])
	})
}

func TestCheckParamsUsed(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		in := Params{}
		err := CheckParamsUsed(in)
		assert.NoError(t, err)
	})

	t.Run("Leftovers", func(t *testing.T) {
		in := Params{
			"z": "last",
			"a": "first",
		}
		err := CheckParamsUsed(in)
		assert.Error(t, err)
		assert.Equal(t, "unknown parameters: a, z", err.Error())
	})
}
