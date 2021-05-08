package fs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/assert"
)

func TestFeaturesDisable(t *testing.T) {
	ft := new(Features)
	ft.Copy = func(ctx context.Context, src Object, remote string) (Object, error) {
		return nil, nil
	}
	ft.CaseInsensitive = true

	assert.NotNil(t, ft.Copy)
	assert.Nil(t, ft.Purge)
	ft.Disable("copy")
	assert.Nil(t, ft.Copy)
	assert.Nil(t, ft.Purge)

	assert.True(t, ft.CaseInsensitive)
	assert.False(t, ft.DuplicateFiles)
	ft.Disable("caseinsensitive")
	assert.False(t, ft.CaseInsensitive)
	assert.False(t, ft.DuplicateFiles)
}

func TestFeaturesList(t *testing.T) {
	ft := new(Features)
	names := strings.Join(ft.List(), ",")
	assert.True(t, strings.Contains(names, ",Copy,"))
}

func TestFeaturesEnabled(t *testing.T) {
	ft := new(Features)
	ft.CaseInsensitive = true
	ft.Purge = func(ctx context.Context, dir string) error { return nil }
	enabled := ft.Enabled()

	flag, ok := enabled["CaseInsensitive"]
	assert.Equal(t, true, ok)
	assert.Equal(t, true, flag, enabled)

	flag, ok = enabled["Purge"]
	assert.Equal(t, true, ok)
	assert.Equal(t, true, flag, enabled)

	flag, ok = enabled["DuplicateFiles"]
	assert.Equal(t, true, ok)
	assert.Equal(t, false, flag, enabled)

	flag, ok = enabled["Copy"]
	assert.Equal(t, true, ok)
	assert.Equal(t, false, flag, enabled)

	assert.Equal(t, len(ft.List()), len(enabled))
}

func TestFeaturesDisableList(t *testing.T) {
	ft := new(Features)
	ft.Copy = func(ctx context.Context, src Object, remote string) (Object, error) {
		return nil, nil
	}
	ft.CaseInsensitive = true

	assert.NotNil(t, ft.Copy)
	assert.Nil(t, ft.Purge)
	assert.True(t, ft.CaseInsensitive)
	assert.False(t, ft.DuplicateFiles)

	ft.DisableList([]string{"copy", "caseinsensitive"})

	assert.Nil(t, ft.Copy)
	assert.Nil(t, ft.Purge)
	assert.False(t, ft.CaseInsensitive)
	assert.False(t, ft.DuplicateFiles)
}

// Check it satisfies the interface
var _ pflag.Value = (*Option)(nil)

func TestOption(t *testing.T) {
	d := &Option{
		Name:  "potato",
		Value: SizeSuffix(17 << 20),
	}
	assert.Equal(t, "17Mi", d.String())
	assert.Equal(t, "SizeSuffix", d.Type())
	err := d.Set("18M")
	assert.NoError(t, err)
	assert.Equal(t, SizeSuffix(18<<20), d.Value)
	err = d.Set("sdfsdf")
	assert.Error(t, err)
}

var errFoo = errors.New("foo")

type dummyPaced struct {
	retry  bool
	called int
	wait   *sync.Cond
}

func (dp *dummyPaced) fn() (bool, error) {
	if dp.wait != nil {
		dp.wait.L.Lock()
		dp.wait.Wait()
		dp.wait.L.Unlock()
	}
	dp.called++
	return dp.retry, errFoo
}

func TestPacerCall(t *testing.T) {
	ctx := context.Background()
	config := GetConfig(ctx)
	expectedCalled := config.LowLevelRetries
	if expectedCalled == 0 {
		ctx, config = AddConfig(ctx)
		expectedCalled = 20
		config.LowLevelRetries = expectedCalled
	}
	p := NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(1*time.Millisecond), pacer.MaxSleep(2*time.Millisecond)))

	dp := &dummyPaced{retry: true}
	err := p.Call(dp.fn)
	require.Equal(t, expectedCalled, dp.called)
	require.Implements(t, (*fserrors.Retrier)(nil), err)
}

