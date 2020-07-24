package rc

import (
	"fmt"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rclone/rclone/fs"
)

func TestErrParamNotFoundError(t *testing.T) {
	e := ErrParamNotFound("key")
	assert.Equal(t, "Didn't find key \"key\" in input", e.Error())
}

func TestIsErrParamNotFound(t *testing.T) {
	assert.Equal(t, true, IsErrParamNotFound(ErrParamNotFound("key")))
	assert.Equal(t, false, IsErrParamNotFound(nil))
	assert.Equal(t, false, IsErrParamNotFound(errors.New("potato")))
}

func TestNotErrParamNotFound(t *testing.T) {
	assert.Equal(t, false, NotErrParamNotFound(ErrParamNotFound("key")))
	assert.Equal(t, false, NotErrParamNotFound(nil))
	assert.Equal(t, true, NotErrParamNotFound(errors.New("potato")))
}

func TestIsErrParamInvalid(t *testing.T) {
	e := ErrParamInvalid{errors.New("potato")}
	assert.Equal(t, true, IsErrParamInvalid(e))
	assert.Equal(t, false, IsErrParamInvalid(nil))
	assert.Equal(t, false, IsErrParamInvalid(errors.New("potato")))
}

func TestReshape(t *testing.T) {
	in := Params{
		"String": "hello",
		"Float":  4.2,
	}
	var out struct {
		String string
		Float  float64
	}
	require.NoError(t, Reshape(&out, in))
	assert.Equal(t, "hello", out.String)
	assert.Equal(t, 4.2, out.Float)
	var inCopy = Params{}
	require.NoError(t, Reshape(&inCopy, out))
	assert.Equal(t, in, inCopy)

	// Now a failure to marshal
	var in2 func()
	require.Error(t, Reshape(&inCopy, in2))

	// Now a failure to unmarshal
	require.Error(t, Reshape(&out, "string"))

}

func TestParamsGet(t *testing.T) {
	in := Params{
		"ok": 1,
	}
	v1, e1 := in.Get("ok")
	assert.NoError(t, e1)
	assert.Equal(t, 1, v1)
	v2, e2 := in.Get("notOK")
	assert.Error(t, e2)
	assert.Equal(t, nil, v2)
	assert.Equal(t, ErrParamNotFound("notOK"), e2)
}

func TestParamsGetString(t *testing.T) {
	in := Params{
		"string":    "one",
		"notString": 17,
	}
	v1, e1 := in.GetString("string")
	assert.NoError(t, e1)
	assert.Equal(t, "one", v1)
	v2, e2 := in.GetString("notOK")
	assert.Error(t, e2)
	assert.Equal(t, "", v2)
	assert.Equal(t, ErrParamNotFound("notOK"), e2)
	v3, e3 := in.GetString("notString")
	assert.Error(t, e3)
	assert.Equal(t, "", v3)
	assert.Equal(t, true, IsErrParamInvalid(e3), e3.Error())
}

func TestParamsGetInt64(t *testing.T) {
	for _, test := range []struct {
		value     interface{}
		result    int64
		errString string
	}{
		{"123", 123, ""},
		{"123x", 0, "couldn't parse"},
		{int(12), 12, ""},
		{int64(13), 13, ""},
		{float64(14), 14, ""},
		{float64(9.3e18), 0, "overflows int64"},
		{float64(-9.3e18), 0, "overflows int64"},
	} {
		t.Run(fmt.Sprintf("%T=%v", test.value, test.value), func(t *testing.T) {
			in := Params{
				"key": test.value,
			}
			v1, e1 := in.GetInt64("key")
			if test.errString == "" {
				require.NoError(t, e1)
				assert.Equal(t, test.result, v1)
			} else {
				require.NotNil(t, e1)
				require.Error(t, e1)
				assert.Contains(t, e1.Error(), test.errString)
				assert.Equal(t, int64(0), v1)
			}
		})
	}
	in := Params{
		"notInt64": []string{"a", "b"},
	}
	v2, e2 := in.GetInt64("notOK")
	assert.Error(t, e2)
	assert.Equal(t, int64(0), v2)
	assert.Equal(t, ErrParamNotFound("notOK"), e2)
	v3, e3 := in.GetInt64("notInt64")
	assert.Error(t, e3)
	assert.Equal(t, int64(0), v3)
	assert.Equal(t, true, IsErrParamInvalid(e3), e3.Error())
}

func TestParamsGetFloat64(t *testing.T) {
	for _, test := range []struct {
		value     interface{}
		result    float64
		errString string
	}{
		{"123.1", 123.1, ""},
		{"123x1", 0, "couldn't parse"},
		{int(12), 12, ""},
		{int64(13), 13, ""},
		{float64(14), 14, ""},
	} {
		t.Run(fmt.Sprintf("%T=%v", test.value, test.value), func(t *testing.T) {
			in := Params{
				"key": test.value,
			}
			v1, e1 := in.GetFloat64("key")
			if test.errString == "" {
				require.NoError(t, e1)
				assert.Equal(t, test.result, v1)
			} else {
				require.NotNil(t, e1)
				require.Error(t, e1)
				assert.Contains(t, e1.Error(), test.errString)
				assert.Equal(t, float64(0), v1)
			}
		})
	}
	in := Params{
		"notFloat64": []string{"a", "b"},
	}
	v2, e2 := in.GetFloat64("notOK")
	assert.Error(t, e2)
	assert.Equal(t, float64(0), v2)
	assert.Equal(t, ErrParamNotFound("notOK"), e2)
	v3, e3 := in.GetFloat64("notFloat64")
	assert.Error(t, e3)
	assert.Equal(t, float64(0), v3)
	assert.Equal(t, true, IsErrParamInvalid(e3), e3.Error())
}

