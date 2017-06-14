package yaml

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYAMLDecodeUnknown(t *testing.T) {
	yamlString := `
key1: "This is a string." # Single Line Comment
key2: 10.50
key3:
  - null
  - nestedKey1: Anothor string
`

	anyData, err := Decode([]byte(yamlString))
	assert.NoError(t, err)
	data := anyData.(map[interface{}]interface{})
	assert.Equal(t, 10.50, data["key2"])
}

func TestYAMLDecodeKnown(t *testing.T) {
	type SampleYAML struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	sampleYAMLString := `name: "NAME"`

	sample := SampleYAML{Name: "NaMe", Description: "DeScRiPtIoN"}
	anyDataPointer, err := Decode([]byte(sampleYAMLString), &sample)
	assert.NoError(t, err)
	data := anyDataPointer.(*SampleYAML)
	assert.Equal(t, "NAME", sample.Name)
	assert.Equal(t, "DeScRiPtIoN", sample.Description)
	assert.Equal(t, "NAME", (*data).Name)
	assert.Equal(t, "DeScRiPtIoN", (*data).Description)

	_, err = Decode([]byte(`- - -`), &YAMLMustError{})
	assert.Error(t, err)
}

func TestYAMLDecodeEmpty(t *testing.T) {
	yamlString := ""

	anyData, err := Decode([]byte(yamlString))
	assert.NoError(t, err)
	assert.Nil(t, anyData)
}

func TestYAMLEncode(t *testing.T) {
	type SampleYAML struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	sample := SampleYAML{Name: "NaMe", Description: "DeScRiPtIoN"}

	yamlBytes, err := Encode(sample)
	assert.NoError(t, err)
	assert.Equal(t, "name: NaMe\ndescription: DeScRiPtIoN\n", string(yamlBytes))

	_, err = Encode(&YAMLMustError{})
	assert.Error(t, err)
}

type YAMLMustError struct{}

func (*YAMLMustError) MarshalYAML() (interface{}, error) {
	return nil, errors.New("marshal error")
}