func TestPacerCallNoRetry(t *testing.T) {
	p := NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(1*time.Millisecond), pacer.MaxSleep(2*time.Millisecond)))

	dp := &dummyPaced{retry: true}
	err := p.CallNoRetry(dp.fn)
	require.Equal(t, 1, dp.called)
	require.Implements(t, (*fserrors.Retrier)(nil), err)
}

// Test options
var (
	nouncOption = Option{
		Name: "nounc",
	}
	copyLinksOption = Option{
		Name:     "copy_links",
		Default:  false,
		NoPrefix: true,
		ShortOpt: "L",
		Advanced: true,
	}
	caseInsensitiveOption = Option{
		Name:     "case_insensitive",
		Default:  false,
		Value:    true,
		Advanced: true,
	}
	testOptions = Options{nouncOption, copyLinksOption, caseInsensitiveOption}
)

func TestOptionsSetValues(t *testing.T) {
	assert.Nil(t, testOptions[0].Default)
	assert.Equal(t, false, testOptions[1].Default)
	assert.Equal(t, false, testOptions[2].Default)
	testOptions.setValues()
	assert.Equal(t, "", testOptions[0].Default)
	assert.Equal(t, false, testOptions[1].Default)
	assert.Equal(t, false, testOptions[2].Default)
}

func TestOptionsGet(t *testing.T) {
	opt := testOptions.Get("copy_links")
	assert.Equal(t, &copyLinksOption, opt)
	opt = testOptions.Get("not_found")
	assert.Nil(t, opt)
}

func TestOptionsOveridden(t *testing.T) {
	m := configmap.New()
	m1 := configmap.Simple{
		"nounc":      "m1",
		"copy_links": "m1",
	}
	m.AddGetter(m1, configmap.PriorityNormal)
	m2 := configmap.Simple{
		"nounc":            "m2",
		"case_insensitive": "m2",
	}
	m.AddGetter(m2, configmap.PriorityConfig)
	m3 := configmap.Simple{
		"nounc": "m3",
	}
	m.AddGetter(m3, configmap.PriorityDefault)
	got := testOptions.Overridden(m)
	assert.Equal(t, configmap.Simple{
		"copy_links": "m1",
		"nounc":      "m1",
	}, got)
}

func TestOptionsNonDefault(t *testing.T) {
	m := configmap.Simple{}
	got := testOptions.NonDefault(m)
	assert.Equal(t, configmap.Simple{}, got)

	m["case_insensitive"] = "false"
	got = testOptions.NonDefault(m)
	assert.Equal(t, configmap.Simple{}, got)

	m["case_insensitive"] = "true"
	got = testOptions.NonDefault(m)
	assert.Equal(t, configmap.Simple{"case_insensitive": "true"}, got)
}

func TestOptionMarshalJSON(t *testing.T) {
	out, err := json.MarshalIndent(&caseInsensitiveOption, "", "")
	assert.NoError(t, err)
	require.Equal(t, `{
"Name": "case_insensitive",
"Help": "",
"Provider": "",
"Default": false,
"Value": true,
"ShortOpt": "",
"Hide": 0,
"Required": false,
"IsPassword": false,
"NoPrefix": false,
"Advanced": true,
"Exclusive": false,
"DefaultStr": "false",
"ValueStr": "true",
"Type": "bool"
}`, string(out))
}

func TestOptionGetValue(t *testing.T) {
	assert.Equal(t, "", nouncOption.GetValue())
	assert.Equal(t, false, copyLinksOption.GetValue())
	assert.Equal(t, true, caseInsensitiveOption.GetValue())
}

func TestOptionString(t *testing.T) {
	assert.Equal(t, "", nouncOption.String())
	assert.Equal(t, "false", copyLinksOption.String())
	assert.Equal(t, "true", caseInsensitiveOption.String())
}

func TestOptionSet(t *testing.T) {
	o := caseInsensitiveOption
	assert.Equal(t, true, o.Value)
	err := o.Set("FALSE")
	assert.NoError(t, err)
	assert.Equal(t, false, o.Value)

	o = copyLinksOption
	assert.Equal(t, nil, o.Value)
	err = o.Set("True")
	assert.NoError(t, err)
	assert.Equal(t, true, o.Value)

	err = o.Set("INVALID")
	assert.Error(t, err)
	assert.Equal(t, true, o.Value)
}

