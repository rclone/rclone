package fs

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestFeaturesDisable(t *testing.T) {
	ft := new(Features)
	ft.Copy = func(src Object, remote string) (Object, error) {
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

func TestFeaturesDisableList(t *testing.T) {
	ft := new(Features)
	ft.Copy = func(src Object, remote string) (Object, error) {
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
	assert.Equal(t, "17M", d.String())
	assert.Equal(t, "SizeSuffix", d.Type())
	err := d.Set("18M")
	assert.NoError(t, err)
	assert.Equal(t, SizeSuffix(18<<20), d.Value)
	err = d.Set("sdfsdf")
	assert.Error(t, err)
}
