package fs

import (
	"strings"
	"testing"

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