func TestOptionType(t *testing.T) {
	assert.Equal(t, "string", nouncOption.Type())
	assert.Equal(t, "bool", copyLinksOption.Type())
	assert.Equal(t, "bool", caseInsensitiveOption.Type())
}

func TestOptionFlagName(t *testing.T) {
	assert.Equal(t, "local-nounc", nouncOption.FlagName("local"))
	assert.Equal(t, "copy-links", copyLinksOption.FlagName("local"))
	assert.Equal(t, "local-case-insensitive", caseInsensitiveOption.FlagName("local"))
}

func TestOptionEnvVarName(t *testing.T) {
	assert.Equal(t, "RCLONE_LOCAL_NOUNC", nouncOption.EnvVarName("local"))
	assert.Equal(t, "RCLONE_LOCAL_COPY_LINKS", copyLinksOption.EnvVarName("local"))
	assert.Equal(t, "RCLONE_LOCAL_CASE_INSENSITIVE", caseInsensitiveOption.EnvVarName("local"))
}

func TestOptionGetters(t *testing.T) {
	// Set up env vars
	envVars := [][2]string{
		{"RCLONE_CONFIG_LOCAL_POTATO_PIE", "yes"},
		{"RCLONE_COPY_LINKS", "TRUE"},
		{"RCLONE_LOCAL_NOUNC", "NOUNC"},
	}
	for _, ev := range envVars {
		assert.NoError(t, os.Setenv(ev[0], ev[1]))
	}
	defer func() {
		for _, ev := range envVars {
			assert.NoError(t, os.Unsetenv(ev[0]))
		}
	}()

	fsInfo := &RegInfo{
		Name:    "local",
		Prefix:  "local",
		Options: testOptions,
	}

	oldConfigFileGet := ConfigFileGet
	ConfigFileGet = func(section, key string) (string, bool) {
		if section == "sausage" && key == "key1" {
			return "value1", true
		}
		return "", false
	}
	defer func() {
		ConfigFileGet = oldConfigFileGet
	}()

	// set up getters

	// A configmap.Getter to read from the environment RCLONE_CONFIG_backend_option_name
	configEnvVarsGetter := configEnvVars("local")

	// A configmap.Getter to read from the environment RCLONE_option_name
	optionEnvVarsGetter := optionEnvVars{fsInfo}

	// A configmap.Getter to read either the default value or the set
	// value from the RegInfo.Options
	regInfoValuesGetterFalse := &regInfoValues{
		fsInfo:     fsInfo,
		useDefault: false,
	}
	regInfoValuesGetterTrue := &regInfoValues{
		fsInfo:     fsInfo,
		useDefault: true,
	}

	// A configmap.Setter to read from the config file
	configFileGetter := getConfigFile("sausage")

	for i, test := range []struct {
		get       configmap.Getter
		key       string
		wantValue string
		wantOk    bool
	}{
		{configEnvVarsGetter, "not_found", "", false},
		{configEnvVarsGetter, "potato_pie", "yes", true},
		{optionEnvVarsGetter, "not_found", "", false},
		{optionEnvVarsGetter, "copy_links", "TRUE", true},
		{optionEnvVarsGetter, "nounc", "NOUNC", true},
		{optionEnvVarsGetter, "case_insensitive", "", false},
		{regInfoValuesGetterFalse, "not_found", "", false},
		{regInfoValuesGetterFalse, "case_insensitive", "true", true},
		{regInfoValuesGetterFalse, "copy_links", "", false},
		{regInfoValuesGetterTrue, "not_found", "", false},
		{regInfoValuesGetterTrue, "case_insensitive", "true", true},
		{regInfoValuesGetterTrue, "copy_links", "false", true},
		{configFileGetter, "not_found", "", false},
		{configFileGetter, "key1", "value1", true},
	} {
		what := fmt.Sprintf("%d: %+v: %q", i, test.get, test.key)
		gotValue, gotOk := test.get.Get(test.key)
		assert.Equal(t, test.wantValue, gotValue, what)
		assert.Equal(t, test.wantOk, gotOk, what)
	}

}