func TestParamsGetDuration(t *testing.T) {
	for _, test := range []struct {
		value     interface{}
		result    time.Duration
		errString string
	}{
		{"86400", time.Hour * 24, ""},
		{"1y", time.Hour * 24 * 365, ""},
		{"60", time.Minute * 1, ""},
		{"0", 0, ""},
		{"-45", -time.Second * 45, ""},
		{"2", time.Second * 2, ""},
		{"2h4m7s", time.Hour*2 + 4*time.Minute + 7*time.Second, ""},
		{"3d", time.Hour * 24 * 3, ""},
		{"off", time.Duration(fs.DurationOff), ""},
		{"", 0, "parse duration"},
		{12, 0, "expecting string"},
		{"34y", time.Hour * 24 * 365 * 34, ""},
		{"30d", time.Hour * 24 * 30, ""},
		{"2M", time.Hour * 24 * 60, ""},
		{"wrong", 0, "parse duration"},
	} {
		t.Run(fmt.Sprintf("%T=%v", test.value, test.value), func(t *testing.T) {
			in := Params{
				"key": test.value,
			}
			v1, e1 := in.GetDuration("key")
			if test.errString == "" {
				require.NoError(t, e1)
				assert.Equal(t, test.result, v1)
			} else {
				require.NotNil(t, e1)
				require.Error(t, e1)
				assert.Contains(t, e1.Error(), test.errString)
				assert.Equal(t, time.Duration(0), v1)
			}
		})
	}
	in := Params{
		"notDuration": []string{"a", "b"},
	}
	v2, e2 := in.GetDuration("notOK")
	assert.Error(t, e2)
	assert.Equal(t, time.Duration(0), v2)
	assert.Equal(t, ErrParamNotFound("notOK"), e2)
	v3, e3 := in.GetDuration("notDuration")
	assert.Error(t, e3)
	assert.Equal(t, time.Duration(0), v3)
	assert.Equal(t, true, IsErrParamInvalid(e3), e3.Error())
}

func TestParamsGetBool(t *testing.T) {
	for _, test := range []struct {
		value     interface{}
		result    bool
		errString string
	}{
		{true, true, ""},
		{false, false, ""},
		{"true", true, ""},
		{"false", false, ""},
		{"fasle", false, "couldn't parse"},
		{int(12), true, ""},
		{int(0), false, ""},
		{int64(13), true, ""},
		{int64(0), false, ""},
		{float64(14), true, ""},
		{float64(0), false, ""},
	} {
		t.Run(fmt.Sprintf("%T=%v", test.value, test.value), func(t *testing.T) {
			in := Params{
				"key": test.value,
			}
			v1, e1 := in.GetBool("key")
			if test.errString == "" {
				require.NoError(t, e1)
				assert.Equal(t, test.result, v1)
			} else {
				require.NotNil(t, e1)
				require.Error(t, e1)
				assert.Contains(t, e1.Error(), test.errString)
				assert.Equal(t, false, v1)
			}
		})
	}
	in := Params{
		"notBool": []string{"a", "b"},
	}
	v2, e2 := Params{}.GetBool("notOK")
	assert.Error(t, e2)
	assert.Equal(t, false, v2)
	assert.Equal(t, ErrParamNotFound("notOK"), e2)
	v3, e3 := in.GetBool("notBool")
	assert.Error(t, e3)
	assert.Equal(t, false, v3)
	assert.Equal(t, true, IsErrParamInvalid(e3), e3.Error())
}

func TestParamsGetStruct(t *testing.T) {
	in := Params{
		"struct": Params{
			"String": "one",
			"Float":  4.2,
		},
	}
	var out struct {
		String string
		Float  float64
	}
	e1 := in.GetStruct("struct", &out)
	assert.NoError(t, e1)
	assert.Equal(t, "one", out.String)
	assert.Equal(t, 4.2, out.Float)

	e2 := in.GetStruct("notOK", &out)
	assert.Error(t, e2)
	assert.Equal(t, "one", out.String)
	assert.Equal(t, 4.2, out.Float)
	assert.Equal(t, ErrParamNotFound("notOK"), e2)

	in["struct"] = "string"
	e3 := in.GetStruct("struct", &out)
	assert.Error(t, e3)
	assert.Equal(t, "one", out.String)
	assert.Equal(t, 4.2, out.Float)
	assert.Equal(t, true, IsErrParamInvalid(e3), e3.Error())
}

func TestParamsGetStructString(t *testing.T) {
	in := Params{
		"struct": `{"String": "one", "Float": 4.2}`,
	}
	var out struct {
		String string
		Float  float64
	}
	e1 := in.GetStruct("struct", &out)
	assert.NoError(t, e1)
	assert.Equal(t, "one", out.String)
	assert.Equal(t, 4.2, out.Float)
}

func TestParamsGetStructMissingOK(t *testing.T) {
	in := Params{
		"struct": Params{
			"String": "one",
			"Float":  4.2,
		},
	}
	var out struct {
		String string
		Float  float64
	}
	e1 := in.GetStructMissingOK("struct", &out)
	assert.NoError(t, e1)
	assert.Equal(t, "one", out.String)
	assert.Equal(t, 4.2, out.Float)

	e2 := in.GetStructMissingOK("notOK", &out)
	assert.NoError(t, e2)
	assert.Equal(t, "one", out.String)
	assert.Equal(t, 4.2, out.Float)

	in["struct"] = "string"
	e3 := in.GetStructMissingOK("struct", &out)
	assert.Error(t, e3)
	assert.Equal(t, "one", out.String)
	assert.Equal(t, 4.2, out.Float)
	assert.Equal(t, true, IsErrParamInvalid(e3), e3.Error())
}
